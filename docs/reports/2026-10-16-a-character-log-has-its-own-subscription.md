# A character log has its own subscription: the change

**Ticket:** `docs/superpowers/specs/2026-10-16-a-character-log-has-its-own-subscription-design.md`,
verified by an agent that did not write it (passes with gaps G1-G3). G1, a
file missing from "What it touches", was fixed by the writer before
`4341b13` committed it.
**Plan:** `docs/superpowers/plans/2026-10-16-a-character-log-has-its-own-subscription.md`,
written by the verifier. Its sign-off line, the MapTool join-order paragraph
under "Rule 9" and the line mapping rows 1-9 to VTT-329..337 were added by
the ticket's writer before `4341b13`.
**The owner's rulings:**
- 2026-10-15, for the arc: four tickets in order; one active character at a
  time; no dual path, no migration, no log ever regenerated.
- 2026-10-16: the arc's third ticket is split in two. This is the first
  half, the store and the campaign only; the second moves a seat onto the
  subscription.
- 2026-10-16, at sign-off: after asking how MapTool handles a client joining
  mid-move, the owner signed the plan with D6's hold ("Ja, bygg med väntan"),
  and every review finding is fixed and each task committed without asking
  unless a finding is a design question ("Ja, samma regel").

**Last code commit:** `20f9ff7`, on `503c53e`, `main` at the time. Every
code reference below is to its tree.

## The period, in commits

    git log --oneline 503c53e..20f9ff7

    20f9ff7 The mutation gate and the coverage floors hold the character subscriptions
    4f11ec9 The campaign delivers each character entry after the live apply
    22f676d The store follows one character log from its own cursor
    4341b13 A character log has its own subscription: the ticket, the plan and VTT-329..337

    git diff --stat 503c53e..20f9ff7

    14 files changed, 3085 insertions(+), 42 deletions(-)

Most of the insertions are tests: the two QA files are about 1,750 lines.

## What the code did on `503c53e`

The store served one subscription, to the world's `events`:
`SubscribeWithNoProgressTimeout` read the history after a cursor under the
store lock, queued it, registered a `subscriber` in `s.subs`, and
`notifyLocked` queued every later world event for every subscriber, deduped
by `env.Sequence <= sub.lastSeq`. `campaign.Campaign` exposed that one
subscription and notified world events only. Through the campaign, a
character log could only be read whole, by `Campaign.CharacterLog`, at every
call, and nothing could follow one.

## Done looks like, answered

1. **[x]** `TestACharacterSubscriptionDeliversItsLogFromTheCursorThenLive`
   (VTT-329, VTT-331) writes three entries to each of two logs, subscribes to
   one after 1, and reads head 3, entries 2 and 3, one live entry, and
   nothing more. It does not compile on `503c53e`.
2. **[x]** `TestACharacterSubscriptionCarriesNoOtherLog` (VTT-330) follows
   `a`, `b` and the world through three batches whose world event and two
   entries share a sequence, notifying `b` first in the second batch, and
   each channel receives exactly its own. QA's
   `TestQASubCarriesNoOtherLogAndNoWorldEvent` holds the same rule for a leak
   from each log placed ahead of some receiver's last sequence; a leak from
   `qa-a` into `qa-b` lands behind `qa-b`'s and passes it, and the first
   test's batch order is what catches that direction.
3. **[x]** `TestACharacterSubscriptionRacingAnAppendSeesEachEntryOnce`
   (VTT-329) subscribes deterministically between an entry's write and its
   notify, then mid-way through 200 concurrent appends, and every entry
   arrives once and in order. QA's `TestQASubRacingAppendsDeliverEachEntryOnce`
   runs nine racers over four rounds.
4. **[x]** `TestACharacterSubscriptionWaitsForALogThatHasNoEntries` (VTT-331,
   VTT-329): head 0 on an actor with no entries, nothing, then entry 1 when
   written; and head 2 at a log's last entry 2, nothing, then entry 3.
5. **[x]** `TestACharacterSubscriptionCarriesWhatTheFanOutWrites` (VTT-329)
   subscribes to the hero's log after 2, appends a narration and a batch whose
   goblin move writes two entries from one envelope, and receives exactly
   what `CharacterLog` reads after 2, `proto.Equal` and in order.
   `TestAFailedAppendNotifiesNoCharacter` (VTT-332) fails an append with a
   trigger on `character_events`: nothing arrives, and the next append's
   entries arrive numbered from the head plus 1.
6. **[x]** `TestAPoisonedCampaignRefusesACharacterSubscription` (VTT-334)
   and `TestClosingTheCampaignEndsCharacterSubscriptions` (VTT-335), the
   latter over a log with entries and an actor with none.
