import { test, expect, type Page } from "@playwright/test";
import { fixture } from "./setup";
import { asDM } from "./as-dm";

// The DM whispers to one character and the character answers; the other
// character's window, a spectator over the first one's shoulder and the DM's
// own window show who heard what. Every actor, line and join name this file
// makes carries the repetition's index, so `--repeat-each` runs against one
// campaign; on a rerun `/w Watcher` names every earlier Tobbe too, and
// Tobbe<i>'s log keeps only its own (VTT-584).

const { base, tokens, ids } = fixture("whispering");
const party = "ACTOR_KIND_PARTY_MEMBER";
const shot = (name: string) => ({ path: `client/e2e/.artifacts/${name}.png`, fullPage: true });

test.afterAll(async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${base}/?token=${tokens.dm}`);
  await asDM(page, tokens.dm, [{ setJoinDoor: { door: "JOIN_DOOR_CLOSED" } }]);
  await ctx.close();
});

async function openAs(page: Page, token: string) {
  await page.goto(`${base}/?token=${token}`);
  await expect(page.locator(".conn")).toHaveText("connected", { timeout: 15_000 });
}

async function say(box: ReturnType<Page["locator"]>, line: string) {
  await box.fill(line);
  await box.press("Enter");
  // Enter sends nothing while a line is unanswered (VTT-535, VTT-523).
  await expect(box).toHaveValue("", { timeout: 10_000 });
}

test("the DM whispers to Tobbe and Tobbe answers, and only the DM and whoever watches through Tobbe read either",
  async ({ browser }) => {
    // VTT-582 VTT-583 VTT-593 VTT-595 VTT-597
    const i = test.info().repeatEachIndex;
    const tobbe = `Tobbe${i}`;
    const lera = `Lera${i}`;
    const arrow = `an arrow from the dark ${i}`;
    const second = `a second voice ${i}`;
    const answerText = `I saw it ${i}`;
    const contexts = await Promise.all([0, 1, 2, 3].map(() => browser.newContext()));
    const [dm, leraPage, tobbePage, eye] = await Promise.all(contexts.map((c) => c.newPage()));

    await openAs(dm!, tokens.dm);
    await asDM(dm!, tokens.dm, [
      { addActor: { actor: { actorId: `w-tobbe-${i}`, name: tobbe, kind: party } } },
      { addActor: { actor: { actorId: `w-lera-${i}`, name: lera, kind: party } } },
      { promoteParticipant: { participantId: ids.spectator, role: "player" } },
      { grantActorControl: { actorId: `w-lera-${i}`, participantId: ids.player, kind: party } },
      { grantActorControl: { actorId: `w-tobbe-${i}`, participantId: ids.spectator, kind: party } },
      { setJoinDoor: { door: "JOIN_DOOR_OPEN" } },
    ]);
    const link = dm!.locator('[data-field="join-link"]');
    await expect(link).toHaveValue(/\?join=/, { timeout: 15_000 });

    const sent: Record<string, unknown>[] = [];
    tobbePage!.on("websocket", (ws) =>
      ws.on("framesent", (f) => sent.push(JSON.parse(String(f.payload)) as Record<string, unknown>)),
    );
    await openAs(leraPage!, tokens.player);
    // Watcher's page opens after the DM's commands, so it boots as the player
    // who plays Tobbe; a rerun's earlier Tobbe comes first by id (VTT-340).
    await openAs(tobbePage!, tokens.spectator);
    await tobbePage!.locator(".player").getByRole("button", { name: tobbe }).click();

    await eye!.goto(await link.inputValue());
    await eye!.locator('[data-field="join-name"]').fill(`Eye${i}`);
    await eye!.locator('[data-action="join"]').click();
    await expect(eye!.locator(".conn")).toHaveText("connected", { timeout: 20_000 });
    await eye!.locator(".perch").getByRole("button", { name: tobbe }).click();
    await expect(eye!.locator(".perched-on")).toHaveText(`Perched on: ${tobbe}`, { timeout: 10_000 });

    const dmSay = dm!.locator('.dm [data-field="say-text"]');
    const whisper = (page: Page, text: string) => page.locator(".feed .whisper").filter({ hasText: text });
    const story = (page: Page) => page.locator(".feed");

    await say(dmSay, `/w ${tobbe} ${arrow}`);
    for (const page of [dm!, tobbePage!, eye!]) {
      await expect(whisper(page, arrow)).toHaveText(`The DM whispers to ${tobbe}: ${arrow}`,
        { timeout: 10_000 });
    }
    await say(dmSay, `the wind turns ${i}`);
    await expect(story(leraPage!)).toContainText(`the wind turns ${i}`, { timeout: 10_000 });
    await expect(story(leraPage!)).not.toContainText(arrow);

    await say(dmSay, `/w Watcher ${second}`);
    for (const page of [tobbePage!, eye!]) {
      await expect(whisper(page, second)).toHaveText(`The DM whispers to ${tobbe}: ${second}`,
        { timeout: 10_000 });
    }
    const everyTobbe = Array.from({ length: i + 1 }, (_, k) => `Tobbe${k}`).join(", ");
    await expect(whisper(dm!, second)).toHaveText(`The DM whispers to ${everyTobbe}: ${second}`, { timeout: 10_000 });

    await say(tobbePage!.locator('.player .say input[data-field="say-text"]'), `/dm ${answerText}`);
    for (const page of [dm!, eye!]) {
      await expect(whisper(page, answerText)).toHaveText(`${tobbe} whispers to the DM: ${answerText}`, { timeout: 10_000 });
    }
    await say(dmSay, `the wind drops ${i}`);
    await expect(story(leraPage!)).toContainText(`the wind drops ${i}`, { timeout: 10_000 });
    await expect(story(leraPage!)).not.toContainText(second);
    await expect(story(leraPage!)).not.toContainText(answerText);

    const answer = sent.filter((c) => JSON.stringify(c).includes(answerText));
    expect(answer.map((c) => Object.keys(c).filter((k) => k !== "requestId"))).toEqual([["addWhisper"]]);
    expect(answer[0]!["addWhisper"]).toEqual({ text: answerText, fromActorId: `w-tobbe-${i}` });

    for (const [name, page] of [["dm", dm], ["lera", leraPage], ["tobbe", tobbePage], ["eye", eye]] as const) {
      await page!.screenshot(shot(`w1-${name}-after-the-answer-${i}`));
    }
    await Promise.all(contexts.map((c) => c.close()));
  });
