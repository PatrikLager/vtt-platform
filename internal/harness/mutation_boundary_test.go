package harness_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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

// TestReconnectCatchUpExcludesTheCursorEvent pins `env.Sequence > after`
// (engine.go's runReconnectStep).
//
// `after` is a CURSOR, not a lower bound: the participant has already seen
// that event, so catch-up must replay only what comes strictly after it.
// Mutated to `>=`, the harness expects the cursor event to be replayed too —
// so a correctly-behaving server, replaying only newer events, gets reported
// as delivering a short catch-up. The harness would fail real servers for
// being right.
//
// No existing reconnect test used an AfterSequence that names an event that
// actually exists, so nothing distinguished the two.
func TestReconnectCatchUpExcludesTheCursorEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
		sent := 0
		events := []*vttv1.Envelope{
			{EventId: "e1", Sequence: 1, Payload: &vttv1.Envelope_SessionStarted{
				SessionStarted: &vttv1.SessionStarted{Name: "s"}}},
			{EventId: "e2", Sequence: 2, Payload: &vttv1.Envelope_SceneCreated{
				SceneCreated: &vttv1.SceneCreated{SceneId: "scn-1", Name: "H", GridWidth: 4, GridHeight: 4}}},
		}
		world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
			env := events[sent]
			sent++
			broadcast(world, env, "dm", "player")
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: env.Sequence}, nil
		}

		sc := &harness.Scenario{
			Name:         "reconnect-cursor",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"startSession":{"name":"s"}}`), Expect: &harness.Expect{OK: true}},
				{By: "dm", Command: []byte(`{"createScene":{"sceneId":"scn-1","name":"H","gridWidth":4,"gridHeight":4}}`),
					Expect: &harness.Expect{OK: true}},
				// after=1 names e1, which player already saw: only e2 may replay.
				{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: 1}},
			},
		}

		// A correct server: replays strictly what follows the cursor.
		dial := func(name string, after int64) (harness.Conn, error) {
			if name == "player" && after > 0 {
				fresh := newFakeConn("player-redialled")
				for _, env := range events[:sent] {
					if env.Sequence > after {
						fresh.trySend(env)
					}
				}
				world["player"] = fresh
				return fresh, nil
			}
			return world[name], nil
		}

		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if !rep.Steps[2].Pass {
			t.Fatalf("a server replaying exactly the events after the cursor must PASS; "+
				"detail %q", rep.Steps[2].Detail)
		}
	})
}

// TestUnresolvedPlaceholderIsNamedExactly pins the arithmetic in
// `raw[idx+len(participantIDPlaceholderPrefix):]` (engine.go).
//
// The error exists to tell an author which participant id is missing. Mutated
// to `idx - len(...)`, the slice starts before the placeholder and the message
// names something like `"a":"{{id:alice` — still an error, still mentioning
// the step, so a test asserting only "an error occurred" passes while the one
// piece of information the message exists to carry is corrupted.
func TestUnresolvedPlaceholderIsNamedExactly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sc := &harness.Scenario{
			Name:         "unresolved",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"grantActorControl":{"actorId":"a","participantId":"{{id:alice}}","kind":"ACTOR_KIND_PARTY_MEMBER"}}`),
					Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(string, int64) (harness.Conn, error) { return newFakeConn("dm"), nil }
		_, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err == nil {
			t.Fatal("an unresolved {{id:...}} must be a hard error, not dispatched literally")
		}
		if !strings.Contains(err.Error(), `participant "alice"`) {
			t.Errorf("the error must name the participant exactly — that is the only thing it "+
				"exists to tell the author — got %v", err)
		}
	})
}

// TestPlaceholderAtOffsetZeroIsStillDetected pins `idx >= 0` (engine.go).
//
// Mutated to `idx > 0`, a placeholder at the very start of the command slips
// through and the literal bytes `{{id:...}}` are dispatched as if they were
// real command data. Every realistic command is JSON and starts with `{`, so
// the placeholder is never at offset 0 in practice — which is exactly why
// nothing covered it, and exactly why the boundary needs pinning rather than
// assuming.
func TestPlaceholderAtOffsetZeroIsStillDetected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sc := &harness.Scenario{
			Name:         "offset-zero",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{{id:ghost}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(string, int64) (harness.Conn, error) { return newFakeConn("dm"), nil }
		_, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err == nil {
			t.Fatal("a placeholder at offset 0 must still be caught; resolution is a byte scan " +
				"that runs before any JSON parsing, so offset 0 is reachable")
		}
		if !strings.Contains(err.Error(), `participant "ghost"`) {
			t.Errorf("error should name the participant, got %v", err)
		}
	})
}

// TestEmptyPlaceholderNameIsReportedAsEmpty pins `end >= 0` (engine.go).
//
// `{{id:}}` puts the closing delimiter immediately after the prefix, so the
// suffix is found at index 0. Mutated to `end > 0`, that lookup is treated as
// "no closing delimiter" and the reported name becomes the whole remainder of
// the command — turning a precise "you left the name blank" into a message
// quoting a slab of JSON.
func TestEmptyPlaceholderNameIsReportedAsEmpty(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sc := &harness.Scenario{
			Name:         "empty-name",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"grantActorControl":{"actorId":"a","participantId":"{{id:}}","kind":"ACTOR_KIND_PARTY_MEMBER"}}`),
					Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(string, int64) (harness.Conn, error) { return newFakeConn("dm"), nil }
		_, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err == nil {
			t.Fatal("{{id:}} is unresolvable and must error")
		}
		if !strings.Contains(err.Error(), `participant ""`) {
			t.Errorf("an empty name must be reported AS empty, not as the rest of the "+
				"command, got %v", err)
		}
	})
}

