// Package conformance is the P4 proof harness (ruleset-interpreter spec
// §8): Run validates that a ruleset directory conforms to the platform's
// contract with `internal/rules` — schema + cross-reference loading
// succeeds (Load), every declared ability resolves against a generic
// fixture actor built entirely from the manifest's own declarations
// (the "smoke" pass), and — for every golden fixture the ruleset ships
// under goldens/*.json — a fixed-seed roll sequence reproduces its exact
// recorded event batch (or exact rejection). Run knows nothing about any
// specific game system: it is driven entirely by what Load returns and by
// data the ruleset itself ships, so the SAME Run(dir) call is the
// conformance gate for tavern-brawl, 5b's dnd45e-minimal, or any future
// ruleset — this genericity, not any one ruleset's content, is the P4
// proof.
//
// # Golden fixture format (owned by this package, not part of the ruleset
// format itself — see format.go/load.go for that)
//
// Each rulesets/<id>/goldens/*.json names one resolution scenario:
//
//	{
//	  "name": "fists-hit",
//	  "ability_id": "fists",
//	  "scene": "tavern",                  // optional, defaults to "scene"
//	  "actors": [
//	    {"id": "brawler", "x": 0, "y": 0, "attributes": {"brawn": 3},
//	     "resources": {"drink": {"current": 0, "max": 3}}}
//	  ],
//	  "conditions": [{"actor_id": "...", "condition_id": "..."}],  // optional pre-existing conditions
//	  "actor_id": "brawler",
//	  "target_ids": ["patron"],
//	  "rolls": [{"results": [15]}],        // one entry per expression that
//	                                       // actually rolls dice, in the
//	                                       // exact order Resolve evaluates
//	                                       // them (attack roll first, then
//	                                       // each hit/miss/effect
//	                                       // resource_change that rolls)
//	  "want_error": "",                    // set XOR want_events, never both
//	  "want_events": [
//	    {"type": "AbilityUsed", "actor_id": "...", "ability_id": "...",
//	     "target_ids": ["..."], "rolls": [...], "outcome_summary": "..."},
//	    {"type": "ResourceChanged", "actor_id": "...", "resource": "...",
//	     "delta": 0, "new_value": 0, "reason": "..."},
//	    {"type": "ConditionApplied", "actor_id": "...",
//	     "condition_id": "...", "source": "..."},
//	    {"type": "ConditionRemoved", "actor_id": "...",
//	     "condition_id": "...", "reason": "..."}
//	  ]
//	}
//
// A golden with want_error set expects Resolve to return a clean error
// containing that substring and a nil event slice; one with want_events
// set (want_error empty) expects Resolve to succeed and produce EXACTLY
// that ordered batch, field for field.
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Run validates the ruleset directory at dir. A nil return means dir is a
// fully conformant ruleset: it loads cleanly, every ability it declares
// resolves against a fixture actor generated from its own manifest, and
// every golden fixture it ships (if any) reproduces exactly.
func Run(dir string) error {
	rs, err := rules.Load(dir)
	if err != nil {
		return fmt.Errorf("conformance: %s: load: %w", dir, err)
	}
	if err := smokeTest(rs); err != nil {
		return fmt.Errorf("conformance: %s: %w", dir, err)
	}
	if err := runGoldens(dir, rs); err != nil {
		return fmt.Errorf("conformance: %s: %w", dir, err)
	}
	return nil
}

// --- smoke pass: every declared ability resolves against a fixture actor
// built entirely from the manifest's own declarations ---

const (
	fixtureSceneID  = "conformance-scene"
	fixtureCasterID = "conformance-caster"
	fixtureTargetID = "conformance-target"

	// fixturePlaceholder is the value given to every declared attribute
	// and defense on the generated fixture actors — arbitrary but
	// generous enough that a typical attack-roll-vs-defense comparison
	// doesn't degenerate (both sides get the same placeholder, so an
	// at-least-one-attribute-plus-d20 roll usually clears it).
	fixturePlaceholder = int32(10)

	// fixtureResourceFallbackMax is used only for a resource that has no
	// default_max_expr — generous enough to cover any at-will ability's
	// resource_change and any limited-use ability's cost in a
	// reasonably-designed ruleset.
	fixtureResourceFallbackMax = int32(1000)
)

// smokeRoller always rolls the minimum face value — deterministic and
// bounded, so the smoke pass never depends on randomness; it exists only
// to let dice-bearing expressions evaluate at all, not to pin any specific
// numeric outcome (that's the golden fixtures' job).
type smokeRoller struct{}

