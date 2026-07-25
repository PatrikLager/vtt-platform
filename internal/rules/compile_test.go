package rules_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// mustParse (shared with expr_test.go) parses src and fails the test
// immediately on error — used throughout this file to hand-build the
// *Expr values a compiled power is expected to contain, from the exact
// fully-substituted source text a reader can independently derive by
// tracing an atom's template through its composition's param bindings
// (spec §6's "hand-derived" requirement).

// TestLoadValidV2Fixture is the primary behavioral pin (task brief: "Load
// of a valid v2 fixture returns compiled powers whose flattened content
// you HAND-DERIVE in the test"). testdata/valid-v2 composes:
//
//	quick-jab = reach-delivery(reach=1) + clash-roll(clash_stat=vim, edge=2) + clash-damage(power=3)
//	rally     = reach-delivery(reach=0) + rally-effect(gain=5)
//	tag-team  = reach-delivery(reach=1) + clash-roll(clash_stat=vim, edge=0) + tag-q + tag-p
//	            (tag-q, tag-p are independent non-dependents — an
//	            in-degree-0 TIE once clash-roll is placed — P10 task-2
//	            review fix: pins the topo tie-break AND the outcome-merge
//	            order, see CompiledPower.BranchOutcomes' doc comment)
//	ward-shift = wide-delivery(max=2) + guard-check(guard_stat=brace) + ward-mark(mark=winded) + ward-clear
//	            (review fix: exercises a defense-kind param, a
//	            condition-kind param, a targeting max_targets placeholder,
//	            an apply_condition + remove_condition outcome pair (via
//	            ward-mark) plus a standalone remove_condition (via
//	            ward-clear, needed because the schema walker only probes
//	            an array's INDEX 0 — ward-mark's effects[0] is
//	            apply_condition, so remove_condition needed its own
//	            effects[0] demonstration) — and is the first fixture to
//	            populate BranchOutcomes[1] and Effects together)
//
// Tracing each atom's raw contribution template through its composition's
// bindings by hand (see testdata/valid-v2/atoms/*.json) gives the exact
// expected CompiledPower below — this test asserts the compiler produces
// precisely that, not merely "compiles without error".
func TestLoadValidV2Fixture(t *testing.T) {
	rs, err := rules.Load(fixture(t, "valid-v2"))
	if err != nil {
		t.Fatalf("Load(valid-v2): unexpected error: %v", err)
	}
	if rs.FormatVersion != "2" {
		t.Fatalf("FormatVersion = %q, want %q", rs.FormatVersion, "2")
	}
	if rs.Compiled == nil {
		t.Fatal("Compiled: want non-nil map for a v2-loaded Ruleset")
	}
	if len(rs.Compiled) != 4 {
		t.Fatalf("len(Compiled) = %d, want 4 (quick-jab, rally, tag-team, ward-shift)", len(rs.Compiled))
	}
	// v2's abilities/*.json are compositions, not v1 Ability-shaped — the
	// v1-shaped map stays present but empty (spec: Resolve reads Compiled
	// only for a v2 ruleset).
	if rs.Abilities == nil || len(rs.Abilities) != 0 {
		t.Errorf("Abilities = %v, want a non-nil, empty map for a v2-loaded Ruleset", rs.Abilities)
	}
	if len(rs.Atoms) != 10 {
		t.Fatalf("len(Atoms) = %d, want 10 (reach-delivery, clash-roll, clash-damage, rally-effect, tag-q, tag-p, wide-delivery, guard-check, ward-mark, ward-clear)", len(rs.Atoms))
	}

	t.Run("quick-jab", func(t *testing.T) {
		got, ok := rs.Compiled["quick-jab"]
		if !ok {
			t.Fatal(`Compiled["quick-jab"]: not present`)
		}

		want := &rules.CompiledPower{
			ID:        "quick-jab",
			Name:      "Quick Jab",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
			Resolution: &rules.CompiledResolution{
				// clash-roll's "1d20 + @caster.{clash_stat} + {edge}" with
				// clash_stat (attribute-kind, bare substitution) = "vim"
				// and edge (int-kind, parenthesized substitution) = 2.
				Roll: mustParse(t, "1d20 + @caster.vim + (2)"),
				// vs is written directly in the atom (no placeholder).
				Vs:       mustParse(t, "@target.brace"),
				Branches: [2]string{"connect", "graze"},
			},
			BranchOutcomes: [2][]rules.Outcome{
				{ // "connect" — clash-damage's outcome, power=3
					{
						Kind: rules.OutcomeResourceChange,
						ResourceChange: &rules.ResourceChangeOutcome{
							Resource:     "focus",
							DeltaExpr:    mustParse(t, "0 - (@caster.vigor + (3))"),
							DeltaExprSrc: "0 - (@caster.vigor + (3))",
						},
					},
				},
				nil, // "graze" — no atom contributes an outcome for it
			},
			Effects: nil,
		}

		if diff := diffCompiledPower(t, got, want); diff != "" {
			t.Errorf("Compiled[\"quick-jab\"] mismatch:\n%s", diff)
		}

		// Semantic pin, not just textual: EvalScoped against a concrete
		// context proves the compiled expression actually MEANS "1d20 +
		// caster's vim + 2" (fixedRoller below always rolls 15 on a d20).
		total, err := rules.EvalScoped(got.Resolution.Roll,
			rules.EvalContext{Attrs: map[string]int{"vim": 4}},
			rules.EvalContext{},
			fixedRoller{result: 15})
		if err != nil {
			t.Fatalf("EvalScoped(quick-jab roll): %v", err)
		}
		if total != 15+4+2 {
			t.Errorf("EvalScoped(quick-jab roll) = %d, want %d (15 + vim(4) + edge(2))", total, 15+4+2)
		}
	})

	t.Run("rally", func(t *testing.T) {
		got, ok := rs.Compiled["rally"]
		if !ok {
			t.Fatal(`Compiled["rally"]: not present`)
		}

		want := &rules.CompiledPower{
			ID:   "rally",
			Name: "Rally",
			Usage: rules.Usage{
				Limited: &rules.LimitedUsage{Resource: "focus", Cost: 1},
			},
			Targeting:      rules.Targeting{Range: 0, MaxTargets: 1},
			Resolution:     nil, // no resolution atom composed — non-attack ability
			BranchOutcomes: [2][]rules.Outcome{nil, nil},
			Effects: []rules.Outcome{
				{
					Kind: rules.OutcomeResourceChange,
					ResourceChange: &rules.ResourceChangeOutcome{
						Resource:     "focus",
						DeltaExpr:    mustParse(t, "(5)"),
						DeltaExprSrc: "(5)",
					},
				},
			},
		}

		if diff := diffCompiledPower(t, got, want); diff != "" {
			t.Errorf("Compiled[\"rally\"] mismatch:\n%s", diff)
		}
	})

	// t.Run("tag-team", ...) is the review fix wave's ordering/determinism
	// pin (P10 task-2 review, item 1): reviewer-proven gaps were a
	// reversed Kahn tie-break (all prior tests still passed) and
	// outcome-merging fed from map iteration (TestCompileDeterministic
	// still passed, since no prior fixture had 2+ outcome contributions
	// landing on the SAME branch). tag-q (compose[2]) and tag-p
	// (compose[3]) are both non-dependents of each other — neither
	// provides a key the other consumes — so both become ready
	// (in-degree 0) in the SAME Kahn step, right after clash-roll
	// (compose[1]) is placed: a genuine tie, broken by compose LIST
	// index (spec §4 guarantee 4), never by name or map order. The
	// correct order is tag-q THEN tag-p (compose[2] < compose[3]); a
	// reversed tie-break would produce tag-p then tag-q instead — this
	// test's exact-order assertion (via outcomesEqual's index-by-index
	// walk) catches either direction of bug.
	t.Run("tag-team", func(t *testing.T) {
		got, ok := rs.Compiled["tag-team"]
		if !ok {
			t.Fatal(`Compiled["tag-team"]: not present`)
		}

		want := &rules.CompiledPower{
			ID:        "tag-team",
			Name:      "Tag Team",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
			Resolution: &rules.CompiledResolution{
				// clash-roll's "1d20 + @caster.{clash_stat} + {edge}" with
				// clash_stat="vim", edge=0 (int, parenthesized).
				Roll:     mustParse(t, "1d20 + @caster.vim + (0)"),
				Vs:       mustParse(t, "@target.brace"),
				Branches: [2]string{"connect", "graze"},
			},
			BranchOutcomes: [2][]rules.Outcome{
				{ // "connect" — tag-q's effect, THEN tag-p's (compose list order, tie broken by index)
					{
						Kind: rules.OutcomeResourceChange,
						ResourceChange: &rules.ResourceChangeOutcome{
							Resource: "focus", DeltaExpr: mustParse(t, "1"), DeltaExprSrc: "1",
						},
					},
					{
						Kind: rules.OutcomeResourceChange,
						ResourceChange: &rules.ResourceChangeOutcome{
							Resource: "focus", DeltaExpr: mustParse(t, "2"), DeltaExprSrc: "2",
						},
					},
				},
				nil, // "graze"
			},
			Effects: nil,
		}

		if diff := diffCompiledPower(t, got, want); diff != "" {
			t.Errorf("Compiled[\"tag-team\"] mismatch:\n%s", diff)
		}
	})

	// t.Run("ward-shift", ...) is the review fix wave's coverage pin (P10
	// task-2 review, item 4): exercises a defense-kind param binding
	// (guard_stat="brace", substituted bare into "@target.{guard_stat}"),
	// a condition-kind param binding (mark="winded", substituted bare
	// into an apply_condition.id), a hardcoded (non-placeholder)
	// remove_condition, and a targeting max_targets "{param}" placeholder
	// (wide-delivery's "range" is a literal 2, "max_targets" is
	// "{max}"=2 — closing the gap where only "range" had ever been
	// placeholder-bound). Also the first fixture where the resolution's
	// SECOND branch ("fail") is populated, so BranchOutcomes[1] finally
	// gets asserted non-nil somewhere in this suite.
	t.Run("ward-shift", func(t *testing.T) {
		got, ok := rs.Compiled["ward-shift"]
		if !ok {
			t.Fatal(`Compiled["ward-shift"]: not present`)
		}

		want := &rules.CompiledPower{
			ID:        "ward-shift",
			Name:      "Ward Shift",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 2, MaxTargets: 2},
			Resolution: &rules.CompiledResolution{
				Roll:     mustParse(t, "1d20 + @caster.vim"),
				Vs:       mustParse(t, "@target.brace"),
				Branches: [2]string{"pass", "fail"},
			},
			BranchOutcomes: [2][]rules.Outcome{
				nil, // "pass" — nothing targets it
				{ // "fail" — ward-mark's apply_condition then remove_condition
					{Kind: rules.OutcomeApplyCondition, ApplyCondition: &rules.ApplyConditionOutcome{ID: "winded"}},
					{Kind: rules.OutcomeRemoveCondition, RemoveCondition: &rules.RemoveConditionOutcome{ID: "steadied"}},
				},
			},
			Effects: []rules.Outcome{
				// ward-clear's unconditional remove_condition.
				{Kind: rules.OutcomeRemoveCondition, RemoveCondition: &rules.RemoveConditionOutcome{ID: "steadied"}},
			},
		}

		if diff := diffCompiledPower(t, got, want); diff != "" {
			t.Errorf("Compiled[\"ward-shift\"] mismatch:\n%s", diff)
		}
	})
}

