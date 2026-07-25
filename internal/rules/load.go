package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// supportedFormatVersion is the only format_version value Load accepts.
// The ruleset format is v1 (spec §4); a future v2 gets its own constant and
// an explicit migration story, not silent multi-version support.
const supportedFormatVersion = "1"

// Load reads and fully validates the ruleset directory at dir: strict JSON
// decoding of ruleset.json, abilities/*.json, and conditions/*.json (no
// unknown fields tolerated), the grammar of every expression (parsed once,
// here, so a loaded Ruleset never fails to parse an expression at runtime),
// and every cross-reference an ability, resource, or threshold makes
// (attributes, resources, defenses, and condition ids must all be declared
// — spec §5). Every error names the offending file and field.
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
	if raw.FormatVersion != supportedFormatVersion {
		return nil, fieldErr(path, "format_version", fmt.Sprintf("unsupported value %q (only %q is supported)", raw.FormatVersion, supportedFormatVersion))
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

// --- cross-reference validation (spec §5) ---

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

	rulesetPath := "ruleset.json"
	for i, r := range rs.Resources {
		field := fmt.Sprintf("resources[%d]", i)
		if err := checkExpr(rulesetPath, field+".default_max_expr", r.DefaultMaxExpr); err != nil {
			return err
		}
		for j, th := range r.Thresholds {
			thField := fmt.Sprintf("%s.thresholds[%d]", field, j)
			if err := checkExpr(rulesetPath, thField+".when", th.When); err != nil {
				return err
			}
			if _, ok := rs.Conditions[th.ApplyCondition]; !ok {
				return fieldErr(rulesetPath, thField+".apply_condition", fmt.Sprintf("references undeclared condition %q", th.ApplyCondition))
			}
		}
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
