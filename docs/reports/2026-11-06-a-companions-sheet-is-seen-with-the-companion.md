# A companion's sheet is seen with the companion: the change

**Ticket:** `docs/superpowers/specs/2026-11-06-a-companions-sheet-is-seen-with-the-companion-design.md`.
An agent that did not write it verified it and passed it with gaps.
**Plan:** `docs/superpowers/plans/2026-11-06-a-companions-sheet-is-seen-with-the-companion.md`,
written by that verifier and amended by Task 0's review.
**Where it came from:** ADR-012 §2's visibility per property, made the ticket
after the vocabulary by the owner's ruling of 2026-11-05
(`docs/reports/2026-11-05-a-ruleset-names-its-properties.md`). The owner set
that design aside on 2026-11-06, as too much for "ett spel bland vänner". It
is parked on the local branch `backup/per-property-visibility`, and nothing
here reads from it.
**The owner's rulings:**
- 2026-11-06, the rule: "En spelare som "ser" sin medspelare, kan se dess
  character sheet, allt, för att göra det enkelt. En spelare som inte ser sin
  medspelare, kan inte se character sheet, för du kan inte se "karaktären" -
  DM ser allt. NPC och monster, kan endast ses av DM".
- 2026-11-06, at sign-off ("ok, kör"): a companion's name is not part of its
  sheet (the plan's G6).
- 2026-11-06, on how a sheet is shown: on demand, by a right-click "Show
  character sheet", and only of a companion the viewer's character has seen.
  That view belongs to the sheet work; this ticket adds no view.
- 2026-10-21: a near-theoretical edge with a small consequence gets no code,
  no test and no debt entry. It decides D7, D12 and Task 2's QA finding 1.
- 2026-09-04: nobody uses the product yet, so nothing is migrated (D8).

**The last code commit is `516fe23`.** Every code reference below is to that
tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `1479471` | Task 0: the ticket, the plan, VTT-657 to VTT-663; VTT-485, VTT-471, VTT-449 and VTT-317 withdrawn; VTT-634 to VTT-656 withdrawn as placeholders |
| `5534066` | Task 1: the projection, its tests at every layer, the id re-points, the independent QA stage's Go tests |
| `72601c0` | Task 2: the Story leaves the first sight's run out; the rebuilt `cmd/vtt/webdist`; the QA stage's client tests |
| `3b0c1ef` | Task 3: the comment ledger, written once by `check-comments.py --write-ledger`. It lowers `client/src/view/feed.ts`, and `internal/adventure/load.go` for the comment `84c4ac8` shortened after the last ledger |
| `bcb7469` | Task 3: the gate's finding, a test fixture this ticket's change had made intermittent, and its recipe in `docs/verification-debt.md` |
| `516fe23` | Task 5's review: VTT-659 withdrawn and succeeded by VTT-664, every citation in `internal/` re-pointed; the committed plan keeps the old id |

## What the code does at `516fe23`

- **An unseen companion.** `introduce` in `internal/perceive/projection.go`
  takes whether the look sees the actor. `transitions` passes `now.sees[id]`;
  `correct`, which runs only in sight, passes true. An actor not seen is sent
  as `unseenAs` gives it: `shownWith(a, false)` with its name, so its id,
  name, module id and kind, its controllers as grants behind it, and no
  attributes, resources, module data or conditions (VTT-658). A creature is
  in a look's `actors` only when it is in `sees`, so `introduce` gives
  `unseenAs`'s form only to party members; its doc warns never to introduce a
  creature in that form.
- **The first sight.** `recall` rebuilds the projection's memory from the
  log's fold at every event. A held actor that agrees with `unseenAs` of the
  world's but not with `shownAs` of it is marked `reshaped`, and `transitions`
  withdraws a reshaped actor and introduces it whole only in sight. So at the
  first event the eyes see a companion with something to strip, its log is
  written a bare `ActorRemoved`, the whole `ActorAdded`, its grants, its
  conditions and then its `TokenPlaced` (VTT-664, VTT-661). A companion with
  nothing to strip is never reshaped, and `correct` brings it to its present.
- **`recall`'s gone test.** `recall` marks a held actor that the world still
  holds gone, to be withdrawn before it is introduced again, only when it
  matches neither the world's actor as its held kind shows it nor `unseenAs`
  of it (VTT-470, VTT-661).
- **Unseen and never seen.** Nothing that carries the companion's values
  reaches the log: `classify`'s `ResourceChanged` arm still requires `saw`
  (VTT-486), and the held form has none to correct (VTT-660).
- **The opening.** `opening` in `internal/campaign/fanout.go` perches through
  no eyes, so it introduces each party member as `unseenAs` gives it
  (VTT-663).
- **`Resume`** refuses a fold that does not hold an actor in sight as
  `shownAs` gives the world's actor, at its present status; the unseen form
  of a companion with something to strip is one such fold. The next event
  then re-introduces that companion, and `correct` brings any other to its
  present.
- **The Story.** `buildFeed` in `client/src/view/feed.ts` leaves out, before
  it groups, the runs `reintroductions` names: an `actorRemoved` with no event
  id, the `actorAdded` of the same actor directly after it, and that actor's
  `actorControlGranted` and `conditionApplied` entries with no event id
  directly behind (VTT-662). The fold, the namer and the Events panel are
  handed the whole log.

## Done looks like, answered

1. `[x]` An unseen companion arrives without its sheet:
   `TestACompanionItsCharacterHasNotSeenArrivesWithoutItsSheet` in
   `internal/perceive/companion_sheet_test.go`. On `1479471`'s
   `projection.go` it failed, reporting the ally introduced with its
   attributes, resources and module data, and its condition behind it.
2. `[x]` The sheet arrives with the first sight:
   `TestACompanionsSheetArrivesWhenItsCharacterFirstSeesIt`, a case for a
   companion with attributes and module data, one with resources and one with
   neither. On `1479471`'s `projection.go` each case failed: for the cases
   with attributes and module data or with resources, the sheet had arrived
   at the introduction, and for the one with neither its condition had.
3. `[~]` The window tells the truth about it, for every log the window holds
   whole: "at a companion's first sight the Story shows it coming into view
   and neither its leaving nor its joining" in
   `client/test/spectator-view.test.ts`, red on `5534066`'s `feed.ts`. What
   moved: a live window folds and paints each entry as it arrives
   (`Session.ingest`, then `paint`), so between the bare removal and the add
   behind it the Story can draw "A left" for a moment. QA's test "%s: entries
   arriving one at a time end with the run untold" in
   `client/test/qa-companion-first-sight.test.ts` holds both halves. Recorded
   under the 2026-10-21 ruling, with no code.
4. `[~]` What already holds still holds, with some assertions changed where
   the rule or the fixture moved them, and a fixture changed another way.
   Observation: item 6's gate, or
   `go test ./internal/perceive/ ./internal/campaign/ ./internal/gateway/`.
   - Fixtures changed so the viewer sees the companion first, assertions kept:
     `TestQACSAKindChangeOutOfSightWaitsUntilTheActorComesIntoSight`,
     `TestAResourceStartedBelowZeroIsCorrectedOnSight` and
     `TestADifferenceOfExactlyMaxInt32IsOneCorrection`.
   - Assertions changed:
     - `TestQALogALateLogOpensWithThePresent` and
       `TestQALogALogOpensForAPartyMemberThatHadNone`: p's opening now holds q
       without its resources (VTT-663), and the pool check moved to q's own
       log (`qaLogHoldsItsOwnPool`, VTT-657). The first also checks q's
       controllers in p's opening.
     - `TestQACSAPartyMemberAddedAgainIsIntroducedAfreshOrCorrected`: its
       check is split, the add's frames checked for the unseen form (VTT-470)
       and the placement's for the whole form (VTT-664).
     - `TestQACSAPartyMemberIsIntroducedWholeOnlyInSight`, renamed from
       `TestQACSAPartyMemberIsIntroducedWhole`: the hero is whole, the friend
       unseen.
     - `TestARemovedActorWhoseIdNeverReturnsStaysInTheFold`: a removal of the
       rogue is allowed only as the bare one directly before its introduction
       at its first sight.
     - `TestAnActorAddedAgainWithAnotherAttributeIsIntroducedAfresh`: it now
       checks that the first frames naming the new scout are a bare
       `ActorRemoved` with its `ActorAdded` behind it, and the weight the fold
       ends holding.
   - Fixture changed another way, assertions kept:
     `TestAWedgedWatcherIsTornDownAndOthersKeepServing` (Deviations).
5. `[x]` End to end: `TestASeatIsSentNoCompanionsSheetBeforeItsCharacterSeesIt`
   in `internal/gateway/companion_sheet_test.go`. On `1479471`'s
   `projection.go` both the player and the spectator were sent frames
   carrying the companions' attributes, resources and module data before
   sight.
6. `[x]` The gate passes: `task check` on `516fe23` exits 0, with
   `check:mutation`'s "zero unadjudicated survivors" (log written outside the
   repository). The runs before it:
   - on `3b0c1ef`, the first failed in `check:mutation` (below, under What
     could not be established);
   - on `3b0c1ef`, the second failed in `check:coverage` on
     `TestAWedgedWatcherIsTornDownAndOthersKeepServing`, the fixture this
     ticket had made intermittent, fixed by `bcb7469`;
   - on `bcb7469`, the third was stopped in `check:mutation` when Task 5's
     review changed the register, since its result would not hold for the
     tree after `516fe23`.

