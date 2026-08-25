import { test, expect } from "bun:test";
import { newState, type State, type Tile } from "../src/state";
import { fitCamera, type Camera } from "../src/view/camera";
import { planFog, planScene } from "../src/view/scene-plan";

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

// --- the camera transform, on a camera that can actually show it wrong ------

/**
 * skewCam is the camera every transform case below uses, and all three of its
 * components are deliberately off their identity values.
 *
 * The transform is `index * cell * scale + offset` per axis, and AT scale 1 /
 * offset 0 MOST OF IT CANNOT BE OBSERVED AT ALL: `n * 1` and `n / 1` agree,
 * `n + 0` and `n - 0` agree. Every other test in this file fits the camera to
 * the pane -- rightly, since that is what the app does -- and a fitted 3x3 at
 * cell 44 in 132x132 is exactly scale 1, offset 0, so those tests read back
 * identical whether this file multiplies or divides by the scale and whether
 * it adds or subtracts the offset. This camera reads differently for each.
 *
 * offsetX and offsetY differ from each other, and the squares asserted below
 * have gx != gy on a NON-SQUARE grid, so the other half of this family --
 * an x formula quietly applied to the y axis -- cannot read back correct
 * either. That transposition is not hypothetical: tokens drew unscaled over
 * scaled terrain earlier on this branch, which is the same class of bug one
 * axis over.
 */
const skewCam: Camera = { scale: 0.5, offsetX: 30, offsetY: 7 };

/**
 * A 3-wide, 2-tall room -- deliberately not square, so a width read as a
 * height shows up -- where every square carries its own art. Naming a square
 * by its picture rather than by its coordinates lets a test read back where
 * the camera put it without the lookup already assuming the answer.
 */
function room3x2(): State {
  const st = newState();
  const tiles: Record<string, Tile> = {};
  for (let y = 0; y < 2; y++) {
    for (let x = 0; x < 3; x++) {
      tiles[`${x},${y}`] = { Kind: "floor", Material: "stone", Art: `sq-${x}-${y}` };
    }
  }
  st.Scenes["room"] = {
    ID: "room",
    Name: "Room",
    GridWidth: 3,
    GridHeight: 2,
    Tiles: tiles,
    Objects: [],
    OpenDoors: {},
  };
  return st;
}

test("a square lands where the camera puts it: scaled by the scale, then offset", () => {
  const ops = planScene(room3x2(), "room", skewCam, 40, 300, 200);
  const byArt = new Map(ops.map((o) => [o.image, o]));

  // (2,1) at cell 40 through scale 0.5 / offset (30,7):
  //   sx = 2 * 40 * 0.5 + 30 = 70   sy = 1 * 40 * 0.5 + 7 = 27   side = 20.
  // Divide by the scale instead and sx is 190; subtract the offset and it is
  // 10; compute sy from gx and it is 47. Each of the four numbers is a
  // separate reading of a separate piece of the transform.
  expect(byArt.get("tile:sq-2-1")).toEqual({
    image: "tile:sq-2-1",
    sx: 70,
    sy: 27,
    sw: 20,
    sh: 20,
    rot: 0,
  });
  // A square on the same row but at gx 0, so "the x term is the one that
  // vanishes when gx is 0" is pinned rather than inferred: sx is the bare
  // offset here, and sy is unchanged from the reading above.
  expect(byArt.get("tile:sq-0-1")).toEqual({
    image: "tile:sq-0-1",
    sx: 30,
    sy: 27,
    sw: 20,
    sh: 20,
    rot: 0,
  });
});

test("GridWidth and GridHeight bound what is drawn: a square keyed past them is not", () => {
  // The scene's declared size is what the map IS -- planGrid rules exactly
  // that rectangle and nothing else, so terrain painted outside it would sit
  // on unruled ground beside the map. The keys are strings arriving from the
  // server (sceneCreated's tile map, and sceneSeen's), so "3,0" on a 3-wide
  // scene is a shape this function can be handed; walking one row or column
  // too far is how it would be drawn, and nothing downstream would object.
  const st = room3x2(); // 3 wide, 2 tall -- the last legal key is "2,1"
  const tiles = st.Scenes["room"]!.Tiles!;
  tiles["3,0"] = { Kind: "floor", Material: "stone", Art: "past-the-right-edge" };
  tiles["0,2"] = { Kind: "floor", Material: "stone", Art: "past-the-bottom-edge" };

  const ops = planScene(st, "room", skewCam, 40, 300, 200);
  // Both strays would land well inside this 300x200 pane if they were walked
  // (sx 90 / sy 47), so their absence is the bound doing the work and not the
  // viewport cull doing it by accident.
  expect(ops).toHaveLength(6);
  expect(ops.some((o) => o.image === "tile:past-the-right-edge")).toBe(false);
  expect(ops.some((o) => o.image === "tile:past-the-bottom-edge")).toBe(false);
});

