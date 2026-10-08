# A command's and a map's ids are bounded

## The problem

`engine.Apply` (`internal/engine/apply.go`) bounds sixteen texts (SPEC-018),
and none of the following. An `ActorControlGranted`'s or
`ActorControlRevoked`'s `participant_id` must be non-empty (`controlTarget`)
and is otherwise unmeasured; a granted one is stored in the actor's
`controller_ids`, which the projection re-sends, one `ActorControlGranted` per
controller, whenever it introduces the actor to a viewer (`introduce` in
`internal/gateway/project.go`). `ToEvent` (`internal/gateway/convert.go`)
copies it from `grant_actor_control` or `revoke_actor_control`, and
`validateGrantActorControl` (`internal/gateway/grant_validate.go`) checks only
the kind, so its one bound is the WebSocket frame (`maxWSFrameBytes`,
SPEC-011). An `ActorAdded` stores a clone of its actor, whose `module_id` and
the keys of whose `resources` and `attributes` are unmeasured; `add_actor`
writes them from the command, and the adventure loader (`loadActors` in
`internal/adventure/load.go`) admits only names a ruleset declares as keys,
which `rules.Load` holds at 128 bytes (VTT-283), and sets no `module_id`. A
`ResourceChanged` must name one of the actor's resources, and the projection's
corrections re-send each key. A `SceneCreated` stores its objects'
`object_id`s in `Scene.Objects` unmeasured; `mapdef.Load`
(`internal/mapdef/load.go`) checks an object's footprint and art but not its
id, neither its length, emptiness nor uniqueness, and `mapdef.Compile` copies
it into the event; an adventure's scene file carries objects in the same form
(`mapdef.ObjectJSON`), and `loadScenes` in `internal/adventure/load.go` checks
only their footprint. An envelope's `session_id` becomes the `Session.ID` of a
`SessionStarted`, unmeasured; `campaign` stamps every envelope it appends with
`sess-` and 32 hex characters, or with none (`stampSessionIDAgainst` in
`internal/campaign/campaign.go`). A `TokenMoved`'s `scene_id` is not set by
`ToEvent`; the server fills it from the folded token (`server.go`, SPEC-013),
whose scene id is bounded, and the fold never reads it. `client/src/fold.ts`
mirrors the fold in every one of these.

## Done looks like

1. `engine.Apply` refuses an `ActorControlGranted` or `ActorControlRevoked`
   whose participant id, an `ActorAdded` whose actor's `module_id` or one of
   whose `resources` or `attributes` keys, a `SceneCreated` one of whose
   objects' ids, a `SessionStarted` whose envelope's session id, and a
   `TokenMoved` whose scene id is longer than the bound, and accepts each at
   exactly the bound; it refuses each of them empty but the `module_id`, and a
   `SceneCreated` two of whose objects share an id: named tests under
   `internal/engine/` fail on today's tree and pass after. The at-bound half
   cannot fail today; a break that makes a check `>=` holds it.
2. `client/src/fold.ts` refuses the same events at the same byte counts: named
   tests under `client/test/` fail on today's tree and pass after.
3. A map file or an adventure one of whose scene objects' id is over the
   bound, empty or shared with another of its objects is refused when it
   loads, naming the file and the field: named tests under `internal/mapdef/`
   and `internal/adventure/` fail on today's tree and pass after.
4. A `grant_actor_control`, `revoke_actor_control` or `add_actor` carrying
   such a string is answered ok=false and appends nothing: named tests under
   `internal/gateway/` fail on today's tree and pass after.
5. Every golden, fixture and shipped content file still loads and folds, and
   `task check` whole is green.

## Rules this puts on the system

- A participant id in a control event, an actor's `module_id` and the keys of
  its `resources` and `attributes`, a scene object's id, a session id and a
  `TokenMoved`'s scene id are at most a fixed number of UTF-8 bytes, in both
  folds.
- Each of them but the `module_id` is refused empty in both folds, and a
  scene's objects' ids are unique in it (the owner's rulings on Q2 and Q3 at
  sign-off).
- A map file or an adventure that carries a scene object id longer than that,
  empty or repeated is refused at load.
- The `add_actor` tool states the bound on an actor's `module_id` and its
  `resources` and `attributes` names (the owner's ruling on Q5).
- A command that carries a longer one appends nothing (VTT-162, which this
  cites rather than restates).

## What it touches

1. `internal/engine/apply.go` (its byte bounds and the `ActorControlGranted`,
   `ActorControlRevoked`, `ActorAdded`, `SceneCreated`, `SessionStarted` and
   `TokenMoved` arms) and its tests
2. `client/src/fold.ts` (the same arms), `client/test/` and the rebuilt
   `cmd/vtt/webdist`
3. `internal/mapdef/load.go` and `internal/adventure/load.go`, their tests and
   fixtures under `testdata/`
4. `internal/gateway/` tests for the three commands; if the tools state the
   bound, `tools/toolgen/main.go`, its tests and
   `contract/testdata/expected_tools.json`
5. the prose the change makes false: `controlTarget`'s doc in `apply.go`,
   `requireControlTarget`'s in `fold.ts`, `server.go`'s sentence that
   `engine.Apply` never reads a move's scene id, `fold.ts`'s that a move's
   `from` and `sceneId` are ignored and the matching test title in
   `fold-unit.test.ts`;
   `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`,
   `docs/specifications/014-loading-a-map.md`, `docs/requirements.md`,
   `docs/verification-debt.md`'s entry on the bounds' copies,
   `tools/comment-ceilings.txt`, and the mutation adjudication keys in
   `apply.go` and `fold.ts` that a new line moves

Three components, in this order: the Go fold and the client fold together,
then the map and adventure loaders; the gateway tests need only the Go fold.

## Specifications this moves

docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
docs/specifications/014-loading-a-map.md

## What could not be established

- The number. The two tickets before this one bound ids at `maxIDBytes`, 128
  bytes, and the ruleset plan's D7 asks this one to bound an actor's keys at
  the same number, since a declared name is the only key the adventure loader
  admits. Whether a participant id, a `module_id` or a session id takes the
  same number is the plan's to propose.
- Whether an empty one is refused: an empty `module_id` is the ordinary case,
  an empty session id is how an envelope outside a session is stamped, and an
  empty participant id is refused already.
- Whether a session id and a `TokenMoved`'s scene id need a fold bound at all,
  since `campaign` stamps the first and the server fills the second from a
  bounded token, so only a log written by hand could carry either over the
  bound.
- Whether a `SceneSeen`'s objects are measured too: the projection builds them
  from a folded scene.
- Whether a scene object's id must also be non-empty and unique in its map,
  which `mergeObjects` assumes when it merges by it.
- Whether the other strings a map file carries belong here: an object's `kind`
  and `art`, a tile's kind, material and art, and a placement's actor id.
- Whether the MCP tools that issue `grant_actor_control`,
  `revoke_actor_control` and `add_actor` should state the bound, as
  `add_actor`'s actor id and name do (VTT-276, VTT-281).
- Whether `grant_actor_control` should refuse a participant identity does not
  know, which would bound the id by identity's own 32 hex characters.