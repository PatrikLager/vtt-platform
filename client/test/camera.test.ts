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

test("a viewport TALLER than the fitted scene centres it vertically, by exactly half the slack", () => {
  // The vertical mirror of the test above, and it has to be its own case:
  // every other fixture in this file either binds on height or fits 1:1, so
  // offsetY comes out 0 in all of them — and 0 is precisely the value that
  // cannot tell "half the slack" apart from any other arithmetic on that
  // slack, because every operator agrees on zero. A portrait viewport (a
  // phone, a narrow docked panel) is the ordinary way a real table hits
  // this axis, and getting it wrong pushes the map off the bottom of the
  // screen rather than merely off-centre.
  //
  // 32*44 = 1408 world px into 600 of WIDTH now binds, at scale 600/1408,
  // so the fitted scene is 600x600 inside a 600x900 viewport: no horizontal
  // slack at all, and 300px of vertical slack with 150 of it above the map.
  const cam = fitCamera(32, 32, 44, 600, 900);
  expect(cam.scale).toBeCloseTo(600 / 1408, 4);
  expect(cam.offsetX).toBeCloseTo(0, 4);
  expect(cam.offsetY).toBeCloseTo(150, 4);

  // And the centring has to be the one worldFromScreen undoes, not just a
  // number that looks right: a click on the fitted map's own top-left
  // corner, 150px down the screen, is world origin.
  const corner = worldFromScreen(0, 150, cam);
  expect(corner.x).toBeCloseTo(0, 4);
  expect(corner.y).toBeCloseTo(0, 4);
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
