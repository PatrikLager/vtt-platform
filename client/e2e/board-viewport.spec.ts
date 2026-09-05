import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";

// THE BOARD FOLLOWS THE WINDOW, proved in a real browser because nothing else
// can prove it (design spec §6.1, 2026-09-05).
//
// The unit suite runs on happy-dom, which has NO LAYOUT ENGINE: every element
// reports clientWidth/clientHeight as 0, which is exactly what makes
// spectator.ts's paneSize fall back and every geometry assertion in
// spectator-view.test.ts deterministic. That determinism is bought by never
// measuring anything real, so the one claim this whole change is about — "the
// board fills the space it is given, and keeps filling it when that space
// changes" — is invisible to it by construction.
//
// WHAT ONLY A BROWSER SETTLES, and each has its own test below:
//
//   1. The board is not 640 wide. That is the constant it replaced.
//   2. It CHANGES when the viewport changes. One measurement proves a number;
//      two prove the coupling, and the coupling is the point.
//   3. The canvas backing store is the CSS box times devicePixelRatio, so the
//      board is not soft on a high-DPI display. Playwright can read canvas.width
//      and getBoundingClientRect() together; happy-dom has neither.
//   4. The clamp holds at both ends. Those bounds are the whole of what stands
//      between us and backlog T1/#19 coming back — a board sized by its content
//      grew the page until the controls sat below the laptop fold — and nothing
//      anywhere proved they hold in a browser.
//
// Screenshots are the remote demo medium (client spec §6): a before/after at two
// viewport sizes says more from a phone than any paragraph here.

const { base, tokens } = fixture("board-viewport");

const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png`, fullPage: true });

// The clamp in client/src/style.css, written out so a change to one without the
// other is a failure rather than a silently weaker test.
const MIN_H = 320;
const MAX_H = 900;
const PREFERRED_VH = 0.6;
// .grid carries a 1px border and `* { box-sizing: border-box }`, so its border
// box IS the specified size and the canvas's CSS box is that minus the two
// borders. Named rather than sprinkled, because a wrong guess here would look
// like a rendering bug.
const BORDER = 2;

/** What the board actually is, in the browser, right now. */
async function readBoard(page: Page) {
  return page.evaluate(() => {
    const grid = document.querySelector(".grid") as HTMLElement;
    const canvas = grid.querySelector("canvas") as HTMLCanvasElement;
    const box = grid.getBoundingClientRect();
    const cbox = canvas.getBoundingClientRect();
    return {
      gridW: box.width,
      gridH: box.height,
      cssW: cbox.width,
      cssH: cbox.height,
      backingW: canvas.width,
      backingH: canvas.height,
      dpr: window.devicePixelRatio,
    };
  });
}

/**
 * Open as the DM and make sure the table has a scene, because renderGrid draws
 * "No scene yet." and NO .grid at all without one — every assertion here is
 * about an element that only exists once something has been loaded.
 *
 * Idempotent: the session and the adventure live on the SERVER, so the second
 * and later tests in this file find them already there and skip both steps.
 */
async function openTableAsDM(page: Page) {
  await page.goto(`${base}/?token=${tokens.dm}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
  const start = page.locator('[data-action="start-session"]');
  if ((await start.count()) > 0) {
    await page.locator('[data-field="session-name"]').fill("Viewport");
    await start.click();
    await expect(page.locator(".session")).toContainText("Viewport", { timeout: 10_000 });
  }
  if ((await page.locator(".grid").count()) === 0) {
    await page.getByRole("button", { name: /^Load / }).first().click();
  }
  await expect(page.locator(".grid")).toBeVisible({ timeout: 15_000 });
  await settleBoard(page);
}

