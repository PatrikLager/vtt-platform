# The DM gives a character an ability and takes one away, in play, and the player is told — report

Ticket: `docs/superpowers/specs/2026-11-14-a-sheets-abilities-change-in-play-design.md`.
Plan: `docs/superpowers/plans/2026-11-14-a-sheets-abilities-change-in-play.md`.
Branch `feat/a-sheets-abilities-change-in-play` from main `12bda75`. Commits:
`8f022e1` (ticket, plan, register), `7dcd4b4` (Task 1, the events),
`0638899` (Task 2, the projection), `6b4436a` (Task 3, the commands, the
tools and the console), `0a71dbd` (Task 4, the goldens), `c30322f` (the
matrix comments of `internal/gateway/authz_test.go` restored to main's
text), `65edeed` (Task 5, the window), `2b935bc` (Task 6's comment ledger),
`24cdd4a` (the test that kills the mutation survivor the gate's run on
`2b935bc` found), `45921b4` (a client mutation adjudication re-pointed, which
the gate's run on `24cdd4a` refused) and `e86b852` (the test that reads
`/api/ruleset` after an `AbilityGiven` and an `AbilityTaken`, the last code
commit). Every code reference below
is to `e86b852`.

## Done looks like

1. [x] **The DM gives an ability.** `GiveAbility`, the arm `give_ability` of
   `ClientCommand.command` (`contract/vtt/v1/commands.proto`); `ToEvent`
   converts it to an `AbilityGiven` (`internal/gateway/convert.go`);
   `commandRoles` gives it to the DM and the agent
   (`internal/gateway/authz.go`); `engine.Apply`'s `AbilityGiven` arm appends
   the id at the end of the list.
   - Give, then use: `internal/gateway/ability_change_command_test.go`'s
     `TestAGiveThenAUseResolvesForTheDMTheAgentAndThePlayer`, a give by the
     DM and by the agent to an actor naming no ability until then, after
     which the DM's, the agent's and the player's use of it resolves. It does
     not build on `12bda75`, which has no `GiveAbility`.
   - Through the tool: `cmd/vtt/mcp_ability_change_e2e_test.go`'s
     `TestMCPGiveAbilityThenAUseResolves`, which sees the use refused first.
   - The dump: `cmd/vtt/ability_change_dump_test.go`'s
     `TestTheDumpAfterGivesAndTakesIsTheCommittedOne`, through `writeDump`,
     the writer of `vtt state dump` (`cmd/vtt/state_dump.go`), against
     `contract/testdata/ability_change_dump.json`.
   - The event appended: `internal/gateway/ability_change_internal_test.go`'s
     `TestAGiveAppendsOneAbilityGivenAndChangesOnlyTheActorsList`, and
     `internal/gateway/qa_ability_change_command_test.go`'s
     `TestQAGiveAbilityFromTheDMAndTheAgentAppendsOneAbilityGivenAndEndsTheList`
     (VTT-713).
2. [x] **The DM takes an ability.** `TakeAbility`, the arm `take_ability`, and
   `AbilityTaken`; `engine.Apply` removes every occurrence of the id, the
   others keeping their order.
   - Take, then use: `TestATakeThenAUseIsRefusedForTheDMTheAgentAndThePlayer`,
     after which the DM's, the agent's and the brawler's player's use is
     refused with `gateway: use_ability: actor "brawler" does not name ability
     "fists"`, the sheet's refusal, and the log's head does not move.
   - Through the tool: `TestMCPTakeAbilityThenAUseIsAToolError`.
   - The event appended and the list left:
     `TestATakeAppendsOneAbilityTakenAndLeavesNoOccurrenceTheOthersInOrder`,
     `TestQATakeAbilityFromTheDMAndTheAgentRemovesEveryOccurrenceAndKeepsTheOrder`
     and `TestQAAUseFollowsTheSheetAfterAGiveAndATake` (VTT-714).
3. [x] **Nothing else about the character changes.** `sameStateBut`
   (`internal/gateway/ability_change_internal_test.go`) compares the state
   before and after each command: its scenes, tokens, sessions, conditions and
   notes by `reflect.DeepEqual`, and every actor by `proto.Equal`, the changed
   actor's list alone set to what the command makes it. Both of item 1's and
   item 2's internal tests run it. Both folds:
   `internal/engine/ability_change_test.go`'s
   `TestAnAbilityGivenEndsTheListAndChangesNothingElse` and
   `TestAnAbilityTakenLeavesNoOccurrenceAndChangesNothingElse`, and
   `client/test/fold-abilities.test.ts`'s "an ability given ends the actor's
   list and changes nothing else" (VTT-715). Over the WebSocket,
   `TestQAGiveAndTakeChangeNothingButTheActorsAbilityList` (VTT-713,
   VTT-714).
4. [x] **What is refused, and appends nothing.** The WebSocket tests below each
   read the log's head after the refusals.
   - A player's and a spectator's:
     `TestAGiveOrATakeFromAPlayerOrASpectatorIsRefusedAndAppendsNothing`, the
     player's on their own party member, so the role table is the only
     reason; `internal/gateway/ability_change_authz_test.go`'s
     `TestOnlyTheDMAndTheAgentMayGiveOrTakeAnAbility` and the rows of
     `authzCases` (VTT-716).
   - An actor the campaign does not hold, a give the sheet already names and a
     take it does not name:
     `TestAGiveOrATakeTheFoldRefusesReachesTheIssuerAndAppendsNothing`, each in
     the fold's words (VTT-717, VTT-720, VTT-721).
   - With a ruleset served, an id it does not hold:
     `TestWithARulesetAGiveOfAnIdItDoesNotHoldIsRefusedNamingTheActorAndTheId`,
     refused as `gateway: give_ability: actor "<actor>": ability_ids: names
     ability "<id>", which the ruleset does not hold`, the shape
     `validateAddActorSheet` gives (VTT-718). The ruleset's refusal comes
     before the fold's unknown actor, as D3 prefers.
   - With no ruleset served:
     `TestWithNoRulesetAGiveOfAnyNonEmptyIdIsAcceptedAndAnEmptyOneRefused`;
     any non-empty id is accepted, and an empty one is refused by the fold
     (VTT-745, VTT-722).
5. [x] **The player's window follows.** `watchable`
   (`internal/gateway/viewpoint.go`) fills `WatchableActor.ability_ids` for
   each party member the participant controls, so a give or a take sends the
   set again; `watchKey` (`client/src/app.ts`) keys on those ids, so the set
   issues a read of `/api/ruleset`.
   - The panel: `client/test/sheet-news.test.ts`'s "a player's panel offers a
     given ability once the read answers, and stops offering a taken one",
     which folds an `abilityGiven` and an `abilityTaken` with the sets that
     carry them and reads the panel after each read answers; "a player's
     window reads /api/ruleset again when a set changes a party member's
     ability ids, and not for an equal set" (VTT-732);
     `client/test/qa-sheet-news.test.ts` (VTT-695).
   - The set: `internal/gateway/watchable_abilities_test.go`'s
     `TestAPlayersSetCarriesTheAbilityIdsOfEachPartyMemberTheyControl`,
     `TestAChangeToAControlledPartyMemberSendsEachOfTheirSeatsANewSetBeforeItsEntry`
     and `TestAGiveToAnotherPlayersPartyMemberSendsThisPlayerNoSet` (VTT-731).
   - `/api/ruleset`: `internal/gateway/ruleset_by_role_test.go`'s
     `TestAPlayersReadFollowsAGiveAndATake`, a player whose character's sheet
     names no ability reads none, then the ability given, then none after the
     take, on one server (VTT-691, which the sort kept; `abilitiesReadBy`
     reads the snapshot's sheets at each request).
6. [x] **The player is told.** `Session.onSheetChanged`
   (`client/src/session.ts`) hands over what each party member gained and
   lost; `app.ts` keeps it until the read the set issued answers or fails,
   then shows `sheetNews` (`client/src/view/naming.ts`) as the toast.
   - `client/test/sheet-news.test.ts`'s "a player is told what a give and a
     take changed once the read it issued answers, a taken ability named by an
     earlier answer": "Ayla can now use Heal.", then "Ayla can no longer use
     Slash.", the taken ability named by an answer the take's own read no
     longer holds (OD-5, VTT-734).
   - Beside it: "a player watching one character is told of a change to
     another they control", "a redial sent a set that changed meanwhile tells
     the change", "a read that fails still tells the change, naming an ability
     no answer has named by its id"; `client/test/session.test.ts`'s "a set
     that changes a party member's ability ids hands over what each gained and
     lost before it replaces the set"; `client/test/naming.test.ts`'s "a
     sheet's news names each character and each ability given and taken, by
     the list's display name, else its id" (VTT-733).
   - No other window: "a DM's and a spectator's window show no toast for a set
     that changes ability ids" (VTT-752).
7. [x] **A character log is told what its eyes see.** `classify` forwards
   both payloads in the `ResourceChanged` case, `passIf(now.party[id] &&
   saw(id))`; the list left `sameIncarnation` and joined `actorBelief`; and
   `statusFrames` brings a log to the world's set with takes, then gives
   (`internal/perceive/projection.go`).
   - The changed party member's own log and a log that saw it:
     `internal/perceive/ability_internal_test.go`'s
     `TestAPartyMembersOwnLogIsSentItsGiveAndTake` and
     `TestALogWhoseEyesSawAPartyMemberIsSentItsGiveAndTake`;
     `internal/perceive/qa_ability_forward_test.go` (VTT-741).
   - A log that does not see it is sent nothing:
     `TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoGiveOrTake` (VTT-741). It
     holds the current list at its next sight:
     `internal/perceive/ability_correction_test.go`'s
     `TestALogThatHeldAPartyMemberWholeOutOfSightIsSentNothingUntilSightBringsTheChanges`,
     `TestALogThatSawAPartyMemberEarlierIsSentNothingOfItsChangesUntilItSeesItAgain`
     and `TestASightCorrectionSendsTheAbilitiesAfterTheConditionsAndBeforeTheControllers`
     (VTT-724, VTT-743).
   - A creature's change reaches no log: `TestNoLogIsSentACreaturesGiveOrTake`,
     whose eyes include the goblin's own player's, and
     `TestACreaturesGiveReachesNoLogAndItArrivesWithoutItsList` (VTT-741,
     VTT-698).
   - No removal, addition or withdrawal:
     `TestALogThatSawAGiveHoldsTheAbilityAndIsNeverSentTheActorAgain`,
     `TestATakeAndAGiveBackWhileUnseenSendNothingAtSight` and
     `internal/perceive/qa_ability_correction_test.go`'s
     `TestQAAbilityCorrectionNoListChangeWithdrawsOrIntroducesAMemberAgain`
     (VTT-744).
   - A resumed log resumes:
     `TestAResumeHoldsTheWorldsAbilityIdsInAnyOrderAndStartsAfreshFromALaggingList`,
     and `internal/campaign/ability_reopen_test.go`'s
     `TestACampaignReopenedAfterAGiveResumesEveryLog` (VTT-742).
   - Over random histories: `internal/eventgen` draws gives and takes, so
     `TestEveryCharacterLogFoldsAtEveryPrefix` folds them. No walk of that
     test is shown to reach a sight correction; the tests above and
     `TestQAAbilityCorrectionAWalkKeepsEveryLogInSightOfEachListAsASet` hold
     it.
   - A kind change in sight still re-introduces an actor only its list tells
     apart (`kindMovesList`):
     `TestANamelessPartyMemberMadeACreatureInSightIsHeldAndSentWithoutItsList`,
     `TestANamelessCreatureMadeAPartyMemberInSightIsHeldWithItsList` and
     `TestANamelessCreatureMadeAPartyMemberInSightIsForwardedATakeThatFolds`;
     and one with no list is sent the grant alone,
     `TestANamelessActorWithNoListChangingKindInSightIsForwardedTheGrantAlone`
     (VTT-755).
8. [x] **The client's fold agrees with the server's.**
   `client/test/fold-parity.test.ts`'s "fold parity: toy-brawl" and
   `internal/harness`'s `TestFoldGoldenCorpus` over
   `scenarios/goldens/toy-brawl`, whose `state.json` was derived by hand
   before `stream.json` was recorded (VTT-727); `cmd/vtt`'s
   `TestScenarioGoldenStreamsHaveNotDrifted` holds the recorded stream. The
   dump fixtures:
   `client/test/fold-abilities.test.ts`'s "the dump after gives and takes
   matches the committed dump, as Go's writeDump does" and "each refusal in
   the committed fixture, the client's fold refuses", beside
   `TestTheDumpAfterGivesAndTakesIsTheCommittedOne` and
   `TestEachRefusalInTheCommittedFixtureTheServerRefuses` (VTT-727).
9. [x] **The agent can do it.** The `give_ability` and `take_ability` entries of
   `tools/toolgen/main.go`, held against `contract/testdata/expected_tools.json`
   by `TestToolsMatchGolden` (`tools/toolgen/main_test.go`), and regenerated
   into `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`.
   - Listed and sent: `internal/mcp/tools_test.go`'s
     `TestListToolsReturnsAllCommandToolsByName`, and
     `internal/mcp/ability_change_tools_test.go`'s
     `TestGiveAndTakeAbilityReachTheWireCarryingTheirArguments`.
   - What they say: `TestGiveAndTakeAbilitySayWhoMayWhatEachRefusesAndThatNothingElseChanges`
     (VTT-728) and `TestGiveAndTakeAbilitySayEachControllingPlayerIsTold`
     (VTT-735); `add_actor`'s `abilityIds`,
     `TestAddActorSaysAnActorUsesOnlyTheAbilitiesItNames` (VTT-729) and
     `TestAddActorSaysAnEmptyAbilityIdRefusesTheCommand` (VTT-749); the served
     texts are the committed ones, `cmd/vtt/qa_use_unnamed_test.go`'s
     `TestQAUseTheCommittedToolTextsAreTheServedOnes`.
10. [x] **The DM console can do it.** `renderDMConsole`'s Abilities group
    (`client/src/view/dm.ts`), between "Remove condition" and "Who controls
    what", fed by `app.ts` from its newest `/api/ruleset` answer through
    `DMDeps.abilities`.
    - `client/test/dm-abilities.test.ts`'s "each actor's row offers a take of
      each id its sheet names, once each, in the sheet's order", which clicks a
      chip and reads the `takeAbility` sent, and "each actor's row offers a give
      of each listed ability its sheet does not name, in the list's order",
      which chooses one, clicks Give and reads the `giveAbility` sent; "no
      ability the sheet names is offered to give, whatever the list's order".
    - `client/test/qa-dm-abilities.test.ts` (VTT-730, with the tests above).
    - Beside them: from the booted page, `client/test/app.test.ts`'s "a DM's
      console offers to give the abilities its ruleset read answered, and
      sends the give"; the `giveAbility` and `takeAbility` rows of
      `COMMAND_SURFACE` (`client/test/command-surface.test.ts`).
11. [x] **The gate.** `task check` is green on `e86b852`, exit 0. On
    `2b935bc`, the ledger commit, it went red at `check:mutation`: one
    survivor in `kindMovesList` no entry excused, which `24cdd4a` kills;
    `check:ts-mutation`, the step after it, did not run. On `24cdd4a`
    `check:mutation` passed with no survivor unjudged, and
    `check:ts-mutation`'s self-test refused an entry, which `45921b4` re-points
    (see "Deviations"). On `45921b4` it was green. `e86b852` adds a gateway
    test after it (see "Deviations"); on `e86b852` `check:mutation` measured
    `internal/gateway` afresh, with no survivor unjudged, and reused each
    other package's last passing verdict.
    `--write-ledger` ran once, in `2b935bc`: between `65edeed` and it only
    `tools/comment-ceilings.txt` changes, where it lowered each ceiling this
    ticket's code brought down, added a row for each new file and raised none.
    No ledger run followed the code commits after it, and `check:comments`
    passed on each later run. Before the runs on `2b935bc`, `24cdd4a` and
    `45921b4` the free space was under `check:mutation`'s floor, and
    `go clean -cache` brought it above; the run on `e86b852` started above it.
    The gate warns, at `check:mutation`, of the space it ran with. The
    client's mutation is not measured: `check:ts-mutation` names `26af7d3` and
    this ticket (see "What could not be established"). `task e2e` is green on
    `e86b852`; the
    test it skips, "a network drop while the table plays on ends in a
    Reconnect", is skipped on `12bda75` too.

## What the rules became

The ticket's rules, each with the id it earned:
- Only the DM and the agent may give or take an ability: VTT-716.
- A give adds one ability to one actor's sheet, and a take removes one;
  nothing else about the actor changes: VTT-713, VTT-714, VTT-715.
- With a ruleset served, a give of an ability the ruleset does not hold is
  refused: VTT-718.
- A give of an ability the sheet already names is refused, and so is a take of
  one it does not name: VTT-720, VTT-721.
- A give or a take naming an actor the campaign does not hold is refused:
  VTT-717.
- A player's panel offers a given ability, and stops offering a taken one,
  without a reload: VTT-731 and VTT-732, beside VTT-695 and VTT-691, which the
  sort kept as they were.
- The player who plays the party member is told of each give and each take:
  VTT-733 and VTT-734, and the tools say so, VTT-735.
- A character log is sent a give or a take only when its eyes see the actor,
  and a creature's never: VTT-741, VTT-724, VTT-743.
- A character log never reads a give or a take as the actor's removal or its
  addition: VTT-744, VTT-742.

The plan's labels:

| Plan | Id | |
|---|---|---|
| R1 | VTT-713 | a give appends one `AbilityGiven`, the id last |
| R2 | VTT-714 | a take appends one `AbilityTaken`, no occurrence left |
| R3 | VTT-715 | both folds change only the list |
| R4 | VTT-716 | a player's and a spectator's refused |
| R5 | VTT-717 | an actor the folds do not hold refused |
| R6 | VTT-718 | an id the served ruleset does not hold refused |
| R7 | VTT-719 | withdrawn in Task 3, succeeded by VTT-745 |
| R8a | VTT-720 | a give of an id the sheet names refused |
| R8b | VTT-721 | a take of an id the sheet does not name refused |
| R9 | VTT-722 | an empty given id refused by both folds |
| R10 | VTT-723 | withdrawn in Task 2, succeeded by VTT-741 |
| R11 | VTT-724 | a log is brought to the world's set at sight |
| R12 | VTT-725 | withdrawn in Task 2, succeeded by VTT-744 |
| R13 | VTT-726 | withdrawn in Task 2, succeeded by VTT-742 |
| R14 | VTT-727 | the client's fold agrees with the dump |
| R15 | VTT-728 | the tools say who, what is refused, what stays |
| R16 | VTT-729 | `add_actor`'s schema; succeeds VTT-703 |
| R17 | VTT-730 | the console offers and sends both |
| R18 | VTT-731 | the set carries a controlled party member's ids |
| R19 | VTT-732 | the window reads again on changed ids |
| R20 | VTT-733 | the toast |
| R21 | VTT-734 | names from every list read since load |
| R22 | VTT-735 | the tools say the controlling player is told |
| R23 | VTT-736 | the Story's words |

Allocated during the work:
- VTT-737 (Task 1): both folds hold an added actor's list until a give or a
  take changes it; succeeds VTT-706.
- VTT-738, VTT-739, VTT-740 (Task 2): succeed VTT-699, VTT-707 and VTT-708.
- VTT-741, VTT-742 (Task 2, from Phase 4a): succeed VTT-723 and VTT-726.
- VTT-743 (Task 2, from Phase 4a): the order of a correction's frames.
- VTT-744 (Task 2, from the re-review): succeeds VTT-725.
- VTT-745, VTT-746 (Task 3): succeed VTT-719 and VTT-686.
- VTT-747, VTT-748, VTT-749 (Task 3, the owner's ruling on an empty id):
  `add_actor` and `adventure.Load` refuse a sheet naming one, and `add_actor`'s
  schema says so.
- VTT-750, VTT-751 (Task 3, from the reading review): succeed VTT-685 and
  VTT-687.
- VTT-752 (Task 5, from Phase 4a): a window that is not a player's shows no
  message and issues no read for a changed set.
- VTT-753 (Task 5, from the reading review): succeeds VTT-697, scoped to the
  panel.
- VTT-754 (Task 6, from the gate's mutation survivor), withdrawn at its
  reading review, succeeded by VTT-755: a log that saw, before the event, an
  actor with no name, values or ability ids is sent no removal or addition of
  it when the event changes its kind.

Withdrawn, each by the task that made it false: VTT-706 (Task 1); VTT-699,
VTT-707, VTT-708, VTT-723, VTT-725 and VTT-726 (Task 2); VTT-685, VTT-686,
VTT-687, VTT-703 and VTT-719 (Task 3); VTT-697 (Task 5); VTT-754 (Task 6,
false as worded for the actor's own log, opened at the event that makes it a
party member). Rows kept that gained evidence: VTT-272, VTT-688, VTT-691,
VTT-695 and VTT-698.

Refused at the sort, as the plan's "Requirements" records: the use of a given
or taken ability (VTT-690 and VTT-137 hold it); `/api/ruleset` answering the
new set at the next read (VTT-691); the panel offering and ceasing to offer
(VTT-695); the frames' sequence (SPEC-016's rows); a controlled creature's
change telling nobody (near-theoretical). The sort split R8 into R8a and R8b.

The open decisions, as the owner signed them off on 2026-11-14: OD-1, A, an
`AbilityGiven` and an `AbilityTaken` each naming one actor and one ability;
OD-2, A, a log holds the ids as a set; OD-3, A, the watchable set carries a
controlled party member's ids; OD-4, A, the Story draws a give and a take in
every window whose log holds the event; OD-5, B, a window names an ability by
every ruleset list it has read since it loaded; OD-6, A, no length bound and
an empty id refused; OD-7, the alpha campaign brought up to date outside this
ticket. Those answer the ticket's own "What could not be established".

The owner's rulings during the work: on 2026-11-14, during Task 2, an empty
ability id is refused where a sheet enters, `add_actor` and adventure loading,
with or without a ruleset, and the folds stay as they are (principle 5). Task
3 carried it.

## The deliberate breaks

Each repaired, each named in its commit:
- Task 1: a take removing the first occurrence only, in either fold (the
  occurrence and dump tests went red); the duplicate refusal dropped, in
  either fold (the refusal and agreement tests); `now.party` dropped from the
  projection's arm (`TestNoLogIsSentACreaturesGiveOrTake`); `saw` dropped
  (`TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoGiveOrTake`).
- Task 2: the list put back in `sameIncarnation` (the out-of-sight, `Resume`
  and campaign reopen tests); the list dropped from `statusFrames` (the sight
  correction and lagging-`Resume` tests); the list compared in order (the
  take-and-give-back and reordered-`Resume` tests); `kindMovesList` removed
  from `recall` (the nameless kind-change tests).
- Task 3: a player cell for `give_ability`
  (`TestOnlyTheDMAndTheAgentMayGiveOrTakeAnAbility`); the validator's case
  dropped (the unheld-id test); the console offering a named ability (the
  console tests); either empty-id refusal removed (its tests); the give lock
  keyed by the actor or the ability alone (a lock test each); the console's
  abilities fed as `[]` (the app-boot test).
- Task 4: the client fold inserting a given id in sorted order (`fold parity:
  toy-brawl`); the conformance replay skipping takes
  (`TestQADataScenarioGoldensCarryAbilityIDs`).
- Task 5: ids for every party member (the no-control set test); ids for
  players only (the controlling-spectator test); the ids dropped from
  `watchKey` (the re-read tests); changes handed over beside a true flag (the
  hand-over test); names from the newest answer alone (the take's toast);
  `me?.role` read as `me.role` (the before-`/api/me` test).
- Task 6, the gate's survivor: `kindMovesList`'s `> 0` made `>= 0` (the new
  nameless kind-change test, in both directions); the forwarded grant's event
  id cleared (the same test, a built grant).
- The report's reading review: each player's controlled actors kept, cloned,
  between reads in `abilitiesReadBy` (`TestAPlayersReadFollowsAGiveAndATake`,
  "after the give bo reads []").

Each task's Phase 4a agent, which could not open the code, broke its own
tests' expectations or inputs instead and saw each go red; Task 4's derived
the goldens by hand from the scenario.

## Rule 9: how MapTool does this

As the ticket records and the plan's P7 confirms at RPTool `maptool`
`f4b7fef6c`: a token holds its own macros, one added or removed at a time
through `updateTokenProperty` with `saveMacro` or `deleteMacro`, which the
server relays to every other client unchecked; who may is checked on the
client alone; every client receives every token's macros; nothing records the
change and nobody is told. Borrowed: one ability given or taken on one
character at a time (OD-1), and a redraw on a signal the client is already
sent rather than a reload (OD-3). Not borrowed: the client's own judgment (the
role table and the fold judge, principle 7); the absence of a record (each
change is an event, principles 2 and 3); every client sent every token's
macros (a log is sent a change only while its eyes see the actor, principles
16 and 17); the copy on the token (the sheet names the ruleset's entry,
principle 27); the silence (the player is told, principle 12 and the owner's
ruling of 2026-11-14).

## The plan's gaps

- G1: `playerRules` gained nothing; `TestEveryPlayerCommandHasARule` holds it.
- G2: the files the plan listed were reached. The walkers
  `TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
  `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable` and
  `TestEveryClientCommandConverts` iterate the oneof and took the new arms
  with no edit; `commandFor`, `authzCases`, `COMMAND_SURFACE` and the name
  lists of `internal/mcp`'s tests gained theirs. The files beyond the plan's
  are under "Deviations".
- G3: R7 is VTT-719, withdrawn in Task 3 and succeeded by VTT-745, since both
  folds refuse an empty id.
- G4: each row was withdrawn in the task that made it false: VTT-706 in Task
  1; VTT-699, VTT-707 and VTT-708 in Task 2; VTT-703 in Task 3, whose schema
  sentence P9 pins is now R16's words (VTT-729).
- G5: SPEC-015 and SPEC-021 are the specification task's, which follows this
  report.
- G6: the alpha campaign, OD-7; see "What could not be established".

## Phase 4a adjudications

- Task 1, QA finding 1: behaviour defect, unreachable at that commit since no
  command wrote the events — a log that saw a party member earlier but not at
  its change was sent a withdrawal and an introduction of it, repeated, at the
  change, `sameIncarnation` still comparing the list — Task 2's, whose D5
  removes the cause — Task 2's
  `TestALogThatSawAPartyMemberEarlierIsSentNothingOfItsChangesUntilItSeesItAgain`
  holds it.
- Task 1, QA finding 2: spec ambiguity — an `AbilityTaken` of an empty id is
  refused as an id the sheet does not name, with no refusal of its own — no
  rule (D2 gives the take none) — nothing changed.
- Task 1, QA finding 3: spec ambiguity — R14 holds end to end only once a
  command writes the events — Task 4's — the toy-brawl goldens hold it.
- Task 2, QA finding 1: spec ambiguity — VTT-723 did not tell a forwarded give
  from a correction's frame, nor except a projection starting afresh at the
  event — reworded — VTT-723 withdrawn, succeeded by VTT-741.
- Task 2, QA finding 2: spec ambiguity — VTT-726 spoke of every actor in
  sight, a creature included, which a log holds without its list — reworded —
  VTT-726 withdrawn, succeeded by VTT-742.
- Task 2, QA finding 3: spec ambiguity — the order of a correction's frames had
  no row — a rule — VTT-743 allocated.
- Task 3, QA finding 1: **behaviour defect** — the give `<select>` carried a
  `data-field`, which SPEC-022 gives no select or button — removed — "qaDMAb:
  the Abilities group's controls follow SPEC-022's data-field rule" holds it.
- Task 3, QA finding 2: spec ambiguity — an empty-id give is worded
  differently with a ruleset served, by the validator (`names ability ""`,
  which the ruleset does not hold), and without one, by the fold (`requires an
  ability id`) — no rule; both reach only the DM and the agent, in the
  gateway's and the fold's form (D2, D3) — nothing changed;
  `TestWithARulesetAGiveOfAnIdItDoesNotHoldIsRefusedNamingTheActorAndTheId`
  and `TestWithNoRulesetAGiveOfAnyNonEmptyIdIsAcceptedAndAnEmptyOneRefused`
  pin each.
- Task 3, QA finding 3: spec ambiguity — D7's "by id" — read as the actors'
  id order, which the console draws — nothing changed; "qaDMAb: one row per
  actor of the world, by id, named by its name else its id" holds it.
- Task 3, QA finding 4: spec ambiguity — an accepted give clearing its choice
  has no row — D7's preference — "qaDMAb: an accepted give clears its choice
  and asks for a repaint; a refused one keeps it and asks for none" holds it.
- Task 3, QA finding 5: spec ambiguity — the console's notice, locks and
  draft have no rows of their own — refused at the sort, covered by VTT-730
  and D7's preferences — nothing changed.
- Task 3: QA's client test needed type fixes only, its assertions unchanged.
- Task 4, QA finding 1: spec ambiguity — the goldens' take of `sober-up`, from
  inside the list, does not pin that the other ids keep their order — held by
  `TestAnAbilityTakenLeavesNoOccurrenceAndChangesNothingElse` and "an ability
  taken leaves no occurrence of it, the other ids in their order" — nothing
  changed.
- Task 4, QA finding 2: spec ambiguity — no id repeats on toy-brawl's sheets,
  so the goldens do not pin "every occurrence" — held by the same unit tests —
  nothing changed.
- Task 5, QA finding 1: spec ambiguity — no row said what a window that is not
  a player's does at a changed set — a rule — VTT-752 allocated.
- Task 5, QA finding 2: spec ambiguity — which toast reads answering out of
  order show — cosmetic: the window has one toast, and the names do not change
  while the ruleset is loaded once — nothing changed; see N2 under "What could
  not be established".
- Task 5, QA finding 3: spec ambiguity — the VTT-605 toast replaced by a sheet
  toast from the same set — near-theoretical — nothing changed.
- Task 5, QA finding 4: spec ambiguity — "the name the set gives" — the new
  set's — nothing changed.
- Task 5, QA finding 5: spec ambiguity — a take and a give back of one id
  between one judgment and the next send the set again and tell nothing —
  near-theoretical — nothing changed; see "What could not be established".
- Task 5: QA's client file set up its own DOM and relied on the page URL
  another file left, so it passed or failed by run order; it imports
  `./support/dom` and sets its page URL.

## Phase 4b

From Task 1 on, one reviewer per task, each re-reviewed until nothing was
left at Must or Should.
- Task 0: the ticket's verification stood for its reading — the
  `verify-ticket` check, by an agent that did not write the ticket (passes,
  with gaps G1 to G6, which the plan carries) — with the owner's sign-off of
  the plan; no further reviewer read it.
- Task 1: no Must. A test of VTT-272 cited under VTT-723 (moved to VTT-272); a
  Go test that compared only the list (it compares the whole state); QA tests
  missing from the register's evidence (added); `classify`'s comment
  contradicting SPEC-016 (it reads "a log is written no creature's sheet").
  Nits: a fresh-slice warning on the take arm; `sheetActor`'s doc dropped;
  test names saying "every" renamed to "each refusal in the committed
  fixture"; a descriptive comment dropped.
- Task 2: Must — a nameless actor with no values and a list, changing kind in
  sight, was no longer caught once the list left `sameIncarnation`: the log
  held a creature with a list or a party member without one, and a later take
  did not fold; `kindMovesList` in `recall` and the nameless kind-change tests.
  The eventgen comment moved and made true; a citation added. At re-review,
  VTT-725 false for a kind change: VTT-744. Carried from Task 1's review,
  `qa_ability_forward_test.go` folds what it is sent.
- Task 3: Musts — QA comment lines over `check-new-prose`'s width; comments
  giving a false reason ("the folds refuse one", which no fold does for an
  `ActorAdded`). Shoulds — the give lock keyed by actor and ability untested ("a
  give still unanswered holds back that actor's give of that ability alone, not
  of another" and "a give still unanswered holds back no other actor's give of
  the same ability" added); `app.ts` feeding the console untested (the
  app-boot test added); `authzCases` lacking the commands (their rows added);
  VTT-685 and VTT-687 loose for an empty id (VTT-750, VTT-751, and
  `TestQAAbAnEmptyIdIsNotHeld` re-cited to VTT-688).
- Task 4: no Must or Should. Nits: a replay comparing the lists as sets, or
  removing only a taken id's first occurrence, would pass toy-brawl's goldens,
  the first also base's check. The reviewer found that a Task 3 nit's edit of
  `internal/gateway/authz_test.go`'s comments would make `check:comments`
  refuse the branch; `c30322f` (see "Deviations").
- Task 5: no Must. Shoulds — the controlling spectator's reading of VTT-731
  pinned by `TestASpectatorWhoControlsAPartyMemberIsSentItsAbilityIds`, and
  test names corrected; a `me?.role` read as `me.role` that no test told apart,
  now told by "sets that change a party member's ability ids before /api/me
  answers leave the window on the last set, and no toast"; VTT-697 false under
  OD-5 (VTT-753, scoped to the panel, and the `rulesetKey` comment scoped
  with it). Nits N1 and N2, left for this report.
- Task 6, the gate's survivor: Must — VTT-754's "whose eyes see" false for
  the actor's own log, which `fanOutLocked` opens at the grant that makes it a
  party member and which `Event` then introduces to it (VTT-754 withdrawn,
  VTT-755 "saw, before the event"). Nits: the test's name and an exact check of
  the frames sent, the forwarded grant told from a built one by its event id.
  Phase 4a did not run: the test pins what the code already did.
- `e86b852`, from this report's reading review, which found item 5's second
  half held by no observation: no Must or Should. Nit: the read before the
  give asserts rather than checks the fixture, since it alone catches a read
  that keeps an empty answer. Phase 4a did not run: the test pins what the
  code already did, `abilitiesReadBy` unchanged.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D4's tests fold what a log is sent | Task 1's read `classify`'s ruling through an internal test, `internal/perceive/ability_internal_test.go`; from Task 2, `qa_ability_forward_test.go` folds what it is sent | Until Task 2 a forwarded give could not fold: `sameIncarnation` still compared the list, so a changed list re-introduced the actor and the log's fold then refused the forwarded give |
| D4: `classify` gains one arm for both payloads | they share the `ResourceChanged` case, through `sheetActor` | `gocyclo` held `classify` at its ceiling |
| Task 1's files | `cmd/vtt/ability_change_dump_test.go` added | R14 says "as `vtt state dump` prints it", so Go's `writeDump` folds the fixture the client test folds, as `cmd/vtt/named_token_dump_test.go` does for named tokens |
| D11: `giveAbility` and `takeAbility` draws | they sit behind `changeAbility`, and `Step`'s default arm moved into `sessionOrActor` | `gocyclo` held `Step` at its ceiling. The walks diverge from base's where the draws took their band; every guard holds, and no seed in `characterLogSeeds` was replaced |
| D5: the list leaves `sameIncarnation` | `recall` also marks reshaped an actor whose kind changes while its held or shown form names abilities (`kindMovesList`) | Task 2's reading review: without it a nameless actor changing kind in sight was held in a form no later take could fold onto |
| No refusal of an empty ability id where a sheet enters | `add_actor` and `adventure.Load` refuse a sheet naming one, with or without a ruleset (VTT-747, VTT-748, VTT-749); VTT-685, VTT-686 and VTT-687 withdrawn, succeeded by VTT-750, VTT-746 and VTT-751 | Task 2's implementer found that an `ActorAdded` naming an empty id with no ruleset served would have a sight correction send an `AbilityGiven` of it, which both folds refuse. The owner ruled: refuse it at entry, the folds unchanged, since principle 5 binds every log already written |
| (within the row above) `adventure.Load` | its check gives the refusal its own words, `must not hold an empty id`, before the ruleset reads the list | `adventure.Load` always reads against a ruleset, so `SheetAbilityFault` refused an empty id already, as an id the ruleset does not hold |
| D3: `handleCommand` calls `validateGiveAbility` after `validateAddActorWith` | the form validators are one switch, `validateForm` | `gocyclo` held `handleCommand` at its ceiling; a command matches one arm, so no order between them changes what is refused |
| D9's texts | `give_ability` names an empty ability id among its refusals; `add_actor`'s `abilityIds` says an empty id refuses the command with or without a ruleset | With no ruleset served the fold refuses an empty give (VTT-722); the owner's ruling on an empty id (VTT-749) |
| Existing tests unchanged in Task 3 | `internal/gateway/qa_ruleset_bound_test.go`'s empty-id case appends its actor's `ActorAdded` directly (`qaRuleBoardWritten`), its assertion unchanged; every console fixture gained `abilities: []` | `add_actor` now refuses the empty id that case puts on a sheet; `DMDeps.abilities` is required |
| Task 4 edits `scenarios/goldens/toy-brawl` | `qaDataGoldenProblems` (`internal/adventure/conformance/qa_data_abilities_test.go`) replays a golden stream's gives and takes, order included, before comparing with `state.json`; `// VTT-727` lines added in `client/test/fold-parity.test.ts` and `internal/harness/fold_golden_test.go` | `TestQADataScenarioGoldensCarryAbilityIDs` required the stream's `actorAdded` list to equal `state.json`'s, true only while a list could not change; the requirements chain accepts a test as VTT-727's evidence only where it cites the row |
| Task 3's nit: the matrix's cell count dropped from `internal/gateway/authz_test.go`'s comments | `c30322f` restored main's text | The lines sit in comment blocks longer than `check:comments` lets an edit into, so the gate would have refused the branch; the count is false and older than this ticket (see "What could not be established") |
| D8: "A spectator's set and the set of a player who controls none carry no ids" | a spectator who controls a party member is sent its ids | VTT-731 reads "each party member the participant controls", and `watchable` fills the ids by `controls` whatever the role; a spectator's window shows nothing for them and reads nothing (VTT-752) |
| D8: the ids each set gave and took, in its order | an id a sheet repeats is told once | Accepted at Task 5's reading review: `sheetChanges` (`client/src/session.ts`) keeps each id once, and a sheet may repeat an id (the 2026-11-13 OD-4) |
| Task 5's contract change: `WatchableActor`'s field 3 | `WatchableChanged`'s comment says "whenever it changes", not "whenever either changes" | The set is sent again when a controlled party member's ids change, so "either", of the names and the flag, was false |
| Task 6: `task check` green on the ledger commit | red at `check:mutation` alone, one survivor unjudged: `kindMovesList`'s `CONDITIONALS_BOUNDARY`, `> 0` made `>= 0`; `24cdd4a` adds the test that kills it, and VTT-755 | Under the mutant any kind change in sight re-introduces the actor. No test held an actor with no name, values or ability ids changing kind in a log that saw it; on `12bda75` `sameIncarnation` left that actor un-reintroduced too, and no mutant reached it there |
| Task 3's `client/src/view/dm.ts` leaves `tools/ts-mutation-equivalents.txt` as it was | `45921b4` re-points the control panel's `EqualityOperator` entry to the second copy of two | The Abilities group repeats the control panel's sort line above it, so the entry recorded one copy where the file holds two, and `check:ts-mutation`'s self-test refused it on the gate's second run; the first run stopped at `check:mutation`, before it. The self-test runs only in `task check` |
| Item 5's read of `/api/ruleset` left to VTT-691's tests, as the sort ruled | `e86b852` adds `TestAPlayersReadFollowsAGiveAndATake` | This report's reading review: VTT-691's tests read the route after a grant and a revocation, none after a give or a take, so item 5's second half named no observation |
| Task 5's "True when": every VTT-697 test passes unchanged | VTT-697 withdrawn, succeeded by VTT-753, scoped to the panel; its test passes, re-cited | Under OD-5 an older read's answer arriving after a newer one adds the names it gives to those the Story and the toast use, and shows the toast of the changes its set handed over, so VTT-697's "drops an answer" was false |

## What could not be established

- **The Story at a list-only party member's first sight.** A party member
  whose sheet holds a list and no other value, held unseen, is now brought to
  its list at its first sight by gives, and so is a list-only character in its
  own log's opening: `TestQAAbilityCorrectionAListOnlyMemberFirstHeldUnseenIsBroughtByGives`
  and `TestQAAbilityCorrectionALogOpenedForAListOnlyMemberBringsItsOwnListByGives`.
  The Story draws each of those frames as "<Name> can now use <ability>", for
  a list that never changed. On `12bda75` they were a withdrawal and an
  introduction, which `reintroductions` (`client/src/view/feed.ts`) leaves out
  of the Story (VTT-662). A correction of a party member with no values, its
  conditions among them, was drawn already. Whether the Story leaves these
  lines out is the owner's.
- **A spectator who controls a party member is sent its ids.** Under VTT-731
  the set of a spectator who controls a party member carries the ids its sheet
  names (`TestASpectatorWhoControlsAPartyMemberIsSentItsAbilityIds`), while
  VTT-692 has `/api/ruleset` answer that spectator no ability
  (`TestQAReadASpectatorReadsNoAbilityEvenWhereItsIdControlsAnActor`). The
  spectator's window shows nothing for them and reads nothing (VTT-752).
  Whether a controlling spectator is sent the ids is the owner's.
- **The matrix comments' cell count.** `authzCases`'s doc comment and the
  comment above `TestEveryPlayerCommandHasARule`
  (`internal/gateway/authz_test.go`) give the matrix a cell count that
  `TestAuthorizeTableAllCommandsAllRoles`'s own assertion contradicts, on
  `12bda75` as on `65edeed`. Neither line can be edited alone, since
  `check:comments` refuses an edit into a block longer than SPEC-010 allows,
  so the fix is a rewrite of both blocks. No ticket or defect file carries it;
  where it goes is the owner's.
- **Task 5's nits.** N1: no test holds `readRuleset`'s rethrow when a
  start-up read of `/api/ruleset` answers 401; `/api/me`'s 401 comes first at
  start-up, and the reviewer found no mutant Stryker would generate there. N2:
  what reads answering out of order show, in the toast and in the names, is
  held by no test; cosmetic. And the tools' sentence "Each player who controls
  the actor while it is a party member is told" holds for a window open at the
  change or redialled in the same page; a page opened later is told nothing,
  since its first set hands over nothing (VTT-733).
- **A take and a give back between one judgment and the next.** D8 says they
  leave the set equal. The set carries the ids in the order the sheet then
  holds them, so it differs from the one last queued and is sent again, and
  the window, keyed on the ids in their order, reads again; nothing is told,
  since `sheetChanges` compares the lists as sets. Near-theoretical (the
  2026-10-21 ruling); no code.
- **The MCP tests' file.** Task 3's plan names
  `cmd/vtt/mcp_ruleset_e2e_test.go`; the give and take tests are in a new
  `cmd/vtt/mcp_ability_change_e2e_test.go`, with their own fixture,
  `mcpAbilityFixture`. Why is not recorded.
- **The client's mutation report.** The stored report measured `26af7d3`;
  `12bda75` changes only `tools/ts-mutation-equivalents.txt` after it, judging
  that report's survivors. It is out of date from Task 1, so `task check`
  judges nothing of the client's mutation and names that commit and the
  tickets added since (SPEC-023). A full Stryker run is the owner's to ask
  for. That run will report the Abilities group's sort by actor id in
  `renderDMConsole`, `x.actorId < y.actorId` made `<=`, as a survivor no entry
  excuses: the same code as the control panel's sort, whose entry `45921b4`
  re-pointed, over a record keyed by actor id.
- **The alpha campaign (G6, OD-7).** `~/dev/vtt-alpha/adventures/two-rooms`,
  outside the repository, declares `dnd45e-minimal`, and no file under its
  `actors/` names `ability_ids` (read again on 2026-11-14). With this ticket
  the agent can give them their abilities in the running campaign
  (principle 26); the actor files need the list before the adventure seeds
  another. When is the owner's.

## What was deliberately left out

The ticket has no "What this is NOT for". What was left out was left out by
the plan's sort and the owner's rulings:
- Held elsewhere, as the sort ruled: the use of a given or taken ability
  (VTT-690, VTT-137), the next read of `/api/ruleset` (VTT-691), the panel's
  offer (VTT-695), the frames' sequence (SPEC-016).
- Near-theoretical, with no code, test or debt entry (the 2026-10-21 ruling):
  a give and a take of one id between one judgment and the next tell nobody; a
  controlled creature's change tells nobody and issues no read, since the
  panel never acts as a creature (SPEC-022); one set that drops a party member
  and changes another's ids lets the sheet's toast replace "You no longer play
  …".
- Accepted under the open decisions: a log may hold the ids in another order
  than the world (OD-2); no fold bounds an ability id's length (OD-6); a
  reload forgets the names earlier answers gave (OD-5); the fold's refusal
  words reach the DM and the agent as they are (D2, D3).
- Left untested on purpose (Task 1's review): the client fold's fresh array
  for a changed list; `fold()` folds a whole log, and `client/src/fold.ts`
  exports only `fold`, `doorKey` and `foldToDumpJSON`, so no caller holds the
  array it replaces.
- The specifications SPEC-007, SPEC-012, SPEC-013, SPEC-015, SPEC-016,
  SPEC-018, SPEC-021, SPEC-022 and SPEC-025 move in the specification task,
  written from this report (the plan's Task 7).
- The alpha campaign: OD-7, outside the repository.
