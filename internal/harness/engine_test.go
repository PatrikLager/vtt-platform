package harness_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// --- scripted fake Conn -----------------------------------------------------
//
// fakeConn is a fully test-scripted harness.Conn: SendCommand's behavior is
// whatever sendFunc does, which — because every fakeConn in a test's "world"
// shares that world map — can push a broadcast envelope onto ANY of them
// (including its own), or none at all. RunScenario never knows the
// difference between this and a real *harness.Client; it only ever sees the
// harness.Conn interface.
type fakeConn struct {
	name   string
	events chan *vttv1.Envelope
	send   func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error)

	// closeErr lets a test script WHY the stream ended, not merely that it
	// did — see CloseErr below.
	closeErr error

	// mu guards closed AND the send on events, so Close can never close the
	// channel out from under an in-flight broadcast.
	mu     sync.Mutex
	closed bool
}

func newFakeConn(name string) *fakeConn {
	return &fakeConn{name: name, events: make(chan *vttv1.Envelope, 16)}
}

func (c *fakeConn) SendCommand(_ context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	if c.send == nil {
		return nil, fmt.Errorf("fakeConn %s: no send scripted", c.name)
	}
	return c.send(cmd)
}

func (c *fakeConn) Events() <-chan *vttv1.Envelope { return c.events }

// Close honours the Conn contract that Client documents and this fake had
// been quietly breaking: "Events() ... is closed when the connection ends
// (server close, local Close, or a protocol/overflow error)". The old Close
// set a flag and left the channel open forever, so any consumer ranging over
// Events() — RunSoak's per-participant drain goroutines, for instance —
// blocked for the life of the process. Invisible under `go test`, which does
// not fail on leaked goroutines; a synctest bubble refuses to exit with them
// and is how this was found.
//
// Idempotent: RunScenario's reconnect path closes a connection the test's own
// t.Cleanup may close again.
func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.events)
	}
	return nil
}

// trySend delivers env unless the connection is closed, and drops it if so.
// A fake gateway must behave like a real one here: the soak's checkpoint
// dials a throwaway connection and closes it while the world still holds a
// reference, and a real server drops writes to a departed socket rather than
// crashing. Holding mu across the send is safe because every channel here is
// buffered far beyond what any scenario emits.
func (c *fakeConn) trySend(env *vttv1.Envelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.events <- env:
	default:
		// A full buffer is a fixture bug, and BLOCKING on it here would be
		// the worst possible way to find out: sync.Mutex.Lock is explicitly
		// not "durably blocked" for synctest's purposes, so a sender parked
		// on a full channel while holding mu stops the bubble advancing its
		// clock AND stops it raising its deadlock panic. The run would hang
		// to the package test timeout with no synctest diagnostic at all.
		// Fail loudly and immediately instead.
		panic(fmt.Sprintf("fakeConn %s: events buffer full (cap %d) — a fixture is emitting "+
			"more than it declared room for; raise the buffer rather than letting this block",
			c.name, cap(c.events)))
	}
}

// isClosed reports whether Close has been called, for the tests that assert
// on reconnect behaviour.
func (c *fakeConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// closeErr, when set, is what CloseErr reports — so a test can script the
// stream ending for a REASON (an overflow disconnect) rather than merely
// ending, which is the distinction harness.Conn.CloseErr exists to make.
func (f *fakeConn) CloseErr() error { return f.closeErr }

var _ harness.Conn = (*fakeConn)(nil)

// broadcast pushes env onto every named conn's events channel (buffered 16 —
// ample for these scripted, single-digit-event scenarios).
func broadcast(world map[string]*fakeConn, env *vttv1.Envelope, to ...string) {
	for _, name := range to {
		world[name].trySend(env)
	}
}

// okSend scripts a SendCommand that accepts the command (ok=true, the given
// sequence) and broadcasts env to precisely the participants named in to —
// letting a test omit one deliberately, to prove the engine notices.
func okSend(world map[string]*fakeConn, seq int64, env *vttv1.Envelope, to ...string) func(*vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	return func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		env.Sequence = seq
		broadcast(world, env, to...)
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: seq}, nil
	}
}

