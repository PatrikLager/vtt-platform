# A sheet refers to the catalogue, and the DM and the agent change what it holds — report

Ticket: `docs/superpowers/specs/2026-11-25-a-sheet-refers-to-the-catalogue-design.md`.
Plan: `docs/superpowers/plans/2026-11-25-a-sheet-refers-to-the-catalogue.md`.
Branch `feat/a-sheet-refers-to-the-catalogue` from main `13eeca0`. Commits:
`4d6c89b` (Task 0: ticket, plan, register), `38f75be` (Task 1: an item's
value refused on a sheet), `db2ab40` (Task 2: places, and a sheet section
of places), `f125a92` (Task 3: the read surface), `110614e` (Task 4: the
contract, the server's fold and what a log is forwarded), `3128371` (Task
5: the client's fold and the Events panel), `c845784` (Task 6: the
projection's corrections), `47663fa` (Task 7: where a sheet enters, by
command and at boot), `8d25667` (Task 8: an adventure's sheets),
`27cd004` (Task 9: the commands in play), `6488a28` (Task 10: the panel
and the console), `ac618a7` (Task 12: dnd45e-minimal and goblin-ambush,
and VTT-1075; the last code commit) and `9271f34` (Task 13's comment
ledger, which changes `tools/comment-ceilings.txt` alone). Task 11 was
withdrawn: it was to tell a player that their character gained or lost an
item, and the owner answered OD-2 B, that no player is told (D22); its
number stays so the tasks after it keep theirs. Every code reference below
is to `ac618a7`.

This is the second of the tickets the owner split ADR-012 step 1 item 5
into on 2026-11-24, which the 5a report
(`docs/reports/2026-11-24-a-ruleset-declares-forms-and-a-catalogue.md`) and
this one call 5a
(forms and the catalogue, `13eeca0` and before), 5b (this one) and 5c (the
actions a sheet reaches through its references, in place of
`Actor.ability_ids`). SPEC-028, new, is written from this report; SPEC-007,
SPEC-012, SPEC-013, SPEC-016, SPEC-018, SPEC-021, SPEC-022, SPEC-023,
SPEC-024, SPEC-025 and SPEC-027 move with it (D23, the plan's Task 14).
SPEC-021 is not in the ticket's list; the verifier added it (G3). SPEC-023
is in neither list: its list of the files the client's mutation hash reads
moved with Task 12, and the specification task found it.

## Done looks like

1. [x] **A ruleset declares the places on a sheet.** `manifestJSON`
   (`internal/rules/load.go`) reads `places` (`placeJSON`); `loadPlaces`,
   after `loadForms`, checks each place against the forms declared beside
   it; `checkManifestBounds` bounds a place's key and each of its states at
   `maxIDBytes` (SPEC-018), place by place, after the form keys; `loadV2`
   copies them, in declared order, to `Ruleset.Places` (`Place` in
   `internal/rules/format.go`), and `Ruleset.Place` finds one by key. A
   place is one kind (D1): a key, a name, a description, `forms`, `copies`
   and `states`.
   - Kept as written, in order, and none when absent, `null` or `[]`:
     `internal/rules/places_test.go`'s
     `TestARulesetKeepsItsPlacesInDeclaredOrder`,
     `TestAPlaceKeepsItsFormsAndStatesInTheOrderWritten`,
     `TestTheFixtureKeepsItsPlaces` and
     `TestARulesetDeclaringNoPlacesLoadsWithNone`; a place writing neither
     `copies` nor `states` holds an entry once and names no state,
     `TestAPlaceWritingNeitherCopiesNorStatesLoadsWithNoCopiesAndNoState`
     (VTT-1015).
   - Refused, naming `ruleset.json` and `places[<i>].<field>`: no key, name
     or description, `TestAPlaceIsRefusedWithoutItsKeyNameOrDescription`; a
     key an earlier place declares,
     `TestAPlaceKeyAnEarlierPlaceDeclaresIsRefused`; a key past the bound,
     `TestAPlaceKeyPastTheIDBoundIsRefused`, and one at it loads,
     `TestAPlaceKeyAtTheIDBoundLoads` (VTT-1016).
   - A form the ruleset does not declare and, beyond the item (G4), no form
     or one form twice: `TestAPlaceNamingNoFormIsRefused` and
     `TestAPlaceNamingAFormItMayNotHoldIsRefused`; a state that is empty or
     that the place names twice,
     `TestAPlaceStateThatIsEmptyOrRepeatedIsRefused`; beyond the item, a
     state past the bound, `TestAPlaceStatePastTheIDBoundIsRefused`, and one
     at it loads, `TestAPlaceStateAtTheIDBoundLoads`; the bounds read before
     a place's other faults,
     `TestTheBoundsOnAPlaceAreReadBeforeItsOtherFaults` (VTT-1017).
   - A field a place does not define is refused,
     `TestAFieldAPlaceDoesNotDefineIsRefused`, and a case variant of
     `places` is read as it, `TestACaseVariantOfPlacesIsReadAsPlaces`
     (VTT-1018, succeeding VTT-996).
   - The schema: `TestTheSchemaDescribesEveryPlaceField` (VTT-1021); the
     bounds it states, by the place-key and state rows of
     `TestTheSchemasStateTheIDBound` and
     `TestQAPFSchemasStateTheBoundsLoadEnforces` (VTT-1022, succeeding
     VTT-1009). `internal/rules/testdata/valid/ruleset.json` declares a place
     with copies and states (`shelf`) and one with neither (`hook`), both
     shown by its "Stores" section on the page "Back", so
     `TestSchemaRequiredFieldsMatchLoaderEnforcement` runs a case for each of
     a place's required fields. Its `places[0].key` case stays green with the
     key refusal dropped, since the load then fails on the sheet naming an
     undeclared place; the direct test holds that refusal.
   - Independently: `internal/rules/qa_places_test.go`.
   - On `13eeca0` the new tests do not build, since `Ruleset` has no
     `Places`. The plan's P1 measured the refusal they replace there: `json:
     unknown field "places"`, for `internal/rules/testdata/valid`,
     `rulesets/dnd45e-minimal` and `rulesets/tavern-brawl`.
2. [x] **An item's value is refused on a sheet.** `Ruleset.SheetKeyFault`
   (`internal/rules/sheet.go`) gains a third sweep, `formKeyFault`, after
   `keyFault`'s sweeps of `attributes` and then `resources`, each answering
   the first key, in byte order, the ruleset does not declare or declares as
   the other type. `formKeyFault` answers the first `attributes` key in byte
   order that a form lists, named with the first form in declared order that
   lists it, by its key (`SheetFault.Form`). The one change reaches every
   entry point the item names (the plan's P5), and conformance's
   `buildFixtureState`, which Task 1 made ask it (VTT-1066, below).
   - The rule: `internal/rules/sheet_item_value_test.go`'s
     `TestSheetKeyFaultRefusesAnAttributeACatalogueFormLists`,
     `TestSheetKeyFaultNamesTheFirstFormInDeclaredOrderThatListsTheKey` and
     `TestSheetKeyFaultReadsAFormsKeyAfterTheOtherKeyFaults` (VTT-1010).
   - `add_actor` with a ruleset served:
     `internal/gateway/add_actor_item_value_internal_test.go`'s
     `TestAddActorWithARulesetRefusesAValueACatalogueFormLists`, beside
     `TestAddActorWithARulesetAcceptsANumberNoFormLists` (VTT-1062).
   - `adventure.Load`: `internal/adventure/actor_item_value_test.go`'s
     `TestAnAdventureWhoseStatblockHoldsAValueACatalogueFormListsIsRefused`,
     beside `TestAnAdventureWhoseStatblockHoldsANumberNoFormListsLoads`
     (VTT-1063).
   - `vtt serve --ruleset`: `cmd/vtt/campaign_item_value_fit_test.go`'s
     `TestACampaignWhoseActorHoldsAValueACatalogueFormListsIsNotServed`,
     beside `TestACampaignWhoseActorsHoldOnlyNumbersNoFormListsIsServed`
     (VTT-1064).
   - The item names "the key and the actor or its file"; the rows say which
     key: the first such in byte order, after every refusal an actor met
     before. An actor that also holds an undeclared key is refused for that
     key, as it was (VTT-1062 to VTT-1065; see the deviations).
   - Beyond the item, by the owner's ruling of 2026-11-25: conformance's
     fixture actors hold only what a sheet may hold, and its smoke pass
     walks every expression of each compiled power (`buildFixtureState`,
     `smokeTest` and `unheldRead` in
     `internal/rules/conformance/conformance.go`), so a ruleset an ability
     of which reads a form-listed key from an actor fails `conformance.Run`
     on any branch: `form_listed_property_test.go`'s
     `TestRunFailsAtTheSmokePassAnAbilityReadingAPropertyAFormLists` and
     `TestRunFailsAtTheSmokePassAnAbilityReadingAListedKeyOnlyWhenItMisses`
     (VTT-1066).
   - Independently: `internal/rules/qa_item_value_test.go`,
     `internal/adventure/qa_item_value_test.go`,
     `internal/gateway/qa_add_actor_item_value_test.go` and
     `internal/rules/conformance/qa_form_listed_property_test.go`; QA also
     drove `vtt serve --ruleset` as a process, which named every misfit
     actor in byte order.
   - On `13eeca0` `SheetKeyFault` answers nil for each key dnd45e-minimal's
     forms list (the plan's P2), so the refusal tests fail there. The order
     tests and the accepting controls pass there by their nature; their
     breaks hold them (`38f75be`'s message).
3. [x] **A sheet holds references.** `vtt.v1.Reference`
   (`contract/vtt/v1/events.proto`: `entry_id`, `place`, `state`) sits in
   `Actor.references`, field 11, a map keyed by the reference's id. The id
   is the sheet's own and is written by whoever adds the reference (D7), so
   two references to one entry are two ids. An `ActorAdded` carries them as
   part of its actor. `git diff --numstat 13eeca0..ac618a7 -- contract/vtt`
   shows lines added and none removed (rule 3).
   - The server's fold: `internal/engine/reference_change_test.go`'s
     `TestAnActorAddedKeepsItsReferencesWhole` and
     `TestAnActorAddedHoldingAReferenceWithAnEmptyIdIsRefusedAfterItsAttributeNames`
     (VTT-1023). The first passed on the contract alone, since
     `proto.Clone` keeps the map; its breaks hold it.
   - The client's fold: `client/src/state.ts`'s `Actor.references`, and
     `client/src/fold.ts`'s `copyActor`, `withReferences` and `actorAdded`
     refusal: `client/test/fold-references.test.ts`'s "an actorAdded keeps
     its actor's references whole, and an actor added with none holds
     none", "an actorAdded's references are the fold's own, not the
     event's" and "an actorAdded holding a reference with an empty id is
     refused, after its attribute names" (VTT-1023).
   - Both folds against one record: the committed fixtures
     `contract/testdata/reference_change_log.json`,
     `reference_change_dump.json` and `reference_change_refused.json`,
     folded by `cmd/vtt/reference_change_dump_test.go`'s
     `TestTheDumpAfterReferenceChangesIsTheCommittedOne` and
     `TestEachReferenceRefusalInTheCommittedFixtureTheServerRefuses`, and by
     `fold-references.test.ts`'s "the dump after reference changes matches
     the committed dump, as Go's writeDump does" and "each refusal in the
     committed reference fixture, the client's fold refuses" (VTT-1028).
   - The round trip: `contract/roundtrip_test.go`'s
     `TestActorWithReferencesRoundTrip`,
     `TestReferenceAddedEnvelopeRoundTrip`,
     `TestReferenceRemovedEnvelopeRoundTrip` and
     `TestReferenceChangedEnvelopeRoundTrip` (VTT-1027).
   - Independently: `internal/engine/qa_reference_fold_test.go`,
     `contract/qa_reference_roundtrip_test.go` and
     `client/test/qa-reference-fold.test.ts`.
   - On `13eeca0` the new tests do not build, since `vttv1.Actor` has no
     `References`; the plan's P3 measured there that `protojson` refuses an
     `Actor` holding `references` as an unknown field.
4. [x] **A sheet's references fit its ruleset where it enters.**
   `Ruleset.ReferenceFault` (`internal/rules/sheet.go`) reads one
   reference: its entry against `Catalogue`, its place against `Places`,
   its entry's form against the place's forms, and its state against the
   place's states (no state where the place names some, a state where it
   names none, a state it does not name). `Ruleset.SheetReferenceFault`
   asks it of each reference in byte order of ids and then, when all fit,
   refuses the later of two references to one entry in a place that holds
   an entry once (`copiesFault`). Each reason begins `reference "<id>"`.
   - The rule: `internal/rules/sheet_reference_test.go`'s
     `TestReferenceFaultReadsAReferenceAgainstTheCatalogueAndItsPlace` and
     `TestSheetReferenceFaultAnswersTheFirstReferenceThatDoesNotFitInByteOrder`
     (VTT-1037), and
     `TestSheetReferenceFaultRefusesASecondReferenceToOneEntryInAPlaceHoldingItOnce`
     (VTT-1038).
   - `add_actor` (`sheetFault`, `internal/gateway/add_actor_validate.go`),
     after its spend rule: `add_actor_references_internal_test.go`'s
     `TestAddActorWithARulesetRefusesAReferenceThatDoesNotFit` (VTT-1039,
     VTT-1070) and `TestAddActorReadsAnActorsReferencesAfterTheSpendRule`;
     an empty id, with a ruleset served or none,
     `TestAddActorRefusesAnEmptyReferenceIdWithOrWithoutARuleset` and
     `TestAddActorRefusesAnEmptyReferenceIdBeforeReadingAReferenceAgainstTheRuleset`
     (VTT-1040); with none served, nothing else of a reference,
     `TestAddActorWithNoRulesetReadsNoReferenceAgainstOne` (VTT-1041).
   - `adventure.Load` (`loadActors`, `actorReferences` and `referenceJSON`
     in `internal/adventure/load.go`), after its spend rule:
     `internal/adventure/actor_references_test.go`'s
     `TestAnActorFileHoldingAnEmptyReferenceIdIsRefused`,
     `TestAnActorFileHoldingAReferenceThatDoesNotFitIsRefused` and
     `TestAnActorFilesFirstMisfitInByteOrderOfIdsIsNamed` (VTT-1043); the
     copies rule,
     `TestAnActorFileHoldingASecondReferenceToAnEntryItsPlaceHoldsOnceIsRefused`
     (VTT-1072); and, beyond the item, a reference written `null`,
     `TestAnActorFileHoldingAReferenceWrittenNullIsRefused` (VTT-1073).
   - `vtt serve --ruleset` (`sheetsFit`, `cmd/vtt/serve_compose.go`):
     `cmd/vtt/campaign_reference_fit_test.go`'s
     `TestACampaignWhoseActorsHoldReferencesThatDoNotFitIsNotServed`,
     `TestACampaignActorsReferencesAreReadAfterTheSpendRule` and
     `TestACampaignWhoseActorsReferencesFitIsServed` (VTT-1044, VTT-1071).
     `sheetsFit` reads no empty id: the fold refuses an `ActorAdded` holding
     one (VTT-1023), so such a log does not open.
   - Independently: `internal/rules/qa_reference_fit_test.go`,
     `internal/gateway/qa_add_actor_references_test.go` and
     `internal/adventure/qa_actor_references_test.go`; QA drove `vtt serve
     --ruleset` as a process, which named every misfit actor in byte order
     with its first fault, key and spend faults before reference faults, and
     served a campaign whose references fit.
   - On `13eeca0` these tests do not build (no `ReferenceFault`, no
     `Actor.references`).
5. [x] **The DM and the agent change a sheet's references in play.**
   `AddReference`, `RemoveReference` and `ChangeReference`
   (`contract/vtt/v1/commands.proto`, command 39 to 41) as
   `add_reference`, `remove_reference` and `change_reference`; `ToEvent`
   (`internal/gateway/convert.go`) turns each into its `ReferenceAdded`,
   `ReferenceRemoved` or `ReferenceChanged` (payload 41 to 43), which
   `engine.Apply` and `client/src/fold.ts` fold (VTT-1024 to VTT-1026). A
   change sends the place and the state whole and keeps the entry, and a
   move names the state it lands with (D8, D9). `validateForm` calls
   `validateAddReference` (`internal/gateway/reference_change.go`);
   `handleReferenceChange` reads an addition and a change against the sheet
   with a ruleset served (`referenceSheetFault`, `referenceChangeFault`,
   `Ruleset.CopyFault`) before anything is written, and appends one
   envelope.
   - An addition: `internal/gateway/reference_change_internal_test.go`'s
     `TestAnAddReferenceAppendsOneReferenceAddedAndChangesOnlyTheSheet`,
     `TestWithARulesetAnAddReferenceThatDoesNotFitIsRefusedAndAppendsNothing`,
     `TestAnAddReferenceIsReadAgainstTheSheetsOthersAndNotForTheirFaults`
     and `TestAnAddReferenceTheFoldRefusesReachesTheIssuerAndAppendsNothing`
     (VTT-1046).
   - A change:
     `TestAChangeReferenceAppendsOneReferenceChangedKeepingItsEntry`,
     `TestAChangeIsReadAgainstThePlaceItMovesTo`,
     `TestWithARulesetAChangeThatDoesNotFitIsRefusedAndAppendsNothing` and
     `TestAChangeKeptInAPlaceHoldingAnEntryOnceIsNotReadAgainstItself`
     (VTT-1047).
   - A removal:
     `TestARemoveReferenceAppendsOneReferenceRemovedAndChangesOnlyTheSheet`
     and
     `TestARemoveReferenceTheFoldRefusesReachesTheIssuerAndAppendsNothing`
     (VTT-1048).
   - With no ruleset served, only an empty id is read:
     `TestAnAddReferenceWithAnEmptyIdIsRefusedWithOrWithoutARuleset`
     (VTT-1046) and
     `TestWithNoRulesetAnAddOrAChangeIsRefusedForNoEntryPlaceStateOrCopy`
     (VTT-1061).
   - A player's and a spectator's are refused:
     `internal/gateway/reference_change_test.go`'s
     `TestOnlyTheDMAndTheAgentMayAddChangeOrRemoveAReference` and
     `TestAPlayerOrASpectatorIsRefusedEveryReferenceCommandAndAppendsNothing`
     (VTT-1049), and `authz_test.go`'s matrix, which gained each reference
     command's cells.
   - No `RulesetNamed` is written,
     `TestNoReferenceCommandAppendsARulesetNamed` (VTT-1050), and no
     participant is sent a new watchable set,
     `TestNoReferenceCommandSendsAPlayerANewWatchableSet` (VTT-1031),
     under OD-2 B.
   - Through the MCP server: `cmd/vtt/mcp_reference_change_e2e_test.go`'s
     `TestMCPAReferenceIsAddedChangedAndRemovedAndARefusedOneAppendsNothing`,
     in VTT-1046 to VTT-1048's evidence (R36 was refused at the sort as a
     rule of its own), and `internal/mcp/reference_tools_test.go`'s
     `TestTheReferenceToolsReachTheWireCarryingTheirArguments`.
   - `client/src/commands.ts` builds them: `addReference`,
     `changeReference` and `removeReference`.
   - Independently: `internal/gateway/qa_reference_commands_test.go`, over
     real roles and both servers.
   - On `13eeca0` no such command exists, and these tests do not build.
6. [x] **Whoever is sent a sheet is sent its references.** In
   `internal/perceive/projection.go`, `classify`'s sheet arm and
   `sheetActor` take the reference events, so a party member's reach a log
   that saw it and a creature's reach none; `actorBelief` and `beliefOf`
   hold references; `statusFrames` calls `referenceFrames` after the
   ability frames and before the control frames, per id in byte order;
   `kindMovesList` counts references; `sameIncarnation` does not compare
   them; and `shownWith` gives a creature none.
   - Forwarded: `reference_event_internal_test.go`'s
     `TestAPartyMembersOwnLogIsSentItsReferenceEvents`,
     `TestALogWhoseEyesSawAPartyMemberIsSentItsReferenceEvents`,
     `TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoReferenceEvent` and
     `TestNoLogIsSentACreaturesReferenceEvent` (VTT-1032).
   - Introduced with none: `reference_correction_test.go`'s
     `TestACreatureIsIntroducedWithoutItsReferencesAndIsNeverBroughtThem`
     and `TestAnUnseenCompanionIsIntroducedWithoutItsReferences`
     (VTT-1033).
   - Brought at first sight, and never re-introduced for it:
     `TestACompanionFirstSeenIsBroughtToItsReferencesAndThenFoldsTheirEvents`,
     `TestReferencesChangedOutOfSightAreCorrectedAtSightInByteOrderAfterTheAbilities`,
     `TestAReferenceRemovedAndAddedBackAsAnotherEntryOutOfSightIsRemovedThenAddedAtSight`,
     `TestAReferenceMovedOutOfSightIsChangedAtSightToItsPlaceAndState` and
     `TestReferenceChangesUndoneOutOfSightSendNothingAtSight` (VTT-1068,
     VTT-1069).
   - A kind change: `ability_correction_test.go`'s
     `TestANamelessActorWithNoListChangingKindInSightIsForwardedTheGrantAlone`
     (VTT-1067, succeeding VTT-755).
   - `perceive.Resume`:
     `TestAResumeHoldsTheWorldsReferencesAndStartsAfreshFromALaggingSet`
     (VTT-1035); a reopened campaign,
     `internal/campaign/reference_reopen_test.go`'s
     `TestACampaignReopenedAfterReferencesAddedChangedAndRemovedResumesEveryLog`
     (VTT-1036).
   - `internal/eventgen/model.go` draws the reference events with their
     refusals, so `TestEveryCharacterLogFoldsAtEveryPrefix` and
     `TestACharacterLogIsWhatItsCharactersEyesAreGiven` walk them. No walk
     reaches a correction that takes a reference off: see "What could not
     be established".
   - Independently: `internal/perceive/qa_reference_events_test.go`,
     `qa_reference_corrections_test.go` and
     `internal/campaign/qa_reference_reopen_test.go`, the last with
     generated walks and reopens.
   - On `13eeca0` these tests do not build: no reference event exists.
7. [x] **The sheet and the console show them.** Where the ruleset's
   `sheet` lays them out: a section may show `places` as its one kind
   (`SheetSection.Places`, `checkSection` in `internal/rules/load.go`),
   naming declared places the sheet shows once, and a declared sheet shows
   every place. `places_test.go`'s
   `TestASectionShowingPlacesLoadsThemInTheOrderWritten`,
   `TestASectionShowingPlacesAndAnotherKindIsRefused` and
   `TestASectionShowingAnEmptyListOfPlacesIsRefused` (VTT-1019, succeeding
   VTT-825); `TestASectionNamingAPlaceTheRulesetLacksIsRefused` and
   `TestASheetShowingAPlaceTwiceIsRefused` (VTT-1060);
   `TestAPlaceNoSectionShowsIsRefused` and
   `TestAnUnshownAbilityIsNamedBeforeAnUnshownPlace` (VTT-1020).
   - The panel: `sheetFor` (`client/src/player.ts`) builds a section of
     places as `ShownReference`s, in the order the section lists its places
     and then by reference id, each by its entry's name, else its entry's
     id, with its state, titled by the entry's description, and leaves out
     a section holding none; `client/src/view/player.ts` draws it.
     `client/test/sheet.test.ts`'s "a section of places draws the
     references in the order it lists its places, then by reference id", "a
     reference is named by its entry's name, else its entry's id, with its
     state, titled by the entry's description" and "a section of places
     holding none of the character's references is left out, and its page
     stays", and `panel-sheet.test.ts`'s "a section of places draws each
     reference by its entry's name and its state, titled by the entry's
     description" (VTT-1055).
   - With no sheet declared, a section per place holding a reference after
     Abilities: `sheet.test.ts`'s "with no sheet declared, a section per
     place holding a reference follows Abilities, in the window's order of
     places" and `panel-sheet.test.ts`'s "with no sheet declared the panel
     draws, after Abilities, a section per place holding a reference, named
     as the window names it" (VTT-1056, succeeding VTT-840).
   - Beyond the item, by the owner's ruling of 2026-11-26: in a section
     listing more than one place, each line names its place, "Longsword
     (Wielded)". `client/test/sheet-place-names.test.ts`'s "in a section
     listing two places each reference carries its own place, named by the
     window, else by its key", "the panel draws a reference of a two-place
     section as its name, then its place and its state in parentheses" and
     "the shipped Equipment section names each of the fighter's references
     with its place" (VTT-1075).
   - The console: `client/src/view/dm.ts`'s "Catalogue" group, an actor's
     row by id, each reference shown by its id, entry, place and state,
     with a place to choose and a state to type for a change, a Remove that
     asks first, and an addition by id, entry, place and state, each sent
     through `sending`. `client/test/dm-references.test.ts`'s "an addition
     offers the window's entries and places, named, and sends the id,
     entry, place and state as given" and "a change sends the place chosen
     and the state typed, whole" (VTT-1057); "a removal asks first, naming
     the entry, the place and the actor, and a no sends nothing" and "a
     removal answered yes sends one remove_reference naming the actor and
     the reference" (VTT-1058). An accepted command clears no field of
     another of the console's forms, `dm-view.test.ts`'s "an accepted
     command clears the fields of its own form and of no other" (VTT-1074,
     the owner's rider of 2026-11-23). `client/test/command-surface.test.ts`
     names the controls `add-reference`, `change-reference` and
     `remove-reference`.
   - The panel, the console and the Events panel read the newest ruleset
     answer's catalogue and places (`client/src/app.ts`): `app.test.ts`'s
     "a ruleset answer's catalogue and places reach the player panel and the
     DM console".
   - Independently: `client/test/qa-reference-panel.test.ts`,
     `qa-reference-console.test.ts` and `qa-reference-place-names.test.ts`.
   - On `13eeca0` the client's `Actor` holds no references, `sheetFor` has
     no kind of places and the console no Catalogue group, so nothing these
     tests look for exists there (by reading; the plan's Task 10 asked that
     they fail there).
8. [x] **The places are served, and the agent is told.**
   `internal/gateway/metadata.go`'s `rulesetJSON.Places` (`placeJSON`)
   starts as an empty slice and answers each place's key, name,
   description, forms, copies and states in declared order, `states` as
   `[]` when it names none and `copies` written false rather than left out;
   a section of places is answered as `{name, kind: "places", places}`
   (`sheetSectionJSON.Places`), its kind now told by ifs in
   `handleRuleset`. `get_ruleset` returns that body verbatim (VTT-621).
   - The route: `internal/gateway/places_route_test.go`'s
     `TestEveryRoleReadsThePlacesInTheirDeclaredOrder`,
     `TestAPlaceIsServedWhetherItCopiesAndAPlaceNamingNoStateAnEmptyList`
     and `TestNoPlacesAreAnEmptyListForEveryRole` (VTT-1052);
     `TestASheetSectionOfPlacesIsServedAsKindPlacesWithItsPlaceKeys`
     (VTT-1053); and, for each shipped ruleset,
     `qa_ruleset_route_test.go`'s `TestQARRRulesetRouteAnswersEveryRole`
     and `TestQARRNothingLoadedAnswersEmptyLists`.
   - `get_ruleset`'s description (`getRulesetDescription`,
     `internal/mcp/ruleset_tool.go`):
     `TestGetRulesetSaysItAnswersThePlacesAndWhatASectionOfPlacesShows`
     (VTT-1054) and
     `TestGetRulesetSaysAKeyACatalogueFormListsIsRefusedOnASheet`
     (VTT-1014). Its sentence on where a number lives is qualified for a
     key a form lists (Task 1's review).
   - `add_actor` (`tools/toolgen/main.go`): its `attributes` says a key a
     form lists refuses the whole command,
     `TestAddActorSaysAKeyACatalogueFormListsIsRefusedOnASheet` (VTT-1014);
     its `references` says what a reference is and what it refuses,
     `TestAddActorSaysWhatASheetsReferencesAre`, under no row, and
     `TestAddActorSaysWhatAReferenceRefuses` (VTT-1051).
   - The reference tools say what each changes, what it refuses with a
     ruleset served and with none, that no player is sent a message of the
     change and whoever is sent the sheet sees it there, and that only the
     DM and the agent may use it:
     `TestTheReferenceToolsSayWhatTheyChangeWhatTheyRefuseAndWhoMayUseThem`
     (VTT-1051). `vtt.v1.Reference`'s override requires `entryId` and
     `place`: `TestAReferenceRequiresItsEntryAndItsPlaceAndNotItsState` and
     `TestTheReferenceToolsRequireWhatTheyCannotDoWithout`, under no row
     (D15's override). `cmd/vtt/tools.json`, `contract/gen/tools/tools.json`
     and `contract/testdata/expected_tools.json` are regenerated:
     `TestToolsMatchGolden` compares the definitions with
     `contract/testdata/expected_tools.json`, and `check:drift` regenerates
     `contract/gen/tools/tools.json` and `cmd/vtt/tools.json` and fails on a
     difference.
   - The client types them: `client/src/metadata.ts`'s `PlaceDef`,
     `RulesetMeta.places` and `SheetSection`'s `places` member.
   - Independently: `internal/gateway/qa_places_route_test.go`, through
     every role's own credentials and a rewriting middleware;
     `internal/mcp/qa_places_description_test.go` and
     `qa_item_value_description_test.go`.
   - On `13eeca0` `rulesetJSON` has no `places`, and the route tests do not
     build (no `rules.Place`).
9. [x] **dnd45e-minimal and its adventure use them.**
   `rulesets/dnd45e-minimal/ruleset.json` declares `wielded` (weapon,
   copies), `worn` (armor, each entry once) and `carried` (weapon and
   armor, copies), none naming a state, and an "Equipment" section, last on
   the "Character" page, showing them in that order (D21). goblin-ambush's
   `act-fighter` wields a longsword, wears chainmail and carries a
   crossbow; `act-cutter` wields a scimitar and wears leather armor;
   `act-archer` wields a shortbow, wears leather armor and carries a
   scimitar; each reference is under its entry's id. `guide.md`'s
   statblocks carry the same references, and its "Equipment" section tells
   the agent how to add, change and remove one, that where a reference is
   decides no roll, and that no player is sent a message of a change.
   - The abilities are unchanged: `git diff --stat 13eeca0..ac618a7 --
     rulesets/` names only dnd45e-minimal's `ruleset.json` and `guide.md`, and
     the `ruleset.json` diff only adds the places and the section; `git diff
     --name-only 13eeca0..ac618a7` names, of the files under a `goldens`
     directory, only `adventures/goblin-ambush/goldens/compiled-batch.json`
     and adventure-night's `state.json` and `stream.json`, and no ability
     golden.
   - The conformance suites load both: `internal/rules/conformance`'s
     `TestConformanceOverRulesetsGlob` and `internal/adventure/conformance`'s
     `TestConformanceOverAdventuresGlob`. The compiled golden names each
     actor's references (`toActorAddedDump`, VTT-1045):
     `qa_shipped_references_test.go`'s
     `TestQASRTheCompiledGoldenNamesEveryActorsReferences` and
     `qa_references_dump_test.go`'s
     `TestQAARShippedGoldensWriteReferencesWhereHeldAndPass`.
   - The guide: `qa_data_abilities_test.go`'s
     `TestQADataGuideStatblocksNameReferences` (VTT-1059), and QA's
     `TestQASRTheGuidesStatblocksNameTheActorsReferences` and
     `TestQASRTheGuideSaysHowToAddChangeAndRemoveAReference`.
   - adventure-night: `TestFoldGoldenCorpus` folds its `state.json`,
     `TestScenarioGoldenStreamsHaveNotDrifted` holds its `stream.json`, and
     QA's `TestQASRTheScenarioGoldensAgreeWithCompileAndTheFold` compares
     both with `Compile` and the fold.
   - The panel over the shipped files: `sheet-place-names.test.ts`'s "the
     shipped Equipment section names each of the fighter's references with
     its place".
   - The content as D21 wrote it: QA's `TestQASRTheContentIsD21sAsWritten`,
     against a digest derived by hand from D21. That each reference's id is
     its entry's id is D21's choice, held there, and no load rule.
   - On `13eeca0` `actorAddedDump` has no field for references (the plan's
     check 2), so no golden could name one.
10. [x] **The gate.** `task check` and `task e2e` are green.
    - `task check` on `9271f34`, from a clean tree, exits 0;
      `check:mutation` ends "zero unadjudicated survivors" over the
      packages it measures, its mutants that timed out counted as killed
      and named as not measured, as the gate prints them.
    - `task e2e` on `9271f34`, from a clean tree, exits 0; every test
      passes but the one the suite skips by `test.skip`,
      `client/e2e/handover.spec.ts`'s "a network drop while the table plays
      on ends in a Reconnect", skipped since `bafaa27` and not touched here.
    - `9271f34` changes `tools/comment-ceilings.txt` alone, so the gate's
      tree holds `ac618a7`'s code.
    - The comment ledger: `python3 tools/check-comments.py --write-ledger`
      ran once, in `9271f34`, after the last code commit. The rows of the
      files whose comment share fell are lowered and the ticket's new files
      get rows of their own; none is raised and none removed: `git show
      9271f34 -- tools/comment-ceilings.txt`.
    - The client's stored mutation report is out of date: its stamp names
      `d1ff728`, and `client/src` has changed since, this ticket among the
      changes. While it is, `check:ts-mutation` judges no client report,
      names that commit and the tickets added since, and passes (SPEC-023).
      A full run, `task check:ts-mutation:full`, is the owner's to ask for.
    - `client/e2e/table.spec.ts` reads the shipped sheet and checks that the
      Human Fighter's second page opens on that page's first section (the
      plan's P10); "Equipment" is the last section of that page.
    - The bundle: `cmd/vtt/webdist` is rebuilt and committed with each
      change that moves it, in `110614e`, `3128371`, `27cd004`, `6488a28` and
      `ac618a7`.

## What the rules became

The ticket's rules, each with what it became:
- A ruleset's places each have a key within the bound on an id, a display
  name and a description, hold entries of a form the ruleset declares, and
  name the states an entry may be in there; any other is refused at load:
  VTT-1015 (kept as written, in order), VTT-1016 (key, name, description,
  a key two places declare, the key's bound), VTT-1017 (forms and states,
  a state's bound), VTT-1018 (a field a place does not define, succeeding
  VTT-996), VTT-1021 (the schema describes places) and VTT-1022 (the
  schema states their bounds, succeeding VTT-1009). A place may name no
  state (D3): VTT-1015's
  `TestAPlaceWritingNeitherCopiesNorStatesLoadsWithNoCopiesAndNoState`.
- A place holds any number of references (the owner, 2026-11-25): refused
  at the sort as a rule of its own. VTT-1015 to VTT-1017 refuse nothing for
  a number, VTT-1038 refuses only a second reference to one entry in a
  place that holds an entry once, and QA's `TestQARF2CopiesAccepted` reads
  a place that holds copies holding several references to one entry.
- A sheet holds no value under a property a catalogue form lists; one that
  does is refused where it enters: VTT-1010 (the rule), VTT-1062
  (`add_actor`), VTT-1063 (`adventure.Load`) and VTT-1064 (`vtt serve`),
  VTT-1065 (the order of `vtt serve`'s key refusals, succeeding VTT-678),
  VTT-1014 (the descriptions), and VTT-1066 (conformance's smoke pass, the
  owner's ruling of 2026-11-25). Kept as worded by D6's order: VTT-617,
  VTT-618 and VTT-677.
- A reference names an entry the catalogue holds, in a place the ruleset
  declares for that entry's form, in a state the place names; with a
  ruleset served, any other is refused where the sheet enters and where it
  changes in play: VTT-1037 (the rule); VTT-1039, VTT-1043 and VTT-1044
  where a sheet enters; VTT-1046 and VTT-1047 in play; VTT-1041 and
  VTT-1061 with none served. An empty reference id is refused served or
  not, VTT-1040 and VTT-1043, and by both folds, VTT-1023 and VTT-1024
  (G6, D13); a reference written `null` in an actor file, VTT-1073. An
  adventure's references reach its `ActorAdded`, VTT-1042, and its golden,
  VTT-1045.
- A place that holds an entry once refuses a second reference to it, and a
  place that holds copies keeps each copy's state apart: VTT-1038 (the
  rule), VTT-1070, VTT-1071 and VTT-1072 where a sheet enters, and VTT-1046
  and VTT-1047 in play (`Ruleset.CopyFault`). No row states the second
  half in its words: each reference has its own id (D7), and the folds
  change only the reference an event names, "changing nothing else"
  (VTT-1024 to VTT-1026, whose tests compare the whole map);
  `dm-references.test.ts`'s "each reference's row shows its id, so two
  copies of one entry in one place and state are told apart" holds the
  console's half.
- Only the DM and the agent change a sheet's references: VTT-1049, with
  each command's cells in `authz_test.go`'s matrix and no `playerRules`
  entry.
- Whoever is sent a sheet is sent its references: VTT-1032 (forwarded),
  VTT-1033 (introduced with none), VTT-1068 and VTT-1069 (brought at first
  sight, succeeding VTT-1034), VTT-1067 (a kind change, succeeding
  VTT-755), VTT-1035 (`Resume`) and VTT-1036 (reopen); by OD-2 B, no
  participant is sent a new watchable set, VTT-1031.
- `/api/ruleset` and `get_ruleset` answer the places, and the tools'
  descriptions say what they refuse: VTT-1052 and VTT-1053 for the route,
  VTT-621 for `get_ruleset`'s body, VTT-1054 for its description, and
  VTT-1014 and VTT-1051 for the tools.

Beyond the ticket's rules: VTT-1019, VTT-1060 and VTT-1020 (a section of
places); VTT-1027 and VTT-1028 (the round trip, and both folds against one
record); VTT-1029 (the Story draws no line, succeeding VTT-809) and
VTT-1030 (the Events panel names them); VTT-1050 (no `RulesetNamed`);
VTT-1055, VTT-1056 (succeeding VTT-840) and VTT-1075 (the panel);
VTT-1057, VTT-1058 and VTT-1074 (the console); VTT-1059 (the guide's
statblocks).

The plan's labels:

| Plan | Id | |
|---|---|---|
| R1 | VTT-1010 | `SheetKeyFault`'s sweep of a form's keys |
| R2 | VTT-1011 | `add_actor`; withdrawn for VTT-1062 |
| R3 | VTT-1012 | `adventure.Load`; withdrawn for VTT-1063 |
| R4 | VTT-1013 | `vtt serve`; withdrawn for VTT-1064 |
| R5 | VTT-1014 | the descriptions say a value a form lists is refused |
| R6 | VTT-1015 | places kept as written, in order |
| R7 | VTT-1016 | a place's key, name, description; the key's bound |
| R8 | VTT-1017 | a place's forms and states |
| R9 | VTT-1018 | the defined fields with `places`; succeeds VTT-996 |
| R10 | VTT-1019 | a sheet's shapes with places; succeeds VTT-825 |
| R10b | VTT-1060 | a section's places named |
| R11 | VTT-1020 | a declared sheet shows every place |
| R13a | VTT-1021 | the schema describes places |
| R13b | VTT-1022 | the schemas state the bounds; succeeds VTT-1009 |
| R14 | VTT-1023 | an `ActorAdded`'s references, both folds |
| R15 | VTT-1024 | `ReferenceAdded`, both folds |
| R16a | VTT-1025 | `ReferenceRemoved`, both folds |
| R16b | VTT-1026 | `ReferenceChanged`, both folds |
| R17 | VTT-1027 | the round trip |
| R18 | VTT-1028 | both folds against the committed fixtures |
| R19a | VTT-1029 | the Story; succeeds VTT-809 |
| R19b | VTT-1030 | the Events panel |
| R19c | VTT-1031 | no new watchable set (OD-2 B) |
| R20 | VTT-1032 | a party member's events forwarded |
| R21 | VTT-1033 | introduced with none |
| R22 | VTT-1034 | first sight; withdrawn for VTT-1068 and VTT-1069 |
| R23 | VTT-1035 | `Resume` |
| R24 | VTT-1036 | reopen |
| R25a | VTT-1037 | a reference fits |
| R25b | VTT-1038 | the copies rule |
| R26a | VTT-1039 | `add_actor`, served |
| R26b | VTT-1040 | `add_actor`, an empty id served or not |
| R26c | VTT-1041 | `add_actor`, none served |
| R27a | VTT-1042 | an adventure's references read and compiled |
| R27b | VTT-1043 | an adventure's references refused |
| R28 | VTT-1044 | `vtt serve` |
| R29 | VTT-1045 | the compiled golden's dump |
| R30 | VTT-1046 | `add_reference` |
| R31 | VTT-1047 | `change_reference` |
| R32 | VTT-1048 | `remove_reference` |
| R33 | VTT-1049 | a player and a spectator refused |
| R34 | VTT-1050 | no `RulesetNamed` |
| R35 | VTT-1051 | the tools' descriptions |
| R37 | VTT-1052 | the route's `places` |
| R38 | VTT-1053 | a section of places served |
| R39 | VTT-1054 | `get_ruleset`'s description |
| R40 | VTT-1055 | the panel's section of places |
| R41 | VTT-1056 | the default page; succeeds VTT-840 |
| R42a | VTT-1057 | the console changes |
| R42b | VTT-1058 | it asks before a removal |
| R45 | VTT-1059 | the guide's statblocks |
| R46 | VTT-1061 | none served, in play |

R12 (a form's charges), R43 and R44 (the player told) left with OD-1 A and
OD-2 B; their labels are not reused.

Added after Task 0: VTT-1062, VTT-1063, VTT-1064 and VTT-1065 (Task 1's
review); VTT-1066 (Task 1, the owner's ruling of 2026-11-25); VTT-1067,
VTT-1068 and VTT-1069 (Task 6's review); VTT-1070, VTT-1071 and VTT-1072
(Task 7's review); VTT-1073 (Task 8's QA); VTT-1074 (Task 10, the owner's
rider of 2026-11-23); VTT-1075 (Task 12, the owner's ruling of
2026-11-26).

Withdrawn, each with its successor:
- On 2026-11-25: VTT-996 for VTT-1018 (Task 2), false as worded once
  `rules.Load` reads `places`; VTT-825 for VTT-1019 (Task 2), false once a
  section may show places; VTT-1009 for VTT-1022 (Task 2), narrower than
  its tests once they probe a place's key and a state; VTT-1011 for
  VTT-1062, VTT-1012 for VTT-1063 and VTT-1013 for VTT-1064 (Task 1's
  review), false as worded, since an actor also holding an undeclared or
  other-type key, or meeting an earlier refusal, is refused for that and
  the listed key is not named; VTT-678 for VTT-1065 (Task 1's review),
  ambiguous once a key a form lists "does not fit" too; VTT-809 for
  VTT-1029 (Task 4), narrower than its tests once they hold the reference
  events; VTT-755 for VTT-1067 (Task 6's review), false once a kind change
  introduces afresh an actor holding references and nothing else it names;
  VTT-1034 for VTT-1068 and VTT-1069 (Task 6's review), false as worded,
  since a party member whose kind changes, or that a log holds without its
  sheet, is introduced afresh at first sight.
- On 2026-11-26: VTT-840 for VTT-1056 (Task 10), false once the default
  page shows a section per place holding a reference.

Reworded in place before they were committed: VTT-1066, which now names
"the first such ability in byte order of ids when no ability before it
fails the smoke pass" (Task 1's second review round); VTT-1073, which now
reads "after its refusal of an empty reference id" and names "the first
such reference in byte order of ids" (Task 8's review); VTT-1074, from a
field "that did not feed it" to "no field of another of the console's
forms" (Task 10's review).

Rows kept that gained evidence: VTT-675 (QA's `add_actor` test with no
ruleset), VTT-621, VTT-831 and VTT-824 (Tasks 2 and 3), VTT-272 (Task 4),
VTT-468 (QA's kind-change tests, Task 6) and VTT-839 (Task 10).

Refused at the sort, each held elsewhere: an addition, a change and a
removal through the MCP server (R36), held by VTT-1046 to VTT-1048's
evidence; a place holding any number of references (above); the
conformance suites loading dnd45e-minimal and goblin-ambush
(`TestConformanceOverRulesetsGlob` and VTT-802); goblin-ambush's actors
holding their weapons and armour (its compiled golden, through VTT-1045).

The owner's rulings this work stands on:
- 2026-11-24: ADR-012 step 1 item 5 is split into 5a, 5b and 5c, and until
  step 2 makes equipping an effect only the DM and the agent change what a
  sheet carries; and, during 5a, "Du kan sätta vilka värden du vill när
  det gäller utrustning. Det viktiga är att vi kan läsa och använda vilka
  värden som helst för vilken utrustning som helst oberoende av
  regelverket." (whatever values you like for equipment; what matters is
  reading and using any values for any equipment, whatever the ruleset), so
  every test uses invented keys.
- 2026-11-25: one ticket, its tasks committed separately. A place has no
  limit: "It is normaly weight vs strength(some attribute) that determines
  how much a character can carry", which comes as the ruleset's own sum
  with derived properties (step 1 item 6), and until then the DM judges.
  The hole 5a left open (its OD-2 A) closes as "an item's value is
  refused": a value a catalogue form lists belongs to the item, and a sheet
  holding it is refused, not the sheet made a form.
- 2026-11-25, at sign-off: OD-1 A, no charges in this ticket; OD-2 B, no
  player is told that their character gained or lost an item, with the
  reason "om en tjuv skulle stjäla något, utan att karaktären märker det
  "pick pocket sucess" då kan vi inte produkten berätta att - du blev av
  med ditt långsvärd" (if a thief steals something without the character
  noticing, a pickpocket's success, the product cannot then tell them they
  lost their longsword); the plan "Approved", with D18's and D21's
  preferences.
- 2026-11-25, at Task 1's review: the conformance fixture's fix rides Task
  1, "Yes, in Task 1" (VTT-1066).
- 2026-11-26, at Task 12's review: keep the one Equipment section and name
  each line's place, "Place in each line" (VTT-1075).
- Standing rulings applied: 2026-11-06, sight decides and no visibility
  per property; 2026-11-08, no value on a sheet the DM did not write;
  2026-11-22 and 2026-11-23, every removal in the console asks first;
  2026-11-23, `.sort(byKey)` rides the next `player.ts` ticket and the
  console-fields row the next console ticket; 2026-10-21, a near-theoretical
  edge case gets no code, test or debt; 2026-09-04, no migration.

## Rule 9

The ticket records MapTool's answer, and the plan's rule-9 section completes
it (G1), measured as the plan's P15 at RPTool `63862e4` and MapTool
`f4b7fef6c`. MapTool's core has no item and no list of what a token carries: a
token holds the properties its type declares, each a string with a
`defaultValue`, an `ownerOnly` and a `gmOnly` (`TokenProperty`). The owner's
rule framework keeps equipment in token properties of fixed name,
`EquippedWeapon`, `EquippedImplement` and `EquippedOffhand`
(`~/dev/RPTool/rule-framework/src/RuleEngine.ts`), each a JSON object holding
the item's own numbers, which an ability reads, and an attack whose weapon
gives no die rolls `1d4`. The older framework it migrates from had `Weapon`
and `Weapon2` (`~/dev/RPTool/Documentation/migration-guide.md`), a second
weapon being a second property of fixed name. A search for an inventory, a
carried list or a backpack over the framework and over MapTool's sources found
none.

Borrowed: a named place on the character holding the item an action will
read; the reading is 5c's. Not borrowed: the copy on the token (principle
27: a reference names an entry and holds only its place and state); place
names fixed in the engine (principle 23: the places are the ruleset's, and
one place holds any number of references, so a second weapon needs no
second name); the fallback die and `TokenProperty`'s default (the ruling of
2026-11-08: no value the DM did not write); `ownerOnly` and `gmOnly` (the
ruling of 2026-11-06: sight decides, never a flag per property).

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as verified | Revised before Task 0's commit: items 3 and 5 and the copies rule lost "charges", and "Not this ticket" gained charges and telling a player; the plan's verdict and checks describe the draft that was verified (its check 3 quotes "keeps each copy's state and charges apart") | The owner answered OD-1 A and OD-2 B at sign-off, after the verification; the ticket and the plan are written for those answers (`4d6c89b`'s message) |
| R2 to R4 and VTT-678, as the sort worded them | VTT-1011, VTT-1012, VTT-1013 and VTT-678 withdrawn for VTT-1062 to VTT-1065 at Task 1's review | An actor also holding a key the ruleset does not declare, or declares as the other type, or meeting an earlier refusal, is refused for that, and the listed key is not named; the rows' own evidence cells cited such counterexamples (M1). VTT-678's "the first of its keys that does not fit" could be read as a key a form lists, while the check and its tests name the first undeclared or other-type key (S1) |
| Task 1's files: `SheetKeyFault` and its callers' tests | VTT-1066 added and `internal/rules/conformance/conformance.go` changed: the fixture actors hold only what a sheet may hold, and the smoke pass walks every expression of each compiled power | Task 1's review (S4): `buildFixtureState` gave each fixture actor every number property, those a form lists among them, so an ability reading a weapon's number from its actor passed the smoke pass while every sheet in play now refuses that key; the owner ruled it into Task 1. Phase 4a then found a key read only on the miss branch still passed, since the smoke resolution takes one branch: the walk (`unheldRead`) is the fix |
| Task 1's files: `cmd/vtt/tools.json` | `contract/testdata/expected_tools.json` and `contract/gen/tools/tools.json` regenerated too | `TestToolsMatchGolden` compares the definitions with `contract/testdata/expected_tools.json`, and `check:drift` regenerates `contract/gen` and `cmd/vtt/tools.json` and fails on a difference; `add_actor`'s description changed, and the plan named `cmd/vtt/tools.json` only |
| D5: `checkSheet` gains a fourth kind | Its section body moved into `checkSection`, written as ifs; the kind refusal's words now name places | `checkSheet` stood at gocyclo's bound, so a fourth kind could not go in place, and rule 2 moves no lint bound. Task 2's reviewer loaded every tracked fixture before and after: each that loaded loads alike, and each refused is refused byte for byte as before but for the kind message |
| The plan's "The state here" lines | Between commits, windows the plan did not name: from `db2ab40` to `f125a92` `/api/ruleset` answered a section of places as kind `abilities`; from `f125a92` to `6488a28` `sheetFor` threw on a served sheet holding a section of places; from `110614e` to `47663fa` `add_actor` accepted any references; from `110614e` to `c845784` a log that met a party member unseen held none of its references, and would have refused a forwarded `ReferenceRemoved` | Each closed in a later task, and none could reach a table: no ruleset under `rulesets/` declared a place until `ac618a7`, which came after `6488a28`, and only the fixture `internal/rules/testdata/valid` did, which no table serves; nothing wrote the reference events until `27cd004`, after `c845784`; the windows existed on the branch alone (`f125a92`'s, `110614e`'s and `c845784`'s messages record theirs, and the others are read from the commits' trees) |
| Task 4's files | `.go-arch-lint.yml` gained an `excludeFiles` line for `contract/qa_reference_roundtrip_test.go` | `contract/` lies outside the `contract` component's pattern, which is why the file already excludes `roundtrip_test.go` and `contract_test.go` for the same reason; the line attaches a test and exempts no product code |
| Task 5: the client's dump writes references | Every map in the browser's dump is now written in the byte order of its UTF-8 (`byUTF8`, `Sorted` and `encode` in `client/src/fold.ts`), and the committed fixture holds integer-like and astral reference ids | Phase 4a found `sortedMap` wrote integer-like keys in numeric order and the rest by UTF-16 code units, while `encoding/json` writes UTF-8 byte order, for every map, actor ids included. The defect is older than this ticket and reached no table; its recipe is in `docs/verification-debt.md` (2026-11-25), and every parity golden is byte for byte as before |
| D19: the namer walks the log for each reference's "last entry and place" | A removal's and a change's line names what its reference held just before that event (`held` in `client/src/view/naming.ts`) | Phase 4a showed that with a reference id reused for another entry, earlier lines named the newer entry. VTT-1030's tests hold the reading; SPEC-022 is to state it |
| D10: the client mirrors each refusal's words | The new arms use `engine.Apply`'s words less `engine: `, with ids interpolated raw where Go uses `%q`; the client's earlier arms keep their own words | Task 5's review (S1) judged D10's form right for the new arms; SPEC-022 says the earlier arms word their own refusals, and is to state both |
| D12's draws: a band in `internal/eventgen` | Cut first from `moveToken`'s band, then moved to the reviewer's variant | The first band starved the give's refused draws (Task 6's review, S3); the variant leaves them as they were. Every later walk changes, and the property, oracle and model guards pass on the shifted walks |
| R26, R27 and R28: refuse a reference that "does not fit" | The copies rule gained rows of its own where a sheet enters: VTT-1070, VTT-1071 and VTT-1072 | Task 7's review (S1): VTT-1037's "does not fit" leaves out the copies rule, which VTT-1038 states apart, so VTT-1039, VTT-1043 and VTT-1044 did not hold it |
| Task 8: an actor file's references | A reference written `null` is refused, after an empty id and before the ruleset is read (VTT-1073) | `actorJSON.References` decodes to a map of pointers, so `null` arrives as nil; the plan named no refusal of it, and Phase 4a found no row held the one the implementer wrote |
| Task 7's commit | `internal/rules/sheet.go` above its comment ceiling from `47663fa`; `ReferenceFault`'s doc shortened to a pointer at VTT-1037's faults in `8d25667` | `check:comments` is not in the pre-commit hook, so `47663fa` passed it; Task 8's implementer found it. The ceiling did not move (rule 2) |
| Task 9: the commands dispatch beside `handleGiveAbility` | `handleSheetChange` (`internal/gateway/server.go`) dispatches give, take, add and change, and `referenceCommandName` (`authz.go`) names the new commands | `handleCommand` stood at gocyclo's bound, and `commandName` would have passed it with the reference commands' arms. Every command reaches the handler it did before: each dispatch arm's break, and VTT-761's, VTT-762's and VTT-988's tests |
| D14: the copies rule against the sheet in the gateway | `Ruleset.CopyFault`, in `internal/rules` beside `SheetReferenceFault`, shares its reason with it | Task 9's review (M1): the gateway's first `copyFault` repeated `copiesFault`'s words and decision, one rule in two homes. Task 7's review had shown why `SheetReferenceFault` over the sheet with the new reference will not do: it blames an existing reference when the new id sorts first |
| D18: the console's group, placed by nothing | "Catalogue" sits before "Remove condition": Notes, Catalogue, Remove condition, Abilities, Who controls what | `client/test/qa-dm-abilities.test.ts` pins Abilities immediately after "Remove condition" and immediately before "Who controls what" (VTT-730). Not put to the owner (under "What could not be established") |
| D18: per actor, its references and an addition | An actor's row is drawn only while it holds a reference or an addition is offered, the group only with a row; each reference's id is shown in its row and in the removal question | Task 10's review (S4, S5), decided without the owner: a row with nothing to show or do was noise, and two copies of one entry in one place and state could not be told apart without the id |
| Task 10's files | `sheetFor`'s fallback sorts with `.sort()`, and `byKey` and its equivalents left `tools/ts-mutation-equivalents.txt`; VTT-1074 allocated; the namer's equivalent re-pointed at its new code line | The owner's rulings of 2026-11-23 had both ride the next `player.ts` and the next console ticket. The fallback sorts the keys of a `Set`, no two equal, so `.sort()` orders them as `byKey` did, and `byKey`'s equivalent mutants go with it. The namer's line gained the catalogue and places, and SPEC-020 keys an entry by its code, so the entry follows its code and its reason names the new members |
| VTT-1074 as first worded | Reworded in place: "clears no field of another of the console's forms" | As first worded ("that did not feed it") it was false for a row's Give, which clears both value fields whether or not they fed it, as SPEC-022 says (Task 10's review, M1); the walk gained the Give, Take and Say forms |
| Task 11 | Withdrawn | OD-2 B (D22): no player is told, so the watchable set gains no field and the window shows no message |
| D17 and VTT-1055: a reference shown by its entry's name and its state | VTT-1075 added after Task 12's review: in a section listing more than one place each line names its place, "Longsword (Wielded)"; D17's wording is now narrower than the code | The one Equipment section, which D21 gave all of its places, drew the fighter's items with no word of where each was. The owner ruled on 2026-11-26 to keep the one section and name each line's place (`ShownReference.place`, filled only in such a section). SPEC-022 is to state it |
| Task 12's files | `tools/ts-mutation-inputs.py`'s `FILES` and its QA test's `NEW_FILES` gained the shipped ruleset, the longsword, chainmail and crossbow entries and the fighter's actor file | `sheet-place-names.test.ts` and `qa-reference-place-names.test.ts` read those shipped files, outside `client/`, and the stored report's hash must cover every file a client test reads, or a report stays CURRENT after one changes (SPEC-023). The hash widens, so a report goes out of date sooner and never later: an input widened, not a gate weakened (Task 12's second review, rule 2) |
| Task 12: adventure-night's `stream.json` re-recorded (P10) | The references were added to its recorded `actorAdded` envelopes, in the order the recorder writes them, and it was not re-recorded; `compiled-batch.json` was derived by hand and checked with `DumpCompiledBatch` | The precedent is `f3cd27b`, which gave the same envelopes their ability ids the same way. `TestScenarioGoldenStreamsHaveNotDrifted` compares the recorded stream byte for byte, so a stream the recorder would not write fails it |
| Task 12's guide | It says where a reference is decides no roll | Task 12's review (S1): the fighter carries its crossbow, and `crossbow-shot` is in the turn flow; nothing told the agent that `use_ability` reads no reference |

## The deliberate breaks

Each applied, seen red unless said otherwise, and restored:
- Task 1 (`38f75be`): the sweep dropped (the rules, gateway, adventure and
  serve tests); run before the attributes sweep and between the two (the
  order tests); the last form named (the two-forms tests); the keys
  unsorted, sorted in place, the first form only, the forms in the outer
  loop; each description sentence dropped (both description tests, the
  tools golden); every number back on the conformance fixture; the walk
  dropped, one branch only, the roll, the vs and the effect entries each
  dropped, the role swapped, the form unset (the conformance cause tests
  and QA's miss case). One shipped-data break stayed green: no shipped key
  is listed by two forms, so "the first in declared order" is held by the
  invented fixture alone.
- Task 2 (`db2ab40`): every refusal dropped in turn; each bound doubled in
  `checkManifestBounds` and removed from the schema; `loadV2`'s copy,
  `Copies` and the states copy dropped; places counted out of a section's
  kinds; the empty-list, undeclared, shown-twice and every-place checks
  dropped, the last moved before the abilities'; each section arm
  disabled; the schema's required forms and its places branch removed.
- Task 3 (`f125a92`): places starting nil; places served to the DM alone;
  the `[]` of `copies`, `forms` and `states` dropped; places sorted; the
  section arm removed and negated; the section's keys dropped; each
  description phrase dropped.
- Task 4 (`110614e`): each fold refusal dropped and reordered; the map
  written in place in each arm; the event's `Reference` stored; the entry
  not kept and the old state kept; the actor stored unclone; `classify`'s
  party and sight conditions and each `sheetActor` arm dropped; contract
  fields dropped and regenerated; the references doc emptied; `STORY` set
  true; `describe` naming the actor for the reference.
- Task 5 (`3128371`): `copyActor` dropping or sharing references; each
  refusal dropped, and refusals reordered; the event's reference stored; a
  change keeping the old place or state; the dump dropping references,
  writing an empty state, or writing state before place; the dump sorted by
  UTF-16, by JavaScript's integer order, by `charCodeAt`, or without its
  length tiebreak; an empty `entry_id` written; the names by id where the
  window holds one; the log walk dropped, skipping each event, or answering
  the latest instead of the reference as it was. Green: a map written in
  place in a client arm, since `fold()` is `apply`'s only caller and the
  log is folded afresh each time.
- Task 6 (`c845784`): references compared in `sameIncarnation`; left out of
  `statusFrames`; a change judged by entry, place or state alone;
  `kindMovesList` without references or counting one side; ids in reverse
  or case-folded order; each branch of `referenceFrames`; the belief
  holding none; the frames before the abilities or after the controllers;
  `shownWith` keeping references; eventgen with no reference or removal
  draw, a draw never refused, an unsorted pool, a removed actor keeping
  references (green until a draw test was added); the creature test's move
  into sight.
- Task 7 (`47663fa`): each of `ReferenceFault`'s checks dropped (dropping
  the entry or the place check panics the later lookups, so the order
  holds them); the copies rule dropped, compared across places, keyed by
  place alone, ignoring `copies`, run before the fit check; ids unsorted;
  `add_actor` reading no reference, reading them before the spend rule, its
  empty-id check dropped, read after the ruleset, read only with a ruleset,
  read before the ability ids or before the empty ability id; references
  read with no ruleset; `sheetsFit` reading none or reading them before the
  spend rule; the doc's sentences dropped.
- Task 8 (`8d25667`): `buildActor` dropping the references; the dump
  writing them always, without `omitempty`, writing or dropping the state;
  the empty-id refusal dropped or moved after the ruleset read; a reference
  written `null` read as empty, or read in map order;
  `SheetReferenceFault` not read; references read before the spend rule;
  the copies rule dropped; each of `ReferenceFault`'s checks dropped.
- Task 9 (`27cd004`): each refusal dropped; the empty id read only with a
  ruleset; the change read against its old place or against itself;
  `SheetReferenceFault` over the sheet with the new reference;
  `CopyFault`'s skip, `Copies`, place and nil-place guard; a player or a
  spectator given a cell; a `RulesetNamed` written; reference ids copied
  into the watchable set; the copies rule read with no ruleset; each
  dispatch and `ToEvent` field dropped; each description sentence; the
  client builders' fields.
- Task 10 (`6488a28`): references ordered by entry name or by id alone; an
  empty section drawn; a removal sent unasked; an entry or a place named by
  its id where a name is held; the default page's place sections out of
  order or named by key; a change sending the old place or state; each
  data-action changed; the panel's state and title dropped; the console's
  locks per actor; each blank-field guard; rows for bare actors, or none
  where an addition is offered; the id left off a row or the question; an
  accepted addition, change, removal or Give clearing another form's field;
  the fallback's keys unsorted.
- Task 12 (`ac618a7`): a statblock with another reference; a golden left
  unmoved; the declared sheet without the Equipment section;
  adventure-night's state left unmoved; the place dropped from a line,
  named in a one-place section, named by key where a name is held, the
  wrong place, named only when a character holds several places.

## The existing tests that changed

- **Citations of withdrawn rows:** every test and comment citing VTT-678 cites
  VTT-1065 (`cmd/vtt`'s `campaign_fit_test.go`, `qa_campaign_fit_test.go`,
  `qa_serve_spend_test.go`, `sheetsFit`'s doc, and `internal/rules/sheet.go`'s
  `SheetKeyFault` doc), and likewise VTT-996, VTT-825 and VTT-1009 (Task 2),
  VTT-809 in `client/` (Task 4), VTT-755 (Task 6) and VTT-840 (Task 10) their
  successors. `git grep -n -E
  'VTT-(996|825|1009|809|840|678|755|1011|1012|1013|1034)([^0-9]|$)' ac618a7
  -- internal cmd client/src client/test tools contract rulesets adventures
  scenarios` prints nothing; the specifications still name some (under "What
  could not be established").
- **Conformance's fixture:** `buildFixtureState` asks `SheetKeyFault` of
  each number property and gives a fixture actor only those a sheet may
  hold (Task 1, VTT-1066).
- **The fixture and the manifest's field list:** `testdata/valid` gains
  places and a section of them; `qa_declared_sheet_test.go`'s field list
  gains `places`; `id_bound_internal_test.go` and
  `qa_property_format_test.go` gain a row each for a place key and a state;
  `formsManifest`, `qaivRuleset` and `sheetRuleset`, which copy the
  fixture and write their own manifest, call `dropPlaces` (Task 2).
  `schema_test.go`'s tables needed no change: the walker skips a section's
  `places` as it skips its `conditions`, `abilities` and `spends` (the
  plan's P9), and `TestASectionShowingPlacesAndAnotherKindIsRefused` holds
  that shape.
- **The route's sheet helpers:** `sheet_route_test.go`'s `sheetAsJSON`,
  `qa_sheet_route_test.go`'s `qaSheetExpect` and `declaredSheetAsRoute`
  gain a places arm, so Task 12's content does not panic them, and QA's
  check that dnd45e-minimal declares no place left its loop (Tasks 2 and
  3). `qa_ruleset_route_test.go`'s comparisons and its nothing-loaded list
  gain `places`; `internal/mcp/ruleset_tool_test.go` gains the
  description's tests (Task 3).
- **The folds' neighbours:** `internal/engine/ability_change_test.go`'s
  `abilityTable` actor holds references, so the ability, resource and
  ruleset tests also check them untouched; the envelope helpers
  `internal/campaign/campaign_test.go`'s `cenv` and
  `internal/perceive/viewer_test.go`'s `envelope` gain the reference arms;
  `contract/events.test.ts` names the new payloads (Task 4). The plan
  expected `projection_internal_test.go`'s rulings to change;
  `TestEveryEnvelopePayloadArmHasAnExplicitRuling` walks the oneof and
  needed no edit once `classify`'s sheet arm took the events.
- **The client's fixtures:** every client test building an `Actor` gains
  `references: {}`, the type now requiring it (Tasks 5 and 10);
  `qa-story-mechanical.test.ts` and `qa-companion-first-sight.test.ts`
  gain the reference kinds (Task 5, at Task 4's review).
- **The projection's walks:** `internal/eventgen/model_test.go`'s kind lists
  and `draws_internal_test.go` gain the reference draws; QA's kind-change
  tests cite VTT-468 (Task 6).
- **The commands:** `authz_test.go`'s matrix gains each reference
  command's cells; `internal/mcp/tools_test.go` and
  `tools_internal_test.go` gain the tool names (Task 9);
  `command-surface.test.ts` named the controls on its not-user-issued
  hatch until Task 10 gave them the console (Task 10).
- **The content:** `TestQAARShippedGoldensHoldNoReferencesAndPass`, which Task
  8's QA added in `8d25667`, became
  `TestQAARShippedGoldensWriteReferencesWhereHeldAndPass`, since D21 made its
  "none" half false; the expected values of the two-place sections in
  `sheet.test.ts`, `panel-sheet.test.ts` and `qa-reference-panel.test.ts`
  gained the place (VTT-1075); QA's panel fixture was made able to fail (Task
  12).
- **Unchanged:** `rulesets/tavern-brawl/`, `internal/harness/`, every
  ability golden, and `tools/mutation-equivalents.txt`.

## Phase 4a adjudications

- Task 1 (`internal/rules/qa_item_value_test.go`,
  `internal/adventure/qa_item_value_test.go`,
  `internal/gateway/qa_add_actor_item_value_test.go`,
  `internal/mcp/qa_item_value_description_test.go`; `vtt serve --ruleset`
  driven as a process): no defect in the sheet's refusal.
  - QA G1: the form is named by its key, not its name — SPEC-025 or
    SPEC-028 to state it — no change.
  - QA G2: no row quotes the refusal's words — the specification to state
    them — no new row.
  - QA G3: any declared form counts, with entries or none — SPEC-027's
    definition of a form — no change.
  - QA G4: `get_ruleset`'s sentence lacks "with a ruleset served" — with
    none loaded `forms` is `[]`, so it names nothing — no change.
- VTT-1066 (`internal/rules/conformance/qa_form_listed_property_test.go`):
  one behaviour defect, fixed.
  - QA finding 1: a form-listed key read only on the miss branch passed
    `Run`, the smoke resolution taking one branch — fixed by walking every
    expression of the compiled power, QA's tests rerun green.
  - QA G1: the ability is named by id and the cause named no form — the
    walk's cause now names the place, the key, the role and the form (Task
    1's second review round).
  - QA G2: which ability is named when several read listed keys — the first
    in byte order of ids — VTT-1066 reworded to say so.
  - QA G3: a resource's `default_max_expr`, or a threshold reading a listed
    key where only an untaken branch changes its resource — near-theoretical
    (the ruling of 2026-10-21) — no code, no debt; the reviewer agreed, and
    named `crossValidateResources` in `rules.Load` as the place if it is
    ever closed.
  - QA G4: "reads" means on any branch — the fix makes it so — no change.
- Task 2 (`internal/rules/qa_places_test.go`): no defect. Each for the
  specification, with no code and no row:
  - QA G1: `null` on a place's key, name, description or forms reads as
    empty and is refused, and `null` copies or states load as none — the
    manifest's rule that absent and `null` read alike (SPEC-024).
  - QA G2: place keys and states compare byte for byte, `Mike` and `mike`
    two places — as SPEC-027's case rule.
  - QA G3: the bounds are checked place by place, a place's key then its
    states.
  - QA G4: the unshown place named is the first in declared order, where
    VTT-828 names abilities in id order — places have a declared order —
    SPEC-028 to state it.
  - QA G5 to G7: where the place refusals sit, before conditions and the
    sheet; the field a kind refusal names, `sheet[<i>].sections[<j>]`; and
    the words — the specification to state them.
  - QA G8: `Ruleset.Place` is held by
    `TestARulesetKeepsItsPlacesInDeclaredOrder` and used from Task 7.
  - Needs an id: D3's order within a place, where its bounds sit,
    `loadPlaces` after `loadForms`, D5's shapes — held by the order tests
    and stated in the specification, as 5a's refusal order is in SPEC-027
    — no new row.
- Task 3 (`internal/gateway/qa_places_route_test.go`,
  `internal/mcp/qa_places_description_test.go`; every role through
  `/api/me`, injections by a rewriting middleware): no defect. For the
  specification, no rows:
  - QA G1: a section's place keys come in the order the section lists them,
    as `values`' keys do (SPEC-012).
  - QA G2: a place's forms and states are served in the order written
    (VTT-1015).
  - QA G3: VTT-1054 is held for presence; SPEC-012 to state the words.
  - QA G4: `copies: false` is written, not left out.
  - Needs an id: `get_ruleset` reads the route verbatim with its own token
    (VTT-621's); the exact fields of a place and the section's one list
    (D16) — held by the specification — no row.
- Task 4 (`internal/engine/qa_reference_fold_test.go`,
  `internal/perceive/qa_reference_events_test.go`,
  `contract/qa_reference_roundtrip_test.go`): no defect. For the
  specification, no rows:
  - QA finding: the refusals' words follow the ability and resource arms'.
  - QA finding: "after its attribute names" means after the fold's existing
    name checks (VTT-290, VTT-297 and SPEC-018's).
  - QA finding: "a fresh map" means no aliasing in either direction.
  - QA finding: "has seen" is SPEC-016's sighted.
  - QA finding: a party member added holding references while unseen is
    introduced without them and brought them at first sight — Task 6's
    subject.
  - QA finding: a nil value in an `ActorAdded`'s references map, possible
    from Go or the binary wire alone — near-theoretical — no code.
- Task 5 (`client/test/qa-reference-fold.test.ts`,
  `qa-reference-events-panel.test.ts`; the server's dumps measured with `go
  test -overlay` in a scratch copy, nothing written to the repository):
  - QA finding 1: behaviour defect, older than this ticket — the browser's
    dump ordered integer-like and astral keys otherwise than Go's — fixed
    (the deviations), its recipe in `docs/verification-debt.md`.
  - QA findings 2 and 3: an ambiguity in D19's "last entry and place" —
    with a reference id reused for another entry, a removal's and a
    change's line named the newer one — ruled that a line names what its
    reference held when its event was written; fixed, SPEC-022 to state it.
  - QA finding: the lines' words, a state shown raw (states have no
    names), a change's line naming the place it sets, and "the names the
    window holds" being the newest `/api/ruleset` answer's — SPEC-022 to
    state them — no rows.
  - QA's byte-order test stayed red after the fix: its expected dump was a
    `String.raw` template, in which bun's transpiler turns non-ASCII into
    `\u` escapes — the harness's, not the client's — that one literal is a
    plain template now.
- Task 6 (`internal/perceive/qa_reference_corrections_test.go`,
  `internal/campaign/qa_reference_reopen_test.go`, with eventgen walks,
  reopens and a trap on logged errors; every assertion red under a harness
  break): no defect. For the specification, no rows:
  - QA finding: the reference frames come after the ability frames and
    before the control frames; the frames of one id stay together, a
    removal then an addition; ids in byte order.
  - QA finding: a nil reference value in an `ActorAdded`'s map, from Go or
    the binary wire alone — near-theoretical; the projection copes, as
    QA's test shows.
  - QA finding: SPEC-016's correction-frame shape, the order of its
    transitions and Perch's sequence have no rows — behaviour older than
    this ticket.
- Task 7 (`internal/rules/qa_reference_fit_test.go`,
  `internal/gateway/qa_add_actor_references_test.go`; `vtt serve --ruleset`
  driven as a process): no defect. For the specification:
  - QA finding: the reasons' words, `reference "<id>" …`, with
    `SheetFault.Key` the reference id, and the empty id's
    `references: must not hold an empty id`.
  - QA finding: with no ruleset served, an empty ability id is refused
    before an empty reference id — SPEC-025's order.
  - QA finding: the schema's required `entryId` and `place` hold only with
    a ruleset served; with none, an empty entry or place is accepted
    (VTT-1041) — near-theoretical; the review then had the docs say so
    (under "What could not be established").
  - QA finding: place keys and states compare byte for byte — SPEC-028 to
    state it, as SPEC-027's case rule.
  - QA finding: a nil `Reference` handed in directly reads as empty fields
    and is refused as an entry the catalogue does not hold.
- Task 8 (`internal/adventure/qa_actor_references_test.go`,
  `internal/adventure/conformance/qa_references_dump_test.go`; injections
  by `go test -overlay`): no defect.
  - QA finding: the refusal of a reference written `null` had no row —
    VTT-1073, its tests tagged.
  - QA finding: a field written `null` reads as left out — an entry `""`
    the catalogue does not hold, a place `""` not declared, no state — the
    specification to state it.
  - QA finding: the `null` message says "giving its entry_id and place"
    while an absent `entry_id` gets the misfit message — absent reads as
    `""` — no change.
  - QA finding: `Run` accepts `"state": ""` as absent, but refuses
    `"references": {}` for an actor holding none while printing the same
    got and want — a diagnostics gap of conformance's diff,
    near-theoretical for goldens written by hand — no code.
  - QA finding: `Load` returns an empty, non-nil map — the specification.
- Task 9 (`internal/gateway/qa_reference_commands_test.go`, over real roles
  and both servers, clean under `-race`): no defect. For the specification,
  no rows:
  - QA finding: a copies refusal names the new or changed reference and
    the one holding the entry, whichever sorts first.
  - QA finding: the refusals read `gateway: <cmd>: actor "<id>":
    references: <reason>` and `reference_id: must not be empty`.
  - QA finding: an addition carrying no reference, with a ruleset served,
    is refused as `names entry ""`, before the fold's "requires a
    reference" — clear enough — no change.
  - QA finding: an addition under an id the sheet holds reaches the fold's
    "already holds".
  - QA finding: `change_reference`'s description does not name an entry
    the catalogue does not hold — reachable only for a sheet out of fit,
    which boot refuses — the review accepted it.
  - QA finding: a sheet left by two writers with two copies in a place that
    holds an entry once blocks no unrelated addition or change; a change of
    either copy kept in place is refused naming the other, and a removal is
    the remedy.
  - QA finding: no byte bound on a reference with no ruleset served — D10,
    as the ruling on an ability id of 2026-11-14; with a ruleset the loader
    bounds every entry id, place and state, so a longer one does not fit.
- Task 10 (`client/test/qa-reference-panel.test.ts`,
  `qa-reference-console.test.ts`; injections at fixtures, answers and the
  confirm): no defect. For SPEC-022, no rows:
  - QA finding: a reference's DOM, `.references > .reference`, drawn "Name
    (state)"; a title only from a non-empty description; an empty entry
    name counted as none, as a condition's.
  - QA finding: the Catalogue's data-fields and data-actions; actors and
    references by id; a blank id, entry or place refused in the console's
    words, the id and the state trimmed; a change row showing the present
    place and state; a removal fed by no field and clearing none; names
    from the window's lists, else ids or keys.
  - QA finding: the group drawn whenever the world holds an actor —
    narrowed at the review (the deviations).
  - QA finding: the removal question's words — under "What could not be
    established".
  - QA finding: the title (D17) and the listing per actor (D18) have no
    rows of their own — held by the tests and stated in the specification.
- VTT-1075 (`client/test/qa-reference-place-names.test.ts`): no defect. An
  empty place name draws by key, as none; a section listing one key twice
  is refused at load (VTT-1060); QA judged the implementer's changes to its
  own panel tests right.
- Task 12 (`internal/adventure/conformance/qa_shipped_references_test.go`,
  D21 held by a digest of a rendering derived by hand, no content word in
  Go): no defect.
  - QA finding: no record binds a place's words to "no digit", as VTT-850
    and VTT-1007 bind abilities' and entries' — the shipped places hold
    none and only QA's test holds it — left.
  - QA finding: that each reference's id is its entry's id is D21's choice,
    held by the digest, not a load rule — no row.
  - QA finding: "Equipment" last on "Character" is held by the test.
  - QA finding: SPEC-024's, SPEC-025's and SPEC-027's sentences gone stale
    — the specification task.

## The reading reviews

One reviewer per task; each Must and Should below was fixed in the task's
commit unless the entry names where it went: a later task, the
deviations, or "What could not be established".
- Task 0: the verifier's check, by an agent that did not write the ticket
  (passes, with gaps G1 to G12), then a reading review. Round one: M1, the
  requirements table said it was written under the recommended answers;
  M2, VTT-1031 had no table row and no task; S1, Tasks 3 and 4 could not
  commit through `client:typecheck` (`sheetFor`'s and `describe`'s
  exhaustive switches); S2, the Events panel's lines need a log walk for
  what a removal and a change do not carry; S3, VTT-1047 refused a legal
  change; S4, the fold's refusals in VTT-1046 and VTT-1047; S5, no row for
  no ruleset served in play (VTT-1061); S6, VTT-1019's clause given its own
  row (VTT-1060); S7, the word `places` against ADR-012 §10's "place", kept
  (under "What could not be established"); nits. Round two: the tools' "no
  player is told" read as a secret where the sheet shows it, now "no player
  is sent a message … whoever is sent the sheet sees it there"; an actor's
  id that no Events line gives, now the actor named as any actor is; D3's
  order sentence; nits.
- Task 1: no code defect. Round one: M1, VTT-1011 to VTT-1013 false as
  worded, withdrawn; S1, VTT-678 ambiguous, withdrawn; M2, `get_ruleset`'s
  "A number property's value is under an actor's `attributes`", false for a
  listed key, qualified, and "is a value of" became "names a value the
  form's entries hold"; S2, a descriptive helper comment deleted; S4, the
  conformance fixture (VTT-1066); S5, evidence cells: a caller-order test
  out of VTT-1010, QA's controls in, a redundant gateway test deleted since
  QA's holds VTT-675, and warnings reworded or deleted. Round two: QA's
  `get_ruleset` description test could no longer fail and left VTT-1014's
  cell; VTT-1066 reworded; the walk's cause, "is missing attribute", untrue
  for a declared key; rule-10 helper descriptions; the walk's roll, vs and
  effect entries unpinned, now tested by the walk's own cause. The smoke
  pass over every loadable tracked ruleset directory answered alike before
  and after.
- Task 2: M1, a QA closure without `t.Helper()`, which the pre-commit lint
  would refuse; S1, VTT-1022's and VTT-1018's cells cited QA tests that
  cannot hold them; S2, a QA probe test that asserted nothing, deleted; S3,
  Task 12 would have reddened the route's sheet helpers and QA's check that
  dnd45e-minimal declares no place, so the arms were added now; S4, the
  windows between Tasks 2, 3 and 10 confirmed (the deviations); nits.
- Task 3: no Must. S1, `get_ruleset`'s `copies` clause read as what a place
  holds now, now "true when it may hold …"; N1, "entries" where the format
  speaks of references; S2, `sheetFor`'s window until Task 10; N2, the
  implementer's VTT-1053 fixture already in key order, so QA's holds the
  row; N3, VTT-621's, VTT-1014's and VTT-831's cells. Every served body was
  byte for byte as before but `testdata/valid`'s section, which `db2ab40`
  misserved.
- Task 4: no Must. S1, the proto docs pointed at SPEC-025, which holds none
  of it, and now point at SPEC-028; S2, VTT-1023 to VTT-1026 say both
  folds and cited the server's tests only, so Task 5 added the client's to
  the same cells; S3 and N1, two client QA lists of every kind lacked the
  reference kinds, for Task 5; N2, `actorBelief`'s doc, for Task 6; N3,
  `add_actor`'s references doc says an empty id refuses the command; N4,
  two toolgen tests cite no row; N6, a warning where `Reference` is rebuilt
  field by field, "Copy every field of Reference here"; N7, SPEC-022's
  citations of VTT-809, for the specification task. It also set the order:
  Task 6 before Task 9.
- Task 5: M1, the dump-order defect had no recipe, now in
  `docs/verification-debt.md`; S1, the refusal words (the deviations); S2,
  the committed fixture did not pin byte order, and `byUTF8`'s length
  tiebreak and an empty `entry_id` survived, so the fixture grew; S3, QA's
  test of a prototype-member name overclaimed: protobuf-es decodes a map
  key `__proto__` onto a plain object and loses it, as for every map,
  near-theoretical, and the test's name narrowed; S4, SPEC-022's account of
  the dump, for the specification task; nits, among them N4, an empty entry
  or place with no ruleset served reads "now holds  in ", near-theoretical,
  and N6, the removal line's words kept.
- Task 6: M1 and M2, VTT-755 and VTT-1034 false as worded (the deviations);
  S1, VTT-1035's and VTT-1036's cells cited QA tests green under a fault in
  `Resume`, now out; S2, no walk reaches a correction that takes a
  reference off, recorded as open debt; S3, the eventgen band (the
  deviations); nits, and a list for SPEC-016 and SPEC-021.
- Task 7: no Must. S1, the copies rule had no row where a sheet enters
  (VTT-1070 to VTT-1072); S2, the schema's required `entryId` and `place`
  hold only with a ruleset served, and the docs now say "With no ruleset
  served it is not read." (option B; under "What could not be
  established"); S3, N1 and N3, `sheet.go`'s docs; N2, a fixture's doc. It
  also asked that the copies check in play name the new reference against
  the sheet's others.
- Task 8: no Must. S1, VTT-1073's cell lacked the tests that pin its place
  in the order; S2, VTT-1042 cited tests that cannot fail for it; S3, the
  shortened `ReferenceFault` doc read two ways, now "its entry, place and
  form read before its state"; N2, VTT-1073 reworded; pointers and tags.
  The reviewer probed that `adventure.Load`, `Compile`, `AppendBatch` and a
  reopen keep the references, and that `sheetsFit` agrees.
- Task 9: M1, the copies rule's two homes, now `Ruleset.CopyFault`; S1,
  `handleSheetChange`'s comment told the next editor to move
  `remove_reference` into it, reworded; S2, VTT-1051 cited QA behaviour
  tests that read no description; N1, `remove_reference`'s clause on a
  sheet out of fit dropped, and `change_reference`'s omission accepted;
  N3, the command-surface rows, for Task 10; N6, the override's test with no
  row, accepted; N7, `commandName`'s default chain, a candidate ticket
  outside this one (under "What was deliberately left out").
- Task 10: M1, VTT-1074 false for a row's Give (the deviations); S1,
  evidence cells; S2, `sheetFor`'s doc; S3, `choiceSelect`'s comment; S4
  and S5, the rows drawn and the ids shown (the deviations); nits; N7, the
  removal question's words not put to the owner; N9, the next full Stryker
  run will report Remove reference's `clears: []`.
- Task 12: round one: M1, the Equipment section named no place, ruled by
  the owner (VTT-1075); S1, the guide's sentence that a reference's place
  decides no roll; N1, "send the place it moves to"; N2, "two sheets may
  use one id", for the goblins' leather armour; N3, adventure-night's
  stream (the deviations). The reviewer probed that the alpha campaign
  serves under the new ruleset. Round two: clean; the hash tool's widening
  judged an input widened, not a gate weakened; nits, among them a case of
  references in only one place of a two-place section.

## The plan's gaps

- **G1.** Rule 9, completed; see "Rule 9".
- **G2.** The files reached are the plan's list and G2's, plus those under
  "Deviations": `contract/testdata/expected_tools.json` and
  `contract/gen/tools/tools.json` (Task 1),
  `internal/rules/conformance/conformance.go` and its tests (Task 1),
  `.go-arch-lint.yml` (Task 4), `client/src/style.css` (Task 10),
  `tools/ts-mutation-equivalents.txt` (Task 10),
  `tools/ts-mutation-inputs.py` and `tools/ts_mutation_inputs_qa_test.py`
  (Task 12), and the tests under "The existing tests that changed".
  `internal/harness/engine.go` and `rulesets/tavern-brawl/` are unchanged,
  as G2 said.
- **G3.** SPEC-021 moves; its reopen is held by VTT-1036.
- **G4.** D3: VTT-1017 refuses a place with no form or one form twice, and
  a state past the bound; a place may name no state (VTT-1015).
- **G5.** OD-1 A: no charges.
- **G6.** D13: an empty reference id is refused with a ruleset served or
  none (VTT-1040, VTT-1043, VTT-1061), as VTT-747 refuses an empty ability
  id.
- **G7.** The plan's P4: a copy of the alpha campaign's log, last written
  2026-11-15, folds to actors none of which holds a key a form lists.
  Task 12's reviewer found it serves under the new ruleset; none of its
  actors holds a reference.
- **G8.** OD-2 B, D22: `WatchableActor` gains no field (VTT-1031).
- **G9.** D5: VTT-825 and VTT-840 succeeded by VTT-1019 and VTT-1056.
- **G10.** D21, as built (item 9).
- **G11.** A place's key and its states are bounded at load (VTT-1016,
  VTT-1017, VTT-1022); the folds bound no length of a reference (D10, and
  QA's `TestQARFReferenceAddedBoundsNoLength`).
- **G12.** A `values` section naming a key a form lists loads and shows
  nothing; nothing shipped holds one.

## What the next full mutation run should find

The client code this ticket adds is `client/src/fold.ts`'s reference arms,
`withReferences`, `copyActor`'s references and the dump's `byUTF8`,
`Sorted` and `encode`; `client/src/view/naming.ts`'s `entry`, `place` and
`held`; `client/src/view/spectator.ts`'s `describe` arms and
`client/src/view/feed.ts`'s `STORY` entries; `client/src/player.ts`'s
section of places and `ShownReference.place`;
`client/src/view/player.ts`'s drawing of it; `client/src/view/dm.ts`'s
Catalogue group; `client/src/commands.ts`'s builders; and
`client/src/app.ts`'s hand-off of the catalogue and places. What `task
check` judged of the stored report is under item 10 (SPEC-023); a full run
is the owner's to ask for.

`tools/ts-mutation-equivalents.txt` was edited, and only where the code an
entry names changed: `byKey`'s entries left with `byKey` (the owner's
rider of 2026-11-23), and the namer's entry in `dm.ts` follows its code
line, which now passes the catalogue and the places, its reason naming the
console's new reads (SPEC-020). No survivor is answered ahead of a run
that names it. Known shapes: Remove reference's `clears: []` (an
`ArrayDeclaration`), as Delete note's and Remove condition's (Task 10's
review, N9); and a map written in place in a client fold arm, which stayed
green as a break since `fold()` is `apply`'s only caller and the log is
folded afresh, so a mutant of that shape that survives does so for that
reason.

On the Go side, the new branches in `internal/rules`,
`internal/rules/conformance`, `internal/gateway`, `internal/engine`,
`internal/perceive`, `internal/adventure` and `internal/adventure/conformance`
are in the mutation gate's packages (`tools/check-mutation.py`'s `PACKAGES`);
`internal/mcp` gained a description's sentences and `internal/campaign` tests
only; `internal/eventgen`, `cmd/vtt` and `tools/toolgen` are not in them.
`formKeyFault`, `checkSection` and `handleRuleset`'s section kind are written
as ifs; the 5a report records why a `switch`'s case conditions go unmeasured.
`tools/mutation-equivalents.txt` is not edited. What the gate found on
`9271f34` is under item 10.

## What could not be established

- **Decided without the owner, to put to him:**
  - The removal question's words: "Remove ⟨id⟩: ⟨entry⟩ in ⟨place⟩ from
    ⟨actor⟩? Its id and state go with it." Each asks-first ticket before
    it put its words to the owner as an open decision (Task 10's review,
    N7).
  - An empty entry or place, with no ruleset served, is not read: option
    B, chosen at Task 7's review, held by VTT-1041 and VTT-1061, and the
    tools say "With no ruleset served it is not read." Option A was to
    refuse it always, as VTT-747 refuses an empty ability id.
  - The format keeps the word `places` while ADR-012 §10 calls a
    contribution's target a place; from this ticket the word is permanent
    in `ruleset.json` and on the wire (rule 3), so §10's is renamed at step
    2 (D1; Task 0's review, S7).
  - The console's Catalogue group sits before "Remove condition" (the
    deviations).
  - The Catalogue draws an actor's row only where there is something to
    show or do, and shows each reference's id in its row and in the
    removal question (the deviations).
  - VTT-1074, the owner's rider of 2026-11-23, was reworded in place to
    "clears no field of another of the console's forms" (Task 10's review,
    M1).
- **D17 is narrower than the code.** It says a reference is shown by its
  entry's name and its state; since VTT-1075 a section listing more than
  one place also names each line's place. SPEC-022 is to state the code's
  reading.
- **SPEC-028 does not exist at `ac618a7`.** `events.proto`,
  `commands.proto`, `client/src/state.ts`, `client/src/commands.ts` and
  `internal/gateway/reference_change.go` point at it. The specifications
  still cite withdrawn rows: SPEC-016 VTT-755; SPEC-018 VTT-1009; SPEC-022
  VTT-809 and VTT-840; SPEC-024 VTT-996, VTT-825 and VTT-1009; SPEC-025
  VTT-678; SPEC-027 VTT-996 (`git grep -n -E
  'VTT-(996|825|1009|809|840|678|755)([^0-9]|$)' ac618a7 --
  docs/specifications`). Sentences gone false: SPEC-024's "Where a value
  lives", that a sheet may hold a form's property; SPEC-027's consequence
  "A property a form lists is a key a sheet may hold", its "No sheet refers
  to an entry", and its search `\.(Catalogue|Forms)`, which now also prints
  `formKeyFault` and `ReferenceFault`; SPEC-025's order of a sheet's
  refusals; SPEC-013's step 4, "nothing for any other command", since
  `validateForm` reads an `add_reference`, and its step 6's dispatch list,
  which lacks `add_reference` and `change_reference`, which
  `handleSheetChange` dispatches beside the give and the take, while its
  "Each of those handlers appends a whole batch through
  campaign.AppendBatch" must not take in `handleReferenceChange`, which
  calls `campaign.Append`; SPEC-022's "JSON.stringify leaves out each that is
  absent", now `encode`. All resolve with the specification task that
  follows this report.
- **No generated walk reaches a correction that takes a reference off.**
  Recorded in `docs/verification-debt.md` ("No generated walk reaches a
  correction that takes a reference off"), with its recipe; the same shape
  leaves `AbilityTaken` and `ResourceRemoved` out of every walk's
  corrections.
- **QA's test pins D21's content.** `TestQASRTheContentIsD21sAsWritten`
  holds the shipped places and references against a digest derived from
  D21, so any change to that content reddens it. 5a cut its QA's pins of
  content under the owner's ruling of 2026-11-24; whether this one is to
  stand is put to the owner with this report.
- **No row holds D15's override.** `vtt.v1.Reference`'s required `entryId`
  and `place` are pinned by
  `TestAReferenceRequiresItsEntryAndItsPlaceAndNotItsState` and
  `TestTheReferenceToolsRequireWhatTheyCannotDoWithout`, which cite no id;
  the reviews of Tasks 4 and 9 accepted it. Nor does a row hold
  `add_actor`'s sentences on what a reference is,
  `TestAddActorSaysWhatASheetsReferencesAre`.
- **The alpha campaign.** `~/dev/vtt-alpha`, outside the repository, serves
  under the new ruleset (G7). When it is served from a tree holding this
  change is the owner's to decide.
- **MapTool was not read again for this report.** "Rule 9" rests on the
  plan's P15 at RPTool `63862e4` and MapTool `f4b7fef6c`.

## What was deliberately left out

- **The ticket's "Not this ticket":** the actions a sheet reaches through
  its references, and the abilities that bind a weapon's numbers today,
  which stay as they are (5c); a limit on what a place holds, weight
  against strength among them (step 1 item 6); a player changing what
  their character carries, and equipping as an action with a cost (step 2,
  ADR-012 §9); a set's condition on its entries, its budget and values per
  entry (ADR-012 §2); charges, and an item destroyed at a bound (step 2);
  telling a player that their character gained or lost an item.
- **By the owner's rulings:** no charges (OD-1 A); no message of a change
  to any player and no field on the watchable set (OD-2 B, D22, with Task
  11); no visibility per property or reference (2026-11-06); no limit on a
  place (2026-11-25); no migration (2026-09-04).
- **Unchanged (D20):** `rulesets/tavern-brawl/`; `internal/harness/`;
  SPEC-026's writers, since no reference command writes a `RulesetNamed`
  (VTT-1050); ADR-012 §2's `ref`; dnd45e-minimal's `"version": "1"`, since
  no ability changed.
- **Near-theoretical, with no code, test or debt entry (the ruling of
  2026-10-21):** two additions of one entry to a place that holds it once,
  each judged before the other lands, which the next `vtt serve --ruleset`
  refuses and a removal remedies; a nil value in an `ActorAdded`'s
  references map; a map key `__proto__`, lost by protobuf-es's decoder as
  in every map; an empty entry or place with no ruleset served, drawn in
  the Events panel as "now holds  in "; a threshold or a
  `default_max_expr` reading a listed key; conformance's diff printing the
  same got and want for `"references": {}` and none.
- **Known and not findings (the plan's "For Phase 4a"):** an adventure file
  writing one reference id twice keeps the last, as for an attribute (the
  plan's P3); a `values` section naming a key a form lists loads and shows
  nothing (G12); an expression reading such a key fails at use, as for any
  key a sheet does not hold (D6).
- **Prose 5c must rewrite.** `guide.md` says that no ability reads a
  reference, and `ruleset.json`'s weapon properties that no power reads
  them yet. Each is true at `ac618a7` and goes false when an ability reads
  its weapon; the ticket that makes it false rewrites it.
- **A candidate ticket:** `commandName`'s default chain, which a one-line
  name from the oneof would end, and with it the gocyclo pressure that
  moved the dispatch into `handleSheetChange` (Task 9's review, N7). This
  report is its only record, and it is put to the owner with the report.
- **No gate work (paused 2026-08-27):** eventgen walks that hide a seen
  party member and then take what its sheet holds.
- **The client's full mutation run**, the owner's to ask for.
- **The specifications:** SPEC-028, new, and SPEC-007, SPEC-012, SPEC-013,
  SPEC-016, SPEC-018, SPEC-021, SPEC-022, SPEC-023, SPEC-024, SPEC-025 and
  SPEC-027,
  in the specification task that follows this report (the plan's Task 14).

