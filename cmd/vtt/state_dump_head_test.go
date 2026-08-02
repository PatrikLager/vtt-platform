package main

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// drainToHead is what stops `vtt state dump` printing a silently incomplete
// state. Its predecessor stopped at the first 300ms of quiet and called that
// "caught up"; mid-replay that gap is ordinary, and the resulting snapshot was
// indistinguishable from a correct one — from the command whose output the
// golden corpus and the TypeScript fold-parity keystone are compared against.

func envAt(seq int64) *vttv1.Envelope { return &vttv1.Envelope{Sequence: seq} }

func TestDrainToHeadSurvivesAGapLongerThanTheQuietWindow(t *testing.T) {
	const window = 20 * time.Millisecond
	ch := make(chan *vttv1.Envelope, 4)
	go func() {
		ch <- envAt(1)
		time.Sleep(4 * window) // the ordinary mid-replay pause that used to end the read
		ch <- envAt(2)
		ch <- envAt(3)
	}()

	got, reached := drainToHead(ch, 0, 3, window, 5*time.Second)
	if !reached {
		t.Fatalf("want head 3 reached across the gap, got reached=false with %d envelopes", len(got))
	}
	if len(got) != 3 {
		t.Fatalf("want 3 envelopes, got %d — this is the truncation the fix exists to stop", len(got))
	}
}

func TestDrainToHeadReportsAStalledReplayRatherThanReturningShort(t *testing.T) {
	// The bool is what lets the caller REFUSE to print. Returning a short
	// slice with no signal is precisely how a truncated dump used to look
	// exactly like a complete one.
	ch := make(chan *vttv1.Envelope, 1)
	ch <- envAt(1)
	got, reached := drainToHead(ch, 0, 99, 10*time.Millisecond, 120*time.Millisecond)
	if reached {
		t.Fatal("want reached=false when the head never arrives")
	}
	if len(got) != 1 {
		t.Fatalf("want the envelopes seen so far, got %d", len(got))
	}
}

func TestDrainToHeadCollectsTheLiveTailPastTheHead(t *testing.T) {
	// Stopping the instant the head lands would drop events broadcast just
	// behind it — a truncation from the other side, and one that would make a
	// dump disagree with a fold of the same log.
	const window = 25 * time.Millisecond
	ch := make(chan *vttv1.Envelope, 8)
	go func() {
		ch <- envAt(1)
		ch <- envAt(2) // the announced head
		ch <- envAt(3) // live, already in flight
	}()

	got, reached := drainToHead(ch, 0, 2, window, 5*time.Second)
	if !reached || len(got) != 3 {
		t.Fatalf("want the tail collected too: reached=%v, %d envelopes", reached, len(got))
	}
}

func TestDrainToHeadOnAnEmptyLogReturnsImmediatelyAtQuiet(t *testing.T) {
	// Head 0 means the log was empty at subscribe time. There is nothing to
	// wait for, so the quiet window is the right signal — the one case where
	// it was never a guess.
	ch := make(chan *vttv1.Envelope)
	start := time.Now()
	got, reached := drainToHead(ch, 0, 0, 20*time.Millisecond, 5*time.Second)
	if !reached {
		t.Fatal("want head 0 treated as already reached")
	}
	if len(got) != 0 {
		t.Fatalf("want no envelopes, got %d", len(got))
	}
	if time.Since(start) > time.Second {
		t.Fatal("want an immediate return on an empty log, not a wait for the backstop")
	}
}

func TestHeadSequenceIsTheMaximum(t *testing.T) {
	if got := headSequence([]*vttv1.Envelope{envAt(3), envAt(1), envAt(2)}); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}

// TestDrainToHeadTreatsAHeadAtTheCursorAsAlreadyCaughtUp pins the general
// already-caught-up predicate, `head <= after`, rather than the special case
// `head == 0` it started as.
//
// Store.Subscribe answers with sub.lastSeq, which is the CURSOR ITSELF when
// nothing newer exists. So a client dialing after=5 against a 5-event log is
// told head=5 and will then wait forever for a Sequence >= 5 that cannot
// come: everything at or below the cursor was excluded from its backlog by
// definition. Reading only head==0 as "nothing to wait for" burns the entire
// catch-up timeout and then REFUSES TO PRINT a state that was complete the
// moment it connected — the failure mode inverted, a false alarm instead of a
// silent truncation, but a caller stuck either way.
//
// `vtt state dump` itself always dials dumpAfter (0), so this is unreachable
// from that command today. It is one flag away everywhere else: events_tail.go
// and client_run.go already take a caller-supplied after.
func TestDrainToHeadTreatsAHeadAtTheCursorAsAlreadyCaughtUp(t *testing.T) {
	ch := make(chan *vttv1.Envelope) // nothing will ever arrive, and nothing should need to
	got, reached := drainToHead(ch, 5, 5, 20*time.Millisecond, 150*time.Millisecond)
	if !reached {
		t.Fatal("want reached=true when the announced head is already at the dial cursor")
	}
	if len(got) != 0 {
		t.Fatalf("want no envelopes, got %d", len(got))
	}
}

// TestDrainToHeadStillWaitsForAHeadAheadOfTheCursor is the other side of the
// same predicate: `head <= after` must not degenerate into "always caught up".
// A cursor of 5 with an announced head of 8 has three envelopes owing, and
// reporting success without them is the original truncation bug.
func TestDrainToHeadStillWaitsForAHeadAheadOfTheCursor(t *testing.T) {
	ch := make(chan *vttv1.Envelope)
	got, reached := drainToHead(ch, 5, 8, 10*time.Millisecond, 120*time.Millisecond)
	if reached {
		t.Fatalf("want reached=false while sequences 6-8 are still owing, got %d envelopes", len(got))
	}
}
