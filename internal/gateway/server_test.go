package gateway_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// --- fixture -----------------------------------------------------------

// gwFixture is a real gateway.Server wired to a real campaign+identity DB
// on a real httptest server, seeded with a small history: SessionStarted,
// SceneCreated "scn1", ActorAdded "a1" (controlled by the player), and
// TokenPlaced "t1" at (3,7) on scn1. Sequences 1-4.
type gwFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken, agentToken, spectatorToken string
	playerToken, otherPlayerToken       string
	playerID                            string
}

// mustAppend seeds one event directly on the campaign (bypassing the
// gateway) for fixture setup. payload must be one of the Envelope_* oneof
// wrapper types (e.g. *vttv1.Envelope_SceneCreated) — accepted as `any`
// since the oneof marker interface (isEnvelope_Payload) is unexported in
// package vttv1 and so cannot be satisfied by any type outside it.
func mustAppend(t *testing.T, c *campaign.Campaign, id string, payload any) int64 {
	t.Helper()
	env := &vttv1.Envelope{EventId: id}
	switch p := payload.(type) {
	case *vttv1.Envelope_SessionStarted:
		env.Payload = p
	case *vttv1.Envelope_SceneCreated:
		env.Payload = p
	case *vttv1.Envelope_ActorAdded:
		env.Payload = p
	case *vttv1.Envelope_TokenPlaced:
		env.Payload = p
	default:
		t.Fatalf("mustAppend: unsupported payload type %T", payload)
	}
	seq, err := c.Append(env)
	if err != nil {
		t.Fatalf("seed append %s: %v", id, err)
	}
	return seq
}

func newGWFixture(t *testing.T) *gwFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	ids, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	agentToken, _, err := ids.CreateInvite("Agent", identity.RoleAgent, nil)
	if err != nil {
		t.Fatal(err)
	}
	spectatorToken, _, err := ids.CreateInvite("Watcher", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}
	playerToken, playerID, err := ids.CreateInvite("Lera", identity.RolePlayer, []string{"a1"})
	if err != nil {
		t.Fatal(err)
	}
	otherPlayerToken, _, err := ids.CreateInvite("Ivo", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}

	mustAppend(t, c, "seed-1", &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s1"}})
	mustAppend(t, c, "seed-2", &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "scn1", Name: "Cave", GridWidth: 10, GridHeight: 10,
	}})
	mustAppend(t, c, "seed-3", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ControllerId: playerID},
	}})
	mustAppend(t, c, "seed-4", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn1", ActorId: "a1", Position: &vttv1.GridPosition{X: 3, Y: 7},
	}})

	srv := gateway.New(c, ids)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &gwFixture{
		t: t, srv: httpSrv,
		dmToken: dmToken, agentToken: agentToken, spectatorToken: spectatorToken,
		playerToken: playerToken, otherPlayerToken: otherPlayerToken, playerID: playerID,
	}
}

// wsURL builds this fixture's /ws URL for the given token and after cursor.
func (f *gwFixture) wsURL(token string, after int64) string {
	u, err := url.Parse(f.srv.URL)
	if err != nil {
		f.t.Fatal(err)
	}
	u.Path = "/ws"
	q := u.Query()
	q.Set("token", token)
	q.Set("after", strconv.FormatInt(after, 10))
	u.RawQuery = q.Encode()
	return u.String()
}

// dial connects with token/after, failing the test on error.
func (f *gwFixture) dial(token string, after int64) *websocket.Conn {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, f.wsURL(token, after), nil)
	if err != nil {
		f.t.Fatalf("dial: %v", err)
	}
	f.t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// --- read/write helpers -------------------------------------------------

func readFrame(t *testing.T, conn *websocket.Conn) *vttv1.ServerFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var frame vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("unmarshal frame: %v (raw=%s)", err, raw)
	}
	return &frame
}

// readResult reads frames until it sees a CommandResult (skipping any
// Envelope frames that race ahead of it — the writer interleaves the pump
// and the command loop, so a broadcast can legitimately arrive first).
func readResult(t *testing.T, conn *websocket.Conn) *vttv1.CommandResult {
	t.Helper()
	for i := 0; i < 10; i++ {
		f := readFrame(t, conn)
		if r := f.GetResult(); r != nil {
			return r
		}
	}
	t.Fatal("readResult: no CommandResult within 10 frames")
	return nil
}

