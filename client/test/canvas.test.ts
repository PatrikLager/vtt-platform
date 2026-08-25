import { test, expect } from "bun:test";
import {
  fogInk,
  missingTileColors,
  paint,
  shadeFog,
  strokeGrid,
  type ImageMap,
} from "../src/view/canvas";

// canvas.ts calls itself untestable, and its header comment says so in as many
// words: "happy-dom has NO canvas implementation, so nothing this function does
// can be asserted by the suite". The first half of that is true and the second
// half does not follow. NOTHING IN canvas.ts EVER CREATES A CANVAS — paint,
// strokeGrid and shadeFog each TAKE a CanvasRenderingContext2D and call
// thirteen members of it. happy-dom's inability to produce one is therefore
// irrelevant: the tests below hand them a recording double instead, and assert
// the exact ordered call log, arguments included.
//
// That distinction matters because the layer is not as thin as the comment
// claims. drawMissingTile computes a four-cell checkerboard — two divisions, a
// parity test, two multiplications and two negations per cell — and paint
// computes a rotation pivot. Every one of those is real arithmetic, and every
// one of them was invisible: canvas.ts sat at a 100% LINE-coverage floor while
// thirty-two distinct mutations of it went undetected by the whole suite,
// including inverting the pivot, doubling every dimension the checkerboard
// uses, and emptying strokeGrid's loop body outright.
//
// The three call sites that existed before this file (spectator-view.test.ts's
// fakeCtx, visibility.test.ts's fogCtx, app.test.ts's recordingCtx) each record
// a NARROW slice on purpose — "only what was actually drawn" — which is right
// for what those files are testing and is exactly why the geometry escaped. A
// double that swallows save/translate/rotate cannot see a pivot move, and a
// double that reports `fillRect:${ctx.fillStyle}` compared against an
// INTERPOLATED `${fogInk}` cannot see the ink change, because both sides of the
// assertion move together. This file records everything and compares against
// literals, which is the whole difference.
//
// This is the discipline scene-plan.test.ts already applies one layer up
// (a pure planner asserted as exact DrawOp arrays), applied one layer down to
// the code that turns those ops into context calls.

/**
 * recorder builds a CanvasRenderingContext2D stand-in that appends a string for
 * every interaction canvas.ts has with it, in order.
 *
 * THIRTEEN MEMBERS, which is the entire surface canvas.ts touches: save,
 * restore, fillStyle, fillRect, translate, strokeStyle, stroke, rotate, moveTo,
 * lineWidth, lineTo, drawImage, beginPath. Anything canvas.ts starts calling
 * that is not here throws rather than no-ops, which is the desired direction:
 * a new context call is a new decision made in the one layer that is meant to
 * make none.
 *
 * PROPERTY ASSIGNMENTS ARE CALLS. fillStyle, strokeStyle and lineWidth are
 * settable properties rather than methods, and three of the surviving mutants
 * lived precisely there — the checkerboard's two colours, the grid's ink and
 * the fog's ink are all only ever observable as an assignment that happens
 * BETWEEN two other calls. Recording them in the same log as the methods is
 * what lets a test say "this colour, then that rect, then the other colour",
 * which is the actual visible behaviour and not an internal.
 */
