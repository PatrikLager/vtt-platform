package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaGoneReason = "gateway: credential no longer valid"

type qaWho struct{ id, token, name string }

type qaTable struct {
	c                                *campaign.Campaign
	ids                              *identity.DB
	srv                              *httptest.Server
	dm, dm2, agent, p, q, spec, idle qaWho
	seq                              int
}

func qaMint(t *testing.T, ids *identity.DB, name string, r identity.Role) qaWho {
	t.Helper()
	tok, id, err := ids.CreateInvite(name, r)
	if err != nil {
		t.Fatal(err)
	}
	return qaWho{id, tok, name}
}

func (tb *qaTable) env(who qaWho, role string) *vttv1.Envelope {
	tb.seq++
	return &vttv1.Envelope{EventId: fmt.Sprintf("qa-ev-%d", tb.seq), OccurredAt: timestamppb.Now(), ParticipantId: who.id, ActorRole: role}
}

// Keep the seed out of id order with no sorted rotation: a lost sort must show.
func (tb *qaTable) seed(t *testing.T, spec []struct {
	id    string
	ctrls []qaWho
}) {
	t.Helper()
	var envs []*vttv1.Envelope
	for _, a := range spec {
		e := tb.env(tb.dm, "dm")
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: a.id, Name: a.id, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}
		envs = append(envs, e)
		for _, c := range a.ctrls {
			g := tb.env(tb.dm, "dm")
			g.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: a.id, ParticipantId: c.id, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}
			envs = append(envs, g)
		}
	}
	if _, err := tb.c.AppendBatch(envs); err != nil {
		t.Fatal(err)
	}
}

func qaNewTable(t *testing.T) *qaTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() { srv.Close(); ids.Close(); c.Close() })
	tb := &qaTable{c: c, ids: ids, srv: srv}
	tb.dm = qaMint(t, ids, "Dee", identity.RoleDM)
	tb.dm2 = qaMint(t, ids, "Dee Two", identity.RoleDM)
	tb.agent = qaMint(t, ids, "Aggie", identity.RoleAgent)
	tb.p = qaMint(t, ids, "Pia", identity.RolePlayer)
	tb.q = qaMint(t, ids, "Quin", identity.RolePlayer)
	tb.spec = qaMint(t, ids, "Spec", identity.RoleSpectator)
	tb.idle = qaMint(t, ids, "Idle", identity.RolePlayer)
	tb.seed(t, []struct {
		id    string
		ctrls []qaWho
	}{
		{"act-zz", []qaWho{tb.p}},
		{"act-mm", []qaWho{tb.p, tb.q}},
		{"act-aa", []qaWho{tb.q, tb.p}},
		{"act-nn", []qaWho{tb.q}},
	})
	return tb
}

type qaConn struct {
	ws     *websocket.Conn
	req    int
	mu     sync.Mutex
	frames []*vttv1.ServerFrame
	closed chan struct{}
	err    error
}

func (tb *qaTable) dial(t *testing.T, who qaWho) *qaConn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" + who.token
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", who.name, err)
	}
	cn := &qaConn{ws: ws, closed: make(chan struct{})}
	t.Cleanup(func() { ws.CloseNow() })
	go func() {
		for {
			_, b, err := ws.Read(context.Background())
			if err != nil {
				cn.mu.Lock()
				cn.err = err
				cn.mu.Unlock()
				close(cn.closed)
				return
			}
			f := &vttv1.ServerFrame{}
			if protojson.Unmarshal(b, f) == nil {
				cn.mu.Lock()
				cn.frames = append(cn.frames, f)
				cn.mu.Unlock()
			}
		}
	}()
	cn.await(t, func(f *vttv1.ServerFrame) bool { return f.GetCatchUpHead() != nil })
	return cn
}

func (cn *qaConn) snapshot() []*vttv1.ServerFrame {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	return append([]*vttv1.ServerFrame(nil), cn.frames...)
}

func (cn *qaConn) find(pred func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	for _, f := range cn.snapshot() {
		if pred(f) {
			return f
		}
	}
	return nil
}

func (cn *qaConn) await(t *testing.T, pred func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f := cn.find(pred); f != nil {
			return f
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("frame never arrived")
	return nil
}

func (cn *qaConn) head() int64 {
	return cn.find(func(f *vttv1.ServerFrame) bool { return f.GetCatchUpHead() != nil }).GetCatchUpHead().GetHeadSequence()
}

func (cn *qaConn) revocations() []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range cn.snapshot() {
		if e := f.GetEvent(); e != nil && e.GetActorControlRevoked() != nil {
			out = append(out, e)
		}
	}
	return out
}

