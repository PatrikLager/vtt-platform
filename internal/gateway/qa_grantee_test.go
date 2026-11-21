package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaGRNoSuch = "There is no such participant."
	qaGRTail   = ": revoke it from them before you make it a non-party actor."
	qaGROnly   = " can only be given a party member."
)

var (
	qaGRParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaGRNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	qaGRNoKind   = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
)

type qaGRWho struct {
	id, name, token string
}

type qaGRConn struct {
	t       *testing.T
	ws      *websocket.Conn
	results chan *vttv1.CommandResult
	next    int
	label   string
}

type qaGRTable struct {
	t     *testing.T
	camp  *campaign.Campaign
	ids   *identity.DB
	srv   *httptest.Server
	dm    qaGRWho
	agent qaGRWho
	zora  qaGRWho
	abel  qaGRWho
	ingr  qaGRWho
	sixt  qaGRWho
	dmc   *qaGRConn
	agc   *qaGRConn
	live  map[string]string
}

func qaGROpen(t *testing.T) *qaGRTable {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = camp.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = ids.Close()
		_ = camp.Close()
	})
	srv := httptest.NewServer(gateway.New(camp, ids).Handler())
	t.Cleanup(srv.Close)
	tb := &qaGRTable{t: t, camp: camp, ids: ids, srv: srv,
		live: map[string]string{}}
	tb.dm = tb.mint("Dagny", identity.RoleDM)
	tb.agent = tb.mint("Oracle", identity.RoleAgent)
	tb.zora, tb.abel = tb.mintAgainstIDOrder("Zora", "Abel")
	tb.ingr = tb.mint("Ingrid", identity.RoleSpectator)
	tb.sixt = tb.mint("Sixten", identity.RoleSpectator)
	tb.dmc = tb.dial(tb.dm, "dm")
	tb.agc = tb.dial(tb.agent, "agent")
	tb.addActor("actor-a", "Zephyr")
	tb.addActor("actor-b", "Moss")
	tb.addActor("actor-c", "Amber")
	tb.addActor("actor-d", "")
	tb.give("actor-a", tb.ingr.id)
	return tb
}

func (tb *qaGRTable) mint(name string, role identity.Role) qaGRWho {
	tb.t.Helper()
	tok, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("CreateInvite(%q): %v", name, err)
	}
	if role == identity.RolePlayer || role == identity.RoleSpectator {
		tb.live[id] = name
	}
	return qaGRWho{id: id, name: name, token: tok}
}

func (tb *qaGRTable) revoke(w qaGRWho) {
	tb.t.Helper()
	if err := tb.ids.Revoke(w.id); err != nil {
		tb.t.Fatalf("Revoke(%s): %v", w.name, err)
	}
	delete(tb.live, w.id)
}

func (tb *qaGRTable) mintAgainstIDOrder(hiName, loName string) (qaGRWho, qaGRWho) {
	tb.t.Helper()
	for i := 0; i < 64; i++ {
		var hi, lo qaGRWho
		if i%2 == 0 {
			hi = tb.mint(hiName, identity.RoleSpectator)
			lo = tb.mint(loName, identity.RoleSpectator)
		} else {
			lo = tb.mint(loName, identity.RoleSpectator)
			hi = tb.mint(hiName, identity.RoleSpectator)
		}
		if hi.id < lo.id {
			return hi, lo
		}
		tb.revoke(hi)
		tb.revoke(lo)
	}
	tb.t.Fatalf("could not mint %s and %s with ids sorting against names",
		hiName, loName)
	return qaGRWho{}, qaGRWho{}
}

func (tb *qaGRTable) dial(w qaGRWho, label string) *qaGRConn {
	tb.t.Helper()
	u := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" +
		url.QueryEscape(w.token)
	ws, _, err := websocket.Dial(context.Background(), u, nil)
	if err != nil {
		tb.t.Fatalf("dial as %s: %v", w.name, err)
	}
	ws.SetReadLimit(1 << 24)
	c := &qaGRConn{t: tb.t, ws: ws, label: label,
		results: make(chan *vttv1.CommandResult, 512)}
	go c.pump()
	tb.t.Cleanup(func() { _ = ws.CloseNow() })
	return c
}

