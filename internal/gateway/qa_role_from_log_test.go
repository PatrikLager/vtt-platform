package gateway_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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
)

const qaRLWait = 5 * time.Second

const qaRLPlayerRefusal = "A participant becomes a player when given a character."

type qaRLWho struct {
	token, id, name string
	role            identity.Role
}

type qaRLTable struct {
	t     *testing.T
	dir   string
	camp  *campaign.Campaign
	ids   *identity.DB
	srv   *httptest.Server
	conns []*qaRLConn
}

type qaRLConn struct {
	t       *testing.T
	who     qaRLWho
	ws      *websocket.Conn
	mu      sync.Mutex
	frames  []*vttv1.ServerFrame
	grew    chan struct{}
	ended   bool
	readErr error
	reqN    int
}

func qaRLOpen(t *testing.T) *qaRLTable {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = camp.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	srv := httptest.NewServer(gateway.New(camp, ids).Handler())
	tb := &qaRLTable{t: t, dir: dir, camp: camp, ids: ids, srv: srv}
	t.Cleanup(func() {
		for _, c := range tb.conns {
			_ = c.ws.CloseNow()
		}
		srv.Close()
		_ = ids.Close()
		_ = camp.Close()
	})
	return tb
}

func (tb *qaRLTable) mint(name string, role identity.Role) qaRLWho {
	tb.t.Helper()
	tok, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("CreateInvite(%q, %q): %v", name, role, err)
	}
	return qaRLWho{token: tok, id: id, name: name, role: role}
}

func (tb *qaRLTable) dial(who qaRLWho) *qaRLConn {
	tb.t.Helper()
	marks := make([]int, len(tb.conns))
	for i, o := range tb.conns {
		marks[i] = o.mark()
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaRLWait)
	defer cancel()
	url := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" + who.token
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		tb.t.Fatalf("dial %s: %v", who.name, err)
	}
	ws.SetReadLimit(1 << 24)
	c := &qaRLConn{t: tb.t, who: who, ws: ws, grew: make(chan struct{})}
	go c.read()
	c.waitFrom(0, "presence snapshot", func(f *vttv1.ServerFrame) bool { return f.GetPresenceSnapshot() != nil })
	if who.role != identity.RoleDM && who.role != identity.RoleAgent {
		c.waitFrom(0, "opening viewpoint", func(f *vttv1.ServerFrame) bool { return f.GetViewpointChanged() != nil })
	}
	for i, o := range tb.conns {
		o.waitFrom(marks[i], who.name+"'s arrival", qaRLPresenceOf(who.id))
	}
	tb.conns = append(tb.conns, c)
	return c
}

func (c *qaRLConn) read() {
	for {
		_, data, err := c.ws.Read(context.Background())
		c.mu.Lock()
		if err != nil {
			c.readErr = err
			c.ended = true
			close(c.grew)
			c.mu.Unlock()
			return
		}
		f := &vttv1.ServerFrame{}
		if uerr := protojson.Unmarshal(data, f); uerr != nil {
			c.readErr = fmt.Errorf("undecodable frame %q: %w", data, uerr)
			c.ended = true
			close(c.grew)
			c.mu.Unlock()
			return
		}
		c.frames = append(c.frames, f)
		close(c.grew)
		c.grew = make(chan struct{})
		c.mu.Unlock()
	}
}

func (c *qaRLConn) mark() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames)
}

func (c *qaRLConn) between(from, to int) []*vttv1.ServerFrame {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.frames[from:to])
}

func (c *qaRLConn) waitFrom(from int, what string, pred func(*vttv1.ServerFrame) bool) (int, *vttv1.ServerFrame) {
	c.t.Helper()
	deadline := time.NewTimer(qaRLWait)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		for i := from; i < len(c.frames); i++ {
			if pred(c.frames[i]) {
				f := c.frames[i]
				c.mu.Unlock()
				return i, f
			}
		}
		from = len(c.frames)
		grew, ended, rerr := c.grew, c.ended, c.readErr
		c.mu.Unlock()
		if ended {
			c.t.Fatalf("%s: connection ended before %s: %v", c.who.name, what, rerr)
		}
		select {
		case <-grew:
		case <-deadline.C:
			c.t.Fatalf("%s: timed out waiting for %s", c.who.name, what)
		}
	}
}

func (c *qaRLConn) do(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	c.reqN++
	rid := fmt.Sprintf("qarl-%s-%d", c.who.name, c.reqN)
	cmd.RequestId = rid
	data, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaRLWait)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
		c.t.Fatalf("%s: write: %v", c.who.name, err)
	}
	_, f := c.waitFrom(0, "result "+rid, func(f *vttv1.ServerFrame) bool { return f.GetResult().GetRequestId() == rid })
	return f.GetResult()
}