// denySend scripts a SendCommand that rejects the command (ok=false, errMsg)
// and — if leakTo is non-empty — ALSO broadcasts leakEnv to those
// participants anyway, letting a test prove the engine catches that leak.
func denySend(errMsg string, world map[string]*fakeConn, leakEnv *vttv1.Envelope, leakTo ...string) func(*vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	return func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		if len(leakTo) > 0 {
			broadcast(world, leakEnv, leakTo...)
		}
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: errMsg}, nil
	}
}

func sessionStartedEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId: id,
		Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s1"}},
	}
}

func sceneCreatedEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId: id,
		Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10,
		}},
	}
}

// --- ok-step: pass / fail --------------------------------------------------

func TestRunScenarioOKStepPassesWhenAllParticipantsObserveBroadcast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		watcher := newFakeConn("watcher")
		world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
		dm.send = okSend(world, 1, sessionStartedEnv("ev-1"), "dm", "watcher")

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "watcher"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("Report.Pass = false, want true; steps = %+v", rep.Steps)
		}
		if len(rep.Steps) != 1 || !rep.Steps[0].Pass {
			t.Fatalf("Steps = %+v, want one passing step", rep.Steps)
		}
	})
}

func TestRunScenarioOKStepFailsWhenBroadcastOmittedForOneParticipant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		watcher := newFakeConn("watcher")
		world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
		// Deliberately omit "watcher" from the broadcast target list.
		dm.send = okSend(world, 1, sessionStartedEnv("ev-1"), "dm")

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "watcher"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, want false (watcher never saw the broadcast)")
		}
		if len(rep.Steps) != 1 || rep.Steps[0].Pass {
			t.Fatalf("Steps = %+v, want one failing step", rep.Steps)
		}
		if !strings.Contains(rep.Steps[0].Detail, "watcher") {
			t.Fatalf("Steps[0].Detail = %q, want it to name \"watcher\"", rep.Steps[0].Detail)
		}
	})
}

// --- denial: pass / fail ----------------------------------------------------

func TestRunScenarioDenialStepPassesWithNoBroadcast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		player.send = denySend("not authorized: token has no controller", world, nil)

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "player", Command: rawCmd(t, `{"moveToken":{"tokenId":"t","to":{"x":1,"y":1}}}`),
					Expect: &harness.Expect{DeniedContaining: "not authorized"}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("Report.Pass = false, want true; steps = %+v", rep.Steps)
		}
	})
}

func TestRunScenarioDenialStepFailsWhenBroadcastLeaksAnyway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		// Deny the command but leak a broadcast to "dm" anyway.
		player.send = denySend("not authorized", world, sessionStartedEnv("leak-1"), "dm")

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "player", Command: rawCmd(t, `{"moveToken":{"tokenId":"t","to":{"x":1,"y":1}}}`),
					Expect: &harness.Expect{DeniedContaining: "not authorized"}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, want false (dm observed a leaked broadcast on a denied command)")
		}
		if !strings.Contains(rep.Steps[0].Detail, "dm") {
			t.Fatalf("Steps[0].Detail = %q, want it to name \"dm\"", rep.Steps[0].Detail)
		}
	})
}

// --- reconnect: pass / fail -------------------------------------------------

