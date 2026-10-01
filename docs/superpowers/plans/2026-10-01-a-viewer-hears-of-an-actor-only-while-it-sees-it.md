# A viewer hears of an actor only while it sees it — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it-design.md`
**Verified:** 2026-10-01, by `verify-ticket`, an agent that did not write the
ticket, against `dcf901d` on `feat/testimony-needs-sight` (`main` at the time;
the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed at
the end and travel with this plan. This plan does not edit the ticket; where a
rule is loose the plan words the requirement and says so.

**Goal, in the ticket's words:** a player or spectator hears of an actor's
resources, conditions, attacks, abilities, control and removal only while it
sees the actor; its own eyes count as seen; a party member stays on every
roster with its status frozen while unseen; an actor seen again is corrected
on sight by bare frames; the DM's and the agent's streams stay whole.

**The owner's rulings, settled and not re-opened.** 2026-09-29: only what a
viewer sees gives them information, for every actor, party members included.
2026-10-01: a player's own characters and a spectator's shoulder are always
known, with their status; a party member nobody's eyes see stays on every
roster, its status frozen while unseen; an actor seen again after changing
unseen is corrected on sight, by bare frames carrying its present status and
no reason or author; an attack, ability or control change naming any actor
the viewer does not see now is withheld entirely.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool sends
every client the whole campaign (`MapToolServer.addRemoteConnection` sends
`SetCampaignMsg(campaign.toDto())`; every token on every layer, with its
states and properties) and relays nearly every message to all clients, so its
clients never hold a stale actor and it has no notion of a status frozen while
unseen or corrected on sight: **MapTool does not solve this, because it sends
everything.** What it filters, it filters in the UI. The map hides an unseen
token, bars and states included: `ZoneViewModel.updateVisibleTokens` keeps
token-layer tokens only inside the visible area, and the renderer clips to
it. The initiative list does not: `InitiativeListModel.isTokenVisible` checks
only `isVisible`, the layer, owner-only and `isHideNPC`, with no sight test,
so a player reads an unseen token's name, bars and states there. Without
individual views, `ZoneViewModel.makePlayerView` gives every player the union
of all PC tokens' sight (`zone.getPlayerTokensWithSight()`); with them, only
owned tokens see (`getOwnedTokensWithSight`). The notes behind this paragraph
were written by an Explore agent against `~/dev/RPTool/maptool` at
`f4b7fef6c`; the verifier read `isTokenVisible`'s body and `makePlayerView`'s
branch. **Borrowed:** the `PlayerView` idea, a viewer keyed by role and the
set of tokens it sees through, which is `Viewer` plus `eyes`; individual views
as the only mode, as this platform already has. **Refused, each with its
reason:** the distribution — every client holds what it must not see and a UI
filter hides it, the model rule 9 names; the initiative list's unseen bars and
states — the exact leak the 2026-10-01 rulings close; client-computed
exposure — sight is decided on the server here (SPEC-016). The correction on
sight has no MapTool precedent, because MapTool never lets a client fall
behind. SPEC-016, the register and the code name no other project; this plan
and the report do.

## Verification, check by check

1. **Every path resolves — by command.** Every file the ticket names exists;
   the twelve seat directories under `scenarios/goldens/*/projections/` are
   exactly the twelve it lists. `look`, `classify`, `transitions` and `eyes`
   are `Projector` methods in `internal/gateway/project.go`;
   `engine.IsPartyMember` is in `internal/engine/actorkind.go`; the four tests
   it names are in `project_test.go`; I2, "Testimony outlives sight", is in
   `docs/reports/2026-08-18-visibility.md`. Every claim in the problem
   paragraph is what the code does: the four status arms read `pr.actors`,
   `transitions` forgets an actor only when `st` lacks it, `knows` passes on
   `pr.actors` or the look, and the look holds every party member.
2. **"Done" is an observation — by command.** The verifier wrote the tests
   items 1, 2, 4, 6 and 7 describe, in a scratch clone, and ran them on
   today's tree: each fails for the ticket's reason (the goblin's condition
   behind a shut door is forwarded; the rogue's condition and resource change
   in the other room are forwarded; the unseen rogue's attack on the player's
   own character is forwarded; the token-less familiar granted to the player
   is not introduced; the rogue's damage is in the player's fold before the
   door opens; the unseen removal is forwarded). Item 3's two tests are
   still-reaches guards; the token-less own character's arrival fails today
   only for a non-party actor, which is the case the test must use. Item 5 is
   VTT-208's existing tests, green on the sketch. Items 8 and 9 are gates.
3. **Each rule is breakable — a reading.** Seven of the nine rules name a
   failing observation as written. Three are loose and the sort words them:
   "brought to its present resources, conditions and controllers" omits the
   kind, which a grant also changes (row H); "an actor's removal reaches only
   a viewer that saw the actor when it was removed" is ambiguous under
   `remove_actor`'s batch and, read as "any `ActorRemoved` frame", contradicts
   the bare removal a reused id needs (rows K and L, Q1); "introduced afresh
   to every viewer" is too broad, since a reused id that is neither seen nor a
   party member is introduced to no viewer (VTT-211 already says it right).
4. **The scope matches the claim — by command, then a reading.** `Project`,
   `reperch`, `look` and `canSeeSquare` have one production caller file,
   `internal/gateway/seat.go` (`receive`, `perch`, `canSee`), whose calls do not
   change. A sketch of the design (Measurements) moves **nine** of the twelve
   projected streams and **four** of their states, and reds exactly two tests
   besides those goldens and the two oracles: `TestARemovalBatchProjectsToTheBytesBothFoldsRead`
   and, by its assertions on the same fixture (read, not run),
   `client/test/removal-batch-parity.test.ts`. The four tests the ticket
   names stay green unchanged. Forced and not named, each
   covered here: `contract/testdata/removal_batch_projected_stream.json` and
   `client/test/removal-batch-parity.test.ts` (D10); the `why` of five seats
   (D12); `internal/eventgen/model.go` if Q5 says yes. The client gains no
   code: its folds already accept every frame the design sends (measured on
   both folds).
5. **No recorded decision is contradicted silently — a reading.** SPEC-016's
   rulings for the four status arms, `knows`, the forgetting loop, and the
   Consequence that testimony reaches holders are overturned; its own Status
   paragraph and the ticket name them. VTT-207 is made false (the ticket
   names it). The 2026-08-18 visibility spec's "party members are always
   known" survives as the roster; its testimony reasoning is history and is
   not edited. No ADR decides testimony (`grep -il 'party member\|testimony\|roster'
   docs/adr/*.md` prints nothing).
6. **The records the work moves are named, and reachable — by command, then a
   reading.** The section names SPEC-016, which resolves. The reading:
   SPEC-015's perch sentences defer to SPEC-016 and stay true; SPEC-007's
   `remove_actor` batch paragraph is unchanged and is the reason for Q1;
   SPEC-013's "an attack checks no sight" is untouched, as the ticket says.
   No other specification carries a testimony sentence.

## Measurements this plan stands on

All at `dcf901d`, by command, in a scratch clone, never in the working tree.
The sketch of D1 to D9 is the verifier's, not the plan's code; it shows the
design runs, and it was discarded.

- `python3 tools/check-requirements-chain.py .`: `240 rows, 202 test files, 10
  specifications`. Next id VTT-241; none is allocated before sign-off.
- Comment shares (share / ceiling): `project.go` 20.2/20.3, `keystone_test.go`
  45.5/45.5, `project_property_test.go` 30.9/31.0, `project_test.go`
  31.5/31.6, `project_internal_test.go` 33.3/33.4, `seat.go` 29.6/29.6,
  `internal/eventgen/model.go` 29.2/29.3. The sketch took `project.go` to 17.0,
  more than 1.0 under its row, so `--write-ledger` follows (D16).
- Mutation keys in `project.go`: three, `objectInSight`'s two
  `CONDITIONALS_BOUNDARY` and `sortedSceneIDsUnion`'s `ARITHMETIC_BASE`, all
  below every edit; `check_mutation_test.py` reported all three stale on the
  sketch.
- **The disk.** `check:mutation` refuses below 16 GiB free on `TMPDIR`; the
  machine had 10.8 GiB, and the go build cache held 7.3 GiB. `task check`
  whole cannot pass here until space is freed (Q7).
- **The sketch passes everything but the goldens it moves.** With D1 to D9,
  `go test ./internal/gateway/` red only `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`
  and `TestARemovalBatchProjectsToTheBytesBothFoldsRead`; the new keystone
  and property clauses (D13, D14) were green; `internal/eventgen`,
  `internal/campaign`, `internal/harness` and `cmd/vtt` green; every emitted
  stream folded in both folds (`bun test client/test/projection-parity.test.ts`
  over the emitted streams and their Go-dumped states: 15 pass).
- **What moves in the corpus** (stream frames before → after; F forwarded, S
  synthesized):

  | Seat | Stream | State | What changes |
  |---|---|---|---|
  | `adventure-night/act-fighter` | same | same | every actor it hears of is its shoulder |
  | `story-table/act-hero` | same | same | |
  | `three-role-exit/act-lera` | same | same | |
  | `door-watch/act-latecomer` | moves | same | act-watcher's grant F at 4 → S at 17, on sight |
  | `door-watch/act-watcher` | moves | same | act-latecomer's grant F at 16 → S at 17 |
  | `toy-brawl/act-brawler` | moves | same | act-patron's grant F at 6 → S at 8 |
  | `toy-brawl/act-patron` | moves | same | act-brawler's grant F at 5 → S at 8 |
  | `session-zero/player` | moves | same | own grants at 5 and 7: F → S (D4) |
  | `session-zero/act-fighter` | moves | moves | act-healer's grant at 7 withheld; act-healer frozen with no controller |
  | `session-zero/spectator` | moves | moves | act-fighter's grant at 5 withheld; frozen with no controller |
  | `shared-control/act-scout` | 11 → 5 | moves | all six control frames about act-warden withheld; act-warden frozen with none |
  | `shared-control/act-warden` | 14 → 12 | moves | act-scout's grant and revoke withheld; `headSequence` 13 → 12 |

  Every move is a control frame. No committed golden corrects a resource or a
  condition, holds a ghost, or reuses an id.
- **The removal batch.** `remove_actor` appends each `TokenRemoved` before the
  `ActorRemoved` (SPEC-007), so at the `ActorRemoved` no viewer sees the actor
  by a token: the sketch forwarded it only to the actor's controllers and a
  spectator perched on it. The removal-batch seat, which watched the goblin,
  keeps it in its fold (8 frames, fixture 9).
- **Fault injections on the sketch**, each run and reverted, against the unit
  probes, the keystone with D13's clause and the property walk with D14's:
  status payloads forwarded to holders — keystone red over `toy-brawl`'s
  unassigned player and unperched spectator, property red in all six seeds;
  control payloads forwarded on `knows` — keystone red over six `door-watch`
  seats, property all six; no correction on sight — keystone red over five
  `door-watch` seats, property seeds 2, 3 and 5; eyes not counted as seen —
  keystone and property red; removal forwarded to every holder — **only the
  unit probe red**; no bare removal before a reused id is introduced again —
  **only the unit probe red**. The last two are why D13 and D14 gain a removal
  clause, and why reuse is held by unit tests (`eventgen` never reuses an id).
- **Coverage of the walk.** Over six seeds the property walk made 18
  corrections on sight (12 with frames, 6 with nothing to correct), none
  through D7's re-introduction and no reuse; with D14's injected resource
  changes, ten corrections carried a `ResourceChanged`, in four seeds of six
  (1, 2, 3, 5). The keystone made 21 corrections with frames.

## Constraints that bind every task

- `CLAUDE.md` rule 2: no gate is weakened. Mutation keys are re-pointed to
  where their mutants now are, never deleted; a survivor in new code is killed
  by a test; the ledger only goes down, only through `--write-ledger`.
- Rule 3: no contract change. Every frame is an existing payload; `check:breaking`
  must report nothing.
- Rule 4: one fold. The projector's memory is snapshots of `st`, taken after
  `engine.Apply` has folded the event; it never applies an event. Its fields
  are not named `Actors`, `Conditions`, `Scenes`, `Tokens` or `Notes`, or
  `.semgrep/event-sourcing.yml`'s indexed-write patterns match them.
- Rule 5: "resource", "condition", "kind" are platform vocabulary; no
  resource name enters `project.go`. Test fixtures use neutral names (`pool`),
  never `hp`.
- Rule 8: this plan, SPEC-016 and the report cite symbols, tests and files,
  never a line; mutation keys keep their coordinates, as rule 8 says.
- Rule 9: the answer above is recorded before any task runs.
- Rule 10 and SPEC-010: a comment is a warning, a pointer or an exported
  symbol's one-line doc. Comments in `project.go` that this work makes false
  are rewritten or deleted in the same commit: the struct's "Forget an actor
  only where transitions does, after classify has forwarded its
  ActorRemoved", `transitions`' "this seat was forwarded its ActorRemoved",
  `classify`'s two group comments. New memory fields get one warning line
  each, paid for by code.
- SPEC-008: ids from `requirement-id` only, after sign-off, at the start of
  the task whose commit carries them; a withdrawn row keeps its row, marked as
  VTT-035 is.
- The `specification` skill: present tense, the old sentence not left beside
  the new, every "only"/"never"/"every" with its search named.
- The `requirements` skill: one thing, breakable, named by a check; refusals
  one line each.
- Tests before code; one deliberate break per new check, both directions, as a
  line in the commit (D15).
- QA tests stay in-process: `check:mutation` re-runs each package per mutant,
  and a subprocess timing out under load counts as a kill.
- A reading review's package is `git diff HEAD`; nothing is stashed or checked
  out while a reviewer reads; the review settles before a gate starts; keys
  move last. `git add` and `git commit` in separate calls, `git show --stat
  HEAD` after each.
- `uptime` before any gate; `task check` detached with
  `start_new_session=True`; `git push` given 600 000 ms and never killed under
  five minutes.
- The ticket, `docs/adr/`, every report under `docs/reports/`, and the
  visibility and projection-record designs are not edited.

## Decisions this plan makes

**D1. "Seen" is the look's new `sees` set: eyes, and the actors of tokens on
seen squares.** Forced by rulings 1 and 4 and item 3. `look` adds each actor
`eyes` returns to `sees` and to `actors` (known), whether or not a token of it
stands anywhere; then each token on a seen square adds its actor to both;
then every party member to `actors` only. So a player's own token-less actor
is seen and introduced (item 4), and a spectator's shoulder, a party member by
`eyes`' own test, is seen. `canSeeSquare` and `canSee` read squares only and
do not change.

**D2. Every actor-naming payload is judged against what the viewer saw
before the event.** Forced by: `Project` judges against the state after the
event, and the fold of a forwarded status event is right only if the viewer's
fold held the actor's status before it; `remove_actor`'s batch takes the
tokens first, so "after" cannot see a removed actor at all; and a payload that
brings an actor into sight is already carried by D6's correction. The
projector keeps `sighted`, the `sees` set of its previous `transitions` (by
event or perch), and `classify` reads it: `ResourceChanged`,
`ConditionApplied`, `ConditionRemoved` and `ActorRemoved` forward when their
actor is in `sighted`; `AttackRolled` (attacker, target), `AbilityUsed`
(actor, targets), `ActorControlGranted` and `ActorControlRevoked` when every
non-empty id they name is. For the status and testimony payloads "before"
equals "after", since none of them changes sight; for control it is a choice
(D4). `knows` and its use of the look's `actors` are deleted.

**D3. The memory a correction needs, per held actor: the current value of
each resource, the set of condition ids, the set of controllers, and the
kind.** Forced by the fold: name, attributes, module, the set of resources
and each resource's maximum are written only by `ActorAdded`
(`engine.Apply`'s `ResourceChanged` arm stores `Max: res.Max`;
`client/src/fold.ts` says "Max is carried over from state, never taken from
the event"; `grep -n 'Resources\[' internal/engine/apply.go` prints only that
arm's read and write), and kind and controllers only by the two control arms.
So within one incarnation of an id, those four are everything that can go
stale. The memory is a snapshot of `st` taken when the actor is introduced or
corrected, and after every event for each actor in `sighted` or in the new
`sees`: for such an actor every event that changed it was forwarded (D2), so
the viewer's fold equals the snapshot. Within one incarnation, the memory is
exactly what the viewer's fold holds of the actor.

**D4. A control change is judged before the event only, and what it brings
into sight arrives by introduction or correction.** Forced by D2's uniformity
and measured both ways: before-or-after also passes every oracle, but sends a
viewer whom a grant brings into sight the grant twice, once synthesized and
once forwarded (`session-zero/player`: 16 → 18 frames); before-only sends it
once, synthesized (16 → 16, F → S). A revoke that takes a player's own actor
out of its eyes is still forwarded, since the actor was seen before. Q3.

**D5. Status is compared as sets; order, source and applied sequence are not
status.** Forced by ruling 2 and "only what a viewer sees". Restoring the
server's order of conditions or controllers would send frames for a change
whose net effect is nothing (a condition removed and re-applied unseen), which
tells the viewer something happened while it did not see. So a correction
removes the conditions the memory has and `st` lacks, applies those `st` has
and the memory lacks in `st`'s order, and the same for controllers; a change
undone unseen sends nothing (row J). The viewer's `ControllerId` mirror and
condition order may then differ from the server's; the client reads
membership (`client/src/player.ts`), and no fold or view keys on order.

**D6. The correction on sight.** Forced by ruling 3 and item 6. In
`transitions`' actor walk, for each actor in the new `sees` that is held, not
gone (D8) and not in `sighted`: frames at `seq`, with no event id, role,
participant, session, reason or source, in this order — one `ResourceChanged`
per resource whose current differs, in name order, `delta` the difference and
`new_value` the present value; `ConditionRemoved` then `ConditionApplied` per
D5; then the control frames: a grant per new controller carrying the present
kind, then a revoke per lost one. When the kind differs and no grant is sent,
a grant re-states a controller the memory holds, before any revoke, carrying
the kind. The actor's tokens follow in the existing token walk. It fires on an
event (a token steps into view, a door opens, an eye is placed, a grant makes
an actor an eye or a party shoulder) and on a perch, the stale shoulder itself
included, since `reperch` runs `transitions` with the new look and the old
`sighted`. The frames carry `perchSequence` there (VTT-186).

**D7. Two corrections the frames above cannot express are re-introductions.**
Forced by the folds. A kind change with no controller on either side (a grant
and its revoke while unseen) has no participant to carry it without naming
one the viewer never saw; and a difference in the set of resources, which one
incarnation cannot have, is a fault. Both send a bare `ActorRemoved`, then the
ordinary introduction. Both folds accept it, since none of the actor's tokens
is on the viewer's board when it enters sight. The feed reads "actor X left",
"actor X joined" (Gap 4).

**D8. An actor removed while unseen stays in the fold, and its id's next
introduction is preceded by a bare `ActorRemoved`.** Forced by item 7, the
measured batch, and both folds refusing a second `ActorAdded`. The forgetting
loop forgets an actor `st` no longer holds only when it is in `sighted` (its
removal was forwarded); otherwise it marks it `gone` and keeps it held, with
its memory frozen. When a gone id is next in the look's `actors` with `st`
holding it, `transitions` sends a bare `ActorRemoved` of it, then the
introduction. A gone id never introduced again stays in the fold. Q1 asks
whether a viewer who watched the removal batch should be told instead.

**D9. A correction whose difference does not fit `int32` passes through
zero.** Forced by the fold's floor and `add_actor`, which accepts any starting
current. The difference can overflow only when the believed value is below
zero (a present value set by a `ResourceChanged` is at least 0, and an unseen
change cannot exceed `int32`'s top), so the first frame carries `delta` 0 and
`new_value` 0, which the floor makes true, and the second the present value.
Its test starts an actor at `math.MinInt32`.

**D10. The removal-batch fixture is re-derived to the new rule.** Forced by
D8 and the fixture's role as bytes both folds read. Its stream loses the
`ActorRemoved` (the seat watched the goblin's token go and is not told of the
removal) and gains the reuse of `goblin`, placed in sight: a bare
`ActorRemoved`, the introduction and the `TokenPlaced`, so both folds still
read a synthesized `ActorRemoved`. `TestARemovalBatchProjectsToTheBytesBothFoldsRead`
asserts the old goblin held with no token after the batch and the new one
after the reuse; `client/test/removal-batch-parity.test.ts` the same of the TS
fold. Under Q1's alternative the fixture keeps its `ActorRemoved` and only the
reuse is added.

**D11. Introductions carry no condition source.** Only if Q2 says yes.
Forced, if so, by ruling 3's reason: an introduction is also a frame that
brings an actor up to date, and today it sends `Source` (`ability:<id>:<phase>`
or `threshold:<resource>`, built by `internal/rules`' `applyOutcomes` and
threshold walk), naming what applied a condition the viewer did not see. It
moves `door-watch/act-watcher`'s stream and state (`threshold:drink`) and
`TestAConditionAppliedOutOfSightArrivesWithTheActor`'s source assertion.

**D12. Which goldens move, and how each is made.** Forced by the corpus
README (state by hand; a projected stream derived, from the projection's
emitted output, read before it is committed).

| Golden | Commit | How |
|---|---|---|
| the nine moving `stream.json` of the table above | C2 | copied from `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`' emitted stream, read frame by frame against D2 to D6 |
| `session-zero/act-fighter`, `session-zero/spectator`, `shared-control/act-scout` `state.json` | C2 | by hand: the unseen party member keeps the controllers it had at its introduction (none) |
| `shared-control/act-warden/state.json` | C2 | by hand: `headSequence` is the last frame's sequence, now 12 |
| `door-watch/act-watcher` stream and state | C2, only if Q2 | by hand: the introduced condition without its source |
| `contract/testdata/removal_batch_projected_stream.json` | C3 | by hand, then held to the emitted stream (D10) |
| `viewer.json` `why` of `door-watch/act-latecomer`, `door-watch/act-watcher`, `shared-control/act-scout`, `shared-control/act-warden`, `toy-brawl/act-brawler` | C2 | rewritten where they describe `knows()`, `passIf(pr.actors[id])`, "control events are knows()-gated, not sight-gated", or "both facts would simply forward"; each says what reaches the seat and why, naming no arm and counting nothing |

The log-level goldens do not move: the projection writes nothing to the log.
The other seven `why`s were read and stay true ("the party ROSTER still does"
holds: party members are still introduced).

**D13. The keystone's oracle gains eyes, status and removal, from the rulings
and not from `project.go`.** Forced by item 6 and by the measured fact that
nothing else catches a removal told to every holder. In
`keystone_test.go`: `visibleState` adds the eyes to `actors` (C1) and gains
`sees` (each `oracleEyes` actor, and the actor of each token on a seen square;
C2).
`walkKeystone` keeps, per seat, the world's status of each actor after the
last event at which the oracle saw it before or after that event, or at the
first prefix it was knowable in its incarnation (an id absent at a prefix
starts a new incarnation); status is D3's four plus the actor's name,
attributes and resource maxima, so a stale incarnation is caught. At every
prefix every actor the seat's fold holds and the world holds must equal that
status (C2). And an actor the world removed at this prefix must have left the
seat's fold if the oracle saw it at the previous prefix, and stayed if not
(C3). It calls `visibleState`, `oracleEyes` and `engine.State`, nothing in
the projection.

**D14. The property walk gains the same clauses, and resources.** Forced by
the measurement that the walk carries no resource. `assertSound`'s two player
clauses ("holds actor ... the server does not have", "believes ... carries
condition ... the server does not") pin today's testimony and are replaced,
for players, by D13's status and removal clauses checked after every event
against `visibleState` (they stay as they are for the DM and the agent).
`eventgen.addActor` declares one resource without a draw (`pool`, current 3,
max 5); the walk applies a `ResourceChanged` on every third step to the actor
picked by step index, clamped, with no draw, projected after the step's own
event and before the next. Floor: a minimum number of seeds whose
corrections carry a `ResourceChanged`, set below what the final schedule
measures (the verifier's schedule measured four of six). Q5.

**D15. The deliberate breaks, one per new check, both directions.** Each is an
act in a scratch state, repaired before the commit, and one line of the
commit message. Measured by the verifier where marked.

| Commit | Check | The break, and what must red |
|---|---|---|
| C1 | row E, D13's eyes | `look` without the eyes: row E's test; keystone over `door-watch/player/p-hidden` and `shared-control/player/p-eli` (measured in the other direction: the oracle without eyes red there) |
| C1 | row F | a token-less non-party actor granted to another player introduced: F's test |
| C2 | row A | status payloads forwarded to holders: A's tests; keystone over `toy-brawl`'s unassigned seats; property, six seeds (measured) |
| C2 | row B | testimony forwarded on `knows`: B's test (measured on today's tree) |
| C2 | row C | control forwarded on `knows`: C's test; keystone over six `door-watch` seats; property, six seeds (measured) |
| C2 | row D | eyes not in `sees`: D's test; keystone; property (measured) |
| C2 | rows G, H | no correction on sight: H's tests; keystone over five `door-watch` seats; property seeds 2, 3, 5 (measured) |
| C2 | row I | a correction carrying the event's reason or metadata: I's test |
| C2 | row J | conditions corrected by order (remove from the first difference, re-apply): J's test |
| C2 | D7, D9 | the re-introduction removed; the through-zero frame removed: each one's test (a fold error) |
| C3 | row K | removal forwarded to every holder: K's test; D13's and D14's removal clauses (to be measured: without them only the unit test reds) |
| C3 | row L | no bare removal before a reused id's introduction: the reuse test (measured: only the unit test reds) |

**D16. Mutation keys and the ledger, last in each commit.** After the review
settles: `--report` on the commit's files; `--write-ledger` where a share fell
more than 1.0 under its row (`project.go` will); the three `project.go` keys
re-pointed by reading the token at the new line and column; both mutation
self-tests print `OK`. New code is put after `noteTransitions`, so only the
struct, `look`, `transitions` and `classify` edits move the keys. A survivor
in D6's or D9's conditionals is killed by a test asserting the exact frames,
not only the folded result.

**D17. Specification sentences land with the code that makes them true.**
Forced by the house rule of the note arc (every sentence true at every
commit). SPEC-016, by commit: C1 — "What a look sees" (eyes in the look's
`actors`), the Status pointer to this ticket and what of it C1 implements,
VTT-207 out of `Requirements`, E and F in; `sees` and "Whose eyes" are C2's
or unchanged. C2 — "What the projector
remembers" (`sighted`, the memory, its snapshot rule), "What `transitions`
sends" (the correction and its order, D7), "How each payload is ruled" (D2,
D4, the status and testimony arms), "A perch" (a stale shoulder is
corrected), "The sequence every frame carries" (corrections carry it),
Consequences: "A party member is on every projected roster, seen or not"
gains "with the status it had when the viewer last saw it"; "A viewer present
at a grant that introduces an actor receives the grant twice" is deleted;
added: a frame with no event id is the projection's own; a change undone
unseen sends nothing; conditions and controllers corrected on sight need not
keep the server's order. C3 — the removal sentences (D8), the Consequence that
an actor removed unseen stays in the fold until its id is introduced again,
"Principles served" without "their roster already holds", and the Status
paragraph without the pending decision.

## The sort

Rows are lettered; nothing is an id before sign-off. Every accepted row is new
work, `**OPEN — no test yet**` when dispensed, filled in the same commit.

| # | Rule (what) | Proposed | The observation that goes red | Test | Commit |
|---|---|---|---|---|---|
| A | A resource or condition change reaches a player or spectator only when it saw the actor before the event. | accept, rule 1 | a goblin behind a door that has shut, or a party member in another scene, and its `ConditionApplied`/`ResourceChanged` is among the seat's frames | `project_test.go#TestAConditionOnACreatureBehindAShutDoorReachesNoPlayer`, `#TestAPartyMemberInAnotherSceneIsNotHeardOf` (new; both red today, measured); property, keystone | C2 |
| B | An attack or an ability use reaches a player or spectator only when it saw every actor the payload names before the event. | accept, rule 2's first half | an unseen party member's `AttackRolled` on the player's own character is forwarded | `#TestAnUnseenPartyMembersAttackReachesNoPlayer` (new; red today, measured) | C2 |
| C | A control change reaches a player or spectator only when it saw the actor before the event. | accept, rule 2's second half, worded per D4 | a grant on an unseen party member is forwarded | `#TestAControlChangeOnAnUnseenPartyMemberReachesNoPlayer` (new); keystone; property | C2 |
| D | A player's own actors and a spectator's shoulder count as seen, with or without a token on a seen square. | accept, rule 3 | a token-less non-party actor the player controls: its `ResourceChanged` is withheld | `#TestAPlayersOwnTokenlessCharacterIsHeardOf` (new; red today for a non-party actor, which is never introduced, measured) | C2 |
| E | A player is introduced to every actor it controls. | accept, rule 4 | a token-less non-party actor granted to the player is absent from its fold | `#TestAPlayerIsIntroducedToATokenlessActorItIsGranted` (new; red today, measured); keystone (eyes) | C1 |
| F | An actor that is neither a party member nor one of the viewer's eyes is introduced only once a token of it stands on a square the viewer sees. | accept, VTT-207's successor | an NPC held by the DM, another player or nobody is introduced with no token in sight | VTT-207's four tests, re-cited: `#TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster`, `#TestAKindlessGrantConfersControlAndNothingElse`, `#TestAnActorWithNoDeclaredKindIsNotAPartyMemberWhoeverHoldsIt`, `#TestGrantingAnAgentTheShippedGoblinArcherDoesNotPublishItToThePlayers` | C1 |
| G | A player's or spectator's fold holds each actor it does not see at the status the actor had after the last event at which the viewer saw it, or at its introduction. | accept, ruling 2 as an invariant | any prefix where a seat's unseen actor differs from that status | `keystone_test.go#TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` (status clause); `project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` | C2 |
| H | An actor that comes into a viewer's sight, by an event or by a perch, is brought in its fold to its present resources, conditions, controllers and kind. | accept, rule 6 with the kind | the fold differs from the server for an actor the viewer sees | `#TestDamageTakenOutOfSightArrivesWhenTheEyeWalksIn` (new; red today, measured), `#TestAKindChangedOutOfSightIsCorrectedOnSight`, `#TestAResourceStartedBelowZeroIsCorrectedOnSight`; `viewpoint_internal_test.go#TestAPerchOntoAStaleShoulderCorrectsIt`; keystone; property | C2 |
| I | A frame that brings an actor up to date carries no event id, role, participant, session, reason or source. | accept, rule 7 (and the introduction's source if Q2) | a correction frame with any of them set | `#TestDamageTakenOutOfSightArrivesWhenTheEyeWalksIn` (cites both) | C2 |
| J | A change an actor undid while the viewer did not see it sends nothing when the actor comes into sight. | accept, D5's rule | a condition applied and removed unseen yields a frame on sight | `#TestAChangeUndoneOutOfSightSendsNothingOnSight` (new) | C2 |
| K | An actor's `ActorRemoved` is forwarded only to a player or spectator that saw the actor before the event. | accept, rule 8 worded for the forwarded event (Q1) | an unseen removal forwarded; a watched removal batch forwarded to the watcher (under Q1's recommendation) | `#TestARemovalOutOfSightIsNotReported` (new; red today, measured), the rewritten `#TestARemovalBatchProjectsToTheBytesBothFoldsRead`; keystone and property removal clauses | C3 |
| L | A bare `ActorRemoved` is sent to a viewer only directly before an introduction of the same id. | accept, rule 9's frame | a seat holding a removed actor whose id never returns is sent an `ActorRemoved` | `#TestAnIdReusedAfterAnUnseenRemovalFoldsAgain` (new; also cites VTT-211), `#TestARemovedActorWhoseIdNeverReturnsStaysInTheFold` (new) | C3 |

**Refused, one line each:**

1. "A party member is introduced to every player and spectator, seen or not"
   — VTT-208 holds it, as the ticket says.
2. "An actor id the world reuses is introduced afresh to every viewer, one
   that missed its removal included" — VTT-211 as worded already covers the
   viewer that missed it; L's reuse test cites VTT-211. "Every viewer" is too
   broad (check 3).
3. "An actor's removal reaches only a viewer that held it" — VTT-215, kept
   (Q4).
4. "A correction carries the event's sequence, or the perch's" — VTT-221 and
   VTT-186.
5. "Every correction folds" — VTT-224.
6. "One log projects to one stream" — VTT-223; D6's walks are in name and
   slice order.
7. "The DM's and the agent's streams stay whole" — VTT-176.
8. "An unperched spectator's fold holds every party member" — VTT-208 (item 5).
9. "The client gains no code" — a constraint, nothing to observe.
10. "A resource's maximum never changes" — the fold's fact (D3), not this
    ticket's rule.
11. "Conditions and controllers are corrected as sets" — how J is achieved.
12. "A kind change with no controller is corrected by re-introduction" — how
    H is achieved (D7).
13. "A correction through zero takes two frames" — how H is achieved (D9).
14. "Status payloads are judged before the event" — how A is achieved.
15. "An attack a player issues at a target it cannot see is not refused" —
    SPEC-013's, out of the ticket.

**Existing rows that move:**

- **VTT-207** — withdrawn in C1, its requirement cell `WITHDRAWN 2026-10-0N:
  <its text>. False as worded since <C1>: a player is introduced to every actor
  it controls, token or not. Succeeded by <E> and <F>.`, its evidence
  `**READING — verify-ticket check 5 of 2026-10-01; withdrawn, held by
  nothing**`, the form VTT-035 set. Its four tests' `// VTT-207` lines become
  F's id; `TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster` keeps
  `VTT-194`.
- **VTT-225, VTT-224** — evidence unchanged; their tests gain G's and K's
  clauses and cite G and K beside them.
- **VTT-211** — evidence gains the reuse test in C3.
- **VTT-215, VTT-208, VTT-209, VTT-210, VTT-218, VTT-226** — read against the
  final rules, true as worded; unchanged. VTT-210 stays true under Q2's yes
  (a condition id still arrives).

## Tasks, in dependency order

Each task is one commit and runs Phase 3, 4a, 4b and 5 for it. "Test first"
names what is written before the code and how it fails on the tree the task
starts from.

### Task 0 — Instruments and baselines

**Files:** none in the repository. `uptime`; the chain checker; `--report`
filtered to the files above; both mutation self-tests; `which requirement-id`;
`df -h $TMPDIR`; `git status --short` (the ticket and this plan untracked).

**Done when:** they match Measurements (240 rows, the shares, `OK` twice —
the disk-refusal self-tests fail below 16 GiB free, so Q7 is answered first).

### Task 1 — A player's own actors are seen (C1)

**Files:** `internal/gateway/project.go` (`look`); `project_test.go`;
`keystone_test.go` (`visibleState`); `sightView`'s and `oracleView`'s `sees`
arrive in C2, where they are read;
`docs/specifications/016-the-projection.md`; `docs/requirements.md` (E, F,
VTT-207); the ledger and keys (D16).

**Test first:** E's test (red: the familiar is not introduced, measured); the
keystone's eyes clause, which reds over `door-watch/player/p-hidden` and
`shared-control/player/p-eli` until `look` changes (measured the other way
round: `look` with eyes and the oracle without red those two seats).

**Do:** D1's eyes; D17's C1 sentences; VTT-207 withdrawn, E and F dispensed,
the four tests re-cited.

**Done when:** `go test ./internal/gateway/` green with no golden moved
(measured: none moves); chain checker 242 rows; QA and review recorded; D16;
committed.

### Task 2 — Testimony needs sight, and the correction on sight (C2)

**Files:** `internal/gateway/project.go` (`Projector`, `NewProjector`,
`sightView`, `look`, `transitions`, `classify`, new functions after
`noteTransitions`); `project_test.go`; `viewpoint_internal_test.go`;
`project_property_test.go`; `keystone_test.go`; `internal/eventgen/model.go`
(Q5); the nine projected streams, four states and five `why`s of D12;
SPEC-016; `docs/requirements.md` (A to D, G to J); `tools/mutation-equivalents.txt`;
the ledger.

**Test first:** A's two tests, B's, C's, D's, H's three, I's, J's and the
perch test, each red on the C1 tree (A, B, D, H's first measured); the
keystone's status clause (red: `toy-brawl`'s unassigned seats hold the
patron's changed drink); the property clause (red in every seed once
`assertSound`'s two player clauses give way). Build every new test's world by
feeding the projector from the log's first event, as a seat does: `sighted`
is memory, and a projector handed an event out of order judges it against
what it saw last.

**Do:** D2 to D7, D9; D11 if Q2; D13's status clause; D14; D12's C2 goldens.
In D14's walk, project the step's own event before applying the injected
change: the verifier's first sketch applied it first and judged one event
against the next one's state.

**Done when:** `go test ./internal/gateway/ ./internal/eventgen/
./internal/campaign/ ./internal/harness/ ./cmd/vtt/` green; `bun test
client/test/projection-parity.test.ts` green over the re-derived goldens;
chain checker 250 rows; `grep -n 'knows' internal/gateway/project.go` prints
nothing; QA and review recorded; D16; committed.

### Task 3 — Removal needs sight (C3)

**Files:** `internal/gateway/project.go`; `project_test.go`;
`keystone_test.go`; `project_property_test.go`;
`contract/testdata/removal_batch_projected_stream.json`;
`client/test/removal-batch-parity.test.ts`; SPEC-016; `docs/requirements.md`
(K, L; VTT-211's evidence); keys; ledger.

**Test first:** K's and L's tests (K red today, measured); the rewritten
removal-batch tests and fixture (red: the `ActorRemoved` is still forwarded);
the two removal clauses, shown red under K's break before the code lands.

**Do:** D8, D10; D13's and D14's removal clauses; D17's C3 sentences, the
Status paragraph last.

**Done when:** `go test ./internal/gateway/` and `bun test` green; chain
checker 252 rows; `grep -n 'not implemented' docs/specifications/016-the-projection.md`
prints nothing; QA and review recorded; D16; committed.

### Task 4 — The whole gate

`uptime`; `df -h $TMPDIR` at least 16 GiB (Q7); `task check` detached, once,
after C3, after its review settled and its keys moved. **Done when:** it exits
0 with every step printing its own completion line. A failure is fixed in a
new commit through the same cycle.

### Task 5 — The report (C4), the records, push

New `docs/reports/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it.md`
per the `implementation-report` skill, against the ticket's nine items, naming
C3 as the last code commit: rows with ids and tests, the refusals, QA
adjudications under their own heading, the break lines, the rule-9 answer,
the goldens and how each was made, the gaps as found or closed, the sign-off
answers. Its own review and commit. Then re-read SPEC-016 against the final
tree; commit only what moved. Fetch, merge if behind, push.

## Commits

| Commit | Carries | Gate steps |
|---|---|---|
| C1 | eyes are seen: `look`, the keystone's eyes, E, F, VTT-207 withdrawn, SPEC-016's eyes sentences | hook |
| C2 | testimony needs sight: memory, `classify`, the correction, the oracles' status clauses, eventgen's resource, the C2 goldens and `why`s, A to D and G to J, SPEC-016's payload sentences | hook |
| C3 | removal needs sight: ghosts, the bare removal, the removal fixture and its two tests, the oracles' removal clauses, K, L, SPEC-016's Status | hook; `task check` whole |
| C4 | the report | hook |

Every commit carries the ledger and keys D16 finds for it. Pre-push runs tiers
2 and 3, `check:drift` and `check:breaking`, about three minutes.

## Gaps that travel with this plan

1. **A forwarded status frame can name what the viewer did not see.** A
   `ResourceChanged` on the player's own character, forwarded because it sees
   its own character, carries `reason` `ability:<id>:<phase>` and the
   envelope's `participant_id` and `actor_role`, naming the ability and the
   participant behind an attacker the player cannot see, whose `AbilityUsed`
   is withheld. The ticket's rules do not reach it. Q6.
2. **Introductions carry a condition's source** unless Q2 says yes.
3. **A removed actor stays in the fold of a viewer that did not see it go**,
   a party member on a spectator's shoulder list included, until its id is
   introduced again; a perch on it is refused as an absent actor. Q1.
4. **The feed labels a re-introduction "left", then "joined"** (D7, D8);
   `describe` is unchanged, since the client gains no code.
5. **No committed golden corrects a resource or a condition, holds a ghost,
   or reuses an id** (measured). Those are held by unit tests, the keystone's
   generated seats and D14's walk; reuse and D7 by unit tests alone.
6. **Withheld events leave gaps in a seat's sequences**, as today; this ticket
   withholds more of them.
7. **An attack a player aims at a target it cannot see** is withheld from that
   player (row B); whether the command should be refused is SPEC-013's.
8. **`task check` cannot pass on this machine** until `TMPDIR` has 16 GiB
   free (Q7).

## Questions for sign-off

1. **A removal batch's `ActorRemoved` is judged at that event, so only the
   actor's controllers and a spectator perched on it are told; a viewer who
   watched its token go keeps it in the fold?** Recommend yes (D8): it is what
   "saw the actor when it was removed" means at the event, it matches VTT-205
   (a removed token reaches a watcher only as hidden), and the alternative —
   remembering that a token left the board by a removal in the event just
   before — ties the projection to `remove_actor`'s batch shape. The cost is
   Gap 3. Under the alternative, D10's fixture keeps its `ActorRemoved`.
2. **Strip a condition's source from introductions too (D11)?** Recommend
   yes, in C2: the source names the ability or threshold that applied a
   condition the viewer did not see, which ruling 3 keeps out of a correction;
   it moves one golden and one assertion.
3. **Control changes judged before the event only (D4), or before or after?**
   Recommend before only: one rule for every actor-naming payload, and no
   grant sent twice. Both pass every oracle (measured).
4. **Keep VTT-215 beside K?** Recommend keep: it is true as worded, and it
   guards a different failure (a fold refusing the removal of an actor it
   never held).
5. **`eventgen.addActor` declares a resource and the walk injects resource
   changes by step index (D14)?** Recommend yes: without it the walk never
   corrects a resource, and neither does the corpus. No draw is added, so the
   walks, the floors and `internal/campaign`'s property test do not move
   (measured).
6. **Gap 1 — a forwarded change's reason and author naming an unseen
   attacker — a follow-up ticket?** Recommend a follow-up ticket: stripping
   them changes what a viewer is told of events it does see, which no ruling
   has decided.
7. **May the implementer run `go clean -cache` before the whole gate?**
   Recommend yes: it is the gate's own remedy, and it frees about 7 GiB.
8. **Three code commits and a report, `task check` whole once after C3?**
   Recommend yes: C1 moves no golden and is reviewable alone; C2's correction
   and its arms cannot be split without an intermediate memory that tracks
   forwarded control (measured: a status-only C2 moves five goldens through
   duplicate grants); C3 is separable and owns the removal fixture.
