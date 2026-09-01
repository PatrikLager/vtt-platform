package mapdef_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

func TestLoadsAMapWhereEverySquareNamesItsTile(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Tiles["0,0"] != "stone-wall" {
		t.Fatalf("square 0,0 is %q, want stone-wall", m.Tiles["0,0"])
	}
	// The override changes the PICTURE only; the square is still what tiles says.
	if m.Tiles["1,1"] != "wood" || m.Overrides["1,1"] != "planks-split-3" {
		t.Fatalf("override did not stay separate from nature: %v / %v",
			m.Tiles["1,1"], m.Overrides["1,1"])
	}
}

// TestLoadsAMapWithNoTilesAsHavingNoTerrain pins Patrik's ruling
// (2026-08-13): a map declaring no "tiles" key at all has no terrain —
// exactly what existed before maps-as-geometry — and that must stay legal
// forever, since this format is written by third parties and by an LLM,
// and an existing file must keep loading. This is deliberately NOT the same
// thing as a map with an incomplete tiles map (that stays an error — see
// TestInvalidMapsAreRefusedWithAUsefulReason's "missing-square" case, a
// genuinely PARTIAL map, 8 of 9 squares present).
func TestLoadsAMapWithNoTilesAsHavingNoTerrain(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/no-terrain.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Tiles) != 0 {
		t.Fatalf("Tiles = %v, want empty (no terrain declared)", m.Tiles)
	}
}

// TestObjectFieldsSurviveTheJSONToMapShapeConversion pins
// checkObjectsInsideGrid's At/Size -> X/Y/W/H split field-by-field. Without
// this, TestLoadsAMapWhereEverySquareNamesItsTile never looks at m.Objects
// at all, so a transposed X/Y or W/H in that conversion would pass every
// other test in this file silently (confirmed at code review by swapping
// the assignment and re-running: see this package's task-2 report for the
// fault-injection record).
func TestObjectFieldsSurviveTheJSONToMapShapeConversion(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Objects) != 1 {
		t.Fatalf("Objects = %+v, want exactly 1", m.Objects)
	}
	want := mapdef.Object{
		ID: "boulder-1", Kind: "boulder",
		X: 0, Y: 1, W: 1, H: 1, Rotation: 0,
		BlocksSight: true, BlocksMove: true,
		Art: "boulder-mossy-2",
	}
	if got := m.Objects[0]; got != want {
		t.Fatalf("Objects[0] = %+v, want %+v", got, want)
	}
}

// Every refusal in spec §4.4 gets a case. Table-driven over fixture dirs,
// following internal/rules/testdata/invalid-v2/'s pattern.
func TestInvalidMapsAreRefusedWithAUsefulReason(t *testing.T) {
	for _, c := range []struct{ dir, want string }{
		{"missing-square", "no tile"},
		{"unknown-tile-name", "unknown tile"},
		{"override-outside-grid", "outside the grid"},
		{"object-outside-grid", "outside the grid"},
		{"token-inside-wall", "inside a wall"},
		{"zero-grid", "must be >= 1"},
		// Not part of task-2-brief.md's enumerated six — added at code
		// review, pre-commit. Both close a real gap rather than merely add
		// belt-and-suspenders coverage:
		//   - tile-outside-grid pins spec §4.4's OTHER tiles rule ("no entry
		//     lies outside the grid"), which checkEverySquarePresent alone
		//     cannot catch: it only proves every REQUIRED square is present,
		//     never that Tiles has no EXTRA square beyond the grid.
		//   - object-non-positive-size pins that an object's footprint must
		//     be at least 1x1: without this, size:[0,0] (or a negative size)
		//     made the bounds check `at+size > grid` trivially true even for
		//     an anchor sitting outside the grid.
		{"tile-outside-grid", "outside the grid"},
		{"object-non-positive-size", "at least 1x1"},
		// Also added at review: grid_height's own zero check shares its
		// code with grid_width's (task-2-brief.md's "zero-grid" only
		// exercises the WIDTH half), and a placement's own bounds check
		// (checkPlacementsNotInWalls) was previously reachable only via a
		// wall lookup that happened to also be in-grid — nothing had ever
		// driven a placement whose x/y is not a square at all.
		{"zero-grid-height", "must be >= 1"},
		{"placement-outside-grid", "outside the grid"},
		// Not one of §4.4's validation rules -- pins the strict-decode
		// clause Load's own doc comment promises ("no unknown fields
		// tolerated"), mirroring internal/adventure/testdata/invalid/
		// unknown-field's identical role for that sibling loader.
		{"unknown-field", "bogus_field"},
		// Patrik's ruling (2026-08-13): tiles is optional, but overrides
		// with no tiles at all is incoherent -- an override names art for
		// a square whose NATURE is declared in tiles, so there is nothing
		// to attach the art to. The exact phrase below (not just
		// "overrides") is deliberate: the fixture directory name itself
		// contains "overrides", so a looser substring would pass even
		// against today's unrelated "tiles[...]: no tile named" error --
		// caught exactly this way in review before this test was trusted.
		{"overrides-without-tiles", "declares overrides but tiles is empty"},
		// Whole-branch-review finding I1: object art was resolved nowhere,
		// so a pack typo produced an invisible barrier (a blocks_move
		// object whose picture never draws, with nothing telling a player
		// why). Empty art is the sharper half of that gap to close at Load
		// itself (no *Pack needed to know it): unlike a tile, an object has
		// no standard-vocabulary fallback (tools/genmappack/std_pack.go's
		// own test says objects have none), so empty art can never draw at
		// ANY pack. A non-empty but unresolvable name is the OTHER half --
		// that needs a real *Pack to disprove, so it is pinned in
		// compile_test.go instead (ResolveObjectArt, resolve.go), not here.
		{"object-art-empty", "must not be empty"},
	} {
		t.Run(c.dir, func(t *testing.T) {
			_, err := mapdef.Load(filepath.Join("testdata/invalid", c.dir, "map.json"))
			if err == nil {
				t.Fatal("this map was accepted; every square must be accounted for")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error was %q, want it to mention %q", err, c.want)
			}
		})
	}
}

