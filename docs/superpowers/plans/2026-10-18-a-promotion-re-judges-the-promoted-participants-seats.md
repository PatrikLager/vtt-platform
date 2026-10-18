# Plan: a promotion re-judges every seat of the participant it promotes

- **Ticket:**
  `docs/superpowers/specs/2026-10-18-a-promotion-re-judges-the-promoted-participants-seats-design.md`.
- **Verdict:** passes with gaps (G1-G6, below), verified 2026-10-18 by an
  agent that did not write the ticket, read-only, against HEAD `526e090`.
  Its probes ran through `go test -overlay` and wrote only to the scratchpad.
- **Signed off:** 2026-10-18 by the owner ("Ja, kör"). The owner chose one
  row for the opening and the queued entries (VTT-387, under "Requirements")
  and moved the gate before the report (Tasks 4 to 6). After sign-off the
  ticket was amended for G1 to G5: item 4 reworded, items 9 (T4b) and 10
  (T6) added, its rules rewritten, and `presence_internal_test.go` listed.
  Phase 4b's reading review before Task 0's commit amended this plan again,
  as listed under "Amended by the reading review before Task 0's commit".
- **Base:** main `526e090`.
- **Branch:** `fix/a-promotion-re-judges-the-seat`.

## The gaps this plan inherits

- **G1.** Item 4's "the last `ViewpointChanged` before its first entry names
  nobody" has no first entry to point at, because a seat on nobody is sent
  none (VTT-341). T4 pins it another way: before the `ViewpointChanged` naming
  nobody, no frame is an event, and a narration appended afterwards sends
  nothing.
- **G2.** The queued entries the problem names, and that the debt's recipe
  describes, are pinned by no rule. This plan adds VTT-387 and T6 (Task 3).
- **G3.** Item 5 is covered by no row. VTT-340 governs players, and VTT-385 a
  spectator who watches nobody. This plan adds VTT-388 for T5.
- **G4.** An idle opening is untested. This plan adds T4b (Task 2).
- **G5.** `presence_internal_test.go` is touched (T7). The ticket did not list
  it at verification; it now does.
- **G6.** Item 6's grep keeps a present-tense sentence. Task 5 keeps the
  sentence word for word and puts the past tense in the heading.

## Constraints that bind every task

Each is named here and stated where it lives:

- CLAUDE.md rules 2 (`task check` is never weakened to pass), 3 (no contract
  change is made or needed), 4 (one fold: nothing here applies an event), 5,
  8 (cite names, never `file:line`), 9 (below) and 10 (a code comment is a
  warning or a pointer).
- SPEC-011. The pump is the only producer of a connection's entries and
  `ViewpointChanged` frames. Presence locks are taken in the order `fanOut`
  then `mu`, never the reverse. A connection joins presence after its head and
  before its pump.
- SPEC-015. The command goroutine never waits on a pump (`perchBox.set`'s
  rule). A seat is touched by its pump alone.
- SPEC-007, VTT-368 and VTT-384. The head comes first. The first
  `ViewpointChanged` names what the seat opened on, and comes before any entry.
- VTT-348, VTT-355 and VTT-367. No entry of a log that was left follows the
  `ViewpointChanged` that left it.
- SPEC-008. Ids come from `requirement-id` after sign-off, never by hand.
- SPEC-010 ceilings in `tools/comment-ceilings.txt`. Measured on HEAD:
  - `server.go`: 196 comment lines of 875, **22.40 against a ceiling of
    22.4**. Every added comment line needs about 3.5 added code lines.
  - `seat.go`: 32 of 195 (16.41 against 16.5). One comment line needs 4 code
    lines.
  - `presence.go`: 32 of 202 (15.84 against 15.9). One comment line needs 5
    code lines, two need 10.
  - `watch_test.go`: 11 of 556 (1.98 against 2.0). New tests there carry no
    comment line except `// VTT-NNN` citation lines, which the share excludes.
  - `seat_internal_test.go`: 3.46 against 3.7, room for about one doc line
    per test.
  - `presence_internal_test.go`: 30.44 against 30.5.
