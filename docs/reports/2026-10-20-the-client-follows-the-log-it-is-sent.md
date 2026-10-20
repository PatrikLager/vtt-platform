# The client follows the log it is sent: the change

**Ticket:** `docs/superpowers/specs/2026-10-20-the-client-follows-the-log-it-is-sent-design.md`,
verified by an agent that did not write it (passes with gaps G1-G11).
**Plan:** `docs/superpowers/plans/2026-10-20-the-client-follows-the-log-it-is-sent.md`,
written by that verifier and amended by the reading review of `bd8f4a6`, the
commit that carried both.
**The owner's rulings, 2026-10-20:**
- ticket 4 split, the browser client first and the Go readers next: "Två,
  klienten först";
- the gateway sends a player or a spectator the party members they may
  watch, at connect and whenever that changes, never in any log, a spectator
  still opening on nobody: "Servern skickar listan";
- the question rule holds for this ticket: "Ja, samma regel";
- Q1, a creature left in a player's control after its run: asked first
  whether a non-party grant should revoke a player's control at once, and
  then whether control should decide kind, he ruled "ok, vi behåller det ...
  Vi fortsätter ha 2 stegslogiken som RPTool har det": a run still ends by a
  non-party grant and a revocation (VTT-389), and the client offers a player
  only the party members the gateway lists;
- Q2, a spectator's board click: "Bara knappar";
- at sign-off: "ja kör".

**Last code commit:** `982b4cc`, on `0db3d18`, `main` at the time. Every code
reference below is to its tree. The plan's last code commit was `9940d57`;
the two after it add one mutation adjudication and two tests, change no
source, and came from the gate (Task 7, under Deviations).

## The period, in commits

| Commit | What it holds |
|---|---|
| `bd8f4a6` | The ticket, the plan, VTT-394..VTT-408 |
| `ecc0c8a` | Task 1: `WatchableChanged` in the contract, its fixtures, the bundle |
| `16b897b` | Task 2: the gateway sends the set; QA's `qa_watchable_test.go` |
| `67f63c9` | Task 3: the client holds one log and redials it; QA's `qa-one-log.test.ts` |
| `6ff73ee` | Task 4: a player and a spectator switch; QA's `qa-switch.test.ts` |
| `b2e8683` | Task 5: the comment sweep |
| `9940d57` | Task 6: the e2e |
| `a8bb0ef` | Task 7: the equivalent mutant in `watchable`'s sort, adjudicated |
| `982b4cc` | Task 7: two client tests for the five TS survivors |

## What the code did on `0db3d18`

The gateway sent a player's or a spectator's connection one character log and
announced each switch with a `ViewpointChanged` (SPEC-007, SPEC-015). The
browser client dropped that frame (`Wire.handleFrame` had no arm for it),
kept the log it held and appended the next to it, so a switch whose new log
opened a session or introduced a scene again replaced the window with "Cannot
derive state from the log: session already open at sequence 1". A redial
dialled `?token=…&after=…` with no `log`, one sequence below the highest
seen, and rolled that sequence back first: the machinery of the projection
the character logs replaced. A player's chips only selected which controlled
actor a board click moved and whose abilities the panel offered, and nothing
sent their `set_viewpoint`; a spectator watching nobody held no log, so its
perch offered "No party members yet." and "No shoulder" and nothing else. No
frame named whom a connection may watch.

## Done looks like, answered

1. [x] `client/test/app.test.ts` "a switch to a log that opens a session and a
   scene the first held shows the second log's board", for a player and for a
   spectator. The verifier measured its shape red on `0db3d18` ("session
   already open at sequence 1"). It does not isolate the reset at a
   `ViewpointChanged` (see the breaks).
2. [x] `client/test/wire.test.ts` "a player's redial names the log it holds and
   the last sequence it folded" (the verifier measured `log` null and `after`
   N-1 on `0db3d18`), and "a DM's redial names its last world sequence and no
   log".
3. [x] `client/test/app.test.ts` "a redial the gateway opens on another log, or
   on the held log from its first entry, shows that log folded from its first
   entry".
