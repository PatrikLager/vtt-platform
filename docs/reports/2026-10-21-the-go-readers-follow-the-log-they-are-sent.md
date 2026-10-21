# The Go readers follow the log they are sent: the change

**Ticket:** `docs/superpowers/specs/2026-10-21-the-go-readers-follow-the-log-they-are-sent-design.md`,
verified by an agent that did not write it (passes with gaps G1-G13).
**Plan:** `docs/superpowers/plans/2026-10-21-the-go-readers-follow-the-log-they-are-sent.md`,
written by that verifier and amended by the reading review of `6dd30ea`, the
commit that carried both.
**The owner's rulings:**
- 2026-10-20: ticket 4 split, the browser client first and the Go readers
  next: "Två, klienten först";
- 2026-10-21: the three browser-client comments that point at SPEC-015 for
  the perch are re-pointed here, and the client's rules get a record of their
  own, SPEC-022, which these readers' rules join;
- 2026-10-21, at sign-off: "Ja, kör, samma regel", with Q1 to Q3 answered as
  recommended (SPEC-022; a dump switched before its head refuses, naming both
  logs; a `"log"` key, absent for the world) and Q4 to Q6 as the plan settles
  them;
- 2026-10-21, on the Task 6 review's S1 (below): "lämna det, inte ens som en
  skuld ... vi kommer inte bygga något som hantera detta 'edge case'".

**Last code commit:** `a1ca051`, on `a801300`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `6dd30ea` | Task 0: the ticket, the plan, VTT-409..VTT-426 |
| `ef44f0e` | Task 1: SPEC-022, and the client's pointers name it |
| `a141394` | Task 2: the harness client hands each switch, a dial names a log; QA's `qa_client_switch_test.go` |
| `c0a5680` | Task 3: the scenario engine holds one log per seat; QA's `qa_one_log_test.go`; the new scenario |
| `45711d4` | Task 4: the soak compares each log seat's fold; QA's `qa_soak_log_test.go` |
| `48a6fdb` | Task 5: `vtt mcp` holds one log and names it; QA's `internal/mcp/qa_one_log_test.go` |
| `a1ca051` | Task 6: `vtt state dump` folds one log and names it; QA's `qa_state_dump_switch_test.go` |

## What the code did on `a801300`

The gateway sent a player's or a spectator's connection one character log and
announced each switch with a `ViewpointChanged`; the browser client followed it
(VTT-394 to VTT-398). The Go readers did not. `Client.readLoop` in
`internal/harness/client.go` skipped every frame but a result, an event and
`CatchUpHead`, and `Dial` and `Dialer` had no way to name a `log`. So the
scenario engine and the soak saw a switch only as a restart at 1, a scenario
that switched a seat and reconnected it failed a correct gateway, and the open
debt entry "The harness cannot see a log seat switched on its old
subscription" recorded a gateway that announced a switch and kept the old log
passing every `cmd/vtt` scenario test. `vtt mcp`, seated as a player, kept
only entries above the old log's highest sequence after a switch, appended the
rest to the old log's history, and redialled with no `log`. `vtt state dump`
waited out `dumpCatchUpTimeout` when a switch came before its head and the new
log was shorter, and folded two logs when it was as long.

## Done looks like, answered

1. [x] `TestAClientHandsItsReaderEachSwitchInOrderWithTheEntries` in
   `internal/harness/client_switch_test.go`: three `ViewpointChanged`, to a1,
   a2 and nobody, handed in order with the entries (VTT-409). QA's `qa_client_switch_test.go` holds
   the same and more.
2. [x] `TestADialNamesTheLogItIsGivenAndNoneWhenItIsGivenNone` in the same file
   (VTT-410): the handshake carries `log` beside `after`, and no `log` key when
   none is given.