test("an object's rect rides the same transform, with width and height kept apart", () => {
  // Objects repeat the tile transform against their own X/Y and add two more
  // scalings for the footprint. The existing rotation test above uses an exact
  // fit (scale 1, offset 0), where a footprint multiplied by the scale and one
  // divided by it are the same number -- so the footprint arithmetic needs
  // this camera, and a 3x1 footprint so a width read as a height shows.
  const st = newState();
  st.Scenes["room"] = {
    ID: "room",
    Name: "Room",
    GridWidth: 3,
    GridHeight: 2,
    Tiles: {},
    Objects: [
      {
        ObjectID: "o1", Kind: "crate", X: 2, Y: 1, Width: 3, Height: 1,
        RotationDegrees: 90, BlocksSight: true, BlocksMove: true, Art: "crate-stack",
      },
    ],
    OpenDoors: {},
  };

  const ops = planScene(st, "room", skewCam, 40, 300, 200);
  const crate = ops.find((o) => o.image === "tile:crate-stack");
  expect(crate).toBeDefined();
  //   sx = 2 * 40 * 0.5 + 30 = 70   sy = 1 * 40 * 0.5 + 7 = 27
  //   sw = 3 * 40 * 0.5 = 60        sh = 1 * 40 * 0.5 = 20
  expect([crate!.sx, crate!.sy, crate!.sw, crate!.sh]).toEqual([70, 27, 60, 20]);
  expect(crate!.rot).toBeCloseTo(Math.PI / 2, 6);
});

// --- the viewport cull, read at its four edges -------------------------------

/**
 * A single 1x1 object at the scene origin. Every edge case below moves the
 * CAMERA rather than the object, which keeps the arithmetic in the test to
 * one line -- at cell 10 and scale 1 the rect is exactly (offsetX, offsetY,
 * 10, 10) -- so each case can say plainly which screen edge it is sitting on.
 */
function loneCrate(): State {
  const st = newState();
  st.Scenes["edge"] = {
    ID: "edge",
    Name: "Edge",
    GridWidth: 4,
    GridHeight: 4,
    Tiles: {},
    Objects: [
      {
        ObjectID: "o1", Kind: "crate", X: 0, Y: 0, Width: 1, Height: 1,
        RotationDegrees: 0, BlocksSight: false, BlocksMove: false, Art: "lone-crate",
      },
    ],
    OpenDoors: {},
  };
  return st;
}

/** Is the lone crate drawn at all, with its top-left corner at (ox, oy)? */
function crateDrawn(ox: number, oy: number): boolean {
  // 100x80, not a square pane: an x test applied to the y axis then reads
  // back differently at the far edges instead of agreeing by coincidence.
  const ops = planScene(loneCrate(), "edge", { scale: 1, offsetX: ox, offsetY: oy }, 10, 100, 80);
  return ops.some((o) => o.image === "tile:lone-crate");
}

test("the viewport cull is exclusive at all four edges: touching is not overlapping", () => {
  // WHAT THIS PINS IS THE NEGATIVE. Culling is a correctness requirement
  // (planScene's own doc comment: 40,000 squares, and canvas.ts cannot tell
  // which of them the camera can see), and a cull is only ever visible in the
  // things it does NOT emit -- a suite whose fixtures all sit comfortably
  // on-screen agrees with a cull that has stopped culling entirely.
  //
  // Each edge gets a rect EXACTLY on it and a rect one pixel inside it,
  // because the two answers on either side of a boundary are the only pair
  // that says where the boundary is. A rect whose right edge is exactly at
  // x = 0 shares a line with the viewport and covers none of it: nothing to
  // draw. One pixel further in and there is a pixel to draw.
  expect(crateDrawn(-10, 0)).toBe(false); // right edge exactly on x = 0
  expect(crateDrawn(-9, 0)).toBe(true); //  ...one pixel of it inside
  expect(crateDrawn(100, 0)).toBe(false); // left edge exactly on x = viewW
  expect(crateDrawn(99, 0)).toBe(true); //  ...one pixel of it inside
  expect(crateDrawn(0, -10)).toBe(false); // bottom edge exactly on y = 0
  expect(crateDrawn(0, -9)).toBe(true); //  ...one pixel of it inside
  expect(crateDrawn(0, 80)).toBe(false); // top edge exactly on y = viewH
  expect(crateDrawn(0, 79)).toBe(true); //  ...one pixel of it inside

  // And well past each edge, which is the case a reader pictures when they
  // hear "culled" and the case every clause has to survive independently:
  // one clause pinned open leaves three still filtering, and only the
  // direction that clause guarded goes unculled.
  expect(crateDrawn(-500, 0)).toBe(false);
  expect(crateDrawn(500, 0)).toBe(false);
  expect(crateDrawn(0, -500)).toBe(false);
  expect(crateDrawn(0, 500)).toBe(false);
});

