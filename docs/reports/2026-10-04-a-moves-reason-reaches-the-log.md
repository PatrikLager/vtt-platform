# A move's reason reaches the log: the change

**Ticket:** `docs/superpowers/specs/2026-10-03-a-moves-reason-reaches-the-log-design.md`,
revised by its writer (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-03-a-moves-reason-reaches-the-log.md`,
verified by `verify-ticket` (Passes with gaps).
**The owner's rulings** at sign-off, of 2026-10-03: a player or spectator who
sees a move is sent it without its reason, the DM and the agent with it (Q1,
option a); a player may give a reason, which only the DM and the agent read
(Q2, option i); a forwarded frame's author, a move's included, goes to a
ticket not yet written (Q3; What was deliberately left out); and Q4 to Q8 as
the plan proposed. The ruling behind the ticket is the owner's of 2026-10-02,
on the removal record's review: the reason is added to `TokenMoved` rather
than dropped. The owner's ruling of 2026-09-29, only what a viewer sees gives
it information, bounds the whole.
**Last code commit:** `637caa0`, on `347796c`, `main` at the time. Every
code reference below is to that tree.

## The period, in commits

    git log --oneline 347796c..637caa0

    637caa0 The feed and the ticker show a move's reason
    904f79b A move's reason reaches the log, and no player or spectator is sent it

`git diff --stat 347796c..637caa0`: 30 files changed, 2261 insertions(+), 100
deletions(-).

The gate, `task check` whole, after `637caa0`, once: it exited 0 with no step
failed. Its check steps' own verdict lines read `check:comments` clean over 252
files, `check:requirements-chain` 263 rows, 208 test files and 11
specifications, `check:doc-owner` 80 files, `check:new-prose` 1415 added lines
clean, `check:coverage` 20 packages at or above their floors, `check:no-pack`,
`check:no-retraction` and `check:no-create-scene` clean, `task lint` 0 issues,
`check:breaking` reporting pre-release with no objection, `check:mutation` 14
packages with zero unadjudicated survivors, every package but `internal/artlib`
and `internal/campaigncfg` mutated afresh, since the contract's generated code
is in the others' closures (six mutants timed out and were counted as killed,
in `internal/sight`, `internal/rules` and `internal/mcp`), and
`check:ts-mutation` 2915 mutants, 2816 killed, 70 survivors all adjudicated
equivalent, zero unadjudicated, 29 timed out and counted as killed.
`check:mutation` began with 30.6 GiB free. `task check:drift` exited 0 after
each code commit.

## Done looks like, answered

1. `[x]` A `move_token` that carries a reason appends a `TokenMoved` whose
   reason is the command's:
   `internal/gateway/server_test.go#TestAMoveWithAReasonAppendsItsReason`
   sends one through the gateway and reads the event the agent is sent,
   which is the log's (VTT-176); it was red on `347796c`'s `ToEvent` with
   `appended reason = ""`. A move with no reason appends one with none:
   `internal/gateway/server_test.go#TestAMoveWithNoReasonAppendsNone`.
   `cmd/vtt/scenario_goldens_test.go#TestScenarioGoldenStreamsHaveNotDrifted`
   is green, so every golden stream is byte-identical.
2. `[x]` Per the owner's ruling, a player or spectator who sees the move is
   sent it without its reason, and the DM and the agent with it:
   `internal/gateway/project_test.go#TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt`
   (a player and a spectator perched on the mover, the forwarded frame equal
   to the event less its reason, the event unchanged, the DM's projector
   returning it with its reason) and
   `internal/gateway/server_visibility_test.go#TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt`
   over the wire, both red on a tree with the field and the copy and
   without `forwardable`. `TestTheDMReceivesEverythingUnchanged` still
   passes; it projects a `TokenPlaced` and sees no reason, so the unit
   test's DM half holds `Project`'s DM arm for a move.
3. `[x]` The client's feed and ticker show a move's reason when the frame
   carries one: `client/test/spectator-view.test.ts#the feed and the ticker
   show a move's reason`, red before `because` with `Received: "t1 moved to
   3,4"`.
4. `[x]` The contract change is additive: `TokenMoved` gains `string reason
   = 5` and `MoveTokenRequest.reason` changed its comment only; `task
   check:breaking` prints its header and no objection; the generated code
   was regenerated with `task generate:contract`, `tools.json` unchanged,
   and the generated code matches the protos (the gate paragraph's
   `check:drift`).
5. `[x]` `TestToEventMoveTokenProducesTokenMoved` gives a reason and asserts
   it; VTT-257 is withdrawn and VTT-260, without its qualifier, succeeds it;
   SPEC-013 says a move's reason is carried and a move without one records
   none (`grep -c 'is dropped' docs/specifications/013-authorization.md`
   prints 0); `docs/verification-debt.md`'s entry sits under
   `## 2026-10-02 — \`move_token\`'s \`reason\` never reached the log` with a
   **Closed by** line naming the two tests.
6. `[x]` `task check` whole is green, as the gate paragraph above records.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A move's reason reaches the event the move appends. | VTT-260, VTT-257's successor: the event a one-envelope command becomes carries every field the command gave, a grant's kind and a move's reason included |
| A move with no reason records none. | VTT-261 |
| A player or spectator is sent a move's reason only as the owner rules. | VTT-262: a player or spectator is sent no move's reason |
| The client shows a move's reason where it labels the move. | VTT-263: the feed and the ticker label a move with its reason when its frame carries one |

VTT-257 is withdrawn, its text kept, in VTT-035's form: its qualifier, "that
the event has a field for", exempted `move_token`'s reason, which
`TokenMoved` now carries; its fourteen citations moved to VTT-260.

## The sort

Four rows accepted (the plan's A to D, now VTT-260 to VTT-263), one withdrawn
(VTT-257) and nine candidates refused, as the plan's D9 lists them; none was
reversed. The refused: a reason reaching the event as its own row (A holds
it); the DM and the agent being sent it (VTT-176); projecting leaving the
event unchanged (VTT-222); a forwarded move being the event less its reason
and nothing else (how C is achieved, held by the unit test's equality); a
player being allowed a reason (the absence of a rule); the change being
additive (rule 3); a reason-less move labelled as before (D's table row); a
correction carrying no reason (VTT-249); the issuer's own seat seeing its
reason (not adopted).

## Phase 4a: QA adjudications

**C1, `904f79b` (VTT-260 to VTT-262, with VTT-176, VTT-206 and VTT-222), QA
on opus, given those rows, SPEC-013 and SPEC-016 whole, the text of
`TokenMoved` and `MoveTokenRequest` and `go doc -all` of
`internal/gateway`, `internal/engine`, `internal/campaign` and
`internal/identity`; it wrote `internal/gateway/qa_move_reason_test.go`.**
Twenty-three tests, none failing; twenty-eight injections into its own
file, all red, the file restored byte for byte.

- A reason of `""` against no reason: unobservable, since `TokenMoved.reason`
  has no presence and both record none; VTT-261 holds both and QA tested
  both. No change.
- VTT-262 holding for a projection that drops the move: forwarding is
  VTT-206's, and "every other field as the event has it" is held by the unit
  test's equality and QA's `TestQAMoveReasonPlayerIsSentTheMoveWithItsReasonCleared`;
  the sort refused it as a row. No change.
- The DM's same pointer and catch-up have no row of their own: SPEC-016
  states the first and `TestTheDMReceivesEverythingUnchanged` holds it;
  VTT-262 is unconditional and QA's catch-up test holds the second. No
  change.
- Requirements QA asked an id for: each is SPEC-013 or SPEC-016 text the
  sort refused or an existing row holds. None dispensed.
- The implementer changed QA's file: three comment lines rewrapped for
  `check:new-prose`, and a false warning about seat order deleted on the
  reading review's finding (the DM's and the agent's frames are the event's
  pointer, so their order changes nothing).

**C2, `637caa0` (VTT-263), QA on opus, given the row, the design doc's
problem paragraph and fourth rule, `spectator.ts`'s exported signatures and
the generated `TokenMoved` type; it wrote `client/test/qa-move-reason.test.ts`.**
Twenty-four tests, none failing; thirteen injections into its own file, all
red.

- A whitespace-only reason is a reason on the wire and is shown after the
  dash; a newline in a reason is kept and collapsed by CSS. No record decides
  either; the label shows what the log holds. No change.
- A frame with no `to` is labelled `moved to 0,0`: older than this ticket
  (What could not be established).
- A reason that begins with a dash doubles it: harmless.
- Requirements QA asked an id for: the exact reason-less label is
  `describe`'s table row; the reason rendering as text and never as markup is
  held by QA's test under VTT-263, and every sink of `describe` writes it as
  text. None dispensed.
- The implementer changed QA's file on the reading review's findings: one
  test deleted, which compared two equal messages and could not fail, and its
  three comments that cited "the ticket" made to name the design doc.

## The breaks

Each commit's message carries them, one line each, with the check that
spoke; each was run in a scratch clone carrying that commit's code and
restored after each. `904f79b` eight: the fixture's field misspelt; `ToEvent`
without its `Reason` line; `ToEvent` writing a default; `forwardable`
returning the event; `forwardable` clearing the event in place; the copy also
clearing `EventId`; the DM arm stripping too; eventgen giving no move a
reason. `637caa0` two: `describe`'s arm without `because`, which
`client:typecheck` also refuses (`TS6133`); `because` always appending.

## Rule 9: how MapTool does this

Answered in the plan, before any task ran, from `~/dev/RPTool/maptool`:
MapTool's move messages carry no annotation, and free text with an audience
is a chat message on a channel, a GM-only one included, relayed to every
client and filtered by the client. Borrowed: the audience, so a reason written
beside a move is for the DM and the agent. Refused: the client-side filter,
since here the projection never sends what a seat may not read, and a channel
chosen per message, which no client offers and the ruling does not need.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the plan's constraint: "The ticket and every report under `docs/reports/` are not edited." | the ticket's writer revised "Specifications this moves", which lost SPEC-007, and "What it touches", which gained the files the verification found the work reaches | Q7 left SPEC-007 unedited, and the verification's check 4 found the work reaching files the ticket did not list (the plan's gap 9) |
| D3: the two wire tests read the appended event "through `f.log`" | they read the event the agent's connection is sent | the agent's stream is the log unchanged (VTT-176), and the removal tests in the same package read appended events the same way |
| D7's `/** label, followed by the reason an event gives when it gives one. */` and D12's "one for `because`" | `because` has none | SPEC-010 allows a doc sentence on an exported symbol only, and `because` is not exported |
| D10: VTT-257's withdrawn cell dated 2026-10-03 | dated 2026-10-04 | the day the withdrawal was written |
| Task 13: `docs/reports/2026-10-03-a-moves-reason-reaches-the-log.md` | `docs/reports/2026-10-04-a-moves-reason-reaches-the-log.md` | the day the report was written |
| D15: QA for C1 given "`go doc ./internal/gateway`"; for C2 "the ticket's problem paragraph and fourth rule" | C1's QA got `go doc -all` of `internal/gateway`, `internal/engine`, `internal/campaign` and `internal/identity`; C2's the ticket's problem paragraph and all four rules | C1's QA could stand up a real server and fold a state only with those packages' signatures; the rules section was handed whole rather than cut |
| D12: "`--write-ledger` only if a share fell more than 1.0 under its row, which the probe tree did not" | C1's `--write-ledger` lowered seven rows by 0.1 to 0.8 and added a row for QA's file | the ledger was written before C1 regardless of the band, which lowers rows and never raises one; SPEC-010 holds a file with no row to the default ceiling, so the new row was not forced |

## What could not be established

- **A forwarded move still carries its author** (`ParticipantId`,
  `ActorRole`), which can name the participant behind a move (What was
  deliberately left out).
- **A reason is bounded only by `maxWSFrameBytes`**, where narration and note
  text have the fold's `maxTextBytes`; the plan's gap 7.
- **The MCP `move_token` tool's `reason` has no description** saying who
  reads it; `tools.json` is unchanged; the plan's gap 8.
- **No golden gives a move a reason**, so the keystone and the projected
  goldens do not hold VTT-262; the property walk does, on the seeds whose
  players are forwarded a reasoned move (its floor is two of six).
- **A frame with no `to` is labelled `moved to 0,0`**, a destination it did
  not carry; older than this ticket, and whether such a frame can occur is
  `handleCommand`'s and `engine.Apply`'s.

## What was deliberately left out, and where it went

- A forwarded frame's author, a move's included: the ticket the owner ruled on
  2026-10-01, not yet written, which
  `docs/reports/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it.md`
  records for a forwarded `ResourceChanged`'s reason and author; this
  sign-off's Q3 adds every forwarded frame's author to it.
- A size cap on a reason and a description on the MCP tool's field: nowhere
  yet; raised to the owner with this report.
- No file under `internal/engine`, `internal/campaign` or `internal/store`
  changed; the fold reads no reason.
