package campaign_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// c.head is unexported, so these tests observe it through the anchor rule:
// every append validates a clone stamped c.head+1, so an anchor exactly at the
// boundary flips between accepted and refused when c.head is off by one.

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

// TestRejectedAnchorAfterBatchDoesNotPoisonTheCampaign: a c.head a batch left
// too high lets a refused anchor pass validation and poison the campaign after
// it persists, so assert the campaign is still usable after the refusal.
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
