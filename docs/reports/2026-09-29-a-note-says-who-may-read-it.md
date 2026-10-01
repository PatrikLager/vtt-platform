# A note says who may read it: the change

**Ticket:** `docs/superpowers/specs/2026-09-29-a-note-says-who-may-read-it-design.md`,
corrected by its author at sign-off on four points: `conformance.go`'s path
written out in full; its seventh and ninth rules each split in two; its fifth
rule and its fourth item reworded to a note that was never public; and "What
it touches" given the new validator file, `compile_test.go`,
`internal/eventgen/model_test.go`, `client/src/state.ts`'s header and
`docs/verification-debt.md`.
**Plan:** `docs/superpowers/plans/2026-09-29-a-note-says-who-may-read-it.md`,
verified by `verify-ticket` (Passes with gaps). Its twelve sign-off questions
were answered on 2026-09-30, all twelve as the plan proposed: the values
`NOTE_VISIBILITY_PUBLIC` and `NOTE_VISIBILITY_SECRET`; a synthesized
`NoteDeleted` rather than a new `NoteHidden`; `describe` labels a
`noteDeleted` "withdrawn"; `omitempty` on `engine.Note.Visibility`;
specification sentences in the commit whose code makes them true; `task check`
whole after C3 and after C6; one clause in the residue's debt entry and
SPEC-012 untouched; `story-table` gains one step; the MCP instructions'
"human players can see everything you do" corrected; rows A, I, J and K
accepted with no specification naming them; `act-fighter`'s `why` without its
counts; and the console's choice blank, the DM answering each time.
**Last code commit:** `05bc2ae`, on `20c23f3`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline 20c23f3..05bc2ae

    05bc2ae A note can say who may read it: an adventure's notes load public
    ce31229 A note can say who may read it: the projection honours it
    cbccc57 A note can say who may read it: the command must state it
    3d6f56f The DM only mark is tested by the class its style rule selects
    106dcaf A note can say who may read it: the DM console asks, the panel marks
    5999523 A note can say who may read it: the folds record it
    ada8418 A note can say who may read it: the contract

`git diff --stat 20c23f3..05bc2ae`: 95 files changed, 5555 insertions(+), 446
deletions(-).

The gate, `task check` whole, ran three times. After `106dcaf` it failed
twice: first in `check:coverage` on
`TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio`, a
precondition failure in a fixture that starts late on a loaded machine, now a
debt entry; then, re-run, in the TypeScript mutation gate on a survivor in
`renderNotes`' "DM only" tag, which `3d6f56f` closed. That second run passed
every other step, `check:mutation` included. After `05bc2ae`: exit 0, no step
failed, and the check steps' own verdict lines read `check:comments` clean
over 247 files, `check:requirements-chain` 240 rows and 10 specifications,
`check:doc-owner` 80 files, `check:new-prose` 3699 added lines clean,
`check:coverage` 20 packages at or above their floors, `check:no-pack`,
`check:no-retraction` and `check:no-create-scene` clean, `task lint` 0 issues,
`check:breaking` reporting pre-release with no objection from buf,
`check:mutation` 14 packages with zero unadjudicated survivors, and
`check:ts-mutation` 2909 mutants, 2811 killed, 70 survivors all adjudicated
equivalent and zero unadjudicated. Six Go mutants and 28 TypeScript mutants
timed out and were counted as killed, with `uptime` reading a load average of
9 during the TypeScript run: four in `internal/sight`'s visibility walk, one
in `internal/rules`' expression evaluator, one in `internal/mcp`'s `Run`, and
the rest in `client/src/wire.ts`, `client/src/view/scene-plan.ts`,
`client/src/view/canvas.ts` and `client/src/session.ts`. None is in code this
change touched, and both gates' own text names a run starved of the machine as
one cause of a timeout.

## Done looks like, answered

