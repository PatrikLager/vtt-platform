# Plan: a player's panel is their character's sheet, laid out as its ruleset declares

- **Ticket:** `docs/superpowers/specs/2026-11-17-the-player-panel-is-the-sheet-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-17 by an agent that did not write the ticket,
    read-only against `0ee160b` on `feat/the-player-panel-is-the-sheet` in
    the worktree `vtt-platform-visibility`, with the ticket added with
    intent-to-add and uncommitted.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e` and Stryker were not run. P1 to P7 ran in copies of the tree
    at `0ee160b` outside the repository, never in the worktree.
- **To be committed as:** `docs/superpowers/plans/2026-11-17-the-player-panel-is-the-sheet.md`.
- **Base:** `0ee160b`. **Branch:** `feat/the-player-panel-is-the-sheet`.
- **Owner's rulings taken in:**
  - 2026-11-17: the player's panel IS the character's sheet; its layout
    (pages shown as tabs, sections, which values each shows) is declared
    in the ruleset, not the campaign; the client draws only what is
    declared and invents no grouping ("Regelmotorn bör helst bara läsa upp
    och visualisera vad som är beskrivet"); a ruleset declaring no layout
    is drawn as one page.
  - 2026-11-17: a player's and a spectator's window draw no resource chip
    on a disc; the DM's and the agent's keep them; the condition dots stay
    in every window.
  - 2026-11-17: an ability's description is the next ticket, not this one.
  - 2026-11-17: one ticket for the format, the read surface, the MCP tool
    and the client, delivered as separately committed tasks.
  - 2026-11-17: a player sees a seen companion's values nowhere once the
    chips go; accepted, a companion's sheet is later work.
  - 2026-11-06: sight decides what a window holds; no per-property
    visibility flag.
  - 2026-09-04: nobody uses the product; no migration.
  - Standing: no counts or measured figures in anything written down; push
    and merge on the owner's word.
- **Signed off:** 2026-11-17, "alla A, med din ändring av OD-8": OD-1 to
  OD-7 A, and OD-8 A as amended by the ticket's writer (a section "Uses"
  showing `flurry_uses` before "Encounter powers"). G7 (a condition named by
  its id on a dot's title and in the Story's and the Events panel's lines)
  is a ticket of its own after the ability's description, on the writer's
  recommendation the owner's "alla A" took in.
- **Ids:** allocated at Task 0. The `R` labels are this plan's: R1
  VTT-824, R2 VTT-825, R3 VTT-826, R4 VTT-827, R4b VTT-828, R5 VTT-829, R7
  VTT-830, R8 VTT-831, R9 VTT-832, R10 VTT-833, R11 VTT-834, R13 VTT-835,
  R14 VTT-836, R15 VTT-837, R16 VTT-838, R17 VTT-839, R18 VTT-840, R19
  VTT-841, R21 VTT-842, R22 VTT-843, R23 VTT-844, R24 VTT-845, R25
  VTT-846. Task 0's reading review found VTT-827, VTT-837 and VTT-839 to
  VTT-844 ambiguous or short of their decisions, and they were worded again
  before Task 0's commit, their ids unchanged (SPEC-008: a row is spent
  once committed); it found the ticket's rule naming an ability by the
  ruleset's words neither accepted nor refused, and R25 was allocated for
  it.
