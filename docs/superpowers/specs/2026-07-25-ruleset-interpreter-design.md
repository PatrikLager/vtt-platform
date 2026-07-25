# Ruleset Loader & Rules Interpreter — Design Spec (sub-project 5a)

**Date:** 2026-07-25
**Status:** Approved design (brainstorming output)
**Parent:** Platform spec §4.6 (there called "rule modules" — terminology
amended by this spec, see §2.4); pillars P2 (rules as data; vocabulary ban)
and P4 (boundary proven by a toy second ruleset); ADR-007 note (module/
ruleset CONTENT schemas are JSON Schema); ADR-009.

## 1. Purpose

The game returns. A RULESET is a data package (JSON + a guide) defining a
game system's mechanics; `internal/rules` is the ONE generic interpreter
that executes any conformant ruleset. 5a delivers the format, the loader,
the interpreter, the atomic outcome path, and the toy ruleset that proves
P4. 5b (own cycle) authors `dnd45e-minimal` as pure data.

## 2. Decisions (locked in brainstorming)

1. **v1 slice = combat-minimal:** stat blocks (resources/defenses via the
   Actor's open maps), attack abilities (roll vs a named defense, damage
   expression, at-will or limited usage), resource change with
   module-declared thresholds (e.g. half → a condition, zero → another),
   and simple named conditions with DM-ended durations. NO turn engine,
   opportunity actions, forced movement, areas, or healing mechanics.
2. **Atomicity = AppendBatch:** one rules action's full outcome (testimony
   + resource changes + conditions) persists all-or-nothing in one store
   transaction with contiguous sequences. New campaign/store capability;
   data-integrity-core review depth.
3. **Dice are rolled once, recorded forever:** the interpreter rolls
   (injectable RNG — crypto-seeded in production, fixed-seed in tests);
   events carry results; replay never re-rolls.
4. **Three-layer terminology (Patrik's correction, binding):**
   RULESET (the rules library — load once) ≠ ADVENTURE (content packages
   like Temple of Elemental Evil, written FOR a ruleset — future format,
   designed with the world layer; loading one compiles to ordinary setup
   events) ≠ CAMPAIGN LOG (play). The umbrella spec's "rule module" term
   is amended to "ruleset" at this sub-project's merge gate. Statblock
   FORMAT is defined by the ruleset schema; statblock INSTANCES are
   content (5a tests use fixtures; adventures bring their own; rulesets
   MAY ship a reference bestiary).

## 3. Generic vocabulary — contract additions (all additive)

New command: `UseAbility{actor_id, ability_id, target_ids[]}` (+ oneof tag,
auto-appearing MCP tool via the tools pipeline).
New events (+ oneof tags): `AbilityUsed{actor_id, ability_id, target_ids[],
rolls[]{expression, results[], total}, outcome_summary}` (testimony);
`ResourceChanged{actor_id, resource, delta, new_value, reason}`;
`ConditionApplied{actor_id, condition_id, source}`;
`ConditionRemoved{actor_id, condition_id, reason}`.
Engine folds the three state-bearing events GENERICALLY: resource
arithmetic on Actor's open `resources` map (clamped at 0; max from the
resource's declared max when present in the actor's resources entry);
a new generic `Conditions []ActorCondition` per actor in State.
No game-system word appears anywhere in platform code (semgrep-enforced);
none of the new messages embed Struct/Value (toolgen backlog stays
dormant). A new command `RemoveCondition{actor_id, condition_id}` covers
the DM-ended durations of the v1 slice.

## 4. Ruleset format v1

Directory: `rulesets/<id>/` containing
- `ruleset.json` — manifest: id, name, format_version ("1"), resource
  definitions `[{name, default_max_expr?, thresholds: [{when: expr,
  apply_condition, remove_when_false: bool}]}]`, defense stat names,
  attribute names.
- `abilities/*.json` — id, name, usage (`at_will` | `{limited: {resource,
  cost}}`), targeting `{range: int, max_targets: int}`, attack
  `{roll: expr, vs: defense_name}` (optional — non-attack abilities
  allowed), `hit`/`miss`/`effect` outcome lists — each outcome one of
  `{resource_change: {resource, delta_expr}}` or `{apply_condition: {id}}`
  or `{remove_condition: {id}}`.
- `conditions/*.json` — id, name, description (mechanical effects beyond
  bookkeeping are v2; v1 conditions are tracked markers the DM narrates).
- `guide.md` — the LLM affordances: how to run this system, served via a
  new `get_ruleset_guide` MCP tool.
- Expressions: dice (`NdM`), integers, `+ - * /`, `floor/max/min`,
  attribute refs (`@str`), resource refs (`#hp`), `half(@x)` sugar —
  the FULL grammar is closed and documented; unknown identifiers are
  load-time errors against the manifest's declared names.
All files validated at load against platform-owned JSON Schemas (the
format schemas live in `internal/rules/schema/*.json`, embedded).
Strict decoding; errors name file + field.

## 5. Interpreter (`internal/rules`)

