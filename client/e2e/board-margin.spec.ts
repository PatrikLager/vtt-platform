import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { asDM } from "./as-dm";
import { fixture } from "./setup";

const { base, tokens, ids } = fixture("board-margin");

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

// The ravine's edge is open ground, so a click beside it that landed on its
// edge square would be a move the server allows, not a wall the client stops.
const repo = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const ravine = JSON.parse(readFileSync(join(repo, "adventures", "goblin-ambush", "scenes", "ravine.json"), "utf8")) as {
  tiles: Record<string, string>; objects?: unknown[];
  placements: { token_id: string; x: number; y: number }[];
};
const start = ravine.placements.find((p) => p.token_id === "tok-fighter")!;
const edge = { x: 0, y: start.y + 1 };
const open = { x: start.x + 2, y: start.y + 1 };

// VTT-934
test("a player's click beside the ravine's grid sends nothing, and one on a seen square of it moves", async ({ browser }) => {
  expect(start.x).toBe(0);
  expect(ravine.objects ?? []).toHaveLength(0);
  for (const sq of [edge, open]) expect(ravine.tiles[`${sq.x},${sq.y}`]).toBe("earth");
  expect(ravine.placements.some((p) => p.x === edge.x && p.y === edge.y)).toBe(false);

  const [dmCtx, playerCtx] = await Promise.all([browser.newContext(), browser.newContext()]);
  const [dm, player] = await Promise.all([dmCtx.newPage(), playerCtx.newPage()]);
  const party = "ACTOR_KIND_PARTY_MEMBER";
  await openAs(dm, tokens.dm);
  await asDM(dm, tokens.dm, [
    { startSession: { name: "Margin" } },
    { loadAdventure: { adventureId: "goblin-ambush" } },
    { grantActorControl: { actorId: "act-fighter", participantId: ids.player, kind: party } },
  ]);

  const sent: Record<string, unknown>[] = [];
  player.on("websocket", (ws) =>
    ws.on("framesent", (f) => sent.push(JSON.parse(String(f.payload)) as Record<string, unknown>)),
  );
  await openAs(player, tokens.player);
  const fighter = player.locator('[data-token-id="tok-fighter"]');
  await expect(fighter).toBeVisible({ timeout: 10_000 });

  // A disc's box is one square of the board's camera: the fighter's places the grid.
  const disc = (await fighter.boundingBox())!;
  const step = disc.width;
  const [gridLeft, gridTop] = [disc.x - start.x * step, disc.y - start.y * step];
  const board = (await player.locator(".grid").boundingBox())!;
  expect(gridLeft - board.x).toBeGreaterThan(step);

  await player.mouse.click(board.x + (gridLeft - board.x) / 2, gridTop + (edge.y + 0.5) * step);
  // Before the move, whose answer would clear a toast the earlier click drew.
  await expect(player.locator(".toast")).toHaveCount(0);
  await player.mouse.click(gridLeft + (open.x + 0.5) * step, gridTop + (open.y + 0.5) * step);
  await expect
    .poll(async () => (await fighter.boundingBox())?.x, { timeout: 10_000 })
    .toBeCloseTo(gridLeft + open.x * step, 0);

  expect(sent.map(({ requestId: _, ...c }) => c)).toEqual([{ moveToken: { tokenId: "tok-fighter", to: open } }]);
  await Promise.all([dmCtx.close(), playerCtx.close()]);
});
