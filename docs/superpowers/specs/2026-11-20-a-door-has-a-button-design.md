# A player works a door by a button for that door, and a board click always moves

## The problem

**A player works a door through a mode.** The player panel offers an
"Arm doors" toggle (`data-action="arm-doors"`, `renderPlayerPanel` in
`client/src/view/player.ts`) while a door of the current scene stands beside
a token of the character a board click moves (`mayWorkAnyDoor`, `actingActor`,
VTT-875). Armed, a board click on a door square sends `open_door` or
`close_door` naming that character, and a click anywhere else sends nothing
(`doorCommandFor` in `client/src/view/doors.ts`, the board's `onCell` in
`client/src/app.ts`, VTT-873, VTT-874); the board shows "Doors armed — click
a door to open or close it." (`renderGrid` in `client/src/view/spectator.ts`),
and a switch of the watched character disarms the tool (`paint` in
`client/src/app.ts`, VTT-879). Unarmed, a click on a door square moves the
watched token there, which a closed door refuses (`Blocked` in
`internal/engine/terrain.go`). So working a door takes two clicks and a
mode, and a player who forgets the mode is armed finds the board has stopped
moving their token. The DM console carries its own copy of the toggle
(`renderDMConsole` in `client/src/view/dm.ts`), offered whatever the scene
holds, and the same bit, `ui.doorsArmed` in `client/src/app.ts`, serves both.

**The owner's rulings.** 2026-11-19, asked why a door needed arming rather
than nearness alone, and answered that a board click means move and an open
door is a square a token can stand in, so a click on a door is ambiguous:
"Knapp per dörr, nästa ärende" — no arming mode for a player; while the
watched character stands beside a door, the panel shows one button per door;
a board click always moves. 2026-11-20: every button names the door's
direction from the character, even when only one door stands beside it,
north being up on the board and the door the character stands in being
"here": "Open door (east)", "Close door (north)", "Close door (here)".

**What MapTool does.** MapTool's core has no door: a door is
vision-blocking geometry (`DungeonDraftImporter` writes a door as a stroke
of VBL) that a macro changes through the topology functions
(`Topology_Functions`), and a macro is run from a macro button
(`MacroButtonProperties`), not from a mode of a click on the map. Borrowed:
the act is a button in a panel, beside the character's other acts. Not borrowed: the macro; here the
window derives the buttons from the board, and the server judges each
command as it does today (VTT-149).

**What this is not.** A change to which door a player may work (VTT-149,
VTT-609) or to what the server accepts. The DM console's arming, which the
ruling does not name and which stays as it is, with its board label. A door
drawn differently on the board, or a button marking its door there. A lock,
a key, or a door a token stands in being refused a close.

## Done looks like

1. **A button per door.** A player's panel shows one button for each door
   of the current scene that a token on that scene of the character they
   watch stands at most one square from on each axis, and none for any
   other door; with no such door, it shows none. A client test that fails
   on today's tree.
2. **The button says which door, and what it does.** A closed door's button
   reads "Open door (<direction>)" and an open one's "Close door
   (<direction>)", the direction one of north, north-east, east,
   south-east, south, south-west, west, north-west from the token to the
   door, north being up on the board, or "here" for the door the token
   stands in. A client test that fails on today's tree.
3. **The button works its door as the watched character.** A click on a
   door's button sends `open_door` for a closed door and `close_door` for an
   open one, at that door's square in the current scene, naming the
   character the player watches. A client test that fails on today's tree.
4. **A player's board click never works a door.** In a player's window a
   click on a door square sends what a click on any other square sends — a
   move or a switch (VTT-408, VTT-460) — and the panel offers no arm-doors
   toggle, whatever the window's arming bit holds. A client test that fails
   on today's tree.
5. **The buttons follow the watched character.** After a switch to a
   character beside other doors, or beside none, the panel shows the
   buttons for that character's doors. A client test.
6. **The DM's arming stands.** The DM console still offers "Arm doors", a
   DM's or an agent's armed click on a door still works it naming no actor
   (VTT-876), and the board still shows the armed label to them. The
   existing tests, still green.
7. **The real wire.** `client/e2e/doors.spec.ts` works the cellar's door
   from a player's window by the door's button, and the Story names the
   character. An e2e check.
8. **The gate.** `task check` is green, the embedded bundle under
   `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
   green.

## Rules this puts on the system

- A player's panel shows one button per door of the current scene that a
  token on that scene of the watched character stands at most one square
  from on each axis, and no other door button.
- A door's button reads "Open door" for a closed door and "Close door" for
  an open one, followed by the door's direction from the token in
  parentheses, "here" for the door the token stands in.
- A door's button sends `open_door` or `close_door` at that door's square in
  the current scene, naming the watched character.
- A player's board click on a door square sends what a click on any other
  square sends.
- A player's window offers no arm-doors toggle.

VTT-873, VTT-874, VTT-875 and VTT-879 describe the player's arming and are
withdrawn or succeeded, as the plan decides; VTT-149, VTT-609 and VTT-876
stand as worded.

## What it touches

In this order:

1. **The client's rules.** `client/src/view/doors.ts` (`mayWorkDoor`,
   `doorCommandFor`, and the pure function that lists a player's doors with
   their direction and state), `client/src/view/player.ts`
   (`PlayerUIState.doorsArmed`, `mayWorkAnyDoor`, `actingActor`, the toggle
   in `renderPlayerPanel`), `client/src/commands.ts` (`openDoor`,
   `closeDoor`, unchanged unless the plan finds otherwise).
2. **The window.** `client/src/app.ts` (`ui`, `paint`'s disarm on a switch,
   the board's `onCell`, the board's doors bit), `client/src/view/dm.ts`
   (`DMDeps.doorsArmed`'s comment, which says the player panel offers the
   same toggle), `client/src/view/spectator.ts` (the armed label, unchanged
   unless the plan finds otherwise), and `cmd/vtt/webdist`.
3. **The tests.** `client/test/doors.test.ts`, `client/test/player.test.ts`,
   `client/test/app.test.ts`, `client/test/qa-door-window.test.ts`,
   `client/test/commands.test.ts`, the client tests that build a
   `PlayerUIState` literal if its shape changes, and
   `client/e2e/doors.spec.ts`.
4. `docs/requirements.md`: the rows the rules earn, and VTT-873, VTT-874,
   VTT-875 and VTT-879.

This stays inside the client.

## Specifications this moves

- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **An actor with two tokens on one scene.** The direction is measured from
  a token; which one, when two tokens of the watched character stand beside
  the same door, is the plan's.
- **The buttons' order and place.** Where in the panel the buttons stand
  and in what order is the plan's; SPEC-022's "Outside the page" places the
  arm-doors toggle below the page on every page today.
- **A window whose role changes while armed.** A DM who arms doors and is
  made a player keeps `ui.doorsArmed`; Done item 4 asks that the bit not
  reach a player's click, and how is the plan's.
