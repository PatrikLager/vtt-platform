import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { newState, type State, type Tile } from "../src/state";
import { fitCamera } from "../src/view/camera";
import { planFog, planScene } from "../src/view/scene-plan";
import { fogInk, shadeFog } from "../src/view/canvas";
import { tokensOnScene } from "../src/view/grid";
import { renderSpectator } from "../src/view/spectator";

// TASK 7: the board draws only what this seat may know.
//
// Everything asserted here is GEOMETRY OR DOM, never pixels. happy-dom has no
// canvas implementation at all (canvas.ts's header records what that once
// cost: the participant list shipped rendering as "ArmakAsmeDM" behind a
// passing test), so a test that reached for a rendered image would be
// asserting nothing. planFog returns rects a test can read, and token discs
// are real DOM nodes.
//
// EVERY VISIBILITY TEST HERE USES A PLAYER-SHAPED STATE — one whose scenes
// carry Visible, because they were folded from a projection. A DM's stream is
// the identity function and carries no sceneSeen at all, so a DM seat can
// catch no projection bug; the DM cases below are present to pin the OTHER
// direction, that nothing this task adds blanks their board.

function env(seq: number, payload: Record<string, unknown>): Envelope {
  return fromJson(EnvelopeSchema, {
    eventId: `evt-${seq}`,
    sequence: String(seq),
    sessionId: "sess-1",
    actorRole: "dm",
    participantId: "p-dm",
    ...payload,
  } as never);
}

const floor = { kind: "floor", material: "stone", art: "" };

/**
 * A 3x3 room a player has walked through: the whole middle row was seen at
 * some point, and only (1,1) is in sight now. So (0,1) and (2,1) are
 * remembered-but-unseen — the fog — and every square of rows 0 and 2 was never
 * sent at all, which is what unexplored looks like on this client.
 */
function walkedThrough(): Envelope[] {
  return [
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { sceneCreated: { sceneId: "s1", name: "Room", gridWidth: 3, gridHeight: 3 } }),
    env(3, {
      sceneSeen: {
        sceneId: "s1",
        tiles: { "0,1": floor, "1,1": floor, "2,1": floor },
      },
    }),
    env(4, { sceneSeen: { sceneId: "s1", tiles: { "1,1": floor } } }),
  ];
}

// --- the fold: Visible is replaced, Explored is not -------------------------

test("sceneSeen REPLACES the visible set and unions into the explored one", () => {
  // The two halves of one arm pulling opposite ways, which is the whole reason
  // both fields exist. Mirrors internal/engine's
  // TestSceneSeenReplacesTheVisibleSetWithoutForgettingTheExploredOne.
  const sc = fold(walkedThrough()).Scenes["s1"]!;
  expect(sc.Visible).toEqual({ "1,1": true });
  expect(sc.Explored).toEqual({ "0,1": true, "1,1": true, "2,1": true });
});

test("an empty sceneSeen darkens the scene and forgets no terrain", () => {
  // This is the wire message internal/gateway sends when a seat can no longer
  // see anything of a scene it has been in. It must land as "you see nothing
  // here now", never as "you were never here" — the second would erase a
  // player's map every time they walked out of a room.
  const st = fold([...walkedThrough(), env(5, { sceneSeen: { sceneId: "s1" } })]);
  const sc = st.Scenes["s1"]!;

  expect(sc.Visible).toEqual({});
  // Not undefined: a seat that has received a projection must stay
  // distinguishable from one that never has (state.ts), because the second
  // draws everything.
  expect(sc.Visible).toBeDefined();
  expect(sc.Explored).toEqual({ "0,1": true, "1,1": true, "2,1": true });
  // And the terrain itself survives, so there is still something to fog.
  expect(sc.Tiles!["0,1"]).toEqual({ Kind: "floor", Material: "stone", Art: "" });
});

test("a scene that has never received a sceneSeen has NO visible set at all", () => {
  // The DM and the agent, whose stream is the identity projection. Undefined
  // rather than {} is what stops the board blanking for them.
  const sc = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { sceneCreated: { sceneId: "s1", name: "Room", gridWidth: 3, gridHeight: 3 } }),
  ]).Scenes["s1"]!;
  expect(sc.Visible).toBeUndefined();
  expect(sc.Explored).toEqual({});
});