func TestRunScenarioReconnectCatchUpEqualityPasses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		env1 := sessionStartedEnv("ev-1")
		env2 := sceneCreatedEnv("ev-2")
		dm.send = sequencedSend(world, []scriptedResult{
			{seq: 1, env: env1, to: []string{"dm", "player"}},
			{seq: 2, env: env2, to: []string{"dm", "player"}},
		})

		// The redial for "player" replays exactly what it saw live.
		reconnected := newFakeConn("player-reconnected")
		reconnected.events <- env1
		reconnected.events <- env2

		dial := func(name string, after int64) (harness.Conn, error) {
			if name == "player" && after == 0 {
				// First call (scenario start) uses the plain fixed dialer path;
				// distinguish by whether this is the reconnect (after Close).
				if player.isClosed() {
					return reconnected, nil
				}
			}
			return fixedDialer(world)(name, after)
		}

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
				{By: "dm", Command: rawCmd(t, `{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`), Expect: &harness.Expect{OK: true}},
				{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: 0}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("Report.Pass = false, want true; steps = %+v", rep.Steps)
		}
		if len(rep.Steps) != 3 || !rep.Steps[2].Pass {
			t.Fatalf("reconnect step = %+v, want Pass=true", rep.Steps[2])
		}
	})
}

func TestRunScenarioReconnectCatchUpEqualityFailsOnMismatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		env1 := sessionStartedEnv("ev-1")
		env2 := sceneCreatedEnv("ev-2")
		dm.send = sequencedSend(world, []scriptedResult{
			{seq: 1, env: env1, to: []string{"dm", "player"}},
			{seq: 2, env: env2, to: []string{"dm", "player"}},
		})

		// The redial replays the WRONG second event (different event_id) —
		// same count, so this fails on content, not on a timeout.
		reconnected := newFakeConn("player-reconnected")
		reconnected.events <- env1
		reconnected.events <- sceneCreatedEnv("ev-2-CORRUPTED")

		dial := func(name string, after int64) (harness.Conn, error) {
			if name == "player" && after == 0 && player.isClosed() {
				return reconnected, nil
			}
			return fixedDialer(world)(name, after)
		}

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
				{By: "dm", Command: rawCmd(t, `{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`), Expect: &harness.Expect{OK: true}},
				{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: 0}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, want false (catch-up replayed a corrupted event_id)")
		}
		if rep.Steps[2].Pass {
			t.Fatalf("reconnect step = %+v, want Pass=false", rep.Steps[2])
		}
	})
}

// --- participant-id placeholder resolution (P6 Task 4 fix round) -----------

// TestRunScenarioResolvesParticipantIDPlaceholderBeforeDispatch proves
// {{id:<name>}} inside a command step's JSON is resolved to ids[<name>]
// BEFORE the command ever reaches Conn.SendCommand — the fake Conn's
// scripted send captures the actually-dispatched command and asserts its
// controllerId field is the real, resolved id, never the literal
// placeholder text.
func TestRunScenarioResolvesParticipantIDPlaceholderBeforeDispatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		world := map[string]*fakeConn{"dm": dm}

		const wantID = "real-participant-id-abc123"
		var gotControllerID string
		dm.send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
			gotControllerID = cmd.GetAddActor().GetActor().GetControllerId()
			env := &vttv1.Envelope{EventId: "e1", Sequence: 1, Payload: &vttv1.Envelope_ActorAdded{
				ActorAdded: &vttv1.ActorAdded{Actor: cmd.GetAddActor().GetActor()},
			}}
			broadcast(world, env, "dm")
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
		}

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"addActor":{"actor":{"actorId":"act-1","name":"X","controllerId":"{{id:lera}}"}}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		ids := map[string]string{"lera": wantID}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), ids, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("Report.Pass = false, want true; steps = %+v", rep.Steps)
		}
		if gotControllerID != wantID {
			t.Fatalf("dispatched command's controllerId = %q, want the resolved id %q (placeholder must resolve before dispatch, not be sent literally)", gotControllerID, wantID)
		}
	})
}