// fixedRoller always returns the same total (and one die showing that
// total) regardless of n/sides — deterministic enough to prove an
// EvalScoped result decomposes exactly as expected (die + refs), without
// needing to control which specific die faces summed to it.
type fixedRoller struct{ result int }

func (f fixedRoller) Roll(n, sides int) (results []int, total int) {
	return []int{f.result}, f.result
}

// diffCompiledPower compares got/want field by field (rather than one
// top-level reflect.DeepEqual) so a mismatch reports WHICH field, not just
// "not equal" — Expr fields are compared via String() (source-text
// round-trip; TestCompileDeterministic separately proves structural
// determinism via full DeepEqual on repeated loads).
func diffCompiledPower(t *testing.T, got, want *rules.CompiledPower) string {
	t.Helper()
	var b strings.Builder
	if got.ID != want.ID {
		b.WriteString("ID: got " + got.ID + ", want " + want.ID + "\n")
	}
	if got.Name != want.Name {
		b.WriteString("Name: got " + got.Name + ", want " + want.Name + "\n")
	}
	if !reflect.DeepEqual(got.Usage, want.Usage) {
		b.WriteString("Usage mismatch\n")
	}
	if got.Targeting != want.Targeting {
		b.WriteString("Targeting mismatch\n")
	}
	if (got.Resolution == nil) != (want.Resolution == nil) {
		b.WriteString("Resolution nil-ness mismatch\n")
	} else if got.Resolution != nil {
		if got.Resolution.Roll.String() != want.Resolution.Roll.String() {
			b.WriteString("Resolution.Roll: got " + got.Resolution.Roll.String() + ", want " + want.Resolution.Roll.String() + "\n")
		}
		if got.Resolution.Vs.String() != want.Resolution.Vs.String() {
			b.WriteString("Resolution.Vs: got " + got.Resolution.Vs.String() + ", want " + want.Resolution.Vs.String() + "\n")
		}
		if got.Resolution.Branches != want.Resolution.Branches {
			b.WriteString("Resolution.Branches mismatch\n")
		}
	}
	for i := 0; i < 2; i++ {
		if !outcomesEqual(got.BranchOutcomes[i], want.BranchOutcomes[i]) {
			b.WriteString("BranchOutcomes mismatch\n")
		}
	}
	if !outcomesEqual(got.Effects, want.Effects) {
		b.WriteString("Effects mismatch\n")
	}
	return b.String()
}

