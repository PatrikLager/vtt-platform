package gateway_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
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

	// ids is exposed for the same reason campaign is: promotion changes
	// IDENTITY, not campaign state, so the only place its effect is visible is
	// the participants table.
	ids *identity.DB

	// path is the campaign file. A test that needs to break identity
	// OPERATIONALLY (rather than revoke somebody, which is a credential fact)
	// reaches the table through here.
	path string

	// campaign is exposed so a test can assert on FOLDED STATE rather than on
	// the result frame alone. A command can answer ok=true and still not mean
	// what the test claims; the state is what the table actually sees.
	campaign *campaign.Campaign
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
		campaign: c,
		ids:      ids,
		path:     path,
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

// --- per-connection frame demultiplexing ---------------------------------
//
// The gateway does NOT order a command's CommandResult relative to the
// Envelopes that command produced. Results are enqueued by the command loop
// and events by the broadcast pump: two independent producers feeding one
// writer goroutine (see serve's "Writer choice" comment in server.go, which
// says so outright -- "the ordering of interleaved results/events is whatever
// order they arrive at outCh"). Either can win the race.
//
// The previous helpers read POSITIONALLY, skipping up to 10 frames of the
// wrong kind and DISCARDING them. That produced two failure modes, both seen
// in CI and both misfiled as a resource-contention flake:
//
//   - a batch bigger than the budget failed outright, "no CommandResult
//     within 10 frames" -- an adventure load emits well over ten events;
//   - a single inversion desynchronised the connection PERMANENTLY, because
//     the discarded frame was the one a later read wanted, so that read
//     blocked until the 20s deadline.
//
// Load never caused either; it only changes how often the race flips. A
// captured frame log shows the inversion plainly: five commands arriving
// RESULT-then-EVENT, then one arriving EVENT-then-RESULT.
//
// frameQueue owns the ONLY reader for a connection and sorts frames by kind
// into separate queues, so asking for a result never consumes an event, and
// no read depends on arrival order. This is exactly what harness.Client
// already does in production -- SendCommand returns the result, Events() is a
// separate channel -- so these tests were the outlier, not the server.
type frameQueue struct {
	results chan *vttv1.CommandResult
	events  chan *vttv1.Envelope
	closed  chan struct{}
	err     error // written before closed is closed; read only after
}

func newFrameQueue(conn *websocket.Conn) *frameQueue {
	q := &frameQueue{
		results: make(chan *vttv1.CommandResult, 256),
		events:  make(chan *vttv1.Envelope, 256),
		closed:  make(chan struct{}),
	}
	go func() {
		defer close(q.closed)
		for {
			// context.Background(), never a deadline. coder/websocket's
			// setupReadTimeout registers an AfterFunc that closes the WHOLE
			// socket when a Read's context expires, so a bounded Read is
			// destructive by design in this library. Reading unbounded here
			// and bounding the WAIT with a channel select avoids that path
			// entirely (Background's Done() is nil, so the AfterFunc is never
			// registered).
			_, raw, err := conn.Read(context.Background())
			if err != nil {
				q.err = err
				return
			}
			var f vttv1.ServerFrame
			if err := protojson.Unmarshal(raw, &f); err != nil {
				q.err = fmt.Errorf("unmarshal frame: %w (raw=%s)", err, raw)
				return
			}
			if r := f.GetResult(); r != nil {
				q.results <- r
			} else if e := f.GetEvent(); e != nil {
				q.events <- e
			}
		}
	}()
	return q
}

var (
	frameQueuesMu sync.Mutex
	frameQueues   = map[*websocket.Conn]*frameQueue{}
)

// queueFor attaches the demultiplexer on first use.
//
// LAZILY, and that matters: TestOverflowForcesSocketClosedAndOthersKeepServing
// depends on a connection nobody reads, so that the server-side subscribe
// buffer overflows and the server force-closes the socket. Attaching a reader
// at dial would drain that connection and the overflow would never happen.
func queueFor(conn *websocket.Conn) *frameQueue {
	frameQueuesMu.Lock()
	defer frameQueuesMu.Unlock()
	q, ok := frameQueues[conn]
	if !ok {
		q = newFrameQueue(conn)
		frameQueues[conn] = q
	}
	return q
}

const frameWait = 3 * time.Second

// readResult returns this connection's next CommandResult. Events that arrive
// first are queued, not discarded, so a later readEvent still sees them.
func readResult(t *testing.T, conn *websocket.Conn) *vttv1.CommandResult {
	t.Helper()
	q := queueFor(conn)
	select {
	case r := <-q.results:
		return r
	case <-q.closed:
		t.Fatalf("readResult: connection closed: %v", q.err)
	case <-time.After(frameWait):
		t.Fatalf("readResult: no CommandResult within %s", frameWait)
	}
	return nil
}

