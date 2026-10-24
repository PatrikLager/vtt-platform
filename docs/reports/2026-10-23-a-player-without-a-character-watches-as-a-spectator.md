# A player who controls no party member watches as a spectator does: the change

**Ticket:** `docs/superpowers/specs/2026-10-23-a-player-without-a-character-watches-as-a-spectator-design.md`.
An agent that did not write it verified it. The first verdict returned it on
four findings (F1-F4); the writer revised it, and the second verdict passed it
with one gap (G1).
**Plan:** `docs/superpowers/plans/2026-10-23-a-player-without-a-character-watches-as-a-spectator.md`,
written by that verifier and amended by the reading review of `59da686`, the
commit that carried both.
**Where it came from:** ruling B-4 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the fourth of the
nine tickets in the order the owner agreed.
**The owner's rulings:**
- B-4, 2026-10-22: "A player whose character the DM has taken watches as a
  spectator does." The role stays player.
- 2026-10-23, while the ticket was written: "Stay on Tobbe" (a player watching
  their only party member when the DM revokes it keeps watching it), and "One
  ticket, server first".
- 2026-10-23, after the first verdict: Q1, "Field on the list" (the browser
  learns that a player controls no party member from an additive field on
  `WatchableChanged`).
- 2026-10-23: the plan signed off, "Ja, kör".

**Last code commit:** `0aacc59`, on `f2dd819`, `main` at the time. Every code
reference below is to its tree. The plan made Task 3's `3c7c0db` the last; the
gate on it found one surviving mutant, and `0aacc59` removed the code it sat
on (Deviations).

## The period, in commits

| Commit | What it holds |
|---|---|
| `59da686` | The ticket, the plan, VTT-450 to VTT-461, and the withdrawal of VTT-343, 346, 351, 402 and 406 |
| `fe6ed1f` | Task 1: `MayWatch` for a player who controls no party member, `judgedBy`, N1-N6 and the stay test, the 23 test edits, QA's `internal/gateway/qa_player_perch_test.go`, the register's cells, the ledger |
| `afd206a` | Task 2: `WatchableChanged.controls_no_party_member` and its comments, `watchable` setting it, `set_viewpoint`'s description, the regenerated contract and tools, the hand-kept `expected_tools.json`, N7, N8, the round trip, QA's `internal/gateway/qa_controls_none_test.go`, the bundle |
| `3c7c0db` | Task 3: the client draws a perched player the perch, the re-pointing of VTT-402 and VTT-406, the comments the change made false, C1-C3, the doors test, the e2e, QA's `client/test/qa-perched-player.test.ts`, the bundle |
| `0aacc59` | `canAct` in `paint` loses its role disjunction, a test comment it made false, the bundle |

## What the code did on `f2dd819`

`MayWatch` in `internal/gateway/viewpoint.go` accepted, for a player, only a
party member whose controllers included them, and refused the empty id. So
`watchable` named no one for a player who controlled none, `defaultViewpoint`
gave them nobody, and a `set_viewpoint` naming any party member, or nobody,
was refused. `seat.judgedBy` answered the told viewpoint while `MayWatch`
accepted it, which for a player was only ever one of their own. In the
browser, `paint` in `startSession` (`client/src/app.ts`) drew the perch only
for the role `spectator` (`mayPerch`); a player who controlled nothing was
drawn the player panel reading "You do not control an actor yet." A revocation
of a player's last party member moved their connection to nobody.

## Done looks like, answered

1. [x] `TestAPlayerWhoControlsNoPartyMemberMayWatchAnyPartyMemberOrNobody` in
   `internal/gateway/viewpoint_test.go` (N1) has two players, one controlling
   nothing and one controlling only a `NON_PARTY` actor. It accepts every party
   member and the empty id for both, and refuses a non-party or absent actor
   with the spectator's string. `TestWatchableNamesEveryPartyMemberForAPlayerWhoControlsNone`
   in `internal/gateway/seat_internal_test.go` (N2) reads the set. Both were
   red on the old code: "may not watch", and an empty set.
