package main

// harness_boot_test.go covers resolveRulesetDir/findRepoRoot (harness_boot.go,
// ruleset-interpreter Task 6): the "relative to repo root" resolution rule
// bootSelfContained uses to turn a scenario's bare Ruleset id into a
// directory rules.Load can open.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// TestResolveRulesetDirFindsCommittedTavernBrawl proves the resolution rule
// against the REAL committed rulesets/tavern-brawl directory: since `go
// test` sets this package's cwd to cmd/vtt itself (the Go toolchain's own
// convention — see resolveRulesetDir's doc comment), this test running at
// all IS the proof the walk-up-to-go.mod logic correctly reaches the repo
// root from a subdirectory two levels below it.
func TestResolveRulesetDirFindsCommittedTavernBrawl(t *testing.T) {
	dir, err := resolveRulesetDir("tavern-brawl")
	if err != nil {
		t.Fatalf("resolveRulesetDir(tavern-brawl): %v", err)
	}
	if !strings.HasSuffix(dir, "rulesets/tavern-brawl") && !strings.HasSuffix(dir, `rulesets\tavern-brawl`) {
		t.Fatalf("resolveRulesetDir(tavern-brawl) = %q, want it to end in rulesets/tavern-brawl", dir)
	}
}

// TestResolveRulesetDirUnknownIDErrorsCleanly proves an id with no matching
// rulesets/<id> directory is a named, clean error — not a panic or a
// silent empty string a later rules.Load call would fail on with a less
// specific message.
func TestResolveRulesetDirUnknownIDErrorsCleanly(t *testing.T) {
	_, err := resolveRulesetDir("no-such-ruleset-id")
	if err == nil {
		t.Fatal("want error for an unknown ruleset id")
	}
	if !strings.Contains(err.Error(), "no-such-ruleset-id") {
		t.Fatalf("error = %q, want it to name the unresolved id", err.Error())
	}
}

// --- resolveMapsDir / installMaps (2026-09-01-create-scene-leaves Task 7) ---

