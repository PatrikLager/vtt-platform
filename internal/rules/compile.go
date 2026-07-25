package rules

// compile.go turns a format-v2 ruleset's atoms + compositions into
// Ruleset.Compiled (spec §6, compile-at-load): resolve each composition's
// atom refs, validate param bindings against the atoms' declared param
// kinds, validate the provides/consumes dependency graph (topological
// execution order, ties broken by composition list order — never
// load-bearing for correctness, only for determinism), validate exactly
// one targeting and at most one resolution contribution, validate every
// outcome's branch/key against its resolution, splice params into
// contribution expression templates HYGIENICALLY (int/expr params as
// parenthesized parsed subtrees; name-kind params as bare identifiers in
// ref/name positions — spec §2 guarantee 4, spec §4), and flatten the
// result into a CompiledPower. Resolve (Task 5) reads Compiled only; it
// never walks an atom graph.
//
// Determinism: every loop that affects compiled OUTPUT structure walks a
// slice (composition list order, or the topo-sorted order derived from
// it) — never a Go map. Maps here are used exclusively for O(1) set
// membership / lookup, never ranged over to produce output; the one place
// map keys are surfaced in an error message (bindAtom's unknown-bind-key
// check) sorts them first so repeated failing runs report the same error.
// TestCompileDeterministic (compile_test.go) pins this by loading the same
// v2 fixture many times in one process and asserting reflect.DeepEqual
// across every run — Go's per-map iteration randomization would surface a
// leak within a handful of runs if one existed.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// compileCompositions compiles every composition ability in compositions
// (already individually decoded/shape-validated by loadCompositions)
// against atoms (already individually decoded/shape-validated by
// loadAtoms) and rs's declared vocabulary, returning one CompiledPower per
// ability id.
func compileCompositions(rs *Ruleset, atoms map[string]*AtomDef, compositions map[string]*compositionAbility) (map[string]*CompiledPower, error) {
	attrSet := toSet(rs.Attributes)
	defSet := toSet(rs.Defenses)
	resSet := map[string]bool{}
	for _, r := range rs.Resources {
		resSet[r.Name] = true
	}
	// A defense's runtime value is exposed through the SAME '@' attribute
	// namespace as attributes (Task 1's EvalContext carries only
	// Attrs/Resources — no separate defense map), so an '@'-ref inside a
	// fully-substituted two-actor expression is valid against the union.
	// Individual param BINDINGS are still checked against their precise
	// declared kind (attribute vs defense) in bindAtom — this union only
	// covers the permissive final-expression check, which also has to
	// accept refs an atom author wrote directly (e.g. "@target.ac"),
	// never having gone through a param placeholder at all.
	attrOrDefSet := make(map[string]bool, len(attrSet)+len(defSet))
	for k := range attrSet {
		attrOrDefSet[k] = true
	}
	for k := range defSet {
		attrOrDefSet[k] = true
	}

	// Stable iteration order, matching v1's crossValidate — determinism of
	// which composition fails first on an invalid ruleset, and (mostly for
	// hygiene, since map equality ignores insertion order) predictable
	// build order for the compiled map's contents.
	ids := make([]string, 0, len(compositions))
	for id := range compositions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make(map[string]*CompiledPower, len(compositions))
	for _, id := range ids {
		cp, err := compilePower(rs, atoms, compositions[id], attrSet, defSet, resSet, attrOrDefSet)
		if err != nil {
			return nil, err
		}
		out[id] = cp
	}
	return out, nil
}