1. `[x]` `UpsertNote` and `NoteUpserted` carry `visibility`, field 4, a
   `NoteVisibility` enum of `NOTE_VISIBILITY_UNSPECIFIED`,
   `NOTE_VISIBILITY_PUBLIC` and `NOTE_VISIBILITY_SECRET` (`ada8418`); no field
   was renamed or renumbered, and `check:breaking` reports no break. The
   generated code, both `tools.json` and `contract/testdata/expected_tools.json`
   are committed and `check:drift` is green after every commit. The
   `upsert_note` schema lists `visibility` as required with its two values:
   `tools/toolgen/main_test.go#TestUpsertNoteRequiredOverrideReplacesDerivedList`
   (VTT-232), and the toolgen golden's `enum`, held by
   `tools/toolgen/main_test.go#TestToolsMatchGolden`.
2. `[x]` An `upsert_note` naming no visibility is refused before anything is
   appended, for the DM and the agent alike, with a message naming the field:
   `internal/gateway/server_test.go#TestAnUpsertNoteNamingNoVisibilityIsRefusedForTheDMAndTheAgent`
   and `internal/gateway/note_validate_test.go#TestAnUpsertNoteWithNoVisibilityIsRefused`;
   every value the contract offers is accepted:
   `#TestEveryNoteVisibilityTheContractOffersIsAccepted` (VTT-231).
3. `[x]` A player and a spectator are sent a public note's `NoteUpserted` and
   nothing of a secret one or of one recorded without a visibility:
   `internal/gateway/project_test.go#TestAPublicNoteReachesEveryPlayerAndSpectator`
   (VTT-234), `#TestANoteRecordedWithoutAVisibilityReachesNoPlayer` (VTT-233),
   `#TestANoteNeverPublicIsNeverNamedToAPlayer` (VTT-235), and over the wire
   `internal/gateway/server_visibility_test.go#TestAPublicNoteReachesAPlayersConnectionAndASecretOneDoesNot`.
   The first fails on `cbccc57`'s projection, where every note is withheld.
4. `[x]` A note made secret leaves every fold that held it
   (`internal/gateway/project_test.go#TestANoteMadeSecretLeavesEveryFoldThatHeldIt`,
   VTT-236); made public, it appears
   (`scenarios/goldens/story-table/projections/act-hero`, where `kobold-den`
   is written secret and then made public, and
   `internal/gateway/project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`,
   which holds after every event that a seat's notes are the server's public
   ones); a deleted public note leaves every fold that held it
   (`internal/gateway/project_test.go#TestADeletedPublicNoteLeavesEveryFoldThatHeldIt`,
   VTT-237); the deletion of a note no seat holds reaches none
   (`internal/gateway/project_test.go#TestADeletionAfterANoteWasMadeSecretReachesNoPlayer`,
   VTT-238); and no frame names the key of a note never public (VTT-235).
   Every projected fold in the property and keystone tests still folds. A
   deletion and a note made secret reach a seat as the same frame
   (`internal/gateway/qa_note_projection_test.go#TestQANoteProjectionAPlayerCannotTellADeletionFromANoteMadeSecret`,
   VTT-239), which the ticket did not ask for; see Deviations.
5. `[x]` `engine.Note` and the client's `Note` record the visibility and the
   two folds agree: `internal/engine/apply_test.go#TestANoteRecordsTheVisibilityItsLatestUpsertStated`
   and `client/test/fold-unit.test.ts` "a note records the visibility its
   latest upsert stated, none included" (VTT-228); `client/test/fold-parity.test.ts`
   and `client/test/projection-parity.test.ts` pass over `story-table` and
   `adventure-night`, whose dumps carry it.
6. `[~]` A note loaded from an adventure is public, and each
   `goldens/compiled-batch.json` shows it
   (`internal/adventure/conformance/conformance_test.go#TestConformanceOverAdventuresGlob`,
   VTT-240). No committed golden holds a PLAYER's seat in a scenario that
   loads `goblin-ambush`: `adventure-night`'s one projected seat,
   `act-fighter`, is a spectator perched on the fighter, and it holds
   `ravine-trail-warning`. The player's seat is shown in-process instead, on a
   compiled `goblin-ambush` batch folded and projected for a player with the
   fighter, a player far from it and a player with no character:
   `internal/gateway/qa_note_adventure_test.go#TestQANoteAdventureEveryPlayerAndSpectatorSeatHoldsTheLoadedNote`.
