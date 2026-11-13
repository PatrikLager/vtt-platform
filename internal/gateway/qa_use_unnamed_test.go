package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaUseTable struct {
	c    *campaign.Campaign
	rs   *rules.Ruleset
	base string
	tok  map[string]string
}

type qaUseConn struct {
	t      *testing.T
	ws     *websocket.Conn
	frames chan *vttv1.ServerFrame
	n      int
}

type qaUseRoller struct{}

func (qaUseRoller) Roll(n, _ int) ([]int, int) {
	out := make([]int, n)
	for i := range out {
		out[i] = 1
	}
	return out, n
}

func qaUseRefusal(actor, ability string) string {
	return fmt.Sprintf("gateway: use_ability: actor %q does not name ability %q", actor, ability)
}

func qaUseActor(id string, kind vttv1.ActorKind, abilities ...string) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       id,
		Kind:       kind,
		Attributes: map[string]int32{"brawn": 3, "grit": 1, "footing": 10},
		Resources: map[string]*vttv1.Resource{
			"drink":   {Current: 0, Max: 100},
			"stamina": {Current: 2, Max: 2},
		},
		AbilityIds: abilities,
	}
}

func qaUseOpen(t *testing.T, withRuleset bool) *qaUseTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	tok := map[string]string{}
	pid := map[string]string{}
	for _, p := range []struct {
		name string
		role identity.Role
	}{
		{"Dee", identity.RoleDM},
		{"Ada", identity.RoleAgent},
		{"Pia", identity.RolePlayer},
		{"Pete", identity.RolePlayer},
		{"Sid", identity.RoleSpectator},
	} {
		token, id, err := ids.CreateInvite(p.name, p.role)
		if err != nil {
			t.Fatalf("CreateInvite %s: %v", p.name, err)
		}
		tok[p.name], pid[p.name] = token, id
	}

	party, other := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	var seq int
	appendEv := func(env *vttv1.Envelope) {
		seq++
		env.EventId = fmt.Sprintf("qa-use-seed-%d", seq)
		env.OccurredAt = timestamppb.Now()
		env.ParticipantId = pid["Dee"]
		env.ActorRole = string(identity.RoleDM)
		if _, err := c.Append(env); err != nil {
			t.Fatalf("seed %d: %v", seq, err)
		}
	}
	for _, sc := range []string{"tavern", "cellar"} {
		appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: sc, Name: sc, GridWidth: 8, GridHeight: 8,
		}}})
	}
	placed := []struct {
		actor *vttv1.Actor
		scene string
		x, y  int32
	}{
		{qaUseActor("brawler", party, "fists"), "tavern", 1, 1},
		{qaUseActor("patron", other), "tavern", 2, 1},
		{qaUseActor("twin", other, "fists", "chair-swing"), "tavern", 3, 1},
		{qaUseActor("rival", party, "fists"), "tavern", 1, 2},
		{qaUseActor("shouter", other, "Fists"), "tavern", 2, 0},
		{qaUseActor("fist", other, "fist"), "tavern", 3, 0},
		{qaUseActor("furious", other, "fists-of-fury"), "tavern", 2, 2},
		{qaUseActor("spaced", other, "fists "), "tavern", 3, 2},
		{qaUseActor("lurker", other, "fists"), "cellar", 0, 0},
	}
	for _, p := range placed {
		appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: p.actor}}})
		appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-" + p.actor.GetActorId(), SceneId: p.scene, ActorId: p.actor.GetActorId(),
			Position: &vttv1.GridPosition{X: p.x, Y: p.y},
		}}})
	}
	for actor, player := range map[string]string{"brawler": "Pia", "rival": "Pete"} {
		appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actor, ParticipantId: pid[player], Kind: party,
		}}})
	}

	srv := gateway.New(c, ids)
	var rs *rules.Ruleset
	if withRuleset {
		rs, err = rules.Load(filepath.Join("..", "..", "rulesets", "tavern-brawl"))
		if err != nil {
			t.Fatalf("rules.Load: %v", err)
		}
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hs.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qaUseTable{c: c, rs: rs, base: hs.URL, tok: tok}
}

