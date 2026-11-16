# A limited ability comes with its resource, and leaves with it — report

Ticket: `docs/superpowers/specs/2026-11-15-an-ability-is-given-with-its-value-design.md`.
Plan: `docs/superpowers/plans/2026-11-15-an-ability-is-given-with-its-value.md`.
Branch `feat/an-ability-is-given-with-its-value` from main `001bdad`. Commits:
`c3adb17` (Task 0: ticket, plan, register), `9dedd04` (Task 1, the resource
events), `86cb32c` (Task 2, the projection), `fc1daca` (Task 3, the give, the
take and the agent), `da86e01` (Task 4, where a sheet enters), `2ab2e11`
(Task 5, the console), `a845abf` (Task 6, the goldens and the scenario
runner), `70b61f3` (Task 7's comment ledger) and `42bd45a` (Task 7, the test
that kills the gate's mutation survivor; the last code commit). Every code
reference below is to `42bd45a`.

## Done looks like

1. [x] **A give carries the resource's value.** `GiveAbility` gained
   `map<string, Resource> resources = 3` (`contract/vtt/v1/commands.proto`);
   `handleGiveAbility` (`internal/gateway/ability_change.go`) appends the
   `AbilityGiven` and then a `ResourceAdded` per carried resource as one
   batch through `appendChange`. The ticket's example is `dnd45e-minimal`'s
   `hunters-flurry`; the tests show the same rule on `tavern-brawl`'s
   `chair-swing`, which spends `stamina`.
   - Give, then use, at a current above 0 and at 0:
     `internal/gateway/ability_value_test.go`'s
     `TestAGiveOfALimitedAbilityCarryingItsResourceAppendsTheGiveThenTheResource`;
     at 0 the use is refused as `insufficient`, not as missing. It does not
     build on `001bdad`, whose `GiveAbility` has no `resources`.
   - Through the tool: `cmd/vtt/mcp_ability_change_e2e_test.go`'s
     `TestMCPGiveAbilityWithItsResourcesValueThenAUseResolves`.
   - Independently: `internal/gateway/qa_ability_value_test.go`'s
     `TestQAAVGiveCarriesTheResourceAtItsValue` and
     `TestQAAVGivenAbilityIsUsableAtItsValue`.
   - The dump: `cmd/vtt/resource_change_dump_test.go`'s
     `TestTheDumpAfterResourcesAddedAndRemovedIsTheCommittedOne`, through
     `writeDump`, against `contract/testdata/resource_change_dump.json`; and
     `scenarios/goldens/toy-brawl/state.json`, in `vtt state dump` shape,
     derived by hand, which `internal/harness`'s `TestFoldGoldenCorpus` folds
     a real run's give of `chair-swing` with `stamina` into.
2. [x] **What a give refuses, appending nothing.** Each test reads the log's
   head before and after (`refusedAppendingNothing`).
   - No value: `TestAGiveOfALimitedAbilityWithoutItsResourceIsRefused`.
   - A resource the sheet holds is refused, and a give carrying none of an
     ability whose resource the sheet holds is accepted alone:
     `TestAGiveCarryingAResourceTheSheetHoldsIsRefusedAndOneCarryingNoneIsTheGiveAlone`,
     on `poolRuleset`, which adds `haymaker` spending `stamina` beside
     `chair-swing`.
   - A key the ability does not spend, an at-will ability's included:
     `TestAGiveCarryingAKeyTheAbilityDoesNotSpendIsRefused`.
   - All of these are in `internal/gateway/ability_value_test.go`;
     independently, `TestQAAVGiveRefusalsAppendNothing`.
3. [x] **A take removes the resource no ability left spends.**
   `handleTakeAbility` appends a `ResourceRemoved` of what `orphanedBy`
   answers, after the `AbilityTaken`.
   - Removed with the last ability spending it:
     `TestATakeRemovesTheResourceTheTakenAbilityAloneSpends`, and through the
     tool `TestMCPTakeAbilityTakesTheResourceItSpendsWithIt`.
   - Kept while another ability spends it:
     `TestATakeKeepsAResourceAnotherAbilityOnTheSheetSpends` (`poolRuleset`),
     and `TestQAAVTakeKeepsASharedResource`.
   - An at-will take removes nothing, and `drink`, which no ability of
     `tavern-brawl` spends, is never removed:
     `TestATakeRemovesNoResourceNoAbilitySpends`.
4. [x] **Nothing else changes.** The folds change nothing at a
   `ResourceAdded` or a `ResourceRemoved` but the named resource, compared
   field by field through `sameExcept` (`internal/engine/ability_change_test.go`):
   `internal/engine/resource_change_test.go`'s
   `TestAResourceAddedGivesTheActorTheResourceAtItsValueAndChangesNothingElse`
   and `TestAResourceRemovedTakesTheResourceFromTheActorAndChangesNothingElse`;
   independently `internal/engine/qa_resource_change_test.go`'s
   `TestQAResourceEventsChangeNothingElse`, and in the client
   `client/test/fold-resources.test.ts`'s "a resource added gives the actor
   the resource at its value and changes nothing else" and "a resource
   removed takes the resource from the actor and changes nothing else". The
   ability events still change only the list (VTT-715, unchanged). The
   command appends only those events: `acceptedAppending`
   (`internal/gateway/ability_value_test.go`) holds the head at the batch's
   length. Through the WebSocket, `TestQAAVGiveCarriesTheResourceAtItsValue`
   and `TestQAAVTakeRemovesTheResourceNoOtherAbilitySpends`
   (`internal/gateway/qa_ability_value_test.go`) compare the actor whole and
   every other actor before and after, through `qaAVWantActor`.
5. [x] **A sheet entering a campaign fits the rule.**
   `Ruleset.SheetSpendFault` (`internal/rules/sheet.go`) runs after
   `SheetAbilityFault` in `sheetFault` (`internal/gateway/add_actor_validate.go`),
   `loadActors` (`internal/adventure/load.go`) and `sheetsFit`
   (`cmd/vtt/serve_compose.go`).
   - The reading: `internal/rules/spend_fault_test.go`'s
     `TestSheetSpendFaultNamesTheFirstAbilityWithoutItsResourceThenTheFirstOrphan`.
   - `add_actor`: `internal/gateway/add_actor_spend_test.go`'s
     `TestAddActorRefusesASheetThatBreaksTheSpendRule`.
   - `adventure.Load`: `internal/adventure/actor_spend_test.go`'s
     `TestAnActorFileNamingALimitedAbilityWithoutItsResourceIsRefused` and
     `TestAnActorFileHoldingAResourceNoAbilityItNamesSpendsIsRefused`.
   - `vtt serve`: `cmd/vtt/campaign_spend_fit_test.go`'s
     `TestACampaignWhoseActorsBreakTheSpendRuleIsNotServedNamingEachActorAndItsFirstFault`.
   - The repository still loads, seeds and serves:
     `internal/adventure/conformance/conformance_test.go`'s
     `TestConformanceOverAdventuresGlob`,
     `internal/adventure/qa_actor_spend_test.go`'s
     `TestQASFCellarRatsAndGoblinAmbushStillLoad`, and
     `cmd/vtt/library_test.go`'s `TestScenarioLibraryRunsSelfContained`.
6. [x] **With no ruleset served, nothing here is checked.** A take removes no
   resource: `TestWithNoRulesetATakeRemovesNoResource`. `add_actor` and
   `vtt serve` refuse nothing this ticket adds:
   `TestWithNoRulesetAddActorAcceptsASheetThatBreaksTheSpendRule` and
   `TestWithNoRulesetACampaignWhoseActorsBreakTheSpendRuleIsServed`. A give
   carrying a value is refused (OD-2, A):
   `TestWithNoRulesetAGiveCarryingResourcesIsRefused`; one carrying none is
   accepted as before (`TestQAAVWithNoRuleset`).
7. [x] **A character log follows the sheet.** `classify` forwards both events
   through `sheetActor`; `sameIncarnation` no longer compares resources;
   `resourceFrames`, called from `statusFrames`, corrects a log's resources
   at sight (`internal/perceive/projection.go`).
   - In sight, forwarded and folded:
     `internal/perceive/resource_correction_test.go`'s
     `TestAResourceAddedOrRemovedInSightIsForwardedAloneAndHeld`;
     `internal/perceive/qa_resource_forward_test.go`'s
     `TestQAForwardedResourceEventsFoldIntoTheLog`.
   - Out of sight, corrected at the next sight, before the condition frames:
     `TestResourcesChangedOutOfSightAreCorrectedAtSightInByteOrderBeforeConditions`.
   - No withdrawal or introduction: the same tests, and
     `internal/perceive/qa_resource_correction_test.go`'s
     `TestQARCHeldUnseenDifferingOnlyInResourcesIsCorrected`.
   - Resumed rather than started over:
     `TestAResumeHoldsTheWorldsResourcesAndStartsAfreshFromALaggingSet` and
     `internal/campaign/resource_reopen_test.go`'s
     `TestACampaignReopenedAfterResourcesAddedAndRemovedResumesEveryLog`.
   - A creature's reaches no player's log:
     `internal/perceive/resource_event_internal_test.go`'s
     `TestNoLogIsSentACreaturesResourceAddedOrRemoved`.
   - Every walk `internal/eventgen` draws, the new events included, folds at
     every prefix: `TestEveryCharacterLogFoldsAtEveryPrefix`.
8. [x] **The client's fold agrees with the server's.**
   `client/test/fold-resources.test.ts`'s "the dump after resources added and
   removed matches the committed dump, as Go's writeDump does" and "each
   resource refusal in the committed fixture, the client's fold refuses",
   beside `cmd/vtt/resource_change_dump_test.go`'s
   `TestTheDumpAfterResourcesAddedAndRemovedIsTheCommittedOne` and
   `TestEachResourceRefusalInTheCommittedFixtureTheServerRefuses` over the
   same fixtures; `client/test/fold-parity.test.ts`'s "fold parity: toy-brawl",
   over a golden stream that now takes and gives `chair-swing` with
   `stamina`.
9. [x] **The agent can do it.** `tools/toolgen/main.go` words `give_ability`,
   `take_ability` and `add_actor`'s entry refusal:
   `tools/toolgen/main_test.go`'s
   `TestGiveAndTakeAbilitySayWhoMayWhatEachRefusesAndWhatChanges` and
   `TestAddActorSaysASpentResourceIsHeldExactlyWhileAnAbilitySpendingItIsNamed`;
   `TestToolsMatchGolden` holds `contract/testdata/expected_tools.json`. A
   current left out is refused naming the give's actor:
   `internal/mcp/resource_current_internal_test.go`'s
   `TestGiveAbilityArgumentsRefuseACurrentWithNoValueNamingTheActor`, and end
   to end `TestMCPGiveAbilityWithItsResourcesValueThenAUseResolves`.
10. [x] **The DM console can do it.** `client/src/view/dm.ts`'s Abilities
    group; `client/test/dm-abilities.test.ts`'s "choosing an ability that
    spends a resource the sheet lacks shows a current and a max field, named
    as the window names it", "no value field is shown for an ability whose
    resource the sheet holds, nor for one that spends none, and none is
    sent", "a give with the value sends the resource under resources, 0
    included" and "a give whose current is empty or not an integer says so
    and sends nothing"; independently `client/test/qa-give-value.test.ts`.
    Beyond the item, the console also refuses a max that is neither empty nor
    an integer (VTT-805) and an integer an int32 cannot hold (VTT-808).
11. [x] **The gate.** `task check` is green on `42bd45a`, and `task e2e` is
    green on it. The first run, on `70b61f3`, went red on a Go mutation
    survivor in `resourceFrames`; `42bd45a` kills it (Deviations). Every Go
    package's survivors are adjudicated. The mutants the gate counted as
    killed without evaluating them sit in `objectInSight`
    (`internal/perceive`), `Blockers` and `VisibleFrom` (`internal/sight`)
    and `Server.Run` (`internal/mcp`), none of it code this branch changed.
    `check:ts-mutation` judged nothing:
    the client's stored report is out of date (SPEC-023), and it named this
    ticket as added since.

## What the rules became

The ticket's rules, each with the id it earned:
- A give of a limited ability whose resource the sheet does not hold is
  refused unless it carries the value, and then the sheet holds it at that
  value: VTT-760, VTT-761, beside VTT-764 for a give that needs none.
- A give carrying a value for a resource the sheet holds, or one the ability
  does not spend, is refused: VTT-762, VTT-763. The refusal's words and order
  are VTT-800 and VTT-801.
- A take removes each resource the taken ability spends that no ability left
  spends, and no other: VTT-767. The batch's order and stamps are VTT-799.
- A sheet entering a campaign is refused when it breaks the rule: VTT-775
  (`add_actor`), VTT-776 (`adventure.Load`), VTT-777 (`vtt serve`). The
  repository's adventures still load: VTT-802. A use the earlier refusals let
  through, by an actor whose sheet names a limited ability without its
  resource, is still refused by the rules engine: VTT-804.
- With no ruleset served: VTT-765 (a value refused), VTT-766 (a give without
  one accepted), VTT-768 (a take removes nothing), VTT-778 (no entry
  refusal).
- A character log never reads a resource change as the actor's removal or
  addition: VTT-779, VTT-780, VTT-781, VTT-782, VTT-783, VTT-784, VTT-793,
  VTT-796.
- The DM console asks for the value exactly when it is needed: VTT-788,
  VTT-789, VTT-805, VTT-806, VTT-807, VTT-808.

Rules the work added beneath the ticket's: the folds' arms (VTT-769 to
VTT-773, VTT-791, VTT-792), the client's agreement with the dump (VTT-774),
the tools' words (VTT-785, VTT-786, VTT-787) and the Story's (VTT-790).

The plan's labels:

| Plan | Id | |
|---|---|---|
| R1 | VTT-760 | a give with the value appends the give, then the resource |
| R2 | VTT-761 | a give without a needed value refused |
| R3 | VTT-762 | a value for a held resource refused |
| R4 | VTT-763 | a key the ability does not spend refused |
| R5 | VTT-764 | a give needing no value appends the give alone; succeeds VTT-713 |
| R6a | VTT-765 | with no ruleset, a value refused; succeeds VTT-745 |
| R6b | VTT-766 | with no ruleset, a give without one accepted; succeeds VTT-745 |
| R7 | VTT-767 | a take removes the orphaned resource; succeeds VTT-714 |
| R8 | VTT-768 | with no ruleset, a take removes nothing |
| R9 | VTT-769 | both folds add a resource |
| R9b | VTT-770 | both folds refuse a bad `ResourceAdded` |
| R10 | VTT-771 | both folds remove a resource |
| R10b | VTT-772 | both folds refuse a bad `ResourceRemoved` |
| R11 | VTT-773 | both folds change nothing else |
| R12 | VTT-774 | the client's fold agrees with the dump |
| R13 | VTT-775 | `add_actor`'s entry refusal |
| R14 | VTT-776 | `adventure.Load`'s |
| R15 | VTT-777 | `vtt serve`'s |
| R16 | VTT-778 | none of them with no ruleset |
| R17 | VTT-779 | forwarding to a log that saw the party member |
| R18 | VTT-780 | the sight correction |
| R19 | VTT-781 | no withdrawal for resources |
| R20 | VTT-782 | `Resume` |
| R21 | VTT-783 | a re-add; succeeds VTT-739 |
| R22 | VTT-784 | the bare `ActorRemoved`; succeeds VTT-740 |
| R23 | VTT-785 | the tools' words; succeeds VTT-728 |
| R24 | VTT-786 | `add_actor`'s schema |
| R25 | VTT-787 | the tool refuses a valueless current |
| R26a | VTT-788 | the console's fields |
| R26b | VTT-789 | the console's empty current |
| R27 | VTT-790 | the Story's words |

Allocated during the work, from a QA report or a reading review: VTT-791
(the folds read no number of the value), VTT-792 (a new map), VTT-796
(succeeds VTT-794 and VTT-795), VTT-799, VTT-800 (succeeds VTT-797), VTT-801
(succeeds VTT-798), VTT-802, VTT-804 (succeeds VTT-803) and VTT-806 to
VTT-808. By the implementer: VTT-793 (succeeds VTT-657), when Task 2 found
VTT-657 false, and VTT-805, the console's max refusal, while Task 5 was
staged.

Withdrawn, each in the task that made it false: VTT-657, VTT-739 and VTT-740
in Task 2; VTT-713, VTT-714, VTT-728 and VTT-745 in Task 3; VTT-794 and
VTT-795 in Task 2 and VTT-797 and VTT-798 in Task 3, each allocated and
withdrawn within its task; VTT-803 in Task 4.

## The deliberate breaks

Each was repaired. Each through Task 7 is named in its commit; the last
entry was run while this report was written:
- **Task 1:**
  - the held-resource refusal dropped in `engine.Apply`
    (`TestAResourceAddedNamingNoActorOrAHeldResourceIsRefused`,
    `TestEachResourceRefusalInTheCommittedFixtureTheServerRefuses`) and in
    the client's fold (its refusal and fixture tests);
  - the map written in place (the map read before the event);
  - the value refused after the actor
    (`TestAResourceAddedWithNoValueIsRefusedBeforeItsActor`);
  - `now.party` dropped from the projection's arm
    (`TestNoLogIsSentACreaturesResourceAddedOrRemoved`);
  - `saw` dropped
    (`TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoResourceAddedOrRemoved`);
  - `sheetActor` not reading a `ResourceRemoved` (both forwarding tests);
  - the arm also writing the actor's name (the `sameExcept` walk).
- **Task 2:**
  - resources put back into `sameIncarnation` (the in-sight, out-of-sight,
    re-add, reopen and property tests, and the campaign dropped a frame set);
  - the max not compared (the max-change and `Resume` tests);
  - resources dropped from `kindMovesList` (the nameless kind-change test);
  - a creature corrected from its whole sheet (the creature resource test);
  - the unlandable clause dropped, and the clause renewing an unchanged
    current
    (`TestACurrentNoResourceChangedCanLandOnIsCorrectedByARemovalAndAnAddition`).
- **Task 3:**
  - the missing-value refusal dropped
    (`TestAGiveOfALimitedAbilityWithoutItsResourceIsRefused`);
  - a shared resource removed by the take
    (`TestATakeKeepsAResourceAnotherAbilityOnTheSheetSpends`);
  - resources accepted with no ruleset
    (`TestWithNoRulesetAGiveCarryingResourcesIsRefused`);
  - a key the ability does not spend accepted
    (`TestAGiveCarryingAKeyTheAbilityDoesNotSpendIsRefused`);
  - a resource the sheet holds accepted. The fold still refused it, so the
    test stayed green until it asserted the gateway's refusal shape, and then
    went red.
- **Task 4:**
  - the orphan half skipped (the rules, adventure and `add_actor` orphan
    tests);
  - the wrong field read for an ability's resource
    (`TestSheetSpendFaultNamesTheFirstAbilityWithoutItsResourceThenTheFirstOrphan`);
  - the rule dropped from `add_actor`, `adventure.Load` and `vtt serve` (each
    entry point's refusal tests).
- **Task 5** (each break turned its `dm-abilities` test red):
  - the fields shown for a resource the sheet holds;
  - an empty current sent;
  - the current sent as the max;
  - the fields never following a choice;
  - the value fields left uncleared;
  - the int32 range dropped.
- **Task 6:**
  - the client's fold writing the current as the max at a `ResourceAdded`,
    and writing the removed resource's old max ("fold parity: toy-brawl");
  - the runner's give and take arms dropped
    (`TestScenarioGoldenStreamsHaveNotDrifted`, at the denied give after the
    take).
- **Task 7** (each break turned
  `TestACurrentAtItsMaxOrAtZeroIsCorrectedByOneResourceChanged` red):
  - `resourceFrames` renewing a current exactly at its max;
  - `resourceFrames` renewing a current of zero.
- **Run on `42bd45a` while this report was written**, since Task 3 and
  Task 4 had not run them (Deviations):
  - the walker naming `actor ""`
    (`TestGiveAbilityArgumentsRefuseACurrentWithNoValueNamingTheActor`);
  - the spend rule run with no ruleset in `sheetFault` and in
    `composeServer`
    (`TestWithNoRulesetAddActorAcceptsASheetThatBreaksTheSpendRule`,
    `TestWithNoRulesetACampaignWhoseActorsBreakTheSpendRuleIsServed`). Each
    went red on a nil pointer: the rule cannot run without a ruleset, so the
    break shows that each test takes the no-ruleset path, not that it tells
    a subtler fault.

## Rule 9: how MapTool does this

The plan records it. MapTool sets or resets a token's property one at a time
(`setProperty`, `resetProperty`). A property may hold nothing, with a default
filling in. No macro is tied to a property, and every client is sent every
update.
- **Borrowed:** the granularity. A resource arrives and leaves by its own
  event (OD-1).
- **Not borrowed:**
  - the default: no automatic value (the owner's 2026-11-08 ruling);
  - the absence of a tie between an ability and what it spends: a sheet holds
    only what can be used (the owner's 2026-11-15 ruling);
  - every client sent every update: a log is sent a resource change only
    while its eyes see the party member.

## The plan's gaps

- G1: `client/src/state.ts` did not change; its `Resource` type already holds
  a current and a max.
- G2: the work reached the files the plan listed, with two differences:
  - the two handlers, `handleGiveAbility` and `handleTakeAbility`, went to
    a new file, `internal/gateway/ability_change.go`, where the plan placed
    them in `ability_validate.go`;
  - the oneof walkers `TestEveryEnvelopePayloadArmHasAnExplicitRuling` and
    `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable` needed no edit,
    since they range over the oneof.

  The plan's other files: `internal/eventgen/model.go` (from Task 2; see
  "Deviations"), `client/src/view/spectator.ts`'s `describe`,
  `contract/roundtrip_test.go`'s fixtures, `add_actor`'s docs,
  `rulesets/dnd45e-minimal/guide.md` and the fixtures the entry rule
  refuses. `internal/harness/engine.go` is beyond them (see "Deviations").
- G3: VTT-714, VTT-739, VTT-740 and VTT-745 were withdrawn, each with its
  successor; VTT-715 stands.
- G4: OD-6, no code; see "What was deliberately left out".
- G5: D10: a value is checked as `add_actor`'s is, and the folds read no
  number of it (VTT-791).
- G6: the alpha campaign was brought into fit before this branch's build
  served it (D13), outside the repository.

## Phase 4a adjudications

- **Task 1, QA finding 1: behaviour defect.** It was unreachable at that
  commit, since no command wrote the events. `sameIncarnation` read a party
  member's `ResourceAdded` or `ResourceRemoved` in sight as another actor.
  The actor was withdrawn and introduced with the new set, and the log's
  fold then refused the forwarded event as a duplicate. Task 2's D5 removes
  the cause; `TestQAForwardedResourceEventsFoldIntoTheLog` was parked and
  committed with Task 2.
- **Task 1, QA finding 2: test defect.** QA's dump test imported
  `internal/engine` from `cmd/vtt`, which `go-arch-lint` refuses. It was
  rewritten onto `harness.Fold`; its unchanged-state-after-a-refusal half is
  held at the fold by `TestQAResourceAddedRefusals`.
- **Task 1, QA's rules needing an id:** the folds read no number of the
  value (VTT-791); a new map (VTT-792).
- **Task 2:** no defect. QA's rule needing an id, a correction's order and
  frame fields, became VTT-794; it was withdrawn at review and succeeded by
  VTT-796. QA's findings on SPEC-016 go to the specification task.
- **Task 3:** no defect. The injection of the `add_actor` entry rule stayed
  green, since that rule is Task 4's. QA's rules needing an id:
  - the refusal's shape and order, VTT-797 and VTT-798, withdrawn at review
    and succeeded by VTT-800 and VTT-801;
  - the batch's order and stamps, VTT-799.

  QA's `add_actor` test with no ruleset became VTT-778's evidence in Task 4.
- **Task 4:** no defect. One spec ambiguity: D4's "its reason names the key
  and the other half" cannot hold for a `resources` fault, since several
  abilities may spend one resource and the reason names none. The code is
  unchanged; SPEC-025 states the reason's words. QA's rules needing an id:
  VTT-802, and VTT-803, withdrawn at review and succeeded by VTT-804.
- **Task 5, gap G3: behaviour defect.** An integer an int32 cannot hold
  reached the wire, where the server closes the connection on a frame it
  cannot decode. Fixed with the console's `int32` reading; held by VTT-808
  and `dm-abilities`' "a current or a max a 32-bit field cannot hold says so
  and sends nothing".
- **Task 5, gap G1: spec ambiguity.** "+3" is accepted as an integer.
- **Task 5, gap G2: spec ambiguity.** A max of blanks is read as none; kept.
- **Task 5, gap G4: spec ambiguity.** The value drafts are kept per actor,
  not per resource, so what was typed for one choice stands under the next
  choice's resource name. Kept; see "What could not be established".
- **Task 5, gap G5:** the refusals' wording, kept as written.
- **Task 5, gap G6:** the refusals' precedence, kept as written.
- **Task 5, gap G7:** the placeholders, kept as written.
- **Task 5, QA's rules needing an id:** the naming (VTT-806); no field shown
  sends no resources (VTT-807); the int32 bound (VTT-808).
- **Task 6:** Phase 4a not run. The goldens are their own independent
  derivation: `state.json` is derived by hand; the stream's new events were
  written by hand, and `TestScenarioGoldenStreamsHaveNotDrifted` holds them
  equal to a real run, dice masked; `TestFoldGoldenCorpus` holds the state
  to the stream. The runner's new arms are held by
  `TestScenarioGoldenStreamsHaveNotDrifted` and
  `TestScenarioLibraryRunsSelfContained`. The task's reviewer agreed.

## Phase 4b

From Task 1 on, one reviewer per task, re-reviewed until nothing was left.
- **Task 0:** the ticket's verification stood as its reading. That was the
  `verify-ticket` check, by an agent that did not write the ticket (passes,
  with gaps G1 to G6, which the plan carries), together with the owner's
  sign-off of the plan.
- **Task 1:**
  - Must: QA's helpers in `internal/perceive/qa_resource_forward_test.go`
    lacked `t.Helper` (fixed).
  - Nits: the field walk shared as `sameExcept`; QA's dump compared exactly;
    `client/test/fold-resources.test.ts`'s header. One was left as it was:
    the `ResourceChanged` arm writes the actor's resource map in place, where
    the new arms write a new map (VTT-792).
  - Carried to Task 2: QA's step asserts the log folds every frame; the doc
    comments of `sameIncarnation` and `actorBelief`.
  - Carried to Task 3: the gateway's half of VTT-770 and VTT-772.
- **Task 2:**
  - Must: a sight correction sent a `ResourceChanged` to a current the log's
    fold cannot reach (below 0, or above a max above 0). This was latent on
    `main` through an `ActorAdded` re-add. Such a resource is now renewed by
    a removal and an addition; the recipe is in `docs/verification-debt.md`
    (2026-11-16).
  - Must: VTT-794 was false where the difference does not fit int32, and
    VTT-795 was ambiguous for no max. Both were withdrawn; VTT-796 succeeds
    them.
  - Must: a stray `.pyc` was staged (removed).
  - Shoulds: VTT-224 cited at QA's step; the entries citing the nameless
    test under VTT-481 and QA's kind test under VTT-781 dropped, since
    neither held its row; a doc comment dropped.
  - Nits: sorts named, a helper renamed, a double report removed.
- **Task 3:**
  - Must: VTT-797 and VTT-798 were false for a player's give, which is
    refused as not authorized first. Both were withdrawn; VTT-800 and
    VTT-801 say "from the DM or the agent".
  - Shoulds: evidence moved to the rows the tests hold; the take half of
    VTT-799 cited.
  - Nits: tool wording ("while the resource it spends lasts"), QA's header,
    `toolgen`'s shared resource override.
  - The carry from Task 1 closed without a gateway test: one command at a
    time, none reaches the fold's refusals of VTT-770 and VTT-772, since the
    gateway answers first. `giveSheetFault` refuses a held resource
    (VTT-762), and `validateGiveAbility` a key the ability does not spend, an
    overlong name among them (VTT-763), or any key with no ruleset
    (VTT-765); an unknown actor is refused before the `ResourceAdded` is
    reached; a take removes only what `orphanedBy` finds on the sheet. Within
    OD-6's window, two gives of abilities sharing a resource can each carry
    it past `giveSheetFault`, and the second batch's `ResourceAdded` meets
    VTT-770's refusal of a held resource. The gateway tests first cited for
    VTT-770 were dropped from its evidence. The rows' "appends nothing" half
    rests on `AppendBatch`, which persists nothing of a batch the fold
    refuses (`TestAppendBatchMidBatchValidationFailurePersistsNothing`).
  - Carried to this report: `giveSheetFault`'s advice (see "What could not
    be established").
  - Carried to the specification task: SPEC-007's renamed test and VTT-728.
- **Task 4:**
  - Must: the conformance list cases also drop the cost resource from the
    golden.
  - Must: VTT-803 was false where an earlier refusal answers first. It was
    withdrawn and VTT-804 succeeds it; D11's test asserts `Resolve`'s whole
    text.
  - Shoulds: VTT-802 cites `TestConformanceOverAdventuresGlob`; pointers at
    `sheetsFit`, `loadActors` and `validateAddActorWith`; a `toolgen` test
    renamed.
  - Nits: all fixed except one, a string comparison left as it was.
- **Task 5:**
  - Must: a QA comment said the console refuses only what is not an integer,
    which is false beside the int32 bound (fixed).
  - Should: the console copied `Namer.resource`'s naming; it now calls
    `namer(...).resource`.
  - Nits: a doc pointer, the current's refusal wording, test names,
    citations.
- **Task 6:**
  - Must: the new `isBatchCommand` comment described an unexported function
    (CLAUDE.md rule 10). It now keeps only its warning and pointers.
  - Shoulds: the stale block above the batch branch in `runCommandStep`
    deleted; the debt recipe's wording; an open debt entry for `remove_actor`
    and `remove_participant`.
  - Re-review: one sentence of that entry, fixed.
- **Task 7:** the ledger was read row by row. Every changed row went down,
  new files entered, and none rose or left.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Task 1 adds the `ResourceAdded` and `ResourceRemoved` draws to `internal/eventgen` | they landed in Task 2 | At Task 1's tree `sameIncarnation` still compared resources, so the campaign property test dropped a frame set: a drawn add re-introduced the actor, and the forwarded event was then a duplicate |
| D5: a name held at the same max at another current is sent a `ResourceChanged`, "as today" | a current below 0, or above a max above 0, is renewed by a `ResourceRemoved` and a `ResourceAdded` (VTT-796) | Task 2's reading review. `engine.Apply`'s `ResourceChanged` arm floors at 0 and caps at a max above 0, so a log's fold refused such a correction, the fan-out dropped the log's frame set, and `Resume` resent it. This was latent on `main` through an `ActorAdded` re-add |
| The plan's withdrawals: VTT-713, VTT-714, VTT-728, VTT-739, VTT-740, VTT-745 | also VTT-657, succeeded by VTT-793 | VTT-657 said a log is introduced to a party member with its resources; a party member differing only in resources is now corrected instead. The verification missed it |
| Task 5's first design: the value fields shown by a repaint at each choice | they show and hide on the choice itself, without a repaint; their drafts still survive a repaint, as D8 says | The repaint at each choice broke QA's repaint-count tests, and a repaint at each arrow key takes the select's focus |
| D8: Give refuses an empty current or one that is not an integer | it also refuses a max that is neither empty nor an integer (VTT-805) and an integer an int32 cannot hold (VTT-808), and it accepts "+3" | The max refusal is the implementer's, while Task 5 was staged: D8 said nothing of a max that is not an integer. The int32 bound is QA's gap G3: the server closes a connection on a frame it cannot decode. "+3" is QA's gap G1, adjudicated as an integer |
| Task 6's files: the scenario and its goldens | also `internal/harness/engine.go`: `isBatchCommand` names `give_ability` and `take_ability`, its doc comment is rewritten, and the stale block above the batch branch is deleted; a recipe and an open entry in `docs/verification-debt.md` | Since Task 3 both handlers append a batch whose `CommandResult` names the first sequence. The runner read one event, and the next denied step found the second as a leak. The touched comment block was longer than `check:comments` lets an edit into; its `load_map` history is in `docs/reports/2026-07-24-simulation-harness.md` |
| Task 6's first draft: `stamina` given back at its old max | at 1 of 3 | At the old max, the plan's break (the client's fold keeping the old max) would have stayed green |
| Task 6: `stream.json` recorded | its new events were written by hand | The notes record no reason. A recording would have rolled the earlier `fists` event's dice anew: the task's reviewer captured a fresh stream, identical byte for byte but for those dice. `TestScenarioGoldenStreamsHaveNotDrifted` holds the written events equal to a real run, masking the dice and re-marshalling both sides, so it compares no key order or whitespace; the byte capture covers those |
| Task 7: `task check` green on the ledger commit | it went red on `70b61f3` on a Go mutation survivor: in `resourceFrames`, `r.GetCurrent() > r.GetMax()` turned to `>=`. `42bd45a` adds `TestACurrentAtItsMaxOrAtZeroIsCorrectedByOneResourceChanged`, and the gate ran again on it | Task 2's tests held a current below zero and one above its max, but none corrected to exactly its max, where VTT-796 says one `ResourceChanged`. The mutant renews it by a `ResourceRemoved` and a `ResourceAdded`, which fold to the same sheet, so no test that compared only the fold could tell. The `<= 0` mutant was already killed by `TestQARCResumeComparesThePartyInSightByResources` |
| Task 3's break of VTT-787 (the walker naming `actor ""`) and Task 4's of VTT-778 (the rule run with no ruleset) | neither ran in its task; both ran on `42bd45a` while this report was written, and each test went red | The notes record no reason. Neither task's commit lists them, and nothing checks a commit's breaks against the plan's |
| The ticket names `internal/gateway/convert.go` | not changed | `handleGiveAbility` and `handleTakeAbility` take `ToEvent`'s envelope for the ability event as it was (D3) |

## What could not be established

- **The Story draws a correction's resource frames.** A log that missed a
  give or a take is corrected at the next sight by `ResourceAdded` and
  `ResourceRemoved` frames, and a current no `ResourceChanged` can land on is
  renewed by both. `reintroductions` (`client/src/view/feed.ts`) leaves out
  of the Story only a withdrawal followed by an introduction, with the
  control-grant and condition frames that restate it (`restates`), so
  `describe` draws each such frame ("<Name> no longer has <resource>",
  "<Name> now has <resource> at <current>"), even for a resource that, in a
  renewal, never left. Whether the Story leaves these lines out is the
  owner's to decide.
- **`giveSheetFault`'s advice can lead to a second refusal.** Take a sheet
  that names a limited ability without the resource it spends. A give of
  that ability carrying no value is refused with "give it with its value".
  The give with its value is then refused by the fold, since the sheet
  already names the ability. The entry rule keeps such a sheet out:
  `add_actor` and `adventure.Load` refuse it, and `vtt serve --ruleset`
  refuses a log holding one through `sheetsFit` before the gateway is given
  the ruleset; with no ruleset, `giveSheetFault` reads nothing. A served
  campaign comes to hold one only through OD-6's second edge, a give
  carrying no value accepted while the take that removes its resource
  lands. The DM then takes the ability and gives it with its value. The
  plan's D11 also names a log folded before the rule, which `sheetsFit`
  refuses.
- **The console's drafts carry across a change of ability.** A value typed
  for one choice stands under the next choice's resource name, visible in
  the field before Give (QA's gap G4). Whether a change of ability clears
  the drafts is the owner's to decide.
- **The last new `toy-brawl` give pins the resource refusal by order.** The
  give carrying `stamina` the sheet holds is also a second give of
  `chair-swing`. `giveSheetFault` reads the resources first, so the step
  holds the resource refusal. In `tavern-brawl` no other ability spends
  `stamina`, so no give isolates it.
- **`remove_actor` and `remove_participant` append batches the runner does not
  name.** This is recorded under "Open debt" in `docs/verification-debt.md`,
  and no scenario sends either command. Whether to add the arms and a
  scenario step is the owner's call on scope.
- **The client's stored mutation report is out of date.** Its stamp names
  `184d917`, which is older than this branch's base. Since then this ticket
  changed `client/src/fold.ts`, `client/src/commands.ts`,
  `client/src/view/dm.ts`, `client/src/view/spectator.ts` and
  `client/src/app.ts`. `task check` judges nothing while the report is out of
  date (SPEC-023); a full run is the owner's to ask for.

## What was deliberately left out

The ticket has no "What this is NOT for". What was left out was left out by
the plan's sort and the owner's rulings:
- **Attributes a roll reads:** the owner's 2026-11-15 ruling leaves them to
  the ruleset.
- **Two writers at once (OD-6, A):** two takes of abilities sharing a
  resource, each seeing the other on the sheet; or a give carrying no value
  while the take that removes its resource lands. This is near-theoretical
  with a small, repairable consequence: no code, no test, no debt entry (the
  2026-10-21 ruling).
- **A current over its max:** D10 keeps `add_actor`'s answer. Refusing it
  everywhere is a ticket of its own if the owner wants it.
- **The player's message names abilities only (OD-4, A);** the Story names
  the resource (VTT-790).
- **Facts for the specification task:**
  - a give's and a take's `CommandResult.sequence` is the batch's first
    event's, as `use_ability`'s is;
  - an accepted give clears both value fields whether or not they fed it.
- **Comments, carried to a commit of their own on this branch after the
  specifications:**
  - `Resolve`'s comment (`internal/rules/resolve.go`) says it never creates
    resource entries because that is `add_actor`'s job, which this ticket
    made incomplete;
  - the unexported `appendChange` and `orphanedBy`
    (`internal/gateway/ability_change.go`) carry doc sentences, which rule 10
    does not allow;
  - `client/test/fold-unit.test.ts`'s header leaves `tokenDescribed` and
    `whisperAdded` out of the arms the corpus never folds. Both predate this
    ticket; `toy-brawl` folds this ticket's `resourceAdded` and
    `resourceRemoved`.
- **The specifications** SPEC-007, SPEC-013, SPEC-016, SPEC-018, SPEC-021,
  SPEC-022 and SPEC-025 move in the specification task, written from this
  report (the plan's Task 8).
