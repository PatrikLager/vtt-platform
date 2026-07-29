package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// supportedFormatVersions is the set of format_version values Load
// accepts. Task 4 (v1 sunset, spec §9's migration): format v1 is retired —
// "2" (spec 2026-07-25-format-v2-composition-design.md §3 — atoms/
// compositions, compiled at load into the small declarative execution
// shape 5a proved; see loadV2 and compile.go) is the only value Load will
// ever load. "1" is a clean break, not a silent guess: both rulesets that
// existed under it (rulesets/tavern-brawl, internal/rules/testdata/valid)
// migrated to v2 in this task, and spec §3 states plainly there are no
// external v1 authors yet to protect.
var supportedFormatVersions = map[string]bool{"2": true}

// formatVersion1RejectedMsg is the clear, spec-naming error Load produces
// for format_version "1" (or any other unsupported value) — spec §3: "clean
// break... both existing rulesets are in-repo and migrate". Named as its
// own constant (rather than inlined into loadManifest's fieldErr call) so
// the exact wording load_test.go's rejection test pins is visibly the same
// text a ruleset author would see, not a paraphrase.
const formatVersion1RejectedMsg = "format v1 is retired (clean break, no external v1 authors — see docs/superpowers/specs/2026-07-25-format-v2-composition-design.md §3/§9); this ruleset must declare \"format_version\": \"2\" and be authored as atoms/compositions (§4)"

// Load reads and fully validates the ruleset directory at dir: strict JSON
// decoding of ruleset.json/conditions/*.json/atoms/*.json/
// abilities/*.json-as-compositions (no unknown fields tolerated), the
// grammar of every expression (parsed once, here, so a loaded Ruleset
// never fails to parse an expression at runtime), every cross-reference a
// resource, threshold, atom, or composition makes (attributes, resources,
// defenses, and condition ids must all be declared — spec §5), and —
// compile-at-load (spec §6) — every composition ability flattened into
// Ruleset.Compiled by compile.go's compileCompositions. Ruleset.Compiled is
// always populated on a successful Load, and Resolve reads Compiled
// exclusively — spec 5c pillar "ONE execution path". Every error names the
// offending file and field.
//
// format_version "1" (spec §3/§9, Task 4's sunset): REJECTED, by
// loadManifest, before this function ever reads conditions/atoms/
// abilities — supportedFormatVersions holds only "2", so manifest.
// FormatVersion is always "2" by the time control reaches here. Both
// rulesets that ever shipped as v1 (rulesets/tavern-brawl,
// internal/rules/testdata/valid) migrated to v2 atoms/compositions in this
// task; the entire v1 file-decode/cross-validate/adapt machinery that used
// to run here (loadAbilities, crossValidate, adaptV1Abilities, and the
// AdaptV1Ability adapter itself) had no caller left and was removed as dead
// code. Resolve executes the compiled form for every ability; hand-built
// test fixtures construct CompiledPower values directly.
func Load(dir string) (*Ruleset, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("rules: load %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("rules: load %s: not a directory", dir)
	}

	manifest, err := loadManifest(filepath.Join(dir, "ruleset.json"))
	if err != nil {
		return nil, err
	}

	return loadV2(dir, manifest)
}

// loadV2 is the Load path (spec §3, §6): manifest plus conditions/
// (unchanged shape), atoms/, and abilities/-as-compositions, compiled into
// Ruleset.Compiled. Resolve reads Compiled only; v2's abilities/*.json files
// are compositions (decoded into a distinct internal type by
// loadCompositions and consumed only by compile.go), never a standalone
// ability shape.
func loadV2(dir string, manifest *loadedManifest) (*Ruleset, error) {
	conditions, err := loadConditions(filepath.Join(dir, "conditions"))
	if err != nil {
		return nil, err
	}

	atoms, err := loadAtoms(filepath.Join(dir, "atoms"))
	if err != nil {
		return nil, err
	}

	compositions, err := loadCompositions(filepath.Join(dir, "abilities"))
	if err != nil {
		return nil, err
	}

	guide, err := os.ReadFile(filepath.Join(dir, "guide.md"))
	if err != nil {
		return nil, fmt.Errorf("rules: load %s: guide.md: %w", dir, err)
	}

	rs := &Ruleset{
		ID:            manifest.ID,
		Name:          manifest.Name,
		FormatVersion: manifest.FormatVersion,
		Resources:     manifest.resources,
		Defenses:      manifest.Defenses,
		Attributes:    manifest.Attributes,
		Conditions:    conditions,
		Atoms:         atoms,
		Guide:         string(guide),
	}

	if err := crossValidateResources(rs); err != nil {
		return nil, err
	}

	compiled, err := compileCompositions(rs, atoms, compositions)
	if err != nil {
		return nil, err
	}
	rs.Compiled = compiled
	return rs, nil
}

// --- ruleset.json ---

type manifestJSON struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	FormatVersion string         `json:"format_version"`
	Attributes    []string       `json:"attributes"`
	Defenses      []string       `json:"defenses"`
	Resources     []resourceJSON `json:"resources"`
}

type resourceJSON struct {
	Name           string          `json:"name"`
	DefaultMaxExpr *string         `json:"default_max_expr"`
	Thresholds     []thresholdJSON `json:"thresholds"`
}