func qaUseDial(t *testing.T, tb *qaUseTable, who string) *qaUseConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u := strings.Replace(tb.base, "http://", "ws://", 1) + "/ws?token=" + url.QueryEscape(tb.tok[who]) + "&after=0"
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial as %s: %v", who, err)
	}
	ws.SetReadLimit(1 << 22)
	conn := &qaUseConn{t: t, ws: ws, frames: make(chan *vttv1.ServerFrame, 4096)}
	go func() {
		defer close(conn.frames)
		for {
			_, data, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, f); err != nil {
				return
			}
			conn.frames <- f
		}
	}()
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func (q *qaUseConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	q.t.Helper()
	q.n++
	cmd.RequestId = fmt.Sprintf("qa-use-%d", q.n)
	data, err := protojson.Marshal(cmd)
	if err != nil {
		q.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := q.ws.Write(ctx, websocket.MessageText, data); err != nil {
		q.t.Fatalf("write %s: %v", cmd.RequestId, err)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case f, ok := <-q.frames:
			if !ok {
				q.t.Fatalf("connection closed before the result of %s", cmd.RequestId)
			}
			if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
				return r
			}
		case <-deadline:
			q.t.Fatalf("no result for %s", cmd.RequestId)
		}
	}
}

func qaUseAbility(actor, ability string, targets ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: actor, AbilityId: ability, TargetIds: targets,
	}}}
}

func qaUseProbe() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
		Text: "qa probe", OutOfCharacter: true,
	}}}
}

func qaUseHead(t *testing.T, c *campaign.Campaign) int64 {
	t.Helper()
	_, unsub, head, err := c.Subscribe(0, 4096)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	unsub()
	return head
}

func qaUseEventsAfter(t *testing.T, c *campaign.Campaign, after int64) []*vttv1.Envelope {
	t.Helper()
	events, unsub, head, err := c.Subscribe(after, 4096)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	deadline := time.After(5 * time.Second)
	for head > after && (len(out) == 0 || out[len(out)-1].GetSequence() < head) {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatalf("subscription closed")
			}
			out = append(out, e)
		case <-deadline:
			t.Fatalf("catch-up to %d did not arrive", head)
		}
	}
	return out
}

func qaUseRefused(t *testing.T, tb *qaUseTable, q *qaUseConn, cmd *vttv1.ClientCommand, want string) *vttv1.CommandResult {
	t.Helper()
	before := qaUseHead(t, tb.c)
	r := q.send(cmd)
	if r.GetOk() {
		t.Errorf("%v: ok=true (sequence %d), want refused with %q", cmd.GetUseAbility(), r.GetSequence(), want)
	} else if want != "" && r.GetError() != want {
		t.Errorf("%v: error %q, want %q", cmd.GetUseAbility(), r.GetError(), want)
	}
	if r.GetSequence() != 0 || len(r.GetWarnings()) != 0 {
		t.Errorf("%v: refusal carries sequence %d, warnings %v", cmd.GetUseAbility(), r.GetSequence(), r.GetWarnings())
	}
	if after := qaUseHead(t, tb.c); after != before {
		t.Errorf("%v: head moved %d -> %d on a refusal", cmd.GetUseAbility(), before, after)
	}
	return r
}

func qaUseStillUsable(t *testing.T, tb *qaUseTable, q *qaUseConn) {
	t.Helper()
	before := qaUseHead(t, tb.c)
	r := q.send(qaUseProbe())
	if !r.GetOk() || r.GetSequence() != before+1 {
		t.Errorf("probe after the refusal: ok=%v error=%q sequence=%d, want ok at %d", r.GetOk(), r.GetError(), r.GetSequence(), before+1)
	}
}

func qaUseResolved(t *testing.T, tb *qaUseTable, q *qaUseConn, actor, ability string, targets ...string) {
	t.Helper()
	before := qaUseHead(t, tb.c)
	r := q.send(qaUseAbility(actor, ability, targets...))
	if !r.GetOk() {
		t.Errorf("use %s %s %v: refused with %q, want resolved", actor, ability, targets, r.GetError())
		return
	}
	if r.GetSequence() != before+1 {
		t.Errorf("use %s %s: sequence %d, want %d", actor, ability, r.GetSequence(), before+1)
	}
	evs := qaUseEventsAfter(t, tb.c, before)
	if len(evs) == 0 {
		t.Errorf("use %s %s: ok, but nothing appended", actor, ability)
		return
	}
	used := evs[0].GetAbilityUsed()
	if used.GetActorId() != actor || used.GetAbilityId() != ability {
		t.Errorf("use %s %s: first appended event %v, want its AbilityUsed", actor, ability, evs[0])
	}
}

var qaUseRoles = []string{"Dee", "Ada", "Pia"}

