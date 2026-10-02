# Removing an actor has a record: the change

**Ticket:** `docs/superpowers/specs/2026-10-02-removing-an-actor-has-a-record-design.md`.
Its scope is the owner's choice of 2026-10-02: `server.go`, `convert.go` and
`join.go` together, the gateway's last three unswept production files, rather
than `server.go` alone. Its writer grew "What it touches" and "Specifications
this moves" after sign-off: by SPEC-011 and SPEC-009, as the verification's
gap 2 asked, and by `cmd/vtt/scenario_goldens_test.go` and
`docs/verification-debt.md`, per the sign-off's Q6 and Q10.
**Plan:** `docs/superpowers/plans/2026-10-02-removing-an-actor-has-a-record.md`,
verified by `verify-ticket` (Passes with gaps). Its twelve sign-off questions
were answered on 2026-10-02, all twelve as the plan proposed; for the disk
(Q11) the owner chose to clear the npm cache, which left 30.6 GiB free.
**The owner's ruling** during the work, on C1's reading review: `move_token`'s
`reason`, which `TokenMoved` has no field for, is the next ticket's, which
adds the field; this one narrows its row.
**Last code commit:** `597051b`, on `e52b4c2`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline e52b4c2..597051b

    597051b Removing an actor has a record, and the gateway's last three files are swept

`git diff --stat e52b4c2..597051b`: 15 files changed, 1092 insertions(+), 366
deletions(-).

The gate, `task check` whole, on the change before C1 was committed, the
review settled: it exited 0 with no step failed. Its check steps' own verdict
lines read `check:comments` clean over 251 files, `check:requirements-chain`
259 rows and 11 specifications, `check:doc-owner` 80 files, `check:new-prose`
94 added lines clean, `check:coverage` 20 packages at or above their floors,
`check:no-pack`, `check:no-retraction` and `check:no-create-scene` clean,
`task lint` 0 issues, `check:breaking` reporting pre-release,
`check:mutation` 14 packages with zero unadjudicated survivors,
`internal/gateway` mutated afresh (4 survivors, 4 adjudicated) and the other
thirteen reusing their verdicts, and `check:ts-mutation` verifying its stored
report (2909 mutants, 2808 killed, 70 survivors all adjudicated equivalent,
31 timed out and counted as killed). `check:mutation` began with 30.5 GiB
free. After the commit, `task check:drift` exited 0.

## Done looks like, answered