func (c *qaGRConn) pump() {
	defer close(c.results)
	dec := protojson.UnmarshalOptions{DiscardUnknown: true}
	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			return
		}
		var f vttv1.ServerFrame
		if dec.Unmarshal(data, &f) != nil {
			continue
		}
		if r := f.GetResult(); r != nil {
			c.results <- r
		}
	}
}

func (c *qaGRConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	c.next++
	cmd.RequestId = fmt.Sprintf("qa-gr-%s-%d", c.label, c.next)
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatalf("write as %s: %v", c.label, err)
	}
	timeout := time.After(20 * time.Second)
	for {
		select {
		case r, ok := <-c.results:
			if !ok {
				c.t.Fatalf("%s's connection closed awaiting %s",
					c.label, cmd.RequestId)
			}
			if r.GetRequestId() == cmd.RequestId {
				return r
			}
		case <-timeout:
			c.t.Fatalf("%s: no result for %s", c.label, cmd.RequestId)
		}
	}
}

func (c *qaGRConn) narrate() int64 {
	c.t.Helper()
	r := c.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: "qa marker", OutOfCharacter: true}}})
	if !r.GetOk() || r.GetSequence() == 0 {
		c.t.Fatalf("%s's marker narration: ok=%v seq=%d err=%q",
			c.label, r.GetOk(), r.GetSequence(), r.GetError())
	}
	return r.GetSequence()
}

func qaGRGrant(actor, pid string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{
			ActorId: actor, ParticipantId: pid, Kind: kind}}}
}

func qaGRRevoke(actor, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{
			ActorId: actor, ParticipantId: pid}}}
}

func (tb *qaGRTable) addActor(id, name string) {
	tb.t.Helper()
	r := tb.dmc.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{
			ActorId: id, Name: name, Kind: qaGRParty}}}})
	if !r.GetOk() {
		tb.t.Fatalf("add_actor %s: %q", id, r.GetError())
	}
}

func (tb *qaGRTable) give(actor, pid string) {
	tb.t.Helper()
	r := tb.dmc.send(qaGRGrant(actor, pid, qaGRParty))
	if !r.GetOk() {
		tb.t.Fatalf("setup grant of %s to %s: %q", actor, pid, r.GetError())
	}
}

func (tb *qaGRTable) controllers(actor string) []string {
	tb.t.Helper()
	a, ok := tb.camp.Actors()[actor]
	if !ok {
		tb.t.Fatalf("the campaign holds no %s", actor)
	}
	return append([]string(nil), a.GetControllerIds()...)
}

func (tb *qaGRTable) kind(actor string) vttv1.ActorKind {
	tb.t.Helper()
	return tb.camp.Actors()[actor].GetKind()
}

func (tb *qaGRTable) controlsText(actor, label string) string {
	tb.t.Helper()
	var named []string
	for _, cid := range tb.controllers(actor) {
		if n, ok := tb.live[cid]; ok {
			named = append(named, n)
		}
	}
	if len(named) == 0 {
		tb.t.Fatalf("fixture: %s has no live player or spectator controller", actor)
	}
	verb := " controls "
	if len(named) > 1 {
		verb = " control "
	}
	return strings.Join(named, " and ") + verb + label + qaGRTail
}

func (tb *qaGRTable) refused(by, marker *qaGRConn, cmd *vttv1.ClientCommand) string {
	tb.t.Helper()
	before := marker.narrate()
	r := by.send(cmd)
	after := marker.narrate()
	if r.GetOk() {
		tb.t.Errorf("%s's %v was accepted (seq %d); want refused",
			by.label, cmd.GetCommand(), r.GetSequence())
	}
	if r.GetSequence() != 0 {
		tb.t.Errorf("refusal carries sequence %d", r.GetSequence())
	}
	if after != before+1 {
		tb.t.Errorf("%s's refused %v appended %d events (markers %d, %d)",
			by.label, cmd.GetCommand(), after-before-1, before, after)
	}
	return r.GetError()
}

