package adventure_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// loadFixtureRuleset loads the small "proving-grounds-mini" ruleset every
// Load test validates statblocks against (testdata/ruleset): attributes
// vim/vigor, defense brace, resource focus — deliberately generic
// vocabulary (vim/brace style), matching internal/rules' own testdata
// fixtures.
func loadFixtureRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load("testdata/ruleset")
	if err != nil {
		t.Fatalf("load fixture ruleset: %v", err)
	}
	return rs
}

// "brace-yard" (testdata/valid) has FIVE scenes (cellar/gate/hall/loft/
// yard) and THREE actors (brace-guard/grit-scout/vim-fighter) — review-fix
// (p12-task-2 fix wave, item 1): a one-scene fixture could not distinguish
// Compile's correct file-name-order slice walk from a broken map-iteration
// one (the reviewer's own mutation probe needed 5 scenes to show "genuine
// order variance"), so both Load's and Compile's golden tests exercise
// that same 5-scene/3-actor shape.
func TestLoadValidFixture(t *testing.T) {
	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load("testdata/valid", rs)
	if err != nil {
		t.Fatal(err)
	}

	if adv.ID != "brace-yard" {
		t.Errorf("ID = %q, want brace-yard", adv.ID)
	}
	if adv.Name != "Brace Yard" {
		t.Errorf("Name = %q, want Brace Yard", adv.Name)
	}
	if adv.RulesetID != "proving-grounds-mini" {
		t.Errorf("RulesetID = %q, want proving-grounds-mini", adv.RulesetID)
	}
	if adv.OpeningNarration != "The yard is quiet before the bell rings." {
		t.Errorf("OpeningNarration = %q", adv.OpeningNarration)
	}
	if adv.GuidePath != "testdata/valid/guide.md" {
		t.Errorf("GuidePath = %q, want testdata/valid/guide.md", adv.GuidePath)
	}

	// scenes/*.json file-name order: cellar, gate, hall, loft, yard.
	wantScenes := []struct {
		id, name     string
		gridW, gridH int32
		placements   []adventure.Placement
	}{
		{"cellar", "The Cellar", 6, 6, []adventure.Placement{
			{TokenID: "tok-scout", ActorID: "grit-scout", X: 1, Y: 1},
		}},
		{"gate", "The Gate", 8, 8, []adventure.Placement{
			{TokenID: "tok-vim-2", ActorID: "vim-fighter", X: 0, Y: 0},
		}},
		{"hall", "The Hall", 7, 7, []adventure.Placement{
			{TokenID: "tok-brace-2", ActorID: "brace-guard", X: 2, Y: 2},
		}},
		{"loft", "The Loft", 5, 5, []adventure.Placement{
			{TokenID: "tok-scout-2", ActorID: "grit-scout", X: 3, Y: 3},
		}},
		{"yard", "The Yard", 10, 10, []adventure.Placement{
			{TokenID: "tok-vim", ActorID: "vim-fighter", X: 2, Y: 3},
			{TokenID: "tok-brace", ActorID: "brace-guard", X: 5, Y: 5},
		}},
	}
	if len(adv.Scenes) != len(wantScenes) {
		t.Fatalf("Scenes = %+v, want %d scenes in file-name order", adv.Scenes, len(wantScenes))
	}
	for i, want := range wantScenes {
		sc := adv.Scenes[i]
		if sc.ID != want.id || sc.Name != want.name || sc.GridW != want.gridW || sc.GridH != want.gridH {
			t.Errorf("Scenes[%d] = %+v, want id=%s name=%s grid=%dx%d", i, sc, want.id, want.name, want.gridW, want.gridH)
		}
		if len(sc.Placements) != len(want.placements) {
			t.Errorf("Scenes[%d].Placements = %+v, want %+v", i, sc.Placements, want.placements)
			continue
		}
		for j, wp := range want.placements {
			if sc.Placements[j] != wp {
				t.Errorf("Scenes[%d].Placements[%d] = %+v, want %+v", i, j, sc.Placements[j], wp)
			}
		}
	}

	// actors/*.json file-name order: brace-guard, grit-scout, vim-fighter.
	wantActorIDs := []string{"brace-guard", "grit-scout", "vim-fighter"}
	if len(adv.Actors) != len(wantActorIDs) {
		t.Fatalf("Actors = %+v, want %d actors in file-name order", adv.Actors, len(wantActorIDs))
	}
	for i, want := range wantActorIDs {
		if adv.Actors[i].ID != want {
			t.Errorf("Actors[%d].ID = %q, want %q (file-name order)", i, adv.Actors[i].ID, want)
		}
	}
	vf := adv.Actors[2]
	if vf.Name != "Vim Fighter" {
		t.Errorf("vim-fighter.Name = %q", vf.Name)
	}
	if vf.Attributes["vim"] != 14 || vf.Attributes["vigor"] != 12 || vf.Attributes["brace"] != 10 {
		t.Errorf("vim-fighter.Attributes = %+v", vf.Attributes)
	}
	if got := vf.Resources["focus"]; got != (adventure.ResourceVal{Current: 8, Max: 10}) {
		t.Errorf("vim-fighter.Resources[focus] = %+v", got)
	}

	if len(adv.Notes) != 1 {
		t.Fatalf("Notes = %+v, want 1 note", adv.Notes)
	}
	wantNote := adventure.AdventureNote{Key: "yard-rumor", Title: "Yard Rumor", Text: "Something stirs behind the well."}
	if adv.Notes[0] != wantNote {
		t.Errorf("Notes[0] = %+v, want %+v", adv.Notes[0], wantNote)
	}
}

