package mapdef_test

// installed_test.go pins LoadInstalled — the ONE function boot-time loading
// (cmd/vtt's loadMapsDir) and request-time loading (internal/gateway's
// handleLoadMap, via mapByID) both go through, so that a map which boots
// cleanly cannot be refused on reload and vice versa. The 2026-09-01
// create-scene-leaves design spec names that divergence as a hazard in its
// own right (§12, "Boot validation and on-demand validation can diverge...
// They should share one function rather than two similar ones"), which is
// why the shared function is a package-level API with its own tests rather
// than a helper inside either caller.
//
// The id-shape refusal below has no boot-time counterpart to compare
// against: at boot every id comes from a directory entry's own name, so it
// is a filename by construction. On demand the id arrives from a client
// (vttv1.LoadMap.MapId), and it is the ONLY input to a filesystem path this
// package builds from something a request supplied.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// writeInstalled writes body as <dir>/<name>, creating dir. Separate from
// writeFile (load_test.go) only in that it makes the directory first, which
// every test here needs because a maps/ directory is the thing being
// populated.
func writeInstalled(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	writeFile(t, path, body)
	return path
}

// validMapJSON is a map that passes every check mapdef.Load makes, so that
// a test asserting some OTHER refusal cannot be satisfied by an
// accidentally broken fixture. "stone" is a real name in standardTiles
// (standard.go) and format_version 1 is the version this server
// understands — both load-bearing, per the note on
// TestLoadRefusesAFormatThisServerDoesNotUnderstand (load_test.go).
func validMapJSON(id string) string {
	return `{"format_version":1,"id":"` + id + `","name":"A Place",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"}}`
}

// TestLoadInstalledLoadsTheMapNamedByItsFile is the happy path: the id
// names the file, and the loaded map comes back.
func TestLoadInstalledLoadsTheMapNamedByItsFile(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	writeInstalled(t, mapsDir, "level-2.json", validMapJSON("level-2"))

	m, err := mapdef.LoadInstalled(mapsDir, "level-2", "")
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if m.ID != "level-2" || m.Name != "A Place" {
		t.Fatalf("loaded id/name = %q/%q, want level-2/A Place", m.ID, m.Name)
	}
}