test("Visible reaches the dump when populated, and is OMITTED (not {}) when empty", () => {
  // foldToDumpJSON is the half the Go/TS keystone actually diffs, and Go's
  // Scene.Visible carries `json:",omitempty"`, so json.Marshal drops the key
  // for nil AND for empty. Emitting `Visible: {}` unconditionally here looks
  // harmless and fails every scenarios/goldens/*/state.json byte comparison,
  // none of which was derived from a stream containing sceneSeen — the same
  // trap Explored fell into in fix-round-1.
  const dumped = JSON.parse(foldToDumpJSON(walkedThrough()));
  expect(dumped.Scenes["s1"].Visible).toEqual({ "1,1": true });

  const darkened = JSON.parse(
    foldToDumpJSON([...walkedThrough(), env(5, { sceneSeen: { sceneId: "s1" } })]),
  );
  expect("Visible" in darkened.Scenes["s1"]).toBe(false);

  const neverProjected = JSON.parse(
    foldToDumpJSON([
      env(1, { sessionStarted: { name: "S" } }),
      env(2, { sceneCreated: { sceneId: "s1", name: "R", gridWidth: 1, gridHeight: 1 } }),
    ]),
  );
  expect("Visible" in neverProjected.Scenes["s1"]).toBe(false);
});

// --- planFog: Explored − Visible, and nothing else --------------------------

/** The camera every fog case below uses: 3x3 at cell 44, fitted to a pane
 *  big enough that nothing is culled, so a missing rect means "not fogged"
 *  rather than "off screen". */
const cam3 = fitCamera(3, 3, 44, 132, 132);

test("fog covers remembered ground, and ONLY remembered ground", () => {
  const st = fold(walkedThrough());
  const fog = planFog(st, "s1", cam3, 44, 132, 132);

  // (0,1) and (2,1): walked through, out of sight now.
  expect(fog).toHaveLength(2);
  expect(fog.map((r) => `${r.x},${r.y}`).sort()).toEqual(["0,44", "88,44"]);
  // The square in sight is NOT fogged...
  expect(fog.some((r) => r.x === 44 && r.y === 44)).toBe(false);
  // ...and neither is any square of rows 0 and 2, which were never sent at
  // all. Unexplored is the ABSENCE of anything drawn, not a heavier fog: the
  // server redacted that terrain, so this client has nothing to conceal.
  expect(fog.some((r) => r.y !== 44)).toBe(false);
});

test("a fog rect covers its whole square, through the camera", () => {
  // The geometry has to agree with planScene's tiles exactly, or the fog would
  // sit half a square off the ground it is dimming.
  const st = fold(walkedThrough());
  const fog = planFog(st, "s1", cam3, 44, 132, 132);
  const ops = planScene(st, "s1", cam3, 44, 132, 132);

  const at01 = fog.find((r) => r.x === 0 && r.y === 44)!;
  const tile01 = ops.find((o) => o.sx === 0 && o.sy === 44)!;
  expect([at01.w, at01.h]).toEqual([tile01.sw, tile01.sh]);
});

test("fog is culled to the viewport, like every other plan in this file", () => {
  // A 200x200 scene is 40,000 squares and canvas.ts cannot tell which the
  // camera can see; if the planner does not filter, nothing downstream will.
  const st = fold(walkedThrough());
  const panned = { ...cam3, offsetX: -1000 };
  expect(planFog(st, "s1", panned, 44, 132, 132)).toHaveLength(0);
});

test("a DM's board fogs nothing, with no role check anywhere", () => {
  // Their stream is the identity projection: no sceneSeen, so no Visible. This
  // is why the board needs no notion of who is looking at it.
  //
  // Explored is POPULATED here on purpose, and that is what makes the test bite
  // rather than pass for a reason in another file. A real DM stream leaves
  // Explored empty too, so a `Visible ?? {}` version of planFog also returns []
  // on the honest fixture — the empty answer would then be a fact about
  // fold.ts's sceneCreated arm, not about this function. Filling Explored
  // separates the two: with the guard the answer is still no fog, and without
  // it every explored square greys over.
  const st = newState();
  const tiles: Record<string, Tile> = {};
  const explored: Record<string, boolean> = {};
  for (let y = 0; y < 3; y++) {
    for (let x = 0; x < 3; x++) {
      tiles[`${x},${y}`] = { Kind: "floor", Material: "stone", Art: "" };
      explored[`${x},${y}`] = true;
    }
  }
  st.Scenes["s1"] = {
    ID: "s1", Name: "Room", GridWidth: 3, GridHeight: 3,
    Tiles: tiles, Objects: [], OpenDoors: {}, Explored: explored,
  };
  expect(st.Scenes["s1"]!.Visible).toBeUndefined();
  expect(planFog(st, "s1", cam3, 44, 132, 132)).toHaveLength(0);
});

test("planFog on a scene that is not there returns nothing rather than throwing", () => {
  expect(planFog(newState(), "nope", cam3, 44, 132, 132)).toEqual([]);
});

// --- shadeFog: the thin fill, and the colour it fills with -------------------

