package mapdef_test

import (
	"fmt"
	"github.com/PatrikLager/vtt-platform/internal/artlib"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

func TestLoadsAMapWhereEverySquareNamesItsTile(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Tiles["0,0"] != "stone-wall" {
		t.Fatalf("square 0,0 is %q, want stone-wall", m.Tiles["0,0"])
	}
	// The override changes the PICTURE only; the square is still what tiles says.
	if m.Tiles["1,1"] != "wood" || m.Overrides["1,1"] != "planks-split-3" {
		t.Fatalf("override did not stay separate from nature: %v / %v",
			m.Tiles["1,1"], m.Overrides["1,1"])
	}
}

// TestLoadsAMapWithNoTilesAsHavingNoTerrain pins Patrik's ruling
// (2026-08-13): a map declaring no "tiles" key at all has no terrain —
// exactly what existed before maps-as-geometry — and that must stay legal
// forever, since this format is written by third parties and by an LLM,
// and an existing file must keep loading. This is deliberately NOT the same
// thing as a map with an incomplete tiles map (that stays an error — see
// TestInvalidMapsAreRefusedWithAUsefulReason's "missing-square" case, a
// genuinely PARTIAL map, 8 of 9 squares present).
func TestLoadsAMapWithNoTilesAsHavingNoTerrain(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/no-terrain.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Tiles) != 0 {
		t.Fatalf("Tiles = %v, want empty (no terrain declared)", m.Tiles)
	}
}

// TestObjectFieldsSurviveTheJSONToMapShapeConversion pins
// checkObjectsInsideGrid's At/Size -> X/Y/W/H split field-by-field. Without
// this, TestLoadsAMapWhereEverySquareNamesItsTile never looks at m.Objects
// at all, so a transposed X/Y or W/H in that conversion would pass every
// other test in this file silently (confirmed at code review by swapping
// the assignment and re-running: see this package's task-2 report for the
// fault-injection record).
func TestObjectFieldsSurviveTheJSONToMapShapeConversion(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Objects) != 1 {
		t.Fatalf("Objects = %+v, want exactly 1", m.Objects)
	}
	want := mapdef.Object{
		ID: "boulder-1", Kind: "boulder",
		X: 0, Y: 1, W: 1, H: 1, Rotation: 0,
		BlocksSight: true, BlocksMove: true,
		Art: "boulder-mossy-2",
	}
	if got := m.Objects[0]; got != want {
		t.Fatalf("Objects[0] = %+v, want %+v", got, want)
	}
}

