package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// supportedFormatVersions is the set of format_version values Load
// accepts: "1" (spec §4, the original interpreter format — loaded exactly
// as before, unchanged by this file's v2 additions) and "2" (spec
// 2026-07-25-format-v2-composition-design.md §3 — atoms/compositions,
// compiled at load into the same execution shape v1 abilities already
// have; see loadV2 and compile.go). Any other value is rejected with a
// clear error naming format_version — no silent multi-version guessing.
var supportedFormatVersions = map[string]bool{"1": true, "2": true}

// Load reads and fully validates the ruleset directory at dir. For
// format_version "1": strict JSON decoding of ruleset.json,
// abilities/*.json, and conditions/*.json (no unknown fields tolerated),
// the grammar of every expression (parsed once, here, so a loaded Ruleset
// never fails to parse an expression at runtime), every cross-reference an
// ability, resource, or threshold makes (attributes, resources, defenses,
// and condition ids must all be declared — spec §5), and — Task 3 —
// adaptV1Abilities flattens every loaded Ability into Ruleset.Compiled via
// AdaptV1Ability, byte-identically to how it already executed (see that
// function's doc comment). For format_version "2": the same
// manifest/conditions handling, plus atoms/*.json and
// abilities/*.json-as-compositions, compiled at load into Ruleset.Compiled
// (see loadV2, compile.go — spec §6). Either way, Ruleset.Compiled is
// always populated on a successful Load, and Resolve (Task 5, rewired
// Task 3) reads Compiled exclusively — spec 5c pillar "ONE execution
// path". Every error names the offending file and field.
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

	if manifest.FormatVersion == "2" {
		return loadV2(dir, manifest)
	}

	conditions, err := loadConditions(filepath.Join(dir, "conditions"))
	if err != nil {
		return nil, err
	}

	abilities, err := loadAbilities(filepath.Join(dir, "abilities"))
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
		Abilities:     abilities,
		Conditions:    conditions,
		Guide:         string(guide),
	}

	if err := crossValidate(rs); err != nil {
		return nil, err
	}

	compiled, err := adaptV1Abilities(rs.Abilities)
	if err != nil {
		return nil, err
	}
	rs.Compiled = compiled
	return rs, nil
}

// loadV2 is format_version "2"'s Load path (spec §3, §6): manifest (shared
// with v1 via loadManifest) plus conditions/ (shared, unchanged shape),
// atoms/, and abilities/-as-compositions, compiled into Ruleset.Compiled.
// Resolve (Task 5) reads Compiled only; Abilities is left as an empty,
// non-nil map for a v2-loaded Ruleset (v2's abilities/*.json files are not
// v1 Ability-shaped — they are compositions, decoded into a distinct
// internal type by loadCompositions and consumed only by compile.go).
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
		Abilities:     map[string]*Ability{},
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
// since manifestJSON.resources needs its exprs parsed before crossValidate
// can walk them for Refs().
type loadedManifest struct {
	ID            string
	Name          string
	FormatVersion string
	Attributes    []string
	Defenses      []string
	resources     []ResourceDef
}

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
		return nil, fieldErr(path, "format_version", fmt.Sprintf("unsupported value %q (supported: \"1\", \"2\")", raw.FormatVersion))
	}

	seenAttr := map[string]bool{}
	for _, a := range raw.Attributes {
		if a == "" {
			return nil, fieldErr(path, "attributes", "must not contain an empty name")
		}
		if !isValidIdentName(a) {
			return nil, fieldErr(path, "attributes", fmt.Sprintf("attribute name %q must match the expression IDENT charset ^[A-Za-z_][A-Za-z0-9_]*$ (it is referenced as '@'+name inside expressions)", a))
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
		if _, dup := out[raw.ID]; dup {
			return nil, fieldErr(path, "id", fmt.Sprintf("duplicate condition id %q", raw.ID))
		}
		out[raw.ID] = &Condition{ID: raw.ID, Name: raw.Name, Description: raw.Description}
	}
	return out, nil
}

// --- abilities/*.json ---

