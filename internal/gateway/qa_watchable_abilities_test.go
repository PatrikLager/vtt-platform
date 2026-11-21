package gateway_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaWAWait = 5 * time.Second

// qaWAConn is one live WebSocket connection to the real gateway, read on a
// goroutine of its own into a queue a test drains in order.
type qaWAConn struct {
	t      *testing.T
	name   string
	c      *websocket.Conn
	frames chan *vttv1.ServerFrame
	mu     sync.Mutex
	all    []*vttv1.ServerFrame
	reqN   int
}

func qaWADial(t *testing.T, base, name, token, log string) *qaWAConn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(base, "http") + "/ws?token=" + token + "&after=0"
	if log != "" {
		url += "&log=" + log
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaWAWait)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("%s: dial: %v", name, err)
	}
	c.SetReadLimit(1 << 20)
	q := &qaWAConn{t: t, name: name, c: c, frames: make(chan *vttv1.ServerFrame, 4096)}
	go func() {
		defer close(q.frames)
		for {
			_, data, err := c.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := protojson.Unmarshal(data, f); err != nil {
				return
			}
			q.mu.Lock()
			q.all = append(q.all, f)
			q.mu.Unlock()
			q.frames <- f
		}
	}()
	t.Cleanup(func() { _ = c.Close(websocket.StatusNormalClosure, "") })
	return q
}

// next returns the next frame, or nil when none arrives within d.
func (q *qaWAConn) next(d time.Duration) *vttv1.ServerFrame {
	select {
	case f, ok := <-q.frames:
		if !ok {
			return nil
		}
		return f
	case <-time.After(d):
		return nil
	}
}

// nextSet reads until a WatchableChanged and returns it with every frame read
// before it.
func (q *qaWAConn) nextSet() (*vttv1.WatchableChanged, []*vttv1.ServerFrame) {
	q.t.Helper()
	var before []*vttv1.ServerFrame
	deadline := time.Now().Add(qaWAWait)
	for time.Now().Before(deadline) {
		f := q.next(time.Until(deadline))
		if f == nil {
			break
		}
		if w := f.GetWatchableChanged(); w != nil {
			return w, before
		}
		before = append(before, f)
	}
	q.t.Fatalf("%s: no WatchableChanged within %v; read %d other frames", q.name, qaWAWait, len(before))
	return nil, nil
}

// noSetWithin fails when a WatchableChanged arrives within d, and returns the
// frames read meanwhile.
func (q *qaWAConn) noSetWithin(d time.Duration, why string) []*vttv1.ServerFrame {
	q.t.Helper()
	var read []*vttv1.ServerFrame
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		f := q.next(time.Until(deadline))
		if f == nil {
			break
		}
		if w := f.GetWatchableChanged(); w != nil {
			q.t.Fatalf("%s: %s: was sent a WatchableChanged %v", q.name, why, qaWADescribe(w))
		}
		read = append(read, f)
	}
	return read
}

// untilEvent reads until an event whose payload matches, returning it, and
// fails when none arrives.
func (q *qaWAConn) untilEvent(match func(*vttv1.Envelope) bool, what string) []*vttv1.ServerFrame {
	q.t.Helper()
	var read []*vttv1.ServerFrame
	deadline := time.Now().Add(qaWAWait)
	for time.Now().Before(deadline) {
		f := q.next(time.Until(deadline))
		if f == nil {
			break
		}
		read = append(read, f)
		if e := f.GetEvent(); e != nil && match(e) {
			return read
		}
	}
	q.t.Fatalf("%s: no %s within %v", q.name, what, qaWAWait)
	return nil
}

func (q *qaWAConn) do(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	q.t.Helper()
	q.reqN++
	cmd.RequestId = q.name + "-" + string(rune('a'+q.reqN%26)) + time.Now().Format("150405.000000000")
	data, err := protojson.Marshal(cmd)
	if err != nil {
		q.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaWAWait)
	defer cancel()
	if err := q.c.Write(ctx, websocket.MessageText, data); err != nil {
		q.t.Fatalf("%s: write: %v", q.name, err)
	}
	deadline := time.Now().Add(qaWAWait)
	for time.Now().Before(deadline) {
		f := q.next(time.Until(deadline))
		if f == nil {
			break
		}
		if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
			return r
		}
	}
	q.t.Fatalf("%s: no result for %s", q.name, cmd.RequestId)
	return nil
}

func (q *qaWAConn) must(cmd *vttv1.ClientCommand) {
	q.t.Helper()
	if r := q.do(cmd); !r.GetOk() {
		q.t.Fatalf("%s: command refused: %s", q.name, r.GetError())
	}
}