7. `[x]` The DM console asks and sends: `client/test/dm-view.test.ts` "a note
   is not sent until who may read it is chosen", "a saved note carries the
   visibility chosen" and "a choice is not carried to the next note,
   re-rendered or not" (VTT-229); the panel marks every note that is not
   public: `client/test/spectator-view.test.ts` "the notes panel marks every
   note that is not public as DM only" (VTT-230); a player's and a spectator's
   panel lists the public notes by `renderNotes`' existing listing, held by
   "notes render with title and body" and fed by item 3's frames.
   `cmd/vtt/webdist` was rebuilt in `106dcaf`, and `check:drift` is green.
8. `[x]` `get_state`'s description names a note's `Visibility`
   (`internal/mcp/read_tools_test.go#TestGetStateDescriptionNamesNotesKey`)
   and its numbers, and the server's instructions say every `upsert_note`
   states who may read the note
   (`internal/mcp/server_test.go#TestServerInstructionsMentionNarrationAsTableMemory`).
   The numbers, and whether the sentences are true, are held only by the
   reading reviews of `cbccc57` and `ce31229`, since a string test cannot
   tell.
9. `[x]` SPEC-016's Status, its note rulings and its Consequence describe the
   new behaviour (`ce31229`); SPEC-007 and SPEC-013 state the refusal
   (`cbccc57`); every rule the sort accepted has a row cited by a test, and
   `python3 tools/check-requirements-chain.py .` holds over 240 rows;
   `task check` whole: green over `05bc2ae` (the period above).

## What the rules became

| Ticket's rule | Became |
|---|---|
| A note records whether it is public or DM-only. | VTT-228 (row A) |
| An `upsert_note` that names no visibility is refused and appends nothing. | VTT-231 (row B); "appends nothing" is VTT-162's, which B's wire test also cites |
| A note recorded without a visibility is DM-only. | VTT-233 (row C) |
| A public note reaches every player and spectator. | VTT-234 (row D) |
| A note that was never public is named in no frame a player or spectator is sent. | VTT-235 (row E) |
| A note made DM-only leaves every player's and spectator's view that held it. | VTT-236 (row F) |
| A deleted note leaves every view that held it. | VTT-237 (row G) |
| A note's deletion reaches only a player or spectator whose view holds it. | VTT-238 (row H) |
| A note loaded from an adventure is public. | VTT-240 (row I) |
| The DM console sends no note until the DM has chosen who may read it. | VTT-229 (row J), reworded "who may read that note" when QA's finding made the choice belong to one note |
| The notes panel marks every note that is not public as DM-only. | VTT-230 (row K) |
| The DM and the agent are sent every note event unchanged. | refused as a new row: VTT-176 holds it |

Two rows came from outside the ticket's list: VTT-232 (row L, the
tool schema requires `visibility`, from the ticket's first item) and VTT-239 (a
deletion and a note made secret reach a seat as the same frame, from Phase
4a's finding on `ce31229`). Rows A, I, J and K are named by no specification:
the engine's fold, the adventure compile and the client have no record, as
sign-off question 10 accepted.

## The sort

Twelve rows accepted, A to L, and fifteen candidates refused, as the plan's
"The sort" lists them. The fifteenth refusal, "a player cannot tell a
deletion from a note made secret", was refused as a Consequence of the plan's
D6 rather than a rule; the work showed D6 did not deliver it, and it became
VTT-239. VTT-216's evidence moved to `TestNarrationReachesAPlayer` when
`TestNarrationReachesAPlayerAndANoteDoesNot` was split.

## Phase 4a: QA adjudications

One line per finding, per commit's QA run, the commits named as the plan
names them; each QA agent was given the
rows, the governing text and the exported surface, never the diff, the source
or the existing tests.

**C2, `5999523` (VTT-228), QA on opus, from the ticket and the exported
surface only**
No failures: 11 Go and 17 TS tests, all green, each relied-on assertion
reddened by an injected fault in the test's own input or expectation.
- QA finding 1 (spec gap: how the dump writes a visibility): spec ambiguity —
  the dump is Go's json.Marshal of engine.State, which writes an enum as its
  number, as it does an actor's kind; the plan's D10 has get_state's
  description say so — no change.
