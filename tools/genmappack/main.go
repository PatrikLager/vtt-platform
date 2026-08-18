// Command genmappack generates TWO packs: maps/cellar/tiles, the starter
// pack Task 10 of the maps-as-geometry arc ships as maps/cellar's own art
// (design spec §4.2, §1.5's "a pack manifest, with no other help"), and
// (added for review finding C2, 2026-08-16) client/public/std-pack, a
// baseline picture for every one of internal/mapdef/standard.go's eleven
// standard natures — see std_pack.go's own header comment for why a square
// with no art override needs this at all, and why it ships from a different
// place than the cellar pack does.
//
// WHY GENERATED RATHER THAN DRAWN OR FETCHED. Patrik's ruling: copy no
// image, fetch art from nowhere else on the web (fantasymapbuilder.com's
// organisation is worth studying; its presets are credited to a named
// artist with no stated license, and are therefore off limits). Generating
// procedurally from Go's standard image/color/draw/png — no new dependency;
// this repo already builds with Go, and Pillow was checked and rejected for
// exactly that reason — buys three things at once: provenance is
// unambiguous (every pixel traces to the code below, not to a URL), the
// pack is RE-TUNABLE rather than an opaque binary (change a colour, rerun,
// diff the PNGs), and this file doubles as a worked example of what a pack
// author must actually produce — pack.json plus images beside it, in the
// exact shape internal/mapdef/load.go's LoadPack expects.
//
// Deliberately simple: flat colour fields, per-pixel noise, and a few lines
// or a filled circle. "Simple textured surfaces... and a handful of object
// glyphs" (Task 10 brief) is the whole brief; nothing here is trying to be
// good art, only to be UNAMBIGUOUS art — a wall reads as a wall, a crate
// reads as a crate, at 64px in a browser tile.
//
// Run: go run ./tools/genmappack [-out maps/cellar/tiles] [-std-out client/public/std-pack]
// Both packs are (re)written on every run — there is no flag to write only one.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
)

// size is the pack's cell_px (design spec §4.2's pack.json field): every
// image this tool emits is size x size. 64, per the Task 10 brief — "Keep
// the images small — a few KB each, 64px is plenty" — big enough to read as
// a distinct texture at typical camera scale, small enough that the whole
// pack stays a handful of KB on disk and over the wire.
const size = 64

// seed is fixed so a rerun with unchanged code reproduces byte-identical
// PNGs — REPRODUCIBLE, this file's own header comment's second promise. A
// random seed would make every rerun a silent diff even when nothing about
// the art was meant to change, defeating "generate the pack from it" as a
// workflow: a pack author reruns this to RE-TUNE one texture, not to
// discover that every other file also moved.
const seed = 20260812

// --- pack.json's on-disk shape --------------------------------------------
//
// Mirrors internal/mapdef/load.go's packTileJSON/packJSON field-for-field
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
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	CellPx  int           `json:"cell_px"`
	Tiles   []packTileOut `json:"tiles"`
	Objects []packTileOut `json:"objects"`
}