/**
 * Wait until the CANVAS has caught up with the .grid box, on BOTH axes.
 *
 * The two move on different schedules and that is the design: .grid is fluid, so
 * the browser resizes it as part of layout, while the canvas is sized by
 * renderSpectator on the next paint — which a ResizeObserver on #app triggers
 * (app.ts). Asserting immediately after setViewportSize would be racing that
 * repaint, and a test that sometimes reads the old canvas is worse than no test.
 *
 * WIDTH ALONE IS NOT A SETTLE, and it read `cssW` only until 2026-09-05 (review,
 * latent finding). The clamp test below changes the viewport HEIGHT at a fixed
 * width, so a width-only poll is already true the moment it is asked and waits
 * for nothing at all. It passes today because what it asserts next is `gridH`,
 * which is CSS-driven and therefore immediate — but the poll would have been
 * decorative for any assertion about the canvas, and the next person to add one
 * inherits a race rather than a wait.
 *
 * BOTH AXES USE THE SAME RULE because the canvas is sized from the previous
 * frame's `.grid` clientWidth/clientHeight (spectator.ts's paneSize, written
 * onto canvas.style), and clientWidth/clientHeight are the border box minus the
 * two 1px borders on each axis alike.
 */
async function settleBoard(page: Page) {
  await expect
    .poll(async () => {
      const b = await readBoard(page);
      return (
        Math.round(b.cssW) === Math.round(b.gridW - BORDER) &&
        Math.round(b.cssH) === Math.round(b.gridH - BORDER)
      );
    }, { timeout: 10_000 })
    .toBe(true);
}

test("the board fills its container rather than the 640px constant it replaced", async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 1000 });
  await openTableAsDM(page);

  const b = await readBoard(page);
  // #app is two columns, minmax(0,2fr) / minmax(280px,1fr), 16px gap and 16px
  // padding, and .board adds 12px of section padding — so at 1600 the board is
  // ~1000 wide. Asserted as "comfortably wider than the constant" rather than as
  // an exact number, because the exact number is the stylesheet's business and
  // pinning it here would make every layout tweak a failure in this file.
  expect(b.gridW).toBeGreaterThan(800);
  expect(Math.round(b.cssW)).toBe(Math.round(b.gridW - BORDER));
  // The canvas is genuinely that wide, which is the half that would still be
  // broken if paneSize fell back while the CSS stretched: a 640px canvas
  // letterboxed inside a 1000px box.
  expect(b.cssW).toBeGreaterThan(800);
  await page.screenshot(shot("10-board-wide-viewport"));
});

test("the board CHANGES with the viewport, which is the coupling one measurement cannot show", async ({ page }) => {
  // BOTH VIEWPORTS ARE WIDE ENOUGH THAT A 640px BOARD WOULD NOT MOVE, and that
  // is the whole design of this fixture. Measured 2026-09-05 while
  // fault-injecting: with the stylesheet reverted to `width: 640px`, comparing
  // 1600 against 900 STILL PASSED — .grid's `max-width: 100%` shrinks a fixed
  // 640 board once the column is narrower than that, so the box changed for a
  // reason that has nothing to do with following the container. At 1150 the
  // column is ~709px, so a fixed board reports 640 at both sizes and the
  // assertions below fail, which is what makes them about the coupling.
  await page.setViewportSize({ width: 1600, height: 1000 });
  await openTableAsDM(page);
  const wide = await readBoard(page);

  await page.setViewportSize({ width: 1150, height: 1000 });
  await settleBoard(page);
  const narrow = await readBoard(page);

  // Still wider than the constant, which is what rules out "pinned at 640 and
  // then capped by max-width" as an explanation for the change.
  expect(narrow.gridW).toBeGreaterThan(640);

  // BOTH the box and the canvas moved, and moved the same way. A CSS-only change
  // would move the first and leave the second — which is the board letterboxed
  // inside its own container, and is what shipping the stylesheet without
  // paneSize would have produced.
  expect(narrow.gridW).toBeLessThan(wide.gridW);
  expect(narrow.cssW).toBeLessThan(wide.cssW);
  expect(Math.round(narrow.cssW)).toBe(Math.round(narrow.gridW - BORDER));
  // SHOT WHILE STILL NARROW, which is the whole point of the pair: the first
  // version of this test restored the viewport before shooting and the two demo
  // images came out byte-identical (measured 2026-09-05, both 510331 bytes).
  // A before/after that is the same picture twice says nothing from a phone.
  await page.screenshot(shot("11-board-narrow-viewport"));

  // And back again, so this is a coupling rather than a one-way shrink.
  await page.setViewportSize({ width: 1600, height: 1000 });
  await settleBoard(page);
  const again = await readBoard(page);
  expect(Math.round(again.cssW)).toBe(Math.round(wide.cssW));
});

