import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";
import { asDM } from "./as-dm";

// The DM boots a participant from the console, and the removed participant's
// own window says so, and the share link admits them again. Every name and
// actor carries the repetition's index, so `--repeat-each` runs against one
// campaign.

const { base, tokens } = fixture("booting");
const party = "ACTOR_KIND_PARTY_MEMBER";
const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png`, fullPage: true });

test.afterAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${base}/?token=${tokens.dm}`);
  await asDM(page, tokens.dm, [{ setJoinDoor: { door: "JOIN_DOOR_CLOSED" } }]);
  await ctx.close();
});

async function openDM(page: Page) {
  await page.goto(`${base}/?token=${tokens.dm}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
  await asDM(page, tokens.dm, [{ setJoinDoor: { door: "JOIN_DOOR_OPEN" } }]);
  const link = page.locator('[data-field="join-link"]');
  await expect(link).toHaveValue(/\?join=/, { timeout: 15_000 });
  return await link.inputValue();
}

async function join(page: Page, link: string, name: string) {
  await page.goto(link);
  await page.locator('[data-field="join-name"]').fill(name);
  await page.locator('[data-action="join"]').click();
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 20_000 });
}

async function participantId(dm: Page, name: string): Promise<string> {
  const id = await dm.evaluate(async ([t, n]) => {
    const r = await fetch("/api/participants", { headers: { Authorization: `Bearer ${t}` } });
    const list = (await r.json()) as { participantId: string; name: string }[];
    return list.find((p) => p.name === n)?.participantId ?? "";
  }, [tokens.dm, name] as const);
  expect(id, `no participant named ${name}`).not.toBe("");
  return id;
}

async function boot(dm: Page, id: string, name: string) {
  let asked = "";
  dm.once("dialog", (d) => {
    asked = d.message();
    void d.accept();
  });
  const row = dm.locator(`.roster-row[data-participant="${id}"]`);
  await expect(row.locator(".who")).toHaveText(name, { timeout: 15_000 });
  await row.locator(`[data-action="boot-${id}"]`).click();
  await expect(row).toHaveCount(0, { timeout: 15_000 });
  expect(asked).toContain(name);
}

test("the DM boots a player, who leaves the table and their characters, and can join again by the link", async ({ browser }) => {
  // VTT-570 VTT-572 VTT-577 VTT-544
  const i = test.info().repeatEachIndex;
  const kit = `Kit${i}`;
  const hero = `boot-hero-${i}`;
  const dmCtx = await browser.newContext();
  const guestCtx = await browser.newContext();
  const dm = await dmCtx.newPage();
  const guest = await guestCtx.newPage();

  const link = await openDM(dm);
  await asDM(dm, tokens.dm, [{ addActor: { actor: { actorId: hero, name: `Hero ${i}`, kind: party } } }]);
  await join(guest, link, kit);
  const id = await participantId(dm, kit);
  await asDM(dm, tokens.dm, [
    { promoteParticipant: { participantId: id, role: "player" } },
    { grantActorControl: { actorId: hero, participantId: id, kind: party } },
  ]);
  const held = dm.locator(`.control-actor[data-actor="${hero}"] .held-who`);
  await expect(held).toHaveText(kit, { timeout: 15_000 });
  await dm.screenshot(shot(`b1-console-before-boot-${i}`));

  await boot(dm, id, kit);
  await expect(guest.locator(".removed")).toContainText("removed", { timeout: 15_000 });
  await expect(guest.locator(".reconnect")).toHaveCount(0);
  await guest.screenshot(shot(`b2-guest-removed-${i}`));
  await expect(held).toHaveCount(0, { timeout: 15_000 });
  await dm.screenshot(shot(`b3-console-after-boot-${i}`));

  await guest.goto(link);
  await expect(guest.locator('[data-field="join-name"]')).toBeVisible({ timeout: 15_000 });
  await join(guest, link, kit.toLowerCase());
  await guest.screenshot(shot(`b4-guest-rejoined-${i}`));

  await guestCtx.close();
  await dmCtx.close();
});

test("a player removed while away is told so when they come back, and can join again by the link", async ({ browser }) => {
  // VTT-573 VTT-577
  const i = test.info().repeatEachIndex;
  const jo = `Jo${i}`;
  const dmCtx = await browser.newContext();
  const guestCtx = await browser.newContext();
  const dm = await dmCtx.newPage();

  const link = await openDM(dm);
  const away = await guestCtx.newPage();
  await join(away, link, jo);
  const id = await participantId(dm, jo);
  await asDM(dm, tokens.dm, [{ promoteParticipant: { participantId: id, role: "player" } }]);
  const present = dm.locator(".participant", { hasText: jo });
  await expect(present).toHaveCount(1, { timeout: 15_000 });
  await away.close();
  await expect(present).toHaveCount(0, { timeout: 15_000 });
  await boot(dm, id, jo);

  const back = await guestCtx.newPage();
  await back.goto(base);
  await expect(back.locator(".removed")).toContainText("removed", { timeout: 15_000 });
  await expect(back.locator(".conn")).toHaveText("closed", { timeout: 15_000 });
  await expect(back.locator(".reconnect")).toHaveCount(0);
  await back.screenshot(shot(`b5-away-guest-told-${i}`));
  await join(back, link, jo.toLowerCase());

  await guestCtx.close();
  await dmCtx.close();
});
