# SPEC-027: A ruleset's catalogue holds entries of the forms it declares

## Status

Accepted. Implemented by `internal/rules/format.go` (`Form`, `Entry`,
`Ruleset.Forms`, `Ruleset.Catalogue`), `internal/rules/load.go`
(`manifestJSON.Forms`, `formJSON`, `loadManifest`, `loadForms`,
`checkManifestBounds`, `loadV2`, `loadConditions`, `entryJSON`,
`loadCatalogue`, `entryValues`, `resolveExtends`, `cycleErr`,
`checkOneNamespace`) and the schemas
`internal/rules/schema/ruleset.schema.json` and `entry.schema.json`; served
by `handleRuleset` (SPEC-012) and typed for the browser client by
`client/src/metadata.ts` (`FormDef`, `CatalogueEntry`);
`rulesets/dnd45e-minimal` declares forms in its `ruleset.json` and entries
under its `catalogue/`. Pinned by `internal/rules/forms_test.go`,
`internal/rules/qa_forms_test.go`, `internal/rules/catalogue_test.go`,
`internal/rules/qa_catalogue_test.go`, `internal/rules/qa_extends_test.go`,
`internal/rules/schema_test.go`'s `TestSchemaPropertiesCoverFixtureFields`
over the top level of `internal/rules/testdata/valid/ruleset.json` and its
`catalogue/plain-crate.json`, and
`TestSchemaRequiredFieldsMatchLoaderEnforcement` over its form and its
entries,
`internal/rules/conformance/conformance_test.go`'s
`TestConformanceOverRulesetsGlob` and
`TestEveryShippedCatalogueEntryCarriesADescriptionWithNoDigit`, and
`internal/rules/conformance/qa_shipped_catalogue_test.go`.

## Principles served

Principles 13, 23, 24, 25, 27 and 35 of `docs/blueprint.md`:

- **13.** Every form and every entry has a display name and a description,
  so whoever is shown one can be told it in the table's words.
- **23.** Forms and entries are the ruleset's: `internal/rules` knows a
  form only as a key listing `number` properties and an entry only as
  integers under those keys, so `weapon` and `armor` are
  `rulesets/dnd45e-minimal`'s words, and no non-test Go under `internal/`
  or `cmd/` holds either (`git grep -n -i -E 'weapon|armou?r' -- internal
  cmd ':!*_test.go'` prints nothing).
- **24.** An entry holds integers under keys its form lists and nothing a
  ruleset runs, and `extends` is resolved when the ruleset loads, a cycle
  refused, so no chain of entries makes the loader loop.
- **25.** A form or an entry that names what its ruleset does not declare,
  a value out of its bounds, an `extends` that names nothing or closes a
  cycle, and an id two of an entry, an ability and a condition declare, are
  refused when the ruleset loads, with the file and the field named.
- **27.** An entry is declared once, in the ruleset, and one that extends
  another names it, its values resolved at each load, so a correction to an
  entry reaches every entry that extends it. No sheet refers to an entry
  yet (Consequences).
- **35.** Every form and every entry is served as data, with its
  description, to every role (SPEC-012).

## How it works

**A form.** `ruleset.json` may hold `forms`, each a `key`, a `name`, a
`description` and, optionally, `properties`, the keys of the properties an
entry of the form carries. `loadManifest` reads them through `loadForms`
once every property has loaded, and `rules.Load` keeps them as
`Ruleset.Forms`, each a `rules.Form` as written, in declared order, and nil
when `forms` is absent, `null` or `[]` (VTT-993); a form whose `properties`
is absent or `null` lists none. Naming `ruleset.json` and the field as
`forms[<i>].<field>`, `loadForms` refuses, form by form: a `key`, `name` or
`description` that is empty, absent and `null` read as empty, in that
order; a key an earlier form declares (VTT-994); then, key by key, as
`forms[<i>].properties[<j>]`, a key no property declares, one the form
lists already, and one whose property is not a `number` (VTT-995). Before
any property is read, and after the property keys' bound,
`checkManifestBounds` refuses a form key longer than `maxIDBytes` (VTT-994,
SPEC-018). `loadForms` and `checkManifestBounds` check nothing else of a
key: its characters are free, and it may equal a property's key or an
entry's id. The strict decode refuses a field a form does not define
(VTT-996, SPEC-024).

