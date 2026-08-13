// maps.go is the shared boot-time maps-directory loader (maps-as-geometry
// Task 7): `vtt serve --maps-dir` (composeServer) walks a directory of
// standalone maps and calls mapdef.Load/mapdef.LoadPack per subdirectory —
// factored here the same way adventures.go factors loadAdventuresDir, and
// deliberately mirroring its shape: same symlink-following os.Stat walk,
// same duplicate-id boot error, same "fail loud at boot" posture
// (adventure-format §7, applied to maps by maps-as-geometry design spec
// §4.4).
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// loadMapsDir is the full walk: every map.json plus its own optional
// tiles/pack.json (maps-as-geometry design spec §4.2 — "Beside a standalone
// map: maps/<id>/tiles/pack.json", the exact sibling convention
// internal/adventure/load.go's loadEmbeddedPack already applies to
// adventures/<id>/tiles/pack.json), an fs.FS rooted at each pack's own
// directory for raw byte serving, plus a boot-time dry run of
// mapdef.Compile per map (discarding the result) so an overrides entry that
// does not resolve against its own pack fails HERE rather than only once
// something eventually calls Compile for real — mirroring loadScenes'
// identical dry-run of mapdef.BuildSceneCreated for adventure-embedded
// scenes (internal/adventure/load.go), and reusing Compile itself rather
// than inventing a second validation path, per Task 4's "one construction
// site" discipline.
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
// declared id rather than any directory name (mirroring loadAdventuresDir's
// own dirOf tracking, since nothing requires either id to match its
// directory): maps by Map.ID (gateway.Server.WithMaps' first argument,
// GET /api/maps), packs by Pack.ID (WithMaps' second argument and
// WithPackFiles' fs.FS map, GET /api/packs/{pack}/{file}). A duplicate id in
// EITHER namespace is a boot error naming both directories — for maps this
// is the same footgun loadAdventuresDir already guards against for
// adventure ids; for packs it is sharper, because packFS is keyed globally
// across every map in dir and a silent collision would let one map's pack
// directory shadow another's images at the SAME route.
//
// composeServer calls loadMapsDir directly (not the exported LoadMapsDir
// below) so it does not have to re-derive packs from a second filesystem
// walk.
func loadMapsDir(dir string) (maps map[string]*mapdef.Map, packs map[string]*mapdef.Pack, packFS map[string]fs.FS, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read maps dir: %w", err)
	}
	maps = make(map[string]*mapdef.Map, len(entries))
	packs = make(map[string]*mapdef.Pack, len(entries))
	packFS = make(map[string]fs.FS, len(entries))
	mapDirOf := make(map[string]string, len(entries))
	packDirOf := make(map[string]string, len(entries))

	for _, e := range entries {
		sub := filepath.Join(dir, e.Name())
		info, statErr := os.Stat(sub) // follows symlinks, matching loadAdventuresDir
		if statErr != nil {
			return nil, nil, nil, fmt.Errorf("stat %s: %w", sub, statErr)
		}
		if !info.IsDir() {
			continue
		}

		m, loadErr := mapdef.Load(filepath.Join(sub, "map.json"))
		if loadErr != nil {
			return nil, nil, nil, loadErr
		}

		packDir := filepath.Join(sub, "tiles")
		var pack *mapdef.Pack
		if _, statErr := os.Stat(filepath.Join(packDir, "pack.json")); statErr == nil {
			pack, err = mapdef.LoadPack(packDir)
			if err != nil {
				return nil, nil, nil, err
			}
			if pack.ID == "" {
				return nil, nil, nil, fmt.Errorf(
					"maps dir %s: %s: pack id must not be empty (GET /api/packs/{pack}/... has nothing to address it by)",
					dir, filepath.Join(packDir, "pack.json"))
			}
			if prior, dup := packDirOf[pack.ID]; dup {
				return nil, nil, nil, fmt.Errorf(
					"maps dir %s: pack id %q declared by both %s and %s", dir, pack.ID, prior, packDir)
			}
			// os.OpenRoot, not os.DirFS: see this function's own doc comment
			// for why plain DirFS is not a symlink-safe boundary. Root.FS()
			// (go1.24+) gives an fs.FS whose Open refuses a symlink that
			// resolves outside packDir, not merely a literal ".." in the name.
			root, rootErr := os.OpenRoot(packDir)
			if rootErr != nil {
				return nil, nil, nil, fmt.Errorf("maps dir %s: open pack dir %s: %w", dir, packDir, rootErr)
			}
			packs[pack.ID] = pack
			packFS[pack.ID] = root.FS()
			packDirOf[pack.ID] = packDir
		} else if !os.IsNotExist(statErr) {
			return nil, nil, nil, fmt.Errorf("stat %s: %w", filepath.Join(packDir, "pack.json"), statErr)
		}

		// Dry run: proves every override actually resolves (kind/material
		// from the standard vocabulary, art from pack) before this map is
		// ever considered bootable — see this function's own doc comment
		// for why Compile, not a bespoke check.
		if _, _, compileErr := mapdef.Compile(m, pack); compileErr != nil {
			return nil, nil, nil, fmt.Errorf("maps dir %s: map %q (%s): %w", dir, m.ID, sub, compileErr)
		}

		if prior, dup := mapDirOf[m.ID]; dup {
			return nil, nil, nil, fmt.Errorf("maps dir %s: map id %q declared by both %s and %s", dir, m.ID, prior, sub)
		}
		maps[m.ID] = m
		mapDirOf[m.ID] = sub
	}

	// Zero maps loaded from an EXISTING dir is a boot error, not a quiet
	// success — the same F4 reasoning loadAdventuresDir already applies:
	// without this, a typo'd or never-synced --maps-dir boots cleanly with
	// nothing configured, inconsistent with a NONEXISTENT dir (which already
	// fails loud via os.ReadDir's own error above).
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