func (c *qaRLConn) ok(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	res := c.do(cmd)
	if !res.GetOk() {
		c.t.Fatalf("%s: %v refused: %q", c.who.name, cmd, res.GetError())
	}
	return res
}

func (c *qaRLConn) refused(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	res := c.do(cmd)
	if res.GetOk() {
		c.t.Fatalf("%s: %v accepted (sequence %d), want refused", c.who.name, cmd, res.GetSequence())
	}
	return res
}

func (c *qaRLConn) event(from int, what string, pred func(*vttv1.Envelope) bool) (int, *vttv1.Envelope) {
	c.t.Helper()
	i, f := c.waitFrom(from, what, func(f *vttv1.ServerFrame) bool {
		return f.GetEvent() != nil && pred(f.GetEvent())
	})
	return i, f.GetEvent()
}

func (c *qaRLConn) viewpoints() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, f := range c.frames {
		if v := f.GetViewpointChanged(); v != nil {
			out = append(out, v.GetActorId())
		}
	}
	return out
}

func qaRLPresenceOf(pid string) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		pc := f.GetPresenceChanged()
		return pc != nil && pc.GetParticipantId() == pid && pc.GetState() == vttv1.PresenceState_PRESENCE_STATE_CONNECTED
	}
}

func qaRLWatchable(ids []string, none bool) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		w := f.GetWatchableChanged()
		if w == nil || w.GetControlsNoPartyMember() != none || len(w.GetActors()) != len(ids) {
			return false
		}
		for i, a := range w.GetActors() {
			if a.GetActorId() != ids[i] {
				return false
			}
		}
		return true
	}
}

func qaRLViewpoint(actorID string) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		v := f.GetViewpointChanged()
		return v != nil && v.GetActorId() == actorID
	}
}

func (tb *qaRLTable) get(who qaRLWho, path string, into any) int {
	tb.t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tb.srv.URL+path, nil)
	if err != nil {
		tb.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+who.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tb.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			tb.t.Fatalf("GET %s: decode: %v", path, err)
		}
	}
	return resp.StatusCode
}

type qaRLRow struct {
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

func (tb *qaRLTable) me(who qaRLWho) string {
	tb.t.Helper()
	var row qaRLRow
	if code := tb.get(who, "/api/me", &row); code != http.StatusOK {
		tb.t.Fatalf("GET /api/me as %s: status %d", who.name, code)
	}
	if row.ParticipantID != who.id {
		tb.t.Fatalf("GET /api/me as %s answered participant %q", who.name, row.ParticipantID)
	}
	return row.Role
}

func (tb *qaRLTable) roster(asker qaRLWho) map[string]string {
	tb.t.Helper()
	var rows []qaRLRow
	if code := tb.get(asker, "/api/participants", &rows); code != http.StatusOK {
		tb.t.Fatalf("GET /api/participants as %s: status %d", asker.name, code)
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.ParticipantID] = r.Role
	}
	return out
}

func (tb *qaRLTable) head() int64 {
	tb.t.Helper()
	_, unsub, head, err := tb.camp.Subscribe(0, 4096)
	if err != nil {
		tb.t.Fatalf("Subscribe: %v", err)
	}
	unsub()
	return head
}

func qaRLAddActor(id, name string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: name, Kind: kind},
	}}}
}

func qaRLGrant(actorID, pid string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: actorID, ParticipantId: pid, Kind: kind,
	}}}
}

func qaRLRevoke(actorID, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{RevokeActorControl: &vttv1.RevokeActorControl{
		ActorId: actorID, ParticipantId: pid,
	}}}
}

func qaRLRemoveActor(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveActor{RemoveActor: &vttv1.RemoveActor{ActorId: actorID}}}
}

func qaRLPromote(pid, role string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{PromoteParticipant: &vttv1.PromoteParticipant{
		ParticipantId: pid, Role: role,
	}}}
}

func qaRLSay(text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
		Text: text, OutOfCharacter: true,
	}}}
}

func qaRLSayAs(actorID, text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
		Text: text, ActorId: actorID,
	}}}
}

func qaRLWatch(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID}}}
}

