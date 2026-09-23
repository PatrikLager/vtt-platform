# Implementation report — the event core (`internal/store`, `internal/engine`, `internal/campaign`)

**Arc:** sub-project 2, "Event Core"
**Plan:** `docs/superpowers/plans/2026-07-23-event-core.md`
**Spec:** `docs/superpowers/specs/2026-07-23-event-core-design.md`
**Range:** `559ffb0..25a109f` — 12 commits, all 2026-07-23 (see §5 for how that was established)
**Written:** 2026-09-19, against the working tree at `48e3fe2` + uncommitted work (untouched)

---

## 1. Vad som byggdes

In one day, 2026-07-23, the platform got its memory: three Go packages, six new
contract events, and two machine gates. `internal/store` is an append-only
SQLite log — it assigns the authoritative sequence, persists the
protobuf-binary `Envelope`, and fans events out to subscribers, while knowing
nothing about game state. `internal/engine` is the derived projection: a
`State` struct and one pure `Apply` function, with no I/O and no imports beyond
the contract. `internal/campaign` is the composition root and the only package
that imports both: `Open` replays the whole log through `Apply`, `Append`
validates against the projection, persists, then advances it. Six lifecycle
messages joined `events.proto` additively at oneof tags 12–17
(`SceneCreated`, `ActorAdded`, `TokenPlaced`, `SessionStarted`, `SessionEnded`,
`EventsRetracted`). Two gates joined `task check` in the same arc rather than
after it: a semgrep ban on game-system vocabulary under `internal/`, and a
go-arch-lint layer map that forbids `store ↔ engine`. And a keystone property
test asserted the one thing the whole design exists to guarantee —
`rebuild(log) == live projection`. Total: 24 files, 2,966 insertions
(`git diff --stat 559ffb0~1 25a109f`).

One of those six events, and about a third of `internal/campaign`, no longer
exists. `EventsRetracted` and `campaign.Undo` were built here exactly as
specified and deleted thirteen months of project-time later; §4 covers that.

---

## 2. Hur det fungerar i dag

Verified against the current tree. `go test ./internal/store/ ./internal/engine/
./internal/campaign/` → `ok` for all three.

### `internal/store`

`Open(path)` opens one SQLite file through `modernc.org/sqlite v1.54.0`
(pure Go, pinned at `go.mod:10`) and creates the table

```
events(seq INTEGER PRIMARY KEY, event_id TEXT NOT NULL UNIQUE,
       session_id TEXT NOT NULL, occurred_at TEXT NOT NULL, payload BLOB NOT NULL)
```

The schema has not changed since the day it landed: `git log -S 'CREATE TABLE
IF NOT EXISTS events' -- internal/store/store.go` returns exactly one commit,
`d025fae`. The connection string gained `?_pragma=busy_timeout(5000)` on
2026-07-24 (`c218279`), which is the one edit to `Open` since.

`Append(env)` rejects a non-zero `Sequence` and an empty `EventId` before
taking the lock. Under `s.mu` it opens a transaction, reads `SELECT
COALESCE(MAX(seq), 0) + 1`, stamps that into `env.Sequence`, marshals the
*stamped* envelope, inserts, and commits. Every failure path resets
`env.Sequence` to 0 so a caller checking `env.Sequence != 0` can never mistake
a failed append for a persisted one.

**`Append` does not notify subscribers.** That was true for one day only; see
§4. Notification is now a separate public method, `Notify(env)`, which callers
invoke *after* the event's effects are observable, and which silently ignores
`Sequence == 0`.

`AppendBatch(envs)` (added 2026-07-25 with the ruleset interpreter, `03096d3`)
stamps contiguous sequences and persists N envelopes in one transaction,
all-or-nothing.

`ReadAfter(afterSeq)` returns every event with `seq > afterSeq`, in order,
unmarshalled; a payload that fails to unmarshal is a loud error, not a skip.

