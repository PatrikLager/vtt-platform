package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
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

const (
	qaRejudgeWait  = 3 * time.Second
	qaRejudgeQuiet = 300 * time.Millisecond
)

type qaRejudgeEnv struct {
	c    *campaign.Campaign
	ids  *identity.DB
	srv  *httptest.Server
	reqs int
}

type qaRejudgeConn struct {
	conn *websocket.Conn
	ch   chan *vttv1.ServerFrame
}

type qaRejudgeOpened struct {
	viewpoint string
	head      int64
	events    []*vttv1.Envelope
}

func qaRejudgeNewEnv(t *testing.T) *qaRejudgeEnv {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(srv.Close)
	return &qaRejudgeEnv{c: c, ids: ids, srv: srv}
}

func (e *qaRejudgeEnv) invite(t *testing.T, name string, role identity.Role) (string, string) {
	t.Helper()
	token, id, err := e.ids.CreateInvite(name, role)
	if err != nil {
		t.Fatalf("CreateInvite %s: %v", name, err)
	}
	return token, id
}

func (e *qaRejudgeEnv) dial(t *testing.T, token, log string) *qaRejudgeConn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/ws?after=0&token=" + url.QueryEscape(token)
	if log != "" {
		u += "&log=" + url.QueryEscape(log)
	}
	ctx, cancel := context.WithCancel(context.Background())
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		cancel()
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(1 << 20)
	rc := &qaRejudgeConn{conn: conn, ch: make(chan *vttv1.ServerFrame, 4096)}
	go func() {
		defer close(rc.ch)
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, f); err != nil {
				return
			}
			rc.ch <- f
		}
	}()
	t.Cleanup(func() {
		cancel()
		_ = conn.CloseNow()
	})
	return rc
}

func qaRejudgeIsSideFrame(f *vttv1.ServerFrame) bool {
	return f.GetResult() != nil || f.GetPresenceChanged() != nil || f.GetPresenceSnapshot() != nil ||
		f.GetWatchableChanged() != nil
}

func (rc *qaRejudgeConn) seatFrame(t *testing.T, d time.Duration) *vttv1.ServerFrame {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-rc.ch:
			if !ok {
				t.Fatalf("connection closed while waiting for a frame")
			}
			if qaRejudgeIsSideFrame(f) {
				continue
			}
			return f
		case <-deadline:
			return nil
		}
	}
}

func (rc *qaRejudgeConn) mustFrame(t *testing.T) *vttv1.ServerFrame {
	t.Helper()
	f := rc.seatFrame(t, qaRejudgeWait)
	if f == nil {
		t.Fatalf("no frame within %s", qaRejudgeWait)
	}
	return f
}

func (rc *qaRejudgeConn) mustQuiet(t *testing.T) {
	t.Helper()
	if f := rc.seatFrame(t, qaRejudgeQuiet); f != nil {
		t.Fatalf("expected no frame, got %v", f)
	}
}

func (rc *qaRejudgeConn) mustEvent(t *testing.T) *vttv1.Envelope {
	t.Helper()
	f := rc.mustFrame(t)
	if f.GetEvent() == nil {
		t.Fatalf("want an event, got %v", f)
	}
	return f.GetEvent()
}

func (rc *qaRejudgeConn) settle(t *testing.T) qaRejudgeOpened {
	t.Helper()
	var o qaRejudgeOpened
	sawHead, sawVP := false, false
	for {
		done := sawHead && sawVP
		if done {
			if o.head == 0 || (len(o.events) > 0 && o.events[len(o.events)-1].GetSequence() >= o.head) {
				return o
			}
		}
		f := rc.mustFrame(t)
		switch {
		case f.GetCatchUpHead() != nil:
			sawHead = true
			o.head = f.GetCatchUpHead().GetHeadSequence()
		case f.GetViewpointChanged() != nil:
			sawVP = true
			o.viewpoint = f.GetViewpointChanged().GetActorId()
		case f.GetEvent() != nil:
			o.events = append(o.events, f.GetEvent())
		}
	}
}

func (e *qaRejudgeEnv) open(t *testing.T, token, log string) (*qaRejudgeConn, qaRejudgeOpened) {
	t.Helper()
	rc := e.dial(t, token, log)
	return rc, rc.settle(t)
}

func (e *qaRejudgeEnv) do(t *testing.T, dm *qaRejudgeConn, cmd *vttv1.ClientCommand) {
	t.Helper()
	e.reqs++
	cmd.RequestId = fmt.Sprintf("q%d", e.reqs)
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaRejudgeWait)
	defer cancel()
	if err := dm.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("write: %v", err)
	}
	deadline := time.After(qaRejudgeWait)
	for {
		select {
		case f, ok := <-dm.ch:
			if !ok {
				t.Fatalf("dm connection closed")
			}
			if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
				if !r.GetOk() {
					t.Fatalf("command refused: %s", r.GetError())
				}
				return
			}
		case <-deadline:
			t.Fatalf("no result for %s", cmd.RequestId)
		}
	}
}

func (e *qaRejudgeEnv) addActor(t *testing.T, dm *qaRejudgeConn, id string, kind vttv1.ActorKind) {
	t.Helper()
	e.do(t, dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: kind},
	}}})
}