## What the rules became

- "A character log is introduced to a party member its character has not
  seen with that party member's id, name, kind, module id and controllers
  alone": VTT-658, with VTT-657 for the party member its eyes see; they
  succeed VTT-485. The rows speak of the eyes that see the party member at
  its introduction, not of whether the character ever saw it, which is why an
  id the world adds again as another is unseen again (Task 1's QA finding 1).
- "A character log is brought to a party member's whole sheet at the event
  at which its character first sees that party member": VTT-664, with VTT-661
  for the bare `ActorRemoved` that brings it. VTT-664 is narrowed to an actor
  held without attributes, resources or module data while the world's actor
  is a party member with some, at the first event since the world last added
  it; a companion with nothing to strip is corrected on sight, by VTT-469.
  Task 0 dispensed
  the rule as VTT-659, which Task 5's review found false as worded: a party
  member the log never saw that the world makes a creature out of its sight
  is introduced at its first sight as a creature, without its name or sheet
  (VTT-481, VTT-465), as `TestQASheetAnUnseenCompanionMadeACreatureArrivesAsOne`
  pins. VTT-659 is withdrawn and succeeded by VTT-664, which holds only while
  the world's actor is a party member.
- "A window watching a character log does not describe … the withdrawal and
  re-introduction that bring it a companion's sheet": VTT-662, succeeding
  VTT-449. It is wider than the rule: every such run is left out, a
  creature's and a kind change's included (D12).
- The narrowed rows: VTT-471 is succeeded by VTT-660, and VTT-317 by VTT-663.
- VTT-634 to VTT-656 hold withdrawn placeholder rows: they were dispensed and
  committed on the parked branch, and SPEC-008 spends a committed number.

## Phase 4a

A QA agent for each task, given the rows, the ticket, the governing
specifications and the exported surface, never the source, the diff or the
existing tests. No behaviour defect was found in either task. Its tests are
committed with the task: `internal/perceive/qa_companion_sheet_test.go`,
`internal/campaign/qa_companion_sheet_test.go` and
`client/test/qa-companion-first-sight.test.ts`.

### QA adjudications

- Task 1, QA finding 1: spec ambiguity — a seen companion removed and added
  again, unseen, as another actor is held stripped from the re-add; an id the
  world adds again as another is an actor the viewer never saw (VTT-470,
  VTT-658). No change; SPEC-016 is to say so at Task 5.
- Task 1, QA finding 2: spec gap, unreachable — a `Perch` onto eyes that see a
  stripped companion brings the whole sheet, and no row says so; `Perch`'s
  only caller outside tests is `opening`, through no eyes. No row.
- Task 1, QA finding 3: no change — a first sight whose frames were dropped
  brings the sheet at the next event after `Resume`;
  `TestQASheetAFirstSightWhoseFramesWereDroppedBringsTheSheetNext` asserts it
  as a regression test holding no row (Deviations).
- Task 1, QA finding 4: no change — when an unseen party member is introduced
  is VTT-361's: every active character log introduces every party member,
  seen or not.
- Task 1, QA finding 5: no change — the ticket's item 5 has no row of its own;
  `TestASeatIsSentNoCompanionsSheetBeforeItsCharacterSeesIt` holds it under
  VTT-658 and VTT-664.
- Task 1, QA finding 6: no change — VTT-176 speaks of a DM's or agent's
  connection, which `internal/campaign` cannot dial; the gateway tests in its
  evidence hold it. QA's `TestQASheetTheWorldLogKeepsEveryEventUnchanged`
  carries the id but observes the world log, so it is not cited.
- Task 2, QA finding 1: spec gap, cosmetic — the momentary "A left" of item 3.
  No code and no debt entry, by the 2026-10-21 ruling; QA's test pins the
  momentary line as it is.
- Task 2, QA finding 2: no change — a creature's run is left out too: every
  re-introduction gets the same treatment (D12).
- Task 2, QA finding 3: no change, unreachable — a bare removal followed by an
  add that carries an event id would be left out with it; `classify` forwards
  no `ActorAdded`.
- Task 2, QA finding 4: no change — "directly behind it" reads as an unbroken
  chain behind the add, which is the order `introduce` writes.
- Task 2, QA finding 5: no change — a narration anchored only to entries the
  Story leaves out is drawn in a beat of its own.

## Phase 4b

A reviewer for each task, on that task's diff, reading each round's fixes
again until a round came back clean. What the readings changed:

- **Task 1.**
  - A loosened check: `TestARemovedActorWhoseIdNeverReturnsStaysInTheFold`
    had come to accept a bare removal and re-introduction of the rogue at any
    step before its removal. It now accepts one at the rogue's first sight
    alone.
  - K7, as the plan worded it (`Resume` accepting the unseen form at its
    incarnation check), was red only under a stronger break: the planned
    test's companion had a resource, so `statusFrames` refused first.
    `TestAResumedLogHoldingAnUnseenCompanionInSightIsBroughtToItsSheet` gained
    a case with no resource, which the weak break turns red.
  - `TestQALogALateLogOpensWithThePresent` gained a present-state check that
    the unseen form keeps: the companion's controllers in the opening.
  - `TestQACSAPartyMemberIsIntroducedWhole` was renamed, since its second half
    now asserts the opposite of its old name.
  - The QA tests gained id lines, all but
    `TestQASheetAFirstSightWhoseFramesWereDroppedBringsTheSheetNext`, which
    holds no row.
  - `unseenAs`'s warning was first written as "never pass it a creature",
    which `recall`'s comparison contradicts; it is scoped to introducing.
- **Task 2.**
  - `feed.ts`'s header said `describe` is handed the whole log; `describe`
    takes one entry.
  - The narration fallback's comment was an argument, and named a cause, a
    late join, that a log read from its first entry cannot have.
  - A mutant no test caught, any entry with no event id opening a run, is now
    caught by "an entry with no event id that is not a removal is told with
    the add behind it" in `client/test/feed.test.ts`.
  - QA's "a narration anchored over the run and the arrival is drawn with it"
    now asserts the narration's beat.
- **The gate's finding** was read by Task 1's reviewer, and the withdrawal of
  VTT-659 by the specifications' reviewer, each before its commit.

## The deliberate breaks

T1 to T6 are the plan's names for the new tests:
`TestACompanionItsCharacterHasNotSeenArrivesWithoutItsSheet` (T1),
`TestACompanionsSheetArrivesWhenItsCharacterFirstSeesIt` (T2),
`TestAResumedLogHoldingAnUnseenCompanionInSightIsBroughtToItsSheet` (T3),
`TestASeatIsSentNoCompanionsSheetBeforeItsCharacterSeesIt` (T4), "at a
companion's first sight the Story shows it coming into view and neither its
leaving nor its joining" (T5) and "a removal that carries an event id, or that
another actor's introduction follows, is told" (T6). Each break was applied
by exact text after checking the broken text was absent, checked landed, run,
and undone by its inverse edit.

| Break | What went red |
|---|---|
| K1 `introduce` always whole | T1, T4, `TestQACSAPartyMemberIsIntroducedWholeOnlyInSight`, `TestQALogALateLogOpensWithThePresent`, `TestQALogALogOpensForAPartyMemberThatHadNone` |
| K2 conditions sent unseen | T1 |
| K3 no `unseenAs` half in `recall`'s gone test | T1: a removal at every unseen change |
| K4 the unseen form left unmarked as reshaped | T2's case with attributes and module data and no resource |
| K5 `unseenAs` without the name | T1 |
| K6 always `unseenAs` | `TestQACSAPartyMemberIsIntroducedWholeOnlyInSight`, T2's whole introductions |
| K7 `Resume` accepting the unseen form in sight | T3; its weak form only T3's case with no resource |
| K8 `buildFeed` leaves nothing out | T5 |
| K9 the run opened whatever the removal's event id | T6's world case |
| K10 the run ending at the `actorAdded` | T5 |
| K11 the `actorAdded` matched whatever its actor | T6's other-actor case |

And the breaks the reviews asked for:
- the first-sight index set past the rogue's first sight, which reds
  `TestARemovedActorWhoseIdNeverReturnsStaysInTheFold`;
- the controllers expected in `TestQALogALateLogOpensWithThePresent`
  changed, which reds that test;
- the removal check in `reintroductions` deleted, which reds the new
  `feed.test.ts` test;
- narration anchoring switched off, which reds QA's narration test.

## Rule 9: how MapTool does this

MapTool sends every client the whole campaign, every token's properties
included. `TokenProperty`'s `ownerOnly` and `gmOnly` filter what a stat sheet
shows in the client (`net.rptools.maptool.model.TokenProperty`, MapTool at
`f4b7fef6c`). That is the per-property design the owner set aside, and a
filter over data already sent.

The nearest precedent for "has this character seen that one" is MapTool's
individual fog of war: `Zone` keeps, for each token, the area it has exposed
(`exposedAreaMeta`, `ExposedAreaMetaData`). It does not fit. It remembers
an area of the map, not actors, and it is a filter in the client over a
campaign every client already holds. Nothing is borrowed.

## The plan's gaps

- **G1 to G5** were closed in the ticket's revision before the plan.
- **G6, the name.** Kept as the ticket reads it, signed off by the owner: a
  log knows every party member by name.
- **G7.** VTT-449 withdrawn and succeeded by VTT-662 at Task 0.
- **G8.** The Events panel is drawn only for the DM and the agent, whose
  windows hold the world log, so `renderTicker` is unchanged (D13).
- **G9.** The change is in `buildFeed`; `describe` keeps every payload's
  words.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| T3 "(R3, VTT-325): a fold that holds the companion unseen, as after a dropped frame set … Green today" | `TestAResumedLogHoldingAnUnseenCompanionInSightIsBroughtToItsSheet` and QA's `TestQASheetAFirstSightWhoseFramesWereDroppedBringsTheSheetNext` hold no row, and T3's fold is built by hand | Both show the sheet arriving at the event after a dropped first sight. VTT-664 says "at the first event … at which the log's eyes see it", which a dropped first sight makes false. VTT-320 speaks of what the log receives for the dropped event, and VTT-325 of an append that fails; neither perceive test asserts either. They stay as regression tests. Built through the projection, T3 would be red on `1479471`, against "Green today" |
| The plan's Phase 4a brief: "`semgrep` bans game words in tests too" | Task 1's QA brief left semgrep out, and its Go fixtures used a resource key `.semgrep/vocabulary.yml` refuses | renamed to `pool` before review; the rule covers Go under `internal/` and `cmd/` alone |
| `TestQACSAPartyMemberIsIntroducedWhole`, as the plan names it | renamed `TestQACSAPartyMemberIsIntroducedWholeOnlyInSight` | its second half now asserts the friend unseen; the committed plan keeps the old name |
| Ticket item 4: a fixture that holds whole a companion its viewer never saw "changes so the viewer sees it first" | `TestAWedgedWatcherIsTornDownAndOthersKeepServing` narrates instead | the test is about a wedged connection and needs only a large entry its player's log receives. Its padded companions now arrive unseen and small, so its socket filled only by chance; narration reaches every log whatever its character sees (VTT-362). The recipe is in `docs/verification-debt.md` |
| "every existing client test unedited but its id lines" | held; `feed.test.ts` also gained a test the review asked for | a mutant of `reintroductions` survived every test |
| "the client's lint" | not run | the client has none: `Taskfile.yml`'s `lint` is golangci-lint over Go |
| `feed.ts`'s header rewritten to point at VTT-662 | the whole header rewritten | `check-comments.py` refuses a touched comment block over its bound, and the header carried history: it said the feed once hid what a retraction marker covered, and argued why an anchored narration is grouped with the events it describes |
| D12: "none of them is a departure: "X left" and a second "X is in the party" are false for each" | for a creature that becomes a party member in sight, the run removes a true "X is in the party" | the log introduced it as a creature, so no earlier line said it joined. The forwarded grant still draws "control of X given". Accepted: the line lost is the creature's first "X is in the party", in a rare move |

## What could not be established

- How long a live window shows the momentary "A left": a browser was not
  measured.
- The client's mutation: this worktree holds no stored report
  (`reports/mutation/mutation.json`), so `check:ts-mutation` measured none;
  `task check:ts-mutation:full` measures it when the owner asks for it.
- Mutants counted as killed because they timed out were not measured. The
  mutant that timed out in `internal/perceive` sits in `objectInSight`, which
  this ticket did not change: `y++` made `y--`, a loop that runs until `y`
  wraps round, far past the deadline.
- A mutant of `reintroductions` that no test catches stays for the owner's
  next full client mutation run, where it is to be adjudicated in
  `tools/ts-mutation-equivalents.txt`: deleting `back.case !== "actorAdded"`,
  whose only effect is a throw on an unset payload behind a bare removal,
  which the server never writes.
- Why the first run found no survivor under an adjudicated key. On
  `3b0c1ef`, `check:mutation` found no surviving or timed-out mutant under
  the adjudicated equivalent in `internal/gateway`'s `viewpoint.go`, its sort
  of `set.Actors` by actor id with `<` made `<=`, and failed on the stale
  entry. Made by hand on a copy of that tree, the mutant survives `go test
  -count=1 ./internal/gateway/`, and it survived the run on `516fe23`, so its
  entry in `tools/mutation-equivalents.txt` stands. This ticket does not touch
  `viewpoint.go`. Which test failed was not established: gremlins discards
  each mutant's test output, and its verdict line names no test. A candidate
  is `TestAWedgedWatcherIsTornDownAndOthersKeepServing`, intermittent on that
  tree (Deviations), which `bcb7469` fixed before the run in which the mutant
  survived again.
- **D12's edges, as they stand.**
  - A kind change in sight still reads "X went out of sight" and "X is at x,y"
    for its token's hiding and placing again.
  - A party member the world adds again under a used id, out of sight, with
    nothing to strip, can end an event with its run; at its first sight
    `correct` sends its bare conditions and grants first, and the chain
    leaves them out of the Story. `add_actor`'s description tells the agent
    never to reuse an id (VTT-390).
- **D7.** A log opened for an actor already a party member before its event
  introduces its own character unseen, and the first event brings it whole.
  It happens only for a campaign written before character logs, or after an
  opening's frames did not fold. No code, by the 2026-10-21 ruling.
- **D8.** A log written before this change keeps every companion's sheet.
  Nothing is migrated, by the owner's ruling of 2026-09-04.

## What was deliberately left out, and where it went

- The sheet view, a right-click "Show character sheet" shown only of a
  companion the viewer's character has seen: the sheet work of ADR-012's
  build order, step 1, item 2 ("Campaign data fits the ruleset it is played
  under") and item 4 ("A sheet offers only what it names").
- Whether the board's resource chips and condition dots stay: asked of the
  owner on 2026-11-06. It has no home until he answers, and until then they
  stay.
- The specifications: SPEC-016, SPEC-021 and SPEC-022 move at Task 5.
