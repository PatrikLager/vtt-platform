# A character log can be subscribed to, from its own cursor: implementation plan

- **Ticket:**
  `docs/superpowers/specs/2026-10-16-a-character-log-has-its-own-subscription-design.md`,
  untracked at the time of verification.
- **Verdict:** PASSES WITH GAPS (G1-G3 below). Verified by an agent that did
  not write the ticket. The probes behind the verdict ran in a scratch copy of
  `503c53e` and are not committed.
- **Signed off:** by the owner on 2026-10-16, with D6's hold, after the
  owner asked how MapTool handles a client joining mid-move (Rule 9, below).
- **Base:** main `503c53e`.
- **Branch:** `feat/a-character-log-has-its-own-subscription`.

Written by the agent that verified the ticket. Before writing it, that agent
read:

- `internal/store/store.go` and `subscribe.go`, and every store test file;
- `internal/campaign/campaign.go`, `fanout.go` and `export_test.go`, and
  `character_log_test.go`, `poison_internal_test.go` and `campaign_test.go`;
- `internal/gateway/server.go`'s `serve`, for how a subscription is consumed;
- SPEC-007's catch-up head, SPEC-011, SPEC-015, SPEC-016, SPEC-019 and
  SPEC-021;
- the arc design §3-§5, the old arc plan's Tasks 6 and 10, and ticket 2's
  plan and report;
- `.go-arch-lint.yml`, `check-mutation.py`'s `PACKAGES`, the `check:coverage`
  task, `tools/check-comments.py`'s rules, `coverage-thresholds.txt` and
  `comment-ceilings.txt`.

## Constraints that bind every task

These are named here, not restated.

- **CLAUDE.md rules:**
  - 2: `task check` is never weakened. No floor is lowered and no ceiling
    raised.
  - 3: additive contract. This plan changes no `.proto`.
  - 4: one fold. Nothing here applies an event; the subscription delivers
    entries the fan-out already folded.
  - 5: no game-system vocabulary. Fixtures reuse `character_log_test.go`'s
    hall, hero and goblin.
  - 8: citations name durable things.
  - 9: RPTool first (answered below).
  - 10: a comment is a warning or a pointer. History goes to the report.
- **The `dev-cycle` package's phases.** Tests first, a deliberate break for
  every new check, Phase 4a QA, a Phase 4b review before every commit, the
  report, then the specification. They live in the package and are not
  repeated here. One Phase 4b review and one commit per task.
- **The owner's rulings:**
  - 2026-10-15, for the arc: one active character at a time; no dual path,
    no flag, no migration; no log is ever regenerated; old solutions are
    deleted outright.
  - 2026-10-16: ticket 3 is split. This is 3a, the store and the campaign
    only. Nothing in `internal/gateway` changes. 3b moves a seat onto the
    subscription, generalises `set_viewpoint`, adds the viewpoint frame,
    moves the move gate and deletes the projection.
- **SPEC-008:** ids come from `requirement-id`, after sign-off.
- **SPEC-010:**
  - A block of more than six comment lines may gain no line, and a block that
    moves counts as added lines. `subscribe.go` holds four such blocks:
    `SubscriberNoProgressTimeout`, `Subscribe`, `SubscribeWithNoProgressTimeout`
    and the one above its `unsubscribe` closure. New code goes beside them,
    never inside them, and none of them moves.
  - Run `tools/check-comments.py` against both `main` and `HEAD` before every
    commit. Ceilings only go down, and a row already committed on the branch
    is never deleted and regenerated.
- **SPEC-020:** an adjudication names the code it excuses.
- **SPEC-015's** grep for `c.log.Append` stays true. This plan adds no append
  call.
- **Arch:** `store` may depend on `contract` and `store` only; `campaign` on
  `contract`, `store`, `engine`, `campaign`, `eventgen` and `perceive`. Test
  files are held to the same lists.
- **Operational lessons in the project memory, binding here:**
  - Review packages are built with `git diff HEAD`.
  - Never stash while a reviewer reads; take before-measurements from
    `git show 503c53e:<file>`.
  - Settle the review before starting the gate.
  - Read the load with `os.getloadavg()` and launch `task check` with
    `start_new_session=True`.
  - Run `check:new-prose` by hand before the last gate run.
  - Verify what landed after every commit. Stage and commit in separate
    calls.
  - Hash the tree before and after a QA or review agent runs.

## Rule 9: how MapTool solves this