// readEvent reads frames until it sees an Envelope (skipping CommandResult
// frames from this same connection's own in-flight commands).
func readEvent(t *testing.T, conn *websocket.Conn) *vttv1.Envelope {
	t.Helper()
	for i := 0; i < 10; i++ {
		f := readFrame(t, conn)
		if e := f.GetEvent(); e != nil {
			return e
		}
	}
	t.Fatal("readEvent: no Envelope within 10 frames")
	return nil
}

func sendCommand(t *testing.T, conn *websocket.Conn, cmd *vttv1.ClientCommand) {
	t.Helper()
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
}

func assertNoFrameWithin(t *testing.T, conn *websocket.Conn, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	_, raw, err := conn.Read(ctx)
	if err == nil {
		t.Fatalf("want no frame within %s, got %s", d, raw)
	}
}

// --- tests ---------------------------------------------------------------

func TestHealthzOK(t *testing.T) {
	f := newGWFixture(t)
	resp, err := http.Get(f.srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestConnectBadTokenRejectedBeforeUpgrade covers the binding design: a
// bad/revoked token gets a plain HTTP 401 and the socket is never upgraded
// (verify runs before websocket.Accept).
func TestConnectBadTokenRejectedBeforeUpgrade(t *testing.T) {
	f := newGWFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, f.wsURL("not-a-real-token", 0), nil)
	if err == nil {
		t.Fatal("want error connecting with a bad token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

// TestConnectRevokedTokenRejectedBeforeUpgrade is the revoked half of the
// same case: a token valid at mint time but revoked before connecting.
func TestConnectRevokedTokenRejectedBeforeUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ids, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()

	token, id, err := ids.CreateInvite("Lera", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ids.Revoke(id); err != nil {
		t.Fatal(err)
	}

	srv := gateway.New(c, ids)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	u, _ := url.Parse(httpSrv.URL)
	u.Path = "/ws"
	q := u.Query()
	q.Set("token", token)
	q.Set("after", "0")
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, u.String(), nil)
	if err == nil {
		t.Fatal("want error connecting with a revoked token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

// TestConnectAfterZeroReceivesFullHistoryThenLive covers catch-up: a fresh
// connection with after=0 sees all 4 seeded events, in sequence order,
// before any new live event.
func TestConnectAfterZeroReceivesFullHistoryThenLive(t *testing.T) {
	f := newGWFixture(t)
	conn := f.dial(f.dmToken, 0)

	wantIDs := []string{"seed-1", "seed-2", "seed-3", "seed-4"}
	for i, want := range wantIDs {
		env := readEvent(t, conn)
		if env.EventId != want || env.Sequence != int64(i+1) {
			t.Fatalf("history[%d] = (%s, seq %d), want (%s, seq %d)", i, env.EventId, env.Sequence, want, i+1)
		}
	}

	// Now a live event: DM starts a new command after catch-up.
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "r-live",
		Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}},
	})
	live := readEvent(t, conn)
	if live.Sequence != 5 {
		t.Fatalf("live event sequence = %d, want 5", live.Sequence)
	}
	if _, ok := live.Payload.(*vttv1.Envelope_SessionEnded); !ok {
		t.Fatalf("live payload = %T, want SessionEnded", live.Payload)
	}
}

// TestTwoClientsBothReceiveAcceptedCommandAsEvent is the fan-out case: a
// DM-issued command is broadcast to every other connected client as an
// Envelope frame, not just returned as a CommandResult to the issuer.
func TestTwoClientsBothReceiveAcceptedCommandAsEvent(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, 4)
	watcherConn := f.dial(f.spectatorToken, 4)

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-1",
		Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
			SceneId: "scn2", Name: "Dungeon", GridWidth: 5, GridHeight: 5,
		}},
	})

	result := readResult(t, dmConn)
	if !result.Ok || result.RequestId != "r-1" {
		t.Fatalf("dm result = %+v, want ok=true request_id=r-1", result)
	}

	dmEvent := readEvent(t, dmConn)
	watcherEvent := readEvent(t, watcherConn)
	for name, env := range map[string]*vttv1.Envelope{"dm": dmEvent, "watcher": watcherEvent} {
		sc, ok := env.Payload.(*vttv1.Envelope_SceneCreated)
		if !ok {
			t.Fatalf("%s payload = %T, want SceneCreated", name, env.Payload)
		}
		if sc.SceneCreated.SceneId != "scn2" {
			t.Fatalf("%s scene id = %q, want scn2", name, sc.SceneCreated.SceneId)
		}
	}

	// TestSequenceInResultMatchesBroadcastEnvelope, folded in here since it
	// needs this exact pair of frames: the sequence CommandResult reports
	// must equal the broadcast Envelope's own sequence.
	if result.Sequence != dmEvent.Sequence {
		t.Fatalf("result.Sequence = %d, dmEvent.Sequence = %d, want equal", result.Sequence, dmEvent.Sequence)
	}
}

