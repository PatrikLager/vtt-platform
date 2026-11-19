import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { asDM } from "./as-dm";
import { fixture } from "./setup";

const { base, tokens, ids } = fixture("doors");

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

const repo = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const cellar = JSON.parse(readFileSync(join(repo, "campaigns", "example", "maps", "cellar.json"), "utf8")) as {
  id: string; tiles: Record<string, string>; objects: { at: [number, number] }[];
};
const doorKey = Object.keys(cellar.tiles).find((k) => cellar.tiles[k]!.includes("door"))!;
const [dx, dy] = doorKey.split(",").map(Number) as [number, number];
const west = { x: dx - 1, y: dy };
const east = { x: dx + 1, y: dy };

// Measure in the page in one step: a repaint replaces the tokens' elements,
// never their geometry.
async function between(page: Page, a: string, b: string) {
  return page.evaluate(([one, two]) => {
    const at = (id: string) => document.querySelector(`[data-token-id="${id}"]`)!;
    at(one).scrollIntoView({ block: "center" });
    const [r1, r2] = [at(one).getBoundingClientRect(), at(two).getBoundingClientRect()];
    return { x: (r1.x + r1.width / 2 + r2.x + r2.width / 2) / 2, y: (r1.y + r1.height / 2 + r2.y + r2.height / 2) / 2 };
  }, [a, b] as const);
}

// VTT-869 VTT-870 VTT-871 VTT-872 VTT-873
test("the Story names who opened and closed a door to whoever saw them, and the DM's Events panel says where", async ({ browser }) => {
  for (const sq of [west, east]) {
    expect(cellar.tiles[`${sq.x},${sq.y}`]).not.toMatch(/wall|door/);
    expect(cellar.objects.some((o) => o.at[0] === sq.x && o.at[1] === sq.y)).toBe(false);
  }
  const [dmCtx, playerCtx] = await Promise.all([browser.newContext(), browser.newContext()]);
  const [dm, player] = await Promise.all([dmCtx.newPage(), playerCtx.newPage()]);
  const party = "ACTOR_KIND_PARTY_MEMBER";
  const npc = "ACTOR_KIND_NON_PARTY";
  const sheet = {
    attributes: { str: 0, dex: 0, con: 0, max_hp: 20, ac: 10, fort: 10, ref: 10, will: 10 },
    resources: { hp: { current: 20, max: 20 } },
  };
  await openAs(dm, tokens.dm);
  await asDM(dm, tokens.dm, [
    { startSession: { name: "Doors" } },
    { addActor: { actor: { actorId: "act-fighter", name: "Brannoc", kind: party, ...sheet } } },
    { addActor: { actor: { actorId: "act-cutpurse", name: "Cutpurse", kind: npc, ...sheet } } },
    { grantActorControl: { actorId: "act-fighter", participantId: ids.player, kind: party } },
    { loadMap: { mapId: cellar.id } },
    { moveToken: { tokenId: "tok-fighter", to: west } },
    { placeToken: { tokenId: "tok-cutpurse", sceneId: cellar.id, actorId: "act-cutpurse", position: east } },
  ]);

  await openAs(player, tokens.player);
  await expect(player.locator('[data-token-id="tok-fighter"]')).toBeVisible({ timeout: 10_000 });

  await asDM(dm, tokens.dm, [{ openDoor: { sceneId: cellar.id, at: { x: dx, y: dy }, actorId: "act-cutpurse" } }]);
  await expect(player.locator('[data-token-id="tok-cutpurse"]')).toBeVisible({ timeout: 10_000 });

  await player.locator('[data-action="arm-doors"]').click();
  await expect(player.locator(".armed-label")).toBeVisible();
  const door = await between(player, "tok-fighter", "tok-cutpurse");
  await player.mouse.click(door.x, door.y);
  await expect(player.locator(".feed")).toContainText("Brannoc closed a door", { timeout: 10_000 });
  // The close's frames follow the open's on one socket, so the open's line, had
  // it been sent, is drawn by now.
  await expect(player.locator(".feed")).not.toContainText("opened a door");

  await expect(dm.locator(".feed")).toContainText("Cutpurse opened a door", { timeout: 10_000 });
  await expect(dm.locator(".feed")).toContainText("Brannoc closed a door");
  await expect(dm.locator(".ticker .tick").filter({ hasText: `Cutpurse opened a door at ${dx},${dy}` })).toHaveCount(1);
  await expect(dm.locator(".ticker .tick").filter({ hasText: `Brannoc closed a door at ${dx},${dy}` })).toHaveCount(1);
  await Promise.all([dmCtx.close(), playerCtx.close()]);
});
