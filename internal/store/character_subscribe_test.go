package store_test

import (
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func entry(actorID, id string) store.Entry {
	return store.Entry{ActorID: actorID, Cause: 0, Env: entryEnv(id)}
}

func persist(t *testing.T, s *store.Store, world string, entries ...store.Entry) *vttv1.Envelope {
	t.Helper()
	w := newEnv(world)
	if _, err := s.AppendBatch([]*vttv1.Envelope{w}, entries); err != nil {
		t.Fatalf("AppendBatch(%s): %v", world, err)
	}
	return w
}

func notifyEntries(s *store.Store, entries ...store.Entry) {
	for _, e := range entries {
		s.NotifyCharacter(e.ActorID, e.Env)
	}
}

func subscribeCharacter(t *testing.T, s *store.Store, actorID string, after int64, buffer int) (<-chan *vttv1.Envelope, int64) {
	t.Helper()
	ch, unsubscribe, head, err := s.SubscribeCharacter(actorID, after, buffer, 0)
	if err != nil {
		t.Fatalf("SubscribeCharacter(%q, %d): %v", actorID, after, err)
	}
	t.Cleanup(unsubscribe)
	return ch, head
}

func recvEntry(t *testing.T, ch <-chan *vttv1.Envelope, id string, seq int64) {
	t.Helper()
	got := recv(t, ch)
	if got.EventId != id || got.Sequence != seq {
		t.Fatalf("got %s at %d, want %s at %d", got.EventId, got.Sequence, id, seq)
	}
}

func assertQuiet(t *testing.T, name string, ch <-chan *vttv1.Envelope) {
	t.Helper()
	select {
	case got, ok := <-ch:
		if !ok {
			t.Fatalf("%s: channel closed, want it open and silent", name)
		}
		t.Fatalf("%s: got %s at %d, want nothing more", name, got.EventId, got.Sequence)
	case <-time.After(200 * time.Millisecond):
	}
}

func assertClosed(t *testing.T, name string, ch <-chan *vttv1.Envelope) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatalf("%s: channel still open", name)
		}
	}
}

// VTT-329 VTT-331
func TestACharacterSubscriptionDeliversItsLogFromTheCursorThenLive(t *testing.T) {
	s := openTemp(t)
	persist(t, s, "w1", entry("a", "a1"), entry("b", "b1"))
	persist(t, s, "w2", entry("a", "a2"), entry("b", "b2"))
	persist(t, s, "w3", entry("a", "a3"), entry("b", "b3"))

	ch, head := subscribeCharacter(t, s, "a", 1, 8)
	if head != 3 {
		t.Fatalf("catch-up head = %d, want 3", head)
	}
	recvEntry(t, ch, "a2", 2)
	recvEntry(t, ch, "a3", 3)

	live := []store.Entry{entry("a", "a4"), entry("b", "b4")}
	persist(t, s, "w4", live...)
	notifyEntries(s, live...)
	recvEntry(t, ch, "a4", 4)
	assertQuiet(t, "a", ch)
}

// VTT-330
func TestACharacterSubscriptionCarriesNoOtherLog(t *testing.T) {
	s := openTemp(t)
	a, _ := subscribeCharacter(t, s, "a", 0, 16)
	b, _ := subscribeCharacter(t, s, "b", 0, 16)
	world, unsubscribe, _, err := s.Subscribe(0, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	// Keep world event i and both entries i at sequence i, and notify b first in
	// the second batch: a leak either way then lands ahead of its receiver.
	for _, i := range []string{"1", "2", "3"} {
		entries := []store.Entry{entry("a", "a"+i), entry("b", "b"+i)}
		w := persist(t, s, "w"+i, entries...)
		s.Notify(w)
		if i == "2" {
			entries[0], entries[1] = entries[1], entries[0]
		}
		notifyEntries(s, entries...)
	}

	for i, id := range []string{"1", "2", "3"} {
		seq := int64(i + 1)
		recvEntry(t, a, "a"+id, seq)
		recvEntry(t, b, "b"+id, seq)
		recvEntry(t, world, "w"+id, seq)
	}
	assertQuiet(t, "a", a)
	assertQuiet(t, "b", b)
	assertQuiet(t, "world", world)
}

// VTT-329
func TestACharacterSubscriptionRacingAnAppendSeesEachEntryOnce(t *testing.T) {
	t.Run("subscribed between the write and its notify", func(t *testing.T) {
		s := openTemp(t)
		raced := entry("a", "a1")
		persist(t, s, "w1", raced)

		ch, head := subscribeCharacter(t, s, "a", 0, 8)
		if head != 1 {
			t.Fatalf("catch-up head = %d, want 1", head)
		}
		notifyEntries(s, raced)
		next := entry("a", "a2")
		persist(t, s, "w2", next)
		notifyEntries(s, next)

		recvEntry(t, ch, "a1", 1)
		recvEntry(t, ch, "a2", 2)
		assertQuiet(t, "a", ch)
	})

	t.Run("subscribed while appends run", func(t *testing.T) {
		s := openTemp(t)
		const total = 200
		halfway := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := range total {
				if i == total/2 {
					close(halfway)
				}
				e := entry("a", envID(i))
				w := newEnv("w" + envID(i))
				if _, err := s.AppendBatch([]*vttv1.Envelope{w}, []store.Entry{e}); err != nil {
					t.Errorf("AppendBatch %d: %v", i, err)
					return
				}
				notifyEntries(s, e)
			}
		}()

		<-halfway
		ch, _ := subscribeCharacter(t, s, "a", 0, 0)
		for i := range total {
			recvEntry(t, ch, envID(i), int64(i+1))
		}
		<-done
		assertQuiet(t, "a", ch)
	})
}

