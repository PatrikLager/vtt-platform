package campaign_test

import (
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// TestUndoRejectsRetractionThatBreaksReplay is the canonical repro for the
// bricking bug: SessionStarted(1), SessionEnded(2), SessionStarted(3), then
// Undo(2,2). Retracting the SessionEnded leaves SessionStarted(3) replaying
// against an already-open session, which rebuildLocked cannot fold. Undo
// must detect this BEFORE persisting the marker: the error must mention the
// replay failure, the marker must not be written to the log (verified by
// reopening the raw store after Close and counting events), and the
// campaign must still open cleanly afterward with its pre-Undo state intact.
func TestUndoRejectsRetractionThatBreaksReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	seq1 := must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "first"}))
	seq2 := must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))
	seq3 := must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "second"}))

	if seq1 != 1 || seq2 != 2 || seq3 != 3 {
		t.Fatalf("want sequences 1,2,3, got %d,%d,%d", seq1, seq2, seq3)
	}

	err = c.Undo(seq2, seq2, "would corrupt replay", nextID(), "sess-1", "dm", "test-participant")
	if err == nil {
		t.Fatal("want error retracting a SessionEnded that a later SessionStarted depends on")
	}
	if !strings.Contains(err.Error(), "replay") && !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("want error mentioning replay/corrupt viability, got %q", err.Error())
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	// The marker must NOT have been persisted: reopen the raw store and
	// confirm ReadAfter(0) still returns exactly the 3 events appended
	// before the rejected Undo call (mirrors
	// TestAppendValidationFailurePersistsNothing's style).
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	events, err := s.ReadAfter(0)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("ReadAfter(0) after rejected Undo: got %d events, want 3 (rejected Undo must persist nothing)", len(events))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// The campaign file must not be bricked: reopening via campaign.Open
	// must succeed and reflect the intact, pre-Undo state.
	c2, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("campaign.Open after rejected Undo: want success (file must not be bricked), got %v", err)
	}
	defer c2.Close()

	st := c2.State()
	if len(st.Sessions) != 2 {
		t.Fatalf("want 2 sessions in reopened state, got %d: %+v", len(st.Sessions), st.Sessions)
	}
	if st.Sessions[0].EndSeq != seq2 {
		t.Fatalf("want first session EndSeq %d, got %d", seq2, st.Sessions[0].EndSeq)
	}
	if st.Sessions[1].EndSeq != 0 {
		t.Fatalf("want second session still open (EndSeq 0), got %d", st.Sessions[1].EndSeq)
	}
}

// TestUndoAcceptsValidTokenMovedRetraction guards against over-tightening:
// a retraction that does NOT break replay (undoing a TokenMoved, which
// nothing downstream depends on) must still succeed end-to-end, including
// the resulting position reversion.
func TestUndoAcceptsValidTokenMovedRetraction(t *testing.T) {
	c, moveSeq := seedMovedToken(t)

	if err := c.Undo(moveSeq, moveSeq, "valid undo", nextID(), "sess-1", "dm", "test-participant"); err != nil {
		t.Fatalf("want valid TokenMoved retraction to succeed, got %v", err)
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
