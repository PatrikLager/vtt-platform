package rules

import (
	"fmt"
	"math"
	"sort"
	"strings"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Resolve executes cmd (a UseAbility command) against rs and a read-only
// snapshot of st, returning the ordered batch of event payloads for exactly
// ONE campaign.AppendBatch call, or a clean error if cmd cannot be resolved
// (see the validation order below). Resolve is PURE: it never mutates st,
// never touches store/campaign, and is deterministic given rng — the same
// (rs, st, cmd, rng-that-replays-the-same-sequence) always produces the
// same ordered batch. Every returned *vttv1.Envelope has ONLY its Payload
// set; Sequence, EventId, SessionId, ActorRole, ParticipantId, and
// OccurredAt are left zero for the caller (campaign.AppendBatch / the
// gateway) to stamp — matching gateway.ToEvent's convention for every other
// command-to-event conversion in this codebase.
//
// # ONE execution path (Task 3, spec 5c §6-7)
//
// Resolve executes rs.Compiled ONLY — never an atom graph. A composition
// compiles to its CompiledPower at Load via compile.go's atom-graph
// flattening; Resolve then reads Compiled exclusively. Every rule below is
// phrased in terms of a CompiledPower's Resolution/BranchOutcomes/Effects.
//
// # Validation (in order; each failure is a clean error — nothing returned)
//
//  1. actor: cmd's ability_id must be a key of rs.Compiled; cmd's actor_id
//     must name an actor present in st.
//  2. targets: cmd must name at least one target and no more than the
//     ability's targeting.max_targets; every named target must be an actor
//     present in st.
//  3. stats: if the ability has a resolution, every '@' (attribute) ref its
//     Roll or Vs expression carries is checked upfront (resource ('#')
//     refs are NOT — see the outcome-list paragraph below, the same lazy
//     treatment applies to any resource ref inside Roll/Vs too): a
//     caster-scoped ref requires the caster to carry that attribute; a
//     target-scoped ref requires EVERY named target to carry it (defenses
//     live in the same generic Actor.Attributes map as attributes — the
//     wire format has no separate defenses map, matching v1's Vs-as-
//     defense-name convention exactly for a v1-adapted ability, whose Vs
//     is always exactly one target-scoped ref).
//  4. usage: if the ability is limited-use, the caster must already carry
//     the named resource (resources are never created by Resolve — spec
//     binding: "resource must already exist on the actor") and its current
//     value must be at least the ability's cost.
//  5. range: the caster must have a token placed (in ANY scene — that
//     scene becomes "the active scene" for this resolution); every target
//     must have a token in that SAME scene; the Chebyshev distance from the
//     caster's token to each target's token must not exceed the ability's
//     declared range. Range 0 therefore accepts only a target occupying the
//     exact same grid cell as the caster (self-targeting abilities pass
//     their own actor id as the target, which trivially satisfies this).
//     An actor with no token anywhere, or a target with no token in the
//     caster's scene, is the same "no-token/no-scene" clean error family.
//
// Outcome-list expression validity (an @/# reference a branch's outcome
// list uses that the relevant actor happens not to carry, or a
// resource_change naming a resource the target doesn't have) is
// deliberately NOT part of the upfront validation pass above: a
// resolution's two branches are mutually exclusive per target (decided by
// comparing Roll's total against Vs's, both themselves deterministic given
// rng), so checking only the branch that actually runs keeps Resolve's
// error behavior exactly as deterministic as its event output —
// EvalScoped's own "unknown attribute/resource" errors (expr.go) surface
// these lazily, at the point of use, and Resolve aborts (returns nil, err)
// the instant any such error occurs, so no partial batch is ever returned.
//
// # Execution
//
// Once validation passes: a limited-use ability's cost is spent first,
// emitted as the leading ResourceChanged (before any per-target event).
// Then, for each target in cmd order: if the ability has a resolution,
// evaluate Roll (via EvalScoped, caster and target contexts both live),
// THEN Vs (same two contexts) fresh for that target — a resolution total
// >= its Vs total selects Branches[0], else Branches[1] (v1-adapted:
// "hit"/"miss", ties included in Branches[0]/"hit", exactly v1's own
// >= comparison) — and apply the winning branch's outcome list; then apply
// the effect list unconditionally (effects always run, with or without a
// resolution). Branch labels flow into AbilityUsed.outcome_summary
// verbatim — testimony speaks the ruleset's own words, and a v1-adapted
// ability's words are always "hit"/"miss". Outcomes apply against the
// TARGET named in that slot — there is no separate "which actor" field on
// an outcome; a self-cast ability names the caster as its own target, and
// an outcome's expression may still reference the CASTER's state via an
// explicit @caster./#caster. ref (v2 only — a v1-adapted outcome
// expression is always target-scoped, matching v1's historical single-
// actor evaluation). apply_condition/remove_condition outcomes are
// idempotent: applying an already-present condition or removing an absent
// one emits nothing (a no-op), rather than risking the engine reject the
// whole atomic batch over a duplicate/absent condition event — this
// mirrors the presence guard the spec mandates for threshold-driven
// condition events (below), generalized to ability-outcome condition
// events for the same reason.
//
// Finally, every (actor, resource) pair that changed value during this
// resolution (usage spend, and/or any hit/miss/effect resource_change) has
// its resource's declared thresholds evaluated, in the order those (actor,
// resource) pairs FIRST CHANGED this resolution, then threshold declaration
// order (matching evalThresholds' own doc — change-order, not resource
// declaration order), against the actor's final
// (post-every-change-in-this-batch) attributes/resources via single-context
// Eval (thresholds are a single-actor position — spec §5 — unchanged by
// Task 3): a threshold whose "when" expression evaluates non-zero applies
// its condition if not already present; one with remove_when_false set
// removes its condition, if present, once "when" evaluates zero.
//
// Every Roll/Vs/resource_change expression that actually invoked rng is
// recorded as a Roll entry on the single AbilityUsed event, in evaluation
// order (Roll, then Vs, then each hit/miss/effect resource_change that
// rolls, all per target, in target order): Roll is ALWAYS recorded, even
// when its expression happens not to contain dice (matching v1's original
// attack-roll contract exactly); Vs and every resource_change are recorded
// ONLY when they actually rolled (see resolve.go's evalRecordingScoped) —
// this is precisely why a v1-adapted ability's Vs
// ("@target.<defense>", never containing dice) never appears in
// AbilityUsed.rolls, keeping every existing v1 golden byte-identical
// through this path. AbilityUsed is always the first event of the
// returned batch, followed by the usage-spend ResourceChanged (if any),
// then every per-target outcome event in target order, then every
// threshold-driven condition event.
// Complexity: the resolution pipeline — guards, cost, roll, outcome,
// envelopes. The branches are sequential stages sharing accumulated state,
// which is what makes extraction cost more clarity than it buys.
//
//nolint:gocyclo
func Resolve(rs *Ruleset, st *engine.State, cmd *vttv1.UseAbility, rng Roller) ([]*vttv1.Envelope, error) {
	ability, ok := rs.Compiled[cmd.GetAbilityId()]
	if !ok {
		return nil, fmt.Errorf("rules: resolve: unknown ability %q", cmd.GetAbilityId())
	}

	casterID := cmd.GetActorId()
	caster, ok := st.Actors[casterID]
	if !ok {
		return nil, fmt.Errorf("rules: resolve: unknown actor %q", casterID)
	}

	targetIDs := cmd.GetTargetIds()
	if len(targetIDs) == 0 {
		return nil, fmt.Errorf("rules: resolve: ability %q requires at least one target", ability.ID)
	}
	if len(targetIDs) > ability.Targeting.MaxTargets {
		return nil, fmt.Errorf("rules: resolve: ability %q allows at most %d target(s), got %d", ability.ID, ability.Targeting.MaxTargets, len(targetIDs))
	}
	targets := make(map[string]*vttv1.Actor, len(targetIDs))
	for _, tid := range targetIDs {
		if _, dup := targets[tid]; dup {
			// Reject rather than silently dedupe: the execution loop applies
			// per-target outcomes for every entry in target_ids, so a repeated
			// id would land the ability's stateful outcomes N times on ONE
			// actor (wire-controllable amplification within max_targets). The
			// spec is silent on duplicate semantics; rejecting is the safe
			// default and never changes a caller's intent behind their back.
			return nil, fmt.Errorf("rules: resolve: duplicate target id %q", tid)
		}
		a, ok := st.Actors[tid]
		if !ok {
			return nil, fmt.Errorf("rules: resolve: unknown target actor %q", tid)
		}
		targets[tid] = a
	}

	// Stats: only what's UNCONDITIONALLY required (every '@' ref Roll/Vs
	// carries, checked against whichever actor its scope names — the
	// defense check for a v1-adapted ability is exactly this, since its Vs
	// is always the single target-scoped ref "@target.<defense>") is
	// validated upfront; hit/miss/effect outcome expressions are validated
	// lazily as they execute (see the doc comment above). '#' (resource)
	// refs are never checked here, matching v1's original attrRefs-only
	// check — a resource ref inside Roll/Vs surfaces lazily too, via
	// EvalScoped's own error, exactly like an outcome's resource ref
	// always has.
	checkAttrScopedRefs := func(refs []ScopedRef) error {
		for _, ref := range refs {
			if ref.Sigil != '@' {
				continue
			}
			switch ref.Scope {
			case ScopeCaster:
				if _, ok := caster.GetAttributes()[ref.Name]; !ok {
					return fmt.Errorf("rules: resolve: actor %q missing attribute %q required by ability %q's resolution roll", casterID, ref.Name, ability.ID)
				}
			case ScopeTarget:
				for _, tid := range targetIDs {
					if _, ok := targets[tid].GetAttributes()[ref.Name]; !ok {
						return fmt.Errorf("rules: resolve: target actor %q missing defense %q required by ability %q", tid, ref.Name, ability.ID)
					}
				}
			}
		}
		return nil
	}
	if ability.Resolution != nil {
		if err := checkAttrScopedRefs(ability.Resolution.Roll.ScopedRefs()); err != nil {
			return nil, err
		}
		if err := checkAttrScopedRefs(ability.Resolution.Vs.ScopedRefs()); err != nil {
			return nil, err
		}
	}

	// Usage: a limited-use ability's resource must already exist on the
	// caster (Resolve never creates resource entries — that's add_actor's
	// job) and hold enough to cover the cost.
	if ability.Usage.Limited != nil {
		lu := ability.Usage.Limited
		r, ok := caster.GetResources()[lu.Resource]
		if !ok {
			return nil, fmt.Errorf("rules: resolve: actor %q has no resource %q required by ability %q's usage cost", casterID, lu.Resource, ability.ID)
		}
		if int(r.GetCurrent()) < lu.Cost {
			return nil, fmt.Errorf("rules: resolve: actor %q has insufficient %q to use ability %q (have %d, need %d)", casterID, lu.Resource, ability.ID, r.GetCurrent(), lu.Cost)
		}
	}

	// Range: Chebyshev distance from the caster's token to each target's
	// token, both required to be in the SAME scene (the caster's own
	// token's scene is "the active scene" for this resolution).
	casterTok, ok := findToken(st, casterID, "")
	if !ok {
		return nil, fmt.Errorf("rules: resolve: actor %q has no token placed (cannot determine range)", casterID)
	}
	for _, tid := range targetIDs {
		tok, ok := findToken(st, tid, casterTok.SceneID)
		if !ok {
			return nil, fmt.Errorf("rules: resolve: target actor %q has no token in the active scene %q (cannot determine range)", tid, casterTok.SceneID)
		}
		if d := chebyshevDistance(casterTok, tok); d > ability.Targeting.Range {
			return nil, fmt.Errorf("rules: resolve: target actor %q is out of range for ability %q (distance %d > range %d)", tid, ability.ID, d, ability.Targeting.Range)
		}
	}

	// --- validation complete; build the ordered event batch ---
	rst := &resolveState{rs: rs, st: st, rng: rng, running: map[resKey]int{}, condOverride: map[string]map[string]bool{}}

	var events []*vttv1.Envelope
	var rolls []*vttv1.AbilityUsed_Roll
	var summaryParts []string

	if ability.Usage.Limited != nil {
		lu := ability.Usage.Limited
		nv := rst.applyDelta(casterID, lu.Resource, -lu.Cost)
		events = append(events, resourceChangedEnvelope(casterID, lu.Resource, -lu.Cost, nv, fmt.Sprintf("ability:%s:usage", ability.ID)))
	}

	for _, tid := range targetIDs {
		if ability.Resolution != nil {
			casterCtx := EvalContext{Attrs: rst.attributesOf(casterID), Resources: rst.resourcesOf(casterID)}
			targetCtx := EvalContext{Attrs: rst.attributesOf(tid), Resources: rst.resourcesOf(tid)}

			total, dice, err := evalRecordingScoped(ability.Resolution.Roll, casterCtx, targetCtx, rng)
			if err != nil {
				return nil, fmt.Errorf("rules: resolve: ability %q resolution roll: %w", ability.ID, err)
			}
			total32, err := int32Checked(total, fmt.Sprintf("ability %q resolution roll total", ability.ID))
			if err != nil {
				return nil, err
			}
			rolls = append(rolls, &vttv1.AbilityUsed_Roll{
				Expression: ability.Resolution.RollSrc,
				Results:    toInt32Slice(dice),
				Total:      total32,
			})

			// Vs is evaluated the SAME way (EvalScoped, fresh per target) but
			// recorded onto AbilityUsed.rolls ONLY when it actually rolled —
			// exactly resource_change's own recording rule (below), NOT the
			// roll's always-recorded rule. This is what keeps a v1-adapted
			// ability's Vs ("@target.<defense>", never containing dice)
			// invisible to testimony, preserving every existing v1 golden.
			vsTotal, vsDice, err := evalRecordingScoped(ability.Resolution.Vs, casterCtx, targetCtx, rng)
			if err != nil {
				return nil, fmt.Errorf("rules: resolve: ability %q resolution vs: %w", ability.ID, err)
			}
			vsTotal32, err := int32Checked(vsTotal, fmt.Sprintf("ability %q resolution vs total", ability.ID))
			if err != nil {
				return nil, err
			}
			if len(vsDice) > 0 {
				rolls = append(rolls, &vttv1.AbilityUsed_Roll{
					Expression: ability.Resolution.VsSrc,
					Results:    toInt32Slice(vsDice),
					Total:      vsTotal32,
				})
			}

			hit := total >= vsTotal
			branchIdx := 1
			if hit {
				branchIdx = 0
			}
			outcomeList := ability.BranchOutcomes[branchIdx]
			word := ability.Resolution.Branches[branchIdx]
			summaryParts = append(summaryParts, fmt.Sprintf("%s on %s: %s (%d vs %d)", ability.Name, tid, word, total, vsTotal))

			evs, evRolls, err := rst.applyOutcomes(casterID, tid, outcomeList, ability.ID, word)
			if err != nil {
				return nil, err
			}
			events = append(events, evs...)
			rolls = append(rolls, evRolls...)
		}

		if len(ability.Effects) > 0 {
			evs, evRolls, err := rst.applyOutcomes(casterID, tid, ability.Effects, ability.ID, "effect")
			if err != nil {
				return nil, err
			}
			events = append(events, evs...)
			rolls = append(rolls, evRolls...)
		}
		if ability.Resolution == nil {
			summaryParts = append(summaryParts, fmt.Sprintf("%s on %s", ability.Name, tid))
		}
	}

	thresholdEvents, err := rst.evalThresholds()
	if err != nil {
		return nil, err
	}
	events = append(events, thresholdEvents...)

	abilityUsed := &vttv1.AbilityUsed{
		ActorId:        casterID,
		AbilityId:      ability.ID,
		TargetIds:      append([]string(nil), targetIDs...),
		Rolls:          rolls,
		OutcomeSummary: strings.Join(summaryParts, "; "),
	}
	out := make([]*vttv1.Envelope, 0, len(events)+1)
	out = append(out, abilityUsedEnvelope(abilityUsed))
	out = append(out, events...)
	return out, nil
}

// resKey identifies one (actor, resource) pair for the running-value and
// changed-order tracking a single Resolve call needs.
type resKey struct{ actor, resource string }

// resolveState accumulates the in-flight, not-yet-persisted effects of one
// Resolve call: resource values as they'd stand after every change so far
// (running, keyed by resKey — engine.State itself is never mutated), which
// (actor, resource) pairs changed and in what order (changedOrder, for the
// threshold phase), and condition presence as it would stand after every
// apply/remove so far (condOverride, layered over st.Conditions the same
// way running is layered over the actors' actual resource values).
type resolveState struct {
	rs  *Ruleset
	st  *engine.State
	rng Roller

	running      map[resKey]int
	changedOrder []resKey

	condOverride map[string]map[string]bool
}

func (r *resolveState) currentResource(actorID, name string) int {
	if v, ok := r.running[resKey{actorID, name}]; ok {
		return v
	}
	return int(r.st.Actors[actorID].GetResources()[name].GetCurrent())
}

func (r *resolveState) maxResource(actorID, name string) int {
	return int(r.st.Actors[actorID].GetResources()[name].GetMax())
}

// applyDelta computes the post-clamp value for actorID's named resource
// (floor 0, cap at max when max > 0 — EXACTLY engine.Apply's ResourceChanged
// clamp, so the emitted new_value always matches what the engine will
// independently recompute), records the (actor, resource) pair the first
// time it's touched (changedOrder, for the threshold phase), and returns
// the new running value.
func (r *resolveState) applyDelta(actorID, name string, delta int) int {
	nv := r.currentResource(actorID, name) + delta
	if nv < 0 {
		nv = 0
	}
	if upper := r.maxResource(actorID, name); upper > 0 && nv > upper {
		nv = upper
	}
	key := resKey{actorID, name}
	if _, seen := r.running[key]; !seen {
		r.changedOrder = append(r.changedOrder, key)
	}
	r.running[key] = nv
	return nv
}

// attributesOf returns actorID's attributes as an int map for Eval — never
// changed during Resolve (no outcome kind modifies attributes), so this is
// always the actor's actual state.
func (r *resolveState) attributesOf(actorID string) map[string]int {
	a := r.st.Actors[actorID].GetAttributes()
	out := make(map[string]int, len(a))
	for k, v := range a {
		out[k] = int(v)
	}
	return out
}

// resourcesOf returns actorID's CURRENT resource values for Eval — every
// resource the actor declares, using the running (post-earlier-changes-
// this-resolve) value when one exists. A resource the actor does not carry
// at all is simply absent from this map, so an expression referencing it
// gets Eval's own "unknown resource" error (the lazy-validation path).
func (r *resolveState) resourcesOf(actorID string) map[string]int {
	res := r.st.Actors[actorID].GetResources()
	out := make(map[string]int, len(res))
	for name := range res {
		out[name] = r.currentResource(actorID, name)
	}
	return out
}

func (r *resolveState) hasCondition(actorID, condID string) bool {
	if m, ok := r.condOverride[actorID]; ok {
		if v, ok2 := m[condID]; ok2 {
			return v
		}
	}
	for _, c := range r.st.Conditions[actorID] {
		if c.ID == condID {
			return true
		}
	}
	return false
}

func (r *resolveState) setCondition(actorID, condID string, present bool) {
	if r.condOverride[actorID] == nil {
		r.condOverride[actorID] = map[string]bool{}
	}
	r.condOverride[actorID][condID] = present
}

// applyOutcomes runs one outcome list (a compiled power's hit/connect/
// pass-branch, miss/graze/fail-branch, or effect list) against actorID —
// the TARGET named in the per-target slot Resolve is currently processing;
// outcomes have no actor-selection field of their own (format.go), so the
// TARGET of every outcome in the list is whichever target Resolve is
// looping over when it calls this. casterID is threaded through
// separately (Task 3): a resource_change's DeltaExpr is a two-actor
// expression (EvalScoped) — a v1-adapted outcome is always target-scoped
// (v1's historical single-actor evaluation, preserved), but a v2 outcome
// may legitimately reference the CASTER's state too (e.g.
// "0 - (@caster.vigor + 3)", proving_grounds' clash-damage atom), so both
// contexts must be live regardless of format version. Returns the ordered
// events the list produced (possibly none — apply/remove on an
// already-in-that-state condition is a deliberate no-op, see Resolve's doc
// comment) and any Roll entries recorded along the way.
func (r *resolveState) applyOutcomes(casterID, actorID string, outcomes []Outcome, abilityID, phase string) ([]*vttv1.Envelope, []*vttv1.AbilityUsed_Roll, error) {
	var events []*vttv1.Envelope
	var rolls []*vttv1.AbilityUsed_Roll
	reason := fmt.Sprintf("ability:%s:%s", abilityID, phase)

	for _, o := range outcomes {
		switch o.Kind {
		case OutcomeResourceChange:
			rc := o.ResourceChange
			if _, ok := r.st.Actors[actorID].GetResources()[rc.Resource]; !ok {
				return nil, nil, fmt.Errorf("rules: resolve: actor %q has no resource %q required by ability %q", actorID, rc.Resource, abilityID)
			}
			casterCtx := EvalContext{Attrs: r.attributesOf(casterID), Resources: r.resourcesOf(casterID)}
			targetCtx := EvalContext{Attrs: r.attributesOf(actorID), Resources: r.resourcesOf(actorID)}
			delta, dice, err := evalRecordingScoped(rc.DeltaExpr, casterCtx, targetCtx, r.rng)
			if err != nil {
				return nil, nil, fmt.Errorf("rules: resolve: ability %q resource_change on %q: %w", abilityID, rc.Resource, err)
			}
			delta32, err := int32Checked(delta, fmt.Sprintf("ability %q resource_change on %q delta", abilityID, rc.Resource))
			if err != nil {
				return nil, nil, err
			}
			if len(dice) > 0 {
				rolls = append(rolls, &vttv1.AbilityUsed_Roll{Expression: rc.DeltaExprSrc, Results: toInt32Slice(dice), Total: delta32})
			}
			nv := r.applyDelta(actorID, rc.Resource, delta)
			if _, err := int32Checked(nv, fmt.Sprintf("ability %q resource_change on %q new_value", abilityID, rc.Resource)); err != nil {
				return nil, nil, err
			}
			events = append(events, resourceChangedEnvelope(actorID, rc.Resource, delta, nv, reason))

		case OutcomeApplyCondition:
			id := o.ApplyCondition.ID
			if !r.hasCondition(actorID, id) {
				events = append(events, conditionAppliedEnvelope(actorID, id, reason))
				r.setCondition(actorID, id, true)
			}

		case OutcomeRemoveCondition:
			id := o.RemoveCondition.ID
			if r.hasCondition(actorID, id) {
				events = append(events, conditionRemovedEnvelope(actorID, id, reason))
				r.setCondition(actorID, id, false)
			}
		}
	}
	return events, rolls, nil
}

// evalThresholds evaluates every (actor, resource) pair that changed value
// during this Resolve call — in the order those pairs first changed — for
// each pair, walking that resource's declared thresholds in manifest
// declaration order (spec §5): apply_condition when "when" is non-zero and
// the condition isn't already present; if remove_when_false is set, remove
// the condition when "when" is zero and it IS present. Not exercised for
// pairs that didn't change this Resolve call — thresholds fire as a
// reaction to a change, not a periodic sweep over all state.
func (r *resolveState) evalThresholds() ([]*vttv1.Envelope, error) {
	byName := make(map[string]*ResourceDef, len(r.rs.Resources))
	for i := range r.rs.Resources {
		byName[r.rs.Resources[i].Name] = &r.rs.Resources[i]
	}

	var events []*vttv1.Envelope
	for _, key := range r.changedOrder {
		def, ok := byName[key.resource]
		if !ok {
			continue // resource names are cross-validated at Load; defensive only
		}
		attrs := r.attributesOf(key.actor)
		resources := r.resourcesOf(key.actor)
		reason := fmt.Sprintf("threshold:%s", key.resource)

		for _, th := range def.Thresholds {
			v, err := Eval(th.When, attrs, resources, r.rng)
			if err != nil {
				return nil, fmt.Errorf("rules: resolve: threshold %q on resource %q: %w", th.WhenSrc, key.resource, err)
			}
			switch {
			case v != 0:
				if !r.hasCondition(key.actor, th.ApplyCondition) {
					events = append(events, conditionAppliedEnvelope(key.actor, th.ApplyCondition, reason))
					r.setCondition(key.actor, th.ApplyCondition, true)
				}
			case th.RemoveWhenFalse:
				if r.hasCondition(key.actor, th.ApplyCondition) {
					events = append(events, conditionRemovedEnvelope(key.actor, th.ApplyCondition, reason))
					r.setCondition(key.actor, th.ApplyCondition, false)
				}
			}
		}
	}
	return events, nil
}

// --- dice-recording Eval wrapper ---

// recordingRoller wraps a caller-supplied Roller, accumulating every
// individual die result rolled across possibly-multiple Roll calls made
// while evaluating ONE expression — an expression can contain more than one
// DICE node (e.g. "1d8 + 1d6"), and AbilityUsed.rolls records one entry per
// EXPRESSION (attack roll, or resource_change delta_expr), not one per DICE
// node.
type recordingRoller struct {
	inner   Roller
	results []int
}

func (rr *recordingRoller) Roll(n, sides int) ([]int, int) {
	res, total := rr.inner.Roll(n, sides)
	rr.results = append(rr.results, res...)
	return res, total
}

// evalRecordingScoped evaluates e (via EvalScoped — Task 3: Resolve
// executes every ability, v1-adapted or v2-native, through EvalScoped
// exclusively, so this is the ONLY dice-recording eval wrapper Resolve
// uses now) and also returns every individual die result rolled while
// doing so (nil if e contains no DICE node, or roller is nil — a nil
// roller is passed straight to EvalScoped unwrapped, so EvalScoped's own
// "dice requires a non-nil Roller" error still fires cleanly instead of a
// nil-pointer panic inside recordingRoller). Used for every expression a
// CompiledPower carries: Resolution.Roll (always recorded), Resolution.Vs
// and every resource_change DeltaExpr (recorded only when dice actually
// rolled — see Resolve's doc comment).
func evalRecordingScoped(e *Expr, caster, target EvalContext, roller Roller) (int, []int, error) {
	if roller == nil {
		total, err := EvalScoped(e, caster, target, nil)
		return total, nil, err
	}
	rr := &recordingRoller{inner: roller}
	total, err := EvalScoped(e, caster, target, rr)
	if err != nil {
		return 0, nil, err
	}
	return total, rr.results, nil
}

func toInt32Slice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		// #nosec G115 -- dice results only. The grammar bounds dice at
		// 1..100 count x 1..1000 sides (expr.go), so every element is far
		// inside int32.
		out[i] = int32(v)
	}
	return out
}

