package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
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

const (
	qaMGUnseen  = "gateway: cannot move there — you cannot see that square"
	qaMGBlocked = "gateway: cannot move there — "
	qaMGWait    = 10 * time.Second
)

type qaMGTable struct {
	t   *testing.T
	c   *campaign.Campaign
	ids *identity.DB
	srv *httptest.Server
	seq atomic.Int64
}

func qaMGNewTable(t *testing.T) *qaMGTable {
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
	tb := &qaMGTable{t: t, c: c, ids: ids, srv: srv}
	tb.rooms()
	return tb
}

// Keep s1's rooms L (columns 0-2), R (4-6) and Z (8-9) sealed by the wall
// columns 3 and 7: every test assumes no room sees another. Keep the crates
// at (0,2) and (6,2) blocking movement and not sight.
func (tb *qaMGTable) rooms() {
	tb.t.Helper()
	tb.append(qaMGScene("s1", 10, 3, func(x, _ int32) bool { return x == 3 || x == 7 },
		qaMGCrate("crate-left", 0, 2), qaMGCrate("crate-right", 6, 2)))
	tb.append(qaMGScene("s2", 3, 3, func(_, _ int32) bool { return false }))
}

func qaMGScene(id string, w, h int32, wall func(x, y int32) bool, objects ...*vttv1.SceneObject) *vttv1.Envelope {
	return qaMGTiledScene(id, w, h, func(x, y int32) string {
		if wall(x, y) {
			return "wall"
		}
		return "floor"
	}, objects...)
}

func qaMGTiledScene(id string, w, h int32, kind func(x, y int32) string, objects ...*vttv1.SceneObject) *vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < w; x++ {
		for y := int32(0); y < h; y++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind(x, y)}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: w, GridHeight: h, Tiles: tiles, Objects: objects,
	}}}
}

func qaMGCrate(id string, x, y int32) *vttv1.SceneObject {
	return &vttv1.SceneObject{
		ObjectId: id, Kind: "crate", At: qaMGAt(x, y), Width: 1, Height: 1,
		BlocksMove: true, BlocksSight: false,
	}
}

func qaMGAt(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }

func (tb *qaMGTable) stamp(env *vttv1.Envelope) *vttv1.Envelope {
	env.EventId = fmt.Sprintf("qamg-event-%d", tb.seq.Add(1))
	env.OccurredAt = timestamppb.Now()
	return env
}

func (tb *qaMGTable) append(envs ...*vttv1.Envelope) {
	tb.t.Helper()
	for _, env := range envs {
		tb.stamp(env)
	}
	var err error
	if len(envs) == 1 {
		_, err = tb.c.Append(envs[0])
	} else {
		_, err = tb.c.AppendBatch(envs)
	}
	if err != nil {
		tb.t.Fatalf("append: %v", err)
	}
}

func (tb *qaMGTable) invite(name string, role identity.Role) (token, id string) {
	tb.t.Helper()
	token, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("CreateInvite %s: %v", name, err)
	}
	return token, id
}

func (tb *qaMGTable) character(actorID string, kind vttv1.ActorKind, tokenID, scene string, x, y int32, controllers ...string) {
	tb.t.Helper()
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: actorID, Name: actorID, Kind: kind},
	}}})
	for _, pid := range controllers {
		tb.grant(actorID, pid, kind)
	}
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tokenID, SceneId: scene, ActorId: actorID, Position: qaMGAt(x, y),
	}}})
}

func (tb *qaMGTable) grant(actorID, pid string, kind vttv1.ActorKind) {
	tb.t.Helper()
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actorID, ParticipantId: pid, Kind: kind,
	}}})
}

func (tb *qaMGTable) revoke(actorID, pid string) {
	tb.t.Helper()
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actorID, ParticipantId: pid,
	}}})
}

func (tb *qaMGTable) remove(actorID, tokenID string) {
	tb.t.Helper()
	tb.append(
		&vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tokenID}}},
		&vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actorID}}},
	)
}

