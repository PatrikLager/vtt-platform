package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REPRODUCIBILITY is this generator's load-bearing property, and these tests
// are what make it checkable rather than merely claimed.
//
// The art under client/public/std-pack is committed, and committed art drifts
// from its source silently: somebody retouches a PNG, or edits a description
// in a manifest, and from then on the generator and the repository disagree
// with nobody noticing. The art was generated rather than taken from a
// map-building tool whose presets carry no stated licence, and that
// licensing argument only holds while the committed bytes really are this
// program's output. A generator nobody re-runs is a generator nobody can
// trust.
//
// BOTH HALVES ARE PINNED AGAIN as of 2026-09-02-art-is-a-flat-library Task 8.
// The cellar half was unpinned between Task 7 and Task 8: Task 7 deleted
// campaigns/example/packs/cellar-basics with the pack format itself, so the
// cellar bytes existed only as this generator's output, reproducible from the
// fixed seed and compared against nothing. Task 8 rewrote this half to emit the
// flat art/ layout and committed campaigns/example/art/, which is the artifact
// the "committed bytes ARE this program's output" half needs to be true of.
//
// THE PICTURES DID NOT CHANGE WHEN THE FORMAT DID, and that is checkable rather
// than asserted here: every draw call kept its place in the one seeded rng, so
// campaigns/example/art/masonry-1.png is byte-for-byte the pack's own
// masonry_1.png, renamed. `git show 8a28f34^:campaigns/example/packs/
// cellar-basics/masonry_1.png` is the copy to diff against.

const (
	committedStd = "../../client/public/std-pack"
	// committedArt is the campaign's own flat art/ — this tool's -out default,
	// so a plain `go run ./tools/genmappack` rewrites exactly what this compares
	// against.
	committedArt      = "../../campaigns/example/art"
	committedCampaign = "../../campaigns/example"
)