type thresholdJSON struct {
	When           string `json:"when"`
	ApplyCondition string `json:"apply_condition"`
	// RemoveWhenFalse is a *bool, not bool: a plain bool can't distinguish
	// "author wrote false" from "author omitted the key" — both decode to
	// Go's zero value. The field is REQUIRED (spec §4 lists it without a
	// "?"), so absence must be a load error, not a silent false.
	RemoveWhenFalse *bool `json:"remove_when_false"`
}

// loadedManifest carries both the raw decoded manifest and its resources
// already converted to the public ResourceDef shape (expressions parsed),
// since manifestJSON.resources needs its exprs parsed before
// crossValidateResources can walk them for Refs().
type loadedManifest struct {
	ID            string
	Name          string
	FormatVersion string
	Attributes    []string
	Defenses      []string
	resources     []ResourceDef
}

// Complexity: straight-line validation of one manifest — a long sequence of
// independent field checks, each with its own error. Splitting it would only
// move the branches somewhere else.
//
//nolint:gocyclo
func loadManifest(path string) (*loadedManifest, error) {
	var raw manifestJSON
	if err := decodeStrict(path, &raw); err != nil {
		return nil, err
	}

	if raw.ID == "" {
		return nil, fieldErr(path, "id", "must not be empty")
	}
	if raw.Name == "" {
		return nil, fieldErr(path, "name", "must not be empty")
	}
	if !supportedFormatVersions[raw.FormatVersion] {
		return nil, fieldErr(path, "format_version", fmt.Sprintf("unsupported value %q — %s", raw.FormatVersion, formatVersion1RejectedMsg))
	}

	seenAttr := map[string]bool{}
	for _, a := range raw.Attributes {
		if a == "" {
			return nil, fieldErr(path, "attributes", "must not contain an empty name")
		}
		if !isValidIdentName(a) {
			return nil, fieldErr(path, "attributes", fmt.Sprintf("attribute name %q must match the expression IDENT charset ^[A-Za-z_][A-Za-z0-9_]*$ (it is referenced as '@'+name inside expressions)", a))
		}
		if reservedScopeWords[a] {
			return nil, fieldErr(path, "attributes", fmt.Sprintf("attribute name %q is a reserved actor scope word (caster/target): format v2 refs it as \"@caster.%s\"/\"@target.%s\", so declaring it as a name would be ambiguous (finding F2)", a, a, a))
		}
		if seenAttr[a] {
			return nil, fieldErr(path, "attributes", fmt.Sprintf("duplicate attribute name %q", a))
		}
		seenAttr[a] = true
	}
	seenDef := map[string]bool{}
	for _, d := range raw.Defenses {
		if d == "" {
			return nil, fieldErr(path, "defenses", "must not contain an empty name")
		}
		if !isValidIdentName(d) {
			return nil, fieldErr(path, "defenses", fmt.Sprintf("defense name %q must match the expression IDENT charset ^[A-Za-z_][A-Za-z0-9_]*$ (kept consistent with attributes/resources)", d))
		}
		if reservedScopeWords[d] {
			return nil, fieldErr(path, "defenses", fmt.Sprintf("defense name %q is a reserved actor scope word (caster/target): format v2 refs it through the same '@'-scope namespace as attributes, so declaring it as a name would be ambiguous (finding F2)", d))
		}
		if seenDef[d] {
			return nil, fieldErr(path, "defenses", fmt.Sprintf("duplicate defense name %q", d))
		}
		// A name declared as BOTH an attribute and a defense is rejected
		// here rather than left to cause trouble later: format v2's
		// two-actor expressions expose defense values through the SAME
		// '@' ref namespace attributes use (compile.go's attrOrDefSet —
		// forced by Task 1's EvalContext having only Attrs/Resources, no
		// separate defense map), so a collision would make "@caster.x" (or
		// "@target.x") genuinely ambiguous — which value does it read?
		// Checked here (loadManifest, shared by both format versions) so
		// it also protects a v1 ruleset from authoring the same latent
		// ambiguity, even though v1 has no union namespace of its own to
		// immediately misbehave from.
		if seenAttr[d] {
			return nil, fieldErr(path, "defenses", fmt.Sprintf("name %q is declared as both an attribute and a defense — a two-actor expression's \"@caster.%s\"/\"@target.%s\" would be ambiguous (format v2 exposes defenses through the same '@' ref namespace as attributes); use distinct names", d, d, d))
		}
		seenDef[d] = true
	}

	seenRes := map[string]bool{}
	resources := make([]ResourceDef, 0, len(raw.Resources))
	for i, r := range raw.Resources {
		field := fmt.Sprintf("resources[%d]", i)
		if r.Name == "" {
			return nil, fieldErr(path, field+".name", "must not be empty")
		}
		if !isValidIdentName(r.Name) {
			return nil, fieldErr(path, field+".name", fmt.Sprintf("resource name %q must match the expression IDENT charset ^[A-Za-z_][A-Za-z0-9_]*$ (it is referenced as '#'+name inside expressions)", r.Name))
		}
		if reservedScopeWords[r.Name] {
			return nil, fieldErr(path, field+".name", fmt.Sprintf("resource name %q is a reserved actor scope word (caster/target): format v2 refs it as \"#caster.%s\"/\"#target.%s\", so declaring it as a name would be ambiguous (finding F2)", r.Name, r.Name, r.Name))
		}
		if seenRes[r.Name] {
			return nil, fieldErr(path, "resources", fmt.Sprintf("duplicate resource name %q", r.Name))
		}
		seenRes[r.Name] = true

		def := ResourceDef{Name: r.Name}
		if r.DefaultMaxExpr != nil {
			e, err := Parse(*r.DefaultMaxExpr)
			if err != nil {
				return nil, fieldErr(path, field+".default_max_expr", err.Error())
			}
			if e.HasDice() {
				return nil, fieldErr(path, field+".default_max_expr", "must not contain dice (v1: default_max_expr is evaluated without recording its rolls — see internal/rules/expr.go)")
			}
			// default_max_expr is a single-actor position (spec v2 §5): a
			// scoped ref ('@caster.x'/'@target.x') has no second context to
			// resolve against here and must be rejected at load time, not
			// left to Eval's defense-in-depth runtime error. Grammar v2
			// (Task 1) parses scopes anywhere syntactically — this loader
			// enforces where they're actually legal. A no-op for every v1
			// fixture (none uses scope syntax), so v1 loading is unaffected.
			if hasScopedRef(e) {
				return nil, fieldErr(path, field+".default_max_expr", "must not contain a scoped reference (@caster.x / @target.x): default_max_expr is a single-actor position, write a bare reference instead")
			}
			def.DefaultMaxExpr = e
			def.DefaultMaxExprSrc = *r.DefaultMaxExpr
		}

		for j, th := range r.Thresholds {
			thField := fmt.Sprintf("%s.thresholds[%d]", field, j)
			if th.When == "" {
				return nil, fieldErr(path, thField+".when", "must not be empty")
			}
			if th.ApplyCondition == "" {
				return nil, fieldErr(path, thField+".apply_condition", "must not be empty")
			}
			if th.RemoveWhenFalse == nil {
				return nil, fieldErr(path, thField+".remove_when_false", "must be set (true or false; the key must be present)")
			}
			whenExpr, err := Parse(th.When)
			if err != nil {
				return nil, fieldErr(path, thField+".when", err.Error())
			}
			if whenExpr.HasDice() {
				return nil, fieldErr(path, thField+".when", "must not contain dice (v1: a threshold 'when' is evaluated without recording its rolls, which would break the rolled-once-recorded-forever testimony contract — see internal/rules/expr.go)")
			}
			// See the identical default_max_expr comment above: threshold
			// 'when' is also a single-actor position.
			if hasScopedRef(whenExpr) {
				return nil, fieldErr(path, thField+".when", "must not contain a scoped reference (@caster.x / @target.x): threshold when is a single-actor position, write a bare reference instead")
			}
			def.Thresholds = append(def.Thresholds, Threshold{
				When:            whenExpr,
				WhenSrc:         th.When,
				ApplyCondition:  th.ApplyCondition,
				RemoveWhenFalse: *th.RemoveWhenFalse,
			})
		}
		resources = append(resources, def)
	}

	return &loadedManifest{
		ID: raw.ID, Name: raw.Name, FormatVersion: raw.FormatVersion,
		Attributes: raw.Attributes, Defenses: raw.Defenses, resources: resources,
	}, nil
}

