package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const msBecomesAPlayer = "A participant becomes a player when given a character."

func msGet(t *testing.T, tb *qaPRTable, path, token string, into any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, tb.base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", path, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("GET %s: %v (body %s)", path, err, body)
	}
}

// msRoles is the role GET /api/me answers who, and the role GET
// /api/participants answers the DM for who.
func msRoles(t *testing.T, tb *qaPRTable, dm, who qaPRWho) (me, roster string) {
	t.Helper()
	var self struct {
		ParticipantID string `json:"participantId"`
		Role          string `json:"role"`
	}
	msGet(t, tb, "/api/me", who.token, &self)
	if self.ParticipantID != who.id {
		t.Fatalf("/api/me answered %s for %s", self.ParticipantID, who.name)
	}
	var list []struct {
		ParticipantID string `json:"participantId"`
		Role          string `json:"role"`
	}
	msGet(t, tb, "/api/participants", dm.token, &list)
	roster = "absent"
	for _, p := range list {
		if p.ParticipantID == who.id {
			roster = p.Role
		}
	}
	return self.Role, roster
}

func msWantRoles(t *testing.T, rule string, tb *qaPRTable, dm, who qaPRWho, want string) {
	t.Helper()
	if me, roster := msRoles(t, tb, dm, who); me != want || roster != want {
		t.Errorf("%s: %s is answered %q by /api/me and %q by the roster, want %q by both", rule, who.name, me, roster, want)
	}
}

func msEvent(c *qaPRConn, seq int64) *vttv1.Envelope {
	c.t.Helper()
	var got *vttv1.Envelope
	c.await(fmt.Sprintf("event %d", seq), func(frames []*vttv1.ServerFrame, _ bool) bool {
		for _, f := range frames {
			if e := f.GetEvent(); e.GetSequence() == seq {
				got = e
			}
		}
		return got != nil
	})
	return got
}

func msFrames(c *qaPRConn) []*vttv1.ServerFrame {
	c.t.Helper()
	return c.await("its frames so far", func([]*vttv1.ServerFrame, bool) bool { return true })
}

func msNarrate(tb *qaPRTable, by *qaPRConn, actorID string) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: "a line", ActorId: actorID, OutOfCharacter: actorID == ""},
	}})
}

func msPerch(tb *qaPRTable, by *qaPRConn, actorID string) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID},
	}})
}

// msTable is a DM, an agent, Lera holding pm-a and pm-b, Ivo sharing pm-b, an
// unheld pm-c and Robin, a spectator who holds nothing.
type msTable struct {
	*qaPRTable
	dmWho, agentWho, lera, ivo, robin qaPRWho
	dm, agent                         *qaPRConn
}

func msNewTable(t *testing.T) *msTable {
	t.Helper()
	tb := &msTable{qaPRTable: qaPRNewTable(t)}
	tb.dmWho = tb.mint("Dee", identity.RoleDM)
	tb.dm = tb.connect(tb.dmWho, "the DM")
	tb.narrate(tb.dm, "the table is laid")
	tb.agentWho = tb.mint("Gus", identity.RoleAgent)
	tb.agent = tb.connect(tb.agentWho, "the agent")
	tb.lera = tb.mint("Lera", identity.RoleSpectator)
	tb.ivo = tb.mint("Ivo", identity.RoleSpectator)
	tb.robin = tb.mint("Robin", identity.RoleSpectator)
	granteeAdd(tb.qaPRTable, tb.dm, "pm-a", "Gobo", granteeParty)
	granteeAdd(tb.qaPRTable, tb.dm, "pm-b", "Ash", granteeParty)
	granteeAdd(tb.qaPRTable, tb.dm, "pm-c", "Bram", granteeParty)
	for _, g := range []struct {
		actor string
		who   qaPRWho
	}{{"pm-a", tb.lera}, {"pm-b", tb.lera}, {"pm-b", tb.ivo}} {
		granteeAccepted(t, "fixture", tb.qaPRTable, tb.dm, g.actor, g.who.id, granteeParty)
	}
	return tb
}