function recorder(): { calls: string[]; ctx: CanvasRenderingContext2D } {
  const calls: string[] = [];
  const pen = { fillStyle: "", strokeStyle: "", lineWidth: 0 };
  const ctx = {
    get fillStyle(): string {
      return pen.fillStyle;
    },
    set fillStyle(v: string) {
      pen.fillStyle = v;
      calls.push(`fillStyle=${v}`);
    },
    get strokeStyle(): string {
      return pen.strokeStyle;
    },
    set strokeStyle(v: string) {
      pen.strokeStyle = v;
      calls.push(`strokeStyle=${v}`);
    },
    get lineWidth(): number {
      return pen.lineWidth;
    },
    set lineWidth(v: number) {
      pen.lineWidth = v;
      calls.push(`lineWidth=${v}`);
    },
    save(): void {
      calls.push("save");
    },
    restore(): void {
      calls.push("restore");
    },
    translate(x: number, y: number): void {
      calls.push(`translate(${x},${y})`);
    },
    rotate(a: number): void {
      calls.push(`rotate(${a})`);
    },
    beginPath(): void {
      calls.push("beginPath");
    },
    moveTo(x: number, y: number): void {
      calls.push(`moveTo(${x},${y})`);
    },
    lineTo(x: number, y: number): void {
      calls.push(`lineTo(${x},${y})`);
    },
    stroke(): void {
      calls.push("stroke");
    },
    fillRect(x: number, y: number, w: number, h: number): void {
      calls.push(`fillRect(${x},${y},${w},${h})`);
    },
    drawImage(img: unknown, dx: number, dy: number, dw: number, dh: number): void {
      calls.push(`drawImage(${(img as { tag: string }).tag},${dx},${dy},${dw},${dh})`);
    },
  } as unknown as CanvasRenderingContext2D;
  return { calls, ctx };
}

/**
 * ART is a stand-in CanvasImageSource carrying a tag, so the log can report
 * WHICH image reached drawImage without a real image ever existing. A bare
 * `{}` would prove drawImage was called and nothing about what with.
 */
const ART = { tag: "art" } as unknown as CanvasImageSource;

/**
 * OP is one draw op chosen so that no two mutations of canvas.ts's arithmetic
 * can collide into the same answer.
 *
 * Every number is non-zero, and no two of them are equal or in a 2:1 ratio,
 * because that is what it takes for `+` and `-`, `*` and `/`, and unary `-`
 * and unary `+` to give three different answers on the SAME expression:
 *
 *   sx + sw/2 = 54,  but sx - sw/2 = 6   and sx + sw*2 = 126
 *   sy + sh/2 = 78,  but sy - sh/2 = 62  and sy + sh*2 = 102
 *   -sw/2 = -24,     but +sw = 48        and -sw*2 = -96
 *   -sh/2 = -8,      but +sh = 16        and -sh*2 = -32
 *
 * With the conventional 44x44 square of the other tests, sw and sh are equal
 * and half the geometry above is indistinguishable from a transposed version
 * of itself. rot is deliberately not a multiple of a right angle for the same
 * family of reasons.
 */
const OP = { image: "tile:a", sx: 30, sy: 70, sw: 48, sh: 16, rot: 0.7 };

test("paint pivots on the rect's centre and draws the image around that centre", () => {
  // THE PIVOT IS THE ASSERTION. scene-plan.ts's DrawOp doc says rot turns about
  // the rect's CENTRE, "never about (sx, sy)", and canvas.ts is the only code
  // that establishes it — planScene emits a corner and an angle and trusts this
  // translate to convert. A translate to the corner, or to a centre computed
  // with sw*2 instead of sw/2, swings a rotated footprint out of its own square
  // and no DrawOp assertion anywhere can see it, because the ops are identical
  // either way.
  //
  // The counterpart is drawImage's own offsets: having moved the origin to the
  // centre, the image must be drawn from HALF ITS SIZE BACK, so its middle
  // lands on the pivot. -sw/2 and +sw are both "an offset derived from sw" and
  // only the exact number distinguishes them.
  const { calls, ctx } = recorder();
  paint(ctx, [OP], { "tile:a": ART });
  expect(calls).toEqual([
    "save",
    "translate(54,78)",
    "rotate(0.7)",
    "drawImage(art,-24,-8,48,16)",
    "restore",
  ]);
});

