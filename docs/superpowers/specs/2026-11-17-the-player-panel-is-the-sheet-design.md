# A player's panel is their character's sheet, laid out as its ruleset declares

## The problem

**The panel holds no sheet.** A player's window draws the panel
`renderPlayerPanel` (`client/src/view/player.ts`) builds: the heading "Your
turn", a chip for each party member the player may switch to, a button for
each ability the acting actor's sheet names (`offered` in
`client/src/player.ts`), target buttons for an armed ability, the arm-doors
toggle and the Say form. An ability button's title names the resource it
spends and its cost, or "at will"; it never says how much of that resource is
left. The panel shows no attribute, no resource value and no condition.

**A value is seen only on the board.** `tokensOnScene`
(`client/src/view/grid.ts`) gives each disc every resource its actor holds,
and `renderDisc` (`client/src/view/spectator.ts`) draws each as a chip,
"current/max", in every role's window. A character with no token on the
board shows its values nowhere. A condition is drawn as a dot whose title is
the condition's id. No window shows an attribute.

**The client never reads a condition's words.** `handleRuleset`
(`internal/gateway/metadata.go`) serves every role the ruleset's conditions,
each with its name and description; `RulesetMeta` (`client/src/metadata.ts`)
declares them, and `client/src/app.ts` keeps only the abilities and the
properties.

**The ruleset says nothing about a sheet's layout.** A ruleset's
`ruleset.json` holds `id`, `name`, `format_version` and `properties`, and
`loadManifest` (`internal/rules/load.go`) decodes it with `decodeStrict`,
which refuses any other field. Nothing a ruleset can say groups its values,
orders them for a sheet, or splits them into pages; `tokensOnScene`'s own
comment notes that the format has no hints for a client.

**The owner's rulings of 2026-11-17.** The player's panel IS the character
sheet. How it is laid out, which pages it has and which values each shows, is
declared by the ruleset, and the client draws what is declared and invents no
grouping of its own: a ruleset like D&D 4e has many things and can need
several pages, a small one needs one. A ruleset that declares no layout is
drawn as one page. A player's and a spectator's board draws no numbers on a
disc; the DM's and the agent's keep them, and the condition dots stay for
every role. One ticket carries the format, the read surface and the client.

**What this is not.** An ability's description: the ability format carries
none (`compositionJSON` in `internal/rules/load.go`), and giving it one is the
next ticket, by the owner's ruling of 2026-11-17. A view of a companion's
sheet from the board (the owner's ruling of 2026-11-06, a right-click on a
token): "kanske senare", the owner's ruling of 2026-11-16. A layout a campaign
declares: the ruleset alone declares it. Changing a value from the sheet.

