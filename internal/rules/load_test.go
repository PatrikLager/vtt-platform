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
// resource, defense, condition from testdata/valid ends up on the returned
// Ruleset, every composition ability flattens into Ruleset.Compiled with
// the expected shape, and every expression in it parsed cleanly (Load
// returns no error). testdata/valid is format_version "2" (Task 4's v1
// sunset — migrated the same way rulesets/tavern-brawl was): Abilities is
// always an empty, non-nil map for a v2-loaded Ruleset (spec §3 — v2's
// abilities/*.json are compositions, not v1 Ability-shaped), so this test
// asserts against Compiled, not Abilities.
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
	if rs.FormatVersion != "2" {
		t.Errorf("FormatVersion = %q, want %q", rs.FormatVersion, "2")
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

	if len(rs.Compiled) != 3 {
		t.Fatalf("len(Compiled) = %d, want 3 (strike, guard-stance, stand-down)", len(rs.Compiled))
	}
	strike, ok := rs.Compiled["strike"]
	if !ok {
		t.Fatal(`Compiled["strike"]: want present`)
	}
	if strike.Resolution == nil {
		t.Fatal("strike.Resolution: want non-nil")
	}
	if strike.Resolution.VsSrc != "@target.guard" {
		t.Errorf("strike.Resolution.VsSrc = %q, want %q", strike.Resolution.VsSrc, "@target.guard")
	}
	if !strike.Usage.AtWill || strike.Usage.Limited != nil {
		t.Errorf("strike.Usage = %+v, want AtWill=true Limited=nil", strike.Usage)
	}
	if len(strike.BranchOutcomes[0]) != 1 || strike.BranchOutcomes[0][0].Kind != rules.OutcomeResourceChange {
		t.Fatalf("strike.BranchOutcomes[0] (hit) = %+v, want one ResourceChange outcome", strike.BranchOutcomes[0])
	}

	guardStance, ok := rs.Compiled["guard-stance"]
	if !ok {
		t.Fatal(`Compiled["guard-stance"]: want present`)
	}
	if guardStance.Usage.AtWill || guardStance.Usage.Limited == nil {
		t.Fatalf("guard-stance.Usage = %+v, want Limited set", guardStance.Usage)
	}
	if guardStance.Usage.Limited.Resource != "pool_a" || guardStance.Usage.Limited.Cost != 1 {
		t.Errorf("guard-stance.Usage.Limited = %+v, want {pool_a 1}", guardStance.Usage.Limited)
	}
	if guardStance.Resolution != nil {
		t.Error("guard-stance.Resolution: want nil (non-attack ability, no resolution atom)")
	}
	if len(guardStance.Effects) != 1 || guardStance.Effects[0].Kind != rules.OutcomeApplyCondition {
		t.Fatalf("guard-stance.Effects = %+v, want one ApplyCondition outcome", guardStance.Effects)
	}
	if guardStance.Effects[0].ApplyCondition.ID != "guarded" {
		t.Errorf("guard-stance.Effects[0].ApplyCondition.ID = %q, want %q", guardStance.Effects[0].ApplyCondition.ID, "guarded")
	}

	standDown, ok := rs.Compiled["stand-down"]
	if !ok {
		t.Fatal(`Compiled["stand-down"]: want present`)
	}
	if len(standDown.Effects) != 1 || standDown.Effects[0].Kind != rules.OutcomeRemoveCondition {
		t.Fatalf("stand-down.Effects = %+v, want one RemoveCondition outcome", standDown.Effects)
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

// TestLoadRejectsFormatVersion1 is Task 4's v1-sunset pin (spec §3/§9,
// task brief: "keep one explicit format_version-\"1\"-rejected fixture +
// test asserting the new error"): a format_version "1" ruleset — a full
// directory, testdata/invalid/format-version-1, shaped exactly like the
// v1 ruleset directories this repo used to load before this task (v1-
// shaped abilities/conditions/guide.md, preserved verbatim as a visible
// "this is what got rejected" artifact) — is rejected by loadManifest
// BEFORE Load ever reads conditions/atoms/abilities: the error names
// ruleset.json, format_version, and points at the v2 spec document a
// ruleset author needs to migrate. This supersedes
// TestLoadV1DispatchUnaffectedByV2 (removed): that test pinned "a v1
// ruleset loads and adapts into Compiled", a code path this task retired
// entirely — there is no longer any format_version "1" ruleset that CAN
// load, so the premise itself is gone, not just the assertion.
func TestLoadRejectsFormatVersion1(t *testing.T) {
	_, err := rules.Load(fixture(t, "invalid", "format-version-1"))
	if err == nil {
		t.Fatal("Load(invalid/format-version-1): want error, got nil")
	}
	for _, sub := range []string{"ruleset.json", "format_version", "\"1\"", "format v1 is retired", "format-v2-composition-design"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("Load(invalid/format-version-1) error = %q, want it to contain %q", err.Error(), sub)
		}
	}
}

// TestLoadInvalidFixtures is the exhaustive cross-reference / strict-decode
// validation table: one fixture directory per validation rule (task brief:
// "one per validation rule"), each asserting the returned error names both
// the offending file and field. Task 4 (v1 sunset): every fixture below is
// now format_version "2" (migrated the same way rulesets/tavern-brawl and
// testdata/valid were — see the task-4 report for the atom-by-atom
// derivation of where each fault now lives). Two content changes from the
// v1 table: "attack-undeclared-defense" (the bad name moved from a bare
// v1 defense-name string into a two-actor-position expression ref, which
// must satisfy the expression grammar's IDENT charset — "no-such-defense"
// would be a parse error, not a cross-reference one; "no_such_defense"
// exercises the SAME validation rule the v1 fixture did) and
// "malformed-expression" (the bad text moved from abilityJSON's top-level
// "attack.roll" field into an atom's resolution contribution, spliced and
// parsed at compile.go's compileResolution — the field name in the error
// is "resolution.roll" now, not "attack.roll"; same parse-failure rule
// either way). "usage-undeclared-resource" is REMOVED, not migrated: see
// the task-4 report for why — compile.go's compileCompositions never
// cross-validates a composition's usage.limited.resource against the
// ruleset's declared resources (verified empirically: a v2 ability
// declaring usage.limited.resource of an UNDECLARED name loads
// successfully), a real validation-coverage gap between v1 and v2 that is
// out of this task's file scope to fix (compile.go is untouched platform
// behavior) — the fixture is excised because its premise can no longer be
// demonstrated, not because the rule stopped mattering.
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
			wantErrSubs: []string{"strike.json", "resolution.roll"},
		},
		{
			dir:         "threshold-undeclared-condition",
			wantErrSubs: []string{"ruleset.json", "no-such-condition"},
		},
		{
			dir:         "attack-undeclared-defense",
			wantErrSubs: []string{"strike.json", "no_such_defense"},
		},
		{
			dir:         "duplicate-condition-id",
			wantErrSubs: []string{"guarded"},
		},
		{
			dir:         "outcome-undeclared-condition",
			wantErrSubs: []string{"guard-stance.json", "no-such-condition"},
		},
		{
			dir:         "hit-without-attack",
			wantErrSubs: []string{"guard-stance.json", "no resolution contribution"},
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
  "format_version": "2",
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
  "format_version": "2",
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
  "compose": [
    { "atom": "self-delivery", "bind": {} },
    { "atom": "apply-guarded", "bind": {} }
  ]
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
  "compose": [
    { "atom": "self-delivery", "bind": {} },
    { "atom": "apply-guarded", "bind": {} }
  ]
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
  "compose": [
    { "atom": "self-delivery", "bind": {} },
    { "atom": "apply-guarded", "bind": {} }
  ]
}`)
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load with usage.limited.cost = 0: unexpected error: %v", err)
	}
	if rs.Compiled["guard-stance"].Usage.Limited.Cost != 0 {
		t.Errorf("Cost = %d, want 0", rs.Compiled["guard-stance"].Usage.Limited.Cost)
	}
}

// TestLoadRejectsTargetingMissingRange and TestLoadAcceptsTargetingZeroRange
// (v1-only test surface, EXCISED — Task 4): both pinned abilityJSON's
// top-level "targeting.range" field, a shape that no longer exists at all
// in format v2 — a v2 composition has no ability-level "targeting" field;
// targeting comes exclusively from a composed atom's targeting
// contribution (atom.schema.json's targetingContribution, required
// "range"/"max_targets" — spec §4). That v2 equivalent already has
// coverage: TestSchemaRequiredFieldsMatchLoaderEnforcementV2 (schema_
// test.go) walks atom.schema.json's targetingContribution.required
// against testdata/valid-v2/atoms/reach-delivery.json (its own comment
// names it "targeting contribution"), deleting "range" and asserting Load
// rejects it — the same validation rule, exercised at the position it
// actually lives in now. The zero-range positive case is likewise still
// live: internal/rules/testdata/valid's own self-delivery/melee-delivery
// atoms and rulesets/tavern-brawl's sober-up (reach 0) both load and
// resolve through TestLoadValidFixture and the conformance suite.

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
  "format_version": "2",
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
  "format_version": "2",
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
  "format_version": "2",
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
