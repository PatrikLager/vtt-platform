# Plan: an ability may carry the ruleset's description of it, and the sheet shows it

- **Ticket:** `docs/superpowers/specs/2026-11-18-an-ability-carries-its-description-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-18 by an agent that did not write the ticket,
    read-only against `a164cac` on `feat/an-ability-carries-its-description`
    in the worktree `vtt-platform-visibility`, with the ticket added with
    intent-to-add and uncommitted.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e` and Stryker were not run. P1, P2 and P4 to P6 ran in copies
    of the tree at `a164cac` outside the repository, never in the
    worktree; P3, P8 and P9 are searches, P7 a script over the texts.
- **To be committed as:** `docs/superpowers/plans/2026-11-18-an-ability-carries-its-description.md`.
- **Base:** `a164cac`. **Branch:** `feat/an-ability-carries-its-description`.
- **Owner's rulings taken in:**
  - 2026-11-18: an ability MAY carry a description; the loader does not
    require one. On the sheet it is shown on hover of the ability's button,
    above its cost. What a description says is its author's and nothing
    checks its content. In the rulesets this repository ships, each
    ability's description says what it does in words, with no dice figures.
  - 2026-11-18: one ticket for the format, the read surface, the MCP
    tool's description and the client, committed as separate tasks.
  - 2026-11-18, at sign-off: "at will" is a D&D 4e word in the platform's
    own text. The word for an ability that spends no resource is the ruleset's:
    `ruleset.json` may name it, `dnd45e-minimal` names it "At will",
    `tavern-brawl` names none, and the platform writes no word of its own.
  - 2026-11-17: an ability's description is this ticket; a condition named
    by its id on a dot and in the Story is the next.
  - 2026-09-04: nobody uses the product; no migration.
  - Standing: no counts or measured figures in anything written down; push
    and merge on the owner's word.
- **Signed off:** 2026-11-18. The owner accepted OD-1, OD-2, OD-3, OD-5
  and OD-6 as recommended and OD-7's texts as proposed; answered OD-4
  otherwise, the ruleset naming an ability that spends no resource; and
  accepted the recommendation that `tavern-brawl` names none. The ticket's
  writer amended the plan for that answer and for Task 0's reading review
  before Task 0's commit: the rulings, OD-4, D11, D13 to D18, R8, R9, R12
  to R17, R5, the rows checked, the sort's refusals, the tests that
  change, Tasks 0 to 4 and 6, and the notes for Phase 4a. The ticket was
  amended after verification too, closing G1, G2 and G5 and taking in the
  owner's answers; the verdict and the gaps below describe the ticket as
  it was verified.
- **Ids:** allocated at Task 0. The `R` labels are this plan's: R1
  VTT-847, R2 VTT-848, R3 VTT-849, R4 VTT-850, R5 VTT-851, R6 VTT-852, R7
  VTT-853, R8 VTT-854, R9 VTT-855, R12 VTT-856, R13 VTT-857, R14 VTT-858,
  R15 VTT-859, R16 VTT-860, R17 VTT-861. VTT-830 is withdrawn in Task 1's
  commit, succeeded by VTT-861 (D18).
