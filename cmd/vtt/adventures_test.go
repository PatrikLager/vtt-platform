package main

// adventures_test.go covers loadAdventuresDir/loadAdventureGuides
// (adventures.go, adventure-format Task 4): the shared boot-time walker
// `vtt serve --adventures-dir` (composeServer) and `vtt mcp
// --adventures-dir` (mcp.go) both use, built against the REAL committed
// adventures/goblin-ambush directory and rulesets/dnd45e-minimal ruleset
// (the same "real fixtures over synthetic ones" precedent
// mcp_ruleset_e2e_test.go and library_test.go both already follow).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func loadDnd45eMinimalForCmd(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load(dnd45e-minimal): %v", err)
	}
	return rs
}

func goblinAmbushAdventureDir(t *testing.T) string {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}
	return filepath.Join(root, "adventures", "goblin-ambush")
}

// TestLoadAdventuresDirFollowsSymlinkedAdventureDirectories proves
// loadAdventuresDir resolves each entry via os.Stat (which follows
// symlinks), not DirEntry.IsDir() alone (which reports a symlink's OWN
// type, ModeSymlink, never ModeDir, even when it points at a real
// directory — confirmed against Go's os.ReadDir semantics). This is the
// mechanism an operator uses to compose a per-table adventures directory
// from symlinks into a shared tree — no content duplication, no risk of a
// forked copy drifting from the source of truth.
//
// This repo no longer ships such a fixture (loadAdventuresDir selects by
// ruleset instead, 2026-08-06), so the behaviour is pinned here rather than
// exercised incidentally by scenarios/adventure-night.json.
func TestLoadAdventuresDirFollowsSymlinkedAdventureDirectories(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	realDir := goblinAmbushAdventureDir(t)

	dir := t.TempDir()
	link := filepath.Join(dir, "goblin-ambush")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	advs, err := loadAdventuresDir(dir, rs)
	if err != nil {
		t.Fatalf("loadAdventuresDir: %v", err)
	}
	if len(advs) != 1 {
		t.Fatalf("loadAdventuresDir: got %d adventures, want 1: %v", len(advs), advs)
	}
	adv, ok := advs["goblin-ambush"]
	if !ok {
		t.Fatalf("loadAdventuresDir: want key %q, got %v", "goblin-ambush", advs)
	}
	if adv.Name != "Goblin Ambush" {
		t.Fatalf("loaded adventure Name = %q, want %q", adv.Name, "Goblin Ambush")
	}
}

// TestLoadAdventuresDirEmptyDirIsBootError covers fix-wave F4: an
// EXISTING --adventures-dir with zero subdirectories must fail loud,
// exactly like a nonexistent directory already does (os.ReadDir error
// above) — before this fix, an existing-but-empty dir returned an empty
// map with a nil error, so a typo'd or never-synced --adventures-dir
// booted "successfully" with no adventures at all, deferring the failure
// to the table (spec §7: "fail loud at startup, not at the table").
func TestLoadAdventuresDirEmptyDirIsBootError(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	dir := t.TempDir() // exists, zero entries

	_, err := loadAdventuresDir(dir, rs)
	if err == nil {
		t.Fatal("want an error for an existing-but-empty --adventures-dir")
	}
	if !strings.Contains(err.Error(), "no adventures") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "no adventures")
	}
}

// TestLoadAdventuresDirOnlyNonDirectoryEntriesIsBootError covers the other
// path to zero loaded adventures (fix-wave F4): a dir that exists and has
// entries, but every entry is a non-directory stray file (all silently
// skipped by the loop) — same zero-loaded outcome as a literally empty
// dir, and must fail the same way.
func TestLoadAdventuresDirOnlyNonDirectoryEntriesIsBootError(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not an adventure"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := loadAdventuresDir(dir, rs)
	if err == nil {
		t.Fatal("want an error when every entry is a non-directory (zero adventures loaded)")
	}
	if !strings.Contains(err.Error(), "no adventures") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "no adventures")
	}
}

