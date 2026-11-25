# A sheet refers to the catalogue, and the DM and the agent change what it holds

## The problem

**A sheet holds no reference.** `vtt.v1.Actor` (`contract/vtt/v1/events.proto`)
carries an actor's `attributes`, each a number under a property's key, its
`resources`, its `module_data`, its controllers and kind, and `ability_ids`,
the ruleset's abilities it names (SPEC-025). No field names a catalogue entry,
so a character cannot carry the longsword
`rulesets/dnd45e-minimal/catalogue/longsword.json` declares. An adventure's
actor file is decoded by `actorJSON` (`internal/adventure/load.go`), which has
`actor_id`, `name`, `kind`, `attributes`, `resources` and `ability_ids`, and no
field for an entry either.

**A ruleset declares no place on a sheet.** `manifestJSON`
(`internal/rules/load.go`) reads `properties`, `forms` and `sheet` among its
fields. A form lists the number properties its entries carry (SPEC-027), and
`sheet` lays out pages of sections, each showing property keys, the
character's conditions or abilities by usage (`SheetSection` in
`internal/rules/format.go`). Nothing declares where on a sheet an entry goes,
in the hands, worn or carried, or the states it may be in there.

**In play, only a sheet's abilities and resources change.** `give_ability` and
`take_ability` (`GiveAbility` and `TakeAbility` in
`contract/vtt/v1/commands.proto`) write `AbilityGiven`, `AbilityTaken`,
`ResourceAdded` and `ResourceRemoved`, which `engine.Apply`
(`internal/engine/apply.go`) folds, and `internal/gateway/authz.go` lets only
the DM and the agent send them. No command gives a character an item, takes
one away, or moves one from the pack to the hand.

**A weapon's value can be put on a character.** A property a form lists is
declared in `properties` as any other, so `Ruleset.SheetKeyFault`
(`internal/rules/sheet.go`) reads it as any number property's key, and
`add_actor` with a ruleset served, `adventure.Load` and `vtt serve --ruleset`'s
`sheetsFit` (`cmd/vtt/serve_compose.go`) all accept a creature holding
`damage_dice` (SPEC-025's Consequences). `rulesets/dnd45e-minimal/guide.md`
tells the agent never to set one on a creature. `add_actor`'s description
(`tools/toolgen/main.go`) says an actor's numbers are each "under a key the
served ruleset declares as a number property", and `get_ruleset`'s
(`getRulesetDescription` in `internal/mcp/ruleset_tool.go`) that "A number
property's value is under an actor's `attributes`"; neither says otherwise for
a property a form lists.

**What a player and the DM see.** The player's panel (`renderPlayerPanel` in
`client/src/view/player.ts`) draws the sheet the ruleset lays out, section by
section, as `values`, `conditions` and `abilities`; no section shows what a
character carries. The DM console (`client/src/view/dm.ts`) gives and takes
abilities, and has nothing for items.

**What MapTool does.** MapTool's core has no item. The owner's own rule
framework for MapTool keeps a character's equipment in three token properties
of fixed name, `EquippedWeapon`, `EquippedImplement` and `EquippedOffhand`,
which `~/dev/RPTool/rule-framework/src/RuleEngine.ts` reads into an ability's
variables. Each is a JSON object holding the weapon's own data, its die, its
proficiency, its enhancement and its reach
(`~/dev/RPTool/Documentation/migration-guide.md`), a copy on every token that
carries it, and an attack reads the equipped weapon's numbers from it. Neither
the framework's documentation nor its sources name a list of what a character
carries. This ticket borrows the idea of a named place on the character that
holds the item an action will read; the reading is the third ticket's. It does
not borrow the copy (principle 27), nor place names fixed in the engine: here
the places are the ruleset's (principle 23).

**What ADR-012 says.** A form says what a sheet looks like as well as an item:
its properties, references, slots and sets (§3). A sheet refers to entries and
never copies them, and what it holds about a reference, its state, charges and
copies, is campaign data (§3, principle 27). A set has the form of its
entries, whether one may appear more than once, and the states an entry may be
in, one at a time (§2). Changing a set and changing an item, equipping it
among them, are effects (§9), which step 2 builds.

**What the owner ruled.** On 2026-11-24 the owner split ADR-012 step 1 item 5
into three tickets. This is the second: a sheet's references, slots and sets,
with the campaign's state per reference, which only the DM and the agent
change until step 2 makes equipping an effect. During the first, the owner
ruled that the platform reads and uses any form, key and value as data,
whatever the ruleset, so this ticket is tested with keys no shipped ruleset
uses and puts no ruleset's words in code. On 2026-11-25: one ticket, its tasks
committed separately. A place on a sheet has no limit on what it holds: what a
character can carry is normally weight against strength, which comes as the
ruleset's own sum once derived properties exist (step 1 item 6), and until
then the DM judges. The hole the first ticket left open closes as "an item's
value is refused": a value a catalogue form lists belongs to the item, and a
sheet that holds it is refused. On 2026-11-06: a player whose character sees a
companion sees that companion's whole sheet, and only the DM sees a creature's
that is not in the party. At the plan's sign-off, on 2026-11-25: no charges in
this ticket; and no player is told that their character gained or lost an
item, since "om en tjuv skulle stjäla något, utan att karaktären märker det
"pick pocket sucess" då kan vi inte produkten berätta att - du blev av med
ditt långsvärd".

