// The canvas layer: executes what Task 8's pure functions already decided.
//
// Deliberately thin: every DECISION lives in planScene, which is pure and fully
// tested. Keep it that way.
//
// CORRECTED 2026-08-25. This said "happy-dom has NO canvas implementation, so
// nothing this function does can be asserted by the suite", and concluded that
// the loop was "small enough to verify by reading". The first half is true and
// the inference from it is false, which made this the most expensive comment in
// the client: it licensed thirty-two surviving mutants — arithmetic, colours,
// the rotation pivot, and guards that could be deleted whole — in a file that
// simultaneously reported 100.00% line coverage. Thirty-one were killed on
// 2026-08-25 without a canvas existing anywhere.
//
// Nothing here ever CREATES a canvas; every export TAKES a
// CanvasRenderingContext2D and touches exactly thirteen of its members. So a
// test passes a recording double and asserts the ordered call log with exact
// arguments — see client/test/canvas.test.ts. "No canvas" bounds how you
// observe this code; it never made it unobservable.
//
// The cautionary tale in the original still stands and is why this matters: the
// participant list once shipped rendering as "ArmakAsmeDM" behind a passing
// test (backlog #13). "Verify by reading" is what was tried here, and thirty-one
// defects-in-waiting got past the reading.

import type { DrawOp, FogRect, GridLine } from "./scene-plan";

/**
 * ImageMap resolves a DrawOp's `image` to something drawImage can consume.
 *
 * The key is used exactly as scene-plan.ts's tileImage/objectImage emit it —
 * "tile:<name>", "std:<kind>/<material>", or either with an "/open" suffix
 * for an open door — and is never split, prefixed, or reinterpreted here.
 * Parsing it would be exactly the kind of decision this file exists not to
 * make, and would silently break the open-door suffix convention: a door
 * whose art the map lookup can't find would draw as shut while it is open,
 * with no test able to catch it (spec §3.3, §7).
 */
export type ImageMap = Record<string, CanvasImageSource>;

/**
 * missingTileColors is the magenta-checker convention (spec §7: "An art
 * name that resolves nowhere draws a visible missing-tile marker... it must
 * be obvious rather than silently absent") — two colours, high-contrast
 * against each other AND against everything genmappack's own textures ever
 * draw (every pack texture in this repo is a muted earth/stone/wood/metal
 * tone; nothing is this magenta), so a missing tile reads as WRONG on sight,
 * never as a slightly odd floor. Exported so a test can assert on the exact
 * marker colour rather than just "something non-empty was drawn".
 */
export const missingTileColors: readonly [string, string] = ["#ff00ff", "#1a1a1a"];

/**
 * drawMissingTile fills op's rect with a 2x2 checkerboard in
 * missingTileColors, called from inside paint()'s own save/translate/rotate
 * block — (x, y) is ALREADY relative to the rect's centre (the same frame
 * drawImage's -sw/2, -sh/2 call uses below), so this function only ever
 * needs sw/sh, never sx/sy or rot.
 */
function drawMissingTile(ctx: CanvasRenderingContext2D, sw: number, sh: number): void {
  const cw = sw / 2;
  const chh = sh / 2;
  for (let gy = 0; gy < 2; gy++) {
    for (let gx = 0; gx < 2; gx++) {
      ctx.fillStyle = (gx + gy) % 2 === 0 ? missingTileColors[0] : missingTileColors[1];
      ctx.fillRect(-sw / 2 + gx * cw, -sh / 2 + gy * chh, cw, chh);
    }
  }
}

/**
 * paint walks planScene's draw ops in order and issues one drawImage per op
 * whose image resolves, or draws the missing-tile marker (spec §7) for one
 * that does not. Nothing here decides position, image choice, visibility or
 * rotation units — every one of those is planScene's job (scene-plan.ts),
 * already pure and tested. This loop only executes the result.
 */