func (tb *qaGRTable) accepted(by *qaGRConn, cmd *vttv1.ClientCommand) {
	tb.t.Helper()
	before := by.narrate()
	r := by.send(cmd)
	if !r.GetOk() {
		tb.t.Errorf("%s's %v refused with %q; want accepted",
			by.label, cmd.GetCommand(), r.GetError())
		return
	}
	if r.GetSequence() != before+1 {
		tb.t.Errorf("%s's accepted %v at sequence %d; want %d",
			by.label, cmd.GetCommand(), r.GetSequence(), before+1)
	}
}

func (tb *qaGRTable) issuers() []*qaGRConn { return []*qaGRConn{tb.dmc, tb.agc} }

func (tb *qaGRTable) heads() []qaGRWho { return []qaGRWho{tb.dm, tb.agent} }

func qaGRKinds() []vttv1.ActorKind {
	return []vttv1.ActorKind{qaGRParty, qaGRNonParty}
}

// VTT-894 VTT-895
func TestQAGRAGrantNamingAParticipantNeverMintedIsRefusedPlainly(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-c", tb.zora.id)
	ghosts := []string{"qa-gr-ghost", "3f2c1d9e-8b7a-4c6d-9e5f-0a1b2c3d4e5f"}
	for _, by := range tb.issuers() {
		for _, kind := range qaGRKinds() {
			for _, actor := range []string{"actor-b", "actor-c", "actor-d"} {
				for _, ghost := range ghosts {
					got := tb.refused(by, by, qaGRGrant(actor, ghost, kind))
					if got != qaGRNoSuch {
						t.Errorf("%s grants %s of %s to %q: got %q, want %q",
							by.label, kind, actor, ghost, got, qaGRNoSuch)
					}
					if strings.Contains(got, ghost) {
						t.Errorf("refusal %q echoes the id %q", got, ghost)
					}
				}
			}
		}
	}
	if c := tb.controllers("actor-c"); len(c) != 1 || c[0] != tb.zora.id {
		t.Errorf("actor-c's controllers became %v", c)
	}
	if c := tb.controllers("actor-b"); len(c) != 0 {
		t.Errorf("actor-b's controllers became %v", c)
	}
}

// VTT-894 VTT-895
func TestQAGRAGrantNamingARemovedParticipantIsRefusedWithTheSameText(t *testing.T) {
	tb := qaGROpen(t)
	rut := tb.mint("Rut", identity.RoleSpectator)
	tove := tb.mint("Tove", identity.RoleSpectator)
	vera := tb.mint("Vera", identity.RoleSpectator)
	tb.give("actor-c", tb.zora.id)
	tb.give("actor-d", rut.id)
	tb.give("actor-d", vera.id)
	tb.revoke(rut)
	tb.revoke(tove)
	rm := tb.dmc.send(&vttv1.ClientCommand{
		Command: &vttv1.ClientCommand_RemoveParticipant{
			RemoveParticipant: &vttv1.RemoveParticipant{ParticipantId: vera.id}}})
	if !rm.GetOk() {
		t.Fatalf("remove_participant of Vera: %q", rm.GetError())
	}
	delete(tb.live, vera.id)
	for _, gone := range []qaGRWho{rut, tove, vera} {
		for _, by := range tb.issuers() {
			for _, kind := range qaGRKinds() {
				for _, actor := range []string{"actor-b", "actor-c", "actor-d"} {
					got := tb.refused(by, by, qaGRGrant(actor, gone.id, kind))
					if got != qaGRNoSuch {
						t.Errorf("%s grants %s of %s to removed %s: got %q, want %q",
							by.label, kind, actor, gone.name, got, qaGRNoSuch)
					}
					if strings.Contains(got, gone.id) || strings.Contains(got, gone.name) {
						t.Errorf("refusal %q names removed %s", got, gone.name)
					}
				}
			}
		}
	}
}

