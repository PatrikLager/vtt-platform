# SPEC-025: A character sheet's values fit its ruleset

## Status

Accepted. Implemented by `contract/vtt/v1/events.proto`
(`Actor.ability_ids`, `AbilityGiven`, `AbilityTaken`, `ResourceAdded`,
`ResourceRemoved`),
`contract/vtt/v1/commands.proto` (`GiveAbility`, `TakeAbility`),
`internal/engine/apply.go` (`Apply`'s `AbilityGiven`, `AbilityTaken`,
`ResourceAdded` and `ResourceRemoved` arms), `internal/rules/sheet.go`
(`SheetFault`, `Ruleset.SheetKeyFault`, `Ruleset.SheetAbilityFault`,
`Ruleset.SheetSpendFault`, `Ruleset.Spends`, `Offers`),
`internal/adventure/load.go` (`actorJSON`, `resourceValJSON`, `loadActors`,
`actorValues`), `internal/adventure/format.go` (`AdventureActor`),
`internal/adventure/compile.go` (`buildActor`),
`internal/adventure/conformance/conformance.go` (`toActorAddedDump`),
`internal/gateway/add_actor_validate.go` (`validateAddActorWith`,
`validateAddActorSheet`, `sheetFault`),
`internal/gateway/ability_validate.go` (`validateGiveAbility`),
`internal/gateway/ability_change.go` (`handleGiveAbility`,
`giveSheetFault`, `handleTakeAbility`, `orphanedBy`),
`internal/gateway/server.go` (`validateForm`),
`internal/gateway/convert.go` (`ToEvent`), `cmd/vtt/serve_compose.go`
(`sheetsFit`), `internal/mcp/resource_current.go` (`refuseValuelessCurrent`),
`client/src/commands.ts` (`valuelessCurrent`) and `tools/toolgen/main.go`
(`add_actor`'s, `give_ability`'s and `take_ability`'s docs and
`vtt.v1.Resource`'s override). Pinned by
`internal/rules/sheet_test.go`, `internal/rules/qa_sheet_key_test.go`,
`internal/rules/qa_sheet_ability_test.go`, `internal/rules/offers_test.go`,
`internal/rules/qa_offers_test.go`, `internal/rules/spends_test.go`,
`internal/rules/spend_fault_test.go`, `internal/rules/qa_spend_fault_test.go`,
`internal/adventure/load_test.go`,
`internal/adventure/actor_spend_test.go`,
`internal/adventure/qa_actor_spend_test.go`,
`internal/adventure/conformance/conformance_test.go`,
`internal/adventure/statblock_type_test.go`,
`internal/adventure/qa_statblock_property_test.go`,
`internal/adventure/actor_value_test.go`,
`internal/adventure/qa_actor_value_test.go`,
`internal/adventure/actor_abilities_test.go`,
`internal/adventure/qa_actor_abilities_test.go`,
`internal/adventure/actor_empty_ability_test.go`,
`internal/adventure/qa_actor_empty_ability_test.go`,
`internal/adventure/conformance/empty_abilities_test.go`,
`internal/engine/ability_change_test.go`,
`internal/engine/qa_ability_change_test.go`,
`internal/engine/resource_change_test.go`,
`internal/engine/qa_resource_change_test.go`,
`internal/gateway/add_actor_fit_internal_test.go`,
`internal/gateway/qa_add_actor_keys_test.go`,
`internal/gateway/add_actor_abilities_internal_test.go`,
`internal/gateway/qa_add_actor_abilities_test.go`,
`internal/gateway/ability_change_internal_test.go`,
`internal/gateway/ability_change_command_test.go`,
`internal/gateway/qa_ability_change_command_test.go`,
`internal/gateway/ability_value_test.go`,
`internal/gateway/qa_ability_value_test.go`,
`internal/gateway/ruleset_named_test.go`,
`internal/gateway/add_actor_spend_test.go`,
`internal/gateway/qa_add_actor_spend_test.go`,
`cmd/vtt/mcp_ability_change_e2e_test.go`,
`internal/mcp/resource_current_internal_test.go`,
`internal/mcp/qa_add_actor_current_test.go`,
`internal/mcp/add_actor_abilities_internal_test.go`,
`internal/engine/qa_actor_ability_ids_test.go`,
`contract/roundtrip_test.go`, `client/test/commands.test.ts`,
`client/test/qa-add-actor-paste.test.ts`,
`client/test/qa-paste-abilities.test.ts`, `cmd/vtt/campaign_fit_test.go`,
`cmd/vtt/qa_campaign_fit_test.go`, `cmd/vtt/campaign_abilities_fit_test.go`,
`cmd/vtt/qa_campaign_abilities_fit_test.go`, `cmd/vtt/qa_use_unnamed_test.go`,
`cmd/vtt/campaign_spend_fit_test.go`, `cmd/vtt/qa_serve_spend_test.go`,
`tools/toolgen/main_test.go`, `tools/toolgen/qa_add_actor_schema_test.go`
and `tools/toolgen/qa_add_actor_abilities_doc_test.go`.

## Principles served

Principles 26, 27 and 34 of `docs/blueprint.md`:

- **26.** An actor that gives a key the ruleset does not declare, or under
  the other type's map, that names an ability the ruleset does not hold,
  that names a limited ability without the resource it spends, or that
  holds a resource some ability of the ruleset spends while no ability it
  names spends it, is refused when an adventure holding it is loaded and
  when `add_actor` adds it with a ruleset served, and a campaign holding one
  is not served with that ruleset; with a ruleset served, a `give_ability`
  of an ability the ruleset does not hold is refused, and so is one of a
  limited ability whose resource the sheet does not hold that does not
  bring that resource's value; each refusal names the key or the ability,
  and the actor or its file.
- **27.** A sheet names the ruleset's abilities it has by their ids and
  holds nothing else of them, and a character is offered only what its
  sheet names: a use (SPEC-013) and a player's read of the ruleset
  (SPEC-012) ask `rules.Offers` of the sheet, and the panel asks
  `sheetNames` (SPEC-022).
- **34.** `add_actor`'s description says what it refuses in a sheet, a key
  the served ruleset does not declare or declares as the other type and a
  resource whose `current` is null or left out (VTT-680), and an ability id
  the served ruleset does not hold (VTT-700); that `use_ability` refuses an
  actor any ability its sheet does not name, so an actor naming none can
  use none until it is given one, and that `give_ability` and
  `take_ability` change the list in play (VTT-729); and that an empty id in
  `abilityIds` refuses the command with or without a ruleset served
  (VTT-749), and that with a ruleset served so does an ability that spends a
  resource the actor does not hold, or a resource some ability spends that no
  ability the actor names spends (VTT-786); its schema requires a resource's
  `current` and not its `max` (VTT-674). `give_ability`'s description says
  when its `resources` is required and when it is refused, and
  `take_ability`'s which resource leaves with the ability (VTT-785,
  SPEC-007).

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
(VTT-688, VTT-750, VTT-751). It answers nil for a list whose every id is
held, repeats included, and for an empty one. A loaded ruleset holds no
empty ability id, since `internal/rules/load.go` refuses an ability file
whose `id` is empty, so it answers an empty id as one the ruleset does not
hold. `rules.Offers` answers whether an
actor's `ability_ids` names an id, compared byte for byte (VTT-690); the
refusal of a use (SPEC-013) and a player's read of `/api/ruleset`
(SPEC-012) ask it, and `git grep -nE 'Offers\(' -- '*.go' ':!*_test.go'`
finds no other caller.

