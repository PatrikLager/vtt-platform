package rules_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// --- fixture ruleset (hand-built via exported format.go types, not through
// Load — Resolve takes an already-loaded *rules.Ruleset, so these tests
// exercise it directly without depending on load.go's cross-validation
// having run; testdata/valid stays load.go's own fixture) ---
//
// Resources: vigor (threshold: nonzero -> winded, removeWhenFalse) / focus
// (plain usage-cost pool, no threshold) / steam (threshold: nonzero ->
// heated, NOT removeWhenFalse — proves a threshold without the flag holds
// its condition once applied, even after "when" goes false again).
// Abilities: jab (attack, hit raises vigor by 1 — the ONLY way this fixture
// raises a threshold-gated resource, so it doubles as the apply-direction
// case) / haymaker (attack, hit lowers vigor by 1d6 — dice-bearing damage,
// and the remove-direction + clamp-floor case) / vent (attack, hit lowers
// steam by a flat 2 — the removeWhenFalse=false negative case) / brace
// (limited-use self-effect, applies braced) / unbrace (at-will self-effect,
// removes braced) / sweep (attack, max_targets 2, hit raises vigor by 1 —
// the multi-target ordering case).

// compileFixtureAbilities adapts every hand-built v1 Ability in abilities
// into a Ruleset.Compiled map via the EXACT SAME adapter Load uses for a
// real format-v1 ruleset (rules.AdaptV1Ability — load.go, Task 3). Resolve
// now executes CompiledPower exclusively, through ONE code path, for
// every format version (spec 5c pillar) — so every fixture in this
// package that builds a *rules.Ruleset by hand, bypassing Load entirely
// (this file's own doc comment: "Resolve takes an already-loaded
// *rules.Ruleset, so these tests exercise it directly without depending
// on load.go's cross-validation having run"), must populate Compiled
// itself, with the identical logic a real Load would have used. Shared by
// every resolve_*_test.go fixture in this package.
func compileFixtureAbilities(t *testing.T, abilities map[string]*rules.Ability) map[string]*rules.CompiledPower {
	t.Helper()
	out := make(map[string]*rules.CompiledPower, len(abilities))
	for id, a := range abilities {
		cp, err := rules.AdaptV1Ability(a)
		if err != nil {
			t.Fatalf("AdaptV1Ability(%q): unexpected error: %v", id, err)
		}
		out[id] = cp
	}
	return out
}

func fixtureRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	abilities := fixtureAbilities(t)
	return &rules.Ruleset{
		ID:         "resolve-fixture",
		Name:       "Resolve Fixture",
		Attributes: []string{"brawn"},
		Defenses:   []string{"guard"},
		Resources: []rules.ResourceDef{
			{
				Name: "vigor",
				Thresholds: []rules.Threshold{
					{When: mustParse(t, "#vigor"), WhenSrc: "#vigor", ApplyCondition: "winded", RemoveWhenFalse: true},
				},
			},
			{Name: "focus"},
			{
				Name: "steam",
				Thresholds: []rules.Threshold{
					{When: mustParse(t, "#steam"), WhenSrc: "#steam", ApplyCondition: "heated", RemoveWhenFalse: false},
				},
			},
		},
		Conditions: map[string]*rules.Condition{
			"winded": {ID: "winded", Name: "Winded"},
			"braced": {ID: "braced", Name: "Braced"},
			"heated": {ID: "heated", Name: "Heated"},
		},
		Abilities: abilities,
		Compiled:  compileFixtureAbilities(t, abilities),
	}
}

