// Command genmappack generates TWO sets of art: the cellar starter art that
// campaigns/example/maps/cellar.json's overrides name, into the campaign's own
// flat art/ directory, and (added for review finding C2, 2026-08-16)
// client/public/std-pack, a baseline picture for every one of
// internal/mapdef/standard.go's eleven standard natures — see std_pack.go's own
// header comment for why a square with no art override needs this at all, and
// why it ships from a different place than the cellar art does.
//
// THE TWO HALVES NOW WRITE DIFFERENT FORMATS, and that is the design rather
// than a migration left half done. The cellar half writes what
// internal/artlib reads (2026-09-02-art-is-a-flat-library design spec §3): one
// picture per file, a sidecar beside each TILE picture and none beside an
// object's, and kebab-case stems that ARE the art ids — no manifest, because
// nothing declares an id. The std half keeps its pack.json, because its reader
// is client/src/view/pack-assets.ts, which fetches a manifest out of the
// client's own bundle and never goes through artlib at all.
//
// THE NAME genmappack IS NOW HALF WRONG and is kept anyway: renaming the
// directory would move every citation to it in this tree for a tool that still
// generates one pack. Whoever renames it should do it on its own.
//
// WHAT THE FLAT FORMAT COSTS THIS TOOL: the per-piece `desc` strings below have
// nowhere on disk to go. A sidecar's fields are fixed by artlib's strict decode
// (format_version, kind, material, open, closed) and no route lists art, so a
// description reaches nobody — where pack.json carried it to a model choosing
// tiles (design spec §1.5's test was an LLM authoring a map from the document
// and a manifest alone). They stay here as this catalogue's own documentation
// and as what the std half still ships; giving them a reader again is a format
// change and belongs to whoever wants one.
//
// WHY GENERATED RATHER THAN DRAWN OR FETCHED. Patrik's ruling: copy no
// image, fetch art from nowhere else on the web (fantasymapbuilder.com's
// organisation is worth studying; its presets are credited to a named
// artist with no stated license, and are therefore off limits). Generating
// procedurally from Go's standard image/color/draw/png — no new dependency;
// this repo already builds with Go, and Pillow was checked and rejected for
// exactly that reason — buys three things at once: provenance is
// unambiguous (every pixel traces to the code below, not to a URL), the
// art is RE-TUNABLE rather than an opaque binary (change a colour, rerun,
// diff the PNGs), and this file doubles as a worked example of what an art
// author must actually produce — which, after Task 8 of the art-is-a-flat-
// library plan, is the flat layout internal/artlib reads rather than the
// manifest below.
//
// Deliberately simple: flat colour fields, per-pixel noise, and a few lines
// or a filled circle. "Simple textured surfaces... and a handful of object
// glyphs" (Task 10 brief) is the whole brief; nothing here is trying to be
// good art, only to be UNAMBIGUOUS art — a wall reads as a wall, a crate
// reads as a crate, at 64px in a browser tile.
//
// Run: go run ./tools/genmappack [-out campaigns/example/art] [-std-out client/public/std-pack] [-cell-px 64]
// Both sets are (re)written on every run — there is no flag to write only one.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"os"
	"path/filepath"
)

// size is how many pixels of art one grid square gets: every image this tool
// emits is size x size. It is what campaign.json's cell_px DECLARES about a
// campaign's art (design spec §6), and it stopped being a pack field when the
// pack left — hence -cell-px below, which sets it.
//
// A var rather than a const, and set once from a flag before anything draws:
// every drawing primitive in this file bounds its loops by it. Nothing writes
// it after generate begins.
//
// 64 by default, per the Task 10 brief — "Keep the images small — a few KB
// each, 64px is plenty" — big enough to read as a distinct texture at typical
// camera scale, small enough that the whole set stays a handful of KB on disk
// and over the wire. campaigns/example/campaign.json declares the same number
// as its cell_px, which is what makes the shipped art and the shipped settings
// agree; the pair is checked from both ends
// (TestTheCommittedArtIsDrawnAtThisToolsOwnDefault here, and cmd/vtt's
// TestTheShippedCampaignDeclaresTheCellSizeItsArtWasDrawnAt through
// campaigncfg, which this tool may not import).
var size = defaultCellPx

