import { test, expect } from "bun:test";
import { newState, type State, type Tile } from "../src/state";
import { fitCamera } from "../src/view/camera";
import { planScene } from "../src/view/scene-plan";

/**
 * cellarState builds a 3x3 scene directly (no fold/envelopes needed --
 * planScene only ever reads State). Every square carries an Art override so
 * every op resolves through the "tile:" branch rather than "std:", which is
 * what the culling test below needs to count cleanly; internal/adventure's
 * real cellar-adv fixture overrides only its centre square (1,1) the same
 * way, which is the one square this file's "override" test also checks.
 */
function cellarState(): State {
  const st = newState();
  const tiles: Record<string, Tile> = {};
  for (let y = 0; y < 3; y++) {
    for (let x = 0; x < 3; x++) {
      const isWall = y === 0 || y === 2;
      tiles[`${x},${y}`] = isWall
        ? { Kind: "wall", Material: "stone", Art: "stone-block-mossy" }
        : { Kind: "floor", Material: "wood", Art: x === 1 ? "planks-split-3" : "planks-basic" };
    }
  }
  st.Scenes["cellar"] = {
    ID: "cellar",
    Name: "The Cellar",
    GridWidth: 3,
    GridHeight: 3,
    Tiles: tiles,
    Objects: [],
    OpenDoors: {},
  };
  return st;
}

test("planScene emits one op per visible square and culls the rest", () => {
  const st = cellarState(); // 3x3
  const cam = fitCamera(3, 3, 44, 132, 132);
  const ops = planScene(st, "cellar", cam, 44, 132, 132);
  expect(ops.filter((o) => o.image.startsWith("tile:")).length).toBe(9);

  // Half the map off-screen: culled, not drawn and clipped.
  const panned = { ...cam, offsetX: -88 };
  const fewer = planScene(st, "cellar", panned, 44, 132, 132);
  expect(fewer.length).toBeLessThan(ops.length);
});

test("an override changes the image and nothing else", () => {
  const st = cellarState();
  const ops = planScene(st, "cellar", fitCamera(3, 3, 44, 132, 132), 44, 132, 132);
  const at11 = ops.find((o) => o.sx === 44 && o.sy === 44)!;
  expect(at11.image).toBe("tile:planks-split-3");
  expect(at11.sw).toBe(44);
  expect(at11.sh).toBe(44);
  expect(at11.rot).toBe(0);
});

test("a square with no override falls back to the standard kind/material picture", () => {
  const st = cellarState();
  st.Scenes["cellar"]!.Tiles!["0,0"] = { Kind: "wall", Material: "stone", Art: "" };
  const ops = planScene(st, "cellar", fitCamera(3, 3, 44, 132, 132), 44, 132, 132);
  const at00 = ops.find((o) => o.sx === 0 && o.sy === 0)!;
  expect(at00.image).toBe("std:wall/stone");
});

test("a door's picture reflects open/closed state; closed is the unmarked default", () => {
  // Doors carry no open/closed field in the wire tile (design spec §3.3) --
  // openness is folded state, read here from Scene.OpenDoors, not from Art.
  const st = newState();
  st.Scenes["vault"] = {
    ID: "vault",
    Name: "Vault",
    GridWidth: 1,
    GridHeight: 1,
    Tiles: { "0,0": { Kind: "door", Material: "wood", Art: "" } },
    Objects: [],
    OpenDoors: {},
  };
  const cam = fitCamera(1, 1, 44, 44, 44);

  const closed = planScene(st, "vault", cam, 44, 44, 44);
  expect(closed[0]!.image).toBe("std:door/wood");

  st.Scenes["vault"]!.OpenDoors = { "0,0": true };
  const open = planScene(st, "vault", cam, 44, 44, 44);
  expect(open[0]!.image).toBe("std:door/wood/open");
});

test("objects draw with their footprint size and rotation converted to radians", () => {
  // Canvas's rotate() takes radians (Task 9 is a thin loop calling it
  // directly), so the degrees-to-radians conversion is a decision made HERE,
  // not left for the untestable canvas layer to get wrong unnoticed.
  const st = newState();
  st.Scenes["hall"] = {
    ID: "hall",
    Name: "Hall",
    GridWidth: 4,
    GridHeight: 4,
    Tiles: {},
    Objects: [
      {
        ObjectID: "o1",
        Kind: "boulder",
        X: 1,
        Y: 1,
        Width: 2,
        Height: 1,
        RotationDegrees: 90,
        BlocksSight: true,
        BlocksMove: true,
        Art: "boulder-mossy-2",
      },
    ],
    OpenDoors: {},
  };
  const cam = fitCamera(4, 4, 44, 176, 176); // exact fit: scale 1, offset 0
  const ops = planScene(st, "hall", cam, 44, 176, 176);
  const obj = ops.find((o) => o.image === "tile:boulder-mossy-2")!;
  expect(obj).toBeDefined();
  expect(obj.sx).toBe(44);
  expect(obj.sy).toBe(44);
  expect(obj.sw).toBe(88); // 2 squares wide
  expect(obj.sh).toBe(44);
  expect(obj.rot).toBeCloseTo(Math.PI / 2, 6);
});

test("an object entirely off the panned viewport is culled like a tile is", () => {
  const st = newState();
  st.Scenes["hall"] = {
    ID: "hall",
    Name: "Hall",
    GridWidth: 4,
    GridHeight: 4,
    Tiles: {},
    Objects: [
      {
        ObjectID: "o1", Kind: "boulder", X: 0, Y: 0, Width: 1, Height: 1,
        RotationDegrees: 0, BlocksSight: false, BlocksMove: false, Art: "boulder-mossy-2",
      },
    ],
    OpenDoors: {},
  };
  const cam = fitCamera(4, 4, 44, 176, 176);
  const onscreen = planScene(st, "hall", cam, 44, 176, 176);
  expect(onscreen.some((o) => o.image === "tile:boulder-mossy-2")).toBe(true);

  const pannedAway = { ...cam, offsetX: cam.offsetX - 1000 };
  const offscreen = planScene(st, "hall", pannedAway, 44, 176, 176);
  expect(offscreen.some((o) => o.image === "tile:boulder-mossy-2")).toBe(false);
});

test("a scene with no terrain at all produces no draw ops, not a crash", () => {
  // Tiles/Objects/OpenDoors are OPTIONAL on Scene (state.ts) precisely so a
  // bare four-field Scene literal -- legal, per Patrik's ruling 2026-08-13 --
  // keeps compiling. planScene must default rather than special-case this.
  const st = newState();
  st.Scenes["blank"] = { ID: "blank", Name: "Blank", GridWidth: 5, GridHeight: 5 };
  const cam = fitCamera(5, 5, 44, 220, 220);
  expect(planScene(st, "blank", cam, 44, 220, 220)).toEqual([]);
});

test("an unknown scene id returns no ops rather than throwing", () => {
  const st = newState();
  expect(planScene(st, "nowhere", fitCamera(1, 1, 44, 44, 44), 44, 44, 44)).toEqual([]);
});
