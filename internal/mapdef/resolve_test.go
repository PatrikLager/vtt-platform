package mapdef_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// writeArt installs one piece of art in dir the way a DM does: a picture, and
// beside it a sidecar when sidecarJSON is non-empty. It is a deliberate copy
// of internal/artlib's own fixture helper of the same name rather than a
// shared one, because that package's tests live in package artlib_test and a
// test helper is not part of an API another package gets to import — the
// alternative is an exported testing surface on artlib that production code
// would see too.
func writeArt(t *testing.T, dir, stem, sidecarJSON string) {
	t.Helper()
	if sidecarJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, stem+".json"), []byte(sidecarJSON), 0o600); err != nil {
			t.Fatalf("write sidecar %s: %v", stem, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, stem+".png"), []byte("fake-png"), 0o600); err != nil {
		t.Fatalf("write picture %s: %v", stem, err)
	}
}

// cellarArtDir installs exactly the art testdata/valid/cellar.json names, plus
// the two pieces the tests below reach for by hand. It is the flat successor
// to the testdata/packs/mossy-keep manifest that 2026-09-02-art-is-a-flat-
// library Task 7 deleted, and the three pieces are chosen for the same
// reasons that pack's own entries were:
//
//   - planks-split-3 declares material "resin" while the square under it is
//     "wood", so a test asserting Material comes from m.Tiles has something
//     real to catch.
//   - mystery-flagstone declares NO kind, so "not declared" and "declared as
//     something else" can be told apart.
//   - boulder-mossy-2 has no sidecar at all, which is what object art is
//     allowed to be (spec §3.4).
func cellarArtDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeArt(t, dir, "planks-split-3", `{"format_version":1,"kind":"floor","material":"resin"}`)
	writeArt(t, dir, "mystery-flagstone", `{"format_version":1,"material":"slate"}`)
	writeArt(t, dir, "boulder-mossy-2", "")
	return dir
}

// Only Material is pinned here against a value the art itself disagrees with
// ("resin" in the sidecar, "wood" expected): the fixture's Kind ("floor")
// deliberately still matches the base, so a mutation that takes Kind from the
// art would NOT fail this test — that mutation is
// TestAWallDrawnAsFloorboardsIsStillAWall's job below, which uses a base with
// a genuinely different Kind on purpose.
func TestOverrideChangesThePictureAndNothingElse(t *testing.T) {
	// The error is CHECKED, not discarded, and it was discarded until
	// 2026-09-04. A broken fixture then reached Resolve as a nil *Map and the
	// test segfaulted instead of saying which file failed to load — observed
	// while Task 5 of 2026-09-02-art-is-a-flat-library was migrating this
	// package's fixtures. A panic and a stack trace is not a worse message
	// than a sentence; it is a different question entirely.
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, _, err := mapdef.Resolve(m, cellarArtDir(t), "1,1")
	if err != nil {
		t.Fatal(err)
	}
	// Nature comes from tiles["1,1"] == "wood". The override supplies art ONLY.
	if got.Kind != "floor" || got.Material != "wood" {
		t.Fatalf("the override changed the square's nature: %+v", got)
	}
	if got.Art != "planks-split-3" {
		t.Fatalf("art is %q, want planks-split-3", got.Art)
	}
}

func TestAWallDrawnAsFloorboardsIsStillAWall(t *testing.T) {
	// Spec §3.2, and it is deliberately NOT an error: this is how an illusory
	// wall is built, one arc before illusions become a feature.
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m.Overrides["0,0"] = "planks-split-3" // floor art on a wall square

	got, warnings, err := mapdef.Resolve(m, cellarArtDir(t), "0,0")
	if err != nil {
		t.Fatalf("a kind mismatch was REFUSED; it must only warn: %v", err)
	}
	if got.Kind != "wall" {
		t.Fatalf("art decided the nature: kind is %q, want wall", got.Kind)
	}
	if len(warnings) == 0 {
		t.Fatal("a kind mismatch produced no warning at all")
	}
}

// FOUR LoadPack TESTS STOOD HERE and left with LoadPack itself
// (2026-09-02-art-is-a-flat-library Task 7) — a missing pack directory, a
// duplicate tile name, a duplicate object name, an empty tile name.
// load_test.go carries the obituary: where each property lives now, and which
// two are deliberately gone rather than moved.