const defaultCellPx = 64

// seed is fixed so a rerun with unchanged code reproduces byte-identical
// PNGs — REPRODUCIBLE, this file's own header comment's second promise. A
// random seed would make every rerun a silent diff even when nothing about
// the art was meant to change, defeating "generate the pack from it" as a
// workflow: a pack author reruns this to RE-TUNE one texture, not to
// discover that every other file also moved.
const seed = 20260812

// --- art/'s on-disk shape (design spec §3) ---------------------------------
//
// A piece of art is a PICTURE, and for tile art a SIDECAR beside it carrying
// what a picture cannot say. Nothing declares an id: the filename stem is the
// id, so artOut.ID is what every filename below is built from and there is no
// second place for the two to disagree.

// sidecarOut is <id>.json, the file internal/artlib's own sidecar type decodes.
//
// ITS KEYS ARE artlib's, NOT A SUPERSET: that decode runs
// DisallowUnknownFields, so one extra key here would make every piece this tool
// writes unreadable. That is why `desc` is absent (see this file's header for
// what the flat format costs) and why omitempty is load-bearing rather than
// tidy — artlib refuses "open"/"closed" on anything whose kind is not a door.
//
// Written WITHOUT importing internal/artlib, for the reason packOut below is
// its own type: this is a content tool, and a shape it writes into a campaign
// directory should not pull it into the server's internals. What keeps the two
// honest is genmappack_test.go, which runs artlib.Validate and artlib.Lookup
// over what this writes — a stronger check than a shared struct, because it
// exercises the reader rather than agreeing with it by construction.
type sidecarOut struct {
	FormatVersion int32  `json:"format_version"`
	Kind          string `json:"kind"`
	Material      string `json:"material"`
	Open          string `json:"open,omitempty"`
	Closed        string `json:"closed,omitempty"`
}

// artFormatVersion mirrors artlib.FormatVersion (1) without importing it, the
// same way packFormatVersion mirrors what its own reader wanted. The test above
// is what stops the two drifting.
const artFormatVersion int32 = 1

// artOut is one piece this tool wrote into art/: what it is, and every file it
// put on disk for it. Returned by generate so a test can walk the pieces
// instead of re-deriving the filenames it would be checking.
//
// Kind and Material are EMPTY FOR OBJECT ART, which has no sidecar at all
// (spec §3.4's asymmetry) — a bare picture is complete, because the map's own
// object entry already says what the thing does.
type artOut struct {
	ID       string
	Kind     string
	Material string
	Desc     string
	Sidecar  string
	Pictures []string
}

// --- pack.json's on-disk shape --------------------------------------------
//
// Mirrored internal/mapdef/load.go's packTileJSON/packJSON field-for-field
// until art-is-a-flat-library Task 7 deleted both
// (same JSON keys: name, kind, material, file, file_open, file_closed,
// desc, cell_px) WITHOUT importing that package. That loader type is
// unexported and shaped for DECODING (no omitempty — a decoder does not
// care), where this tool is shaped for ENCODING a clean, human-readable
// file a pack author might have hand-written; importing the loader's
// private wire type would also wire this content tool to an internal
// package for no reason a change to either side should have to consider.

type packTileOut struct {
	Name       string `json:"name"`
	Kind       string `json:"kind,omitempty"`
	Material   string `json:"material,omitempty"`
	File       string `json:"file,omitempty"`
	FileOpen   string `json:"file_open,omitempty"`
	FileClosed string `json:"file_closed,omitempty"`
	Desc       string `json:"desc"`
}

type packOut struct {
	// FormatVersion mirrors internal/mapdef.PackFormatVersion — deliberately
	// a bare literal (packFormatVersion below), not an import of that
	// constant, for the same reason this whole type exists unimported: see
	// the "pack.json's on-disk shape" section comment above packTileOut.
	// LoadPack (internal/mapdef/load.go) refused any pack.json omitting
	// this field, so every pack this tool writes carries it; that loader is
	// gone and nothing reads the result today (see this file's header).
	FormatVersion int32         `json:"format_version"`
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	CellPx        int           `json:"cell_px"`
	Tiles         []packTileOut `json:"tiles"`
	Objects       []packTileOut `json:"objects"`
}