// TestLoadInstalledRefusesAFilenameThatDisagreesWithTheID is the shared
// half of the refusal cmd/vtt's own
// TestLoadMapsDirRefusesAFilenameThatDisagreesWithTheID pins at boot
// (design spec §6): mapdef.Compile takes the scene id from the map's own ID
// field, so a file whose name disagrees would put a SECOND scene in the
// world for the same place. Naming both strings is what makes the error
// actionable.
//
// The assertion is on the worded phrase plus the DECLARED id, not on the
// filename alone: the file's own path is in most of this package's error
// messages, so a check for "sunken-cellar" by itself would pass on a
// completely different failure. "declares id" and the id it declares appear
// only in this refusal.
func TestLoadInstalledRefusesAFilenameThatDisagreesWithTheID(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	writeInstalled(t, mapsDir, "sunken-cellar.json", validMapJSON("cellar"))

	_, err := mapdef.LoadInstalled(mapsDir, "sunken-cellar", "")
	if err == nil {
		t.Fatal("want a filename/id mismatch refused")
	}
	for _, want := range []string{`declares id "cellar"`, "sunken-cellar"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

// TestLoadInstalledRefusesAnIDThatIsNotAPlainFilename is the guard on the
// one input a request controls. On demand the id comes from
// vttv1.LoadMap.MapId, and joining it to a directory unchecked would let
// "../elsewhere" read a file the campaign's maps/ does not contain — and,
// since the map's own ID field would agree with the id asked for, INSTALL
// it as a legitimate map.
//
// The fixture is deliberately a map that LOADS: the assertion below proves
// mapdef.Load accepts it at its real path first, so the refusal can only be
// the id check and not an unreadable file. A second refusal is proven at
// the same time — a nested id ("sub/level-2"), which is not an escape at
// all but WOULD be a divergence, since loadMapsDir's flat walk can never
// produce that id at boot, so a map loadable on demand would vanish on
// restart.
func TestLoadInstalledRefusesAnIDThatIsNotAPlainFilename(t *testing.T) {
	root := t.TempDir()
	mapsDir := filepath.Join(root, "maps")
	writeInstalled(t, mapsDir, "keep.json", validMapJSON("keep"))

	outside := writeInstalled(t, root, "elsewhere.json", validMapJSON("../elsewhere"))
	if _, err := mapdef.Load(outside); err != nil {
		t.Fatalf("test setup bug: the escape target must itself be a loadable map, got %v", err)
	}
	nested := writeInstalled(t, filepath.Join(mapsDir, "sub"), "level-2.json", validMapJSON("sub/level-2"))
	if _, err := mapdef.Load(nested); err != nil {
		t.Fatalf("test setup bug: the nested map must itself be loadable, got %v", err)
	}

	for _, id := range []string{"../elsewhere", "sub/level-2", "", ".", ".."} {
		_, err := mapdef.LoadInstalled(mapsDir, id, "")
		if err == nil {
			t.Fatalf("id %q was accepted; a map id is one plain filename in maps/", id)
		}
		if !strings.Contains(err.Error(), "not a map id") {
			t.Errorf("id %q: error = %q, want it to say what is wrong with the id", id, err)
		}
	}
}

// TestLoadInstalledReportsAMapThatIsNotInstalledAsNotExist pins the
// contract the gateway's own refusal depends on: "nothing is installed
// under that name" must be distinguishable from "something is installed and
// it is broken", because the first is an ordinary unknown-map answer and
// the second is a real error a DM has to act on. errors.Is against
// fs.ErrNotExist is how the caller tells them apart without matching on
// message text or leaking the server's filesystem layout into a
// CommandResult.
func TestLoadInstalledReportsAMapThatIsNotInstalledAsNotExist(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	writeInstalled(t, mapsDir, "keep.json", validMapJSON("keep"))

	_, err := mapdef.LoadInstalled(mapsDir, "nowhere", "")
	if err == nil {
		t.Fatal("want an error for a map that is not installed")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want it to wrap fs.ErrNotExist so a caller can tell "+
			"\"not installed\" from \"installed and broken\"", err)
	}
}

// TestLoadInstalledRefusesAnOverrideWhoseArtCannotBeRead is the §12
// anti-divergence proof at this level: loading on demand runs the SAME
// mapdef.Compile dry run boot does (cmd/vtt's
// TestLoadMapsDirFailsLoudWhenArtCannotBeRead pins the boot side), against
// the same art directory. Without it, a map with a broken sidecar would
// boot-fail but reload cleanly — the exact inversion the spec warns about.
//
// The fixture is art that EXISTS and cannot be read, not art that is absent:
// since 2026-09-02-art-is-a-flat-library Task 3 an absent reference degrades
// one square and warns rather than refusing anything (spec §4), so absence
// can no longer drive a refusal at any level.
func TestLoadInstalledRefusesAnOverrideWhoseArtCannotBeRead(t *testing.T) {
	root := t.TempDir()
	mapsDir := filepath.Join(root, "maps")
	artDir := filepath.Join(root, "art")
	writeInstalled(t, mapsDir, "shrine.json", `{"format_version":1,
		"id":"shrine","name":"Obsidian Shrine","grid_width":1,"grid_height":1,
		"tiles":{"0,0":"stone"},"overrides":{"0,0":"wood-planks-split-3"}}`)
	writeInstalled(t, artDir, "wood-planks-split-3.json", `{"format_version":99}`)
	writeInstalled(t, artDir, "wood-planks-split-3.png", "fake-png")

	_, err := mapdef.LoadInstalled(mapsDir, "shrine", artDir)
	if err == nil {
		t.Fatal("an override naming art that cannot be read loaded cleanly; " +
			"boot refuses it, and on demand must refuse it identically")
	}
	if !strings.Contains(err.Error(), "wood-planks-split-3") {
		t.Fatalf("error = %q, want it to name the art it could not read", err)
	}
}

// --- fix round 1 (2026-09-01-create-scene-leaves Task 6) ---------------------

// TestNoLoadInstalledErrorNamesTheDirectoryItRead is the CLASS fix for the
// path disclosure review found in round 1, pinned as a class rather than as
// the one instance that was reported. Every error LoadInstalled can return
// travels verbatim to whoever issued load_map (internal/gateway's mapByID
// forwards all but two), so none of them may carry mapsDir — which on a
// real server is an absolute path saying where the campaign lives.
//
// Round 1 translated only fs.ErrNotExist and let everything else through.
// ENAMETOOLONG is the case review actually fired (a 260-character id from
// an ordinary seat), but it is not special: a directory sitting where a map
// file should be fails on the READ rather than the open, and a map with one
// typo in a tile name never touches an os error at all and still used to
// name the file by its full path.
//
// The positive half of each case matters as much as the negative: an error
// that named nothing would pass a "does not contain mapsDir" check while
// being useless, so every case also demands the relative name.
func TestNoLoadInstalledErrorNamesTheDirectoryItRead(t *testing.T) {
	root := t.TempDir()
	mapsDir := filepath.Join(root, "maps")

	writeInstalled(t, mapsDir, "mismatch.json", validMapJSON("some-other-id"))
	// A tile name outside the standard vocabulary: an ordinary authoring
	// typo, no os error anywhere in it.
	writeInstalled(t, mapsDir, "typo.json", `{"format_version":1,"id":"typo","name":"A Place",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stoen"}}`)
	writeInstalled(t, mapsDir, "future.json", `{"format_version":9,"id":"future","name":"A Place",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"}}`)
	writeInstalled(t, mapsDir, "art.json", `{"format_version":1,"id":"art","name":"A Place",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"wood-planks-split-3"}}`)
	// A DIRECTORY where a map file should be: os.Open succeeds and the
	// first read fails, so this exercises the decode side of decodeStrict
	// rather than the open side.
	if err := os.MkdirAll(filepath.Join(mapsDir, "adir.json"), 0o750); err != nil {
		t.Fatal(err)
	}

	// Art that exists and cannot be read: the one art-side failure that is
	// still a refusal, and the art directory sits under root so a leak of
	// EITHER path fails this test.
	artDir := filepath.Join(root, "art")
	writeInstalled(t, artDir, "wood-planks-split-3.json", `{"format_version":99}`)
	writeInstalled(t, artDir, "wood-planks-split-3.png", "fake-png")

	for _, tc := range []struct{ name, id string }{
		{"not installed", "nowhere"},
		{"name too long", strings.Repeat("z", 260)},
		{"filename disagrees with id", "mismatch"},
		{"tile name typo", "typo"},
		{"format this server does not understand", "future"},
		{"art that cannot be read", "art"},
		{"a directory where a map should be", "adir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mapdef.LoadInstalled(mapsDir, tc.id, artDir)
			if err == nil {
				t.Fatalf("id %q loaded cleanly; this fixture is broken and must be refused", tc.id)
			}
			if strings.Contains(err.Error(), mapsDir) || strings.Contains(err.Error(), root) {
				t.Errorf("error names the directory it read, which reaches a client verbatim:\n  %v", err)
			}
			if want := "maps/" + tc.id + ".json"; !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to name the file as %q", err, want)
			}
		})
	}
}

