package gateway_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	spkPartyKind = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	spkNonParty  = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func spkActor(id, name string, k vttv1.ActorKind, ctl ...string) *vttv1.Actor {
	a := &vttv1.Actor{ActorId: id, Name: name, Kind: k, ControllerIds: ctl}
	if len(ctl) > 0 {
		a.ControllerId = ctl[0]
	}
	return a
}

func spkState() *engine.State {
	st := engine.NewState()
	for _, a := range []*vttv1.Actor{
		spkActor("aria", "Aria", spkPartyKind, "ada"),
		spkActor("bram", "Bram", spkPartyKind, "bo"),
		spkActor("gob", "Gob", spkNonParty, "ada"),
		spkActor("blank", "", spkPartyKind, "ada"),
		spkActor("orphan", "Orphan", spkPartyKind),
	} {
		st.Actors[a.ActorId] = a
	}
	return st
}

func spkNarr(as, actor string, ooc bool) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "r",
		Command: &vttv1.ClientCommand_AddNarration{
			AddNarration: &vttv1.AddNarration{
				Text: "hello", As: as, ActorId: actor,
				OutOfCharacter: ooc}}}
}

func spkP(id string, role identity.Role) *identity.Participant {
	return &identity.Participant{ID: id, Name: "N-" + id, Role: role}
}

// Player half: VTT-495.
func TestQASpkPlayerRefusalsAndAcceptances(t *testing.T) {
	st := spkState()
	ada := spkP("ada", identity.RolePlayer)
	cases := []struct {
		name string
		cmd  *vttv1.ClientCommand
		ok   bool
	}{
		// VTT-495: out of character and names no actor: accepted
		{"ooc no actor", spkNarr("", "", true), true},
		// VTT-495: `as` does not decide a line out of character
		{"ooc as DM", spkNarr("DM", "", true), true},
		// VTT-495
		{"ooc naming own", spkNarr("", "aria", true), false},
		// VTT-495: in character naming a controlled party member
		{"own party member", spkNarr("", "aria", false), true},
		{"own party member as lies", spkNarr("DM", "aria", false), true},
		// VTT-495: a creature that is not a party member
		{"own creature", spkNarr("", "gob", false), false},
		// VTT-495: a party member the player does not control
		{"other's party", spkNarr("", "bram", false), false},
		{"uncontrolled party", spkNarr("", "orphan", false), false},
		// VTT-495: absent actor
		{"absent actor", spkNarr("", "nobody", false), false},
		// VTT-495: as DM or empty, no actor, in character
		{"as DM no actor", spkNarr("DM", "", false), false},
		{"empty no actor", spkNarr("", "", false), false},
		{"as own name no actor", spkNarr("Aria", "", false), false},
		// VTT-495: whose name is not empty
		{"nameless own party", spkNarr("", "blank", false), false},
	}
	for _, c := range cases {
		err := gateway.Authorize(ada, c.cmd, st)
		if c.ok && err != nil {
			t.Errorf("%s: want accepted, got %v", c.name, err)
		}
		if !c.ok {
			if err == nil {
				t.Errorf("%s: want refused, got nil", c.name)
			} else if !errors.Is(err, gateway.ErrUnauthorized) {
				t.Errorf("%s: refusal does not wrap: %v", c.name, err)
			}
		}
	}
}

// VTT-506
func TestQASpkNamedRefusalsShareOneText(t *testing.T) {
	st := spkState()
	ada := spkP("ada", identity.RolePlayer)
	var texts []string
	for _, id := range []string{"nobody", "bram", "gob", "orphan"} {
		err := gateway.Authorize(ada, spkNarr("", id, false), st)
		if err == nil {
			t.Fatalf("%s accepted", id)
		}
		texts = append(texts, err.Error())
	}
	for _, s := range texts[1:] {
		if s != texts[0] {
			t.Errorf("texts differ: %q vs %q", s, texts[0])
		}
	}
	err := gateway.Authorize(ada, spkNarr("", "blank", false), st)
	if err == nil || !strings.Contains(err.Error(), "DM") ||
		!strings.Contains(err.Error(), "out of character") {
		t.Errorf("nameless text names neither the DM nor out of character: %v", err)
	}
}

