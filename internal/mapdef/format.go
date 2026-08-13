// Package mapdef is the loader/validator for the map format (maps-as-geometry
// design spec §4.1): a described space of walls, floors, doors, and scenery
// that an LLM DM can reason about directly, rather than a picture it cannot
// see into. Load reads and fully validates one map file, failing loud at
// load time rather than at the table (spec §4.4, matching the adventure
// format's §7 posture) — the same discipline internal/adventure applies to
// an adventure directory, followed here as the sibling pattern to match.
//
// A square's own tile name (Map.Tiles) resolves only against the STANDARD
// vocabulary (standard.go) — that never needs a pack. A square's ART, when
// overridden, resolves against its own pack manifest (Pack/PackTile,
// resolve.go's Resolve); this package owns that manifest format directly
// rather than importing one, because a pack is content (design spec §4.2)
// with no engine behaviour riding on it — nothing about Kind/Material ever
// comes from a pack (see Resolve's doc comment for why that boundary is
// load-bearing). Compiling a loaded Map into wire events is still a later
// task: this package intentionally has no dependency on
// contract/gen/go/vtt/v1.
package mapdef

// Map is one fully-loaded, fully-validated map file (spec §4.1's two-layer
// shape). Tiles and Overrides are BOTH keyed "x,y" (column then row; a comma
// rather than a dot because a dot reads as a decimal) — deliberately at the
// same granularity, so each layer can be read independently of the other.
type Map struct {
	ID, Name     string
	GridW, GridH int32

	// Pack names the custom pack Overrides values resolve against (spec
	// §4.2). Load does not read the pack file itself — LoadPack and Resolve
	// (resolve.go) do that, separately, since a map names its pack by ID
	// rather than embedding it — so Pack is carried through unvalidated by
	// Load; it may legally be empty for a map that uses only standard tiles.
	Pack string

	// Tiles declares the NATURE of every square: what it structurally IS,
	// enforced by the engine (spec §3.2). It must have an entry for every
	// square in GridW x GridH — that completeness is the point of the
	// format, checked by Load before anything else that depends on it.
	Tiles map[string]string

	// Overrides is sparse and optional: it changes a square's PICTURE only,
	// never its nature. Deleting the entire map renders and plays
	// identically in every way that matters (spec §4.1). Values are pack
	// tile names, carried opaque by Load — resolving one against a *Pack is
	// Resolve's job (resolve.go), not Load's: Load never takes a pack
	// argument, and per-square resolution needs one.
	Overrides map[string]string

	Objects    []Object
	Placements []Placement
}

// Object is one piece of scenery: it occupies a footprint, may block sight
// or movement, and carries a picture, but it never acts (spec §3.4 — the
// line that keeps this from becoming a second, half-implemented entity
// system: anything that acts or holds state is an actor with a token, which
// the platform already models fully).
type Object struct {
	ID string

	// Kind is an OPEN descriptive label ("boulder", "chest", "table") for a
	// human or an LLM to talk about the object by — the platform never
	// interprets it. This is deliberately the same field name a tile's
	// Kind uses for a CLOSED spatial set (wall/floor/door) that the engine
	// DOES interpret; spec §3.4 calls the name collision intentional and
	// warns a reader not to infer behaviour from this Kind's value. An
	// object's structural effect comes only from BlocksSight/BlocksMove.
	Kind string

	X, Y, W, H, Rotation    int32
	BlocksSight, BlocksMove bool
	Art                     string
}

// Placement is one token placement, declared inline in its owning map. It
// deliberately mirrors internal/adventure.Placement's shape rather than
// importing that package: a map loads standalone (spec §4.3 — "the ability
// to be loaded outside the adventure"), so this package must not depend on
// the adventure package just to name a field shape the two formats happen
// to share.
type Placement struct {
	TokenID, ActorID string
	X, Y             int32
}

// PackTile is one named entry from a pack manifest (spec §4.2) — a tile
// picture or an object picture; the two share this shape because a
// pack.json entry looks identical whichever list it sits in, and neither
// list needs a different one. Kind and Material here are ADVISORY: authoring
// metadata a human or an LLM uses to pick a tile deliberately (spec §1.5),
// carrying no authority over a square's actual nature — Resolve (resolve.go)
// never reads them as fact, only m.Tiles does.
type PackTile struct {
	Name, Kind, Material       string
	File, FileOpen, FileClosed string
	Desc                       string
}

// Pack is one loaded pack manifest (spec §4.2), keyed by tile/object name
// for the O(1) lookup Resolve needs per square. LoadPack never touches a
// Map: a pack is reusable content, not bound to any one map, mirroring spec
// §4.3's "load standalone" principle applied to art rather than geometry.
type Pack struct {
	ID, Name string
	CellPx   int32
	Tiles    map[string]PackTile
	Objects  map[string]PackTile
}
