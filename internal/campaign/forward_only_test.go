package campaign_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// TestCampaignOffersNoWayToUnmakeHistory pins the removal itself as an
// invariant rather than as the absence of one named method: no exported entry
// point on *Campaign undoes or retracts anything.
//
// STATED AS A PROPERTY OF THE WHOLE METHOD SET because a test naming Undo
// alone would go green the moment somebody added Retract, RollBack or
// UndoRange — and re-adding it under a new name is exactly the way a removed
// concept comes back (Patrik, 2026-08-30: a retraction's purpose is to make
// something not have happened, and it cannot do that).
//
// IT ASSERTS AN ABSENCE, SO IT NEEDS A POSITIVE CONTROL, and this is the same
// shape contract/events.test.ts's checkControl carries on the TypeScript side
// of this branch. Without one the test passes on an EMPTY CORPUS: changing
// reflect.TypeOf(&campaign.Campaign{}) to reflect.TypeOf(campaign.Campaign{})
// collapses the method set from six to zero — a value type has none of the
// pointer-receiver methods — and every assertion below still holds, vacuously,
// forever. Found in whole-branch review 2026-09-01; measured by making exactly
// that edit and watching the suite stay green.
//
// So the corpus is required to be real, and the matcher is required to match
// something it must catch. Neither is a claim about how many methods Campaign
// has: a floor, not a count, so adding or removing an ordinary method does not
// rot this (the "state the invariant, not the count" rule).
func TestCampaignOffersNoWayToUnmakeHistory(t *testing.T) {
	typ := reflect.TypeOf(&campaign.Campaign{})

	// THE CORPUS IS REAL. Append is named because it is the method this whole
	// invariant is about — the only way anything reaches the log — so a corpus
	// that has lost it is one that can no longer prove anything about going
	// forward, whatever its size.
	if n := typ.NumMethod(); n < 2 {
		t.Fatalf("the method set collapsed to %d: this test filters what it walks, so an empty corpus passes it vacuously", n)
	}
	if _, ok := typ.MethodByName("Append"); !ok {
		t.Fatal("Campaign.Append is not in the walked method set — the corpus is not the one this test means to walk")
	}

	// THE MATCHER CATCHES WHAT IT EXISTS TO CATCH. `unmakes` is applied to the
	// two names the removal was about, and to one it must leave alone, so a
	// predicate loosened to always-false (or tightened to always-true) fails
	// here rather than passing silently over a clean method set.
	for _, name := range []string{"Undo", "UndoRange", "RetractEvents", "retract"} {
		if !unmakes(name) {
			t.Errorf("the matcher does not catch %q, so it would not catch a real reintroduction", name)
		}
	}
	for _, name := range []string{"Append", "AppendBatch", "State", "Subscribe", "Close"} {
		if unmakes(name) {
			t.Errorf("the matcher catches %q, which is an ordinary method: it would red on a clean campaign", name)
		}
	}

	for i := range typ.NumMethod() {
		name := typ.Method(i).Name
		if unmakes(name) {
			t.Errorf("Campaign.%s exists; the log only goes forward, so nothing on Campaign may unmake an event", name)
		}
	}
}

// unmakes reports whether a method name claims to take an event back. Split
// out of the loop above so the positive control can drive the SAME predicate
// the walk uses — a control against a re-typed copy of the condition would
// pass while the walk's own condition was broken.
func unmakes(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "undo") || strings.Contains(lower, "retract")
}

// TestOpenRefusesALogThatDoesNotReplay is rebuildLocked's half of the same
// contract TestFoldPrefixRefusesALogThatDoesNotReplay pins for FoldPrefix: a
// campaign whose log cannot be folded does not open at all. It must not open
// with a partial projection — that would serve state which does not match the
// log, the exact divergence the poison contract exists to refuse (see the
// Campaign doc comment).
//
// THE LOG IS WRITTEN THROUGH store.Store, NOT THROUGH Append, because Append
// validates: it folds a snapshot clone first and rejects a move of a token
// nothing placed, so no campaign API can produce this file. A log that does not
// replay is what a corrupt or hand-edited file looks like, and Open is the only
// place it can be caught.
//
// AFTER-THE-FACT, each assertion proven by injection (ADR-009 §3), run and
// reverted 2026-08-31:
//
//   - rebuildLocked's `if err != nil { return err }` after foldEvents replaced
//     by a discard — Open then SUCCEEDS, and the test fails on "want
//     campaign.Open to refuse a log that does not replay";
//   - the corrupt-log message stripped of its sequence — fails on "the error
//     must name the sequence".
//
// It restores the reach TestUndoRejectsRetractionThatBreaksReplay had into this
// path. That test asserted the POSITIVE side too — a rejected retraction leaves
// the file openable — which needs no test now that no operation can leave a log
// unreplayable in the first place.
func TestOpenRefusesALogThatDoesNotReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")

	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range []*vttv1.Envelope{
		cenv(nextID(), &vttv1.SessionStarted{Name: "n"}),
		cenv(nextID(), &vttv1.TokenMoved{
			TokenId: "ghost", SceneId: "scn",
			From: &vttv1.GridPosition{X: 1, Y: 1},
			To:   &vttv1.GridPosition{X: 2, Y: 2},
		}),
	} {
		if _, err := s.Append(env); err != nil {
			t.Fatalf("seeding the raw store: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	c, err := campaign.Open(path)
	if err == nil {
		c.Close()
		t.Fatal("want campaign.Open to refuse a log that does not replay")
	}
	if !strings.Contains(err.Error(), "seq 2") {
		t.Errorf("the error must name the sequence that failed, got %q", err.Error())
	}
}
