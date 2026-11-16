package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// spendingRuleset is the fixture ruleset holding swing, which spends resource.
func spendingRuleset(t *testing.T, resource string) *rules.Ruleset {
	t.Helper()
	rs := loadFixtureRuleset(t)
	rs.Compiled["swing"] = &rules.CompiledPower{Usage: rules.Usage{Limited: &rules.LimitedUsage{Resource: resource, Cost: 1}}}
	return rs
}

// VTT-776
func TestAnActorFileNamingALimitedAbilityWithoutItsResourceIsRefused(t *testing.T) {
	_, err := adventure.Load(writeBraceGuardSheet(t, braceGuardFits, `, "ability_ids": ["swing"]`), spendingRuleset(t, "zeal"))
	if err == nil {
		t.Fatal("an actor naming swing without zeal was loaded")
	}
	for _, want := range []string{"brace-guard.json", `field "ability_ids"`,
		`names ability "swing", which spends resource "zeal" the sheet does not hold`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not hold %q", err, want)
		}
	}
}

// VTT-776
func TestAnActorFileHoldingAResourceNoAbilityItNamesSpendsIsRefused(t *testing.T) {
	_, err := adventure.Load(writeBraceGuardSheet(t, braceGuardFits, ``), spendingRuleset(t, "focus"))
	if err == nil {
		t.Fatal("an actor holding focus, which swing spends, while naming no ability was loaded")
	}
	for _, want := range []string{"brace-guard.json", `field "resources"`,
		`holds resource "focus", which no ability the sheet names spends`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not hold %q", err, want)
		}
	}
}

// VTT-776: every actor of the fixture holds focus, so each names swing.
func TestActorFilesNamingALimitedAbilityBesideItsResourceLoad(t *testing.T) {
	dir := writeBraceGuardSheet(t, braceGuardFits, `, "ability_ids": ["swing"]`)
	for _, name := range []string{"grit-scout.json", "vim-fighter.json"} {
		path := filepath.Join(dir, "actors", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var actor map[string]any
		if err := json.Unmarshal(raw, &actor); err != nil {
			t.Fatal(err)
		}
		actor["ability_ids"] = []string{"swing"}
		out, err := json.Marshal(actor)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := adventure.Load(dir, spendingRuleset(t, "focus")); err != nil {
		t.Fatalf("actors naming swing beside focus were refused: %v", err)
	}
}