func main() {
	out := flag.String("out", "maps/cellar/tiles", "directory to write the cellar starter pack's pack.json and images into")
	stdOut := flag.String("std-out", "client/public/std-pack",
		"directory to write the standard-vocabulary baseline pack's pack.json and images into "+
			"(see std_pack.go's header comment for why this ships from the client bundle, not a "+
			"GET /api/packs/{pack}/... route)")
	flag.Parse()

	cellar, std := generate(*out, *stdOut)
	fmt.Printf("genmappack: wrote %d tile(s), %d object(s) and pack.json into %s\n",
		len(cellar.Tiles), len(cellar.Objects), *out)
	fmt.Printf("genmappack: wrote %d standard tile(s) and pack.json into %s\n",
		len(std.Tiles), *stdOut)
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
func generate(out, stdOut string) (packOut, packOut) {
	mustMkdirAll(out)

	// #nosec G404 -- math/rand is REQUIRED here, not a shortcut. The seed is a
	// fixed constant so this generator is REPRODUCIBLE: re-running it must emit
	// byte-identical images, or every regeneration would dirty the repo and the
	// committed pack could never be verified against its source. crypto/rand
	// would make the output different every run, which is the opposite of what
	// a committed asset generator needs. Nothing here is a secret.
	rng := rand.New(rand.NewSource(seed))

	tiles := []packTileOut{
		writeTile(out, rng, "masonry-1", "wall", "stone", "masonry_1.png",
			"coursed stone blockwork, the standard wall face for the cellar pack",
			drawMasonry),
		writeTile(out, rng, "earth-1", "floor", "earth", "earth_1.png",
			"packed dirt floor, uneven and speckled with small stones",
			drawEarth),
		writeTile(out, rng, "flagstone-1", "floor", "stone", "flagstone_1.png",
			"cut flagstone paving, mortared in irregular slabs",
			drawFlagstone),
	}
	tiles = append(tiles, writeDoor(out, rng, "cellar-door", "wood", "cellar_door_closed.png", "cellar_door_open.png",
		"a banded wooden door; closed and open pictures are the SAME nature (spec §3.3) — "+
			"opening it changes only which of these two files the renderer picks, never the tile's kind",
		drawDoorClosed, drawDoorOpen))

	objects := []packTileOut{
		writeObject(out, rng, "pillar-stone", "pillar_stone.png",
			"a round stone column, wide enough to block a square's line of sight and passage",
			drawPillar),
		writeObject(out, rng, "crate-wood", "crate_wood.png",
			"a stacked wooden shipping crate — good cover, or just clutter, depending on how it is placed",
			drawCrate),
		writeObject(out, rng, "barrel", "barrel.png",
			"an upright wine barrel, banded in iron",
			drawBarrel),
		writeObject(out, rng, "brazier", "brazier.png",
			"a standing iron brazier, coals lit — decorative: it blocks neither sight nor movement",
			drawBrazier),
	}

	manifest := packOut{
		ID:      "cellar-basics",
		Name:    "Cellar Basics",
		CellPx:  size,
		Tiles:   tiles,
		Objects: objects,
	}
	writeManifest(out, manifest)

	// The standard-vocabulary baseline pack (review finding C2, std_pack.go's
	// own header comment for the full why/where). rng is NOT re-seeded here —
	// threaded straight from the cellar pack's own draws above, so a rerun of
	// the WHOLE tool is what the fixed seed reproduces byte-identically, not
	// just one half of it in isolation.
	stdManifest := writeStandardPack(stdOut, rng)
	return manifest, stdManifest
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

// writeTile draws one plain (non-door) tile via draw, PNG-encodes it to
// out/file, and returns the packTileOut entry pack.json should carry for
// it. rng is threaded through from main's single seeded source rather than
// re-seeded per call, so the WHOLE run — not just one texture in
// isolation — is what a fixed seed reproduces.
func writeTile(out string, rng *rand.Rand, name, kind, material, file, desc string, draw func(*image.RGBA, *rand.Rand)) packTileOut {
	img := newCanvas()
	draw(img, rng)
	writePNG(filepath.Join(out, file), img)
	return packTileOut{Name: name, Kind: kind, Material: material, File: file, Desc: desc}
}

// writeDoor draws BOTH of a door tile's pictures (design spec §3.3: one
// nature, two pictures — file_open/file_closed, no plain file) and returns
// the single packTileOut carrying both filenames.
func writeDoor(out string, rng *rand.Rand, name, material, closedFile, openFile, desc string, drawClosed, drawOpen func(*image.RGBA, *rand.Rand)) packTileOut {
	closed := newCanvas()
	drawClosed(closed, rng)
	writePNG(filepath.Join(out, closedFile), closed)

	open := newCanvas()
	drawOpen(open, rng)
	writePNG(filepath.Join(out, openFile), open)

	return packTileOut{Name: name, Kind: "door", Material: material, FileClosed: closedFile, FileOpen: openFile, Desc: desc}
}

// writeObject draws one scenery glyph ON A TRANSPARENT CANVAS (newObjectCanvas,
// not newCanvas) — unlike a tile, an object does not cover its whole square
// in the real world, so the floor tile underneath should show through the
// corners canvas.ts's drawImage composites against whatever was drawn
// first (planTiles runs before planObjects in scene-plan.ts's planScene).
func writeObject(out string, rng *rand.Rand, name, file, desc string, draw func(*image.RGBA, *rand.Rand)) packTileOut {
	img := newObjectCanvas()
	draw(img, rng)
	writePNG(filepath.Join(out, file), img)
	return packTileOut{Name: name, File: file, Desc: desc}
}

func writeManifest(out string, p packOut) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: encode pack.json: %v\n", err)
		os.Exit(1)
	}
	b = append(b, '\n')
	if err := os.WriteFile(filepath.Join(out, "pack.json"), b, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "genmappack: write pack.json: %v\n", err)
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
// prop, and maps/cellar's own brazier-1 carries blocks_sight/blocks_move
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