// readEvent returns this connection's next Envelope. Results that arrive
// first are queued, not discarded.
func readEvent(t *testing.T, conn *websocket.Conn) *vttv1.Envelope {
	t.Helper()
	q := queueFor(conn)
	select {
	case e := <-q.events:
		return e
	case <-q.closed:
		t.Fatalf("readEvent: connection closed: %v", q.err)
	case <-time.After(frameWait):
		t.Fatalf("readEvent: no Envelope within %s", frameWait)
	}
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

// assertNoFrameWithin fails if ANY frame arrives within d. Routed through the
// queue like every other read, so it no longer destroys the connection on the
// expected path -- coder/websocket closes the socket when a Read's context
// expires, which made the old version single-use.
func assertNoFrameWithin(t *testing.T, conn *websocket.Conn, d time.Duration) {
	t.Helper()
	q := queueFor(conn)
	select {
	case r := <-q.results:
		t.Fatalf("want no frame within %s, got result %s", d, r.GetRequestId())
	case e := <-q.events:
		t.Fatalf("want no frame within %s, got event seq=%d", d, e.GetSequence())
	case <-q.closed:
		t.Fatalf("want no frame within %s, connection closed: %v", d, q.err)
	case <-time.After(d):
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
		Command:   &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}},
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
//
// P6 Task 4 pre-step (ADR-009 binding, controller decision): the result's
// Sequence must equal the broadcast marker Envelope's Sequence, closing the
// P4 carry-forward ("RetractEvents' result carries no sequence" — spec §3
// EXCEPTION note). Before campaign.Undo returned the marker's sequence,
// result.Sequence was always left 0 here (handleRetraction's zero-value
// CommandResult literal), so this assertion is genuine behavioral RED
// against the pre-fix code — not a hypothetical.
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
	if agentEvent.Sequence != watcherEvent.Sequence {
		t.Fatalf("marker sequence mismatch across connections: agent=%d watcher=%d", agentEvent.Sequence, watcherEvent.Sequence)
	}
	if result.Sequence == 0 {
		t.Fatalf("result.Sequence = 0, want the marker's non-zero sequence (%d)", agentEvent.Sequence)
	}
	if result.Sequence != agentEvent.Sequence {
		t.Fatalf("result.Sequence = %d, want it to equal the broadcast marker's sequence %d", result.Sequence, agentEvent.Sequence)
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

	// Both connections open with their catch-up head and presence snapshot;
	// the close asserted below comes after badConn's handshake.
	expectCatchUpHead(t, badConn)
	expectPresenceSnapshot(t, badConn)

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

// TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned covers the world-
// layer (Task 3) precedent RemoveCondition already set: the gateway forwards
// add_narration/upsert_note/delete_note through the SAME single-Append path
// as every other command (server.go's handleCommand has no world-layer-
// specific branch), so a fold-level validation rejection — an absent
// delete_note key, or oversized narration text past the 8 KiB cap
// (internal/engine/apply.go's size posture) — must surface as an ordinary
// ok=false CommandResult and leave the connection (and the campaign) fully
// usable afterward, never poisoned (campaign.Append validates against a
// snapshot BEFORE persisting — see that method's doc comment).
func TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, 4)

	// A valid upsert first, so there is a real key to delete-after-recovery.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-upsert",
		Command: &vttv1.ClientCommand_UpsertNote{UpsertNote: &vttv1.UpsertNote{
			Key: "kobold-den", Title: "Kobold Den", Text: "Three kobolds guard the east tunnel.",
		}},
	})
	upsertResult := readResult(t, dmConn)
	if !upsertResult.Ok {
		t.Fatalf("want ok=true for a valid upsert_note, got error %q", upsertResult.Error)
	}

	// Rejection 1: delete_note for a key that was never upserted.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-delete-absent",
		Command:   &vttv1.ClientCommand_DeleteNote{DeleteNote: &vttv1.DeleteNote{Key: "no-such-note"}},
	})
	deleteAbsentResult := readResult(t, dmConn)
	if deleteAbsentResult.Ok {
		t.Fatal("want ok=false deleting a note key that was never upserted")
	}
	if deleteAbsentResult.Error == "" {
		t.Fatal("want non-empty error on the absent-key rejection")
	}

	// Rejection 2: add_narration with text past the 8 KiB size cap.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-narration-oversized",
		Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
			Text: strings.Repeat("x", 8193),
		}},
	})
	oversizedResult := readResult(t, dmConn)
	if oversizedResult.Ok {
		t.Fatal("want ok=false for narration text exceeding the 8 KiB cap")
	}
	if oversizedResult.Error == "" {
		t.Fatal("want non-empty error on the size-cap rejection")
	}

	// Recovery: the SAME connection can still issue a valid command
	// afterward — proof the two rejections above left the campaign
	// unpoisoned, exactly like TestPlayerOwnershipDenialNoBroadcast's own
	// recovery step proves for an authz denial.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-delete-ok",
		Command:   &vttv1.ClientCommand_DeleteNote{DeleteNote: &vttv1.DeleteNote{Key: "kobold-den"}},
	})
	recoveryResult := readResult(t, dmConn)
	if !recoveryResult.Ok {
		t.Fatalf("want ok=true deleting the real note after two rejections, got error %q (campaign appears poisoned)", recoveryResult.Error)
	}
}

// TestNarrationForwardAnchorRejectedCleanConnectionIntact closes the merge-
// gate MUST-FIX anchor-rejection wire seam: every existing wire test
// (TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned above,
// scenarios/story-table.json) exercises only the ACCEPTED anchor path or
// non-anchor rejections — no wire-level test ever sent an INVALID anchor
// and asserted the clean ok=false + unpoisoned-connection contract spec §6
// promises ("size-cap and anchor rejections surface as clean ok=false").
// This sits precisely on the provisional-stamp seam (campaign.go's Append)
// this branch changed: a regression there, or a gateway-side change that
// drops/zeroes/swaps the anchor fields in ToEvent before Append, would make
// an invalid anchor silently accepted while every other existing wire test
// stayed green.
func TestNarrationForwardAnchorRejectedCleanConnectionIntact(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, 4)

	// Forward anchor: anchorToSeq (999) is nowhere near before this
	// narration's own (about-to-be-assigned) sequence — engine.Apply
	// rejects any anchor_to_seq >= the narrating event's own sequence
	// (internal/engine/apply.go), pinning the exact error string this test
	// asserts on.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-narration-forward-anchor",
		Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
			Text: "a narration with a bad anchor", AnchorFromSeq: 1, AnchorToSeq: 999,
		}},
	})
	result := readResult(t, dmConn)
	if result.Ok {
		t.Fatal("want ok=false for a forward anchor (anchorToSeq far beyond any recorded sequence)")
	}
	if !strings.Contains(result.Error, "must be before") {
		t.Fatalf("want error containing %q, got %q", "must be before", result.Error)
	}

	// Connection intact, campaign unpoisoned: the SAME connection can still
	// issue a valid command afterward.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-narration-ok",
		Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
			Text: "a plain, unanchored narration",
		}},
	})
	recoveryResult := readResult(t, dmConn)
	if !recoveryResult.Ok {
		t.Fatalf("want ok=true for a valid narration after the forward-anchor rejection, got error %q (campaign appears poisoned)", recoveryResult.Error)
	}
}

