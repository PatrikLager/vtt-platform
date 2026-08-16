// The canvas layer: executes what Task 8's pure functions already decided.
//
// Deliberately thin. happy-dom has NO canvas implementation, so nothing this
// function does can be asserted by the suite — which is exactly how the
// participant list shipped rendering as "ArmakAsmeDM" behind a passing test
// (backlog #13). Every decision therefore lives in planScene, which is pure and
// fully tested; this loop is small enough to verify by reading.

import type { DrawOp, GridLine } from "./scene-plan";

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
 * paint walks planScene's draw ops in order and issues one drawImage per op.
 * Nothing here decides position, image choice, visibility or rotation units —
 * every one of those is planScene's job (scene-plan.ts), already pure and
 * tested. This loop only executes the result.
 */
export function paint(ctx: CanvasRenderingContext2D, ops: DrawOp[], images: ImageMap): void {
  for (const op of ops) {
    const image = images[op.image];
    // Not a visibility decision — planScene already decided this op belongs
    // on screen. This guards against an ImageMap that has not (yet, or ever)
    // resolved this key, which today is EVERY key: no task in this arc loads
    // real pack images into one. Skipping rather than throwing keeps a
    // partially-populated map from taking the whole board down.
    if (!image) continue;

    ctx.save();
    // DrawOp.rot rotates about the rect's CENTRE (scene-plan.ts's doc comment
    // on DrawOp), so translate there before rotating — rotating about
    // (sx, sy) would swing a footprint out of its own square.
    ctx.translate(op.sx + op.sw / 2, op.sy + op.sh / 2);
    ctx.rotate(op.rot);
    ctx.drawImage(image, -op.sw / 2, -op.sh / 2, op.sw, op.sh);
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