// Keep pc-a first by id and not first added: a default taken in creation
// order must fail.
func (tb *qaMGTable) party(pid string) {
	tb.t.Helper()
	tb.character("pc-z", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-z", "s1", 9, 1, pid)
	tb.character("pc-b", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-b", "s1", 5, 1, pid)
	tb.character("pc-a", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-a", "s1", 1, 1, pid)
}

// Never read a result by position: results and the other frames are not
// ordered against each other.
type qaMGConn struct {
	t        *testing.T
	name     string
	ws       *websocket.Conn
	mu       sync.Mutex
	frames   []*vttv1.ServerFrame
	results  map[string]*vttv1.CommandResult
	readErr  error
	wake     chan struct{}
	next     atomic.Int64
	consumed int
}

func (tb *qaMGTable) dial(name, token, log string) *qaMGConn {
	tb.t.Helper()
	u := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" + url.QueryEscape(token) + "&after=0"
	if log != "" {
		u += "&log=" + url.QueryEscape(log)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaMGWait)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		tb.t.Fatalf("dial %s: %v", name, err)
	}
	ws.SetReadLimit(200 * 1024)
	qc := &qaMGConn{t: tb.t, name: name, ws: ws, results: map[string]*vttv1.CommandResult{}, wake: make(chan struct{}, 1)}
	readCtx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go qc.read(readCtx, done)
	tb.t.Cleanup(func() {
		stop()
		_ = ws.CloseNow()
		<-done
	})
	return qc
}

func (qc *qaMGConn) read(ctx context.Context, done chan struct{}) {
	defer close(done)
	for {
		_, data, err := qc.ws.Read(ctx)
		if err != nil {
			qc.mu.Lock()
			qc.readErr = err
			qc.mu.Unlock()
			qc.poke()
			return
		}
		f := &vttv1.ServerFrame{}
		if err := protojson.Unmarshal(data, f); err != nil {
			qc.mu.Lock()
			qc.readErr = fmt.Errorf("decode frame: %w", err)
			qc.mu.Unlock()
			qc.poke()
			return
		}
		qc.mu.Lock()
		qc.frames = append(qc.frames, f)
		if r := f.GetResult(); r != nil {
			qc.results[r.GetRequestId()] = r
		}
		qc.mu.Unlock()
		qc.poke()
	}
}

func (qc *qaMGConn) poke() {
	select {
	case qc.wake <- struct{}{}:
	default:
	}
}

func (qc *qaMGConn) waitFor(what string, cond func() bool) {
	qc.t.Helper()
	deadline := time.NewTimer(qaMGWait)
	defer deadline.Stop()
	for {
		qc.mu.Lock()
		ok := cond()
		err := qc.readErr
		qc.mu.Unlock()
		if ok {
			return
		}
		if err != nil {
			qc.t.Fatalf("%s: waiting for %s: connection read ended: %v", qc.name, what, err)
		}
		select {
		case <-qc.wake:
		case <-deadline.C:
			qc.t.Fatalf("%s: timed out waiting for %s", qc.name, what)
		}
	}
}

func (qc *qaMGConn) viewpointsLocked() []string {
	var out []string
	for _, f := range qc.frames {
		if vc := f.GetViewpointChanged(); vc != nil {
			out = append(out, vc.GetActorId())
		}
	}
	return out
}

func (qc *qaMGConn) nextViewpoint() string {
	qc.t.Helper()
	n := qc.consumed
	qc.waitFor(fmt.Sprintf("ViewpointChanged #%d", n+1), func() bool { return len(qc.viewpointsLocked()) > n })
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.consumed++
	return qc.viewpointsLocked()[n]
}

func (qc *qaMGConn) expectViewpoint(want string) {
	qc.t.Helper()
	if got := qc.nextViewpoint(); got != want {
		qc.t.Fatalf("%s: ViewpointChanged names %q, want %q", qc.name, got, want)
	}
}

func (qc *qaMGConn) waitCatchUpHead() {
	qc.t.Helper()
	qc.waitFor("CatchUpHead", func() bool {
		for _, f := range qc.frames {
			if f.GetCatchUpHead() != nil {
				return true
			}
		}
		return false
	})
}

func (qc *qaMGConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	qc.t.Helper()
	id := fmt.Sprintf("%s-%d", qc.name, qc.next.Add(1))
	cmd.RequestId = id
	b, err := protojson.Marshal(cmd)
	if err != nil {
		qc.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaMGWait)
	defer cancel()
	if err := qc.ws.Write(ctx, websocket.MessageText, b); err != nil {
		qc.t.Fatalf("%s: write: %v", qc.name, err)
	}
	qc.waitFor("result "+id, func() bool { _, ok := qc.results[id]; return ok })
	qc.mu.Lock()
	defer qc.mu.Unlock()
	return qc.results[id]
}

func (qc *qaMGConn) move(tokenID string, x, y int32) *vttv1.CommandResult {
	qc.t.Helper()
	return qc.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tokenID, To: qaMGAt(x, y),
	}}})
}