// VTT-690
func TestQAUseEachRoleIsRefusedAnAbilityTheSheetDoesNotName(t *testing.T) {
	for _, who := range qaUseRoles {
		t.Run(who, func(t *testing.T) {
			tb := qaUseOpen(t, true)
			q := qaUseDial(t, tb, who)
			qaUseRefused(t, tb, q, qaUseAbility("brawler", "chair-swing", "patron"), qaUseRefusal("brawler", "chair-swing"))
			qaUseStillUsable(t, tb, q)
			qaUseRefused(t, tb, q, qaUseAbility("brawler", "sober-up", "brawler"), qaUseRefusal("brawler", "sober-up"))
			qaUseResolved(t, tb, q, "brawler", "fists", "patron")
		})
	}
}

// VTT-690
func TestQAUseAnActorNamingNoAbilityIsRefusedEveryOne(t *testing.T) {
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Dee")
	for _, ab := range []string{"fists", "chair-swing", "splash-of-water"} {
		qaUseRefused(t, tb, q, qaUseAbility("patron", ab, "brawler"), qaUseRefusal("patron", ab))
	}
	qaUseStillUsable(t, tb, q)
}

// VTT-690
func TestQAUseTheRefusedUsesWouldResolveForAnActorWhoseSheetNamesThem(t *testing.T) {
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Dee")
	qaUseResolved(t, tb, q, "twin", "chair-swing", "patron")
	qaUseResolved(t, tb, q, "twin", "fists", "patron")
}

// VTT-690
func TestQAUseEachRoleResolvesAUseTheSheetNames(t *testing.T) {
	for _, who := range qaUseRoles {
		t.Run(who, func(t *testing.T) {
			tb := qaUseOpen(t, true)
			q := qaUseDial(t, tb, who)
			qaUseResolved(t, tb, q, "brawler", "fists", "patron")
		})
	}
}

// VTT-690
func TestQAUseAnAbilityTheRulesetDoesNotHoldIsRefusedAsUnnamed(t *testing.T) {
	for _, who := range qaUseRoles {
		t.Run(who, func(t *testing.T) {
			tb := qaUseOpen(t, true)
			q := qaUseDial(t, tb, who)
			qaUseRefused(t, tb, q, qaUseAbility("brawler", "kick", "patron"), qaUseRefusal("brawler", "kick"))
			qaUseStillUsable(t, tb, q)
		})
	}
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Dee")
	qaUseRefused(t, tb, q, qaUseAbility("patron", "kick", "brawler"), qaUseRefusal("patron", "kick"))
	qaUseRefused(t, tb, q, qaUseAbility("lurker", "kick", "lurker"), qaUseRefusal("lurker", "kick"))
}

// VTT-690
func TestQAUseAnEmptyAbilityIdIsRefusedAsUnnamed(t *testing.T) {
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Dee")
	qaUseRefused(t, tb, q, qaUseAbility("brawler", "", "patron"), qaUseRefusal("brawler", ""))
}

// Keep an actor the campaign does not hold to the rules engine's own refusal.
func TestQAUseAnActorTheCampaignDoesNotHoldGetsTheRulesEnginesRefusal(t *testing.T) {
	for _, who := range []string{"Dee", "Ada"} {
		t.Run(who, func(t *testing.T) {
			tb := qaUseOpen(t, true)
			q := qaUseDial(t, tb, who)
			for _, ab := range []string{"fists", "chair-swing", "kick"} {
				cmd := qaUseAbility("ghost", ab, "patron")
				_, engineErr := rules.Resolve(tb.rs, tb.c.State(), cmd.GetUseAbility(), qaUseRoller{})
				if engineErr == nil {
					t.Fatalf("rules.Resolve accepted a ghost caster with %s", ab)
				}
				t.Logf("ghost %s: the rules engine answers %q", ab, engineErr.Error())
				r := qaUseRefused(t, tb, q, cmd, engineErr.Error())
				if r.GetError() == qaUseRefusal("ghost", ab) {
					t.Errorf("ghost %s: answered with the sheet refusal", ab)
				}
			}
			qaUseRefused(t, tb, q, qaUseAbility("ghost", "fists", "patron"), `rules: resolve: unknown actor "ghost"`)
			qaUseStillUsable(t, tb, q)
		})
	}
}

