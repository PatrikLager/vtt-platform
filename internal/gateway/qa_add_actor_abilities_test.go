package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaAbTable struct {
	c      *campaign.Campaign
	wsBase string
	dm     string
	agent  string
}

type qaAbConn struct {
	t      *testing.T
	frames chan *vttv1.ServerFrame
	events map[int64]*vttv1.Envelope
	conn   *websocket.Conn
}

var qaAbRequests atomic.Int64

func qaAbTavern(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load("../../rulesets/tavern-brawl")
	if err != nil {
		t.Fatalf("rules.Load(tavern-brawl): %v", err)
	}
	return rs
}

func qaAbNewTable(t *testing.T, rs *rules.Ruleset) *qaAbTable {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	dmTok, _, err := ids.CreateInvite("Qa Dm", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite dm: %v", err)
	}
	agentTok, _, err := ids.CreateInvite("Qa Agent", identity.RoleAgent)
	if err != nil {
		t.Fatalf("CreateInvite agent: %v", err)
	}
	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return &qaAbTable{c: c, wsBase: "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws", dm: dmTok, agent: agentTok}
}

func (tb *qaAbTable) dial(t *testing.T, token string) *qaAbConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, tb.wsBase+"?token="+url.QueryEscape(token)+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(1 << 22)
	qc := &qaAbConn{t: t, frames: make(chan *vttv1.ServerFrame, 256), events: map[int64]*vttv1.Envelope{}, conn: conn}
	go func() {
		defer close(qc.frames)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, f); err != nil {
				return
			}
			qc.frames <- f
		}
	}()
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })
	return qc
}

func (qc *qaAbConn) next() *vttv1.ServerFrame {
	qc.t.Helper()
	select {
	case f, ok := <-qc.frames:
		if !ok {
			qc.t.Fatal("connection closed while waiting for a frame")
		}
		if ev := f.GetEvent(); ev != nil {
			qc.events[ev.GetSequence()] = ev
		}
		return f
	case <-time.After(10 * time.Second):
		qc.t.Fatal("timed out waiting for a frame")
	}
	return nil
}

func (qc *qaAbConn) addActor(actor *vttv1.Actor) *vttv1.CommandResult {
	qc.t.Helper()
	rid := fmt.Sprintf("qa-ab-%d", qaAbRequests.Add(1))
	cmd := &vttv1.ClientCommand{RequestId: rid, Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: actor}}}
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		qc.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := qc.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		qc.t.Fatalf("write: %v", err)
	}
	for {
		if r := qc.next().GetResult(); r != nil && r.GetRequestId() == rid {
			return r
		}
	}
}

func (qc *qaAbConn) event(seq int64) *vttv1.Envelope {
	qc.t.Helper()
	for {
		if ev, ok := qc.events[seq]; ok {
			return ev
		}
		qc.next()
	}
}

func qaAbPC(id string, abilities ...string) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       "Qa " + id,
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		Attributes: map[string]int32{"brawn": 2, "grit": 1},
		Resources:  map[string]*vttv1.Resource{"drink": {Current: 0, Max: 4}},
		AbilityIds: abilities,
	}
}

func qaAbUnheld(actor, id string) string {
	return `gateway: add_actor: actor "` + actor + `": ability_ids: names ability "` + id + `", which the ruleset does not hold`
}

func qaAbMustAccept(t *testing.T, qc *qaAbConn, actor *vttv1.Actor) int64 {
	t.Helper()
	r := qc.addActor(actor)
	if !r.GetOk() {
		t.Fatalf("add_actor %q refused: %s", actor.GetActorId(), r.GetError())
	}
	return r.GetSequence()
}

func qaAbWantAppended(t *testing.T, tb *qaAbTable, qc *qaAbConn, seq int64, id string, want []string) {
	t.Helper()
	ev := qc.event(seq)
	added := ev.GetActorAdded()
	if added == nil || added.GetActor().GetActorId() != id {
		t.Fatalf("event %d = %v, want ActorAdded for %q", seq, ev, id)
	}
	if got := added.GetActor().GetAbilityIds(); !slices.Equal(got, want) {
		t.Fatalf("appended ActorAdded ability_ids = %q, want %q", got, want)
	}
	folded := tb.c.State().Actors[id]
	if folded == nil {
		t.Fatalf("fold holds no actor %q", id)
	}
	if got := folded.GetAbilityIds(); !slices.Equal(got, want) {
		t.Fatalf("folded actor ability_ids = %q, want %q", got, want)
	}
}

