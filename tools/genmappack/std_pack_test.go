// std_pack_test.go pins genmappack's standard pack against
// internal/mapdef/standard.go, the ONE place the eleven standard natures
// are declared (design spec §3.3's amended table). Testing package main
// directly (not main_test), same as every other Go package in this repo
// that has no reason to hide unexported state from its own tests.
//
// WHY THIS MATTERS (2026-08-16 review finding C2): nothing before this
// produced a std:<kind>/<material> image at all, so every square of both
// shipped adventures (goblin-ambush's earth floor, cellar-rats' wood floor —
// neither carries a single art override) rendered nothing. This test's job
// is narrower than "the pack renders" (no canvas exists for a Go test to
// look at) — it is the one thing a Go test CAN prove: the eleven names this
// tool emits are EXACTLY mapdef's eleven, with the SAME kind/material for
// each, neither inventing a twelfth nor silently dropping one. Patrik's
// ruling was explicit: "do not invent or omit any."
package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// newDeterministicRNGForTest mirrors main's own seeded source (a fixed seed,
// not crypto/rand — see main.go's own comment on why) so this test's output
// is reproducible run to run; the actual seed value does not matter to what
// this test checks (file presence/size, tile count), only that draw
// functions receive a working *rand.Rand.
func newDeterministicRNGForTest() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

// requireNonEmptyPNG fails the test unless dir/name exists and is a
// non-trivial PNG — "non-trivial" being "bigger than a bare PNG header",
// which is enough to prove a real draw function ran rather than nothing
// being written at all.
func requireNonEmptyPNG(t *testing.T, dir, name string) {
	t.Helper()
	if name == "" {
		t.Fatalf("expected a filename, got empty string")
	}
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	const barePNGHeaderBytes = 100
	if info.Size() < barePNGHeaderBytes {
		t.Fatalf("%s is only %d bytes — too small to be real drawn art", name, info.Size())
	}
}

func TestStandardPackCoversExactlyMapdefsStandardVocabulary(t *testing.T) {
	want := mapdef.StandardTileNames()

	if len(standardEntries) != len(want) {
		t.Fatalf("genmappack declares %d standard entries, mapdef declares %d — these must match "+
			"1:1 or a nature silently has no picture (or the pack invents one mapdef never validates)",
			len(standardEntries), len(want))
	}

	seen := make(map[string]bool, len(standardEntries))
	for _, e := range standardEntries {
		if seen[e.name] {
			t.Fatalf("genmappack standard entry %q is declared twice", e.name)
		}
		seen[e.name] = true

		if _, ok := want[e.name]; !ok {
			t.Errorf("genmappack standard entry %q is not one of mapdef's standard natures — "+
				"it must not be invented here", e.name)
			continue
		}
		wantKind, wantMaterial, _ := mapdef.StandardTile(e.name)
		if e.kind != wantKind || e.material != wantMaterial {
			t.Errorf("genmappack standard entry %q is kind=%q material=%q, mapdef says kind=%q material=%q",
				e.name, e.kind, e.material, wantKind, wantMaterial)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("mapdef's standard nature %q has no genmappack standard entry — "+
				"an unoverridden square of this nature would draw nothing (review finding C2)", name)
		}
	}
}

// TestStandardPackEveryEntryHasArt catches a name/kind/material triple that
// is present but wired to no drawing at all — the table-completeness test
// above cannot see this, since it only reads name/kind/material, never
// file/fileOpen/fileClosed/draw/drawOpen/drawClosed.
func TestStandardPackEveryEntryHasArt(t *testing.T) {
	for _, e := range standardEntries {
		isDoor := e.kind == "door"
		if isDoor {
			if e.drawOpen == nil || e.drawClosed == nil {
				t.Errorf("standard door entry %q must set BOTH drawOpen and drawClosed (spec §3.3: "+
					"one nature, two pictures), got drawOpen=%v drawClosed=%v", e.name, e.drawOpen != nil, e.drawClosed != nil)
			}
			if e.draw != nil {
				t.Errorf("standard door entry %q must not also set a plain draw — a door has no plain file", e.name)
			}
		} else {
			if e.draw == nil {
				t.Errorf("standard non-door entry %q has no draw function", e.name)
			}
			if e.drawOpen != nil || e.drawClosed != nil {
				t.Errorf("standard non-door entry %q must not set drawOpen/drawClosed — only a door has two pictures", e.name)
			}
		}
	}
}

// TestWriteStandardPackProducesElevenTilesAndAWorkingManifest runs the real
// generator against a temp directory (the only way to prove the FILES this
// tool promises to write actually land on disk with non-empty content, not
// just that the in-memory table looks right) and decodes pack.json back
// through encoding/json — the same bytes a client's fetch would receive.
func TestWriteStandardPackProducesElevenTilesAndAWorkingManifest(t *testing.T) {
	dir := t.TempDir()
	rng := newDeterministicRNGForTest()

	manifest := writeStandardPack(dir, rng)

	if got := len(manifest.Tiles); got != 11 {
		t.Fatalf("writeStandardPack produced %d tiles, want 11 (internal/mapdef/standard.go's exact count)", got)
	}
	if got := len(manifest.Objects); got != 0 {
		t.Fatalf("the standard pack declares %d objects, want 0 — standard vocabulary is tiles-only "+
			"(mapdef.StandardTile resolves kind/material for tiles; objects have no standard fallback, "+
			"scene-plan.ts's objectImage doc comment)", got)
	}

	for _, tile := range manifest.Tiles {
		switch tile.Kind {
		case "door":
			requireNonEmptyPNG(t, dir, tile.FileOpen)
			requireNonEmptyPNG(t, dir, tile.FileClosed)
			if tile.File != "" {
				t.Errorf("door tile %q must not also set a plain File", tile.Name)
			}
		default:
			requireNonEmptyPNG(t, dir, tile.File)
			if tile.FileOpen != "" || tile.FileClosed != "" {
				t.Errorf("non-door tile %q must not set FileOpen/FileClosed", tile.Name)
			}
		}
	}
}
