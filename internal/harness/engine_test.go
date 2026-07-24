package harness_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

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

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

var _ harness.Conn = (*fakeConn)(nil)

// broadcast pushes env onto every named conn's events channel (buffered 16 —
// ample for these scripted, single-digit-event scenarios).
func broadcast(world map[string]*fakeConn, env *vttv1.Envelope, to ...string) {
	for _, name := range to {
		world[name].events <- env
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
}

func TestRunScenarioOKStepFailsWhenBroadcastOmittedForOneParticipant(t *testing.T) {
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
}

// --- denial: pass / fail ----------------------------------------------------

func TestRunScenarioDenialStepPassesWithNoBroadcast(t *testing.T) {
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
}

func TestRunScenarioDenialStepFailsWhenBroadcastLeaksAnyway(t *testing.T) {
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
}

// --- reconnect: pass / fail -------------------------------------------------

func TestRunScenarioReconnectCatchUpEqualityPasses(t *testing.T) {
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
			if player.closed {
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
}

func TestRunScenarioReconnectCatchUpEqualityFailsOnMismatch(t *testing.T) {
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
		if name == "player" && after == 0 && player.closed {
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
}

// --- participant-id placeholder resolution (P6 Task 4 fix round) -----------

// TestRunScenarioResolvesParticipantIDPlaceholderBeforeDispatch proves
// {{id:<name>}} inside a command step's JSON is resolved to ids[<name>]
// BEFORE the command ever reaches Conn.SendCommand — the fake Conn's
// scripted send captures the actually-dispatched command and asserts its
// controllerId field is the real, resolved id, never the literal
// placeholder text.
func TestRunScenarioResolvesParticipantIDPlaceholderBeforeDispatch(t *testing.T) {
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
}

// TestRunScenarioErrorsOnUnresolvedParticipantIDPlaceholder proves a
// {{id:<name>}} referencing a name missing from ids is a framework-level
// error (RunScenario returns it directly, nil report) raised BEFORE any
// participant is dialed or any command dispatched — dm.send is left
// unscripted (nil), so a call would panic/error immediately if resolution
// let the run proceed that far.
func TestRunScenarioErrorsOnUnresolvedParticipantIDPlaceholder(t *testing.T) {
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
			ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-1", Name: "Ursus"}}}}, to: []string{"dm"}},
		{seq: 4, env: &vttv1.Envelope{EventId: "e4", Payload: &vttv1.Envelope_TokenPlaced{
			TokenPlaced: &vttv1.TokenPlaced{TokenId: "tok-1", SceneId: "scn-1", ActorId: "act-1",
				Position: &vttv1.GridPosition{X: 3, Y: 4}}}}, to: []string{"dm"}},
	})

	sc := &harness.Scenario{
		Participants: []harness.Participant{{Name: "dm"}},
		Steps: []harness.Step{
			{By: "dm", Command: rawCmd(t, `{"startSession":{"name":"s1"}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"addActor":{"actor":{"actorId":"act-1","name":"Ursus"}}}`), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"placeToken":{"tokenId":"tok-1","sceneId":"scn-1","actorId":"act-1","position":{"x":3,"y":4}}}`), Expect: &harness.Expect{OK: true}},
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
		rep := runMiniScenario(t, []harness.Probe{{TokenAt: &harness.TokenAtProbe{TokenId: "tok-1", X: 3, Y: 4}}})
		if !rep.Probes[0].Pass {
			t.Fatalf("tokenAt probe = %+v, want Pass=true", rep.Probes[0])
		}
	})
	t.Run("tokenAt fail", func(t *testing.T) {
		rep := runMiniScenario(t, []harness.Probe{{TokenAt: &harness.TokenAtProbe{TokenId: "tok-1", X: 9, Y: 9}}})
		if rep.Probes[0].Pass {
			t.Fatalf("tokenAt probe = %+v, want Pass=false (wrong position)", rep.Probes[0])
		}
	})
	t.Run("sessionCount pass", func(t *testing.T) {
		rep := runMiniScenario(t, []harness.Probe{{SessionCount: &harness.SessionCountProbe{Open: 1, Total: 1}}})
		if !rep.Probes[0].Pass {
			t.Fatalf("sessionCount probe = %+v, want Pass=true", rep.Probes[0])
		}
	})
	t.Run("sessionCount fail", func(t *testing.T) {
		rep := runMiniScenario(t, []harness.Probe{{SessionCount: &harness.SessionCountProbe{Open: 0, Total: 1}}})
		if rep.Probes[0].Pass {
			t.Fatalf("sessionCount probe = %+v, want Pass=false (session is still open)", rep.Probes[0])
		}
	})
	t.Run("actorExists pass", func(t *testing.T) {
		rep := runMiniScenario(t, []harness.Probe{{ActorExists: &harness.ActorExistsProbe{ActorId: "act-1"}}})
		if !rep.Probes[0].Pass {
			t.Fatalf("actorExists probe = %+v, want Pass=true", rep.Probes[0])
		}
	})
	t.Run("actorExists fail", func(t *testing.T) {
		rep := runMiniScenario(t, []harness.Probe{{ActorExists: &harness.ActorExistsProbe{ActorId: "act-nonexistent"}}})
		if rep.Probes[0].Pass {
			t.Fatalf("actorExists probe = %+v, want Pass=false (actor was never added)", rep.Probes[0])
		}
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
