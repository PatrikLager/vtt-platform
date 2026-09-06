// maps.go is the shared boot-time maps-directory loader (maps-as-geometry
// Task 7; layout changed by Task 3 of the create_scene-leaves plan, 2026-
// 09-01, "the kernel serves maps, it does not make them"): composeServer
// (serve_compose.go) walks the campaign directory itself and calls
// mapdef.LoadInstalled — there is no --maps-dir flag any more as of
// Task 5 of that same plan, maps belong to the campaign that uses them —
// factored here the same way adventures.go factors loadAdventuresDir, and
// sharing its "fail loud at boot" posture (adventure-format §7, applied to
// maps by maps-as-geometry design spec §4.4).
//
// SINCE TASK 3, every "<dir>/maps/<id>.json" is one standalone map, named by
// its own filename — not one map per subdirectory. The ID-mismatch refusal that
// keeps that true lives in mapdef.LoadInstalled since Task 6 of the
// create_scene-leaves plan, and it is the whole point of Task 3 rather than an
// incidental rule: mapdef.Compile takes a SceneCreated's id from the map's own
// ID field, so if the filename alone governed identity, renaming a file and
// reloading it would silently mint a SECOND scene for the same place while the
// original stayed in the world.
//
// THERE IS NO SECOND TREE ANY MORE. This walk read a sibling "<dir>/packs/"
// until 2026-09-02-art-is-a-flat-library Task 7, keyed each pack by its own
// declared id, and handed the result to gateway.WithMaps/WithPackFiles so
// GET /api/packs/{pack}/{file} could serve raw bytes out of it. Task 3 of that
// plan had already moved every map's own art resolution to "<dir>/art", the
// campaign's one flat art directory, inside mapdef.LoadInstalled — which is
// also the function internal/gateway's mapByID calls when a map turns up after
// boot, so boot-time and request-time validation are not merely alike, they are
// one function (Task 6 of the create_scene-leaves plan; that plan's design spec
// §12 names their divergence as a hazard in its own right). Task 5 then made a
// map that DECLARES a pack a refusal, and Task 7 took the tree, the route and
// mapdef.Pack itself. That refusal's message carried migration instructions
// until 2026-09-06, when Patrik ruled the route out; what refuses such a map now
// is mapdef's strict decoding, `json: unknown field "pack"`. Nothing serves
// art bytes until that plan's Task 6 builds GET /api/art/{file}.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// loadMapsDir walks "<dir>/maps" and loads every "<id>.json" through
// mapdef.LoadInstalled, which validates each map and dry-runs mapdef.Compile
// against "<dir>/art" — so an overrides entry naming art written for a
// LATER format_version than this server understands fails HERE rather than only
// once something eventually calls Compile for real, mirroring loadScenes'
// identical dry-run of mapdef.BuildSceneCreated for adventure-embedded scenes
// (internal/adventure/load.go), and reusing Compile itself rather than
// inventing a second validation path, per Task 4's "one construction site"
// discipline.
//
// THAT IS THE ONLY ART FAILURE LEFT THAT CAN FAIL A BOOT. An entry naming art
// that is merely ABSENT stopped failing at Task 3, and since Patrik's ruling
// of 2026-09-04 so has one naming art that is installed and CANNOT BE READ — a
// missing brace, a truncated copy, a door naming one picture. Each degrades
// one square and warns (2026-09-02-art-is-a-flat-library design spec §4), and
// this walk discards the warnings for the same reason mapdef.LoadInstalled
// does. The second change is not tidying: this walk really did refuse a boot
// over one such file, measured 2026-09-03, exit status 1 with every other map
// in the campaign fine.
//
// AN ABSENT maps/ IS NOT AN ERROR, and that is what lets composeServer wire the
// maps directory with no guard around this call (art-is-a-flat-library Task 7).
// A campaign that has installed nothing yet is the ordinary starting state
// (2026-09-01-create-scene-leaves design spec §4, "Install, then load"), and
// the guard that used to say so — an os.Stat(mapsDir) in composeServer, whose
// ONLY reason was that this walk failed on a missing directory — is the exact
// shape of the defect this sub-project exists to remove: a check on one
// directory deciding whether another one gets loaded (design spec §1). The
// packs half of this walk already tolerated a missing packs/ the same way; when
// it was deleted, the maps half adopted its tolerance rather than inheriting a
// guard nobody needed.
//
// ANY OTHER READ FAILURE STILL FAILS LOUD — permissions, a plain file sitting
// where maps/ belongs — and that arm is the one place in this function where
// the WRONG answer is silent: a campaign that cannot be read would otherwise
// boot serving zero maps, telling the DM "no maps available" and the operator
// nothing. It is held by
// TestAnUnreadableMapsPathIsABootErrorNotAnEmptyCampaign (maps_test.go), added
// 2026-09-04 after review measured that deleting the arm left the whole
// package green.
//
// Maps are addressed by the map's own declared id, which since Task 3 is always
// exactly its filename minus ".json" (mapdef.LoadInstalled's own refusal), and
// a filesystem cannot hold two entries of the same name in one directory — so
// the duplicate-id collision loadAdventuresDir guards against for adventures
// cannot arise here by construction, and nothing analogous is checked or tested
// for maps.
//
// composeServer calls loadMapsDir rather than the exported LoadMapsDir below
// only because the two are now the same call; the split is kept because
// LoadMapsDir is this package's name for "does this campaign's maps boot".
func loadMapsDir(dir string) (map[string]*mapdef.Map, error) {
	mapsDir := filepath.Join(dir, "maps")

	// The campaign's flat art directory (2026-09-02-art-is-a-flat-library
	// design spec §3): the root every map's overrides and object art resolve
	// against, handed down rather than discovered, because cmd/vtt owns the
	// filesystem (ADR-008). It need not exist — a campaign that has installed
	// no art is ordinary, its maps still load, and each unresolved reference
	// costs one warning rather than the map (spec §4).
	artDir := filepath.Join(dir, "art")

	mapEntries, readErr := os.ReadDir(mapsDir)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("read maps dir: %w", readErr)
		}
		// Nothing installed yet — see this function's own doc comment. An
		// empty non-nil map, so a caller cannot tell "no maps/" from "maps/
		// with nothing in it" by nil-ness and then act on the difference.
		return map[string]*mapdef.Map{}, nil
	}

	maps := make(map[string]*mapdef.Map, len(mapEntries))
	for _, e := range mapEntries {
		mapPath := filepath.Join(mapsDir, e.Name())
		info, statErr := os.Stat(mapPath) // follows symlinks, matching loadAdventuresDir
		if statErr != nil {
			return nil, fmt.Errorf("stat %s: %w", mapPath, statErr)
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
			return nil, fmt.Errorf("maps dir %s: %w", mapsDir, loadErr)
		}

		maps[m.ID] = m
	}

	// Zero maps loaded from an EXISTING maps/ dir is a boot error, not a
	// quiet success — the same F4 reasoning loadAdventuresDir already
	// applies: without this, an empty-but-present maps/ (e.g. a manually
	// mkdir'd directory nothing was ever copied into) boots cleanly with
	// nothing configured, inconsistent with a NONEXISTENT maps/, which is
	// "nothing installed yet" and returns above without reaching this check
	// (see 2026-09-01-create-scene-leaves Task 5, and this function's own
	// doc comment for why the two answers differ).
	if len(maps) == 0 {
		return nil, fmt.Errorf("maps dir %s contains no maps", dir)
	}
	return maps, nil
}

