package gateway

import (
	"fmt"

	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// MayPerch reports whether p may ride actorID's shoulder — the spectator's
// affordance, and the whole of visibility spec §3.1.1: "you as a spectator can
// jump between tokens, like a bird hopping from one shoulder to another."
//
// ONLY A PARTY MEMBER. This is the constraint the whole idea rests on. A
// spectator perched on the Goblin Archer would watch the ambush from INSIDE IT,
// undoing this arc in a single click. Enforced HERE, on the server, never by
// which names a UI happens to offer. Projector.eyes refuses the same perch a
// second time, for one that arrives by some route this function never saw
// (spec §8, defence in depth).
//
// WHAT IS ACTUALLY CHECKED IS engine.IsPartyMember, and it is the same
// predicate spec §5's roster and Projector.eyes now run on — one rule, all
// three call sites.
// It used to be "has any controller", which was a DIFFERENT sentence from the
// one the spec wrote: a DM who granted themselves the Goblin Archer made it
// perchable, demonstrated in review and left flagged here for adjudication.
// Patrik ruled on 2026-08-23 (spec §5.1): kind belongs to the ACTOR, not to
// whoever holds it. The gap this comment used to describe is closed, and the
// roster is no longer PRECEDENT for leaving it open — the two are one rule now,
// and fixing only one of them would have left this half wide.
//
// ONLY A SPECTATOR. "An unassigned PLAYER does not perch" — their answer to an
// empty board is to be GIVEN a character, which is the onboarding flow working
// as intended, and the DM and the agent already see everything. commandRoles
// denies set_viewpoint to all three, so this arm is not what stops them over
// the wire; it is here because MayPerch is exported and its subject is p, and a
// caller reaching it another way must not hear something the table would not
// say.
//
// THE TWO REFUSALS ARE ONE STRING, and that is not tidiness. "There is no such
// actor" and "that actor is an NPC" are different answers, and the difference
// between them is precisely the knowledge spec §2 refuses to model: a watcher
// could enumerate the DM's cast by perching on guesses and reading the
// refusals back. It is Task 5's move_token oracle one command over, and it is
// answered the same way — one message for both, naming only the id the asker
// themselves just sent. TestAPerchRefusalDoesNotSayWhetherTheActorExists pins
// the byte-equality.
func MayPerch(p *identity.Participant, actorID string, st *engine.State) error {
	if p.Role != identity.RoleSpectator {
		return fmt.Errorf("%w: role %q does not perch — a viewpoint is the spectator's",
			ErrUnauthorized, p.Role)
	}
	if actorID == "" {
		// Naming no actor is how a bird LEAVES a shoulder without immediately
		// sitting on another. It reveals nothing and shows nothing: eyes reads
		// the empty id as no eyes at all, which is the state every spectator
		// starts a connection in.
		return nil
	}
	a, ok := st.Actors[actorID]
	if !ok || !engine.IsPartyMember(a) {
		// "not a party member" rather than the old "not a character a player
		// controls": after §5.1 that sentence is no longer true of what is
		// checked, and a refusal that misdescribes the rule teaches the reader
		// the wrong one. It is still ONE string for both refusals, which is the
		// property TestAPerchRefusalDoesNotSayWhetherTheActorExists pins.
		return fmt.Errorf("%w: %q is not a party member, so it is no shoulder to sit on",
			ErrUnauthorized, actorID)
	}
	return nil
}
