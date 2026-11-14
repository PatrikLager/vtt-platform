# The DM gives a character an ability and takes one away, in play, and the player is told

## The problem

**A sheet's abilities are written once.** An actor's ability list is the
contract's `Actor.ability_ids` (`contract/vtt/v1/events.proto`), and the only
event that writes it is `ActorAdded`: `engine.Apply`'s `ActorAdded` arm
(`internal/engine/apply.go`) stores the actor the event carries, and no other
arm touches its list. `add_actor` and `load_adventure` are the commands that
produce an `ActorAdded`. The `ClientCommand` oneof
(`contract/vtt/v1/commands.proto`) changes an actor that already exists only
through its control (`grant_actor_control`, `revoke_actor_control`), its
conditions (`remove_condition`, and `use_ability`'s outcomes), its resources
(`use_ability`'s outcomes) and its removal (`remove_actor`). Nothing gives a
character an ability or takes one away.

**The DM's way today costs the character.** To change what a character can
do, the DM removes the actor and adds it again. `remove_actor` removes its
tokens with it, control ends with the actor, and an actor added again under
the same id is controlled by nobody, because `engine.Apply` refuses an
`ActorAdded` that names a controller (SPEC-017, SPEC-013). Its conditions
leave with it, and its resources begin again at whatever the new `add_actor`
says. The player is told "You no longer play …" (`onPartyMembersLeft` in
`client/src/app.ts`), loses the character from their window until the DM
grants it again, and the record says the character was removed rather than
that it learned or lost something.

**A changed list would read as another character.** The projection
(`internal/perceive/projection.go`) compares a held actor with the world's
through `sameIncarnation`, which compares the fields only an `ActorAdded`
writes, the ability ids among them (SPEC-016). `recall` marks an actor whose
held fields match neither its shown form nor its unseen form as gone, and
`Resume` starts over when a held actor in sight does not match. A list that
changed after the actor was added would be read the same way as an actor
removed and added again under the same id.

**A player's window would not follow a change.** A player reads, through
`/api/ruleset`, the abilities named by the sheets of the actors they control
(`abilitiesReadBy` in `internal/gateway/metadata.go`), and the panel offers the
window's list filtered by the acting actor's sheet (`offered` in
`client/src/player.ts`). The window reads the route again only when
`watchKey` (`client/src/app.ts`) changes: the watchable set, the
no-party-member flag, or the role. A change to a controlled actor's list
changes none of those, so the panel would not offer an ability given to the
player's own character, which the window's list does not hold, until the page
reloads.

**What MapTool does.** A token holds its own macros
(`Token.macroPropertiesMap`), and a macro is added to or removed from one token
at a time: `updateTokenProperty` with `Token.Update.saveMacro` or
`deleteMacro` (`ServerCommandClientImpl`), which the server relays to every
other client (`ServerMessageHandler`) and each client applies and redraws
(`ClientMessageHandler`, `Zone.tokenMacroChanged`, `SelectionPanel`). Who may do
it is checked on the client alone (`MacroButtonPopupMenu`, `MacroFunctions`),
every client receives every token's macros (`Token.toDto`) and hides those of
tokens the player does not own, and nothing records the change: no chat line,
no undo. This ticket borrows the granularity, one ability given or taken at a
time on one character, which is how a DM thinks of a level gained or a curse
laid. It does not borrow the rest. The server judges each change (principle 7),
each is a record that only grows (principles 2 and 3), a player is sent only
what their character's log is written (principles 16 and 17), and the sheet
names the ruleset's entry rather than holding a copy of it (principle 27).

**What the owner ruled.** Giving or taking an ability on a character already in
a campaign is its own ticket, right after "a sheet offers only what it names"
(the owner, 2026-11-13). The command, the MCP tool and the DM console's
controls are all in this ticket; the player is told both when an ability is
given and when one is taken; and it is one ticket, not split (the owner,
2026-11-14). Bringing campaign data up to date is the DM's (principle 26).

## Done looks like

1. **The DM gives an ability.** A command names an actor and an ability; the DM
   and the agent may send it. Afterwards the actor's sheet names the ability,
   `vtt state dump` shows it in the actor's `ability_ids`, and a `use_ability`
   of it by that actor resolves where it was refused before with
   `does not name ability`. A gateway test that gives and then uses fails on
   today's tree, which has no such command.
2. **The DM takes an ability.** A command names an actor and an ability its
   sheet names; afterwards the sheet does not name it, and a `use_ability` of it
   by that actor is refused with the sheet's refusal (SPEC-013), for the DM, the
   agent and a player alike.
3. **Nothing else about the character changes.** After a give or a take the
   actor's tokens, controllers, conditions, resources and every other field of
   its sheet are what they were: a test compares the state before and after,
   field by field, and only the ability list differs.
4. **What is refused, and appends nothing.**
   - a player's and a spectator's give or take (`not authorized`);
   - a give or take naming an actor the campaign does not hold;
   - with a ruleset served, a give of an ability the ruleset does not hold,
     naming the ability, as `add_actor` names one (SPEC-025);
   - a give of an ability the sheet already names, and a take of one it does
     not name.
   Each has a test that sends the command and finds the log's length
   unchanged. With no ruleset served a give is checked against nothing, as
   `add_actor`'s list is then (SPEC-025).
5. **The player's window follows.** In the window of a player who plays the
   party member, the panel offers a given ability and stops offering a taken one
   without a reload, and `/api/ruleset` answers that player the new set. A
   client test folds a give and a take into a player's window and reads the
   panel after each.
6. **The player is told.** The player who plays the party member is shown a
   message in their window when an ability is given to it and when one is taken
   from it, naming the character and the ability in the table's words
   (principle 13). A client test reads the message after each.
7. **A character log is told what its eyes see.** The log of the party member
   given or taken from, and the log of a party member whose eyes see it, are
   sent the change; a log whose eyes do not see it is sent nothing, and holds
   the current list once its eyes next see it; a creature's change reaches no
   player's log, a player who controls the creature included, since a creature
   is shown without its list (SPEC-016) and a player's panel never acts as one. No log
   is sent a removal, an addition or a withdrawal for a give or a take, and a
   log resumed after one resumes rather than starting over. Projection tests
   in `internal/perceive` hold each case.
8. **The client's fold agrees with the server's.** The client folds the new
   events to the same state `vtt state dump` prints, held by the fold-parity
   goldens (`client/test/fold-parity.test.ts`, `scenarios/goldens`).
9. **The agent can do it.** `vtt mcp` lists a tool for each command, with a
   description that says who may send it; `contract/testdata/expected_tools.json`
   and the generated `contract/gen/tools/tools.json` hold them.
10. **The DM console can do it.** The DM's console offers, for an actor, a give
    of an ability the ruleset holds that its sheet does not name and a take of
    one it names, and sends the commands; a client test drives both.
11. **The gate.** `task check` is green.

## Rules this puts on the system

- Only the DM and the agent may give or take an ability.
- A give adds one ability to one actor's sheet, and a take removes one; nothing
  else about the actor changes.
- With a ruleset served, a give of an ability the ruleset does not hold is
  refused.
- A give of an ability the sheet already names is refused, and so is a take of
  one it does not name.
- A give or a take naming an actor the campaign does not hold is refused.
- A player's panel offers a given ability, and stops offering a taken one,
  without a reload.
- The player who plays the party member is told of each give and each take.
- A character log is sent a give or a take only when its eyes see the actor,
  and a creature's never.
- A character log never reads a give or a take as the actor's removal or its
  addition.

## What it touches

In the order the change must reach them:

- The contract: `contract/vtt/v1/commands.proto` (`ClientCommand`) and
  `contract/vtt/v1/events.proto` (`Envelope`), additively (ADR-007), and what
  `task generate:contract` regenerates under `contract/gen`.
- The fold: `internal/engine/apply.go` (`Apply`), the one place game state
  changes.
- The ruleset's check: `internal/rules/sheet.go` (`SheetAbilityFault`,
  `Offers`), or a check beside it.
- The gateway: `internal/gateway/authz.go` (`commandName`, `commandRoles`,
  `playerRules`), `internal/gateway/convert.go`, the command path in
  `internal/gateway/server.go` (`handleCommand`) and a validator beside
  `internal/gateway/add_actor_validate.go`.
- The projection: `internal/perceive/projection.go` (`classify`,
  `sameIncarnation`, `recall`, `Resume`, and the frames sent at a sight).
- The agent's tools: `tools/toolgen/main.go`, with the generated
  `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`, and
  `contract/testdata/expected_tools.json`.
- The client: `client/src/state.ts` and `client/src/fold.ts` (the new arms);
  `client/src/commands.ts`; `client/src/app.ts` (`watchKey`, `readRuleset` and
  the message); `client/src/session.ts` if the message comes from it;
  `client/src/view/dm.ts` (the console's controls);
  `client/src/view/spectator.ts` if the Story draws a line for it; and the
  rebuilt `cmd/vtt/webdist`.
- The goldens: `scenarios/goldens` and the scenarios that feed
  `client/test/fold-parity.test.ts`.

This crosses more than two components and names more than ten files. The owner
ruled on 2026-11-14 that it stays one ticket.

## Specifications this moves

- docs/specifications/007-the-wire-contract.md
- docs/specifications/013-authorization.md
- docs/specifications/016-the-projection.md
- docs/specifications/021-each-party-member-keeps-its-own-log.md
- docs/specifications/022-a-client-holds-one-log.md
- docs/specifications/025-a-sheets-values-fit-its-ruleset.md
- docs/specifications/012-the-read-surface.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md

## What could not be established

- **Whether the Story shows a give or a take.** The owner ruled that the player
  is told. Whether the Story also draws a line for it, in the player's window,
  in the DM's, or in both, is not ruled; the plan proposes and the sign-off
  decides.
- **How the message names a taken ability.** After a take, the player's read of
  `/api/ruleset` no longer holds the ability, and the Story names an ability its
  window's list lacks by its id (SPEC-022). Whether the message is written from
  the list the window held before the take, or names the ability some other
  way, is the plan's.
- **The alpha campaign.** The actors of `two-rooms` in `~/dev/vtt-alpha`,
  outside the repository, name no ability. With this ticket the DM, the agent
  included, can give them their abilities in a running campaign. Whether that
  is how that campaign is brought up to date, or its files are changed before it
  seeds another, is the owner's.
- **One event per ability or the whole list.** Whether a give and a take are two
  events naming one ability each, or one event carrying the sheet's new list, is
  the plan's; the rules above hold either way.
