package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Refuse UNSPECIFIED and nothing else: an allowlist refuses a kind the contract
// may add (TestEveryActorKindTheContractOffersIsAcceptedByAGrant).
//
// Never wrap this in ErrUnauthorized: a missing field is not a missing
// permission (SPEC-013).
func validateGrantActorControl(cmd *vttv1.GrantActorControl) error {
	if cmd.GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		return fmt.Errorf("gateway: grant_actor_control: kind — a grant must say what the actor IS "+
			"(%s for a character the party knows, %s for a creature they must discover); "+
			"an unstated kind cannot be told from a deliberate one, so it is refused rather than guessed",
			vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	}
	return nil
}
