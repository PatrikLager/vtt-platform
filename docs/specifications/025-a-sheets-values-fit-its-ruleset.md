# SPEC-025: A character sheet's values fit its ruleset

## Status

Accepted. Implemented by `contract/vtt/v1/events.proto`
(`Actor.ability_ids`), `internal/rules/sheet.go` (`SheetFault`,
`Ruleset.SheetKeyFault`, `Ruleset.SheetAbilityFault`, `Offers`),
`internal/adventure/load.go` (`actorJSON`, `resourceValJSON`, `loadActors`,
`actorValues`), `internal/adventure/format.go` (`AdventureActor`),
`internal/adventure/compile.go` (`buildActor`),
`internal/adventure/conformance/conformance.go` (`toActorAddedDump`),
`internal/gateway/add_actor_validate.go` (`validateAddActorWith`,
`validateAddActorSheet`), `cmd/vtt/serve_compose.go` (`sheetsFit`),
`internal/mcp/resource_current.go` (`refuseValuelessCurrent`),
`client/src/commands.ts` (`valuelessCurrent`) and `tools/toolgen/main.go`
(`add_actor`'s docs and `vtt.v1.Resource`'s override). Pinned by
`internal/rules/sheet_test.go`, `internal/rules/qa_sheet_key_test.go`,
`internal/rules/qa_sheet_ability_test.go`, `internal/rules/offers_test.go`,
`internal/rules/qa_offers_test.go`, `internal/adventure/load_test.go`,
`internal/adventure/statblock_type_test.go`,
`internal/adventure/qa_statblock_property_test.go`,
`internal/adventure/actor_value_test.go`,
`internal/adventure/qa_actor_value_test.go`,
`internal/adventure/actor_abilities_test.go`,
`internal/adventure/qa_actor_abilities_test.go`,
`internal/adventure/conformance/empty_abilities_test.go`,
`internal/gateway/add_actor_fit_internal_test.go`,
`internal/gateway/qa_add_actor_keys_test.go`,
`internal/gateway/add_actor_abilities_internal_test.go`,
`internal/gateway/qa_add_actor_abilities_test.go`,
`internal/mcp/resource_current_internal_test.go`,
`internal/mcp/qa_add_actor_current_test.go`,
`internal/mcp/add_actor_abilities_internal_test.go`,
`internal/engine/qa_actor_ability_ids_test.go`,
`contract/roundtrip_test.go`, `client/test/commands.test.ts`,
`client/test/qa-add-actor-paste.test.ts`,
`client/test/qa-paste-abilities.test.ts`, `cmd/vtt/campaign_fit_test.go`,
`cmd/vtt/qa_campaign_fit_test.go`, `cmd/vtt/campaign_abilities_fit_test.go`,
`cmd/vtt/qa_campaign_abilities_fit_test.go`, `cmd/vtt/qa_use_unnamed_test.go`,
`tools/toolgen/main_test.go`, `tools/toolgen/qa_add_actor_schema_test.go`
and `tools/toolgen/qa_add_actor_abilities_doc_test.go`.

## Principles served

Principles 26, 27 and 34 of `docs/blueprint.md`:

- **26.** An actor that gives a key the ruleset does not declare, or under
  the other type's map, or that names an ability the ruleset does not hold,
  is refused when an adventure holding it is loaded and when `add_actor`
  adds it with a ruleset served, and a campaign holding one is not served
  with that ruleset; each refusal names the key or the ability, and the
  actor or its file.
- **27.** A sheet names the ruleset's abilities it has by their ids and
  holds nothing else of them, and a character is offered only what its
  sheet names: a use (SPEC-013) and a player's read of the ruleset
  (SPEC-012) ask `rules.Offers` of the sheet, and the panel asks
  `sheetNames` (SPEC-022).
- **34.** `add_actor`'s description says what it refuses in a sheet, a key
  the served ruleset does not declare or declares as the other type and a
  resource whose `current` is null or left out (VTT-680), and an ability id
  the served ruleset does not hold (VTT-700), and that an actor uses only
  the abilities its `abilityIds` names (VTT-703); its schema requires a
  resource's `current` and not its `max` (VTT-674).

## How it works

**A sheet's values.** What this record reads of a character sheet is its
values: an actor's `attributes`, each a number under a property's key, and
its `resources`, each a `current`, and a `max` if it has one, under a
property's key; and the abilities the sheet names. Which keys and which
abilities fit is below. The sheet SPEC-016 sends a character log also holds
the actor's module data and conditions, which this record does not read.
How an expression reads a value is SPEC-024's.

**A sheet's abilities.** `Actor.ability_ids`, field 10 of `vtt.v1.Actor`
and `abilityIds` in JSON, names the ruleset's abilities the actor has, each
by its id in the ruleset's `Compiled`, and holds nothing else of them, so
what a use resolves is the ruleset's entry. A list keeps the order it was
written in and any repeat, and an `add_actor` the server accepts appends it
in that order whichever client sent it (VTT-683); no fold bounds its length
or an id's length (SPEC-018).
`Ruleset.SheetAbilityFault` reads the ids in byte order, from a sorted copy
that leaves its caller's list in its order, and answers the first that
`Compiled` does not hold as a `SheetFault`: the field `ability_ids`, the id,
and the reason `names ability "<id>", which the ruleset does not hold`
(VTT-685, VTT-687). It answers nil for a list whose every id is held,
repeats included, and for an empty one. `rules.Offers` answers whether an
actor's `ability_ids` names an id, compared byte for byte (VTT-690); the
refusal of a use (SPEC-013) and a player's read of `/api/ruleset`
(SPEC-012) ask it, and `git grep -nE 'Offers\(' -- '*.go' ':!*_test.go'`
finds no other caller.

