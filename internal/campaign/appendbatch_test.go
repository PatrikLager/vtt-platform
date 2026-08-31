package campaign_test

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// TestAppendBatchAssignsContiguousSequencesAndAppliesAll covers the happy
// path (N>=2): all events persist with contiguous sequences and the live
// projection reflects every one of them.
func TestAppendBatchAssignsContiguousSequencesAndAppliesAll(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"})) // occupies seq 1

	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10}),
		cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"}}),
		cenv(nextID(), &vttv1.TokenPlaced{
			TokenId: "t1", SceneId: "scn", ActorId: "a1",
			Position: &vttv1.GridPosition{X: 3, Y: 7},
		}),
	}
	first, err := c.AppendBatch(envs)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	if first != 2 {
		t.Fatalf("firstSeq: got %d, want 2", first)
	}
	for i, env := range envs {
		want := int64(2 + i)
		if env.Sequence != want {
			t.Fatalf("envs[%d].Sequence: got %d, want %d", i, env.Sequence, want)
		}
	}

	st := c.State()
	if _, ok := st.Scenes["scn"]; !ok {
		t.Fatal("want scene scn present after AppendBatch")
	}
	if _, ok := st.Actors["a1"]; !ok {
		t.Fatal("want actor a1 present after AppendBatch")
	}
	tok, ok := st.Tokens["t1"]
	if !ok || tok.X != 3 || tok.Y != 7 {
		t.Fatalf("want token t1 at (3,7) after AppendBatch, got %+v (present=%v)", tok, ok)
	}
}

// TestAppendBatchRejectsEmptyBatch covers the empty-batch posture at the
// campaign layer.
func TestAppendBatchRejectsEmptyBatch(t *testing.T) {
	c := openTemp(t)
	if _, err := c.AppendBatch(nil); err == nil {
		t.Fatal("want error for empty (nil) batch")
	}
	if _, err := c.AppendBatch([]*vttv1.Envelope{}); err == nil {
		t.Fatal("want error for empty (zero-length) batch")
	}
}

// TestAppendBatchMidBatchValidationFailurePersistsNothing is the core
// atomicity guarantee: the second event of a three-event batch is invalid
// (TokenMoved for an unknown token). Nothing from the batch may persist —
// verified by reopening the raw store on the same file — and the campaign
// must remain fully usable afterward (not poisoned): a following Append
// succeeds normally.
func TestAppendBatchMidBatchValidationFailurePersistsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10}))

	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"}}),
		cenv(nextID(), &vttv1.TokenMoved{ // invalid: no token "missing-token" exists
			TokenId: "missing-token", SceneId: "scn",
			From: &vttv1.GridPosition{X: 0, Y: 0},
			To:   &vttv1.GridPosition{X: 1, Y: 1},
		}),
		cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a2", Name: "Sidekick", ModuleId: "m"}}),
	}
	if _, err := c.AppendBatch(envs); err == nil {
		t.Fatal("want error for a batch whose 2nd event is invalid")
	}

	st := c.State()
	if _, ok := st.Actors["a1"]; ok {
		t.Fatal("want actor a1 NOT present: even the valid event before the failing one must not persist")
	}
	if _, ok := st.Actors["a2"]; ok {
		t.Fatal("want actor a2 NOT present")
	}

	// Campaign fully usable after the rejected batch: a normal Append works.
	if _, err := c.Append(cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a3", Name: "Survivor", ModuleId: "m"},
	})); err != nil {
		t.Fatalf("Append after rejected AppendBatch: %v", err)
	}
	if _, ok := c.State().Actors["a3"]; !ok {
		t.Fatal("want actor a3 present: campaign must still accept new events")
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	events, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	// SessionStarted, SceneCreated, then the successful post-rejection
	// ActorAdded(a3) = 3 events. NOTHING from the 3-event rejected batch.
	if len(events) != 3 {
		t.Fatalf("ReadAfter(0) after rejected AppendBatch: got %d events, want 3 (rejected batch must persist nothing)", len(events))
	}
}

