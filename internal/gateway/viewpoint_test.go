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

// TestAnActorWithNoDeclaredKindIsNoShoulderHoweverManyHoldIt is the INVERSION
// of TestAnActorFromBeforeTheKindFieldIsStillPerchableWhenControlled, which
// stood here until 2026-08-24 and asserted the opposite.
//
// That test pinned the migration rule at the perch: an actor with no kind but
// a controller was a party member, so a spectator could ride it. The rule
// existed to keep logs written before the kind field behaving as they had,
// there are none, and it is deleted — an absent kind is NOT a party member,
// always, and nothing reads controller_ids to decide.
//
// THE PERCH IS WHERE THAT MATTERS MOST, which is why the inverted test stays
// rather than being dropped: §3.1.1 calls this the constraint the whole idea
// rests on. A spectator perched on the Goblin Archer watches the ambush from
// inside it, and "somebody controls it" was one of the two ways to become
// perchable without anyone saying what the actor was.
func TestAnActorWithNoDeclaredKindIsNoShoulderHoweverManyHoldIt(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wisp", Name: "Wisp"}})
	if k := st.Actors["wisp"].GetKind(); k != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatalf("fixture check: the wisp must declare NO kind, got %v", k)
	}
	// A kindless grant, which the command boundary refuses and the fold still
	// accepts: the only remaining way to hold an actor while saying nothing.
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "wisp", ParticipantId: "p-2"})
	mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "wisp", ParticipantId: "dm-1"})

	if err := gateway.MayPerch(watcher(), "wisp", st); err == nil {
		t.Error("an actor that never said what it is is no shoulder to sit on, " +
			"however many participants hold it")
	}
	// The control, and it is load-bearing: a MayPerch that refused everything
	// would satisfy the assertion above while deleting the feature.
	if err := gateway.MayPerch(watcher(), "hero", st); err != nil {
		t.Errorf("a declared party member is still a shoulder to sit on: %v", err)
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