Checked against `~/dev/RPTool/maptool` at `f4b7fef6c`.

**Borrowed, already: the per-connection queue.**

- `AbstractConnection` holds one unbounded `LinkedBlockingQueue<byte[]>`
  `outQueue` per connection, drained by its own sender.
- `store.subscriber` already follows that shape, and so does its no-progress
  budget (`SubscriberNoProgressTimeout`'s comment names
  `net.rptools.clientserver`). A character subscription reuses the same
  `subscriber`, so it borrows the same queue.

**Where MapTool does not help: a stream per character.**

- `ServerMessageHandler.sendToClients` and `sendToAllClients` call
  `MapToolServer.broadcastMessage`, which sends one message to every client
  but the excluded one.
- A client receives the whole campaign on connect and has no cursor to resume
  from.
- So MapTool has no per-character stream, no catch-up from a cursor, and no
  numbering per log. The ticket's sentence on this holds.

**Refused, as in tickets 1 and 2:** the distribution model.

**Refused: MapTool's order on a join (D6).**

- `ServerMessageHandler.handleMessage` schedules a `PUT_TOKEN_MSG` onto
  Swing's `EventQueue.invokeLater` and calls `sendToClients` at once, so
  every other client is sent the move before the server's campaign holds
  it.
- A joining client is added to the router first (`installConnection`, then
  `router.addConnection`), and then sent `campaign.toDto()` in a
  `SetCampaignMsg` (`addRemoteConnection`). That runs when the handshake
  completes (`ServerHandshake.sendConnectionSuccessful`), ordinarily on the
  thread that read the client's last handshake message, under no lock
  shared with the `EventQueue`.
- A move made during that join can reach the client before a snapshot that
  predates it, or go out before the client was added while the snapshot was
  taken before the server applied it. Either way the token stands wrong on
  that client until it moves again.
- Here, the store's lock already rules out a missed or a repeated entry
  (rows 1 and 3). D6's hold refuses the other half: an entry never leaves
  before the append that wrote it is applied.

## Decisions this plan makes

Each decision names what forced it.

**D1. A character subscription delivers `*vttv1.Envelope`, each entry's
envelope stamped with the log's own sequence, on the channel type the world's
subscription uses.**

- It does not deliver `world_seq`.

*Forced by:*

- Arc §5 and SPEC-021: `world_seq` is never folded and never sent on the
  wire.
- `serve` consumes `<-chan *vttv1.Envelope` from
  `SubscribeWithNoProgressTimeout`, so 3b can swap a seat's subscription
  without changing a type.
- `subscriber`, its queue, its pump and its dedupe are reused unchanged.

**D2. The store keeps one registry per actor, `Store.chars
map[string][]*subscriber`, beside `s.subs`.**

*Forced by:*

