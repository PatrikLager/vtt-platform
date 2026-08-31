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
func TestCampaignOffersNoWayToUnmakeHistory(t *testing.T) {
	typ := reflect.TypeOf(&campaign.Campaign{})
	for i := range typ.NumMethod() {
		name := typ.Method(i).Name
		lower := strings.ToLower(name)
		if strings.Contains(lower, "undo") || strings.Contains(lower, "retract") {
			t.Errorf("Campaign.%s exists; the log only goes forward, so nothing on Campaign may unmake an event", name)
		}
	}
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
