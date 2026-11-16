package rules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// qaSFTavern is the repository's tavern-brawl ruleset: chair-swing spends
// stamina, fists, sober-up and splash-of-water are at will, and no ability
// spends drink.
func qaSFTavern(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(filepath.Join("..", "..", "rulesets", "tavern-brawl"))
	if err != nil {
		t.Fatalf("load tavern-brawl: %v", err)
	}
	return rs
}

// qaSFCopyDir copies the directory tree src into dst.
func qaSFCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o600)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// qaSFVariant is tavern-brawl with a resource "anger" declared after stamina
// (so declaration order and byte order disagree), "rage-punch" spending
// anger, and "bottle-smash" spending stamina beside chair-swing (a shared
// resource). drink is still spent by no ability.
func qaSFVariant(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	qaSFCopyDir(t, filepath.Join("..", "..", "rulesets", "tavern-brawl"), dir)

	manifest := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	props, ok := doc["properties"].([]any)
	if !ok {
		t.Fatalf("ruleset.json properties: %T", doc["properties"])
	}
	doc["properties"] = append(props, map[string]any{
		"key":         "anger",
		"type":        "resource",
		"name":        "Anger",
		"description": "QA fixture resource spent by rage-punch.",
		"thresholds":  []any{},
	})
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, out, 0o600); err != nil {
		t.Fatal(err)
	}

	swing, err := os.ReadFile(filepath.Join(dir, "abilities", "chair-swing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []struct{ id, name, resource string }{
		{"rage-punch", "Rage Punch", "anger"},
		{"bottle-smash", "Bottle Smash", "stamina"},
	} {
		var ab map[string]any
		if err := json.Unmarshal(swing, &ab); err != nil {
			t.Fatal(err)
		}
		ab["id"] = a.id
		ab["name"] = a.name
		ab["usage"] = map[string]any{"limited": map[string]any{"resource": a.resource, "cost": 1}}
		b, err := json.MarshalIndent(ab, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "abilities", a.id+".json"), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load variant ruleset: %v", err)
	}
	return rs
}

// TestQASFSpendsAnswersTheResourceOfAHeldLimitedAbility pins Ruleset.Spends:
// the resource a held limited ability spends, or none (VTT-775's reading of
// "a limited ability whose resource").
func TestQASFSpendsAnswersTheResourceOfAHeldLimitedAbility(t *testing.T) {
	rs := qaSFVariant(t)
	cases := []struct {
		id       string
		resource string
		ok       bool
	}{
		{"chair-swing", "stamina", true},
		{"bottle-smash", "stamina", true},
		{"rage-punch", "anger", true},
		{"fists", "", false},
		{"sober-up", "", false},
		{"splash-of-water", "", false},
		{"no-such-ability", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := rs.Spends(c.id)
		if got != c.resource || ok != c.ok {
			t.Errorf("Spends(%q) = (%q, %v), want (%q, %v)", c.id, got, ok, c.resource, c.ok)
		}
	}
}

type qaSFCase struct {
	name      string
	abilities []string
	resources []string
	// wantField empty means the sheet fits (nil fault).
	wantField string
	wantKey   string
	// wantOther is the other half the reason names beside the key: the
	// resource, for an ability_ids fault. Which ability, if any, a resources
	// fault names is not promised, so it is left unasserted.
	wantOther string
}

func qaSFRun(t *testing.T, rs *rules.Ruleset, cases []qaSFCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := rs.SheetSpendFault(c.abilities, c.resources)
			if c.wantField == "" {
				if f != nil {
					t.Fatalf("SheetSpendFault(%q, %q) = %+v, want nil", c.abilities, c.resources, *f)
				}
				return
			}
			if f == nil {
				t.Fatalf("SheetSpendFault(%q, %q) = nil, want field %q key %q", c.abilities, c.resources, c.wantField, c.wantKey)
			}
			if f.Field != c.wantField || f.Key != c.wantKey {
				t.Fatalf("SheetSpendFault(%q, %q) = field %q key %q, want field %q key %q (reason %q)",
					c.abilities, c.resources, f.Field, f.Key, c.wantField, c.wantKey, f.Reason)
			}
			t.Logf("reason: %s", f.Reason)
			if !strings.Contains(f.Reason, c.wantKey) {
				t.Errorf("reason %q does not name the key %q", f.Reason, c.wantKey)
			}
			if c.wantOther != "" && !strings.Contains(f.Reason, c.wantOther) {
				t.Errorf("reason %q does not name the other half %q", f.Reason, c.wantOther)
			}
		})
	}
}

