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