// TestAMapWhoseArtIsEntirelyUninstalledStillLoads is spec §8's keystone at
// the level both load paths share: "A map loads with every art reference
// unresolvable, and every square renders from its kind. This is §4's whole
// claim and it is the keystone: it fails if anything in the load path still
// treats art as required."
//
// It replaces TestLoadInstalledSaysWhichPackIsNotLoaded and
// TestAMapDeclaringNoPackDoesNotClaimAPackIsMissing, which between them
// pinned the two halves of the refusal this task removed: that a map naming
// an unloaded pack said WHICH pack, and that a map naming no pack was not
// blamed for one. Neither sentence has a subject any more — nothing declares
// a container — and the answer they were shaping is now "it loads".
func TestAMapWhoseArtIsEntirelyUninstalledStillLoads(t *testing.T) {
	root := t.TempDir()
	mapsDir := filepath.Join(root, "maps")
	writeInstalled(t, mapsDir, "bare.json", `{"format_version":1,"id":"bare",
		"name":"A Place","grid_width":1,"grid_height":1,
		"tiles":{"0,0":"stone"},"overrides":{"0,0":"cave-floor-1"}}`)

	m, err := mapdef.LoadInstalled(mapsDir, "bare", filepath.Join(root, "art"))
	if err != nil {
		t.Fatalf("LoadInstalled: %v — a map whose art is not installed still loads (spec §4)", err)
	}
	if m.ID != "bare" {
		t.Fatalf("loaded map = %+v, want the map called bare", m)
	}
}

// TestTheBootWalksOwnUnusableIdsAreRefusedByFilename is M2: round 1's doc
// claimed the id-shape refusal was reachable only from the on-demand path,
// "because at boot every id comes from a directory entry's own name". Three
// real filenames disprove it — loadMapsDir trims ".json" off the entry
// name, so ".json" gives "", "..json" gives ".", and "...json" gives "..",
// all of which reach the refusal from a plain boot walk. The message has to
// name the FILE for those, because an operator staring at a directory has
// no idea what an id of "." means and never typed one.
func TestTheBootWalksOwnUnusableIdsAreRefusedByFilename(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	for _, tc := range []struct{ id, file string }{
		{"", ".json"},
		{".", "..json"},
		{"..", "...json"},
	} {
		writeInstalled(t, mapsDir, tc.file, validMapJSON("x"))
		_, err := mapdef.LoadInstalled(mapsDir, tc.id, "")
		if err == nil {
			t.Fatalf("id %q was accepted", tc.id)
		}
		if !strings.Contains(err.Error(), strconv.Quote(tc.file)) {
			t.Errorf("id %q: error = %q, want it to name the file %q an operator can actually see",
				tc.id, err, tc.file)
		}
	}
}