func qaAbWantNothingAppended(t *testing.T, tb *qaAbTable, qc *qaAbConn, before int64, refusedID, probeID string) {
	t.Helper()
	if refusedID != "" {
		if _, ok := tb.c.State().Actors[refusedID]; ok {
			t.Fatalf("fold holds the refused actor %q", refusedID)
		}
	}
	after := qaAbMustAccept(t, qc, qaAbPC(probeID))
	if after != before+1 {
		t.Fatalf("probe after the refusal got sequence %d, want %d: the refusal appended", after, before+1)
	}
}

// VTT-683
func TestQAAbAnAcceptedAddActorAppendsItsAbilityIdsInTheirOrder(t *testing.T) {
	tb := qaAbNewTable(t, qaAbTavern(t))
	for _, who := range []struct{ name, token string }{{"dm", tb.dm}, {"agent", tb.agent}} {
		t.Run(who.name, func(t *testing.T) {
			qc := tb.dial(t, who.token)
			want := []string{"sober-up", "fists", "splash-of-water", "chair-swing"}
			id := "qa-ordered-" + who.name
			seq := qaAbMustAccept(t, qc, qaAbPC(id, want...))
			qaAbWantAppended(t, tb, qc, seq, id, want)
		})
	}
}

// VTT-683
func TestQAAbARepeatedHeldIdIsAcceptedAndAppendedAsSent(t *testing.T) {
	tb := qaAbNewTable(t, qaAbTavern(t))
	qc := tb.dial(t, tb.dm)
	want := []string{"fists", "chair-swing", "fists"}
	seq := qaAbMustAccept(t, qc, qaAbPC("qa-repeat", want...))
	qaAbWantAppended(t, tb, qc, seq, "qa-repeat", want)
}

// VTT-685
func TestQAAbAnUnheldIdIsRefusedNamingTheActorAndFirstUnheldIdInByteOrder(t *testing.T) {
	tb := qaAbNewTable(t, qaAbTavern(t))
	for _, who := range []struct{ name, token string }{{"dm", tb.dm}, {"agent", tb.agent}} {
		t.Run(who.name, func(t *testing.T) {
			qc := tb.dial(t, who.token)
			before := qaAbMustAccept(t, qc, qaAbPC("qa-before-"+who.name))
			id := "qa-unheld-" + who.name
			r := qc.addActor(qaAbPC(id, "fists", "zz-unheld", "beta-unheld", "Zed-unheld", "chair-swing"))
			if r.GetOk() {
				t.Fatal("add_actor naming unheld ids was accepted")
			}
			if want := qaAbUnheld(id, "Zed-unheld"); r.GetError() != want {
				t.Fatalf("refusal = %q, want %q", r.GetError(), want)
			}
			if r.GetSequence() != 0 {
				t.Fatalf("refusal carries sequence %d, want none", r.GetSequence())
			}
			qaAbWantNothingAppended(t, tb, qc, before, id, "qa-probe-"+who.name)
		})
	}
}

// VTT-685
func TestQAAbAHeldIdFirstInByteOrderDoesNotHideAnUnheldOne(t *testing.T) {
	tb := qaAbNewTable(t, qaAbTavern(t))
	qc := tb.dial(t, tb.dm)
	r := qc.addActor(qaAbPC("qa-late", "splash-of-water", "zz-unheld", "chair-swing"))
	if want := qaAbUnheld("qa-late", "zz-unheld"); r.GetOk() || r.GetError() != want {
		t.Fatalf("result = ok %v %q, want refusal %q", r.GetOk(), r.GetError(), want)
	}
	r = qc.addActor(qaAbPC("qa-case", "fists", "Fists"))
	if want := qaAbUnheld("qa-case", "Fists"); r.GetOk() || r.GetError() != want {
		t.Fatalf("result = ok %v %q, want refusal %q", r.GetOk(), r.GetError(), want)
	}
}

