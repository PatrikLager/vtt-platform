# An ability may carry the ruleset's description of it, and the sheet shows it — report

Ticket: `docs/superpowers/specs/2026-11-18-an-ability-carries-its-description-design.md`.
Plan: `docs/superpowers/plans/2026-11-18-an-ability-carries-its-description.md`.
Branch `feat/an-ability-carries-its-description` from main `a164cac`.
Commits: `56e1b86` (Task 0: ticket, plan, register), `b2f4979` (Task 1, the
format), `e920cd2` (Task 2, the shipped rulesets' texts), `f634329` (Task 3,
the read surface and `get_ruleset`), `16f0b75` (Task 4, the sheet's title;
the last code commit) and `32a6a58` (the comment ledger). Every code
reference below is to `16f0b75`.

## Done looks like

1. [x] **An ability declares a description, and a ruleset names an ability
   that spends no resource.** `loadCompositions` (`internal/rules/load.go`)
   decodes an ability's `description` and `compilePower`
   (`internal/rules/compile.go`) keeps it as `CompiledPower.Description`;
   `loadManifest` decodes `free_usage_name` and `loadV2` keeps it as
   `Ruleset.FreeUsageName`. Without either, the ruleset loads as before,
   with none.
   - `internal/rules/ability_description_test.go`'s
     "TestAnAbilityKeepsTheDescriptionItDeclares",
     "TestAnAbilityDeclaringItsDescriptionNullHasNone" and
     "TestARulesetKeepsTheNameItGivesAnAbilityThatSpendsNoResource".
   - Independently: `internal/rules/qa_ability_description_test.go`'s
     "TestQADescKeptOnItsOwnAbilityOnly", "TestQADescNoneWhenAbsentOrNull",
     "TestQADescKeptAsWritten" and "TestQADescFreeUsageNameKeptOrNone".
   - On `a164cac` these tests do not build, since neither field exists
     there. The strict decode that refused both fields as unknown is what
     Task 1's breaks "the description not decoded" and "the name not
     decoded" put back: the first turned
     "TestAnAbilityKeepsTheDescriptionItDeclares" and
     "TestAnAbilityDeclaringItsDescriptionNullHasNone" red, the second
     "TestARulesetKeepsTheNameItGivesAnAbilityThatSpendsNoResource".
2. [x] **The format says so.** `internal/rules/schema/ability.schema.json`
   describes `description` and `ruleset.schema.json` `free_usage_name`, each
   an optional string of at least one character.
   - "TestTheAbilitySchemaDescribesAnOptionalNonEmptyDescription" and
     "TestTheRulesetSchemaDescribesAnOptionalNonEmptyName"
     (`internal/rules/ability_description_test.go`), which also require the
     fixtures under `internal/rules/testdata/valid` to declare both;
     `internal/rules/schema_test.go`'s "TestSchemaPropertiesCoverFixtureFields"
     and "TestSchemaRequiredFieldsMatchLoaderEnforcement" hold the schemas to
     those fixtures.
   - "TestQASheetSchemaDescribesTheSheet"
     (`internal/rules/qa_declared_sheet_test.go`) holds the ruleset schema's
     top level to the manifest's fields, `free_usage_name` among them.
   - On `a164cac` neither schema describes either field.
3. [x] **The shipped rulesets describe their abilities.** Every ability under
   `rulesets/dnd45e-minimal/abilities` and `rulesets/tavern-brawl/abilities`
   declares the description the owner signed off on; `dnd45e-minimal`'s
   `ruleset.json` names an ability that spends no resource "At will" and
   `tavern-brawl`'s names none.
   - `internal/rules/conformance/conformance_test.go`'s
     "TestEveryShippedAbilityCarriesADescriptionWithNoDigit".
   - Independently: `internal/rules/conformance/qa_shipped_descriptions_test.go`'s
     "TestQAShippedAbilityDescriptionsNonEmptyNoDigit",
     "TestQAShippedCompiledDescriptionMatchesFile" and
     "TestQAShippedFreeUsageName".
   - Both still load and pass their goldens: "TestConformanceOverRulesetsGlob";
     the compiled goldens are unchanged on the branch, since
     `compiledPowerDump` carries no description.
