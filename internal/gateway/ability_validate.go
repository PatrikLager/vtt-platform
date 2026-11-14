package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Read a give against rs alone, and nothing without rs: the fold refuses what
// the state does (SPEC-013, VTT-718, VTT-745).
func validateGiveAbility(cmd *vttv1.GiveAbility, rs *rules.Ruleset) error {
	if rs == nil {
		return nil
	}
	if f := rs.SheetAbilityFault([]string{cmd.GetAbilityId()}); f != nil {
		return fmt.Errorf("gateway: give_ability: actor %q: %s: %s", cmd.GetActorId(), f.Field, f.Reason)
	}
	return nil
}
