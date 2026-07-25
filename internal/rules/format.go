// Package rules is the ONE generic interpreter for ruleset data (spec
// pillar P2: rules as declarative data). It knows resources, abilities,
// conditions, defenses, and attributes only as generic named concepts — no
// game-system word (a specific stat name, a specific condition name)
// appears anywhere in this package; those live exclusively in
// rulesets/*/*.json content, loaded at runtime by Load.
//
// A Ruleset is a directory (rulesets/<id>/) of JSON files plus a guide.md;
// Load reads, strictly decodes, and fully validates one (schemas,
// cross-references, and every expression's grammar — see load.go). Resolve
// (Task 5) is the pure function that executes a UseAbility command against
// a loaded Ruleset and an engine.State snapshot.
package rules

// Ruleset is one fully-loaded, fully-validated rules library.
type Ruleset struct {
	ID            string
	Name          string
	FormatVersion string

	Resources  []ResourceDef
	Defenses   []string
	Attributes []string

	// Abilities and Conditions are keyed by their declared id. Load
	// rejects duplicate ids (within and across files) before either map
	// is populated, so every entry here is unique by construction.
	// Abilities is non-empty for a format-v1-loaded Ruleset (v1's
	// abilities/*.json decode directly into this shape) and a non-nil,
	// EMPTY map for a format-v2-loaded Ruleset (v2's abilities/*.json are
	// compositions, not v1 Ability-shaped — see Compiled). Resolve (Task
	// 5, rewired Task 3) never reads Abilities: it executes Compiled
	// exclusively, for every format version — see Compiled's doc comment.
	Abilities  map[string]*Ability
	Conditions map[string]*Condition

	// Guide is the raw contents of guide.md — LLM affordances served
	// verbatim (spec §4), not parsed or validated by this package.
	Guide string

	// Atoms holds every format-v2 ruleset-authored atom, keyed by its
	// declared id (spec §4, sub-project 5c). Compile-time input only —
	// Resolve (Task 5) never sees atoms, it executes Compiled powers
	// exclusively (spec §6). nil for a format-v1-loaded Ruleset. Exposed
	// for conformance/debug introspection (spec §6's "/power-debug"
	// lesson): the atoms a power was flattened FROM, alongside the
	// flattened form itself in Compiled.
	Atoms map[string]*AtomDef

	// Compiled holds every ability's flattened execution form, keyed by
	// ability id — populated by Load itself, for EVERY format version
	// (Task 3, spec pillar "Resolve executes CompiledPower through ONE
	// code path"): a v2 composition compiles via compile.go's atom-graph
	// flattening (spec §6); a v1 ability adapts via load.go's
	// AdaptV1Ability, a byte-identical-behavior translation of v1's
	// {attack roll+vs-defense-name, hit/miss/effect lists} into the exact
	// same CompiledPower shape. Either way, Resolve (Task 5, rewired Task
	// 3) reads Compiled ONLY — it never branches on FormatVersion and
	// never sees an Ability or an atom graph directly. Always non-nil
	// after a successful Load; the sunset removal of Abilities/the v1
	// code path entirely is Task 4.
	Compiled map[string]*CompiledPower
}

// AtomDef is one ruleset-authored atomic statement (format v2, spec §4): a
// named, parameterized fragment of an ability's execution, contributing
// targeting/resolution/outcome pieces to whatever composition(s) include
// it. The SAME atom may be composed by many abilities, each supplying its
// own param bindings — an atom's Contributes fields are therefore raw
// templates (may contain "{param}" placeholders), never a parsed *Expr;
// only a composition's compile step (compile.go), which has concrete
// bindings, produces parsed/evaluable expressions. See Contribution.
type AtomDef struct {
	ID          string
	Params      []ParamDef
	Provides    []string
	Consumes    []string
	Contributes []Contribution

	// sourcePath is the atoms/*.json file this atom was decoded from, kept
	// for the same reason Ability.sourcePath is: cross-reference and
	// compile errors discovered after decoding name the real file.
	sourcePath string
}