// TestLoadInvalidFixtures walks the full validation catalogue
// (task-12-2-brief.md): one focused invalid fixture under testdata/invalid,
// one rule violated each. Every case asserts the error names both the
// offending file and the offending field (or, for the two directory-level
// rules — empty-adventure and unknown-field's strict-decode message — the
// distinguishing substrings decodeStrict/the empty-adventure check actually
// produce).
func TestLoadInvalidFixtures(t *testing.T) {
	cases := []struct {
		dir  string
		want []string // every substring must appear in err.Error()
	}{
		{"format-version-bad", []string{"adventure.json", `field "format_version"`, `"7"`}},
		{"ruleset-mismatch", []string{"adventure.json", `field "ruleset"`}},
		{"undeclared-attribute", []string{"vim-fighter.json", `field "attributes"`, `"grit"`}},
		{"undeclared-resource", []string{"vim-fighter.json", `field "resources"`, `"mana"`}},
		{"resource-current-over-max", []string{"vim-fighter.json", `field "resources.focus.current"`}},
		{"note-key-too-long", []string{"opening.json", `field "[0].key"`}},
		{"note-title-too-long", []string{"opening.json", `field "[0].title"`}},
		{"note-text-too-long", []string{"opening.json", `field "[0].text"`}},
		{"narration-empty", []string{"adventure.json", `field "opening_narration"`, "must not be empty"}},
		{"narration-too-long", []string{"adventure.json", `field "opening_narration"`, "at most 8192 bytes"}},
		{"duplicate-scene-id", []string{"yard.json", `field "id"`, `duplicate scene id "yard"`}},
		{"duplicate-actor-id", []string{"vim-fighter", `field "actor_id"`, `duplicate actor id "vim-fighter"`}},
		{"duplicate-token-id", []string{"yard.json", `field "placements[1].token_id"`, `duplicate token id "tok-vim"`}},
		{"duplicate-note-id", []string{"opening.json", `field "[1].key"`, `duplicate note key "yard-rumor"`}},
		{"placement-unknown-actor", []string{"yard.json", `field "placements[0].actor_id"`, `unknown actor "ghost-actor"`}},
		// The four "must not be empty" rules below had NO fixture until
		// 2026-08-04. gremlins reported their error returns NOT COVERED --
		// a category check-mutation.py does not fail on, so nothing in the
		// gate was ever going to point at them.
		{"placement-token-id-empty", []string{"yard.json", `field "placements[0].token_id"`, "must not be empty"}},
		{"placement-actor-id-empty", []string{"yard.json", `field "placements[0].actor_id"`, "must not be empty"}},
		{"note-key-empty", []string{"opening.json", `field "[0].key"`, "must not be empty"}},
		{"note-text-empty", []string{"opening.json", `field "[0].text"`, "must not be empty"}},
		{"empty-adventure", []string{"empty adventure"}},
		{"grid-width-zero", []string{"yard.json", `field "grid_width"`}},
		{"grid-height-zero", []string{"yard.json", `field "grid_height"`}},
		// "got 10" is load-bearing, not decoration: these two fixtures are the
		// ONLY pin for the INCLUSIVE upper bound (`p.X >= raw.GridWidth`), and
		// they only pin it while they sit exactly on it. y used to be -1, which
		// tested the lower bound instead and left `p.Y >= GridHeight` untested
		// entirely. Asserting the coordinate stops that drifting back silently.
		{"placement-x-out-of-bounds", []string{"yard.json", `field "placements[0].x"`, "got 10"}},
		{"placement-y-out-of-bounds", []string{"yard.json", `field "placements[0].y"`, "got 10"}},
		// Not part of the enumerated catalogue — pins the "Load: strict
		// decode" clause the brief states separately from the numbered
		// rules, mirroring internal/rules/testdata/invalid/unknown-field.
		{"unknown-field", []string{"adventure.json", "bogus_field"}},
	}

	rs := loadFixtureRuleset(t)
	for _, c := range cases {
		t.Run(c.dir, func(t *testing.T) {
			_, err := adventure.Load("testdata/invalid/"+c.dir, rs)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
}

// TestLoadInvalidFixturesCatalogueIsComplete guards against a fixture
// directory existing on disk with no corresponding case above (or vice
// versa) — the two must name exactly the same set of rules.
func TestLoadInvalidFixturesCatalogueIsComplete(t *testing.T) {
	entries, err := os.ReadDir("testdata/invalid")
	if err != nil {
		t.Fatal(err)
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			onDisk[e.Name()] = true
		}
	}
	// Mirrors the case table's dir column, deliberately hand-kept in sync:
	// a mismatch here means someone added/removed a fixture directory
	// without updating TestLoadInvalidFixtures (or vice versa).
	want := []string{
		"format-version-bad", "ruleset-mismatch", "undeclared-attribute", "undeclared-resource",
		"resource-current-over-max", "note-key-too-long", "note-title-too-long",
		"note-text-too-long", "narration-empty", "narration-too-long",
		"duplicate-scene-id", "duplicate-actor-id", "duplicate-token-id",
		"duplicate-note-id", "placement-unknown-actor", "empty-adventure",
		"grid-width-zero", "grid-height-zero", "placement-x-out-of-bounds",
		"placement-y-out-of-bounds", "unknown-field",
		"placement-token-id-empty", "placement-actor-id-empty",
		"note-key-empty", "note-text-empty",
	}
	if len(want) != len(onDisk) {
		t.Errorf("testdata/invalid has %d dirs, case table names %d", len(onDisk), len(want))
	}
	for _, w := range want {
		if !onDisk[w] {
			t.Errorf("case table names %q, no such testdata/invalid directory", w)
		}
	}
	for d := range onDisk {
		found := false
		for _, w := range want {
			if w == d {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("testdata/invalid/%s exists but is not in the case table", d)
		}
	}
}

// TestLoadRejectsMissingDirectory pins the base directory-existence check
// (not part of the JSON validation catalogue, but Load's first guard).
func TestLoadRejectsMissingDirectory(t *testing.T) {
	rs := loadFixtureRuleset(t)
	if _, err := adventure.Load("testdata/does-not-exist", rs); err == nil {
		t.Fatal("want error for a missing directory")
	}
}

// TestLoadDoesNotRequireGuideFileToExist pins a deliberate design decision
// (format.go's Adventure.GuidePath doc comment): guide.md is served
// verbatim to the DM via MCP and never read by this package, so Load must
// not fail just because dir/guide.md happens to be absent — GuidePath is
// set to the path unconditionally, and whichever future caller actually
// opens it (a later task) owns that error.
func TestLoadDoesNotRequireGuideFileToExist(t *testing.T) {
	dir := copyFixtureDirExcluding(t, "testdata/valid", "guide.md")

	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatalf("Load must succeed without a guide.md file, got: %v", err)
	}
	want := filepath.Join(dir, "guide.md")
	if adv.GuidePath != want {
		t.Errorf("GuidePath = %q, want %q (set unconditionally, not checked for existence)", adv.GuidePath, want)
	}
	if _, err := os.Stat(adv.GuidePath); !os.IsNotExist(err) {
		t.Fatalf("test setup bug: guide.md should be absent, stat err = %v", err)
	}
}

// TestLoadRejectsMissingFormatVersion pins the absent-key half of
// format_version validation (fix-wave F1, load.go's supportedFormatVersions
// check): a manifest that omits the "format_version" key entirely decodes
// FormatVersion to Go's zero value (""), which supportedFormatVersions
// rejects through the exact same code path as an explicit wrong value
// (testdata/invalid/format-version-bad's "7") — both fail
// supportedFormatVersions[raw.FormatVersion]. Deliberately NOT a second
// catalogue fixture directory (that would duplicate the same rejection
// under a different name in TestLoadInvalidFixtures); a focused, hand-built
// temp fixture instead, following TestLoadDoesNotRequireGuideFileToExist's
// copyFixtureDirExcluding pattern — copy testdata/valid except its
// adventure.json, then write a fresh one with no format_version key at all.
func TestLoadRejectsMissingFormatVersion(t *testing.T) {
	dir := copyFixtureDirExcluding(t, "testdata/valid", "adventure.json")
	manifest := `{
  "id": "brace-yard",
  "name": "Brace Yard",
  "ruleset": "proving-grounds-mini",
  "opening_narration": "The yard is quiet before the bell rings."
}`
	if err := os.WriteFile(filepath.Join(dir, "adventure.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write adventure.json without format_version: %v", err)
	}

	rs := loadFixtureRuleset(t)
	_, err := adventure.Load(dir, rs)
	if err == nil {
		t.Fatal("want error for a manifest missing the format_version key entirely")
	}
	for _, want := range []string{"adventure.json", `field "format_version"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// copyFixtureDirExcluding copies srcDir into a fresh t.TempDir(), skipping
// any file whose base name is in skip.
func copyFixtureDirExcluding(t *testing.T, srcDir string, skip ...string) string {
	t.Helper()
	skipSet := map[string]bool{}
	for _, s := range skip {
		skipSet[s] = true
	}
	dst := t.TempDir()
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if skipSet[d.Name()] {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatalf("copy fixture dir: %v", err)
	}
	return dst
}

// TestLoadAcceptsValuesExactlyOnEveryLimit pins that every limit load.go
// checks is INCLUSIVE. testdata/at-every-boundary sits exactly on all of them
// at once — 8192-byte narration and note text, a 128-byte note key, a
// 256-byte title, a 1x1 grid, a placement at (0,0), and a resource with max 0
// and a non-zero current — and every one of those is legal.
//
// One fixture rather than seven because the limits share a failure mode:
// loosen any single comparison by one character (`>` to `>=`, `<` to `<=`) and
// this adventure stops loading. A fixture one byte UNDER each limit would load
// either way and pin nothing, which is how all seven boundaries came to be
// unpinned in the first place.
//
// The placement at (0,0) is NOT one of the seven -- `p.X < 0` and `p.Y < 0`
// were already killed by testdata/valid/scenes/gate.json, which has had a
// (0,0) placement all along. It is kept as a deliberate redundant pin, so that
// the lower bound does not depend on a single fixture the way the upper bound
// turned out to (see placement-y-out-of-bounds above).
//
// The resource case is the odd one and worth naming: max 0 means UNLIMITED, so
// `rv.Max > 0 && rv.Current > rv.Max` must skip the comparison entirely. Under
// `rv.Max >= 0` it does not skip, and a current of 7 against a max of 0 reads
// as an overflow that was never declared.
func TestLoadAcceptsValuesExactlyOnEveryLimit(t *testing.T) {
	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load("testdata/at-every-boundary", rs)
	if err != nil {
		t.Fatalf("Load(at-every-boundary): every value here is exactly on a limit and legal: %v", err)
	}

	// Assert the boundary values survived the load rather than merely that it
	// returned: a Load that silently dropped them would still be error-free.
	if got := len(adv.OpeningNarration); got != 8192 {
		t.Errorf("opening narration = %d bytes, want 8192 (exactly maxTextBytes)", got)
	}
	if len(adv.Scenes) != 1 || adv.Scenes[0].GridW != 1 || adv.Scenes[0].GridH != 1 {
		t.Fatalf("want one 1x1 scene, got %+v", adv.Scenes)
	}
	if len(adv.Scenes[0].Placements) != 1 || adv.Scenes[0].Placements[0].X != 0 || adv.Scenes[0].Placements[0].Y != 0 {
		t.Errorf("want one placement at (0,0), got %+v", adv.Scenes[0].Placements)
	}
	if len(adv.Notes) != 1 {
		t.Fatalf("want one note, got %d", len(adv.Notes))
	}
	if got := len(adv.Notes[0].Key); got != 128 {
		t.Errorf("note key = %d bytes, want 128 (exactly maxNoteKeyBytes)", got)
	}
	if got := len(adv.Notes[0].Title); got != 256 {
		t.Errorf("note title = %d bytes, want 256 (exactly maxNoteTitleBytes)", got)
	}
	if got := len(adv.Notes[0].Text); got != 8192 {
		t.Errorf("note text = %d bytes, want 8192 (exactly maxTextBytes)", got)
	}
	if len(adv.Actors) != 1 {
		t.Fatalf("want one actor, got %d", len(adv.Actors))
	}
	if rv, ok := adv.Actors[0].Resources["focus"]; !ok || rv.Max != 0 || rv.Current != 7 {
		t.Errorf("want focus current=7 max=0 (0 meaning unlimited), got %+v ok=%v", rv, ok)
	}
}
