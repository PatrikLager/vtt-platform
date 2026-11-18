# An ability may carry the ruleset's description of it, and the sheet shows it

## The problem

**An ability carries no explanation.** A ruleset's `abilities/*.json` holds
an ability's `id`, `name`, `usage` and `compose`; `loadCompositions`
(`internal/rules/load.go`) decodes it into `compositionJSON` with the strict
decode, which refuses any other field, and `compilePower`
(`internal/rules/compile.go`) gives `rules.CompiledPower` its id and name.
A property carries a description its loader requires (`loadProperty`), and a
condition one its loader keeps; an ability has none to carry.

**Nothing serves one.** `handleRuleset` (`internal/gateway/metadata.go`)
answers each ability as `abilityJSON`: its `id`, `name`, `range`,
`maxTargets` and `usage`. `get_ruleset` (`internal/mcp/ruleset_tool.go`)
returns that body, and its description names the abilities' range,
targets and usage. How an ability resolves is not served (SPEC-012).

**The sheet cannot say what a power does.** The player panel's ability
button (`renderPlayerPanel`, `costTitle` in `client/src/view/player.ts`) is
labelled by the ability's name and titled by its cost, or "at will". A
player choosing between their character's abilities is told what each costs
and not what it does. Blueprint principle 35 says every ability can be read
"with the ruleset's own explanation"; today none can.

**The owner's rulings of 2026-11-18.** An ability may carry a description;
the loader does not require one. On the sheet, the description is shown on
hover of the ability's button, above its cost. What a description says is
its author's, and nothing checks its content; in the rulesets this
repository ships, each ability's description says what it does in words,
with no dice figures, so the text holds whatever the composition's numbers
become. At the plan's sign-off: the button's "at will" is a D&D 4e word in
the platform's own text, so the word for an ability that spends no
resource is the ruleset's: `ruleset.json` may name it, and the platform
writes none of its own.

**What this costs.** Principle 35 is met for an ability whose author wrote
a description, and not for one whose author did not. A property's
description stays required, and a condition's stays optional.

**What this is not.** The format's own keyword for an ability that
spends no resource, `at_will` in an ability's file and a sheet's
`abilities`, and `atWill` on the wire: the question the owner answered
said the ruling concerns the word a player sees. A description on any
other part of a ruleset. A
condition named by its id on a disc's dot and in the Story (G7 of
`docs/superpowers/plans/2026-11-17-the-player-panel-is-the-sheet.md`): the
next ticket. Serving how an ability resolves. The DM
console's give and take of an ability. The Story's line of an ability's
use.

**What MapTool does.** A macro button carries a `toolTip`
(`MacroButtonProperties`), the text shown when the pointer rests on it, set
by whoever writes the macro. The owner's 4.5e framework under
`~/dev/RPTool/rule-framework` gives an ability an optional `flavorText`
(`schema/ability.schema.ts`), which its power card prints in the chat after
a use. Borrowed: the author's text, optional, on hover of the button that
uses the ability. Not borrowed: printing it to the chat at each use, since
the Story writes what happens and not what an ability is.

## Done looks like

1. **An ability declares a description, and a ruleset names an ability
   that spends no resource.** An `abilities/*.json` holding `description`
   loads, and `rules.Load` returns it with the ability; one without loads
   as today, with none. A `ruleset.json` holding `free_usage_name` loads,
   and `rules.Load` returns it; one without loads as today. Go tests that
   fail on today's tree, where the strict decode refuses both fields.
2. **The format says so.** `internal/rules/schema/ability.schema.json`
   describes `description` as optional and `ruleset.schema.json`
   `free_usage_name`, and `internal/rules/schema_test.go` holds the schemas
   to fixtures that declare them. Go tests that fail on today's tree.
3. **The shipped rulesets describe their abilities.** Every ability under
   `rulesets/dnd45e-minimal/abilities` and `rulesets/tavern-brawl/abilities`
   declares a description in words, with no dice figure;
   `rulesets/dnd45e-minimal/ruleset.json` names an ability that spends no
   resource "At will" and `rulesets/tavern-brawl/ruleset.json` names none;
   both rulesets still load and pass their conformance goldens.