// int32Checked converts v to int32, rejecting anything outside the int32 wire
// range with a clean rules error rather than silently truncating. Resolve
// computes resource math and roll totals in 64-bit int, but the contract's
// ResourceChanged.delta/new_value and AbilityUsed_Roll.total fields are int32:
// a legal, loadable expression (the grammar's closed '*' has no magnitude
// bound beyond the int-literal fit) can evaluate past int32, and truncating
// it would either poison Resolve↔engine parity (the engine independently
// recomputes the clamp and rejects the mismatch) or, in the mod-2^32 corner,
// write false Delta testimony into the append-only log. Bounding here keeps
// every emitted value honest — and keeps the pairing with engine.Apply's
// int64 clamp exact.
//
// The usage-spend ResourceChanged is deliberately NOT routed through here:
// its delta is -cost and its new_value is current-cost, and Resolve's own
// insufficient-resource guard already proved cost <= current, where current
// is an int32 — so both provably fit int32 and cannot overflow.
func int32Checked(v int, what string) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("rules: resolve: %s value %d is outside the int32 wire range [%d, %d]", what, v, math.MinInt32, math.MaxInt32)
	}
	return int32(v), nil
}

// --- envelope builders (payload only — see Resolve's doc comment) ---

func abilityUsedEnvelope(au *vttv1.AbilityUsed) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityUsed{AbilityUsed: au}}
}

