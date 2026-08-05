package rules_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// int32Ruleset is a minimal fixture for the F2 int32-parity tests: a single
// uncapped resource plus two abilities that drive an out-of-int32 value out
// of a legal, loadable expression.
func int32Ruleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	return &rules.Ruleset{
		ID: "int32-fixture", Name: "Int32 Fixture",
		Attributes: []string{"brawn"},
		Defenses:   []string{"guard"},
		Resources:  []rules.ResourceDef{{Name: "vigor"}},
		Conditions: map[string]*rules.Condition{},
		Compiled:   int32FixtureCompiled(t),
	}
}

// int32FixtureCompiled builds the int32-parity abilities directly as
// CompiledPowers (the shape Resolve executes) — no intermediate ability/
// adapter.
func int32FixtureCompiled(t *testing.T) map[string]*rules.CompiledPower {
	t.Helper()
	rc := func(src string) rules.Outcome {
		return rules.Outcome{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
			Resource: "vigor", DeltaExpr: mustParse(t, src), DeltaExprSrc: src,
		}}
	}
	return map[string]*rules.CompiledPower{
		"overload": {
			ID: "overload", Name: "Overload", Usage: rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
			Effects:   []rules.Outcome{rc("2000000 * 2000")},
		},
		// bigswing: an attack whose roll total overflows int32 when the
		// caster's brawn attribute (an int32) sits near its max.
		"bigswing": {
			ID: "bigswing", Name: "Big Swing", Usage: rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
			Resolution: &rules.CompiledResolution{
				Roll: mustParse(t, "1d20 + @caster.brawn"), RollSrc: "1d20 + @brawn",
				Vs: mustParse(t, "@target.guard"), VsSrc: "@target.guard",
				Branches: [2]string{"hit", "miss"},
			},
			BranchOutcomes: [2][]rules.Outcome{{rc("1")}, nil},
		},
		// exactly-max / exactly-min: deltas landing EXACTLY on the int32
		// bounds, which must be ACCEPTED. Every other ability here overflows,
		// so the range was only ever tested from outside — int32Checked's two
		// comparisons (`v < MinInt32`, `v > MaxInt32`) differ from `<=`/`>=`
		// at precisely these two values and nowhere else.
		"exactly-max": {
			ID: "exactly-max", Name: "Exactly Max", Usage: rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
			Effects:   []rules.Outcome{rc("2147483647")},
		},
		"exactly-min": {
			ID: "exactly-min", Name: "Exactly Min", Usage: rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
			// The grammar has no unary minus, so the literal is reached by
			// subtraction.
			Effects: []rules.Outcome{rc("0 - 2147483648")},
		},
		// creep: a self-effect whose delta itself fits int32 (2e9) but
		// whose new_value (current 2e9 + 2e9) does not, on an uncapped
		// resource — isolates the new_value bound from the delta bound.
		"creep": {
			ID: "creep", Name: "Creep", Usage: rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
			Effects:   []rules.Outcome{rc("2000000000")},
		},
	}
}

// TestResolveRejectsOverflowDelta is the F2a repro: a loadable, cross-valid
// delta_expr ('2000000 * 2000' = 4e9) must be rejected at Resolve time with a
// clean rules error — NOT emitted as a truncated ResourceChanged the engine
// then rejects with a misleading integrity error.
func TestResolveRejectsOverflowDelta(t *testing.T) {
	rs := int32Ruleset(t)
	st := newTestState()
	// Mirror the repro exactly: vigor current=10, max=20 (delta truncation
	// wrapped to a negative that clamped to the max pre-fix).
	putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"vigor": res(10, 20)})
	putToken(st, "ta", "s1", "a", 0, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "overload", "a"), &queueRoller{})
	wantErr(t, err, "int32")
	wantNoEvents(t, envs)
}

// TestResolveRejectsOverflowNewValue isolates the new_value bound: the delta
// fits int32 but the resulting value on an uncapped resource does not.
func TestResolveRejectsOverflowNewValue(t *testing.T) {
	rs := int32Ruleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"vigor": res(2000000000, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "creep", "a"), &queueRoller{})
	wantErr(t, err, "int32")
	wantNoEvents(t, envs)
}

// TestResolveRejectsOverflowRollTotal covers the roll-total bound: an attack
// roll whose total exceeds int32 (a near-max int32 attribute plus a die) must
// be rejected cleanly rather than recording truncated Total testimony.
func TestResolveRejectsOverflowRollTotal(t *testing.T) {
	rs := int32Ruleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 2147483647}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "bigswing", "b"), &queueRoller{queue: []int{15}})
	wantErr(t, err, "int32")
	wantNoEvents(t, envs)
}