// TestRunScenarioErrorsOnUnresolvedParticipantIDPlaceholder proves a
// {{id:<name>}} referencing a name missing from ids is a framework-level
// error (RunScenario returns it directly, nil report) raised BEFORE any
// participant is dialed or any command dispatched — dm.send is left
// unscripted (nil), so a call would panic/error immediately if resolution
// let the run proceed that far.
func TestRunScenarioErrorsOnUnresolvedParticipantIDPlaceholder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		world := map[string]*fakeConn{"dm": dm}

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"addActor":{"actor":{"actorId":"act-1","name":"X","controllerId":"{{id:nobody}}"}}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err == nil {
			t.Fatalf("RunScenario: want a non-nil framework error for an unresolved {{id:nobody}} placeholder, got nil (report=%+v)", rep)
		}
		if !strings.Contains(err.Error(), "nobody") {
			t.Fatalf("RunScenario error = %q, want it to name the unresolved participant %q", err.Error(), "nobody")
		}
		if rep != nil {
			t.Fatalf("RunScenario: want nil report on a framework-level resolution error, got %+v", rep)
		}
	})
}

// --- fresh-campaign assumption ----------------------------------------------

// TestRunScenarioErrorsOnPreExistingCatchUpEvents proves a non-fresh
// campaign — one whose after=0 initial catch-up delivers events BEFORE
// RunScenario ever dispatches a command — is reported as a clear, named
// framework error, not left to surface as a confusing step-level
// observe-mismatch. Before the fix, the pre-existing envelope sits ahead of
// the step's own broadcast in dm's Events() channel, so observeOnAll reads
// IT first, sees the wrong Sequence, and reports "event (sequence 2) not
// observed matching by: dm" — a correct-sounding but misleading diagnosis
// that hides the real problem (the scenario format assumes absolute
// sequence numbers from a fresh campaign; relative-sequence scenarios are a
// planned format extension, not implemented).
func TestRunScenarioErrorsOnPreExistingCatchUpEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		world := map[string]*fakeConn{"dm": dm}
		// Simulate a non-fresh campaign: one event already queued on dm's
		// connection at dial time — exactly what a real gateway's after=0
		// catch-up replay of prior history would deliver before any command has
		// been issued.
		dm.events <- sessionStartedEnv("pre-existing-1")
		dm.send = okSend(world, 2, sceneCreatedEnv("ev-2"), "dm")

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err == nil {
			t.Fatalf("RunScenario: want a framework error for a non-fresh campaign, got nil (report=%+v)", rep)
		}
		if !strings.Contains(err.Error(), "fresh campaign") {
			t.Fatalf("RunScenario error = %q, want it to name the fresh-campaign requirement", err.Error())
		}
		if !strings.Contains(err.Error(), "1 pre-existing") {
			t.Fatalf("RunScenario error = %q, want it to report the count of pre-existing events", err.Error())
		}
		if rep != nil {
			t.Fatalf("RunScenario: want nil report on a framework-level fresh-campaign error, got %+v", rep)
		}
	})
}

// --- probes: pass / fail per kind -------------------------------------------