func (smokeRoller) Roll(n, sides int) ([]int, int) {
	res := make([]int, n)
	for i := range res {
		res[i] = 1
	}
	return res, n
}

func smokeTest(rs *rules.Ruleset) error {
	st := buildFixtureState(rs)

	ids := make([]string, 0, len(rs.Abilities))
	for id := range rs.Abilities {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic order; map iteration never leaks out

	for _, id := range ids {
		cmd := &vttv1.UseAbility{ActorId: fixtureCasterID, AbilityId: id, TargetIds: []string{fixtureTargetID}}
		if _, err := rules.Resolve(rs, st, cmd, smokeRoller{}); err != nil {
			return fmt.Errorf("ability %q failed to resolve against the generated fixture actor: %w", id, err)
		}
	}
	return nil
}

// buildFixtureState builds a two-actor, one-scene engine.State entirely
// from rs's own declarations: every declared attribute and defense name
// gets fixturePlaceholder on BOTH actors (a smoke test can't know which
// role — attacker or target — any given ability expects of which actor,
// so both carry every stat); every declared resource gets a max computed
// from its default_max_expr when present (evaluated against the same
// placeholder attributes) or fixtureResourceFallbackMax otherwise, with
// current seeded to that max so at-will/limited usage costs and
// resource_change outcomes have generous headroom. Both actors' tokens
// share one grid cell, so every declared range (including 0) is
// satisfied regardless of which ability is being smoke-tested.
func buildFixtureState(rs *rules.Ruleset) *engine.State {
	st := engine.NewState()
	st.Scenes[fixtureSceneID] = engine.Scene{ID: fixtureSceneID, Name: fixtureSceneID, GridWidth: 1, GridHeight: 1}

	attrs := map[string]int32{}
	for _, a := range rs.Attributes {
		attrs[a] = fixturePlaceholder
	}
	for _, d := range rs.Defenses {
		attrs[d] = fixturePlaceholder
	}
	attrsInt := make(map[string]int, len(attrs))
	for k, v := range attrs {
		attrsInt[k] = int(v)
	}

	buildResources := func() map[string]*vttv1.Resource {
		out := make(map[string]*vttv1.Resource, len(rs.Resources))
		for _, r := range rs.Resources {
			max := fixtureResourceFallbackMax
			if r.DefaultMaxExpr != nil {
				if v, err := rules.Eval(r.DefaultMaxExpr, attrsInt, nil, smokeRoller{}); err == nil && v > 0 {
					max = int32(v)
				}
			}
			out[r.Name] = &vttv1.Resource{Current: max, Max: max}
		}
		return out
	}

	cloneAttrs := func() map[string]int32 {
		out := make(map[string]int32, len(attrs))
		for k, v := range attrs {
			out[k] = v
		}
		return out
	}

	st.Actors[fixtureCasterID] = &vttv1.Actor{ActorId: fixtureCasterID, Name: fixtureCasterID, Attributes: cloneAttrs(), Resources: buildResources()}
	st.Actors[fixtureTargetID] = &vttv1.Actor{ActorId: fixtureTargetID, Name: fixtureTargetID, Attributes: cloneAttrs(), Resources: buildResources()}
	st.Tokens["conformance-caster-token"] = engine.Token{ID: "conformance-caster-token", SceneID: fixtureSceneID, ActorID: fixtureCasterID, X: 0, Y: 0}
	st.Tokens["conformance-target-token"] = engine.Token{ID: "conformance-target-token", SceneID: fixtureSceneID, ActorID: fixtureTargetID, X: 0, Y: 0}
	return st
}

// --- golden pass ---

func runGoldens(dir string, rs *rules.Ruleset) error {
	paths, err := filepath.Glob(filepath.Join(dir, "goldens", "*.json"))
	if err != nil {
		return fmt.Errorf("goldens: %w", err)
	}
	sort.Strings(paths) // deterministic order across a filesystem's own listing
	covered := make(map[string]bool, len(rs.Abilities))
	for _, p := range paths {
		g, err := decodeGolden(p)
		if err != nil {
			return fmt.Errorf("golden %s: %w", p, err)
		}
		if err := runGolden(g, rs); err != nil {
			return fmt.Errorf("golden %s: %w", p, err)
		}
		covered[g.AbilityID] = true
	}

	// Spec §8: a golden scenario per ability. Enforce that every declared
	// ability has at least one golden — otherwise the forever-gate can
	// silently lose all its pins (a goldens/ rename, a .JSON typo, an
	// accidental deletion) with zero signal, or a future ruleset can satisfy
	// "the SAME suite untouched" with no golden coverage at all. Deterministic
	// error (sorted) naming the first uncovered ability.
	var uncovered []string
	for id := range rs.Abilities {
		if !covered[id] {
			uncovered = append(uncovered, id)
		}
	}
	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		return fmt.Errorf("ability %q has no golden scenario (spec §8 requires at least one golden per declared ability)", uncovered[0])
	}
	return nil
}

