# A board click moves only onto a square the character sees and can enter; any other click does nothing

## The problem

**A click off the map lands on its edge.** The board's click handler in
`renderGrid` (`client/src/view/spectator.ts`) takes the click's offset from
the board's box, turns it into world pixels through `worldFromScreen`, and
hands `cellFromPoint` (`client/src/view/grid.ts`) the result. `cellFromPoint`
clamps each coordinate into the grid, so a click in the empty pane beside
the map, where `fitCamera` (`client/src/view/camera.ts`) centres a scene
narrower than the pane, or on the board's border, becomes the nearest edge
square. The
board's `onCell` in `client/src/app.ts` then sends what that square asks for:
in a player's window `playerClickFor` (`client/src/view/player.ts`), a
`set_viewpoint` or a `move_token` of the watched one's token; in a DM's or an
agent's window `doorCommandFor` (`client/src/view/doors.ts`) with doors
armed, else `moveCommandFor`. So a stray click beside the map moves a
character, or works an edge door, at the far side of the board.

**A click in the dark sends a move the server refuses.** A player's board
draws three kinds of square: those the watched character sees now
(`Scene.Visible`), those it remembers, dimmed (`planFog` in
`client/src/view/scene-plan.ts`, `Explored − Visible`), and those it has
never seen, where only the lattice is drawn. `playerClickFor` sends a move to
any of them. The server's move gate (`handleCommand` in
`internal/gateway/server.go`, SPEC-013) refuses a square the judging eyes do
not see with `You cannot move there: you cannot see that square.` (VTT-601),
and a seen square `engine.State.Blocked` blocks with `You cannot move there: `
and the reason, such as `a closed door` (VTT-602), and the window shows the
refusal.

**The owner's rulings, 2026-11-21.** "endast när du klickar på 'synliga'
rutor så kan du förflytta dig, klickar du utanför, så händer ingenting -
inte ens en varning. Man får lära sig att klicka på de rutor man kan gå till
som användare". Asked what "synliga" covers: the squares the character sees
now, not the remembered ones and not the unseen ones. Asked about a seen
square the character cannot enter, a wall, a closed door or blocking
scenery: nothing there either.

**What MapTool does.** Its map has no edge to click off. It does refuse a
player's movement on the client, from its own copy of the map: its path
walker (`AbstractAStarWalker`) refuses a step out of the exposed fog for a
view that is not the GM's (`fowBlocksMovement`), and one across
movement-blocking topology (`vblBlocksMovement`). Borrowed: the window
refuses from what it holds, and the GM's view is exempt. Not borrowed:
MapTool's bound is the exposed area, every square ever seen, which is this
platform's `Explored`; the ruling's bound is `Visible`. Nor the path.

**What this is not.** A change to the server's move gate or its refusal
texts (VTT-601, VTT-602 stand as worded): a move that reaches the server is
judged as today. A change to what a DM's or an agent's click does on a
square of the grid. A path, a distance, or a token standing in the way. A
change to the door buttons, to the spectator's board, which already sends
nothing (the owner's ruling "Bara knappar", 2026-10-20), or to how the
board is drawn.

## Done looks like

1. **A click off the grid does nothing.** In a player's window, and in a
   DM's window with doors armed and unarmed, a click on the board outside
   the drawn grid sends no command, on each of the four sides. A client test
   that fails on today's tree.
2. **A player's click on a square they do not see does nothing.** A click
   on a remembered square of the current scene, and one on a never-seen
   square, sends no command, neither a move nor a switch. A client test that
   fails on today's tree.
3. **A player's click on a seen square the character cannot enter does
   nothing.** A click on a seen wall, a seen closed door, and a seen square
   covered by scenery that blocks movement sends no move. A client test that
   fails on today's tree.
4. **A seen square the character can enter still moves.** A player's click
   on a seen floor square, a seen open door, and a seen square with no
   terrain moves the watched one's token there (VTT-408), and a click on a
   seen square holding another listed party member's token still switches
   to them (VTT-919). The existing tests, plus a client test for the open
   door.
5. **No warning appears.** In the real client against the real server, a
   player's click beside the map and a click on a square their character
   does not see leave the token where it was and show no refusal text. An
   e2e check.
6. **The DM's grid stands.** A DM's or an agent's click on a square of the
   grid sends what it sends today. The existing tests, still green.
7. **The gate.** `task check` is green, the embedded bundle under
   `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
   green.

## Rules this puts on the system

- A board click outside the drawn grid sends no command, in any window.
- A player's board click on a square of the current scene the watched
  character does not see now sends no command.
- A player's board click that would move the watched one's token onto a seen
  square the window holds as a wall, a closed door, or covered by scenery
  that blocks movement sends no command.

VTT-408 stands as worded. VTT-919 (a click on a listed party member's
token switches) is false as worded for a square not seen now, and VTT-884
(a click on a door square sends what a square with no door sends) for a
seen closed door; the plan decides their successors.

## What it touches

In this order:

1. **The click's square.** `client/src/view/grid.ts` (`cellFromPoint` and
   its clamp) and `client/src/view/spectator.ts` (`renderGrid`'s click
   handler).
2. **What a click sends.** `client/src/view/player.ts` (`playerClickFor`,
   `moveCommandFor`), `client/src/view/doors.ts` (`doorCommandFor`, only if
   the off-grid case reaches it), `client/src/app.ts` (the board's `onCell`),
   and wherever the plan places the client's reading of a seen square's
   terrain (`Scene.Tiles`, `Scene.OpenDoors`, `Scene.Objects` in
   `client/src/state.ts`).
3. **The tests.** `client/test/grid.test.ts` ("clicks outside the board are
   clamped, never negative or past the edge" pins today's clamp),
   `client/test/camera.test.ts`, `client/test/spectator-view.test.ts`,
   `client/test/player.test.ts`, `client/test/app.test.ts`, and an e2e spec
   under `client/e2e/`.
4. **The bundle.** `cmd/vtt/webdist`, rebuilt.

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **The server's sight and the window's `Visible` can disagree for a
  moment**: between a move the server has accepted and the `sceneSeen` that
  follows it, the window holds the old set. A click in that gap on a square
  newly seen sends nothing, and one on a square no longer seen still reaches
  the server and draws its refusal. Not traced further.
- **A player's scene with no `Visible` at all.** `tokensOnScene` and
  `planFog` read an absent `Visible` as "this viewer sees everything", which
  is the DM's and the agent's stream. Whether a player's window can hold a
  scene before its first `sceneSeen` was not traced.
- **`client/e2e/table.spec.ts`'s "a player sees their own panel and can
  move their token"** clicks a fixed pixel offset into `.grid`. Whether that
  square is one the player's character sees in that fixture was not
  checked; if it is not, the test goes red under this ticket.
- **The client's reading of a blocked square repeats `engine.State.Blocked`**
  in `internal/engine/terrain.go`. How the two are kept from drifting is the
  plan's.