func outcomesEqual(got, want []rules.Outcome) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		g, w := got[i], want[i]
		if g.Kind != w.Kind {
			return false
		}
		switch g.Kind {
		case rules.OutcomeResourceChange:
			if g.ResourceChange.Resource != w.ResourceChange.Resource {
				return false
			}
			if g.ResourceChange.DeltaExpr.String() != w.ResourceChange.DeltaExpr.String() {
				return false
			}
		case rules.OutcomeApplyCondition:
			if g.ApplyCondition.ID != w.ApplyCondition.ID {
				return false
			}
		case rules.OutcomeRemoveCondition:
			if g.RemoveCondition.ID != w.RemoveCondition.ID {
				return false
			}
		}
	}
	return true
}

// TestCompileHygienicSplice is THE load-bearing anti-implicit-contract
// test (task brief): binding the expr "1 + 1" into the template "2 * {p}"
// must compile to "2 * (1 + 1)" and evaluate to 4 — never 3, which is what
// naive unparenthesized string concatenation ("2 * " + "1 + 1" -> parsed
// as "2 * 1 + 1") would produce. Built inline (t.TempDir()) rather than as
// a checked-in fixture: this is a single, narrow, self-explanatory
// behavioral pin, not a member of the validation-rule catalogue table.
func TestCompileHygienicSplice(t *testing.T) {
	dir := t.TempDir()
	writeSpliceFixture(t, dir, "1 + 1")

	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	cp := rs.Compiled["probe"]
	if cp == nil {
		t.Fatal(`Compiled["probe"]: not present`)
	}

	const wantSrc = "2 * (1 + 1)"
	if got := cp.Resolution.Roll.String(); got != wantSrc {
		t.Fatalf("compiled roll source = %q, want %q (hygienic splice: bound expr must be parenthesized)", got, wantSrc)
	}

	total, err := rules.EvalScoped(cp.Resolution.Roll, rules.EvalContext{}, rules.EvalContext{}, nil)
	if err != nil {
		t.Fatalf("EvalScoped: %v", err)
	}
	if total != 4 {
		t.Fatalf("EvalScoped(%q) = %d, want 4 (2 * (1 + 1)) — got 3 would mean the splice broke hygiene (2 * 1 + 1)", wantSrc, total)
	}
}