func TestGeneratorReproducesTheCommittedStandardPackByteForByte(t *testing.T) {
	gotCellar, gotStd := t.TempDir(), t.TempDir()
	cellar, std := generate(gotCellar, gotStd)
	if len(cellar) == 0 || len(std.Tiles) == 0 {
		t.Fatalf("generated %d piece(s) of cellar art and %d standard tile(s); an empty set "+
			"would make every comparison below vacuous", len(cellar), len(std.Tiles))
	}

	for _, pair := range []struct{ name, committed, got string }{
		{"cellar", committedArt, gotCellar},
		{"standard", committedStd, gotStd},
	} {
		t.Run(pair.name, func(t *testing.T) {
			want, err := os.ReadDir(pair.committed)
			if err != nil {
				t.Fatal(err)
			}
			if len(want) == 0 {
				t.Fatalf("%s is committed empty, so this test would pass vacuously", pair.committed)
			}

			for _, e := range want {
				wantBytes, err := os.ReadFile(filepath.Join(pair.committed, e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				gotBytes, err := os.ReadFile(filepath.Join(pair.got, e.Name()))
				if err != nil {
					t.Errorf("the generator did not emit %s, which %s contains: %v",
						e.Name(), pair.committed, err)
					continue
				}
				if !bytes.Equal(gotBytes, wantBytes) {
					t.Errorf("%s differs from the committed pack (%d bytes generated, %d committed). "+
						"Either the generator changed and the pack was not regenerated, or the pack "+
						"was edited by hand. Re-run: go run ./tools/genmappack",
						e.Name(), len(gotBytes), len(wantBytes))
				}
			}

			// The other direction: art the generator emits that nobody
			// committed means a regeneration was never picked up.
			gotEntries, err := os.ReadDir(pair.got)
			if err != nil {
				t.Fatal(err)
			}
			if len(gotEntries) != len(want) {
				t.Errorf("generator emitted %d files, %s has %d — one side carries art the other "+
					"does not", len(gotEntries), pair.committed, len(want))
			}
		})
	}
}

func TestRunningTwiceEmitsIdenticalBytes(t *testing.T) {
	// Separate from the test above because that one also fails when the
	// COMMITTED pack is wrong. This isolates the generator's own determinism —
	// the property main.go's "#nosec G404" justification rests on, and the
	// reason a fixed seed is correct here where crypto/rand would be wrong.
	a1, a2 := t.TempDir(), t.TempDir()
	b1, b2 := t.TempDir(), t.TempDir()
	generate(a1, a2)
	generate(b1, b2)

	for _, pair := range [][2]string{{a1, b1}, {a2, b2}} {
		entries, err := os.ReadDir(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			x, err := os.ReadFile(filepath.Join(pair[0], e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			y, err := os.ReadFile(filepath.Join(pair[1], e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(x, y) {
				t.Fatalf("%s differs between two runs — the generator is not deterministic, so no "+
					"committed pack could ever be verified against it", e.Name())
			}
		}
	}
}

func TestEveryManifestEntryNamesArtThatWasActuallyWritten(t *testing.T) {
	// desc is not decoration for the STANDARD pack: design spec §4.2 says it is
	// what lets a model choose tiles deliberately rather than at random, and
	// spec §1.5's test is an LLM authoring a map from the document and a
	// manifest alone. A manifest naming a file it never wrote fails that
	// silently, at the table.
	//
	// THE CELLAR HALF HAS NO MANIFEST ANY MORE and its descs reach nobody — the
	// flat format has no field for them (main.go's header records the loss). It
	// is still checked here, because a catalogue entry with no sentence saying
	// what it draws is the thing whoever re-tunes these textures has to read the
	// pixels to recover.
	cellarDir, stdDir := t.TempDir(), t.TempDir()
	cellar, std := generate(cellarDir, stdDir)

	t.Run("cellar", func(t *testing.T) {
		if len(cellar) == 0 {
			t.Fatal("no cellar art generated, so this check is vacuous")
		}
		for _, e := range cellar {
			if e.Desc == "" {
				t.Errorf("%q ships with no desc — whoever re-tunes it has only the pixels", e.ID)
			}
			if len(e.Pictures) == 0 {
				t.Errorf("%q is a piece of art with no picture", e.ID)
			}
			for _, f := range append(append([]string{}, e.Pictures...), e.Sidecar) {
				if f == "" {
					continue
				}
				if _, err := os.Stat(filepath.Join(cellarDir, f)); err != nil {
					t.Errorf("%q names %s, which the generator did not write", e.ID, f)
				}
			}
		}
	})

	t.Run("standard", func(t *testing.T) {
		for _, e := range append(append([]packTileOut{}, std.Tiles...), std.Objects...) {
			if e.Desc == "" {
				t.Errorf("%q ships with no desc — a model choosing from this manifest has "+
					"nothing to choose on", e.Name)
			}
			for _, f := range []string{e.File, e.FileOpen, e.FileClosed} {
				if f == "" {
					continue
				}
				if _, err := os.Stat(filepath.Join(stdDir, f)); err != nil {
					t.Errorf("%q names %s, which the generator did not write", e.Name, f)
				}
			}
		}
	})
}

// TestTheCommittedArtIsDrawnAtThisToolsOwnDefault is the check that a plain
// `go run ./tools/genmappack` does not silently redraw the whole campaign at
// another size. -cell-px now sets the picture size, so the default and the
// committed pixels are two numbers that can drift apart, and the byte-for-byte
// test above would report that as nine changed files rather than as one changed
// number.
//
// THE OTHER HALF OF THE PAIR IS IN cmd/vtt:
// TestTheShippedCampaignDeclaresTheCellSizeItsArtWasDrawnAt reads
// campaigns/example/campaign.json through campaigncfg and compares its cell_px
// to these same pixels. It lives there rather than here because this tool may
// not import campaigncfg (.go-arch-lint.yml: genmappack depends on mapdef and
// nothing else), and cmd/vtt is where every other cross-package agreement about
// that number is already asserted.
func TestTheCommittedArtIsDrawnAtThisToolsOwnDefault(t *testing.T) {
	f, err := os.Open(filepath.Join(committedArt, "masonry-1.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != img.Height {
		t.Errorf("masonry-1.png is %dx%d; a grid square of art is square", img.Width, img.Height)
	}
	if img.Width != defaultCellPx {
		t.Errorf("masonry-1.png is %d pixels wide and this tool's -cell-px default is %d; "+
			"a plain `go run ./tools/genmappack` would rewrite the committed art",
			img.Width, defaultCellPx)
	}
}

// TestACellSizeThatWouldWriteAnUndrawablePictureIsRefused covers the one thing
// this tool must catch itself. The campaign format's real bounds (8..1024) are
// enforced by campaigncfg and mapdef when the file is loaded, loudly and with
// the file named, so a -cell-px 2000 run wastes a regeneration and is then
// refused at boot — annoying, and never silent. Zero is the case that IS
// silent: image.NewRGBA accepts an empty rectangle and png.Encode writes it, so
// the campaign would ship pictures every reader accepts and no board can draw.
func TestACellSizeThatWouldWriteAnUndrawablePictureIsRefused(t *testing.T) {
	for _, px := range []int{0, -1, -64} {
		if err := checkCellPx(px); err == nil {
			t.Errorf("checkCellPx(%d) = nil, want a refusal", px)
		}
	}
	for _, px := range []int{1, defaultCellPx, 2000} {
		if err := checkCellPx(px); err != nil {
			t.Errorf("checkCellPx(%d) = %v, want it accepted — this tool is not the "+
				"authority on the campaign format's bounds", px, err)
		}
	}
}

// TestTheSidecarsThisToolWritesAreTheOnesTheSpecShows reads the committed
// sidecars as JSON rather than through artlib, because artlib's decode would
// accept a file with different whitespace, a different key order, or a
// format_version this tool never wrote — and what a human opens art/ to see is
// the file. Design spec §3.4 prints the shape.
func TestTheSidecarsThisToolWritesAreTheOnesTheSpecShows(t *testing.T) {
	for _, tc := range []struct {
		file string
		want map[string]any
	}{
		{"masonry-1.json", map[string]any{
			"format_version": 1.0, "kind": "wall", "material": "stone"}},
		{"cellar-door.json", map[string]any{
			"format_version": 1.0, "kind": "door", "material": "wood",
			"open": "cellar-door-open.png", "closed": "cellar-door-closed.png"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(committedArt, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("%s has keys %v, want exactly %v — an extra key is refused by "+
					"artlib's strict decode and takes the square with it", tc.file, got, tc.want)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("%s[%q] = %v, want %v", tc.file, k, got[k], want)
				}
			}
		})
	}
}

// TestNoPictureIsOrphanedAndNoSidecarIsAlone walks the COMMITTED directory as a
// stranger would, rather than through the generator's own return value: every
// .json has a picture the sidecar can actually name, and art/ is FLAT.
//
// It is deliberately not artlib.Validate — that runs at boot over whatever is
// there. This says what must be true of what this repository SHIPS.
func TestNoPictureIsOrphanedAndNoSidecarIsAlone(t *testing.T) {
	entries, err := os.ReadDir(committedArt)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("campaigns/example/art is empty, so this test would pass vacuously")
	}
	present := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("%s is a directory; art/ is flat, and a subdirectory is a namespace "+
				"and a namespace is a pack (design spec §3.3)", e.Name())
			continue
		}
		present[e.Name()] = true
	}
	for name := range present {
		switch filepath.Ext(name) {
		case ".png":
			continue
		case ".json":
			stem := name[:len(name)-len(".json")]
			var sc sidecarOut
			raw, err := os.ReadFile(filepath.Join(committedArt, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &sc); err != nil {
				t.Fatal(err)
			}
			pictures := []string{stem + ".png"}
			if sc.Kind == "door" {
				pictures = []string{sc.Open, sc.Closed}
			}
			for _, pic := range pictures {
				if !present[pic] {
					t.Errorf("%s names %q, which is not in art/", name, pic)
				}
			}
		default:
			t.Errorf("art/%s is neither a picture nor a sidecar; artlib.IsArtFileName "+
				"refuses to serve it and nothing can name it", name)
		}
	}
}

// TestAPlainRunTargetsExactlyWhatIsCommitted is the assertion the -out default
// never had, and its absence is what let that default outlive the directory it
// pointed at.
//
// Task 7 of 2026-09-02-art-is-a-flat-library deleted
// campaigns/example/packs/cellar-basics; the default kept naming it, so
// `go run ./tools/genmappack` recreated the deleted pack as UNTRACKED files and
// the next `git add -A` would have re-committed what that task removed. Nothing
// failed, because a flag default is not a value any test had looked at.
//
// The paths are compared against the ones this file's other tests read, so the
// two cannot disagree about which directory is "the committed one".
func TestAPlainRunTargetsExactlyWhatIsCommitted(t *testing.T) {
	for _, tc := range []struct{ flagDefault, committed, why string }{
		{defaultOut, committedArt, "a plain run must rewrite the committed art, so `git diff` " +
			"is the check that the generator and the repository still agree"},
		{defaultStdOut, committedStd, "same, for the standard pack the client bundles"},
	} {
		// The test paths are relative to this package; the tool's are relative
		// to the repository root it is run from.
		if want := filepath.Join("..", "..", tc.flagDefault); tc.committed != want {
			t.Errorf("default %q resolves to %q, and this file reads %q — %s",
				tc.flagDefault, want, tc.committed, tc.why)
		}
	}
}

// TestRunRefusesAnUndrawableCellSizeBeforeWritingAnything drives the flag layer
// itself, which os.Exit inside main used to put out of reach: a refused size
// must cost nothing, because a generator that writes half a set and then quits
// leaves a campaign holding pictures at two different resolutions.
func TestRunRefusesAnUndrawableCellSizeBeforeWritingAnything(t *testing.T) {
	restore := size
	t.Cleanup(func() { size = restore })

	out, stdOut := t.TempDir(), t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-cell-px", "0", "-out", out, "-std-out", stdOut}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run(-cell-px 0) = 0, want a non-zero exit: png.Encode writes a 0x0 picture "+
			"every reader accepts and no board can draw (stdout %q)", stdout.String())
	}
	if !strings.Contains(stderr.String(), "one pixel") {
		t.Errorf("stderr = %q, want it to say what is wrong with the number", stderr.String())
	}
	for _, dir := range []string{out, stdOut} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("%s holds %d file(s) after a refused run", dir, len(entries))
		}
	}
}