`Subscribe(afterSeq, buffer) → (events, unsubscribe, catchUpHead, err)` reads
history under the store lock, seeds the subscriber's `pending` queue with it,
records `lastSeq`, and starts a dedicated pump goroutine — the only writer to
the channel and the only closer of it. `catchUpHead` is the highest sequence
queued before `Subscribe` returned, which is how a client knows catch-up has
ended.

**The overflow policy this arc shipped is gone.** Originally the channel was
bounded (`len(history)+buffer`) and `notifyLocked` did a non-blocking send; a
full channel meant that subscriber's channel closed on the spot. Since
`2a5675b` (2026-08-06) the per-subscriber queue is an unbounded slice and
liveness is decided by a timer: `SubscriberNoProgressTimeout = 30 * time.Second`,
measured from the last *successful hand-off*, not from queue depth. `buffer`
now only sizes the hand-off channel. The invariant that survived both designs
is the one the arc actually decided: **no subscriber may block an append, and
the log is the recovery path.**

### `internal/engine`

`State` holds `Scenes`, `Actors`, `Tokens`, `Sessions`, and — from later arcs —
`Conditions` and `Notes`. `Scene` has grown `Tiles`, `Objects`, `OpenDoors`,
`Explored` and `Visible`, none of which are this arc's. `NewState()` builds the
empty maps; `Snapshot()` deep-copies so that readers never alias live state.

`Apply(st, env)` is one switch, validate-before-mutate: any error return leaves
`st` untouched. It had 8 arms at `22a10e7`; it has 23 today
(`grep -c 'case \*vttv1.Envelope_' internal/engine/apply.go` → 23). Of this
arc's original arms, all survive unchanged in shape:
`SessionStarted`/`SessionEnded` (exactly one open session),
`SceneCreated` (duplicate id refused), `ActorAdded`, `TokenPlaced` (unknown
scene, unknown actor, missing position all refused), `TokenMoved`,
`AttackRolled` (deliberate no-op), and the `default` arm returning
`ErrUnknownVariant`. The `EventsRetracted` no-op arm is gone.

### `internal/campaign`

`Open(dir)` takes a **directory**, not a file — that changed in `8059bcb`
("A campaign is a directory, so it can own its maps"). `LogPath(dir)` returns
`dir/log.db`. Open creates the directory at `0o750` if absent, refuses a plain
file, opens the store, and calls `rebuildLocked`.

`rebuildLocked` reads the whole log and folds it through `foldEvents`, which is
a single pass applying every envelope in sequence order; unknown variants are
skipped with a warning (forward compatibility), any other apply error is a
corrupt log and fails loudly. It also sets `c.head` to the last sequence.

`Append(env)`:
1. refuse if poisoned;
2. refuse a nil payload;
3. stamp `session_id` (campaign is authoritative, not the caller — `7090172`, 2026-07-24);
4. clone the envelope, stamp the clone with `c.head + 1`, and fold the clone
   against `c.state.Snapshot()` — validation;
5. `c.log.Append(env)` — **the commit point**;
6. `c.head = seq`;
7. `engine.Apply(c.state, env)` on the live state; a failure here poisons the Campaign;
8. `c.log.Notify(env)`.

`AppendBatch` mirrors that for N envelopes against one evolving snapshot, under
one acquisition of the same mutex.

`State()` returns a snapshot, or `nil` if poisoned. `Subscribe` /
`SubscribeWithNoProgressTimeout` delegate to the store. `Close` closes the store.

**`Undo` is gone, and nothing in this layer replaced it.** `campaign.Undo`,
`retractedSet` and `foldEvents`'s first pass were deleted in `133e896`
(2026-08-31). The platform's answer to a mistake now lives at the command
level: `remove_token` / `remove_actor`, which fold through the `TokenRemoved`
and `ActorRemoved` arms of `engine.Apply` and say "no longer part of the world
going forward", never "this never happened". Two gates keep it out:
`tools/check-no-retraction.py` (wired as `task check:no-retraction`) and
`TestCampaignOffersNoWayToUnmakeHistory`, which asserts a property of
`*Campaign`'s whole method set rather than the absence of one name.
`grep -rn 'EventsRetracted\|RetractEvents' --include='*.go' --include='*.proto'`
returns only comments and one test's string literal — no live identifier.

