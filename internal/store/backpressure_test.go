package store_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/store"
)

// A subscriber used to be dropped the instant a live event found its channel
// full, and that made an ATOMIC BATCH bigger than the buffer fatal to every
// connected subscriber — no matter how fast they were.
//
// The reason is scheduling, not slowness. notifyLocked runs under the store
// lock and pushes the whole batch in a tight loop with non-blocking sends, so
// a consumer goroutine has no realistic opportunity to drain between them. A
// client on an idle CPU and a fast socket was severed exactly as readily as a
// wedged one. The doc justified the drop with "a subscriber that falls
// behind", which described neither.
//
// Concretely: loading an adventure appends its compiled events as one batch,
// so the buffer constant became a ceiling on how large an adventure could be
// before loading it disconnected the table. That is the coupling these tests
// break — burst size is now bounded by memory and a liveness timeout, never by
// a constant someone picked for something else.
//
// Prior art (MapTool, net.rptools.clientserver): an unbounded per-connection
// outQueue drained by a dedicated send thread, with liveness decided by a
// socket timeout against a 20s client heartbeat — never by queue depth.

func TestABurstLargerThanTheBufferDoesNotDropAKeepingUpSubscriber(t *testing.T) {
	s := openTemp(t)
	// A deliberately tiny buffer: the point is that buffer size no longer
	// decides who survives a batch.
	ch, cancel, _, err := s.Subscribe(0, 4)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	// Publish the whole burst with NOBODY reading — the shape of an atomic
	// AppendBatch, and deterministic: under the old policy the fifth event
	// finds the channel full and closes this subscriber.
	const burst = 200
	for i := range burst {
		env := newEnv(envID(i))
		s.Append(env)
		s.Notify(env)
	}

	for i := range burst {
		got := recv(t, ch)
		if got.EventId != envID(i) {
			t.Fatalf("event %d = %q, want %q — order must survive the queue", i, got.EventId, envID(i))
		}
	}
}

func TestOneWedgedSubscriberDoesNotAffectTheOthers(t *testing.T) {
	// The property the old drop policy existed to protect, and which must
	// survive the change: no subscriber may block appends or starve its peers.
	s := openTemp(t)
	wedged, cancelWedged, _, _ := s.Subscribe(0, 1)
	defer cancelWedged()
	healthy, cancelHealthy, _, _ := s.Subscribe(0, 1)
	defer cancelHealthy()

	const burst = 50
	for i := range burst {
		env := newEnv(envID(i))
		s.Append(env)
		s.Notify(env) // must return promptly despite `wedged` never reading
	}

	for i := range burst {
		if got := recv(t, healthy); got.EventId != envID(i) {
			t.Fatalf("healthy subscriber event %d = %q, want %q", i, got.EventId, envID(i))
		}
	}

	// And the wedged one is still OPEN: nothing about this burst was its
	// fault, and its budget has not elapsed. Asserting it — rather than
	// leaving it as scenery — is what makes this test notice a regression
	// that severs a subscriber for being behind rather than for not reading.
	select {
	case _, ok := <-wedged:
		if !ok {
			t.Fatal("wedged subscriber closed by a burst alone — depth is not supposed to be the trigger")
		}
	default:
	}
}

// The replacement for the old depth-based drop: a consumer that stops reading
// is still cut loose, but on a TIMER, because "has not consumed for 30s while
// events were waiting" is the actual question — and unlike a depth, it cannot
// turn into a limit on how large an adventure may be.
func TestASubscriberThatNeverReadsIsDroppedAfterTheNoProgressTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := openTemp(t)
		ch, cancel, _, _ := s.Subscribe(0, 1)
		defer cancel()

		for i := range 8 {
			env := newEnv(envID(i))
			s.Append(env)
			s.Notify(env)
		}

		// Past the timeout with nothing consumed.
		time.Sleep(store.SubscriberNoProgressTimeout + time.Second)
		synctest.Wait()

		// Drain: whatever was buffered may still be readable, but the channel
		// must end CLOSED rather than stay open forever holding memory.
		closed := false
		for range 64 {
			select {
			case _, ok := <-ch:
				if !ok {
					closed = true
				}
			default:
			}
			if closed {
				break
			}
		}
		if !closed {
			t.Fatal("a subscriber that never consumed anything must be dropped once the " +
				"no-progress timeout elapses — otherwise a wedged client holds events forever")
		}
	})
}

func TestASubscriberThatKeepsConsumingIsNeverDropped(t *testing.T) {
	// The mirror-image failure, and the one that would be worse: a timeout
	// that fires on a live connection disconnects the table for no reason.
	// Each read is progress, so a slow-but-alive consumer must survive
	// arbitrarily long — well past the timeout.
	synctest.Test(t, func(t *testing.T) {
		s := openTemp(t)
		// buffer 0, NOT 1. With a free slot every hand-off completes
		// instantly and the pump spends each dawdle idle in its wake/done
		// select with NO TIMER ARMED — the test would claim to exercise the
		// budget while never arming it. Unbuffered forces the hand-off to
		// park, which is the only state in which the timer runs at all.
		ch, cancel, _, _ := s.Subscribe(0, 0)
		defer cancel()

		for i := range 6 {
			env := newEnv(envID(i))
			s.Append(env)
			s.Notify(env)

			// Dawdle for most of the budget, then consume. Repeated, this far
			// exceeds the timeout in total elapsed time without ever once
			// failing to make progress.
			time.Sleep(store.SubscriberNoProgressTimeout - time.Second)
			synctest.Wait()

			got, ok := <-ch
			if !ok {
				t.Fatalf("subscriber dropped at event %d despite consuming every one", i)
			}
			if got.EventId != envID(i) {
				t.Fatalf("event %d = %q, want %q", i, got.EventId, envID(i))
			}
		}
	})
}

func envID(i int) string {
	return string(rune('a'+i/26)) + string(rune('a'+i%26))
}