// packFormatVersion mirrored internal/mapdef.PackFormatVersion's value (1)
// without importing that package, and outlives it (see the "pack.json's on-disk
// shape" section comment above packTileOut for why packOut/packTileOut are
// their own encoding-shaped types).
const packFormatVersion int32 = 1

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// defaultOut and defaultStdOut are the two COMMITTED directories this tool
// writes, named as constants so a test can say that a plain run targets exactly
// what the repository ships.
//
// THAT IS THE WHOLE JOB OF A DEFAULT HERE: a run with no flags must rewrite the
// art this repository ships, so `git diff` is the check on whether the generator
// and the committed bytes still agree. -out pointed at
// campaigns/example/packs/cellar-basics until art-is-a-flat-library Task 8 — a
// directory Task 7 had deleted — so a plain run resurrected the deleted pack as
// UNTRACKED files, and the next `git add -A` would have re-committed what Task 7
// removed. Nothing said so, because a default is not a value any test had ever
// looked at; TestAPlainRunTargetsExactlyWhatIsCommitted is that test.
const (
	defaultOut    = "campaigns/example/art"
	defaultStdOut = "client/public/std-pack"
)

// run is main's whole body, with its arguments and its two streams passed in.
//
// SPLIT OUT FOR THE SAME REASON generate IS: main owns the process and this owns
// the behaviour. Here the behaviour worth owning is the flag layer itself — the
// defaults, and the refusal of a cell size that would write pictures nothing can
// draw — which os.Exit inside main put permanently out of reach of a test.
//
// It returns an exit code rather than calling os.Exit so that a test can see it.
// 2 for a flag error, matching what flag.ExitOnError would have produced;
// flag.ContinueOnError has already printed the message and the usage to stderr
// by then.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("genmappack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", defaultOut,
		"directory to write the cellar starter art into, as the flat layout "+
			"internal/artlib reads: <id>.png per picture, <id>.json beside each tile picture")
	stdOut := fs.String("std-out", defaultStdOut,
		"directory to write the standard-vocabulary baseline pack's pack.json and images into "+
			"(see std_pack.go's header comment for why this ships from the client bundle rather "+
			"than an authenticated art route)")
	cellPx := fs.Int("cell-px", defaultCellPx,
		"pixels per grid square: every picture written is this many pixels on a side. "+
			"It is what a campaign's campaign.json declares as cell_px (bounded 8..1024 there), "+
			"and it was a pack.json field until the pack left")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := checkCellPx(*cellPx); err != nil {
		fmt.Fprintf(stderr, "genmappack: %v\n", err)
		return 1
	}
	size = *cellPx

	cellar, std := generate(*out, *stdOut)
	fmt.Fprintf(stdout, "genmappack: wrote %d piece(s) of art at %dpx into %s\n",
		len(cellar), size, *out)
	fmt.Fprintf(stdout, "genmappack: wrote %d standard tile(s) and pack.json into %s\n",
		len(std.Tiles), *stdOut)
	return 0
}

// checkCellPx refuses a size that would write pictures nothing can draw.
//
// IT IS DELIBERATELY NOT THE CAMPAIGN FORMAT'S BOUND. cell_px is bounded 8..1024
// where it is READ — campaigncfg.MinCellPx and mapdef.MinCellPx carry the
// numbers and the reasoning — and a campaign.json outside that is refused by
// name at boot, with an operator holding the file. Repeating those two numbers
// here would put a third copy in a package that is not allowed to import either
// (.go-arch-lint.yml), so nothing could compare them; a run at 2000 wastes a
// regeneration and is then refused loudly, which is a cost worth paying to keep
// one authority.
//
// ZERO IS THE CASE THAT IS SILENT, and the only one this must catch itself:
// image.NewRGBA accepts an empty rectangle and png.Encode writes it, so the
// campaign would ship pictures every reader accepts and no board can draw.
//
// SPLIT OUT OF main SO IT CAN BE TESTED, like generate below: main owns flags
// and exit codes, this owns the decision.
func checkCellPx(v int) error {
	if v < 1 {
		return fmt.Errorf("-cell-px %d: a picture is at least one pixel on a side "+
			"(the campaign format bounds cell_px 8..1024 where it is read)", v)
	}
	return nil
}

