package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaruWait     = 10 * time.Second
	qaruStrike   = "fists"
	qaruUnnamed  = "chair-swing"
	qaruOne      = "qaru-one"
	qaruTwo      = "qaru-two"
	qaruNobody   = "qaru-nobody"
	qaruRoomID   = "qaru-room"
	qaruNewBlurb = "QA rewrote this description."
)

type qaruSet struct{ id, version, name, blurb string }

func qaruRuleset(t *testing.T, s qaruSet) *rules.Ruleset {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "set")
	if err := os.CopyFS(dst, os.DirFS(filepath.Join("..", "..", "rulesets", "tavern-brawl"))); err != nil {
		t.Fatalf("copy ruleset: %v", err)
	}
	qaruRewrite(t, filepath.Join(dst, "ruleset.json"), map[string]any{"id": s.id, "version": s.version, "name": s.name})
	if s.blurb != "" {
		qaruRewrite(t, filepath.Join(dst, "abilities", qaruStrike+".json"), map[string]any{"description": s.blurb})
	}
	rs, err := rules.Load(dst)
	if err != nil {
		t.Fatalf("load ruleset %+v: %v", s, err)
	}
	return rs
}

func qaruRewrite(t *testing.T, path string, fields map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		doc[k] = v
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

func qaruRoom() *mapdef.Map {
	return &mapdef.Map{
		FormatVersion: mapdef.MapFormatVersion, ID: qaruRoomID, Name: "Room", GridW: 4, GridH: 3,
		Placements: []mapdef.Placement{
			{TokenID: "qaru-t1", ActorID: qaruOne, X: 1, Y: 1},
			{TokenID: "qaru-t2", ActorID: qaruTwo, X: 2, Y: 1},
		},
	}
}

type qaruTable struct {
	t     *testing.T
	dir   string
	c     *campaign.Campaign
	ids   *identity.DB
	hs    *httptest.Server
	conns []*qaruConn
	tok   map[string]string
	pid   map[string]string
}

func qaruNewTable(t *testing.T) *qaruTable {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "camp")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	return &qaruTable{t: t, dir: dir, tok: map[string]string{}, pid: map[string]string{}}
}