// VTT-894 VTT-895
func TestQAGRAGrantNamingAnEmptyParticipantIdIsRefusedPlainly(t *testing.T) {
	tb := qaGROpen(t)
	for _, by := range tb.issuers() {
		for _, kind := range qaGRKinds() {
			got := tb.refused(by, by, qaGRGrant("actor-b", "", kind))
			if got != qaGRNoSuch {
				t.Errorf("%s grants %s of actor-b to \"\": got %q, want %q",
					by.label, kind, got, qaGRNoSuch)
			}
		}
	}
}

// VTT-894 VTT-895 VTT-913 VTT-915
func TestQAGRAGrantOfAnActorTheTableDoesNotHoldIsJudgedByItsGrantee(t *testing.T) {
	tb := qaGROpen(t)
	for _, by := range tb.issuers() {
		got := tb.refused(by, by, qaGRGrant("actor-x", "qa-gr-ghost", qaGRNonParty))
		if got != qaGRNoSuch {
			t.Errorf("%s, nobody, actor-x: got %q, want %q", by.label, got, qaGRNoSuch)
		}
		got = tb.refused(by, by, qaGRGrant("actor-x", tb.abel.id, qaGRNonParty))
		if want := "Abel" + qaGROnly; got != want {
			t.Errorf("%s, Abel, actor-x: got %q, want %q", by.label, got, want)
		}
		got = tb.refused(by, by, qaGRGrant("actor-x", tb.dm.id, qaGRNonParty))
		if got == qaGRNoSuch || strings.HasSuffix(got, qaGRTail) || got == "" {
			t.Errorf("%s, the DM, actor-x: got %q, want the fold's refusal",
				by.label, got)
		}
	}
}

// VTT-913 VTT-915
func TestQAGRANonPartyGrantToAPlayerOrASpectatorNamesThem(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-c", tb.zora.id)
	tb.give("actor-c", tb.sixt.id)
	grantees := []qaGRWho{tb.zora, tb.abel, tb.ingr, tb.sixt}
	for _, by := range tb.issuers() {
		for _, who := range grantees {
			for _, actor := range []string{"actor-a", "actor-b", "actor-c", "actor-d"} {
				got := tb.refused(by, by, qaGRGrant(actor, who.id, qaGRNonParty))
				if want := who.name + qaGROnly; got != want {
					t.Errorf("%s grants non-party %s to %s: got %q, want %q",
						by.label, actor, who.name, got, want)
				}
			}
		}
	}
	if k := tb.kind("actor-c"); k != qaGRParty {
		t.Errorf("actor-c's kind became %s", k)
	}
	if c := tb.controllers("actor-b"); len(c) != 0 {
		t.Errorf("actor-b's controllers became %v", c)
	}
}

// VTT-926 VTT-927
func TestQAGRANonPartyGrantToTheDMOrTheAgentOfAnActorOnePlayerControls(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-c", tb.zora.id)
	want := "Zora controls Amber: revoke it from them before you make it a non-party actor."
	for _, by := range tb.issuers() {
		for _, head := range tb.heads() {
			got := tb.refused(by, by, qaGRGrant("actor-c", head.id, qaGRNonParty))
			if got != want {
				t.Errorf("%s grants non-party actor-c to %s: got %q, want %q",
					by.label, head.name, got, want)
			}
		}
	}
	if k := tb.kind("actor-c"); k != qaGRParty {
		t.Errorf("actor-c's kind became %s", k)
	}
	if c := tb.controllers("actor-c"); len(c) != 1 || c[0] != tb.zora.id {
		t.Errorf("actor-c's controllers became %v", c)
	}
}

// VTT-926 VTT-927
func TestQAGRASpectatorControllerIsNamedLikeAPlayer(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-b", tb.sixt.id)
	want := "Sixten controls Moss" + qaGRTail
	for _, by := range tb.issuers() {
		for _, head := range tb.heads() {
			got := tb.refused(by, by, qaGRGrant("actor-b", head.id, qaGRNonParty))
			if got != want {
				t.Errorf("%s grants non-party actor-b to %s: got %q, want %q",
					by.label, head.name, got, want)
			}
		}
	}
}