// generate writes both packs and returns their manifests.
//
// SPLIT OUT OF main SO IT CAN BE TESTED. main owns flags and stdout; this owns
// the behaviour, and the behaviour that matters is REPRODUCIBILITY —
// genmappack_test.go regenerates into temp dirs and compares byte-for-byte
// against what is committed. That is what turns "generated, not borrowed" from
// a claim in a commit message into something a gate checks: committed art
// drifts silently from its source the moment somebody retouches a PNG or edits
// a description, and the licensing argument for this art only holds while the
// committed bytes really are this program's output.
func generate(out, stdOut string) ([]artOut, packOut) {
	mustMkdirAll(out)

	// #nosec G404 -- math/rand is REQUIRED here, not a shortcut. The seed is a
	// fixed constant so this generator is REPRODUCIBLE: re-running it must emit
	// byte-identical images, or every regeneration would dirty the repo and the
	// committed art could never be verified against its source. crypto/rand
	// would make the output different every run, which is the opposite of what
	// a committed asset generator needs. Nothing here is a secret.
	rng := rand.New(rand.NewSource(seed))

	// THE ORDER OF THESE DRAWS IS LOAD-BEARING and outlives the pack: one rng
	// runs through the cellar art and on into the standard pack below, so
	// inserting, removing or reordering a draw here changes every std-pack PNG
	// too. The ids and the drawings are the same ones the pack shipped, with
	// snake_case filenames rewritten to the kebab-case stems that ARE the art
	// ids — so these pictures are byte-identical to
	// campaigns/example/packs/cellar-basics' own, renamed.
	art := []artOut{
		writeArtTile(out, rng, "masonry-1", "wall", "stone",
			"coursed stone blockwork, the standard wall face for the cellar art",
			drawMasonry),
		writeArtTile(out, rng, "earth-1", "floor", "earth",
			"packed dirt floor, uneven and speckled with small stones",
			drawEarth),
		writeArtTile(out, rng, "flagstone-1", "floor", "stone",
			"cut flagstone paving, mortared in irregular slabs",
			drawFlagstone),
		writeArtDoor(out, rng, "cellar-door", "wood",
			"a banded wooden door; closed and open pictures are the SAME nature (spec §3.4) — "+
				"opening it changes only which of these two files the renderer picks, never the tile's kind",
			drawDoorClosed, drawDoorOpen),
		writeArtObject(out, rng, "pillar-stone",
			"a round stone column, wide enough to block a square's line of sight and passage",
			drawPillar),
		writeArtObject(out, rng, "crate-wood",
			"a stacked wooden shipping crate — good cover, or just clutter, depending on how it is placed",
			drawCrate),
		writeArtObject(out, rng, "barrel",
			"an upright wine barrel, banded in iron",
			drawBarrel),
		writeArtObject(out, rng, "brazier",
			"a standing iron brazier, coals lit — decorative: it blocks neither sight nor movement",
			drawBrazier),
	}

	// The standard-vocabulary baseline pack (review finding C2, std_pack.go's
	// own header comment for the full why/where). rng is NOT re-seeded here —
	// threaded straight from the cellar pack's own draws above, so a rerun of
	// the WHOLE tool is what the fixed seed reproduces byte-identically, not
	// just one half of it in isolation.
	stdManifest := writeStandardPack(stdOut, rng)
	return art, stdManifest
}

// mustMkdirAll creates dir (and any missing parents), or exits loudly.
//
// 0o750 rather than the more usual 0o755 because gosec (G301) asks for it
// and complying costs nothing here: git records only the executable bit, so
// the committed pack is byte-identical either way. A gate is never weakened
// to pass it, and this one did not need to be.
func mustMkdirAll(dir string) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: %v\n", err)
		os.Exit(1)
	}
}

