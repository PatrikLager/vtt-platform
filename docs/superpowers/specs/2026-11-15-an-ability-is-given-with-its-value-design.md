# A sheet holds a limited ability only with the resource it spends, and a resource an ability spends only with an ability that spends it

## The problem

**A limited ability spends a resource the sheet need not hold.** A ruleset's
ability says how it is used in `usage`: `at_will`, or `limited`, which names a
resource and a cost (`Usage` and `LimitedUsage` in `internal/rules/format.go`).
`hunters-flurry` (`rulesets/dnd45e-minimal/abilities/hunters-flurry.json`)
spends `flurry_uses`, and `chair-swing`
(`rulesets/tavern-brawl/abilities/chair-swing.json`) spends `stamina`. A
resource on a sheet is a `Resource`, a current and a max
(`contract/vtt/v1/events.proto`), in the actor's `resources` map. `Resolve`
(`internal/rules/resolve.go`) refuses every use of a limited ability by an
actor whose sheet holds no entry for the resource it spends ("has no resource
… required by ability …'s usage cost"). Nothing adds a resource to an actor the
campaign already holds: `engine.Apply`'s `ResourceChanged` arm
(`internal/engine/apply.go`) refuses a resource the actor does not hold, and of
the `ClientCommand`s (`contract/vtt/v1/commands.proto`) only `use_ability`
changes the resources of an actor the campaign holds, through its outcomes,
which change entries already there.

**A give puts a limited ability on a sheet without its resource.**
`give_ability` carries an actor and an ability and nothing else
(`GiveAbility`); `validateGiveAbility` (`internal/gateway/ability_validate.go`)
reads the ability id against the ruleset alone, and the `AbilityGiven` arm of
`engine.Apply` adds the id to the list. A give of `hunters-flurry` to an actor
whose sheet holds no `flurry_uses` is accepted, and the character holds an
ability that every use refuses and that nothing at the table can repair short
of removing the actor and adding it again.

**A take leaves the resource behind.** `AbilityTaken` removes the id from the
list and nothing else. After a take of `hunters-flurry`, the sheet still holds
`flurry_uses`, which no ability on it spends.

**A sheet enters a campaign the same way.** `add_actor`
(`validateAddActorSheet` and `sheetFault` in
`internal/gateway/add_actor_validate.go`), `adventure.Load` (`loadActors` in
`internal/adventure/load.go`) and `vtt serve --ruleset` (`sheetsFit` in
`cmd/vtt/serve_compose.go`) read a sheet's keys against the ruleset
(`SheetKeyFault`) and its ability ids against the abilities it holds
(`SheetAbilityFault`, both in `internal/rules/sheet.go`). None reads an ability
against the resource it spends, so a sheet naming `hunters-flurry` without
`flurry_uses`, or holding `flurry_uses` without naming `hunters-flurry`, enters
whichever way it comes. Principle 26 says a sheet that does not fit its ruleset
is refused when it enters the campaign, whichever way it enters.

**What the owner ruled (2026-11-15).**
- A give of an ability whose resource the sheet does not hold carries that
  resource's value at the same time; 0 is a value, and a missing value is
  refused.
- A sheet holds only what can be used: a take removes each resource the taken
  ability spends that no ability left on the sheet spends. A resource spent by
  several abilities, like a pool every ability draws on, stays while one of them
  is on the sheet.
- The same rule holds where a sheet enters a campaign, in this ticket.
- A give that carries a value for a resource the sheet already holds is refused.
- Attributes are the ruleset's: a roll reads an attribute whatever its value,
  and this ticket checks no attribute.

The rule reaches only a resource some ability of the ruleset spends as its
cost: today `flurry_uses` and `stamina`. A resource no ability spends, like
`hp` in dnd45e-minimal and `drink` in tavern-brawl, is the character's own
counter, changed by abilities aimed at it and read by the ruleset's
thresholds, and no take removes it.

**What MapTool does.** A token's property may hold no value
(`Token.getProperty` returns what its map holds, `null` included), its property
type supplies a default (`Token.getPropertyDefault`,
`TokenProperty.getDefaultValue`), and a macro on the token reads whatever is
there; nothing ties a macro to a property, so a token can hold a macro whose
property it lacks. This ticket takes the opposite answer, for the reason
principle 26 gives: a missing value is never filled in by a default (the
owner, 2026-11-08), and a limited ability never sits on a sheet without the
resource it spends.

## Done looks like

Each observation below is with a ruleset served unless it says otherwise.

1. **A give carries the resource's value.** A `give_ability` of a limited
   ability whose resource the actor's sheet does not hold, carrying a value for
   that resource, appends what makes the sheet name the ability and hold the
   resource at that value: given `hunters-flurry` with `flurry_uses` at 1, the
   actor's `use_ability` of it resolves where today it is refused with
   `has no resource`; given it at 0, the sheet holds `flurry_uses` at 0 and the
   use is refused as too few, not as missing. `vtt state dump` shows the
   resource. A gateway test fails on today's tree, whose `GiveAbility` carries
   no value.
2. **What a give refuses, appending nothing.** Each with a test that finds the
   log's length unchanged:
   - a give of a limited ability whose resource the sheet does not hold,
     carrying no value, by an error naming the actor, the ability and the
     resource;
   - a give carrying a value for a resource the sheet already holds, which
     stays as it was;
   - a give carrying a value for a resource the ability does not spend, an
     at-will ability's included.
   A give of a limited ability whose resource the sheet holds, carrying no
   value, is accepted and leaves the resource as it was.
3. **A take removes the resource no ability left spends.** A `take_ability` of
   a limited ability removes from the sheet the resource it spends when no
   ability left on the sheet spends it: after a take of `hunters-flurry` the
   state holds no `flurry_uses`. When another ability on the sheet spends the
   same resource, it stays at its value; a test with a ruleset in which two
   abilities spend one resource holds it. A take of an at-will ability removes
   no resource, and no take removes a resource no ability of the ruleset spends
   (`hp`, `drink`).
4. **Nothing else changes.** After a give or a take, the actor's tokens,
   controllers, conditions, attributes and every resource but the one the give
   added or the take removed are what they were; a test compares the state
   before and after, field by field.
5. **A sheet entering a campaign fits the rule.** `add_actor`, `adventure.Load`
   and `vtt serve --ruleset` each refuse a sheet that names a limited ability
   without the resource it spends, and a sheet holding a resource some ability
   of the ruleset spends while no ability the sheet names spends it, naming the
   actor (or the file), the field and the key. Each has a test that fails on
   today's tree. The adventures and scenarios in the repository still load,
   seed and serve.
6. **With no ruleset served, nothing here is checked.** A take, `add_actor`
   and `vtt serve` read nothing against a ruleset, as SPEC-025 has it for an
   ability id today: a take removes no resource, and `add_actor` is refused for
   nothing this ticket adds. What a give carrying a value does then is below,
   under what could not be established.
7. **A character log follows the sheet.** A log whose eyes see the party member
   when a give adds or a take removes a resource holds the sheet's resources
   afterwards; a log whose eyes do not see it holds them once its eyes next see
   the actor; no log is sent an `ActorRemoved` or an `ActorAdded` for either,
   and a log resumed after one resumes rather than starting over. A creature's
   give or take still reaches no player's log. Projection tests in
   `internal/perceive` hold each case, and fail on today's tree, where
   `sameIncarnation` and `statusFrames` (`internal/perceive/projection.go`)
   read a changed set of resources as another actor.
8. **The client's fold agrees with the server's.** The client folds a give that
   adds a resource and a take that removes one to the state `vtt state dump`
   prints for the same log, refusals included.
9. **The agent can do it.** `vtt mcp` lists `give_ability` with the value in its
   schema, saying when the value is required and when it is refused, and
   `take_ability` saying which resource leaves with the ability;
   `contract/testdata/expected_tools.json` and the generated
   `contract/gen/tools/tools.json` hold them. A value left without its current
   is refused as `add_actor`'s is (`refuseValuelessCurrent` in
   `internal/mcp/resource_current.go`).
