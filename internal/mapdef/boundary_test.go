package mapdef_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// BOUNDARY VALUES, and every test here exists because a mutant survived.
//
// The first mutation run over this package (maps-as-geometry, 2026-08-17) left
// 13 survivors and every one was CONDITIONALS_BOUNDARY or ARITHMETIC_BASE:
// `< 1` mutated to `<= 1`, `>= w` to `> w`, `+` to `-` in the footprint
// arithmetic. Nothing else lived. That is the exact signature of a suite whose
// fixtures are all comfortably valid or comfortably invalid and never sit ON
// the edge — the tests proved the checks reject nonsense, not that they reject
// the FIRST bad value and accept the LAST good one.
//
// These are written against the exported Check* helpers rather than through
// Load, so each boundary is pinned in isolation: a file-level test would need
// a whole fixture directory per case and would still leave which check fired
// ambiguous.

func errf(field, msg string) error {
	return errors.New(field + ": " + msg)
}

func fullTiles(w, h int32) map[string]string {
	t := map[string]string{}
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			t[squareKeyForTest(x, y)] = "stone"
		}
	}
	return t
}

// squareKeyForTest mirrors mapdef's own "x,y" convention. Deliberately spelled
// out rather than exported from the package: a test that shared the formatter
// would pass even if that formatter were wrong.
func squareKeyForTest(x, y int32) string {
	return itoa(x) + "," + itoa(y)
}

func itoa(v int32) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func TestAOneBySquareGridIsTheSmallestLegalMap(t *testing.T) {
	// Kills `GridWidth < 1` -> `<= 1` and the GridHeight twin: a 1x1 map is
	// the smallest thing anyone can author, and mutating the comparison
	// rejects it while still rejecting 0, so every existing fixture passes.
	if err := mapdef.CheckEverySquarePresent(fullTiles(1, 1), 1, 1, errf); err != nil {
		t.Fatalf("a 1x1 grid with its one square declared was refused: %v", err)
	}
	if err := mapdef.CheckTilesInsideGrid(fullTiles(1, 1), 1, 1, errf); err != nil {
		t.Fatalf("a 1x1 grid's only square was called out-of-grid: %v", err)
	}
}

// TestRequireEverySquarePresentHasNoOptOut pins the ONE property the split
// between the two completeness functions exists to create, in the package that
// owns them.
//
// CheckEverySquarePresent returns nil on an empty tiles map — the file format's
// opt-out, so a map authored before the format had terrain keeps loading
// (Patrik's ruling, 2026-08-13). RequireEverySquarePresent is the same walk
// WITHOUT it, for internal/gateway's create_scene, which has no such past.
//
// WITHOUT THIS TEST THE SPLIT IS UNGUARDED HERE. Every other test in this
// package reaches the walk through the lenient wrapper, so re-adding
// `if len(tiles) == 0 { return nil }` to the strict function leaves mapdef's
// own suite entirely green — and check:mutation runs per package, so that
// mutant would survive with the only evidence against it living one package
// away.
func TestRequireEverySquarePresentHasNoOptOut(t *testing.T) {
	// The pair on the SAME input, which is the whole claim: one exempts an
	// empty map, the other counts it as short of every square.
	if err := mapdef.CheckEverySquarePresent(map[string]string{}, 2, 2, errf); err != nil {
		t.Fatalf("the file path's opt-out is gone: an empty tiles map was refused: %v", err)
	}
	err := mapdef.RequireEverySquarePresent(map[string]string{}, 2, 2, errf)
	if err == nil {
		t.Fatal("a tiles map declaring nothing was accepted by the strict walk — " +
			"an undeclared square is undeclared whether it is one of four or four of four")
	}
	if !strings.Contains(err.Error(), `tiles["0,0"]`) {
		t.Fatalf("the refusal does not name a missing square: %v", err)
	}

	// And the strict walk still ACCEPTS a complete map, so "refuses everything"
	// cannot pass for "has no opt-out".
	if err := mapdef.RequireEverySquarePresent(fullTiles(2, 2), 2, 2, errf); err != nil {
		t.Fatalf("a complete 2x2 was refused by the strict walk: %v", err)
	}
}

func TestTheLastSquareInsideTheGridIsAcceptedAndTheFirstOutsideIsNot(t *testing.T) {
	// Kills `x >= w` -> `x > w` (and the y twin) at both call sites: with the
	// mutation, a square at exactly x==w is accepted — one column beyond the
	// grid, which is the single most likely off-by-one an author hits.
	const w, h int32 = 3, 2
	inside := fullTiles(w, h)
	if err := mapdef.CheckTilesInsideGrid(inside, w, h, errf); err != nil {
		t.Fatalf("the last in-grid square (%d,%d) was refused: %v", w-1, h-1, err)
	}

	for _, bad := range []struct{ x, y int32 }{{w, 0}, {0, h}} {
		out := fullTiles(w, h)
		out[squareKeyForTest(bad.x, bad.y)] = "stone"
		err := mapdef.CheckTilesInsideGrid(out, w, h, errf)
		if err == nil {
			t.Fatalf("a square at (%d,%d) was accepted on a %dx%d grid — that is one "+
				"step outside, the first value that must fail", bad.x, bad.y, w, h)
		}
		if !strings.Contains(err.Error(), "outside the grid") {
			t.Fatalf("wrong refusal for (%d,%d): %v", bad.x, bad.y, err)
		}
	}
}

