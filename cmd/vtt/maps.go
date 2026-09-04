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
// named by its own filename (the ID-mismatch refusal that keeps that true
// lives in mapdef.LoadInstalled since Task 6 of the create_scene-leaves
// plan — this is the whole point of Task 3, not an incidental rule:
// mapdef.Compile takes a SceneCreated's id from the map's own ID field, so
// if the filename alone governed identity, renaming a file and reloading it
// would silently mint a SECOND scene for the same place while the original
// stayed in the world); every "<dir>/packs/<name>/pack.json" is one pack,
// keyed by its OWN declared id (packs/<name>'s directory name need not
// match — only the pack.json "id" field does, exactly as before Task 3).
// The two trees are independent, and since 2026-09-02-art-is-a-flat-library
// Task 3 they no longer meet at all: a map's overrides resolve against
// "<dir>/art", the campaign's one flat art directory, inside
// mapdef.LoadInstalled — which is also the function internal/gateway's
// mapByID calls when a map turns up after boot, so boot-time and
// request-time validation are not merely alike, they are one function
// (Task 6 of the create_scene-leaves plan; that plan's design spec §12
// names their divergence as a hazard in its own right). A map that DECLARES a
// pack is now refused outright by mapdef.Load, naming the field and pointing at
// art/ (Task 5 of the art plan, that plan's design spec §7), so this walk's own
// packs half serves nothing but GET /api/packs/{pack}/{file} — Task 7 deletes
// the tree it still builds.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// loadMapsDir is the full walk: every packs/<name>/pack.json first (the pack
// set is still built and served over GET /api/packs/{pack}/{file}, but since
// 2026-09-02-art-is-a-flat-library Task 3 no map RESOLVES against it and since
// Task 5 no map may even NAME it — Task 7 deletes this half), then every
// maps/<id>.json through
// mapdef.LoadInstalled, which validates each map and dry-runs mapdef.Compile
// against dir/art — so an overrides entry naming art that is installed and
// unreadable fails HERE rather than only once something eventually calls
// Compile for real, mirroring loadScenes' identical dry-run of
// mapdef.BuildSceneCreated for adventure-embedded scenes
// (internal/adventure/load.go), and reusing Compile itself rather than
// inventing a second validation path, per Task 4's "one construction site"
// discipline. An entry naming art that is merely ABSENT no longer fails at
// all: it degrades one square and warns (spec §4), and this walk discards
// the warnings for the same reason mapdef.LoadInstalled does.
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
// id refusal (mapdef.LoadInstalled) means a map's key is always exactly its
// own filename (minus ".json"), and a filesystem cannot hold two entries of
// the same
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

	// The campaign's flat art directory (2026-09-02-art-is-a-flat-library
	// design spec §3): the root every map's overrides and object art resolve
	// against, handed down rather than discovered, because cmd/vtt owns the
	// filesystem (ADR-008). It need not exist — a campaign that has installed
	// no art is ordinary, its maps still load, and each unresolved reference
	// costs one warning rather than the map (spec §4).
	artDir := filepath.Join(dir, "art")

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

		// mapdef.LoadInstalled is the WHOLE per-map check — read, validate,
		// filename-is-the-id, and the dry-run Compile against the campaign's
		// art directory — and it is the same function internal/gateway's
		// mapByID calls when a map turns up after boot (2026-09-01-create-
		// scene-leaves Task 6). One function rather than two similar ones,
		// because the design spec names their divergence as a hazard of its
		// own (§12): a map that boots cleanly must not be refused on
		// reload, nor the reverse. Everything this loop used to do inline
		// lives there now, with its reasoning.
		m, loadErr := mapdef.LoadInstalled(mapsDir, id, artDir)
		if loadErr != nil {
			return nil, nil, nil, fmt.Errorf("maps dir %s: %w", mapsDir, loadErr)
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

// artRootIsOpenable is the ONE art check boot makes that a request-time load
// deliberately does not (Patrik's ruling, 2026-09-03): an art/ that exists and
// cannot be opened — a plain file sitting where the directory belongs, a mode
// that forbids it — fails the boot loudly, while mapdef.Resolve degrades the
// same condition to a warning when a DM issues load_map. An operator is at a
// terminal and can fix a directory; a DM in a browser cannot, and a campaign
// that worked five minutes ago should not stop working at the table.
//
// IT IS CALLED FROM composeServer, NOT FROM loadMapsDir BELOW, and that is the
// whole point rather than a detail of placement. composeServer calls
// loadMapsDir only when campaignPath/maps EXISTS, so a check living inside the
// walk would not run for a campaign that has art and no map yet — which is
// this sub-project's own reason to exist, quoted in the design spec's §1:
// "composeServer gates the pack load on maps/ existing, so a campaign with art
// and no map yet boots with no art at all." Measured on the first version of
// this check, which did live inside the walk: broken art/ with no maps/ booted
// clean, broken art/ with maps/ present refused. WithMapsDir sits outside that
// guard for the same reason; so does this.
//
// AN ABSENT art/ PASSES, because a campaign that has installed no art is
// ordinary and its maps still load and draw plain. Anything else present at
// that path is a broken installation, not an empty one.
//
// This error names the directory, unlike everything mapdef returns: it is
// printed to whoever started the server and reaches no client, and a path is
// the only useful thing to say to someone who has to go and chmod it.
//
// It opens rather than stats, so a directory with the wrong mode is caught
// with the same call that a lookup would fail on.
//
// artlib.Validate DOES NOT SUBSUME THIS, which this comment predicted it would
// until Task 4 of that plan wired it up. composeServer runs both, in this
// order, and they have different verdicts: Validate finds problems in
// individual files and REPORTS them while the server starts anyway (Patrik's
// severity ruling, 2026-09-03 — one hand-copied file must not cost a table its
// server), and a report is all it is: what each of its findings does at the
// table ranges from refusing the map to rendering with nothing objecting, and
// internal/artlib's Validate doc comment carries the measured table. This, by
// contrast, is the one art condition that still refuses the boot. An
// unopenable ROOT is not one piece failing, it is every piece failing, and
// starting on it would serve a campaign with no art at all and a warning on
// every load.
func artRootIsOpenable(artDir string) error {
	root, err := os.OpenRoot(artDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("art dir %s: %w", artDir, err)
	}
	return root.Close()
}
