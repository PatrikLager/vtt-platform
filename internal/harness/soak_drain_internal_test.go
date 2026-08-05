package harness

import (
	"context"
	"errors"
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

// TestFreshCatchUpResumesAfterAnOverflowDisconnect pins the recovery path
// eventBuffer's doc comment prescribes and the soak never implemented.
//
// The client's Events() buffer is 256 and deliverEvent TEARS THE CONNECTION
// DOWN rather than blocking when it fills — deliberately, because this side
// cannot signal the server to slow down. The documented recovery is the
// caller's: "Dial again with a fresh `after` cursor, the same recovery story
// the gateway's own overflow gives every other consumer."
//
// The soak's fresh catch-up dialled ONCE. On CI 2026-08-04 it was disconnected
// at 257 envelopes of 480 (the buffer plus the one in flight) and reported the
// resulting short history as a keystone failure — "incremental fold (480) !=
// fresh catch-up fold (257)" — i.e. rebuild != live, the one claim this
// repo's event-sourcing design rests on, appearing to break when it had not.
//
// This scripts exactly that: the first connection delivers part of the replay
// and dies with ErrEventsOverflow, the second resumes from the cursor. The
// assertion is that the catch-up completes, and that the SECOND dial asked for
// the right cursor — resuming from 0 would double-count and resuming past the
// gap would silently lose envelopes.
func TestFreshCatchUpResumesAfterAnOverflowDisconnect(t *testing.T) {
	const target = 10
	var dialedAfter []int64

	dial := func(name string, after int64) (Conn, error) {
		dialedAfter = append(dialedAfter, after)
		c := newStubConn()
		go func() {
			defer c.finish()
			if after == 0 {
				// First attempt: deliver 1..4, then die the way a real
				// overflow does.
				for i := int64(1); i <= 4; i++ {
					c.send(env(i))
				}
				c.err = ErrEventsOverflow
				return
			}
			for i := after + 1; i <= target; i++ {
				c.send(env(i))
			}
		}()
		return c, nil
	}

	got, reached := drainFreshCatchUp(dial, target, denialAbsenceWindow, freshCatchUpTimeout)
	if !reached {
		t.Fatalf("catch-up did not reach %d across an overflow disconnect: %d envelopes", target, len(got))
	}
	if len(got) != target {
		t.Fatalf("want %d envelopes across the resume, got %d", target, len(got))
	}
	for i, e := range got {
		if e.GetSequence() != int64(i+1) {
			t.Fatalf("envelope %d has sequence %d — the resume cursor dropped or duplicated", i, e.GetSequence())
		}
	}
	if len(dialedAfter) != 2 {
		t.Fatalf("want exactly 2 dials (initial + one resume), got %d: %v", len(dialedAfter), dialedAfter)
	}
	if dialedAfter[0] != 0 {
		t.Errorf("first dial after=%d, want 0", dialedAfter[0])
	}
	if dialedAfter[1] != 4 {
		t.Errorf("resume dial after=%d, want 4 — the last sequence actually received", dialedAfter[1])
	}
}

// The three tests below pin guards that the resume test alone leaves open. A
// review's mutants proved it: `overflowed || true`, deleting the
// no-forward-progress check, and raising freshCatchUpAttempts to a million all
// SURVIVED against the resume test, because it only ever exercises one
// overflow followed by success.

func TestFreshCatchUpDoesNotResumeAfterANonOverflowEnd(t *testing.T) {
	// Resuming from an ordinary read error would mask it behind up to eight
	// dials and a generic timeout message. Only an overflow has a documented
	// recovery; everything else must surface as itself.
	dials := 0
	dial := func(name string, after int64) (Conn, error) {
		dials++
		c := newStubConn()
		go func() {
			defer c.finish()
			c.send(env(1))
			c.err = errors.New("connection reset by peer")
		}()
		return c, nil
	}
	got, reached := drainFreshCatchUp(dial, 10, 20*time.Millisecond, 2*time.Second)
	if reached {
		t.Fatal("want reached=false: a non-overflow end has no recovery path")
	}
	if dials != 1 {
		t.Errorf("dialed %d times, want exactly 1 — only an overflow may be resumed from", dials)
	}
	if len(got) != 1 {
		t.Errorf("want the 1 envelope that did arrive, got %d", len(got))
	}
}

func TestFreshCatchUpStopsWhenAResumeMakesNoProgress(t *testing.T) {
	// A connection that overflows before delivering anything would otherwise
	// be re-dialled at the same cursor until the attempt cap, turning an
	// immediate failure into a slow one. The guard fires on the FIRST attempt:
	// no envelope arrived, so the cursor cannot advance, so another dial is
	// futile by construction. (This expectation was wrong when first written —
	// it said 2 — and the test corrected it.)
	dials := 0
	dial := func(name string, after int64) (Conn, error) {
		dials++
		c := newStubConn()
		go func() {
			defer c.finish()
			c.err = ErrEventsOverflow // overflowed having delivered nothing
		}()
		return c, nil
	}
	_, reached := drainFreshCatchUp(dial, 10, 20*time.Millisecond, 2*time.Second)
	if reached {
		t.Fatal("want reached=false")
	}
	if dials != 1 {
		t.Errorf("dialed %d times, want exactly 1 — the FIRST attempt already made no progress, "+
			"and re-dialling the same cursor cannot change that", dials)
	}
}

func TestFreshCatchUpIsBoundedByTheAttemptCap(t *testing.T) {
	// Forward progress every time, but never enough: without the cap this
	// loops until the deadline instead of failing as a real failure.
	dials := 0
	dial := func(name string, after int64) (Conn, error) {
		dials++
		c := newStubConn()
		go func() {
			defer c.finish()
			c.send(env(after + 1)) // one envelope, then die: progress, but slow
			c.err = ErrEventsOverflow
		}()
		return c, nil
	}
	_, reached := drainFreshCatchUp(dial, 1000, 20*time.Millisecond, 30*time.Second)
	if reached {
		t.Fatal("want reached=false: the target is unreachable one envelope at a time")
	}
	if dials != freshCatchUpAttempts {
		t.Errorf("dialed %d times, want the cap of %d", dials, freshCatchUpAttempts)
	}
}

// stubConn is the smallest Conn that can END FOR A REASON, which is the whole
// point of the test above: a closed Events() channel alone cannot distinguish
// an overflow disconnect from a finished replay.
type stubConn struct {
	events chan *vttv1.Envelope
	err    error
}

func newStubConn() *stubConn {
	return &stubConn{events: make(chan *vttv1.Envelope, 64)}
}

func (s *stubConn) send(e *vttv1.Envelope)         { s.events <- e }
func (s *stubConn) finish()                        { close(s.events) }
func (s *stubConn) Events() <-chan *vttv1.Envelope { return s.events }
func (s *stubConn) CloseErr() error                { return s.err }
func (s *stubConn) Close() error                   { return nil }
func (s *stubConn) SendCommand(context.Context, *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	return nil, nil
}
