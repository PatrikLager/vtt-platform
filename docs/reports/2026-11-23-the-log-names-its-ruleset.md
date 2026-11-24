# The log names the ruleset and the version a campaign is played under — report

Ticket: `docs/superpowers/specs/2026-11-23-the-log-names-its-ruleset-design.md`.
Plan: `docs/superpowers/plans/2026-11-23-the-log-names-its-ruleset.md`.
Branch `feat/the-log-names-its-ruleset` from main `8c06de7`. Commits:
`f004fe9` (Task 0: ticket, plan, register), `676f194` (Task 1: the version
and the load bounds), `62eec43` (Task 2: the event, both folds, the
projection, the words and the rebuilt bundle), `e424914` (Task 3: the
character logs), `b53fa12` (Tasks 4 and 5: the gateway writes the entry
first in a use's batch, and the goldens), `3e73975` (Task 4b: a take after a
change opens with the entry too, and the plan amended for it), `5e72741`
(the comment ledger) and `7f2281f` (the gate's fix: tests that reach three
branches of `internal/eventgen` the walks stopped reaching, and that file's
ledger row; the last code commit). Every code reference below is to
`7f2281f`. SPEC-026, which the contract's and the code's comments already
point at, is written from this report; SPEC-007, SPEC-012, SPEC-013,
SPEC-016, SPEC-018, SPEC-021, SPEC-022, SPEC-024 and SPEC-025 move with it.

## Done looks like

1. [x] **A ruleset declares its version.** `manifestJSON`
   (`internal/rules/load.go`) reads `version`; `loadManifest` refuses it
   absent or empty as `field "version": must not be empty`, naming the file,
   as it does `id` and `name`; `loadV2` copies it to `Ruleset.Version`
   (`internal/rules/format.go`). `rulesets/tavern-brawl/ruleset.json` and
   `rulesets/dnd45e-minimal/ruleset.json` declare `"version": "1"` (D12).
   - Kept as written: `internal/rules/ruleset_version_test.go`'s
     `TestARulesetKeepsTheVersionItDeclares` and
     `TestACaseVariantOfVersionIsReadAsTheVersion` (VTT-970).
   - Refused absent, empty or null, by the whole error naming the file and
     `version`: `TestARulesetWithoutAVersionIsRefused` (VTT-971).
   - Both shipped rulesets declare one:
     `internal/rules/qa_ruleset_version_test.go`'s
     `TestQARVEveryShippedRulesetDeclaresItsVersion` (VTT-970), and the
     conformance suite, which loads each.
   - The schema requires it: `TestTheRulesetSchemaDescribesARequiredVersion`
     (VTT-986), and `TestSchemaRequiredFieldsMatchLoaderEnforcement` runs a
     `version` case. A field beside it that the format does not define is
     still refused: `TestAnUndefinedFieldBesideTheVersionIsRefused` (VTT-973).
   - The sub-bullet: `checkManifestBounds` refuses an `id` or a `version`
     longer than 128 bytes and, under OD-3 A, a `name` longer than 256, each
     naming the field: `TestARulesetWhoseIDVersionOrNameIsPastItsBoundIsRefused`
     and `TestARulesetWhoseIDVersionAndNameAreAtTheirBoundsLoads` (VTT-972).
     The schema's `id` description states its bound:
     `TestQAPFSchemasStateTheBoundsLoadEnforces`'s ruleset id probe
     (VTT-625).
   - On `8c06de7` the new tests do not build, since `Ruleset` has no
     `Version`. The refusal they replace was measured there by the plan's
     P1 (`json: unknown field "version"`, for both shipped rulesets and
     `testdata/valid`), and a too-long id and name loading by P2.
2. [x] **An outcome names the rules that decided it.** `(*Server).rulesetNamed`
   (`internal/gateway/ruleset.go`) compares the snapshot's `RulesetID` and
   `RulesetVersion` with the served ruleset's, and returns a `RulesetNamed`
   of the served ruleset's id, version and name when either differs.
   `handleUseAbility` puts it first in the batch `rules.Resolve` returns,
   before the stamping loop, so one append holds the entry and then the
   outcome, and no other entry can fall between them.
   - `internal/gateway/ruleset_named_test.go`'s
     `TestAUseNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother`, its
     case "the log names none": a campaign directory served through
     `gateway.New(...).WithRuleset` under a copy of tavern-brawl written as
     R at version 1, a use, then the log read. The batch begins with R's
     entry, the `AbilityUsed` follows, no second entry is in it, and the
     result answers the entry's sequence (VTT-975).
     `TestUseAbilityHitProducesBatchFirstSequence` reads the batch off the
     DM's connection, `rulesetNamed` first (VTT-137, VTT-975).
   - Through the binary's own boot, `composeServer` by way of
     `cmd/vtt/harness_boot.go`: the `toy-brawl`, `door-watch` and
     `adventure-night` goldens open their first use's batch with the entry,
     and `cmd/vtt`'s `TestScenarioGoldenStreamsHaveNotDrifted` holds each
     recorded stream.
   - Independently: `internal/gateway/qa_ruleset_named_use_test.go`'s
     `TestQARUTheFirstResolvedUseAppendsTheEntryAndThenTheOutcomeInOneBatch`;
     a refused use appends neither,
     `TestQARUARefusedUseAppendsNothingNorTheEntry`.
   - The entry names its issuer:
     `TestTheEntryCarriesItsIssuerToTheDMAndNoneToAPlayer` (VTT-976).
   - On `8c06de7` these do not build (no `RulesetNamed`); no payload of
     `Envelope` named a ruleset there.
3. [x] **A correction shows where it began.** The "another version" case of
   `TestAUseNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother`: the same
   directory closed and served again under R at version 2, after the first
   `AbilityUsed`, and the next use's batch begins with version 2's entry.
   `TestAUseUnderTheRulesetTheLogNamesAppendsNoEntry`'s case "reopened under
   another version" does the same after uses that wrote none. Independently:
   `TestQARUAReopenUnderAnotherVersionOrIdNamesItAndARenameDoesNot`. Comparing
   the id alone turned the case red.
4. [x] **A switch of ruleset is recorded.**
   - The server starts: `cmd/vtt/campaign_fit_test.go`'s
     `TestACampaignWhoseLogNamesAnotherRulesetIsServed`, in which
     `composeServer` serves under tavern-brawl a campaign whose sheets fit it
     and whose log names `other-rules` (VTT-977). `composeServer` is
     unchanged.
   - The entry: the case "another id at the version the log names" of
     `TestAUseNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother`, Q at
     the version string the log last names (VTT-975); and
     `TestQARUAReopenUnderAnotherVersionOrIdNamesItAndARenameDoesNot`.
     Comparing the version alone turned the case red.
   - The halves are held by two tests: the `cmd/vtt` test uses no ability,
     and the gateway test serves through `WithRuleset`, the call
     `composeServer` makes, not through `composeServer`. The `cmd/vtt` test
     passed before Task 4's code, since `composeServer` reads no ruleset id;
     its break, `composeServer` refusing a log naming another id, turned it
     red.
