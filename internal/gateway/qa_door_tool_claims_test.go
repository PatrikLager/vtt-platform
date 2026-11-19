package gateway_test

import (
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const qaDoorClaimNoSuchCharacter = "There is no such character."

func qaDoorClaimSeed(t *testing.T, f *gwFixture) {
	t.Helper()
	f.seedCellar(t)
	mustAppend(t, f.campaign, "qa-far-actor", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "qa-far", Name: "Farwalker", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}})
	mustAppend(t, f.campaign, "qa-far-token", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "qa-far-t", SceneId: "scn1", ActorId: "qa-far", Position: &vttv1.GridPosition{X: 3, Y: 6}}})
	mustAppend(t, f.campaign, "qa-ghost-actor", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "qa-ghost", Name: "Ghost", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}})
}

func qaDoorClaimCmd(open bool, actor string) *vttv1.ClientCommand {
	pos := &vttv1.GridPosition{X: 0, Y: 1}
	if open {
		return &vttv1.ClientCommand{RequestId: qaDoorClaimReqID(), Command: &vttv1.ClientCommand_OpenDoor{
			OpenDoor: &vttv1.OpenDoor{SceneId: "cellar", At: pos, ActorId: actor}}}
	}
	return &vttv1.ClientCommand{RequestId: qaDoorClaimReqID(), Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: "cellar", At: pos, ActorId: actor}}}
}

// qaDoorClaimNamedBy sends the command from the agent and returns the actor id the
// door event carries.
func qaDoorClaimNamedBy(t *testing.T, f *gwFixture, cmd *vttv1.ClientCommand, open bool) string {
	t.Helper()
	conn := f.dial(f.agentToken, f.head(t))
	defer conn.Close(websocket.StatusNormalClosure, "")
	accepted(t, conn, cmd)
	for _, e := range drainEvents(t, conn, 300*time.Millisecond) {
		if open && e.GetDoorOpened() != nil {
			return e.GetDoorOpened().GetActorId()
		}
		if !open && e.GetDoorClosed() != nil {
			return e.GetDoorClosed().GetActorId()
		}
	}
	t.Fatalf("no door event reached the agent")
	return ""
}

// VTT-867
func TestQADoorToolClaimAgentNamesActorWithTokenFarAway(t *testing.T) {
	f := newGWFixture(t)
	qaDoorClaimSeed(t, f)
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(true, "qa-far"), true); got != "qa-far" {
		t.Fatalf("event names %q, want qa-far", got)
	}
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(false, "qa-far"), false); got != "qa-far" {
		t.Fatalf("close event names %q, want qa-far", got)
	}
}

// VTT-867
func TestQADoorToolClaimAgentNamesActorWithNoToken(t *testing.T) {
	f := newGWFixture(t)
	qaDoorClaimSeed(t, f)
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(true, "qa-ghost"), true); got != "qa-ghost" {
		t.Fatalf("event names %q, want qa-ghost", got)
	}
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(false, "qa-ghost"), false); got != "qa-ghost" {
		t.Fatalf("close event names %q, want qa-ghost", got)
	}
}

// VTT-867
func TestQADoorToolClaimAgentNamesPlayersPartyMember(t *testing.T) {
	f := newGWFixture(t)
	qaDoorClaimSeed(t, f)
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(true, "act-fighter"), true); got != "act-fighter" {
		t.Fatalf("event names %q, want act-fighter", got)
	}
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(false, "a1"), false); got != "a1" {
		t.Fatalf("close event names %q, want a1", got)
	}
}

// VTT-867
func TestQADoorToolClaimAgentNamesNoneFromNoToken(t *testing.T) {
	f := newGWFixture(t)
	qaDoorClaimSeed(t, f)
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(true, ""), true); got != "" {
		t.Fatalf("open event names %q, want none", got)
	}
	if got := qaDoorClaimNamedBy(t, f, qaDoorClaimCmd(false, ""), false); got != "" {
		t.Fatalf("close event names %q, want none", got)
	}
}

// VTT-868
func TestQADoorToolClaimAgentRefusedUnheldActorInExactWords(t *testing.T) {
	f := newGWFixture(t)
	qaDoorClaimSeed(t, f)
	conn := f.dial(f.agentToken, f.head(t))
	defer conn.Close(websocket.StatusNormalClosure, "")
	for _, open := range []bool{true, false} {
		got := f.refusedUnmoved(t, conn, qaDoorClaimCmd(open, "qa-nobody"))
		if got != qaDoorClaimNoSuchCharacter {
			t.Fatalf("open=%v refusal %q, want %q", open, got, qaDoorClaimNoSuchCharacter)
		}
	}
}

var qaDoorClaimSeq int

func qaDoorClaimReqID() string {
	qaDoorClaimSeq++
	return "qa-t6-" + string(rune('a'+qaDoorClaimSeq%26)) + string(rune('a'+qaDoorClaimSeq/26))
}