func (qc *qaMGConn) setViewpoint(actorID string) *vttv1.CommandResult {
	qc.t.Helper()
	return qc.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{
		ActorId: actorID,
	}}})
}

func qaMGWantOK(t *testing.T, what string, r *vttv1.CommandResult) {
	t.Helper()
	if !r.GetOk() {
		t.Errorf("%s: refused with %q, want ok", what, r.GetError())
	}
}

func qaMGWantRefused(t *testing.T, what string, r *vttv1.CommandResult, want string) {
	t.Helper()
	if r.GetOk() {
		t.Errorf("%s: accepted, want refused", what)
		return
	}
	if want != "" && r.GetError() != want {
		t.Errorf("%s: refused with %q, want %q", what, r.GetError(), want)
	}
}

// VTT-340 VTT-350 VTT-453
func TestQAMGTheDefaultViewpointJudgesEveryControlledTokensMove(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")

	qaMGWantRefused(t, "pc-b's token onto (4,0), which pc-b sees and pc-a does not", pat.move("tok-b", 4, 0), qaMGUnseen)
	qaMGWantRefused(t, "pc-z's token onto (8,0), which pc-z sees and pc-a does not", pat.move("tok-z", 8, 0), qaMGUnseen)
	qaMGWantOK(t, "pc-a's token onto (0,0), which pc-a sees", pat.move("tok-a", 0, 0))
}

// VTT-339 VTT-350 VTT-453
func TestQAMGTwoConnectionsOfOnePlayerAreEachJudgedByTheirOwnCharacter(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	one := tb.dial("pat-one", tok, "")
	one.expectViewpoint("pc-a")
	two := tb.dial("pat-two", tok, "pc-b")
	two.expectViewpoint("pc-b")

	qaMGWantRefused(t, "connection one (pc-a): pc-b's token onto (4,0)", one.move("tok-b", 4, 0), qaMGUnseen)
	qaMGWantOK(t, "connection two (pc-b): pc-b's token onto (4,0)", two.move("tok-b", 4, 0))
	qaMGWantRefused(t, "connection two (pc-b): pc-a's token onto (0,0)", two.move("tok-a", 0, 0), qaMGUnseen)
	qaMGWantOK(t, "connection one (pc-a): pc-a's token onto (0,0)", one.move("tok-a", 0, 0))

	qaMGWantOK(t, "connection one: set_viewpoint pc-b", one.setViewpoint("pc-b"))
	one.expectViewpoint("pc-b")
	qaMGWantOK(t, "connection two: set_viewpoint pc-z", two.setViewpoint("pc-z"))
	two.expectViewpoint("pc-z")
	qaMGWantOK(t, "connection one (pc-b): pc-b's token onto (6,0)", one.move("tok-b", 6, 0))
	qaMGWantRefused(t, "connection two (pc-z): pc-b's token onto (4,0)", two.move("tok-b", 4, 0), qaMGUnseen)
	qaMGWantOK(t, "connection two (pc-z): pc-z's token onto (8,0)", two.move("tok-z", 8, 0))
	qaMGWantRefused(t, "connection one (pc-b): pc-z's token onto (9,0)", one.move("tok-z", 9, 0), qaMGUnseen)
}

