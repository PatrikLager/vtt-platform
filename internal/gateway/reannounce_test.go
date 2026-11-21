package gateway_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const reannounceRule = "VTT-906"

func reannounceAdd(tb *qaPRTable, by *qaPRConn, actorID string) {
	tb.t.Helper()
	res := by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: actorID, Name: actorID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
	}})
	if !res.GetOk() {
		tb.t.Fatalf("%s: adding %s was refused: %q", by.label, actorID, res.GetError())
	}
}

func reannounceGrant(tb *qaPRTable, by *qaPRConn, actorID string, who qaPRWho) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{ActorId: actorID, ParticipantId: who.id,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
	}})
}

func reannounceRevoke(tb *qaPRTable, by *qaPRConn, actorID string, who qaPRWho) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{ActorId: actorID, ParticipantId: who.id},
	}})
}

func reannounceRemove(tb *qaPRTable, by *qaPRConn, actorID string) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_RemoveActor{
		RemoveActor: &vttv1.RemoveActor{ActorId: actorID},
	}})
}

func reannounceMust(tb *qaPRTable, what string, res *vttv1.CommandResult) {
	tb.t.Helper()
	if !res.GetOk() {
		tb.t.Fatalf("%s was refused: %q", what, res.GetError())
	}
}

// reannounceSince is what c was sent naming who between the arrivals of the
// fences from and to: fan-outs are serialised (SPEC-011), so it is exactly
// what was announced of who after from was asked for.
func reannounceSince(c *qaPRConn, from, to qaPRWho, who qaPRWho) []*vttv1.PresenceChanged {
	c.t.Helper()
	return qaPRNaming(c.ahead(to), who)[len(qaPRNaming(c.ahead(from), who)):]
}

// VTT-906
func TestAControlChangeIsAnnouncedToEveryUnrevokedConnectionNamingWhomItNames(t *testing.T) {
	cases := []struct {
		name string
		// role is the named participant's role at the mint.
		role identity.Role
		// held is whether the named participant controls pm-1 before the change.
		held bool
		// change makes the change under test; own is a connection of the named
		// participant.
		change func(tb *qaPRTable, dm, agent, own *qaPRConn, named qaPRWho) *vttv1.CommandResult
	}{
		{"a DM's grant", identity.RoleSpectator, false,
			func(tb *qaPRTable, dm, _, _ *qaPRConn, named qaPRWho) *vttv1.CommandResult {
				return reannounceGrant(tb, dm, "pm-1", named)
			}},
		{"an agent's grant", identity.RoleSpectator, false,
			func(tb *qaPRTable, _, agent, _ *qaPRConn, named qaPRWho) *vttv1.CommandResult {
				return reannounceGrant(tb, agent, "pm-1", named)
			}},
		{"a DM's revocation", identity.RolePlayer, true,
			func(tb *qaPRTable, dm, _, _ *qaPRConn, named qaPRWho) *vttv1.CommandResult {
				return reannounceRevoke(tb, dm, "pm-1", named)
			}},
		{"an agent's revocation", identity.RolePlayer, true,
			func(tb *qaPRTable, _, agent, _ *qaPRConn, named qaPRWho) *vttv1.CommandResult {
				return reannounceRevoke(tb, agent, "pm-1", named)
			}},
		{"a player's own revocation", identity.RolePlayer, true,
			func(tb *qaPRTable, _, _, own *qaPRConn, named qaPRWho) *vttv1.CommandResult {
				return reannounceRevoke(tb, own, "pm-1", named)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb := qaPRNewTable(t)
			dm := tb.host()
			agent := tb.connect(tb.mint("Ag", identity.RoleAgent), "the agent")
			reannounceAdd(tb, dm, "pm-1")
			named := tb.mint("Tam", tc.role)
			if tc.held {
				reannounceMust(tb, "the grant before the change", reannounceGrant(tb, dm, "pm-1", named))
			}
			own := []*qaPRConn{
				tb.connect(named, "the named participant's first device"),
				tb.connect(named, "the named participant's second device"),
			}
			everyone := []*qaPRConn{
				dm, agent,
				tb.connect(tb.mint("Wren", identity.RolePlayer), "a player"),
				tb.connect(tb.mint("Sy", identity.RoleSpectator), "a spectator"),
				own[0], own[1],
			}
			rook := tb.mint("Rook", identity.RolePlayer)
			lost := tb.connect(rook, "the revoked watcher")
			from := tb.fence(dm, append(everyone, lost)...)
			tb.revoke(rook, lost)

			reannounceMust(tb, tc.name, tc.change(tb, dm, agent, own[0], named))
			to := tb.fence(dm, everyone...)

			want := &vttv1.PresenceChanged{ParticipantId: named.id, DisplayName: named.name, State: qaPRHere}
			for _, c := range everyone {
				if got := reannounceSince(c, from, to, named); len(got) != 1 || !proto.Equal(got[0], want) {
					t.Errorf("%s: after %s, %s was sent %v naming %s, want exactly %v", reannounceRule, tc.name, c.label, got, named.name, want)
				}
			}
			// An append may close it first, on its next delivered event (VTT-032).
			_ = lost.send(tb.viewpoint())
			if frames, _ := lost.end(); len(qaPRNaming(frames, named)) != 0 {
				t.Errorf("VTT-032: the revoked watcher was sent %v naming %s", qaPRNaming(frames, named), named.name)
			}
		})
	}
}

