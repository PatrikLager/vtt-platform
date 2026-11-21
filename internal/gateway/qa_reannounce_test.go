package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaRAWait = 10 * time.Second

type qaRAWho struct {
	id, name, token string
}

type qaRAConn struct {
	who    qaRAWho
	ws     *websocket.Conn
	mu     sync.Mutex
	frames []*vttv1.ServerFrame
	err    error
	wake   chan struct{}
	closed chan struct{}
}

func (c *qaRAConn) read() {
	defer close(c.closed)
	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			c.mu.Lock()
			c.err = err
			c.mu.Unlock()
			return
		}
		f := &vttv1.ServerFrame{}
		if uerr := protojson.Unmarshal(data, f); uerr != nil {
			c.mu.Lock()
			c.err = fmt.Errorf("decode %q: %w", data, uerr)
			c.mu.Unlock()
			return
		}
		c.mu.Lock()
		c.frames = append(c.frames, f)
		c.mu.Unlock()
		select {
		case c.wake <- struct{}{}:
		default:
		}
	}
}

func (c *qaRAConn) copyFrames() []*vttv1.ServerFrame {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*vttv1.ServerFrame(nil), c.frames...)
}

func (c *qaRAConn) waitFrame(t *testing.T, from int, what string,
	match func(*vttv1.ServerFrame) bool) int {
	t.Helper()
	deadline := time.NewTimer(qaRAWait)
	defer deadline.Stop()
	gone := false
	for {
		c.mu.Lock()
		for i := from; i < len(c.frames); i++ {
			if match(c.frames[i]) {
				c.mu.Unlock()
				return i
			}
		}
		from = len(c.frames)
		err := c.err
		c.mu.Unlock()
		if gone {
			t.Fatalf("%s: connection ended (%v) before %s", c.who.name, err, what)
		}
		select {
		case <-c.wake:
		case <-c.closed:
			gone = true
		case <-deadline.C:
			t.Fatalf("%s: %s not seen within %v", c.who.name, what, qaRAWait)
		}
	}
}

func (c *qaRAConn) command(t *testing.T, tbl *qaRATable,
	cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t.Helper()
	tbl.seq++
	cmd.RequestId = fmt.Sprintf("qa-ra-%d", tbl.seq)
	data, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaRAWait)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("%s: write: %v", c.who.name, err)
	}
	id := cmd.RequestId
	i := c.waitFrame(t, 0, "result "+id, func(f *vttv1.ServerFrame) bool {
		return f.GetResult() != nil && f.GetResult().GetRequestId() == id
	})
	return c.copyFrames()[i].GetResult()
}

type qaRATable struct {
	t       *testing.T
	camp    *campaign.Campaign
	ids     *identity.DB
	http    *httptest.Server
	conns   []*qaRAConn
	revoked map[string]bool
	seq     int
}

func qaRANewTable(t *testing.T) *qaRATable {
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
	tbl := &qaRATable{t: t, camp: camp, ids: ids, revoked: map[string]bool{}}
	tbl.http = httptest.NewServer(gateway.New(camp, ids).Handler())
	t.Cleanup(func() {
		for _, c := range tbl.conns {
			_ = c.ws.CloseNow()
			select {
			case <-c.closed:
			case <-time.After(qaRAWait):
			}
		}
		tbl.http.Close()
		_ = ids.Close()
		_ = camp.Close()
	})
	return tbl
}

func (tbl *qaRATable) mint(name string, role identity.Role) qaRAWho {
	tbl.t.Helper()
	token, id, err := tbl.ids.CreateInvite(name, role)
	if err != nil {
		tbl.t.Fatalf("CreateInvite %s: %v", name, err)
	}
	return qaRAWho{id: id, name: name, token: token}
}

func (tbl *qaRATable) appendLog(payload func(*vttv1.Envelope)) {
	tbl.t.Helper()
	tbl.seq++
	env := &vttv1.Envelope{
		EventId:    fmt.Sprintf("qa-ra-fixture-%d", tbl.seq),
		OccurredAt: timestamppb.Now(),
		ActorRole:  "dm",
	}
	payload(env)
	if _, err := tbl.camp.Append(env); err != nil {
		tbl.t.Fatalf("fixture append: %v", err)
	}
}

func (tbl *qaRATable) addActor(id, name string) {
	tbl.appendLog(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: id, Name: name,
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		}}
	})
}

func (tbl *qaRATable) logGrant(actorID, pid string) {
	tbl.appendLog(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actorID,
				ParticipantId: pid, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		}
	})
}