type goldenFile struct {
	Name       string            `json:"name"`
	AbilityID  string            `json:"ability_id"`
	Scene      string            `json:"scene"`
	Actors     []goldenActor     `json:"actors"`
	Conditions []goldenCondition `json:"conditions"`
	ActorID    string            `json:"actor_id"`
	TargetIDs  []string          `json:"target_ids"`
	Rolls      []goldenRoll      `json:"rolls"`
	WantError  string            `json:"want_error"`
	WantEvents []goldenEvent     `json:"want_events"`
}

type goldenActor struct {
	ID         string                    `json:"id"`
	X          int32                     `json:"x"`
	Y          int32                     `json:"y"`
	Attributes map[string]int32          `json:"attributes"`
	Resources  map[string]goldenResource `json:"resources"`
}

type goldenResource struct {
	Current, Max int32
}

func (r *goldenResource) UnmarshalJSON(data []byte) error {
	var obj struct {
		Current int32 `json:"current"`
		Max     int32 `json:"max"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&obj); err != nil {
		return err
	}
	r.Current, r.Max = obj.Current, obj.Max
	return nil
}

type goldenCondition struct {
	ActorID     string `json:"actor_id"`
	ConditionID string `json:"condition_id"`
}

type goldenRoll struct {
	Results []int `json:"results"`
}

type goldenEvent struct {
	Type           string            `json:"type"`
	ActorID        string            `json:"actor_id"`
	AbilityID      string            `json:"ability_id"`
	TargetIDs      []string          `json:"target_ids"`
	Rolls          []goldenEventRoll `json:"rolls"`
	OutcomeSummary string            `json:"outcome_summary"`
	Resource       string            `json:"resource"`
	Delta          int32             `json:"delta"`
	NewValue       int32             `json:"new_value"`
	Reason         string            `json:"reason"`
	ConditionID    string            `json:"condition_id"`
	Source         string            `json:"source"`
}

type goldenEventRoll struct {
	Expression string  `json:"expression"`
	Results    []int32 `json:"results"`
	Total      int32   `json:"total"`
}

func decodeGolden(path string) (*goldenFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var g goldenFile
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &g, nil
}

func runGolden(g *goldenFile, rs *rules.Ruleset) error {
	scene := g.Scene
	if scene == "" {
		scene = "scene"
	}
	st := engine.NewState()
	st.Scenes[scene] = engine.Scene{ID: scene, Name: scene, GridWidth: 100, GridHeight: 100}
	for i, a := range g.Actors {
		resources := make(map[string]*vttv1.Resource, len(a.Resources))
		for name, r := range a.Resources {
			resources[name] = &vttv1.Resource{Current: r.Current, Max: r.Max}
		}
		st.Actors[a.ID] = &vttv1.Actor{ActorId: a.ID, Name: a.ID, Attributes: a.Attributes, Resources: resources}
		tokID := fmt.Sprintf("%s-tok-%d", g.Name, i)
		st.Tokens[tokID] = engine.Token{ID: tokID, SceneID: scene, ActorID: a.ID, X: a.X, Y: a.Y}
	}
	for _, c := range g.Conditions {
		st.Conditions[c.ActorID] = append(st.Conditions[c.ActorID], engine.ActorCondition{ID: c.ConditionID, Source: "golden"})
	}

	roller := &tableRoller{steps: g.Rolls}
	cmd := &vttv1.UseAbility{ActorId: g.ActorID, AbilityId: g.AbilityID, TargetIds: g.TargetIDs}
	envs, err := rules.Resolve(rs, st, cmd, roller)

	if g.WantError != "" {
		if err == nil {
			return fmt.Errorf("%s: want error containing %q, got nil (events: %v)", g.Name, g.WantError, envs)
		}
		if !strings.Contains(err.Error(), g.WantError) {
			return fmt.Errorf("%s: error = %q, want it to contain %q", g.Name, err.Error(), g.WantError)
		}
		if envs != nil {
			return fmt.Errorf("%s: want nil events alongside a rejection, got %v", g.Name, envs)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: unexpected error: %w", g.Name, err)
	}
	return compareEvents(g.Name, envs, g.WantEvents)
}

// tableRoller replays a golden's recorded roll sequence, one step per Roll
// call, in order — the "simple counter/table roller" the task calls for:
// no math/rand, no dependency on any Go version's PRNG stream, just a
// fixed table this package owns outright.
type tableRoller struct {
	steps []goldenRoll
	i     int
}

func (r *tableRoller) Roll(n, sides int) ([]int, int) {
	if r.i >= len(r.steps) {
		// A golden with too few recorded steps for what Resolve actually
		// evaluates is a broken fixture, not a platform bug — return a
		// zero-valued roll rather than panicking; the resulting mismatch
		// against want_events (or the wrong branch taken entirely) will
		// surface as an ordinary conformance failure naming the golden.
		return make([]int, n), 0
	}
	step := r.steps[r.i]
	r.i++
	total := 0
	for _, v := range step.Results {
		total += v
	}
	return step.Results, total
}

func compareEvents(name string, got []*vttv1.Envelope, want []goldenEvent) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s: got %d events, want %d\ngot:  %v\nwant: %v", name, len(got), len(want), got, want)
	}
	for i, w := range want {
		if err := compareOneEvent(got[i], w); err != nil {
			return fmt.Errorf("%s: event[%d]: %w", name, i, err)
		}
	}
	return nil
}

func compareOneEvent(env *vttv1.Envelope, w goldenEvent) error {
	switch w.Type {
	case "AbilityUsed":
		p, ok := env.Payload.(*vttv1.Envelope_AbilityUsed)
		if !ok {
			return fmt.Errorf("payload = %T, want AbilityUsed", env.Payload)
		}
		au := p.AbilityUsed
		if au.ActorId != w.ActorID || au.AbilityId != w.AbilityID || au.OutcomeSummary != w.OutcomeSummary {
			return fmt.Errorf("AbilityUsed = %+v, want actor_id=%q ability_id=%q outcome_summary=%q", au, w.ActorID, w.AbilityID, w.OutcomeSummary)
		}
		if !equalStrings(au.TargetIds, w.TargetIDs) {
			return fmt.Errorf("AbilityUsed.TargetIds = %v, want %v", au.TargetIds, w.TargetIDs)
		}
		if len(au.Rolls) != len(w.Rolls) {
			return fmt.Errorf("AbilityUsed.Rolls = %v, want %v", au.Rolls, w.Rolls)
		}
		for i, r := range w.Rolls {
			got := au.Rolls[i]
			if got.Expression != r.Expression || got.Total != r.Total || !equalInt32s(got.Results, r.Results) {
				return fmt.Errorf("AbilityUsed.Rolls[%d] = %+v, want %+v", i, got, r)
			}
		}
		return nil

	case "ResourceChanged":
		p, ok := env.Payload.(*vttv1.Envelope_ResourceChanged)
		if !ok {
			return fmt.Errorf("payload = %T, want ResourceChanged", env.Payload)
		}
		rc := p.ResourceChanged
		if rc.ActorId != w.ActorID || rc.Resource != w.Resource || rc.Delta != w.Delta || rc.NewValue != w.NewValue || rc.Reason != w.Reason {
			return fmt.Errorf("ResourceChanged = %+v, want %+v", rc, w)
		}
		return nil

	case "ConditionApplied":
		p, ok := env.Payload.(*vttv1.Envelope_ConditionApplied)
		if !ok {
			return fmt.Errorf("payload = %T, want ConditionApplied", env.Payload)
		}
		ca := p.ConditionApplied
		if ca.ActorId != w.ActorID || ca.ConditionId != w.ConditionID || ca.Source != w.Source {
			return fmt.Errorf("ConditionApplied = %+v, want %+v", ca, w)
		}
		return nil

	case "ConditionRemoved":
		p, ok := env.Payload.(*vttv1.Envelope_ConditionRemoved)
		if !ok {
			return fmt.Errorf("payload = %T, want ConditionRemoved", env.Payload)
		}
		cr := p.ConditionRemoved
		if cr.ActorId != w.ActorID || cr.ConditionId != w.ConditionID || cr.Reason != w.Reason {
			return fmt.Errorf("ConditionRemoved = %+v, want %+v", cr, w)
		}
		return nil

	default:
		return fmt.Errorf("golden declares unknown event type %q", w.Type)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInt32s(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