// colorRGBA is color.RGBA{r, g, b, 0xff} spelled without three zero-padded
// hex bytes and a trailing 0xff at every one of std_pack.go's texture call
// sites — purely a call-site-brevity helper, opaque colour values only,
// nothing this file interprets (CLAUDE.md rule 5 is about game-system
// vocabulary, not RGB triples, but the same "opaque data, not meaning" spirit
// applies).
func colorRGBA(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 0xff}
}

// drawPicture draws one canvas via draw and PNG-encodes it to out/file.
//
// rng is threaded through from generate's single seeded source rather than
// re-seeded per call, so the WHOLE run — not just one texture in isolation — is
// what a fixed seed reproduces. Every picture either half of this tool emits
// goes through here, which is what keeps that thread unbroken.
func drawPicture(out string, rng *rand.Rand, file string, canvas *image.RGBA, draw func(*image.RGBA, *rand.Rand)) {
	draw(canvas, rng)
	writePNG(filepath.Join(out, file), canvas)
}

// writeArtTile writes one plain (non-door) piece of TILE art: the picture
// <id>.png, and the sidecar <id>.json beside it that a tile is REQUIRED to have
// (design spec §3.4) because kind and material are what the engine acts on and
// a picture cannot say them.
func writeArtTile(out string, rng *rand.Rand, id, kind, material, desc string, draw func(*image.RGBA, *rand.Rand)) artOut {
	picture := id + ".png"
	drawPicture(out, rng, picture, newCanvas(), draw)
	sidecar := writeSidecar(out, id, sidecarOut{
		FormatVersion: artFormatVersion, Kind: kind, Material: material,
	})
	return artOut{ID: id, Kind: kind, Material: material, Desc: desc,
		Sidecar: sidecar, Pictures: []string{picture}}
}

// writeArtDoor writes the one piece whose pictures are not named after it: a
// door has an open one and a closed one and NO <id>.png (design spec §3.4), so
// its sidecar is the only thing that says which file is which.
//
// The picture stems are <id>-open and <id>-closed, which makes them ordinary
// object art to anything that goes looking — a bare picture with no sidecar
// resolves, and nothing in artlib objects to art nobody names. That is
// deliberate: it keeps every filename in art/ a legal art id, so the directory
// has no entries that only make sense from inside another file.
func writeArtDoor(out string, rng *rand.Rand, id, material, desc string, drawClosed, drawOpen func(*image.RGBA, *rand.Rand)) artOut {
	closed, open := id+"-closed.png", id+"-open.png"
	drawPicture(out, rng, closed, newCanvas(), drawClosed)
	drawPicture(out, rng, open, newCanvas(), drawOpen)
	sidecar := writeSidecar(out, id, sidecarOut{
		FormatVersion: artFormatVersion, Kind: "door", Material: material,
		Open: open, Closed: closed,
	})
	return artOut{ID: id, Kind: "door", Material: material, Desc: desc,
		Sidecar: sidecar, Pictures: []string{closed, open}}
}

// writeArtObject writes one scenery glyph and NO SIDECAR, which is spec §3.4's
// asymmetry made real: the map's own object entry already carries blocks_sight,
// blocks_move, kind, size and rot, so a bare picture is complete. This is the
// improvisation case the spec celebrates — drop a PNG in, name it from a map.
//
// ON A TRANSPARENT CANVAS (newObjectCanvas, not newCanvas) — unlike a tile, an
// object does not cover its whole square in the real world, so the floor tile
// underneath should show through the corners canvas.ts's drawImage composites
// against whatever was drawn first (planTiles runs before planObjects in
// scene-plan.ts's planScene).
func writeArtObject(out string, rng *rand.Rand, id, desc string, draw func(*image.RGBA, *rand.Rand)) artOut {
	picture := id + ".png"
	drawPicture(out, rng, picture, newObjectCanvas(), draw)
	return artOut{ID: id, Desc: desc, Pictures: []string{picture}}
}

