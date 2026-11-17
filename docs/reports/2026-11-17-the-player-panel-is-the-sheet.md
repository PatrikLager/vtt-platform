# A player's panel is their character's sheet, laid out as its ruleset declares — report

Ticket: `docs/superpowers/specs/2026-11-17-the-player-panel-is-the-sheet-design.md`.
Plan: `docs/superpowers/plans/2026-11-17-the-player-panel-is-the-sheet.md`.
Branch `feat/the-player-panel-is-the-sheet` from main `0ee160b`.
Commits: `38fb523` (Task 0: ticket, plan, register), `105f2b5` (Task 1, the
format), `7b0ace1` (Task 2, the read surface and `get_ruleset`), `33a339d`
(Task 3, no chip in a player's or a spectator's window), `82c9773` (Task 4,
the client's model of the sheet), `3851b70` (Task 5, the panel draws it; the
last code commit) and `b894e8e` (the comment ledger). Every code reference
below is to `3851b70`.

## Done looks like

1. [x] **A ruleset declares its sheet.** `rulesets/dnd45e-minimal/ruleset.json`
   declares the pages "In play" and "Character", each of named sections;
   `rules.Load` returns them in `Ruleset.Sheet` (`internal/rules/format.go`)
   in the order declared.
   - `internal/rules/declared_sheet_test.go`'s
     "TestADeclaredSheetLoadsInTheOrderDeclared" and
     "TestTheShippedRulesetsLoadTheSheetTheyDeclare", which compares
     `Ruleset.Sheet` with what each shipped `ruleset.json` declares.
   - Independently: `internal/rules/qa_declared_sheet_test.go`'s
     "TestQASheetCommittedRulesetReturnsItsDeclaredSheet" and
     "TestQASheetOrderIsTheOrderDeclared".
   - On `0ee160b` the loader refuses the field: `rules: …ruleset.json: json:
     unknown field "sheet"` (the plan's P1).
2. [x] **A sheet that does not fit its ruleset is refused.** `checkSheet`
   (`internal/rules/load.go`) runs after the abilities compile and refuses,
   naming the file and the field, a sheet of the wrong shape, a key no
   property declares, a `spends` of no resource property, a repeat, and an
   ability whose usage no section shows; `decodeStrict` and
   `ShownUsage.UnmarshalJSON` refuse a field the sheet does not define.
   - `declared_sheet_test.go`'s "TestASheetOfTheWrongShapeIsRefused",
     "TestASheetNamingWhatTheRulesetLacksIsRefused",
     "TestASheetNamingAThingTwiceIsRefused",
     "TestAnAbilityWhoseUsageNoSectionShowsIsRefused" and
     "TestAFieldTheSheetDoesNotDefineIsRefused", and
     `qa_declared_sheet_test.go`'s "TestQASheetRefusesAMalformedShape",
     "TestQASheetRefusesAKeyThatFitsNoProperty", "TestQASheetRefusesARepeat",
     "TestQASheetRefusesAnAbilityWhoseUsageNoSectionNames" and
     "TestQASheetRefusesAnUnknownField".
   - The schema: `internal/rules/schema/ruleset.schema.json` describes
     `sheet`. `declared_sheet_test.go`'s
     "TestTheSchemaDescribesEverySheetField" holds every field the fixture's
     sheet uses to it; `internal/rules/schema_test.go`'s
     "TestSchemaPropertiesCoverFixtureFields" holds `sheet` itself, and its
     "TestSchemaRequiredFieldsMatchLoaderEnforcement" runs its subtests for
     `sheet[0].name`, `sheet[0].sections`, `sheet[0].sections[0].name` and
     `sheet[0].sections[0].values` against `internal/rules/testdata/valid`,
     which now declares a sheet of more than one page.
3. [x] **The read surface serves it.** `handleRuleset`
   (`internal/gateway/metadata.go`) answers `sheet` to every role, `[]` for
   a ruleset declaring none and for none loaded; `get_ruleset` returns the
   route's body as it was and its description (`getRulesetDescription`,
   `internal/mcp/ruleset_tool.go`) says so.
   - `internal/gateway/sheet_route_test.go`'s
     "TestTheRulesetRouteAnswersEveryRoleTheDeclaredSheetInOrder" and
     "TestTheRulesetRouteAnswersNoSheetAsAnEmptyList";
     `internal/gateway/qa_sheet_route_test.go`'s
     "TestQASheetRouteServesCommittedSheetToEveryRole",
     "TestQASheetRouteKeepsDeclaredOrderAndShapes" and
     "TestQASheetRouteIsEmptyListWithNoRulesetOrNoSheet".
   - `internal/mcp/ruleset_tool_test.go`'s
     "TestGetRulesetSaysItAnswersTheDeclaredSheet", which reads the
     description through an MCP session.
   - `cmd/vtt/qa_get_ruleset_test.go`'s
     "TestQARRGetRulesetReturnsTheRouteBodyVerbatim" now fails as a
     degenerate fixture when `dnd45e-minimal`'s route serves no sheet.
   - Each fails on `105f2b5`, Task 2's parent: the route carried no `sheet`
     and the description said nothing of it.
