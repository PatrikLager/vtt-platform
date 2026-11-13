# A character is offered only the abilities its sheet names, and a sheet names only abilities its ruleset holds

## The problem

**A sheet names no ability.** An actor's sheet is the contract's `Actor`
(`contract/vtt/v1/events.proto`): its id, name, module id, `attributes`,
`resources`, `module_data`, controllers and kind. Nothing in it says which of
the ruleset's abilities the character has. An adventure's actor file is
decoded with `decodeStrict` (`loadActors` in `internal/adventure/load.go`),
which refuses a field the file format does not know, so an actor file cannot
name an ability either.

**Any actor may use any ability.** `rules.Resolve`
(`internal/rules/resolve.go`) looks the command's ability up in the ruleset's
`Compiled` abilities and its actor in the state's `Actors`, and refuses an
unknown ability or an unknown actor; nothing ties the one to the other.
`handleUseAbility` (`internal/gateway/ruleset.go`) calls it once a ruleset is
served, after authorization, which lets the DM and the agent send it for any
actor and a player for an actor they control (`authorizeActorOwnership` in
`internal/gateway/authz.go`'s `playerRules`). The MCP
tool `use_ability` takes an actor id, an ability id and target ids, and
nothing else. The agent, which has no panel, can make any actor use any
ability, and the use resolves.

**The panel offers every ability to every character.** `renderPlayerPanel`
(`client/src/view/player.ts`) lists every ability of `/api/ruleset` for
whichever actor is selected, for a player and for the DM alike, so the
fighter of `adventures/goblin-ambush` is offered `goblin-scimitar`.

**Every role reads every ability.** `handleRuleset`
(`internal/gateway/metadata.go`) answers every authenticated participant
the same: every ability in the ruleset's `Compiled`. The MCP tool
`get_ruleset` reads that route with the MCP server's own token, and its
description (`internal/mcp/ruleset_tool.go`) says it reads the ruleset "as
every participant can". The client fetches the route once, after `/api/me`
(`client/src/app.ts`), and never again.

**What the owner ruled.** A character has only the powers and skills it has
itself, and those alone show in its window (the owner, 2026-11-05). A
ruleset holds every ability; a sheet names the ones its character has; a
sheet naming an ability its ruleset does not hold is refused, and the use of
an ability a character's sheet does not name is refused for every role (the
owner, 2026-11-13). A player reads only the abilities the sheets of the
characters they control name; the DM and the agent read all (the owner,
2026-11-13). Giving or taking an ability on a character already in a
campaign is a ticket of its own, right after this one (the owner,
2026-11-13).

MapTool keeps each token's macros on the token
(`Token.macroPropertiesMap`), and the impersonated token's panel
(`ImpersonatePanel`) shows that token's macros alone. Each macro is the
token's own copy of the macro text (`MacroButtonProperties.command`), which
is what principle 27 forbids: a correction reaches no token that already
holds a copy. This ticket borrows the panel, a character's own list and
nothing else, and the sheet names the ruleset's entry rather than copying
it.

## Done looks like

1. **A sheet names its abilities.** An actor added through `add_actor`,
   through the console's paste or the MCP tool, and an actor in an
   adventure's actor file, names the abilities it has by their ids, and the
   actor the fold holds names the same ones. New tests in
   `internal/adventure` and `internal/gateway`; on today's tree an actor file
   naming an ability is refused as an unknown field, and an `add_actor` has
   nowhere to name one.
2. **A sheet naming an ability its ruleset does not hold is refused.**
   `adventure.Load` refuses such an actor, naming the file and the ability;
   `add_actor`, with a ruleset served, refuses it, naming the actor and the
   ability; `vtt serve --ruleset` refuses to start a campaign any of whose
   actors names one, naming each actor and the ability, and leaves nothing
   open. New tests in `internal/adventure`, `internal/gateway` and
   `cmd/vtt`. With no ruleset served, `add_actor` and `vtt serve` read no
   ability against one, as they read no key (SPEC-025).
