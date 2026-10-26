package gateway_test

import (
	"errors"
	"testing"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func speakingWorld() *engine.State {
	st := engine.NewState()
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	other := vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	for _, a := range []*vttv1.Actor{
		{ActorId: "a1", Name: "Hero", Kind: party, ControllerIds: []string{"p-1"}},
		{ActorId: "a2", Name: "Rival", Kind: party, ControllerIds: []string{"p-2"}},
		{ActorId: "a3", Name: "Lurker", Kind: other, ControllerIds: []string{"p-1"}},
		{ActorId: "a4", Kind: party, ControllerIds: []string{"p-1"}},
	} {
		a.ControllerId = a.ControllerIds[0]
		st.Actors[a.ActorId] = a
	}
	return st
}

func sayCmd(actorID, as string, ooc bool) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{
			Text: "a line", As: as, ActorId: actorID, OutOfCharacter: ooc,
		},
	}}
}

func speaker(id string, role identity.Role) *identity.Participant {
	return &identity.Participant{ID: id, Name: "Ada", Role: role}
}

func mustRefuse(t *testing.T, err error, what string) {
	t.Helper()
	if !errors.Is(err, gateway.ErrUnauthorized) {
		t.Fatalf("%s: err = %v, want ErrUnauthorized", what, err)
	}
}

func mustAccept(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: refused: %v", what, err)
	}
}

// VTT-495
func TestAPlayerSpeaksAsTheirOwnPartyMemberAndNoOtherActor(t *testing.T) {
	st, p := speakingWorld(), speaker("p-1", identity.RolePlayer)
	mustAccept(t, gateway.Authorize(p, sayCmd("a1", "", false), st), "as Hero")
	mustRefuse(t, gateway.Authorize(p, sayCmd("a2", "", false), st), "as Rival")
}

// VTT-495 VTT-506
func TestAPlayersLineThatSpeaksAsNoPartyMemberOfTheirsIsRefused(t *testing.T) {
	st, p := speakingWorld(), speaker("p-1", identity.RolePlayer)
	mustRefuse(t, gateway.Authorize(p, sayCmd("", "DM", false), st), "as DM")
	mustRefuse(t, gateway.Authorize(p, sayCmd("", "", false), st), "nameless")
	absent := gateway.Authorize(p, sayCmd("nobody", "", false), st)
	mustRefuse(t, absent, "as an absent id")
	creature := gateway.Authorize(p, sayCmd("a3", "", false), st)
	mustRefuse(t, creature, "as their own creature")
	if creature.Error() != absent.Error() {
		t.Fatalf("a creature's refusal %q differs from an absent id's %q",
			creature, absent)
	}
}

// VTT-495
func TestAPlayerCannotSpeakAsAPartyMemberWithNoName(t *testing.T) {
	st, p := speakingWorld(), speaker("p-1", identity.RolePlayer)
	mustRefuse(t, gateway.Authorize(p, sayCmd("a4", "", false), st), "as a4")
}

// VTT-495 VTT-499
func TestALineOutOfCharacterSpeaksAsNoActor(t *testing.T) {
	st := speakingWorld()
	for _, p := range []*identity.Participant{
		speaker("p-3", identity.RolePlayer),
		speaker("dm", identity.RoleDM),
		speaker("agent", identity.RoleAgent),
	} {
		mustAccept(t, gateway.Authorize(p, sayCmd("", "", true), st),
			string(p.Role)+" out of character")
	}
	for _, p := range []*identity.Participant{
		speaker("p-1", identity.RolePlayer),
		speaker("dm", identity.RoleDM),
		speaker("agent", identity.RoleAgent),
	} {
		mustRefuse(t, gateway.Authorize(p, sayCmd("a1", "", true), st),
			string(p.Role)+" out of character as Hero")
	}
}

// VTT-499
func TestTheDMAndTheAgentVoiceAnyoneByAsAndNoActorByID(t *testing.T) {
	st := speakingWorld()
	for _, r := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := speaker("x", r)
		mustRefuse(t, gateway.Authorize(p, sayCmd("a1", "", false), st),
			string(r)+" as Hero by id")
		mustAccept(t, gateway.Authorize(p, sayCmd("", "Goblin chief", false), st),
			string(r)+" as Goblin chief")
	}
}

func nextNarration(t *testing.T, conn *websocket.Conn) *vttv1.NarrationAdded {
	t.Helper()
	for range 64 {
		if n := readEvent(t, conn).GetNarrationAdded(); n != nil {
			return n
		}
	}
	t.Fatal("no NarrationAdded among 64 events")
	return nil
}

func speak(t *testing.T, conn *websocket.Conn, cmd *vttv1.ClientCommand) {
	t.Helper()
	sendCommand(t, conn, cmd)
	if r := readResult(t, conn); !r.Ok {
		t.Fatalf("refused: %q", r.Error)
	}
}

// VTT-496 VTT-503
func TestAPlayersLineIsAppendedUnderTheirPartyMembersName(t *testing.T) {
	f := newGWFixture(t)
	agentConn := f.dial(f.agentToken, f.head(t))
	speak(t, f.dial(f.playerToken, gwSeedHead), sayCmd("a1", "Someone Else", false))
	n := nextNarration(t, agentConn)
	if n.GetAs() != "Hero" || n.GetOutOfCharacter() {
		t.Fatalf("appended as %q, out of character %v; want Hero, in character",
			n.GetAs(), n.GetOutOfCharacter())
	}
}

// VTT-497 VTT-503
func TestALineOutOfCharacterIsAppendedUnderItsSendersName(t *testing.T) {
	f := newGWFixture(t)
	agentConn := f.dial(f.agentToken, f.head(t))
	for _, c := range []struct{ token, want string }{
		{f.playerToken, "Lera"},
		{f.dmToken, "DM"},
		{f.agentToken, "Agent"},
	} {
		conn := agentConn
		if c.token != f.agentToken {
			conn = f.dial(c.token, f.head(t))
		}
		speak(t, conn, sayCmd("", "Hero", true))
		n := nextNarration(t, agentConn)
		if n.GetAs() != c.want || !n.GetOutOfCharacter() {
			t.Fatalf("appended as %q, out of character %v; want %s, out of character",
				n.GetAs(), n.GetOutOfCharacter(), c.want)
		}
	}
}

// VTT-497
func TestACharacterLogKeepsALineOutOfCharactersMarkAndName(t *testing.T) {
	f := newGWFixture(t)
	playerConn := f.dial(f.playerToken, gwSeedHead)
	speak(t, playerConn, sayCmd("", "Hero", true))
	n := nextNarration(t, playerConn)
	if n.GetAs() != "Lera" || !n.GetOutOfCharacter() {
		t.Fatalf("sent as %q, out of character %v; want Lera, out of character",
			n.GetAs(), n.GetOutOfCharacter())
	}
}

// VTT-498 VTT-503
func TestTheDMsAndTheAgentsLineKeepsTheAsTheyGave(t *testing.T) {
	f := newGWFixture(t)
	agentConn := f.dial(f.agentToken, f.head(t))
	dmConn := f.dial(f.dmToken, f.head(t))
	for _, conn := range []*websocket.Conn{dmConn, agentConn} {
		for _, as := range []string{"Goblin chief", ""} {
			speak(t, conn, sayCmd("", as, false))
			if n := nextNarration(t, agentConn); n.GetAs() != as || n.GetOutOfCharacter() {
				t.Fatalf("appended as %q, out of character %v; want %q, in character",
					n.GetAs(), n.GetOutOfCharacter(), as)
			}
		}
	}
}