// TestCompileHygienicSpliceNameKindPrecedence is the "precedence-
// adversarial case for name-kinds too" the brief calls for: a name-kind
// param substitutes as a BARE identifier (never parenthesized — it is
// never itself a sub-expression), so it must slot into the ref position
// exactly, with no wrapping that would change how the surrounding
// expression parses. Binding "vim" into "@caster.{stat} * 2" must produce
// "@caster.vim * 2" (the ref "@caster.vim", then "* 2" — NOT
// "@caster.(vim) * 2", which wouldn't even parse, since a ref's name
// segment is a bare IDENT, not an expression position).
func TestCompileHygienicSpliceNameKindPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeSpliceFixture(t, dir, "1") // p (expr-kind) is unused by this test's assertion; kept present so the shared fixture atom still type-checks

	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	cp := rs.Compiled["probe"]
	total, err := rules.EvalScoped(cp.Resolution.Vs,
		rules.EvalContext{},
		rules.EvalContext{Attrs: map[string]int{"vim": 7}},
		nil)
	if err != nil {
		t.Fatalf("EvalScoped(vs): %v", err)
	}
	if total != 14 {
		t.Fatalf("EvalScoped(vs) = %d, want 14 (@target.vim(7) * 2)", total)
	}
}

// writeSpliceFixture writes a minimal v2 ruleset at dir with one atom
// ("probe") declaring an int-kind targeting param, an attribute-kind
// "stat" param used bare in a ref position ("@target.{stat} * 2"), and an
// expr-kind "p" param used at a value position ("2 * {p}") — and one
// ability ("probe-ability") composing it with stat="vim" and p=pExprSrc.
func writeSpliceFixture(t *testing.T, dir, pExprSrc string) {
	t.Helper()
	mkdirs(t, dir, "atoms", "abilities", "conditions")
	writeFile(t, dir+"/ruleset.json", `{
  "id": "splice-check", "name": "Splice Check", "format_version": "2",
  "attributes": ["vim"], "defenses": [], "resources": []
}`)
	writeFile(t, dir+"/guide.md", "splice check fixture\n")
	writeFile(t, dir+"/atoms/probe.json", `{
  "id": "probe",
  "params": [
    { "name": "reach", "kind": "int" },
    { "name": "stat", "kind": "attribute" },
    { "name": "p", "kind": "expr" }
  ],
  "provides": ["clash"],
  "consumes": [],
  "contributes": [
    { "kind": "targeting", "range": "{reach}", "max_targets": 1 },
    {
      "kind": "resolution",
      "key": "clash",
      "roll": "2 * {p}",
      "vs": "@target.{stat} * 2",
      "branches": ["hit", "miss"]
    }
  ]
}`)
	writeFile(t, dir+"/abilities/probe-ability.json", `{
  "id": "probe", "name": "Probe", "usage": "at_will",
  "compose": [
    { "atom": "probe", "bind": { "reach": 1, "stat": "vim", "p": "`+pExprSrc+`" } }
  ]
}`)
}

