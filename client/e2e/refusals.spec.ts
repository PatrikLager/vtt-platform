import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";
import { asDM } from "./as-dm";

// SPEC-022 is the record of what a refused or an accepted command leaves typed.

const { base, tokens, ids } = fixture("refusals");

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

const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png` });

async function openAsDM(page: Page) {
  await page.goto(`${base}/?token=${tokens.dm}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
  // Type only once the adventures' paint has drawn the Load buttons: an
  // earlier paint replaces the box being filled.
  await expect(page.getByRole("button", { name: /^Load / }).first()).toBeVisible({
    timeout: 10_000,
  });
}

// VTT-517
test("a refused Place leaves its five boxes as typed, beside the refusal",
  async ({ page }) => {
    await openAsDM(page);
    const typed: [string, string][] = [
      ["token-id", "tok-x"],
      ["token-scene", "nowhere"],
      ["token-actor", "nobody"],
      ["token-x", "2"],
      ["token-y", "3"],
    ];
    const box = (field: string) => page.locator(`[data-field="${field}"]`);
    for (const [field, value] of typed) await box(field).fill(value);
    // Read before the click as well: a fill a repaint lost leaves x or y
    // empty, which sends 0 and is refused too, and must fail here.
    for (const [field, value] of typed) await expect(box(field)).toHaveValue(value);

    await page.locator('[data-action="place-token"]').click();
    // Match `refused: ` and nothing looser: the console's own guard toasts too.
    await expect(page.locator(".toast")).toHaveText(/^refused: /, {
      timeout: 10_000,
    });
    for (const [field, value] of typed) await expect(box(field)).toHaveValue(value);

    await page.locator('[data-action="place-token"]').scrollIntoViewIfNeeded();
    await page.screenshot(shot("r1-refused-place-keeps-what-was-typed"));
  });

// VTT-520 VTT-521
test("a DM's accepted line empties say-text and keeps say-as for the next line",
  async ({ page }) => {
    await openAsDM(page);
    await asDM(page, tokens.dm!, [
      { startSession: { name: "Refusals" } },
      { loadAdventure: { adventureId: "goblin-ambush" } },
      {
        grantActorControl: {
          actorId: "act-cutter", participantId: ids.dm, kind: "ACTOR_KIND_NON_PARTY",
        },
      },
    ]);

    const speaker = page.locator('.player .say input[data-field="say-as"]');
    const say = page.locator('.player .say input[data-field="say-text"]');
    const line = (text: string) => page.locator(".feed p").filter({ hasText: text });
    await expect(speaker).toBeVisible({ timeout: 10_000 });
    await speaker.fill("Grimble");
    await say.fill("Hello");
    await say.press("Enter");
    await expect(line("Hello")).toHaveText("Grimble: Hello", { timeout: 10_000 });
    // Wait for the accepted answer before the next line: Enter sends nothing
    // while a line is unanswered (VTT-523).
    await expect(say).toHaveValue("", { timeout: 10_000 });
    await expect(speaker).toHaveValue("Grimble");

    await say.fill("Bye");
    await say.press("Enter");
    await expect(line("Bye")).toHaveText("Grimble: Bye", { timeout: 10_000 });
    await expect(say).toHaveValue("", { timeout: 10_000 });
    await expect(speaker).toHaveValue("Grimble");
  });
