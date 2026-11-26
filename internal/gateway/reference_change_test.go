package gateway_test

import (
	"fmt"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func addReferenceCmd(actorID, id, entry, place, state string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddReference{AddReference: &vttv1.AddReference{
		ActorId: actorID, ReferenceId: id, Reference: &vttv1.Reference{EntryId: entry, Place: place, State: state}}}}
}

func changeReferenceCmd(actorID, id, place, state string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_ChangeReference{ChangeReference: &vttv1.ChangeReference{
		ActorId: actorID, ReferenceId: id, Place: place, State: state}}}
}

func removeReferenceCmd(actorID, id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveReference{RemoveReference: &vttv1.RemoveReference{
		ActorId: actorID, ReferenceId: id}}}
}

// Keep a1 the actor p-1 controls: the role table must refuse a player who
// controls the very actor named, before any player rule is asked.
// VTT-1049
func TestOnlyTheDMAndTheAgentMayAddChangeOrRemoveAReference(t *testing.T) {
	st := ownershipFixture()
	for _, name := range []string{"add_reference", "change_reference", "remove_reference"} {
		for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator} {
			err := gateway.Authorize(&identity.Participant{ID: authzCaseID(role), Role: role}, commandFor(t, name), st)
			if role == identity.RoleDM || role == identity.RoleAgent {
				if err != nil {
					t.Errorf("%s: %s refused: %v", role, name, err)
				}
				continue
			}
			want := fmt.Sprintf("gateway: not authorized: role %q may not issue %q", role, name)
			if err == nil || err.Error() != want {
				t.Errorf("%s: %s answered %v, want %q", role, name, err, want)
			}
		}
	}
}

// Keep a2 without a token: a1's log is sent nothing of a2's sheet, so a1's
// seat could be told of a change by a set alone.
// VTT-1031
func TestNoReferenceCommandSendsAPlayerANewWatchableSet(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a2", Name: "Quill"}},
		&vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	onA1 := f.watch(f.playerToken, 0, "")
	onA1.openWithSet("a1:Hero a2:Quill", "a1")
	onA1.requireEntries(f, "a1's log at the opening", "a1", 0)
	onA2 := f.watch(f.playerToken, 0, "a2")
	onA2.openWithSet("a1:Hero a2:Quill", "a2")
	onA2.requireEntries(f, "a2's log at the opening", "a2", 0)
	dm := f.watch(f.dmToken, 0, "")

	for _, c := range []struct {
		cmd *vttv1.ClientCommand
		is  func(*vttv1.Envelope) bool
	}{
		{addReferenceCmd("a2", "r", "echo", "mike", "lima"),
			func(e *vttv1.Envelope) bool { return e.GetReferenceAdded().GetReferenceId() == "r" }},
		{changeReferenceCmd("a2", "r", "kilo", ""),
			func(e *vttv1.Envelope) bool { return e.GetReferenceChanged().GetPlace() == "kilo" }},
		{removeReferenceCmd("a2", "r"),
			func(e *vttv1.Envelope) bool { return e.GetReferenceRemoved().GetReferenceId() == "r" }},
	} {
		heldA1, heldA2 := len(logOf(t, f, "a1")), len(logOf(t, f, "a2"))
		changeAbility(t, dm, c.cmd)
		requireLastEntry(t, "after the change", f, "a2", heldA2, c.is)
		onA2.requireEntries(f, "a2's log after the change, with no set before it", "a2", heldA2)
		onA2.quietAll(300 * time.Millisecond)
		if got := len(logOf(t, f, "a1")); got != heldA1 {
			t.Fatalf("a1's log grew from %d to %d entries at a2's change, so its seat's silence shows nothing", heldA1, got)
		}
		onA1.quietAll(100 * time.Millisecond)
	}
}
