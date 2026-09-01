package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// validTileKinds is the closed spatial set create_scene's tiles.kind must
// come from (design spec §3.3: "kind is a small closed set the platform
// understands spatially: wall, floor, door"; contract/vtt/v1/events.proto's
// TileRef.kind comment says the same). It is derived from mapdef's PUBLIC
// vocabulary (StandardTileNames + StandardTile) rather than restated here as
// a literal {"wall","floor","door"}: mapdef has no exported "is this a valid
// kind" check of its own to call directly — CheckTileNamesKnown validates a
// map-file NAME ("stone-wall") against the standard table, a different shape
// from a wire TileRef, which never carries a name, only an already-resolved
// kind/material pair — so this is the smallest way to track that vocabulary
// without hand-duplicating it as a second literal that could drift from
// standard.go silently (the exact risk §3.3's own amendment note warns
// about). Computed once at package init.
var validTileKinds = func() map[string]bool {
	out := make(map[string]bool, 3)
	for name := range mapdef.StandardTileNames() {
		kind, _, _ := mapdef.StandardTile(name)
		out[kind] = true
	}
	return out
}()

// validateCreateSceneTerrain is whole-branch-review finding C5's fix:
// create_scene was a THIRD terrain entry point (load_map's mapdef.Load and
// the movement check's Blocked() call in handleCommand being the other two)
// carrying NO validation at all — a tile kind of "Wall", "wall " or "banana"
// reached engine state unexamined, and terrain.go's exact-match switch let a
// player stand on all three (proven empirically by the whole-branch review).
// Called from handleCommand (server.go), at the same seam and for the same
// reason the movement check documents there: engine.Apply is the fold, and
// by the time an event reaches it the scene is already history.
//
// What this checks, reusing mapdef's own exported checks rather than
// reimplementing them (they already carry design spec §4.4's logic and stay
// correct if that logic ever changes):
//
//   - kind: every declared tile's kind must be in validTileKinds above.
//     Exact string match, same as terrain.go's own switch — "Wall" and
//     "wall " are refused for the identical reason "banana" is: none of the
//     three is the string terrain.go tests for.
//   - tile keys parse as "x,y" and lie inside the declared grid
//     (mapdef.CheckTilesInsideGrid).
//   - completeness: every grid square must have an entry
//     (mapdef.RequireEverySquarePresent), so terrain is mandatory for any
//     scene that HAS squares. It is not universally mandatory, and the gap is
//     worth knowing rather than glossing: the walk iterates y < h then x < w,
//     so on a degenerate grid — 0x0, 5x0, or either dimension negative — the
//     loops never run and an empty tiles map passes. Nothing in this package
//     or in mapdef validates grid DIMENSIONS at all, which predates this rule
//     and is not fixed by it; recorded here so the next reader does not take
//     the sentence above as a guarantee it cannot make. A map FILE keeps the
//     opt-out — tiles is optional in the file
//     format (Patrik's ruling, 2026-08-13), because a file authored before
//     maps-as-geometry must keep loading — and mapdef.CheckEverySquarePresent
//     is that same walk with that exemption attached. create_scene has no
//     such legacy: nobody has authored a create_scene command in advance, it
//     is the IMPROVISED path by which a place comes into existence during
//     play, and a scene created with no terrain is a featureless grid on
//     which internal/sight has nothing to occlude with, so everyone sees
//     everything. An improvised room does not get to opt out of the rule
//     maps-as-geometry exists to enforce.
//   - objects lie inside the grid, full footprint not just anchor, and are
//     at least 1x1 (mapdef.CheckObjectFootprints).
//
// What this deliberately does NOT check: material. TileRef.material's own
// doc comment calls it "OPAQUE... never the platform's", and design spec
// §3.3 states the reason directly: material is opaque "which is what keeps
// CLAUDE.md rule 5 satisfied: no game vocabulary reaches platform code,
// because the platform only ever sees strings it does not read." A map
// FILE's material is always one of the eleven standard ones only because
// Resolve DERIVES it from a standard tile name (mapdef.Resolve reads
// StandardTile(base), never a caller-supplied material) — the file format
// has no mechanism for a custom material at all, so "reuse
// CheckTileNamesKnown for material too" is not available even in principle:
// that check validates a NAME against the standard table, and create_scene's
// TileRef never carries a name, only an already-resolved kind/material pair.
// Whitelisting material here would invent a rule the file path never had,
// and would put platform code in the business of deciding which
// ruleset-flavour words are legitimate — exactly what rule 5 forbids.
//
// mapdef.RequireEverySquarePresent and mapdef.CheckTilesInsideGrid both take a
// map[string]string (the map-file shape) and read only their KEYS, never
// their values (see each function's own doc comment: presence and range
// checks, nothing else) — so tileKeys below re-keys cmd's
// map[string]*TileRef into that shape without touching either function. That
// is reuse, not a fork: a future change to either function's key logic
// applies here for free.
func validateCreateSceneTerrain(cmd *vttv1.CreateScene) error {
	w, h := cmd.GetGridWidth(), cmd.GetGridHeight()
	tiles := cmd.GetTiles()

	errf := func(field, msg string) error {
		return fmt.Errorf("gateway: create_scene: %s — %s", field, msg)
	}

	tileKeys := make(map[string]string, len(tiles))
	for k := range tiles {
		tileKeys[k] = k
	}
	if err := mapdef.RequireEverySquarePresent(tileKeys, w, h, errf); err != nil {
		return err
	}
	if err := mapdef.CheckTilesInsideGrid(tileKeys, w, h, errf); err != nil {
		return err
	}
	for key, t := range tiles {
		if !validTileKinds[t.GetKind()] {
			return errf(fmt.Sprintf("tiles[%q].kind", key),
				fmt.Sprintf("%q is not wall, floor, or door", t.GetKind()))
		}
	}

	objs := make([]mapdef.Object, 0, len(cmd.GetObjects()))
	for _, o := range cmd.GetObjects() {
		objs = append(objs, mapdef.Object{
			ID: o.GetObjectId(), Kind: o.GetKind(),
			X: o.GetAt().GetX(), Y: o.GetAt().GetY(),
			W: o.GetWidth(), H: o.GetHeight(),
			Rotation:    o.GetRotationDegrees(),
			BlocksSight: o.GetBlocksSight(), BlocksMove: o.GetBlocksMove(),
			Art: o.GetArt(),
		})
	}
	return mapdef.CheckObjectFootprints(objs, w, h, errf)
}