// Every refusal in spec §4.4 gets a case. Table-driven over fixture dirs,
// following internal/rules/testdata/invalid-v2/'s pattern.
func TestInvalidMapsAreRefusedWithAUsefulReason(t *testing.T) {
	for _, c := range []struct{ dir, want string }{
		{"missing-square", "no tile"},
		{"unknown-tile-name", "unknown tile"},
		{"override-outside-grid", "outside the grid"},
		{"object-outside-grid", "outside the grid"},
		{"token-inside-wall", "inside a wall"},
		{"zero-grid", "must be >= 1"},
		// Not part of task-2-brief.md's enumerated six — added at code
		// review, pre-commit. Both close a real gap rather than merely add
		// belt-and-suspenders coverage:
		//   - tile-outside-grid pins spec §4.4's OTHER tiles rule ("no entry
		//     lies outside the grid"), which checkEverySquarePresent alone
		//     cannot catch: it only proves every REQUIRED square is present,
		//     never that Tiles has no EXTRA square beyond the grid.
		//   - object-non-positive-size pins that an object's footprint must
		//     be at least 1x1: without this, size:[0,0] (or a negative size)
		//     made the bounds check `at+size > grid` trivially true even for
		//     an anchor sitting outside the grid.
		{"tile-outside-grid", "outside the grid"},
		{"object-non-positive-size", "at least 1x1"},
		// Also added at review: grid_height's own zero check shares its
		// code with grid_width's (task-2-brief.md's "zero-grid" only
		// exercises the WIDTH half), and a placement's own bounds check
		// (checkPlacementsNotInWalls) was previously reachable only via a
		// wall lookup that happened to also be in-grid — nothing had ever
		// driven a placement whose x/y is not a square at all.
		{"zero-grid-height", "must be >= 1"},
		{"placement-outside-grid", "outside the grid"},
		// Not one of §4.4's validation rules -- pins the strict-decode
		// clause Load's own doc comment promises ("no unknown fields
		// tolerated"), mirroring internal/adventure/testdata/invalid/
		// unknown-field's identical role for that sibling loader.
		{"unknown-field", "bogus_field"},
		// Patrik's ruling (2026-08-13): tiles is optional, but overrides
		// with no tiles at all is incoherent -- an override names art for
		// a square whose NATURE is declared in tiles, so there is nothing
		// to attach the art to. The exact phrase below (not just
		// "overrides") is deliberate: the fixture directory name itself
		// contains "overrides", so a looser substring would pass even
		// against today's unrelated "tiles[...]: no tile named" error --
		// caught exactly this way in review before this test was trusted.
		{"overrides-without-tiles", "declares overrides but tiles is empty"},
		// Whole-branch-review finding I1: object art was resolved nowhere,
		// so a pack typo produced an invisible barrier (a blocks_move
		// object whose picture never draws, with nothing telling a player
		// why). Empty art is the sharper half of that gap to close at Load
		// itself (no *Pack needed to know it): unlike a tile, an object has
		// no standard-vocabulary fallback (tools/genmappack/std_pack.go's
		// own test says objects have none), so empty art can never draw at
		// ANY pack. A non-empty but unresolvable name is the OTHER half --
		// that needs a real *Pack to disprove, so it is pinned in
		// compile_test.go instead (ResolveObjectArt, resolve.go), not here.
		{"object-art-empty", "must not be empty"},
	} {
		t.Run(c.dir, func(t *testing.T) {
			_, err := mapdef.Load(filepath.Join("testdata/invalid", c.dir, "map.json"))
			if err == nil {
				t.Fatal("this map was accepted; every square must be accounted for")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error was %q, want it to mention %q", err, c.want)
			}
		})
	}
}

// TestLoadRejectsMissingFile pins Load's base file-existence guard (not
// part of the §4.4 validation catalogue -- it never reaches JSON decoding
// at all), mirroring internal/adventure's TestLoadRejectsMissingDirectory.
func TestLoadRejectsMissingFile(t *testing.T) {
	if _, err := mapdef.Load("testdata/does-not-exist.json"); err == nil {
		t.Fatal("want an error for a missing map file")
	}
}

