package store

import (
	"fmt"
	"sync"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// SubscriberNoProgressTimeout is how long a subscriber may fail to consume an
// event that is WAITING FOR IT before the store cuts it loose.
//
// It replaces a depth-based drop, and the difference is the point. Depth
// answered "is this subscriber more than N events behind", which sounds like a
// liveness test and is not one: an atomic batch is delivered in a tight loop
// under the store lock, so a consumer goroutine cannot drain between sends no
// matter how fast it is. Every subscriber was severed by a batch larger than
// its buffer, and that buffer constant thereby became a ceiling on how large
// an adventure could be before loading it disconnected the table.
//
// A timer asks the question actually worth asking — "is anyone still reading?"
// A FULLY wedged consumer therefore holds at most this long of events before
// it is dropped and the log becomes its recovery path, exactly as before.
//
// Be precise about what that does NOT bound. The timer restarts on every
// successful hand-off, so a consumer reading one event per 29s never trips it
// and `pending` has no cap: measured during review, such a consumer was still
// live after 9m40s holding 380 undelivered events. That is the intended
// SEMANTIC — a slow-but-alive client must not be severed, which is the whole
// point — but it is not a memory bound, and this comment previously claimed it
// was. A ceiling sized by memory (far above any adventure) is the missing
// piece; it is deliberately not a ceiling sized by content.
//
// 30s, chosen against the closest prior art: MapTool
// (net.rptools.clientserver) settles the same question the same way — an
// UNBOUNDED per-connection queue drained by a dedicated send thread, liveness
// decided by a one-minute socket timeout against a 20s client heartbeat, and
// queue depth never consulted.
const SubscriberNoProgressTimeout = 30 * time.Second

// subscriber owns one consumer's outbound queue. notifyLocked appends to
// pending under the store lock and returns; a dedicated pump goroutine hands
// events to ch at whatever rate the consumer accepts them. Appends therefore
// never block on a slow reader — the invariant the old drop policy protected —
// without that reader's buffer bounding what anyone else may publish.
type subscriber struct {
	ch   chan *vttv1.Envelope
	done chan struct{}
	stop sync.Once

	// wake carries "pending is non-empty" to the pump. Buffered 1 and sent
	// non-blockingly: it is an edge, not a count.
	wake chan struct{}

	mu      sync.Mutex
	pending []*vttv1.Envelope
	// lastSeq is the highest sequence QUEUED, not delivered. It is what makes
	// Notify idempotent per subscriber and what closes the race where
	// Subscribe lands between an event's persist and its caller's Notify.
	lastSeq int64
	closed  bool
}

// enqueue accepts env for later delivery. Never blocks and never drops —
// callers hold the store lock, and nothing a consumer does may stall an
// append.
func (sub *subscriber) enqueue(env *vttv1.Envelope) {
	sub.mu.Lock()
	if sub.closed || env.Sequence <= sub.lastSeq {
		sub.mu.Unlock()
		return
	}
	sub.pending = append(sub.pending, env)
	sub.lastSeq = env.Sequence
	sub.mu.Unlock()

	select {
	case sub.wake <- struct{}{}:
	default: // already signalled; the pump will see the new tail
	}
}

func (sub *subscriber) next() (*vttv1.Envelope, bool) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed || len(sub.pending) == 0 {
		return nil, false
	}
	env := sub.pending[0]
	// Nil the slot before resliding: the backing array outlives the reslice,
	// so without this every delivered envelope up to the high-water mark stays
	// reachable whenever the queue never fully drains.
	sub.pending[0] = nil
	sub.pending = sub.pending[1:]
	if len(sub.pending) == 0 {
		// Release the backing array whenever the queue drains, so a long-lived
		// subscriber does not retain its high-water allocation forever.
		sub.pending = nil
	}
	return env, true
}

func (sub *subscriber) isClosed() bool {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.closed
}

// markStopped ends the subscription. Idempotent, and safe to call from either
// the store (unsubscribe, Close) or the pump itself (no-progress timeout).
//
// It does NOT close ch. Only the pump does that, on its way out, so a
// concurrent hand-off can never send on a closed channel.
func (sub *subscriber) markStopped() {
	sub.mu.Lock()
	sub.closed = true
	sub.pending = nil
	sub.mu.Unlock()
	sub.stop.Do(func() { close(sub.done) })
}

// pump is the ONLY writer to ch and the only closer of it.
func (sub *subscriber) pump(timeout time.Duration) {
	defer close(sub.ch)
	for {
		env, ok := sub.next()
		if !ok {
			select {
			case <-sub.wake:
				continue
			case <-sub.done:
				return
			}
		}
		select {
		case sub.ch <- env:
			// Progress. The timer below only runs while an event is WAITING,
			// so a subscriber is judged on whether it is reading — never on
			// how much it was sent.
		case <-sub.done:
			return
		case <-time.After(timeout):
			sub.markStopped()
			return
		}
	}
}

