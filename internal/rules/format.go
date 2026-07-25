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
	Abilities  map[string]*Ability
	Conditions map[string]*Condition

	// Guide is the raw contents of guide.md — LLM affordances served
	// verbatim (spec §4), not parsed or validated by this package.
	Guide string
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