7. **[~]** The ticket says a subscription "that is not read closes after its
   no-progress budget". What closes it is a waiting entry left unaccepted by
   the hand-off channel for the budget, as for the world's subscriber: QA
   probed a subscription with buffer 8 holding 3 unread entries open through
   ten budgets. VTT-336 is worded that way, and
   `TestAStalledCharacterSubscriberIsDroppedAfterTheNoProgressTimeout`
   (VTT-336, VTT-337, buffer 1, four entries) and QA's
   `TestQASubStalledSubscriptionIsClosedAfterItsBudget` hold it.
8. **[x]** `git diff --stat 503c53e..20f9ff7 -- internal/gateway scenarios`
   prints nothing.
9. **[x]** `task check` is green on `20f9ff7`, whole, with
   `internal/store` and `internal/campaign` at zero mutation survivors and the
   store's and the campaign's floors raised, none lowered.

## What the rules became

The tests that hold each row are in `docs/requirements.md`.

- "History then live, each once and in order": **VTT-329**.
- "No entry of another log and no world event": **VTT-330**.
- "The catch-up head": **VTT-331**.
- "Persisted and applied, and a failed append reaches none": split by the
  sort into **VTT-332** (a failed append delivers nothing) and **VTT-333**
  (only after persisted and applied), the latter `**READING — Phase 4b**`.
- "A poisoned campaign refuses, and closing ends every one": **VTT-334** and
  **VTT-335**.
- "Dropped after its no-progress budget, and no other reader affected":
  **VTT-336** and **VTT-337**, which fail apart: a reaper that drops the
  actor's whole registry keeps the first and breaks the second.

VTT-333 is held by Task 2's review, which read every path through
`appendLocked`: a refusal, a validation failure, a failed fan-out and a
failed store write persist and deliver nothing; a failed live apply poisons
and returns before either notify loop; success notifies the world's
envelopes, then the entries in the fan-out's order, both after the live
apply. `Campaign.SubscribeCharacter`'s hold of `c.mu` makes the poisoned
check and the catch-up one step, and keeps a catch-up out of the window
between the store's commit and the live apply: without it, the reviewer's
probe, a subscription whose check passed before a poisoning append and whose
catch-up was delayed 100 ms past it, queued that append's entry. The hold
therefore carries VTT-332's poisoning case too, and the doc says so.

## The sort

Accepted: nine, as VTT-329..337. Refused, one line each:
- "The campaign delivers what the fan-out writes" (item 5): VTT-329 at the
  campaign; its test is in that row.
- Items 8 and 9: a scope bound and the gate.
- D3's and D5's shapes (one constructor, one notify loop, no key deleted):
  how, not what.
- D4's zero-sequence guard: how; the world's `Notify` has no row for it
  either, and `TestNotifyCharacterIgnoresZeroSequence` holds it.
- D8, any actor id: a consequence of VTT-329 and VTT-331.
- Errors after `Close`: the store's existing contract,
  `TestOperationsFailAfterClose`.
