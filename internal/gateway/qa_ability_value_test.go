package gateway_test

// Pins the give and take rows over the wire; see SPEC-025.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaAVSession = "sess-qa-av"
	qaAVScene   = "scn-qa-av"
	qaAVBrawl   = "../../rulesets/tavern-brawl"
)

var qaAVRoles = []identity.Role{
	identity.RoleDM, identity.RoleAgent,
	identity.RolePlayer, identity.RoleSpectator,
}

type qaAVTable struct {
	c      *campaign.Campaign
	wsURL  string
	tokens map[identity.Role]string
	pids   map[identity.Role]string
	reqN   int
}

type qaAVOutcome struct {
	res      *vttv1.CommandResult
	appended []*vttv1.Envelope
	before   *engine.State
	after    *engine.State
	role     identity.Role
	pid      string
}

func qaAVRes(cur, mx int32) *vttv1.Resource {
	return &vttv1.Resource{Current: cur, Max: mx}
}

func qaAVActor(id string, abilities []string, res map[string]*vttv1.Resource) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       "QA " + id,
		Attributes: map[string]int32{"brawn": 3, "grit": 1, "footing": 10},
		Resources:  res,
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		AbilityIds: abilities,
	}
}

// qaAVCast is the standard cast; act-u, act-o and act-c are seeded unfit on purpose.
func qaAVCast() []*vttv1.Actor {
	return []*vttv1.Actor{
		qaAVActor("act-a", []string{"fists"},
			map[string]*vttv1.Resource{"drink": qaAVRes(0, 4)}),
		qaAVActor("act-t", []string{"fists"},
			map[string]*vttv1.Resource{"drink": qaAVRes(0, 5)}),
		qaAVActor("act-c", []string{"fists"},
			map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2), "drink": qaAVRes(0, 4)}),
		qaAVActor("act-d", nil, nil),
		qaAVActor("act-u", []string{"chair-swing"}, nil),
		qaAVActor("act-e", []string{"fists", "chair-swing", "sober-up"},
			map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2), "drink": qaAVRes(2, 4)}),
		qaAVActor("act-r", []string{"chair-swing", "fists", "chair-swing"},
			map[string]*vttv1.Resource{"stamina": qaAVRes(2, 2)}),
		qaAVActor("act-o", []string{"old-move", "fists"},
			map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2), "drink": qaAVRes(1, 4)}),
		qaAVActor("act-s", []string{"sober-up", "chair-swing", "fists"},
			map[string]*vttv1.Resource{"stamina": qaAVRes(2, 3)}),
	}
}

func qaAVOpen(t *testing.T, rs *rules.Ruleset, actors ...*vttv1.Actor) *qaAVTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	tb := &qaAVTable{c: c, tokens: map[identity.Role]string{}, pids: map[identity.Role]string{}}
	for _, r := range qaAVRoles {
		tok, id, err := ids.CreateInvite("qa-"+string(r), r)
		if err != nil {
			t.Fatalf("CreateInvite %s: %v", r, err)
		}
		tb.tokens[r] = tok
		tb.pids[r] = id
	}
	seed := []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "qa"}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: qaAVScene, Name: "QA", GridWidth: 16, GridHeight: 4,
		}}},
	}
	for i, a := range actors {
		seed = append(seed,
			&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}},
			&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
				TokenId: "tok-" + a.GetActorId(), SceneId: qaAVScene, ActorId: a.GetActorId(),
				Position: &vttv1.GridPosition{X: int32(i), Y: 0},
			}}})
		if a.GetActorId() == "act-a" {
			seed = append(seed, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
				ActorControlGranted: &vttv1.ActorControlGranted{
					ActorId: "act-a", ParticipantId: tb.pids[identity.RolePlayer],
					Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
				},
			}})
		}
	}
	for i, e := range seed {
		e.EventId = fmt.Sprintf("evt-qa-av-seed-%d", i)
		e.SessionId = qaAVSession
		e.OccurredAt = timestamppb.Now()
		e.ActorRole = string(identity.RoleDM)
		e.ParticipantId = tb.pids[identity.RoleDM]
	}
	if _, err := c.AppendBatch(seed); err != nil {
		t.Fatalf("seed AppendBatch: %v", err)
	}
	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	tb.wsURL = "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws"
	return tb
}

func qaAVLoad(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load %s: %v", dir, err)
	}
	return rs
}