// TestResolveMapsDirFindsTheCommittedCorpusMaps is resolveRulesetDir's test
// above, aimed at the other resolver: the walk reaching scenarios/maps from
// cmd/vtt is the whole claim, and this test running from that cwd is the
// proof.
func TestResolveMapsDirFindsTheCommittedCorpusMaps(t *testing.T) {
	dir, err := resolveMapsDir("scenarios/maps")
	if err != nil {
		t.Fatalf("resolveMapsDir(scenarios/maps): %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(dir), "scenarios/maps") {
		t.Fatalf("resolveMapsDir = %q, want it to end in scenarios/maps", dir)
	}
}

// TestResolveMapsDirUnknownDirErrorsCleanly: a maps directory that is not
// there is a named error rather than an empty string that would surface much
// later as "maps dir contains no maps" against a path nobody asked for.
func TestResolveMapsDirUnknownDirErrorsCleanly(t *testing.T) {
	_, err := resolveMapsDir("scenarios/no-such-maps-dir")
	if err == nil {
		t.Fatal("want error for a maps directory that does not exist")
	}
	if !strings.Contains(err.Error(), "no-such-maps-dir") {
		t.Fatalf("error = %q, want it to name the unresolved directory", err.Error())
	}
}

// TestInstallMapsPutsEveryMapWhereTheCampaignLooks is the install half of
// "install, then load" (design spec §4): after it runs, LoadMapsDir — the
// very function `vtt serve` boots through — must load the campaign's maps
// from the campaign directory, by id.
//
// It asserts through LoadMapsDir rather than by listing files, deliberately.
// A directory listing would pass if installMaps wrote the files somewhere the
// server does not read, or under names that are not ids; going through the
// loader ties the assertion to the thing that has to be true.
func TestInstallMapsPutsEveryMapWhereTheCampaignLooks(t *testing.T) {
	srcDir := t.TempDir()
	writeMapFile(t, srcDir, "hall", 2, 1)
	writeMapFile(t, srcDir, "cellar", 1, 2)
	// Not a map, and not installed: the walk takes *.json only.
	if err := os.WriteFile(filepath.Join(srcDir, "notes.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	campaign := t.TempDir()
	if err := installMaps(srcDir, campaign); err != nil {
		t.Fatalf("installMaps: %v", err)
	}
	maps, err := LoadMapsDir(campaign)
	if err != nil {
		t.Fatalf("LoadMapsDir after install: %v", err)
	}
	if len(maps) != 2 || maps["hall"] == nil || maps["cellar"] == nil {
		t.Fatalf("loaded %d map(s) %v, want exactly hall and cellar", len(maps), sortedMapIDs(maps))
	}
	if got := maps["hall"].GridW; got != 2 {
		t.Fatalf("hall grid width = %d, want 2 — the installed file is not the one authored", got)
	}
	if _, err := os.Stat(filepath.Join(campaign, "maps", "notes.txt")); !os.IsNotExist(err) {
		t.Fatalf("notes.txt was installed (stat err = %v), want *.json only", err)
	}
}

// TestInstallMapsRefusesASourceThatIsNotThere: nothing to install is an error
// here rather than a campaign that boots with no maps and fails at the first
// load_map — the same fail-loud-at-boot posture composeServer gives a broken
// map (maps.go's own doc comment).
func TestInstallMapsRefusesASourceThatIsNotThere(t *testing.T) {
	err := installMaps(filepath.Join(t.TempDir(), "nope"), t.TempDir())
	if err == nil {
		t.Fatal("want error installing from a directory that does not exist")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %q, want it to name the directory it could not read", err.Error())
	}
}

// TestInstallMapsRefusesACampaignItCannotWriteInto covers the other end: a
// campaign path that is a FILE cannot gain a maps/ directory, and the error
// says which path failed rather than leaving a half-done install behind.
func TestInstallMapsRefusesACampaignItCannotWriteInto(t *testing.T) {
	srcDir := t.TempDir()
	writeMapFile(t, srcDir, "hall", 1, 1)
	notADir := filepath.Join(t.TempDir(), "campaign")
	if err := os.WriteFile(notADir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := installMaps(srcDir, notADir)
	if err == nil {
		t.Fatal("want error installing into a campaign path that is a file")
	}
	if !strings.Contains(err.Error(), "maps") {
		t.Fatalf("error = %q, want it to name the maps directory it could not create", err.Error())
	}
}

// TestInstallMapsReportsAMapItCannotWrite: an entry that cannot be written is
// reported by NAME, not swallowed — a half-installed campaign whose missing
// map only shows up as "unknown map" mid-session is exactly the failure this
// whole sub-project moves earlier.
func TestInstallMapsReportsAMapItCannotWrite(t *testing.T) {
	srcDir := t.TempDir()
	writeMapFile(t, srcDir, "hall", 1, 1)
	campaign := t.TempDir()
	// A DIRECTORY standing where the file must be written.
	if err := os.MkdirAll(filepath.Join(campaign, "maps", "hall.json"), 0o750); err != nil {
		t.Fatal(err)
	}
	err := installMaps(srcDir, campaign)
	if err == nil {
		t.Fatal("want error when a map cannot be written into the campaign")
	}
	if !strings.Contains(err.Error(), "hall.json") {
		t.Fatalf("error = %q, want it to name the map that did not install", err.Error())
	}
}

// TestInstallMapsReportsAMapItCannotRead is the mirror: an unreadable source
// entry is named too, rather than producing a campaign silently short one map.
func TestInstallMapsReportsAMapItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a 0o000 file is still readable, so this proves nothing")
	}
	srcDir := t.TempDir()
	writeMapFile(t, srcDir, "hall", 1, 1)
	if err := os.Chmod(filepath.Join(srcDir, "hall.json"), 0o000); err != nil {
		t.Fatal(err)
	}
	err := installMaps(srcDir, t.TempDir())
	if err == nil {
		t.Fatal("want error when a source map cannot be read")
	}
	if !strings.Contains(err.Error(), "hall.json") {
		t.Fatalf("error = %q, want it to name the map it could not read", err.Error())
	}
}

// writeMapFile writes one minimal, VALID map — valid so that a test asserting
// some other refusal is not accidentally passing on a malformed fixture.
func writeMapFile(t *testing.T, dir, id string, w, h int) {
	t.Helper()
	tiles := map[string]string{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = "stone"
		}
	}
	raw, err := json.Marshal(map[string]any{
		"format_version": 1, "id": id, "name": id,
		"grid_width": w, "grid_height": h, "tiles": tiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// sortedMapIDs names what actually loaded, for a failure message.
func sortedMapIDs(maps map[string]*mapdef.Map) []string {
	out := make([]string, 0, len(maps))
	for id := range maps {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
