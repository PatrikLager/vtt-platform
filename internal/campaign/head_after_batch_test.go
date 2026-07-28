package campaign_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// c.head after AppendBatch was completely unasserted. Mutating its arithmetic
// at campaign.go:357 — `c.head = firstSeq + int64(len(envs)) - 1` — survived
// three separate mutations (INVERT_NEGATIVES and two ARITHMETIC_BASE), which
// is to say: the batch could leave the campaign's sequence cursor at the wrong
// value and the entire suite stayed green.
//
// head is not directly observable — it is unexported with no accessor — but it
// is far from internal. Every subsequent append validates a CLONE stamped with
// `c.head + 1` (campaign.go:190, :348), and NarrationAdded's anchor rule is
// sequence-dependent: an anchor must point strictly BEFORE the narrating
// event's own sequence (engine/apply.go:194). So a head that is off by one
// shifts the provisional sequence, and an anchor exactly at the boundary flips
// from accepted to rejected.
//
// That is the observable these tests use, and it is the same mechanism as the
// P11 platform bug where single Append validated folds with Sequence=0 and
// made anchored narration impossible. Getting head wrong would also corrupt
// subscription catch-up and the headSequence staleness contract internal/mcp's
// read tools depend on — but those are farther from this package, and the
// anchor boundary pins it exactly.

// appendBatchOfN appends n scene-creation events as one batch and returns the
// first assigned sequence.
func appendBatchOfN(t *testing.T, c interface {
	AppendBatch([]*vttv1.Envelope) (int64, error)
}, n int) int64 {
	t.Helper()
	envs := make([]*vttv1.Envelope, 0, n)
	for i := 0; i < n; i++ {
		envs = append(envs, cenv(nextID(), &vttv1.SceneCreated{
			SceneId: nextID(), Name: "s", GridWidth: 2, GridHeight: 2,
		}))
	}
	first, err := c.AppendBatch(envs)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	return first
}

// TestHeadAfterAppendBatchAcceptsAnchorAtTheBoundary pins head from BELOW: a
// narration anchored to the batch's LAST sequence must be accepted, because
// that sequence is strictly before the narration's own. If head were left too
// low, the narration would be stamped with a sequence at or before its anchor
// and the fold would reject it.
func TestHeadAfterAppendBatchAcceptsAnchorAtTheBoundary(t *testing.T) {
	c := openTemp(t)

	const n = 3
	first := appendBatchOfN(t, c, n)
	last := first + n - 1

	seq, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{
		Text:          "describes the whole batch",
		AnchorFromSeq: first,
		AnchorToSeq:   last,
	}))
	if err != nil {
		t.Fatalf("narration anchored to the batch's last sequence (%d) must be accepted "+
			"— head is wrong after AppendBatch: %v", last, err)
	}
	if seq != last+1 {
		t.Errorf("next sequence = %d, want %d (one past the batch's last) — head did not "+
			"advance to the end of the batch", seq, last+1)
	}
}

// TestHeadAfterAppendBatchRejectsAnchorPastTheBoundary pins head from ABOVE:
// an anchor one past the batch's last sequence points at the narration's own
// sequence, which the fold forbids. If head were left too HIGH, this would be
// wrongly accepted.
//
// Both directions are needed. A single-sided assertion would leave half the
// arithmetic mutations alive — that is exactly the "one variant tested, its
// sibling assumed" shape this codebase keeps producing.
func TestHeadAfterAppendBatchRejectsAnchorPastTheBoundary(t *testing.T) {
	c := openTemp(t)

	const n = 3
	first := appendBatchOfN(t, c, n)
	last := first + n - 1

	if _, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{
		Text:          "anchored at its own sequence",
		AnchorFromSeq: first,
		AnchorToSeq:   last + 1,
	})); err == nil {
		t.Fatalf("narration anchored to %d (its own sequence) must be REJECTED — head is "+
			"too high after AppendBatch", last+1)
	}
}

// TestHeadAfterConsecutiveBatchesStaysContiguous pins that head tracks across
// more than one batch: the second batch must start exactly one past the first
// batch's last sequence. A head off by len(envs) shows up here even when the
// single-batch boundary happens to line up.
func TestHeadAfterConsecutiveBatchesStaysContiguous(t *testing.T) {
	c := openTemp(t)

	firstA := appendBatchOfN(t, c, 2)
	firstB := appendBatchOfN(t, c, 4)

	if want := firstA + 2; firstB != want {
		t.Errorf("second batch started at %d, want %d — head did not advance by exactly "+
			"the first batch's length", firstB, want)
	}

	seq, err := c.Append(cenv(nextID(), &vttv1.SessionStarted{Name: "after"}))
	if err != nil {
		t.Fatal(err)
	}
	if want := firstB + 4; seq != want {
		t.Errorf("next single append got %d, want %d", seq, want)
	}
}

// TestRejectedAnchorAfterBatchDoesNotPoisonTheCampaign is the test that
// actually kills the `- 1` in c.head's batch arithmetic, and it took finding
// the right observable to write.
//
// The obvious assertions do not work. head is corrected on the next single
// Append (campaign.go:198, `c.head = seq` from the store), and the sequence
// Append RETURNS comes from the store too — so a batch that leaves head wrong
// looks identical from the outside for one append, then heals.
//
// The damage happens inside that one window. Append validates a CLONE stamped
// `c.head + 1` (campaign.go:190) before persisting. If head is too high, an
// anchor that should be rejected passes validation, the event is PERSISTED,
// and only then does the real fold reject it — which by that function's own
// contract poisons the Campaign rather than serving a projection behind the
// log. So the difference between correct and mutated is not "does this
// append fail" (both fail) but "is the campaign still usable afterwards".
//
// That makes this a safety property worth pinning for its own sake: a
// rejection must be clean.
func TestRejectedAnchorAfterBatchDoesNotPoisonTheCampaign(t *testing.T) {
	c := openTemp(t)

	const n = 3
	first := appendBatchOfN(t, c, n)
	last := first + n - 1

	// Anchored to the sequence this narration will itself receive — invalid,
	// and with a correct head it is rejected during validation, before the
	// store is touched.
	if _, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{
		Text:          "anchored at its own sequence",
		AnchorFromSeq: first,
		AnchorToSeq:   last + 1,
	})); err == nil {
		t.Fatal("narration anchored at its own sequence must be rejected")
	}

	// The campaign must still work. If the rejection came from the fold AFTER
	// persisting — which is what a too-high head causes — the campaign is
	// poisoned and this fails.
	if _, err := c.Append(cenv(nextID(), &vttv1.SessionStarted{Name: "still alive"})); err != nil {
		t.Fatalf("campaign is unusable after a rejected anchor — the rejection happened "+
			"after persisting instead of during validation, which means head was wrong "+
			"after AppendBatch: %v", err)
	}
}