4. [x] **The panel draws the pages.** `renderPlayerPanel`
   (`client/src/view/player.ts`) draws a `.tabs` of one `.tab` per page for
   a sheet of more than one page and keeps the page shown in
   `PlayerUIState.page`.
   - `client/test/panel-sheet.test.ts`'s "a sheet of several pages draws one
     tab per page in order and shows the first page";
     `client/test/qa-panel-sheet.test.ts`'s "several pages draw one tab
     each, in declared order, first shown" and "a click on a tab shows its
     page after the repaint".
   - End to end: `client/e2e/table.spec.ts`'s "a player's panel draws the
     pages the ruleset declares, and only the DM's board draws chips" reads
     `dnd45e-minimal`'s pages from its `ruleset.json`, finds them as the
     player's tabs, clicks the second and finds its first section's heading.
   - `panel-sheet.test.ts`'s tests each fail on `0ee160b`.
5. [x] **A section shows its values in the ruleset's words.** A value is a
   `.value`, "<name> <value>", titled by the property's description; a
   resource reads `current/max`; a value the character does not hold is
   left out.
   - `panel-sheet.test.ts`'s "a page draws its sections in order, headed by
     name, values in the section's order" and "a value whose key the ruleset
     does not declare is named by its key, untitled";
     `qa-panel-sheet.test.ts`'s "a value is the display name and value,
     titled by the description" and "a key the ruleset does not declare as
     that type is named by its key".
   - The model's: `client/test/sheet.test.ts`'s "a value reads as its
     property's name and value, titled by its description" and "a key the
     character's sheet does not hold draws nothing".
6. [x] **Conditions are named.** A `.condition` by the ruleset's name and
   titled by its description, else by its id, in the order applied.
   - `panel-sheet.test.ts`'s "conditions are drawn in the order applied, by
     name and description, else by id"; `qa-panel-sheet.test.ts`'s
     "conditions in the order applied, named and titled by the ruleset" and
     "an undeclared condition is named by its id with no title";
     `sheet.test.ts`'s "conditions come in the order applied, named and
     titled by the ruleset, else by id".
7. [x] **Abilities stay where play happens.** Each ability the panel offers
   is one button in the section whose usages name its usage; the button is
   titled "<resource> <current>/<max>, costs <cost>", with " — not enough"
   when the character cannot afford it (`costTitle`).
   - `panel-sheet.test.ts`'s "each offered ability is one button in the
     section of its usage, labelled by its name, else its id", "a resource
     ability's button names the resource, its current and max, and the cost"
     and "an ability armed on one page keeps its targets drawn on another";
     each fails on `0ee160b`.
   - Arming and targeting as before: the tests behind VTT-436, VTT-439 and
     VTT-441 pass unchanged, and the e2e "a player's ability reaches the
     creature they aim it at" arms "Longsword Strike" on the first page.
   - The title tests that read the old words now read the new ones exactly:
     `client/test/qa-property-names.test.ts`, `spectator-view.test.ts`,
     `qa-older-answer.test.ts` and `app.test.ts`.
