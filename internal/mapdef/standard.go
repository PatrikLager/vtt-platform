package mapdef

// standardTiles is the published vocabulary of NATURES. A custom pack adds
// pictures; it never adds natures (spec §3.3). A door is ONE nature — whether
// it is open is folded state, never part of the name.
var standardTiles = map[string]struct{ Kind, Material string }{
	"stone-wall": {"wall", "stone"},
	"wood-wall":  {"wall", "wood"},
	"wood-door":  {"door", "wood"},
	"stone":      {"floor", "stone"},
	"wood":       {"floor", "wood"},
	"earth":      {"floor", "earth"},
	"grass":      {"floor", "grass"},
	"sand":       {"floor", "sand"},
	"water":      {"floor", "water"},
	"metal":      {"floor", "metal"},
	"ice":        {"floor", "ice"},
}

// StandardTile resolves name against the standard vocabulary. kind is the
// closed spatial set the engine reads ("wall"/"floor"/"door"); material is
// an opaque tag this package never interprets — CLAUDE.md rule 5 forbids
// platform code from meaning anything about what "water" or "stone" DO,
// which is exactly the line StandardTile's callers must not cross.
func StandardTile(name string) (kind, material string, ok bool) {
	t, ok := standardTiles[name]
	return t.Kind, t.Material, ok
}

// StandardTileNames returns every name in the standard vocabulary (spec
// §3.3's table), so a test can walk it without duplicating the eleven names
// as a second literal that would silently drift from standardTiles itself —
// exactly the risk §3.3's own amendment note describes ("a one-way door
// should not be specified in a form where an entry can go missing without
// anyone noticing"). A copy, not standardTiles itself: the caller gets a map
// it cannot use to mutate the package's own vocabulary.
func StandardTileNames() map[string]struct{} {
	out := make(map[string]struct{}, len(standardTiles))
	for name := range standardTiles {
		out[name] = struct{}{}
	}
	return out
}