- **The sort's refusals:** R10 (an older answer leaves the description in
  the title as the newest gives it), held by VTT-753, since the
  description travels on the ability object that row governs; R11 (an
  ability file's unknown field refused), not this ticket's rule: the strict
  decode holds it today and the ticket changes only which fields it
  defines; no row holds it, a gap the next work on the ability format
  records; the shipped texts' content (OD-7) and the shipped
  `free_usage_name` values, the owner's sign-off, not rules; no generated artefact changing (scenery).

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists:
   `internal/rules/load.go`, `internal/rules/compile.go`,
   `internal/rules/format.go`, `internal/rules/schema/ability.schema.json`,
   `internal/rules/schema_test.go`, `internal/rules/testdata/valid/abilities`,
   `internal/rules/conformance/conformance.go`, both rulesets' `abilities/`
   and `goldens/compiled/`, `internal/gateway/metadata.go`,
   `internal/mcp/ruleset_tool.go`, `internal/mcp/ruleset_tool_test.go`,
   `client/src/metadata.ts`, `client/src/view/player.ts`, `cmd/vtt/webdist`,
   `docs/requirements.md`, `docs/blueprint.md` and SPEC-012, SPEC-018,
   SPEC-022 and SPEC-024. Every symbol is where the ticket says:
   `compositionJSON`, `loadCompositions`, `loadProperty`, `loadConditions`
   (which keeps a condition's description), `compilePower`,
   `rules.CompiledPower`, `rules.Load`, `rules.Resolve`, `abilityJSON`,
   `handleRuleset`, `getRulesetDescription`, `renderPlayerPanel`,
   `costTitle`, `Ability`, `DumpCompiledPower`; the rows VTT-691 to VTT-693
   and VTT-844. MapTool's `MacroButtonProperties` holds `toolTip`, and the
   framework's `rule-framework/schema/ability.schema.ts` an optional
   `flavorText`. One citation is loose (G5).
2. **"Done" is an observation. By command for items 1, 2, 3 and 6, by
   reading for 4 and 5.** Item 1: today's `rules.Load` refuses an ability
   holding `description` with `json: unknown field "description"` (P1).
   Item 2: with `strike.json` in `testdata/valid` declaring one,
   `TestSchemaPropertiesCoverFixtureFields` fails on today's schema and
   passes once the schema declares the field (P4). Item 3: the shipped
   rulesets, given descriptions, are refused today (P1) and load and pass
   conformance under a probe loader (P2). Items 4 and 5, by reading:
   `abilityJSON` has no description field, and `costTitle` answers the cost
   line or "at will" alone. Item 6: the tests of VTT-844, VTT-691 to
   VTT-693 and VTT-815 stayed green under the probes (P2, P5). Item 7 is
   the gate. The ticket names no test, so none could be run red by name.
3. **Each rule is breakable. By reading.** Its rule 1 fails when the decode
   refuses the field or `rules.Load` drops it; rule 2 when a role's answer
   lacks the description of an ability it is served; rule 3 when
   `get_ruleset`'s description is silent; rule 4 when a title holds the
   cost line first, or the description is missing. The items carry rules
   the list does not (G1); R1 to R9 hold them all.
4. **The scope matches the claim. By command, then reading.** Each thing
   the ticket changes has one production caller: `compositionJSON` is
   decoded in `loadCompositions` alone; `CompiledPower` is built in
   `compilePower` alone; `abilityJSON` is written in `handleRuleset` alone;
   `getRulesetDescription` is read by `registerRulesetTool` alone;
   `costTitle` is called from `renderPlayerPanel` alone. Outside
   `internal/rules`, `rs.Compiled` is read by `handleRuleset` and by
   `internal/rules/conformance` alone. No `abilities/` directory exists but
   the two rulesets' and the fixtures' under `internal/rules`. The tests and
   files the work reaches are wider than the ticket names (G2).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-012's principle 35 ("a summary of every ability … How an ability
     resolves … is not served"): a description is not how an ability
     resolves; the summary grows, by the ticket's naming of SPEC-012.
   - Blueprint 35 ("with the ruleset's own explanation"): met only for an
     ability whose author wrote one; the ticket names the principle, the
     cost and the owner's ruling, as the blueprint's preamble asks.
   - Blueprint 27: the description stays in the ruleset and reaches the
     sheet through `/api/ruleset`; no sheet copies it.
   - `CompiledPower`'s doc calls it the declarative execution form Resolve
     runs; a description beside `Name` is text Resolve never reads (OD-1).
   - SPEC-018: an ability's display name is listed among what
     `/api/ruleset` carries to whom; its description joins that sentence.
     Named by the ticket.
   - The 2026-11-17 plan's Rule 9 left a group label on each ability for a
     later change to the ability format; this ticket adds no label.
6. **The records the work moves are named. By command, then reading.** The
   section is present, each of its paths resolves, and nothing is `New:`.
   By search (`grep -ln` over `docs/specifications/` for `CompiledPower`,
   `compositionJSON`, `abilities/`, `ability.schema`, `abilityJSON`,
   `get_ruleset` and `at will`), only SPEC-012, SPEC-022 and SPEC-024 name
   the subject, and SPEC-018 the strings `/api/ruleset` carries: exactly
   the specifications the ticket lists. No specification records the ability file as such
   (the ticket's own last point; OD-6).

**Split: not offered.** The owner ruled one ticket, committed as separate
tasks (Tasks 1 to 4).

## The gaps this plan inherits

- **G1. Rules the ticket's list does not carry.** The schema's description
  of the field (item 2), the shipped rulesets' descriptions in words (item
  3), no description for an ability that declares none (item 4), the title
  with no description (item 5) and an empty description (its "could not be
  established"). R2, R3, R4, R6 and R9 carry them.
- **G2. Tests and files the ticket does not name.** `compositionAbility`
  (in `load.go`, which the ticket names), a new test file in
  `internal/rules`, `internal/rules/conformance/conformance_test.go` (R4),
  `internal/gateway/ruleset_by_role_test.go` or a new route test file (R5,
  R6), `cmd/vtt/qa_get_ruleset_test.go` (VTT-621's degenerate-fixture
  guard), `client/test/panel-sheet.test.ts` and `client/test/app.test.ts`
  (R8, R9), `client/e2e/table.spec.ts` (D13) and, if a ceiling falls,
  `tools/comment-ceilings.txt`. The ticket says "its tests" and "the
  client tests that read an ability button's title", so this is a gap, not
  a return.
- **G3. The ticket's "What could not be established" travels.** Where the
  description lives (OD-1); an empty one (OD-2); SPEC-024's subject
  (OD-6); how a long title is drawn, and that some touch platforms draw
  none, which the sheet's values and conditions share today.
- **G4. A disabled button's title.** The panel disables a button whose
  ability the character cannot afford, or that has no token (VTT-695);
  whether a browser draws the native title of a disabled button is the
  browser's. Today's ` — not enough` rides on the same title. Not measured:
  no browser was run.
- **G5. A loose citation.** "G7 of the sheet's ticket" names a gap of the
  sheet's plan and report (`docs/superpowers/plans/2026-11-17-the-player-panel-is-the-sheet.md`,
  `docs/reports/2026-11-17-the-player-panel-is-the-sheet.md`), not of its
  ticket file. The writer's to correct.
- **G6. Words the composition binds.** The proposed texts carry no figure,
  but they name the stat, the defense and the condition each composition
  binds, in the ruleset's display names. Those are words, and they drift if
  a binding changes. OD-7 asks.

## Measurements this plan stands on

- **P1. Today's loader refuses a description.** In a copy of the tree at
  `a164cac`, `tavern-brawl` with `description` added to one ability:
  `rules.Load` answered `rules: <path>/abilities/fists.json: json: unknown
  field "description"`.
- **P2. A description in the loader, in the shipped abilities and on the
  route turns no Go test red.** In a copy with `Description` decoded in
  `compositionJSON`, carried on `compositionAbility` and `CompiledPower`,
  written by `handleRuleset` on every ability as `"description"` (empty
  when none, the stricter of OD-3's shapes), `compiledPowerDump` unchanged,
  and every shipped ability given a description: `go test` of
  `./internal/rules/...`, `./internal/gateway/`, `./internal/mcp/`,
  `./internal/adventure/...`, `./internal/campaign/...`,
  `./internal/harness/...`, `./internal/engine/...`,
  `./internal/identity/...`, `./tools/...` and `./cmd/vtt/` passed, the
  conformance goldens among them. The route's field alone, before the
  shipped texts, turned nothing red either.
- **P3. No test reads a route answer strictly.** By search, no gateway or
  `cmd/vtt` test decodes `/api/ruleset` with `DisallowUnknownFields`;
  `qaRRCheckAbilities` asks for the fields it names and allows others.
- **P4. The schema fixture.** In the same copy, `testdata/valid/abilities/strike.json`
  given a description: `TestSchemaPropertiesCoverFixtureFields` alone
  turned red, with `internal/adventure` and `internal/mapdef`, which read
  that tree, green; with `description` declared in `ability.schema.json`
  as an optional string, every `internal/rules` test passed.
- **P5. The title turns no client test red.** In a copy of the client at
  `a164cac` (the suite green there first), `Ability` given `description?:
  string` and the button titled the description, `\n` and the cost line
  when it has one: `bun test client/test` passed and `tsc` for the client
  passed. No client fixture builds an ability with a description.
- **P6. A required field's reach.** With `description: string` in
  `Ability`, `tsc` failed in `command-surface.test.ts`,
  `dm-abilities.test.ts`, `panel-offers.test.ts`, `panel-sheet.test.ts`,
  `player.test.ts`, `qa-ability-target.test.ts`, `qa-give-value.test.ts`,
  `qa-panel-say.test.ts`, `qa-panel-sheet.test.ts`,
  `qa-property-names.test.ts`, `qa-sheet-model.test.ts`,
  `qa-what-a-window-shows.test.ts`, `qa-window-offers.test.ts`,
  `qa-window-reread.test.ts`, `sheet-news.test.ts`, `sheet.test.ts`,
  `spectator-view.test.ts` and `token-card.test.ts`.
- **P7. The e2e Say check reads the panel's attributes.**
  `client/e2e/refusals.spec.ts` "a DM granted a character draws its chip,
  and the console's Say alone" matches `/\bsay|speak|narrat|\booc\b|\/ooc|console|\bsend\b/i`
  against the DM's panel's text and every attribute, while the DM controls
  the Goblin Cutter, whose button is `goblin-scimitar`'s. None of the
  proposed texts matches it, and none holds a digit (checked by script).
- **P8. The vocabulary gate.** `.semgrep/vocabulary.yml` refuses, in Go
  under `internal/` and `cmd/`, tests included, `fortitude`, `bloodied`,
  `\bhp\b`, `hit_?points` and others, case-insensitively. The proposed
  `staggering-blow` text names Fortitude, so a Go test reads every
  description from its file and never writes one.
- **P9. The comment ceilings.** `tools/comment-ceilings.txt` holds
  `internal/mcp/ruleset_tool.go` at a ceiling that admits no added comment,
  and `internal/gateway/metadata.go` and `client/src/view/player.ts` close
  to their present shares.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no assertion is weakened: a title assertion
  takes the new words exactly, never a looser match); 3 (the protobuf
  contract does not change: `/api/ruleset` is JSON outside `contract/`, and
  `get_ruleset` is Go-registered, so `contract/gen/tools/tools.json` and
  `cmd/vtt/tools.json` stay as they are); 4 (the fold is untouched); 5 (no
  game word in Go under `internal/` or `cmd/`, tests included, P8: a Go
  test reads a shipped description from its file as JSON, and a fixture
  speaks in neutral words); 8 and 10 (names, never line numbers; a comment
  is a warning or a pointer; no comment is added to `ruleset_tool.go`, P9;
  a pointer joins an existing comment line where a ceiling is tight); 9
  (answered below).
- **Blueprint principles 13, 23, 27, 34 and 35.** 13: the shipped texts
  name what an ability reads and changes by the ruleset's display names. 23:
  the description is the ruleset's data; the engine stores and serves it
  and never reads it. 27: the sheet reaches the description through
  `/api/ruleset`; nothing copies it into a sheet or the log. 34: the
  `get_ruleset` description says what the route answers, absent included.
  35: the description is served with the ability, to whoever is served it.
- **SPEC-012** (every list non-null; abilities filtered as read from
  `Compiled`), **SPEC-018** (a ruleset's descriptions are unbounded),
  **SPEC-024** (the strict decode; the refusal's form `rules: <path>: field
  "<field>": <message>`; a field present as `null` reads as absent),
  **SPEC-022** (the panel draws from the newest answer; the tree is rebuilt
  at every paint).
- **SPEC-020, SPEC-023.** No adjudicated mutant in
  `tools/ts-mutation-equivalents.txt` sits on `costTitle`. The client's
  stored mutation report goes out of date with Task 4's commit; `task
  check` names that and passes; a full run is the owner's to ask for. The
  Go mutation gate judges the new loader and route lines in Task 5.
- **Principle 33, as it stands.** `/api/ruleset` is described twice, by
  `abilityJSON` and by `Ability`; Task 4's end-to-end check holds them
  together on the real wire.
- **The project's mechanical gates:** a client change is committed with its
  `cmd/vtt/webdist`, rebuilt with `task build:client`; bun and go run under
  `env -u CLAUDECODE`; `task client:typecheck`; `tools/check-new-prose.py`
  and `python3 tools/check-comments.py` before each commit; stage and
  commit in separate calls; Go tests scoped to the packages a task touches
  until Task 5. Each task gets Phase 4a's QA and one reviewer before its
  commit.

## Rule 9: how MapTool does this

The ticket's paragraph holds against `~/dev/RPTool`:
- `MacroButtonProperties` keeps a `toolTip`, an empty string by default,
  set by the macro's author; `MacroButton.getToolTipText` returns it
  through `getEvaluatedToolTip`, and returns none when it is empty, so an
  empty tooltip draws nothing. A tooltip starting with `{` or `[` is run as
  macro code.
- The framework's `rule-framework/schema/ability.schema.ts` gives an
  ability an optional `flavorText`; `useAbility.mts` prints it in italics
  under the ability's label in the chat, after a use. Its data files write
  mechanics with figures into it.
- **Borrowed:** the author's own text, optional, shown on hover of the
  button that uses the ability.
- **Not borrowed:** printing it at each use (the Story says what happened,
  not what an ability is); running a tooltip as code (principle 24);
  figures in the shipped texts (the owner's ruling). MapTool's empty
  tooltip meaning none is OD-2's option B.

## Decisions

1. **D1. The description is decoded in `loadCompositions` and kept on
   `CompiledPower`** (OD-1 A). `compositionJSON` gains `Description
   *string` as `description`; `compositionAbility` and `CompiledPower` gain
   `Description string`, empty when none is declared; `compilePower`
   copies it beside `Name`.
   - **Forced by** `handleRuleset` reading every ability it serves from
     `rs.Compiled`, and SPEC-012's "filtered as they are read from
     `Compiled`": one place holds what is served of an ability.
2. **D2. An empty description is refused** (OD-2 A): in
   `loadCompositions`, after the name's check, a `description` present as
   `""` is refused as `fieldErr(path, "description", "must not be empty")`;
   absent, or present as `null`, it reads as none.
   - **Forced by** the pointer: a plain `string` cannot tell `""` from
     absent; SPEC-024's convention that a field present as `null` reads as
     absent; and `loadProperty`'s refusal of an empty description, the one
     rule the format has on one.
3. **D3. A description is unbounded.**
   - **Forced by** SPEC-018: a ruleset's descriptions are among the strings
     its loader does not bound.
4. **D4. The compiled goldens do not carry it** (OD-1 A).
   `compiledPowerDump` and every `goldens/compiled/*.json` stay as they
   are.
   - **Forced by** the goldens pinning what Resolve runs, which never reads
     a description; and by the ruling that nothing checks a description's
     content, which a golden repeating it would.
5. **D5. The schema and its fixture.** `ability.schema.json` declares
   `description` as `{"type": "string", "minLength": 1}` with an
   explanation, outside `required`; its title paragraph names the field.
   `testdata/valid/abilities/strike.json` declares one, in neutral words;
   `guard-stance.json` and `stand-down.json` declare none.
   - **Forced by** `TestSchemaPropertiesCoverFixtureFields` reading
     `strike.json` (P4), and `TestSchemaRequiredFieldsMatchLoaderEnforcement`,
     which would delete a required field and expect a refusal.
6. **D6. `format_version` stays `"3"`** (OD-5 A).
   - **Forced by** the field being optional: every ruleset without it
     loads as before, and VTT-616 stays true.
7. **D7. The shipped texts** are the ones signed off under "Proposed
   descriptions", each written after `name` in its file. R4's test sits
   beside `TestConformanceOverRulesetsGlob` in
   `internal/rules/conformance/conformance_test.go`, reading every
   `rulesets/*/abilities/*.json` as JSON; it is not a check of
   `conformance.Run`.
   - **Forced by** the ruling: what a description says is its author's,
     so `Run`, every ruleset's contract, checks no content; the figures
     rule binds this repository's rulesets alone.
8. **D8. The route:** `abilityJSON` gains `Description string` as
   `description,omitempty`, set from `CompiledPower.Description` (OD-3 A).
   - **Forced by** item 4 ("with none for an ability that declares
     none") and principle 33 as it stands: under D2 a served description is
     never empty, so the route's absence and the client's optional field
     say the same thing.
9. **D9. `get_ruleset`'s description gains the words** within its
   abilities' parenthesis, "(range, targets, usage, and `description`, the
   ruleset's own words for what the ability does, absent when it gives
   none)", leaving VTT-694's and VTT-833's sentences whole. No comment is
   added to `ruleset_tool.go`; no generated artefact changes.
   - **Forced by** VTT-694's and VTT-833's tests, which match those
     sentences as written; P9; `TestQARRGetRulesetIsListedAndTakesNoArguments`,
     which holds the tool out of the contract manifest.
10. **D10. The client's `Ability` gains `description?: string`** (OD-3 A).
    - **Forced by** D8's absence, and P6: a required field reaches every
      client test file that builds an ability.
11. **D11. The title.** `renderPlayerPanel` titles an ability's button,
    through a new pure function beside `costTitle`, by its non-empty
    description and its cost line, each when present, joined by a line
    break (`\n`), the description first (OD-4 as answered). A resource
    ability's cost line is `costTitle`'s, whose words do not change
    (VTT-844); an ability that spends no resource has as its cost line
    the word the window's newest `/api/ruleset` answer gives it
    (`freeUsageName`), and none when the answer gives none or gives `""`,
    so its title is its description alone, or empty. `costTitle` no longer writes `at will`. An empty
    description reads as none, and a test pins it.
    - **Forced by** item 5 and the ruling ("above its cost"); VTT-844's
      words, which item 6 keeps; the pin keeps the empty case from being
      an equivalent mutant.
12. **D12. A description's own line breaks stand in the title as
    written** (OD-4 A).
    - **Forced by** the ruling that nothing checks a description's
      content.
13. **D13. The real wire.** `client/e2e/table.spec.ts` gains a check that
    the player's "Longsword Strike" button is titled the description read
    from `rulesets/dnd45e-minimal/abilities/longsword-strike.json`, a line
    break and the `free_usage_name` read from
    `rulesets/dnd45e-minimal/ruleset.json`, the panel showing the first
    page.
    - **Forced by** principle 33 as it stands; the e2e spec already reads
      `dnd45e-minimal`'s files from disk.
14. **D14. The register.** VTT-830 is withdrawn (D18); every other row
    the work reaches stands, its evidence following the tests that change,
    the at-will assertions of VTT-844's tests moving to VTT-855's.
15. **D15. The specifications last,** after the report, from the report:
    SPEC-024 (OD-6, and the manifest's `free_usage_name`), SPEC-012 (the
    abilities' `description` and `freeUsageName`), SPEC-022 (the title as
    D11 gives it) and SPEC-018's sentence (both strings).
16. **D16. The ruleset's word for an ability that spends no resource.**
    `ruleset.json` may hold `free_usage_name`, a non-empty string;
    `loadManifest` decodes it as `*string`, so absent or `null` is none, and
    refuses `""` naming the file and the field; `rules.Load` keeps it as
    `Ruleset.FreeUsageName`. `ruleset.schema.json` describes it as optional
    with `minLength: 1`. `rulesets/dnd45e-minimal/ruleset.json` declares
    `"At will"`; `rulesets/tavern-brawl/ruleset.json` declares none.
    `format_version` stays `"3"`.
    - **Forced by** the owner's answer to OD-4, principle 23 (no game
      word in the platform's text), and OD-2's form for an empty string.
17. **D17. The route serves it.** `rulesetJSON` gains `freeUsageName`,
    `omitempty`, to every role, as OD-3 serves an absent description; the
    client's `RulesetMeta` gains `freeUsageName?: string`, and the window
    keeps it from its newest answer in `layout` beside the sheet and the
    conditions, an answer without it read as none (VTT-845's form).
    `getRulesetDescription` gains a sentence saying so.
    - **Forced by** D16 and D9; the window draws the title from what its
      newest answer gave, as it draws the layout.
18. **D18. VTT-830 is withdrawn,** succeeded by R17, which adds
    `free_usage_name` to the manifest's fields.
    - **Forced by** VTT-830's words, which name the manifest's fields one
      by one and would refuse the new field.

## Open decisions for the owner

- **OD-1. Where the description lives once loaded, and whether the
  compiled goldens carry it** (the ticket's first unestablished point).
  - **A (recommended).** On `CompiledPower`, beside `Name`; the goldens do
    not carry it (D1, D4). The route reads one map, as today.
  - **B.** On `CompiledPower`, and `compiledPowerDump` gains `description`,
    so every shipped compiled golden repeats its ability's text, and a
    reworded description fails conformance until it is dumped again.
  - **C.** Apart: `Ruleset` gains a map of descriptions by ability id, and
    `CompiledPower` stays what Resolve runs. The route reads two maps, and
    a description of an id `Compiled` does not hold is a state to rule out.
- **OD-2. An empty description, `"description": ""`.**
  - **A (recommended).** Refused, naming the file and the field, as a
    property's empty description is; absent or `null` reads as none (D2);
    the schema says `minLength: 1`. An author who writes the key meant to
    write something, and principle 25 has the fault found at load.
  - **B.** Read as none, as MapTool reads an empty tooltip and as
    `loadConditions` keeps a condition's empty description today.
  - **C.** Refused when empty or only whitespace: a check of content,
    against the ruling.
- **OD-3. The route's field when none is declared, and the client's
  type.**
  - **A (recommended).** Absent (`omitempty`); `Ability.description?:
    string` (D8, D10). The route and the client say the same thing, and no
    client fixture changes.
  - **B.** `""`, as `conditionJSON` answers a condition with none;
    `Ability.description: string`, and every client test file P6 names
    gains the field.
  - **C.** `""` on the route and `description?: string` in the client: the
    route's two descriptions disagree on when the field is present.
- **OD-4. The title's form, and a description's own line breaks.**
  - **A (recommended).** The description, a line break, then today's line:
    `<description>\nat will`, `<description>\n<resource>
    <current>/<max>, costs <cost>`, and the same ending ` — not enough`
    when unaffordable. A line break inside a description stands as written
    (D11, D12).
  - **B.** One line, `<description> — <cost line>`; the dash already
    opens ` — not enough`, so a title can hold two.
  - **C.** A's form, with the panel folding a description's line breaks
    into spaces: the panel rewrites the author's text.
  - **Answered otherwise at sign-off:** A's form, but `at will` is not the
    platform's word. The cost line of an ability that spends no resource is the
    word the window's ruleset gives it (D16 to D18), and none when it gives
    none; a description's own line breaks stand as written.
- **OD-5. `format_version`.**
  - **A (recommended). Stays `"3"`** (D6).
  - **B. `"4"`.** Every ruleset and fixture changes its version and VTT-616
    is withdrawn and succeeded, for no reader that tells the two apart.
- **OD-6. Where the ability's description is recorded** (the ticket's
  last point).
  - **A (recommended).** SPEC-024 gains a paragraph, "An ability's words",
    beside "Where a property's meaning is read": the field, its refusal,
    where it is kept, that the goldens do not carry it; its Status names
    the new symbols and tests. The ticket's list stands.
  - **B.** A new specification of the ability file whole (`id`, `name`,
    `description`, `usage`, `compose`, the strict decode). The ticket would
    list it as `New:`, which is the writer's to add.
  - **C.** No paragraph in SPEC-024; SPEC-012 and SPEC-022 say what is
    served and drawn, and the format stays unrecorded.
- **OD-7. The shipped texts** (G6).
  - **A (recommended).** As proposed below: what each ability does and to
    whom, naming the stat it rolls, the defense it rolls against and what
    it changes, by the ruleset's display names, with no figure.
  - **B.** As A with no stat or defense named ("Shoot a creature at range;
    a hit deals damage."): nothing a binding can drift from, and a player
    no longer told that Staggering Blow aims at another defense.
  - **C.** The owner's own words.

## Proposed descriptions

Written from each file's `compose`, the atoms it binds and the ruleset's
`guide.md`; no figure, no digit, one line each (OD-7 A). For the owner to
approve at sign-off.

`rulesets/dnd45e-minimal/abilities/`:
- **crossbow-shot:** "Shoot a creature at range with a crossbow, rolling
  Dexterity against its Armor Class; a hit deals damage."
- **goblin-scimitar:** "Slash a creature within reach with a scimitar,
  rolling Dexterity against its Armor Class; a hit deals damage."
- **goblin-shortbow:** "Shoot a creature at range with a shortbow, rolling
  Dexterity against its Armor Class; a hit deals damage."
- **hunters-flurry:** "Loose a volley at range that can strike more than
  one creature, rolling Dexterity against each one's Armor Class; a hit
  deals damage plus your Dexterity, and even a miss grazes for part of it."
- **longsword-strike:** "Swing a longsword at a creature within reach,
  rolling Strength against its Armor Class; a hit deals damage plus your
  Strength."
- **rally:** "Rally yourself or a creature within reach, with no attack
  roll: they regain hit points plus their Constitution."
- **staggering-blow:** "Swing at a creature within reach, rolling Strength
  against its Fortitude; a hit deals damage plus your Strength and leaves
  it Dazed."

`rulesets/tavern-brawl/abilities/`:
- **chair-swing:** "Swing a chair at a patron within reach, rolling Brawn
  against their Footing; a hit spills Drink on them and leaves them Dazed
  by Ale."
- **fists:** "Throw a punch at a patron within reach, rolling Brawn
  against their Footing; a hit spills Drink on them."
- **sober-up:** "Sober yourself up: all your Drink drains away, which
  clears Dazed by Ale."
- **splash-of-water:** "Splash water on a patron within reach to clear
  Dazed by Ale, however much Drink is still on them."

How each reads its composition: "within reach" is a `melee-delivery` or
`brawl-reach` range, "at range" a `ranged-delivery` one, "yourself" a
range that reaches the caster alone; "rolling X against its Y" is
`weapon-attack-roll` or `footing-contest` bound to X and Y; "plus your X"
is a damage expression adding the caster's X; "a miss grazes" is
`graze-damage`; "leaves it Dazed" and "Dazed by Ale" are
`condition-rider` and `stagger-from-drink`; "regain hit points plus their
Constitution" is `recovery` bound to the target's `con`; "clears Dazed by
Ale" is `towel-off`, and for sober-up the `drink` threshold, as `drink`'s
own description says. None mentions a cost: the title's cost line says it.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill, under the
signed-off answers; the register's words are the ones that bind. Every new
row reads `**OPEN — no test yet**` until its task cites its tests. Written
under the recommended answers; another answer rewrites the rows it touches.

| R | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|
| R1 | 1 | `rules.Load` keeps, on each ability of `Ruleset.Compiled`, the `description` its `abilities/*.json` declares, and none for one that declares none or declares it `null` | the field refused, a description lost, or given to another ability |
| R2 | 1 | `rules.Load` refuses an ability whose `description` is present and empty, naming the file and the field `description` | that ability loading |
| R3 | 2 | `internal/rules/schema/ability.schema.json` describes `description` as an optional, non-empty string | the fixture's field undeclared, or `description` required |
| R4 | 3 | Every ability under `rulesets/` declares a non-empty `description` holding no digit | an ability without one, or a digit in one |
| R5 | 4 | `GET /api/ruleset` answers each ability it serves whose ruleset declares a description for it with `description`, that description, to every role it serves that ability | a role answered without it, or another ability's text |
| R6 | 4 | `GET /api/ruleset` answers an ability whose ruleset declares no description with no `description` field | the field present |
| R7 | 4 | `get_ruleset`'s description says it answers each ability with the ruleset's description of it, and without one when the ruleset gives none | the description silent |
| R8 | 5 | The title of the player panel's button for an ability is its non-empty description as written and its non-empty cost line, each when it has one, joined by a line break, the description first, and empty when it has neither; a resource ability's cost line is the text VTT-844 describes, its ` — not enough` included | the description missing, after the cost line, joined otherwise, rewritten, or a line break beside a single part |
| R9 | 5 | The cost line of the player panel's button for an ability that spends no resource is the name the window's newest `/api/ruleset` answer gives one, and none when that answer gives none or gives it empty | `at will` or another word of the platform's, the name missing, an empty line, or an older answer's name |
| R12 | 5 | `rules.Load` keeps, as `Ruleset.FreeUsageName`, the `free_usage_name` a `ruleset.json` declares, and none when it declares none or `null` | the field refused, or the name lost |
| R13 | 5 | `rules.Load` refuses a `ruleset.json` whose `free_usage_name` is present and empty, naming the file and the field | that ruleset loading |
| R14 | 5 | `internal/rules/schema/ruleset.schema.json` describes `free_usage_name` as an optional, non-empty string | the field undescribed, or required |
| R15 | 5 | `GET /api/ruleset` answers every role whose bearer token the server accepts, as `freeUsageName`, the loaded ruleset's `free_usage_name`, and with no `freeUsageName` field when it declares none or none is loaded | a role answered without it, or the field present for none |
| R16 | 5 | `get_ruleset`'s description says it answers the ruleset's name for an ability that spends no resource, and none when the ruleset gives none | the description silent |
| R17 | 1 | A `ruleset.json` holding a field the format does not define, at its top level one whose name matches none of `id`, `name`, `format_version`, `properties`, `sheet` and `free_usage_name`, or within `sheet` one its form does not define, compared without regard to case, is refused when it loads, by an error naming the file and the field (succeeds VTT-830) | an unknown field loading, or `free_usage_name` refused |

**Candidates the sort may refuse:**
- **R10.** An answer to an older `/api/ruleset` read arriving after the
  newest leaves the description in the player panel's button titles as
  the newest answer gives it. Likely merged into VTT-753: the description
  travels on the ability object VTT-753 governs, where VTT-817 needed a row
  because the resource's name comes from another list.
- **R11.** An `abilities/*.json` holding a field the format does not
  define, any but `id`, `name`, `description`, `usage` and `compose`,
  compared without regard to case, is refused when it loads, naming the
  file and the field. Today's behaviour, held by `decodeStrict` and by no
  row; this work changes the set it names. The sort decides whether a rule
  the ticket does not change is this ticket's to record.

**Rows checked and left standing,** their evidence cells updated where a
named test is rewritten or renamed:
- **VTT-844** stays true as worded: the title still names the resource,
  its current and max and the cost, and still ends ` — not enough`; R8 puts
  the description above it. Its evidence's at-will assertions move to
  VTT-855, whose tests read the ruleset's word instead of `at will`, and
  the tests whose names say "at will" are renamed. **VTT-815** and
  **VTT-817** stand.
- **VTT-694** and **VTT-833** stay true; D9 keeps their sentences whole,
  and R7 is their sibling.
- **VTT-691**, **VTT-692** and **VTT-693** stand: which abilities a role
  reads does not change, and R5 serves the description with each.
- **VTT-621** stands; `TestQARRGetRulesetReturnsTheRouteBodyVerbatim`
  gains a degenerate-fixture guard, as it has for the sheet: the route
  serves a description for some ability of each shipped ruleset.
- **VTT-540** and **VTT-532** stand; their e2e evidence reads the DM's
  panel's attributes, and goblin-scimitar's text matches none of its words
  (P7).
- **VTT-616** stays (OD-5 A). VTT-610, VTT-753, VTT-843, VTT-846 and
  VTT-695 stand unchanged.

## The existing tests that change

By P2 to P6, none turns red from the description in the shipped
abilities or from the route's fields; the answer to OD-4 turns red the
at-will assertions below.
- **Task 1:** `TestSchemaPropertiesCoverFixtureFields` turns red when D5's
  fixture lands and green when the schema declares the field, inside the
  task (P4). `internal/rules/qa_declared_sheet_test.go`'s
  "TestQASheetSchemaDescribesTheSheet" asserts the schema's top level holds
  exactly the manifest's fields; its list gains `free_usage_name` and its
  count with it, the assertion moved, not weakened (rule 2), and it is
  cited for VTT-858 too.
- **Task 3:** `cmd/vtt/qa_get_ruleset_test.go`'s
  `TestQARRGetRulesetReturnsTheRouteBodyVerbatim` gains its guard; no
  assertion changes.
- **Task 4:** the tests that read an at-will ability's title as `at will`:
  `client/test/panel-sheet.test.ts`'s "a resource ability's button names
  the resource, its current and max, and the cost",
  `client/test/spectator-view.test.ts`'s "an at-will ability is enabled and
  says so on hover", and
  `client/test/qa-panel-sheet.test.ts`'s "an at-will button's title says
  at will and names no cost". Each takes the word its fixture's ruleset
  answer gives, or none, exactly; none is loosened (rule 2).

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R17 as
  the answers leave them; allocate ids with `requirement-id`; commit the
  ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: the format (R1 to R3, R12 to R14, R17; D1 to D6, D16, D18; items 1 and 2)

- **Files:** `internal/rules/load.go` (`compositionJSON`,
  `compositionAbility`, `loadCompositions`); `internal/rules/compile.go`
  (`compilePower`); `internal/rules/format.go` (`CompiledPower`);
  `internal/rules/schema/ability.schema.json`;
  `internal/rules/testdata/valid/abilities/strike.json` (D5);
  `manifestJSON`, `loadManifest` and `Ruleset.FreeUsageName` (D16),
  `internal/rules/schema/ruleset.schema.json` and
  `internal/rules/testdata/valid/ruleset.json`, which declares a
  `free_usage_name` in neutral words, never a game word (P8) (R12 to
  R14), VTT-830's withdrawal and R17's evidence (D18), its pointers
  re-pointed to VTT-861: the `(VTT-830)` in `ShownUsage.UnmarshalJSON`'s
  comment in `load.go` and the `// VTT-830` above tests in
  `declared_sheet_test.go`, `qa_declared_sheet_test.go` and
  `qa_property_format_test.go`; a new
  `internal/rules/ability_description_test.go`, written first and seen red
  on the base, its rulesets built by editing a copy of `testdata/valid` as
  JSON with `copyDir` and `setJSONField`, in neutral words (P8): a declared
  description kept on its ability and on no other, none for an ability
  without one or with `null`, `""` refused naming the file and the field;
  `docs/requirements.md`.
- **True when:** the new tests pass and failed before the change to
  `load.go`; `env -u CLAUDECODE go test ./internal/rules/...
  ./internal/adventure/... ./internal/mapdef/ ./internal/gateway/` passes;
  `rules.Load` of both rulesets succeeds as before; `git diff --stat main --
  rulesets` prints nothing; `go vet ./...`, `task
  check:requirements-chain`, `tools/check-new-prose.py` and `python3
  tools/check-comments.py` pass.
- **Deliberate breaks:**
  - Drop `Description` from `compositionJSON`: R1's tests turn red, the
    fixture refused as an unknown field.
  - Decode it and leave `compilePower` without it: R1's kept-description
    test turns red.
  - Accept `""`: R2's test turns red.
  - Drop `FreeUsageName` from `manifestJSON`: R12's and R17's tests turn
    red; accept an empty `free_usage_name`: R13's test turns red; leave it
    out of the schema: `TestSchemaPropertiesCoverFixtureFields` and
    "TestQASheetSchemaDescribesTheSheet" turn red; list it under
    `required`: `TestSchemaRequiredFieldsMatchLoaderEnforcement` turns red
    (R14).
  - Leave `description` out of the schema's `properties`:
    `TestSchemaPropertiesCoverFixtureFields` turns red; list it under
    `required`: `TestSchemaRequiredFieldsMatchLoaderEnforcement` turns red
    (R3).
- **The state here:** an ability may carry a description, and an empty one
  is refused; nothing served or drawn changes.

### Task 2: the shipped rulesets describe their abilities (R4; D7, D16; item 3)

- **Files:** every file under `rulesets/dnd45e-minimal/abilities` and
  `rulesets/tavern-brawl/abilities`, each given its signed-off text after
  `name`; `rulesets/dnd45e-minimal/ruleset.json`, given
  `"free_usage_name": "At will"` (D16); `internal/rules/conformance/conformance_test.go` (R4's test,
  written first and seen red on Task 1's commit); `docs/requirements.md`.
- **True when:** R4's test passes and failed before the texts;
  `TestConformanceOverRulesetsGlob` passes; `git diff --stat main --
  rulesets/*/goldens` prints nothing; no text matches P7's pattern; the
  checks of Task 1's last line pass.
- **Deliberate breaks:**
  - Remove one ability's description: R4's test turns red.
  - Write a digit into one: R4's test turns red.
- **The state here:** every shipped ability carries its description in
  words; nothing serves it yet.

### Task 3: the read surface and the agent's tool (R5 to R7, R15, R16; D8, D9, D17; item 4)

- **Files:** `internal/gateway/metadata.go` (`abilityJSON`,
  `handleRuleset`); a gateway test, in `ruleset_by_role_test.go` over
  `newByRoleTable` or a new file beside it, written first: every role that
  is served an ability of `dnd45e-minimal` reads its description as that
  ability's file declares it, read from the file as JSON; and a copy of a
  shipped ruleset with one ability's description removed, edited as JSON,
  answers that ability with no `description` key;
  `rulesetJSON`'s `freeUsageName` and its test over every role, reading
  `dnd45e-minimal`'s value from its `ruleset.json` rather than writing the
  word in Go, and over `tavern-brawl` and over no ruleset loaded, each
  answering no `freeUsageName` (R15, D17); `internal/mcp/ruleset_tool.go`
  (`getRulesetDescription`); `internal/mcp/ruleset_tool_test.go` (a new
  test of D9's and D17's words, R7 and R16);
  `cmd/vtt/qa_get_ruleset_test.go` (the guard); `docs/requirements.md`.
- **True when:** the new tests pass and failed before the change;
  `env -u CLAUDECODE go test ./internal/gateway/ ./internal/mcp/
  ./cmd/vtt/` passes; `git diff --stat main -- contract cmd/vtt/tools.json`
  prints nothing; the checks of Task 1's last line pass.
- **Deliberate breaks:**
  - Write the description only for the DM: R5's every-role test turns red.
  - Drop `omitempty`: R6's test turns red.
  - Write another ability's description: R5's test turns red.
  - Drop D9's words: R7's test turns red; drop D17's: R16's turns red.
  - Serve `freeUsageName` to the DM alone, then drop its `omitempty`:
    R15's tests turn red.
- **The state here:** `/api/ruleset` and `get_ruleset` carry each
  ability's description; the client ignores it.

### Task 4: the sheet shows it on hover (R8, R9; D10 to D13; items 5 and 6)

- **Files:** `client/src/metadata.ts` (`Ability.description`,
  `RulesetMeta.freeUsageName`); `client/src/view/player.ts` (the title
  function beside `costTitle`, `costTitle` without `at will`,
  `PanelLayout.freeUsageName`, and `renderPlayerPanel`'s use of them);
  `client/src/app.ts` (`layout` keeps the newest answer's
  `freeUsageName`, R9); `client/test/panel-sheet.test.ts`, new
  tests written first: a resource ability's title with a description,
  the description first and the cost line after one line break, and with
  none or `""`, the cost line alone as today; an ability that spends no
  resource titled by its description and the window's `freeUsageName`,
  by its description alone when the answer gives none or `""`, by the
  name alone with no description, and empty with neither (R8, R9); a
  description holding a line break kept as written; in `app.test.ts`, the
  name from the newest answer, an older answer arriving after it changing
  nothing (R9);
  `client/test/app.test.ts` (a booted window whose `/api/ruleset` answer
  gives an ability a description titles its button by it);
  `client/e2e/table.spec.ts` (D13); `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **True when:** the new tests pass and failed before the change;
  `env -u CLAUDECODE bun test client/test` and `task client:typecheck`
  pass; VTT-846's tests and VTT-844's resource-ability assertions pass
  unchanged, and the at-will assertions of the tests "The existing tests
  that change" names take R9's words under new names; `task build:client`
  has run after the last change to `client/src` and its output is in the
  commit; the prose and comment checks pass.
- **Deliberate breaks:**
  - Put the cost line first: R8's tests turn red.
  - Join with a space: R8's tests turn red.
  - Test the description against `undefined` alone: R8's `""` test turns
    red.
  - Write `at will` again for an ability that spends no resource: R9's tests
    turn red; take the word from every answer: R9's newest-answer test
    turns red.
  - Replace a description's line breaks with spaces: D12's test turns red.
- **The state here:** a player hovering an ability's button reads what it
  does, then what it costs.

### Task 5: the gate (item 7)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger`
  once, last, if a ceiling can come down; check free space and the load
  (`os.getloadavg()`); run `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion
  line; run `task e2e`. Tell the owner the client's stored mutation report
  is out of date from Task 4 and which commit it measured (SPEC-023); a
  full Stryker run is his to ask for.
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 6: the report, then the specifications

- **The report:** `docs/reports/2026-11-18-an-ability-carries-its-description.md`
  with the `implementation-report` skill and its reading review: each
  "Done looks like" item with its observation; the rows and what turned
  each red; Rule 9; G1 to G6 and each OD's answer; Phase 4a's
  adjudications under their own heading; the gate.
- **The specifications,** with the `specification` skill, from the report:
  - **SPEC-024** (OD-6): an ability may carry `description`; `""`
    refused; kept on `CompiledPower`; not in the compiled goldens; the
    schema; the shipped rulesets' texts carry no figure; the manifest may
    hold `free_usage_name`, `""` refused, kept as `Ruleset.FreeUsageName`,
    `dnd45e-minimal` naming it and `tavern-brawl` not; the manifest's
    fields as VTT-861 names them, VTT-830 no longer cited.
  - **SPEC-012:** `abilityJSON` holds `description`, absent when none;
    `rulesetJSON` holds `freeUsageName`, to every role, absent when none;
    principle 35's summary of an ability holds its description;
    `get_ruleset`'s description says both.
  - **SPEC-022:** an ability button's title as D11 gives it: the
    description and the cost line, each when present, joined by a line
    break, empty with neither; a resource ability's cost line as VTT-844
    words it, and an ability's that spends no resource the newest answer's
    `freeUsageName`; the platform writes no word of its own.
  - **SPEC-018:** an ability's description and the manifest's
    `free_usage_name` among a ruleset's unbounded strings and among what
    `/api/ruleset` carries: the first to whom its ability's display name
    reaches, the second to every participant.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the
surfaces as signatures (`go doc ./internal/rules`, `go doc
./internal/gateway`) and names, never the source: `rules.Load`,
`CompiledPower`, `Ruleset.FreeUsageName`; `GET /api/ruleset`;
`get_ruleset`; `renderPlayerPanel`'s signature, `PanelLayout` and the
`Ability` and `RulesetMeta` types; `boot` from `client/src/app.ts` with its
fetch and socket seams as `app.test.ts` uses them. Tell it: never run
`task check`, `task e2e` or Stryker; its files are new `qa-*` test files;
run client tests with `env -u CLAUDECODE bun test`; no game word in a Go
literal under `internal/` or `cmd/` (P8), so it reads shipped texts from
their files; `go-arch-lint` holds the layers; the tree is hashed before
and after it. Known and not findings: a disabled button's title is drawn
as the browser draws it (G4); a description is not shown on the DM
console's give and take; a condition is named by its id on a dot and in
the Story (the next ticket).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check that
the Go diff holds no game word (P8) and the refusal's form is SPEC-024's;
check that `git diff --stat main -- contract cmd/vtt/tools.json
rulesets/*/goldens` is empty; check each comment under `internal/`, `cmd/`
and `client/src` is a warning or a pointer; check every assertion this
work changed for being weakened rather than moved (rule 2); and read each
shipped text against its file's `compose`, for a claim the composition
does not make.