5. [x] **Nothing is added when nothing changed.**
   - `TestAUseUnderTheRulesetTheLogNamesAppendsNoEntry`: a second use under
     R at version 1 appends no entry; reopened under the same, none;
     reopened under R at version 2 with another name and `fists`'s
     description changed, right after the log names version 2, none
     (VTT-978).
   - With no ruleset served:
     `TestAUseWithNoRulesetServedIsRefusedAndTheLogIsUnchanged`, a use
     refused as `gateway: no ruleset loaded`, the head unmoved and the log
     still naming R (VTT-132); a take,
     `TestATakeUnderTheRulesetTheLogNamesOrWithNoneServedAppendsNoEntry`,
     whose batch begins with its `AbilityTaken` (VTT-989); a give on a log
     naming none writes none and the use after it does,
     `TestAGiveOnALogNamingNoneAppendsNoEntryAndTheNextUseDoes`, and on a log
     naming the served ruleset, another version or another id, or with none
     served, QA's `TestQARTAGiveNeverWritesTheEntryAndTheNextUseStillDoes`
     (VTT-990).
   - The character logs: `internal/campaign/ruleset_named_log_test.go`'s
     `TestAReopenedCampaignWritesNoSecondRulesetNamed`.
   - As the ticket says, this held before the code that writes the entry.
     What shows the tests bite are the breaks: the entry prepended always,
     the name compared too, and `Snapshot`'s copy dropped each turned
     `TestAUseUnderTheRulesetTheLogNamesAppendsNoEntry` red.
6. [x] **Every participant is sent it.** `classify`
   (`internal/perceive/projection.go`) forwards a `RulesetNamed` in the
   `SessionStarted` and `SessionEnded` case, whatever the eyes see, and
   `forwardable` clears its issuer; `opening` and `resume`
   (`internal/campaign/fanout.go`) write one of the ruleset the world names.
   - Every log active at the entry:
     `internal/perceive/ruleset_named_internal_test.go`'s
     `TestEveryLogIsSentARulesetNamedWhateverItsEyesSee`, and
     `internal/campaign/ruleset_named_log_test.go`'s
     `TestALogOpenBeforeTheWorldNamesARulesetIsSentTheWorlds` (VTT-980).
   - A log that opens after it:
     `TestALogOpenedAfterTheWorldNamesARulesetOpensWithIt`, the entry right
     after the open session and before the party (VTT-981).
   - A log that resumes after it:
     `TestAPausedLogResumesWithTheRulesetTheWorldNamedWhileItWasPaused`, and
     `TestAPausedLogWhoseRulesetIsTheWorldsResumesWithNone` (VTT-982).
   - The seats, over the WebSocket:
     `TestQARUTheEntryReachesEverySeatAndNamesItsIssuerOnlyOnTheWorld`. The
     DM and the agent are sent it with its issuer (VTT-377, VTT-976); a
     player, and a spectator watching a character, are sent it with none
     (VTT-272).
   - Independently: `internal/perceive/qa_ruleset_named_test.go` and
     `internal/campaign/qa_ruleset_named_log_test.go`.
   - Read so: a spectator watching nobody is sent no event at all (SPEC-007,
     SPEC-015), so a spectator's stream is the log the spectator watches
     (the QA finding 2 of Tasks 4 and 5).
7. [x] **The client folds it as the server does.** `fold.ts`'s
   `rulesetNamed` arm bounds each field as `engine.Apply`'s does and sets
   `State`'s `RulesetID`, `RulesetVersion` and `RulesetName`;
   `foldToDumpJSON` writes the keys as `vtt state dump` does.
   - The goldens: `client/test/fold-parity.test.ts`'s "fold parity:
     ${name}" and `internal/harness`'s `TestFoldGoldenCorpus` over
     `scenarios/goldens`, whose `toy-brawl`, `door-watch` and
     `adventure-night` streams carry the entry and whose `state.json` names
     the ruleset. Each `state.json` was edited by hand before its
     `stream.json` was recorded (ADR-009) (VTT-983).
   - The dump fixtures: `cmd/vtt/ruleset_named_dump_test.go`'s
     `TestTheDumpAfterRulesetsNamedIsTheCommittedOne` and
     `TestEachRulesetNamedRefusalInTheCommittedFixtureTheServerRefuses`,
     beside `client/test/fold-ruleset-named.test.ts`'s "the dump after
     rulesets named matches the committed dump, as Go's writeDump does" and
     "each rulesetNamed refusal in the committed fixture, the client's fold
     refuses", over `contract/testdata/ruleset_named_log.json`,
     `ruleset_named_dump.json` and `ruleset_named_refused.json` (VTT-983,
     VTT-979, VTT-974).
   - A log naming none dumps nothing of one: `cmd/vtt`'s
     `TestTheDumpOfALogThatNamesNoRulesetWritesNothingOfOne`, and the client's
     "the dump of a log that names no ruleset writes nothing of one"
     (VTT-987).
   - The client fold setting the id and not the version turned fold parity
     red on `toy-brawl`, `door-watch` and `adventure-night`.
8. [x] **The gate.** `task check`, whole, is green on `7f2281f`; its first
   run, on `5e72741`, went red at `check:coverage` (the deviations).
   `check:mutation` reports zero unadjudicated survivors in each package it
   measures; `check:ts-mutation` judges nothing, because the client's stored
   report measured `d1ff728` and is out of date (SPEC-023). `task e2e` is
   green on `7f2281f`, with `handover.spec.ts`'s `test.skip` "a network drop
   while the table plays on ends in a Reconnect" the one test skipped.
   `--write-ledger` ran twice: in `5e72741`, and in `7f2281f` for the one
   file that commit adds; every row either changed is lowered or belongs to a
   file this ticket adds, and none is raised. `cmd/vtt/webdist` is rebuilt
   in `62eec43`, the one commit that changes `client/src`, and
   `check:drift`, a step of `task check`, finds no difference.

## What the rules became

The ticket's rules, each with what it became:
- A ruleset declares a version, and one that declares none, or an empty one,
  is refused when it loads: VTT-970 (kept as written), VTT-971 (absent or
  empty refused, naming the file and the field), VTT-986 (the schema says
  so), and VTT-973, which succeeds VTT-861 with `version` among the defined
  fields.
