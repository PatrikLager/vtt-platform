package harness_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// A denied command must produce NO broadcast. The scenario engine proves that
// negative by waiting denialAbsenceWindow (300ms) and checking nothing
// arrived, and that constant's own doc comment states the resulting weakness
// plainly:
//
//	"it can only ever false-PASS an implementation that eventually (but
//	 slowly) broadcasts past this window"
//
// This test makes that weakness concrete rather than leaving it as a comment.
// A server that leaks a denied command's broadcast 400ms later is a real bug —
// the event reaches every participant and lands in the log — and the harness
// reports the scenario as PASSING.
//
// It also costs: that 300ms wait runs on every denial assertion and accounts
// for 64 of the harness suite's 93 seconds (measured by dropping the window to
// 10ms: 93s -> 29s), which is in turn why internal/harness has been excluded
// from mutation testing as "hours per run".
//
// Both problems have one cause — proving absence by waiting — and one fix:
// prove it by ORDERING instead. Per-connection delivery is sequence-ordered,
// so a leaked event must arrive before the NEXT accepted step's event. Waiting
// for that event and finding nothing extra ahead of it is a proof, not a
// timeout: it is faster AND catches a leak at any delay.
//
// HOW THIS TEST IS WRITTEN, AND WHY: it asserts the CURRENT, WRONG behaviour
// — that the late leak is MISSED. That is deliberate. A test asserting the
// correct behaviour would have to be skipped to keep `task check` green, and a
// skipped test is precisely the honor system this repo's gates exist to
// remove: nothing would fail if the fix were never written.
//
// Pinned this way round, the redesign CANNOT land quietly. Implement
// absence-by-ordering and this test fails, forcing whoever does it to flip the
// assertion to the commented-out form below and rename the test. That is the
// hand-over: a RED waiting to happen, wired into the gate rather than a note
// in a ledger.
func TestDeniedCommandLeakingAfterTheWindowIsMISSED_knownGap(t *testing.T) {
	world := map[string]*fakeConn{"dm": newFakeConn("dm"), "player": newFakeConn("player")}

	leak := &vttv1.Envelope{
		EventId:  "leaked",
		Sequence: 1,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "leaked"}},
	}
	accepted := &vttv1.Envelope{
		EventId: "accepted",
		Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "ok"}},
	}

	// Step 0: denied, but the server broadcasts anyway — LATE, past the
	// 300ms absence window a timeout-based proof can see.
	world["player"].send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		go func() {
			time.Sleep(400 * time.Millisecond)
			broadcast(world, leak, "dm", "player")
		}()
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: false, Error: "not authorized"}, nil
	}
	// Step 1: accepted, and its broadcast is the ordering witness — the leak
	// above must already have arrived by the time this one does.
	world["dm"].send = okSend(world, 2, accepted, "dm", "player")

	sc := &harness.Scenario{
		Name:         "late-leak",
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

	// The gap, pinned. When absence-by-ordering lands, this assertion breaks.
	if !rep.Pass {
		t.Fatal("this scenario now FAILS, which means the late leak is being caught — the " +
			"absence-by-ordering redesign has landed. Flip this test to its correct form " +
			"(below), rename it to TestDeniedCommandLeakingAfterTheWindowIsCaught, and " +
			"delete this note.")
	}
	t.Log(strings.Join([]string{
		"KNOWN GAP: a denied command broadcast 400ms later reached every participant",
		"and the scenario passed. denialAbsenceWindow proves absence by waiting, so any",
		"leak slower than the window is invisible — its own doc comment says so.",
	}, " "))

	// The correct assertions, ready to swap in:
	//
	//	if rep.Pass {
	//		t.Fatal("scenario PASSED despite a denied command broadcasting to every participant")
	//	}
	//	if len(rep.Steps) == 0 || rep.Steps[0].Pass {
	//		t.Errorf("the DENIED step (index 0) must be the one reported failing; got %+v", rep.Steps)
	//	}
	//	if d := rep.Steps[0].Detail; !strings.Contains(d, "broadcast") {
	//		t.Errorf("failure detail should name the unexpected broadcast, got %q", d)
	//	}
}
