import { expect, test, type Page } from "@playwright/test";
import { fixture } from "./setup";

// Independent QA of a repaint keeping the box being typed in, in a real
// Chromium: VTT-494 and the ticket's Done items 5 and 11, from the ticket
// 2026-10-25-a-repaint-keeps-what-is-being-typed alone.

const t = fixture("qa-repaint");

interface Frame {
  event?: { sessionStarted?: { name?: string }; sessionEnded?: object };
  result?: { requestId?: string; ok?: boolean; error?: string };
}

interface Line {
  frames: Frame[];
  send(cmd: Record<string, unknown>): void;
  close(): void;
}

async function dial(token: string): Promise<Line> {
  const url = `${t.base.replace(/^http/, "ws")}/ws?token=${token}&after=0`;
  const ws = new WebSocket(url);
  const frames: Frame[] = [];
  ws.onmessage = (m) => frames.push(JSON.parse(String(m.data)) as Frame);
  await new Promise<void>((ok, bad) => {
    ws.onopen = () => ok();
    ws.onerror = () => bad(new Error(`could not dial ${url}`));
  });
  let n = 0;
  return {
    frames,
    send: (cmd) => ws.send(JSON.stringify({ requestId: `qa-${++n}`, ...cmd })),
    close: () => ws.close(),
  };
}

async function openAsDM(page: Page): Promise<void> {
  await page.goto(`${t.base}/?token=${t.tokens["dm"]}`);
  await expect(page.locator(".conn")).toHaveText("connected");
  await expect(page.locator('[data-field="session-name"]')).toBeVisible();
}

async function settle(page: Page): Promise<void> {
  await page.evaluate(
    () => new Promise<void>((r) =>
      requestAnimationFrame(() => requestAnimationFrame(() => r()))),
  );
}

interface Seen {
  scrollY: number;
  field: string | null;
  value: string;
  start: number | null;
  end: number | null;
}

function seen(page: Page): Promise<Seen> {
  return page.evaluate(() => {
    const a = document.activeElement as HTMLInputElement | null;
    return {
      scrollY: window.scrollY,
      field: a?.getAttribute("data-field") ?? null,
      value: a?.value ?? "",
      start: a?.selectionStart ?? null,
      end: a?.selectionEnd ?? null,
    };
  });
}

async function typeOpenCaretAt2(page: Page): Promise<void> {
  await page.locator('[data-field="session-name"]').click();
  await page.keyboard.type("Open");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.press("ArrowLeft");
}

// Whether the session-name box lies wholly below or above the viewport.
function boxIs(page: Page, where: "below" | "above"): Promise<boolean> {
  return page.evaluate((w) => {
    const el = document.querySelector('[data-field="session-name"]');
    if (!el) return false;
    const r = el.getBoundingClientRect();
    return w === "below" ? r.top >= window.innerHeight : r.bottom <= 0;
  }, where);
}

// A plain focus() on the box, as a check that the fixture is one where a
// focus that scrolls would move the page.
async function plainFocusMoves(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const el = document.querySelector('[data-field="session-name"]');
    if (!(el instanceof HTMLElement)) return false;
    const y = window.scrollY;
    el.blur();
    el.focus();
    return window.scrollY !== y;
  });
}

// Marks the box as it stands, so a later read can tell a repaint built a
// new one: without a repaint every assertion below holds vacuously.
async function markBox(page: Page): Promise<void> {
  await page.evaluate(() => {
    const el = document.querySelector('[data-field="session-name"]');
    if (el) el.setAttribute("data-qa-old", "");
  });
}

function boxIsNew(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const el = document.querySelector('[data-field="session-name"]');
    return el !== null && !el.hasAttribute("data-qa-old");
  });
}

let started = false;

test.afterAll(async () => {
  if (!started) return;
  const line = await dial(t.tokens["dm"]!);
  line.send({ endSession: {} });
  await expect.poll(() => line.frames.some((f) => f.event?.sessionEnded)).toBe(true);
  line.close();
});

for (const where of ["below", "above"] as const) {
  // VTT-494
  test(`qaRP: a box scrolled ${where} the viewport keeps the page still`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: 900, height: 260 });
    await openAsDM(page);
    await typeOpenCaretAt2(page);

    await page.evaluate((w) => {
      window.scrollTo(0, w === "below" ? 0 : document.documentElement.scrollHeight);
    }, where);
    await settle(page);
    expect(await boxIs(page, where)).toBe(true);
    const before = await seen(page);
    expect(before.field).toBe("session-name");
    if (where === "above") expect(before.scrollY).toBeGreaterThan(0);
    await markBox(page);

    const line = await dial(t.tokens["dm"]!);
    const words = `qa-${where}-${Date.now()}`;
    try {
      line.send({ addNarration: { text: words } });
      await expect(page.locator(".feed")).toContainText(words);
    } finally {
      line.close();
    }
    await settle(page);
    expect(await boxIsNew(page)).toBe(true);

    const after = await seen(page);
    expect(after.scrollY).toBe(before.scrollY);
    expect(after.field).toBe("session-name");
    expect(after.value).toBe("Open");
    expect([after.start, after.end]).toEqual([2, 2]);
    expect(await plainFocusMoves(page)).toBe(true);
  });
}

// The ticket's Done item 5.
test("qaRP: text inserted after a repaint names the session it starts", async ({
  page,
}) => {
  await openAsDM(page);
  await page.locator('[data-field="session-name"]').click();
  await markBox(page);

  const line = await dial(t.tokens["dm"]!);
  try {
    const words = `qa-start-${Date.now()}`;
    line.send({ addNarration: { text: words } });
    await expect(page.locator(".feed")).toContainText(words);
    expect(await boxIsNew(page)).toBe(true);

    await page.keyboard.insertText("Open Table");
    await page.getByRole("button", { name: "Start session" }).click();
    await expect.poll(() =>
      line.frames.some((f) => f.event?.sessionStarted?.name === "Open Table"),
    ).toBe(true);
    started = true;
  } finally {
    line.close();
  }
});