- A ruleset whose id or version the entry could not carry is refused when it
  loads: VTT-972, the name included under OD-3 A. The folds' half is
  VTT-974.
- An outcome is appended only after an entry naming the ruleset that decides
  it, none naming another between them: VTT-975 for a use, VTT-976 for the
  entry's issuer; after the owner's ruling of 2026-11-24, VTT-988 for a take
  and VTT-991 for the take's own events after it. Its second sentence,
  outcomes a log already holds stay as they are, earned no row: the log only
  grows, and nothing migrates (the 2026-09-04 ruling).
- A campaign served under a ruleset whose id or version differs gains an
  entry before its next outcome: the "another id or another version" clause
  of VTT-975 and VTT-988. The sort refused R7 as a row of its own: VTT-975's
  test has a case for each, and comparing the id alone or the version alone
  turns one red.
- A campaign served under the ruleset and version its log last names gains
  no entry: VTT-978 for a use, VTT-989 for a take. VTT-990 holds that a give
  never writes one.
- A campaign served with no ruleset gains no entry: refused at the sort, held
  by VTT-132, under which a use with no ruleset is answered ok=false; VTT-132
  gained `TestAUseWithNoRulesetServedIsRefusedAndTheLogIsUnchanged` and
  `TestQARUARefusedUseAppendsNothingNorTheEntry`, which assert the log
  unchanged. After Task 4b, VTT-989 holds it for a take.
- A campaign served under a ruleset of another id, whose sheets fit it, is
  served, not refused: VTT-977.
- Every seat and every character log is sent the entry, one that opens or
  resumes after it among them: VTT-980 (every active log), VTT-981 (an
  opening, succeeds VTT-663), VTT-982 (a resume, succeeds VTT-318). The DM's
  and the agent's streams were refused at the sort, held by VTT-377 (they are
  sent the world), which gained
  `TestQARUTheEntryReachesEverySeatAndNamesItsIssuerOnlyOnTheWorld`. That a
  log is sent it without its issuer is VTT-272's, which gained the perceive
  and gateway tests.

The plan's labels:

| Plan | Id | |
|---|---|---|
| R1 | VTT-970 | the version kept as written |
| R2 | VTT-971 | absent or empty refused |
| R3 | VTT-972 | id, version and name bounded at load |
| R4 | VTT-973 | the defined fields with `version`; succeeds VTT-861 |
| R5 | VTT-974 | both folds bound the entry |
| R6, with R7 and R8's entry | VTT-975 | a use's batch opens with the entry after a change |
| R8's other half | VTT-977 | served, not refused |
| R9 | VTT-978 | none under the id and version the log names |
| R10 | VTT-979 | both folds hold it, as `vtt state dump` prints it |
| R10's dump clause | VTT-987 | a dump of a log naming none writes nothing of one |
| R11 | VTT-980 | every active log |
| R12 | VTT-981 | an opening; succeeds VTT-663 |
| R13 | VTT-982 | a resume; succeeds VTT-318 |
| R14 | VTT-983 | the client's fold agrees with the dump |
| R15 | VTT-984, VTT-985 | the Story shows it (succeeds VTT-822); its words |

Added by the sort: VTT-976 (the use's entry carries its issuer); VTT-986
(the schema), which the sort refused and Task 0's review restored.
Allocated after the owner's ruling of 2026-11-24: VTT-988, VTT-989 and
VTT-990. From Task 4b's reading review: VTT-991, which succeeds VTT-767,
and VTT-992 (a take's result carries its batch's first sequence).

Withdrawn, each by the task that made it false: VTT-861 for VTT-973 (Task
1), VTT-822 for VTT-984 (Task 2), VTT-663 for VTT-981 and VTT-318 for
VTT-982 (Task 3), each on 2026-11-23; VTT-767 for VTT-991 (Task 4b,
2026-11-24). Rows kept that gained evidence: VTT-132, VTT-272, VTT-377 and
VTT-799, whose "each event of the batch" now covers a take's entry
(`TestATakeNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother`, which the
issuer break alone turns red). VTT-137 stays true, its test re-pointed;
VTT-625 stays true through the schema's `id` description; VTT-768 (with no
ruleset served, a take appends its `AbilityTaken` and nothing else) stays
true, since such a take writes no entry.

Refused at the sort, each held elsewhere: R7 (VTT-975); the shipped
rulesets' versions (VTT-971 and the conformance suite); the DM's and the
agent's streams (VTT-377); a campaign served with no ruleset (VTT-132); D5's
repeated entry (near-theoretical, the ruling of 2026-10-21). Refused during
the work: the order in which a `RulesetNamed`'s fields are refused (the
plan's D2, and SPEC-026); a window's exact words (the tests pin them, as the
precedents' sorts ruled). One refusal was reversed at Task 0's review: the
plan proposed that the schema's `required` needed no row, and
`TestSchemaRequiredFieldsMatchLoaderEnforcement` builds its cases from that
same `required`, so it stays green when `version` leaves it; VTT-986.

The open decisions, as the owner signed them off on 2026-11-23: OD-1 A, the
entry first in the batch of the first outcome after a change, not at boot;
OD-2 A, every window's Story draws it, with an event id or none; OD-3 A, the
entry carries the ruleset's name beside its id and version; OD-4 A, a new
SPEC-026. With them D3's `RulesetNamed`, D2's version bound of 128 bytes and
D12's `"1"` for both shipped rulesets. They answer the ticket's own "What
could not be established": the Story draws it (OD-2), and it is written with
the outcome, not at the boot (OD-1); the alpha campaign is under "What could
not be established" below.

The owner's ruling during the work: on 2026-11-24, asked by Task 4's reading
review, a `take_ability` after a ruleset change also opens its batch with the
entry, since the resource it removes is chosen by the ruleset's `Spends`; a
`give_ability` does not, since what it writes is the DM's. Task 4b carried
it.

## Rule 9

The ticket records it, with the verifier's corrections taken in before Task
0's commit (the plan's G7, measured as P13 at RPTool `maptool` `f4b7fef6c`).
An add-on library's `library.json` names a `namespace` and a free-text
`version`. `AddOnLibrary`'s constructor means to refuse a library with none
(`Objects.requireNonNull` with `library.error.emptyVersion`), but
`AddOnLibraryDto` is a proto3 message parsed by `JsonFormat`, whose string
getter never returns null, so the check never fires and MapTool loads a
library with an empty or absent version. A campaign file keeps each
library's current copy, stored under its MD5
(`PersistenceUtil.saveAddOnLibraries`, `loadAddOnLibraries`).
`AddOnLibraryManager.registerLibrary` refuses a second library under a
namespace, but the import dialog (`AddOnLibrariesDialogController`) asks
`library.error.addOnLibraryExists` and, on yes, deregisters the old copy and
registers the new: an operator replaces a library mid-campaign, and nothing
records which version decided an outcome, or when.