// TestResolveWithNoOverrideReturnsJustTheBaseNature pins Resolve's plain
// path: most squares on a real map carry no override at all, so this is the
// common case, not an edge case -- without a test dedicated to it, every
// other test here happens to also set an override, and that path could
// silently break (e.g. always return a non-empty Art) without any test
// noticing.
func TestResolveWithNoOverrideReturnsJustTheBaseNature(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, warnings, err := mapdef.Resolve(m, cellarArtDir(t), "0,0") // stone-wall, no override
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Kind != "wall" || got.Material != "stone" || got.Art != "" {
		t.Fatalf("got %+v, want kind=wall material=stone with no art", got)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings for a square with no override: %v", warnings)
	}
}

// TestResolveRejectsASquareTheMapDoesNotName pins the guard against a
// square key the map's own grid never declared -- distinct from the
// art-side degrade below, this one fires before art is ever consulted, and
// it is still a REFUSAL: a square that is not on the map has no nature to
// fall back to, which is exactly what makes degrading the art-side case safe.
func TestResolveRejectsASquareTheMapDoesNotName(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, _, err := mapdef.Resolve(m, cellarArtDir(t), "99,99"); err == nil {
		t.Fatal("want an error for a square the map does not name")
	}
}

// TestResolveRejectsABaseTileNameOutsideTheStandardVocabulary is defensive:
// Load already refuses this at load time (checkTileNamesKnown), so it can
// never happen to a *Map that came from Load. But Resolve is its own
// exported function, callable with any *Map a caller assembles by hand
// (tests, or a future construction path) -- it must not trust m.Tiles
// blindly just because Load usually sits in front of it.
func TestResolveRejectsABaseTileNameOutsideTheStandardVocabulary(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m.Tiles["0,0"] = "not-a-real-tile"
	if _, _, err := mapdef.Resolve(m, cellarArtDir(t), "0,0"); err == nil {
		t.Fatal("want an error for a base tile name outside the standard vocabulary")
	}
}

// TestAnOverrideResolvedAgainstNoArtDirectoryAtAllDegrades is the successor to
// TestResolveRejectsAnOverrideWithNoPackGiven, which pinned the `p == nil`
// refusal this task deleted. The situation it guarded is still reachable — a
// caller with an override in hand and nowhere for it to resolve from — and the
// answer has inverted: a campaign that has installed no art is ordinary, and
// its maps still load and draw plain (spec §4). What must NOT happen is a
// crash or a refusal, and this is what says so.
func TestAnOverrideResolvedAgainstNoArtDirectoryAtAllDegrades(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, warnings, err := mapdef.Resolve(m, filepath.Join(t.TempDir(), "no-art-here"), "1,1")
	if err != nil {
		t.Fatalf("resolve with no art directory: %v — a campaign with no art still loads", err)
	}
	if got.Kind != "floor" || got.Material != "wood" || got.Art != "" {
		t.Fatalf("got %+v, want the nature from Tiles and no art", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "planks-split-3") {
		t.Fatalf("warnings %q must name the reference that did not resolve", warnings)
	}
}

// TestTileArtWithNoSidecarDegradesAndNamesTheFileToWrite is Patrik's ruling of
// 2026-09-03, and it REPLACES the refusal round 1 of this task shipped.
//
// The refusal was defensible on its own terms — a bare PNG is complete OBJECT
// art (spec §3.4, and TestObjectArtNeedsNoSidecar below pins that) and
// incomplete TILE art, since a square's kind is a fact the engine acts on —
// but its blast radius was not. composeServer turns ANY loadMapsDir error into
// a refusal to start, so one missing JSON file next to one picture stopped the
// whole server booting: the campaign down for everyone, over exactly the move
// the design teaches (drop a PNG into art/ and use it). Degrading costs one
// square its picture.
//
// THE WARNING MUST NOT SAY "NOT INSTALLED", which was the one true half of the
// original argument: the file is sitting in art/ where the DM can see it, and
// sending them to hunt for a missing picture would waste the trip. It names
// the sidecar to write instead, which is the whole remedy.
//
// Spec §3.4 and exit criterion 6 carry this ruling as of 2026-09-03. Criterion
// 6 read "tile art without one is refused" until that amendment, so a reader
// holding an older copy of the spec will find it disagreeing with this test;
// the amended text is the one that stands.
func TestTileArtWithNoSidecarDegradesAndNamesTheFileToWrite(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "bare-picture", "")
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "bare-picture"}}
	got, warnings, err := mapdef.Resolve(m, artDir, "0,0")
	if err != nil {
		t.Fatalf("Resolve: %v — a picture with no sidecar degrades one square, "+
			"it does not refuse the map (Patrik, 2026-09-03)", err)
	}
	if got.Kind != "wall" || got.Material != "stone" || got.Art != "" {
		t.Fatalf("got %+v, want the nature from Tiles and no art", got)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
	if !strings.Contains(warnings[0], "art/bare-picture.json") {
		t.Fatalf("warning = %q, want it to name the sidecar to write", warnings[0])
	}
	if strings.Contains(warnings[0], "not installed") {
		t.Fatalf("warning = %q — the picture IS installed; saying otherwise sends the "+
			"DM hunting for a file that is sitting in art/", warnings[0])
	}
}

