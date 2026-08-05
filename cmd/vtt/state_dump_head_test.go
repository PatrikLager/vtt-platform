package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
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

	got, reached, _ := drainToHead(ch, nil, 0, 3, window, 5*time.Second)
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
	got, reached, _ := drainToHead(ch, nil, 0, 99, 10*time.Millisecond, 120*time.Millisecond)
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

	got, reached, _ := drainToHead(ch, nil, 0, 2, window, 5*time.Second)
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
	got, reached, _ := drainToHead(ch, nil, 0, 0, 20*time.Millisecond, 5*time.Second)
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
	got, reached, _ := drainToHead(ch, nil, 5, 5, 20*time.Millisecond, 150*time.Millisecond)
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
	got, reached, _ := drainToHead(ch, nil, 5, 8, 10*time.Millisecond, 120*time.Millisecond)
	if reached {
		t.Fatalf("want reached=false while sequences 6-8 are still owing, got %d envelopes", len(got))
	}
}

// A short read has two causes that call for OPPOSITE responses, and until
// 2026-08-06 this function reported them identically — it returned the same
// (events, false) whether the deadline expired or the channel closed, and the
// caller unconditionally blamed dumpCatchUpTimeout.
//
// So a dump truncated because THIS PROCESS read too slowly (harness.Client
// tears itself down with ErrEventsOverflow at 256 buffered envelopes) printed
// a 30s timeout it had never come close to hitting, sending the next reader
// after gateway latency when the fix was on the reading side. That exact
// misdiagnosis already cost one CI investigation in the soak keystone
// (internal/harness/soak.go's drainFreshCatchUp comment); this is the same bug
// in the shipped CLI.
//
// The channel alone cannot answer it: a closed channel looks identical either
// way. Only the connection's CloseErr can, which is why drainToHead now takes
// one.
func TestDrainToHeadNamesAnOverflowRatherThanBlamingTheClock(t *testing.T) {
	ch := make(chan *vttv1.Envelope, 1)
	ch <- envAt(1)
	close(ch) // torn down mid-replay, exactly as an overflow ends it

	got, reached, why := drainToHead(ch, func() error { return harness.ErrEventsOverflow },
		0, 99, 10*time.Millisecond, 30*time.Second)

	if reached {
		t.Fatal("want reached=false — the head never arrived")
	}
	if len(got) != 1 {
		t.Fatalf("want the one envelope seen before teardown, got %d", len(got))
	}
	if !errors.Is(why, harness.ErrEventsOverflow) {
		t.Fatalf("want the stop reason to name the overflow, got %v", why)
	}
}

// The other side of the same predicate: a genuine timeout must NOT be dressed
// up as an overflow. A guard that says "overflow" for every short read is as
// useless as one that says "timeout" for every short read — it just moves the
// wrong answer.
func TestDrainToHeadStillReportsATimeoutAsATimeout(t *testing.T) {
	ch := make(chan *vttv1.Envelope) // open, silent, never closed
	_, reached, why := drainToHead(ch, func() error { return nil },
		0, 99, 10*time.Millisecond, 80*time.Millisecond)

	if reached {
		t.Fatal("want reached=false")
	}
	if why == nil {
		t.Fatal("want a stop reason naming the timeout, got nil")
	}
	if errors.Is(why, harness.ErrEventsOverflow) {
		t.Fatalf("a silent stream is not an overflow, got %v", why)
	}
	// "non-nil and not an overflow" is NOT enough — a fault injection making
	// the deadline branch return streamClosedReason satisfied both and still
	// misdirected. The two reasons must be told APART, and structurally: an
	// earlier version of this test asserted on the wording, which pinned the
	// phrasing of an unexported helper and would have broken on a reword that
	// changed no behaviour.
	if !errors.Is(why, errCatchUpDeadline) {
		t.Fatalf("want the deadline named, got %v", why)
	}
	if errors.Is(why, errStreamClosed) {
		t.Fatalf("the stream never closed, got %v", why)
	}
}

