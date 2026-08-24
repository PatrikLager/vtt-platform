package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// validateAddActor refuses an add_actor that tries to confer control, and an
// add_actor that will not say what it is creating. Two refusals, one rule:
// CREATING AN ACTOR IS A STATEMENT ABOUT WHAT IT IS, NEVER ABOUT WHO HOLDS IT,
// and it may not be made in silence (visibility spec §5.1).
//
// It therefore CREATES UNGRANTED PARTY MEMBERS, deliberately. `add_actor
// {actorId: "hollis", kind: PARTY_MEMBER}` is a pregenerated character sitting
// in the campaign before anyone is assigned to one, and the party seeing that
// those four sheets exist is correct rather than a leak. Nothing was inferred;
// the caller said it. (§5.1's first sentence — "an ungranted actor is NOT a
// party member" — is what that pairing falsifies, and Patrik is amending it.
// Pinned by TestAddActorDeclaresAPartyMemberWithoutGrantingIt below.)
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
// THE SECOND REFUSAL: A KIND IS REQUIRED, and stating why corrects the
// argument the three tasks before this one were justified by.
//
// Those tasks reasoned "one writer beats two" — that letting add_actor confer
// control alongside grant_actor_control would leave two rules that must agree
// forever, the shape RPTool shipped and still carries as an invariant bug
// (clearAllOwners leaves ownerType == OWNER_TYPE_ALL, worked around at
// EditTokenDialog.java:885 rather than fixed). That argument was a PROXY for
// the real rule, and it was about CONTROL. It does not reach here: saying what
// a creature IS is not saying who holds it, and this command still confers
// nothing.
//
// THE DANGER WAS NEVER MULTIPLICITY. IT WAS A WRITER THAT COULD STAY SILENT.
// The archer leak came from inference from absence — an actor holding a
// controller with no kind had to be GUESSED at, and the guess was wrong. Two
// writers are fine as long as neither can be mute, and after this refusal
// neither is: every path that creates an actor states its kind, including the
// authored one (internal/adventure's loadActors refuses a kindless
// actors/*.json for the same reason).
//
// A REFUSAL RATHER THAN A DEFAULT, exactly as validateGrantActorControl's own
// comment argues at length: proto3 has no `required`, so an omitted enum
// arrives as the zero value and cannot be told from a caller who deliberately
// said "unspecified". Both available defaults are wrong in a way that reaches
// a live table — default to party member and one forgotten field publishes a
// monster's whole stat block; default to non-party and one forgotten field
// drops a character out of its own party's roster. Refusing is the only answer
// that asks instead of guessing. Patrik, 2026-08-24: "when you create an actor
// you should know what for — is it an NPC or a PC."
//
// UNSPECIFIED-ONLY, not an allowlist of the two values that exist today.
// ActorKind's doc comment says a third is foreseeable (a neutral, a familiar,
// a summon) and that every future value reads as "not a party member" until
// something deliberately says otherwise. An allowlist here would refuse a
// value the contract offers, turning additive growth into a broken command.
//
// AND ONLY ONCE THERE IS AN ACTOR TO ASK ABOUT. An absent Actor reads as
// UNSPECIFIED through the same getter a real kindless actor does, so an
// unguarded check would tell a caller who sent no usable actor that their KIND
// was the problem. That refusal belongs to the fold, which names the actual
// mistake ("actor_added requires an actor with an id") — a refusal that
// misdescribes the rule teaches the wrong one.
//
// "ABSENT" MEANS NO ID, NOT A NIL POINTER, and the difference is the whole
// value of the guard (review finding, 2026-08-24). A nil check alone defers
// `{"addActor":{}}` and nothing else — but protojson delivers
// `{"addActor":{"actor":{}}}` as a NON-NIL empty Actor, which is the shape a
// confused caller actually sends, and it fell straight through to the kind
// refusal. Keying on the id covers both and matches the fold's own wording.
//
// WHAT THIS DOES NOT DO. add_actor STAYS, and Actor.kind on it is now
// mandatory rather than merely permitted. Authored characters live in the
// campaign file; add_actor is the runtime path for the creature nobody wrote
// in advance — an LLM DM inventing a bandit when the party goes somewhere
// unplanned. What is removed is its ability to seed a CONTROLLER, not the
// command, and not the actor's ability to say what it is.
//
// WHY THE CONTROLLER REFUSAL IS HERE AS WELL AS IN THE FOLD, which is where
// it differs from validateGrantActorControl. engine.Apply REFUSES an ActorAdded
// that names a controller too (its own arm says why), so the invariant does not
// depend on this function: nothing can put the shape in a log by any route.
// What this adds is the ANSWER. A fold error is a poisoned append that names an
// event; this names the command that DOES confer control, before anything is
// written, to the caller who tried the one-step — an LLM DM reading a tool
// result or a human at the DM console. Belt and braces, in that order.
//
// AND WHY THE KIND REFUSAL IS HERE AND NOWHERE ELSE, which is the opposite
// answer to the same question and worth reading beside it. The fold does not
// reject a kindless ActorAdded and must not: an absent kind is a REAL STATE on
// a recorded event, meaning "not a party member" (ActorKind's own doc comment),
// so a fold that refused it would be refusing something the contract defines.
// The boundary is the only seam that can tell a command being ISSUED from an
// event being REPLAYED, which is exactly validateGrantActorControl's argument
// for living here — and, unlike the controller rule, the kind rule needs it.
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
	if a.GetControllerId() != "" || len(a.GetControllerIds()) > 0 {
		// A declared-but-empty set (`controller_ids: [""]`) lands here too, and
		// deliberately. It would confer no control on anyone — an empty id names
		// no participant — but a caller who wrote that field meant to seed a
		// controller, and the answer to "you cannot seed one here" must not
		// depend on whether the id they picked happened to be usable.
		// engine.Apply refuses it on the same terms; it used to STRIP such ids
		// instead, which is the behaviour this refusal replaced.
		//
		// CHECKED BEFORE THE KIND, so a command that does both wrong things is
		// answered with the one that is a misunderstanding of the model rather
		// than a forgotten field. Told "state a kind", a caller adds one and
		// sends the same forbidden shape again.
		return fmt.Errorf("gateway: add_actor: controller — creating an actor does not hand it to " +
			"anyone; control is conferred by grant_actor_control, which also says whether the " +
			"actor is a party member or not. Add the actor with no controller, then grant it")
	}
	// Deferred to the fold, which names the id — see the doc comment above.
	// AFTER the controller check on purpose: "creation does not confer control"
	// is worth saying whatever else is wrong with the command, because fixing
	// the id alone would earn the same refusal one round trip later.
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
