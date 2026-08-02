package campaign_test

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// seedMovedToken opens a fresh campaign and appends SessionStarted,
// SceneCreated, ActorAdded, TokenPlaced, TokenMoved (in that order),
// returning the campaign and the sequence of the TokenMoved event.
func seedMovedToken(t *testing.T) (*campaign.Campaign, int64) {
	t.Helper()
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	}))
	must(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 3, Y: 7},
	}))
	moveSeq := must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 7},
		To:   &vttv1.GridPosition{X: 5, Y: 8},
	}))
	return c, moveSeq
}

// TestUndoRetractsMoveTokenBackToPriorPosition covers position-reversion:
// retracting the TokenMoved event returns the token to its position before
// the move, derived by rebuilding from the log with that event excluded.
func TestUndoRetractsMoveTokenBackToPriorPosition(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	if _, err := c.Undo(moveSeq, moveSeq, "mistake", nextID(), "dm", "test-participant"); err != nil {
		t.Fatal(err)
	}

	st := c.State()
	tok, ok := st.Tokens["t1"]
	if !ok {
		t.Fatal("want token t1 present after undo")
	}
	if tok.X != 3 || tok.Y != 7 {
		t.Fatalf("token position after undo: got (%d,%d), want (3,7)", tok.X, tok.Y)
	}
}

// TestUndoRejectsRangeContainingRetractionMarker covers no-nesting: a
// retraction range that contains a prior EventsRetracted marker is rejected,
// even when the rest of the range is ordinary, non-retracted events.
func TestUndoRejectsRangeContainingRetractionMarker(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	if _, err := c.Undo(moveSeq, moveSeq, "first undo", nextID(), "dm", "test-participant"); err != nil {
		t.Fatal(err)
	}
	markerSeq := moveSeq + 1 // sequence of the EventsRetracted marker just appended

	// A fresh, non-retracted event after the marker.
	nextSeq := must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 7},
		To:   &vttv1.GridPosition{X: 9, Y: 9},
	}))

	if _, err := c.Undo(markerSeq, nextSeq, "nested", nextID(), "dm", "test-participant"); err == nil {
		t.Fatal("want error retracting a range that contains a retraction marker (no nesting)")
	}
}

// TestUndoRejectsAlreadyRetractedRange covers double-retraction: retracting
// a range that overlaps an already-retracted span is rejected.
func TestUndoRejectsAlreadyRetractedRange(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	if _, err := c.Undo(moveSeq, moveSeq, "first undo", nextID(), "dm", "test-participant"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Undo(moveSeq, moveSeq, "double undo", nextID(), "dm", "test-participant"); err == nil {
		t.Fatal("want error retracting an already-retracted range")
	}
}

// TestUndoRejectsPartialOverlapRetraction covers PARTIAL overlap, distinct
// from TestUndoRejectsAlreadyRetractedRange's exact-range repeat: retracting
// [moveSeq,moveSeq] then attempting a WIDER range [moveSeq-1,moveSeq+1] —
// which only partially overlaps the already-retracted span, with in-bounds
// sequences on both sides that were never retracted — must still be
// rejected. This exercises Undo's per-sequence already[env.Sequence] check
// (campaign.go) across every sequence in the requested range, not just an
// identical repeat of a prior range.
func TestUndoRejectsPartialOverlapRetraction(t *testing.T) {
	c, moveSeq := seedMovedToken(t)
	// A second move gives a sequence after moveSeq, so the wider range
	// [moveSeq-1, moveSeq+1] is in-bounds (moveSeq+1 must exist in the log).
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 5, Y: 8},
		To:   &vttv1.GridPosition{X: 6, Y: 9},
	}))

	if _, err := c.Undo(moveSeq, moveSeq, "first undo", nextID(), "dm", "test-participant"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Undo(moveSeq-1, moveSeq+1, "partial overlap", nextID(), "dm", "test-participant"); err == nil {
		t.Fatal("want error retracting a range that partially overlaps an already-retracted sequence")
	}
}

// TestUndoRejectsOutOfRangeSequence covers out-of-range: the range must be
// well-formed (from >= 1, to >= from) and not extend past the log head.
func TestUndoRejectsOutOfRangeSequence(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	tests := []struct {
		name     string
		from, to int64
	}{
		{"to beyond log head", moveSeq, moveSeq + 100},
		{"from less than 1", 0, moveSeq},
		{"to less than from", moveSeq, moveSeq - 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.Undo(tc.from, tc.to, "oob", nextID(), "dm", "test-participant"); err == nil {
				t.Fatalf("want error for out-of-range retraction [%d,%d]", tc.from, tc.to)
			}
		})
	}
}