// TestAnUnopenableArtDirectoryDegradesAtResolveTime is the request-time half
// of Patrik's second ruling of 2026-09-03. An art/ that exists and cannot be
// opened — a plain file where the directory belongs, or a mode that forbids it
// — is a broken installation, and every lookup against it fails identically.
//
// Resolve DEGRADES it, because Resolve is what runs when a DM issues load_map:
// a DM cannot chmod a directory from a browser, and a campaign that worked
// five minutes ago should not stop working at the table. The boot walk refuses
// the same condition, where an operator is at a terminal — cmd/vtt's
// TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists pins that half
// (this cited TestLoadMapsDirFailsLoudWhenTheArtRootCannotBeOpened, a name no
// test in the tree has ever carried; corrected 2026-09-04), and the two halves
// together are deliberate asymmetry rather than a divergence.
func TestAnUnopenableArtDirectoryDegradesAtResolveTime(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "art")
	if err := os.WriteFile(notADir, []byte("a file where art/ belongs"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "masonry-1"}}

	got, warnings, err := mapdef.Resolve(m, notADir, "0,0")
	if err != nil {
		t.Fatalf("Resolve: %v — an unopenable art root degrades at request time", err)
	}
	if got.Kind != "wall" || got.Material != "stone" || got.Art != "" {
		t.Fatalf("got %+v, want the nature from Tiles and no art", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "art directory cannot be read") {
		t.Fatalf("warnings = %v, want one saying the art directory cannot be read", warnings)
	}
}

