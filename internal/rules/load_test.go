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

// TestLoadValidFixture pins the happy path: every property, condition and
// composition of testdata/valid ends up on the returned Ruleset, and every
// expression in it parsed.
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
	if rs.FormatVersion != "3" {
		t.Errorf("FormatVersion = %q, want %q", rs.FormatVersion, "3")
	}

	var keys, types []string
	for _, p := range rs.Properties {
		keys = append(keys, p.Key)
		types = append(types, p.Type)
	}
	if !equalStrings(keys, []string{"pool_a", "brawn", "grit", "guard"}) {
		t.Errorf("property keys = %v, want [pool_a brawn grit guard]", keys)
	}
	if !equalStrings(types, []string{"resource", "number", "number", "number"}) {
		t.Errorf("property types = %v, want [resource number number number]", types)
	}

	pool := rs.Property("pool_a")
	if pool == nil || pool.Resource == nil {
		t.Fatal(`Property("pool_a"): want a resource property`)
	}
	res := pool.Resource
	if res.DefaultMaxExpr == nil {
		t.Fatal("pool_a's DefaultMaxExpr: want non-nil")
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

// TestLoadRejectsFormatVersion1 loads a whole format v1 directory, which is
// refused on its format_version before anything else is read.
// VTT-616
func TestLoadRejectsFormatVersion1(t *testing.T) {
	_, err := rules.Load(fixture(t, "invalid", "format-version-1"))
	if err == nil {
		t.Fatal("Load(invalid/format-version-1): want error, got nil")
	}
	for _, sub := range []string{"ruleset.json", `field "format_version": unsupported value "1": this loader reads format_version "3"`, "docs/adr/012-the-rules-contract.md"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("Load(invalid/format-version-1) error = %q, want it to contain %q", err.Error(), sub)
		}
	}
}

