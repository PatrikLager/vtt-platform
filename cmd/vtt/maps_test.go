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
// own id — not a directory.
//
// THERE IS NO SIBLING packs/ TREE ANY MORE. This walk built one, keyed by each
// pack.json's own declared id, and served it over GET /api/packs/{pack}/{file};
// 2026-09-02-art-is-a-flat-library deleted the coupling in three steps and then
// the thing itself — Task 3 moved request-time art resolution to the campaign's
// flat art/ directory, Task 5 deleted mapdef.Map.Pack and made a map file
// declaring "pack" a refusal, and Task 7 deleted the walk, the route and
// mapdef.Pack. Two tests went with it; their obituaries are below, beside the
// duplicate-id test that used to guard the route's namespace.

import (
	"log/slog"
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

	maps, err := loadMapsDir(root)
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

	_, err := loadMapsDir(root)
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

// TestLoadMapsDirLoadsAValidMap walks a campaign holding one map. It asserted
// the map's own Pack field until Task 5 of 2026-09-02-art-is-a-flat-library
// deleted mapdef.Map.Pack and made a map file declaring "pack" a refusal
// (design spec §7) — hence the name, which read "...AndItsPack" — and its
// fixture kept a packs/ tree beside the map until Task 7 deleted the walk that
// read one.
func TestLoadMapsDirLoadsAValidMap(t *testing.T) {
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
}

// TestLoadMapsDirFailsLoudWhenArtDeclaresAFormatItDoesNotUnderstand pins the
// fuller promise of maps-as-geometry spec §4.4 that mapdef.Load alone cannot
// check (it reads no art — see resolve.go's own doc comment): an overrides
// entry naming art written for a format_version this server does not
// understand must fail at BOOT, not only once something tries to Compile it.
// loadMapsDir proves this by dry-running mapdef.Compile
// per map against the campaign's own art/ — the same directory
// internal/gateway/map.go's handleLoadMap will resolve against at request
// time, and the same technique internal/adventure/load.go's loadScenes
// applies to adventure-embedded scenes, reused here rather than a second
// hand-rolled check, per maps-as-geometry Task 4's "one construction site"
// discipline.
//
// IT USED TO DRIVE ART THE PACK DID NOT DEFINE, then art that simply could not
// be read, and both of those now degrade instead: 2026-09-02-art-is-a-flat-
// library spec §4 drops an art reference with no file behind it to a plain
// square and one warning (Task 3), and Patrik's ruling of 2026-09-04 does the
// same for a file that is installed and broken (Task 4b). The only art-side
// boot refusal left is the one this drives — a sidecar declaring a format this
// server does not understand.
//
// TestABootLoadedMapWhoseArtIsNotInstalledStillBoots and
// TestACampaignHoldingACorruptSidecarStillBoots (maps_e2e_test.go) are the
// other half of that pair, and it is the half worth having: without them,
// "fails loud" could be satisfied by a loader that refuses everything, which
// is measurably what this one did until Task 4b.
func TestLoadMapsDirFailsLoudWhenArtDeclaresAFormatItDoesNotUnderstand(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "art"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Art that IS installed and declares a format this server does not
	// understand — the file is not broken, this server is too old to read it,
	// and that is the one art fact a boot still refuses on.
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
		t.Fatal("an override naming art written for a later format loaded cleanly; " +
			"it should have failed at boot, not waited for someone to Compile it")
	}
	if !strings.Contains(err.Error(), "wood-planks-split-3") {
		t.Errorf("error should name the art it refused, got: %v", err)
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

// TestEveryArtProblemIsReportedAtBootAndTheServerStartsAnyway is Patrik's
// severity ruling of 2026-09-03, which overrides the plan's own Task 4 brief
// ("fail the boot on a subdirectory or an orphan sidecar"). RUN THE CHECK AT
// START, REPORT EVERY PROBLEM, AND START ANYWAY: a campaign with two hundred
// good pieces and one Masonry-1.png copied off a Windows box must not fail to
// boot, because the refusal fires on files no map has ever named and fixing
// five mistakes would otherwise cost five boots. `vtt art install` (Task 6)
// still refuses outright, because there the operator is holding the file.
//
// WHAT THE RULING BUYS IS THE BOOT, NOT A GUARANTEE ABOUT RENDERING. This
// comment said "nothing malformed can render regardless — artlib.Lookup refuses
// each broken piece individually when a map names it" until 2026-09-03, and
// that is false in three of the five arms (review finding F1): a relative
// symlink and a wrong-cased filename both RENDER, and an orphan sidecar or a
// subdirectory draws the square plain rather than refusing it. internal/artlib's
// Validate doc comment carries the measured table. That is what makes THIS test
// matter more, not less — for two arms the boot report is the only notice
// anyone gets, so a boot that says nothing is a defect that ships silently.
//
// THE "no maps yet" SUBTEST IS THE ONE THAT MATTERS, exactly as in
// TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists above: composeServer
// calls loadMapsDir only when campaignPath/maps EXISTS, so a validation call
// placed inside that guard never runs for a campaign that has art and no map
// yet — design spec §1's own defect, rebuilt one directory over. A fresh
// campaign IS the improvisation case.
//
// Three problems sit around a piece that is FINE. os.ReadDir sorts by name and
// 'M' sorts before 'a', so the entries arrive Masonry-1.png, aaa-good.json,
// aaa-good.png, cellar-basics/, orphan.json — a walk that answers with its
// first finding names Masonry-1.png and stops, and cellar-basics and orphan are
// what it could not say. internal/artlib's own
// TestValidateReportsEveryProblemNotOnlyTheFirst pins the collecting arm by
// arm; this pins that composeServer runs the walk at all and says what it found.
func TestEveryArtProblemIsReportedAtBootAndTheServerStartsAnyway(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withMaps bool
	}{
		{"no maps yet — the campaign this sub-project exists for", false},
		{"maps installed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := filepath.Join(t.TempDir(), "campaign")
			artDir := filepath.Join(campaignPath, "art")
			if err := os.MkdirAll(filepath.Join(artDir, "cellar-basics"), 0o750); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(artDir, "aaa-good.json"),
				`{"format_version":1,"kind":"wall","material":"stone"}`)
			writeFile(t, filepath.Join(artDir, "aaa-good.png"), "fake-png")
			writeFile(t, filepath.Join(artDir, "orphan.json"),
				`{"format_version":1,"kind":"wall","material":"stone"}`)
			writeFile(t, filepath.Join(artDir, "Masonry-1.png"), "fake-png")
			if tc.withMaps {
				if err := os.MkdirAll(filepath.Join(campaignPath, "maps"), 0o750); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(campaignPath, "maps", "shrine.json"), `{
					"format_version": 1, "id": "shrine", "name": "Obsidian Shrine",
					"grid_width": 1, "grid_height": 1, "tiles": {"0,0":"wood"}
				}`)
			}

			boot := captureBootLog(t)
			_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
			if err != nil {
				t.Fatalf("composeServer: %v — one hand-copied file must not cost a table "+
					"its server, and none of these is a file any map has named", err)
			}
			t.Cleanup(func() {
				if closeErr := closeFn(); closeErr != nil {
					t.Error(closeErr)
				}
			})

			said := boot.String()
			for _, want := range []string{"cellar-basics", "orphan", "Masonry-1.png"} {
				if !strings.Contains(said, want) {
					t.Errorf("boot said:\n%s\nwant it to name %q — an operator who is told about "+
						"one problem per boot pays one boot per problem", said, want)
				}
			}
			if strings.Contains(said, "aaa-good") {
				t.Errorf("boot said:\n%s\naaa-good is a well-formed piece and must not be named", said)
			}
		})
	}
}

