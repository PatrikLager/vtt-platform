import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { fixture } from "./setup";

// The demo gate for presence and actor control (spec §8), against the SHIPPED
// binary with TWO real browser contexts.
//
// Every beat below was reachable in unit tests long before it was reachable by
// a person: the contract, both folds, authz and the MCP tools all carried
// grant/revoke while the gateway's ToEvent had no arm and the client had no
// control. Two separate reviews found those two halves. So this file is not
// decoration — it is the only thing in the repo that asserts a HUMAN can do
// what the feature claims, in a browser, end to end.
//
// The screenshots are the deliverable as much as the assertions: Patrik
// reviews from a phone, so "what did each person see" has to be an artifact.

const { base, tokens, ids } = fixture();
const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png`, fullPage: true });

async function expectNoRefusal(page: Page, step: string) {
  if ((await page.locator(".toast").count()) === 0) return;
  const text = (await page.locator(".toast").textContent()) ?? "";
  throw new Error(`${step}: the server refused it — ${text}`);
}

async function openAs(page: Page, role: keyof typeof tokens) {
  await page.goto(`${base}/?token=${tokens[role]}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
}

/** The actor ids this file creates, kept distinct from table.spec.ts's. */
const FIRST = "act-hand-a";
const SECOND = "act-hand-b";

