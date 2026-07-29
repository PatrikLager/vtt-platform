package store_test

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func recv(t *testing.T, ch <-chan *vttv1.Envelope) *vttv1.Envelope {
	t.Helper()
	select {
	case env, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		return env
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
		return nil
	}
}

func TestSubscribeCatchUpThenLive(t *testing.T) {
	s := openTemp(t)
	s.Append(newEnv("e1"))
	s.Append(newEnv("e2"))

	ch, cancel, err := s.Subscribe(1, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if got := recv(t, ch); got.EventId != "e2" {
		t.Fatalf("catch-up: got %s, want e2", got.EventId)
	}
	e3 := newEnv("e3")
	s.Append(e3)
	s.Notify(e3)
	if got := recv(t, ch); got.EventId != "e3" {
		t.Fatalf("live: got %s, want e3", got.EventId)
	}
}

func TestSubscribeOverflowClosesThatSubscriberOnly(t *testing.T) {
	s := openTemp(t)
	small, cancelSmall, _ := s.Subscribe(0, 1)
	defer cancelSmall()
	big, cancelBig, _ := s.Subscribe(0, 16)
	defer cancelBig()

	for i := 0; i < 4; i++ {
		env := newEnv(string(rune('a' + i)))
		s.Append(env)
		s.Notify(env)
	}
	// small (cap 1, never drained) must end CLOSED; drain to find closure.
	deadline := time.After(2 * time.Second)
	closed := false
	for !closed {
		select {
		case _, ok := <-small:
			if !ok {
				closed = true
			}
		case <-deadline:
			t.Fatal("small subscriber never closed on overflow")
		}
	}
	// big subscriber unaffected: sees all 4.
	for i := 0; i < 4; i++ {
		recv(t, big)
	}
}

// TestSubscribeAcceptsZeroBuffer pins the boundary of Subscribe's guard:
// only NEGATIVE buffers are invalid. buffer=0 means "no extra live-event
// slack beyond the catch-up batch" and must be accepted — every other test
// in this file happens to pass buffer>=1, leaving the buffer==0 boundary
// itself unexercised.
func TestSubscribeAcceptsZeroBuffer(t *testing.T) {
	s := openTemp(t)
	s.Append(newEnv("e1"))
	s.Append(newEnv("e2"))

	ch, cancel, err := s.Subscribe(0, 0)
	if err != nil {
		t.Fatalf("want buffer=0 accepted (only negative buffers are invalid), got error: %v", err)
	}
	defer cancel()

	if got := recv(t, ch); got.EventId != "e1" {
		t.Fatalf("catch-up: got %s, want e1", got.EventId)
	}
	if got := recv(t, ch); got.EventId != "e2" {
		t.Fatalf("catch-up: got %s, want e2", got.EventId)
	}
}

func TestSubscribeRejectsNegativeBuffer(t *testing.T) {
	s := openTemp(t)
	if _, _, err := s.Subscribe(0, -1); err == nil {
		t.Fatal("want error for negative subscribe buffer")
	}
}

// TestAppendDoesNotNotify covers the decoupling itself: Append persists but
// no longer notifies. A subscriber established before Append must see
// nothing until the caller explicitly calls Notify.
func TestAppendDoesNotNotify(t *testing.T) {
	s := openTemp(t)
	ch, cancel, err := s.Subscribe(0, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	env := newEnv("e1")
	if _, err := s.Append(env); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		t.Fatalf("want no delivery from Append alone, got %v", got)
	case <-time.After(200 * time.Millisecond):
	}

	s.Notify(env)
	if got := recv(t, ch); got.EventId != "e1" {
		t.Fatalf("after explicit Notify: got %s, want e1", got.EventId)
	}
}

// TestNotifyAfterApplyOrdering_NoDuplicateOnRace covers the subscribe-
// between-persist-and-notify race that per-subscriber sequence dedupe
// closes: a subscriber that catches up on an event via Subscribe's history
// preload must not receive that same event again when the caller's
// subsequent explicit Notify call for it lands.
func TestNotifyAfterApplyOrdering_NoDuplicateOnRace(t *testing.T) {
	s := openTemp(t)
	env := newEnv("e1")
	if _, err := s.Append(env); err != nil {
		t.Fatal(err)
	}

	// Subscribe catches up to seq 1 (the appended-but-not-yet-notified
	// event) via history preload.
	ch, cancel, err := s.Subscribe(0, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if got := recv(t, ch); got.EventId != "e1" {
		t.Fatalf("catch-up: got %s, want e1", got.EventId)
	}

	// The caller now runs the deferred Notify for the same envelope (the
	// race: subscribe landed between persist and notify). Dedupe must skip
	// it since the subscriber's lastSeq already covers it.
	s.Notify(env)

	select {
	case got := <-ch:
		t.Fatalf("want no duplicate delivery on raced Notify, got %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestNotifyIgnoresZeroSequence covers defensive hardening on the public
// Notify method: an envelope whose Sequence is the proto3 zero value (0) —
// never a value Append assigns, since Append's next-sequence query is
// COALESCE(MAX(seq),0)+1, minimum 1 — is silently dropped rather than fanned
// out, protecting against a caller that invokes Notify without ever having
// persisted the event via Append.
//
// The subscriber below is caught up from afterSeq=-1, not the realistic
// afterSeq=0: with afterSeq=0 the pre-existing per-subscriber dedupe
// (notifyLocked's env.Sequence <= sub.lastSeq check) already happens to
// swallow a zero-sequence envelope on its own (0 <= 0), which would mask
// whether Notify's own guard fired at all. afterSeq=-1 gives the subscriber
// lastSeq=-1, so 0 <= -1 is false and the dedupe does NOT intercept it —
// isolating the guard as the only thing that can still stop delivery.
//
// This test's teeth therefore depend on Subscribe accepting a NEGATIVE
// afterSeq unclamped (see Subscribe's doc comment in subscribe.go) — it is
// deliberately load-bearing for this isolation, not incidental: if Subscribe
// ever added a floor/clamp on afterSeq (e.g. treating negative values as 0),
// this test would silently fall back to the masked afterSeq=0 case above and
// stop proving anything about the guard.
func TestNotifyIgnoresZeroSequence(t *testing.T) {
	s := openTemp(t)
	ch, cancel, err := s.Subscribe(-1, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	s.Notify(&vttv1.Envelope{
		EventId:   "zero-seq",
		SessionId: "sess-1",
		ActorRole: "dm",
		Sequence:  0,
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "zero"},
		},
	})
	select {
	case got := <-ch:
		t.Fatalf("want zero-sequence envelope silently dropped, got delivered: %v", got)
	case <-time.After(200 * time.Millisecond):
	}

	// A normal, correctly-stamped envelope still delivers afterward — the
	// guard must not disable the subscriber or the store.
	live := newEnv("e-real")
	if _, err := s.Append(live); err != nil {
		t.Fatal(err)
	}
	s.Notify(live)
	if got := recv(t, ch); got.EventId != "e-real" {
		t.Fatalf("live delivery after guard: got %s, want e-real", got.EventId)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	s := openTemp(t)
	ch, cancel, _ := s.Subscribe(0, 4)
	cancel()
	after := newEnv("after")
	s.Append(after)
	s.Notify(after)
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received event after unsubscribe")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("channel not closed after unsubscribe")
	}
}