10. **The DM console can do it.** Choosing, in an actor's row, a limited ability
    whose resource the sheet does not hold asks for that resource's value and
    sends it with the give; choosing one whose resource the sheet holds, or an
    at-will ability, asks for none; a give with the value asked for and left
    empty sends nothing and says so. A client test drives each.
11. **The gate.** `task check` is green.

## Rules this puts on the system

- A give of a limited ability whose resource the sheet does not hold is refused
  unless it carries that resource's value, and then the sheet holds the
  resource at that value.
- A give carrying a value for a resource the sheet already holds, or for one
  the ability does not spend, is refused.
- A take removes each resource the taken ability spends that no ability left on
  the sheet spends, and no other resource.
- A sheet entering a campaign is refused when it names a limited ability
  without the resource it spends, or holds a resource some ability of the
  ruleset spends while no ability it names spends it.
- With no ruleset served, a take removes no resource, and a sheet entering a
  campaign is refused for none of the above.
- A character log never reads a resource added by a give or removed by a take
  as the actor's removal or its addition.
- The DM console asks for a resource's value exactly when the chosen ability
  spends one the sheet does not hold.

These overturn, as worded, VTT-713 (a give of an ability the sheet does not
name appends an `AbilityGiven`), VTT-715 (the folds change nothing of an actor
at an `AbilityGiven` or an `AbilityTaken` but its `ability_ids`) and VTT-728
(the tools say nothing else about the actor changes).

