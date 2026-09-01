// maps.go is the shared boot-time maps-directory loader (maps-as-geometry
// Task 7; layout changed by Task 3 of the create_scene-leaves plan, 2026-
// 09-01, "the kernel serves maps, it does not make them"): composeServer
// (serve_compose.go) walks the campaign directory itself and calls
// mapdef.Load/mapdef.LoadPack — there is no --maps-dir flag any more as of
// Task 5 of that same plan, maps belong to the campaign that uses them —
// factored here the same way adventures.go factors loadAdventuresDir, and
// sharing its "fail loud at boot" posture (adventure-format §7, applied to
// maps by maps-as-geometry design spec §4.4).
//
// SINCE TASK 3, maps and packs are two SEPARATE trees under dir, not one
// map-per-subdirectory: every "<dir>/maps/<id>.json" is one standalone map,
// named by its own filename (see the ID-mismatch refusal below — this is
// the whole point of Task 3, not an incidental rule: mapdef.Compile takes a
// SceneCreated's id from the map's own ID field, so if the filename alone
// governed identity, renaming a file and reloading it would silently mint
// a SECOND scene for the same place while the original stayed in the
// world); every "<dir>/packs/<name>/pack.json" is one pack, keyed by its
// OWN declared id (packs/<name>'s directory name need not match — only the
// pack.json "id" field does, exactly as before Task 3). The two trees are
// independent: a map names the pack it wants via its own "pack" field, and
// this loader resolves that reference by ID lookup (packs[m.Pack]) — the
// SAME lookup handleLoadMap makes at request time (internal/gateway/
// map.go's own "s.packs[m.Pack]... may legally be nil/absent for a map
// with no overrides" comment), so the boot-time dry run below fails on
// exactly the references handleLoadMap would fail on later, and cannot
// drift from it.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// loadMapsDir is the full walk: every packs/<name>/pack.json first (so a
// map's pack lookup below always sees the complete pack set), then every
// maps/<id>.json plus a boot-time dry run of mapdef.Compile per map
// (discarding the result) so an overrides entry that does not resolve
// against its pack fails HERE rather than only once something eventually
// calls Compile for real — mirroring loadScenes' identical dry-run of
// mapdef.BuildSceneCreated for adventure-embedded scenes
// (internal/adventure/load.go), and reusing Compile itself rather than
// inventing a second validation path, per Task 4's "one construction site"
// discipline.
//
// Each pack's fs.FS comes from os.OpenRoot(packDir).FS(), NOT os.DirFS —
// this was fixed after review found the difference load-bearing.
// os.DirFS's own doc comment says plainly what it does not do: "if
// /prefix/file is a symbolic link pointing outside the /prefix tree, then
// using DirFS does not stop the access any more than using os.Open does...
// DirFS is therefore not a general substitute for a chroot-style security
// mechanism." A community-authored pack containing a symlink at, say,
// tiles/evil.png pointing at the campaign's own SQLite file would have
// served that file's bytes to any authenticated participant, over an
// ordinary GET with no ".." anywhere in it — fs.ValidPath (what stops a
// dotdot traversal, see WithPackFiles' doc comment) never even engages,
// because a symlink target is a different mechanism entirely. os.Root
// (go1.24+; this repo is on go1.26) is the primitive that actually closes
// this: "Methods on Root will follow symbolic links, but symbolic links may
// not reference a location outside the root" (go doc os.Root). OpenRoot's
// own error is handled as a boot error below, consistent with every other
// failure this function reports — a pack directory this process cannot
// open at all is exactly as fail-loud-worthy as one whose pack.json cannot
// be parsed.
//
// Maps and packs are addressed by two DIFFERENT keys, both the thing's own
// declared id rather than any directory or file name (mirroring
// loadAdventuresDir's own dirOf tracking): maps by Map.ID (gateway.Server.
// WithMaps' first argument, GET /api/maps), packs by Pack.ID (WithMaps'
// second argument and WithPackFiles' fs.FS map, GET /api/packs/{pack}/
// {file}). For PACKS a directory-name collision remains possible (two
// packs/ subdirectories may declare the same id) and is a boot error
// naming both directories — the same footgun loadAdventuresDir already
// guards against for adventure ids, sharper here because packFS is keyed
// globally and a silent collision would let one pack directory shadow
// another's images at the SAME route. For MAPS, since Task 3, a duplicate
// id is no longer reachable through this walk at all: the filename-is-the-
// id refusal below means a map's key is always exactly its own filename
// (minus ".json"), and a filesystem cannot hold two entries of the same
// name in one directory — the collision loadAdventuresDir's map-id check
// guards against for adventures cannot arise here by construction, so
// nothing analogous is checked (or tested) for maps.
//
// composeServer calls loadMapsDir directly (not the exported LoadMapsDir
// below) so it does not have to re-derive packs from a second filesystem
// walk.
func loadMapsDir(dir string) (maps map[string]*mapdef.Map, packs map[string]*mapdef.Pack, packFS map[string]fs.FS, err error) {
	packsDir := filepath.Join(dir, "packs")
	mapsDir := filepath.Join(dir, "maps")

	packs = make(map[string]*mapdef.Pack)
	packFS = make(map[string]fs.FS)
	packDirOf := make(map[string]string)

	// packsDir is OPTIONAL: a maps dir whose every map uses only the
	// standard vocabulary (no overrides at all) legitimately has no packs/
	// tree — mirroring the pre-Task-3 loader's own tolerance of a map with
	// no beside-it tiles/pack.json. Any OTHER read failure (permissions,
	// not-a-directory) still fails loud.
	packEntries, statErr := os.ReadDir(packsDir)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, nil, nil, fmt.Errorf("read packs dir: %w", statErr)
	}
	for _, e := range packEntries {
		packDir := filepath.Join(packsDir, e.Name())
		info, statErr := os.Stat(packDir) // follows symlinks, matching loadAdventuresDir
		if statErr != nil {
			return nil, nil, nil, fmt.Errorf("stat %s: %w", packDir, statErr)
		}
		if !info.IsDir() {
			continue
		}

		pack, loadErr := mapdef.LoadPack(packDir)
		if loadErr != nil {
			return nil, nil, nil, loadErr
		}
		if pack.ID == "" {
			return nil, nil, nil, fmt.Errorf(
				"packs dir %s: %s: pack id must not be empty (GET /api/packs/{pack}/... has nothing to address it by)",
				packsDir, filepath.Join(packDir, "pack.json"))
		}
		if prior, dup := packDirOf[pack.ID]; dup {
			return nil, nil, nil, fmt.Errorf(
				"packs dir %s: pack id %q declared by both %s and %s", packsDir, pack.ID, prior, packDir)
		}
		// os.OpenRoot, not os.DirFS: see this function's own doc comment
		// for why plain DirFS is not a symlink-safe boundary. Root.FS()
		// (go1.24+) gives an fs.FS whose Open refuses a symlink that
		// resolves outside packDir, not merely a literal ".." in the name.
		root, rootErr := os.OpenRoot(packDir)
		if rootErr != nil {
			return nil, nil, nil, fmt.Errorf("packs dir %s: open pack dir %s: %w", packsDir, packDir, rootErr)
		}
		packs[pack.ID] = pack
		packFS[pack.ID] = root.FS()
		packDirOf[pack.ID] = packDir
	}

	mapEntries, err := os.ReadDir(mapsDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read maps dir: %w", err)
	}
	maps = make(map[string]*mapdef.Map, len(mapEntries))

	for _, e := range mapEntries {
		mapPath := filepath.Join(mapsDir, e.Name())
		info, statErr := os.Stat(mapPath) // follows symlinks, matching loadAdventuresDir
		if statErr != nil {
			return nil, nil, nil, fmt.Errorf("stat %s: %w", mapPath, statErr)
		}
		if info.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")

		m, loadErr := mapdef.Load(mapPath)
		if loadErr != nil {
			return nil, nil, nil, loadErr
		}

		// THE FILENAME IS THE ID (Task 3's own reason for existing — see
		// this function's package-level doc comment above): mapdef.Compile
		// takes a scene's id from m.ID, never from the file it was loaded
		// from, so a filename that disagrees with the map's own declared id
		// would let a rename silently mint a second scene for the same
		// place while the original stayed in the world. Refused here,
		// loudly, naming BOTH the filename and the id it disagrees with, so
		// the mismatch is loud instead of silent.
		if m.ID != id {
			return nil, nil, nil, fmt.Errorf(
				"maps/%s.json declares id %q: a map's filename is its id, and a "+
					"disagreement would put a second scene in the world for the same "+
					"place — mapdef.Compile takes the scene id from the map's ID",
				id, m.ID)
		}

		// pack lookup by the map's OWN declared id, not by directory
		// co-location (packs are a sibling tree since Task 3) — packs[""]
		// is a legal, deliberate no-op lookup for a map that declares no
		// Pack, exactly mirroring internal/gateway/map.go's handleLoadMap,
		// so this dry run fails on precisely the references that handler
		// would fail on at request time.
		pack := packs[m.Pack]

		// Dry run: proves every override actually resolves (kind/material
		// from the standard vocabulary, art from pack) before this map is
		// ever considered bootable — see this function's own doc comment
		// for why Compile, not a bespoke check.
		if _, _, compileErr := mapdef.Compile(m, pack); compileErr != nil {
			return nil, nil, nil, fmt.Errorf("maps dir %s: map %q (%s): %w", mapsDir, m.ID, mapPath, compileErr)
		}

		maps[m.ID] = m
	}

	// Zero maps loaded from an EXISTING maps/ dir is a boot error, not a
	// quiet success — the same F4 reasoning loadAdventuresDir already
	// applies: without this, an empty-but-present maps/ (e.g. a manually
	// mkdir'd directory nothing was ever copied into) boots cleanly with
	// nothing configured, inconsistent with a NONEXISTENT maps/ (which
	// composeServer's own caller-side guard, and — for any other caller —
	// os.ReadDir's error above, already handle as "nothing installed yet"
	// rather than a boot error; see 2026-09-01-create-scene-leaves Task 5).
	if len(maps) == 0 {
		return nil, nil, nil, fmt.Errorf("maps dir %s contains no maps", dir)
	}
	return maps, packs, packFS, nil
}

// LoadMapsDir is loadMapsDir's boot-facing, test-facing entry point: every
// map in dir loads and validates before `vtt serve` ever accepts a
// connection (maps-as-geometry design spec §4.4 — "fail loud at boot,
// never at the table" — matching adventure-format §7's own posture).
// Callers who only need "does this directory of maps boot cleanly" are not
// forced to know packs/packFS exist at all; composeServer itself calls
// loadMapsDir directly to get all three without walking the filesystem
// twice.
func LoadMapsDir(dir string) (map[string]*mapdef.Map, error) {
	maps, _, _, err := loadMapsDir(dir)
	return maps, err
}