// compilePower flattens one composition ability into a CompiledPower.
func compilePower(rs *Ruleset, atoms map[string]*AtomDef, ca *compositionAbility, attrSet, defSet, resSet, attrOrDefSet map[string]bool) (*CompiledPower, error) {
	path := ca.sourcePath
	n := len(ca.Compose)

	atomInstances := make([]*AtomDef, n)
	names := make([]string, n)
	textBindings := make([]map[string]string, n)
	intBindings := make([]map[string]int, n)

	for i, entry := range ca.Compose {
		atom, ok := atoms[entry.Atom]
		if !ok {
			return nil, fieldErr(path, fmt.Sprintf("compose[%d].atom", i), fmt.Sprintf("unknown atom %q", entry.Atom))
		}
		atomInstances[i] = atom
		names[i] = entry.Atom

		tb, ib, err := bindAtom(rs, path, i, atom, entry.Bind, attrSet, resSet, defSet)
		if err != nil {
			return nil, err
		}
		textBindings[i] = tb
		intBindings[i] = ib
	}

	order, err := buildAndSortDAG(path, atomInstances, names)
	if err != nil {
		return nil, err
	}

	targeting, err := compileTargeting(path, atomInstances, intBindings)
	if err != nil {
		return nil, err
	}

	resolution, resolutionKey, err := compileResolution(path, atomInstances, textBindings, attrOrDefSet, resSet)
	if err != nil {
		return nil, err
	}

	var branchOutcomes [2][]Outcome
	var effects []Outcome
	for _, i := range order {
		for ci, c := range atomInstances[i].Contributes {
			if c.Kind != "outcome" {
				continue
			}
			outcomes, err := compileEffects(path, i, ci, c.Effects, textBindings[i], rs, attrOrDefSet, resSet)
			if err != nil {
				return nil, err
			}
			if c.Branch == "always" {
				effects = append(effects, outcomes...)
				continue
			}
			if resolution == nil {
				return nil, fieldErr(path, "compose", fmt.Sprintf("atom %q at compose[%d] contributes an outcome for resolution key %q, but this composition has no resolution contribution", names[i], i, *c.OutcomeKey))
			}
			if *c.OutcomeKey != resolutionKey {
				return nil, fieldErr(path, "compose", fmt.Sprintf("atom %q at compose[%d] contributes an outcome for key %q, but this composition's resolution provides key %q", names[i], i, *c.OutcomeKey, resolutionKey))
			}
			switch c.Branch {
			case resolution.Branches[0]:
				branchOutcomes[0] = append(branchOutcomes[0], outcomes...)
			case resolution.Branches[1]:
				branchOutcomes[1] = append(branchOutcomes[1], outcomes...)
			default:
				return nil, fieldErr(path, "compose", fmt.Sprintf("atom %q at compose[%d] outcome branch %q is not among the resolution's labels (%q, %q)", names[i], i, c.Branch, resolution.Branches[0], resolution.Branches[1]))
			}
		}
	}

	return &CompiledPower{
		ID: ca.ID, Name: ca.Name,
		Usage: ca.Usage, Targeting: targeting,
		Resolution: resolution, BranchOutcomes: branchOutcomes, Effects: effects,
	}, nil
}