// VTT-350 VTT-453
func TestQAMGAWatchedCharacterWithoutATokenSeesNothing(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "pc-b")
	pat.expectViewpoint("pc-b")
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "tok-b"}}})

	qaMGWantRefused(t, "watching pc-b with no token, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0), qaMGUnseen)
	qaMGWantRefused(t, "watching pc-b with no token, pc-z's token onto (8,0)", pat.move("tok-z", 8, 0), qaMGUnseen)
}

// VTT-350
func TestQAMGAClosedDoorHidesTheRoomBeyondIt(t *testing.T) {
	tb := qaMGNewTable(t)
	dmTok, _ := tb.invite("dm", identity.RoleDM)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	// Keep the door at (3,1) the only opening between the two rooms.
	tb.append(qaMGTiledScene("s3", 7, 3, func(x, y int32) string {
		switch {
		case x == 3 && y == 1:
			return "door"
		case x == 3:
			return "wall"
		}
		return "floor"
	}))
	tb.character("pc-a", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-a", "s3", 1, 1, pid)
	tb.character("pc-b", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-b", "s3", 5, 1, pid)

	dm := tb.dial("dm", dmTok, "")
	dm.waitCatchUpHead()
	// Ignore close_door's result: a door's starting state is not specified.
	dm.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_CloseDoor{CloseDoor: &vttv1.CloseDoor{SceneId: "s3", At: qaMGAt(3, 1)}}})

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")
	qaMGWantRefused(t, "door closed: pc-b's token onto (4,1)", pat.move("tok-b", 4, 1), qaMGUnseen)

	qaMGWantOK(t, "DM: open_door (3,1)", dm.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_OpenDoor{OpenDoor: &vttv1.OpenDoor{SceneId: "s3", At: qaMGAt(3, 1)}}}))
	qaMGWantOK(t, "door open: pc-b's token onto (4,1)", pat.move("tok-b", 4, 1))

	qaMGWantOK(t, "DM: close_door (3,1)", dm.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_CloseDoor{CloseDoor: &vttv1.CloseDoor{SceneId: "s3", At: qaMGAt(3, 1)}}}))
	qaMGWantRefused(t, "door closed again: pc-b's token onto (5,1)", pat.move("tok-b", 5, 1), qaMGUnseen)
}

