// Package mapdef is the loader/validator for the map format (maps-as-geometry
// design spec §4.1): a described space of walls, floors, doors, and scenery
// that an LLM DM can reason about directly, rather than a picture it cannot
// see into. Load reads and fully validates one map file, failing loud at
// load time rather than at the table (spec §4.4, matching the adventure
// format's §7 posture) — the same discipline internal/adventure applies to
// an adventure directory, followed here as the sibling pattern to match.
//
// A square's own tile name (Map.Tiles) resolves only against the STANDARD
// vocabulary (standard.go) — that needs nothing else. A square's ART, when
// overridden, resolves by FILENAME inside the campaign's one flat art/
// directory, through internal/artlib (resolve.go's Resolve, and
// 2026-09-02-art-is-a-flat-library design spec §3.2). Nothing about
// Kind/Material ever comes from art — see Resolve's doc comment for why that
// boundary is load-bearing.
//
// A MAP THAT DECLARES A PACK IS REFUSED, by Load, naming the field and
// pointing at art/ (that plan's design spec §7: "There is no compatibility
// layer, and none is added later"). Map has no Pack field at all any more —
// mapJSON keeps the JSON one solely so the refusal can be worded — and since
// Task 7 of that plan there is no Pack, PackTile or LoadPack in this package
// either. Nothing here reads a manifest of any kind: the only art code in the
// tree is internal/artlib, and it looks a piece up by filename.
//
// Compiling a loaded Map into wire events is compile.go's job
// (Task 4, spec §5) — the one and only reason this package depends on
// contract/gen/go/vtt/v1 at all; nothing in format.go, load.go, standard.go,
// or resolve.go touches it.
package mapdef

// MapFormatVersion is the map format this server understands. A map declares
// its own, and a mismatch is refused by name rather than guessed at.
//
// It used to be one of two independent version numbers, the other being
// PackFormatVersion — a pack was shared by many maps, so a single shared
// number would have forced every pack on disk to be rewritten the first time
// the map format moved. Packs left at 2026-09-02-art-is-a-flat-library Task 7.
// The successor split lives in internal/artlib, whose sidecars carry their own
// format_version for the same reason: one picture is named by many maps.
const MapFormatVersion int32 = 1

// Map is one fully-loaded, fully-validated map file (spec §4.1's two-layer
// shape). Tiles and Overrides are BOTH keyed "x,y" (column then row; a comma
// rather than a dot because a dot reads as a decimal) — deliberately at the
// same granularity, so each layer can be read independently of the other.
type Map struct {
	// FormatVersion is the format this map file declares itself written in
	// (design spec §7, "Format versions, on maps and on packs separately" —
	// the pack half of that sentence left at art-is-a-flat-library Task 7; an
	// art sidecar carries its own version now, see internal/artlib).
	// Load refuses a file that omits it or names one this server does not
	// understand — see Load's own checks immediately after decodeStrict —
	// so by the time a *Map exists, FormatVersion is always
	// MapFormatVersion; it is carried through anyway so a caller can name
	// the fact rather than assume it.
	FormatVersion int32

	ID, Name     string
	GridW, GridH int32

	// Tiles declares the NATURE of every square: what it structurally IS,
	// enforced by the engine (spec §3.2).
	//
	// OPTIONAL, and empty is legal (Patrik's ruling, 2026-08-13): a scene
	// that declares no tiles has no terrain, exactly as every scene did
	// before maps-as-geometry, and still loads. Making it mandatory broke
	// the on-disk format for anything authored earlier, which a format meant
	// to be written by third parties and by an LLM does not get to do.
	//
	// IF DECLARED, it must hold an entry for every square in GridW x GridH —
	// that completeness is the point of the format, and Load checks it
	// before anything else that depends on it. So the invariant is not
	// weakened, it applies where terrain is claimed. Do NOT assume
	// len(Tiles) == GridW*GridH: assert on it and you reintroduce the
	// breakage the ruling removed.
	Tiles map[string]string

	// Overrides is sparse and optional: it changes a square's PICTURE only,
	// never its nature. Deleting the entire map renders and plays
	// identically in every way that matters (spec §4.1). Values are ART IDS
	// — one kebab-case filename stem in the campaign's art/ — carried opaque
	// by Load: resolving one is Resolve's job (resolve.go), not Load's, since
	// Load never takes an art directory and per-square resolution needs one.
	// A value that resolves to nothing costs its square's picture and one
	// warning, never the map.
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