- The ticket's point on the dedupe: `enqueue` drops `env.Sequence <=
  sub.lastSeq`, which only compares numbers of one log.
- Probe: one shared registry reds only the "no other log" test (item 2).

**D3. One constructor and one notify loop serve both subscriptions.**

- One function refuses a negative buffer, defaults a non-positive budget to
  `SubscriberNoProgressTimeout`, preloads the history, sets `lastSeq` and
  starts the pump. `SubscribeWithNoProgressTimeout` and `SubscribeCharacter`
  both call it.
- One function enqueues an envelope to a slice of subscribers and returns the
  slice without its stopped ones. `notifyLocked` and `NotifyCharacter` both
  call it.
- The world's behaviour is unchanged, and its tests pass unchanged.
- The `unsubscribe` closure and its comment stay in
  `SubscribeWithNoProgressTimeout`. `SubscribeCharacter` writes its own
  three-line closure.

*Forced by:*

- Item 7: a stalled character subscriber is let go "as the world's is".
- Gremlins measures one copy. The world's `TestSubscribeAcceptsZeroBuffer`,
  `TestSubscribeRejectsNegativeBuffer` and
  `TestASubscriberThatKeepsConsumingIsNeverDropped` then kill the shared
  boundary mutants. A second copy would need tests of its own.
- SPEC-010: moving the closure would move its 13-line comment block.

**D4. Each layer has one `SubscribeCharacter(actorID string, afterSeq int64,
buffer int, noProgress time.Duration)`, with no twin lacking the budget. The
store adds `NotifyCharacter(actorID string, env *vttv1.Envelope)`.**

- A non-positive budget means `SubscriberNoProgressTimeout`.
- `NotifyCharacter` ignores an envelope of `Sequence` 0, as `Notify` does.

*Forced by:*

- The one production caller to come, 3b's `serve`, passes its budget, as it
  already does to `SubscribeWithNoProgressTimeout`. A twin without the budget
  would have test callers only. *Partly a preference.*
- `Notify`'s contract: an envelope never persisted reaches nobody. A failed
  `AppendBatch` resets every entry to 0, and the cursor is accepted
  unclamped, so a subscriber with a negative cursor would otherwise take a
  0.

**D5. `Open` makes the registry, and `Close` stops every subscriber in it and
empties it with `clear`, never setting it to nil. No key is ever deleted.**

- A stopped subscriber stays in its actor's slice until that actor's next
  notify or `Close`, as the world's lazy compaction already works
  (`SubscribeWithNoProgressTimeout`'s `unsubscribe` comment).

*Forced by:*

- Probe: `NotifyCharacter` after a `Close` that sets the map to nil panics
  ("assignment to entry in nil map"). `TestOperationsFailAfterClose` pins
  errors, not panics.
- A guard that deletes an emptied key has an equivalent mutant: `> 0` turned
  into `>= 0` leaves an empty slice, which nothing observes. SPEC-020 would
  need an adjudication for it.
- Cost:
  - one key per actor ever followed, which the party bounds;
  - on a paused log, one stopped subscriber per unsubscribe until `Close`.
    That is a struct of three channels.

**D6. `Campaign.SubscribeCharacter` holds `c.mu` across the store's
subscribe, and refuses with `errPoisoned` while poisoned.**

- The lock order is `c.mu`, then the store's `mu`, as in `appendLocked`.
- `Subscribe` keeps its early unlock. Changing it is not this ticket's.

*Forced by:*

- The ticket's rule that an entry reaches a subscriber only after the append
  that wrote it is persisted and applied. Without the hold, a catch-up between
  the store's commit and the live apply queues the entry early.
- No test can see that difference. Probe: moving the notify before the live
  apply reds nothing, since every read through the campaign takes `c.mu`
  (G2). The hold makes the rule true by construction rather than by an
  ordering nobody can observe.
- Cost: a subscription waits for an append in progress, its fan-out included
  (SPEC-021's cost).

**D7. `appendLocked` notifies each entry after the world's envelopes, after
the live apply, in the order `fanOutLocked` returned them. `SubscribeCharacter`
lives in `campaign.go`, beside `Subscribe`.**

*Forced by:*

- The per-actor dedupe drops an entry notified after a later entry of the
  same log, so the order within an actor matters.
- `AppendBatch` stamps each entry's envelope in place (`Store.appendLocked`:
  `e.Env.Sequence = seq`), so `entries` holds the persisted sequences. Probe:
  every delivered envelope is `proto.Equal` to what `CharacterLog` reads.
- `fanout.go` sits at its ceiling of 14.2%. A two-line doc there puts it at
  14.3%, which `check:comments` refuses. `campaign.go` has room under 30.4.
- The order between the world's notify and the entries' is not a rule. They
  go to different subscriptions, and SPEC-011 promises nothing about arrival
  timing across kinds.

**D8. The campaign subscribes any actor id, without consulting the world.**

*Forced by:*

- Item 4: a log with no entries can be followed.
- SPEC-021: a campaign written before the table existed has party members
  with no log until their next append.
- Who may follow which log is authorisation, which is SPEC-013's, in 3b.

**D9. How the tests force their cases.**

- **Item 3:** deterministically at the window. Persist with `AppendBatch`,
  subscribe, then `NotifyCharacter`. A concurrent loop runs beside it as a
  `-race` smoke. *Forced by* the lesson that a green sample is not evidence
  of absence: a concurrent loop samples interleavings, and the window is the
  mechanism.
- **Item 4:** adds a log whose entries all sit at or below the cursor (cursor
  2 of a log of 2): the head is 2, nothing is delivered, and entry 3 arrives
  when it is written. *Forced by* "degenerate fixtures hide mutants": cursor 0
  cannot tell head = cursor from head = 0.
- **Item 5's batch:** `hallWithHero(t, c, 5)`, then one `AppendBatch` of a
  `NarrationAdded` and the goblin's `TokenMoved` from (5,1) to (2,1). Probe:
  the batch writes the narration, then `ActorAdded goblin` and `TokenPlaced
  t-gob`, two entries from one envelope, so order matters. The subscription
  is taken after cursor 2 of a log of 4, so catch-up and live both count.
- **The failed append:** a `BEFORE INSERT ON character_events` trigger with
  `RAISE(ABORT, …)` on a second handle, as
  `TestAFailedFanOutDoesNotAdvanceACharactersMemory` does. After the trigger
  is dropped, the next append's entries arrive, numbered from the head before
  the failure plus 1.
- **Item 6's poison test** is white-box, in `poison_internal_test.go`, and
  sets `c.poisoned` as `TestPoisonedCampaignRejectsAllOperations` does.
- **Item 7** runs in `synctest` with the default budget, as
  `TestAWedgedSubscriberIsDroppedAndOnlyThatSubscriber` does.
- **The store gets its own close test.** `check:coverage` runs `go test
  -cover` without `-coverpkg`, so a package counts its own tests only, and
  `Store.Close`'s new loop would be uncovered by the campaign's test.
- `TestOperationsFailAfterClose` gains two cases: `SubscribeCharacter` (an
  error) and `NotifyCharacter` (returns).

## Gaps carried from the verdict

- **G1. A file the ticket does not name.** "What it touches" item 1 names
  `internal/store/subscribe.go` for the registry and for "`Close` ending
  them". `Store.Close` and the `Store` struct that must hold the registry are
  in `internal/store/store.go` (`grep -n 'func (s \*Store) Close\|subs
  \[\]\*subscriber' internal/store/*.go`). Task 1 names it. Neither what must
  become true nor the size of the work changes: one field, one line in
  `Open`, and one loop in `Close`.
