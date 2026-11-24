# A ruleset declares forms and a catalogue of entries

## The problem

**A ruleset has no catalogue of things.** `rules.Load`
(`internal/rules/load.go`) reads a ruleset directory's `ruleset.json`
(`manifestJSON`: `id`, `name`, `format_version`, `version`, `properties`,
and optionally `sheet` and `free_usage_name`), its `conditions/`
(`loadConditions`), its `atoms/` (`loadAtoms`) and its `abilities/`
(`loadCompositions`), and `decodeStrict` refuses any other field. Nothing in
the format declares a kind of thing a sheet can hold, or a thing of that
kind: no form, no item, no entry another can extend. `rules.Ruleset`
(`internal/rules/format.go`) carries `Properties`, `Sheet`, `Conditions`,
`Atoms` and `Compiled`, and nothing else a sheet could refer to.

**So equipment lives inside abilities.** `rulesets/dnd45e-minimal/abilities/longsword-strike.json`
binds the longsword's numbers into the ability itself: `"prof": 3` in
`weapon-attack-roll` and `"damage_expr": "1d8 + @caster.str"` in
`weapon-damage`; `crossbow-shot`, `goblin-scimitar` and `goblin-shortbow`
do the same for theirs. Only the ability that swings a weapon holds its
numbers as data, and a second ability that used the same weapon would copy them.
Format v2's §10 held that equipment is not a platform concept, its numbers
lying in an actor's attributes; ADR-012 overturns that: equipment and its
numbers are the ruleset's content, and the mechanism is a sheet that refers
to catalogue entries (ADR-012 §3, "What this overturns"; principle 27 of
`docs/blueprint.md`).

**What a client is told.** `/api/ruleset` (`handleRuleset` in
`internal/gateway/metadata.go`) answers `rulesetJSON`: the ruleset's id,
name, abilities, conditions, resources, properties, sheet and free usage
name. There is no catalogue to answer.

**What MapTool does.** MapTool has no catalogue and no item: a token holds
the properties of the property type the campaign declares for it, each
property possibly marked owner-only or GM-only, and a framework that wants an
inventory writes the items' data into those properties, a copy per token,
which is the opposite of principle 27. The owner's own rule framework for
MapTool (`~/dev/RPTool/rptool-rules.mtlib`'s `library/public/data/`) keeps
abilities, conditions and templates, a template naming a list of the
templates it `extends` (`templates/FireDamage.json` extends
`ImplementDamage`), resolved when an ability is used and with no check for a
cycle. This ticket borrows the idea of an entry that extends another, and
resolves it, cycles refused, when the ruleset loads. It does not borrow the
copy.

**What the owner ruled.** ADR-012's build order puts the catalogue and
references at step 1 item 5. On 2026-11-24 the owner split it into three
tickets, in order: this one, forms and catalogue entries in the ruleset
format, which changes no sheet; then a sheet's references, slots and sets
with the campaign's state per reference, which only the DM and the agent
change; then the actions a sheet reaches through its references, in place of
`Actor.ability_ids`. The first real catalogue is `dnd45e-minimal`'s, the
ruleset the alpha campaign is served under; `tavern-brawl` gets only what
tests need. Equipment and its numbers are the ruleset's content (the owner,
2026-11-02).

**What ADR-012 says of an entry's text.** Text in a catalogue entry is
offered to the DM and never sent to a player by itself (ADR-012 §18,
principles 17 and 32). An entry's description is its explanation, as a
property's is (principle 35), and how it is served is the plan's to propose.