// qaAVBrawlWith copies tavern-brawl and writes files into the copy.
func qaAVBrawlWith(t *testing.T, files map[string]string, props ...map[string]any) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tavern-brawl")
	if err := os.CopyFS(dir, os.DirFS(qaAVBrawl)); err != nil {
		t.Fatalf("CopyFS: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if len(props) > 0 {
		path := filepath.Join(dir, "ruleset.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read ruleset.json: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode ruleset.json: %v", err)
		}
		list, _ := doc["properties"].([]any)
		for _, p := range props {
			list = append(list, p)
		}
		doc["properties"] = list
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatalf("encode ruleset.json: %v", err)
		}
		if err := os.WriteFile(path, out, 0o600); err != nil {
			t.Fatalf("write ruleset.json: %v", err)
		}
	}
	return dir
}

func qaAVLimited(id, resource string) string {
	return fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "usage": { "limited": { "resource": %q, "cost": 1 } },
  "compose": [
    { "atom": "brawl-reach", "bind": { "reach": 1 } },
    { "atom": "footing-contest", "bind": { "attack_stat": "brawn" } },
    { "atom": "drink-soak", "bind": { "amount": "1 + 1d4" } },
    { "atom": "stagger-from-drink", "bind": {} }
  ]
}`, id, "QA "+id, resource)
}

func (tb *qaAVTable) dial(ctx context.Context, t *testing.T, role identity.Role) *websocket.Conn {
	t.Helper()
	url := tb.wsURL + "?token=" + tb.tokens[role] + "&after=0"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial as %s: %v", role, err)
	}
	conn.SetReadLimit(1 << 22)
	return conn
}

func qaAVRead(ctx context.Context, t *testing.T, conn *websocket.Conn) *vttv1.ServerFrame {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var f vttv1.ServerFrame
	if err := protojson.Unmarshal(data, &f); err != nil {
		t.Fatalf("decode frame %s: %v", data, err)
	}
	return &f
}

// log reads the whole log as the DM sees it, up to the catch-up head.
func (tb *qaAVTable) log(t *testing.T) []*vttv1.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	conn := tb.dial(ctx, t, identity.RoleDM)
	defer conn.Close(websocket.StatusNormalClosure, "")
	first := qaAVRead(ctx, t, conn)
	if first.GetCatchUpHead() == nil {
		t.Fatalf("first frame is not a CatchUpHead: %v", first)
	}
	head := first.GetCatchUpHead().GetHeadSequence()
	var evs []*vttv1.Envelope
	for head > 0 && (len(evs) == 0 || evs[len(evs)-1].GetSequence() < head) {
		if e := qaAVRead(ctx, t, conn).GetEvent(); e != nil {
			evs = append(evs, e)
		}
	}
	return evs
}

func (tb *qaAVTable) send(t *testing.T, role identity.Role, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t.Helper()
	tb.reqN++
	cmd.RequestId = fmt.Sprintf("req-qa-av-%d", tb.reqN)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	conn := tb.dial(ctx, t, role)
	defer conn.Close(websocket.StatusNormalClosure, "")
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("encode command: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("write command: %v", err)
	}
	for {
		if r := qaAVRead(ctx, t, conn).GetResult(); r != nil && r.GetRequestId() == cmd.GetRequestId() {
			return r
		}
	}
}

func (tb *qaAVTable) do(t *testing.T, role identity.Role, cmd *vttv1.ClientCommand) qaAVOutcome {
	t.Helper()
	logBefore := tb.log(t)
	stBefore := tb.c.State()
	res := tb.send(t, role, cmd)
	logAfter := tb.log(t)
	stAfter := tb.c.State()
	if len(logAfter) < len(logBefore) {
		t.Fatalf("log shrank from %d to %d", len(logBefore), len(logAfter))
	}
	for i := range logBefore {
		if !proto.Equal(logBefore[i], logAfter[i]) {
			t.Fatalf("log entry %d changed", i)
		}
	}
	return qaAVOutcome{
		res: res, appended: logAfter[len(logBefore):],
		before: stBefore, after: stAfter, role: role, pid: tb.pids[role],
	}
}

func qaAVGive(actor, ability string, res map[string]*vttv1.Resource) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GiveAbility{
		GiveAbility: &vttv1.GiveAbility{ActorId: actor, AbilityId: ability, Resources: res},
	}}
}

func qaAVTake(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_TakeAbility{
		TakeAbility: &vttv1.TakeAbility{ActorId: actor, AbilityId: ability},
	}}
}

func qaAVDescribe(e *vttv1.Envelope) string {
	switch p := e.GetPayload().(type) {
	case *vttv1.Envelope_AbilityGiven:
		return "AbilityGiven " + p.AbilityGiven.GetActorId() + " " + p.AbilityGiven.GetAbilityId()
	case *vttv1.Envelope_AbilityTaken:
		return "AbilityTaken " + p.AbilityTaken.GetActorId() + " " + p.AbilityTaken.GetAbilityId()
	case *vttv1.Envelope_ResourceAdded:
		v := p.ResourceAdded.GetValue()
		if v == nil {
			return "ResourceAdded " + p.ResourceAdded.GetActorId() + " " +
				p.ResourceAdded.GetResource() + " <no value>"
		}
		return fmt.Sprintf("ResourceAdded %s %s %d/%d", p.ResourceAdded.GetActorId(),
			p.ResourceAdded.GetResource(), v.GetCurrent(), v.GetMax())
	case *vttv1.Envelope_ResourceRemoved:
		return "ResourceRemoved " + p.ResourceRemoved.GetActorId() + " " + p.ResourceRemoved.GetResource()
	default:
		return fmt.Sprintf("%T", p)
	}
}

func qaAVWantAppended(t *testing.T, o qaAVOutcome, want ...string) {
	t.Helper()
	if !o.res.GetOk() {
		t.Fatalf("refused: %q; want appended %v", o.res.GetError(), want)
	}
	got := make([]string, 0, len(o.appended))
	for _, e := range o.appended {
		got = append(got, qaAVDescribe(e))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appended %q, want %q", got, want)
	}
	for i, e := range o.appended {
		if e.GetParticipantId() != o.pid || e.GetActorRole() != string(o.role) {
			t.Errorf("event %d stamped %q/%q, want %q/%q",
				i, e.GetParticipantId(), e.GetActorRole(), o.pid, o.role)
		}
		if i > 0 && e.GetSequence() != o.appended[i-1].GetSequence()+1 {
			t.Errorf("event %d sequence %d does not follow %d",
				i, e.GetSequence(), o.appended[i-1].GetSequence())
		}
		if e.GetEventId() == "" || (i > 0 && e.GetEventId() == o.appended[i-1].GetEventId()) {
			t.Errorf("event %d id %q is empty or repeats the one before", i, e.GetEventId())
		}
	}
	qaAVSameBoard(t, o)
}

func qaAVWantRefused(t *testing.T, o qaAVOutcome) {
	t.Helper()
	if o.res.GetOk() {
		t.Fatalf("accepted (sequence %d), want refused", o.res.GetSequence())
	}
	if o.res.GetError() == "" {
		t.Errorf("refusal carries no error text")
	}
	t.Logf("refused: %s", o.res.GetError())
	if len(o.appended) != 0 {
		got := make([]string, 0, len(o.appended))
		for _, e := range o.appended {
			got = append(got, qaAVDescribe(e))
		}
		t.Fatalf("refused command appended %q", got)
	}
	if len(o.before.Actors) != len(o.after.Actors) {
		t.Fatalf("actor count %d -> %d", len(o.before.Actors), len(o.after.Actors))
	}
	for id, a := range o.before.Actors {
		if !proto.Equal(a, o.after.Actors[id]) {
			t.Errorf("actor %s changed by a refused command:\nbefore %v\nafter  %v",
				id, a, o.after.Actors[id])
		}
	}
	qaAVSameBoard(t, o)
}

func qaAVSameBoard(t *testing.T, o qaAVOutcome) {
	t.Helper()
	if !reflect.DeepEqual(o.before.Tokens, o.after.Tokens) {
		t.Errorf("tokens changed")
	}
	if !reflect.DeepEqual(o.before.Conditions, o.after.Conditions) {
		t.Errorf("conditions changed")
	}
}

// qaAVWantActor holds the actor to want and every other actor unchanged.
func qaAVWantActor(t *testing.T, o qaAVOutcome, want *vttv1.Actor) {
	t.Helper()
	got := o.after.Actors[want.GetActorId()]
	if !proto.Equal(got, want) {
		t.Errorf("actor %s:\ngot  %v\nwant %v", want.GetActorId(), got, want)
	}
	for id, a := range o.before.Actors {
		if id != want.GetActorId() && !proto.Equal(a, o.after.Actors[id]) {
			t.Errorf("bystander %s changed", id)
		}
	}
}

func qaAVBefore(t *testing.T, o qaAVOutcome, id string) *vttv1.Actor {
	t.Helper()
	a, ok := o.before.Actors[id]
	if !ok {
		t.Fatalf("fixture: no actor %s before", id)
	}
	return proto.CloneOf(a)
}

func qaAVShape(t *testing.T, o qaAVOutcome, actor string, names ...string) {
	t.Helper()
	msg := o.res.GetError()
	re := regexp.MustCompile(`^gateway: give_ability: actor "` + regexp.QuoteMeta(actor) + `": [^ :]+: .+`)
	if !re.MatchString(msg) {
		t.Errorf("refusal %q lacks the shape `gateway: give_ability: actor %q: <field>: <reason>`", msg, actor)
	}
	for _, n := range names {
		if !strings.Contains(msg, n) {
			t.Errorf("refusal %q does not name %q", msg, n)
		}
	}
}

func qaAVContains(t *testing.T, o qaAVOutcome, want string) {
	t.Helper()
	if !strings.Contains(o.res.GetError(), want) {
		t.Errorf("refusal %q does not contain %q", o.res.GetError(), want)
	}
}

func qaAVIssuers() []identity.Role {
	return []identity.Role{identity.RoleDM, identity.RoleAgent}
}

// VTT-760, VTT-791, VTT-799: the value as carried, 0 and no max included.
func TestQAAVGiveCarriesTheResourceAtItsValue(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	values := []struct{ cur, mx int32 }{{2, 2}, {0, 0}, {0, 2}, {1, 0}, {5, 2}, {-1, 2}}
	for _, role := range qaAVIssuers() {
		for _, v := range values {
			t.Run(fmt.Sprintf("%s/%d-of-%d", role, v.cur, v.mx), func(t *testing.T) {
				tb := qaAVOpen(t, rs, qaAVCast()...)
				o := tb.do(t, role, qaAVGive("act-a", "chair-swing",
					map[string]*vttv1.Resource{"stamina": qaAVRes(v.cur, v.mx)}))
				qaAVWantAppended(t, o, "AbilityGiven act-a chair-swing",
					fmt.Sprintf("ResourceAdded act-a stamina %d/%d", v.cur, v.mx))
				want := qaAVBefore(t, o, "act-a")
				want.AbilityIds = append(want.AbilityIds, "chair-swing")
				want.Resources["stamina"] = qaAVRes(v.cur, v.mx)
				qaAVWantActor(t, o, want)
				if _, held := o.after.Actors["act-a"].GetResources()["stamina"]; !held {
					t.Errorf("sheet holds no stamina after the give")
				}
				t.Logf("result sequence %d, appended %d..%d", o.res.GetSequence(),
					o.appended[0].GetSequence(), o.appended[len(o.appended)-1].GetSequence())
			})
		}
	}
}

// VTT-760 and the ticket's item 1: a use after the give resolves at 1 and is
// refused as too few, not as missing, at 0.
func TestQAAVGivenAbilityIsUsableAtItsValue(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	use := func(t *testing.T, tb *qaAVTable) *vttv1.CommandResult {
		t.Helper()
		return tb.send(t, identity.RoleDM, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{
			UseAbility: &vttv1.UseAbility{ActorId: "act-a", AbilityId: "chair-swing", TargetIds: []string{"act-t"}},
		}})
	}
	t.Run("at-1", func(t *testing.T) {
		tb := qaAVOpen(t, rs, qaAVCast()...)
		o := tb.do(t, identity.RoleDM, qaAVGive("act-a", "chair-swing",
			map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2)}))
		qaAVWantAppended(t, o, "AbilityGiven act-a chair-swing", "ResourceAdded act-a stamina 1/2")
		if r := use(t, tb); !r.GetOk() {
			t.Fatalf("use after the give refused: %q", r.GetError())
		}
	})
	t.Run("at-0", func(t *testing.T) {
		tb := qaAVOpen(t, rs, qaAVCast()...)
		o := tb.do(t, identity.RoleDM, qaAVGive("act-a", "chair-swing",
			map[string]*vttv1.Resource{"stamina": qaAVRes(0, 2)}))
		qaAVWantAppended(t, o, "AbilityGiven act-a chair-swing", "ResourceAdded act-a stamina 0/2")
		r := use(t, tb)
		if r.GetOk() {
			t.Fatalf("use at 0 accepted")
		}
		if !strings.Contains(r.GetError(), `insufficient "stamina"`) || strings.Contains(r.GetError(), "has no resource") {
			t.Errorf("use at 0 refused as %q, want too few, not missing", r.GetError())
		}
	})
}

// VTT-761, VTT-762, VTT-763, VTT-718, VTT-717, VTT-720, VTT-800, VTT-801.
func TestQAAVGiveRefusalsAppendNothing(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	res := func(kv ...any) map[string]*vttv1.Resource {
		m := map[string]*vttv1.Resource{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1].(*vttv1.Resource)
		}
		return m
	}
	cases := []struct {
		name  string
		cmd   *vttv1.ClientCommand
		check func(t *testing.T, o qaAVOutcome)
	}{
		// VTT-761
		{"limited-without-its-value", qaAVGive("act-a", "chair-swing", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a", "chair-swing", "stamina")
			}},
		// VTT-761: a sheet naming the ability without its resource
		{"limited-without-its-value-unfit-sheet", qaAVGive("act-u", "chair-swing", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-u", "chair-swing", "stamina")
			}},
		// VTT-762
		{"value-for-a-held-resource", qaAVGive("act-c", "chair-swing", res("stamina", qaAVRes(2, 2))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-c", "stamina")
				if got := o.after.Actors["act-c"].GetResources()["stamina"]; !proto.Equal(got, qaAVRes(1, 2)) {
					t.Errorf("stamina now %v, want 1/2", got)
				}
			}},
		// VTT-763: a declared key the ability does not spend, beside its own
		{"declared-key-not-spent", qaAVGive("act-d", "chair-swing",
			res("stamina", qaAVRes(2, 2), "drink", qaAVRes(1, 4))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-d", "chair-swing", "drink")
			}},
		// VTT-763: an undeclared key
		{"undeclared-key", qaAVGive("act-a", "chair-swing",
			res("stamina", qaAVRes(2, 2), "luck", qaAVRes(1, 1))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a", "chair-swing", "luck")
			}},
		// VTT-763: any key for an at-will ability
		{"at-will-with-a-key", qaAVGive("act-d", "sober-up", res("stamina", qaAVRes(1, 2))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-d", "sober-up", "stamina")
			}},
		// VTT-763 before VTT-762 (D3's order): at-will, key the sheet holds
		{"at-will-with-a-held-key", qaAVGive("act-a", "sober-up", res("drink", qaAVRes(1, 4))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a", "sober-up", "drink")
			}},
		// VTT-718, text as SPEC-013 states it
		{"ability-the-ruleset-does-not-hold", qaAVGive("act-a", "no-such", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				want := `gateway: give_ability: actor "act-a": ability_ids: names ability "no-such", which the ruleset does not hold`
				if o.res.GetError() != want {
					t.Errorf("refusal %q, want %q", o.res.GetError(), want)
				}
			}},
		// VTT-718 before the key check (D3's order)
		{"unknown-ability-with-a-key", qaAVGive("act-a", "no-such", res("stamina", qaAVRes(1, 2))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a", "ability_ids", "no-such")
			}},
		// VTT-718: an empty id is one the ruleset does not hold
		{"empty-ability-id", qaAVGive("act-a", "", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a", "ability_ids")
			}},
		// VTT-717
		{"unknown-actor", qaAVGive("act-ghost", "fists", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, `ability_given names unknown actor "act-ghost"`)
			}},
		// VTT-717, D3: an actor the snapshot lacks is left to the fold
		{"unknown-actor-limited-without-value", qaAVGive("act-ghost", "chair-swing", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, `ability_given names unknown actor "act-ghost"`)
			}},
		// VTT-717
		{"unknown-actor-with-its-value", qaAVGive("act-ghost", "chair-swing", res("stamina", qaAVRes(1, 2))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, `ability_given names unknown actor "act-ghost"`)
			}},
		// VTT-720
		{"already-named", qaAVGive("act-a", "fists", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, `actor "act-a" already names ability "fists"`)
			}},
		// VTT-720, D3: the whole batch is refused, the ResourceAdded with it
		{"already-named-with-its-value", qaAVGive("act-u", "chair-swing", res("stamina", qaAVRes(1, 2))),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, `actor "act-u" already names ability "chair-swing"`)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb := qaAVOpen(t, rs, qaAVCast()...)
			o := tb.do(t, identity.RoleDM, tc.cmd)
			qaAVWantRefused(t, o)
			tc.check(t, o)
		})
	}
}

// VTT-764: the AbilityGiven alone, at-will or with the resource already held.
func TestQAAVGiveAppendsTheAbilityAlone(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	for _, role := range qaAVIssuers() {
		t.Run(string(role)+"/at-will", func(t *testing.T) {
			tb := qaAVOpen(t, rs, qaAVCast()...)
			o := tb.do(t, role, qaAVGive("act-a", "sober-up", nil))
			qaAVWantAppended(t, o, "AbilityGiven act-a sober-up")
			want := qaAVBefore(t, o, "act-a")
			want.AbilityIds = append(want.AbilityIds, "sober-up")
			qaAVWantActor(t, o, want)
		})
		t.Run(string(role)+"/limited-resource-held", func(t *testing.T) {
			tb := qaAVOpen(t, rs, qaAVCast()...)
			o := tb.do(t, role, qaAVGive("act-c", "chair-swing", nil))
			qaAVWantAppended(t, o, "AbilityGiven act-c chair-swing")
			want := qaAVBefore(t, o, "act-c")
			want.AbilityIds = append(want.AbilityIds, "chair-swing")
			qaAVWantActor(t, o, want)
		})
		t.Run(string(role)+"/to-an-empty-sheet", func(t *testing.T) {
			tb := qaAVOpen(t, rs, qaAVCast()...)
			o := tb.do(t, role, qaAVGive("act-d", "fists", nil))
			qaAVWantAppended(t, o, "AbilityGiven act-d fists")
			want := qaAVBefore(t, o, "act-d")
			want.AbilityIds = []string{"fists"}
			qaAVWantActor(t, o, want)
		})
	}
}

// VTT-767 VTT-799: a take with a ruleset served.
func TestQAAVTakeRemovesTheResourceNoOtherAbilitySpends(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	cases := []struct {
		name, actor, ability string
		appended             []string
		ids                  []string
		gone                 string
	}{
		{"limited", "act-e", "chair-swing",
			[]string{"AbilityTaken act-e chair-swing", "ResourceRemoved act-e stamina"},
			[]string{"fists", "sober-up"}, "stamina"},
		{"at-will", "act-e", "fists",
			[]string{"AbilityTaken act-e fists"}, []string{"chair-swing", "sober-up"}, ""},
		{"at-will-beside-an-unspent-resource", "act-e", "sober-up",
			[]string{"AbilityTaken act-e sober-up"}, []string{"fists", "chair-swing"}, ""},
		{"limited-whose-resource-is-not-held", "act-u", "chair-swing",
			[]string{"AbilityTaken act-u chair-swing"}, []string{}, ""},
		{"repeated-id", "act-r", "chair-swing",
			[]string{"AbilityTaken act-r chair-swing", "ResourceRemoved act-r stamina"},
			[]string{"fists"}, "stamina"},
		{"id-the-ruleset-does-not-hold", "act-o", "old-move",
			[]string{"AbilityTaken act-o old-move"}, []string{"fists"}, ""},
		{"limited-from-an-unsorted-list", "act-s", "chair-swing",
			[]string{"AbilityTaken act-s chair-swing", "ResourceRemoved act-s stamina"},
			[]string{"sober-up", "fists"}, "stamina"},
	}
	for _, role := range qaAVIssuers() {
		for _, tc := range cases {
			t.Run(string(role)+"/"+tc.name, func(t *testing.T) {
				tb := qaAVOpen(t, rs, qaAVCast()...)
				o := tb.do(t, role, qaAVTake(tc.actor, tc.ability))
				qaAVWantAppended(t, o, tc.appended...)
				want := qaAVBefore(t, o, tc.actor)
				want.AbilityIds = tc.ids
				if tc.gone != "" {
					delete(want.Resources, tc.gone)
				}
				qaAVWantActor(t, o, want)
			})
		}
	}
}

// VTT-767: a resource two abilities spend stays while one is on the sheet.
func TestQAAVTakeKeepsASharedResource(t *testing.T) {
	dir := qaAVBrawlWith(t, map[string]string{
		"abilities/haymaker.json": qaAVLimited("haymaker", "stamina"),
	})
	rs := qaAVLoad(t, dir)
	if got, ok := rs.Spends("haymaker"); !ok || got != "stamina" {
		t.Fatalf("fixture: haymaker spends %q %v", got, ok)
	}
	for _, role := range qaAVIssuers() {
		t.Run(string(role), func(t *testing.T) {
			tb := qaAVOpen(t, rs, qaAVActor("act-f", []string{"chair-swing", "haymaker"},
				map[string]*vttv1.Resource{"stamina": qaAVRes(2, 2), "drink": qaAVRes(0, 4)}))
			o := tb.do(t, role, qaAVTake("act-f", "chair-swing"))
			qaAVWantAppended(t, o, "AbilityTaken act-f chair-swing")
			want := qaAVBefore(t, o, "act-f")
			want.AbilityIds = []string{"haymaker"}
			qaAVWantActor(t, o, want)

			// VTT-764: the resource is held, so the give carries none.
			o = tb.do(t, role, qaAVGive("act-f", "chair-swing", nil))
			qaAVWantAppended(t, o, "AbilityGiven act-f chair-swing")

			o = tb.do(t, role, qaAVTake("act-f", "haymaker"))
			qaAVWantAppended(t, o, "AbilityTaken act-f haymaker")

			o = tb.do(t, role, qaAVTake("act-f", "chair-swing"))
			qaAVWantAppended(t, o, "AbilityTaken act-f chair-swing", "ResourceRemoved act-f stamina")
			want = qaAVBefore(t, o, "act-f")
			want.AbilityIds = []string{}
			delete(want.Resources, "stamina")
			qaAVWantActor(t, o, want)
			if _, held := o.after.Actors["act-f"].GetResources()["drink"]; !held {
				t.Errorf("drink, which no ability spends, left the sheet")
			}
		})
	}
}

// VTT-721, VTT-717: a refused take appends nothing, its resource included.
func TestQAAVTakeRefusalsAppendNothing(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	cases := []struct {
		name, actor, ability, text string
	}{
		{"unnamed-but-resource-held", "act-c", "chair-swing", `actor "act-c" does not name ability "chair-swing"`},
		{"unnamed", "act-a", "chair-swing", `actor "act-a" does not name ability "chair-swing"`},
		{"unknown-actor", "act-ghost", "chair-swing", `ability_taken names unknown actor "act-ghost"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb := qaAVOpen(t, rs, qaAVCast()...)
			o := tb.do(t, identity.RoleAgent, qaAVTake(tc.actor, tc.ability))
			qaAVWantRefused(t, o)
			qaAVContains(t, o, tc.text)
		})
	}
}