Borrowed: the version as free text the author declares and keeps true (the
owner's ruling 2 of 2026-11-23). Not borrowed: the copy kept inside the
campaign, since a ruleset stays a directory the operator serves; the MD5,
since nothing checks a ruleset's files under one version (ruling 2); the
empty version, refused here at load, which is the check MapTool meant and
does not make, so the non-empty rule is this platform's own; the silence, as
the log names the ruleset and each switch (ruling 1, principle 28).

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as verified | Revised before Task 0's commit to take in G1 to G9 and G7's MapTool corrections; the plan's verdict, checks and gaps describe the draft that was verified, and say so | The verifier found the gaps; Task 0's review found that the plan's status still described the draft as if it were the ticket committed (M1), and the plan gained the bullet that says which is which |
| OD-1 A and D9: only a use writes the entry, an outcome being the batch `rules.Resolve` returns | A take after a change opens its batch with the entry too (VTT-988), and a give never (VTT-990); Task 4b, `3e73975`, with the plan amended | Task 4's reading review: `handleTakeAbility` appends a `ResourceRemoved` the ruleset's `Spends` chooses (`orphanedBy`) and did not call the helper. The owner ruled on 2026-11-24 that a take's resource is the ruleset's choice and a give's is the DM's. VTT-767 ("and nothing else") was false once a take opens with the entry, as VTT-714 was before it, so it was withdrawn for VTT-991; no row said a take's result carries its batch's first sequence, so VTT-992 |
| Item 1's sub-bullet bounds the id and the version at load | The name is bounded too, at 256 bytes, at load and in both folds (VTT-972, VTT-974) | OD-3 A puts the name in the entry; the folds bound what an entry carries (SPEC-018), and a bound set after the first log holds the event would refuse that log (principle 5). The plan's D2 |
| D10(a): `version` decodes and nothing else changes before step (d) | Step (a) also added `version` to the schema's `properties` and to `TestQASheetSchemaDescribesTheSheet`'s field list; `required` and the count waited for step (e) | `TestSchemaPropertiesCoverFixtureFields` requires every field of `testdata/valid/ruleset.json` to be a schema property, and `TestQASheetSchemaDescribesTheSheet` pins the top-level field list, so step (d) could not pass otherwise |
| Task 2's files: the envelope fixture | `cmd/vtt/ruleset_named_dump_test.go` and `contract/testdata/ruleset_named_log.json`, `ruleset_named_dump.json` and `ruleset_named_refused.json` added; VTT-983 closed in Task 2 rather than Task 5 | VTT-979 says "as `vtt state dump` prints it" and VTT-987 names `vtt state dump`, so Go's `writeDump` folds the fixture the client test folds, the pattern VTT-774's dump test set |
| D6: `classify` gains an arm of its own, with a pointer | `RulesetNamed` joins the `SessionStarted` and `SessionEnded` case, with no comment; the pointer to SPEC-026 sits on the proto message and on `engine.State`'s fields | An arm of its own put `classify` over `gocyclo`'s ceiling, and a comment line put `projection.go` over its comment ceiling; rule 2 moves neither |
| R15: a window words it "⟨name, else id⟩" | `describe` words it by name alone: `rules: <name>, version <version>` | Under OD-3 A both folds refuse an empty name, so an entry a window holds always has one; VTT-985 says name and version |
| D4: `foldToDumpJSON` writes each key only when non-empty | It writes the keys unconditionally, with a warning to keep them optional in `State` | Task 2's reading review: the guards `st.RulesetID ? {...} : {}` were equivalent mutants, since `JSON.stringify` drops an undefined key as Go's `omitempty` drops `""`. The break `?? ""` turned the VTT-987 tests and fold parity on every golden red |
| D11: the walks meet the entry at openings and resumes | They meet it at openings and as forwarded entries, never at a resume; the oracle's `pausingHistory` gained a `RulesetNamed` before the pause and one of another version during it | `grantControl` always grants a party member, ids are never reused, and a revoke leaves the kind, so a walk pauses logs and never resumes one. The oracle takes each seat's lead from the log, so it folds the resume's entry but does not hold VTT-982 (Task 3's review); the named resume tests do |
| D11's draw | `nameRuleset` takes a band carved from narration's in `Step` and spends no random draw; every walk after its first draw in the band differs from base's | The bands share one draw; the property, oracle and model guards pass on the shifted walks |
| Task 6: `task check` green on the ledger commit | The first run, on `5e72741`, went red at `check:coverage`: `internal/eventgen` fell under its floor; `7f2281f` adds `internal/eventgen/draws_internal_test.go`, and the floor is unchanged | The draw's band changes every walk after its first draw in it, so the package's fixed seeds stopped reaching three branches of `model.go` that were there before: `closeDoor`'s tie-break on the row, `removeActor`'s arm with nothing removable, and `takeAbility`'s fallback to the model's actors. Each is now reached by a test that asserts what the code promises there: `closeDoor`'s and `removeActor`'s warnings at the branch, and for `takeAbility`'s fallback `changeAbility`'s doc comment (`TestACloseDrawsTheSameOpenDoorForTheSameSeed`, `TestARemovalWithEveryActorOnTheBoardDrawsNothingOrARefusal`, `TestATakeWhenNoActorNamesAnAbilityDrawsARefusedTakeFromAKnownActor`); dropping the sort, making the row tie-break return 0, making the arm remove a held actor, and changing the absent id each turned its test red. The package's uncovered blocks are again the ones it had at `62eec43` |
| D9: the helper's doc warns that every path appending an outcome calls it first | It reads "Prepend its entry to each use's and take's batch; needs s.ruleset (SPEC-026)." | `internal/gateway/ruleset.go`'s comment share stood at its ceiling (rule 2). Task 4's review found "every batch appending an outcome" named no bound, and SPEC-026 did not exist yet; Task 4b's named both callers. The warning to a later outcome path is SPEC-026's |
| Task 4: the renamed case after the switch to Q | `TestAUseUnderTheRulesetTheLogNamesAppendsNoEntry` serves the renamed R at version 2 right after R at version 2 | After Q, whose id differs, an entry would be correct, and the case would test nothing |
| Tasks 4 and 5 as two commits, Task 4's leaving the goldens drifted until Task 5 | `b53fa12` holds both | The goldens' recorded streams follow from Task 4's code, so one commit keeps every commit's tree free of a drifted golden; the hand-edited `state.json` files were written before the streams were recorded, and `TestFoldGoldenCorpus` passed against them as they stood |
| Task 4's "True when": the new tests fail on `8c06de7` | They fail there only because they do not build; at `e424914`, before Task 4's code, `TestACampaignWhoseLogNamesAnotherRulesetIsServed` already passes, and only its break makes it bite | `composeServer` never read a ruleset id. Like item 5, the test guards code that might change, not code this ticket writes |