**Not this ticket.** The actions a sheet reaches through its references, and
the abilities that bind a weapon's numbers today, which stay as they are (the
third ticket). A limit on what a place holds, weight against strength among
them (step 1 item 6). A player changing what their character carries, and
equipping as an action with a cost (step 2, ADR-012 §9). A set's condition on
its entries, its budget, and values per entry (ADR-012 §2), which need
expressions over a set. Charges, and an item destroyed at a bound (step 2;
the owner, 2026-11-25). Telling a player that their character gained or lost
an item (the owner, 2026-11-25).

## Done looks like

1. **A ruleset declares the places on a sheet.** A ruleset may declare
   places, each with a key, a display name, a description, the form of the
   entries it holds, whether it may hold an entry more than once, and the
   states an entry in it may be in. `rules.Load` keeps them and refuses,
   naming the file and the field: a place with no key, name or description;
   a key two places declare, or past the bound on an id (SPEC-018); a form the
   ruleset does not declare; a state that is empty, or that the place
   declares twice. An `internal/rules` test of each fails on today's tree,
   where the field is unknown.
2. **An item's value is refused on a sheet.** An actor whose `attributes`
   hold a key a catalogue form lists is refused, naming the key and the
   actor or its file, by `add_actor` with a ruleset served, by
   `adventure.Load`, and by `vtt serve --ruleset` for a campaign holding one.
   A test of each fails on today's tree, which accepts it.
3. **A sheet holds references.** An actor's sheet holds its references, each
   naming a catalogue entry, the place it is in and its state there, and two
   references to one entry are told apart. An
   `ActorAdded` carries them, both folds keep them (`engine.Apply` and
   `client/src/fold.ts`), and the contract's round trip holds them. A test of
   each fails on today's tree, where `vtt.v1.Actor` has no such field.
4. **A sheet's references fit its ruleset where it enters.** With a ruleset
   served, `add_actor`, `adventure.Load` and `vtt serve --ruleset` refuse a
   reference naming an entry the catalogue does not hold, a place the ruleset
   does not declare, an entry of a form the place does not hold, a state the
   place does not declare, and a second reference to one entry in a place
   that holds an entry once, naming the reference and the actor or its file.
   With no ruleset served, `add_actor` reads none of it against one. A test of
   each fails on today's tree.
5. **The DM and the agent change a sheet's references in play.** Commands add
   a reference to an actor's sheet, remove one, move one to another place,
   and change its state. Each writes an event both folds
   apply, and with a ruleset served each is read against it, as in item 4,
   before anything is written. A player's is refused (the owner,
   2026-11-24). A gateway test of each, and one through the MCP server, fail
   on today's tree.
6. **Whoever is sent a sheet is sent its references.** A character log is
   sent a party member's references, and their changes, as it is sent the
   sheet's other values (SPEC-016), and no player is sent those of a creature
   that is not a party member. A projection test of each fails on today's
   tree.
7. **The sheet and the console show them.** The player's panel shows the
   references of the character it follows, by their entries' names, where the
   ruleset's `sheet` lays them out, and the DM console adds, removes and moves
   them and changes their state. A client test of each fails on
   today's tree.
8. **The places are served, and the agent is told.** `/api/ruleset` and
   `get_ruleset` answer the places in the ruleset's words (SPEC-012), and
   `get_ruleset`'s description says what they are. `add_actor`'s description
   says what it refuses of a reference and that a value a form lists is
   refused on a sheet, and each new tool's description says what it changes
   and what it refuses (principle 34). A test of each fails on today's tree.
9. **dnd45e-minimal and its adventure use them.** `rulesets/dnd45e-minimal`
   declares places for its weapons and its armour, its `sheet` shows them,
   and the actors of `adventures/goblin-ambush` hold the weapons they fight
   with and their armour, their abilities unchanged; `guide.md` tells the
   agent how. The conformance suites, `internal/rules/conformance` and
   `internal/adventure/conformance`, load both, and the adventure's compiled
   golden names each actor's references, which it cannot on today's tree.
10. **The gate.** `task check` and `task e2e` are green.

## Rules this puts on the system

- A ruleset's places each have a key within the bound on an id, a display
  name and a description, hold entries of a form the ruleset declares, and
  name the states an entry may be in there; any other place is refused when
  the ruleset loads.