const (
	qaRLParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaRLNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qaRLFix struct {
	tb                   *qaRLTable
	dm, agent, moss, ash qaRLWho
	dmc, agc             *qaRLConn
}

func qaRLStandard(t *testing.T) *qaRLFix {
	t.Helper()
	tb := qaRLOpen(t)
	f := &qaRLFix{tb: tb}
	f.dm = tb.mint("Keeper", identity.RoleDM)
	f.agent = tb.mint("Oracle", identity.RoleAgent)
	f.moss = tb.mint("Moss", identity.RoleSpectator)
	f.ash = tb.mint("Ash", identity.RoleSpectator)
	f.dmc = tb.dial(f.dm)
	f.agc = tb.dial(f.agent)
	for _, a := range []struct {
		id, name string
		kind     vttv1.ActorKind
	}{
		{"pm-d", "Bob", qaRLParty},
		{"pm-b", "Yan", qaRLParty},
		{"np-x", "Crow", qaRLNonParty},
		{"pm-a", "Zed", qaRLParty},
		{"pm-c", "Abe", qaRLParty},
	} {
		f.dmc.ok(qaRLAddActor(a.id, a.name, a.kind))
	}
	return f
}

func (f *qaRLFix) expectRole(step string, who qaRLWho, want string) {
	t := f.tb.t
	t.Helper()
	if got := f.tb.me(who); got != want {
		t.Errorf("%s: GET /api/me as %s = %q, want %q", step, who.name, got, want)
	}
	for _, asker := range []qaRLWho{f.dm, f.agent} {
		if got := f.tb.roster(asker)[who.id]; got != want {
			t.Errorf("%s: GET /api/participants as %s lists %s as %q, want %q", step, asker.name, who.name, got, want)
		}
	}
}

func (f *qaRLFix) expectSays(c *qaRLConn, step string) {
	t := f.tb.t
	t.Helper()
	text := "said " + step
	m := f.dmc.mark()
	res := c.do(qaRLSay(text))
	if !res.GetOk() {
		t.Errorf("%s: %s's out-of-character line refused: %q, want it judged as a player's", step, c.who.name, res.GetError())
	} else {
		_, ev := f.dmc.event(m, "the line "+step, func(e *vttv1.Envelope) bool { return e.GetNarrationAdded().GetText() == text })
		if ev.GetActorRole() != "player" {
			t.Errorf("%s: %s's line carries actor_role %q, want \"player\"", step, c.who.name, ev.GetActorRole())
		}
		if ev.GetParticipantId() != c.who.id {
			t.Errorf("%s: %s's line carries participant_id %q, want %q", step, c.who.name, ev.GetParticipantId(), c.who.id)
		}
		if ev.GetSequence() != res.GetSequence() {
			t.Errorf("%s: result sequence %d, event sequence %d", step, res.GetSequence(), ev.GetSequence())
		}
	}
	before := f.tb.head()
	if res := c.do(qaRLWatch("pm-a")); res.GetOk() {
		t.Errorf("%s: %s's set_viewpoint on a party member they do not control accepted, want a player's refusal", step, c.who.name)
	}
	if res := c.do(qaRLWatch("")); res.GetOk() {
		t.Errorf("%s: %s's set_viewpoint on nobody accepted, want a player's refusal", step, c.who.name)
	}
	if after := f.tb.head(); after != before {
		t.Errorf("%s: refused set_viewpoint moved the log head %d -> %d", step, before, after)
	}
}

func (f *qaRLFix) expectSilenced(c *qaRLConn, step string) {
	t := f.tb.t
	t.Helper()
	before := f.tb.head()
	if res := c.do(qaRLSay("unheard " + step)); res.GetOk() {
		t.Errorf("%s: %s's out-of-character line accepted (sequence %d), want it judged as a spectator's", step, c.who.name, res.GetSequence())
	}
	if after := f.tb.head(); after != before {
		t.Errorf("%s: a refused line moved the log head %d -> %d", step, before, after)
	}
	m := c.mark()
	vps := c.viewpoints()
	res := c.do(qaRLWatch("pm-a"))
	if !res.GetOk() {
		t.Errorf("%s: %s's set_viewpoint on pm-a refused: %q, want a spectator's acceptance", step, c.who.name, res.GetError())
		return
	}
	if len(vps) == 0 || vps[len(vps)-1] != "pm-a" {
		c.waitFrom(m, step+": the perch on pm-a", qaRLViewpoint("pm-a"))
	}
}

func qaRLIsRevocation(actorID, pid string) func(*vttv1.Envelope) bool {
	return func(e *vttv1.Envelope) bool {
		r := e.GetActorControlRevoked()
		return r != nil && r.GetActorId() == actorID && r.GetParticipantId() == pid
	}
}

func (f *qaRLFix) expectControllers(step string, want map[string][]string) {
	t := f.tb.t
	t.Helper()
	actors := f.tb.camp.Actors()
	for id, ctl := range want {
		a, held := actors[id]
		if !held {
			t.Errorf("%s: actor %s is not held", step, id)
			continue
		}
		got := slices.Clone(a.GetControllerIds())
		slices.Sort(got)
		w := slices.Clone(ctl)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Errorf("%s: %s's controllers = %v, want %v", step, id, got, w)
		}
	}
}

