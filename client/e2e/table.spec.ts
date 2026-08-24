import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";

// End-to-end against the SHIPPED binary: embedded client, wire, HTTP
// metadata and authz together. Every seam below has unit tests; what only a
// browser proves is that the assembled product opens and a human can use it.
//
// The screenshots are the point as much as the assertions — spec §6 makes
// them the remote demo medium, because reviewing from a phone means "what
// does each role see" has to be an artifact rather than an instruction.

const { base, tokens, ids } = fixture("table");

// Leave the table as this file found it: it opens a session, and a rerun of
// this same file would otherwise find "End session" where it expects "Start".
// See joining.spec.ts for the measurement.
test.afterAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${base}/?token=${tokens.dm}`);
  await page.locator(".dm").waitFor({ timeout: 15_000 });
  const end = page.locator('[data-action="end-session"]');
  if ((await end.count()) > 0) await end.click();
  await ctx.close();
});
const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png`, fullPage: true });

/**
 * Fail with the refusal TEXT rather than "expected 0, got 1". A count-only
 * assertion tells you something was refused but not what, which is the least
 * useful moment to be vague.
 */
async function expectNoRefusal(page: Page, step: string) {
  if ((await page.locator(".toast").count()) === 0) return;
  const text = (await page.locator(".toast").textContent()) ?? "";
  throw new Error(`${step}: the server refused it — ${text}`);
}

