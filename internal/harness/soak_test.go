package harness_test

// soak_test.go is task-5-brief.md's ADR-009 Step 1: a fake-Conn-driven
// generator determinism test (same seed twice -> byte-identical dispatched
// command sequence), a mix-ratio sanity test (1000 draws land close to the
// brief's pinned percentages), a denied-attempt bookkeeping test, and a
// players-only-move-own invariant test. All four drive RunSoak through its
// real, exported entry point (never an internal function) against a
// self-contained fake "world" — a small, independently re-implemented
// authz table + engine.State fold, the same "deliberately re-documents the
// gateway's wire protocol" shape client_test.go's fake wire server already
// established (this package may not import internal/gateway, even in test
// files — client.go's package comment; internal/engine IS an allowed
// dependency per .go-arch-lint.yml's harness component, so the fake world
// reuses engine.NewState/engine.Apply directly rather than hand-rolling its
// own state derivation).

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// --- the fake world: a general-purpose, stateful authz+fold simulator ------

// soakCommandRoles is THIS FILE's own independent copy of
// internal/gateway/authz.go's commandRoles table (same values) — duplicated
// deliberately, not imported (harness test files may not depend on
// internal/gateway).
var soakCommandRoles = map[string]map[string]bool{
	"move_token":     {"dm": true, "agent": true, "player": true},
	"create_scene":   {"dm": true, "agent": true},
	"add_actor":      {"dm": true, "agent": true},
	"place_token":    {"dm": true, "agent": true},
	"start_session":  {"dm": true, "agent": true},
	"end_session":    {"dm": true, "agent": true},
	"retract_events": {"dm": true, "agent": true},
}

func soakCommandKind(cmd *vttv1.ClientCommand) string {
	switch cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		return "move_token"
	case *vttv1.ClientCommand_CreateScene:
		return "create_scene"
	case *vttv1.ClientCommand_AddActor:
		return "add_actor"
	case *vttv1.ClientCommand_PlaceToken:
		return "place_token"
	case *vttv1.ClientCommand_StartSession:
		return "start_session"
	case *vttv1.ClientCommand_EndSession:
		return "end_session"
	case *vttv1.ClientCommand_RetractEvents:
		return "retract_events"
	default:
		return ""
	}
}

// soakWorld is a general-purpose (not single-scripted-sequence, unlike
// engine_test.go's fakeConn.send scripts) fake gateway: it accepts any
// number of commands from any of the four soak participants, decides
// ok/denied via soakCommandRoles + a player-ownership check mirroring
// gateway/authz.go's authorizeTokenOwnership, and folds accepted commands
// into its own engine.State (retraction included, via harness.Fold — the
// exported function under test elsewhere in this package, reused here
// exactly as a real client-side rebuild would).
type soakWorld struct {
	mu          sync.Mutex
	st          *engine.State
	history     []*vttv1.Envelope
	seq         int64
	roles       map[string]string // participant name -> role
	ids         map[string]string // participant name -> id
	conns       []*fakeConn       // every currently-live connection (broadcast targets)
	dispatchLog []string          // protojson of every command received, in dispatch order
}

func newSoakWorld(ids map[string]string) *soakWorld {
	return &soakWorld{
		st: engine.NewState(),
		roles: map[string]string{
			"dm": "dm", "player1": "player", "player2": "player", "agent": "agent",
		},
		ids: ids,
	}
}

// dial implements harness.Dialer: preloads catch-up (every envelope with
// Sequence > after) into a fresh fakeConn, registers it as a broadcast
// target, and wires its send to handle. A generously-sized channel (well
// beyond engine_test.go's 16-capacity default) avoids the fake ever
// blocking mid-catch-up for the event counts these tests use.
func (w *soakWorld) dial(name string, after int64) (harness.Conn, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, known := w.roles[name]; !known {
		return nil, fmt.Errorf("soakWorld: unknown participant %q", name)
	}
	c := newFakeConn(name)
	c.events = make(chan *vttv1.Envelope, 8192)
	for _, env := range w.history {
		if env.Sequence > after {
			c.events <- env
		}
	}
	c.send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		return w.handle(name, cmd), nil
	}
	w.conns = append(w.conns, c)
	return c, nil
}

