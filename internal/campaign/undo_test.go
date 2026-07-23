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

	if err := c.Undo(moveSeq, moveSeq, "mistake", nextID(), "sess-1"); err != nil {
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

	if err := c.Undo(moveSeq, moveSeq, "first undo", nextID(), "sess-1"); err != nil {
		t.Fatal(err)
	}
	markerSeq := moveSeq + 1 // sequence of the EventsRetracted marker just appended

	// A fresh, non-retracted event after the marker.
	nextSeq := must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 7},
		To:   &vttv1.GridPosition{X: 9, Y: 9},
	}))

	if err := c.Undo(markerSeq, nextSeq, "nested", nextID(), "sess-1"); err == nil {
		t.Fatal("want error retracting a range that contains a retraction marker (no nesting)")
	}
}

// TestUndoRejectsAlreadyRetractedRange covers double-retraction: retracting
// a range that overlaps an already-retracted span is rejected.
func TestUndoRejectsAlreadyRetractedRange(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	if err := c.Undo(moveSeq, moveSeq, "first undo", nextID(), "sess-1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Undo(moveSeq, moveSeq, "double undo", nextID(), "sess-1"); err == nil {
		t.Fatal("want error retracting an already-retracted range")
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
			if err := c.Undo(tc.from, tc.to, "oob", nextID(), "sess-1"); err == nil {
				t.Fatalf("want error for out-of-range retraction [%d,%d]", tc.from, tc.to)
			}
		})
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
	if err := c.Undo(moveSeq, moveSeq, "mistake", markerID, "sess-1"); err != nil {
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
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscriber to receive retraction marker")
	}
}