test("only a DOOR's picture varies with openness -- a floor listed as open is still a floor", () => {
  // OpenDoors is folded from doorOpened/doorClosed (fold.ts) and is keyed by
  // square, not by tile: nothing in the fold checks that the square it marks
  // holds a door, so a stale key -- a door square rebuilt as floor by a later
  // sceneCreated, an event pair applied to the wrong scene -- reaches this
  // function as "this floor is open". The kind is what decides whether
  // openness means anything at all, and a floor has no open picture to draw:
  // "std:floor/wood/open" is a tile no pack ships, so it would resolve to
  // canvas.ts's missing-tile marker and put a visible hole in the ground.
  const st = newState();
  st.Scenes["vault"] = {
    ID: "vault",
    Name: "Vault",
    GridWidth: 1,
    GridHeight: 1,
    Tiles: { "0,0": { Kind: "floor", Material: "wood", Art: "" } },
    Objects: [],
    OpenDoors: { "0,0": true },
  };
  const cam = fitCamera(1, 1, 44, 44, 44);
  expect(planScene(st, "vault", cam, 44, 44, 44)[0]!.image).toBe("std:floor/wood");
});

// --- planFog rides the same camera, and the same grid bounds -----------------
//
// client/test/visibility.test.ts pins WHICH squares fog -- Explored minus
// Visible, and nothing else -- on a camera fitted to its pane, which is the
// right fixture for that question and the wrong one for this: at scale 1 and
// offset 0 a fog rect computed WITHOUT the camera transform lands in exactly
// the same place as one computed with it. The two cases here pin the geometry
// instead, on the skewed camera above, because fog that does not track terrain
// exactly is fog sitting half a square off the ground it is dimming -- and
// planFog carries its own copy of the transform, so agreeing with planTiles is
// something a test has to hold it to rather than something the code shares.

/**
 * A 3x2 room with no terrain, an explicit explored set, and a Visible that is
 * DEFINED. Defined-and-empty is a seat that has received a projection and can
 * currently see nothing here; an ABSENT Visible is a DM, for whom planFog
 * returns early and fogs nothing (state.ts). A geometry case needs the first.
 */
function fogRoom(explored: Record<string, boolean>): State {
  const st = newState();
  st.Scenes["room"] = {
    ID: "room",
    Name: "Room",
    GridWidth: 3,
    GridHeight: 2,
    Tiles: {},
    Objects: [],
    OpenDoors: {},
    Explored: explored,
    Visible: {},
  };
  return st;
}

test("a fog rect covers the same screen square its terrain would, through the camera", () => {
  // Same square, same camera, same numbers as the terrain case above: (2,1)
  // at cell 40 through scale 0.5 / offset (30,7) is (70, 27, 20, 20).
  expect(planFog(fogRoom({ "2,1": true }), "room", skewCam, 40, 300, 200)).toEqual([
    { x: 70, y: 27, w: 20, h: 20 },
  ]);
});

test("explored ground keyed past the declared grid is not fogged", () => {
  // Explored is unioned from sceneSeen's tile keys, so it holds whatever the
  // server sent; the grid bounds are what stop a stray key greying out ground
  // beside the map, where there is no map to remember.
  const explored = { "2,1": true, "3,0": true, "0,2": true };
  expect(planFog(fogRoom(explored), "room", skewCam, 40, 300, 200)).toEqual([
    { x: 70, y: 27, w: 20, h: 20 },
  ]);
});
