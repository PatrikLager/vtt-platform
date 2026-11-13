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
(`compilePower`, `bindAtom`, `compileEffects`, `compileTwoActorExpr`),
`internal/rules/sheet.go` (`SheetFault`, `Ruleset.SheetKeyFault`), the
schemas `internal/rules/schema/ruleset.schema.json` and `atom.schema.json`;
for a sheet, `internal/adventure/load.go` (`actorJSON`, `resourceValJSON`,
`loadActors`, `actorValues`), `internal/gateway/add_actor_validate.go`
(`validateAddActorWith`), `cmd/vtt/serve_compose.go` (`sheetsFit`),
`internal/mcp/resource_current.go` (`refuseValuelessCurrent`) and
`client/src/commands.ts` (`valuelessCurrent`). Pinned by
`internal/rules/property_test.go`, `internal/rules/qa_property_format_test.go`,
`internal/rules/load_test.go`, `internal/rules/schema_test.go`,
`internal/rules/sheet_test.go`, `internal/rules/qa_sheet_key_test.go`,
`internal/adventure/load_test.go`,
`internal/adventure/statblock_type_test.go`,
`internal/adventure/qa_statblock_property_test.go`,
`internal/adventure/actor_value_test.go`,
`internal/adventure/qa_actor_value_test.go`,
`internal/gateway/add_actor_fit_internal_test.go`,
`internal/gateway/qa_add_actor_keys_test.go`,
`internal/mcp/resource_current_internal_test.go`,
`internal/mcp/qa_add_actor_current_test.go`, `client/test/commands.test.ts`,
`client/test/qa-add-actor-paste.test.ts`, `cmd/vtt/campaign_fit_test.go`,
`cmd/vtt/qa_campaign_fit_test.go`, `tools/toolgen/main_test.go` and
`tools/toolgen/qa_add_actor_schema_test.go`. Every ruleset under
`rulesets/` is written in this format.

## Principles served

Principles 13, 23, 24, 25, 26 and 35 of `docs/blueprint.md`:

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
- **26.** An actor that gives a key the ruleset does not declare, or under
  the other type's map, is refused when an adventure holding it is loaded and
  when `add_actor` adds it with a ruleset served, and a campaign holding one
  is not served with that ruleset; each refusal names the key, and the actor
  or its file.
- **35.** Each property carries its own explanation, which is served to
  every participant (SPEC-012).

## How it works

**The manifest.** A ruleset's `ruleset.json` holds `id`, `name`,
`format_version` and `properties`. `loadManifest` decodes it with
`decodeStrict`, which refuses a field the format does not know, in the
manifest or in a property, as `rules: <path>: json: unknown field "<field>"`;
`encoding/json` matches a field's name without regard to case (VTT-627).
Every other refusal of a ruleset below has the form `rules: <path>: field
"<field>": <message>`, which `fieldErr` writes; an adventure's has the same
form after `adventure: `. `<path>` is the file's path as it was loaded,
except that `crossValidateResources`' refusals name `ruleset.json` alone.

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
`delta_expr` (VTT-630).

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

**A sheet's keys.** `Ruleset.SheetKeyFault` reads a sheet's `attributes`
keys and then its `resources` keys, each in byte order, from a sorted copy
that leaves its caller's keys in their order, and answers the first that
does not fit as a `SheetFault`: the map that holds it, the key, and the
reason, `references undeclared property "<key>"` for a key no property
declares (VTT-617) or `gives <type> property "<key>" a <type>'s value` for a
key under the other type's map (VTT-618). Its callers are these, and
`git grep -n 'SheetKeyFault(' -- '*.go' ':!*_test.go'` finds no other
beside its definition:
- `adventure.Load`, through `loadActors`, for each actor file, as
  `adventure: <path>: field "<map>": <reason>`.
- `handleCommand`, for an `add_actor`, through `validateAddActorWith`, after
  `validateAddActor`'s refusals (SPEC-013) and only for an actor with an id:
  with a ruleset served, `gateway: add_actor: actor "<id>": <map>:
  <reason>` (VTT-677); with none, no key is read against one (VTT-675).
- `composeServer`, through `sheetsFit`, when it is handed a ruleset, as
  `vtt serve --ruleset` hands it one: right after the ruleset loads and
  before any adventure, it reads every actor `campaign.Campaign.Actors`
  holds and, when any does not fit,
  closes the identity store and the campaign and refuses the boot with
  `vtt serve: ruleset <dir>: the campaign's actors do not fit it: ` and,
  joined by `; `, `actor "<id>": <map>: <reason>` for every actor that does
  not fit, in byte order of their ids (VTT-678, VTT-681). With no ruleset,
  no key is read (VTT-676).