// TestPlayerOwnershipDenialNoBroadcast covers the authz-failure contract: a
// player moving a token they don't control gets an ok=false Result on their
// own connection, the connection stays open (proven by a follow-up command
// succeeding on it), and NO Envelope is broadcast to anyone.
func TestPlayerOwnershipDenialNoBroadcast(t *testing.T) {
	f := newGWFixture(t)

	// Second actor/token controlled by nobody the player controls.
	afterSeq := f.dmSeedOtherToken()

	playerConn := f.dial(f.playerToken, afterSeq)
	monitorConn := f.dial(f.dmToken, afterSeq)

	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "r-deny",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "t2", To: &vttv1.GridPosition{X: 1, Y: 1},
		}},
	})
	result := readResult(t, playerConn)
	if result.Ok {
		t.Fatal("want ok=false denying a token the player does not control")
	}
	if result.Error == "" {
		t.Fatal("want non-empty error on denial")
	}

	// No event broadcast for the denied command.
	assertNoFrameWithin(t, monitorConn, 300*time.Millisecond)

	// Connection stays open: the SAME connection can still issue a command
	// the player IS authorized for (moving their own token).
	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "r-ok",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "t1", To: &vttv1.GridPosition{X: 4, Y: 4},
		}},
	})
	okResult := readResult(t, playerConn)
	if !okResult.Ok {
		t.Fatalf("want ok=true moving own token after a denial, got error %q", okResult.Error)
	}
}

// dmSeedOtherToken appends ActorAdded "a2" (controlled by nobody the
// fixture's player controls) + TokenPlaced "t2", and returns the sequence
// after which fresh connections should subscribe.
func (f *gwFixture) dmSeedOtherToken() int64 {
	f.t.Helper()
	// Issue the seed as a DM command over the wire (rather than reopening
	// the campaign file directly) so it goes through the same Append path
	// as everything else and keeps the fixture to one DB handle.
	dmConn := f.dial(f.dmToken, 4)
	sendCommand(f.t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-a2",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "a2", Name: "Villain", ControllerId: "someone-else"},
		}},
	})
	r1 := readResult(f.t, dmConn)
	if !r1.Ok {
		f.t.Fatalf("seed AddActor a2: %s", r1.Error)
	}
	sendCommand(f.t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-t2",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "t2", SceneId: "scn1", ActorId: "a2", Position: &vttv1.GridPosition{X: 0, Y: 0},
		}},
	})
	r2 := readResult(f.t, dmConn)
	if !r2.Ok {
		f.t.Fatalf("seed PlaceToken t2: %s", r2.Error)
	}
	return r2.Sequence
}

// TestSpectatorCommandDenied covers the default-deny role: a spectator can
// connect and receive the stream, but issuing ANY command comes back
// ok=false, and the connection stays open.
func TestSpectatorCommandDenied(t *testing.T) {
	f := newGWFixture(t)
	conn := f.dial(f.spectatorToken, 4)

	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "r-spec",
		Command:   &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}},
	})
	result := readResult(t, conn)
	if result.Ok {
		t.Fatal("want ok=false for a spectator command")
	}
	if result.Error == "" {
		t.Fatal("want non-empty error")
	}
}