func qaRALive(c *qaRAConn) bool {
	select {
	case <-c.closed:
		return false
	default:
		return true
	}
}

// Keep the wait for the arrival here: one still in flight lands in the next
// window and is counted as the change's frame.
func (tbl *qaRATable) dial(who qaRAWho) *qaRAConn {
	tbl.t.Helper()
	arrival := true
	from := map[*qaRAConn]int{}
	for _, o := range tbl.conns {
		if !qaRALive(o) {
			continue
		}
		if o.who.id == who.id {
			arrival = false
			continue
		}
		if !tbl.revoked[o.who.id] {
			from[o] = len(o.copyFrames())
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaRAWait)
	defer cancel()
	u := "ws" + strings.TrimPrefix(tbl.http.URL, "http") + "/ws?token=" +
		url.QueryEscape(who.token)
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		tbl.t.Fatalf("dial %s: %v", who.name, err)
	}
	ws.SetReadLimit(1 << 22)
	c := &qaRAConn{who: who, ws: ws, wake: make(chan struct{}, 1),
		closed: make(chan struct{})}
	tbl.conns = append(tbl.conns, c)
	go c.read()
	c.waitFrame(tbl.t, 0, "presence snapshot", func(f *vttv1.ServerFrame) bool {
		return f.GetPresenceSnapshot() != nil
	})
	if arrival {
		for o, i := range from {
			o.waitFrame(tbl.t, i, "arrival of "+who.name,
				qaRAIs(who.id, vttv1.PresenceState_PRESENCE_STATE_CONNECTED))
		}
	}
	return c
}

func (tbl *qaRATable) revoke(who qaRAWho) {
	tbl.t.Helper()
	if err := tbl.ids.Revoke(who.id); err != nil {
		tbl.t.Fatalf("Revoke %s: %v", who.name, err)
	}
	tbl.revoked[who.id] = true
}

func (tbl *qaRATable) hangUp(c *qaRAConn) {
	tbl.t.Helper()
	_ = c.ws.Close(websocket.StatusNormalClosure, "")
	select {
	case <-c.closed:
	case <-time.After(qaRAWait):
		tbl.t.Fatalf("%s: hang-up did not finish", c.who.name)
	}
}

func qaRAIs(pid string, state vttv1.PresenceState) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		pc := f.GetPresenceChanged()
		return pc != nil && pc.GetParticipantId() == pid && pc.GetState() == state
	}
}

// Fence by an arrival: presence fan-outs run in one order (SPEC-011).
func (tbl *qaRATable) fence(watchers ...*qaRAConn) map[*qaRAConn]int {
	tbl.t.Helper()
	f := tbl.mint(fmt.Sprintf("Fence %d", tbl.seq+1), identity.RoleSpectator)
	tbl.seq++
	tbl.dial(f)
	at := make(map[*qaRAConn]int, len(watchers))
	for _, w := range watchers {
		at[w] = w.waitFrame(tbl.t, 0, "arrival of "+f.name,
			qaRAIs(f.id, vttv1.PresenceState_PRESENCE_STATE_CONNECTED))
	}
	return at
}

func qaRAWindow(c *qaRAConn, lo, hi int) []*vttv1.PresenceChanged {
	var out []*vttv1.PresenceChanged
	frames := c.copyFrames()
	for i := lo + 1; i < hi && i < len(frames); i++ {
		if pc := frames[i].GetPresenceChanged(); pc != nil {
			out = append(out, pc)
		}
	}
	return out
}

// Count only between two fences, and refuse a presence frame want does not
// name (VTT-906).
func qaRAExpectWindow(t *testing.T, watchers []*qaRAConn, before,
	after map[*qaRAConn]int, want map[qaRAWho]int, ignore ...qaRAWho) {
	t.Helper()
	byID := map[string]qaRAWho{}
	for who := range want {
		byID[who.id] = who
	}
	skip := map[string]bool{}
	for _, who := range ignore {
		skip[who.id] = true
	}
	for _, w := range watchers {
		got := map[string]int{}
		for _, pc := range qaRAWindow(w, before[w], after[w]) {
			if skip[pc.GetParticipantId()] {
				continue
			}
			who, named := byID[pc.GetParticipantId()]
			if !named {
				t.Errorf("%s was sent an unpromised presence frame %v",
					w.who.name, pc)
				continue
			}
			exact := &vttv1.PresenceChanged{ParticipantId: who.id,
				DisplayName: who.name,
				State:       vttv1.PresenceState_PRESENCE_STATE_CONNECTED}
			if !proto.Equal(pc, exact) {
				t.Errorf("%s was sent %v, want %v", w.who.name, pc, exact)
			}
			got[who.id]++
		}
		for who, n := range want {
			if got[who.id] != n {
				t.Errorf("%s was sent %d frames naming %s, want %d",
					w.who.name, got[who.id], who.name, n)
			}
		}
	}
}