func (e *qaRejudgeEnv) grant(t *testing.T, dm *qaRejudgeConn, actor, participant string, kind vttv1.ActorKind) {
	t.Helper()
	e.do(t, dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: actor, ParticipantId: participant, Kind: kind,
	}}})
}

func (e *qaRejudgeEnv) promote(t *testing.T, dm *qaRejudgeConn, participant string, role identity.Role) {
	t.Helper()
	e.do(t, dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{PromoteParticipant: &vttv1.PromoteParticipant{
		ParticipantId: participant, Role: string(role),
	}}})
}

func (e *qaRejudgeEnv) narrate(t *testing.T, dm *qaRejudgeConn) {
	t.Helper()
	e.do(t, dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
		Text: "the room is quiet",
	}}})
}

func (e *qaRejudgeEnv) logHead(t *testing.T) int64 {
	t.Helper()
	events, unsub, head, err := e.c.Subscribe(0, 4096)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	_ = events
	return head
}

const qaRejudgeParty = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER

type qaRejudgeTable struct {
	env *qaRejudgeEnv
	dm  *qaRejudgeConn
}

func qaRejudgeNewTable(t *testing.T) *qaRejudgeTable {
	t.Helper()
	env := qaRejudgeNewEnv(t)
	dmToken, _ := env.invite(t, "dm", identity.RoleDM)
	return &qaRejudgeTable{env: env, dm: env.dial(t, dmToken, "")}
}

// VTT-923 VTT-896 VTT-342
func TestQARejudgeAPlayerMadeASpectatorKeepsItsPartyMemberLog(t *testing.T) {
	tb := qaRejudgeNewTable(t)
	e := tb.env
	pTok, pid := e.invite(t, "p1", identity.RolePlayer)
	rTok, _ := e.invite(t, "r1", identity.RoleSpectator)
	e.addActor(t, tb.dm, "a1", qaRejudgeParty)
	e.grant(t, tb.dm, "a1", pid, qaRejudgeParty)
	e.narrate(t, tb.dm)

	p, o := e.open(t, pTok, "")
	if o.viewpoint != "a1" {
		t.Fatalf("player opened on %q, want a1", o.viewpoint)
	}
	r, _ := e.open(t, rTok, "a1")
	e.promote(t, tb.dm, pid, identity.RoleSpectator)

	want := r.mustEvent(t)
	if rv := want.GetActorControlRevoked(); rv.GetActorId() != "a1" || rv.GetParticipantId() != pid {
		t.Fatalf("a1's log gained %v after the player was made a spectator, want their revocation", want)
	}
	if got := p.mustEvent(t); !proto.Equal(got, want) {
		t.Fatalf("made a spectator, the connection was sent %v, want a1's revocation %v and no move", got, want)
	}
	e.narrate(t, tb.dm)
	want = r.mustEvent(t)
	got := p.mustEvent(t)
	if got.GetSequence() != want.GetSequence() {
		t.Fatalf("event sequence %d, want %d", got.GetSequence(), want.GetSequence())
	}
}

func TestQARejudgePromotingAnotherParticipantLeavesTheSeatAlone(t *testing.T) {
	tb := qaRejudgeNewTable(t)
	e := tb.env
	_, pid := e.invite(t, "p1", identity.RolePlayer)
	sTok, _ := e.invite(t, "s1", identity.RoleSpectator)
	_, oid := e.invite(t, "s2", identity.RoleSpectator)
	e.addActor(t, tb.dm, "a1", qaRejudgeParty)
	e.grant(t, tb.dm, "a1", pid, qaRejudgeParty)
	e.addActor(t, tb.dm, "a2", qaRejudgeParty)
	e.grant(t, tb.dm, "a2", oid, qaRejudgeParty)
	e.narrate(t, tb.dm)

	s, o := e.open(t, sTok, "a1")
	if o.viewpoint != "a1" {
		t.Fatalf("spectator opened on %q, want a1", o.viewpoint)
	}
	e.promote(t, tb.dm, oid, identity.RoleSpectator)
	s.mustQuiet(t)
}

// VTT-896
func TestQARejudgeASpectatorPromotedToSpectatorKeepsItsPerch(t *testing.T) {
	tb := qaRejudgeNewTable(t)
	e := tb.env
	_, pid := e.invite(t, "p1", identity.RolePlayer)
	sTok, sid := e.invite(t, "s1", identity.RoleSpectator)
	e.addActor(t, tb.dm, "a1", qaRejudgeParty)
	e.grant(t, tb.dm, "a1", pid, qaRejudgeParty)
	e.narrate(t, tb.dm)

	s, o := e.open(t, sTok, "a1")
	if o.viewpoint != "a1" {
		t.Fatalf("spectator opened on %q, want a1", o.viewpoint)
	}
	before := e.logHead(t)
	e.promote(t, tb.dm, sid, identity.RoleSpectator)
	s.mustQuiet(t)
	if after := e.logHead(t); after != before {
		t.Fatalf("making a spectator who controls nothing a spectator moved the log head from %d to %d", before, after)
	}
}