// bindAtom validates compose[idx]'s bind map against atom's declared
// params (spec §4 validation catalogue: kind mismatch vs manifest
// declarations) and returns two lookup tables for template substitution:
// textBindings (every param's HYGIENIC substitution text — parenthesized
// for value kinds "int"/"expr", bare for name kinds) and intBindings (raw
// integer values, for "int"-kind params only, used by targeting's
// range/max_targets which are compile-time constants rather than parsed
// expressions).
func bindAtom(rs *Ruleset, path string, idx int, atom *AtomDef, bind map[string]json.RawMessage, attrSet, resSet, defSet map[string]bool) (textBindings map[string]string, intBindings map[string]int, err error) {
	textBindings = make(map[string]string, len(atom.Params))
	intBindings = make(map[string]int, len(atom.Params))

	declared := make(map[string]bool, len(atom.Params))
	for _, p := range atom.Params {
		declared[p.Name] = true
	}

	// Sorted so a fixture with multiple unknown bind keys reports the same
	// one on every run (determinism — see this file's package doc).
	unknownKeys := make([]string, 0, len(bind))
	for k := range bind {
		if !declared[k] {
			unknownKeys = append(unknownKeys, k)
		}
	}
	if len(unknownKeys) > 0 {
		sort.Strings(unknownKeys)
		return nil, nil, fieldErr(path, fmt.Sprintf("compose[%d].bind.%s", idx, unknownKeys[0]), fmt.Sprintf("atom %q declares no param %q", atom.ID, unknownKeys[0]))
	}

	for _, p := range atom.Params {
		field := fmt.Sprintf("compose[%d].bind.%s", idx, p.Name)
		raw, ok := bind[p.Name]
		if !ok {
			return nil, nil, fieldErr(path, field, fmt.Sprintf("missing binding for atom %q's %q param (kind %q)", atom.ID, p.Name, p.Kind))
		}

		switch p.Kind {
		case "int":
			var n int
			if err := json.Unmarshal(raw, &n); err != nil {
				return nil, nil, fieldErr(path, field, fmt.Sprintf("must be an integer: %v", err))
			}
			if n < 0 {
				// Caught HERE, naming the bind field directly, rather than
				// left to surface as a cryptic parse error wherever this
				// value gets spliced (an int-kind binding is parenthesized
				// verbatim — "(-5)" — and the grammar has no unary minus,
				// so Parse would fail deep inside the substituted
				// expression with no indication which bind caused it).
				return nil, nil, fieldErr(path, field, fmt.Sprintf(`must not be negative (got %d): the expression grammar has no unary minus, so a negative int-kind binding cannot be spliced into an expression position. If you need a negative contribution here, use an "expr"-kind param instead and write the "0 - %d" idiom`, n, -n))
			}
			intBindings[p.Name] = n
			textBindings[p.Name] = "(" + strconv.Itoa(n) + ")"

		case "expr":
			var s string
			if err := json.Unmarshal(raw, &s); err != nil || s == "" {
				return nil, nil, fieldErr(path, field, "must be a non-empty string (an expression)")
			}
			if _, perr := Parse(s); perr != nil {
				return nil, nil, fieldErr(path, field, fmt.Sprintf("invalid expression: %v", perr))
			}
			textBindings[p.Name] = "(" + s + ")"

		case "attribute":
			name, berr := bindName(path, field, raw)
			if berr != nil {
				return nil, nil, berr
			}
			if !attrSet[name] {
				return nil, nil, fieldErr(path, field, fmt.Sprintf("references undeclared attribute %q", name))
			}
			textBindings[p.Name] = name

		case "resource":
			name, berr := bindName(path, field, raw)
			if berr != nil {
				return nil, nil, berr
			}
			if !resSet[name] {
				return nil, nil, fieldErr(path, field, fmt.Sprintf("references undeclared resource %q", name))
			}
			textBindings[p.Name] = name

		case "defense":
			name, berr := bindName(path, field, raw)
			if berr != nil {
				return nil, nil, berr
			}
			if !defSet[name] {
				return nil, nil, fieldErr(path, field, fmt.Sprintf("references undeclared defense %q", name))
			}
			textBindings[p.Name] = name

		case "condition":
			name, berr := bindName(path, field, raw)
			if berr != nil {
				return nil, nil, berr
			}
			if _, ok := rs.Conditions[name]; !ok {
				return nil, nil, fieldErr(path, field, fmt.Sprintf("references undeclared condition %q", name))
			}
			textBindings[p.Name] = name
		}
	}

	return textBindings, intBindings, nil
}

// bindName decodes a NAME-kind param binding: a non-empty string matching
// the expression grammar's IDENT charset (it must be substitutable as a
// bare identifier into a ref/name position).
func bindName(path, field string, raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return "", fieldErr(path, field, "must be a non-empty string (a declared name)")
	}
	if !isValidIdentName(s) {
		return "", fieldErr(path, field, fmt.Sprintf("%q must match the expression IDENT charset ^[A-Za-z_][A-Za-z0-9_]*$", s))
	}
	return s, nil
}