2. [x] `TestAPlayerWhoControlsNoPartyMemberPerchesAndReopensOnIt` in
   `internal/gateway/watchable_test.go` (N3) opens such a player on every party
   member, perches, is answered ok, and is sent the `ViewpointChanged` and the
   log from its first entry; a dial naming that log opens on it. It was red at
   the opening set.
3. [x] `TestAPlayerMayWatchOnlyThePartyMembersTheyControl`,
   `TestAPlayerMayNotWatchNobody` and
   `TestAPerchRefusalDoesNotSayWhetherTheActorExists` are green, unedited
   beyond their citation lines.
4. [x] `TestQAMGAPerchedPlayerMovesNoToken` in
   `internal/gateway/qa_move_gate_test.go` (N4) perches a player who controls
   only a pet on another player's `pc-c`, and every pet move is refused as
   unseen, the crate square included. `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken`
   now opens `log=pc-c` on `pc-c` and keeps every refusal.
   `TestQAMGLosingTheOnlyPartyMemberLeavesNoTokenMovable` now asserts the player
   stays on `pc-b` after the revocation, by a narration of `pc-b`'s log with no
   switch before it, and every pet move is still refused.
5. [x] `TestAPerchedPlayerGrantedAPartyMemberMovesToTheFirst` in
   `internal/gateway/watchable_test.go` (N5): a player perched on an
   uncontrolled party member is granted two of their own. The set names only
   theirs, then a `ViewpointChanged` names the lower id, then its log follows
   from the first entry.
6. [x] `TestAPlayerWhoseLastPartyMemberIsRevokedKeepsWatchingIt` in
   `internal/gateway/watch_test.go` (N6) uses a table of two party members.
   After the revocation the set names both and says the player controls none.
   The revocation's own entry and a narration of `a1`'s log follow, with no
   switch. `TestARevocationOfATablesOnlyPartyMemberSendsTheSetAgain` (N8) holds
   the one-member table, where only the flag changes.
7. [x] `TestTheSetSaysWhetherTheParticipantControlsNoPartyMember` (N7) reads
   the flag at the opening for five participants:
   - a player who controls a party member, false;
   - one who holds it after another player, false;
   - one who controls only a `NON_PARTY` actor, true;
   - one who controls nothing, true;
   - a spectator, true.

   N8 holds the resend when only the flag changes. Two contract tests hold
   the wire name: `TestServerFrameWatchableChangedControlsNoneRoundTrip` in
   `contract/roundtrip_test.go`, and the case for
   `server_frame_watchable_changed_controls_none.json` in
   `contract/events.test.ts`. Both fail to decode on `f2dd819`'s schema.
8. [x] The 23 tests encoding the old rule are edited and green (plan D9):
   - six now tell a player who controls none every party member;
   - eleven promotions: ten give the promoted participant a party member of
     their own and expect the move, and the eleventh, a subtest of
     `TestQAWatchableAPromotionChangesTheSet`, keeps its set and its perch;
   - four revocations: `TestNoEntryCausedByTheLossIsSent` and the "control
     revoked" case of `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch`
     gain a second party member, so VTT-348 is still tested, and two now
     assert the stay;
   - two more: `TestQAMGAPlayerControllingOnlyANonPartyActorMovesNoToken`
     now accepts `pc-c` for a player who controls only a pet, and
     `TestQACreatureSetViewpointNeedsAPartyMemberTheyControl` first grants
     the second player a party member, so "not theirs" is still refused.

   Three were renamed, their names having become false, and their register
   cells moved with them.
9. [x] In `client/test/app.test.ts`:
   - "a player whose set says they control no party member is offered the
     perch, and no player panel" (C1) asserts "Whose eyes", every party member
     and "No shoulder", `.player` count 0, and the `set_viewpoint` that
     Lera's button and "No shoulder" each send;
   - "a perched player's board click sends nothing, on a listed party
     member's token or on an empty square" (C3) holds the board.

   C1 to C3 were red on the old client. C3 went red at its precondition,
   the perch; its click half went red under Kc3 (The deliberate breaks).
