# A ruleset declares forms and a catalogue of entries — report

Ticket: `docs/superpowers/specs/2026-11-24-a-ruleset-declares-forms-and-a-catalogue-design.md`.
Plan: `docs/superpowers/plans/2026-11-24-a-ruleset-declares-forms-and-a-catalogue.md`.
Branch `feat/forms-and-the-catalogue` from main `e0bf9ef`. Commits:
`a396c4f` (Task 0: ticket, plan, register), `7495669` (Task 1: forms),
`dab4b54` (Task 2: catalogue entries and one id namespace), `0adcde5` (Task
3: `extends`), `22e1b17` (Task 4: the read surface), `ecd7d72` (Task 5:
dnd45e-minimal's catalogue), `0dbbee1` (the comment ledger) and `214a950`
(`resolveExtends`'s link refusals written as ifs; the last code commit).
Every code reference below is to `214a950`.

This is the first of the three tickets the owner split ADR-012 step 1 item 5
into on 2026-11-24. The working notes, and this report, call them 5a (this
one: forms and catalogue entries in the ruleset format, no sheet changed),
5b (a sheet's references, slots and sets, with the campaign's state per
reference) and 5c (the actions a sheet reaches through its references, in
place of `Actor.ability_ids`). SPEC-027, new, is written from this report;
SPEC-024, SPEC-012, SPEC-018 and, under OD-2 A, SPEC-025 move with it (the
plan's Task 7), and so does SPEC-026, whose Requirements list names VTT-973,
which VTT-996 succeeds. SPEC-022 does not move: under OD-1 A no property type is
added, so the client's `Property.type` does not widen.

## Done looks like

1. [x] **A ruleset declares forms.** `manifestJSON`
   (`internal/rules/load.go`) reads `forms`; `loadForms` checks each form
   against the properties declared beside it; `checkManifestBounds` bounds
   its key at `maxIDBytes` (SPEC-018); `loadV2` copies them, in declared
   order, to `Ruleset.Forms` (`internal/rules/format.go`, `Form`).
   - Kept as written, in order, and none when absent, `null` or `[]`:
     `internal/rules/forms_test.go`'s
     `TestARulesetKeepsItsFormsInDeclaredOrder`, `TestTheFixtureKeepsItsForm`
     and `TestARulesetDeclaringNoFormsLoadsWithNone` (VTT-993).
   - Refused, naming `ruleset.json` and `forms[<i>].<field>`: no key, name or
     description, `TestAFormIsRefusedWithoutItsKeyNameOrDescription`; a key
     an earlier form declares, `TestAFormKeyAnEarlierFormDeclaresIsRefused`;
     a key past the bound, `TestAFormKeyPastTheIDBoundIsRefused`, and one at
     it loads, `TestAFormKeyAtTheIDBoundLoads` (VTT-994).
   - Refused under `forms[<i>].properties[<j>]`: a key no property declares,
     a key listed twice, and, under OD-1 A, a property that is not a
     `number`: `TestAFormListingAPropertyItMayNotCarryIsRefused` (VTT-995).
   - A field a form does not define is refused,
     `TestAFieldAFormDoesNotDefineIsRefused`, and a case variant of `forms`
     is read as it, `TestACaseVariantOfFormsIsReadAsForms` (VTT-996).
   - The schema: `TestTheSchemaDescribesEveryFormField` (VTT-1008); the
     key's bound, stated in its description, by the form-key rows of
     `TestTheSchemasStateTheIDBound` and
     `TestQAPFSchemasStateTheBoundsLoadEnforces` (VTT-1009).
     `testdata/valid/ruleset.json` declares a form (D13), so
     `TestSchemaPropertiesCoverFixtureFields` and
     `TestSchemaRequiredFieldsMatchLoaderEnforcement` read it; the latter
     runs, rather than skips, a case for each of a form's required fields:
     `go test ./internal/rules/ -run
     'TestSchemaRequiredFieldsMatchLoaderEnforcement$' -v`.
   - Independently: `internal/rules/qa_forms_test.go`.
   - On `e0bf9ef` the new tests do not build, since `Ruleset` has no
     `Forms`. The refusal they replace was measured there by the plan's P1:
     `json: unknown field "forms"`, for both shipped rulesets and
     `testdata/valid`.
2. [x] **A ruleset holds a catalogue of entries.** `loadCatalogue` reads
   each `catalogue/*.json` through `jsonFilesIn` and `decodeStrict`, one
   entry per file (OD-6 A), and checks its values with `entryValues`;
   `loadV2` calls it after `checkSheet`, then `checkOneNamespace`, and
   keeps the result as `Ruleset.Catalogue`, keyed by id (`Entry`).
   - Kept by id with its name, description, form and values, and none
     without a `catalogue/`: `internal/rules/catalogue_test.go`'s
     `TestARulesetKeepsEveryCatalogueEntryByItsID` and
     `TestARulesetWithNoCatalogueLoadsWithNone` (VTT-997).
   - Refused, naming the entry's file and the field: no id, name,
     description or form,
     `TestAnEntryIsRefusedWithoutItsIDNameDescriptionOrForm`; an id past
     the bound, `TestAnEntryIDPastTheIDBoundIsRefused`, and one at it
     loads, `TestAnEntryIDAtTheIDBoundLoads`; a form no form declares,
     `TestAnEntryNamingAFormNoFormDeclaresIsRefused`; an id an earlier file
     declares, `TestAnEntryIDAnEarlierFileDeclaresIsRefused`; a field the
     entry format does not define, `TestAFieldAnEntryDoesNotDefineIsRefused`,
     and a case variant of a defined one read as it,
     `TestACaseVariantOfAnEntryFieldIsReadAsIt` (VTT-998).
   - Refused under `values.<key>`: a property the entry's form does not
     list, `TestAnEntryValueItsFormDoesNotListIsRefused`, the first by key
     when there are several,
     `TestAnEntryHoldingSeveralUnlistedValuesIsRefusedOnTheFirstByKey`; a
     value written `null` or not an integer within an `int32`,
     `TestAnEntryValueThatIsNotAnInt32IsRefused`, and each end of an `int32`
     loads, `TestAnEntryValueAtEachEndOfAnInt32Loads` (VTT-999). Under OD-1
     A a form lists `number` properties alone, so "a value that is not of
     its property's type" is a value that is not such an integer.
   - An entry need not carry every property its form lists (D5), and holds
     no value for one it does not carry:
     `TestAnEntryCarryingNoValueForAListedPropertyHoldsNone` (VTT-1000).
   - One id names one thing across entries, abilities and conditions (OD-3
     A), refused naming both files and `id`:
     `TestAnIDTwoOfAnEntryAnAbilityAndAConditionDeclareIsRefused`
     (VTT-1001). The check reaches a ruleset with no catalogue too, an
     ability and a condition sharing an id; every tracked `ruleset.json`
     loaded alike before and after it (Task 2's overlay probe; the plan's
     P6).
   - The schema: `internal/rules/schema/entry.schema.json`, held by
     `TestTheSchemaDescribesEveryEntryField` (VTT-1008) and the entry-id rows
     of `TestTheSchemasStateTheIDBound` and
     `TestQAPFSchemasStateTheBoundsLoadEnforces` (VTT-1009).
     `TestSchemaRequiredFieldsMatchLoaderEnforcement` runs a case for each
     of an entry's required fields, as for a form's (item 1).
   - Independently: `internal/rules/qa_catalogue_test.go`.
   - On `e0bf9ef` the new tests do not build (no `Catalogue`, no `Entry`);
     the plan's P2 measured there that a `catalogue/` holding a file that
     is not JSON loads, since nothing read the directory.
3. [x] **An entry may extend another.** `resolveExtends`, called at the end
   of `loadCatalogue`, checks every link in id order, then walks each
   entry's chain with a visited set, the nearest value winning; `cycleErr`
   names a cycle.
   - The values: `TestAnExtendingEntryHoldsEveryValueItDoesNotSetItself`,
     `TestAnEntryHoldsValuesThroughAChainOfExtends`,
     `TestAnEntrysValuesDoNotDependOnTheOrderOfItsFilesOrIDs`,
     `TestAnEntryExtendingOneThatHoldsNoValueForAPropertyHoldsNone` and
     `TestTheFixturesExtendingEntryHoldsTheValuesOfTheOneItExtends`
     (VTT-1002, and VTT-997's resolved values).
   - Refused on the field `extends`, naming the extending entry's file and
     the id it names: an entry the catalogue does not hold,
     `TestAnExtendsNamingAnEntryTheCatalogueDoesNotHoldIsRefused`; and,
     beyond the item, `extends` written empty,
     `TestAnExtendsWrittenEmptyIsRefused`, and an entry of another form
     (OD-5 A), `TestAnExtendsNamingAnEntryOfAnotherFormIsRefused`
     (VTT-1003).
   - A cycle, on the file of its lowest id, naming every entry in it in id
     order: `TestACycleOfExtendsIsRefusedNamingEveryEntryInIt`, its cases
     "of one", "of two", "of three" and "reached from an entry outside it",
     each loading in a goroutine with a deadline, so a walk that does not
     end fails rather than hangs (VTT-1003).
   - Independently: `internal/rules/qa_extends_test.go`, among them
     `TestQAEXRefusesACycle`, `TestQAEXRefusesAnUnresolvableExtends` and
     `TestQAEXIndependentOfFileNamesAndReadOrder`, which loads the catalogue
     under every order of its file names.
   - On `e0bf9ef` these do not build; nothing read `extends` there.
4. [x] **dnd45e-minimal has a catalogue.**
   `rulesets/dnd45e-minimal/ruleset.json` declares the `number` properties
   `proficiency`, `damage_dice`, `damage_die`, `range` and `armor_bonus`,
   and the forms `weapon` (the first four) and `armor` (the last);
   `rulesets/dnd45e-minimal/catalogue/` holds `longsword`, `crossbow`,
   `scimitar`, `shortbow`, `chainmail` and `leather-armor` (D11).
   - Each weapon entry carries the numbers its ability binds, the plan's P7:
     the `prof` of `weapon-attack-roll` as `proficiency`; a damage of `1dN`
     as `damage_dice` 1 and `damage_die` N (OD-1 A); and, on the crossbow
     and the shortbow alone, `ranged-delivery`'s `distance` as `range`. The
     scimitar's `+ 2` stays the goblin's, in `goblin-scimitar`: it is not a
     number of the weapon. A reader compares them by command,
     `git grep -n -E '"prof"|damage_expr|"distance"' 214a950 --
     rulesets/dnd45e-minimal/abilities/`, beside each entry's file. No kept
     test holds them equal (OD-7 B); Task 5's QA compared every weapon
     entry with its ability once, green, and did not keep the probe.
   - The armour entries' numbers are the author's content: no ability binds
     one, and the owner ruled on 2026-11-24 that equipment's values are free
     (the Phase 4a adjudications).
   - Its abilities are unchanged: `git diff --stat e0bf9ef..214a950 --
     rulesets/ scenarios/ contract/ cmd/` names only dnd45e-minimal's
     `ruleset.json`, `guide.md` and new `catalogue/` files, and the
     `ruleset.json` diff only adds properties and forms, so its abilities,
     atoms, conditions, sheet and version stand as they were;
     `git diff --stat e0bf9ef..214a950` names no golden. Each weapon's
     numbers therefore stand in its ability and in its entry until 5c makes
     an ability read its weapon.
   - The conformance suite loads it and every shipped ruleset:
     `internal/rules/conformance`'s `TestConformanceOverRulesetsGlob`, a
     subtest per ruleset under `rulesets/`; an entry naming an undeclared
     form turned it red (`ecd7d72`'s break). Each entry's description holds
     no digit:
     `TestEveryShippedCatalogueEntryCarriesADescriptionWithNoDigit`
     (VTT-1007, D12).
   - Independently: `internal/rules/conformance`'s
     `qa_shipped_catalogue_test.go`, its
     `TestQASCShippedEntriesLoadAsWritten` (VTT-997) and
     `TestQASCShippedEntryDescriptionsHoldNoDigit` (VTT-1007).
5. [x] **The catalogue is served.** `rulesetJSON`
   (`internal/gateway/metadata.go`) gains `forms` (`formJSON`, in declared
   order) and `catalogue` (`entryJSON`, sorted by id), both started as
   empty slices; `handleRuleset` fills them for every role (OD-4 A), each
   entry's `values` as `rules.Load` resolved them, a form listing none
   answering `"properties": []` and an entry carrying none `"values": {}`.
   `extends` is not served (D8).
   - The forms, to every role in `qaRRRoles` (the DM, the agent, a player
     and a spectator): `internal/gateway/catalogue_route_test.go`'s
     `TestEveryRoleReadsTheFormsInTheirDeclaredOrder` (VTT-1004), over forms
     declared out of key order, so a sort fails it.
   - The catalogue, sorted and resolved:
     `TestEveryRoleReadsTheCatalogueSortedByIDWithWhatEachExtends`
     (VTT-1005), over entries written in descending id order and a chain of
     `extends`; removing the sort turned it red on every run of the break
     (`22e1b17`'s message).
   - `[]`, never `null`, with none declared and with no ruleset:
     `TestNoFormsAndNoCatalogueAreEmptyListsForEveryRole` (VTT-1004,
     VTT-1005).
   - In the ruleset's words, for each shipped ruleset: `qaRRCheckForms` and
     `qaRRCheckCatalogue`, called by `qa_ruleset_route_test.go`'s
     `TestQARRRulesetRouteAnswersEveryRole`, compare the answer with the
     loaded ruleset; `TestQARRNothingLoadedAnswersEmptyLists` checks both
     lists empty.
   - `get_ruleset` returns the route's body verbatim (VTT-621):
     `internal/mcp/ruleset_tool_test.go`'s
     `TestGetRulesetReturnsTheRulesetRouteVerbatim`, whose answer now holds
     a form and an entry, and `qa_ruleset_catalogue_test.go`'s
     `TestQACRGetRulesetReturnsTheRoutesBodyVerbatim`. Its description says
     so (principle 34): `getRulesetDescription`'s new sentence, held by
     `TestGetRulesetSaysItAnswersEveryRoleTheFormsAndTheCatalogue`
     (VTT-1006).
   - The client types them: `client/src/metadata.ts`'s `FormDef`,
     `CatalogueEntry` and `RulesetMeta`'s `forms` and `catalogue` (D10);
     nothing in `client/src` reads them yet.
   - Independently: `internal/gateway/qa_catalogue_route_test.go`, among
     them `TestQACRTheWireNamesEachFieldExactly`, and
     `internal/mcp/qa_ruleset_catalogue_test.go`.
   - On `e0bf9ef` the gateway tests do not build (no `rules.Form`), and
     `rulesetJSON` had no field to answer either list.
6. [x] **The gate.** `task check` and `task e2e` are green on `214a950`.
   - `task check` passed on `0dbbee1` first; `214a950` followed it and
     `task check` passed again on it, with `go clean -cache` before and
     after each run.
   - `task e2e` passed on `214a950`, but for the one test the suite skips by
     `test.skip`, `handover.spec.ts`'s "a network drop while the table plays
     on ends in a Reconnect", skipped since `bafaa27` and not touched here.
   - The client's stored mutation report is out of date: `task check`'s
     `check:ts-mutation` judged nothing, named `d1ff728` as the commit it
     measured and this ticket among those added since, and passed
     (SPEC-023). A full run, `task check:ts-mutation:full`, is the owner's
     to ask for.
   - The comment ledger: `python3 tools/check-comments.py --write-ledger`
     ran once, in `0dbbee1`, after Task 5's commit. Every row it changed
     is lowered or belongs to a file this ticket adds, and none is raised:
     `git show 0dbbee1 -- tools/comment-ceilings.txt`. `214a950` adds one
     comment line to `internal/rules/load.go`, which raises that file's
     share and so lowers no row; `check:comments` judged it within the
     file's row.
   - The bundle: `client/src/metadata.ts` gains type declarations only;
     `22e1b17`'s message records the bundle unchanged, and
     `git diff --stat e0bf9ef..214a950` names nothing under
     `cmd/vtt/webdist`.

## What the rules became

The ticket's rules, each with what it became:
- A ruleset's forms each have a key, a display name and a description, and
  list properties the ruleset declares: VTT-993 (kept as written, in
  order), VTT-994 (key, name and description; the key's bound; a key two
  forms declare), VTT-995 (a key no property declares, listed twice, or,
  under OD-1 A, not a `number`), VTT-996 (a field a form does not define,
  succeeding VTT-973), VTT-1008 (the schema describes it) and VTT-1009 (the
  schema states the key's bound, succeeding VTT-625).
- A catalogue entry has an id within the bound, a name, a description and a
  declared form, its ids distinct: VTT-997 (kept), VTT-998 (each refusal),
  VTT-1001 (one id namespace across entries, abilities and conditions,
  OD-3 A), VTT-1008 and VTT-1009.
- An entry carries values only for properties its form lists, each of its
  type: VTT-999. VTT-1000 holds D5: an entry need not carry every one.
- An entry that extends another holds its values except those it sets:
  VTT-1002; the refusals, an empty `extends` and one of another form (OD-5
  A) among them, VTT-1003.
- `/api/ruleset` and `get_ruleset` answer the forms and the catalogue:
  VTT-1004 and VTT-1005 for the route. `get_ruleset`'s half was refused at
  the sort, held by VTT-621 (its body is the route's, verbatim), which
  gained `TestQACRGetRulesetReturnsTheRoutesBodyVerbatim`; VTT-1006 holds
  its description.

The plan's labels:

| Plan | Id | |
|---|---|---|
| R1 | VTT-993 | forms kept as written, in order |
| R2 | VTT-994 | a form's key, name, description; the key's bound |
| R3 | VTT-995 | a form's properties |
| R4 | VTT-996 | the defined fields with `forms`; succeeds VTT-973 |
| R5 | VTT-997 | entries kept by id, values resolved |
| R6 | VTT-998 | an entry's refusals |
| R7 | VTT-999 | an entry's values |
| R8 | VTT-1000 | an unset value held as none |
| R9 | VTT-1001 | one id namespace |
| R10 | VTT-1002 | `extends` gives its values |
| R11 | VTT-1003 | `extends`' refusals, the cycle among them |
| R12 | VTT-1004 | the route's `forms` |
| R13 | VTT-1005 | the route's `catalogue` |
| R14 | VTT-1006 | `get_ruleset`'s description |
| R15 | VTT-1007 | a shipped entry's description holds no digit |
| R16 | VTT-1008 | the schemas describe forms and entries |

Added by the sort: VTT-1009, which succeeds VTT-625 and names a form's key
among the bounds the schemas state.

Withdrawn on 2026-11-24: VTT-973 for VTT-996 (Task 1), false as worded
once `rules.Load` reads `forms`; VTT-625 for VTT-1009 (Task 2), true but
narrower than its tests once they probe a form's key. Rows kept that
gained evidence: VTT-581
(`TestQACRARequestWithoutAnAcceptedTokenGetsNoCatalogue`), VTT-621 (above)
and VTT-850 (`TestQASCShippedAbilityDescriptionsHoldNoDigit`).

Refused at the sort, each held elsewhere: `get_ruleset` answering the two
lists (VTT-621); the conformance suite loading every shipped ruleset
(`TestConformanceOverRulesetsGlob`); dnd45e-minimal's abilities unchanged
(its compiled goldens); the weapon entries carrying their abilities'
numbers (OD-7 B: nothing now, the compiled goldens once 5c reads them).

The open decisions, as the owner signed them off on 2026-11-24: OD-1 A, an
entry carries `number` values alone; OD-3 A, one id namespace; OD-4 A,
every role is served the forms and the catalogue; OD-5 A, no `extends`
across forms; OD-6 A, one file per entry under `catalogue/`; OD-7 B,
nothing holds a weapon's two copies of its numbers equal; with D5, D11 and
D16 ("Godkänd", approved). OD-2 A, a form lists keys of the one
`properties` vocabulary, came after the owner asked whether it was a
shortcut: "Bygger vi nu en 'fuskväg' för att vi inte har utrustning/vapen
hantering?" (are we now building a shortcut because we have no handling of
equipment and weapons?). Told that a property a form lists is a key a sheet
may hold until 5b, and offered closing that now by making the sheet a form,
he chose "Godta hålet till 5b" (accept the hole until 5b). The hole is
under "What could not be established".

## Rule 9

The ticket records it, with the verifier's G1 corrections taken in before
Task 0's commit, measured as the plan's P10 at RPTool `63862e4` and MapTool
`f4b7fef6c`. MapTool's core has no item and no catalogue: a campaign
declares token types (`CampaignProperties.tokenTypeMap`), each a list of
properties with a visibility (`TokenProperty`'s `ownerOnly` and `gmOnly`),
and a value is a string on the token. The owner's rule framework keeps a
weapon as an object on the token (`EquippedWeapon`), which the attack
templates read, and lets a template `extends` a list of templates
(`templates/FireDamage.json` extends `ImplementDamage`).

Its two resolvers differ, and Task 0's review asked that they be kept
apart: the library's `rule-framework/library/mtscript/public/lib/TR_Visit.mts`
marks a template visited before it
walks its `extends`, so a cycle ends the walk silently, neither refused nor
reported; `rule-framework/src/TemplateResolver.ts`'s `resolveAbility` marks
it only after its parents, so a cycle recurses without end. Both resolve
when an ability is used (the ticket, P10), so an unknown template is found
only then.

Borrowed: a weapon's numbers as named values on the thing itself; a child's
own value over its parent's. Not borrowed: the copy on each token (an entry
lives once, in the ruleset, principle 27); a list of parents (ADR-012 §3:
an entry "may extend another"); a fault found at use, and a cycle ended
silently or not at all (each refused at load, naming the entries,
principle 25). Weighed and left to the owner: the property list per token
type (OD-2 B), the visibility per property (OD-4 C), the damage die as
text (OD-1 B).

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as verified | Revised before Task 0's commit to take in G1 to G9; the plan's verdict, checks and gaps describe the draft that was verified, and its status says so | The verifier found the gaps. Task 0's review then made VTT-1000 agree with VTT-1002 for an entry that carries no value but extends one that sets it (M1), worded VTT-997's values as resolved (S1), added a test of an entry's undefined field and a test inside a form's schema to the plan (S2, S3), and had item 4 name the numbers (S4) |
| D1: `manifestJSON.Forms` a `*[]formJSON` | A plain `[]formJSON`; absent, `null` and `[]` each load as no forms | VTT-993 makes all three alike, so nothing needs them told apart, unlike the sheet's `[]`, which VTT-825 tells from no sheet. Task 1's review asked that the difference be reported (N5) |
| D2: a form's `properties` an optional list of property keys | The schema's form `properties` also carries `uniqueItems: true` | The schema then refuses the list the loader refuses as listing a key twice (VTT-995) |
| Task 1's files | `internal/rules/load_test.go` changed: `TestLoadValidFixture` pins the fixture's property keys, which gained `heft`; `numberAt` now edits the decoded JSON | `numberAt` inserted a property by replacing every `"properties": [`, which the fixture's form list now also opens, so `TestARulesetWhoseIDOrNameExceedsTheBoundIsRefused` (its "number key" case), `TestARulesetWhoseIDsAndNamesAreExactlyTheBoundLoads` and `resolve_test.go`'s `TestARulesetAtTheBoundResolvesToEventsTheFoldAccepts` failed to decode. The plan's list for Task 1 named the tests that pin the schema's field list, not those that pin the fixture's property list or edit its text. Task 1's review corrected the implementer's list of the tests broken (N7) |
| D13: the fixture's entries, over a property only its form lists, "so no test that edits the fixture's other properties meets a new refusal first" | The fixture's `catalogue/` broke tests that copy `testdata/valid` and write their own `ruleset.json`: `formsManifest` (`forms_test.go`), `propertyRuleset` through `writeProperties` (`property_test.go`) and `TestLoadAcceptsThresholdExplicitFalse`. Each now calls `dropCatalogue` after the copy; no assertion changed | D13 guarded edits of the properties, not a manifest written anew: one written without the fixture's `crate` form leaves its entries naming a form nobody declares, and the load is refused before the case's own check |
| Task 3's files: `load.go` and `catalogue_test.go` | `entry.schema.json`'s `values` description and `Entry.Values`' doc comment were rewritten; VTT-1000 gained `TestAnEntryExtendingOneThatHoldsNoValueForAPropertyHoldsNone` | Both said, from Task 2, that an entry's values are its own; once `extends` resolves, they hold inherited ones too. VTT-1000's "and extends no entry holding one" needed the case of an entry extending one that holds none |
| D7: "The walk keeps a visited set, so a cycle ends it" | The walk keeps it, and `cycleErr`, which collects the cycle's members, bounds its own loop by the number of links: `for range len(extends)`, with a `break` when it returns to `at` | Task 3's review (Must): the loop first ran `for next := extends[at]; next != at; …`, and gremlins' negation of `!=` made a self-extending entry append to the cycle without end; a test failing at its deadline leaves its goroutine running, so a mutation run's memory grew without bound. The implementer had already preferred the visited set to a step counter, whose mutant could hand `cycleErr` an entry outside any cycle |
| Task 4's break "serve an entry's own values" | Made in `resolveExtends`, not in the gateway | `rules.Load` keeps no entry's own values once resolved, and no `extends` (D8), so `handleRuleset` holds nothing unresolved to serve |
| D12: a test "beside" VTT-850's | `TestEveryShippedCatalogueEntryCarriesADescriptionWithNoDigit` and `TestEveryShippedAbilityCarriesADescriptionWithNoDigit` share `checkShippedDescriptions` | The two checks differ only in the directory they read; one helper, not a copy. A digit put into an ability's description after the change still turned VTT-850's test red (`ecd7d72`'s break) |
| Task 5's files: `ruleset.json`, `catalogue/`, `conformance_test.go` | `rulesets/dnd45e-minimal/guide.md` changed too | `get_ruleset_guide` serves it to the agent, and two of its sentences went false or misleading: "there is no separate equipment layer in this slice", and that a number property goes in `attributes`, now that a weapon's and an armor's numbers are number properties a form lists. It now says the catalogue lists the weapons and armor, that no sheet refers to an entry and no ability reads one, and never to set a form's property on a creature. Task 5's review (S2) renamed the goblin's `+ 2` from "baked-in proficiency", now the weapon's property, to "baked-in damage bonus, not a stat and not its scimitar's" |
| The plan's commits: the comment ledger after the last code commit | A code commit, `214a950`, after the ledger's `0dbbee1`: `resolveExtends`'s refusals of an empty `extends`, of one the catalogue does not hold and of one of another form were a `switch`'s case conditions from Task 3, and are now three ifs, in the same order with the same text, under a one-line warning to keep them ifs | Go's cover profile counts a case from its body, so gremlins scored every mutant of those conditions NOT COVERED and ran none (`tools/mutation-scope.md`). `loadForms` is written with ifs; Task 3's review did not read for it, and writing this report is what surfaced it. Each refusal had its own test throughout, red under Task 3's breaks and under `214a950`'s; what was missing was the gate's measure of them. No ledger row moved: the added comment line raises `load.go`'s share |
| D11: the armour named, its numbers left open (G5) | `chainmail` and `leather-armor` each carry an `armor_bonus` the implementer took from 4e's armour bonuses; the owner was told at Task 5's summary | No ability binds an armour number, so there was none to copy. The owner's ruling of 2026-11-24 makes equipment's values the author's to set (the Phase 4a adjudications) |

## The deliberate breaks

Each applied and restored, and red unless said otherwise:
- Task 1 (`7495669`): each refusal dropped in turn (its case of the forms
  tests); `loadV2`'s copy dropped, and the forms copied reversed
  (`TestARulesetKeepsItsFormsInDeclaredOrder`); a resource accepted in a
  form (the resource case); the bound dropped from `checkManifestBounds`
  (`TestAFormKeyPastTheIDBoundIsRefused` and the form-key probe of
  `TestQAPFSchemasStateTheBoundsLoadEnforces`); the bound dropped from the
  schema (`TestTheSchemasStateTheIDBound`,
  `TestTheSchemaDescribesEveryFormField`); `properties` deleted from the
  schema's form item (`TestTheSchemaDescribesEveryFormField`); `forms`
  deleted from the schema (`TestSchemaPropertiesCoverFixtureFields`).
- Task 2 (`dab4b54`): each refusal dropped in turn (its case of the
  catalogue tests); values decoded straight into integers, or as `int64`
  (the not-an-`int32` tests); an absent value filled with 0 (the VTT-1000
  test); the namespace check dropped, or its refusal put on the earlier file
  (the VTT-1001 tests); `loadCatalogue` skipped
  (`TestARulesetKeepsEveryCatalogueEntryByItsID`); `json.Unmarshal` in
  place of `decodeStrict` (the undefined-field tests); the value keys read
  unsorted
  (`TestAnEntryHoldingSeveralUnlistedValuesIsRefusedOnTheFirstByKey`);
  `values` deleted from `entry.schema.json`
  (`TestSchemaPropertiesCoverFixtureFields`).
- Task 3 (`0adcde5`): the parent's value over the child's, and one link
  only (the chain and order tests); the walk's visited check dropped (the
  cycle tests failed at their deadline, without growing memory);
  `cycleErr`'s stop negated (the cycle tests red within their deadline, at
  a small fixed memory); the empty, unknown and other-form refusals each
  dropped (its test); the cycle reported on the revisited entry's file, or
  its names left unsorted (the cycle tests).
- Task 4 (`22e1b17`): the forms dropped, reversed, or served to the DM
  alone (`TestEveryRoleReadsTheFormsInTheirDeclaredOrder`); the catalogue's
  sort removed, the catalogue served to the DM alone, or holding its own
  values only, the last made in `resolveExtends`
  (`TestEveryRoleReadsTheCatalogueSortedByIDWithWhatEachExtends`); either
  list started as nil, or a form's absent properties served as `null`
  (`TestNoFormsAndNoCatalogueAreEmptyListsForEveryRole` and QA's route
  tests); the description's sentence removed
  (`TestGetRulesetSaysItAnswersEveryRoleTheFormsAndTheCatalogue`).
- Task 5 (`ecd7d72`): a digit in one entry's description, an emptied
  description, and the catalogue removed (the VTT-1007 test); a digit in an
  ability's description, after the two tests came to share one check
  (VTT-850's test); an entry naming an undeclared form (the conformance
  suite); a weapon's proficiency moved off its ability's binding (nothing
  red, as OD-7 B accepts).
- `214a950`: each of the three link refusals made unreachable in turn
  (`TestAnExtendsWrittenEmptyIsRefused`,
  `TestAnExtendsNamingAnEntryTheCatalogueDoesNotHoldIsRefused`, and
  `TestAnExtendsNamingAnEntryOfAnotherFormIsRefused` with QA's
  `TestQAEXRefusesAnUnresolvableExtends`).

## The existing tests that changed

- **The manifest schema's field list:** `qa_declared_sheet_test.go`'s
  `TestQASheetSchemaDescribesTheSheet` gained `forms` (Task 1).
- **The fixture:** `TestLoadValidFixture` (`internal/rules/load_test.go`)
  pins `heft` among the property keys, and `numberAt` edits the decoded
  JSON (Task 1); `formsManifest`, `propertyRuleset` and
  `TestLoadAcceptsThresholdExplicitFalse` drop the fixture's catalogue
  after copying it (Task 2). See the deviations.
- **The schema tables:** `schema_test.go`'s
  `TestSchemaPropertiesCoverFixtureFields` and
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` gained a row for
  `entry.schema.json` over `testdata/valid/catalogue/` (Task 2).
- **The bound probes:** `id_bound_internal_test.go`'s
  `TestTheSchemasStateTheIDBound` gained the entry's id and the form's
  key; `qa_property_format_test.go`'s
  `TestQAPFSchemasStateTheBoundsLoadEnforces` gained a "form key" and an
  "entry id" probe (Tasks 1 and 2).
- **The route:** `qa_ruleset_route_test.go`'s `qaRRCheckBody` and
  `TestQARRRulesetRouteAnswersEveryRole` call `qaRRCheckForms` and
  `qaRRCheckCatalogue` over `qaRRRulesetDirs`, and
  `TestQARRNothingLoadedAnswersEmptyLists` checks both lists; both tests
  joined VTT-1004's and VTT-1005's evidence (Task 4). Until Task 5 the
  shipped rulesets compared `[]` with `[]`.
- **`get_ruleset`:** `TestGetRulesetReturnsTheRulesetRouteVerbatim`'s
  answer holds a form and an entry (Task 4).
- **Conformance:** VTT-850's test calls `checkShippedDescriptions` (Task
  5).
- **The requirements chain's QA:**
  `tools/check_requirements_chain_qa_test.py`'s
  `test_a_test_file_citing_the_projects_undefined_id_is_refused` and
  `test_a_specification_naming_the_projects_undefined_id_is_refused`
  injected `<TAG>-999` as an undefined id, which the register defines once
  it reaches VTT-999; they cite the copy's first free id (`self.free`), as
  the file's other tests do, their assertion unchanged. Pinned to a
  defined id, both go red (Task 0).
- **Citations:** every test and comment citing VTT-973 now cites VTT-996,
  `ShownUsage.UnmarshalJSON`'s doc among them, and every one citing
  VTT-625 cites VTT-1009. `TestQARuleTheSchemasStateTheBoundLoadEnforces`
  cited VTT-625 without standing in its evidence, and joined VTT-1009's;
  `TestQARuleARulesetWhoseNamesAreAllAtTheBoundLoads` reads no schema and
  no longer cites it (Task 2). `git grep -n -E 'VTT-(973|625)([^0-9]|$)'
  214a950 -- internal cmd client/src tools` prints nothing.
  `docs/specifications` still names them, in SPEC-018's, SPEC-024's and
  SPEC-026's records, for the specification task.
- **Unchanged:** `rulesets/tavern-brawl/`, every manifest but
  `testdata/valid`'s and dnd45e-minimal's (the plan's P4), every golden,
  the contract and both folds (D14).

## Phase 4a adjudications

- Task 1 (`internal/rules/qa_forms_test.go`): no defect.
  - QA finding 1: a form with `properties` absent or `null` loads listing
    none — the schema leaves it optional — no row.
  - QA finding 2: a `null` key, name or description is refused as empty —
    as every manifest field — no change.
  - QA finding 3: a key, name or description of whitespace alone loads — as
    every other free-text field of the format — no change.
  - QA finding 4: a form key's characters are free — an entry names a form
    by the string, and nothing uses a key as a path — no change.
  - QA finding 5: keys differing only in case are two keys —
    near-theoretical (the ruling of 2026-10-21) — no change.
  - QA finding 6: a form's key equal to a property's key loads — form keys
    and property keys are separate namespaces — no change.
  - QA finding 7: two case variants of one field in one object, the later
    wins — `encoding/json`'s, true of every field — no change.
  - QA finding 8: the strict decode names an unknown field inside a form by
    its name alone, as inside `sheet`, while the loader's own checks name
    the path — VTT-996 holds either — no change.
  - QA finding 9: no forms is `nil` — no row.
  - QA finding 10: the order of the refusals is stated nowhere — no row.
  - Needs an id: a case variant read as the field — SPEC-024 states it for
    every manifest field — no row.
  - Needs an id: `forms` absent from the schema's `required` — held by the
    schema walker's required test — no row.
- Task 2 (`internal/rules/qa_catalogue_test.go`): no defect.
  - QA finding: a `null` id, name, description or form is refused as empty,
    as a manifest's are (SPEC-024 reads `null` as absent) — no row.
  - QA finding: `1.0` and `1e2` are refused as not integers, as the loader's
    other integer fields refuse them, while JSON Schema's `integer` accepts
    `1.0` — SPEC-027 to state it — no row.
  - QA finding: `"values": null` loads as none, and `values` of another type
    is refused by the decoder — no row.
  - QA finding: a value's key and an entry's form match case exactly — they
    are data, not field names — no change.
  - QA finding: a file in `catalogue/` that is not `.json`, or a
    subdirectory, is no entry, as in `conditions/` and `abilities/`
    (`jsonFilesIn` reads only the `*.json` files directly inside the
    directory) — no change.
  - QA finding: a duplicate id's refusal names both files, where VTT-998
    asks for one — no row.
  - QA finding: the order of the refusals — no row.
  - QA finding: an id's characters — no row.
  - QA finding: the schema's `int32` range on a value — held by VTT-1008's
    tests, with no bound row of its own — no new row.
- Task 3 (`internal/rules/qa_extends_test.go`): no defect. Its ruling tests
  carried no id, and now cite the rows that hold the rulings:
  `TestQAEXRulingOneIDNamespace` VTT-1001,
  `TestQAEXRulingValuesAreNumbersOnly` VTT-999 and
  `TestQAEXRulingOneEntryPerFile` VTT-997.
  - QA finding: `"extends": null` reads as absent (a `*string`), as
    SPEC-024 reads an optional field — no row.
  - QA finding: a file in `catalogue/` that is not `.json` — as at Task 2 —
    no change.
  - QA finding: the cycle's refusal names one member's file, the lowest
    id's, and every member in id order, but not an entry that reaches the
    cycle from outside it — SPEC-027 to state it — no change.
  - QA finding: ids are case-sensitive, so a case variant of an id in
    `extends` names no entry — SPEC-027 to state it — no change.
  - QA finding: the order of the refusals — every link fault in id order,
    then the first cycle a walk in id order meets, then the shared-id check
    — SPEC-027 to state it — no row.
- Task 4 (`internal/gateway/qa_catalogue_route_test.go`,
  `internal/mcp/qa_ruleset_catalogue_test.go`): no defect.
  - QA finding: the wire's shape — `properties` an array, `values` an
    object keyed by property, no `extends`, `{}` for an entry carrying no
    value, `"properties": []` for a form listing none, the catalogue in the
    byte order of its ids — SPEC-012 to state it — no new row.
  - QA finding: a request without an accepted bearer token is answered 401
    — VTT-578 and VTT-581 — no new row; after Task 4's review the test
    cites VTT-581 and stands in its evidence.
  - QA finding: `get_ruleset`'s answer ends in the route's trailing newline
    — the body verbatim (VTT-621) — no change.
- Task 5 (`internal/rules/conformance/qa_shipped_catalogue_test.go`): no
  defect. QA's probe found every weapon entry equal to its ability's
  bindings, the scimitar's flat bonus not on the entry; run once, green,
  and not kept (OD-7 B).
  - Cut whole, each with its witness: `TestQASCUnchangedSinceBase`, and
    `TestQASCEveryShippedRulesetConforms`, part of which read git. They
    read `22e1b17` through git (`git show`, `git ls-tree`): a check true at
    this ticket's moment, red for good once 5c changes an ability, and
    failing in a shallow clone. Run once, green: the abilities, atoms,
    conditions and goldens byte for byte as at `22e1b17`, the sheet equal,
    the version `"1"`, the properties the ruleset declared before
    unchanged. Item 4's `git diff --stat` is the observation that stands,
    and the conformance half of `TestQASCEveryShippedRulesetConforms` is
    held by `TestConformanceOverRulesetsGlob` (item 4).
  - Cut: `TestQASCCatalogueHoldsTheEntriesOfD11` and its witness, a pin of
    the number of forms, and a pin that every form lists a property. They
    pin content, and the owner ruled on 2026-11-24: "Du kan sätta vilka
    värden du vill när det gäller utrustning. Det viktiga är att vi kan
    läsa och använda vilka värden som helst för vilken utrustning som helst
    oberoende av regelverket." (You may set whatever values you like for
    equipment. What matters is that we can read and use any values at all,
    for any equipment at all, whatever the ruleset.) The armour's numbers,
    and every weapon number beyond the copy item 4 names, are therefore the
    author's content, and no test pins them. A form listing none is legal:
    Task 4's route test serves one (`charm` in `catalogueRouteManifest`).
  - Cut at Task 5's review (S3): QA's check that the shipped forms list
    declared `number` properties, and its witness, which indexed `Forms[1]`
    and so went red on a legal change of content; the loader's own
    refusals (VTT-994, VTT-995) hold what it checked, and it could not fail
    where they do not.
  - Kept, each with a witness that breaks a copy or the loaded ruleset:
    `TestQASCShippedEntryDescriptionsHoldNoDigit` (VTT-1007),
    `TestQASCShippedAbilityDescriptionsHoldNoDigit` (VTT-850) and
    `TestQASCShippedEntriesLoadAsWritten` (VTT-997).
  - QA finding: what a "digit" is — under "What could not be
    established".
  - QA finding: a game word the vocabulary gate misses — under "What could
    not be established".

## The reading reviews

One reviewer per task; each Must and Should below was fixed in the task's
commit.
- Task 0: the verifier's check, by an agent that did not write the ticket
  (passes, with gaps G1 to G9), then a reading review. Must: VTT-1000
  contradicted VTT-1002 for an entry that carries no value but extends one
  that sets it; VTT-1000, D5 and R8 now read "and extends no entry holding
  one" (M1). Shoulds: VTT-997's values resolved through `extends` (S1); no
  test for an entry's undefined field, added to D4 and Task 2 with a break
  (S2); no test inside a form's schema, a nested one added to Task 1, as
  VTT-829's is for the sheet (S3); item 4 names the numbers (S4). Nits:
  VTT-1001 names the field; VTT-999 "within the range of"; VTT-1004 and
  VTT-1005 "every role whose bearer token the server accepts"; the ticket's
  §10 and "named only by" sentences; wraps; Task 0 names the requirements
  chain's QA file; the plan's disclaimer broadened; the two RPTool
  resolvers kept apart (see "Rule 9").
- Task 1: no Must or Should. Nits: `loadForms`' doc points at VTT-993 too;
  `formsManifest`'s descriptive comment made a pointer; QA's top-level
  undefined-field check looks for `unknown field "<f>"`, since a bare
  `form` matched any `forms[…]` refusal; D1's pointer against the code's
  slice, and the tests the fixture broke, to be reported (the deviations).
  The reviewer's notes, unchanged: the schema refuses `"forms": null`
  where the loader loads it, as it does `sheet` and `free_usage_name`;
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` checks one direction
  only, as it did before this ticket. Task 2 was to add the entry schema's
  test and the entry-id probe to VTT-1008 and VTT-1009, and did.
- Task 2: no Must. Should: VTT-625's withdrawal said the row was false, and
  it was not — it held, narrower than its tests once they probe a form's
  key; reworded in the form VTT-417's withdrawal uses (S1). Nits: the entry
  schema and the loader disagree on `"extends": ""` (until Task 3),
  `"values": null`, `1.0` and `1e2`, and case variants, the last three as
  elsewhere in the format, and no test pins the refusal of `1.0` or `1e2`
  (N1, see "What could not be established"); the refusal compacts a nested
  value before naming it (N2); two comments reworded as orders (N3).
- Task 3: Must: `cycleErr`'s unbounded loop (the deviations). Nits: QA's
  order test also cites VTT-997; QA's refusal test joined VTT-1002's
  evidence; `loadCatalogue`'s doc pointer.
- Task 4: Must: QA comment lines past the wrap band `check:new-prose`
  holds (`tools/check-comment-wrap.py`), a step that runs in `task check`
  and at no other time; rewrapped. Should: the refused-token test pointed
  at VTT-1005, and now points at VTT-581 and stands in its evidence (S1).
  Nit: `get_ruleset`'s sentence called the forms "the kinds of entry the
  ruleset's catalogue holds", false for a form with no entry; it now says
  "the ruleset declares" (N1).
- Task 5: no Must. Shoulds: the forms' descriptions said a weapon has "the
  bonus it adds" and armour "adds its bonus", while `armor_bonus` says
  nothing adds it; both now say "the bonus it is meant to add", and "dice
  it deals" became "damage dice it rolls" (S1); `guide.md`'s goblin bonus
  (S2, the deviations); two QA witnesses went red on legal content
  changes, one indexing `Forms[1]`, one needing a shipped ruleset with no
  catalogue: the forms check and its witness were cut, and the
  no-catalogue witness now copies the ruleset and removes its `catalogue/`
  (S3). Nits: the property descriptions' "each still rolls" became "each
  attack still rolls", since `rally` rolls neither an attack bonus nor a
  weapon's damage (N1); QA's glossed row comments reduced to bare ids, and
  an unexported doc line dropped (N2); a `[0]` index guarded,
  `qascCatalogueRuleset` now requiring an entry (N3); `get_ruleset`'s and
  `add_actor`'s texts on `attributes` (N4, inside OD-2: see "What could not
  be established").
- `214a950`: no finding; its cover profile puts each condition inside a
  counted block (`214a950`'s message).

## The plan's gaps

- **G1.** Taken into the ticket before Task 0's commit; see "Rule 9".
- **G2.** The rules wider than items 1 and 2: VTT-994 refuses a form with
  no description, VTT-998 an entry with no id, name or description and an
  entry id past the bound; observed under items 1 and 2.
- **G3.** ADR-012 §18, read by D15: an entry's `description` is its
  explanation in the ruleset's words (principle 35), served as a
  property's and a condition's are; §18's text is the story offered to the
  DM, which no entry carries yet and which arrives as a field of its own.
  SPEC-027 states the reading.
- **G4.** Each weapon's numbers in two places: OD-7 B (item 4, and "What
  was deliberately left out").
- **G5.** The armour's numbers: the author's content (the deviations, and
  the owner's ruling under Task 5's adjudications).
- **G6.** The files reached are the plan's list, plus
  `internal/rules/load_test.go` (Task 1), `property_test.go` (Task 2) and
  `rulesets/dnd45e-minimal/guide.md` (Task 5); Task 3 also reached
  `format.go` and `entry.schema.json`, which only Tasks 1 and 2 named. See
  the deviations. `rulesets/tavern-brawl/` and
  `internal/rules/conformance/conformance.go` are unchanged, as G6 said.
- **G7.** D5, held by VTT-1000.
- **G8.** `get_ruleset`'s description moved, held by VTT-1006.
- **G9.** Under OD-2 A, SPEC-025 gains the consequence that a form's
  property is a key a sheet may hold until 5b; the specification task's.
  SPEC-026's Requirements list moves from VTT-973 to VTT-996. SPEC-022
  does not move under OD-1 A.

## What the next full mutation run should find

The client code this ticket adds is `client/src/metadata.ts`'s `FormDef`,
`CatalogueEntry` and two fields of `RulesetMeta`: type declarations, which
compile to nothing (D10), so it adds no client code a mutant could change.
What `task check` judged of the client's stored report is under item 6
(SPEC-023); a full run is the owner's to ask for. No survivor is answered
ahead of a run that names it, and neither `tools/mutation-equivalents.txt`
nor `tools/ts-mutation-equivalents.txt` is edited.

On the Go side, the new branches in `internal/rules` and `internal/gateway`
are in the mutation gate's packages. `cycleErr`'s loop is written so that
no mutant gremlins makes of its stop grows without end (Task 3's review).
`resolveExtends`'s link refusals were a `switch`'s case conditions from
Task 3 until `214a950` wrote them as ifs (see "Deviations"), so the gate's
run on `214a950` is the first to run mutants on them. `gremlins unleash
./internal/rules/ --dry-run` on `214a950` lists the `==` of the empty
refusal and the `!=` of the other-form one RUNNABLE; `!held` is a mutant of
no gremlins operator, and its break holds it. `tools/mutation-equivalents.txt`
excuses no mutant in `load.go`, and the mutation gate passed on `214a950`,
so it killed both.

## What could not be established

- **What a "digit" is.** VTT-850 and VTT-1007 say a description holds "no
  digit". `checkShippedDescriptions` refuses the ASCII digits alone; QA's
  `TestQASCShippedEntryDescriptionsHoldNoDigit` and
  `TestQASCShippedAbilityDescriptionsHoldNoDigit` use `unicode.IsDigit`, a
  digit of any script, and `TestQASCWitnessEntryDescriptions` breaks one
  with an Arabic-Indic digit. A number spelled out passes both. Which the
  rows mean is not ruled; SPEC-027 is to state the reading.
- **A game word the vocabulary gate misses.** QA saw that semgrep's
  `no-game-system-vocabulary-in-engine` (`.semgrep/vocabulary.yml`)
  matches `hit_?points`, which does not match "hit points" with a space or
  "hit-points" with a hyphen, so either in Go under `internal/` or `cmd/`
  passes rule 5's gate.
  The gap is older than this ticket, and the rule's other multi-word terms,
  `saving_?throw` among them, share it. This report's commit records it,
  with its recipe, in `docs/verification-debt.md`. Changing the pattern is
  gate work, paused since 2026-08-27.
- **The OD-2 hole, accepted until 5b.** A property a form lists is a key
  `SheetKeyFault` (`internal/rules/sheet.go`) accepts on a sheet, so
  `add_actor` (`internal/gateway/add_actor_validate.go`) and
  `adventure.Load` take a weapon's number as a creature's attribute, and
  nothing refuses it (the plan's OD-2 A). `guide.md` says never to set one
  on a creature, but `get_ruleset`'s description ("A number property's
  value is under an actor's `attributes`") and `add_actor`'s `attributes`
  text in `tools/toolgen/main.go` ("each under a key the served ruleset
  declares as a number property") still invite a form's properties onto a
  creature (Task 5's review, N4). 5b closes the hole, and these texts with
  it.
- **Where the schemas and the loader disagree.** `ruleset.schema.json`
  refuses `"forms": null`, which the loader loads as none, as for `sheet`
  and `free_usage_name`; `entry.schema.json` refuses `"values": null`,
  which loads as none; JSON Schema's `integer` accepts `1.0` and `1e2`,
  which the loader refuses, and no test pins that refusal (VTT-999's
  wording does not ask for one); the schemas match field names exactly,
  where the loader reads a case variant as the field.
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` checks one direction
  only, as before this ticket.
- **SPEC-027 does not exist at `214a950`.** No comment points at it yet:
  the code points at row ids. D15's reading of ADR-012 §18, the refusal
  order and the cycle's report are stated by no specification until the
  specification task that follows this report.
- **MapTool was read again only in part.** "Rule 9" rests on the plan's
  P10 and Task 0's review; for this report only
  `rule-framework/library/mtscript/public/lib/TR_Visit.mts` was re-read,
  at RPTool `63862e4`.

## What was deliberately left out

- **The ticket's "Not this ticket":** a sheet that refers to an entry, a
  form's slots and sets, and what a sheet holds per reference (5b); the
  actions reached through references, and the abilities that bind a
  weapon's numbers today, which stay as they are (5c, and ADR-012 step 2's
  procedures); derived properties (ADR-012 step 1 item 6); equipping as an
  effect (ADR-012 §9, step 2). Each is a step of ADR-012's build order, in
  the order the owner set on 2026-11-24.
- **Prose that 5b and 5c must rewrite.** In
  `rulesets/dnd45e-minimal/ruleset.json`, the descriptions of
  `proficiency`, `damage_dice`, `damage_die` and `range` say "no power
  reads it yet", and `armor_bonus`'s that "nothing adds it yet";
  `guide.md` says no sheet refers to an entry and no ability reads one.
  Each is true at `214a950` and goes false when a sheet refers to an entry
  (5b) or an ability reads its weapon (5c); the ticket that makes it false
  rewrites it.
- **The weapons' numbers in two places (OD-7 B).** Each weapon's
  proficiency, damage and range stand in its ability's bindings and in its
  entry, and nothing holds them equal: nothing reads an entry's numbers at
  the table, so a drift changes no outcome. When 5c makes an ability read
  its weapon, dnd45e-minimal's compiled goldens, which pin today's
  bindings, change for any weapon whose entry differs, and the drift shows
  there.
- **By the owner's rulings:** no `dice` or `tags` type (OD-1 A), so a
  damage is two numbers until ADR-012 step 2's dice ticket, which may
  re-encode these entries, our own rulesets, with no migration
  (2026-09-04); no visibility per form (OD-4 A), which arrives with its own
  ticket; no test pairing an ability with its weapon (OD-7 B); the OD-2
  hole until 5b; dnd45e-minimal's version kept at `"1"` (D16), since no
  ability, the rules that decide outcomes, changed (principle 28), so a
  campaign's log names no new version for this change (SPEC-026 writes its
  entry only when the id or the version differs).
- **Not served:** `extends` (D8); the resolved values are what a reader
  reads.
- **Not read by the client:** `forms` and `catalogue` are typed and unread
  (D10).
- **No check for a misspelled `catalogue/`:** a directory the loader does
  not read is ignored, as any other is (the plan's P2, OD-6 A); 5b's
  references would be refused against it.
- **Near-theoretical, with no code, test or debt entry (the ruling of
  2026-10-21):** form keys that differ only in case.
- **No gate work (paused 2026-08-27):** the vocabulary pattern, and the
  schema walker's other direction.
- **The client's full mutation run**, the owner's to ask for.
- **The specifications:** SPEC-027, new, and SPEC-024, SPEC-012, SPEC-018,
  SPEC-025 and SPEC-026, in the specification task that follows this
  report (the plan's Task 7).