- `Load(dir) (*Ruleset, error)` — full validation: schemas, cross-refs
  (every ability's conditions/resources/defenses declared), expression
  parse of every expr at LOAD time (no runtime surprises).
- `Resolve(rs *Ruleset, st *engine.State, cmd UseAbility, rng Roller)
  (*Outcome, error)` — pure given rng: validates actor/targets exist and
  have the referenced stats, usage availability (limited-use = a resource
  spend, emitted as ResourceChanged), range via grid Chebyshev distance
  (generic geometry), rolls attack per target vs the target's defense
  value from its open maps, applies hit/miss/effect outcome lists,
  evaluates thresholds AFTER resource changes (emitting condition
  events), returns the ordered event list for ONE AppendBatch. Rejections
  (unknown ability, out of range, insufficient usage resource) are clean
  errors — nothing persists.
- arch-lint: `rules → {engine, contract, rules}` — no store/campaign/
  gateway/identity/harness. The vocabulary gate scans it like everything
  else.

## 6. AppendBatch (campaign + store)

`store.AppendBatch(envs []*Envelope) (firstSeq int64, err error)` — one
transaction, sequences assigned contiguously, all-or-nothing, notify AFTER
commit in order. `campaign.AppendBatch(envs)` — same lock discipline as
Append: stamp session ids, snapshot-validate the WHOLE batch by folding it
against a snapshot (any failure → nothing persists), persist, live-apply
all, notify all. Poison rules unchanged (post-persist failure poisons).
Undo interaction: a batch is retractable as a range like any events (the
existing range machinery already covers contiguous spans).

## 7. Wiring

`vtt serve --ruleset <dir>` (optional; serving without one keeps today's
behavior — UseAbility commands are then rejected with "no ruleset
loaded"). Gateway: `use_ability` + `remove_condition` in the authz table
(dm/agent: any actor; players: only actors they control — same ownership
check as MoveToken); handler calls rules.Resolve then campaign.AppendBatch;
CommandResult carries the FIRST sequence of the batch. MCP: use_ability +
remove_condition tools auto-appear (toolgen manifest +2);
`get_ruleset_guide` returns guide.md (or a clear "no ruleset loaded").

## 8. The P4 proof — conformance suite + toy ruleset

`internal/rules/conformance` runs against ANY ruleset dir: schema + cross-
ref validation; every ability resolves against fixture statblocks
generated from the manifest's own declarations; golden scenario per
ability (fixed-seed rolls → exact expected event batch). The TOY ruleset
`rulesets/tavern-brawl/` (two attributes, fists + chair-swing abilities,
one "dazed-by-ale" condition, one drink resource with a threshold) passes
the suite with zero platform changes — committed, in task check, forever.
5b's dnd45e-minimal must pass the SAME suite untouched.

## 9. Testing (ADR-009 binding)

Stub-first behavioral RED throughout; the expression evaluator gets
exhaustive table tests + a fuzz pass; AppendBatch gets the data-integrity
review depth (atomicity injection: kill mid-batch in a throwaway → nothing
persisted; property test extended with random ability uses once 5b
exists); interpreter golden scenarios with fixed seeds; wire-level: the
scenario format gains a `useAbility` step so the harness library can
exercise rules over the WebSocket (one new library scenario using the toy
ruleset); MCP e2e extended: Claude's tool list grows use_ability/
remove_condition/get_ruleset_guide against a toy-ruleset server.

## 10. Non-goals (YAGNI)

Turn/initiative engine; reactions/opportunity actions; forced movement;
areas of effect; healing mechanics; condition mechanical enforcement
(movement/action restrictions — v2 format); adventure package format
(future, with the world layer); ruleset hot-reload; multi-ruleset serving;
sheet UI descriptors (sub-project 7); bestiary distribution.

## 11. Open questions (deferred, with owners)

- Expression grammar extensions (keywords, resistances) — v2 format,
  driven by 5b's real 4.5e needs; the v1 grammar is deliberately closed.
- Whether ResourceChanged should carry max for clamping vs reading the
  actor's declared max — decided at plan time against the Actor shape.
- Adventure format — designed with the world layer; Temple-of-Evil-on-5e
  is its acceptance vision (Patrik, this session).

## 12. Amendments (2026-07-25, merge gate)

1. **Ability-outcome idempotency:** `apply_condition`/`remove_condition` outcomes are
   IDEMPOTENT — an already-present condition is not re-applied, an absent condition's
   removal is silently skipped. Generalizes §5's threshold-only guard; required because a
   re-applied condition would doom the atomic batch at the engine fold.
2. **Cross-resource threshold evaluation order:** FIRST-TOUCH CHANGE ORDER — the order in
   which (actor, resource) pairs are first changed during the resolution — then threshold
   declaration order. NOT resource declaration order.
3. **Grammar decisions frozen at Task 4:** expression-referenceable names (attributes,
   defenses, resources) use IDENT charset `[A-Za-z_][A-Za-z0-9_]*`; ability/condition ids
   are kebab-case; NO unary minus (write `0 - x`); integer literals must fit int64 (parse
   error otherwise); dice bounds are count 1..100, sides 1..1000; parser recursion depth is
   capped at 200.
4. **Dice rejected in threshold/max expressions:** dice terms are REJECTED at load time in
   threshold `when` and in `default_max_expr` — a v1 restriction preserving §2 decision 3's
   rolled-once-recorded-forever testimony; threshold evaluation does not record rolls.
5. **Numeric value contract:** expression results, resource deltas, and `new_value`s are
   bounded to int32 (the wire contract); `Resolve` rejects out-of-range values with a clean
   rules error; the engine computes its clamp verification in int64 (no wrap acceptance).
6. **Duplicate target_ids rejected (Patrik-ruled 2026-07-25):** duplicate `target_ids` in
   one `UseAbility` are REJECTED with a clean validation error — the ruleset author's
   `max_targets` fan-out cap cannot be concentrated onto a single target from the wire.
