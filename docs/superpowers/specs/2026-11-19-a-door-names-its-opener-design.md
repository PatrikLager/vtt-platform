# A door's line in the Story names the character that opened or closed it

## The problem

**A door opens and nobody did it.** `OpenDoor` and `CloseDoor`
(`contract/vtt/v1/commands.proto`) carry a scene id and a square, and so do
the events they become, `DoorOpened` and `DoorClosed`
(`contract/vtt/v1/events.proto`; `ToEvent` in
`internal/gateway/convert.go` copies the two fields). The Story words a
door event "a door opened" or "a door closed" (`tell` in
`client/src/view/spectator.ts`, VTT-819) and the Events panel "a door opened
at x,y" (`describe`). A player may work a door when any token on its scene
whose actor they control stands beside it (`mayWorkDoor` in
`internal/gateway/authz.go`, VTT-149), so a player with two characters
beside a door opens it as neither. The board's door tool offers a door by
the same test (`mayWorkDoor` in `client/src/view/doors.ts`) and sends
`openDoor(sceneId, cell)` (`doorCommandFor` in the same file, `openDoor` in
`client/src/commands.ts`).
The DM and the agent work a door from anywhere (VTT-147); the agent's
`open_door` and `close_door` tools are generated from the two commands
(`tools/toolgen/main.go`).