## What it touches

In the order the change must reach them:

- The contract: `contract/vtt/v1/commands.proto` (`GiveAbility`) and
  `contract/vtt/v1/events.proto` (`AbilityGiven`, `AbilityTaken`),
  additively (ADR-007), and what `task generate:contract` regenerates under
  `contract/gen`.
- The fold: `internal/engine/apply.go` (`Apply`'s `AbilityGiven` and
  `AbilityTaken` arms).
- The ruleset's check: `internal/rules/sheet.go`, beside `SheetKeyFault` and
  `SheetAbilityFault`.
- The gateway: `internal/gateway/ability_validate.go`
  (`validateGiveAbility`), `internal/gateway/add_actor_validate.go`
  (`sheetFault`), `internal/gateway/convert.go`, and the command path in
  `internal/gateway/server.go`, which a take needs to read the sheet and the
  ruleset.
- Where a sheet enters: `internal/adventure/load.go` (`loadActors`) and
  `cmd/vtt/serve_compose.go` (`sheetsFit`).
- The projection: `internal/perceive/projection.go` (`sameIncarnation`,
  `statusFrames`, `classify`, `Resume`).
- The agent's tools: `tools/toolgen/main.go`, `internal/mcp/resource_current.go`,
  the generated `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`, and
  `contract/testdata/expected_tools.json`.
- The client: `client/src/fold.ts` and `client/src/state.ts` (the two arms),
  `client/src/commands.ts` (`giveAbility`), `client/src/view/dm.ts` (the
  console's give), and the rebuilt `cmd/vtt/webdist`.
- The goldens: `scenarios/goldens` and the scenarios that feed
  `client/test/fold-parity.test.ts`.

This crosses more than two components and names more than ten files, as the
give-and-take ticket did. The parts share one rule and one pair of events, and
the owner ruled on 2026-11-15 that where a sheet enters is in this ticket; the
plan proposes whether anything splits off.

## Specifications this moves

- docs/specifications/025-a-sheets-values-fit-its-ruleset.md
- docs/specifications/007-the-wire-contract.md
- docs/specifications/013-authorization.md
- docs/specifications/016-the-projection.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **The alpha campaign.** `tobbe` in `~/dev/vtt-alpha/adventures/two-rooms`,
  outside the repository, holds `flurry_uses` and names no ability. Once this lands,
  `adventure.Load` refuses that adventure, which `composeServer` loads at start
  when it is handed the adventures directory, and `vtt serve --ruleset` refuses
  the campaign unless its state names `hunters-flurry` for that actor by then. Whether it is brought up to date before or after is the owner's
  (principle 26: bringing it up to date is the DM's).
- **What the events carry.** Whether an `AbilityGiven` carries the resource and
  its value and an `AbilityTaken` the resources it removes, or the give and the
  take append a resource's own event beside them, is the plan's. The engine
  reads no ruleset, so whichever it is, the event says which resource changes.
- **A value with no ruleset served.** With no ruleset, the gateway cannot tell
  whether an ability spends a resource. Whether a give carrying a value is then
  accepted, writing the resource, or refused, is the plan's to propose.
- **The test fixtures.** Tests that add an actor with a resource no ability on
  its sheet spends, or a limited ability without its resource, under a served
  ruleset, will be refused once item 5 lands. How many there are was not
  counted here.
- **The message and the Story.** The player is told of a give and a take
  (VTT-733), and the Story draws a line for each (VTT-736). Whether either
  also names the resource that came or went is not ruled.