// A connection that closes CLEANLY mid-replay is a third case: not this
// process's fault, not the clock. Reporting it as either would misdirect the
// same way, so it gets its own reason — and the server's OWN words survive
// into the message, because "reason = ..." is the fragment an operator acts on.
//
// The cause here is scripted realistically on purpose. A `func() error { return
// nil }` accessor is a FICTION: readLoop (internal/harness/client.go) calls
// teardown(err) in the loop body and closes c.events only in a deferred call,
// so there is no observable state where Events() is closed and CloseErr() is
// nil. Testing against the fiction let a real mutant live — wrapping the cause
// only for ErrEventsOverflow, which silently deletes the server's reason from
// every other message, passed the whole suite.
func TestDrainToHeadCarriesTheServersOwnReasonThroughACleanClose(t *testing.T) {
	cause := errors.New(`failed to get reader: received close frame: status = ` +
		`StatusNormalClosure and reason = "gateway: shutting down"`)
	ch := make(chan *vttv1.Envelope, 1)
	ch <- envAt(1)
	close(ch)

	_, reached, why := drainToHead(ch, func() error { return cause },
		0, 99, 10*time.Millisecond, 30*time.Second)

	if reached {
		t.Fatal("want reached=false")
	}
	if !errors.Is(why, errStreamClosed) {
		t.Fatalf("want the stream close named, got %v", why)
	}
	if errors.Is(why, harness.ErrEventsOverflow) {
		t.Fatalf("a clean close is not an overflow, got %v", why)
	}
	if !errors.Is(why, cause) {
		t.Fatalf("want the server's cause preserved for errors.Is, got %v", why)
	}
	if !strings.Contains(why.Error(), "shutting down") {
		t.Fatalf("want the server's reason string readable in the message, got %q", why)
	}
}

// The two `if reached` guards on the closed-channel and deadline branches are
// the difference between refusing to print a COMPLETE state and printing it.
// Both survived the whole suite as mutants until these two tests existed: no
// other test ever closes its channel after the head has been reached, and the
// three close tests all use head=99, which is never reached.

func TestDrainToHeadPrintsAStateWhoseStreamClosedAfterTheHead(t *testing.T) {
	// The ordinary production path: the server sends the backlog and closes.
	// Nothing is missing, so this must be printable and carry no stop reason.
	ch := make(chan *vttv1.Envelope, 2)
	ch <- envAt(1)
	ch <- envAt(2) // the announced head
	close(ch)

	got, reached, why := drainToHead(ch, func() error { return errors.New("normal closure") },
		0, 2, 10*time.Millisecond, 5*time.Second)

	if !reached {
		t.Fatal("want reached=true — the head arrived before the close")
	}
	if why != nil {
		t.Fatalf("a complete read has no stop reason, got %v", why)
	}
	if len(got) != 2 {
		t.Fatalf("want both envelopes, got %d", len(got))
	}
}

func TestDrainToHeadPrintsAStateStillBusyAtTheDeadline(t *testing.T) {
	// A campaign broadcasting faster than one envelope per quiet window never
	// lets the window elapse, so a long enough run reaches the DEADLINE with
	// the head long since passed. That is a complete read, not a truncation —
	// the deadline is a backstop, not a verdict.
	ch := make(chan *vttv1.Envelope)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) }) // closed by the test only, exactly once
	go func() {
		for i := int64(1); ; i++ {
			select {
			case ch <- envAt(i):
			case <-stop:
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	got, reached, why := drainToHead(ch, func() error { return nil },
		0, 2, 40*time.Millisecond, 200*time.Millisecond)

	if !reached {
		t.Fatal("want reached=true — head 2 passed long before the deadline")
	}
	if why != nil {
		t.Fatalf("a read that passed the head has no stop reason, got %v", why)
	}
	if len(got) < 2 {
		t.Fatalf("want at least the head, got %d", len(got))
	}
}

// A nil accessor must not panic. drainToHead is called from test helpers that
// hold a raw channel and have no connection to ask.
func TestDrainToHeadToleratesNoCloseErrAccessor(t *testing.T) {
	ch := make(chan *vttv1.Envelope)
	close(ch)
	_, reached, why := drainToHead(ch, nil, 0, 99, 10*time.Millisecond, time.Second)
	if reached {
		t.Fatal("want reached=false")
	}
	if why == nil {
		t.Fatal("want a stop reason even with no accessor")
	}
}
