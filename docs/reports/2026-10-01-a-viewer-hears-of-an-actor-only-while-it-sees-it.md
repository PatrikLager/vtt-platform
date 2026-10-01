# A viewer hears of an actor only while it sees it: the change

**Ticket:** `docs/superpowers/specs/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it-design.md`,
corrected by its author at sign-off on four points the verification found:
"What it touches" gained `contract/testdata/removal_batch_projected_stream.json`,
`client/test/removal-batch-parity.test.ts` and `internal/eventgen`; the rule
about an actor coming into sight names its kind; the removal rule reads "saw
the actor before the removal"; and the rule about a reused id names the
viewer that missed the removal rather than every viewer.
**Plan:** `docs/superpowers/plans/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it.md`,
verified by `verify-ticket` (Passes with gaps). Its eight sign-off questions
were answered on 2026-10-01, all eight as the plan proposed: a removal
batch's `ActorRemoved` is judged at that event, so a viewer that watched the
token leave keeps the actor; introductions carry no condition source; a
control change is judged before the event only; VTT-215 is kept; eventgen's
actors carry a resource the walk changes; a forwarded change's reason and
author naming an unseen attacker is a ticket of its own; `go clean -cache`
before the whole gate; three code commits and a report, `task check` whole
once after the third.
**The owner's rulings** behind the ticket, of 2026-09-29 and 2026-10-01: only
what a viewer sees gives it information, for every actor, party members
included; a player's own characters and a spectator's shoulder are always
known; a party member nobody's eyes see stays on every roster with its status
frozen; an actor seen again is corrected on sight; an attack, ability or
control change naming an unseen actor is withheld entirely. During the work
the owner ruled once more, on C2's review: an actor's kind is status like the
rest.
**Last code commit:** `e75c416`, on `dcf901d`, `main` at the time. Every
code reference below is to that tree.

## The period, in commits

    git log --oneline dcf901d..e75c416

    e75c416 A viewer hears of an actor only while it sees it: removal
    390239a A viewer hears of an actor only while it sees it: testimony and the correction
    9705c43 A viewer hears of an actor only while it sees it: a player's own actors

`git diff --stat dcf901d..e75c416`: 37 files changed, 4799 insertions(+), 689
deletions(-).

The gate, `task check` whole, after `e75c416`: it ran twice. The first run
failed in `check:race` on `TestQAMCPConnectEndsASubprocessThatIgnoresSIGTERM`,
"the fixture never got to run", the late-starting shell of the
`TestQAMCPConnect*` deadline fixture that `docs/verification-debt.md` already
carries; no commit of this change touches `cmd/vtt`. The second exited 0 with
no step failed, and the check steps' own verdict lines read `check:comments`
clean over 250 files, `check:requirements-chain` 252 rows and 10
specifications, `check:doc-owner` 80 files, `check:new-prose` 3689 added lines
clean, `check:coverage` 20 packages at or above their floors, `check:no-pack`,
`check:no-retraction` and `check:no-create-scene` clean, `task lint` 0 issues,
`check:breaking` reporting pre-release with no objection from buf,
`check:mutation` 14 packages with zero unadjudicated survivors, and
`check:ts-mutation` 2909 mutants, 2808 killed, 70 survivors all adjudicated
equivalent and zero unadjudicated. 31 TypeScript mutants timed out and were
counted as killed, in `client/src/wire.ts`, `client/src/view/scene-plan.ts`,
`client/src/view/canvas.ts` and `client/src/session.ts`; six Go mutants carry
a timed-out verdict reused from their package's last passing run, since the
package did not change: two in `internal/sight`'s `Blockers`, two in its
`VisibleFrom`, one in `internal/rules`' lexer (`lexer.next`) and one in
`internal/mcp`'s `Run`. None of those files is in this change.

## Done looks like, answered