// VTT-889 VTT-890
func TestQARLTheRoleAnsweredFollowsControlThroughGrantsAndRevocations(t *testing.T) {
	f := qaRLStandard(t)
	f.expectRole("before any grant", f.moss, "spectator")
	f.expectRole("before any grant", f.ash, "spectator")
	f.expectRole("before any grant", f.dm, "dm")
	f.expectRole("before any grant", f.agent, "agent")

	f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
	f.expectRole("after Ash's grant", f.moss, "spectator")
	f.expectRole("after Ash's grant", f.ash, "player")

	f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
	f.expectRole("after the first grant", f.moss, "player")
	f.agc.ok(qaRLGrant("pm-b", f.moss.id, qaRLParty))
	f.expectRole("after the second grant", f.moss, "player")

	f.dmc.ok(qaRLRevoke("pm-c", f.moss.id))
	f.expectRole("after the first revocation", f.moss, "player")
	f.dmc.ok(qaRLRevoke("pm-b", f.moss.id))
	f.expectRole("after the last revocation", f.moss, "spectator")
	f.expectRole("after the last revocation", f.ash, "player")
	f.expectRole("after the last revocation", f.dm, "dm")
	f.expectRole("after the last revocation", f.agent, "agent")
}

// VTT-889 VTT-890 VTT-891 VTT-892 VTT-893 VTT-139
func TestQARLEveryWayOfEndingControlMakesASpectatorAtTheLastActor(t *testing.T) {
	ways := []struct {
		name string
		end  func(f *qaRLFix, mc *qaRLConn, actorID string)
		all  bool
	}{
		{"the DM's revoke", func(f *qaRLFix, _ *qaRLConn, a string) { f.dmc.ok(qaRLRevoke(a, f.moss.id)) }, false},
		{"the agent's revoke", func(f *qaRLFix, _ *qaRLConn, a string) { f.agc.ok(qaRLRevoke(a, f.moss.id)) }, false},
		{"their own revoke", func(f *qaRLFix, mc *qaRLConn, a string) {
			m := f.dmc.mark()
			mc.ok(qaRLRevoke(a, f.moss.id))
			_, ev := f.dmc.event(m, "Moss's own revocation of "+a, qaRLIsRevocation(a, f.moss.id))
			if ev.GetActorRole() != "player" || ev.GetParticipantId() != f.moss.id {
				f.tb.t.Errorf("Moss's own revocation of %s stamped (%q, %q), want (%q, \"player\")", a, ev.GetParticipantId(), ev.GetActorRole(), f.moss.id)
			}
		}, false},
		{"remove_actor", func(f *qaRLFix, _ *qaRLConn, a string) { f.dmc.ok(qaRLRemoveActor(a)) }, false},
		{"Make spectator", func(f *qaRLFix, _ *qaRLConn, _ string) { f.dmc.ok(qaRLPromote(f.moss.id, "spectator")) }, true},
	}
	for _, w := range ways {
		t.Run(w.name, func(t *testing.T) {
			f := qaRLStandard(t)
			f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
			f.dmc.ok(qaRLGrant("pm-b", f.ash.id, qaRLParty))
			f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
			f.dmc.ok(qaRLGrant("pm-b", f.moss.id, qaRLParty))
			mc := f.tb.dial(f.moss)
			f.expectRole("while controlling two", f.moss, "player")
			f.expectSays(mc, "while controlling two")
			if !w.all {
				w.end(f, mc, "pm-c")
				f.expectRole("after the first ending", f.moss, "player")
				f.expectSays(mc, "after the first ending")
			}
			w.end(f, mc, "pm-b")
			f.expectRole("after the last ending", f.moss, "spectator")
			f.expectRole("after the last ending", f.ash, "player")
			f.expectSilenced(mc, "after the last ending")
		})
	}
}

// VTT-889 VTT-890 VTT-891 VTT-892
func TestQARLAParticipantStoredAsAPlayerIsASpectatorUntilGivenACharacter(t *testing.T) {
	f := qaRLStandard(t)
	pell := f.tb.mint("Pell", identity.RoleSpectator)
	raw, err := sql.Open("sqlite", campaign.LogPath(f.tb.dir)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`UPDATE participants SET role = 'player' WHERE id = ?`, pell.id); err != nil {
		t.Fatal(err)
	}
	if p, err := f.tb.ids.Lookup(pell.id); err != nil || p.Role != identity.RolePlayer {
		t.Fatalf("fixture: identity holds Pell as %+v (%v), want a stored player", p, err)
	}
	pc := f.tb.dial(pell)
	f.expectRole("before any grant", pell, "spectator")
	f.expectSilenced(pc, "before any grant")

	f.dmc.ok(qaRLGrant("pm-d", pell.id, qaRLParty))
	f.expectRole("after the grant", pell, "player")
	f.expectSays(pc, "after the grant")

	f.dmc.ok(qaRLRevoke("pm-d", pell.id))
	f.expectRole("after the revocation", pell, "spectator")
	f.expectSilenced(pc, "after the revocation")
}

