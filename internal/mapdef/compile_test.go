package mapdef_test

import (
	"fmt"
	"strconv"
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
	envs, _, err := mapdef.Compile(m, cellarArtDir(t))
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
	envs, _, err := mapdef.Compile(m, cellarArtDir(t))
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
	envs, _, err := mapdef.Compile(m, cellarArtDir(t))
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
	// Appended rather than replaced, so the fixture's own placement stays
	// first and declaration order is a claim about THIS slice's order.
	m.Placements = append(m.Placements,
		mapdef.Placement{TokenID: "tok-rogue", ActorID: "act-rogue", X: 3, Y: 2},
		mapdef.Placement{TokenID: "tok-cleric", ActorID: "act-cleric", X: 1, Y: 2},
	)

	envs, _, err := mapdef.Compile(m, cellarArtDir(t))
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

// fullyTiledMap builds a w x h map whose every square declares a tile, which
// is what CheckEverySquarePresent requires of any map that declares tiles at
// all. Tile COUNT is what these tests are about — see BuildSceneCreated's wire
// ceiling — so the nature is uniform and uninteresting.
func fullyTiledMap(w, h int32) *mapdef.Map {
	tiles := make(map[string]string, int(w)*int(h))
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = "stone"
		}
	}
	return &mapdef.Map{ID: "big", Name: "Big", GridW: w, GridH: h, Tiles: tiles}
}

// TestASceneTooLargeForTheWireIsRefusedAtCompileRatherThanAtTheTable pins the
// transport ceiling spec §7 measured but nothing enforced.
//
// A SceneCreated carries one TileRef per DECLARED tile as protojson, at 43.5
// bytes a tile or 45.5 depending on the BUILD — protojson adds a space after
// every comma in roughly half of them, seeded from a hash of the binary by
// internal/detrand, on purpose. So a 32x32 is 43.5 KiB or 45.5 KiB, and both
// are correct. See MaxWireTiles's own doc comment; quote both or neither.
//
// The frame stops arriving somewhere past 60x60 — the first grid over is 69x69
// compact and 67x67 spaced with no art overrides, and as early as 60x60 with
// them — against the 200 KiB read limit Go clients set
// (internal/harness/client.go). Compare in BYTES: 67x67 spaced is 205224
// against readLimit's 204800, over by 424, and a KiB display rounds it to
// "200.4", which is how it was first recorded as 68x68. The failure lands as a
// torn-down connection mid-session, not as a load error, which is how loading
// goblin-ambush used to kill every connection before that limit was raised.
//
// THE LIMIT IS ON TILES, NOT ON GRID AREA, and the difference is not academic:
// tiles are optional (Patrik's ruling 2026-08-13), so a huge grid that
// declares NO terrain costs nothing on the wire and must still load. Sizing
// this on GridW*GridH would refuse maps that are free to send —
// internal/rules/conformance builds a 100x100 tile-less scene for exactly that
// reason.
func TestASceneTooLargeForTheWireIsRefusedAtCompileRatherThanAtTheTable(t *testing.T) {
	t.Run("a fully tiled map at the ceiling still compiles", func(t *testing.T) {
		m := fullyTiledMap(60, 60) // 3600 tiles: 153.6 KiB compact, 160.6 KiB spaced
		if _, _, err := mapdef.BuildSceneCreated(m, ""); err != nil {
			t.Fatalf("a 60x60 map is inside the stated ceiling and must compile: %v", err)
		}
	})

	t.Run("one tile past the ceiling is refused", func(t *testing.T) {
		m := fullyTiledMap(61, 61) // 3721 tiles
		_, _, err := mapdef.BuildSceneCreated(m, "")
		if err == nil {
			t.Fatal("a map too large to reach any client compiled without complaint")
		}
		// The number has to be IN the message: an author who cannot see the
		// budget cannot resize the map to fit it.
		if !strings.Contains(err.Error(), "3721") || !strings.Contains(err.Error(), "3600") {
			t.Errorf("error must name both the map's tile count and the ceiling, got: %v", err)
		}
	})

	t.Run("a grid far past the ceiling that declares no tiles is fine", func(t *testing.T) {
		// The tiles-optional case. 40000 squares, zero wire cost.
		m := &mapdef.Map{ID: "outdoor", Name: "Outdoor", GridW: 200, GridH: 200}
		if _, _, err := mapdef.BuildSceneCreated(m, ""); err != nil {
			t.Fatalf("a tile-less scene costs nothing on the wire and must compile: %v", err)
		}
	})
}

