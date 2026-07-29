package gateway_test

import (
	"context"
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

// --- fixture -------------------------------------------------------------

// exitFixture is a real gateway.Server wired to a fresh, EMPTY temp
// campaign+identity DB (no seeded history — unlike gwFixture in
// server_test.go): the three-role exit scenario (spec §10) builds its own
// history live, over real WebSocket connections, from sequence 1. Invites
// are minted directly against identity.DB on the same temp campaign file,
// per the task brief.
type exitFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken, dmID               string
	playerToken, playerID       string
	agentToken, agentID         string
	spectatorToken, spectatorID string
}

func newExitFixture(t *testing.T) *exitFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "exit-scenario.db")

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

	dmToken, dmID, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The player's invite controls act-lera ONLY: no other actor is ever
	// listed here, and no actor added below grants this participant control
	// besides act-lera (act-ursus is added controllerless).
	playerToken, playerID, err := ids.CreateInvite("Player", identity.RolePlayer, []string{"act-lera"})
	if err != nil {
		t.Fatal(err)
	}
	agentToken, agentID, err := ids.CreateInvite("Agent", identity.RoleAgent, nil)
	if err != nil {
		t.Fatal(err)
	}
	spectatorToken, spectatorID, err := ids.CreateInvite("Spectator", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}

	srv := gateway.New(c, ids)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &exitFixture{
		t: t, srv: httpSrv,
		dmToken: dmToken, dmID: dmID,
		playerToken: playerToken, playerID: playerID,
		agentToken: agentToken, agentID: agentID,
		spectatorToken: spectatorToken, spectatorID: spectatorID,
	}
}

func (f *exitFixture) wsURL(token string, after int64) string {
	f.t.Helper()
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

// dial connects with token/after and wraps the result in a scenarioConn
// (see its doc comment for why: this scenario reuses every connection
// across multiple absence checks, which server_test.go's connections never
// do). Registers CloseNow on t.Cleanup, mirroring gwFixture.dial.
func (f *exitFixture) dial(token string, after int64) *scenarioConn {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, f.wsURL(token, after), nil)
	if err != nil {
		f.t.Fatalf("dial: %v", err)
	}
	f.t.Cleanup(func() { conn.CloseNow() })
	return newScenarioConn(conn)
}

// --- scenarioConn: non-destructive bounded reads --------------------------

// scenarioConn is a thin wrapper over the shared frameQueue (server_test.go),
// which owns the connection's only reader and sorts frames by kind.
//
// It used to maintain its OWN reader and channel, deliberately not reusing
// server_test.go's helpers, because those called conn.Read with an expiring
// context: coder/websocket's setupReadTimeout registers an AfterFunc that
// calls c.close(), closing the WHOLE socket rather than aborting one read
// (see vendored conn.go:188-199, read.go prepareRead/finishRead), so a
// genuinely-timed-out Read is destructive by design in that library. This
// scenario issues MULTIPLE "assert nothing arrived" checks per connection and
// keeps using those connections afterward, so that path was not an option.
//
// frameQueue now reads with context.Background() and bounds every wait with a
// channel select instead, so the destructive path is unreachable for ALL
// tests in this package -- Background's Done() is nil, so setupReadTimeout's
// first check (`if ctx.Done() == nil { return false }`) skips registering the
// AfterFunc at all. The separate implementation therefore no longer earns its
// keep, and sharing one means the result/event demultiplexing is not
// something a future connection wrapper can forget.
type scenarioConn struct {
	conn *websocket.Conn
	q    *frameQueue
}

func newScenarioConn(conn *websocket.Conn) *scenarioConn {
	return &scenarioConn{conn: conn, q: queueFor(conn)}
}