// TestLoadAdventuresDirSkipsNonDirectoryEntries covers a stray file sitting
// alongside real adventure subdirectories — silently skipped, not an
// error (mirrors adventure.Load's own jsonFilesIn precedent of ignoring
// non-matching entries).
func TestLoadAdventuresDirSkipsNonDirectoryEntries(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	realDir := goblinAmbushAdventureDir(t)

	dir := t.TempDir()
	if err := os.Symlink(realDir, filepath.Join(dir, "goblin-ambush")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not an adventure"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	advs, err := loadAdventuresDir(dir, rs)
	if err != nil {
		t.Fatalf("loadAdventuresDir: %v", err)
	}
	if len(advs) != 1 {
		t.Fatalf("loadAdventuresDir: got %d adventures, want 1 (stray file skipped): %v", len(advs), advs)
	}
}

// TestLoadAdventuresDirDuplicateManifestIDIsBootError covers two
// subdirectories that declare the SAME manifest id (as opposed to two
// different directory names) — a silent map-key collision would let the
// second overwrite the first; this must be a named boot error instead.
func TestLoadAdventuresDirDuplicateManifestIDIsBootError(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	realDir := goblinAmbushAdventureDir(t)

	dir := t.TempDir()
	if err := os.Symlink(realDir, filepath.Join(dir, "goblin-ambush-a")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "goblin-ambush-b")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err := loadAdventuresDir(dir, rs)
	if err == nil {
		t.Fatal("want an error when two subdirectories declare the same adventure id")
	}
	if !strings.Contains(err.Error(), "goblin-ambush") {
		t.Fatalf("error = %q, want it to name the colliding adventure id", err.Error())
	}
}

// TestLoadAdventureGuidesReadsRealGuideContent proves loadAdventureGuides
// reads each adventure's guide.md content, keyed by adventure id.
func TestLoadAdventureGuidesReadsRealGuideContent(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	realDir := goblinAmbushAdventureDir(t)
	want, err := os.ReadFile(filepath.Join(realDir, "guide.md"))
	if err != nil {
		t.Fatalf("read committed guide.md: %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink(realDir, filepath.Join(dir, "goblin-ambush")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	advs, err := loadAdventuresDir(dir, rs)
	if err != nil {
		t.Fatalf("loadAdventuresDir: %v", err)
	}

	guides, err := loadAdventureGuides(advs)
	if err != nil {
		t.Fatalf("loadAdventureGuides: %v", err)
	}
	if got := guides["goblin-ambush"]; got != string(want) {
		t.Fatalf("guide content mismatch: got %d bytes, want %d bytes (committed guide.md)", len(got), len(want))
	}
}

// TestLoadAdventureGuidesRejectsEmptyGuide covers the non-empty-guide.md
// binding (mirrors internal/adventure/conformance's own checkGuide rule,
// reimplemented here — see loadAdventureGuides' doc comment for why).
func TestLoadAdventureGuidesRejectsEmptyGuide(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	realDir := goblinAmbushAdventureDir(t)

	// Build a fixture adventure directory with an empty guide.md by copying
	// every OTHER real file/dir via symlink and writing a fresh, empty
	// guide.md over it — the adventure content itself is irrelevant to this
	// test, only the empty-guide rejection is under test.
	fixture := t.TempDir()
	adv := filepath.Join(fixture, "goblin-ambush")
	if err := os.Mkdir(adv, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	for _, name := range []string{"adventure.json", "scenes", "actors", "notes"} {
		if err := os.Symlink(filepath.Join(realDir, name), filepath.Join(adv, name)); err != nil {
			t.Fatalf("Symlink %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(adv, "guide.md"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile empty guide.md: %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink(adv, filepath.Join(dir, "goblin-ambush")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	advs, err := loadAdventuresDir(dir, rs)
	if err != nil {
		t.Fatalf("loadAdventuresDir: %v", err)
	}

	_, err = loadAdventureGuides(advs)
	if err == nil {
		t.Fatal("want an error for an empty guide.md")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %q, want it to name the empty guide", err.Error())
	}
}

// --- selecting by ruleset ---------------------------------------------------
//
// AMENDED BINDING (Patrik, 2026-08-06). The P12 plan bound this the other way:
// "adventures with a DIFFERENT ruleset id than served: boot error too — the
// dir is for THIS table". In practice that made the repo's own ./adventures
// unbootable — cellar-rats declares tavern-brawl, goblin-ambush declares
// dnd45e-minimal — so a symlinked single-ruleset fixture existed purely to
// work around it, and THAT symlink is dropped by gremlins' workdir copy, which
// is what made cmd/vtt unmeasurable (tools/mutation-scope.md).
//
// The dir is now a library: serve what is for this table, skip what is not.
// The property fix-wave F4 protects is kept — zero adventures loaded is still
// a boot error — so the silent case is narrowed to "some matched", never "none
// did", and the error names the served ruleset so the operator can see why.

func TestLoadAdventuresDirServesOnlyWhatIsForThisTable(t *testing.T) {
	rs := loadDnd45eMinimalForCmd(t)
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	// The REAL committed library, which is genuinely mixed: goblin-ambush
	// declares dnd45e-minimal, cellar-rats declares tavern-brawl. This is the
	// exact directory the "serve --adventures-dir ./adventures cannot boot"
	// carry-forward was about.
	advs, err := loadAdventuresDir(filepath.Join(root, "adventures"), rs)
	if err != nil {
		t.Fatalf("loadAdventuresDir: %v — a mixed library must serve its matching adventures, not refuse to boot", err)
	}
	if len(advs) != 1 {
		t.Fatalf("got %d adventures, want only the dnd45e-minimal one: %v", len(advs), advs)
	}
	if _, ok := advs["goblin-ambush"]; !ok {
		t.Fatalf("want goblin-ambush served, got %v", advs)
	}
	if _, ok := advs["cellar-rats"]; ok {
		t.Fatal("cellar-rats declares tavern-brawl and must NOT be served to a dnd45e-minimal table")
	}
}

func TestLoadAdventuresDirFailsLoudWhenNothingIsForThisTable(t *testing.T) {
	// The F4 property, preserved. Skipping is a library affordance, not a
	// licence to boot a table with nothing on it — and the message must say
	// WHICH ruleset found no adventures, or the operator sees only "no
	// adventures" for a directory that visibly contains some.
	rs := loadDnd45eMinimalForCmd(t)
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "adventures", "cellar-rats"), filepath.Join(dir, "cellar-rats")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err = loadAdventuresDir(dir, rs)
	if err == nil {
		t.Fatal("want a boot error: nothing in the dir is for the served ruleset")
	}
	if !strings.Contains(err.Error(), "dnd45e-minimal") {
		t.Fatalf("error = %q, want it to name the served ruleset that matched nothing", err)
	}
	// Naming the ruleset is not enough on its own: the PROPAGATED mismatch
	// error also contains "dnd45e-minimal" ("...but the served ruleset is
	// ..."), so without this the test passes just as happily when the skip
	// never happened and the mismatch simply bubbled up. Pin the branch, not
	// a substring both branches share.
	if !strings.Contains(err.Error(), "skipped:") {
		t.Fatalf("error = %q — that is the mismatch propagating, not the empty-library branch; "+
			"the F4 property this test claims to pin was never exercised", err)
	}
}

func TestLoadAdventuresDirStillFailsLoudOnAMalformedAdventure(t *testing.T) {
	// The distinction the sentinel exists for, and the one a weaker test
	// misses: the library ALSO contains a perfectly good adventure, so a
	// "skip everything that fails to load" bug would boot happily serving it
	// while the broken one vanished. Asserting merely "an error" cannot see
	// that — an empty library errors too, for an entirely different reason.
	rs := loadDnd45eMinimalForCmd(t)
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "adventures", "goblin-ambush"),
		filepath.Join(dir, "goblin-ambush")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	broken := filepath.Join(dir, "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(broken, "adventure.json"),
		[]byte(`{"ruleset":"dnd45e-minimal"`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = loadAdventuresDir(dir, rs)
	if err == nil {
		t.Fatal("want a boot error: a malformed adventure must not be silently skipped " +
			"just because another adventure loaded fine")
	}
	if strings.Contains(err.Error(), "no adventures") {
		t.Fatalf("error = %q — that is the EMPTY-library message; the malformed adventure "+
			"was skipped rather than reported", err)
	}
}