func fixtureAbilities(t *testing.T) map[string]*rules.Ability {
	t.Helper()
	return map[string]*rules.Ability{
		"jab": {
			ID: "jab", Name: "Jab",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
			Attack:    &rules.Attack{Roll: mustParse(t, "1d20 + @brawn"), RollSrc: "1d20 + @brawn", Vs: "guard"},
			Hit: []rules.Outcome{
				{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
					Resource: "vigor", DeltaExpr: mustParse(t, "1"), DeltaExprSrc: "1",
				}},
			},
		},
		"haymaker": {
			ID: "haymaker", Name: "Haymaker",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
			Attack:    &rules.Attack{Roll: mustParse(t, "1d20 + @brawn"), RollSrc: "1d20 + @brawn", Vs: "guard"},
			Hit: []rules.Outcome{
				{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
					Resource: "vigor", DeltaExpr: mustParse(t, "0 - 1d6"), DeltaExprSrc: "0 - 1d6",
				}},
			},
		},
		"vent": {
			ID: "vent", Name: "Vent",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 1},
			Attack:    &rules.Attack{Roll: mustParse(t, "1d20 + @brawn"), RollSrc: "1d20 + @brawn", Vs: "guard"},
			Hit: []rules.Outcome{
				{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
					Resource: "steam", DeltaExpr: mustParse(t, "0 - 2"), DeltaExprSrc: "0 - 2",
				}},
			},
		},
		"brace": {
			ID: "brace", Name: "Brace",
			Usage:     rules.Usage{Limited: &rules.LimitedUsage{Resource: "focus", Cost: 1}},
			Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
			Effect: []rules.Outcome{
				{Kind: rules.OutcomeApplyCondition, ApplyCondition: &rules.ApplyConditionOutcome{ID: "braced"}},
			},
		},
		"unbrace": {
			ID: "unbrace", Name: "Unbrace",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 0, MaxTargets: 1},
			Effect: []rules.Outcome{
				{Kind: rules.OutcomeRemoveCondition, RemoveCondition: &rules.RemoveConditionOutcome{ID: "braced"}},
			},
		},
		"sweep": {
			ID: "sweep", Name: "Sweep",
			Usage:     rules.Usage{AtWill: true},
			Targeting: rules.Targeting{Range: 1, MaxTargets: 2},
			Attack:    &rules.Attack{Roll: mustParse(t, "1d20 + @brawn"), RollSrc: "1d20 + @brawn", Vs: "guard"},
			Hit: []rules.Outcome{
				{Kind: rules.OutcomeResourceChange, ResourceChange: &rules.ResourceChangeOutcome{
					Resource: "vigor", DeltaExpr: mustParse(t, "1"), DeltaExprSrc: "1",
				}},
			},
		},
	}
}

// --- state-building helpers ---

func newTestState() *engine.State {
	st := engine.NewState()
	st.Scenes["s1"] = engine.Scene{ID: "s1", Name: "s1", GridWidth: 20, GridHeight: 20}
	return st
}

func res(current, max int32) *vttv1.Resource { return &vttv1.Resource{Current: current, Max: max} }

func putActor(st *engine.State, id string, attrs map[string]int32, resources map[string]*vttv1.Resource) {
	st.Actors[id] = &vttv1.Actor{ActorId: id, Name: id, Attributes: attrs, Resources: resources}
}

func putToken(st *engine.State, tokenID, sceneID, actorID string, x, y int32) {
	st.Tokens[tokenID] = engine.Token{ID: tokenID, SceneID: sceneID, ActorID: actorID, X: x, Y: y}
}

func useAbility(actorID, abilityID string, targetIDs ...string) *vttv1.UseAbility {
	return &vttv1.UseAbility{ActorId: actorID, AbilityId: abilityID, TargetIds: targetIDs}
}

func wantErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Resolve: want error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("Resolve error = %q, want it to contain %q", err.Error(), substr)
	}
}

func wantNoEvents(t *testing.T, envs []*vttv1.Envelope) {
	t.Helper()
	if envs != nil {
		t.Fatalf("Resolve: want nil events on error, got %v", envs)
	}
}

func assertEnvelopes(t *testing.T, got, want []*vttv1.Envelope) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Resolve: got %d envelopes, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("envelope[%d]:\n got  %v\n want %v", i, got[i], want[i])
		}
	}
}

func envAbilityUsed(au *vttv1.AbilityUsed) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityUsed{AbilityUsed: au}}
}
func envResourceChanged(actorID, resource string, delta, newValue int32, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actorID, Resource: resource, Delta: delta, NewValue: newValue, Reason: reason,
	}}}
}
func envConditionApplied(actorID, conditionID, source string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actorID, ConditionId: conditionID, Source: source,
	}}}
}
func envConditionRemoved(actorID, conditionID, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
		ActorId: actorID, ConditionId: conditionID, Reason: reason,
	}}}
}

// --- validation tests ---

func TestResolveUnknownAbility(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	envs, err := rules.Resolve(rs, st, useAbility("a", "no-such-ability", "a"), &queueRoller{})
	wantErr(t, err, `unknown ability "no-such-ability"`)
	wantNoEvents(t, envs)
}

func TestResolveUnknownActor(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	envs, err := rules.Resolve(rs, st, useAbility("ghost", "jab", "ghost"), &queueRoller{})
	wantErr(t, err, `unknown actor "ghost"`)
	wantNoEvents(t, envs)
}