// --- conditions/*.json ---

type conditionJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func loadConditions(dir string) (map[string]*Condition, error) {
	out := map[string]*Condition{}
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		var raw conditionJSON
		if err := decodeStrict(path, &raw); err != nil {
			return nil, err
		}
		if raw.ID == "" {
			return nil, fieldErr(path, "id", "must not be empty")
		}
		if raw.Name == "" {
			return nil, fieldErr(path, "name", "must not be empty")
		}
		if reservedScopeWords[raw.ID] {
			return nil, fieldErr(path, "id", fmt.Sprintf("condition id %q is a reserved actor scope word (caster/target): reserved against condition ids too so a condition-kind param binding cannot occupy a ref scope position (finding F2)", raw.ID))
		}
		if _, dup := out[raw.ID]; dup {
			return nil, fieldErr(path, "id", fmt.Sprintf("duplicate condition id %q", raw.ID))
		}
		out[raw.ID] = &Condition{ID: raw.ID, Name: raw.Name, Description: raw.Description}
	}
	return out, nil
}

// --- outcome/effect JSON shapes ---
//
// These three types (outcomeJSON + its two field types) are the shape an
// atom's outcome contribution's "effects" array is made of
// (decodeOutcomeContribution, below, via its own rawEffects []outcomeJSON).

type outcomeJSON struct {
	ResourceChange  *resourceChangeJSON  `json:"resource_change,omitempty"`
	ApplyCondition  *applyConditionJSON  `json:"apply_condition,omitempty"`
	RemoveCondition *removeConditionJSON `json:"remove_condition,omitempty"`
}

type resourceChangeJSON struct {
	Resource  string `json:"resource"`
	DeltaExpr string `json:"delta_expr"`
}

type applyConditionJSON struct {
	ID string `json:"id"`
}

type removeConditionJSON struct {
	ID string `json:"id"`
}