// TestCompileInjectionImpossible directly demonstrates the two shapes of
// "escape" a naive string-splice implementation would be vulnerable to,
// and pins that both are rejected before ever reaching substitution — the
// mechanism (bindAtom validates an expr-kind binding Parses standalone
// BEFORE it is ever embedded; compileTwoActorExpr validates the FINAL
// substituted expression's scope legality) makes injection structurally
// impossible, not merely disallowed by convention.
func TestCompileInjectionImpossible(t *testing.T) {
	t.Run("unbalanced_parens_cannot_escape_the_wrapping_parens", func(t *testing.T) {
		dir := t.TempDir()
		writeSpliceFixture(t, dir, "1) * 1000 + (0")
		_, err := rules.Load(dir)
		if err == nil {
			t.Fatal("Load: want error (unbalanced-paren binding must fail to parse standalone, before ever being spliced), got nil")
		}
		if !strings.Contains(err.Error(), "probe-ability.json") || !strings.Contains(err.Error(), "invalid expression") {
			t.Errorf("error = %q, want it to name probe-ability.json and report an invalid expression", err.Error())
		}
	})

	t.Run("bare_ref_smuggled_through_an_expr_param_is_still_caught", func(t *testing.T) {
		dir := t.TempDir()
		writeSpliceFixture(t, dir, "@vim")
		_, err := rules.Load(dir)
		if err == nil {
			t.Fatal("Load: want error (a bare ref smuggled in via an expr-kind param must still fail the two-actor scope check on the FINAL substituted expression), got nil")
		}
		if !strings.Contains(err.Error(), "bare reference") {
			t.Errorf("error = %q, want it to mention \"bare reference\"", err.Error())
		}
	})
}

