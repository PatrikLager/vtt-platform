package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Check the controller before the id and the kind: a caller wrong in both must
// be told the one that is a misunderstanding of the model (SPEC-013).
//
// Never move the kind refusal into engine.Apply: a kindless ActorAdded is a
// legal recorded state (SPEC-013).
func validateAddActor(cmd *vttv1.AddActor) error {
	a := cmd.GetActor()
	if a.GetControllerId() != "" || len(a.GetControllerIds()) > 0 {
		// Refuse a declared-but-empty set too: the caller meant to seed a controller
		// (TestAddActorSeedingAnEmptyControllerSetIsRefused).
		return fmt.Errorf("gateway: add_actor: controller — creating an actor does not hand it to " +
			"anyone; control is conferred by grant_actor_control, which also says whether the " +
			"actor is a party member or not. Add the actor with no controller, then grant it")
	}
	// Leave a missing id to the fold, which says the id is missing (VTT-154).
	if a.GetActorId() == "" {
		return nil
	}
	if a.GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		return fmt.Errorf("gateway: add_actor: kind — creating an actor must say what it IS "+
			"(%s for a character the party knows about, %s for a creature they must discover "+
			"by seeing it); an unstated kind cannot be told from a deliberate one, so it is "+
			"refused rather than guessed",
			vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	}
	return nil
}