// TestAppendBatchSubscriberSeesContiguousRunUnderConcurrentAppend proves
// the "no interleaved foreign events" property directly: a batch and
// several single-event Appends race on the SAME campaign, and a subscriber
// caught up beforehand must see the batch's 3 events delivered back to back
// (no foreign event's delivery lands between them), with contiguous
// sequence numbers. This holds because campaign.AppendBatch and
// campaign.Append share ONE mutex (c.mu) covering the whole
// validate-persist-live-apply-notify sequence for each call — no other
// Campaign operation can run concurrently with it, let alone interleave a
// notification into the middle of it.
func TestAppendBatchSubscriberSeesContiguousRunUnderConcurrentAppend(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10}))
	seedSeq := must(t, c, cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"}}))

	// Subscribe from the head (seedSeq), not 0: subscribing from 0 would
	// preload the 3 seed events into the channel via catch-up history (see
	// Subscribe's doc comment), and the drain loop below only accounts for
	// the batch+foreign live events — an undrained history prefix would
	// silently eat into the live-event budget whenever the batch loses its
	// race against the foreign Appends, making this test flaky (found by
	// review: ~60% failure without -race, ~7% with -race, "want 3 batch
	// deliveries, got 0"). The undo tests subscribed from the head for the
	// same reason until they left with retraction on 2026-08-31, so this is
	// now the only test in the package that starts a subscription mid-log —
	// the reason is written out here rather than cited to a sibling.
	ch, unsubscribe, _, err := c.Subscribe(seedSeq, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	const foreignCount = 12
	batchIDs := map[string]bool{}
	batchEnvs := make([]*vttv1.Envelope, 3)
	for i := range batchEnvs {
		id := nextID()
		batchIDs[id] = true
		batchEnvs[i] = cenv(id, &vttv1.TokenPlaced{
			TokenId: "batch-tok-" + id, SceneId: "scn", ActorId: "a1",
			Position: &vttv1.GridPosition{X: int32(i), Y: int32(i)},
		})
	}

	// Pre-build every foreign envelope (and its id) BEFORE spawning
	// goroutines: nextID()'s shared counter is not itself meant to be
	// called concurrently (it's a test helper, not part of the code under
	// test), so any concurrent id generation belongs outside the race
	// window this test is actually trying to observe.
	foreignEnvs := make([]*vttv1.Envelope, foreignCount)
	for i := range foreignEnvs {
		id := nextID()
		foreignEnvs[i] = cenv(id, &vttv1.TokenPlaced{
			TokenId: "foreign-tok-" + id, SceneId: "scn", ActorId: "a1",
			Position: &vttv1.GridPosition{X: int32(i), Y: int32(i)},
		})
	}

	var wg sync.WaitGroup
	wg.Add(1 + foreignCount)
	go func() {
		defer wg.Done()
		if _, err := c.AppendBatch(batchEnvs); err != nil {
			t.Errorf("AppendBatch: %v", err)
		}
	}()
	for i := 0; i < foreignCount; i++ {
		go func(env *vttv1.Envelope) {
			defer wg.Done()
			if _, err := c.Append(env); err != nil {
				t.Errorf("Append: %v", err)
			}
		}(foreignEnvs[i])
	}
	wg.Wait()

	// Drain every delivered event (3 + foreignCount), recording delivery
	// order and which ones belong to the batch.
	total := 3 + foreignCount
	var order []*vttv1.Envelope
	deadline := time.After(2 * time.Second)
	for len(order) < total {
		select {
		case env := <-ch:
			order = append(order, env)
		case <-deadline:
			t.Fatalf("timeout: only received %d/%d events", len(order), total)
		}
	}

	// Find the batch's 3 delivery positions.
	var batchPositions []int
	for i, env := range order {
		if batchIDs[env.EventId] {
			batchPositions = append(batchPositions, i)
		}
	}
	if len(batchPositions) != 3 {
		t.Fatalf("want 3 batch deliveries, got %d", len(batchPositions))
	}
	if batchPositions[1] != batchPositions[0]+1 || batchPositions[2] != batchPositions[1]+1 {
		t.Fatalf("batch deliveries not contiguous in the notification stream: positions %v (a foreign event was interleaved)", batchPositions)
	}

	// Sequences of the batch's own envelopes must also be contiguous.
	seqs := []int64{batchEnvs[0].Sequence, batchEnvs[1].Sequence, batchEnvs[2].Sequence}
	if seqs[1] != seqs[0]+1 || seqs[2] != seqs[1]+1 {
		t.Fatalf("batch sequences not contiguous: %v", seqs)
	}
}