4. [x] **The read surface serves it.** `handleRuleset`
   (`internal/gateway/metadata.go`) answers each ability it serves with
   `description` (`abilityJSON`, `omitempty`), and `freeUsageName`
   (`rulesetJSON`, `omitempty`) to every role; `getRulesetDescription`
   (`internal/mcp/ruleset_tool.go`) says both.
   - `internal/gateway/ability_description_route_test.go`'s
     "TestEveryRoleReadsTheDescriptionOfEachAbilityItIsServed",
     "TestAnAbilityWhoseRulesetDeclaresNoDescriptionIsAnsweredWithoutOne",
     "TestEveryRoleReadsTheRulesetsNameForAnAbilityThatSpendsNoResource" and
     "TestNoRoleReadsAFreeUsageNameWhenTheRulesetGivesNoneOrNoneIsLoaded".
   - `internal/mcp/ruleset_tool_test.go`'s
     "TestGetRulesetSaysAnAbilityCarriesTheRulesetsDescriptionOfIt" and
     "TestGetRulesetSaysItAnswersTheRulesetsNameForFreeUsage".
   - Independently: `internal/gateway/qa_ability_description_route_test.go`
     and `internal/mcp/qa_ruleset_tool_description_test.go`.
   - `cmd/vtt/qa_get_ruleset_test.go`'s
     "TestQARRGetRulesetReturnsTheRouteBodyVerbatim" refuses a
     `dnd45e-minimal` body lacking either field, so its verbatim comparison
     covers both.
   - On `a164cac` the route answers neither field.
5. [x] **The sheet shows it on hover.** `renderPlayerPanel`
   (`client/src/view/player.ts`) titles an ability's button through
   `buttonTitle`: the non-empty description and the non-empty cost line,
   joined by a line break, the description first. `costTitle` gives a
   resource ability's cost line as before and an ability's that spends no
   resource the `freeUsageName` of the window's `layout`, which `app.ts`
   takes from the newest `/api/ruleset` answer; no window writes "at will"
   of its own.
   - `client/test/panel-sheet.test.ts`'s "a resource ability's title is its
     description, a line break and its cost line", "a resource ability with
     no description, or an empty one, is titled by its cost line alone",
     "an ability that spends no resource is titled by its description and
     the name the ruleset gives such an ability" and "a description's own
     line breaks stand in the title as written".
   - `client/test/app.test.ts`'s "a player's ability button is titled by the
     description and the free-usage name of the newest ruleset answer".
   - Independently: `client/test/qa-ability-title.test.ts`.
   - On the real wire: `client/e2e/table.spec.ts`'s "a player's panel draws
     the pages the ruleset declares, and only the DM's board draws chips"
     holds the player's Longsword Strike button to the description and the
     `free_usage_name` read from their files.
   - On `a164cac`'s client the panel-sheet tests of a title with a
     description, of an ability that spends no resource and of a
     description's line breaks fail; the test of a cost line alone pins
     what the client already did.
6. [x] **What stays.** The button's label (VTT-846), a resource ability's
   cost line (VTT-844) and which abilities a role reads (VTT-691 to VTT-693)
   keep their tests, but for the at-will assertions, which item 5 changes:
   the tests that read an at-will button's
   title as "at will" take their fixture's word, or none, exactly, under
   names saying so (see "The existing tests that changed").
7. [x] **The gate.** `task check` is green on `32a6a58`, the ledger commit
   on top of `16f0b75`, and `task e2e` is green on the same commit; the
   embedded bundle under `cmd/vtt/webdist` was rebuilt in `16f0b75` from
   that commit's client. `task check` did not measure the client's
   mutation: its stored report measured `09c55a9`, and it names this
   ticket and the sheet's among those added since (SPEC-023).

## What the rules became

The ticket's rules, after the sort (the plan's "The sort's refusals"
carries what it refused):

- An ability may declare a description, kept with it: VTT-847; an empty one
  refused: VTT-848.
