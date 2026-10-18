# A seat streams its character's log: the change

**Ticket:** `docs/superpowers/specs/2026-10-17-a-seat-streams-its-characters-log-design.md`,
verified by an agent that did not write it (passes with gaps G1-G10; G11 was
added by the reading review of the first commit).
**Plan:** `docs/superpowers/plans/2026-10-17-a-seat-streams-its-characters-log.md`,
written by the verifier, amended by the ticket's writer after the owner's
answers and before `c73ab61` committed it.
**The owner's rulings:**
- 2026-10-15, for the arc: four tickets in order, one active character at a
  time, no dual path, no migration; 2026-10-16: the third ticket split, this
  being its second half.
- 2026-10-17, at the ticket's start: a player's default viewpoint is their
  first own party member by id, and every review finding is fixed and each
  task committed without asking unless it is a design question ("Ja, samma
  regel"); before the plan: the DM and the agent keep the world, and a DM who
  wants a character's view opens a spectator tab.
- 2026-10-17, at sign-off: D10's narrowing of the harness's denial proof
  approved under rule 2 ("Godkänn"); a reconnect whose `log` names a log it
  may watch opens on it ("Tjuven direkt"); G7 is not the end state, so a
  controlled creature that is no party member gets a log of its own, under a
  new kind of grant, closed when it goes and never reused, in its own ticket
  after this one; one ticket, nine tasks ("Ja, kör").
- 2026-10-17, on G11: a player is not to be told world sequences ("Exponera
  inte 'loggsiffran' känns som ett renare sätt att lösa det på"). That it is
  its own ticket after this one was stated to the owner, not asked.

**Last code commit:** `9d81ea5`, on `fa6a067`, `main` at the time. Every
code reference below is to its tree.

## The period, in commits

    git log --oneline fa6a067..9d81ea5

    9d81ea5 The register says what a seat's head and a spectator's nobody are
    86b7e9b An unperched spectator's silence is checked every run
    93604b6 The gates hold the seat's code
    ab11c2b The projection goldens and the pointers to them go
    ece9014 The harness checks that a character seat's stream is one log
    20ebd8c A player's move is judged by the character they watch
    1b664a4 A seat streams its character's log, and the projection goes
    562dff5 The harness proves a denial on its world seats only
    1891d42 The campaign wakes an append watch after each append
    db608e9 The contract carries ViewpointChanged
    c73ab61 A seat streams its character's log: the ticket, the plan and VTT-338..370

    git diff --stat fa6a067..9d81ea5

    127 files changed, 9751 insertions(+), 31463 deletions(-)

Most of the deletions are the projection: `scenarios/` alone loses 20,128
lines (the seven `scenarios/goldens/*/projections/` directories and the
README's section on them), and `internal/gateway` 10,374, among them
`project.go`, 14 test files and `testdata/history-fold/`.

## What the code did on `fa6a067`

A player's or a spectator's connection was sent `internal/gateway`'s
projection of the world log: `Projector` folded each world event through
`internal/perceive` for the connection's eyes (a player's every controlled
actor, a spectator's perch) and sent the frames it chose, numbered by world
sequence. The DM and the agent were sent the world. `set_viewpoint` was the
spectator's command alone, and a perch was applied by re-projecting. The move
gate judged a player's move through every actor they controlled
(`perceive.Eyes{Controller: p.ID}`). The per-character logs that
`internal/campaign` writes since the arc's second ticket, and the
subscription to one that 3a added, had no reader on the wire.

## Done looks like, answered

1. **[x]** `TestAPlayersConnectionIsItsCharactersLog` (VTT-338, VTT-368): a
   player who controls one party member opens watching it, at a head equal to
   the length of what `Campaign.CharacterLog` holds, is sent exactly that log
   (`requireLog`, `proto.Equal` and in order), then each later entry, then
   nothing within 300 ms. The test does not exist on `fa6a067`; Task 4's break
   1, the world sent to a log seat, reds it.
2. **[x]** `TestAPlayerWatchesTheirFirstPartyMemberByID` (VTT-340, VTT-341,
   VTT-385): a player controlling `a0` and `a1` opens on `a0`; a player with
   no party member and an unperched spectator open on nobody at head 0 and are
   sent nothing for a narration within 300 ms each; the player with none,
   granted `b1` while connected, is switched to it with no entry before the
   `ViewpointChanged` and sent its log. Task 4's break 2, the default taken as
   the last id, reds it. The spectator's 300 ms is `86b7e9b`'s (below).
3. **[x]** `TestAPlayerSwitchesToAnotherOfTheirOwn` (VTT-342, VTT-367):
   answered ok, then `ViewpointChanged` naming `a1` with no entry before it,
   then `a1` from its first entry and its next live entry, and nothing more.
   `TestAPlayerMayWatchOnlyTheirOwnPartyMembers` (VTT-343, VTT-346, VTT-347):
   another player's party member, a non-party actor the player controls and an
   absent actor are refused with one string once the id is masked, and an
   empty `set_viewpoint` is refused. `TestASpectatorWatchesThePartyMemberItPerchesOn`
   does the same through a perch, and leaves for nobody.
   `TestAuthorizeTableAllCommandsAllRoles` still refuses the DM and the agent,
   and `TestTheDMAndTheAgentWatchNoCharacterLog` holds it at `MayWatch`.
4. **[x]** `TestEverySeatCanReachTheCatchUpHeadItIsGiven` (VTT-384) runs for
   the DM, the agent, the player and the spectator, and each catch-up ends at
   the head it was told. `TestQASeatAPlayerOpensOnTheirFirstPartyMemberByID`
   (VTT-378) reads `CatchUpHead`, then `PresenceSnapshot`, then
   `ViewpointChanged` before the first entry; VTT-374's QA tests read head 0
   and a `ViewpointChanged` naming no actor for a seat that opens on nobody.
   The `server_test.go` tests that read the snapshot after the head on a
   player's or a spectator's socket, `TestPresenceAnnouncesAnArrival` and
   `TestARevokedSpectatorStopsSeeingTheTable` among them, pass in the gate's
   run.
5. **[x]** `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch` (VTT-340,
   VTT-348, VTT-386): for control revoked, made a non-party actor, and
   removed, the player is switched to nobody and no entry the loss caused is
   sent; its subtests switch a player to their next party member, and a
   spectator whose shoulder leaves the party to nobody. The edit the open debt
   entry in `docs/verification-debt.md` records (a switch announced on the old
   subscription) reds it.
6. **[x]** `TestACursorIsHonouredOnlyInTheLogItWasReadFrom` (VTT-339,
   VTT-349): a reconnect naming `a1` at cursor N opens on `a1` and is sent its
   entries after N; one naming no log opens on the default `a0` from its
   first entry; and after `a1` is revoked, one naming `a1` opens on `a0` from
   its first entry. `TestALogNamingAnActorItMayNotWatchOpensOnTheDefault`
   (VTT-340, VTT-369): a player naming another player's party member opens on
   their default from its first entry, and a spectator naming a non-party
   actor opens on nobody at head 0. Task 4's breaks 5a (the cursor counted in
   whatever log is named) and 5b (`log` ignored), each run on these two
   tests, went red.
7. **[x]** `TestAPlayersMoveIsJudgedByTheCharacterTheyWatch` (VTT-350): Ivo
   controls `ana` and `bo` in two rooms and watches `ana`; `bo`'s move onto a
   square only `bo` sees is refused with the unseen-square string, and after a
   switch to `bo` the same move is accepted. Task 5's break 1, the gate on the
   union of controlled actors, reds it.
8. **[x]** `grep -rn 'Projector\|pastResume\|subscribeFrom\|reperch\|perchSequence'
   internal/gateway cmd` prints nothing (it printed 82 lines across 21 files
   on `fa6a067`, the verifier's probe). `ls -d scenarios/goldens/*/projections`
   matches nothing; `internal/gateway/testdata/history-fold/` and
   `client/test/projection-parity.test.ts` do not exist. `internal/perceive`'s
   only change is one comment in `projection.go` (`MayPerch` to `MayWatch`),
   and its tests pass unchanged in the gate's run.
9. **[x]** `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection` (VTT-176)
   passes under its old name, beside QA's
   `TestQASeatTheDMAndTheAgentAreSentTheWorldAndIgnoreLog` (VTT-377). The
   DM's tests in `server_test.go` and `cmd/vtt`'s state dump, events tail and
   MCP end-to-end tests pass in the gate's run.
10. **[x]** The five tests pass in the gate's run on `9d81ea5`, as they did
    after each of Tasks 3 to 7. The break, run on `93604b6` after its gate: in
    `Server.handleCommand`, a refusal from `authorize` still appended the
    command's event (through `ToEvent`, with the move's scene backfilled so
    the append is valid) and returned the refusal unchanged.
    `TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario`,
    `TestThreeRoleExitScenarioOverLiveServeSubprocess`,
    `TestScenarioLibraryRunsSelfContained` (`denials.json`, `story-table.json`
    and `three-role-exit.json`) and
    `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts` went
    red, each with an "unexpected broadcast observed by … after a denied
    command" line naming world seats only: `agent, dm` for the three-role-exit
    runs and the soak, `dm` for `denials.json` and `story-table.json`, which
    have no agent. `TestEveryScenarioFileIsActuallyRun`
    stayed green; it holds that every scenario file is run, not their
    verdicts.
11. **[x]** `task check` on `9d81ea5`: green in 2,160 s, exit 0.
    - `tools/coverage-thresholds.txt`: campaign raised 94.0 to 94.7, harness
      85.0 to 89.5, gateway kept at 93.5; none lowered. The gate measured
      gateway 94.0, campaign 95.0 and harness 90.7.
    - `check:mutation`: 15 packages, zero unadjudicated survivors; the
      gateway, mutated afresh, at one survivor, its existing adjudication in
      `authz.go`, and every other package's verdict reused, since nothing in
      its dependency closure had changed since the run on `93604b6`.
    - `check:breaking`: pre-release, reporting, and it listed no objection to
      `ViewpointChanged`.

## What the rules became

The tests that hold each row are in `docs/requirements.md`.

- "Sent the entries of the one log it watches and no world event":
  **VTT-338**; **VTT-366**, that such a connection holds no world
  subscription, `**READING — Phase 4b**`; and **VTT-368**, a
  `ViewpointChanged` before any entry.
- "Names a log its participant may watch, opens on it": **VTT-339**; with
  **VTT-369** (a spectator's `log` naming no party member opens on nobody)
  and **VTT-377** (the DM and the agent ignore `log`).
- "The first party member by id, or nobody": **VTT-340**.
- "A connection that watches nobody is sent no event": **VTT-341**; and
  **VTT-374**, its catch-up head of 0.
- "A switch is announced by one `ViewpointChanged`, then the new log from its
  first entry": **VTT-342**; with **VTT-367** (no entry of the previous log
  after it) and **VTT-375** (naming what is shown sends nothing).
- "Who may watch whom": **VTT-343** (a player's own party members),
  **VTT-345** (a spectator's nobody), **VTT-346** (not a player's nobody),
  **VTT-385** (a spectator on nobody stays on nobody until it perches),
  **VTT-386** (a spectator whose shoulder leaves the party moves to nobody) and
  **VTT-344** (the player's other refused commands, succeeding VTT-141). The
  spectator's any party member is VTT-181 and the DM's and agent's world is
  VTT-140 and VTT-176, both refused by the sort as rules already held.
- "A refusal does not say whether the actor exists": **VTT-347**.
- "No entry after the connection may no longer watch": **VTT-348**, read
  causally (G5); with **VTT-352** (a revoked connection is closed at the next
  append) and **VTT-376** (with the policy-violation status).
- "Only entries after its cursor": **VTT-349**.
- "A move refused for a square the watched character cannot see":
  **VTT-350**; with **VTT-351** (the shown character, or the default at the
  command's snapshot), **VTT-370** (no party member moves nothing, G7's
  present shape) and **VTT-379** (the agent moves anywhere).

The plan's decisions put further rules on the system:
- the campaign's append watch (D1): **VTT-353** to **VTT-357** and
  **VTT-373**, with VTT-355, the wake's order, `**READING — Phase 4b**`;
- the fan-out to every active log, succeeding rows worded for "every player
  and spectator": **VTT-361** to **VTT-365**, and the entry's provenance,
  **VTT-371** and **VTT-372**;
- the opening frames: **VTT-378**;
- the harness (D10, D11): **VTT-358** to **VTT-360** and **VTT-380** to
  **VTT-383**.

VTT-366 is held by Task 4's review: the one `SubscribeWithNoProgressTimeout`
in `internal/gateway` sits behind `!watchesALog`, and a promotion can make no
DM or agent. VTT-355 is held by Task 2's review: `appendLocked` returns
early on six paths before the wake, then applies, wakes, and notifies the
world and the logs, and a character entry reaches a subscriber only by a
catch-up under `c.mu` or by `NotifyCharacter`. Its "after the live apply"
half is also held by the poisoning subtest of
`TestARefusedOrFailedAppendWakesNobody`, which Task 2's break 1 reds.

Withdrawn, each with its successor in the register, VTT-185's being none:
VTT-086, 141, 155, 177, 178, 179, 180, 183, 185, 186, 187, 190, 191, 194,
208, 216, 217, 225, 226, 234, 241, 246 and 314, twenty-three rows; nothing
is folded per seat, so VTT-185 has no successor. VTT-086 (the head is the
last sequence a seat is sent) and VTT-180 (a spectator is shown nothing
until it perches) were withdrawn by `9d81ea5`, after the specification
step's reviews found each false as worded: a seat that switches before its
catch-up is sent is never sent the entry its head names, and a spectator
whose `log` names a party member opens on it. VTT-384 succeeds 086, and
VTT-341, VTT-369 and VTT-385 succeed 180; VTT-386, a spectator whose
shoulder leaves the party moves to nobody, was a rule of item 5 that no row
held. VTT-189's test, `TestHoppingWhileTheTableIsBusyKeepsOneOrder`, was
rewritten, and G9 below is its measurement.

## The sort

Before Task 1: 29 candidates accepted, then 33 after the reading review of
the first commit split NEW-4 and added NEW-4c, NEW-25 and NEW-26, as
VTT-338..370. Refused, one line each, as the plan records them:
- the spectator's any party member or nobody: VTT-181 and VTT-345;
- the DM and the agent watching the world: VTT-140 and VTT-176;
- a log seat's catch-up head: VTT-086, since succeeded by VTT-384, and
  VTT-331;
- a burst of switches coalesced: VTT-188;
- `Actors()` returning a copy, and `perchBox` surviving: how, not what;
- the projection's deletion: a one-time change, held by item 8's grep and
  standing as VTT-366;
- D10's narrowing: a gate's scope, approved under rule 2 and recorded here.

During the work, from QA's "Requirements needing an ID" and the reviews:
- allocated: VTT-371 and VTT-372 (Task 2's QA), VTT-373 (Task 2's review),
  VTT-374 to VTT-378 (Task 4's QA), VTT-379 (Task 5's QA), VTT-380 to
  VTT-382 (Task 6's QA), VTT-383 (Task 6's QA defect) and VTT-384 to VTT-386
  (the specification step's reviews);
- refused, after Task 6: the redial mechanism (how); "a failure is `Pass`
  false with a `Detail`" (a representation); the first participant's fold
  and fresh catch-up (fixture constraints); "by event id and sequence"
  (inside VTT-360's "differs");
- not allocated, after Task 3: the harness's own proof rules; the harness
  has no specification and no rows (below).

## Phase 4a

Run on Tasks 2 to 6, each agent given the rows as allocated, the ticket's
rules, the specifications as they stood and the exported surface, and none of
the source, the tests, the diff or the plan. For Tasks 5 and 6 the author's
notes record the tree's hash before and after the QA agent, unchanged; for
Tasks 2 to 4 they record only the hash before. Each agent added one file, and
each file is committed with its task.

- **Task 2** (the campaign): `internal/campaign/qa_append_watch_test.go`, 15
  tests, 34 injections in its own file, all red; no behaviour defect.
- **Task 3** (the denial proofs): `internal/harness/qa_world_seat_test.go`,
  14 tests and 11 subtests, 24 injections (23 red, the last a robustness
  variant); no behaviour defect.
- **Task 4** (the seat): `internal/gateway/qa_seat_test.go`, 15 tests green
  under `-race`, 37 injections, all red; no behaviour defect.
- **Task 5** (the move gate): `internal/gateway/qa_move_gate_test.go`, 21
  tests, green on three `-race` runs and at `-race -count=20`; about 37
  injections, all red; no behaviour defect.
- **Task 6** (the harness's log check): `internal/harness/qa_log_seat_test.go`,
  23 test functions, 29 injections, all red once QA fixed one degenerate
  fixture (a cursor at the head made a replay empty); one behaviour defect,
  below.

Tasks 1, 7, 8 and 9 had no QA: a contract fixture; deletions and prose, plus
the soak's settle at every checkpoint; the gates' numbers, plus three tests
and the `pumpState.lookup` seam; and the gate. Nor did `86b7e9b`, one line of
a test, nor `9d81ea5`, a register correction and one white-box test. Task 7's
settle and Task 8's seam were each read by their task's reviewer in place of
QA; the settle is pinned by
`TestAPeriodicSoakCheckpointReadsWhatALateFanOutDelivers`, and the seam serves
`TestAnEntryTakenAsTheCredentialGoesIsNotSent`.

### QA adjudications

Task 2:
- VTT-354's poisoning and VTT-357 unreachable from the exported API: not a
  gap; the implementer's poisoning subtest reaches it from outside, and
  VTT-357 is held by the internal test.
- VTT-355's order unobservable: known, the row is a reading; QA's two
  VTT-355 tests retagged VTT-353, one renamed
  `TestQAWatchAWakeWaitsWhenEachRoundsFirstEntryArrives`.
- How many wakes per append, `stop`, and a watch asked after `Close`: spec
  ambiguities, settled by the code (one waiting signal per watch; `stop`
  removes it and closes nothing; refused with an error) and carried to the
  specification step.
- VTT-365's third case: none; a token always sees its own square.
- The `Campaign` type's doc listed the calls a poisoned campaign refuses and
  omitted four: rewritten.
- Provenance and world sequence in no row: allocated as VTT-371 and
  VTT-372.

Task 3:
- A witness step failing after a leak: intended, and it says so.
- `RunScenario`'s doc said a denied step asserts no participant observes any
  broadcast in a 300 ms window, false since the 2026-07-28 ordering proof
  (E6, `late_leak_test.go`): deferred to Task 6, which rewrote it.
- `SoakReport.Report` empty on a failing run inside the harness:
  `cmd/vtt/client_soak.go` fills it; no change.
- A below-witness envelope reaching a world seat after the witness: needs an
  out-of-order stream that per-connection delivery cannot produce.

Task 4:
- A refusal quotes the id that was sent: VTT-347 holds per id; the
  specification says the refusal names only that id.
- A cursor beyond the head is taken unclamped, as the world's is: no
  change; a consequence for SPEC-007 and SPEC-011.
- No switch to a new lower-id party member while watching another, and no
  switch for a spectator on nobody: by design (VTT-340; a spectator chooses).
- A batch of a move and a revocation drops the move's entries too:
  fail-closed by D3; for SPEC-015.
- A revoked connection stays open until the next append: VTT-352.
- No `ViewpointChanged` for the DM and the agent: by design, since a world
  seat watches no log, and no test asserts its absence. A perch on the shown
  actor sending nothing: VTT-375.
- QA's "Requirements needing an ID": allocated as VTT-374 (head 0 for
  nobody), VTT-375 (naming what is shown sends nothing), VTT-376 (the
  policy-violation status), VTT-377 (the DM and the agent ignore `log`) and
  VTT-378 (the opening frames); QA's citation lines naming the plan's
  decisions replaced by those ids.

Task 5:
- Four "at the command's snapshot" assertions cannot tell the fallback from
  a switch already applied: held by the white-box tests (breaks 2 and 3).
- The agent's freedom from the gate in no row: allocated as VTT-379.
- Sight range, a wall square's own visibility, a door's starting state,
  off-grid squares and the refusal texts: facts SPEC-013 and SPEC-016 leave
  to the code.
- SPEC-013's role table still calls `set_viewpoint` the spectator's only
  cell: for its rewrite.
- The DM's `open_door` and `close_door` on a square with no door tile are
  accepted: the fold writes `OpenDoors` for any square and sight reads it
  only on a door tile; no rule requires one; for the owner, at the branch's
  hand-over.

Task 6:
- **Behaviour defect:** a checkpoint's log-break line carried the previous
  checkpoint's number, since `rep.Checkpoints` was incremented after those
  lines. `runSoakCheckpoint` now counts first (VTT-383); the implementer's
  own test had pinned a "checkpoint 0" that no run has.
- A non-batch command answered with two world events, and one ended world
  seat beside a live one: spec ambiguities; the code stands.
- `RunScenario`'s doc tied VTT-358 to a denial a world seat saw:
  corrected; `SoakReport.Checkpoints` counted "accepted events" in its doc
  and accepted actions in the code: corrected.
- Gaps 1-9 (checkpoint naming, a switch inside a soak, the fake's hidden
  contracts and others): for a harness specification, which does not
  exist.

## The verification and Phase 4b

The verifier's probes, on `fa6a067`: `cmd/vtt/webdist` embeds
`commands.proto`'s descriptor (the `fileDesc` prefix appears once in
`assets/index.js`, G1); the TS suite without
`client/test/projection-parity.test.ts` passed 1,163 tests with `fold.ts`
still at 400 of 400 lines; four guards green,
`TestEverySeatCanReachTheCatchUpHeadItIsGiven`,
`TestAuthorizeTableAllCommandsAllRoles`,
`TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection` and
`TestARevokedSpectatorStopsSeeingTheTable`; item 8's grep at 82 lines.

One reviewer per commit, and a second, focused reading where noted:

- **The ticket and the plan:** 3 Must (VTT-349 unscoped for the DM; VTT-348
  missing "of a log"; Task 7's grep unable to reach two files), 10 Should, 7
  Nit, all fixed; the rows re-dispensed under the same ids, VTT-367..370
  added, and G11 found.
- **Task 1:** no Must or Should; three Nits, one carried to SPEC-007 (an
  empty `actor_id` means nobody).
- **Task 2:** no Must. The plan's break-1 prediction was wrong (Deviations); the
  wake's comment, VTT-371's wording, VTT-373 and the `errPoisoned` doc
  fixed; `TestEveryAppendWakesEveryWatcher` given a 2 s deadline.
- **Task 3:** 1 Must, that `worldSeat` counted participants while the skip
  is by name, so one name listed as DM and as player passed a real leak:
  fixed and pinned. Shoulds: a now-empty soak settle removed, a false doc
  line, `deliverLeak` replaced by a filter inside the fake's one delivery
  order.
- **Task 4:** 1 Must (`authz_test.go`'s prose named `MayPerch` and the
  spectator's command), 6 Should, 6 Nit. The Shoulds that moved code: the
  credential check now runs after `admit` takes a wake and after the settle
  loop; the pump re-resolves the participant, so a promotion is judged with
  the current role (`TestAPromotedSpectatorWatchesTheirFirstPartyMember`).
  The pump then reached a cyclomatic complexity of 33 and became a loop over
  `pumpState` methods; a second focused reading found one Should, fixed.
- **Task 5:** 0 Must, 3 Should, 5 Nit. The told viewpoint stored before its
  frame was held only by a race; `TestAMoveOnAConnectionOpenedOnALogIsJudgedByThatLog`
  pins it. Five evidence entries that stayed green under their row's break
  were dropped: four on VTT-343 and one on VTT-339.
- **Task 6:** 2 Must. `runReconnectStep`'s doc claimed a log seat's current
  log is what a redial opens on, false after any switch the default does not
  explain: now a warning, QA's spectator case removed, and the open debt
  entry written. The final soak checkpoint checked the log seats before their
  last entries arrived: a settle added, and
  `TestTheLastSoakCheckpointReadsWhatALateFanOutDelivers`. 6 Should, 3 Nit.
- **Task 7:** 2 Must (a comment saying a grant "declares the kind", which
  SPEC-013 contradicts; a citation that did not hold the claim beside it), 7
  Should, 6 Nit. The commit hook then found the escaped defect below; its fix
  had its own focused reading: 0 Must, 2 Should, 1 Nit.
- **Task 8:** 0 Must, 1 Should (the racing-switch test switched onto the log
  already shown and asserted only the seat; it now switches from `c1` to
  `a1` with `a1`'s revocation waiting), 1 Nit.
- **`86b7e9b`:** the edit this report's reading review proposed (its S1),
  recorded as that review; no separate reading.
- **`9d81ea5`:** the SPEC-015/016 reviewer read the staged change: no Must;
  two Shoulds (VTT-384's wording and the uncovered settle case; `seat.go`'s
  stale pointer) and two Nits, applied.

## The verifier's gaps, G1-G11

- **G1:** `cmd/vtt/webdist` rebuilt and committed in Task 1.
- **G2:** the register rows the ticket's classes missed were sorted by the
  plan's table; twenty-one withdrawn.
- **G3:** SPEC-012, SPEC-017 and SPEC-019 are amended at the specification
  step after this report.
- **G4:** `watch_test.go` reads frames by kind: a result by request id
  (`watchConn.command`), and `untilViewpoint` returns the entries before the
  `ViewpointChanged`.
- **G5:** VTT-348 reads "caused by the event that took away … or by a later
  event", held by `TestNoEntryCausedByTheLossIsSent` and Task 4's break 4.
- **G6:** each file was touched where the plan said.
- **G7:** the owner's ruling; until its ticket, VTT-370.
- **G8:** stands until ticket 4.
- **G9:** measured, below.
- **G10:** open; ticket 4's client and the harness's `state dump`.
- **G11:** the owner's ruling; its own ticket.

## The deliberate breaks

Every break was applied, asserted to have landed, and restored by copy with a
byte comparison.

- **Task 1:** a fixture's key misspelt reds the Go round trip and the TS
  fixture test; an explicit empty `actorId` reds the Go round trip.
- **Task 2:** fourteen breaks, all red, among them the wake moved before the
  live apply (the poisoning subtest), a blocking wake (a 2 s timeout), and
  five of `perceive`'s clauses for every active log (the table test).
- **Task 3:** `leakedSince` reading no seat (five tests) and `leakedBelow`
  reading no seat (four soak tests); the no-world-seat branch never taken;
  `leakedSince` reading character seats again (red at once) and
  `leakedBelow` doing the same (red only after QA); and `worldSeat`'s
  comparison loosened.
- **Task 4:** every break red but one. Break 4, `admit` without its
  priority receive, reds `TestNoEntryCausedByTheLossIsSent` and leaves the
  wire test green, as the plan predicted. Break 6, Task 3's skip reverted,
  reds the seed-1 soak with "unexpected broadcast observed by player1" three
  times and player2 twice, which is D10's reason. Break 7b, no credential
  check before an entry, stayed green: the wake path catches the same
  revocation unless the pump takes the entry first. Task 8 killed it.
- **Task 5:** the gate on the union, no fallback, a fallback that keeps a
  lost viewpoint, `told` never stored, and D6's original order: each red.
- **Task 6:** every break of its checks and rows went red, two of them only
  once the fixture they exposed was added (a log opening at 4, a replay's
  entry at sequence 2). The plan's break 3, `ViewpointChanged` sent without
  the subscription reset, stayed green in every `cmd/vtt` scenario test (the
  open debt entry), and reds the gateway's own test.
- **Task 8:** both survivors applied by hand red their new tests. The plan's
  break: with `TestNoEntryCausedByTheLossIsSent` deleted, a gateway-only
  gremlins pass killed 168, let 1 live (the adjudicated `authz.go`
  boundary) and left 6 not covered, none in `seat.go`. So no mutant survives
  without that test, and no gremlins operator removes `admit`'s priority
  select: what holds the priority receive is that test and break 4, not the
  mutation gate.
- **`86b7e9b`:** `openSeat` subscribing an unperched spectator to the world
  from 0: the old line red in 11 of 20 runs, the new in 20 of 20.
- **`9d81ea5`:** the log seat's and the world seat's head + 1 red the four
  VTT-384 tests; `watch` keeping its first subscription's head reds only the
  new test; `resolve` giving a spectator on nobody the first party member at
  a wake reds the three VTT-385 tests, and
  `TestASpectatorHopsFromOneShoulderToAnother`, green, drops its VTT-180
  citation; `resolve` keeping a shoulder that left the party reds both
  VTT-386 tests.
- **Item 10:** above.

## Rule 9: how MapTool does this

`net.rptools.maptool.client.ui.zone.ZoneViewModel.makePlayerView` builds a
player's view on the client from the tokens the player selected; with none
selected it falls back to every owned token with sight when views are
individual or the server is personal, otherwise to every player token with
sight, and with none of those to the role alone;
`ServerMessageHandler.handleMessage` relays moves with no sight check
(`~/dev/RPTool/maptool` at `f4b7fef6c`). Borrowed: the view follows one
chosen character at a time. Refused: both union fallbacks (the owner struck
the union), the role-only fallback (a seat watching nobody is sent nothing),
selection as client-only state (here the stream is the boundary, so a switch
is a command and an answer), the absent move gate, and the distribution model.

## Figures

- `task check` on `93604b6`: 6,042 s, exit 0; 21.9 GiB free when
  `check:mutation` began and 15.9 GiB after. The run on `86b7e9b` was stopped
  in its mutation stage once `9d81ea5` was to follow. On `9d81ea5`: 2,160 s,
  exit 0, 19.8 GiB free when `check:mutation` began and 18.2 GiB after; only
  the gateway was mutated afresh.
- `check:mutation`, standalone on `ab11c2b` with Task 8's floors
  uncommitted: 55 minutes, two gateway survivors (Task 8). The gateway-only
  gremlins pass with the test deleted: 20 min 27 s.
- `check:ts-mutation` in the run on `93604b6`: 2,944 mutants, 2,868 killed,
  9 timed out, 67 adjudicated; the run on `9d81ea5` verified that report,
  since no client input had changed.
- D2, measured in Task 2 on the adventure-night golden (12 events, 3 actors):
  `State()` 51.4 µs, 153 KB and 88 allocations; `Actors()` 3.1 µs, 3.0 KB
  and 73 allocations.
- G9, on `93604b6` at load 2.5 to 3.3, through a probe test since deleted: the
  seed-1, 500-event soak leaves two character logs, 75 and 87 entries. On a
  copy of its campaign, `Campaign.SubscribeCharacter` on the 87-entry log
  from 0, which holds `c.mu` across the history read an append waits on, took
  a median 0.39 ms over 200 runs (p95 0.61 ms, max 0.92 ms); `State()` on
  the same state took 0.66 ms. On the wire, a spectator's switch from the
  75-entry log to the 87-entry log, from sending `set_viewpoint` to receiving
  entry 87, took a median 16.8 ms over 50 runs (p95 23.9 ms, max 40.2 ms),
  of which the command's result alone was about 5 ms; the owning player's
  fresh connection to entry 87 took a median 4.7 ms. So an append waits a
  median of about 0.4 ms, and in three runs of 200 at most 1.5 ms, for a
  switch on the soak's longest log. VTT-189 ("Perching stalls no other
  participant's command") is kept under the plan's reading, that "stall"
  means waiting on the perching seat's pump; read literally, a switch holds
  `c.mu` for that long, and another participant's append can wait on it.
  The probe's figures are the author's record; its source survives only in
  the author's scratchpad.
- `cmd/vtt`'s scenario and soak tests: 113 s on Task 3's tree, 164 s on Task
  5's, 162 to 173 s on Task 6's; the soak gained about 1.8 s from settling at
  every checkpoint.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D6: the move gate reads a viewpoint the pump writes only after it sends `ViewpointChanged` | `seat.watch` stores `told` where the seat moves, before the frame naming it is queued | Stored after the send, a client that read the frame and moved at once could be judged by the character it had left, and a connection opened on `log` was judged by the default until the pump's first frame; what is stored has passed `MayWatch` and is checked again at the command's snapshot |
| Task 5: the grant case in `watch_test.go`, the revocation case inside item 7's test | Both white-box tests in `seat_internal_test.go` with no pump running | End to end, the window each tests is open only when the scheduler leaves it open |
| Task 5: "the three move-gate tests (VTT-156, VTT-158) rewritten for one character" | Six existing move tests open the player on `act-fighter` with `log` | The fixtures grant `act-fighter` to a player who also controls `a1`, the default by id: three went red and two passed vacuously; the sixth's result before the change is not recorded |
| D2: `Actors()` against `State()` on the soak's campaign at 500 events | On the adventure-night golden, 12 events and 3 actors | The soak's generator is in `internal/harness`, which campaign tests cannot import; G9 later timed `State()` alone on the soak's final state (0.66 ms) |
| Task 2: the table test in `append_watch_test.go`; the `Campaign` doc untouched | Its own file, `every_log_told_test.go`; the doc rewritten | It is not about the watch; the doc's list of refused calls was stale |
| Task 2's break 1 predicted to red no deterministic test | The poisoning subtest reds | A poisoning append is the one that persists and then fails its apply |
| The plan's id table, VTT-338..370 | VTT-338..383 | Thirteen rules from QA's lists and the reviews (the sort, above) |
| Task 3: the reconnect test reconnects the DM | The agent | The DM's later command needs its live connection |
| Task 3's break 3 (`leakedBelow`'s skip reverted) predicted green until Task 4 | Red after QA, through `TestQAWSSoakDoesNotReadCharacterSeats` | QA's test reads the soak's narrowing directly |
| Task 4: every re-evidenced cell keeps a surviving entry | VTT-319 had none; its oracle ported to `internal/campaign/character_log_oracle_test.go` | Its only entry was the deleted gateway test |
| Task 4: the pump as planned | A loop over `pumpState` methods, re-resolving the participant on each wake | The review's fixes took it to a cyclomatic complexity of 33, over the lint's 30; VTT-031's live promotion |
| Task 4's file list names no presence test | `TestASecondDeviceIsNotASecondArrivalOrDeparture` waits for the watcher's snapshot | A player on nobody now joins presence before the DM's world subscribe, and the test assumed a completed dial had joined |
| Task 4: `TestAClosedWakeClosesTheConnection`, white-box in `seat_internal_test.go` (D13, break 8) | `TestAClosedCampaignClosesAWatchersConnection`, black-box in `watch_test.go`, and break 8 ran on it | A closed wake has one producer, `Campaign.Close`, and a seat watching nobody sees it through its wake alone, which that test drives end to end; why the white-box test was not written is not recorded |
| Task 6: the contiguity check in `observeOnAll`'s and `observeBatchOnAll`'s drain | In the extracted `runStep`, over everything each step drained | `RunScenario` reached a cyclomatic complexity of 31 |
| Task 6: the soak checks each log seat at each checkpoint | It settles them first, at every checkpoint, in Task 7's commit | The escaped defect below |
| "No change to `internal/perceive`, `internal/store`, `engine` or `sight`" | Comment edits in `internal/perceive/projection.go` (`MayPerch` to `MayWatch`) and in `internal/engine`'s `apply.go`, `state.go` and `actorkind.go` | Task 7's sweep: each comment named a deleted file or function; no code changed |
| Task 7: every touched comment block ends at six lines or fewer | `tools/check-no-create-scene.py`'s docstring, about 163 lines, kept | `tools/` is outside `check:comments`' roots, and the edit changed one clause |
| "`.go-arch-lint.yml`'s gateway list is not tightened" | The gateway's `eventgen` edge removed | No gateway file imports `eventgen` once its property test left, and the comment beside it named that test |
| Task 8: survivors killed with tests | Also a seam: `pumpState.lookup`, set to `s.ids.Lookup` | `onEntry`'s credential check could be reached only by a lookup that answers valid then gone |
| Task 1: one Go round-trip test | Two | `roundTrip` takes one fixture per test |
| The last code commit, `93604b6` | `86b7e9b`, one line of `TestAPlayerWatchesTheirFirstPartyMemberByID` | This report's reading review found the unperched spectator's silence checked with `quiet(0)`, which caught a waiting frame about half the time; with a spectator fed the world on purpose, the old line went red in 11 of 20 runs and the new in 20 of 20 |
| VTT-086 and VTT-180 kept; no row for item 5's spectator | Both withdrawn and VTT-384 to VTT-386 allocated in `9d81ea5`, the tests that hold them citing the new ids | The specification step's reviews found 086 and 180 false as worded and the spectator rule held by no row; each new row's tests went red under a break of its rule |

## What could not be established

- **Why a switch is slower than an opening.** G9's switch took a median
  16.8 ms against an opening's 4.7 ms for the same 87 entries, and the
  `set_viewpoint` result alone about 5 ms, where `State()` takes 0.66 ms.
  Not traced.
- **VTT-355's second half** (before any entry is delivered) and **VTT-366**
  have no test; each is held by its task's review.
- **`admit`'s priority receive** is held by one test and a deliberate break;
  no mutant expresses its removal.
- **The harness cannot see a switch on the old subscription**, and its
  reconnect check fails a correct server after a non-default switch: the
  open debt entry in `docs/verification-debt.md`, for ticket 4.
- **No specification governs `internal/harness`.** QA was given the task's
  text as its whole statement of intent, and its gaps for the harness have
  nowhere to go.
- **A stray envelope that is the last thing a step drains** fails that step
  and the next, since its successor does not continue it: VTT-359 read
  literally. A scenario's last step is checked only as far as its quiet
  drain.
- **Found while writing this report:** the open debt entry "The oracle
  corpus contains no non-party viewpoint" names `MayPerch`, which Task 4
  deleted, and `TestARefusedLookStillFillsTheRoster`, which
  `docs/reports/2026-09-29-the-seat-and-the-perch-have-a-record.md` already
  recorded as missing; its corpus, the keystone's projected goldens, went
  with this ticket. For the owner, at the branch's hand-over.
- **Found by the specification step's review:** a promotion appends nothing
  and wakes no seat, so a promoted participant's seat keeps what it shows
  until the next append's wake re-judges it. The entries its subscription
  already holds are still sent, though no entry written after the promotion
  reaches it first: a spectator promoted to a player can go on being sent
  the queued rest of a character's log they may no longer watch, which
  VTT-340's "whenever" does not allow for. Recorded in
  `docs/verification-debt.md`'s open debt with its recipe; which side moves,
  the row or the code, is the owner's, at the branch's hand-over.
- **The opening `ViewpointChanged` does not say** whether the log follows
  after the cursor or from its first entry, which `watch`'s settle loop can
  choose when a wake arrives while the seat opens; only the first entry's
  sequence says. Ticket 4's client must read it so.
- **Found while writing the specifications:** SPEC-018's bound table says of
  `TokenHidden.token_id` and `SceneSeen.*` "none reaches the log", written
  before character logs stored them; it predates this ticket, for the owner.

## What was deliberately left out, and where it went

- The client resetting on `ViewpointChanged`, redialling with `log`, and its
  pointers to deleted files and names (`client/src/fold.ts`,
  `client/src/app.ts`, `client/src/view/spectator.ts`, `client/src/wire.ts`,
  `client/src/view/dm.ts` and seven tests): ticket 4. Until then a player's
  or a spectator's browser is broken (G8).
- The harness folding each character log, its `Dialer` passing `log`, and a
  `state dump` that stops at `ViewpointChanged` (G10): ticket 4.
- A log for a controlled creature that is no party member: its own ticket,
  ruled by the owner and not yet written, which will withdraw VTT-370.
- A player's `CommandResult.sequence` (G11): its own ticket, ruled by the
  owner and not yet written.
- Re-measuring SPEC-021's fan-out cost: named by "The cost after the third
  ticket" in
  `docs/superpowers/specs/2026-10-16-each-party-member-keeps-its-own-log-design.md`;
  no ticket carries it yet, for the owner.
- `open_door` and `close_door` on a non-door square: for the owner, at the
  branch's hand-over.
- `check:coverage` names eleven packages whose floors could rise. Harness is
  one of them, set a point under on purpose by Task 8; the other ten are not
  this ticket's.
- The dated phrase "the visibility projection landed", eight times in
  gateway test comments, names `perceive`'s projection, which remains: the
  standing sweep's.
- `Subscribe`'s early unlock and 3a's D6 hold: unchanged, as planned.
- SPEC-015, SPEC-016, SPEC-011, SPEC-013, SPEC-007, SPEC-009, SPEC-021,
  SPEC-012, SPEC-017 and SPEC-019: the specification step after this report.

### The escaped defect

Task 6's `TestASoakCheckpointFailsACharacterStreamThatSkips` failed in Task
7's commit hook under load: `runSoakCheckpoint` read the log seats'
histories without settling them, so which checkpoint reported a break
depended on scheduling. On `ece9014`'s `soak.go` it failed 64 of 120 runs at
`-cpu 1,2` beside other packages' tests; every run before the commit had
been at the default `GOMAXPROCS`. Fixed in `ab11c2b` with
`TestAPeriodicSoakCheckpointReadsWhatALateFanOutDelivers`; the recipe is
`docs/verification-debt.md`'s 2026-10-17 entry.

## History the code no longer carries

Removed from comments under rule 10 in this period, and recorded here:

- `campaign.FoldPrefix`'s doc said it exists so the gateway grows no second
  fold; that it has run one pass since retraction left on 2026-08-31; that a
  caller folding every prefix does O(n²) work and `seat.receive` was that
  caller, with line of sight at 15-176 ms per eye the larger term; and that
  the per-character-logs design would delete its only non-test caller.
- `engine.IsPartyMember`'s doc said the predicate had been transcribed five
  times, twice into the keystone's oracle, which is why the keystone agreed
  with the projection while both were wrong; that `len(GetControllerIds()) >
  0` had published a monster's stat block the moment a DM took control; and
  that a migration arm treating an absent kind with a controller as a party
  member was deleted on 2026-08-24, on the owner's ruling that no such logs
  exist. A floating block said the function moved from `internal/gateway` on
  2026-09-16 and that the keystone's `oracleIsPartyMember` must not be
  merged into it.
- `apply.go`'s actor-creation arm said it used to seed the control set from a
  declared `controller_id`, which made the archer leak reachable through a
  second door, and that it used to strip empty ids.
- `engine.State`'s `Visible` doc said the field was built from the tile keys
  until 2026-08-22, hiding a player's own token on a scene with no tiles;
  its `Explored` and `Visible` docs recorded the 2026-08-22 runs in which
  dropping the tag or emitting an empty map failed all eight goldens.
- `.go-arch-lint.yml`'s `eventgen` note said both campaign's rebuild
  property and gateway's per-seat fold property drove the generator, and
  that its earlier wording, "only campaign drives it today", was true when
  written on 2026-09-12 and false one commit later.
- `eventgen`'s package doc said the model lived inside the campaign's
  property test until 2026-09-12.
- `TestAnEncodeFailureTearsTheConnectionRatherThanTheBatch`'s doc said the
  pump used to skip a frame it could not encode, harmless while one event was
  one frame and not once a seat received several per event.
- `testStateDumpForASeatWatchingNobody`'s doc said `vtt state dump` once
  waited out its 30 s deadline for any player or spectator token, because the
  head it was told was the world's.
- `TestEveryMapTheCorpusNamesIsInstalledAndUsed`'s doc recorded a 2026-09-02
  measurement: with the map id in `denials.json` misspelt,
  `TestScenarioLibraryRunsSelfContained/denials.json` stayed green and only
  that gate went red.
- `RunScenario`'s doc said a denied step asserts that no participant
  observes any broadcast within 300 ms, false since the 2026-07-28 ordering
  proof (E6).
