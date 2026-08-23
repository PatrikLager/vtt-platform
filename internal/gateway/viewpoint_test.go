package gateway_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// watcher is the participant every test in this file asks about: someone with
// no character of their own, which is the seat perching exists for.
func watcher() *identity.Participant {
	return &identity.Participant{ID: "s-1", Role: identity.RoleSpectator}
}

func TestASpectatorMayPerchOnAPartyMemberButNotOnAnNPC(t *testing.T) {
	// twoRooms (project_test.go) gives "hero" a controller and "goblin" none.
	st := twoRooms()

	if err := gateway.MayPerch(watcher(), "hero", st); err != nil {
		t.Errorf("a spectator may ride a party member: %v", err)
	}
	// THE CONSTRAINT THE WHOLE IDEA RESTS ON (spec §3.1.1). A spectator
	// perched on the Goblin Archer watches the ambush from inside it, and the
	// arc is undone in a single click.
	if err := gateway.MayPerch(watcher(), "goblin", st); err == nil {
		t.Fatal("perching on an NPC must be REFUSED by the server, not merely absent from a menu")
	}
}

func TestASpectatorMayNotPerchOnAnNPCTheDMControls(t *testing.T) {
	// The rule above, held to what an actor IS rather than to whether anyone
	// holds it (spec §5.1). A DM taking control of the Goblin Archer is an
	// ordinary act — grant_actor_control constrains the issuer, not the target
	// — and under "has any controller" it made that monster a shoulder any
	// watcher could sit on, which §3.1.1 calls the constraint the whole idea
	// rests on. Demonstrated in review; this is the test for it.
	st := twoRooms()
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1"})

	if err := gateway.MayPerch(watcher(), "goblin", st); err == nil {
		t.Fatal("a monster the DM holds is still a monster: perching on it must be refused")
	}
}

func TestAnActorFromBeforeTheKindFieldIsStillPerchableWhenControlled(t *testing.T) {
	// MIGRATION at the perch (spec §5.1): absent + a controller means party
	// member, here as everywhere. twoRooms' hero declares no kind and is
	// controlled by p-1, which is exactly the shape of every character in
	// every log recorded before the field existed — and a spectator who could
	// ride them yesterday must still be able to today.
	st := twoRooms()
	if k := st.Actors["hero"].GetKind(); k != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatalf("fixture check: the hero must declare NO kind for this to be the migration case, got %v", k)
	}

	if err := gateway.MayPerch(watcher(), "hero", st); err != nil {
		t.Errorf("a party member from before the kind field is still a shoulder to sit on: %v", err)
	}
}

// TestAPerchRefusalDoesNotSayWhetherTheActorExists is the oracle half of the
// rule above, and it is Task 5's move_token lesson one command over: a refusal
// that VARIES tells the asker something, and a spectator may send as many as
// they like.
//
// The differential is the SAME id against two worlds — one where "goblin" is an
// NPC standing in the far room, one where no such actor was ever added. If
// those answers differ, a watcher can enumerate the DM's cast by perching on
// guesses and reading the refusals, which is exactly the knowledge spec §2
// refuses to model.
//
// Byte-equality rather than a substring check, for the reason
// TestAPlayerCannotProbeTheDarkWithMoveCommands gives: a message that merely
// avoided the word "NPC" would still be an oracle. Nothing is lost by it — the
// refusal names the id the spectator themselves just sent, so it stays
// actionable while telling them nothing they did not already hold.
func TestAPerchRefusalDoesNotSayWhetherTheActorExists(t *testing.T) {
	present := twoRooms()

	absent := twoRooms()
	delete(absent.Actors, "goblin")
	delete(absent.Tokens, "t-gob")

	npc := gateway.MayPerch(watcher(), "goblin", present)
	unknown := gateway.MayPerch(watcher(), "goblin", absent)
	if npc == nil || unknown == nil {
		t.Fatalf("both must be refused (npc=%v unknown=%v)", npc, unknown)
	}
	if npc.Error() != unknown.Error() {
		t.Fatalf("a perch refusal must not say whether the actor exists:\n"+
			" npc: %s\n absent: %s", npc, unknown)
	}
}

// TestOnlyASpectatorRidesAShoulder pins the other half of MayPerch's subject.
//
// "An unassigned PLAYER does not perch" (spec §3.1.1): their answer to an empty
// board is to be given a character, and honouring a viewpoint for them would
// make Viewpoint a client-settable field naming any actor at the table — which
// is session zero with an extra step.
//
// commandRoles already denies set_viewpoint to every non-spectator, so this
// arm is not what stops a player over the wire today. It is here because
// MayPerch is EXPORTED and its subject is p: a caller who reaches it by
// another route must not be told something the table would not say.
func TestOnlyASpectatorRidesAShoulder(t *testing.T) {
	st := twoRooms()
	for _, role := range []identity.Role{
		identity.RolePlayer, identity.RoleDM, identity.RoleAgent,
	} {
		p := &identity.Participant{ID: "p-1", Role: role}
		if err := gateway.MayPerch(p, "hero", st); err == nil {
			t.Errorf("role %q must not perch: perching is the spectator's affordance", role)
		}
	}
}

// TestUnperchingNamesNoActorAndIsAllowed covers the empty id, which is how a
// spectator gets OFF a shoulder without immediately sitting on another.
func TestUnperchingNamesNoActorAndIsAllowed(t *testing.T) {
	if err := gateway.MayPerch(watcher(), "", twoRooms()); err != nil {
		t.Fatalf("naming no actor is how a bird leaves a shoulder: %v", err)
	}
}
