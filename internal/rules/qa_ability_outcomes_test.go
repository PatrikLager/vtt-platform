package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaasRoller struct {
	faces []int
	next  int
}

func (r *qaasRoller) Roll(n, _ int) ([]int, int) {
	out := make([]int, n)
	sum := 0
	for i := range out {
		face := 1
		if r.next < len(r.faces) {
			face = r.faces[r.next]
		}
		r.next++
		out[i] = face
		sum += face
	}
	return out, sum
}

var qaasRulesetFiles = map[string]string{
	"ruleset.json": `{
  "id": "qa-sight", "name": "QA Sight", "format_version": "2",
  "attributes": ["knack"], "defenses": ["guard"],
  "resources": [{ "name": "pool", "default_max_expr": "5", "thresholds": [] }]
}`,
	"guide.md": "# QA Sight\n",
	"atoms/aim.json": `{
  "id": "aim",
  "params": [{ "name": "distance", "kind": "int" }, { "name": "targets", "kind": "int" }],
  "provides": ["aimed"], "consumes": [],
  "contributes": [{ "kind": "targeting", "range": "{distance}", "max_targets": "{targets}" }]
}`,
	"atoms/contest.json": `{
  "id": "contest", "params": [], "provides": ["clash"], "consumes": ["aimed"],
  "contributes": [{ "kind": "resolution", "key": "clash",
    "roll": "1d20 + @caster.knack", "vs": "@target.guard",
    "branches": ["struck", "glanced"] }]
}`,
	"atoms/rolled-contest.json": `{
  "id": "rolled-contest", "params": [], "provides": ["clash"], "consumes": ["aimed"],
  "contributes": [{ "kind": "resolution", "key": "clash",
    "roll": "1d20 + @caster.knack", "vs": "@target.guard + 1d4",
    "branches": ["struck", "glanced"] }]
}`,
	"atoms/mend.json": `{
  "id": "mend", "params": [], "provides": [], "consumes": [],
  "contributes": [{ "kind": "outcome", "key": null, "branch": "always",
    "effects": [{ "resource_change": { "resource": "pool", "delta_expr": "1" } }] }]
}`,
	"abilities/volley.json": `{
  "id": "volley", "name": "Volley", "usage": "at_will",
  "compose": [
    { "atom": "aim", "bind": { "distance": 15, "targets": 2 } },
    { "atom": "contest", "bind": {} }
  ]
}`,
	"abilities/duel.json": `{
  "id": "duel", "name": "Duel", "usage": "at_will",
  "compose": [
    { "atom": "aim", "bind": { "distance": 15, "targets": 1 } },
    { "atom": "rolled-contest", "bind": {} }
  ]
}`,
	"abilities/salve.json": `{
  "id": "salve", "name": "Salve", "usage": "at_will",
  "compose": [
    { "atom": "aim", "bind": { "distance": 15, "targets": 1 } },
    { "atom": "mend", "bind": {} }
  ]
}`,
}

func qaasLoad(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	for name, body := range qaasRulesetFiles {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return rs
}

type qaasFigure struct {
	id    string
	x     int32
	attrs map[string]int32
	pools map[string]*vttv1.Resource
}

func qaasState(t *testing.T, figures ...qaasFigure) *engine.State {
	t.Helper()
	st := engine.NewState()
	seq := int64(0)
	apply := func(env *vttv1.Envelope) {
		seq++
		env.Sequence = seq
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("apply %d: %v", seq, err)
		}
	}
	apply(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: "yard", Name: "Yard", GridWidth: 20, GridHeight: 20,
		},
	}})
	for _, f := range figures {
		apply(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
			ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId:    f.id,
				Name:       f.id,
				Attributes: f.attrs,
				Resources:  f.pools,
				Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			}},
		}})
		apply(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{
			TokenPlaced: &vttv1.TokenPlaced{
				TokenId:  "tok-" + f.id,
				SceneId:  "yard",
				ActorId:  f.id,
				Position: &vttv1.GridPosition{X: f.x, Y: 0},
			},
		}})
	}
	return st
}