1. `[x]` A `ResourceChanged`, `ConditionApplied` or `ConditionRemoved` for an
   actor the viewer once saw and does not see now reaches no player or
   spectator:
   `internal/gateway/project_test.go#TestAConditionOnACreatureBehindAShutDoorReachesNoPlayer`
   and `internal/gateway/project_test.go#TestAPartyMemberInAnotherSceneIsNotHeardOf`
   (VTT-243), both red on `9705c43`'s projection.
2. `[x]` An attack, ability or control change naming any actor the viewer
   does not see now reaches no player or spectator:
   `internal/gateway/project_test.go#TestAnUnseenPartyMembersAttackReachesNoPlayer`
   (VTT-244) and
   `internal/gateway/project_test.go#TestAControlChangeOnAnUnseenPartyMemberReachesNoPlayer`
   (VTT-245), both red on `9705c43`'s projection.
3. `[x]` The same payloads naming only actors the viewer sees still reach it,
   and a viewer's eyes count as seen with or without a token. A seen party
   member's change: the status clause of
   `internal/gateway/keystone_test.go#TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees`
   and
   `internal/gateway/qa_testimony_sight_test.go#TestQATestimonySightEveryPairOfStepsKeepsEachSeatToWhatItSees`,
   whose forwarded path asserts it; no test in `project_test.go` names that
   case. The player's own token-less character:
   `internal/gateway/project_test.go#TestAPlayersOwnTokenlessCharacterIsHeardOf`
   (VTT-246) and
   `internal/gateway/qa_testimony_sight_test.go#TestQATestimonySightOwnActorsAndTheShoulderAreHeardWithoutAToken`.
   A deed naming only seen actors: the last step of
   `internal/gateway/project_test.go#TestAnUnseenPartyMembersAttackReachesNoPlayer`.
4. `[x]` A player granted a non-party actor with no token is introduced to it
   and receives the grant:
   `internal/gateway/project_test.go#TestAPlayerIsIntroducedToATokenlessActorItIsGranted`
   (VTT-241), red on `dcf901d`.
5. `[x]` A party member is still introduced to every player and spectator,
   seen or not: VTT-208's tests stay green, among them
   `internal/gateway/project_test.go#TestAPartyMemberStaysKnownEvenWhenOutOfSight`,
   and for an unperched spectator
   `internal/gateway/qa_testimony_eyes_test.go#TestQATestimonyEyesAViewpointGivesAPlayerNoEyesAndAnEmptyOneGivesASpectatorNone`
   (VTT-208).
6. `[x]` An actor coming into sight is brought to its present status by bare
   frames at that event's or the perch's sequence, the door opening on it in
   the first test:
   `internal/gateway/project_test.go#TestDamageTakenOutOfSightArrivesWhenTheEyeWalksIn`
   (VTT-248, VTT-249),
   `internal/gateway/viewpoint_internal_test.go#TestAPerchOntoAStaleShoulderCorrectsIt`
   (VTT-248), and the status clause of
   `internal/gateway/keystone_test.go#TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees`
   and
   `internal/gateway/project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`
   (VTT-247), both red on `9705c43`'s projection.
7. `[x]` An actor removed while the viewer does not see it is not reported,
   and its id used again still folds:
   `internal/gateway/project_test.go#TestARemovalOutOfSightIsNotReported`
   (VTT-251) and
   `internal/gateway/project_test.go#TestAnIdReusedAfterAnUnseenRemovalFoldsAgain`
   (VTT-252, VTT-211), both red on `390239a`'s projection.
8. `[x]` Every projected golden whose stream changed was re-derived: nine
   streams in `390239a`, five states by hand and five `why`s; the keystone,
   `client/test/projection-parity.test.ts` and `client/test/fold-parity.test.ts`
   pass. Three seats' streams did not move.
9. `[x]` SPEC-016 describes the new behaviour and its Status carries no
   pending decision (`grep -n 'not implemented'
   docs/specifications/016-the-projection.md` prints nothing); VTT-207 is
   withdrawn; every rule the sort accepted has a row cited by a test, and
   `python3 tools/check-requirements-chain.py .` holds over 252 rows;
   `task check` whole is green over `e75c416` (the period above).

