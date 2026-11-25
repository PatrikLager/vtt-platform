# SPEC-024: A ruleset declares its properties, each by key, type and the table's words

## Status

Accepted. Implemented by `internal/rules/format.go` (`Property`,
`PropertyNumber`, `PropertyResource`, `ResourceDef`, `Ruleset.Version`,
`Ruleset.Properties`,
`Ruleset.Property`, `Ruleset.Sheet`, `SheetPage`, `SheetSection`,
`ShownUsage`, `Ruleset.FreeUsageName`, `CompiledPower.Description`),
`internal/rules/load.go` (`formatVersion`,
`formatVersionWant`, `checkFormatVersion`, `loadManifest`, `decodeStrict`,
`checkPropertyKey`, `loadProperty`, `loadResourceDef`, `maxNameBytes`,
`checkManifestBounds`, `propertyTypes`, `checkPropertyRef`,
`checkExprRefs`, `crossValidateResources`, `loadAtoms`,
`validParamKinds`, `checkSheet`, `ShownUsage.UnmarshalJSON`,
`loadCompositions`),
`internal/rules/compile.go`
(`compilePower`, `bindAtom`, `compileEffects`, `compileTwoActorExpr`)
and the schemas `internal/rules/schema/ruleset.schema.json`,
`atom.schema.json` and `ability.schema.json`. Pinned by
`internal/rules/property_test.go`,
`internal/rules/qa_property_format_test.go`, `internal/rules/load_test.go`,
`internal/rules/schema_test.go`, `internal/rules/declared_sheet_test.go`,
`internal/rules/qa_declared_sheet_test.go`,
`internal/rules/ruleset_version_test.go`,
`internal/rules/qa_ruleset_version_test.go`,
`internal/rules/ability_description_test.go`,
`internal/rules/qa_ability_description_test.go`,
`internal/rules/forms_test.go` and `internal/rules/qa_forms_test.go` for a
form's fields,
`internal/rules/conformance/conformance_test.go`,
`internal/rules/conformance/qa_shipped_descriptions_test.go` and
`internal/rules/conformance/qa_shipped_catalogue_test.go`'s
`TestQASCShippedAbilityDescriptionsHoldNoDigit`.
Every ruleset under
`rulesets/` is written in this format. How a character's sheet is read
against the properties is SPEC-025, and a ruleset's forms and its
catalogue of entries are SPEC-027's.

## Principles served

Principles 13, 23, 24, 25, 27, 28 and 35 of `docs/blueprint.md`:

- **13.** Every property has a display name and an explanation, so whoever is
  shown it can be told it in the table's words.
- **23.** A property is declared by its ruleset, never by the engine, which
  knows a property only as a `number` or a `resource` and nothing of any
  game system's; how a character's sheet is laid out is the ruleset's too,
  and so is the word for an ability that spends no resource.
- **24.** A property is a `number` or a `resource`, and a ruleset declares no
  other kind.
- **25.** A key a ruleset reads that no property declares, or reads as the
  other type, is refused when the ruleset loads, with the file, the field
  and the key named, and so is a declared `sheet` that does not fit the
  ruleset, and a manifest whose id, version or name the log could not carry.
- **27.** A declared `sheet`'s sections name property keys and usages, never
  an ability, so an ability the ruleset adds stands in its usage's section.
- **28.** Every ruleset declares its own version, which the log names before
  the outcomes the ruleset decides (SPEC-026).
- **35.** Each property carries its own explanation, which is served to
  every participant, and an ability may carry the ruleset's description of
  it, which is served with the ability (SPEC-012).

## How it works

**The manifest.** A ruleset's `ruleset.json` holds `id`, `name`,
`format_version`, `version`, `properties` and, optionally, `sheet`,
`free_usage_name` and `forms`; the forms, and the entries `rules.Load`
reads from the ruleset's `catalogue/`, are SPEC-027's. `loadManifest`
decodes it with `decodeStrict`, which refuses a field the format does not
know, in the manifest, in a property, in the sheet or in a form, as
`rules: <path>: json: unknown field "<field>"`, in a usage after `invalid
shown usage object: `; `encoding/json` matches a field's name
without regard to case, so a case variant of a defined field is read as it
(VTT-996).
Every other refusal of a ruleset below has the form `rules: <path>: field
"<field>": <message>`, which `fieldErr` writes. `<path>` is the file's path
as it was loaded, except that `crossValidateResources`' refusals name
`ruleset.json` alone.