// writeFile writes body to path for a test, failing loudly on any write
// error rather than leaving a subsequent Load call to explain a missing
// file it never expected.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestLoadRefusesAMapWithNoFormatVersion pins design spec §7's rule: a map
// declares the format it is written in, and this server refuses to guess
// when it doesn't. No implicit fallback — a missing format_version is
// refused, never assumed to be 1, even though 1 is currently the only
// version that exists.
func TestLoadRefusesAMapWithNoFormatVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "no-version.json")
	writeFile(t, p, `{"id":"x","name":"X","grid_width":1,"grid_height":1,
		"tiles":{"0,0":"floor"}}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("want a map with no format_version refused: an undeclared format is " +
			"undeclared, and this platform does not default one")
	}
	if !strings.Contains(err.Error(), "format_version") {
		t.Fatalf("error = %q, want it to name format_version", err)
	}
}

// TestLoadRefusesAFormatThisServerDoesNotUnderstand pins the OTHER half of
// the same rule: a DECLARED format this server does not understand is
// refused by name, not guessed at or silently accepted.
//
// Two things this test must get right that a first draft got wrong: the
// fixture's tile name must be one CheckTileNamesKnown actually accepts (see
// the "stone" comment below), or Load fails for an unrelated reason before
// ever reaching the version check; and the assertion must not be satisfiable
// by t.TempDir()'s own path, which is large random digits — a bare
// strings.Contains(err, "2") / "1" passes on path noise alone even with
// Load's version-mismatch branch deleted entirely. Asserting on the worded
// phrases below closes both gaps.
func TestLoadRefusesAFormatThisServerDoesNotUnderstand(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "future.json")
	// "stone" is a real name in standardTiles (standard.go): an invalid
	// tile name here would make CheckTileNamesKnown fail FIRST, so err !=
	// nil for the wrong reason and this test would falsely appear to
	// discriminate on the format-version check it exists to pin.
	writeFile(t, p, `{"format_version":2,"id":"x","name":"X","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"}}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("want format 2 refused while this server understands only 1")
	}
	// Assert on the WORDED phrases, not bare digits: t.TempDir()'s own path
	// contains large random integers, so a bare "2"/"1" substring check
	// would pass even with the version check deleted entirely.
	for _, want := range []string{"declares 2", "understands 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

// TestLoadAcceptsTheVersionItUnderstands pins the positive case: a map
// declaring format_version 1 (the version every other fixture in this
// package now carries) loads, and Load reports that version back on Map.
func TestLoadAcceptsTheVersionItUnderstands(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.FormatVersion != mapdef.MapFormatVersion {
		t.Fatalf("FormatVersion = %d, want %d", m.FormatVersion, mapdef.MapFormatVersion)
	}
}

// TestNoPackTypeOrLoaderRemainsInThisPackage asserts an ABSENCE, so it was
// written before the removal and failed until 2026-09-02-art-is-a-flat-library
// Task 7 landed it — the shape client/test/command-surface.test.ts uses for
// create_scene and for retraction, and the shape that plan's Task 9 generalises
// into tools/check-no-pack.py for the whole tree. What it does now is keep the
// pack from growing back HERE, in the package that owned it.
//
// A DELETED IDENTIFIER CANNOT BE NAMED IN A COMPILING TEST, which is why this
// reads source text rather than calling anything — the same move
// internal/identity's TestVerifyUsesConstantTimeCompare and
// internal/gateway's TestServeNeverClosesAConnectionsOutboundChannel already make on
// their own files. It matches
// It matches a LIST OF NAME STRINGS, not "declarations" in any grammatical
// sense, and the difference is what review found on 2026-09-06: the sentence
// here used to claim it matched declarations, which made the list look
// exhaustive when it was six names. What is on the list instead is every
// SPELLING a resurrection would use, including the `type Pack`/`*Pack`/`Pack{`
// forms and the `Package` ones the tree-wide gate structurally cannot see.
//
// THIS TEST GOT MORE LOAD-BEARING ON 2026-09-06, NOT LESS. Until that day the
// package still declared mapJSON.Pack and mapJSON.Package, so the container's
// own word lived here legitimately and the list had to route around it. Patrik
// ruled the migration route out and both fields went; the file format is now
// refused by decodeStrict's DisallowUnknownFields, which knows no names at all.
// This list is what is left that knows the names — the only thing in this
// package that would notice `type Pack` or `LoadPackage` coming back.
func TestNoPackTypeOrLoaderRemainsInThisPackage(t *testing.T) {
	// Reads this package's own directory: `go test` runs a test binary with
	// its cwd set to the package under test, which is what makes "." right
	// here and what apidoc_test.go's "../../docs" relies on from the other
	// direction.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// THE BARE TOKEN `Pack` IS NOT ON THIS LIST, and the reason is now only
	// about false positives rather than about self-sabotage: it was excluded
	// while mapJSON.Pack was the refusal itself, and that field left with the
	// migration route on 2026-09-06. What is banned is every way of DECLARING
	// or USING a type by that name — `type Pack`, `*Pack`, `Pack{` — which is
	// what a resurrected container actually looks like and which the six name
	// strings below do not cover. Review, 2026-09-06: adding `type Pack struct`
	// and `func Load(dir string) (*Pack, error)` to load.go left this test and
	// tools/check-no-pack.py BOTH green, because the gate's exemption for this
	// file allows the word `Pack` and this list did not carry the forms.
	//
	// THE `Package` SPELLINGS ARE HERE FOR A DIFFERENT REASON. The tree-wide
	// gate carves `pack`-followed-by-`age` out of its needle so Go's keyword
	// does not red every file, and that carve-out is a SUBSTRING one, so
	// `ArtPackage` and `LoadPackage` escape it entirely. This package is where
	// the container lived, so this is where that spelling is refused. Note
	// `type Pack` and `*Pack` already catch `type Package` and `*Package` by
	// substring; `Package{`, `ArtPackage`, `LoadPackage` and `PackageTile` do
	// not follow and are named. The bare word `Package` is NOT banned — this
	// file's own `// Package mapdef` doc line survives comment-stripping in
	// spirit.
	banned := []string{"LoadPack", "PackTile", "PackFormatVersion",
		"packJSON", "packTileJSON", "packTileMap",
		"type Pack", "*Pack", "Pack{",
		"Package{", "ArtPack", "ArtPackage", "LoadPackage", "PackageTile"}
	var scanned int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		// COMMENTS STRIPPED, the same way internal/gateway's
		// server_internal_test.go strips them before its own source assertion,
		// and for the same reason: MapFormatVersion's doc comment says truthfully
		// that PackFormatVersion used to sit beside it, and a gate that cannot
		// tell code from the comment about the code trains people to delete the
		// comment. Crude (a "//" inside a string literal would truncate that
		// line), which is safe in this direction — it can only make the scanned
		// text shorter, never invent a match.
		text := stripGoComments(string(src))
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s still names %q in CODE — the pack left this package at "+
					"2026-09-02-art-is-a-flat-library Task 7, and art resolves by "+
					"filename through internal/artlib now", name, b)
			}
		}
	}
	// Without this the test passes vacuously the day someone moves the
	// package or breaks the cwd assumption above.
	if scanned == 0 {
		t.Fatal("scanned no non-test .go files in this package; the assertion above proved nothing")
	}
}

