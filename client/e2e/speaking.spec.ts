import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";
import { asDM } from "./as-dm";

// Two players at one table: what one says, in character and out of it,
// as the other's Story shows it.

const { base, tokens, ids } = fixture("speaking");

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

async function openAs(page: Page, token: string) {
  await page.goto(`${base}/?token=${token}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
}

// VTT-496 VTT-497 VTT-500 VTT-501 VTT-502
test("another player's Story names a line by its speaker, in character and out of it",
  async ({ browser }) => {
    const leraCtx = await browser.newContext();
    const lera = await leraCtx.newPage();
    await openAs(lera, tokens.dm);
    const party = "ACTOR_KIND_PARTY_MEMBER";
    await asDM(lera, tokens.dm, [
      { startSession: { name: "Speaking" } },
      { loadAdventure: { adventureId: "goblin-ambush" } },
      { grantActorControl: { actorId: "act-fighter", participantId: ids.player, kind: party } },
      { promoteParticipant: { participantId: ids.spectator, role: "player" } },
      { addActor: { actor: { actorId: "act-wren", name: "Wren", kind: party } } },
      { grantActorControl: { actorId: "act-wren", participantId: ids.spectator, kind: party } },
    ]);

    // Every narration Lera's page sends, so the speaker is seen to be the
    // server's.
    const sent: Record<string, unknown>[] = [];
    lera.on("websocket", (ws) =>
      ws.on("framesent", (f) => {
        const c = JSON.parse(String(f.payload)) as { addNarration?: Record<string, unknown> };
        if (c.addNarration) sent.push(c.addNarration);
      }),
    );
    await openAs(lera, tokens.player);
    // Watcher's page opens after the DM's commands, so it boots as a player
    // who plays Wren.
    const watcherCtx = await browser.newContext();
    const watcher = await watcherCtx.newPage();
    await openAs(watcher, tokens.spectator);

    const say = lera.locator('.player .say input[data-field="say-text"]');
    await expect(say).toBeVisible({ timeout: 10_000 });
    await expect(lera.locator('[data-field="say-as"]')).toHaveCount(0);
    await say.fill("I draw my sword");
    await say.press("Enter");
    const spoken = watcher.locator(".feed p").filter({ hasText: "I draw my sword" });
    await expect(spoken).toHaveText("Human Fighter: I draw my sword", { timeout: 10_000 });
    await expect(spoken).toHaveClass(/speech/);
    // Wait for Lera's own answer, not the watcher's feed, before the next
    // line: Enter sends nothing while a line is unanswered (VTT-523).
    await expect(say).toHaveValue("", { timeout: 10_000 });

    await say.fill("/ooc brb");
    await say.press("Enter");
    const aside = watcher.locator(".feed p").filter({ hasText: "brb" });
    await expect(aside).toHaveText("Lera: (( brb ))", { timeout: 10_000 });
    await expect(aside).toHaveClass(/ooc/);

    expect(sent).toEqual([
      { text: "I draw my sword", actorId: "act-fighter" },
      { text: "brb", outOfCharacter: true },
    ]);
    await leraCtx.close();
    await watcherCtx.close();
  });
