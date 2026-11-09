# A sheet holds only values its ruleset declares and none written empty, and a campaign holding another is not served

## The problem

**`add_actor` is not read against the ruleset.** An actor's sheet is the
`attributes` and `resources` maps of the contract's `Actor`
(`contract/vtt/v1/events.proto`), each key a property the ruleset declares
(SPEC-024). `validateAddActor` (`internal/gateway/add_actor_validate.go`)
refuses a controller and a missing kind, and reads no key of either map; the
fold (`engine.Apply`'s `ActorAdded` arm) reads those keys for their length
alone. An actor added through `add_actor`, from the console's raw paste or
the MCP tool, can hold a key the ruleset does not declare, or a value under
the other type's map. SPEC-024's Consequences names this ticket as the one
that carries the check.

**An adventure's null becomes a value.** `adventure.Load` refuses an
undeclared key and a key under the other type's map (`checkStatblockKey`,
VTT-617, VTT-618). It decodes an actor's `attributes` into
`map[string]int32` and its `resources` into `resourceValJSON` with
`encoding/json` (`actorJSON`), which refuses a value written as an empty
string and reads `"brawn": null` as `0`, a resource written as `null` as
current `0` and maximum `0`, a resource whose `current` is `null` as current
`0`, and a resource written as `{}` the same way. Each loads, and the sheet
shows a value its author never wrote.

**`add_actor` refuses a null map value, not a null current.** `DecodeCommand`
(`internal/gateway/codec.go`) and the MCP server's `commandFromArgs`
(`internal/mcp/server.go`) decode with `protojson`, which refuses a number
or a resource written as `null` or as an empty string, and a `current`
written as an empty string. A resource whose `current` is written as `null`,
or left out, decodes to a `Resource` whose current is `0`, which proto3
cannot tell from a current written as `0`.

**A campaign is served whatever its sheets hold.** `composeServer`
(`cmd/vtt/serve_compose.go`) loads the ruleset `--ruleset` names and serves
the campaign without reading its actors against it. A campaign whose sheets
hold a key the ruleset does not declare is served.

**What a sheet holds is the DM's.** The owner ruled on 2026-11-08 that a
sheet is unique to its character and holds the values that character has: no
form says which properties a sheet must hold, no value is set automatically,
and keeping a campaign's sheets in step with its ruleset is the DM's, the
campaign owner's. What is refused is a value the ruleset does not have, and a
value that has none: written as `null` or as an empty string. A value of `0`
is a value, such as a resource spent to nothing (the owner, the same day). An
ability that reads a property the sheet does not hold is refused at
resolution by `rules.Resolve`, as today.

MapTool keeps a property type per token and reads a default where a token
holds no value (`CampaignProperties.tokenTypeMap`,
`Token.getPropertyDefault`); it keeps a value its type does not list, hidden.
This ticket borrows none of it: the owner struck forms and defaults, and
principle 26 refuses the hidden value.

## Done looks like

1. **`add_actor` refuses a value the ruleset does not have.** With a ruleset
   served, `add_actor` is refused, naming the actor, the map and the key,
   when the actor holds a key the ruleset does not declare, and when it holds
   a value under the other type's map. New tests in `internal/gateway`; on
   today's tree each is accepted. With no ruleset served, `add_actor` reads
   no key against a ruleset, as today.
2. **An adventure refuses a value that has none.** `adventure.Load` refuses
   an actor whose number is written as `null`, whose resource is written as
   `null`, or whose resource's `current` is written as `null` or left out,
   naming the file, the field and the key. New tests in
   `internal/adventure`; on today's tree each loads as `0`. A value of `0`
   still loads.
3. **A resource whose current has no value is refused where it is
   written.** The MCP tool `add_actor` and the console's raw paste of an
   actor each refuse an actor whose resource's `current` is written as
   `null` or left out, naming the actor and the key, with or without a
   ruleset served, before any command is sent. New tests in `internal/mcp`
   and `client/test`; on today's tree each is accepted with current `0`. A
   current written as `0` is still accepted. The gateway reads no such
   thing: both the browser and the MCP server leave a `current` of `0` out
   of the command they send, so in a command a `current` left out is a
   `0` (the owner, 2026-11-08, choosing the check where the value is
   written).
