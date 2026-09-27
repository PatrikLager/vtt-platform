package gateway

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// A ping is only ever a verdict about the pong (SPEC-011): a pong lost past
// the budget reaps the peer, a ping frame the busy writer could not send does
// not. Keep TestAPongThatNeverComesReapsThePeer first and positive: a suite
// of negative assertions alone stays green with reaping switched off. The
// pinger tests run in a synctest bubble, so "ten intervals passed and
// nothing happened" is an assertion rather than a sleep.
// VTT-092
func TestAPongThatNeverComesReapsThePeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = time.Second
		const pongBudget = 3 * interval

		stop := make(chan struct{})
		defer close(stop)
		reaped := make(chan struct{})

		go pingUntilStopped(context.Background(), interval, pongBudget, stop,
			func() bool { return false },
			func(pctx context.Context) error { <-pctx.Done(); return pctx.Err() },
			func() { close(reaped) },
		)

		time.Sleep(interval + pongBudget + interval)
		synctest.Wait()

		select {
		case <-reaped:
		default:
			t.Fatal("the pong budget expired with no answer and nobody was reaped — a peer " +
				"that has silently gone stays CONNECTED at the table forever, because " +
				"departure hangs off serve() returning and serve() is parked in conn.Read on " +
				"a socket nobody has told it is gone")
		}
	})
}

// TestThePongBudgetStaysAtLeastThreeIntervals pins the ratio SPEC-011 states
// between the two constants and nothing else enforces. Not a kill claim: both
// sit on const declarations, which no coverage profile sees, so gremlins
// reports them NOT COVERED whatever this asserts. The positive check guards a
// zero interval, which makes the ratio vacuous.
// VTT-095
func TestThePongBudgetStaysAtLeastThreeIntervals(t *testing.T) {
	if gatewayPingInterval <= 0 || gatewayPingTimeout <= 0 {
		t.Fatalf("both budgets must be positive; interval=%v timeout=%v — a non-positive "+
			"interval makes time.NewTicker panic and makes the ratio below vacuous",
			gatewayPingInterval, gatewayPingTimeout)
	}
	if floor := 3 * gatewayPingInterval; gatewayPingTimeout < floor {
		t.Errorf("gatewayPingTimeout is %v against a %v interval (%.1fx); SPEC-011 "+
			"holds 3x as a FLOOR, because at less than that a single late pong from a phone "+
			"on a slow cell reaps a player who is perfectly fine — want >= %v",
			gatewayPingTimeout, gatewayPingInterval,
			float64(gatewayPingTimeout)/float64(gatewayPingInterval), floor)
	}
}

