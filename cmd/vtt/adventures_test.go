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
// mechanism scenarios/adventure-night.json's own adventures dir
// (scenarios/testdata/dnd45e-minimal-adventures/) relies on: a per-table,
// single-ruleset adventures directory built from symlinks into the real,
// single committed adventures/ tree — no content duplication, no risk of a
// forked copy drifting from the source of truth.
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

// TestLoadAdventuresDirRulesetMismatchFailsLoud covers the boot-time
// binding: a subdirectory declaring a DIFFERENT ruleset than the one
// passed in fails the whole call, naming the file+field (adventure.Load's
// own error, propagated verbatim — see loadAdventuresDir's doc comment).
func TestLoadAdventuresDirRulesetMismatchFailsLoud(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir: %v", err)
	}
	rs, err := rules.Load(rulesetDir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}

	dir := t.TempDir()
	// cellar-rats declares ruleset "tavern-brawl", not "dnd45e-minimal".
	if err := os.Symlink(filepath.Join(root, "adventures", "cellar-rats"), filepath.Join(dir, "cellar-rats")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err = loadAdventuresDir(dir, rs)
	if err == nil {
		t.Fatal("want an error loading an adventure that declares a different ruleset")
	}
	if !strings.Contains(err.Error(), "ruleset") {
		t.Fatalf("error = %q, want it to name the ruleset mismatch", err.Error())
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