- QA finding 2 (spec gap: a note with none has no Visibility key in the dump
  but 0 in the client's in-memory fold): spec ambiguity — by design, D2's
  omitempty tag, mirrored by foldToDumpJSON; the in-memory shape is not a wire
  shape — no change.
- QA finding 3 (spec gap: a value outside the enum, 7, is recorded as stated
  by both folds): spec ambiguity — by design, the folds store what the log
  said (D2) and the projection reads every value but PUBLIC as not public
  (C5), so an unknown value fails closed — no change.
- QA finding 4 (the Go dump function is not named to QA): no defect — the
  comparison against json.Marshal(engine.State.Notes) is what writeDump does —
  no change.
- QA finding 5 (no golden holds a stated visibility yet): no defect — true
  until C4, which gives story-table's notes stated visibilities, so
  fold-parity.test.ts and TestFoldGoldenCorpus hold the cross-language
  agreement from C4 — no change.
- QA finding 6 (Snapshot carrying the visibility is QA's reading of "folded
  state"): no defect — Snapshot copies Notes by value — no change.
- Owner's ruling: TestQANoteGoAndClientFoldsAgreeOnEveryNotesVisibility and
  TestQANoteClientDumpShowsTheVisibilityAsGoMarshalsIt removed before commit —
  they ran client/src/fold.ts under bun from internal/engine's tests, which
  check:mutation reruns per mutant, where a timeout under load is counted as a
  kill; the golden parity tests hold the same agreement from C4.
- Disclosure from QA: one command printed the import lines of existing
  client/test files (no bodies); a compile error revealed a helper name in
  qa_role_test.go (not opened).

**C3, `106dcaf` (VTT-229, VTT-230), QA on opus, from the ticket, the contract
and the exported surface; the DOM discovered by rendering**
12 tests; 11 green, 1 red.
- QA finding 1 (test 6, "a choice made for one note is not sent for the next
  note the DM writes"): behaviour defect — the visibility select kept its
  choice in the module-level draft after Save, so a note written after a
  public one went out public unless the DM changed it; owner's ruling
  2026-09-30: the choice belongs to one note — Save now resets the select and
  clears its draft key; VTT-229 reworded to "who may read that note".
- QA finding 2 (spec gap: a remembered answer surviving a re-render; the draft
  is module-level and leaks between test files in one bun process): no defect
  in the product — the draft is the console's deliberate re-render memory;
  QA's file resets it through the UI ("a fresh console shows no choice, and
  Save with a key and text sends no upsert_note" reworked in-process on the
  owner's cost rule, no child bun) — no change.
- QA finding 3 (spec gap: the refusal's wording): no defect — the ticket says
  the console asks; dm-view.test.ts pins the exact text — no change.
- QA finding 4 (spec gap: editing an existing note does not load its
  visibility): no defect — a flip is a re-save with the other choice (plan Gap
  1) — no change.
- QA finding 5 (spec gap: Delete independent of the choice): no defect —
  Delete sends only a key — no change.
- QA finding 6 (spec gap: "DM only" versus "DM-only"): no defect — the label
  is "DM only"; QA matches either — no change.
- QA finding 7 (a Note without Visibility is outside the typed contract): no
  defect — the fold always sets it — no change.

**C4, `cbccc57` (VTT-231, VTT-232), QA on opus, from SPEC-013, SPEC-007, the
contract and the exported surface; in-process only**
9 tests (5 gateway, 4 cmd/vtt through an in-process MCP server), all green; 19
injections all red. No failures.
- QA finding 1 (spec gap: a visibility the contract does not define, 3, is
  accepted and recorded): no defect — SPEC-013 states the validator accepts
  every value but UNSPECIFIED so the enum may grow, and the projection reads
  every value but PUBLIC as not public (SPEC-016 from C5), so it fails closed;
  the MCP layer does not validate calls against the schema, the gateway is the
  one guard — no change.
- QA finding 2 (spec gap: upsert_note's title is not in the tool's required
  list though SPEC-007's derivation rule would require an unannotated field):
  no defect of this change — toolgen's upsert_note override has always carried
  it ("Optional; may be empty."); SPEC-007 does not record the override; named
  here for a later record edit — no change.
- QA finding 3 (requirements needing an ID: validators run after Authorize for
  every role; refusals do not wrap ErrUnauthorized; a refused command appends
  nothing and leaves the connection open): no new rows — VTT-162 holds
  "appends nothing"; the rest are SPEC-013 prose this change did not move.

**C5, `ce31229` (VTT-233..238, then VTT-239), QA on opus, from SPEC-016,
SPEC-015, the contract and the exported surface; in-process**
14 tests incl. exhaustive 4096 two-key and 3125 one-key sequences, 60 random
logs mixing sight and control, a perch and a reconnect over the real server.
- QA finding 1 (F1, "a player can tell a deletion from a note made secret":
  the forwarded NoteDeleted carried the DM's event_id, occurred_at, actor_role
  and participant_id, the synthesized one only the sequence, so a client could
  infer a secret note still existed): behaviour defect — owner's ruling
  2026-09-30: classify withholds the real NoteDeleted and noteTransitions
  sends every holder the same bare NoteDeleted (key and sequence) for a
  deletion and for a note made non-public; SPEC-016 rewritten; VTT-239
  dispensed and cited by QA's test, which QA re-derived from the new text; 18
  injections red.
- QA finding 2 (G1, a built frame's fields unstated): spec ambiguity — closed
  by the new SPEC-016 text.
- QA finding 3 (G2, a perch sending no note frame unstated): spec ambiguity —
  SPEC-016 now says "A perch changes no note, so it sends no note frame".
- QA finding 4 (G3, VTT-238 assumes a reconnecting client kept its fold): no
  defect of this change — true of every withdrawal frame (TokenHidden alike);
  SPEC-015's resume is the record; named for it — no change.
- QA finding 5 (G4, a command putting a value outside the enum in the log
  untested here): no defect — C4's QA showed the gateway accepts it by
  SPEC-013's design and the projection withholds it — no change.

**C6, `05bc2ae` (VTT-240), QA on opus, from the ticket, the adventure-format
design, the contract, the adventure data and the exported surfaces;
in-process**
11 tests (6 adventure, 5 gateway through Projector for seven seats), all
green; 12 injections red; one branch uninjected (unreachable: Load refuses the
field).
- QA finding 1 (spec gap: the adventure-format design's "No read-visibility
  mechanism exists or is needed" is now false): no defect — that design is a
  ticket of 2026-07-26 and tickets are not rewritten; the present tense is
  SPEC-016 (the projection) and SPEC-013 (the command), and the compile has no
  record (plan Gap 3) — no change.
- QA finding 2 (spec gap: nothing says whether a note file may carry a
  visibility; strict decoding refuses it): no defect — the ruling keeps the
  adventure format unchanged, so the loader's unknown-field refusal is the
  format's own rule — no change.
- QA finding 3 (spec gap: the note-key residue untested): no defect of this
  change — SPEC-012 records it and docs/verification-debt.md carries it; C7
  adds the clause that the overwrite now takes the visibility with it (plan
  D8, Q7) — no change here.
- QA finding 4 (spec gap: an adventure with no notes): no defect — QA's own
  test pins that none is compiled — no change.
- QA finding 5 (requirements needing an ID: a loaded note keeps its file's
  key, title and text; a note loaded before any player holds a character still
  reaches that player; notes compile in declared order): no new rows — the
  first and third are the adventure format's own behaviour, held by
  TestCompileValidFixtureExactEnvelopeList and outside this ticket's rules;
  the second is VTT-240 with VTT-234, which QA's gateway test cites together.
- VTT-240's evidence gains QA's
  TestQANoteAdventureEveryPlayerAndSpectatorSeatHoldsTheLoadedNote, the one
  test that holds the ticket's "a player's projected seat holds
  ravine-trail-warning" from a fresh compile.

## The breaks

Each commit's message carries its breaks, one line each, with the check that
spoke; every break was reverted and the repaired tree ran clean. In summary:
`ada8418` two (the enum renamed `_DM_ONLY`, the fixture misspelt); `5999523`
five (each fold arm, the dump, `ToEvent`, eventgen); `106dcaf` six (the Save
guard, a constant visibility, the panel marking too little and too much, the
reset after Save, the select's reset alone); `3d6f56f` one (the tag's class);
`cbccc57` four (the call removed, the validator too strict, the schema's
required list, the refusal made to append); `ce31229` seven (four on the
projection's arms and memory, the real `NoteDeleted` forwarded to every seat
and to holders, the keystone's oracle); `05bc2ae` three (`Compile` omitting
the visibility or loading it secret, the dump dropping it), and two on QA's
tightened tests.

## Rule 9: how MapTool does this

Answered in the plan, before any task was dispatched, from
`~/dev/RPTool/maptool` at `f4b7fef6c`; the ticket carries no rule-9 answer. MapTool
has no free-standing note: a `Token` carries two text fields, `notes` and
`gmNotes`, and the only gate is `Token.getGMNotes`, which returns them to a GM
or a trusted macro and an empty string to anyone else; `Token.toDto` writes
`gmNotes` whenever it is set, whatever the recipient, so every client receives
them. Borrowed: the
two-level split between the GM and everyone, and a label on each note saying
who sees it. Refused: the whole-campaign distribution with a client-side
getter, which rule 9 names; visibility as two text fields, since a note here
is a titled record the server includes or drops whole; and MapTool's lack of
write enforcement, since SPEC-013 gates commands by role. MapTool's nearest
default is a GM preference for whether a new token is visible to players;
here a note has no default, and the console and the tool ask every time.

## The goldens, and how each was made

| Golden | Commit | Made |
|---|---|---|
| `contract/testdata/note_upserted_envelope.json` (new) | `ada8418` | by hand |
| `contract/testdata/expected_tools.json` | `ada8418`, `cbccc57` | regenerated, read against the plan's D1 and D5 |
| `contract/testdata/upsert_note_command.json` | `106dcaf` | by hand |
| `scenarios/goldens/story-table/state.json` | `cbccc57` | by hand from `scenarios/story-table.json` |
| `scenarios/goldens/story-table/stream.json` | `cbccc57` | edited to the recorded stream |
| `story-table/projections/act-hero/stream.json` | `cbccc57`, `ce31229` | the projection's emitted stream, read against SPEC-016 before commit |
| `story-table/projections/act-hero/state.json`, `viewer.json` | `cbccc57`, `ce31229` | by hand |
| the five `compiled-batch.json` | `05bc2ae` | by hand, before the code |
| `scenarios/goldens/adventure-night/state.json` | `05bc2ae` | by hand |
| `scenarios/goldens/adventure-night/stream.json` | `05bc2ae` | edited to the recorded stream |
| `adventure-night/projections/act-fighter/stream.json` | `05bc2ae` | the projection's emitted stream: one entry, the note's upsert at 9 |
| `adventure-night/projections/act-fighter/state.json`, `viewer.json` | `05bc2ae` | by hand |

Six more `viewer.json` changed in `05bc2ae` by one clause each: their
paragraph on what reaches a seat before its actor enters now names a public
note beside narration.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D6: `classify` forwards a real `NoteDeleted` to a seat whose memory holds the key, and `noteTransitions` synthesizes one only for a note made not public | `classify` withholds every `NoteDeleted`, and `noteTransitions` sends every holder the same bare `NoteDeleted`, key and sequence only, for a deletion and for a note made not public; VTT-239 dispensed | Phase 4a on `ce31229` found the forwarded deletion carrying the DM's `event_id`, `occurred_at`, `actor_role` and `participant_id` while the synthesized one carried only the sequence, so a player could tell that a note it had read still existed, made secret. The owner ruled the bare deletion in both cases. D6 had named the two as indistinguishable in content and not compared their envelopes. |
| D13's every sentence true at every commit, against D5, D10 and Task 4, which place four strings in C4: the refusal message, the tool's `visibility` description, `get_state`'s description and the instructions' sentence | the four strings in `cbccc57` say what a public note does before `ce31229` made it so | the plan contradicted itself; the owner ruled them written once, in `cbccc57`, rather than written for C4 and rewritten for C5; between the two commits a public note reached fewer seats than they promised, never more |
| Row I's evidence: the keystone's `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`, and D17's C6 break reddening the `act-fighter` golden | the evidence is `cmd/vtt/scenario_goldens_test.go#TestScenarioGoldenStreamsHaveNotDrifted` with the compile and conformance tests and QA's gateway test | the break left the keystone and `TestFoldGoldenCorpus` green: both read the committed `stream.json` rather than compiling afresh, so only the drift test, which runs the server, sees `Compile`'s output reach the log |
| the plan's commit list, C1 to C7 | an eighth commit, `3d6f56f`, between the console and the command | the whole-gate run after the console commit found a survivor in `renderNotes`' tag that no test held |
| Task 7: `task check` whole exits 0 after C3 | it never did: the first run failed on the MCP fixture now in the debt file, the second only in `check:ts-mutation`, `3d6f56f` closed that survivor, and the run was not repeated | the owner ruled the TypeScript mutation gate re-run in the whole gate after C6, which `3d6f56f`'s message records |
| the residue's debt clause (Q7), placed in no commit by the plan | in `05bc2ae` | that commit is the one that makes an overwritten note public, so its reading review asked for the clause there |
| the plan's Done-when counts: 238 rows after C5, 239 after C6 | 239 and 240 | VTT-239 was dispensed in C5 |
| C2's QA tests committed whole | two Go tests that ran the client fold under `bun` from `internal/engine` were left out | the owner ruled it: `check:mutation` reruns that suite per mutant, where a subprocess timing out under load counts as a kill; the golden parity tests hold the same agreement |
| the console's choice asked for each note, per D9 | the same, and reset after every Save | QA on `106dcaf` found a choice carried to the next note, so a secret note written after a public one went out public; the owner ruled the choice belongs to one note |
| `05bc2ae` touches `conformance.go` and the drift test for the note alone | `conformance.go`'s package doc states its envelope-metadata sentence in three lines instead of four and names its stampers, `handleLoadAdventure` and `campaign.AppendBatch`, and `TestScenarioGoldenStreamsHaveNotDrifted`'s doc comment is a warning and a pointer | the note example's second line took `conformance.go` over its comment ceiling; gofmt's separator before the drift test's citation touched a doc block over SPEC-010's bound, and SPEC-010 keeps a warning or a pointer |

## What could not be established

- **The note-key residue is untested.** A `load_adventure` whose note key
  collides with a note upserted between its snapshot and its append
  overwrites that note, and now makes it public with the adventure's text;
  `docs/verification-debt.md` carries it, with the clause this change added.
- **No corpus gate holds note steps.** A denied `upsert_note` in a scenario
  could carry the refused shape; the gateway's refusal holds every
  ok-expecting step.
- **The keystone never sees `NOTE_VISIBILITY_UNSPECIFIED`**, since the corpus
  goes through the gateway, which refuses it; VTT-233 is held by its unit
  test, the property test and QA's sequences.
- **`upsert_note`'s `title` is not required** although SPEC-007's derivation
  rule would require an unannotated field; toolgen's override has always
  carried it, and SPEC-007 does not record the override. Found by QA on
  `cbccc57`, outside this change. No destination is open: SPEC-007 records
  none of toolgen's per-tool overrides, and this one is named here for its
  next edit.
- **The adventure-format design's "No read-visibility mechanism exists or is
  needed"** (`docs/superpowers/specs/2026-07-26-adventure-format-design.md`)
  is no longer true. It is a ticket, and tickets are not rewritten; SPEC-016
  holds the present tense.
- **`internal/gateway/scenario_test.go` is not gofmt-clean**, since before
  this change; no gate runs gofmt, and nothing here touched the file.

## What was deliberately left out, and where it went

- A third level (a note for one player or one owner) and notes a player may
  write: out of the ticket; no ticket.
- The testimony rule for actors: the next ticket, recorded in
  `docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`
  and named in SPEC-016's Status.
- A per-note "make public / make DM only" control in the console: a flip is a
  re-save with the other choice; an agent flips one in one call. No ticket.
- `client/e2e/handover.spec.ts`'s skipped test fills the note form and saves;
  un-skipped, it must choose a visibility first. Out of the ticket, as the
  ticket says.
- The late-starting shell in the `TestQAMCPConnect*` deadline fixture,
  `cmd/vtt/qa_e2e_wait_test.go`: a debt entry in `docs/verification-debt.md`,
  committed with this report.