// VTT-889 VTT-890 VTT-891 VTT-892
func TestAGrantMakesASpectatorAPlayerOverTheWire(t *testing.T) {
	tb := msNewTable(t)
	robin := tb.connect(tb.robin, "Robin")
	msWantRoles(t, "VTT-889", tb.qaPRTable, tb.dmWho, tb.robin, "spectator")
	if res := msNarrate(tb.qaPRTable, robin, "pm-b"); res.GetOk() {
		t.Fatalf("VTT-891: Robin spoke as Ash before any grant")
	}

	granteeAccepted(t, "the grant", tb.qaPRTable, tb.dm, "pm-b", tb.robin.id, granteeParty)
	msWantRoles(t, "VTT-889 VTT-890", tb.qaPRTable, tb.dmWho, tb.robin, "player")
	msWantRoles(t, "VTT-890", tb.qaPRTable, tb.dmWho, tb.lera, "player")
	msWantRoles(t, "VTT-890", tb.qaPRTable, tb.dmWho, tb.ivo, "player")

	res := msNarrate(tb.qaPRTable, robin, "pm-b")
	if !res.GetOk() {
		t.Fatalf("VTT-891: Robin, granted Ash, was refused a line as Ash: %q", res.GetError())
	}
	ev := msEvent(tb.dm, res.GetSequence())
	if ev.GetParticipantId() != tb.robin.id || ev.GetActorRole() != string(identity.RolePlayer) ||
		ev.GetNarrationAdded().GetAs() != "Ash" {
		t.Errorf("VTT-892: Robin's line was appended as %v, want hers, stamped player, as Ash", ev)
	}
}

// VTT-893 VTT-921
func TestAGrantMovesAPerchedSpectatorOntoTheirOwnPartyMember(t *testing.T) {
	tb := msNewTable(t)
	robin := tb.connect(tb.robin, "Robin")
	if res := msPerch(tb.qaPRTable, robin, "pm-b"); !res.GetOk() {
		t.Fatalf("fixture: Robin's perch on Lera's Ash was refused: %q", res.GetError())
	}
	robin.await("the perch on pm-b", func(frames []*vttv1.ServerFrame, _ bool) bool {
		return slices.ContainsFunc(frames, func(f *vttv1.ServerFrame) bool {
			return f.GetViewpointChanged() != nil && f.GetViewpointChanged().GetActorId() == "pm-b"
		})
	})
	mark := len(msFrames(robin))

	granteeAccepted(t, "the grant", tb.qaPRTable, tb.dm, "pm-c", tb.robin.id, granteeParty)
	frames := robin.await("the move onto pm-c", func(frames []*vttv1.ServerFrame, _ bool) bool {
		return slices.ContainsFunc(frames[mark:], func(f *vttv1.ServerFrame) bool {
			return f.GetViewpointChanged() != nil
		})
	})[mark:]
	moved := slices.IndexFunc(frames, func(f *vttv1.ServerFrame) bool { return f.GetViewpointChanged() != nil })
	if got := frames[moved].GetViewpointChanged().GetActorId(); got != "pm-c" {
		t.Fatalf("VTT-921: granted Bram, Robin's connection moved to %q, want pm-c", got)
	}
	var set *vttv1.WatchableChanged
	for _, f := range frames[:moved] {
		if w := f.GetWatchableChanged(); w != nil {
			set = w
		}
	}
	if len(set.GetActors()) != 1 || set.GetActors()[0].GetActorId() != "pm-c" || set.GetControlsNoPartyMember() {
		t.Errorf("VTT-893: before the move Robin's connection was told %v, want Bram alone", set)
	}
}