// VTT-765, VTT-766, VTT-768, VTT-720, VTT-722: no ruleset served.
func TestQAAVWithNoRuleset(t *testing.T) {
	refusals := []struct {
		name  string
		cmd   *vttv1.ClientCommand
		check func(t *testing.T, o qaAVOutcome)
	}{
		// VTT-765
		{"limited-with-its-value", qaAVGive("act-a", "chair-swing",
			map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2)}),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a")
			}},
		// VTT-765: a resource at 0 with no max is still carried
		{"any-id-with-a-zero-value", qaAVGive("act-a", "anything-goes",
			map[string]*vttv1.Resource{"x": qaAVRes(0, 0)}),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a")
			}},
		// VTT-765 before the fold's VTT-722 (D3's order)
		{"empty-id-with-a-value", qaAVGive("act-a", "",
			map[string]*vttv1.Resource{"x": qaAVRes(1, 1)}),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVShape(t, o, "act-a")
			}},
		// VTT-722
		{"empty-id", qaAVGive("act-a", "", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, "ability_given requires an ability id")
			}},
		// VTT-720
		{"already-named", qaAVGive("act-a", "fists", nil),
			func(t *testing.T, o qaAVOutcome) {
				t.Helper()
				qaAVContains(t, o, `actor "act-a" already names ability "fists"`)
			}},
	}
	for _, tc := range refusals {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			tb := qaAVOpen(t, nil, qaAVCast()...)
			o := tb.do(t, identity.RoleDM, tc.cmd)
			qaAVWantRefused(t, o)
			tc.check(t, o)
		})
	}
	for _, role := range qaAVIssuers() {
		// VTT-766
		for _, id := range []string{"anything-goes", "chair-swing"} {
			t.Run(string(role)+"/give-"+id, func(t *testing.T) {
				tb := qaAVOpen(t, nil, qaAVCast()...)
				o := tb.do(t, role, qaAVGive("act-a", id, nil))
				qaAVWantAppended(t, o, "AbilityGiven act-a "+id)
				want := qaAVBefore(t, o, "act-a")
				want.AbilityIds = append(want.AbilityIds, id)
				qaAVWantActor(t, o, want)
			})
		}
		// VTT-768
		takes := []struct {
			actor string
			ids   []string
		}{
			{"act-e", []string{"fists", "sober-up"}}, {"act-r", []string{"fists"}},
			{"act-s", []string{"sober-up", "fists"}},
		}
		for _, tk := range takes {
			t.Run(string(role)+"/take-from-"+tk.actor, func(t *testing.T) {
				tb := qaAVOpen(t, nil, qaAVCast()...)
				o := tb.do(t, role, qaAVTake(tk.actor, "chair-swing"))
				qaAVWantAppended(t, o, "AbilityTaken "+tk.actor+" chair-swing")
				want := qaAVBefore(t, o, tk.actor)
				want.AbilityIds = tk.ids
				qaAVWantActor(t, o, want)
			})
		}
	}
}