// VTT-891 VTT-892 VTT-139
func TestQARLASpectatorGivenACharacterIssuesAPlayersCommandsUntilTheLastIsGone(t *testing.T) {
	f := qaRLStandard(t)
	f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
	mc := f.tb.dial(f.moss)
	f.expectSilenced(mc, "before the grant")
	before := f.tb.head()
	if res := mc.do(qaRLSayAs("pm-c", "not yet mine")); res.GetOk() {
		t.Errorf("before the grant: an in-character line as pm-c accepted, want a spectator's refusal")
	}
	if res := mc.do(qaRLRevoke("pm-c", f.moss.id)); res.GetOk() {
		t.Errorf("before the grant: a self-revocation accepted, want a spectator's refusal")
	}
	if after := f.tb.head(); after != before {
		t.Errorf("before the grant: refused commands moved the log head %d -> %d", before, after)
	}

	f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
	f.expectSays(mc, "after the grant")
	m := f.dmc.mark()
	res := mc.ok(qaRLSayAs("pm-c", "in my own voice"))
	_, ev := f.dmc.event(m, "the in-character line", func(e *vttv1.Envelope) bool {
		return e.GetNarrationAdded().GetText() == "in my own voice"
	})
	if ev.GetActorRole() != "player" || ev.GetSequence() != res.GetSequence() {
		t.Errorf("the in-character line: actor_role %q sequence %d (result %d), want \"player\"", ev.GetActorRole(), ev.GetSequence(), res.GetSequence())
	}

	m = f.dmc.mark()
	res = mc.ok(qaRLRevoke("pm-c", f.moss.id))
	_, ev = f.dmc.event(m, "the self-revocation of the last actor", qaRLIsRevocation("pm-c", f.moss.id))
	if ev.GetActorRole() != "player" || ev.GetParticipantId() != f.moss.id || ev.GetSequence() != res.GetSequence() {
		t.Errorf("the self-revocation of the last actor: stamped (%q, %q) at %d (result %d), want (%q, \"player\")",
			ev.GetParticipantId(), ev.GetActorRole(), ev.GetSequence(), res.GetSequence(), f.moss.id)
	}
	f.expectRole("after their own last revocation", f.moss, "spectator")
	f.expectSilenced(mc, "after their own last revocation")
	before = f.tb.head()
	if res := mc.do(qaRLRevoke("pm-c", f.moss.id)); res.GetOk() {
		t.Errorf("after the last revocation: a second self-revocation accepted, want a spectator's refusal")
	}
	if after := f.tb.head(); after != before {
		t.Errorf("after the last revocation: a refused self-revocation moved the log head %d -> %d", before, after)
	}
}