// vsOverflowRuleset hand-builds a Ruleset with one CompiledPower whose Vs
// expression rolls dice and overflows int32 — the one int32-bounded position
// (resolve.go's vs-total check) no golden or other resolve test drives. A
// v2 Vs is a full two-actor expression evaluated per target (Task 3), so it
// can overflow exactly like a roll or a delta; this fixture is the pin that
// makes dropping the vs-total int32 check a caught mutation.
func vsOverflowRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	bigvs := &rules.CompiledPower{
		ID: "bigvs", Name: "Big Vs",
		Usage:     rules.Usage{AtWill: true},
		Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
		Resolution: &rules.CompiledResolution{
			Roll: mustParse(t, "1d20 + @caster.brawn"), RollSrc: "1d20 + @caster.brawn",
			// 1000000000 * 1d4 overflows int32 for any die face >= 3.
			Vs: mustParse(t, "1000000000 * 1d4"), VsSrc: "1000000000 * 1d4",
			Branches: [2]string{"hit", "miss"},
		},
		BranchOutcomes: [2][]rules.Outcome{nil, nil},
	}
	return &rules.Ruleset{
		ID: "vsoverflow-fixture", Name: "Vs Overflow Fixture",
		Attributes: []string{"brawn"},
		Defenses:   []string{"guard"},
		Resources:  []rules.ResourceDef{{Name: "vigor"}},
		Conditions: map[string]*rules.Condition{},
		Compiled:   map[string]*rules.CompiledPower{"bigvs": bigvs},
	}
}

// TestResolveRejectsOverflowVsTotal is the vs-total int32 pin (pre-authorized
// item 2): a Vs expression that evaluates past int32 must be rejected with a
// clean rules error naming the vs total, NOT truncated into a bogus branch
// comparison. The die roll happens SECOND (roll first, then vs — Resolve's
// recording order), so the queue is [roll, vs]: 1d20 -> 10 (roll total 12),
// then 1d4 -> 3 makes the vs total 3e9, past int32.
func TestResolveRejectsOverflowVsTotal(t *testing.T) {
	rs := vsOverflowRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 2}, nil)
	putActor(st, "b", map[string]int32{"guard": 5}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "bigvs", "b"), &queueRoller{queue: []int{10, 3}})
	wantErr(t, err, "int32")
	wantErr(t, err, "vs total") // pins that it is the VS bound specifically, not the roll or a delta
	wantNoEvents(t, envs)
}

// TestResolveAcceptsDeltasExactlyOnTheInt32Bounds pins int32Checked's range as
// INCLUSIVE (resolve.go:626).
//
// Every other test in this file drives a value OUTSIDE the range and asserts
// the rejection, so both comparisons were only ever tested from one side —
// `v < MinInt32` and `v <= MinInt32` agree everywhere except at MinInt32
// itself, and likewise at the top. Both CONDITIONALS_BOUNDARY mutants survived
// the whole suite.
//
// Under either mutant a delta of exactly ±2147483647/8 — a legal int32, the
// widest a resource change can be — is refused as "outside the int32 wire
// range", which is the opposite of true.
func TestResolveAcceptsDeltasExactlyOnTheInt32Bounds(t *testing.T) {
	for _, tc := range []struct {
		ability string
		current int32
		want    int32
	}{
		// current 0 so new_value equals the delta and stays in range too.
		{"exactly-max", 0, 2147483647},
		{"exactly-min", 0, -2147483648},
	} {
		t.Run(tc.ability, func(t *testing.T) {
			rs := int32Ruleset(t)
			st := newTestState()
			putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"vigor": res(tc.current, 0)})
			putToken(st, "ta", "s1", "a", 0, 0)

			envs, err := rules.Resolve(rs, st, useAbility("a", tc.ability, "a"), &queueRoller{})
			if err != nil {
				t.Fatalf("Resolve(%s) = %v, want it accepted — %d is a legal int32", tc.ability, err, tc.want)
			}
			var got int32
			var saw bool
			for _, e := range envs {
				if rc := e.GetResourceChanged(); rc != nil {
					got, saw = rc.GetDelta(), true
				}
			}
			if !saw {
				t.Fatalf("Resolve(%s) produced no ResourceChanged", tc.ability)
			}
			if got != tc.want {
				t.Errorf("delta = %d, want %d — the bound value must survive the conversion intact", got, tc.want)
			}
		})
	}
}
