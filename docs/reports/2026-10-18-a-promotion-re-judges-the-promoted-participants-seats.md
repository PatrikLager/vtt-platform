# A promotion re-judges every seat of the participant it promotes: the change

**Ticket:** `docs/superpowers/specs/2026-10-18-a-promotion-re-judges-the-promoted-participants-seats-design.md`,
verified by an agent that did not write it (passes with gaps G1-G6).
**Plan:** `docs/superpowers/plans/2026-10-18-a-promotion-re-judges-the-promoted-participants-seats.md`,
written by the verifier and amended by the ticket's writer after the owner's
answers and after the reading review of `2180ac8`, the commit that carried
both.
**The owner's rulings, 2026-10-18:**
- on the question ticket 3b's specification step left open: a promotion
  re-judges the seat itself, rather than VTT-340 being succeeded by a row that
  waits for an append ("A, kör");
- at sign-off: the plan as written ("Ja, kör"); one register row for a
  promotion that lands while a connection opens and for the entries already
  queued, which became VTT-387 ("En rad"); `task check` on the last code
  commit before the records; and the rule of ticket 3b for review findings
  ("Ja, samma regel").

**Last code commit:** `3ddf8e7`, on `526e090`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `2180ac8` | The ticket, the plan, VTT-387 and VTT-388 |
| `cfc217d` | Task 1: a promotion signals every connection of the participant |
| `ce5b234` | Task 2: the pump re-judges after its opening announcement |
| `3ddf8e7` | Task 3: an entry is judged by the role the participant now holds |

The commit carrying this report also closes the debt entry (item 6). The
specification step follows it in its own commit.

## What the code did on `526e090`

`handlePromotion` in `internal/gateway/server.go` called
`identity.DB.SetRole` and `announcePromotion` and appended nothing (VTT-036),
so `Campaign.WatchAppends` raised no wake. A log seat's pump re-judged what
the seat showed only in `onWake` and in `admit` when an append wake was
already waiting; `onPerch` judges the actor a perch names, not what the seat
shows. `pumpState.revoked` replaced the seat's participant on
every step, which re-judged nothing. A spectator promoted to player therefore
kept the log they watched, and were sent its queued rest, until the next
append anywhere in the campaign. The same held while a connection opened: the
pump's start re-resolved the participant and announced what `openSeat` had
chosen without re-judging it.

## Done looks like, answered

| Item | Mark | Observation |
|---|---|---|
| 1 | `[x]` | T1 `TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch` in `internal/gateway/watch_test.go`: perched on a1, promoted, nothing appended; `ViewpointChanged` naming nobody with no entry before it, then a narration and silence. It failed on `526e090` with "no frame within 3s". QA's `TestQARejudgePromotedSpectatorOnAnothersPartyMemberWatchesNobody` holds the same, and the head unchanged. |
| 2 | `[x]` | T2 `TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold` in `watch_test.go`: c1 granted before the spectator dials, promoted, `ViewpointChanged` naming c1, then c1's log from its first entry. It failed on `526e090`. QA's `TestQARejudgePromotedSpectatorHoldingAPartyMemberWatchesItFromItsFirstEntry` grants a2, a1 and a3 so that first by id differs from first and last granted. |
| 3 | `[x]` | T3 `TestAPromotionMovesEveryConnectionOfTheParticipant` in `watch_test.go`, two connections opened with `log=a1`; and T7 `TestARejudgeSignalsEveryConnectionOfTheParticipantAndNoOther` in `presence_internal_test.go` for the registry. T3 failed on `526e090`; T7 calls a method `526e090` did not have. |
| 4 | `[x]` | T4 `TestAPromotionAsTheSeatOpensSendsNoEntryOfTheShoulder` in `internal/gateway/seat_internal_test.go`: the seat opens on x1, `SetRole` runs in `pumpUnderTest`'s hook before the pump starts; the first `ViewpointChanged` names x1, the next names nobody, no x1 entry comes before it, and a narration afterwards sends nothing. It failed on `cfc217d`'s code: x1's first entry was sent. The wording was changed after verification (G1). |
| 5 | `[x]` | T5 `TestAPlayerMadeASpectatorKeepsWatchingTheirPartyMember` in `watch_test.go`; it passed on `526e090` and passes. T5b `TestAPromotedSpectatorKeepsWatchingAPartyMemberTheyHold` adds the other direction (G3). QA's `TestQARejudgePlayerPromotedToSpectatorKeepsItsPartyMemberLog` holds it too, and `TestQAQueuedDemotedPlayerKeepsStreamingItsLog` with a log still being sent. |
| 6 | `[x]` | The entry moved out of Open debt into the section "2026-10-18 — a promotion re-judged a seat only at the next append's wake" of `docs/verification-debt.md`, closed by a paragraph naming the tests of items 1 to 4, 9 and 10. `grep -n 'A promotion re-judges a seat only at the next append' docs/verification-debt.md` prints a line above `## Open debt`. |
| 7 | `[ ]` | Not done at this report's commit: SPEC-015 is the specification step's, which follows this commit. The item stands, and that step's check is its grep. |
| 8 | `[x]` | `task check` whole exited 0 on `3ddf8e7`, in 2180 s, launched detached; the figures are below. |
| 9 | `[x]` | T4b `TestAPromotionAsTheSeatOpensMovesItOntoTheirPartyMember` in `seat_internal_test.go`: a spectator holding c1 opens on nobody, is promoted in the hook; the first `ViewpointChanged` names nobody, the next c1, then c1's log from its first entry. It failed on `cfc217d`'s code with "no frame within 3s". |
| 10 | `[x]` | T6 `TestAnEntryTakenAfterAPromotionIsJudgedByTheNewRole` and T6b `TestARoleReadAfterAdmitStillJudgesTheEntry` in `seat_internal_test.go`: an entry of x1 taken with no wake waiting, judged after `SetRole` (T6) or with a role only the lookup after `admit` returns (T6b); `onEntry` queues one `ViewpointChanged` naming nobody and no entry. Both failed on `ce5b234`. |