// TestNoPingGoesOutWhileTheWriterIsBusy pins the cheaper half of the fix, and
// the one that removes the contention rather than merely surviving it.
//
// A socket with data flowing on it does not need a keepalive — the keepalive
// exists to stop an IDLE connection looking dead to intermediaries, and a
// connection mid-write is not idle. Skipping the tick means the ping never
// contends for writeFrameMu at all.
//
// Detection is not weakened by this. While the writer works, writeTimeout
// (30s) is already asking whether the peer reads; when the writer goes quiet,
// the next tick pings. The two cover disjoint halves of one question — and the
// second half is asserted here rather than merely claimed: nothing else in
// this package notices a busy check that skips a tick and never re-enables,
// which would leave any connection that was ever busy with no keepalive at all.
//
// It is NOT here to catch `continue` becoming `break`. A reviewer proposed that
// mutant as the reason and was wrong about the language: Go's break terminates
// the innermost for, switch, or SELECT, so inside this select a break leaves
// the select and the loop iterates exactly as continue does. Verified against
// the spec, a scratch program, and the mutation itself, which passes the whole
// suite because it is equivalent. Recorded because the wrong reason is more
// durable than the right one once it is written down.
// VTT-094
func TestNoPingGoesOutWhileTheWriterIsBusy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = time.Second
		const busyRounds = 10

		// Closed explicitly below, not deferred: this test asserts that
		// keepAlive RETURNS when stop closes, so the close has to happen while
		// the bubble is still running and observable.
		stop := make(chan struct{})
		done := make(chan struct{})

		var busy atomic.Bool
		busy.Store(true)
		var pings atomic.Int64

		go func() {
			defer close(done)
			keepAlive(context.Background(), interval, stop,
				busy.Load,
				func() error { pings.Add(1); return nil },
				func() { t.Error("fail() ran on a connection that was never even pinged") },
			)
		}()

		time.Sleep(busyRounds * interval)
		synctest.Wait()

		if n := pings.Load(); n != 0 {
			t.Fatalf("the writer was busy for every one of %d intervals and %d ping(s) still "+
				"went out — each one contends for writeFrameMu against a write that may hold "+
				"it for up to writeTimeout, and coder/websocket gives a control frame only 5s "+
				"to win that race before reporting a failure this loop reads as peer death",
				busyRounds, n)
		}

		// The writer finishes. The keepalive must come back — a skip is a skip,
		// not a stop.
		busy.Store(false)
		time.Sleep(2 * interval)
		synctest.Wait()

		if n := pings.Load(); n == 0 {
			t.Fatal("the writer went quiet and no ping followed — the busy check skipped the " +
				"tick permanently instead of skipping that one tick, so a connection that was " +
				"ever busy is left with no keepalive at all and reaps silently to the first " +
				"intermediary that notices")
		}

		close(stop)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Error("keepAlive did not return after stop closed")
		}
	})
}

// TestASendFailureIsNotAVerdictAboutThePeer pins the half that decides whether
// somebody stays at the table.
//
// The injected error is the exact shape writeControl produces when it loses
// the race for writeFrameMu: its own five-second context expired while OURS —
// the sixty-second pong budget — has barely started. Nothing has been learned
// about the peer, so nothing may be concluded about it.
//
// This is the direction that would be catastrophic to get wrong, and it is
// invisible from the outside: a spuriously reaped player sees exactly what a
// genuinely disconnected one sees. The bug being fixed cost one person their
// connection once; this failure mode costs everyone theirs, repeatedly,
// whenever the table is busy enough to matter.
// VTT-093
func TestASendFailureIsNotAVerdictAboutThePeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = time.Second
		const pongBudget = 60 * time.Second

		stop := make(chan struct{})
		defer close(stop)
		var pings atomic.Int64
		reaped := make(chan struct{})

		go pingUntilStopped(context.Background(), interval, pongBudget, stop,
			func() bool { return false }, // writer idle, so the tick is not skipped
			func(pctx context.Context) error {
				pings.Add(1)
				if err := pctx.Err(); err != nil {
					t.Errorf("our own pong budget expired, which this test is not about: %v", err)
				}
				return fmt.Errorf("failed to write control frame %v: %w",
					"opPing", context.DeadlineExceeded)
			},
			func() { close(reaped) },
		)

		time.Sleep(5 * interval)
		synctest.Wait()

		select {
		case <-reaped:
			t.Fatal("a ping that could not be SENT was treated as proof the peer is gone — " +
				"writeControl's own hard 5s expired while our 60s pong budget had barely " +
				"started, so the only thing demonstrated was that our writer held the frame " +
				"lock, and a live player was force-closed for it")
		default:
		}

		if n := pings.Load(); n < 2 {
			t.Errorf("the loop stopped after %d ping(s); a send failure must not end the "+
				"keepalive either, since the next interval may well find the writer idle", n)
		}
	})
}

