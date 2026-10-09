# A field outside SPEC-018's table is unbounded on purpose

## The problem

`docs/specifications/018-the-fold-bounds-free-text-in-bytes.md` lists the
texts `engine.Apply` bounds and says that every other text has no bound in
the fold. It does not say whether that absence is a decision or a gap, and
the two latest reports send the fields outside the table "nowhere yet; raised
to the owner": `docs/reports/2026-10-06-a-rulesets-ids-and-names-are-bounded.md`
a ruleset's display names, expressions, atom ids, param names and graph keys
and its manifest's id and name, and
`docs/reports/2026-10-07-a-commands-and-a-maps-ids-are-bounded.md` an
object's `kind` and `art`, a tile's kind, material and art, and a placement's
`actor_id`; the second sends an envelope's `event_id`, `participant_id` and
`actor_role` and an actor's `module_data` "nowhere, unless the owner names
them". A reader who finds one of them unbounded finds no
answer in the tree, so the next arc raises it again. The owner ruled on
2026-10-09 that no further field is bounded unless a need is shown.

Each text field of `contract/vtt/v1/events.proto` outside the table, its
strings, map keys and `google.protobuf.Struct` fields, is held some other way,
or by nothing, and SPEC-018 names few of them:

- The fold requires an id that names something it has already folded:
  `TokenMoved.token_id`, `TokenPlaced.scene_id` and `actor_id`,
  `TokenRemoved.token_id`, `ActorRemoved.actor_id`, `DoorOpened.scene_id`,
  `DoorClosed.scene_id`, `ResourceChanged.actor_id` and `resource`,
  `ConditionApplied.actor_id`, `ConditionRemoved.actor_id` and
  `condition_id`, `NoteDeleted.key`, and a control event's `actor_id`.
- `ActorAdded` refuses an actor that declares `Actor.controller_id` or
  `Actor.controller_ids`.
- `TokenHidden.token_id` and `SceneSeen`'s `scene_id`, `tiles` keys and
  `visible` are built by `internal/gateway/project.go` from folded state.
- `Resolve` in `internal/rules/resolve.go` composes
  `ConditionApplied.source`, `ResourceChanged.reason` and
  `ConditionRemoved.reason` from an ability id and a phase or from a resource
  name, and `ToEvent` in `internal/gateway/convert.go` writes a
  `remove_condition`'s reason as `manual`.
- `mapdef.BuildSceneCreated`, which the map path and the adventure path both
  call, writes `TileRef.kind` and `material` from `StandardTile`'s
  vocabulary, `TileRef.art` and `SceneObject.art` only for an id
  `internal/artlib` resolves, whose `isArtID` refuses one longer than
  `maxArtIDLen`, and `SceneCreated.tiles` keys only for the squares of the
  grid it walks.
- The server stamps `Envelope.event_id` (`newEventID`), `participant_id` and
  `actor_role` (from the issuing participant), and every envelope's
  `session_id` (`stampSessionIDAgainst`), a `SessionStarted`'s with a fresh
  id that the table bounds.
- The fold keeps no `AttackRolled` field and no `AbilityUsed` field, and
  nothing in production writes an `AttackRolled`, `Modifier.source` included.
  `Resolve` requires `AbilityUsed.actor_id` and each of `target_ids` to be a
  folded actor, writes `outcome_summary` from an ability's display name,
  each target's id and, for an ability with a resolution, a branch label and
  two totals, and writes `Roll.expression` from a ruleset's expressions; no loader bounds a display name or an expression.
- `SceneObject.kind` is bounded by nothing, and the fold keeps it.
- `Actor.module_data`, a `google.protobuf.Struct`, is bounded by the
  WebSocket frame alone; `add_actor`, a command of the DM and the agent,
  writes it, and the fold keeps it.

No command a player may issue (SPEC-013's table) puts text the player chose
into a field outside the table, other than an id that the fold or `Resolve`
requires to name something already folded or declared by the loaded ruleset,
which `Resolve` also copies into `outcome_summary` and the reasons. A
player's `use_ability` and `remove_condition` otherwise write such fields from
the ruleset and the server, not from the player.

## Done looks like

1. SPEC-018 names every text field of `contract/vtt/v1/events.proto`,
   either in its table or with what holds it. A command that lists the
   proto's text fields as `Message.field` and searches SPEC-018's table rows
   for each
   prints the fields it does not find; on today's tree it prints some, after
   this ticket none.
2. SPEC-018 says that a field outside its table is unbounded by decision,
   and names what would bring a field into the table. Held by Phase 4b's
   reading.
3. SPEC-018 names the strings of a map, an adventure and a ruleset file that
   no bound holds, with the same answer. Held by Phase 4b's reading.
4. `docs/map-format.md` §10 states the bounds the map loader already
   enforces on a map's `id`, its `name` and a placement's `token_id`
   (`internal/mapdef/load.go`'s `maxIDBytes` and `maxNameBytes`), without
   renumbering its list. Held by Phase 4b's reading.
5. `task check` whole is green.

## Rules this puts on the system

None. What would bring a field into the table is a rule on later decisions,
not on the system: no observation of the running code goes red when it is
broken.

## What it touches

1. `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`
2. `docs/map-format.md` §10, which states the object id bound and none of
   the three older ones; `docs/map-format.md` cites its item 7 as "§10 rule
   7", so its items keep their numbers

## Specifications this moves

docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- Whether the ruling also belongs in `CLAUDE.md` as a numbered rule, or in
  SPEC-018 alone.
- Whether the refusals that quote an unbounded value, `load_map`'s of an
  unknown actor, the adventure loader's of an undeclared key, `Resolve`'s and
  the fold's of an unknown id, are covered by the same answer. Each reaches
  the issuer, or, when a file is refused at boot, whoever starts the
  server.
- Whether an implementation report follows. This ticket changes no code.
