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
// This test is written to FAIL when that lands: fix the attribution and the
// assertions below break, forcing whoever does it to update them.
func TestDeniedCommandLeakIsCaughtButMisattributed(t *testing.T) {
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

	// DETECTION — the guarantee that matters, and it holds today.
	if rep.Pass {
		t.Fatal("a denied command broadcast to every participant and the scenario PASSED — " +
			"detection is broken")
	}

	// ATTRIBUTION — today's defect, pinned so a fix cannot land silently.
	if !rep.Steps[0].Pass {
		t.Fatal("the DENIED step now fails, which means attribution has been fixed — update " +
			"this test to assert that step fails with a detail naming the unexpected " +
			"broadcast, and delete the misattribution assertions below")
	}
	if rep.Steps[1].Pass {
		t.Fatal("the accepted step passed unexpectedly; this test no longer models what it claims")
	}
	if d := rep.Steps[1].Detail; !strings.Contains(d, "not observed") {
		t.Errorf("expected the innocent step to be blamed with an 'event not observed' message, got %q", d)
	}
	t.Logf("KNOWN DEFECT (attribution): the denied step reports pass=true; blame lands on step 1 "+
		"as %q, pointing an operator at the wrong command", rep.Steps[1].Detail)
}