// runMiniScenario drives a single "dm" participant through StartSession,
// CreateScene, AddActor, and PlaceToken (tok-1 at act-1 in scn-1, (3,4)),
// then evaluates probes against the resulting state — the shared fixture
// every probe subtest below builds on.
func runMiniScenario(t *testing.T, probes []harness.Probe) *harness.Report {
	t.Helper()
	dm := newFakeConn("dm")
	world := map[string]*fakeConn{"dm": dm}

	dm.send = sequencedSend(world, []scriptedResult{
		{seq: 1, env: &vttv1.Envelope{EventId: "e1", Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "s1"}}}, to: []string{"dm"}},
		{seq: 2, env: &vttv1.Envelope{EventId: "e2", Payload: &vttv1.Envelope_SceneCreated{
			SceneCreated: &vttv1.SceneCreated{SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10}}}, to: []string{"dm"}},
		{seq: 3, env: &vttv1.Envelope{EventId: "e3", Payload: &vttv1.Envelope_ActorAdded{
			ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-1", Name: "Ursus",
				Resources: map[string]*vttv1.Resource{"vigor": {Current: 7, Max: 10}}}}}}, to: []string{"dm"}},
		{seq: 4, env: &vttv1.Envelope{EventId: "e4", Payload: &vttv1.Envelope_TokenPlaced{
			TokenPlaced: &vttv1.TokenPlaced{TokenId: "tok-1", SceneId: "scn-1", ActorId: "act-1",
				Position: &vttv1.GridPosition{X: 3, Y: 4}}}}, to: []string{"dm"}},
		// A rules event so resourceAt/hasCondition probes have concrete state
		// to assert against. The harness fake decouples the sent command from
		// the scripted broadcast (exactly as the ActorAdded step above does),
		// so a use_ability command drives a ConditionApplied into the fold.
		{seq: 5, env: &vttv1.Envelope{EventId: "e5", Payload: &vttv1.Envelope_ConditionApplied{
			ConditionApplied: &vttv1.ConditionApplied{ActorId: "act-1", ConditionId: "dazed", Source: "test"}}}, to: []string{"dm"}},
		// A NoteUpserted so noteAt probes have concrete state to assert
		// against (world-layer Task 3 — same "decoupled command vs.
		// broadcast" shape as the rules event above: the sent command is an
		// upsertNote, matched by sequence position, not content).
		{seq: 6, env: &vttv1.Envelope{EventId: "e6", Payload: &vttv1.Envelope_NoteUpserted{
			NoteUpserted: &vttv1.NoteUpserted{Key: "kobold-den", Title: "Kobold Den", Text: "Three kobolds guard the east tunnel."}}}, to: []string{"dm"}},
	})

	sc := &harness.Scenario{
		Participants: []harness.Participant{{Name: "dm"}},
		Steps: []harness.Step{
			{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"addActor":{"actor":{"actorId":"act-1","name":"Ursus"}}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"placeToken":{"tokenId":"tok-1","sceneId":"scn-1","actorId":"act-1","position":{"x":3,"y":4}}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"useAbility":{"actorId":"act-1","abilityId":"daze","targetIds":["act-1"]}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"upsertNote":{"key":"kobold-den","title":"Kobold Den","text":"Three kobolds guard the east tunnel."}}`), Expect: &harness.Expect{OK: true}},
		},
		Probes: probes,
	}
	rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	for i, sr := range rep.Steps {
		if !sr.Pass {
			t.Fatalf("setup step %d unexpectedly failed: %+v", i, sr)
		}
	}
	return rep
}

func TestRunScenarioProbesPerKind(t *testing.T) {
	t.Run("tokenAt pass", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{TokenAt: &harness.TokenAtProbe{TokenId: "tok-1", X: 3, Y: 4}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("tokenAt probe = %+v, want Pass=true", rep.Probes[0])
			}
		})
	})
	t.Run("tokenAt fail", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{TokenAt: &harness.TokenAtProbe{TokenId: "tok-1", X: 9, Y: 9}}})
			if rep.Probes[0].Pass {
				t.Fatalf("tokenAt probe = %+v, want Pass=false (wrong position)", rep.Probes[0])
			}
		})
	})
	t.Run("sessionCount pass", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{SessionCount: &harness.SessionCountProbe{Open: 1, Total: 1}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("sessionCount probe = %+v, want Pass=true", rep.Probes[0])
			}
		})
	})
	t.Run("sessionCount fail", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{SessionCount: &harness.SessionCountProbe{Open: 0, Total: 1}}})
			if rep.Probes[0].Pass {
				t.Fatalf("sessionCount probe = %+v, want Pass=false (session is still open)", rep.Probes[0])
			}
		})
	})
	t.Run("actorExists pass", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{ActorExists: &harness.ActorExistsProbe{ActorId: "act-1"}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("actorExists probe = %+v, want Pass=true", rep.Probes[0])
			}
		})
	})
	t.Run("actorExists fail", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{ActorExists: &harness.ActorExistsProbe{ActorId: "act-nonexistent"}}})
			if rep.Probes[0].Pass {
				t.Fatalf("actorExists probe = %+v, want Pass=false (actor was never added)", rep.Probes[0])
			}
		})
	})
	t.Run("resourceAt pass", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{ResourceAt: &harness.ResourceAtProbe{ActorId: "act-1", Resource: "vigor", Value: 7}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("resourceAt probe = %+v, want Pass=true (vigor is 7)", rep.Probes[0])
			}
		})
	})
	t.Run("resourceAt fail", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{ResourceAt: &harness.ResourceAtProbe{ActorId: "act-1", Resource: "vigor", Value: 99}}})
			if rep.Probes[0].Pass {
				t.Fatalf("resourceAt probe = %+v, want Pass=false (vigor is 7, not 99)", rep.Probes[0])
			}
		})
	})
	t.Run("hasCondition pass", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{HasCondition: &harness.HasConditionProbe{ActorId: "act-1", ConditionId: "dazed", Present: true}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("hasCondition probe = %+v, want Pass=true (dazed is present)", rep.Probes[0])
			}
		})
	})
	t.Run("hasCondition fail", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{HasCondition: &harness.HasConditionProbe{ActorId: "act-1", ConditionId: "dazed", Present: false}}})
			if rep.Probes[0].Pass {
				t.Fatalf("hasCondition probe = %+v, want Pass=false (dazed IS present, probe wanted absent)", rep.Probes[0])
			}
		})
	})
	t.Run("noteAt pass bare key", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{NoteAt: &harness.NoteAtProbe{Key: "kobold-den"}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("noteAt probe = %+v, want Pass=true (key present, no title/text filter)", rep.Probes[0])
			}
		})
	})
	t.Run("noteAt pass titleIs and textContains", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{NoteAt: &harness.NoteAtProbe{
				Key: "kobold-den", TitleIs: "Kobold Den", TextContains: "east tunnel",
			}}})
			if !rep.Probes[0].Pass {
				t.Fatalf("noteAt probe = %+v, want Pass=true (title and text substring both match)", rep.Probes[0])
			}
		})
	})
	t.Run("noteAt fail wrong titleIs", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{NoteAt: &harness.NoteAtProbe{Key: "kobold-den", TitleIs: "Goblin Warren"}}})
			if rep.Probes[0].Pass {
				t.Fatalf("noteAt probe = %+v, want Pass=false (title is \"Kobold Den\", not \"Goblin Warren\")", rep.Probes[0])
			}
		})
	})
	t.Run("noteAt fail wrong textContains", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{NoteAt: &harness.NoteAtProbe{Key: "kobold-den", TextContains: "west bridge"}}})
			if rep.Probes[0].Pass {
				t.Fatalf("noteAt probe = %+v, want Pass=false (text does not contain \"west bridge\")", rep.Probes[0])
			}
		})
	})
	t.Run("noteAt fail absent key", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rep := runMiniScenario(t, []harness.Probe{{NoteAt: &harness.NoteAtProbe{Key: "no-such-note"}}})
			if rep.Probes[0].Pass {
				t.Fatalf("noteAt probe = %+v, want Pass=false (key was never upserted)", rep.Probes[0])
			}
		})
	})
}

// --- shared test helpers -----------------------------------------------------

// rawCmd is a protojson ClientCommand fragment (the oneof case only, no
// requestId — RunScenario assigns one) as raw JSON, matching the shape
// LoadScenario would hand a Step.Command field.
func rawCmd(t *testing.T, oneofJSON string) []byte {
	t.Helper()
	return []byte(oneofJSON)
}

// fixedDialer returns every participant's already-built fakeConn from
// world, ignoring after (world's conns are pre-scripted for after=0 in
// every test that uses this directly — reconnect tests build a bespoke
// Dialer instead, see TestRunScenarioReconnect*).
func fixedDialer(world map[string]*fakeConn) harness.Dialer {
	return func(name string, after int64) (harness.Conn, error) {
		c, ok := world[name]
		if !ok {
			return nil, fmt.Errorf("fixedDialer: no fakeConn for participant %q", name)
		}
		return c, nil
	}
}

// scriptedResult is one accepted command's canned outcome for sequencedSend.
type scriptedResult struct {
	seq int64
	env *vttv1.Envelope
	to  []string
}

// sequencedSend scripts a SendCommand that walks through results in order,
// one per call — the shape every multi-step "ok" scenario in this file
// needs (each step's SendCommand call consumes the next scripted result).
func sequencedSend(world map[string]*fakeConn, results []scriptedResult) func(*vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	i := 0
	return func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		if i >= len(results) {
			return nil, fmt.Errorf("sequencedSend: no scripted result left for call %d", i)
		}
		r := results[i]
		i++
		r.env.Sequence = r.seq
		broadcast(world, r.env, r.to...)
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: r.seq}, nil
	}
}

// --- a dead stream is not evidence -----------------------------------------

// A denial passes on ABSENCE: nothing was broadcast to anyone within
// denialAbsenceWindow. That inference holds only for connections that could
// have received something. A participant torn down for overflow is silent for
// an unrelated reason, and counting its silence as proof lets the assertion
// pass vacuously — under exactly the CI load that makes overflows happen.
func TestRunScenarioDenialIsUnprovableWhenAParticipantsStreamDied(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		dm.send = denySend("not authorized: token has no controller", world, nil)

		// player was disconnected for reading too slowly before the denial
		// ran. It is deliberately NOT participant 0: that one is the freshness
		// probe, which now refuses to run against a dead stream of its own
		// accord, and this test is about the denial assertion specifically.
		player.closeErr = harness.ErrEventsOverflow
		_ = player.Close()

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"moveToken":{"tokenId":"t","to":{"x":1,"y":1}}}`),
					Expect: &harness.Expect{DeniedContaining: "not authorized"}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, but player could not have received anything — " +
				"its silence proves nothing about what the denied command broadcast")
		}
		if !strings.Contains(rep.Steps[0].Detail, "player") {
			t.Fatalf("Steps[0].Detail = %q, want it to name the participant whose stream ended", rep.Steps[0].Detail)
		}
		if !strings.Contains(rep.Steps[0].Detail, "unprovable") {
			t.Fatalf("Steps[0].Detail = %q, want it to say the assertion is unprovable rather than "+
				"reporting a leak that did not happen", rep.Steps[0].Detail)
		}
	})
}

