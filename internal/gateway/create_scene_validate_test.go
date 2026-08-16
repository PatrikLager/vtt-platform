package gateway

import (
	"fmt"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

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
	tiles := map[string]*vttv1.TileRef{}
	for y := int32(0); y < 3; y++ {
		for x := int32(0); x < 3; x++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
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
// applies here too, once tiles is non-empty — the SAME rule
// mapdef.CheckEverySquarePresent enforces on a map file, reused rather than
// re-implemented.
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

// TestValidateCreateSceneTerrainAllowsNoTilesAtAll pins Patrik's ruling
// (2026-08-13): tiles stays OPTIONAL. A scene with no terrain at all is a
// bare grid, exactly as every scene was before maps-as-geometry, and this
// fix must not break that.
func TestValidateCreateSceneTerrainAllowsNoTilesAtAll(t *testing.T) {
	cmd := &vttv1.CreateScene{GridWidth: 5, GridHeight: 5}
	if err := validateCreateSceneTerrain(cmd); err != nil {
		t.Fatalf("a scene declaring no terrain at all was refused: %v", err)
	}
}

// TestValidateCreateSceneTerrainRefusesObjectOutsideGrid covers the fourth
// bullet: objects lie inside the grid (mapdef.CheckObjectFootprints, reused
// for its full-footprint bounds check, not merely the anchor).
func TestValidateCreateSceneTerrainRefusesObjectOutsideGrid(t *testing.T) {
	cmd := &vttv1.CreateScene{
		GridWidth: 2, GridHeight: 2,
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
		GridWidth: 3, GridHeight: 3,
		Objects: []*vttv1.SceneObject{
			{ObjectId: "o1", Kind: "boulder", At: &vttv1.GridPosition{X: 1, Y: 1}, Width: 1, Height: 1},
		},
	}
	if err := validateCreateSceneTerrain(cmd); err != nil {
		t.Fatalf("an object inside the grid was refused: %v", err)
	}
}
