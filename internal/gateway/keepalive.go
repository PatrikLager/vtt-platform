package gateway

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type writeActivity struct{ inFlight atomic.Bool }

func (w *writeActivity) begin()     { w.inFlight.Store(true) }
func (w *writeActivity) end()       { w.inFlight.Store(false) }
func (w *writeActivity) busy() bool { return w.inFlight.Load() }

const gatewayPingInterval = 20 * time.Second

// Keep gatewayPingTimeout at least three times gatewayPingInterval: at less, one
// late pong from a slow link reaps a healthy peer
// (TestThePongBudgetStaysAtLeastThreeIntervals, SPEC-011).
const gatewayPingTimeout = 60 * time.Second

// keepAlive must run on its own goroutine, never the writer's: Conn.Ping blocks
// until the pong, and only a concurrent Reader reads it (SPEC-011).
func keepAlive(
	ctx context.Context,
	interval time.Duration,
	stop <-chan struct{},
	busy func() bool,
	ping func() error,
	fail func(),
) {
	// A ticker, not a sleep loop: a slow ping must not push the next tick out.
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		// Keep this arm: a send failure does not end the loop, so without it a
		// closed connection whose stop was never closed is pinged forever.
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-t.C:
			// Keep the skip: a ping into a busy writer loses the library's five-second
			// control-frame race, and one that wins the lock on a stalled socket is
			// closed by the library itself (SPEC-011).
			if busy() {
				continue
			}
			if ping() != nil {
				fail()
				return
			}
		}
	}
}

// closeNow, never a reasoned Close: a peer that missed its pong reads no close
// frame. A reap that coincides with the pump's Close costs the peer its reason
// (SPEC-011).
func pingUntilStopped(
	ctx context.Context,
	interval, timeout time.Duration,
	stop <-chan struct{},
	busy func() bool,
	ping func(context.Context) error,
	closeNow func(),
) {
	keepAlive(ctx, interval, stop, busy, func() error {
		pctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		err := ping(pctx)
		// Reap only on pctx's deadline: a send failure or net.ErrClosed says nothing
		// about the peer, and the read loop's own error is what ends a dead socket
		// (SPEC-011).
		if err == nil || !errors.Is(pctx.Err(), context.DeadlineExceeded) {
			return nil
		}
		return err
	}, closeNow)
}

// Keep the stamping in this helper, wrapped around serve's real write: the
// busy skip is inert without it, and no test pins that join.
func stampedWrite(a *writeActivity, write func([]byte) bool) func([]byte) bool {
	return func(b []byte) bool {
		a.begin()
		defer a.end()
		return write(b)
	}
}