// ParamDef declares one parameter an atom's Contributes templates may
// reference via a "{name}" placeholder. Kind is one of the six closed
// param kinds (spec §4):
//
//   - "int", "expr" — VALUE kinds. A binding splices into a contribution
//     expression as a parenthesized parsed subtree (hygienic: the binding
//     can never change the surrounding expression's precedence — spec §2
//     guarantee 4, spec §4). Legal at expression VALUE positions, e.g.
//     "1d20 + {prof}".
//   - "attribute", "resource", "defense", "condition" — NAME kinds. A
//     binding substitutes as a bare identifier into a ref or name
//     position — never parenthesized, since the result is never itself a
//     sub-expression. Legal inside a ref's name segment, e.g.
//     "@caster.{attack_stat}", or a plain name field, e.g. a
//     resource_change's "resource": "{pool}".
//
// Bindings are validated against the manifest's declared vocabulary at
// compile time (attribute/resource/defense against the ruleset's declared
// names; condition against declared condition ids).
type ParamDef struct {
	Name string
	Kind string
}

// Contribution is one piece an atom contributes to its composition's
// flattened execution form (spec §4). Only the fields belonging to Kind
// are populated (mirrors Outcome/Usage's oneof-by-Kind pattern elsewhere
// in this package). All *Src string fields are RAW template text — they
// may contain "{param}" placeholders and are not parsed until a
// composition's compile step substitutes concrete bindings (compile.go).
type Contribution struct {
	// Kind is "targeting", "resolution", or "outcome" (spec §4's closed
	// contribution-kind vocabulary).
	Kind string

	// --- Kind == "targeting" (exactly one required per composition) ---
	//
	// RangeSrc/MaxTargetsSrc are each either the decimal text of a
	// literal integer, or a whole-field "{param}" placeholder naming an
	// int-kind param — targeting is compiled into v1's plain
	// Targeting{Range,MaxTargets int}, a compile-time constant, never a
	// runtime expression.
	RangeSrc      string
	MaxTargetsSrc string

	// --- Kind == "resolution" (at most one per composition, spec §4) ---
	//
	// Key is the provided/consumed graph key this resolution is filed
	// under (normally one of the owning atom's own Provides entries).
	// RollSrc/VsSrc are two-actor-position expression templates. Branches
	// is [ge-label, lt-label]: the roll total >= Vs selects Branches[0],
	// else Branches[1].
	Key      string
	RollSrc  string
	VsSrc    string
	Branches [2]string

	// --- Kind == "outcome" ---
	//
	// OutcomeKey names the resolution (by its Key) this outcome is
	// conditioned on; nil exactly when Branch == "always" (an
	// unconditional effect-phase outcome — spec §4). Branch is either
	// "always" or one of that resolution's two Branches labels. Effects
	// is v1's outcome-list shape, as raw templates.
	OutcomeKey *string
	Branch     string
	Effects    []EffectTemplate
}

// EffectTemplate is one atom-contributed outcome effect, in raw
// (unsubstituted) template form — the atom-composition analogue of v1's
// Outcome/ResourceChangeOutcome/ApplyConditionOutcome/
// RemoveConditionOutcome, before a composition's concrete param bindings
// turn it into a real Outcome. Exactly one of the three template groups is
// populated, per Kind — matching OutcomeKind's three cases.
type EffectTemplate struct {
	Kind OutcomeKind

	// Kind == OutcomeResourceChange
	ResourceSrc  string // name field; NAME-kind placeholder substitution only
	DeltaExprSrc string // two-actor-position expression template

	// Kind == OutcomeApplyCondition
	ApplyConditionSrc string // name field; NAME-kind placeholder substitution only

	// Kind == OutcomeRemoveCondition
	RemoveConditionSrc string // name field; NAME-kind placeholder substitution only
}