// VTT-906
func TestARemovedActorIsAnnouncedNamingEachOfItsControllers(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	reannounceAdd(tb, dm, "pm-1")
	lera, ivo := tb.mint("Lera", identity.RoleSpectator), tb.mint("Ivo", identity.RoleSpectator)
	reannounceMust(tb, "the grant to Lera", reannounceGrant(tb, dm, "pm-1", lera))
	reannounceMust(tb, "the grant to Ivo", reannounceGrant(tb, dm, "pm-1", ivo))
	everyone := []*qaPRConn{dm, tb.connect(lera, "Lera"), tb.connect(ivo, "Ivo"),
		tb.connect(tb.mint("Wren", identity.RolePlayer), "a player")}
	from := tb.fence(dm, everyone...)

	reannounceMust(tb, "the removal of pm-1", reannounceRemove(tb, dm, "pm-1"))
	to := tb.fence(dm, everyone...)

	for _, who := range []qaPRWho{lera, ivo} {
		want := &vttv1.PresenceChanged{ParticipantId: who.id, DisplayName: who.name, State: qaPRHere}
		for _, c := range everyone {
			if got := reannounceSince(c, from, to, who); len(got) != 1 || !proto.Equal(got[0], want) {
				t.Errorf("%s: after the removal of an actor %s controlled, %s was sent %v naming them, want exactly %v", reannounceRule, who.name, c.label, got, want)
			}
		}
	}
}

// VTT-907
func TestAControlChangeOfAParticipantWithNoConnectionIsAnnouncedToNobody(t *testing.T) {
	for _, gone := range []bool{false, true} {
		name := "never connected"
		if gone {
			name = "hung up"
		}
		t.Run(name, func(t *testing.T) {
			tb := qaPRNewTable(t)
			dm := tb.host()
			reannounceAdd(tb, dm, "pm-1")
			witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
			named := tb.mint("Tam", identity.RoleSpectator)
			if gone {
				tb.connect(named, "the named participant").hangUp()
				witness.awaitTold(named, qaPRGone, 1)
			}
			from := tb.fence(dm, dm, witness)

			reannounceMust(tb, "the grant", reannounceGrant(tb, dm, "pm-1", named))
			to := tb.fence(dm, dm, witness)

			for _, c := range []*qaPRConn{dm, witness} {
				if got := reannounceSince(c, from, to, named); len(got) != 0 {
					t.Errorf("VTT-907: after a grant to a participant with no connection, %s was sent %v naming them", c.label, got)
				}
			}
		})
	}
}

// VTT-924
func TestARevocationTheFoldRefusesAnnouncesNothing(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	named := tb.mint("Tam", identity.RoleSpectator)
	everyone := []*qaPRConn{dm, tb.connect(named, "the named participant"),
		tb.connect(tb.mint("Wren", identity.RolePlayer), "a player")}
	from := tb.fence(dm, everyone...)

	if res := reannounceRevoke(tb, dm, "no-such-actor", named); res.GetOk() {
		t.Fatalf("VTT-924: the DM's revocation of an actor the log does not hold was accepted")
	}
	to := tb.fence(dm, everyone...)

	for _, c := range everyone {
		if got := reannounceSince(c, from, to, named); len(got) != 0 {
			t.Errorf("VTT-924: after a refused revocation, %s was sent %v naming %s", c.label, got, named.name)
		}
	}
}
