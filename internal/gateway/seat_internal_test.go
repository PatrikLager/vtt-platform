package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestARapidHopIsCoalescedToTheShoulderItEndedOn pins perchBox's latest-wins
// slot (SPEC-015). Keep the burst ending on a shoulder it did not start on, and
// on the empty one: a first-wins box and a box that reads "" as empty pass
// anything else.
// VTT-188
func TestARapidHopIsCoalescedToTheShoulderItEndedOn(t *testing.T) {
	b := newPerchBox()
	b.set("hero")
	b.set("goblin")
	b.set("hero")
	b.set("") // and off again, between shoulders

	got, ok := b.take()
	if !ok || got != "" {
		t.Fatalf("the pump must see the shoulder the spectator ended on — here, none at all: "+
			"got %q (ok=%v)", got, ok)
	}
	// And the slot is empty afterwards: a second wake-up with nothing new in it
	// must not re-apply the last shoulder.
	if _, ok := b.take(); ok {
		t.Error("taking twice must not hand the same shoulder out again")
	}
}

// TestAPerchSetWhileNothingWaitsStillWakesThePump holds wake's capacity
// (SPEC-015). Receive from wake without blocking: a blocking receive hangs
// where this must fail.
// VTT-306
func TestAPerchSetWhileNothingWaitsStillWakesThePump(t *testing.T) {
	b := newPerchBox()
	b.set("hero")

	select {
	case <-b.wake:
	default:
		t.Fatal("a perch set while nothing waited on wake left no wake-up for the pump")
	}
	if got, ok := b.take(); !ok || got != "hero" {
		t.Fatalf("the wake-up must hand the pump the shoulder set, got %q (ok=%v)", got, ok)
	}
}

func seatAppend(t *testing.T, c *campaign.Campaign, id string, payload any) {
	t.Helper()
	env := &vttv1.Envelope{EventId: id}
	switch p := payload.(type) {
	case *vttv1.SceneCreated:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.ActorControlGranted:
		env.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: p}
	case *vttv1.ActorControlRevoked:
		env.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: p}
	case *vttv1.TokenPlaced:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.NarrationAdded:
		env.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: p}
	default:
		t.Fatalf("seatAppend: no arm for %T", payload)
	}
	if _, err := c.Append(env); err != nil {
		t.Fatalf("append %s: %v", id, err)
	}
}

