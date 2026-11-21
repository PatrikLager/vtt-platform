package gateway_test

import (
	"fmt"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Keep a1 the actor p-1 controls: the role table must refuse a player who
// controls the very actor named, before any player rule is asked.
// VTT-716
func TestOnlyTheDMAndTheAgentMayGiveOrTakeAnAbility(t *testing.T) {
	st := ownershipFixture()
	for _, name := range []string{"give_ability", "take_ability"} {
		for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator} {
			err := gateway.Authorize(&identity.Participant{ID: authzCaseID(role), Role: role}, commandFor(t, name), st)
			if role == identity.RoleDM || role == identity.RoleAgent {
				if err != nil {
					t.Errorf("%s: %s refused: %v", role, name, err)
				}
				continue
			}
			want := fmt.Sprintf("gateway: not authorized: role %q may not issue %q", role, name)
			if err == nil || err.Error() != want {
				t.Errorf("%s: %s answered %v, want %q", role, name, err, want)
			}
		}
	}
}