// stripGoComments blanks everything from the first "//" on each line, so a
// source assertion reads CODE rather than the prose about it. See
// TestNoPackTypeOrLoaderRemainsInThisPackage for why, and for the one way it is
// crude.
func stripGoComments(src string) string {
	var out strings.Builder
	for line := range strings.Lines(src) {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i] + "\n"
		}
		out.WriteString(line)
	}
	return out.String()
}

// TestAMapDeclaringAPackOrAPackageIsStillRefused is what survives the deletion
// of the migration route (Patrik's ruling, 2026-09-06: "We never used the
// platform, there is no need for a migration route. We talked about this
// before."). Three refusals carried instructions for moving a pack's pictures
// into art/; nothing has ever shipped, contract/RELEASED does not exist, and
// every campaign that has ever existed is in this repository, all fourteen
// pre-migration fixtures rewritten by Task 5 of the same plan. The instructions
// were written for an audience of nobody.
//
// WHAT MUST NOT GO WITH THEM IS THE REFUSAL, and it does not: loadAs decodes
// through decodeStrict's DisallowUnknownFields, so a key mapJSON has no field
// for fails before any check in this package runs. That is asserted here rather
// than argued from the decoder's documentation, because the arms this replaces
// are exactly what the argument used to rest on.
//
// PRESENCE, NOT VALUE, which is what the deleted arms refused too — and it now
// comes free rather than costing a json.RawMessage per spelling.
// DisallowUnknownFields fires on the KEY, so `""`, `null` and a named container
// are one case rather than three shapes a Go type had to be picked to tell
// apart. All three are still driven here: `null` is what an author, or an LLM
// told to remove a field, writes second, and it is the shape that once loaded
// SILENTLY under a *string (review, 2026-09-04).
//
// `package` IS HERE FOR ITS OWN REASON. tools/check-no-pack.py carves
// `pack`-followed-by-`age` out of its needle so Go's own keyword does not red
// every file in the repository, and that carve-out is a SUBSTRING one, so a
// container renamed `ArtPackage` escapes the tree-wide gate entirely. The file
// format is one of the two halves that CAN be closed, and strict decoding closes
// it for every spelling at once, this one included.
//
// It asserts `unknown field "pack"` rather than the bare word: t.TempDir()'s
// path carries this test's own name, so a bare "pack" substring check would pass
// on path noise alone with the strictness switched off — the trap
// TestLoadRefusesAFormatThisServerDoesNotUnderstand records for bare digits,
// which is the same trap.
func TestAMapDeclaringAPackOrAPackageIsStillRefused(t *testing.T) {
	for _, key := range []string{"pack", "package"} {
		for _, c := range []struct{ name, value string }{
			{"named", `"cellar-basics"`},
			{"empty-string", `""`},
			{"null", `null`},
		} {
			t.Run(key+"/"+c.name, func(t *testing.T) {
				dir := t.TempDir()
				p := filepath.Join(dir, "old.json")
				// Valid in every other respect — "stone" is a real name in
				// standardTiles — so a refusal here can only be the container.
				// A fixture broken some other way would make this test pass for
				// a reason that has nothing to do with what it claims.
				writeFile(t, p, `{"format_version":1,"id":"old","name":"Old","grid_width":1,
					"grid_height":1,"tiles":{"0,0":"stone"},"`+key+`":`+c.value+`}`)

				_, err := mapdef.Load(p)
				if err == nil {
					t.Fatalf("a map declaring %q:%s was accepted: its overrides name art in a "+
						"namespace that no longer exists", key, c.value)
				}
				if want := `unknown field "` + key + `"`; !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %q, want it to contain %q", err, want)
				}
			})
		}
	}
}

