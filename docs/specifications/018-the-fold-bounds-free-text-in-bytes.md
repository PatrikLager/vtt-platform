# SPEC-018: The fold bounds an event's free text and ids in bytes

## Status

Accepted. Implemented by `internal/engine/apply.go` (the const block with
`maxNoteKeyBytes`, `maxNoteTitleBytes`, `maxTextBytes`, `maxNarrationAsBytes`,
`maxMoveReasonBytes`, `maxNameBytes` and `maxIDBytes`, and `Apply`'s
`SessionStarted`, `SceneCreated`, `ActorAdded`, `TokenPlaced`, `TokenMoved`,
`NarrationAdded`, `NoteUpserted`, `AdventureLoaded`, `AbilityUsed` and
`ConditionApplied` arms) and its TypeScript mirror `client/src/fold.ts`
(`checkLen` and the same ten arms); mirrored for an adventure's notes, opening
narration, names and ids by `internal/adventure/load.go`'s constants and for a
map's name and ids by `internal/mapdef/load.go`'s `maxNameBytes` and
`maxIDBytes`, and for a ruleset's ability and condition ids, attribute,
defense and resource names and branch labels by `internal/rules/load.go`'s
`maxIDBytes`, which the ruleset schemas in `internal/rules/schema/` state;
stated to an agent by `tools/toolgen/main.go`'s `manifest` for `move_token`'s
`reason`, `add_actor`'s actor `name` and `actorId`, `start_session`'s `name`
and `place_token`'s `tokenId`. Pinned by `internal/engine/apply_test.go`,
`apply_boundary_test.go`, `move_reason_internal_test.go`,
`name_bound_internal_test.go`, `id_bound_internal_test.go`,
`internal/campaign/qa_id_bound_test.go`, `internal/gateway/server_test.go`,
`qa_move_reason_bound_test.go`, `qa_name_bound_test.go`,
`qa_id_bound_test.go`, `client/test/fold-rejections.test.ts`,
`qa-move-reason-bound.test.ts`, `qa-name-bound.test.ts`,
`qa-id-bound.test.ts`, `internal/adventure/format_test.go`, `load_test.go`,
`qa_name_bound_test.go`, `qa_id_bound_test.go`,
`internal/mapdef/load_test.go`, `installed_test.go`, `qa_name_bound_test.go`,
`qa_id_bound_test.go`, `internal/rules/load_test.go`, `resolve_test.go`,
`id_bound_internal_test.go`, `qa_ruleset_bound_test.go`,
`internal/engine/qa_ruleset_bound_test.go`,
`internal/gateway/qa_ruleset_bound_test.go`,
`internal/campaign/qa_ruleset_bound_test.go`,
`client/test/qa-ruleset-bound.test.ts` and `tools/toolgen/main_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a log is append-only, so what the fold accepts once it must accept
forever.

## How it works

**Sixteen fields are bounded, each in UTF-8 bytes, inclusive.**

| Field | Bound | May be empty |
|---|---|---|
| `NoteUpserted.key` | 128 (`maxNoteKeyBytes`) | no |
| `NoteUpserted.title` | 256 (`maxNoteTitleBytes`) | yes |
| `NoteUpserted.text` | 8192 (`maxTextBytes`) | no |
| `NarrationAdded.text` | 8192 (`maxTextBytes`) | no |
| `NarrationAdded.as` | 256 (`maxNarrationAsBytes`) | yes |
| `TokenMoved.reason` | 256 (`maxMoveReasonBytes`) | yes |
| `SessionStarted.name` | 256 (`maxNameBytes`) | yes |
| `SceneCreated.name` | 256 (`maxNameBytes`) | yes |
| `ActorAdded`'s `Actor.name` | 256 (`maxNameBytes`) | yes |
| `AdventureLoaded.name` | 256 (`maxNameBytes`) | yes |
| `SceneCreated.scene_id` | 128 (`maxIDBytes`) | no |
| `ActorAdded`'s `Actor.actor_id` | 128 (`maxIDBytes`) | no |
| `TokenPlaced.token_id` | 128 (`maxIDBytes`) | no |
| `AdventureLoaded.adventure_id` | 128 (`maxIDBytes`) | no |
| `ConditionApplied.condition_id` | 128 (`maxIDBytes`) | no |
| `AbilityUsed.ability_id` | 128 (`maxIDBytes`) | no |

`engine.Apply` measures each with `len`, which counts bytes, and refuses a
field over its bound, or empty where it may not be, with an error naming the
field, the bound and the length (`engine: move reason must be at most 256
bytes, got 257`; `engine: note key must be 1-128 bytes, got 0`), except an
`ActorAdded` whose actor id is empty, which is refused as `engine: actor_added
requires an actor with an id`. A `TokenMoved` is checked for a known token,
then for a destination at all, then its reason. A `SceneCreated` is checked
for its id, then a duplicate, then its name; an `ActorAdded` for an actor with
an id, then the id's length, a duplicate and a declared controller, then its
name; a `TokenPlaced` for its id, then a duplicate, a known scene, a known
actor and a position; an `AdventureLoaded` for its id, then its name; a
`ConditionApplied` for its condition id, then a known actor, then a duplicate;
an `AbilityUsed` for its ability id alone; a `SessionStarted` for an open
session, then its name. These sixteen are the only texts `apply.go` bounds
above; beyond them it requires a control event's participant id to be
non-empty. Every other text has no bound in the fold: one a command carries is
bounded by `maxWSFrameBytes`, the read limit on a WebSocket frame (SPEC-011);
one a map, adventure or ruleset file carries is bounded only by what its
loader checks. A `ConditionApplied`'s `source`, which `Resolve` composes from
an ability id and its phase, a branch label or `effect`, or from a resource
name, is bounded by no fold.

**The refusal reaches the issuer.** `campaign.Append` folds the envelope
before it persists anything and returns the fold's error, so a command whose
event exceeds a bound is answered ok=false with that text and appends nothing
(SPEC-013's command path, VTT-161, VTT-162). A log that already holds such an
event does not fold, so `campaign.Open` refuses it.

**The client's fold mirrors every bound.** `fold.ts` calls `checkLen(what, s,
min, max)` for each of the sixteen fields with the same numbers as literals,
in the same order within each arm; `checkLen` counts UTF-8 bytes with
`TextEncoder`, as Go's `len` does, and throws `FoldError` reading `<field>
exceeds <max> bytes` or `<field> is shorter than <min> bytes`. Nothing ties a
literal to its Go constant but each side's tests.

**Other mirrors.** `internal/adventure/load.go` holds its own copies of
`maxNoteKeyBytes`, `maxNoteTitleBytes` and `maxTextBytes`, which
`TestSizeCapsMirrorEngine` pins to the literals 128, 256 and 8192, and of
`maxNameBytes`, which `TestTheNameBoundMirrorsEngine` pins to 256, and of
`maxIDBytes`, for an adventure's id, its scenes' and actors' ids and its
placements' token ids, which `TestTheIDBoundMirrorsEngine` pins to 128;
`internal/mapdef/load.go` holds its own `maxNameBytes` and `maxIDBytes`, the
second for a map's id and its placements' token ids, held by
`TestInvalidMapsAreRefusedWithAUsefulReason`,
`TestAMapNameOfExactlyTheBoundLoads` and `TestAMapIDOfExactlyTheBoundLoads`;
`internal/rules/load.go` holds its own `maxIDBytes`, for a ruleset's ability
and condition ids, its attribute, defense and resource names and its
resolutions' branch labels, which `TestTheIDBoundMirrorsEngine` pins to 128,
and the ruleset schemas state it, which `TestTheSchemasStateTheIDBound`
requires; nothing compares any copy with the engine's constants. A map, an
adventure or a ruleset over one of them is refused at load rather than at the
fold, by an error naming the file and the field. The MCP `move_token` tool
describes its `reason` field as at most 256 bytes of UTF-8 (`manifest`'s
`fieldDocs`), and `TestTheMoveToolStatesTheFoldsReasonBound` requires the
generated `contract/gen/tools/tools.json` to state `maxMoveReasonBytes`; the
`add_actor` and `start_session` tools describe their name the same way, and
`TestTheToolsStateTheFoldsNameBound` requires it to state `maxNameBytes`;
`add_actor` describes its actor id and `place_token` its token id the same
way, and `TestTheToolsStateTheFoldsIDBound` requires them to state
`maxIDBytes`.

## Consequences

- A bound may be raised and never lowered: a log that folded under the old
  bound would stop folding.
- Changing a bound changes its mirrors in the same change: `fold.ts`'s
  literal, `internal/adventure`'s, `internal/mapdef`'s and `internal/rules`'
  copy where there is one, the ruleset schemas' statement of the id bound, for
  a move's reason the tool's description, for a name the `add_actor` and
  `start_session` descriptions, and for an id the `add_actor` and
  `place_token` descriptions.
- A field added to an event without a bound here is bounded by no fold: by the
  WebSocket frame when a command carries it, by nothing when a map, adventure
  or ruleset file does.

## Requirements

VTT-264, VTT-265, VTT-266, VTT-274, VTT-275, VTT-276, VTT-277, VTT-278,
VTT-279, VTT-281, VTT-282, VTT-283, VTT-284, VTT-285, VTT-286, VTT-287.