// open serves the campaign under rs, or under no ruleset for nil.
func (tb *qaruTable) open(rs *rules.Ruleset) {
	tb.t.Helper()
	c, err := campaign.Open(tb.dir)
	if err != nil {
		tb.t.Fatalf("open campaign: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(tb.dir))
	if err != nil {
		tb.t.Fatalf("open identity: %v", err)
	}
	tb.c, tb.ids = c, ids
	if len(tb.tok) == 0 {
		seats := map[string]identity.Role{
			"dm": identity.RoleDM, "agent": identity.RoleAgent,
			"player": identity.RoleSpectator, "spectator": identity.RoleSpectator,
		}
		for seat, role := range seats {
			tok, id, err := ids.CreateInvite("qaru "+seat, role)
			if err != nil {
				tb.t.Fatalf("invite %s: %v", seat, err)
			}
			tb.tok[seat], tb.pid[seat] = tok, id
		}
	}
	srv := gateway.New(c, ids).WithMaps(map[string]*mapdef.Map{qaruRoomID: qaruRoom()})
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	tb.hs = httptest.NewServer(srv.Handler())
	tb.t.Cleanup(tb.shut)
}

// shut closes every connection before the campaign: one writer at a time.
func (tb *qaruTable) shut() {
	if tb.hs == nil {
		return
	}
	for _, c := range tb.conns {
		c.close()
	}
	tb.conns = nil
	tb.hs.Close()
	tb.hs = nil
	if err := tb.ids.Close(); err != nil {
		tb.t.Errorf("close identity: %v", err)
	}
	if err := tb.c.Close(); err != nil {
		tb.t.Errorf("close campaign: %v", err)
	}
}

// boot adds two actors, places them side by side and makes "player" control
// the first; it returns the DM's connection, dialled with log naming an actor.
func (tb *qaruTable) boot() *qaruConn {
	tb.t.Helper()
	dm := tb.dial("dm", "&after=0&log="+qaruOne)
	sheet := func(id string, kind vttv1.ActorKind) *vttv1.ClientCommand {
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{
			ActorId: id, Name: id, Kind: kind,
			Attributes: map[string]int32{"brawn": 2, "grit": 1, "footing": 10},
			Resources:  map[string]*vttv1.Resource{"drink": {Current: 0, Max: 4}},
			AbilityIds: []string{qaruStrike, "sober-up", "splash-of-water"},
		}}}}
	}
	for _, cmd := range []*vttv1.ClientCommand{
		sheet(qaruOne, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		sheet(qaruTwo, vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		{Command: &vttv1.ClientCommand_LoadMap{LoadMap: &vttv1.LoadMap{MapId: qaruRoomID}}},
		{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: qaruOne, ParticipantId: tb.pid["player"], Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
	} {
		if r := dm.send(cmd); !r.GetOk() {
			tb.t.Fatalf("boot command %T refused: %s", cmd.GetCommand(), r.GetError())
		}
	}
	return dm
}

func (tb *qaruTable) dial(seat, query string) *qaruConn {
	tb.t.Helper()
	u := "ws" + strings.TrimPrefix(tb.hs.URL, "http") + "/ws?token=" + url.QueryEscape(tb.tok[seat]) + query
	ctx, cancel := context.WithCancel(context.Background())
	ws, resp, err := websocket.Dial(ctx, u, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		cancel()
		tb.t.Fatalf("dial %s: %v", seat, err)
	}
	ws.SetReadLimit(1 << 22)
	c := &qaruConn{t: tb.t, seat: seat, ws: ws, cancel: cancel, poke: make(chan struct{}, 1), done: make(chan struct{})}
	go c.read()
	tb.conns = append(tb.conns, c)
	return c
}

type qaruConn struct {
	t      *testing.T
	seat   string
	ws     *websocket.Conn
	cancel context.CancelFunc
	mu     sync.Mutex
	frames []*vttv1.ServerFrame
	bad    error
	poke   chan struct{}
	done   chan struct{}
	n      int
}

func (c *qaruConn) read() {
	defer close(c.done)
	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			return
		}
		f := &vttv1.ServerFrame{}
		if err := protojson.Unmarshal(data, f); err != nil {
			c.mu.Lock()
			c.bad = err
			c.mu.Unlock()
			return
		}
		c.mu.Lock()
		c.frames = append(c.frames, f)
		c.mu.Unlock()
		select {
		case c.poke <- struct{}{}:
		default:
		}
	}
}

func (c *qaruConn) close() {
	_ = c.ws.Close(websocket.StatusNormalClosure, "")
	<-c.done
	c.cancel()
}

func (c *qaruConn) find(match func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range c.frames {
		if match(f) {
			return f
		}
	}
	return nil
}

func (c *qaruConn) await(what string, match func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	c.t.Helper()
	deadline := time.NewTimer(qaruWait)
	defer deadline.Stop()
	for {
		if f := c.find(match); f != nil {
			return f
		}
		select {
		case <-c.poke:
		case <-c.done:
			if f := c.find(match); f != nil {
				return f
			}
			c.t.Fatalf("%s's connection ended before %s (decode error: %v)", c.seat, what, c.bad)
		case <-deadline.C:
			c.t.Fatalf("%s was not sent %s within %s", c.seat, what, qaruWait)
		}
	}
}

func (c *qaruConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	c.n++
	id := fmt.Sprintf("qaru-%s-%d", c.seat, c.n)
	cmd.RequestId = id
	data, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaruWait)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
		c.t.Fatalf("%s write: %v", c.seat, err)
	}
	return c.await("the result of "+id, func(f *vttv1.ServerFrame) bool {
		return f.GetResult().GetRequestId() == id
	}).GetResult()
}

func (c *qaruConn) envelopes() []*vttv1.Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []*vttv1.Envelope
	for _, f := range c.frames {
		if e := f.GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

// world returns the world from sequence 1 through seq, read on a world seat.
func (c *qaruConn) world(seq int64) map[int64]*vttv1.Envelope {
	c.t.Helper()
	c.await(fmt.Sprintf("world event %d", seq), func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetSequence() == seq
	})
	w := map[int64]*vttv1.Envelope{}
	for _, e := range c.envelopes() {
		w[e.GetSequence()] = e
	}
	for s := int64(1); s <= seq; s++ {
		if w[s] == nil {
			c.t.Fatalf("%s's world lacks sequence %d", c.seat, s)
		}
	}
	return w
}

