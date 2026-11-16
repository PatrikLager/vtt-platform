package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

const qaSFTavernDir = "../../rulesets/tavern-brawl"

func qaSFCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o600)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func qaSFRuleset(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load ruleset %s: %v", dir, err)
	}
	return rs
}

func qaSFJSONFile(t *testing.T, path string, edit func(map[string]any)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// qaSFVariant is tavern-brawl with a resource "anger" declared after stamina,
// "rage-punch" spending anger and "bottle-smash" spending stamina beside
// chair-swing. drink is still spent by no ability.
func qaSFVariant(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	qaSFCopyDir(t, qaSFTavernDir, dir)
	qaSFJSONFile(t, filepath.Join(dir, "ruleset.json"), func(doc map[string]any) {
		props, _ := doc["properties"].([]any)
		doc["properties"] = append(props, map[string]any{
			"key": "anger", "type": "resource", "name": "Anger",
			"description": "QA fixture resource spent by rage-punch.", "thresholds": []any{},
		})
	})
	swing, err := os.ReadFile(filepath.Join(dir, "abilities", "chair-swing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []struct{ id, resource string }{{"rage-punch", "anger"}, {"bottle-smash", "stamina"}} {
		path := filepath.Join(dir, "abilities", a.id+".json")
		if err := os.WriteFile(path, swing, 0o600); err != nil {
			t.Fatal(err)
		}
		qaSFJSONFile(t, path, func(ab map[string]any) {
			ab["id"] = a.id
			ab["name"] = a.id
			ab["usage"] = map[string]any{"limited": map[string]any{"resource": a.resource, "cost": 1}}
		})
	}
	return qaSFRuleset(t, dir)
}

type qaSFServer struct {
	c     *campaign.Campaign
	conn  *websocket.Conn
	ctx   context.Context
	reqNo int
}

// qaSFServe builds a campaign holding one SessionStarted and then extra,
// appended straight to the log, a DM invite, the gateway (with rs when
// non-nil) over httptest, and a DM WebSocket.
func qaSFServe(t *testing.T, rs *rules.Ruleset, extra ...*vttv1.Envelope) *qaSFServer {
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
	token, _, err := ids.CreateInvite("qa-dm", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	envs := append([]*vttv1.Envelope{{
		Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "qa"}},
	}}, extra...)
	for i, e := range envs {
		e.EventId = "qa-sf-event-" + strconv.Itoa(i)
		e.SessionId = "qa-sf-session"
		e.OccurredAt = timestamppb.Now()
	}
	head, err := c.AppendBatch(envs)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	url := "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws?token=" + token + "&after=" + strconv.FormatInt(head, 10)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		hs.Close()
		cancel()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qaSFServer{c: c, conn: conn, ctx: ctx}
}

// head answers the log's last sequence.
func (s *qaSFServer) head(t *testing.T) int64 {
	t.Helper()
	_, unsub, h, err := s.c.Subscribe(0, 4096)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	unsub()
	return h
}

// addActor sends one add_actor and answers its CommandResult.
func (s *qaSFServer) addActor(t *testing.T, a *vttv1.Actor) *vttv1.CommandResult {
	t.Helper()
	return s.send(t, &vttv1.ClientCommand{
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}},
	})
}

// send sends cmd under a fresh request id and answers its CommandResult.
func (s *qaSFServer) send(t *testing.T, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t.Helper()
	s.reqNo++
	reqID := fmt.Sprintf("qa-sf-%d", s.reqNo)
	cmd.RequestId = reqID
	b, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.conn.Write(s.ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("write: %v", err)
	}
	for {
		_, raw, err := s.conn.Read(s.ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &f); err != nil {
			t.Fatalf("frame %s: %v", raw, err)
		}
		if r := f.GetResult(); r != nil && r.GetRequestId() == reqID {
			return r
		}
	}
}

func qaSFActor(id string, abilities []string, resources map[string]*vttv1.Resource) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       id,
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		Attributes: map[string]int32{"brawn": 2},
		Resources:  resources,
		AbilityIds: abilities,
	}
}

