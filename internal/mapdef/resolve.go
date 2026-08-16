package mapdef

import "fmt"

// Resolved is what one square becomes on the wire (spec §5): the facts the
// engine enforces (Kind, Material) plus the picture name the renderer draws
// (Art). Kind and Material NEVER come from a pack — see Resolve's doc
// comment — so a Resolved value is safe to hand downstream precisely because
// its facts already passed through StandardTile before any pack was ever
// consulted.
type Resolved struct{ Kind, Material, Art string }

// Resolve turns one square into the facts the engine needs plus the art name
// the renderer needs.
//
// NATURE ALWAYS COMES FROM m.Tiles. The override supplies art and nothing
// else (spec §3.2, Patrik: "the ART will never decide the 'nature' of the
// square/item"). A kind mismatch WARNS rather than refusing, because a wall
// that looks like a passage is an illusory wall — legitimate dungeon craft,
// and refusing it would forbid a feature one arc away.
func Resolve(m *Map, p *Pack, square string) (Resolved, []string, error) {
	base, ok := m.Tiles[square]
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s has no tile", square)
	}
	kind, material, ok := StandardTile(base)
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s names unknown tile %q", square, base)
	}
	art, hasArt := m.Overrides[square]
	if !hasArt {
		return Resolved{Kind: kind, Material: material}, nil, nil
	}
	// Load accepts an empty Map.Pack alongside a non-empty Overrides (it has
	// no reason to cross-check the two fields against each other — Pack's
	// own doc comment only promises it MAY be empty), so a caller can reach
	// here with p == nil on a Load-accepted map. Refuse rather than let
	// p.Tiles panic: a nil-pointer crash mid-resolve is a worse failure mode
	// than a fenced error naming the actual problem.
	if p == nil {
		return Resolved{}, nil, fmt.Errorf(
			"mapdef: square %s names art %q but no pack was given to resolve it", square, art)
	}
	// The two resolution levels are the map's OWN pack, then standard (spec
	// §4.2) — not any pack the caller happens to hand in. Without this check
	// a caller that loaded the wrong pack directory would resolve silently
	// against art names that mean nothing on this map, and the mismatch
	// would only surface as wrong pictures at the table.
	if m.Pack != "" && p.ID != m.Pack {
		return Resolved{}, nil, fmt.Errorf(
			"mapdef: square %s's map names pack %q, not the pack %q given to Resolve", square, m.Pack, p.ID)
	}
	pt, ok := p.Tiles[art]
	if !ok {
		return Resolved{}, nil, fmt.Errorf(
			"mapdef: square %s names art %q, which pack %q does not define", square, art, p.ID)
	}
	var warnings []string
	// pt.Kind is advisory and OPTIONAL (packTileMap requires only Name), so an
	// empty pt.Kind means "not declared" rather than "declared as nothing" —
	// treating it as a mismatch would warn on every override drawn from a
	// pack tile whose author never filled in the metadata, which is noise
	// this task exists to keep OUT of the one warning channel that matters.
	if pt.Kind != "" && pt.Kind != kind {
		warnings = append(warnings, fmt.Sprintf(
			"square %s is %s but its art %q was drawn for %s", square, kind, art, pt.Kind))
	}
	return Resolved{Kind: kind, Material: material, Art: art}, warnings, nil
}

// ResolveObjectArt resolves one object's art against pack p, closing spec
// §4.4's "every `art` name resolves" for objects — the pack-dependent half
// CheckObjectArtDeclared (load.go) cannot perform, since Load never takes a
// *Pack (whole-branch-review finding I1: before this function existed,
// p.Objects was loaded by LoadPack and read by nothing in Go — a typo'd
// object art passed every check this package ran and produced an invisible
// barrier, a blocked square nothing ever explained).
//
// This is deliberately its OWN function rather than folded into Resolve
// above, and not a call TO Resolve either: Resolve is keyed by a square
// ("x,y", the one unit spec §4.1's two layers are BOTH keyed by), and an
// object has no square key of its own — its X/Y is an anchor, not an
// identity — so forcing it through Resolve's square-shaped signature would
// misrepresent what an object is. idx identifies the object by its position
// in Map.Objects (the same "objects[N]" addressing load.go's other Check*
// functions already use for this exact reason), since an object's own ID is
// author-supplied and neither required nor guaranteed unique.
//
// An empty o.Art is refused here too, redundantly with
// CheckObjectArtDeclared: BuildSceneCreated (compile.go, which calls this
// per object) is the ONE shared construction site both the standalone-map
// and adventure-embedded load paths call directly, and a caller that builds
// a *Map by hand — as internal/adventure/load.go's own dry run, and several
// tests in this package, both do — can reach here without ever having run
// Load's checks at all.
func ResolveObjectArt(idx int, o Object, p *Pack) error {
	if o.Art == "" {
		return fmt.Errorf("mapdef: objects[%d] has no art (objects have no standard art fallback — only tiles do)", idx)
	}
	// Same reasoning as Resolve's own p == nil guard above: Load accepts an
	// object naming art alongside an empty/absent Pack (LoadPack and Load
	// are independent, and Load has no pack to cross-check against), so a
	// caller can reach here with p == nil for an object that legitimately
	// needs one. Refuse rather than let p.Objects panic.
	if p == nil {
		return fmt.Errorf("mapdef: objects[%d] names art %q but no pack was given to resolve it", idx, o.Art)
	}
	if _, ok := p.Objects[o.Art]; !ok {
		return fmt.Errorf("mapdef: objects[%d] names art %q, which pack %q does not define", idx, o.Art, p.ID)
	}
	return nil
}