// buildAndSortDAG validates the composition's provides/consumes graph
// (spec §4: every consumed key provided by exactly one atom, acyclic) and
// returns its execution order — Kahn's algorithm, always picking the
// LOWEST not-yet-placed index with zero remaining in-degree, so ties break
// by composition list order deterministically (never load-bearing for
// correctness, only for output determinism — spec §4 guarantee).
func buildAndSortDAG(path string, atomInstances []*AtomDef, names []string) ([]int, error) {
	n := len(atomInstances)

	providerOf := map[string]int{}
	for i, a := range atomInstances {
		for _, k := range a.Provides {
			if prev, dup := providerOf[k]; dup {
				return nil, fieldErr(path, "compose", fmt.Sprintf("key %q is provided by more than one atom in this composition (%q at compose[%d] and %q at compose[%d])", k, names[prev], prev, names[i], i))
			}
			providerOf[k] = i
		}
	}

	adj := make([][]int, n)
	inDeg := make([]int, n)
	for i, a := range atomInstances {
		for _, k := range a.Consumes {
			p, ok := providerOf[k]
			if !ok {
				return nil, fieldErr(path, "compose", fmt.Sprintf("atom %q at compose[%d] consumes key %q, but no atom in this composition provides it", names[i], i, k))
			}
			adj[p] = append(adj[p], i)
			inDeg[i]++
		}
	}

	placed := make([]bool, n)
	order := make([]int, 0, n)
	for len(order) < n {
		pick := -1
		for i := 0; i < n; i++ {
			if !placed[i] && inDeg[i] == 0 {
				pick = i
				break
			}
		}
		if pick == -1 {
			return nil, fieldErr(path, "compose", "the provides/consumes graph has a dependency cycle")
		}
		placed[pick] = true
		order = append(order, pick)
		for _, j := range adj[pick] {
			inDeg[j]--
		}
	}
	return order, nil
}

// compileTargeting finds the composition's single targeting contribution
// (spec §4: exactly one required) and resolves its range/max_targets to
// concrete ints.
func compileTargeting(path string, atomInstances []*AtomDef, intBindings []map[string]int) (Targeting, error) {
	found := -1
	var contrib Contribution
	for i, a := range atomInstances {
		for _, c := range a.Contributes {
			if c.Kind != "targeting" {
				continue
			}
			if found != -1 {
				return Targeting{}, fieldErr(path, "compose", fmt.Sprintf("more than one targeting contribution in this composition (compose[%d] and compose[%d]) — exactly one is required", found, i))
			}
			found = i
			contrib = c
		}
	}
	if found == -1 {
		return Targeting{}, fieldErr(path, "compose", "no atom in this composition contributes targeting — every ability needs exactly one")
	}

	rng, err := resolveIntField(contrib.RangeSrc, intBindings[found])
	if err != nil {
		return Targeting{}, fieldErr(path, fmt.Sprintf("compose[%d]", found), fmt.Sprintf("targeting.range: %v", err))
	}
	if rng < 0 {
		return Targeting{}, fieldErr(path, fmt.Sprintf("compose[%d]", found), "targeting.range must not be negative")
	}
	maxT, err := resolveIntField(contrib.MaxTargetsSrc, intBindings[found])
	if err != nil {
		return Targeting{}, fieldErr(path, fmt.Sprintf("compose[%d]", found), fmt.Sprintf("targeting.max_targets: %v", err))
	}
	if maxT < 1 {
		return Targeting{}, fieldErr(path, fmt.Sprintf("compose[%d]", found), "targeting.max_targets must be at least 1")
	}
	return Targeting{Range: rng, MaxTargets: maxT}, nil
}

// resolveIntField resolves a targeting RangeSrc/MaxTargetsSrc value
// (either decimal literal text, or a whole-field "{param}" placeholder —
// see checkIntOrPlaceholderField) to a concrete int using this compose
// entry's int-kind bindings.
func resolveIntField(src string, intBindings map[string]int) (int, error) {
	if placeholderWholeFieldRe.MatchString(src) {
		name := src[1 : len(src)-1]
		v, ok := intBindings[name]
		if !ok {
			return 0, fmt.Errorf("placeholder {%s} does not resolve to an int-kind bound value", name)
		}
		return v, nil
	}
	return strconv.Atoi(src)
}