test("paint restores the context once per op, so one op's rotation cannot leak into the next", () => {
  // save/restore is not decoration: the transform is CUMULATIVE state on the
  // context, so an op that rotates and never restores rotates every op after
  // it too. On a scene with one rotated object that is the whole board sliding
  // askew behind it. Asserted as a balanced, per-op pairing with the transform
  // strictly inside, rather than as a count, because two saves followed by two
  // restores would satisfy a count and still be wrong.
  const { calls, ctx } = recorder();
  paint(
    ctx,
    [OP, { image: "tile:b", sx: 90, sy: 12, sw: 20, sh: 36, rot: 0 }],
    { "tile:a": ART, "tile:b": { tag: "second" } as unknown as CanvasImageSource },
  );
  expect(calls).toEqual([
    "save",
    "translate(54,78)",
    "rotate(0.7)",
    "drawImage(art,-24,-8,48,16)",
    "restore",
    "save",
    "translate(100,30)",
    "rotate(0)",
    "drawImage(second,-10,-18,20,36)",
    "restore",
  ]);
});

test("paint draws the missing-tile checkerboard in magenta and near-black, alternating, within the op's own rect", () => {
  // Spec §7: an art name that resolves nowhere "must be obvious rather than
  // silently absent". This asserts the two things that make it obvious and
  // that nothing else in the suite can see.
  //
  // FIRST, THE COLOURS AS LITERALS. spectator-view.test.ts and
  // visibility.test.ts both assert their colours as `${missingTileColors[0]}`
  // and `${fogInk}` — interpolating the very constant under test, so blanking
  // the constant to "" blanks both sides of the comparison and the test stays
  // green while the marker turns invisible. The magenta is a user-visible
  // decision (canvas.ts's own comment: high-contrast against every muted
  // earth/stone/wood/metal tone genmappack ships, so a hole reads as WRONG
  // rather than as a slightly odd floor), so it is written out here in full.
  //
  // SECOND, THE GEOMETRY. Four cells, each a quarter of the rect, laid out
  // from the CENTRE because drawMissingTile runs inside paint's own translate —
  // hence the negative starting corner. Two rows, two columns, and a parity
  // that alternates: get the loop bound, the cell size, the step or the parity
  // wrong and you get a marker that is still magenta, still drawn, and no
  // longer the size or shape of the square that is missing.
  const { calls, ctx } = recorder();
  paint(ctx, [OP], {});
  expect(calls).toEqual([
    "save",
    "translate(54,78)",
    "rotate(0.7)",
    "fillStyle=#ff00ff",
    "fillRect(-24,-8,24,8)",
    "fillStyle=#1a1a1a",
    "fillRect(0,-8,24,8)",
    "fillStyle=#1a1a1a",
    "fillRect(-24,0,24,8)",
    "fillStyle=#ff00ff",
    "fillRect(0,0,24,8)",
    "restore",
  ]);
});

test("the exported missing-tile colours are the two the marker actually draws with", () => {
  // The export exists so other tests can name the marker without hard-coding
  // it (spectator-view.test.ts does exactly that). That is only safe while the
  // exported pair and the drawn pair are the same thing, which is a claim about
  // two separate lines of canvas.ts and belongs in its own assertion — an
  // emptied array literal, or a swap, leaves every interpolating caller green.
  expect(missingTileColors).toEqual(["#ff00ff", "#1a1a1a"]);
});

test("paint draws nothing at all for an empty op list", () => {
  const { calls, ctx } = recorder();
  paint(ctx, [], {});
  expect(calls).toEqual([]);
});

test("strokeGrid strokes every line as one path, in the grid's own ink", () => {
  // One path for all lines, not one stroke per line, is the documented reason
  // this function exists in this shape — so the assertion is the whole
  // sequence: open the path once, walk every line into it, stroke once. An
  // emptied loop body leaves beginPath and stroke intact and draws nothing, a
  // shape the suite could not previously tell from a full lattice.
  //
  // The ink is written out rather than interpolated from the module, for the
  // reason the missing-tile test gives at length: a constant compared against
  // itself cannot fail. It is also not exported, so a literal is the only way.
  //
  // The two lines are deliberately asymmetric — a vertical and a horizontal,
  // neither starting at an origin — so that moveTo taking (x1,y1) and lineTo
  // taking (x2,y2) is pinned rather than assumed.
  const { calls, ctx } = recorder();
  strokeGrid(ctx, [
    { x1: 12, y1: 3, x2: 12, y2: 97 },
    { x1: 5, y1: 41, x2: 88, y2: 41 },
  ]);
  expect(calls).toEqual([
    "save",
    "strokeStyle=rgba(0, 0, 0, 0.22)",
    "lineWidth=1",
    "beginPath",
    "moveTo(12,3)",
    "lineTo(12,97)",
    "moveTo(5,41)",
    "lineTo(88,41)",
    "stroke",
    "restore",
  ]);
});