type abilityJSON struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Usage     Usage         `json:"usage"`
	Targeting targetingJSON `json:"targeting"`
	Attack    *attackJSON   `json:"attack"`
	Hit       []outcomeJSON `json:"hit"`
	Miss      []outcomeJSON `json:"miss"`
	Effect    []outcomeJSON `json:"effect"`
}

type targetingJSON struct {
	// Range is a *int, not int: a plain int can't distinguish "author
	// wrote 0" (a legitimate self/no-range ability — see
	// testdata/valid/abilities/guard-stance.json) from "author omitted
	// range entirely". MaxTargets stays a plain int: it has no legitimate
	// zero value (must be >= 1), so the existing "< 1" check already
	// catches omission without needing the same treatment.
	Range      *int `json:"range"`
	MaxTargets int  `json:"max_targets"`
}

type attackJSON struct {
	Roll string `json:"roll"`
	Vs   string `json:"vs"`
}

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

func loadAbilities(dir string) (map[string]*Ability, error) {
	out := map[string]*Ability{}
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		var raw abilityJSON
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
		// zero value (AtWill=false, Limited=nil) without (*Usage).
		// UnmarshalJSON ever running — encoding/json only invokes a field's
		// UnmarshalJSON when the key is present. Reject that silently-
		// unusable state explicitly rather than letting it through.
		if !raw.Usage.AtWill && raw.Usage.Limited == nil {
			return nil, fieldErr(path, "usage", `must be set (either "at_will" or {"limited": {...}})`)
		}
		if raw.Targeting.MaxTargets < 1 {
			return nil, fieldErr(path, "targeting.max_targets", "must be at least 1")
		}
		if raw.Targeting.Range == nil {
			return nil, fieldErr(path, "targeting.range", "must be set (the key must be present, even for range 0)")
		}
		if *raw.Targeting.Range < 0 {
			return nil, fieldErr(path, "targeting.range", "must not be negative")
		}

		ability := &Ability{
			ID:         raw.ID,
			Name:       raw.Name,
			Usage:      raw.Usage,
			Targeting:  Targeting{Range: *raw.Targeting.Range, MaxTargets: raw.Targeting.MaxTargets},
			sourcePath: path,
		}

		if raw.Attack != nil {
			if raw.Attack.Roll == "" {
				return nil, fieldErr(path, "attack.roll", "must not be empty")
			}
			if raw.Attack.Vs == "" {
				return nil, fieldErr(path, "attack.vs", "must not be empty")
			}
			rollExpr, err := Parse(raw.Attack.Roll)
			if err != nil {
				return nil, fieldErr(path, "attack.roll", err.Error())
			}
			ability.Attack = &Attack{Roll: rollExpr, RollSrc: raw.Attack.Roll, Vs: raw.Attack.Vs}
		} else if len(raw.Hit) > 0 || len(raw.Miss) > 0 {
			return nil, fieldErr(path, "hit/miss", "must be empty on an ability with no attack")
		}

		ability.Hit, err = convertOutcomes(path, "hit", raw.Hit)
		if err != nil {
			return nil, err
		}
		ability.Miss, err = convertOutcomes(path, "miss", raw.Miss)
		if err != nil {
			return nil, err
		}
		ability.Effect, err = convertOutcomes(path, "effect", raw.Effect)
		if err != nil {
			return nil, err
		}

		out[raw.ID] = ability
	}
	return out, nil
}

