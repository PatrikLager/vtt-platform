# Format v2 — Composition Layer Implementation Plan (sub-project 5c)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver format v2 per the approved spec (`docs/superpowers/specs/2026-07-25-format-v2-composition-design.md`): scoped refs + expression-sized dice in the grammar, ruleset-authored atoms with declared provides/consumes compiled at load into the proven flat execution form, conformance extended with compiled-form goldens, and tavern-brawl rebuilt on its own v2 atoms.

**Architecture:** ALL changes inside `internal/rules` (+ its schema docs + conformance) and `rulesets/tavern-brawl`. Engine, contract, store, campaign, gateway, harness, MCP, cmd: FROZEN — the wire never learns v2 happened. Resolve executes CompiledPower only; it never sees atoms.

**Tech Stack:** existing; stdlib-only stays (5a controller decision stands).

## Global Constraints

- Branch `feat/format-v2-composition` from main. Adapted review-before-commit flow; reviewers READ-ONLY; injections in throwaway rsync copies only.
- The untracked `rulesets/dnd45e-minimal/` in the working tree belongs to PAUSED sub-project 5b — never stage, commit, or delete it in 5c.
- ADR-009 binding: stub-first behavioral RED; injection proofs for after-the-fact tests; boundary behavior only.
- Spec §2 reusability guarantees are binding: ZERO atoms in platform code; vocabulary gate (incl. bare `hp`) scans everything as before; no game-system word in `internal/`.
- No new implicit contracts (spec §2.4): params splice as parsed subtrees; composition order derived from the DAG; scope rules are load errors, never conventions.
- `format_version: "1"` rejected with a clear error once v2 lands (clean break; Task 4 migrates tavern-brawl in the same branch so main never has a rejected committed ruleset — Tasks 2–4 land as commits on the BRANCH; the gate that must stay green on every commit is `go test ./internal/rules/...` scoped until Task 4 restores the full `task check` (glob test needs a loadable tavern-brawl). Task ordering inside the branch handles this: Task 2 rejects v1 ONLY behind a compile path that Task 4's migration completes; run full `task check` at Task 4 and Task 5.
  BINDING simplification: Tasks 1–3 keep v1 loading INTACT and add v2 alongside; the v1-rejection flip is Task 4's first step, immediately followed by the migration — full `task check` green at every commit point after all.
- Pinned counts (property/soak/scenario library) byte-identical throughout; tavern-brawl batch goldens change ONLY where scoping syntax appears in recorded expression strings (each such change listed in the Task 4 report with before/after).

## Shared interfaces (binding across tasks)

```go
// expr.go (Task 1)
type Scope int // ScopeNone | ScopeCaster | ScopeTarget — recorded on every ref node
type EvalContext struct{ Attrs, Resources map[string]int }
// Two-actor positions:
func EvalScoped(e *Expr, caster, target EvalContext, r Roller) (int, error)
// Single-actor positions (thresholds, default_max_expr) — unchanged signature, bare refs only:
func Eval(e *Expr, attrs, resources map[string]int, r Roller) (int, error)
// New introspection (load-time position validation):
func (e *Expr) Scopes() []Scope      // every ref's scope, for position rules
func (e *Expr) HasDice() bool        // unchanged; expression-sized dice count as dice

// format.go / compile (Task 2)
type AtomDef struct{ ID string; Params []ParamDef; Provides, Consumes []string; Contributes []Contribution }
type ParamDef struct{ Name, Kind string } // kinds: int|expr|attribute|resource|defense|condition
type Contribution struct{ Kind string; /* targeting|resolution|outcome fields per spec §4 */ }
type CompiledPower struct {
    ID, Name  string
    Usage     Usage            // v1 type unchanged
    Targeting Targeting        // v1 type unchanged
    Resolution *CompiledResolution // nil for non-attack compositions
    BranchOutcomes [2][]Outcome    // aligned with Resolution.Branches; zero-valued when Resolution nil
    Effects   []Outcome            // unconditional ("always") outcomes
}
type CompiledResolution struct{ Roll, Vs *Expr; Branches [2]string }
// Ruleset (Task 2): gains Compiled map[string]*CompiledPower; v2 abilities compile at Load
```

---

### Task 1: Grammar v2 — scoped refs + expression-sized dice

**Files:** `internal/rules/{expr.go,expr_test.go,expr_fuzz_test.go}`.

Parser: `ref := ('@'|'#') (scope '.')? IDENT` with scopes exactly `caster`/`target` (anything else before `.` = parse error naming position); scope recorded on the node; the parser ACCEPTS scopes anywhere — position legality is the loader's job (Task 2). `DICE := factor 'd' factor` — count/sides may be any factor (INT keeps its parse-time bounds fast-path; non-literal factors defer bounds to eval: count 1..100, sides 1..1000, clean rules error naming the computed values). Precedence: `d` binds at factor level — `1d6+2` ≡ `(1d6)+2`, `2*3d6` ≡ `2*(3d6)`; pin with tests. `EvalScoped` resolves `@caster.x`/`#target.y` against the matching context; BARE refs in EvalScoped are an eval error (defense-in-depth — the loader should have rejected them); scoped refs in single-context `Eval` likewise error. `Scopes()` exposes ref scopes for Task 2's position validation. Grammar doc comment updated verbatim (spec §5). Fuzz corpus extended (scoped refs, expr dice, `xd`/`d6`/`caster.`/nested-paren-dice edge cases); 30s run clean.

- [ ] RED: exhaustive table tests for every new production/precedence/error case + EvalScoped scope-resolution matrix (caster attr, target resource, mixed expr, bare-ref error, wrong-context error) — fail against stubs.
- [ ] GREEN; `go test ./internal/rules/... -race -count=1`; fuzz 30s; v1 expressions still parse/eval byte-identically (regression suite untouched).
- [ ] Commit: `feat: grammar v2 — scoped refs and expression-sized dice`

### Task 2: Atoms, compositions, compile-at-load

**Files:** `internal/rules/{format.go,load.go,compile.go,schema/atom.schema.json,schema/ability.schema.json,load_test.go,compile_test.go,schema_test.go}` (+ testdata fixtures).

v2 detection: manifest `format_version` "2" → v2 loading (atoms/ + composition abilities); "1" continues to load UNCHANGED (flip happens in Task 4). Atom schema per spec §4 verbatim (params/provides/consumes/contributes; closed kinds). Validation (each with a focused invalid fixture + file+field error): unknown param kind; binding kind mismatch (attribute name not in manifest, etc.); unsatisfied consume; doubly-provided key; cycle; zero/duplicate targeting; resolution branch count ≠ 2; outcome branch not in resolution's labels; `always` outcome with non-null key; scope-position violations (bare ref in two-actor position, scoped ref in threshold — uses Task 1's `Scopes()`); param placeholder `{name}` unknown; string-substitution injection impossible (hygienic-splice test: bind expr `1 + 1` into `2 * {p}` → compiled expr equals `2 * (1 + 1)` = 4, NOT 3 — the load-bearing anti-implicit-contract test). Compile: topo-sort (ties by list order), flatten to CompiledPower, deterministic (two loads → deep-equal compiled output; map-iteration leak test). Schema docs extended; nested-required anti-drift walker covers atom.schema.json automatically (5a fix wave's walker — verify it picks the new file up, extend the test's file list if explicit).

