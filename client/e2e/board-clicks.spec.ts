import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { asDM } from "./as-dm";
import { fixture } from "./setup";

const { base, tokens, ids } = fixture("board-clicks");

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
type Square = { x: number; y: number };
const west: Square = { x: dx - 1, y: dy };
const east: Square = { x: dx + 1, y: dy };
// Seen through the open door, remembered once it closes.
const beyond: Square = { x: dx + 2, y: dy };
// In the fighter's own room, seen whatever the door does.
const floor: Square = { x: dx - 3, y: dy };

// VTT-935
test("a player's click on a square they no longer see sends nothing, and one on a seen floor moves", async ({ browser }) => {
  for (const sq of [west, east, beyond, floor]) {
    expect(cellar.tiles[`${sq.x},${sq.y}`]).not.toMatch(/wall|door/);
    expect(cellar.objects.some((o) => o.at[0] === sq.x && o.at[1] === sq.y)).toBe(false);
  }
  const [dmCtx, playerCtx] = await Promise.all([browser.newContext(), browser.newContext()]);
  const [dm, player] = await Promise.all([dmCtx.newPage(), playerCtx.newPage()]);
  const party = "ACTOR_KIND_PARTY_MEMBER";
  const sheet = {
    attributes: { str: 0, dex: 0, con: 0, max_hp: 20, ac: 10, fort: 10, ref: 10, will: 10 },
    resources: { hp: { current: 20, max: 20 } },
  };
  await openAs(dm, tokens.dm);
  await asDM(dm, tokens.dm, [
    { startSession: { name: "Board clicks" } },
    { addActor: { actor: { actorId: "act-fighter", name: "Brannoc", kind: party, ...sheet } } },
    { addActor: { actor: { actorId: "act-cutpurse", name: "Cutpurse", kind: "ACTOR_KIND_NON_PARTY", ...sheet } } },
    { grantActorControl: { actorId: "act-fighter", participantId: ids.player, kind: party } },
    { loadMap: { mapId: cellar.id } },
    { moveToken: { tokenId: "tok-fighter", to: west } },
    { placeToken: { tokenId: "tok-cutpurse", sceneId: cellar.id, actorId: "act-cutpurse", position: east } },
  ]);

  // Every command the player's page sends: a frame not sent cannot be refused.
  const sent: Record<string, unknown>[] = [];
  player.on("websocket", (ws) =>
    ws.on("framesent", (f) => sent.push(JSON.parse(String(f.payload)) as Record<string, unknown>)),
  );
  await openAs(player, tokens.player);
  const fighter = player.locator('[data-token-id="tok-fighter"]');
  const cutpurse = player.locator('[data-token-id="tok-cutpurse"]');
  await expect(fighter).toBeVisible({ timeout: 10_000 });
  await asDM(dm, tokens.dm, [{ openDoor: { sceneId: cellar.id, at: { x: dx, y: dy } } }]);
  await expect(cutpurse).toBeVisible({ timeout: 10_000 });
  await asDM(dm, tokens.dm, [{ closeDoor: { sceneId: cellar.id, at: { x: dx, y: dy } } }]);
  await expect(cutpurse).toHaveCount(0, { timeout: 10_000 });

  // A disc's box is one square of the board's camera: the fighter's places them all.
  const disc = (await fighter.boundingBox())!;
  const step = disc.width;
  const centre = (sq: Square) => ({
    x: disc.x + (sq.x - west.x + 0.5) * step, y: disc.y + (sq.y - west.y + 0.5) * step,
  });
  await player.mouse.click(centre(beyond).x, centre(beyond).y);
  // Before the move, whose answer would clear a toast an earlier click drew.
  await expect(player.locator(".toast")).toHaveCount(0);
  await player.mouse.click(centre(floor).x, centre(floor).y);
  await expect
    .poll(async () => (await fighter.boundingBox())?.x, { timeout: 10_000 })
    .toBeCloseTo(disc.x + (floor.x - west.x) * step, 0);

  expect(sent.map(({ requestId: _, ...c }) => c)).toEqual([{ moveToken: { tokenId: "tok-fighter", to: floor } }]);
  await Promise.all([dmCtx.close(), playerCtx.close()]);
});