// TestCompilePropagatesAResolveFailure pins that Compile does not swallow a
// Resolve error: the whole call must fail loud rather than silently emitting
// a SceneCreated with a hole in its Tiles map.
//
// The fixture had to change with this task. It used to be an override naming
// art the pack did not define, and that case is no longer an error at all —
// art that is not installed degrades one square and warns (spec §4). What
// still refuses, and so is what this can be driven with, is art that IS
// installed and cannot be read.
func TestCompilePropagatesAResolveFailure(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	artDir := cellarArtDir(t)
	writeArt(t, artDir, "unreadable", `{"format_version":99,"kind":"floor"}`)
	m.Overrides["1,1"] = "unreadable"
	if _, _, err := mapdef.Compile(m, artDir); err == nil {
		t.Fatal("want an error compiling a map whose override does not resolve")
	}
}

// TestCompileDegradesAnObjectWhoseArtIsNotInstalled is what became of
// TestCompileRefusesAnObjectWhoseArtDoesNotResolve, and the inversion is
// deliberate rather than a loosening. Whole-branch-review finding I1 was that
// an object's art was checked against NOTHING: Pack.Objects was loaded and
// read by no Go code, so the reviewer's typo (copy maps/cellar, misspell
// "pillar-stone" as "pillar-stoen") produced an INVISIBLE BARRIER — the
// object still blocks its square, nothing draws there, and nothing at load
// time said why. Spec §4 now rules that the object STAYS ("an object is a
// thing in the world before it is a picture"), so the barrier is no longer
// the defect; the silence was. What this pins is that the name is read and
// the DM is told.
func TestCompileDegradesAnObjectWhoseArtIsNotInstalled(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Objects[0].Art = "pillar-stoen" // the reviewer's exact typo, one letter transposed
	envs, warnings, err := mapdef.Compile(m, cellarArtDir(t))
	if err != nil {
		t.Fatalf("compile: %v — the object stays, it just loses its picture", err)
	}
	sc := firstSceneCreated(t, envs)
	if len(sc.GetObjects()) != 1 {
		t.Fatalf("Objects = %v, want the object still there", sc.GetObjects())
	}
	got := sc.GetObjects()[0]
	if got.GetArt() != "" {
		t.Fatalf("art is %q, want empty: there is no picture to draw", got.GetArt())
	}
	if !got.GetBlocksMove() || !got.GetBlocksSight() {
		t.Fatalf("object = %v, want its blocking behaviour intact (spec §4)", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "pillar-stoen") {
		t.Fatalf("warnings = %v, want exactly one naming the unresolved art", warnings)
	}
}

// TestCompileDegradesAnObjectArtWithNoArtDirectoryAtAll pins the object half
// of "a campaign that has installed no art still loads": a map whose ONLY art
// comes from an object (no tile overrides at all) compiles against an empty
// art root, keeps its object, and warns. It is the successor to
// TestCompileRefusesAnObjectArtWithNoPackGiven, which pinned the refusal this
// task replaced, and it exercises the one path
// TestCompileWithNoArtDirectoryResolvesStandardOnlyTiles deliberately does
// NOT (that test clears m.Objects so it stays about tiles).
func TestCompileDegradesAnObjectArtWithNoArtDirectoryAtAll(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Overrides = nil // the OBJECT's art is the only art left
	_, warnings, err := mapdef.Compile(m, "")
	if err != nil {
		t.Fatalf("compile with no art directory: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "boulder-mossy-2") {
		t.Fatalf("warnings = %v, want exactly one naming the object's art", warnings)
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

	artDir := cellarArtDir(t)
	first, _, err := mapdef.Compile(m, artDir)
	if err != nil {
		t.Fatal(err)
	}
	for round := 1; round < 10; round++ {
		got, _, err := mapdef.Compile(m, artDir)
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
	envs, _, err := mapdef.Compile(m, "")
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
// grid walk (y outer, x inner) is load-bearing, not cosmetic: two DIFFERENT
// unresolved art names on the same map must produce their warnings in the
// SAME order every time (0,0's before 2,2's — row-major visits y=0's whole
// row before y=2's), which only holds if the square loop walks the grid
// directly rather than ranging m.Tiles (whose iteration order Go
// re-randomizes). warningTally preserves first-encounter order, so the walk
// is still what decides it.
// Confirmed by fault injection: swapping the nested y/x loop for `for key
// := range m.Tiles` makes this test flake across repeated runs (verified
// with `go test -run WarningsSurfaceInRowMajorOrder -count=20`, reverted).
//
// TWO DIFFERENT ART NAMES, not one on two squares, and the change is forced:
// warnings are deduplicated by exact string since art-is-a-flat-library Task
// 3's fix round, so the same art on two squares now collapses to ONE line and
// has no order to observe. Two names give two lines and the same proof.
func TestWarningsSurfaceInRowMajorOrder(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	// Both 0,0 and 2,2 are stone-wall squares; neither name is installed, so
	// each produces its own not-installed line.
	m.Overrides["0,0"] = "absent-alpha"
	m.Overrides["2,2"] = "absent-omega"

	_, warnings, err := mapdef.Compile(m, cellarArtDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2", warnings)
	}
	if !strings.Contains(warnings[0], "absent-alpha") || !strings.Contains(warnings[1], "absent-omega") {
		t.Fatalf("warnings = %v, want 0,0's art before 2,2's (row-major order)", warnings)
	}
}

// TestAKindMismatchNamesItsSquares is the exception to "warnings name the art,
// never the square". Three of the four warning kinds are actionable on the art
// name alone — install the file, write the sidecar, fix the directory — so
// collapsing them loses nothing. A kind mismatch is different: its remedy is
// one of two opposite things, "this is a deliberate illusory wall" (spec §3.2,
// the whole reason it warns instead of refusing) or "I put the wrong art on
// this square", and NOTHING BUT THE SQUARE tells them apart. A DM with one
// deliberate illusion and one typo sharing an art name would otherwise get a
// single line and no way to act on it.
func TestAKindMismatchNamesItsSquares(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	// planks-split-3 is floor art; 0,0 and 2,2 are both stone-wall squares.
	m.Overrides["0,0"] = "planks-split-3"
	m.Overrides["2,2"] = "planks-split-3"

	_, warnings, err := mapdef.Compile(m, cellarArtDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1 — one art name, one line", warnings)
	}
	// The ellipsis is a PROMISE that there are more squares than listed, and
	// nothing pinned its absence: `n > len(places)` -> `n >= len(places)` keeps
	// internal/mapdef AND internal/gateway green while every mismatch gains a
	// "…" naming squares that do not exist — including the "(1 squares: 0,0, …)"
	// shape TestASingleWarningCarriesNoCountAtAll was written to prevent one
	// field over. Both squares are listed here, so there is nothing to elide.
	if strings.Contains(warnings[0], "…") {
		t.Errorf("warning = %q, want no ellipsis: every square it counts is listed, "+
			"so a \"…\" promises squares that do not exist", warnings[0])
	}
	for _, want := range []string{"planks-split-3", "0,0", "2,2"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning = %q, want it to contain %q", warnings[0], want)
		}
	}
}

// TestAKindMismatchOnManySquaresStopsListingAndCounts pins the cap: listing
// every square would rebuild the unbounded output deduplication exists to
// remove, so past a handful the count carries the scale and an ellipsis says
// the list was cut. Mismatches are rare by construction, so this is the
// unusual case rather than the normal one.
func TestAKindMismatchOnManySquaresStopsListingAndCounts(t *testing.T) {
	tiles := make(map[string]string, 9)
	overrides := make(map[string]string, 9)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			key := strconv.Itoa(x) + "," + strconv.Itoa(y)
			tiles[key] = "stone-wall"
			overrides[key] = "planks-split-3" // floor art on nine wall squares
		}
	}
	m := &mapdef.Map{ID: "wall", Name: "Wall", GridW: 3, GridH: 3,
		Tiles: tiles, Overrides: overrides}

	_, warnings, err := mapdef.Compile(m, cellarArtDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
	if !strings.Contains(warnings[0], "9 squares") || !strings.Contains(warnings[0], "…") {
		t.Fatalf("warning = %q, want the full count and an ellipsis marking the cut list",
			warnings[0])
	}
	// Row-major, so the listed keys are 0,0 1,0 2,0 0,1 — the last square the
	// walk reaches must NOT be in the message, which is what proves the list
	// was cut rather than merely decorated with an ellipsis.
	if strings.Contains(warnings[0], "2,2") {
		t.Fatalf("warning = %q, want at most a handful of squares listed", warnings[0])
	}
}

// TestASingleWarningCarriesNoCountAtAll kills a mutant that survived the first
// version of warningTally: `n > 1` mutated to `n > 0` left internal/mapdef and
// internal/gateway entirely green, and every single-square warning would then
// have read "… (1 squares)" — ungrammatical text, on the shipped campaign, in
// front of a DM. A count is only worth saying when there is more than one thing
// to count.
func TestASingleWarningCarriesNoCountAtAll(t *testing.T) {
	m := &mapdef.Map{ID: "hall", Name: "Hall", GridW: 1, GridH: 1,
		Tiles:     map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "absent-once"}}

	_, warnings, err := mapdef.Compile(m, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
	if strings.Contains(warnings[0], "(") {
		t.Fatalf("warning = %q — one square needs no count, and \"(1 squares)\" is text a "+
			"DM would read on the shipped campaign", warnings[0])
	}
}

// TestOneMissingArtNameCostsOneWarningNoMatterHowManySquares is spec §4's
// "the DM is told... once", and it is a size limit as much as a tidiness rule.
// Measured on campaigns/example/maps/cellar.json before deduplication landed:
// four missing art names over 90 squares produced 96 warnings and 6840 bytes,
// which client/src/app.ts joins into a single untruncated toast; the same
// shape at MaxWireTiles is roughly 270 KB in one CommandResult, over the
// 200 KiB read limit Go clients set — so the frame would not arrive at all and
// a DM would see a dropped connection rather than a warning.
//
// The COUNT is asserted, not just the collapse: "not installed" without "how
// much of the map went plain" leaves a DM unable to tell one stray square from
// a whole floor.
func TestOneMissingArtNameCostsOneWarningNoMatterHowManySquares(t *testing.T) {
	tiles := make(map[string]string, 9)
	overrides := make(map[string]string, 9)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			key := strconv.Itoa(x) + "," + strconv.Itoa(y)
			tiles[key] = "stone-wall"
			overrides[key] = "absent-everywhere"
		}
	}
	m := &mapdef.Map{ID: "wall", Name: "Wall", GridW: 3, GridH: 3,
		Tiles: tiles, Overrides: overrides}

	_, warnings, err := mapdef.Compile(m, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1 for one art name across 9 squares", warnings)
	}
	if !strings.Contains(warnings[0], "absent-everywhere") || !strings.Contains(warnings[0], "9 squares") {
		t.Fatalf("warning = %q, want it to name the art and how many squares went plain", warnings[0])
	}
}

// TestSquaresAndObjectsAreTalliedSeparately pins that the same missing art
// name reaching both layers is reported as two facts, not summed into one
// number a DM cannot map onto anything: a floor drawn plain and a barrel drawn
// plain are different things to go and look at.
func TestSquaresAndObjectsAreTalliedSeparately(t *testing.T) {
	m := &mapdef.Map{ID: "hall", Name: "Hall", GridW: 1, GridH: 1,
		Tiles:     map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "shared-name"},
		Objects: []mapdef.Object{
			{ID: "o1", Kind: "barrel", Art: "shared-name"},
			{ID: "o2", Kind: "barrel", Art: "shared-name"},
		}}
	_, warnings, err := mapdef.Compile(m, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want 2 (one for the square layer, one for the objects)", warnings)
	}
	if !strings.Contains(warnings[0], "drawing it plain") {
		t.Fatalf("warnings[0] = %q, want the square line first", warnings[0])
	}
	if !strings.Contains(warnings[1], "2 objects") {
		t.Fatalf("warnings[1] = %q, want the object line to count objects, not squares", warnings[1])
	}
}

// TestCompileWithNoArtDirectoryResolvesStandardOnlyTiles pins that a map
// using only standard tiles (no overrides) compiles with no art root at all —
// the common case for a map that names no custom art, and the reason the
// built-in vocabulary is the layer everything else degrades TO.
//
// m.Objects is cleared alongside m.Overrides so this stays a test about
// tiles: cellar.json's one object names art ("boulder-mossy-2"), which
// against an empty art root now degrades and WARNS rather than refusing, and
// a stray warning here would be a second thing this test was silently also
// asserting. That object case has its own test:
// TestCompileDegradesAnObjectArtWithNoArtDirectoryAtAll.
func TestCompileWithNoArtDirectoryResolvesStandardOnlyTiles(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Overrides = nil // no art overrides left: nothing needs a pack
	m.Objects = nil   // ditto for the one object's own art (see doc comment above)

	envs, _, err := mapdef.Compile(m, "")
	if err != nil {
		t.Fatal(err)
	}
	sc := firstSceneCreated(t, envs)
	if got := sc.GetTiles()["1,1"]; got.GetArt() != "" || got.GetKind() != "floor" || got.GetMaterial() != "wood" {
		t.Fatalf("tiles[1,1] = %v, want plain wood floor with no art", got)
	}
}
