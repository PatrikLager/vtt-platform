package rules_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// VTT-690
func TestOffersAnswersWhetherTheSheetNamesTheAbility(t *testing.T) {
	a := &vttv1.Actor{AbilityIds: []string{"rally", "fists"}}
	for id, want := range map[string]bool{"fists": true, "rally": true, "chair-swing": false, "Fists": false, "": false} {
		if got := rules.Offers(a, id); got != want {
			t.Errorf("Offers(sheet naming [rally fists], %q) = %v, want %v", id, got, want)
		}
	}
	if rules.Offers(&vttv1.Actor{}, "fists") {
		t.Error("a sheet naming no ability offers fists")
	}
}