**What MapTool does.** Its core has no character sheet. A token's properties
reach a player on the hover stat sheet (`StatSheetListener`, `StatSheet`),
filtered per property by "show on stat sheet", GM-only and owner-only flags
(`TokenProperty`). The Selected and Impersonate panels
(`SelectionPanel`, `ImpersonatePanel`) show macro buttons and no values. A
framework builds a sheet as an HTML frame a macro opens (`html.frame5`), docked
and redrawn when the selected or impersonated token changes. The owner's own
4.5e framework under `~/dev/RPTool/rule-framework` has none built;
`~/dev/RPTool/Documentation/overview.md`, which documents that framework,
plans a power panel of power cards and active
conditions. The owner's paper sheet for play
(`~/dev/RPTool/Ursus_250907.pptx`, slide 10)
groups powers by how they are used, each with a box to mark a use, beside the
hit points and the current conditions. Borrowed: a panel that is always shown
and follows the character played, abilities grouped as the game groups them,
and a value's explanation on hover. Not borrowed: per-property flags, since
sight decides what a window holds (the owner's ruling of 2026-11-06).

## Done looks like

1. **A ruleset declares its sheet.** `rulesets/dnd45e-minimal/ruleset.json`
   declares a sheet of more than one named page, each of named sections, each
   naming what it shows; `rules.Load` accepts it and returns the pages,
   sections and what each shows in the order declared. A Go test that fails
   on today's tree, where `decodeStrict` refuses the field.
2. **A sheet that does not fit its ruleset is refused.** `rules.Load` refuses,
   naming the file and the field, a sheet whose section names a property the
   ruleset does not declare, and a page or section with an empty name.
   `internal/rules/schema/ruleset.schema.json` describes the field and
   `internal/rules/schema_test.go` holds the schema to the loader. Go tests.
3. **The read surface serves it.** `/api/ruleset` carries the declared sheet
   to every role, and carries none for a ruleset declaring none; `get_ruleset`
   carries it too, and its description says so. Go tests that fail on today's
   tree.
4. **The panel draws the pages.** A player's window whose ruleset declares
   two pages draws one tab per page, named as declared and in that order,
   shows the first page, and shows the second after a click on its tab. A
   client test that fails on today's tree.
5. **A section shows its values in the ruleset's words.** A number property
   is drawn as its name and value, a resource property as its name and
   current/max, each with the property's description on hover; a value the
   actor's sheet does not hold is left out. A client test.
6. **Conditions are named.** A section that shows conditions lists the
   actor's conditions in the order they were applied, by the name the ruleset
   gives each, with its description on hover, and by its id when the ruleset
   declares no such condition. A client test.
7. **Abilities stay where play happens.** A section that shows abilities draws
   the buttons the panel offers today (VTT-695), grouped as the ruleset
   declares; arming one and choosing a target work as today. A button for an
   ability that spends a resource names on hover the resource, its current
   and max, and the cost. Client tests that fail on today's tree.
8. **A ruleset that declares no sheet is drawn as one page.** With
   `rulesets/tavern-brawl/ruleset.json`, which declares none, and with no
   ruleset at all, the panel draws one page holding every value the actor's
   sheet holds, its conditions and its abilities, and no tab. A client test.
9. **The panel names its character.** The panel's heading is the name of the
   character it shows. A client test that fails on today's tree.
10. **No numbers on a player's discs.** A player's and a spectator's window
    draw no resource chip on any disc; a DM's and an agent's window draw them
    as today; every window draws the condition dots. A client test that fails
    on today's tree.
11. **What stays.** The party-member chips, the target buttons, the arm-doors
    toggle and the Say form behave as today; their tests stay green.
12. **The gate.** `task check` is green, the embedded bundle under
    `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
    green.

## Rules this puts on the system

- A ruleset may declare its sheet as named pages of named sections, each
  naming what it shows.
- A ruleset whose sheet names a property it does not declare is refused when
  it loads.
- The read surface serves a ruleset's declared sheet to every role.
- The player's panel draws the pages and sections the window's ruleset
  declares and no grouping of its own; with none declared, one page.
- The panel names each value, condition and ability by the ruleset's words
  and offers each property's and condition's explanation.
- An ability button that spends a resource tells the resource's current and
  max and the cost.
- The panel is headed by the name of the character it shows.
- A player's and a spectator's window draw no resource chip on a disc; a
  DM's and an agent's draw one per resource; every window draws the
  condition dots.
- `get_ruleset`'s description says it answers the declared sheet.
- The panel draws its layout and condition names from the window's newest
  `/api/ruleset` answer, as it does its abilities (VTT-753).

A ruleset's manifest that may hold `sheet` makes VTT-627, which refuses any
field but `id`, `name`, `format_version` and `properties`, false as worded;
it is withdrawn and succeeded.

## What it touches

In this order, each step depending on the one before:

1. **The format.** `internal/rules/load.go` (`manifestJSON`, `loadManifest`),
   `internal/rules/format.go` (`Ruleset`),
   `internal/rules/schema/ruleset.schema.json`,
   `internal/rules/schema_test.go` and its fixture
   `internal/rules/testdata/valid/ruleset.json`, a new test file in
   `internal/rules`, and `rulesets/dnd45e-minimal/ruleset.json`.
2. **The read surface.** `internal/gateway/metadata.go` (`rulesetJSON`,
   `handleRuleset`) and `internal/gateway/metadata_test.go`;
   `internal/mcp/ruleset_tool.go` (`getRulesetDescription`),
   `internal/mcp/ruleset_tool_test.go`, `cmd/vtt/qa_ruleset_by_role_test.go`
   and `cmd/vtt/qa_get_ruleset_test.go`. `get_ruleset` is registered in Go
   and is not in `contract/gen/tools/tools.json`.
3. **The client.** `client/src/metadata.ts` (`RulesetMeta`),
   `client/src/app.ts` (what it keeps of `/api/ruleset`),
   `client/src/view/player.ts` (`renderPlayerPanel`), `client/src/player.ts`,
   `client/src/view/grid.ts` (`tokensOnScene`), `client/src/view/spectator.ts`
   (`renderDisc`), `client/src/style.css`, the client tests that draw the
   panel or a disc's chips (the plan names them), `client/e2e/table.spec.ts`
   and `cmd/vtt/webdist`.
4. `docs/requirements.md`: VTT-627, withdrawn and succeeded; the rows whose
   tests read a chip or the button's title in a player's window (VTT-809,
   VTT-815, VTT-816, VTT-817), their evidence moved. No row says a disc
   draws its chips in every window; SPEC-022 and the comments in
   `tokensOnScene` and `renderDisc` do.

This crosses the rules engine, the gateway, the MCP server and the client.
The owner ruled on 2026-11-17 that it is one ticket.

## Specifications this moves

- docs/specifications/024-a-ruleset-declares-its-properties.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/022-a-client-holds-one-log.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- **The shape of the declaration.** How a section names conditions and
  abilities apart from a property key, how abilities are grouped (by how they
  are used, by the resource they spend, or by naming them), whether one value
  may stand in two sections, and whether `format_version` stays `"3"` are the
  plan's to propose and the owner's to sign off.
- **A condition's words elsewhere.** A disc's dot is titled by the
  condition's id (`tokensOnScene`), and the Story and the Events panel word a
  condition gained or lost by its id (`describe` in
  `client/src/view/spectator.ts`), against blueprint principle 13. The sheet
  names it; the others are not this ticket's, and are raised to the owner at
  sign-off.
- **A companion's values.** Once a player's discs draw no chips, a player
  sees a seen companion's values nowhere: the panel shows the characters the
  player plays. The owner accepted this on 2026-11-17; a view of a
  companion's sheet is later work.
- **The DM's view of a sheet.** The DM's and the agent's windows draw this
  panel only for an actor they control, as today; no view shows them an
  uncontrolled actor's sheet beyond its disc and the console.