func convertOutcomes(path, field string, raw []outcomeJSON) ([]Outcome, error) {
	out := make([]Outcome, 0, len(raw))
	for i, o := range raw {
		itemField := fmt.Sprintf("%s[%d]", field, i)
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
			return nil, fieldErr(path, itemField, "must set exactly one of resource_change, apply_condition, or remove_condition")
		}

		switch {
		case o.ResourceChange != nil:
			rc := o.ResourceChange
			if rc.Resource == "" {
				return nil, fieldErr(path, itemField+".resource_change.resource", "must not be empty")
			}
			if rc.DeltaExpr == "" {
				return nil, fieldErr(path, itemField+".resource_change.delta_expr", "must not be empty")
			}
			e, err := Parse(rc.DeltaExpr)
			if err != nil {
				return nil, fieldErr(path, itemField+".resource_change.delta_expr", err.Error())
			}
			out = append(out, Outcome{
				Kind: OutcomeResourceChange,
				ResourceChange: &ResourceChangeOutcome{
					Resource: rc.Resource, DeltaExpr: e, DeltaExprSrc: rc.DeltaExpr,
				},
			})
		case o.ApplyCondition != nil:
			if o.ApplyCondition.ID == "" {
				return nil, fieldErr(path, itemField+".apply_condition.id", "must not be empty")
			}
			out = append(out, Outcome{Kind: OutcomeApplyCondition, ApplyCondition: &ApplyConditionOutcome{ID: o.ApplyCondition.ID}})
		case o.RemoveCondition != nil:
			if o.RemoveCondition.ID == "" {
				return nil, fieldErr(path, itemField+".remove_condition.id", "must not be empty")
			}
			out = append(out, Outcome{Kind: OutcomeRemoveCondition, RemoveCondition: &RemoveConditionOutcome{ID: o.RemoveCondition.ID}})
		}
	}
	return out, nil
}

// --- v1-to-CompiledPower adapter (Task 3, spec 5c pillar: "Resolve
// executes CompiledPower through ONE code path") ---

// v1RefRe matches one v1-shaped ref occurrence: a sigil ('@' or '#'),
// optional whitespace (the grammar's tokenizer allows whitespace between
// the sigil and its identifier — they lex as two separate tokens, and
// skipSpace runs before every token), then an IDENT. '@'/'#' have no other
// grammatical meaning in this expression language, and v1-authored
// expression source never contains scope syntax (the '.'-scope grammar
// postdates v1 content, and scopeV1Source only ever runs on text that has
// ALREADY Parse()d successfully under v1's bare-ref-only usage at
// ability-decode time — loadAbilities/convertOutcomes, above) — so
// rewriting every occurrence this regex finds is safe unconditionally, not
// just for the fixtures this task happens to exercise.
var v1RefRe = regexp.MustCompile(`[@#][ \t\r\n]*[A-Za-z_][A-Za-z0-9_]*`)

// scopeV1Source rewrites every bare ref occurrence in src into its
// explicitly-scoped v2 equivalent under scope ("caster" or "target"), e.g.
// scopeV1Source("1d20 + @vim", "caster") == "1d20 + @caster.vim". Used by
// AdaptV1Ability to translate v1's IMPLICIT positional scoping — an attack
// roll evaluates against the CASTER; a hit/miss/effect outcome expression
// evaluates against the TARGET (resolve.go's Resolve doc comment
// documents this as v1's contract, preserved verbatim as the exact
// behavior this adapter must keep observably true) — into the EXPLICIT
// scoping EvalScoped requires (Task 1), since Resolve now executes every
// ability, v1 and v2 alike, through EvalScoped only.
//
// The scope word this function inserts is always one of the two
// compile-time constants "caster"/"target", and it is always inserted
// FIRST, immediately after the sigil — so the rewritten text is
// guaranteed Parse-able regardless of what identifier follows, even an
// attribute coincidentally NAMED "caster" or "target" (that identifier
// becomes the ref's NAME segment, after the dot, exactly like any other
// name; parseRef only ever treats the FIRST identifier after a sigil as a
// candidate scope word, and this function's output always has the real
// scope word occupying that position, never the original identifier).
func scopeV1Source(src, scope string) string {
	return v1RefRe.ReplaceAllStringFunc(src, func(m string) string {
		ident := strings.TrimLeft(m[1:], " \t\r\n")
		return string(m[0]) + scope + "." + ident
	})
}

