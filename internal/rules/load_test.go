package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func fixture(t *testing.T, elem ...string) string {
	t.Helper()
	return filepath.Join(append([]string{"testdata"}, elem...)...)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %s: %v", path, err)
	}
}

// TestLoadValidFixture pins the full happy path: every declared attribute,
// resource, defense, ability, and condition from testdata/valid ends up on
// the returned Ruleset, and every expression in it parsed cleanly (Load
// returns no error).
func TestLoadValidFixture(t *testing.T) {
	rs, err := rules.Load(fixture(t, "valid"))
	if err != nil {
		t.Fatalf("Load(valid): unexpected error: %v", err)
	}
	if rs == nil {
		t.Fatal("Load(valid): want non-nil Ruleset")
	}

	if rs.ID != "test-ruleset" {
		t.Errorf("ID = %q, want %q", rs.ID, "test-ruleset")
	}
	if rs.Name != "Test Ruleset" {
		t.Errorf("Name = %q, want %q", rs.Name, "Test Ruleset")
	}
	if rs.FormatVersion != "1" {
		t.Errorf("FormatVersion = %q, want %q", rs.FormatVersion, "1")
	}

	if !equalStrings(rs.Attributes, []string{"brawn", "grit"}) {
		t.Errorf("Attributes = %v, want [brawn grit]", rs.Attributes)
	}
	if !equalStrings(rs.Defenses, []string{"guard"}) {
		t.Errorf("Defenses = %v, want [guard]", rs.Defenses)
	}

	if len(rs.Resources) != 1 {
		t.Fatalf("len(Resources) = %d, want 1", len(rs.Resources))
	}
	res := rs.Resources[0]
	if res.Name != "pool_a" {
		t.Errorf("Resources[0].Name = %q, want %q", res.Name, "pool_a")
	}
	if res.DefaultMaxExpr == nil {
		t.Fatal("Resources[0].DefaultMaxExpr: want non-nil")
	}
	if got, err := rules.Eval(res.DefaultMaxExpr, map[string]int{"brawn": 3}, nil, nil); err != nil || got != 13 {
		t.Errorf("Eval(default_max_expr) = (%d, %v), want (13, nil)", got, err)
	}
	if len(res.Thresholds) != 1 {
		t.Fatalf("len(Resources[0].Thresholds) = %d, want 1", len(res.Thresholds))
	}
	th := res.Thresholds[0]
	if th.ApplyCondition != "guarded" {
		t.Errorf("Thresholds[0].ApplyCondition = %q, want %q", th.ApplyCondition, "guarded")
	}
	if !th.RemoveWhenFalse {
		t.Error("Thresholds[0].RemoveWhenFalse: want true")
	}
	if th.When == nil {
		t.Fatal("Thresholds[0].When: want non-nil")
	}

	if len(rs.Abilities) != 3 {
		t.Fatalf("len(Abilities) = %d, want 3 (strike, guard-stance, stand-down)", len(rs.Abilities))
	}
	strike, ok := rs.Abilities["strike"]
	if !ok {
		t.Fatal(`Abilities["strike"]: want present`)
	}
	if strike.Attack == nil {
		t.Fatal("strike.Attack: want non-nil")
	}
	if strike.Attack.Vs != "guard" {
		t.Errorf("strike.Attack.Vs = %q, want %q", strike.Attack.Vs, "guard")
	}
	if !strike.Usage.AtWill || strike.Usage.Limited != nil {
		t.Errorf("strike.Usage = %+v, want AtWill=true Limited=nil", strike.Usage)
	}
	if len(strike.Hit) != 1 || strike.Hit[0].Kind != rules.OutcomeResourceChange {
		t.Fatalf("strike.Hit = %+v, want one ResourceChange outcome", strike.Hit)
	}

	guardStance, ok := rs.Abilities["guard-stance"]
	if !ok {
		t.Fatal(`Abilities["guard-stance"]: want present`)
	}
	if guardStance.Usage.AtWill || guardStance.Usage.Limited == nil {
		t.Fatalf("guard-stance.Usage = %+v, want Limited set", guardStance.Usage)
	}
	if guardStance.Usage.Limited.Resource != "pool_a" || guardStance.Usage.Limited.Cost != 1 {
		t.Errorf("guard-stance.Usage.Limited = %+v, want {pool_a 1}", guardStance.Usage.Limited)
	}
	if guardStance.Attack != nil {
		t.Error("guard-stance.Attack: want nil (non-attack ability)")
	}
	if len(guardStance.Effect) != 1 || guardStance.Effect[0].Kind != rules.OutcomeApplyCondition {
		t.Fatalf("guard-stance.Effect = %+v, want one ApplyCondition outcome", guardStance.Effect)
	}
	if guardStance.Effect[0].ApplyCondition.ID != "guarded" {
		t.Errorf("guard-stance.Effect[0].ApplyCondition.ID = %q, want %q", guardStance.Effect[0].ApplyCondition.ID, "guarded")
	}

	standDown, ok := rs.Abilities["stand-down"]
	if !ok {
		t.Fatal(`Abilities["stand-down"]: want present`)
	}
	if len(standDown.Effect) != 1 || standDown.Effect[0].Kind != rules.OutcomeRemoveCondition {
		t.Fatalf("stand-down.Effect = %+v, want one RemoveCondition outcome", standDown.Effect)
	}

	if len(rs.Conditions) != 1 {
		t.Fatalf("len(Conditions) = %d, want 1", len(rs.Conditions))
	}
	guarded, ok := rs.Conditions["guarded"]
	if !ok {
		t.Fatal(`Conditions["guarded"]: want present`)
	}
	if guarded.Name != "Guarded" {
		t.Errorf("Conditions[guarded].Name = %q, want %q", guarded.Name, "Guarded")
	}

	if !strings.Contains(rs.Guide, "Test Ruleset") {
		t.Errorf("Guide = %q, want it to contain guide.md's content", rs.Guide)
	}
}