export function paint(ctx: CanvasRenderingContext2D, ops: DrawOp[], images: ImageMap): void {
  for (const op of ops) {
    const image = images[op.image];

    ctx.save();
    // DrawOp.rot rotates about the rect's CENTRE (scene-plan.ts's doc comment
    // on DrawOp), so translate there before rotating — rotating about
    // (sx, sy) would swing a footprint out of its own square. Applies
    // equally to the missing-tile marker: a rotated object with unresolved
    // art still marks its OWN rotated footprint, not an axis-aligned box
    // that disagrees with where planScene actually placed it.
    ctx.translate(op.sx + op.sw / 2, op.sy + op.sh / 2);
    ctx.rotate(op.rot);
    if (image) {
      ctx.drawImage(image, -op.sw / 2, -op.sh / 2, op.sw, op.sh);
    } else {
      // Not a silent visibility decision (spec §7's whole point) — planScene
      // already decided this op belongs on screen; an ImageMap that has not
      // (yet, or ever) resolved this key must still draw SOMETHING obvious,
      // never nothing. Review finding C2/C3 (2026-08-16): every square of
      // both shipped adventures hit this exact branch and, before this
      // marker existed, drew nothing at all — with a test
      // (spectator-view.test.ts) actively pinning that silence as correct.
      drawMissingTile(ctx, op.sw, op.sh);
    }
    ctx.restore();
  }
}

/**
 * gridInk is the lattice's colour, and the one judgement this file makes.
 *
 * It has to stay legible over BOTH a dark earth floor and pale flagstone
 * without competing with either, so it is a low-opacity near-black: dark
 * enough to read on the pale side, faint enough not to cage the dark side.
 * A tactical map has to be countable AND look like a place; a heavy lattice
 * wins the first and loses the second.
 *
 * Colour rather than position, which is why it is allowed to live here: WHERE
 * the lines go is planGrid's decision and is asserted; how they look is
 * presentation, and presentation is the only thing this untestable layer may
 * own.
 */
const gridInk = "rgba(0, 0, 0, 0.22)";

/**
 * strokeGrid draws planGrid's square boundaries in one path.
 *
 * One path, not one per line: a stroke per line would be hundreds of context
 * calls on a large scene for an identical result.
 *
 * Called AFTER paint, deliberately — the lattice belongs on top of the terrain
 * it divides. Drawn first, every tile would cover it and the board would be
 * uncountable again, which is the defect this exists to fix.
 */
export function strokeGrid(ctx: CanvasRenderingContext2D, lines: GridLine[]): void {
  if (lines.length === 0) return;
  ctx.save();
  ctx.strokeStyle = gridInk;
  ctx.lineWidth = 1;
  ctx.beginPath();
  for (const l of lines) {
    ctx.moveTo(l.x1, l.y1);
    ctx.lineTo(l.x2, l.y2);
  }
  ctx.stroke();
  ctx.restore();
}

/**
 * fogInk is what remembered-but-unseen ground is shaded with, and the second
 * judgement this file makes, allowed here for the same reason gridInk is:
 * WHERE the fog goes is planFog's decision and is asserted; how it looks is
 * presentation, and presentation is the only thing this untestable layer may
 * own (spec §6.1 makes that division the whole reason fog is a pass rather
 * than a per-op `dim` flag).
 *
 * Darker than gridInk's 0.22 by enough that "I remember this" is never
 * mistaken for "I can see this", and light enough that the terrain underneath
 * stays legible — you are meant to read your own map, not be denied it. The
 * figure follows the precedent spec §6.1 records for the same purpose in
 * RPTool's FogRenderer, 100/255 ≈ 0.39, rounded.
 *
 * Exported so a test can assert the fog was filled with THIS, rather than
 * merely that some fillRect happened — the same reason missingTileColors is
 * exported above.
 */
export const fogInk = "rgba(0, 0, 0, 0.4)";

/**
 * shadeFog fills planFog's regions, and is strokeGrid's twin in every respect:
 * it decides nothing, it is called between the two draws whose order matters,
 * and it returns early on an empty list rather than opening a pointless
 * save/restore.
 *
 * BETWEEN paint AND strokeGrid, which spectator.ts's renderGrid is what
 * actually establishes. Terrain, then fog, then grid: the lattice stays crisp
 * over remembered ground, because you remember a room's SHAPE and dimming its
 * squares would make remembered floor harder to count for no gain (spec §6.1).
 */
export function shadeFog(ctx: CanvasRenderingContext2D, rects: FogRect[]): void {
  if (rects.length === 0) return;
  ctx.save();
  ctx.fillStyle = fogInk;
  for (const r of rects) {
    ctx.fillRect(r.x, r.y, r.w, r.h);
  }
  ctx.restore();
}