// TestOversizedFrameClosesConnectionMaxLegalPayloadWorks covers the
// amendment-mandated fix (server.go's maxWSFrameBytes doc comment): the
// gateway's per-command websocket frame bound (32 KiB) is now an explicit,
// OWNED part of the size posture (handleWS's conn.SetReadLimit call) rather
// than silently inherited from coder/websocket's undocumented default read
// limit — no SetReadLimit call existed anywhere in internal/ or cmd/
// before this fix. Two directions: a frame one byte over the limit closes
// the connection with StatusMessageTooBig — pure transport-layer
// enforcement, before DecodeCommand ever sees the bytes, so any content
// triggers it, not just a well-formed oversized command — while a legal
// command whose encoded frame sits at EXACTLY the limit still round-trips
// cleanly (proving the explicit cap didn't shrink the effective bound
// below the library's own prior default).
func TestOversizedFrameClosesConnectionMaxLegalPayloadWorks(t *testing.T) {
	t.Run("frame one byte over the limit closes the connection", func(t *testing.T) {
		f := newGWFixture(t)
		conn := f.dial(f.dmToken, 4)

		// Consume the catch-up head BEFORE provoking the close, and not
		// after. Of the closes the READ LOOP can reach, this one is the
		// odd one out: coder/websocket writes it from inside conn.Read
		// itself (read.go's limitReader), so it never reaches shutdown(),
		// and shutdown() is what drains the writer goroutine. Worse than a
		// lost race — writeFrame LATCHES closeSentErr once a close goes
		// out, so every later write returns net.ErrClosed before it even
		// takes the write lock. The queued head is then never flushed at
		// all and the client sees only StatusMessageTooBig. Reading first
		// removes that ordering rather than hiding it: this connection
		// drives no broadcasts, so until it sends the oversized frame
		// there is nothing that can close it.
		//
		// Not a universal rule about the server. The malformed-frame case
		// above deliberately keeps the opposite order, because the server
		// owns that close and routes it through shutdown(), which drains
		// outCh first. And the pump's overflow force-close (server.go's
		// `if !closing.Load()` branch) bypasses shutdown() too, on a
		// connection that merely read too SLOWLY rather than misbehaving —
		// so a test that drives broadcasts cannot borrow this reasoning.
		expectCatchUpHead(t, conn)
		expectPresenceSnapshot(t, conn)

		writeCtx, writeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer writeCancel()
		oversized := []byte(strings.Repeat("x", 32769))
		if err := conn.Write(writeCtx, websocket.MessageText, oversized); err != nil {
			t.Fatalf("write oversized frame: %v", err)
		}

		readCtx, readCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer readCancel()
		if _, _, err := conn.Read(readCtx); err == nil {
			t.Fatal("want the connection to close after an oversized frame, got a clean read")
		} else if status := websocket.CloseStatus(err); status != websocket.StatusMessageTooBig {
			t.Fatalf("close status = %v, want StatusMessageTooBig (err=%v)", status, err)
		}
	})

	t.Run("a command frame at exactly the limit still works", func(t *testing.T) {
		f := newGWFixture(t)
		conn := f.dial(f.dmToken, 4)

		// Compute the request_id padding needed to land the encoded
		// ClientCommand frame at EXACTLY maxWSFrameBytes, rather than
		// hardcoding a byte count that would silently drift out of sync
		// with protojson's own field-encoding overhead.
		probe := &vttv1.ClientCommand{
			RequestId: "r",
			Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
				Text: "a legal narration riding a frame at exactly the read limit",
			}},
		}
		probeRaw, err := protojson.Marshal(probe)
		if err != nil {
			t.Fatal(err)
		}
		extraNeeded := 32768 - len(probeRaw)
		if extraNeeded < 0 {
			t.Fatalf("test setup: probe command already exceeds 32768 bytes (%d)", len(probeRaw))
		}
		probe.RequestId = strings.Repeat("r", 1+extraNeeded)
		raw, err := protojson.Marshal(probe)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) != 32768 {
			t.Fatalf("test setup: encoded frame is %d bytes, want exactly 32768", len(raw))
		}

		writeCtx, writeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer writeCancel()
		if err := conn.Write(writeCtx, websocket.MessageText, raw); err != nil {
			t.Fatalf("write max-legal-payload frame: %v", err)
		}

		result := readResult(t, conn)
		if !result.Ok {
			t.Fatalf("want ok=true for a command frame at exactly the 32768-byte read limit, got error %q", result.Error)
		}
	})
}

// expectCatchUpHead reads the ONE frame every connection now opens with and
// returns its head sequence, failing if the first frame is anything else.
//
// Tests that read the socket directly (rather than through frameQueue, which
// demultiplexes and drops what it was not asked for) must consume it before
// asserting on what follows. It doubles as the pin that the server really does
// announce the catch-up boundary FIRST — a client cannot use it to decide when
// catch-up ended if it arrives in the middle of the backlog.
func expectCatchUpHead(t *testing.T, conn *websocket.Conn) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read catch-up head frame: %v", err)
	}
	var f vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &f); err != nil {
		t.Fatalf("catch-up head frame did not decode: %v (raw=%s)", err, raw)
	}
	h := f.GetCatchUpHead()
	if h == nil {
		t.Fatalf("want CatchUpHead as the first frame, got %T (raw=%s)", f.GetFrame(), raw)
	}
	return h.GetHeadSequence()
}

// expectPresenceSnapshot consumes the PresenceSnapshot that every connection
// receives immediately after its catch-up head (spec §4), returning the
// participant ids it lists.
//
// It exists so tests that assert on what comes NEXT stay honest about the
// handshake rather than tolerating "some frame". Two tests began failing when
// presence was added precisely because they asserted the frame after the head
// was a CLOSE — that was a real change to the opening sequence, and the fix is
// to name the new frame, not to loosen the assertion.
func expectPresenceSnapshot(t *testing.T, conn *websocket.Conn) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read presence snapshot frame: %v", err)
	}
	var f vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &f); err != nil {
		t.Fatalf("presence snapshot frame did not decode: %v (raw=%s)", err, raw)
	}
	snap := f.GetPresenceSnapshot()
	if snap == nil {
		t.Fatalf("want PresenceSnapshot right after the catch-up head, got %T (raw=%s)", f.GetFrame(), raw)
	}
	ids := make([]string, 0, len(snap.GetPresent()))
	for _, e := range snap.GetPresent() {
		ids = append(ids, e.GetParticipantId())
	}
	return ids
}

// expectPresenceChanged reads frames until it sees a PresenceChanged for
// participantID, skipping anything else (events and results interleave freely
// on this channel). Fails if the deadline passes without one.
func expectPresenceChanged(t *testing.T, conn *websocket.Conn, participantID string, want vttv1.PresenceState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, raw, err := conn.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("waiting for presence %v of %q: %v", want, participantID, err)
		}
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &f); err != nil {
			t.Fatalf("frame did not decode: %v (raw=%s)", err, raw)
		}
		pc := f.GetPresenceChanged()
		if pc == nil || pc.GetParticipantId() != participantID {
			continue
		}
		if pc.GetState() != want {
			t.Fatalf("presence of %q = %v, want %v", participantID, pc.GetState(), want)
		}
		return
	}
	t.Fatalf("no PresenceChanged{%v} for %q before the deadline", want, participantID)
}