**A sheet's list in play.** An actor's `ability_ids` arrive with its
`ActorAdded` and change in play by two events, each naming one actor and one
ability, by the owner's ruling OD-1 of 2026-11-14: `AbilityGiven`, which
`ToEvent` writes for a `give_ability`, and `AbilityTaken`, which it writes
for a `take_ability` (SPEC-007). `engine.Apply`'s `AbilityGiven` arm
refuses an empty ability id (`engine: ability_given requires an ability
id`, VTT-722), then an actor the state does not hold (`engine:
ability_given names unknown actor "<actor>"`, VTT-717), then an id the
actor's list already names (`engine: actor "<actor>" already names ability
"<id>"`, VTT-720), and otherwise sets the list to a fresh copy ending with
the id (VTT-764). Its `AbilityTaken` arm refuses an actor the state does
not hold (`engine: ability_taken names unknown actor "<actor>"`), then an
id the list does not name (`engine: actor "<actor>" does not name ability
"<id>"`, VTT-721), and otherwise sets the list to a fresh one holding no
occurrence of the id, the others in their order (VTT-991, VTT-768). Neither
arm changes anything else of the actor or of the state (VTT-715), and the
client's fold refuses and changes what this one does (SPEC-022). A refused
give or take reaches its issuer and appends nothing (SPEC-013). With a
ruleset served, a `give_ability` is read against it and against the actor's
sheet before anything is written, and a `take_ability`'s handler asks it
which resource leaves with the ability (below); the fold refuses a take of
an id the sheet does not name. Who may give and take is SPEC-013's,
what the tools say of them SPEC-007's, and what a character log is sent of
them SPEC-016's.

**A sheet's resources in play.** An actor's `resources` arrive with its
`ActorAdded` and change in play by two events, each naming one actor and one
resource: `ResourceAdded`, which carries the resource's value, and
`ResourceRemoved` (SPEC-007). `engine.Apply`'s `ResourceAdded` arm refuses a
resource name that is empty or longer than 128 bytes (SPEC-018), then an
event with no value (`engine: resource_added requires a value`), then an
actor the state does not hold (`engine: resource_added names unknown actor
"<actor>"`), then a resource the actor already holds (`engine: actor
"<actor>" already holds resource "<resource>"`) (VTT-770), and otherwise
writes the actor's resources as a new map holding the resource at the
event's current and max (VTT-769, VTT-792), whatever their numbers, a
current above its max or below zero included (VTT-791). Its
`ResourceRemoved` arm refuses an actor the state does not hold (`engine:
resource_removed names unknown actor "<actor>"`), then a resource the actor
does not hold (`engine: actor "<actor>" does not hold resource
"<resource>"`) (VTT-772), and otherwise writes a new map without it
(VTT-771, VTT-792). Neither arm changes anything else of the actor or of the
state (VTT-773), and the client's fold refuses and changes what this one
does (SPEC-022). `ResourceChanged` refuses a resource the actor does not
hold, so a resource's value changes in play only while the sheet holds it.

**What an ability spends.** `Ruleset.Spends` answers the resource a limited
ability the ruleset holds spends, its `usage`'s `limited` resource, and
nothing for an at-will ability or an id the ruleset does not hold.
`Ruleset.SheetSpendFault` reads a sheet's ability ids in byte order and
answers the first naming a limited ability whose resource the sheet does not
hold, as a `SheetFault` with the field `ability_ids`, the id, and the reason
`names ability "<id>", which spends resource "<resource>" the sheet does not
hold`; then its resources in byte order, and answers the first that some
ability of the ruleset spends and no ability the sheet names spends, with
the field `resources`, the key, and the reason `holds resource "<key>",
which no ability the sheet names spends`, a reason that names no ability,
since several may spend the resource; and otherwise nil (VTT-775 to
VTT-777). Each reads from a sorted copy that leaves its caller's lists in
their order. A resource no ability of the ruleset spends, such as `hp` in
`dnd45e-minimal` or `drink` in `tavern-brawl`, is the character's own
counter, and neither half reads it.

**A give and a take, with a ruleset served.** A `give_ability` of a limited
ability whose resource the actor's sheet does not hold, carrying that
resource's value under `resources`, appends the `AbilityGiven` and then a
`ResourceAdded` of that resource at the carried current and max, after
which the sheet names the ability and holds the resource at that value, a
current of 0 included (VTT-760). A give carrying no `resources`, of an
ability that spends no resource or whose resource the sheet holds, is the
`AbilityGiven` alone (VTT-764); no give's batch holds a `RulesetNamed`,
since what a give writes is the DM's (VTT-990). A `take_ability` appends,
after any `RulesetNamed` SPEC-026 says opens its batch (VTT-988), the
`AbilityTaken`
and, when the taken ability spends a resource the sheet holds that no other
id the sheet names spends, a `ResourceRemoved` of it, and nothing else, so
a resource several abilities spend stays while one of them is named
(VTT-991). What each refuses, and with none served what each does, is
SPEC-013's. The gateway reads no `current`: the give copies the value it
carries whole, as `add_actor` copies an actor's.

**A sheet's keys.** `Ruleset.SheetKeyFault` reads a sheet's `attributes`
keys and then its `resources` keys, each in byte order, from a sorted copy
that leaves its caller's keys in their order, and answers the first that
does not fit as a `SheetFault`: the map that holds it, the key, and the
reason, `references undeclared property "<key>"` for a key no property
declares (VTT-617) or `gives <type> property "<key>" a <type>'s value` for a
key under the other type's map (VTT-618).

