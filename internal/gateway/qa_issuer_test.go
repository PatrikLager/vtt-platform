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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaIssP1      = "qa-iss-player-one"
	qaIssP2      = "qa-iss-player-two"
	qaIssDM      = "qa-iss-dm"
	qaIssAgent   = "qa-iss-agent"
	qaIssSpec    = "qa-iss-spectator"
	qaIssSession = "qa-iss-session"
	qaIssDazed   = "dazed-by-ale"
)

func qaIssActor(id string, kind vttv1.ActorKind) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       id,
		Kind:       kind,
		Attributes: map[string]int32{"brawn": 3, "grit": 1, "footing": 0},
		Resources: map[string]*vttv1.Resource{
			"drink":   {Current: 0, Max: 5},
			"stamina": {Current: 2, Max: 2},
		},
	}
}

func qaIssAt(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }

func qaIssKind(env *vttv1.Envelope) string {
	m := env.ProtoReflect()
	fd := m.WhichOneof(m.Descriptor().Oneofs().ByName("payload"))
	if fd == nil {
		return ""
	}
	return string(fd.Name())
}

func qaIssCause(env *vttv1.Envelope) string {
	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_TokenMoved:
		return p.TokenMoved.GetReason()
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetReason()
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetSource()
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetReason()
	}
	return ""
}

func qaIssLessIssuerAndCause(env *vttv1.Envelope) *vttv1.Envelope {
	want := proto.Clone(env).(*vttv1.Envelope)
	want.ParticipantId = ""
	want.ActorRole = ""
	switch p := want.GetPayload().(type) {
	case *vttv1.Envelope_TokenMoved:
		p.TokenMoved.Reason = ""
	case *vttv1.Envelope_ResourceChanged:
		p.ResourceChanged.Reason = ""
	case *vttv1.Envelope_ConditionApplied:
		p.ConditionApplied.Source = ""
	case *vttv1.Envelope_ConditionRemoved:
		p.ConditionRemoved.Reason = ""
	}
	return want
}

type qaIssConn struct {
	t      *testing.T
	ws     *websocket.Conn
	in     chan *vttv1.ServerFrame
	got    []*vttv1.ServerFrame
	reqSeq int
}

func qaIssDial(t *testing.T, base, token string) *qaIssConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(base, "http") + "/ws?token=" + url.QueryEscape(token) + "&after=0"
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ws.SetReadLimit(1 << 22)
	c := &qaIssConn{t: t, ws: ws, in: make(chan *vttv1.ServerFrame, 8192)}
	go func() {
		defer close(c.in)
		for {
			_, data, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if protojson.Unmarshal(data, f) != nil {
				f = &vttv1.ServerFrame{}
			}
			c.in <- f
		}
	}()
	t.Cleanup(func() { _ = ws.CloseNow() })
	return c
}

func (c *qaIssConn) until(desc string, pred func(*vttv1.ServerFrame) bool) {
	c.t.Helper()
	for _, f := range c.got {
		if pred(f) {
			return
		}
	}
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				c.t.Fatalf("connection closed while waiting for %s", desc)
			}
			if f.GetFrame() == nil {
				c.t.Fatalf("undecodable frame while waiting for %s", desc)
			}
			c.got = append(c.got, f)
			if pred(f) {
				return
			}
		case <-time.After(10 * time.Second):
			c.t.Fatalf("no %s within 10s", desc)
		}
	}
}

func (c *qaIssConn) do(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	c.reqSeq++
	cmd.RequestId = fmt.Sprintf("qa-iss-req-%d", c.reqSeq)
	b, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, b); err != nil {
		c.t.Fatalf("write: %v", err)
	}
	var res *vttv1.CommandResult
	c.until("result "+cmd.RequestId, func(f *vttv1.ServerFrame) bool {
		if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
			res = r
			return true
		}
		return false
	})
	return res
}

func (c *qaIssConn) must(cmd *vttv1.ClientCommand) int64 {
	c.t.Helper()
	r := c.do(cmd)
	if !r.GetOk() {
		c.t.Fatalf("command %v refused: %s", cmd, r.GetError())
	}
	return r.GetSequence()
}

