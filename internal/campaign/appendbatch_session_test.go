package campaign_test

import (
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// TestAppendBatchDoubleSessionEndedRejectsCleanlyWithoutBricking pins the F1
// data-integrity fix: a batch carrying two SessionEnded events must be
// rejected by snapshot validation BEFORE anything persists.
//
// The pre-fix bug: validation folded envelopes while their Sequence was still
// 0 (sequences are stamped later by the store), and SessionEnded writes
// EndSeq = env.Sequence, where EndSeq==0 is the "session still open"
// sentinel. So during validation the first SessionEnded left the session
// looking open and the second passed too — the batch committed, then
// live-apply (with real sequences) failed on the second SessionEnded,
// poisoning the campaign AND leaving an on-disk log that never replays
// ("corrupt log ... no open session to end") — the documented recovery
// (reopen) becomes impossible.
func TestAppendBatchDoubleSessionEndedRejectsCleanlyWithoutBricking(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "s"})) // one open session

	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.SessionEnded{}),
		cenv(nextID(), &vttv1.SessionEnded{}),
	}
	if _, err := c.AppendBatch(envs); err == nil {
		t.Fatal("want a clean rejection of a double-SessionEnded batch, got nil error")
	} else if strings.Contains(err.Error(), "diverged") {
		t.Fatalf("batch was rejected only AFTER persisting (live-apply divergence, campaign poisoned): %v", err)
	}

	// Validation rejected before persist, so the campaign is NOT poisoned.
	if st := c.State(); st == nil {
		t.Fatal("campaign poisoned by a batch that should have been rejected before persisting")
	}

	// The campaign is still fully usable and the on-disk log still replays.
	if _, err := c.Append(cenv(nextID(), &vttv1.SessionEnded{})); err != nil {
		t.Fatalf("a single SessionEnded after the rejected batch should succeed: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	c2, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("reopen after the rejected batch: log must still replay, got: %v", err)
	}
	c2.Close()
}

// TestAppendBatchSessionStartEndStartAccepted pins the mirror direction: a
// [SessionStarted, SessionEnded, SessionStarted] batch is legal (the same
// events succeed appended one at a time) and must be ACCEPTED — the pre-fix
// Sequence==0 validation spuriously rejected it with "session already open".
func TestAppendBatchSessionStartEndStartAccepted(t *testing.T) {
	c := openTemp(t)
	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.SessionStarted{Name: "s1"}),
		cenv(nextID(), &vttv1.SessionEnded{}),
		cenv(nextID(), &vttv1.SessionStarted{Name: "s2"}),
	}
	if _, err := c.AppendBatch(envs); err != nil {
		t.Fatalf("[SessionStarted, SessionEnded, SessionStarted] must be accepted: %v", err)
	}
	st := c.State()
	if len(st.Sessions) != 2 {
		t.Fatalf("want 2 sessions after the batch, got %d", len(st.Sessions))
	}
	open := 0
	for _, s := range st.Sessions {
		if s.EndSeq == 0 {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("want exactly 1 open session after [start,end,start], got %d", open)
	}
}

// TestAppendBatchSessionLifecycleMatchesSequentialAppends is the equivalence
// pin the fix rests on: because AppendBatch holds c.mu for the whole call and
// every append routes through campaign methods under that lock, the store's
// MAX(seq)+1 assignment is stable, so folding the validation clones with
// provisional sequences head+1..head+N produces EXACTLY what sequential
// Append calls produce. If the two ever diverged, this test would fail.
func TestAppendBatchSessionLifecycleMatchesSequentialAppends(t *testing.T) {
	seqC := openTemp(t)
	batchC := openTemp(t)

	mk := func() []*vttv1.Envelope {
		return []*vttv1.Envelope{
			cenv(nextID(), &vttv1.SessionStarted{Name: "s1"}),
			cenv(nextID(), &vttv1.SessionEnded{}),
			cenv(nextID(), &vttv1.SessionStarted{Name: "s2"}),
		}
	}
	for _, e := range mk() {
		if _, err := seqC.Append(e); err != nil {
			t.Fatalf("sequential append: %v", err)
		}
	}
	if _, err := batchC.AppendBatch(mk()); err != nil {
		t.Fatalf("batch append: %v", err)
	}

	sa, sb := seqC.State().Sessions, batchC.State().Sessions
	if len(sa) != len(sb) {
		t.Fatalf("session count: sequential=%d batch=%d", len(sa), len(sb))
	}
	// Session ids are fresh random values per SessionStarted, so compare the
	// sequence structure (StartSeq/EndSeq), which is what equivalence pins.
	for i := range sa {
		if sa[i].StartSeq != sb[i].StartSeq || sa[i].EndSeq != sb[i].EndSeq {
			t.Fatalf("session[%d] structure differs: sequential {start:%d end:%d} batch {start:%d end:%d}",
				i, sa[i].StartSeq, sa[i].EndSeq, sb[i].StartSeq, sb[i].EndSeq)
		}
	}
}