// The accepted-step twin: "not observed matching by: dm" sends the reader
// after a broadcast that may have been perfect. The stream ending is a
// different fault with a different fix, so it is reported as one.
func TestRunScenarioSaysUnobservableRatherThanNotObservedWhenAStreamDied(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		dm.send = okSend(world, 1, sessionStartedEnv("e-1"), "dm", "player")

		// The dead one is an OBSERVER, not participant 0 (the freshness probe).
		player.closeErr = harness.ErrEventsOverflow
		_ = player.Close()

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"moveToken":{"tokenId":"t","to":{"x":1,"y":1}}}`),
					Expect: &harness.Expect{OK: true}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, want false: player's stream was gone")
		}
		if !strings.Contains(rep.Steps[0].Detail, "unobservable") {
			t.Fatalf("Steps[0].Detail = %q, want the ended stream named as such rather than "+
				"a plain 'not observed'", rep.Steps[0].Detail)
		}
	})
}

// The reconnect catch-up's short-read report said "before timing out" for a
// stream that had CLOSED — the misdiagnosis written into a variable name
// (`timedOut = true` on !ok). A redial whose connection dies mid-replay is a
// different fault from a server that stopped sending, and observeTimeout is
// not what elapsed.
func TestRunScenarioReconnectCatchUpNamesADeadStreamNotATimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		env1 := sessionStartedEnv("ev-1")
		env2 := sceneCreatedEnv("ev-2")
		dm.send = sequencedSend(world, []scriptedResult{
			{seq: 1, env: env1, to: []string{"dm", "player"}},
			{seq: 2, env: env2, to: []string{"dm", "player"}},
		})

		// The redial replays the first event, then the stream is torn down
		// for overflow before the second.
		reconnected := newFakeConn("player-reconnected")
		reconnected.events <- env1
		reconnected.closeErr = harness.ErrEventsOverflow
		_ = reconnected.Close()

		dial := func(name string, after int64) (harness.Conn, error) {
			if name == "player" && after == 0 && player.isClosed() {
				return reconnected, nil
			}
			return fixedDialer(world)(name, after)
		}

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
				{By: "dm", Command: rawCmd(t, `{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`), Expect: &harness.Expect{OK: true}},
				{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: 0}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, want false: the catch-up never delivered event 2")
		}
		detail := rep.Steps[2].Detail
		if strings.Contains(detail, "timing out") {
			t.Fatalf("Steps[2].Detail = %q — nothing timed out; the stream was torn down", detail)
		}
		if !strings.Contains(detail, "overflow") {
			t.Fatalf("Steps[2].Detail = %q, want the overflow named", detail)
		}
	})
}

// A reconnect at the current head has an EMPTY `want`: it asserts "nothing
// replays". That is another proof by absence, and the catch-up loop only
// consults CloseErr from inside `for range want` — which never runs. The tail
// check then reads a closed channel as `ok == false`, i.e. "nothing extra
// arrived", and certifies the assertion against a socket that was already
// dead.
func TestRunScenarioReconnectAtHeadStillNoticesADeadRedial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		env1 := sessionStartedEnv("ev-1")
		dm.send = sequencedSend(world, []scriptedResult{
			{seq: 1, env: env1, to: []string{"dm", "player"}},
		})

		// Redialled at the head, so nothing is owed — and torn down anyway.
		reconnected := newFakeConn("player-reconnected")
		reconnected.closeErr = harness.ErrEventsOverflow
		_ = reconnected.Close()

		dial := func(name string, after int64) (harness.Conn, error) {
			if name == "player" && player.isClosed() {
				return reconnected, nil
			}
			return fixedDialer(world)(name, after)
		}

		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
				{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: 1}},
			},
		}
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true — 'nothing replayed' was certified by a dead socket, " +
				"which cannot replay anything by construction")
		}
		if !strings.Contains(rep.Steps[1].Detail, "overflow") {
			t.Fatalf("Steps[1].Detail = %q, want the ended stream named", rep.Steps[1].Detail)
		}
	})
}

// Every trailing denial rests its absence claim on the SAME dead participant,
// so every one of them is equally unproven. Marking only the first — the shape
// markLeaked correctly uses, because a leak provably came from exactly one
// denied step — would leave the rest printing pass=true on evidence that does
// not exist. scenarios/denials.json has 14 trailing denials in a row.
func TestRunScenarioMarksEveryTrailingDenialUnprovableNotJustTheFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dm := newFakeConn("dm")
		player := newFakeConn("player")
		world := map[string]*fakeConn{"dm": dm, "player": player}
		dm.send = denySend("not authorized: token has no controller", world, nil)

		player.closeErr = harness.ErrEventsOverflow
		_ = player.Close()

		denied := harness.Step{
			By:      "dm",
			Command: rawCmd(t, `{"moveToken":{"tokenId":"t","to":{"x":1,"y":1}}}`),
			Expect:  &harness.Expect{DeniedContaining: "not authorized"},
		}
		sc := &harness.Scenario{
			Participants: []harness.Participant{{Name: "dm"}, {Name: "player"}},
			Steps:        []harness.Step{denied, denied, denied},
		}
		rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Pass {
			t.Fatal("Report.Pass = true, want false")
		}
		for i, sr := range rep.Steps {
			if sr.Pass {
				t.Fatalf("Steps[%d].Pass = true — its absence claim rests on the same dead "+
					"participant as Steps[0]'s, so it is exactly as unproven", i)
			}
			if !strings.Contains(sr.Detail, "unprovable") {
				t.Fatalf("Steps[%d].Detail = %q, want it to say the claim is unprovable", i, sr.Detail)
			}
		}
	})
}
