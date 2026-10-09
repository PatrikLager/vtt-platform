# SPEC-018: The fold bounds an event's free text and ids in bytes

## Status

Accepted. Implemented by `internal/engine/apply.go` (the const block with
`maxNoteKeyBytes`, `maxNoteTitleBytes`, `maxTextBytes`, `maxNarrationAsBytes`,
`maxMoveReasonBytes`, `maxNameBytes` and `maxIDBytes`, and `Apply`'s
`SessionStarted`, `SceneCreated`, `ActorAdded`, `TokenPlaced`, `TokenMoved`,
`NarrationAdded`, `NoteUpserted`, `AdventureLoaded`, `AbilityUsed`,
`ConditionApplied`, `ActorControlGranted` and `ActorControlRevoked` arms, the
last two through `controlTarget`, and `keyLength` for an actor's keys) and its
TypeScript mirror `client/src/fold.ts` (`checkLen`, `checkKeys` for an actor's
keys, and the same twelve arms, the control arms through
`requireControlTarget`); mirrored for an adventure's notes, opening narration,
names and ids by `internal/adventure/load.go`'s constants and for a map's name
and ids by `internal/mapdef/load.go`'s `maxNameBytes` and `maxIDBytes`, for a
map's and an adventure's scene objects' ids by `internal/mapdef`'s
`CheckObjectIDs`, which both loaders call, and for a ruleset's ability and
condition ids, attribute, defense and resource names and branch labels by
`internal/rules/load.go`'s `maxIDBytes`, which the ruleset schemas in
`internal/rules/schema/` state; stated to an agent by
`tools/toolgen/main.go`'s `manifest` for `move_token`'s `reason`,
`add_actor`'s actor `name`, `actorId`, `moduleId`, `attributes` and
`resources`, `start_session`'s `name` and `place_token`'s `tokenId`. Pinned by
`internal/engine/apply_test.go`, `apply_boundary_test.go`,
`move_reason_internal_test.go`, `name_bound_internal_test.go`,
`id_bound_internal_test.go`, `internal/campaign/qa_id_bound_test.go`,
`internal/gateway/server_test.go`, `qa_move_reason_bound_test.go`,
`qa_name_bound_test.go`, `qa_id_bound_test.go`,
`client/test/fold-rejections.test.ts`, `qa-move-reason-bound.test.ts`,
`qa-name-bound.test.ts`, `qa-id-bound.test.ts`,
`internal/adventure/format_test.go`, `load_test.go`, `qa_name_bound_test.go`,
`qa_id_bound_test.go`, `internal/mapdef/load_test.go`, `installed_test.go`,
`qa_name_bound_test.go`, `qa_id_bound_test.go`, `internal/rules/load_test.go`,
`resolve_test.go`, `id_bound_internal_test.go`, `qa_ruleset_bound_test.go`,
`internal/engine/qa_ruleset_bound_test.go`,
`internal/gateway/qa_ruleset_bound_test.go`,
`internal/campaign/qa_ruleset_bound_test.go`,
`client/test/qa-ruleset-bound.test.ts`,
`internal/engine/qa_cmd_id_bound_test.go`,
`internal/gateway/qa_cmd_id_bound_test.go`,
`internal/mapdef/qa_cmd_id_bound_test.go`,
`internal/adventure/qa_cmd_id_bound_test.go`,
`internal/campaign/qa_cmd_id_bound_test.go`,
`client/test/qa-cmd-id-bound.test.ts` and `tools/toolgen/main_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a log is append-only, so what the fold accepts once it must accept
forever.

## How it works

**Twenty-four fields are bounded, each in UTF-8 bytes, inclusive.**

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
| `ActorControlGranted.participant_id` | 128 (`maxIDBytes`) | no |
| `ActorControlRevoked.participant_id` | 128 (`maxIDBytes`) | no |
| `ActorAdded`'s `Actor.module_id` | 128 (`maxIDBytes`) | yes |
| `ActorAdded`'s `Actor.resources` keys | 128 (`maxIDBytes`) | no |
| `ActorAdded`'s `Actor.attributes` keys | 128 (`maxIDBytes`) | no |
| `SceneCreated`'s `SceneObject.object_id` | 128 (`maxIDBytes`) | no |
| `SessionStarted`'s `Envelope.session_id` | 128 (`maxIDBytes`) | no |
| `TokenMoved.scene_id` | 128 (`maxIDBytes`) | no |

`engine.Apply` measures each with `len`, which counts bytes, and refuses a
field over its bound, or empty where it may not be, with an error naming the
field, the bound and the length (`engine: move reason must be at most 256
bytes, got 257`; `engine: note key must be 1-128 bytes, got 0`), except an
`ActorAdded` whose actor id is empty, which is refused as `engine: actor_added
requires an actor with an id`, a control event whose participant id is empty,
refused as `engine: actor_control_granted requires a participant id` or its
`actor_control_revoked` twin, and a key of an actor's `resources` or
`attributes`, whose refusal names the longest key's length, or 0 when any key
is empty, however long the others are. A `TokenMoved` is checked for a known
token, then for a destination at all, then its reason, then its scene id. A
`SceneCreated` is checked for its id, then a duplicate, then its name, then
each object in turn, its id's length and then whether an earlier object has
the same id, a repeat refused as `engine: object "<id>" appears twice in scene
"<scene>"`; an `ActorAdded` for an actor with an id, then the id's length, a
duplicate and a declared controller, then its name, then its module id, its
resource names and its attribute names; a `TokenPlaced` for its id, then a
duplicate, a known scene, a known actor and a position; an `AdventureLoaded`
for its id, then its name; a `ConditionApplied` for its condition id, then a
known actor, then a duplicate; an `AbilityUsed` for its ability id alone; a
`SessionStarted` for an open session, then its name, then its session id; a
control event for a participant id, then its length, then a known actor. These
twenty-four are the only texts `apply.go` bounds above; beyond them it
requires a scene's object ids to be distinct. Every other text has no bound
of its own in the fold, and what holds each is below.

**A text outside the table has no bound of its own, by decision.** A field
joins the table when one of two things is shown. The first is that a role
other than the DM and the agent can put text it chose into the field in the
log, other than an id that `engine.Apply` or `Resolve` requires to name
something already folded or declared by the loaded ruleset. The second is,
by a test or a recipe, that a value of the field written for play rather
than built to reach a limit, by a command or a file, breaks a frame, a view
or a refusal that a participant other than its writer receives; a file's
writer is whoever installs it. The DM, the agent and whoever installs a file
are trusted with a text's length as they are with its content. No field
outside the table meets the first: each of SPEC-013's player cells puts text
it chose into the log only as a field of the table or as an id that
`engine.Apply` or `Resolve` looks up, also where `Resolve` copies that id
into `AbilityUsed.outcome_summary` or a reason, and the spectator's one cell,
`set_viewpoint`, appends nothing. What holds each text instead:

| Field | What holds it |
|---|---|
| `TokenMoved.token_id`, `TokenPlaced.scene_id`, `TokenPlaced.actor_id`, `TokenRemoved.token_id`, `ActorRemoved.actor_id`, `DoorOpened.scene_id`, `DoorClosed.scene_id`, `ResourceChanged.actor_id`, `ResourceChanged.resource`, `ConditionApplied.actor_id`, `ConditionRemoved.actor_id`, `ConditionRemoved.condition_id`, `NoteDeleted.key`, `ActorControlGranted.actor_id`, `ActorControlRevoked.actor_id` | `engine.Apply` refuses a value that names nothing it has folded, so each equals an id or a name the table bounds |
| `Actor.controller_id`, `Actor.controller_ids` | `engine.Apply` refuses an `ActorAdded` that declares either, and fills them from `ActorControlGranted.participant_id` |
| `AbilityUsed.actor_id`, `AbilityUsed.target_ids` | `Resolve` refuses a value that names no folded actor |
| `ConditionApplied.source`, `ResourceChanged.reason`, `ConditionRemoved.reason` | `Resolve` writes each as `ability:` followed by an ability id and a branch label, `effect` or `usage`, or as `threshold:` followed by a resource name, and `internal/rules/load.go` bounds the id, the label and the name; `ToEvent` writes a `remove_condition`'s reason as `manual` |
| `AbilityUsed.outcome_summary`, `AbilityUsed.Roll.expression` | `Resolve` writes the first from an ability's display name, each target's id and, for an ability with a resolution, a branch label and two totals, and the second from the ruleset's compiled expressions; no loader bounds a display name or an expression |
| `TileRef.kind`, `TileRef.material` | `BuildSceneCreated` writes them from `StandardTile`'s vocabulary |
| `TileRef.art`, `SceneObject.art` | `BuildSceneCreated` writes an art id only when `internal/artlib` resolves it, and `isArtID` refuses one longer than `maxArtIDLen` |
| `SceneCreated.tiles` keys | `BuildSceneCreated` writes a key only for a square of the grid |
| `TokenHidden.token_id`, `SceneSeen.scene_id`, `SceneSeen.tiles`, `SceneSeen.visible` | none reaches the log: `internal/gateway/project.go` builds them for one viewer from folded state (SPEC-016) |
| `Envelope.event_id`, `Envelope.participant_id`, `Envelope.actor_role`, and `Envelope.session_id` but a `SessionStarted`'s | the server writes them: `newEventID`, the issuing participant's id and role, and `stampSessionIDAgainst`, which writes the open session's id or none |
| `AttackRolled.attacker_id`, `AttackRolled.target_id`, `AttackRolled.expression`, `AttackRolled.versus`, `AttackRolled.outcome`, `Modifier.source` | nothing in production writes an `AttackRolled`, and `engine.Apply` reads none of it |
| `SceneObject.kind` | nothing: it is a map's or an adventure's object `kind`, which no loader checks, and the fold keeps it; `describeBlockage` clips it where a player's move refusal names it (SPEC-013) |
| `Actor.module_data` | the WebSocket frame alone (`maxWSFrameBytes`, SPEC-011): `add_actor`, a command of the DM and the agent, is its one writer, and the fold keeps it; its shape is a rule module's (SPEC-007) |

The cells that say nothing or none rest on these searches, each a `git grep`
over `'*.go' ':!*_test.go' ':!contract/gen' ':!contract-spike'`: `-E
'TokenHidden\{|SceneSeen\{'` prints only `internal/gateway/project.go`;
`'AttackRolled{'` prints nothing; `ModuleData` prints nothing, so a
`module_data` reaches the log only inside an `add_actor`'s actor, which
`ToEvent` copies whole; and `-E 'o\.Kind|obj\.Kind' -- internal/mapdef
internal/adventure ':!*_test.go'` prints only the loader's and
`BuildSceneCreated`'s copies of an object's `kind`. `grep -n -E
'len\([^)]*(Name|Expr|Src|Description)[^)]*\) >' internal/rules/load.go`
prints only the resource-name bound.

A string a map, an adventure or a ruleset file carries and its loader does not
bound is under the same decision: a ruleset's display names, descriptions,
expressions, atom ids, param names and graph keys, its manifest's id and name,
and its guide; a map's or an adventure's `overrides` values and objects'
`kind` and `art`; and a map's placement `actor_id`. Where one reaches the log,
it is only as a field of the two tables above; a ruleset's display names,
descriptions and manifest id and name reach every participant through
`/api/ruleset`, and its guide through `/api/ruleset/guide` (SPEC-012). A
refusal that quotes such a value, or an id a command gave that names nothing,
goes to the issuer of the command that met it (SPEC-013), or, when a file is
refused at boot, to whoever starts the server.

**The refusal reaches the issuer.** `campaign.Append` folds the envelope
before it persists anything and returns the fold's error, so a command whose
event exceeds a bound is answered ok=false with that text and appends nothing
(SPEC-013's command path, VTT-161, VTT-162). A log that already holds such an
event does not fold, so `campaign.Open` refuses it.

**The client's fold mirrors every bound.** `fold.ts` calls `checkLen(what, s,
min, max)` for each of the twenty-four fields, an actor's resource and
attribute names one by one, with the same numbers as literals, in the same
order within each arm; `checkLen` counts UTF-8 bytes with `TextEncoder`, as
Go's `len` does, and throws `FoldError` reading `<field> exceeds <max> bytes`
or `<field> is shorter than <min> bytes`. Nothing ties a literal to its Go
constant but each side's tests.

**Other mirrors.** `internal/adventure/load.go` holds its own copies of
`maxNoteKeyBytes`, `maxNoteTitleBytes` and `maxTextBytes`, which
`TestSizeCapsMirrorEngine` pins to the literals 128, 256 and 8192, and of
`maxNameBytes`, which `TestTheNameBoundMirrorsEngine` pins to 256, and of
`maxIDBytes`, for an adventure's id, its scenes' and actors' ids and its
placements' token ids, which `TestTheIDBoundMirrorsEngine` pins to 128;
`internal/mapdef/load.go` holds its own `maxNameBytes` and `maxIDBytes`, the
second for a map's id and its placements' token ids, held by
`TestInvalidMapsAreRefusedWithAUsefulReason`,
`TestAMapNameOfExactlyTheBoundLoads` and `TestAMapIDOfExactlyTheBoundLoads`,
and through `CheckObjectIDs` a map's and an adventure's scene objects' ids,
held by the `object-id-too-long` rows, `TestAnObjectIDOfExactlyTheBoundLoads`
and `TestLoadAcceptsValuesExactlyOnEveryLimit`; `internal/rules/load.go` holds
its own `maxIDBytes`, for a ruleset's ability and condition ids, its
attribute, defense and resource names and its resolutions' branch labels,
which `TestTheIDBoundMirrorsEngine` pins to 128, and the ruleset schemas state
it, which `TestTheSchemasStateTheIDBound` requires; nothing compares any copy
with the engine's constants. A map, an adventure or a ruleset over one of them
is refused at load rather than at the fold, by an error naming the file and
the field. The MCP `move_token` tool describes its `reason` field as at most
256 bytes of UTF-8 (`manifest`'s `fieldDocs`), and
`TestTheMoveToolStatesTheFoldsReasonBound` requires the generated
`contract/gen/tools/tools.json` to state `maxMoveReasonBytes`; the `add_actor`
and `start_session` tools describe their name the same way, and
`TestTheToolsStateTheFoldsNameBound` requires it to state `maxNameBytes`;
`add_actor` describes its actor id and `place_token` its token id the same
way, and `TestTheToolsStateTheFoldsIDBound` requires them to state
`maxIDBytes`; `add_actor` describes its module id, attributes and resources
the same way, and `TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys`
requires them to state it too.

## Consequences

- A bound may be raised and never lowered: a log that folded under the old
  bound would stop folding.
- Changing a bound changes its mirrors in the same change: `fold.ts`'s
  literal, `internal/adventure`'s, `internal/mapdef`'s and `internal/rules`'
  copy where there is one, the ruleset schemas' statement of the id bound, for
  a move's reason the tool's description, for a name the `add_actor` and
  `start_session` descriptions, and for an id the `add_actor` and
  `place_token` descriptions and `add_actor`'s module id, attribute and
  resource descriptions.
- A field added to an event without a bound here is bounded by no fold: by the
  WebSocket frame when a command carries it, by nothing when a map, adventure
  or ruleset file does. It goes into the second table above, with what holds
  it, in the same change. Nothing checks that the second table is complete.
- No field joins the first table unless one of the two tests in How it works
  is met. A change that takes away what holds a text in the second table, or
  that lets a role other than the DM and the agent write one, puts that text
  to the tests again.
- A bound added to a field later refuses every log that already holds a
  longer value, as a lowered bound does; that cost is why a field joins the
  first table only on one of the two tests.

## Requirements

VTT-264, VTT-265, VTT-266, VTT-274, VTT-275, VTT-276, VTT-277, VTT-278,
VTT-279, VTT-281, VTT-282, VTT-283, VTT-284, VTT-285, VTT-286, VTT-287,
VTT-289, VTT-290, VTT-291, VTT-292, VTT-293, VTT-294, VTT-296, VTT-297.
