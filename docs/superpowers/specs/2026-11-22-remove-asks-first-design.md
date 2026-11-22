# The console's Remove asks the DM first

## The problem

**Remove sends at once.** The DM console's "Remove" button
(`client/src/view/dm.ts`, action `remove-actor`) sends `remove_actor` for the
actor id typed in the actor field, with no question asked. `handleRemoveActor`
(`internal/gateway/server.go`) appends one batch that takes every token of the
actor off the board and then the actor itself. Nothing undoes it: the log only
goes forward, and an actor added again, even under the same id, is controlled
by nobody and has none of its old tokens (SPEC-017).

**The people it reaches are barely told.** Only the DM and the agent are sent
the removal (SPEC-017, Consequences). A player whose character saw one of the
actor's tokens is sent only that the token went out of sight
(`internal/perceive/projection.go`), which the Story does not draw
(`client/src/view/feed.ts`); a player whose own character is removed has that
character's log paused and, if connected, is shown `You no longer play
<name>` (VTT-605); one who is not connected is told nothing and comes back
watching another party member or nobody (SPEC-015, SPEC-021).

**The console's own rule says to ask.** `dm.ts`'s opening comment: "Ask first
before an action nothing undoes, whose damage reaches people not here to be
told". "New link", "Boot" and "Make spectator" ask through `DMDeps.confirm`;
Remove does not, and the comment above it gives a reason that is false ("exactly
one control asks first"; "A removal lands on the log every seat at this table
can see"), repeated in `client/test/dm-view.test.ts` above the test that presses
Remove.

**The owner's ruling, 2026-11-22.** Asked whether Remove should ask first:
"Fråga alltid", always, like Boot and Make spectator.

**What MapTool does.** Deleting tokens asks by default: `msg.confirm.deleteToken`,
"Are you sure you want to delete the selected tokens?", guarded by the
preference `tokensWarnWhenDeleted` (`AppPreferences`, default on), which the
dialog lets the user switch off. Borrowed: asking before an irreversible
removal. Not borrowed: the switch to stop asking, which the ruling's "always"
rules out.

**What this is not.** A change to the server, to `remove_actor` or to what a
removal does or who is told of it. A change to "Remove" for a token (`remove-token`)
or to any other control. A way for the agent, which removes through MCP, to be
asked.

## Done looks like

1. **Remove asks first.** Pressing Remove with an actor id in the field asks the
   DM, in a message holding the actor's name (its id when the window holds no
   actor of that id) and saying nothing brings it back; when the DM declines,
   nothing is sent and the field keeps what was typed; when the DM accepts,
   `remove_actor` is sent as today. A client test that fails on today's tree.
2. **An empty field still asks nothing.** Pressing Remove with the field empty
   or blank says "name the actor to remove", asks nothing and sends nothing (the
   existing test, still green).
3. **No false reason is left.** The comment above Remove in `dm.ts` and the one
   above the Remove test in `client/test/dm-view.test.ts` no longer say that one
   control asks first or that every seat sees a removal.
4. **The gate.** `task check` is green, `cmd/vtt/webdist` is rebuilt from the
   changed client, and `task e2e` is green.

## Rules this puts on the system

- The console's Remove asks the DM first, in a message holding the actor's
  name, and sends nothing when the DM declines.

## What it touches

`client/src/view/dm.ts`, `client/test/dm-view.test.ts`, and `cmd/vtt/webdist`
rebuilt.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **Whether SPEC-017, which holds what a removal does, also states the
  console's controls.** If it does, it moves too.
- **The message's exact words** beyond the actor's name and that nothing brings
  it back (the option the owner chose carried an example: "Remove Goblin? Nothing
  brings it back, and its tokens leave the board."); whether it should also name the actor's
  controllers, as "Make spectator" names what a player stops controlling, was
  not decided.