// VTT-497, VTT-498, VTT-499
func TestQASpkDMAndAgentRules(t *testing.T) {
	st := spkState()
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := spkP("x", role)
		ok := map[string]*vttv1.ClientCommand{
			// VTT-498: as given, empty included
			"as given": spkNarr("Goblin chief", "", false),
			"empty as": spkNarr("", "", false),
			// VTT-497
			"ooc":         spkNarr("", "", true),
			"ooc with as": spkNarr("Zed", "", true),
		}
		for n, c := range ok {
			if err := gateway.Authorize(p, c, st); err != nil {
				t.Errorf("%s %s: %v", role, n, err)
			}
		}
		bad := map[string]*vttv1.ClientCommand{
			// VTT-499: naming an actor is refused, any actor
			"party":    spkNarr("", "aria", false),
			"creature": spkNarr("", "gob", false),
			"absent":   spkNarr("", "nobody", false),
			"orphan":   spkNarr("x", "orphan", false),
			// VTT-499
			"ooc+actor": spkNarr("", "aria", true),
		}
		for n, c := range bad {
			err := gateway.Authorize(p, c, st)
			if err == nil {
				t.Errorf("%s %s: accepted", role, n)
			} else if !errors.Is(err, gateway.ErrUnauthorized) {
				t.Errorf("%s %s: no wrap: %v", role, n, err)
			}
		}
	}
	// SPEC-013 table: spectator holds no add_narration cell
	sp := spkP("s", identity.RoleSpectator)
	if gateway.Authorize(sp, spkNarr("", "", true), st) == nil {
		t.Error("spectator accepted")
	}
}

type spkWorld struct {
	url                string
	c                  *campaign.Campaign
	dm, agent, ada, bo string
	adaID, boID        string
	dmName, adaName    string
	seq                int64
}

func spkOpen(t *testing.T) *spkWorld {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	w := &spkWorld{c: c, dmName: "Dana", adaName: "Ada"}
	mk := func(n string, r identity.Role) (string, string) {
		tok, id, err := ids.CreateInvite(n, r)
		if err != nil {
			t.Fatal(err)
		}
		return tok, id
	}
	w.dm, _ = mk("Dana", identity.RoleDM)
	w.agent, _ = mk("Agnes", identity.RoleAgent)
	w.ada, w.adaID = mk("Ada", identity.RolePlayer)
	w.bo, w.boID = mk("Bo", identity.RolePlayer)
	n := 0
	add := func(p *vttv1.Envelope) {
		n++
		p.EventId = "seed-" + strconv.Itoa(n)
		if _, err := c.Append(p); err != nil {
			t.Fatal(err)
		}
	}
	actor := func(id, name string, k vttv1.ActorKind, ctl string) {
		add(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
			ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId: id, Name: name, Kind: k}}}})
		add(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{
				ActorId: id, ParticipantId: ctl, Kind: k}}})
	}
	actor("aria", "Aria", spkPartyKind, w.adaID)
	actor("bram", "Bram", spkPartyKind, w.boID)
	actor("gob", "Gob", spkNonParty, w.adaID)
	w.seq = int64(n)
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	w.url = "ws" + strings.TrimPrefix(srv.URL, "http")
	return w
}

type spkConn struct {
	t    *testing.T
	conn *websocket.Conn
	ctx  context.Context
}

func (w *spkWorld) dial(t *testing.T, tok string, after int64) *spkConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	u := w.url + "/ws?token=" + tok + "&after=" + strconv.FormatInt(after, 10)
	cn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cn.CloseNow() })
	return &spkConn{t, cn, ctx}
}