**Where a sheet is read.** `SheetKeyFault`, `SheetAbilityFault` and
`SheetSpendFault` have the callers below, and `git grep -nE
'Sheet(Key|Ability|Spend)Fault\(' -- '*.go' ':!*_test.go'` finds no other
beside their definitions. Where a caller reads a whole sheet it asks for the
keys first, for the ability ids only when every key fits, and for the spend
rule only when every id is held, so a sheet is refused for its first fault.
Where a sheet enters the campaign, by `add_actor`, with a ruleset served or
none, or by `adventure.Load`, which always reads against one, an empty ability
id is refused in words of its own, before the ids are read against a ruleset,
while both folds keep their own refusal of an `AbilityGiven` whose id is
empty:
- `adventure.Load`, through `loadActors`, for each actor file, after its
  refusals of the actor's id, name and kind: its keys, as
  `adventure: <path>: field "<map>": <reason>`; then its values (below);
  then an `ability_ids` holding an empty id, as `adventure: <path>: field
  "ability_ids": must not hold an empty id` (VTT-748); then its ability
  ids, as `adventure: <path>: field "ability_ids": <reason>` (VTT-751);
  then the spend rule, as `adventure: <path>: field "<field>": <reason>`
  (VTT-776).
  `adventure.Load` always reads against a ruleset: it compares the
  adventure's declared ruleset with the one it is handed before it reads
  an actor.