// compileResolution finds the composition's resolution contribution, if
// any (spec §4: at most one), splices bindings into its roll/vs templates,
// and parses + position-validates the result. RollSrc/VsSrc (Task 3) are
// set to the same fully-substituted text (rollSub/vsSub) Roll/Vs are
// parsed from — v2's testimony text is its post-splice source, unlike a
// v1-adapted ability's (see CompiledResolution's doc comment).
func compileResolution(path string, atomInstances []*AtomDef, textBindings []map[string]string, attrOrDefSet, resSet map[string]bool) (resolution *CompiledResolution, key string, err error) {
	found := -1
	var contrib Contribution
	for i, a := range atomInstances {
		for _, c := range a.Contributes {
			if c.Kind != "resolution" {
				continue
			}
			if found != -1 {
				return nil, "", fieldErr(path, "compose", fmt.Sprintf("more than one resolution contribution in this composition (compose[%d] and compose[%d]) — v2.0 allows at most one resolution per composition", found, i))
			}
			found = i
			contrib = c
		}
	}
	if found == -1 {
		return nil, "", nil
	}

	rollSub := substTemplate(contrib.RollSrc, textBindings[found])
	vsSub := substTemplate(contrib.VsSrc, textBindings[found])

	rollExpr, rerr := compileTwoActorExpr(path, fmt.Sprintf("compose[%d].resolution.roll", found), rollSub, attrOrDefSet, resSet)
	if rerr != nil {
		return nil, "", rerr
	}
	vsExpr, verr := compileTwoActorExpr(path, fmt.Sprintf("compose[%d].resolution.vs", found), vsSub, attrOrDefSet, resSet)
	if verr != nil {
		return nil, "", verr
	}

	return &CompiledResolution{
		Roll: rollExpr, RollSrc: rollSub,
		Vs: vsExpr, VsSrc: vsSub,
		Branches: contrib.Branches,
	}, contrib.Key, nil
}

// compileEffects splices bindings into and validates one outcome
// contribution's effect templates, producing v1-shaped Outcomes.
func compileEffects(path string, atomIdx, contribIdx int, templates []EffectTemplate, bindings map[string]string, rs *Ruleset, attrOrDefSet, resSet map[string]bool) ([]Outcome, error) {
	out := make([]Outcome, 0, len(templates))
	for ei, t := range templates {
		field := fmt.Sprintf("compose[%d].contributes[%d].effects[%d]", atomIdx, contribIdx, ei)
		switch t.Kind {
		case OutcomeResourceChange:
			resourceName := substTemplate(t.ResourceSrc, bindings)
			if !resSet[resourceName] {
				return nil, fieldErr(path, field+".resource_change.resource", fmt.Sprintf("references undeclared resource %q", resourceName))
			}
			deltaSub := substTemplate(t.DeltaExprSrc, bindings)
			deltaExpr, err := compileTwoActorExpr(path, field+".resource_change.delta_expr", deltaSub, attrOrDefSet, resSet)
			if err != nil {
				return nil, err
			}
			out = append(out, Outcome{
				Kind: OutcomeResourceChange,
				ResourceChange: &ResourceChangeOutcome{
					Resource: resourceName, DeltaExpr: deltaExpr, DeltaExprSrc: deltaSub,
				},
			})

		case OutcomeApplyCondition:
			id := substTemplate(t.ApplyConditionSrc, bindings)
			if _, ok := rs.Conditions[id]; !ok {
				return nil, fieldErr(path, field+".apply_condition.id", fmt.Sprintf("references undeclared condition %q", id))
			}
			out = append(out, Outcome{Kind: OutcomeApplyCondition, ApplyCondition: &ApplyConditionOutcome{ID: id}})

		case OutcomeRemoveCondition:
			id := substTemplate(t.RemoveConditionSrc, bindings)
			if _, ok := rs.Conditions[id]; !ok {
				return nil, fieldErr(path, field+".remove_condition.id", fmt.Sprintf("references undeclared condition %q", id))
			}
			out = append(out, Outcome{Kind: OutcomeRemoveCondition, RemoveCondition: &RemoveConditionOutcome{ID: id}})
		}
	}
	return out, nil
}