3. [x] `scenarios/a-player-switches-and-reconnects.json`, run by
   `TestScenarioLibraryRunsSelfContained`, passes on `a1ca051`. On the harness
   at `a801300` it fails at its `setViewpoint` step ("event (sequence 0) not
   observed matching by: dm") and at its first reconnect, and passes its
   second, as the verifier measured (G11) and the report's reading review
   measured again on an archive of `a801300`.
   Under the debt entry's recipe in `seat.watch` it fails at its first
   narration ("sequence 5 straight after a switch to act-bram") and at its
   first reconnect, measured in Task 3. The entry now sits under its own dated
   heading in `docs/verification-debt.md`, with what closes it.
4. [x] `TestASoakCheckpointFailsALogSeatWhoseFreshDialFoldsDifferently` in
   `internal/harness/soak_log_seat_test.go` (VTT-416), with the file's seven
   other tests, among them `TestASoakCheckpointDialsALogSeatNamingTheLogItHolds`
   (VTT-428, G13). The seed-1 keystone,
   `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts`, passes
   with its counts unchanged.
5. [x] `TestASwitchedPlayerIsAnsweredFromTheNewLogAlone` in
   `internal/mcp/one_log_test.go` (VTT-418, VTT-422), and against a real
   gateway `TestMCPSeatedAsAPlayerFollowsItsSetViewpoint` in
   `cmd/vtt/mcp_player_e2e_test.go`, which pages `get_events_since` from 0 and
   compares each entry with a fresh dial of the new log. With
   `internal/mcp` as it stood at `45711d4`, the e2e test is red (break M7).
6. [x] `TestARedialNamesTheHeldLogAndItsHighestSequence` in the same file
   (VTT-420, VTT-421): after `a1` to 3 and `a2` to 2, the redial asks
   `log=a2`, `after=2`.
7. [x] `TestADumpSwitchedBeforeItsHeadRefusesAndNamesBothLogs`,
   `TestADumpSwitchedAfterItsHeadPrintsTheLogItHeld`,
   `TestADumpNamesTheLogItIsTheFoldOf`,
   `TestAPlayersDumpNamesItsLogAgainstARealGateway` and
   `TestADumpToldItsOwnLogAgainFoldsNothingAfterIt` in
   `cmd/vtt/state_dump_switch_test.go` (VTT-423 to VTT-426, VTT-435). Before
   the code, the shorter new log waited until the fake closed and the as-long
   one failed while folding both logs. A world dump that sees no switch is
   held as the fold's fields plus `headSequence`, with no `log`, by QA's
   `TestQASDSWorldDumpCarriesNoLog`; `TestADumpNamesTheLogItIsTheFoldOf` holds
   that it has no `log` and the player's keys less `log`; and F5 also reddens
   `TestMCPSpecSevenExitCriteria`. No test holds its bytes:
   the Task 6 reviewer ran the old and new `writeDump` against one real gateway
   and found the DM's identical on `smoke.json` and on the new scenario, and
   the player's longer by exactly `"log": "act-ash"`. The golden corpus and the
   TS fold parity pass with no golden touched, which shows that the two
   mirrors and the goldens did not move (D14), not the dump's bytes.
8. [x] `grep -n 'SPEC-015' client/src/view/spectator.ts client/src/app.ts`
   prints nothing on `a1ca051` (exit 1).
9. [x] `task check` on `a1ca051` exits 0, in about 3200 s (Figures).
10. [ ] Half. SPEC-007's Status no longer holds the question: since `ef44f0e`
    it says "What a client holds and offers, its window included, is
    SPEC-022's." What these readers hold and what their redials name is the
    specification step, which follows this report (the plan's Task 9); the
    item stands.

## What the rules became

| The ticket's rule | Rows |
|---|---|
| A Go reader of a player's or a spectator's connection holds one log: at a `ViewpointChanged` naming another log, it discards what it holds and keeps what follows. | VTT-409 and VTT-427 (the harness client hands each switch and every entry); VTT-411 (the engine and the soak break a seat whose first entry after a switch is not 1); VTT-413 (a reconnect compares the current run whole); VTT-418, VTT-419 (`vtt mcp` discards at another log and keeps at its own). The dump does not discard and keep: it refuses before its head and stops after it (D13, Q2), VTT-423, VTT-425, VTT-435 |
| A Go reader's redial names the log it holds. | VTT-410 (`Dial`), VTT-414 (the engine's reconnect names it) and VTT-412 (it fails a redial told another), VTT-420 (`vtt mcp`), VTT-428 and VTT-429 (the soak's fresh dial); VTT-432 says the dump, which never redials, names none |
| `vtt mcp`'s redial resumes after the highest sequence it was sent of the log it holds. | VTT-421 |
| `get_state`, `get_events_since` and `vtt state dump` name the log their answer is of. | VTT-422, VTT-426 |
| A dump never prints the fold of more than one log. | VTT-423, VTT-424 (what the refusal names), VTT-425, VTT-435; VTT-433 says what else the dump reads past, and VTT-434 what its `headSequence` is |
| A soak checkpoint fails when a log seat whose stream has not broken its log folds it otherwise than a fresh dial of the same log. | VTT-416, VTT-430 (a fresh dial short of the head), VTT-431 (a run or a fresh dial that does not fold) |
| No rule of the ticket's: the plan's D8 | VTT-415 (an accepted step that names no sequence waits for no world seat) |

VTT-360 is withdrawn, succeeded by VTT-412 and VTT-413 (`c0a5680`), and
VTT-417 is withdrawn, succeeded by VTT-429 (`45711d4`). Every row's evidence
names its tests, and `check:requirements-chain` holds them. The sort refused
seven candidates, recorded in the plan. The rows the plan did not foresee came
from QA and review, one line each under Phase 4a and Phase 4b.

## Phase 4a

Five QA agents, one for each of Tasks 2 to 6. Each was handed its task's rows,
the governing specifications whole and the exported surface, and was withheld
the implementation's bodies, the task's tests, the plan's decisions and the
diff. Task 6's agent left the tree's hash, its own file excluded, as it found
it.

- Task 2, `internal/harness/qa_client_switch_test.go` (`qaCS`): 11 tests, no
  failure, 16 injections red.
- Task 3, `internal/harness/qa_one_log_test.go` (`qaOL`): 6 tests and 52
  subtests, 2 failing (findings F1 and F2), 9 injections, each red where QA
  expected red.
- Task 4, `internal/harness/qa_soak_log_test.go` (`qaSLS`): 15 tests with
  twins, on its own fake gateway, 1 failing (finding 1), 7 injection sets,
  each red where QA expected red.
- Task 5, `internal/mcp/qa_one_log_test.go` (`qaMOL`): 13 tests, no failure,
  28 injections red; it dropped three assertions of its own that nothing
  backed.
- Task 6, `cmd/vtt/qa_state_dump_switch_test.go` (`qaSDS`): 8 tests with
  no-switch twins, no failure, 13 injections red (its numbering skips I13).

### QA adjudications

- Task 2, gap 1: whether `after=0` is sent — no change; the client sends it,
  and SPEC-011 reads an absent `after` as 0.
- Task 2, gap 2: "sends no log" read as no key — no change; VTT-410's intent.
- Task 2, gap 3: an actor id that needs escaping — no change; it round-trips.
- Task 2, gap 4: the base URL against `/ws` — no change; outside the rows.
- Task 2, gap 5: the readers are not handed `WatchableChanged` — no change;
  D1.
- Task 2, gap 6: VTT-409's "each" for a world seat — no change; consistent
  with the row.
- Task 2, gap 7: a `Delivery` carries exactly one of its two fields — no
  change; it holds.
- Task 2, needing an id: "every entry it is sent, whatever its sequence or
  event id" — VTT-427 allocated.
- Task 3, finding F1: behaviour defect — a reconnect as the first step closed
  the old connection with its opening unread, so it named no log; a log seat's
  old connection is now drained before its history is split. Only an opening
  that names an actor shows it, and a gateway opens a fresh campaign's seats
  on nobody.
- Task 3, finding F2: behaviour defect, the same root — a first-step redial
  told another log passed; fixed by the same drain.
- Task 3, gap G1: nobody counts as another log than the one held — no
  change; VTT-412 as worded.
- Task 3, gap G2: a redial told the held log but sent it from its first entry
  fails as extra — no change; a gateway sends that only for a log no longer
  watchable, and then names another.
- Task 3, gap G3: a redial sent no `ViewpointChanged` passes — no change;
  SPEC-007 says one is always sent, and no row fails its absence.
- Task 3, gap G4: the first dial, a world seat's redial and a never-told
  seat's redial name no log — no change; consistent with VTT-414.
- Task 3, gap G5: replayed entries count as sent live for a later reconnect —
  no change; the history is rebuilt from them.
- Task 3, gap G6: an opening's entries belong to the run — no change; always
  so after F1's fix.
- Task 3, gap G7: any switch to another log in a replay fails — no change;
  VTT-412's "told".
- Task 3, gap G8: an accepted step naming a sequence, in a scenario with no
  world seat, passes — no change; outside the ticket.
- Task 3, note: "holds" and "current run" have no record yet — the
  specification step.
- Task 4, finding 1: behaviour defect — a fresh dial of a seat on nobody that
  was told an actor's log passed, since the told check ran only for a seat on
  an actor; `drainFreshCatchUp` now checks it whenever the seat was told a log.
- Task 4, gap G1: which checkpoint failed is shown by the FAIL line's number
  (VTT-383), and QA did not assert it — no change.
- Task 4, gap G2: a break is permanent — no change; D11's skip.
- Task 4, gap G3: any switch on a fresh dial is checked — no change.
- Task 4, gap G4: a seat never told gets a dial naming no log and no told
  check — no change.
- Task 4, gap G5: the fresh read ends at the live run's highest sequence — no
  change.
- Task 4, gap G6: a live character seat that overflows is not redialled — no
  change; older than this ticket, and outside it.
- Task 4, gap G7: SPEC-022's Status said the soak's dials name no log —
  rewritten in Task 4's commit.
- Task 4, needing an id: the fresh dial naming the held log — VTT-428.
- Task 5, gap G1: `CatchUpHead` does not move `vtt mcp`'s cursor — no change;
  the harness client hands its reader no `CatchUpHead`, and VTT-421 counts
  entries sent.
- Task 5, gap G2: the held log re-sent from 1 after a same-log
  `ViewpointChanged` is not folded twice — no change; `recordEvent` drops
  entries at or below its cursor.
- Task 5, gap G3: `headSequence` and `more` after a discard are stated by the
  tool descriptions alone — no change here; for the specification step.
- Task 5, gap G4: a seat between `CatchUpHead` and its opening
  `ViewpointChanged` is never told and answers as the world — no change.
- Task 5, gap G5: the first dial, at 0 with no log, is no redial — no change.
- Task 5, gap G6: "names no log" means an absent key — no change; as
  asserted.
- Task 5, stale prose: SPEC-022's Status said `vtt mcp` skips the switch —
  rewritten.
- Task 6, gap 1: a `ViewpointChanged` naming the held log is not "another
  log" — the dump ends its read at any after the first, which the gateway does
  send (Phase 4b, Task 6); VTT-435.
- Task 6, gap 2: a held log restarting at 1 with no switch — no change; no
  gateway behaviour.
- Task 6, gap 3: whether the refusal names actors by id or by name — no
  change; by id, as tested.
- Task 6, gap 4: VTT-424's "held nobody" half is unreachable from a real
  gateway, since a seat on nobody is told head 0 at `after` 0 and has reached
  it (D13) — no change.
- Task 6, gap 5: "the same JSON as today" — held as the fold's fields plus
  `headSequence` by QA's world test; no test holds the bytes (item 7) — no
  change.
- Task 6, gap 6: `--server` is the `/ws` URL — no change; outside the ticket.
- Task 6, gap 7: `headSequence` is 0 when nothing is folded — no change.
- Task 6, gap 8: "stops reading" has no time bound; QA asserted under 5 s
  against a trickling log — no change.
- Task 6, gap 9: a player and a spectator look alike on the wire — no change.
- Task 6, needing an id: the dump's dial — VTT-432; presence and watchable
  frames — VTT-433; `headSequence` past the head — VTT-434. Its two section
  pointers became those ids, and its tests joined VTT-423 to VTT-426's
  evidence.

## Phase 4b

One reviewer per task, at high effort, by command; each task's review ran two
passes. The review files of Tasks 0 to 5 were deleted by the machine restart;
this report's reading review rebuilt them from the previous session's record
and found the account below matching them.

- Task 0 (`6dd30ea`). First pass, 4 Must, among them a row that would have
  failed a correct stream and a soak row that contradicted the plan's own
  skip: the rows were removed, reworded, split where two obligations broke
  separately, and dispensed again in the plan's order; the tasks were
  renumbered so that the move to SPEC-022 comes before any code. Second pass,
  no Must.
- Task 1 (`ef44f0e`). Must: SPEC-007 said the window shows no sequence and
  left out VTT-393's exception; SPEC-022's Consequence 2 said "only" falsely,
  and named no search for it. Should: two pointers cited SPEC-022 for something it
  never said; test comments still cited SPEC-015; SPEC-022's Status said the
  readers' redials name no `log`, which does not fit the dump; SPEC-007's
  first Consequence. The second pass's Must: Consequence 2 said "A client"
  offers a switch only to a party member it may watch, while `vtt mcp` offers
  one to any id, since `set_viewpoint`'s `actorId` is a free string; it now
  says "The browser client".
- Task 2 (`a141394`). Must: a skipped switch restarted the quiet window in
  `drainQuietInto`, `drainToSequence` and `drainToHead` (a probe drained 2
  entries at 250 ms, not 1 at 100 ms), so each keeps one timer and has a
  `testing/synctest` test; `Dial`'s doc carried a claim, false since
  `121be93`, that `serve`'s lifetime is independent of the request's context;
  a descriptive helper comment. Should: QA's world-seat test stayed green under B1, so it
  left VTT-409's evidence; SPEC-022's Status and the open debt entry said the
  client drops a switch.
- Task 3 (`c0a5680`). Must: at a negative cursor `replayWanted` put a switch
  into what it wanted, and `RunScenario` panicked; `runReconnectStep`'s doc was wrong for a
  seat on nobody; a descriptive comment. Should: the probe fold read only a
  world seat's current run; the debt entry said "the harness" where only the
  engine held switches yet. The negative-cursor test was first written on a
  fixture with no switch inside the run and stayed green under its break;
  rewritten with a log told twice, it goes red.
- Task 4 (`45711d4`). Must: QA's nobody test cited VTT-416, which it does not
  hold; on the second pass, the two new tests cited VTT-416 for rules no row
  stated, so VTT-430 and VTT-431 were allocated. Should: a fold error and an
  unreached head had no test; VTT-417 left out a seat on nobody, so it was
  withdrawn for VTT-429.
- Task 5 (`48a6fdb`). Must, six: QA's fake read with `context.Background`; the
  e2e test survived M1, because `act-ash`'s and `act-bram`'s logs agree in
  payload over their shared prefix — it now compares each entry with a fresh
  dial; the docs of `Run`, `recordEvent`, `handleGetEventsSince` and
  `paginateSince`. The re-read found `recordEvent`'s new reason false (a
  catch-up sends each entry after its cursor once, VTT-176, VTT-349) and the
  e2e's M1 kill resting on a property no row states, that an opening's
  entries carry no event id; a narration that only `act-ash`'s log holds now
  sets the two logs apart in payload.
- Task 6 (`a1ca051`). Must: the sentinels' comment said "the two ways" a read
  ends short when there were three; and the implementer's claim that the
  gateway never sends a `ViewpointChanged` naming the shown log mid-stream was
  false. `seat.watch` keeps moving while a waiting wake resolves elsewhere, so
  a perch whose target's revocation commits inside `onPerch`'s window settles
  back on the shown actor, and the pump announces it again, that log
  following from 1: the reviewer's gateway probe, rerun by the implementer.
  Skipping such a switch survived every dump, drain and MCP test, so VTT-435
  and its test were added. Should: S1, below; two mirrors' docs said they
  match `writeDump` byte for byte, now true of a world seat's dump only; the
  commit message's wording. The second pass found a mutant that copies `vtt
  mcp`'s dedup at a same-log switch after the head (R7) surviving, because the
  after-head case went red only on a fold error; one more entry in its fixture
  reddens it. Left, with a reason: the after-head test sees F3 only when the
  switch reaches the dump inside the 300 ms quiet window, and QA's
  `TestQASDSSwitchAfterHeadStopsReadingAThrottledNewLog` holds VTT-425 without
  that dependency.
- Task 6, S1: a seat on nobody counts as caught up at once, so a dump whose
  opening `ViewpointChanged` reaches it more than 300 ms after its head prints
  with no `"log"` key. Measured on a real gateway, 300 dials of a spectator on
  nobody: 32 µs median and 0.2 ms at most at a load of 2.6, under 1 µs with 16
  CPU-bound processes on 8 cores and one Go thread, both frames then already
  buffered. The owner ruled it is not debt and nothing is built for it.

## The verifier's gaps, G1-G13

- G1, a command that appends nothing: VTT-415 and
  `TestACommandThatAppendsNothingPasses`; the scenario's `setViewpoint` step.
- G2, VTT-360: withdrawn, succeeded by VTT-412 and VTT-413.
- G3, a reconnect compared by `EventId` and `Sequence`: compared whole,
  `TestAReconnectComparesWholeEntries`; QA's old test is
  `TestQALSReconnectComparesWholeEntries`, expecting the reworded replay to
  fail.
- G4, `vtt mcp` keeping its log at a redial: VTT-419,
  `TestARedialToldTheHeldLogKeepsWhatWasHeld`, which the Task 5 review made
  require the fold and four entries paged.
- G5, the dump's key and the golden corpus: no golden moved (D14, Q3).
- G6, the dump against SPEC-007: refuse before the head (Q2), and stop at any
  later `ViewpointChanged` (VTT-435).
- G7, unnamed files and `gocyclo`: every task named its files; `gocyclo -over
  30` reported nothing after Tasks 3 and 4, and over `cmd/vtt` after Task 6,
  whose commit records golangci-lint; over `internal/harness`, `internal/mcp`
  and `cmd/vtt` the highest is 28, `RunSoak` and `RunScenario`.
- G8, SPEC-022: Task 1.
- G9, "blamed once": a broken seat is not compared again (D11);
  `TestASoakCheckpointFailsACharacterStreamThatSkips` holds it (break S6).
- G10, the Rule 9 correction: recorded in the plan; it holds.
- G11, item 3's `setViewpoint` step: measured by the verifier and again by
  this report's reading review, on `a801300`; D8 fixes it.
- G12, timing: the green run took about 3200 s; of the Go packages only
  `internal/mcp` was re-mutated, the other fourteen reusing their last passing
  verdicts, and the client was re-mutated in its container.
- G13, the seed-1 soak cannot show a fresh dial naming its log: a fake whose
  dial with no `log` opens another, in
  `TestASoakCheckpointDialsALogSeatNamingTheLogItHolds`, and QA's
  `TestQASLSFreshDialNamesTheLogTheSeatHolds`.

## The verifier's probes

The plan's measurements, G12's from earlier gate runs and the rest on a copy
of `a801300` before any code changed:

- G1: a `set_viewpoint` step returned ok with no sequence, then failed "event
  (sequence 0) not observed matching by: dm".
- G3: entries 1 to 3 of two different logs were byte-identical, and the
  reconnect matched three entries of the wrong log before it failed.
- D1: a switch to nobody was the opening `{"viewpointChanged":{}}`, with
  nothing after it until a grant.
- D4: a log seat's fresh connection opens with a `ViewpointChanged`.
- D5: every switch was followed by entry 1.
- D6: under the debt recipe the player was sent
  `{"viewpointChanged":{"actorId":"act-bram"}}` and then `act-ash`'s entry 5;
  without it, `act-bram`'s entries 1 to 5.
- D7: `after=3` with no `log` replayed `act-ash` from 1.
- D10: `act-bram`'s log after the switch held three opening entries with no
  event id, then the grant as 4 and the first narration as 5; on that tree the
  scenario fails at its `setViewpoint` step and at its first reconnect, and
  passes its second.
- D11: the seed-1 keystone took 13.7 s.
- D12: a redial naming the held log is sent a `ViewpointChanged` naming it.
- D14 and Q3: every golden stream is the DM's.
- The constraints: nearly every touched file sat at or within 0.1 of its
  comment ceiling, and a comment-only edit under `client/src` left
  `cmd/vtt/webdist` byte-identical.
- G12: `internal/mcp` re-mutates in about 857 s by the gate's own figure, and
  the client's last mutation run had taken 42 minutes 47 seconds.

This report's reading review ran the committed scenario on an archive of
`a801300` and measured G1, G3's wrong match ("catch-up[3] … at sequence 4")
and D10's result there again (item 3).

## The deliberate breaks

Each restored by copy and checked with `cmp`. For Tasks 1 to 4 the break
scripts and their output did not survive the machine restart (Deviations), so
those rows rest on the commit messages and the implementer's notes; the logs
of Tasks 5 and 6 survive. Task 5's breaks ran again after the restart, on a
tree whose hash matched the one taken before it; Task 6's ran only after it.

| Task | Break | Red |
|---|---|---|
| 1 | one perch comment set back to SPEC-015 | item 8's grep prints it |
| 2 | B1 the switch arm removed; B2 a switch held past the next entry; B3 nobody's switch skipped | `TestAClientHandsItsReaderEachSwitchInOrderWithTheEntries` |
| 2 | B4 `log` ignored; B5 `log` always set | `TestADialNamesTheLogItIsGivenAndNoneWhenItIsGivenNone` |
| 2 | each of the three readers back to a timer restarted at every pass | its `testing/synctest` quiet-window test |
| 3 | C1 `logBreak` keeping the run across a switch | `TestASeatThatAnnouncesASwitchAndKeepsTheOldLogFailsTheStep` |
| 3 | C2 comparing by event id and sequence | `TestAReconnectComparesWholeEntries` |
| 3 | C3 the redial's switch unchecked | `TestAReconnectToldAnotherLogFails` |
| 3 | C4 a named log expected from its first entry | `TestAReconnectOfAHeldLogIsComparedAfterTheCursor` and the scenario |
| 3 | C5 the run from the last entry of sequence 1 alone | `TestAReconnectOfASeatOnNobodyExpectsNothing` |
| 3 | C6 `dialerFor` dropping the log | the scenario |
| 3 | C7 the engine dialling with no log | `TestAReconnectNamesTheLogTheSeatHolds` |
| 3 | C8 waiting on world seats for sequence 0 | `TestACommandThatAppendsNothingPasses` and the scenario |
| 3 | the debt entry's recipe in `seat.watch` | the scenario, at its first narration and its first reconnect |
| 3 | no drain before a reconnect | QA's two first-step subtests |
| 3 | no nil check in `replayWanted` | `TestAReconnectOfAHeldLogIsComparedAfterTheCursor` ("got 3/4 events") |
| 4 | S1 the per-seat comparison skipped | the fold, naming, overflow and told-another tests |
| 4 | S2 the fresh dial naming no log | `TestASoakCheckpointDialsALogSeatNamingTheLogItHolds`, the overflow and told-another tests |
| 4 | S3 event ids compared instead of folds | `TestASoakCheckpointFailsALogSeatWhoseFreshDialFoldsDifferently` |
| 4 | S4 the fresh dial's switch unchecked | `TestASoakCheckpointFailsAFreshDialToldAnotherLog` |
| 4 | S5 the resume naming no log | `TestALogSeatsFreshDialThatOverflowsResumesTheSameLog` |
| 4 | S6 a broken seat compared again | `TestASoakCheckpointFailsACharacterStreamThatSkips`, `TestASoakCheckpointFailsALogSeatThatKeepsItsOldLogAfterASwitch` |
| 4 | S7 `soakHistories` dropping switches | the live-switch test and four more |
| 4 | the told check limited to a seat on an actor | QA's `TestQASLSFreshDialOfASeatOnNobodyToldALogFails` |
| 4 | a fresh fold error passed; a live one passed; an unreached head unchecked | `TestASoakCheckpointFailsAFreshDialThatDoesNotFold`; `TestASoakCheckpointFailsALogSeatWhoseLiveRunDoesNotFold`; `TestASoakCheckpointFailsAFreshDialThatEndsShortOfItsHead` |
| 5 | M1 no discard at a switch | `TestASwitchedPlayerIsAnsweredFromTheNewLogAlone`, `TestARedialNamesTheHeldLogAndItsHighestSequence`, `TestMCPSeatedAsAPlayerFollowsItsSetViewpoint` |
| 5 | M2 a discard at every switch | `TestARedialToldTheHeldLogKeepsWhatWasHeld` |
| 5 | M2b no told guard | `TestAPlayerOnNobodyIsAnsweredUnderAnEmptyLog` |
| 5 | M3 the redial naming no log | `TestARedialNamesTheHeldLogAndItsHighestSequence` |
| 5 | M4 the cursor kept across a switch | the same and `TestASwitchedPlayerIsAnsweredFromTheNewLogAlone` |
| 5 | M5 no log key | the four new tests of a told connection |
| 5 | M6 a log key on the world; M6b `get_events_since`'s `log` without `omitempty` | `TestTheAgentsAnswersNameNoLog` |
| 5 | M7 `internal/mcp` as at `45711d4` | `TestMCPSeatedAsAPlayerFollowsItsSetViewpoint` |
| 6 | F1 no stop at a switch | every refusal case and `TestADumpSwitchedAfterItsHeadPrintsTheLogItHeld` |
| 6 | F2 a refusal without the sentinel; F2b one naming neither log | the three cases of `TestADumpSwitchedBeforeItsHeadRefusesAndNamesBothLogs` |
| 6 | F2c nobody unnamed | its move-to-nobody case |
| 6 | F3 reading past a switch after the head | `TestADumpSwitchedAfterItsHeadPrintsTheLogItHeld` |
| 6 | F4 no log key | `TestADumpNamesTheLogItIsTheFoldOf`, the after-head test, `TestAPlayersDumpNamesItsLogAgainstARealGateway` |
| 6 | F5 a log key on the world | `TestADumpNamesTheLogItIsTheFoldOf`, the real-gateway test, and `TestMCPSpecSevenExitCriteria` |
| 6 | a switch naming the held log skipped; one after the head read on past the entries it repeats | `TestADumpToldItsOwnLogAgainFoldsNothingAfterIt` |
| 6 | the dump naming a log, and dialling after 1 | `TestQASDSDialNamesNoLog` |
| 6 | the harness client ending the stream at a `WatchableChanged`; at a `PresenceChanged` | every `TestQASDS` test; `TestQASDSFramesTakenByKind` |
| 6 | `headSequence` written as the announced head | `TestQASDSFramesTakenByKind`, `TestQASDSSwitchAfterHeadPrintsHeldLog` |

## Rule 9: how MapTool does this

The plan's answer, in its own Rule 9 section, holds, with its correction
(G10): MapTool has no reconnect at all, so the cursor and the `log` a redial
names have no counterpart there, and replace-on-arrival is what was taken.

## Figures

From the green `task check` on `a1ca051`:

- Go coverage: 21 packages at or above their floors, `internal/harness` 91.7%
  (floor 89.5), `internal/mcp` 89.9% (87.5), `cmd/vtt` 87.0% (85.0); the gate
  notes 11 that clear theirs by more than 1.0pp.
- Client coverage: 22 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors. `internal/mcp`,
  re-mutated: 3 survivors, all adjudicated, and 1 timed out and counted as
  killed, not measured. The other fourteen reused their last passing verdicts.
- Client mutation: re-mutated in its container, Stryker's own "Done in 31
  minutes and 22 seconds": 2957 mutants, 2882 killed, 9 timed out and counted
  as killed, 66 survivors all adjudicated equivalent.
- Race: both runs pass.
- Requirements chain: 435 rows, 283 test files, 16 specifications.
- Comments: 319 files, 176 comment lines added against `main`, clean; two
  notices. `cmd/vtt/events_tail_truncation_test.go` is at 22.83 against its
  22.6: it sat at 22.6 on `a801300`, and Task 2's edit dropped the contract
  import it no longer needed, one code line, with no comment line added.
  `internal/gateway/export_test.go` is at 49.18 against 45.9, as on
  `a801300`.
- Duration: about 3200 s, from the log's first write to its last, against
  3663 s for the last green run before.

## The soak's time

The seed-1 keystone, which now dials each log seat at every checkpoint, took
16.9 s at a load of 4 to 6 on Task 4's tree, against the 13.7 s the plan
measured before the change.

## The comment ceilings

`git diff a801300..a1ca051 -- tools/comment-ceilings.txt`: fourteen rows
lowered, twelve added, none raised.

| File | Ceiling |
|---|---|
| `client/src/fold.ts` | 33.9 -> 33.5 |
| `cmd/vtt/events_tail.go` | 29.6 -> 28.9 |
| `cmd/vtt/mcp_e2e_test.go` | 25.3 -> 24.9 |
| `cmd/vtt/state_dump.go` | 45.1 -> 26.8 |
| `cmd/vtt/state_dump_head_test.go` | 29.8 -> 27.7 |
| `internal/harness/client.go` | 48.3 -> 46.8 |
| `internal/harness/engine.go` | 30.1 -> 27.2 |
| `internal/harness/engine_test.go` | 17.6 -> 17.3 |
| `internal/harness/soak.go` | 42.7 -> 40.5 |
| `internal/harness/soak_test.go` | 34.9 -> 34.6 |
| `internal/mcp/guide_tool.go` | 30.8 -> 25.0 |
| `internal/mcp/read_tools.go` | 30.8 -> 24.0 |
| `internal/mcp/read_tools_test.go` | 24.9 -> 24.3 |
| `internal/mcp/server.go` | 48.1 -> 43.8 |
| twelve new test files, QA's five among them | new, 0.0 each |

## The history moved out of the rewritten comments

What D18 and rule 10 took out of code comments, by task. What stayed at the
line is a warning or a pointer.

- **Task 2, `Conn.CloseErr`.** As of 2026-08-06 every `Events()` consumer
  accounted for a closed channel, directly (`drainPreExisting`, the reconnect
  catch-up loop and its trailing check, `observeOnAll`, `recvWithin` and
  `collectBatchRun`, `drainAllForSilence`, `soakHistories.start`) or through
  `drainFreshCatchUp` (`drainToSequence`). An earlier version of the comment
  claimed that completeness while `soakHistories.start` still had the hole.
  Four readers had returned a wrong verdict: a truncated batch counted
  complete, a scenario denial proved by a deaf participant, a soak denial
  proved by stopped histories, and a reconnect at head certifying nothing
  replayed against a dead socket. Kept: a bounded wait proves a negative only
  against a live connection.
- **Task 3.** `drainPreExisting`'s eleven lines became four, the reasoning that
  a bounded wait proves a negative staying in `denialAbsenceWindow`'s doc.
  `RunScenario`'s six lines name VTT-411 to VTT-413 in place of VTT-360;
  nothing moved out of them.
- **Task 4.** `drainToSequence` replaced a drain that stopped at the first
  quiet window; `RunSoak`'s doc had listed generator and assertion failures as
  never an error.
- **Task 5.** `Server.history`: unbounded for v1, at table-top scale, with
  retention a ledgered future concern. `handleGetState`: it reproduced the
  dump's contract verbatim, per an earlier task brief; kept, keep it in step
  with `writeDump` by hand, since `internal/mcp` cannot import `cmd/vtt`.
  `marshalStateWithHead`: the `wireConnected` key came from an earlier final
  review's fix. `Run`: "the redial uses after=<last seen event sequence>",
  false after a switch. `recordEvent`: eight lines on the dedupe being reused
  rather than reimplemented, pointing at a `pump` doc that never said it.
  `pump`: it used to be an inline dedupe-and-advance check. `paginateSince`: a
  linear-scan argument resting on the retention note above.
  `handleGetRulesetGuide`: its contrast with `historySnapshot`, now deleted,
  and its account of `vtt mcp`'s `--ruleset` boot step. `mintInviteToken`: the
  shared identity-handle pattern of `mintInvites` and `serve_e2e_test.go`.
  `get_state`'s description named `headSequence` and `wireConnected` as "the
  two" keys beside the body.
- **Task 6.** `newStateDumpCmd`'s and `dumpAfter`'s merged block, which left
  the function undocumented: `headSequence` is the caller's staleness check,
  for sub-project 6's LLM tools comparing it with a sequence they know; the
  cursor is named because `drainToHead`'s caught-up test is `head <= after`,
  and a literal in two places is how they would stop agreeing.
  `drainToHead`'s 34 lines: a short read has two causes that call for
  opposite responses, the deadline (the server slow, or the head never came)
  and an early close (the client tore itself down because this process could
  not drain `Events()`, `ErrEventsOverflow` at 256 buffered); it used to
  return the same answer for both, and the caller blamed `dumpCatchUpTimeout`,
  printing a 30 s timeout for a connection that died in under three seconds;
  the same misdiagnosis cost a CI investigation in the soak before
  `drainFreshCatchUp` asked; stopping at head alone drops entries broadcast
  just behind it, and at quiet alone was the bug replaced; `head <= after` is
  not only head 0, since `Store.Subscribe` answers with the cursor itself when
  nothing newer exists; the function is a sibling of `drainToSequence`, not an
  import, because `cmd/vtt` depends on the harness and never the reverse. The
  sentinels: the overflow side already had `harness.ErrEventsOverflow` and the
  timeout side had nothing, which left a `strings.Contains` on the wording as
  the only assertion. `writeDump`: `st` is `harness.Fold`'s state, named only
  by inference, and the doc pointed at a `newStateDumpCmd` doc "on why cmd/vtt
  never imports internal/engine", which said no such thing; the round trip
  through a map keeps it generic over the state's type. `foldToDumpJSON` in
  `client/src/fold.ts`: Go emits the top level from a map, in sorted keys, and
  each struct in its field order, and the wrong order would fail a byte
  comparison that is otherwise the strongest check there is.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D5: a seat's switches kept beside its history, with an index | The history is a list of `Delivery`, switches in arrival order | A reconnect rebuilds the history from a prefix and what the redial sent; an index beside it would have to be re-derived there. D5's rules are unchanged |
| The quiet window untouched in Task 2 | `drainQuietInto`, `drainToSequence` and `drainToHead` keep one timer | Task 2's review: a skipped switch restarted the window, so a switch extended a drain |
| A reconnect redials at once | It drains the old connection first | Task 3's QA findings F1 and F2: a first-step reconnect lost the opening's switch |
| VTT-417 as dispensed | Withdrawn, succeeded by VTT-429 | It left out a seat on nobody, whose fresh dial names no log |
| VTT-409 to VTT-426 | Nine more, VTT-427 to VTT-435: VTT-429 succeeds VTT-417 (the row above); VTT-427, VTT-428 and VTT-432 to VTT-434 came from QA; VTT-430, VTT-431 and VTT-435 from review | Each is a rule a test held and no row stated |
| D17's e2e for `vtt mcp`: `get_state` folds `act-bram`'s grant, not `act-ash`'s | It compares each entry of `get_events_since` with a fresh dial of `act-bram`, after a narration only `act-ash`'s log holds | `act-bram`'s log carries `act-ash`'s grant too, so the plan's check could not be asserted, and its first form survived M1 |
| The plan names neither `historySnapshot` nor `readSnapshot` | A new `readSnapshot` gives both read tools one snapshot of history, head and log, and `historySnapshot`, which then only wrapped it, was deleted; its tests read `readSnapshot` | The history, its head and its log are copied together, so both tools see the same stream as each other and as `pump`; the deletion is Task 5's review |
| D13 and VTT-425: the dump stops at a `ViewpointChanged` naming another log | It stops at any after the first (VTT-435) | SPEC-007's snapshot sentence, and the gateway does send one naming the shown log, followed by that log from 1 |
| D9: probes fold participant 0's current run | A log seat's probes fold its current run, and a world seat's fold its every entry, as on `a801300` | Task 3's review: a DM that missed entry 1 would have had every probe judged against an empty state |
| The closed debt entry shows D10's scenario and VTT-411's test red under the recipe | It shows only the scenario red under the recipe, and names VTT-411's test as holding the row | The recipe edits `seat.watch` in the gateway, and VTT-411's unit test runs on fakes, so it cannot go red under it |
| Each task's files, as the plan lists them | Also: Task 1's comments in `client/test/app.test.ts` and `client/test/spectator-view.test.ts`; SPEC-022's Status in Tasks 2, 4, 5 and 6; Task 2's debt entry text and the new `internal/harness/switch_quiet_internal_test.go`; Task 5's `internal/mcp/guide_tool.go`, `cmd/vtt/mcp_e2e_test.go`, `read_tools_test.go`, `record_event_internal_test.go`, and a fifth test, `TestAPlayerOnNobodyIsAnsweredUnderAnEmptyLog`; Task 6's docs of `dumpJSON` in `internal/harness/fold_golden_test.go` and `foldToDumpJSON` in `client/src/fold.ts` | Per file: the test comments still cited SPEC-015 (Task 1's review); SPEC-022's Status named each reader as unchanged (Task 2's review, Task 4's QA gap G7, Task 5's QA, and the implementer in Task 6); the debt entry said the client drops a switch (Task 2's review); the quiet-window fix needed its own tests (Task 2's review); `guide_tool.go`'s doc named `historySnapshot` (Task 5's review); `mintInviteToken` now returns `mintParticipant`'s token (Task 5's review); `read_tools_test.go`'s response shapes gained the log (D14); `record_event_internal_test.go` read `historySnapshot`, which was deleted; the nobody half of VTT-422 had no test (Task 5's review); the two mirrors are true of a world seat's dump only, and `fold.ts`'s block was nine lines, so it was rewritten whole (Task 6's review). The new signatures also forced edits to their callers, such as Task 3's `soak.go` and Task 6's other callers of `drainToHead` and `writeDump` |
| `internal/mcp/read_tools_test.go`'s ceiling set in Task 5 | Lowered again in Task 6's commit, 24.7 to 24.3 | Task 5's re-read fix deleted a doc after that task's ledger was written |
| Task 5's tests, breaks and stress run straight after its review | Run after a machine restart | The machine ran out of ephemeral ports: 21 397 TCP connections stood in TIME_WAIT and did not expire, so every connect failed. The restart emptied the session scratchpad, which held the break script and the commit message; both were rebuilt from the session transcript, and the tree's hash matched the one taken before. M3 as first written did not compile, so its first red was the compiler's; it was rewritten so that its test reddens it. The tests ran once the load, above 10 for more than an hour after the restart, fell under 6, with Spotlight's indexing paused by the owner |

## What could not be established

- The `internal/mcp` mutant that timed out in the gate, a negated condition in
  `Run`, and the client's nine: counted as killed, and not evaluated.

## What was deliberately left out, and where it went

- Whether `vtt events tail` marks a switch: the ticket leaves its one-line
  format as it is.
- Whether `vtt mcp` tells an agent seated as a player whom it may watch
  (`WatchableChanged`): outside the owner's scope, and D1 keeps the frame
  ignored.
- A dump of a seat on nobody whose opening `ViewpointChanged` arrives after the
  quiet window: the owner's ruling, nothing built and nothing recorded.
- What these readers hold and what their redials name, in SPEC-022, and QA's
  notes for its text: the specification step, which follows this report.
- A live character seat in the soak that overflows is not redialled: older
  than this ticket and outside it (Task 4's QA gap G6). It is left unrecorded;
  whether it earns a ticket or a debt entry is the owner's to say.
- `cmd/vtt/events_tail_truncation_test.go`'s and
  `internal/gateway/export_test.go`'s comment shares, above their ceilings
  with no comment line added (the notices under Figures): the next change that
  adds a comment line to either has to bring it under.