- A place holds any number of references (the owner, 2026-11-25).
- A sheet holds no value under a property a catalogue form lists; one that
  does is refused where it enters.
- A reference names an entry the catalogue holds, in a place the ruleset
  declares for that entry's form, in a state the place names; with a ruleset
  served, any other is refused where the sheet enters and where it changes in
  play.
- A place that holds an entry once refuses a second reference to it; a place
  that holds copies keeps each copy's state apart.
- Only the DM and the agent change a sheet's references.
- Whoever is sent a sheet is sent its references.
- `/api/ruleset` and `get_ruleset` answer the places, and the tools'
  descriptions say what they refuse.

## What it touches

In the order the change must reach them. The list crosses every component
from the format to the client; the owner chose one ticket for it, its tasks
committed separately (2026-11-25).

- The format: `internal/rules/format.go`, `internal/rules/load.go`,
  `internal/rules/sheet.go` (`SheetKeyFault`, and what reads a sheet's
  references), `internal/rules/schema/ruleset.schema.json` with
  `internal/rules/schema_test.go`, and `internal/rules/testdata/valid`.
- The contract: `contract/vtt/v1/events.proto` and
  `contract/vtt/v1/commands.proto`, regenerated by `task generate:contract`
  into `contract/gen` and `cmd/vtt/tools.json`, with
  `contract/roundtrip_test.go`, `contract/events.test.ts` and
  `contract/testdata`.
- The folds: `internal/engine/apply.go` and `client/src/fold.ts`, and the
  models property tests draw from, `internal/eventgen/model.go` and
  `internal/harness/engine.go`.
- Where a sheet enters: `internal/adventure/load.go`,
  `internal/adventure/format.go`, `internal/adventure/compile.go` and
  `internal/adventure/conformance/conformance.go`;
  `internal/gateway/add_actor_validate.go` and `internal/gateway/server.go`
  (`validateForm`); `cmd/vtt/serve_compose.go` (`sheetsFit`).
- In play: `internal/gateway/convert.go` (`ToEvent`),
  `internal/gateway/authz.go`, and handlers beside
  `internal/gateway/ability_change.go`.
- The projection: `internal/perceive/projection.go`.
- The read surface: `internal/gateway/metadata.go`,
  `internal/mcp/ruleset_tool.go` and `client/src/metadata.ts`.
- The tools: `tools/toolgen/main.go`.
- The client: `client/src/commands.ts`, `client/src/player.ts`
  (`sheetFor`), `client/src/view/dm.ts` and `client/src/view/player.ts`,
  rebuilt into `cmd/vtt/webdist`; the plan's G2 names the rest.
- The content: `rulesets/dnd45e-minimal/` (`ruleset.json`, `guide.md`),
  `adventures/goblin-ambush/actors/`, and the goldens their conformance
  suites compare.

## Specifications this moves

- docs/specifications/024-a-ruleset-declares-its-properties.md
- docs/specifications/025-a-sheets-values-fit-its-ruleset.md
- docs/specifications/027-a-rulesets-catalogue-holds-entries-of-its-forms.md
- docs/specifications/007-the-wire-contract.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/013-authorization.md
- docs/specifications/016-the-projection.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
- docs/specifications/021-each-party-member-keeps-its-own-log.md
- docs/specifications/022-a-client-holds-one-log.md
- New: a sheet's references to the catalogue, and how they change in play.

## What could not be established

- **One kind of place or two.** ADR-012 has slots, which may have a capacity,
  and sets, which have states and copies. With no limit on a place, what
  still tells a slot from a set is the plan's to propose.
- **Whether a place holds entries of one form or of several.** A hand may hold
  a weapon or a shield; whether a shield is its own form is the ruleset's,
  and whether a place may name more than one form is the plan's.
- **What tells two references apart.** Two copies of one entry in one place
  need an identity of their own, for a removal or a move to name; whether the
  DM gives it or the server does is the plan's.
- **What changes in play.** The owner named commands to add, remove and move
  a reference. That its state changes by a command too is this
  ticket's reading of "the campaign's state per reference"; how many commands
  carry it is the plan's.
- **Whether a move carries a state.** A reference moved from one place to
  another may keep its state, take the new place's first, or be given one.
- **ADR-012 §2's `ref` property.** One entry of a form on a sheet, such as a
  class, may be a place holding one entry or a property of its own; nothing in
  the shipped rulesets needs one yet.
- **What the panel shows of a reference.** Its entry's name, its state or its
  entry's values, and how the ruleset's `sheet` places them, is the plan's to
  propose.
- **The live campaign.** Whether the campaign the alpha test is served under
  holds an actor with a key a form lists, which `vtt serve --ruleset` would
  then refuse, is not established; its directory is outside the repository.