// handle is the fake's ENTIRE server-side logic: log the dispatched
// command, authorize it (denying with a "gateway: not authorized" message,
// matching internal/gateway/authz.go's ErrUnauthorized text — RunSoak's own
// deniedAttempt bucket checks for exactly that substring), then either fold
// it (plain commands) or apply it as a retraction (RetractEvents).
func (w *soakWorld) handle(name string, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	raw, err := protojson.Marshal(cmd)
	if err != nil {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "soakWorld: marshal: " + err.Error()}
	}
	w.dispatchLog = append(w.dispatchLog, string(raw))

	role := w.roles[name]
	kind := soakCommandKind(cmd)
	allowed, known := soakCommandRoles[kind]
	if !known || !allowed[role] {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false,
			Error: fmt.Sprintf("gateway: not authorized: role %q may not issue %q", role, kind)}
	}
	if kind == "move_token" && role == "player" {
		if errMsg := w.checkOwnership(name, cmd.GetMoveToken()); errMsg != "" {
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: errMsg}
		}
	}

	if r, ok := cmd.GetCommand().(*vttv1.ClientCommand_RetractEvents); ok {
		return w.applyRetraction(cmd.GetRequestId(), r.RetractEvents)
	}

	env := w.toEnvelope(name, cmd)
	if err := engine.Apply(w.st, env); err != nil {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "soakWorld: apply: " + err.Error()}
	}
	w.history = append(w.history, env)
	w.broadcast(env)
	return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: env.Sequence}
}

// checkOwnership mirrors gateway/authz.go's authorizeTokenOwnership: the
// token must exist, its actor must exist, and that actor's controller_id
// must equal the issuing participant's own id — returning "" means
// authorized.
func (w *soakWorld) checkOwnership(name string, req *vttv1.MoveTokenRequest) string {
	tok, ok := w.st.Tokens[req.GetTokenId()]
	if !ok {
		return fmt.Sprintf("gateway: not authorized: unknown token %q", req.GetTokenId())
	}
	actor := w.st.Actors[tok.ActorID]
	myID := w.ids[name]
	if actor.GetControllerId() == "" || actor.GetControllerId() != myID {
		return fmt.Sprintf("gateway: not authorized: token %q is not controlled by participant %q", req.GetTokenId(), myID)
	}
	return ""
}

func (w *soakWorld) toEnvelope(name string, cmd *vttv1.ClientCommand) *vttv1.Envelope {
	w.seq++
	env := &vttv1.Envelope{
		EventId: fmt.Sprintf("ev-%d", w.seq), Sequence: w.seq,
		ParticipantId: w.ids[name], ActorRole: w.roles[name],
	}
	switch c := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		env.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: c.MoveToken.GetTokenId(), To: c.MoveToken.GetTo(),
		}}
	case *vttv1.ClientCommand_CreateScene:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: c.CreateScene.GetSceneId(), Name: c.CreateScene.GetName(),
			GridWidth: c.CreateScene.GetGridWidth(), GridHeight: c.CreateScene.GetGridHeight(),
		}}
	case *vttv1.ClientCommand_AddActor:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: c.AddActor.GetActor()}}
	case *vttv1.ClientCommand_PlaceToken:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: c.PlaceToken.GetTokenId(), SceneId: c.PlaceToken.GetSceneId(),
			ActorId: c.PlaceToken.GetActorId(), Position: c.PlaceToken.GetPosition(),
		}}
	case *vttv1.ClientCommand_StartSession:
		env.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: c.StartSession.GetName()}}
	case *vttv1.ClientCommand_EndSession:
		env.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}
	}
	return env
}

// applyRetraction appends an EventsRetracted marker and rebuilds w.st via
// harness.Fold(w.history) — the exact rebuild a real client-side observer
// would do, dogfooding the package's own published derivation.
func (w *soakWorld) applyRetraction(reqID string, r *vttv1.RetractEvents) *vttv1.CommandResult {
	from, to := r.GetFromSequence(), r.GetToSequence()
	for _, env := range w.history {
		if env.Sequence < from || env.Sequence > to {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			return &vttv1.CommandResult{RequestId: reqID, Ok: false, Error: "soakWorld: cannot retract a retraction"}
		}
	}
	w.seq++
	marker := &vttv1.Envelope{
		EventId: fmt.Sprintf("ev-%d", w.seq), Sequence: w.seq,
		Payload: &vttv1.Envelope_EventsRetracted{EventsRetracted: &vttv1.EventsRetracted{
			FromSequence: from, ToSequence: to, Reason: r.GetReason(),
		}},
	}
	w.history = append(w.history, marker)

	rebuilt, err := harness.Fold(w.history)
	if err != nil {
		return &vttv1.CommandResult{RequestId: reqID, Ok: false, Error: "soakWorld: rebuild after retraction: " + err.Error()}
	}
	w.st = rebuilt
	w.broadcast(marker)
	return &vttv1.CommandResult{RequestId: reqID, Ok: true, Sequence: marker.Sequence}
}

func (w *soakWorld) broadcast(env *vttv1.Envelope) {
	for _, c := range w.conns {
		c.events <- env
	}
}

// --- fixed test ids ----------------------------------------------------------

func soakTestIDs() map[string]string {
	return map[string]string{"player1": "id-player1", "player2": "id-player2"}
}

// --- Step 1: generator determinism -------------------------------------------

