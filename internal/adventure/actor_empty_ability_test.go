package adventure_test

import (
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
)

// VTT-748
func TestAnActorFileWhoseAbilityIdsHoldAnEmptyIdIsRefused(t *testing.T) {
	for _, ids := range []string{`[""]`, `["fists", ""]`, `["nope", ""]`} {
		t.Run(ids, func(t *testing.T) {
			_, err := adventure.Load(writeBraceGuardSheet(t, braceGuardFits, `, "ability_ids": `+ids),
				abilityRuleset(t, "fists"))
			if err == nil {
				t.Fatal("an actor whose ability_ids hold an empty id was loaded")
			}
			for _, want := range []string{"brace-guard.json", `field "ability_ids": must not hold an empty id`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
}

// VTT-748
func TestAnActorFilesKeysAndValuesAreReadBeforeAnEmptyAbilityId(t *testing.T) {
	for _, c := range []struct{ name, attributes, want string }{
		{"an undeclared key", `{"vim": 10, "vigor": 10, "nope": 1}`, `field "attributes"`},
		{"a value written null", `{"vim": 10, "vigor": 10, "brace": null}`, `field "attributes.brace"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := adventure.Load(writeBraceGuardSheet(t, c.attributes, `, "ability_ids": [""]`), abilityRuleset(t))
			if err == nil {
				t.Fatal("an actor holding a faulty sheet and an empty ability id was loaded")
			}
			if !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "ability_ids") {
				t.Errorf("error = %q, want the %s's refusal and not the ability's", err.Error(), c.name)
			}
		})
	}
}
