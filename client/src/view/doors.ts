// What a click on a square means when doors are armed, and who may make it
// mean that.
//
// A separate module rather than more of view/player.ts: this predicate is
// shared by the DM console and the player panel, and player.ts is already
// the player's own surface.
//
// NO DOM, NO WIRE SEND. doorCommandFor only builds the ClientCommand a later
// task (arming a board mode) hands to send() from app.ts's onCell; nothing
// here decides whether a click WILL toggle a door, only what it would mean
// if one were sent.
//
// THE CLIENT HALF OF A RULE THAT LIVES IN TWO PLACES.
// internal/gateway/authz.go's mayWorkDoor is the enforcing copy — this one
// only decides whether to OFFER the control. They must agree on the
// geometry (Chebyshev distance <= 1), or this module offers a control the
// server is certain to refuse, the same "affordance whose every use is a
// refusal" this codebase rejects elsewhere. See client/test/doors.test.ts
// and internal/gateway/authz_test.go's
// TestAuthorizePlayerMayWorkDiagonallyAdjacentDoor /
// TestAuthorizePlayerMayNotWorkDiagonallyDistantDoor for the shared numbers.

import type { State } from "../state";
import type { Me } from "../metadata";
import type { ClientCommand } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import { openDoor, closeDoor, type Point } from "../commands";
import { withinRange } from "../player";
import { doorKey } from "../fold";

/**
 * The scene a board click applies to: the scene whose id sorts
 * lexicographically last. Same convention as view/spectator.ts's
 * renderSpectator, which calls this same pick "the most recently created
 * one" — a claim about ids sorting in creation order, not something this
 * function actually checks. Stated here more precisely: it is the
 * lexicographically greatest scene id, which happens to match creation
 * order for every id this codebase currently mints, and nothing more. There
 * is no "current scene" on the wire yet (a scene selector belongs with the
 * DM console, per spectator.ts's own comment).
 *
 * THE `?? ""` FALLBACK IS UNREACHABLE UNDER THE FOLD INVARIANT, WHICH IS NOT
 * THE SAME AS EQUIVALENT, and an earlier draft of this comment said the
 * wrong one. A distinguishing state EXISTS and has been run: empty Scenes,
 * one token carrying SceneID "", its actor controlled by the caller and
 * adjacent to the cell — mayWorkDoor answers true as written and false under
 * a different fallback string. So `tools/ts-mutation-equivalents.txt`'s own
 * test ("is there ANY observable that distinguishes them?") is NOT met, and
 * this must never be transcribed there as an equivalence.
 *
 * What is true is narrower: no state fold() can PRODUCE reaches that branch.
 * The fallback fires only when st.Scenes has no keys (`.at(-1)` on an empty
 * array is undefined), and tokenPlaced — the only arm that ADDS a key to
 * st.Tokens — refuses a token whose scene does not already exist ("token
 * placed on unknown scene"). Two further premises carry it and are stated
 * because they are the ones that can break: nothing anywhere removes a scene
 * from st.Scenes, and session.ts holds the last good state when a fold
 * throws, so a TRUNCATED log holding a tokenPlaced whose sceneCreated sits
 * below its cursor never yields a state at all. A scene-hidden arm symmetric
 * with tokenHidden would falsify the first without touching st.Tokens.
 *
 * Note what this comment cannot claim: that callers only ever pass
 * fold-produced states. doors.test.ts hands this module several states fold()
 * cannot build — a Scene with no Tiles, a Scene with no OpenDoors, a token
 * whose actor is missing — and each of those tests earns its keep.
 */
function currentSceneId(st: State): string {
  return Object.keys(st.Scenes).sort().at(-1) ?? "";
}

/**
 * mayWorkDoor mirrors internal/gateway/authz.go's mayWorkDoor (maps-as-
 * geometry spec §6: "hard for players, free for DM") — but FAILS CLOSED
 * where the Go side's early return does not have to. authz.go's own
 * commandRoles table grants open_door/close_door to dm, agent and player
 * only, so Authorize refuses a spectator (or anything else) before
 * mayWorkDoor ever runs there; its `p.Role != identity.RolePlayer` early
 * return is safe only because a spectator can never reach it. This module
 * has no such upstream gate — it IS the only check standing between a role
 * and the control — so it enumerates explicitly: dm and agent pass
 * unconditionally (they author the world and are free of the adjacency
 * rule, the same bypass move_token's ownership check never applies to them
 * either); player takes the adjacency test below; every other role,
 * spectator included, and any role this build has never heard of, is
 * refused. Offering a control a role can never actually use would be
 * exactly the "affordance whose every use is a refusal" this codebase
 * rejects elsewhere.
 *
 * SPATIAL ONLY, deliberately (CLAUDE.md rule 5): this asks WHERE a
 * participant's tokens are, never what edition or ruleset is in play.
 * Adjacency reuses player.ts's withinRange (Chebyshev distance — a diagonal
 * step costs the same as an orthogonal one), matching the Go side's
 * `abs(dx) <= 1 && abs(dy) <= 1` exactly.
 */
export function mayWorkDoor(st: State, me: Me, cell: Point): boolean {
  if (me.role === "dm" || me.role === "agent") return true;
  if (me.role !== "player") return false;
  const sceneId = currentSceneId(st);
  return Object.values(st.Tokens).some((tok) => {
    if (tok.SceneID !== sceneId) return false;
    const actor = st.Actors[tok.ActorID];
    if (!actor || !actor.controllerIds.includes(me.participantId)) return false;
    return withinRange({ x: tok.X, y: tok.Y }, cell, 1);
  });
}

/**
 * doorCommandFor is what a board click means when the door tool is armed:
 * open a closed door, close an open one, or nothing when the cell holds no
 * door at all or this viewer may not work it.
 *
 * `!armed` returns null unconditionally — a plain click is a move, and
 * deciding that is not this module's business (view/player.ts's
 * moveCommandFor owns it). Door state is read from the folded scene's
 * Tiles/OpenDoors exactly as fold.ts writes them (doorKey(x, y)), so this
 * and the fold always agree on what "a door here" and "open" mean.
 */
export function doorCommandFor(
  st: State,
  me: Me,
  armed: boolean,
  cell: Point,
): ClientCommand | null {
  if (!armed) return null;
  const sceneId = currentSceneId(st);
  const scene = st.Scenes[sceneId];
  if (!scene) return null;
  const key = doorKey(cell.x, cell.y);
  if (scene.Tiles?.[key]?.Kind !== "door") return null;
  if (!mayWorkDoor(st, me, cell)) return null;
  return scene.OpenDoors?.[key] ? closeDoor(sceneId, cell) : openDoor(sceneId, cell);
}