// scenarioConnReadBudget bounds each wait. It was raised 10s->20s chasing
// what was recorded as a contention flake; the real cause was result/event
// interleaving desynchronising the connection (see frameQueue in
// server_test.go), which no budget could fix -- the awaited frame had already
// been consumed and discarded. Kept generous because it now only guards
// against a genuinely wedged server.
const scenarioConnReadBudget = 20 * time.Second

// readResult returns the next CommandResult. An Envelope that arrives first
// stays queued for the next readEvent instead of being thrown away.
func (sc *scenarioConn) readResult(t *testing.T) *vttv1.CommandResult {
	t.Helper()
	select {
	case r := <-sc.q.results:
		return r
	case <-sc.q.closed:
		t.Fatalf("scenarioConn.readResult: connection closed: %v", sc.q.err)
	case <-time.After(scenarioConnReadBudget):
		t.Fatalf("scenarioConn.readResult: no CommandResult within %s", scenarioConnReadBudget)
	}
	return nil
}

// readEvent returns the next Envelope. A CommandResult that arrives first
// stays queued for the next readResult.
func (sc *scenarioConn) readEvent(t *testing.T) *vttv1.Envelope {
	t.Helper()
	select {
	case e := <-sc.q.events:
		return e
	case <-sc.q.closed:
		t.Fatalf("scenarioConn.readEvent: connection closed: %v", sc.q.err)
	case <-time.After(scenarioConnReadBudget):
		t.Fatalf("scenarioConn.readEvent: no Envelope within %s", scenarioConnReadBudget)
	}
	return nil
}

// assertNoFrameWithin waits up to d and fails if ANY frame arrives (or the
// connection closes). Non-destructive: this scenario keeps using these
// connections afterward.
func (sc *scenarioConn) assertNoFrameWithin(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case r := <-sc.q.results:
		t.Fatalf("scenarioConn: want no frame within %s, got result %s", d, r.GetRequestId())
	case e := <-sc.q.events:
		t.Fatalf("scenarioConn: want no frame within %s, got event seq=%d", d, e.GetSequence())
	case <-sc.q.closed:
		t.Fatalf("scenarioConn: connection unexpectedly closed while expecting no frame within %s", d)
	case <-time.After(d):
		// Nothing arrived: exactly what we want, connection stays open.
	}
}

func (sc *scenarioConn) sendCommand(t *testing.T, cmd *vttv1.ClientCommand) {
	t.Helper()
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sc.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
}

// --- scenario helpers ------------------------------------------------------

// participant pairs a scenario role's display name/id with its live
// connection, so every helper below can both issue commands and verify
// per-connection delivery/participant_id without threading four separate
// variables through every call.
type participant struct {
	name string
	id   string
	role string
	conn *scenarioConn
}