// UnmarshalJSON decodes Usage from either the bare string "at_will" or an
// object {"limited": {"resource": ..., "cost": ...}} (spec §4) — a
// heterogeneous shape encoding/json's struct tags alone can't express,
// hence the hand-rolled dispatch. Both branches strictly reject unknown
// fields/values.
func (u *Usage) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		if asString != "at_will" {
			return fmt.Errorf("invalid usage %q (want \"at_will\" or an object with \"limited\")", asString)
		}
		*u = Usage{AtWill: true}
		return nil
	}

	var obj struct {
		Limited *struct {
			Resource string `json:"resource"`
			// Cost is a *int, not int: a plain int can't distinguish
			// "author wrote 0" (a legitimate limited-use-count ability
			// that costs no resource points) from "author omitted cost
			// entirely" — and omission must not silently behave as
			// free-to-use.
			Cost *int `json:"cost"`
		} `json:"limited"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&obj); err != nil {
		return fmt.Errorf("invalid usage object: %w", err)
	}
	if obj.Limited == nil {
		return fmt.Errorf(`usage object must set "limited"`)
	}
	if obj.Limited.Resource == "" {
		return fmt.Errorf(`usage.limited.resource must not be empty`)
	}
	if obj.Limited.Cost == nil {
		return fmt.Errorf(`usage.limited.cost must be set`)
	}
	// A negative cost would let Resolve (Task 5) GRANT the resource back
	// on every use instead of spending it — economy-breaking, and must be
	// caught at load time rather than discovered during play.
	if *obj.Limited.Cost < 0 {
		return fmt.Errorf(`usage.limited.cost must not be negative (got %d)`, *obj.Limited.Cost)
	}
	*u = Usage{Limited: &LimitedUsage{Resource: obj.Limited.Resource, Cost: *obj.Limited.Cost}}
	return nil
}

// --- cross-reference validation (spec §5) ---

// checkExprRefs validates every attribute/resource ref e.Refs() finds
// against attrSet/resSet, naming path/field on the first miss. Shared by
// crossValidateResources and compile.go's v2 cross-reference checks (spec
// §5's "v1-style cross-ref rules carried over" requirement) so both
// report undeclared names identically.
func checkExprRefs(path, field string, e *Expr, attrSet, resSet map[string]bool) error {
	if e == nil {
		return nil
	}
	attrs, resources := e.Refs()
	for _, a := range attrs {
		if !attrSet[a] {
			return fieldErr(path, field, fmt.Sprintf("references undeclared attribute %q", a))
		}
	}
	for _, r := range resources {
		if !resSet[r] {
			return fieldErr(path, field, fmt.Sprintf("references undeclared resource %q", r))
		}
	}
	return nil
}

// crossValidateResources checks every resource's default_max_expr and
// threshold 'when' ref, and every threshold's apply_condition id, against
// the manifest's declared attribute/resource set and declared conditions.
// resources+thresholds are declared identically in ruleset.json regardless
// of format version (spec §3); loadV2 calls this directly. (An ability's
// own cross-references are validated in compile.go against the compiled
// composition, not here.)
func crossValidateResources(rs *Ruleset) error {
	attrSet := toSet(rs.Attributes)
	resSet := map[string]bool{}
	for _, r := range rs.Resources {
		resSet[r.Name] = true
	}

	rulesetPath := "ruleset.json"
	for i, r := range rs.Resources {
		field := fmt.Sprintf("resources[%d]", i)
		if err := checkExprRefs(rulesetPath, field+".default_max_expr", r.DefaultMaxExpr, attrSet, resSet); err != nil {
			return err
		}
		for j, th := range r.Thresholds {
			thField := fmt.Sprintf("%s.thresholds[%d]", field, j)
			if err := checkExprRefs(rulesetPath, thField+".when", th.When, attrSet, resSet); err != nil {
				return err
			}
			if _, ok := rs.Conditions[th.ApplyCondition]; !ok {
				return fieldErr(rulesetPath, thField+".apply_condition", fmt.Sprintf("references undeclared condition %q", th.ApplyCondition))
			}
		}
	}
	return nil
}

func toSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, it := range items {
		out[it] = true
	}
	return out
}

// hasScopedRef reports whether e contains any scoped ref (@caster.x /
// @target.x, either sigil) — used to reject scopes in v2's single-actor
// positions (default_max_expr, threshold when; spec v2 §5).
func hasScopedRef(e *Expr) bool {
	for _, s := range e.Scopes() {
		if s != ScopeNone {
			return true
		}
	}
	return false
}

// --- atoms/*.json (format v2, spec §4) ---

// placeholderRe matches one "{name}" template placeholder occurrence.
// IDENT charset matches expr.go's (^[A-Za-z_][A-Za-z0-9_]*$) so a
// placeholder name is always a legal param name.
var placeholderRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// placeholderWholeFieldRe matches a JSON string value that is EXACTLY one
// placeholder and nothing else — the shape targeting's range/max_targets
// fields require (spec §4: targeting is a compile-time constant, never an
// expression, so no arithmetic/parenthesization applies there).
var placeholderWholeFieldRe = regexp.MustCompile(`^\{[A-Za-z_][A-Za-z0-9_]*\}$`)

// scopePositionPlaceholderRe matches a "{param}" placeholder occupying a
// ref's SCOPE position — directly between a sigil ('@'/'#') and the '.'
// that introduces the ref name, e.g. "@{who}.vim". See
// checkNoScopePositionPlaceholder (finding F2).
var scopePositionPlaceholderRe = regexp.MustCompile(`[@#]\{[A-Za-z_][A-Za-z0-9_]*\}\.`)

// reservedScopeWords are the two actor scope words (expr.go's
// parseScopeName). They may not be declared as an attribute, defense,
// resource, or condition id (finding F2, direction b): format v2 exposes
// attribute/defense values through the '@'-scope namespace ("@caster.x"),
// so a name colliding with a scope word makes such a ref ambiguous — and
// lets a name-kind param binding of "caster"/"target" silently occupy a
// ref's scope position (substTemplate relies on the words being reserved,
// alongside the scope-position-placeholder rejection below).
var reservedScopeWords = map[string]bool{"caster": true, "target": true}

// reservedBranchLabels are the branch-label words a resolution may NOT use
// because Resolve stamps them as fixed reason phases (finding R3): "always"
// marks an unconditional outcome contribution's branch, "usage" is the
// usage-spend event's reason phase, and "effect" is unconditional effect-
// phase outcomes' reason phase. Each maps to the reason it would forge.
var reservedBranchLabels = map[string]string{
	"always": "it marks an unconditional outcome contribution's branch",
	"usage":  "it is the fixed reason phase for the usage-spend event",
	"effect": "it is the fixed reason phase for unconditional effect-phase outcomes",
}

// checkNoScopePositionPlaceholder rejects a contribution expression
// template in which a "{param}" placeholder occupies a ref's SCOPE position
// — directly between a sigil and the '.' introducing the ref name (e.g.
// "@{who}.vim"). Such a placeholder is NOT substituted as the ref's name:
// its bound VALUE becomes the scope word, so a binding of "caster"/"target"
// changes the parse SHAPE of the surrounding expression (a scoped ref to a
// different name) while any other binding is a parse/scope error — the
// hygiene hole finding F2 documents. Name-kind params belong in the ref's
// NAME segment ("@caster.{stat}") or a plain name field, never its scope
// position.
func checkNoScopePositionPlaceholder(path, field, raw string) error {
	if m := scopePositionPlaceholderRe.FindString(raw); m != "" {
		return fieldErr(path, field, fmt.Sprintf("placeholder in %q occupies a reference's scope position (between the sigil and '.'): a name-kind param's bound value would become the actor scope, changing the expression's parse shape — put the placeholder in the ref's NAME segment instead (e.g. \"@caster.{name}\"), never its scope position", m))
	}
	return nil
}

// validParamKinds is the six closed param kinds (spec §4).
var validParamKinds = map[string]bool{
	"int": true, "expr": true,
	"attribute": true, "resource": true, "defense": true, "condition": true,
}

type atomJSON struct {
	ID          string            `json:"id"`
	Params      []paramJSON       `json:"params"`
	Provides    []string          `json:"provides"`
	Consumes    []string          `json:"consumes"`
	Contributes []json.RawMessage `json:"contributes"`
}

type paramJSON struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// loadAtoms decodes and statically validates every atoms/*.json file:
// strict JSON shape, the six closed param kinds (unknown param kind — spec
// §4 / validation catalogue), and every "{name}" placeholder appearing in
// any contribution template naming a param the atom actually declares
// (unknown param placeholder). It does NOT validate bindings, the
// provides/consumes graph, or produce any parsed *Expr — those need a
// specific composition's concrete bindings and are compile.go's job
// (compileCompositions), since the SAME atom is reused, differently bound,
// by many abilities.
func loadAtoms(dir string) (map[string]*AtomDef, error) {
	out := map[string]*AtomDef{}
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		var raw atomJSON
		if err := decodeStrict(path, &raw); err != nil {
			return nil, err
		}
		if raw.ID == "" {
			return nil, fieldErr(path, "id", "must not be empty")
		}
		if _, dup := out[raw.ID]; dup {
			return nil, fieldErr(path, "id", fmt.Sprintf("duplicate atom id %q", raw.ID))
		}
		if len(raw.Contributes) == 0 {
			return nil, fieldErr(path, "contributes", "must not be empty")
		}

		paramNames := map[string]bool{}
		paramKinds := map[string]string{}
		params := make([]ParamDef, 0, len(raw.Params))
		for i, p := range raw.Params {
			field := fmt.Sprintf("params[%d]", i)
			if p.Name == "" {
				return nil, fieldErr(path, field+".name", "must not be empty")
			}
			if !isValidIdentName(p.Name) {
				return nil, fieldErr(path, field+".name", fmt.Sprintf("param name %q must match the expression IDENT charset ^[A-Za-z_][A-Za-z0-9_]*$ (it is substituted into \"{name}\" placeholders)", p.Name))
			}
			if paramNames[p.Name] {
				return nil, fieldErr(path, field+".name", fmt.Sprintf("duplicate param name %q", p.Name))
			}
			if !validParamKinds[p.Kind] {
				return nil, fieldErr(path, field+".kind", fmt.Sprintf("unknown param kind %q (want one of: int, expr, attribute, resource, defense, condition)", p.Kind))
			}
			paramNames[p.Name] = true
			paramKinds[p.Name] = p.Kind
			params = append(params, ParamDef(p))
		}

		contributes := make([]Contribution, 0, len(raw.Contributes))
		for i, c := range raw.Contributes {
			contrib, err := decodeContribution(path, i, c, paramNames)
			if err != nil {
				return nil, err
			}
			contributes = append(contributes, contrib)
		}

		// Key-validity (F1, spec §4's third composition-validity clause):
		// a resolution contribution's key must be among this atom's
		// provides; a non-"always" outcome contribution's key must be among
		// its consumes. Atom-local, so checked here per atom — see the
		// composition validity matrix in compile.go.
		if err := checkAtomContributionKeys(path, raw.Provides, raw.Consumes, contributes); err != nil {
			return nil, err
		}

		out[raw.ID] = &AtomDef{
			ID: raw.ID, Params: params, Provides: raw.Provides, Consumes: raw.Consumes,
			Contributes: contributes, sourcePath: path,
		}
	}
	return out, nil
}

// checkPlaceholders scans raw for every "{name}" occurrence and confirms
// each name is a declared param — the "unknown param placeholder"
// validation rule, applied uniformly to every template string an atom's
// contributions carry.
func checkPlaceholders(path, field, raw string, paramNames map[string]bool) error {
	for _, m := range placeholderRe.FindAllStringSubmatch(raw, -1) {
		name := m[1]
		if !paramNames[name] {
			return fieldErr(path, field, fmt.Sprintf("placeholder {%s} does not name a declared param", name))
		}
	}
	return nil
}

// checkIntOrPlaceholderField validates a targeting range/max_targets raw
// field: either a JSON integer, or a JSON string holding EXACTLY one
// "{param}" placeholder (spec §4: targeting is a compile-time constant).
// Returns the field's normalized source text (decimal digits, or the bare
// placeholder text) for compile.go's resolveIntField to resolve per
// composition.
func checkIntOrPlaceholderField(path, field string, raw json.RawMessage, paramNames map[string]bool) (string, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("%d", n), nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if !placeholderWholeFieldRe.MatchString(s) {
			return "", fieldErr(path, field, fmt.Sprintf("string value %q must be exactly one \"{param}\" placeholder (targeting fields take a literal integer or a whole-field param reference, never a partial-text template)", s))
		}
		if err := checkPlaceholders(path, field, s, paramNames); err != nil {
			return "", err
		}
		return s, nil
	}
	return "", fieldErr(path, field, "must be an integer or a \"{param}\" placeholder string")
}

