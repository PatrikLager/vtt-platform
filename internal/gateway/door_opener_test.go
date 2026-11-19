package gateway_test

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const noSuchCharacter = "There is no such character."

func doorBy(open bool, sq qaDSSquare, actor string) *vttv1.ClientCommand {
	at := &vttv1.GridPosition{X: sq.x, Y: sq.y}
	if open {
		return &vttv1.ClientCommand{RequestId: "door-by-open", Command: &vttv1.ClientCommand_OpenDoor{
			OpenDoor: &vttv1.OpenDoor{SceneId: sq.scene, At: at, ActorId: actor}}}
	}
	return &vttv1.ClientCommand{RequestId: "door-by-close", Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: sq.scene, At: at, ActorId: actor}}}
}

// doorTable is the cellar and the hall, whose door the player's runner stands
// beside, and an actor with no token.
func doorTable(t *testing.T) *gwFixture {
	t.Helper()
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	mustAppend(t, f.campaign, "loner", &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "loner", Name: "Loner"}}})
	return f
}

var hallDoor = qaDSSquare{"qa-hall", 1, 1}

// VTT-867
func TestTheDMAndTheAgentNameAnyActorOrNoneOnADoorFromAnywhere(t *testing.T) {
	f := doorTable(t)
	for who, tok := range map[string]string{"dm": f.dmToken, "agent": f.agentToken} {
		conn := f.dial(tok, f.head(t))
		for _, actor := range []string{"a1", "act-fighter", "loner", ""} {
			for _, open := range []bool{true, false} {
				accepted(t, conn, doorBy(open, hallDoor, actor))
				ev := readEvent(t, conn)
				got := ev.GetDoorOpened().GetActorId() + ev.GetDoorClosed().GetActorId()
				if (open && ev.GetDoorOpened() == nil) || (!open && ev.GetDoorClosed() == nil) || got != actor {
					t.Fatalf("%s naming %q (open %v): event %v, want the door event naming %q", who, actor, open, ev, actor)
				}
			}
		}
	}
}

// VTT-868
func TestADoorCommandNamingAnActorTheWorldLacksIsRefusedInPlainWordsForEveryRole(t *testing.T) {
	f := doorTable(t)
	watcher := f.dial(f.dmToken, f.head(t))
	for who, tok := range map[string]string{"dm": f.dmToken, "agent": f.agentToken, "player": f.playerToken} {
		conn := f.dial(tok, f.head(t))
		for _, ghost := range []string{"ghost", "hasOwnProperty"} {
			for _, open := range []bool{true, false} {
				if got := f.refusedUnmoved(t, conn, doorBy(open, hallDoor, ghost)); got != noSuchCharacter {
					t.Errorf("%s naming %q (open %v): refusal %q, want %q", who, ghost, open, got, noSuchCharacter)
				}
			}
		}
	}
	if n := qaDSDoorEvents(drainEvents(t, watcher, 300*time.Millisecond)); n != 0 {
		t.Errorf("a watcher received %d door events from refusals", n)
	}
}

// VTT-608 VTT-880
func TestAnAbsentActorIsAnsweredAfterTheIssuerAndTheSquare(t *testing.T) {
	f := doorTable(t)
	dm := f.dial(f.dmToken, f.head(t))
	if got := f.refusedUnmoved(t, dm, doorBy(true, qaDSSquare{"qa-hall", 2, 1}, "ghost")); got != qaDSNoDoor {
		t.Errorf("the DM at a floor naming an absent actor: refusal %q, want %q", got, qaDSNoDoor)
	}
	player := f.dial(f.playerToken, f.head(t))
	rule := f.refusedUnmoved(t, player, doorBy(true, qaDSSquare{"cellar", 0, 1}, ""))
	if far := f.refusedUnmoved(t, player, doorBy(true, qaDSSquare{"cellar", 0, 1}, "ghost")); far != rule {
		t.Errorf("a player far from the door naming an absent actor: refusal %q, want the door rule's %q", far, rule)
	}
}