func (q *qaWAConn) sawAnySet() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, f := range q.all {
		if f.GetWatchableChanged() != nil {
			return true
		}
	}
	return false
}

func qaWADescribe(w *vttv1.WatchableChanged) string {
	var b strings.Builder
	for _, a := range w.GetActors() {
		b.WriteString(a.GetActorId() + "(" + a.GetName() + ")[" + strings.Join(a.GetAbilityIds(), ",") + "] ")
	}
	if w.GetControlsNoPartyMember() {
		b.WriteString("controlsNone")
	}
	return b.String()
}

func qaWAAdd(id, name string, kind vttv1.ActorKind, ids ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: name, Kind: kind, AbilityIds: ids},
	}}}
}

func qaWAGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: actor, ParticipantId: participant, Kind: kind,
	}}}
}

func qaWAGive(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GiveAbility{GiveAbility: &vttv1.GiveAbility{
		ActorId: actor, AbilityId: ability,
	}}}
}

func qaWATake(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_TakeAbility{TakeAbility: &vttv1.TakeAbility{
		ActorId: actor, AbilityId: ability,
	}}}
}

func qaWASet(none bool, actors ...*vttv1.WatchableActor) *vttv1.WatchableChanged {
	return &vttv1.WatchableChanged{Actors: actors, ControlsNoPartyMember: none}
}

func qaWAActor(id, name string, ids ...string) *vttv1.WatchableActor {
	return &vttv1.WatchableActor{ActorId: id, Name: name, AbilityIds: ids}
}

func qaWAExpectSet(t *testing.T, who string, got, want *vttv1.WatchableChanged) {
	t.Helper()
	if !proto.Equal(got, want) {
		t.Fatalf("%s: set\n got %s\nwant %s", who, qaWADescribe(got), qaWADescribe(want))
	}
}

func qaWAIsGiven(actor, ability string) func(*vttv1.Envelope) bool {
	return func(e *vttv1.Envelope) bool {
		g := e.GetAbilityGiven()
		return g != nil && g.GetActorId() == actor && g.GetAbilityId() == ability
	}
}

func qaWAIsTaken(actor, ability string) func(*vttv1.Envelope) bool {
	return func(e *vttv1.Envelope) bool {
		g := e.GetAbilityTaken()
		return g != nil && g.GetActorId() == actor && g.GetAbilityId() == ability
	}
}

// qaWATable is a live gateway with a DM, two players, a player who controls
// nothing and a spectator. Party members: a1 and a2 are P's (a2 shared with
// Q), a3 is Q's alone, a4 is nobody's; c1 is a creature the DM controls.
type qaWATable struct {
	base                   string
	ids                    *identity.DB
	dm                     *qaWAConn
	pTok, qTok, rTok, sTok string
	pID, qID, rID, sID     string
}

