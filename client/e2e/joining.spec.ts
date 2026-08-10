import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";

// The demo gate for joining a table (joining-a-table spec, plan J6), against
// the SHIPPED binary with two real browser contexts.
//
// THIS FILE IS THE POINT OF TASK J6. Every layer of this feature was complete
// and gated before it existed — identity, the /join endpoint, promotion, live
// authorization, the client's join view — and `identity.SetJoinOpen` had NO
// CALLER anywhere outside its own tests. The door ships closed, so the shared
// link refused everybody, forever, and no human could reach the feature at
// all. Five finished tasks, every gate green, and nobody could join a table.
//
// The presence branch shipped ONE layer dead and two reviews found it eight
// commits apart. This would have shipped the whole feature dead. So the rule
// this file enforces: a task is not done when its layer works, it is done when
// something that already existed can REACH it.
//
// The screenshots are the deliverable as much as the assertions — Patrik
// reviews from a phone, so "what did each person see" has to be an artifact.

const { base, tokens } = fixture("joining");

// Leave the table as this file found it.
//
// Per-FILE isolation does not cover a second run of the SAME file: the
// campaign is append-only and survives, so a rerun finds the session this file
// opened and waits 30s for a Start button that has been replaced by End.
// Proven by review with --repeat-each=6: 1 passed, 5 failed. That also means
// retries could never be turned on without converting a flake into a permanent
// misdiagnosed failure, and --repeat-each is exactly how a new e2e gets
// checked for flakiness.
test.afterAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${base}/?token=${tokens.dm}`);
  await page.locator(".dm").waitFor({ timeout: 15_000 });
  const end = page.locator('[data-action="end-session"]');
  if ((await end.count()) > 0) await end.click();
  const shut = page.locator('[data-action="close-door"]');
  if ((await shut.count()) > 0) await shut.click();
  await ctx.close();
});
const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png`, fullPage: true });

