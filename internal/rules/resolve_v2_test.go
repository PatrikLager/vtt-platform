package rules_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// --- Resolve against REAL format-v2 compositions (Task 3) ---
//
// Loads testdata/valid-v2 (Task 2's fixture — proving_grounds: attributes
// vim/vigor, defense brace, resource focus with a nonzero->winded
// removeWhenFalse threshold) and exercises quick-jab, rally, tag-team, and
// ward-shift end to end through the real Load -> compile.go pipeline (not
// hand-built structs, unlike fixtureRuleset — this is the "against the
// valid-v2 fixture" half of the task brief's requirement). Every
// CompiledPower's exact flattened shape is independently pinned by
// TestLoadValidV2Fixture (compile_test.go); these tests hand-derive the
// EXECUTION batch a fixed-seed roller must produce, proving Resolve's new
// EvalScoped-based two-actor execution against a real compiled composition,
// not just against the v1-adapted fixtures fixtureRuleset already covers.
//
// A vs-expression-with-dice case (proving Vs is recorded onto
// AbilityUsed.rolls, in "roll then vs" order, ONLY when it actually rolls)
// is deliberately NOT added to testdata/valid-v2 itself: doing so would
// change len(rs.Compiled)/len(rs.Atoms), breaking compile_test.go's
// pinned exact counts (TestLoadValidV2Fixture, TestCompileDeterministic) —
// compile_test.go is Task 2's file, out of this task's scope. See
// TestResolveV2VsExpressionWithDiceRecordsInOrder below, which hand-builds
// a standalone CompiledPower (exactly fixtureRuleset's own established
// idiom for exercising Resolve without depending on Load) for that one
// case instead.

func loadV2Fixture(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(fixture(t, "valid-v2"))
	if err != nil {
		t.Fatalf("Load(valid-v2): unexpected error: %v", err)
	}
	return rs
}

// TestResolveV2QuickJabConnect: quick-jab = reach-delivery(reach=1) +
// clash-roll(clash_stat=vim, edge=2) + clash-damage(power=3). Roll "1d20 +
// @caster.vim + (2)" (die 15, vim 4 -> total 21) >= Vs "@target.brace" (10,
// no dice, never recorded) selects "connect": clash-damage's outcome
// applies delta_expr "0 - (@caster.vigor + (3))" (caster vigor 5 -> -8) to
// the TARGET's own "focus" resource (10 -> 2, clamped nowhere since 2 >=
// 0), which crosses the threshold's own "#focus" check (still nonzero) —
// no, wait: focus was already nonzero before this resolve (10), so the
// threshold doesn't "cross" here, it just re-affirms an already-satisfied
// condition; winded is asserted present in the batch regardless (the
// threshold pass doesn't know or care whether this is the first time it's
// evaluated true — see resolve.go's evalThresholds doc, it runs on every
// (actor,resource) pair that changed value this Resolve call).
func TestResolveV2QuickJabConnect(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 4, "vigor": 5}, nil)
	putActor(st, "b", map[string]int32{"brace": 10}, map[string]*vttv1.Resource{"focus": res(10, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "quick-jab", "b"), &queueRoller{queue: []int{15}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "quick-jab", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.vim + (2)", Results: []int32{15}, Total: 21}},
			OutcomeSummary: "Quick Jab on b: connect (21 vs 10)",
		}),
		envResourceChanged("b", "focus", -8, 2, "ability:quick-jab:connect"),
		envConditionApplied("b", "winded", "threshold:focus"),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2QuickJabGraze proves the "graze" (miss-equivalent) branch
// path: a low roll never reaches BranchOutcomes[0] at all, and quick-jab
// contributes nothing to "graze" (BranchOutcomes[1] is nil) — so the
// batch is exactly the AbilityUsed event, no ResourceChanged, no threshold
// event (focus never changed this Resolve call).
func TestResolveV2QuickJabGraze(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 4, "vigor": 5}, nil)
	putActor(st, "b", map[string]int32{"brace": 30}, map[string]*vttv1.Resource{"focus": res(10, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "quick-jab", "b"), &queueRoller{queue: []int{2}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "quick-jab", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.vim + (2)", Results: []int32{2}, Total: 8}},
			OutcomeSummary: "Quick Jab on b: graze (8 vs 30)",
		}),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2RallyNonAttackEffect: rally = reach-delivery(reach=0) +
// rally-effect(gain=5) — no resolution atom composed at all (Resolution
// nil), Usage limited(focus, cost 1), Effects unconditionally applies
// delta_expr "(5)" to the target's focus. Self-targeting (range 0),
// proving a v2 non-attack composition runs the EXACT same
// Resolution==nil path (Effects-only, plain "%s on %s" summary) v1's
// non-attack abilities (brace/unbrace, fixtureRuleset) already exercise —
// same code, same shape, different format_version's data.
func TestResolveV2RallyNonAttackEffect(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 4, "vigor": 5}, map[string]*vttv1.Resource{"focus": res(3, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "rally", "a"), &queueRoller{})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "rally", TargetIds: []string{"a"},
			OutcomeSummary: "Rally on a",
		}),
		envResourceChanged("a", "focus", -1, 2, "ability:rally:usage"),
		envResourceChanged("a", "focus", 5, 7, "ability:rally:effect"),
		envConditionApplied("a", "winded", "threshold:focus"),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2TagTeamOutcomeMergeOrder: tag-team composes clash-roll plus