// TestACancelledConnectionStopsItsOwnPinger pins the exit that stopped
// existing the moment a send failure stopped being a verdict.
//
// Before the verdict fix, ANY error ended this loop, including the
// net.ErrClosed a closed connection hands back — so a dead connection reaped
// its own pinger as a side effect of being wrong about everything else. Now
// the only way out is an explicit one, and depending on the caller to close
// stop on every path including a panic is the honour-system arrangement this
// repo keeps rediscovering the failure of. ctx is the second exit, and it is
// tested rather than trusted.
func TestACancelledConnectionStopsItsOwnPinger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = time.Second

		ctx, cancel := context.WithCancel(context.Background())
		stop := make(chan struct{}) // deliberately never closed
		done := make(chan struct{})

		go func() {
			defer close(done)
			keepAlive(ctx, interval, stop,
				func() bool { return false },
				func() error { return nil },
				func() { t.Error("fail() ran on a cancelled connection, which is teardown, not peer death") },
			)
		}()

		time.Sleep(2 * interval)
		cancel()
		synctest.Wait()

		select {
		case <-done:
		default:
			t.Fatal("the connection's ctx was cancelled and its pinger kept running — with " +
				"stop unclosed this goroutine, its ticker and one futile Ping per interval " +
				"outlive the connection they belong to, for as long as the process does")
		}
	})
}

// TestWriteActivityReportsAWriteInFlight pins the seam serve() hands keepAlive
// as its busy predicate.
//
// Three one-line methods look too small to test, and that is exactly the shape
// this package has been burned by: the last seam here was argued for in a doc
// comment, reached only through a live connection, and turned out to be inert
// under mutation. A predicate stuck at false silently disables the skip; a
// predicate stuck at true silently disables the KEEPALIVE, and neither shows up
// as a failure anywhere else. Both directions are asserted.
// VTT-094
func TestWriteActivityReportsAWriteInFlight(t *testing.T) {
	var w writeActivity

	if w.busy() {
		t.Error("a connection that has never written reports its writer busy — every tick " +
			"would be skipped and the connection would never be pinged at all")
	}

	w.begin()
	if !w.busy() {
		t.Error("a write is in flight and the writer does not report busy — the ping will " +
			"contend for writeFrameMu against it and lose the 5s race the library allows a " +
			"control frame")
	}

	w.end()
	if w.busy() {
		t.Error("the write finished and the writer still reports busy — a skip that never " +
			"re-enables leaves the connection with no keepalive, which is the bug this whole " +
			"file exists to prevent")
	}
}

// TestTheWriterIsReportedBusyForExactlyTheDurationOfAWrite pins the JOIN
// between writeActivity and the writer — the one part of this mechanism that
// nothing else observes.
//
// keepAlive is driven by tests with an INJECTED busy predicate, and
// writeActivity is driven directly, so both halves are covered and the wiring
// between them was not: deleting the begin() stamp from serve() left every test
// in this package green. A connection-level test cannot close that gap either,
// because a client that stops reading (the only way to hold a write open) also
// stops processing the ping frames the assertion would need to count.
//
// So the stamping lives in a helper and is asserted here, deterministically,
// with no sockets and no clock.
// VTT-094
func TestTheWriterIsReportedBusyForExactlyTheDurationOfAWrite(t *testing.T) {
	var a writeActivity
	var busyDuring bool

	w := stampedWrite(&a, func([]byte) bool {
		busyDuring = a.busy()
		return true
	})

	if a.busy() {
		t.Error("a connection reports its writer busy before it has written anything — " +
			"every keepalive tick would be skipped and the connection never pinged at all")
	}

	w([]byte("a frame"))

	if !busyDuring {
		t.Error("the writer was NOT reported busy while its write was in flight — the ping " +
			"is then free to contend for writeFrameMu against a write that may hold it for " +
			"up to writeTimeout, and coder/websocket gives a control frame 5s to win that " +
			"race before reporting a failure. That is the exact defect this seam exists to " +
			"prevent, and with the stamp inline in serve() nothing in this package noticed " +
			"its absence")
	}

	if a.busy() {
		t.Error("the write returned and the writer still reports busy — a skip that never " +
			"re-enables leaves the connection with no keepalive for the rest of its life")
	}
}
