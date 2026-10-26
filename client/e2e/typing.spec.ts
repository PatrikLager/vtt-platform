import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";

// What someone is typing when a repaint arrives, in a real browser: the
// focus, the page's scroll, and the text of a box that keeps none itself.

const { base, tokens, ids } = fixture("typing");

// Leave the table as this file found it, as table.spec.ts does.
test.afterAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${base}/?token=${tokens.dm}`);
  await page.locator(".dm").waitFor({ timeout: 15_000 });
  const end = page.locator('[data-action="end-session"]');
  if ((await end.count()) > 0) await end.click();
  await ctx.close();
});

// Short enough that the console's session-name box can leave the viewport,
// in Chromium's full headless: the default shell keeps every page focused,
// emulation on or off, so a page in the background still matches :focus.
test.use({ viewport: { width: 1280, height: 300 }, channel: "chromium" });

async function openAsDM(page: Page) {
  await page.goto(`${base}/?token=${tokens.dm}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
  // The console is drawn once an adventure is listed. A later paint, the
  // maps' answer or a presence frame, only hands the focus on again.
  await expect(page.getByRole("button", { name: /^Load / }).first()).toBeVisible({
    timeout: 10_000,
  });
}

/** Add a narration from a second connection, and wait for it in the feed. */
async function narrate(page: Page, text: string) {
  await page.evaluate(
    async ([t, x]) => {
      const ws = new WebSocket(`${location.origin.replace("http", "ws")}/ws?token=${t}&after=0`);
      await new Promise((r) => ws.addEventListener("open", r));
      ws.send(JSON.stringify({ requestId: `e2e-${x}`, addNarration: { text: x } }));
    },
    [tokens.dm, text] as const,
  );
  await expect(page.locator(".feed")).toContainText(text, { timeout: 10_000 });
}

/** The page's scroll once two frames have laid out what the last repaint drew. */
const scrollY = (page: Page) =>
  page.evaluate(
    () =>
      new Promise<number>((r) =>
        requestAnimationFrame(() => requestAnimationFrame(() => r(window.scrollY))),
      ),
  );

// VTT-491 VTT-494
test("a box below the viewport keeps the focus across a repaint, and the page stays put",
  async ({ page }) => {
    await openAsDM(page);
    const box = page.locator('[data-field="session-name"]');
    await box.focus();
    // Scrolled and measured in one call, so no repaint falls between them,
    // and the box is read where the call runs: a repaint can replace it at
    // any moment, as the second connection's presence does, and a detached
    // box measures all zeros.
    const top = await page.evaluate(() => {
      window.scrollTo(0, 0);
      return document.querySelector('[data-field="session-name"]')!.getBoundingClientRect().top;
    });
    expect(top).toBeGreaterThan(300);

    // Between the focus and the typing, as Playwright's own fill is.
    await narrate(page, "a repaint below");
    expect(await scrollY(page)).toBe(0);

    await page.keyboard.insertText("Open Table");
    await page.locator('[data-action="start-session"]').click();
    await expect(page.locator(".session")).toHaveText("session: Open Table", {
      timeout: 10_000,
    });
    await page.locator('[data-action="end-session"]').click();
    await expect(page.locator('[data-action="start-session"]')).toBeVisible({
      timeout: 10_000,
    });
  });

// VTT-494
test("a repaint leaves the page put when the focused box is above the viewport",
  async ({ page }) => {
    await openAsDM(page);
    // A live event comes after the log's catch-up, so the rows above the box
    // have their final height before the box is measured and scrolled past.
    await narrate(page, "the table settles");
    const box = page.locator('[data-field="session-name"]');
    await box.focus();
    const at = await page.evaluate(() => {
      const box = () => document.querySelector('[data-field="session-name"]')!;
      window.scrollTo(0, Math.ceil(box().getBoundingClientRect().bottom + window.scrollY + 50));
      return { y: window.scrollY, bottom: box().getBoundingClientRect().bottom };
    });
    expect(at.bottom).toBeLessThan(0);

    await narrate(page, "a repaint above");
    expect(await scrollY(page)).toBe(at.y);
  });

