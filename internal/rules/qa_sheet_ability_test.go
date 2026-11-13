package rules_test

import (
	"slices"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func qaAbLoadTavern(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load("../../rulesets/tavern-brawl")
	if err != nil {
		t.Fatalf("rules.Load(tavern-brawl): %v", err)
	}
	for _, id := range []string{"chair-swing", "fists", "sober-up", "splash-of-water"} {
		if rs.Compiled[id] == nil {
			t.Fatalf("fixture: tavern-brawl does not hold %q", id)
		}
	}
	return rs
}

func qaAbWantFault(t *testing.T, got *rules.SheetFault, key string) {
	t.Helper()
	if got == nil {
		t.Fatalf("SheetAbilityFault = nil, want a fault naming %q", key)
	}
	want := rules.SheetFault{
		Field:  "ability_ids",
		Key:    key,
		Reason: `names ability "` + key + `", which the ruleset does not hold`,
	}
	if *got != want {
		t.Fatalf("SheetAbilityFault = %+v, want %+v", *got, want)
	}
}

// VTT-685
func TestQAAbAnEmptyOrAbsentListHasNoFault(t *testing.T) {
	rs := qaAbLoadTavern(t)
	if f := rs.SheetAbilityFault(nil); f != nil {
		t.Fatalf("SheetAbilityFault(nil) = %+v, want nil", *f)
	}
	if f := rs.SheetAbilityFault([]string{}); f != nil {
		t.Fatalf("SheetAbilityFault([]) = %+v, want nil", *f)
	}
}

// VTT-685
func TestQAAbEveryHeldIdInAnyOrderHasNoFault(t *testing.T) {
	rs := qaAbLoadTavern(t)
	if f := rs.SheetAbilityFault([]string{"sober-up", "fists", "splash-of-water", "chair-swing"}); f != nil {
		t.Fatalf("SheetAbilityFault(all held) = %+v, want nil", *f)
	}
}

// VTT-685
func TestQAAbARepeatedHeldIdHasNoFault(t *testing.T) {
	rs := qaAbLoadTavern(t)
	if f := rs.SheetAbilityFault([]string{"fists", "chair-swing", "fists"}); f != nil {
		t.Fatalf("SheetAbilityFault(repeated held) = %+v, want nil", *f)
	}
}

// VTT-685
func TestQAAbTheFirstUnheldIdInByteOrderIsNamed(t *testing.T) {
	rs := qaAbLoadTavern(t)
	qaAbWantFault(t, rs.SheetAbilityFault([]string{"zz-unheld", "beta-unheld", "fists", "Zed-unheld"}), "Zed-unheld")
}

// VTT-685
func TestQAAbAHeldIdFirstInByteOrderIsPassedOver(t *testing.T) {
	rs := qaAbLoadTavern(t)
	qaAbWantFault(t, rs.SheetAbilityFault([]string{"splash-of-water", "zz-unheld", "chair-swing"}), "zz-unheld")
}

// VTT-685
func TestQAAbAnIdDifferingOnlyInCaseIsNotHeld(t *testing.T) {
	rs := qaAbLoadTavern(t)
	qaAbWantFault(t, rs.SheetAbilityFault([]string{"fists", "Fists"}), "Fists")
}

// VTT-685
func TestQAAbAnEmptyIdIsNotHeld(t *testing.T) {
	rs := qaAbLoadTavern(t)
	qaAbWantFault(t, rs.SheetAbilityFault([]string{"fists", ""}), "")
}

// VTT-685
func TestQAAbHeldMeansHeldInCompiled(t *testing.T) {
	rs := qaAbLoadTavern(t)
	qaAbWantFault(t, rs.SheetAbilityFault([]string{"qa-extra"}), "qa-extra")
	rs.Compiled["qa-extra"] = &rules.CompiledPower{ID: "qa-extra", Name: "QA Extra"}
	if f := rs.SheetAbilityFault([]string{"qa-extra", "fists"}); f != nil {
		t.Fatalf("SheetAbilityFault after adding qa-extra to Compiled = %+v, want nil", *f)
	}
	delete(rs.Compiled, "fists")
	qaAbWantFault(t, rs.SheetAbilityFault([]string{"qa-extra", "fists"}), "fists")
}

// VTT-683
func TestQAAbTheCallersListKeepsItsOrder(t *testing.T) {
	rs := qaAbLoadTavern(t)
	ids := []string{"zz-unheld", "fists", "Alpha-unheld", "chair-swing", "fists"}
	before := slices.Clone(ids)
	qaAbWantFault(t, rs.SheetAbilityFault(ids), "Alpha-unheld")
	if !slices.Equal(ids, before) {
		t.Fatalf("caller's ids after SheetAbilityFault = %q, want unchanged %q", ids, before)
	}
	held := []string{"sober-up", "fists", "chair-swing"}
	heldBefore := slices.Clone(held)
	if f := rs.SheetAbilityFault(held); f != nil {
		t.Fatalf("SheetAbilityFault(held) = %+v, want nil", *f)
	}
	if !slices.Equal(held, heldBefore) {
		t.Fatalf("caller's held ids after SheetAbilityFault = %q, want unchanged %q", held, heldBefore)
	}
}
