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
	s.Append(newEnv("e3"))
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
		s.Append(newEnv(string(rune('a' + i))))
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

func TestUnsubscribeStopsDelivery(t *testing.T) {
	s := openTemp(t)
	ch, cancel, _ := s.Subscribe(0, 4)
	cancel()
	s.Append(newEnv("after"))
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received event after unsubscribe")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("channel not closed after unsubscribe")
	}
}