// TestUndoAcceptsFromSequenceOne pins the boundary of Undo's range guard:
// from==1 (retracting the very first event in the log) is VALID — only
// from<1 is rejected. Every other Undo test in this file retracts moveSeq
// (5), leaving the from==1 boundary itself unexercised. Retracting
// SessionStarted alone replays cleanly: no other seeded event depends on an
// open session existing.
func TestUndoAcceptsFromSequenceOne(t *testing.T) {
	c, _ := seedMovedToken(t)

	if _, err := c.Undo(1, 1, "retract session start", nextID(), "dm", "test-participant"); err != nil {
		t.Fatalf("want from=1 accepted (only from<1 is invalid), got error: %v", err)
	}
}

// TestUndoOnEmptyLogRejectsGracefully pins the empty-log edge of the
// out-of-range check: with zero events ever appended, maxSeq must default to
// 0 and any requested range (from>=1) is beyond the log head — Undo returns
// that error rather than indexing the empty events slice. Every other Undo
// test seeds at least one event first, leaving len(events)==0 unexercised.
func TestUndoOnEmptyLogRejectsGracefully(t *testing.T) {
	c := openTemp(t)

	if _, err := c.Undo(1, 1, "nothing to undo", nextID(), "dm", "test-participant"); err == nil {
		t.Fatal("want error retracting a range on an empty log")
	}
}

