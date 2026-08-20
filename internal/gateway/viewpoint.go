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
// ONLY A PLAYER-CONTROLLED ACTOR. This is the constraint the whole idea rests
// on. An actor with an EMPTY control set is DM/agent-only — an NPC, the same
// meaning authorizeTokenOwnership gives it — and a spectator perched on the
// Goblin Archer would watch the ambush from INSIDE IT, undoing this arc in a
// single click. Enforced HERE, on the server, never by which names a UI happens
// to offer. Projector.eyes refuses the same perch a second time, for one that
// arrives by some route this function never saw (spec §8, defence in depth).
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
	if !ok || len(a.GetControllerIds()) == 0 {
		return fmt.Errorf("%w: %q is not a character a player controls, so it is no shoulder to sit on",
			ErrUnauthorized, actorID)
	}
	return nil
}