// TestNoArtFailureNamesTheDirectoryItRead is the class guard for the path
// disclosure this task introduced and round 1 of it missed. Round 1's own
// report claimed the leak was unreachable; it was reachable through
// load_adventure, which resolves art PER REQUEST (adventure.Compile) and
// returns err.Error() verbatim — to RoleAgent as well as RoleDM
// (internal/gateway/authz.go). An MCP agent seat received the operator's
// filesystem layout.
//
// WARNINGS ARE CHECKED AS WELL AS ERRORS, and that is the half a
// refusal-shaped test would miss: a warning rides back on an ok=true
// CommandResult to exactly the same seats (CommandResult.warnings, field 5),
// so mapdef.LoadInstalled's written promise that no error names the path it
// opened has to cover them too.
//
// THIS TABLE IS NOT A CLOSED LIST, and the sentence it replaces claimed it
// was: "the fixtures are every art failure that can carry a path". Three were
// named; a fourth existed and was the one leaking. A reader consults exactly
// that kind of sentence before deciding whether to widen a table, so it stopped
// the leak being found for a round.
//
// What decides the coverage is internal/artlib's TestNoLookupErrorNamesThe
// DirectoryItRead, which walks SYSCALL PHASES (openat, read, statat, parse)
// rather than failures anyone thought of — see bareCause's doc comment for why
// the read phase is the one that carries an absolute path. The cases here are
// this layer's end-to-end echo of that: the two unopenable-root shapes, a
// directory wearing a sidecar's name (the read phase), and a sidecar that
// cannot be parsed. artDir sits INSIDE root, so a leak of either path fails
// this.
//
// FOUR OF THE FIVE DEGRADE AND ONE REFUSES, which is why this test collects
// warnings AND errors into one `said` string rather than asserting on an
// error. Three were refusals when it was written: the unopenable root became a
// warning on 2026-09-03, and the read-phase and parse-phase sidecars on
// 2026-09-04. A version of this that only read err would have gone vacuous,
// silently, on each of those days — the assertion would still run and there
// would be nothing left in it.
//
// THE PARSE ROW AND THE VERSION ROW ARE SEPARATE ROWS, and they were one row
// carrying the wrong fixture until 2026-09-04: it was called "sidecar cannot
// be parsed" and its fixture was `{"format_version":99}`, which is not a parse
// failure at all. That mattered the moment the two answers diverged — the row
// exercised the error path under a name that now describes the warning path,
// so the parse phase would have been covered by nothing.
func TestNoArtFailureNamesTheDirectoryItRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, root string) string
	}{
		{"art root is a plain file", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			if err := os.WriteFile(p, []byte("not a dir"), 0o600); err != nil {
				t.Fatal(err)
			}
			return p
		}},
		{"art root cannot be opened", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			if err := os.Mkdir(p, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o700) })
			return p
		}},
		{"a directory wearing a sidecar's name", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			if err := os.MkdirAll(filepath.Join(p, "masonry-1.json"), 0o750); err != nil {
				t.Fatal(err)
			}
			return p
		}},
		{"sidecar cannot be parsed", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			if err := os.Mkdir(p, 0o750); err != nil {
				t.Fatal(err)
			}
			// Truncated, so the JSON decoder is what fails. This DEGRADES,
			// so it is exercised through a warning.
			writeArt(t, p, "masonry-1", `{"format_version":1,"kind":"wa`)
			return p
		}},
		{"sidecar declares a format this server does not understand", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			if err := os.Mkdir(p, 0o750); err != nil {
				t.Fatal(err)
			}
			// The one art failure that still REFUSES, so it is the one row
			// here exercised through an error.
			writeArt(t, p, "masonry-1", `{"format_version":99}`)
			return p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			artDir := tc.build(t, root)
			m := &mapdef.Map{ID: "hall", Name: "Hall", GridW: 1, GridH: 1,
				Tiles:     map[string]string{"0,0": "stone-wall"},
				Overrides: map[string]string{"0,0": "masonry-1"},
				Objects:   []mapdef.Object{{ID: "o1", Kind: "barrel", Art: "masonry-1"}}}

			_, warnings, err := mapdef.Compile(m, artDir)
			said := strings.Join(warnings, "\n")
			if err != nil {
				said += "\n" + err.Error()
			}
			if said == "" {
				t.Fatal("neither an error nor a warning: this fixture is broken and must say something")
			}
			for _, secret := range []string{artDir, root, os.TempDir()} {
				if strings.Contains(said, secret) {
					t.Errorf("a client was told where the campaign lives:\n  %s", said)
				}
			}
		})
	}
}

// TestResolveObjectArtRejectsEmptyArt pins ResolveObjectArt's own defensive
// empty-art guard directly (whole-branch-review finding I1) — the one
// branch CheckObjectArtDeclared (load.go) makes UNREACHABLE for any object
// that went through mapdef.Load, and so the one branch no Load-based test
// can ever exercise. It exists because BuildSceneCreated (compile.go, the
// ONE shared construction site both load paths call) can be reached with a
// hand-built *Map that never ran Load's checks at all — this package's own
// compile tests do exactly that (m.Overrides = nil, e.g.), and
// internal/adventure/load.go's own dry run builds one from raw fields
// too — so ResolveObjectArt cannot assume o.Art is already proven non-empty.
//
// It stays a REFUSAL after this task while an art name that does not resolve
// became a degrade, and the two are different facts: an object that names no
// art at all is a map file with a hole in it, not a picture that is missing.
func TestResolveObjectArtRejectsEmptyArt(t *testing.T) {
	if _, _, err := mapdef.ResolveObjectArt(0, mapdef.Object{ID: "boulder-1", Art: ""}, t.TempDir()); err == nil {
		t.Fatal("want an error resolving an object with no art, not silent success")
	}
}

