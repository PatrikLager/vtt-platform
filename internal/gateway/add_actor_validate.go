package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// validateAddActor refuses an add_actor that tries to confer control, which
// closes the last route by which an actor became a party member with NOBODY
// HAVING SAID SO (visibility spec §5.1).
//
// IT DOES NOT MAKE §5.1's FIRST RULE LITERALLY TRUE, and the difference is
// worth stating rather than glossing. That rule reads "an ungranted actor is
// NOT a party member", and its justification is "a monster nobody has been
// granted has no grant to carry a kind, so it defaults closed". The
// justification is now exactly true: nothing INFERS party membership, from
// control or from anything else. The sentence is not, because Actor.kind is
// still settable on the actor itself, so `add_actor {actorId: "x", kind:
// PARTY_MEMBER}` creates an ungranted party member — deliberately, by a caller
// who typed the words. Task 2 kept that field on purpose (the pregen case, and
// the DM console's paste path round-trips a whole Actor through it), and
// removing it was not part of Patrik's ruling here. Pinned by
// TestAddActorMayStillDeclareAPartyMemberWithoutGrantingIt below so the
// behaviour is a decision rather than an accident, and FLAGGED: if §5.1's
// sentence is meant literally, this field is the remaining work.
//
// THE HOLE IT CLOSED. Two commands could put a controller on an actor.
// grant_actor_control was made to declare a kind and to refuse silence
// (validateGrantActorControl); add_actor was not. So the original leak stayed
// reachable through the other door: create the actor with a controller_id and
// no kind, and §5.1's migration rule as it then stood — "absent + has a
// controller -> party member" — read it as a party member on the spot. The
// whole cloned Actor reached every player's roster, and MayPerch and eyes()
// opened on it. Both seats that may issue the command could do it: the MCP
// agent through cmd/vtt/tools.json, and a human through the DM console's old
// "controller participant id (optional)" box.
//
// That migration rule was DELETED the same day (§5.1, 2026-08-24): an absent
// kind is not a party member, whoever holds the actor. So this refusal and the
// deletion close the hole twice over, from opposite ends — nothing can seed a
// controller, and a controller would confer no standing if it could.
//
// WHY A REFUSAL AND NOT "add_actor must also state a kind". That repair works,
// and it leaves TWO commands able to confer control — therefore two rules that
// must agree with each other forever, in an append-only log where a
// disagreement is permanent. RPTool shipped that shape and carries the
// resulting invariant bug in its tree today: clearAllOwners leaves ownerType ==
// OWNER_TYPE_ALL, worked around at EditTokenDialog.java:885 rather than fixed.
// One conferring command cannot disagree with itself. Patrik's ruling,
// 2026-08-24, choosing exit (b) of Task 2's three.
//
// WHAT THIS DOES NOT DO. add_actor STAYS, and so does Actor.kind on it.
// Authored characters live in the campaign file; add_actor is the runtime path
// for the creature nobody wrote in advance — an LLM DM inventing a bandit when
// the party goes somewhere unplanned. What is removed is its ability to seed a
// CONTROLLER, not the command, and not the actor's ability to say what it is.
//
// WHY HERE AS WELL AS IN THE FOLD, which is where this differs from
// validateGrantActorControl. engine.Apply REFUSES an ActorAdded that names a
// controller too (its own arm says why), so the invariant does not depend on
// this function: nothing can put the shape in a log by any route. What this
// adds is the ANSWER. A fold error is a poisoned append that names an event;
// this names the command that DOES confer control, before anything is written,
// to the caller who tried the one-step — an LLM DM reading a tool result or a
// human at the DM console. Belt and braces, in that order.
//
// (An earlier draft of this comment argued the opposite — that the fold had to
// keep accepting the shape because logs already written were full of them, and
// that the command boundary was therefore the ONLY place the rule could live.
// That was true of the grant's kind and never of this: Patrik ruled on
// 2026-08-24 that no campaign exists outside this repo's own fixtures, so there
// is no history to protect and the fold takes the simple, fail-closed answer.)
//
// BOTH SPELLINGS, and neither is redundant. controller_ids is the
// AUTHORITATIVE field (Actor.controller_ids' doc comment) and controller_id is
// its mirror, so a check written against the mirror alone leaves the real door
// open — and one written against the set alone misses the field every old
// fixture and every LLM caller reaches for first.
func validateAddActor(cmd *vttv1.AddActor) error {
	a := cmd.GetActor()
	if a.GetControllerId() == "" && len(a.GetControllerIds()) == 0 {
		return nil
	}
	// A declared-but-empty set (`controller_ids: [""]`) lands here too, and
	// deliberately. It would confer no control on anyone — an empty id names no
	// participant — but a caller who wrote that field meant to seed a
	// controller, and the answer to "you cannot seed one here" must not depend
	// on whether the id they picked happened to be usable. engine.Apply refuses
	// it on the same terms; it used to STRIP such ids instead, which is the
	// behaviour this refusal replaced.
	return fmt.Errorf("gateway: add_actor: controller — creating an actor does not hand it to " +
		"anyone; control is conferred by grant_actor_control, which also says whether the " +
		"actor is a party member or not. Add the actor with no controller, then grant it")
}
