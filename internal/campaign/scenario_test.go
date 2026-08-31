package campaign_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// statesEqual compares two engine.State snapshots. Actors are proto messages
// (unexported protoimpl bookkeeping fields make them unsafe to compare with
// reflect.DeepEqual across a fresh unmarshal), so they're compared with
// proto.Equal; Scenes, Tokens, Sessions, Conditions, and Notes are plain
// structs and compared with ordinary equality.
func statesEqual(a, b *engine.State) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !reflect.DeepEqual(a.Scenes, b.Scenes) {
		return false
	}
	if !reflect.DeepEqual(a.Tokens, b.Tokens) {
		return false
	}
	if !reflect.DeepEqual(a.Sessions, b.Sessions) {
		return false
	}
	if !reflect.DeepEqual(a.Conditions, b.Conditions) {
		return false
	}
	if !reflect.DeepEqual(a.Notes, b.Notes) {
		return false
	}
	if len(a.Actors) != len(b.Actors) {
		return false
	}
	for id, av := range a.Actors {
		bv, ok := b.Actors[id]
		if !ok || !proto.Equal(av, bv) {
			return false
		}
	}
	return true
}

// TestExitScenario is the spec §9 exit scenario: a full session lifecycle —
// scene, actors, tokens, moves, session end — followed by a close/reopen and
// a full-state comparison against the pre-close snapshot. This is the seed for
// the future simulation harness (sub-project 4).
//
// IT HELD A MID-LOG UNDO until 2026-08-31, and a subscription that existed
// only to watch the EventsRetracted marker arrive live. Both left with
// retraction (spec 2026-08-30-retraction-leaves). Live delivery to a
// subscriber is pinned by TestSubscriberSeesAppendedEvents, so nothing but
// the marker went with the subscription.
func TestExitScenario(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "exit-scenario"}))

	must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 20, GridHeight: 20,
	}))

	must(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	}))
	must(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a2", Name: "Sidekick", ModuleId: "m"},
	}))

	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 2, Y: 2},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: "t2", SceneId: "scn", ActorId: "a2",
		Position: &vttv1.GridPosition{X: 8, Y: 8},
	}))

	// Three TokenMoved events: the first two move t1 in succession, the third
	// moves t2. The pair on t1 is what makes the close/reopen comparison below
	// say something — a fold that kept the FIRST position of a token rather
	// than its last would still round-trip a single move — and t2's
	// independent move keeps the check from being about one token alone.
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 2, Y: 2},
		To:   &vttv1.GridPosition{X: 3, Y: 3},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 3},
		To:   &vttv1.GridPosition{X: 4, Y: 4},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t2", SceneId: "scn",
		From: &vttv1.GridPosition{X: 8, Y: 8},
		To:   &vttv1.GridPosition{X: 9, Y: 9},
	}))

	st := c.State()
	tok1, ok := st.Tokens["t1"]
	if !ok {
		t.Fatal("want token t1 present")
	}
	if tok1.X != 4 || tok1.Y != 4 {
		t.Fatalf("t1 position: got (%d,%d), want (4,4) (the last move it was given)", tok1.X, tok1.Y)
	}
	tok2, ok := st.Tokens["t2"]
	if !ok {
		t.Fatal("want token t2 present")
	}
	if tok2.X != 9 || tok2.Y != 9 {
		t.Fatalf("t2 position: got (%d,%d), want (9,9)", tok2.X, tok2.Y)
	}

	must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))

	before := c.State()
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	c2, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { c2.Close() })

	after := c2.State()
	if !statesEqual(before, after) {
		t.Fatalf("state mismatch after close/reopen\nbefore: %+v\nafter:  %+v", before, after)
	}
}
