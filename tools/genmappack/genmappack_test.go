package main

import (
	"bytes"
	"os"
	"path/filepath"
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
// THE CELLAR HALF OF THIS CHECK IS CURRENTLY UNPINNED, and that is a real gap
// with an owner rather than a decision. campaigns/example/packs/cellar-basics
// was the OTHER committed output, and 2026-09-02-art-is-a-flat-library Task 7
// deleted it with the pack format itself — its bytes now exist only as this
// generator's output, reproducible from the fixed seed but compared against
// nothing. Task 8 of that plan rewrites this generator to emit the flat art/
// layout and commits campaigns/example/art/; it must add that directory back to
// the pairs below, or the licensing argument above holds for one of the two
// shipped art sets and not the other.
//
// WHAT IS STILL PINNED FOR THE CELLAR ART, so the gap is exactly one property
// wide: TestRunningTwiceEmitsIdenticalBytes proves the generator is
// deterministic for BOTH halves (it compares two temp-dir runs, needing nothing
// committed), and TestEveryManifestEntryNamesArtThatWasActuallyWritten proves
// every cellar entry names a file that was really written. What is gone is
// "the committed bytes ARE this program's output", which is the half that
// needs a committed artifact to be true of.

const committedStd = "../../client/public/std-pack"

func TestGeneratorReproducesTheCommittedStandardPackByteForByte(t *testing.T) {
	gotCellar, gotStd := t.TempDir(), t.TempDir()
	cellar, std := generate(gotCellar, gotStd)
	if len(cellar.Tiles) == 0 || len(cellar.Objects) == 0 || len(std.Tiles) == 0 {
		t.Fatalf("generated cellar{%d tiles, %d objects} std{%d tiles}; an empty pack is not a pack",
			len(cellar.Tiles), len(cellar.Objects), len(std.Tiles))
	}

	for _, pair := range []struct{ name, committed, got string }{
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
	// desc is not decoration: design spec §4.2 says it is what lets a model
	// choose tiles deliberately rather than at random, and spec §1.5's test is
	// an LLM authoring a map from the document and a manifest alone. A manifest
	// naming a file it never wrote fails that silently, at the table.
	cellarDir, stdDir := t.TempDir(), t.TempDir()
	cellar, std := generate(cellarDir, stdDir)

	check := func(t *testing.T, dir string, p packOut) {
		t.Helper()
		for _, e := range append(append([]packTileOut{}, p.Tiles...), p.Objects...) {
			if e.Desc == "" {
				t.Errorf("%q ships with no desc — a model choosing from this manifest has "+
					"nothing to choose on", e.Name)
			}
			for _, f := range []string{e.File, e.FileOpen, e.FileClosed} {
				if f == "" {
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
					t.Errorf("%q names %s, which the generator did not write", e.Name, f)
				}
			}
		}
	}
	t.Run("cellar", func(t *testing.T) { check(t, cellarDir, cellar) })
	t.Run("standard", func(t *testing.T) { check(t, stdDir, std) })
}
