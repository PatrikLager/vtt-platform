package engine

import "fmt"

// Blocked reports whether a square cannot be entered, and why.
//
// SPATIAL ONLY, deliberately (CLAUDE.md rule 5; maps-as-geometry spec §6).
// "A token cannot stand inside solid rock" is the same kind of fact as
// "inside the grid" — it is not a game-system rule, so this stays untouched
// by ruleset concerns. Difficult terrain, flying and phasing are the
// ruleset's business and must never appear here; Material is opaque and is
// never branched on. The gateway is the one place this gets enforced
// (Task 6) — this method only answers the question.
//
// An unknown scene blocks. Fail closed: refusing a move into a scene this
// state cannot describe is recoverable (retry once the scene exists);
// silently permitting one is not — nothing would ever have checked it.
func (st *State) Blocked(sceneID string, x, y int32) (bool, string) {
	sc, ok := st.Scenes[sceneID]
	if !ok {
		return true, fmt.Sprintf("unknown scene %q", sceneID)
	}
	if x < 0 || y < 0 || x >= sc.GridWidth || y >= sc.GridHeight {
		return true, "outside the grid"
	}
	key := gridKey(x, y)
	switch t := sc.Tiles[key]; t.Kind {
	case "wall":
		return true, "a wall"
	case "door":
		// A key absent from OpenDoors means the door was never toggled,
		// which the SceneCreated arm (apply.go) and DoorOpened/DoorClosed's
		// delete-on-close both treat as CLOSED — the fail-closed direction.
		// A scene with no Tiles recorded never reaches this case at all:
		// the zero-value Tile{} has Kind "", which matches neither switch
		// arm, so a terrain-free scene is never blocked by terrain (Patrik's
		// ruling 2026-08-13 — no special case needed).
		if !sc.OpenDoors[key] {
			return true, "a closed door"
		}
	}
	for _, o := range sc.Objects {
		if o.BlocksMove && covers(o, x, y) {
			return true, "scenery: " + o.Kind
		}
	}
	return false, ""
}

// gridKey formats a grid square the way the wire format does: column then
// row, comma-separated (maps-as-geometry spec §4.1) — SceneCreated's Tiles
// map and OpenDoors are both keyed this way, so every lookup and mutation
// against either has to agree on this one function rather than restating the
// format at each call site.
func gridKey(x, y int32) string {
	return fmt.Sprintf("%d,%d", x, y)
}

// covers reports whether object o's footprint includes square (x, y).
// Anchored at o's top-left corner (X, Y), Width columns by Height rows.
//
// RotationDegrees is stored on SceneObject but deliberately not consulted
// here: no spec or test in this arc defines how rotation reshapes a
// footprint, and inventing that transform without one driving it would be
// exactly the untested behaviour ADR-009 exists to prevent. mapdef's loader
// already rejects a sub-1x1 footprint (internal/mapdef/load.go), so Width
// and Height arriving here are trusted to be at least 1 — this function does
// not re-clamp them, matching Blocked's own trust of GridWidth/GridHeight.
func covers(o SceneObject, x, y int32) bool {
	return x >= o.X && x < o.X+o.Width && y >= o.Y && y < o.Y+o.Height
}