func TestResolveUnknownTarget(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "ghost"), &queueRoller{})
	wantErr(t, err, `unknown target actor "ghost"`)
	wantNoEvents(t, envs)
}

func TestResolveNoTargets(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab"), &queueRoller{})
	wantErr(t, err, "at least one target")
	wantNoEvents(t, envs)
}

func TestResolveTooManyTargets(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, nil)
	putActor(st, "c", map[string]int32{"guard": 10}, nil)
	// jab's max_targets is 1.
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b", "c"), &queueRoller{})
	wantErr(t, err, "allows at most 1 target")
	wantNoEvents(t, envs)
}

func TestResolveMissingAttackAttribute(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{}, nil) // no "brawn"
	putActor(st, "b", map[string]int32{"guard": 10}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{})
	wantErr(t, err, `missing attribute "brawn"`)
	wantNoEvents(t, envs)
}

func TestResolveMissingDefenseStat(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{}, nil) // no "guard"
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{})
	wantErr(t, err, `missing defense "guard"`)
	wantNoEvents(t, envs)
}

func TestResolveUsageResourceMissing(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil) // no "focus" resource at all
	putToken(st, "ta", "s1", "a", 0, 0)
	envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "a"), &queueRoller{})
	wantErr(t, err, `has no resource "focus"`)
	wantNoEvents(t, envs)
}

// TestResolveUsageInsufficientResource is one of the two required rejection
// goldens (spec: usage-exhausted rejection).
func TestResolveUsageInsufficientResource(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"focus": res(0, 3)})
	putToken(st, "ta", "s1", "a", 0, 0)
	envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "a"), &queueRoller{})
	wantErr(t, err, `insufficient "focus"`)
	wantNoEvents(t, envs)
}

func TestResolveNoToken(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, nil)
	// no tokens placed at all
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{})
	wantErr(t, err, `actor "a" has no token placed`)
	wantNoEvents(t, envs)
}

func TestResolveTargetNoTokenInScene(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	st.Scenes["s2"] = engine.Scene{ID: "s2", Name: "s2", GridWidth: 20, GridHeight: 20}
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s2", "b", 0, 0) // different scene
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{})
	wantErr(t, err, `target actor "b" has no token in the active scene`)
	wantNoEvents(t, envs)
}

// TestResolveOutOfRange is the other required rejection golden.
func TestResolveOutOfRange(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 2, 0) // distance 2 > jab's range 1
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{})
	wantErr(t, err, "out of range")
	wantNoEvents(t, envs)
}

// TestResolveRangeZeroAcceptsSameSquareOnly pins the binding range-0
// semantics: distance <= range, so range 0 accepts only a target token
// occupying the exact same grid cell as the caster's — an adjacent (but not
// coincident) token is still out of range.
func TestResolveRangeZeroAcceptsSameSquareOnly(t *testing.T) {
	rs := fixtureRuleset(t)

	t.Run("same square (self) accepted", func(t *testing.T) {
		st := newTestState()
		putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"focus": res(1, 1)})
		putToken(st, "ta", "s1", "a", 5, 5)
		envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "a"), &queueRoller{})
		if err != nil {
			t.Fatalf("Resolve: unexpected error: %v", err)
		}
		if len(envs) != 3 { // AbilityUsed + usage ResourceChanged + ConditionApplied
			t.Fatalf("len(envs) = %d, want 3: %v", len(envs), envs)
		}
	})

	t.Run("adjacent (distance 1) rejected", func(t *testing.T) {
		st := newTestState()
		putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"focus": res(1, 1)})
		putActor(st, "b", map[string]int32{}, nil)
		putToken(st, "ta", "s1", "a", 5, 5)
		putToken(st, "tb", "s1", "b", 6, 5) // distance 1
		envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "b"), &queueRoller{})
		wantErr(t, err, "out of range")
		wantNoEvents(t, envs)
	})
}

func TestResolveResourceChangeUnknownResourceOnActor(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 5}, nil) // no "vigor" resource on b
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{queue: []int{15}})
	wantErr(t, err, `has no resource "vigor"`)
	wantNoEvents(t, envs)
}

// --- execution / golden-shaped tests ---

func TestResolveHitFullEnvelope(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	roller := &queueRoller{queue: []int{15}} // 1d20 -> 15; total 15+3=18 >= 10 -> hit
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), roller)
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}

	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "jab", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @brawn", Results: []int32{15}, Total: 18}},
			OutcomeSummary: "Jab on b: hit (18 vs 10)",
		}),
		envResourceChanged("b", "vigor", 1, 1, "ability:jab:hit"),
		envConditionApplied("b", "winded", "threshold:vigor"),
	}
	assertEnvelopes(t, envs, want)
}

