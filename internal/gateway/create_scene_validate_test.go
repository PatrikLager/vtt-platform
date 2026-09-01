package gateway

import (
	"fmt"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// floorGrid declares every square of a w x h grid as floor — the smallest
// legal terrain a scene can carry now that create_scene refuses an
// undeclared square. Tests below that are about something OTHER than
// completeness (an object's footprint, a stray out-of-grid key) use it so
// their subject is what fails, not the terrain rule standing in front of it.
func floorGrid(w, h int32) map[string]*vttv1.TileRef {
	tiles := make(map[string]*vttv1.TileRef, w*h)
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	return tiles
}

// TestValidateCreateSceneTerrainAcceptsTheClosedKinds proves every kind
// terrain.go actually reads (wall, floor, door — its own exact-match switch,
// internal/engine/terrain.go) is accepted, alone on a 1x1 grid so no other
// check can interfere. A regression that tightened validateCreateSceneTerrain
// past the documented closed set would break a legitimate scene here.
func TestValidateCreateSceneTerrainAcceptsTheClosedKinds(t *testing.T) {
	for _, kind := range []string{"wall", "floor", "door"} {
		t.Run(kind, func(t *testing.T) {
			cmd := &vttv1.CreateScene{
				GridWidth: 1, GridHeight: 1,
				Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: kind}},
			}
			if err := validateCreateSceneTerrain(cmd); err != nil {
				t.Fatalf("kind %q refused: %v", kind, err)
			}
		})
	}
}

// TestValidateCreateSceneTerrainRefusesUnknownKind is the reviewer's own
// three empirical proof cases from the whole-branch review (a bad
// capitalization, trailing whitespace, and an invented kind), plus one more
// — an empty kind, the zero value a caller gets by simply forgetting the
// field — driven directly against the function rather than over the wire,
// so the table can also assert on the exact field named in the error.
func TestValidateCreateSceneTerrainRefusesUnknownKind(t *testing.T) {
	cases := []string{"Wall", "wall ", "banana", ""}
	for _, kind := range cases {
		t.Run(kind, func(t *testing.T) {
			cmd := &vttv1.CreateScene{
				GridWidth: 1, GridHeight: 1,
				Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: kind}},
			}
			err := validateCreateSceneTerrain(cmd)
			if err == nil {
				t.Fatalf("kind %q was accepted — the closed spatial vocabulary is not enforced", kind)
			}
			if !strings.Contains(err.Error(), `tiles["0,0"].kind`) {
				t.Fatalf("error does not name the offending field: %v", err)
			}
			t.Logf("refusal wording: %v", err)
		})
	}
}

// TestValidateCreateSceneTerrainIgnoresMaterial pins the deliberate other
// half of the fix: material is opaque (TileRef.material's own doc comment;
// design spec §3.3's CLAUDE.md-rule-5 reasoning) and must never be checked
// against a whitelist, so ANY string — including one that matches no
// standard tile's material and an outright empty one — passes as long as
// kind is valid.
func TestValidateCreateSceneTerrainIgnoresMaterial(t *testing.T) {
	for _, material := range []string{"stone", "obsidian-blend", "", "BANANA"} {
		t.Run(material, func(t *testing.T) {
			cmd := &vttv1.CreateScene{
				GridWidth: 1, GridHeight: 1,
				Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "wall", Material: material}},
			}
			if err := validateCreateSceneTerrain(cmd); err != nil {
				t.Fatalf("material %q alongside a valid kind was refused: %v", material, err)
			}
		})
	}
}

// TestValidateCreateSceneTerrainRefusesOutOfGridKey is the reviewer's fourth
// proof case: every in-grid square is declared (so completeness cannot be
// what fires) plus one stray key outside the declared grid.
func TestValidateCreateSceneTerrainRefusesOutOfGridKey(t *testing.T) {
	tiles := floorGrid(3, 3)
	tiles["5,5"] = &vttv1.TileRef{Kind: "floor"}

	cmd := &vttv1.CreateScene{GridWidth: 3, GridHeight: 3, Tiles: tiles}
	err := validateCreateSceneTerrain(cmd)
	if err == nil {
		t.Fatal("a tile key outside the declared grid was accepted")
	}
	if !strings.Contains(err.Error(), `"5,5"`) {
		t.Fatalf("error does not name the offending square: %v", err)
	}
	t.Logf("refusal wording: %v", err)
}

// TestValidateCreateSceneTerrainRefusesIncompleteTiles proves the
// completeness rule (spec §4.1: "there is no implicit fallback anywhere")
// applies here too — the SAME rule mapdef owns for a map file, reused
// rather than re-implemented.
func TestValidateCreateSceneTerrainRefusesIncompleteTiles(t *testing.T) {
	cmd := &vttv1.CreateScene{
		GridWidth: 2, GridHeight: 2,
		Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}}, // 3 squares missing
	}
	err := validateCreateSceneTerrain(cmd)
	if err == nil {
		t.Fatal("a partial tiles map (terrain declared, squares missing) was accepted")
	}
	t.Logf("refusal wording: %v", err)
}

