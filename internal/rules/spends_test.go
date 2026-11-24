package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Spends is the reading VTT-761 and VTT-991 rest on.
func TestSpendsAnswersTheResourceAHeldLimitedAbilitySpends(t *testing.T) {
	for dir, cases := range map[string]map[string]string{
		"tavern-brawl":   {"chair-swing": "stamina", "fists": "", "sober-up": "", "no-such-ability": ""},
		"dnd45e-minimal": {"hunters-flurry": "flurry_uses", "rally": "", "longsword-strike": ""},
	} {
		rs, err := rules.Load(filepath.Join("..", "..", "rulesets", dir))
		if err != nil {
			t.Fatalf("rules.Load(%s): %v", dir, err)
		}
		for id, want := range cases {
			got, spends := rs.Spends(id)
			if got != want || spends != (want != "") {
				t.Errorf("%s: Spends(%q) = %q, %v; want %q, %v", dir, id, got, spends, want, want != "")
			}
		}
	}
}