- **G2. "and applied" in the fourth rule has no observation.** Probe:
  notifying the entries before the live apply reds no test, since every read
  through the campaign waits on `c.mu` and the live apply cannot fail
  reachably. The rule also joins two obligations. The sort splits it. The
  "persisted, and a failed append reaches none" half is held by a test, and
  the probe's break of it reds. The "applied" half is recorded as
  `**READING — Phase 4b**`, made true by D6 and D7.
- **G3. The ticket's own.** Whether a world subscriber and a character
  subscriber share one liveness budget per connection. It is 3b's question.
  D4 gives each subscription its own.

## Questions for the owner

None. The candidates were settled by a recorded rule, not by preference:

- D1 by arc §5;
- D6 by the ticket's own rule;
- D8 by item 4 and SPEC-013's layering.

If the owner would rather `SubscribeCharacter` mirror `Subscribe`'s early
unlock, the "and applied" clause should leave the ticket, since nothing would
then hold it.

## Tasks

The tasks are in dependency order. Each one is one Phase 4b review and one
commit.

### Before Task 1: allocate the requirement ids (after sign-off)

The sort, run over the ticket's six rules, accepts nine. Each gets an id from
`requirement-id` and reads `**OPEN — no test yet**` until its test lands,
except the READING row. Rows 1 to 9 are VTT-329 to VTT-337, in order:

| # | Rule, in short | Test that holds it | Task |
|---|---|---|---|
| 1 | a subscription delivers every entry of its log after its cursor, history then live, each once and in order | `TestACharacterSubscriptionDeliversItsLogFromTheCursorThenLive`, `TestACharacterSubscriptionRacingAnAppendSeesEachEntryOnce`, `TestACharacterSubscriptionCarriesWhatTheFanOutWrites` | 1, 2 |
| 2 | it delivers no entry of another log and no world event | `TestACharacterSubscriptionCarriesNoOtherLog` | 1 |
| 3 | its catch-up head is the last entry it queued before it returned, or its cursor when it queued none | `TestACharacterSubscriptionDeliversItsLogFromTheCursorThenLive`, `TestACharacterSubscriptionWaitsForALogThatHasNoEntries` | 1 |
| 4 | an append that fails delivers no entry to any character subscription | `TestAFailedAppendNotifiesNoCharacter` | 2 |
| 5 | an entry is delivered only after the append that wrote it is persisted and applied | `**READING — Phase 4b**` (G2) | 2 |
| 6 | a poisoned campaign refuses a character subscription | `TestAPoisonedCampaignRefusesACharacterSubscription` | 2 |
| 7 | closing the campaign ends every character subscription | `TestClosingTheCampaignEndsCharacterSubscriptions` | 2 |
| 8 | a subscription that leaves a waiting entry unaccepted for its no-progress budget is closed | `TestAStalledCharacterSubscriberIsDroppedAfterTheNoProgressTimeout` | 1 |
| 9 | a subscription that stops reading delays and closes no other subscription | `TestAStalledCharacterSubscriberIsDroppedAfterTheNoProgressTimeout` | 1 |