func (cn *qaConn) eventsAfter(seq int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range cn.snapshot() {
		if e := f.GetEvent(); e != nil && e.GetSequence() > seq {
			out = append(out, e)
		}
	}
	return out
}

func (cn *qaConn) remove(t *testing.T, targetID string) *vttv1.CommandResult {
	t.Helper()
	cn.req++
	rid := fmt.Sprintf("qa-req-%d", cn.req)
	cmd := &vttv1.ClientCommand{RequestId: rid, Command: &vttv1.ClientCommand_RemoveParticipant{RemoveParticipant: &vttv1.RemoveParticipant{ParticipantId: targetID}}}
	b, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cn.ws.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
	f := cn.await(t, func(f *vttv1.ServerFrame) bool { return f.GetResult().GetRequestId() == rid })
	return f.GetResult()
}

func (cn *qaConn) closeInfo(t *testing.T) (websocket.StatusCode, string) {
	t.Helper()
	select {
	case <-cn.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("connection was not closed")
	}
	cn.mu.Lock()
	defer cn.mu.Unlock()
	var ce websocket.CloseError
	if !qaAsClose(cn.err, &ce) {
		return -1, cn.err.Error()
	}
	return ce.Code, ce.Reason
}

func qaAsClose(err error, ce *websocket.CloseError) bool {
	return errors.As(err, ce)
}

func (cn *qaConn) stayedOpen(d time.Duration) bool {
	select {
	case <-cn.closed:
		return false
	case <-time.After(d):
		return true
	}
}

func qaStatus(t *testing.T, tb *qaTable, token string) int {
	t.Helper()
	url := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, resp, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		ws.CloseNow()
		return http.StatusSwitchingProtocols
	}
	if resp == nil {
		t.Fatalf("dial error without response: %v", err)
	}
	return resp.StatusCode
}

func qaControllers(tb *qaTable, actor string) map[string]bool {
	out := map[string]bool{}
	a := tb.c.State().Actors[actor]
	for _, id := range a.GetControllerIds() {
		out[id] = true
	}
	return out
}

func qaWaitHead(t *testing.T, tb *qaTable, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tb.dial(t, tb.dm).head() >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("log never reached %d", want)
}

// VTT-559 VTT-560 VTT-562 VTT-563 VTT-392
func TestQARemovalByDMRevokesEachControlInActorOrder(t *testing.T) {
	tb := qaNewTable(t)
	dm := tb.dial(t, tb.dm)
	before := dm.head()
	res := dm.remove(t, tb.p.id)
	if !res.GetOk() {
		t.Fatalf("refused: %s", res.GetError())
	}
	if res.GetSequence() != before+1 {
		t.Fatalf("result sequence %d, want %d", res.GetSequence(), before+1)
	}
	dm.await(t, func(f *vttv1.ServerFrame) bool { return len(dm.revocations()) >= 3 })
	time.Sleep(300 * time.Millisecond)
	ev := dm.eventsAfter(before)
	if len(ev) != 3 {
		t.Fatalf("appended %d events, want exactly 3", len(ev))
	}
	for i, want := range []string{"act-aa", "act-mm", "act-zz"} {
		r := ev[i].GetActorControlRevoked()
		if r == nil || r.GetActorId() != want || r.GetParticipantId() != tb.p.id {
			t.Fatalf("event %d = %v, want revoke of p on %s", i, ev[i], want)
		}
		if ev[i].GetParticipantId() != tb.dm.id || ev[i].GetActorRole() != "dm" {
			t.Fatalf("event %d issuer = %q/%q", i, ev[i].GetParticipantId(), ev[i].GetActorRole())
		}
		if ev[i].GetSequence() != before+1+int64(i) {
			t.Fatalf("event %d sequence %d", i, ev[i].GetSequence())
		}
	}
	for _, a := range []string{"act-aa", "act-mm", "act-zz"} {
		if qaControllers(tb, a)[tb.p.id] {
			t.Fatalf("%s still controlled by p", a)
		}
	}
	for _, a := range []string{"act-aa", "act-mm", "act-nn"} {
		if !qaControllers(tb, a)[tb.q.id] {
			t.Fatalf("q lost %s", a)
		}
	}
	if k := tb.c.State().Actors["act-aa"].GetKind(); k != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Fatalf("kind after revocation = %v", k)
	}
}