10. [x] "a player is offered exactly the party members the set names" and "a
    player switches by a button and by a click on the token" are green. "the
    set's flag moves a player between the perch and the player panel" (C2)
    holds the way back.
11. [x] In `client/e2e/handover.spec.ts`:
    - "the DM takes a player's only character back, and the player chooses
      another shoulder" (W1) revokes both of Lera's characters with the
      console's Revoke button. Lera stays on Ash with the perch, goes to
      nobody (an empty Story, no token), chooses Bram and is shown his log, and
      a reload keeps Bram.
    - Beat 2 of "a DM hands a second character over, and the player keeps
      both across a reconnect" now asserts the perch on nobody, offering Ash
      and Bram, where it read "You do not control an actor yet."

    On `afd206a`'s bundle beat 2 was red at `.perch`. W1 was red at its
    precondition, because the first test had stopped before its grants. The
    reviewer carried the first test past its grants on that bundle, and W1
    went red at `expect(perch).toBeVisible`. `task e2e` on `0aacc59`: 20 passed
    and 1 skipped, the existing `test.skip`; both tests above passed.
12. [x] `grep -n 'may watch only a party member they control\|only a spectator
    may ask' tools/toolgen/main.go contract/vtt/v1/commands.proto` prints
    nothing. `TestToolsMatchGolden` is green with the hand-kept
    `contract/testdata/expected_tools.json`, in `tools/toolgen/main_test.go`.
    `check:drift` passed in the gate on `0aacc59`.

`task check`, whole, on `0aacc59`: green, `TASK_EXIT=0`, in 3931 s. The first
run, on `3c7c0db`, failed at `check:ts-mutation`, its last step, and nowhere
else (Deviations). `task e2e` on `0aacc59`: 20 passed, 1 skipped, in 25.9 s.

## What the rules became

- **"A player who controls no party member may watch any party member, or
  nobody"** became VTT-452, with the stay as its own row, VTT-456: "An event
  that leaves a player controlling no party member does not move a connection
  of theirs off the party member it watches, when that one is still a party
  member after the event."
- **"A player who controls a party member may watch only the party members
  they control, and not nobody"** became VTT-450 and VTT-451.
- **"A player's move is judged only by the eyes of a party member they
  control"** became VTT-453. VTT-370, "A player who controls no party member
  may move no token", is unchanged and gains N4.
- **"A player's or a spectator's connection is told whether its participant
  controls no party member, with whom it may watch"** became VTT-454, and
  VTT-455, the resend when only that changes.
- **"The browser offers a player who controls no party member the perch a
  spectator is offered, and no player panel"** became:
  - VTT-457, the panel's buttons for a player who controls one;
  - VTT-458, the perch;
  - VTT-459, "No shoulder";
  - VTT-460, the board click that switches;
  - VTT-461, the perched board click that sends nothing.
- **Withdrawn:** VTT-343, VTT-346, VTT-351, VTT-402 and VTT-406, each naming
  its successors. No code or test cites them.
- **Reworded before their first commit,** while nothing cited them:
  - VTT-456 first claimed a lasting stay, which a perch or a later grant ends;
  - VTT-457, 458, 460 and 461 lacked "the role the client last read" and the
    set the client was sent, and VTT-457 and 458 the fold exception;
  - VTT-453 placed the command's snapshot on one clause only.

The sort refused:
- "`watchable` names every party member for a player who controls none", which
  VTT-399 and VTT-400 already state through `MayWatch`;
- the move to the first party member on a grant, which is VTT-340;
- the perch and reload, which VTT-339, 342, 399 and 442 hold;
- the one refusal string, which is VTT-347;
- the tool's wording, which `TestToolsMatchGolden` and a grep hold.