func (s *spkConn) next() *vttv1.ServerFrame {
	_, b, err := s.conn.Read(s.ctx)
	if err != nil {
		s.t.Fatalf("read: %v", err)
	}
	f := &vttv1.ServerFrame{}
	if err := protojson.Unmarshal(b, f); err != nil {
		s.t.Fatal(err)
	}
	return f
}

func (s *spkConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	s.t.Helper()
	b, _ := protojson.Marshal(cmd)
	if err := s.conn.Write(s.ctx, websocket.MessageText, b); err != nil {
		s.t.Fatal(err)
	}
	for {
		if r := s.next().GetResult(); r != nil {
			return r
		}
	}
}

func (s *spkConn) narration(text string) *vttv1.Envelope {
	s.t.Helper()
	for {
		e := s.next().GetEvent()
		if e.GetNarrationAdded().GetText() == text {
			return e
		}
	}
}

func (w *spkWorld) say(t *testing.T, tok, text string, a *vttv1.AddNarration) (
	*vttv1.CommandResult, *vttv1.Envelope) {
	t.Helper()
	obs := w.dial(t, w.dm, w.seq)
	obs.next() // catch-up head
	me := w.dial(t, tok, w.seq)
	a.Text = text
	res := me.send(&vttv1.ClientCommand{RequestId: "q",
		Command: &vttv1.ClientCommand_AddNarration{AddNarration: a}})
	if !res.GetOk() {
		return res, nil
	}
	return res, obs.narration(text)
}

// VTT-496, VTT-503.
func TestQASpkInCharacterLineIsAppendedUnderTheActorsName(t *testing.T) {
	w := spkOpen(t)
	for _, lie := range []string{"", "DM", "Bram", "Whoever"} {
		txt := "draw " + lie
		res, ev := w.say(t, w.ada, txt, &vttv1.AddNarration{
			As: lie, ActorId: "aria", AnchorFromSeq: 1, AnchorToSeq: 2})
		if !res.GetOk() {
			t.Fatalf("lie %q refused: %s", lie, res.GetError())
		}
		n := ev.GetNarrationAdded()
		// VTT-496: as is the actor's name whatever the command carried
		if n.GetAs() != "Aria" {
			t.Errorf("lie %q: as = %q", lie, n.GetAs())
		}
		// VTT-496: not out of character
		if n.GetOutOfCharacter() {
			t.Error("marked out of character")
		}
		// VTT-503: other fields the event has are carried
		if n.GetAnchorFromSeq() != 1 || n.GetAnchorToSeq() != 2 {
			t.Errorf("anchors lost: %v", n)
		}
		if ev.GetParticipantId() != w.adaID || ev.GetActorRole() != "player" {
			t.Errorf("stamp: %v", ev)
		}
	}
}

// VTT-497.
func TestQASpkOutOfCharacterLineNamesItsSender(t *testing.T) {
	w := spkOpen(t)
	for _, lie := range []string{"", "DM", "Aria"} {
		txt := "brb " + lie
		res, ev := w.say(t, w.ada, txt, &vttv1.AddNarration{
			As: lie, OutOfCharacter: true,
			AnchorFromSeq: 1, AnchorToSeq: 2})
		if !res.GetOk() {
			t.Fatalf("refused: %s", res.GetError())
		}
		n := ev.GetNarrationAdded()
		// VTT-497: marked, and as = the sender's display name
		if !n.GetOutOfCharacter() || n.GetAs() != "Ada" {
			t.Errorf("lie %q: %v", lie, n)
		}
		if n.GetAnchorToSeq() != 2 { // VTT-503
			t.Errorf("anchor lost: %v", n)
		}
	}
	// VTT-497 applies to "a narration": DM's and agent's too
	for tok, want := range map[string]string{w.dm: "Dana", w.agent: "Agnes"} {
		res, ev := w.say(t, tok, "ooc "+want, &vttv1.AddNarration{
			As: "Goblin", OutOfCharacter: true})
		if !res.GetOk() {
			t.Fatalf("%s refused: %s", want, res.GetError())
		}
		n := ev.GetNarrationAdded()
		if !n.GetOutOfCharacter() || n.GetAs() != want {
			t.Errorf("%s: %v", want, n)
		}
	}
}