// TestAMapNamingNoContainerAtAllStillLoads is the negative control for the
// refusal above: it must fire on the declaration and on nothing else. Without
// it, strictness that rejected every map in the world would pass every
// assertion up there.
//
// IT IS ONE TEST WHERE IT WAS TWO. TestAMapWithNoPackFieldStillLoads and
// TestAMapWithNoPackageFieldStillLoads each stood as the control for its own
// refusal arm; with both arms gone the two fixtures were byte-identical — a map
// with no container key of any spelling — and two copies of one assertion are
// not two assertions.
func TestAMapNamingNoContainerAtAllStillLoads(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ordinary.json")
	writeFile(t, p, `{"format_version":1,"id":"ordinary","name":"Ordinary","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"}}`)

	if _, err := mapdef.Load(p); err != nil {
		t.Fatalf("load: %v — a map that names no container is the ordinary case", err)
	}
}

// THE FOUR MIGRATION TESTS THAT STOOD HERE ARE GONE, with the two refusal arms
// and the two mapJSON fields that existed only to feed them (Patrik's ruling,
// 2026-09-06: "We never used the platform, there is no need for a migration
// route"). This is where each property they pinned lives now — written down
// because deleting a function makes the compiler shout and deleting a test
// makes nothing shout at all. It is the SECOND such record in this file, and
// the block below it is the first: the pack left this package in two acts, and
// each act owes the reader the same accounting.
//
//   - TestAMapDeclaringAPackIsRefusedByName pinned two things at once, and only
//     one of them survives. "A map carrying `pack` is REFUSED" is
//     TestAMapDeclaringAPackOrAPackageIsStillRefused above, through
//     decodeStrict's DisallowUnknownFields, which is where it always actually
//     came from — loadAs's own arm could only be reached by a decoder that had
//     already accepted the field. "…with a message naming the field and
//     pointing at art/" is GONE ON PURPOSE: it was migration instructions, and
//     `unknown field "pack"` is adequate for the only people who will ever read
//     it. Nothing has shipped, contract/RELEASED does not exist, and Task 5 of
//     this plan already rewrote all fourteen fixtures that declared one.
//   - TestAMapDeclaringAnEmptyPackIsRefusedToo pinned PRESENCE rather than
//     value, over `""` and `null`. Both are cases of the test above.
//     mapJSON.Pack's json.RawMessage — chosen in review on 2026-09-04 because a
//     `*string` reads `null` as an absent key and let it load — has no successor
//     and needs none: DisallowUnknownFields decides on the KEY, so every way of
//     writing a value is one case rather than a Go type to be picked carefully.
//   - TestAMapDeclaringAPackageIsRefusedByName pinned the one rename
//     tools/check-no-pack.py structurally cannot see. It is the `package` arm of
//     the test above, and strict decoding closes it for every unknown spelling
//     at once rather than for the one that was guessed.
//   - TestAPackIsTheFirstThingReportedAboutAPreMigrationMap pinned that a map
//     which BOTH declares a container and names a square outside its grid is
//     told about the container. NOT carried over, and it cannot be:
//     decodeStrict runs before every check in loadAs, so the file never reaches
//     a geometry check to lose the race. The property is structural where it
//     used to be a choice about arm order, and there is no fault to inject that
//     would reorder it without also deleting the refusal the test above pins.

