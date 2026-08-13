package mapdef

import (
	"encoding/json"
	"fmt"
	"os"
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
	Objects    []objectJSON      `json:"objects"`
	Placements []placementJSON   `json:"placements"`
}

type objectJSON struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	At          [2]int32 `json:"at"`
	Size        [2]int32 `json:"size"`
	Rot         int32    `json:"rot"`
	BlocksSight bool     `json:"blocks_sight"`
	BlocksMove  bool     `json:"blocks_move"`
	Art         string   `json:"art"`
}

type placementJSON struct {
	TokenID string `json:"token_id"`
	ActorID string `json:"actor_id"`
	X       int32  `json:"x"`
	Y       int32  `json:"y"`
}

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

	if err := checkEverySquarePresent(path, raw.Tiles, raw.GridWidth, raw.GridHeight); err != nil {
		return nil, err
	}
	if err := checkTilesInsideGrid(path, raw.Tiles, raw.GridWidth, raw.GridHeight); err != nil {
		return nil, err
	}
	if err := checkTileNamesKnown(path, raw.Tiles); err != nil {
		return nil, err
	}
	if err := checkOverridesInsideGrid(path, raw.Overrides, raw.GridWidth, raw.GridHeight); err != nil {
		return nil, err
	}
	objects, err := checkObjectsInsideGrid(path, raw.Objects, raw.GridWidth, raw.GridHeight)
	if err != nil {
		return nil, err
	}
	placements, err := checkPlacementsNotInWalls(path, raw.Placements, raw.Tiles, raw.GridWidth, raw.GridHeight)
	if err != nil {
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

// checkEverySquarePresent enforces the completeness rule that is the whole
// point of the keyed format (spec §4.1: "there is no implicit fallback
// anywhere. Every square names its own tile."). It walks the GRID, not the
// Tiles map, because completeness is a property of what is MISSING — a map
// iteration only ever sees what is present. Run before checkTileNamesKnown
// so a file that is both incomplete and has an invalid name reports the more
// fundamental defect (a square with no answer at all) first.
func checkEverySquarePresent(path string, tiles map[string]string, w, h int32) error {
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			key := squareKey(x, y)
			if _, ok := tiles[key]; !ok {
				return fieldErr(path, fmt.Sprintf("tiles[%q]", key), "no tile named for this square")
			}
		}
	}
	return nil
}

// checkTilesInsideGrid is checkEverySquarePresent's mirror image: that
// function proves every REQUIRED square has an entry; this one proves
// Tiles has no EXTRA entry outside the grid (spec §4.4: "no entry lies
// outside the grid"). The two are independent facts — a map can be
// complete AND carry a stray out-of-grid key at the same time — so a
// single loop cannot stand in for both, and this is deliberately its own
// function rather than folded into checkEverySquarePresent: doing so would
// mean deleting ONE function to fault-inject BOTH rules, leaving the
// completeness rule's own fault injection unable to prove it in isolation.
func checkTilesInsideGrid(path string, tiles map[string]string, w, h int32) error {
	for key := range tiles {
		x, y, ok := parseSquareKey(key)
		if !ok || x < 0 || x >= w || y < 0 || y >= h {
			return fieldErr(path, fmt.Sprintf("tiles[%q]", key), "names a square outside the grid")
		}
	}
	return nil
}

// checkTileNamesKnown validates every Tiles VALUE against the standard
// vocabulary. It runs after checkEverySquarePresent and checkTilesInsideGrid
// have proven every grid square has an entry and no entry lies outside the
// grid, so walking the Tiles map here (rather than the grid again) covers
// exactly the same squares. Pack-declared names are not checked — resolving
// those needs the pack manifest, which is Task 3.
func checkTileNamesKnown(path string, tiles map[string]string) error {
	for key, name := range tiles {
		if _, _, ok := StandardTile(name); !ok {
			return fieldErr(path, fmt.Sprintf("tiles[%q]", key),
				fmt.Sprintf("unknown tile %q (not in the standard vocabulary; pack tiles resolve in a later step)", name))
		}
	}
	return nil
}