**The format version, read first.** Before the strict decode,
`loadManifest` reads
`format_version` alone, when the whole file parses as one JSON object or as
`null`, through `checkFormatVersion`; a file that is `null` reads as one with
no `format_version`, and any other file is left to `decodeStrict`.
`checkFormatVersion` refuses anything but the string `"3"` on that field, a
`format_version` written as `null` among it: an absent field
with `is absent: ` and `formatVersionWant`, any other value with
`unsupported value `, the value as written, `: ` and `formatVersionWant`.
`formatVersionWant` says that this loader reads format `"3"`, which declares
every property under `properties` with its key, type, name and description,
and names `docs/adr/012-the-rules-contract.md` (VTT-616).

**The ruleset's id, name and version.** `id` and `name` are the ruleset's
id and display name, and `version` is the ruleset's own version, not the
format's: free text its author declares and keeps true. After the strict
decode `loadManifest` refuses each of the three absent, `null` or empty, in
that order, as `field "<field>": must not be empty`, and `rules.Load` keeps
them as written as `Ruleset.ID`, `Ruleset.Name` and `Ruleset.Version`
(VTT-970, VTT-971). `checkManifestBounds` then refuses an `id` or a
`version` longer than `maxIDBytes` and a `name` longer than `maxNameBytes`,
in that order, each as `field "<field>": must be at most <bound> bytes, got
<length>`, before it reads a property key (VTT-972); the bounds are the
ones the log's entry naming the ruleset has (SPEC-018, SPEC-026).
`ruleset.schema.json` lists `version` as required, a string of at least one
character whose description states its bound (VTT-986), and the
descriptions of `id` and `name` state theirs (VTT-1009 for the id).
`rulesets/dnd45e-minimal` and `rulesets/tavern-brawl` each declare
`"version": "1"`. What the log makes of the version is SPEC-026's.

**A property.** Each entry of `properties` holds `key`, `type`, `name` and
`description`. `rules.Ruleset.Properties` keeps them in the order the file
declares them, each a `rules.Property`, and `Ruleset.Property(key)` returns
the one under a key, or nil.
- **`key`** is what an expression reads. `checkPropertyKey` refuses one that
  is empty, that does not match `^[A-Za-z_][A-Za-z0-9_]*$`, that is
  `caster` or `target`, or that an earlier property declares, whatever
  either's type (VTT-612, VTT-626). `checkManifestBounds` bounds it at
  `maxIDBytes` (SPEC-018).
- **`type`** is `number` (`PropertyNumber`) or `resource`
  (`PropertyResource`); `loadProperty` refuses a type that is absent or
  any other string, naming it (VTT-611).
- **`name`** is the display name and **`description`** what the property
  means at the table; `loadProperty` refuses either absent or empty (VTT-610).
  Neither has a length bound (SPEC-018).

**What a resource carries.** A `resource` property may hold
`default_max_expr` and `thresholds`, which `loadResourceDef` parses into the
property's `Resource`, a `*rules.ResourceDef`; `Resource` is set exactly when
the type is `resource`. A threshold requires `when`, `apply_condition` and
`remove_when_false` (VTT-628), and its `apply_condition` names a declared
condition (VTT-629). Neither `default_max_expr` nor a threshold's `when`
holds dice, and neither holds a scoped reference (VTT-630). A
`number` property that holds either field is refused, naming the field
(VTT-615); a field present as `null` reads as absent.