test("the clamp holds at BOTH ends, which is what keeps a board from growing the page", async ({ page }) => {
  // BACKLOG T1/#19 IS WHAT THIS PROTECTS. The board was gridWidth*CELL px tall —
  // 1408 for a 32x32 scene — so the page grew with the map and the controls sat
  // ~1450px down it, below every laptop fold. The height now follows the
  // VIEWPORT and never the content, and the two bounds are what make that safe
  // in both directions.
  await page.setViewportSize({ width: 1400, height: 2000 });
  await openTableAsDM(page);
  const tall = await readBoard(page);
  // 60vh of 2000 is 1200, and the ceiling is what stops it. Without the ceiling
  // this is the old defect in a new coordinate system.
  expect(Math.round(tall.gridH)).toBe(MAX_H);
  expect(2000 * PREFERRED_VH).toBeGreaterThan(MAX_H); // the fixture really does exercise the ceiling

  await page.setViewportSize({ width: 1400, height: 400 });
  await settleBoard(page);
  const short = await readBoard(page);
  // 60vh of 400 is 240, and the floor is what stops it. Without the floor the
  // board collapses to a strip on a laptop with a browser toolbar open.
  expect(Math.round(short.gridH)).toBe(MIN_H);
  expect(400 * PREFERRED_VH).toBeLessThan(MIN_H); // the fixture really does exercise the floor
  // The short viewport is the one worth looking at: it is the shape backlog
  // T1/#19 was about, and the board is still a usable board rather than a strip.
  await page.screenshot(shot("13-board-short-viewport"));

  // AND THE HEIGHT IS NEVER THE SCENE'S. Both viewports are showing the same
  // adventure, so a board still sized by its content would report one number
  // twice — and that number would be neither of these.
  expect(tall.gridH).not.toBe(short.gridH);
});

test("the canvas backing store is the CSS box times devicePixelRatio, so the board is not soft", async ({ page }) => {
  // At dpr 1 this says the two are equal, which is the ordinary display and the
  // case every unit test runs in. The describe block below is the one that can
  // tell a correct implementation from a hard-coded 1.
  await page.setViewportSize({ width: 1200, height: 900 });
  await openTableAsDM(page);
  const b = await readBoard(page);
  expect(b.dpr).toBe(1);
  expect(b.backingW).toBe(Math.round(b.cssW * b.dpr));
  expect(b.backingH).toBe(Math.round(b.cssH * b.dpr));
});

test.describe("on a high-DPI display", () => {
  // The assertion happy-dom cannot make at all: it reports devicePixelRatio 1
  // and has no canvas, so a backing store that ignored the ratio — the state
  // this client shipped in until 2026-09-05, soft on every retina screen — was
  // invisible to the whole suite. Playwright gives a real ratio through the
  // browser context.
  test.use({ deviceScaleFactor: 2 });

  test("the backing store is doubled while the CSS box is not", async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 900 });
    await openTableAsDM(page);
    const b = await readBoard(page);
    expect(b.dpr).toBe(2);
    // DOUBLED in device pixels...
    expect(b.backingW).toBe(Math.round(b.cssW * 2));
    expect(b.backingH).toBe(Math.round(b.cssH * 2));
    // ...and UNCHANGED in CSS pixels, or the element itself would draw at twice
    // its box and overflow the container. Both halves, because either alone is a
    // different bug: a backing store without the ctx.scale draws the whole board
    // into its own top-left quarter.
    expect(Math.round(b.cssW)).toBe(Math.round(b.gridW - BORDER));
    expect(Math.round(b.cssH)).toBe(Math.round(b.gridH - BORDER));
    await page.screenshot(shot("12-board-high-dpi"));
  });
});
