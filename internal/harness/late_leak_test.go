package harness_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// CORRECTION (2026-07-28). The first version of this file claimed the harness
// could MISS a denied command's broadcast entirely, and that claim was used to
// justify replacing denialAbsenceWindow with an ordering-based proof. The
// claim was wrong, and the test that "proved" it modelled an impossible
// failure.
//
// What it did: delayed the leaked broadcast until AFTER the next accepted
// step's broadcast. That is OUT-OF-ORDER delivery. The gateway broadcasts only
// from the store subscription, which delivers in sequence order per
// connection, so a leak cannot overtake a later event. And out-of-order
// delivery would defeat an ordering-based proof too — the test contradicted
// the design it was arguing for.
//
// What is actually true, pinned below: a realistic leak IS caught today.
// observeOnAll reads exactly ONE event per participant and fails unless its
// sequence matches, so a leak arriving first makes the next accepted step fail
// and the scenario fail with it. denialAbsenceWindow is not load-bearing for
// detection.
//
// The real defect is ATTRIBUTION. The denied step — the one that actually
// misbehaved — is reported PASSING, and the innocent step after it is blamed
// with a misleading "event not observed" message. An operator reading that
// report investigates the wrong command.
//
// So the case for absence-by-ordering is COST AND CLARITY, not correctness:
//   - 64s of internal/harness's 93s and 28s of cmd/vtt's 42s is this one
//     300ms window (measured by dropping it to 10ms), which is why both
//     packages are excluded from check:mutation;
//   - and a failure should name the step that caused it.
//
// FIXED (2026-07-28, E6). Absence is now proven by ORDERING: a denied step's
// verdict is deferred and settled against the next accepted step's own event.
// Anything the denied command wrongly produced carries a lower sequence and
// must already have arrived by then, so finding none is a proof rather than a
// timeout — no wait, and no dependence on how fast the server is. The
// assertions below were flipped from pinning the defect to pinning the fix,
// which is exactly what the handover comment demanded.
func TestDeniedCommandLeakFailsTheDeniedStep(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}

	leak := &vttv1.Envelope{
		EventId:  "leaked",
		Sequence: 1,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "leaked"}},
	}
	accepted := &vttv1.Envelope{
		EventId:  "accepted",
		Sequence: 2,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}},
	}

	// A denied command the server wrongly persists and broadcasts. Both
	// broadcasts are SLOW (past the 300ms absence window) but IN ORDER, which
	// is the only way a real gateway can behave.
	world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		go func() {
			time.Sleep(400 * time.Millisecond)
			broadcast(world, leak, "dm", "player")
		}()
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "not authorized"}, nil
	}
	world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		go func() {
			time.Sleep(500 * time.Millisecond)
			broadcast(world, accepted, "dm", "player")
		}()
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 2}, nil
	}

	sc := &harness.Scenario{
		Name:         "leak-attribution",
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

	// DETECTION — the guarantee that always held.
	if rep.Pass {
		t.Fatal("a denied command broadcast to every participant and the scenario PASSED — " +
			"detection is broken")
	}

	// ATTRIBUTION — what absence-by-ordering fixed. The step that misbehaved
	// is the one blamed, and its message names the leak rather than reading
	// as a mysterious missing event on an innocent command.
	if rep.Steps[0].Pass {
		t.Error("the DENIED step must be the one reported failing — it is the command that " +
			"produced the broadcast")
	}
	if d := rep.Steps[0].Detail; !strings.Contains(d, "unexpected broadcast") {
		t.Errorf("denied step's detail should name the unexpected broadcast, got %q", d)
	}
	if d := rep.Steps[0].Detail; !strings.Contains(d, "dm") || !strings.Contains(d, "player") {
		t.Errorf("detail should name every participant that received it, got %q", d)
	}
	// The witness step is collateral — it read the leaked event instead of
	// its own — and says so, so nobody investigates it first.
	if d := rep.Steps[1].Detail; !strings.Contains(d, "caused by the leaked broadcast") {
		t.Errorf("the witness step should point back at the denied step, got %q", d)
	}
}

