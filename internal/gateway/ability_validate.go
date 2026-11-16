package gateway

import (
	"fmt"
	"maps"
	"slices"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Read a give against rs alone; the sheet is read by handleGiveAbility and the
// rest by the fold (SPEC-013, VTT-718, VTT-763, VTT-765, VTT-766).
func validateGiveAbility(cmd *vttv1.GiveAbility, rs *rules.Ruleset) error {
	keys := slices.Sorted(maps.Keys(cmd.GetResources()))
	if rs == nil {
		if len(keys) > 0 {
			return giveFault(cmd, "resources", fmt.Sprintf(
				"carries resource %q, and no ruleset is served to say what the ability spends", keys[0]))
		}
		return nil
	}
	if f := rs.SheetAbilityFault([]string{cmd.GetAbilityId()}); f != nil {
		return giveFault(cmd, f.Field, f.Reason)
	}
	spent, limited := rs.Spends(cmd.GetAbilityId())
	for _, key := range keys {
		if !limited || key != spent {
			return giveFault(cmd, "resources", fmt.Sprintf("ability %q spends no resource %q", cmd.GetAbilityId(), key))
		}
	}
	return nil
}

func giveFault(cmd *vttv1.GiveAbility, field, reason string) error {
	return fmt.Errorf("gateway: give_ability: actor %q: %s: %s", cmd.GetActorId(), field, reason)
}