`FoldPrefix(events)` (`foldprefix.go`) exports `foldEvents` so the gateway can
ask what a prefix of the log produced without keeping its own event-application
loop.

The property test's generator left this package on 2026-09-12 for
`internal/eventgen` (`773cd0f`), because a `_test.go` file cannot be imported.

### The gates

`.semgrep/vocabulary.yml` (`task check:vocabulary`) and `.go-arch-lint.yml`
(`task check:arch`) both still run. The arch map today reads `store:
{mayDependOn: [contract, store]}`, `engine: {mayDependOn: [contract, engine]}`,
`campaign: {mayDependOn: [contract, store, engine, campaign, eventgen,
perceive]}`. A third gate, `check:invariants` over
`.semgrep/event-sourcing.yml`, is the one that actually enforces "only the
engine writes state" — it arrived on 2026-07-30 (`d8d62d2`), a week after this
arc claimed the property was enforced. See §4.

---

## 3. Besluten och varför

**One fold, used by both replay and live append (spec's "Approach C").**
Rejected: separate code paths for replay and for incremental live update.
Reason: with two paths, "replayed state" and "live state" can drift and nothing
structurally prevents it. With one `Apply`, divergence is impossible by
construction, and the keystone property test enforces it besides. This became
CLAUDE.md rule 4.

*One caveat the rule's wording hides:* "one fold" means one `Apply` **function**,
not one loop. Four loops feed it today — `campaign.foldEvents`, `campaign`'s two
live-apply call sites, and `internal/harness.Fold` (which is a deliberate second
loop, bound by the harness's wire-only boundary) — plus `client/src/fold.ts`, a
719-line hand-written TypeScript mirror. The invariant that holds is: one
canonical `Apply` per language, and every Go loop delegates to it.

**The store assigns the sequence, transactionally, and refuses a caller's.**
Rejected: SQLite `AUTOINCREMENT` (which is what the *approved spec* said) and
caller-supplied ordering. Reason: `MAX(seq)+1` read inside the same transaction
under the write lock makes the store the single ordering authority, and
rejecting a non-zero incoming `Sequence` protects against replayed or forged
ordering.

**`event_id` is caller-supplied and UNIQUE.** Rejected: store-generated ids.
Reason: a caller-supplied unique id is a free idempotency guard — a
double-append is a constraint violation rather than a duplicated event.

**Validate against the projection *before* persisting.** Rejected:
persist-then-apply with rollback. Reason: the log is append-only, so there is
no rollback. Validation folds a `Snapshot()` clone so a rejection cannot
half-mutate live state, and a validation failure writes nothing.

**No subscriber may block an append.** Rejected: blocking sends, or an
unbounded blocking queue. Reason: the log is durable and re-readable, so a
subscriber that falls behind can always recover from it — there is no reason to
let one slow reader stall the table. (The *mechanism* for cutting a subscriber
loose has been replaced once; the rule has not.)

**Poison on post-persist failure.** Rejected: keep serving the projection, or
attempt in-process repair. Reason: after the commit point the log holds an event
the projection could not fold; serving on compounds a divergence nobody can see.
There is no in-process recovery — Close and Open replays from scratch and heals.
Store-level errors (I/O, duplicate `event_id`) persist nothing and deliberately
do *not* poison.

**Enforcement lands with the code, not after it.** Rejected: ship the packages,
add the gates in a later hardening pass. Reason: the gates guard the first
engine packages from their first commit, before there is anything to retrofit.
This decision is the one the repo has since leaned on hardest — and §4 shows the
one place it was claimed and not done.

**Pure-Go SQLite.** Rejected: cgo-based `mattn/go-sqlite3`. Reason:
single-binary distribution.

**No snapshots, no compaction, no cross-process subscribers.** Reason: replay is
fast at table scale (thousands of events); the schema does not preclude adding
snapshots later; cross-process fan-out is the gateway's job.

**Readers get deep copies, never aliases.** `Snapshot()` is the only way out of
the projection. Reason: the single-writer rule is only real if it holds at the
type level.

**`AttackRolled` is a deliberate no-op.** Rejected: giving it state meaning here.
Reason: it is testimony, not state — rules meaning belongs to the rules layer.

**Undo as a compensating marker (built here, later reversed).** Rejected at the
time: truncating or mutating the log. Reason then: the log must never be
rewritten, so an undo has to be an ordinary appended event that derivation skips.
Reason it was reversed on 2026-08-30 (Patrik): *"a retraction exists to make
something not have happened, and it cannot — the person already read the log."*

---

## 4. Vad som visade sig fel

### (a) The approved schema described a mechanism that was never built

Spec as approved: `events(seq INTEGER PRIMARY KEY AUTOINCREMENT, …)`.
What shipped: `seq INTEGER PRIMARY KEY`, with the sequence assigned by the store
as `MAX(seq)+1` inside the append transaction. Corrected in `25a109f`:
*"Sequence is assigned transactionally by the store (`MAX(seq)+1` under the write
lock), not by SQLite AUTOINCREMENT."*

### (b) The spec claimed an enforcement that did not exist, for a week

Spec as approved: *"go-arch-lint: `store` ⊄ `engine`, `engine` ⊄ `store`; only
`campaign` may import both; **nothing outside `internal/engine` mutates
`engine.State`**."*

The last clause was false when written. go-arch-lint checks *imports*, not
writes, and `State`'s fields are all exported. Amended at `25a109f` to
*"mutation confinement to `engine.Apply` holds by construction and code review."*
It stayed unenforced until 2026-07-30, when `.semgrep/event-sourcing.yml`
(`d8d62d2`) finally added the guard. ADR-003 now carries the admission in its own
text: *"That semgrep guard did not exist until today — this ADR named its own
enforcement for a week and did not have it."* The property is true today: a
bounded search for direct writes outside the engine
(`grep -rn -E '\.(Scenes|Actors|Tokens|Conditions|Notes)\[' --include='*.go'
internal cmd tools | grep -v '^internal/engine/'`) returns only `_test.go` files
and `internal/rules/conformance`, both explicitly excluded by the rule.

### (c) The overflow signal was described as something it is not

Spec as approved: *"overflow closes THAT subscriber **with an error**."*
There is no error value — the closed channel is the whole signal. Corrected at
`25a109f`: *"channel close is the only signal — no error value."*

### (d) The poison contract was wrong in both directions

Spec as approved: *"Store I/O errors propagate; the campaign is unusable after a
failed append (fail-loud, no partial state) — caller reopens (replay heals)."*

That makes every store error fatal to the campaign, which is neither what was
wanted nor what shipped. Corrected at `25a109f` and implemented at `7fe9adf`:
store-level errors (I/O, duplicate `event_id`) persist nothing, propagate, and
do **not** poison; only a *post-persist* failure poisons. The amendment also
states the residual risk plainly: a commit-stage error is treated as
not-persisted, and in the marginal case where the commit was nonetheless durable,
the divergence is healed on reopen.

### (e) The defect of the arc: `Undo` persisted before it knew the log would still replay

`Undo` as shipped at `d951ec7` checked the *shape* of the retraction range —
in bounds, contains no marker, not already retracted — and then appended the
marker and rebuilt. None of those checks answer the question that matters:
**does the log still fold cleanly once this range is skipped?** Retract the
`SceneCreated` that a later `TokenPlaced` depends on and the rebuild fails — but
the marker is already persisted, so every subsequent `Open` fails too. The
campaign is bricked, permanently, by an operation whose entire purpose was to
repair a mistake.

Found by the keystone property test within hours of it being written. The fix
commit (`c725457`) says so: *"Dry-run of the filtered fold via shared
foldEvents; rejects retractions that would corrupt replay. Found by the keystone
property test."* The fix dry-runs the filtered fold against a scratch state
before persisting anything, and reuses `foldEvents` rather than writing a second
loop — which is how `foldEvents` came to be extracted at all.

The tail of that story is recorded in the spec's own §6 banner and is worth
carrying forward: *"It dry-ran THE LOG, which is exactly the assumption that
broke once a seat could receive a projection of the log rather than the log …
and that defect is what led to the ruling that removed the operation
altogether."*

### (f) The undo half of the arc was reversed in full

Plan Goal, as approved: *"append-only SQLite log, single-fold projection,
subscriptions, **compensating-marker undo**, six additive contract events."*
Spec §1: *"an append-only SQLite log …, live subscriptions, and **truthful
undo**."*

Patrik's ruling of 2026-08-30 ended it. `EventsRetracted` and `RetractEvents`
left the contract in `59542e1`; `campaign.Undo` and `retractedSet` in `133e896`;
both 2026-08-31. The client's `fold.ts` second pass went in `d3e2f28`, the
harness's in `92f1284`. Nothing replaced it inside this layer.

Two second-order consequences are worth naming because neither was predicted:

- **Every fold in the platform was two-pass because of this one decision.** The
  spec says it: *"'Derivation skips retracted ranges' is the sentence to watch:
  it is what made every fold in the platform two-pass, in Go and in TypeScript
  alike, and removing it is what collapsed all four of them to one pass."*
- **The arc's own additive-only constraint was suspended to undo it.** The plan's
  Global Constraints read *"Contract evolution is ADDITIVE ONLY … `check:breaking`
  must stay green throughout."* `EventsRetracted` was deleted with **no
  `reserved` left behind**, on the grounds that the platform is pre-release and
  ADR-007's additive rule does not bind until the `contract/RELEASED` marker
  turns it on.

### (g) `Append` notified subscribers — for exactly one day

The code at `d951ec7` read:

```go
seq, err := c.log.Append(env) // persists AND notifies subscribers
```

A subscriber could therefore observe an event *before* the live projection had
folded it, and read a state that did not yet contain what it had just been told
about. Corrected the next day in `03e5ef1` ("decouple store notification from
append; notify after live apply"). Today the same line reads
`// persists; does not notify`, and `Notify` is a separate call made after the
live apply — with per-subscriber sequence dedupe to close the
subscribe-between-persist-and-notify race.

### (h) The subscriber overflow design coupled adventure size to buffer size

Spec §11, open question: *"Bounded-buffer size for subscribers: pick a default
in implementation (constant, documented); tune when the gateway exists."*
Resolved differently at `25a109f` — a caller-supplied `buffer` parameter instead
of a constant. Then the premise itself was overturned on 2026-08-06, and
`subscribe.go`'s own comment states the cost:

> *"Depth answered 'is this subscriber more than N events behind', which sounds
> like a liveness test and is not one: an atomic batch is delivered in a tight
> loop under the store lock, so a consumer goroutine cannot drain between sends
> no matter how fast it is. Every subscriber was severed by a batch larger than
> its buffer, and that buffer constant thereby became a ceiling on how large an
> adventure could be before loading it disconnected the table."*

That is a real, user-visible defect that escaped this arc: loading a large
adventure disconnected every connected client, and the mechanism was the
overflow-close policy shipped at `a19707e`.

The replacement comment then made a false claim of its own, and says so at
`subscribe.go:31-32`: *"That is the intended SEMANTIC … but it is not a memory
bound, and an earlier version of this comment wrongly claimed it was."* That one
belongs to the 2026-08-06 arc's report, not this one; I note it because it is the
same lineage.

### (i) A validation bug latent in `Append` from day one

`Append` as shipped validated by folding the envelope **unstamped**, with
`Sequence == 0`. Harmless for every arm that only *writes* `env.Sequence`
(`SessionStarted.StartSeq`, `SessionEnded.EndSeq`, …) because the throwaway
clone's value is discarded. Not harmless for any arm whose *rejection* decision
reads the sequence: when the world layer added `NarrationAdded`'s anchor-sanity
check (`anchor_to_seq >= env.Sequence`), **every anchored narration was
rejected**, regardless of anchor validity. Fixed by cloning and stamping the
provisional sequence `c.head + 1` (`2672192`, with the `AppendBatch` twin at
`37c8d8b`); `c.head` exists only to make that clone carry the sequence the store
is about to assign. Pinned by `TestSessionEndedEndSeqUnaffectedByValidationSequence`.

### (j) `AttackRolled`'s forward reference never came true

Plan, Global Constraints: *"Engine rule: `AttackRolled` applies as a deliberate
no-op (testimony, not state — **rules meaning is sub-project 5**)."* The same
sentence is still in the code at `apply.go:409`.

Sub-project 5 landed on 2026-07-25 and gave rules meaning to `AbilityUsed`,
`ResourceChanged`, `ConditionApplied` and `ConditionRemoved` — not to
`AttackRolled`. `AttackRolled` has no producer anywhere in live code:
`grep -rn 'Envelope_AttackRolled{\|AttackRolled{' --include='*.go' --include='*.ts' .
| grep -v node_modules | grep -v contract/gen | grep -v _test` returns only the
frozen `contract-spike/`. It survives as a contract message, a no-op fold arm, a
`perceive` classifier arm and round-trip fixtures. The no-op is still correct;
the clause promising where its meaning would arrive is not.

### (k) What I could not establish

- **Whether the branch `feat/event-core` ever existed.** The plan requires it
  (Global Constraints; Task 1 Step 1: `git checkout -b feat/event-core main`).
  `git branch -a` shows only `main`, `origin/main` and `feat/per-character-logs`;
  history through this arc is a single unbranched chain, and the repo's first
  merge commit is `83be590` on 2026-07-25 — two days *after* this arc. So the
  branch name is plan text with no corroboration in the repo.
- **Whether the per-task reviews the plan mandates happened.** No commit in the
  range carries a body except `c725457`'s two lines, and `.superpowers/sdd/`
  holds task reports only from 2026-08-12 onward.
- **The tool versions Task 6 Step 5 required.** *"Record semgrep and
  go-arch-lint versions in the report."* I found no such report in the tree.
- **`internal/store/store.go:34-37`'s attribution.** It credits the
  `busy_timeout` pragma to a "P6 Task 4 review", but comment and code landed
  together in `c218279` (2026-07-24), which sits in the simulation-harness run.
  I could not reconcile the repo's `P<n>` numbering with its sub-project
  numbering, so I am not asserting either way. Adjacent arc; flagging for
  whoever writes it.

---

## 5. Spår

**Plan** — `docs/superpowers/plans/2026-07-23-event-core.md` (1,147 lines).
Carries a 2026-08-30 amendment banner stating that every undo/retraction step in
it describes machinery that no longer exists.

**Spec** — `docs/superpowers/specs/2026-07-23-event-core-design.md` (258 lines).
Same banner; §6 is marked obsolete in full and deliberately kept.

**Commit range — `559ffb0..25a109f`, 12 commits, all dated 2026-07-23:**

| Commit | What |
|---|---|
| `559ffb0` | docs: event core design spec (sub-project 2) |
| `394bd44` | docs: add event core implementation plan |
| `a5ab1d4` | Task 1 — lifecycle + retraction events, contract tags 12–17 |
| `d025fae` | Task 2 — append-only SQLite store, authoritative sequencing |
| `a19707e` | Task 3 — subscriptions, atomic catch-up, overflow-close |
| `22a10e7` | Task 4 — engine State and the single `Apply` fold |
| `d951ec7` | Task 5 — campaign composition: replay, validated append, undo |
| `bef967f` | Task 6 — vocabulary and architecture gates join `task check` |
| `fa6f506` | Task 7 — keystone rebuild-equals-live property + exit scenario |
| `c725457` | fix — `Undo` validates replay viability before persisting (§4e) |
| `7fe9adf` | final-review wave — semgrep test coverage, poison contract, hardening |
| `25a109f` | docs — five evidence-driven spec amendments (§4a–d, §4h) |

`git diff --stat 559ffb0~1 25a109f` → 24 files, 2,966 insertions, 27 deletions.

**Branch:** none recorded. See §4k.

**How I established the range — four independent checks, so you can redo them:**

1. **The plan quotes its own commit subjects.** Each of the seven "Commit point"
   lines gives an exact subject. For all seven,
   `git log --oneline --grep="<subject>" --fixed-strings` returns **exactly one**
   commit, and the seven come back in plan order.
2. **The chain is unbranched.**
   `git log --format='%h %p %s' 559ffb0~1..25a109f` shows twelve commits, each
   the sole parent of the next — no merge, no fork.
3. **The neighbours bound it.** `git log --oneline --reverse` puts `f1c0a7e`
   (the contract-pipeline arc's final-review wave) immediately before `559ffb0`,
   and `e3a0393` ("docs: API gateway & permissions design spec (sub-project 3)")
   immediately after `25a109f`.
4. **The project memory names the same end point independently.**
   `~/.claude/projects/-Users-patriklager-dev-vtt-platform/memory/vtt-platform-project.md:37`
   — *"Sub-project 2 COMPLETE (merged to main, spec amended at `25a109f`)."*

**Commits outside the range that changed what this arc built** — where the trail
continues:

| Commit | Date | Change |
|---|---|---|
| `03e5ef1` | 07-24 | `Notify` decoupled from `Append`, moved after live apply (§4g) |
| `7090172` | 07-24 | campaign stamps `session_id` under lock |
| `c218279` | 07-24 | `busy_timeout(5000)` on the store connection |
| `03096d3` | 07-25 | `AppendBatch` in store and campaign |
| `a7b3bd3` | 07-29 | golangci-lint adoption touches `store.Open`'s error handling |
| `d8d62d2` | 07-30 | `.semgrep/event-sourcing.yml` — the ADR-003 guard (§4b) |
| `2a5675b` | 08-06 | no-progress timeout replaces overflow-close (§4h) |
| `2672192`, `37c8d8b` | 08 | provisional-sequence validation fix (§4i) |
| `8059bcb` | — | a campaign becomes a directory; `LogPath` appears |
| `59542e1`, `133e896` | 08-31 | retraction leaves the contract and the campaign (§4f) |
| `e76580a` | — | `cancel` → `unsubscribe` rename in `Subscribe` |
| `773cd0f` | 09-12 | property generator extracted to `internal/eventgen` |

**Other sources consulted:** `docs/adr/003-event-sourced-state.md` (carries its
own 2026-07-30 enforcement note and 2026-09-01 undo amendment),
`docs/adr/004-engine-module-boundary.md`, `docs/verification-debt.md` (no entry
touches these three packages), and the memory files `retraction-is-out.md`,
`false-prose-is-the-dominant-defect.md`, `vtt-platform-project.md`.

---

## 6. Kommentarsblock denna rapport friar

**The rule I applied:** a block is freed only if it narrates how something came
to be — this arc's history — and a reader could change that line correctly
without it. A block that states an invariant, a caller contract, a cost a caller
must know about, or a trap a future edit would fall into **stays**, even when it
reads like history.

**Baseline, so the cleanup can be measured** (comment-only lines vs. total, in
non-test files):

```
internal/store:     179 / 517
internal/engine:    538 / 1123
internal/campaign:  276 / 514
```

This report frees **56 lines**. That is deliberately small: most of the comment
mass in these three packages — nearly all of `internal/engine`'s 538 lines —
belongs to the *later* arcs (maps-as-geometry, visibility, actor-kind, world
layer, ruleset interpreter, retraction-leaves) and must wait for their reports.

### Freed

| File | Lines | What it says |
|---|---|---|
| `internal/campaign/campaign.go` | **5–9** | "IT NO LONGER COMPOSES AN UNDO" — spec §6 specified one; Undo, the retracted set and the fold's first pass left on 2026-08-31. |
| `internal/campaign/campaign.go` | **156–158** | "NOTHING IS SKIPPED BY SEQUENCE any more" — `foldEvents` used to take a set of retracted sequences; dropping that parameter is the whole of retraction's departure. |
| `internal/campaign/campaign.go` | **179–187** | "THERE IS NO EventsRetracted GUARD ANY MORE" — `Append`/`AppendBatch` refused the payload for one day, and why the guard could go once the message left the contract. |
| `internal/campaign/foldprefix.go` | **18–26** | "ONE PASS SINCE 2026-08-31" — why the function took a slice (retroactive retraction had to learn the whole retracted set first) and why that reason is gone. |
| `internal/campaign/scenario_test.go` | **56–60** | "IT HELD A MID-LOG UNDO until 2026-08-31" — what the exit scenario lost, and that live delivery is still pinned elsewhere. |
| `internal/campaign/session_stamp_test.go` | **313–318** | "CASE (f) IS GONE, NOT UNTESTED" — the sixth session-stamp case covered Undo's self-generated marker. |
| `internal/campaign/property_test.go` | **55–60** | "IT USED TO SKIP ONE OF TWO" — a second guard, `assertUndoExercised`, reded a walk that never retracted, and left with the action. |
| `internal/store/subscribe.go` | **14–24** | The depth-based drop this constant replaced, and that every subscriber was severed by a batch larger than its buffer. (§4h) |
| `internal/engine/apply.go` | **410, trailing clause only** | "rules meaning arrives in sub-project 5" — a forward reference that never came true (§4j). **Keep** "testimony, not state" and the deliberate-no-op fact; replace only the promise. |

### Deliberately NOT freed — these read like history but are load-bearing

- `internal/campaign/campaign.go:148–154` — states the one-fold rule and its two
  callers. A future editor adding a third loop must read this.
- `internal/campaign/campaign.go:30–37` (poison contract) and `40–47` (the
  Campaign doc) — caller contract.
- `internal/campaign/campaign.go:200–234` — the clone-and-stamp equivalence
  proof. It carries the reasoning any edit to `Append`'s validation must preserve
  (§4i).
- `internal/campaign/foldprefix.go:28–36` (the O(n²) cost) and `38–46` (the cheap
  shape, deliberately not taken) — a caller and a future editor both need these.
- `internal/campaign/forward_only_test.go:13–34` — explains why the test asserts a
  property of the whole method set and why it needs a positive control. Delete it
  and the next person weakens the test.
- `internal/store/subscribe.go:11–13, 26–49` — the constant's meaning, what it
  does *not* bound, and the standing instruction *"Do not add a cap here without
  a measured case."*
- `internal/store/subscribe.go:238–250` — the `unsubscribe` naming rationale and
  the lazy-reclamation fact. Borderline, but a reader changing `unsubscribe`
  needs to know the stopped subscriber stays in `s.subs` until `Notify` compacts.
- `internal/store/subscribe.go:166–198` — the `Subscribe` contract, including the
  shared-pointer immutability rule and the deliberate unclamped negative
  `afterSeq`.
- `internal/store/store.go:96–114, 169–176, 178–189` — `AppendBatch`'s
  reset-on-failure contract and `Notify`'s ordering contract.
- `internal/engine/state.go:1–3` — "Apply is the only state mutator in the
  codebase." Now machine-enforced; the sentence is the rule, not its history.
- `internal/engine/apply.go:47–59` — validate-before-mutate, and the
  `//nolint:gocyclo` justification that tells a future editor *not* to split the
  switch.