- From QA, after Task 1: the world subscription carries no character entry
  (the world's subscription has no rows, and this plan adds none); the
  store's own Close (VTT-335's store half, cited there); a never-persisted
  envelope at the store (VTT-332's store half, cited there); the default
  budget (how, D4); an idle subscription never closed (a corollary of
  VTT-336's "waiting entry").
- From QA, after Task 2: no entry at or before the cursor (VTT-329's "after
  its cursor", red under QA's injection and under every exact-list test);
  an error after a closed campaign and a failed write leaving the campaign
  unpoisoned (existing contracts, `TestOperationsFailAfterClose` and
  VTT-325).

## Phase 4a

Run once per code task, each agent given the rows, the ticket, SPEC-021 and
the exported surface, and none of the source, the tests, the diff or the
plan. The tree was hashed before and after each; each agent added one file
and changed nothing else. Both files are committed with their task.

- **Task 1** (the store): 14 tests in
  `internal/store/qa_character_subscribe_test.go`; QA reports 38
  injections, every one red, and no behaviour defect.
- **Task 2** (the campaign): 13 tests in
  `internal/campaign/qa_character_subscribe_test.go`, every injection red, no
  behaviour defect. QA reached the poisoning branch from outside, deleting
  the head world row on a second handle and then appending a narration that
  validates at head plus 1 and is stored at head, and saw the poisoning
  append's character rows committed and nothing delivered.

### QA adjudications

Task 1:
- Two `thelper` lint findings in QA's helpers: fixed with `t.Helper()`.
- "Unaccepted" (VTT-336) against the ticket's "not read" (item 7): spec
  ambiguity in the ticket's sentence; the row's word is right, item 7 is
  `[~]` above, and SPEC-021 defines accepted.
- A negative cursor is accepted unclamped, as the world's is: no change;
  SPEC-021 says the cursor is taken as the world's subscription takes it.
- A subscription asked of a closed store is refused with an error: the
  store's existing contract.
- An empty actor id, a negative buffer, a second unsubscribe, a notify with
  no subscriber: behaviour no rule names; no change.
- An entry notified for another actor, or out of order: the campaign is the
  one caller (D7); no change.
- "Delays no other" measured as zero fake time under synctest: accepted.

Task 2:
- QA's own fixture assumed the opening's `SessionStarted` keeps the session
  id it was given; it carries the one the campaign stamped: neither defect
  nor ambiguity.
- VTT-333's order on a successful append cannot be observed: held by the
  review, as registered.
- Poisoning leaves open subscriptions open: as the world's; no new rule.
- `SubscribeCharacter` after `Close` returns the store's error, as
  `Subscribe` does: no change; QA's assertion-free tail now asserts it
  (review N4).
- Entries waiting when a subscription closes are dropped, as the world's:
  no change.
- A dropped frame set through a subscription: untested, by construction
  (below).
- Two wall-clock tests moved into synctest (deviations).

## The verification and Phase 4b

The verifier ran its probes in a scratch copy of `503c53e`: a shared
registry reds only the no-other-log test; `NotifyCharacter` after a `Close`
that sets the registry to nil panics; `fanout.go` would rise from its 14.2%
ceiling to 14.3% with a two-line doc, so `SubscribeCharacter` went in
`campaign.go`; moving the notify before the live apply reds nothing.

One reviewer per code task:

- **Task 1:** one Must, that QA's racing test waited for entry n even when
  its cursor was n and timed out on correct code at `-cpu=1` (8 time-outs
  in four runs, each a racer whose cursor was 120, the log's head); fixed by looping on the expected count, then green 5 of 5
  at `-cpu=1`. Three Shoulds: the break record predated QA's file; VTT-335,
  the campaign's row, was filled in Task 1 with one test that is not
  evidence, and went back to OPEN; one test's teeth hung on an unexplained
  line, moot once the zero-sequence guard moved. Five Nits, among them a
  leak direction neither no-other-log test saw, the guard given one home in
  `enqueueAll`, and four comments made warnings.
- **Task 2:** no code defect, and the reading of VTT-333 above. One Must:
  the plan's prediction for break 5 no longer held (below). One Should, that
  the doc name VTT-332. Six Nits: a QA test cited for VTT-336 that cannot
  fail when VTT-336 breaks, retagged VTT-329; an unbounded fixture loop; an
  assertion-free tail; the doc line; two notes for the specification.
- **Task 3** was read by its author: two numbers, each shown by
  `check:coverage`'s own run.

## Rule 9: how MapTool does this

MapTool has no per-character stream, cursor or numbering per log; a
character subscription reuses the store's subscriber, which already follows
MapTool's per-connection queue. `ServerMessageHandler.handleMessage`
schedules every `PUT_TOKEN_MSG`'s apply on Swing's `EventQueue` and sends it
to the other clients at once, and on a join
`MapToolServer.addRemoteConnection` sends a joining client `campaign.toDto()` after `installConnection` has added it to
the router, under no lock shared with the `EventQueue`; a move made during
the join can stand wrong on that client until it moves again
(`~/dev/RPTool/maptool` at `f4b7fef6c`). D6's hold refuses that order, and
the store's one lock already refuses a missed or a repeated entry.

## Figures

- `check:mutation`, standalone on `4f11ec9` with the raised floors
  uncommitted in the tree: 32 minutes, 15
  packages, zero unadjudicated survivors; store 0, campaign 0, gateway 1
  (an existing adjudication). The timed-out mutants were all in packages the
  skip cache reused.
- Coverage, `go test -cover` three times each: store 96.3%, campaign 94.2%,
  no spread; `check:coverage` the same.
- `task check` on `20f9ff7`: green in 19 minutes, 01:33 to 01:52 on
  2026-10-17, its mutation stage reusing the standalone run's verdicts for
  the store and the campaign, since nothing in their closure had changed.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Task 1: the history read "through `readCharactersLocked`" | `readCharacterLocked` extracted in `store.go`; `ReadCharacterAfter` and `SubscribeCharacter` share it | One query, one home |
| D4: `NotifyCharacter` ignores Sequence 0 "as `Notify` does" | Both guards removed; `enqueueAll` holds the one guard | Task 1's review (N4): one home and one mutant, killed by both zero-sequence tests |
| D5: one key per actor ever followed | One key per actor id ever followed, and per actor ever sent an entry | `NotifyCharacter` writes the actor's key unguarded, so no guard and no equivalent mutant |
| Task 1: seven breaks | Ten: plus `Close` nilling the registry, and a leak from `a` into `b` and from `b` into `a` | D5's panic had no break of its own; the leak directions were Task 1's review N1 |
| Task 1's break 6, the lock released between the history read and the registration, predicted to red no deterministic test, only the concurrent loop by chance | The implementer's tests stay green; QA's `TestQASubRacingAppendsDeliverEachEntryOnce` reds | QA's racers sample the window; a sample, not a mechanism, so the review still reads the critical section |
| Task 1's done-check: `character_subscribe_test.go` the only new store file | Two, with QA's | QA's tests are committed with their task |
| Task 1's row list: evidence for VTT-329-331, 336, 337 | VTT-335 filled in Task 1, then set back to OPEN until Task 2 | Task 1's review (S2): it is the campaign's row |
| Task 2's break 5, the notify before the live apply, predicted to red nothing (G2) | Reds QA's `TestQACSubAPoisoningAppendDeliversNothingAndRefusesSubscriptions` | QA reached the poisoning branch, where an append persists and then fails its live apply; on a successful append the move stays unobservable, so VTT-333 stays a reading |
| QA's tests as delivered | Task 2's `TestQACSubAStalledReaderIsClosedAndDelaysNoOther` and `TestQACSubASlowReaderWithinItsBudgetIsKept` wrapped in `synctest.Test` | They asserted wall-clock thresholds that machine load can break; three of QA's injections reran red inside the bubble |
| `store.go` comments untouched but for the warning in `Close` | The `busy_timeout` comment in `Open` shortened from four lines to two | Moving the zero-sequence guard out of `Notify` raised the file's comment share to 10.701 against its committed ceiling of 10.7; the comment's history went (below) |
| `tools/comment-ceilings.txt`: rows for the files this ticket touches | Also `internal/gateway/character_log_test.go` 5.5 to 5.4 | `--write-ledger` rewrites every row whose share it recomputes lower; the file is untouched |
| Task 3's break: delete the no-other-log test, one mutation pass | Two passes, the second with QA's `TestQASubCarriesNoOtherLogAndNoWorldEvent` deleted too | The first left QA's test killing whatever mutants there were; the second shows there are none |
| One Phase 4b review per task | Task 3 read by its author | Two numbers, each shown by the gate |

## What could not be established

- **VTT-333's order on a successful append** has no test: every read
  through the campaign waits on `c.mu`, so a notify before the live apply,
  or a catch-up without the hold, is invisible unless the live apply fails.
  It is held by Task 2's review.
- **The mutation gate does not reach VTT-330.** With both of the store's
  no-other-log tests deleted, in a clone, the store's mutation pass still
  reported zero survivors: one registry shared by every log is no mutant
  gremlins makes. The rule is held only by the tests in its row: the store's
  `TestACharacterSubscriptionCarriesNoOtherLog` and
  `TestQASubCarriesNoOtherLogAndNoWorldEvent`, and QA's
  `TestQACSubCarriesItsOwnLogAndNothingElse` at the campaign. Task 1's break
  1, re-run on `20f9ff7` by the report's review, reds all three.
- **G3**, whether a world subscriber and a character subscriber should share
  one liveness budget per connection, is the second half's: here each
  subscription has its own.
- **D6's cost runs both ways** and was not measured: a subscription waits
  for an append in progress, its fan-out included, and an append, `State`
  and `CharacterLog` wait for a subscription's catch-up, read and
  unmarshalled under `c.mu`, and for up to the store's 5 s `busy_timeout`
  while another handle writes the file.
- **A dropped frame set** (VTT-320) reaches no subscriber by construction:
  the notify loop walks what `AppendBatch` wrote. No test drives a drop
  through a subscription.
- The implementer's own VTT-331 tests cannot tell the cursor from the log's
  head when nothing is queued; QA's past-the-end cursor cases can.

## What was deliberately left out, and where it went

- No seat follows a character log; `set_viewpoint`, the viewpoint frame, the
  move gate and the projection are unchanged: the second half of the third
  ticket.
- The world's subscription has no specification and no register row, and
  this ticket records none.
- Poisoning leaves open character subscriptions open and silent, as it does
  the world's (QA, Task 2): no new rule.
- `subscribe.go`'s history comments stay where they are: the standing sweep's.
- `check:coverage` names eleven other packages whose floors could rise: not
  this ticket's.
- `SubscribeCharacter`, `NotifyCharacter`, the shared constructor and notify
  loop, the registry per actor, the hold and the order of delivery go into
  SPEC-021's "Following a log", at the specification step after this report.

## History the code no longer carries

Removed from comments under rule 10 in this period, and recorded here:

- `store.Open`'s `busy_timeout` comment said a concurrent `SQLITE_BUSY` from
  another handle on the same file, intra-process (a `Store` and an
  `identity.DB` open on one campaign) or cross-process, retries for up to
  5 s, and that the setting was a ledgered carry-forward from P6 Task 4's
  review.
