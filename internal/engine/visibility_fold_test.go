package engine_test

import (
	"reflect"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func TestTokenHiddenForgetsOnlyThatToken(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))
	for _, id := range []string{"a1", "a2"} {
		must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: id, Name: id}})))
	}
	must(t, engine.Apply(st, env(4, &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "s", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 0, Y: 0}})))
	must(t, engine.Apply(st, env(5, &vttv1.TokenPlaced{
		TokenId: "t2", SceneId: "s", ActorId: "a2",
		Position: &vttv1.GridPosition{X: 1, Y: 1}})))

	must(t, engine.Apply(st, env(6, &vttv1.TokenHidden{TokenId: "t1"})))

	if _, still := st.Tokens["t1"]; still {
		t.Error("t1 was hidden and must be gone from the viewer's state")
	}
	if _, gone := st.Tokens["t2"]; !gone {
		t.Error("t2 was not hidden and must survive")
	}
}

func TestHidingATokenTwiceIsNotAnError(t *testing.T) {
	// The projection is idempotent by design; a repeated hide must not fail
	// the fold and take a player's client down.
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	if err := engine.Apply(st, env(2, &vttv1.TokenHidden{TokenId: "never-existed"})); err != nil {
		t.Fatalf("hiding an unknown token must be a no-op, got %v", err)
	}
}

func TestSceneSeenUnionsIntoExploredAndNeverShrinks(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))

	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}}})))
	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"1,1": {Kind: "wall"}}})))

	sc := st.Scenes["s"]
	if !sc.Explored["0,0"] {
		t.Error("0,0 was seen first and must still be explored — memory never shrinks")
	}
	if !sc.Explored["1,1"] {
		t.Error("1,1 was seen second and must be explored")
	}
	if sc.Tiles["1,1"].Kind != "wall" {
		t.Error("a seen tile must land in Tiles so it can be drawn")
	}
}

// TestSceneSeenForUnknownSceneIsRejectedAndLeavesStateUnchanged pins the
// STRICT half of SceneSeen's contract, which fix-round-1 found unpinned on
// the Go side: fold.ts's mirror of this arm already asserts the exact
// message (client/test/fold-rejections.test.ts, "scene seen for unknown
// scene..."), but nothing in this package exercised apply.go's own
// `if !ok { return ... }` branch at all — so a change that made Go's arm
// silently tolerant here, or drifted the message, would have passed every
// existing Go test while the TS test alone caught the divergence. Asserts
// the exact message (matching fold-rejections.test.ts's rigor, "engine: "
// namespace prefix aside — every other arm's Go message carries that same
// prefix TS never does, e.g. TokenPlaced's "engine: token placed on unknown
// scene..." vs fold.ts's "token placed on unknown scene...") and that state
// is byte-for-byte unchanged after the rejection, matching Apply's own
// "validates BEFORE mutating" doc comment promise (internal/engine/apply.go)
// and TestApplyRejections' established pattern for every other arm.
func TestSceneSeenForUnknownSceneIsRejectedAndLeavesStateUnchanged(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	before := st.Snapshot()

	err := engine.Apply(st, env(2, &vttv1.SceneSeen{SceneId: "nope",
		Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}}}))

	const want = `engine: scene seen for unknown scene "nope"`
	if err == nil || err.Error() != want {
		t.Fatalf("got error %v, want %q", err, want)
	}

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("state changed after a rejected SceneSeen\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// TestSceneSeenInitializesANilTilesMapWithoutPanicking exercises the OTHER
// branch fix-round-1 found at 0% coverage: the `if sc.Tiles == nil` guard
// (apply.go). Every Scene built through the SceneCreated arm already has a
// non-nil Tiles (even when empty — SceneCreated always calls `make`), so no
// test using the normal SceneCreated->SceneSeen path can ever reach this
// guard's true branch. It exists for the same reason DoorOpened/DoorClosed's
// nil-OpenDoors guards do (apply.go's own comments on those arms): a Scene
// can land in st.Scenes some other way — internal/rules/conformance's
// synthetic tooling assigns bare Scene{} literals directly, semgrep-exempted
// from the fold-only-writer rule — and writing into a nil map would panic
// instead of erroring, breaking Apply's "validates BEFORE mutating" promise
// by crashing the whole fold over one malformed-looking Scene.
func TestSceneSeenInitializesANilTilesMapWithoutPanicking(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	// Bypass SceneCreated deliberately, mirroring conformance.go's pattern,
	// so Tiles starts nil rather than an empty-but-non-nil map.
	st.Scenes["s"] = engine.Scene{ID: "s", Name: "S", GridWidth: 3, GridHeight: 3}
	if st.Scenes["s"].Tiles != nil {
		t.Fatal("test setup bug: Tiles must start nil to exercise the guard")
	}

	must(t, engine.Apply(st, env(2, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}}})))

	if st.Scenes["s"].Tiles["0,0"].Kind != "floor" {
		t.Error("SceneSeen must lazily initialize a nil Tiles map, not panic or silently drop the tile")
	}
}