- **The sort's refusals:** R6 (an unknown field inside `sheet`), merged
  into R7, one rule that the format refuses a field it does not define;
  R12 (the DM's and the agent's chips), merged into R11, which says when a
  chip is drawn and when none is; R20 (an undeclared key named by its key),
  merged into R19, one rule for how a value reads; R5's half that the loader
  refuses a field the schema marks required, held by R2 and by
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` for every field; R22's
  half that arming and targeting work as today, held by VTT-436, VTT-439
  and VTT-441; a ruleset without `sheet` loading, held by every ruleset and
  fixture that loads today; a property the sheet leaves out (an absence,
  which nothing could turn red but a refusal, and R1's `dnd45e-minimal`
  leaving out `max_hp` shows it); `dnd45e-minimal`'s pages (content, not a
  rule); no generated artefact changing (scenery). R4 was split: R4 holds
  the repeats and R4b the ability whose usage no section names; a `spends`
  naming no resource moved from R4 to R3, beside the undeclared key.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists:
   `client/src/view/player.ts`, `client/src/player.ts`,
   `client/src/view/grid.ts`, `client/src/view/spectator.ts`,
   `internal/gateway/metadata.go`, `client/src/metadata.ts`,
   `client/src/app.ts`, `internal/rules/load.go`,
   `internal/rules/format.go`, `internal/rules/schema/ruleset.schema.json`,
   `internal/rules/schema_test.go`, both rulesets' `ruleset.json`,
   `internal/gateway/metadata_test.go`, `internal/mcp/ruleset_tool.go`,
   `cmd/vtt/qa_get_ruleset_test.go`, `client/src/style.css`,
   `cmd/vtt/webdist`, `docs/requirements.md` and SPEC-012, SPEC-022 and
   SPEC-024. Every symbol is where the ticket says: `renderPlayerPanel`,
   `offered`, `tokensOnScene`, `renderDisc`, `describe`, `RulesetMeta`,
   `handleRuleset`, `rulesetJSON`, `manifestJSON`, `loadManifest`,
   `decodeStrict`, `checkPropertyKey`, `checkFormatVersion`,
   `compositionJSON`, `Ruleset`, `getRulesetDescription`. MapTool's
   `StatSheetListener`, `StatSheet`, `TokenProperty` (its `ownerOnly`,
   `gmOnly` and `highPriority` flags), `SelectionPanel`,
   `ImpersonatePanel` and `frame5` (`HTMLFrame`) exist, and so does
   `~/dev/RPTool/Ursus_250907.pptx`. One path does not resolve as written
   (G1).
2. **"Done" is an observation. By command for items 1, 7, 9 and 10, by
   reading for the others.** Item 1: today's `rules.Load` refuses a
   `dnd45e-minimal` that declares a sheet with `json: unknown field
   "sheet"` (P1). Items 9 and 10: the tests that pin today's heading and a
   player's chips pass today (P5, P4), so tests asserting the new heading
   and no chip fail today. Item 7's title: the tests pinning today's words
   pass today (P6). Items 3 to 6 and 8, by reading `handleRuleset` and
   `renderPlayerPanel`: today the route writes no sheet and the panel draws
   no tab, value or condition. Item 11 names tests that pass today and stay
   green (P4 to P7 found none of them red). Item 12 is the gate.
3. **Each rule is breakable. By reading.** Each of the ticket's rules has an
   observation that fails when it is broken: a ruleset declaring a sheet
   refused or its order lost (rule 1); a sheet naming an undeclared
   property loading (rule 2); a role answered no sheet (rule 3); a section
   the ruleset did not declare drawn, or a tab with no sheet declared (rule
   4); a key shown where the ruleset gives a name, or no description on
   hover (rule 5); a resource button's title without current and max (rule
   6); a chip on a player's or a spectator's disc (rule 7). The ticket's
   items carry rules its list does not (G5); R1 to R24 below hold them all.
4. **The scope matches the claim. By command, then reading.** The callers:
   `renderPlayerPanel` from `paint` in `client/src/app.ts` and from tests
   across the client suite; `tokensOnScene` from `renderGrid` alone, and
   from `grid.test.ts`, `qa-property-names.test.ts` and
   `visibility.test.ts`; `renderDisc` from `renderGrid` alone;
   `rulesetJSON` in `handleRuleset` alone; `loadManifest` from `Load`
   alone; `fetchRuleset` from `app.ts`. The production files the work
   changes are the ticket's list. The tests and fixtures it reaches are
   wider (G2), and so are the records (G3, G4). The ticket says "the client
   tests that draw the panel or a disc's chips", so this is a gap, not a
   return.
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-022's "A disc's resource chip shows `current/max`", for every
     window, and the "ALL resources and ALL conditions" stance in
     `tokensOnScene`'s and `renderDisc`'s comments (from the client
     design's §4 and §9, "client-hints the format does not have"): the
     ticket overturns both by the owner's ruling, and names SPEC-022.
   - SPEC-024's manifest of `id`, `name`, `format_version` and
     `properties`: named. VTT-627, which lists the same four fields, is
     made false and the ticket does not name it (G4).
   - SPEC-012's "answer with every slice non-nil": kept by D5 and R9.
   - Blueprint 13: the dots' titles and the Story's and the Events
     panel's condition lines name a condition by its id; the ticket names
     this and raises it at sign-off (G7).
   - Blueprint 27 and 35, VTT-691: not contradicted under OD-2 A; OD-2 B
     would serve a player ability ids `/api/ruleset` withholds from them.
6. **The records the work moves are named. By command, then reading.** The
   section is present, each of its paths resolves, and nothing is `New:`. By
   search (`grep -ln` over `docs/specifications/` for the player panel,
   `renderPlayerPanel`, `tokensOnScene`, a resource chip, `get_ruleset`,
   `/api/ruleset`, `ruleset.json` and `loadManifest`), SPEC-018 moves too:
   it lists the unbounded strings of a ruleset file and those
   `/api/ruleset` carries to every participant, and page and section names
   join both (G3). SPEC-025 says "the panel asks `sheetNames`", which stays
   true; SPEC-011 and SPEC-007 name the route and do not change.

**Split: not offered.** The owner ruled one ticket, delivered as separately
committed tasks (Tasks 1 to 5).

## The gaps this plan inherits

The ticket's writer closed G1 to G5 in the ticket before sign-off; OD-6
answers G6, and G7 travels. The verdict above and the gaps below are the
verifier's record of the ticket as it was verified.

- **G1. The framework's overview is not under `rule-framework`.** The
  ticket's MapTool paragraph places `Documentation/overview.md` under
  `~/dev/RPTool/rule-framework`; it is `~/dev/RPTool/Documentation/overview.md`,
  which documents that framework and plans `powerPanel.html`, "power card +
  active conditions", as the ticket says. The evidence holds at the right
  path; the paragraph is the writer's to correct.
- **G2. Tests and fixtures the ticket does not name.** P3 to P6 name every
  test that turns red. Beyond the ticket's list the work reaches
  `internal/rules/testdata/valid/ruleset.json` (the schema walker's
  fixture, D20), `internal/mcp/ruleset_tool_test.go` and
  `cmd/vtt/qa_ruleset_by_role_test.go` (the description's tests), a new
  test file in `internal/rules`, `client/e2e/table.spec.ts` (Task 5's
  real-wire check) and, if a ceiling falls, `tools/comment-ceilings.txt`.
- **G3. SPEC-018 moves and is not named.** Page and section names are
  strings of a ruleset file its loader does not bound, and they reach every
  participant through `/api/ruleset`. Task 7 adds them to SPEC-018's
  sentence.
- **G4. VTT-627 is made false and is not named.** It refuses every field
  but `id`, `name`, `format_version` and `properties`; `sheet` loads after
  Task 1. Withdrawn in Task 1, succeeded by R7. The ticket's item 4 under
  "What it touches" names rows "that say a disc draws its chips in every
  window": by search of `docs/requirements.md` for chip, disc, dot,
  `current/max` and hover, no row says so; SPEC-022 and the code's comments
  do. The rows whose evidence moves are VTT-809, VTT-815, VTT-816 and
  VTT-817 (below).
- **G5. Rules the ticket's list does not carry.** The heading (item 9), the
  DM's and the agent's chips and every window's dots (item 10), the
  description of `get_ruleset` (item 3), the refusals of item 2 beyond an
  undeclared property, and the newest answer's hold on the layout. R1 to
  R24 carry them.
- **G6. "No grouping of its own", and the page drawn for none declared.**
  The undeclared page must be arranged somehow; OD-6 decides how, in the
  platform's words, and R18 holds it.
- **G7. The ticket's "What could not be established" travels:** a
  condition is named by its id on a dot's title and in the Story's and the
  Events panel's lines (`tokensOnScene`, `describe`), against principle 13,
  raised to the owner at sign-off; a player sees a seen companion's values
  nowhere; the DM's and the agent's windows show the sheet only of an actor
  they control.

## Measurements this plan stands on

- **P1. Today's loader refuses a declared sheet.** In a copy of the tree at
  `0ee160b`, `dnd45e-minimal/ruleset.json` given a `sheet`, `rules.Load`
  answered `rules: rulesets/dnd45e-minimal/ruleset.json: json: unknown
  field "sheet"`.
- **P2. A sheet in `dnd45e-minimal` turns no Go test red.** In the same
  copy, with a probe loader that decoded a `sheet` and checked its keys
  against the declared properties, `go test` of `./internal/rules/...`,
  `./internal/gateway/`, `./internal/adventure/`, `./internal/mcp/`,
  `./internal/campaign/...`, `./cmd/vtt/` and `./tools/...` passed.
- **P3. A section's key named `properties` collides with the fixtures.**
  With a sheet added to `internal/rules/testdata/valid/ruleset.json` whose
  sections listed keys under `"properties"`,
  `TestARulesetWhoseIDOrNameExceedsTheBoundIsRefused`,
  `TestARulesetWhoseIDsAndNamesAreExactlyTheBoundLoads` (both
  `load_test.go`) and `TestARulesetAtTheBoundResolvesToEventsTheFoldAccepts`
  (`resolve_test.go`) turned red: `numberAt` rewrites every `"properties":
  [` in the file as text, the sheet's included. With the key named
  `values`, only `TestSchemaPropertiesCoverFixtureFields` turned red, as it
  must until the schema declares `sheet`.
- **P4. What no chip on a player's and a spectator's disc turns red.** In
  a copy of the client at `0ee160b` (the suite green there first),
  `tokensOnScene` gave a disc no resource unless its namer was a DM's.
  Red: `grid.test.ts` "EVERY resource is shown, in a stable order, with
  current and max"; `qa-property-names.test.ts` "a declared resource's chip
  is titled by its name, face current/max", "a number property does not
  name a resource chip of the same key", "an undeclared resource's chip is
  titled by its key", "chips are ordered by key, not by display name or
  insertion", "tokensOnScene labels a declared resource's chip by its name"
  and "with no properties every chip is titled by its key";
  `spectator-view.test.ts` "every resource shows as a current/max chip,
  named on hover", "a token WITH them renders both containers", "a token's
  INNER content shrinks with the camera too, not just its outer box" and "a
  resource chip, its Events panel line and an ability's cost name the
  resource by its display name"; `qa-companion-first-sight.test.ts`
  "player: the fold holds the companion whole after the run" and
  "spectator: …"; `app.test.ts` "the ruleset's properties reach the chip
  and the cost, and the Story draws no resource change". Nothing else.
- **P5. What the heading turns red.** The panel's `h2` made the selected
  chip's label, and no `h2` with no character: `spectator-view.test.ts`
  "the panel's headings say what each section is" alone.
- **P6. What the button's new title turns red.** A resource ability's title
  made `<resource> <current>/<max>, costs <cost>` and ` — not enough` as
  today: `qa-property-names.test.ts` "a cost exactly equal to what the
  actor holds is affordable", "a number property does not name a cost of
  the same key", "an affordable cost names the resource with no
  shortfall", "an unaffordable cost names the resource and says not
  enough", "an undeclared resource's cost is named by its key" and "with no
  properties every cost is named by its key"; `spectator-view.test.ts` "a
  resource ability naming no resource names no resource in its cost", "an
  affordable resource ability names its cost without the shortfall note"
  and the display-name test P4 names; `qa-older-answer.test.ts` "an older
  read's answer arriving after the newest does not change how the player
  panel's ability button names the resource it spends"; and `app.test.ts`'s
  test P4 names.
- **P7. The undeclared page turns nothing red.** The panel given a values
  section (`h3` "Values") and a conditions section (`h3` "Conditions")
  before today's abilities block, in every window: the suite stayed green,
  `qa-window-number.test.ts` and `qa-what-a-window-shows.test.ts` among it.
- **P8. MapTool (rule 9), the grouping.** `AbstractMacroPanel` and the
  `buttongroups` (`AreaGroup`) draw a token's macro buttons in areas keyed
  by `MacroButtonProperties`' `group`, a free label each macro carries;
  `Tab` holds the Global, Campaign, GM, Selected and Impersonate panels,
  not pages of a character.
- **P9. The e2e Say check reads the panel's attributes.**
  `client/e2e/refusals.spec.ts` "a DM granted a character draws its chip,
  and the console's Say alone" matches `/\bsay|speak|narrat|\booc\b|\/ooc|console|\bsend\b/i`
  against the DM's panel's text and every attribute. Of
  `dnd45e-minimal`'s property, condition and ability words, `dazed`'s
  description matches (`narration`). By reading, the spec applies no
  condition to its cutter; Task 6's `task e2e` is the observation.
- **P10. The vocabulary gate.** `.semgrep/vocabulary.yml`
  (`no-game-system-vocabulary-in-engine`, Go under `internal/` and `cmd/`)
  refuses `\bhp\b`, `hit_?points`, `bloodied`, `fortitude`,
  `encounter_?power`, `at_?will_?power` and others, case-insensitively.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no assertion is weakened to pass: a chip
  assertion in a player's window becomes an assertion of none, with the
  value asserted on the fold or the DM's disc; a title assertion takes the
  new words, never a looser match); 3 (the protobuf contract does not
  change: `/api/ruleset` is JSON, outside `contract/`); 4 (the fold is
  untouched: the panel reads `st.Actors` and `st.Conditions`); 5 (no game
  word in Go under `internal/` or `cmd/`, tests included, P10: a Go test
  reads `dnd45e-minimal`'s sheet from its `ruleset.json`, as
  `declaredKeys` in `internal/gateway/ruleset_properties_test.go` reads
  its keys, and builds its own with neutral words); 8 and 10 (names, never
  line numbers; a comment is a warning or a pointer; every comment this
  work makes false is rewritten: `tokensOnScene`'s first paragraph,
  `renderDisc`'s "ALL resources, ALL conditions", the `.chips` rule's
  comment in `style.css`, and the "Abilities" block's comments in
  `view/player.ts`); 9 (answered below).
- **Blueprint principles 13, 23, 25, 27 and 35.** 13: the sheet names each
  value, condition and ability by the ruleset's words, an id only where no
  name exists. 23: page and section names are the ruleset's data; the
  platform knows "values", "conditions" and "abilities" only. 25: every
  fault of a sheet is refused at load (R2 to R4b). 27: the sheet names
  property keys and usages, never a copy of an ability (OD-2). 35: the
  sheet is served to every role (R8).
- **SPEC-012** (every list non-null, the conditions and properties every
  role's), **SPEC-018** (page and section names are unbounded, as every
  ruleset string outside its table), **SPEC-024** (the strict decode, the
  refusal's form `rules: <path>: field "<field>": <message>`), **SPEC-022**
  (the newest answer sets what the panel offers; the tree is rebuilt at
  every paint).
- **SPEC-020, SPEC-023.** No adjudicated mutant in
  `tools/ts-mutation-equivalents.txt` sits on code this changes: `grid.ts`'s
  sit on the final sort, which D8 does not edit. The client's stored
  mutation report goes out of date with Task 3's commit; `task check` names
  that and passes; a full run is the owner's to ask for. The Go mutation
  gate judges the new loader and route code in Task 6.
- **Principle 33, as it stands.** `/api/ruleset` is described twice, by
  `rulesetJSON` and by `RulesetMeta`; this work adds the sheet to both, and
  Task 5's end-to-end check is what holds them together on the real wire.
- **The project's mechanical gates:** a client change is committed with its
  `cmd/vtt/webdist`, rebuilt with `task build:client`; client tests run
  with `env -u CLAUDECODE`; `task client:typecheck`;
  `tools/check-new-prose.py` and `python3 tools/check-comments.py` before
  each commit; stage and commit in separate calls; Go tests scoped to the
  packages a task touches until Task 6.

## Rule 9: how MapTool does this

The ticket's paragraph, verified (check 1), and P8:
- MapTool's core has no character sheet. A token's properties reach a
  player on the hover stat sheet, filtered per property by flags; the
  Selected and Impersonate panels show macro buttons, grouped by a label
  each macro carries, and no values; a framework builds a sheet as an HTML
  frame.
- **Borrowed:** a panel always shown that follows the character played;
  abilities grouped as the game groups them; an explanation on hover.
- **Not borrowed:** per-property flags (sight decides what a window holds);
  a group label on each ability (OD-2 C), since the sheet can group by the
  usage every ability already declares, with no change to the ability
  format, which the next ticket opens.

## Decisions

1. **D1. The declaration is an optional `sheet` in `ruleset.json`**: a list
   of pages, each `{"name", "sections"}`, each section `{"name"}` and
   exactly one of `values` (property keys), `conditions` (`true`) or
   `abilities` (usages), under OD-1 A, OD-2 A and OD-4 A. In
   `internal/rules/format.go`, `Ruleset.Sheet []SheetPage`, nil when none
   is declared; `SheetPage` holds `Name` and `Sections []SheetSection`;
   `SheetSection` holds `Name`, `Values []string`, `Conditions bool` and
   `Abilities []ShownUsage`; `ShownUsage` holds `AtWill bool` and
   `Resource string`.
   - **Forced by** SPEC-024 ("a new field on a property … a change to the
     loader and the schema"), and a field on the manifest is decoded by the
     same `decodeStrict`; the field's name is the ticket's and the owner's
     word. The Go names are a preference: `SheetFault` and `SheetKeyFault`
     already speak of an actor's sheet in the same package, and a `Sheet`
     prefix on a type says which sheet.
2. **D2. Shape in `loadManifest`, references last in `loadV2`.**
   `manifestJSON` gains the `sheet` field, decoded by `decodeStrict`, so an
   unknown field anywhere in it is refused and VTT-627's case rule holds; a
   usage decodes through its own `UnmarshalJSON`, `"at_will"` or
   `{"spends": "<key>"}`, strictly, as `Usage` does. A new `checkSheet`,
   in `load.go`, runs after `compileCompositions` and before `Load`
   returns, and checks names, kinds, keys, duplicates and every ability's
   usage.
   - **Forced by** the usage check, which reads each ability's usage from
     `rs.Compiled`, after `compilePower` has checked its resource; and by
     SPEC-024's refusals, every one of which keeps its message and its
     order when the sheet is checked last. In `load.go` and not a new file:
     a new file takes a comment-ceiling row of its own (a preference).
3. **D3. A refusal names the file as loaded and the field,** through
   `fieldErr`: `sheet`, `sheet[i].name`, `sheet[i].sections`,
   `sheet[i].sections[j].name`, `….values[k]`, `….abilities[k]`, and, for
   an ability whose usage no section names, the ability's id in the
   message on the field `sheet`.
   - **Forced by** item 2 and SPEC-024's form. `crossValidateResources`
     names `ruleset.json` bare; `checkSheet` is handed the path `Load`
     opened, as `loadManifest`'s refusals name it.
4. **D4. Page and section names are unbounded.**
   - **Forced by** SPEC-018: a ruleset string its loader does not bound is
     under the same decision as the display names.
5. **D5. The route:** `rulesetJSON` gains `Sheet []sheetPageJSON` as
   `sheet`, starting as an empty slice; a page is `{"name", "sections"}`, a
   section `{"name", "kind", …}` with `kind` `values` and `values`, `kind`
   `conditions`, or `kind` `abilities` and `abilities`, each a `usageJSON`
   with no cost (`{"kind": "atWill"}` or `{"kind": "resource", "resource":
   <key>}`).
   - **Forced by** SPEC-012 (never `null` where a client expects a list);
     `Ability.usage` already speaks `atWill` and `resource`, so the client
     compares a usage with a section by the same words; a `kind` lets
     `SheetSection` in `metadata.ts` be a union that a switch with no
     `default` covers, so a kind added to the route fails `task
     client:typecheck` until the panel draws it (as `describe` is held).
6. **D6. The sheet is served whole to every role.**
   - **Forced by** OD-2 A: the sheet names property keys and usages, which
     every role already reads (VTT-619, VTT-631), and no ability id, so
     `abilitiesReadBy` (VTT-691) has nothing to filter.
7. **D7. `get_ruleset`'s description gains a sentence;** no generated
   artefact changes.
   - **Forced by** `get_ruleset` being Go-registered:
     `TestQARRGetRulesetIsListedAndTakesNoArguments` asserts it absent from
     the contract manifest, so `contract/gen/tools/tools.json`,
     `cmd/vtt/tools.json` and `task generate:contract` are untouched. Its
     answer carries the sheet with no code change, since it returns the
     route's body verbatim (VTT-621).
8. **D8. No chip is decided in `tokensOnScene`, by `names.dm`.** A disc's
   `resources` is empty unless its window's namer is a DM's or an
   agent's; `renderDisc` draws what it is handed, and only its comment
   changes.
   - **Forced by** `spectator.ts`'s header (every decision lives where it
     is unit-tested without a browser) and by `Namer.dm`, which is already
     the window's role bit (`ViewExtras.dm`, set from `isDM` in `paint`).
9. **D9. The panel's model is a pure function in `client/src/player.ts`**
   (`sheetFor`, from the character, its conditions, the window's
   abilities, properties, conditions and sheet), which `renderPlayerPanel`
   draws.
   - **Forced by** `view/player.ts`'s header: every rule the panel applies
     lives in `player.ts` as a tested pure function.
10. **D10. The panel's order:** the heading; the party-member chips' row;
    "That actor has no token on the board yet." when it has none; the tabs;
    the shown page's sections; then, outside the tabs, the target buttons or
    "Click the board to move.", the arm-doors toggle and the Say form
    (OD-5 A).
    - **Forced by** the tests that read the panel's first `.row` as the
      party chips (`app.test.ts`'s `chips`, `qa-perched-player.test.ts`,
      `qa-switch.test.ts`, `qa-reload-viewpoint.test.ts`), and those that
      read the row after the `h3` "Abilities" as the ability buttons
      (`qa-window-offers.test.ts`'s `qaWinAbilityButtons`).
11. **D11. Tabs:** a `div.tabs` of `button.tab`, the shown one `tab sel`,
    drawn only for a sheet of more than one page; the shown page is kept
    in `PlayerUIState.page` by its name, across repaints and switches of
    character, and a name the sheet does not declare shows the first page.
    - **Forced by** SPEC-022 (the tree is rebuilt at every paint, the UI
      state survives it); R2 makes a page's name unique and so a key; a
      single tab selects nothing. `tab`, not `chip`, keeps the
      `button.chip` locators of `client/e2e/table.spec.ts` and
      `client/e2e/refusals.spec.ts` on chips.
12. **D12. A section with nothing to show for the character is left out,
    its heading with it; every page's tab is drawn.**
    - **Forced by** VTT-695's evidence `panel-offers.test.ts` "an actor
      whose sheet names none is offered no ability and no Abilities
      heading", and by the layout being the ruleset's, the same for every
      character, so tabs do not come and go between characters.
13. **D13. Values.** A section draws its keys in its order; the
    undeclared page draws the window's properties in their order, then
    every key the character holds that they do not declare, in key order by
    `<`, an attribute before a resource of the same key. A key is declared
    only as the type its map says: a number property for `attributes`, a
    resource property for `resources`. Each value is a `.value` element:
    the property's display name, a space and the value (a number as itself,
    a resource as `current/max`), titled by the property's description; an
    undeclared key is named by the key and titled nothing.
    - **Forced by** VTT-619 (properties are served in the ruleset's order),
      `tokensOnScene`'s key sort for a dictionary, and VTT-816's rule that a
      `number` property names no resource.
14. **D14. Conditions.** `st.Conditions[actorId]` in the fold's order, each
    a `.condition` element named by the newest answer's `conditions` and
    titled by its description, or named by its id and titled nothing.
    - **Forced by** item 6, and `tokensOnScene`'s warning that the fold
      keeps the applied order and the view must not re-sort.
15. **D15. Abilities.** A section draws, from `offered(abilities, actor)`
    and in that order, the abilities whose usage it names, as today's
    buttons (class `chip`, disabled while unaffordable or tokenless, a
    click arming or disarming). The undeclared page's abilities section is
    today's block, `h3` "Abilities" and its `.row`.
    - **Forced by** VTT-695 and item 7; P7 shows today's tests hold the
      undeclared page as it stands.
16. **D16. A resource ability's title** is `<resource> <current>/<max>,
    costs <cost>`, the resource named by `Namer.resource`, then ` — not
    enough` when the character cannot afford it; with the resource not on
    the sheet, `<resource>, costs <cost> — not enough`. An at-will button
    keeps "at will" (OD-7 A).
    - **Forced by** item 7 and VTT-815 ("on an ability's cost" by the
      display name); the words are a preference the owner signs off.
17. **D17. The heading** is the selected chip's label (the name the
    switcher's set or the fold gives, else the id, VTT-404); with no
    character the panel draws no `h2`, and "You do not control an actor
    yet." as today.
    - **Forced by** one character named one way in one panel;
      `client/e2e/refusals.spec.ts` "a DM's accepted line empties say-text
      and keeps say-as for the next line" reads that line.
18. **D18. The window keeps the sheet and the conditions from the newest
    answer,** beside the abilities and the properties, under the same
    ticket in `readRuleset`, and hands them to `renderPlayerPanel` in one
    trailing optional parameter; an answer without either is read as none
    declared.
    - **Forced by** VTT-753 and SPEC-022's answers paragraph, which R24
      extends; every existing call of `renderPlayerPanel` compiles
      unchanged; the client suite's fake answers often omit a list, as they
      omit `properties` today.
19. **D19. `dnd45e-minimal` declares a sheet; `tavern-brawl` declares
    none** (OD-8 A for the content).
    - **Forced by** items 1 and 8.
20. **D20. The schema walker's fixture declares a sheet.**
    `internal/rules/testdata/valid/ruleset.json` gains a sheet of more than
    one page naming `pool_a`, `brawn`, `grit` and `guard`, the conditions
    and the fixture's usages, so `TestSchemaRequiredFieldsMatchLoaderEnforcement`
    exercises the sheet's required fields and
    `TestSchemaPropertiesCoverFixtureFields` holds `sheet` to the schema.
    - **Forced by** the walker exercising only what `testdata/valid`
      demonstrates (its own skip message), and by P3, which is why the
      section's key is not `properties`.
21. **D21. The register.** VTT-627 is withdrawn in Task 1's commit,
    succeeded by R7; every other row the work reaches stands, its evidence
    following the tests that change.
22. **D22. The specifications last,** after the report, from the report:
    SPEC-024, SPEC-012, SPEC-022 and SPEC-018's sentence.

## Open decisions for the owner

- **OD-1. How a section says what it shows** (the ticket's first
  unestablished point).
  - **A (recommended).** Each section holds exactly one of `"values":
    ["str", "dex"]`, `"conditions": true` or `"abilities": [ … ]`; the
    loader refuses none or two, as it refuses an outcome effect setting
    none or two. A property key stands as a value only in a list under
    `values`, and in `abilities` only inside a usage's `{"spends": …}`, so
    it cannot collide with `conditions` or `abilities`.
  - **B.** The same, with the key list under `"properties"`. P3: it
    collides as text with the manifest's own `properties` in the fixtures
    `numberAt` edits, and reads as a second declaration of properties.
  - **C.** A tagged list per section: `"shows": [{"property": "hp"},
    {"conditions": true}, {"abilities": "at_will"}]`, so one section can
    mix a value with the conditions. More words per entry and a mixed
    section to draw.
- **OD-2. How abilities are grouped.**
  - **A (recommended). By usage:** a section names usages, `"at_will"` or
    `{"spends": "<resource key>"}`, and shows each offered ability whose
    usage matches. Every ability declares its usage already; nothing is
    copied, and an ability added to the ruleset lands in its section, or is
    refused at load when no section names its usage (OD-3).
  - **B. By naming ability ids.** A second list of the ruleset's abilities,
    kept by hand; and the sheet, served to every role, would name to a
    player the ids `/api/ruleset` withholds from them (VTT-691, SPEC-012)
    unless the route filtered it per role.
  - **C. By a label on each ability,** as MapTool's macro groups (P8).
    Changes the ability format, which the next ticket opens for the
    description.
- **OD-3. Repeats and completeness.**
  - **A (recommended).** A property key stands at most once among the
    sections' `values`, a usage at most once among their `abilities`, and
    the conditions in at most one section; a key may stand both as a value
    and in the usage that spends it, as `flurry_uses` does in OD-8 A; a
    property need not appear (the ruleset
    shows what it chooses); the usage of every ability the ruleset holds
    stands in exactly one section, or the ruleset is refused, so no ability
    a sheet names is unreachable from the panel (principle 25). Allowing a
    repeat later loosens a check, which no ruleset notices; forbidding one
    later refuses rulesets that load today.
  - **B.** Repeats allowed; every property must appear.
  - **C.** Everything exactly once, every property included.
- **OD-4. `format_version`.**
  - **A (recommended). Stays `"3"`.** The field is optional, every
    `"3"` ruleset without it loads as before, and one loader reads them all;
    `formatVersionWant` stays true.
  - **B. `"4"`.** Every ruleset and fixture changes its version, VTT-616 is
    withdrawn and succeeded, for no reader that needs to tell the two apart.
- **OD-5. Where the platform's controls sit.**
  - **A (recommended).** Outside the tabs: the party-member chips above
    them, under the heading; the target buttons (or "Click the board to
    move."), the arm-doors toggle and the Say form below the shown page, on
    every page. An ability armed on one page keeps its targets shown on
    another.
  - **B.** The target buttons inside the abilities section that armed one.
  - **C.** The Say form on a tab of its own.
- **OD-6. The page drawn for a ruleset that declares none.**
  - **A (recommended).** One page, no tab, of three sections headed
    "Values", "Conditions" and "Abilities", in that order, each left out
    when it has nothing to show (D12); "Abilities" is today's heading.
  - **B.** The values with no heading, then "Conditions" and "Abilities".
  - **C.** "Properties" for the values, the route's and SPEC-024's word.
- **OD-7. A resource ability's title.**
  - **A (recommended).** `Flurry uses 1/1, costs 1`, and `Flurry uses
    0/1, costs 1 — not enough` (D16).
  - **B.** `Flurry uses 1/1 (cost 1)`.
  - **C.** `Costs 1 of Flurry uses, 1/1 left`.
- **OD-8. `dnd45e-minimal`'s sheet.**
  - **A (recommended).** Page "In play": "Hit points" (`hp`), "Conditions",
    "At-will powers" (`at_will`), "Uses" (`flurry_uses`), "Encounter
    powers" (spends `flurry_uses`). "Uses" was added at sign-off by the
    ticket's writer: without it `flurry_uses`'s value stood on no page,
    only in a button's title. Page "Character": "Ability scores" (`str`, `dex`,
    `con`), "Defenses" (`ac`, `fort`, `ref`, `will`). `max_hp` is left out:
    its description calls it a copy of the hit points' maximum, which the
    hit points already show. Powers by how they are used beside hit points
    and conditions is the owner's paper sheet (`Ursus_250907.pptx`, slide
    10), and the abilities on the first page keep `client/e2e/table.spec.ts`
    "a player's ability reaches the creature they aim it at" as it is.
  - **B.** As A with `max_hp` under "Hit points".
  - **C.** Page "Character" first and "Powers" second; the e2e spec clicks
    the tab before the ability.
- **To raise at sign-off (G7):** whether a condition's name replaces its id
  on a dot's title and in the Story's and the Events panel's lines, as a
  ticket of its own.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill, under the
signed-off answers; the register's words are the ones that bind. Every new
row reads `**OPEN — no test yet**` until its task cites its tests. Written
under the recommended answers; another answer rewrites the rows it touches.

| R | Id | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|---|
| R1 | VTT-824 | 1 | `rules.Load` returns a declared `sheet`'s pages, sections and what each shows, in the order declared | the field refused, or an order lost |
| R2 | VTT-825 | 2 | `rules.Load` refuses, naming the file and the field, a `sheet` with no page, a page unnamed, named as another or with no section, a section unnamed, showing none or more than one kind, or showing an empty list | any of them loading |
| R3 | VTT-826 | 2 | `rules.Load` refuses, naming the file and the field, a section whose `values` names an undeclared key, or whose `abilities` names a `spends` of a key no resource property declares | either loading |
| R4 | VTT-827 | 2 | `rules.Load` refuses, naming the file and the field, a `sheet` whose `values` name one key more than once, whose `abilities` name one usage more than once, or that shows the conditions in more than one section; a key may stand as a value and in a `spends` | a repeat loading |
| R4b | VTT-828 | 7 | `rules.Load` refuses a ruleset declaring a `sheet` and holding an ability whose usage no section names, naming the file and the ability | that ruleset loading |
| R5 | VTT-829 | 2 | `internal/rules/schema/ruleset.schema.json` describes `sheet` and every field it holds | a field the fixture uses undescribed |
| R7 | VTT-830 | 2 | A `ruleset.json` holding a field the format does not define, at its top level or within `sheet`, compared without regard to case, is refused when it loads, naming the file and the field (succeeds VTT-627) | an unknown field loading, or `sheet` refused |
| R8 | VTT-831 | 3 | `GET /api/ruleset` answers every role, as `sheet`, the declared pages in order, each with its name and its sections in order, each section with its name and what it shows | a role answered otherwise, or an order lost |
| R9 | VTT-832 | 3 | With no ruleset loaded, or one declaring no `sheet`, `/api/ruleset` answers `sheet` as an empty, non-null list | `null` or absent |
| R10 | VTT-833 | 3 | `get_ruleset`'s description says it answers the declared sheet, pages of named sections and what each shows, and an empty list when none is declared | the description silent |
| R11 | VTT-834 | 10 | A browser window draws a disc's resource chips, one per resource, when the role the client last read is `dm` or `agent`, and none otherwise | a chip in a player's or a spectator's window, or none in a DM's |
| R13 | VTT-835 | 10 | Every browser window draws on a disc one dot per condition, in the order applied | a dot missing in some role |
| R14 | VTT-836 | 9 | The player panel is headed by the name of the character it shows, as its party-member chip names it, and draws no heading while it shows none | another heading, or "Your turn" |
| R15 | VTT-837 | 4 | With a sheet of more than one page, the panel draws one tab per page, named and ordered as declared, and shows the first page until a tab is clicked, then that tab's page; with one page, no tab | no tab, a tab misnamed or misordered, a click showing nothing new, or a tab for one page |
| R16 | VTT-838 | 4 | The panel keeps the page it shows across a repaint and a switch of character, and shows the first page when the sheet no longer declares that page | the first page after a repaint |
| R17 | VTT-839 | 4 | The panel draws on the page it shows the sections that page declares and no other, in order, each headed by its name with its values in the section's order, leaving out a section with nothing to show | an invented section, a reorder, or an empty section drawn |
| R18 | VTT-840 | 8 | With no sheet declared, or no ruleset, the panel draws no tab and one page of "Values", "Conditions" and "Abilities", each left out when empty, the values in the window's properties' order, then by key, an attribute before a resource of one key | a tab drawn, a value missing, an empty section drawn, or another order |
| R19 | VTT-841 | 5, 8 | A value reads as its property's display name and value, a number as itself and a resource as `current/max`, titled by the description; named by its key with no title when the ruleset does not declare it as that type; a key the sheet does not hold draws nothing | a key shown where a name exists, no title, or a missing value drawn |
| R21 | VTT-842 | 6 | The panel lists the conditions in the order applied, named by the ruleset's name and titled by its description, or named by id with no title when undeclared | another order, or an id where a name exists |
| R22 | VTT-843 | 7 | With a sheet declared, the panel draws one button per offered ability, in the one section whose usages name its usage, in the window's list order | a button misplaced, missing or doubled |
| R23 | VTT-844 | 7 | A resource ability's button is titled the resource as the window names it, its current and max (neither when the sheet lacks it) and the cost, and ` — not enough` when unaffordable | the current and max missing, or the cost missing |
| R24 | VTT-845 | 4 | The panel's pages, sections and condition names come from the window's newest `/api/ruleset` answer; an older answer arriving after it changes none of them | the older answer's layout or names drawn |
| R25 | VTT-846 | 7 | The panel labels each ability's button by the name the window's ruleset list gives it, or by its id when that name is empty | an id where a name exists |

**Withdrawn, in Task 1, with its successor:** VTT-627 (R7), read
"WITHDRAWN 2026-11-17: <its words>. False as worded once `rules.Load` reads
`sheet`. Succeeded by …", its evidence `**READING — <the review>; withdrawn,
held by nothing**`, as the register's recent withdrawals do.

**Rows checked and left standing,** their evidence cells updated where a
named test is rewritten or renamed:
- **VTT-815 and VTT-816** stay true as worded: a chip is drawn only where
  R11 draws one, and a cost still names the resource by its display name,
  else its key. Their tests in a player's window (P4, P6) are re-pointed to
  a DM's window or to the new title's words.
- **VTT-817** stays true; its test expects the new title (P6).
- **VTT-695** stays true: under R4 and R4b every offered ability's usage
  stands in exactly one section, and R22 draws one button for it there.
- **VTT-753** stays true for the abilities; R24 is its sibling for the
  layout.
- **VTT-809** stays true; its evidence `client/test/app.test.ts`, "the
  ruleset's properties reach the chip and the cost, and the Story draws no
  resource change", which VTT-815 cites too, reads its chip in a DM's
  window after Task 3 and its cost in the new title after Task 5, its name
  kept or both rows' citations moved with it.
- `client/test/qa-companion-first-sight.test.ts`'s "<role>: the fold holds
  the companion whole after the run", which no row cites, asserts no chip
  in a player's and a spectator's window, the value held on the fold, and
  the chip in the DM's and the agent's (R11).
- **VTT-616** stays (OD-4 A). VTT-114, VTT-619, VTT-620, VTT-621, VTT-631,
  VTT-693, VTT-694, VTT-710, VTT-436, VTT-439, VTT-441, VTT-457, VTT-500,
  VTT-532 and VTT-540 stand unchanged.

## The existing tests that change

By P3 to P6, and by reading the tests that draw the panel:

**Task 1:** none turn red with the section key `values` (P3);
`TestSchemaPropertiesCoverFixtureFields` turns red when D20's fixture
lands and green when the schema declares `sheet`, inside the task.

**Task 3 (P4):**
- `client/test/grid.test.ts`, "EVERY resource is shown, in a stable
  order, with current and max": its `discsOf` takes a DM's namer, and an
  assertion that a player's namer gives every disc no resource is added
  (R11).
- `client/test/qa-property-names.test.ts`, the chip tests P4 names: drawn
  through a DM's namer or `ViewExtras.dm: true`; their words unchanged.
- `client/test/spectator-view.test.ts`, "every resource shows as a
  current/max chip, named on hover", "a token WITH them renders both
  containers" and "a token's INNER content shrinks with the camera too,
  not just its outer box": rendered with `dm: true`; the dots' assertions
  stay. "a resource chip, its Events panel line and an ability's cost name
  the resource by its display name": its chip half in a DM's window, its
  cost half takes Task 5's words.
- `client/test/qa-companion-first-sight.test.ts`, "<role>: the fold holds
  the companion whole after the run": the chip expectation is `["5/8"]` for
  the DM and the agent and `[]` for a player and a spectator; the fold's
  and the dots' expectations do not change.
- `client/test/app.test.ts`, "the ruleset's properties reach the chip and
  the cost, and the Story draws no resource change": its chip read in a
  DM's window; its cost takes Task 5's words.

**Task 5 (P5, P6):**
- `client/test/spectator-view.test.ts`, "the panel's headings say what each
  section is": the `h2` list is the character's name.
- The title tests P6 names, in `qa-property-names.test.ts`,
  `spectator-view.test.ts`, `qa-older-answer.test.ts` and `app.test.ts`:
  each expects D16's words for the same resource, cost and shortfall,
  never a looser match; "a resource ability naming no resource names no
  resource in its cost" expects `, costs 2 — not enough`.

**Stay green and are not changed:** every party-chip, target, arm-doors and
Say test (item 11), `panel-offers.test.ts`, `qa-window-offers.test.ts`,
`qa-panel-say.test.ts` and every test that reads the "Abilities" heading,
since their fixtures declare no sheet and the undeclared page keeps today's
block (P7).

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R24 as
  the answers leave them; allocate ids with `requirement-id`; commit the
  ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task
  check:requirements-chain` passes.

### Task 1: the format (R1 to R7; D1 to D4, D19 to D21; items 1 and 2)

- **Files:** `internal/rules/format.go` (`Ruleset.Sheet`, `SheetPage`,
  `SheetSection`, `ShownUsage`); `internal/rules/load.go` (`manifestJSON`,
  the usage decoder, `checkSheet` called from `loadV2`);
  `internal/rules/schema/ruleset.schema.json`;
  `internal/rules/testdata/valid/ruleset.json` (D20); a new
  `internal/rules/declared_sheet_test.go`, written first and seen red on the
  base, its manifests built by editing a copy of `testdata/valid` as JSON,
  never as text, in neutral words (P10); `rulesets/dnd45e-minimal/ruleset.json`
  (OD-8); `docs/requirements.md` (VTT-627's withdrawal, R1 to R7's
  evidence).
- **True when:** the new tests pass and failed before the change to
  `load.go`; `env -u CLAUDECODE go test ./internal/rules/...
  ./internal/adventure/ ./internal/gateway/ ./internal/campaign/...` passes;
  `rules.Load` of both rulesets succeeds, `dnd45e-minimal`'s with
  `Ruleset.Sheet` as declared and `tavern-brawl`'s with none;
  `TestSchemaPropertiesCoverFixtureFields` and
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` pass, the latter's
  subtests for a page's `name` and `sections` and a section's `name`
  running rather than skipping; `go vet ./...`,
  `task check:requirements-chain`, `tools/check-new-prose.py` and
  `python3 tools/check-comments.py` pass.
- **Deliberate breaks:**
  - Drop `Sheet` from `manifestJSON`: R1's tests turn red.
  - Accept an empty page name, then two pages of one name, then a section
    showing two kinds: R2's tests turn red.
  - Skip the key check in `checkSheet`: R3's undeclared-key test turns red;
    skip the repeat check: R4's repeat test turns red.
  - Skip the every-ability check: R4b's test of an ability whose usage no
    section names turns red; accept a `spends` naming a number property:
    R3's test of it turns red.
  - Accept a page whose `name` is missing: R2's test and
    `TestSchemaRequiredFieldsMatchLoaderEnforcement`'s subtest for a page's
    `name` turn red (R2). Leave `sheet` out of the schema's `properties`:
    `TestSchemaPropertiesCoverFixtureFields` turns red (R5).
  - Drop `DisallowUnknownFields` from the usage decoder: R7's test of a
    usage carrying a `cost` turns red.
- **The state here:** a ruleset may declare a sheet and a faulty one is
  refused; nothing served or drawn changes.

### Task 2: the read surface and the agent's tool (R8 to R10; D5 to D7; item 3)

- **Files:** `internal/gateway/metadata.go` (`rulesetJSON`, `handleRuleset`,
  the page and section types); `internal/gateway/metadata_test.go` (every
  role reads the same sheet, read from `dnd45e-minimal`'s `ruleset.json`
  itself, in order; `[]` for `tavern-brawl` and for no ruleset);
  `internal/mcp/ruleset_tool.go` (`getRulesetDescription`);
  `internal/mcp/ruleset_tool_test.go` and
  `cmd/vtt/qa_ruleset_by_role_test.go` (the description says the sheet);
  `cmd/vtt/qa_get_ruleset_test.go` (`TestQARRGetRulesetReturnsTheRouteBodyVerbatim`
  fails as a degenerate fixture when the route serves no sheet for
  `dnd45e-minimal`, as it does for no property); `docs/requirements.md`.
- **True when:** the new tests pass and failed before the change;
  `env -u CLAUDECODE go test ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/`
  passes; `git diff --stat main -- contract cmd/vtt/tools.json` prints
  nothing; the checks of Task 1's last line pass.
- **Deliberate breaks:**
  - Serve the sheet only to the DM: R8's every-role test turns red.
  - Leave `Sheet` nil for a ruleset declaring none: R9's test turns red.
  - Serve the sections in reverse: R8's order test turns red.
  - Drop the sentence from `getRulesetDescription`: R10's tests turn red.
- **The state here:** `/api/ruleset` and `get_ruleset` carry the sheet;
  the client ignores it.

### Task 3: the board draws no chip in a player's window (R11 to R13; D8; item 10)

- **Files:** `client/src/view/grid.ts` (`tokensOnScene` and its first
  paragraph, rewritten as a pointer to R11 and SPEC-022);
  `client/src/view/spectator.ts` (`renderDisc`'s comment only);
  `client/src/style.css` (the `.chips` rule's comment); the tests Task 3's
  list above names; new tests: a booted window of each role
  (`client/test/app.test.ts`, `/api/me` answering `dm`, `agent`, `player`,
  `spectator`, and no role) draws chips on a disc for the DM and the agent
  alone and dots for all; `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **True when:** the new tests pass and failed before the change;
  `env -u CLAUDECODE bun test client/test` passes; `task client:typecheck`
  passes; `git diff --stat main -- client/src` lists `grid.ts`,
  `spectator.ts` and `style.css`, the last two in comments alone; `task
  build:client` has run after the last change to `client/src` and its
  output is in the commit; the prose and comment checks pass.
- **Deliberate breaks:**
  - Key the chips on `true` in place of `names.dm`: the player's and the
    spectator's window tests turn red (R11).
  - Key them on `false`: the DM's and the agent's turn red (R11).
  - Draw dots only where chips are drawn: the companion test and the
    every-role test turn red for a player and a spectator (R13).
- **The state here:** a player's and a spectator's board draw dots and no
  chip; the panel is as before.

### Task 4: the client's model of the sheet (R17 to R22 as data; D9, D13 to D15)

- **Files:** `client/src/metadata.ts` (`SheetPage`, the `SheetSection`
  union, `RulesetMeta.sheet`); `client/src/player.ts` (`sheetFor`, the
  pages, sections, values, conditions and abilities to draw for one
  character, the undeclared page included); a new
  `client/test/sheet.test.ts`, written first, holding R17 to R22 on the
  model: a declared sheet's order kept and no section invented, an empty
  section left out, the undeclared page's order, a key named only when
  declared as its map's type, a condition by name or by id, each offered
  ability in the one section of its usage.
- **True when:** the new tests pass and failed before `sheetFor` existed;
  `env -u CLAUDECODE bun test client/test` and `task client:typecheck`
  pass; `git diff --stat main -- client/src` adds `metadata.ts` and
  `player.ts` to Task 3's; the prose and comment checks pass. Nothing is
  drawn yet, so `cmd/vtt/webdist` does not change.
- **Deliberate breaks:**
  - Sort a section's values by key: the order test turns red (R17).
  - Order the undeclared page's values by key alone, then keep its empty
    "Conditions" section: R18's order test and its empty-section test turn
    red.
  - Keep an empty section: D12's test turns red (R17).
  - Name a key declared as the other type by the declared name: R19's test
    turns red.
  - Re-sort the conditions by id: R21's order test turns red.
  - Put an ability in every section: R22's test turns red.

### Task 5: the panel draws the sheet (R14 to R16, R23 to R25; D10, D11, D16 to D18; items 4 to 9, 11)

- **Files:** `client/src/view/player.ts` (`renderPlayerPanel`: the
  heading, the tabs, the sections drawn from `sheetFor`, the title, the
  trailing parameter, `PlayerUIState.page`); `client/src/app.ts`
  (`readRuleset` keeps `sheet` and `conditions` under the ticket; `paint`
  hands them on); `client/src/style.css` (`.player .tabs`, `.tab`,
  `.tab.sel`, `.value`, `.condition`); `client/test/spectator-view.test.ts`
  and `client/test/app.test.ts` (new tests: a sheet of several pages draws
  a tab per page and the first page, a click shows another, a repaint and
  a switch keep it; a sheet of one page draws no tab; the heading names the
  character, the DM's by the fold; a value, a condition and a button's
  title in the ruleset's words; the older answer's layout not drawn, beside
  VTT-753's tests in `client/test/qa-window-reread.test.ts`); the tests
  Task 5's list above names; `client/e2e/table.spec.ts` (in a new test,
  the player's panel draws the tabs `dnd45e-minimal`'s `ruleset.json`
  names, read from the file, and a click on the second shows its first
  section; the player's discs draw no `.chips` and the DM's do);
  `docs/requirements.md`; a rebuilt `cmd/vtt/webdist`.
- **True when:** the new tests pass and failed before the change;
  `env -u CLAUDECODE bun test client/test` passes; `task client:typecheck`
  passes; `task build:client` has run after the last change and its output
  is in the commit; the prose and comment checks pass;
  `task check:requirements-chain` passes.
- **Deliberate breaks:**
  - Head the panel "Your turn" again: R14's test turns red.
  - Draw tabs for a single page: the one-page test turns red (R15).
  - Reset `ui.page` at every paint: R16's repaint test turns red.
  - Draw the targets inside the page: the test that arms on one page and
    finds the targets on the other turns red (OD-5 A).
  - Drop current and max from the title: R23's tests turn red.
  - Set the sheet from every answer: R24's test turns red.
  - Label a button by the ability's id: R25's test turns red.
- **The state here:** a player's panel is their character's sheet, laid
  out as the ruleset declares, with the controls of today around it.

### Task 6: the gate (item 12)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once,
  last, if a ceiling can come down; check free space and the load
  (`os.getloadavg()`); run `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion
  line; run `task e2e`. Tell the owner the client's stored mutation report
  is out of date from Task 3 and which commit it measured (SPEC-023); a
  full Stryker run is his to ask for.
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 7: the report, then the specifications

- **The report:** `docs/reports/2026-11-17-the-player-panel-is-the-sheet.md`
  with the `implementation-report` skill and its reading review: each
  "Done looks like" item with its observation; R1 to R25 and what turned
  each red; Rule 9; G1 to G7 and each OD's answer; Phase 4a's
  adjudications under their own heading; the gate.
- **The specifications,** with the `specification` skill, from the report:
  - **SPEC-024:** the manifest holds `sheet`; its shape, its refusals and
    where `checkSheet` runs; the Consequences' sentence on a new field.
  - **SPEC-012:** `/api/ruleset` answers `sheet` to every role, `[]` for
    none; `get_ruleset`'s description says so.
  - **SPEC-022:** the player panel's heading, tabs, sections and the
    undeclared page; the button's title; the newest answer's hold on the
    layout; "A disc's resource chip shows `current/max`" for the DM's and
    the agent's windows only.
  - **SPEC-018:** page and section names among a ruleset's unbounded
    strings and among what `/api/ruleset` carries to every participant.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the surfaces
as signatures (`go doc ./internal/rules`, `go doc ./internal/gateway`) and
names, never the source: `rules.Load`, `Ruleset.Sheet` and its types;
`GET /api/ruleset`; `get_ruleset`; `sheetFor`; `renderPlayerPanel`'s
signature and `PlayerUIState`; `tokensOnScene`; `boot` from
`client/src/app.ts` with its fetch and socket seams as `app.test.ts` uses
them. Tell it: never run `task check`, `task e2e` or Stryker; its files are
new `qa-*` test files; run client tests with `env -u CLAUDECODE bun test`;
no game word in a Go literal under `internal/` or `cmd/` (P10), and
`go-arch-lint` holds the layers; the client fold refuses a sheet that does
not fit, so a fixture is built as the gateway writes a log; the tree is
hashed before and after it. Known and not findings: a condition's id on a
dot's title and in the Story (G7); a companion's values shown nowhere to a
player; the DM's panel showing only a controlled actor's sheet; an ability
not on the sheet's first page needing a tab click.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check that
the Go diff holds no game word (P10) and the refusals' form is SPEC-024's;
check that `git diff --stat main -- contract cmd/vtt/tools.json` is empty;
check each comment under `internal/`, `cmd/` and `client/src` is a warning
or a pointer, and that `tokensOnScene`'s and `renderDisc`'s old stance is
gone; check every assertion this work changed for being weakened rather
than moved (rule 2), the chip assertions in a player's window above all;
and check VTT-627's withdrawal names R7's id.
