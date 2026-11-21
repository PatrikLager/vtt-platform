package gateway_test

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	granteeParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	granteeNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func granteeAdd(tb *qaPRTable, by *qaPRConn, actorID, name string, kind vttv1.ActorKind) {
	tb.t.Helper()
	res := by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: actorID, Name: name, Kind: kind}},
	}})
	if !res.GetOk() {
		tb.t.Fatalf("%s: adding %s was refused: %q", by.label, actorID, res.GetError())
	}
}

func granteeGrant(tb *qaPRTable, by *qaPRConn, actorID, participantID string, kind vttv1.ActorKind) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{ActorId: actorID, ParticipantId: participantID, Kind: kind},
	}})
}

// granteeHead is the sequence a narration by the DM lands at, so a test can
// tell that nothing was appended between two of them.
func granteeHead(tb *qaPRTable, dm *qaPRConn) int64 {
	tb.t.Helper()
	return tb.narrate(dm, "a beat passes").GetSequence()
}

// granteeRefused asserts the grant is refused with want and appends nothing.
func granteeRefused(t *testing.T, rule string, tb *qaPRTable, dm, by *qaPRConn, actorID, participantID string, kind vttv1.ActorKind, want string) {
	t.Helper()
	before := granteeHead(tb, dm)
	res := granteeGrant(tb, by, actorID, participantID, kind)
	if res.GetOk() || res.GetError() != want {
		t.Errorf("%s: %s's grant of %s was answered ok=%v %q, want refused %q", rule, by.label, actorID, res.GetOk(), res.GetError(), want)
	}
	if after := granteeHead(tb, dm); after != before+1 {
		t.Errorf("%s: %s's refused grant of %s appended: the next narration landed at %d, want %d", rule, by.label, actorID, after, before+1)
	}
}

func granteeAccepted(t *testing.T, rule string, tb *qaPRTable, by *qaPRConn, actorID, participantID string, kind vttv1.ActorKind) {
	t.Helper()
	if res := granteeGrant(tb, by, actorID, participantID, kind); !res.GetOk() {
		t.Errorf("%s: %s's grant of %s was refused: %q", rule, by.label, actorID, res.GetError())
	}
}

// VTT-894 VTT-895
func TestAGrantNamingAParticipantTheTableDoesNotHoldIsRefused(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	agent := tb.connect(tb.mint("Ag", identity.RoleAgent), "the agent")
	granteeAdd(tb, dm, "pm-1", "Ash", granteeParty)
	removed := tb.mint("Rook", identity.RoleSpectator)
	if err := tb.ids.Revoke(removed.id); err != nil {
		t.Fatal(err)
	}
	for _, by := range []*qaPRConn{dm, agent} {
		granteeRefused(t, "VTT-894", tb, dm, by, "pm-1", "never-minted", granteeParty, "There is no such participant.")
		granteeRefused(t, "VTT-895", tb, dm, by, "pm-1", removed.id, granteeParty, "There is no such participant.")
	}
}

// VTT-147 VTT-894
func TestAPartyMemberGrantToAnyoneTheTableHoldsIsAppended(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	agentWho := tb.mint("Ag", identity.RoleAgent)
	agent := tb.connect(agentWho, "the agent")
	dmWho := tb.mint("Dot", identity.RoleDM)
	player := tb.mint("Lera", identity.RolePlayer)
	spectator := tb.mint("Robin", identity.RoleSpectator)
	for i, who := range []qaPRWho{player, spectator, dmWho, agentWho} {
		actorID := "pm-" + string(rune('a'+i))
		granteeAdd(tb, dm, actorID, "Ash", granteeParty)
		granteeAccepted(t, "VTT-894", tb, dm, actorID, who.id, granteeParty)
		granteeAccepted(t, "VTT-894", tb, agent, actorID, who.id, granteeParty)
	}
}

// VTT-928
func TestAGrantIsJudgedForItsKindBeforeItsGrantee(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	granteeAdd(tb, dm, "pm-1", "Ash", granteeParty)
	res := granteeGrant(tb, dm, "pm-1", "never-minted", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)
	if res.GetOk() || !strings.Contains(res.GetError(), "kind") || res.GetError() == "There is no such participant." {
		t.Errorf("VTT-894: a kindless grant naming nobody was answered ok=%v %q, want the kind's refusal", res.GetOk(), res.GetError())
	}
}

