# SPEC-018: The fold bounds an event's free text in bytes

## Status

Accepted. Implemented by `internal/engine/apply.go` (the const block with
`maxNoteKeyBytes`, `maxNoteTitleBytes`, `maxTextBytes`, `maxNarrationAsBytes`,
`maxMoveReasonBytes` and `maxNameBytes`, and `Apply`'s `SessionStarted`,
`SceneCreated`, `ActorAdded`, `TokenMoved`, `NarrationAdded`, `NoteUpserted`
and `AdventureLoaded` arms) and its TypeScript mirror `client/src/fold.ts`
(`checkLen` and the same seven arms); mirrored for an adventure's notes,
opening narration and names by `internal/adventure/load.go`'s constants and
for a map's name by `internal/mapdef/load.go`'s `maxNameBytes`; stated to an
agent by `tools/toolgen/main.go`'s `manifest` for `move_token`'s `reason`,
`add_actor`'s actor `name` and `start_session`'s `name`. Pinned by
`internal/engine/apply_test.go`, `apply_boundary_test.go`,
`move_reason_internal_test.go`, `name_bound_internal_test.go`,
`internal/gateway/server_test.go`, `qa_move_reason_bound_test.go`,
`qa_name_bound_test.go`, `client/test/fold-rejections.test.ts`,
`qa-move-reason-bound.test.ts`, `qa-name-bound.test.ts`,
`internal/adventure/format_test.go`, `load_test.go`, `qa_name_bound_test.go`,
`internal/mapdef/load_test.go`, `qa_name_bound_test.go` and
`tools/toolgen/main_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a log is append-only, so what the fold accepts once it must accept
forever.

## How it works

**Ten fields are bounded, each in UTF-8 bytes, inclusive.**

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

`engine.Apply` measures each with `len`, which counts bytes, and refuses a
field over its bound, or empty where it may not be, with an error naming the
field, the bound and the length (`engine: move reason must be at most 256
bytes, got 257`; `engine: note key must be 1-128 bytes, got 0`). A
`TokenMoved` is checked for a known token, then for a destination at all, then
its reason. A `SceneCreated` is checked for a duplicate, then its name; an
`ActorAdded` for an actor with an id, a duplicate and a declared controller,
then its name; a `SessionStarted` for an open session, then its name.
These ten are the only texts `apply.go` bounds above; beyond them it requires
an `ActorAdded`'s actor id and a control event's participant id to be
non-empty. Every other text has no bound in the fold: one a command carries is
bounded by `maxWSFrameBytes`, the read limit on a WebSocket frame (SPEC-011);
one a map or adventure file carries is bounded only by what its loader checks,
which for an adventure's scene id is 128 bytes (`maxIDBytes` in
`internal/adventure/load.go`).

**The refusal reaches the issuer.** `campaign.Append` folds the envelope
before it persists anything and returns the fold's error, so a command whose
event exceeds a bound is answered ok=false with that text and appends nothing
(SPEC-013's command path, VTT-161, VTT-162). A log that already holds such an
event does not fold, so `campaign.Open` refuses it.

**The client's fold mirrors every bound.** `fold.ts` calls `checkLen(what, s,
min, max)` for each of the ten fields with the same numbers as literals, in
the same order within each arm; `checkLen` counts UTF-8 bytes with
`TextEncoder`, as Go's `len` does, and throws `FoldError` reading `<field>
exceeds <max> bytes` or `<field> is shorter than <min> bytes`. Nothing ties a
literal to its Go constant but each side's tests.

**Other mirrors.** `internal/adventure/load.go` holds its own copies of
`maxNoteKeyBytes`, `maxNoteTitleBytes` and `maxTextBytes`, which
`TestSizeCapsMirrorEngine` pins to the literals 128, 256 and 8192, and of
`maxNameBytes`, which `TestTheNameBoundMirrorsEngine` pins to 256;
`internal/mapdef/load.go` holds its own `maxNameBytes`, held by
`TestInvalidMapsAreRefusedWithAUsefulReason` and
`TestAMapNameOfExactlyTheBoundLoads`; nothing compares any copy with the
engine's constants. A map or an adventure over one of them is refused at load
rather than at the fold, by an error naming the file and the field. The MCP
`move_token` tool describes its `reason` field as at most 256 bytes of UTF-8
(`manifest`'s `fieldDocs`), and `TestTheMoveToolStatesTheFoldsReasonBound`
requires the generated `contract/gen/tools/tools.json` to state
`maxMoveReasonBytes`; the `add_actor` and `start_session` tools describe their
name the same way, and `TestTheToolsStateTheFoldsNameBound` requires it to
state `maxNameBytes`.

## Consequences

- A bound may be raised and never lowered: a log that folded under the old
  bound would stop folding.
- Changing a bound changes its mirrors in the same change: `fold.ts`'s
  literal, `internal/adventure`'s and `internal/mapdef`'s copy where there is
  one, for a move's reason the tool's description, and for a name the
  `add_actor` and `start_session` descriptions.
- A field added to an event without a bound here is bounded by no fold: by
  the WebSocket frame when a command carries it, by nothing when a map or
  adventure file does.

## Requirements

VTT-264, VTT-265, VTT-266, VTT-274, VTT-275, VTT-276.
