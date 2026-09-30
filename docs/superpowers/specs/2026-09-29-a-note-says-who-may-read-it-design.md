# A note says who may read it, and the projection honours it

## The problem

A note is a key, a title and a text, and nothing else: `UpsertNote` and
`NoteUpserted` (`contract/vtt/v1/commands.proto`, `events.proto`) carry those
three fields, `engine.Note` (`internal/engine/state.go`) holds `Title`, `Text`
and `UpdatedSeq`, and `client/src/state.ts`'s `Note` mirrors it. Nothing on a
note says who may read it. `classify` (`internal/gateway/project.go`) withholds
`NoteUpserted` and `NoteDeleted` from every player and spectator, and `Project`
hands the DM and the agent every event unchanged, so the DM and the agent hold
every note and every player and spectator holds none: `renderNotes`
(`client/src/view/spectator.ts`), which `renderSpectator` renders for every
seat, reads "No notes." for all of them. The adventure format calls
`notes/*.json` "the initially-REVEALED world notes"
(`docs/superpowers/specs/2026-07-26-adventure-format-design.md`), `Compile`
(`internal/adventure/compile.go`) emits them as `NoteUpserted`, and
`adventures/goblin-ambush/guide.md` tells the DM the party can already see
`ravine-trail-warning` at load; no player can. The DM console
(`renderDMConsole`, `client/src/view/dm.ts`) sends `upsertNote(key, title,
text)` and offers no choice of reader, and the MCP `upsert_note` tool, derived
from the contract (`tools/toolgen/main.go`, `cmd/vtt/tools.json`), offers none
either. The owner ruled on 2026-09-29 that a note carries a visibility, public
or DM-only, and SPEC-016's Status names this ticket as the one that lands it.
The owner also ruled, the same day, that a note recorded without a visibility
is DM-only, that a note loaded from an adventure is public, and that an
`upsert_note` naming no visibility is refused.

Not in this ticket: a third level (a note for one player or one owner), notes
a player may write, the testimony rule for actors (the next ticket), and the
skipped `client/e2e/handover.spec.ts` test that expects a player to see the
DM's note.

## Done looks like

1. `UpsertNote` and `NoteUpserted` each carry a visibility with two chosen
   values, public and DM-only, added without renaming or renumbering any field
   (`check:breaking` reports no break); the generated code, `cmd/vtt/tools.json`
   and the toolgen golden are regenerated and committed, and `check:drift` is
   green; the MCP `upsert_note` tool's schema lists the visibility as required
   with its two values.
2. An `upsert_note` naming no visibility is refused before anything is
   appended, for the DM and for the agent alike, with a message naming the
   field; a named gateway test shows it, and the same command with a
   visibility is accepted.
3. A player and a spectator are sent a public note's `NoteUpserted` and are
   sent nothing of a DM-only one, nor of one recorded without a visibility;
   named tests in `internal/gateway/project_test.go` show each, and the first
   fails on today's tree, where every note is withheld.
4. A note whose visibility changes reaches each player's and spectator's fold
   correctly: made DM-only, it leaves every fold that held it; made public, it
   appears. A deleted public note leaves every fold that held it; the deletion
   of a note no player or spectator holds reaches none of them, and no frame
   they are sent names the key of a note that was never public. Named tests
   show each, and every projected fold in the property and keystone tests
   still folds.
5. `engine.Note` and the client's `Note` record the visibility, `engine.Apply`
   and `client/src/fold.ts` agree on it, and the fold-parity and projection
   goldens pass with it in the dumps.
6. A note loaded from an adventure is public: each adventure's
   `goldens/compiled-batch.json` shows it, and a player's projected seat in a
   scenario that loads `goblin-ambush` holds `ravine-trail-warning`.
7. The DM console asks for the visibility when a note is saved and sends it;
   the notes panel marks, for a seat that holds DM-only notes, which ones they
   are; a player's and a spectator's panel lists the public notes. Named client
   tests show each, and `cmd/vtt/webdist` is rebuilt.
8. What an agent reads names the visibility: `get_state`'s description and the
   MCP server's instructions say what the field is.
9. SPEC-016's Status paragraph, its notes ruling and its Consequence about the
   notes panel describe the new behaviour; SPEC-007 and SPEC-013 state the
   refusal; every rule the sort accepts has a row cited by a test;
   `task check` whole is green.