// VTT-926 VTT-927
func TestQAGRTwoPlayersAreNamedInTheOrderTheActorListsThem(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-b", tb.abel.id)
	tb.give("actor-b", tb.zora.id)
	tb.give("actor-c", tb.zora.id)
	tb.give("actor-c", tb.abel.id)
	ob, oc := tb.controllers("actor-b"), tb.controllers("actor-c")
	if len(ob) != 2 || len(oc) != 2 {
		t.Fatalf("controllers: actor-b %v, actor-c %v", ob, oc)
	}
	if ob[0] == oc[0] {
		t.Logf("the fold lists both actors' controllers alike (%v); "+
			"the two grant orders could not be told apart", ob)
	}
	cases := []struct{ actor, label string }{
		{"actor-b", "Moss"}, {"actor-c", "Amber"},
	}
	for _, cs := range cases {
		order := tb.controllers(cs.actor)
		want := tb.live[order[0]] + " and " + tb.live[order[1]] +
			" control " + cs.label + qaGRTail
		if order[0] == tb.zora.id {
			if want != "Zora and Abel control "+cs.label+qaGRTail {
				t.Fatalf("expectation built wrong: %q", want)
			}
		}
		for _, by := range tb.issuers() {
			for _, head := range tb.heads() {
				got := tb.refused(by, by, qaGRGrant(cs.actor, head.id, qaGRNonParty))
				if got != want {
					t.Errorf("%s grants non-party %s to %s: got %q, want %q",
						by.label, cs.actor, head.name, got, want)
				}
			}
		}
	}
}

// VTT-926 VTT-927
func TestQAGRThreePlayersAreAllJoinedByAnd(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-c", tb.zora.id)
	tb.give("actor-c", tb.ingr.id)
	tb.give("actor-c", tb.abel.id)
	order := tb.controllers("actor-c")
	if len(order) != 3 {
		t.Fatalf("actor-c's controllers: %v", order)
	}
	want := tb.live[order[0]] + " and " + tb.live[order[1]] + " and " +
		tb.live[order[2]] + " control Amber" + qaGRTail
	for _, by := range tb.issuers() {
		got := tb.refused(by, by, qaGRGrant("actor-c", tb.dm.id, qaGRNonParty))
		if got != want {
			t.Errorf("%s: got %q, want %q", by.label, got, want)
		}
	}
}

// VTT-926 VTT-927
func TestQAGRTheDMOrTheAgentAsACoControllerIsNotNamed(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-b", tb.dm.id)
	tb.give("actor-b", tb.zora.id)
	tb.give("actor-c", tb.zora.id)
	tb.give("actor-c", tb.agent.id)
	tb.give("actor-d", tb.dm.id)
	tb.give("actor-d", tb.abel.id)
	tb.give("actor-d", tb.agent.id)
	tb.give("actor-d", tb.zora.id)
	cases := []struct{ actor, want string }{
		{"actor-b", "Zora controls Moss" + qaGRTail},
		{"actor-c", "Zora controls Amber" + qaGRTail},
		{"actor-d", tb.controlsText("actor-d", "actor-d")},
	}
	for _, cs := range cases {
		for _, by := range tb.issuers() {
			for _, head := range tb.heads() {
				got := tb.refused(by, by, qaGRGrant(cs.actor, head.id, qaGRNonParty))
				if got != cs.want {
					t.Errorf("%s grants non-party %s to %s: got %q, want %q",
						by.label, cs.actor, head.name, got, cs.want)
				}
			}
		}
	}
}

