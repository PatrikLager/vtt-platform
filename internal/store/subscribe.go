package store

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

type subscriber struct {
	ch      chan *vttv1.Envelope
	closed  bool
	lastSeq int64
}

// Subscribe delivers every event with seq > afterSeq: history first (loaded
// atomically under the store lock, so no gap or duplicate versus live
// appends), then live events. buffer bounds the channel beyond the catch-up
// batch; a subscriber that falls behind has its channel closed — the log is
// the recovery path, and no subscriber may block appends.
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
// unrelated per-subscriber dedupe in notifyLocked, which would otherwise
// mask the guard at the more "realistic" afterSeq=0. Clamping negative
// afterSeq to 0 here would silently break that test's teeth.
// The third return value is the CATCH-UP HEAD: the highest sequence preloaded
// into the subscriber's buffer before Subscribe returned, or afterSeq when the
// log had nothing newer. It is exact — computed under the same lock as the
// preload — and it is the number a client needs to know when catch-up has
// ENDED, which the wire could not previously express (contract CatchUpHead).
func (s *Store) Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), int64, error) {
	if buffer < 0 {
		return nil, nil, 0, fmt.Errorf("store: negative subscribe buffer %d", buffer)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	history, err := s.readAfterLocked(afterSeq)
	if err != nil {
		return nil, nil, 0, err
	}
	sub := &subscriber{ch: make(chan *vttv1.Envelope, len(history)+buffer), lastSeq: afterSeq}
	for _, env := range history {
		sub.ch <- env // fits by construction
		sub.lastSeq = env.Sequence
	}
	s.subs = append(s.subs, sub)

	// cancel marks sub closed but does not remove it from s.subs itself —
	// dropLocked only flips the closed flag and closes the channel. The
	// closed subscriber stays in s.subs until the next Notify's
	// notifyLocked compaction pass sweeps it out. Deliberate lazy
	// reclamation: cancel doesn't need to touch the slice under lock beyond
	// the single dropLocked call, and the compaction it defers to already
	// runs on every Notify regardless.
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.dropLocked(sub)
	}
	return sub.ch, cancel, sub.lastSeq, nil
}

// notifyLocked delivers env to all live subscribers; callers hold s.mu.
// Skips any subscriber whose lastSeq already covers env.Sequence — either
// because its catch-up preload already included it, or a prior notifyLocked
// call already delivered it. This makes Notify idempotent per subscriber
// and closes the race where Subscribe lands between an event's persist and
// its caller's later Notify call: the catch-up preload already delivered
// the event, so the raced Notify is a no-op for that subscriber.
func (s *Store) notifyLocked(env *vttv1.Envelope) {
	for _, sub := range s.subs {
		if sub.closed {
			continue
		}
		if env.Sequence <= sub.lastSeq {
			continue
		}
		select {
		case sub.ch <- env:
			sub.lastSeq = env.Sequence
		default: // overflow: close, drop
			s.dropLocked(sub)
		}
	}
	// compact dropped subscribers
	live := s.subs[:0]
	for _, sub := range s.subs {
		if !sub.closed {
			live = append(live, sub)
		}
	}
	s.subs = live
}

func (s *Store) dropLocked(sub *subscriber) {
	if !sub.closed {
		sub.closed = true
		close(sub.ch)
	}
}