function fogCtx(calls: string[]): CanvasRenderingContext2D {
  return {
    fillStyle: "",
    save() {},
    restore() {},
    fillRect(x: number, y: number, w: number, h: number) {
      calls.push(`fillRect:${(this as { fillStyle: string }).fillStyle}:${x},${y},${w},${h}`);
    },
  } as unknown as CanvasRenderingContext2D;
}

test("shadeFog fills one rect per region, in the fog's own ink", () => {
  const calls: string[] = [];
  shadeFog(fogCtx(calls), [
    { x: 0, y: 44, w: 44, h: 44 },
    { x: 88, y: 44, w: 44, h: 44 },
  ]);
  expect(calls).toEqual([
    `fillRect:${fogInk}:0,44,44,44`,
    `fillRect:${fogInk}:88,44,44,44`,
  ]);
});

test("shadeFog draws nothing at all when there is no fog", () => {
  const calls: string[] = [];
  shadeFog(fogCtx(calls), []);
  expect(calls).toEqual([]);
});

// --- tokensOnScene: the visible set is INPUT --------------------------------

/** A player-shaped state: one scene whose middle row is explored, only (1,1)
 *  visible, with a goblin standing on remembered ground at (2,1) and the
 *  player's own actor at (1,1). */
function playerBoard(): State {
  const st = fold(walkedThrough());
  st.Actors["hero"] = {
    actorId: "hero", name: "Lera", moduleId: "", attributes: {}, resources: {},
    controllerId: "p-me", controllerIds: ["p-me"],
  };
  st.Actors["goblin"] = {
    actorId: "goblin", name: "Goblin", moduleId: "", attributes: {}, resources: {},
    controllerId: "", controllerIds: [],
  };
  st.Tokens["t-hero"] = { ID: "t-hero", SceneID: "s1", ActorID: "hero", X: 1, Y: 1 };
  st.Tokens["t-gob"] = { ID: "t-gob", SceneID: "s1", ActorID: "goblin", X: 2, Y: 1 };
  return st;
}

test("a token on remembered-but-unseen ground produces NO disc at all", () => {
  // THE NEGATIVE IS THE TEST THAT MATTERS (spec §6.1, §7): not a disc
  // something later declines to paint — none. You remember the room, not the
  // goblin standing in it.
  const st = playerBoard();
  const ids = tokensOnScene(st, "s1", st.Scenes["s1"]!.Visible).map((d) => d.tokenId);
  expect(ids).toEqual(["t-hero"]);
});

test("a token on a LIT square that declares no terrain silently loses its disc", () => {
  // THE CLIENT MIRROR of internal/gateway's
  // TestAVisibleSquareWithNoTerrainIsAbsentFromTheVisibleSet: a 3x3 room that
  // declares floor on EVERY square but 1,1, folded in the order and shape the
  // projection actually emits — the redacted sceneCreated first, then the
  // actors, then the tokens, and sceneSeen LAST (it is the final loop in
  // transitions). The sceneSeen below therefore carries all eight squares that
  // declare terrain and omits only 1,1, which is exactly the 8 -> 7 the Go test
  // measures. The hero stands on the one square that cannot be reported.
  //
  // WHAT THIS TEST CAN AND CANNOT OBSERVE, because the previous version of it
  // claimed more than it had. It is a mirror in SHAPE, not a check on the
  // server: the sceneSeen is written here, so no change to sceneSeenFor can
  // move it — the gateway test is the pin for what the projection emits. What
  // this pins is the half that lives on this side: that the fold turns such a
  // message into a hole in all three maps, and that a token standing in that
  // hole draws nothing.
  //
  // PINNING TODAY'S BEHAVIOUR, not endorsing it — the ruling is Patrik's, and
  // this is what makes either answer a deliberate change. Neither shipped
  // adventure can reach it: both declare a tile for every square.
  const lit: Record<string, unknown> = {};
  for (let y = 0; y < 3; y++) {
    for (let x = 0; x < 3; x++) {
      if (x === 1 && y === 1) continue; // the square with no terrain to send
      lit[`${x},${y}`] = floor;
    }
  }
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { sceneCreated: { sceneId: "s1", name: "Room", gridWidth: 3, gridHeight: 3 } }),
    env(3, { actorAdded: { actor: { actorId: "hero", name: "Hero" } } }),
    env(4, { actorAdded: { actor: { actorId: "goblin", name: "Goblin" } } }),
    env(5, { tokenPlaced: { tokenId: "t-hero", sceneId: "s1", actorId: "hero", position: { x: 1, y: 1 } } }),
    env(6, { tokenPlaced: { tokenId: "t-gob", sceneId: "s1", actorId: "goblin", position: { x: 2, y: 1 } } }),
    env(7, { sceneSeen: { sceneId: "s1", tiles: lit } }),
  ]);
  const sc = st.Scenes["s1"]!;

  // Eight of nine squares, and the hero's own is in NONE of the three maps —
  // which is the inference itself, and why this client cannot tell "you cannot
  // see that square" from "that square declares no terrain".
  expect(Object.keys(sc.Visible!)).toHaveLength(8);
  expect(sc.Visible!["1,1"]).toBeUndefined();
  expect(sc.Explored!["1,1"]).toBeUndefined();
  expect(sc.Tiles!["1,1"]).toBeUndefined();
  // The control: the square beside it was reported and is whole.
  expect(sc.Visible!["2,1"]).toBe(true);

  // So the goblin at 2,1 IS drawn — the board is working, seven eighths of it
  // lit — and the hero at 1,1, standing in plain sight, is not.
  expect(tokensOnScene(st, "s1", sc.Visible).map((d) => d.tokenId)).toEqual(["t-gob"]);
});

