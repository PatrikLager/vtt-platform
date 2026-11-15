# Plan: a limited ability comes with its resource, and leaves with it

- **Ticket:** `docs/superpowers/specs/2026-11-15-an-ability-is-given-with-its-value-design.md`
  (blueprint principles 2, 3, 4, 6, 16, 17, 26, 27 and 34).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-15 by an agent that did not write the ticket,
    read-only against `001bdad` on `feat/an-ability-is-given-with-its-value`
    in the worktree `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and gremlins were not run. Probes P1 and P3 ran in a
    copy of the tree in the session's scratchpad, never in the worktree.
- **To be committed as:** `docs/superpowers/plans/2026-11-15-an-ability-is-given-with-its-value.md`.
- **Base:** `001bdad`. **Branch:** `feat/an-ability-is-given-with-its-value`.
- **Owner's rulings taken in:**
  - 2026-11-15: a give of an ability whose resource the sheet lacks carries
    that resource's value; 0 is a value; a missing value is refused. A sheet
    holds only what can be used: a take removes each resource the taken
    ability spends that no ability left on the sheet spends, and a shared
    resource stays while some ability on the sheet spends it. The same rule
    where a sheet enters, in this ticket. A give carrying a value for a
    resource the sheet already holds is refused. Attributes are the
    ruleset's and are not checked. The rule reaches only a resource some
    ruleset ability spends as its `usage.limited` cost; `hp` and `drink` are
    never removed by a take (read to the owner, not objected to).
  - 2026-11-15: the alpha campaign keeps working once this lands (D13).
  - 2026-11-14: OD-1 of the give-and-take plan: a sheet's list changes by
    `AbilityGiven` and `AbilityTaken`, each naming one actor and one
    ability. OD-1 below keeps that shape.
  - 2026-11-08: no automatic default values; only clearly wrong data is
    refused; the DM owns the fit (principle 26).
  - 2026-10-21: a near-theoretical edge with a small consequence gets no
    code, no test and no debt entry (OD-6).
  - 2026-10-15: a table has one DM in practice (OD-6).
  - 2026-09-04: nobody uses the product; no migration. Principle 5 still
    binds every log already written: none holds the new events.
  - Standing: no counts or measured figures in anything written down;
    push and merge on the owner's word.
- **Signed off:** by the owner, 2026-11-15: every recommendation of OD-1
  to OD-6.
- **Ids:** VTT-760 to VTT-790, allocated at Task 0 (under
  "Requirements"); the `R` labels are this plan's.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists.
   Every symbol it names is where it says: `Usage` and `LimitedUsage` in
   `format.go`; `Resource`, `AbilityGiven` and `AbilityTaken` in
   `events.proto`; `GiveAbility` and `ClientCommand` in `commands.proto`;
   `Resolve` and its refusal `has no resource` in `resolve.go`; `Apply` and
   its `ResourceChanged` and `AbilityGiven` arms; `validateGiveAbility`;
   `validateAddActorSheet` and `sheetFault`; `loadActors`; `sheetsFit` and
   `composeServer`; `SheetKeyFault` and `SheetAbilityFault` (methods on
   `*Ruleset` in `sheet.go`); `sameIncarnation`, `statusFrames`, `classify`
   and `Resume`; `refuseValuelessCurrent`; `giveAbility`;
   `client/test/fold-parity.test.ts`; every specification under
   "Specifications this moves"; MapTool's `Token.getProperty`,
   `Token.getPropertyDefault` and `TokenProperty.getDefaultValue` (P7).
   **One misdescription:** "`client/src/state.ts` (the two arms)": that file
   holds the state's types and no event arm (P9); the arms are
   `client/src/fold.ts`'s. Not returned for: the file is still the type
   file the fold writes, and the plan routes around it (G1).
2. **"Done" is an observation. By command for item 7, by reading for the
   others.** Each item names what will be true and the test or dump that
   shows it; no named test exists yet, so none could be run to fail.
   - Item 7: a party member whose resource set changes is sent a bare
     `ActorRemoved` and a fresh `ActorAdded` at the next unrelated event, in
     sight and out of it, and again when it comes back into sight (P3). That
     is what the ticket's "read a changed set of resources as another
     actor" predicts.
   - Items 1 to 3: no event adds or removes a resource of an actor the
     campaign holds, and `validateGiveAbility` reads the ability id alone
     (reading).
   - Item 5: the entry rule refuses fixtures today's tree accepts (P1, P2).
   - Item 6 leaves the no-ruleset give to the plan, as the ticket says
     (OD-2). Items 4 and 8 to 11 name tests and gates that cannot hold today
     (reading).
3. **Each rule is breakable. By reading.** Each rule line maps to rows R1 to
   R26 below, each with what turns it red. The third rule ("a take removes
   each resource … and no other") holds with a ruleset served only; the
   fifth rule says so, and R7 and R8 carry it.
4. **The scope matches the claim. By command, then reading.** The callers
   (P4): `sameIncarnation` from `recall` and `Resume`; `statusFrames` from
   `correct` and `Resume`; `beliefOf` from `recall` and `Resume`;
   `kindMovesList` from `recall`; `validateGiveAbility` from `validateForm`;
   `sheetFault` from `validateAddActorSheet`; `sheetsFit` from
   `composeServer`; `loadActors` from `adventure.Load`, which
   `loadAdventuresDir` calls for both `vtt serve` and `vtt mcp`;
   `refuseValuelessCurrent` from `commandFromArgs`; `describe` from
   `renderFeed` and `renderTicker`; `giveAbility` from the console alone;
   `Envelope_AbilityGiven{` and `Envelope_AbilityTaken{` are written by
   `ToEvent`, `statusFrames` and `internal/eventgen`. The work reaches files
   the ticket does not name: G2.
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-025's consequence "No command changes which keys a sheet holds"
     is overturned; the ticket names SPEC-025 and its Problem states the
     change.
   - SPEC-013's "A `take_ability` has no validator" and "A give and a take
     … become one `AbilityGiven` or `AbilityTaken`" are overturned; the
     ticket names SPEC-013 and `server.go`.
   - SPEC-016 defines `sameIncarnation` over the fields only `ActorAdded`
     writes, resource names and maxima among them, and VTT-739 and VTT-740
     name resources among what re-introduces an actor. A give and a take
     now write them, so by SPEC-016's own definition they leave
     `sameIncarnation` (D5), and both rows become false as worded. The
     ticket names SPEC-016 but not the rows: G3.
   - The ticket says VTT-715 is overturned. Under OD-1's recommendation it
     stays true (the ability events still change only the list) and new
     rows hold the resource events; under OD-1's B it is withdrawn. VTT-714
     and VTT-745 become false as worded under the recommendations and are
     not named: G3.
   - OD-1 of 2026-11-14 ("each naming one actor and one ability") holds
     under OD-1's recommendation; B below would amend it, and says so.
6. **The records the work moves are named. By command, then reading.**
   The section is present; every path under it resolves; none is `New:`.
   By reading, the list is complete for the recommendations: SPEC-012
   (whose handlers do not change) and SPEC-015 (whose `watchable` carries
   no resource) do not move; SPEC-021 moves at most in the reopen tests it
   names, if Task 2 adds one there.

**Split: not offered.** The owner ruled on 2026-11-15 that where a sheet
enters is in this ticket. Of the rest, nothing stands alone: the projection
must land before any command writes the new events (or a give withdraws
the actor from every log that did not see it, P3); the command needs the
ruleset's reading of what an ability spends; the console and the MCP tool
are the only ways to send the value, and without them every give of a
limited ability to a sheet lacking its resource is refused with no way to
satisfy it. The tasks are ordered so each commit passes its hooks and no
intermediate commit gives the table a give that breaks a log.

## The gaps this plan inherits

- **G1. `client/src/state.ts` holds no arm.** The ticket's "the two arms"
  are `client/src/fold.ts`'s; `state.ts`'s `Resource` type already holds a
  current and a max, so under OD-1's recommendation it does not change.
- **G2. Files the work reaches that the ticket does not name.**
  - `internal/eventgen/model.go`: it draws the histories
    `TestEveryCharacterLogFoldsAtEveryPrefix` folds, and draws
    `AbilityGiven` and `AbilityTaken` today. Without the new events in it,
    the property that catches a projection sending a frame a log's fold
    refuses never sees them (Task 1).
  - `client/src/view/spectator.ts` (`describe`): its switch has no default
    and `strict` is on, so `client:typecheck` refuses a new payload it does
    not word (P8, OD-4, Task 1).
  - The oneof and field walkers: `TestEveryEnvelopePayloadArmHasAnExplicitRuling`
    and `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`
    (`internal/perceive/projection_internal_test.go`), and
    `contract/roundtrip_test.go` with fixtures for the new envelopes and
    the command's new field.
  - `tools/toolgen/main.go`'s `add_actor` docs (`resources`, `abilityIds`):
    principle 34 and SPEC-025's principle 34 bullet say the tool states what
    it refuses in a sheet, so the entry refusal is stated there (R24). The
    ticket's item 9 names `give_ability` and `take_ability` only.
  - `rulesets/dnd45e-minimal/guide.md`: "it only exists on an actor whose
    `add_actor` call included it in `resources`" is false once a give can
    add `flurry_uses` (Task 3).
  - The test files whose fixtures the entry rule refuses (P1; "The existing
    tests and data that change").
  - `internal/gateway`: the give's and the take's handlers, beside
    `handleUseAbility` (D3).
- **G3. Rows the ticket does not name.** VTT-714, VTT-739, VTT-740 and
  VTT-745 become false as worded; VTT-715 does not under OD-1's
  recommendation ("Requirements" below).
- **G4. A concurrency edge the ticket does not raise.** The gateway reads a
  snapshot and appends after; OD-6.
- **G5. The value's bounds.** The ticket does not say whether a give may
  carry a current over its max. D10 keeps `add_actor`'s answer.
- **G6. The alpha campaign** (the ticket's first "could not be
  established"): resolved before this plan, outside the repository (D13).

## Measurements this plan stands on

- **P1. The entry rule, measured.** On 2026-11-15, in a copy of the tree at
  `001bdad` in the session's scratchpad, a probe `SheetSpendFault` was wired
  into `sheetFault`, `loadActors` and `sheetsFit` after `SheetAbilityFault`,
  refusing a sheet naming a limited ability without the resource it spends,
  or holding a resource some ruleset ability spends that no ability the
  sheet names spends, and logging each refusal with the test that reached
  it. Every Go package was run (tier 1 alone, then `internal/gateway` and
  `internal/mcp`, then `cmd/vtt`, each `-p 1` as the Taskfile runs them).
  The observation, dated: the Go test files below hold fixtures the rule
  refuses; the tests named below fail, except the conformance mutation
  cases and the `qa_campaign_fit` tests named as refused yet passing,
  which expected some refusal. No TypeScript test
  is refused: no `bun test` reaches a server, and the Playwright specs add
  actors with no resources and load `goblin-ambush`, which fits. The repo's
  adventures, scenarios and guide statblocks fit (P6). The files:
  - `internal/gateway/qa_add_actor_abilities_test.go`
    (`TestQAAbAnAcceptedAddActorAppendsItsAbilityIdsInTheirOrder`, both
    roles; `TestQAAbARepeatedHeldIdIsAcceptedAndAppendedAsSent`):
    `qaAbPC` names `chair-swing` without `stamina`.
  - `internal/gateway/qa_ruleset_bound_test.go`
    (`TestQARuleAThresholdRefusalOnTheWireNamesTheResourceNotTheExpression`:
    the actor holds `qa_pool`, which `qa-spend` spends and the sheet does
    not name; `TestQARuleAUseRefusedForAResourceNameAtTheBoundIsAnsweredOnAnOpenConnection`:
    the actor names `qa-spend` without its resource, on purpose, to reach
    `Resolve`'s usage refusal, D11).
  - `internal/adventure/conformance/empty_abilities_test.go`
    (`TestAnActorNamingAnEmptyListMatchesAGoldenWithoutTheKey`: `hollis`'s
    list emptied, `stamina` kept).
  - `internal/adventure/conformance/qa_data_abilities_test.go`
    (`TestQADataConformancePinsAbilityIDs`, the "list removed" and "list
    emptied" cases of both adventures: refused by the new rule before the
    golden is compared, so they pass for another reason).
  - `cmd/vtt/campaign_abilities_fit_test.go`
    (`TestACampaignWhoseActorsNameAbilitiesTheRulesetDoesNotHoldIsNotServed`,
    `TestACampaignWhoseActorsNameHeldAbilitiesIsServed`:
    `abilityMisfitCampaign` and `writeFitCampaign` name `chair-swing`
    without `stamina`).
  - `cmd/vtt/qa_campaign_abilities_fit_test.go` (the
    `TestQAAbServe…` tests and
    `TestQAAbTheMCPToolAppendsAnActorNamingItsAbilityIdsInTheirOrder`:
    `qaAbSheet` gives `stamina` to sheets that may not name `chair-swing`).
  - `cmd/vtt/qa_campaign_fit_test.go`
    (`TestQAFitACampaignWhoseSheetsFitIsServed` and
    `TestQAFitEveryMisfitActorIsNamedWithItsFirstKey` fail;
    `TestQAFitARefusedCampaignCanBeOpenedAgain` and
    `TestQAFitMisfitActorsAreNamedInByteOrderOfTheirIds` are refused and
    pass: `qaFitSheet` holds `stamina` and names no ability).
  - `cmd/vtt/qa_use_unnamed_test.go` (the `TestQAUse…` tests over
    `qaUseMCPSeed`: `qaUseMCPActor` gives every actor `stamina`, whatever it
    names).
- **P2. The baseline.** On `001bdad` in the worktree, `go test -run` over
  exactly the tests P1 saw fail passes every one (`-v` shows each
  `--- PASS`). `git status` afterwards shows only the ticket.
- **P3. Today's projection, probed.** A test in the scratch copy's
  `internal/perceive`, on `newSheetTable` with `sheetAlly(t, true, true)`:
  the door opens so `hero`'s log holds `ally` whole; the world's `ally` is
  given a resource by assignment, as no event can yet; then a
  `NarrationAdded`. In sight, the log is sent `TokenHidden`, `ActorRemoved`
  and `ActorAdded` of `ally`; with the door closed first, `ActorRemoved` and
  an unseen `ActorAdded`, and again both when the door reopens.
- **P4. The callers** in check 4, by `git grep -n -F` over `'*.go'
  ':!*_test.go' ':!contract/gen' ':!contract-spike'` and over `client/src`.
- **P5. No served ruleset in the repository has two abilities spending one
  resource.** `git grep -l '"limited"'` under `rulesets/` finds
  `hunters-flurry` (`flurry_uses`) and `chair-swing` (`stamina`) alone. The
  shared-resource case of item 3 needs a ruleset written for its test.
- **P6. The data fits.** By reading every actor file under `adventures/`
  and every `add_actor` in `scenarios/*.json` with Python: each sheet naming
  `hunters-flurry` holds `flurry_uses`, each naming `chair-swing` holds
  `stamina`, and no other holds either. P1 saw no refusal from them, the
  guide checks or `internal/harness`.
- **P7. MapTool** at RPTool `maptool` `f4b7fef6c`: `Token.getProperty`
  returns what the token's map holds, `null` included;
  `Token.getPropertyDefault` and `TokenProperty.getDefaultValue` supply a
  default; `Token.Update` holds `setProperty` and `resetProperty`, sent
  through `ServerCommand.updateTokenProperty`, one property at a time.
- **P8. `describe`'s switch** in `client/src/view/spectator.ts` ends at
  `case undefined` with no default, its return type is `string`, and
  `client/tsconfig.json` sets `strict`.
- **P9.** `grep -c 'case "' client/src/state.ts` prints 0.
- **P10.** `Resolve` refuses a limited use whose caster lacks the spent
  resource (`has no resource … required by ability …'s usage cost`) and,
  separately, an outcome on a target lacking the resource it changes (`has
  no resource … required by ability`), by reading `resolve.go`.
- **P11.** `.semgrep/vocabulary.yml` bans, under `internal/` and `cmd/`,
  `hp`, `hit points`, `action point`, `fortitude` and the other words of its
  regex, tests included.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves: a test whose assertion changes
  changes because a rule here changed, and the task says which; a fixture
  is fitted to the rule, never the rule to the fixture); 3 (additive:
  `GiveAbility` takes field 3, `Envelope.payload` takes 38 and 39, never the
  gap at 17; `check:breaking` reports, it does not enforce); 4
  (`engine.Apply` is the one fold: the new arms are in it, and the
  projection's correction frames fold through it); 5 (the rules helpers
  speak of abilities, resources and costs; no game word in Go, tests
  included, P11); 8 and 10 (names, never line numbers; a comment is a
  warning or a pointer, and one this change makes false is rewritten:
  `sameIncarnation`'s, `kindMovesList`'s, `actorBelief`'s and
  `statusFrames`' docs, `validateGiveAbility`'s, the `give_ability` and
  `take_ability` texts); 9 (answered below).
- **Principle 4.** The fold judges each new event against the live state:
  it refuses adding a resource the actor holds and removing one it does not.
  The gateway judges what only it can, against the served ruleset and the
  snapshot.
- **Principle 6.** The console's value fields are an aid; the server's
  refusal is what stands.
- **Principle 34.** Every tool description and every guide sentence the
  agent reads is true at each commit that changes what it describes.
- **SPEC-025's** "the gateway does not read a `current`" stays true: the
  give copies the carried `Resource` into its event and reads no field of
  it.
- **ADR-009.** A golden is derived by hand first: `state.json` is edited by
  hand before `stream.json` is recorded (`scenarios/goldens/README.md`).
- **The project's mechanical gates**, as the give-and-take plan lists them
  under "Constraints that bind every task": a client change is committed
  with its `cmd/vtt/webdist`, rebuilt with `task build:client`; client
  tests are measured with `env -u CLAUDECODE`; `tools/check-new-prose` and
  `python3 tools/check-comments.py` run before each commit; stage and commit
  in separate calls.

## Rule 9: how MapTool does this

The ticket records MapTool's answer, and P7 confirms it:
- A token's property may hold nothing, a default fills in, and a macro on
  the token reads whatever is there; nothing ties a macro to a property. A
  property is set or reset one at a time (`setProperty`, `resetProperty`),
  and every client is sent the update.
- **Borrowed:** the granularity: a resource arrives and leaves by its own
  update, one resource at a time (OD-1).
- **Not borrowed:** the default (no automatic value, the owner's 2026-11-08
  ruling); the absence of a tie between an ability and what it spends (the
  owner's 2026-11-15 ruling: a sheet holds only what can be used); every
  client sent every update (a log is sent a resource change only while its
  eyes see the party member, principles 16 and 17).

## Decisions

1. **D1. The contract, under OD-1 and OD-3's recommendations.**
   - `GiveAbility` gains `map<string, Resource> resources = 3`: the
     resources the give brings, keyed as `Actor.resources` is.
   - New `ResourceAdded { string actor_id = 1; string resource = 2;
     Resource value = 3; }` and `ResourceRemoved { string actor_id = 1;
     string resource = 2; }`, arms `resource_added = 38` and
     `resource_removed = 39` of `Envelope.payload`. `AbilityGiven`,
     `AbilityTaken` and `TakeAbility` do not change.
   - Each message's comment is a pointer to SPEC-025, the arms' to SPEC-007.
   - **Forced by** item 7: a correction must bring a log's resource set to
     the world's without an ability frame, because a set can change while an
     ability list ends where it began (a give carrying no value, then the
     take that removes the resource), and a frame naming an ability the log
     never saw given would borrow an identifier. Rule 3 forces the names and
     numbers. The map is OD-3's.
   - Task 1 adds the events, Task 3 the command's field, so each commit's
     walkers pass with the code that answers them.
2. **D2. The fold.** `engine.Apply` and `client/src/fold.ts` each gain an
   arm per event, validating before they change anything:
   - `ResourceAdded`: refused when the resource name is empty or over
     `maxIDBytes` (SPEC-018's first table, as `ActorAdded`'s keys), when it
     carries no value, when the actor is unknown, and when the actor holds
     the resource; otherwise the actor's resources become a fresh map
     holding the resource at the event's current and max.
   - `ResourceRemoved`: refused when the actor is unknown or does not hold
     the resource; otherwise a fresh map without it.
   - Neither changes anything else of the actor or the state. The value's
     numbers are not checked, as `ActorAdded`'s are not (D10).
   - **Forced by** principle 4 and rule 4; the order of refusals follows the
     `AbilityGiven` arm's (form, then actor, then state). A fresh map
     follows the ability arms' "a caller may hold the old one".
3. **D3. The command path.** A `give_ability` and a `take_ability` are
   dispatched before `ToEvent`, beside `use_ability`, to new handlers that
   take `ToEvent`'s envelope for the ability event, add the resource events
   after it, stamp every envelope as `handleUseAbility` does, and append the
   whole through `campaign.AppendBatch`, so a refusal anywhere appends
   nothing.
   - `validateGiveAbility(cmd, rs)` keeps the refusals that read the ruleset
     alone, in this order: with none served, a give carrying any
     `resources` (OD-2); the ability id the ruleset does not hold (VTT-718);
     a carried key the ability does not spend, any key for an at-will
     ability.
   - The give's handler then reads the snapshot's actor, when it holds one:
     a carried key the sheet already holds is refused; a limited ability
     whose resource the sheet does not hold, carrying nothing, is refused,
     naming the actor, the ability and the resource. An actor the snapshot
     does not hold is left to the fold, which refuses the `AbilityGiven`
     (VTT-717).
   - The take's handler, with a ruleset served, appends a `ResourceRemoved`
     of the resource the taken ability spends when the snapshot's sheet
     holds it and no id the sheet names other than the taken one spends it;
     with none served, the `AbilityTaken` alone.
   - Each refusal keeps the shape `gateway: give_ability: actor "<actor>":
     <field>: <reason>`, as `validateAddActorSheet`'s does, and reaches only
     the DM and the agent.
   - **Forced by** SPEC-013's split: a validator refuses a form against the
     ruleset; a reading of the snapshot belongs to the handler, as the
     sheet's refusal of a use does in `handleUseAbility`.
4. **D4. The ruleset's reading of what an ability spends.** In
   `internal/rules/sheet.go`, beside `SheetKeyFault` and
   `SheetAbilityFault`:
   - `Ruleset.Spends(abilityID)` answers the resource a held limited ability
     spends, or none.
   - `Ruleset.SheetSpendFault(abilityIDs, resources)` answers, as a
     `SheetFault`, the first id in byte order naming a limited ability
     whose resource is not among `resources` (field `ability_ids`), and
     otherwise the first resource in byte order that some ability of the
     ruleset spends and no named ability spends (field `resources`); its
     reason names the key and the other half. Abilities before resources is
     a preference: an unusable ability fails at the table, an orphan does
     not.
   - **Forced by** one reading for the give, the take and every entry
     point, so they cannot disagree on what an ability spends.
5. **D5. The projection.**
   - `classify` forwards a `ResourceAdded` and a `ResourceRemoved` as it
     forwards a `ResourceChanged`: a party member's, to a log whose eyes saw
     it before the event (`sheetActor` learns both).
   - `sameIncarnation` compares no resource names or maxima, which a give
     and a take now write; it keeps name, module id, module data and
     attributes.
   - `kindMovesList` also holds when the held actor or `shownAs` of the
     world's holds a resource, so a kind change with resources involved is
     still withdrawn and introduced afresh (VTT-468, VTT-481), and no
     correction sends a creature's resource frame.
   - `actorBelief` keeps each resource's max beside its current.
     `statusFrames` walks the union of the believed and the present names
     in byte order: a `ResourceRemoved` for a name believed and not present
     or present at another max, then a `ResourceAdded` at the world's
     current and max for a name present and not believed or at another max,
     and otherwise a `ResourceChanged` when the current differs, as today,
     all before the condition frames. It answers false for the kind case
     alone, so `correct` no longer withdraws an actor for its resources.
   - `Resume` does not change: a fold whose party member in sight differs
     in resources gets frames from `statusFrames` and starts afresh.
   - **Forced by** item 7 and P3, and by SPEC-016's own definition of
     `sameIncarnation`, which the give-and-take plan's D5 applied to
     ability ids the same way.
6. **D6. Where a sheet enters.** `SheetSpendFault` runs, with a ruleset
   only, after `SheetAbilityFault` in `sheetFault` (`gateway: add_actor:
   actor "<id>": <field>: <reason>`), in `loadActors` (`adventure: <path>:
   field "<field>": <reason>`) and in `sheetsFit` (each actor's first fault,
   in byte order of ids). With none served nothing is read.
   - **Forced by** the owner's 2026-11-15 ruling and principle 26; the place
     after the ability ids, because the rule reads compiled abilities.
7. **D7. The agent's tools.**
   - `give_ability` says when `resources` is required (the ability spends a
     resource the sheet does not hold), when it is refused (a resource the
     sheet holds, one the ability does not spend, any with no ruleset
     served), that 0 is a value and the max may be left out for none, and
     what else changes: that resource alone.
   - `take_ability` says which resource leaves with the ability, and that no
     other does.
   - `add_actor`'s `resources` and `abilityIds` docs state the entry refusal
     (R24).
   - `refuseValuelessCurrent` names a give's actor: the walker reads
     `actor_id` from `vtt.v1.GiveAbility` as it does from `vtt.v1.Actor`,
     keyed by message name, never by tool.
   - **Forced by** principle 34 and item 9.
8. **D8. The console, under OD-5's recommendation.** In an actor's row,
   choosing an ability whose `usage` spends a resource the actor's sheet does
   not hold shows a current field and a max field, named as the window names
   the resource; Give with the current empty or not an integer says so and
   sends nothing; a max left empty sends none. Any other choice shows no
   value field and sends none. `giveAbility` takes the resources it sends.
   - **Forced by** item 10; the fields' drafts survive a repaint as every
     console field's do.
9. **D9. The Story and the message, under OD-4's recommendation.**
   `describe` words a `resourceAdded` "<actor> now has <resource> at
   <current>" and a `resourceRemoved` "<actor> no longer has <resource>",
   the resource named by `Namer.resource`. `sheetNews` does not change.
   - **Forced by** P8 for the Story; the words are OD-4's.
10. **D10. The value is checked as `add_actor`'s is.** No gateway check of
    a current against a max; the MCP tool refuses a current left out, the
    console an empty one.
    - **Forced by** SPEC-025's "the gateway does not read a `current`" and
      one answer for every command that writes a resource. Refusing a current
      over a max above 0 everywhere is a ticket of its own if the owner
      wants it.
11. **D11. `Resolve`'s usage refusal stays.** No served command path reaches
    it once the entry rule and the give land, but a log folded before them,
    or OD-6's edge, can, and `Resolve` decides from the snapshot it is
    handed. `TestQARuleAUseRefusedForAResourceNameAtTheBoundIsAnsweredOnAnOpenConnection`
    moves to the outcome refusal P10 names, which still reaches the wire.
    - **Forced by** principle 4: the rules engine refuses what it cannot
      resolve, whatever the gateway let through.
12. **D12. The order within a batch.** The ability event first, then each
    resource event in byte order of its name. A preference: the Story reads
    the act, then what came or went with it; both folds accept either.
13. **D13. The alpha campaign: a precondition met.** Reported by the
    coordinator on 2026-11-15, outside the repository and carrying no
    ticket or commit: `tobbe.json` names `hunters-flurry`, `crossbow-shot`
    and `rally`, `lera.json` `longsword-strike`, `staggering-blow` and
    `rally` (the owner confirmed the sets); `~/go/bin/vtt` was rebuilt from
    `001bdad`, the campaign served with the adventure loaded at boot, the
    six abilities given over MCP, the state dump confirmed (`tobbe` holds
    `flurry_uses` and `hp`, `lera` `hp`), and the server stopped. Under the
    entry rule it fits.
    - **The order constraint:** the alpha campaign's actors had to name
      their abilities on a build of current main before this branch's build
      serves it, since this branch's `adventure.Load` and `sheetsFit` refuse
      a sheet holding `flurry_uses` while naming no ability that spends it.
      Met; nothing in the alpha directory is touched by this work.

## Open decisions for the owner

- **OD-1. What the events carry. Recommended: A.**
  - **A: two events of their own, `ResourceAdded` and `ResourceRemoved`
    (D1), appended in one batch after the ability event.** The ability
    events stay as OD-1 of 2026-11-14 made them, and VTT-715 stays true. A
    correction can bring any resource set to the world's, so no log is ever
    withdrawn for its resources (item 7). It is MapTool's granularity (P7).
    Cost: two new payloads in both folds, the projection, the Story and the
    event generator; a give and a take become batches.
  - **B: `AbilityGiven` carries the resource and value it brings, and
    `AbilityTaken` the resources it removes.** One envelope per command, no
    batch. Cost: amends OD-1 of 2026-11-14 and withdraws VTT-715; and a
    correction can only add or remove a resource inside an ability frame,
    so a set changed while the list ended where it began has no frame to
    carry it and falls back to withdrawing the actor, which item 7 forbids.
- **OD-2. A give carrying a value with no ruleset served. Recommended: A.**
  - **A: refused, appending nothing.** Nothing can say which resource the
    ability spends; with no ruleset a take removes no resource, so a
    resource written now could never leave by a take; and with no ruleset
    no use resolves, so nothing written now could be used before a ruleset
    is served, when the give can be made with its check. A give carrying no
    value is accepted as today. VTT-745 is withdrawn and succeeded (R6a, R6b).
  - **B: accepted, writing the resource.** Matches "with no ruleset,
    nothing is checked". Cost: a wrong key stays until the actor is removed,
    and `vtt serve --ruleset` then refuses the campaign.
- **OD-3. The command's shape. Recommended: A.**
  - **A: `map<string, Resource> resources`, the shape `add_actor` uses.**
    The agent and the console write a resource one way; the MCP walker's
    map arm already refuses a missing current. Any extra key is refused
    (item 2's third case).
  - **B: `string resource` and `Resource value`.** One resource by
    construction, since a ruleset ability spends at most one. Cost: a
    second shape for the same thing.
- **OD-4. Whether the Story or the message names the resource.
  Recommended: A.**
  - **A: the Story words each new event (D9); the player's message names
    abilities only.** The Story must word every payload (P8); the message
    reads `WatchableChanged`, which carries ability ids and no resources,
    and the player's log, forwarded the resource event, shows it in the
    Story.
  - **B: the message also names the resource that came or went.** Cost:
    `WatchableChanged` or the window must carry resources it does not today
    (SPEC-015, VTT-731), for a fact the Story already shows.
- **OD-5. How the console asks. Recommended: A.**
  - **A: a current, required, and a max, optional, shown only when the
    chosen ability needs them (D8).** As the paste and the MCP tool take a
    resource: a max left out is none (VTT-679).
  - **B: both required.** Cost: a resource with no maximum cannot be given
    from the console though the tool and the paste allow it.
- **OD-6. Two writers at once (G4). Recommended: A.**
  - **A: no code; the edge is named.** The fold refuses a second add of one
    resource and a removal of one already gone, so two gives of one resource
    cannot both land. What it cannot see: two takes of abilities sharing a
    resource, each seeing the other still on the sheet, so neither removes
    it; or a give carrying no value while the take that removes its
    resource lands. Both need two authoring connections within one
    read-to-append window, and a table has one DM in practice (2026-10-15);
    the consequence is one sheet out of fit until the DM takes and gives
    again, and the next `vtt serve --ruleset` names the actor. Near-
    theoretical with a small, repairable consequence: no code, no test, no
    debt entry (2026-10-21).
  - **B: one gateway lock held across a give's and a take's snapshot and
    append.** Closes it. Cost: a lock and a scheduling test that must
    isolate the mechanism rather than count runs.

## Requirements

Sorted and allocated at Task 0, 2026-11-15, after the owner's sign-off. The
plan's tasks keep the `R` labels; the register carries the ids and each
row's exact words. Every row is new work and reads `**OPEN — no test yet**`
until its task cites its tests.

The sort split R6 into R6a (VTT-765, a value refused with no ruleset) and
R6b (VTT-766, a give with none accepted), R9 into R9 (VTT-769, the add) and
R9b (VTT-770, its refusals), R10 into R10 (VTT-771) and R10b (VTT-772), and
R26 into R26a (VTT-788, the fields) and R26b (VTT-789, the empty current).
It refused R8's first half, that no take removes a resource no ability of
the ruleset spends, as a rule of its own: R7's "and nothing else" holds it,
and a test of `hp` or `drink` cites R7. R8 (VTT-768) is the no-ruleset take
alone. R5 (VTT-764) is the ruleset-served half of VTT-713's successor; R6b
is the other. R27 (VTT-790) is the Story's row under OD-4.

The ids: R1 VTT-760, R2 VTT-761, R3 VTT-762, R4 VTT-763, R5 VTT-764,
R6a VTT-765, R6b VTT-766, R7 VTT-767, R8 VTT-768, R9 VTT-769, R9b VTT-770,
R10 VTT-771, R10b VTT-772, R11 VTT-773, R12 VTT-774, R13 VTT-775,
R14 VTT-776, R15 VTT-777, R16 VTT-778, R17 VTT-779, R18 VTT-780,
R19 VTT-781, R20 VTT-782, R21 VTT-783, R22 VTT-784, R23 VTT-785,
R24 VTT-786, R25 VTT-787, R26a VTT-788, R26b VTT-789, R27 VTT-790.

The table below is the rules as proposed; the register's words are the
ones that bind.

| R | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|
| R1 | 1 | With a ruleset served, a `give_ability` from the DM or the agent of a limited ability whose resource the sheet does not hold, carrying that resource under `resources`, appends one `AbilityGiven` and then one `ResourceAdded` of it at the carried current and max, after which the sheet names the ability and holds the resource at that value, 0 included | nothing appended, or the resource missing or at another value |
| R2 | 2 | With a ruleset served, a `give_ability` of a limited ability whose resource the sheet does not hold, carrying no `resources`, is refused by an error naming the actor, the ability and the resource, and appends nothing | appended, or a resource unnamed |
| R3 | 2 | With a ruleset served, a `give_ability` carrying a resource the sheet already holds is refused, naming the actor and the resource, appends nothing, and leaves the resource at its value | appended, or the value moved |
| R4 | 2 | With a ruleset served, a `give_ability` carrying a key the ability does not spend, any key for an at-will ability, is refused, naming the actor, the ability and the key, and appends nothing | appended |
| R5 | 1, 2 | A `give_ability` from the DM or the agent naming an actor the campaign holds and an ability its sheet does not name, one the served ruleset holds when one is served, carrying no `resources`, of an at-will ability or one whose resource the sheet holds, appends one `AbilityGiven` and nothing else (succeeds VTT-713) | a second event, or the resource changed |
| R6 | 6, OD-2 | With no ruleset served, a `give_ability` carrying `resources` is refused and appends nothing, and one carrying none of any non-empty id the sheet does not name appends one `AbilityGiven` (succeeds VTT-745) | a resource written, or a valid give refused |
| R7 | 3 | With a ruleset served, a `take_ability` from the DM or the agent of an ability the sheet names appends one `AbilityTaken` and, when the ability spends a resource the sheet holds that no other id on the sheet spends, one `ResourceRemoved` of it, and nothing else; after it the sheet names no occurrence of the id and keeps its other ids in their order (succeeds VTT-714) | the resource kept, a shared one removed, or another resource removed |
| R8 | 3, 6 | No `take_ability` removes a resource no ability of the served ruleset spends, and with no ruleset served a `take_ability` appends one `AbilityTaken` alone | a resource removed |
| R9 | D2 | Both folds add, at a `ResourceAdded`, the resource to the actor at the event's current and max, and refuse one naming an actor they do not hold, a resource the actor holds, no value, or a resource name empty or over 128 bytes | accepted |
| R10 | D2 | Both folds remove, at a `ResourceRemoved`, the resource from the actor, and refuse one naming an actor they do not hold or a resource it does not hold | accepted |
| R11 | 4 | Both folds change nothing at a `ResourceAdded` or a `ResourceRemoved` but the named resource of the named actor | another field differs |
| R12 | 8 | The client's fold of a `ResourceAdded` and a `ResourceRemoved` gives the state `vtt state dump` prints for the same log, refusals included | a dump differs |
| R13 | 5 | With a ruleset served, `add_actor` refuses, after its ability-id refusals, an actor naming a limited ability whose resource its sheet does not hold, or holding a resource some ruleset ability spends while no ability it names spends it, naming the actor, the field and the key, ability ids before resources, each in byte order, and appends nothing | appended |
| R14 | 5 | `adventure.Load` refuses, after its ability-id refusals, an actor file R13's rule refuses, naming the file, the field and the key | loaded |
| R15 | 5 | `vtt serve` with a ruleset refuses to start a campaign any of whose actors R13's rule refuses, naming each such actor and its first fault | served |
| R16 | 6 | With no ruleset served, neither `add_actor` nor `vtt serve` refuses an actor for the abilities it names against the resources it holds | refused |
| R17 | 7 | A character log is forwarded a `ResourceAdded` or a `ResourceRemoved` of a party member exactly when its eyes saw it before the event and its projection does not start afresh at that event, and is sent neither, forwarded or as a correction, of an actor that is not a party member | sent out of sight, or a creature's |
| R18 | 7 | A character log holding a party member whose resource names or maxima differ from the world's is sent, at the first event its eyes see it, a `ResourceRemoved` per resource it holds that the sheet does not, or at another max, and a `ResourceAdded` at the world's value per resource the sheet holds that it does not, or at another max, in byte order of the names, before the correction's condition frames | the set left stale, or a withdrawal |
| R19 | 7 | No character log is sent an `ActorRemoved` or an `ActorAdded` of an actor because its resource names or maxima differ from the world's while its kind does not change | a withdrawal or an introduction sent (P3) |
| R20 | 7 | `perceive.Resume` resumes a log whose fold holds each party member in sight at the resource names, maxima and currents of the world's sheet, and starts afresh from one whose fold differs in any | resumes a lagging log, or starts afresh from a current one |
| R21 | 7, G3 | An actor the world adds again after a removal is introduced afresh to a viewer that sees it when the viewer no longer holds the id, or holds one that differs from what it may be sent of the new actor, and is corrected otherwise; for a party member that is its name, module id, attributes or module data, and not its resource names, maxima or ability ids, which are corrected; for any other actor, its module id (succeeds VTT-739) | re-introduced for its resources |
| R22 | 7, G3 | A character log is sent a bare `ActorRemoved` of an actor it holds as a party member with no attributes, resources or module data, whatever ability ids it holds, while the world's actor has attributes or module data, only at the first event since the world last added it at which the log's eyes see it, or when the world adds its id again with another name or module id; a world's actor whose only difference is its resources is corrected (succeeds VTT-740) | withdrawn for resources alone |
| R23 | 9 | The MCP tools `give_ability` and `take_ability` say they are the DM's and the agent's, what each refuses, when `give_ability`'s `resources` is required and when refused, which resource leaves with a take, and that nothing else of the actor changes (succeeds VTT-728) | a sentence missing or false |
| R24 | 5, G2 | The `add_actor` tool states that, with a ruleset served, it refuses a sheet naming a limited ability without the resource it spends, or holding a resource some ruleset ability spends that no ability it names spends | the sentence missing |
| R25 | 9 | The MCP tool `give_ability` refuses a resource whose current is null or left out, naming the actor and the key, before any command is sent; a current of 0 passes | sent, or `actor ""` |
| R26 | 10 | The DM console asks for a current and a max exactly when the chosen ability spends a resource the actor's sheet does not hold, sends them under `resources`, and sends nothing, saying so, when the current is empty | a field shown when not needed, or a give sent without a value |

The Story's words (D9) are R27, as VTT-736 is for the ability events.

**Rows withdrawn**, each in the task that makes it false, each with its
successor:
- **VTT-713** ("appends one `AbilityGiven`"): false once a give without a
  needed value is refused and one with a value appends two events. R1, R2,
  R5, R6b. Task 3.
- **VTT-714** ("appends one `AbilityTaken`"): false read as the whole append
  once a take removes a resource. R7, R8. Task 3.
- **VTT-728** ("nothing else about the actor changes"): R23. Task 3.
- **VTT-739** ("resource names or maxima" re-introduce): R21. Task 2.
- **VTT-740** (re-introduced while the world's actor has "attributes,
  resources or module data"): R22. Task 2.
- **VTT-745** ("a give of any non-empty id … appends one `AbilityGiven`"):
  false under OD-2's A for a give carrying `resources`. R6a, R6b. Task 3.

**Rows checked and left standing** under the recommendations: VTT-715 (the
ability events still change only the list; withdrawn under OD-1's B),
VTT-724, VTT-727, VTT-731, VTT-733, VTT-735, VTT-736, VTT-737, VTT-738,
VTT-741, VTT-742, VTT-743 (resource frames stay before the condition
frames), VTT-744, VTT-746, VTT-747, VTT-750, VTT-751, VTT-468, VTT-481
(D5's `kindMovesList`), VTT-660, VTT-680 (R24 stands beside it), VTT-683,
VTT-690 and VTT-729.

## The existing tests and data that change

By search and by P1; each task reruns its own packages first and names what
turned red.
- **P1's fixtures** (Task 4): each fitted to the rule (the resource beside
  the limited ability, or the cost resource dropped where no ability is
  meant), except where the misfit is the test's subject; there the expected
  refusal is asserted by its words.
  `TestQADataConformancePinsAbilityIDs`'s "list removed" and "list emptied"
  cases drop the cost resource with the list, so they are refused by the
  golden and not by the entry rule, and assert which.
  `TestQARuleAUseRefusedForAResourceNameAtTheBoundIsAnsweredOnAnOpenConnection`
  moves to the outcome refusal (D11).
- **The projection's tests** (Task 2): those citing VTT-739 and VTT-740
  that re-add or reveal a party member differing only in resources assert a
  correction instead; P3's shape becomes a test.
- **The give-and-take tests** that count a take's append (Task 3): a take of
  `chair-swing` or `hunters-flurry` from a sheet holding its resource now
  appends the removal too; each counts what R7 says.
- **The tool golden** (Tasks 3 and 4): `contract/testdata/expected_tools.json`,
  `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`, and the tests
  pinning the old `give_ability`, `take_ability` and `add_actor` sentences
  (`TestGiveAndTakeAbilitySayWhoMayWhatEachRefusesAndThatNothingElseChanges`,
  `TestQAGiveAndTakeToolsSayWhoMayWhatEachRefusesAndThatNothingElseChanges`
  and the `add_actor` doc tests).
- **The data** (Task 6): `scenarios/toy-brawl.json` and
  `scenarios/goldens/toy-brawl`.
- **Unchanged:** `Resolve`; `SheetKeyFault`, `SheetAbilityFault` and
  `Offers`; `introduce`, `shownWith` and `unseenAs`; `watchable` and
  `sheetNews`; the repo's adventures and scenarios' sheets (P6).

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answers OD-1 to OD-6; run the `requirements` sort over
  R1 to R26 (and the Story's row under OD-4); allocate ids with
  `requirement-id`; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and the requirements chain passes.

### Task 1: the resource events, in both folds and at the log's door (R9, R10, R11, R12's fold half, R17's forwarding; D1, D2, D9, D12)

- **Files:** `contract/vtt/v1/events.proto` (`ResourceAdded`,
  `ResourceRemoved`, `Envelope.payload` 38 and 39); what
  `task generate:contract` writes under `contract/gen`;
  `contract/roundtrip_test.go` and `contract/testdata` fixtures for both
  envelopes; `internal/engine/apply.go` (both arms) and a new
  `internal/engine` test; `client/src/fold.ts` (both arms) and a client fold
  test; `internal/perceive/projection.go` (`classify`, `sheetActor`);
  `projection_internal_test.go`'s lists; new `internal/perceive` tests:
  forwarded to the party member's own log and to a log that saw it,
  withheld from one that did not, withheld for a creature a log sees;
  `internal/eventgen/model.go` (draws both, and an illegal one it marks
  `MustFail`); `client/src/view/spectator.ts` (`describe`) and the Story's
  every-kind tests; a rebuilt `cmd/vtt/webdist`.
- **True when:** each fold adds and removes as D2 says, refuses in D2's
  order, and leaves every other field as it was, compared field by field;
  `TestEveryEnvelopePayloadArmHasAnExplicitRuling`,
  `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable` and
  `TestEveryCharacterLogFoldsAtEveryPrefix` pass; `client:typecheck`
  passes; `git diff main -- contract/vtt` adds the two messages and their
  arms and nothing else; `task check:breaking` reports nothing breaking.
- **Deliberate breaks:** accept an add of a held resource (the refusal test
  turns red in that fold); drop `now.party` from the arm (the creature test
  turns red); drop `saw` (the out-of-sight test turns red).
- **The state here:** no command writes either event; nothing at the table
  changes.

### Task 2: the projection (R18 to R22; D5; item 7)

- **Files:** `internal/perceive/projection.go` (`sameIncarnation`,
  `kindMovesList`, `actorBelief`, `beliefOf`, `statusFrames`, and the docs
  that name the fields); new `internal/perceive` tests: P3's in-sight and
  out-of-sight cases made with the events, sent no `ActorRemoved` or
  `ActorAdded` then or at later events and holding the world's resources at
  sight; a max change corrected by a removal and an addition; `Resume` over
  a fold in sight with the world's resources and over one without; a
  party member re-added with other resources corrected; one held unseen
  whose world's sheet differs only in resources corrected at first sight; a
  kind change with resources still re-introduced; the tests P1 and G3 name;
  a campaign reopen after a resource event resumes every log
  (`internal/campaign`); `docs/requirements.md` (VTT-739, VTT-740).
- **True when:** the new tests pass; every VTT-468, VTT-481, VTT-724,
  VTT-738, VTT-742, VTT-743 and VTT-744 test passes unchanged.
- **Deliberate breaks:** keep the resource comparison in `sameIncarnation`
  (the out-of-sight test turns red, as P3); drop the max from
  `actorBelief` (the max-change test turns red); drop resources from
  `kindMovesList` (the kind-change test turns red).
- **The state here:** no command writes the events; logs fed them by hand
  stay whole.

### Task 3: the command, the take and the agent (R1 to R8, R23, R25; D3, D4's `Spends`, D7, D10, D12; OD-2, OD-3; items 1 to 3, 9)

- **Files:** `internal/rules/sheet.go` (`Spends`) and its tests;
  `contract/vtt/v1/commands.proto` (`GiveAbility.resources`), regenerated;
  command fixtures and `contract/roundtrip_test.go`;
  `internal/gateway/ability_validate.go` (`validateGiveAbility`, the two
  handlers); `server.go` (`handleCommand`'s dispatch); new
  `internal/gateway` tests through the WebSocket: each of items 1 to 3's
  cases with the log's length and the state compared field by field; the
  shared resource kept, on a ruleset written for it (P5); no ruleset
  served; `internal/mcp/resource_current.go` and its test;
  `tools/toolgen/main.go` (`give_ability`, `take_ability`) and
  `main_test.go`; the regenerated tool copies and `expected_tools.json`;
  `cmd/vtt/mcp_ability_change_e2e_test.go` (through the tools, a give with a
  value then a use resolves; a take then the state holds no resource);
  `rulesets/dnd45e-minimal/guide.md` (G2's sentence); the tests G3 and
  "The existing tests and data that change" name; `docs/requirements.md`
  (VTT-713, VTT-714, VTT-728, VTT-745).
- **True when:** the new tests pass; `TestEveryClientCommandConverts` and
  the MCP walks pass; every tool text and guide sentence is true at this
  commit; `git grep -nE 'GetCurrent\(\)|\.Current([^A-Za-z0-9_]|$)' --
  internal/gateway ':!*_test.go'` still prints nothing.
- **Deliberate breaks:** drop the sheet's refusal of a missing value (R2's
  test turns red); remove a resource another named ability spends (the
  shared-resource test turns red); accept `resources` with no ruleset (R6a's
  test turns red); let the walker name `actor ""` (R25's test turns red).
- **The state here:** a give brings its resource and a take removes it; a
  sheet out of fit can still enter, as today, until Task 4. The console's
  give of a limited ability to a sheet lacking its resource is refused, in
  the server's words, until Task 5.

### Task 4: where a sheet enters (R13 to R16, R24; D4's `SheetSpendFault`, D6, D11)

- **Files:** `internal/rules/sheet.go` (`SheetSpendFault`) and its tests,
  the shared-resource case among them; `internal/gateway/add_actor_validate.go`
  (`sheetFault`) and tests through the WebSocket;
  `internal/adventure/load.go` (`loadActors`) and tests;
  `cmd/vtt/serve_compose.go` (`sheetsFit`) and tests; P1's files;
  `tools/toolgen/main.go` (`add_actor`'s docs) and `main_test.go`; the
  regenerated tool copies and `expected_tools.json`.
- **True when:** each entry point refuses each half of the rule, names the
  actor or file, the field and the key, and refuses nothing for it with no
  ruleset; `go test ./internal/... ./cmd/...` passes; the repo's adventures
  load, its scenarios seed and serve (P6); `add_actor`'s text is true.
- **Deliberate breaks:** skip the resources half (the orphan test turns
  red); read the ability's resource from the wrong field (the limited test
  turns red); run the rule with no ruleset (R16's test turns red).
- **The state here:** a sheet out of fit cannot enter, and no command makes
  one.

### Task 5: the console (R26; D8; OD-5; item 10)

- **Files:** `client/src/commands.ts` (`giveAbility`) and its test;
  `client/src/view/dm.ts` (the Abilities group); a console test driving a
  give needing a value, one needing none, an at-will one, and an empty
  current; a rebuilt `cmd/vtt/webdist`.
- **True when:** the console test passes; `client:typecheck` passes; every
  VTT-730 test passes.
- **Deliberate breaks:** show the fields for a resource the sheet holds (the
  no-value test turns red); send with an empty current (the empty test turns
  red).

### Task 6: the goldens (R12; item 8)

- **Files:** `scenarios/toy-brawl.json` (a take of `chair-swing`, a give of
  it back with `stamina`, and the refusals of a give without the value and
  one carrying a held resource); `scenarios/goldens/toy-brawl`: `state.json`
  edited by hand first, then `stream.json` recorded.
- **True when:** `internal/harness.TestFoldGoldenCorpus`,
  `cmd/vtt.TestScenarioGoldenStreamsHaveNotDrifted`,
  `client/test/fold-parity.test.ts` and the corpus half of
  `TestEveryCharacterLogFoldsAtEveryPrefix` pass.
- **Deliberate break:** let the client fold keep the old max at a
  `ResourceAdded` of a resource just removed (fold parity turns red).

### Task 7: the gate

- **Do:** `python3 tools/check-comments.py main` and `--write-ledger` once,
  last, if a ceiling can come down; check free space and the load; run
  `task check` whole on the last code commit, detached under `/bin/sh`,
  output to a file, requiring its completion line; run `task e2e`. Tell the
  owner the client's stored mutation report is out of date from Task 1 and
  which commit it measured (SPEC-023); a full Stryker run is his to ask for.
- **True when:** `task check` and `task e2e` are green (item 11).

### Task 8: the report, then the specifications

- **The report:** `docs/reports/2026-11-15-an-ability-is-given-with-its-value.md`
  with the `implementation-report` skill, and its reading review: each
  "Done looks like" item with its observation; R1 to R26 and what turned
  each red; Rule 9; G1 to G6 and each OD's answer; Phase 4a's adjudications
  under their own heading; the gate.
- **The specifications**, with the `specification` skill, from the report:
  - **SPEC-007:** the new events and their arms; `GiveAbility.resources`;
    the give's and the take's batches; the tools' words.
  - **SPEC-013:** the give and the take dispatched to handlers; what
    `validateGiveAbility` refuses and what the handlers read of the
    snapshot; the take reads the ruleset.
  - **SPEC-016:** `classify`'s arm; `sameIncarnation` without resource
    names and maxima; `kindMovesList`; the belief's max; the correction's
    resource frames; the fallback gone for resources; what re-introduces an
    actor.
  - **SPEC-018:** the new fields, in the first table or the second.
  - **SPEC-021:** read against the tree; its pins, if Task 2's reopen test
    is cited there.
  - **SPEC-022:** the fold's arms; the Story's words; the console's value
    fields.
  - **SPEC-025:** a sheet's resources change by a give and a take;
    `Spends` and `SheetSpendFault` and their callers; the Consequence on
    what no command changes, rewritten; the value checked as `add_actor`'s.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the surfaces
as signatures and names, never the source: `go doc ./internal/engine`
(`Apply`), `go doc ./internal/perceive` (`New`, `Resume`, `Event`), `go doc
./internal/rules` (`Spends`, `SheetSpendFault`, `SheetFault`), `go doc
./internal/gateway` (`ToEvent`), the `give_ability`, `take_ability` and
`add_actor` entries in `contract/gen/tools/tools.json`, and the signatures
of `giveAbility` and `renderDMConsole`'s `DMDeps`. Tell it: never run `task
check`, `task e2e`, Stryker or gremlins; its files are new `qa_` files;
semgrep bans game words in Go tests too (P11); go-arch-lint holds the layer
imports, and `internal/perceive`'s tests import only its own package,
`contract`, `engine` and `sight`; the tree is hashed before and after it.
Known and not findings: OD-6's edge; a give's value unchecked against its
max (D10); `Resolve`'s usage refusal unreachable from a served command
(D11); a log may hold a party member's resources in another order than the
world, since a map has none; the refusals' words reach the DM and the agent
as the gateway writes them.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "never", "no other") names the search that bounds it, P4
and P5 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer; check the tool descriptions and the
dnd45e guide against what the server refuses at that commit; check that
`git diff main -- contract/vtt` adds `ResourceAdded`, `ResourceRemoved`,
their arms and `GiveAbility.resources`, and nothing else.
