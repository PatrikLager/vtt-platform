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
// to testdata/packs/mossy-keep, and the three pieces are chosen for the same
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

// TestLoadPackRejectsAMissingDirectory mirrors load_test.go's
// TestLoadRejectsMissingFile: LoadPack fails at the same os.Open boundary
// Load does (decodeStrict is shared), so a typo'd pack directory is caught
// here rather than surfacing later as Resolve reading a nil Pack.Tiles.
func TestLoadPackRejectsAMissingDirectory(t *testing.T) {
	if _, err := mapdef.LoadPack("testdata/packs/does-not-exist"); err == nil {
		t.Fatal("want an error for a missing pack directory")
	}
}

// TestLoadPackRejectsADuplicateTileName pins that two tiles sharing a name
// fail loud rather than the second silently overwriting the first in the
// Tiles map: an author would otherwise only discover the collision when the
// wrong picture shows up on a table, long after authoring, with no error
// anywhere to point at the cause.
func TestLoadPackRejectsADuplicateTileName(t *testing.T) {
	if _, err := mapdef.LoadPack("testdata/packs/invalid/duplicate-tile-name"); err == nil {
		t.Fatal("want an error for a duplicate tile name")
	}
}

// TestLoadPackRejectsADuplicateObjectName pins that packTileMap's duplicate
// check is exercised on BOTH arrays LoadPack keys by name, not just Tiles:
// Objects goes through the identical call, and a collision there is exactly
// as unreferenceable as a tile collision.
func TestLoadPackRejectsADuplicateObjectName(t *testing.T) {
	if _, err := mapdef.LoadPack("testdata/packs/invalid/duplicate-object-name"); err == nil {
		t.Fatal("want an error for a duplicate object name")
	}
}

// TestLoadPackRejectsAnEmptyTileName pins packTileMap's other refusal: a
// tile with no name at all can never be the target of an override (Overrides
// values are matched against Pack.Tiles by exact name), so it would load
// silently into the manifest and then be permanently unreachable.
func TestLoadPackRejectsAnEmptyTileName(t *testing.T) {
	if _, err := mapdef.LoadPack("testdata/packs/invalid/empty-tile-name"); err == nil {
		t.Fatal("want an error for an empty tile name")
	}
}

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
// TestLoadMapsDirFailsLoudWhenTheArtRootCannotBeOpened pins that half, and the
// two halves together are deliberate asymmetry rather than a divergence.
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
// this layer's end-to-end echo of that: the two unopenable-root shapes (which
// degrade, so they are exercised through WARNINGS rather than errors), a
// directory wearing a sidecar's name (the read phase), and a sidecar that
// cannot be parsed (which refuses). artDir sits INSIDE root, so a leak of
// either path fails this.
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

// TestObjectArtThatExistsButCannotBeReadStillRefuses is the object-side
// counterpart of TestArtThatExistsButCannotBeReadStillRefuses. Without it the
// object path could degrade EVERYTHING — a broken sidecar included — and the
// only test watching it would be the tile one.
func TestObjectArtThatExistsButCannotBeReadStillRefuses(t *testing.T) {
	artDir := t.TempDir()
	writeArt(t, artDir, "broken-boulder", `{"format_version":99}`)
	if _, _, err := mapdef.ResolveObjectArt(
		0, mapdef.Object{ID: "boulder-1", Art: "broken-boulder"}, artDir); err == nil {
		t.Fatal("malformed object art was degraded; it must refuse")
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

func TestArtThatExistsButCannotBeReadStillRefuses(t *testing.T) {
	// The distinction that makes §4 safe: ABSENT art is a square to draw
	// plain; MALFORMED art is a defect to fix. Degrading both would let a
	// broken sidecar ship silently.
	artDir := t.TempDir()
	writeArt(t, artDir, "broken", `{"format_version":99,"kind":"wall","material":"stone"}`)
	m := &mapdef.Map{Tiles: map[string]string{"0,0": "stone-wall"},
		Overrides: map[string]string{"0,0": "broken"}}
	if _, _, err := mapdef.Resolve(m, artDir, "0,0"); err == nil {
		t.Fatal("malformed art was degraded; it must refuse")
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