8. [x] **A ruleset that declares no sheet is drawn as one page.** With no
   sheet, or `[]`, the route's answer for `tavern-brawl` and for no ruleset
   (item 3), the panel draws no tab and one page of "Values", "Conditions"
   and "Abilities", each left out when empty.
   - `panel-sheet.test.ts`'s "with no sheet declared the panel draws no tab
     and one page of Values, Conditions and Abilities";
     `qa-panel-sheet.test.ts`'s "no sheet: one page of Values, Conditions,
     Abilities, no tab" and "no sheet: each section with nothing to show is
     left out"; `sheet.test.ts`'s "with no sheet declared, one page of
     Values, Conditions and Abilities".
9. [x] **The panel names its character.** The `h2` is the shown
   character's chip label, which is its name, else its id; else, with no
   chip naming it, its name on the fold; no heading while the panel shows no
   character.
   - `panel-sheet.test.ts`'s "the panel is headed by the name of the
     character it shows", "a panel showing no character draws no heading"
     and "a selection no character answers to draws no heading, and one no
     chip names is headed by its name"; `spectator-view.test.ts`'s "the
     panel's headings say what each section is" reads "Lera" where it read
     "Your turn" on `0ee160b`.
10. [x] **No numbers on a player's discs.** `tokensOnScene`
    (`client/src/view/grid.ts`) gives a disc its resources only when the
    window's namer is a DM's or an agent's; every window draws the dots.
    - `client/test/app.test.ts`'s "a window whose role is read as %s draws a
      disc's resource chips: %p, and its condition dots always", over a DM,
      an agent, a player, a spectator and a window whose role was never
      read; `client/test/grid.test.ts`'s "a window that is not a DM's or an
      agent's gives no disc a resource" and "EVERY condition is shown, in
      applied order"; `client/test/qa-chips-by-role.test.ts`; the e2e test
      of item 4.
    - Each "none" case fails on `7b0ace1`, Task 3's parent.
11. [x] **What stays.** The party-member chips, the target buttons, the
    arm-doors toggle and the Say form are drawn and behave as before,
    outside the page: every client test file `0ee160b..3851b70` leaves
    unchanged passes, `panel-offers.test.ts`, `qa-window-offers.test.ts`,
    `qa-panel-say.test.ts` and `qa-switch.test.ts` among them, in `task
    check`'s `check:ts-coverage`, which runs the client's tests and fails on
    any failure (item 12).
12. [x] **The gate.** `task check` is green on `b894e8e`, the ledger commit
    on top of `3851b70`. Its Go mutation step reports no unadjudicated
    survivor; the mutants it timed out and counted as killed all sit in
    code this ticket did not change (`internal/perceive`'s
    `projection.go`, `internal/sight`'s `sight.go`, `internal/mcp`'s
    `server.go`), `internal/perceive`'s and `internal/sight`'s reused from
    their last passing run. It
    judged no stored client mutation report: the last measured `09c55a9`,
    and it names this ticket as added since. Its `build:client` rebuilt the
    bundle and `check:drift` found nothing changed under `cmd/vtt/webdist`,
    so the committed bundle is the client's build. `task e2e` is green on
    `b894e8e`; the test it skips is `handover.spec.ts`'s "a network drop
    while the table plays on ends in a Reconnect", a `test.skip` on
    `0ee160b` too.

## What the rules became

The ticket's rules, after the sort (the plan's header carries its refusals
and merges):