Items 9 and 10 were added after verification, for G4 and G2.

## What the rules became

| Ticket rule | Became |
|---|---|
| A promotion re-judges, with no append, what every connection of the promoted participant shows, by the new role. | VTT-340, as it stood. Its evidence gained T1, T2, T3, T7 and T4b, four of QA's rejudge tests, both of QA's opening tests and `TestQAQueuedPromotedSpectatorMovesToItsOwnFirstPartyMember`. No row of its own. |
| Every entry a player's or a spectator's connection queues after its pump has re-resolved its participant to a new role is of a log that role may watch. | VTT-387. Evidence: T4, T6, T6b. |
| A change of a participant's role leaves each of their connections watching what it watches while the new role may watch it. | VTT-388: "A promotion leaves each of the promoted participant's connections watching what it watches while the new role may watch it." Evidence: T5, T5b and five QA tests. |

VTT-387 was worded twice before `2180ac8`. As first dispensed it said
"after re-resolving its participant", and the reading review showed `serve`'s
read loop re-resolves before every command without replacing the seat's
participant, so the row was false under that reading; it was regenerated
under the same id to name the pump. A VTT-389, "A promotion appends nothing
to the log.", was dispensed for QA's first "needing an ID" and dropped before
`cfc217d`, uncommitted, as a duplicate of VTT-036; the QA tests cite VTT-036
and are in its evidence.

## Phase 4a

One QA agent per code task, each given its requirement, SPEC-015, SPEC-009,
SPEC-011 and SPEC-007 whole and `go doc` of `internal/gateway`,
`internal/identity` and `internal/campaign`, and each allowed one test file.
The writer hashed the tree before and after each, excluding the QA file, and
the hashes matched.

| Task | File | Tests | Failures |
|---|---|---|---|
| 1 | `internal/gateway/qa_promotion_rejudge_test.go` | 8 | 0 |
| 2 | `internal/gateway/qa_promotion_opening_test.go` | 2, of 42 parallel subtests each racing the dial against the promotion | 0 |
| 3 | `internal/gateway/qa_promotion_queued_test.go` | 4 | 0 |

### QA adjudications

- QA finding 1 (Task 1): requirement needing an ID — "a promotion appends nothing" — already VTT-036; the two head-checking tests cite it and are in its evidence.
- QA finding 2 (Task 1): requirement QA offered for an ID should the owner want it pinned separately — a participant controlling only a non-party actor watches nobody after a promotion — held by VTT-340's "or nobody when they control none" with VTT-181; no row. (`cfc217d`'s message names only VTT-036.)
- QA finding 3 (Task 1): spec gap — SPEC-015, SPEC-009 and SPEC-011 still describe the old behaviour — the specification step's.
- QA finding 4 (Task 2): lint — `seatPromote` in `seat_internal_test.go` lacked `t.Helper()` — fixed before `ce5b234`.
- QA finding 5 (Task 2): spec ambiguity — item 4's "before a `ViewpointChanged` naming nobody, no entry" is underdetermined when the promotion lands after the open, since VTT-339 lets the log's catch-up arrive first — item 4 is the window before the pump starts, and after the open it is item 1; no change.
- QA finding 6 (Task 2): requirements needing an ID — "after it a narration sends nothing" and "whatever the timing the connection ends where the new role's rules say" — VTT-341 and VTT-340; no rows.
- QA finding 7 (Task 3): spec gap — VTT-387's boundary is not observable from outside, since a client cannot tell when the pump re-resolved — the QA tests assert the weaker outcome, and T4, T6 and T6b hold the row from inside.
- QA finding 8 (Task 3): requirements needing an ID — a frame already queued before the re-resolution is still written; fewer than the whole log is sent to a slow reader — the first is VTT-387's carve-out, the second the debt recipe's outcome; no rows.

