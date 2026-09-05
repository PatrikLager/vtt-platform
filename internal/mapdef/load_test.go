package mapdef_test

import (
	"fmt"
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
// DECLARATIONS, not the word: mapJSON.Pack survives on purpose (it is the only
// way loadAs can refuse a file that declares one — see mapJSON's own comment),
// so a bare "Pack" search would fail forever and be deleted by whoever hit it.
func TestNoPackTypeOrLoaderRemainsInThisPackage(t *testing.T) {
	// Reads this package's own directory: `go test` runs a test binary with
	// its cwd set to the package under test, which is what makes "." right
	// here and what apidoc_test.go's "../../docs" relies on from the other
	// direction.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"LoadPack", "PackTile", "PackFormatVersion",
		"packJSON", "packTileJSON", "packTileMap"}
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

// TestAMapDeclaringAPackIsRefusedByName pins design spec §7's migration rule:
// "There is no compatibility layer, and none is added later. A map carrying a
// `"pack"` field is refused with a message naming the field and pointing at
// `art/`." Ignoring the field would load a map whose art references were
// written against a namespace that no longer exists — the override values were
// resolved inside ONE named pack, and since Task 3 of the same plan they name a
// file in the campaign's one flat art/ directory instead. Same strings, a
// different world: the map would load and draw the wrong thing, or nothing.
//
// The message is the whole of what a DM sees when a campaign authored before
// this change is opened, so the assertions below are about the message and not
// merely about err != nil: it must name the FIELD (so the line to delete is
// unambiguous) and point at art/ (so the reader knows where the pictures go
// now). fieldErr supplies the third part, the file, which LoadInstalled renders
// as the "maps/<id>.json" a DM knows rather than a server path.
//
// It asserts on `field "pack"` rather than the bare word: t.TempDir()'s path
// carries this test's own name, and a bare "pack" substring check would pass on
// path noise alone with the refusal deleted — the trap
// TestLoadRefusesAFormatThisServerDoesNotUnderstand records for bare digits,
// which is the same trap.
func TestAMapDeclaringAPackIsRefusedByName(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "old.json")
	// Everything else about this file is valid — "stone" is a real name in
	// standardTiles — so a refusal here can only be the pack declaration. A
	// fixture broken some other way would make this test pass for a reason
	// that has nothing to do with what it claims to pin.
	writeFile(t, p, `{"format_version":1,"id":"old","name":"Old","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"},"pack":"cellar-basics"}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal(`a map declaring "pack" was accepted: its overrides name art in a namespace that no longer exists`)
	}
	for _, want := range []string{`field "pack"`, "art/"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q — the message is the whole migration experience", err, want)
		}
	}
}

// TestAMapDeclaringAnEmptyPackIsRefusedToo pins that it is the FIELD'S
// PRESENCE that is refused, not a non-empty value. Spec §7 says "a map carrying
// a `"pack"` field", and the difference is not pedantry: emptying the value is
// exactly the half-migration an author — or an LLM told to remove a field —
// reaches for first, and it fixes nothing, because every overrides value under
// it is still the name it had inside the pack.
//
// BOTH WAYS OF WRITING "NOTHING" ARE HERE, and the second is why mapJSON.Pack
// is a json.RawMessage. Neither a `string` nor a `*string` can carry this rule:
// a `string` cannot tell `"pack":""` from an absent key, and a `*string` — the
// first implementation of this test, which passed — comes back nil for
// `"pack":null` exactly as it does for an absent key, so `null` LOADED. Found
// in review, 2026-09-04, by probing the four shapes rather than the one the
// test happened to drive. A json.RawMessage is nil only when the key is truly
// absent: `null` arrives as the four bytes "null", `""` as the two bytes `""`.
// A *json.RawMessage does NOT work either — it is nil for null, same trap.
func TestAMapDeclaringAnEmptyPackIsRefusedToo(t *testing.T) {
	for _, c := range []struct{ name, value string }{
		{"empty-string", `""`},
		// The one that shipped broken: told to remove "pack", an author or an
		// LLM writes null at least as readily as it deletes the line.
		{"null", `null`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "blanked.json")
			writeFile(t, p, `{"format_version":1,"id":"blanked","name":"Blanked","grid_width":1,
				"grid_height":1,"tiles":{"0,0":"stone"},"pack":`+c.value+`}`)

			_, err := mapdef.Load(p)
			if err == nil {
				t.Fatalf(`a map declaring "pack":%s was accepted: the field is what is refused, not its value`, c.value)
			}
			if !strings.Contains(err.Error(), `field "pack"`) {
				t.Fatalf("error = %q, want it to name the field — a refusal for some other reason proves nothing here", err)
			}
		})
	}
}

// TestAMapWithNoPackFieldStillLoads is the other half of the pair above, and it
// is the one that would catch the refusal firing on every map in the world: the
// ordinary, migrated file — no "pack" key at all — must load exactly as before.
func TestAMapWithNoPackFieldStillLoads(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "migrated.json")
	writeFile(t, p, `{"format_version":1,"id":"migrated","name":"Migrated","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone"}}`)

	if _, err := mapdef.Load(p); err != nil {
		t.Fatalf("load: %v — a map that names no pack is the ordinary case", err)
	}
}

// TestAPackIsTheFirstThingReportedAboutAPreMigrationMap pins WHICH error a map
// authored before the flat art library gets, not merely that it gets one. Such
// a file is very likely to have other complaints against it — the fixture below
// also names a square outside its own grid — and reporting that one first sends
// its author to fix a stray coordinate in a file whose actual problem is its
// age. Load's doc comment states the rule this follows: the first error a
// broken file produces should be the most useful one to fix.
func TestAPackIsTheFirstThingReportedAboutAPreMigrationMap(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "old-and-broken.json")
	writeFile(t, p, `{"format_version":1,"id":"old","name":"Old","grid_width":1,
		"grid_height":1,"tiles":{"0,0":"stone","9,9":"stone"},"pack":"cellar-basics"}`)

	_, err := mapdef.Load(p)
	if err == nil {
		t.Fatal("this map was accepted; it declares a pack AND names a square outside its grid")
	}
	if !strings.Contains(err.Error(), `field "pack"`) {
		t.Fatalf("error = %q, want the pack declaration reported first, not the stray square", err)
	}
}

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
// failure this format refuses everywhere else (see the "pack" refusal above).
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