// TestValidateCreateSceneTerrainAcceptsEverySquareDeclared is the accepting
// half of the completeness boundary: a room that names all six of its
// squares is a room a DM has actually described, and it must pass. It is the
// control for the refusal below — without it, a validator that refused
// EVERY create_scene would satisfy that test alone.
//
// Six squares rather than one, because a 1x1 grid cannot tell "walks the
// whole grid" from "checks the origin".
func TestValidateCreateSceneTerrainAcceptsEverySquareDeclared(t *testing.T) {
	cmd := &vttv1.CreateScene{GridWidth: 3, GridHeight: 2, Tiles: floorGrid(3, 2)}
	if err := validateCreateSceneTerrain(cmd); err != nil {
		t.Fatalf("a scene declaring all 6 of its squares was refused: %v", err)
	}
}

// TestValidateCreateSceneTerrainRefusesASquareShortOfComplete is the
// refusing half, and the rule this task exists for: create_scene is the
// IMPROVISED path — how a place comes into existence at the table when no
// authored map file exists — and a square nobody declared is a square
// internal/sight cannot reason about. The refusal must NAME a missing square,
// because a DM told only "incomplete" has to find it themselves.
//
// The two rows are the same rule at two scales, and the SMALL one is the
// hole this closes:
//
//   - a 3x3 room missing its far corner was already refused, by
//     mapdef.CheckEverySquarePresent, which create_scene has always called.
//   - a 1x1 room missing its only square is that same claim at the boundary
//     where "one square short" and "no terrain declared at all" become the
//     same fixture — and THAT is where the old rule stopped: it returned
//     early on an empty tiles map, so a featureless grid was accepted as if
//     it had been described. A map FILE keeps that opt-out (an existing file
//     must keep loading, Patrik's ruling 2026-08-13); a create_scene command,
//     which no one has authored yet, does not.
func TestValidateCreateSceneTerrainRefusesASquareShortOfComplete(t *testing.T) {
	cases := []struct {
		name    string
		w, h    int32
		missing string
	}{
		{"a 3x3 room missing its far corner", 3, 3, "2,2"},
		{"a 1x1 room missing its only square", 1, 1, "0,0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tiles := map[string]*vttv1.TileRef{}
			for y := int32(0); y < tc.h; y++ {
				for x := int32(0); x < tc.w; x++ {
					key := fmt.Sprintf("%d,%d", x, y)
					if key == tc.missing {
						continue
					}
					tiles[key] = &vttv1.TileRef{Kind: "floor"}
				}
			}
			cmd := &vttv1.CreateScene{GridWidth: tc.w, GridHeight: tc.h, Tiles: tiles}
			err := validateCreateSceneTerrain(cmd)
			if err == nil {
				t.Fatalf("a %dx%d scene with square %q undeclared was accepted — "+
					"a square nobody declared is terrain the platform cannot see",
					tc.w, tc.h, tc.missing)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("tiles[%q]", tc.missing)) {
				t.Fatalf("the refusal does not name the missing square %q, so a DM "+
					"cannot fix it: %v", tc.missing, err)
			}
			t.Logf("refusal wording: %v", err)
		})
	}
}

// TestValidateCreateSceneTerrainRefusesObjectOutsideGrid covers the fourth
// bullet: objects lie inside the grid (mapdef.CheckObjectFootprints, reused
// for its full-footprint bounds check, not merely the anchor).
func TestValidateCreateSceneTerrainRefusesObjectOutsideGrid(t *testing.T) {
	cmd := &vttv1.CreateScene{
		GridWidth: 2, GridHeight: 2, Tiles: floorGrid(2, 2),
		Objects: []*vttv1.SceneObject{
			{ObjectId: "o1", Kind: "boulder", At: &vttv1.GridPosition{X: 5, Y: 5}, Width: 1, Height: 1},
		},
	}
	err := validateCreateSceneTerrain(cmd)
	if err == nil {
		t.Fatal("an object anchored outside the grid was accepted")
	}
	t.Logf("refusal wording: %v", err)
}

// TestValidateCreateSceneTerrainAcceptsAnObjectInsideTheGrid is the positive
// control for the object check above.
func TestValidateCreateSceneTerrainAcceptsAnObjectInsideTheGrid(t *testing.T) {
	cmd := &vttv1.CreateScene{
		GridWidth: 3, GridHeight: 3, Tiles: floorGrid(3, 3),
		Objects: []*vttv1.SceneObject{
			{ObjectId: "o1", Kind: "boulder", At: &vttv1.GridPosition{X: 1, Y: 1}, Width: 1, Height: 1},
		},
	}
	if err := validateCreateSceneTerrain(cmd); err != nil {
		t.Fatalf("an object inside the grid was refused: %v", err)
	}
}