// TestPresenceAnnouncesAnArrival: the table learns when someone joins.
func TestPresenceAnnouncesAnArrival(t *testing.T) {
	f := newGWFixture(t)
	watcher := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, watcher)
	if ids := expectPresenceSnapshot(t, watcher); len(ids) != 1 {
		t.Fatalf("the first connection's snapshot should hold only itself, got %v", ids)
	}

	joiner := f.dial(f.playerToken, 0)
	defer joiner.CloseNow()
	expectPresenceChanged(t, watcher, f.playerID, vttv1.PresenceState_PRESENCE_STATE_CONNECTED)

	// And the joiner's own snapshot sees the table it walked into, itself
	// included — two participants, not one.
	expectCatchUpHead(t, joiner)
	if ids := expectPresenceSnapshot(t, joiner); len(ids) != 2 {
		t.Fatalf("the joiner's snapshot should list everyone present, got %v", ids)
	}
}

// TestPresenceAnnouncesACleanDeparture: the ordinary "I closed the tab" path.
func TestPresenceAnnouncesACleanDeparture(t *testing.T) {
	f := newGWFixture(t)
	watcher := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, watcher)
	expectPresenceSnapshot(t, watcher)

	leaver := f.dial(f.playerToken, 0)
	expectPresenceChanged(t, watcher, f.playerID, vttv1.PresenceState_PRESENCE_STATE_CONNECTED)

	_ = leaver.Close(websocket.StatusNormalClosure, "bye")
	expectPresenceChanged(t, watcher, f.playerID, vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED)
}

// TestDMGrantsControlOverTheWire is the branch's motivating flow, end to end:
// a DM hands a second character to a player, and the player ends up genuinely
// controlling both (spec §3.1, §8's demo).
//
// It exists because that flow was IMPOSSIBLE for eight commits. The contract
// carried the command, both folds applied its event, the 60-cell authz matrix
// permitted it and the MCP tool list advertised it — and ToEvent had no arm,
// so the server answered "unknown or empty command" every time. The authz
// tests stopped at Authorize and never crossed into conversion, so nothing
// noticed. A test that ends at the permission check is not evidence the
// command works.
func TestDMGrantsControlOverTheWire(t *testing.T) {
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	// a1 is seeded controlled by the player; grant it to a SECOND participant.
	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "grant-1",
		Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "a1", ParticipantId: "p-second",
		}},
	})
	res := readResult(t, dm)
	if !res.GetOk() {
		t.Fatalf("grant_actor_control refused: %q — the command is authorized and advertised, "+
			"so a refusal here means the feature does not exist", res.GetError())
	}

	st := f.campaign.State()
	actor, ok := st.Actors["a1"]
	if !ok {
		t.Fatal("actor a1 vanished")
	}
	ids := actor.GetControllerIds()
	if len(ids) != 2 || ids[1] != "p-second" {
		t.Fatalf("controller_ids = %v, want the original controller plus p-second", ids)
	}
	// The mirror still points at the FIRST controller: granting a second one
	// must not silently move the character away from the first.
	if actor.GetControllerId() != ids[0] {
		t.Fatalf("controller_id = %q, want the mirror of controller_ids[0] = %q",
			actor.GetControllerId(), ids[0])
	}

	// And revoke takes it back off.
	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "revoke-1",
		Command: &vttv1.ClientCommand_RevokeActorControl{RevokeActorControl: &vttv1.RevokeActorControl{
			ActorId: "a1", ParticipantId: "p-second",
		}},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("revoke_actor_control refused: %q", res.GetError())
	}
	if ids := f.campaign.State().Actors["a1"].GetControllerIds(); len(ids) != 1 {
		t.Fatalf("controller_ids = %v, want the grant undone", ids)
	}
}

// TestDMPromotesASpectatorOverTheWire is the seam: authorized, converted,
// APPLIED. The branch this work follows shipped grant_actor_control
// authorized and dead because nothing tested past the permission check, so
// this asserts on IDENTITY — the only place a role change is visible — rather
// than on the result frame.
func TestDMPromotesASpectatorOverTheWire(t *testing.T) {
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	watcher, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if watcher.Role != identity.RoleSpectator {
		t.Fatalf("fixture drifted: %q", watcher.Role)
	}

	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "promote-1",
		Command: &vttv1.ClientCommand_PromoteParticipant{
			PromoteParticipant: &vttv1.PromoteParticipant{
				ParticipantId: watcher.ID, Role: string(identity.RolePlayer),
			},
		},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("promotion refused: %q", res.GetError())
	}

	// The SAME token now carries the new role — which is what a reconnect will
	// read, and the whole reason role stays beside the credential.
	after, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if after.Role != identity.RolePlayer {
		t.Fatalf("role = %q, want player — the command was accepted but changed nothing",
			after.Role)
	}
}

func TestPromotingToDMIsRefusedOverTheWire(t *testing.T) {
	// End to end, because the escalation guard lives in Authorize and a
	// refactor could move the check past it.
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	watcher, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "escalate-1",
		Command: &vttv1.ClientCommand_PromoteParticipant{
			PromoteParticipant: &vttv1.PromoteParticipant{
				ParticipantId: watcher.ID, Role: string(identity.RoleDM),
			},
		},
	})
	if res := readResult(t, dm); res.GetOk() {
		t.Fatal("promotion to dm must be refused — the shared join link would otherwise " +
			"reach full authority in two steps")
	}
	after, _ := f.ids.Verify(f.spectatorToken)
	if after.Role != identity.RoleSpectator {
		t.Fatalf("a refused promotion still changed the role to %q", after.Role)
	}
}

