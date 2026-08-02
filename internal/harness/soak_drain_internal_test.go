package harness

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// drainToSequence replaced a drain that stopped at the first 300ms of silence.
//
// That silence was being used as "the replay has finished", and mid-replay on
// a loaded CI runner a 300ms gap is ordinary — so the fresh catch-up truncated
// (257 envelopes against the observer's 480) and runSoakCheckpoint reported
//
//	incremental fold (480 events) != fresh catch-up fold (257 events)
//
// which is this repo's keystone claim, rebuild == live, appearing to fail when
// it had not. These tests pin the property that fixes it: a gap longer than
// the window must NOT end the drain while the target sequence is still
// outstanding.

func env(seq int64) *vttv1.Envelope { return &vttv1.Envelope{Sequence: seq} }

func TestDrainToSequenceSurvivesAGapLongerThanTheWindow(t *testing.T) {
	const window = 20 * time.Millisecond
	ch := make(chan *vttv1.Envelope, 4)

	go func() {
		ch <- env(1)
		// Four windows of silence, exactly the shape that ended the old drain
		// early and produced a false fold divergence.
		time.Sleep(4 * window)
		ch <- env(2)
		ch <- env(3)
	}()

	got, reached := drainToSequence(ch, 3, window, 5*time.Second)
	if !reached {
		t.Fatalf("want target sequence 3 reached across a %v gap, got reached=false with %d envelopes", 4*window, len(got))
	}
	if len(got) != 3 {
		t.Fatalf("want all 3 envelopes across the gap, got %d", len(got))
	}
	if highestSequence(got) != 3 {
		t.Fatalf("want head 3, got %d", highestSequence(got))
	}
}

func TestDrainToSequenceReportsNotReachedRatherThanReturningShortSilently(t *testing.T) {
	// A replay that genuinely stalls must be DISTINGUISHABLE from a complete
	// one. The old drain returned a short slice and no signal, so the caller
	// blamed the fold; the bool is what lets runSoakCheckpoint say "never
	// reached sequence N" instead.
	ch := make(chan *vttv1.Envelope, 1)
	ch <- env(1)

	got, reached := drainToSequence(ch, 99, 10*time.Millisecond, 120*time.Millisecond)
	if reached {
		t.Fatal("want reached=false when the target never arrives")
	}
	if len(got) != 1 {
		t.Fatalf("want the envelopes seen so far, got %d", len(got))
	}
}

func TestDrainToSequenceCollectsTheTailAfterTheTarget(t *testing.T) {
	// Stopping the moment the target lands would drop envelopes broadcast
	// just behind it, which would ALSO produce a spurious fold difference —
	// the same bug from the other side.
	const window = 25 * time.Millisecond
	ch := make(chan *vttv1.Envelope, 8)
	go func() {
		ch <- env(1)
		ch <- env(2) // target
		ch <- env(3) // tail, already in flight
		ch <- env(4)
	}()

	got, reached := drainToSequence(ch, 2, window, 5*time.Second)
	if !reached {
		t.Fatal("want target reached")
	}
	if len(got) != 4 {
		t.Fatalf("want the tail past the target collected, got %d envelopes (head %d)", len(got), highestSequence(got))
	}
}

func TestDrainToSequenceWithNoTargetDegradesToAQuiescentDrain(t *testing.T) {
	// An empty observer history means there is nothing to catch up to, and the
	// silence window is then the only sensible signal.
	ch := make(chan *vttv1.Envelope, 2)
	ch <- env(1)
	got, reached := drainToSequence(ch, 0, 10*time.Millisecond, time.Second)
	if !reached {
		t.Fatal("want a head of 0 treated as already reached")
	}
	if len(got) != 1 {
		t.Fatalf("want 1 envelope, got %d", len(got))
	}
}

func TestHighestSequenceIsTheMaximumNotTheLast(t *testing.T) {
	if got := highestSequence([]*vttv1.Envelope{env(3), env(1), env(2)}); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
	if got := highestSequence(nil); got != 0 {
		t.Fatalf("want 0 for an empty history, got %d", got)
	}
}