func (c *qaIssConn) untilNarration(text string) {
	c.t.Helper()
	c.until("narration "+text, func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetNarrationAdded().GetText() == text
	})
}

func (c *qaIssConn) events() []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range c.got {
		if e := f.GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

type qaIssTable struct {
	t      *testing.T
	c      *campaign.Campaign
	url    string
	tokens map[identity.Role]string
	ids    map[identity.Role]string
}

func qaIssOpenTable(t *testing.T) *qaIssTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	rs, err := rules.Load("../../rulesets/tavern-brawl")
	if err != nil {
		t.Fatalf("ruleset: %v", err)
	}
	hs := httptest.NewServer(gateway.New(c, ids).WithRuleset(rs).Handler())
	t.Cleanup(hs.Close)
	tb := &qaIssTable{t: t, c: c, url: hs.URL, tokens: map[identity.Role]string{}, ids: map[identity.Role]string{}}
	for _, r := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator} {
		tok, id, err := ids.CreateInvite("qa-iss-"+string(r), r)
		if err != nil {
			t.Fatalf("invite: %v", err)
		}
		tb.tokens[r], tb.ids[r] = tok, id
	}
	return tb
}

func (tb *qaIssTable) dial(r identity.Role) *qaIssConn {
	tb.t.Helper()
	return qaIssDial(tb.t, tb.url, tb.tokens[r])
}

func (tb *qaIssTable) appendAsDM(id string, env *vttv1.Envelope) {
	tb.t.Helper()
	env.EventId = id
	env.OccurredAt = timestamppb.Now()
	env.ParticipantId = tb.ids[identity.RoleDM]
	env.ActorRole = string(identity.RoleDM)
	if _, err := tb.c.Append(env); err != nil {
		tb.t.Fatalf("append %s: %v", id, err)
	}
}

func (tb *qaIssTable) readLog() []*vttv1.Envelope {
	tb.t.Helper()
	ch, unsub, head, err := tb.c.Subscribe(0, 8192)
	if err != nil {
		tb.t.Fatalf("subscribe: %v", err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	for head > 0 {
		select {
		case e := <-ch:
			out = append(out, proto.Clone(e).(*vttv1.Envelope))
			if e.GetSequence() >= head {
				return out
			}
		case <-time.After(10 * time.Second):
			tb.t.Fatalf("log catch-up stalled at %d of %d", len(out), head)
		}
	}
	return out
}

func qaIssCmdAbility(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: actor, AbilityId: ability, TargetIds: []string{"patron"},
	}}}
}

func qaIssCmdNarrate(text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: text}}}
}

func qaIssCmdPlace(tok, scene, actor string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: tok, SceneId: scene, ActorId: actor, Position: qaIssAt(x, y),
	}}}
}

func qaIssCmdPerch(actor string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{ActorId: actor}}}
}

const qaIssMarker = "qa-iss-marker"

type qaIssPlay struct {
	tb     *qaIssTable
	dm, ag *qaIssConn
	p1, sp *qaIssConn
	actsAt int64
}