// VTT-893 VTT-921 VTT-923
func TestQARLWhatAConnectionWatchesFollowsTheControlItsParticipantHolds(t *testing.T) {
	f := qaRLStandard(t)
	f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
	mc := f.tb.dial(f.moss)
	all := []string{"pm-a", "pm-b", "pm-c", "pm-d"}
	mc.waitFrom(0, "the spectator's set at the opening", qaRLWatchable(all, true))
	m := mc.mark()
	mc.ok(qaRLWatch("pm-a"))
	mc.waitFrom(m, "the perch on Ash's pm-a", qaRLViewpoint("pm-a"))

	m = mc.mark()
	f.dmc.ok(qaRLGrant("pm-d", f.moss.id, qaRLParty))
	mc.waitFrom(m, "the set after pm-d's grant", qaRLWatchable([]string{"pm-d"}, false))
	mc.waitFrom(m, "the move onto Moss's own pm-d", qaRLViewpoint("pm-d"))
	if res := mc.do(qaRLWatch("pm-a")); res.GetOk() {
		t.Errorf("a player's set_viewpoint on Ash's pm-a accepted")
	}
	if res := mc.do(qaRLWatch("")); res.GetOk() {
		t.Errorf("a player's set_viewpoint on nobody accepted")
	}

	m = mc.mark()
	f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
	mc.waitFrom(m, "the set after pm-c's grant", qaRLWatchable([]string{"pm-c", "pm-d"}, false))
	m = mc.mark()
	f.agc.ok(qaRLGrant("pm-b", f.moss.id, qaRLParty))
	mc.waitFrom(m, "the set after pm-b's grant", qaRLWatchable([]string{"pm-b", "pm-c", "pm-d"}, false))

	m = mc.mark()
	f.dmc.ok(qaRLRevoke("pm-d", f.moss.id))
	mc.waitFrom(m, "the set after the watched pm-d's revocation", qaRLWatchable([]string{"pm-b", "pm-c"}, false))
	mc.waitFrom(m, "the move to the first party member by id", qaRLViewpoint("pm-b"))

	m = mc.mark()
	f.agc.ok(qaRLRevoke("pm-c", f.moss.id))
	mc.waitFrom(m, "the set after pm-c's revocation", qaRLWatchable([]string{"pm-b"}, false))

	m = mc.mark()
	f.dmc.ok(qaRLRevoke("pm-b", f.moss.id))
	mc.waitFrom(m, "the spectator's set after the last revocation", qaRLWatchable(all, true))

	m = mc.mark()
	mc.ok(qaRLWatch("pm-a"))
	mc.waitFrom(m, "a spectator's perch on Ash's pm-a", qaRLViewpoint("pm-a"))
	m = mc.mark()
	mc.ok(qaRLWatch(""))
	mc.waitFrom(m, "a spectator's perch on nobody", qaRLViewpoint(""))

	want := []string{"", "pm-a", "pm-d", "pm-b", "pm-a", ""}
	if got := mc.viewpoints(); !slices.Equal(got, want) {
		t.Errorf("Moss's viewpoints = %q, want %q", got, want)
	}
}

// VTT-921 VTT-893
func TestQARLAPlayersConnectionOpensOnTheirFirstPartyMemberByID(t *testing.T) {
	f := qaRLStandard(t)
	f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
	f.dmc.ok(qaRLGrant("pm-d", f.moss.id, qaRLParty))
	f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
	f.dmc.ok(qaRLGrant("pm-b", f.moss.id, qaRLParty))
	mc := f.tb.dial(f.moss)
	mc.waitFrom(0, "Moss's own set", qaRLWatchable([]string{"pm-b", "pm-c", "pm-d"}, false))
	if got := mc.viewpoints(); !slices.Equal(got, []string{"pm-b"}) {
		t.Errorf("Moss's opening viewpoints = %q, want [\"pm-b\"]", got)
	}
}

// VTT-896 VTT-897 VTT-898 VTT-906 VTT-889 VTT-890 VTT-923
func TestQARLMakeSpectatorRevokesEachActorTheParticipantControlsAsOneBatch(t *testing.T) {
	for _, by := range []string{"DM", "agent"} {
		t.Run("by the "+by, func(t *testing.T) {
			f := qaRLStandard(t)
			issuer, ic := f.dm, f.dmc
			if by == "agent" {
				issuer, ic = f.agent, f.agc
			}
			f.dmc.ok(qaRLGrant("np-x", f.agent.id, qaRLNonParty))
			f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
			f.dmc.ok(qaRLGrant("pm-b", f.ash.id, qaRLParty))
			f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
			f.dmc.ok(qaRLGrant("pm-b", f.moss.id, qaRLParty))
			ac := f.tb.dial(f.ash)
			mc := f.tb.dial(f.moss)
			mc.waitFrom(0, "Moss's own set", qaRLWatchable([]string{"pm-b", "pm-c"}, false))
			conns := []*qaRLConn{f.dmc, f.agc, ac, mc}
			marks := make([]int, len(conns))
			for i, c := range conns {
				marks[i] = c.mark()
			}
			before := f.tb.head()

			res := ic.ok(qaRLPromote(f.moss.id, "spectator"))

			if res.GetSequence() != before+1 {
				t.Errorf("result sequence = %d, want the batch's first, %d", res.GetSequence(), before+1)
			}
			i1, first := f.dmc.event(marks[0], "the revocation of pm-b", qaRLIsRevocation("pm-b", f.moss.id))
			i2, second := f.dmc.event(marks[0], "the revocation of pm-c", qaRLIsRevocation("pm-c", f.moss.id))
			if first.GetSequence() != before+1 || second.GetSequence() != before+2 {
				t.Errorf("revocations at %d (pm-b) and %d (pm-c), want %d and %d", first.GetSequence(), second.GetSequence(), before+1, before+2)
			}
			for _, ev := range []*vttv1.Envelope{first, second} {
				if ev.GetParticipantId() != issuer.id || ev.GetActorRole() != string(issuer.role) {
					t.Errorf("revocation at %d stamped (%q, %q), want (%q, %q)", ev.GetSequence(), ev.GetParticipantId(), ev.GetActorRole(), issuer.id, issuer.role)
				}
			}
			last := max(i1, i2)
			var seen []int64
			for _, fr := range f.dmc.between(marks[0], last+1) {
				if e := fr.GetEvent(); e != nil {
					seen = append(seen, e.GetSequence())
				}
			}
			if !slices.Equal(seen, []int64{before + 1, before + 2}) {
				t.Errorf("the DM was sent events %v for the promotion, want [%d %d]", seen, before+1, before+2)
			}
			if after := f.tb.head(); after != before+2 {
				t.Errorf("log head after the promotion = %d, want %d", after, before+2)
			}
			f.expectControllers("after Make spectator", map[string][]string{
				"pm-a": {f.ash.id}, "pm-b": {f.ash.id}, "pm-c": {}, "pm-d": {}, "np-x": {f.agent.id},
			})
			f.expectRole("after Make spectator", f.moss, "spectator")
			f.expectRole("after Make spectator", f.ash, "player")
			for i, c := range conns {
				c.waitFrom(marks[i], "the re-announcement of Moss", qaRLPresenceOf(f.moss.id))
			}
			mc.waitFrom(marks[3], "Moss's set as a spectator", qaRLWatchable([]string{"pm-a", "pm-b", "pm-c", "pm-d"}, true))
			m := mc.mark()
			mc.ok(qaRLWatch("pm-a"))
			mc.waitFrom(m, "a spectator's perch on pm-a", qaRLViewpoint("pm-a"))
			if got := mc.viewpoints(); !slices.Equal(got, []string{"pm-b", "pm-a"}) {
				t.Errorf("Moss's viewpoints = %q, want [\"pm-b\" \"pm-a\"]: Make spectator moved the seat off pm-b", got)
			}
		})
	}
}