func qaasAbilityUsed(t *testing.T, envs []*vttv1.Envelope) *vttv1.AbilityUsed {
	t.Helper()
	if len(envs) == 0 {
		t.Fatal("no events")
	}
	used := envs[0].GetAbilityUsed()
	if used == nil {
		t.Fatalf("first event is not AbilityUsed: %v", envs[0])
	}
	return used
}

func qaasVolleyState(t *testing.T) *engine.State {
	t.Helper()
	return qaasState(t,
		qaasFigure{id: "caster", x: 0, attrs: map[string]int32{"knack": 3}},
		qaasFigure{id: "zed", x: 2, attrs: map[string]int32{"guard": 18}},
		qaasFigure{id: "abe", x: 4, attrs: map[string]int32{"guard": 9}},
	)
}

// VTT-473
func TestQAASEachTargetOfAResolvedAbilityGetsItsOwnOutcomeInTargetOrder(t *testing.T) {
	rs := qaasLoad(t)
	st := qaasVolleyState(t)
	envs, err := rules.Resolve(rs, st, &vttv1.UseAbility{
		ActorId: "caster", AbilityId: "volley", TargetIds: []string{"zed", "abe"},
	}, &qaasRoller{faces: []int{8, 20}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got := qaasAbilityUsed(t, envs).GetOutcomes()
	want := []struct {
		target, result string
		total          int32
	}{
		{"zed", "glanced", 11},
		{"abe", "struck", 23},
	}
	if len(got) != len(want) {
		t.Fatalf("outcomes = %v, want %d of them", got, len(want))
	}
	for i, w := range want {
		o := got[i]
		if o.GetTargetId() != w.target || o.GetResult() != w.result ||
			o.GetTotal() != w.total {
			t.Errorf("outcome %d = %v, want %+v", i, o, w)
		}
	}
}

// VTT-473
func TestQAASAnOutcomeCarriesTheTargetTheBranchAndTheTotalAndNothingElse(t *testing.T) {
	desc := (&vttv1.AbilityUsed_Outcome{}).ProtoReflect().Descriptor()
	var names []string
	for i := 0; i < desc.Fields().Len(); i++ {
		names = append(names, string(desc.Fields().Get(i).Name()))
	}
	if strings.Join(names, ",") != "target_id,result,total" {
		t.Errorf("AbilityUsed.Outcome fields = %v", names)
	}

	rs := qaasLoad(t)
	envs, err := rules.Resolve(rs, qaasVolleyState(t), &vttv1.UseAbility{
		ActorId: "caster", AbilityId: "volley", TargetIds: []string{"zed", "abe"},
	}, &qaasRoller{faces: []int{8, 20}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, o := range qaasAbilityUsed(t, envs).GetOutcomes() {
		o.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			switch fd.Name() {
			case "target_id", "result", "total":
			default:
				t.Errorf("outcome %v carries %s", o, fd.Name())
			}
			return true
		})
		if len(o.ProtoReflect().GetUnknown()) != 0 {
			t.Errorf("outcome %v carries unknown fields", o)
		}
		text, err := protojson.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		for _, guard := range []string{"18", "9"} {
			if strings.Contains(string(text), guard) {
				t.Errorf("outcome %s carries the defense %s", text, guard)
			}
		}
	}
}

// VTT-473
func TestQAASAnOutcomeTotalIsTheResolutionRollAndNotWhatItWasComparedWith(t *testing.T) {
	rs := qaasLoad(t)
	st := qaasState(t,
		qaasFigure{id: "caster", x: 0, attrs: map[string]int32{"knack": 3}},
		qaasFigure{id: "zed", x: 2, attrs: map[string]int32{"guard": 10}},
	)
	envs, err := rules.Resolve(rs, st, &vttv1.UseAbility{
		ActorId: "caster", AbilityId: "duel", TargetIds: []string{"zed"},
	}, &qaasRoller{faces: []int{14, 4}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got := qaasAbilityUsed(t, envs).GetOutcomes()
	if len(got) != 1 {
		t.Fatalf("outcomes = %v, want one", got)
	}
	if got[0].GetTotal() != 17 || got[0].GetResult() != "struck" ||
		got[0].GetTargetId() != "zed" {
		t.Errorf("outcome = %v, want zed struck 17 (14 + 3 against 10 + 4)", got[0])
	}
}

// VTT-473
func TestQAASAnAbilityWithoutAResolutionCarriesNoOutcome(t *testing.T) {
	rs := qaasLoad(t)
	st := qaasState(t,
		qaasFigure{id: "caster", x: 0, attrs: map[string]int32{"knack": 3}},
		qaasFigure{id: "zed", x: 2, pools: map[string]*vttv1.Resource{
			"pool": {Current: 1, Max: 5},
		}},
	)
	envs, err := rules.Resolve(rs, st, &vttv1.UseAbility{
		ActorId: "caster", AbilityId: "salve", TargetIds: []string{"zed"},
	}, &qaasRoller{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := qaasAbilityUsed(t, envs).GetOutcomes(); len(got) != 0 {
		t.Errorf("outcomes = %v, want none", got)
	}
}

// VTT-473
func TestQAASTheOutcomeSummaryStillCarriesTheComparedValue(t *testing.T) {
	rs := qaasLoad(t)
	envs, err := rules.Resolve(rs, qaasVolleyState(t), &vttv1.UseAbility{
		ActorId: "caster", AbilityId: "volley", TargetIds: []string{"zed", "abe"},
	}, &qaasRoller{faces: []int{8, 20}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	summary := qaasAbilityUsed(t, envs).GetOutcomeSummary()
	for _, part := range []string{"zed: glanced (11 vs 18)", "abe: struck (23 vs 9)"} {
		if !strings.Contains(summary, part) {
			t.Errorf("outcome_summary = %q, want it to hold %q", summary, part)
		}
	}
}

// VTT-473
func TestQAASTheShippedRulesetsOutcomesSpeakTheirOwnBranchWords(t *testing.T) {
	cases := []struct {
		dir, ability string
		caster       map[string]int32
		target       qaasFigure
		faces        []int
		result       string
		total        int32
	}{
		{
			dir: "tavern-brawl", ability: "fists",
			caster: map[string]int32{"brawn": 3, "grit": 1},
			target: qaasFigure{id: "patron", x: 1,
				attrs: map[string]int32{"footing": 10},
				pools: map[string]*vttv1.Resource{"drink": {Current: 0, Max: 5}}},
			faces: []int{12}, result: "hit", total: 15,
		},
		{
			dir: "dnd45e-minimal", ability: "crossbow-shot",
			caster: map[string]int32{"dex": 1},
			target: qaasFigure{id: "sentry", x: 9,
				attrs: map[string]int32{"ac": 1000}},
			faces: []int{7}, result: "miss", total: 10,
		},
	}
	for _, c := range cases {
		t.Run(c.dir, func(t *testing.T) {
			rs, err := rules.Load(filepath.Join("..", "..", "rulesets", c.dir))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			st := qaasState(t, qaasFigure{id: "caster", x: 0, attrs: c.caster}, c.target)
			envs, err := rules.Resolve(rs, st, &vttv1.UseAbility{
				ActorId: "caster", AbilityId: c.ability,
				TargetIds: []string{c.target.id},
			}, &qaasRoller{faces: c.faces})
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			got := qaasAbilityUsed(t, envs).GetOutcomes()
			want := fmt.Sprintf("%s %s %d", c.target.id, c.result, c.total)
			if len(got) != 1 || fmt.Sprintf("%s %s %d", got[0].GetTargetId(),
				got[0].GetResult(), got[0].GetTotal()) != want {
				t.Errorf("outcomes = %v, want [%s]", got, want)
			}
		})
	}
}

// VTT-472
func TestQAASATargetNamingNoActorIsRefusedByUnknownTarget(t *testing.T) {
	rs := qaasLoad(t)
	_, err := rules.Resolve(rs, qaasVolleyState(t), &vttv1.UseAbility{
		ActorId: "caster", AbilityId: "volley", TargetIds: []string{"zed", "ghost"},
	}, &qaasRoller{faces: []int{8, 20}})
	if err == nil || err.Error() != rules.UnknownTarget("ghost").Error() {
		t.Errorf("err = %v, want %v", err, rules.UnknownTarget("ghost"))
	}
}