Rows 8 and 9 fail apart. A reaper that drops the actor's whole registry when
one subscriber stalls keeps row 8 and breaks row 9.

Refused, one line each:

- "The campaign delivers what the fan-out writes" (item 5): row 1 at the
  campaign level. Its test goes into row 1's evidence.
- Items 8 and 9 (no connection changes; the gate holds the new code): a
  scope bound and the gate, not rules on the system.
- D3's and D5's shapes (one constructor, one notify loop, no key deleted):
  how, not what.
- D4's zero-sequence guard: how. `Notify`'s own guard has no row either, and
  the guard is held by its test.
- D8, "any actor id": a consequence of rules 1 and 3, held by
  `TestACharacterSubscriptionWaitsForALogThatHasNoEntries`.
- Errors after `Close`: the store's existing contract
  (`TestOperationsFailAfterClose`), which has no row. Registering it is not
  this ticket's.

The commit carries the ticket, this plan and the rows.

**True when done:** `task check:requirements-chain` is green.
**Gate:** pre-commit hook.

### Task 1: the store follows one character log (items 1-4 and 7; rows 1-3, 8, 9)

**Files:**

- `internal/store/store.go` (G1):
  - `chars` on `Store`;
  - `Open` makes it;
  - `Close` stops every subscriber in it and `clear`s it (D5).
- `internal/store/subscribe.go`:
  - the shared constructor and notify loop (D3);
  - `SubscribeCharacter`, reading the history under the store lock with the
    query `ReadCharacterAfter` uses, through `readCharactersLocked`;
  - `NotifyCharacter` (D4).
  - Each new symbol gets a doc of one or two lines. No existing block gains a
    line.
- `internal/store/character_subscribe_test.go` (new). Each test cites its row
  on the line above it:
  - `TestACharacterSubscriptionDeliversItsLogFromTheCursorThenLive`: two
    actors' logs of three entries each; subscribe to one after 1. The head
    is 3, entries 2 and 3 arrive, then one live entry, then nothing more.
  - `TestACharacterSubscriptionCarriesNoOtherLog`: a subscription to `a` and
    one to the world, then three batches. Each batch is one world event with
    an entry for `a` and one for `b`. Entry sequences coincide across `a` and
    `b`, and coincide with the world's. Each channel gets exactly its own,
    and nothing after.
  - `TestACharacterSubscriptionRacingAnAppendSeesEachEntryOnce`: the window
    case (D9), then a concurrent loop of 200 appends, each notified, with a
    subscription taken mid-loop at buffer 0. Every sequence arrives once, in
    order, and nothing after.
  - `TestACharacterSubscriptionWaitsForALogThatHasNoEntries`: cursor 0 on an
    actor with none (head 0, nothing, then entry 1 when written), and the
    case of a cursor at the log's last entry (D9).
  - `TestAStalledCharacterSubscriberIsDroppedAfterTheNoProgressTimeout`, in
    `synctest`: a wedged and a healthy subscriber on the same log. The
    healthy one reads four entries; the wedged one is closed after the
    budget; the healthy one still receives the next entry.
  - `TestClosingTheStoreEndsCharacterSubscriptions`: a subscription to a log
    with entries and one to an actor with none. Both channels close on
    `Close`.
  - `TestNotifyCharacterIgnoresZeroSequence`, at cursor -1, as
    `TestNotifyIgnoresZeroSequence` does.
- `internal/store/store_failure_test.go`: `TestOperationsFailAfterClose`
  gains its two cases (D9).
- `docs/requirements.md`: evidence for rows 1-3, 8 and 9.
- `tools/comment-ceilings.txt`: `check-comments.py --write-ledger`.
  `subscribe.go` falls from 50.8% to about 43.8% (probe), more than the band
  under its ceiling, so its row is lowered in this commit.

**True when done:**

- The seven new tests pass, and none compiles on `503c53e`.
- Every existing store test passes unchanged: `git diff --stat HEAD --
  internal/store/` names no test file but `store_failure_test.go`, and
  `git status --short internal/store/` shows `character_subscribe_test.go` as
  the only new file.