- A ruleset may declare its sheet: VTT-824 (returned in order), VTT-825
  (its shape), VTT-829 (the schema describes it), VTT-830 (a field the
  format does not define is refused; succeeds VTT-627, withdrawn).
- A sheet naming what the ruleset lacks is refused: VTT-826; a repeat:
  VTT-827; an ability whose usage no section shows: VTT-828.
- The read surface serves it to every role: VTT-831; `[]` when none:
  VTT-832; `get_ruleset`'s description says so: VTT-833.
- The panel draws the declared pages and sections, else one page: VTT-837
  (tabs), VTT-838 (the page kept), VTT-839 (sections), VTT-840 (the
  undeclared page), VTT-843 (abilities by usage).
- The panel names each value, condition and ability in the ruleset's words:
  VTT-841, VTT-842, VTT-846.
- A resource ability's button tells current, max and cost: VTT-844.
- The panel is headed by its character: VTT-836.
- Chips only in a DM's and an agent's window, dots in every window:
  VTT-834, VTT-835.
- The newest answer sets the layout: VTT-845.

Every row from VTT-824 to VTT-846 cites tests; none is open.

## The deliberate breaks

Each was made by an edit, run, and reverted by its inverse edit, the file's
hash checked afterwards; each turned its test red.

- **Task 1:** Load not keeping the sheet; an empty page name accepted (also
  the schema walker's `sheet[0].name`); two pages of one name; a section of
  two kinds; the undeclared-key check skipped; a `spends` of a number
  property accepted; the repeat check skipped; the every-ability check
  skipped; the usage decoder accepting unknown fields; `sheet` left out of
  the schema; after the review, an empty `spends` refused without its
  position.
- **Task 2:** the sheet served to the DM alone; `sheet` left nil when none
  is declared; the sections served in reverse; no page served at all (also
  the verbatim test's guard); the description's clause on an empty sheet
  dropped.
- **Task 3:** chips keyed on true; chips keyed on false; dots drawn only
  where chips are; after the review, the actor guard dropped and the dots
  sorted by id.
- **Task 4:** a section's values sorted by key; an empty declared section
  kept; a key of the other type named by the declared name; conditions
  sorted by id; every offered ability in every abilities section; the
  undeclared page's values by key alone; its empty "Conditions" kept; after
  the review, a resource section taking an ability used at will whose usage
  carries a resource.
- **Task 5:** the heading "Your turn" again; tabs for a one-page sheet; the
  page reset at every paint; the targets drawn inside the page; current and
  max dropped from the title; the layout taken from every answer; a button
  labelled by its id; after the review, the page reset at a DM's chip
  click, at a player's switch of viewpoint and at a new answer, and the
  heading's fallbacks undone.

The break script's guard stopped the first form of the usage decoder's
break in Task 1 and of the targets' break in Task 5 before they were
applied, because the broken text already occurred in the file; each was
rerun as an edit that could be reverted exactly, and turned its test red.
The first form of Task 1's "Load not keeping the sheet" deleted the field
from `manifestJSON` and broke the build, so no test ran; it was rerun as
`loadV2` leaving `rs.Sheet` unset, which turned
"TestADeclaredSheetLoadsInTheOrderDeclared" and
"TestTheShippedRulesetsLoadTheSheetTheyDeclare" red.

## Rule 9: how MapTool does this

The ticket's paragraph, checked against `~/dev/RPTool` by an agent before
the ticket was written and again by the plan's verifier. MapTool's core
has no character sheet: a token's properties reach a player on the hover
stat sheet (`StatSheetListener`, `StatSheet`), filtered per property by
"show on stat sheet", GM-only and owner-only flags (`TokenProperty`); the
Selected and Impersonate panels show macro buttons and no values; a
framework builds a sheet as an HTML frame a macro opens (`html.frame5`),
docked and redrawn when the selected or impersonated token changes. The
owner's 4.5e framework has none built (`~/dev/RPTool/Documentation/overview.md`
plans a power panel of power cards and conditions), and his paper sheet for
play (`~/dev/RPTool/Ursus_250907.pptx`, slide 10) groups powers by how they
are used beside the hit points and the current conditions. Borrowed: a
panel always shown that follows the character played, abilities grouped by
how they are used, and a value's explanation on hover. Not borrowed: the
per-property flags, since sight decides what a window holds.

## The plan's gaps and the owner's answers

- **G1 to G5** (a path, files, SPEC-018, VTT-627, rules the ticket's list
  lacked) were closed in the ticket by its writer before sign-off; **G6**
  (the undeclared page) was answered by OD-6; **G7** (a condition named by
  its id on a dot's title and in the Story's and the Events panel's lines)
  is a ticket of its own after the ability's description.
- **Signed off 2026-11-17**, "alla A, med din ändring av OD-8": OD-1 A (a
  section shows exactly one of `values`, `conditions: true` and
  `abilities`), OD-2 A (abilities grouped by usage), OD-3 A (no repeats;
  every ability's usage shown; a property need not stand), OD-4 A
  (`format_version` stays "3"), OD-5 A (the controls outside the tabs),
  OD-6 A (the undeclared page), OD-7 A (the title's words), OD-8 A with a
  section "Uses" showing `flurry_uses` before "Encounter powers".

## Phase 4a adjudications

Each task's QA agent derived its tests from the rows without reading the
code; none found a behaviour defect.

- Task 1, QA finding 1: spec ambiguity — "naming the file and the field" —
  a decode fault names the field by its name (`json: unknown field "x"`), a
  check's fault by its path (`sheet[1].sections[1].values[0]`), as the
  manifest did; goes into SPEC-024.
- Task 1, QA finding 2: spec ambiguity — "compared without regard to case"
  — a case variant of a defined field is read as it, as `encoding/json`
  reads the manifest; QA's reading kept; goes into SPEC-024.
- Task 1, QA finding 3: spec ambiguity — `conditions: false` — refused as a
  section showing none (VTT-825); held by both test files.
- Task 1, QA finding 4: spec gap — a `spends` of a resource no ability
  spends — it loads and its section shows nothing; goes into SPEC-024.
- Task 1, QA finding 5: spec gap — `"sheet": null`, and a kind's list
  present as `null` beside another kind — read as absent; goes into
  SPEC-024.
- Task 1, QA finding 6: a key twice within one list — refused (VTT-827); no
  change.
- Task 2, QA finding 1: spec ambiguity — a `conditions` section on the route
  — carries its name and kind alone (D5); QA's reading kept; goes into
  SPEC-012.
- Task 2, QA finding 2: spec gap — the order within a section's list — held
  by both test files and not by VTT-831's words; goes into SPEC-012.
- Task 2, QA finding 3: VTT-833 untested by QA — the tool list cannot be
  reached from `internal/mcp`'s exported surface without its source; held
  by `internal/mcp/ruleset_tool_test.go`.
- Task 2, QA finding 4: "served whole to every role" (D6) — held by VTT-831;
  no new row.
- Task 3, QA finding 1: spec gap — SPEC-022 said a disc's chips are drawn
  in every window — the rows win; goes into SPEC-022.
- Task 3, QA finding 2: spec gap — chip order by key — held by
  `qa-property-names.test.ts`'s "chips are ordered by key, not by display
  name or insertion", now in a DM's window; no new row.
- Task 3, QA finding 3: spec gap — "the order applied" — the fold's list
  order, which `tokensOnScene` does not re-sort; goes into SPEC-022.
- Task 3, QA finding 4: a window whose `/api/me` read fails — its role was
  never read, and it draws no chips (VTT-834); no change.
- Task 3, QA finding 5: untested — a role re-read at a presence frame
  turning chips on or off — recorded below; no new row.
- Task 3: the controller switched QA's file to `client/test/support/dom.ts`.
- Task 4, QA finding 1: spec gap — the undeclared page's name — "" in the
  model and never shown, since that page draws no tab (VTT-840); no row.
- Task 4, QA finding 2: spec gap — a value's name, text and title apart in
  the model — the panel draws "<name> <value>"; held by the panel's tests.
- Task 4, QA finding 3: spec gap — a key held as both an attribute and a
  resource, named in a section — both drawn, the attribute first; goes into
  SPEC-022.
- Task 4, QA finding 4: spec gap — a repeat, or an ability no section shows
  — cannot reach the client from a loaded ruleset (VTT-827, VTT-828); the
  model does not guard them.
- Task 4, QA finding 5: spec gap — an ability whose resource the character
  lacks — drawn in its section, its button disabled, as before.
- Task 4, QA finding 6: the tab decision — the panel's (VTT-837, VTT-840).
- Task 4: the controller typed QA's fixture builders, which cast past the
  client's types.
- Task 5, QA finding 1: spec gap — SPEC-022 described neither the heading,
  the tabs nor the sections, and gave the old title — goes into SPEC-022.
- Task 5, QA finding 2: spec gap — where undeclared keys stand on the
  undeclared page — after the declared properties, by key (D13); goes into
  SPEC-022.
- Task 5, QA finding 3: as Task 4's finding 3.
- Task 5, QA finding 4: as Task 4's finding 4.
- Task 5, QA finding 5: spec ambiguity — the at-will title's words — "at
  will" exactly (D16); held by `panel-sheet.test.ts`.
- Task 5, QA finding 6: the DM's heading — from the chip its panel draws
  for a controlled actor; goes into SPEC-022.
- Task 5, QA finding 7: as Task 3's finding 3.

## Phase 4b

One reviewer per task read each task's diff before its commit.

- **Task 0:** VTT-827 as first worded would have refused the signed-off
  sheet (a key shown as a value and in the usage that spends it); VTT-837,
  VTT-839 to VTT-844 were ambiguous or short of their decisions; the
  ticket's rule naming an ability by the ruleset's words had no row. All
  were worded again before the commit, their ids unchanged (SPEC-008: a row
  is spent once committed), and VTT-846 was allocated.
- **Task 1:** an empty `spends` was refused without its position; QA's
  refusal helper matched words in the temporary directory's path; the
  schema described the client before it existed.
- **Task 2:** the verbatim test's guard failed its tavern-brawl subtest run
  alone; a comment described rather than warned.
- **Task 3:** the new test sat under a helper's doc comment; the
  vanished-actor tests went vacuous once chips depended on the role; the
  dot-order fixture was already sorted.
- **Task 4:** a QA comment broke `check-new-prose`'s column band;
  `SheetUsage` became a union on `kind`, and the model's filters became
  switches over the kinds; tests became exact lists.
- **Task 5:** VTT-838's switch of character was held only by a test that
  assigned the selection; the heading was drawn from an id for a selection
  no character answers to. Both were fixed with tests seen red.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Rows worded once, at allocation | VTT-827, VTT-837, VTT-839 to VTT-844 worded again before Task 0's commit; VTT-846 added | Task 0's review found them ambiguous; SPEC-008 spends a row at its commit, not at its allocation |
| The route's tests in `internal/gateway/metadata_test.go` | A new `internal/gateway/sheet_route_test.go`, with a fixture serving `tavern-brawl` beside `newMetaFixture` | The tests needed a fixture of their own, since `newMetaFixture` serves `dnd45e-minimal` or nothing, and a file of their own kept that fixture beside the tests that use it |
| `cmd/vtt/qa_ruleset_by_role_test.go` holds the description | `internal/mcp/ruleset_tool_test.go` does | It reads the description through an MCP session already; a second check would assert the same words |
| `tokensOnScene`'s comment rewritten | Cut to a pointer and a warning | `tools/check-comments.py` refuses a touched comment block over its bound (SPEC-010); its account moves here, below |
| Panel tests in `spectator-view.test.ts`, `app.test.ts`, `qa-window-reread.test.ts` | A new `client/test/panel-sheet.test.ts`; VTT-845's beside VTT-753's in `app.test.ts` | The panel's sheet tests share one fixture; `qa-window-reread.test.ts` is an earlier QA agent's file, and the implementer's tests stay out of QA's files |
| `app.test.ts`'s "the ruleset's properties reach the chip and the cost, and the Story draws no resource change" reads its chip in a DM's window | The whole test boots a DM's window | The chip is drawn in a DM's and an agent's window alone (VTT-834); the player's cost stays held by `qa-property-names.test.ts` and `qa-older-answer.test.ts`, and the player's Story by `app.test.ts`'s "every role's Story draws what happens in the game and no mechanical entry, and the DM's and the agent's Events panel draws each" |
| The heading from the chip, else the id | From the chip, else the fold's name, and only while a character is shown | Task 5's review: a DM whose selected actor was revoked or removed saw the id |
| The e2e test independent | It reads the session, the adventure and the fighter's grant of the tests before it, and says so | Granting a character the e2e test can read the sheet of is what the test before it does |

**What `tokensOnScene`'s comment said, and where it went** (readable at
`7b0ace1:client/src/view/grid.ts`). Its first paragraph said every
resource and condition is drawn, since choosing a primary one would need
client hints the format lacked; that was made false by this ticket and was
dropped. The rest argued that `visible` is the server's own set since
2026-08-22 (`SceneSeen` field 4), so the filter is a consistency check
against that answer rather than a visibility decision, kept because it
costs one lookup and guards a silent failure, a creature drawn on ground
the seat only remembers; that the earlier rationale, the last line against
a leak, held only while the set was derived from terrain, which hid a
player's own token on a scene with no tiles; that RPTool reached the same
seam and then handed its renderer the full list anyway; and that the
parameter is required so no call site falls into the draw-everything answer
by omission. The comment keeps the warning on an undefined set, pointing at
`Scene.Visible` in `client/src/state.ts`.

## What could not be established

- **The client's mutation.** `task check` judges no stored client mutation
  report: the last full run measured `09c55a9`, and this branch adds client
  code. Task 4's reviewer expects `byKey` (`client/src/player.ts`) to
  survive as `<` → `<=`, equivalent because its keys come from a `Set` and
  never tie, and as `a > b` → false, equivalent because the sort reads only
  whether the comparator is negative. A full run is the owner's to ask for
  (SPEC-023).
- **The schema walker's skipped subtests.** It probes the first section and
  the first usage alone, so its subtests for `sheet[0].sections[0]`'s
  `conditions` and `abilities` and for a usage's `spends` skip;
  `declared_sheet_test.go` and `qa_declared_sheet_test.go` hold those
  refusals directly.
- **A role re-read at a presence frame.** That a window whose role changes
  without a reload starts or stops drawing chips is untested; `paint`
  reads the role at every paint.
- **A condition's name elsewhere** (G7): a dot's title and the Story's and
  the Events panel's lines still name a condition by its id.
- **A companion's values.** A player sees a seen companion's values
  nowhere; the owner accepted this on 2026-11-17.
- **The DM's view of a sheet.** The DM's and the agent's windows draw the
  panel only for an actor they control.

## What was deliberately left out

- An ability's description: the next ticket, by the owner's ruling of
  2026-11-17.
- A condition named by its id on a dot and in the Story and the Events
  panel (G7): a ticket after that one.
- A view of a companion's sheet from the board: "kanske senare", the
  owner's ruling of 2026-11-16.
- A layout declared by a campaign, and changing a value from the sheet:
  out of the ticket.
