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
