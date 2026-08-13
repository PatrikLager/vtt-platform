import { test, expect } from "bun:test";
import { fitCamera, worldFromScreen } from "../src/view/camera";
import { cellFromPoint } from "../src/view/grid";

test("a scene bigger than the viewport is scaled to fit, not cropped", () => {
  const cam = fitCamera(32, 32, 44, 900, 600);
  // 32*44 = 1408 world px into 600 of height -- the binding dimension.
  expect(cam.scale).toBeCloseTo(600 / 1408, 4);
  expect(cam.scale).toBeLessThan(1);
});

test("a viewport wider than the fitted scene centres it, rather than pinning to a corner", () => {
  // Same camera as above: height binds at scale 600/1408, so the fitted
  // scene is 600x600 inside a 900x600 viewport -- 300px of horizontal slack.
  // Centring it (not leaving it flush left) is what makes "always start
  // seeing the whole map" (spec §7) look intentional rather than accidental.
  const cam = fitCamera(32, 32, 44, 900, 600);
  expect(cam.offsetX).toBeCloseTo(150, 4);
  expect(cam.offsetY).toBeCloseTo(0, 4);
});

test("clicking maps back to the square under the cursor at any zoom", () => {
  const cam = fitCamera(10, 10, 44, 440, 440); // 1:1
  const geom = { cell: 44, width: 10, height: 10 };
  // worldFromScreen undoes scale+offset to WORLD PIXELS; cellFromPoint is
  // the piece that turns those into a cell index (grid.ts:52 already does
  // this division -- composing with it, not duplicating its /cell here, is
  // the whole reason worldFromScreen does not take a cell argument).
  const hit = worldFromScreen(45, 45, cam);
  expect(cellFromPoint(hit.x, hit.y, geom)).toEqual({ x: 1, y: 1 });

  const zoomed = { ...cam, scale: 0.5 };
  const zoomedHit = worldFromScreen(45, 45, zoomed);
  expect(cellFromPoint(zoomedHit.x, zoomedHit.y, geom)).toEqual({ x: 2, y: 2 });
});

test("panning shifts what a click resolves to, in the direction of the pan", () => {
  // A pan is just a change in offset; worldFromScreen must fold it in or a
  // dragged board would mis-click everywhere except its starting position.
  const cam = fitCamera(10, 10, 44, 440, 440);
  const panned = { ...cam, offsetX: cam.offsetX - 44, offsetY: cam.offsetY - 44 };
  const geom = { cell: 44, width: 10, height: 10 };
  const hit = worldFromScreen(45, 45, panned);
  expect(cellFromPoint(hit.x, hit.y, geom)).toEqual({ x: 2, y: 2 });
});