// VTT-331 VTT-329
func TestACharacterSubscriptionWaitsForALogThatHasNoEntries(t *testing.T) {
	t.Run("a log with no entries", func(t *testing.T) {
		s := openTemp(t)
		persist(t, s, "w1", entry("b", "b1"), entry("b", "b2"))

		ch, head := subscribeCharacter(t, s, "a", 0, 8)
		if head != 0 {
			t.Fatalf("catch-up head = %d, want 0", head)
		}
		assertQuiet(t, "a before its first entry", ch)

		first := entry("a", "a1")
		persist(t, s, "w2", first)
		notifyEntries(s, first)
		recvEntry(t, ch, "a1", 1)
	})

	t.Run("a cursor at the log's last entry", func(t *testing.T) {
		s := openTemp(t)
		persist(t, s, "w1", entry("a", "a1"), entry("a", "a2"))

		ch, head := subscribeCharacter(t, s, "a", 2, 8)
		if head != 2 {
			t.Fatalf("catch-up head = %d, want 2", head)
		}
		assertQuiet(t, "a at its head", ch)

		third := entry("a", "a3")
		persist(t, s, "w2", third)
		notifyEntries(s, third)
		recvEntry(t, ch, "a3", 3)
	})
}

// VTT-336 VTT-337
func TestAStalledCharacterSubscriberIsDroppedAfterTheNoProgressTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := openTemp(t)
		wedged, _ := subscribeCharacter(t, s, "a", 0, 1)
		healthy, _ := subscribeCharacter(t, s, "a", 0, 16)

		for i := range 4 {
			e := entry("a", envID(i))
			persist(t, s, "w"+envID(i), e)
			notifyEntries(s, e)
		}
		for i := range 4 {
			recvEntry(t, healthy, envID(i), int64(i+1))
		}

		time.Sleep(store.SubscriberNoProgressTimeout + time.Second)
		synctest.Wait()

		closed := false
		for range 16 {
			select {
			case _, ok := <-wedged:
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
			t.Fatal("the subscription that never read must be closed once its no-progress budget elapses")
		}

		after := entry("a", "after")
		persist(t, s, "w-after", after)
		notifyEntries(s, after)
		recvEntry(t, healthy, "after", 5)
	})
}

// VTT-335
func TestClosingTheStoreEndsCharacterSubscriptions(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "campaign.db"))
	if err != nil {
		t.Fatal(err)
	}
	persist(t, s, "w1", entry("a", "a1"))
	withEntries, unsubscribeA, _, err := s.SubscribeCharacter("a", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribeA()
	empty, unsubscribeZ, _, err := s.SubscribeCharacter("z", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribeZ()

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertClosed(t, "a", withEntries)
	assertClosed(t, "z", empty)
}

func TestNotifyCharacterIgnoresZeroSequence(t *testing.T) {
	s := openTemp(t)
	// At -1, so the subscriber's own dedupe cannot hide the guard.
	ch, _ := subscribeCharacter(t, s, "a", -1, 8)

	s.NotifyCharacter("a", entryEnv("zero-seq"))
	assertQuiet(t, "a after a zero-sequence notify", ch)

	stamped := entry("a", "a1")
	persist(t, s, "w1", stamped)
	notifyEntries(s, stamped)
	recvEntry(t, ch, "a1", 1)
}