// VTT-778: with no ruleset served, add_actor reads no ability against the
// resources it holds.
func TestQAAVNoRulesetAddActorReadsNoSpending(t *testing.T) {
	sheets := []*vttv1.Actor{
		qaAVActor("act-n1", []string{"chair-swing"}, nil),
		qaAVActor("act-n2", nil, map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2)}),
	}
	for _, a := range sheets {
		t.Run(a.GetActorId(), func(t *testing.T) {
			tb := qaAVOpen(t, nil, qaAVCast()...)
			o := tb.do(t, identity.RoleDM, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{
				AddActor: &vttv1.AddActor{Actor: a},
			}})
			qaAVWantAppended(t, o, "*vttv1.Envelope_ActorAdded")
			if !proto.Equal(o.appended[0].GetActorAdded().GetActor(), a) {
				t.Errorf("added %v, want %v", o.appended[0].GetActorAdded().GetActor(), a)
			}
		})
	}
}

// VTT-716: a player, even one controlling the actor, and a spectator.
func TestQAAVOnlyTheDMAndTheAgentGiveAndTake(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	cmds := []struct {
		name string
		cmd  func() *vttv1.ClientCommand
	}{
		{"give", func() *vttv1.ClientCommand { return qaAVGive("act-a", "sober-up", nil) }},
		{"give-with-value", func() *vttv1.ClientCommand {
			return qaAVGive("act-a", "chair-swing", map[string]*vttv1.Resource{"stamina": qaAVRes(1, 2)})
		}},
		{"take", func() *vttv1.ClientCommand { return qaAVTake("act-a", "fists") }},
	}
	for _, served := range []*rules.Ruleset{rs, nil} {
		for _, role := range []identity.Role{identity.RolePlayer, identity.RoleSpectator} {
			for _, c := range cmds {
				t.Run(fmt.Sprintf("ruleset-%v/%s/%s", served != nil, role, c.name), func(t *testing.T) {
					tb := qaAVOpen(t, served, qaAVCast()...)
					ctl := tb.c.State().Actors["act-a"].GetControllerIds()
					if !reflect.DeepEqual(ctl, []string{tb.pids[identity.RolePlayer]}) {
						t.Fatalf("fixture: act-a controlled by %v", ctl)
					}
					o := tb.do(t, role, c.cmd())
					qaAVWantRefused(t, o)
					qaAVContains(t, o, "not authorized")
				})
			}
		}
	}
}