// TestLoadMissingDirectory pins the not-a-directory rejection.
func TestLoadMissingDirectory(t *testing.T) {
	_, err := rules.Load(fixture(t, "does-not-exist"))
	if err == nil {
		t.Fatal("Load(does-not-exist): want error, got nil")
	}
}

// TestLoadV1DispatchUnaffectedByV2 pins Load's format_version dispatch
// (P10 task-2): a format_version "1" ruleset takes the EXACT same v1 path
// it always has — Compiled/Atoms are nil (v2-only fields), Abilities is
// v1-Ability-shaped and non-empty, exactly like TestLoadValidFixture
// already established before this task existed. This is the thin
// regression pin for "v1 loading stays fully intact" at the Load()
// entry-point level; TestLoadValidFixture and the rest of this file cover
// v1's actual decode/validation behavior in depth and are unchanged.
func TestLoadV1DispatchUnaffectedByV2(t *testing.T) {
	rs, err := rules.Load(fixture(t, "valid"))
	if err != nil {
		t.Fatalf("Load(valid): unexpected error: %v", err)
	}
	if rs.FormatVersion != "1" {
		t.Fatalf("FormatVersion = %q, want %q", rs.FormatVersion, "1")
	}
	if rs.Compiled != nil {
		t.Errorf("Compiled = %v, want nil for a format_version \"1\" Ruleset", rs.Compiled)
	}
	if rs.Atoms != nil {
		t.Errorf("Atoms = %v, want nil for a format_version \"1\" Ruleset", rs.Atoms)
	}
	if len(rs.Abilities) == 0 {
		t.Error("Abilities: want the usual non-empty v1 ability set, unaffected by v2's addition")
	}
}

// TestLoadInvalidFixtures is the exhaustive cross-reference / strict-decode
// validation table: one fixture directory per validation rule (task brief:
// "one per validation rule"), each asserting the returned error names both
// the offending file and field.
func TestLoadInvalidFixtures(t *testing.T) {
	cases := []struct {
		dir         string
		wantErrSubs []string // every substring must appear in err.Error()
	}{
		{
			dir:         "unknown-resource-ref",
			wantErrSubs: []string{"strike.json", "no_such_pool"},
		},
		{
			dir:         "undeclared-attribute",
			wantErrSubs: []string{"strike.json", "no_such_attr"},
		},
		{
			dir:         "bad-format-version",
			wantErrSubs: []string{"ruleset.json", "format_version"},
		},
		{
			dir:         "unknown-field",
			wantErrSubs: []string{"ruleset.json"},
		},
		{
			dir:         "duplicate-ability-id",
			wantErrSubs: []string{"strike"},
		},
		{
			dir:         "malformed-expression",
			wantErrSubs: []string{"strike.json", "attack.roll"},
		},
		{
			dir:         "threshold-undeclared-condition",
			wantErrSubs: []string{"ruleset.json", "no-such-condition"},
		},
		{
			dir:         "attack-undeclared-defense",
			wantErrSubs: []string{"strike.json", "no-such-defense"},
		},
		{
			dir:         "duplicate-condition-id",
			wantErrSubs: []string{"guarded"},
		},
		{
			dir:         "usage-undeclared-resource",
			wantErrSubs: []string{"guard-stance.json", "no_such_pool"},
		},
		{
			dir:         "outcome-undeclared-condition",
			wantErrSubs: []string{"guard-stance.json", "no-such-condition"},
		},
		{
			dir:         "hit-without-attack",
			wantErrSubs: []string{"guard-stance.json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			_, err := rules.Load(fixture(t, "invalid", tc.dir))
			if err == nil {
				t.Fatalf("Load(invalid/%s): want error, got nil", tc.dir)
			}
			for _, sub := range tc.wantErrSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("Load(invalid/%s) error = %q, want it to contain %q", tc.dir, err.Error(), sub)
				}
			}
		})
	}
}