3. **The use of an ability the caster's sheet does not name is refused.**
   A `use_ability` whose actor's sheet does not name the ability is refused
   for the DM, the agent and a player alike, naming the character and the
   ability, and appends nothing. New tests in `internal/gateway` and through
   the MCP tool; on today's tree each use resolves. A use the sheet names
   resolves as today.
4. **The panel offers a character only what its sheet names.**
   `renderPlayerPanel` lists, for the selected actor, the abilities its sheet
   names and no other, for a player and for the DM. A new test in
   `client/test`; on today's tree the panel lists every ability of the
   ruleset.
5. **A player reads only their characters' abilities.** `/api/ruleset`
   answers a player with the abilities named by the sheets of the actors
   they control, and no other; a spectator, and a player controlling no
   actor, with none; the DM and the agent with every ability. Conditions and
   properties are answered to every role as today. `get_ruleset`'s
   description says what it reads. New tests in `internal/gateway` and
   `internal/mcp`; on today's tree a player reads every ability.
6. **A change of control reaches the panel without a reload.** A player
   granted control of a character mid-session is offered that character's
   abilities, and one whose control is taken back no longer reads them,
   without reloading the page. A new test in `client/test`; on today's tree
   the ability list is the one fetched at start.
7. **A log holds no ability list it may not.** A character log holds a
   creature, and a party member its eyes have not seen, without the
   abilities its sheet names, as it holds it without its attributes and
   resources (VTT-465, VTT-658), and a party member its eyes see with them.
   New tests in `internal/perceive`. On today's tree there is no list to
   hold, and `shownWith` builds the actor it sends a log from the fields it
   names, so the tests are guards: what shows they can fail is a deliberate
   break that copies the list into `shownWith`'s actor.
8. **The repository's data names the abilities its actors use.** Every
   actor in `adventures/` names abilities its ruleset holds, the ones the
   plan gives each actor and the owner approves at sign-off, and the
   scenarios, goldens and end-to-end tests that use an ability pass with the
   actors they use naming it.
9. `task check` exits 0 on the branch, and `task e2e` passes.

Still refused afterwards: everything SPEC-025 refuses in a sheet, a use with
no ruleset served (`gateway: no ruleset loaded`), and every refusal
`rules.Resolve` and the player's sight gate (SPEC-013) make today. Still
accepted: a sheet that names no ability, whose character is then offered
none.

## Rules this puts on the system

- With a ruleset served, `add_actor` refuses an actor whose sheet names an
  ability the ruleset does not hold.
- `adventure.Load` refuses an actor whose sheet names an ability the ruleset
  does not hold.
- `vtt serve --ruleset` refuses to start a campaign any of whose actors
  names an ability the ruleset does not hold, naming each actor and the
  ability.
- A `use_ability` whose actor's sheet does not name the ability is refused,
  for every role, and appends nothing.
- `/api/ruleset` answers a player only the abilities the sheets of the actors
  they control name, a spectator none, and the DM and the agent all.
- The panel offers a character only the abilities its sheet names.
- A character log holds a creature, and a party member its eyes have not
  seen, without the abilities its sheet names.

## What it touches

In the order the change must reach them:

- The contract: `contract/vtt/v1/events.proto` (`Actor`), additively
  (ADR-007), and what `task generate:contract` regenerates under
  `contract/gen`.
- The ruleset and the sheet: `internal/rules/sheet.go` (`SheetFault`,
  `Ruleset.SheetKeyFault`) or a check beside it, and
  `internal/rules/resolve.go` (`Resolve`) or
  `internal/gateway/ruleset.go` (`handleUseAbility`), wherever the plan puts
  the use's refusal.
- Where a sheet enters: `internal/adventure/load.go` (`actorJSON`,
  `loadActors`), `internal/gateway/add_actor_validate.go`
  (`validateAddActorWith`), `cmd/vtt/serve_compose.go` (`sheetsFit`).