// contributionKeys enumerates the JSON keys decodeContribution recognizes
// for each contribution kind — used to reject a field that belongs to a
// DIFFERENT kind's shape (e.g. a "roll" key on a "targeting" contribution)
// even though the Go decode type below declares all of them (so
// DisallowUnknownFields alone can't catch a cross-kind field).
var contributionKeys = map[string]map[string]bool{
	"targeting":  {"kind": true, "range": true, "max_targets": true},
	"resolution": {"kind": true, "key": true, "roll": true, "vs": true, "branches": true},
	"outcome":    {"kind": true, "key": true, "branch": true, "effects": true},
}

func decodeContribution(path string, idx int, raw json.RawMessage, paramNames map[string]bool) (Contribution, error) {
	field := fmt.Sprintf("contributes[%d]", idx)

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Contribution{}, fieldErr(path, field, fmt.Sprintf("invalid JSON object: %v", err))
	}
	kindRaw, ok := probe["kind"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".kind", "must be set")
	}
	var kind string
	if err := json.Unmarshal(kindRaw, &kind); err != nil {
		return Contribution{}, fieldErr(path, field+".kind", "must be a string")
	}

	allowed, ok := contributionKeys[kind]
	if !ok {
		return Contribution{}, fieldErr(path, field+".kind", fmt.Sprintf("unknown contribution kind %q (want one of: targeting, resolution, outcome)", kind))
	}
	for k := range probe {
		if !allowed[k] {
			return Contribution{}, fieldErr(path, field, fmt.Sprintf("field %q is not valid on a %q contribution", k, kind))
		}
	}

	switch kind {
	case "targeting":
		return decodeTargetingContribution(path, field, probe, paramNames)
	case "resolution":
		return decodeResolutionContribution(path, field, probe, paramNames)
	default: // "outcome"
		return decodeOutcomeContribution(path, field, probe, paramNames)
	}
}