// two independent atoms (tag-q, tag-p) that both contribute a
// resource_change to the SAME "connect" branch — CompiledPower.
// BranchOutcomes' documented merge order (topo order, tag-q before tag-p,
// pinned at compile time by TestLoadValidV2Fixture's own "tag-team"
// subtest) must be preserved through EXECUTION too: this proves Resolve
// applies BranchOutcomes[0] in exactly that order (+1 THEN +2 — a
// reversed order would still total the same final new_value on the LAST
// event but would swap which ResourceChanged.delta/new_value pair comes
// first, which this exact-sequence assertEnvelopes catches).
func TestResolveV2TagTeamOutcomeMergeOrder(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 5, "vigor": 0}, nil)
	putActor(st, "b", map[string]int32{"brace": 10}, map[string]*vttv1.Resource{"focus": res(0, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	// clash-roll's edge=0 for tag-team (unlike quick-jab's edge=2): 1d20(10) + vim(5) + 0 = 15.
	envs, err := rules.Resolve(rs, st, useAbility("a", "tag-team", "b"), &queueRoller{queue: []int{10}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "tag-team", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.vim + (0)", Results: []int32{10}, Total: 15}},
			OutcomeSummary: "Tag Team on b: connect (15 vs 10)",
		}),
		envResourceChanged("b", "focus", 1, 1, "ability:tag-team:connect"), // tag-q
		envResourceChanged("b", "focus", 2, 3, "ability:tag-team:connect"), // tag-p, running on top of tag-q's +1
		envConditionApplied("b", "winded", "threshold:focus"),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2WardShiftPassRunsEffectsOnly proves BranchOutcomes AND
// Effects both run together, and that Effects' remove_condition is
// idempotent exactly like v1's (TestResolveRemoveConditionIdempotentNoOp)
// even when reached via a v2 composition's "always" outcome: ward-shift's
// "pass" branch (BranchOutcomes[0]) is nil — nothing contributed to it —
// so the only event besides AbilityUsed is Effects' unconditional
// remove_condition(steadied), which fires because the target actually
// carries "steadied" here.
func TestResolveV2WardShiftPassRunsEffectsOnly(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 3}, nil)
	putActor(st, "b", map[string]int32{"brace": 5}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 2, 0) // ward-shift's range is 2
	st.Conditions["b"] = []engine.ActorCondition{{ID: "steadied", Source: "test", AppliedSeq: 1}}

	// 1d20(15) + vim(3) = 18 >= brace(5) -> "pass".
	envs, err := rules.Resolve(rs, st, useAbility("a", "ward-shift", "b"), &queueRoller{queue: []int{15}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "ward-shift", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.vim", Results: []int32{15}, Total: 18}},
			OutcomeSummary: "Ward Shift on b: pass (18 vs 5)",
		}),
		envConditionRemoved("b", "steadied", "ability:ward-shift:effect"),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2WardShiftFailAppliesAndNoOpRemoves covers ward-shift's
// "fail" branch: BranchOutcomes[1] applies "winded" then attempts to
// remove "steadied" (idempotent no-op — the target never had it), and
// Effects' own unconditional remove_condition(steadied) is a SECOND,
// independently idempotent no-op right after — proving two DIFFERENT
// no-op remove_condition outcomes in the SAME resolve, from two DIFFERENT
// atoms (ward-mark's branch-outcome remove, ward-clear's always-effect
// remove), both correctly emit nothing.
func TestResolveV2WardShiftFailAppliesAndNoOpRemoves(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 3}, nil)
	putActor(st, "b", map[string]int32{"brace": 10}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 2, 0)
	// "b" does NOT carry "steadied" here.

	// 1d20(2) + vim(3) = 5 < brace(10) -> "fail".
	envs, err := rules.Resolve(rs, st, useAbility("a", "ward-shift", "b"), &queueRoller{queue: []int{2}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "ward-shift", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.vim", Results: []int32{2}, Total: 5}},
			OutcomeSummary: "Ward Shift on b: fail (5 vs 10)",
		}),
		envConditionApplied("b", "winded", "ability:ward-shift:fail"),
		// ward-mark's remove_condition(steadied) and ward-clear's
		// unconditional remove_condition(steadied) are BOTH no-ops here
		// (b never had "steadied") — neither emits an event.
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2WardShiftMultiTarget exercises ward-shift's max_targets=2
// (wide-delivery's "{max}" placeholder, compiled to a literal 2 —
// TestLoadValidV2Fixture's own pin) against two targets in one call,
// proving v2's targeting compiles into a genuinely usable multi-target
// bound, not just a value that happens to round-trip through
// diffCompiledPower.
func TestResolveV2WardShiftMultiTarget(t *testing.T) {
	rs := loadV2Fixture(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 3}, nil)
	putActor(st, "b", map[string]int32{"brace": 30}, nil) // will fail
	putActor(st, "c", map[string]int32{"brace": 1}, nil)  // will pass
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 2, 0)
	putToken(st, "tc", "s1", "c", -2, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "ward-shift", "b", "c"), &queueRoller{queue: []int{2, 15}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	au, ok := envs[0].Payload.(*vttv1.Envelope_AbilityUsed)
	if !ok {
		t.Fatalf("envs[0] = %v, want AbilityUsed", envs[0])
	}
	if got := au.AbilityUsed.TargetIds; len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Errorf("AbilityUsed.TargetIds = %v, want [b c]", got)
	}
	wantSummary := "Ward Shift on b: fail (5 vs 30); Ward Shift on c: pass (18 vs 1)"
	if au.AbilityUsed.OutcomeSummary != wantSummary {
		t.Errorf("OutcomeSummary = %q, want %q", au.AbilityUsed.OutcomeSummary, wantSummary)
	}
	// b (fail) gets ConditionApplied(winded); c (pass) gets nothing from
	// BranchOutcomes and no "steadied" to remove via Effects.
	var applied []string
	for _, e := range envs[1:] {
		if p, ok := e.Payload.(*vttv1.Envelope_ConditionApplied); ok {
			applied = append(applied, p.ConditionApplied.ActorId)
		}
	}
	if len(applied) != 1 || applied[0] != "b" {
		t.Errorf("ConditionApplied actors = %v, want [b]", applied)
	}
}

