package store_test

import "testing"

// Subscribe's catch-up head is what finally lets a client know when catch-up
// has ENDED. Everything downstream rests on it — the gateway's CatchUpHead
// frame, harness.Client.CatchUpHead, and `vtt state dump` refusing to print a
// truncated state — so it is pinned at the source.
//
// Before it existed, clients inferred that boundary from a 300ms silence, and
// a slow moment mid-replay produced a silently short read: a dump that looked
// complete, and (in the soak harness) a report that the FOLD had diverged.

func TestSubscribeHeadIsTheHighestPreloadedSequence(t *testing.T) {
	s := openTemp(t)
	for _, id := range []string{"e1", "e2", "e3", "e4", "e5"} {
		s.Append(newEnv(id))
	}

	ch, unsubscribe, head, err := s.Subscribe(0, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	if head != 5 {
		t.Fatalf("catch-up head = %d, want 5", head)
	}
	// The head is a PROMISE that those envelopes are already queued. Check the
	// promise against the queue, not just its value.
	var highest int64
	for i := 0; i < 5; i++ {
		if env := recv(t, ch); env.Sequence > highest {
			highest = env.Sequence
		}
	}
	if highest != head {
		t.Fatalf("preload reached %d but the announced head was %d — promise and queue disagree", highest, head)
	}
}

func TestSubscribeHeadIsTheLogHeadNotTheCursor(t *testing.T) {
	s := openTemp(t)
	for _, id := range []string{"e1", "e2", "e3", "e4"} {
		s.Append(newEnv(id))
	}
	_, unsubscribe, head, err := s.Subscribe(2, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if head != 4 {
		t.Fatalf("catch-up head = %d, want 4", head)
	}
}

func TestSubscribeHeadOnAnEmptyLogIsZero(t *testing.T) {
	// Zero tells a client there is nothing to wait for. Sequence 0 is never a
	// real event, so it is unambiguous.
	s := openTemp(t)
	_, unsubscribe, head, err := s.Subscribe(0, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if head != 0 {
		t.Fatalf("catch-up head = %d, want 0 on an empty log", head)
	}
}

func TestSubscribeHeadExcludesEventsAppendedAfterwards(t *testing.T) {
	// A point-in-time promise about the PRELOAD. Counting a later append would
	// make a client wait for an event that was never part of catch-up — which
	// on a quiet table means waiting until the backstop timeout.
	s := openTemp(t)
	s.Append(newEnv("e1"))

	_, unsubscribe, head, err := s.Subscribe(0, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	later := newEnv("e2")
	s.Append(later)
	s.Notify(later)

	if head != 1 {
		t.Fatalf("catch-up head = %d, want 1 — the head at subscribe time", head)
	}
}