// VTT-889 VTT-890 VTT-891
func TestTheLastRevocationMakesAPlayerASpectator(t *testing.T) {
	const refusal = `gateway: not authorized: role "spectator" may not issue "add_narration"`
	for _, c := range []struct {
		name string
		last func(tb *msTable, robin *qaPRConn) *vttv1.CommandResult
	}{
		{"by the DM", func(tb *msTable, _ *qaPRConn) *vttv1.CommandResult {
			return reannounceRevoke(tb.qaPRTable, tb.dm, "pm-c", tb.robin)
		}},
		{"by the agent", func(tb *msTable, _ *qaPRConn) *vttv1.CommandResult {
			return reannounceRevoke(tb.qaPRTable, tb.agent, "pm-c", tb.robin)
		}},
		{"by herself", func(tb *msTable, robin *qaPRConn) *vttv1.CommandResult {
			return reannounceRevoke(tb.qaPRTable, robin, "pm-c", tb.robin)
		}},
		{"by remove_actor", func(tb *msTable, _ *qaPRConn) *vttv1.CommandResult {
			return reannounceRemove(tb.qaPRTable, tb.dm, "pm-c")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := msNewTable(t)
			granteeAccepted(t, "fixture", tb.qaPRTable, tb.dm, "pm-b", tb.robin.id, granteeParty)
			granteeAccepted(t, "fixture", tb.qaPRTable, tb.dm, "pm-c", tb.robin.id, granteeParty)
			robin := tb.connect(tb.robin, "Robin")
			reannounceMust(tb.qaPRTable, "the first revocation", reannounceRevoke(tb.qaPRTable, tb.dm, "pm-b", tb.robin))
			msWantRoles(t, "VTT-889 VTT-890", tb.qaPRTable, tb.dmWho, tb.robin, "player")
			if res := msNarrate(tb.qaPRTable, robin, ""); !res.GetOk() {
				t.Fatalf("VTT-891: Robin, holding Bram, was refused a line out of character: %q", res.GetError())
			}

			reannounceMust(tb.qaPRTable, "the last revocation", c.last(tb, robin))
			msWantRoles(t, "VTT-889 VTT-890", tb.qaPRTable, tb.dmWho, tb.robin, "spectator")
			if res := msNarrate(tb.qaPRTable, robin, ""); res.GetOk() || res.GetError() != refusal {
				t.Errorf("VTT-891: after her last revocation Robin's line out of character was answered ok=%v %q, want %q",
					res.GetOk(), res.GetError(), refusal)
			}
		})
	}
}

// VTT-896 VTT-897 VTT-898 VTT-906 VTT-923
func TestMakeSpectatorRevokesEveryActorThePlayerControlsInActorIDOrder(t *testing.T) {
	for _, by := range []string{"the DM", "the agent"} {
		t.Run(by, func(t *testing.T) {
			tb := msNewTable(t)
			granteeAdd(tb.qaPRTable, tb.dm, "pm-0", "Cole", granteeParty)
			granteeAccepted(t, "fixture", tb.qaPRTable, tb.dm, "pm-0", tb.lera.id, granteeParty)
			issuer, issuerWho := tb.dm, tb.dmWho
			if by == "the agent" {
				issuer, issuerWho = tb.agent, tb.agentWho
			}
			lera := tb.connect(tb.lera, "Lera")
			mark := len(msFrames(lera))
			before := granteeHead(tb.qaPRTable, tb.dm)

			res := tb.promote(issuer, tb.lera, identity.RoleSpectator)
			if !res.GetOk() || res.GetSequence() != before+1 {
				t.Fatalf("VTT-898: making Lera a spectator answered ok=%v %q at sequence %d, want ok at %d",
					res.GetOk(), res.GetError(), res.GetSequence(), before+1)
			}
			for i, actor := range []string{"pm-0", "pm-a", "pm-b"} {
				ev := msEvent(tb.dm, before+1+int64(i))
				rv := ev.GetActorControlRevoked()
				if rv.GetActorId() != actor || rv.GetParticipantId() != tb.lera.id {
					t.Errorf("VTT-896: event %d is %v, want %s's revocation naming Lera", before+1+int64(i), ev, actor)
				}
				if ev.GetParticipantId() != issuerWho.id || ev.GetActorRole() != string(issuerWho.role) {
					t.Errorf("VTT-897: event %d is stamped %q as %q, want %s's id as %q",
						before+1+int64(i), ev.GetParticipantId(), ev.GetActorRole(), by, issuerWho.role)
				}
			}
			if after := granteeHead(tb.qaPRTable, tb.dm); after != before+4 {
				t.Errorf("VTT-896: the next narration landed at %d, want %d: the batch is three revocations and nothing else", after, before+4)
			}
			msWantRoles(t, "VTT-896", tb.qaPRTable, tb.dmWho, tb.lera, "spectator")
			msWantRoles(t, "VTT-896", tb.qaPRTable, tb.dmWho, tb.ivo, "player")

			fence := tb.fence(issuer, lera)
			qaPRWantTold(t, "VTT-906", lera, lera.ahead(fence)[mark:], tb.lera, qaPRHere)
			var set *vttv1.WatchableChanged
			for _, f := range lera.ahead(fence)[mark:] {
				if f.GetViewpointChanged() != nil {
					t.Errorf("VTT-923: made a spectator, Lera's connection was moved: %v", f)
				}
				if w := f.GetWatchableChanged(); w != nil {
					set = w
				}
			}
			if !set.GetControlsNoPartyMember() || len(set.GetActors()) != 4 {
				t.Errorf("VTT-896: made a spectator, Lera's connection was told %v, want every party member and that she controls none", set)
			}
		})
	}
}