// CompiledPower is one composition ability flattened to the small
// declarative execution form 5a proved (spec §6): the same shape
// Resolve (Task 5) already knows how to run, plus branch labels and a
// vs-expression. Resolve executes CompiledPowers ONLY — it never sees
// AtomDef/Contribution.
type CompiledPower struct {
	ID, Name string

	Usage     Usage     // v1 type, unchanged
	Targeting Targeting // v1 type, unchanged

	// Resolution is nil for a non-attack composition (no resolution
	// contribution at all — a composition needs no resolution atom to be
	// valid, spec §4).
	Resolution *CompiledResolution

	// BranchOutcomes is aligned with Resolution.Branches:
	// BranchOutcomes[0] runs when the roll selects Branches[0],
	// BranchOutcomes[1] when it selects Branches[1]. Zero-valued (both
	// nil) when Resolution is nil.
	//
	// MERGE ORDER (when more than one outcome contribution targets the
	// SAME branch, e.g. two independent atoms both reacting to a "hit"):
	// entries appear in the graph's topological execution order (compile.
	// go's buildAndSortDAG) — the same order every other cross-atom effect
	// in this composition runs in. Two atoms that are mutual non-
	// dependents (neither provides a key the other consumes — an
	// in-degree-0 TIE at the point both become ready) are ordered by their
	// position in the composition's own "compose" list, lower index
	// first — deterministic, but per spec §4 guarantee 4 never load-
	// bearing for CORRECTNESS, only for reproducibility (goldens, replay).
	// TestLoadValidV2Fixture's "tag-team" case and TestCompileDeterministic
	// (compile_test.go) both exercise this exact tie shape.
	BranchOutcomes [2][]Outcome

	// Effects are the composition's unconditional ("always") outcomes —
	// always run, attack or not.
	Effects []Outcome
}

// CompiledResolution is a composition's single resolution contribution,
// flattened: Roll and Vs are fully substituted, parsed, two-actor-position
// expressions (spec §7: evaluated via EvalScoped); Branches carries the
// [ge-label, lt-label] pair verbatim into testimony (AbilityUsed.
// outcome_summary speaks the ruleset's own words).
//
// RollSrc/VsSrc are the DISPLAY text Resolve records onto
// AbilityUsed.rolls[].expression when Roll/Vs actually rolls dice (Task 3,
// spec pillar "ONE execution path" — the v1-to-CompiledPower adapter and
// the v2 compiler both populate a CompiledPower, but from different
// "source of truth" text): for a v2 composition (compile.go), RollSrc/
// VsSrc are the fully-substituted, POST-SPLICE source text Roll/Vs were
// parsed from — exactly what Roll.String()/Vs.String() would already
// return, kept as an explicit field so testimony code never has to know
// whether it's holding a v1-adapted or v2-compiled power. For a v1-adapted
// ability (load.go's AdaptV1Ability), RollSrc is v1's ORIGINAL, UNSCOPED
// Attack.RollSrc text — Roll itself is a DIFFERENT (caster-scoped) parse
// of a scoped rewrite of that same text, needed to execute through
// EvalScoped, but the adapter must never let that rewritten text leak into
// a v1 ability's already-committed golden testimony. VsSrc has no true v1
// "original" (v1's Attack.Vs was a bare defense NAME, never an
// expression) — it is set to the synthesized "@target.<defense>" text,
// which is only ever OBSERVABLE if evaluating it rolls dice; a bare
// attribute ref never does, so no v1 golden ever displays it. See
// AdaptV1Ability's doc comment for the full "SourceText decision"
// rationale.
type CompiledResolution struct {
	Roll, Vs       *Expr
	RollSrc, VsSrc string
	Branches       [2]string
}