func TestResolveMissFullEnvelope(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 30}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	roller := &queueRoller{queue: []int{2}} // 1d20 -> 2; total 2+3=5 < 30 -> miss
	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), roller)
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}

	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "jab", TargetIds: []string{"b"},
			Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @brawn", Results: []int32{2}, Total: 5}},
			OutcomeSummary: "Jab on b: miss (5 vs 30)",
		}),
		// jab's miss list is empty: no ResourceChanged, no threshold events.
	}
	assertEnvelopes(t, envs, want)
}

func TestResolveNonAttackEffectAppliesCondition(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"focus": res(2, 2)})
	putToken(st, "ta", "s1", "a", 0, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "a"), &queueRoller{})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "brace", TargetIds: []string{"a"},
			OutcomeSummary: "Brace on a",
		}),
		envResourceChanged("a", "focus", -1, 1, "ability:brace:usage"),
		envConditionApplied("a", "braced", "ability:brace:effect"),
	}
	assertEnvelopes(t, envs, want)
}

func TestResolveNonAttackEffectRemovesCondition(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	st.Conditions["a"] = []engine.ActorCondition{{ID: "braced", Source: "test", AppliedSeq: 1}}

	envs, err := rules.Resolve(rs, st, useAbility("a", "unbrace", "a"), &queueRoller{})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "unbrace", TargetIds: []string{"a"},
			OutcomeSummary: "Unbrace on a",
		}),
		envConditionRemoved("a", "braced", "ability:unbrace:effect"),
	}
	assertEnvelopes(t, envs, want)
}

func TestResolveApplyConditionIdempotentNoOp(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"focus": res(2, 2)})
	putToken(st, "ta", "s1", "a", 0, 0)
	st.Conditions["a"] = []engine.ActorCondition{{ID: "braced", Source: "test", AppliedSeq: 1}}

	envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "a"), &queueRoller{})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	// braced is already present: the usage cost still spends (it always
	// does), but no second ConditionApplied is emitted.
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "brace", TargetIds: []string{"a"},
			OutcomeSummary: "Brace on a",
		}),
		envResourceChanged("a", "focus", -1, 1, "ability:brace:usage"),
	}
	assertEnvelopes(t, envs, want)
}

func TestResolveRemoveConditionIdempotentNoOp(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	// braced is NOT present.

	envs, err := rules.Resolve(rs, st, useAbility("a", "unbrace", "a"), &queueRoller{})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "unbrace", TargetIds: []string{"a"},
			OutcomeSummary: "Unbrace on a",
		}),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveThresholdAppliesConditionOnCross is the threshold apply-