func qaSFKeys(m map[string]*vttv1.Resource) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// wantSpendRefusal sends a and asserts the shape `gateway: add_actor:
// actor "<id>": <field>: <reason>`, with the fault Ruleset.SheetSpendFault
// answers, and that nothing was appended (VTT-775).
func (s *qaSFServer) wantSpendRefusal(t *testing.T, rs *rules.Ruleset, a *vttv1.Actor, field, key string) {
	t.Helper()
	f := rs.SheetSpendFault(a.GetAbilityIds(), qaSFKeys(a.GetResources()))
	if f == nil || f.Field != field || f.Key != key {
		t.Fatalf("fixture fault %+v, want field %q key %q", f, field, key)
	}
	before := s.head(t)
	r := s.addActor(t, a)
	if r.GetOk() {
		t.Fatalf("add_actor %q accepted; want %s %q refused", a.GetActorId(), field, key)
	}
	want := fmt.Sprintf("gateway: add_actor: actor %q: %s: %s", a.GetActorId(), field, f.Reason)
	if r.GetError() != want {
		t.Fatalf("error\n got: %s\nwant: %s", r.GetError(), want)
	}
	if after := s.head(t); after != before {
		t.Fatalf("log head moved %d -> %d on a refused add_actor", before, after)
	}
	if _, ok := s.c.Actors()[a.GetActorId()]; ok {
		t.Fatalf("refused actor %q is in the state", a.GetActorId())
	}
}

func (s *qaSFServer) wantAccepted(t *testing.T, a *vttv1.Actor) {
	t.Helper()
	before := s.head(t)
	r := s.addActor(t, a)
	if !r.GetOk() {
		t.Fatalf("add_actor %q refused: %s", a.GetActorId(), r.GetError())
	}
	if after := s.head(t); after != before+1 {
		t.Fatalf("log head %d -> %d, want one ActorAdded", before, after)
	}
	if _, ok := s.c.Actors()[a.GetActorId()]; !ok {
		t.Fatalf("accepted actor %q is not in the state", a.GetActorId())
	}
}

func qaSFR(current, maximum int32) *vttv1.Resource {
	return &vttv1.Resource{Current: current, Max: maximum}
}

// TestQASFAddActorRefusesEachHalfWithARulesetServed pins VTT-775's two
// halves on the wire, the error shape, and "appends nothing".
func TestQASFAddActorRefusesEachHalfWithARulesetServed(t *testing.T) {
	rs := qaSFRuleset(t, qaSFTavernDir)
	s := qaSFServe(t, rs)
	t.Run("a limited ability without its resource", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-no-stamina", []string{"fists", "chair-swing"},
			map[string]*vttv1.Resource{"drink": qaSFR(0, 5)}), "ability_ids", "chair-swing")
	})
	t.Run("a limited ability and no resources at all", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-bare", []string{"chair-swing"}, nil), "ability_ids", "chair-swing")
	})
	t.Run("a spent resource no named ability spends", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-orphan", []string{"fists", "sober-up"},
			map[string]*vttv1.Resource{"drink": qaSFR(0, 5), "stamina": qaSFR(2, 2)}), "resources", "stamina")
	})
	t.Run("a spent resource and no ability named", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-orphan-bare", nil,
			map[string]*vttv1.Resource{"stamina": qaSFR(2, 2)}), "resources", "stamina")
	})
}

// TestQASFAddActorAcceptsSheetsThatFit is the control for the refusals
// above and pins the rule's reach: drink, spent by no ability, needs none;
// a limited ability with its resource at 0 holds the resource (VTT-775).
func TestQASFAddActorAcceptsSheetsThatFit(t *testing.T) {
	rs := qaSFRuleset(t, qaSFTavernDir)
	s := qaSFServe(t, rs)
	s.wantAccepted(t, qaSFActor("qa-fits", []string{"chair-swing", "fists"},
		map[string]*vttv1.Resource{"drink": qaSFR(0, 5), "stamina": qaSFR(2, 2)}))
	s.wantAccepted(t, qaSFActor("qa-stamina-zero", []string{"chair-swing"},
		map[string]*vttv1.Resource{"stamina": qaSFR(0, 2)}))
	s.wantAccepted(t, qaSFActor("qa-drink-only", nil, map[string]*vttv1.Resource{"drink": qaSFR(1, 5)}))
	s.wantAccepted(t, qaSFActor("qa-drink-at-will", []string{"fists", "splash-of-water"},
		map[string]*vttv1.Resource{"drink": qaSFR(1, 5)}))
	s.wantAccepted(t, qaSFActor("qa-at-will-bare", []string{"fists"}, nil))
}