**Where a value lives.** A number property's value is an entry of an actor's
`attributes`, a resource property's current value and maximum an entry of its
`resources`, each under the property's key. A `number` property a form lists
is also what a catalogue entry of that form holds under its key in `values`
(SPEC-027); it is declared in `properties` as any other, so an expression
may read it and a sheet may hold it (SPEC-025). An expression reads a number
property as `@key` and a resource property's current value as `#key`: bare in
a single-actor position, a resource's `default_max_expr` and a threshold's
`when`, and scoped as `@caster.key`, `@target.key`, `#caster.key` or
`#target.key` in a two-actor one, a composed atom's `roll`, `vs` and
`delta_expr` (VTT-630). Which keys a sheet may hold, and where it is read
against the ruleset, is SPEC-025.

**Where a key is checked.** `propertyTypes` maps each declared key to its
type, and `checkPropertyRef` refuses a key no property declares (`references
undeclared property "<key>"`, VTT-613) or one declared as the other type
(`reads <type> property "<key>" as a <type>`, VTT-614), naming the file and
the field. It is called at these places, and `git grep -nE
'checkPropertyRef\(|checkExprRefs\(' -- internal/rules ':!*_test.go'` finds
no other:
`crossValidateResources` through `checkExprRefs`, for a resource's
`default_max_expr` and a threshold's `when`; `compilePower`, for an ability's
cost, `usage.limited.resource`, which names a resource; `bindAtom`, for a
binding to an atom parameter of kind `number` or `resource`; and, in an atom
an ability composes, after that ability's bindings are spliced in,
`compileTwoActorExpr` for the `roll`, the `vs` and an effect's `delta_expr`,
and `compileEffects` for an effect's `resource_change.resource`. A fault in a
composed atom is named against the ability's file, with the path through its
composition. An atom no ability composes is never compiled, and an `expr`
binding no template splices in is parsed and nothing more, so a key named
only there is not checked.

**An atom's parameters.** An atom parameter's kind is `int`, `expr`,
`number`, `resource` or `condition` (`validParamKinds`, which `loadAtoms`
checks), and `bindAtom` reads each binding by it. `int` takes a
non-negative integer; `expr` takes an expression that parses, whose keys are
checked where it is spliced in; `number` and `resource` name a property of
that type; `condition` names a declared condition (VTT-629).

**The sheet.** `sheet` is how a character's sheet is laid out: a list of
pages, each with a `name` and its `sections`, each section with a `name`
and exactly one of `values`, a list of property keys, `conditions`, which
is `true`, and `abilities`, a list of usages, each the string `"at_will"`
or an object `{"spends": "<key>"}`. `rules.Load` returns it as
`Ruleset.Sheet`, a `[]SheetPage` whose sections are `SheetSection` and
their usages `ShownUsage`, in the order declared, and nil for a manifest
without `sheet` or with `"sheet": null`
(VTT-824). In a usage, `ShownUsage.UnmarshalJSON` refuses any other
string, naming the string, a field other than `spends`, naming the field,
and an object without `spends`; none of these refusals names where in the
sheet the usage stands. A section's kind present as `null` reads as absent,
and a kind of another JSON type is refused by the decode. `checkSheet` runs
after the abilities compile, so every refusal above keeps its message and
its order, and refuses, naming the field by its path from `sheet`, as
`sheet[<i>].sections[<j>].values[<k>]`:
- a sheet with no page, a page with an empty name, the name of a page
  before it or no section, a section with an empty name, showing none or
  more than one of `values`, `conditions` and `abilities`, with
  `conditions` `false`, or with an empty list (VTT-825);
- a key under `values` no property declares, and a `spends` of a key no
  resource property declares, an empty key among them (VTT-826);
- a key named more than once among the sections' `values`, a usage more
  than once among their `abilities`, and the conditions in more than one
  section (VTT-827);
- on the field `sheet`, naming the ability, an ability of the ruleset whose
  usage no section names, the first in id order (VTT-828).

A key may stand both as a value and in the usage that spends it, a property
need not stand at all, and a `spends` of a resource no ability spends loads.
`ruleset.schema.json` describes `sheet` and every field it holds (VTT-829).
`sheet` is optional, and `format_version` is `"3"` either way.
`rulesets/dnd45e-minimal` declares a sheet; `rulesets/tavern-brawl` declares
none. `GET /api/ruleset` and `get_ruleset` serve it (SPEC-012), and
the browser client's panel draws it (SPEC-022).

