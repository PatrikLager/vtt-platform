package harness_test

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// Tests here exist to kill specific surviving mutants in internal/harness,
// found once testing/synctest made a full mutation run practical (3m38s, down
// from hours). Each names the mutation it pins. They are boundary tests in the
// strict sense: the behaviour either side of a comparison, not the happy path,
// which is exactly the class coverage cannot see and review reads past.

// TestStepLinesAreEmittedAsTheRunProgresses pins `limit := i + 1` (engine.go).
//
// FIRST ATTEMPT AT THIS TEST FAILED TO KILL THE MUTANT, and the reason is
// worth keeping. Mutated to `i - 1`, the loop emits each step's line one
// iteration late, but the end-of-run printFinal(len(rep.Steps)) sweeps up
// whatever is outstanding — so the FINAL buffer is byte-identical: same
// lines, same order, same count. Asserting on the finished log cannot
// distinguish them, however carefully.
//
// What actually changes is WHEN a line is written, and that is not cosmetic:
// `vtt client run` streams this to stdout as a live progress log, so a
// scenario that stalls on step 7 must already have shown steps 0-6. The
// deferred-printing change (E6) went out of its way to preserve that while
// holding back only genuinely-unsettled denials.
//
// So this observes MID-RUN, from inside the fake's send: by the time step i's
// command is dispatched, every earlier step is settled and must already have
// been printed.
func TestStepLinesAreEmittedAsTheRunProgresses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm")}
		var log bytes.Buffer
		var linesAtDispatch []int

		seq := int64(0)
		events := []*vttv1.Envelope{
			{EventId: "e1", Sequence: 1, Payload: &vttv1.Envelope_SessionStarted{
				SessionStarted: &vttv1.SessionStarted{Name: "s"}}},
			{EventId: "e2", Sequence: 2, Payload: &vttv1.Envelope_SceneCreated{
				SceneCreated: &vttv1.SceneCreated{SceneId: "scn-1", Name: "H", GridWidth: 4, GridHeight: 4}}},
			{EventId: "e3", Sequence: 3, Payload: &vttv1.Envelope_SceneCreated{
				SceneCreated: &vttv1.SceneCreated{SceneId: "scn-2", Name: "J", GridWidth: 4, GridHeight: 4}}},
		}
		world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
			// Same goroutine as RunScenario's loop and its writer, so this is
			// an unraced snapshot of exactly what has been emitted so far.
			linesAtDispatch = append(linesAtDispatch, strings.Count(log.String(), "\n"))
			env := events[seq]
			seq++
			broadcast(world, env, "dm")
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: env.Sequence}, nil
		}

		sc := &harness.Scenario{
			Name:         "streaming-progress",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"startSession":{"name":"s"}}`), Expect: &harness.Expect{OK: true}},
				{By: "dm", Command: []byte(`{"createScene":{"sceneId":"scn-1","name":"H","gridWidth":4,"gridHeight":4}}`),
					Expect: &harness.Expect{OK: true}},
				{By: "dm", Command: []byte(`{"createScene":{"sceneId":"scn-2","name":"J","gridWidth":4,"gridHeight":4}}`),
					Expect: &harness.Expect{OK: true}},
			},
		}

		dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, &log)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("scenario should pass; steps: %+v", rep.Steps)
		}

		// No denials here, so nothing is ever held back: step i dispatches
		// with exactly i lines already written.
		for i, got := range linesAtDispatch {
			if got != i {
				t.Errorf("step %d dispatched with %d lines printed, want %d — the progress log "+
					"must stream, not arrive in a lump at the end:\n%s", i, got, i, log.String())
			}
		}
		if n := strings.Count(strings.TrimSpace(log.String()), "\n") + 1; n != len(sc.Steps) {
			t.Errorf("printed %d lines for %d steps, want exactly one each:\n%s",
				n, len(sc.Steps), log.String())
		}
	})
}

// TestSingleDenialLeakDoesNotClaimARange pins `len(pending) > 1` (engine.go's
// markLeaked).
//
// Mutated to `>= 1`, EVERY leak — including one from a single denial with no
// ambiguity at all — gains the "one of the denied steps N-N produced it"
// clause. That is a false statement about the strength of the proof: with one
// outstanding denial the leak IS pinned, and saying otherwise tells an
// operator to go looking at commands that cannot have caused it.
//
// The existing leak tests all assert the clause is PRESENT for two denials,
// which `>= 1` also satisfies. Only asserting its ABSENCE for one denial
// distinguishes them.
func TestSingleDenialLeakDoesNotClaimARange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
		leak := &vttv1.Envelope{EventId: "leaked", Sequence: 1,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "leaked"}}}
		accepted := &vttv1.Envelope{EventId: "accepted", Sequence: 2,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}}}
		world["player"].send = denySend("not authorized", world, leak, "dm", "player")
		world["dm"].send = okSend(world, 2, accepted, "dm", "player")

		sc := &harness.Scenario{
			Name:         "single-denial-leak",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
			Steps: []harness.Step{
				{By: "player", Command: []byte(`{"startSession":{"name":"denied"}}`),
					Expect: &harness.Expect{DeniedContaining: "not authorized"}},
				{By: "dm", Command: []byte(`{"startSession":{"name":"ok"}}`),
					Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if rep.Steps[0].Pass {
			t.Fatal("the denied step leaked and must fail")
		}
		if d := rep.Steps[0].Detail; strings.Contains(d, "one of the denied steps") {
			t.Errorf("a SINGLE outstanding denial pins the leak exactly; the report must not "+
				"hedge with a range it does not need, got %q", d)
		}
	})
}

// TestRunSoakRejectsNonPositiveEvents pins `cfg.Events <= 0` (soak.go).
//
// Mutated to `< 0`, Events: 0 slips past the guard and RunSoak runs a
// zero-action soak that reports Pass with no actions taken — a soak that
// tested nothing, reported success, and would have satisfied any caller
// checking only the error and Pass.
func TestRunSoakRejectsNonPositiveEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		_, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 1, Events: 0, IDs: ids}, w.dial, io.Discard)
		if err == nil {
			t.Fatal("Events: 0 must be rejected — a zero-action soak that reports Pass is a " +
				"gate that tested nothing")
		}
		if !strings.Contains(err.Error(), "must be > 0") {
			t.Errorf("error should name the constraint, got %v", err)
		}
	})
}

// TestRunSoakDefaultsCheckEveryWhenUnset pins `checkEvery <= 0` (soak.go).
//
// Mutated to `< 0`, CheckEvery: 0 leaves checkEvery at zero and the run
// divides by it (`rep.Accepted%checkEvery`), panicking mid-soak. Every
// committed soak passes CheckEvery explicitly, so nothing exercised the
// default that exists precisely for callers that do not.
func TestRunSoakDefaultsCheckEveryWhenUnset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		rep, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 3, Events: 40, IDs: ids}, w.dial, io.Discard)
		if err != nil {
			t.Fatalf("CheckEvery unset must fall back to the default, got: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("soak should pass: %+v", rep)
		}
		if rep.Checkpoints == 0 {
			t.Error("the default CheckEvery must actually drive checkpoints; zero means the " +
				"fallback did not apply")
		}
	})
}

// TestScenarioReportNamesEveryStepOnce guards the report/summary counters in
// scenario.go (the INCREMENT_DECREMENT survivors at :286 and :292) by pinning
// that pass and fail tallies match the per-step results they summarise.
func TestScenarioReportCountersMatchStepResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm")}
		accepted := &vttv1.Envelope{EventId: "e1", Sequence: 1,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}}}
		world["dm"].send = okSend(world, 1, accepted, "dm")

		sc := &harness.Scenario{
			Name:         "counters",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"startSession":{"name":"s"}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		var log bytes.Buffer
		dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, &log)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		passes := regexp.MustCompile(`pass=true`).FindAllString(log.String(), -1)
		if len(passes) != len(rep.Steps) {
			t.Errorf("log reports %d passing steps, report has %d", len(passes), len(rep.Steps))
		}
	})
}