// ResourceDef declares one named resource pool a ruleset's actors may
// have (e.g. a limited-use pool that abilities spend from). DefaultMaxExpr
// is nil when the manifest omits default_max_expr.
type ResourceDef struct {
	Name              string
	DefaultMaxExpr    *Expr
	DefaultMaxExprSrc string
	Thresholds        []Threshold
}

// Threshold fires apply_condition when When evaluates non-zero (true) and
// the actor's Conditions doesn't already carry it; when RemoveWhenFalse is
// set, it also removes apply_condition once When evaluates zero (false).
// Evaluated by Resolve (Task 5) after a resource's value changes, against
// the owning actor's attrs/resources via Eval — When has no comparison
// operators of its own (the grammar is arithmetic-only, see expr.go); a
// ruleset expresses "at or below" thresholds by choosing an expression
// that itself evaluates to zero exactly at the boundary.
type Threshold struct {
	When            *Expr
	WhenSrc         string
	ApplyCondition  string
	RemoveWhenFalse bool
}

// Usage is a oneof: exactly one of AtWill or Limited is set. JSON encodes
// this as either the bare string "at_will" or an object {"limited": {...}}
// — see (*Usage).UnmarshalJSON in load.go.
type Usage struct {
	AtWill  bool
	Limited *LimitedUsage
}

// LimitedUsage names the resource an ability's use spends, and how much.
type LimitedUsage struct {
	Resource string
	Cost     int
}

// Targeting bounds how an ability may be aimed: Range in grid cells
// (Chebyshev distance, resolved by Task 5) and MaxTargets, the largest
// number of targets one use may name.
type Targeting struct {
	Range      int
	MaxTargets int
}

// Attack is present only on abilities that roll against a defense. Roll
// produces the attack roll total; Vs names which of the ruleset's declared
// defenses it's checked against.
type Attack struct {
	Roll    *Expr
	RollSrc string
	Vs      string
}

// OutcomeKind tags which of Outcome's three fields is populated.
type OutcomeKind int

const (
	OutcomeResourceChange OutcomeKind = iota
	OutcomeApplyCondition
	OutcomeRemoveCondition
)

// Outcome is one step of an ability's hit/miss/effect list: exactly one of
// ResourceChange, ApplyCondition, or RemoveCondition is set, matching Kind.
type Outcome struct {
	Kind            OutcomeKind
	ResourceChange  *ResourceChangeOutcome
	ApplyCondition  *ApplyConditionOutcome
	RemoveCondition *RemoveConditionOutcome
}

// ResourceChangeOutcome adjusts a named resource by DeltaExpr (which may be
// negative, e.g. a cost, or positive, e.g. a gain).
type ResourceChangeOutcome struct {
	Resource     string
	DeltaExpr    *Expr
	DeltaExprSrc string
}

// ApplyConditionOutcome names a condition to apply to the outcome's target.
type ApplyConditionOutcome struct {
	ID string
}

// RemoveConditionOutcome names a condition to remove from the outcome's
// target.
type RemoveConditionOutcome struct {
	ID string
}

// Ability is one usable ability: a targeting envelope, an optional attack
// roll, and up to three outcome lists. Attack == nil marks a non-attack
// ability — Hit and Miss must then both be empty (Load rejects otherwise);
// Effect always runs unconditionally.
type Ability struct {
	ID        string
	Name      string
	Usage     Usage
	Targeting Targeting
	Attack    *Attack
	Hit       []Outcome
	Miss      []Outcome
	Effect    []Outcome

	// sourcePath is the abilities/*.json file this ability was decoded
	// from, kept so cross-reference errors discovered after decoding (in
	// crossValidate) name the real file rather than guessing one from the
	// ability id.
	sourcePath string
}

// Condition is a named marker a ruleset's abilities can apply or remove.
// v1 conditions carry no mechanical effect of their own (engine.
// ActorCondition doc, ruleset-interpreter spec §4) — they are DM-narrated
// bookkeeping the platform tracks structurally only.
type Condition struct {
	ID          string
	Name        string
	Description string
}