// substTemplate replaces every "{name}" placeholder in raw with
// bindings[name] — the hygienic splice itself. For a value-kind param
// (int/expr), bindings[name] is ALREADY parenthesized ("(" + text + ")"),
// so the substitution result is textual, but the parens guarantee the
// eventual Parse() below produces the same AST as splicing a pre-parsed
// subtree directly would: a binding of "1 + 1" spliced into "2 * {p}"
// yields "2 * (1 + 1)", which parses to BinOp(*, 2, BinOp(+, 1, 1)) —
// evaluates to 4, never 3, regardless of what operator surrounds the
// placeholder (TestCompileHygienicSplice pins this exact case). For a
// name-kind param, bindings[name] is a bare identifier — substituting it
// unparenthesized into a ref position ("@caster.{attack_stat}") or a name
// field ("resource": "{pool}") is correct precisely because it is NEVER
// itself a sub-expression.
//
// Injection is impossible by construction, not just by convention: every
// value-kind binding was independently validated to Parse() as a complete
// standalone expression BEFORE this substitution runs (bindAtom's "expr"
// case; "int" bindings are decoded straight from a JSON number, never
// free text) — a string with unbalanced parens or trailing garbage fails
// that check and never reaches here, so wrapping it in "(" ")" can never
// let it "escape" into the surrounding template text. Every name-kind
// binding was independently validated to match the IDENT charset
// (bindName) — no delimiter characters, all sigil/scope-position meaning
// stays exactly where the template placed it. There is no scenario in
// which a crafted param value changes the shape of the surrounding
// expression beyond substituting its own value.
func substTemplate(raw string, bindings map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(raw, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := bindings[name]; ok {
			return v
		}
		// Unreachable given loadAtoms' checkPlaceholders (every placeholder
		// in an atom's templates is validated against its declared params)
		// and bindAtom (every declared param is validated bound) running
		// before this is ever called — left as a visible no-op rather than
		// a panic so a future bug here fails a parse/cross-ref check loudly
		// downstream instead of crashing Load.
		return m
	})
}

// compileTwoActorExpr parses a fully-substituted resolution roll/vs or
// outcome delta_expr template and validates it for a two-actor position
// (spec v2 §5): every ref must carry an explicit scope (bare ref in a
// two-actor position — validation catalogue), and every ref's name must
// be declared. Attribute refs ('@') are checked against attrOrDef (the
// union of the ruleset's attributes and defenses: Task 1's EvalContext has
// only Attrs/Resources, no separate defense map, so a defense's runtime
// value is exposed through the same '@' namespace attributes use — see
// compileCompositions' attrOrDefSet comment); resource refs ('#') against
// resSet.
func compileTwoActorExpr(path, field, substituted string, attrOrDef, resSet map[string]bool) (*Expr, error) {
	e, err := Parse(substituted)
	if err != nil {
		return nil, fieldErr(path, field, err.Error())
	}
	for _, r := range e.ScopedRefs() {
		if r.Scope == ScopeNone {
			return nil, fieldErr(path, field, fmt.Sprintf("bare reference %c%s not allowed here: this is a two-actor position, write %c%s.%s or %c%s.%s", r.Sigil, r.Name, r.Sigil, scopeName(ScopeCaster), r.Name, r.Sigil, scopeName(ScopeTarget), r.Name))
		}
		switch r.Sigil {
		case '@':
			if !attrOrDef[r.Name] {
				return nil, fieldErr(path, field, fmt.Sprintf("references undeclared attribute %q", r.Name))
			}
		case '#':
			if !resSet[r.Name] {
				return nil, fieldErr(path, field, fmt.Sprintf("references undeclared resource %q", r.Name))
			}
		}
	}
	return e, nil
}
