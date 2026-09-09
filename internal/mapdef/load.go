package mapdef

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/artlib"
)

// mapJSON is the on-disk shape of a map file (design spec §4.1). Field names
// match the spec's JSON examples exactly; Go-side validation and the
// friendlier Map/Object/Placement shapes live in format.go and below.
type mapJSON struct {
	FormatVersion int32  `json:"format_version"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	GridWidth     int32  `json:"grid_width"`
	GridHeight    int32  `json:"grid_height"`
	// CellPx is a json.RawMessage because PRESENCE is what decides. A plain
	// int32 cannot tell an absent cell_px from an explicit {"cell_px": 0}, so
	// "an undeclared map inherits the campaign default" and "zero pixels is
	// refused" would be one branch pretending to be two — and a *int32 only
	// moves the hole, since JSON null unmarshals into a pointer as nil and would
	// read as absent. A bare json.RawMessage is nil ONLY when the key is truly
	// absent: `null` arrives as the four bytes "null". Raw bytes decide nothing
	// until loadAs decides.
	CellPx     json.RawMessage   `json:"cell_px"`
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
// perform without an art directory (resolving an art name against one is
// Resolve's job, resolve.go). Every error names the offending file and field; Load returns
// (nil, err) as soon as the first violation is found, matching adventure's
// fail-loud-at-load posture (spec §7).
//
// THE STRICTNESS IS LOAD-BEARING AND NOT MERELY TIDY. It is the whole of what
// refuses a map authored before 2026-09-02-art-is-a-flat-library — one carrying
// a `"pack"` container, or that container under any rename — whose overrides
// values were written against a namespace that no longer exists and would
// otherwise draw the wrong thing with nobody told. Two loadAs arms carried that
// refusal with migration instructions attached until 2026-09-06, when Patrik
// ruled the route out (nothing has ever shipped, and every campaign that has
// ever existed is in this repository). What is left is
// TestAMapDeclaringAPackOrAPackageIsStillRefused, which drives the file through
// Load rather than trusting this sentence.
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
func Load(path string) (*Map, error) { return loadAs(path, path) }

// loadAs is Load with the name its errors carry (display) held separate
// from the file they read (path) — see decodeStrict's doc comment for why
// that separation exists and who uses it. Load itself passes the path for
// both, so nothing about Load's errors changed.
func loadAs(path, display string) (*Map, error) {
	var raw mapJSON
	if err := decodeStrict(path, display, &raw); err != nil {
		return nil, err
	}

	if raw.FormatVersion == 0 {
		return nil, fieldErr(display, "format_version", fmt.Sprintf(
			"required: this server understands %d, and an undeclared format is not "+
				"assumed to be any of them", MapFormatVersion))
	}
	if raw.FormatVersion != MapFormatVersion {
		return nil, fieldErr(display, "format_version", fmt.Sprintf(
			"declares %d; this server understands %d", raw.FormatVersion, MapFormatVersion))
	}

	// cell_px, when the map declares one (art-is-a-flat-library design spec §6 as
	// amended 2026-09-05). Checked HERE, after format_version and before the
	// geometry, because it is a property of the whole map rather than of any
	// square — and refused rather than clamped, for the reason MinCellPx's own
	// doc comment gives.
	var cellPx int32
	if raw.CellPx != nil {
		if err := json.Unmarshal(raw.CellPx, &cellPx); err != nil {
			return nil, fieldErr(display, "cell_px", fmt.Sprintf(
				"must be a whole number of pixels between %d and %d: %v",
				MinCellPx, MaxCellPx, err))
		}
		if cellPx < MinCellPx || cellPx > MaxCellPx {
			return nil, fieldErr(display, "cell_px", fmt.Sprintf(
				"is %d; one grid square of art is between %d and %d pixels. Leave the field out "+
					"to use the campaign's own cell_px (campaign.json), which is what every map "+
					"that declares nothing does", cellPx, MinCellPx, MaxCellPx))
		}
	}

	if raw.GridWidth < 1 {
		return nil, fieldErr(display, "grid_width", fmt.Sprintf("must be >= 1, got %d", raw.GridWidth))
	}
	if raw.GridHeight < 1 {
		return nil, fieldErr(display, "grid_height", fmt.Sprintf("must be >= 1, got %d", raw.GridHeight))
	}

	errf := func(field, msg string) error { return fieldErr(display, field, msg) }

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
	if err := CheckObjectArtDeclared(objects, errf); err != nil {
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
		FormatVersion: raw.FormatVersion,
		ID:            raw.ID,
		Name:          raw.Name,
		GridW:         raw.GridWidth,
		GridH:         raw.GridHeight,
		CellPx:        cellPx,
		Tiles:         raw.Tiles,
		Overrides:     raw.Overrides,
		Objects:       objects,
		Placements:    placements,
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
// The walk itself is RequireEverySquarePresent below; this function is that
// walk plus the file format's opt-out, and the opt-out is the only
// difference between them. Callers run this before CheckTileNamesKnown so a
// file that is both incomplete and has an invalid name reports the more
// fundamental defect (a square with no answer at all) first.
func CheckEverySquarePresent(tiles map[string]string, w, h int32, errf FieldErrFunc) error {
	if len(tiles) == 0 {
		return nil
	}
	return RequireEverySquarePresent(tiles, w, h, errf)
}

// RequireEverySquarePresent is the completeness walk itself, WITHOUT the
// opt-out above: every square of w x h must be named, and a tiles map that
// names none of them is short of all of them rather than exempt.
//
// IT HAS ONE CALLER AND IT IS CheckEverySquarePresent, one function up. It
// was exported from 2026-09-01 for internal/gateway's create_scene — the
// IMPROVISED path by which a place came into existence mid-session, which had
// no authored files to keep loading and so no claim on the opt-out — and that
// command left the platform on 2026-09-02 (Patrik's ruling, 2026-09-01: the
// kernel serves maps, it does not make them).
//
// SO IT SITS AT NO BOUNDARY AT ALL NOW, and that is worth stating plainly
// rather than dressing up. Its one caller has already returned on an empty
// tiles map before reaching here, so re-adding `if len(tiles) == 0 { return
// nil }` to this function would be behaviour-preserving for the entire
// program: no input to Load can tell the two apart.
// TestRequireEverySquarePresentHasNoOptOut therefore pins an INTERNAL, which
// sits awkwardly against CLAUDE.md rule 1 ("tests pin boundary behavior, never
// internals"), and that is the real cost of the split — not a mutant that
// would otherwise escape.
//
// KEPT EXPORTED ANYWAY, for a reason about tests rather than about callers:
// every test file in this package is `package mapdef_test`, so unexporting
// this would mean introducing the package's only internal test file to keep
// one guard. That is a worse trade for a symbol already confined to
// internal/. The BOUNDARY coverage is independent of all of it and does not
// move: load_test.go's "missing-square" fixture drives a genuinely partial map
// file — 8 of a 3x3's 9 squares — through mapdef.Load and gets the refusal.
//
// The split is a split of the RULE from its exemption, not a fork of the
// rule: one walk, one message, and the only difference is whether an empty
// tiles map is a legitimate claim.
//
// It walks the GRID, not the tiles map, because completeness is a property
// of what is MISSING — a map iteration only ever sees what is present. It
// names the FIRST missing square rather than counting them, because a
// caller's next act is to go and declare that square.
func RequireEverySquarePresent(tiles map[string]string, w, h int32, errf FieldErrFunc) error {
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			key := squareKey(x, y)
			if _, ok := tiles[key]; !ok {
				return errf(fmt.Sprintf("tiles[%q]", artlib.Clip(key, artlib.MaxFragment)), "no tile named for this square")
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
			return errf(fmt.Sprintf("tiles[%q]", artlib.Clip(key, artlib.MaxFragment)), "names a square outside the grid")
		}
	}
	return nil
}

// CheckTileNamesKnown validates every tiles VALUE against the standard
// vocabulary. Callers run this after CheckEverySquarePresent and
// CheckTilesInsideGrid have proven every grid square has an entry and no
// entry lies outside the grid, so walking the tiles map here (rather than
// the grid again) covers exactly the same squares. Overrides values (art ids)
// are NOT checked here — that needs an art directory, which neither Load nor
// this function ever takes as an argument; Resolve (resolve.go) does that
// check per square instead, and since
// 2026-09-02-art-is-a-flat-library Task 3 a name that does not resolve costs
// its square's picture and a warning rather than the map.
func CheckTileNamesKnown(tiles map[string]string, errf FieldErrFunc) error {
	for key, name := range tiles {
		if _, _, ok := StandardTile(name); !ok {
			return errf(fmt.Sprintf("tiles[%q]", artlib.Clip(key, artlib.MaxFragment)),
				fmt.Sprintf("unknown tile %q (not in the standard vocabulary; art names resolve in a later step)", artlib.Clip(name, artlib.MaxFragment)))
		}
	}
	return nil
}

// CheckOverridesInsideGrid validates that every overrides KEY names a square
// the grid actually contains. Overrides is sparse (spec §4.1), so unlike
// tiles there is no completeness rule — only a bounds rule. The VALUE is not
// inspected: it is an opaque art id that only Resolve (resolve.go), given an
// art directory, can look up — neither Load nor this function has one to
// check it against, and since 2026-09-02-art-is-a-flat-library Task 3 a value
// that looks up to nothing is a warning rather than a refusal anyway.
func CheckOverridesInsideGrid(overrides map[string]string, w, h int32, errf FieldErrFunc) error {
	for key := range overrides {
		x, y, ok := parseSquareKey(key)
		if !ok || x < 0 || x >= w || y < 0 || y >= h {
			return errf(fmt.Sprintf("overrides[%q]", artlib.Clip(key, artlib.MaxFragment)), "names a square outside the grid")
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
// negative, which is also less than w and so also wrongly passes. This
// function's own job stops at GEOMETRY — an object's ART is checked
// separately, split across two functions by what each can prove without an art
// directory: CheckObjectArtDeclared (below) proves a name was declared at all;
// ResolveObjectArt (resolve.go) proves a declared name actually resolves
// against the campaign's flat art/ directory, run by BuildSceneCreated
// (compile.go) during the same dry run that already catches an unresolvable
// tile override. It said "against the map's pack" until Task 5 of
// 2026-09-02-art-is-a-flat-library, which deleted the field a map named one
// with; Task 3 of the same plan had already moved the resolution itself, and
// Task 7 deleted mapdef.Pack.
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

// CheckObjectArtDeclared validates that every object names non-empty art —
// the one part of spec §4.4's "every `art` name resolves" that Load can
// prove without an art directory (whole-branch-review finding I1: object art
// was resolved NOWHERE before this function and ResolveObjectArt existed — the
// objects half of a pack manifest was loaded and read by nothing in Go; the
// manifest itself left at 2026-09-02-art-is-a-flat-library Task 7). A tile's
// own art may legally be empty (it falls back to the standard vocabulary —
// CheckTileNamesKnown / StandardTile), but an object has no such fallback:
// the client's bundled baseline covers the eleven standard tile natures only,
// never objects (tools/genmappack/std_pack.go, whose own test says so
// outright: "objects have no standard fallback"). So an object with empty art can never draw,
// under any circumstances — exactly the invisible-barrier defect spec §1.3
// exists to prevent (a blocks_move object with nothing telling a player why
// their square is blocked), and unlike a merely WRONG art name (which starts
// drawing the moment the right file is installed under that name), an EMPTY
// one names nothing that could ever be installed — so it is refused here, at
// Load, rather than deferred to whichever caller happens to have an art
// directory in hand.
//
// Whether a non-empty name is actually installed is a separate question this
// function has no way to answer, and since
// 2026-09-02-art-is-a-flat-library Task 3 the answer "no" is a warning on one
// object rather than a refusal — see ResolveObjectArt (resolve.go).
func CheckObjectArtDeclared(objs []Object, errf FieldErrFunc) error {
	for i, o := range objs {
		if o.Art == "" {
			return errf(fmt.Sprintf("objects[%d].art", i),
				"must not be empty — objects have no standard art fallback (only tiles do), so an object with no art can never draw")
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

// decodeStrict decodes the JSON file at path into v with unknown fields
// disallowed — reused shape from internal/adventure/load.go, so a map
// author gets the same quality of "you misspelled a field" error an
// adventure author already gets.
//
// display is the name the ERROR carries, held separate from the path
// OPENED so a caller can name a file the way its reader knows it. Load
// passes the path itself and reads exactly as it always did;
// LoadInstalled passes "maps/<id>.json", because its errors travel to a
// client over the wire and an absolute server path is not that client's
// business (2026-09-01-create-scene-leaves Task 6, fix round 1).
//
// os.Open's own error is UNWRAPPED to its bare cause before wrapping: an
// *fs.PathError prints the path it was given ("open /abs/x.json: no such
// file or directory"), so forwarding it whole would put the path back in
// the message display was chosen to keep out — and would say it twice for
// the callers that pass the path anyway. Unwrapping keeps
// errors.Is(err, fs.ErrNotExist) working, because that answer comes from
// the syscall.Errno underneath, not from the PathError around it.
func decodeStrict(path, display string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("mapdef: %s: %w", display, artlib.BoundErr(unpath(err)))
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		// The DECODE side needs unpath just as much as the open side: a
		// path whose entry is a directory opens fine and fails on the
		// first read, with an *fs.PathError of its own ("read /abs/x:
		// is a directory").
		return fmt.Errorf("mapdef: %s: %w", display, artlib.BoundErr(unpath(err)))
	}
	return nil
}

// unpath strips the filesystem path an *fs.PathError carries, leaving the
// bare cause. Every error decodeStrict returns is already named by display,
// and a PathError would put the opened path back beside it — twice for the
// callers that pass the path as display, and where it does not belong for
// LoadInstalled, whose errors travel to a client.
//
// errors.Is survives: "no such file or directory" is answered by the
// syscall.Errno underneath (its own Is method maps ENOENT to
// fs.ErrNotExist), not by the PathError wrapper around it. Anything that is
// not an *fs.PathError is returned untouched.
func unpath(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// fieldErr builds a load error naming both the offending file and field —
// reused shape from internal/adventure/load.go's fieldErr.
func fieldErr(path, field, msg string) error {
	// A BACKSTOP, not the bound. Each caller clips the value it interpolates,
	// which keeps the platform's own sentence intact — clipping the whole msg
	// here instead truncated from the END, so a DM got 240 characters of the
	// file's garbage and lost "(not in the standard vocabulary…)", the half
	// telling them where to look.
	//
	// It stays because internal/adventure builds its own errf over these same
	// mapdef.Check* functions, and a caller added later is likelier to forget a
	// clip than to route round this. If its mutant survives the gate, that is
	// what a backstop is and it gets adjudicated rather than deleted — deleting
	// a guard because no test could distinguish it is exactly the mistake this
	// round is repairing.
	return fmt.Errorf("mapdef: %s: field %q: %s", path, field, artlib.Clip(msg, artlib.MaxMessage))
}