// --- schema/loader-drift fixes (review fix wave): numeric/presence gaps a
// plain Go zero value can't distinguish from an author's explicit,
// legitimate zero/false. Each test below is a direct pin of one such gap;
// the generalized TestSchemaRequiredFieldsMatchLoaderEnforcement in
// schema_test.go additionally exercises these (and every other nested
// "required" the schema documents declare) structurally.

// TestLoadRejectsThresholdMissingRemoveWhenFalse: remove_when_false must be
// present in the JSON. A plain `bool` field can't tell "author wrote
// false" apart from "author omitted the key entirely" — both decode to
// Go's zero value. Omission is a fixture/authoring bug the loader must
// catch, not silently treat as false.
func TestLoadRejectsThresholdMissingRemoveWhenFalse(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "1",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        { "when": "#pool_a", "apply_condition": "guarded" }
      ]
    }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with thresholds[].remove_when_false absent: want error, got nil")
	}
	if !strings.Contains(err.Error(), "remove_when_false") {
		t.Errorf("error = %q, want it to mention remove_when_false", err)
	}
}

// TestLoadAcceptsThresholdExplicitFalse is the companion positive case:
// an explicit `"remove_when_false": false` must still load cleanly (the
// fix targets ABSENCE, not the value false).
func TestLoadAcceptsThresholdExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "1",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        { "when": "#pool_a", "apply_condition": "guarded", "remove_when_false": false }
      ]
    }
  ]
}`)
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load with explicit remove_when_false=false: unexpected error: %v", err)
	}
	if rs.Resources[0].Thresholds[0].RemoveWhenFalse {
		t.Error("RemoveWhenFalse = true, want false (explicit value must round-trip)")
	}
}

// TestLoadRejectsUsageLimitedMissingCost: usage.limited.cost must be
// present. A plain `int` can't tell "author wrote 0" (a legitimate,
// free-to-use-but-still-limited ability) apart from "author omitted cost
// entirely" (almost certainly a mistake).
func TestLoadRejectsUsageLimitedMissingCost(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "abilities", "guard-stance.json"), `{
  "id": "guard-stance",
  "name": "Guard Stance",
  "usage": { "limited": { "resource": "pool_a" } },
  "targeting": { "range": 0, "max_targets": 1 },
  "effect": [ { "apply_condition": { "id": "guarded" } } ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with usage.limited.cost absent: want error, got nil")
	}
	if !strings.Contains(err.Error(), "cost") {
		t.Errorf("error = %q, want it to mention cost", err)
	}
}