// TestQASFAddActorOrderAndSharedResource pins the order on the wire
// (VTT-775: ability ids before resources, each in byte order) and a shared
// resource fitting while any spender is named.
func TestQASFAddActorOrderAndSharedResource(t *testing.T) {
	rs := qaSFVariant(t)
	s := qaSFServe(t, rs)
	t.Run("ability ids in byte order", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-two-limited", []string{"rage-punch", "chair-swing"}, nil), "ability_ids", "chair-swing")
	})
	t.Run("first id whose resource is missing", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-one-covered", []string{"rage-punch", "bottle-smash"},
			map[string]*vttv1.Resource{"stamina": qaSFR(1, 2)}), "ability_ids", "rage-punch")
	})
	t.Run("resources in byte order", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-two-orphans", []string{"fists"},
			map[string]*vttv1.Resource{"stamina": qaSFR(1, 2), "anger": qaSFR(1, 1)}), "resources", "anger")
	})
	t.Run("abilities before resources", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-both-halves", []string{"chair-swing"},
			map[string]*vttv1.Resource{"anger": qaSFR(1, 1)}), "ability_ids", "chair-swing")
	})
	t.Run("shared resource with no spender named", func(t *testing.T) {
		s.wantSpendRefusal(t, rs, qaSFActor("qa-shared-orphan", []string{"fists"},
			map[string]*vttv1.Resource{"stamina": qaSFR(1, 2)}), "resources", "stamina")
	})
	t.Run("shared resource with the other spender named", func(t *testing.T) {
		s.wantAccepted(t, qaSFActor("qa-shared", []string{"bottle-smash"}, map[string]*vttv1.Resource{"stamina": qaSFR(1, 2)}))
	})
}

// TestQASFAddActorEarlierRefusalsComeFirst pins VTT-775's "after its
// ability-id refusals", and through them VTT-750 and VTT-677's "after its
// controller and kind refusals": an actor that also breaks this rule is
// refused for the earlier fault, appending nothing.
func TestQASFAddActorEarlierRefusalsComeFirst(t *testing.T) {
	rs := qaSFRuleset(t, qaSFTavernDir)
	s := qaSFServe(t, rs)
	spendReason := rs.SheetSpendFault([]string{"chair-swing"}, nil).Reason

	withController := qaSFActor("qa-ctl", []string{"chair-swing"}, nil)
	withController.ControllerIds = []string{"someone"}
	noKind := qaSFActor("qa-kindless", []string{"chair-swing"}, nil)
	noKind.Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	badKey := qaSFActor("qa-badkey", []string{"chair-swing"}, map[string]*vttv1.Resource{"zest": qaSFR(1, 1)})
	numberAsResource := qaSFActor("qa-wrongmap", []string{"chair-swing"}, map[string]*vttv1.Resource{"brawn": qaSFR(1, 1)})

	cases := []struct {
		name string
		a    *vttv1.Actor
		want string // a substring; empty means only "not the spend refusal"
	}{
		{"a controller (refused before the sheet is read)", withController, "gateway: add_actor: controller"},
		{"no kind (refused before the sheet is read)", noKind, "gateway: add_actor: kind"},
		{"an undeclared key (VTT-677)", badKey, `gateway: add_actor: actor "qa-badkey": resources: references undeclared property "zest"`},
		{"a key under the other type's map (VTT-677)", numberAsResource, `gateway: add_actor: actor "qa-wrongmap": resources: `},
		{"an id the ruleset does not hold (VTT-750)", qaSFActor("qa-unheld", []string{"chair-swing", "no-such-ability"}, nil),
			`gateway: add_actor: actor "qa-unheld": ability_ids: names ability "no-such-ability", which the ruleset does not hold`},
		{"an empty id (VTT-747)", qaSFActor("qa-emptyid", []string{"chair-swing", ""}, nil),
			`gateway: add_actor: actor "qa-emptyid": ability_ids: must not hold an empty id`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := s.head(t)
			r := s.addActor(t, c.a)
			if r.GetOk() {
				t.Fatal("accepted")
			}
			t.Logf("refused: %s", r.GetError())
			if strings.Contains(r.GetError(), spendReason) {
				t.Fatalf("error %q is the spend refusal, not the earlier one", r.GetError())
			}
			if c.want != "" && !strings.Contains(r.GetError(), c.want) {
				t.Fatalf("error %q does not contain %q", r.GetError(), c.want)
			}
			if after := s.head(t); after != before {
				t.Fatalf("log head moved %d -> %d", before, after)
			}
		})
	}
}