## History cut from the code (rule 10)

`TestUseAbilityHitProducesBatchFirstSequence`
(`internal/gateway/ruleset_test.go`) carried a doc comment and an in-body
block describing the test and its batch, with each event's sequence; once the
batch opens with the entry, one of their sentences was false. Both are cut to
a warning, "Keep patron's footing at 0 (newRulesetFixture): a miss changes
the batch.", and the ids VTT-137 and VTT-975. What they said lives in the
test's own `wantKinds`, in tavern-brawl's `ruleset.json` and
`abilities/fists.json`, and in SPEC-012's account of a use's batch. The QA
comments that pointed at the owner's rulings now point at row ids and
SPEC-026 (Task 4's review). Every other comment the ticket touched was
re-pointed from a withdrawn row to its successor or reworded in place.

## The deliberate breaks

Each applied, seen red, and restored byte for byte, as each commit records:
- Task 1: the empty check dropped (`TestARulesetWithoutAVersionIsRefused`,
  and `TestSchemaRequiredFieldsMatchLoaderEnforcement`'s `version` case);
  `loadV2`'s copy of `Version` dropped (the keep, case-variant and at-bound
  tests); each bound dropped in turn (that field's case of
  `TestARulesetWhoseIDVersionOrNameIsPastItsBoundIsRefused`, and for the id
  the schema probe); `>` made `>=` on each bound (the at-bound test);
  `version` dropped from the schema's `required`
  (`TestTheRulesetSchemaDescribesARequiredVersion`); the `id` description's
  bound dropped (the ruleset id probe).
- Task 2: each bound dropped in either fold, and `>=` at a bound (that fold's
  refusal tests and the committed fixture's refusal test); `Snapshot`'s copy
  dropped (the engine ruleset tests); `classify` withholding the entry (the
  perceive tests); soak's `statesEqual` without the compare
  (`TestStatesEqualTellsApartTheRulesetEachNames`); the client dump writing
  the keys when empty (the VTT-987 tests and fold parity on every golden);
  Go's `omitempty` dropped (the Go dump test); `STORY`'s entry made false,
  and `describe` wording it by id (the Story and wording tests).
- Task 3: the opening's entry dropped, or put after the party
  (`TestALogOpenedAfterTheWorldNamesARulesetOpensWithIt`,
  `TestAReopenedCampaignWritesNoSecondRulesetNamed`); the resume's dropped
  (`TestAPausedLogResumesWithTheRulesetTheWorldNamedWhileItWasPaused`);
  written whenever the world names one, or the name compared too
  (`TestAPausedLogWhoseRulesetIsTheWorldsResumesWithNone`); the id alone or
  the version alone compared (one case each of the changed-version test);
  `classify` withholding it
  (`TestALogOpenBeforeTheWorldNamesARulesetIsSentTheWorlds`); the eventgen
  band dropped (`TestAWalkDrawsEveryKind`). None turned the oracle or the
  property tests red, which check folding at every prefix, not what an
  opening holds.
- Tasks 4 and 5: the prepend dropped (the VTT-975, VTT-976 and VTT-978 tests
  and `TestUseAbilityHitProducesBatchFirstSequence`); the id alone, or the
  version alone, compared (the matching case of
  `TestAUseNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother`); the entry
  prepended always, or the name compared too
  (`TestAUseUnderTheRulesetTheLogNamesAppendsNoEntry`); the entry stamped
  without its issuer (`TestTheEntryCarriesItsIssuerToTheDMAndNoneToAPlayer`);
  `Snapshot`'s copy dropped (the second-use case); `composeServer` refusing a
  log naming another id (`TestACampaignWhoseLogNamesAnotherRulesetIsServed`);
  the client fold setting the id and not the version (fold parity on
  `toy-brawl`, `door-watch` and `adventure-night`).
- Task 4b: the take's entry dropped
  (`TestATakeNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother`); written
  always (`TestATakeUnderTheRulesetTheLogNamesOrWithNoneServedAppendsNoEntry`
  and the re-pointed take tests, so the re-pointing hides no extra entry); the
  served-ruleset guard dropped (a nil-pointer panic closed the connection, and
  the VTT-989 test went red); the entry put in a give's batch
  (`TestAGiveOnALogNamingNoneAppendsNoEntryAndTheNextUseDoes` and the give
  tests); the entry stamped without its issuer (the VTT-988 test alone, which
  is why it joined VTT-799's evidence).
- The gate's fix, `7f2281f`: the sort dropped, and the row tie-break made to
  return 0 (`TestACloseDrawsTheSameOpenDoorForTheSameSeed`); the
  nothing-removable arm removing a held actor
  (`TestARemovalWithEveryActorOnTheBoardDrawsNothingOrARefusal`); the absent
  ability id changed
  (`TestATakeWhenNoActorNamesAnAbilityDrawsARefusedTakeFromAKnownActor`).

## The existing tests that changed

- **Every manifest declares a version** (D10): each `ruleset.json` under
  `rulesets/`, `internal/rules/testdata`, `internal/rules/conformance/testdata`,
  `internal/adventure/testdata` and `internal/adventure/conformance/testdata`,
  and each manifest written inline in a Go test, gained `"version": "1"`.
  After step (e), `git grep -L '"version"' -- '*ruleset.json'` prints only
  `internal/rules/testdata/invalid/bad-format-version` and
  `invalid/format-version-1`, which are refused on `format_version` before
  any other field is read.
- **The schema's field list:** `internal/rules/qa_declared_sheet_test.go`'s
  top-level field list and its `required` (Task 1).
- **The lists of every kind**, which name kinds by hand: `app.test.ts`'s
  `EVERY_KIND`, `qa-story-mechanical.test.ts`'s script and kept kinds,
  `qa-what-a-window-shows.test.ts`'s map and `spectator-view.test.ts`'s
  wording test gained `rulesetNamed`. `qa-companion-first-sight.test.ts`'s
  `MECHANICAL` did not, as the entry is not mechanical (Task 2).
  `internal/eventgen/model_test.go`'s `TestAWalkDrawsEveryKind` gained it
  (Task 3); `internal/campaign`'s test helpers `cenv` and `describe` gained
  `RulesetNamed` and `AbilityUsed`, which stands for an outcome in those tests.
- **The gateway tests that read the first event at a use's sequence** (the
  plan's P6): `TestUseAbilityHitProducesBatchFirstSequence` now expects
  `rulesetNamed` first; `TestQARuleAUseWhoseIDsAreAtTheBoundIsRecordedWhole`,
  and `qa_use_unnamed_test.go`'s `qaUseResolved`, which
  `TestQAUseEachRoleIsRefusedAnAbilityTheSheetDoesNotName`,
  `TestQAUseTheRefusedUsesWouldResolveForAnActorWhoseSheetNamesThem` and
  `TestQAUseEachRoleResolvesAUseTheSheetNames` share, find the `AbilityUsed`
  by kind (Task 4).
- **The goldens** `toy-brawl`, `door-watch` and `adventure-night`: each
  `state.json` gained the ruleset's keys and moved by one every sequence the
  plan's P12 names at or after the first use's batch, by hand, before each
  `stream.json` was recorded through `captureNormalizedStream`
  (`cmd/vtt/scenario_goldens_test.go`), run through a `go test -overlay` kept
  outside the tree, the method of
  `docs/superpowers/plans/2026-11-22-a-scenario-participant-becomes-a-player-only-by-a-grant.md`.
  `TestFoldGoldenCorpus` passed against the hand edits as they stood.
  `adventure-night`'s entry carries no session id, since that scenario opens
  no session.
- **The take tests that read the batch by position or counted the head's
  move** (Task 4b): the shared fixture's take tests,
  `TestATakeRemovesTheResourceTheTakenAbilityAloneSpends`,
  `TestATakeKeepsAResourceAnotherAbilityOnTheSheetSpends`,
  `TestATakeRemovesNoResourceNoAbilitySpends` and
  `TestATakeThenAUseIsRefusedForTheDMTheAgentAndThePlayer`, go through
  `acceptedTake`, which skips a leading `RulesetNamed` and still counts the
  take's own events exactly; a take after the first keeps
  `acceptedAppending`, which an extra entry turns red. The QA fixtures
  `qaAVOpen` and `qaabNewTable` seed a `RulesetNamed` of the served ruleset,
  only when one is served, so their takes write none.
- **Citations:** each test and comment citing VTT-861, VTT-822, VTT-663,
  VTT-318 or VTT-767 now cites its successor, and `rules.Ruleset.Spends`,
  `orphanedBy` and `ShownUsage.UnmarshalJSON` with them.

## Phase 4a adjudications

- Task 1 (`internal/rules/qa_ruleset_version_test.go`): no defect.
  - QA finding 1: spec gap — "as written" is the decoded JSON string —
    SPEC-024 reads JSON — no change.
  - QA finding 2: spec gap — a non-string version is refused by the decoder —
    no change, no row.
  - QA finding 3: a null version read as absent, as SPEC-024 reads null —
    asserted under VTT-971 — no change.
  - QA finding 4: case-insensitive matching follows `encoding/json`'s folding,
    U+017F included — SPEC-024's existing behaviour — no change.
  - QA finding 5: two case variants of one member, the last wins —
    `encoding/json`'s, near-theoretical — no change.
  - QA finding 6: JSON Schema's `maxLength` counts code points, so the bound
    is stated in the description, VTT-625's convention — no change.
  - QA finding 7: several fields past their bound, the id named first — no
    row — no change.
  - QA finding 8: an unknown field inside a property entry is outside
    VTT-973 — SPEC-024's existing rule — no change.
  - Needs an id, the shipped rulesets declaring a version — refused at the
    sort, held by VTT-971 and the conformance suite — stands.
- Task 2 (`internal/engine/qa_ruleset_named_test.go`,
  `internal/perceive/qa_ruleset_named_test.go`,
  `client/test/qa-ruleset-named.test.ts`): no defect. QA's client tests
  titled by template literals (the bounds, `describe`, the Story per role)
  were left uncited at Task 2, on the mistaken belief that the requirements
  chain cannot cite them; it can, and the report's reading review had them
  cited in VTT-974, VTT-979, VTT-984, VTT-985 and VTT-987.
  - QA finding 1: the refusal order (id, version, name) is in no row — the
    plan's D2 and SPEC-026 state it — no row.
  - QA finding 2: the refusal text names the bound and the length as
    SPEC-018's table does — the specification task adds `RulesetNamed`'s
    fields to that table (G4) — no row.
  - QA finding 3: the exact words — pinned by the implementer's tests; VTT-985
    says name and version — no change.
  - QA finding 4: the dump's shape — held byte for byte by
    `cmd/vtt/ruleset_named_dump_test.go` — no change.
  - QA finding 5: the frame's fields — SPEC-016's `forwardable` — no change.
  - QA finding 6: which roles draw the Events panel — `app.test.ts`'s
    every-role test, in VTT-984's evidence — no change.
  - QA finding 7: a value of whitespace alone is non-empty free text, as for
    every other bounded field — no change.
  - Needs an id, the DM's and the agent's world connection — held by
    VTT-377 — no row.
- Task 3 (`internal/campaign/qa_ruleset_named_log_test.go`): no defect.
  - QA finding 1: with no open session, the entry is the opening's first,
    before the party — no change; SPEC-021 states it.
  - QA finding 2: VTT-981, as VTT-663 did, lists "the public notes and each
    party member" while the opening writes the party, then the notes — the
    row lists contents, not an order — no change; SPEC-021 states the order.
  - QA finding 3: a log paused across a rename at one id and version keeps
    the old name in its fold, since only the id and the version are compared
    (the owner's ruling) — no change; SPEC-021 and SPEC-026 say it.
  - QA finding 4: the resume's entry carries no event id, session or time,
    by SPEC-021's rule for entries no event caused — no row.
  - Needs an id, the entry's place — a how, held by the opening and resume
    tests' exact order — no row.
- Tasks 4 and 5 (`internal/gateway/qa_ruleset_named_use_test.go`, through the
  WebSocket): no defect.
  - QA finding 1: a use's result answers the entry's sequence, its batch's
    first (SPEC-007) — SPEC-007, SPEC-012 and SPEC-026 say so — no row.
  - QA finding 2: a spectator watching nobody is sent no event at all
    (SPEC-007, SPEC-015); "every participant" is read through the logs a seat
    reads — no change; SPEC-026 says it.
  - QA finding 3: VTT-977's "is served" is `composeServer`'s
    (`TestACampaignWhoseLogNamesAnotherRulesetIsServed`); through the gateway
    only the accepted, named use shows — no change.
  - QA finding 4: versions compare as strings, byte for byte, so "1" and
    "1.0" differ — the owner's ruling (free text, the author's) — no row;
    SPEC-026 says it.
  - QA finding 5: two first uses at once may both append an entry — D5,
    near-theoretical (2026-10-21) — no change.
  - QA finding 6: an outcome is the batch `rules.Resolve` returns; give,
    take, `remove_condition`, `add_actor` and `load_adventure` wrote no entry
    — SPEC-026 says it — no change then; the take changed with the ruling of
    2026-11-24 (Task 4b).
  - QA findings 7 and 8: the entry's envelope fields, and a batch the fold
    refuses — the stamping loop and `AppendBatch`'s atomicity (SPEC-012) —
    no change.
  - Needs an id: no entry at boot — held by the heads the gateway and QA
    tests read; the result's sequence — SPEC-007's convention; the refusal
    texts — VTT-132 and VTT-690 — no new rows.
- Task 4b (the `TestQART` tests in the same file): no defect.
  - QA finding 1: a take's result answers the entry's sequence — SPEC-007's
    convention — no row then; the reading review then added VTT-992.
  - QA finding 2: VTT-799's "each event of the batch" covers the take's
    entry, and the issuer break turns only the VTT-988 test red — that test
    joined VTT-799's evidence — no new row.
  - QA finding 3: a take with no ruleset served, on a log naming one, writes
    no entry and removes no resource, as no ruleset decided anything — no
    change; SPEC-026 says it.
  - QA finding 4: only a use and a take write the entry (the owner's
    rulings) — SPEC-026 names the others — no change.
  - QA finding 5: VTT-767's shared-resource branch — held by
    `TestATakeKeepsAResourceAnotherAbilityOnTheSheetSpends`, now VTT-991's —
    no change.
  - QA finding 6: two takes at once — D5, near-theoretical — no change.

## The reading reviews

One reviewer per task, re-reviewed until nothing was left at Must or Should.
- Task 0: the verifier's check, by an agent that did not write the ticket
  (passes, with gaps G1 to G9), then a reading review. Musts: the plan's
  status described the draft, not the ticket committed (M1); the wording of
  VTT-970 against VTT-972 (M2); VTT-625's schema id probe (M3); the schema's
  `required`, held by no test, became VTT-986 (M4). Shoulds: VTT-987 (S1);
  VTT-975 and VTT-978 read against the state the use was resolved against,
  so D5's race leaves both true (S2, S3); VTT-976's and VTT-977's breaks (S4);
  the alpha sentence (S5). A second round: the status bullet's scope,
  `qa_property_format_test.go` "unchanged", the dump's keys, and "resolves to
  an outcome".
- Task 1: Must — VTT-971 cited `TestQARVEveryShippedRulesetDeclaresItsVersion`,
  which holds VTT-970 alone (with the empty check removed it stayed green);
  the citation went and its comment now names VTT-970. Should — a comment
  naming "the ticket's Done item 1" named no ticket. Nits: `seen < 2` made
  `seen == 0`; `qarvOfBytes`'s doc; the schema's `name` bound held by no test
  (see "What could not be established").
- Task 2: no Must. Should — the dump's keys (the deviations). Nits: the
  version fixtures made multi-byte in the Go test, the client's
  `fold-rejections.test.ts` and `ruleset_named_refused.json`; a comment now
  says `engine.State`; a double blank line.
- Task 3: Must — QA comment lines over `check-new-prose`'s column band, since
  the implementer ran the check before QA's file existed and the step runs
  only in `task check`; rewrapped. Should — the claim that the walks hold
  VTT-982, corrected (the deviations). Nits: a pointer "(SPEC-021, VTT-272)";
  descriptive test-helper comments left, as the package's precedent; `resume`'s
  `before.RulesetID != ""` guard decides nothing on its own, since the
  comparison is equal when both name none and a world never stops naming one,
  and was left, as it matches the row.
- Tasks 4 and 5: no Must. Shoulds — `rulesetNamed`'s doc comment named no
  bound and pointed at nothing yet written (the deviations), and the reviewer
  asked whether a take should write the entry, which went to the owner (the
  2026-11-24 ruling, Task 4b); the P6 test's comments described the test and
  one sentence was false (the history cut). Nits:
  `TestTheEntryCarriesItsIssuerToTheDMAndNoneToAPlayer` joined VTT-272's
  evidence; QA comments pointing at rulings now point at ids and SPEC-026;
  the stamping comment says it stamps the entry's issuer (VTT-976).
- Task 4b: Must — VTT-767 false as worded (withdrawn for VTT-991, its evidence
  and the code's citations in `internal/gateway`, `internal/rules`'s
  `sheet.go` and `spends_test.go` and `cmd/vtt`'s MCP test moved). Shoulds —
  the plan said VTT-989's and VTT-990's tests fail on `b53fa12`, and only
  VTT-988's do; the take's batch added to the specification task for SPEC-007,
  SPEC-013 and SPEC-025, and the plan's "one writer" refusal amended;
  `handleTakeAbility`'s and `rulesetNamed`'s doc comments; VTT-992; the give
  test renamed for what it covers, a log naming none, QA covering the rest.
  Nits: the search behind "toy-brawl alone takes" named; a count in a comment
  removed; nothing checks a give's or a take's `occurred_at` (see "What could
  not be established").

## The plan's gaps

- **G1.** `describe` has no `default`, so `client:typecheck` required the
  kind worded; the Events panel draws it in the DM's and the agent's window,
  and under OD-2 A every window's Story draws it (VTT-984), worded by OD-3 A
  (VTT-985).
- **G2.** Logs that open or resume after the entry: D7, VTT-981 and VTT-982,
  observed under item 6.
- **G3.** SPEC-012 moves; the specification task's.
- **G4.** SPEC-018's sentence leaving the manifest's id and name unbounded is
  reversed for the id, the version and the name; the specification task's.
- **G5.** The files reached are the plan's list. The walkers that iterate the
  oneof (`TestEveryEnvelopePayloadArmHasAnExplicitRuling` among them) took the
  new arm with no edit; `TestNoEventPayloadNamesARole` never went red. The
  lists that name kinds by hand do not turn red on a new kind: `EVERY_KIND`
  and `qa-story-mechanical.test.ts`'s lists gained it, `MECHANICAL` did not
  (see "What was deliberately left out"). `cmd/vtt/serve_compose.go`,
  `cmd/vtt/harness_boot.go`, `internal/harness`'s runner and
  `client/e2e/setup.ts` are unchanged, as G5 said.
- **G6.** D10 edited the set P5 measured, not what `git grep -l
  format_version` lists.
- **G7.** Taken into the ticket before Task 0's commit; see "Rule 9".
- **G8.** The alpha campaign; see "What could not be established".
- **G9.** The ticket's third rule gained "Outcomes a log already holds stay as
  they are" before Task 0's commit, and VTT-975 is worded for an outcome the
  server resolves now.
- **P6 and P12** matched what Tasks 4 and 5 met: the tests P6 named were the
  ones re-pointed, and the sequences P12 named were the ones that moved.

## What the next full mutation run should find

The client's stored mutation report measured `d1ff728` and was already out
of date at `8c06de7`, where the Delete-note and Remove-condition tickets had
changed `client/src`; Task 2 changes it again. While it is, `task check`
judges no client report and names that commit and the tickets added since,
this one among them (SPEC-023).
A full run is the owner's to ask for.

The client code this ticket adds is `fold.ts`'s `rulesetNamed` arm,
`foldToDumpJSON`'s ruleset keys, `STORY`'s `rulesetNamed` entry and
`describe`'s arm. No survivor is answered ahead of a run that names it, and
`tools/ts-mutation-equivalents.txt` is not edited. The one shape known to
make equivalent mutants is gone: `foldToDumpJSON` first wrote each key behind
`st.RulesetX ? {...} : {}`, and since `JSON.stringify` drops an undefined key,
no test could tell those guards' mutants from the code (Task 2's review); the
keys are now written unconditionally, under a warning to keep them optional
in `State`. A survivor on those keys would mean that warning no longer holds.
QA's client tests whose titles are template literals run in the suite and
are cited in the rows they hold.

## What could not be established

- **The resume's entry over random histories.** No walk resumes a paused log
  (the deviations), so VTT-982 is held by the named resume tests and QA's
  `TestQARLAPausedLogResumesToTheWorldsRuleset` and
  `TestQARLABatchResumingAfterNamingARulesetResumesWithIt`, by no random
  history, and not by the oracle, which reads each lead from the log. Whether
  the walks should learn to resume is not decided.
- **A give's and a take's `occurred_at`.** Nothing checks that either batch's
  events carry a time (Task 4b's review); the gap is older than this ticket.
  `TestTheEntryCarriesItsIssuerToTheDMAndNoneToAPlayer` checks the use's
  entry's time; the take's entry is checked for its issuer alone. No row holds
  it; it is recorded in `docs/verification-debt.md` ("Nothing checks the time
  a give's or a take's batch carries"), with its recipe.
- **The schema's `name` bound.** `ruleset.schema.json`'s `name` description
  states 256 bytes, and no test reads it: VTT-625's test covers ids, and the
  QA probe's scope was not widened to the name (Task 1's review). No row
  holds it; it is recorded in `docs/verification-debt.md` ("Nothing checks
  the bound `ruleset.schema.json` states for a ruleset's `name`"), with its
  recipe.
- **SPEC-026 does not exist at `7f2281f`.** `events.proto`, `engine.State`,
  `client/src/state.ts`, `fanout.go`'s `resume`, `rulesetNamed`'s doc and two
  `// SPEC-026` comments in `qa_ruleset_named_use_test.go` point at it, and
  specifications still cite withdrawn rows: SPEC-016 and SPEC-021 VTT-663 and
  VTT-318, SPEC-022 VTT-822, SPEC-024 VTT-861, SPEC-013 and SPEC-025 VTT-767
  (`git grep -n -E 'VTT-(663|318|861|822|767)' 7f2281f --
  docs/specifications`). All resolve with the specification task that follows
  this report.
- **MapTool was not read again for this report.** "Rule 9" rests on the
  verifier's P13 at RPTool `maptool` `f4b7fef6c`.
- **The alpha campaign (G8).** `~/dev/vtt-alpha`, outside the repository, is
  served under `rulesets/dnd45e-minimal` from the shared tree. Once it is
  served from a tree holding this change, its next use, or its next take,
  opens with the entry; the outcomes already in its log name no ruleset and
  stay so. When it is served is the owner's to decide.

## What was deliberately left out

- **The ticket's "Not this ticket":** a ruleset's options (ADR-012 §1), which
  no ruleset declares yet; the version in `/api/ruleset` or `get_ruleset`; the
  ruleset an adventure declares (`adventure.json`'s `ruleset`, an id with no
  version); the ruleset `vtt mcp --ruleset` loads for the agent's own use.
- **By the owner's rulings:** a log whose eyes see none of a use's or a
  take's actors is still sent the entry that opens its batch, at that point:
  asked after this report's review whether that breaks principle 17, the
  owner ruled on 2026-11-24 that which ruleset is in force is mechanical, not
  the story, and every participant sees it change; no fingerprint or digest
  of a ruleset's files under one version (ruling 2 of 2026-11-23); a switch
  of id recorded, not refused (ruling 1); a give writes no entry
  (2026-11-24); no migration, so a log's earlier outcomes name no ruleset
  (2026-09-04).
- **Other writers:** `remove_condition`, `add_actor` and `load_adventure`
  write no entry; an outcome is the batch `rules.Resolve` returns, and a
  take's after the 2026-11-24 ruling. No gate makes a later outcome path,
  ADR-012 step 2's, call `rulesetNamed`; its doc names its callers, and
  SPEC-026 carries the warning.
- **Near-theoretical, with no code, test or debt entry (the ruling of
  2026-10-21):** two first uses, or takes, at once each appending an entry
  (D5).
- **No semgrep change (D13):** `no-direct-state-writes` does not see a write
  to a string field of `State`; gate work is paused (2026-08-27).
  `git grep -n -E '\.Ruleset(ID|Version|Name)[^=!]* = ' -- '*.go'
  ':!*_test.go' ':!contract/gen'` finds the new fields written only in
  `internal/engine`'s `Apply` and `Snapshot`, and `git grep -n -E
  '\bRuleset(ID|Version|Name):'` over the same files finds no struct literal
  that sets them.
- **The hand-kept kind lists** (G5): `MECHANICAL`, `EVERY_KIND` and
  `qa-story-mechanical.test.ts`'s lists do not turn red on a kind the
  contract adds; making them do so is gate work, which the owner paused on
  2026-08-27.
- **Rows refused:** the order a `RulesetNamed`'s fields are refused in, and a
  window's exact words.
- **The client's full mutation run**, the owner's to ask for.
- **The specifications:** SPEC-026, new, and SPEC-007, SPEC-012, SPEC-013,
  SPEC-016, SPEC-018, SPEC-021, SPEC-022, SPEC-024 and SPEC-025, in the
  specification task that follows this report (the plan's Task 7).