// checkOverridesInsideGrid validates that every Overrides KEY names a square
// the grid actually contains. Overrides is sparse (spec §4.1), so unlike
// Tiles there is no completeness rule — only a bounds rule. The VALUE is not
// inspected: it is an opaque pack tile name until Task 3 can resolve it
// against a pack manifest.
func checkOverridesInsideGrid(path string, overrides map[string]string, w, h int32) error {
	for key := range overrides {
		x, y, ok := parseSquareKey(key)
		if !ok || x < 0 || x >= w || y < 0 || y >= h {
			return fieldErr(path, fmt.Sprintf("overrides[%q]", key), "names a square outside the grid")
		}
	}
	return nil
}

// checkObjectsInsideGrid validates that every object's full FOOTPRINT (not
// merely its anchor square) lies inside the grid — a 2x2 object anchored at
// the grid's last column would otherwise hang half off the map with nothing
// catching it. It also performs the JSON-to-Map shape conversion (At/Size
// splitting into X/Y/W/H) so callers never see the wire shape. art
// resolution is Task 3's concern, not checked here.
//
// Size must be at least 1x1: a zero (or omitted) size made the OLD version
// of this check pass for an out-of-grid anchor too, because `at+0 > w`
// is false exactly when `at >= w` is what should have failed — the
// footprint check and the anchor check were accidentally the same
// comparison, and a zero size broke it silently. The int64 arithmetic
// guards the same comparison against int32 overflow: At and Size both come
// straight from author-supplied JSON, and `at:2147483647, size:1` wraps a
// naive int32 sum negative, which is also less than w and so also wrongly
// passes.
func checkObjectsInsideGrid(path string, objs []objectJSON, w, h int32) ([]Object, error) {
	out := make([]Object, 0, len(objs))
	for i, o := range objs {
		field := fmt.Sprintf("objects[%d]", i)
		if o.Size[0] < 1 || o.Size[1] < 1 {
			return nil, fieldErr(path, field+".size",
				fmt.Sprintf("footprint must be at least 1x1, got %dx%d", o.Size[0], o.Size[1]))
		}
		if o.At[0] < 0 || o.At[1] < 0 ||
			int64(o.At[0])+int64(o.Size[0]) > int64(w) ||
			int64(o.At[1])+int64(o.Size[1]) > int64(h) {
			return nil, fieldErr(path, field+".at", "places the object outside the grid")
		}
		out = append(out, Object{
			ID:          o.ID,
			Kind:        o.Kind,
			X:           o.At[0],
			Y:           o.At[1],
			W:           o.Size[0],
			H:           o.Size[1],
			Rotation:    o.Rot,
			BlocksSight: o.BlocksSight,
			BlocksMove:  o.BlocksMove,
			Art:         o.Art,
		})
	}
	return out, nil
}

// checkPlacementsNotInWalls is the check the flat scene-plus-four-numbers
// format could never express (design spec §4.4): a token must not start
// inside a wall. It runs last because it is the one check that reads a
// square's TILE NAME rather than merely its coordinates — by the time this
// runs, checkEverySquarePresent and checkTileNamesKnown have already proven
// every in-grid square has a known name, so the only new failure this step
// can find is a placement whose OWN square is outside the grid (checked
// here too, since Tiles has no entry to read otherwise) or whose square
// resolves to kind "wall".
func checkPlacementsNotInWalls(path string, placements []placementJSON, tiles map[string]string, w, h int32) ([]Placement, error) {
	out := make([]Placement, 0, len(placements))
	for i, p := range placements {
		field := fmt.Sprintf("placements[%d]", i)
		if p.X < 0 || p.X >= w || p.Y < 0 || p.Y >= h {
			return nil, fieldErr(path, field, "names a square outside the grid")
		}
		name := tiles[squareKey(p.X, p.Y)]
		kind, _, _ := StandardTile(name)
		if kind == "wall" {
			return nil, fieldErr(path, field,
				fmt.Sprintf("token %q would start inside a wall (square %d,%d is %q)", p.TokenID, p.X, p.Y, name))
		}
		out = append(out, Placement(p))
	}
	return out, nil
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