// --- second two-actor outcome-context pin (finding R5) ---

// drainCasterCtxRuleset hand-builds a Ruleset with one CompiledPower whose
// HIT outcome delta references @caster.<attr> where BOTH actors carry that
// attr under DISTINCT values — the asymmetric-statblock shape that makes the
// caster<->target swap in applyOutcomes' EvalContext pair (resolve.go)
// observable. TestResolveV2QuickJabConnect is the only OTHER pin of that
// pair; this is the independent second pin R5 asks for (its outcome
// references @caster, not a self-targeted or ref-free delta), so the swap
// mutation is caught by >= 2 tests.
func drainCasterCtxRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	drain := &rules.CompiledPower{
		ID: "drain", Name: "Drain",
		Usage:     rules.Usage{AtWill: true},
		Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
		Resolution: &rules.CompiledResolution{
			Roll: mustParse(t, "1d20 + @caster.might"), RollSrc: "1d20 + @caster.might",
			Vs: mustParse(t, "@target.guard"), VsSrc: "@target.guard",
			Branches: [2]string{"hit", "miss"},
		},
		BranchOutcomes: [2][]rules.Outcome{
			{
				// delta = 0 - @caster.might: reads the CASTER's might, applied
				// to the TARGET's pool. caster might=7, target might=1, so the
				// swap changes the delta from -7 to -1 — caught by the exact
				// batch below.
				{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
					Resource: "pool", DeltaExpr: mustParse(t, "0 - @caster.might"), DeltaExprSrc: "0 - @caster.might",
				}},
			},
			nil,
		},
	}
	return &rules.Ruleset{
		ID: "drain-fixture", Name: "Drain Fixture",
		Attributes: []string{"might"},
		Defenses:   []string{"guard"},
		Resources:  []rules.ResourceDef{{Name: "pool"}},
		Conditions: map[string]*rules.Condition{},
		Compiled:   map[string]*rules.CompiledPower{"drain": drain},
	}
}