// VTT-926 VTT-927
func TestQAGRARemovedControllerIsNotNamedAndDoesNotCount(t *testing.T) {
	tb := qaGROpen(t)
	rut := tb.mint("Rut", identity.RoleSpectator)
	tb.give("actor-b", rut.id)
	tb.give("actor-b", tb.zora.id)
	tb.give("actor-c", tb.abel.id)
	tb.give("actor-c", rut.id)
	tb.give("actor-c", tb.zora.id)
	tb.revoke(rut)
	cases := []struct{ actor, want string }{
		{"actor-b", "Zora controls Moss" + qaGRTail},
		{"actor-c", tb.controlsText("actor-c", "Amber")},
	}
	if !strings.Contains(cases[1].want, " control Amber") {
		t.Fatalf("expectation built wrong: %q", cases[1].want)
	}
	for _, cs := range cases {
		for _, by := range tb.issuers() {
			for _, head := range tb.heads() {
				got := tb.refused(by, by, qaGRGrant(cs.actor, head.id, qaGRNonParty))
				if got != cs.want {
					t.Errorf("%s grants non-party %s to %s: got %q, want %q",
						by.label, cs.actor, head.name, got, cs.want)
				}
			}
		}
	}
}

// VTT-926 VTT-927
func TestQAGRAnActorOnlyARemovedParticipantAndTheDMControlIsMadeNonParty(t *testing.T) {
	tb := qaGROpen(t)
	rut := tb.mint("Rut", identity.RoleSpectator)
	tb.give("actor-c", tb.dm.id)
	tb.give("actor-c", rut.id)
	tb.give("actor-b", rut.id)
	tb.give("actor-b", tb.agent.id)
	tb.revoke(rut)
	tb.accepted(tb.dmc, qaGRGrant("actor-c", tb.dm.id, qaGRNonParty))
	tb.accepted(tb.agc, qaGRGrant("actor-b", tb.agent.id, qaGRNonParty))
	for _, a := range []string{"actor-b", "actor-c"} {
		if k := tb.kind(a); k != qaGRNonParty {
			t.Errorf("%s's kind is %s after an accepted non-party grant", a, k)
		}
	}
}

// VTT-926 VTT-927
func TestQAGRAnUnnamedActorIsNamedByItsId(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-d", tb.zora.id)
	want := "Zora controls actor-d" + qaGRTail
	for _, by := range tb.issuers() {
		for _, head := range tb.heads() {
			got := tb.refused(by, by, qaGRGrant("actor-d", head.id, qaGRNonParty))
			if got != want {
				t.Errorf("%s grants non-party actor-d to %s: got %q, want %q",
					by.label, head.name, got, want)
			}
		}
	}
}

// VTT-926 VTT-927
func TestQAGRANonPartyGrantOfAnActorNobodyControlsIsAppended(t *testing.T) {
	tb := qaGROpen(t)
	tb.accepted(tb.dmc, qaGRGrant("actor-b", tb.dm.id, qaGRNonParty))
	tb.accepted(tb.agc, qaGRGrant("actor-b", tb.agent.id, qaGRNonParty))
	tb.accepted(tb.agc, qaGRGrant("actor-d", tb.dm.id, qaGRNonParty))
	tb.accepted(tb.dmc, qaGRGrant("actor-c", tb.agent.id, qaGRNonParty))
	for _, a := range []string{"actor-b", "actor-c", "actor-d"} {
		if k := tb.kind(a); k != qaGRNonParty {
			t.Errorf("%s's kind is %s after an accepted non-party grant", a, k)
		}
	}
	got := tb.controllers("actor-b")
	if len(got) != 2 {
		t.Errorf("actor-b's controllers: %v, want the DM and the agent", got)
	}
}

// VTT-894 VTT-913 VTT-147
func TestQAGRAPartyMemberGrantToAnyoneTheTableHoldsIsAppended(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-c", tb.abel.id)
	plan := []struct {
		by    *qaGRConn
		actor string
		who   []qaGRWho
	}{
		{tb.dmc, "actor-b", []qaGRWho{tb.zora, tb.sixt, tb.dm, tb.agent}},
		{tb.agc, "actor-d", []qaGRWho{tb.abel, tb.sixt, tb.dm, tb.agent}},
		{tb.dmc, "actor-a", []qaGRWho{tb.dm, tb.zora}},
		{tb.agc, "actor-c", []qaGRWho{tb.agent, tb.sixt}},
	}
	for _, p := range plan {
		for _, w := range p.who {
			tb.accepted(p.by, qaGRGrant(p.actor, w.id, qaGRParty))
			found := false
			for _, c := range tb.controllers(p.actor) {
				found = found || c == w.id
			}
			if !found {
				t.Errorf("%s is not among %s's controllers after %s's grant",
					w.name, p.actor, p.by.label)
			}
		}
	}
}

