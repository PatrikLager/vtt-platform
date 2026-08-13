package mapdef

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// mapJSON is the on-disk shape of a map file (design spec §4.1). Field names
// match the spec's JSON examples exactly; Go-side validation and the
// friendlier Map/Object/Placement shapes live in format.go and below.
type mapJSON struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	GridWidth  int32             `json:"grid_width"`
	GridHeight int32             `json:"grid_height"`
	Pack       string            `json:"pack"`
	Tiles      map[string]string `json:"tiles"`
	Overrides  map[string]string `json:"overrides"`
	Objects    []ObjectJSON      `json:"objects"`
	Placements []placementJSON   `json:"placements"`
}

// ObjectJSON is the on-disk shape of one object entry (spec §4.1): an anchor
// (At) and footprint (Size) pair rather than four separate fields. Exported
// — rather than kept private like placementJSON below — because a second
// format that embeds this exact two-layer shape (internal/adventure's
// scenes, spec §4.3: "an adventure still carries its own maps") decodes
// straight into it and calls ToObject/CheckObjectFootprints instead of
// re-implementing the At/Size wire encoding and its overflow-safe bounds
// arithmetic a second time.
type ObjectJSON struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	At          [2]int32 `json:"at"`
	Size        [2]int32 `json:"size"`
	Rot         int32    `json:"rot"`
	BlocksSight bool     `json:"blocks_sight"`
	BlocksMove  bool     `json:"blocks_move"`
	Art         string   `json:"art"`
}

// ToObject converts the wire At/Size shape into Object's X/Y/W/H fields. Pure
// renaming, no validation — CheckObjectFootprints (below) is what a caller
// runs against the result before trusting it.
func (o ObjectJSON) ToObject() Object {
	return Object{
		ID: o.ID, Kind: o.Kind,
		X: o.At[0], Y: o.At[1], W: o.Size[0], H: o.Size[1],
		Rotation:    o.Rot,
		BlocksSight: o.BlocksSight, BlocksMove: o.BlocksMove,
		Art: o.Art,
	}
}

type placementJSON struct {
	TokenID string `json:"token_id"`
	ActorID string `json:"actor_id"`
	X       int32  `json:"x"`
	Y       int32  `json:"y"`
}

// FieldErrFunc builds one load-time validation error naming the offending
// field and a human-readable reason. The Check* functions below take one
// rather than building errors themselves, so Load's own "mapdef: <path>:
// field %q: msg" shape and a second loader's own shape (internal/adventure's
// loadScenes uses "adventure: <path>: field %q: msg") can both drive the
// identical validation logic without either package's errors carrying the
// other's name.
type FieldErrFunc func(field, msg string) error

// Load reads and fully validates the map file at path: strict JSON decoding
// (no unknown fields tolerated, matching internal/adventure/load.go's
// decodeStrict), then every check spec §4.4 requires that this package can
// perform without a pack manifest (art and pack-tile-name resolution are
// Task 3). Every error names the offending file and field; Load returns
// (nil, err) as soon as the first violation is found, matching adventure's
// fail-loud-at-load posture (spec §7).
//
// Checks run in an order chosen so the FIRST error a broken file produces is
// the most useful one to fix: grid sanity gates everything else (there is no
// point naming a square outside a grid whose own size is nonsense); then
// Tiles SHAPE — every required square present, no extra square outside the
// grid — because that is the format's whole point and is more fundamental
// than what a present square NAMES, which is Tiles validity, checked next;
// then Overrides and Objects, which only need a sane grid and nothing from
// the checks after them; and finally Placements, which is the one check
// that reads a square's already-validated tile name — it must run last
// because it depends on Tiles having already been proven complete, bounded,
// and valid.
//
// Load is now a thin path-decoding wrapper around the exported Check*
// functions below (Task 4, maps-as-geometry): they carry the actual
// completeness/bounds/name logic so internal/adventure's loadScenes can
// apply the exact same checks to an embedded scene without re-implementing
// any of them.
func Load(path string) (*Map, error) {
	var raw mapJSON
	if err := decodeStrict(path, &raw); err != nil {
		return nil, err
	}

	if raw.GridWidth < 1 {
		return nil, fieldErr(path, "grid_width", fmt.Sprintf("must be >= 1, got %d", raw.GridWidth))
	}
	if raw.GridHeight < 1 {
		return nil, fieldErr(path, "grid_height", fmt.Sprintf("must be >= 1, got %d", raw.GridHeight))
	}

	errf := func(field, msg string) error { return fieldErr(path, field, msg) }

	if err := CheckEverySquarePresent(raw.Tiles, raw.GridWidth, raw.GridHeight, errf); err != nil {
		return nil, err
	}
	if err := CheckTilesInsideGrid(raw.Tiles, raw.GridWidth, raw.GridHeight, errf); err != nil {
		return nil, err
	}
	if err := CheckTileNamesKnown(raw.Tiles, errf); err != nil {
		return nil, err
	}
	if err := CheckOverridesInsideGrid(raw.Overrides, raw.GridWidth, raw.GridHeight, errf); err != nil {
		return nil, err
	}
	if err := CheckOverridesRequireTiles(raw.Tiles, raw.Overrides, errf); err != nil {
		return nil, err
	}

	objects := make([]Object, 0, len(raw.Objects))
	for _, o := range raw.Objects {
		objects = append(objects, o.ToObject())
	}
	if err := CheckObjectFootprints(objects, raw.GridWidth, raw.GridHeight, errf); err != nil {
		return nil, err
	}

	placements := make([]Placement, 0, len(raw.Placements))
	for _, p := range raw.Placements {
		placements = append(placements, Placement(p))
	}
	if err := CheckPlacementsNotInWalls(placements, raw.Tiles, raw.GridWidth, raw.GridHeight, errf); err != nil {
		return nil, err
	}

	return &Map{
		ID:         raw.ID,
		Name:       raw.Name,
		GridW:      raw.GridWidth,
		GridH:      raw.GridHeight,
		Pack:       raw.Pack,
		Tiles:      raw.Tiles,
		Overrides:  raw.Overrides,
		Objects:    objects,
		Placements: placements,
	}, nil
}