// TestRunSoakGeneratorDeterministicForSameSeed is task-5-brief.md's "same
// seed twice -> byte-identical command sequence": two INDEPENDENT worlds,
// same seed, same ids — every dispatched command (protojson bytes,
// including the deterministically-assigned request_id) must match exactly,
// in order, across both runs.
func TestRunSoakGeneratorDeterministicForSameSeed(t *testing.T) {
	const events = 250
	ids := soakTestIDs()

	w1 := newSoakWorld(ids)
	rep1, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 7, Events: events, CheckEvery: events + 1, IDs: ids}, w1.dial, io.Discard)
	if err != nil {
		t.Fatalf("RunSoak (run 1): %v", err)
	}

	w2 := newSoakWorld(ids)
	rep2, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 7, Events: events, CheckEvery: events + 1, IDs: ids}, w2.dial, io.Discard)
	if err != nil {
		t.Fatalf("RunSoak (run 2): %v", err)
	}

	if !rep1.Pass {
		t.Fatalf("run 1: Report.Pass = false, want true: %+v", rep1)
	}
	if !rep2.Pass {
		t.Fatalf("run 2: Report.Pass = false, want true: %+v", rep2)
	}

	if len(w1.dispatchLog) != len(w2.dispatchLog) {
		t.Fatalf("dispatched command count differs: run1=%d run2=%d", len(w1.dispatchLog), len(w2.dispatchLog))
	}
	if len(w1.dispatchLog) != events {
		t.Fatalf("dispatched command count = %d, want %d (one per action)", len(w1.dispatchLog), events)
	}
	for i := range w1.dispatchLog {
		if w1.dispatchLog[i] != w2.dispatchLog[i] {
			t.Fatalf("command %d differs between same-seed runs:\n run1: %s\n run2: %s", i, w1.dispatchLog[i], w2.dispatchLog[i])
		}
	}
}

// TestRunSoakGeneratorDiffersForDifferentSeed is the determinism test's
// converse — proves the byte-identical result above isn't a trivial
// "always identical" degenerate generator: a different seed must diverge
// somewhere in the dispatched sequence.
func TestRunSoakGeneratorDiffersForDifferentSeed(t *testing.T) {
	const events = 250
	ids := soakTestIDs()

	w1 := newSoakWorld(ids)
	if _, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 1, Events: events, CheckEvery: events + 1, IDs: ids}, w1.dial, io.Discard); err != nil {
		t.Fatalf("RunSoak (seed 1): %v", err)
	}
	w2 := newSoakWorld(ids)
	if _, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 2, Events: events, CheckEvery: events + 1, IDs: ids}, w2.dial, io.Discard); err != nil {
		t.Fatalf("RunSoak (seed 2): %v", err)
	}

	identical := len(w1.dispatchLog) == len(w2.dispatchLog)
	if identical {
		for i := range w1.dispatchLog {
			if w1.dispatchLog[i] != w2.dispatchLog[i] {
				identical = false
				break
			}
		}
	}
	if identical {
		t.Fatal("seed=1 and seed=2 produced byte-identical dispatched sequences — the generator isn't actually seeded")
	}
}

// --- Step 1: mix-ratio sanity -------------------------------------------------

// TestRunSoakActionMixRatioSanity is task-5-brief.md's "mix-ratio sanity
// over 1000 draws": Report.Counts, expressed as a fraction of Events, must
// land close to the brief's pinned percentages (create scene 5%, add actor
// 10%, place 15%, move-own 50%, session churn 5%, retraction 10%, deliberate
// authz-denied 5%). Tolerance is generous (±5 points) — this is a sanity
// check on the mix, not a statistical test of the RNG.
func TestRunSoakActionMixRatioSanity(t *testing.T) {
	const events = 1000
	ids := soakTestIDs()
	w := newSoakWorld(ids)

	rep, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 42, Events: events, CheckEvery: events + 1, IDs: ids}, w.dial, io.Discard)
	if err != nil {
		t.Fatalf("RunSoak: %v", err)
	}
	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true: counts=%+v", rep.Counts)
	}

	want := map[string]float64{
		"createScene":   0.05,
		"addActor":      0.10,
		"placeToken":    0.15,
		"moveOwn":       0.50,
		"sessionChurn":  0.05,
		"retraction":    0.10,
		"deniedAttempt": 0.05,
	}
	const tolerance = 0.05
	for kind, wantFrac := range want {
		got := float64(rep.Counts[kind]) / float64(events)
		if diff := got - wantFrac; diff < -tolerance || diff > tolerance {
			t.Errorf("action %q: got fraction %.3f (%d/%d), want ~%.3f ± %.2f: counts=%+v",
				kind, got, rep.Counts[kind], events, wantFrac, tolerance, rep.Counts)
		}
	}

	sum := 0
	for _, n := range rep.Counts {
		sum += n
	}
	if sum != events {
		t.Fatalf("sum of Counts = %d, want %d (every action must land in exactly one bucket): counts=%+v", sum, events, rep.Counts)
	}
}

