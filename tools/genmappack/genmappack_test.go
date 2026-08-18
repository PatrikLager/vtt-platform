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
// The packs under maps/cellar/tiles and client/public/std-pack are committed
// art, and committed art drifts from its source silently: somebody retouches a
// PNG, or edits a description in pack.json, and from then on the generator and
// the repository disagree with nobody noticing. The art was generated rather
// than taken from a map-building tool whose presets carry no stated licence,
// and that licensing argument only holds while the committed bytes really are
// this program's output. A generator nobody re-runs is a generator nobody can
// trust.

const (
	committedCellar = "../../maps/cellar/tiles"
	committedStd    = "../../client/public/std-pack"
)

func TestGeneratorReproducesBothCommittedPacksByteForByte(t *testing.T) {
	gotCellar, gotStd := t.TempDir(), t.TempDir()
	cellar, std := generate(gotCellar, gotStd)
	if len(cellar.Tiles) == 0 || len(cellar.Objects) == 0 || len(std.Tiles) == 0 {
		t.Fatalf("generated cellar{%d tiles, %d objects} std{%d tiles}; an empty pack is not a pack",
			len(cellar.Tiles), len(cellar.Objects), len(std.Tiles))
	}

	for _, pair := range []struct{ name, committed, got string }{
		{"cellar", committedCellar, gotCellar},
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
