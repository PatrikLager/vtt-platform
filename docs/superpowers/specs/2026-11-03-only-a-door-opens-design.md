# Only a door opens or closes

## The problem

**`open_door` and `close_door` are accepted at any square of a scene,
whatever its tile is.** Nothing on the command path reads the square's
nature. `Authorize` runs `mayWorkDoor` (`internal/gateway/authz.go`), which
asks a player only where the tokens they control stand and returns nil for
every other role. `handleCommand` (`internal/gateway/server.go`) has no door
check. `ToEvent` (`internal/gateway/convert.go`) turns the command into a
`DoorOpened` or a `DoorClosed`. `engine.Apply`'s two door arms
(`internal/engine/apply.go`) refuse only a scene the state does not hold and
a missing position, then write the square's key into the scene's `OpenDoors`
or delete it. So an `open_door` at a wall is appended, folded and sent, and
its result is ok. The wall still blocks: `engine.State.Blocked`
(`internal/engine/terrain.go`) answers `a wall` before it reads `OpenDoors`,
and `internal/sight` reads `OpenDoors` only for a door tile. The log holds a
door opened where there is none. Both tools' descriptions, written by
`tools/toolgen/main.go`, say the opposite: "Refused unless the square's tile
is a door".

The window does not send one. Its door control builds a command only for a
square whose tile is a door (`doorCommandFor` in `client/src/view/doors.ts`),
for the DM and for a player alike. The agent does: it sends `open_door`
through `vtt mcp` at the square it names, and it trusts the description.

The alpha test of 2026-10-22 found it: the agent opened a stone wall square,
and the result was ok (A-8 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`). At the table, an
agent that names the square beside a door is told the door opened, narrates a
door the players cannot walk through, and is not told that it named a wall.

What a door is, is already written. The maps-as-geometry design
(`docs/superpowers/specs/2026-08-12-maps-as-geometry-design.md`, §3.3 and
§6) makes `door` one of three tile kinds the platform reads, and working a
door a spatial act, not a game system's (CLAUDE.md rule 5). On 2026-11-02 the
owner put movement and the board on the board engine's side of the line
between it and the rules engine; this ticket stays on that side.

MapTool has no door to work. A door there is topology, not a square's
nature: `DungeonDraftImporter` imports a closed door as a `Wall` or as a
stroke of VBL and MBL, and a macro (`eraseVBL`) erases topology without asking
what lies there. No command opens a door at a square, so nothing refuses one.

## Done looks like

1. The agent's `open_door` at a square whose tile is a wall is refused, and
   nothing is appended. A gateway test names it, and fails on today's tree,
   where the result is ok.
2. `open_door` and `close_door` are refused, appending nothing, at a wall, at
   a floor, and at a square of the scene with no tile; for the DM, for the
   agent, and for a player with a controlled token beside the square. A
   gateway test names each.
3. The refusal is in plain words, names no package and no id, and reads the
   same to every role. The test of item 2 names its text.
4. `open_door` and `close_door` at a door square are accepted for the DM, the
   agent and a player beside it, and the tests that hold VTT-139, VTT-147 and
   VTT-149 pass unchanged.
5. `task check` exits 0 on the branch.

## Rules this puts on the system

- `open_door` and `close_door` are refused, for every role, at a square
  whose tile is not a door, and append nothing.

VTT-147 stands as worded: the DM and the agent work a door from anywhere, and
what they work is still a door.

## What it touches

1. `internal/gateway/server.go` (`handleCommand`), and `internal/engine` if the
   plan asks the engine the question, as the move gate asks
   `engine.State.Blocked`; with their tests.
2. `docs/requirements.md`: the rule above.

One component, the board engine, in at most two packages.

## Specifications this moves

docs/specifications/013-authorization.md

## What could not be established

- What the refusal reads, and whether it names what the square is. The
  plan's, with the words put to the owner at sign-off.
- Whether a door command at a scene the state does not hold keeps
  `engine.Apply`'s refusal or is answered by the new one. The plan's.