// VTT-144
func TestQAUseAuthorizationAnswersBeforeTheSheet(t *testing.T) {
	cases := []struct {
		who, actor string
	}{
		{"Pia", "rival"},
		{"Pia", "twin"},
		{"Pia", "patron"},
		{"Sid", "brawler"},
		{"Pete", "brawler"},
	}
	for _, c := range cases {
		t.Run(c.who+"-"+c.actor, func(t *testing.T) {
			tb := qaUseOpen(t, true)
			q := qaUseDial(t, tb, c.who)
			named := qaUseRefused(t, tb, q, qaUseAbility(c.actor, "fists", "patron"), "")
			for _, ab := range []string{"chair-swing", "kick", "sober-up"} {
				r := qaUseRefused(t, tb, q, qaUseAbility(c.actor, ab, "patron"), named.GetError())
				if strings.Contains(r.GetError(), "does not name ability") {
					t.Errorf("%s as %s using %s: answered by the sheet: %q", c.who, c.actor, ab, r.GetError())
				}
			}
		})
	}
}

// VTT-472
func TestQAUseThePlayersSightGateAnswersBeforeTheSheet(t *testing.T) {
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Pia")
	unseen := rules.UnknownTarget("lurker").Error()
	for _, ab := range []string{"fists", "chair-swing", "kick", "sober-up"} {
		qaUseRefused(t, tb, q, qaUseAbility("brawler", ab, "lurker"), unseen)
		qaUseRefused(t, tb, q, qaUseAbility("brawler", ab, "patron", "lurker"), unseen)
	}
	absent := rules.UnknownTarget("nobody").Error()
	qaUseRefused(t, tb, q, qaUseAbility("brawler", "chair-swing", "nobody"), absent)
	qaUseRefused(t, tb, q, qaUseAbility("brawler", "fists", "nobody"), absent)
	qaUseStillUsable(t, tb, q)
}

// VTT-482
func TestQAUseTheDMAndTheAgentMeetTheSheetWhereThePlayerMeetsSight(t *testing.T) {
	for _, who := range []string{"Dee", "Ada"} {
		t.Run(who, func(t *testing.T) {
			tb := qaUseOpen(t, true)
			q := qaUseDial(t, tb, who)
			qaUseRefused(t, tb, q, qaUseAbility("brawler", "chair-swing", "lurker"), qaUseRefusal("brawler", "chair-swing"))
			qaUseRefused(t, tb, q, qaUseAbility("brawler", "kick", "lurker"), qaUseRefusal("brawler", "kick"))
		})
	}
}

// VTT-487
func TestQAUseAPlayerWhoseEyesSeeEveryTargetMeetsTheSheet(t *testing.T) {
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Pia")
	qaUseRefused(t, tb, q, qaUseAbility("brawler", "chair-swing", "patron"), qaUseRefusal("brawler", "chair-swing"))
	qaUseRefused(t, tb, q, qaUseAbility("brawler", "kick", "patron", "twin"), qaUseRefusal("brawler", "kick"))
	qaUseRefused(t, tb, q, qaUseAbility("brawler", "sober-up", "brawler"), qaUseRefusal("brawler", "sober-up"))
}

// VTT-132
func TestQAUseNoRulesetAnswersBeforeTheSheet(t *testing.T) {
	for _, who := range qaUseRoles {
		t.Run(who, func(t *testing.T) {
			tb := qaUseOpen(t, false)
			q := qaUseDial(t, tb, who)
			for _, ab := range []string{"fists", "chair-swing", "kick"} {
				qaUseRefused(t, tb, q, qaUseAbility("brawler", ab, "patron"), "gateway: no ruleset loaded")
			}
			qaUseStillUsable(t, tb, q)
		})
	}
}

// VTT-690
func TestQAUseASheetNamingTheAbilityInAnotherSpellingDoesNotOfferIt(t *testing.T) {
	tb := qaUseOpen(t, true)
	q := qaUseDial(t, tb, "Dee")
	for _, actor := range []string{"shouter", "fist", "furious", "spaced"} {
		qaUseRefused(t, tb, q, qaUseAbility(actor, "fists", "patron"), qaUseRefusal(actor, "fists"))
	}
	for _, ab := range []string{"Fists", "FISTS", "fist", "fists-of-fury", "fists ", " fists"} {
		qaUseRefused(t, tb, q, qaUseAbility("brawler", ab, "patron"), qaUseRefusal("brawler", ab))
	}
	qaUseStillUsable(t, tb, q)

	p := qaUseDial(t, tb, "Pia")
	qaUseRefused(t, tb, p, qaUseAbility("brawler", "Fists", "patron"), qaUseRefusal("brawler", "Fists"))
	qaUseRefused(t, tb, p, qaUseAbility("brawler", "fist", "patron"), qaUseRefusal("brawler", "fist"))
}
