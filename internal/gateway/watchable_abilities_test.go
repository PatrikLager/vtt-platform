package gateway_test

import (
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func abilitiesOf(t *testing.T, what string, fr *vttv1.ServerFrame) string {
	t.Helper()
	set := fr.GetWatchableChanged()
	if set == nil {
		t.Fatalf("%s: got %v, want a WatchableChanged", what, fr)
	}
	pairs := make([]string, len(set.GetActors()))
	for i, a := range set.GetActors() {
		pairs[i] = a.GetActorId() + ":" + strings.Join(a.GetAbilityIds(), ",")
	}
	return strings.Join(pairs, " ")
}

func requireAbilities(t *testing.T, what string, fr *vttv1.ServerFrame, want string) {
	t.Helper()
	if got := abilitiesOf(t, what, fr); got != want {
		t.Fatalf("%s: the set's ability ids are [%s], want [%s]", what, got, want)
	}
}

func (w *watchConn) openWithAbilities(want, viewpoint string) {
	w.t.Helper()
	if head := w.mustAny(); head.GetCatchUpHead() == nil {
		w.t.Fatalf("the first frame is %v, want the catch-up head", head)
	}
	requireAbilities(w.t, "the frame after the head", w.mustAny(), want)
	requireViewpoint(w.t, "the frame after the set", w.mustAny(), viewpoint)
}

func changeAbility(t *testing.T, dm *watchConn, cmd *vttv1.ClientCommand) {
	t.Helper()
	if r := dm.command(cmd); !r.GetOk() {
		t.Fatalf("the DM's %v was refused: %s", cmd.GetCommand(), r.GetError())
	}
}

func requireLastEntry(t *testing.T, what string, f *gwFixture, actorID string, held int, is func(*vttv1.Envelope) bool) {
	t.Helper()
	log := logOf(t, f, actorID)
	if len(log) != held+1 || !is(log[held]) {
		t.Fatalf("%s: %s's log grew from %d to %d entries, want one more, the change", what, actorID, held, len(log))
	}
}

// Keep a2's list out of byte order: the set must carry the sheet's order.
// VTT-731
func TestAPlayersSetCarriesTheAbilityIdsOfEachPartyMemberTheyControl(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a2", Name: "Quill", AbilityIds: []string{"heal", "dart"}}},
		&vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "pet", Name: "Pet", AbilityIds: []string{"bite"}}},
		&vttv1.ActorControlGranted{ActorId: "pet", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	dm := f.watch(f.dmToken, 0, "")
	changeAbility(t, dm, giveAbilityCmd("a1", "slash"))

	lera := f.watch(f.playerToken, 0, "")
	lera.openWithAbilities("a1:slash a2:heal,dart", "a1")
}

// VTT-731
func TestTheSetsOfASpectatorAndAPlayerWhoControlNoPartyMemberCarryNoAbilityIds(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a2", Name: "Quill", AbilityIds: []string{"heal"},
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	dm := f.watch(f.dmToken, 0, "")
	changeAbility(t, dm, giveAbilityCmd("a1", "slash"))

	for who, token := range map[string]string{"the spectator": f.spectatorToken, "Ivo": f.otherPlayerToken} {
		w := f.watch(token, 0, "")
		if head := w.mustAny(); head.GetCatchUpHead() == nil {
			t.Fatalf("%s: the first frame is %v, want the catch-up head", who, head)
		}
		requireAbilities(t, who+"'s opening set", w.mustAny(), "a1: a2:")
	}
}

// Keep a1 naming an ability: the spectator's set must carry the ids of the
// party member it controls alone.
// VTT-731
func TestASpectatorWhoControlsAPartyMemberIsSentItsAbilityIds(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "c1", Name: "Corin", AbilityIds: []string{"ward"}}},
		&vttv1.ActorControlGranted{ActorId: "c1", ParticipantId: participantID(t, f, f.spectatorToken),
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	dm := f.watch(f.dmToken, 0, "")
	changeAbility(t, dm, giveAbilityCmd("a1", "slash"))

	w := f.watch(f.spectatorToken, 0, "")
	w.openWithAbilities("a1: c1:ward", "")
	changeAbility(t, dm, giveAbilityCmd("c1", "dart"))
	requireAbilities(t, "the spectator's seat, after the give", w.mustAny(), "a1: c1:ward,dart")
	w.quietAll(300 * time.Millisecond)
}

// Keep a2 without a token: a1's log must be sent nothing of a2's change, so
// a1's seat is told by the set alone.
// VTT-731
func TestAChangeToAControlledPartyMemberSendsEachOfTheirSeatsANewSetBeforeItsEntry(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a2", Name: "Quill", AbilityIds: []string{"heal"}}},
		&vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	onA1 := f.watch(f.playerToken, 0, "")
	onA1.openWithAbilities("a1: a2:heal", "a1")
	onA1.requireEntries(f, "a1's log at the opening", "a1", 0)
	onA2 := f.watch(f.playerToken, 0, "a2")
	onA2.openWithAbilities("a1: a2:heal", "a2")
	onA2.requireEntries(f, "a2's log at the opening", "a2", 0)
	dm := f.watch(f.dmToken, 0, "")

	for _, c := range []struct {
		cmd  *vttv1.ClientCommand
		want string
		is   func(*vttv1.Envelope) bool
	}{
		{giveAbilityCmd("a2", "dart"), "a1: a2:heal,dart",
			func(e *vttv1.Envelope) bool { return e.GetAbilityGiven().GetAbilityId() == "dart" }},
		{takeAbilityCmd("a2", "heal"), "a1: a2:dart",
			func(e *vttv1.Envelope) bool { return e.GetAbilityTaken().GetAbilityId() == "heal" }},
	} {
		heldA1, heldA2 := len(logOf(t, f, "a1")), len(logOf(t, f, "a2"))
		changeAbility(t, dm, c.cmd)
		requireAbilities(t, "the seat on a2, after the change", onA2.mustAny(), c.want)
		requireLastEntry(t, "after the change", f, "a2", heldA2, c.is)
		onA2.requireEntries(f, "a2's log after its set", "a2", heldA2)
		requireAbilities(t, "the seat on a1, after the change", onA1.mustAny(), c.want)
		if got := len(logOf(t, f, "a1")); got != heldA1 {
			t.Fatalf("a1's log grew from %d to %d entries at a2's change, so its seat's silence shows nothing", heldA1, got)
		}
		onA1.quietAll(300 * time.Millisecond)
		onA2.quietAll(100 * time.Millisecond)
	}
}

// VTT-731
func TestAGiveToAnotherPlayersPartyMemberSendsThisPlayerNoSet(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, partyMemberOf("p2", participantID(t, f, f.otherPlayerToken))...)
	lera := f.watch(f.playerToken, 0, "")
	lera.openWithAbilities("a1:", "a1")
	lera.requireEntries(f, "a1's log at the opening", "a1", 0)
	ivo := f.watch(f.otherPlayerToken, 0, "")
	ivo.openWithAbilities("p2:", "p2")
	ivo.requireEntries(f, "p2's log at the opening", "p2", 0)
	dm := f.watch(f.dmToken, 0, "")

	held := len(logOf(t, f, "a1"))
	changeAbility(t, dm, giveAbilityCmd("p2", "dart"))
	requireAbilities(t, "Ivo's seat, after the give", ivo.mustAny(), "p2:dart")
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "the give changed nothing Lera controls"})
	lera.requireEntries(f, "a1's log after the give and a narration", "a1", held)
	lera.quietAll(300 * time.Millisecond)
}
