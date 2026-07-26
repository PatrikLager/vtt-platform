# Ruleset Format v2 — Composition Layer (sub-project 5c)

**Date:** 2026-07-25
**Status:** Approved design (brainstorming output)
**Parent:** 5a spec (format v1, interpreter); supersedes format v1.
**Trigger:** 5b's honest-stop — v1 outcome expressions evaluate `@`-refs
against the target only, so "weapon die + wielder's strength" is
inexpressible without entangling ability data with statblock instances.
Patrik's ruling: build the general mechanism, not the easy patch.
**Inspiration (not template):** RPTool's template system
(`Documentation/template-system.md`, `rule-framework/data/templates/`) —
abilities as compositions of atomic statements. Its strengths are kept;
its implicit contracts (shared-variable coupling, load-bearing array
order, `t`-prefix scoping, data-as-scripting) are the named enemies.

## 1. Purpose

Abilities become COMPOSITIONS of ruleset-authored ATOMS with declared,
load-time-validated contracts. The composition graph is flattened at load
into the small declarative execution form 5a proved (compile-at-load);
runtime references (current hp, caster stats) stay live in expressions.
The interpreter grows only: scoped refs, roll-vs-value resolution,
expression-sized dice. Engine, contract, gateway, harness, MCP: UNTOUCHED.

## 2. Binding reusability guarantees (the point of 5c)

1. **The platform ships the mechanism, ZERO atoms.** Atom definitions are
   ruleset content. No atom name, kind, or game concept appears in
   `internal/` — if two rulesets both want a "weapon" idea they each
   declare their own atoms. Semgrep vocabulary gate extends unchanged.
2. **P4 proof:** two rulesets — rebuilt `tavern-brawl` and (in 5b)
   `dnd45e-minimal` — share ZERO atoms, have different mechanics, and
   pass the same untouched conformance suite.
3. **Honest residue:** anything still shaped like one game tradition is
   listed in §10, not passed off as generic.
4. **No new implicit contracts:** every inter-atom relationship is
   declared (provides/consumes), every ref is scoped where two actors are
   in play, params splice as parsed subtrees (never string substitution).
   Execution order is DAG-derived; among DAG-independent atoms,
   composition list order breaks ties, and that tie order is
   semantically meaningful — not cosmetic — when tied outcomes touch the
   same clamped resource (non-commutative under floor/cap), so authors
   order ties deliberately; the order is deterministic and pinned by
   compiled goldens (corrected 2026-07-26, final review: the original
   "never load-bearing" claim was demonstrably false).

## 3. Format v2 layout

```
rulesets/<id>/
  ruleset.json      manifest: id, name, format_version "2",
                    attributes, defenses, resources+thresholds (as v1)
  atoms/*.json      NEW — the ruleset's atomic statements
  abilities/*.json  now compositions: atom refs + param bindings
                    (+ the unchanged plain fields: id, name, usage)
  conditions/*.json unchanged
  goldens/*.json    unchanged format, plus compiled-form goldens (§8)
  guide.md          unchanged role
```
`format_version: "1"` is REJECTED with a clear error (clean break: both
existing rulesets are in-repo and migrate; there are no external authors
yet). v1's plain-field decisions survive: `usage` stays a plain ability
field (RPTool lesson: usage-as-template was noise), targeting lives in a
delivery atom's contribution, `reliable`-style flags stay out (YAGNI).

## 4. Atoms

```json
{
  "id": "weapon-strike-roll",
  "params": [
    {"name": "attack_stat", "kind": "attribute"},
    {"name": "prof", "kind": "int"}
  ],
  "provides": ["strike"],
  "consumes": [],
  "contributes": [
    {
      "kind": "resolution",
      "key": "strike",
      "roll": "1d20 + @caster.{attack_stat} + {prof}",
      "vs": "@target.ac",
      "branches": ["hit", "miss"]
    }
  ]
}
```

- **Param kinds** (closed set): `int`, `expr`, `attribute`, `resource`,
  `defense`, `condition`. Bindings are validated against the manifest's
  declarations at load. `expr`/`int` params splice into contribution
  expressions as parenthesized parsed subtrees (hygienic — a binding of
  `1 + 1` can never change surrounding precedence); name-kinds substitute
  into ref positions only. Splice hygiene: a placeholder may not occupy
  scope position (directly between a sigil and `.`); the words `caster`
  and `target` are reserved and may not be declared as attribute,
  defense, or resource names, or condition ids.
