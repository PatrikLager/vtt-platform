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
//	                                       // them, per target: the
//	                                       // resolution roll first, then —
//	                                       // for a v2 vs-with-dice
//	                                       // resolution — the Vs roll
//	                                       // (recorded only when it rolls),
//	                                       // then each hit/miss/effect
//	                                       // resource_change that rolls
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
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Run validates the ruleset directory at dir. A nil return means dir is a
// fully conformant ruleset: it loads cleanly, every ability it declares
// resolves against a fixture actor generated from its own manifest, every
// golden fixture it ships (if any) reproduces exactly, and — for a
// format-v2 ruleset (Task 3, spec §8) — every declared ability ships a
// matching compiled-form golden under goldens/compiled/.
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
	if err := runCompiledGoldens(dir, rs); err != nil {
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

	// Iterates rs.Compiled, not rs.Abilities (Task 3, spec 5c pillar:
	// "Resolve executes CompiledPower through ONE code path"): Compiled is
	// now populated for EVERY successfully-loaded ruleset, v1 (via the
	// load.go adapter) and v2 (via compile.go) alike, with the exact same
	// key set Abilities would have had for a v1 ruleset — so this is a
	// pure rename for v1's existing behavior, and closes a real gap for
	// v2: a v2-loaded Ruleset's Abilities is deliberately empty (v2's
	// abilities/*.json are compositions, not v1 Ability-shaped), so this
	// smoke pass previously iterated zero abilities for any v2 ruleset.
	ids := make([]string, 0, len(rs.Compiled))
	for id := range rs.Compiled {
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
// from its default_max_expr when that expression is present AND evaluates
// to a POSITIVE value (against the same placeholder attributes), or
// fixtureResourceFallbackMax otherwise — 0 falls back exactly as a missing
// expression does, since a stand-in actor with a max of 0 could afford no
// limited ability and every smoke failure would say nothing about the
// ruleset (pinned by minimal-v2-zero-default-max), with
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
			largest := fixtureResourceFallbackMax
			if r.DefaultMaxExpr != nil {
				if v, err := rules.Eval(r.DefaultMaxExpr, attrsInt, nil, smokeRoller{}); err == nil && v > 0 {
					// A ruleset authors this expression, and nothing bounds it
					// to int32 — see int32Checked's doc comment in resolve.go:
					// the grammar's closed '*' has no magnitude bound beyond
					// the int literal fit. An unclamped conversion wraps to a
					// NEGATIVE Max, which silently produces a nonsense smoke
					// fixture instead of failing. Clamp: this is a generic
					// stand-in actor, so a saturated ceiling is the honest
					// reading of "bigger than any resource could need".
					if v > math.MaxInt32 {
						v = math.MaxInt32
					}
					largest = int32(v)
				}
			}
			out[r.Name] = &vttv1.Resource{Current: largest, Max: largest}
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
	covered := make(map[string]bool, len(rs.Compiled))
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
	// error (sorted) naming the first uncovered ability. Iterates rs.Compiled
	// (Task 3), not rs.Abilities — see smokeTest's comment; this is what
	// makes batch-golden enforcement apply to v2 rulesets' abilities for the
	// first time, not just v1's.
	var uncovered []string
	for id := range rs.Compiled {
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

// --- compiled-form goldens (Task 3, spec 5c §6/§8) ---
//
// A compiled-form golden (goldens/compiled/<ability>.json) pins the
// flattened CompiledPower a ruleset's Load produces for one ability — spec
// §6's "inspectable artifact... conformance can dump it" — so a refactor
// of a v2 ruleset's atoms that changes what an ability compiles TO (not
// just how it's authored) is a visible, reviewed drift, not a silent
// behavior change. REQUIRED per declared ability: every ruleset loads as
// format_version "2" (format v1 is retired — load.go), so every ability a
// ruleset declares ships a compiled-form golden. The canonical
// serialization is compiledPowerDump below:
// stable field order (a struct, not CompiledPower's own map-shaped
// internals — it has none, but the DTO shape is what's actually
// marshaled, independent of Go's zero-value/field-order quirks), and
// expression SourceText (RollSrc/VsSrc/DeltaExprSrc — spec's explicit
// "not AST" requirement; CompiledPower's *Expr fields have no exported
// internals to marshal at all, so this DTO is the only way to serialize
// one regardless).

// DumpCompiledPower renders cp as the canonical JSON serialization
// compiled-form goldens pin. Exported as the "dump helper... for
// authoring" the task brief calls for: derive a golden by loading the
// real ruleset, running this against rs.Compiled[id], and writing the
// result verbatim to goldens/compiled/<id>.json — runCompiledGoldens
// (below) is the read side of the exact same DTO.
func DumpCompiledPower(cp *rules.CompiledPower) ([]byte, error) {
	b, err := json.MarshalIndent(toCompiledPowerDump(cp), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("conformance: dump compiled power %q: %w", cp.ID, err)
	}
	return append(b, '\n'), nil
}

type compiledPowerDump struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Usage          usageDump        `json:"usage"`
	Targeting      targetingDump    `json:"targeting"`
	Resolution     *resolutionDump  `json:"resolution,omitempty"`
	BranchOutcomes [2][]outcomeDump `json:"branch_outcomes"`
	Effects        []outcomeDump    `json:"effects,omitempty"`
}

type usageDump struct {
	AtWill  bool         `json:"at_will,omitempty"`
	Limited *limitedDump `json:"limited,omitempty"`
}

type limitedDump struct {
	Resource string `json:"resource"`
	Cost     int    `json:"cost"`
}

type targetingDump struct {
	Range      int `json:"range"`
	MaxTargets int `json:"max_targets"`
}

type resolutionDump struct {
	Roll     string    `json:"roll"`
	Vs       string    `json:"vs"`
	Branches [2]string `json:"branches"`
}

// outcomeDump mirrors rules.Outcome's oneof-by-Kind shape (format.go):
// exactly one of Resource+DeltaExpr, ConditionID (for apply_condition), or
// ConditionID (for remove_condition — Kind disambiguates which) is
// populated, matching the field it came from.
type outcomeDump struct {
	Kind        string `json:"kind"`
	Resource    string `json:"resource,omitempty"`
	DeltaExpr   string `json:"delta_expr,omitempty"`
	ConditionID string `json:"condition_id,omitempty"`
}

const (
	outcomeKindResourceChange  = "resource_change"
	outcomeKindApplyCondition  = "apply_condition"
	outcomeKindRemoveCondition = "remove_condition"
)

func toCompiledPowerDump(cp *rules.CompiledPower) compiledPowerDump {
	dto := compiledPowerDump{
		ID:             cp.ID,
		Name:           cp.Name,
		Usage:          usageDump{AtWill: cp.Usage.AtWill},
		Targeting:      targetingDump{Range: cp.Targeting.Range, MaxTargets: cp.Targeting.MaxTargets},
		BranchOutcomes: [2][]outcomeDump{toOutcomeDumps(cp.BranchOutcomes[0]), toOutcomeDumps(cp.BranchOutcomes[1])},
		Effects:        toOutcomeDumps(cp.Effects),
	}
	if cp.Usage.Limited != nil {
		dto.Usage.Limited = &limitedDump{Resource: cp.Usage.Limited.Resource, Cost: cp.Usage.Limited.Cost}
	}
	if cp.Resolution != nil {
		dto.Resolution = &resolutionDump{
			Roll:     cp.Resolution.RollSrc,
			Vs:       cp.Resolution.VsSrc,
			Branches: cp.Resolution.Branches,
		}
	}
	return dto
}

func toOutcomeDumps(outcomes []rules.Outcome) []outcomeDump {
	if len(outcomes) == 0 {
		return nil
	}
	out := make([]outcomeDump, len(outcomes))
	for i, o := range outcomes {
		switch o.Kind {
		case rules.OutcomeResourceChange:
			out[i] = outcomeDump{Kind: outcomeKindResourceChange, Resource: o.ResourceChange.Resource, DeltaExpr: o.ResourceChange.DeltaExprSrc}
		case rules.OutcomeApplyCondition:
			out[i] = outcomeDump{Kind: outcomeKindApplyCondition, ConditionID: o.ApplyCondition.ID}
		case rules.OutcomeRemoveCondition:
			out[i] = outcomeDump{Kind: outcomeKindRemoveCondition, ConditionID: o.RemoveCondition.ID}
		}
	}
	return out
}

// runCompiledGoldens enforces spec §8's per-ability compiled-form golden:
// goldens/compiled/<id>.json must exist and deep-equal (via
// compiledPowerDump, so JSON formatting/key-order differences never cause a
// false failure — only real content drift does) the ruleset's actual
// rs.Compiled[id]. A missing file or a content mismatch is a named failure,
// naming both the ability and the golden path, with a got/want dump on
// mismatch (mirroring compareEvents' diagnostic style below for the
// batch-golden pass). Every ruleset loads as format_version "2" (v1 is
// retired), so there is no exemption.
func runCompiledGoldens(dir string, rs *rules.Ruleset) error {
	ids := make([]string, 0, len(rs.Compiled))
	for id := range rs.Compiled {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic order; map iteration never leaks out

	for _, id := range ids {
		path := filepath.Join(dir, "goldens", "compiled", id+".json")
		wantBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("ability %q: missing compiled golden %s: %w", id, path, err)
		}
		var want compiledPowerDump
		dec := json.NewDecoder(bytes.NewReader(wantBytes))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&want); err != nil {
			return fmt.Errorf("ability %q: compiled golden %s: decode: %w", id, path, err)
		}
		got := toCompiledPowerDump(rs.Compiled[id])
		if !reflect.DeepEqual(got, want) {
			gotBytes, dumpErr := DumpCompiledPower(rs.Compiled[id])
			if dumpErr != nil {
				return fmt.Errorf("ability %q: compiled golden %s does not match the compiled power (drift), and re-dumping for diagnostics failed: %w", id, path, dumpErr)
			}
			return fmt.Errorf("ability %q: compiled golden %s does not match the compiled power (drift)\ngot:\n%s\nwant:\n%s", id, path, gotBytes, wantBytes)
		}
	}

	// Reverse direction (finding R1): a goldens/compiled/*.json file whose
	// name matches no compiled ability is a stale pin (a rename or deletion
	// left it behind) — the forward loop above never reads it, so without
	// this check it sits in the repo as an authoritative-looking artifact
	// forever. Reject it, naming the orphan. The golden filename = ability id
	// convention is thus validated in BOTH directions.
	compiledDir := filepath.Join(dir, "goldens", "compiled")
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No compiled-golden directory at all: the forward loop already
			// failed above for any declared ability, and a ruleset with zero
			// abilities has nothing to orphan.
			return nil
		}
		return fmt.Errorf("reading compiled goldens directory %s: %w", compiledDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if _, ok := rs.Compiled[id]; !ok {
			return fmt.Errorf("orphan compiled golden %s: names ability %q, which this ruleset does not declare (a stale pin left by a rename or deletion)", filepath.Join(compiledDir, e.Name()), id)
		}
	}
	return nil
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
