package mapdef_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// firstSceneCreated pulls the one SceneCreated payload a compiled batch must
// carry — a small local helper rather than importing conformance's own dump
// machinery, which this package may not depend on (mapdef stays self-only
// plus contract; conformance is adventure's proof harness, a different
// component entirely).
func firstSceneCreated(t *testing.T, envs []*vttv1.Envelope) *vttv1.SceneCreated {
	t.Helper()
	for _, e := range envs {
		if sc := e.GetSceneCreated(); sc != nil {
			return sc
		}
	}
	t.Fatal("no SceneCreated found in the compiled batch")
	return nil
}

// TestASceneCreatedCarriesEverySquare pins Compile's scene half: a 3x3 map
// (testdata/valid/cellar.json) produces a SceneCreated whose Tiles map has
// one entry per grid square, and the override at "1,1" leaves Material
// alone (spec §3.2 — nature always comes from the base tile, never the
// pack).
func TestASceneCreatedCarriesEverySquare(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}
	envs, _, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}
	sc := firstSceneCreated(t, envs)
	if len(sc.GetTiles()) != 9 {
		t.Fatalf("3x3 scene carries %d squares, want 9", len(sc.GetTiles()))
	}
	if sc.GetTiles()["1,1"].GetMaterial() != "wood" {
		t.Fatalf("material did not survive compile: %v", sc.GetTiles()["1,1"])
	}
}

// TestASceneCreatedCarriesArtAndObjects pins the two facts
// TestASceneCreatedCarriesEverySquare does not: the override's ART survives
// (not just that Material stayed put), and Objects converts field-by-field
// — mirroring load_test.go's own
// TestObjectFieldsSurviveTheJSONToMapShapeConversion precedent that an
// untested conversion is exactly where a transposed field hides.
func TestASceneCreatedCarriesArtAndObjects(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}
	envs, _, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}
	sc := firstSceneCreated(t, envs)

	if got := sc.GetTiles()["1,1"]; got.GetArt() != "planks-split-3" || got.GetKind() != "floor" {
		t.Fatalf("tiles[1,1] = %v, want art planks-split-3, kind floor", got)
	}
	if got := sc.GetTiles()["0,0"]; got.GetArt() != "" || got.GetKind() != "wall" || got.GetMaterial() != "stone" {
		t.Fatalf("tiles[0,0] = %v, want a plain stone-wall with no art", got)
	}

	if len(sc.GetObjects()) != 1 {
		t.Fatalf("Objects = %v, want exactly 1", sc.GetObjects())
	}
	want := &vttv1.SceneObject{
		ObjectId: "boulder-1", Kind: "boulder",
		At:    &vttv1.GridPosition{X: 0, Y: 1},
		Width: 1, Height: 1, RotationDegrees: 0,
		BlocksSight: true, BlocksMove: true,
		Art: "boulder-mossy-2",
	}
	if got := sc.GetObjects()[0]; !proto.Equal(got, want) {
		t.Fatalf("Objects[0] = %v, want %v", got, want)
	}
}

// TestCompileEmitsSceneThenOneTokenPlacedPerPlacement pins Compile's own
// ordering promise (this task's Interfaces section): exactly one
// SceneCreated followed by one TokenPlaced per placement, in declaration
// order — not "a SceneCreated somewhere in the batch".
func TestCompileEmitsSceneThenOneTokenPlacedPerPlacement(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}
	envs, _, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}

	// cellar.json declares exactly one placement (tok-fighter at 2,1).
	if len(envs) != 2 {
		t.Fatalf("got %d envelopes, want 2 (one SceneCreated, one TokenPlaced)", len(envs))
	}
	if envs[0].GetSceneCreated() == nil {
		t.Fatalf("envs[0] = %v, want a SceneCreated", envs[0])
	}
	tp := envs[1].GetTokenPlaced()
	if tp == nil {
		t.Fatalf("envs[1] = %v, want a TokenPlaced", envs[1])
	}
	want := &vttv1.TokenPlaced{
		TokenId: "tok-fighter", SceneId: "cellar", ActorId: "act-fighter",
		Position: &vttv1.GridPosition{X: 2, Y: 1},
	}
	if !proto.Equal(tp, want) {
		t.Fatalf("TokenPlaced = %v, want %v", tp, want)
	}
}