// TestSceneSeenObjectsMergeReplacingDuplicatesAndAppendingNew exercises
// mergeObjects' two branches directly (0% covered by the three tests above,
// which only ever send Tiles): a repeated ObjectID must REPLACE in place, a
// new one must APPEND, matching the doc comment's "SceneSeen carries the
// whole currently-visible set each time" invariant — the same object arrives
// on every frame it stays visible and must not accumulate duplicates.
func TestSceneSeenObjectsMergeReplacingDuplicatesAndAppendingNew(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))

	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Objects: []*vttv1.SceneObject{
			{ObjectId: "crate-1", Kind: "crate", At: &vttv1.GridPosition{X: 0, Y: 0}, BlocksSight: false},
		}})))
	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s",
		Objects: []*vttv1.SceneObject{
			// crate-1 moved AND its sight-blocking changed: same ID, must replace.
			{ObjectId: "crate-1", Kind: "crate", At: &vttv1.GridPosition{X: 1, Y: 0}, BlocksSight: true},
			// pillar-1 is new: must append, not disturb crate-1's slot.
			{ObjectId: "pillar-1", Kind: "pillar", At: &vttv1.GridPosition{X: 2, Y: 2}},
		}})))

	objs := st.Scenes["s"].Objects
	if len(objs) != 2 {
		t.Fatalf("want 2 objects after merge (1 replaced, 1 appended), got %d: %+v", len(objs), objs)
	}
	var crate, pillar *engine.SceneObject
	for i := range objs {
		switch objs[i].ObjectID {
		case "crate-1":
			crate = &objs[i]
		case "pillar-1":
			pillar = &objs[i]
		}
	}
	if crate == nil || crate.X != 1 || !crate.BlocksSight {
		t.Errorf("crate-1 must be REPLACED in place with its new position and flag, got %+v", crate)
	}
	if pillar == nil {
		t.Error("pillar-1 must be APPENDED as a new object")
	}
}

// TestSnapshotDeepCopiesExplored is Snapshot's analogue of the existing
// TestSnapshotOfDoorStateDoesNotAliasLiveState (apply_test.go): Explored is
// the THIRD map SceneSeen mutates after a Scene is created (Tiles and
// OpenDoors are the other two, both already guarded), so Snapshot must copy
// it rather than alias it, or a snapshot holder would watch a projection's
// fog of war grow out from under it — exactly the promise Snapshot's own
// doc comment makes.
func TestSnapshotDeepCopiesExplored(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))
	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}}})))

	snap := st.Snapshot()

	// Mutate the ORIGINAL's Explored after the snapshot was taken.
	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"1,1": {Kind: "wall"}}})))

	if snap.Scenes["s"].Explored["1,1"] {
		t.Error("snapshot's Explored was mutated after Snapshot() returned — the map was aliased, not copied")
	}
	if !snap.Scenes["s"].Explored["0,0"] {
		t.Error("snapshot must still carry what was explored BEFORE it was taken")
	}
}

// TestSnapshotOfUnprojectedSceneLeavesExploredNil covers the OTHER half of
// Snapshot's Explored branch: a Scene folded from the real log (no SceneSeen
// ever applied) must snapshot with Explored still nil, not silently promoted
// to an empty map — Snapshot copies the zero value, it does not change it
// (state.go's doc comment: "EMPTY FOR THE DM AND FOR THE LOG").
func TestSnapshotOfUnprojectedSceneLeavesExploredNil(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))

	snap := st.Snapshot()

	if snap.Scenes["s"].Explored != nil {
		t.Errorf("a scene with no SceneSeen ever applied must snapshot with Explored nil, got %#v",
			snap.Scenes["s"].Explored)
	}
}