test("a DM hands a second character over, and the player keeps both across a reconnect",
  async ({ browser }) => {
    // Two CONTEXTS, not two pages: a page shares localStorage with its
    // context, and the whole point is two people holding different invites.
    const dmCtx: BrowserContext = await browser.newContext();
    const playerCtx: BrowserContext = await browser.newContext();
    const dm = await dmCtx.newPage();
    const player = await playerCtx.newPage();

    // --- beat 1: the table, with both people in it ----------------------
    await openAs(dm, "dm");
    await dm.locator('[data-field="session-name"]').fill("Handover Night");
    await dm.locator('[data-action="start-session"]').click();
    await expect(dm.locator(".session")).toContainText("Handover Night", { timeout: 10_000 });
    await dm.getByRole("button", { name: /^Load / }).first().click();
    await expect(dm.locator(".grid")).toBeVisible({ timeout: 10_000 });
    const sceneId = await dm.locator(".grid").getAttribute("data-scene-id");

    await openAs(player, "player");
    // Presence: the DM learns someone joined WITHOUT reloading. This is the
    // frame, not a refresh — nothing here navigates the DM's page.
    await expect(dm.locator(".participant")).toContainText(["Lera"], { timeout: 10_000 });
    await dm.screenshot(shot("h1-dm-sees-the-player-arrive"));

    // --- beat 2: characters exist, and NOBODY holds them -----------------
    //
    // This is how a campaign actually starts (spec §3.1): you log in with no
    // character and the DM assigns one or more afterwards. The creation form's
    // controller field is left EMPTY on purpose — the committed adventure's
    // own actors carry no controller either, they are DM-run until somebody
    // hands them over.
    //
    // It also puts the sharpest case on camera. Granting onto an EMPTY set is
    // the one shape where controller_id actually MOVES: everywhere else the
    // mirror is controller_ids[0] and a grant appends, so it does not change.
    // That is precisely the transition the Go/TS fold divergence shipped on.
    for (const [id, name] of [[FIRST, "Ash"], [SECOND, "Bram"]] as const) {
      await dm.locator('[data-field="actor-id"]').fill(id);
      await dm.locator('[data-field="actor-name"]').fill(name);
      await dm.locator('[data-action="add-actor"]').click();
      await dm.waitForTimeout(300);
      await expectNoRefusal(dm, `add actor ${id}`);

      await dm.locator('[data-field="token-id"]').fill(`tok-${id}`);
      await dm.locator('[data-field="token-scene"]').fill(sceneId ?? "");
      await dm.locator('[data-field="token-actor"]').fill(id);
      await dm.locator('[data-action="place-token"]').click();
      await dm.waitForTimeout(300);
      await expectNoRefusal(dm, `place token for ${id}`);
    }
    await expect(dm.locator(`[data-token-id="tok-${SECOND}"]`)).toBeVisible({ timeout: 10_000 });

    // The player holds NOTHING yet, and is told so rather than shown an empty
    // list. Asserted before any grant, so every change after it means
    // something.
    const panel = player.locator(".player");
    await expect(panel.locator(".empty")).toHaveText("You do not control an actor yet.", { timeout: 10_000 });
    await player.screenshot(shot("h2-player-holds-nothing-yet"));

    // --- beat 3: THE DM ASSIGNS, through their own console ---------------
    const give = async (actorId: string, label: string) => {
      const row = dm.locator(`.control-actor[data-actor="${actorId}"]`);
      await expect(row).toBeVisible({ timeout: 10_000 });
      await row.locator(".grant-target").selectOption(ids.player);
      await row.locator(".grant").click();
      await dm.waitForTimeout(400);
      await expectNoRefusal(dm, `grant ${actorId}`);
      // The DM's own panel names the holder — by DISPLAY NAME, not a uuid.
      await expect(row.locator(".held-who")).toContainText("Lera", { timeout: 10_000 });
      await dm.screenshot(shot(label));
    };

    // The FIRST assignment: an empty set gains its first controller, so
    // controller_id moves from "" to Lera. Asserted on the player's side with
    // no reload — the grant crossed the wire and both folds applied it.
    await give(FIRST, "h3-dm-assigns-the-first-character");
    await expect(panel.getByRole("button", { name: "Ash" })).toBeVisible({ timeout: 10_000 });
    await player.screenshot(shot("h4-player-gains-a-character"));

    // The SECOND: one person holding two characters through one connection,
    // which is what spec §3.1 means by controlling many actors at once.
    await give(SECOND, "h5-dm-assigns-a-second-character");
    await expect(panel.getByRole("button", { name: "Bram" })).toBeVisible({ timeout: 10_000 });
    await expect(panel.getByRole("button", { name: "Ash" })).toBeVisible();
    await player.screenshot(shot("h6-player-with-both-characters"));

    // --- beat 4: the player leaves, and the table is told ----------------
    await playerCtx.close();
    await expect(dm.locator(".participant")).not.toContainText(["Lera"], { timeout: 20_000 });
    await dm.screenshot(shot("h7-dm-sees-the-player-leave"));

    // --- beat 5: they come back, and still hold both ---------------------
    const backCtx = await browser.newContext();
    const back = await backCtx.newPage();
    await openAs(back, "player");
    // Control is CAMPAIGN-scoped and presence is connection-scoped (spec
    // §3.1): losing the connection must not lose the characters.
    const backPanel = back.locator(".player");
    await expect(backPanel.getByRole("button", { name: "Ash" })).toBeVisible({ timeout: 15_000 });
    await expect(backPanel.getByRole("button", { name: "Bram" })).toBeVisible();
    await expect(dm.locator(".participant")).toContainText(["Lera"], { timeout: 10_000 });
    await back.screenshot(shot("h8-player-back-with-both"));

    await backCtx.close();
    await dmCtx.close();
  });

test("a closed connection offers the player a way back in", async ({ page, context }) => {
  // Manual reconnect (spec §3.4). No timer and no backoff: the server cannot
  // know when someone's network returned, so the client offers the action and
  // the person decides. Shown separately because killing the socket without
  // killing the page is the only way to photograph the "closed" state.
  await openAs(page, "player");
  await context.setOffline(true);
  await expect(page.locator(".reconnect")).toBeVisible({ timeout: 20_000 });
  await page.screenshot(shot("h9-connection-closed-offering-reconnect"));

  await context.setOffline(false);
  await page.locator(".reconnect").click();
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 20_000 });
  await page.screenshot(shot("h10-reconnected"));
});