// TestSoakLoneDenialLeakDoesNotClaimARange pins `len(pending) > 1` in soak.go
// — the twin of the engine-side mutant killed alongside it.
//
// Mutated to `>= 1`, a leak from a SINGLE outstanding denial is reported as
// "one of the denied actions N-N produced it", hedging about an attribution
// that is in fact exact. The soak's existing leak test has TWO denials
// outstanding, where the clause is correct and the mutant is indistinguishable
// from the original; only a lone denial separates them.
//
// Seed 2 puts a lone denial at action 50 with an accepted action next; the
// fake leaks on that denial only (ordinal 0).
func TestSoakLoneDenialLeakDoesNotClaimARange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			seed          = 2
			events        = 60
			leakOnOrdinal = 0 // the lone denial at action 50
		)
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		w.leakOnDenial = func(ordinal int) bool { return ordinal == leakOnOrdinal }

		var log bytes.Buffer
		rep, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: seed, Events: events, CheckEvery: events + 1, IDs: ids}, w.dial, &log)
		if err != nil {
			t.Fatalf("RunSoak: %v", err)
		}
		if w.leaked != 1 {
			t.Fatalf("fixture broke: leaked %d times, want exactly 1 — reseed via the probe", w.leaked)
		}
		if rep.Pass {
			t.Fatal("a denied action broadcast to every participant; the soak must fail")
		}
		if strings.Contains(log.String(), "one of the denied actions") {
			t.Errorf("one outstanding denial pins the leak exactly; the report must not hedge "+
				"with a range. Log:\n%s", log.String())
		}
	})
}

// TestEmptyParticipantListDoesNotPanic pins both `len(sc.Participants) > 0`
// guards (engine.go, the freshness drain and the probe fold).
//
// Nothing in scenario.go's validation requires a participant, so a scenario
// declaring none is constructible and an author will eventually write one.
// Mutated to `>= 0` the guards are always true and both bodies index
// `sc.Participants[0]`, panicking on an empty slice — `vtt client run`
// crashing on a valid-if-pointless input file instead of reporting on it.
func TestEmptyParticipantListDoesNotPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sc := &harness.Scenario{Name: "no-participants"}
		dial := func(string, int64) (harness.Conn, error) {
			t.Error("dial must not be called when there are no participants")
			return nil, nil
		}
		rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
		if err != nil {
			t.Fatalf("an empty scenario should report, not error: %v", err)
		}
		if !rep.Pass {
			t.Errorf("a scenario with nothing to fail should pass, got %+v", rep)
		}
	})
}

