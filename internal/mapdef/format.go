// Package mapdef is the loader/validator for the map format (maps-as-geometry
// design spec §4.1): a described space of walls, floors, doors, and scenery
// that an LLM DM can reason about directly, rather than a picture it cannot
// see into. Load reads and fully validates one map file, failing loud at
// load time rather than at the table (spec §4.4, matching the adventure
// format's §7 posture) — the same discipline internal/adventure applies to
// an adventure directory, followed here as the sibling pattern to match.
//
// This package resolves a square's tile name only against the STANDARD
// vocabulary (standard.go). Resolving a name against a map's own pack, and
// compiling a loaded Map into wire events, are later tasks: this package
// intentionally has no dependency on contract/gen/go/vtt/v1 or on any pack
// manifest format.
package mapdef

// Map is one fully-loaded, fully-validated map file (spec §4.1's two-layer
// shape). Tiles and Overrides are BOTH keyed "x,y" (column then row; a comma
// rather than a dot because a dot reads as a decimal) — deliberately at the
// same granularity, so each layer can be read independently of the other.
type Map struct {
	ID, Name     string
	GridW, GridH int32

	// Pack names the custom pack Overrides values resolve against (spec
	// §4.2). Load does not read the pack file — resolving a name against it
	// is Task 3 — so Pack is carried through unvalidated; it may legally be
	// empty for a map that uses only standard tiles.
	Pack string

	// Tiles declares the NATURE of every square: what it structurally IS,
	// enforced by the engine (spec §3.2). It must have an entry for every
	// square in GridW x GridH — that completeness is the point of the
	// format, checked by Load before anything else that depends on it.
	Tiles map[string]string

	// Overrides is sparse and optional: it changes a square's PICTURE only,
	// never its nature. Deleting the entire map renders and plays
	// identically in every way that matters (spec §4.1). Values are pack
	// tile names, carried opaque here — resolving one against Pack is
	// Task 3's job, not this package's.
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
