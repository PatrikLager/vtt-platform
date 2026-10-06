package gateway_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaRuleBound  = 128
	qaRulePool   = "qa_pool"
	qaRuleMarker = "48611"
	qaRuleHero   = "qa-hero"
	qaRuleMark   = "qa-mark"
)

type qaRuleCase struct {
	label string
	id    string
}

func qaRuleOver() []qaRuleCase {
	return []qaRuleCase{
		{"129 ascii", strings.Repeat("a", 129)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
		{"empty", ""},
	}
}

func qaRuleEdge() []qaRuleCase {
	return []qaRuleCase{
		{"128 ascii", "S" + strings.Repeat("a", 126) + "E"},
		{"42 three-byte runes and two ascii", strings.Repeat("€", 42) + "ab"},
	}
}

func qaRuleWriteJSON(t *testing.T, p string, doc any) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal %s: %v", p, err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func qaRuleAbilityDoc(id string, usage any, atoms ...string) map[string]any {
	compose := make([]map[string]any, 0, len(atoms))
	for _, a := range atoms {
		compose = append(compose, map[string]any{"atom": a, "bind": map[string]any{}})
	}
	return map[string]any{"id": id, "name": "QA", "usage": usage, "compose": compose}
}

func qaRuleAlways(effect map[string]any) []map[string]any {
	return []map[string]any{{"kind": "outcome", "key": nil, "branch": "always", "effects": []map[string]any{effect}}}
}

func qaRuleRulesetOnDisk(t *testing.T, res string, thresholds []map[string]any) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	qaRuleWriteJSON(t, filepath.Join(dir, "ruleset.json"), map[string]any{
		"id": "qa-rules", "name": "QA Rules", "format_version": "2",
		"attributes": []string{"qa_attr"}, "defenses": []string{},
		"resources": []map[string]any{{"name": res, "thresholds": thresholds}},
	})
	qaRuleWriteJSON(t, filepath.Join(dir, "conditions", "qa-mark.json"), map[string]any{"id": qaRuleMark, "name": "QA Mark"})
	atoms := map[string][]map[string]any{
		"qa-self":       {{"kind": "targeting", "range": 0, "max_targets": 1}},
		"qa-apply-mark": qaRuleAlways(map[string]any{"apply_condition": map[string]any{"id": qaRuleMark}}),
		"qa-drain":      qaRuleAlways(map[string]any{"resource_change": map[string]any{"resource": res, "delta_expr": "0 - 1"}}),
	}
	for id, contributes := range atoms {
		qaRuleWriteJSON(t, filepath.Join(dir, "atoms", id+".json"), map[string]any{
			"id": id, "params": []any{}, "provides": []string{}, "consumes": []string{}, "contributes": contributes,
		})
	}
	limited := map[string]any{"limited": map[string]any{"resource": res, "cost": 1}}
	qaRuleWriteJSON(t, filepath.Join(dir, "abilities", "qa-mark.json"), qaRuleAbilityDoc("qa-mark-ability", "at_will", "qa-self", "qa-apply-mark"))
	qaRuleWriteJSON(t, filepath.Join(dir, "abilities", "qa-drain.json"), qaRuleAbilityDoc("qa-drain", "at_will", "qa-self", "qa-drain"))
	qaRuleWriteJSON(t, filepath.Join(dir, "abilities", "qa-spend.json"), qaRuleAbilityDoc("qa-spend", limited, "qa-self", "qa-apply-mark"))
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# QA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaRuleHarmless() map[string]any {
	return map[string]any{"when": "#" + qaRulePool + " - #" + qaRulePool, "apply_condition": qaRuleMark, "remove_when_false": false}
}

type qaRuleTable struct {
	srv      *httptest.Server
	camp     *campaign.Campaign
	rs       *rules.Ruleset
	dmToken  string
	obsToken string
}