// captureBootLog redirects the default slog logger into a buffer for one test,
// which is how composeServer's art report is read back: it is a WARNING and
// not an error, so it cannot come back through composeServer's return values,
// and slog is where internal/ already puts "the platform continues and here is
// what is wrong" (internal/campaign's replay skip, internal/gateway's withheld
// projection).
//
// syncBuffer rather than a bytes.Buffer: another test's server may still be
// draining a goroutine that logs, and the default logger is process-wide.
// Nothing in this package runs t.Parallel, so the swap itself is safe.
func captureBootLog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := newSyncBuffer()
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prior) })
	return buf
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

// TWO TESTS STOOD HERE AND LEFT WITH THE PACK WALK
// (2026-09-02-art-is-a-flat-library Task 7). Both guarded the namespace
// GET /api/packs/{pack}/{file} addressed packs by, and that route is gone:
//
//   - TestLoadMapsDirRefusesDuplicatePackIds: two packs/ subdirectories
//     declaring the same id, which would have let the second silently shadow
//     the first's images at the same URL.
//   - TestLoadMapsDirRefusesAnUnnamedPack: a pack.json with no id at all, which
//     no URL could ever address.
//
// NEITHER PROPERTY MOVES, and neither needs to. Art is addressed by FILENAME
// inside one flat art/ directory now (that plan's design spec §3), so a
// filesystem that cannot hold two entries of the same name is the whole of the
// uniqueness rule — the plan says so outright in its Global Constraints ("No
// duplicate check is written anywhere") — and nothing declares an id that could
// be missing. What replaces the "a name nothing can address" refusal is
// internal/artlib's TestValidateRefusesAFilenameThatIsNotAnArtName.