**An entry.** Each `*.json` directly under a ruleset's `catalogue/` is one
entry. `loadCatalogue` reads them through `jsonFilesIn`, in byte order of
their paths, so a subdirectory or a file of another extension is no entry,
and decodes each with `decodeStrict`: `id`, `name`, `description` and
`form`, its form's key, and, optionally, `extends`, another entry's id, and
`values`, an object from a property key to an integer. `rules.Load` keeps
them as `Ruleset.Catalogue`, a map from id to `*rules.Entry`, empty and
never nil for a ruleset with no `catalogue/` (VTT-997). Naming the entry's
file and the field, file by file, it refuses: a field the entry format does
not define, a case variant of a defined one being read as it, as for a
manifest (SPEC-024); an `id`, `name`, `description` or `form` that is
empty, absent and `null` read as empty, in that order; an `id` longer than
`maxIDBytes`; an `id` an earlier file declares, as `duplicate entry id
"<id>", declared by <path>`; a `form` no form declares (VTT-998); then,
through `entryValues`, the values in byte order of their keys, under
`values.<key>`: a key the entry's form does not list, a value written
`null`, and a value that is not an integer within `int32`, the type of
`Actor.attributes`' values, `1.0`, `1e2` and a quoted number among them
(VTT-999). `"values": null` holds none. `loadCatalogue` checks nothing else
of an id, and `checkOneNamespace` only that no ability or condition holds it
(below).

**What an entry holds.** An entry need not hold a value for every property
its form lists, and holds no key for one that neither it nor an entry along
its `extends` sets (VTT-1000); `Entry.Values` is empty, never nil, for an
entry that holds none. An entry that names another in `extends` holds every
value that entry holds, after its own `extends`, that it does not set
itself, so along a chain the nearest value is the one held (VTT-1002);
`"extends": null` is no `extends`. `resolveExtends` reads each chain from
the values its entries set themselves, so what an entry holds does not
depend on the order of the files or their names, and `Entry` keeps no
`extends`. Once every file has loaded, `resolveExtends` refuses, in byte
order of the ids of the entries that extend, on the extending entry's file
and the field `extends`: one written empty, one naming an id the catalogue
does not hold, and one naming an entry of another form. Then it walks each
chain, in the same order, with a visited set, and `cycleErr` refuses the
first cycle a walk meets, on the file of the lowest id in it, as `is in a
cycle of entries extending each other: ` and every id in the cycle in byte
order, naming no entry that reaches the cycle from outside it (VTT-1003).

**One id namespace.** Once the catalogue has loaded, `checkOneNamespace`
refuses an id two of an entry, an ability and a condition declare: the
first such id in byte order, on the field `id` of the second file that
declares it in byte order of their paths, naming the first, as `"<id>" is
declared by <the first file's path> too; an entry, an ability and a
condition never share an id`
(VTT-1001). It runs whether or not the ruleset has a catalogue, so an
ability and a condition share no id either. Form keys and property keys are
not in it.

**Case.** Ids, form keys and property keys are compared byte for byte, each as
a map's key: an entry's `form`, its `extends` and its value keys name a form,
an entry and a listed property only in their exact case, `a` and `A` are two
entries, and `crate` and `Crate` two forms.

**The order of the refusals.** `rules.Load` stops at the first. The forms'
come within the manifest's, before any condition, atom or ability is read:
a form key's bound in `checkManifestBounds`, the rest in `loadForms` after
the properties' (SPEC-024). The catalogue is read last in `loadV2`, after
`checkSheet`, so a ruleset without one meets every other refusal in its
order; within the catalogue, each file's refusals come in the order of the
files, then the links' of `extends`, then a cycle's, then the namespace's.

**An entry's words.** An entry's `description` is its explanation in the
ruleset's words, as a property's and a condition's are, and is served with
it to every role (SPEC-012). It is not the text ADR-012 §18 says a
catalogue entry offers the DM and never sends a player by itself: the
format gives an entry no field for that text.