VTT-405 is not withdrawn: its party-member half covers any switch button, and
VTT-459 covers the player's "No shoulder".

After Phase 4a the sort refused every sentence QA cited without an id.
- From Task 1: the set's sort order, and "a set that has not changed is not
  sent again". Both are existing behaviour held by tests that cite VTT-399 and
  VTT-400.
- From Task 2: the field's polarity, which its name states; SPEC-007's
  ordering sentences; and "a grant sets the kind", which is VTT-391 and
  VTT-392.
- From Task 3: the perch's texts and its "Whose eyes" heading, which are
  definitions; and "Bara knappar", which the existing spectator tests hold.

One behaviour has no row: a window that cannot act is shown no armed board
(Deviations). The test "a player who perches while doors are armed is shown no
armed board" holds it, and SPEC-022 is to say it.

## Phase 4a

QA ran once per code task. Each agent was given:
- the rows and the owner's rulings;
- the governing specification whole: SPEC-015 for Tasks 1 and 2, with SPEC-007
  for Task 2, and SPEC-022 for Task 3;
- the surface: `go doc -all` of `internal/gateway`, `internal/campaign` and
  `internal/identity`, or for the client, a written page surface.

Each was withheld the source, the tests, the plan and the diff. `0aacc59` had
no QA: it deletes a term and changes nothing over the roles the server sends,
and Task 3's QA file stayed green on it.

| Task | File | Tests | Injections | Failures |
|---|---|---|---|---|
| 1 | `internal/gateway/qa_player_perch_test.go` | 17 | about 37, all red | none |
| 2 | `internal/gateway/qa_controls_none_test.go` | 19 | 24, all red | none |
| 3 | `client/test/qa-perched-player.test.ts` | 17 | about 33, all red but one | none |

The one injection that stayed green is inherent: a late `/api/me` answering
`spectator` draws the same perch, since the rules give a perched player and a
spectator one window. Task 3's file failed `task client:typecheck` as
delivered: a possibly undefined socket (two errors), two `NodeList` spreads
and four possibly undefined reads. `!` and `Array.from` fixed it, and no
assertion changed.

### QA adjudications

- Task 1, G1 and G2: a batch, or one wake, holding two revocations. No
  change: SPEC-015 judges appends once per wake, the connection is never
  moved, and VTT-456 holds as worded.
- Task 1, G3: nothing is sent while watching nobody, and nobody is kept
  through a promotion. Observable only as an absence, with no barrier, so
  no change.
- Task 1, G4: the empty id's refusal string differs from VTT-347's. No
  change: VTT-347 is about a named actor, and the empty id names none.
- Task 1, G5: which square the move gate asks about. The destination, since
  `handleCommand` passes `to`. SPEC-015 is to say it.
- Task 1: SPEC-015's consequence "watches nobody from the pump's next
  re-judging" is stale. SPEC-015, updated after this report.
- Task 1: the set's sort order, and "a set that has not changed is not sent
  again", needed ids. Refused, as above.
- Task 2, gap 1: SPEC-015 is stale on who may watch whom. SPEC-015.
- Task 2, gap 2: SPEC-007's empty-frame sentence can no longer occur, because
  an empty set always carries the flag. SPEC-007, replaced rather than
  amended.
- Task 2, gap 3: a false flag's wire form, absent or explicit. No change:
  protojson omits a false bool, and both decode alike.
- Task 2, gap 4: a promotion that changes neither the actors nor the flag
  sends no set. No change: it follows from "sent only when the set has
  changed".
- Task 2: the field's polarity, SPEC-007's ordering, and "a grant sets the
  kind" needed ids. Refused, as above.
- Task 3: the perch's order, "by label and then by id", does not say which
  comparison. SPEC-022 is to say it is string order.
- Task 3: doors armed, untested by QA. No change: the existing door tests in
  `client/test/app.test.ts` hold it.