func qaRuleServe(t *testing.T, rs *rules.Ruleset) *qaRuleTable {
	t.Helper()
	mapsDir := filepath.Join(t.TempDir(), "maps")
	qaRuleWriteJSON(t, filepath.Join(mapsDir, "qa-board.json"), map[string]any{
		"format_version": 1, "id": "qa-board", "name": "qa", "grid_width": 3, "grid_height": 3,
	})
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = c.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	srv := gateway.New(c, ids).WithMapsDir(mapsDir)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hs.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	dmToken, _, err := ids.CreateInvite("qa dm", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	obsToken, _, err := ids.CreateInvite("qa observer", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	return &qaRuleTable{srv: hs, camp: c, rs: rs, dmToken: dmToken, obsToken: obsToken}
}

type qaRuleConn struct {
	t       *testing.T
	ws      *websocket.Conn
	head    int64
	events  []*vttv1.Envelope
	results map[string]*vttv1.CommandResult
}

func qaRuleDial(t *testing.T, tb *qaRuleTable, token string) *qaRuleConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" + token + "&after=0"
	ws, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ws.SetReadLimit(1 << 22)
	qc := &qaRuleConn{t: t, ws: ws, results: map[string]*vttv1.CommandResult{}}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	first := qc.read()
	if first.GetCatchUpHead() == nil {
		t.Fatalf("first frame is not catch_up_head: %v", first)
	}
	qc.head = first.GetCatchUpHead().GetHeadSequence()
	return qc
}

func (qc *qaRuleConn) read() *vttv1.ServerFrame {
	qc.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, data, err := qc.ws.Read(ctx)
	if err != nil {
		qc.t.Fatalf("read: %v", err)
	}
	f := &vttv1.ServerFrame{}
	if err := protojson.Unmarshal(data, f); err != nil {
		qc.t.Fatalf("decode frame %s: %v", data, err)
	}
	if ev := f.GetEvent(); ev != nil {
		qc.events = append(qc.events, ev)
	}
	if r := f.GetResult(); r != nil {
		qc.results[r.GetRequestId()] = r
	}
	return f
}

func (qc *qaRuleConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	qc.t.Helper()
	data, err := protojson.Marshal(cmd)
	if err != nil {
		qc.t.Fatalf("encode: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := qc.ws.Write(ctx, websocket.MessageText, data); err != nil {
		qc.t.Fatalf("write: %v", err)
	}
	for {
		if r, ok := qc.results[cmd.GetRequestId()]; ok {
			return r
		}
		qc.read()
	}
}

func (qc *qaRuleConn) event(seq int64) *vttv1.Envelope {
	qc.t.Helper()
	for {
		for _, e := range qc.events {
			if e.GetSequence() == seq {
				return e
			}
		}
		qc.read()
	}
}

func qaRuleAdd(req string, actor *vttv1.Actor) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: actor}}}
}

func qaRuleUse(req, abilityID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: qaRuleHero, AbilityId: abilityID, TargetIds: []string{qaRuleHero},
	}}}
}

func qaRuleOK(t *testing.T, qc *qaRuleConn, cmd *vttv1.ClientCommand, wantSeq int64) *vttv1.CommandResult {
	t.Helper()
	res := qc.send(cmd)
	if !res.GetOk() {
		t.Fatalf("%s refused: %q", cmd.GetRequestId(), res.GetError())
	}
	if wantSeq != 0 && res.GetSequence() != wantSeq {
		t.Fatalf("%s landed at %d, want %d", cmd.GetRequestId(), res.GetSequence(), wantSeq)
	}
	return res
}