1. `[x]` `docs/specifications/017-removing-an-actor.md` states what
   `remove_actor` appends and in what order (pointing at SPEC-007 for the
   batch's shape), that the batch is appended whole or not at all, that
   `handleRemoveActor` checks nothing and `engine.Apply`'s `ActorRemoved` arm
   refuses in its place, and that no event ends a control grant; C1's
   reading review read each sentence against `handleRemoveActor`,
   `campaign.AppendBatch`, `store.AppendBatch` and `engine.Apply`.
   `grep -c 'has no record of its own' docs/specifications/013-authorization.md`
   prints 0; at `e52b4c2` it printed 1.
2. `[x]` `grep -c "(server.go's" internal/gateway/convert.go` prints 0; at
   `e52b4c2` it printed 1.
3. `[x]` Five rows the sort accepted are cited by tests under
   `internal/gateway/` that observe them, VTT-257 by a test in `cmd/vtt` as
   well; `python3 tools/check-requirements-chain.py .` prints `259 rows, 206
   test files, 11 specifications; every citation resolves and every row's
   evidence holds`. One row is OPEN: VTT-258, a removal appended whole, which
   no test drives through `handleRemoveActor` (What could not be
   established).
4. `[x]` `python3 tools/check-comments.py --report` prints `banned 0` and
   `blocks>6 0` for `server.go`, `convert.go` and `join.go`, and for every
   other production file under `internal/gateway/`; that every block left is
   a warning, a pointer or an exported symbol's doc sentence is C1's reading
   review's verdict (VTT-051), a reading and not a check. The three ledger
   rows were lowered by `--write-ledger` in C1, and no other row moved.
5. `[x]` The go/scanner token streams of the three files and of the test
   files C1 touched (`remove_actor_test.go`, `server_test.go`,
   `convert_test.go`, `cmd/vtt/scenario_goldens_test.go`), and of
   `server_internal_test.go` and `join_test.go`, are `e52b4c2`'s: 4657, 850,
   399, 1645, 13016, 2940, 1235, 6583 and 2323 tokens, each `same` (the
   plan's D10 loop).
6. `[x]` `task check` whole is green, as the gate paragraph above records.

## What the rules became

| Ticket's rule | Became |
|---|---|
| Removing an actor appends one `TokenRemoved` per token of that actor, in token-id order, and then the `ActorRemoved`. | two rows: VTT-254 (the content, no other event) and VTT-255 (the order) |
| A removal is appended whole or not at all. | VTT-258, OPEN |
| Removing an actor that does not exist is refused and appends nothing. | refused: the refusal is `engine.Apply`'s, and "appends nothing" is VTT-162 |
| A token placed for the actor after the snapshot the removal read makes the removal refused. | refused: the guard is `engine.Apply`'s, and the command's half is VTT-258 |
| Removing an actor ends every control grant on it, with no event of its own. | refused: "no event of its own" is VTT-254's last clause; "ends every grant" is structure, stated by SPEC-017 |
| Removing an actor with no token appends the `ActorRemoved` alone. | refused: VTT-254 at zero tokens, its test among VTT-254's evidence |
| A `set_join_door` that says neither open nor closed is refused and changes nothing. | VTT-256, "is refused"; "changes nothing" is stated by no specification, and only `docs/verification-debt.md` carries it, since no test observes it |
| A one-envelope command's conversion carries every field the command gave, a grant's kind included. | VTT-257, narrowed to "every field the command gave that the event has a field for" (Deviations) |

One row came from the plan rather than the ticket: VTT-259, a removal's
result carries the first sequence of its batch, as VTT-137 and VTT-175 are for
the other batch handlers.

## The sort

Five rows accepted (the plan's A, B, G, H and J, now VTT-254, 255, 256, 257
and 259), one OPEN (W, VTT-258), and seven candidates refused with two
clauses, as the plan's D4 lists them: C and D, the fold's refusals; E, the
grants ending; F, the zero-token case; I, the stamping of participant and
role, which no test observes; K, a fresh event id and one `OccurredAt`, which
are how; L, the snapshot's origin, which is SPEC-013's. None was reversed.

## The sweep

Comment lines over non-blank lines, counted by `tools/check-comments.py`'s
own `measure` with citation lines set aside, at `e52b4c2` and at `597051b`:

| File | Before | After | Banned | Blocks over the bound |
|---|---|---|---|---|
| `internal/gateway/server.go` | 358 / 959 | 203 / 804 | 8 to 0 | 11 to 0 |
| `internal/gateway/convert.go` | 59 / 165 | 10 / 116 | 7 to 0 | 3 to 0 |
| `internal/gateway/join.go` | 78 / 147 | 15 / 84 | 4 to 0 | 4 to 0 |

The test files gained citation lines only: sixteen new lines and two
extended (`TestDMGrantsControlOverTheWire`'s `// VTT-147`,
`TestScenarioGoldenStreamsHaveNotDrifted`'s `// VTT-240`). Ledger rows, old
to new: `server.go` 37.4 to 25.3, `convert.go` 35.8 to 8.7, `join.go` 53.1
to 17.9, and no other.

What the kept blocks are. `server.go`: on the `Server` type and `New`, their
doc sentences with a pointer; on the fields, a warning each for `buffer`,
`noProgress`, `writeTimeout`, `pingInterval`, `presence` and `onServeDone`
and a pointer for `encodeFrame` and `static`; above `announcePresence`,
`announceDeparture`, `revoked`, `announcePromotion`, `handleRemoveActor`,
`handleJoinDoor`, `handleRotateJoinLink`, `handlePromotion` and
`credentialGone`, a sentence naming the function with a pointer, and for
four of them a warning; inside them, warnings above `broadcast`, `deny`,
`announceIfPresent`, the stamping loop, the join door's refusal, the
admission limit, the rotation's result and the promotion's target check;
above the two sentinel errors, a warning. The 71 blocks the earlier sweeps
left were re-read and not rewritten. `convert.go`: `ErrUnknownCommand`'s and
`ToEvent`'s doc sentences, one warning above the type switch in place of
the four arm blocks that each said there was nothing to validate, and one
on the grant arm; `newEventID`'s doc went.
`join.go`: a warning or a pointer on each of its eight blocks.

Facts the cut blocks held that a record took: SPEC-017 took the batch's
construction from the snapshot, its stamping, its one append, the fold's
refusals in the handler's place and the grants ending with the actor;
SPEC-011 took that a presence frame that cannot be encoded is sent to nobody
and ends no connection; SPEC-013 took that a one-envelope command's event
carries the command's fields. VTT-020 already holds that rotating the link
leaves participants already admitted alone.

Facts dropped, each with the reason. History and argument (rule 10):
`writeTimeout`'s "how its absence went unnoticed"; the arrival's argument
for needing no absence re-check; the departure inversion measured at one in
20,000 rounds before the re-check, and that a reconnect is manual; the cost
of `revoked` per presence frame; why the promotion frame is a nudge, found by
the end-to-end test, and why it goes to everyone (the DM's console lists
roles too); `handleRemoveActor`'s reasons for the order (Go randomises map
iteration and the log is permanent), `handleLoadMap` as the precedent for
its shape, and the client freeze a token outliving its actor would cause
through `client/src/session.ts`'s re-fold; "nobody left at the table could
undo" a demoted DM; the join's "two calls it used to be"; `usableDisplayName`'s
argument that its frames reach logs, the CLI and the MCP surface, and the
bidi example; the spent-budget joiner's cost of not being told why; and that
the admission limit's pass-through keeps the CLI and the wire on one
default.

Four of the cut sentences were false, each established by command:
`convert.go`'s "`server.go`'s `handleUseAbility`" (it is in
`internal/gateway/ruleset.go`); `convert.go`'s claim that a kindless grant
"DEMOTES" its character and "fails closed" (`engine.Apply`'s
`ActorControlGranted` arm sets the kind only when the grant states one);
`server.go`'s `noProgress` field naming a `serveWS` that does not exist; and
`announcePresence`'s "the ONE delivery path that does not run through the
pump", which the catch-up head and every command result also are.

## Phase 4a: QA adjudications

Skipped, as the plan's D16 states and the sign-off accepted: no behaviour
changed, since every touched Go file's token stream is `e52b4c2`'s, so there
was nothing for an agent to derive tests against. What could be wrong was a
sentence, which C1's reading review held.

## The breaks

C1's message carries them, one line each, with the check that spoke; each
was run in a scratch clone of the change, committed there as that clone's
`main`, every gate first clean, and reverted by its inverse edit. Eight: a
`// VTT-999` citation; a row's evidence pointed at a file that does not carry
its id; SPEC-017 citing `VTT-999`; a comment line inside `handleRemoveActor`;
the token ids sorted in reverse; the false "(server.go's" sentence put back;
`Task 9` in a warning; a warning opening with another function's name.

## Rule 9: how MapTool does this

Answered in the plan, before any task ran, from `~/dev/RPTool/maptool`:
MapTool keeps a token's owners on the token, so removing it removes them with
it, which is this platform's shape (`controller_ids` on the actor) and was
taken as confirmation, not as a change. Refused: its client-first delete, its
silent skip of an unknown id and its unordered removal, each because here the
server's fold judges and the log must keep folding.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D4's row H: "The event a one-envelope command becomes carries every field the command gave, a grant's kind included." | VTT-257 reads "every field the command gave that the event has a field for"; SPEC-013 says `move_token`'s `reason` has none in `TokenMoved` and is dropped | C1's reading review called `ToEvent` with a `MoveTokenRequest` carrying a `reason` and found it missing from the envelope: `TokenMoved` has no such field, while `commands.proto` says the reason is shown in the log and the MCP `move_token` tool offers it. No cited test sets `Reason`. The owner ruled the field is the next ticket's |
| D2's Consequence: "a client receives a removal as contiguous `TokenRemoved` events then the `ActorRemoved`" | the DM and the agent do; what a player or spectator receives is SPEC-016's | `classify` withholds a `TokenRemoved` from every projected seat and forwards an `ActorRemoved` only to a viewer that saw the actor (C1's reading review) |
| D2: "an actor added later under the same id is controlled by nobody, since an `add_actor` that names a controller is refused" | SPEC-017 cites `engine.Apply`'s refusal of an `ActorAdded` naming a controller, whichever command carries it | `load_adventure` also appends `ActorAdded`; the fold bounds every path (C1's reading review) |
| D6's `writeTimeout` text: "with one knob the store always drops first, and the bound on a write is never exercised" | "with one knob the write bound fires first, and `TestAWedgedConnectionIsTornDownAndOthersKeepServing` stops reaching the store's drop" | C1's reading review measured the opposite: with both bounds equal the write's deadline fired first in every run, and the wedged-connection test still passed without reaching the store's drop |
| D3: "SPEC-013, three changes and its Requirements line (row H)" | its Status also names `convert.go` (`ToEvent`), `convert_test.go` and `cmd/vtt/scenario_goldens_test.go` | the Requirements line carries VTT-257, whose evidence those are (C1's reading review) |
| D8's `maxDisplayNameRunes` text: "a third of the room" | "as little as a quarter of the room" | a third is true only of three-byte scripts (C1's reading review) |
| D14's B2: "one new row's evidence re-pointed at a test that does not carry its id", expected red | pointed at a test in the same file it stayed green; pointed at a file that does not carry the id it went red | the gate reads that the file carries the id, as SPEC-008 states |
| D11: the dry run's ledger rows "37.4 to 25.4, 35.8 to 9.5, 53.1 to 17.9" | 25.3, 8.7, 17.9 | not established: the final texts in `server.go` and `convert.go` are D6's and D7's word for word except `writeTimeout`'s, which the review lengthened, so the dry run's drafts or their wrapping must have differed |
| Q10: one debt entry in C2 | two | the owner's ruling on C1's review sent `move_token`'s dropped `reason` to the next ticket, and the debt file carries it until then |

## What could not be established

- **No test observes that a removal is appended whole** (VTT-258, OPEN). The
  plan's probe P5 replaced `AppendBatch` with a loop of `campaign.Append`
  and `go test -count=1 ./internal/gateway/...` stayed green; no test under
  `cmd/vtt` drives `remove_actor` (the plan's check 3). The recipe for the missing test: an internal
  test that takes a snapshot, places a second token for the actor, then
  hands `handleRemoveActor` the stale snapshot; the whole batch must be
  refused and the first token must still stand. The verifier wrote it in a
  scratch clone, green at `e52b4c2` and red under P5; it was not committed,
  since this ticket changes no code.
- **No test observes the removal batch's participant and role, nor that a
  refused `set_join_door` leaves an open door open** (the plan's P8 and P9b,
  the gateway package green under each);
  `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` reads the door only
  from a shut start. `docs/verification-debt.md` carries both.
- **`add_actor`'s kind is observed only by
  `TestScenarioGoldenStreamsHaveNotDrifted`** in `cmd/vtt` (the plan's H4b);
  the gateway's `TestToEventAddActorProducesActorAdded` asserts the actor's
  id alone.

## What was deliberately left out, and where it went

- `move_token`'s dropped `reason`: the next ticket, which adds the field to
  `TokenMoved` and decides who sees it; `docs/verification-debt.md` carries it
  until then.
- Pointers outside the ticket's files left weaker, each the test-prose
  sweep's: `TestRemoveConditionAppliedThenRemoved`'s doc in
  `ruleset_test.go` ("see convert.go", whose comment is gone);
  `TestDecodeCommandMalformedJSONErrorsCleanly`'s bare line citation of
  `server.go` in `codec_test.go`; `TestEveryClientCommandConverts`'s string
  literal crediting `use_ability` to "(server.go)"; the doc and the
  `t.Fatalf` literal of `TestToEventGrantActorControlCarriesTheKind`, both
  saying a dropped kind reads as something it does not; and
  `remove_actor_test.go`'s package doc, which restates the cut comment with
  ticket tasks. Literals stay, since this ticket changes no token.
- No file under `internal/engine`, `internal/campaign`, `internal/store`,
  `contract/` or `client/src` changed.