// VTT-498.
func TestQASpkDMAndAgentLinesKeepTheirAs(t *testing.T) {
	w := spkOpen(t)
	for _, tok := range []string{w.dm, w.agent} {
		for _, as := range []string{"Goblin chief", ""} {
			txt := "said " + as + tok[:3]
			res, ev := w.say(t, tok, txt, &vttv1.AddNarration{As: as})
			if !res.GetOk() {
				t.Fatalf("refused: %s", res.GetError())
			}
			n := ev.GetNarrationAdded()
			if n.GetAs() != as || n.GetOutOfCharacter() { // VTT-498
				t.Errorf("as %q: %v", as, n)
			}
		}
	}
}

// VTT-499, VTT-495: refusals are ok=false and append nothing.
func TestQASpkRefusedLinesAppendNothing(t *testing.T) {
	w := spkOpen(t)
	cases := []struct {
		tok string
		a   *vttv1.AddNarration
	}{
		{w.dm, &vttv1.AddNarration{ActorId: "aria"}},   // VTT-499
		{w.agent, &vttv1.AddNarration{ActorId: "gob"}}, // VTT-499
		{w.ada, &vttv1.AddNarration{ActorId: "bram"}},  // VTT-495
		{w.ada, &vttv1.AddNarration{ActorId: "gob"}},   // VTT-495
		{w.ada, &vttv1.AddNarration{As: "DM"}},         // VTT-495
		{w.ada, &vttv1.AddNarration{}},                 // VTT-495
		{w.ada, &vttv1.AddNarration{ActorId: "aria",
			OutOfCharacter: true}}, // VTT-495
	}
	for i, c := range cases {
		res, ev := w.say(t, c.tok, "x"+strconv.Itoa(i), c.a)
		if res.GetOk() || ev != nil {
			t.Errorf("case %d accepted", i)
		}
	}
	chk := w.dial(t, w.dm, 0)
	h := chk.next().GetCatchUpHead().GetHeadSequence()
	if h != w.seq { // nothing was appended beyond the seed
		t.Errorf("head %d, want %d", h, w.seq)
	}
}

// VTT-497 and VTT-496: the mark reaches a player's connection.
func TestQASpkAPlayersConnectionIsSentTheMarkAndName(t *testing.T) {
	w := spkOpen(t)
	bo := w.dial(t, w.bo, 0)
	ada := w.dial(t, w.ada, 0)
	res := ada.send(&vttv1.ClientCommand{RequestId: "m",
		Command: &vttv1.ClientCommand_AddNarration{
			AddNarration: &vttv1.AddNarration{Text: "pizza?", As: "lie",
				OutOfCharacter: true}}})
	if !res.GetOk() {
		t.Fatal(res.GetError())
	}
	n := bo.narration("pizza?").GetNarrationAdded()
	if !n.GetOutOfCharacter() || n.GetAs() != "Ada" {
		t.Errorf("another seat: %v", n)
	}
	res = ada.send(&vttv1.ClientCommand{RequestId: "n",
		Command: &vttv1.ClientCommand_AddNarration{
			AddNarration: &vttv1.AddNarration{Text: "sword", As: "lie",
				ActorId: "aria"}}})
	if !res.GetOk() {
		t.Fatal(res.GetError())
	}
	n = bo.narration("sword").GetNarrationAdded()
	if n.GetOutOfCharacter() || n.GetAs() != "Aria" {
		t.Errorf("another seat: %v", n)
	}
}