// VTT-557 VTT-562
func TestQARemovalByAgentRecordsTheAgent(t *testing.T) {
	tb := qaNewTable(t)
	ag := tb.dial(t, tb.agent)
	before := ag.head()
	if res := ag.remove(t, tb.q.id); !res.GetOk() {
		t.Fatalf("refused: %s", res.GetError())
	}
	ag.await(t, func(f *vttv1.ServerFrame) bool { return len(ag.revocations()) >= 3 })
	for _, e := range ag.eventsAfter(before) {
		if e.GetParticipantId() != tb.agent.id || e.GetActorRole() != "agent" {
			t.Fatalf("issuer = %q/%q", e.GetParticipantId(), e.GetActorRole())
		}
	}
	if _, err := tb.ids.Lookup(tb.q.id); !errors.Is(err, identity.ErrInvalidToken) {
		t.Fatalf("Lookup of the participant the agent removed: %v, want ErrInvalidToken", err)
	}
}

// VTT-557 VTT-376 VTT-104
func TestQARemovalClosesEveryConnectionAtOnce(t *testing.T) {
	tb := qaNewTable(t)
	dm := tb.dial(t, tb.dm)
	obs := tb.dial(t, tb.agent)
	p1, p2 := tb.dial(t, tb.p), tb.dial(t, tb.p)
	other := tb.dial(t, tb.q)
	if !p1.stayedOpen(200*time.Millisecond) || !p2.stayedOpen(0) {
		t.Fatal("p closed before removal")
	}
	if res := dm.remove(t, tb.p.id); !res.GetOk() {
		t.Fatalf("refused: %s", res.GetError())
	}
	for i, c := range []*qaConn{p1, p2} {
		code, reason := c.closeInfo(t)
		if code != websocket.StatusPolicyViolation || reason != qaGoneReason {
			t.Fatalf("conn %d closed %d %q", i, code, reason)
		}
		if len(c.revocations()) != 0 {
			t.Fatalf("conn %d was sent the removal's events", i)
		}
	}
	if !other.stayedOpen(300 * time.Millisecond) {
		t.Fatal("another participant's connection was closed")
	}
	obs.await(t, func(f *vttv1.ServerFrame) bool {
		pc := f.GetPresenceChanged()
		return pc != nil && pc.GetParticipantId() == tb.p.id && pc.GetState() == vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED
	})
}

// VTT-557 VTT-558 VTT-561 VTT-376
func TestQARemovalOfAParticipantWithNoControlAppendsNothingButCloses(t *testing.T) {
	tb := qaNewTable(t)
	dm := tb.dial(t, tb.dm)
	idle := tb.dial(t, tb.idle)
	before := dm.head()
	if res := dm.remove(t, tb.idle.id); !res.GetOk() {
		t.Fatalf("refused: %s", res.GetError())
	}
	code, reason := idle.closeInfo(t)
	if code != websocket.StatusPolicyViolation || reason != qaGoneReason {
		t.Fatalf("closed %d %q", code, reason)
	}
	time.Sleep(300 * time.Millisecond)
	if n := len(dm.eventsAfter(before)); n != 0 {
		t.Fatalf("appended %d events", n)
	}
	if h := tb.dial(t, tb.dm).head(); h != before {
		t.Fatalf("head moved %d -> %d", before, h)
	}
	if st := qaStatus(t, tb, tb.idle.token); st != http.StatusUnauthorized {
		t.Fatalf("status %d", st)
	}
}

// VTT-557 VTT-080
func TestQARemovalOfASpectatorWhoIsNotConnected(t *testing.T) {
	tb := qaNewTable(t)
	dm := tb.dial(t, tb.dm)
	if st := qaStatus(t, tb, tb.spec.token); st != http.StatusSwitchingProtocols {
		t.Fatalf("spectator before removal: %d", st)
	}
	if res := dm.remove(t, tb.spec.id); !res.GetOk() {
		t.Fatalf("refused: %s", res.GetError())
	}
	if st := qaStatus(t, tb, tb.spec.token); st != http.StatusUnauthorized {
		t.Fatalf("spectator after removal: %d", st)
	}
}