**A sheet's values in an adventure.** `actorJSON` and `resourceValJSON`
decode an actor file's values through pointers, so `actorValues` sees what
was written. After the key check, it reads `attributes` and then
`resources`, each in byte order, and refuses a number written `null`
(field `attributes.<key>`, `must be a number, 0 included, not null`), a
resource written `null` (`resources.<key>`, `must be an object giving its
current, not null`), and a resource whose `current` is written `null` or
left out (`resources.<key>.current`, `must be a number, 0 included; a
resource gives its current`), naming the file (VTT-670). A value of `0`
loads, a `max` left out or written `null` is no maximum (VTT-679), and a
`current` over a `max` above `0` is refused. `"attributes": null` or
`"resources": null` loads as a sheet holding nothing in that map.

**Where a `current` with no value is refused.** Where it is written: in an
adventure (above), in the MCP tool `add_actor` and in the DM console's paste
of an actor. The gateway does not read it (`git grep -nE
'GetCurrent\(\)|\.Current([^A-Za-z0-9_]|$)' -- internal/gateway
':!*_test.go'` finds nothing), since the browser and the MCP server both
leave a `current` of `0` out of the command they send, so in a command a
`current` left out is a `0`.
- `commandFromArgs` runs `refuseValuelessCurrent` over a tool's arguments
  once `protojson` has accepted them. It walks them by the command's
  descriptor, never into a `google.protobuf` message, and refuses the first
  `vtt.v1.Resource`, map entries in byte order of their keys, whose
  `current` is `null` or left out, with `mcp: invalid arguments for
  add_actor: actor "<id>": resources.<key>.current must be a number, 0
  included; a resource gives its current`, before any command is sent
  (VTT-671). It runs before the gateway's refusals, and names an actor with
  no id `actor ""`. A resource written `null` is refused by `protojson`, in
  its own words.
- `parseActorJSON` runs `valuelessCurrent` over the paste as written, after
  its refusals of a missing id, a controller and a missing kind, and refuses
  the first resource, in UTF-16 code-unit order of its keys, whose `current`
  is `null` or left out, naming the actor and the key, before any command is
  built (VTT-672).
- A `current` of `0` passes both, and a `max` left out or written `null` is
  no maximum (VTT-679). `add_actor`'s schema requires a resource's `current`
  and not its `max` (VTT-674, SPEC-007), and its description says what it
  refuses in a sheet (VTT-680).

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
- An actor gives only the keys its ruleset declares, each in its type's
  map, whether it comes from an adventure or through `add_actor`, and a
  campaign served with a ruleset holds no other. `git grep -n
  'Envelope_ActorAdded{' -- '*.go' ':!*_test.go' ':!contract/gen'` finds
  every writer of an `ActorAdded`; of them, `adventure.Compile`, for an
  adventure loaded against the ruleset, and `ToEvent`, for an `add_actor`
  the gateway has read, write to a campaign, while `internal/eventgen`
  draws histories for property tests and `perceive` writes a character
  log. A campaign served with no ruleset is read against none, so its
  actors may hold any keys, and it is refused when it is next served with a
  ruleset they do not fit.
- No command changes which keys a sheet holds: in `engine.Apply` an
  actor's keys arrive with its `ActorAdded`, and `ResourceChanged` refuses a
  resource the actor does not hold (`git grep -nE 'Attributes\[|Resources\['
  -- internal/engine ':!*_test.go'` finds that arm alone, and `git grep -nE
  'Actors\[[^]]+\] *=' -- internal/engine ':!*_test.go'` finds
  `ActorAdded`'s store and `State.Snapshot`'s clone). The DM brings a
  refused campaign up to date by serving it with a ruleset it fits, or with
  none, removing an actor that does not fit and adding a corrected one
  under a new id (VTT-390).
- A sheet need not hold a value for every property its ruleset declares;
  what it holds is the DM's.
- A command that reaches the gateway with a resource's `current` left out
  or `null` holds a `0`, whoever wrote it: a `current` with no value is
  refused by the writer or not at all. `vtt client run`'s scenarios write
  commands without that check.
- `rules.Resolve`'s refusals of an ability name a key, and say attribute
  and defense; they are not in the table's words.

## Requirements

VTT-610, VTT-611, VTT-612, VTT-613, VTT-614, VTT-615, VTT-616, VTT-617,
VTT-618, VTT-626, VTT-627, VTT-628, VTT-629, VTT-630, VTT-670, VTT-671,
VTT-672, VTT-674, VTT-675, VTT-676, VTT-677, VTT-678, VTT-679, VTT-680,
VTT-681.
