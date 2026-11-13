package rules_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func qaUseSheet(ids ...string) *vttv1.Actor {
	return &vttv1.Actor{ActorId: "caster", AbilityIds: ids}
}

// VTT-690
func TestQAUseOffersEveryAbilityTheSheetNamesWhereverItStands(t *testing.T) {
	sheet := qaUseSheet("sober-up", "chair-swing", "fists")
	for _, id := range []string{"sober-up", "chair-swing", "fists"} {
		if !rules.Offers(sheet, id) {
			t.Errorf("Offers(%v, %q) = false, want true", sheet.GetAbilityIds(), id)
		}
	}
}

// VTT-690
func TestQAUseOffersNothingFromASheetNamingNone(t *testing.T) {
	for _, sheet := range []*vttv1.Actor{qaUseSheet(), {ActorId: "caster", AbilityIds: []string{}}} {
		for _, id := range []string{"fists", "chair-swing", ""} {
			if rules.Offers(sheet, id) {
				t.Errorf("Offers(%v, %q) = true, want false", sheet.GetAbilityIds(), id)
			}
		}
	}
}

// VTT-690
func TestQAUseOffersOnlyAByteEqualId(t *testing.T) {
	cases := []struct {
		named []string
		asked string
	}{
		{[]string{"Fists"}, "fists"},
		{[]string{"fists"}, "Fists"},
		{[]string{"fists"}, "FISTS"},
		{[]string{"fist"}, "fists"},
		{[]string{"fists"}, "fist"},
		{[]string{"fists-of-fury"}, "fists"},
		{[]string{"fists"}, "fists-of-fury"},
		{[]string{"fists "}, "fists"},
		{[]string{"fists"}, " fists"},
		{[]string{"chair-swing"}, "chair"},
		{[]string{"chair-swing"}, "swing"},
		{[]string{"café"}, "café"},
		{[]string{"fists"}, ""},
		{[]string{"sober-up", "chair-swing"}, "fists"},
	}
	for _, c := range cases {
		if rules.Offers(qaUseSheet(c.named...), c.asked) {
			t.Errorf("Offers(%q, %q) = true, want false", c.named, c.asked)
		}
	}
}

// VTT-690
func TestQAUseOffersReadsTheSheetItIsHandedAndNoOther(t *testing.T) {
	a := qaUseSheet("fists")
	b := qaUseSheet("chair-swing")
	if !rules.Offers(a, "fists") || rules.Offers(a, "chair-swing") {
		t.Errorf("sheet %v: Offers fists=%v chair-swing=%v, want true false",
			a.GetAbilityIds(), rules.Offers(a, "fists"), rules.Offers(a, "chair-swing"))
	}
	if rules.Offers(b, "fists") || !rules.Offers(b, "chair-swing") {
		t.Errorf("sheet %v: Offers fists=%v chair-swing=%v, want false true",
			b.GetAbilityIds(), rules.Offers(b, "fists"), rules.Offers(b, "chair-swing"))
	}
	if got := a.GetAbilityIds(); len(got) != 1 || got[0] != "fists" {
		t.Errorf("Offers changed the sheet it read: %v", got)
	}
}