// TestLoadInvalidFixtures loads one fixture directory per validation rule,
// each refusal naming the offending file and the key or field that fails.
// "attack-undeclared-defense" keeps its name for a number property that a
// resolution's vs reads.
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
		// The two below reach checkExprRefs through its RESOURCE call sites
		// (load.go:465, :470) rather than through an ability. Inverting its
		// nil guard makes it skip every non-nil expression, and the whole
		// suite still passed: undeclared-attribute and unknown-resource-ref
		// above go through a different path entirely.
		{
			dir:         "resource-default-max-undeclared-ref",
			wantErrSubs: []string{"ruleset.json", "default_max_expr", "nosuch"},
		},
		{
			dir:         "threshold-when-undeclared-ref",
			wantErrSubs: []string{"ruleset.json", "thresholds[0].when", "nosuch_pool"},
		},
		{
			dir:         "bad-format-version",
			wantErrSubs: []string{"ruleset.json", "format_version"},
		},
		{
			dir:         "unknown-field",
			wantErrSubs: []string{"unknown field"},
		},
		{
			dir:         "duplicate-ability-id",
			wantErrSubs: []string{"duplicate ability id"},
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
			wantErrSubs: []string{"duplicate condition id"},
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

	// Catalogue completeness, checked from the SAME table the assertions use.
	// internal/adventure has had this since its catalogue grew; internal/rules
	// did not, so three directories added 2026-08-05 had nothing to satisfy.
	// An orphaned fixture is worse than a missing one -- it looks like coverage
	// in a directory listing while no test ever loads it.
	named := map[string]bool{
		// Loaded by its OWN test rather than through this table, because it
		// asserts the format-v1 retirement message specifically. Listed here
		// so the completeness check does not read it as an orphan -- and so
		// that deleting that test makes this line the next thing to notice.
		"format-version-1": true,
	}
	for _, tc := range cases {
		named[tc.dir] = true
	}
	entries, err := os.ReadDir(filepath.Join("testdata", "invalid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() && !named[e.Name()] {
			t.Errorf("testdata/invalid/%s exists but no case names it -- nothing loads it", e.Name())
		}
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
  "format_version": "3",
  "properties": [
    {
      "key": "brawn",
      "type": "number",
      "name": "Brawn",
      "description": "The fixture's brawn."
    },
    {
      "key": "grit",
      "type": "number",
      "name": "Grit",
      "description": "The fixture's grit."
    },
    {
      "key": "guard",
      "type": "number",
      "name": "Guard",
      "description": "The fixture's guard."
    },
    {
      "key": "pool_a",
      "type": "resource",
      "name": "Pool a",
      "description": "The fixture's pool a.",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        {
          "when": "#pool_a",
          "apply_condition": "guarded"
        }
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
  "format_version": "3",
  "properties": [
    {
      "key": "brawn",
      "type": "number",
      "name": "Brawn",
      "description": "The fixture's brawn."
    },
    {
      "key": "grit",
      "type": "number",
      "name": "Grit",
      "description": "The fixture's grit."
    },
    {
      "key": "guard",
      "type": "number",
      "name": "Guard",
      "description": "The fixture's guard."
    },
    {
      "key": "pool_a",
      "type": "resource",
      "name": "Pool a",
      "description": "The fixture's pool a.",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        {
          "when": "#pool_a",
          "apply_condition": "guarded",
          "remove_when_false": false
        }
      ]
    }
  ]
}`)
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load with explicit remove_when_false=false: unexpected error: %v", err)
	}
	if rs.Property("pool_a").Resource.Thresholds[0].RemoveWhenFalse {
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
  "format_version": "3",
  "properties": [
    {
      "key": "brawn",
      "type": "number",
      "name": "Brawn",
      "description": "The fixture's brawn."
    },
    {
      "key": "grit-bonus",
      "type": "number",
      "name": "Grit-bonus",
      "description": "The fixture's grit-bonus."
    },
    {
      "key": "guard",
      "type": "number",
      "name": "Guard",
      "description": "The fixture's guard."
    },
    {
      "key": "pool_a",
      "type": "resource",
      "name": "Pool a",
      "description": "The fixture's pool a.",
      "default_max_expr": "10 + @brawn",
      "thresholds": []
    }
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
  "format_version": "3",
  "properties": [
    {
      "key": "brawn",
      "type": "number",
      "name": "Brawn",
      "description": "The fixture's brawn."
    },
    {
      "key": "grit",
      "type": "number",
      "name": "Grit",
      "description": "The fixture's grit."
    },
    {
      "key": "guard",
      "type": "number",
      "name": "Guard",
      "description": "The fixture's guard."
    },
    {
      "key": "pool-a",
      "type": "resource",
      "name": "Pool-a",
      "description": "The fixture's pool-a.",
      "thresholds": []
    }
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
  "format_version": "3",
  "properties": [
    {
      "key": "brawn",
      "type": "number",
      "name": "Brawn",
      "description": "The fixture's brawn."
    },
    {
      "key": "grit",
      "type": "number",
      "name": "Grit",
      "description": "The fixture's grit."
    },
    {
      "key": "will-power",
      "type": "number",
      "name": "Will-power",
      "description": "The fixture's will-power."
    },
    {
      "key": "pool_a",
      "type": "resource",
      "name": "Pool a",
      "description": "The fixture's pool a.",
      "thresholds": []
    }
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

type rulesetBound struct {
	ability, condition, number, resource, branch string
}

// numberAt adds a number property under key at the front of dir's
// properties.
func numberAt(t *testing.T, dir, key string) {
	t.Helper()
	rulesetEdit(t, dir, "ruleset.json", `"properties": [`, `"properties": [
    { "key": "`+key+`", "type": "number", "name": "At the bound", "description": "A key at the bound." },`)
}

func rulesetEdit(t *testing.T, dir, file, old, replacement string) {
	t.Helper()
	path := filepath.Join(dir, file)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s holds no %q", file, old)
	}
	writeFile(t, path, strings.ReplaceAll(string(data), old, replacement))
}

func rulesetCopy(t *testing.T, src string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	copyDir(t, src, dir)
	return dir
}

func rulesetAtTheBound(t *testing.T) (string, rulesetBound) {
	t.Helper()
	b := rulesetBound{
		ability:   strings.Repeat("b", 128),
		condition: strings.Repeat("é", 64),
		number:    strings.Repeat("x", 128),
		resource:  "p" + strings.Repeat("a", 127),
		branch:    strings.Repeat("m", 128),
	}
	dir := rulesetCopy(t, fixture(t, "valid"))
	for _, f := range []string{"ruleset.json", "conditions/guarded.json", "atoms/apply-guarded.json", "atoms/remove-guarded.json"} {
		rulesetEdit(t, dir, f, `"guarded"`, `"`+b.condition+`"`)
	}
	for _, f := range []string{"ruleset.json", "atoms/strike-damage.json", "abilities/guard-stance.json"} {
		rulesetEdit(t, dir, f, "pool_a", b.resource)
	}
	numberAt(t, dir, b.number)
	rulesetEdit(t, dir, "atoms/strike-roll.json", `"miss"`, `"`+b.branch+`"`)
	writeFile(t, filepath.Join(dir, "abilities", "at-the-bound.json"), `{
  "id": "`+b.ability+`",
  "name": "At The Bound",
  "usage": { "limited": { "resource": "`+b.resource+`", "cost": 1 } },
  "compose": [
    { "atom": "self-delivery", "bind": {} },
    { "atom": "apply-guarded", "bind": {} }
  ]
}`)
	return dir, b
}

// VTT-624 VTT-284
func TestARulesetWhoseIDOrNameExceedsTheBoundIsRefused(t *testing.T) {
	over := strings.Repeat("o", 129)
	for _, c := range []struct {
		name, file, field string
		edit              func(t *testing.T, dir string)
	}{
		{"ability id", "strike.json", "id", func(t *testing.T, dir string) {
			t.Helper()
			rulesetEdit(t, dir, "abilities/strike.json", `"id": "strike"`, `"id": "`+over+`"`)
		}},
		{"condition id", "guarded.json", "id", func(t *testing.T, dir string) {
			t.Helper()
			for _, f := range []string{"ruleset.json", "conditions/guarded.json", "atoms/apply-guarded.json", "atoms/remove-guarded.json"} {
				rulesetEdit(t, dir, f, `"guarded"`, `"`+strings.Repeat("é", 64)+`i"`)
			}
		}},
		{"number key", "ruleset.json", "properties[0].key", func(t *testing.T, dir string) {
			t.Helper()
			numberAt(t, dir, over)
		}},
		{"resource key", "ruleset.json", "properties[0].key", func(t *testing.T, dir string) {
			t.Helper()
			for _, f := range []string{"ruleset.json", "atoms/strike-damage.json", "abilities/guard-stance.json"} {
				rulesetEdit(t, dir, f, "pool_a", over)
			}
		}},
		{"branch label", "strike-roll.json", "contributes[0].branches[1]", func(t *testing.T, dir string) {
			t.Helper()
			rulesetEdit(t, dir, "atoms/strike-roll.json", `"miss"`, `"`+over+`"`)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := rulesetCopy(t, fixture(t, "valid"))
			c.edit(t, dir)
			_, err := rules.Load(dir)
			want := c.file + `: field "` + c.field + `": must be at most 128 bytes, got 129`
			if err == nil || !strings.HasSuffix(err.Error(), want) {
				t.Fatalf("Load = %v, want an error ending %q", err, want)
			}
		})
	}
}

// VTT-624 VTT-284
func TestARulesetWhoseIDsAndNamesAreExactlyTheBoundLoads(t *testing.T) {
	dir, b := rulesetAtTheBound(t)
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("a ruleset whose ids and names are 128 bytes was refused: %v", err)
	}
	if rs.Compiled[b.ability] == nil {
		t.Errorf("no compiled ability under the 128-byte id")
	}
	if rs.Conditions[b.condition] == nil {
		t.Errorf("no condition under the 128-byte id")
	}
	if p := rs.Property(b.number); p == nil || p.Type != rules.PropertyNumber {
		t.Errorf("no number property under the 128-byte key")
	}
	if p := rs.Property(b.resource); p == nil || p.Resource == nil {
		t.Errorf("no resource property under the 128-byte key")
	}
	if strike := rs.Compiled["strike"]; strike == nil || strike.Resolution == nil || strike.Resolution.Branches[1] != b.branch {
		t.Errorf("strike's resolution does not hold the 128-byte branch label")
	}
}

// VTT-624
func TestARulesetNameOfSeventyThousandBytesIsRefusedAtLoad(t *testing.T) {
	dir := rulesetCopy(t, filepath.Join("conformance", "testdata", "minimal-smoke-fail"))
	long := strings.Repeat("f", 70000)
	for _, f := range []string{"ruleset.json", "abilities/big-move.json"} {
		rulesetEdit(t, dir, f, `"focus"`, `"`+long+`"`)
	}
	_, err := rules.Load(dir)
	want := `ruleset.json: field "properties[0].key": must be at most 128 bytes, got 70000`
	if err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("Load = %.200v, want an error ending %q", err, want)
	}
	if len(err.Error()) >= len(dir)+128 {
		t.Fatalf("the refusal is %d bytes, want under the directory's path plus 128", len(err.Error()))
	}
}