func TestAnObjectMayFillTheGridExactlyButNotOverhangItByOne(t *testing.T) {
	// Kills the ARITHMETIC_BASE pair (`X + W` -> `X - W`) and the
	// CONDITIONALS_BOUNDARY pair on the same lines. An object whose footprint
	// ends exactly at the grid edge is legal — a wall running the full width
	// of a room is the ordinary case — and one square more is not. With `+`
	// mutated to `-`, every in-grid object still passes and the check stops
	// meaning anything.
	const w, h int32 = 4, 3
	exact := []mapdef.Object{{ID: "o1", Kind: "wall", X: 0, Y: 0, W: w, H: h, Art: "a"}}
	if err := mapdef.CheckObjectFootprints(exact, w, h, errf); err != nil {
		t.Fatalf("an object filling the grid exactly was refused: %v", err)
	}

	corner := []mapdef.Object{{ID: "o2", Kind: "crate", X: w - 1, Y: h - 1, W: 1, H: 1, Art: "a"}}
	if err := mapdef.CheckObjectFootprints(corner, w, h, errf); err != nil {
		t.Fatalf("a 1x1 object on the last square was refused: %v", err)
	}

	for _, bad := range []mapdef.Object{
		{ID: "o3", Kind: "crate", X: 0, Y: 0, W: w + 1, H: h, Art: "a"},
		{ID: "o4", Kind: "crate", X: 0, Y: 0, W: w, H: h + 1, Art: "a"},
		{ID: "o5", Kind: "crate", X: w, Y: 0, W: 1, H: 1, Art: "a"},
	} {
		if err := mapdef.CheckObjectFootprints([]mapdef.Object{bad}, w, h, errf); err == nil {
			t.Fatalf("object %q (at %d,%d size %dx%d) overhangs a %dx%d grid by one and was "+
				"accepted", bad.ID, bad.X, bad.Y, bad.W, bad.H, w, h)
		}
	}
}

func TestAnObjectFootprintOfExactlyOneIsLegalAndZeroIsNot(t *testing.T) {
	// Kills `W < 1` -> `<= 1`: 1x1 is by far the commonest object size (every
	// pillar, crate and barrel in the demo pack), so a mutation rejecting it
	// while still rejecting 0 would break nearly every real map — and no test
	// used the value 1 at the boundary.
	const w, h int32 = 2, 2
	ok := []mapdef.Object{{ID: "o1", Kind: "pillar", X: 0, Y: 0, W: 1, H: 1, Art: "a"}}
	if err := mapdef.CheckObjectFootprints(ok, w, h, errf); err != nil {
		t.Fatalf("a 1x1 object was refused: %v", err)
	}
	for _, bad := range []mapdef.Object{
		{ID: "o2", Kind: "pillar", X: 0, Y: 0, W: 0, H: 1, Art: "a"},
		{ID: "o3", Kind: "pillar", X: 0, Y: 0, W: 1, H: 0, Art: "a"},
	} {
		if err := mapdef.CheckObjectFootprints([]mapdef.Object{bad}, w, h, errf); err == nil {
			t.Fatalf("object %q with a zero dimension (%dx%d) was accepted", bad.ID, bad.W, bad.H)
		}
	}
}

func TestLoadAcceptsTheSmallestPossibleMapFile(t *testing.T) {
	// Kills Load's OWN `grid_width < 1` / `grid_height < 1` -> `<= 1` (the two
	// survivors at load.go:106 and :109). The Check* helpers are exercised
	// directly above, but Load duplicates the grid-sanity test before calling
	// them, and only a real FILE reaches that copy. Every other valid fixture
	// is 3x3 or larger, so 1 — the first legal value — was never loaded.
	m, err := mapdef.Load("testdata/valid/one-square.json")
	if err != nil {
		t.Fatalf("a 1x1 map file was refused: %v", err)
	}
	if m.GridW != 1 || m.GridH != 1 || len(m.Tiles) != 1 {
		t.Fatalf("loaded %dx%d with %d tiles, want 1x1 with 1", m.GridW, m.GridH, len(m.Tiles))
	}
}

func TestOverrideAndPlacementBoundsAcceptTheLastSquareAndRefuseTheFirstOutside(t *testing.T) {
	// Kills the CONDITIONALS_BOUNDARY pairs at load.go:270 (overrides) and
	// :358 (placements) — the same `>= w` -> `> w` shape already pinned for
	// tiles, in two checks that had no boundary case of their own. Three
	// copies of one comparison need three tests; a suite that pins only the
	// first is pinning a habit, not a rule.
	const w, h int32 = 3, 2

	last := squareKeyForTest(w-1, h-1)
	if err := mapdef.CheckOverridesInsideGrid(map[string]string{last: "art"}, w, h, errf); err != nil {
		t.Fatalf("an override on the last in-grid square %s was refused: %v", last, err)
	}
	for _, bad := range []struct{ x, y int32 }{{w, 0}, {0, h}} {
		key := squareKeyForTest(bad.x, bad.y)
		if err := mapdef.CheckOverridesInsideGrid(map[string]string{key: "art"}, w, h, errf); err == nil {
			t.Fatalf("an override at %s was accepted on a %dx%d grid", key, w, h)
		}
	}

	onLast := []mapdef.Placement{{TokenID: "t", ActorID: "a", X: w - 1, Y: h - 1}}
	if err := mapdef.CheckPlacementsNotInWalls(onLast, fullTiles(w, h), w, h, errf); err != nil {
		t.Fatalf("a placement on the last in-grid square was refused: %v", err)
	}
	for _, bad := range []mapdef.Placement{
		{TokenID: "t", ActorID: "a", X: w, Y: 0},
		{TokenID: "t", ActorID: "a", X: 0, Y: h},
	} {
		if err := mapdef.CheckPlacementsNotInWalls([]mapdef.Placement{bad}, fullTiles(w, h), w, h, errf); err == nil {
			t.Fatalf("a placement at (%d,%d) was accepted on a %dx%d grid", bad.X, bad.Y, w, h)
		}
	}
}