- Coverage floor for `internal/gateway` (`tools/coverage-thresholds.txt`):
  93.5. `go test -cover` on HEAD measured 93.9. That is a single-package
  figure, not the gate's merged one.
- Mutation gate: zero unadjudicated survivors (`tools/mutation-equivalents.txt`,
  SPEC-020). Its one `internal/gateway` row is in `authz.go`, which this work
  does not touch.
- No exported symbol changes. `go doc ./internal/gateway` is the same before
  and after, and Phase 4a is handed it unchanged.
- `review-record` refuses untracked files. Commit the ticket and this plan
  before Task 1's review.

## Rule 9: how MapTool does this

It does not. A MapTool client's role is set once, at the handshake, from the
server's `ConnectionSuccessfulMsg` (`ClientHandshake.handle`). `Players.setRole`
"will not change the role of a currently logged in player, they will have to
log out and log back in for it to take effect". MapTool has two roles, GM and
PLAYER, and no spectator, so there is no seat to re-judge. Its view is a
client-side filter over a campaign that every client holds, which is the
distribution model rule 9 forbids borrowing. This was checked, and nothing is
borrowed. The ticket's answer is correct.

## The design, argued

### (a) Where the per-connection re-judge signal lives

| Option | For | Against |
|---|---|---|
| **A1. On `presenceConn`, raised by a new `presenceRegistry` method** | The registry is already the server's one map of live connections, keyed per connection and carrying the participant id (SPEC-011: one participant, several connections). A connection joins it before the pump starts and leaves it before the pump stops (`serve`'s `shutdown` order). `handlePromotion` already reaches it. | Presence ("who is online") gains a second concern. The registry's `mu` gains a user. With a test buffer of 0, `joinAndSend` sends under `mu` for up to `presenceSendBudget`, so a promotion could wait behind a joiner's snapshot in such a test. |
| A2. A separate registry on `Server` (participant id → signal channels) | Keeps presence about presence. It could be registered before `openSeat`, which enables option c4 below. | It copies the presence registry's membership and its join and leave lifecycle, which then have to be kept in step: the 2026-09-23 ruling is to point at the source, not copy it. It is one more structure for mutants and teardown. |
| A3. The receive end on `seat` (beside `wake`), so `watch` and `admit` read it | It mirrors `wake`. | A seat cannot read the role; only `pumpState.revoked` can. A seat method that took the signal would judge by a role read before the take, and the signal would be lost (see D2). It still needs A1 or A2 to be found. |
| A4. An identity-level watch: `identity.DB` raises a wake on `SetRole`, like `WatchAppends` | Any in-process role change raises it. | It crosses into `internal/identity`, outside the ticket's one component. It wakes every seat on any role change. `SetRole` has one caller. |
| A5. Merge into the append wake | It reuses every `wake` reader. | The gateway cannot raise `Campaign.WatchAppends`'s channel. And every `wake` reader (`watch`'s settle loop, `admit`) judges with a role read before it took the wake, so a promotion taken there is lost. |

**Recommended: A1**, with the receive end in `pumpState` (D1, D2).

### (b) How the pump consumes it, so an entry taken after it is judged by the new role

| Option | For | Against |
|---|---|---|
| b1. One select arm only | It is the smallest change. | Go's `select` picks at random among ready cases. With the signal waiting and entries queued, queued entries of the old log are still sent until the arm wins: the debt's recipe, unclosed. |
| b2. `onEntry` takes a waiting signal first, as `admit` takes a waiting wake, then looks the role up and rechecks | It is the analogy the ticket draws. | It is correct only if the take comes before the lookup. With the two lines swapped, the signal is taken under the old role and lost, and no deterministic test can hold that order, because a test would have to land `SetRole` between the two lines. It also leaves the window between `SetRole` and the signal. |
| b3. Recheck on every entry | No new state. | It clones every actor under the campaign mutex (`Campaign.Actors`) once per entry per seat. Today that happens once per append per seat, and the same mutex serialises appends. |
| **b4. The seat records the role it was last judged by, and `onEntry` rechecks, after its last lookup and before it queues the entry, when the participant now holds another one** | It is a pure state comparison: internal tests drive it directly (T6, and T6b with a lookup that answers differently on its two calls). It also covers the window between `SetRole` and the signal, and any future role-change path that forgets to signal, for every seat that is still receiving entries. | It adds one field to `seat`, which must be set wherever the seat resolves (`watch`, `recheck`). |