4. **A campaign holding a value the ruleset does not have is not served.**
   `vtt serve --ruleset` refuses to start a campaign any of whose actors holds
   a key the ruleset does not declare, or a value under the other type's map,
   naming each actor and the key, and leaves nothing open. A new test in
   `cmd/vtt`; on today's tree that campaign is served. With no ruleset, it
   is served, as today.
5. `task check` exits 0 on the branch.

Still refused afterwards: everything VTT-617 and VTT-618 refuse, every
refusal of SPEC-024, and every value written as an empty string or, where
`protojson` refuses it today, as `null`. Still accepted: a sheet that holds
no value for a property the ruleset declares, and a resource with no
maximum, its `max` left out or written as `null`, as today.

## Rules this puts on the system

- With a ruleset served, `add_actor` refuses an actor holding a key the
  ruleset does not declare, or a value under the other type's map.
- `adventure.Load` refuses an actor value written as `null`, and a resource
  whose `current` is written as `null` or left out.
- The MCP tool `add_actor` and the console's raw paste refuse a resource
  whose `current` is written as `null` or left out.
- `vtt serve` refuses to start a campaign any of whose actors holds a key the
  ruleset it is started with does not declare, or a value under the other
  type's map, naming each actor and the key.
- With no ruleset, neither `add_actor` nor `vtt serve` reads an actor's keys
  against a ruleset.

VTT-617 and VTT-618 stand.

## What it touches

- The check of an actor's keys against the ruleset: `checkStatblockKey` in
  `internal/adventure/load.go` today, and wherever the plan gives it one home
  for its callers: `adventure.Load`, `add_actor` and `vtt serve`'s boot.
- `internal/adventure/load.go` (`actorJSON`, `resourceValJSON`, `loadActors`).
- `internal/gateway/server.go` (`handleCommand`) and
  `internal/gateway/add_actor_validate.go`.
- Where an actor is still as it was written: `internal/mcp/server.go`
  (`commandFromArgs`), and `client/src/commands.ts` (`parseActorJSON`)
  with the rebuilt `cmd/vtt/webdist`.
- `cmd/vtt/serve_compose.go` (`composeServer`).
- `tools/toolgen/main.go`, whose doc for `add_actor`'s `attributes` and
  `resources` says nothing of the ruleset, and the generated
  `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`.
- The tests that add an actor with a ruleset served, and the test data whose
  actors hold a key their ruleset does not declare, if any.

## Specifications this moves

- docs/specifications/024-a-ruleset-declares-its-properties.md
- docs/specifications/013-authorization.md
- docs/specifications/022-a-client-holds-one-log.md
- docs/specifications/007-the-wire-contract.md

## What could not be established

- A command from a writer other than the browser and the MCP tool whose
  resource's `current` is written as `null` or left out reads as `0`. One
  such writer ships in the binary: `vtt client run`'s scenarios, which
  `runCommandStep` in `internal/harness/engine.go` decodes with `protojson`
  and sends through the same `harness.Client` the MCP server sends with; no
  participant plays through it. The gateway could tell a `current` left out
  from `0` only through a contract change that gives the field presence,
  which is not an addition (ADR-007), or by every writer writing a
  `current` of `0` explicitly (`"current": 0`), which neither client does; a `null` it could read in
  the frame, though neither client writes one there.
- How a campaign that is refused is brought up to date. Principle 26 and the
  owner make that the DM's; no command changes which keys a sheet holds, so the
  DM's way is to start it with a ruleset its sheets fit, or with none, and
  there remove the actor that does not fit (`remove_actor`) and add a
  corrected one under a new id.
- Whether a campaign served with no ruleset stays allowed. `--ruleset` is
  optional today and this ticket keeps it so: with no ruleset, nothing is
  read against one. Which ruleset a campaign is played under is recorded by
  step 1's next ticket (principle 28).
- Left out on the owner's word of 2026-11-08: forms, defaults, a check that
  a sheet holds every property, a resource's start carried by its first
  change, and the console's choice of form. Also left out, as proposed and
  not taken up: the sheet view (right-click, "Show character sheet"), which
  rides step 1's item 4; friend and foe, and things off the board, each its
  own ticket in step 1.