// TestDeniedThenCleanAcceptPasses is the sibling case, and its absence let
// three mutants survive in the ordering proof itself — found by check:mutation
// within minutes of the code being written.
//
// The proof compares `env.GetSequence() < witnessSeq` (engine.go). Mutated to
// `<=`, the witness event — the accepted step's OWN event — counts as a leak,
// so every denial followed by an accepted command would fail. No test in this
// package caught that, because the only ordering test had a leak in it. The
// committed scenarios exercise this path (denials.json alone has 14 denials)
// but they run from cmd/vtt, which cannot protect internal/harness's own
// mutants.
//
// Pins the well-behaved path: a denial that leaks nothing, followed by an
// accepted command, must pass — and the accepted event must not be mistaken
// for the leak the previous step didn't produce.
func TestDeniedThenCleanAcceptPasses(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
	accepted := &vttv1.Envelope{
		EventId:  "accepted",
		Sequence: 1,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}},
	}

	// Denied, and correctly silent — no broadcast at all.
	world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "not authorized"}, nil
	}
	world["dm"].send = okSend(world, 1, accepted, "dm", "player")

	sc := &harness.Scenario{
		Name:         "denied-then-clean",
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

	if !rep.Pass {
		t.Fatalf("a silent denial followed by an accepted command must pass; steps: %+v", rep.Steps)
	}
	if !rep.Steps[0].Pass {
		t.Errorf("the denied step must pass — it broadcast nothing; detail %q", rep.Steps[0].Detail)
	}
	if !rep.Steps[1].Pass {
		t.Errorf("the accepted step must pass — its own event is the WITNESS, not a leak; "+
			"detail %q", rep.Steps[1].Detail)
	}
}

// TestTrailingDenialStillProvesAbsence covers the branch with no ordering
// witness: a denial as the LAST step falls back to the bounded wait. Without
// this the fallback is dead code as far as the suite is concerned, and a
// scenario ending in a denial would prove nothing at all.
func TestTrailingDenialStillProvesAbsence(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
	leak := &vttv1.Envelope{
		EventId:  "leaked",
		Sequence: 1,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "leaked"}},
	}
	world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		broadcast(world, leak, "dm", "player")
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "not authorized"}, nil
	}

	sc := &harness.Scenario{
		Name:         "trailing-denial",
		Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
		Steps: []harness.Step{
			{By: "player", Command: []byte(`{"startSession":{"name":"denied"}}`),
				Expect: &harness.Expect{DeniedContaining: "not authorized"}},
		},
	}

	dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
	rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if rep.Pass {
		t.Fatal("a trailing denied step that broadcast to everyone must fail — with no later " +
			"step to order against, the bounded wait is the only proof left")
	}
	if d := rep.Steps[0].Detail; !strings.Contains(d, "unexpected broadcast") {
		t.Errorf("detail should name the broadcast, got %q", d)
	}
}

// TestConsecutiveDenialsBlameTheEarliestOutstandingDenial pins the case E6 set
// out to fix and did not: two denials in a row.
//
// Absence was settled against a SINGLE outstanding denial, and a denial leaves
// firstSeq == 0, so the second denial simply overwrote the first's snapshot.
// The first denial's claim was then never settled at all, and the leak — found
// against the second's snapshot — was reported against the WRONG step. Five of
// the six committed scenarios contain consecutive denials (denials.json alone
// has fourteen), so this was the common case, not a corner.
//
// Detection always survived (nothing drains between steps, so the leaked
// envelope is still queued when the witness step reads it). This is about
// blame: an operator must not be sent to a command that behaved correctly.
//
// Note what is NOT claimed: with no accepted event between the two denials
// there is no ordering information to separate them, so the report names the
// earliest outstanding denial AND the range. Guessing one would be a lie
// dressed as precision.
func TestConsecutiveDenialsBlameTheEarliestOutstandingDenial(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
	leak := &vttv1.Envelope{
		EventId:  "leaked",
		Sequence: 1,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "leaked"}},
	}
	accepted := &vttv1.Envelope{
		EventId:  "accepted",
		Sequence: 2,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}},
	}

	denials := 0
	world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		denials++
		if denials == 1 {
			broadcast(world, leak, "dm", "player") // only the FIRST denial misbehaves
		}
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "not authorized"}, nil
	}
	world["dm"].send = okSend(world, 2, accepted, "dm", "player")

	sc := &harness.Scenario{
		Name:         "consecutive-denials",
		Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
		Steps: []harness.Step{
			{By: "player", Command: []byte(`{"startSession":{"name":"denied-a"}}`),
				Expect: &harness.Expect{DeniedContaining: "not authorized"}},
			{By: "player", Command: []byte(`{"startSession":{"name":"denied-b"}}`),
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

	if rep.Pass {
		t.Fatal("a denied command broadcast to every participant and the scenario PASSED")
	}
	if rep.Steps[0].Pass {
		t.Error("the EARLIEST outstanding denial must carry the failure; blaming the second " +
			"sends an operator to a command that behaved correctly")
	}
	d := rep.Steps[0].Detail
	if !strings.Contains(d, "unexpected broadcast") {
		t.Errorf("detail should name the unexpected broadcast, got %q", d)
	}
	if !strings.Contains(d, "steps 0-1") {
		t.Errorf("detail must disclose that the leak cannot be pinned past the range of "+
			"outstanding denials, got %q", d)
	}
}

// TestDenialFollowedByReconnectDoesNotPanic pins a crash.
//
// runReconnectStep REBUILDS history rather than appending to it, and on the
// timed-out path keeps only what actually arrived — so history can be SHORTER
// than the snapshot a pending denial took before it. leakedSince indexed
// envs[lens[name]:] with no bound, so settling that denial panicked with a
// slice-out-of-range instead of reporting a failed scenario. The soak twin
// guards exactly this (soak.go's leakedBelow); the engine copy did not.
//
// A panic here is `vtt client run` crashing on an operator's scenario file.
func TestDenialFollowedByReconnectDoesNotPanic(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}

	// A redialled connection whose channel is already closed: catch-up reads
	// nothing, so runReconnectStep takes the timedOut path and rebuilds
	// history["player"] SHORTER than the pending denial's snapshot.
	dead := newFakeConn("player-redialled")
	close(dead.events)

	// Two accepted events that FOLD cleanly in order (a second SessionStarted
	// would be rejected by the engine, failing the run for an unrelated
	// reason): open the session, then create a scene.
	sent := 0
	events := []*vttv1.Envelope{
		{EventId: "e1", Sequence: 1,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}}},
		{EventId: "e2", Sequence: 2,
			Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
				SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10}}},
	}
	world["dm"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		env := events[sent]
		sent++
		broadcast(world, env, "dm", "player")
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: env.Sequence}, nil
	}
	world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "not authorized"}, nil
	}

	sc := &harness.Scenario{
		Name:         "denial-then-reconnect",
		Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
		Steps: []harness.Step{
			{By: "dm", Command: []byte(`{"startSession":{"name":"a"}}`), Expect: &harness.Expect{OK: true}},
			{By: "player", Command: []byte(`{"startSession":{"name":"denied"}}`),
				Expect: &harness.Expect{DeniedContaining: "not authorized"}},
			{By: "player", Reconnect: &harness.ReconnectSpec{AfterSequence: 0}},
			{By: "dm", Command: []byte(`{"createScene":{"sceneId":"scn-1","name":"Hall","gridWidth":10,"gridHeight":10}}`),
				Expect: &harness.Expect{OK: true}},
		},
	}

	playerDials := 0
	dial := func(name string, _ int64) (harness.Conn, error) {
		if name == "player" {
			playerDials++
			if playerDials > 1 {
				return dead, nil
			}
		}
		return world[name], nil
	}

	// The assertion IS that this returns rather than panicking.
	rep, err := harness.RunScenario(context.Background(), sc, dial, nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if rep.Pass {
		t.Error("the reconnect replayed nothing and must fail the scenario")
	}
}