**Recommended: b4 for entries, plus b1's arm for an idle pump** (D4, D6).
Under b4, an entry queued after any lookup of the pump has returned the new
role is judged by that role, because the comparison follows `onEntry`'s last
lookup. The only entry still sent under the old role is one whose last lookup
ran before `SetRole` committed, which is the window the ticket's "What could
not be established" accepts.

### (c) The opening race (ticket item 4)

The role `openSeat` judges by is read by `Verify` in `handleWS`. The signal's
registration (`joinAndSend`) comes after the head is queued and before
`go pump`.

| Option | For | Against |
|---|---|---|
| c1. Re-judge at pump start, before the opening `ViewpointChanged` | One frame, never a flicker. | The head names the opened log's catch-up and the single `ViewpointChanged` names another. SPEC-007 tells a point-in-time reader to read "until it has seen `head_sequence`, or until a `ViewpointChanged` after the first". There is no second one, so a reader that follows SPEC-007 waits for a head that never comes. VTT-368 ("naming what it opens on") stops being literally true. |
| **c2. Announce what the seat opened on, then re-judge** | The head, the first `ViewpointChanged` and SPEC-007 stay as they are. A promotion at opening becomes an ordinary move: a `ViewpointChanged`, then no entry of the log left (VTT-367). It is one expression, built from the existing `moved(recheck())`. | Two `ViewpointChanged` frames when a promotion lands while the seat opens. It costs one `Actors()` call per opening. |
| c3. Create the signal channel already holding one signal | No start code. | It is "a signal nobody raised" that a reader will tidy away. It ties (c) to (b): remove one and the other silently goes too. |
| c4. Register before `openSeat`, look the role up again, and open with that answer | The opening already has the new role. | It needs A2, because presence must join after the head. It adds a second identity read on every opening, which needs its own error policy. (b) is still needed afterwards. |

**Recommended: c2** (D5). Why it closes the race is a READING, recorded for
Phase 4b:

- A promotion whose `SetRole` commits before the pump's first lookup is seen
  by that lookup, and the start re-judge acts on it.
- A promotion whose `SetRole` commits after that lookup raises its signal
  after the commit. `joinAndSend` registered the connection before `go pump`,
  so the signal finds it.

No interleaving escapes both. No test can order the pump's first lookup
against a registration from outside, so this half is a reading, and stays one.

## Decisions