// VTT-494
test("a repaint leaves the page put when the focused box is in view, scrolled down",
  async ({ page }) => {
    await openAsDM(page);
    await narrate(page, "the table settles");
    const box = page.locator('[data-field="session-name"]');
    await box.focus();
    const at = await page.evaluate(() => {
      const box = () => document.querySelector('[data-field="session-name"]')!;
      window.scrollTo(0, Math.ceil(box().getBoundingClientRect().top + window.scrollY - 100));
      const r = box().getBoundingClientRect();
      return { y: window.scrollY, top: r.top, bottom: r.bottom };
    });
    expect(at.y).toBeGreaterThan(0);
    expect(at.top).toBeGreaterThanOrEqual(0);
    expect(at.bottom).toBeLessThanOrEqual(300);

    await narrate(page, "a repaint in view");
    expect(await scrollY(page)).toBe(at.y);
  });

// VTT-491
test("a box in a background tab keeps the focus across a repaint",
  async ({ page, context }) => {
    await openAsDM(page);
    const box = page.locator('[data-field="session-name"]');
    await box.focus();
    await page.evaluate(() => {
      const b = document.querySelector<HTMLInputElement>('[data-field="session-name"]')!;
      b.value = "Open";
      b.dispatchEvent(new Event("input"));
      b.setSelectionRange(2, 2);
    });
    const cdp = await context.newCDPSession(page);
    await cdp.send("Emulation.setFocusEmulationEnabled", { enabled: false });
    const front = await context.newPage();
    await front.bringToFront();
    expect(await page.evaluate(() => document.hasFocus())).toBe(false);
    expect(await page.evaluate(() => document.querySelector(":focus") === null)).toBe(true);

    await narrate(page, "a repaint behind");
    await page.bringToFront();
    await page.evaluate(() => window.focus());

    const at = await page.evaluate(() => {
      const a = document.activeElement as HTMLInputElement;
      return { field: a.dataset["field"], caret: [a.selectionStart, a.selectionEnd] };
    });
    expect(at).toEqual({ field: "session-name", caret: [2, 2] });
    await page.keyboard.insertText("X");
    await expect(box).toHaveValue("OpXen");
  });

/** Send the DM's commands on a raw socket, one at a time; any refusal fails the test. */
async function asDM(page: Page, commands: Record<string, unknown>[]) {
  const refused = await page.evaluate(
    async ([t, cs]) => {
      const ws = new WebSocket(`${location.origin.replace("http", "ws")}/ws?token=${t}&after=0`);
      await new Promise((r) => ws.addEventListener("open", r));
      const out: string[] = [];
      for (const [i, c] of cs.entries()) {
        const requestId = `e2e-dm-${i}`;
        const answer = new Promise<{ ok?: boolean; error?: string }>((r) =>
          ws.addEventListener("message", (m) => {
            const f = JSON.parse(String(m.data));
            if (f.result?.requestId === requestId) r(f.result);
          }),
        );
        ws.send(JSON.stringify({ requestId, ...c }));
        const res = await answer;
        if (!res.ok) out.push(`${Object.keys(c)[0]}: ${res.error}`);
      }
      ws.close();
      return out;
    },
    [tokens.dm, commands] as const,
  );
  expect(refused).toEqual([]);
}

// VTT-491 VTT-493
test("what a player types in Say survives a repaint, focus and all", async ({ page }) => {
  await page.goto(`${base}/?token=${tokens.dm}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
  await asDM(page, [
    { startSession: { name: "Say" } },
    { loadAdventure: { adventureId: "goblin-ambush" } },
    {
      grantActorControl: {
        actorId: "act-fighter", participantId: ids.player, kind: "ACTOR_KIND_PARTY_MEMBER",
      },
    },
  ]);

  await page.goto(`${base}/?token=${tokens.player}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
  const say = page.locator('.player .say input[data-field="say-text"]');
  await expect(say).toBeVisible({ timeout: 10_000 });
  await say.focus();
  await page.keyboard.type("I draw my");

  await narrate(page, "the wind rises");

  // Retrying matchers: the narration brings more than one repaint, and a
  // box read once may already be gone.
  await expect(say).toHaveValue("I draw my");
  await expect(say).toBeFocused();
});

