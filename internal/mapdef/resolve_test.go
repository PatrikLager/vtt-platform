package mapdef_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// Only Material is pinned here against a value the pack tile itself
// disagrees with ("resin" in the fixture, "wood" expected): the fixture's
// Kind ("floor") deliberately still matches the base, so a mutation that
// takes Kind from the pack tile would NOT fail this test — that mutation is
// TestAWallDrawnAsFloorboardsIsStillAWall's job below, which uses a base
// with a genuinely different Kind on purpose.
func TestOverrideChangesThePictureAndNothingElse(t *testing.T) {
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	got, _, err := mapdef.Resolve(m, p, "1,1")
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
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	m.Overrides["0,0"] = "planks-split-3" // a floor tile on a wall square
	p, _ := mapdef.LoadPack("testdata/packs/mossy-keep")

	got, warnings, err := mapdef.Resolve(m, p, "0,0")
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

func TestAnUnknownTileNameIsRefusedRatherThanFallingThrough(t *testing.T) {
	// There are exactly two levels and no name-chasing between packs: a custom
	// name means nothing outside the pack that defines it (spec §4.2).
	m, _ := mapdef.Load("testdata/valid/cellar.json")
	m.Overrides["1,1"] = "no-such-tile"
	p, _ := mapdef.LoadPack("testdata/packs/mossy-keep")
	if _, _, err := mapdef.Resolve(m, p, "1,1"); err == nil {
		t.Fatal("an unresolvable art name was accepted")
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
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	got, warnings, err := mapdef.Resolve(m, p, "0,0") // stone-wall, no override
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
// pack-side "unresolvable art" refusal above, this one fires before a pack
// is ever consulted.
func TestResolveRejectsASquareTheMapDoesNotName(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, _, err := mapdef.Resolve(m, p, "99,99"); err == nil {
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
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, _, err := mapdef.Resolve(m, p, "0,0"); err == nil {
		t.Fatal("want an error for a base tile name outside the standard vocabulary")
	}
}

// TestResolveRejectsAnOverrideWithNoPackGiven pins that Resolve fails loud
// rather than dereferencing a nil *Pack: Load accepts an empty Map.Pack
// alongside a non-empty Overrides (the two fields are never cross-checked at
// load time), so a caller can genuinely reach Resolve this way on a map Load
// already accepted.
func TestResolveRejectsAnOverrideWithNoPackGiven(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, _, err := mapdef.Resolve(m, nil, "1,1"); err == nil {
		t.Fatal("want an error resolving an override with no pack given, not a panic")
	}
}

// TestResolveRejectsAPackThatIsNotTheMapsOwn pins the identity check: the
// two resolution levels are the map's OWN pack, then standard (spec §4.2) —
// not any pack a caller happens to hand in. testdata/packs/other-keep
// defines a tile with the SAME name ("planks-split-3") as mossy-keep's, so
// this proves the refusal comes from the identity check and not merely from
// a lookup miss.
func TestResolveRejectsAPackThatIsNotTheMapsOwn(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json") // names pack "mossy-keep"
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p, err := mapdef.LoadPack("testdata/packs/other-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, _, err := mapdef.Resolve(m, p, "1,1"); err == nil {
		t.Fatal("want an error resolving against a pack that is not the map's own")
	}
}

// TestAnUndeclaredPackKindNeverProducesASpuriousMismatchWarning pins that an
// advisory kind left blank (packTileMap requires only Name) reads as "not
// declared", never as "declared and different" — the opposite reading would
// warn on every override drawn from a tile whose author hasn't classified it
// yet, which is exactly the noise this task's one warning channel must stay
// free of to remain trustworthy.
func TestAnUndeclaredPackKindNeverProducesASpuriousMismatchWarning(t *testing.T) {
	m, err := mapdef.Load("testdata/valid/cellar.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m.Overrides["1,1"] = "mystery-flagstone" // pack tile with no declared kind
	p, err := mapdef.LoadPack("testdata/packs/mossy-keep")
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	got, warnings, err := mapdef.Resolve(m, p, "1,1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Kind != "floor" {
		t.Fatalf("kind is %q, want floor (from the base tile)", got.Kind)
	}
	if len(warnings) != 0 {
		t.Fatalf("an undeclared pack kind produced a warning: %v", warnings)
	}
}