type qaRAWorld struct {
	*qaRATable
	dm, agent, corin, pia, pell, tamsin, sky qaRAWho
}

// Keep b-well shared and not first by id: the actor asked about must be
// neither the only one nor the first.
func qaRANewWorld(t *testing.T) *qaRAWorld {
	t.Helper()
	tbl := qaRANewTable(t)
	w := &qaRAWorld{qaRATable: tbl}
	w.dm = tbl.mint("Dana", identity.RoleDM)
	w.agent = tbl.mint("Aldo", identity.RoleAgent)
	w.corin = tbl.mint("Corin", identity.RoleSpectator)
	w.pia = tbl.mint("Pia", identity.RoleSpectator)
	w.pell = tbl.mint("Pell", identity.RoleSpectator)
	w.tamsin = tbl.mint("Tamsin", identity.RoleSpectator)
	w.sky = tbl.mint("Sky", identity.RoleSpectator)
	tbl.addActor("a-lamp", "Lamp")
	tbl.addActor("b-well", "Well")
	tbl.addActor("c-gate", "Gate")
	tbl.addActor("d-mill", "Mill")
	tbl.logGrant("a-lamp", w.corin.id)
	tbl.logGrant("b-well", w.pia.id)
	tbl.logGrant("b-well", w.pell.id)
	tbl.logGrant("d-mill", w.pia.id)
	got := tbl.camp.Actors()["b-well"].GetControllerIds()
	if len(got) != 2 || got[0] != w.pia.id || got[1] != w.pell.id {
		t.Fatalf("fixture: b-well controllers %v", got)
	}
	return w
}

func qaRAGrant(actorID, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{ActorId: actorID,
			ParticipantId: pid, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
	}}
}

func qaRARevoke(actorID, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{ActorId: actorID,
			ParticipantId: pid},
	}}
}

func qaRARemoveActor(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveActor{
		RemoveActor: &vttv1.RemoveActor{ActorId: actorID},
	}}
}

func qaRAPromote(pid, role string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: pid,
			Role: role},
	}}
}

func qaRAAccepted(t *testing.T, res *vttv1.CommandResult) {
	t.Helper()
	if !res.GetOk() {
		t.Fatalf("command refused: %q", res.GetError())
	}
}

func qaRARefused(t *testing.T, res *vttv1.CommandResult) {
	t.Helper()
	if res.GetOk() {
		t.Fatalf("command accepted, want refused")
	}
}

// VTT-906
func TestQARADMGrantIsAnnouncedToEveryRoleAndBothTargetConnections(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	pell := w.dial(w.pell)
	sky := w.dial(w.sky)
	tam1 := w.dial(w.tamsin)
	tam2 := w.dial(w.tamsin)
	watchers := []*qaRAConn{dm, agent, pia, pell, sky, tam1, tam2}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAGrant("b-well", w.tamsin.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.tamsin: 1, w.pia: 0, w.pell: 0, w.corin: 0})
}

// VTT-906
func TestQARAAgentGrantIsAnnouncedToEveryRole(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	corin := w.dial(w.corin)
	sky := w.dial(w.sky)
	tam1 := w.dial(w.tamsin)
	tam2 := w.dial(w.tamsin)
	watchers := []*qaRAConn{dm, agent, corin, sky, tam1, tam2}

	before := w.fence(watchers...)
	qaRAAccepted(t, agent.command(t, w.qaRATable, qaRAGrant("c-gate", w.tamsin.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.tamsin: 1, w.corin: 0})
}

// VTT-906
func TestQARAAgentGrantToItselfIsAnnouncedOnItsOwnConnection(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, pia, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, agent.command(t, w.qaRATable, qaRAGrant("c-gate", w.agent.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.agent: 1})
}

// VTT-906
func TestQARARegrantIsAnnouncedThoughNothingChanged(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	pell := w.dial(w.pell)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, pell, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAGrant("b-well", w.pell.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pell: 1, w.pia: 0})
}

// VTT-906
func TestQARADMRevocationIsAnnouncedToEveryRoleAndBothTargetConnections(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	pell1 := w.dial(w.pell)
	pell2 := w.dial(w.pell)
	sky := w.dial(w.sky)
	tam := w.dial(w.tamsin)
	watchers := []*qaRAConn{dm, agent, pia, pell1, pell2, sky, tam}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRARevoke("b-well", w.pell.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pell: 1, w.pia: 0, w.tamsin: 0})
}

