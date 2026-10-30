// Independent QA of the DM console's Say: VTT-517, VTT-521, VTT-522,
// VTT-527 to VTT-531 and VTT-533 to VTT-539, with the owner's rulings the
// ticket quotes, and D9 of the plan.

import { afterEach, beforeEach, describe, expect, spyOn, test } from "bun:test";
import "./support/dom";
import { create } from "@bufbuild/protobuf";
import {
  CommandResultSchema, type AddNarration, type ClientCommand,
  type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { renderDMConsole } from "../src/view/dm";
import { newState, type State } from "../src/state";
import type { Participant } from "../src/session";
import { held, ok, refused, settle } from "./support/answers";

type Answer = (c: ClientCommand) => Promise<CommandResult>;

interface Table {
  sent: ClientCommand[];
  answer: Answer;
  repaints: number;
  notices: string[];
  st: State;
  participants: Participant[];
}

function table(st: State = newState()): Table {
  return {
    sent: [],
    answer: () => Promise.resolve(ok()),
    repaints: 0,
    notices: [],
    st,
    participants: [],
  };
}

function aWorldWhereTheDMControlsAnActor(): Table {
  const st = newState();
  st.Actors["hero"] = {
    actorId: "hero", name: "Hero", moduleId: "", attributes: {},
    resources: {}, controllerId: "dm-1", controllerIds: ["dm-1"],
    kind: ActorKind.PARTY_MEMBER,
  };
  const t = table(st);
  t.participants = [{ participantId: "dm-1", displayName: "Dee" }];
  return t;
}

function draw(t: Table): HTMLElement {
  const root = renderDMConsole({
    st: t.st,
    participants: t.participants,
    adventures: [],
    maps: [],
    guideFor: () => Promise.resolve(null),
    joinLink: null,
    roster: null,
    origin: "http://localhost:8080",
    refreshSharing: () => {},
    send: (c) => {
      t.sent.push(c);
      return t.answer(c);
    },
    rerender: () => {
      t.repaints++;
    },
    notify: (m) => {
      t.notices.push(m);
    },
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  });
  document.body.replaceChildren(root);
  return root;
}

function box(root: HTMLElement, field: string): HTMLInputElement {
  const el = root.querySelector<HTMLInputElement>(`[data-field="${field}"]`);
  if (!el) throw new Error("the console draws no box " + field);
  return el;
}

function typeInto(root: HTMLElement, field: string, value: string): void {
  const el = box(root, field);
  el.value = value;
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

function say(root: HTMLElement, as: string, text: string): void {
  typeInto(root, "say-as", as);
  typeInto(root, "say-text", text);
}

function sendButton(root: HTMLElement): HTMLButtonElement {
  const el = root.querySelector<HTMLButtonElement>(
    'button[data-action="add-narration"]');
  if (!el) throw new Error("the console draws no Send");
  return el;
}

function pressSend(root: HTMLElement): void {
  sendButton(root).click();
}

function pressEnter(root: HTMLElement): void {
  box(root, "say-text").dispatchEvent(new KeyboardEvent("keydown", {
    key: "Enter", bubbles: true, cancelable: true,
  }));
}

function lines(t: Table): AddNarration[] {
  return t.sent.flatMap((c) =>
    c.command.case === "addNarration" ? [c.command.value] : []);
}

function values(root: HTMLElement): string[] {
  const els = root.querySelectorAll<HTMLInputElement>("input, textarea, select");
  return Array.from(els, (el) => el.value);
}

const releases: (() => void)[] = [];

function hold(t: Table): ReturnType<typeof held> {
  const h = held();
  t.answer = h.send;
  releases.push(() => h.release());
  return h;
}

async function clearTheConsole(): Promise<void> {
  for (const release of releases.splice(0)) release();
  await settle();
  const root = draw(table());
  for (const f of ["say-as", "say-text", "session-name"]) {
    if (root.querySelector(`[data-field="${f}"]`)) typeInto(root, f, "");
  }
  document.body.replaceChildren();
}

beforeEach(clearTheConsole);
afterEach(clearTheConsole);

describe("the console draws a Say", () => {
  // VTT-527
  test("a DM who controls no actor is drawn say-as, say-text and Send", () => {
    const root = draw(table());
    expect(root.querySelectorAll('[data-field="say-as"]').length).toBe(1);
    expect(root.querySelectorAll('[data-field="say-text"]').length).toBe(1);
    const sends = root.querySelectorAll('button[data-action="add-narration"]');
    expect(sends.length).toBe(1);
    expect(sends[0]?.textContent?.trim()).toBe("Send");
  });

  // VTT-527
  test("a DM who controls an actor is drawn say-as, say-text and Send", () => {
    const root = draw(aWorldWhereTheDMControlsAnActor());
    expect(root.querySelectorAll('[data-field="say-as"]').length).toBe(1);
    expect(root.querySelectorAll('[data-field="say-text"]').length).toBe(1);
    const sends = root.querySelectorAll('button[data-action="add-narration"]');
    expect(sends.length).toBe(1);
  });

  // VTT-527 (the owner's ruling: first, above the session controls)
  test("the Say is first in the console, headed Say something", () => {
    const root = draw(table());
    const all = Array.from(root.querySelectorAll("*"));
    const at = (el: Element): number => all.indexOf(el);
    const heading = all.find((el) =>
      el.textContent?.trim() === "Say something" &&
      Array.from(el.children).every((c) => c.textContent?.trim() === ""));
    expect(heading).toBeDefined();
    const control = "[data-field], [data-action], input, textarea, select, button";
    const say = [box(root, "say-as"), box(root, "say-text"), sendButton(root)];
    const firstControl = all.findIndex((el) => el.matches(control));
    expect(firstControl).toBeGreaterThan(at(heading as Element));
    const lastOfSay = Math.max(...say.map(at));
    const others = all.filter((el) => el.matches(control) && !say.includes(
      el as HTMLInputElement));
    expect(others.length).toBeGreaterThan(0);
    for (const el of others) expect(at(el)).toBeGreaterThan(lastOfSay);
    expect(at(box(root, "session-name"))).toBeGreaterThan(lastOfSay);
  });

  // VTT-528
  test("say-text's placeholder names /ooc", () => {
    const root = draw(table());
    expect(box(root, "say-text").placeholder).toContain("/ooc");
  });
});

describe("what a line sends", () => {
  // VTT-529
  test("Send says Hello in character as Grimble", async () => {
    const t = table();
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    await settle();
    expect(t.sent.length).toBe(1);
    const [line] = lines(t);
    expect(line?.text).toBe("Hello");
    expect(line?.as).toBe("Grimble");
    expect(line?.outOfCharacter).toBe(false);
  });

  // VTT-529
  test("Enter in say-text says Hello in character as Grimble", async () => {
    const t = table();
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressEnter(root);
    await settle();
    expect(t.sent.length).toBe(1);
    const [line] = lines(t);
    expect(line?.text).toBe("Hello");
    expect(line?.as).toBe("Grimble");
    expect(line?.outOfCharacter).toBe(false);
  });

  // VTT-529
  test("a blank say-as sends the line in character with no as", async () => {
    const t = table();
    const root = draw(t);
    say(root, "   ", "Hello");
    pressSend(root);
    await settle();
    const [line] = lines(t);
    expect(lines(t).length).toBe(1);
    expect(line?.text).toBe("Hello");
    expect(line?.as).toBe("");
    expect(line?.outOfCharacter).toBe(false);
  });

  // VTT-529
  test("say-as is sent trimmed", async () => {
    const t = table();
    const root = draw(t);
    say(root, "  Grimble  ", "Hello");
    pressSend(root);
    await settle();
    expect(lines(t)[0]?.as).toBe("Grimble");
  });

  // VTT-529
  test("a first word that only begins with /ooc stays in character", async () => {
    const t = table();
    const root = draw(t);
    say(root, "Grimble", "/oocx hi");
    pressSend(root);
    await settle();
    const [line] = lines(t);
    expect(line?.outOfCharacter).toBe(false);
    expect(line?.text).toBe("/oocx hi");
    expect(line?.as).toBe("Grimble");
  });

  // VTT-530
  test("/ooc brb with a speaker goes out of character as brb", async () => {
    const t = table();
    const root = draw(t);
    say(root, "Grimble", "/ooc brb");
    pressSend(root);
    await settle();
    expect(lines(t).length).toBe(1);
    const [line] = lines(t);
    expect(line?.outOfCharacter).toBe(true);
    expect(line?.text).toBe("brb");
  });

  // VTT-530
  test("/ooc brb with no speaker, by Enter, goes out of character", async () => {
    const t = table();
    const root = draw(t);
    say(root, "", "/ooc brb");
    pressEnter(root);
    await settle();
    expect(lines(t).length).toBe(1);
    const [line] = lines(t);
    expect(line?.outOfCharacter).toBe(true);
    expect(line?.text).toBe("brb");
    expect(line?.as).toBe("");
  });

  // D9 of docs/superpowers/plans/2026-10-30-the-dm-speaks-from-the-console.md
  test("/ooc from the console carries what say-as holds", async () => {
    const t = table();
    const root = draw(t);
    say(root, "Grimble", "/ooc brb");
    pressSend(root);
    await settle();
    expect(lines(t)[0]?.as).toBe("Grimble");
  });

  // VTT-531
  test("a blank line sends nothing and takes no lock", async () => {
    const t = table();
    for (const blank of ["", "   "]) {
      const root = draw(t);
      say(root, "Grimble", blank);
      pressSend(root);
      pressEnter(root);
      await settle();
    }
    expect(t.sent.length).toBe(0);
    const root = draw(t);
    typeInto(root, "say-text", "Hi");
    pressSend(root);
    await settle();
    expect(lines(t).length).toBe(1);
  });

  // VTT-531
  test("/ooc with nothing after it sends nothing", async () => {
    const t = table();
    for (const bare of ["/ooc", "  /ooc   "]) {
      const root = draw(t);
      say(root, "Grimble", bare);
      pressSend(root);
      pressEnter(root);
      await settle();
    }
    expect(t.sent.length).toBe(0);
    const root = draw(t);
    typeInto(root, "say-text", "/ooc hi");
    pressEnter(root);
    await settle();
    expect(lines(t).length).toBe(1);
    expect(lines(t)[0]?.text).toBe("hi");
  });
});

describe("what an answer leaves typed", () => {
  // VTT-517
  test("a refused line keeps both boxes at the next paint", async () => {
    const t = table();
    t.answer = () => Promise.resolve(refused("not now"));
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    await settle();
    expect(lines(t).length).toBe(1);
    const next = draw(t);
    expect(box(next, "say-text").value).toBe("Hello");
    expect(box(next, "say-as").value).toBe("Grimble");
  });

  // VTT-538
  test("a refused line asks for no repaint", async () => {
    const t = table();
    t.answer = () => Promise.resolve(refused("not now"));
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    await settle();
    expect(lines(t).length).toBe(1);
    expect(t.repaints).toBe(0);
  });

  // VTT-517
  test("an ok false with no error, a closed socket, keeps both boxes", async () => {
    const t = table();
    t.answer = () => Promise.resolve(create(CommandResultSchema, { ok: false }));
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressEnter(root);
    await settle();
    expect(lines(t).length).toBe(1);
    const next = draw(t);
    expect(box(next, "say-text").value).toBe("Hello");
    expect(box(next, "say-as").value).toBe("Grimble");
  });

  // VTT-533
  test("an accepted line empties say-text and asks for a repaint", async () => {
    const t = table();
    const blank = box(draw(t), "say-text").value;
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    await settle();
    expect(lines(t).length).toBe(1);
    expect(t.repaints).toBeGreaterThan(0);
    expect(box(draw(t), "say-text").value).toBe(blank);
  });

  // VTT-521
  test("an accepted line keeps say-as for the next line", async () => {
    const t = table();
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    await settle();
    const next = draw(t);
    expect(box(next, "say-as").value).toBe("Grimble");
    typeInto(next, "say-text", "Again");
    pressSend(next);
    await settle();
    expect(lines(t).map((l) => l.as)).toEqual(["Grimble", "Grimble"]);
  });

  // VTT-533 (the owner's ruling "Töm ändå")
  test("text typed while waiting is emptied when the line is accepted", async () => {
    const t = table();
    const blank = box(draw(t), "say-text").value;
    const h = hold(t);
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    typeInto(root, "say-text", "Hello there");
    expect(box(draw(t), "say-text").value).toBe("Hello there");
    h.release(ok());
    await settle();
    const next = draw(t);
    expect(box(next, "say-text").value).toBe(blank);
    expect(box(next, "say-as").value).toBe("Grimble");
    expect(lines(t).map((l) => l.text)).toEqual(["Hello"]);
  });

  // VTT-522
  test("say-as changed while waiting shows the change after accept", async () => {
    const t = table();
    const h = hold(t);
    const root = draw(t);
    say(root, "Grimble", "Hello");
    pressSend(root);
    typeInto(root, "say-as", "Bob");
    h.release(ok());
    await settle();
    expect(box(draw(t), "say-as").value).toBe("Bob");
  });

  // VTT-534
  test("say-text shows what was typed across repaints and a refusal", async () => {
    const t = table();
    t.answer = () => Promise.resolve(refused("not now"));
    typeInto(draw(t), "say-text", "Hel");
    draw(t);
    expect(box(draw(t), "say-text").value).toBe("Hel");
    const root = draw(t);
    pressSend(root);
    await settle();
    typeInto(draw(t), "say-text", "Hello");
    expect(box(draw(t), "say-text").value).toBe("Hello");
    expect(lines(t).map((l) => l.text)).toEqual(["Hel"]);
  });
});

describe("one press, one line", () => {
  // VTT-535
  test("Send twice while unanswered sends one line", async () => {
    const t = table();
    const h = hold(t);
    const root = draw(t);
    say(root, "", "Hello");
    pressSend(root);
    pressSend(root);
    const again = draw(t);
    pressSend(again);
    pressEnter(again);
    await settle();
    expect(lines(t).length).toBe(1);
    h.release(refused("not now"));
    await settle();
    pressSend(draw(t));
    await settle();
    expect(lines(t).length).toBe(2);
    h.release(ok());
    await settle();
  });

  // VTT-535
  test("Send then Enter while unanswered sends one line", async () => {
    const t = table();
    const h = hold(t);
    const root = draw(t);
    say(root, "", "Hello");
    pressSend(root);
    pressEnter(root);
    await settle();
    expect(lines(t).length).toBe(1);
    h.release(ok());
    await settle();
    const next = draw(t);
    typeInto(next, "say-text", "Next");
    pressEnter(next);
    await settle();
    expect(lines(t).length).toBe(2);
    h.release(ok());
    await settle();
  });

  // VTT-535
  test("Enter twice while unanswered sends one line", async () => {
    const t = table();
    const h = hold(t);
    const root = draw(t);
    say(root, "", "Hello");
    pressEnter(root);
    pressEnter(root);
    await settle();
    expect(lines(t).length).toBe(1);
    h.release(ok());
    await settle();
    const next = draw(t);
    typeInto(next, "say-text", "Next");
    pressSend(next);
    await settle();
    expect(lines(t).length).toBe(2);
    h.release(ok());
    await settle();
  });

  // VTT-535 (the owner's ruling "Nej, osynligt lås")
  test("a press the lock stops shows no notice and asks no repaint", async () => {
    const t = table();
    const h = hold(t);
    const root = draw(t);
    say(root, "", "Hello");
    pressSend(root);
    await settle();
    pressSend(root);
    pressEnter(root);
    await settle();
    expect(lines(t).length).toBe(1);
    expect(t.notices).toEqual([]);
    expect(t.repaints).toBe(0);
    h.release(ok());
    await settle();
  });

  // VTT-539
  test("another console control sends while the Say is unanswered", async () => {
    const t = table();
    const h = held();
    releases.push(() => h.release());
    t.answer = (c) =>
      c.command.case === "addNarration" ? h.send(c) : Promise.resolve(ok());
    const root = draw(t);
    say(root, "", "Hello");
    pressSend(root);
    const next = draw(t);
    typeInto(next, "session-name", "Night one");
    const start = next.querySelector<HTMLButtonElement>(
      'button[data-action="start-session"]');
    start?.click();
    await settle();
    const starts = t.sent.filter((c) => c.command.case === "startSession");
    expect(starts.length).toBe(1);
    expect(lines(t).length).toBe(1);
    h.release(ok());
    await settle();
  });
});

describe("a press that sends draws nothing new before its answer", () => {
  // VTT-536
  test("Send changes nothing the console draws while unanswered", async () => {
    const t = table();
    const h = hold(t);
    say(draw(t), "Grimble", "Hello");
    const control = draw(t).outerHTML;
    const before = draw(t);
    const markup = before.outerHTML;
    const shown = values(before);
    expect(markup).toBe(control);
    pressSend(before);
    await settle();
    expect(lines(t).length).toBe(1);
    expect(values(before)).toEqual(shown);
    expect(sendButton(before).disabled).toBe(false);
    expect(box(before, "say-text").disabled).toBe(false);
    expect(box(before, "say-as").disabled).toBe(false);
    expect(t.repaints).toBe(0);
    const after = draw(t);
    expect(after.outerHTML).toBe(markup);
    expect(values(after)).toEqual(shown);
    h.release(ok());
    await settle();
  });

  // VTT-536
  test("Enter changes nothing the console draws while unanswered", async () => {
    const t = table();
    const h = hold(t);
    say(draw(t), "Grimble", "Hello");
    const before = draw(t);
    const markup = before.outerHTML;
    const shown = values(before);
    pressEnter(before);
    await settle();
    expect(lines(t).length).toBe(1);
    expect(values(before)).toEqual(shown);
    expect(t.repaints).toBe(0);
    const after = draw(t);
    expect(after.outerHTML).toBe(markup);
    expect(values(after)).toEqual(shown);
    h.release(ok());
    await settle();
  });
});

describe("a send whose promise rejects", () => {
  // VTT-537
  test("clears nothing, asks no repaint, frees Send, is logged", async () => {
    const spy = spyOn(console, "error").mockImplementation(() => {});
    try {
      const t = table();
      const boom = new Error("socket gone");
      t.answer = () => Promise.reject(boom);
      const root = draw(t);
      say(root, "Grimble", "Hello");
      pressSend(root);
      await settle();
      expect(lines(t).length).toBe(1);
      expect(spy).toHaveBeenCalled();
      expect(spy.mock.calls.some((args) => args.includes(boom))).toBe(true);
      expect(t.repaints).toBe(0);
      const next = draw(t);
      expect(box(next, "say-text").value).toBe("Hello");
      expect(box(next, "say-as").value).toBe("Grimble");
      t.answer = () => Promise.resolve(ok());
      pressSend(next);
      await settle();
      expect(lines(t).length).toBe(2);
    } finally {
      spy.mockRestore();
    }
  });

  // VTT-537
  test("frees Enter in say-text", async () => {
    const spy = spyOn(console, "error").mockImplementation(() => {});
    try {
      const t = table();
      t.answer = () => Promise.reject(new Error("socket gone"));
      const root = draw(t);
      say(root, "", "Hello");
      pressEnter(root);
      await settle();
      expect(spy).toHaveBeenCalled();
      t.answer = () => Promise.resolve(ok());
      pressEnter(draw(t));
      await settle();
      expect(lines(t).length).toBe(2);
    } finally {
      spy.mockRestore();
    }
  });
});