**The schemas.** `ruleset.schema.json` describes `forms` and every field of
a form, and `entry.schema.json` every field of an entry, each with its
required fields, and the descriptions of a form's key and an entry's id
state their bound (VTT-1008, SPEC-018). Each types an optional field by
what it holds, so neither admits the `null` the loader reads as absent for
`forms`, a form's `properties`, `extends` and `values`.

**The shipped rulesets.** `rulesets/dnd45e-minimal` declares the forms
`weapon` and `armor`, each over `number` properties it declares for them,
and a catalogue of its weapons and armor, no entry extending another (`git
grep -n extends -- rulesets/` prints nothing); `rulesets/tavern-brawl`
declares no form and has no `catalogue/`. `TestConformanceOverRulesetsGlob`
loads every ruleset under `rulesets/`. A shipped entry's description holds
no digit, in the sense SPEC-024 states for an ability's (VTT-1007):
`checkShippedDescriptions`, which
`TestEveryShippedCatalogueEntryCarriesADescriptionWithNoDigit` runs over
the files under `rulesets/*/catalogue/`, refuses the ASCII digits, and
`TestQASCShippedEntryDescriptionsHoldNoDigit`, in the loaded descriptions,
any rune `unicode.IsDigit` reports. Each fails when it finds nothing to
read.

## Consequences

- No sheet refers to an entry and no ability reads one: `vtt.v1.Actor` has
  no field for an entry, and outside `internal/rules/load.go` only
  `handleRuleset` reads `Ruleset.Forms` and `Ruleset.Catalogue` (`git grep
  -n -E '\.(Catalogue|Forms)([^A-Za-z]|$)' -- '*.go' ':!*_test.go'`). So
  an entry's values decide no outcome: `rulesets/dnd45e-minimal`'s weapons'
  numbers stand both in their entries and in the bindings of the abilities
  that use them, a use rolls the binding, and no test holds the two equal:
  none names `proficiency`, `damage_dice`, `damage_die` or `armor_bonus`
  (`git grep -l -E 'damage_dice|damage_die|proficiency|armor_bonus' --
  '*_test.go' '*.test.ts'` prints nothing), and `qascValueFaults`, in
  `qa_shipped_catalogue_test.go`, compares a shipped entry's loaded values,
  `range` among them, only with the entry's own file, and
  `qaRRCheckCatalogue`, in `internal/gateway/qa_ruleset_route_test.go`, the
  values `/api/ruleset` serves only with the loaded ones.
- A property a form lists is a key a sheet may hold (SPEC-025).
- The format gives a form no visibility, so every participant is served
  every form and every entry (SPEC-012); ADR-012 gives a visibility per
  property or per form.
- An entry carries `number` values alone, so what is not one integer is
  written as several, a damage of dice among them: dnd45e-minimal's
  `damage_dice` and `damage_die`. A value of another type needs a change to
  `loadForms`, `entryValues`, `rules.Entry`, both schemas, the route's
  `entryJSON`, `getRulesetDescription` and `client/src/metadata.ts`'s
  `CatalogueEntry`.
- An ability or a condition a ruleset adds takes an id no entry holds, and
  an entry an id no ability or condition holds.
- `extends` is resolved when the ruleset loads and not kept, so no reader
  can tell a value an entry sets from one it inherits, and a correction to
  an entry reaches the entries that extend it at the ruleset's next load.
- A directory the loader does not read, a misspelled `catalogue/` among
  them, loads as no catalogue, and nothing refuses it.
- The loader is the authority, as each schema's description says: a
  document a schema refuses may load, a `null` above or a case variant of a
  field's name, and one it admits may be refused, a value written `1.0` or
  `1e2`, which a JSON Schema `integer` admits, among them.
- The browser client types both lists, and nothing in `client/src` reads
  them (`git grep -n -w -E 'forms|catalogue' -- client/src` prints only
  `RulesetMeta`'s `forms` and `catalogue` and a comment of
  `client/src/view/dm.ts` on the console's forms).

## Requirements

VTT-993, VTT-994, VTT-995, VTT-997, VTT-998, VTT-999, VTT-1000, VTT-1001,
VTT-1002, VTT-1003, VTT-1007, VTT-1008.