// VTT-906
func TestQARAAgentRevocationIsAnnounced(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia1 := w.dial(w.pia)
	pia2 := w.dial(w.pia)
	pell := w.dial(w.pell)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, pia1, pia2, pell, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, agent.command(t, w.qaRATable, qaRARevoke("b-well", w.pia.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pia: 1, w.pell: 0})
}

// VTT-906
func TestQARAPlayerRevokingThemselvesIsAnnouncedOnBothOwnConnections(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia1 := w.dial(w.pia)
	pia2 := w.dial(w.pia)
	pell := w.dial(w.pell)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, pia1, pia2, pell, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, pia1.command(t, w.qaRATable, qaRARevoke("b-well", w.pia.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pia: 1, w.pell: 0})
}

// VTT-906
func TestQARADMRevocationOfANonControllerIsAnnounced(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	tam := w.dial(w.tamsin)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, tam, sky}

	before := w.fence(watchers...)
	res := dm.command(t, w.qaRATable, qaRARevoke("b-well", w.tamsin.id))
	after := w.fence(watchers...)

	if !res.GetOk() {
		t.Fatalf("VTT-906: the DM's revocation of a participant who does not control the actor was refused: %q", res.GetError())
	}
	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.tamsin: 1, w.pia: 0})
}

// VTT-906
func TestQARARemoveActorNamesEachController(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	corin := w.dial(w.corin)
	pia := w.dial(w.pia)
	pell1 := w.dial(w.pell)
	pell2 := w.dial(w.pell)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, corin, pia, pell1, pell2, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRARemoveActor("b-well")))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pia: 1, w.pell: 1, w.corin: 0})
	if _, still := w.camp.Actors()["b-well"]; still {
		t.Fatalf("b-well still in the world after remove_actor")
	}
}

// VTT-906
func TestQARAAgentRemoveActorNamesItsOneController(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	corin := w.dial(w.corin)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, corin, pia, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, agent.command(t, w.qaRATable, qaRARemoveActor("d-mill")))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pia: 1, w.corin: 0})
}

// VTT-906
func TestQARARemoveActorWithNoControllerNamesNobody(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRARemoveActor("c-gate")))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.pia: 0})
}

// VTT-907
func TestQARARemoveActorNamesOnlyItsConnectedControllers(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, pia, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRARemoveActor("b-well")))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pia: 1, w.pell: 0})
}

// VTT-907
func TestQARAGrantToANeverConnectedParticipantIsAnnouncedToNobody(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, pia, sky}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAGrant("c-gate", w.tamsin.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.tamsin: 0})
	for _, c := range watchers {
		for _, f := range c.copyFrames() {
			if f.GetPresenceChanged().GetParticipantId() == w.tamsin.id {
				t.Errorf("%s was sent %v", c.who.name, f.GetPresenceChanged())
			}
		}
	}
}

// VTT-907
func TestQARAGrantToAHungUpParticipantIsAnnouncedToNobody(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, agent, pia, sky}
	tam := w.dial(w.tamsin)
	for _, c := range watchers {
		c.waitFrame(t, 0, "Tamsin's arrival",
			qaRAIs(w.tamsin.id, vttv1.PresenceState_PRESENCE_STATE_CONNECTED))
	}
	w.hangUp(tam)
	for _, c := range watchers {
		c.waitFrame(t, 0, "Tamsin's departure",
			qaRAIs(w.tamsin.id, vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED))
	}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAGrant("c-gate", w.tamsin.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.tamsin: 0})
}

// VTT-907
func TestQARARevocationOfAHungUpParticipantIsAnnouncedToNobody(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, sky}
	pell := w.dial(w.pell)
	for _, c := range watchers {
		c.waitFrame(t, 0, "Pell's arrival",
			qaRAIs(w.pell.id, vttv1.PresenceState_PRESENCE_STATE_CONNECTED))
	}
	w.hangUp(pell)
	for _, c := range watchers {
		c.waitFrame(t, 0, "Pell's departure",
			qaRAIs(w.pell.id, vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED))
	}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRARevoke("b-well", w.pell.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pell: 0, w.pia: 0})
}

// VTT-906
func TestQARAGrantToAParticipantWithOneOfTwoConnectionsLeft(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	tam1 := w.dial(w.tamsin)
	tam2 := w.dial(w.tamsin)
	w.hangUp(tam1)
	watchers := []*qaRAConn{dm, pia, sky, tam2}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAGrant("c-gate", w.tamsin.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.tamsin: 1})
}