// issueAndVerify sends cmd on issuer.conn, requires an ok=true result, then
// reads the resulting broadcast Envelope off EVERY connection in all
// (issuer's own connection included — the issuer receives its own broadcast
// too, per TestTwoClientsBothReceiveAcceptedCommandAsEvent's precedent) and
// requires every one of them to carry the identical (event_id, sequence)
// pair, the given wantParticipant as participant_id, and issuer.role as
// actor_role (spec §4: "every accepted command stamps actor_role AND ...
// participant_id" — this pins both, for every command including
// RetractEvents, not just participant_id). It returns the envelope read on
// the connection named "dm", so callers can build the ordered "what DM
// received live" record the final reconnect check needs.
//
// wantResultSequence: for every command EXCEPT RetractEvents, the
// CommandResult's Sequence must equal the broadcast Envelope's Sequence
// (TestSequenceInResultMatchesBroadcastEnvelope's contract, folded into
// server_test.go's TestTwoClientsBothReceiveAcceptedCommandAsEvent). The one
// documented exception is RetractEvents: server.go's handleRetraction
// deliberately leaves CommandResult.Sequence unset (0) because
// campaign.Undo does not return the marker's sequence — that sequence is
// still visible on the broadcast Envelope itself, just not echoed back in
// the result. Callers pass wantResultSequence=false only for that command.
func issueAndVerify(t *testing.T, issuer participant, cmd *vttv1.ClientCommand, wantParticipant string, wantResultSequence bool, all []participant) *vttv1.Envelope {
	t.Helper()
	issuer.conn.sendCommand(t, cmd)
	result := issuer.conn.readResult(t)
	if !result.Ok {
		t.Fatalf("%s (%s): want ok=true, got error %q", cmd.GetRequestId(), issuer.name, result.Error)
	}

	var first, dmEnv *vttv1.Envelope
	for _, p := range all {
		env := p.conn.readEvent(t)
		if env.ParticipantId != wantParticipant {
			t.Fatalf("%s: %s's connection saw ParticipantId=%q, want %q", cmd.GetRequestId(), p.name, env.ParticipantId, wantParticipant)
		}
		if env.ActorRole != issuer.role {
			t.Fatalf("%s: %s's connection saw ActorRole=%q, want %q (issuer %s's role)", cmd.GetRequestId(), p.name, env.ActorRole, issuer.role, issuer.name)
		}
		if first == nil {
			first = env
		} else if env.EventId != first.EventId || env.Sequence != first.Sequence {
			t.Fatalf("%s: %s saw (event_id=%s, seq=%d), want (%s, %d) matching every other client",
				cmd.GetRequestId(), p.name, env.EventId, env.Sequence, first.EventId, first.Sequence)
		}
		if p.name == "dm" {
			dmEnv = env
		}
	}
	if dmEnv == nil {
		t.Fatalf("%s: \"dm\" was not present in the participant list passed to issueAndVerify", cmd.GetRequestId())
	}
	if wantResultSequence && result.Sequence != first.Sequence {
		t.Fatalf("%s: result.Sequence=%d, broadcast Sequence=%d, want equal", cmd.GetRequestId(), result.Sequence, first.Sequence)
	}
	return dmEnv
}

// expectDenied sends cmd on issuer.conn, requires an ok=false result with a
// non-empty error, and then requires NO connection in all — issuer included
// — to observe any broadcast frame within a bounded window. Used for both
// denial cases the scenario enumerates: player→controllerless-token and
// spectator→StartSession.
func expectDenied(t *testing.T, issuer participant, cmd *vttv1.ClientCommand, all []participant) {
	t.Helper()
	issuer.conn.sendCommand(t, cmd)
	result := issuer.conn.readResult(t)
	if result.Ok {
		t.Fatalf("%s (%s): want ok=false", cmd.GetRequestId(), issuer.name)
	}
	if result.Error == "" {
		t.Fatalf("%s (%s): want non-empty error on denial", cmd.GetRequestId(), issuer.name)
	}
	for _, p := range all {
		p.conn.assertNoFrameWithin(t, 300*time.Millisecond)
	}
}

// drainEnvelopes reads exactly n Envelope frames off conn (catch-up
// replay), in order.
func drainEnvelopes(t *testing.T, conn *scenarioConn, n int) []*vttv1.Envelope {
	t.Helper()
	out := make([]*vttv1.Envelope, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, conn.readEvent(t))
	}
	return out
}

// --- the scenario ----------------------------------------------------------