- A ruleset may name an ability that spends no resource, kept: VTT-856; an
  empty name refused: VTT-857; the schema describes it: VTT-858; the
  manifest's fields as the format defines them: VTT-861, which succeeds
  VTT-830, withdrawn.
- `ability.schema.json` describes `description` as optional: VTT-849.
- Every shipped ability declares a description with no digit: VTT-850.
- The read surface serves an ability's description to whoever it serves
  the ability: VTT-851; none when none: VTT-852; `get_ruleset` says so:
  VTT-853.
- The read surface serves the name to every role: VTT-859; `get_ruleset`
  says so: VTT-860.
- An ability button's title: VTT-854; the cost line of an ability that
  spends no resource: VTT-855.

Refused at the sort: an older answer leaving the description as the newest
gave it (held by VTT-753, since the description travels on the ability
object that row governs); an ability file's unknown field refused (today's
behaviour, held by `decodeStrict` and by no row, a gap the plan leaves to
the next work on the ability format).

Every row from VTT-847 to VTT-861 cites tests; none is open.

## The deliberate breaks

Each was made by an edit, run, and reverted by its inverse edit, the file's
hash checked afterwards; each turned its test red.

- **Task 1:** the description not decoded; `compilePower` leaving it out;
  an empty description accepted; the name not decoded; an empty name
  accepted; each field left out of its schema; each field listed as
  required.
- **Task 2:** one ability's description removed; a digit written into one.
- **Task 3:** the description written only for the DM; another ability's
  description written; the description's `omitempty` dropped; the tool's
  words for each field dropped; `freeUsageName` served to the DM alone; its
  `omitempty` dropped.
- **Task 4:** the cost line first; the two joined by a space; the
  description tested against `undefined` alone; "at will" written again; the
  word taken from every answer, not the newest; a description's line breaks
  replaced by spaces.
- **After the reviews:** QA's empty-name test with an empty name accepted
  (Task 1); the loader dropping the name, against QA's reworked shipped-name
  test (Task 2); D9's "absent when it gives none" dropped, against QA's
  tightened tool-description test (Task 3); the free-usage name leaking onto
  a resource ability (Task 4).

The break script's guard stopped these breaks before they were applied,
because the broken text already occurred in the file: `compilePower`'s
(Task 1), the description's `omitempty` (Task 3: the bare tag also sits on
`conditionJSON` and `propertyJSON`) and D9's words (Task 3, after the
constant was reflowed). Each was rerun with an edit that could not match
twice.

## Rule 9: how MapTool does this

MapTool's macro button keeps a `toolTip` (`MacroButtonProperties`), an
empty string by default and set by the macro's author; an empty tooltip
draws nothing. The owner's 4.5e framework under `~/dev/RPTool/rule-framework`
gives an ability an optional `flavorText` (`schema/ability.schema.ts`),
printed in the chat after a use. Borrowed: the author's own text, optional,
on hover of the button that uses the ability. Not borrowed: printing it at
each use, since the Story says what happened and not what an ability is;
running a tooltip as code; figures in the shipped texts.

## The plan's gaps and the owner's answers