// VTT-342 VTT-350 VTT-453
func TestQAMGASwitchedViewpointJudgesTheMovesAfterTheSwitch(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")
	qaMGWantRefused(t, "before the switch, pc-b's token onto (4,0)", pat.move("tok-b", 4, 0), qaMGUnseen)

	qaMGWantOK(t, "set_viewpoint pc-b", pat.setViewpoint("pc-b"))
	pat.expectViewpoint("pc-b")
	qaMGWantOK(t, "after the switch to pc-b, pc-b's token onto (4,0)", pat.move("tok-b", 4, 0))
	qaMGWantRefused(t, "after the switch to pc-b, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0), qaMGUnseen)

	qaMGWantOK(t, "set_viewpoint pc-a", pat.setViewpoint("pc-a"))
	pat.expectViewpoint("pc-a")
	qaMGWantOK(t, "after the switch back to pc-a, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0))
	qaMGWantRefused(t, "after the switch back to pc-a, pc-b's token onto (6,0)", pat.move("tok-b", 6, 0), qaMGUnseen)
}

// VTT-339 VTT-350 VTT-453 VTT-368
func TestQAMGAConnectionOpenedOnALogJudgesByThatCharacter(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "pc-b")
	pat.expectViewpoint("pc-b")

	qaMGWantRefused(t, "pc-a's token onto (0,0), which pc-b does not see", pat.move("tok-a", 0, 0), qaMGUnseen)
	qaMGWantOK(t, "pc-b's token onto (4,0), which pc-b sees", pat.move("tok-b", 4, 0))

	pat.mu.Lock()
	defer pat.mu.Unlock()
	firstVC, firstEvent := -1, -1
	for i, f := range pat.frames {
		if firstVC < 0 && f.GetViewpointChanged() != nil {
			firstVC = i
		}
		if firstEvent < 0 && f.GetEvent() != nil {
			firstEvent = i
		}
	}
	if firstEvent >= 0 && firstEvent < firstVC {
		t.Errorf("an entry (frame %d) arrived before the first ViewpointChanged (frame %d)", firstEvent, firstVC)
	}
}

// VTT-340 VTT-453
func TestQAMGALogThePlayerMayNotWatchOpensOnTheFirstPartyMember(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	_, oli := tb.invite("oli", identity.RolePlayer)
	tb.party(pid)
	// Keep every log named below one that sees room R, which pc-a does not.
	tb.character("pc-o", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-o", "s1", 6, 1, oli)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 6, 0, pid)
	tb.character("npc-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-goblin", "s1", 4, 2)

	for i, log := range []string{"pc-o", "npc-pet", "npc-goblin", "pc-missing"} {
		pat := tb.dial(fmt.Sprintf("pat-log-%s", log), tok, log)
		pat.expectViewpoint("pc-a")
		qaMGWantRefused(t, fmt.Sprintf("log=%s: pc-b's token onto (4,%d)", log, i%3), pat.move("tok-b", 4, int32(i%3)), qaMGUnseen)
		x := int32(i % 2 * 2)
		qaMGWantOK(t, fmt.Sprintf("log=%s: pc-a's token onto (%d,0)", log, x), pat.move("tok-a", x, 0))
	}
}

// VTT-340 VTT-342 VTT-453
func TestQAMGRevokingTheWatchedCharacterJudgesByTheFirstPartyMember(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "pc-b")
	pat.expectViewpoint("pc-b")
	qaMGWantRefused(t, "watching pc-b, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0), qaMGUnseen)

	tb.revoke("pc-b", pid)
	qaMGWantOK(t, "at the snapshot after the revoke, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0))

	pat.expectViewpoint("pc-a")
	qaMGWantOK(t, "after the switch to pc-a, pc-a's token onto (2,2)", pat.move("tok-a", 2, 2))
	qaMGWantRefused(t, "after the switch to pc-a, pc-z's token onto (8,0)", pat.move("tok-z", 8, 0), qaMGUnseen)
}

// VTT-340 VTT-453
func TestQAMGMakingTheWatchedCharacterNonPartyJudgesByTheFirstPartyMember(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "pc-b")
	pat.expectViewpoint("pc-b")
	qaMGWantRefused(t, "watching pc-b, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0), qaMGUnseen)

	tb.grant("pc-b", pid, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	qaMGWantOK(t, "at the snapshot after pc-b turns non-party, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0))
	qaMGWantRefused(t, "at the snapshot after pc-b turns non-party, pc-b's token onto (4,0)", pat.move("tok-b", 4, 0), qaMGUnseen)

	pat.expectViewpoint("pc-a")
	qaMGWantOK(t, "after the switch to pc-a, pc-a's token onto (2,2)", pat.move("tok-a", 2, 2))
}

// VTT-340 VTT-453
func TestQAMGRemovingTheWatchedCharacterJudgesByTheFirstPartyMember(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)

	pat := tb.dial("pat", tok, "pc-b")
	pat.expectViewpoint("pc-b")
	qaMGWantRefused(t, "watching pc-b, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0), qaMGUnseen)

	tb.remove("pc-b", "tok-b")
	qaMGWantOK(t, "at the snapshot after pc-b is removed, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0))

	pat.expectViewpoint("pc-a")
	qaMGWantOK(t, "after the switch to pc-a, pc-a's token onto (2,2)", pat.move("tok-a", 2, 2))
	qaMGWantRefused(t, "after the switch to pc-a, pc-z's token onto (8,0)", pat.move("tok-z", 8, 0), qaMGUnseen)
}

// VTT-370 VTT-453 VTT-456
func TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.character("pc-b", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-b", "s1", 5, 1, pid)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 4, 1, pid)

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-b")
	qaMGWantOK(t, "watching pc-b, the non-party pet's token onto (4,0)", pat.move("tok-pet", 4, 0))

	tb.revoke("pc-b", pid)
	qaMGWantRefused(t, "at the snapshot after the revoke, the pet's token onto (4,2)", pat.move("tok-pet", 4, 2), qaMGUnseen)

	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: "pat stays on pc-b"}}})
	pat.waitFor("pc-b's narration", func() bool {
		for _, f := range pat.frames {
			if f.GetEvent().GetNarrationAdded().GetText() == "pat stays on pc-b" {
				return true
			}
		}
		return false
	})
	pat.mu.Lock()
	switches := len(pat.viewpointsLocked()) - pat.consumed
	pat.mu.Unlock()
	if switches != 0 {
		t.Fatalf("the revocation moved pat off pc-b: %d more ViewpointChanged", switches)
	}
	qaMGWantRefused(t, "on pc-b after the revoke, the pet's token onto (4,2)", pat.move("tok-pet", 4, 2), qaMGUnseen)
	qaMGWantRefused(t, "on pc-b after the revoke, the pet's token onto (5,1)", pat.move("tok-pet", 5, 1), qaMGUnseen)
}

// VTT-340 VTT-370 VTT-452 VTT-453
func TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("gil", identity.RolePlayer)
	tb.character("pc-c", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-c", "s1", 0, 0)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 1, 1, pid)

	for _, log := range []string{"", "npc-pet", "pc-c"} {
		gil := tb.dial("gil-log-"+log, tok, log)
		if log == "pc-c" {
			gil.expectViewpoint("pc-c")
		} else {
			gil.expectViewpoint("")
		}
		qaMGWantRefused(t, "log="+log+": the pet's token onto (2,1)", gil.move("tok-pet", 2, 1), qaMGUnseen)
		qaMGWantRefused(t, "log="+log+": the pet's token onto (1,0)", gil.move("tok-pet", 1, 0), qaMGUnseen)
		qaMGWantRefused(t, "log="+log+": the pet's token onto the crate at (0,2)", gil.move("tok-pet", 0, 2), qaMGUnseen)
	}
}

// VTT-340 VTT-342 VTT-453 VTT-370
func TestQAMGAFirstPartyMemberGrantedMidConnectionMovesWithoutReconnecting(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("nia", identity.RolePlayer)
	tb.character("pc-c", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-c", "s1", 1, 1)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 2, 1, pid)

	nia := tb.dial("nia", tok, "")
	nia.expectViewpoint("")
	qaMGWantRefused(t, "before the grant, the pet's token onto (2,2)", nia.move("tok-pet", 2, 2), qaMGUnseen)

	tb.grant("pc-c", pid, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	nia.expectViewpoint("pc-c")
	qaMGWantOK(t, "after the grant, the pet's token onto (2,2)", nia.move("tok-pet", 2, 2))
	qaMGWantOK(t, "after the grant, pc-c's token onto (0,0)", nia.move("tok-c", 0, 0))
	qaMGWantRefused(t, "after the grant, pc-c's token onto (4,0)", nia.move("tok-c", 4, 0), qaMGUnseen)
}

// VTT-453
func TestQAMGAFirstPartyMemberGrantedMidConnectionJudgesTheNextMoveAtItsSnapshot(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("nia", identity.RolePlayer)
	tb.character("pc-c", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-c", "s1", 1, 1)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 2, 1, pid)

	nia := tb.dial("nia", tok, "")
	nia.expectViewpoint("")

	tb.grant("pc-c", pid, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	qaMGWantOK(t, "at the snapshot after the grant, the pet's token onto (2,2)", nia.move("tok-pet", 2, 2))
	qaMGWantOK(t, "at the snapshot after the grant, pc-c's token onto (0,0)", nia.move("tok-c", 0, 0))
}

// VTT-350 VTT-453
func TestQAMGPartyMembersOnDifferentScenesJudgeOnlyTheirOwnScene(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("quinn", identity.RolePlayer)
	tb.character("pc-f", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-f", "s2", 1, 1, pid)
	tb.character("pc-e", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-e", "s1", 1, 1, pid)

	quinn := tb.dial("quinn", tok, "")
	quinn.expectViewpoint("pc-e")
	qaMGWantRefused(t, "watching pc-e, pc-f's token onto s2 (0,0)", quinn.move("tok-f", 0, 0), qaMGUnseen)
	qaMGWantOK(t, "watching pc-e, pc-e's token onto s1 (0,0)", quinn.move("tok-e", 0, 0))

	qaMGWantOK(t, "set_viewpoint pc-f", quinn.setViewpoint("pc-f"))
	quinn.expectViewpoint("pc-f")
	qaMGWantOK(t, "watching pc-f, pc-f's token onto s2 (0,0)", quinn.move("tok-f", 0, 0))
	qaMGWantRefused(t, "watching pc-f, pc-e's token onto s1 (2,2)", quinn.move("tok-e", 2, 2), qaMGUnseen)
}

// VTT-450 VTT-453
func TestQAMGARefusedSetViewpointLeavesTheMoveJudgedByTheCharacterShown(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	_, oli := tb.invite("oli", identity.RolePlayer)
	tb.party(pid)
	tb.character("pc-o", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-o", "s1", 6, 1, oli)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 6, 0, pid)
	tb.character("npc-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-goblin", "s1", 4, 2)

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")
	for _, actor := range []string{"pc-o", "npc-pet", "npc-goblin", "pc-missing"} {
		qaMGWantRefused(t, "set_viewpoint "+actor, pat.setViewpoint(actor), "")
	}
	qaMGWantRefused(t, "after the refused switches, pc-b's token onto (4,0)", pat.move("tok-b", 4, 0), qaMGUnseen)
	qaMGWantOK(t, "after the refused switches, pc-a's token onto (0,0)", pat.move("tok-a", 0, 0))
}

// VTT-156 VTT-350
func TestQAMGAnUnseenSquareIsRefusedWithOneStringWhateverStandsOnIt(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)
	tb.character("npc-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-goblin", "s1", 8, 2)

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")
	squares := []struct {
		what string
		x, y int32
	}{
		{"empty floor in room R", 4, 0},
		{"the crate in room R", 6, 2},
		{"pc-b's own token in room R", 5, 1},
		{"the wall between R and Z", 7, 1},
		{"empty floor in room Z", 8, 0},
		{"a goblin in room Z", 8, 2},
		{"pc-z's token in room Z", 9, 1},
	}
	for _, sq := range squares {
		r := pat.move("tok-a", sq.x, sq.y)
		qaMGWantRefused(t, fmt.Sprintf("pc-a's token onto %s (%d,%d)", sq.what, sq.x, sq.y), r, qaMGUnseen)
	}
}

// VTT-156 VTT-157
func TestQAMGASeenBlockedSquareIsRefusedWithTheObstructionNamed(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	tb.party(pid)
	crate := qaMGBlocked + "something (a crate) is in the way"

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")
	qaMGWantRefused(t, "watching pc-a, pc-a's token onto the crate at (0,2)", pat.move("tok-a", 0, 2), crate)
	qaMGWantRefused(t, "watching pc-a, pc-b's token onto the crate at (6,2)", pat.move("tok-b", 6, 2), qaMGUnseen)

	qaMGWantOK(t, "set_viewpoint pc-b", pat.setViewpoint("pc-b"))
	pat.expectViewpoint("pc-b")
	qaMGWantRefused(t, "watching pc-b, pc-b's token onto the crate at (6,2)", pat.move("tok-b", 6, 2), crate)
	qaMGWantRefused(t, "watching pc-b, pc-a's token onto the crate at (0,2)", pat.move("tok-a", 0, 2), qaMGUnseen)
}

// VTT-158
func TestQAMGTheDMMovesATokenOntoAnySquare(t *testing.T) {
	tb := qaMGNewTable(t)
	dmTok, _ := tb.invite("dm", identity.RoleDM)
	_, pid := tb.invite("pat", identity.RolePlayer)
	tb.character("pc-a", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-a", "s1", 1, 1, pid)
	tb.character("npc-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-goblin", "s1", 2, 0)

	dm := tb.dial("dm", dmTok, "")
	dm.waitCatchUpHead()
	qaMGWantOK(t, "DM: the goblin onto the wall at (3,1)", dm.move("tok-goblin", 3, 1))
	qaMGWantOK(t, "DM: the goblin onto room Z's (9,1), which no player sees", dm.move("tok-goblin", 9, 1))
	qaMGWantOK(t, "DM: the goblin onto the wall at (7,0)", dm.move("tok-goblin", 7, 0))
	qaMGWantOK(t, "DM: pc-a's token onto the wall at (3,2)", dm.move("tok-a", 3, 2))
	qaMGWantOK(t, "DM: pc-a's token onto room R's (4,0)", dm.move("tok-a", 4, 0))
}

// VTT-379
func TestQAMGTheAgentMovesATokenOntoAnySquare(t *testing.T) {
	tb := qaMGNewTable(t)
	agentTok, _ := tb.invite("agent", identity.RoleAgent)
	_, pid := tb.invite("pat", identity.RolePlayer)
	tb.character("pc-a", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-a", "s1", 1, 1, pid)
	tb.character("npc-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-goblin", "s1", 2, 0)

	agent := tb.dial("agent", agentTok, "")
	agent.waitCatchUpHead()
	qaMGWantOK(t, "agent: the goblin onto the wall at (3,0)", agent.move("tok-goblin", 3, 0))
	qaMGWantOK(t, "agent: the goblin onto room Z's (8,1), which no player sees", agent.move("tok-goblin", 8, 1))
	qaMGWantOK(t, "agent: pc-a's token onto the wall at (7,2)", agent.move("tok-a", 7, 2))
}

// VTT-142 VTT-139
func TestQAMGAPlayerMovesOnlyTokensTheyControlAndASpectatorNone(t *testing.T) {
	tb := qaMGNewTable(t)
	tok, pid := tb.invite("pat", identity.RolePlayer)
	specTok, _ := tb.invite("sid", identity.RoleSpectator)
	_, oli := tb.invite("oli", identity.RolePlayer)
	tb.party(pid)
	tb.character("pc-o", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-o", "s1", 2, 2, oli)
	tb.character("npc-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-goblin", "s1", 2, 0)

	pat := tb.dial("pat", tok, "")
	pat.expectViewpoint("pc-a")
	qaMGWantRefused(t, "pat: the uncontrolled goblin onto (1,0), which pc-a sees", pat.move("tok-goblin", 1, 0), "")
	qaMGWantRefused(t, "pat: oli's pc-o onto (2,1), which pc-a sees", pat.move("tok-o", 2, 1), "")
	qaMGWantRefused(t, "pat: a token the table does not hold onto (1,0)", pat.move("tok-missing", 1, 0), "")
	qaMGWantOK(t, "pat: pc-a onto (0,0), which pc-a sees", pat.move("tok-a", 0, 0))

	sid := tb.dial("sid", specTok, "")
	sid.waitCatchUpHead()
	qaMGWantRefused(t, "spectator: the goblin onto (1,0)", sid.move("tok-goblin", 1, 0), "")
	qaMGWantRefused(t, "spectator: pc-a onto (1,0)", sid.move("tok-a", 1, 0), "")
}

// VTT-370 VTT-452 VTT-453
func TestQAMGAPerchedPlayerMovesNoToken(t *testing.T) {
	tb := qaMGNewTable(t)
	_, ann := tb.invite("ann", identity.RolePlayer)
	tok, pid := tb.invite("gil", identity.RolePlayer)
	tb.character("pc-c", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "tok-c", "s1", 0, 0, ann)
	tb.character("npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "tok-pet", "s1", 1, 1, pid)

	gil := tb.dial("gil", tok, "")
	gil.expectViewpoint("")
	qaMGWantOK(t, "the perch on ann's pc-c", gil.setViewpoint("pc-c"))
	gil.expectViewpoint("pc-c")
	qaMGWantRefused(t, "on pc-c, the pet's token onto (2,1)", gil.move("tok-pet", 2, 1), qaMGUnseen)
	qaMGWantRefused(t, "on pc-c, the pet's token onto (1,0)", gil.move("tok-pet", 1, 0), qaMGUnseen)
	qaMGWantRefused(t, "on pc-c, the pet's token onto the crate at (0,2)", gil.move("tok-pet", 0, 2), qaMGUnseen)
}