// TestObjectArtThatIsNotInstalledLeavesTheObjectInPlace is spec §4's object
// half, stated there in its own paragraph: "Object art that does not resolve
// leaves the object in place, with its blocking behaviour intact, drawn from
// its kind. An object is a thing in the world before it is a picture, and
// dropping it because its picture is missing would change what the room IS."
//
// This is the successor to TestResolveObjectArtRejectsAnUnresolvableName,
// which pinned the opposite answer against a pack. What that test proved and
// this one still proves is that the name is READ at all: before
// ResolveObjectArt existed, an object's art was checked against nothing and a
// typo produced an invisible barrier nobody could explain.
func TestObjectArtThatIsNotInstalledLeavesTheObjectInPlace(t *testing.T) {
	art, warnings, err := mapdef.ResolveObjectArt(
		0, mapdef.Object{ID: "boulder-1", Art: "boulder-mosy-2"}, cellarArtDir(t))
	if err != nil {
		t.Fatalf("ResolveObjectArt: %v — the object stays, it just loses its picture", err)
	}
	if art != "" {
		t.Fatalf("art is %q, want empty: there is no picture to draw", art)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "boulder-mosy-2") {
		t.Fatalf("warnings %q must name the reference", warnings)
	}
}

// TestObjectArtDeclaringAFormatThisServerDoesNotUnderstandStillRefuses is the
// object-side counterpart of
// TestArtDeclaringAFormatThisServerDoesNotUnderstandStillRefuses. Without it
// the object path could degrade EVERYTHING — a v2 art set included — and the
// only test watching it would be the tile one. The two arms are written
// separately in resolve.go and there is nothing forcing them to agree.
func TestObjectArtDeclaringAFormatThisServerDoesNotUnderstandStillRefuses(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "future-boulder", `{"format_version":99}`)
	if _, _, err := mapdef.ResolveObjectArt(
		0, mapdef.Object{ID: "boulder-1", Art: "future-boulder"}, artDir); err == nil {
		t.Fatal("object art written for a later format was degraded; it must refuse")
	}
}

// TestObjectArtWithACorruptSidecarDegradesAndNamesTheCause is the object half
// of Patrik's 2026-09-04 ruling, and it is not implied by the tile half:
// ResolveObjectArt has its own switch, so a split applied to one arm and not
// the other compiles and passes every tile test in this file.
//
// The object stays either way (spec §4) — an object is a thing in the world
// before it is a picture — so all that changes is that the map still loads.
func TestObjectArtWithACorruptSidecarDegradesAndNamesTheCause(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "half-copied-boulder", `{"format_version":1,"kind":"bo`)
	art, warnings, err := mapdef.ResolveObjectArt(
		0, mapdef.Object{ID: "boulder-1", Art: "half-copied-boulder"}, artDir)
	if err != nil {
		t.Fatalf("ResolveObjectArt: %v — a sidecar that cannot be read costs the object "+
			"its picture, not the map its load", err)
	}
	if art != "" {
		t.Fatalf("art is %q, want empty: there is no picture to draw", art)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "half-copied-boulder") ||
		!strings.Contains(warnings[0], "cannot be used") {
		t.Fatalf("warnings %q must name the piece and why it dropped", warnings)
	}
	if strings.Contains(warnings[0], "not installed") {
		t.Fatalf("warning = %q — the file IS installed and broken", warnings[0])
	}
}

// TestObjectArtNeedsNoSidecar is §3.4's asymmetry from the side only mapdef
// can see: artlib resolves a bare picture happily either way, and it is this
// function that must NOT go on to demand the sidecar its tile sibling does.
// boulder-mossy-2 is written by cellarArtDir with no sidecar at all.
func TestObjectArtNeedsNoSidecar(t *testing.T) {
	art, warnings, err := mapdef.ResolveObjectArt(
		0, mapdef.Object{ID: "boulder-1", Art: "boulder-mossy-2"}, cellarArtDir(t))
	if err != nil {
		t.Fatalf("resolve object art: %v", err)
	}
	if art != "boulder-mossy-2" {
		t.Fatalf("art is %q, want boulder-mossy-2", art)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings for art that resolved: %v", warnings)
	}
}