// VTT-685
func TestQAAbTheControllerKindAndKeyRefusalsComeFirst(t *testing.T) {
	cases := []struct {
		name string
		edit func(a *vttv1.Actor)
		want string
	}{
		{"a controller id", func(a *vttv1.Actor) { a.ControllerId = "someone" }, ""},
		{"an empty controller in the set", func(a *vttv1.Actor) { a.ControllerIds = []string{""} }, ""},
		{"no kind", func(a *vttv1.Actor) { a.Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED }, ""},
		{"an undeclared key", func(a *vttv1.Actor) { a.Attributes["qa-undeclared"] = 1 },
			`gateway: add_actor: actor "qa-order": attributes: references undeclared property "qa-undeclared"`},
		{"a number under the resources", func(a *vttv1.Actor) { a.Resources["brawn"] = &vttv1.Resource{Current: 1} },
			`gateway: add_actor: actor "qa-order": resources: gives number property "brawn" a resource's value`},
	}
	tb := qaAbNewTable(t, qaAbTavern(t))
	qc := tb.dial(t, tb.dm)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held := qaAbPC("qa-order", "fists")
			tc.edit(held)
			heldResult := qc.addActor(held)
			if heldResult.GetOk() {
				t.Fatalf("control: add_actor with %s and held ids was accepted", tc.name)
			}
			before := qaAbMustAccept(t, qc, qaAbPC(fmt.Sprintf("qa-order-before-%d", i)))
			unheld := qaAbPC("qa-order", "fists", "zz-unheld")
			tc.edit(unheld)
			r := qc.addActor(unheld)
			if r.GetOk() {
				t.Fatalf("add_actor with %s and an unheld id was accepted", tc.name)
			}
			t.Logf("refusal: %s", r.GetError())
			if strings.Contains(r.GetError(), "ability_ids") {
				t.Fatalf("refusal = %q, want the earlier refusal", r.GetError())
			}
			if r.GetError() != heldResult.GetError() {
				t.Fatalf("refusal with an unheld id = %q, want the one with held ids, %q", r.GetError(), heldResult.GetError())
			}
			if tc.want != "" && r.GetError() != tc.want {
				t.Fatalf("refusal = %q, want %q", r.GetError(), tc.want)
			}
			qaAbWantNothingAppended(t, tb, qc, before, "qa-order", fmt.Sprintf("qa-order-probe-%d", i))
		})
	}
}

// VTT-154
func TestQAAbAnActorWithNoIdIsNotReadForItsAbilities(t *testing.T) {
	tb := qaAbNewTable(t, qaAbTavern(t))
	qc := tb.dial(t, tb.dm)
	before := qaAbMustAccept(t, qc, qaAbPC("qa-noid-before"))
	r := qc.addActor(qaAbPC("", "zz-unheld"))
	if r.GetOk() {
		t.Fatal("add_actor with no id was accepted")
	}
	t.Logf("refusal: %s", r.GetError())
	if strings.Contains(r.GetError(), "ability_ids") || strings.Contains(r.GetError(), "does not hold") {
		t.Fatalf("refusal of an actor with no id = %q, want it not read for its abilities", r.GetError())
	}
	qaAbWantNothingAppended(t, tb, qc, before, "", "qa-noid-probe")
}

// VTT-686
func TestQAAbWithNoRulesetServedNoAbilityIdIsRefused(t *testing.T) {
	tb := qaAbNewTable(t, nil)
	for _, who := range []struct{ name, token string }{{"dm", tb.dm}, {"agent", tb.agent}} {
		t.Run(who.name, func(t *testing.T) {
			qc := tb.dial(t, who.token)
			want := []string{"zz-unheld", "fists", "Zed-unheld", "", "zz-unheld"}
			id := "qa-free-" + who.name
			a := qaAbPC(id, want...)
			a.Attributes["qa-undeclared"] = 7
			seq := qaAbMustAccept(t, qc, a)
			qaAbWantAppended(t, tb, qc, seq, id, want)
		})
	}
}
