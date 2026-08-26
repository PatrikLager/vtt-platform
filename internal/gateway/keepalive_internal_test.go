package gateway

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// These tests are about the SECOND failure this keepalive had, and it was not
// the one it was built for. The first draft reaped a peer whenever conn.Ping
// returned an error — but Ping fails for several unrelated reasons, and only
// one of them is both a statement about the peer AND ours to act on. The full
// taxonomy lives on pingUntilStopped in keepalive.go and is deliberately not
// restated here; a second copy is a second thing to go stale, and an earlier
// version of this header already carried a shorter list that stopped matching.
// The two that these tests exercise:
//
//  1. the pong never came back inside our budget — the peer is gone, reap it;
//  2. the ping FRAME could not be sent, because coder/websocket wraps every
//     control frame in a hard-coded five-second context (write.go:277, pinned
//     v1.8.15) and a data write was holding writeFrameMu for longer than that.
//
// (2) says nothing whatever about the peer. It says our own writer is busy.
// And it is not a corner: gatewayPingInterval (20s) is SHORTER than
// Server.writeTimeout (30s), so any write that uses its full budget straddles
// a tick by construction. A client that was merely receiving a large frame got
// killed at five seconds, by the very policy whose doc comment promised it
// could be idle without being kicked out.
//
// THE FIRST DRAFT OF THESE TESTS HAD THE OPPOSITE HOLE, and a reviewer found
// it: every assertion was a negative one. They proved the keepalive does not
// reap and never once proved that it does, so `err == nil` -> `err != nil` on
// the verdict line — a default-enabled gremlins mutant on a gated package —
// survived the whole suite while switching reaping off entirely. A dead peer
// would have been pinged forever and the table would have kept a ghost. That
// is strictly worse than the bug being fixed here, which is why
// TestAPongThatNeverComesReapsThePeer leads.
//
// All of these run in a synctest bubble: the ticker is the subject, and a fake
// clock makes "ten intervals passed and nothing happened" an assertion rather
// than a sleep long enough to hope.

// TestAPongThatNeverComesReapsThePeer is the positive case, and the one the
// whole file exists to make true.
//
// The injected ping blocks until our own budget expires and then reports that
// deadline — which is exactly what conn.Ping does when the frame goes out and
// no pong ever comes back (conn.go:255 wraps ctx.Err()). Without this, every
// other test here passes on a keepalive that never reaps anyone.
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

// TestThePongBudgetStaysAtLeastThreeIntervals pins the one relationship
// gatewayPingTimeout's doc comment calls "a floor rather than a nicety" and
// that nothing otherwise enforces.
//
// Three intervals is what lets two pongs be lost or arrive late before anyone
// is declared dead, and it is the entire reason this is safe to point at a
// real table instead of a loopback. Patrik chose 60s against 40s and 20s on
// 2026-08-26 for exactly that margin. Tighten either constant without the
// other and the argument in the comment silently stops describing the code —
// which is the failure this repo keeps finding in prose that no test reads.
//
// DISCLOSED, because it changes how much weight this deserves: this test was
// written because golangci-lint's `unused` flagged both constants (nothing
// wires them until the serve() seam lands). It is kept because it turned out
// to be a real guard on a stated invariant, not because it silences a linter —
// but it would not have been written otherwise, and a reader should know that.
//
// The positive floor check is not redundant either, but NOT for the reason it
// looks like: without it an interval of zero makes the ratio vacuously true,
// and that is worth guarding — against a HUMAN edit, not a mutant. Both
// constants sit on `const` declarations, which emit no statement and so never
// appear in a coverage profile, which is why gremlins reports them NOT COVERED
// and never executes them. Nothing written here can move them to KILLED. In a
// mutation-gated package a check like this reads as a kill claim; it is not
// one, and saying so is cheaper than letting the next reader assume it.
func TestThePongBudgetStaysAtLeastThreeIntervals(t *testing.T) {
	if gatewayPingInterval <= 0 || gatewayPingTimeout <= 0 {
		t.Fatalf("both budgets must be positive; interval=%v timeout=%v — a non-positive "+
			"interval makes time.NewTicker panic and makes the ratio below vacuous",
			gatewayPingInterval, gatewayPingTimeout)
	}
	if floor := 3 * gatewayPingInterval; gatewayPingTimeout < floor {
		t.Errorf("gatewayPingTimeout is %v against a %v interval (%.1fx); the doc comment "+
			"argues 3x is a FLOOR, because at less than that a single late pong from a phone "+
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