func qaRuleBoard(t *testing.T, dm *qaRuleConn, resources map[string]*vttv1.Resource) {
	t.Helper()
	res := qaRuleOK(t, dm, &vttv1.ClientCommand{RequestId: "board", Command: &vttv1.ClientCommand_LoadMap{
		LoadMap: &vttv1.LoadMap{MapId: "qa-board"},
	}}, 1)
	scene := dm.event(res.GetSequence()).GetSceneCreated()
	if scene == nil {
		t.Fatalf("load_map's first event is not a SceneCreated")
	}
	qaRuleOK(t, dm, qaRuleAdd("hero", &vttv1.Actor{
		ActorId: qaRuleHero, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, Resources: resources,
	}), 2)
	qaRuleOK(t, dm, &vttv1.ClientCommand{RequestId: "place", Command: &vttv1.ClientCommand_PlaceToken{
		PlaceToken: &vttv1.PlaceToken{
			TokenId: "qa-tok", SceneId: scene.GetSceneId(), ActorId: qaRuleHero, Position: &vttv1.GridPosition{X: 1, Y: 1},
		},
	}}, 3)
}

func qaRuleStillOpenNothingAppended(t *testing.T, tb *qaRuleTable, issuer, observer *qaRuleConn, h0 int64) {
	t.Helper()
	follow := qaRuleAdd("follow", &vttv1.Actor{ActorId: "qa-follow", Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	next := issuer.send(follow)
	if !next.GetOk() {
		t.Fatalf("the connection did not take the next command: %q", next.GetError())
	}
	if next.GetSequence() != h0+1 {
		t.Fatalf("next command landed at %d, want %d: the refusal appended", next.GetSequence(), h0+1)
	}
	if observer.event(h0+1).GetEventId() != issuer.event(h0+1).GetEventId() {
		t.Fatalf("observer's event %d is not the follow-up", h0+1)
	}
	for i, e := range observer.events {
		if e.GetSequence() != int64(i+1) {
			t.Fatalf("observer received sequence %d at position %d", e.GetSequence(), i)
		}
	}
	if fresh := qaRuleDial(t, tb, tb.obsToken); fresh.head != h0+1 {
		t.Fatalf("log head is %d, want %d", fresh.head, h0+1)
	}
}

type qaRuleRoller struct{}

func (qaRuleRoller) Roll(n, sides int) ([]int, int) {
	out := make([]int, n)
	for i := range out {
		out[i] = sides
	}
	return out, n * sides
}

func qaRuleLocalRefusal(t *testing.T, tb *qaRuleTable, cmd *vttv1.ClientCommand) string {
	t.Helper()
	st := tb.camp.State()
	envs, err := rules.Resolve(tb.rs, st, cmd.GetUseAbility(), qaRuleRoller{})
	if err != nil {
		return err.Error()
	}
	for i, env := range envs {
		env.Sequence = 100 + int64(i)
		if err := engine.Apply(st, env); err != nil {
			return err.Error()
		}
	}
	t.Fatalf("the local resolve and fold accepted the use the test expects the server to refuse")
	return ""
}

func qaRuleElide(msg, id string) string {
	if id == "" {
		return msg
	}
	return strings.ReplaceAll(msg, id, "<id>")
}

func qaRuleWithAbilityID(rs *rules.Ruleset, id string) {
	p := *rs.Compiled["qa-mark-ability"]
	p.ID = id
	rs.Compiled[id] = &p
}

func qaRuleWithConditionID(t *testing.T, rs *rules.Ruleset, id string) {
	t.Helper()
	p := rs.Compiled["qa-mark-ability"]
	for i := range p.Effects {
		if p.Effects[i].Kind == rules.OutcomeApplyCondition {
			p.Effects[i].ApplyCondition = &rules.ApplyConditionOutcome{ID: id}
			rs.Conditions[id] = &rules.Condition{ID: id, Name: "QA Held"}
			return
		}
	}
	t.Fatalf("the loaded qa-mark-ability applies no condition")
}

// VTT-161 VTT-162 VTT-285 VTT-286
func TestQARuleAUseWhoseAbilityIDCrossesTheBoundIsRefusedWithTheFoldsText(t *testing.T) {
	for _, c := range qaRuleOver() {
		t.Run(c.label, func(t *testing.T) {
			rs := qaRuleRulesetOnDisk(t, qaRulePool, []map[string]any{})
			qaRuleWithAbilityID(rs, c.id)
			tb := qaRuleServe(t, rs)
			issuer := qaRuleDial(t, tb, tb.dmToken)
			observer := qaRuleDial(t, tb, tb.obsToken)
			qaRuleBoard(t, issuer, nil)
			cmd := qaRuleUse("use", c.id)
			want := qaRuleLocalRefusal(t, tb, cmd)
			res := issuer.send(cmd)
			t.Logf("refusal: %s", qaRuleElide(res.GetError(), c.id))
			if res.GetOk() {
				t.Fatalf("use_ability of a %d-byte ability id accepted at %d", len(c.id), res.GetSequence())
			}
			if !strings.Contains(res.GetError(), want) {
				t.Fatalf("refusal %q does not carry %q", qaRuleElide(res.GetError(), c.id), qaRuleElide(want, c.id))
			}
			qaRuleStillOpenNothingAppended(t, tb, issuer, observer, 3)
		})
	}
}

// VTT-161 VTT-162 VTT-285 VTT-286
func TestQARuleAUseWhoseConditionIDCrossesTheBoundIsRefusedWithTheFoldsText(t *testing.T) {
	for _, c := range qaRuleOver() {
		t.Run(c.label, func(t *testing.T) {
			rs := qaRuleRulesetOnDisk(t, qaRulePool, []map[string]any{})
			qaRuleWithConditionID(t, rs, c.id)
			tb := qaRuleServe(t, rs)
			issuer := qaRuleDial(t, tb, tb.dmToken)
			observer := qaRuleDial(t, tb, tb.obsToken)
			qaRuleBoard(t, issuer, nil)
			cmd := qaRuleUse("use", "qa-mark-ability")
			want := qaRuleLocalRefusal(t, tb, cmd)
			res := issuer.send(cmd)
			t.Logf("refusal: %s", qaRuleElide(res.GetError(), c.id))
			if res.GetOk() {
				t.Fatalf("use_ability applying a %d-byte condition id accepted at %d", len(c.id), res.GetSequence())
			}
			if !strings.Contains(res.GetError(), want) {
				t.Fatalf("refusal %q does not carry %q", qaRuleElide(res.GetError(), c.id), qaRuleElide(want, c.id))
			}
			qaRuleStillOpenNothingAppended(t, tb, issuer, observer, 3)
		})
	}
}

// VTT-285
func TestQARuleAUseWhoseIDsAreAtTheBoundIsRecordedWhole(t *testing.T) {
	for _, c := range qaRuleEdge() {
		t.Run(c.label, func(t *testing.T) {
			rs := qaRuleRulesetOnDisk(t, qaRulePool, []map[string]any{})
			qaRuleWithConditionID(t, rs, c.id)
			qaRuleWithAbilityID(rs, c.id)
			tb := qaRuleServe(t, rs)
			dm := qaRuleDial(t, tb, tb.dmToken)
			qaRuleBoard(t, dm, nil)
			res := qaRuleOK(t, dm, qaRuleUse("use", c.id), 4)
			if got := dm.event(res.GetSequence()).GetAbilityUsed().GetAbilityId(); got != c.id {
				t.Fatalf("recorded ability id is %d bytes, sent %d", len(got), len(c.id))
			}
			if got := dm.event(res.GetSequence() + 1).GetConditionApplied().GetConditionId(); got != c.id {
				t.Fatalf("recorded condition id is %d bytes, want %d", len(got), len(c.id))
			}
		})
	}
}

// VTT-132 VTT-162 VTT-288
func TestQARuleAThresholdRefusalOnTheWireNamesTheResourceNotTheExpression(t *testing.T) {
	failing := "@qa_attr" + strings.Repeat(" + 0", 40) + " + " + qaRuleMarker
	thresholds := []map[string]any{qaRuleHarmless(), qaRuleHarmless(), qaRuleHarmless()}
	thresholds[2]["when"] = failing
	rs := qaRuleRulesetOnDisk(t, qaRulePool, thresholds)
	tb := qaRuleServe(t, rs)
	issuer := qaRuleDial(t, tb, tb.dmToken)
	observer := qaRuleDial(t, tb, tb.obsToken)
	qaRuleBoard(t, issuer, map[string]*vttv1.Resource{qaRulePool: {Current: 9, Max: 9}})
	cmd := qaRuleUse("drain", "qa-drain")
	want := qaRuleLocalRefusal(t, tb, cmd)
	res := issuer.send(cmd)
	t.Logf("refusal: %s", res.GetError())
	if res.GetOk() {
		t.Fatalf("use_ability whose threshold cannot be evaluated accepted at %d", res.GetSequence())
	}
	if !strings.Contains(res.GetError(), want) {
		t.Errorf("refusal %q does not carry Resolve's %q", res.GetError(), want)
	}
	if !strings.Contains(res.GetError(), qaRulePool) {
		t.Errorf("refusal %q does not name the resource %s", res.GetError(), qaRulePool)
	}
	if strings.Contains(res.GetError(), failing) || strings.Contains(res.GetError(), qaRuleMarker) {
		t.Errorf("refusal %q carries the threshold's expression", res.GetError())
	}
	qaRuleStillOpenNothingAppended(t, tb, issuer, observer, 3)
}

// VTT-132 VTT-162 VTT-283
func TestQARuleAUseRefusedForAResourceNameAtTheBoundIsAnsweredOnAnOpenConnection(t *testing.T) {
	name := "res_" + strings.Repeat("r", qaRuleBound-4)
	rs := qaRuleRulesetOnDisk(t, name, []map[string]any{})
	tb := qaRuleServe(t, rs)
	issuer := qaRuleDial(t, tb, tb.dmToken)
	observer := qaRuleDial(t, tb, tb.obsToken)
	qaRuleBoard(t, issuer, nil)
	cmd := qaRuleUse("spend", "qa-spend")
	want := qaRuleLocalRefusal(t, tb, cmd)
	res := issuer.send(cmd)
	t.Logf("refusal (%d bytes): %s", len(res.GetError()), qaRuleElide(res.GetError(), name))
	if res.GetOk() {
		t.Fatalf("use_ability spending a resource the actor lacks accepted at %d", res.GetSequence())
	}
	if !strings.Contains(res.GetError(), want) {
		t.Errorf("refusal %q does not carry Resolve's %q", qaRuleElide(res.GetError(), name), qaRuleElide(want, name))
	}
	qaRuleStillOpenNothingAppended(t, tb, issuer, observer, 3)
}

// VTT-132 VTT-162
func TestQARuleAUseOfAnAbilityTheServerCannotServeIsRefusedOnAnOpenConnection(t *testing.T) {
	long := strings.Repeat("a", qaRuleBound+1)
	cases := []struct {
		label string
		rs    func(*testing.T) *rules.Ruleset
	}{
		{"no ruleset", func(*testing.T) *rules.Ruleset { return nil }},
		{"an ability the ruleset does not hold", func(t *testing.T) *rules.Ruleset {
			t.Helper()
			return qaRuleRulesetOnDisk(t, qaRulePool, []map[string]any{})
		}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			tb := qaRuleServe(t, c.rs(t))
			issuer := qaRuleDial(t, tb, tb.dmToken)
			observer := qaRuleDial(t, tb, tb.obsToken)
			qaRuleBoard(t, issuer, nil)
			res := issuer.send(qaRuleUse("use", long))
			t.Logf("refusal: %s", qaRuleElide(res.GetError(), long))
			if res.GetOk() || res.GetError() == "" {
				t.Fatalf("use_ability the server cannot serve: ok=%v err=%q", res.GetOk(), res.GetError())
			}
			qaRuleStillOpenNothingAppended(t, tb, issuer, observer, 3)
		})
	}
}
