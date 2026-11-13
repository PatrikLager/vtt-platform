# SPEC-024: A ruleset declares its properties, each by key, type and the table's words

## Status

Accepted. Implemented by `internal/rules/format.go` (`Property`,
`PropertyNumber`, `PropertyResource`, `ResourceDef`, `Ruleset.Properties`,
`Ruleset.Property`), `internal/rules/load.go` (`formatVersion`,
`formatVersionWant`, `checkFormatVersion`, `loadManifest`, `decodeStrict`,
`checkPropertyKey`, `loadProperty`, `loadResourceDef`,
`checkManifestBounds`, `propertyTypes`, `checkPropertyRef`,
`checkExprRefs`, `crossValidateResources`, `loadAtoms`,
`validParamKinds`), `internal/rules/compile.go`
(`compilePower`, `bindAtom`, `compileEffects`, `compileTwoActorExpr`)
and the schemas `internal/rules/schema/ruleset.schema.json` and
`atom.schema.json`. Pinned by `internal/rules/property_test.go`,
`internal/rules/qa_property_format_test.go`, `internal/rules/load_test.go`
and `internal/rules/schema_test.go`. Every ruleset under `rulesets/` is
written in this format. How a sheet is read against the properties is
SPEC-025.

## Principles served

Principles 13, 23, 24, 25 and 35 of `docs/blueprint.md`:

- **13.** Every property has a display name and an explanation, so whoever is
  shown it can be told it in the table's words.
- **23.** A property is declared by its ruleset, never by the engine, which
  knows a property only as a `number` or a `resource` and nothing of any
  game system's.
- **24.** A property is a `number` or a `resource`, and a ruleset declares no
  other kind.
- **25.** A key a ruleset reads that no property declares, or reads as the
  other type, is refused when the ruleset loads, with the file, the field
  and the key named.
- **35.** Each property carries its own explanation, which is served to
  every participant (SPEC-012).

## How it works

**The manifest.** A ruleset's `ruleset.json` holds `id`, `name`,
`format_version` and `properties`. `loadManifest` decodes it with
`decodeStrict`, which refuses a field the format does not know, in the
manifest or in a property, as `rules: <path>: json: unknown field "<field>"`;
`encoding/json` matches a field's name without regard to case (VTT-627).
Every other refusal of a ruleset below has the form `rules: <path>: field
"<field>": <message>`, which `fieldErr` writes. `<path>` is the file's path
as it was loaded, except that `crossValidateResources`' refusals name
`ruleset.json` alone.

**The version, read first.** Before the strict decode, `loadManifest` reads
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
`resources`, each under the property's key. An expression reads a number
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
- A new type of property needs a change to `loadProperty` and the schema,
  and a new field on a property, a visibility among them, a change to the
  loader and the schema, since the strict decode refuses a field it does not
  know. ADR-012 gives a property a visibility; the format has no such field,
  so every participant is served every property.
- A ruleset declares, as a property of the type it is read as, every key
  it reads in a resource's `default_max_expr` or a threshold's `when`, a
  cost, a `number` or `resource` binding, or an atom an ability composes.
- `rules.Resolve`'s refusals of an ability name a key, and say attribute
  and defense; they are not in the table's words.

## Requirements

VTT-610, VTT-611, VTT-612, VTT-613, VTT-614, VTT-615, VTT-616, VTT-626,
VTT-627, VTT-628, VTT-629, VTT-630.