4. [x] `client/test/app.test.ts` "a player switches by a button and by a click
   on the token": both send `set_viewpoint` naming the other party member.
5. [x] `internal/gateway/watchable_test.go`:
   `TestAConnectionIsToldWhomItMayWatchBeforeItsOpeningViewpoint` (a player,
   a spectator and a player who controls nothing),
   `TestTheSetIsSentAgainWhenItChanges` (a grant the watched character does
   not see, a revocation, a non-party grant, a removal, and none after a
   narration), `TestASpectatorIsToldOfEveryPartyMember`,
   `TestAPromotionTellsTheSeatTheNewRolesSet` and
   `TestTheDMAndTheAgentAreToldNoSet`. QA's `qa_watchable_test.go` holds the
   same and more.
6. [x] `client/test/app.test.ts` "a player is offered exactly the party members
   the set names" and `client/test/spectator-view.test.ts` "a spectator
   watching nobody is offered every party member the set names, and nobody".
7. [x] `client/e2e/table.spec.ts` "a spectator chooses a shoulder and is shown
   that log" and `client/e2e/handover.spec.ts` "a DM hands a second character
   over, and the player keeps both across a reconnect", its beat 3b; `task
   e2e`: 17 passed and the one test skipped before this ticket.
8. [x] The grep prints nothing on `982b4cc`, and so does the plan's G7 search
   over `client/src`.
9. [x] `task check` on `982b4cc` exits 0, the third run (Task 7, under
   Deviations); `task check:drift` green after each commit that rebuilt
   `cmd/vtt/webdist` (`ecc0c8a`, `67f63c9`, `6ff73ee`).
10. [ ] Not yet: the specification step follows this report (the plan's Task
    9), and moves SPEC-016 as well (G4).

## What the rules became

| The ticket's rule | Rows |
|---|---|
| A client holds one log at a time: at a `ViewpointChanged` naming another log than the one it holds, and when the log it is sent starts again from its first entry, it discards what it folded and folds what follows. | VTT-394 (another party member, or nobody), VTT-395 (sequence 1 again), VTT-396 (the held log is kept) |
| A redial names the log the client holds and the last sequence it folded from it. | VTT-397 (`log`), VTT-398 (`after`, the highest sequence since the last discard) |
| The gateway sends a player's or a spectator's connection the party members `MayWatch` accepts for it, at its opening and whenever that set changes, and sends a DM's or an agent's connection none. | VTT-399, VTT-400, VTT-401 |
| A player is offered a switch to each party member that list names, by its button and by a click on its token, and a spectator to each one and to nobody. | VTT-402, VTT-403, VTT-404 (labels), VTT-405 (a button sends), VTT-406 (a token click switches) |
| No rule of the ticket's: from the plan's decisions | VTT-407 (the window shows the last `ViewpointChanged`'s actor), VTT-408 (a player moves the character they watch) |

Every row's evidence names its tests; `check:requirements-chain` holds it.
The sort refused ten candidates, recorded in the plan.

## Phase 4a

Three QA agents, one for each of Tasks 2, 3 and 4. Each was given its task's
rows, the owner's rulings, the governing specifications whole and the exported
surface (`go doc` for the gateway, declaration files for the client), and was
withheld the implementation's bodies, the existing tests of its task, the plan,
the verdict and the diff.

- Task 2, `internal/gateway/qa_watchable_test.go`: 13 tests, all passing; 21
  injections into their own expectations, all red.
- Task 3, `client/test/qa-one-log.test.ts`: 27 cases from 23 declarations, 26
  passing and one failing; 21 injections, all red but one control.
- Task 4, `client/test/qa-switch.test.ts`: 23 tests, all passing once one was
  rewritten (finding 1); 20 injections, all red.

### QA adjudications

- Task 2, gap 1: the order within the set is unspecified — no change; the
  gateway sorts it by id (D2), for SPEC-015 to say.
- Task 2, gap 2: duplicate ids are not stated as forbidden — no change; the
  set is built from the actors map, keyed by id.
- Task 2, gap 3: where the set falls against the entries of a change that
  moves no seat — observed before them; for SPEC-015 to say.
- Task 2, gap 4: a grant and a revocation in back-to-back appends are coalesced
  by the wake into no set at all (40 of 40 runs) — not a defect: one waiting
  signal stands for any number of appends (SPEC-021), and `ViewpointChanged`
  behaves the same; VTT-400's "whenever" reads wider than that, and narrowing
  it would be a new row, which is the owner's.
- Task 2, gap 5: a same-id re-add in one batch sends a set naming the new name
  — no change.
- Task 2, gap 6: "no set when unchanged" rests on no row — refused at the sort
  as how (D3); its test stays.
- Task 2, gap 7: the set leaves out "nobody" — by design; VTT-403's button for
  nobody is separate.
- Task 2, needing an id: the order before the `ViewpointChanged` (refused as
  how, D4), the empty set's wire form (the contract's round-trips hold it), the
  converse of gap 6 (refused) — none allocated.
- Task 3, finding 1: behaviour defect, older than this ticket — a Reconnect
  whose dial fails left `session.reconnect()`'s rejection unhandled (bun
  aborted the test; a browser logs it uncaught) — both of `app.ts`'s dials now
  catch it, and the first dial's hole has its own test; its recipe is in
  `docs/verification-debt.md`.
- Task 3, gap 1: `CatchUpHead` does not move the cursor — no change; for
  SPEC-007 to say the cursor moves on entries.
- Task 3, gap 2: SPEC-007's "resumes and rolls back" — stale; the
  specification step rewrites it.
- Task 3, gaps 3 and 4: the "Perched on" name before the log names the actor,
  and how a player's window shows who is watched — Task 4 names it from the
  set and marks the chip.
- Task 3, gap 5: `after` while watching nobody — 0, as asserted.
- Task 3, gap 7: a `ViewpointChanged` sent to a DM — never sent (SPEC-007).
- Task 3: QA disclosed that bun's error print quoted six lines of `wire.ts`'s
  `onerror` handler; nothing else of the source was seen.
- Task 4, finding 1: spec ambiguity, in the brief — a DM's chips; QA first read
  "may act as ANY actor" off `actableActors`' doc comment and expected every
  actor, while the panel offers the actors granted to the DM, unchanged by this
  ticket — resolved to that; no change.
- Task 4, gaps: chip order (the set's, by id) and shoulder order (by label,
  then id); a click on the watched one's own token sends nothing; an unlisted
  actor's token moves the watched one onto its square; two listed on one square
  switch to the lowest id; an armed ability does not stop a switch, and the
  `ViewpointChanged` then disarms it; "Perched on" names by the set; a
  `ViewpointChanged` naming an actor outside the set marks no chip — for the
  specification step.
- Task 4, needing an id: Q1, Q2, the doors tool, the DM's chips and two
  strings of the window — held by existing tests or refused at the sort; none
  allocated.
- All three files: what was changed in QA's text is under Deviations.

## Phase 4b

One reviewer per task, by command on copies of the tree.

- Task 0 (`bd8f4a6`), three passes. Must: VTT-405 against the watched chip
  (D12 now sends on every chip); VTT-406 against armed doors and several
  members on one square; VTT-398's "last" against the kept out-of-order test;
  then a same-log restart that did not zero the cursor, which would skip
  entries on a redial cut short (D6 now zeroes it). Shoulds: a switch to nobody
  and VTT-397's nobody half were untested (tests and breaks C9, C10 added); two
  `spectator.ts` comments had no task; a list of client rows was false. The rows
  were removed and re-dispensed under the same ids, twice.
- Task 1 (`ecc0c8a`). No Must. `SetViewpoint`'s "only a spectator may watch"
  nobody was false (a player can watch nobody); `Actor.kind`'s "never on who
  holds it" was false of `MayWatch`'s player arm; a pointer named a path.
- Task 2 (`16b897b`). No Must. The set judged in `watch`'s settle loop was
  unpinned, and the DM test passed on a silent DM: both now tested. SPEC-011's
  and SPEC-015's mentions of `sendViewpoint` go to the specification step. One
  gateway failure in the reviewer's copy, its output not kept.
- Task 3 (`67f63c9`). Must: an empty `Session.close` survived a scoped Stryker
  run, its old kill an unhandled rejection that the new catch removed — a test
  now closes the socket. Shoulds: VTT-393 cited by tests that assert D9; an
  adjudication that miscounted the cursor's readers.
- Task 4 (`6ff73ee`). Must: the viewpoint sync's condition made true survived
  Stryker and would have kept every player from arming an ability — a test now
  arms one, then switches. Shoulds: `Switcher.current` unread; two comments
  false after the change; four evidence entries not asserting their row.
- Task 5 (`b2e8683`), two passes. Must: the first rewrite denied the removal
  fixture, which does carry a `tokenHidden` and a `sceneSeen`; `fold.ts`'s
  Explored block beside an edit still described the deleted projected corpus;
  then the fixture's one bare `actorRemoved`. Shoulds: `fold-unit.test.ts`'s
  never-folded list (re-measured), its section banner, its door passages.
- Task 6 (`9940d57`). Must: the spectator e2e's VTT-394 was unheld, since a
  spectator watching nobody has nothing folded to discard — a last "No
  shoulder" step now holds it. Shoulds: the stated reason Ash is watched; a
  check that could run before Ash's log arrived.
- Task 7 (`a8bb0ef`). No Must. Should: the entry's expiry named no toolchain,
  though its second leg rests on go1.26.4's sort. Nits: the loop's no-write
  premise unstated; the first measurement, a panic on two equal ids over the
  gateway package alone, could not show absence, because `openSeat` runs in
  the `/ws` handler and net/http recovers a panic there — re-measured over
  `./internal/...`, `./cmd/...` and `./contract/...` with an instrument that
  writes a file, which left none, and wrote one with its condition inverted.
- Task 7 (`982b4cc`). No Must; each of the five mutants reproduced in a copy
  reddens only its new test. Should: the session test cited VTT-402 and
  VTT-407 one way, rows that do not describe it — dropped. Nit: "the first
  actor granted" could not be shown with one grant — renamed.

## The verifier's gaps, G1-G11

- G1, the ordinary redial: VTT-396; "a redial the gateway opens on the held
  log after its cursor keeps what was folded".
- G2, a redial after a switch: "a redial after a switch resumes the new log,
  not the old one", and the case the plan missed, "a redial before the new
  log's first entry names the new log from its start".
- G3, a promotion: `TestAPromotionTellsTheSeatTheNewRolesSet`.
- G4, SPEC-016: the specification step.
- G5: `SetViewpoint`'s proto comment in Task 1, `commands.ts`'s in Task 5.
- G6, `look()`: the comments that named it were rewritten; item 8's grep is
  as the ticket wrote it.
- G7, the second search: prints nothing over `client/src`.
- G8, the bundle: rebuilt in Task 1, the contract change alone having moved it.
- G9, the unnamed files: all moved, with `qa-window-number.test.ts`'s
  `openWith`, `qa-one-log.test.ts`'s `openSeat` and the fixtures of D15.
- G10: names come from the set (D1); the set precedes the `ViewpointChanged`
  (D4); Q1; timing: the green run, its duration under Figures, reused every
  Go package's last passing verdict and re-mutated the client; the figure on
  record is 4501 s; the run on `a8bb0ef`, which re-mutated 13 of the 15 Go
  packages as well, took 7209 s.
- G11: Q2, buttons only.

## The deliberate breaks

Each restored by copy and `cmp`, and each bundle rebuilt afterwards
byte-identical. Each client break ran against the whole client suite. Task
2's first seven ran against `watchable_test.go`'s five tests by `-run`, the
announcement-order break among them against the whole gateway package; what
the two its review added ran against was not kept. Each count is what went
red in the suite as it stood when the break was made, before the task's QA
file and any test added after the break; re-run on the task's commit, some
count more.

| Task | Break | Red |
|---|---|---|
| 1 | the two-actor fixture's `actorId` renamed | its Go and TS round-trips |
| 1 | arm 7 given `json_name "watchSet"` | both fixtures' Go and TS round-trips, and `TestAnEmptyWatchableSetStillSerializesAsItsFrame` |
| 2 | `tell` returns early | all five new tests |
| 2 | `resolve` records no set | four |
| 2 | no world-seat guard | `TestTheDMAndTheAgentAreToldNoSet` |
| 2 | `engine.IsPartyMember` for `MayWatch` | three |
| 2 | the set queued after the announcement | eight, three of them `seat_internal_test.go`'s |
| 2 | no `proto.Equal` check | five |
| 2 | each actor named by its id | five |
| 2 | the set judged in `recheck`, not `resolve` | `TestASeatThatMovesWhileItSubscribesRecordsTheSetItSettledBy` |
| 2 | a world seat's pump silent after the head | `TestTheDMAndTheAgentAreToldNoSet` |
| 3 | no reset at a `ViewpointChanged` naming another viewpoint | three: `app.test.ts` "a spectator who switches to nobody is shown no board", `session.test.ts` "a ViewpointChanged naming nobody empties what was folded", and the count in `wire.test.ts` "a reconnect whose dial fails leaves the cursor and the log" |
| 3 | a reset at every `ViewpointChanged` | two |
| 3 | the sequence-1 reset skipped, its zeroing kept | two |
| 3 | no `log` in the URL | five |
| 3 | `after` one below the cursor | twelve |
| 3 | the cursor left alone at a switch | none at first; then the new "a redial before the new log's first entry names the new log from its start" |
| 3 | the indicator set from the ok result | VTT-407's test |
| 3 | the failure kept across a reset | "a reset clears the fold failure" |
| 3 | `log=` sent for nobody | "a redial while watching nobody names no log" |
| 3 | the cursor kept at a sequence-1 reset | two |
| 3 | `onError` without its paint | "a fold failure is drawn as soon as it happens, not at the next paint" |
| 3 | an empty `Session.close` | "close closes the socket" |
| 3 | the log id unencoded | "a redial names a log whose id needs encoding exactly" |
| 4 | the chips from the fold | VTT-402's test |
| 4 | a chip click that only selects | "a player switches by a button and by a click on the token" |
| 4 | a board click without the switch check | the same |
| 4 | the watched token moving | "a click on the watched character's token sends nothing" |
| 4 | the shoulders from the fold | five |
| 4 | the spectator's labels by id | fourteen |
| 4 | the player's labels by id | four |
| 4 | no "No shoulder" | five |
| 4 | a reset emptying the set | sixteen |
| 4 | the mover not following the viewpoint | two |
| 4 | the cell's row, or column, check made true | "a listed party member on the same row or column, not the same square, does not switch", each |
| 4 | a DM treated as a player | "a DM's chips pick from the fold and send nothing, and the board moves the pick" |
| 4 | the viewpoint sync's condition made true | "a player's armed ability stays armed, and a switch disarms it" |
| 4 | the armed ability kept across a switch | the same |
| 5 | a stale pointer put back in `fold.ts` | item 8's grep prints it |
| 6 | Task 3's bundle | both new e2e beats; every other test in `table.spec.ts` and `handover.spec.ts` passes, `handover.spec.ts`'s skipped one aside |
| 6 | no reset at a `ViewpointChanged`, in the bundle | the spectator e2e's last step |

## Rule 9: how MapTool does this

The plan's answer, in its own Rule 9 section, holds. The owner's Q1 answer
keeps MapTool's shape on a second axis: `Token.setType` and
`addOwner`/`removeOwner` stay separate, as kind and control do here.

## Figures

From the one green `task check`, on `982b4cc`:

- Go coverage: 21 packages at or above their floors; the gate notes 11 that
  clear theirs by more than 1.0pp.
- Client coverage: 22 files gated, each at 100.00%; 1 excluded.
- Client tests: 1263 passing across 43 files, from `bun test client/test
  contract contract-spike` on the same tree after the run, outside the gate,
  whose log of this run holds no bun count.
- Go mutation: 15 packages, zero unadjudicated survivors. All 15 reused their
  last passing verdicts, `internal/gateway`'s from the run on `a8bb0ef` (2
  survivors, both adjudicated); those verdicts hold 7 timed-out mutants.
- Client mutation: re-mutated in its Linux container, Stryker's run 42
  minutes 47 seconds: 2957 mutants, 2882 killed, 9 timed out and counted as
  killed, 66 survivors all adjudicated equivalent.
- Race: every package passes.
- Requirements chain: 408 rows, 271 test files, 15 specifications.
- Comments: 307 files, 112 comment lines added against `main`, clean; one
  notice, `internal/gateway/export_test.go` (49.18 against 45.9).
- Free disk: 19.0 GiB as the Go mutation gate began, and 18.8 GiB read with
  `shutil.disk_usage` once the run had ended.
- Duration: 3663 s.

## D4's and D5's measurements

- D4: on the plan's probe, the set queued after the `ViewpointChanged`
  failed three `seat_internal_test.go` tests; on the implementation, Task
  2's break of the same shape reddened eight, three of them
  `seat_internal_test.go`'s.
- D5: on the plan's probe, the frame in place with the three readers
  unchanged failed 27 gateway tests, and none once they were changed. Measured
  again on 2026-10-20, on a copy of `16b897b` with the three readers
  reverted: 27 gateway tests fail.

## The comment ceilings

`git diff 0db3d18..982b4cc -- tools/comment-ceilings.txt`: twelve rows
lowered, two added, none raised.

| File | Ceiling |
|---|---|
| `client/src/app.ts` | 60.4 -> 49.7 |
| `client/src/commands.ts` | 45.4 -> 43.2 |
| `client/src/fold.ts` | 40.8 -> 33.9 |
| `client/src/session.ts` | 51.0 -> 36.0 |
| `client/src/view/dm.ts` | 41.9 -> 40.7 |
| `client/src/view/player.ts` | 40.9 -> 27.6 |
| `client/src/view/spectator.ts` | 49.6 -> 46.3 |
| `client/src/wire.ts` | 66.6 -> 30.6 |
| `internal/gateway/seat.go` | 16.2 -> 16.0 |
| `internal/gateway/seat_internal_test.go` | 3.0 -> 2.9 |
| `internal/gateway/server.go` | 22.3 -> 22.1 |
| `internal/gateway/viewpoint.go` | 12.0 -> 11.5 |
| `internal/gateway/qa_watchable_test.go` | new, 2.4 |
| `internal/gateway/watchable_test.go` | new, 3.9 |

## Deviations

| Intended | Happened | Why |
|---|---|---|
| VTT-399 to VTT-404 and VTT-406 as signed | Each names `WatchableChanged` | So that each row stands alone, without "the set" |
| VTT-394, VTT-398 and VTT-406 as first dispensed | Reworded under the same ids before their commit | Task 0's review |
| D12 as signed: a click on the watched chip sends nothing | Every chip sends | VTT-405's text; VTT-375 answers a re-send with nothing, as the spectator's buttons already did |
| D6 as signed: the cursor never goes down within one log | Zeroed at a sequence-1 restart | Task 0's review: a redial cut short after a restart would ask past entries never sent |
| Task 1's second break, the empty fixture's arm dropped | Arm 7 renamed in JSON | `{}` round-trips trivially, so the plan's break left everything green |
| `events.test.ts`'s pointer to SPEC-016 | Names `TestEveryEnvelopePayloadArmHasAnExplicitRuling` | The control it compares with exists, by that name |
| C1 reddens item 1's test | It stays green | Every switched log starts at sequence 1, so VTT-395's reset covers item 1 as well; C1 is held by the three tests its row in the breaks names |
| C6 reddens G2's test | Nothing went red | The new log's first entry zeroes the cursor anyway; what C6 breaks is a drop between the switch and that entry, which now has its own test |
| D12: the selection follows a `ViewpointChanged` naming another actor | It follows the viewpoint at each paint | The same effect, written once |
| D13.3: nothing on a cell holding only the watched one's token | Nothing on any cell holding it | The same practical result; the plan's version moves it onto its own square |
| `Switcher` carries `current` | Dropped | Nothing read it; the mark comes from the selection |
| `mayWorkAnyDoor` keeps its copy of the current scene | It calls `currentSceneId` | One source, now that it is exported; its doc about three copies had gone stale |
| Task 5: item 8's pointers | Also `fold-unit.test.ts`'s account of the deleted projected corpus | Task 5's review found it false beside the edits; re-measured against every golden stream |
| The plan's "For Phase 4a": each QA agent given the specifications' governing sections | Each was told to read SPEC-007, SPEC-015 and SPEC-021 whole (Task 2: SPEC-007 and SPEC-015, with SPEC-021 and SPEC-016 allowed), the plan's sections named as the ones that govern | The installed process's `qa-prompt.md` hands QA "the whole spec" and forbids a section list, after `docs/verification-debt.md`'s 2026-09-16 entry; the plan's lists departed from it, and the change was not recorded when the briefs were written |
| `watchable_test.go` carries citation lines only | Four helper docs too | As `watch_test.go`'s helpers have |
| QA's files committed as written | Citation lines shortened in all three; the Task 3 file's "kept last" warning dropped | `check:new-prose`'s wrap; the abort it warned of no longer happens |
| `tableAs` seeds every role's table | A spectator opens on nobody and is sent no entry | The gateway's own opening (VTT-369, VTT-341) |
| Task 7: `task check` on `9940d57` exits 0 | `check:mutation` stopped it on one unadjudicated survivor, `<` -> `<=` in `watchable`'s sort; the entry went in as `a8bb0ef` and the gate ran again on that | The Go mutation gate runs only inside `task check`, which the plan runs at Task 7, so Task 2's sort met it there; no test can see the mutant (D17 kills one where a test can), since the set's ids are the actors map's keys and no two are equal |
| The second run, on `a8bb0ef`, exits 0 | `check:ts-mutation` found five survivors: in `app.ts`, the `ui` object emptied (`{}`) and its two initial `""` (`selectedActorId`, `selectedAbilityId`), and `Session`'s empty set and `viewpoint`'s `""`; two tests went in as `982b4cc` and the gate ran a third time | Each could be seen, so each was killed rather than adjudicated (D17). A player's `ui.selectedActorId` and armed ability are overwritten from the viewpoint at the first paint, so only a DM's board click before any chip still reads the initial values, and no test made one; the session's two defaults show only before a set or a `ViewpointChanged` arrives, or on a world connection, which no test asserted |

## What could not be established

- The gateway failure the Task 2 reviewer saw once in its copy of the tree:
  its output was not kept, and six full verbose runs at `-cpu 1,2` were green.
- Whether VTT-400 should say "at each judgment that finds a different set"
  rather than "whenever": QA's coalescing case, and the owner's to decide.
- What a DM's board move moves, and what "control" means for a DM's chips,
  beyond what the panel already did before this ticket (Task 4, finding 1).

## What was deliberately left out, and where it went

- The Go readers — the harness, `vtt mcp`, `vtt state dump` — and the open
  debt entry "The harness cannot see a log seat switched on its old
  subscription": ticket 4b, by "Två, klienten först".
- A non-party grant revoking a player's control, and control deciding kind:
  the owner kept the two steps (Q1).
- A spectator's board click: Q2, buttons only.
- The rest of `fold-unit.test.ts`'s history paragraphs: its terrain section's
  table is now marked as counted when it was written; the file was not swept,
  and needs no destination, since rule 10 binds comments under `internal/`,
  `cmd/` and `client/src`, not `client/test`.
- SPEC-007, SPEC-011, SPEC-015 and SPEC-016: the specification step, with
  QA's gaps for their text.
- `internal/gateway/export_test.go`'s comment share, above its ceiling as on
  `main` before this ticket (the notice under Figures).
