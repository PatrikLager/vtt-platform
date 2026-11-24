# Plan: a ruleset declares forms and a catalogue of entries

- **Ticket:** `docs/superpowers/specs/2026-11-24-a-ruleset-declares-forms-and-a-catalogue-design.md`
  (blueprint principles 13, 23, 24, 25, 27 and 35; ADR-012 §2, §3, §11,
  §18, "What this overturns" and "Build order", step 1 item 5, the first of
  its three tickets).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-24 by an agent that did not write the ticket, against
    `e0bf9ef` on `feat/forms-and-the-catalogue` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker, gremlins, `task build:client`, `go clean -cache`
    and `requirement-id` were not run. Probes P1, P2 and P13 ran through
    `go test -overlay`, their files kept in the session's scratchpad;
    `git status` after each showed only the ticket and this plan. The rest
    are searches, targeted `go test` runs and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-24-a-ruleset-declares-forms-and-a-catalogue.md`.
- **Base:** `e0bf9ef`. **Branch:** `feat/forms-and-the-catalogue`.
- **Owner's rulings taken in:**
  - 2026-11-24: ADR-012 step 1 item 5 is three tickets, in order: this one,
    forms and catalogue entries in the ruleset format, changing no sheet;
    then a sheet's references, slots and sets, with the campaign's state per
    reference, which only the DM and the agent change until step 2; then the
    actions a sheet reaches through its references, in place of
    `Actor.ability_ids`. `dnd45e-minimal` gets the first real catalogue (the
    alpha campaign runs it); `tavern-brawl` only what tests need.
  - 2026-11-02: equipment and its numbers are the ruleset's content; a sheet
    refers to the catalogue and never copies; the forms are closed.
  - 2026-11-04: the rulesets in the repository are the project's own systems,
    and format v2 is replaced outright.
  - Standing: a near-theoretical edge case gets no code, no test and no debt
    (2026-10-21); no migration (2026-09-04); one source per fact, pointed at
    (2026-09-23); gate and tooling work is paused (2026-08-27); no counts or
    measured figures in prose meant to last; a code comment is a warning or a
    pointer (rule 10); the client's full mutation run only when the owner
    asks; push and merge on his word.
- **The ticket was revised after this verification**, before Task 0's
  commit, to take in G1 to G9. Where the checks and gaps below find the
  ticket wrong or short, or name what it names, they describe the draft
  that was verified; the committed ticket has taken each gap in.
- **Signed off:** 2026-11-24 by the owner: OD-1 A, OD-4 A, and the plan
  "Godkänd" with OD-3 A, OD-5 A, OD-6 A, OD-7 B, D5, D11 and D16. OD-2 A
  after the owner asked whether it was a shortcut ("Bygger vi nu en
  'fuskväg' för att vi inte har utrustning/vapen hantering?"): told that a
  property a form lists is a key a sheet may hold until the next ticket, and
  offered closing that now by making the sheet a form, he chose "Godta hålet
  till 5b".
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off. The `R`
  labels below are this plan's; each task's citations read through them.
  R1 VTT-993, R2 VTT-994, R3 VTT-995, R4 VTT-996 (succeeds VTT-973 in Task
  1), R5 VTT-997, R6 VTT-998, R7 VTT-999, R8 VTT-1000, R9 VTT-1001, R10
  VTT-1002, R11 VTT-1003, R12 VTT-1004, R13 VTT-1005, R14 VTT-1006, R15
  VTT-1007, R16 VTT-1008, and VTT-1009, which the sort added to succeed
  VTT-625 (a form's key joins the bounds the schemas state) in Tasks 1 and
  2. The sort refused what the plan refuses below, each held elsewhere.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol it names is where it says: `Load`, `loadV2`,
   `manifestJSON`, `loadConditions`, `loadAtoms`, `loadCompositions` and
   `decodeStrict` in `internal/rules/load.go`; `Ruleset` with `Properties`,
   `Sheet`, `Conditions`, `Atoms` and `Compiled` in
   `internal/rules/format.go`; `internal/rules/schema/` and
   `internal/rules/schema_test.go`; `internal/rules/testdata`;
   `internal/rules/conformance`; `rulesetJSON` and `handleRuleset` in
   `internal/gateway/metadata.go`; `internal/mcp/ruleset_tool.go`;
   `RulesetMeta` in `client/src/metadata.ts`; every specification under
   "Specifications this moves". `longsword-strike.json` binds `"prof": 3`
   and `"damage_expr": "1d8 + @caster.str"`, and `crossbow-shot`,
   `goblin-scimitar` and `goblin-shortbow` bind theirs (P7).
   `~/dev/RPTool/rptool-rules.mtlib` is a zip whose `library/public/data/`
   holds `abilities/`, `conditions/` and `templates/`. **One citation is
   wrong:** `templates/WeaponDamage.json` names no template it extends — its
   `extends` is `[]`; `FireDamage.json`, which extends `ImplementDamage`, is
   one that does (G1).
2. **"Done" is an observation. By command for items 1, 2 and 4, by reading
   for 3 and 5.**
   - Items 1 and 2: a `ruleset.json` declaring `forms` or `catalogue` is
     refused today as `json: unknown field`, for both shipped rulesets and
     `internal/rules/testdata/valid` alike (P1); a `catalogue/` or `forms/`
     directory holding a file that is not JSON loads (P2). A test that a
     form or an entry is kept, or that a bad one is refused, fails on
     `e0bf9ef` whichever place on disk OD-6 picks.
   - Item 3: nothing reads `extends`, so its tests fail today for the same
     reason.
   - Item 4: `TestConformanceOverRulesetsGlob` passes today for both shipped
     rulesets (P3). What item 4 adds is the catalogue the suite then loads;
     "the weapon entries carrying the numbers the abilities bind" names no
     observation that would fail if an entry carried another number (G4).
   - Item 5: `rulesetJSON` has no field for forms or a catalogue, so a
     gateway test reading them fails today. `get_ruleset` returns the route
     verbatim (VTT-621), so it answers them once the route does; its
     description is what moves (G8).
   - Item 6 names the gate.
3. **Each rule is breakable. By reading.** Each rule line maps to a row of
   "Requirements" with what turns it red. The rules are wider than the Done
   items that observe them: a form's description, an entry's id, name and
   description, and an entry id's bound are refused by the rules and named
   by no Done item (G2). The rows observe them.
4. **The scope matches the claim. By command, then reading.**
   - Callers: `rules.Load` is called outside tests by `composeServer`,
     `cmd/vtt/mcp.go`, `internal/rules/conformance`'s `Run` and
     `internal/adventure/conformance`; `handleRuleset` is registered once in
     `Handler` and writes the one `rulesetJSON`; `/api/ruleset` is read by
     `get_ruleset` and by `fetchRuleset` in `client/src/metadata.ts`, the one
     place `RulesetMeta` is named. Gateway tests build `rules.Ruleset` by
     literal with named fields, so new fields break none.
   - Every switch on a property's type compares for equality with
     `PropertyNumber` or `PropertyResource` (`git grep -n
     'PropertyNumber\|PropertyResource\|\.Type ==\|\.Type !='` over non-test
     Go), so a third type would be refused everywhere a number or a
     resource is wanted, but `loadProperty`'s switch and `checkSheet`'s
     `values` would need a branch (OD-1 B's cost).
   - The ticket's file list omits tests and records that move (G6).
5. **No recorded decision is contradicted silently. By reading.**
   - ADR-012 §18: "Text in … a catalogue entry is offered to the DM and never
     sent to a player by itself." Item 5 serves each entry's description;
     the ticket leaves who is served open and does not name §18 (G3). D15
     reads §18 against principle 35 and records the reading.
   - SPEC-024's principle-24 line ("A property is a `number` or a
     `resource`, and a ruleset declares no other kind") is overturned only
     under OD-1 B; the ticket lists SPEC-024 and leaves the types open.
   - SPEC-012's per-role answers move under OD-4; the ticket lists SPEC-012.
   - Format v2's §10 is overturned, as the ticket says, through ADR-012's
     "What this overturns".
   - The owner's ruling that this ticket changes no sheet holds under every
     recommendation below; under OD-2 A a property a form lists is also a key
     a sheet may hold until the next ticket makes the sheet a form (G9).
6. **The records the work moves are named. By command, then reading.** Every
   path under "Specifications this moves" resolves, and the new one is
   written `New:`. By reading: SPEC-025 gains a consequence under OD-2 A
   (G9); SPEC-022 moves only under OD-1 B with OD-2 A, where the panel's
   `Property.type` widens (G9). No other specification names a catalogue
   (`grep -ln 'catalogue\|catalog' docs/specifications/*.md` prints
   nothing).

## The gaps this plan inherits

- **G1. Rule 9: the ticket's MapTool paragraph, corrected and completed
  (P10).** `templates/WeaponDamage.json`'s `extends` is empty; the shape
  the ticket means is `FireDamage.json`'s. The rule framework's `extends` is
  a list, resolved when an ability is used, not at load, and
  `resolveAbility` in `src/TemplateResolver.ts` marks a template visited only
  after its parents, so a cycle recurses without end; refusing a dangling or
  cyclic `extends` at load is this platform's own (principle 25). MapTool's
  core, which the ticket summarises, declares a property list per token
  type (`CampaignProperties.tokenTypeMap`) and a visibility per property
  (`TokenProperty`'s `ownerOnly` and `gmOnly`); both bear on OD-2 and OD-4,
  and the ticket does not draw on them.
- **G2. The rules are wider than the Done items.** Item 1 refuses a form
  with no key or name, not one with no description; item 2 refuses no entry
  whose id, name or description is missing and names no bound on an entry's
  id, though SPEC-018 is listed and the next ticket writes entry ids into
  the log. R2, R7 and R8 observe them.
- **G3. ADR-012 §18 is not named** where the ticket asks who is served.
  D15 and OD-4.
- **G4. Item 4 writes each weapon's numbers twice.** The abilities keep
  binding them (the ticket: "Its abilities are unchanged") and the entries
  carry them; nothing in Done shows the two agree. OD-7.
- **G5. "And for armour" names no number.** No ability binds an armour
  number today; `ac` is a plain number on every sheet. Which armour, and
  what it carries, is content this plan proposes (D11).
- **G6. "What it touches", measured.** Not named, and changed:
  `internal/rules/qa_declared_sheet_test.go`
  (`TestQASheetSchemaDescribesTheSheet` pins the manifest's top-level
  field list and its length); `internal/rules/id_bound_internal_test.go`
  (`TestTheSchemasStateTheIDBound`) and
  `internal/rules/qa_property_format_test.go`
  (`TestQAPFSchemasStateTheBoundsLoadEnforces`), for VTT-625;
  `internal/rules/testdata/valid`, without which
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` skips every new required
  field silently (P5, D13); `internal/mcp/ruleset_tool_test.go` (G8);
  `internal/rules/conformance/conformance_test.go` (D12);
  `docs/requirements.md` (VTT-973's field list, VTT-625's words). Under
  OD-1 B, also VTT-611, VTT-614, VTT-615, VTT-618, VTT-677, VTT-678 and
  VTT-826, worded for two types, and, with OD-2 A, `client/src/player.ts`,
  `cmd/vtt/webdist` and `tools/toolgen/main.go`'s `add_actor` docs. Named
  and, under the recommendations, unchanged: `rulesets/tavern-brawl/` and
  `internal/rules/conformance/conformance.go` (D14).
- **G7. Whether an entry carries every property its form lists** is not
  said. D5.
- **G8. `get_ruleset`'s description moves** (principle 34; VTT-694, VTT-833,
  VTT-853 and VTT-860 are its precedents), and the ticket does not say so.
  Under OD-1 B its "its type, number or resource" becomes false.
- **G9. SPEC-025 and SPEC-022.** Under OD-2 A a property a form lists is a
  key `SheetKeyFault` accepts on a sheet until the next ticket; SPEC-025
  gains that consequence. SPEC-022 moves only under OD-1 B with OD-2 A.

## Measurements this plan stands on

All at `e0bf9ef`.

- **P1.** An overlay test in `internal/rules` copies each of
  `rulesets/dnd45e-minimal`, `rulesets/tavern-brawl` and `testdata/valid`,
  adds `"forms": []`, and separately `"catalogue": []`, after
  `format_version`, and calls `rules.Load`: each is refused, `json: unknown
  field "forms"` or `"catalogue"`.
- **P2.** The same copies with a `catalogue/x.json` and a `forms/x.json`
  that are not JSON: each loads. A directory `rules.Load` does not read is
  ignored, so a misspelled `catalog/` would load as no catalogue.
- **P3.** `go test ./internal/rules/conformance/ -run
  'TestConformanceOverRulesetsGlob|TestEveryShippedAbilityCarriesADescriptionWithNoDigit'`
  passes, with a subtest for each shipped ruleset.
- **P4.** `git ls-files '*ruleset.json'` lists every manifest; `git grep -l
  -F '"format_version": "3"' -- '*.go' '*.ts'` and its compact twin list
  the Go test files holding an inline manifest, in `internal/rules`,
  `internal/gateway` and `internal/adventure`, and no TypeScript. Under an
  optional `forms` and an optional `catalogue/`, none of them changes.
- **P5.** `go test ./internal/rules/ -run
  'TestSchemaRequiredFieldsMatchLoaderEnforcement$' -v` skips today the
  sheet's `spends`, `conditions` and `abilities` cases: the walker tests a
  required field only where `testdata/valid` demonstrates it, and
  `t.Skipf`s the rest.
- **P6.** A script over every directory holding a `ruleset.json` compares
  its abilities' ids with its conditions' ids: no id is both.
- **P7.** What dnd45e-minimal's abilities bind per weapon, by reading:

  | Ability | Weapon | `prof` | `damage_expr` | Delivery |
  |---|---|---|---|---|
  | `longsword-strike` | longsword | 3 | `1d8 + @caster.str` | `melee-delivery`, reach 1 |
  | `crossbow-shot` | crossbow | 2 | `1d8` | `ranged-delivery`, distance 15 |
  | `goblin-scimitar` | scimitar | 2 | `1d6 + 2` | `melee-delivery`, reach 1 |
  | `goblin-shortbow` | shortbow | 2 | `1d8` | `ranged-delivery`, distance 10 |

  `staggering-blow` and `hunters-flurry` bind weapon numbers and name no
  weapon; `rally` names none. The compiled goldens under
  `rulesets/dnd45e-minimal/goldens/compiled/` pin each binding as it
  compiles (`"roll": "1d20 + @caster.str + (3)"`, `"delta_expr": "0 - (1d8 +
  @caster.str)"` for the longsword).
- **P8.** The route tests over shipped rulesets compare the answer with the
  loaded ruleset rather than with literals (`qaRRCheckProperties`,
  `qaRRPropertyCount`, `qaRRRulesetDirs`), so a property dnd45e-minimal
  adds changes no test's outcome.
- **P9.** `scenarios/adventure-night.json` and `goblin-fight.json` are
  served `dnd45e-minimal`; `scenarios/goldens/adventure-night`'s
  `state.json` holds `"RulesetVersion": "1"` and its `stream.json` an entry
  naming version `"1"`. A new version of dnd45e-minimal moves that golden;
  no test pins the shipped version otherwise.
- **P10.** RPTool at `63862e4`: `rptool-rules.mtlib`'s
  `library/public/data/templates/WeaponDamage.json` (`"extends": []`) and
  `FireDamage.json` (`"extends": ["ImplementDamage"]`);
  `rule-framework/src/TemplateResolver.ts`'s `resolveAbility` (parents
  visited before the template is marked; an unknown template thrown at use;
  `{...template.defaultParams, ...ability.params}`, so the child's value
  wins); `rule-framework/data/templates/MeleeAttack.json` and
  `RangedAttack.json` read the weapon from the token's own
  `EquippedWeapon` object (`die`, `proficiency`, `enhancement`, `range`).
  MapTool at `maptool` `f4b7fef6c`: `CampaignProperties.tokenTypeMap` and
  `TokenProperty`'s fields.
- **P11.** By reading `client/src/player.ts`: `shownValue` takes `type:
  "number" | "resource"`, and `sheetFor`'s no-sheet branch passes each
  property's `p.type` to it; widening `Property.type` in
  `client/src/metadata.ts` fails `client:typecheck` there. Nothing in
  `client/src` reads a field of `RulesetMeta` it does not name.
- **P12.** By reading: `TestQASheetSchemaDescribesTheSheet` asserts the
  manifest schema's top-level fields are exactly `id`, `name`,
  `format_version`, `version`, `properties`, `sheet` and `free_usage_name`,
  and that `required` holds four; `TestTheSchemasStateTheIDBound` and
  `TestQAPFSchemasStateTheBoundsLoadEnforces` hold a row per bounded id.
- **P13.** An overlay test in `internal/rules` parses
  `(@damage_dice)d(@damage_die)`, `@damage_dice d @damage_die` and `(2 *
  @damage_dice)d(@damage_die)` with `rules.Parse` and evaluates each with
  `rules.Eval` over `damage_dice` 1 and `damage_die` 8 and a roller that
  rolls each die's highest face: each parses, `HasDice` is true, and they
  total 8, 8 and 16. Today's grammar rolls a die count and a die size held
  as numbers, a weapon's dice taken twice among them.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves; a test whose assertion changes,
  changes because a decision here changed the rule, and the task says
  which); 3 is not engaged — no `.proto` changes, and `/api/ruleset` is
  JSON outside the wire contract; 4 (no fold changes; no event is added);
  5 (no game word in `internal/` or `cmd/` Go, tests included: semgrep's
  `no-game-system-vocabulary-in-engine`; dnd45e-minimal's content stays
  under `rulesets/`); 8 and 10 (names, not line numbers; a new comment is a
  warning or a pointer, to SPEC-027, SPEC-024 or a requirement); 9
  (answered below).
- **Principles** 13 (every form and entry carries its name and
  description), 23 (the words `weapon`, `armor` and their values live only
  in `rulesets/`), 24 (forms and their values are a closed list the loader
  checks), 25 (every fault named at load, file and field), 27, 35.
- **SPEC-018's mirror rule:** an id the loader bounds is bounded at
  `maxIDBytes` and stated in its schema (VTT-625).
- **The mechanical gates**, as `docs/superpowers/plans/2026-11-23-the-log-names-its-ruleset.md`
  lists them. The Go mutation gate's `PACKAGES` hold `internal/rules`,
  `internal/rules/conformance`, `internal/gateway` and `internal/mcp`: every
  new branch gets a test that kills it, every new bound a test at it and
  one past it. A client change is committed with its `cmd/vtt/webdist` when
  `task build:client` changes it; client tests are measured with `env -u
  CLAUDECODE`; `tools/check-new-prose` and `python3 tools/check-comments.py`
  run before each commit; stage and commit in separate calls; a new test is
  inserted after a closing brace, not inside a neighbour's doc comment.

## Rule 9: how MapTool does this

Read at the commits P10 names.
- MapTool's core has no item and no catalogue. A campaign declares token
  types, each its own list of properties with a display name, a default
  and a visibility (`ownerOnly`, `gmOnly`); a value is a string on the
  token.
- The owner's rule framework keeps a weapon as a JSON object on the token
  (`EquippedWeapon`: `die` as `"1d12"`, `proficiency`, `enhancement`,
  `range`), which the attack templates read: a copy per token, the opposite
  of principle 27. Its templates `extends` a list of templates, ancestors
  first, a diamond once, an unknown one thrown when an ability is used, and
  no cycle check.
- **Borrowed:** a weapon's numbers as named values on the thing itself
  (`proficiency`, `range`, the damage die); inheritance in which the child's
  own value wins over its parent's (`defaultParams` under `params`).
- **Not borrowed:** the copy on the token (an entry lives once, in the
  ruleset); a list of parents (ADR-012 §3: an entry "may extend another");
  a fault found at use (refused at load, principle 25); the unbounded walk
  (a cycle is refused).
- **Weighed, and left to the owner:** MapTool's property list per token
  type is OD-2 B's shape; its per-property visibility is OD-4 C's; its
  damage die as text is OD-1 B's.

## Decisions

1. **D1. Forms live in `ruleset.json`, as an optional `forms` list.**
   `manifestJSON` gains `Forms *[]formJSON`; `rules.Ruleset` gains `Forms
   []Form`, in declared order, nil when none is declared or it is `null`.
   **Forced by** a form naming property keys declared in the same file, the
   manifest already holding the vocabulary and the sheet (which ADR-012 §3
   calls a form), and P4: optional, it changes no manifest and no inline
   literal. `TestQASheetSchemaDescribesTheSheet`'s field list gains `forms`
   (P12) and VTT-973 is succeeded.
2. **D2. A form.** `{"key", "name", "description", "properties"}`: the
   first three required and non-empty, `properties` an optional list of
   property keys. `Form` is `{Key, Name, Description string; Properties
   []string}`. Refused, naming `ruleset.json` and `forms[<i>].<field>`: a
   key, name or description absent or empty; a key longer than `maxIDBytes`
   (checked in `checkManifestBounds`, after the property keys); a key an
   earlier form declares; under `properties[<j>]`, a key no property
   declares, a key the form lists already, and, under OD-1 A, a property
   that is not a `number`. A form may list none. **Forced by** ticket item
   1, rule 1 and G2; the bound by SPEC-018's mirror rule. A key's charset is
   not constrained: nothing reads a form's key in an expression.
3. **D3. Entries.** Under OD-6 A, one entry per file under an optional
   `catalogue/`, read by a new `loadCatalogue` through `jsonFilesIn` and
   `decodeStrict`: `{"id", "name", "description", "form", "extends",
   "values"}`, the first four required, `extends` optional, `values` an
   optional object from a property key to its value. `rules.Ruleset`
   gains `Catalogue map[string]*Entry`, keyed by id as `Conditions` is,
   always non-nil after `Load`; `Entry` is `{ID, Name, Description, Form
   string; Values map[string]int}`, its values those after D7. Loaded and
   checked last in `loadV2`, after `checkSheet`, so every refusal a ruleset
   without a catalogue meets keeps its message and its order.
4. **D4. An entry's refusals**, each naming the entry's file and the
   field: an `id`, `name`, `description` or `form` absent or empty; an `id`
   longer than `maxIDBytes` (and one at it loads); an `id` an earlier file
   declares; a `form` no form declares; and, under `values.<key>`, a key
   the entry's form does not list, a value written `null`, and a value that
   is not an integer within an `int32`. Values are decoded as
   `map[string]json.RawMessage`, because `null` decodes into an integer as
   0 without error. An entry holding a field the entry format does not
   define is refused, naming the file and the field, and a case variant of
   a defined field (`"ID"`) is read as it, as `decodeStrict` does for a
   manifest. **Forced by** items 2, rules 2 and 3, and G2; the id's
   bound by SPEC-018's mirror rule, since the next ticket writes entry ids
   into the log; the `int32` by `Actor.attributes`, where an expression
   reads the numbers an entry's will stand beside.
5. **D5. An entry need not carry every property its form lists** (G7),
   and holds no value for one it neither carries nor inherits through
   `extends` (VTT-1000 beside VTT-1002). **Forced by** `extends` (a
   base entry may set some, its children the rest) and SPEC-025's sheet,
   which no absent key refuses.
6. **D6. Under OD-3 A, one namespace.** After the catalogue loads, an id
   held by two of entries, abilities and conditions is refused, naming the
   later file in byte order and the field `id`, and the other file in the
   message. Measured: no shipped or fixture ruleset holds an ability and a
   condition under one id (P6).
7. **D7. `extends`.** After every entry loads, entries are resolved in id
   order: an entry's values are those of the one it extends, after that
   one's own resolution, with its own over them. Refused on the field
   `extends`, naming the extending entry's file: written empty; naming an
   entry the catalogue does not hold; under OD-5 A, naming one of another
   form; and a cycle, reported once, on the file of the cycle's lowest id,
   naming every entry in the cycle in id order. The walk keeps a visited
   set, so a cycle ends it. **Forced by** item 3 and principle 25; P10 is
   the counter-example.
8. **D8. The read surface, under OD-4 A.** `rulesetJSON` gains `forms`,
   each `{key, name, description, properties}` in declared order, and
   `catalogue`, each `{id, name, description, form, values}` sorted by id
   with `slices.SortFunc` (the catalogue is a map), `values` an object from
   key to number. Both start as empty slices, as SPEC-012 requires of every
   list; a form listing none answers `"properties": []` and an entry
   carrying none `"values": {}`. Both are every role's. `extends` is not
   served: the resolved values are what a reader reads, and nothing after
   load keeps it. **Preference:** the JSON names `forms` and `catalogue`,
   ADR-012's words.
9. **D9. `getRulesetDescription` gains one sentence**: it answers, to
   every role, `forms`, each with its key, name, description and the keys
   of the properties its entries carry, and `catalogue`, each entry's id,
   name, description, form and values, after what it extends, keyed by
   property; each `[]` when the ruleset declares none. **Forced by**
   principle 34 and G8.
10. **D10. The client's types.** `RulesetMeta` gains `forms: FormDef[]` and
    `catalogue: CatalogueEntry[]`, typed as D8 serves them; nothing reads
    them yet. Interfaces erase, so `task build:client` is expected to leave
    `cmd/vtt/webdist` unchanged, and the task checks it. **Forced by** the
    rule `client/src/metadata.ts` states for `MapsResponse`: a response
    type that hid a field would be lying about the wire.
11. **D11. dnd45e-minimal's catalogue, under OD-1 A and OD-2 A.** New
    `number` properties `proficiency`, `damage_dice`, `damage_die`, `range`
    and `armor_bonus`, each with a name and a description holding no digit;
    forms `weapon` (proficiency, damage dice, damage die, range) and `armor`
    (armor bonus); entries `longsword`, `crossbow`, `scimitar` and
    `shortbow` with P7's numbers — a damage of `1dN` as `damage_dice` 1 and
    `damage_die` N, `range` only on the two ranged weapons, since a melee
    ability's reach 1 is bound alike by `staggering-blow` and `rally`, which
    name no weapon — and `chainmail` and `leather-armor` with an
    `armor_bonus`. The scimitar's `+ 2` stays the goblin's: it is not a
    number of the weapon. Its abilities, sheet, atoms and goldens do not
    change. **Preference:** the words ("armor", as the ruleset's own Armor
    Class spells it) and the armour's numbers, which no ability binds
    (G5); the owner may name others at sign-off.
12. **D12. A shipped entry's description holds no digit**, as an ability's
    must (VTT-850), so its words stay true whatever its values become. A
    test beside `TestEveryShippedAbilityCarriesADescriptionWithNoDigit`
    reads `rulesets/*/catalogue/*.json` and fails when it finds none.
13. **D13. `internal/rules/testdata/valid` carries a form and two
    entries**, one extending the other, over a `number` property only that
    form lists (so no test that edits the fixture's other properties meets a
    new refusal first). `TestSchemaRequiredFieldsMatchLoaderEnforcement`
    then reaches the form's and the entry's required fields, which it would
    otherwise skip silently (P5); its and
    `TestSchemaPropertiesCoverFixtureFields`' hand-kept tables gain a row
    for the entry schema.
14. **D14. Unchanged:** `conformance.Run` (it loads through `rules.Load`,
    and `TestConformanceOverRulesetsGlob` covers every shipped ruleset);
    `rulesets/tavern-brawl/` (every test of the format reads fixtures); the
    contract, both folds and every event (rule 4).
15. **D15. ADR-012 §18, read.** An entry's `description` is its
    explanation in the ruleset's words (principle 35), served as a
    property's and a condition's are; §18's text is the story a cell, a
    branch, a menu option or an entry offers the DM (principle 32), which no
    entry carries yet and which arrives as a field of its own. SPEC-027
    states the reading.
16. **D16. dnd45e-minimal keeps `"version": "1"`.** **Preference.** No
    outcome is decided otherwise: its abilities, the rules that decide
    outcomes, do not change (principle 28). A new version moves
    `scenarios/goldens/adventure-night` (P9); the owner may name one at
    sign-off.
17. **D17. The records.** A new SPEC-027 holds the forms, the catalogue,
    `extends`, their refusals and D15's reading, and points at SPEC-012 for
    who reads them. SPEC-024, SPEC-012, SPEC-018 and, under OD-2 A,
    SPEC-025 each move for their part (Task 7).

## Open decisions for the owner

- **OD-1. Which property types an entry may carry. Recommended: A.**
  P7 is what must be carried: a proficiency, a damage of one die, a range.
  - **A: `number` alone.** A damage of `1d8` is two numbers, `damage_dice`
    1 and `damage_die` 8. Cost: none at load beyond D4's integer check,
    none in the schema's property enum, none on the read surface's
    `properties` or the client, none in SPEC-018 (an integer carries no
    text). The expression grammar already rolls two numbers as dice, a
    weapon's dice taken twice included (P13), so step 2 can roll a weapon's
    damage from them without a new type. What it costs: the catalogue
    shows "1" and "8" where the table says "1d8"; ADR-012 §7's "a dice
    property taken a number of times" is step 2's dice ticket, which may
    re-encode these entries (our
    own rulesets, no migration). A form lists `number` properties only; a
    `resource` on an entry is per-reference state, the next ticket's
    (ADR-012 §3).
  - **B: `number` and `dice`**, a dice value being one term `NdM` that
    `Parse` accepts within the grammar's bounds. At load: a case in
    `loadProperty` (refusing a resource's fields), a refusal of a dice key
    in a sheet section's `values`, a parse per value. In the schema: the
    property enum and a value shape, and a test that the enum matches the
    loader, since none does today. On the read surface, under OD-2 A: the
    `properties` list serves `"dice"`, so `Property.type` widens and
    `sheetFor` fails `client:typecheck` (P11), SPEC-022 moves, the bundle is
    rebuilt, `get_ruleset`'s "number or resource" and `add_actor`'s docs are
    reworded, and VTT-611, VTT-614, VTT-615, VTT-618, VTT-677, VTT-678 and
    VTT-826 are succeeded, each worded for two types; under OD-2 B, none of
    that, the type living on forms alone. SPEC-018: the text is bounded by
    what parses and reaches no log. The rule framework keeps its die as
    text (P10).
  - **C: B and `tags`** (light, versatile): a declared word list on the
    property, a value a set of its words, three refusals, the list on the
    read surface. No ability binds a tag today, so nothing would carry one.
- **OD-2. Where a form's properties are declared, and when the sheet
  becomes a form. Recommended: A, the sheet in the next ticket.**
  - **A: a form lists keys of the one `properties` vocabulary.** ADR-012 §2
    names every property once and §3 lets a form override a property's
    expression or thresholds, which presumes a property outside the form.
    Cost: dnd45e-minimal's weapon and armour numbers join `properties`,
    served to every role (P8: no test moves); until the sheet is a form, a
    property a form lists is also a key `SheetKeyFault` accepts on a sheet
    and an expression may read, so an author could copy a weapon's number
    onto a sheet and nothing would refuse it. Only the DM and the agent
    write sheets until step 2 (the owner's ruling), and the next ticket
    closes it. SPEC-025 gains the consequence.
  - **B: each form declares its own properties**, key, type, name and
    description, scoped to the form, as MapTool's token types do (P10).
    `properties`, the client and every sheet check stay as they are, and
    no sheet can hold an entry's key. Cost: a second declaration path; a
    key may mean two things in two forms; when the sheet becomes a form,
    either every manifest's `properties` moves into it (P4's whole set) or
    two places to declare a property remain.
  - **C: A, and a property a form lists is not a sheet key now.** Closes
    A's hole at once, but changes `SheetKeyFault`, `add_actor`'s,
    `adventure.Load`'s and `sheetsFit`'s refusals and SPEC-025 — a sheet
    change the owner kept out of this ticket, which the next ticket would
    redo when the sheet becomes a form.
- **OD-3. One id namespace or several. Recommended: A.**
  - **A: one, across entries, abilities and conditions** (D6). ADR-012
    makes a condition an entry (§11) and an entry with a procedure an
    action (§3); the next ticket writes entry ids into the log beside
    `ability_ids`, and a namespace joined after that would meet logs where
    one id names two things. Cost: one check and its test; no ruleset in
    the tree changes (P6).
  - **B: entries among themselves only**; whichever ticket brings
    conditions or abilities into the catalogue joins the namespaces, after
    logs may hold an entry id equal to an ability's.
- **OD-4. Who is served the catalogue. Recommended: A.**
  - **A: every role**, as conditions, properties and the sheet are (D8,
    D15). An entry holds no procedure and no story, so nothing in it is
    what the abilities' filter guards (an offer, SPEC-012). Visibility per
    form arrives with its own ticket, as SPEC-024 says of a property's.
    Cost: none beyond the every-role tests.
  - **B: the DM and the agent; a player and a spectator none until the
    next ticket**, which could answer a player the entries the sheets they
    control refer to, as `abilitiesReadBy` does. Cost: a role filter and
    its tests now, and a player reading no weapon the ruleset declares.
  - **C: a `visibility` per form now** (`seen`, `owner`, `dm`, ADR-012 §2;
    MapTool's `ownerOnly` and `gmOnly`). Cost: a field, its loader check and
    schema, a filter per role on the route, and SPEC-024's consequence on a
    property's visibility answered for forms but not properties.
- **OD-5. Whether an entry may extend one of another form. Recommended:
  A.**
  - **A: no** (D7). Every inherited value is then of a property the
    child's form lists. Every `extends` between entries in the contract
    drafts stays within one form. Cost: one refusal.
  - **B: yes, and an inherited value for a property the child's form does
    not list is refused** (or dropped). Cost: the rule and its two tests;
    a parent's later change can break a child of another form.
  - **C: a child names no form and takes its parent's**, as the drafts'
    OSRS and Shadowrun encodings write it. Cost: item 2's "an entry naming
    no form" is refused only for an entry that extends none — the ticket's
    rule reworded.
- **OD-6. Where entries live on disk. Recommended: A.**
  - **A: one file per entry under `catalogue/`**, as `conditions/` and
    `abilities/` are, the two kinds ADR-012 makes entries. Each refusal
    names the entry's own file. Cost: a new `entry.schema.json`, a row in
    the walker's and the coverage test's tables (D13), a row in
    `TestTheSchemasStateTheIDBound` and
    `TestQAPFSchemasStateTheBoundsLoadEnforces`; a misspelled directory
    loads as no catalogue, as any other would (P2) — the next ticket's
    references would be refused against it, so no check is added.
  - **B: a `catalogue` list in `ruleset.json`.** No new schema file; the
    manifest's field list gains a second name (P12, VTT-973); every refusal
    names `ruleset.json` and `catalogue[<i>]`; one file grows with every
    item the ruleset holds.
- **OD-7. The weapons' numbers in two places until the third ticket (G4).
  Recommended: B.**
  - **A: a test pairs each dnd45e-minimal ability with its weapon entry**
    and asserts the bindings equal the values. Cost: a hand-kept pairing
    of the ruleset's own ids in a Go test under `internal/` (rule 5's
    spirit), deleted by the third ticket.
  - **B: nothing holds them equal now.** Nothing reads an entry's numbers
    until the third ticket, so a drift changes nothing at the table: the
    abilities' bindings are what is rolled. When the third ticket makes an
    ability read its weapon's numbers, the compiled goldens, which pin
    today's bindings (P7), change for any weapon whose entry differs, and
    the drift shows there. The report names the interim copy.

## Requirements

Proposed for Task 0's sort, under the recommended answers, to be allocated
after sign-off. Each row reads `**OPEN — no test yet**` until its task cites
its tests.

| R | Item | Rule (the register's row, in short) | Red under |
|---|---|---|---|
| R1 | 1 | `rules.Load` loads a `ruleset.json` declaring `forms`, and `Ruleset.Forms` holds each form's key, name, description and property keys as written, in declared order; one declaring none, or `null`, loads with none | a form lost, changed or reordered |
| R2 | 1 | `rules.Load` refuses, naming the file and `forms[<i>].<field>`, a form whose key, name or description is absent or empty, whose key is longer than 128 bytes of UTF-8, or whose key an earlier form declares, and loads one whose key is exactly 128 bytes | loads past a fault, or refuses at the bound |
| R3 | 1, OD-1 | `rules.Load` refuses, naming the file and `forms[<i>].properties[<j>]`, a form listing a key no property declares, a key it lists already, or a property that is not a `number` | loads |
| R4 | D1 | A `ruleset.json` holding a field the format does not define — at its top level one matching none of `id`, `name`, `format_version`, `version`, `properties`, `sheet`, `free_usage_name` and `forms`, or within `sheet` or a form one its form does not define — compared without regard to case, is refused, naming the file and the field (succeeds VTT-973) | loads |
| R5 | 2 | `rules.Load` loads every `catalogue/*.json` as an entry, and `Ruleset.Catalogue` holds each by its id with its name, description and form as written, and its values resolved through its `extends`; a ruleset with no `catalogue/` loads with none | an entry lost or changed |
| R6 | 2 | `rules.Load` refuses, naming the entry's file and the field, an entry whose id, name, description or form is absent or empty, whose id is longer than 128 bytes of UTF-8, whose id another entry declares, whose form no form declares, or that holds a field the entry format does not define; and loads one whose id is exactly 128 bytes | loads past a fault, or refuses at the bound |
| R7 | 2 | `rules.Load` refuses, naming the file and `values.<key>`, a value for a property the entry's form does not list, a value written `null`, and one that is not an integer within a 32-bit signed integer | loads |
| R8 | D5 | An entry that carries no value for a property its form lists, and extends no entry holding one, loads, and holds no value for it | refused, or a value held |
| R9 | OD-3 | `rules.Load` refuses a ruleset in which one id names two of an entry, an ability and a condition, naming both files and the field `id` | loads |
| R10 | 3 | An entry naming another in `extends` holds every value of that one, after its own extends, that it does not set, and its own values over them | a value missing, or the parent's over the child's |
| R11 | 3, OD-5 | `rules.Load` refuses, naming the extending entry's file and `extends`, an `extends` written empty, naming an entry the catalogue does not hold, or naming one of another form, and a cycle of entries extending each other, naming every entry in it | loads, or does not return |
| R12 | 5, OD-4 | `GET /api/ruleset` answers every role whose bearer token the server accepts, as `forms`, each form the loaded ruleset declares, with its key, name, description and property keys, in declared order, and `[]` when it declares none or none is loaded | a form missing, reordered or withheld from a role; `null` |
| R13 | 5, OD-4 | `GET /api/ruleset` answers every role whose bearer token the server accepts, as `catalogue`, each entry with its id, name, description, form and its values after what it extends, sorted by id, and `[]` when there is none or no ruleset | an entry missing, unsorted, unresolved or withheld; `null` |
| R14 | D9 | The MCP tool `get_ruleset`'s description says that it answers, to every role, the forms and the catalogue, each entry's values after what it extends, and `[]` when the ruleset declares none | the sentence gone |
| R15 | D12 | Every catalogue entry of a ruleset under `rulesets/` declares a description holding no digit | a digit passes, or no entry is found |
| R16 | D3 | `ruleset.schema.json` describes `forms` and every field a form holds, and `entry.schema.json` every field an entry holds, each with its required fields and the bound on a form's key and an entry's id | a field undescribed, or a bound unstated |

**Refused at the sort**, each with what holds it:
- `get_ruleset` answers the forms and the catalogue: it returns the route
  verbatim (VTT-621), and R12 and R13 hold the route.
- The conformance suite loads every shipped ruleset:
  `TestConformanceOverRulesetsGlob`.
- dnd45e-minimal's abilities are unchanged: its compiled goldens.
- The weapons' entries carry their abilities' numbers: under OD-7 B, held
  by nothing now and by the compiled goldens when the third ticket reads
  them; under OD-7 A, a row.

**Rows a task withdraws or rewords:** VTT-973 by R4 (Task 1); VTT-625,
"each id, property key and branch label", by VTT-1009, which names a form's
key too (an entry's id is an id), withdrawn when its tests gain a probe for
each (Tasks 1 and 2).

## The existing tests and data that change

By search; each task reruns its own packages first and names what turned
red.
- **Task 1:** `internal/rules/qa_declared_sheet_test.go`'s field list;
  `internal/rules/testdata/valid/ruleset.json` (D13);
  `id_bound_internal_test.go` and `qa_property_format_test.go` (VTT-625).
- **Task 2:** `internal/rules/schema_test.go`'s two tables (D13);
  `testdata/valid/catalogue/`.
- **Task 4:** `internal/gateway/qa_ruleset_route_test.go`'s comparisons
  over `qaRRRulesetDirs` and its empty-list checks gain the two fields;
  `internal/mcp/ruleset_tool_test.go`.
- **Task 5:** `rulesets/dnd45e-minimal/ruleset.json` and a new
  `catalogue/`.
- **Unchanged:** every manifest but `testdata/valid`'s and
  dnd45e-minimal's, and every inline manifest literal (P4);
  `rulesets/tavern-brawl/`; every golden; `rules.Resolve`; the client's
  behaviour (D10).

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answers OD-1 to OD-7 (and D11's and D16's
  preferences); run the `requirements` sort over R1 to R16 and the
  refusals above, against the answers; allocate ids with
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id`;
  write the rows; commit the ticket, this plan and `docs/requirements.md`,
  with `tools/check_requirements_chain_qa_test.py`, whose two tests that
  injected `<TAG>-999` as an undefined id went red once the register
  reached VTT-999 and now cite the copy's first free id (their assertion
  unchanged).
  Where an answer differs from the recommendation, the tasks below change
  as each OD's option says before any is dispatched.
- **True when:** `git status` is clean and the requirements chain passes.

### Task 1: a ruleset declares forms (R1 to R4, R16's form half; D1, D2, D13's form; item 1)

- **Files:** `internal/rules/format.go` (`Form`, `Ruleset.Forms`);
  `internal/rules/load.go` (`manifestJSON`, `formJSON`, `loadedManifest`,
  the form checks after the properties in `loadManifest`,
  `checkManifestBounds` for the key, `loadV2`'s copy);
  `internal/rules/schema/ruleset.schema.json` (`forms`, its items'
  `required`, the key's bound in its description);
  `internal/rules/testdata/valid/ruleset.json` (a `number` property only
  the form lists, and the form); `qa_declared_sheet_test.go`'s field list;
  `id_bound_internal_test.go` and `qa_property_format_test.go`, a probe
  each for the form key; a new `internal/rules/forms_test.go`: forms kept
  in order, none, `null`; each refusal of D2 naming the field; a key at 128
  bytes loads and at 129 is refused; a case variant `"Forms"` read as
  `forms`; an undefined field inside a form refused; a test that the
  schema describes every field a form holds, its `required` and
  `additionalProperties: false` (R16, as `TestTheSchemaDescribesEverySheetField`
  does for the sheet's, VTT-829);
  `docs/requirements.md` (VTT-973 → R4, VTT-625).
- **True when:** the new tests pass and fail on `e0bf9ef` (P1);
  `TestSchemaRequiredFieldsMatchLoaderEnforcement` runs, not skips, a case
  for each of the form's required fields; `go test ./internal/rules/...
  ./internal/adventure/... ./internal/gateway/ ./internal/mcp/` passes.
- **Deliberate breaks:** drop each refusal in turn (its R2 or R3 case
  turns red); drop `loadV2`'s copy (R1); copy the forms reversed (R1's
  order case); accept a `resource` in a form (R3); drop the bound from
  `checkManifestBounds` (R2's 129 case); drop the bound from the schema's
  description (the VTT-625 probes); delete `properties` from the form
  item in the schema (R16's nested test); delete `forms` from the schema's
  top-level properties (`TestSchemaPropertiesCoverFixtureFields`, since
  `testdata/valid` declares forms, and the field-list test).
- **The state here:** a ruleset may declare forms; nothing holds an entry.

### Task 2: a ruleset holds a catalogue (R5 to R9, R16's entry half; D3 to D6, D13's entries; item 2)

- **Files:** `internal/rules/format.go` (`Entry`, `Ruleset.Catalogue`);
  `internal/rules/load.go` (`entryJSON`, `loadCatalogue`, D6's namespace
  check, `loadV2` calling both after `checkSheet`); a new
  `internal/rules/schema/entry.schema.json`; `internal/rules/schema_test.go`
  (a row each in `TestSchemaPropertiesCoverFixtureFields` and
  `TestSchemaRequiredFieldsMatchLoaderEnforcement`);
  `id_bound_internal_test.go` and `qa_property_format_test.go`, a probe
  each for the entry id; `internal/rules/testdata/valid/catalogue/`, two
  entries; a new `internal/rules/catalogue_test.go`: entries kept with
  their values; no `catalogue/` loads with an empty catalogue; each
  refusal of D4 naming its file and field, `values.<key>` among them, a
  `null` value, a fraction, and a value one past each end of an `int32`
  (and each end loading); an id at 128 bytes loads, at 129 refused; a
  missing value held as none (R8); an entry id equal to an ability's, to a
  condition's, and an ability's equal to a condition's, each refused naming
  both files (R9); an entry holding an undefined field refused naming the
  file and the field, and a case variant `"ID"` read as `id` (R6);
  `docs/requirements.md` (VTT-625).
- **True when:** the new tests pass and fail on `e0bf9ef` (P2);
  `go test ./internal/rules/... ./internal/adventure/...
  ./internal/gateway/` passes; the walker runs a case for each of the
  entry's required fields.
- **Deliberate breaks:** drop each refusal in turn; decode values straight
  into integers (R7's `null` case loads as 0); widen to `int64` (R7's
  out-of-range case); fill an absent value with 0 (R8); drop the
  namespace check (R9); skip `loadCatalogue` (R5); decode entries with
  `json.Unmarshal` in place of `decodeStrict` (R6's undefined-field case);
  delete `values` from
  `entry.schema.json`'s properties (`TestSchemaPropertiesCoverFixtureFields`).
- **The state here:** entries load, each with only its own values; Task
  2's R5 test uses entries that extend none, and Task 3 adds R5's extending
  case, citing VTT-997 beside VTT-1002.

### Task 3: an entry extends another (R10, R11; D7; item 3)

- **Files:** `internal/rules/load.go` (the resolution after
  `loadCatalogue`); `internal/rules/catalogue_test.go`: a child holds its
  parent's values under its own, a grandchild through two links, a child's
  value over its parent's; each refusal of D7, a cycle of one, of two and of
  three, each naming every entry in it; the cycle tests run `rules.Load` in
  a goroutine with a deadline, so a walk that does not end fails rather
  than hangs.
- **True when:** the new tests pass and fail on `e0bf9ef`;
  `go test ./internal/rules/...` passes.
- **Deliberate breaks:** overlay the parent over the child (R10's
  override case); resolve one link only (R10's chain case); drop each
  refusal (R11's case); drop the visited set (R11's cycle cases fail by
  their deadline).
- **The state here:** the format is whole; nothing serves it.

### Task 4: the read surface (R12 to R14; D8 to D10, D15; item 5)

- **Files:** `internal/gateway/metadata.go` (`formJSON`, `entryJSON`,
  `rulesetJSON`'s two fields, `handleRuleset`); a new
  `internal/gateway/catalogue_route_test.go`: every role reads the same
  forms and catalogue, in order, resolved, from a ruleset built with
  entries in non-sorted id order and one extending another; `[]` with no
  ruleset and with none declared; `internal/gateway/qa_ruleset_route_test.go`'s
  comparisons over `qaRRRulesetDirs` and its empty-list checks gain both
  fields; `internal/mcp/ruleset_tool.go` (`getRulesetDescription`) and
  `internal/mcp/ruleset_tool_test.go`; `client/src/metadata.ts`
  (`FormDef`, `CatalogueEntry`, `RulesetMeta`).
- **True when:** the new tests pass and fail on `e0bf9ef`; `go test
  ./internal/gateway/ ./internal/mcp/ ./cmd/vtt/` passes; `task
  client:typecheck` passes; `task build:client` leaves `cmd/vtt/webdist`
  unchanged, or the rebuilt bundle is committed with the change.
- **Deliberate breaks:** drop `forms` from the answer (R12); drop the
  catalogue's sort (R13's order case); serve an entry's own values (R13's
  extends case); start either list as nil (R12's or R13's `[]` case);
  answer either to the DM alone (the every-role cases); drop the new
  sentence (R14).
- **The state here:** a ruleset's forms and catalogue are read; no shipped
  ruleset has one.

### Task 5: dnd45e-minimal's catalogue (R15; D11, D12; item 4)

- **Files:** `rulesets/dnd45e-minimal/ruleset.json` (the properties and
  forms of D11); `rulesets/dnd45e-minimal/catalogue/` (D11's entries);
  `internal/rules/conformance/conformance_test.go` (D12's test, generic over
  `rulesets/*`).
- **True when:** `TestConformanceOverRulesetsGlob` and the gateway's
  route tests over `qaRRRulesetDirs` pass with the shipped catalogue;
  D12's test passes and fails with a digit put into one shipped
  description; every compiled golden and scenario golden is unchanged
  (`git diff --stat` names no `goldens/`).
- **Deliberate breaks:** a digit in one shipped entry's description (R15);
  move `catalogue/` aside (R15's "none found").
- **The state here:** the alpha campaign's ruleset serves its weapons and
  armour; nothing reads them at the table.

### Task 6: the gate (item 6)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger`
  once, last, if any file's comment share fell (SPEC-010); `go clean
  -cache`; check free space and the load; `task check` whole on the last
  code commit, detached under `/bin/sh`, output to a file, requiring its
  completion line; `task e2e`. Tell the owner whether the client's stored
  mutation report is out of date and which commit it measured (SPEC-023).
- **True when:** `task check` and `task e2e` are green.

### Task 7: the report, then the specifications

- **The report:** `docs/reports/2026-11-24-a-ruleset-declares-forms-and-a-catalogue.md`
  with the `implementation-report` skill: each Done item with its
  observation; R1 to R16 and what turned each red; rule 9 and G1; G1 to G9
  and each OD's answer; under OD-7 B, the interim copy of the weapons'
  numbers and what will show a drift; Phase 4a's adjudications under their
  own heading; the gate.
- **The specifications**, with the `specification` skill, from the report:
  - **SPEC-027 (new):** forms, the catalogue, `extends`, their refusals and
    their order among a ruleset's refusals, the namespace, D15's reading of
    ADR-012 §18; principles 13, 23, 24, 25, 27 and 35.
  - **SPEC-024:** `forms` in the manifest and its field list; a property
    as what a form lists as well as what a sheet holds; that `catalogue/`
    is read.
  - **SPEC-012:** `forms` and `catalogue` in what `/api/ruleset` answers,
    to every role, and `get_ruleset`'s new sentence; its principle-35 line.
  - **SPEC-018:** entry ids and form keys among the ids
    `internal/rules/load.go` bounds and the schemas state; forms' and
    entries' names and descriptions among the ruleset strings left
    unbounded by decision, and among what reaches every participant
    through `/api/ruleset`.
  - **SPEC-025 (under OD-2 A):** the consequence that a property a form
    lists is a key a sheet may hold until the sheet is a form.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, SPEC-027 as drafted, and the surfaces as
signatures and names, never the source: `go doc ./internal/rules` (`Load`,
`Ruleset`, `Form`, `Entry`), the two schemas, and `/api/ruleset`'s shape as
SPEC-012 states it. Tell it: never run `task check`, `task e2e`, Stryker or
gremlins; its files are new `qa_` files; semgrep bans game words in Go
under `internal/` and `cmd/`, tests included; go-arch-lint holds the layer
imports; the tree is hashed before and after it. Known and not findings:
the weapons' numbers stand in the abilities and the entries alike (OD-7);
under OD-2 A a property a form lists is a key a sheet may hold; a
directory the loader does not read is ignored (P2); `extends` is not
served (D8).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "never", "whatever") names the search that bounds it,
P4 and P6 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer; check that no game word reached
Go; check that every refusal a ruleset without a catalogue meets keeps its
message and its order (D3); check that `git diff main -- contract/`
prints nothing.