- [ ] RED-first throughout (behavioral: Load of a valid v2 fixture returns compiled powers with exact expected flattened content — hand-derived in the test).
- [ ] `go test ./internal/rules/... -race -count=1` green (full task check still green because v1 path untouched).
- [ ] Commit: `feat: atoms and compile-at-load — compositions flatten to the proven execution form`

### Task 3: Resolve executes CompiledPower + conformance compiled-form goldens

**Files:** `internal/rules/{resolve.go,resolve_test.go}`, `internal/rules/conformance/{conformance.go,conformance_test.go}` (+ testdata).

Resolve: consumes CompiledPower (v1 abilities compile trivially into the same struct at load — ONE execution path, no dual code; the v1→CompiledPower adapter is part of this task and pins v1 behavior byte-identically via the existing tavern-brawl batch goldens). Deltas: `Vs` evaluated via EvalScoped (may roll; recorded like all rolls, ordering pinned: roll expr first, then vs expr, per target); branch labels flow into outcome_summary verbatim (format: existing summary shape with the ruleset's label word); EvalScoped everywhere two actors exist, Eval for thresholds (unchanged). All 5a semantics preserved: event ordering contract, usage-spend leading, threshold change-order, idempotent conditions, duplicate-target rejection, int32 checks (now on EvalScoped results too). Conformance: compiled-form goldens (`goldens/compiled/<ability>.json`, deep-equal against a canonical JSON serialization of CompiledPower) enforced per ability for v2 rulesets (v1 rulesets exempt until none exist); dump helper for authoring; missing/mismatched compiled golden → named failure.

- [ ] RED-first; all existing resolve/conformance/tavern-brawl(v1) tests stay green — the adapter proves zero observable change.
- [ ] `go test ./... -race -count=1` + full `task check` green.
- [ ] Commit: `feat: Resolve on compiled powers + compiled-form conformance goldens`

### Task 4: v1 sunset + tavern-brawl rebuilt on its own atoms

**Files:** `internal/rules/load.go` (v1 rejection: `format_version "1"` → clear error pointing at the v2 spec), `rulesets/tavern-brawl/**` (atoms/ added; abilities rewritten as compositions; goldens migrated incl. compiled/), invalid-fixture updates in testdata (v1 fixtures become the v2 equivalents; keep one explicit `format_version "1" rejected` fixture).

Tavern-brawl atoms are ITS OWN (drink mechanics — e.g. `brawl-delivery`, `footing-contest`, `drink-soak`; NO 4e names, zero atoms shared with future dnd45e): behavior-preserving where mechanics unchanged — batch goldens differ ONLY in recorded expression strings (scoping syntax); each diff listed in the report with before/after and why. Compiled goldens authored for all four abilities. The internal/rules testdata/valid fixture migrates to v2 the same way (its goldens likewise).

- [ ] RED: flip v1-rejection first → conformance glob + testdata suites FAIL (tavern-brawl unloadable) — that failure is the RED for the migration.
- [ ] Migrate; GREEN; full `go test ./... -race -count=1` + `task check` green ×2; scenario library (toy-brawl.json runs the MIGRATED ruleset over the wire — its scenario steps unchanged, proving observable behavior held).
- [ ] Commit: `feat: tavern-brawl on v2 atoms — v1 sunset, zero shared atoms, wire behavior identical`

### Task 5: Workflow-level final review (Patrik's standing preference) + fix wave → merge gate

Lenses: (1) compile-correctness (hand-flatten compositions independently, verify compiled goldens + hygienic splicing + determinism); (2) grammar/eval correctness incl. scope matrix and dice-expr bounds; (3) reusability-guarantee audit (spec §2: zero atoms in platform, vocabulary sweep, no new implicit contracts, residue list honest); (4) test-integrity per ADR-009 (injection-proof sampling, golden honesty, v1-behavior-preservation evidence). Dedup → 3-refuter adversarial verify → triage. Fix wave → opus verify → merge gate to Patrik. Merge bundle: 5a spec amendment note (superseded-by-v2 pointer), memory update, 5b resume plan (rebase its branch, update its plan's expressions to v2 syntax).