// VTT-896 VTT-898
func TestQARLMakeSpectatorOfAParticipantWhoControlsNothingAppendsNothing(t *testing.T) {
	for _, by := range []string{"DM", "agent"} {
		t.Run("by the "+by, func(t *testing.T) {
			f := qaRLStandard(t)
			ic := f.dmc
			if by == "agent" {
				ic = f.agc
			}
			pell := f.tb.mint("Pell", identity.RoleSpectator)
			fern := f.tb.mint("Fern", identity.RoleSpectator)
			f.dmc.ok(qaRLGrant("pm-c", fern.id, qaRLParty))
			f.dmc.ok(qaRLRevoke("pm-c", fern.id))
			for _, who := range []qaRLWho{f.moss, pell, fern} {
				before := f.tb.head()
				res := ic.ok(qaRLPromote(who.id, "spectator"))
				if res.GetSequence() != 0 {
					t.Errorf("%s: result sequence = %d, want none", who.name, res.GetSequence())
				}
				if after := f.tb.head(); after != before {
					t.Errorf("%s: log head %d -> %d, want nothing appended", who.name, before, after)
				}
				f.expectRole("after Make spectator", who, "spectator")
			}
		})
	}
}

// VTT-906 VTT-896 VTT-898
func TestQARLMakeSpectatorOfAnAbsentParticipantAnnouncesNobody(t *testing.T) {
	f := qaRLStandard(t)
	f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
	f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
	ac := f.tb.dial(f.ash)
	conns := []*qaRLConn{f.dmc, f.agc, ac}
	marks := make([]int, len(conns))
	for i, c := range conns {
		marks[i] = c.mark()
	}
	before := f.tb.head()
	res := f.dmc.ok(qaRLPromote(f.moss.id, "spectator"))
	if res.GetSequence() != before+1 {
		t.Errorf("result sequence = %d, want %d", res.GetSequence(), before+1)
	}
	f.dmc.event(marks[0], "the revocation of pm-c", qaRLIsRevocation("pm-c", f.moss.id))
	if after := f.tb.head(); after != before+1 {
		t.Errorf("log head %d -> %d, want one revocation", before, after)
	}
	f.tb.dial(f.tb.mint("Fern", identity.RoleSpectator))
	for i, c := range conns {
		for _, fr := range c.between(marks[i], c.mark()) {
			if pc := fr.GetPresenceChanged(); pc != nil && pc.GetParticipantId() == f.moss.id {
				t.Errorf("%s was sent %v for an absent Moss", c.who.name, pc)
			}
		}
	}
}