// TestLoadRejectsMissingFile pins Load's base file-existence guard (not
// part of the §4.4 validation catalogue -- it never reaches JSON decoding
// at all), mirroring internal/adventure's TestLoadRejectsMissingDirectory.
func TestLoadRejectsMissingFile(t *testing.T) {
	if _, err := mapdef.Load("testdata/does-not-exist.json"); err == nil {
		t.Fatal("want an error for a missing map file")
	}
}

// writeFile writes body to path for a test, failing loudly on any write
// error rather than leaving a subsequent Load call to explain a missing
// file it never expected.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestLoadRefusesAMapWithNoFormatVersion pins design spec §7's rule: a map
// declares the format it is written in, and this server refuses to guess
// when it doesn't. No implicit fallback — a missing format_version is
// refused, never assumed to be 1, even though 1 is currently the only
// version that exists.
func TestLoadRefusesAMapWithNoFormatVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "no-version.json")
	writeFile(t, p, `{"id":"x","name":"X","grid_width":1,"grid_height":1,
		"tiles":{"0,0":"floor"}}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("want a map with no format_version refused: an undeclared format is " +
			"undeclared, and this platform does not default one")
	}
	if !strings.Contains(err.Error(), "format_version") {
		t.Fatalf("error = %q, want it to name format_version", err)
	}
}

// TestLoadRefusesAFormatThisServerDoesNotUnderstand pins the OTHER half of
// the same rule: a DECLARED format this server does not understand is
// refused by name, not guessed at or silently accepted.
//
// Two things this test must get right that a first draft got wrong: the
// fixture's tile name must be one CheckTileNamesKnown actually accepts (see
// the "stone" comment below), or Load fails for an unrelated reason before
// ever reaching the version check; and the assertion must not be satisfiable
// by t.TempDir()'s own path, which is large random digits — a bare
// strings.Contains(err, "2") / "1" passes on path noise alone even with
// Load's version-mismatch branch deleted entirely. Asserting on the worded
// phrases below closes both gaps.
func TestLoadRefusesAFormatThisServerDoesNotUnderstand(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "future.json")
	// "stone" is a real name in standardTiles (standard.go): an invalid
	// tile name here would make CheckTileNamesKnown fail FIRST, so err !=
	// nil for the wrong reason and this test would falsely appear to
	// discriminate on the format-version check it exists to pin.
	writeFile(t, p, `{"format_version":2,"id":"x","name":"X","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"}}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("want format 2 refused while this server understands only 1")
	}
	// Assert on the WORDED phrases, not bare digits: t.TempDir()'s own path
	// contains large random integers, so a bare "2"/"1" substring check
	// would pass even with the version check deleted entirely.
	for _, want := range []string{"declares 2", "understands 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

// TestLoadAcceptsTheVersionItUnderstands pins the positive case: a map
// declaring format_version 1 (the version every other fixture in this
// package now carries) loads, and Load reports that version back on Map.
func TestLoadAcceptsTheVersionItUnderstands(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.FormatVersion != mapdef.MapFormatVersion {
		t.Fatalf("FormatVersion = %d, want %d", m.FormatVersion, mapdef.MapFormatVersion)
	}
}
