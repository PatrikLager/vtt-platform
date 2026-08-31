package campaign_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestAppendBatchAcceptsAnAdventureLoadBatch pins that an adventure-format
// load batch goes through campaign.AppendBatch as one atomic run and folds:
// every event kind adventure.Compile emits, in its own binding order
// (AdventureLoaded, SceneCreated, ActorAdded, TokenPlaced, NoteUpserted,
// NarrationAdded — see internal/adventure/compile.go).
//
// internal/campaign may not import internal/adventure (arch-lint: campaign's
// mayDependOn lists only [contract, store, engine, campaign] — adventure is a
// content-format layer ABOVE campaign, not a campaign dependency), so the
// batch is built directly with cenv, matching Compile's own envelope shapes by
// hand rather than calling adventure.Compile — content lifted from the real,
// committed adventures/goblin-ambush fixture (scene/actor/note/narration text)
// so this test reads as "the real adventure's batch shape", not an arbitrary
// one.
//
// THIS IS THE SURVIVING HALF OF TestUndoRetractsAdventureLoadBatch, which
// appended this batch and then retracted its whole range (adventure-format
// Task 4, spec §7: "the adventure batch retracts as a range"). Retraction left
// the platform on 2026-08-31 and the retracting half went with it. The
// appending half is kept deliberately rather than deleted along with it: it is
// the only place in this package where an AdventureLoaded envelope is folded
// at all, and deleting a test deletes coverage nothing else names.
func TestAppendBatchAcceptsAnAdventureLoadBatch(t *testing.T) {
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
	if _, err := c.AppendBatch(envs); err != nil {
		t.Fatalf("AppendBatch(adventure batch): %v", err)
	}

	loaded := c.State()
	if statesEqual(before, loaded) {
		t.Fatal("state after the adventure batch == pre-batch state — the batch did not land, this test proves nothing")
	}
	if _, ok := loaded.Scenes["ravine"]; !ok {
		t.Fatal(`want scene "ravine" present after the adventure batch`)
	}
	if _, ok := loaded.Actors["act-fighter"]; !ok {
		t.Fatal(`want actor "act-fighter" present after the adventure batch`)
	}
	if _, ok := loaded.Tokens["tok-fighter"]; !ok {
		t.Fatal(`want token "tok-fighter" present after the adventure batch`)
	}
	if _, ok := loaded.Notes["ravine-trail-warning"]; !ok {
		t.Fatal(`want note "ravine-trail-warning" present after the adventure batch`)
	}
}