1. **D1. The signal is a capacity-1 channel on `presenceConn`, raised by a
   `presenceRegistry` method that takes `mu` alone and never blocks on the
   channel.** Forced by A1's row. It never blocks on the channel for
   `perchBox.set`'s reason (SPEC-015); it can wait on `mu` while
   `joinAndSend` sends under it (A1's row). It never takes `fanOut`, because a
   fan-out can wait `presenceSendBudget` for each wedged connection.
2. **D2. The receive end is a `pumpState` field, not a `seat` field.** Forced
   because only `pumpState.revoked` reads the role, and the role must be read
   after the signal is taken.
3. **D3. `handlePromotion` raises the signal after `SetRole` and before
   `announcePromotion`.** After, so the lookup that follows the take reads the
   new role. Before, because `announcePromotion`'s fan-out can wait, and the
   re-judge needs nothing from it.
4. **D4. The pump's select gains an arm for the signal that runs `onWake`.**
   Forced because `onWake` already is "re-resolve, then recheck", in that order.
   A second body would be a second copy of the re-judge. Its doc sentence then
   names a promotion as well as an append.
5. **D5. At start, the pump announces what the seat opened on, then always
   runs `moved(recheck())`.** Forced by SPEC-007's point-in-time reader and by
   VTT-368/VTT-384 (option c1's row). The start re-judge is also the only path
   for an idle opening (T4b): a seat that receives no entry gets no admission
   check, and a promotion that commits before `joinAndSend` raises no signal
   it can see. It is unconditional because a
   role-changed guard would add a conditional with an equivalent mutant (a
   recheck when nothing changed moves nothing), for the saving of one
   `Actors()` call per opening.
6. **D6. A log seat records the role it was last judged by, set in `watch`
   and in `recheck`. `onEntry`, after its last lookup and before it queues the
   entry, rechecks a log seat whose participant now holds another role, and
   sends no entry of a log that recheck left.** Forced by b2's untestable line
   order, b3's cost under the campaign mutex, and `onEntry`'s second lookup:
   a comparison inside `admit` runs before that lookup, so a role it returns
   would go unjudged and VTT-387 would be false (found by Phase 4b's reading
   review before Task 0's commit). A world seat is never compared: its
   `wake` is nil, and a dm or an agent cannot be promoted.
7. **D7. `watch`'s settle loop does not read the signal.** Forced because the
   loop exists so that `openSeat`'s head names the log shown (VTT-384). After
   the opening the head is spent. A signal that arrives during a later `watch`
   is taken by the pump's next select, or caught by D6 at the next entry.
8. **D8. Every connection gets a channel, a world seat's included.** This is a
   PREFERENCE, named as one: it avoids a role branch in `serve`. A world seat's
   channel never fires, because `handlePromotion` refuses a dm or agent target.
9. **D9. T2 appends its grant before the spectator connects.** Forced because
   a grant appended while the spectator is connected raises a wake. If the
   promotion lands before the pump takes that wake, `onWake` re-judges by the
   new role, and the test passes on today's tree.
10. **D10. `pumpUnderTest` gains the named log and a hook that runs between
    `openSeat` and the pump.** Its one caller
    (`TestAPerchTheActorsNoLongerAllowIsNotApplied`) passes `""` and a no-op.
    Forced by one harness rather than two (single source), and because T4 and
    T4b need that window.

## Requirements

Allocated by `requirement-id` after sign-off. Each new row starts as
`**OPEN — no test yet**` and is filled in the commit whose test verifies it.

- **VTT-340** (exists, "Whenever ..."): its evidence gains T1, T2, T3, T4b and
  T7. Ticket rule 1 is this row holding without an append, so it gets no row
  of its own.
- **VTT-387** (ticket rule 2, G2; the owner folded the proposed R-open into
  it): "Every entry a player's or a spectator's connection queues after its
  pump has re-resolved its participant to a new role is of a log that role
  may watch." The re-resolution is `pumpState.revoked`'s; the read loop's
  lookup before a command does not replace the seat's participant. Verified
  by T4 (the opening instance), T6 and T6b.
- **VTT-388** (ticket rule 3, G3): "A promotion leaves each of the promoted
  participant's connections watching what it watches while the new role may
  watch it." Verified by T5 and T5b.


## Tasks

The pattern is the same in every code task. Write the tests and see each one
fail for the reason stated. Implement. See the tests pass. Apply each
deliberate break and see the named test go red, then restore the code exactly,
using the inverse edit, never `git checkout --`. Run
`go test ./internal/gateway/ -count=1` and
`go test -race ./internal/gateway/ -run '<the task's tests>'`. Check the load
with `os.getloadavg()`, then run the task's scheduling-sensitive tests with
`-count=20 -cpu 1,2`. Run `python3 tools/check-comments.py main` after
staging. Each task is its own commit through the review gate.

### Task 0: sign-off and ids

- Files: `docs/requirements.md`, the ticket and this plan, both committed.
- True when: the owner has signed off D1-D10, and `requirement-id` has
  allocated VTT-387 and VTT-388, each OPEN. The ticket and the plan are
  tracked.

### Task 1: a promotion signals every connection of the participant, and the idle pump re-judges

- Files:
  - `internal/gateway/presence.go`: `presenceConn` gains the channel, and a
    new `presenceRegistry` method raises it.
  - `internal/gateway/server.go`: `serve` creates the channel on `pc` and
    hands it to `pump`. `pumpState` gains the field. `pump`'s select gains the
    arm (D4). `handlePromotion` raises the signal (D3).
  - `internal/gateway/seat_internal_test.go`: `pumpUnderTest` passes a channel.
  - `internal/gateway/presence_internal_test.go`: its `conn` helper gives each
    connection a channel, plus T7.
  - `internal/gateway/watch_test.go`: T1, T2, T3, T5, T5b, and one promote
    helper with no doc comment.
  - `docs/requirements.md`: VTT-340's and VTT-388's evidence.
- Tests:
  - **T1 `TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch`**
    (VTT-340). The spectator perches on a1 (`setViewpoint`) and reads a1's log
    dry. The DM sends `promote_participant` to player. `untilViewpoint`
    answers `""` with no entry before it. A narration is appended and the
    spectator stays `quiet`. Today it fails: no frame within 3s.
  - **T2 `TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold`** (VTT-340).
    `partyMemberOf("c1", spectator)` is appended before the spectator dials
    (D9). The opening is head 0 and nobody. After the promotion,
    `untilViewpoint` answers `c1` with no entry before it, then `requireLog`
    sees c1's log from its first entry. Today it fails.
  - **T3 `TestAPromotionMovesEveryConnectionOfTheParticipant`** (VTT-340). Two
    connections of the spectator, both on a1. After the promotion, each one's
    `untilViewpoint` answers `""` with no entry before it. Today it fails.
  - **T5 `TestAPlayerMadeASpectatorKeepsWatchingTheirPartyMember`** (VTT-388).
    The player reads a1 dry and is promoted to spectator. A narration is
    appended, `entries(1)` is a1's new entry, and the connection stays
    `quiet(300ms)`. It passes today. Once the test blocks in `quiet`, the pump
    has the signal as its only ready case, which is what makes the third break
    below deterministic.
  - **T5b `TestAPromotedSpectatorKeepsWatchingAPartyMemberTheyHold`**
    (VTT-388). The spectator controls `c1` and `c2`, perches on `c2` and reads
    it dry, and is promoted to player. A narration is appended, `entries(1)`
    is c2's new entry, and the connection stays `quiet(300ms)`. It passes
    today; the player's default would be `c1`, so a promotion that moved every
    seat to its default sends a `ViewpointChanged` naming `c1`.
  - **T7 `TestARejudgeSignalsEveryConnectionOfTheParticipantAndNoOther`**
    (VTT-340, in `presence_internal_test.go`). Two connections of `p-1` and
    one of `p-2` join. The method is raised twice for `p-1` inside a goroutine
    bounded at 1s. Each `p-1` channel then holds one signal and `p-2`'s holds
    none. Its "and no other" half is held by no row; it stays as the
    registry method's unit check.
- True when: T1, T2, T3 and T7 pass, and T5 and T5b still pass. Every existing
  promotion test passes. By reading, they promote participants who control
  nothing and watch nobody, so none of them moves. The package run is what
  confirms it.
- Breaks:
  - Delete the signal call in `handlePromotion`: T1, T2 and T3 go red, T5,
    T5b and T7 stay green.
  - Return after the first matching connection in the registry method: T3 and
    T7 go red.
  - Replace the arm's `onWake` with `ps.revoked()`, a `watch` of
    `defaultViewpoint(ps.sub.p, …)`, then `announce`: T5 goes red (a
    `ViewpointChanged` naming nobody) and T5b goes red (one naming `c1`).
    Both must be red at `-cpu 1` and at `-cpu 2`.
  - Drop the registry send's `default`: T7 goes red on its bound.

### Task 2: the pump re-judges after its opening announcement

- Files:
  - `internal/gateway/server.go`: `pump`'s start (D5).
  - `internal/gateway/seat_internal_test.go`: `pumpUnderTest` per D10, plus T4
    and T4b.
  - `docs/requirements.md`: VTT-340's evidence (T4b). T4 carries VTT-387's
    citation from here, and VTT-387's evidence is filled in Task 3, once the
    rule also holds for an entry mid-stream.
- Tests:
  - **T4 `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder`** (VTT-387,
    ticket item 4, G1). A spectator's seat opens on `x1`, another player's
    party member. The hook calls `SetRole(player)`, then the pump starts. The
    test reads frames until a `ViewpointChanged` naming `""` arrives (3s
    deadline) and fails on any event frame before it. Then it appends a
    narration and wants no frame within 300ms. Today it fails: the pump sends
    `ViewpointChanged x1` and then x1's two entries (observed by the probe run
    during verification).
  - **T4b `TestAPromotionAsTheSeatOpensMovesItOntoTheirPartyMember`** (VTT-340,
    ticket item 9, G4). A spectator who controls `c1` opens on nobody and is
    promoted in the hook. The test reads until a `ViewpointChanged` naming
    `c1` (3s deadline), then reads c1's log from its first entry. Today it
    fails: one `ViewpointChanged` naming nobody, then nothing.
- True when: T4 and T4b pass. Every existing `pumpUnderTest` and opening test
  passes, because with the role and the actors unchanged the start re-judge
  moves nothing. `openSeat`'s choice is a fixed point of `resolve`.
- Break: delete the start `moved(recheck())`. T4 and T4b both go red, because
  Task 3's check does not exist yet.

### Task 3: an entry is admitted by the role the participant now holds

- Files:
  - `internal/gateway/seat.go`: the new `seat` field, set in `watch` and
    `recheck`.
  - `internal/gateway/server.go`: `onEntry`'s role comparison after its last
    lookup (D6), and its doc sentence.
  - `internal/gateway/seat_internal_test.go`: T6 and T6b.
  - `docs/requirements.md`: VTT-387's evidence: T4, T6 and T6b.
- Tests:
  - **T6 `TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole`** (VTT-387,
    ticket item 10, G2). A spectator's seat is opened on `x1` with no wake
    waiting (a fixture check, `len(sub.wake) == 0`). `SetRole(player)` runs
    and x1's first entry is taken. A `pumpState` with `lookup: s.ids.Lookup`
    and `closing` set runs `onEntry`, the construction
    `TestAnEntryTakenAsTheCredentialGoesIsNotSent` uses. It answers true,
    the out channel holds no event frame and one `ViewpointChanged` naming
    `""`, and `sub.shown` is `""`. Today it fails: the entry is sent and the
    seat shows `x1`.
  - **T6b `TestARoleReadAfterAdmitStillJudgesTheEntry`** (VTT-387). The same
    seat, with no `SetRole`: the `pumpState`'s `lookup` answers the spectator
    on its first call and the player from its second, the call-counting fake
    `TestAnEntryTakenAsTheCredentialGoesIsNotSent` uses. `onEntry` answers
    true, the out channel holds no event frame and one `ViewpointChanged`
    naming `""`. Today it fails as T6 does.
- True when: T6 and T6b pass, along with `TestNoEntryCausedByTheLossIsSent`
  and the whole package.
- Breaks:
  - Delete `onEntry`'s role comparison: T6 and T6b go red.
  - Move the comparison before `onEntry`'s second lookup: T6b goes red and T6
    stays green.
  - Apply Task 2's break again with Task 3 in place: T4b goes red and T4 stays
    green. This is expected, because T4 is now held by both mechanisms. The
    report records it.

This is the last code commit.

### Task 4: `task check`, whole, on the last code commit (ticket item 8)

- True when `task check` exits 0 and prints its own completion lines, on
  Task 3's commit, before any record is written. Launch it with
  `start_new_session=True`. The coverage floor stays at 93.5 or is raised,
  never lowered. No comment-ceiling row is raised; `--write-ledger` lowers
  rows that have dropped by more than 1.0. Every mutation survivor is killed
  or adjudicated in `tools/mutation-equivalents.txt`, with a reason
  (SPEC-020).
- The records after it change no `.go` or `.ts` file. Each record commit runs
  `check:requirements-chain`, `check:new-prose` and `check:comments`, which are
  the gates a record can move.

### Task 5: the debt entry closes, and the implementation report (ticket item 6)

One commit, as ticket 3b's report and debt entry were.

- Files:
  - `docs/verification-debt.md`.
  - `docs/reports/2026-10-18-a-promotion-re-judges-the-promoted-participants-seats.md`.
- True when the debt entry has moved out of `## Open debt` into a dated
  section above it, which follows the 2026-10-17 entry's shape:
  - The bold sentence is kept word for word (G6), and the heading says what
    happened, in the past tense.
  - The recipe becomes the edits that put the defect back: delete the signal
    call in `handlePromotion`, `onEntry`'s role comparison, and the pump's
    start `moved(recheck())`.
  - The entry says which gate should have caught it and why it did not, and
    keeps the label `test data missing`.
  - A **Closed by** paragraph names T1, T2, T3, T4, T4b, T6 and T6b, and says
    which of them reds on which edit: T1 to T3 on the first, T6 and T6b on the
    second, T4b on the third, and T4 only on the second and third together.
- `grep -n 'A promotion re-judges a seat only at the next append'
  docs/verification-debt.md` prints a line above `## Open debt`.
- True when the `implementation-report` skill's report exists, written
  against the ticket's ten done items, each with its observation. It
  includes:
  - G1-G6 and how each was handled.
  - The two-mechanism note on T4.
  - Phase 4a's adjudications under their own heading, one line each.
  - The coverage figure and comment shares from Task 4's run.

### Task 6: the specification step (ticket item 7)

- Files:
  - `docs/specifications/015-the-seat-and-the-perch.md`. The opening paragraph
    gains the re-judge after the announcement. "What moves a seat" gains a
    promotion bullet. The paragraph beginning "A promotion appends nothing" is
    rewritten. The paragraph "No entry is sent after the right is lost" gains
    the role clause. Its Requirements list gains VTT-387 and VTT-388.
  - `docs/specifications/009-identity-and-joining.md`. The re-resolution
    schedule under "Authentication happens once" gains "every promotion signal
    it takes". "Promotion is a command" gains the signal.
  - `docs/specifications/011-the-connection.md`. The presence-registry
    paragraph gains the signal method's lock and its no-blocking rule.
    "Delivery re-resolves the credential" points to the new schedule.
- True when each changed passage passes the `specification` skill's catches
  list, read against the committed code, and
  `grep -n 'wakes no seat' docs/specifications/015-the-seat-and-the-perch.md`
  prints nothing.

## For Phase 4a

- Hand the QA agent `go doc ./internal/gateway`, which is unchanged.
- Name `go-arch-lint` in the brief. No layer import is added.
- Hash the tree before and after the QA agent runs.
- The ordering half of decision (c) is a READING for Phase 4b, not a test.

## Amended by the reading review before Task 0's commit

Phase 4b's reading of the ticket, this plan and VTT-387/388, 2026-10-18, in
two rounds:

- M1: D6's comparison sat in `admit`, before `onEntry`'s second lookup, so a
  role that lookup returned went unjudged and VTT-387 was false as worded.
  The comparison moved after the last lookup (b4, D6, Task 3) and T6b was
  added. The row is unchanged.
- S1: Task 5's recipe gains the start re-judge's deletion, and Closed-by
  says which test reds on which edit.
- S2: the c1 row no longer argues from `vtt state dump`: the harness client
  drops `ViewpointChanged`, so the dump would time out under c2 as well.
- S3: VTT-388 gains T5b, a spectator promoted to player who keeps a party
  member they hold that is not their first.
- S4, N1 to N5: the header, G5, the ticket's record count, D1's "never
  blocks on the channel", D6's world seat, VTT-387's evidence moved to Task
  3, and T7's uncited half.
- VTT-387 names the pump: the read loop also re-resolves the participant,
  before every command, and does not replace the seat's. The uncommitted row
  was regenerated under the same id.
- N6 was not taken: item 6's grep names the bold sentence, so the entry
  keeps the debt's words, and the heading carries the past tense (G6).