// VTT-925
func TestQAGRAGrantStatingNoKindIsToldAboutItsKindWhoeverItNames(t *testing.T) {
	tb := qaGROpen(t)
	rut := tb.mint("Rut", identity.RoleSpectator)
	tb.give("actor-c", tb.abel.id)
	tb.revoke(rut)
	grantees := []struct{ label, id string }{
		{"nobody", "qa-gr-ghost"}, {"removed Rut", rut.id},
		{"Zora", tb.zora.id}, {"Sixten", tb.sixt.id},
		{"the DM", tb.dm.id}, {"the agent", tb.agent.id},
	}
	for _, by := range tb.issuers() {
		for _, g := range grantees {
			for _, actor := range []string{"actor-b", "actor-c"} {
				got := tb.refused(by, by, qaGRGrant(actor, g.id, qaGRNoKind))
				if !strings.Contains(strings.ToLower(got), "kind") {
					t.Errorf("%s's kindless grant of %s to %s: %q names no kind",
						by.label, actor, g.label, got)
				}
				if got == qaGRNoSuch || strings.HasSuffix(got, qaGROnly) ||
					strings.HasSuffix(got, qaGRTail) {
					t.Errorf("%s's kindless grant of %s to %s answered %q",
						by.label, actor, g.label, got)
				}
			}
		}
	}
}

// VTT-894
func TestQAGRAPlayersOrSpectatorsGrantIsRefusedByTheTableWhateverItNames(t *testing.T) {
	tb := qaGROpen(t)
	rut := tb.mint("Rut", identity.RoleSpectator)
	tb.give("actor-c", tb.zora.id)
	tb.revoke(rut)
	pc := tb.dial(tb.ingr, "player")
	sc := tb.dial(tb.sixt, "spectator")
	targets := []string{"qa-gr-ghost", rut.id, tb.zora.id, tb.ingr.id,
		tb.sixt.id, tb.dm.id, tb.agent.id}
	issuers := []struct {
		by, marker *qaGRConn
		role       string
	}{{pc, pc, "player"}, {sc, tb.dmc, "spectator"}}
	for _, is := range issuers {
		first := ""
		for _, pid := range targets {
			for _, kind := range qaGRKinds() {
				for _, actor := range []string{"actor-a", "actor-c"} {
					got := tb.refused(is.by, is.marker, qaGRGrant(actor, pid, kind))
					if first == "" {
						first = got
					}
					if got != first {
						t.Errorf("%s's grant naming %q: %q differs from %q",
							is.role, pid, got, first)
					}
					if got == qaGRNoSuch || got == "" {
						t.Errorf("%s's grant naming %q answered %q",
							is.role, pid, got)
					}
				}
			}
		}
		if !strings.Contains(first, "grant_actor_control") ||
			!strings.Contains(first, is.role) {
			t.Errorf("%s's refusal %q does not name both the command and the role",
				is.role, first)
		}
	}
}

// VTT-147 VTT-927
func TestQAGRTheDMRevokesAPlayersControlAndThenMakesTheActorNonParty(t *testing.T) {
	tb := qaGROpen(t)
	tb.give("actor-c", tb.zora.id)
	got := tb.refused(tb.dmc, tb.dmc, qaGRGrant("actor-c", tb.dm.id, qaGRNonParty))
	if want := "Zora controls Amber" + qaGRTail; got != want {
		t.Errorf("before the revoke: got %q, want %q", got, want)
	}
	tb.accepted(tb.dmc, qaGRRevoke("actor-c", tb.zora.id))
	tb.accepted(tb.dmc, qaGRGrant("actor-c", tb.dm.id, qaGRNonParty))
	if k := tb.kind("actor-c"); k != qaGRNonParty {
		t.Errorf("actor-c's kind is %s", k)
	}
}
