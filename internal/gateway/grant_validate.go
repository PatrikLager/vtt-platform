package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// validateGrantActorControl refuses a grant that does not say what the actor
// IS — visibility spec §5.1's third rule, which the spec calls the
// load-bearing one.
//
// WHY A REFUSAL AND NOT A DEFAULT. proto3 has no `required`, so an omitted
// enum arrives as the zero value and is INDISTINGUISHABLE from a caller who
// deliberately said "unspecified". Both available defaults are wrong in a way
// that shows up as a live table getting the wrong information: default to
// party member and one forgotten field publishes a monster's whole stat block
// to everyone; default to non-party and one forgotten field drops a
// character out of its own party's roster. Refusing is the only answer that
// asks rather than guesses.
//
// (This used to reason from §5.1's migration rule — that the rule read absence
// and could not tell an old log from a grant issued today that forgot. The
// rule was deleted 2026-08-24: an absent kind is not a party member, full
// stop. That makes the SECOND default the safe one and this refusal no longer
// the only thing between the table and the leak — but a grant that silently
// demotes a character is still a wrong answer given confidently, which is what
// this refuses to do.)
//
// WHY HERE AND NOT IN THE FOLD. engine.Apply must keep accepting every
// kindless grant already recorded (its own ActorControlGranted arm says so at
// length): a fold that refused them would poison every campaign in existence.
// The command boundary is the one place that can tell a grant being ISSUED
// from a grant being REPLAYED, so it is the only place this rule can live.
//
// WHY NOT IN Authorize. This is not a rule about who: the DM and the agent
// are both entitled to hand a character over, and neither may do it without
// saying what they are handing over. Wrapping it in ErrUnauthorized would
// teach the reader that the caller lacked permission, when what they lacked
// was a field — the same "a refusal that misdescribes the rule teaches the
// wrong one" argument MayPerch's own refusal string was corrected under.
// It sits beside validateCreateSceneTerrain in handleCommand instead, which
// is the same seam for the same reason: format validity, checked before
// anything is written, for every role.
//
// The check is UNSPECIFIED-only rather than an allowlist of the two values
// that exist today. ActorKind's own doc comment says a third value is
// foreseeable — a neutral, a familiar, a summon — and that every future value
// reads as "not a party member" until something deliberately says otherwise.
// An allowlist here would refuse a value the contract offers, turning additive
// growth into a broken command; UNSPECIFIED-only lets the enum grow and keeps
// the growth failing closed at the reader, which is where that decision lives.
func validateGrantActorControl(cmd *vttv1.GrantActorControl) error {
	if cmd.GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		return fmt.Errorf("gateway: grant_actor_control: kind — a grant must say what the actor IS "+
			"(%s for a character the party knows, %s for a creature they must discover); "+
			"an unstated kind cannot be told from a deliberate one, so it is refused rather than guessed",
			vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	}
	return nil
}