async function openAs(page: Page, role: keyof typeof tokens) {
  await page.goto(`${base}/?token=${tokens[role]}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
}

test("the token in the URL is stored and then removed from the address bar", async ({ page }) => {
  // A bearer credential left in the URL lands in history and in the Referer
  // of every outbound link.
  await openAs(page, "spectator");
  expect(page.url()).not.toContain("token=");
  expect(await page.evaluate(() => localStorage.getItem("vtt.token"))).toBe(tokens.spectator);
});

test("a spectator sees the table and no controls", async ({ page }) => {
  await openAs(page, "spectator");
  await expect(page.locator(".feed")).toBeVisible();
  await expect(page.locator(".ticker")).toBeVisible();
  await expect(page.locator(".notes")).toBeVisible();
  // Role gating, from the outside: a spectator gets neither panel.
  await expect(page.locator(".player")).toHaveCount(0);
  await expect(page.locator(".dm")).toHaveCount(0);
  await page.screenshot(shot("01-spectator"));
});

test("a DM starts a session, creates a scene and loads an adventure", async ({ page }) => {
  await openAs(page, "dm");
  await expect(page.locator(".dm")).toBeVisible();
  await page.screenshot(shot("02-dm-console"));

  await page.locator('[data-field="session-name"]').fill("Demo Night");
  await page.locator('[data-action="start-session"]').click();
  await expect(page.locator(".session")).toContainText("Demo Night", { timeout: 10_000 });

  // Loading the adventure populates the table: scenes, actors, tokens, notes.
  await page.getByRole("button", { name: /^Load / }).first().click();
  await expect(page.locator(".token")).not.toHaveCount(0, { timeout: 10_000 });
  await expect(page.locator(".grid")).toBeVisible();
  await page.screenshot(shot("03-dm-table-populated"));

  // Hand an actor to the player. The adventure's own actors carry no
  // controller_id — they are DM-run until assigned — and the gateway
  // authorizes moves by that field, so this step is what makes the player
  // able to act at all. It also exercises the console's add-actor, grant and
  // place-token controls, which is why it is done through the UI.
  //
  // TWO STEPS, and that is the point rather than an inconvenience (visibility
  // spec §5.1, Patrik's ruling 2026-08-24). Creating the actor makes a
  // character; the grant gives it a controller AND says what it is. The
  // add-actor form used to carry a controller box that did both at once and
  // could not state a kind, which made Lera a party member by accident of the
  // migration rule rather than by anybody's decision.
  const sceneId = await page.locator(".grid").getAttribute("data-scene-id");
  await page.locator('[data-field="actor-id"]').fill("act-lera");
  await page.locator('[data-field="actor-name"]').fill("Lera");
  await page.locator('[data-action="add-actor"]').click();
  await page.waitForTimeout(400);
  // Checked here so a refusal is reported at the action that caused it,
  // rather than surfacing two steps later as a missing token.
  await expectNoRefusal(page, "add actor");

  const grantRow = page.locator('.control-actor[data-actor="act-lera"]');
  await expect(grantRow).toBeVisible({ timeout: 10_000 });
  await grantRow.locator(".grant-target").selectOption(ids.player);
  await grantRow.locator(".grant-kind").selectOption("ACTOR_KIND_PARTY_MEMBER");
  await grantRow.locator("button.grant").click();
  await page.waitForTimeout(400);
  await expectNoRefusal(page, "grant control");

  await page.locator('[data-field="token-id"]').fill("tok-lera");
  await page.locator('[data-field="token-scene"]').fill(sceneId ?? "");
  await page.locator('[data-field="token-actor"]').fill("act-lera");
  await page.locator('[data-action="place-token"]').click();
  await page.waitForTimeout(400);
  await expectNoRefusal(page, "place token");

  await expect(page.locator('[data-token-id="tok-lera"]')).toBeVisible({ timeout: 10_000 });
  await page.screenshot(shot("03b-dm-assigned-actor"));
});

test("the story feed and ticker record what happened", async ({ page }) => {
  await openAs(page, "spectator");
  // The adventure loaded in the previous test is in the log this client
  // replays from sequence 0, which is the replay path working.
  await expect(page.locator(".tick")).not.toHaveCount(0, { timeout: 10_000 });
  await expect(page.locator(".seq").first()).toContainText("#");
  await page.screenshot(shot("04-spectator-populated"));
});

test("a player sees their own panel and can move their token", async ({ page }) => {
  await openAs(page, "player");
  await expect(page.locator(".player")).toBeVisible();
  // A player gets no DM console — role gating in the shipped client.
  await expect(page.locator(".dm")).toHaveCount(0);
  await page.screenshot(shot("05-player-panel"));

  const mine = page.locator('[data-token-id="tok-lera"]');
  await expect(mine).toBeVisible({ timeout: 10_000 });
  const before = await mine.boundingBox();

  // Click a cell well away from where anything currently is.
  const box = (await page.locator(".grid").boundingBox())!;
  await page.mouse.click(box.x + 6 * 44 + 22, box.y + 5 * 44 + 22);

  await expect
    .poll(async () => (await page.locator('[data-token-id="tok-lera"]').boundingBox())?.x, {
      timeout: 10_000,
    })
    .not.toBe(before?.x);
  await page.screenshot(shot("06-player-moved"));
});

test("what the DM is typing survives events arriving", async ({ page }) => {
  // The console re-renders on every event, and at a live table events arrive
  // constantly. Before this was fixed each rebuild replaced the inputs with
  // empty ones, so a DM lost whatever they were part-way through typing — in
  // practice making the longer forms impossible to finish, because a note
  // takes longer to type than the gap between two events.
  //
  // Only an e2e can catch this: it needs a real event stream running
  // alongside a real form.
  await openAs(page, "dm");
  const note = page.locator('[data-field="note-text"]');
  await note.fill("half-written thought");

  // Provoke an event from another client while the text sits unsent.
  await page.evaluate(async (t) => {
    const ws = new WebSocket(`${location.origin.replace("http", "ws")}/ws?token=${t}&after=0`);
    await new Promise((r) => ws.addEventListener("open", r));
    ws.send(JSON.stringify({ requestId: "e2e-poke", addNarration: { text: "an event arrives" } }));
  }, tokens.dm);

  await expect(page.locator(".feed")).toContainText("an event arrives", { timeout: 10_000 });
  await expect(note).toHaveValue("half-written thought");
  await page.screenshot(shot("07-dm-draft-survives"));
});

test("a player is refused the DM's adventure guide", async ({ page }) => {
  // Guides carry DM secrets. This is the role table from T3, asserted from
  // the browser rather than from a Go test.
  await openAs(page, "player");
  const status = await page.evaluate(async (t) => {
    const r = await fetch("/api/adventures/goblin-ambush/guide", {
      headers: { Authorization: `Bearer ${t}` },
    });
    return r.status;
  }, tokens.player);
  expect(status).toBe(403);
});

test("a spectator's command is refused by the server", async ({ page }) => {
  // The client hides the controls; the SERVER is what actually refuses. This
  // proves the gating is not merely cosmetic.
  await openAs(page, "spectator");
  const status = await page.evaluate(async (t) => {
    const r = await fetch("/api/me", { headers: { Authorization: `Bearer ${t}` } });
    return (await r.json()).role;
  }, tokens.spectator);
  expect(status).toBe("spectator");
});