**A sheet's keys.** `Ruleset.SheetKeyFault` reads a sheet's `attributes`
keys and then its `resources` keys, each in byte order, from a sorted copy
that leaves its caller's keys in their order, and answers the first that
does not fit as a `SheetFault`: the map that holds it, the key, and the
reason, `references undeclared property "<key>"` for a key no property
declares (VTT-617) or `gives <type> property "<key>" a <type>'s value` for a
key under the other type's map (VTT-618).

**Where a sheet is read.** `SheetKeyFault` and `SheetAbilityFault` have the
same callers, and `git grep -nE 'Sheet(Key|Ability)Fault\(' -- '*.go'
':!*_test.go'` finds no other beside their definitions. Each asks for the
keys first and for the ability ids only when every key fits, so a sheet is
refused for its first fault:
- `adventure.Load`, through `loadActors`, for each actor file, after its
  refusals of the actor's id, name and kind: its keys, as
  `adventure: <path>: field "<map>": <reason>`; then its values (below);
  then its ability ids, as `adventure: <path>: field "ability_ids":
  <reason>` (VTT-687).
- `handleCommand`, for an `add_actor`, through `validateAddActorWith` and
  `validateAddActorSheet`, after `validateAddActor`'s refusals (SPEC-013)
  and only for an actor with an id: with a ruleset served,
  `gateway: add_actor: actor "<id>": <field>: <reason>`, the field being
  the map for a key (VTT-677) and `ability_ids` for an ability (VTT-685);
  with none, neither a key nor an ability id is read against one (VTT-675,
  VTT-686).
- `composeServer`, through `sheetsFit`, when it is handed a ruleset, as
  `vtt serve --ruleset` hands it one: right after the ruleset loads and
  before any adventure, it reads every actor `campaign.Campaign.Actors`
  holds and, when any does not fit, closes the identity store and the
  campaign and refuses the boot with
  `vtt serve: ruleset <dir>: the campaign's actors do not fit it: ` and,
  joined by `; `, `actor "<id>": <field>: <reason>` for every actor that
  does not fit, each with its first fault, in byte order of their ids
  (VTT-678, VTT-681, VTT-688). With no ruleset, nothing is read (VTT-676,
  VTT-689).

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

**A sheet's abilities in an adventure.** `actorJSON` decodes an actor
file's `ability_ids` as a list of strings, and a list written `null` or
left out loads as none. `loadActors` keeps the list as written and
`buildActor` copies it, so the `ActorAdded` that `Compile` gives for the
actor names the same ids in the same order (VTT-684).
`internal/adventure/conformance`'s `toActorAddedDump` writes a compiled
actor's `ability_ids` only when it names some, so an adventure's
compiled-batch golden pins each actor's list, and an actor naming none
writes no field there.

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

## Consequences

- An actor gives only the keys its ruleset declares, each in its type's
  map, and names only abilities its ruleset holds, whether it comes from an
  adventure or through `add_actor`, and a campaign served with a ruleset
  holds no other. `git grep -n
  'Envelope_ActorAdded{' -- '*.go' ':!*_test.go' ':!contract/gen'` finds
  every writer of an `ActorAdded`; of them, `adventure.Compile`, for an
  adventure loaded against the ruleset, and `ToEvent`, for an `add_actor`
  the gateway has read, write to a campaign, while `internal/eventgen`
  draws histories for property tests and `perceive` writes a character
  log. A campaign served with no ruleset is read against none, so its
  actors may hold any keys and name any ids, and it is refused when it is
  next served with a ruleset they do not fit.
- No command changes which keys a sheet holds or which abilities it names:
  in `engine.Apply` an actor's keys and its ability ids arrive with its
  `ActorAdded`, `ResourceChanged` refuses a resource the actor does not
  hold (`git grep -nE 'Attributes\[|Resources\[' -- internal/engine
  ':!*_test.go'` finds that arm alone, and `git grep -nE
  'Actors\[[^]]+\] *=' -- internal/engine ':!*_test.go'` finds
  `ActorAdded`'s store and `State.Snapshot`'s clone), and no arm names an
  ability id (`git grep -n AbilityIds -- internal/engine ':!*_test.go'`
  finds nothing). The DM brings a refused campaign up to date by serving it
  with a ruleset it fits, or with none, removing an actor that does not fit
  and adding a corrected one under a new id (VTT-390).
- A sheet need not hold a value for every property its ruleset declares,
  nor name any ability; what it holds is the DM's. A character whose sheet
  names no ability can use none (SPEC-013).
- A correction to an ability in the ruleset reaches every sheet that names
  it, since a sheet holds the ability's id and nothing else of it.
- A command that reaches the gateway with a resource's `current` left out
  or `null` holds a `0`, whoever wrote it: a `current` with no value is
  refused by the writer or not at all. `vtt client run`'s scenarios write
  commands without that check.
- What a party member's sheet shows, its ability list included, and to
  whom, is the projection's (SPEC-016), not this record's.

## Requirements

VTT-617, VTT-618, VTT-670, VTT-671, VTT-672, VTT-674, VTT-675, VTT-676,
VTT-677, VTT-678, VTT-679, VTT-680, VTT-681, VTT-683, VTT-684, VTT-685,
VTT-686, VTT-687, VTT-688, VTT-689, VTT-690, VTT-700, VTT-703.