// THE FOUR LoadPack TESTS THAT STOOD HERE ARE GONE, with LoadPack itself
// (2026-09-02-art-is-a-flat-library Task 7), and this is where each property
// they pinned lives now — written down because deleting a function makes the
// compiler shout and deleting a test makes nothing shout at all.
//
//   - "a declared pack format this server does not understand is refused by
//     NAME, not guessed at": internal/artlib's
//     TestADeclaredFormatVersionThisServerDoesNotUnderstandCarriesItsOwnSentinel,
//     driven against an art sidecar. An art sidecar is what carries a
//     format_version now, and for the same reason a pack did: one picture is
//     named by many maps, so its format has to move independently of the map
//     format (see MapFormatVersion's own doc comment, format.go).
//   - "a pack with no format_version is refused": NOT carried over as a
//     refusal, and the answer changed rather than moved. Patrik's ruling of
//     2026-09-04 split the art side — a sidecar this server cannot read
//     degrades one square, and only a DECLARED version it does not understand
//     refuses — and an undeclared version is the first of those.
//     internal/artlib's
//     TestASidecarThatDeclaresNoFormatVersionIsCorruptRatherThanNewer
//     is where the field is still required, and internal/mapdef's
//     TestACorruptSidecarDegradesTheSquareAndNamesTheCause is what the caller
//     does with it. A MAP with no format_version is still
//     refused outright — TestLoadRefusesAMapWithNoFormatVersion above — and
//     the two are different files with different readers.
//   - "a pack declaring the version it understands loads, so the two refusals
//     above fail for the version reason and no other": the positive arm of
//     every internal/artlib Lookup test, e.g.
//     TestTileArtDeclaresItsNature, which loads format_version 1 and reads the
//     nature back.
//   - "a missing pack directory is an error rather than a nil Tiles map read
//     later": NOT carried over, deliberately — the answers genuinely differ.
//     internal/artlib's TestACampaignWithNoArtDirectoryDegrades pins that an
//     absent art/ costs one square its picture and a warning rather than the
//     map (design spec §4), and TestAnArtPathThatIsNotADirectoryIsRefused
//     pins the one shape that IS still an error. There is no manifest to read
//     a nil field off, so the failure that test protected against cannot occur.
//   - "two tiles or two objects sharing a name fail loud rather than the second
//     silently overwriting the first" and "a tile with no name at all is
//     refused": GONE, and gone on purpose. Uniqueness is the filesystem's now
//     (that plan's Global Constraints: "No duplicate check is written
//     anywhere") — one directory cannot hold two entries of the same name, and
//     a file always has a name. What replaces the empty-name refusal is
//     internal/artlib's TestValidateRefusesAFilenameThatIsNotAnArtName and
//     TestLookupRefusesAnIdThatIsNotAFilename, which refuse a name that is not
//     a legal art id at all.