// VTT-770: a resource name at the 128-byte bound is added.
func TestQAAVGiveAtTheResourceNameBound(t *testing.T) {
	edge := strings.Repeat("s", 128)
	dir := qaAVBrawlWith(t, map[string]string{
		"abilities/edge-swing.json": qaAVLimited("edge-swing", edge),
	}, map[string]any{
		"key": edge, "type": "resource", "name": "QA edge",
		"description": "QA", "default_max_expr": "2", "thresholds": []any{},
	})
	rs := qaAVLoad(t, dir)
	tb := qaAVOpen(t, rs, qaAVCast()...)
	o := tb.do(t, identity.RoleDM, qaAVGive("act-a", "edge-swing",
		map[string]*vttv1.Resource{edge: qaAVRes(1, 2)}))
	qaAVWantAppended(t, o, "AbilityGiven act-a edge-swing", "ResourceAdded act-a "+edge+" 1/2")
}

// A resource written null is a malformed frame, and nothing is appended.
func TestQAAVGiveWithANullValueAppendsNothing(t *testing.T) {
	rs := qaAVLoad(t, qaAVBrawl)
	tb := qaAVOpen(t, rs, qaAVCast()...)
	before := tb.log(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	conn := tb.dial(ctx, t, identity.RoleDM)
	defer conn.Close(websocket.StatusNormalClosure, "")
	raw := `{"requestId":"req-qa-av-null","giveAbility":{"actorId":"act-a",` +
		`"abilityId":"chair-swing","resources":{"stamina":null}}}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}
	var outcome string
	for outcome == "" {
		_, data, err := conn.Read(ctx)
		if err != nil {
			outcome = "closed: " + err.Error()
			break
		}
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(data, &f); err != nil {
			t.Fatalf("decode frame: %v", err)
		}
		if r := f.GetResult(); r != nil && r.GetRequestId() == "req-qa-av-null" {
			outcome = fmt.Sprintf("result ok=%v %q", r.GetOk(), r.GetError())
		}
	}
	t.Logf("a null resource was answered: %s", outcome)
	after := tb.log(t)
	if len(after) != len(before) {
		t.Fatalf("appended %d events for a resource written null", len(after)-len(before))
	}
	if strings.HasPrefix(outcome, "result ok=true") {
		t.Fatalf("accepted with nothing appended: %s", outcome)
	}
}