// TestACampaignWithNoMapsDirectoryLoadsCleanly asserts an ABSENCE — of the
// refusal this walk used to make — so it was written before the change and
// failed until 2026-09-02-art-is-a-flat-library Task 7 landed it.
//
// IT IS WHAT LETS composeServer's os.Stat(mapsDir) GUARD GO. That guard existed
// for one reason: this walk failed on a missing maps/, so a brand-new campaign
// could not be walked at all. The packs half already tolerated a missing
// packs/, and with the packs half deleted the maps half is the only thing left
// that did not — so it adopts the same tolerance, the guard has nothing to
// decide, and a boot-order guard around a load is exactly the shape sub-project
// 15 shipped as a defect (design spec §1). A campaign that has installed
// nothing yet is the ordinary starting state (2026-09-01-create-scene-leaves
// design spec §4, "Install, then load"), not a broken one.
//
// An empty-but-PRESENT maps/ is still a boot error — see the test below, which
// is the case this one must not be confused with.
func TestACampaignWithNoMapsDirectoryLoadsCleanly(t *testing.T) {
	maps, err := LoadMapsDir(t.TempDir())
	if err != nil {
		t.Fatalf("LoadMapsDir on a campaign with no maps/ at all: %v — a campaign "+
			"that has installed nothing yet is ordinary, not broken", err)
	}
	if len(maps) != 0 {
		t.Fatalf("maps = %v, want none: there is no maps/ to have loaded any from", keys(maps))
	}
}

// TestAnUnreadableMapsPathIsABootErrorNotAnEmptyCampaign is what HOLDS the
// discrimination the test above introduced, and without it that discrimination
// is prose. loadMapsDir tells "maps/ is not there" (nothing installed yet —
// boot, empty set) from "maps/ is there and I cannot read it" (boot error), and
// review measured on 2026-09-04 that deleting the second arm entirely left
// `go build`, `go vet` and this whole package green: NOTHING in the tree held
// it.
//
// THE WRONG BRANCH HERE IS SILENT, WHICH IS WHY IT IS WORTH A TEST OF ITS OWN.
// At BASE this function made no discrimination at all — every os.ReadDir
// failure was an error — and absence-tolerance lived in composeServer's
// os.Stat guard, where deciding wrongly sent you INTO loadMapsDir and still
// failed loud. Task 7 moved the decision to the one place where getting it
// wrong says nothing: a campaign whose maps/ is a plain file, or a directory
// this process cannot read after a bad `cp -a`, a restore, or a container
// volume mount, would boot cleanly serving zero maps. The DM sees "no maps
// available"; the operator sees nothing at all.
//
// A PLAIN FILE AT maps/ rather than a mode, deliberately, and for the reason
// TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists already records: a
// permissions fixture passes trivially for a process running as root, and CI
// containers often are. os.ReadDir on a plain file is ENOTDIR, which is not
// os.IsNotExist, so it lands on exactly the arm under test with no privileges
// involved.
func TestAnUnreadableMapsPathIsABootErrorNotAnEmptyCampaign(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "maps"), "a plain file where maps/ belongs")

	maps, err := LoadMapsDir(dir)
	if err == nil {
		t.Fatalf("a campaign whose maps/ cannot be read booted with %d map(s) and no "+
			"error — the DM sees an empty map list and the operator is told nothing",
			len(maps))
	}
	// Names the path, so an operator who has to go and fix an ownership or a
	// stray file knows which one. This error is printed to whoever started the
	// server and reaches no client, the same reasoning artRootIsOpenable's own
	// doc comment gives for naming its directory.
	if !strings.Contains(err.Error(), filepath.Join(dir, "maps")) {
		t.Errorf("error = %q, want it to name the path an operator has to go and fix", err)
	}
}

// TestLoadMapsDirEmptyDirIsBootError mirrors loadAdventuresDir's own F4 fix
// (adventures_test.go's TestLoadAdventuresDirEmptyDirIsBootError): a maps/
// that exists but was never populated (an empty mkdir, or a sync that
// dropped its files but not itself) booting cleanly with zero maps
// configured is a quiet failure, not a loud one — inconsistent with a
// NONEXISTENT dir, which is "nothing installed yet" and boots (see
// TestACampaignWithNoMapsDirectoryLoadsCleanly directly above).
//
// THE FIXTURE MKDIRS maps/, and until 2026-09-02-art-is-a-flat-library Task 7
// it did not — it handed LoadMapsDir a bare t.TempDir(), so the case it
// actually exercised was the NONEXISTENT one its own comment disclaims, and it
// passed on os.ReadDir's not-exist error rather than on the len(maps) == 0
// check it names. That went unnoticed because both answers were an error. Task
// 7 splits the two answers apart, which is what made the degenerate fixture
// visible; the assertion below is unchanged, and now runs against the directory
// state the sentence above describes.
func TestLoadMapsDirEmptyDirIsBootError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o750); err != nil {
		t.Fatal(err)
	}
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

// writeShrineMap writes a minimal but VALID map as maps/<id>.json under dir —
// Task 3's flat layout. id names the map FILE, and a map's filename IS its id
// per the refusal this file pins above.
//
// It wrote a packs/mossy-keep/pack.json beside it until
// 2026-09-02-art-is-a-flat-library Task 7. The override is kept and still names
// wood-planks-split-3: since Task 3 an override naming art that is not
// installed costs that square its picture and one warning rather than the map
// (design spec §4), so this fixture exercises the degrade path on purpose
// rather than by omission.
func writeShrineMap(t *testing.T, dir, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "maps", id+".json"), `{
		"format_version": 1,
		"id": "`+id+`", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1,
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
