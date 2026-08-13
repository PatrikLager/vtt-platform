package mapdef_test

import (
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
