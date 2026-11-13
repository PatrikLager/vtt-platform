# SPEC-025: A character sheet's values fit its ruleset

## Status

Accepted. Implemented by `internal/rules/sheet.go` (`SheetFault`,
`Ruleset.SheetKeyFault`), `internal/adventure/load.go` (`actorJSON`,
`resourceValJSON`, `loadActors`, `actorValues`),
`internal/gateway/add_actor_validate.go` (`validateAddActorWith`),
`cmd/vtt/serve_compose.go` (`sheetsFit`), `internal/mcp/resource_current.go`
(`refuseValuelessCurrent`), `client/src/commands.ts` (`valuelessCurrent`)
and `tools/toolgen/main.go` (`add_actor`'s docs and `vtt.v1.Resource`'s
override). Pinned by `internal/rules/sheet_test.go`,
`internal/rules/qa_sheet_key_test.go`, `internal/adventure/load_test.go`,
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
`tools/toolgen/qa_add_actor_schema_test.go`.

## Principles served

Principles 26 and 34 of `docs/blueprint.md`:

- **26.** An actor that gives a key the ruleset does not declare, or under
  the other type's map, is refused when an adventure holding it is loaded and
  when `add_actor` adds it with a ruleset served, and a campaign holding one
  is not served with that ruleset; each refusal names the key, and the actor
  or its file.
- **34.** `add_actor`'s description says what it refuses in a sheet, a key
  the served ruleset does not declare or declares as the other type and a
  resource whose `current` is null or left out (VTT-680), and its schema
  requires a resource's `current` and not its `max` (VTT-674).

## How it works

**A sheet's values.** What this record reads of a character sheet is its
values: an actor's `attributes`, each a number under a property's key, and
its `resources`, each a `current`, and a `max` if it has one, under a
property's key. Which keys fit is below. The sheet SPEC-016 sends a
character log also holds the actor's module data and conditions, which
this record does not read. How an expression reads a value is SPEC-024's.

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
  holds and, when any does not fit, closes the identity store and the
  campaign and refuses the boot with
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

## Consequences

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
- What a party member's sheet shows, and to whom, is the projection's
  (SPEC-016), not this record's.

## Requirements

VTT-617, VTT-618, VTT-670, VTT-671, VTT-672, VTT-674, VTT-675, VTT-676,
VTT-677, VTT-678, VTT-679, VTT-680, VTT-681.