// TestAPromotionBitesWithoutReconnecting is what J4 exists for (spec §3.2).
//
// Everyone who joins through the shared link arrives as a SPECTATOR, so if a
// promotion only took effect on reconnect, a reconnect would sit on the
// critical path of every single person who ever joins — making the shared link
// more cumbersome than the per-person invites it replaces.
func TestAPromotionBitesWithoutReconnecting(t *testing.T) {
	f := newGWFixture(t)
	watcher := f.dial(f.spectatorToken, 0)
	expectCatchUpHead(t, watcher)
	expectPresenceSnapshot(t, watcher)

	// As a spectator, narrating is refused.
	sendCommand(t, watcher, &vttv1.ClientCommand{
		RequestId: "before",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "hello"}},
	})
	if res := readResult(t, watcher); res.GetOk() {
		t.Fatal("a spectator must not be able to narrate")
	}

	// The DM promotes them, on a DIFFERENT connection.
	me, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)
	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "promote",
		Command: &vttv1.ClientCommand_PromoteParticipant{
			PromoteParticipant: &vttv1.PromoteParticipant{
				ParticipantId: me.ID, Role: string(identity.RolePlayer),
			},
		},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("promotion refused: %q", res.GetError())
	}

	// SAME connection, no reconnect: it now works.
	sendCommand(t, watcher, &vttv1.ClientCommand{
		RequestId: "after",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "hello"}},
	})
	if res := readResult(t, watcher); !res.GetOk() {
		t.Fatalf("a promoted participant must be able to act on their EXISTING connection, "+
			"got %q — otherwise everyone who joins must reconnect immediately after joining",
			res.GetError())
	}
}

// TestRevokingRemovesSomebodyWhoIsStillConnected closes a hole that predates
// this branch: the only Verify in the WS path ran at CONNECT, so a revoked
// participant kept playing — moving tokens, narrating — until they chose to
// disconnect. Throwing someone out of a table did nothing without their
// cooperation.
func TestRevokingRemovesSomebodyWhoIsStillConnected(t *testing.T) {
	f := newGWFixture(t)
	conn := f.dial(f.playerToken, 0)
	expectCatchUpHead(t, conn)
	expectPresenceSnapshot(t, conn)

	p, err := f.ids.Verify(f.playerToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.Revoke(p.ID); err != nil {
		t.Fatal(err)
	}

	before := f.head(t)

	// Their very next action must not land.
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "after-revoke",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "still here"}},
	})
	// Drain until the connection ENDS rather than asserting on the very next
	// frame: anything already queued for this connection is still written out
	// on the way down, so "the next read errors" would be a race.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("a revoked participant must be cut off on their next command, not left " +
				"playing until they decide to leave")
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, _, err := conn.Read(ctx)
		cancel()
		if err == nil {
			continue // a frame that was already queued; keep draining
		}
		// A CLOSE, not a timeout. The first version of this loop returned on
		// any error, so when the server did NOT hang up the read simply
		// expired and the test read that as success — it passed under an
		// injection that ignored the lookup failure entirely.
		if websocket.CloseStatus(err) == -1 {
			t.Fatalf("the connection did not close — a revoked participant is still being "+
				"served (read ended with %v, not a websocket close)", err)
		}
		break
	}

	// AND THE COMMAND ITSELF NEVER LANDED. The doc comment above claims "their
	// very next action must not land", and until this the test only checked
	// that the socket eventually closed — so failing open (run the command
	// with the cached participant, THEN close) passed, with the revoked
	// player's narration appended to the permanent log on the way out. Closing
	// the door after somebody has already walked through it is not a lock.
	//
	// The LOG is the witness, not folded state: NarrationAdded deliberately
	// does not mutate state, so campaign.State() cannot tell these apart.
	if after := f.head(t); after != before {
		t.Fatalf("the revoked participant's command was APPENDED anyway (log head %d → %d): "+
			"they were cut off, but only after their action had landed permanently", before, after)
	}
}

// head reads the campaign's current log head through a throwaway connection.
//
// It exists because NarrationAdded does NOT mutate folded state, so
// campaign.State() cannot answer "did that command actually land?". The log
// can, and the catch-up head is the log's sequence in one frame.
func (f *gwFixture) head(t *testing.T) int64 {
	t.Helper()
	c := f.dial(f.dmToken, 0)
	defer c.Close(websocket.StatusNormalClosure, "")
	return expectCatchUpHead(t, c)
}

// TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody separates the
// two things a failed Lookup can mean.
//
// Re-resolving per command put identity I/O on a path that previously had
// none — and so it put identity's FAILURE MODES there too. Revoked and unknown
// are facts about a credential and end the connection. A database that cannot
// answer is not a fact about anybody's credential: closing on it tells a
// player in good standing "your credential is no longer valid", which is
// false, and throws them out of a live table over a transient. identity.go's
// own comment records this exact shared file blocking the full busy_timeout(5000)
// and then failing under another handle's write transaction, so the contention
// case is measured behaviour in this repo, not a hypothetical.
//
// The precedent is already in handleCommand one screen below: a campaign that
// cannot answer produces ok=false and leaves the connection open.
func TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody(t *testing.T) {
	f := newGWFixture(t)
	conn := f.dial(f.playerToken, 0)
	expectCatchUpHead(t, conn)
	expectPresenceSnapshot(t, conn)

	// OPERATIONAL, not a credential fact — nobody was revoked, and the row is
	// still there. Only the table it lives in has moved out from under the
	// query, which is what an unhealthy database looks like from here.
	raw, err := sql.Open("sqlite", f.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`ALTER TABLE participants RENAME TO participants_elsewhere`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "while-unwell",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "one"}},
	})
	res := readResult(t, conn)
	if res.GetOk() {
		t.Fatal("a command must not be authorized while identity cannot say who the caller is")
	}

	// AND THEY ARE STILL AT THE TABLE. This is the half that fails when an
	// operational error is misread as a revocation: readResult reports a
	// closed connection rather than timing out, so the failure names itself.
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "still-here",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "two"}},
	})
	if res := readResult(t, conn); res.GetOk() {
		t.Fatal("the command must still be refused while identity is unwell")
	}

	// AND THE TABLE STILL REACHES THEM. Delivery re-resolves as well, so the
	// same distinction has to hold there: dropping frames — or the connection
	// — on an unhealthy database would make a transient look like everyone
	// being thrown out at once. Losing an event is worse than a moment's delay
	// in removing somebody, and the very next event catches a revoked watcher.
	mustAppend(t, f.campaign, "while-unwell", &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{SceneId: "attic", Name: "Attic", GridWidth: 4, GridHeight: 4},
	})
	// This connection dialed at 0, so the seeded history is queued ahead of it.
	// readEvent fails the test outright if the connection ends instead.
	for i := 0; ; i++ {
		if i == 8 {
			t.Fatal("the event appended while identity was unreadable never arrived")
		}
		if readEvent(t, conn).GetEventId() == "while-unwell" {
			return
		}
	}
}