func seatParty(t *testing.T, c *campaign.Campaign, actorID, participantID string) {
	t.Helper()
	seatAppend(t, c, actorID+"-added", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: actorID, Name: actorID,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	seatAppend(t, c, actorID+"-granted", &vttv1.ActorControlGranted{ActorId: actorID, ParticipantId: participantID,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
}

func seatTake(t *testing.T, events <-chan *vttv1.Envelope, what string) *vttv1.Envelope {
	t.Helper()
	select {
	case env, ok := <-events:
		if !ok {
			t.Fatalf("%s: the subscription closed", what)
		}
		return env
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: no entry within 3s", what)
	}
	return nil
}

// VTT-348
func TestNoEntryCausedByTheLossIsSent(t *testing.T) {
	c, err := campaign.Open(filepath.Join(t.TempDir(), "campaign"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	seatAppend(t, c, "hall", &vttv1.SceneCreated{SceneId: "s", Name: "Hall", GridWidth: 4, GridHeight: 4})
	seatParty(t, c, "a1", "p-1")
	seatAppend(t, c, "t1", &vttv1.TokenPlaced{TokenId: "t1", SceneId: "s", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 1, Y: 1}})

	s, head, err := openSeat(c, &identity.Participant{ID: "p-1", Role: identity.RolePlayer}, "", 0, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	for i := int64(0); i < head; i++ {
		seatTake(t, s.events, "catch-up")
	}

	seatAppend(t, c, "still", &vttv1.NarrationAdded{Text: "still watched"})
	seatTake(t, s.events, "the narration")
	if send, moved, err := s.admit(); !send || moved || err != nil {
		t.Fatalf("an entry of a log the seat may still watch: send=%v moved=%v err=%v", send, moved, err)
	}

	seatAppend(t, c, "lost", &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: "p-1"})
	if env := seatTake(t, s.events, "the revoke's entry"); env.GetEventId() != "lost" {
		t.Fatalf("fixture: the revoke's entry is %v", env)
	}
	if send, moved, err := s.admit(); send || !moved || err != nil || s.shown != "" {
		t.Fatalf("the loss's own entry: send=%v moved=%v err=%v shown=%q, want it dropped and the seat on nobody",
			send, moved, err, s.shown)
	}
}

// VTT-089
func TestAWedgedWatcherIsTornDownAndOthersKeepServing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ids, err := identity.Open(campaign.LogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	driverToken, _, err := ids.CreateInvite("Driver", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	victimToken, victimID, err := ids.CreateInvite("Victim", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	healthyToken, _, err := ids.CreateInvite("Healthy", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}
	seatParty(t, c, "hero", victimID)

	newServer := func(buffer int, noProgress time.Duration) *httptest.Server {
		srv := New(c, ids)
		srv.buffer, srv.noProgress = buffer, noProgress
		srv.pingInterval, srv.pingTimeout = 10*time.Minute, 10*time.Minute
		hs := httptest.NewUnstartedServer(srv.Handler())
		hs.Listener = &sndbufListener{Listener: hs.Listener}
		hs.Start()
		return hs
	}
	main := newServer(gatewayBuffer, 10*time.Minute)
	defer main.Close()
	wedged := newServer(0, 5*time.Millisecond)
	defer wedged.Close()

	dial := func(base, token string, client *http.Client) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var opts *websocket.DialOptions
		if client != nil {
			opts = &websocket.DialOptions{HTTPClient: client}
		}
		conn, _, err := websocket.Dial(ctx, base+"/ws?token="+token+"&after=0", opts)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conn.SetReadLimit(200 * 1024)
		return conn
	}
	driver := dial(main.URL, driverToken, nil)
	defer driver.CloseNow()
	healthy := dial(main.URL, healthyToken, nil)
	defer healthy.CloseNow()
	victim := dial(wedged.URL, victimToken, tinyRecvBufClient())
	defer victim.CloseNow()

	readAll := func(conn *websocket.Conn) chan struct{} {
		gone := make(chan struct{})
		go func() {
			defer close(gone)
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				_, _, err := conn.Read(ctx)
				cancel()
				if err != nil {
					return
				}
			}
		}()
		return gone
	}
	driverGone, healthyGone := readAll(driver), readAll(healthy)

	// Introduce party members the victim's hero knows of, each too big for the
	// shrunken socket buffers, and never read the victim.
	for i := range 40 {
		cmd := &vttv1.ClientCommand{RequestId: strconv.Itoa(i), Command: &vttv1.ClientCommand_AddActor{
			AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: "act-" + strconv.Itoa(i),
				ModuleData: bigPadding(t), Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}}
		raw, err := protojson.Marshal(cmd)
		if err != nil {
			t.Fatal(err)
		}
		wctx, wcancel := context.WithTimeout(context.Background(), 30*time.Second)
		werr := driver.Write(wctx, websocket.MessageText, raw)
		wcancel()
		if werr != nil {
			t.Fatalf("driver write %d: %v", i, werr)
		}
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	closed := false
	for range 50 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _, err := victim.Read(ctx)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("the wedged watcher was not closed: a read timed out instead")
			}
			closed = true
			break
		}
	}
	if !closed {
		t.Fatal("the wedged watcher was never closed")
	}
	select {
	case <-driverGone:
		t.Fatal("the driver's connection ended with the watcher's")
	case <-healthyGone:
		t.Fatal("the healthy connection ended with the watcher's")
	default:
	}
}

// pumpUnderTest runs s.pump for p over a real socket, its frames on out.
func pumpUnderTest(t *testing.T, s *Server, p *identity.Participant) (*perchBox, chan []byte) {
	t.Helper()
	conns := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conns <- conn
		<-release
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(ctx, hs.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	sub, _, err := openSeat(s.campaign, p, "", 0, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	perches, out, stop, done := newPerchBox(), make(chan []byte, 64), make(chan struct{}), make(chan struct{})
	writerDone := make(chan struct{})
	var closing atomic.Bool
	go func() {
		defer close(done)
		defer sub.close()
		s.pump(sub, p, perches, stop, out, writerDone, <-conns, &closing)
	}()
	t.Cleanup(func() {
		closing.Store(true)
		close(stop)
		close(writerDone)
		<-done
		close(release)
		client.CloseNow()
		hs.Close()
	})
	return perches, out
}

func frameOf(t *testing.T, out chan []byte) *vttv1.ServerFrame {
	t.Helper()
	select {
	case b := <-out:
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		return &f
	case <-time.After(3 * time.Second):
		t.Fatal("no frame within 3s")
	}
	return nil
}

func seatServer(t *testing.T) (*Server, *campaign.Campaign, *identity.Participant) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(campaign.LogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })
	_, playerID, err := ids.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	return New(c, ids), c, &identity.Participant{ID: playerID, Role: identity.RolePlayer}
}

// VTT-343
func TestAPerchTheActorsNoLongerAllowIsNotApplied(t *testing.T) {
	s, c, p := seatServer(t)
	seatParty(t, c, "a1", p.ID)
	seatParty(t, c, "b1", "someone-else")

	perches, out := pumpUnderTest(t, s, p)
	if vc := frameOf(t, out).GetViewpointChanged(); vc == nil || vc.GetActorId() != "a1" {
		t.Fatalf("the seat opened on %v, want a1", vc)
	}
	held, err := c.CharacterLog("a1")
	if err != nil {
		t.Fatal(err)
	}
	for range held {
		frameOf(t, out)
	}

	// As if Authorize had judged a snapshot in which b1 was still the player's.
	// Append only once the pump has taken the slot.
	perches.set("b1")
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(time.Millisecond) {
		perches.mu.Lock()
		taken := !perches.full
		perches.mu.Unlock()
		if taken {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the pump never took the perch")
		}
	}
	seatAppend(t, c, "after", &vttv1.NarrationAdded{Text: "after the perch"})
	f := frameOf(t, out)
	if f.GetViewpointChanged() != nil || f.GetEvent().GetNarrationAdded().GetText() != "after the perch" {
		t.Fatalf("a perch onto another's party member was applied: got %v", f)
	}
}

func seatRooms(t *testing.T, c *campaign.Campaign) {
	t.Helper()
	tiles := map[string]*vttv1.TileRef{}
	for y := range int32(3) {
		for x := range int32(7) {
			kind := "floor"
			if x == 3 {
				kind = "wall"
			}
			tiles[strconv.Itoa(int(x))+","+strconv.Itoa(int(y))] = &vttv1.TileRef{Kind: kind}
		}
	}
	seatAppend(t, c, "rooms", &vttv1.SceneCreated{SceneId: "rooms", Name: "Rooms", GridWidth: 7, GridHeight: 3,
		Tiles: tiles})
	for id, x := range map[string]int32{"ana": 1, "bo": 5} {
		seatAppend(t, c, id+"-actor", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id, Name: id,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
		seatAppend(t, c, id+"-token", &vttv1.TokenPlaced{TokenId: "tok-" + id, SceneId: "rooms", ActorId: id,
			Position: &vttv1.GridPosition{X: x, Y: 1}})
	}
}

func seatGrant(t *testing.T, c *campaign.Campaign, actorID, participantID string) {
	t.Helper()
	seatAppend(t, c, actorID+"-granted", &vttv1.ActorControlGranted{ActorId: actorID, ParticipantId: participantID,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
}

func seatMove(tokenID string, x int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "move", Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: tokenID, To: &vttv1.GridPosition{X: x, Y: 1}}}}
}

// Start no pump here: the seat must not have read the grant's wake.
// VTT-351
func TestAMoveRightAfterAGrantIsJudgedByTheNewCharacter(t *testing.T) {
	s, c, p := seatServer(t)
	seatRooms(t, c)
	sub, _, err := openSeat(c, p, "", 0, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.close)

	seatGrant(t, c, "ana", p.ID)
	if sub.shown != "" {
		t.Fatalf("fixture: the seat shows %q, want nobody", sub.shown)
	}
	if r := s.handleCommand(p, seatMove("tok-ana", 2), sub); !r.GetOk() {
		t.Fatalf("a move of the granted character's token onto a square it sees: %s", r.GetError())
	}
}

// Start no pump here: the seat must still show ana when the move is judged.
// VTT-351
func TestAMoveRightAfterARevocationIsNotJudgedByTheLostCharacter(t *testing.T) {
	s, c, p := seatServer(t)
	seatRooms(t, c)
	seatGrant(t, c, "ana", p.ID)
	seatGrant(t, c, "bo", p.ID)
	sub, _, err := openSeat(c, p, "", 0, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.close)

	seatAppend(t, c, "lost", &vttv1.ActorControlRevoked{ActorId: "ana", ParticipantId: p.ID})
	if sub.shown != "ana" {
		t.Fatalf("fixture: the seat shows %q, want ana", sub.shown)
	}
	const unseen = "gateway: cannot move there — you cannot see that square"
	if r := s.handleCommand(p, seatMove("tok-bo", 2), sub); r.GetOk() || r.GetError() != unseen {
		t.Fatalf("bo's move onto a square only ana sees: ok=%v %q, want %q", r.GetOk(), r.GetError(), unseen)
	}
}

// Start no pump here: the move must be judged before any frame is sent.
// VTT-351
func TestAMoveOnAConnectionOpenedOnALogIsJudgedByThatLog(t *testing.T) {
	s, c, p := seatServer(t)
	seatRooms(t, c)
	seatGrant(t, c, "ana", p.ID)
	seatGrant(t, c, "bo", p.ID)
	sub, _, err := openSeat(c, p, "bo", 0, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.close)

	if r := s.handleCommand(p, seatMove("tok-bo", 4), sub); !r.GetOk() {
		t.Fatalf("opened on bo, a move onto a square bo sees and ana, the default, does not: %s", r.GetError())
	}
}
