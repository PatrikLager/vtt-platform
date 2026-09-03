package main

// maps_test.go covers loadMapsDir/LoadMapsDir (maps.go, maps-as-geometry
// Task 7; layout changed by Task 3 of the 2026-09-01 create_scene-leaves
// plan — "the kernel serves maps, it does not make them"): the boot-time
// walker composeServer uses (over the campaign directory itself as of that
// plan's Task 5 — there is no --maps-dir flag any more) to load and
// validate every standalone map before the server ever accepts a
// connection — the same fail-loud-at-boot posture loadAdventuresDir
// already gives adventures (adventure-format §7, maps-as-geometry design
// spec §4.4).
//
// SINCE TASK 3: a map is a flat file, <dir>/maps/<id>.json, named by its
// own id — not a directory. A pack is a directory, <dir>/packs/<name>/
// pack.json, in a SIBLING tree — not co-located beside any one map. The
// two are linked only by a map's own "pack" field naming a pack's declared
// id, resolved by loadMapsDir the same way internal/gateway/map.go's
// handleLoadMap resolves it at request time (packs[m.Pack]).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadMapsDirReadsFlatFilesNamedByTheirID pins Task 3's own layout: one
// map is one file, maps/<id>.json, not a directory.
func TestLoadMapsDirReadsFlatFilesNamedByTheirID(t *testing.T) {
	root := t.TempDir()
	writeMap(t, filepath.Join(root, "maps", "cellar.json"), "cellar")

	maps, _, _, err := loadMapsDir(root)
	if err != nil {
		t.Fatalf("loadMapsDir: %v", err)
	}
	if _, ok := maps["cellar"]; !ok {
		t.Fatalf("maps = %v, want a map keyed cellar from maps/cellar.json", keys(maps))
	}
}

// THE FILENAME IS THE ID, and this is the refusal that keeps it true. Without
// it, renaming a file and loading it produces a SECOND scene for the same
// place, silently, while the original stays in the world — because
// mapdef.Compile takes the scene's id from the map's own ID field.
func TestLoadMapsDirRefusesAFilenameThatDisagreesWithTheID(t *testing.T) {
	root := t.TempDir()
	writeMap(t, filepath.Join(root, "maps", "sunken-cellar.json"), "cellar")

	_, _, _, err := loadMapsDir(root)
	if err == nil {
		t.Fatal("want a filename/id mismatch refused")
	}
	for _, want := range []string{"sunken-cellar", "cellar"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to name %q", err, want)
		}
	}
}

// TestBootRefusesAnInvalidMapRatherThanServingIt pins adventure-format §7's
// posture applied to standalone maps (maps-as-geometry design spec §4.4):
// one broken map among several stops the WHOLE boot, rather than serving
// the good ones and discovering the bad one at the table.
// testdata/maps-with-one-broken/maps/ carries two flat files —
// "fine.json" (loads cleanly) and "broken.json" (grid_width: 0, which
// mapdef.Load itself refuses) — so this proves the walk does not silently
// skip a malformed entry.
//
// Asserts on "broken.json", not the weaker "broken": the fixture's own
// PARENT directory is named "maps-with-one-broken", so any error at all —
// even a wrong one, even one that never touched the broken map — contains
// "broken" for a reason that has nothing to do with this test. Checking
// the full filename instead proves the error actually names the offending
// FILE, not merely that some string in some path happened to match.
func TestBootRefusesAnInvalidMapRatherThanServingIt(t *testing.T) {
	_, err := LoadMapsDir("testdata/maps-with-one-broken")
	if err == nil {
		t.Fatal("a broken map loaded; the table would find out instead of us")
	}
	if !strings.Contains(err.Error(), "broken.json") {
		t.Errorf("error should name the offending file broken.json, got: %v", err)
	}
}

// TestLoadMapsDirLoadsAValidMapAndItsPack is the happy path: a map naming a
// pack that lives in the sibling packs/ tree (Task 3's decoupled layout),
// loaded and validated with no error.
func TestLoadMapsDirLoadsAValidMapAndItsPack(t *testing.T) {
	dir := t.TempDir()
	writeShrineMap(t, dir, "shrine")

	maps, err := LoadMapsDir(dir)
	if err != nil {
		t.Fatalf("LoadMapsDir: %v", err)
	}
	m, ok := maps["shrine"]
	if !ok {
		t.Fatalf("LoadMapsDir: want key %q, got %v", "shrine", maps)
	}
	if m.Name != "Obsidian Shrine" {
		t.Errorf("Name = %q, want %q", m.Name, "Obsidian Shrine")
	}
	if m.Pack != "mossy-keep" {
		t.Errorf("Pack = %q, want %q", m.Pack, "mossy-keep")
	}
}