func (c *qaruConn) head() int64 {
	c.t.Helper()
	return c.await("the catch-up head", func(f *vttv1.ServerFrame) bool {
		return f.GetCatchUpHead() != nil
	}).GetCatchUpHead().GetHeadSequence()
}

func qaruUse(actor, ability string, targets ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: actor, AbilityId: ability, TargetIds: targets,
	}}}
}

func qaruSay(text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
		Text: text, As: "Narrator",
	}}}
}

func qaruOK(t *testing.T, what string, r *vttv1.CommandResult) int64 {
	t.Helper()
	if !r.GetOk() || r.GetSequence() == 0 {
		t.Fatalf("%s: want ok with a sequence, got ok=%v sequence=%d error=%q", what, r.GetOk(), r.GetSequence(), r.GetError())
	}
	return r.GetSequence()
}

func qaruEntryIs(t *testing.T, what string, e *vttv1.Envelope, s qaruSet) {
	t.Helper()
	rn := e.GetRulesetNamed()
	if rn == nil {
		t.Fatalf("%s: sequence %d is %T, want a RulesetNamed of %+v", what, e.GetSequence(), e.GetPayload(), s)
	}
	if rn.GetRulesetId() != s.id || rn.GetVersion() != s.version || rn.GetName() != s.name {
		t.Errorf("%s: RulesetNamed{%q %q %q}, want {%q %q %q}", what,
			rn.GetRulesetId(), rn.GetVersion(), rn.GetName(), s.id, s.version, s.name)
	}
}

func qaruOutcomeIs(t *testing.T, what string, e *vttv1.Envelope, actor string) {
	t.Helper()
	au := e.GetAbilityUsed()
	if au == nil || au.GetActorId() != actor || au.GetAbilityId() != qaruStrike {
		t.Fatalf("%s: sequence %d is %T (%v), want the AbilityUsed of %s's %s", what, e.GetSequence(), e.GetPayload(), au, actor, qaruStrike)
	}
}

func qaruEntries(w map[int64]*vttv1.Envelope) int {
	n := 0
	for _, e := range w {
		if e.GetRulesetNamed() != nil {
			n++
		}
	}
	return n
}