## What the rules became

| Ticket's rule | Became |
|---|---|
| A resource or condition change reaches a player or spectator only while it sees the actor. | VTT-243 (row A) |
| An attack, an ability or a control change reaches a player or spectator only when it sees every actor the payload names. | VTT-244 (row B, attacks and abilities) and VTT-245 (row C, control changes) |
| A viewer's own eyes count as seen, with or without a token on a seen square. | VTT-246 (row D) |
| A player is introduced to every actor it controls. | VTT-241 (row E) |
| A party member is introduced to every player and spectator, seen or not. | refused as a new row: VTT-208 holds it |
| An actor that comes into sight is brought to its present resources, conditions, controllers and kind in the viewer's fold. | VTT-248 (row H) |
| A frame that brings an actor up to date carries no reason, source or author. | VTT-249 (row I) |
| An actor's removal reaches only a viewer that saw the actor before the removal. | VTT-251 (row K) |
| An actor id the world reuses is introduced afresh to a viewer that missed its removal. | refused as a new row (the plan's refusal 2): VTT-211 holds it and now cites `internal/gateway/project_test.go#TestAnIdReusedAfterAnUnseenRemovalFoldsAgain`; VTT-252 (row L) is the bare frame that lets it fold |

Three rows came from the plan rather than the ticket's list: VTT-242 (row F,
VTT-207's successor: an actor that is neither a party member nor one of the
viewer's eyes is introduced only once a token of it is seen), VTT-247 (row G,
the frozen status) and VTT-250 (row J, a change undone unseen sends
nothing). VTT-207 is withdrawn in VTT-035's form.

## The sort

Twelve rows accepted, A to L, and fifteen candidates refused, as the plan's
"The sort" lists them; none of the refusals was reversed.

## Phase 4a: QA adjudications

One line per finding, per commit's QA run, the commits named as the plan
names them; each QA agent was given the rows, the governing text and the
exported surface, never the diff, the source or the existing tests.