- `handleCommand`, for an `add_actor`, through `validateForm`,
  `validateAddActorWith`, `validateAddActorSheet` and `sheetFault`, after
  `validateAddActor`'s refusals (SPEC-013) and only for an actor with an
  id, as `gateway: add_actor: actor "<id>": <field>: <reason>`: with a
  ruleset served, its keys, the field being the map (VTT-677); then, with a
  ruleset served or none, an `ability_ids` holding an empty id, with the
  reason `must not hold an empty id` (VTT-747); then, with a ruleset served,
  its ability ids (VTT-750); then the spend rule (VTT-775). With none,
  neither a key, nor a non-empty ability id, nor an ability against a
  resource is read against one (VTT-675, VTT-746, VTT-778).
- `handleCommand`, for a `give_ability`, through `validateForm` and
  `validateGiveAbility`, with a ruleset served: the give's one ability id,
  as `gateway: give_ability: actor "<actor>": ability_ids: <reason>`, the
  shape `validateAddActorSheet` gives, whether or not the campaign holds
  the actor (VTT-718); then, asking `Spends`, each key under `resources`
  against the resource the ability spends, and in `giveSheetFault` against
  the actor's sheet (SPEC-013). It asks `SheetKeyFault` of no key: a key it
  accepts is the resource the ability spends, which the ruleset declares as
  a resource, since its compile refuses a `usage.limited.resource` it does
  not. With none served it refuses a
  give carrying any `resources` and reads nothing else (SPEC-013).
