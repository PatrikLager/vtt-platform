# An id is bounded

## The problem

`engine.Apply` in `internal/engine/apply.go` bounds ten texts in UTF-8 bytes
(SPEC-018) and no id. The four ids that create an identity in the log, a
scene's (`SceneCreated.scene_id`), an actor's (`Actor.actor_id`, carried by
`ActorAdded`), a token's (`TokenPlaced.token_id`) and an adventure's
(`AdventureLoaded.adventure_id`), are accepted at any length by both folds;
`client/src/fold.ts` checks none of them either. Of the four, only an actor's
id must be non-empty (`actor_added requires an actor with an id`); an empty
scene, token or adventure id folds. Every later event that names one of them,
a move, a removal, a grant, a status change, carries the same string, and so
does every frame the projection builds about it. They reach the log four
ways. A map's id is its filename (`mapdef.LoadInstalled` in
`internal/mapdef/installed.go`), bounded only by what the filesystem allows,
and `mapdef`'s `Compile` makes it the `SceneCreated`'s scene id; it also
emits one `TokenPlaced` per placement the map file declares, carrying the
placement's `token_id` unbounded (the shipped
`campaigns/example/maps/cellar.json` places `tok-fighter`).
`adventure`'s loader (`internal/adventure/load.go`) bounds a scene's id at 128
bytes (`maxIDBytes`) and an adventure's, an actor's and a token's id not at
all, and its own comment records the map path as "left alone rather than
tightened to match", which this ticket would overturn. `ToEvent`
(`internal/gateway/convert.go`) turns `add_actor` and
`place_token` into events carrying the command's ids, bounded only by
`maxWSFrameBytes` (SPEC-011). SPEC-018 records that every text but its ten
has no bound in the fold, and
`docs/reports/2026-10-05-a-name-is-bounded.md` raised ids to the owner, who
asked for this ticket on 2026-10-06. The longest id in the repository's
content (goldens, scenarios, adventures, maps and contract test data) is 18
bytes, and the longest map filename 16, apart from
`internal/adventure/testdata/at-every-boundary`'s scene id, which sits at the
loader's 128, and `internal/adventure/testdata/invalid/scene-id-too-long`'s
200-byte id, which exists to be refused; the longest creating id any test,
golden, scenario or eventgen draw folds is 20 bytes (the plan's
measurement).

## Done looks like

1. `engine.Apply` refuses a `SceneCreated`, an `ActorAdded`, a `TokenPlaced`
   or an `AdventureLoaded` whose id is longer than its bound, and accepts one
   of exactly the bound: named tests under `internal/engine/` fail on today's
   tree and pass after. The at-bound half cannot fail today; a break that
   makes the check `>=` holds it.
2. `client/src/fold.ts` refuses the same events at the same byte counts:
   named tests under `client/test/` fail on today's tree and pass after,
   their at-bound half held the same way.
3. A map whose id, or one of whose placements' token ids, is over the bound,
   and an adventure whose own id or one of whose actors' or tokens' ids is
   over it, are refused when they load,
   naming the file and the field, before anything is appended: named tests
   under `internal/mapdef/` and `internal/adventure/` fail on today's tree and
   pass after.
4. An `add_actor` or a `place_token` whose id is over the bound is answered
   ok=false and appends nothing: named tests under `internal/gateway/` fail on
   today's tree and pass after.
5. Every golden, fixture and shipped content file still loads and folds, and
   `task check` whole is green.

## Rules this puts on the system

- A scene's, an actor's, a token's and an adventure's id is at most a fixed
  number of UTF-8 bytes, in both folds.
- A map or an adventure that carries a longer id, a map's placements
  included, is refused at load.
- A scene's, a token's and an adventure's id may not be empty, in both folds,
  as an actor's may not; a map placement with an empty token id is refused at
  load (the owner's ruling on Q2 at sign-off).
- A command that carries a longer id appends nothing (VTT-162, which this
  cites rather than restates).

## What it touches

1. `internal/engine/apply.go` (its byte bounds and the `SceneCreated`,
   `ActorAdded`, `TokenPlaced` and `AdventureLoaded` arms) and its tests
2. `client/src/fold.ts` (the same four arms) and `client/test/`
3. `internal/mapdef/installed.go` and `load.go`, `internal/adventure/load.go`
   (where `loadScenes` sits at `gocyclo`'s ceiling, so a further check needs
   the placement loop moved into a function of its own), their tests, their
   fixtures and catalogue lists, and `at-every-boundary`'s at-cap values; the
   comment blocks that become false: `load.go`'s `maxIDBytes` block and
   `internal/artlib/artlib.go`'s doc listing what is still unbounded
4. `internal/gateway/` tests for `add_actor` and `place_token`; if the tools
   state the bound, `tools/toolgen/main.go`, its tests (one of which requires
   `actorId` to have no description) and `contract/testdata/expected_tools.json`
5. `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`;
   `docs/specifications/014-loading-a-map.md`, which states what a map's id
   may be; `docs/requirements.md`; `docs/verification-debt.md`'s entry on the
   bounds' copies; `tools/comment-ceilings.txt`; the mutation adjudication
   keys in `apply.go`, `fold.ts` and `internal/adventure/load.go` that a new
   line moves

Three components, in this order: the engine, then the client fold, then the
two loaders; the gateway tests need only the engine.

## Specifications this moves

docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
docs/specifications/014-loading-a-map.md

## What could not be established

- The number. The repository bounds an identifier a person types at 128 bytes
  (`maxNoteKeyBytes`, `maxIDBytes`); a map's id is a filename, which APFS
  bounds at 255 UTF-16 units rather than bytes and ext4 at 255 bytes less the
  `.json` suffix, so no byte bound admits every filename. Whether a map's id
  takes the fold's bound, and a longer map filename stops being installable,
  is the plan's to propose and the owner's to rule.
- Whether the ids that only refer, such as a move's token id or a grant's
  actor id, need a check of their own, or are bounded by the fold's
  requirement that what they name exists.
- Whether other id-like strings belong here: a condition's id, a resource's
  name, an ability's id, an object's id, an actor's `module_id`, a control
  event's participant id.
- Whether the MCP tools that issue `add_actor` and `place_token` should state
  the bound, as `add_actor`'s name does (VTT-276).
- Where a map whose filename is over the bound is refused: at boot, at
  `load_map`, or when it is installed.