- **provides/consumes:** plain strings, ruleset-chosen. A composition is
  valid iff every consumed key is provided by exactly one atom in the
  composition, the graph is acyclic, and every contribution's `key`
  refers to a key the atom provides or consumes; a provides/consumes edge
  that the fixed execution phase order (branch outcomes, then
  always-effects) cannot honor — e.g. an always-outcome atom providing a
  key a branch-outcome atom consumes — is a LOAD ERROR, rejected like a
  cycle. Execution order is the topological order of the graph; among
  DAG-independent atoms, ties break by composition list order, and that
  tie order is semantically meaningful — when tied outcomes touch the
  same clamped resource, the result is non-commutative under floor/cap,
  so authors order ties deliberately. The order is deterministic and
  pinned by compiled-form goldens (corrected 2026-07-26, final review:
  the original "never load-bearing" claim was demonstrably false).
- **Contribution kinds** (closed set, the execution vocabulary):
  - `targeting` — `{range, max_targets}` (exactly one per composition).
  - `resolution` — `{key, roll, vs, branches: [<ge-label>, <lt-label>]}`:
    roll expression compared to a threshold EXPRESSION (`vs`); total >=
    threshold selects the first label, else the second. Labels are
    ruleset-chosen words; they appear in testimony. At most one
    resolution per provided key; v2.0 allows one resolution per
    composition (multi-resolution sequences are §10 residue).
  - `outcome` — `{key, branch, effects: [...]}` where effects are v1's
    outcome list (`resource_change` / `apply_condition` /
    `remove_condition`, now with scoped exprs). `branch` names a branch
    of the resolution provided under `key`; `branch: "always"` with
    `key: null` contributes unconditional effect-phase outcomes
    (non-attack abilities need no resolution atom at all).

An ability composition:

```json
{
  "id": "longsword-strike",
  "name": "Longsword Strike",
  "usage": "at_will",
  "compose": [
    {"atom": "melee-delivery", "bind": {"reach": 1}},
    {"atom": "weapon-strike-roll", "bind": {"attack_stat": "str", "prof": 3}},
    {"atom": "weapon-damage", "bind": {"die": "1d8", "damage_stat": "str"}}
  ]
}
```

## 5. Expression changes (grammar v2)

- **Scoped refs:** `ref := ('@'|'#') (scope '.')? IDENT`, scopes `caster`
  and `target`. In two-actor positions (resolution `roll`/`vs`, outcome
  effect expressions) a scope is REQUIRED — bare refs are a load error
  naming file+field ("ambiguous: write @caster.str or @target.str").
  In single-actor positions (threshold `when`, `default_max_expr`)
  scopes are FORBIDDEN — bare refs mean the resource's owner, as v1.
- **Expression-sized dice:** `DICE := factor 'd' factor` — count and
  sides may be parenthesized expressions (`(@caster.weapon_count)d(@caster.weapon_die)`),
  enabling weapon-categories-as-actor-data with no platform equipment
  concept. Bounds (count 1..100, sides 1..1000) move to EVAL time with a
  clean rules error; plain-integer dice keep their parse-time check too.
  Dice remain banned in threshold `when`/`default_max_expr`.
- Everything else (operators, functions, int32 value contract, no unary
  minus, no comparison operators, depth cap, recorded rolls) unchanged.

## 6. Compile-at-load

`Load(dir)` for v2: load manifest/conditions/atoms/compositions →
validate (schemas; param kinds vs manifest; provides/consumes DAG;
exactly-one targeting; branch refs exist; scope rules; cross-refs as v1;
every expression parsed) → FLATTEN each composition into a **compiled
power**: `{id, name, usage, targeting, resolution {roll, vs, branches},
branch-labeled outcome lists, effect list}` — the 5a execution shape
plus labels and vs-expression. Resolve() executes compiled powers ONLY;
it never sees atoms. Rejections and event emission semantics are
unchanged from 5a (ordering contract, testimony, threshold evaluation,
duplicate-target rejection, idempotent condition outcomes, int32 checks).

The compiled power is an inspectable artifact (RPTool's `/power-debug`
lesson): conformance can dump it, and compiled-form goldens pin it (§8).

## 7. Interpreter deltas (internal/rules only)