// VTT-899 VTT-900
func TestQARLPromotionToPlayerIsRefusedInPlainWords(t *testing.T) {
	for _, by := range []string{"DM", "agent"} {
		t.Run("by the "+by, func(t *testing.T) {
			f := qaRLStandard(t)
			ic := f.dmc
			if by == "agent" {
				ic = f.agc
			}
			pell := f.tb.mint("Pell", identity.RoleSpectator)
			f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
			for _, c := range []struct {
				who  qaRLWho
				role string
			}{{f.moss, "player"}, {pell, "spectator"}} {
				before := f.tb.head()
				res := ic.refused(qaRLPromote(c.who.id, "player"))
				if res.GetError() != qaRLPlayerRefusal {
					t.Errorf("%s: refusal = %q, want %q", c.who.name, res.GetError(), qaRLPlayerRefusal)
				}
				if after := f.tb.head(); after != before {
					t.Errorf("%s: log head %d -> %d, want nothing appended", c.who.name, before, after)
				}
				f.expectRole("after a refused promotion to player", c.who, c.role)
			}
			f.expectControllers("after the refusals", map[string][]string{"pm-c": {f.moss.id}})
		})
	}
}

// VTT-899
func TestQARLPromotionToAnyRoleButSpectatorIsRefused(t *testing.T) {
	for _, by := range []string{"DM", "agent"} {
		t.Run("by the "+by, func(t *testing.T) {
			f := qaRLStandard(t)
			ic := f.dmc
			if by == "agent" {
				ic = f.agc
			}
			f.dmc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty))
			for _, role := range []string{"dm", "agent", "", "wizard", "Spectator", "PLAYER"} {
				before := f.tb.head()
				if res := ic.do(qaRLPromote(f.moss.id, role)); res.GetOk() {
					t.Errorf("a promotion to %q accepted (sequence %d)", role, res.GetSequence())
				}
				if after := f.tb.head(); after != before {
					t.Errorf("a promotion to %q moved the log head %d -> %d", role, before, after)
				}
			}
			f.expectRole("after the refusals", f.moss, "player")
			f.expectControllers("after the refusals", map[string][]string{"pm-c": {f.moss.id}})
		})
	}
}

// VTT-026 VTT-899
func TestQARLPromotionCannotUnmakeADMOrAnAgent(t *testing.T) {
	f := qaRLStandard(t)
	f.dmc.ok(qaRLGrant("np-x", f.agent.id, qaRLNonParty))
	f.dmc.ok(qaRLGrant("pm-d", f.dm.id, qaRLParty))
	issuers := []struct {
		name string
		c    *qaRLConn
	}{{"the DM", f.dmc}, {"the agent", f.agc}}
	for _, is := range issuers {
		for _, target := range []qaRLWho{f.dm, f.agent} {
			for _, role := range []string{"spectator", "player"} {
				before := f.tb.head()
				if res := is.c.do(qaRLPromote(target.id, role)); res.GetOk() {
					t.Errorf("%s's promotion of %s to %q accepted (sequence %d)", is.name, target.name, role, res.GetSequence())
				}
				if after := f.tb.head(); after != before {
					t.Errorf("%s's promotion of %s to %q moved the log head %d -> %d", is.name, target.name, role, before, after)
				}
			}
		}
	}
	f.expectRole("after the refusals", f.dm, "dm")
	f.expectRole("after the refusals", f.agent, "agent")
	f.expectControllers("after the refusals", map[string][]string{"np-x": {f.agent.id}, "pm-d": {f.dm.id}})
}

// VTT-906
func TestQARLGrantRevokeAndRemovalReannounceEachParticipantTheyChange(t *testing.T) {
	f := qaRLStandard(t)
	f.dmc.ok(qaRLGrant("pm-a", f.ash.id, qaRLParty))
	f.dmc.ok(qaRLGrant("pm-b", f.ash.id, qaRLParty))
	ac := f.tb.dial(f.ash)
	mc := f.tb.dial(f.moss)
	conns := []*qaRLConn{f.dmc, f.agc, ac, mc}
	step := func(name string, run func(), named ...qaRLWho) {
		t.Helper()
		marks := make([]int, len(conns))
		for i, c := range conns {
			marks[i] = c.mark()
		}
		run()
		for i, c := range conns {
			for _, who := range named {
				c.waitFrom(marks[i], name+": the re-announcement of "+who.name, qaRLPresenceOf(who.id))
			}
		}
	}
	step("the DM's grant", func() { f.dmc.ok(qaRLGrant("pm-b", f.moss.id, qaRLParty)) }, f.moss)
	step("the agent's grant", func() { f.agc.ok(qaRLGrant("pm-c", f.moss.id, qaRLParty)) }, f.moss)
	step("the agent's revoke", func() { f.agc.ok(qaRLRevoke("pm-c", f.moss.id)) }, f.moss)
	step("the removal of the shared pm-b", func() { f.dmc.ok(qaRLRemoveActor("pm-b")) }, f.moss, f.ash)
}