// TestResolveV2OutcomeCasterContextAsymmetric pins that a v2 outcome
// delta_expr's @caster ref resolves against the CASTER (not the target),
// using distinct caster/target values for the SAME attribute name so the
// two-actor swap mutation is observable here independently of
// TestResolveV2QuickJabConnect.
func TestResolveV2OutcomeCasterContextAsymmetric(t *testing.T) {
	rs := drainCasterCtxRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"might": 7}, nil)
	putActor(st, "b", map[string]int32{"might": 1, "guard": 5}, map[string]*vttv1.Resource{"pool": res(10, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	// roll: 1d20 -> 10, + caster might(7) = 17 >= guard(5) -> hit.
	// hit delta = 0 - caster.might(7) = -7; target pool 10 -> 3.
	envs, err := rules.Resolve(rs, st, useAbility("a", "drain", "b"), &queueRoller{queue: []int{10}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "drain", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.might", Results: []int32{10}, Total: 17}},
			OutcomeSummary: "Drain on b: hit (17 vs 5)",
		}),
		envResourceChanged("b", "pool", -7, 3, "ability:drain:hit"),
	}
	assertEnvelopes(t, envs, want)
}

// --- vs-expression-with-dice (hand-built CompiledPower — see this file's
// top doc comment for why this is not added to testdata/valid-v2) ---

// vsDiceFixtureRuleset hand-builds a *rules.Ruleset carrying ONE
// CompiledPower directly (rules.CompiledPower/CompiledResolution — the
// same "hand-built via exported format.go types, not through Load" idiom
// resolve_test.go's own fixtureRuleset already uses for v1), whose Vs
// expression ("@target.brace + 1d4") actually rolls dice — the one shape
// none of testdata/valid-v2's four abilities exercise (their Vs is always
// a bare "@target.brace").
func vsDiceFixtureRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	feint := &rules.CompiledPower{
		ID: "feint", Name: "Feint",
		Usage:     rules.Usage{AtWill: true},
		Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
		Resolution: &rules.CompiledResolution{
			Roll: mustParse(t, "1d20 + @caster.vim"), RollSrc: "1d20 + @caster.vim",
			Vs: mustParse(t, "@target.brace + 1d4"), VsSrc: "@target.brace + 1d4",
			Branches: [2]string{"connect", "graze"},
		},
		BranchOutcomes: [2][]rules.Outcome{
			{
				{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
					Resource: "focus", DeltaExpr: mustParse(t, "1"), DeltaExprSrc: "1",
				}},
			},
			nil,
		},
	}
	return &rules.Ruleset{
		ID: "vsdice-fixture", Name: "Vs-Dice Fixture",
		Attributes: []string{"vim"},
		Defenses:   []string{"brace"},
		Resources:  []rules.ResourceDef{{Name: "focus"}},
		Conditions: map[string]*rules.Condition{},
		Compiled:   map[string]*rules.CompiledPower{"feint": feint},
	}
}