// LoadMapsDir is loadMapsDir's boot-facing, test-facing entry point: every
// map in dir loads and validates before `vtt serve` ever accepts a
// connection (maps-as-geometry design spec §4.4 — "fail loud at boot,
// never at the table" — matching adventure-format §7's own posture).
//
// IT IS A PLAIN FORWARD NOW. It existed to hide a second and third return
// value (a pack set and a per-pack fs.FS) from callers who only wanted "does
// this directory of maps boot cleanly", and 2026-09-02-art-is-a-flat-library
// Task 7 deleted both. What is left is the exported name, kept because it is
// the one this package's tests and harness_boot ask that question by.
func LoadMapsDir(dir string) (map[string]*mapdef.Map, error) {
	return loadMapsDir(dir)
}

// artRootIsOpenable is the ONE art check boot makes that a request-time load
// deliberately does not (Patrik's ruling, 2026-09-03): an art/ that exists and
// cannot be opened — a plain file sitting where the directory belongs, a mode
// that forbids it — fails the boot loudly, while mapdef.Resolve degrades the
// same condition to a warning when a DM issues load_map. An operator is at a
// terminal and can fix a directory; a DM in a browser cannot, and a campaign
// that worked five minutes ago should not stop working at the table.
//
// IT IS CALLED FROM composeServer, NOT FROM loadMapsDir ABOVE, and that is the
// whole point rather than a detail of placement. When this check was written,
// composeServer called loadMapsDir only when campaignPath/maps EXISTED, so a
// check living inside the walk would not run for a campaign that has art and no
// map yet — which is this sub-project's own reason to exist, quoted in the
// design spec's §1: "composeServer gates the pack load on maps/ existing, so a
// campaign with art and no map yet boots with no art at all." Measured on the
// first version of this check, which did live inside the walk: broken art/ with
// no maps/ booted clean, broken art/ with maps/ present refused.
//
// THAT GUARD IS GONE as of Task 7 of the same plan — loadMapsDir tolerates an
// absent maps/ itself, so there is nothing left to sit outside of — and this
// check STAYS HERE anyway, on purpose. Moving it into the walk would re-couple
// an art verdict to a maps read for no reason but proximity, and the next
// person to add an early return to that walk would silently take the art check
// with them. cmd/vtt's TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists
// is what catches that, and its "no maps yet" subtest is the one that matters.
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
