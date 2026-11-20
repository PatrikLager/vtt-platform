import { test, expect } from "bun:test";
import { toJson } from "@bufbuild/protobuf";
import { newState, type State } from "../src/state";
import { doorButtonsFor, doorCommandFor, doorDirection } from "../src/view/doors";
import type { Me } from "../src/metadata";
import { ClientCommandSchema, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

/**
 * THE CLIENT HALF OF A RULE THAT LIVES IN TWO PLACES.
 * internal/gateway/authz.go's mayWorkDoor is the enforcing copy; this one
 * decides whether to OFFER the control. They must agree on the geometry, so
 * cases 7 and 8 below use the same offsets as its two paired Go tests —
 * TestAuthorizePlayerMayWorkDiagonallyAdjacentDoor and
 * TestAuthorizePlayerMayNotWorkDiagonallyDistantDoor. Standalone functions,
 * NOT part of authz_test.go's table-driven
 * TestAuthorizeTableAllCommandsAllRoles — added alongside this file because
 * no pre-existing Go case had |dx| = |dy| = 1 (see those two tests' own
 * comment for exactly which deltas the older cases used, and why none of
 * them can tell Chebyshev from Manhattan).
 *
 * DEVIATION FROM THE BRIEF'S LITERAL FIXTURE TEXT, forced by the real
 * shapes in state.ts/fold.ts (same situation task-2-report.md hit with
 * `client:typecheck`, this time visible even without it): Scene's id field
 * is `ID`, not `SceneId` (fold.ts's sceneCreated arm: `st.Scenes[v.sceneId]
 * = { ID: v.sceneId, ... }`), and a Tiles entry is a `Tile` object
 * (`{Kind, Material, Art}`, built by fold.ts's sceneCreated arm), never a
 * bare string — a plain string "door" has no `.Kind`, so a doorCommandFor
 * that (correctly) reads `tile.Kind` would silently treat the brief's
 * literal fixture as "not a door" and every case built on it would read
 * wrong for the wrong reason. Fixed to the shape scene-plan.test.ts's "a
 * door's picture reflects open/closed state; closed is the unmarked
 * default" test already builds (a Scene literal with a door Tile, both
 * open and closed exercised) — grid.test.ts's own Scene literals carry no
 * Tiles at all, so citing that file in the previous round was wrong. A
 * "5,5" wall tile was added (absent from the brief) because case 4 needs
 * one and the given fixture had none.
 *
 * A second deviation, in case 1's own assertion, corrected in fix round 1
 * after review: `at` is a protobuf-es create()d GridPosition message, so it
 * carries a `$typeName` field toEqual has no tolerance for. The first pass
 * switched to `toMatchObject({ at: {x,y} })`, but that is a genuine
 * weakening — a subset match says nothing about `sceneId`, the one field
 * doorCommandFor computes non-trivially (via currentSceneId), so a bug that
 * sent the wrong scene would still pass. Fixed properly: round-trip through
 * toJson (which drops `$typeName` and every zero-valued field, same as the
 * wire), then toEqual the exact protojson shape. NOT "the pattern
 * commands.test.ts uses everywhere" (a claim two earlier rounds of this
 * comment got wrong in two different ways — first "everywhere", then "the
 * three below are the exception", and they are not: that file's exact-object
 * checks all go through its sameShape helper, four fixture tests among them.
 * What is true is narrower — an ad-hoc shape check there reaches for
 * toMatchObject, on toJson output or on the raw value with no round-trip at
 * all, as its "placeToken carries its geometry" test does — a test that
 * carried createScene too, and lost that half on 2026-09-02 when the command
 * left the platform).
 * The precise citation: this repo's own
 * openDoor/closeDoor/loadMap wire-shape tests from Task 1 —
 * "openDoor matches the client's own expected shape, scene and square in
 * order", "closeDoor matches the client's own expected shape, scene and
 * square in order", and "loadMap matches the client's own expected shape" —
 * whose own comment explains why those three specifically assert "the
 * EXACT object (toEqual, not toMatchObject)" — the same reason this one
 * does: a subset match would let a stray or transposed field (sceneId,
 * here) through unnoticed.
 */
function scene(opts: { open?: boolean } = {}): State {
  const st = newState();
  st.Scenes["scn"] = {
    ID: "scn", Name: "Cellar", GridWidth: 10, GridHeight: 9,
    Tiles: {
      "3,3": { Kind: "door", Material: "wood", Art: "" },
      "5,5": { Kind: "wall", Material: "stone", Art: "" },
    },
    OpenDoors: opts.open ? { "3,3": true } : {},
  };
  return st;
}

/**
 * scene() with one token added at (x, y), controlled by controllerId
 * (default "p-1", matching `player` below) in sceneId (default "scn", the
 * only scene scene() creates). Generalized past a single "the player's own
 * token" shape so it can also build the two refusal fixtures fix round 1
 * added: a token in a DIFFERENT scene (I2 — the same-scene filter was dead
 * code by test) and a token in the RIGHT scene that someone ELSE controls
 * (I3 — the control check was dead code by test).
 */
function sceneWithToken(
  x: number,
  y: number,
  opts: { sceneId?: string; controllerId?: string } = {},
): State {
  const st = scene();
  const controllerId = opts.controllerId ?? "p-1";
  st.Actors["a1"] = {
    actorId: "a1", name: "Hero", moduleId: "", attributes: {}, resources: {},
    controllerId, controllerIds: [controllerId], kind: ActorKind.PARTY_MEMBER, abilityIds: [],
  };
  st.Tokens["t1"] = { ID: "t1", SceneID: opts.sceneId ?? "scn", ActorID: "a1", X: x, Y: y };
  return st;
}

const player: Me = { participantId: "p-1", role: "player" } as Me;

// VTT-888
test("armed, a closed door opens", () => {
  const cmd = doorCommandFor(scene(), true, { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("openDoor");
  const j = toJson(ClientCommandSchema, cmd!) as Record<string, any>;
  expect(j["openDoor"]).toEqual({ sceneId: "scn", at: { x: 3, y: 3 } });
});

// VTT-888
test("armed, an open door closes — one control, both verbs", () => {
  const cmd = doorCommandFor(scene({ open: true }), true, { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("closeDoor");
});

// VTT-888
test("armed, cell is floor -> null", () => {
  // (0,0) carries no Tiles entry at all -- a sparse map's default, and legal
  // (state.ts: "Tiles may be empty and that is legal").
  const cmd = doorCommandFor(scene(), true, { x: 0, y: 0 });
  expect(cmd).toBeNull();
});

// VTT-888
test("armed, cell is wall -> null", () => {
  const cmd = doorCommandFor(scene(), true, { x: 5, y: 5 });
  expect(cmd).toBeNull();
});

test("NOT armed, cell is a closed door -> null (a move is not this module's business)", () => {
  const cmd = doorCommandFor(scene(), false, { x: 3, y: 3 });
  expect(cmd).toBeNull();
});

// VTT-881
test("player, no controlled token in scene -> no direction", () => {
  expect(doorDirection(scene(), player, "a1", { x: 3, y: 3 })).toBeUndefined();
});

// VTT-881
test("player, controlled token at distance 1 diagonally -> a direction", () => {
  // Door at (3,3), token at (4,4): dx=1, dy=1 -- the Chebyshev case. A
  // Manhattan distance (summing the deltas to 2) would reject this.
  const st = sceneWithToken(4, 4);
  expect(doorDirection(st, player, "a1", { x: 3, y: 3 })).toBe("north-west");
});

// VTT-881
test("player, controlled token at distance 2 -> no direction", () => {
  // Door at (3,3), token at (5,5): dx=2, dy=2.
  const st = sceneWithToken(5, 5);
  expect(doorDirection(st, player, "a1", { x: 3, y: 3 })).toBeUndefined();
});

// VTT-886
test("DM and agent get no direction, though a token of an actor they control stands beside the door", () => {
  // Both roles looped: the DM console arms their doors, and a direction
  // would draw a player's door button naming the chip's actor.
  for (const role of ["dm", "agent"] as const) {
    const me: Me = { participantId: "p-1", role } as Me;
    expect(doorDirection(sceneWithToken(4, 4), me, "a1", { x: 3, y: 3 })).toBeUndefined();
  }
});

// --- fix round 1: I2, I3, I5 ------------------------------------------------

// VTT-881
test("player, controlled token adjacent but in a DIFFERENT scene -> no direction", () => {
  // Same shape as internal/gateway/authz_test.go's
  // TestAuthorizePlayerDoorAdjacencyIgnoresOtherScenes: the token is at
  // (4,4), numerically adjacent to the door at (3,3) (dx=1, dy=1), so this
  // fails only because the scene filter runs -- without it, the distance
  // check alone would wrongly allow this. Kills the mutant that deletes
  // `if (tok.SceneID !== sceneId) return false;`.
  const st = sceneWithToken(4, 4, { sceneId: "a-different-scene" });
  expect(doorDirection(st, player, "a1", { x: 3, y: 3 })).toBeUndefined();
});

// VTT-881
test("player, a DIFFERENT participant's token is adjacent -> no direction", () => {
  // Same shape as authz_test.go's ownership tests: a token at (4,4),
  // adjacent to the door and in the right scene, but controlled by
  // "someone-else" rather than "p-1" (player's own id). Kills both the
  // `||` -> `&&` mutant and the mutant that deletes
  // `!actor.controllerIds.includes(me.participantId)` entirely -- either
  // mutation would let ANY adjacent token authorize ANY player.
  const st = sceneWithToken(4, 4, { controllerId: "someone-else" });
  expect(doorDirection(st, player, "a1", { x: 3, y: 3 })).toBeUndefined();
});

// VTT-881
test("spectator with an adjacent token -> no direction (fail closed)", () => {
  // Patrik's ruling, fix round 1: authz.go's commandRoles never routes
  // open_door/close_door to a spectator at all, so its Go mayWorkDoor's
  // `p.Role != identity.RolePlayer` early return is safe -- a spectator
  // never reaches it. This module has no such upstream gate and is the
  // only thing standing between a role and the control, so it must refuse
  // explicitly rather than default to "not a player, so yes". The token
  // here would satisfy the adjacency test for a PLAYER (same (4,4)/(3,3)
  // shape as the two tests above); only the role should refuse it.
  const st = sceneWithToken(4, 4);
  const spectator: Me = { participantId: "p-1", role: "spectator" } as Me;
  expect(doorDirection(st, spectator, "a1", { x: 3, y: 3 })).toBeUndefined();
});

// --- fix round 2: agent branch above; missing-actor and empty-scenes below -

// VTT-881
test("player, a token references an actor that no longer exists -> no direction", () => {
  // No fixture before this one ever left `st.Actors[tok.ActorID]`
  // undefined -- sceneWithToken always creates the actor it points the
  // token at. This one deliberately doesn't: the token below is in the
  // right scene, adjacent to the door (same (4,4)/(3,3) shape as the tests
  // above), but its ActorID ("ghost") names no actor at all. Kills the
  // mutant that deletes the `!actor ||` term from `if (!actor ||
  // !actor.controllerIds.includes(...))`: without that guard, the next
  // read (`actor.controllerIds`) throws on undefined `actor`, which fails
  // this test as surely as a wrong boolean would.
  const st = scene();
  st.Tokens["t-orphan"] = { ID: "t-orphan", SceneID: "scn", ActorID: "ghost", X: 4, Y: 4 };
  expect(doorDirection(st, player, "ghost", { x: 3, y: 3 })).toBeUndefined();
});

test("doorCommandFor with no scenes at all -> null", () => {
  // newState() alone: st.Scenes is empty, so currentSceneId's
  // `Object.keys(st.Scenes).sort().at(-1) ?? ""` falls through to its own
  // `?? ""`, and `st.Scenes[""]` is undefined. No prior fixture ever left
  // st.Scenes empty -- scene() always seeds "scn" -- so
  // `if (!scene) return null;` had nothing forcing it to run. Kills the
  // mutant that deletes that line: without it, the next read
  // (`scene.Tiles?.[key]`) throws on undefined `scene`.
  const cmd = doorCommandFor(newState(), true, { x: 3, y: 3 });
  expect(cmd).toBeNull();
});

// --- fix round 3: currentSceneId's .sort()/.at(-1), and the Tiles/OpenDoors
// optional-chaining reads in doorCommandFor. The `?? ""` fallback in
// currentSceneId is NOT tested here -- see that function's own doc comment
// in doors.ts for the equivalence argument (fold.ts's tokenPlaced arm makes
// st.Scenes empty imply st.Tokens empty, for every State this module is
// ever handed, so no fallback string is distinguishable from any other).

test("doorCommandFor resolves the LEXICOGRAPHICALLY GREATEST scene id, not the one inserted last", () => {
  // Insertion order is ["zzz", "aaa"] -- the reverse of sort order.
  // Object.keys preserves insertion order for non-numeric string keys, so
  // without .sort() (or reading .at(0) instead of .at(-1) after sorting),
  // currentSceneId would resolve to "aaa" here. "aaa" has no door at (3,3);
  // "zzz" does. Also the fixture that makes currentSceneId's own doc
  // comment ("the lexicographically greatest scene id") a claim something
  // actually tests -- every earlier fixture in this file has at most one
  // scene, so nothing before this could tell "greatest id" from "last
  // inserted" apart.
  const st = newState();
  st.Scenes["zzz"] = {
    ID: "zzz", Name: "Greatest id", GridWidth: 10, GridHeight: 9,
    Tiles: { "3,3": { Kind: "door", Material: "wood", Art: "" } },
    OpenDoors: {},
  };
  st.Scenes["aaa"] = {
    ID: "aaa", Name: "Inserted last", GridWidth: 10, GridHeight: 9,
    Tiles: {},
    OpenDoors: {},
  };
  const cmd = doorCommandFor(st, true, { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("openDoor");
  const j = toJson(ClientCommandSchema, cmd!) as Record<string, any>;
  expect(j["openDoor"]).toEqual({ sceneId: "zzz", at: { x: 3, y: 3 } });
});

test("armed, a scene with no Tiles map at all -> null, not a throw", () => {
  // A bare Scene literal omitting Tiles entirely -- legal (state.ts marks
  // it optional), and the same shape grid.test.ts's own scene literals
  // already use elsewhere in this test suite. No prior fixture in this file
  // omitted Tiles; scene() always sets it. Kills the mutant that turns
  // `scene.Tiles?.[key]` into `scene.Tiles![key]`: the `!` asserts non-null
  // to the type checker but performs no runtime check, so with Tiles
  // actually undefined the mutant throws instead of reading `undefined`.
  const st = newState();
  st.Scenes["bare"] = { ID: "bare", Name: "No terrain recorded", GridWidth: 4, GridHeight: 4 };
  const cmd = doorCommandFor(st, true, { x: 3, y: 3 });
  expect(cmd).toBeNull();
});

test("armed, a closed door in a scene with no OpenDoors map at all -> opens, not a throw", () => {
  // Tiles present (a door at (3,3)) but OpenDoors omitted entirely --
  // legal, and exactly the state fold.ts's own ensureOpenDoors comment
  // describes: "OpenDoors is optional on Scene ONLY to let bare Scene
  // literals elsewhere in this test suite keep compiling ... a Scene built
  // some other way might not have it". No prior fixture in this file
  // omitted OpenDoors; scene() always sets it (to {} or to {"3,3":true}).
  // Kills the mutant that turns `scene.OpenDoors?.[key]` into
  // `scene.OpenDoors![key]`: with OpenDoors actually undefined, the mutant
  // throws instead of reading `undefined` (falsy, so this correctly reads
  // as "closed" and opens the door).
  const st = newState();
  st.Scenes["undocumented"] = {
    ID: "undocumented", Name: "Doors never toggled", GridWidth: 4, GridHeight: 4,
    Tiles: { "3,3": { Kind: "door", Material: "iron", Art: "" } },
  };
  const cmd = doorCommandFor(st, true, { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("openDoor");
});

// VTT-876
test("the armed door command names no actor, though a token stands beside the door", () => {
  const cmd = doorCommandFor(sceneWithToken(4, 4), true, { x: 3, y: 3 });
  expect((toJson(ClientCommandSchema, cmd!) as Record<string, unknown>)["openDoor"]).toEqual({ sceneId: "scn", at: { x: 3, y: 3 } });
});

// --- a player's door buttons --------------------------------------------------

/** A token of a1 at (4,4) standing in an open door, a door on every neighbour,
 * the tiles listed out of the board's reading order. */
function ringOfDoors(): State {
  const st = sceneWithToken(4, 4);
  const door = { Kind: "door", Material: "wood", Art: "" };
  st.Scenes["scn"]!.Tiles = {};
  for (const key of ["5,5", "3,4", "4,3", "4,4", "3,5", "5,3", "4,5", "3,3", "5,4"]) st.Scenes["scn"]!.Tiles[key] = door;
  st.Scenes["scn"]!.OpenDoors = { "4,4": true, "5,3": true };
  return st;
}

const sent = (cmd: ClientCommand) => {
  const json = toJson(ClientCommandSchema, cmd) as Record<string, unknown>;
  delete json["requestId"];
  return json;
};

// VTT-881 VTT-882 VTT-887
test("a player's door buttons name each door's direction from the watched token, in the board's reading order", () => {
  expect(doorButtonsFor(ringOfDoors(), player, "a1").map((b) => b.label)).toEqual([
    "Open door (north-west)", "Open door (north)", "Close door (north-east)",
    "Open door (west)", "Close door (here)", "Open door (east)",
    "Open door (south-west)", "Open door (south)", "Open door (south-east)",
  ]);
});

// VTT-883
test("a closed door's button opens it and an open one's closes it, at its square, naming the watched character", () => {
  const byLabel = new Map(doorButtonsFor(ringOfDoors(), player, "a1").map((b) => [b.label, b.command()]));
  expect(sent(byLabel.get("Open door (south-east)")!)).toEqual({ openDoor: { sceneId: "scn", at: { x: 5, y: 5 }, actorId: "a1" } });
  expect(sent(byLabel.get("Close door (north-east)")!)).toEqual({ closeDoor: { sceneId: "scn", at: { x: 5, y: 3 }, actorId: "a1" } });
  expect(sent(byLabel.get("Close door (here)")!)).toEqual({ closeDoor: { sceneId: "scn", at: { x: 4, y: 4 }, actorId: "a1" } });
});

// VTT-881
test("no button for a door two squares off, a wall, a door of another scene, or a door beside another's token", () => {
  expect(doorButtonsFor(sceneWithToken(5, 4), player, "a1")).toEqual([]);
  expect(doorButtonsFor(sceneWithToken(5, 3), player, "a1")).toEqual([]);
  expect(doorButtonsFor(sceneWithToken(3, 5), player, "a1")).toEqual([]);
  expect(doorButtonsFor(sceneWithToken(4, 4), player, "a1").map((b) => b.label)).toEqual(["Open door (north-west)"]);
  const elsewhere = sceneWithToken(4, 4, { sceneId: "other" });
  expect(doorButtonsFor(elsewhere, player, "a1")).toEqual([]);
  expect(doorButtonsFor(sceneWithToken(4, 4, { controllerId: "p-2" }), player, "a1")).toEqual([]);
});

// VTT-881
test("a door beside another of the player's characters gets no button for the one they watch", () => {
  const st = sceneWithToken(8, 7);
  st.Actors["a2"] = { ...st.Actors["a1"]!, actorId: "a2", name: "Second" };
  st.Tokens["t2"] = { ID: "t2", SceneID: "scn", ActorID: "a2", X: 2, Y: 2 };
  expect(doorButtonsFor(st, player, "a1")).toEqual([]);
  expect(doorButtonsFor(st, player, "a2").map((b) => b.label)).toEqual(["Open door (south-east)"]);
});

// VTT-882
test("a scene with no OpenDoors reads its door closed", () => {
  const st = sceneWithToken(3, 4);
  delete (st.Scenes["scn"] as { OpenDoors?: unknown }).OpenDoors;
  expect(doorButtonsFor(st, player, "a1").map((b) => b.label)).toEqual(["Open door (north)"]);
});

// VTT-886
test("a DM and an agent get no door button, whatever actor they ask for", () => {
  for (const role of ["dm", "agent"] as const) {
    const me: Me = { participantId: "p-1", role } as Me;
    expect(doorButtonsFor(sceneWithToken(4, 4), me, "a1")).toEqual([]);
  }
});

// VTT-881
test("no scene, no door button", () => {
  const st = newState();
  st.Actors["a1"] = sceneWithToken(4, 4).Actors["a1"]!;
  expect(doorButtonsFor(st, player, "a1")).toEqual([]);
});