func qaWANewTable(t *testing.T) *qaWATable {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(filepath.Join(dir, "camp"))
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(filepath.Join(dir, "ids.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gateway.New(camp, ids).Handler())
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
		_ = ids.Close()
		_ = camp.Close()
	})
	tb := &qaWATable{base: srv.URL, ids: ids}
	dmTok, dmID, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	if tb.pTok, tb.pID, err = ids.CreateInvite("Pia", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	if tb.qTok, tb.qID, err = ids.CreateInvite("Quin", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	if tb.rTok, tb.rID, err = ids.CreateInvite("Rolf", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	if tb.sTok, tb.sID, err = ids.CreateInvite("Sami", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	tb.dm = qaWADial(t, tb.base, "dm", dmTok, "")
	pm, np := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	// Sheet orders that are not sorted, so a sorted copy is told apart.
	tb.dm.must(qaWAAdd("a1", "Alda", pm, "slash", "dodge"))
	tb.dm.must(qaWAAdd("a2", "Bram", pm, "ward", "heal"))
	tb.dm.must(qaWAAdd("a3", "Cora", pm, "zap"))
	tb.dm.must(qaWAAdd("a4", "Dun", pm, "kick"))
	tb.dm.must(qaWAAdd("c1", "Grub", np, "bite"))
	tb.dm.must(qaWAGrant("a1", tb.pID, pm))
	tb.dm.must(qaWAGrant("a2", tb.pID, pm))
	tb.dm.must(qaWAGrant("a2", tb.qID, pm))
	tb.dm.must(qaWAGrant("a3", tb.qID, pm))
	tb.dm.must(qaWAGrant("c1", dmID, np))
	return tb
}

// qaWAOpen dials and drains the opening set and ViewpointChanged.
func qaWAOpen(t *testing.T, tb *qaWATable, name, tok, log string) (*qaWAConn, *vttv1.WatchableChanged) {
	t.Helper()
	c := qaWADial(t, tb.base, name, tok, log)
	set, _ := c.nextSet()
	for {
		f := c.next(qaWAWait)
		if f == nil {
			t.Fatalf("%s: no opening ViewpointChanged", name)
		}
		if f.GetViewpointChanged() != nil {
			break
		}
	}
	return c, set
}

// VTT-731: each party member the participant controls carries its sheet's ids
// in the sheet's order; a creature is in no set; a spectator's and a player's
// set carry none while they control no party member.
func TestQAWatchableAbilitiesEachRoleOpensWithTheIdsItIsOwed(t *testing.T) {
	tb := qaWANewTable(t)

	_, pSet := qaWAOpen(t, tb, "p", tb.pTok, "")
	qaWAExpectSet(t, "player P", pSet, qaWASet(false,
		qaWAActor("a1", "Alda", "slash", "dodge"),
		qaWAActor("a2", "Bram", "ward", "heal")))

	_, qSet := qaWAOpen(t, tb, "q", tb.qTok, "")
	qaWAExpectSet(t, "player Q", qSet, qaWASet(false,
		qaWAActor("a2", "Bram", "ward", "heal"),
		qaWAActor("a3", "Cora", "zap")))

	everyoneBare := qaWASet(true,
		qaWAActor("a1", "Alda"), qaWAActor("a2", "Bram"),
		qaWAActor("a3", "Cora"), qaWAActor("a4", "Dun"))
	_, rSet := qaWAOpen(t, tb, "r", tb.rTok, "")
	qaWAExpectSet(t, "player R who controls nothing", rSet, everyoneBare)
	_, sSet := qaWAOpen(t, tb, "s", tb.sTok, "")
	qaWAExpectSet(t, "spectator S who controls none", sSet, everyoneBare)

	if tb.dm.sawAnySet() {
		t.Fatal("the DM's connection was sent a WatchableChanged")
	}
}

// VTT-731: a give and a take on a controlled party member re-send the set to
// each of that player's connections, whichever character each watches, before
// the entry on the connection that watches it; a redial opens with the new ids.
func TestQAWatchableAbilitiesAGiveAndATakeReachEveryConnectionBeforeTheEntry(t *testing.T) {
	tb := qaWANewTable(t)
	p1, _ := qaWAOpen(t, tb, "p1", tb.pTok, "a1")
	p2, _ := qaWAOpen(t, tb, "p2", tb.pTok, "a2")

	tb.dm.must(qaWAGive("a1", "parry"))
	afterGive := qaWASet(false,
		qaWAActor("a1", "Alda", "slash", "dodge", "parry"),
		qaWAActor("a2", "Bram", "ward", "heal"))
	set, before := p1.nextSet()
	qaWAExpectSet(t, "p1 after the give", set, afterGive)
	for _, f := range before {
		if e := f.GetEvent(); e != nil && qaWAIsGiven("a1", "parry")(e) {
			t.Fatal("p1 (watching a1) was sent the AbilityGiven before the set")
		}
	}
	p1.untilEvent(qaWAIsGiven("a1", "parry"), "AbilityGiven of a1 after the set")
	set, _ = p2.nextSet()
	qaWAExpectSet(t, "p2 (watching a2) after the give", set, afterGive)

	tb.dm.must(qaWATake("a1", "slash"))
	afterTake := qaWASet(false,
		qaWAActor("a1", "Alda", "dodge", "parry"),
		qaWAActor("a2", "Bram", "ward", "heal"))
	set, before = p1.nextSet()
	qaWAExpectSet(t, "p1 after the take", set, afterTake)
	for _, f := range before {
		if e := f.GetEvent(); e != nil && qaWAIsTaken("a1", "slash")(e) {
			t.Fatal("p1 (watching a1) was sent the AbilityTaken before the set")
		}
	}
	p1.untilEvent(qaWAIsTaken("a1", "slash"), "AbilityTaken of a1 after the set")
	set, _ = p2.nextSet()
	qaWAExpectSet(t, "p2 (watching a2) after the take", set, afterTake)

	_, p3Set := qaWAOpen(t, tb, "p3", tb.pTok, "")
	qaWAExpectSet(t, "a redial after the give and take", p3Set, afterTake)
}

// VTT-731: a give to a party member another player alone controls sends this
// player none; a give to a shared party member reaches both; a give to a
// party member nobody controls, or to a creature, re-sends nobody's set.
func TestQAWatchableAbilitiesAGiveReachesOnlyThePlayersWhoControlThePartyMember(t *testing.T) {
	tb := qaWANewTable(t)
	p, _ := qaWAOpen(t, tb, "p", tb.pTok, "a1")
	q, _ := qaWAOpen(t, tb, "q", tb.qTok, "a3")
	r, _ := qaWAOpen(t, tb, "r", tb.rTok, "")
	s, _ := qaWAOpen(t, tb, "s", tb.sTok, "")

	tb.dm.must(qaWAGive("a3", "zip"))
	set, _ := q.nextSet()
	qaWAExpectSet(t, "Q after a give to a3", set, qaWASet(false,
		qaWAActor("a2", "Bram", "ward", "heal"),
		qaWAActor("a3", "Cora", "zap", "zip")))
	p.noSetWithin(500*time.Millisecond, "a give to a3, which only Q controls")

	tb.dm.must(qaWAGive("a4", "jab"))
	tb.dm.must(qaWAGive("c1", "claw"))
	p.noSetWithin(500*time.Millisecond, "a give to a4 nobody controls and to c1, a creature")
	q.noSetWithin(100*time.Millisecond, "a give to a4 nobody controls and to c1, a creature")

	// The barrier: a give to the shared a2 is the next set both players get.
	tb.dm.must(qaWAGive("a2", "mend"))
	set, _ = p.nextSet()
	qaWAExpectSet(t, "P after a give to the shared a2 (its first set since the give to a3)", set,
		qaWASet(false,
			qaWAActor("a1", "Alda", "slash", "dodge"),
			qaWAActor("a2", "Bram", "ward", "heal", "mend")))
	set, _ = q.nextSet()
	qaWAExpectSet(t, "Q after a give to the shared a2", set, qaWASet(false,
		qaWAActor("a2", "Bram", "ward", "heal", "mend"),
		qaWAActor("a3", "Cora", "zap", "zip")))

	// The barrier for the bare sets: a new party member is their next set.
	tb.dm.must(qaWAAdd("a5", "Eir", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "glow"))
	everyone := qaWASet(true,
		qaWAActor("a1", "Alda"), qaWAActor("a2", "Bram"), qaWAActor("a3", "Cora"),
		qaWAActor("a4", "Dun"), qaWAActor("a5", "Eir"))
	set, _ = r.nextSet()
	qaWAExpectSet(t, "R's first set after every give", set, everyone)
	set, _ = s.nextSet()
	qaWAExpectSet(t, "S's first set after every give", set, everyone)
	if tb.dm.sawAnySet() {
		t.Fatal("the DM's connection was sent a WatchableChanged")
	}
}

// VTT-731: a party member granted to a player enters their set with its ids.
func TestQAWatchableAbilitiesAGrantedPartyMemberEntersTheSetWithItsIds(t *testing.T) {
	tb := qaWANewTable(t)
	p, _ := qaWAOpen(t, tb, "p", tb.pTok, "")
	tb.dm.must(qaWAGrant("a4", tb.pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	set, _ := p.nextSet()
	qaWAExpectSet(t, "P after a grant of a4", set, qaWASet(false,
		qaWAActor("a1", "Alda", "slash", "dodge"),
		qaWAActor("a2", "Bram", "ward", "heal"),
		qaWAActor("a4", "Dun", "kick")))

	r, _ := qaWAOpen(t, tb, "r", tb.rTok, "")
	tb.dm.must(qaWAGrant("a3", tb.rID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	set, _ = r.nextSet()
	qaWAExpectSet(t, "R after its first party member", set, qaWASet(false,
		qaWAActor("a3", "Cora", "zap")))
}

// VTT-735: both tools say each player who controls the actor while it is a
// party member is told.
func TestQAWatchableAbilitiesTheGiveAndTakeToolsSayTheControllingPlayerIsTold(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &tools); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"give_ability", "take_ability"} {
		i := slices.IndexFunc(tools, func(x struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}) bool {
			return x.Name == name
		})
		if i < 0 {
			t.Fatalf("no tool %s", name)
		}
		d := strings.ToLower(tools[i].Description)
		for _, w := range []string{"player", "control", "party member", "told"} {
			if !strings.Contains(d, w) {
				t.Errorf("%s's description lacks %q: %s", name, w, tools[i].Description)
			}
		}
	}
}