## Rules this puts on the system

- A note records whether it is public or DM-only.
- An `upsert_note` that names no visibility is refused and appends nothing.
- A note recorded without a visibility is DM-only.
- A public note reaches every player and spectator.
- A note that was never public is named in no frame a player or spectator is
  sent.
- A note made DM-only leaves every player's and spectator's view that held it.
- A deleted note leaves every view that held it.
- A note's deletion reaches only a player or spectator whose view holds it.
- A note loaded from an adventure is public.
- The DM console sends no note until the DM has chosen who may read it.
- The notes panel marks every note that is not public as DM-only.
- The DM and the agent are sent every note event unchanged (VTT-176 holds
  this already).

## What it touches

In the order the change must go, one component after another:

1. The contract: `contract/vtt/v1/commands.proto`, `events.proto`; the
   generated `contract/gen/go/vtt/v1/`, `contract/gen/ts/vtt/v1/`,
   `contract/gen/tools/tools.json`, `cmd/vtt/tools.json`; `tools/toolgen/`
   (`main.go`, `main_test.go`), `contract/testdata/expected_tools.json`,
   `contract/testdata/upsert_note_command.json` and its round-trip tests.
2. The engine: `internal/engine/state.go`, `apply.go` and their tests;
   `internal/eventgen/model.go` and `model_test.go`; the note equality tests
   in `internal/campaign` and `internal/harness`.
3. Adventures: `internal/adventure/compile.go`, `compile_test.go`,
   `internal/adventure/conformance/conformance.go`, each adventure's and each
   conformance fixture's `goldens/compiled-batch.json`.
4. The gateway: `internal/gateway/convert.go`, `server.go` and a new
   `note_validate.go` (the refusal), `project.go` (the notes ruling and
   whatever memory it needs), and their tests.
5. MCP: `internal/mcp/read_tools.go`, `server.go` and their tests;
   `cmd/vtt/mcp_e2e_test.go`.
6. The client: `client/src/state.ts` (its header comment too), `fold.ts`,
   `commands.ts`, `view/dm.ts`, `view/spectator.ts`, their tests, and
   `cmd/vtt/webdist`.
7. Scenarios and goldens: `scenarios/story-table.json`,
   `scenarios/adventure-night.json` and their `scenarios/goldens/` trees.
8. Records: `docs/specifications/016-the-projection.md`,
   `007-the-wire-contract.md`, `013-authorization.md`,
   `docs/requirements.md`, `docs/verification-debt.md` (one clause on the
   note-key residue), `tools/mutation-equivalents.txt` and
   `tools/ts-mutation-equivalents.txt` where lines move,
   `tools/comment-ceilings.txt` by `--write-ledger` only.
   `adventures/goblin-ambush/guide.md` is left: its sentence becomes true.

This crosses more than two components and names more than ten files; the
owner ruled on 2026-09-29 to keep it one ticket, since the client's fold must
mirror the engine's in the same commit and a flag nobody can set or see is not
the ruling.

## Specifications this moves

docs/specifications/016-the-projection.md
docs/specifications/007-the-wire-contract.md
docs/specifications/013-authorization.md

## What could not be established

- The visibility's names. VTT-038 (`TestQANoEventPayloadNamesARole`) refuses
  an event enum value with the whole word `DM`, `AGENT`, `PLAYER` or
  `SPECTATOR`, so "DM-only" cannot be spelled `..._DM_ONLY` on the wire. An
  enum with an UNSPECIFIED zero, as `ActorKind` and `JoinDoor` do, is the
  precedent; the plan proposes the names.
- How a note made DM-only leaves a player's fold: `Project` judges an event
  against the state after it, where a deleted note is already gone, so the
  projection needs a memory of the notes each seat holds, and a frame that
  removes a note (a synthesized `NoteDeleted`, as `transitions` synthesizes
  `TokenHidden`) or a new projection-only payload. The plan decides.
- Whether the note-key residue (SPEC-012: a note upserted between an
  adventure's compile and its append is overwritten) now also overwrites a
  DM-only note with a public one, and whether that needs more than the record
  saying so.
- Which scenario golden a player's projected seat first shows a public note
  in, and how many of `scenarios/goldens/` move.