// squareKey formats a square's "x,y" key exactly as Map.Tiles and
// Map.Overrides key theirs — column then row, comma separator (format.go's
// Map doc comment explains why a comma: a dot reads as a decimal).
func squareKey(x, y int32) string {
	return fmt.Sprintf("%d,%d", x, y)
}

// CheckEverySquarePresent enforces the completeness rule that is the whole
// point of the keyed format (spec §4.1: "there is no implicit fallback
// anywhere. Every square names its own tile.") — but ONLY once a map has
// opted into declaring terrain at all. tiles is OPTIONAL (Patrik's ruling,
// 2026-08-13): a map with no "tiles" key has no terrain, exactly as every
// map did before this format existed, and that must stay legal forever —
// this format is written by third parties and by an LLM, and an existing
// file must keep loading. An EMPTY tiles map and one with SOME entries are
// therefore different claims: empty means "no terrain, nothing to check";
// any entries at all means the map has committed to declaring terrain, and
// from that point on completeness is not negotiable — a partial tiles map
// is still an error, with the exact same message this function has always
// produced.
//
// It walks the GRID, not the tiles map, because completeness is a property
// of what is MISSING — a map iteration only ever sees what is present.
// Callers run this before CheckTileNamesKnown so a file that is both
// incomplete and has an invalid name reports the more fundamental defect (a
// square with no answer at all) first.
func CheckEverySquarePresent(tiles map[string]string, w, h int32, errf FieldErrFunc) error {
	if len(tiles) == 0 {
		return nil
	}
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			key := squareKey(x, y)
			if _, ok := tiles[key]; !ok {
				return errf(fmt.Sprintf("tiles[%q]", key), "no tile named for this square")
			}
		}
	}
	return nil
}

// CheckOverridesRequireTiles pins the one coherence rule tiles-as-optional
// creates (Patrik's ruling, 2026-08-13): an overrides entry names the ART
// for a square whose NATURE is declared in tiles (spec §4.1's two-layer
// shape — nature always comes from tiles, never the override, per Resolve's
// own doc comment). A non-empty overrides map with an EMPTY tiles map has
// nothing to attach its art to, so it is refused rather than silently
// accepted or silently dropped. A non-empty overrides map against a
// non-empty (and therefore, by CheckEverySquarePresent above, COMPLETE)
// tiles map needs no further check here — every override key is already
// proven in-bounds by CheckOverridesInsideGrid, and a complete tiles map
// covers every in-bounds square by construction.
func CheckOverridesRequireTiles(tiles, overrides map[string]string, errf FieldErrFunc) error {
	if len(overrides) > 0 && len(tiles) == 0 {
		return errf("overrides", "declares overrides but tiles is empty — "+
			"an override names art for a square whose nature is declared in "+
			"tiles, so there is nothing to attach it to")
	}
	return nil
}

// CheckTilesInsideGrid is CheckEverySquarePresent's mirror image: that
// function proves every REQUIRED square has an entry; this one proves tiles
// has no EXTRA entry outside the grid (spec §4.4: "no entry lies outside the
// grid"). The two are independent facts — a map can be complete AND carry a
// stray out-of-grid key at the same time — so a single loop cannot stand in
// for both, and this is deliberately its own function rather than folded
// into CheckEverySquarePresent: doing so would mean deleting ONE function to
// fault-inject BOTH rules, leaving the completeness rule's own fault
// injection unable to prove it in isolation.
func CheckTilesInsideGrid(tiles map[string]string, w, h int32, errf FieldErrFunc) error {
	for key := range tiles {
		x, y, ok := parseSquareKey(key)
		if !ok || x < 0 || x >= w || y < 0 || y >= h {
			return errf(fmt.Sprintf("tiles[%q]", key), "names a square outside the grid")
		}
	}
	return nil
}