// scopedParse re-parses src (an expression source string that has ALREADY
// Parse()d successfully once, as plain v1 source) after scoping every bare
// ref in it to scope. Returns an error rather than panicking if the
// rewritten text somehow fails to parse — provably unreachable given the
// invariants scopeV1Source's own doc comment establishes (src already
// parses under v1's grammar, a strict subset of v2's; the inserted scope
// word is always one of the two legal words) — but the adapter surfaces
// this as a clean, named load error rather than trusting that invariant
// silently, matching this file's "no silent multi-version guessing"
// convention throughout.
func scopedParse(src, scope string) (*Expr, error) {
	e, err := Parse(scopeV1Source(src, scope))
	if err != nil {
		return nil, fmt.Errorf("rules: adapt: scoping %q to %s: %w", src, scope, err)
	}
	return e, nil
}

// AdaptV1Ability flattens one format-v1 Ability into a CompiledPower — the
// v1-to-CompiledPower adapter itself. Exported (unlike the rest of this
// file's v1 machinery) so a caller that builds a *Ruleset by hand,
// bypassing Load entirely (internal/rules/resolve_test.go's
// fixtureRuleset — a deliberate, pre-existing test pattern for exercising
// Resolve without depending on Load's cross-validation), can populate
// Ruleset.Compiled with the EXACT same logic adaptV1Abilities uses for
// every real format-v1 Load. This keeps "Resolve executes CompiledPower
// through ONE code path" true for hand-built test rulesets too, rather
// than requiring Resolve to special-case a Ruleset with a populated
// Abilities map but no Compiled entries.
//
// v1's attack {roll, vs-defense-name} becomes a CompiledResolution{Roll,
// Vs}, with Branches fixed at ["hit", "miss"] — v1's own words, verbatim —
// so a v1 ability's AbilityUsed.outcome_summary is byte-identical through
// the new path (rulesets/tavern-brawl's committed goldens, and
// resolve_test.go's envelope-shaped tests, are the proof this must hold).
//
// # The SourceText decision
//
// v1's implicit positional scoping (see scopeV1Source's doc comment) is
// made EXPLICIT here via scopedParse — but the EXECUTED expression and the
// DISPLAYED (testimony) text must diverge for a v1-adapted ability:
// CompiledResolution.Roll must be the CASTER-scoped parse (so EvalScoped
// can run it), while CompiledResolution.RollSrc must stay v1's ORIGINAL,
// UNSCOPED Attack.RollSrc text — Resolve records RollSrc onto
// AbilityUsed.rolls[].expression, and a v1 golden's recorded expression
// string (e.g. "1d20 + @brawn") must never change to "1d20 + @caster.brawn"
// through this adapter, or every existing v1 batch golden would break.
// The same split applies to every hit/miss/effect resource_change's
// DeltaExpr/DeltaExprSrc (adaptV1Outcomes, below): DeltaExpr becomes the
// TARGET-scoped executable parse; DeltaExprSrc is left exactly as
// loadAbilities/convertOutcomes already set it at v1-decode time (v1's
// original, unscoped source), never touched by this function. VsSrc is
// the one field with no true v1 "original" to preserve — v1's Attack.Vs
// was a bare defense NAME, never an expression at all — so it is set to
// the synthesized "@target.<defense>" text; this is only ever OBSERVABLE
// (recorded onto AbilityUsed.rolls) if evaluating it actually rolls dice,
// and a bare attribute ref never does (see resolve.go's Vs-recording
// contract), so no v1 golden ever displays it regardless.
func AdaptV1Ability(a *Ability) (*CompiledPower, error) {
	cp := &CompiledPower{
		ID:        a.ID,
		Name:      a.Name,
		Usage:     a.Usage,
		Targeting: a.Targeting,
	}

	if a.Attack != nil {
		rollExpr, err := scopedParse(a.Attack.RollSrc, "caster")
		if err != nil {
			return nil, fmt.Errorf("rules: adapt: ability %q: attack.roll: %w", a.ID, err)
		}
		vsSrc := "@target." + a.Attack.Vs
		vsExpr, err := Parse(vsSrc)
		if err != nil {
			return nil, fmt.Errorf("rules: adapt: ability %q: attack.vs: %w", a.ID, err)
		}
		cp.Resolution = &CompiledResolution{
			Roll: rollExpr, RollSrc: a.Attack.RollSrc,
			Vs: vsExpr, VsSrc: vsSrc,
			Branches: [2]string{"hit", "miss"},
		}

		hitOutcomes, err := adaptV1Outcomes(a.Hit)
		if err != nil {
			return nil, fmt.Errorf("rules: adapt: ability %q: hit: %w", a.ID, err)
		}
		missOutcomes, err := adaptV1Outcomes(a.Miss)
		if err != nil {
			return nil, fmt.Errorf("rules: adapt: ability %q: miss: %w", a.ID, err)
		}
		cp.BranchOutcomes[0] = hitOutcomes
		cp.BranchOutcomes[1] = missOutcomes
	}

	effects, err := adaptV1Outcomes(a.Effect)
	if err != nil {
		return nil, fmt.Errorf("rules: adapt: ability %q: effect: %w", a.ID, err)
	}
	cp.Effects = effects
	return cp, nil
}