// mkdirs creates the named subdirectories of dir.
func mkdirs(t *testing.T, dir string, sub ...string) {
	t.Helper()
	for _, s := range sub {
		if err := os.MkdirAll(dir+"/"+s, 0o755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
}

// TestLoadInvalidV2Fixtures is the v2 validation-catalogue table (task
// brief: "each with a focused invalid fixture + a file+field-naming error
// + a test") — the v2 analogue of TestLoadInvalidFixtures in load_test.go.
func TestLoadInvalidV2Fixtures(t *testing.T) {
	cases := []struct {
		dir         string
		wantErrSubs []string
	}{
		{"atom-unknown-param-kind", []string{"bad.json", "bogus"}},
		{"binding-kind-mismatch", []string{"bad-bind.json", "nope"}},
		{"atom-unknown-param-placeholder", []string{"bad.json", "nope"}},
		{"unsatisfied-consume", []string{"bad.json", "delivery"}},
		{"doubly-provided-key", []string{"bad.json", "delivery"}},
		{"dependency-cycle", []string{"bad.json", "cycle"}},
		{"zero-targeting-atoms", []string{"bad.json", "targeting"}},
		{"duplicate-targeting", []string{"bad.json", "more than one targeting"}},
		{"resolution-branch-count", []string{"roll.json", "exactly 2"}},
		{"outcome-branch-not-in-labels", []string{"bad.json", "miss", "not among"}},
		{"always-outcome-nonnull-key", []string{"dmg.json", "always"}},
		{"bare-ref-two-actor-position", []string{"bad.json", "bare reference"}},
		{"scoped-ref-single-actor-position", []string{"ruleset.json", "scoped reference"}},
		{"undeclared-resource-in-contribution", []string{"bad.json", "no_such_pool"}},
		{"attribute-defense-name-collision", []string{"ruleset.json", "brace"}},
		{"negative-int-binding", []string{"bad.json", "must not be negative"}},
		{"compose-missing-bind", []string{"bad.json", "bind"}},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			_, err := rules.Load(fixture(t, "invalid-v2", tc.dir))
			if err == nil {
				t.Fatalf("Load(invalid-v2/%s): want error, got nil", tc.dir)
			}
			for _, sub := range tc.wantErrSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("Load(invalid-v2/%s) error = %q, want it to contain %q", tc.dir, err.Error(), sub)
				}
			}
		})
	}
}

// TestCompileDeterministic pins spec §4's "never load-bearing" tie-break
// guarantee at the implementation level: repeatedly loading the same v2
// fixture in one process must produce byte-for-byte (reflect.DeepEqual)
// identical Compiled output every time. Go's per-map iteration
// randomization means a compiler that let any map range leak into output
// order (topo tie-breaks, provides/consumes bookkeeping, param binding
// lookups) would surface a mismatch within a handful of iterations — this
// loop runs enough of them in-process to make that leak-detection
// meaningful, not just a single lucky pass.
func TestCompileDeterministic(t *testing.T) {
	dir := fixture(t, "valid-v2")
	first, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i := 0; i < 50; i++ {
		rs, err := rules.Load(dir)
		if err != nil {
			t.Fatalf("Load run %d: %v", i, err)
		}
		if !reflect.DeepEqual(rs.Compiled, first.Compiled) {
			t.Fatalf("Load run %d: Compiled differs from run 0 — non-deterministic compile output (map-iteration leak)\nrun0=%#v\nrunN=%#v", i, first.Compiled, rs.Compiled)
		}
	}
}