// VTT-557 VTT-080 VTT-074 VTT-034 VTT-544
func TestQARemovalRevokesTheCredentialAndFreesTheName(t *testing.T) {
	tb := qaNewTable(t)
	dm := tb.dial(t, tb.dm)
	if st := qaStatus(t, tb, tb.p.token); st != http.StatusSwitchingProtocols {
		t.Fatalf("before: %d", st)
	}
	if res := dm.remove(t, tb.p.id); !res.GetOk() {
		t.Fatalf("refused: %s", res.GetError())
	}
	if st := qaStatus(t, tb, tb.p.token); st != http.StatusUnauthorized {
		t.Fatalf("after: %d", st)
	}
	if _, err := tb.ids.Verify(tb.p.token); err == nil {
		t.Fatal("Verify still answers")
	}
	if _, err := tb.ids.Lookup(tb.p.id); err == nil {
		t.Fatal("Lookup still answers")
	}
	listed := map[string]bool{}
	ps, err := tb.ids.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range ps {
		listed[x.ID] = true
	}
	if listed[tb.p.id] || !listed[tb.q.id] {
		t.Fatalf("listing wrong: %v", listed)
	}
	if _, _, err := tb.ids.CreateInvite(tb.p.name, identity.RolePlayer); err != nil {
		t.Fatalf("name not free: %v", err)
	}
	if _, _, err := tb.ids.CreateInvite(tb.q.name, identity.RolePlayer); err == nil {
		t.Fatal("a held name was handed out")
	}
}

// VTT-564 VTT-557 VTT-162
func TestQARemovalOfADMAnAgentUnknownOrRevokedIsRefused(t *testing.T) {
	tb := qaNewTable(t)
	dm := tb.dial(t, tb.dm)
	watcher := tb.dial(t, tb.agent)
	dm2 := tb.dial(t, tb.dm2)
	before := dm.head()
	for name, target := range map[string]string{"dm": tb.dm2.id, "self-dm": tb.dm.id, "agent": tb.agent.id, "unknown": "no-such-participant"} {
		if res := dm.remove(t, target); res.GetOk() {
			t.Fatalf("%s: removal accepted", name)
		}
	}
	if res := dm.remove(t, tb.q.id); !res.GetOk() {
		t.Fatalf("positive removal refused: %s", res.GetError())
	}
	mark := watcher.head()
	qaWaitHead(t, tb, before+1)
	after := tb.dial(t, tb.dm).head()
	if res := dm.remove(t, tb.q.id); res.GetOk() {
		t.Fatal("second removal accepted")
	}
	time.Sleep(300 * time.Millisecond)
	if h := tb.dial(t, tb.dm).head(); h != after {
		t.Fatalf("refused removals moved head %d -> %d", after, h)
	}
	for _, e := range watcher.eventsAfter(mark) {
		if e.GetSequence() > after {
			t.Fatalf("watcher received %v from a refusal", e)
		}
	}
	for _, w := range []qaWho{tb.dm, tb.dm2, tb.agent} {
		if _, err := tb.ids.Verify(w.token); err != nil {
			t.Fatalf("%s revoked: %v", w.name, err)
		}
	}
	if !dm2.stayedOpen(300 * time.Millisecond) {
		t.Fatal("dm2 connection closed")
	}
	if !qaControllers(tb, "act-zz")[tb.p.id] {
		t.Fatal("p lost control through a refusal")
	}
}

// VTT-565
func TestQAOnlyADMOrAgentMayRemove(t *testing.T) {
	tb := qaNewTable(t)
	pl, sp := tb.dial(t, tb.q), tb.dial(t, tb.spec)
	dm := tb.dial(t, tb.dm)
	before := dm.head()
	for name, c := range map[string]*qaConn{"player": pl, "spectator": sp} {
		if res := c.remove(t, tb.p.id); res.GetOk() {
			t.Fatalf("%s removed a participant", name)
		}
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := tb.ids.Verify(tb.p.token); err != nil {
		t.Fatalf("p revoked by a non-privileged issuer: %v", err)
	}
	if h := tb.dial(t, tb.dm).head(); h != before {
		t.Fatalf("head moved %d -> %d", before, h)
	}
	if res := dm.remove(t, tb.p.id); !res.GetOk() {
		t.Fatalf("dm refused: %s", res.GetError())
	}
}