// CheckTileNamesKnown validates every tiles VALUE against the standard
// vocabulary. Callers run this after CheckEverySquarePresent and
// CheckTilesInsideGrid have proven every grid square has an entry and no
// entry lies outside the grid, so walking the tiles map here (rather than
// the grid again) covers exactly the same squares. Overrides values
// (pack-declared art names) are NOT checked here — that needs a *Pack, which
// neither Load nor this function ever takes as an argument; Resolve
// (resolve.go) does that check per square instead.
func CheckTileNamesKnown(tiles map[string]string, errf FieldErrFunc) error {
	for key, name := range tiles {
		if _, _, ok := StandardTile(name); !ok {
			return errf(fmt.Sprintf("tiles[%q]", key),
				fmt.Sprintf("unknown tile %q (not in the standard vocabulary; pack tiles resolve in a later step)", name))
		}
	}
	return nil
}

// CheckOverridesInsideGrid validates that every overrides KEY names a square
// the grid actually contains. Overrides is sparse (spec §4.1), so unlike
// tiles there is no completeness rule — only a bounds rule. The VALUE is not
// inspected: it is an opaque pack tile name that only Resolve (resolve.go),
// given a *Pack, can validate — neither Load nor this function has a pack
// argument to check it against.
func CheckOverridesInsideGrid(overrides map[string]string, w, h int32, errf FieldErrFunc) error {
	for key := range overrides {
		x, y, ok := parseSquareKey(key)
		if !ok || x < 0 || x >= w || y < 0 || y >= h {
			return errf(fmt.Sprintf("overrides[%q]", key), "names a square outside the grid")
		}
	}
	return nil
}

// CheckObjectFootprints validates that every object's full FOOTPRINT (not
// merely its anchor square) lies inside the grid — a 2x2 object anchored at
// the grid's last column would otherwise hang half off the map with nothing
// catching it.
//
// Size must be at least 1x1: a zero (or omitted) size made the OLD version
// of this check pass for an out-of-grid anchor too, because `at+0 > w` is
// false exactly when `at >= w` is what should have failed — the footprint
// check and the anchor check were accidentally the same comparison, and a
// zero size broke it silently. The int64 arithmetic guards the same
// comparison against int32 overflow: At and Size both come straight from
// author-supplied JSON, and `at:2147483647, size:1` wraps a naive int32 sum
// negative, which is also less than w and so also wrongly passes. Object art
// resolution is not checked here or anywhere yet: this function has no
// *Pack to resolve it against, and Resolve (resolve.go) resolves a square's
// tile override only — resolving an object's own art is later work, left
// for whichever task gives objects their own resolution path.
func CheckObjectFootprints(objs []Object, w, h int32, errf FieldErrFunc) error {
	for i, o := range objs {
		field := fmt.Sprintf("objects[%d]", i)
		if o.W < 1 || o.H < 1 {
			return errf(field+".size", fmt.Sprintf("footprint must be at least 1x1, got %dx%d", o.W, o.H))
		}
		if o.X < 0 || o.Y < 0 ||
			int64(o.X)+int64(o.W) > int64(w) ||
			int64(o.Y)+int64(o.H) > int64(h) {
			return errf(field+".at", "places the object outside the grid")
		}
	}
	return nil
}

// CheckPlacementsNotInWalls is the check the flat scene-plus-four-numbers
// format could never express (design spec §4.4): a token must not start
// inside a wall. Callers run this last because it is the one check that
// reads a square's TILE NAME rather than merely its coordinates — by the
// time this runs, CheckEverySquarePresent and CheckTileNamesKnown have
// already proven every in-grid square has a known name WHENEVER TILES WERE
// DECLARED AT ALL. When tiles is empty there is nothing to have proven and
// nothing to stand in: a lookup yields "", StandardTile("") reports unknown,
// the kind is never "wall", and every placement passes. That is correct — a
// scene with no terrain has no walls to stand inside.
//
// So the only new failure this step can find is a placement whose OWN square
// is outside the grid (checked here too, since tiles has no entry to read
// otherwise) or whose square resolves to kind "wall".
func CheckPlacementsNotInWalls(placements []Placement, tiles map[string]string, w, h int32, errf FieldErrFunc) error {
	for i, p := range placements {
		field := fmt.Sprintf("placements[%d]", i)
		if p.X < 0 || p.X >= w || p.Y < 0 || p.Y >= h {
			return errf(field, "names a square outside the grid")
		}
		name := tiles[squareKey(p.X, p.Y)]
		kind, _, _ := StandardTile(name)
		if kind == "wall" {
			return errf(field,
				fmt.Sprintf("token %q would start inside a wall (square %d,%d is %q)", p.TokenID, p.X, p.Y, name))
		}
	}
	return nil
}

