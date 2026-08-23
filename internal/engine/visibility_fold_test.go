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

// TestSceneSeenReplacesTheVisibleSetWithoutForgettingTheExploredOne pins the
// half of SceneSeen that Explored alone cannot express.
//
// Explored is a union and never shrinks, which is right for terrain memory and
// useless for answering "can this seat see that square RIGHT NOW". Both answers
// are needed at once, because the fog the client draws is the DIFFERENCE
// (visibility spec §6.1): Explored − Visible is ground you remember and cannot
// currently see. SceneSeen carries the whole current visible set every time
// (spec §5), so the newest one IS that set — hence replaced, never merged.
func TestSceneSeenReplacesTheVisibleSetWithoutForgettingTheExploredOne(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))

	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles:   map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}},
		Visible: []string{"0,0"}})))
	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s",
		Tiles:   map[string]*vttv1.TileRef{"1,1": {Kind: "wall"}},
		Visible: []string{"1,1"}})))

	sc := st.Scenes["s"]
	if !sc.Visible["1,1"] {
		t.Error("the newest SceneSeen is the whole current visible set: 1,1 must be visible")
	}
	if sc.Visible["0,0"] {
		t.Error("0,0 was visible one event ago and is not now — Visible is REPLACED, never unioned")
	}
	if !sc.Explored["0,0"] || !sc.Explored["1,1"] {
		t.Errorf("both squares have been seen at some point and must stay explored, got %v", sc.Explored)
	}
}

// TestAnEmptySceneSeenDarkensTheSceneAndForgetsNoTerrain is the wire message
// the projection sends when a seat can no longer see anything of a scene it has
// been in (internal/gateway's transitions). It must land as "you see nothing
// here now", NOT as "you were never here" — the second would erase a player's
// map every time they walked out of a room.
func TestAnEmptySceneSeenDarkensTheSceneAndForgetsNoTerrain(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))
	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles:   map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}},
		Visible: []string{"0,0"}})))

	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s"})))

	sc := st.Scenes["s"]
	if len(sc.Visible) != 0 {
		t.Errorf("an empty SceneSeen means nothing is visible, got %v", sc.Visible)
	}
	if sc.Visible == nil {
		t.Error("a seat that has received a projection must be distinguishable from one that " +
			"never has: darkened is an EMPTY set, not the nil that means 'no SceneSeen ever'")
	}
	if !sc.Explored["0,0"] {
		t.Error("terrain already mapped survives the room going dark")
	}
	if sc.Tiles["0,0"].Kind != "floor" {
		t.Error("and so does the terrain itself — there is no message that un-explores a square")
	}
}

// TestASnapshotHoldsTheVisibleSetItWasTakenAt is TestSnapshotDeepCopiesExplored's
// twin in shape, and deliberately NOT in its claim.
//
// It does not prove Snapshot deep-copies Visible, and it cannot: replacing the
// copy with `visible := v.Visible` was tried and the whole engine suite stayed
// green. The reason is in the fold — the SceneSeen arm ASSIGNS a new Visible
// map on every message and never writes into the one a snapshot is holding, so
// the alias has nothing to observe. What this test does hold is that the
// snapshot carries the set as it stood, both directions: what was visible then
// is there, and what became visible afterwards is not. The second half is the
// one that would start failing the day anything mutates Visible in place, which
// is when state.go's copy stops being belt-and-braces.
func TestASnapshotHoldsTheVisibleSetItWasTakenAt(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))
	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles:   map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}},
		Visible: []string{"0,0"}})))

	snap := st.Snapshot()

	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s",
		Tiles:   map[string]*vttv1.TileRef{"1,1": {Kind: "wall"}},
		Visible: []string{"1,1"}})))

	if !snap.Scenes["s"].Visible["0,0"] {
		t.Error("the snapshot must still hold what was visible when it was taken")
	}
	if snap.Scenes["s"].Visible["1,1"] {
		t.Error("a snapshot must not pick up what became visible after it was taken")
	}
}

// TestSnapshotOfUnprojectedSceneLeavesVisibleNil is the nil/empty distinction
// state.go calls load-bearing, held at the Snapshot boundary: promoting nil to
// an empty map here would tell a renderer downstream that a DM's scene had
// received a projection saying "you see nothing".
func TestSnapshotOfUnprojectedSceneLeavesVisibleNil(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))

	if st.Scenes["s"].Visible != nil {
		t.Errorf("SceneCreated must leave Visible nil, got %#v", st.Scenes["s"].Visible)
	}
	if snap := st.Snapshot(); snap.Scenes["s"].Visible != nil {
		t.Errorf("a scene with no SceneSeen ever applied must snapshot with Visible nil, got %#v",
			snap.Scenes["s"].Visible)
	}
}

// TestVisibleComesFromItsOwnFieldNotFromTheTiles pins the change of source
// this arm underwent on 2026-08-22, and it is written so that reverting to the
// old rule fails it rather than merely looking different.
//
// Before, Visible was built inside the tile loop, so "visible" silently meant
// "visible AND declares terrain". SceneSeen now carries the square set as
// itself, and the two fields no longer share a source at all: Visible is the
// server's sight answer, Explored is terrain remembered. A message can
// therefore say "you can see these nine squares, and none of them has terrain",
// which is exactly a bare canvas — and the old rule could not express it.
func TestVisibleComesFromItsOwnFieldNotFromTheTiles(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 2, GridHeight: 2})))

	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Visible: []string{"0,0", "0,1", "1,0", "1,1"}})))

	sc := st.Scenes["s"]
	if len(sc.Visible) != 4 {
		t.Errorf("all four squares are visible whether or not they declare terrain, got %v", sc.Visible)
	}
	if len(sc.Explored) != 0 {
		t.Errorf("no terrain arrived, so nothing is explored, got %v", sc.Explored)
	}
	if len(sc.Tiles) != 0 {
		t.Errorf("and nothing is drawable, got %v", sc.Tiles)
	}
}

// TestTerrainWithoutSightIsRememberedButNotVisible is the opposite corner, and
// the pair is what stops a reader assuming the two fields track each other. A
// message may carry terrain for a square it does NOT list as visible — that is
// what the projection sends for ground you have walked out of, and it is the
// fog (Explored minus Visible).
func TestTerrainWithoutSightIsRememberedButNotVisible(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 2, GridHeight: 2})))

	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles:   map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}, "1,1": {Kind: "wall"}},
		Visible: []string{"0,0"}})))

	sc := st.Scenes["s"]
	if !sc.Explored["1,1"] {
		t.Error("terrain that arrived is remembered, whether or not it is currently in sight")
	}
	if sc.Visible["1,1"] {
		t.Error("but it is NOT visible: the visible set is the server's own answer, not the tile keys")
	}
	if !sc.Visible["0,0"] || !sc.Explored["0,0"] {
		t.Error("the square that is both must be both")
	}
}