// Subscribe delivers every event with seq > afterSeq: history first (read
// atomically under the store lock, so no gap or duplicate versus live
// appends), then live events.
//
// buffer sizes the hand-off channel — slack for a consumer that reads in
// bursts, NOT a limit on how far behind it may fall. That is
// SubscriberNoProgressTimeout's job, and the distinction is why a large
// AppendBatch no longer disconnects every connected subscriber.
//
// A subscriber that stops reading has its channel closed once that timeout
// elapses; the log is the recovery path, and no subscriber may block appends.
//
// Delivered envelopes are shared pointers: the same *vttv1.Envelope handed
// to one subscriber's channel is handed to every other subscriber's channel
// (and returned from ReadAfter/State callers) too. Consumers must treat them
// as immutable — mutating a received envelope corrupts what every other
// subscriber sees.
//
// afterSeq accepts negative values unclamped (no lower bound is enforced —
// only buffer is validated above): this is deliberate, not an oversight.
// TestNotifyIgnoresZeroSequence (subscribe_test.go) relies on subscribing
// from afterSeq=-1 to isolate Notify's zero-sequence guard from the
// unrelated per-subscriber dedupe in enqueue, which would otherwise
// mask the guard at the more "realistic" afterSeq=0. Clamping negative
// afterSeq to 0 here would silently break that test's teeth.
//
// The third return value is the CATCH-UP HEAD: the highest sequence QUEUED for
// this subscriber before Subscribe returned, or afterSeq when the log had
// nothing newer. It is exact — computed under the same lock as the queueing —
// and it is the number a client needs to know when catch-up has ENDED, which
// the wire could not previously express (contract CatchUpHead). It is now
// QUEUED rather than already resident in the channel; the sequence a consumer
// eventually reads is unchanged.
func (s *Store) Subscribe(afterSeq int64, buffer int) (events <-chan *vttv1.Envelope, unsubscribe func(), catchUpHead int64, err error) {
	return s.SubscribeWithNoProgressTimeout(afterSeq, buffer, 0)
}

// SubscribeWithNoProgressTimeout is Subscribe with an explicit liveness
// budget. A non-positive noProgress means SubscriberNoProgressTimeout, so the
// constant stays the single place the default lives.
//
// The gateway sets this the same way it sets buffer: it is the layer that
// knows what a stalled CONNECTION is, and its own tests need a budget shorter
// than a wall-clock half-minute. Tests elsewhere use Subscribe and get the
// default.
func (s *Store) SubscribeWithNoProgressTimeout(afterSeq int64, buffer int, noProgress time.Duration) (events <-chan *vttv1.Envelope, unsubscribe func(), catchUpHead int64, err error) {
	if noProgress <= 0 {
		noProgress = SubscriberNoProgressTimeout
	}
	if buffer < 0 {
		return nil, nil, 0, fmt.Errorf("store: negative subscribe buffer %d", buffer)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	history, err := s.readAfterLocked(afterSeq)
	if err != nil {
		return nil, nil, 0, err
	}
	sub := &subscriber{
		ch:      make(chan *vttv1.Envelope, buffer),
		done:    make(chan struct{}),
		wake:    make(chan struct{}, 1),
		pending: history,
		lastSeq: afterSeq,
	}
	if n := len(history); n > 0 {
		sub.lastSeq = history[n-1].Sequence
	}
	s.subs = append(s.subs, sub)
	go sub.pump(noProgress)

	// NOT `cancel`. It reads as a context.CancelFunc to any Go programmer, and
	// it is not one — it ends a SUBSCRIPTION and touches no context. That
	// misreading cost a real debugging session: a defect was diagnosed as
	// "shutdown cancels the connection context" and written up that way in the
	// backlog, from the name alone, when this function never touches a context
	// at all. The name is the fix; a comment explaining the name would have
	// been read just as little.
	//
	// It marks sub stopped but does not remove it from s.subs itself. The
	// stopped subscriber stays in s.subs until the next Notify's compaction
	// pass sweeps it out. Deliberate lazy reclamation: it need not touch the
	// slice under lock beyond the single dropLocked call, and the compaction it
	// defers to already runs on every Notify regardless.
	unsubscribe = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.dropLocked(sub)
	}
	return sub.ch, unsubscribe, sub.lastSeq, nil
}

// notifyLocked queues env for all live subscribers; callers hold s.mu.
// Skips any subscriber whose lastSeq already covers env.Sequence — either
// because its catch-up preload already included it, or a prior notifyLocked
// call already queued it. This makes Notify idempotent per subscriber and
// closes the race where Subscribe lands between an event's persist and its
// caller's later Notify call: the catch-up preload already queued the event,
// so the raced Notify is a no-op for that subscriber.
func (s *Store) notifyLocked(env *vttv1.Envelope) {
	for _, sub := range s.subs {
		sub.enqueue(env)
	}
	// compact stopped subscribers
	live := s.subs[:0]
	for _, sub := range s.subs {
		if !sub.isClosed() {
			live = append(live, sub)
		}
	}
	s.subs = live
}

func (s *Store) dropLocked(sub *subscriber) {
	sub.markStopped()
}