- Task 3: VTT-460 says "one of those party members", and SPEC-022 says "the
  lowest such id". No change: both are true, and `client/test/player.test.ts`'s
  two-on-one-cell test holds the stricter one.
- Task 3: QA's chosen readings. A nobody button is a switch button, "sends
  nothing" means no frame, and the perch includes its "Perched on" line. No
  change: all three match the rows.
- Task 3: SPEC-022 is stale on who perches, and cites VTT-402 and VTT-406.
  SPEC-022.
- Task 3: the perch texts, "Whose eyes", a discard keeping the set, and "Bara
  knappar" needed ids. Refused, as above.
- Every task: QA's tests were added to the evidence of the new rows they
  cite, VTT-450 to VTT-461, and some to VTT-340's. The other older rows they
  cite do not list them.

## Phase 4b

One reviewer per task, two passes each, by command, on copies of the tree.

- **Task 0 (`59da686`).**
  - Pass 1: 2 Must.
    - VTT-456 claimed a lasting stay.
    - VTT-457, 458 and 460 were false for a window with no role, a stale role,
      or a log that cannot fold.
  - Pass 1: 3 Should.
    - Comments and a test name saying only a spectator has the perch were
      unlisted.
    - Task 3 could not meet its own re-pointing check.
    - A citation named "the last plan".
  - Pass 2: 3 Should.
    - The rows tied the set to the window, not the client.
    - Task 6's SPEC-015 bullet was false under the stay.
    - Ticket item 9 lacked the exceptions.
- **Task 1 (`fe6ed1f`).**
  - Pass 1: 2 Must.
    - Two VTT-456 entries named tests that do not hold it.
    - A "set comes before the ViewpointChanged" line sat above a test that no
      longer checks it.
  - Pass 1: 3 Should.
    - Two VTT-450 entries held VTT-340.
    - A silence check was dropped from a promotion test.
    - QA's file had no ledger row.
  - One finding was left as it stands: `docs/verification-debt.md` names
    `TestQAQueuedPromotedSpectatorOnAnothersPartyMemberMovesToNobody`, the
    name Task 1's rename retired, in a closed entry that records what was
    observed on `3ddf8e7`.
  - Pass 2: nothing.
- **Task 2 (`afd206a`).**
  - Pass 1: 2 Should.
    - A flag counting only a party member's first controller survived every
      test, so N7 gained a fifth case.
    - `WatchableChanged`'s message comment had not been updated.
  - Pass 2: 1 Nit, a ledger row.
- **Task 3 (`3c7c0db`).**
  - Pass 1: 1 Must. Stryker's `(true && controlsNone)` mutant in `mayPerch`
    survived every test, so the renamed shoulder test now sends the DM and
    the agent a flagged set.
  - Pass 1: 5 Should.
    - `style.css`'s perch heading still said the perch was the spectator's.
    - `spectator.ts`'s comment warned of the wrong hazard.
    - W1's "is shown that log" could not fail, since Ash and Bram both stand
      at 0,0.
    - The e2e red recorded was a precondition's.
    - The `isPlayer` departure was recorded with a false reason.
  - Pass 2: 1 Should. `player.ts`'s doc quoted the board's old copy of the
    doors bit.
- **`0aacc59`.**
  - Pass 1: 2 Must.
    - The bundle was not rebuilt, so `check:drift` would have failed.
    - The comment in "a DM gets the player panel as well as the console" said
      `canAct` lists each role in its own arm.
  - Pass 1: 3 Nits on the commit message, among them that an unknown role
    would now be offered the panel.
  - Pass 2: 1 Nit on the commit message.

## The deliberate breaks

Each was applied by a script to the working tree, run, and undone by its
inverse edit, with each file's hash compared afterwards. Each count was
measured during its own task, before that task's review, so a test its review
or a later task added is not in it. The doors test is one: on `3c7c0db`, Kc1
turns 4 red and Kc2 5. Counts are without the task's QA file unless marked; the
reviewers' counts with it follow in brackets where measured.