// adaptV1Outcomes scopes every resource_change outcome's DeltaExpr to
// "target" (v1's implicit outcome-evaluation actor — AdaptV1Ability's doc
// comment), leaving DeltaExprSrc, Resource, and every apply_condition/
// remove_condition outcome exactly as v1 decoded it. Never mutates the
// Outcome/ResourceChangeOutcome values reachable from the ORIGINAL
// Ability — outcomes is a.Hit/a.Miss/a.Effect, still owned by
// Ruleset.Abilities after this returns (Task 2's decision: Abilities stays
// populated for a v1-loaded Ruleset), so this copies before writing a new
// DeltaExpr.
func adaptV1Outcomes(outcomes []Outcome) ([]Outcome, error) {
	if len(outcomes) == 0 {
		return nil, nil
	}
	out := make([]Outcome, len(outcomes))
	for i, o := range outcomes {
		out[i] = o
		if o.Kind == OutcomeResourceChange {
			rc := *o.ResourceChange // copy: never mutate the original v1 Outcome tree
			scoped, err := scopedParse(rc.DeltaExprSrc, "target")
			if err != nil {
				return nil, fmt.Errorf("resource_change on %q: %w", rc.Resource, err)
			}
			rc.DeltaExpr = scoped
			out[i].ResourceChange = &rc
		}
	}
	return out, nil
}

// adaptV1Abilities runs AdaptV1Ability over every ability in abilities,
// building the Ruleset.Compiled map for a format-v1 Load (Load's v1 path,
// above). Iterates in sorted-id order — not for output determinism (the
// result is a map; map equality does not depend on build order) but so
// that IF AdaptV1Ability ever failed (provably unreachable per its own doc
// comment, for any Ability that already passed crossValidate), the
// reported error would name the same ability on every run, matching this
// file's and compile.go's shared determinism convention for error
// reporting.
func adaptV1Abilities(abilities map[string]*Ability) (map[string]*CompiledPower, error) {
	ids := make([]string, 0, len(abilities))
	for id := range abilities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make(map[string]*CompiledPower, len(abilities))
	for _, id := range ids {
		cp, err := AdaptV1Ability(abilities[id])
		if err != nil {
			return nil, err
		}
		out[id] = cp
	}
	return out, nil
}

// --- cross-reference validation (spec §5) ---

// checkExprRefs validates every attribute/resource ref e.Refs() finds
// against attrSet/resSet, naming path/field on the first miss. Shared by
// crossValidate (v1) and compile.go's v2 cross-reference checks (spec §5's
// "v1-style cross-ref rules carried over" requirement) so both versions
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
// Split out of crossValidate (review: v2 sub-project 5c) because
// resources+thresholds are declared identically in ruleset.json for both
// format versions (spec §3) — loadV2 needs exactly this check, with none
// of crossValidate's v1-Ability-specific logic below.
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