func decodeTargetingContribution(path, field string, probe map[string]json.RawMessage, paramNames map[string]bool) (Contribution, error) {
	rangeRaw, ok := probe["range"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".range", "must be set")
	}
	maxRaw, ok := probe["max_targets"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".max_targets", "must be set")
	}
	rangeSrc, err := checkIntOrPlaceholderField(path, field+".range", rangeRaw, paramNames)
	if err != nil {
		return Contribution{}, err
	}
	maxSrc, err := checkIntOrPlaceholderField(path, field+".max_targets", maxRaw, paramNames)
	if err != nil {
		return Contribution{}, err
	}
	return Contribution{Kind: "targeting", RangeSrc: rangeSrc, MaxTargetsSrc: maxSrc}, nil
}

func decodeResolutionContribution(path, field string, probe map[string]json.RawMessage, paramNames map[string]bool) (Contribution, error) {
	var key, roll, vs string
	var branches []string
	// Fixed key/roll/vs order (not a map range): when more than one of these
	// is missing/mistyped/empty, the reported field must be deterministic
	// across process runs — Go's randomized map iteration would otherwise
	// flake a CI/log-diff pinned on the error text (finding R2; matches this
	// package's determinism discipline — see compile.go's doc).
	for _, f := range []struct {
		name string
		dst  *string
	}{{"key", &key}, {"roll", &roll}, {"vs", &vs}} {
		raw, ok := probe[f.name]
		if !ok {
			return Contribution{}, fieldErr(path, field+"."+f.name, "must be set")
		}
		if err := json.Unmarshal(raw, f.dst); err != nil {
			return Contribution{}, fieldErr(path, field+"."+f.name, "must be a string")
		}
		if *f.dst == "" {
			return Contribution{}, fieldErr(path, field+"."+f.name, "must not be empty")
		}
	}
	branchesRaw, ok := probe["branches"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".branches", "must be set")
	}
	if err := json.Unmarshal(branchesRaw, &branches); err != nil {
		return Contribution{}, fieldErr(path, field+".branches", "must be an array of strings")
	}
	if len(branches) != 2 {
		return Contribution{}, fieldErr(path, field+".branches", fmt.Sprintf("must have exactly 2 entries ([ge-label, lt-label]), got %d", len(branches)))
	}
	for i, b := range branches {
		if b == "" {
			return Contribution{}, fieldErr(path, fmt.Sprintf("%s.branches[%d]", field, i), "must not be empty")
		}
		// Reserve "always"/"usage"/"effect" as branch labels (finding R3):
		// Resolve stamps each outcome event's reason as
		// "ability:<id>:<phase>", where phase is the branch label for branch
		// outcomes but the FIXED words "usage" (the usage-spend event) and
		// "effect" (unconditional effect-phase outcomes). A resolution branch
		// sharing one of those words would forge that fixed reason shape,
		// making the append-only log's testimony unable to distinguish a
		// conditional branch outcome from a usage-spend or an unconditional
		// effect — so they are rejected here, like "always".
		if why, ok := reservedBranchLabels[b]; ok {
			return Contribution{}, fieldErr(path, fmt.Sprintf("%s.branches[%d]", field, i), fmt.Sprintf("%q is reserved (%s) and cannot be a resolution branch label", b, why))
		}
	}
	if branches[0] == branches[1] {
		return Contribution{}, fieldErr(path, field+".branches", fmt.Sprintf("both branch labels are %q — labels must be distinct", branches[0]))
	}

	if err := checkPlaceholders(path, field+".roll", roll, paramNames); err != nil {
		return Contribution{}, err
	}
	if err := checkPlaceholders(path, field+".vs", vs, paramNames); err != nil {
		return Contribution{}, err
	}
	if err := checkNoScopePositionPlaceholder(path, field+".roll", roll); err != nil {
		return Contribution{}, err
	}
	if err := checkNoScopePositionPlaceholder(path, field+".vs", vs); err != nil {
		return Contribution{}, err
	}

	return Contribution{
		Kind: "resolution", Key: key, RollSrc: roll, VsSrc: vs,
		Branches: [2]string{branches[0], branches[1]},
	}, nil
}