| Break | Red |
|---|---|
| K1, `MayWatch` accepts any party member for every player | 39 (47 with QA's file) |
| K2, the old player rule kept | 16 (29), N1-N6 and the stay among them |
| K3, the empty id accepted for every player | 3, `TestAPlayerMayNotWatchNobody` among them |
| K4, `judgedBy` left on `MayWatch` | 3 (4), N4 and both edited move-gate tests |
| K5, `judgedBy` trusting any party member | 4 |
| K8, `resolve` moving every player who controls none off any party member at every wake | 6 (15) |
| K8p (reviewer's), a move only on the wake that takes the last party member | 7 with QA's file, exactly VTT-456's evidence |
| K9, the predicate counting a `NON_PARTY` actor | 6, N1 among them; N1 without its second player stays green |
| K6, the flag always false | 6 (21) |
| K7, `tell` comparing actors alone | 1 (7), N8 |
| K9 on the flag | 4 (10), N7 among them |
| O2 (reviewer's), a flag counting only the first controller | 0, until N7's fifth case; then 1 |
| Kc1, `mayPerch` ignoring the flag | 3, C1-C3 |
| Kc2, the panel drawn for a perched window | 4 |
| Kc3, the board click left wired | 2, C3 among them |
| Kc4, a discard clearing the flag | 2, C1 and C3 |
| Kc5, the perch dropping "" for a player | 1, C1 |
| A7, `(true && controlsNone)` | 1, the renamed shoulder test |
| `canAct &&` dropped from the board's `doorsArmed` | 1, the doors test |
| `0aacc59`: Stryker scoped to `client/src/app.ts`, same container and config | 222 mutants, 222 killed; the six on the new `canAct` line among them |

Kc3b, the plan's Kc3 routed through `playerClickFor`, is Kc3 here: `isPlayer`
is unchanged, so a wired click goes through `playerClickFor` already. Under
Kc1 to Kc5, "a player is offered exactly the party members the set names" and
"a player switches by a button and by a click on the token" stay green.

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool/maptool`, at `f4b7fef6c`.
`ZoneViewModel.makePlayerView` builds a player's view from their selected owned
tokens with sight. Without a selection, it uses all their owned tokens with
sight when `ServerPolicy.isUseIndividualViews` holds or the server is a
personal one, and every PC token with sight otherwise, which is the default.
So by default a player who owns no token sees through the whole party. A move
is gated by ownership alone.

What is borrowed: a player with no character of their own sees through the
party. What is not:
- **The union.** Here they choose one party member's shoulder, as a spectator
  does, since a client holds one log.
- **The ownership-only move rule.** Here a move is judged by a party member
  the player controls, and one who controls none moves nothing.
- **The distribution.** MapTool sends every client the whole campaign. The
  plan's rule 9 section carries the detail.

## Figures

From the `task check` on `0aacc59`, unless a line says `3c7c0db`:

- Go coverage: 21 packages at or above their floors; the gate notes 11 that
  clear theirs by more than 1.0pp. `internal/gateway` measured 93.9% against
  its 93.5% floor, as on `0d7ebc8`, the previous ticket's last code commit.
- Client coverage: 22 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - On `3c7c0db`, 13 of the 15 were measured afresh; `internal/artlib` and
    `internal/campaigncfg` reused earlier verdicts.
  - On `0aacc59` every package reused its last passing verdict, since its
    Go code is `3c7c0db`'s.
  - `internal/gateway` has 2 survivors, both adjudicated before this ticket:
    `abs`'s boundary in `authz.go`, and the boundary of `watchable`'s sort in
    `viewpoint.go`, a line this ticket left byte-identical.
  - 7 timed out and were counted as killed, not measured: 4 in
    `internal/sight`, 1 each in `internal/mcp`, `internal/perceive` and
    `internal/rules`. This ticket changed none of those packages.
- Client mutation, in its container: Stryker's own "Done in 45 minutes and 48
  seconds". 2966 mutants: 2891 killed, 9 timed out and counted as killed, and
  66 survivors, all adjudicated equivalent. The adjudication files are
  unchanged since `f2dd819`.
  - `client/src/app.ts`: 222 killed, none survived; 224 on `0d7ebc8`.
  - `client/src/session.ts`: 62 killed and 2 survived, against 60 and 2.
  - `client/src/wire.ts`: 123 killed, 1 timed out and 2 survived, as on
    `0d7ebc8`.
  - `client/src/view/spectator.ts`: 378 killed and 2 survived, as on
    `0d7ebc8`. The two are `renderPerch`'s sort, untouched here.
  - `client/src/view/player.ts`: 241 killed, none survived, as on `0d7ebc8`.
  - On `3c7c0db`: 2981 mutants, 2905 killed, 9 timed out and 67 survivors.
    One survivor was unadjudicated, in `app.ts`.
- Race: both runs pass.
- Requirements chain: 461 rows, 290 test files, 16 specifications.
- Comments: 321 files, clean, with 17 comment lines added against `main`.
  Its two notices are the ones `0d7ebc8` had:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
- New prose: 2959 added lines across 31 files, clean.
- Disk: 41 GiB free by `df -h` just before the launch, after
  `go clean -cache`; 39.5 GiB, by the gate's own line, when the Go mutation
  step began.
- Duration: 3931 s, from the launch at 04:25:32 to the exit at 05:31:03,
  against 5071 s on `0d7ebc8`. The run on `3c7c0db` took 7715 s, with every
  Go package but two measured afresh.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D7: a perched player is left out of `isPlayer`'s selection sync | `isPlayer` is unchanged | The reviewer measured it: leaving them out keeps an armed ability armed across a perch and back, against app.ts's own rule that an armed ability is the watched character's (VTT-408). Nothing reads the synced fields while `canAct` is false |
| D7 names no change to doors | The board's `doorsArmed` is `canAct && ui.doorsArmed` | The Task 3 review found a perched player shown "Doors armed" with no toggle and no wired click |
| D5 changes the field's comment and `SetViewpoint`'s | `WatchableChanged`'s message comment changed too | The Task 2 review: it said the frame is sent again when the set of party members changes, and it now also is when the flag does |
| P2b: the field adds no Go red beyond the rule's 23 | Two of QA's Task 1 tests went red in Task 2 | They compare whole sets with `proto.Equal`, and were written before the field existed. They gained `qappWatchableNone` |
| N7 holds four participants | Five | The Task 2 review's O2 survived every test until a co-controller's case was added |
| Task 1's ledger changes viewpoint.go and viewpoint_test.go | `--write-ledger` lowered eleven rows, `client/src/app.ts` 49.7 to 49.5 among them | The tool lowers every row to its measured share and never raises one; Task 3 then added no comment line to app.ts |
| D7: "`player.ts` is edited only to re-point its VTT-402 and VTT-406 pointers, and `spectator.ts` only to reword its two comments E9 names, in place" | Both files' `doorsArmed` docs, `PlayerUIState.doorsArmed` and `ViewExtras.doorsArmed`, were also rewritten, as one-line docs. `spectator.ts`'s `PerchControl` doc, which E9 does not name and the ticket's "What it touches" does, was reworded from "The spectator's perch" to "The perch" | check:comments holds a touched block to six lines and refuses its task numbers; the long blocks described how the bit is shared, which this report now holds: app.ts owns it and hands the console and the board copies |
| QA files committed as delivered | Task 3's file had its types fixed. Task 1's five-round QA test runs one round. `TestQAPPTheWatchableSetFollowsThePlayersControl` gained `tb.expectWatching("pm-b", at+1, cn)`, and Task 1's file lost VTT-450 from two tests' citations and VTT-456 from one, and gained VTT-454 on two in Task 2. Task 2's file gained VTT-455 on `TestQACNTheDMAndTheAgentAreSentNoSetAsControlChanges` | The typecheck. The Task 1 review found that the rounds counted runs rather than isolating a mechanism. Its Must, that two VTT-456 entries named tests that do not hold it, is answered by the assertion, which makes that test hold the stay, and by the VTT-456 citation's removal from `TestQAPPAWatchedActorThatLeavesThePartyMovesThePlayerToNobody`. Its Should, that two VTT-450 entries held VTT-340, is answered by the two VTT-450 removals. The Task 2 test sends Ada the set again at a revocation and a grant that change only its flag, which is VTT-455 |
| Task 3 is the last code commit, and Task 4's gate runs on it | The gate on `3c7c0db` failed at `check:ts-mutation`, its last step, and nowhere else; `0aacc59` became the last code commit, and the gate ran again on it | One mutant survived: `canAct`'s role disjunction replaced by `true`. Task 3 put `!perches` in front of it, and `perches` is true for every spectator, so over the four roles `identity.ParseRole` accepts the disjunction decided nothing. The plan allows no adjudication on code it writes, so the term went. A role the server cannot send would now be offered the panel. Task 3's reviewer applied Stryker-shaped mutants by hand and found `mayPerch`'s; nothing ran Stryker itself before the gate |
| Plan order step 1: W1 red at the perch on Task 2's bundle | W1 red at its precondition in the writer's run on `afd206a`'s bundle | The first test stopped at beat 2, before its grants; the reviewer's run carried it past them and saw W1 red at the perch |
| D8 relies on the one-worker config for W1 to follow the first test | `client/e2e/handover.spec.ts` runs serially, by `test.describe.configure({ mode: "serial" })` | W1 starts where the first test ends. One worker already runs the file in order: in the writer's run on `afd206a`'s bundle W1 still ran after the first test failed, and went red at its precondition. Serial mode skips W1 when the first test fails |
| W1 goes from Ash to Bram | W1 clicks "No shoulder" first, then Bram | The Task 3 review: Ash and Bram both stand at 0,0, so "is shown that log" could not fail; an empty Story and no token between them makes it able to |
| The opening watcher's `forbidden` check kept | Deleted with its field | Its only setter was the test whose old expectation the rule reverses |

## What could not be established

- 16 mutants were counted as killed without being measured: 9 in the client
  and 7 in Go. None of them is in code this ticket changed. 6 are in
  `client/src/view/scene-plan.ts` and 2 in `client/src/view/canvas.ts`. One
  is `wire.ts`'s `BlockStatement` of `ws.onerror` in `connect`, which this
  ticket did not touch and which timed out on `0d7ebc8` too. The Go 7 are in
  packages this ticket did not change. The gate reads such a count as a
  measurement of the machine, and no test was changed.
- Whether a player who controls only a `NON_PARTY` creature should still be
  able to `use_ability` or `open_door` with it through `vtt mcp`: `Authorize`
  judges no sight for either, the browser offers neither to a perched player,
  and this ticket changed neither (plan, Forward coupling).
- The perch's order compares labels as strings; whether a table needs locale
  order was not asked.

## What was deliberately left out, and where it went

- Whether a player without a character may speak: B-2, whispering, the sixth
  ticket. A perched player has no narration box, as a player with no actor had
  none before.
- What a token shows of a creature: B-5, K-1 and A-6, the fifth ticket. A
  perched player is sent the log of the party member they chose, so whatever
  that ticket withholds from a character's log, it withholds from them.
- Telling a player why their window changed at a revocation: A-7, the eighth
  ticket. Under "Stay on Tobbe" the window no longer empties; the perch's
  "Perched on" line names who they watch.
- The specifications: SPEC-015, SPEC-013, SPEC-022, SPEC-007 and SPEC-021, in
  the plan's Task 6, after this report. They carry QA's G5, the perch's sort
  comparison, the doors behaviour above, and the stale sentences QA found.