// TestARevokedSpectatorStopsSeeingTheTable closes the half of revocation that
// re-resolving per COMMAND cannot reach, and it is the half this branch
// created.
//
// commandRoles has no spectator row anywhere: a spectator may issue NO command
// at all. So the command loop's Lookup never fires for one, and the shared
// join link mints nothing BUT spectators. A stranger who got in through a
// leaked link and was then revoked kept watching the entire session — the
// branch's own headline, that a leaked link can be closed and the stranger
// removed, had a hole exactly in the population the branch creates.
//
// DELIVERY is where a spectator meets the server, so delivery is where it has
// to bite. Not on a timer and not on connect: on the next thing the table
// would have shown them.
func TestARevokedSpectatorStopsSeeingTheTable(t *testing.T) {
	f := newGWFixture(t)
	conn := f.dial(f.spectatorToken, 0)
	expectCatchUpHead(t, conn)
	expectPresenceSnapshot(t, conn)

	p, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.Revoke(p.ID); err != nil {
		t.Fatal(err)
	}

	// The table plays on, which is the only way a watcher is a watcher.
	mustAppend(t, f.campaign, "after-revoke", &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{SceneId: "vault", Name: "Vault", GridWidth: 5, GridHeight: 5},
	})

	deadline := time.Now().Add(10 * time.Second)
	for {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, data, err := conn.Read(ctx)
		timedOut := ctx.Err() != nil // captured BEFORE cancel, which overwrites it
		cancel()
		if err == nil {
			// Anything already queued may still be written on the way down,
			// but NOT this: it was appended after the revocation.
			if strings.Contains(string(data), "after-revoke") {
				t.Fatal("a revoked spectator was delivered an event that happened AFTER they " +
					"were thrown out")
			}
			continue
		}
		// A TIMEOUT is not a close, and conflating the two is what made the
		// sibling test below pass under an injection that did nothing.
		if timedOut {
			t.Fatal("the connection did not end — a revoked spectator is still being served, " +
				"and no command of theirs can be refused instead, because they can issue none")
		}
		// AND THEY ARE TOLD WHY. This is the ONLY path that can end a
		// spectator's connection — they issue no commands, so the command
		// loop's revocation check never runs for them — and it used to drop
		// the socket with no close frame. A person whose client says "closed"
		// with no reason cannot tell being thrown out from a flaky network,
		// and the two want completely different responses from them.
		//
		// The two revocation paths said different things and which one fired
		// was a race: measured 5 failures in 30 runs of the sibling test,
		// shifting with unrelated timing elsewhere in serve.
		if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
			t.Fatalf("the connection ended with status %v, want StatusPolicyViolation — a "+
				"revoked spectator must be told why, not merely dropped (err: %v)", status, err)
		}
		return
	}
}

// --- the door, driven the way a DM drives it (plan J6) --------------------

// postJoin exercises the real /join endpoint against this fixture's server,
// returning the status and the minted token (empty on refusal).
func (f *gwFixture) postJoin(t *testing.T, secret, name string) (int, string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"secret": secret, "displayName": name})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(f.srv.URL+"/join", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out.Token
}

func setDoor(t *testing.T, conn *websocket.Conn, door vttv1.JoinDoor) *vttv1.CommandResult {
	t.Helper()
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "r-door",
		Command:   &vttv1.ClientCommand_SetJoinDoor{SetJoinDoor: &vttv1.SetJoinDoor{Door: door}},
	})
	return readResult(t, conn)
}

// TestTheDMCanActuallyOpenTheDoor is the seam test this whole task exists for.
//
// Before it, identity.SetJoinOpen had ZERO non-test callers. The door is closed
// by default, so /join was live and refused everyone, forever, and no human
// could reach the feature at all — five completed tasks, every gate green, and
// nobody could join a table. The presence branch shipped ONE layer dead;
// this would have shipped the whole feature dead.
func TestTheDMCanActuallyOpenTheDoor(t *testing.T) {
	f := newGWFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}

	// Closed by default: this is the state the feature ships in.
	if status, _ := f.postJoin(t, secret, "Kim"); status == http.StatusOK {
		t.Fatal("the door must start closed")
	}

	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)
	if res := setDoor(t, dm, vttv1.JoinDoor_JOIN_DOOR_OPEN); !res.GetOk() {
		t.Fatalf("a DM must be able to open the door: %s", res.GetError())
	}

	status, token := f.postJoin(t, secret, "Kim")
	if status != http.StatusOK {
		t.Fatalf("with the door open a joiner must get in, got %d", status)
	}
	// And what they got is REAL: a spectator credential this server accepts.
	p, err := f.ids.Verify(token)
	if err != nil {
		t.Fatalf("the minted token must verify: %v", err)
	}
	if p.Role != identity.RoleSpectator {
		t.Fatalf("role = %q, want spectator", p.Role)
	}

	// And closing it again shuts the newcomers out.
	if res := setDoor(t, dm, vttv1.JoinDoor_JOIN_DOOR_CLOSED); !res.GetOk() {
		t.Fatalf("a DM must be able to close the door again: %s", res.GetError())
	}
	if status, _ := f.postJoin(t, secret, "Robin"); status == http.StatusOK {
		t.Fatal("a door that only opens is not a door")
	}
}

func TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt(t *testing.T) {
	// The reason the contract carries an ENUM and not a bool. protojson omits
	// zero values, so `bool open` would put CLOSED on the wire as an ABSENT
	// FIELD — and a sender that forgot the field entirely would be
	// indistinguishable from one asking to shut the door. Guessing either way
	// is wrong: guess OPEN and a bug admits strangers, guess CLOSED and a bug
	// locks the table out mid-session.
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	res := setDoor(t, dm, vttv1.JoinDoor_JOIN_DOOR_UNSPECIFIED)
	if res.GetOk() {
		t.Fatal("an unspecified door must be refused, not resolved to a default")
	}
	if f.ids.JoinOpen() {
		t.Fatal("a refused command must not have moved the door")
	}
}

func TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse(t *testing.T) {
	// The property spec §2 calls close to required: a LEAKED link must be
	// closable without re-inviting anyone already in.
	f := newGWFixture(t)
	old, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)
	if res := setDoor(t, dm, vttv1.JoinDoor_JOIN_DOOR_OPEN); !res.GetOk() {
		t.Fatal(res.GetError())
	}
	// Somebody joined on the old link before it leaked.
	_, earlyToken := f.postJoin(t, old, "Kim")
	if earlyToken == "" {
		t.Fatal("setup: the first joiner should have got in")
	}

	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "r-rot",
		Command:   &vttv1.ClientCommand_RotateJoinLink{RotateJoinLink: &vttv1.RotateJoinLink{}},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("a DM must be able to rotate the link: %s", res.GetError())
	}

	if status, _ := f.postJoin(t, old, "Stranger"); status == http.StatusOK {
		t.Fatal("the OLD link must stop working — that is the entire point of rotating")
	}
	fresh, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if fresh == old {
		t.Fatal("rotating must actually change the secret")
	}
	if status, _ := f.postJoin(t, fresh, "Robin"); status != http.StatusOK {
		t.Fatalf("the NEW link must work, got %d", status)
	}
	// And the person already through the old link is untouched: their token is
	// theirs, not the link's. Without this, closing a leak would mean
	// re-inviting the whole table at the worst possible moment.
	if _, err := f.ids.Verify(earlyToken); err != nil {
		t.Fatalf("rotating must not disturb anyone already in: %v", err)
	}
	// Rotating is INDEPENDENT of the door: it says nothing about whether the
	// link is open, and a rotate that quietly shut the table would be a very
	// unwelcome surprise mid-session.
	if !f.ids.JoinOpen() {
		t.Fatal("rotating the link must not close the door")
	}
}

// TestPromotionCannotUNMAKEADMOrAgent closes the mirror image of the rule
// spec §3.1a states.
//
// §3.1a bounds what a promotion may promote TO — only player or spectator, so
// the shared link is never a route to full authority in two steps. It says
// nothing about who may be promoted FROM, and the two are not the same rule:
// promote_participant(dm_id, "spectator") names an allowed role, so the target
// check passes, and a DM becomes a spectator at their own table.
//
// That is not recoverable in-band. Promotion cannot reach `dm` by design, so
// nobody left at the table can undo it — it takes host access and `vtt
// invite`. AGENTS ARE AUTHORIZED to promote, which is the sharp end: an agent
// having a bad day can lock every human out of their own campaign.
func TestPromotionCannotUNMAKEADMOrAgent(t *testing.T) {
	f := newGWFixture(t)
	dmP, err := f.ids.Verify(f.dmToken)
	if err != nil {
		t.Fatal(err)
	}
	agentP, err := f.ids.Verify(f.agentToken)
	if err != nil {
		t.Fatal(err)
	}

	// Issued BY the agent, because that is the case that matters: a DM doing
	// this to themselves is a mistake, an agent doing it is a takeover.
	conn := f.dial(f.agentToken, 0)
	expectCatchUpHead(t, conn)
	expectPresenceSnapshot(t, conn)

	for _, target := range []struct {
		what string
		id   string
	}{
		{"the DM", dmP.ID},
		{"another agent", agentP.ID},
	} {
		sendCommand(t, conn, &vttv1.ClientCommand{
			RequestId: "r-" + target.id,
			Command: &vttv1.ClientCommand_PromoteParticipant{
				PromoteParticipant: &vttv1.PromoteParticipant{
					ParticipantId: target.id, Role: "spectator",
				},
			},
		})
		if res := readResult(t, conn); res.GetOk() {
			t.Fatalf("%s was demoted to a spectator — nobody left at the table can undo "+
				"that, because promotion cannot reach dm or agent", target.what)
		}
		now, err := f.ids.Lookup(target.id)
		if err != nil {
			t.Fatal(err)
		}
		if now.Role == identity.RoleSpectator {
			t.Fatalf("%s is now a spectator despite the refusal", target.what)
		}
	}

	// And an ordinary promotion still works — a guard that refuses everything
	// is not a guard.
	p, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "r-ok",
		Command: &vttv1.ClientCommand_PromoteParticipant{
			PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: p.ID, Role: "player"},
		},
	})
	if res := readResult(t, conn); !res.GetOk() {
		t.Fatalf("promoting a spectator must still work: %s", res.GetError())
	}
}

// TestAPromotionIsAnnouncedToThePromotedPersonThemselves covers the half of
// promotion that live re-resolution does not reach.
//
// The server starts accepting a promoted spectator's commands immediately
// (J4). Their BROWSER read its role once, at connect, via /api/me — and
// nothing ever told it that role moved, so they could act and their own client
// offered them nothing to act with: the server saying yes to a screen with no
// controls on it. The re-announcement is the nudge that makes the client
// re-read.
//
// Two things are asserted and both are load-bearing. That a frame arrives AT
// ALL is the point of the announcement. That it carries the promoted person's
// own DISPLAY NAME is what pins which connection the registry looked up — a
// lookup that matched the wrong participant would announce somebody else's
// name under this id, and the table's roster would quietly rename them.
func TestAPromotionIsAnnouncedToThePromotedPersonThemselves(t *testing.T) {
	f := newGWFixture(t)

	// TWO connections, deliberately: with only one, a registry lookup that
	// matched the wrong participant would find nobody and merely fall silent,
	// which the first assertion catches by accident rather than on purpose.
	// With a second connection present it returns the WRONG NAME, and only an
	// assertion on the name can tell.
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	watcher := f.dial(f.spectatorToken, 0)
	expectCatchUpHead(t, watcher)
	expectPresenceSnapshot(t, watcher)

	p, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}

	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "r-promote",
		Command: &vttv1.ClientCommand_PromoteParticipant{
			PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: p.ID, Role: "player"},
		},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("promotion refused: %s", res.GetError())
	}

	// Read on the PROMOTED person's own socket: they are the one who most
	// needs this, and broadcast excludes nobody for exactly that reason.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("a promoted participant was never told anything on their own connection, " +
				"so their client goes on believing the role it read at connect")
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, raw, err := watcher.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var frame vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("frame did not decode: %v (raw=%s)", err, raw)
		}
		pc := frame.GetPresenceChanged()
		if pc == nil || pc.GetParticipantId() != p.ID {
			continue
		}
		if pc.GetDisplayName() != p.Name {
			t.Fatalf("the announcement names %q under %q's id, want %q — the registry "+
				"matched the wrong connection, and the table would rename them",
				pc.GetDisplayName(), p.ID, p.Name)
		}
		return
	}
}