// parseSquareKey parses a "x,y" square key — the inverse of squareKey. It
// requires BOTH parts to be consumed entirely by strconv (unlike
// fmt.Sscanf's "%d,%d", which stops at the first non-digit and would accept
// something like "1,1abc" as "1,1"). Unparseable input reports ok=false
// rather than panicking, so a malformed Overrides key fails loud through the
// same bounds-error path as one that is merely out of range, instead of
// crashing the loader.
func parseSquareKey(key string) (x, y int32, ok bool) {
	xs, ys, found := strings.Cut(key, ",")
	if !found {
		return 0, 0, false
	}
	xi, err := strconv.ParseInt(xs, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	yi, err := strconv.ParseInt(ys, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	return int32(xi), int32(yi), true
}

// packJSON is the on-disk shape of a pack manifest (design spec §4.2):
// pack.json beside the images it names. Tiles and Objects share one JSON
// shape (packTileJSON) because a pack.json entry looks identical whichever
// array it sits in — mirrored in Go by PackTile itself (format.go) being the
// one exported type for both.
type packJSON struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	CellPx  int32          `json:"cell_px"`
	Tiles   []packTileJSON `json:"tiles"`
	Objects []packTileJSON `json:"objects"`
}

type packTileJSON struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Material   string `json:"material"`
	File       string `json:"file"`
	FileOpen   string `json:"file_open"`
	FileClosed string `json:"file_closed"`
	Desc       string `json:"desc"`
}

// LoadPack reads and validates the pack manifest at dir/pack.json: strict
// JSON decoding (decodeStrict, the same shape Load uses), then keys Tiles
// and Objects by name so Resolve (resolve.go) gets an O(1) lookup per
// square. LoadPack never reads a *Map — a pack is reusable across many maps
// (spec §4.3's "load standalone" principle applied to art), so it takes only
// a directory.
func LoadPack(dir string) (*Pack, error) {
	path := filepath.Join(dir, "pack.json")
	var raw packJSON
	if err := decodeStrict(path, &raw); err != nil {
		return nil, err
	}

	tiles, err := packTileMap(path, "tiles", raw.Tiles)
	if err != nil {
		return nil, err
	}
	objects, err := packTileMap(path, "objects", raw.Objects)
	if err != nil {
		return nil, err
	}

	return &Pack{
		ID:      raw.ID,
		Name:    raw.Name,
		CellPx:  raw.CellPx,
		Tiles:   tiles,
		Objects: objects,
	}, nil
}

// packTileMap keys a pack.json array by its own entries' Name, refusing
// (rather than silently keeping the last one) an empty or a repeated name:
// a map author's override references a pack tile by name alone, so a name
// that cannot uniquely address one entry would let two authored pictures
// collide, with whichever JSON array element decoded last winning silently
// and the other becoming permanently unreferenceable. Field names which
// array ("tiles" or "objects") an error came from, matching this package's
// existing fieldErr convention of naming the offending JSON path.
func packTileMap(path, field string, items []packTileJSON) (map[string]PackTile, error) {
	out := make(map[string]PackTile, len(items))
	for i, it := range items {
		loc := fmt.Sprintf("%s[%d].name", field, i)
		if it.Name == "" {
			return nil, fieldErr(path, loc, "must not be empty")
		}
		if _, dup := out[it.Name]; dup {
			return nil, fieldErr(path, loc, fmt.Sprintf("duplicate name %q", it.Name))
		}
		// packTileJSON and PackTile share field order and types exactly (kept
		// in lockstep deliberately), so this is a straight conversion rather
		// than a literal that would drift silently if a field were ever
		// added to one and not the other.
		out[it.Name] = PackTile(it)
	}
	return out, nil
}

// decodeStrict decodes the JSON file at path into v with unknown fields
// disallowed — reused shape from internal/adventure/load.go, so a map
// author gets the same quality of "you misspelled a field" error an
// adventure author already gets.
func decodeStrict(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("mapdef: %s: %w", path, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("mapdef: %s: %w", path, err)
	}
	return nil
}

// fieldErr builds a load error naming both the offending file and field —
// reused shape from internal/adventure/load.go's fieldErr.
func fieldErr(path, field, msg string) error {
	return fmt.Errorf("mapdef: %s: field %q: %s", path, field, msg)
}