// --- cell_px is a property of the MAP (art-is-a-flat-library, 2026-09-05) ----
//
// PATRIK'S RULING, taken from how MapTool solves the same problem: grid size
// lives on the ZONE, not on the campaign — `Grid.size`, per map, clamped
// MIN_GRID_SIZE 9 to MAX_GRID_SIZE 350, default 100. That is right and this
// sub-project's first placement was wrong. Design spec §6 argued "a grid is
// uniform... One number per campaign says that plainly", and the sentence is
// true of ONE map and false of a campaign: grid size is exactly what varies
// between an art set drawn at 64 and one drawn at 128, so the first time a DM
// installs both, a campaign-wide number is wrong for one of them.
//
// campaign.json keeps its value as the DEFAULT a map inherits when it declares
// none, which is every map that exists today.

// TestAMapMayDeclareItsOwnCellPx is the field's whole reason to be here.
func TestAMapMayDeclareItsOwnCellPx(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hall.json")
	writeFile(t, p, `{"format_version":1,"id":"hall","name":"Hall","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"},"cell_px":128}`)

	m, err := mapdef.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CellPx != 128 {
		t.Fatalf("CellPx = %d, want 128 — the map's own declaration", m.CellPx)
	}
}

// TestAMapDeclaringNoCellPxInheritsRatherThanGuesses pins the ZERO, and the
// zero is load-bearing: it is how a *Map says "I did not declare one" to the
// caller that holds the campaign default. Every map in this repo is in this
// state, so a Load that invented a number here would silently override a
// campaign that had set one.
func TestAMapDeclaringNoCellPxInheritsRatherThanGuesses(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hall.json")
	writeFile(t, p, `{"format_version":1,"id":"hall","name":"Hall","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"}}`)

	m, err := mapdef.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CellPx != 0 {
		t.Fatalf("CellPx = %d, want 0 — an undeclared field is not a declaration, and 0 is how "+
			"this type says so to whoever holds the campaign default", m.CellPx)
	}
}

// TestACellPxOutsideTheBoundsIsRefusedByName is MapTool's clamp, kept as a
// REFUSAL rather than a silent clamp-into-range: a file that says 100000 means
// something, and quietly serving 1024 instead would be the "silently ignoring"
// failure this format refuses everywhere else — the same posture decodeStrict
// takes to a key it has no field for (TestAMapDeclaringAPackOrAPackageIsStillRefused).
//
// The message names BOTH bounds, because an author who got one wrong cannot
// tell from a message that names only the one they crossed.
func TestACellPxOutsideTheBoundsIsRefusedByName(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"zero", "0"},
		{"negative", "-64"},
		{"below the floor", "1"},
		{"absurdly large", "100000"},
		{"not a whole number", "63.5"},
		{"not a number at all", `"64"`},
		{"null", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "hall.json")
			writeFile(t, p, `{"format_version":1,"id":"hall","name":"Hall","grid_width":1,
				"grid_height":1,"tiles":{"0,0":"stone"},"cell_px":`+tc.value+`}`)

			_, err := mapdef.Load(p)
			if err == nil {
				t.Fatalf("a map declaring cell_px %s was accepted", tc.value)
			}
			if !strings.Contains(err.Error(), `field "cell_px"`) {
				t.Errorf("error = %q, want it to name the field", err)
			}
		})
	}
}

// TestTheCellPxBoundsAreNamedInTheRefusal keeps the message from degrading to
// "invalid": the numbers an author has to choose between are the whole content
// of it.
func TestTheCellPxBoundsAreNamedInTheRefusal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hall.json")
	writeFile(t, p, `{"format_version":1,"id":"hall","name":"Hall","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"},"cell_px":100000}`)
	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("cell_px 100000 was accepted")
	}
	for _, want := range []string{
		fmt.Sprint(mapdef.MinCellPx), fmt.Sprint(mapdef.MaxCellPx), "100000",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}
}