// TestLoadMapsDirFailsLoudWhenArtCannotBeRead pins the fuller promise of
// maps-as-geometry spec §4.4 that mapdef.Load alone cannot check (it reads no
// art — see resolve.go's own doc comment): an overrides entry naming art that
// is installed and cannot be read must fail at BOOT, not only once something
// tries to Compile it. loadMapsDir proves this by dry-running mapdef.Compile
// per map against the campaign's own art/ — the same directory
// internal/gateway/map.go's handleLoadMap will resolve against at request
// time, and the same technique internal/adventure/load.go's loadScenes
// applies to adventure-embedded scenes, reused here rather than a second
// hand-rolled check, per maps-as-geometry Task 4's "one construction site"
// discipline.
//
// It used to drive art the PACK did not define. That case no longer fails
// anything: 2026-09-02-art-is-a-flat-library spec §4 degrades an art
// reference with no file behind it to a plain square and one warning, so the
// only art-side boot refusal left is a piece that exists and cannot be read.
// TestABootLoadedMapWhoseArtIsNotInstalledStillBoots below is the other half
// of that pair, and it is the half worth having: without it, "fails loud"
// could be satisfied by a loader that refuses everything.
func TestLoadMapsDirFailsLoudWhenArtCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "art"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Art that IS installed and declares a format this server does not
	// understand — the file is there, so this is a defect to fix rather than
	// a square to draw plain.
	writeFile(t, filepath.Join(dir, "art", "wood-planks-split-3.json"),
		`{"format_version": 99, "kind": "floor", "material": "wood"}`)
	writeFile(t, filepath.Join(dir, "art", "wood-planks-split-3.png"), "fake-png")
	writeFile(t, filepath.Join(dir, "maps", "shrine.json"), `{
		"format_version": 1,
		"id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1,
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"}
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("an override naming art that cannot be read loaded cleanly; " +
			"it should have failed at boot, not waited for someone to Compile it")
	}
	if !strings.Contains(err.Error(), "wood-planks-split-3") {
		t.Errorf("error should name the art it could not read, got: %v", err)
	}
}

// TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists is the BOOT half
// of Patrik's ruling of 2026-09-03, and it drives composeServer rather than
// LoadMapsDir for a reason the first version of this test learned the hard way.
//
// THE SUBTEST THAT MATTERS IS "no maps yet". The check originally lived inside
// loadMapsDir, and composeServer calls loadMapsDir only when campaignPath/maps
// EXISTS — so a campaign with a broken art/ and no map yet booted CLEAN and the
// check never ran. Measured: broken art/ with no maps/ → composeServer returned
// nil; broken art/ with maps/ present → refused. That is design spec §1's own
// sentence, rebuilt one directory over: "composeServer gates the pack load on
// maps/ existing, so a campaign with art and no map yet boots with no art at
// all." A fresh campaign IS the improvisation case, so it is precisely the one
// that must not slip through. Only a test through composeServer can see this;
// a LoadMapsDir test passes either way, which is why that is not what this is.
//
// The asymmetry with request time is deliberate and is the other half of the
// ruling: internal/mapdef's TestAnUnopenableArtDirectoryDegradesAtResolveTime
// pins that the same condition only warns once a DM is at the table.
//
// A plain FILE where art/ belongs, rather than a mode: a permissions fixture
// passes trivially for a process running as root, and CI containers often do.
func TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withMaps bool
	}{
		{"no maps yet — the campaign this sub-project exists for", false},
		{"maps installed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := filepath.Join(t.TempDir(), "campaign")
			if err := os.MkdirAll(campaignPath, 0o750); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(campaignPath, "art"), "a plain file where art/ belongs")
			if tc.withMaps {
				if err := os.MkdirAll(filepath.Join(campaignPath, "maps"), 0o750); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(campaignPath, "maps", "shrine.json"), `{
					"format_version": 1, "id": "shrine", "name": "Obsidian Shrine",
					"grid_width": 1, "grid_height": 1, "tiles": {"0,0":"wood"}
				}`)
			}

			_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
			if err == nil {
				if closeErr := closeFn(); closeErr != nil {
					t.Error(closeErr)
				}
				t.Fatal("a campaign whose art/ cannot be opened started a server; an " +
					"operator is the one person who can fix that, and only at boot are " +
					"they looking")
			}
			if !strings.Contains(err.Error(), filepath.Join(campaignPath, "art")) {
				t.Errorf("error should name the directory an operator has to go and fix, got: %v", err)
			}
		})
	}
}

// TestAnAbsentArtDirectoryIsNotABootFailure is the other side of the same
// check, and without it "fails loud" would be satisfied by a walk that refuses
// every campaign: a campaign that has installed no art at all is the ordinary
// starting state, and reproducing sub-project 15's boot-order defect — a guard
// on one directory silently gating another's loading — is exactly what this
// sub-project exists to remove. It goes through composeServer for the same
// reason the test above does.
func TestAnAbsentArtDirectoryIsNotABootFailure(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign")
	if err := os.MkdirAll(filepath.Join(campaignPath, "maps"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(campaignPath, "maps", "shrine.json"), `{
		"format_version": 1, "id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1, "tiles": {"0,0":"wood"}
	}`)

	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer: %v — no art/ at all is ordinary and must boot", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}

// TestABootLoadedMapWhoseArtIsNotInstalledStillBoots is spec §8's keystone at
// the boot walk: "A map loads with every art reference unresolvable, and every
// square renders from its kind... it fails if anything in the load path still
// treats art as required."
//
// It replaces TestLoadMapsDirFailsLoudWhenObjectArtDoesNotResolveAgainstThePack,
// and carries forward the fixture that test was built around — whole-branch
// review finding I1's exact reproduction, an object whose art is misspelled
// one letter ("boulder-mosy-2"), which used to boot cleanly and then block its
// square forever with nothing drawn there. The verdict on that fixture has
// inverted deliberately: spec §4 rules that the object STAYS, because an
// object is a thing in the world before it is a picture. What I1 was really
// about was the SILENCE, and the silence is closed elsewhere now — Compile
// returns a warning naming the reference (internal/mapdef's
// TestCompileDegradesAnObjectWhoseArtIsNotInstalled), and load_map carries it
// to the DM (internal/gateway's TestALoadMapWarningReachesTheIssuer). This
// walk still discards warnings, which is why it can only assert the boot.
func TestABootLoadedMapWhoseArtIsNotInstalledStillBoots(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No art/ directory at all: a campaign that has installed no art yet.
	writeFile(t, filepath.Join(dir, "maps", "shrine.json"), `{
		"format_version": 1,
		"id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1,
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"},
		"objects": [{"id":"boulder-1","kind":"boulder","at":[0,0],"size":[1,1],
		             "blocks_move":true,"art":"boulder-mosy-2"}]
	}`)

	maps, err := LoadMapsDir(dir)
	if err != nil {
		t.Fatalf("LoadMapsDir: %v — a campaign with no art installed still boots (spec §4)", err)
	}
	if _, ok := maps["shrine"]; !ok {
		t.Fatalf("maps = %v, want the shrine map loaded", maps)
	}
}

// TestLoadMapsDirRefusesDuplicatePackIds guards the namespace GET
// /api/packs/{pack}/{file} addresses by: two packs/ subdirectories
// declaring the SAME pack id would otherwise let the second silently
// shadow the first's images at that route — the identical footgun
// loadAdventuresDir already guards against for adventure ids
// (adventures.go's own doc comment).
func TestLoadMapsDirRefusesDuplicatePackIds(t *testing.T) {
	dir := t.TempDir()
	writeShrineMap(t, dir, "shrine-a")
	// A second pack directory, elsewhere in the packs/ tree, whose OWN
	// pack.json happens to declare the same pack id "mossy-keep" as
	// shrine-a's.
	if err := os.MkdirAll(filepath.Join(dir, "packs", "mossy-keep-duplicate"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "packs", "mossy-keep-duplicate", "pack.json"), `{
		"format_version": 1,
		"id": "mossy-keep", "name": "Mossy Keep (duplicate)", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3","file":"planks_03.png"}]
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("two packs/ subdirectories declaring the same pack id both loaded; " +
			"the second would silently shadow the first's images at /api/packs/{pack}/...")
	}
	if !strings.Contains(err.Error(), "mossy-keep") {
		t.Errorf("error should name the colliding pack id, got: %v", err)
	}
}