// TestAnUndeclaredSidecarKindNeverProducesASpuriousMismatchWarning pins that
// an advisory kind left blank reads as "not declared", never as "declared and
// different" — the opposite reading would warn on every override drawn from a
// piece whose author hasn't classified it yet, which is exactly the noise the
// one warning channel must stay free of to remain trustworthy. artlib's
// sidecar decoder requires only format_version, so a sidecar with no "kind" is
// a legal thing to find on disk.
func TestAnUndeclaredSidecarKindNeverProducesASpuriousMismatchWarning(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m.Overrides["1,1"] = "mystery-flagstone" // art with no declared kind
	got, warnings, err := mapdef.Resolve(m, cellarArtDir(t), "1,1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Kind != "floor" {
		t.Fatalf("kind is %q, want floor (from the base tile)", got.Kind)
	}
	if len(warnings) != 0 {
		t.Fatalf("an undeclared sidecar kind produced a warning: %v", warnings)
	}
}

func TestArtThatIsNotInstalledDegradesTheSquareAndWarns(t *testing.T) {
	artDir := t.TempDir()
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "absent"}}
	got, warnings, err := mapdef.Resolve(m, artDir, "0,0")
	if err != nil {
		t.Fatalf("Resolve: %v — absent art degrades, it does not refuse (spec §4)", err)
	}
	if got.Kind != "wall" || got.Material != "stone" {
		t.Fatalf("got %+v, want the nature from Tiles, unchanged", got)
	}
	if got.Art != "" {
		t.Fatalf("got Art=%q, want empty: there is no picture to draw", got.Art)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "absent") {
		t.Fatalf("warnings %q must name the reference", warnings)
	}
}

// TestArtDeclaringAFormatThisServerDoesNotUnderstandStillRefuses is the one
// art failure left that refuses a map, and it is a fact about the SERVER
// rather than about the file (Patrik, 2026-09-04). Everything else a sidecar
// can get wrong degrades one square — see
// TestACorruptSidecarDegradesTheSquareAndNamesTheCause below, which is what
// this test used to assert.
//
// THE REFUSAL NAMES BOTH VERSIONS, because that is the whole content of the
// message: the remedy is a newer server, and a DM cannot guess which one from
// "this art is broken".
func TestArtDeclaringAFormatThisServerDoesNotUnderstandStillRefuses(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "from-the-future", `{"format_version":99,"kind":"wall","material":"stone"}`)
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "from-the-future"}}
	_, warnings, err := mapdef.Resolve(m, artDir, "0,0")
	if err == nil {
		t.Fatalf("art written for a later format was degraded (warnings %q); one refusal "+
			"naming both versions says \"this server is too old\", and ninety warnings "+
			"saying \"your art is broken\" do not", warnings)
	}
	said := err.Error()
	if !strings.Contains(said, "99") || !strings.Contains(said, "understands 1") {
		t.Fatalf("error = %q, want it to name the version the file declares AND the one "+
			"this server understands", said)
	}
}