// VTT-896 VTT-898
func TestMakeSpectatorOfAParticipantWhoControlsNothingAppendsNothing(t *testing.T) {
	tb := msNewTable(t)
	for _, issuer := range []*qaPRConn{tb.dm, tb.agent} {
		before := granteeHead(tb.qaPRTable, tb.dm)
		res := tb.promote(issuer, tb.robin, identity.RoleSpectator)
		if !res.GetOk() || res.GetSequence() != 0 {
			t.Errorf("VTT-898: %s making Robin, who controls nothing, a spectator answered ok=%v %q at sequence %d, want ok with none",
				issuer.label, res.GetOk(), res.GetError(), res.GetSequence())
		}
		if after := granteeHead(tb.qaPRTable, tb.dm); after != before+1 {
			t.Errorf("VTT-896: %s's promotion of Robin appended: the next narration landed at %d, want %d", issuer.label, after, before+1)
		}
	}
	msWantRoles(t, "VTT-896", tb.qaPRTable, tb.dmWho, tb.robin, "spectator")
}

// VTT-899 VTT-900
func TestAPromotionToAnyRoleButSpectatorIsRefusedAndAppendsNothing(t *testing.T) {
	tb := msNewTable(t)
	never := qaPRWho{id: "never-minted", name: "Nobody"}
	for _, issuer := range []*qaPRConn{tb.dm, tb.agent} {
		for _, who := range []qaPRWho{tb.robin, tb.lera, never} {
			before := granteeHead(tb.qaPRTable, tb.dm)
			if res := tb.promote(issuer, who, identity.RolePlayer); res.GetOk() || res.GetError() != msBecomesAPlayer {
				t.Errorf("VTT-900: %s promoting %s to player was answered ok=%v %q, want %q",
					issuer.label, who.name, res.GetOk(), res.GetError(), msBecomesAPlayer)
			}
			for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
				if res := tb.promote(issuer, who, role); res.GetOk() {
					t.Errorf("VTT-899: %s promoting %s to %s was accepted", issuer.label, who.name, role)
				}
			}
			if after := granteeHead(tb.qaPRTable, tb.dm); after != before+1 {
				t.Errorf("VTT-899: %s's refused promotions of %s appended: the next narration landed at %d, want %d",
					issuer.label, who.name, after, before+1)
			}
		}
	}
	msWantRoles(t, "VTT-899", tb.qaPRTable, tb.dmWho, tb.robin, "spectator")
	msWantRoles(t, "VTT-899", tb.qaPRTable, tb.dmWho, tb.lera, "player")
}

// VTT-691
func TestASpectatorGrantedAPartyMemberReadsItsAbilities(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-robin", identity.RoleSpectator)
	q.qaReadInvite("qa-lera", identity.RoleSpectator)
	q.qaReadAddActor("qa-ash", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadAddActor("qa-bram", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "staggering-blow", "crossbow-shot")
	q.qaReadGrant("qa-ash", "qa-lera", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-bram", "qa-lera", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-robin")
	q.qaReadGrant("qa-bram", "qa-robin", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-robin", "crossbow-shot", "staggering-blow")
	q.qaReadRevoke("qa-bram", "qa-robin")
	q.qaReadWantAbilities("qa-robin")
}