**C1, `9705c43` (VTT-241, VTT-242), QA on opus, from SPEC-016 and SPEC-015
whole as C1 leaves them, the register and the exported surface; in-process**
11 tests (internal/gateway/qa_testimony_eyes_test.go), all green; 18
injections red; the VTT-222 state and event checks, two kind assertions and
one demotion step uninjected.
- QA finding 1 (spec gap: no row says a player keeps an actor in its fold once
  its control is revoked): no defect of this commit — SPEC-016's "transitions
  removes one only when st no longer holds it" says so today, and the
  testimony ruling (the plan's C2 and C3) decides what the viewer is told of
  it afterwards — no change.
- QA finding 2 (spec gap: a grant before the actor is added cannot be built):
  no defect — both folds refuse it (VTT-152 at the command) — no change.
- QA finding 3 (spec gap: a demotion of an actor the viewer holds): no defect
  of this commit — inside the pending ruling, which C2's correction on sight
  carries (the plan's D6, D7) — no change.
- QA finding 4 (requirements needing an ID: the projector's retention
  sentence; `knows` for control changes; the DM's stream as the event itself
  by pointer): no new rows — the first two are what C2 replaces (rows C and G
  of the plan), the third is VTT-176's "unchanged", which its tests pin by
  pointer.

**C2, `390239a` (VTT-243 to VTT-250), QA on opus, from SPEC-016 and SPEC-015
whole as C2 leaves them, the register and the exported surface; in-process**
9 tests (internal/gateway/qa_testimony_sight_test.go: every ordered pair from
a 29-step set, 40 seeded walks of 40 steps, seven player and spectator seats
plus the DM and the agent, an independent oracle on every event), all green;
23 injections red; VTT-213 and the zero-frames rule for an unseen control
change never shown red.
- QA finding 1 (spec gap: no exported perch entry, so VTT-186 and the perch
  half of VTT-248 cannot be driven): no defect — the perch is the seat's,
  internal to the package; internal/gateway/viewpoint_internal_test.go's
  TestAPerchOntoAStaleShoulderCorrectsIt holds VTT-248's perch half — no
  change.
- QA finding 2 (spec ambiguity: VTT-249 does not say whether a correction may
  carry occurred_at): spec ambiguity — corrections carry none, and SPEC-016's
  correction sentence now says "nothing else of an envelope, no event id,
  time, role, participant or session"; the row stands as worded.
- QA finding 3 (spec ambiguity: VTT-247's "the last event at which the viewer
  saw it", before or after the event): spec ambiguity resolved by SPEC-016's
  memory paragraph, which states the snapshot for an actor in sighted (before)
  or the new sees (after); QA's oracle reads it so and agrees — no change to
  the row.
- QA finding 4 (spec gap: the through-zero step rests on the fold's floor and
  an uncapped maximum at or below zero): no defect — SPEC-016 names the floor;
  the uncapped maximum is how such a value arises, engine.Apply's and not the
  projection's — no change.
- QA finding 5 (requirements needing an ID: the forwarded event last; the
  correction's frame order; the re-statement; the re-introduction; the
  through-zero pair; zero frames for a withheld change): no new rows — the
  first is SPEC-016's existing ordering, the next four are how VTT-248 is
  achieved (the plan's refusals 11 to 13), and the last is what VTT-243 to
  VTT-245's "reaches only when" means, which their tests assert as an empty
  frame list.

**C3, `e75c416` (VTT-251, VTT-252), QA on opus, from SPEC-016 and SPEC-015
whole as C3 leaves them, the register and the exported surface; in-process**
13 tests (internal/gateway/qa_testimony_removal_test.go: 1032 sequences,
exhaustive depth 3 from three prefixes and 80 seeded walks of 16, an
independent sight oracle; paired worlds showing a viewer's stream
byte-identical with and without an unseen removal, whatever its time), all
green; 15 injections red; the bare-to-non-holder branch, two source and reason
variants and VTT-221 on non-bare frames uninjected.
- QA finding 1 (spec gap: a forwarded ActorRemoved can reach only a viewer
  whose eye the actor is, which SPEC-016 does not say): spec ambiguity —
  SPEC-016 gains the Consequence, with its reason (the fold's refusal while a
  token stands, remove_actor's order).
- QA finding 2 (spec gap: "bare" undefined for ActorRemoved): spec ambiguity —
  SPEC-016's transitions sentence now says withdraw's frame carries the
  actor's id and the event's sequence and nothing else.
- QA finding 3 (spec gap: a spectator whose shoulder's id is reused for a
  party member gets eyes on the new actor without perching again): no defect
  of this change — the viewpoint names an id (SPEC-015) and eyes reads it from
  the state; named for the report — no change.
- QA finding 4 (requirements needing an ID: the positive direction of the
  removal ruling; the bare frame as the projection's own; an actor out of the
  world held until its id returns; an unseen removal leaving the stream
  identical): no new rows — the first is VTT-251's other half, which its test
  holds; the second and third are SPEC-016 Consequences held by VTT-252's
  tests; the fourth is what VTT-251 means for the viewer, held by QA's
  paired-world tests.

## The breaks

Each commit's message carries its breaks, one line each, with the check that
spoke; every break was reverted and the repaired tree ran clean. In summary:
`9705c43` three (the eyes taken out of the look, every controlled actor known,
every token-less controlled actor known); `390239a` twelve (the status,
testimony and control arms each on the old rule, the eyes out of `sees`, no
correction, a correction carrying a reason, every resource sent, conditions
corrected by order, no re-introduction, no pass through zero, `>=` at the
int32 edge, an introduction carrying its source); `e75c416` three (removal
forwarded to every holder, no bare removal before a reused id, a ghost
forgotten anyway).

## Rule 9: how MapTool does this

Answered in the plan, before any task was dispatched, from
`~/dev/RPTool/maptool` at `f4b7fef6c`. MapTool sends every client the whole
campaign and relays nearly every message to all, so its clients never hold a
stale actor and it has no status frozen while unseen and no correction on
sight: it does not solve this, because it sends everything. Its map hides an
unseen token, bars and states included; its initiative list
(`InitiativeListModel.isTokenVisible`) does not, with no sight test at all.
Borrowed: a viewer keyed by role and the set of tokens it sees through, which
is `Viewer` plus `eyes`, and individual views as the only mode. Refused: the
distribution, the initiative list's unseen bars and states, which are the
exact leak this change closes, and client-computed exposure.

## The goldens, and how each was made

| Golden | Commit | Made |
|---|---|---|
| nine `projections/*/stream.json`: `door-watch/act-latecomer`, `door-watch/act-watcher`, `session-zero/act-fighter`, `session-zero/player`, `session-zero/spectator`, `shared-control/act-scout`, `shared-control/act-warden`, `toy-brawl/act-brawler`, `toy-brawl/act-patron` | `390239a` | the projection's emitted stream, read frame by frame against the rulings before commit |
| `session-zero/act-fighter`, `session-zero/spectator`, `shared-control/act-scout` `state.json` | `390239a` | by hand: an unseen party member keeps the controllers it was introduced with, none |
| `shared-control/act-warden/state.json` | `390239a` | by hand: its head sequence is 12 |
| `door-watch/act-watcher/state.json` | `390239a` | by hand: `dazed-by-ale` without its source |
| five `viewer.json` `why`s, act-scout's rewritten twice | `390239a` | rewritten where they described the old arms |
| `contract/testdata/removal_batch_projected_stream.json` | `e75c416` | by hand, then held to the emitted stream |

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the plan as signed off (D17 and Task 1, corrected in C1 before its commit): C1 adds `sees` and moves "Whose eyes" | C1 added the eyes to the look's actors alone; `sees` arrived in C2, and "Whose eyes" stayed true | `sees` is read only by C2's arms, and C1's reading review found the plan naming work C1 did not do; the plan was corrected before it was committed in C1 |
| Row F held by VTT-207's four tests | VTT-242's evidence gained `internal/gateway/qa_testimony_eyes_test.go#TestQATestimonyEyesAnActorGrantedToTwoPlayersIsIntroducedToBothWithBothControllers` and the keystone | all four tests place a token behind a door, so a projection introducing any token-less controlled actor left them green; C1's review measured it |
| D15's break for row J, conditions corrected by order, reddening `TestAChangeUndoneOutOfSightSendsNothingOnSight` | that test cannot see it; `internal/gateway/project_test.go#TestConditionsReorderedOutOfSightSendNothingOnSight` was added in C2 | the test's rogue holds no condition to reorder; C2's reading review measured the plan's break leaving the package green |
| C2's correction pinned by its four H tests | `internal/gateway/project_test.go#TestADifferenceOfExactlyMaxInt32IsOneCorrection` was added | C2's review found `d > math.MaxInt32` mutated to `>=` surviving the package, which the mutation gate would have kept |
| an actor's kind corrected like the rest of its status, with nothing more said | the owner ruled on C2's review that kind is status like the rest, and SPEC-016 gained the Consequence that two viewers' rosters can differ and a spectator can be offered a shoulder no longer a party member | a viewer holding an actor keeps its last-seen kind while one that never held it is introduced to a new party member with its present status; no ruling had covered it |
| D13: the keystone's removal clause holds row K | the keystone runs the clause over no removal; VTT-251 cites the property walk and unit tests, and `docs/verification-debt.md` records the gap | no golden log carries an `ActorRemoved` (`grep -l actorRemoved scenarios/goldens/*/stream.json` prints nothing); C3's review counted the clause's runs |
| D13's status clause as C2 wrote it | `keystoneStatusDiff` holds a ghost's status until its id is knowable again | C3's review found the oracle forgetting a ghost's status when the world drops the id, so a ghost whose id is reused unseen would have read as a failure; latent, since nothing in the corpus or the walk reuses an id |
| the edited files' comments otherwise untouched | the keystone oracle's doc (99 lines), the property test's doc, `assertSound`'s doc, and the doc blocks of `TestAConditionAppliedOutOfSightArrivesWithTheActor`, `TestARemovedActorReachesOnlyTheSeatThatHeldIt` and `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh` became warnings or pointers | each held a sentence the change made false, and SPEC-010 refuses an edit inside a block over its bound; the owner chose to shrink the keystone's when C1's review raised it, and the rest followed that choice |
| `perchFixtureLog` used as it stood | its sequence-7 grant now reaches the seat | it had no payload arm for a grant, so the hero was never granted; C2's review found it, and every test stayed green with it fixed |
| QA's C2 file committed as written | its resource renamed from `hp` to `pool` | the vocabulary gate refused the first commit |
| QA's files committed as QA wrote them | the implementer changed three things in them: `qaEyesControllers` takes the rule it reports (C1's review), its other-seat expectation for a token-less party member became "no controller" (C2, since VTT-247 freezes it), and the C2 file's long citation lines were split to pass `check:new-prose` | the C1 expectation pinned C1's behaviour, which the ruling changes in C2; the rest are gate and review fixes. The C3 file is QA's own final version |
| Task 4: `task check` whole once after C3, a failure fixed in a new commit | the first run failed in `check:race` on `TestQAMCPConnectEndsASubprocessThatIgnoresSIGTERM`; the same tree was run again and passed | the failure is the deadline fixture's late shell, which `docs/verification-debt.md` carries and to which this entry adds the sighting; no commit here touches `cmd/vtt`, and nothing in the tree was wrong |

## What could not be established

- **A forwarded status change can name what the viewer did not see.** A
  `ResourceChanged` on a player's own character, forwarded because it sees
  that character, carries `reason` and the envelope's author, naming the
  ability and the participant behind an attacker it cannot see. The owner made
  it a ticket of its own (Q6); none is written yet, and it is raised to the
  owner with this report.
- **No golden corrects a resource or a condition, holds a ghost, or reuses an
  id.** Those cases are held by unit tests, QA's sequences and the property
  walk, and reuse by unit tests and QA's removal tests; the keystone's share
  is in `docs/verification-debt.md`.
- **A spectator whose shoulder's id is used again for a party member sees
  through the new actor without perching again**, since its viewpoint names an
  id. QA on `e75c416` found it; SPEC-015 is where it would be stated. No ticket
  is written; it is raised to the owner with this report.
- **A player may name an actor it does not see as an attack's target**, since
  no command checks sight but a move; such an attack reaches the attacker's
  player only if it sees the target. That is SPEC-013's question. No ticket is
  written; it is raised to the owner with this report.
- **A correction applies conditions and controllers in the server's order**,
  so it shows the order of applications the viewer did not see; the plan's D5
  chose it, and an introduction already shows the same order.
- **`check:doc-owner` skips test files**, so a test inserted between another
  test's doc comment and its function passes the gate; it happened in C1 of
  this change and in the note-visibility change before it, and both were
  caught only by reading. `docs/verification-debt.md` carries it.
- **Withheld events leave gaps in a seat's sequences**, as they did before;
  this change withholds more of them (the plan's Gap 6). Nothing reads the
  gaps.
- **`internal/gateway/scenario_test.go` is not gofmt-clean** on `dcf901d` and
  on `e75c416` alike; no gate runs gofmt, and the change that next edits the
  file is where a gofmt run belongs. The disk the plan's Gap 8 warned of was
  answered by `go clean -cache` before each gate run (Q7).

## What was deliberately left out, and where it went

- The forwarded status change's reason and author (the plan's Q6): a ticket
  of its own, not yet written, raised to the owner with this report.
- A client view of an actor's last-seen status, or of a ghost: the client
  folds what it is sent and gained no code.
- The feed's labels: a re-introduction reads "left" then "joined"; `describe`
  is unchanged.
- The keystone's missing removal and reuse corpus: a debt entry in
  `docs/verification-debt.md`.
- `check:doc-owner`'s blindness to test files: a debt entry in
  `docs/verification-debt.md`.
