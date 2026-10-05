# A name is bounded

## The problem

`engine.Apply` in `internal/engine/apply.go` bounds six texts in UTF-8 bytes
(SPEC-018: a note's key, title and text, a narration's text and speaker, a
move's reason) and no name. A scene's name (`SceneCreated.name`), an actor's
name (`Actor.name`, carried by `ActorAdded`), an adventure's name
(`AdventureLoaded.name`) and a session's name (`SessionStarted.name`, which
`ToEvent` copies from `start_session` and every seat is sent) are accepted at
any length by both folds:
`client/src/fold.ts` checks none of them either. They reach the log three
ways. `mapdef`'s `Compile` (`internal/mapdef/compile.go`) builds a
`SceneCreated` from a map file's `name`, and `mapdef`'s loader
(`internal/mapdef/load.go`) bounds no text in it. `adventure`'s `Compile`
(`internal/adventure/compile.go`) builds an `AdventureLoaded`, scenes and
`ActorAdded` events from an adventure's files, and its loader
(`internal/adventure/load.go`) bounds a scene's id (`maxIDBytes`, 128) and a
note's texts but no name. `ToEvent` (`internal/gateway/convert.go`) turns
`add_actor` into an `ActorAdded` carrying the command's actor whole, and
`validateAddActor` (`internal/gateway/add_actor_validate.go`) checks its
controllers and kind, not its name; such a command is bounded only by
`maxWSFrameBytes`, the WebSocket read limit (SPEC-011). So a map file, an
adventure or an `add_actor` can append a name of any length a file or a frame
holds, and the log keeps it forever. A scene's and an actor's name reach
every projected viewer that is introduced to them (`introduce` in
`internal/gateway/project.go` copies them); an `AdventureLoaded` is withheld
from every projected seat and reaches the DM and the agent. SPEC-018 records this ("every scene's name and
an adventure's actor names among them, is bounded by nothing"), and
`docs/reports/2026-10-04-a-moves-reason-is-bounded.md` raised it to the
owner, who asked for this ticket on 2026-10-05. The longest name that
reaches an event anywhere in the repository's content and tests is 19 bytes,
apart from `bigPaddingName` in `internal/gateway/server_internal_test.go`, a
28,672-byte string four backpressure tests use as an actor's or a scene's
name to make a frame large.

## Done looks like

1. `engine.Apply` refuses a `SessionStarted`, a `SceneCreated`, an
   `ActorAdded` or an `AdventureLoaded` whose name is longer than its bound, and accepts one of
   exactly the bound: named tests under `internal/engine/` fail on today's
   tree and pass after. The at-bound half cannot fail today, when nothing is
   refused; a break that makes the check `>=` holds it.
2. `client/src/fold.ts` refuses the same events at the same byte counts:
   named tests under `client/test/` fail on today's tree and pass after,
   their at-bound half held the same way.
3. A map file and an adventure whose name, or one of whose scenes' or actors'
   names, is over the bound are refused when they load, naming the file and
   the field, before anything is appended: named tests under
   `internal/mapdef/` and `internal/adventure/` fail on today's tree and pass
   after.
4. An `add_actor` whose actor's name, or a `start_session` whose name, is
   over the bound is answered ok=false and appends nothing: a named test
   under `internal/gateway/` fails on today's tree and passes after.
5. Every golden, fixture and shipped content file still loads and folds, and
   `task check` whole is green.

## Rules this puts on the system

- A session's, a scene's, an actor's and an adventure's name is at most a
  fixed number of UTF-8 bytes, in both folds.
- A map or an adventure that carries a longer name is refused at load.
- A command that carries a longer name appends nothing (VTT-162, which this
  cites rather than restates).

## What it touches

1. `internal/engine/apply.go` (its byte bounds and the `SessionStarted`,
   `SceneCreated`, `ActorAdded` and `AdventureLoaded` arms) and its tests
2. `client/src/fold.ts` (the same four arms) and `client/test/`
3. `internal/mapdef/load.go` and `internal/adventure/load.go`, and their
   tests
4. `internal/gateway/` tests for `add_actor`, and the four backpressure
   tests in `internal/gateway/server_internal_test.go` whose
   `bigPaddingName` is a name over any bound this sets
5. the loaders' fixtures: `internal/mapdef` and `internal/adventure` testdata,
   among them the adventure's at-every-boundary fixture
6. `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`;
   `docs/requirements.md`; `docs/verification-debt.md`'s open entry on the
   bounds' copies; `tools/comment-ceilings.txt`; the mutation adjudication
   keys in `apply.go`, `fold.ts` and `internal/adventure/load.go` that a new
   line moves

Three components, in this order: the engine, then the client fold, then the
two loaders; the gateway test needs only the engine.

## Specifications this moves

docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- The number. SPEC-018 bounds a note's title and a narration's speaker at 256
  bytes and a note's key at 128; whether a name takes 256, as the nearest
  kind of text, is the plan's to propose and the owner's to rule.
- Whether a name may be empty: the folds accept an empty name today, and the
  map and adventure loaders may already refuse one.
- Whether ids get the same treatment. `SceneCreated.scene_id`, `Actor.actor_id`,
  `TokenPlaced.token_id` and `AdventureLoaded.adventure_id` are unbounded in
  the folds too, and only an adventure's scene id is bounded by its loader.
  This ticket names names; the plan says whether ids belong here.
- Whether the MCP tools that issue `add_actor` should state the bound, as
  `move_token`'s `reason` does (VTT-265).