- `go test -race ./internal/store/ ./internal/campaign/ ./internal/identity/`
  is green.

**Deliberate breaks:** each is made in the working tree, seen red, and
undone. Assert that each edit landed before trusting a green run.

1. Register character subscribers in `s.subs`, and notify entries through
   `notifyLocked`: the no-other-log test reds (probe: only that one).
2. `Close` skips `chars`: the store close test reds (probe).
3. `NotifyCharacter`'s guard inverted so the registry is dropped on every
   notify: the no-other-log, racing and stalled tests red (probe; the
   cursor-then-live test stays green, since it has one live entry).
4. `SubscribeCharacter` replaces the actor's slice instead of appending to
   it: the stalled test reds, because the wedged subscriber is never queued
   anything.
5. The constructor leaves `lastSeq` at the cursor when the history is
   non-empty. Probe: the racing test's window case (a duplicate) and the
   cursor-then-live test's head red, and so do the world's
   `TestNotifyAfterApplyOrdering_NoDuplicateOnRace` and three
   `TestSubscribeHead…` tests.
6. `SubscribeCharacter` reads the history, unlocks, and locks again to
   register: no deterministic test reds. Only the concurrent loop can, by
   chance. Named so the review reads the one critical section.
7. `NotifyCharacter` without its guard: the zero-sequence test reds.

The other direction, on the repaired code: a subscription with nothing new
written stays silent in every test that ends with "nothing after".

**Gate:** pre-commit hook; `go test -race ./internal/store/`;
`check:requirements-chain`; `check-comments.py` against `main` and `HEAD`;
`go-arch-lint check`.

### Task 2: the campaign delivers each entry after the live apply (items 5 and 6; rows 1, 4-7)

**Files:**

- `internal/campaign/campaign.go`:
  - `SubscribeCharacter` beside `Subscribe` (D6), with a doc of at most two
    lines;
  - in `appendLocked`, after the world's notify loop, one loop over
    `entries` calling `c.log.NotifyCharacter(e.ActorID, e.Env)` (D7).
  - The `Campaign` doc block, which lists the calls that fail when poisoned,
    is eight lines and is not edited. SPEC-021 states the refusal (Phase 5).