Eval takes caster AND target contexts (attrs+resources each); scope
resolution per §5. `vs` evaluates like any two-actor expression (it may
roll dice; recorded like all rolls). Branch labels flow into
`AbilityUsed.outcome_summary` verbatim (testimony speaks the ruleset's
words). Engine folds, contract messages, gateway authz/handler, harness,
MCP: zero changes — the wire never learns v2 happened.

## 8. Conformance extensions

Unchanged: load + per-ability smoke + per-ability exact-batch goldens
(the enforcement from the 5a fix wave stays). New: **compiled-form
goldens** — `goldens/compiled/<ability>.json` pins each composition's
flattened form; the suite fails on drift (a refactor of atoms that
changes a compiled power is a visible, reviewed event). Both existing
rulesets carry them.

## 9. Migration & sequencing

5c rebuilds `rulesets/tavern-brawl` on v2 atoms (its own atoms — drink
mechanics, no 4e shapes) with identical observable behavior where
mechanics are unchanged (batch goldens updated only where scoping
syntax requires). Then 5b RESUMES on v2: dnd45e-minimal authored as
atoms+compositions (the four blocked abilities become the proof the gap
is closed — `@caster.str` damage), its 9 finished golden derivations
carried over with syntax updates. The 5b demo-rematch merge gate is
unchanged and now gates both.

## 10. Non-goals and named residue (honest accounting)

- **Modifier contributions** (BloodyBonus-style: an atom adding terms to
  another atom's roll) — residue; needed for real 4.5e feats/CA; design
  candidate: `modifies: {key, slot}` contribution kind.
- **Multi-resolution sequences / secondary attacks** — residue.
- **Degrees of success** (>2 branches) — residue; branch list is already
  a list, the comparison semantics are what's binary today.
- **Condition mechanical effects queried at resolution** (RPTool's
  GrantsCombatAdvantage idea) — residue, wants the modifier mechanism.
- **Equipment swap commands / structured loadout data** — not a platform
  concept; actor attributes already carry weapon data for v2 dice-exprs;
  a dedicated update-actor command is world-layer work.
- **Reactions/interrupts, turn engine, areas, healing systems** — as 5a.
- **Comparison-direction is fixed:** `total >= threshold` always selects
  the first branch label (ties favor the first/ge label — the d20-family
  convention); traditions requiring a strict must-exceed comparison need
  a threshold+1 workaround that alters the wire-visible `vs` text —
  named residue for a future comparison-direction field.
- The `hit/miss`-shaped binary resolution itself is generic
  roll-vs-threshold, but single-resolution-per-ability is a real
  constraint listed above.

## 11. Open questions (deferred, with owners)

- Modifier-contribution design (slot addressing, stacking rules) — with
  the first ruleset that needs feats/CA (4.5e full, post-5b).
- Compiled-power caching/versioning if rulesets ever hot-reload — with
  the hot-reload feature (explicitly out of scope since 5a).
- Cross-ruleset atom libraries ("share my atoms as a package") — only if
  a real second author appears; guarded against baking in by guarantee #1.

## 12. Amendments (2026-07-26, merge gate)

1. **Defenses are valued in the actor's attribute map:** declared under
   the manifest's `defenses` list, defense values are read the same way
   attributes are — `@scope.<name>` — not through a separate lookup;
   attribute/defense name collisions are rejected at load.
2. **Negative int-kind bindings are rejected:** an `int`-kind param bound
   to a negative literal is a load error; rulesets needing a negative
   constant use the `expr`-kind `0 - N` idiom (consistent with the
   no-unary-minus grammar rule).
3. **`compose` entries require `bind`:** every composition entry must
   carry a `bind` object, even when empty (`{}`) — no implicit omission.
4. **`AbilityUsed.rolls[].expression` records SourceText:** in v2 this is
   the post-splice text (the expression as spliced, not the pre-splice
   atom source) — testimony speaks the author's written expression while
   scoped semantics execute underneath it.
5. **Compiled-form goldens use the canonical serialization:** stable
   field order, SourceText (not AST) for expressions; orphaned
   compiled-golden files (no corresponding ability) fail conformance.
6. **Branch labels `always`, `usage`, and `effect` are reserved** and may
   not be used as ruleset-chosen resolution branch labels.
7. **`vs` expressions may roll dice:** the roll, when present, is
   recorded between the resolution roll and the outcome rolls, per
   target.