- What a role reads: `internal/gateway/metadata.go` (`handleRuleset`),
  `internal/mcp/ruleset_tool.go` (`getRulesetDescription`), and
  `tools/toolgen/main.go` for `add_actor`'s and `use_ability`'s docs, with
  the generated `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`.
- The projection: `internal/perceive/projection.go` (`shownWith`, read for
  item 7; `sameIncarnation`, which compares the fields only an
  `ActorAdded` writes, SPEC-016).
- The client: `client/src/state.ts` (`Actor`) and `client/src/fold.ts`
  (`copyActor`, `foldToDumpJSON`), since the client's fold is held equal to
  `vtt state dump`; `client/src/player.ts`; `client/src/view/player.ts`
  (`renderPlayerPanel`), `client/src/app.ts` (the ruleset fetch),
  `client/src/metadata.ts`, and the rebuilt `cmd/vtt/webdist`.
- The data and its records: `adventures/*/actors/*.json` and
  `adventures/*/goldens/compiled-batch.json`; `rulesets/dnd45e-minimal/guide.md`,
  whose reference statblocks say an actor has no `abilities` field and that
  nothing binds an ability to an actor; `contract/testdata/expected_tools.json`
  and `contract/roundtrip_test.go`; and every test, golden and scenario
  whose actor uses an ability. `git grep -l -E
  'UseAbility\{|use_ability|useAbility\('` names the Go and TypeScript
  files that send one; `git grep -l -E '"useAbility"|use-ability' --
  scenarios client/e2e` names the scenarios and the Playwright suite it
  misses, and `scenarios/goldens` holds the goldens of those scenarios that
  have one.

This crosses more than two components and names more than ten files;
whether to split it is asked at the plan's sign-off.

## Specifications this moves

- docs/specifications/025-a-sheets-values-fit-its-ruleset.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/013-authorization.md
- docs/specifications/016-the-projection.md
- docs/specifications/022-a-client-holds-one-log.md
- docs/specifications/007-the-wire-contract.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- **What a player's Story calls an ability they cannot read.**
  `abilityLine` (`client/src/view/spectator.ts`) names an ability by the
  name `/api/ruleset` gave the window, and by its id when it gave none. Once
  a player reads only their characters' abilities, a creature's or a
  companion's ability that a player sees used is drawn by its id. Whether
  that stands, or the name reaches the Story some other way, is a decision
  for the plan's sign-off.
- **The ruleset's guide.** `/api/ruleset/guide` is answered to every role
  (SPEC-012), and `rulesets/dnd45e-minimal/guide.md` names the goblins'
  abilities. Whether the guide stays open to a player is not settled by
  the owner's ruling, which named `/api/ruleset` and `get_ruleset`.
- **A name twice on one sheet**, and whether the fold bounds the new list
  (SPEC-018), are the plan's.
- **Where the use's refusal runs**, relative to the player's sight gate
  (SPEC-013), which answers an unseen target before anything else the
  command carries, is the plan's.
- **Campaigns already played.** An actor added before this ticket names no
  ability, and is offered none. Principle 26 makes bringing it up to date
  the DM's, and until the next ticket gives a sheet's abilities a command,
  the DM's way is to remove the actor and add it again (`remove_actor`,
  SPEC-017). The alpha campaign's adventure, `two-rooms` in
  `~/dev/vtt-alpha`, outside the repository, plays under
  `dnd45e-minimal` and its actor files name no ability: every use in a
  campaign it seeded is refused until its actors name abilities, and its
  files need the list before it seeds another. When is the owner's.
- Left out on the owner's word of 2026-11-13: giving or taking an ability
  on a character already in a campaign (the next ticket), the console's
  choice of abilities when adding an actor, the sheet view (right-click,
  "Show character sheet"), and the log naming its ruleset and version
  (step 1's item 3, which this ticket precedes by the owner's choice).
