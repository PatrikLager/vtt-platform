package campaign_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

func watch(t *testing.T, c *campaign.Campaign) <-chan struct{} {
	t.Helper()
	wake, stop, err := c.WatchAppends()
	if err != nil {
		t.Fatalf("WatchAppends: %v", err)
	}
	t.Cleanup(stop)
	return wake
}

// Fail instead of hanging: a blocking wake holds the campaign's lock for good,
// so nothing may take that campaign's lock afterwards.
func appendWithin(t *testing.T, c *campaign.Campaign, envs ...*vttv1.Envelope) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := c.AppendBatch(envs)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an append did not return; a wake blocked it")
	}
}

// Read without waiting: an append raises its wake before it returns.
func woken(wake <-chan struct{}) bool {
	select {
	case _, ok := <-wake:
		return ok
	default:
		return false
	}
}

// VTT-353
func TestEveryAppendWakesEveryWatcher(t *testing.T) {
	c, err := campaign.Open(filepath.Join(t.TempDir(), "campaign"))
	if err != nil {
		t.Fatal(err)
	}
	// Register no stop on Cleanup: it takes the lock a blocked wake holds.
	a, _, errA := c.WatchAppends()
	b, _, errB := c.WatchAppends()
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if woken(a) || woken(b) {
		t.Fatal("a watch woke before any append")
	}

	appendWithin(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "one"}))
	if !woken(a) || !woken(b) {
		t.Fatal("an append did not wake both watches")
	}

	for i := range 3 {
		appendWithin(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: fmt.Sprintf("burst %d", i)}))
	}
	if !woken(a) {
		t.Fatal("a burst of three appends left no waiting signal")
	}
	if woken(a) {
		t.Fatal("a burst of three appends left more than one waiting signal")
	}

	woken(b)
	appendWithin(t, c,
		cenv(nextID(), &vttv1.NarrationAdded{Text: "batch one"}),
		cenv(nextID(), &vttv1.NarrationAdded{Text: "batch two"}))
	if !woken(b) {
		t.Fatal("a batch append did not wake the watch")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

// VTT-354
func TestARefusedOrFailedAppendWakesNobody(t *testing.T) {
	t.Run("refused", func(t *testing.T) {
		c := openTemp(t)
		wake := watch(t, c)
		if _, err := c.Append(&vttv1.Envelope{EventId: nextID()}); err == nil {
			t.Fatal("an envelope with no payload was accepted")
		}
		if _, err := c.Append(cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-none", SceneId: "s", To: at(1, 1)})); err == nil {
			t.Fatal("a move of an unknown token was accepted")
		}
		if woken(wake) {
			t.Fatal("a refused append woke the watch")
		}
	})

	t.Run("failed", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "campaign")
		c := openAt(t, dir)
		wake := watch(t, c)
		raw := rawLog(t, dir)
		execSQL(t, raw, `CREATE TRIGGER refuse_events BEFORE INSERT ON events
			BEGIN SELECT RAISE(ABORT, 'refused by the test'); END`)
		if _, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{Text: "lost"})); err == nil ||
			!strings.Contains(err.Error(), "refused by the test") {
			t.Fatalf("the append = %v, want the trigger's refusal", err)
		}
		if woken(wake) {
			t.Fatal("a failed append woke the watch")
		}

		execSQL(t, raw, `DROP TRIGGER refuse_events`)
		must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "kept"}))
		if !woken(wake) {
			t.Fatal("the watch did not wake once the append succeeded; the failed case proves nothing")
		}
	})

	t.Run("poisoning", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "campaign")
		c := openAt(t, dir)
		head := must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the last event"}))
		wake := watch(t, c)
		// Keep the DELETE: the store then numbers the next event head, its anchor
		// reaches its own sequence, and the live apply diverges.
		execSQL(t, rawLog(t, dir), fmt.Sprintf(`DELETE FROM events WHERE seq = %d`, head))
		anchored := cenv(nextID(), &vttv1.NarrationAdded{Text: "the poison", AnchorFromSeq: 1, AnchorToSeq: head})
		if _, err := c.Append(anchored); err == nil {
			t.Fatal("fixture: the diverging append succeeded")
		}
		if c.State() != nil {
			t.Fatal("fixture: the diverging append did not poison the campaign")
		}
		if woken(wake) {
			t.Fatal("a poisoning append woke the watch")
		}
	})
}

func TestStoppingAWatcherEndsOnlyThat(t *testing.T) {
	c := openTemp(t)
	a, stopA, err := c.WatchAppends()
	if err != nil {
		t.Fatal(err)
	}
	b := watch(t, c)

	stopA()
	stopA()
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "after the stop"}))
	if woken(a) {
		t.Fatal("a stopped watch was woken")
	}
	if !woken(b) {
		t.Fatal("stopping one watch silenced another")
	}
}

// VTT-356 VTT-373
func TestClosingTheCampaignEndsEveryWatcher(t *testing.T) {
	c, err := campaign.Open(filepath.Join(t.TempDir(), "campaign"))
	if err != nil {
		t.Fatal(err)
	}
	a, stopA, err := c.WatchAppends()
	if err != nil {
		t.Fatal(err)
	}
	b, stopB, err := c.WatchAppends()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	for name, wake := range map[string]<-chan struct{}{"a": a, "b": b} {
		select {
		case _, ok := <-wake:
			if ok {
				t.Fatalf("watch %s: received a signal after Close, want the channel closed", name)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("watch %s: still open after Close", name)
		}
	}
	stopA()
	stopB()
	if _, _, err := c.WatchAppends(); err == nil {
		t.Fatal("a closed campaign accepted a watch")
	}
}

func TestActorsIsACopyOfTheLiveActors(t *testing.T) {
	c := openTemp(t)
	hallWithHero(t, c, 5)

	got := c.Actors()
	want := c.State().Actors
	if len(got) != len(want) {
		t.Fatalf("Actors holds %d actors, the state %d", len(got), len(want))
	}
	for id, a := range want {
		if !proto.Equal(got[id], a) {
			t.Fatalf("Actors[%q] = %v, the state holds %v", id, got[id], a)
		}
	}

	got["hero"].Kind = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	delete(got, "goblin")
	after := c.State().Actors
	if after["hero"].GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Fatal("editing an actor Actors returned changed the live actor")
	}
	if _, ok := after["goblin"]; !ok {
		t.Fatal("deleting from the map Actors returned removed the live actor")
	}
}