// TestEveryDialledConnectionIsClosed pins `if c != nil` (engine.go's cleanup
// defer).
//
// Mutated to `c == nil`, the loop closes only nil connections — that is, none
// — and every real one leaks. Under `go test` that is invisible: nothing
// fails, nothing warns, and the process exits. The property is worth asserting
// on its own terms anyway, since `vtt client run` executes scenarios in a
// long-lived process where leaked sockets accumulate.
func TestEveryDialledConnectionIsClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
		accepted := &vttv1.Envelope{EventId: "e1", Sequence: 1,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}}}
		world["dm"].send = okSend(world, 1, accepted, "dm", "player")

		sc := &harness.Scenario{
			Name:         "cleanup",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"startSession":{"name":"s"}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
		if _, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard); err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		for name, c := range world {
			if !c.isClosed() {
				t.Errorf("connection %q was left open; RunScenario must close every connection "+
					"it dialled", name)
			}
		}
	})
}

// TestAuthoredRequestIdIsPreserved pins `cmd.RequestId == ""` (engine.go).
//
// The default exists for commands that do not set one. Mutated to `!=`, the
// logic inverts: an AUTHORED request id is overwritten with
// `scenario-step-N`, and a blank one is left blank. Overwriting is the
// damaging half — request_id is how a client correlates a result to the
// command that caused it (see ServerFrame's ordering contract), so silently
// rewriting it breaks correlation for any scenario that sets one deliberately.
func TestAuthoredRequestIdIsPreserved(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm")}
		var seenIDs []string
		accepted := &vttv1.Envelope{EventId: "e1", Sequence: 1,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}}}
		world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
			seenIDs = append(seenIDs, cmd.GetRequestId())
			broadcast(world, accepted, "dm")
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
		}

		sc := &harness.Scenario{
			Name:         "request-id",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"requestId":"authored-id","startSession":{"name":"s"}}`),
					Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
		if _, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard); err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if len(seenIDs) != 1 || seenIDs[0] != "authored-id" {
			t.Errorf("the scenario authored request_id %q; the harness must send it unchanged, "+
				"got %v — request_id is how a result is correlated to its command",
				"authored-id", seenIDs)
		}
	})
}

// TestUnsetRequestIdGetsTheStepDefault is the other half: a command with no
// request id must receive the generated one, so results remain correlatable.
func TestUnsetRequestIdGetsTheStepDefault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		world := map[string]*fakeConn{"dm": newFakeConn("dm")}
		var seen string
		accepted := &vttv1.Envelope{EventId: "e1", Sequence: 1,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}}}
		world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
			seen = cmd.GetRequestId()
			broadcast(world, accepted, "dm")
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
		}
		sc := &harness.Scenario{
			Name:         "request-id-default",
			Participants: []harness.Participant{{Name: "dm", Role: "dm"}},
			Steps: []harness.Step{
				{By: "dm", Command: []byte(`{"startSession":{"name":"s"}}`), Expect: &harness.Expect{OK: true}},
			},
		}
		dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
		if _, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard); err != nil {
			t.Fatalf("RunScenario: %v", err)
		}
		if seen == "" {
			t.Error("a command with no request_id must be given one; an empty id makes the " +
				"result uncorrelatable")
		}
	})
}

// TestSoakGeneratedIdsStartAtOneAndAscend pins the `m.sceneN++` /
// `m.actorN++` / `m.tokenN++` counters (soak.go).
//
// Mutated to `--`, the generator still produces UNIQUE ids — `soak-scn--1`,
// `soak-scn--2` — so every determinism, ratio and bookkeeping assertion in
// the soak suite still passes: both runs of a same-seed comparison are
// mutated identically, and nothing else reads the ids' shape. The only
// observable is the id text itself, which is exactly what a scenario author
// reading a soak log or reproducing a failure by hand relies on.
func TestSoakGeneratedIdsStartAtOneAndAscend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		rep, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 5, Events: 120, CheckEvery: 1000, IDs: ids}, w.dial, io.Discard)
		if err != nil {
			t.Fatalf("RunSoak: %v", err)
		}
		if !rep.Pass {
			t.Fatalf("soak should pass: %+v", rep)
		}

		// The first id minted for each kind must be 1, and they must ascend.
		for _, kind := range []struct{ prefix, field string }{
			{"soak-scn-", "sceneId"},
			{"soak-actor-", "actorId"},
			{"soak-tok-", "tokenId"},
		} {
			re := regexp.MustCompile(regexp.QuoteMeta(kind.prefix) + `(-?\d+)`)
			var seen []int
			for _, cmd := range w.dispatchLog {
				for _, m := range re.FindAllStringSubmatch(cmd, -1) {
					var n int
					if _, err := fmt.Sscanf(m[1], "%d", &n); err == nil {
						seen = append(seen, n)
					}
				}
			}
			if len(seen) == 0 {
				t.Fatalf("no %s ids in the dispatch log; the fixture no longer exercises this kind",
					kind.prefix)
			}
			if seen[0] != 1 {
				t.Errorf("first %s id is %d, want 1 — generated ids must start at 1 so a soak "+
					"log can be read and reproduced by hand", kind.prefix, seen[0])
			}
			high := 0
			for _, n := range seen {
				if n > high {
					high = n
				}
				if n < 1 {
					t.Errorf("%s id %d is below 1; ids must ascend, not descend", kind.prefix, n)
					break
				}
			}
			if high < 1 {
				t.Errorf("%s ids never exceeded %d", kind.prefix, high)
			}
		}
	})
}

// TestSoakSurvivesShortRunsAcrossSeeds pins `canPlaceToken`'s
// `len(m.scenes) > 0 && len(m.actors) > 0` (soak.go).
//
// Mutated to `>= 0`, either half is always true and planPlaceToken runs with
// an empty pool: `m.scenes[rng.Intn(len(m.scenes))]` calls rng.Intn(0), which
// PANICS. The committed soak tests all run long enough that scenes and actors
// exist by the time placeToken is drawn, so none of them ever evaluates the
// guard in the state it exists to protect.
//
// Short runs across many seeds is what reaches it: with only a handful of
// actions the generator is repeatedly asked to act before the model has
// anything to act on.
func TestSoakSurvivesShortRunsAcrossSeeds(t *testing.T) {
	// 321 is not decoration. Both halves of the guard need a state where the
	// OTHER half is already satisfied, and those states are rare: reaching
	// "a scene exists but no actor does" needs a createScene draw (p=0.05)
	// followed by a placeToken draw (p=0.15) with no intervening draw, because
	// planStep's catch-all fallback is planAddActor and almost every
	// unproductive draw therefore creates an actor. A 9000-run sweep with the
	// actors-side mutation applied found seed 321 at Events=2 as the first
	// hit; seeds 1-24 alone leave that half unpinned. Do not trim this list
	// without re-running that sweep.
	seeds := []int64{321}
	for s := int64(1); s <= 24; s++ {
		seeds = append(seeds, s)
	}
	for _, seed := range seeds {
		synctest.Test(t, func(t *testing.T) {
			ids := soakTestIDs()
			w := newSoakWorld(ids)
			events := 4
			if seed == 321 {
				events = 2 // the length at which 321 reaches the actors-side guard
			}
			rep, err := harness.RunSoak(context.Background(),
				harness.SoakConfig{Seed: seed, Events: events, CheckEvery: 1000, IDs: ids}, w.dial, io.Discard)
			if err != nil {
				t.Fatalf("seed %d: RunSoak: %v", seed, err)
			}
			if !rep.Pass {
				t.Fatalf("seed %d: a short soak must still pass: %+v", seed, rep)
			}
		})
	}
}

// TestSoakIssuerChoiceIsPinnedForASeed pins `rng.Intn(2) == 0` in
// pickDMOrAgent (soak.go).
//
// Mutated to `!=`, dm and agent simply swap. Every existing assertion
// survives that: the same-seed determinism tests compare two runs that are
// BOTH mutated, the action-mix ratios are unchanged, and both issuers remain
// authorized for the commands in question. The only observable is which
// participant issued which action for a given seed — and that IS a contract
// here, because the soak's value depends on a seed reproducing a run exactly
// (the pinned seed-1 counts in the docs rest on it).
//
// So this is a golden: the issuer sequence for the first lifecycle commands
// of a fixed seed. It is deliberately narrow — the first four — so a genuine
// generator change produces a small, readable diff rather than a wall.
//
// RE-DERIVED 2026-08-24, from "dm,agent,dm,dm,dm,agent", by the actor-kind
// branch: add_actor stopped accepting a controller (visibility spec §5.1), so
// assigning an actor to a player is now TWO commands, and planPendingGrant
// issues the second one with the SAME issuer as the first rather than drawing
// a fresh one. Hence the doubled entries — each pair is one add-then-grant,
// not a changed coin flip. Measured over three runs before the golden was
// moved; pickDMOrAgent itself is untouched, and the mutant this pins still
// swaps every dm for an agent.
//
// RE-DERIVED AGAIN 2026-08-31, from "dm,dm,agent,agent,dm,dm", by
// 2026-08-31-retraction-leaves task-4-brief.md: removing the retraction
// bucket and giving its freed 10% to move-own (pickBucket's own doc comment
// carries the ruling) shifts which bucket every rng.Float64() draw in
// [0.80, 0.95) lands in — draws at or above 0.95 still land in
// deniedAttempt, unchanged — which cascades into how many further rng draws
// each action consumes — so the ENTIRE subsequent draw sequence for seed 7
// differs from here on, pickDMOrAgent itself untouched. Confirmed stable
// across three runs.
func TestSoakIssuerChoiceIsPinnedForASeed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		if _, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 7, Events: 40, CheckEvery: 1000, IDs: ids}, w.dial, io.Discard); err != nil {
			t.Fatalf("RunSoak: %v", err)
		}

		if len(w.dispatchIssuers) < 6 {
			t.Fatalf("seed 7 dispatched only %d commands; the golden needs at least 6",
				len(w.dispatchIssuers))
		}
		issuers := w.dispatchIssuers
		got := strings.Join(issuers[:6], ",")
		const want = "dm,agent,agent,dm,dm,dm"
		if got != want {
			t.Errorf("issuer sequence for seed 7 = %q, want %q — a seed must reproduce a run "+
				"exactly, issuers included", got, want)
		}
	})
}

// TestSoakSlowBroadcastIsNotMistakenForALeak pins the
// `if lastAcceptedSeq > 0 { waitAllCaughtUp(...) }` guard RunSoak runs
// before a denied step's "before" snapshot (soak.go).
//
// The guard's job, per its own comment, is to make sure every participant has
// caught up to the last ACCEPTED sequence before the "before" snapshot is
// taken. Without it the snapshot can be taken while a legitimate broadcast is
// still in flight; that envelope then lands after the snapshot with a
// sequence BELOW the next accepted event's, which is precisely leakedBelow's
// definition of a leak. The soak would report a broadcast the server never
// wrongly made — a false accusation, on a passing server.
//
// Nothing caught the guard's removal because with an inline fan-out the drain
// almost always wins the race. Delaying the accepted broadcast makes the
// ordering deterministic: a correct run waits for it, a guardless one does
// not and cries leak.
func TestSoakSlowBroadcastIsNotMistakenForALeak(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		// Long enough to land well after SendCommand returns; free under the
		// fake clock, which advances only once everything is durably blocked.
		w.slowBroadcast = 50 * time.Millisecond

		var log bytes.Buffer
		rep, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 22, Events: 60, CheckEvery: 1000, IDs: ids}, w.dial, &log)
		if err != nil {
			t.Fatalf("RunSoak: %v", err)
		}
		w.bcWG.Wait()

		if !rep.Pass {
			t.Fatalf("no denied command broadcast anything here; a merely SLOW fan-out must "+
				"not be reported as a leak. Log:\n%s", log.String())
		}
		if strings.Contains(log.String(), "unexpected broadcast") {
			t.Errorf("a legitimate late broadcast was reported as a leak:\n%s", log.String())
		}
	})
}

// TestSoakSlowBroadcastWithALeakDoesNotDeadlock is the permanent regression
// pin for fix round 1, I3: deliverInOrder's reorder buffer (added this task
// to fix TestSoakSlowBroadcastIsNotMistakenForALeak's own sibling defect) used
// to key its "next expected delivery" on env.Sequence, assuming sequences
// reaching it were dense from 1. That assumption is false whenever
// w.leakOnDenial is ALSO set: maybeLeak bumps w.seq and used to broadcast
// inline, bypassing the buffer entirely, so the sequence it consumed was a
// hole the buffer would wait on forever — the same hole toEnvelope's own
// w.seq++ leaves behind whenever the subsequent engine.Apply then rejects
// the envelope inside handle, which returns before broadcasting anything.
// Combining w.slowBroadcast with w.leakOnDenial reproduced it every time:
// `bcWG.Wait()` deadlocked the whole synctest bubble. The fix keys the
// buffer by a private ordinal assigned only when something is actually
// enqueued for delayed delivery (soakWorld's pendingBroadcast field
// comment), which cannot have holes by construction, and routes maybeLeak's
// own delivery through the identical gate (deliverEnvelope) rather than
// broadcasting inline, so a leak can no longer jump the queue ahead of
// whatever the buffer still has waiting either.
//
// Same seed/events/leak-ordinal shape as TestSoakSlowBroadcastIsNotMistaken
// ALeak and TestSoakLoneDenialLeakDoesNotClaimARange, combined: this is
// deliberately the intersection those two only ever covered separately.
func TestSoakSlowBroadcastWithALeakDoesNotDeadlock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		w.slowBroadcast = 50 * time.Millisecond
		w.leakOnDenial = func(ordinal int) bool { return ordinal == 0 }

		var log bytes.Buffer
		rep, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 22, Events: 60, CheckEvery: 1000, IDs: ids}, w.dial, &log)
		if err != nil {
			t.Fatalf("RunSoak: %v", err)
		}
		// The assertion IS reaching this line: pre-fix, bcWG.Wait() below
		// never returns and synctest panics the whole bubble as deadlocked
		// before any t.Fatal here would even run.
		w.bcWG.Wait()

		if w.leaked != 1 {
			t.Fatalf("fixture broke: the fake leaked %d times, want exactly 1 — reseed via the "+
				"probe rather than deleting this assertion", w.leaked)
		}
		if rep.Pass {
			t.Fatalf("a denied action broadcast an envelope to every participant; the soak must "+
				"fail even with slowBroadcast delaying delivery. Log:\n%s", log.String())
		}
	})
}

// TestSoakWithNoAcceptedActionsDoesNotWaitForSequenceZero pins
// `waitForSeq > 0` in runSoakCheckpoint (soak.go).
//
// I nearly adjudicated this one equivalent, on the argument that a soak
// always accepts its first action so waitForSeq is never 0. That argument is
// wrong: it describes the GENERATOR, not the server. A server that rejects
// everything leaves rep.Accepted at 0, so the final checkpoint — which runs
// unconditionally — sees waitForSeq == 0.
//
// At `>= 0` the checkpoint then calls waitFor(observer, 0, observeTimeout),
// and waitFor does NOT short-circuit on 0: it scans for an envelope with
// Sequence == 0, sequences start at 1, so it spins to the deadline and
// reports "never caught up to sequence 0" — a fabricated second failure
// stacked on an already-failing run, pointing at the wrong thing.
func TestSoakWithNoAcceptedActionsDoesNotWaitForSequenceZero(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ids := soakTestIDs()
		w := newSoakWorld(ids)
		w.denyEverything = true

		var log bytes.Buffer
		rep, err := harness.RunSoak(context.Background(),
			harness.SoakConfig{Seed: 4, Events: 3, CheckEvery: 1000, IDs: ids}, w.dial, &log)
		if err != nil {
			t.Fatalf("RunSoak: %v", err)
		}
		if rep.Pass {
			t.Fatal("every command was denied; the soak must fail")
		}
		if rep.Accepted != 0 {
			t.Fatalf("fixture broke: Accepted = %d, want 0", rep.Accepted)
		}
		// Also pins `rep.Denied++` on the unexpected-denial path: every one of
		// the three actions was rejected, so all three must be counted. That
		// counter is the only record of how badly a run went, and nothing
		// asserted it before this fixture existed to drive the path at all.
		if rep.Denied != 3 {
			t.Errorf("Denied = %d, want 3 — every rejected action must be counted", rep.Denied)
		}
		if strings.Contains(log.String(), "caught up to sequence 0") {
			t.Errorf("with nothing accepted there is no sequence to catch up to; the checkpoint "+
				"must skip the wait rather than invent a failure:\n%s", log.String())
		}
	})
}
