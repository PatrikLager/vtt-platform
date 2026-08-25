import { test, expect } from "bun:test";
import { newState, type State, type Tile } from "../src/state";
import { fitCamera } from "../src/view/camera";
import { planGrid } from "../src/view/scene-plan";

// The grid is what a tactical map is FOR. maps/cellar rendered beautifully --
// stone walls, dirt and flagstone, pillars, a plank door -- and shipped with no
// squares on it at all, so nobody could tell whether that pillar was three
// squares away or four. The old CSS lattice (.grid's background-size) was
// painted over the moment terrain drew on the canvas, leaving NO grid, which is
// worse than a misaligned one.
//
// No test in this repo could catch that: happy-dom has no canvas, and an
// ABSENCE on a canvas fails no assertion. It was found by looking at a
// screenshot. These tests exist so the line POSITIONS are pinned even though
// the drawing itself never can be.

function scene(w: number, h: number): State {
  const st = newState();
  const tiles: Record<string, Tile> = {};
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      tiles[`${x},${y}`] = { Kind: "floor", Material: "earth", Art: "" };
    }
  }
  st.Scenes["s"] = {
    ID: "s", Name: "S", GridWidth: w, GridHeight: h,
    Tiles: tiles, Objects: [], OpenDoors: {},
  };
  return st;
}

test("a grid line sits on every square boundary, in world order", () => {
  // 3x3 at cell 44 into a 132x132 pane: scale 1, offset 0. Four verticals and
  // four horizontals -- both edges plus the two interior boundaries.
  const st = scene(3, 3);
  const cam = fitCamera(3, 3, 44, 132, 132);
  const lines = planGrid(st, "s", cam, 44, 132, 132);

  const verticals = lines.filter((l) => l.x1 === l.x2).map((l) => l.x1).sort((a, b) => a - b);
  const horizontals = lines.filter((l) => l.y1 === l.y2).map((l) => l.y1).sort((a, b) => a - b);
  expect(verticals).toEqual([0, 44, 88, 132]);
  expect(horizontals).toEqual([0, 44, 88, 132]);
});

test("grid lines follow the camera at a non-unit scale and a non-zero offset", () => {
  // The load-bearing case. At scale 1 / offset 0 a line drawn WITHOUT the
  // camera transform lands in exactly the same place as one drawn with it, so
  // that test passes whether or not the transform is applied -- which is how a
  // real coordinate-space bug already slipped through on this branch (tokens
  // drew unscaled over scaled terrain).
  const st = scene(3, 3);
  const cam = { scale: 0.5, offsetX: 20, offsetY: 7 };
  const lines = planGrid(st, "s", cam, 44, 300, 300);

  const verticals = lines.filter((l) => l.x1 === l.x2).map((l) => l.x1).sort((a, b) => a - b);
  // gx * cell * scale + offsetX  ->  20, 42, 64, 86
  expect(verticals).toEqual([20, 42, 64, 86]);

  // A vertical spans the scene's drawn HEIGHT, not the pane's: it must start
  // and end where the map does, or the lines overhang into empty pane.
  const v = lines.find((l) => l.x1 === l.x2)!;
  expect(v.y1).toBe(7);
  expect(v.y2).toBe(7 + 3 * 44 * 0.5);
});

test("grid lines are culled to the viewport", () => {
  // Culling is correctness, not optimisation: a 200x200 scene is 402 lines
  // uncalled, and planScene's own tile ops are already culled the same way.
  const st = scene(200, 200);
  const cam = { scale: 1, offsetX: 0, offsetY: 0 };
  const all = planGrid(st, "s", cam, 44, 100_000, 100_000);
  const windowed = planGrid(st, "s", cam, 44, 200, 200);

  expect(all.length).toBe(402); // 201 verticals + 201 horizontals
  expect(windowed.length).toBeLessThan(all.length);
  // A 200px pane at cell 44 shows ~5 boundaries per axis.
  expect(windowed.length).toBeLessThan(20);
});

test("a scene with no terrain still gets its grid", () => {
  // A terrain-free scene is legal (Patrik's ruling 2026-08-13) and is exactly
  // the case that most needs a lattice: with no tiles drawn, the grid is the
  // ONLY thing showing where the squares are.
  const st = newState();
  st.Scenes["bare"] = { ID: "bare", Name: "Bare", GridWidth: 2, GridHeight: 2 };
  const cam = fitCamera(2, 2, 44, 88, 88);
  expect(planGrid(st, "bare", cam, 44, 88, 88).length).toBe(6);
});

test("a horizontal spans the scene's drawn WIDTH, as a vertical spans its height", () => {
  // `right` is computed once and read ONLY by the horizontals' x2, so the test
  // above -- which reads a vertical's span -- cannot see it at all, and the
  // three tests before it read only x1/y1. A lattice whose horizontals stop
  // short leaves the right-hand columns unruled; one whose horizontals run
  // backwards draws them off the left of the pane, overhanging exactly the
  // empty pane beside a letterboxed map that this function's doc comment says
  // scene-spanning lines exist to avoid.
  //
  // 3 wide by 2 tall, so a width read as a height reads back wrong rather than
  // right, and on a scaled, offset camera, so `GridWidth * step` cannot be
  // confused with `GridWidth / step` (step is 1 only at scale 1, cell 1).
  const st = scene(3, 2);
  const cam = { scale: 0.5, offsetX: 30, offsetY: 7 };
  const lines = planGrid(st, "s", cam, 40, 300, 200);

  const h = lines.find((l) => l.y1 === l.y2);
  expect(h).toBeDefined();
  expect(h!.x1).toBe(30); // the scene's left edge: cam.offsetX
  expect(h!.x2).toBe(90); // left + GridWidth * step = 30 + 3 * (40 * 0.5)
});

test("a boundary off the top or the left is culled, not drawn at a negative coordinate", () => {
  // The far edges are the easy half of a two-sided cull to get right, and the
  // near ones are the half no on-screen fixture reaches: a viewport showing
  // the middle of a panned map has lines above it and to the LEFT of it, and
  // those are filtered by the `>= 0` half alone. A line at x = -50 is not
  // visible, but it is not free either -- it is a stroke canvas.ts is told to
  // make, and this is the only code that can decline to ask for it.
  const st = scene(3, 2);
  const cam = { scale: 1, offsetX: -50, offsetY: -30 };
  const lines = planGrid(st, "s", cam, 40, 200, 200);

  const verticals = lines.filter((l) => l.x1 === l.x2).map((l) => l.x1).sort((a, b) => a - b);
  const horizontals = lines.filter((l) => l.y1 === l.y2).map((l) => l.y1).sort((a, b) => a - b);
  // Four verticals exist at -50, -10, 30, 70 and three horizontals at
  // -30, 10, 50; the ones off the near edges never reach the list.
  expect(verticals).toEqual([30, 70]);
  expect(horizontals).toEqual([10, 50]);
});

test("an unknown scene plans no grid rather than throwing", () => {
  const cam = fitCamera(3, 3, 44, 132, 132);
  expect(planGrid(newState(), "nope", cam, 44, 132, 132)).toEqual([]);
});