// VTT-032 VTT-906
func TestQARARevokedWatcherIsNotSentTheReannouncement(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	sky := w.dial(w.sky)
	tam := w.dial(w.tamsin)
	corin := w.dial(w.corin)
	watchers := []*qaRAConn{dm, pia, sky, tam}

	before := w.fence(append(watchers, corin)...)
	w.revoke(w.corin)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAGrant("c-gate", w.tamsin.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.tamsin: 1},
		w.corin)
	select {
	case <-corin.closed:
	case <-time.After(qaRAWait):
		t.Fatalf("the revoked watcher's connection did not close")
	}
	frames := corin.copyFrames()
	for i := before[corin] + 1; i < len(frames); i++ {
		if pc := frames[i].GetPresenceChanged(); pc != nil {
			t.Errorf("revoked Corin was sent %v", pc)
		}
	}
	corin.mu.Lock()
	err := corin.err
	corin.mu.Unlock()
	var ce websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != websocket.StatusPolicyViolation {
		t.Errorf("revoked Corin's connection ended with %v, want policy violation",
			err)
	}
}

// VTT-924
func TestQARAPlayerGrantIsRefusedAndAnnouncesNothing(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	tam := w.dial(w.tamsin)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, tam, sky}

	before := w.fence(watchers...)
	qaRARefused(t, pia.command(t, w.qaRATable, qaRAGrant("b-well", w.tamsin.id)))
	qaRARefused(t, pia.command(t, w.qaRATable, qaRAGrant("d-mill", w.pia.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.tamsin: 0, w.pia: 0})
}

// VTT-924
func TestQARAPlayerRevokingSomebodyElseIsRefusedAndAnnouncesNothing(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	pell := w.dial(w.pell)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, pell, sky}

	before := w.fence(watchers...)
	qaRARefused(t, pia.command(t, w.qaRATable, qaRARevoke("b-well", w.pell.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.pell: 0, w.pia: 0})
	if got := w.camp.Actors()["b-well"].GetControllerIds(); len(got) != 2 {
		t.Fatalf("b-well controllers after a refused revocation: %v", got)
	}
}

// VTT-924
func TestQARAOtherRefusalsAnnounceNothing(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	pia := w.dial(w.pia)
	pell := w.dial(w.pell)
	tam := w.dial(w.tamsin)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, pia, pell, tam, sky}

	before := w.fence(watchers...)
	noKind := qaRAGrant("c-gate", w.tamsin.id)
	noKind.GetGrantActorControl().Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	qaRARefused(t, dm.command(t, w.qaRATable, noKind))
	qaRARefused(t, pia.command(t, w.qaRATable, qaRARemoveActor("b-well")))
	qaRARefused(t, tam.command(t, w.qaRATable, qaRARevoke("b-well", w.pia.id)))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{
		w.tamsin: 0, w.pia: 0, w.pell: 0})
}

// VTT-924
func TestQARAGrantOfAnUnknownActorAnnouncesOnlyIfAccepted(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	tam := w.dial(w.tamsin)
	sky := w.dial(w.sky)
	watchers := []*qaRAConn{dm, tam, sky}

	before := w.fence(watchers...)
	res := dm.command(t, w.qaRATable, qaRAGrant("z-none", w.tamsin.id))
	after := w.fence(watchers...)

	if res.GetOk() {
		t.Fatalf("VTT-924: a grant of an actor the log does not hold was accepted")
	}
	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.tamsin: 0})
}

// VTT-906
func TestQARAMakeSpectatorIsAnnouncedOnceToEveryoneAndItsTarget(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	corin1 := w.dial(w.corin)
	corin2 := w.dial(w.corin)
	watchers := []*qaRAConn{dm, agent, pia, corin1, corin2}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAPromote(w.corin.id, "spectator")))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.corin: 1})
}

// VTT-907
func TestQARAMakeSpectatorOfAnUnconnectedParticipantIsAnnouncedToNobody(t *testing.T) {
	w := qaRANewWorld(t)
	dm := w.dial(w.dm)
	agent := w.dial(w.agent)
	pia := w.dial(w.pia)
	watchers := []*qaRAConn{dm, agent, pia}

	before := w.fence(watchers...)
	qaRAAccepted(t, dm.command(t, w.qaRATable, qaRAPromote(w.corin.id, "spectator")))
	after := w.fence(watchers...)

	qaRAExpectWindow(t, watchers, before, after, map[qaRAWho]int{w.corin: 0})
}
