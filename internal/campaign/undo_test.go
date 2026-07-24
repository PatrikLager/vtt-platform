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

// TestUndoSubscriberReceivesRetractionMarker covers
// subscriber-sees-the-marker: a subscriber caught up to the pre-undo log
// receives the EventsRetracted envelope itself as a live event.
func TestUndoSubscriberReceivesRetractionMarker(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	ch, cancel, err := c.Subscribe(moveSeq, 4)
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