4. **The read surface serves it.** `/api/ruleset` answers each ability it
   serves with its description, to whoever it serves that ability, and with
   none for an ability that declares none, and the ruleset's name for an
   ability that spends no resource, to every role, and none when it names
   none; `get_ruleset`'s description says both. Go tests that fail on
   today's tree.
5. **The sheet shows it on hover.** On the player panel, an ability's button
   is titled by its description and, below it, its cost line: a resource
   ability's cost as today, and for an ability that spends no resource the
   word its ruleset gives one, or none; an ability with no description is
   titled by its cost line alone. No window titles a button "at will"
   unless its ruleset names it so. A client test that fails on today's
   tree.
6. **What stays.** The button's label, a resource ability's cost line's
   words (VTT-844) and which abilities a role reads (VTT-691 to VTT-693)
   are unchanged; their tests stay green, but for the at-will assertions
   that item 5 changes.
7. **The gate.** `task check` is green, the embedded bundle under
   `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
   green.

## Rules this puts on the system

- An ability may declare a description, which `rules.Load` keeps with it;
  an ability that declares none has none, and an empty one is refused.
- A ruleset may name an ability that spends no resource, which `rules.Load`
  keeps; an empty name is refused, and `ruleset.schema.json` describes it.
- `ability.schema.json` describes `description` as optional.
- Every ability of a ruleset this repository ships declares a description
  with no dice figure.
- The read surface serves an ability's description with the ability, to
  whoever it serves the ability, and none for an ability that has none.
- `get_ruleset`'s description says an ability carries its description,
  and that the ruleset may name an ability that spends no resource.
- A ruleset may name an ability that spends no resource, and the read
  surface serves that name to every role.
- An ability button's title is the ability's description, then its cost
  line; with no description, the cost line alone; the cost line of an
  ability that spends no resource is the name its ruleset gives one, or
  none.

## What it touches

In this order, each step depending on the one before:

1. **The format.** `internal/rules/load.go` (`compositionJSON`,
   `loadCompositions`), `internal/rules/compile.go` (`compilePower`),
   `internal/rules/format.go` (`CompiledPower`),
   `internal/rules/schema/ability.schema.json`, `internal/rules/schema_test.go`
   and its fixture under `internal/rules/testdata/valid/abilities`, a new
   test file in `internal/rules`, and
   `internal/rules/conformance/conformance_test.go`, which holds every
   shipped ability to a description; `loadManifest`, `ruleset.schema.json` and
   `rulesets/dnd45e-minimal/ruleset.json` for the name of an ability that
   spends no resource; the abilities under
   `rulesets/dnd45e-minimal/abilities` and `rulesets/tavern-brawl/abilities`.
2. **The read surface.** `internal/gateway/metadata.go` (`abilityJSON`,
   `rulesetJSON`, `handleRuleset`) and its tests; `internal/mcp/ruleset_tool.go`
   (`getRulesetDescription`) and `internal/mcp/ruleset_tool_test.go`;
   `cmd/vtt/qa_get_ruleset_test.go`.
3. **The client.** `client/src/metadata.ts` (`Ability`, `RulesetMeta`),
   `client/src/app.ts` (the window's `layout`),
   `client/src/view/player.ts` (`costTitle`), the client tests that read an
   ability button's title (`client/test/panel-sheet.test.ts` and
   `client/test/app.test.ts` among them), `client/e2e/table.spec.ts`, and
   `cmd/vtt/webdist`.
4. `docs/requirements.md`: the rows whose words change, as the plan
   decides.

This crosses the rules engine, the gateway, the MCP server and the client,
as the sheet's ticket did; the owner ruled that one on 2026-11-17 to be one
ticket, and this one is smaller.

## Specifications this moves

- docs/specifications/024-a-ruleset-declares-its-properties.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **Where the description lives once loaded,** and **an empty
  description**: answered at the plan's sign-off (its OD-1 and OD-2).
- **A long description in a tooltip.** A browser draws a `title` as the
  platform does, and some platforms do not show one on touch; the sheet's
  values and conditions are shown the same way today.
- **SPEC-024's subject.** It records the manifest and the properties, and
  is the only specification that names how an ability's file is loaded; no
  specification records the ability format as such.