// --- Step 1: denied-attempt bookkeeping --------------------------------------

// TestRunSoakDeniedAttemptBookkeeping is task-5-brief.md's "denied-attempt
// bookkeeping": Report.Denied must equal Counts["deniedAttempt"] exactly —
// nothing outside the deliberate 5% bucket should ever come back denied —
// and that bucket must actually have fired at least once for the run to
// prove anything.
func TestRunSoakDeniedAttemptBookkeeping(t *testing.T) {
	const events = 500
	ids := soakTestIDs()
	w := newSoakWorld(ids)

	rep, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 3, Events: events, CheckEvery: events + 1, IDs: ids}, w.dial, io.Discard)
	if err != nil {
		t.Fatalf("RunSoak: %v", err)
	}
	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true: %+v", rep)
	}
	if rep.Counts["deniedAttempt"] == 0 {
		t.Fatal("Counts[deniedAttempt] = 0 — this run proves nothing about the deliberate authz-denied bucket")
	}
	if rep.Denied != rep.Counts["deniedAttempt"] {
		t.Fatalf("Denied = %d, Counts[deniedAttempt] = %d, want equal (nothing outside the deliberate bucket should ever be denied)",
			rep.Denied, rep.Counts["deniedAttempt"])
	}
	if rep.Accepted+rep.Denied != events {
		t.Fatalf("Accepted(%d) + Denied(%d) != Events(%d)", rep.Accepted, rep.Denied, events)
	}
}

// --- Step 1: players-only-move-own invariant ---------------------------------

// TestRunSoakPlayersOnlyMoveOwnTokens is task-5-brief.md's "players-only-
// move-own invariant": every ACCEPTED TokenMoved event issued by a player
// (identified by ParticipantId — the fake world stamps the issuer's real id
// the same way gateway/convert.go's ToEvent does) must move a token whose
// actor is controlled by that SAME player — reconstructed here directly
// from the fake world's own final state and full history, independent of
// RunSoak's internal model bookkeeping.
func TestRunSoakPlayersOnlyMoveOwnTokens(t *testing.T) {
	const events = 500
	ids := soakTestIDs()
	w := newSoakWorld(ids)

	rep, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 11, Events: events, CheckEvery: events + 1, IDs: ids}, w.dial, io.Discard)
	if err != nil {
		t.Fatalf("RunSoak: %v", err)
	}
	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true: %+v", rep)
	}

	playerByID := map[string]string{}
	for name, id := range ids {
		playerByID[id] = name
	}

	checked := 0
	for _, env := range w.history {
		tm, ok := env.Payload.(*vttv1.Envelope_TokenMoved)
		if !ok {
			continue
		}
		playerName, isPlayer := playerByID[env.ParticipantId]
		if !isPlayer {
			continue // dm/agent-issued move — no ownership restriction
		}
		tok, ok := w.st.Tokens[tm.TokenMoved.GetTokenId()]
		if !ok {
			t.Fatalf("sequence %d: moved token %q no longer exists in final state", env.Sequence, tm.TokenMoved.GetTokenId())
		}
		actor := w.st.Actors[tok.ActorID]
		if actor.GetControllerId() != env.ParticipantId {
			t.Fatalf("sequence %d: %s moved token %q (actor %q, controller %q) — not their own token",
				env.Sequence, playerName, tm.TokenMoved.GetTokenId(), tok.ActorID, actor.GetControllerId())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no player-issued TokenMoved events observed — this run proves nothing about the invariant")
	}
}

// --- checkpoint mechanics (fake-world smoke) ---------------------------------

// TestRunSoakCheckpointsRunPeriodicallyAndAtEnd proves RunSoak actually
// invokes the checkpoint fold-equality machinery — periodic (every
// CheckEvery accepted events) AND once more at the end — against the fake
// world (the REAL end-to-end proof, against a live gateway, is Step 2's
// cmd-level e2e test).
func TestRunSoakCheckpointsRunPeriodicallyAndAtEnd(t *testing.T) {
	const events = 120
	ids := soakTestIDs()
	w := newSoakWorld(ids)

	var log bytes.Buffer
	rep, err := harness.RunSoak(context.Background(),
		harness.SoakConfig{Seed: 5, Events: events, CheckEvery: 50, IDs: ids}, w.dial, &log)
	if err != nil {
		t.Fatalf("RunSoak: %v", err)
	}
	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true: %+v\nlog:\n%s", rep, log.String())
	}
	if rep.Checkpoints < 2 {
		t.Fatalf("Checkpoints = %d, want >= 2 (at least one periodic + the final one) for %d events at CheckEvery=50: log:\n%s",
			rep.Checkpoints, events, log.String())
	}
}