test("strokeGrid touches the context zero times when there are no lines", () => {
  // The early return is the point, and "no lines were drawn" is NOT what this
  // asserts — that is true either way, since a path with nothing in it strokes
  // nothing. What the guard buys is not opening a save/restore and not
  // repainting the pen at all, which only a log that records property
  // assignments can see. Every previous double in the suite ignored save,
  // strokeStyle and lineWidth, so deleting this line changed nothing anyone
  // was looking at.
  const { calls, ctx } = recorder();
  strokeGrid(ctx, []);
  expect(calls).toEqual([]);
});

test("shadeFog fills one rect per region in the fog's own ink, under a single save", () => {
  // visibility.test.ts already asserts the rects; what it cannot assert is the
  // ink, because it compares against an interpolated `${fogInk}`. The figure
  // is a deliberate one (canvas.ts: darker than the grid's 0.22 by enough that
  // "I remember this" is never read as "I can see this", light enough that the
  // terrain underneath stays legible, following RPTool's 100/255), so it is
  // pinned as the literal a viewer would actually see.
  //
  // One save for the whole pass, not one per rect: fog is a single flat wash
  // and re-saving per region would be a per-square decision in the layer that
  // makes none.
  const { calls, ctx } = recorder();
  shadeFog(ctx, [
    { x: 13, y: 29, w: 48, h: 16 },
    { x: 61, y: 29, w: 48, h: 16 },
  ]);
  expect(calls).toEqual([
    "save",
    "fillStyle=rgba(0, 0, 0, 0.4)",
    "fillRect(13,29,48,16)",
    "fillRect(61,29,48,16)",
    "restore",
  ]);
});

test("shadeFog touches the context zero times when nothing is fogged", () => {
  // A DM's board fogs nothing at all (planFog returns [] for a stream with no
  // Visible set), so this is the common case, not the edge one — and the
  // counterpart to strokeGrid's guard above, for the same unobservable reason:
  // without the early return the pen is still set to fog ink inside a
  // save/restore nobody was recording.
  const { calls, ctx } = recorder();
  shadeFog(ctx, []);
  expect(calls).toEqual([]);
});

test("the exported fog ink is the one shadeFog actually fills with", () => {
  // Same claim as the missing-tile colours test, for the same reason: the
  // export is there so visibility.test.ts can name the ink, and that is only
  // sound while the exported string and the filled string are one thing.
  expect(fogInk).toBe("rgba(0, 0, 0, 0.4)");
});

test("an ImageMap key is used exactly as scene-plan emitted it, suffix and all", () => {
  // canvas.ts's ImageMap doc is emphatic that the key is "never split,
  // prefixed, or reinterpreted here", and names the failure: a door whose
  // "/open" suffix got parsed away would draw SHUT while it is open. The
  // marker branch is what makes that assertable at all — a key that does not
  // resolve verbatim falls through to the checkerboard, which is loud.
  const { calls, ctx } = recorder();
  const images: ImageMap = {
    "std:door/wood/open": { tag: "open-door" } as unknown as CanvasImageSource,
    "std:door/wood": { tag: "shut-door" } as unknown as CanvasImageSource,
  };
  paint(ctx, [{ image: "std:door/wood/open", sx: 0, sy: 0, sw: 48, sh: 16, rot: 0 }], images);
  expect(calls).toContain("drawImage(open-door,-24,-8,48,16)");
});