// TestTheBoundsAreTheOnesTheFormatDocuments guards the two constants against
// drifting away from the doc that tells an author what to write, and against
// a bound so wide it excludes nothing. MapTool's own pair is 9..350; ours is
// wider at the top because nothing here renders at native size, and the point
// of the ceiling is to catch a typo rather than to police resolution.
func TestTheBoundsAreTheOnesTheFormatDocuments(t *testing.T) {
	if mapdef.MinCellPx != 8 {
		t.Errorf("MinCellPx = %d, want 8", mapdef.MinCellPx)
	}
	if mapdef.MaxCellPx != 1024 {
		t.Errorf("MaxCellPx = %d, want 1024", mapdef.MaxCellPx)
	}
	// The bounds must admit the default, or every campaign that declares
	// nothing is holding a number its own maps could not declare.
	if mapdef.MinCellPx > 64 || mapdef.MaxCellPx < 64 {
		t.Errorf("the bounds %d..%d exclude the documented default 64",
			mapdef.MinCellPx, mapdef.MaxCellPx)
	}
}

// TestTheBoundsThemselvesAreAccepted pins the edges as INCLUSIVE, which a
// `<`/`<=` slip changes without any other test noticing.
func TestTheBoundsThemselvesAreAccepted(t *testing.T) {
	for _, v := range []int32{mapdef.MinCellPx, mapdef.MaxCellPx} {
		dir := t.TempDir()
		p := filepath.Join(dir, "hall.json")
		writeFile(t, p, fmt.Sprintf(`{"format_version":1,"id":"hall","name":"Hall","grid_width":1,
			"grid_height":1,"tiles":{"0,0":"stone"},"cell_px":%d}`, v))
		m, err := mapdef.Load(p)
		if err != nil {
			t.Fatalf("cell_px %d (a bound) was refused: %v", v, err)
		}
		if m.CellPx != v {
			t.Errorf("CellPx = %d, want %d", m.CellPx, v)
		}
	}
}

// TestNoMapMessageCarriesUnboundedAuthorBytes is artlib's invariant on this
// side of the seam: a map file's own bytes must not reach a caller whole.
//
// These go to CommandResult.ERROR rather than .warnings — handleLoadMap's
// refusal arm — and that half predates the art sub-project entirely. A map is
// author-controlled in the same way a sidecar is: its keys, its tile names and
// its token ids are all typed by whoever wrote the file.
func TestNoMapMessageCarriesUnboundedAuthorBytes(t *testing.T) {
	const huge = 20000
	big := strings.Repeat("A", huge)

	for _, tc := range []struct{ name, body string }{
		{"an unknown field", `{"format_version":1,"id":"m","name":"M","grid_width":1,` +
			`"grid_height":1,"tiles":{"0,0":"stone-wall"},%q:1}`},
		{"a tile name nothing knows", `{"format_version":1,"id":"m","name":"M","grid_width":1,` +
			`"grid_height":1,"tiles":{"0,0":%q}}`},
		{"a square key outside the grid", `{"format_version":1,"id":"m","name":"M","grid_width":1,` +
			`"grid_height":1,"tiles":{"0,0":"stone-wall"},"overrides":{%q:"x"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapsDir := filepath.Join(t.TempDir(), "maps")
			writeInstalled(t, mapsDir, "m.json", fmt.Sprintf(tc.body, big))

			_, err := mapdef.LoadInstalled(mapsDir, "m", "")
			if err == nil {
				t.Fatal("want an error: this map file is broken")
			}
			if n := len(err.Error()); n > 4*artlib.MaxMessage {
				t.Errorf("error is %d bytes from a %d-byte value — the file's bytes "+
					"are reaching the caller substantially whole", n, huge)
			}
			if !strings.Contains(err.Error(), "maps/m.json") {
				t.Errorf("error = %q, want it to still name the file", err.Error())
			}
		})
	}
}