// TestCompileEmitsEveryPlacementOfAMapThatDeclaresMoreThanOne is the
// multi-placement case, and it exists because the single-placement test above
// cannot be what its own name promises: with one placement, "in declaration
// order" has no order to observe, and one TokenPlaced comes out whether
// Compile ranges the slice or returns its first element.
//
// It also kills ARITHMETIC_BASE at compile.go:23:38 — the `1+len(m.Placements)`
// capacity hint, mutated to `1-len(...)`. That mutant looks like the map
// capacity hints adjudicated as equivalent in tools/mutation-equivalents.txt
// (campaign.go:448), and it is NOT: those are maps, this is a slice, and gc
// panics on a negative slice capacity where it tolerates a negative map hint
// ("makeslice: cap out of range", verified). So the mutation is observable
// from two placements up — and survived only because nothing compiled two.
//
// It survived a manual gremlins run as a TIMED OUT mutant, which gremlins
// scores as killed in its efficacy percentage; the gate's own
// timeout-coefficient let it run to completion, where it LIVED. A timeout is
// not a kill.
func TestCompileEmitsEveryPlacementOfAMapThatDeclaresMoreThanOne(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}
	// Appended rather than replaced, so the fixture's own placement stays
	// first and declaration order is a claim about THIS slice's order.
	m.Placements = append(m.Placements,
		mapdef.Placement{TokenID: "tok-rogue", ActorID: "act-rogue", X: 3, Y: 2},
		mapdef.Placement{TokenID: "tok-cleric", ActorID: "act-cleric", X: 1, Y: 2},
	)

	envs, _, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 4 {
		t.Fatalf("got %d envelopes, want 4 (one SceneCreated, three TokenPlaced)", len(envs))
	}
	if envs[0].GetSceneCreated() == nil {
		t.Fatalf("envs[0] = %v, want a SceneCreated", envs[0])
	}

	want := []*vttv1.TokenPlaced{
		{TokenId: "tok-fighter", SceneId: "cellar", ActorId: "act-fighter",
			Position: &vttv1.GridPosition{X: 2, Y: 1}},
		{TokenId: "tok-rogue", SceneId: "cellar", ActorId: "act-rogue",
			Position: &vttv1.GridPosition{X: 3, Y: 2}},
		{TokenId: "tok-cleric", SceneId: "cellar", ActorId: "act-cleric",
			Position: &vttv1.GridPosition{X: 1, Y: 2}},
	}
	for i, w := range want {
		got := envs[i+1].GetTokenPlaced()
		if got == nil {
			t.Fatalf("envs[%d] = %v, want a TokenPlaced", i+1, envs[i+1])
		}
		if !proto.Equal(got, w) {
			t.Errorf("envs[%d] TokenPlaced = %v, want %v", i+1, got, w)
		}
	}
}

// TestCompilePropagatesAResolveFailure pins that Compile does not swallow a
// Resolve error (an override naming an art the pack does not define, say):
// the whole call must fail loud rather than silently emitting a SceneCreated
// with a hole in its Tiles map.
func TestCompilePropagatesAResolveFailure(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Overrides["1,1"] = "no-such-tile"
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := mapdef.Compile(m, p); err == nil {
		t.Fatal("want an error compiling a map whose override does not resolve")
	}
}

// TestCompileRefusesAnObjectWhoseArtDoesNotResolve is whole-branch-review
// finding I1's exact reproduction: p.Objects was loaded by LoadPack and read
// by nothing in Go, so a typo'd object art (the reviewer's own proof: copy
// maps/cellar, misspell "pillar-stone" as "pillar-stoen") passed every check
// this package ran and produced an INVISIBLE BARRIER — the object still
// blocks its square (blocks_move survives untouched), but nothing draws
// there and nothing at load time said why. Mirrors
// TestCompilePropagatesAResolveFailure's shape exactly, one layer over:
// an override's bad art fails through Resolve; an object's bad art must
// fail the identical way through the new ResolveObjectArt.
func TestCompileRefusesAnObjectWhoseArtDoesNotResolve(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Objects[0].Art = "pillar-stoen" // the reviewer's exact typo, one letter transposed
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = mapdef.Compile(m, p)
	if err == nil {
		t.Fatal("want an error compiling a map whose object art does not resolve — " +
			"this is I1's invisible barrier: the object still blocks its square, " +
			"but nothing would ever be drawn there")
	}
	if !strings.Contains(err.Error(), "pillar-stoen") {
		t.Fatalf("error should name the unresolved art, got: %v", err)
	}
}

// TestCompileRefusesAnObjectArtWithNoPackGiven pins the OTHER half of
// Resolve's own nil-pack guard (see its doc comment), now applied to
// objects: a map whose only art comes from an object (no tile overrides at
// all) must still refuse to compile against p == nil, exactly as it would if
// the same art sat in overrides instead. Without this, a map with zero
// overrides but one object could compile "successfully" against a
// completely missing pack — the object's art silently never resolving to
// anything, which is the invisible-barrier bug the WHOLE task exists to
// close, reached through the one path TestCompileWithNilPackResolvesStandardOnlyTiles
// deliberately does NOT exercise (that test clears m.Objects specifically so
// it stays about tiles).
func TestCompileRefusesAnObjectArtWithNoPackGiven(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Overrides = nil // no tile art needs a pack; the OBJECT's art still does
	if _, _, err := mapdef.Compile(m, nil); err == nil {
		t.Fatal("want an error compiling a map whose object names art but gives Compile no pack to resolve it against")
	}
}