// TestACorruptSidecarDegradesTheSquareAndNamesTheCause is Patrik's ruling of
// 2026-09-04 and the reason Task 4b exists at all. Task 4's review measured
// the cost of the refusal this replaces: composeServer turns ANY map-load
// error into a refusal to start, so ONE corrupt sidecar named by ONE committed
// map stopped the server booting — exit status 1, every other map fine, and
// the boot log saying "starting anyway" one line earlier.
//
// THE WARNING IS NOT THE NOT-INSTALLED SENTENCE, for §3.4's reason: the file
// is sitting in art/ where the DM can see it, and sending them to hunt for a
// missing picture wastes the trip. It names the piece and the cause.
//
// THE FIXTURE IS A TRUNCATED FILE, the "missing brace, a truncated copy" spec
// §4 names. A well-typed sidecar with one wrongly-typed field would work as
// well; this one is the shape an interrupted cp actually leaves.
func TestACorruptSidecarDegradesTheSquareAndNamesTheCause(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "half-copied", `{"format_version":1,"kind":"wa`)
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "half-copied"}}

	got, warnings, err := mapdef.Resolve(m, artDir, "0,0")
	if err != nil {
		t.Fatalf("Resolve: %v — a sidecar that cannot be read degrades one square; "+
			"refusing took the whole server down with it (Patrik, 2026-09-04)", err)
	}
	if got.Kind != "wall" || got.Material != "stone" || got.Art != "" {
		t.Fatalf("got %+v, want the nature from Tiles and no art", got)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
	if !strings.Contains(warnings[0], "half-copied") {
		t.Fatalf("warning = %q, want it to name the piece", warnings[0])
	}
	if strings.Contains(warnings[0], "not installed") {
		t.Fatalf("warning = %q — the file IS installed and broken; saying otherwise sends "+
			"the DM hunting for a file that is sitting in art/", warnings[0])
	}
	// The CAUSE, not merely the name. Without this the sentence could be
	// artNotInstalled with two words changed, and a DM would be told a piece
	// dropped without being told there is a file to go and fix.
	if !strings.Contains(warnings[0], "cannot be used") {
		t.Fatalf("warning = %q, want it to say why the piece dropped", warnings[0])
	}
	// THE CAUSE MUST BE THE PARSE FAILURE, and this is the assertion the whole
	// test rested on without making (review, 2026-09-05). artlib reads the
	// version in a pass of its own before the strict decode; delete that pass's
	// error return and a truncated file reaches the version check with an
	// empty format_version, so it reports `field "format_version": required`
	// instead. The verdict is identical — degrade, one warning, this square
	// plain — so nothing else here moves, and the DM is sent to add a line to a
	// file that is cut in half.
	if !strings.Contains(warnings[0], "unexpected EOF") {
		t.Fatalf("warning = %q, want it to quote the parse failure", warnings[0])
	}
	if strings.Contains(warnings[0], "format_version") {
		t.Fatalf("warning = %q blames the version field for a file that never got as far "+
			"as having one", warnings[0])
	}
}

// TestADoorMissingOnePictureDegradesRatherThanRefusingTheMap is the case
// Patrik's 2026-09-04 ruling does not name, decided here: a door sidecar
// declaring only kind and material is CORRUPT, not newer.
//
// The line the ruling draws is "is this file broken, or is this server too
// old". A door with one picture is a broken file: this server reads it
// perfectly and finds it incomplete, and the remedy is to edit that one file,
// which is exactly the remedy for a missing brace. Nothing about it says the
// content was written for a later format.
//
// IT IS SAFE FOR THE SAME REASON EVERY OTHER DEGRADE IS. The square's nature
// comes from m.Tiles ("wood-door" here), never from art, so sight and movement
// still see a door; only the picture drops. The map stays playable.
//
// AND THE SHIPPED CAMPAIGN IS WHY IT MATTERS. campaigns/example/maps/cellar.json
// overrides 5,4 with "cellar-door", so under a refusal an operator who drops
// the "open" line out of art/cellar-door.json cannot start the server at all —
// every other map in the campaign down with it. Degraded, they get one warning
// naming cellar-door, a door square drawn plain, and a table that plays.
//
// NO CODE DECIDES THIS, and that is the evidence the line is in the right
// place: the door error is simply not one of the three sentinels, so it falls
// into the degrade arm by construction. Only the fact that goes the other way
// needed a sentinel of its own.
func TestADoorMissingOnePictureDegradesRatherThanRefusingTheMap(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "cellar-door", `{"format_version":1,"kind":"door","material":"wood"}`)
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "wood-door"},
		Overrides: map[string]string{"0,0": "cellar-door"}}

	got, warnings, err := mapdef.Resolve(m, artDir, "0,0")
	if err != nil {
		t.Fatalf("Resolve: %v — an incomplete door is a broken file, not content written "+
			"for a later server, and the shipped campaign has one", err)
	}
	if got.Kind != "door" || got.Art != "" {
		t.Fatalf("got %+v, want the nature from Tiles and no art: the square is still a "+
			"door to sight and movement", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "cellar-door") ||
		!strings.Contains(warnings[0], "a door declares both") {
		t.Fatalf("warnings = %v, want one naming the piece and what its sidecar is missing",
			warnings)
	}
}

