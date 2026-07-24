package campaign_test

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// statesEqual compares two engine.State snapshots. Actors are proto messages
// (unexported protoimpl bookkeeping fields make them unsafe to compare with
// reflect.DeepEqual across a fresh unmarshal), so they're compared with
// proto.Equal; Scenes, Tokens, and Sessions are plain structs and compared
// with ordinary equality.
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
// scene, actors, tokens, moves, a mid-log undo, session end — followed by a
// close/reopen and a full-state comparison against the pre-close snapshot.
// This is the seed for the future simulation harness (sub-project 4).
func TestExitScenario(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	ch, cancel, err := c.Subscribe(0, 32)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer cancel()

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

	// Three TokenMoved events: the first two move t1 (t1's second move is the
	// one that gets retracted below), the third moves t2. Because the third
	// move targets a different token, retracting the middle move leaves t1's
	// final position exactly where it was right before the retracted move —
	// the clean "reverts to the pre-retracted-move position" check — while
	// t2's independent move remains intact, proving the retraction didn't
	// affect events outside its range.
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 2, Y: 2},
		To:   &vttv1.GridPosition{X: 3, Y: 3},
	}))
	middleMoveSeq := must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 3},
		To:   &vttv1.GridPosition{X: 4, Y: 4},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t2", SceneId: "scn",
		From: &vttv1.GridPosition{X: 8, Y: 8},
		To:   &vttv1.GridPosition{X: 9, Y: 9},
	}))

	markerID := nextID()
	if _, err := c.Undo(middleMoveSeq, middleMoveSeq, "mistaken move", markerID, "dm", "test-participant"); err != nil {
		t.Fatalf("undo middle move: %v", err)
	}

	st := c.State()
	tok1, ok := st.Tokens["t1"]
	if !ok {
		t.Fatal("want token t1 present after undo")
	}
	if tok1.X != 3 || tok1.Y != 3 {
		t.Fatalf("t1 position after undo: got (%d,%d), want (3,3) (the pre-retracted-move position)", tok1.X, tok1.Y)
	}
	tok2, ok := st.Tokens["t2"]
	if !ok {
		t.Fatal("want token t2 present after undo")
	}
	if tok2.X != 9 || tok2.Y != 9 {
		t.Fatalf("t2 position after undo: got (%d,%d), want (9,9) (unaffected by the retraction)", tok2.X, tok2.Y)
	}

	// The subscriber, caught up since before any of this happened, must
	// receive the EventsRetracted marker itself as a live event.
	foundMarker := false
drain:
	for {
		select {
		case env, ok := <-ch:
			if !ok {
				break drain
			}
			if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker && env.EventId == markerID {
				foundMarker = true
				break drain
			}
		case <-time.After(2 * time.Second):
			break drain
		}
	}
	if !foundMarker {
		t.Fatal("subscriber did not receive the EventsRetracted marker")
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