// TestAgentRetractEventsBroadcastToAll covers the retraction path end to
// end: an agent's RetractEvents command succeeds (ok=true) and the
// EventsRetracted marker is broadcast to every connected client, this one
// included.
func TestAgentRetractEventsBroadcastToAll(t *testing.T) {
	f := newGWFixture(t)
	agentConn := f.dial(f.agentToken, 4)
	watcherConn := f.dial(f.spectatorToken, 4)

	sendCommand(t, agentConn, &vttv1.ClientCommand{
		RequestId: "r-undo",
		Command: &vttv1.ClientCommand_RetractEvents{RetractEvents: &vttv1.RetractEvents{
			FromSequence: 4, ToSequence: 4, Reason: "test retraction",
		}},
	})
	result := readResult(t, agentConn)
	if !result.Ok {
		t.Fatalf("want ok=true for agent retraction, got error %q", result.Error)
	}

	agentEvent := readEvent(t, agentConn)
	watcherEvent := readEvent(t, watcherConn)
	for name, env := range map[string]*vttv1.Envelope{"agent": agentEvent, "watcher": watcherEvent} {
		if _, ok := env.Payload.(*vttv1.Envelope_EventsRetracted); !ok {
			t.Fatalf("%s payload = %T, want EventsRetracted", name, env.Payload)
		}
	}
}

// TestMalformedFrameClosesOnlyThatConnection covers isolation: a
// syntactically invalid frame closes the sender's own connection, but a
// second, unrelated connection stays fully live.
func TestMalformedFrameClosesOnlyThatConnection(t *testing.T) {
	f := newGWFixture(t)
	badConn := f.dial(f.dmToken, 4)
	otherConn := f.dial(f.dmToken, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := badConn.Write(ctx, websocket.MessageText, []byte("{not valid json")); err != nil {
		t.Fatal(err)
	}

	// badConn must be closed by the server.
	readCtx, readCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer readCancel()
	if _, _, err := badConn.Read(readCtx); err == nil {
		t.Fatal("want badConn closed after sending a malformed frame")
	}

	// otherConn is untouched: it can still issue a command and see it
	// broadcast.
	sendCommand(t, otherConn, &vttv1.ClientCommand{
		RequestId: "r-alive",
		Command:   &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}},
	})
	result := readResult(t, otherConn)
	if !result.Ok {
		t.Fatalf("want ok=true on the surviving connection, got error %q", result.Error)
	}
}

// TestMoveTokenBroadcastBackfillsSceneAndFrom is the controller decision's
// dedicated test: the broadcast TokenMoved carries SceneId and From
// backfilled from the token's state BEFORE the move, not just the
// destination the client sent.
func TestMoveTokenBroadcastBackfillsSceneAndFrom(t *testing.T) {
	f := newGWFixture(t)
	playerConn := f.dial(f.playerToken, 4)

	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "r-move",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "t1", To: &vttv1.GridPosition{X: 9, Y: 9},
		}},
	})
	result := readResult(t, playerConn)
	if !result.Ok {
		t.Fatalf("want ok=true moving own token, got error %q", result.Error)
	}

	env := readEvent(t, playerConn)
	tm, ok := env.Payload.(*vttv1.Envelope_TokenMoved)
	if !ok {
		t.Fatalf("payload = %T, want TokenMoved", env.Payload)
	}
	if tm.TokenMoved.SceneId != "scn1" {
		t.Fatalf("SceneId = %q, want scn1 (backfilled)", tm.TokenMoved.SceneId)
	}
	if tm.TokenMoved.From == nil || tm.TokenMoved.From.X != 3 || tm.TokenMoved.From.Y != 7 {
		t.Fatalf("From = %+v, want (3,7) (backfilled, the token's pre-move position)", tm.TokenMoved.From)
	}
	if tm.TokenMoved.To == nil || tm.TokenMoved.To.X != 9 || tm.TokenMoved.To.Y != 9 {
		t.Fatalf("To = %+v, want (9,9)", tm.TokenMoved.To)
	}
}