func TestASquareWithNoOverrideIsUnchanged(t *testing.T) {
	// The control. This path predates the change and must not move.
	//
	// It is not a duplicate of TestResolveWithNoOverrideReturnsJustTheBaseNature
	// above, which walks a map loaded from disk against art that is really
	// there: this one hands Resolve an EMPTY art directory, so it fails the
	// moment the artlib lookup is hoisted above the "is there an override at
	// all" guard. A hoisted lookup would ask for the id "" — which is not an
	// art id, so artlib answers ErrNotFound — and the degrade arm would then
	// produce a warning on a square that names no art at all. len(warnings) != 0
	// is what catches it.
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "earth"}}
	got, warnings, err := mapdef.Resolve(m, t.TempDir(), "0,0")
	if err != nil || len(warnings) != 0 || got.Kind != "floor" || got.Art != "" {
		t.Fatalf("got %+v warnings=%v err=%v; the no-override path must not change",
			got, warnings, err)
	}
}

// TestACaseOnlyMismatchTellsTheDMTheFilenameItFound is the actionable half of
// the exact-name rule artlib.Library enforces.
//
// A DM on macOS installs Masonry-1.png, and the square degrades — correctly,
// because that filename is not the id and would resolve on their machine and
// nowhere else. But "art "masonry-1" is not installed" is a sentence they read
// while looking straight at the file in the directory, and it sends them to
// check a path that is fine. The remedy is a rename, and nothing says so.
func TestACaseOnlyMismatchTellsTheDMTheFilenameItFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Masonry-1.png"), []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &mapdef.Map{ID: "hall", Name: "Hall", GridW: 1, GridH: 1,
		Tiles:     map[string]string{"0,0": "stone"},
		Overrides: map[string]string{"0,0": "masonry-1"}}

	_, warnings, err := mapdef.Compile(m, dir)
	if err != nil {
		t.Fatalf("Compile: %v — a case-only mismatch degrades, it does not refuse", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	// THE FILENAME ON DISK, so the DM can see what to rename.
	if !strings.Contains(warnings[0], "Masonry-1.png") {
		t.Errorf("warning = %q, want it to name the file the directory actually holds — "+
			"a DM told only 'not installed' is looking right at it", warnings[0])
	}
	// And still the degrade warning, so the existing arm's behaviour is intact.
	if !strings.Contains(warnings[0], "masonry-1") {
		t.Errorf("warning = %q, want it to name the art the map asked for", warnings[0])
	}
}

// TestAnObjectsCaseMismatchNamesTheFileToo is the object half of the same arm.
//
// The split is written out TWICE, here and in Resolve, and nothing forces the
// two switches to agree — ResolveObjectArt's own doc says so. The first version
// of the case-mismatch arm existed only on the tile side, so an object degraded
// with the plain "is not installed" while the DM looked at the file. Objects are
// where a hand-copied picture most often lands, so this is the side that needs
// the filename most.
func TestAnObjectsCaseMismatchNamesTheFileToo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Crate-Wood.png"), []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &mapdef.Map{ID: "hall", Name: "Hall", GridW: 1, GridH: 1,
		Tiles: map[string]string{"0,0": "stone"},
		Objects: []mapdef.Object{{
			ID: "obj-1", Kind: "crate", X: 0, Y: 0, W: 1, H: 1, Art: "crate-wood",
		}}}

	_, warnings, err := mapdef.Compile(m, dir)
	if err != nil {
		t.Fatalf("Compile: %v — a miscased object picture degrades, it does not refuse", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	if !strings.Contains(warnings[0], "Crate-Wood.png") {
		t.Errorf("warning = %q, want it to name the file on disk — the tile side "+
			"says it and the object side used to not", warnings[0])
	}
	// And the object still stays, which is the object arm's own promise.
	if !strings.Contains(warnings[0], "the object stays") {
		t.Errorf("warning = %q, want it to keep the object's own sentence", warnings[0])
	}
}