// TestCompileIsDeterministic mirrors internal/adventure's own
// TestCompileIsDeterministic: BuildSceneCreated's square loop walks the grid
// by coordinate (row-major), never ranges m.Tiles directly, so two calls
// against the same (m, p) must always agree — across many rounds, since Go's
// map iteration order is re-randomized per range statement and a
// map-iteration regression would very likely disagree with itself across
// several rounds even where two calls happened to agree once.
func TestCompileIsDeterministic(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}

	first, _, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}
	for round := 1; round < 10; round++ {
		got, _, err := mapdef.Compile(m, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(first) {
			t.Fatalf("round %d: got %d envelopes, want %d", round, len(got), len(first))
		}
		for i := range first {
			if !proto.Equal(got[i], first[i]) {
				t.Errorf("round %d, envelope[%d]:\n got  %v\n want %v", round, i, got[i], first[i])
			}
		}
	}
}

// TestBuildSceneCreatedWithNoTilesHasNoTerrain pins Patrik's ruling
// (2026-08-13): a map with no tiles declared compiles to a SceneCreated
// with an empty/absent Tiles map — not an error, and not a Resolve call per
// square (which would otherwise fail "square has no tile" for every one of
// them). TokenPlaced is unaffected — a placement does not need terrain to
// exist as a fact on the log.
func TestBuildSceneCreatedWithNoTilesHasNoTerrain(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/no-terrain.json")
	if err != nil {
		t.Fatal(err)
	}
	envs, _, err := mapdef.Compile(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	sc := firstSceneCreated(t, envs)
	if len(sc.GetTiles()) != 0 {
		t.Fatalf("Tiles = %v, want empty (no terrain declared)", sc.GetTiles())
	}
	if len(envs) != 2 { // SceneCreated + the fixture's one TokenPlaced
		t.Fatalf("got %d envelopes, want 2", len(envs))
	}
}

// TestWarningsSurfaceInRowMajorOrder proves BuildSceneCreated's row-major
// grid walk (y outer, x inner) is load-bearing, not cosmetic: two mismatched
// overrides on the same map must produce their warnings in the SAME order
// every time (0,0 before 2,2 — row-major visits y=0's whole row before
// y=2's), which only holds if the square loop walks the grid directly
// rather than ranging m.Tiles (whose iteration order Go re-randomizes).
// Confirmed by fault injection: swapping the nested y/x loop for `for key
// := range m.Tiles` makes this test flake across repeated runs (verified
// with `go test -run WarningsSurfaceInRowMajorOrder -count=20`, reverted).
func TestWarningsSurfaceInRowMajorOrder(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	// planks-split-3 is a floor tile (spec: pack advisory kind "floor");
	// both 0,0 and 2,2 are stone-wall squares, so both mismatch.
	m.Overrides["0,0"] = "planks-split-3"
	m.Overrides["2,2"] = "planks-split-3"
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatal(err)
	}

	_, warnings, err := mapdef.Compile(m, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2", warnings)
	}
	if !strings.Contains(warnings[0], "square 0,0") || !strings.Contains(warnings[1], "square 2,2") {
		t.Fatalf("warnings = %v, want square 0,0 before square 2,2 (row-major order)", warnings)
	}
}

// TestCompileWithNilPackResolvesStandardOnlyTiles pins that a map using only
// standard tiles (no overrides) compiles with p == nil — the common case for
// a map that names no custom pack at all, and the same nil-tolerance
// Resolve itself documents. m.Objects is cleared alongside m.Overrides,
// deliberately, now that ResolveObjectArt exists (whole-branch-review I1):
// cellar.json's one object still names art ("boulder-mossy-2"), and unlike a
// tile an object has no standard fallback, so it would ALSO need a pack —
// leaving it in place would make this test assert something no longer true
// and fail for a reason unrelated to what it is actually pinning. The
// object-specific case (an object's art with no pack to resolve it against)
// has its own test: TestCompileRefusesAnObjectArtWithNoPackGiven.
func TestCompileWithNilPackResolvesStandardOnlyTiles(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Overrides = nil // no art overrides left: nothing needs a pack
	m.Objects = nil   // ditto for the one object's own art (see doc comment above)

	envs, _, err := mapdef.Compile(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	sc := firstSceneCreated(t, envs)
	if got := sc.GetTiles()["1,1"]; got.GetArt() != "" || got.GetKind() != "floor" || got.GetMaterial() != "wood" {
		t.Fatalf("tiles[1,1] = %v, want plain wood floor with no art", got)
	}
}