QA's tests were changed after their reports, each change named in its
task's commit: in Task 1, one fixture given a lower-id party member so its
default differs from its perch, and a citation dropped; in Task 2, two
citations narrowed and an event-id comparison replaced by `proto.Equal`; in
Task 3, the entry count cut from 3000 to 1500, citations narrowed, the
promote read deadline raised to 10s, and `quiet` made to fail only on an
entry or a switch.

## Phase 4b

One reviewer per commit, each re-confirming its own findings' fixes once.

| Commit | Must | What else it found |
|---|---|---|
| `2180ac8` | VTT-387 false under the planned design: the role comparison sat in `admit`, before `onEntry`'s second lookup | stale wording in the plan and the ticket, a half-tested VTT-388, VTT-387 naming the connection rather than the pump; the plan's last section lists them |
| `cfc217d` | VTT-389 duplicated VTT-036 | `rejudge`'s comment overclaimed; T7's double raise let a first-match mutant pass; two QA tests in VTT-388's evidence did not hold it |
| `ce5b234` | none | no test pinned the announce-then-re-judge order; QA citations claimed VTT-387 and VTT-368 |
| `3ddf8e7` | QA tests listed for VTT-387 stayed green without Task 3 | the QA promote deadline equalled the presence send budget; an equivalent mutant in `rejudge`'s guard; the QA `quiet` rejected a presence frame |

Every finding was applied, except one Nit of the review of `2180ac8` (N6),
which the plan's last section records with its reason.

## The verifier's gaps, G1-G6

- G1, item 4 had no first entry to point at: the item was reworded, and T4 asserts the first `ViewpointChanged` names x1 and the next nobody, with no entry before it.
- G2, nothing pinned the queued entries the problem named: VTT-387, item 10, T6 and T6b.
- G3, item 5 rested on rows that did not cover it: VTT-388, T5 and T5b.
- G4, an idle opening was untested: item 9 and T4b.
- G5, `presence_internal_test.go` was unlisted: listed before `2180ac8`.
- G6, item 6's grep keeps the debt's present-tense sentence: kept, with the past tense in the section's heading.

## The deliberate breaks

Each restored from a saved copy and checked with `filecmp` or `cmp`. Counts
are runs at `-cpu 1,2`.

| Break | Measured on | Red | Green |
|---|---|---|---|
| no `rejudge` call in `handlePromotion` | `cfc217d`, and again on `3ddf8e7` | T1, T2, T3 | T5, T5b, T7 |
| `presenceRegistry.rejudge` stops at the first matching connection | `cfc217d` | T3; T7 50 of 50 | |
| the pump's rejudge arm moves the seat to the default and announces | `cfc217d` | T5, T5b, QA's keep-a-perch test, every run | |
| the registry's send without `default` | `cfc217d` | T7, on its 1s bound | |
| `handlePromotion` appends a narration | `cfc217d` | six QA tests | |
| the pump's start re-judge deleted | `ce5b234` | T4, T4b, 6 of 6 | |
| the re-judge before the opening announcement | `ce5b234` | T4, T4b, 6 of 6 | |
| a silent re-judge, then one announcement | `ce5b234` | T4, T4b, 6 of 6 | |
| `onEntry`'s role comparison deleted | `3ddf8e7` | T6, T6b, 6 of 6 | T4 |
| the comparison before `onEntry`'s second lookup | `3ddf8e7` | T6b, 6 of 6 | T6 |
| `rejudge`'s `==` flipped | `3ddf8e7` | T6, T6b | |
| the start re-judge deleted, Task 3 in place | `3ddf8e7` | T4b, 6 of 6 | T4 |
| the comparison and the start re-judge both deleted | `3ddf8e7`, after `task check` only | T4, T4b, T6, T6b, 6 of 6 | |
| Task 1's signal and Task 3's comparison both deleted | `3ddf8e7` | QA's two queued move tests, 6 of 6 | |

