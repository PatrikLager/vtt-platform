package main

// maps_test.go covers loadMapsDir/LoadMapsDir (maps.go, maps-as-geometry
// Task 7): the boot-time walker `vtt serve --maps-dir` (composeServer) uses
// to load and validate every standalone map before the server ever accepts
// a connection — the same fail-loud-at-boot posture loadAdventuresDir
// already gives adventures (adventure-format §7, maps-as-geometry design
// spec §4.4).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBootRefusesAnInvalidMapRatherThanServingIt pins adventure-format §7's
// posture applied to standalone maps (maps-as-geometry design spec §4.4):
// one broken map among several stops the WHOLE boot, rather than serving
// the good ones and discovering the bad one at the table. testdata/maps-
// with-one-broken/ carries two subdirectories — "fine" (loads cleanly) and
// "broken" (grid_width: 0, which mapdef.Load itself refuses) — so this
// proves the walk does not silently skip a malformed entry.
func TestBootRefusesAnInvalidMapRatherThanServingIt(t *testing.T) {
	_, err := LoadMapsDir("testdata/maps-with-one-broken")
	if err == nil {
		t.Fatal("a broken map loaded; the table would find out instead of us")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the offending map/file, got: %v", err)
	}
}

// TestLoadMapsDirLoadsAValidMapAndItsPack is the happy path: a map.json
// beside its own tiles/pack.json (spec §4.2's standalone-map convention),
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

// TestLoadMapsDirFailsLoudWhenOverridesDoNotResolveAgainstThePack pins the
// fuller promise of spec §4.4 that mapdef.Load alone cannot check (it takes
// no *Pack argument — see resolve.go's own doc comment): an overrides entry
// naming an art the pack does not define must fail at BOOT, not only once
// something tries to Compile it. loadMapsDir proves this by dry-running
// mapdef.Compile per map (discarding the result) — the same technique
// internal/adventure/load.go's loadScenes already applies to adventure-
// embedded scenes, reused here rather than a second hand-rolled check, per
// Task 4's "one construction site" discipline.
func TestLoadMapsDirFailsLoudWhenOverridesDoNotResolveAgainstThePack(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "shrine")
	if err := os.MkdirAll(filepath.Join(sub, "tiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A pack that does not define "wood-planks-split-3", the art the map
	// below references.
	writeFile(t, filepath.Join(sub, "tiles", "pack.json"), `{
		"id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
		"tiles": [{"name":"some-other-tile","file":"x.png"}]
	}`)
	writeFile(t, filepath.Join(sub, "map.json"), `{
		"id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1, "pack": "mossy-keep",
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"}
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("an override naming art the pack does not define loaded cleanly; " +
			"it should have failed at boot, not waited for someone to Compile it")
	}
	if !strings.Contains(err.Error(), "wood-planks-split-3") {
		t.Errorf("error should name the unresolved art, got: %v", err)
	}
}

// TestLoadMapsDirRefusesDuplicatePackIds guards the namespace GET
// /api/packs/{pack}/{file} addresses by: two map directories declaring the
// SAME pack id would otherwise let the second silently shadow the first's
// images at that route — the identical footgun loadAdventuresDir already
// guards against for adventure ids (adventures.go's own doc comment).
func TestLoadMapsDirRefusesDuplicatePackIds(t *testing.T) {
	dir := t.TempDir()
	writeShrineMap(t, dir, "shrine-a")
	// A second map, different id, whose OWN pack.json happens to declare
	// the same pack id "mossy-keep" as shrine-a's.
	sub := filepath.Join(dir, "shrine-b")
	if err := os.MkdirAll(filepath.Join(sub, "tiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "tiles", "pack.json"), `{
		"id": "mossy-keep", "name": "Mossy Keep (duplicate)", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3","file":"planks_03.png"}]
	}`)
	writeFile(t, filepath.Join(sub, "map.json"), `{
		"id": "shrine-b", "name": "Second Shrine",
		"grid_width": 1, "grid_height": 1, "pack": "mossy-keep",
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"}
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("two map directories declaring the same pack id both loaded; " +
			"the second would silently shadow the first's images at /api/packs/{pack}/...")
	}
	if !strings.Contains(err.Error(), "mossy-keep") {
		t.Errorf("error should name the colliding pack id, got: %v", err)
	}
}

// TestLoadMapsDirRefusesDuplicateMapIds mirrors loadAdventuresDir's own
// TestLoadAdventuresDirDuplicateManifestIDIsBootError: two directories
// declaring the SAME map id would otherwise silently collide in the
// returned map (the second overwriting the first), which is exactly the
// footgun that test guards against for adventures.
func TestLoadMapsDirRefusesDuplicateMapIds(t *testing.T) {
	dir := t.TempDir()
	// Two directory NAMES, but map.json in each declares the same "id".
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a", "map.json"), `{
		"id": "shrine", "name": "First", "grid_width": 1, "grid_height": 1
	}`)
	writeFile(t, filepath.Join(dir, "b", "map.json"), `{
		"id": "shrine", "name": "Second", "grid_width": 1, "grid_height": 1
	}`)

	_, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatal("two map directories declaring the same map id both loaded; " +
			"the second would silently overwrite the first in the returned map")
	}
	if !strings.Contains(err.Error(), "shrine") {
		t.Errorf("error should name the colliding map id, got: %v", err)
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
	sub := filepath.Join(dir, "shrine")
	if err := os.MkdirAll(filepath.Join(sub, "tiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "tiles", "pack.json"), `{
		"name": "Nameless", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3","file":"planks_03.png"}]
	}`)
	writeFile(t, filepath.Join(sub, "map.json"), `{
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
// (adventures_test.go's TestLoadAdventuresDirEmptyDirIsBootError): a typo'd
// or never-synced --maps-dir booting cleanly with zero maps configured is a
// quiet failure, not a loud one — inconsistent with a NONEXISTENT dir, which
// already fails loud via os.ReadDir's own error.
func TestLoadMapsDirEmptyDirIsBootError(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadMapsDir(dir); err == nil {
		t.Fatal("an empty maps dir loaded with zero maps and no error")
	}
}

// --- fixtures ----------------------------------------------------------

// writeShrineMap writes a minimal but VALID map+pack pair (the spec §4.2
// worked example, trimmed) under dir/id, using id as both the directory
// name and the map's own declared id — nothing in loadMapsDir requires the
// two to match (mirroring loadAdventuresDir's own dirOf tracking), but a
// fixture keeping them aligned reads easier.
func writeShrineMap(t *testing.T, dir, id string) {
	t.Helper()
	sub := filepath.Join(dir, id)
	if err := os.MkdirAll(filepath.Join(sub, "tiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "tiles", "pack.json"), `{
		"id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3", "file":"planks_03.png",
		           "kind":"floor", "material":"wood"}]
	}`)
	writeFile(t, filepath.Join(sub, "map.json"), `{
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
