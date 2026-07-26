package campaign_test

// append_sequence_validation_test.go pins the fix for the single-Append
// twin of the F1/5c AppendBatch bug (appendbatch_session_test.go's
// TestAppendBatchDoubleSessionEndedRejectsCleanlyWithoutBricking's own doc
// comment describes the general shape): Campaign.Append validated the
// caller's envelope directly while its Sequence field was still 0 (the
// store assigns the real value strictly AFTER that validating call — see
// store.Store.Append's own "envelope sequence must be 0 on append" guard).
// Any fold that is sequence-DEPENDENT for a REJECTION decision (not just a
// stored VALUE) validated against the wrong number.
//
// world-layer Task 3's investigation (see .superpowers/sdd/
// p11-task-3-report.md's "Critical finding" section) found exactly this
// for NarrationAdded's anchor-sanity check (internal/engine/apply.go):
// `anchor_to_seq >= env.Sequence` compared against 0 at validation time,
// so ANY anchored narration (anchor_to_seq > 0) was rejected regardless of
// whether the anchor was otherwise perfectly valid. Controller-adjudicated
// as the same bug class as F1/5c, fixed here the same way: Append now
// validates a proto.Clone stamped with the provisional sequence c.head+1
// — provably identical to what store.Append is about to assign, by the
// SAME equivalence AppendBatch's own doc comment already proves (c.head
// tracks store's MAX(seq) under the same c.mu every log-appending path
// holds for its whole call).

import (
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// TestAppendAcceptsAnchoredNarrationPointingAtRealPriorSequence is the
// headline RED: an anchor pointing at TWO ALREADY-RECORDED, real prior
// sequences (from <= to, both > 0, both strictly before the narration's
// own about-to-be-assigned sequence) must be ACCEPTED. Pre-fix, this was
// rejected unconditionally (env.Sequence == 0 at validation time makes
// `anchor_to_seq >= 0` true for any positive anchor).
func TestAppendAcceptsAnchoredNarrationPointingAtRealPriorSequence(t *testing.T) {
	c := openTemp(t)

	seq1 := must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "s1", Name: "Cave", GridWidth: 5, GridHeight: 5}))
	seq2 := must(t, c, cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero"}}))

	seq3, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{
		Text: "the hero enters the cave", AnchorFromSeq: seq1, AnchorToSeq: seq2,
	}))
	if err != nil {
		t.Fatalf("anchored narration pointing at real prior sequences (%d,%d) must be accepted, got: %v", seq1, seq2, err)
	}
	if seq3 != seq2+1 {
		t.Fatalf("narration sequence = %d, want %d (contiguous after seq2)", seq3, seq2+1)
	}
}

// --- equivalence pin (AppendBatch test-file style): the provisional
// sequence validation uses must EXACTLY equal what the store is about to
// assign, pinned at the tightest possible boundary in both directions — a
// wrong provisional value (still 0, or off by one) would flip exactly one
// of these two outcomes. -----------------------------------------------

// TestAppendAnchoredNarrationAtHeadBoundaryAccepted anchors at
// anchor_to_seq == head (the immediately preceding event's own sequence —
// the tightest valid boundary: one less than the narration's own
// about-to-be-assigned sequence). Only accepted if the provisional
// sequence used for validation is EXACTLY head+1 (not 0, not head, not
// head+2).
func TestAppendAnchoredNarrationAtHeadBoundaryAccepted(t *testing.T) {
	c := openTemp(t)
	head := must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "s1", Name: "Cave", GridWidth: 5, GridHeight: 5}))

	seq, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{
		Text: "narrating the immediately preceding event", AnchorFromSeq: head, AnchorToSeq: head,
	}))
	if err != nil {
		t.Fatalf("anchor_to_seq == head (%d) must be accepted, got: %v", head, err)
	}
	if seq != head+1 {
		t.Fatalf("narration sequence = %d, want %d", seq, head+1)
	}
}

// TestAppendAnchoredNarrationAtOwnFutureSequenceRejected anchors at
// anchor_to_seq == head+1 — the narration event's OWN about-to-be-assigned
// sequence, which the spec forbids (anchors point strictly backward, never
// at or beyond the narrating event's own sequence). This must STILL be
// rejected after the fix: proof the fix corrected the provisional value
// used for validation without disabling the check itself (a naive
// "always accept" regression would pass the boundary test above too, but
// would wrongly accept this one).
func TestAppendAnchoredNarrationAtOwnFutureSequenceRejected(t *testing.T) {
	c := openTemp(t)
	head := must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "s1", Name: "Cave", GridWidth: 5, GridHeight: 5}))

	if _, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{
		Text: "anchoring at my own future sequence", AnchorFromSeq: head + 1, AnchorToSeq: head + 1,
	})); err == nil {
		t.Fatalf("anchor_to_seq == head+1 (%d, the narration's own future sequence) must still be rejected", head+1)
	}
}

// TestAppendAnchoredNarrationReopenReplaysCleanly proves an accepted
// anchored narration is not just accepted live but replays cleanly from
// the log on reopen — the SAME snapshot-then-persist-then-live-apply
// sequencing that made the pre-fix bug so easy to miss (a validation-time
// rejection never reaches the log at all; this proves acceptance produces
// a log entry that folds back to the identical state).
func TestAppendAnchoredNarrationReopenReplaysCleanly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	seq1 := must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "s1", Name: "Cave", GridWidth: 5, GridHeight: 5}))
	seq2 := must(t, c, cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero"}}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{
		Text: "the hero enters the cave", AnchorFromSeq: seq1, AnchorToSeq: seq2,
	}))

	before := c.State()
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	c2, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("reopen: log did not replay cleanly after an accepted anchored narration: %v", err)
	}
	t.Cleanup(func() { c2.Close() })

	after := c2.State()
	if !statesEqual(before, after) {
		t.Fatalf("state mismatch after close/reopen with an anchored narration in the log\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// --- the OTHER sequence-dependent single-Append fold: SessionEnded -----

// TestSessionEndedEndSeqUnaffectedByValidationSequence proves the OTHER
// sequence-dependent fold (SessionEnded writes EndSeq = env.Sequence, and
// EndSeq==0 is the "session still open" sentinel — the SAME field
// AppendBatch's own pre-fix bug corrupted across MULTIPLE batched events)
// was never actually observably broken for a SINGLE Append, unlike
// NarrationAdded's anchor check: SessionEnded's fold only WRITES using
// env.Sequence, it never REJECTS based on it (internal/engine/apply.go's
// SessionEnded case checks only "is there an open session", never the
// Sequence value) — so the validation pass's stale Sequence==0 was written
// into a THROWAWAY clone/snapshot that gets discarded either way; the
// LIVE-state write always happened on Append's second (post-persist)
// engine.Apply call, which always saw the correctly store-assigned
// Sequence, both before and after this fix. This test pins that the
// live-recorded EndSeq is correct — proving no divergence exists to fix
// here, exactly as expected once the validation-pass fix (which changes
// only the THROWAWAY clone's Sequence, never the live env's) is in place.
func TestSessionEndedEndSeqUnaffectedByValidationSequence(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "s"}))
	endSeq := must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))

	st := c.State()
	if len(st.Sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(st.Sessions))
	}
	if st.Sessions[0].EndSeq != endSeq {
		t.Fatalf("Sessions[0].EndSeq = %d, want %d (the SessionEnded event's own real, store-assigned sequence)", st.Sessions[0].EndSeq, endSeq)
	}
}