// TestRunWritesBothSetsAtTheSizeItWasGiven is the other half: the flag reaches
// the pictures. -cell-px is the only input that changes what is drawn rather
// than where it lands, so a run that ignored it would still produce a complete,
// correct-looking art directory — at the wrong resolution, found by whoever
// first put it beside art drawn at 64.
func TestRunWritesBothSetsAtTheSizeItWasGiven(t *testing.T) {
	restore := size
	t.Cleanup(func() { size = restore })

	out, stdOut := t.TempDir(), t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-cell-px", "32", "-out", out, "-std-out", stdOut}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), out) || !strings.Contains(stdout.String(), stdOut) {
		t.Errorf("stdout = %q, want both directories named — it is the only report an "+
			"operator gets that the files went where they meant", stdout.String())
	}
	for _, path := range []string{
		filepath.Join(out, "masonry-1.png"),
		filepath.Join(stdOut, "std_stone_wall.png"),
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if img.Width != 32 || img.Height != 32 {
			t.Errorf("%s is %dx%d, want 32x32 — -cell-px reached neither set", path, img.Width, img.Height)
		}
	}
}

// TestAnUnknownFlagIsRefusedRatherThanIgnored: flag.ContinueOnError reports and
// returns, where flag.ExitOnError used to end the process — so the code this
// returns is the only thing left saying a misspelled flag did not run.
func TestAnUnknownFlagIsRefusedRatherThanIgnored(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-cellpx", "64"}, &stdout, &stderr); code == 0 {
		t.Fatalf("run(-cellpx) = 0; a misspelled flag would then write both COMMITTED "+
			"directories with the defaults (stdout %q)", stdout.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q after a refused run", stdout.String())
	}
}
