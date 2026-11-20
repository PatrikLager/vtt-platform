// A player's door buttons and a DM's or an agent's armed click (SPEC-022).
// NO DOM, NO WIRE SEND: view/player.ts and app.ts's onCell send what this builds.
// Keep doorDirection's geometry authz.go's mayWorkDoor's (VTT-149), or a player
// is offered a button the server refuses; see doors.test.ts and authz_test.go's
// TestAuthorizePlayerMayWorkDiagonallyAdjacentDoor.

import type { State } from "../state";
import type { Me } from "../metadata";
import type { ClientCommand } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import { openDoor, closeDoor, type Point } from "../commands";
import { withinRange } from "../player";
import { doorKey } from "../fold";

/**
 * currentSceneId is the scene a board click and a door button apply to: the
 * greatest scene id, as renderSpectator picks it. Never file its `?? ""` in
 * tools/ts-mutation-equivalents.txt: player.test.ts's "a prototype-injected
 * empty-string scene defeats the `?? \"\"` equivalence claim" kills it.
 */
export function currentSceneId(st: State): string {
  return Object.keys(st.Scenes).sort().at(-1) ?? "";
}

// Keep each word the owner's (VTT-882).
const DIRECTIONS = [
  ["north-west", "north", "north-east"],
  ["west", "here", "east"],
  ["south-west", "south", "south-east"],
];

/** doorDirection is the way to cell from a player's token of actorId beside it (VTT-881, VTT-886). */
export function doorDirection(st: State, me: Me, actorId: string, cell: Point): string | undefined {
  if (me.role !== "player") return undefined;
  const sceneId = currentSceneId(st);
  const tok = Object.values(st.Tokens).find((t) => {
    if (t.SceneID !== sceneId || t.ActorID !== actorId) return false;
    const actor = st.Actors[t.ActorID];
    if (!actor || !actor.controllerIds.includes(me.participantId)) return false;
    return withinRange({ x: t.X, y: t.Y }, cell, 1);
  });
  if (!tok) return undefined;
  return DIRECTIONS[cell.y - tok.Y + 1]![cell.x - tok.X + 1];
}

/** DoorButton is one door a player's panel offers: its label and, per click, its command (VTT-882, VTT-883). */
export interface DoorButton {
  label: string;
  command: () => ClientCommand;
}

/** doorButtonsFor is a player's door buttons for actorId, in the board's reading order (VTT-881, VTT-887). */
export function doorButtonsFor(st: State, me: Me, actorId: string): DoorButton[] {
  const sceneId = currentSceneId(st);
  const scene = st.Scenes[sceneId];
  if (!scene) return [];
  const doors: { at: Point; direction: string; open: boolean }[] = [];
  for (const [key, tile] of Object.entries(scene.Tiles ?? {})) {
    if (tile.Kind !== "door") continue;
    const [x, y] = key.split(",").map(Number) as [number, number];
    const direction = doorDirection(st, me, actorId, { x, y });
    if (direction !== undefined) doors.push({ at: { x, y }, direction, open: scene.OpenDoors?.[key] === true });
  }
  return doors
    .sort((a, b) => a.at.y - b.at.y || a.at.x - b.at.x)
    .map(({ at, direction, open }) => ({
      label: `${open ? "Close" : "Open"} door (${direction})`,
      command: () => (open ? closeDoor(sceneId, at, actorId) : openDoor(sceneId, at, actorId)),
    }));
}

/** doorCommandFor is what a DM's or an agent's armed board click on cell sends, naming no actor (VTT-876, VTT-888). */
export function doorCommandFor(st: State, armed: boolean, cell: Point): ClientCommand | null {
  if (!armed) return null;
  const sceneId = currentSceneId(st);
  const scene = st.Scenes[sceneId];
  if (!scene) return null;
  const key = doorKey(cell.x, cell.y);
  if (scene.Tiles?.[key]?.Kind !== "door") return null;
  return scene.OpenDoors?.[key] ? closeDoor(sceneId, cell) : openDoor(sceneId, cell);
}