// Complexity: one decoder over a wide oneof; the branches are mutually
// exclusive shapes, not nested logic.
//
//nolint:gocyclo
func decodeOutcomeContribution(path, field string, probe map[string]json.RawMessage, paramNames map[string]bool) (Contribution, error) {
	keyRaw, ok := probe["key"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".key", `must be set (a string, or explicit null for an "always" outcome)`)
	}
	var key *string
	if string(keyRaw) != "null" {
		var k string
		if err := json.Unmarshal(keyRaw, &k); err != nil {
			return Contribution{}, fieldErr(path, field+".key", "must be a string or null")
		}
		if k == "" {
			return Contribution{}, fieldErr(path, field+".key", "must not be empty (use null for an unconditional outcome)")
		}
		key = &k
	}

	branchRaw, ok := probe["branch"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".branch", "must be set")
	}
	var branch string
	if err := json.Unmarshal(branchRaw, &branch); err != nil {
		return Contribution{}, fieldErr(path, field+".branch", "must be a string")
	}
	if branch == "" {
		return Contribution{}, fieldErr(path, field+".branch", "must not be empty")
	}

	if branch == "always" && key != nil {
		return Contribution{}, fieldErr(path, field+".key", `must be null when branch is "always" (an unconditional outcome names no resolution)`)
	}
	if branch != "always" && key == nil {
		return Contribution{}, fieldErr(path, field+".key", `must be set (non-null) when branch is not "always" — it names which resolution's branch this outcome is conditioned on`)
	}

	effectsRaw, ok := probe["effects"]
	if !ok {
		return Contribution{}, fieldErr(path, field+".effects", "must be set")
	}
	var rawEffects []outcomeJSON
	if err := json.Unmarshal(effectsRaw, &rawEffects); err != nil {
		return Contribution{}, fieldErr(path, field+".effects", fmt.Sprintf("invalid JSON: %v", err))
	}

	effects := make([]EffectTemplate, 0, len(rawEffects))
	for i, o := range rawEffects {
		itemField := fmt.Sprintf("%s.effects[%d]", field, i)
		set := 0
		if o.ResourceChange != nil {
			set++
		}
		if o.ApplyCondition != nil {
			set++
		}
		if o.RemoveCondition != nil {
			set++
		}
		if set != 1 {
			return Contribution{}, fieldErr(path, itemField, "must set exactly one of resource_change, apply_condition, or remove_condition")
		}
		switch {
		case o.ResourceChange != nil:
			if o.ResourceChange.Resource == "" {
				return Contribution{}, fieldErr(path, itemField+".resource_change.resource", "must not be empty")
			}
			if o.ResourceChange.DeltaExpr == "" {
				return Contribution{}, fieldErr(path, itemField+".resource_change.delta_expr", "must not be empty")
			}
			if err := checkPlaceholders(path, itemField+".resource_change.resource", o.ResourceChange.Resource, paramNames); err != nil {
				return Contribution{}, err
			}
			if err := checkPlaceholders(path, itemField+".resource_change.delta_expr", o.ResourceChange.DeltaExpr, paramNames); err != nil {
				return Contribution{}, err
			}
			if err := checkNoScopePositionPlaceholder(path, itemField+".resource_change.delta_expr", o.ResourceChange.DeltaExpr); err != nil {
				return Contribution{}, err
			}
			effects = append(effects, EffectTemplate{
				Kind: OutcomeResourceChange, ResourceSrc: o.ResourceChange.Resource, DeltaExprSrc: o.ResourceChange.DeltaExpr,
			})
		case o.ApplyCondition != nil:
			if o.ApplyCondition.ID == "" {
				return Contribution{}, fieldErr(path, itemField+".apply_condition.id", "must not be empty")
			}
			if err := checkPlaceholders(path, itemField+".apply_condition.id", o.ApplyCondition.ID, paramNames); err != nil {
				return Contribution{}, err
			}
			effects = append(effects, EffectTemplate{Kind: OutcomeApplyCondition, ApplyConditionSrc: o.ApplyCondition.ID})
		case o.RemoveCondition != nil:
			if o.RemoveCondition.ID == "" {
				return Contribution{}, fieldErr(path, itemField+".remove_condition.id", "must not be empty")
			}
			if err := checkPlaceholders(path, itemField+".remove_condition.id", o.RemoveCondition.ID, paramNames); err != nil {
				return Contribution{}, err
			}
			effects = append(effects, EffectTemplate{Kind: OutcomeRemoveCondition, RemoveConditionSrc: o.RemoveCondition.ID})
		}
	}

	return Contribution{Kind: "outcome", OutcomeKey: key, Branch: branch, Effects: effects}, nil
}