// TestDefaultLogNamesTheDeniedStep pins E6's actual goal in the output an
// operator actually sees.
//
// A denied step's line was printed at the end of its own iteration, but its
// verdict is not settled until the NEXT accepted step — so the flip to
// pass=false was written into the Report struct and never re-printed. Under
// `vtt client run` without --json the human log is the ONLY output, so it
// showed `[step 0] ... pass=true` for the command that leaked and blamed the
// innocent step after it: the exact defect E6 was built to remove, still
// present everywhere except --json.
//
// The two tests above this one pass io.Discard, which is why the suite could
// not see it. This one asserts on the bytes.
func TestDefaultLogNamesTheDeniedStep(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}
	leak := &vttv1.Envelope{
		EventId:  "leaked",
		Sequence: 1,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "leaked"}},
	}
	accepted := &vttv1.Envelope{
		EventId:  "accepted",
		Sequence: 2,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}},
	}
	world["player"].send = denySend("not authorized", world, leak, "dm", "player")
	world["dm"].send = okSend(world, 2, accepted, "dm", "player")

	sc := &harness.Scenario{
		Name:         "log-attribution",
		Participants: []harness.Participant{{Name: "dm", Role: "dm"}, {Name: "player", Role: "player"}},
		Steps: []harness.Step{
			{By: "player", Command: []byte(`{"startSession":{"name":"denied"}}`),
				Expect: &harness.Expect{DeniedContaining: "not authorized"}},
			{By: "dm", Command: []byte(`{"startSession":{"name":"ok"}}`),
				Expect: &harness.Expect{OK: true}},
		},
	}

	var log bytes.Buffer
	dial := func(name string, _ int64) (harness.Conn, error) { return world[name], nil }
	if _, err := harness.RunScenario(context.Background(), sc, dial, nil, &log); err != nil {
		t.Fatalf("RunScenario: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("want a line per step, got %q", log.String())
	}
	if !strings.Contains(lines[0], "pass=false") {
		t.Errorf("step 0 leaked a broadcast; its printed line must say pass=false, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "unexpected broadcast") {
		t.Errorf("step 0's printed line must name the leak, got %q", lines[0])
	}
	if !strings.Contains(log.String(), "caused by the leaked broadcast") {
		t.Errorf("the witness step's line must point back at the denied step, got %q", log.String())
	}
}