// VTT-928
func TestAPlayersGrantIsRefusedByTheTableWhoeverItNames(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	granteeAdd(tb, dm, "pm-1", "Ash", granteeParty)
	player := tb.connect(tb.mint("Lera", identity.RolePlayer), "a player")
	res := granteeGrant(tb, player, "pm-1", "never-minted", granteeParty)
	if res.GetOk() || res.GetError() == "There is no such participant." {
		t.Errorf("VTT-894: a player's grant naming nobody was answered ok=%v %q, want the table's refusal, which says nothing of which ids exist", res.GetOk(), res.GetError())
	}
}

// VTT-913 VTT-915
func TestANonPartyGrantToAPlayerOrASpectatorIsRefused(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	agent := tb.connect(tb.mint("Ag", identity.RoleAgent), "the agent")
	granteeAdd(tb, dm, "np-1", "Gob", granteeNonParty)
	granteeAdd(tb, dm, "pm-1", "Ash", granteeParty)
	for _, who := range []qaPRWho{tb.mint("Robin", identity.RoleSpectator), tb.mint("Lera", identity.RolePlayer)} {
		for _, by := range []*qaPRConn{dm, agent} {
			granteeRefused(t, "VTT-913", tb, dm, by, "np-1", who.id, granteeNonParty, who.name+" can only be given a party member.")
			granteeRefused(t, "VTT-915", tb, dm, by, "pm-1", who.id, granteeNonParty, who.name+" can only be given a party member.")
		}
	}
}

// VTT-926 VTT-927
func TestANonPartyGrantOfAnActorAPlayerControlsIsRefused(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	agentWho := tb.mint("Ag", identity.RoleAgent)
	agent := tb.connect(agentWho, "the agent")
	dmWho := tb.mint("Dot", identity.RoleDM)
	lera, ivo := tb.mint("Lera", identity.RoleSpectator), tb.mint("Ivo", identity.RoleSpectator)
	rook := tb.mint("Rook", identity.RoleSpectator)
	granteeAdd(tb, dm, "pm-ash", "Ash", granteeParty)
	granteeAdd(tb, dm, "pm-shared", "Gobo", granteeParty)
	granteeAdd(tb, dm, "pm-unnamed", "", granteeParty)
	granteeAdd(tb, dm, "pm-gone", "Wisp", granteeParty)
	for _, g := range []struct {
		actor string
		who   qaPRWho
	}{{"pm-ash", lera}, {"pm-shared", ivo}, {"pm-shared", dmWho}, {"pm-shared", lera},
		{"pm-unnamed", ivo}, {"pm-gone", rook}, {"pm-gone", dmWho}} {
		granteeAccepted(t, "fixture", tb, dm, g.actor, g.who.id, granteeParty)
	}
	if err := tb.ids.Revoke(rook.id); err != nil {
		t.Fatal(err)
	}
	for _, by := range []struct {
		conn *qaPRConn
		self qaPRWho
	}{{dm, dmWho}, {agent, agentWho}} {
		granteeRefused(t, "VTT-927", tb, dm, by.conn, "pm-ash", by.self.id, granteeNonParty,
			"Lera controls Ash: revoke it from them before you make it a non-party actor.")
		granteeRefused(t, "VTT-927", tb, dm, by.conn, "pm-shared", by.self.id, granteeNonParty,
			"Ivo and Lera control Gobo: revoke it from them before you make it a non-party actor.")
		granteeRefused(t, "VTT-927", tb, dm, by.conn, "pm-unnamed", by.self.id, granteeNonParty,
			"Ivo controls pm-unnamed: revoke it from them before you make it a non-party actor.")
	}
	// VTT-926: an actor no participant but the DM and a removed one controls.
	granteeAccepted(t, "VTT-926", tb, dm, "pm-gone", dmWho.id, granteeNonParty)
}

// VTT-926
func TestANonPartyGrantOfAnActorNoPlayerControlsIsAppended(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	agentWho := tb.mint("Ag", identity.RoleAgent)
	agent := tb.connect(agentWho, "the agent")
	dmWho := tb.mint("Dot", identity.RoleDM)
	granteeAdd(tb, dm, "np-1", "Gob", granteeNonParty)
	granteeAdd(tb, dm, "pm-1", "Ash", granteeParty)
	granteeAccepted(t, "VTT-926", tb, dm, "np-1", dmWho.id, granteeNonParty)
	granteeAccepted(t, "VTT-926", tb, agent, "np-1", agentWho.id, granteeNonParty)
	granteeAccepted(t, "VTT-926", tb, dm, "pm-1", agentWho.id, granteeNonParty)
}