**An ability's words.** An `abilities/*.json` may hold `description`, the
ruleset's own words for what the ability does. `loadCompositions` refuses
one present and empty, as `field "description": must not be empty`
(VTT-848), and `compilePower` keeps it as `CompiledPower.Description`, none
for an ability that declares none or declares it `null` (VTT-847).
`rules.Load` checks nothing of what it says beyond its being non-empty,
`rules.Resolve` does not read it, and `compiledPowerDump` has no field for
it, so the compiled goldens do not carry it. `ability.schema.json` describes
it as an optional string of at least one character (VTT-849). A
`ruleset.json` may hold `free_usage_name`, what the table calls an ability
that spends no resource: `loadManifest` refuses one present and empty, as
`field "free_usage_name": must not be empty` (VTT-857), and `rules.Load`
keeps it as `Ruleset.FreeUsageName`, none when it is absent or `null`
(VTT-856); `ruleset.schema.json` describes it as an optional string of at
least one character (VTT-858). Every ability under `rulesets/` declares a
description with no digit, so its words stay true whatever figures its
composition binds (VTT-850). A digit is a rune `unicode.IsDigit` reports, a
decimal digit of any script: `checkShippedDescriptions`, which
`TestEveryShippedAbilityCarriesADescriptionWithNoDigit` runs over the files
under `rulesets/*/abilities/`, refuses the ASCII digits, and
`TestQASCShippedAbilityDescriptionsHoldNoDigit`, in the loaded
descriptions, and `TestQAShippedAbilityDescriptionsNonEmptyNoDigit`, in
each ability file's, refuse any; a number spelled out, and a numeral that
is no decimal digit, such as `²`, passes every one of them, and each fails
when it finds nothing to read. `rulesets/dnd45e-minimal` names an ability
that spends no resource "At will"; `rulesets/tavern-brawl` names none. `GET
/api/ruleset` and `get_ruleset` serve both (SPEC-012), and the browser
client's panel titles an ability's button by them (SPEC-022).

**Where a property's meaning is read.** A property's `description` says
what it means at the table. `GET /api/ruleset` and the MCP tool
`get_ruleset` serve every property's key, type, name and description
(SPEC-012), and each guide under `rulesets/` points to `get_ruleset` for
them.

## Consequences

- A `ruleset.json` that parses as one JSON object in another format is
  refused on its `format_version` before any other field is read, so its
  author is told which format this loader reads rather than which field is
  unknown.
- A `ruleset.json` without a `version` is refused, so every ruleset an
  operator serves names one; a correction its author makes without
  changing it is served as the same version (SPEC-026).
- A new type of property needs a change to `loadProperty` and the schema,
  and a new field on a property, a visibility among them, a change to the
  loader and the schema, since the strict decode refuses a field it does not
  know. ADR-012 gives a property a visibility; the format has no such field,
  so every participant is served every property.
- A new kind of section needs a change to the loader, the schema, the
  route's answer (SPEC-012) and the client, where `sheetFor` fails the
  typecheck until it handles the kind `SheetSection` names (SPEC-022).
- A ruleset that declares a sheet shows every ability's usage in a section,
  so one that adds an ability of a new usage names that usage in a section.
- A ruleset declares, as a property of the type it is read as, every key
  it reads in a resource's `default_max_expr` or a threshold's `when`, a
  cost, a `number` or `resource` binding, or an atom an ability composes.
- `rules.Resolve`'s refusals of an ability name a key, and say attribute
  and defense; they are not in the table's words.
- An ability's description is not read against its composition, so a
  stat, a defense or a condition it names is a word a change to the
  composition must carry along, and nothing goes red if it does not.

## Requirements

VTT-610, VTT-611, VTT-612, VTT-613, VTT-614, VTT-615, VTT-616, VTT-626,
VTT-628, VTT-629, VTT-630, VTT-824, VTT-825, VTT-826, VTT-827, VTT-828,
VTT-829, VTT-847, VTT-848, VTT-849, VTT-850, VTT-856, VTT-857, VTT-858,
VTT-970, VTT-971, VTT-972, VTT-986, VTT-996, VTT-1009.