// TestThreeRoleExitScenarioOverLiveWebSockets is the sub-project 3 exit
// criterion (spec §10) and sub-project 4's harness seed: four real
// WebSocket clients (DM, a player controlling act-lera ONLY, an agent, and
// a spectator) drive a full session over a real in-process server, covering
// every step the task brief enumerates:
//
//  1. DM: StartSession, CreateScene, AddActor×2 (act-ursus controllerless,
//     act-lera controlled by the player), PlaceToken×2 — each broadcast to
//     all four clients with the DM's participant_id.
//  2. Player: moves act-lera's token OK (own participant_id); moves
//     act-ursus's token → ok=false AND no broadcast to any client (a
//     controllerless actor is DM/agent-only).
//  3. Agent: moves act-ursus's token OK (own participant_id); retracts that
//     move via RetractEvents → the EventsRetracted marker reaches all four
//     clients.
//  4. Spectator: attempts StartSession → ok=false, no broadcast anywhere.
//  5. DM: EndSession.
//  6. All four clients disconnect. The player reconnects with after=0: its
//     full catch-up replay must equal, envelope for envelope (order, event
//     ids, and participant_id), both what the DM client received live
//     across the whole run AND what a brand-new fresh subscriber (also
//     after=0) sees.
func TestThreeRoleExitScenarioOverLiveWebSockets(t *testing.T) {
	f := newExitFixture(t)

	dm := participant{name: "dm", id: f.dmID, role: "dm", conn: f.dial(f.dmToken, 0)}
	player := participant{name: "player", id: f.playerID, role: "player", conn: f.dial(f.playerToken, 0)}
	agent := participant{name: "agent", id: f.agentID, role: "agent", conn: f.dial(f.agentToken, 0)}
	spectator := participant{name: "spectator", id: f.spectatorID, role: "spectator", conn: f.dial(f.spectatorToken, 0)}
	all := []participant{dm, player, agent, spectator}

	// dmLive is the ordered record of every envelope the DM client received
	// LIVE across the whole run — the baseline the final reconnect/catch-up
	// check compares against.
	var dmLive []*vttv1.Envelope

	// --- DM: StartSession, CreateScene, AddActor×2, PlaceToken×2 ---

	env := issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-start-session",
		Command:   &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "Exit Scenario"}},
	}, dm.id, true, all)
	if _, ok := env.Payload.(*vttv1.Envelope_SessionStarted); !ok {
		t.Fatalf("dm-start-session: payload = %T, want SessionStarted", env.Payload)
	}
	dmLive = append(dmLive, env)

	env = issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-create-scene",
		Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
			SceneId: "scn-exit", Name: "Exit Hall", GridWidth: 10, GridHeight: 10,
		}},
	}, dm.id, true, all)
	if _, ok := env.Payload.(*vttv1.Envelope_SceneCreated); !ok {
		t.Fatalf("dm-create-scene: payload = %T, want SceneCreated", env.Payload)
	}
	dmLive = append(dmLive, env)

	env = issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-add-ursus",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-ursus", Name: "Ursus"}, // ControllerId left empty: DM/agent-only.
		}},
	}, dm.id, true, all)
	aa, ok := env.Payload.(*vttv1.Envelope_ActorAdded)
	if !ok {
		t.Fatalf("dm-add-ursus: payload = %T, want ActorAdded", env.Payload)
	}
	if aa.ActorAdded.Actor.GetControllerId() != "" {
		t.Fatalf("dm-add-ursus: ControllerId = %q, want empty (controllerless)", aa.ActorAdded.Actor.GetControllerId())
	}
	dmLive = append(dmLive, env)

	env = issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-add-lera",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-lera", Name: "Lera", ControllerId: player.id},
		}},
	}, dm.id, true, all)
	aa, ok = env.Payload.(*vttv1.Envelope_ActorAdded)
	if !ok {
		t.Fatalf("dm-add-lera: payload = %T, want ActorAdded", env.Payload)
	}
	if aa.ActorAdded.Actor.GetControllerId() != player.id {
		t.Fatalf("dm-add-lera: ControllerId = %q, want player id %q", aa.ActorAdded.Actor.GetControllerId(), player.id)
	}
	dmLive = append(dmLive, env)

	env = issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-place-ursus",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-ursus", SceneId: "scn-exit", ActorId: "act-ursus",
			Position: &vttv1.GridPosition{X: 0, Y: 0},
		}},
	}, dm.id, true, all)
	if _, ok := env.Payload.(*vttv1.Envelope_TokenPlaced); !ok {
		t.Fatalf("dm-place-ursus: payload = %T, want TokenPlaced", env.Payload)
	}
	dmLive = append(dmLive, env)

	env = issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-place-lera",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-lera", SceneId: "scn-exit", ActorId: "act-lera",
			Position: &vttv1.GridPosition{X: 1, Y: 1},
		}},
	}, dm.id, true, all)
	if _, ok := env.Payload.(*vttv1.Envelope_TokenPlaced); !ok {
		t.Fatalf("dm-place-lera: payload = %T, want TokenPlaced", env.Payload)
	}
	dmLive = append(dmLive, env)

	// --- Player: moves lera's token OK; moves ursus's token denied ---

	env = issueAndVerify(t, player, &vttv1.ClientCommand{
		RequestId: "player-move-lera",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-lera", To: &vttv1.GridPosition{X: 2, Y: 2},
		}},
	}, player.id, true, all)
	if _, ok := env.Payload.(*vttv1.Envelope_TokenMoved); !ok {
		t.Fatalf("player-move-lera: payload = %T, want TokenMoved", env.Payload)
	}
	dmLive = append(dmLive, env)

	expectDenied(t, player, &vttv1.ClientCommand{
		RequestId: "player-move-ursus-denied",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-ursus", To: &vttv1.GridPosition{X: 9, Y: 9},
		}},
	}, all)

	// --- Agent: moves ursus's token OK, then retracts that move ---

	env = issueAndVerify(t, agent, &vttv1.ClientCommand{
		RequestId: "agent-move-ursus",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-ursus", To: &vttv1.GridPosition{X: 5, Y: 5},
		}},
	}, agent.id, true, all)
	tm, ok := env.Payload.(*vttv1.Envelope_TokenMoved)
	if !ok {
		t.Fatalf("agent-move-ursus: payload = %T, want TokenMoved", env.Payload)
	}
	if tm.TokenMoved.GetTokenId() != "tok-ursus" {
		t.Fatalf("agent-move-ursus: TokenId = %q, want tok-ursus", tm.TokenMoved.GetTokenId())
	}
	dmLive = append(dmLive, env)
	retractSeq := env.Sequence

	// Unlike every other command, RetractEvents does not flow through
	// gateway.ToEvent — campaign.Undo (internal/campaign/campaign.go)
	// constructs the EventsRetracted marker itself (ToEvent deliberately
	// never builds one; see ErrIsRetraction's doc comment). Attribution is
	// still gateway-supplied: handleRetraction passes the issuing
	// participant's role and id through to Undo's trailing params, so the
	// marker carries the AGENT's participant_id and ActorRole here, same as
	// every other command.
	env = issueAndVerify(t, agent, &vttv1.ClientCommand{
		RequestId: "agent-retract-move",
		Command: &vttv1.ClientCommand_RetractEvents{RetractEvents: &vttv1.RetractEvents{
			FromSequence: retractSeq, ToSequence: retractSeq, Reason: "test retraction",
		}},
	}, agent.id, false, all)
	er, ok := env.Payload.(*vttv1.Envelope_EventsRetracted)
	if !ok {
		t.Fatalf("agent-retract-move: payload = %T, want EventsRetracted", env.Payload)
	}
	if er.EventsRetracted.GetFromSequence() != retractSeq || er.EventsRetracted.GetToSequence() != retractSeq {
		t.Fatalf("agent-retract-move: range = [%d,%d], want [%d,%d]",
			er.EventsRetracted.GetFromSequence(), er.EventsRetracted.GetToSequence(), retractSeq, retractSeq)
	}
	dmLive = append(dmLive, env)

	// --- Spectator: StartSession denied, no broadcast ---

	expectDenied(t, spectator, &vttv1.ClientCommand{
		RequestId: "spectator-start-session-denied",
		Command:   &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "hijack"}},
	}, all)

	// --- DM: EndSession ---

	env = issueAndVerify(t, dm, &vttv1.ClientCommand{
		RequestId: "dm-end-session",
		Command:   &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}},
	}, dm.id, true, all)
	if _, ok := env.Payload.(*vttv1.Envelope_SessionEnded); !ok {
		t.Fatalf("dm-end-session: payload = %T, want SessionEnded", env.Payload)
	}
	dmLive = append(dmLive, env)

	// --- All clients disconnect ---

	for _, p := range all {
		p.conn.conn.CloseNow()
	}

	// --- Player reconnects with after=0: full catch-up ---

	playerReconn := f.dial(f.playerToken, 0)
	playerCatchup := drainEnvelopes(t, playerReconn, len(dmLive))
	playerReconn.assertNoFrameWithin(t, 300*time.Millisecond) // exactly len(dmLive), nothing more

	// A brand-new, never-before-connected client (also after=0) must see the
	// identical catch-up replay — "full catch-up equals what a fresh
	// subscriber sees".
	freshConn := f.dial(f.spectatorToken, 0)
	freshCatchup := drainEnvelopes(t, freshConn, len(dmLive))
	freshConn.assertNoFrameWithin(t, 300*time.Millisecond)

	if len(playerCatchup) != len(dmLive) {
		t.Fatalf("player catch-up length = %d, want %d (what DM received live)", len(playerCatchup), len(dmLive))
	}
	if len(freshCatchup) != len(dmLive) {
		t.Fatalf("fresh subscriber catch-up length = %d, want %d (what DM received live)", len(freshCatchup), len(dmLive))
	}

	// Final assertion (per-event, not spot-checked): the live DM sequence,
	// the reconnected player's catch-up, and a fresh subscriber's catch-up
	// must all agree — order, event_id, sequence, AND participant_id — for
	// every single one of the len(dmLive) envelopes.
	for i, want := range dmLive {
		pc, fc := playerCatchup[i], freshCatchup[i]
		if pc.EventId != want.EventId || pc.Sequence != want.Sequence {
			t.Fatalf("catch-up[%d] (player) = (event_id=%s, seq=%d), want (%s, %d) matching DM's live sequence",
				i, pc.EventId, pc.Sequence, want.EventId, want.Sequence)
		}
		if pc.ParticipantId != want.ParticipantId {
			t.Fatalf("catch-up[%d] (player) ParticipantId = %q, want %q matching DM's live sequence",
				i, pc.ParticipantId, want.ParticipantId)
		}
		if fc.EventId != want.EventId || fc.Sequence != want.Sequence {
			t.Fatalf("catch-up[%d] (fresh subscriber) = (event_id=%s, seq=%d), want (%s, %d) matching DM's live sequence",
				i, fc.EventId, fc.Sequence, want.EventId, want.Sequence)
		}
		if fc.ParticipantId != want.ParticipantId {
			t.Fatalf("catch-up[%d] (fresh subscriber) ParticipantId = %q, want %q matching DM's live sequence",
				i, fc.ParticipantId, want.ParticipantId)
		}
		// Direct pairwise check too (not merely transitive through dmLive):
		// player's catch-up must equal the fresh subscriber's catch-up.
		if pc.EventId != fc.EventId || pc.Sequence != fc.Sequence || pc.ParticipantId != fc.ParticipantId {
			t.Fatalf("catch-up[%d]: player (event_id=%s, seq=%d, participant=%q) != fresh subscriber (%s, %d, %q)",
				i, pc.EventId, pc.Sequence, pc.ParticipantId, fc.EventId, fc.Sequence, fc.ParticipantId)
		}
	}

	// Sequence numbers themselves must be strictly increasing 1..N — the
	// scenario appended exactly len(dmLive) events/markers and nothing else.
	for i, want := range dmLive {
		if want.Sequence != int64(i+1) {
			t.Fatalf("dmLive[%d].Sequence = %d, want %d", i, want.Sequence, i+1)
		}
	}
}