// TestLoadMapsDirRefusesAnUnnamedPack pins a check this task's OWN routing
// needs that mapdef.LoadPack itself does not make (packTileMap requires
// non-empty NAMES for tiles/objects, but never checks the pack's own
// top-level id): GET /api/packs/{pack}/{file} addresses a pack by that id,
// so a pack.json with no id at all cannot be served by any URL — refused at
// boot rather than silently keyed under "" and only unreachable-in-practice.
func TestLoadMapsDirRefusesAnUnnamedPack(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "packs", "nameless"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "packs", "nameless", "pack.json"), `{
		"format_version": 1,
		"name": "Nameless", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3","file":"planks_03.png"}]
	}`)
	writeFile(t, filepath.Join(dir, "maps", "shrine.json"), `{
		"format_version": 1,
		"id": "shrine", "name": "Shrine",
		"grid_width": 1, "grid_height": 1
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("a pack.json with no declared id loaded without error; " +
			"nothing could ever address it at /api/packs/{pack}/...")
	}
}

// TestLoadMapsDirEmptyDirIsBootError mirrors loadAdventuresDir's own F4 fix
// (adventures_test.go's TestLoadAdventuresDirEmptyDirIsBootError): a maps/
// that exists but was never populated (an empty mkdir, or a sync that
// dropped its files but not itself) booting cleanly with zero maps
// configured is a quiet failure, not a loud one — inconsistent with a
// NONEXISTENT dir, which already fails loud via os.ReadDir's own error
// (and which composeServer's own caller-side guard treats as "nothing
// installed yet", not this function's concern — 2026-09-01-create-
// scene-leaves Task 5).
func TestLoadMapsDirEmptyDirIsBootError(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadMapsDir(dir); err == nil {
		t.Fatal("an empty maps dir loaded with zero maps and no error")
	}
}

// --- fixtures ----------------------------------------------------------

// writeMap writes a minimal but VALID standalone map to path, declaring id
// as its own "id" field: format_version (Task 1 made it mandatory) and one
// REAL standard-vocabulary tile name, "stone" (internal/mapdef/standard.go)
// — never a bare kind like "floor", which CheckTileNamesKnown refuses on
// its own and would make a "must be refused" test built on this helper pin
// the WRONG check for the wrong reason. No pack is declared or needed: a
// standard tile with no override resolves on its own.
func writeMap(t *testing.T, path, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{
		"format_version": 1,
		"id": "`+id+`", "name": "Test Map",
		"grid_width": 1, "grid_height": 1,
		"tiles": {"0,0":"stone"}
	}`)
}