// direction required golden: vigor crosses 0 -> nonzero on a hit, and
// "winded" (not previously present) is applied.
func TestResolveThresholdAppliesConditionOnCross(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{queue: []int{20}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	last := envs[len(envs)-1]
	ca, ok := last.Payload.(*vttv1.Envelope_ConditionApplied)
	if !ok {
		t.Fatalf("last event = %v, want ConditionApplied", last)
	}
	if ca.ConditionApplied.ActorId != "b" || ca.ConditionApplied.ConditionId != "winded" || ca.ConditionApplied.Source != "threshold:vigor" {
		t.Errorf("ConditionApplied = %+v, want {b winded threshold:vigor}", ca.ConditionApplied)
	}
}

// TestResolveThresholdRemovesConditionWhenFalse is the threshold remove-
// direction required golden: vigor (already nonzero, winded present) drops
// to exactly 0 on a hit, and remove_when_false removes winded.
func TestResolveThresholdRemovesConditionWhenFalse(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(3, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)
	st.Conditions["b"] = []engine.ActorCondition{{ID: "winded", Source: "test", AppliedSeq: 1}}

	// 1d20 -> 20 (hit), 1d6 -> 3: vigor 3 - 3 = 0.
	envs, err := rules.Resolve(rs, st, useAbility("a", "haymaker", "b"), &queueRoller{queue: []int{20, 3}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := []*vttv1.Envelope{
		envAbilityUsed(&vttv1.AbilityUsed{
			ActorId: "a", AbilityId: "haymaker", TargetIds: []string{"b"},
			Rolls: []*vttv1.AbilityUsed_Roll{
				{Expression: "1d20 + @brawn", Results: []int32{20}, Total: 23},
				{Expression: "0 - 1d6", Results: []int32{3}, Total: -3},
			},
			OutcomeSummary: "Haymaker on b: hit (23 vs 10)",
		}),
		envResourceChanged("b", "vigor", -3, 0, "ability:haymaker:hit"),
		envConditionRemoved("b", "winded", "threshold:vigor"),
	}
	assertEnvelopes(t, envs, want)
}

// TestResolveThresholdHoldsWithoutRemoveWhenFalse proves removeWhenFalse's
// absence is load-bearing: steam's threshold has no remove_when_false, so
// once "heated" is applied it survives steam returning to 0.
func TestResolveThresholdHoldsWithoutRemoveWhenFalse(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"steam": res(2, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)
	st.Conditions["b"] = []engine.ActorCondition{{ID: "heated", Source: "test", AppliedSeq: 1}}

	envs, err := rules.Resolve(rs, st, useAbility("a", "vent", "b"), &queueRoller{queue: []int{20}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	for _, e := range envs {
		if _, ok := e.Payload.(*vttv1.Envelope_ConditionRemoved); ok {
			t.Fatalf("unexpected ConditionRemoved in %v: removeWhenFalse is false, condition must hold", envs)
		}
	}
}

func TestResolveResourceClampFloorZero(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(2, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	// 1d20 -> 20 (hit), 1d6 -> 6: vigor 2 - 6 = -4, clamped to 0.
	envs, err := rules.Resolve(rs, st, useAbility("a", "haymaker", "b"), &queueRoller{queue: []int{20, 6}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	var rc *vttv1.ResourceChanged
	for _, e := range envs {
		if p, ok := e.Payload.(*vttv1.Envelope_ResourceChanged); ok {
			rc = p.ResourceChanged
		}
	}
	if rc == nil {
		t.Fatal("no ResourceChanged event found")
	}
	if rc.NewValue != 0 {
		t.Errorf("ResourceChanged.NewValue = %d, want 0 (clamped floor)", rc.NewValue)
	}
	if rc.Delta != -6 {
		t.Errorf("ResourceChanged.Delta = %d, want -6 (unclamped raw delta)", rc.Delta)
	}
}

func TestResolveResourceClampCapMax(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(2, 2)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{queue: []int{20}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	var rc *vttv1.ResourceChanged
	for _, e := range envs {
		if p, ok := e.Payload.(*vttv1.Envelope_ResourceChanged); ok {
			rc = p.ResourceChanged
		}
	}
	if rc == nil {
		t.Fatal("no ResourceChanged event found")
	}
	if rc.NewValue != 2 {
		t.Errorf("ResourceChanged.NewValue = %d, want 2 (clamped at max)", rc.NewValue)
	}
}

func TestResolveMultipleTargetsOrderPreserved(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putActor(st, "c", map[string]int32{"guard": 30}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)
	putToken(st, "tc", "s1", "c", -1, 0)

	// target order [c, b]; roll #1 (vs c) misses, roll #2 (vs b) hits.
	envs, err := rules.Resolve(rs, st, useAbility("a", "sweep", "c", "b"), &queueRoller{queue: []int{2, 20}})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	au, ok := envs[0].Payload.(*vttv1.Envelope_AbilityUsed)
	if !ok {
		t.Fatalf("envs[0] = %v, want AbilityUsed", envs[0])
	}
	if got := au.AbilityUsed.TargetIds; len(got) != 2 || got[0] != "c" || got[1] != "b" {
		t.Errorf("AbilityUsed.TargetIds = %v, want [c b]", got)
	}
	if len(au.AbilityUsed.Rolls) != 2 {
		t.Fatalf("len(Rolls) = %d, want 2", len(au.AbilityUsed.Rolls))
	}
	// Only b's ResourceChanged should appear (c missed, empty miss list).
	var changed []string
	for _, e := range envs[1:] {
		if p, ok := e.Payload.(*vttv1.Envelope_ResourceChanged); ok {
			changed = append(changed, p.ResourceChanged.ActorId)
		}
	}
	if len(changed) != 1 || changed[0] != "b" {
		t.Errorf("ResourceChanged actors = %v, want [b]", changed)
	}
}

func TestResolveEnvelopesOnlyPayloadSet(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putToken(st, "ta", "s1", "a", 0, 0)
	st.Conditions["a"] = []engine.ActorCondition{{ID: "braced", Source: "test", AppliedSeq: 1}}

	envs, err := rules.Resolve(rs, st, useAbility("a", "unbrace", "a"), &queueRoller{})
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	for i, e := range envs {
		if e.EventId != "" || e.Sequence != 0 || e.SessionId != "" || e.ActorRole != "" || e.ParticipantId != "" || e.OccurredAt != nil {
			t.Errorf("envs[%d]: only Payload should be set, got %+v", i, e)
		}
	}
}

func TestResolveDeterministicGivenRoller(t *testing.T) {
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	envs1, err1 := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{queue: []int{15}})
	envs2, err2 := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{queue: []int{15}})
	if err1 != nil || err2 != nil {
		t.Fatalf("Resolve: unexpected errors: %v / %v", err1, err2)
	}
	assertEnvelopes(t, envs2, envs1)
}

// TestResolveOutputFoldsCleanlyThroughEngine is the integration-boundary
// proof the task brief calls for: Resolve's own math is not enough — every
// emitted ResourceChanged.new_value must match what engine.Apply
// INDEPENDENTLY recomputes and verifies (apply.go rejects a mismatch), and
// every ConditionApplied/ConditionRemoved must fold without a duplicate/
// absent rejection. Runs each scenario's batch through engine.Apply in
// sequence against a real snapshot, exactly as campaign.AppendBatch would.
func TestResolveOutputFoldsCleanlyThroughEngine(t *testing.T) {
	rs := fixtureRuleset(t)

	fold := func(t *testing.T, st *engine.State, envs []*vttv1.Envelope) {
		t.Helper()
		snap := st.Snapshot()
		for i, env := range envs {
			env.Sequence = int64(i + 1)
			if err := engine.Apply(snap, env); err != nil {
				t.Fatalf("engine.Apply(envs[%d] = %v): unexpected error: %v", i, env, err)
			}
		}
	}

	t.Run("hit crosses threshold", func(t *testing.T) {
		st := newTestState()
		putActor(st, "a", map[string]int32{"brawn": 3}, nil)
		putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 0)})
		putToken(st, "ta", "s1", "a", 0, 0)
		putToken(st, "tb", "s1", "b", 1, 0)
		envs, err := rules.Resolve(rs, st, useAbility("a", "jab", "b"), &queueRoller{queue: []int{15}})
		if err != nil {
			t.Fatalf("Resolve: unexpected error: %v", err)
		}
		fold(t, st, envs)
	})

	t.Run("limited usage plus already-present condition no-op", func(t *testing.T) {
		st := newTestState()
		putActor(st, "a", map[string]int32{"brawn": 3}, map[string]*vttv1.Resource{"focus": res(2, 2)})
		putToken(st, "ta", "s1", "a", 0, 0)
		st.Conditions["a"] = []engine.ActorCondition{{ID: "braced", Source: "test", AppliedSeq: 1}}
		envs, err := rules.Resolve(rs, st, useAbility("a", "brace", "a"), &queueRoller{})
		if err != nil {
			t.Fatalf("Resolve: unexpected error: %v", err)
		}
		fold(t, st, envs)
	})

	t.Run("threshold remove_when_false", func(t *testing.T) {
		st := newTestState()
		putActor(st, "a", map[string]int32{"brawn": 3}, nil)
		putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(3, 0)})
		putToken(st, "ta", "s1", "a", 0, 0)
		putToken(st, "tb", "s1", "b", 1, 0)
		st.Conditions["b"] = []engine.ActorCondition{{ID: "winded", Source: "test", AppliedSeq: 1}}
		envs, err := rules.Resolve(rs, st, useAbility("a", "haymaker", "b"), &queueRoller{queue: []int{20, 3}})
		if err != nil {
			t.Fatalf("Resolve: unexpected error: %v", err)
		}
		fold(t, st, envs)
	})

	t.Run("multi-target sweep", func(t *testing.T) {
		st := newTestState()
		putActor(st, "a", map[string]int32{"brawn": 3}, nil)
		putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 2)})
		putActor(st, "c", map[string]int32{"guard": 30}, map[string]*vttv1.Resource{"vigor": res(0, 2)})
		putToken(st, "ta", "s1", "a", 0, 0)
		putToken(st, "tb", "s1", "b", 1, 0)
		putToken(st, "tc", "s1", "c", -1, 0)
		envs, err := rules.Resolve(rs, st, useAbility("a", "sweep", "c", "b"), &queueRoller{queue: []int{2, 20}})
		if err != nil {
			t.Fatalf("Resolve: unexpected error: %v", err)
		}
		fold(t, st, envs)
	})
}