- `internal/campaign/character_subscribe_test.go` (new). Each test cites its
  row:
  - `TestACharacterSubscriptionCarriesWhatTheFanOutWrites` (D9's batch).
    Once the batch returns, the delivered envelopes are `proto.Equal`, one
    for one and in order, to `CharacterLog`'s entries after the cursor, and
    nothing more arrives.
  - `TestAFailedAppendNotifiesNoCharacter` (D9's trigger). Nothing arrives
    for the failed append. After the trigger is dropped, the next append's
    entries arrive, the first numbered one past the head before the failure.
  - `TestClosingTheCampaignEndsCharacterSubscriptions`: one subscription to
    the hero and one to an actor with no log. Both channels close on
    `c.Close()`.
- `internal/campaign/poison_internal_test.go`:
  `TestAPoisonedCampaignRefusesACharacterSubscription`.
- `docs/requirements.md`: evidence for rows 1 and 4-7; row 5 reads
  `**READING — Phase 4b**`.
- `tools/comment-ceilings.txt`: `--write-ledger` if a share falls more than
  the band. Probe: `campaign.go` measures 29.8% against its 30.4 ceiling,
  inside the band, and `fanout.go` stays at 14.2%.

**True when done:**

- The four new tests pass, and none compiles on `503c53e`.
- Every existing campaign and gateway test passes unchanged.
- `TestRebuildEqualsLiveProperty` is green, and the goldens are
  byte-identical.

**Deliberate breaks:**

1. Delete the notify loop: item 5 reds (probe).
2. Notify the trial's stamped frames inside `fanOutLocked`, before the store
   write: the failed-append test reds (probe: "a failed append delivered
   ConditionApplied goblin dazed").
3. Notify an actor's entries in reverse: item 5 reds, because the dedupe
   drops the earlier ones.
4. Drop the poisoned check: the poison test reds.
5. Move the notify loop before the live apply: no test reds (probe). This is
   row 5's READING, and the review reads where the loop sits.
6. Release `c.mu` before the store's subscribe: no test reds. The same
   reading.

**Gate:** pre-commit hook; `go test -race ./internal/store/
./internal/campaign/`; `go test ./internal/gateway/`;
`check:requirements-chain`; `check-comments.py` against `main` and `HEAD`;
`go-arch-lint check`.

### Task 3: the gates hold the new code (item 9)

**Files:**

- `tools/coverage-thresholds.txt`: the store (94.0) and campaign (92.5)
  floors raised to a little under what they measure, if they rose. Never
  lowered.
- `tools/mutation-equivalents.txt`: survivors in store and campaign are
  killed with tests. D5 removes the one equivalent the verifier foresaw, the
  guard on an emptied key, and D3 leaves the shared boundary mutants to the
  world's tests. An adjudication needs a one-sentence observable (SPEC-020).
  Run the gate's two self-tests first.
- `tools/comment-ceilings.txt`: `--write-ledger` only. Never raise a row by
  hand.

**True when done:**

- `check:coverage`, `check:comments` and `check:requirements-chain` are
  green.
- `check:mutation` is green for store and campaign, with its runtime
  recorded for the report.
- `check:new-prose` has been run by hand and is green.

**Deliberate break:** delete
`TestACharacterSubscriptionCarriesNoOtherLog` and run the store's mutation
pass. Record whether a mutant survives, or whether coverage holds, and say
which.

### Task 4: `task check`, whole

- Check the load with `os.getloadavg()` first.
- Launch with `start_new_session=True`.
- Do not touch the tree during the run.

**True when done:**

- Green, which is item 9.
- Item 8 seen: `git diff --stat main -- internal/gateway scenarios` prints
  nothing.

### Then the process's Phases 4a, 4b and 5

- **Phase 4a** runs on Task 1 and on Task 2, each before its Phase 4b. QA is
  given:
  - the rows the task holds, as allocated;
  - the ticket's "Rules this puts on the system";
  - SPEC-021 as it stands;
  - the exported surface from the uncommitted tree:
    `go doc -all ./internal/store`, plus `go doc ./internal/campaign
    Campaign` for Task 2;
  - the arch lists above. Store tests may import `contract` and `store`
    only; campaign tests may not import `internal/testdb`, `sight` or
    `gateway`.

  QA is not given the diff, the source, the existing tests or this plan.
  Hash the tree before and after it runs. Its passing tests are committed
  with the task, and its adjudications go in the report under their own
  heading, one line each.
- **The report**, `docs/reports/2026-10-16-a-character-log-has-its-own-subscription.md`,
  holds:
  - the period's commits and its last code commit;
  - "Done looks like", answered with the observation for each item;
  - the verifier's probes, true at `503c53e` in a scratch copy:
    - breaks 1-5 of Task 1 and 1, 2, 3 and 5 of Task 2, and which test
      spoke;
    - the nil-map panic behind D5;
    - `fanout.go`'s 14.3% behind D7;
    - `subscribe.go`'s 43.8%;
  - G1-G3 and what became of them;
  - the runtimes from Task 3;
  - the deviations;
  - the QA adjudications.
- **The specification, after the report:**
  - **SPEC-021** gains a paragraph, "Following a log", in the present tense:
    - `Campaign.SubscribeCharacter` and the store's registry per actor;
    - catch-up and registration under one store lock;
    - the dedupe by the log's own sequence, and the catch-up head;
    - live delivery after the live apply, in the fan-out's order;
    - the poisoned refusal under the campaign's mutex, and `Close`;
    - the no-progress budget per subscription;
    - any actor id, a log with no entries included.
  - Its Status names `SubscribeCharacter`, `NotifyCharacter` and the shared
    constructor and notify loop. Its Requirements line gains the new ids.
  - "What this record does not decide" extends its grep to
    `'CharacterLog(\|ReadCharacter\|SubscribeCharacter'`, which must still
    print only `internal/campaign` and `internal/store`.
  - **SPEC-007, SPEC-011, SPEC-015, SPEC-016 and SPEC-019** are re-read.
    Each describes the world's subscription as the gateway consumes it, or
    `Close`'s hold, and item 8 leaves them true. Any sentence that does not
    hold is amended, and the report names it.

## What this plan does not do

- No seat follows a character log; `set_viewpoint`, the move gate and the
  projection are unchanged (3b).
- No change to `.proto`, `engine`, `perceive`, `sight` or the gateway.
- `Subscribe`'s early unlock stays (D6).
- The world's subscription has no specification and no register row. This
  plan records none for it; that is a ticket of its own, if anyone wants
  one.
- `subscribe.go`'s history comments stay where they are. Moving them out is
  the standing sweep's work, not this ticket's.