// TestARevokedWatcherIsNotEvenToldWhoElseArrives closes the half of revocation
// that per-event re-resolution cannot reach.
//
// The pump re-resolves before delivering an EVENT. Presence frames never travel
// that channel — announcePresence writes straight into each connection's out —
// so a revoked stranger who came in on a leaked link went on watching the guest
// list arrive and leave until the table happened to append something. Nothing
// of the campaign leaked, but spec §3.2 states the property without
// qualification, and "the next thing the table would have shown them" plainly
// includes somebody walking in.
//
// The absence is asserted against a POSITIVE control on the same frame: an
// unrevoked watcher must receive it. Without that, a broadcast that reached
// nobody at all would pass.
func TestARevokedWatcherIsNotEvenToldWhoElseArrives(t *testing.T) {
	f := newGWFixture(t)

	stranger := f.dial(f.spectatorToken, 0)
	expectCatchUpHead(t, stranger)
	expectPresenceSnapshot(t, stranger)

	// The positive control, connected before the revocation so both sockets
	// are in exactly the same state.
	witness := f.dial(f.otherPlayerToken, 0)
	expectCatchUpHead(t, witness)
	expectPresenceSnapshot(t, witness)
	// The stranger legitimately learns the witness arrived.
	expectPresenceChanged(t, stranger, f.playerIDFor(t, f.otherPlayerToken),
		vttv1.PresenceState_PRESENCE_STATE_CONNECTED)

	p, err := f.ids.Verify(f.spectatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.Revoke(p.ID); err != nil {
		t.Fatal(err)
	}

	// Somebody else joins. No event is appended — people are just arriving.
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)
	dmID := f.playerIDFor(t, f.dmToken)

	// The witness is told, so the frame really was broadcast.
	expectPresenceChanged(t, witness, dmID, vttv1.PresenceState_PRESENCE_STATE_CONNECTED)

	// The revoked stranger is not.
	expectNoPresenceWithin(t, stranger, 500*time.Millisecond)
}

// expectNoPresenceWithin fails if any PresenceChanged reaches conn within d.
//
// Reads the socket DIRECTLY, and that is the whole reason it exists.
// assertNoFrameWithin routes through the frame queue, which demultiplexes into
// results, events and closed — and DROPS presence frames. Asserting "no
// presence arrived" through a helper that cannot see presence is an assertion
// with no teeth, and this one had none: reverting the deny set failed nothing
// at all. The fourteenth instance of that defect class on this branch, and I
// wrote it while fixing the thirteenth.
//
// Single-use per connection: coder/websocket closes the socket when a Read's
// context expires, so this belongs last in a test.
func expectNoPresenceWithin(t *testing.T, conn *websocket.Conn, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, raw, err := conn.Read(ctx)
		quiet := ctx.Err() != nil // captured BEFORE cancel overwrites it
		cancel()
		if err != nil {
			if quiet {
				return // nothing arrived, which is the point
			}
			t.Fatalf("read: %v", err)
		}
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &f); err != nil {
			t.Fatalf("frame did not decode: %v (raw=%s)", err, raw)
		}
		if pc := f.GetPresenceChanged(); pc != nil {
			t.Fatalf("a revoked participant was told that %q is %v — they are meant to see "+
				"nothing at all", pc.GetDisplayName(), pc.GetState())
		}
	}
}

// playerIDFor resolves a token to its participant id.
func (f *gwFixture) playerIDFor(t *testing.T, token string) string {
	t.Helper()
	p, err := f.ids.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// TestTheWireCarriesTheAdmissionBudget is the other half of the seam
// TestTheDMCanActuallyOpenTheDoor exists for.
//
// The DM console opens the door over the WEBSOCKET, not the CLI. A budget the
// CLI could set and the wire could not would mean the console opens an
// eight-person door every time with no way to say otherwise — and nothing would
// fail, because the CLI tests would be green.
func TestTheWireCarriesTheAdmissionBudget(t *testing.T) {
	f := newGWFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	conn := f.dial(f.dmToken, 0)
	defer conn.CloseNow()

	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "r-door",
		Command: &vttv1.ClientCommand_SetJoinDoor{SetJoinDoor: &vttv1.SetJoinDoor{
			Door:       vttv1.JoinDoor_JOIN_DOOR_OPEN,
			AdmitLimit: 1,
		}},
	})
	if res := readResult(t, conn); !res.GetOk() {
		t.Fatalf("opening the door with a budget failed: %s", res.GetError())
	}

	if status, _ := f.postJoin(t, secret, "First"); status != http.StatusOK {
		t.Fatalf("the one admission this door allows was refused (%d)", status)
	}
	if status, _ := f.postJoin(t, secret, "Second"); status != http.StatusForbidden {
		t.Fatalf("a second joiner got %d against admit_limit=1 — the wire field is decoded "+
			"but never reaches the door", status)
	}
}

// TestADoorOpenedOverTheWireWithNoBudgetStillAdmits is the protojson trap, and
// this repo has already been bitten by it once (JoinDoor is an enum and not a
// bool for exactly this reason).
//
// protojson OMITS ZERO VALUES, so a console that does not set admit_limit sends
// bytes indistinguishable from one that deliberately sends 0. Taking that
// literally opens a door which refuses everybody: the DM sees "open", every
// joiner sees the same 403 a stranger sees, and nothing anywhere says why.
func TestADoorOpenedOverTheWireWithNoBudgetStillAdmits(t *testing.T) {
	f := newGWFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	conn := f.dial(f.dmToken, 0)
	defer conn.CloseNow()

	// admit_limit deliberately UNSET — the wire shape a client that never
	// heard of the field produces.
	if res := setDoor(t, conn, vttv1.JoinDoor_JOIN_DOOR_OPEN); !res.GetOk() {
		t.Fatalf("opening the door failed: %s", res.GetError())
	}

	if status, _ := f.postJoin(t, secret, "Somebody"); status != http.StatusOK {
		t.Fatalf("a door opened with no stated budget refused a joiner (%d) — an absent "+
			"field was read as 'admit nobody', which is an open door nobody can get through "+
			"and nothing explains", status)
	}
}