async function openAs(page: Page, role: keyof typeof tokens) {
  await page.goto(`${base}/?token=${tokens[role]}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
}

test("a stranger with one link joins, watches, is promoted, and plays", async ({ browser }) => {
  const dmCtx = await browser.newContext();
  const guestCtx = await browser.newContext();
  const dm = await dmCtx.newPage();
  const guest = await guestCtx.newPage();

  // --- beat 1: the DM opens the door ------------------------------------
  await openAs(dm, "dm");
  await dm.locator('[data-field="session-name"]').fill("Open Table");
  await dm.locator('[data-action="start-session"]').click();
  await expect(dm.locator(".session")).toContainText("Open Table", { timeout: 10_000 });

  // The adventure puts characters on the board, and its actors deliberately
  // carry NO controller — they are DM-run until somebody is handed one, which
  // is how a campaign actually starts (spec §3.1: you log in holding nothing).
  // Beat 5 needs one to hand over.
  await dm.getByRole("button", { name: /^Load / }).first().click();
  await expect(dm.locator(".grid")).toBeVisible({ timeout: 10_000 });

  // Closed is the state the feature SHIPS in, so it is asserted before
  // anything is done to it — otherwise "open" proves nothing.
  await expect(dm.locator(".door-state")).toContainText("closed", { timeout: 10_000 });
  await dm.screenshot(shot("j1-dm-door-closed"));

  // The link is READABLE BEFORE the door opens, deliberately: the alternative
  // ordering leaves the only way to get the link being to stand the door open
  // while you go and find somebody to send it to.
  const link = await dm.locator('[data-field="join-link"]').inputValue();
  expect(link).toContain("?join=");

  // A stranger with the link is refused while the door is shut. This is the
  // security property the whole design rests on (spec §2), and it is checked
  // in a real browser rather than only against the endpoint.
  await guest.goto(link);
  await guest.locator('[data-field="join-name"]').fill("Robin");
  await guest.locator('[data-action="join"]').click();
  await expect(guest.locator(".error")).toBeVisible({ timeout: 10_000 });
  await guest.screenshot(shot("j2-guest-refused-while-closed"));

  await dm.locator('[data-action="open-door"]').click();
  await expect(dm.locator(".door-state")).toContainText("open", { timeout: 10_000 });
  await dm.screenshot(shot("j3-dm-door-open"));

  // --- beat 2: the stranger joins, by name, with no invite ---------------
  //
  // The same page that was refused a moment ago, retried: the failure did not
  // cost them their typing and did not leave the form dead.
  await guest.locator('[data-action="join"]').click();
  await expect(guest.locator(".conn")).toHaveText("connected", { timeout: 20_000 });
  // The secret is gone from the address bar, checked in a REAL browser where
  // it means something — happy-dom does not move location on replaceState, so
  // the unit test can only assert the call, not the result.
  expect(guest.url()).not.toContain("join=");
  await guest.screenshot(shot("j4-guest-joined-as-spectator"));

  // The DM sees them arrive WITHOUT reloading — presence, over the same
  // connection that was already open.
  await expect(dm.locator(".participant")).toContainText(["Robin"], { timeout: 15_000 });

  // --- beat 3: a spectator can only watch --------------------------------
  //
  // Asserted before the promotion, so every change after it means something.
  // No player panel is the visible half; the roster naming them a spectator is
  // the authoritative half.
  await expect(guest.locator(".player")).toHaveCount(0);
  // THE ROLE CELL, not the row. The row's text also contains the BUTTON —
  // "Make player" — so a containment check on the row matches "player" while
  // Robin is still a spectator, and matches "spectator" after they have been
  // promoted. Both assertions passed in both directions: this file, the one
  // written to catch features that do not work, could not tell promoted from
  // not. Caught by review, after two screenshots named j5-dm-sees-a-spectator
  // and j6-dm-promoted-them came out byte-identical.
  await expect(
    dm.locator('.roster-row:has-text("Robin") .role'),
  ).toHaveText("spectator", { timeout: 10_000 });
  await dm.screenshot(shot("j5-dm-sees-a-spectator"));

  // --- beat 4: the DM promotes them, and it bites with NO reconnect ------
  //
  // The J4 property, end to end in a browser: the guest's socket is the one
  // they joined on and is never touched. Everyone arrives as a spectator, so
  // a reconnect here would sit on the critical path of every person who ever
  // joins — the shared link would be more cumbersome than the invites it
  // replaces.
  await dm.locator('.roster-row:has-text("Robin") button').click();
  await expect(
    dm.locator('.roster-row:has-text("Robin") .role'),
  ).toHaveText("player", { timeout: 15_000 });
  await dm.screenshot(shot("j6-dm-promoted-them"));

  // --- beat 5: they can act ----------------------------------------------
  //
  // The DM hands over a character (spec §3.1: you log in holding nothing), and
  // the promoted spectator's panel appears on the connection they already had.
  const row = dm.locator(".control-actor").first();
  await expect(row).toBeVisible({ timeout: 10_000 });
  await row.locator(".grant-target").selectOption({ label: "Robin" });
  await row.locator(".grant").click();
  await expect(row.locator(".held-who")).toContainText("Robin", { timeout: 15_000 });

  await expect(guest.locator(".player")).toBeVisible({ timeout: 15_000 });
  await guest.screenshot(shot("j7-promoted-guest-holding-a-character"));

  // --- beat 6: rotating locks out the old link and nobody else -----------
  //
  // The property that makes a leak survivable: the guest is already in and
  // stays in, while the link they used stops working.
  dm.once("dialog", (d) => void d.accept());
  await dm.locator('[data-action="rotate-link"]').click();
  await expect
    .poll(async () => await dm.locator('[data-field="join-link"]').inputValue(), { timeout: 15_000 })
    .not.toBe(link);

  const thirdCtx = await browser.newContext();
  const third = await thirdCtx.newPage();
  await third.goto(link); // the OLD link
  await third.locator('[data-field="join-name"]').fill("Sam");
  await third.locator('[data-action="join"]').click();
  await expect(third.locator(".error")).toBeVisible({ timeout: 10_000 });
  await third.screenshot(shot("j8-old-link-refused-after-rotation"));

  // And the guest who was already in is untouched — still connected, still
  // holding their character.
  await expect(guest.locator(".conn")).toHaveText("connected");
  await expect(guest.locator(".player")).toBeVisible();

  await thirdCtx.close();
  await guestCtx.close();
  await dmCtx.close();
});