// writeSidecar writes <id>.json and returns its filename.
func writeSidecar(out, id string, sc sidecarOut) string {
	name := id + ".json"
	mustWriteJSON(filepath.Join(out, name), sc)
	return name
}

// writeTile draws one plain (non-door) tile via draw, PNG-encodes it to
// out/file, and returns the packTileOut entry pack.json should carry for
// it. THE STANDARD PACK IS ITS ONLY CALLER since art-is-a-flat-library Task 8
// — the cellar half writes art/, not a manifest — and std_pack.go's header
// says why that half keeps a pack at all.
func writeTile(out string, rng *rand.Rand, name, kind, material, file, desc string, draw func(*image.RGBA, *rand.Rand)) packTileOut {
	drawPicture(out, rng, file, newCanvas(), draw)
	return packTileOut{Name: name, Kind: kind, Material: material, File: file, Desc: desc}
}

// writeDoor draws BOTH of a door tile's pictures (one nature, two pictures)
// and returns the single packTileOut carrying both filenames. Standard-pack
// only, like writeTile.
func writeDoor(out string, rng *rand.Rand, name, material, closedFile, openFile, desc string, drawClosed, drawOpen func(*image.RGBA, *rand.Rand)) packTileOut {
	drawPicture(out, rng, closedFile, newCanvas(), drawClosed)
	drawPicture(out, rng, openFile, newCanvas(), drawOpen)
	return packTileOut{Name: name, Kind: "door", Material: material, FileClosed: closedFile, FileOpen: openFile, Desc: desc}
}

func writeManifest(out string, p packOut) {
	mustWriteJSON(filepath.Join(out, "pack.json"), p)
}

// mustWriteJSON writes one indented JSON file with a trailing newline, or exits
// loudly. ONE construction site for both of this tool's JSON outputs — a
// sidecar and a pack manifest — so the two cannot drift on indentation or on
// the trailing newline, and so the pair of failure branches neither of them can
// exercise is written once rather than twice.
func mustWriteJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: encode %s: %v\n", path, err)
		os.Exit(1)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: write %s: %v\n", path, err)
		os.Exit(1)
	}
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: encode %s: %v\n", path, err)
		os.Exit(1)
	}
}

// --- drawing primitives ----------------------------------------------------
//
// Everything below is intentionally crude: flat fields, per-pixel noise, a
// handful of lines and filled circles. The brief this tool fulfils asks for
// "simple textured surfaces" and "a handful of object glyphs", not
// illustration — see this file's own header comment.

// newCanvas returns a fully OPAQUE size x size RGBA. Every tile picture uses
// this: a floor or wall fills its whole square, and a transparent tile
// would let the page's own background show through as a visible hole where
// the ground should be.
func newCanvas() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, color.RGBA{0, 0, 0, 255})
	return img
}

// newObjectCanvas returns a fully TRANSPARENT size x size RGBA — see
// writeObject's own comment for why an object's picture starts empty
// rather than opaque.
func newObjectCanvas() *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, size, size))
}

func fill(img *image.RGBA, c color.RGBA) {
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// noise jitters every pixel's existing colour by up to +/-amount, clamped
// to [0,255] — cheap per-pixel texture that keeps a flat fill from reading
// as a flat, obviously-computer-generated rectangle.
//
// ONE delta per pixel, applied to all three channels equally, not three
// independent ones: an earlier version jittered R/G/B separately and the
// result read as coloured static rather than dirt or stone — brightness
// noise stays inside the base colour's hue, which is what a real speckled
// surface actually looks like; per-channel noise invents colours that were
// never in the palette.
func noise(img *image.RGBA, rng *rand.Rand, amount int) {
	clamp := func(v, d int) uint8 {
		n := v + d
		if n < 0 {
			return 0
		}
		if n > 255 {
			return 255
		}
		return uint8(n)
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := rng.Intn(2*amount+1) - amount
			c := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{clamp(int(c.R), d), clamp(int(c.G), d), clamp(int(c.B), d), c.A})
		}
	}
}