func qaIssPlayTable(t *testing.T) *qaIssPlay {
	t.Helper()
	tb := qaIssOpenTable(t)
	pl := &qaIssPlay{tb: tb}
	pl.dm, pl.ag = tb.dial(identity.RoleDM), tb.dial(identity.RoleAgent)
	pl.p1, pl.sp = tb.dial(identity.RolePlayer), tb.dial(identity.RoleSpectator)

	tb.appendAsDM("qa-iss-hall", &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "hall", Name: "Hall", GridWidth: 10, GridHeight: 10,
	}}})
	tb.appendAsDM("qa-iss-cellar", &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "cellar", Name: "Cellar", GridWidth: 4, GridHeight: 4,
	}}})
	pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "qa night"}}})
	party, other := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	for _, a := range []*vttv1.Actor{qaIssActor("hero", party), qaIssActor("patron", party), qaIssActor("lurker", other)} {
		pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}}})
	}
	pl.dm.must(qaIssCmdPlace("hero-tok", "hall", "hero", 1, 1))
	pl.dm.must(qaIssCmdPlace("patron-tok", "hall", "patron", 3, 1))
	pl.dm.must(qaIssCmdPlace("patron-cellar-tok", "cellar", "patron", 2, 1))
	pl.dm.must(qaIssCmdPlace("lurker-tok", "cellar", "lurker", 1, 1))
	pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: "hero", ParticipantId: tb.ids[identity.RolePlayer], Kind: party,
	}}})
	pl.sp.must(qaIssCmdPerch("hero"))
	pl.dm.must(qaIssCmdNarrate("perched"))
	pl.sp.untilNarration("perched")

	reason := "edging toward the bar"
	pl.actsAt = pl.p1.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: "hero-tok", To: qaIssAt(2, 1), Reason: &reason,
	}}})
	pl.p1.must(qaIssCmdNarrate("knuckles crack"))
	pl.dm.must(qaIssCmdAbility("lurker", "chair-swing"))
	pl.dm.must(qaIssCmdAbility("lurker", "splash-of-water"))
	pl.p1.must(qaIssCmdAbility("hero", "chair-swing"))
	pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveCondition{RemoveCondition: &vttv1.RemoveCondition{
		ActorId: "patron", ConditionId: qaIssDazed,
	}}})
	pl.dm.must(qaIssCmdAbility("lurker", "fists"))
	pl.ag.must(qaIssCmdNarrate("the barkeep sighs"))
	pl.dm.must(qaIssCmdNarrate(qaIssMarker))
	for _, c := range []*qaIssConn{pl.dm, pl.ag, pl.p1, pl.sp} {
		c.untilNarration(qaIssMarker)
	}
	return pl
}

func qaIssNames(e *vttv1.Envelope) []string {
	switch {
	case e.GetAbilityUsed() != nil:
		return append([]string{e.GetAbilityUsed().GetActorId()}, e.GetAbilityUsed().GetTargetIds()...)
	case e.GetResourceChanged() != nil:
		return []string{e.GetResourceChanged().GetActorId()}
	case e.GetConditionApplied() != nil:
		return []string{e.GetConditionApplied().GetActorId()}
	case e.GetConditionRemoved() != nil:
		return []string{e.GetConditionRemoved().GetActorId()}
	}
	return nil
}

func qaIssCheckWire(t *testing.T, who string, c *qaIssConn, log []*vttv1.Envelope, from int64) {
	t.Helper()
	byID := map[string]*vttv1.Envelope{}
	for _, e := range log {
		byID[e.GetEventId()] = e
	}
	sent := map[string]bool{}
	for _, f := range c.events() {
		if f.GetParticipantId() != "" || f.GetActorRole() != "" {
			t.Errorf("%s: sent %s seq %d with issuer participant=%q role=%q", who, qaIssKind(f), f.GetSequence(), f.GetParticipantId(), f.GetActorRole())
		}
		if cause := qaIssCause(f); cause != "" {
			t.Errorf("%s: sent %s seq %d with cause %q", who, qaIssKind(f), f.GetSequence(), cause)
		}
		if f.GetEventId() == "" {
			continue
		}
		e := byID[f.GetEventId()]
		if e == nil {
			t.Errorf("%s: sent event id %q the log does not hold", who, f.GetEventId())
			continue
		}
		// Take the entry's sequence: a log numbers its entries itself (VTT-321, VTT-371).
		want := qaIssLessIssuerAndCause(e)
		want.Sequence = f.GetSequence()
		if !proto.Equal(f, want) {
			t.Errorf("%s: sent %s that is not the logged event less issuer and cause:\n got  %v\n want %v", who, f.GetEventId(), f, want)
		}
		sent[f.GetEventId()] = true
	}
	var reached, kept, issued int
	for _, e := range log {
		if e.GetSequence() < from {
			continue
		}
		if e.GetParticipantId() != "" && e.GetActorRole() != "" {
			issued++
		}
		names := qaIssNames(e)
		if names == nil && e.GetTokenMoved() == nil && e.GetNarrationAdded() == nil {
			continue
		}
		unseen := false
		for _, n := range names {
			unseen = unseen || n == "lurker"
		}
		switch {
		case unseen && sent[e.GetEventId()]:
			t.Errorf("%s: %s seq %d names lurker, whom it never saw, and was sent", who, qaIssKind(e), e.GetSequence())
		case !unseen && !sent[e.GetEventId()]:
			t.Errorf("%s: %s seq %d (cause %q) was not sent", who, qaIssKind(e), e.GetSequence(), qaIssCause(e))
		case !unseen:
			reached++
			if qaIssCause(e) != "" {
				kept++
			}
		}
	}
	if reached < 14 || kept < 10 || issued == 0 {
		t.Errorf("%s: only %d events reached it, %d with a logged cause, %d issued: the scenario proves nothing", who, reached, kept, issued)
	}
}