**The owner's rulings.** 2026-11-16: "om tobbe öppnar en dörr, så ska det
stå 'tobbe öppnade dörren'", a door being an event in the game that carries
the story on. 2026-11-19: the opener is the character, never the
participant, and closing is worded the same way; a player with several
characters opens as the one they watch, the one a board click moves, and is
offered the door only when that character stands beside it; the DM and the
agent may name any character of the world or none, and the DM console names
none; a viewer that did not see the opener before the door moved is sent no
line, as with an ability used; an actor the world does not hold is refused
by the gateway in plain words; and no check is built that a player's named
character is theirs or stands beside the door, since only a command sent
by hand, outside the window, names another ("detta är ett teoretiskt fall,
vi bygger inte kod/kontroller för det").

**What MapTool does.** MapTool's core has no door: a door is vision-blocking
geometry a macro changes, and a macro's chat output speaks as the token the
user impersonates (`ImpersonateMacro`, `CommandPanel.TokenIdentity`), which
a server may restrict to tokens the player owns
(`ServerPolicy.isRestrictedImpersonation`). Borrowed: the act is the
character's the player is acting as. Not borrowed: the server restricting
whom a player may name; here the window offers only the character the
player watches, and the gateway asks of a player what it asks today, and of
every role only that a named actor exists.

**What this is not.** A door's lock, key or any rule of who may open what.
A name for the DM's own opening. The square in the Story. A participant's
name anywhere a player reads. A change to which door a player may work
(VTT-149), or a check of the character a player's command names beyond its
existence.

## Done looks like

1. **The event names the opener.** A `DoorOpened` and a `DoorClosed` carry
   the id of the actor that opened or closed the door, or none, copied from
   the command; `engine.Apply` refuses one naming an actor it has not folded.
   A Go test that fails on today's tree.
2. **A named character exists.** A door command naming an actor the world
   does not hold is refused in plain words, naming no package and no id,
   whatever the issuer's role, and appends nothing. A Go test that fails on
   today's tree.
3. **The DM and the agent may name any character, or none.** A DM's or an
   agent's door command naming an actor the world holds, or none, is
   accepted from any square, and the event names it. A Go test.
4. **A viewer sees the opener only if it saw them.** A player or spectator
   is sent a door event naming an actor only when it saw that actor before
   the event; one that sees the square and did not see the actor is sent
   the door's new state with no event id, so its Story draws no line. A Go
   test that fails on today's tree.
5. **The lines name the opener.** The Story draws a door event naming an
   actor as "<actor> opened a door" or "<actor> closed a door", and the
   Events panel adds " at x,y"; one naming none reads as today. A client
   test that fails on today's tree.
6. **The window sends the opener.** In a player's window with doors armed,
   a click on a door sends a door command naming the character the player
   watches, and the tool offers no door that character's token does not
   stand beside; a DM's window sends none. A client test that fails on
   today's tree.
7. **The agent's tools carry it.** The `open_door` and `close_door` tools'
   input schema carries the actor and its description says who may name
   one. A Go test.
8. **The gate.** `task check` is green, the embedded bundle under
   `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
   green.

## Rules this puts on the system

- A door event names the actor that opened or closed it, or none, as its
  command named it.
- A door event naming an actor the fold has not folded is refused.
- A door command naming an actor the world does not hold is refused in
  plain words, for every role.
- A DM's or an agent's door command may name any actor the world holds, or
  none, from any square.
- A player or spectator is forwarded a door event naming an actor only when
  it saw that actor before the event.
- A player or spectator that sees a door's square and is not forwarded its
  door event learns the door's new state without an event id.
- The Story words a door event naming an actor "<actor> opened a door" or
  "<actor> closed a door"; the Events panel adds the square.
- A player's door click names the character they watch, and the door tool
  offers only a door that character's token stands beside.
- The DM console's door command names no actor.
- The agent's `open_door` and `close_door` tools take the actor without
  requiring it, and say whom it may name.

VTT-819 is narrowed by these rules; VTT-149, VTT-212 and VTT-609 stand
as worded, as the plan decides.

## What it touches

In this order:

1. **The contract.** `contract/vtt/v1/commands.proto` (`OpenDoor`,
   `CloseDoor`), `contract/vtt/v1/events.proto` (`DoorOpened`,
   `DoorClosed`), the generated code under `contract/gen` and
   `cmd/vtt/tools.json`, and the fixtures under `contract/testdata`
   (`expected_tools.json` among them) with `contract/roundtrip_test.go` and
   `contract/events.test.ts`.
2. **The folds.** `internal/engine/apply.go` (the `DoorOpened` and
   `DoorClosed` arms), `client/src/fold.ts` (its `doorOpened` and
   `doorClosed` arms), and `internal/eventgen/model.go` (`openDoor`,
   `closeDoor`).
3. **The gateway.** `internal/gateway/convert.go` (`ToEvent`) and
   `internal/gateway/server.go` (the door gate in `handleCommand`, which
   gains the check that a named actor exists).
4. **The projection.** `internal/perceive/projection.go` (`classify`'s door
   arms, `doorTransitions`).
5. **The agent's tools.** `tools/toolgen/main.go` (`open_door`,
   `close_door`) and what it generates.
6. **The client.** `client/src/commands.ts` (`openDoor`, `closeDoor`),
   `client/src/view/doors.ts` (`mayWorkDoor`, `doorCommandFor`),
   `client/src/view/player.ts` (`moveCommandFor`'s choice of actor,
   `mayWorkAnyDoor`),
   `client/src/view/spectator.ts` (`describe`, `tell`), `client/src/app.ts`
   (the board's `onCell`), and `cmd/vtt/webdist`.
7. `docs/requirements.md`: the rows whose words change, as the plan
   decides.

This crosses the contract, the engine, the gateway, the projection, the
agent's tools and the client; the owner ruled it one ticket (2026-11-19).

## Specifications this moves

- docs/specifications/007-the-wire-contract.md
- docs/specifications/013-authorization.md
- docs/specifications/016-the-projection.md
- docs/specifications/018-the-fold-bounds-free-text-in-bytes.md
- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **The refusal's words for an actor the world does not hold.** The owner
  ruled the gateway answers it in plain words; the words are the plan's.
- **A player who watches a character they do not control.** Whether the
  set a player watches can hold such a character was not traced; the
  client offers no door then.
- **The real wire.** No spec under `client/e2e` works a door; whether an
  e2e check is added is the plan's.