// --- abilities/*.json as compositions (format v2, spec §4) ---

// compositionAbility is a v2 ability decoded but not yet compiled: id,
// name, and usage are plain fields identical to v1 (spec §3 — "usage
// stays a plain ability field"); Compose is the atom-ref+binding list
// compile.go's compileCompositions flattens into a CompiledPower.
type compositionAbility struct {
	ID      string
	Name    string
	Usage   Usage
	Compose []composeEntry

	sourcePath string
}

// composeEntry is one "{atom, bind}" element of a composition's "compose"
// list. Bind values are kept as raw JSON (validated against the atom's
// declared param kinds in compile.go, which is the first point a
// composeEntry's atom reference has been resolved to an actual AtomDef).
type composeEntry struct {
	Atom string
	Bind map[string]json.RawMessage
}

type compositionJSON struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Usage   Usage              `json:"usage"`
	Compose []composeEntryJSON `json:"compose"`
}

// composeEntryJSON's Bind is json.RawMessage, not map[string]json.RawMessage
// directly: a plain map can't distinguish "the \"bind\" key is absent" from
// "the \"bind\" key is present as {}" — both decode to a nil Go map. The
// schema requires "bind" (composeEntry.required = ["atom","bind"]), and for
// a PARAM-LESS atom omitting it entirely would otherwise go undetected (an
// atom WITH params already fails indirectly, via bindAtom's per-param
// "missing binding" check finding nothing in a nil map — but that
// consequential catch doesn't fire when there's nothing to bind).
type composeEntryJSON struct {
	Atom string          `json:"atom"`
	Bind json.RawMessage `json:"bind"`
}

func loadCompositions(dir string) (map[string]*compositionAbility, error) {
	out := map[string]*compositionAbility{}
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		var raw compositionJSON
		if err := decodeStrict(path, &raw); err != nil {
			return nil, err
		}
		if raw.ID == "" {
			return nil, fieldErr(path, "id", "must not be empty")
		}
		if raw.Name == "" {
			return nil, fieldErr(path, "name", "must not be empty")
		}
		if _, dup := out[raw.ID]; dup {
			return nil, fieldErr(path, "id", fmt.Sprintf("duplicate ability id %q", raw.ID))
		}
		// A JSON document with no "usage" key at all decodes Usage to its
		// zero value without UnmarshalJSON ever running.
		if !raw.Usage.AtWill && raw.Usage.Limited == nil {
			return nil, fieldErr(path, "usage", `must be set (either "at_will" or {"limited": {...}})`)
		}
		if len(raw.Compose) == 0 {
			return nil, fieldErr(path, "compose", "must not be empty")
		}
		compose := make([]composeEntry, 0, len(raw.Compose))
		for i, c := range raw.Compose {
			field := fmt.Sprintf("compose[%d]", i)
			if c.Atom == "" {
				return nil, fieldErr(path, field+".atom", "must not be empty")
			}
			if c.Bind == nil {
				return nil, fieldErr(path, field+".bind", `must be set (use {} for an atom with no params)`)
			}
			var bind map[string]json.RawMessage
			if err := json.Unmarshal(c.Bind, &bind); err != nil {
				return nil, fieldErr(path, field+".bind", fmt.Sprintf("must be an object: %v", err))
			}
			compose = append(compose, composeEntry{Atom: c.Atom, Bind: bind})
		}

		out[raw.ID] = &compositionAbility{
			ID: raw.ID, Name: raw.Name, Usage: raw.Usage, Compose: compose, sourcePath: path,
		}
	}
	return out, nil
}

// --- shared decode/error helpers ---

// decodeStrict decodes the JSON file at path into v with unknown fields
// disallowed (recursively — encoding/json.Decoder's DisallowUnknownFields
// applies through nested struct fields too, not just the top level).
func decodeStrict(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("rules: %s: %w", path, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("rules: %s: %w", path, err)
	}
	return nil
}

// fieldErr builds a load error naming both the offending file and field —
// every Load error goes through this (or decodeStrict's file-only variant
// for pure JSON-shape errors, which still name the file).
func fieldErr(path, field, msg string) error {
	return fmt.Errorf("rules: %s: field %q: %s", path, field, msg)
}

// jsonFilesIn returns the sorted list of *.json paths directly inside dir
// (non-recursive; dir not existing is not an error — a ruleset with no
// conditions, for instance, is legal, so an absent conditions/ directory
// yields zero files rather than a load error).
func jsonFilesIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("rules: %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}