func TestQARUTheFirstResolvedUseAppendsTheEntryAndThenTheOutcomeInOneBatch(t *testing.T) {
	alpha := qaruSet{id: "qaru-alpha", version: "1", name: "Alpha Rules"}
	tb := qaruNewTable(t)
	tb.open(qaruRuleset(t, alpha))
	dm := tb.boot()
	pl := tb.dial("player", "")
	mark := qaruOK(t, "marker", dm.send(qaruSay("before")))

	first := qaruOK(t, "the player's first use", pl.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	// VTT-975: the result names the batch's first sequence, the entry.
	if first != mark+1 {
		t.Fatalf("VTT-975: first use's result sequence %d, want %d", first, mark+1)
	}
	w := dm.world(first + 1)
	// VTT-975
	qaruEntryIs(t, "VTT-975 first use", w[first], alpha)
	qaruOutcomeIs(t, "VTT-975 the outcome follows the entry", w[first+1], qaruOne)
	// VTT-976: the issuer is the player, not the DM.
	if got := w[first]; got.GetParticipantId() != tb.pid["player"] || got.GetActorRole() != string(identity.RolePlayer) {
		t.Errorf("VTT-976: entry carries participant %q role %q, want %q %q",
			got.GetParticipantId(), got.GetActorRole(), tb.pid["player"], identity.RolePlayer)
	}

	// VTT-978: the log now names the served id and version.
	second := qaruOK(t, "the DM's second use", dm.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	third := qaruOK(t, "the player's third use", pl.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	end := qaruOK(t, "marker", dm.send(qaruSay("after")))
	w = dm.world(end)
	qaruOutcomeIs(t, "VTT-978 second use", w[second], qaruOne)
	qaruOutcomeIs(t, "VTT-978 third use", w[third], qaruOne)
	if n := qaruEntries(w); n != 1 {
		t.Errorf("VTT-978: the world holds %d RulesetNamed after three uses under one ruleset, want 1", n)
	}
}

func TestQARUTheEntryReachesEverySeatAndNamesItsIssuerOnlyOnTheWorld(t *testing.T) {
	alpha := qaruSet{id: "qaru-alpha", version: "1", name: "Alpha Rules"}
	tb := qaruNewTable(t)
	tb.open(qaruRuleset(t, alpha))
	dm := tb.boot()
	ag := tb.dial("agent", "&after=0&log="+qaruTwo)
	pl := tb.dial("player", "")
	sp := tb.dial("spectator", "&after=0&log="+qaruOne)
	for _, c := range []*qaruConn{pl, sp} {
		c.await("a viewpoint on "+qaruOne, func(f *vttv1.ServerFrame) bool {
			return f.GetViewpointChanged().GetActorId() == qaruOne
		})
	}

	seq := qaruOK(t, "the agent's first use", ag.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	// VTT-976, VTT-377: the world seats are sent the issuer, whatever log names.
	for _, c := range []*qaruConn{dm, ag} {
		e := c.world(seq + 1)[seq]
		qaruEntryIs(t, "VTT-377 "+c.seat+"'s stream", e, alpha)
		if e.GetParticipantId() != tb.pid["agent"] || e.GetActorRole() != string(identity.RoleAgent) {
			t.Errorf("VTT-976: %s is sent the entry with participant %q role %q, want %q %q",
				c.seat, e.GetParticipantId(), e.GetActorRole(), tb.pid["agent"], identity.RoleAgent)
		}
	}
	// VTT-272, VTT-980.
	for _, c := range []*qaruConn{pl, sp} {
		c.await("the outcome", func(f *vttv1.ServerFrame) bool {
			return f.GetEvent().GetAbilityUsed().GetActorId() == qaruOne
		})
		entries := 0
		for _, e := range c.envelopes() {
			if e.GetRulesetNamed() != nil {
				entries++
				qaruEntryIs(t, "ruling: "+c.seat+"'s stream", e, alpha)
			}
			if e.GetParticipantId() != "" || e.GetActorRole() != "" {
				t.Errorf("VTT-272: %s is sent %T at %d with participant %q role %q",
					c.seat, e.GetPayload(), e.GetSequence(), e.GetParticipantId(), e.GetActorRole())
			}
		}
		if entries == 0 {
			t.Errorf("ruling: %s was sent the outcome but no RulesetNamed", c.seat)
		}
	}
}

// qaruPhase serves the campaign under s, runs one resolved use from the DM
// and returns the world, the use's result sequence and the log's last
// sequence; prev is the last sequence of the phase before, 0 for none.
func qaruPhase(t *testing.T, tb *qaruTable, s qaruSet, prev int64) (w map[int64]*vttv1.Envelope, seq, end int64) {
	t.Helper()
	tb.open(qaruRuleset(t, s))
	var dm *qaruConn
	if prev == 0 {
		dm = tb.boot()
	} else {
		dm = tb.dial("dm", "&after=0")
		// SPEC-026
		if head := dm.head(); head != prev {
			t.Errorf("ruling: serving %s@%s moved the log from %d to %d before any command", s.id, s.version, prev, head)
		}
	}
	say := qaruOK(t, "marker", dm.send(qaruSay("phase "+s.id+" "+s.version)))
	seq = qaruOK(t, "use under "+s.id+"@"+s.version, dm.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	end = qaruOK(t, "marker", dm.send(qaruSay("end")))
	w = dm.world(end)
	if prev != 0 && (say != prev+1 || w[say].GetNarrationAdded() == nil) {
		t.Errorf("ruling: under %s@%s the first command after the switch got sequence %d (%T), want a narration at %d",
			s.id, s.version, say, w[say].GetPayload(), prev+1)
	}
	tb.shut()
	return w, seq, end
}

func TestQARUAReopenUnderAnotherVersionOrIdNamesItAndARenameDoesNot(t *testing.T) {
	alpha1 := qaruSet{id: "qaru-alpha", version: "1", name: "Alpha Rules"}
	alpha2 := qaruSet{id: "qaru-alpha", version: "2", name: "Alpha Rules"}
	beta2 := qaruSet{id: "qaru-beta", version: "2", name: "Beta Rules"}
	beta2renamed := qaruSet{id: "qaru-beta", version: "2", name: "Beta Renamed", blurb: qaruNewBlurb}
	tb := qaruNewTable(t)

	w, seq, end := qaruPhase(t, tb, alpha1, 0)
	// VTT-975: a log naming none.
	qaruEntryIs(t, "VTT-975 log names none", w[seq], alpha1)
	qaruOutcomeIs(t, "VTT-975 log names none", w[seq+1], qaruOne)

	w, seq, end = qaruPhase(t, tb, alpha2, end)
	// VTT-975: the log last names another version of the served id.
	qaruEntryIs(t, "VTT-975 another version", w[seq], alpha2)
	qaruOutcomeIs(t, "VTT-975 another version", w[seq+1], qaruOne)

	w, seq, end = qaruPhase(t, tb, beta2, end)
	// VTT-975, VTT-977: another id at the same version string is served and named.
	qaruEntryIs(t, "VTT-977 another id", w[seq], beta2)
	qaruOutcomeIs(t, "VTT-977 another id", w[seq+1], qaruOne)

	w, seq, end = qaruPhase(t, tb, beta2renamed, end)
	// VTT-978: same id and version, another name and another description.
	qaruOutcomeIs(t, "VTT-978 renamed", w[seq], qaruOne)
	if n := qaruEntries(w); n != 3 {
		t.Errorf("VTT-978: the world holds %d RulesetNamed after a rename, want 3", n)
	}

	w, seq, _ = qaruPhase(t, tb, alpha1, end)
	// VTT-975: the log names alpha@1 earlier but last names beta@2.
	qaruEntryIs(t, "VTT-975 last names", w[seq], alpha1)
	qaruOutcomeIs(t, "VTT-975 last names", w[seq+1], qaruOne)
	if n := qaruEntries(w); n != 4 {
		t.Errorf("VTT-975: the world holds %d RulesetNamed, want 4", n)
	}
}

func qaruRefused(t *testing.T, what, says string, r *vttv1.CommandResult) {
	t.Helper()
	// VTT-132
	if r.GetOk() || r.GetSequence() != 0 || !strings.Contains(r.GetError(), says) {
		t.Errorf("VTT-132 %s: want ok=false with an error saying %q and no sequence, got ok=%v sequence=%d error=%q",
			what, says, r.GetOk(), r.GetSequence(), r.GetError())
	}
}

func TestQARUARefusedUseAppendsNothingNorTheEntry(t *testing.T) {
	alpha := qaruSet{id: "qaru-alpha", version: "1", name: "Alpha Rules"}
	tb := qaruNewTable(t)

	tb.open(nil)
	dm := tb.boot()
	mark := qaruOK(t, "marker", dm.send(qaruSay("before")))
	qaruRefused(t, "no ruleset served", "no ruleset loaded", dm.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	// VTT-132: the connection stays open; VTT-975: nothing was appended.
	after := qaruOK(t, "marker", dm.send(qaruSay("after")))
	if after != mark+1 {
		t.Errorf("VTT-975: a use with no ruleset served moved the log from %d to %d", mark, after-1)
	}
	tb.shut()

	tb.open(qaruRuleset(t, alpha))
	dm = tb.dial("dm", "&after=0")
	pl := tb.dial("player", "")
	mark = qaruOK(t, "marker", dm.send(qaruSay("before")))
	qaruRefused(t, "the DM's use of an ability the sheet does not name", "does not name ability", dm.send(qaruUse(qaruOne, qaruUnnamed, qaruTwo)))
	qaruRefused(t, "the player's use of an ability the sheet does not name", "does not name ability", pl.send(qaruUse(qaruOne, qaruUnnamed, qaruTwo)))
	qaruRefused(t, "a use at a target that names no actor", qaruNobody, dm.send(qaruUse(qaruOne, qaruStrike, qaruNobody)))
	qaruRefused(t, "a use by an actor the campaign does not hold", qaruNobody, dm.send(qaruUse(qaruNobody, qaruStrike, qaruTwo)))

	// VTT-132: both connections stay open after their refusals.
	seq := qaruOK(t, "the first resolved use", pl.send(qaruUse(qaruOne, qaruStrike, qaruTwo)))
	end := qaruOK(t, "marker", dm.send(qaruSay("end")))
	// VTT-975: the refusals appended no entry, so the resolved use's batch does.
	if seq != mark+1 {
		t.Fatalf("VTT-975: the refusals moved the log: resolved use at %d, want %d", seq, mark+1)
	}
	w := dm.world(end)
	qaruEntryIs(t, "VTT-975 after refusals", w[seq], alpha)
	qaruOutcomeIs(t, "VTT-975 after refusals", w[seq+1], qaruOne)
	if n := qaruEntries(w); n != 1 {
		t.Errorf("VTT-975: the world holds %d RulesetNamed, want 1", n)
	}
}
