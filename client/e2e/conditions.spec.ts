import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { asDM } from "./as-dm";
import { fixture } from "./setup";

const { base, tokens } = fixture("conditions");

test.afterAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${base}/?token=${tokens.dm}`);
  await page.locator(".dm").waitFor({ timeout: 15_000 });
  const end = page.locator('[data-action="end-session"]');
  if ((await end.count()) > 0) await end.click();
  await ctx.close();
});

async function openAs(page: Page, token: string) {
  await page.goto(`${base}/?token=${token}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
}

const shipped = join(dirname(fileURLToPath(import.meta.url)), "..", "..", "rulesets", "dnd45e-minimal");
const dazed = JSON.parse(readFileSync(join(shipped, "conditions", "dazed.json"), "utf8")) as {
  id: string; name: string; description: string;
};

const attributes = (fort: number) => ({ str: 0, dex: 0, con: 0, max_hp: 20, ac: 10, fort, ref: 10, will: 10 });

// VTT-862 VTT-863 VTT-864 VTT-865
test("a DM's window names a condition a hit applies on its dot, its Story, its Events panel and its removal button", async ({ page }) => {
  expect(dazed.name).not.toBe("");
  expect(dazed.description).not.toBe("");
  await openAs(page, tokens.dm);
  const npc = "ACTOR_KIND_NON_PARTY";
  await asDM(page, tokens.dm, [
    { startSession: { name: "Conditions" } },
    { loadAdventure: { adventureId: "goblin-ambush" } },
    { addActor: { actor: {
      actorId: "act-striker", name: "Striker", kind: npc, attributes: attributes(10),
      resources: { hp: { current: 20, max: 20 } }, abilityIds: ["staggering-blow"],
    } } },
    { addActor: { actor: {
      actorId: "act-dummy", name: "Dummy", kind: npc, attributes: attributes(1),
      resources: { hp: { current: 20, max: 20 } },
    } } },
    { placeToken: { tokenId: "tok-striker", sceneId: "ravine", actorId: "act-striker", position: { x: 10, y: 10 } } },
    { placeToken: { tokenId: "tok-dummy", sceneId: "ravine", actorId: "act-dummy", position: { x: 11, y: 10 } } },
    { useAbility: { actorId: "act-striker", abilityId: "staggering-blow", targetIds: ["act-dummy"] } },
  ]);

  const dot = page.locator('.token[data-token-id="tok-dummy"] .dot');
  await expect(dot).toHaveCount(1, { timeout: 10_000 });
  await expect(dot).toHaveAttribute("title", `${dazed.name}\n${dazed.description}`);
  await expect(page.locator(".feed")).toContainText(`Dummy gained ${dazed.name}`);
  await expect(page.locator(".ticker .tick").filter({ hasText: `Dummy gained ${dazed.name}` })).toHaveCount(1);

  const remove = page.locator('[data-action="remove-condition"]');
  await expect(remove).toHaveText([`Dummy: ${dazed.name} ✕`]);
  await remove.click();
  await expect(page.locator(".feed")).toContainText(`Dummy lost ${dazed.name}`, { timeout: 10_000 });
  await expect(dot).toHaveCount(0);
});