// TestQASFSheetSpendFaultEachHalfOnTavernBrawl pins VTT-775/VTT-776's two
// halves through Ruleset.SheetSpendFault on the repository's ruleset: a limited
// ability without its resource (field ability_ids), a resource some ability
// spends with no named ability spending it (field resources), and a resource
// no ability spends (drink) fitting.
func TestQASFSheetSpendFaultEachHalfOnTavernBrawl(t *testing.T) {
	rs := qaSFTavern(t)
	qaSFRun(t, rs, []qaSFCase{
		{name: "limited ability with its resource fits", abilities: []string{"chair-swing"}, resources: []string{"stamina"}},
		{name: "the cellar-rats sheet fits", abilities: []string{"chair-swing", "fists", "sober-up", "splash-of-water"}, resources: []string{"drink", "stamina"}},
		{name: "empty sheet fits", abilities: nil, resources: nil},
		{name: "at-will abilities and no resource fit", abilities: []string{"fists", "sober-up"}, resources: nil},
		{name: "drink with no ability fits", abilities: nil, resources: []string{"drink"}},
		{name: "drink with at-will abilities fits", abilities: []string{"fists", "splash-of-water"}, resources: []string{"drink"}},
		{name: "limited ability without its resource", abilities: []string{"chair-swing"}, resources: nil,
			wantField: "ability_ids", wantKey: "chair-swing", wantOther: "stamina"},
		{name: "limited ability holding only drink", abilities: []string{"fists", "chair-swing"}, resources: []string{"drink"},
			wantField: "ability_ids", wantKey: "chair-swing", wantOther: "stamina"},
		{name: "spent resource with no ability", abilities: nil, resources: []string{"stamina"},
			wantField: "resources", wantKey: "stamina"},
		{name: "spent resource with only at-will abilities", abilities: []string{"fists", "sober-up"}, resources: []string{"drink", "stamina"},
			wantField: "resources", wantKey: "stamina"},
		{name: "repeated limited id with its resource fits", abilities: []string{"chair-swing", "chair-swing"}, resources: []string{"stamina"}},
	})
}

// TestQASFSheetSpendFaultOrderAndSharedResource pins VTT-775's order: ability
// ids before resources, each in byte order, not in the caller's order or the
// ruleset's declaration order; and that a resource several abilities spend
// fits while any of them is named.
func TestQASFSheetSpendFaultOrderAndSharedResource(t *testing.T) {
	rs := qaSFVariant(t)
	qaSFRun(t, rs, []qaSFCase{
		// Shared resource: stamina is spent by chair-swing and bottle-smash.
		{name: "shared resource with one spender named fits", abilities: []string{"bottle-smash"}, resources: []string{"stamina"}},
		{name: "shared resource with the other spender named fits", abilities: []string{"chair-swing"}, resources: []string{"stamina"}},
		{name: "shared resource with both spenders named fits", abilities: []string{"chair-swing", "bottle-smash"}, resources: []string{"stamina"}},
		{name: "shared resource with no spender named", abilities: []string{"fists"}, resources: []string{"stamina"},
			wantField: "resources", wantKey: "stamina"},
		// Ability ids in byte order, whatever order the caller gives.
		{name: "two limited abilities without resources, byte order", abilities: []string{"rage-punch", "chair-swing"}, resources: nil,
			wantField: "ability_ids", wantKey: "chair-swing", wantOther: "stamina"},
		{name: "first id whose resource is missing, not first limited id", abilities: []string{"rage-punch", "bottle-smash"}, resources: []string{"stamina"},
			wantField: "ability_ids", wantKey: "rage-punch", wantOther: "anger"},
		// Resources in byte order: anger is declared after stamina but sorts first.
		{name: "two orphan resources, byte order not declaration order", abilities: []string{"fists"}, resources: []string{"stamina", "anger"},
			wantField: "resources", wantKey: "anger"},
		{name: "orphan after a covered resource", abilities: []string{"rage-punch"}, resources: []string{"anger", "stamina"},
			wantField: "resources", wantKey: "stamina"},
		// Ability ids before resources even when the resource sorts first.
		{name: "abilities before resources", abilities: []string{"chair-swing"}, resources: []string{"anger"},
			wantField: "ability_ids", wantKey: "chair-swing", wantOther: "stamina"},
		{name: "drink still fits in the variant", abilities: nil, resources: []string{"drink"}},
	})
}