func hline(img *image.RGBA, y int, c color.RGBA) {
	if y < 0 || y >= size {
		return
	}
	for x := 0; x < size; x++ {
		img.SetRGBA(x, y, c)
	}
}

func vline(img *image.RGBA, x int, c color.RGBA) {
	if x < 0 || x >= size {
		return
	}
	for y := 0; y < size; y++ {
		img.SetRGBA(x, y, c)
	}
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || x >= size || y < 0 || y >= size {
				continue
			}
			img.SetRGBA(x, y, c)
		}
	}
}

// fillCircle fills a hard-edged disc — good enough at 64px for a column or a
// barrel's round profile; anti-aliasing would not read any more clearly at
// this size and is not worth the extra code for what this tool is for.
func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y > r*r {
				continue
			}
			px, py := cx+x, cy+y
			if px < 0 || px >= size || py < 0 || py >= size {
				continue
			}
			img.SetRGBA(px, py, c)
		}
	}
}

// --- tile textures -----------------------------------------------------

// drawMasonry: coursed stone blockwork — horizontal mortar lines every 16px,
// vertical joints every 21px, offset on alternating rows so the joints do
// not line up in a column (the one thing that would make it read as tiles
// rather than brick).
func drawMasonry(img *image.RGBA, rng *rand.Rand) {
	fill(img, color.RGBA{0x6b, 0x6f, 0x76, 0xff})
	noise(img, rng, 10)
	mortar := color.RGBA{0x3a, 0x3c, 0x40, 0xff}
	for y := 0; y < size; y += 16 {
		hline(img, y, mortar)
	}
	for row := 0; row*16 < size; row++ {
		offset := 0
		if row%2 == 1 {
			offset = 10
		}
		for x := offset; x < size; x += 21 {
			for y := row * 16; y < (row+1)*16 && y < size; y++ {
				if x >= 0 && x < size {
					img.SetRGBA(x, y, mortar)
				}
			}
		}
	}
}

// drawEarth: packed dirt — heavy noise plus a scatter of small darker
// pebbles, so it does not read as a flat brown square.
func drawEarth(img *image.RGBA, rng *rand.Rand) {
	fill(img, color.RGBA{0x5b, 0x46, 0x32, 0xff})
	noise(img, rng, 18)
	pebble := color.RGBA{0x2a, 0x1f, 0x15, 0xff}
	for i := 0; i < 16; i++ {
		cx, cy := rng.Intn(size), rng.Intn(size)
		fillCircle(img, cx, cy, 1+rng.Intn(2), pebble)
	}
}

// drawFlagstone: cut paving — light grey, low noise, divided into a handful
// of irregular slabs by darker mortar lines (not a regular grid, which
// would read as a chequerboard rather than quarried stone).
func drawFlagstone(img *image.RGBA, rng *rand.Rand) {
	fill(img, color.RGBA{0x9a, 0x9a, 0x92, 0xff})
	noise(img, rng, 8)
	mortar := color.RGBA{0x66, 0x66, 0x5f, 0xff}
	hline(img, 22+rng.Intn(6), mortar)
	hline(img, 44+rng.Intn(6), mortar)
	vline(img, 20+rng.Intn(6), mortar)
	vline(img, 46+rng.Intn(6), mortar)
}

// --- door pictures -----------------------------------------------------
//
// One nature, two pictures (spec §3.3): drawDoorClosed and drawDoorOpen are
// deliberately built from the SAME plank-and-band motif so the open picture
// reads as "this door, swung", not as a different door.

func doorPlanks(img *image.RGBA, rng *rand.Rand, x0, x1 int) {
	fillRect(img, x0, 0, x1, size, color.RGBA{0x6b, 0x4a, 0x2f, 0xff})
	noise(img, rng, 12)
	plank := color.RGBA{0x4a, 0x31, 0x1e, 0xff}
	for x := x0 + 10; x < x1; x += 10 {
		vline(img, x, plank)
	}
	band := color.RGBA{0x33, 0x35, 0x38, 0xff}
	fillRect(img, x0, 14, x1, 18, band)
	fillRect(img, x0, 46, x1, 50, band)
}

