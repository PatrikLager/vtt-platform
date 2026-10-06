# A ruleset's ids and names are bounded

## The problem

`rules.Load` (`internal/rules/load.go`) decodes and validates a ruleset by
hand; the JSON schemas in `internal/rules/schema/` are documents for ruleset
authors, which `schema_test.go` cross-checks against the loader and nothing
validates against (`Schemas`' doc in `schema.go`). Neither bounds a length: an
ability's `id` and a condition's `id` are any string of at least one
character, and an attribute, defense or resource name matches
`^[A-Za-z_][A-Za-z0-9_]*$` at any length; nothing in `load.go` measures them.
`rules.Resolve` (`internal/rules/resolve.go`) writes them into the events a
`use_ability` appends: `AbilityUsed.ability_id`, `ResourceChanged.resource`,
`ConditionApplied.condition_id` and `ConditionRemoved.condition_id`, and it
composes `ConditionApplied.source` and the other two events' `reason` from
them (`ability:<id>:<phase>`, `threshold:<resource>`). `engine.Apply` returns
nil for an `AbilityUsed` without reading it, stores a `ConditionApplied`'s
condition id and source in `st.Conditions` without measuring them, and accepts
a `ResourceChanged` whose resource names one of the actor's resources;
`client/src/fold.ts` mirrors it. SPEC-018 bounds fourteen texts, none of
these. The same names reach the issuer of a refused `use_ability`: `Resolve`'s
refusals interpolate them with `%q`, and `handleUseAbility`
(`internal/gateway/ruleset.go`) returns `err.Error()` as the result's error.
`docs/verification-debt.md`'s entry "A ruleset's names reach a `use_ability`
refusal at any length" records a 70,000-byte resource name that loads and a
refusal of 70,089 bytes; a threshold's `when` expression reaches a refusal
whole the same way (`threshold %q on resource %q`). `vtt serve` and `vtt mcp`
load the ruleset at start (`cmd/vtt/serve_compose.go`, `cmd/vtt/mcp.go`), and
`serve` refuses to start when `rules.Load` fails. The shipped rulesets are
`rulesets/dnd45e-minimal` and `rulesets/tavern-brawl`.

## Done looks like

1. `rules.Load` refuses a ruleset that declares an ability id, a condition id,
   an attribute, defense or resource name, or a resolution's branch label
   longer than the bound, naming the file and the field, and loads one whose
   every such string is exactly the bound: named tests under `internal/rules/`
   fail on today's tree and pass after. The at-bound half cannot fail today; a
   break that makes a check `>=` holds it.
2. `engine.Apply` refuses a `ConditionApplied` whose condition id, and an
   `AbilityUsed` whose ability id, is empty or longer than the bound, and
   accepts one of exactly the bound: named tests under `internal/engine/` fail
   on today's tree and pass after, their at-bound half held the same way.
3. `client/src/fold.ts` refuses the same events at the same byte counts: named
   tests under `client/test/` fail on today's tree and pass after.
4. The debt entry's recipe is refused at `rules.Load`, naming the file and the
   field, and a `use_ability` refusal read off a real connection carries no
   ruleset text longer than the bound, a threshold's expression included:
   named tests fail on today's tree and pass after, and close the entry, whose
   own closing condition names the connection.
5. Every shipped ruleset and every ruleset fixture meant to load still loads,
   every golden still folds, and `task check` whole is green.

## Rules this puts on the system

- A ruleset's ability ids, condition ids, attribute, defense and resource
  names and its resolutions' branch labels are at most a fixed number of UTF-8
  bytes; a ruleset carrying a longer one is refused at load.
- Both folds refuse a `ConditionApplied` whose condition id, or an
  `AbilityUsed` whose ability id, is longer than that bound, or empty (the
  owner's ruling on Q4 at sign-off).
- A refused `use_ability` carries no ruleset text longer than that bound.

## What it touches

1. `internal/rules/load.go` (where `loadManifest` already carries
   `//nolint:gocyclo`), `resolve.go`'s refusals that quote a ruleset's text,
   the schema documents in `internal/rules/schema/` and `schema_test.go`,
   their tests and fixtures under `internal/rules/testdata/` and
   `internal/rules/conformance/testdata/`
2. `internal/engine/apply.go` (its byte bounds and the `ConditionApplied` and
   `AbilityUsed` arms) and its tests, and `internal/gateway/` for a refusal
   read off a connection
3. `client/src/fold.ts` (the same two arms), `client/test/`, including
   `fold-unit.test.ts`'s comment that `abilityUsed` shares its arm with
   `attackRolled`, and the rebuilt `cmd/vtt/webdist`
4. `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`,
   `docs/requirements.md`, both of `docs/verification-debt.md`'s entries on
   the bounds (this one and the count in "Nothing ties a byte bound's
   copies"), and the mutation adjudications in `apply.go` and `fold.ts`, where
   `fold.ts`'s `abilityUsed` entry must go rather than move once the arm
   checks something

Three components, in this order: the ruleset loader, then the Go fold, then
the client fold; the loader can land first because the fold's bound then
refuses nothing a loaded ruleset can produce.

## Specifications this moves

docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- The number. 128 bytes is `maxIDBytes`, the bound on the ids the fold
  already holds. A resource or attribute name is also a key of an actor's
  `resources` and `attributes`, which `add_actor` and the adventure loader
  write; those keys belong to the second ticket the owner asked for, and
  whether the two must share one constant is the plan's to propose.
- Whether the ruleset's other texts belong here: an ability's and a
  condition's display `name`, the expressions (`roll`, `vs`, `delta_expr`, a
  threshold's `when`), the branch labels and the atom ids. An ability's name
  reaches `AbilityUsed.outcome_summary` once per target, an expression reaches
  `AbilityUsed.rolls` and some refusals, and a branch label reaches
  `ConditionApplied.source`, which the fold stores.
- Whether `ConditionApplied.source`, and the `reason` of a `ResourceChanged`
  or `ConditionRemoved`, need a fold bound of their own or follow from their
  parts.
- Whether the folds should refuse an empty condition id or ability id.
- Whether `ResourceChanged.resource` needs a fold bound, since it must name
  one of the actor's resources and the actor's keys are left to the second
  ticket.