func resourceChangedEnvelope(actorID, resource string, delta, newValue int, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		// #nosec G115 -- callers guarantee int32 range, and the two paths do
		// it differently: the outcome path routes both values through
		// int32Checked immediately before calling (resolve.go:473,481); the
		// usage-spend path is deliberately exempt because cost <= current is
		// already proven by the insufficient-resource guard and current is an
		// int32 (see int32Checked's doc comment).
		ActorId: actorID, Resource: resource, Delta: int32(delta), NewValue: int32(newValue), Reason: reason,
	}}}
}

func conditionAppliedEnvelope(actorID, conditionID, source string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actorID, ConditionId: conditionID, Source: source,
	}}}
}

func conditionRemovedEnvelope(actorID, conditionID, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
		ActorId: actorID, ConditionId: conditionID, Reason: reason,
	}}}
}

// --- range geometry (generic grid Chebyshev distance) ---

// findToken returns the token placed for actorID — filtered to sceneID
// when sceneID is non-empty, or searched across every scene when it's
// empty (used to establish the caster's OWN "active scene" in the first
// place). If more than one token happens to reference the same actor (the
// engine does not forbid this), the LOWEST token id wins — a deterministic,
// arbitrary tie-break so range resolution never depends on Go's map
// iteration order.
func findToken(st *engine.State, actorID, sceneID string) (engine.Token, bool) {
	var ids []string
	for id, tok := range st.Tokens {
		if tok.ActorID != actorID {
			continue
		}
		if sceneID != "" && tok.SceneID != sceneID {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return engine.Token{}, false
	}
	sort.Strings(ids)
	return st.Tokens[ids[0]], true
}

func chebyshevDistance(a, b engine.Token) int {
	dx := int(a.X) - int(b.X)
	if dx < 0 {
		dx = -dx
	}
	dy := int(a.Y) - int(b.Y)
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