// drawDoorClosed: a full-square banded wooden door.
func drawDoorClosed(img *image.RGBA, rng *rand.Rand) {
	doorPlanks(img, rng, 0, size)
}

// drawDoorOpen: the door swung back against the frame (a narrow plank strip
// on the left) with the rest of the square drawn as the dim passage beyond
// it — visually distinct from closed at a glance, which is the property
// scene-plan.ts's "/open" key exists to make renderable.
func drawDoorOpen(img *image.RGBA, rng *rand.Rand) {
	fill(img, color.RGBA{0x24, 0x24, 0x28, 0xff})
	noise(img, rng, 10)
	doorPlanks(img, rng, 0, 14)
}

// --- object glyphs -------------------------------------------------------

// drawPillar: a round stone column, shaded so it reads as cylindrical
// (lighter on the upper-left, as if lit from above) rather than a flat disc.
func drawPillar(img *image.RGBA, rng *rand.Rand) {
	cx, cy, r := size/2, size/2, size/2-4
	fillCircle(img, cx, cy, r, color.RGBA{0x8a, 0x8c, 0x8f, 0xff})
	fillCircle(img, cx-r/4, cy-r/4, r*2/3, color.RGBA{0xa3, 0xa5, 0xa8, 0xff})
	fillCircle(img, cx+r/3, cy+r/3, r/2, color.RGBA{0x5f, 0x61, 0x64, 0xff})
	noise(img, rng, 6)
}

// drawCrate: a square wooden crate, cross-braced — the archetypal cover
// object (Task 10 brief: "enough that an ambush is possible").
func drawCrate(img *image.RGBA, rng *rand.Rand) {
	fillRect(img, 6, 6, size-6, size-6, color.RGBA{0x8a, 0x5a, 0x2f, 0xff})
	noise(img, rng, 10)
	edge := color.RGBA{0x4a, 0x30, 0x18, 0xff}
	fillRect(img, 6, 6, size-6, 9, edge)
	fillRect(img, 6, size-9, size-6, size-6, edge)
	fillRect(img, 6, 6, 9, size-6, edge)
	fillRect(img, size-9, 6, size-6, size-6, edge)
	// The X brace.
	for i := 6; i < size-6; i++ {
		img.SetRGBA(i, i, edge)
		img.SetRGBA(size-1-i, i, edge)
	}
}

// drawBarrel: an upright barrel — round-topped rectangle with two iron
// bands.
func drawBarrel(img *image.RGBA, rng *rand.Rand) {
	fillRect(img, 14, 8, size-14, size-8, color.RGBA{0x7a, 0x52, 0x2c, 0xff})
	fillCircle(img, size/2, 8, size/2-14, color.RGBA{0x7a, 0x52, 0x2c, 0xff})
	fillCircle(img, size/2, size-8, size/2-14, color.RGBA{0x7a, 0x52, 0x2c, 0xff})
	noise(img, rng, 10)
	band := color.RGBA{0x2e, 0x2f, 0x32, 0xff}
	fillRect(img, 12, 20, size-12, 23, band)
	fillRect(img, 12, size-23, size-12, size-20, band)
}

// drawBrazier: a squat bowl on a tripod stem, with a lit coal glow — the one
// glyph that is deliberately decorative (Task 10 brief lists it as a
// prop, and campaigns/example/maps/cellar.json's own brazier-1 carries blocks_sight/blocks_move
// both false).
func drawBrazier(img *image.RGBA, rng *rand.Rand) {
	stem := color.RGBA{0x33, 0x33, 0x36, 0xff}
	fillRect(img, size/2-2, 30, size/2+2, size-10, stem)
	fillRect(img, 18, size-12, size-18, size-8, stem)
	bowl := color.RGBA{0x3a, 0x3b, 0x3f, 0xff}
	fillCircle(img, size/2, 26, 16, bowl)
	fillCircle(img, size/2, 24, 12, color.RGBA{0xd9, 0x6a, 0x1f, 0xff})
	fillCircle(img, size/2, 22, 6, color.RGBA{0xf2, 0xb0, 0x3d, 0xff})
	noise(img, rng, 6)
}
