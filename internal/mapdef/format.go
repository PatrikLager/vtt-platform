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
// mapJSON keeps the JSON one solely so the refusal can be worded. Pack,
// PackTile and LoadPack DO still live in this package, unreachable from any
// map: nothing resolves against them, and Task 7 of that plan deletes them.
//
// Compiling a loaded Map into wire events is compile.go's job
// (Task 4, spec §5) — the one and only reason this package depends on
// contract/gen/go/vtt/v1 at all; nothing in format.go, load.go, standard.go,
// or resolve.go touches it.
package mapdef

// MapFormatVersion is the map format this server understands. A map declares
// its own, and a mismatch is refused by name rather than guessed at — see
// LoadPack's PackFormatVersion for why the two version independently.
const MapFormatVersion int32 = 1

// Map is one fully-loaded, fully-validated map file (spec §4.1's two-layer
// shape). Tiles and Overrides are BOTH keyed "x,y" (column then row; a comma
// rather than a dot because a dot reads as a decimal) — deliberately at the
// same granularity, so each layer can be read independently of the other.
type Map struct {
	// FormatVersion is the format this map file declares itself written in
	// (design spec §7, "Format versions, on maps and on packs separately").
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

// PackTile is one named entry from a pack manifest (spec §4.2) — a tile
// picture or an object picture; the two share this shape because a
// pack.json entry looks identical whichever list it sits in, and neither
// list needs a different one. Kind and Material here are ADVISORY: authoring
// metadata a human or an LLM uses to pick a tile deliberately (spec §1.5),
// carrying no authority over a square's actual nature. RESOLVE NO LONGER READS
// A PackTile AT ALL — it reads a sidecar through internal/artlib
// (2026-09-02-art-is-a-flat-library Task 3), where the same advisory/authority
// split is stated on Resolve itself; this said "Resolve never reads them as
// fact, only m.Tiles does" until Task 5 of that plan, which is a true sentence
// about a call that stopped happening.
type PackTile struct {
	Name, Kind, Material       string
	File, FileOpen, FileClosed string
	Desc                       string
}

// PackFormatVersion is the pack format this server understands, and it
// moves INDEPENDENTLY of MapFormatVersion. A pack is shared (design spec §7,
// "Format versions, on maps and on packs separately") — one tileset backs
// many maps — so a single shared version number would force every pack on
// disk to be rewritten the first time the map format moved. LoadPack
// (load.go) refuses a pack that omits format_version or names one this
// server does not understand, mirroring Load's own two-step refusal for
// maps.
const PackFormatVersion int32 = 1

// Pack is one loaded pack manifest (spec §4.2), keyed by tile/object name.
// THAT KEYING NOW SERVES NOBODY: it existed for the O(1) lookup Resolve made
// per square, and Resolve stopped taking a *Pack at
// 2026-09-02-art-is-a-flat-library Task 3. LoadPack never touched a Map even
// then — a pack is reusable content, not bound to any one map, mirroring spec
// §4.3's "load standalone" principle applied to art rather than geometry — and
// since Task 5 of that plan no map can name one at all. Task 7 deletes this
// type; until then LoadPack is still run over a campaign's packs/ at boot, so
// a malformed pack.json is refused rather than silently ignored mid-removal.
type Pack struct {
	// FormatVersion is the format this pack manifest declares itself
	// written in (design spec §7, "Format versions, on maps and on packs
	// separately"). LoadPack refuses a file that omits it or names one this
	// server does not understand — see LoadPack's own checks immediately
	// after decodeStrict — so by the time a *Pack exists, FormatVersion is
	// always PackFormatVersion; it is carried through anyway so a caller
	// can name the fact rather than assume it.
	FormatVersion int32

	ID, Name string
	CellPx   int32
	Tiles    map[string]PackTile
	Objects  map[string]PackTile
}