// crossValidate checks every attribute/resource ref, every declared
// resource/condition/defense name an ability or threshold points at,
// against the sets manifest.json/conditions/abilities actually declared.
// Runs after every file has been individually decoded and every
// expression parsed, so any failure here is purely a referential-integrity
// problem, not a syntax one.
func crossValidate(rs *Ruleset) error {
	attrSet := toSet(rs.Attributes)
	defSet := toSet(rs.Defenses)
	resSet := map[string]bool{}
	for _, r := range rs.Resources {
		resSet[r.Name] = true
	}
	checkExpr := func(path, field string, e *Expr) error {
		return checkExprRefs(path, field, e, attrSet, resSet)
	}

	if err := crossValidateResources(rs); err != nil {
		return err
	}

	// Stable iteration order for deterministic error messages across runs.
	ids := make([]string, 0, len(rs.Abilities))
	for id := range rs.Abilities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		a := rs.Abilities[id]
		path := a.sourcePath

		if a.Attack != nil {
			if err := checkExpr(path, "attack.roll", a.Attack.Roll); err != nil {
				return err
			}
			if !defSet[a.Attack.Vs] {
				return fieldErr(path, "attack.vs", fmt.Sprintf("references undeclared defense %q", a.Attack.Vs))
			}
		}

		if a.Usage.Limited != nil {
			if !resSet[a.Usage.Limited.Resource] {
				return fieldErr(path, "usage.limited.resource", fmt.Sprintf("references undeclared resource %q", a.Usage.Limited.Resource))
			}
		}

		for _, list := range []struct {
			field    string
			outcomes []Outcome
		}{
			{"hit", a.Hit}, {"miss", a.Miss}, {"effect", a.Effect},
		} {
			for i, o := range list.outcomes {
				itemField := fmt.Sprintf("%s[%d]", list.field, i)
				switch o.Kind {
				case OutcomeResourceChange:
					if err := checkExpr(path, itemField+".resource_change.delta_expr", o.ResourceChange.DeltaExpr); err != nil {
						return err
					}
					if !resSet[o.ResourceChange.Resource] {
						return fieldErr(path, itemField+".resource_change.resource", fmt.Sprintf("references undeclared resource %q", o.ResourceChange.Resource))
					}
				case OutcomeApplyCondition:
					if _, ok := rs.Conditions[o.ApplyCondition.ID]; !ok {
						return fieldErr(path, itemField+".apply_condition.id", fmt.Sprintf("references undeclared condition %q", o.ApplyCondition.ID))
					}
				case OutcomeRemoveCondition:
					if _, ok := rs.Conditions[o.RemoveCondition.ID]; !ok {
						return fieldErr(path, itemField+".remove_condition.id", fmt.Sprintf("references undeclared condition %q", o.RemoveCondition.ID))
					}
				}
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
			params = append(params, ParamDef{Name: p.Name, Kind: p.Kind})
		}

		contributes := make([]Contribution, 0, len(raw.Contributes))
		for i, c := range raw.Contributes {
			contrib, err := decodeContribution(path, i, c, paramNames)
			if err != nil {
				return nil, err
			}
			contributes = append(contributes, contrib)
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
	for name, dst := range map[string]*string{"key": &key, "roll": &roll, "vs": &vs} {
		raw, ok := probe[name]
		if !ok {
			return Contribution{}, fieldErr(path, field+"."+name, "must be set")
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			return Contribution{}, fieldErr(path, field+"."+name, "must be a string")
		}
		if *dst == "" {
			return Contribution{}, fieldErr(path, field+"."+name, "must not be empty")
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
		if b == "always" {
			return Contribution{}, fieldErr(path, fmt.Sprintf("%s.branches[%d]", field, i), `"always" is reserved (it marks an unconditional outcome contribution's branch) and cannot be a resolution branch label`)
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

	return Contribution{
		Kind: "resolution", Key: key, RollSrc: roll, VsSrc: vs,
		Branches: [2]string{branches[0], branches[1]},
	}, nil
}

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
		// Same rationale as v1's loadAbilities: a JSON document with no
		// "usage" key at all decodes Usage to its zero value without
		// UnmarshalJSON ever running.
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