// TestResolveV2VsExpressionWithDiceRecordsInOrder pins the binding's
// explicit "vs-expression-with-dice case... pin vs-dice recording order"
// requirement: Vs total depends on a die roll, that roll is recorded onto
// AbilityUsed.rolls as its OWN entry (Resolution.Roll's own roll comes
// FIRST, Vs's SECOND — "roll expr then vs expr" per Resolve's doc comment
// and the task's binding ordering requirement), and the branch selected
// genuinely depends on the vs roll's result (not just the attack roll's).
func TestResolveV2VsExpressionWithDiceRecordsInOrder(t *testing.T) {
	rs := vsDiceFixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 2}, nil)
	putActor(st, "b", map[string]int32{"brace": 5}, map[string]*vttv1.Resource{"focus": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	// roll: 1d20 -> 10, total 10+2=12. vs: brace(5) + 1d4 -> 3, total 8.
	// 12 >= 8 -> connect.
	envs, err := rules.Resolve(rs, st, useAbility("a", "feint", "b"), &queueRoller{queue: []int{10, 3}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "feint", TargetIds: []string{"b"},
			Rolls: []*vttv1.AbilityUsed_Roll{
				{Expression: "1d20 + @caster.vim", Results: []int32{10}, Total: 12},
				{Expression: "@target.brace + 1d4", Results: []int32{3}, Total: 8},
			},
			OutcomeSummary: "Feint on b: connect (12 vs 8)",
		}),
		envResourceChanged("b", "focus", 1, 1, "ability:feint:connect"),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveV2VsExpressionWithDiceGraze proves the branch genuinely
// depends on the ROLLED vs total, not merely the static "brace" part of
// it: an attack roll total that would have beaten the bare defense (6 >=
// 5) instead loses once the vs roll pushes the defense's total higher (6
// < 9) — and, since "graze" contributes nothing (BranchOutcomes[1] is
// nil), no ResourceChanged follows.
func TestResolveV2VsExpressionWithDiceGraze(t *testing.T) {
	rs := vsDiceFixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"vim": 2}, nil)
	putActor(st, "b", map[string]int32{"brace": 5}, map[string]*vttv1.Resource{"focus": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	// roll: 1d20 -> 4, total 4+2=6. vs: brace(5) + 1d4 -> 4, total 9. 6 < 9 -> graze.
	envs, err := rules.Resolve(rs, st, useAbility("a", "feint", "b"), &queueRoller{queue: []int{4, 4}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "feint", TargetIds: []string{"b"},
			Rolls: []*vttv1.AbilityUsed_Roll{
				{Expression: "1d20 + @caster.vim", Results: []int32{4}, Total: 6},
				{Expression: "@target.brace + 1d4", Results: []int32{4}, Total: 9},
			},
			OutcomeSummary: "Feint on b: graze (6 vs 9)",
		}),
	}
	assertEnvelopes(t, envs, want)
}