"Measured on" names the tree the break was applied to; the rows of `3ddf8e7`
were measured on its staged tree before the commit and, where the debt
entry cites them, again after `task check`.

T4 is held by both the start re-judge and the per-entry comparison, so it
reds only with both gone; T4b holds the start alone. QA's opening race
tests redded the start's deletion in 3 and 1 of 10 runs: they land in the
window only sometimes. QA's queued tests are held by either Task 1's signal
or Task 3's comparison, which is why they are not VTT-387's evidence.

## Rule 9: how MapTool does this

It does not. A MapTool client's role is set once, at the handshake, from the
server's connection-successful message (`ClientHandshake`), and
`Players.setRole`'s doc says it "will not change the role of a currently
logged in player, they will have to log out and log back in for it to take
effect". MapTool has two roles, GM and PLAYER, and its view is a client-side
filter over a campaign every client holds. Nothing was borrowed.

## Figures

Measured on `3ddf8e7` by `task check`, 2026-10-18; the run's log is not
committed.

- Coverage: `internal/gateway` 94.0% against its 93.5 floor, `cmd/vtt` 87.0%
  against 85.0; 21 packages at or above their floors, 11 of them clearing it
  by more than 1.0 point.
- Mutation: 15 packages, zero unadjudicated survivors. Only
  `internal/gateway` was mutated afresh: one survivor, adjudicated, the
  `authz.go` row of `tools/mutation-equivalents.txt`. The other 14 packages'
  verdicts, and their 7 timed-out mutants, were reused as unchanged. The
  client's figures are its stored report, checked rather than re-mutated:
  2944 mutants, 2868 killed, 9 timed out, 67 survivors adjudicated.
- `check:race`: `internal/gateway` 78.5 s, `cmd/vtt` 172.2 s.
- `check:requirements-chain`: 388 rows; `check:new-prose` and
  `check:comments` clean.
- Free disk at the end: 22.1 GiB.

Comment shares after `--write-ledger`, which only lowered rows:
`internal/gateway/server.go` 22.3 (from 22.4), `seat.go` 16.2 (from 16.5),
`presence.go` 15.6 (from 15.9), `seat_internal_test.go` 3.0 (from 3.7),
`presence_internal_test.go` 29.3 (from 30.5), `watch_test.go` 1.7 (from
2.0); the three QA files 0.0.

QA's queued tests, measured by the writer on `3ddf8e7`'s staged tree, 20
runs at `-cpu 1,2` for each size: before the switch at most 461 entries were
sent with 3000 in the log (mean 267), 465 with 600 and 464 with 1500. The
bound is the subscription's and the writer's buffers plus the socket's, not
the log's length; 600 left too little room, so 1500 was kept. Their `-race`
run fell from 21.6s and 614 MB of resident memory to 11.9s and 442 MB.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D2: the signal's receive end a `pumpState` field | a `pump` parameter | only `pump`'s select reads it, so a field would be state no method uses |
| D6: a world seat never compared, by a `wake == nil` guard | `openSeat` sets `judgedAs` for every seat and the guard is gone | the guard's `||` mutant survived the package, an equivalent; a world seat's participant cannot be promoted, so its role equals `judgedAs` |
| T5b perches on c2 | opens with `log=c2` | the same seat state, one command fewer |

## What could not be established

- Whether an entry the pump judged before it read the new role, or a frame already queued for the writer, is ever sent after `handlePromotion` returns. The DM's goroutine and the promoted connection's pump share no order; VTT-387 is written to that boundary and no test can order the two from outside.
- An unexplained failure of the `internal/gateway` package, which the reading reviewer of `ce5b234` saw once in 13 runs of its own and kept no output of; the writer's 8 further runs were green. A failure of the same pre-push step, `cross-layer`, hit the push of `main` at `526e090` earlier the same day, before this ticket, also with its test's name lost. Neither is identified.
- How long a promotion's result can wait. `presenceRegistry.rejudge` takes `mu`, which `joinAndSend` holds across its snapshot send, up to `presenceSendBudget` when the joiner's buffer is full; `announcePromotion` already waits that long per wedged connection.

## What was deliberately left out, and where it went

- Revocation by `vtt revoke`, which runs in another process and reaches no in-process signal: left as it is; VTT-352 holds that the next append closes the connection, and VTT-032 that the next command is refused (the ticket's "What could not be established").
- SPEC-015, SPEC-009 and SPEC-011: the specification step after this commit.