- `composeServer`, through `sheetsFit`, when it is handed a ruleset, as
  `vtt serve --ruleset` hands it one: right after the ruleset loads and
  before any adventure, it reads every actor `campaign.Campaign.Actors`
  holds and, when any does not fit, closes the identity store and the
  campaign and refuses the boot with
  `vtt serve: ruleset <dir>: the campaign's actors do not fit it: ` and,
  joined by `; `, `actor "<id>": <field>: <reason>` for every actor that
  does not fit, each with its first fault, its keys, then its ability ids,
  then the spend rule, in byte order of their ids (VTT-678, VTT-681,
  VTT-688, VTT-777). With no ruleset, nothing is read (VTT-676, VTT-689,
  VTT-778). `composeServer` reads no ruleset id from the log, so a
  campaign whose log names a ruleset of another id is served when its
  sheets fit, and its next use or take records the switch (SPEC-026,
  VTT-977).

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
adventure (above), in the MCP tools `add_actor` and `give_ability`, in the DM
console's paste of an actor and in its Give. The gateway does not read it
(`git grep -nE 'GetCurrent\(\)|\.Current([^A-Za-z0-9_]|$)' -- internal/gateway
':!*_test.go'` finds nothing), since the browser and the MCP server both leave
a `current` of `0` out of the command they send, so in a command a `current`
left out is a `0`.
- `commandFromArgs` runs `refuseValuelessCurrent` over a tool's arguments
  once `protojson` has accepted them. It walks them by the command's
  descriptor, never into a `google.protobuf` message, and refuses the first
  `vtt.v1.Resource`, map entries in byte order of their keys, whose
  `current` is `null` or left out, with `mcp: invalid arguments for
  <command>: actor "<id>": resources.<key>.current must be a number, 0
  included; a resource gives its current`, the command being `add_actor` or
  `give_ability` and the actor the `actor_id` of the `vtt.v1.Actor` or the
  `vtt.v1.GiveAbility` that holds the resource, before any command is sent
  (VTT-671, VTT-787). It runs before the gateway's refusals, and names an
  actor with no id `actor ""`. A resource written `null` is refused by
  `protojson`, in its own words.
- `parseActorJSON` runs `valuelessCurrent` over the paste as written, after
  its refusals of a missing id, a controller and a missing kind, and refuses
  the first resource, in UTF-16 code-unit order of its keys, whose `current`
  is `null` or left out, naming the actor and the key, before any command is
  built (VTT-672).
- The DM console's Give refuses a current that is empty or not an integer
  before it sends anything (VTT-789, SPEC-022).
- A `current` of `0` passes each, and a `max` left out or written `null` is
  no maximum (VTT-679). `add_actor`'s schema requires a resource's `current`
  and not its `max` (VTT-674, SPEC-007), and its description says what it
  refuses in a sheet (VTT-680).

## Consequences

- An actor gives only the keys its ruleset declares, each in its type's
  map, and names only abilities its ruleset holds, whether it comes from an
  adventure or through `add_actor`, or is given one by `give_ability`, and
  a campaign served with a ruleset holds no other. With a ruleset served,
  it names a limited ability only beside the resource that ability spends,
  and holds a resource some ability of the ruleset spends only beside an
  ability that spends it, wherever it enters, and a give and a take keep it
  so, save two writers at once (below). The adventures under the
  repository's `adventures` directory load against the rulesets they name
  (VTT-802). `git grep -n
  'Envelope_ActorAdded{' -- '*.go' ':!*_test.go' ':!contract/gen'` finds
  every writer of an `ActorAdded`; of them, `adventure.Compile`, for an
  adventure loaded against the ruleset, and `ToEvent`, for an `add_actor`
  the gateway has read, write to a campaign, while `internal/eventgen`
  draws histories for property tests and `perceive` writes a character
  log. A campaign served with no ruleset is read against none, so its
  actors may hold any keys and name any ids, and it is refused when it is
  next served with a ruleset they do not fit.
