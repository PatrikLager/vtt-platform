package gateway

import (
	"fmt"

	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// MayPerch reports whether p may perch on actorID's shoulder in st (SPEC-015).
func MayPerch(p *identity.Participant, actorID string, st *engine.State) error {
	if p.Role != identity.RoleSpectator {
		return fmt.Errorf("%w: role %q does not perch — a viewpoint is the spectator's",
			ErrUnauthorized, p.Role)
	}
	if actorID == "" {
		// Accept the empty id: it is how a spectator leaves a shoulder
		// (TestUnperchingNamesNoActorAndIsAllowed).
		return nil
	}
	a, ok := st.Actors[actorID]
	if !ok || !engine.IsPartyMember(a) {
		// Answer an absent actor and a non-party one with this one string: a
		// difference enumerates the DM's cast
		// (TestAPerchRefusalDoesNotSayWhetherTheActorExists).
		return fmt.Errorf("%w: %q is not a party member, so it is no shoulder to sit on",
			ErrUnauthorized, actorID)
	}
	return nil
}
