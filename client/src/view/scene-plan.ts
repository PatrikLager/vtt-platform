// planScene: given state, a camera and a pack-naming convention (never a
// pack file -- see tileImage below), decide exactly what to draw and where.
//
// Every decision needed to paint a frame lives HERE and is a pure function of
// its arguments, because the client suite runs on happy-dom, which has NO
// canvas implementation whatsoever. Nothing that touches a canvas context can
// be asserted by any test in this repo -- that is how the participant list
// once shipped rendering as "ArmakAsmeDM" behind a passing test: the
// assertion checked the DOM tree, which was correct, while the rendering was
// wrong and nobody looked (design spec §8). Task 9's canvas layer consumes
// the DrawOp list this file produces and does nothing but call drawImage
// with it, small enough to verify by reading.

import type { Scene, SceneObject, State, Tile } from "../state";
import type { Camera } from "./camera";

export interface DrawOp {
  image: string;
  sx: number;
  sy: number;
  sw: number;
  sh: number;
  /**
   * Radians. Rotates about the rect's CENTRE -- (sx + sw/2, sy + sh/2) --
   * never about (sx, sy). Rotating about the corner would swing a footprint
   * out of its own square, which is wrong for the one thing this field is
   * for (a rotated object, §3.4). Task 9's canvas.ts is the consumer that
   * establishes this; recorded here because DrawOp is Task 9's contract with
   * this file and a pivot left undocumented is exactly the kind of implicit
   * geometric assumption this file's whole premise (planScene decides
   * everything) is meant not to leave lying around.
   */
  rot: number;
}

/**
 * planScene walks every square and object of one scene, in world order, and
 * emits a DrawOp for each that intersects the camera's viewport.
 *
 * Culling is a correctness requirement, not an optimisation: a 200x200 scene
 * is 40,000 squares, and Task 9's canvas layer has no way to know which of
 * them the camera can even see -- if this function does not filter, nothing
 * downstream will.
 */
export function planScene(
  st: State,
  sceneId: string,
  cam: Camera,
  cell: number,
  viewW: number,
  viewH: number,
): DrawOp[] {
  const scene = st.Scenes[sceneId];
  if (!scene) return [];

  const ops: DrawOp[] = [];
  planTiles(scene, cam, cell, viewW, viewH, ops);
  planObjects(scene, cam, cell, viewW, viewH, ops);
  return ops;
}

function planTiles(
  scene: Scene,
  cam: Camera,
  cell: number,
  viewW: number,
  viewH: number,
  ops: DrawOp[],
): void {
  // Tiles is OPTIONAL (state.ts: a terrain-free scene is legal, Patrik's
  // ruling 2026-08-13) -- `?? {}` is the one place this file has to know
  // that, rather than every square lookup below needing its own guard.
  const tiles = scene.Tiles ?? {};
  const openDoors = scene.OpenDoors ?? {};

  for (let gy = 0; gy < scene.GridHeight; gy++) {
    for (let gx = 0; gx < scene.GridWidth; gx++) {
      const tile = tiles[`${gx},${gy}`];
      // A terrain-free scene has GridWidth/GridHeight but no per-square
      // entries at all: skip rather than draw something for a square with
      // no declared nature.
      if (!tile) continue;

      const sx = gx * cell * cam.scale + cam.offsetX;
      const sy = gy * cell * cam.scale + cam.offsetY;
      const size = cell * cam.scale;
      if (!intersectsViewport(sx, sy, size, size, viewW, viewH)) continue;

      const open = openDoors[`${gx},${gy}`] === true;
      ops.push({ image: tileImage(tile, open), sx, sy, sw: size, sh: size, rot: 0 });
    }
  }
}

function planObjects(
  scene: Scene,
  cam: Camera,
  cell: number,
  viewW: number,
  viewH: number,
  ops: DrawOp[],
): void {
  for (const obj of scene.Objects ?? []) {
    const sx = obj.X * cell * cam.scale + cam.offsetX;
    const sy = obj.Y * cell * cam.scale + cam.offsetY;
    const sw = obj.Width * cell * cam.scale;
    const sh = obj.Height * cell * cam.scale;
    if (!intersectsViewport(sx, sy, sw, sh, viewW, viewH)) continue;

    ops.push({
      image: objectImage(obj),
      sx,
      sy,
      sw,
      sh,
      // Canvas's rotate() (Task 9's only use of this value) takes radians.
      // Converting here, rather than leaving RotationDegrees for the canvas
      // loop to convert, keeps that loop a pure "call drawImage" walk with
      // no arithmetic of its own to get wrong unnoticed.
      rot: (obj.RotationDegrees * Math.PI) / 180,
    });
  }
}

/** intersectsViewport: true when a screen rect overlaps [0,viewW)x[0,viewH). */
function intersectsViewport(
  sx: number,
  sy: number,
  sw: number,
  sh: number,
  viewW: number,
  viewH: number,
): boolean {
  return sx + sw > 0 && sx < viewW && sy + sh > 0 && sy < viewH;
}

/**
 * tileImage resolves a square's Tile to the image Task 9 must draw, so the
 * drawing layer needs no knowledge of resolution rules (design spec §4.2):
 * two levels only, the map's own pack or the standard vocabulary, never
 * anything in between.
 *
 * Art non-empty means the square was overridden against its own pack at
 * compile time (mapdef.Resolve) -- draw that pack tile. Art empty means
 * standard: draw the picture for its Kind/Material pair, which the platform
 * treats as opaque strings (CLAUDE.md rule 5) and never interprets further.
 *
 * A door is one nature whose picture varies with folded state (§3.3): a
 * pack door tile supplies file_open/file_closed, and the standard door has
 * an open and a closed picture too. Closed is the unmarked default -- it
 * matches fold.ts's own treatment of a door "never toggled", so only the
 * open case needs a marker at all.
 */
function tileImage(tile: Tile, open: boolean): string {
  const base = tile.Art !== "" ? `tile:${tile.Art}` : `std:${tile.Kind}/${tile.Material}`;
  return tile.Kind === "door" && open ? `${base}/open` : base;
}

/**
 * objectImage resolves a SceneObject's Art. Unlike a tile, an object has no
 * standard vocabulary to fall back to (spec §4.2's pack manifest declares
 * objects separately, with no platform-known kind/material pair for them) --
 * every object's art must resolve in its scene's pack, so it is always
 * "tile:<name>". Validation (mapdef, §4.4) refuses an object whose art does
 * not resolve before this code ever runs; an empty Art reaching here would
 * be a bug upstream, not something this function can repair, and letting a
 * plain "tile:" key through to Task 9's missing-tile marker (spec §7) is the
 * honest, fail-loud answer.
 */
function objectImage(obj: SceneObject): string {
  return `tile:${obj.Art}`;
}