- No command changes an actor's attributes, and only a give and a take
  change which resources it holds: in `engine.Apply` an actor's keys
  arrive with its `ActorAdded`, its resources change by a `ResourceAdded`
  or a `ResourceRemoved`, and `ResourceChanged` refuses a resource the
  actor does not hold (`git grep -nE 'Attributes\[|Resources\[' --
  internal/engine ':!*_test.go'` finds that arm alone, `git grep -nE
  '\.Resources *=' -- internal/engine ':!*_test.go'` finds the
  `ResourceAdded` and `ResourceRemoved` arms alone, and `git grep -nE
  'Actors\[[^]]+\] *=' -- internal/engine ':!*_test.go'` finds
  `ActorAdded`'s store and `State.Snapshot`'s clone). Which abilities a
  sheet names changes by an `AbilityGiven` or an `AbilityTaken` alone:
  `git grep -n AbilityIds -- internal/engine ':!*_test.go'` finds those two
  arms and no other. In the world they come from a `give_ability` or a
  `take_ability`, the DM's and the agent's (SPEC-013): `git grep -nE
  'Envelope_(Ability(Given|Taken)|Resource(Added|Removed))\{' -- '*.go'
  ':!*_test.go' ':!contract/gen'` finds `ToEvent`'s, the give's and the
  take's handlers' resource events, the projection's correction (SPEC-016)
  and `internal/eventgen`'s draws for property tests.
- The DM brings a refused campaign up to date by serving it with a ruleset
  it fits, or by serving it with none and taking with `take_ability` an
  ability id the ruleset does not hold or a limited ability whose resource
  the sheet does not hold, or giving with `give_ability` an ability that
  spends a resource the sheet holds and no ability it names spends, or by
  removing an actor that does not fit and adding a corrected one under a
  new id (VTT-390).
- A sheet need not hold a value for every property its ruleset declares,
  nor name any ability; but for the spend rule, what it holds is the DM's.
  A character whose sheet names no ability can use none (SPEC-013) until it
  is given one.
- A take never removes a resource no ability of the ruleset spends, nor,
  save two writers at once (below), one another ability on the sheet
  spends.
- Two writers at once can leave a sheet out of the spend rule: two takes of
  abilities that share a resource, each read before the other lands, each
  leave it, and a give carrying no value can land beside the take that
  removes the resource it spends (OD-6 of
  `docs/superpowers/plans/2026-11-15-an-ability-is-given-with-its-value.md`);
  so can a give carrying no value that lands just after the `add_actor` of
  an actor its read did not hold. The next `vtt serve --ruleset` refuses the
  campaign, naming the actor.
- `Resolve` still refuses a use of a limited ability by an actor whose sheet
  does not hold the resource it spends, once the earlier refusals let it
  through, naming the actor, the resource and the ability (VTT-804), since
  two writers at once can leave such a sheet (above), and `Resolve` decides
  from the snapshot it is handed.
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
VTT-677, VTT-678, VTT-679, VTT-680, VTT-681, VTT-683, VTT-684, VTT-688,
VTT-689, VTT-690, VTT-700, VTT-715, VTT-717, VTT-718, VTT-720, VTT-721,
VTT-722, VTT-729, VTT-746, VTT-747, VTT-748, VTT-749, VTT-750, VTT-751,
VTT-760, VTT-764, VTT-768, VTT-769, VTT-770, VTT-771, VTT-772,
VTT-773, VTT-775, VTT-776, VTT-777, VTT-778, VTT-785, VTT-786, VTT-787,
VTT-789, VTT-791, VTT-792, VTT-802, VTT-804, VTT-977, VTT-988, VTT-990,
VTT-991.