// TestLoadRejectsUsageLimitedNegativeCost: a negative cost would let
// Resolve (Task 5) GRANT the resource back on every use instead of
// spending it — economy-breaking. Must be rejected at load time, not
// discovered during play.
func TestLoadRejectsUsageLimitedNegativeCost(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "abilities", "guard-stance.json"), `{
  "id": "guard-stance",
  "name": "Guard Stance",
  "usage": { "limited": { "resource": "pool_a", "cost": -5 } },
  "targeting": { "range": 0, "max_targets": 1 },
  "effect": [ { "apply_condition": { "id": "guarded" } } ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with usage.limited.cost = -5: want error, got nil")
	}
	if !strings.Contains(err.Error(), "cost") {
		t.Errorf("error = %q, want it to mention cost", err)
	}
}

// TestLoadAcceptsUsageLimitedZeroCost is the companion positive case: an
// explicit cost of 0 is legitimate (a limited-use-count ability that costs
// no resource points) and must still load cleanly.
func TestLoadAcceptsUsageLimitedZeroCost(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "abilities", "guard-stance.json"), `{
  "id": "guard-stance",
  "name": "Guard Stance",
  "usage": { "limited": { "resource": "pool_a", "cost": 0 } },
  "targeting": { "range": 0, "max_targets": 1 },
  "effect": [ { "apply_condition": { "id": "guarded" } } ]
}`)
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load with usage.limited.cost = 0: unexpected error: %v", err)
	}
	if rs.Abilities["guard-stance"].Usage.Limited.Cost != 0 {
		t.Errorf("Cost = %d, want 0", rs.Abilities["guard-stance"].Usage.Limited.Cost)
	}
}

// TestLoadRejectsTargetingMissingRange: targeting.range must be present.
// A plain `int` can't tell "author wrote 0" (a legitimate self/no-range
// ability, e.g. valid/abilities/guard-stance.json) apart from "author
// omitted range entirely".
func TestLoadRejectsTargetingMissingRange(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "abilities", "strike.json"), `{
  "id": "strike",
  "name": "Strike",
  "usage": "at_will",
  "targeting": { "max_targets": 1 },
  "attack": { "roll": "1d20 + @brawn", "vs": "guard" },
  "hit": [ { "resource_change": { "resource": "pool_a", "delta_expr": "0 - 1" } } ],
  "miss": [],
  "effect": []
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with targeting.range absent: want error, got nil")
	}
	if !strings.Contains(err.Error(), "range") {
		t.Errorf("error = %q, want it to mention range", err)
	}
}

// TestLoadAcceptsTargetingZeroRange is the companion positive case: range
// 0 is a legitimate, already-fixtured value (valid/abilities/guard-stance.json)
// and must still load cleanly.
func TestLoadAcceptsTargetingZeroRange(t *testing.T) {
	rs, err := rules.Load(fixture(t, "valid"))
	if err != nil {
		t.Fatalf("Load(valid): unexpected error: %v", err)
	}
	if rs.Abilities["guard-stance"].Targeting.Range != 0 {
		t.Errorf("guard-stance.Targeting.Range = %d, want 0", rs.Abilities["guard-stance"].Targeting.Range)
	}
}

// TestLoadRejectsInvalidAttributeIdentifier pins that manifest attribute/
// defense/resource names must match the expression grammar's IDENT
// production ([A-Za-z_][A-Za-z0-9_]*) — anything else can be declared but
// can never actually be referenced via '@'/'#' in an expression (e.g. a
// hyphenated name lexes as subtraction, not one identifier; see
// isValidIdentName's doc). Caught at load time rather than surfacing as a
// confusing "undeclared attribute" cross-ref error the first time someone
// tries to reference it.
func TestLoadRejectsInvalidAttributeIdentifier(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "1",
  "attributes": ["brawn", "grit-bonus"],
  "defenses": ["guard"],
  "resources": [
    { "name": "pool_a", "default_max_expr": "10 + @brawn", "thresholds": [] }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal(`Load with attribute name "grit-bonus": want error, got nil`)
	}
	if !strings.Contains(err.Error(), "grit-bonus") {
		t.Errorf("error = %q, want it to name grit-bonus", err)
	}
}

// TestLoadRejectsInvalidResourceIdentifier: same rule, resource names.
func TestLoadRejectsInvalidResourceIdentifier(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "1",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    { "name": "pool-a", "thresholds": [] }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal(`Load with resource name "pool-a": want error, got nil`)
	}
	if !strings.Contains(err.Error(), "pool-a") {
		t.Errorf("error = %q, want it to name pool-a", err)
	}
}

// TestLoadRejectsInvalidDefenseIdentifier: same rule, defense names — kept
// consistent with attributes/resources even though defense names are
// currently only ever used as plain string field values (attack.vs), never
// lexed inside an expression, so ruleset authors don't have to remember
// which of the three declared-name categories allows hyphens.
func TestLoadRejectsInvalidDefenseIdentifier(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "1",
  "attributes": ["brawn", "grit"],
  "defenses": ["will-power"],
  "resources": [
    { "name": "pool_a", "thresholds": [] }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal(`Load with defense name "will-power": want error, got nil`)
	}
	if !strings.Contains(err.Error(), "will-power") {
		t.Errorf("error = %q, want it to name will-power", err)
	}
}

// TestLoadAcceptsValidIdentifierNames is the companion positive case: the
// existing valid fixture's attribute/defense/resource names (brawn, grit,
// guard, pool_a) already satisfy the charset and must keep loading
// cleanly — a regression guard against the new check being too strict.
func TestLoadAcceptsValidIdentifierNames(t *testing.T) {
	if _, err := rules.Load(fixture(t, "valid")); err != nil {
		t.Fatalf("Load(valid): unexpected error: %v", err)
	}
}