test("a seat that can currently see nothing draws no token, including its own", () => {
  const st = playerBoard();
  expect(tokensOnScene(st, "s1", {})).toEqual([]);
});

test("undefined is not an empty set: a stream with no projection draws every token", () => {
  // The DM and the agent. Conflating undefined with {} would blank their board
  // entirely, which is the failure this distinction exists to prevent.
  const st = playerBoard();
  const ids = tokensOnScene(st, "s1", undefined).map((d) => d.tokenId);
  expect(ids).toEqual(["t-gob", "t-hero"]);
});

// --- the wiring: what renderSpectator actually does with all of it ----------

/** Records drawImage, fillRect and stroke in call order — the three marks
 *  whose ORDER is the assertion — and nothing else. */
function orderingCtx(calls: string[]): CanvasRenderingContext2D {
  return {
    fillStyle: "", strokeStyle: "", lineWidth: 0,
    save() {}, restore() {}, translate() {}, rotate() {},
    beginPath() {}, moveTo() {}, lineTo() {},
    drawImage() { calls.push("drawImage"); },
    fillRect() { calls.push("fillRect"); },
    stroke() { calls.push("stroke"); },
  } as unknown as CanvasRenderingContext2D;
}

/** Every key resolves to itself, so terrain draws via drawImage rather than
 *  hitting canvas.ts's missing-tile marker — which also fills rects, and
 *  would make the fog assertion below unable to tell the two apart. */
function stubImages(): Record<string, CanvasImageSource> {
  return new Proxy({}, { get: (_t, prop: string) => prop }) as unknown as Record<
    string,
    CanvasImageSource
  >;
}

test("renderSpectator shades fog AFTER the terrain and BEFORE the grid", () => {
  // The production wiring, which is the only place the order exists. Under
  // happy-dom canvas.getContext("2d") always returns null, so this seam (an
  // optional getContext override, never set by app.ts) is the only way any
  // test reaches the code that actually calls these three in sequence.
  const calls: string[] = [];
  const root = document.createElement("div");
  renderSpectator(root, playerBoard(), [], "connected", {
    images: stubImages(),
    getContext: () => orderingCtx(calls),
  });

  const lastTerrain = calls.lastIndexOf("drawImage");
  const fog = calls.indexOf("fillRect");
  const grid = calls.indexOf("stroke");
  expect(lastTerrain).toBeGreaterThanOrEqual(0);
  expect(fog).toBeGreaterThanOrEqual(0);
  expect(grid).toBeGreaterThanOrEqual(0);
  expect(fog).toBeGreaterThan(lastTerrain);
  expect(grid).toBeGreaterThan(fog);
});

test("a player's board carries no disc for a creature on ground they merely remember", () => {
  // The whole task, end to end and in the DOM: the goblin at (2,1) is standing
  // in a room this seat has walked out of.
  const root = document.createElement("div");
  renderSpectator(root, playerBoard(), [], "connected");

  const ids = Array.from(root.querySelectorAll(".token")).map((n) => (n as HTMLElement).dataset["tokenId"]);
  expect(ids).toEqual(["t-hero"]);
});

test("the same board rendered for a stream with no projection keeps every disc", () => {
  // The DM direction, and the reason it has to be asserted: a board that
  // simply drew nothing would pass the test above for the wrong reason.
  const st = playerBoard();
  delete st.Scenes["s1"]!.Visible;
  delete st.Scenes["s1"]!.Explored;
  const root = document.createElement("div");
  renderSpectator(root, st, [], "connected");

  const ids = Array.from(root.querySelectorAll(".token")).map((n) => (n as HTMLElement).dataset["tokenId"]);
  expect(ids).toEqual(["t-gob", "t-hero"]);
});
