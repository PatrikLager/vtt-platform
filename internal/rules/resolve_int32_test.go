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
		Abilities: map[string]*rules.Ability{
			// overload: self-effect whose delta_expr evaluates to 4e9, past
			// int32 — the review's '2000000 * 2000' repro.
			"overload": {
				ID: "overload", Name: "Overload",
				Usage:     rules.Usage{AtWill: true},
				Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
				Effect: []rules.Outcome{
					{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
						Resource: "vigor", DeltaExpr: mustParse(t, "2000000 * 2000"), DeltaExprSrc: "2000000 * 2000",
					}},
				},
			},
			// bigswing: an attack whose roll total overflows int32 when the
			// caster's brawn attribute (an int32) sits near its max.
			"bigswing": {
				ID: "bigswing", Name: "Big Swing",
				Usage:     rules.Usage{AtWill: true},
				Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
				Attack:    &rules.Attack{Roll: mustParse(t, "1d20 + @brawn"), RollSrc: "1d20 + @brawn", Vs: "guard"},
				Hit: []rules.Outcome{
					{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
						Resource: "vigor", DeltaExpr: mustParse(t, "1"), DeltaExprSrc: "1",
					}},
				},
			},
			// creep: a self-effect whose delta itself fits int32 (2e9) but
			// whose new_value (current 2e9 + 2e9) does not, on an uncapped
			// resource — isolates the new_value bound from the delta bound.
			"creep": {
				ID: "creep", Name: "Creep",
				Usage:     rules.Usage{AtWill: true},
				Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
				Effect: []rules.Outcome{
					{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
						Resource: "vigor", DeltaExpr: mustParse(t, "2000000000"), DeltaExprSrc: "2000000000",
					}},
				},
			},
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