**Not this ticket.** A sheet that refers to an entry, a form's slots and
sets, and what a sheet holds per reference (the next ticket). Actions reached
through references, and the abilities that bind a weapon's numbers today,
which stay as they are (the third ticket, and ADR-012 step 2's procedures).
Derived properties (step 1 item 6). Equipping as an effect (ADR-012 §9, step
2).

## Done looks like

1. **A ruleset declares forms.** A ruleset may declare forms, each with a
   key, a display name, a description and the properties an entry of that
   form carries. `rules.Load` keeps them, and refuses, naming the file and
   the field: a form with no key, name or description, a key two forms
   declare, a key past the bound on an id (SPEC-018), and a form listing a
   property the ruleset does not declare. An `internal/rules` test of each
   fails on today's tree, where the field is unknown.
2. **A ruleset holds a catalogue of entries.** Each entry has an id, a
   display name, a description, a form, and a value for properties its form
   lists. `rules.Load` keeps them, and refuses, naming the file and the
   field: an entry with no id, name or description, or an id past the bound
   on an id (SPEC-018); an entry naming no form or a form the ruleset does
   not declare; an id two entries declare; a value for a property its form
   does not list; a value that is not of its property's type. An
   `internal/rules` test of each fails on today's tree.
3. **An entry may extend another.** An entry that names another as the one
   it extends holds that entry's values except those it sets itself, and the
   values reach through a chain of such entries. `rules.Load` refuses an
   entry extending one the catalogue does not hold, and a cycle of entries
   extending each other, naming the entries.
4. **dnd45e-minimal has a catalogue.** It declares forms for its weapons and
   its armour, and an entry for each weapon its abilities name today (a
   longsword, a crossbow, a scimitar, a shortbow) and for armour, the
   weapon entries carrying each weapon's own numbers the abilities bind:
   its proficiency, its damage die, and a ranged weapon's range. No ability
   binds an armour value today, so the armour entries' numbers are the
   ruleset's new content. Its abilities are unchanged, so each weapon's
   numbers stand in two places until the third ticket makes an ability read
   its weapon. The conformance suite (`internal/rules/conformance`) loads it
   and every shipped ruleset.
5. **The catalogue is served.** `/api/ruleset` and the agent's `get_ruleset`
   answer the forms and the catalogue, each entry with its values after its
   extends are resolved, in the ruleset's words (principle 13), and
   `get_ruleset`'s description says so (principle 34). A gateway test reads
   them, and fails on today's tree.
6. **The gate.** `task check` and `task e2e` are green.

## Rules this puts on the system

- A ruleset's forms each have a key, a display name and a description, and
  list properties the ruleset declares; any other form is refused when the
  ruleset loads.
- A catalogue entry has an id within the bound on an id, a display name, a
  description and a form the ruleset declares, and its ids are distinct; any
  other entry is refused when the ruleset loads.
- An entry carries values only for properties its form lists, each of its
  property's type; any other is refused when the ruleset loads.
- An entry that extends another holds its values except those it sets
  itself; an extends naming no entry, or a cycle, is refused when the
  ruleset loads.
- `/api/ruleset` and `get_ruleset` answer the forms and the catalogue, each
  entry's values resolved through its extends.

## What it touches

In the order the change must reach them:

- The format: `internal/rules/format.go` (`Ruleset`), `internal/rules/load.go`
  (`manifestJSON` or a new directory loader beside `loadConditions`, and the
  checks), and a schema in `internal/rules/schema/` for what is new, with
  `internal/rules/schema_test.go`'s checks of the schemas against the loader.
- The rulesets: `rulesets/dnd45e-minimal/`, and `rulesets/tavern-brawl/` and
  the fixtures under `internal/rules/testdata` as far as tests need.
- The conformance suite: `internal/rules/conformance`.
- The read surface: `internal/gateway/metadata.go` (`rulesetJSON`,
  `handleRuleset`), the agent's `get_ruleset`
  (`internal/mcp/ruleset_tool.go`, its description and
  `ruleset_tool_test.go`), and `client/src/metadata.ts` where `RulesetMeta`
  is typed.
- The tests that walk the format: `internal/rules/qa_declared_sheet_test.go`'s
  field lists, the two tests that check the schemas state the bound on an id
  (`TestTheSchemasStateTheIDBound`, `TestQAPFSchemasStateTheBoundsLoadEnforces`),
  and `internal/rules/testdata/valid`, which the schema walker reads and must
  carry forms and entries for the new required fields to be checked; and the
  register's rows they cite (`docs/requirements.md`: VTT-973, VTT-625).

## Specifications this moves

- docs/specifications/024-a-ruleset-declares-its-properties.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
- New: a ruleset's forms and its catalogue of entries.
- Depending on the plan's decisions:
  docs/specifications/025-a-sheets-values-fit-its-ruleset.md (if a form
  lists properties of the one vocabulary a sheet also draws from)
  and docs/specifications/022-a-client-holds-one-log.md (if the client's
  property types widen).

## What could not be established

- **Which property types an entry may carry.** The format has `number` and
  `resource` (SPEC-024); ADR-012 §2 also lists `category`, `tags`, `dice`,
  `text` and others. A weapon's damage is a dice form ("1d8") and a weapon's
  properties are tags (light, versatile). Which types this ticket adds, if
  any, is the plan's to propose and the owner's to decide.
- **Where a form's properties are declared.** Whether a form lists keys of
  the ruleset's one `properties` vocabulary, or declares properties of its
  own, and whether the sheet's own properties become a form (ADR-012 §3
  calls a sheet a form), is the plan's to propose.
- **One namespace or several.** Whether an entry's id may equal an
  ability's or a condition's id, which are also the ruleset's entries a sheet
  names, is not decided; ADR-012 makes a condition a catalogue entry of a
  form the ruleset makes a condition, which the third ticket and step 2
  reach.
- **Who is served the catalogue.** SPEC-012 answers abilities per role and
  the rest to every participant. Whether every participant is served every
  entry is not ruled; ADR-012 gives visibility per property or per form.
- **Whether an entry may extend one of another form.**
- **Whether an entry must carry a value for every property its form lists**,
  or may leave some unset.
