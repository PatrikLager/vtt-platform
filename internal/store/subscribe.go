package store

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

type subscriber struct {
	ch     chan *vttv1.Envelope
	closed bool
}

// Subscribe delivers every event with seq > afterSeq: history first (loaded
// atomically under the store lock, so no gap or duplicate versus live
// appends), then live events. buffer bounds the channel beyond the catch-up
// batch; a subscriber that falls behind has its channel closed — the log is
// the recovery path, and no subscriber may block appends.
func (s *Store) Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history, err := s.readAfterLocked(afterSeq)
	if err != nil {
		return nil, nil, err
	}
	sub := &subscriber{ch: make(chan *vttv1.Envelope, len(history)+buffer)}
	for _, env := range history {
		sub.ch <- env // fits by construction
	}
	s.subs = append(s.subs, sub)

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.dropLocked(sub)
	}
	return sub.ch, cancel, nil
}

// notifyLocked delivers env to all live subscribers; callers hold s.mu.
func (s *Store) notifyLocked(env *vttv1.Envelope) {
	for _, sub := range s.subs {
		if sub.closed {
			continue
		}
		select {
		case sub.ch <- env:
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