// TestUndoRetractsWholeAppendBatchRange proves the existing Undo range
// machinery retracts a batch's contiguous sequence run WITHOUT any
// modification: append a 3-event batch, capture the pre-batch state, undo
// the batch's whole [firstSeq, firstSeq+2] range in one call, and assert
// the resulting state equals the pre-batch snapshot and the marker's own
// FromSequence/ToSequence cover exactly the batch's range.
func TestUndoRetractsWholeAppendBatchRange(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	}))

	before := c.State()

	// The batch spans the rules event family too (ResourceChanged +
	// ConditionApplied), so undoing the whole range and rebuilding must
	// restore Resources AND Conditions to their pre-batch state — pinned by
	// statesEqual, which now compares Conditions (F9).
	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "a1", Name: "Hero", ModuleId: "m",
			Resources: map[string]*vttv1.Resource{"vigor": {Current: 3, Max: 10}},
		}}),
		cenv(nextID(), &vttv1.TokenPlaced{
			TokenId: "t1", SceneId: "scn", ActorId: "a1",
			Position: &vttv1.GridPosition{X: 3, Y: 7},
		}),
		cenv(nextID(), &vttv1.ResourceChanged{
			ActorId: "a1", Resource: "vigor", Delta: 2, NewValue: 5, Reason: "ability:x:hit",
		}),
		cenv(nextID(), &vttv1.ConditionApplied{
			ActorId: "a1", ConditionId: "marked", Source: "threshold:vigor",
		}),
	}
	firstSeq, err := c.AppendBatch(envs)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	lastSeq := envs[len(envs)-1].Sequence

	ch, cancel, _, err := c.Subscribe(lastSeq, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	markerID := nextID()
	if _, err := c.Undo(firstSeq, lastSeq, "retract whole batch", markerID, "dm", "test-participant"); err != nil {
		t.Fatalf("Undo(batch range): %v", err)
	}

	after := c.State()
	if !statesEqual(before, after) {
		t.Fatalf("state after undoing the whole batch != pre-batch state\nbefore: %+v\nafter:  %+v", before, after)
	}

	select {
	case got := <-ch:
		if got.EventId != markerID {
			t.Fatalf("marker event id: got %s, want %s", got.EventId, markerID)
		}
		marker, ok := got.Payload.(*vttv1.Envelope_EventsRetracted)
		if !ok {
			t.Fatalf("want EventsRetracted payload, got %T", got.Payload)
		}
		if marker.EventsRetracted.FromSequence != firstSeq || marker.EventsRetracted.ToSequence != lastSeq {
			t.Fatalf("marker range: got [%d,%d], want [%d,%d] (exactly the batch's range)",
				marker.EventsRetracted.FromSequence, marker.EventsRetracted.ToSequence, firstSeq, lastSeq)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscriber to receive retraction marker")
	}
}

// TestUndoRetractsAdventureLoadBatch proves the existing Undo range
// machinery retracts an adventure-format load batch (adventure-format Task
// 4, spec §7: "the adventure batch retracts as a range (existing
// machinery) — tested") exactly like any other AppendBatch batch —
// TestUndoRetractsWholeAppendBatchRange above already proves the GENERIC
// machinery against a rules-shaped batch (ActorAdded/TokenPlaced/
// ResourceChanged/ConditionApplied); this test proves it for the
// adventure-shaped one specifically: every event kind adventure.Compile
// emits, in its own binding order (AdventureLoaded, SceneCreated,
// ActorAdded, TokenPlaced, NoteUpserted, NarrationAdded — see
// internal/adventure/compile.go). internal/campaign may not import
// internal/adventure (arch-lint: campaign's own mayDependOn lists only
// [contract, store, engine, campaign] — adventure is a content-format layer
// ABOVE campaign, not a campaign dependency), so the batch is built
// directly with cenv, matching Compile's own envelope shapes by hand rather
// than calling adventure.Compile — content lifted from the real, committed
// adventures/goblin-ambush fixture (scene/actor/note/narration text) so
// this test reads as "the real adventure's batch shape", not an arbitrary
// one.
func TestUndoRetractsAdventureLoadBatch(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))

	before := c.State()

	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.AdventureLoaded{AdventureId: "goblin-ambush", Name: "Goblin Ambush"}),
		cenv(nextID(), &vttv1.SceneCreated{
			SceneId: "ravine", Name: "The Ravine Trail", GridWidth: 32, GridHeight: 32,
		}),
		cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "act-fighter", Name: "Human Fighter",
			Attributes: map[string]int32{"str": 4, "dex": 2, "con": 3},
			Resources:  map[string]*vttv1.Resource{"vigor": {Current: 28, Max: 28}},
		}}),
		cenv(nextID(), &vttv1.TokenPlaced{
			TokenId: "tok-fighter", SceneId: "ravine", ActorId: "act-fighter",
			Position: &vttv1.GridPosition{X: 0, Y: 0},
		}),
		cenv(nextID(), &vttv1.NoteUpserted{
			Key: "ravine-trail-warning", Title: "Trail Warning",
			Text: "Crude markings are scratched into the rock near the ravine's mouth.",
		}),
		cenv(nextID(), &vttv1.NarrationAdded{Text: "The trail narrows into a ravine."}),
	}
	firstSeq, err := c.AppendBatch(envs)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	lastSeq := envs[len(envs)-1].Sequence

	// Sanity: the batch actually landed before undoing it — otherwise this
	// test would trivially pass for the wrong reason.
	loaded := c.State()
	if statesEqual(before, loaded) {
		t.Fatal("state after the adventure batch == pre-batch state — the batch did not land, this test proves nothing")
	}
	if _, ok := loaded.Scenes["ravine"]; !ok {
		t.Fatal(`want scene "ravine" present after the adventure batch`)
	}
	if _, ok := loaded.Notes["ravine-trail-warning"]; !ok {
		t.Fatal(`want note "ravine-trail-warning" present after the adventure batch`)
	}

	if _, err := c.Undo(firstSeq, lastSeq, "retract adventure load", nextID(), "dm", "test-participant"); err != nil {
		t.Fatalf("Undo(adventure batch range): %v", err)
	}

	undone := c.State()
	if !statesEqual(before, undone) {
		t.Fatalf("state after undoing the adventure batch != pre-batch state\nbefore: %+v\nafter:  %+v", before, undone)
	}
}

// TestUndoSubscriberReceivesRetractionMarker covers
// subscriber-sees-the-marker: a subscriber caught up to the pre-undo log
// receives the EventsRetracted envelope itself as a live event.
func TestUndoSubscriberReceivesRetractionMarker(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	ch, cancel, _, err := c.Subscribe(moveSeq, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	markerID := nextID()
	if _, err := c.Undo(moveSeq, moveSeq, "mistake", markerID, "dm", "test-participant"); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		if got.EventId != markerID {
			t.Fatalf("subscriber event id: got %s, want %s", got.EventId, markerID)
		}
		if _, ok := got.Payload.(*vttv1.Envelope_EventsRetracted); !ok {
			t.Fatalf("want EventsRetracted payload, got %T", got.Payload)
		}
		if got.OccurredAt == nil {
			t.Fatal("want non-nil OccurredAt on retraction marker")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscriber to receive retraction marker")
	}
}