- **G1** (rules the ticket's list does not carry): carried by VTT-848,
  VTT-849, VTT-850, VTT-852 and VTT-854.
- **G2** (files the ticket does not name): touched as the plan listed them.
- **G3** (the ticket's open questions): answered by OD-1, OD-2 and OD-6;
  how a long title draws, and that some touch platforms draw none, is
  unchanged and unmeasured.
- **G4** (a disabled button's title): unmeasured; no browser was asked
  whether it draws a disabled button's native title.
- **G5** (a loose citation in the ticket): closed before Task 0's commit;
  the ticket names the sheet's plan.
- **G6** (words the composition binds): the owner accepted the texts naming
  the stat, the defense and the condition (OD-7).

The owner's answers, 2026-11-18: OD-1 A (on `CompiledPower`, not in the
goldens), OD-2 A (an empty description refused), OD-3 A (absent on the wire,
optional in the client), OD-5 A (`format_version` stays "3"), OD-6 A
(SPEC-024 is to record it), OD-7 A (the proposed texts). OD-4 was answered
otherwise: the title's form as proposed, but "at will" is a D&D 4e word in
the platform's own text, so the ruleset names an ability that spends no
resource (`free_usage_name`), `dnd45e-minimal` "At will" and `tavern-brawl`
none, and the platform writes no word of its own.

## The existing tests that changed

- "TestQASheetSchemaDescribesTheSheet" counts the schema's top-level fields
  against its list, which gained `free_usage_name` (Task 1); it is cited for
  VTT-858 too.
- `client/test/panel-sheet.test.ts`'s "a resource ability's button names the
  resource, its current and max, and the cost" lost its at-will assertion,
  which moved into the VTT-855 test.
- `client/test/spectator-view.test.ts`'s "an at-will ability is enabled and
  says so on hover" is "an ability that spends no resource is enabled, and
  untitled when the ruleset names no such ability", its title `""`.
- `client/test/qa-panel-sheet.test.ts`'s "an at-will button's title says at
  will and names no cost" is "a button for an ability that spends no
  resource is titled by the ruleset's name for it and names no cost", exact
  where it was `toContain`, and cited for VTT-855 instead of VTT-844.
- `cmd/vtt/qa_get_ruleset_test.go`'s
  "TestQARRGetRulesetReturnsTheRouteBodyVerbatim" gained its guard (Task 3);
  no assertion changed.
- `client/e2e/table.spec.ts`'s "a player's panel draws the pages the
  ruleset declares, and only the DM's board draws chips" gained the title
  assertion (Task 4).

## Phase 4a adjudications

No QA agent found a behaviour defect. One line per finding.

- Task 1 (`internal/rules/qa_ability_description_test.go`), spec gap:
  SPEC-024 names neither field — goes into SPEC-024.
- Task 1, spec gap: dnd45e-minimal declared no name yet — Task 2's.
- Task 1, ambiguity: the refusal's words for an empty value — "must not be
  empty", as the plan gives them.
- Task 1, ambiguity: an unknown field inside `sheet` — named by its name,
  as the decode names it.
- Task 1, candidate row: a description kept unbounded and as written —
  held by VTT-847's words; no new row.
- Task 2 (`internal/rules/conformance/qa_shipped_descriptions_test.go`),
  ambiguity: what counts as a digit — QA reads any Unicode digit, the
  implementer's test ASCII digits; both hold the shipped texts.
- Task 2, spec gap: VTT-850 does not mention the name — VTT-856's tests
  hold it.
- Task 3 (`internal/gateway/qa_ability_description_route_test.go`,
  `internal/mcp/qa_ruleset_tool_description_test.go`), spec gap: SPEC-012
  names neither field — goes into SPEC-012.
- Task 3, ambiguity: the tool's exact words — D9's and D17's, held by
  `ruleset_tool_test.go`.
- Task 3, ambiguity: an empty description or name — never reaches the
  route, since `rules.Load` refuses both.
- Task 3, candidate row: a role served no ability is served no
  description — follows from VTT-851 with VTT-691.
- Task 4 (`client/test/qa-ability-title.test.ts`), spec gap: SPEC-022 still
  says "at will" — goes into SPEC-022.
- Task 4, ambiguity: VTT-844's words for a resource the sheet does not
  hold — VTT-844's tests hold them.
- Task 4, ambiguity: a whitespace-only description — kept as written, as
  VTT-854 says.
- Task 4, ambiguity: an empty title — `""`, what the DOM's `title` holds.
- Task 4, out of scope: a button disabled when unaffordable or with no
  token — the panel's rule before this ticket (the plan's G4), which no
  row words.
- Task 4, a QA test that could not fail: its spectator test accepted a
  panel drawing no button — not kept.

## Phase 4b

One reviewer read each task's diff before its commit.

- **Task 0,** read again after its fixes: the QA schema test that counts
  the manifest's fields; VTT-844's at-will assertions moving to VTT-855; a
  decision left saying no row is withdrawn; the specifications' plan
  missing `free_usage_name`; rows reworded before commit (VTT-851,
  VTT-854, VTT-855 and VTT-860: "spends no resource", and a title's empty
  cases), and Task 3's tests extended to no ruleset loaded.
- **Task 1:** QA's copy-based tests asserted that every other shipped
  ability has no description, which Task 2 made false (QA's copy now
  strips every description first); QA's empty-name test passed on today's
  unknown-field error (it now requires "must not be empty"); the schemas
  described serving and hover before either existed.
- **Task 2:** every text checked against its ability's composition, and
  none false. QA's comments described behaviour; a QA test that could not
  fail when VTT-850 breaks duplicated "TestConformanceOverRulesetsGlob"
  (deleted); QA's free-usage test pinned which shipped ruleset names one;
  Task 1's doc comment on `FreeUsageName` was over-long.
- **Task 3:** a lint refusal in QA's route test; QA's tool-description test
  satisfied by a later sentence; a QA test cited for a row it cannot show.
  The review joined the string literals of both versions of
  `getRulesetDescription` and found D9's and D17's words the only change.
- **Task 4:** no test held a resource ability's title while the ruleset
  names free usage, so a leak of the name onto a resource ability passed
  (assertions added, seen red); QA tests cited for VTT-845 and VTT-695,
  which they do not read.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The plan's "Rows checked and left standing" (VTT-621) has the route serve a description for some ability of each shipped ruleset, which the `cmd/vtt` guard was to check | It checks `dnd45e-minimal` | The guard exists so that the verbatim comparison covers the new fields, and one ruleset serving both is enough for that; the existing sheet guard checks `dnd45e-minimal` alone, in the same way |
| `getRulesetDescription` gaining D9's and D17's words | Its constant was also reflowed | One added fragment ran past the file's width; the joined string differs from `a164cac`'s by those words alone |
| Task 1 committed with `tools/check-new-prose.py` clean, as its "True when" requires | Task 1's commit carried a doc comment on `Ruleset.FreeUsageName` that the prose check refuses as over-long; Task 2's commit shortened it | The prose check ran clean before the reading review's rewording of that comment, and was not run again before the commit; Task 2's review caught it |
| Every task committed with `tools/check-comments.py` clean | It reported `conformance_test.go`'s share fallen under its ceiling from Task 2's commit until the ledger commit | A lowered ceiling cannot be raised back, so the ledger is written once, after the last code commit (`32a6a58`) |

A decision made in Task 3: `rulesetJSON.FreeUsageName` carries no comment.
A one-line warning to keep `omitempty` took `metadata.go`'s comment share
over its ceiling, and a ceiling only comes down;
"TestNoRoleReadsAFreeUsageNameWhenTheRulesetGivesNoneOrNoneIsLoaded" goes
red without `omitempty`, as the description's test does.

The full Go suite during Task 2 went red in `internal/gateway`, refusing
`fists.json`'s empty description: the QA agent's own break had blanked the
shipped file while the suite read it. Rerun with the file restored, the
package was green, and every shipped file's diff was its one added line.
Nothing in the code caused it; the suite and a QA agent that breaks shared
files must not run at once.

## What could not be established

- **Whether a browser draws the title of a disabled button** (G4), and how
  a long title draws on a touch platform (G3). No browser was asked.
- **Fists' text and Dazed by Ale.** A fists hit spills Drink, and Drink's
  threshold leaves the target Dazed by Ale; fists' signed-off text says
  only that it spills Drink, where chair-swing's names the daze. It is not
  false. Whether it should say more is the author's, and the owner's to
  decide.
- **The client's stored mutation report** measured `09c55a9`; Task 4
  changed `client/src/view/player.ts`, `client/src/app.ts` and
  `client/src/metadata.ts` after it, so `task check` judges nothing of it
  until a full Stryker run, which is the owner's to ask for (SPEC-023).

## What was deliberately left out

- The format's keyword `at_will` / `atWill`: the ruling concerns the word a
  player sees.
- A description on any other part of a ruleset.
- A condition named by its id on a disc's dot and in the Story (G7 of the
  sheet's plan): the next ticket.
- Serving how an ability resolves; the DM console's give and take; the
  Story's line of an ability's use.