// keys returns m's keys as a slice, so a t.Fatalf can print what
// loadMapsDir actually returned. Only the PRINT depends on this — no
// assertion in this file depends on Go's randomized map iteration order.
func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// writeShrineMap writes a minimal but VALID map+pack pair (the spec §4.2
// worked example, trimmed) as maps/<id>.json and packs/mossy-keep/pack.json
// under dir — Task 3's decoupled layout. id names the map FILE (a map's
// filename IS its id, per the refusal this file pins above); the pack
// directory's own name need not match pack.ID — only pack.json's own "id"
// field does, and loadMapsDir keys packs by that field, never by directory
// name (mirroring loadAdventuresDir's own dirOf tracking).
func writeShrineMap(t *testing.T, dir, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "packs", "mossy-keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "packs", "mossy-keep", "pack.json"), `{
		"format_version": 1,
		"id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3", "file":"planks_03.png",
		           "kind":"floor", "material":"wood"}]
	}`)
	writeFile(t, filepath.Join(dir, "maps", id+".json"), `{
		"format_version": 1,
		"id": "`+id+`", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1, "pack": "mossy-keep",
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"}
	}`)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestBootRefusesAFileWhoseNameTrimsToNoIdAndNamesIt closes the gap review
// found in round 1 of 2026-09-01-create-scene-leaves Task 6: that round's
// mapdef.idIsAFilename claimed its refusal was unreachable from boot,
// "because at boot every id comes from a directory entry's own name". This
// walk derives an id by trimming ".json" off that name, so a file called
// ".json" yields an id of "" and lands on exactly that refusal. The
// operator is looking at a directory listing, not at an id they typed, so
// the message has to name the FILE — round 1's named only the empty id.
func TestBootRefusesAFileWhoseNameTrimsToNoIdAndNamesIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "maps", ".json"), `{
		"format_version": 1, "id": "", "name": "Nameless",
		"grid_width": 1, "grid_height": 1, "tiles": {"0,0":"stone"}
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("a file named .json booted as a map; its id would be the empty string")
	}
	if !strings.Contains(err.Error(), `".json"`) {
		t.Errorf("error = %q, want it to name the file an operator can actually see", err)
	}
}