// TestQASFAddActorWithNoRulesetRefusesNeitherHalf pins VTT-778's add_actor
// half: with no ruleset served, add_actor is refused for nothing this rule
// adds.
func TestQASFAddActorWithNoRulesetRefusesNeitherHalf(t *testing.T) {
	s := qaSFServe(t, nil)
	s.wantAccepted(t, qaSFActor("qa-nr-no-stamina", []string{"chair-swing"}, map[string]*vttv1.Resource{"drink": qaSFR(0, 5)}))
	s.wantAccepted(t, qaSFActor("qa-nr-orphan", []string{"fists"}, map[string]*vttv1.Resource{"stamina": qaSFR(2, 2)}))
	s.wantAccepted(t, qaSFActor("qa-nr-orphan-bare", nil, map[string]*vttv1.Resource{"stamina": qaSFR(2, 2)}))
}

// TestQASFUseOverALogFoldedBeforeTheRuleIsStillRefused (VTT-804):
// rules.Resolve's usage refusal stays, so an actor appended straight to the
// log, past every entry point, naming a limited ability without its
// resource is still refused the use ("has no resource ... required by
// ability ...'s usage cost"), appending nothing.
func TestQASFUseOverALogFoldedBeforeTheRuleIsStillRefused(t *testing.T) {
	rs := qaSFRuleset(t, qaSFTavernDir)
	actor := func(id string, abilities []string, res map[string]*vttv1.Resource) *vttv1.Envelope {
		a := qaSFActor(id, abilities, res)
		a.Attributes = map[string]int32{"brawn": 2, "grit": 1, "footing": 10}
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
	}
	token := func(tok, actorID string, x int32) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tok, SceneId: "qa-room", ActorId: actorID, Position: &vttv1.GridPosition{X: x, Y: 0},
		}}}
	}
	s := qaSFServe(t, rs,
		&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "qa-room", Name: "qa-room", GridWidth: 4, GridHeight: 4,
		}}},
		actor("qa-pre", []string{"chair-swing"}, map[string]*vttv1.Resource{"drink": qaSFR(0, 4)}),
		actor("qa-target", []string{"fists"}, map[string]*vttv1.Resource{"drink": qaSFR(0, 4)}),
		token("tok-pre", "qa-pre", 0),
		token("tok-target", "qa-target", 1),
	)
	before := s.head(t)
	r := s.send(t, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: "qa-pre", AbilityId: "chair-swing", TargetIds: []string{"qa-target"},
	}}})
	if r.GetOk() {
		t.Fatal("use_ability of chair-swing by an actor holding no stamina was accepted")
	}
	t.Logf("refused: %s", r.GetError())
	if want := `rules: resolve: actor "qa-pre" has no resource "stamina" required by ability "chair-swing"'s usage cost`; r.GetError() != want {
		t.Fatalf("error %q is not the usage refusal %q", r.GetError(), want)
	}
	if after := s.head(t); after != before {
		t.Fatalf("log head moved %d -> %d", before, after)
	}
}