// VTT-272 VTT-273 VTT-262 VTT-243 VTT-244
func TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause(t *testing.T) {
	pl := qaIssPlayTable(t)
	log := pl.tb.readLog()
	qaIssCheckWire(t, "live player", pl.p1, log, pl.actsAt)
	qaIssCheckWire(t, "live perched spectator", pl.sp, log, pl.actsAt)
}

// VTT-272 VTT-273 VTT-262 VTT-249
func TestQAIssACatchUpIsSentNoIssuerOrCause(t *testing.T) {
	pl := qaIssPlayTable(t)
	log := pl.tb.readLog()
	p1 := pl.tb.dial(identity.RolePlayer)
	p1.untilNarration(qaIssMarker)
	qaIssCheckWire(t, "catch-up player", p1, log, pl.actsAt)

	// Perch before waiting: a spectator watching nobody is sent nothing (VTT-341).
	sp := pl.tb.dial(identity.RoleSpectator)
	sp.must(qaIssCmdPerch("hero"))
	sp.untilNarration(qaIssMarker)
	hero, err := pl.tb.c.CharacterLog("hero")
	if err != nil {
		t.Fatal(err)
	}
	sp.until("hero's whole log", func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetSequence() == int64(len(hero))
	})
	for _, e := range sp.events() {
		if e.GetParticipantId() != "" || e.GetActorRole() != "" || qaIssCause(e) != "" {
			t.Errorf("catch-up spectator: %s seq %d carries issuer or cause: %v", qaIssKind(e), e.GetSequence(), e)
		}
		correction := e.GetResourceChanged() != nil || e.GetConditionRemoved() != nil
		if e.GetEventId() == "" && correction && (e.GetSessionId() != "" || e.GetOccurredAt() != nil) {
			t.Errorf("catch-up spectator: correction %s carries session or time: %v", qaIssKind(e), e)
		}
	}
}

// SPEC-016 Consequences: "the DM and the agent are sent each event as the log
// holds it".
// VTT-176 VTT-222
func TestQAIssTheDMAndTheAgentAreSentEachEventAsLogged(t *testing.T) {
	pl := qaIssPlayTable(t)
	log := pl.tb.readLog()
	late := pl.tb.dial(identity.RoleDM)
	late.untilNarration(qaIssMarker)
	var issued, caused int
	for _, e := range log {
		if e.GetParticipantId() != "" && e.GetActorRole() != "" {
			issued++
		}
		if qaIssCause(e) != "" {
			caused++
		}
	}
	if issued != len(log) || caused < 10 {
		t.Fatalf("log premise: %d of %d events issued, %d caused", issued, len(log), caused)
	}
	for who, c := range map[string]*qaIssConn{"live dm": pl.dm, "live agent": pl.ag, "catch-up dm": late} {
		got := c.events()
		if len(got) != len(log) {
			t.Errorf("%s: sent %d events, the log holds %d", who, len(got), len(log))
			continue
		}
		for i := range log {
			if !proto.Equal(got[i], log[i]) {
				t.Errorf("%s: event %d differs from the log:\n got  %v\n want %v", who, i, got[i], log[i])
			}
		}
	}
}
