// The player panel draws a Say form for a player only (VTT-532).

import "./support/dom";
import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  CommandResultSchema, type ClientCommand, type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Ability, Me } from "../src/metadata";
import { newState, type State } from "../src/state";
import {
  renderPlayerPanel, type PlayerUIState, type Switcher,
} from "../src/view/player";
import { held, ok, refused, settle } from "./support/answers";

type Role = Me["role"];
type Send = (c: ClientCommand) => Promise<CommandResult>;

const STRIKE: Ability = {
  id: "strike", name: "Strike", range: 1, maxTargets: 1,
  usage: { kind: "atWill" },
};

function world(controller: string): State {
  const st = newState();
  for (const [id, name] of [["a-1", "Mira"], ["a-2", "Pell"]] as const) {
    st.Actors[id] = {
      actorId: id, name, moduleId: "", attributes: {}, resources: {},
      controllerId: controller, controllerIds: [controller],
      kind: ActorKind.PARTY_MEMBER,
    };
  }
  st.Scenes["s-1"] = { ID: "s-1", Name: "Hall", GridWidth: 5, GridHeight: 5 };
  st.Tokens["t-1"] = { ID: "t-1", SceneID: "s-1", ActorID: "a-1", X: 1, Y: 1 };
  st.Tokens["t-2"] = { ID: "t-2", SceneID: "s-1", ActorID: "a-2", X: 2, Y: 1 };
  return st;
}

function uiFor(actorId: string): PlayerUIState {
  return { selectedActorId: actorId, selectedAbilityId: "", doorsArmed: false };
}

interface Table {
  st: State;
  me: Me;
  ui: PlayerUIState;
  send: Send;
  repaints: number;
  switcher?: Switcher;
}

function table(role: Role, send: Send, actorId = "a-1"): Table {
  const pid = role + "-1";
  return {
    st: world(pid), me: { participantId: pid, name: "Who", role },
    ui: uiFor(actorId), send, repaints: 0,
  };
}

function paint(t: Table): HTMLElement {
  const rerender = () => { t.repaints++; };
  return t.switcher
    ? renderPlayerPanel(t.st, t.me, [STRIKE], t.ui, t.send, rerender, t.switcher)
    : renderPlayerPanel(t.st, t.me, [STRIKE], t.ui, t.send, rerender);
}

function recorder(answer: () => Promise<CommandResult> = async () => ok()) {
  const sent: ClientCommand[] = [];
  const send: Send = (c) => { sent.push(c); return answer(); };
  return { sent, send };
}

function line(el: HTMLElement): HTMLInputElement {
  const box = el.querySelector<HTMLInputElement>('[data-field="say-text"]');
  if (!box) throw new Error("no say-text");
  return box;
}

function sendButton(el: HTMLElement): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>('[data-action="add-narration"]');
  if (!b) throw new Error("no Send");
  return b;
}

function type(el: HTMLElement, text: string): void {
  const box = line(el);
  box.value = text;
  box.dispatchEvent(new Event("input", { bubbles: true }));
}

function enter(el: HTMLElement): void {
  line(el).dispatchEvent(
    new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
  );
}

function narration(c: ClientCommand | undefined) {
  if (c?.command.case !== "addNarration") throw new Error("not a narration");
  return c.command.value;
}

const SAYING = /\bsay|speak|narrat|\booc\b|\/ooc|console|\bsend\b/i;

function wordsOf(el: Element): string[] {
  const out = [el.textContent ?? ""];
  for (const n of [el, ...Array.from(el.querySelectorAll("*"))]) {
    for (const a of Array.from(n.attributes)) out.push(a.value);
  }
  return out;
}

function expectNoSay(el: HTMLElement): void {
  expect(el.querySelector('[data-field="say-text"]')).toBeNull();
  expect(el.querySelector('[data-field="say-as"]')).toBeNull();
  expect(el.querySelector('[data-action="add-narration"]')).toBeNull();
  expect(el.querySelectorAll("input, textarea").length).toBe(0);
  expect(wordsOf(el).filter((w) => SAYING.test(w))).toEqual([]);
}

function chipNames(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll("button")).map((b) => b.textContent ?? "");
}

describe("a DM's and an agent's panel draw no trace of Say", () => {
  // VTT-532 VTT-540
  test("a DM's panel, controlling an actor, draws no Say", () => {
    const t = table("dm", recorder().send);
    expectNoSay(paint(t));
  });

  // VTT-532 VTT-540
  test("an agent's panel, controlling an actor, draws no Say", () => {
    const t = table("agent", recorder().send);
    expectNoSay(paint(t));
  });

  // VTT-532 VTT-540
  test("a DM's panel handed a switcher draws no Say", () => {
    const t = table("dm", recorder().send);
    t.switcher = {
      actors: [{ actorId: "a-1", name: "Mira" }], onSwitch: () => {},
    };
    expectNoSay(paint(t));
  });

  // VTT-532 VTT-540
  test("an agent's panel handed a switcher draws no Say", () => {
    const t = table("agent", recorder().send);
    t.switcher = {
      actors: [{ actorId: "a-1", name: "Mira" }], onSwitch: () => {},
    };
    expectNoSay(paint(t));
  });

  // VTT-532 VTT-540
  test("a DM's panel draws no Say after a repaint either", () => {
    const t = table("dm", recorder().send);
    paint(t);
    expectNoSay(paint(t));
  });

  // VTT-438
  test("a DM's panel still draws its actors' chips and abilities", () => {
    const t = table("dm", recorder().send);
    const names = chipNames(paint(t));
    expect(names).toContain("Mira");
    expect(names).toContain("Pell");
    expect(names).toContain("Strike");
  });

  // VTT-438
  test("an agent's panel still draws its actors' chips and abilities", () => {
    const t = table("agent", recorder().send);
    const names = chipNames(paint(t));
    expect(names).toContain("Mira");
    expect(names).toContain("Pell");
    expect(names).toContain("Strike");
  });
});

describe("a player's panel draws its Say form", () => {
  // VTT-532
  test("a player's panel draws say-text and a Send", () => {
    const el = paint(table("player", recorder().send));
    expect(el.querySelector('[data-field="say-text"]')).not.toBeNull();
    expect(sendButton(el).textContent).toBe("Send");
  });

  // VTT-500
  test("a player's panel draws no box for a speaker", () => {
    const el = paint(table("player", recorder().send));
    expect(el.querySelector('[data-field="say-as"]')).toBeNull();
    const boxes = Array.from(el.querySelectorAll("input, textarea"));
    expect(boxes.map((b) => b.getAttribute("data-field"))).toEqual(["say-text"]);
  });

  // VTT-528
  test("a player's say-text names /ooc in its placeholder", () => {
    const el = paint(table("player", recorder().send));
    expect(line(el).placeholder).toContain("/ooc");
  });
});

describe("what a player's line sends", () => {
  // VTT-500
  test("Send speaks as the party member the panel acts as", async () => {
    const r = recorder();
    const t = table("player", r.send, "a-2");
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    await settle();
    expect(r.sent.length).toBe(1);
    const n = narration(r.sent[0]);
    expect(n.text).toBe("Hello");
    expect(n.actorId).toBe("a-2");
    expect(n.outOfCharacter).toBe(false);
    expect(n.as).toBe("");
  });

  // VTT-500
  test("Enter in say-text speaks as the party member too", async () => {
    const r = recorder();
    const t = table("player", r.send, "a-1");
    const el = paint(t);
    type(el, "Onward");
    enter(el);
    await settle();
    expect(r.sent.length).toBe(1);
    const n = narration(r.sent[0]);
    expect(n.text).toBe("Onward");
    expect(n.actorId).toBe("a-1");
    expect(n.outOfCharacter).toBe(false);
  });

  // VTT-501
  test("/ooc goes out of character with the rest as its text", async () => {
    const r = recorder();
    const t = table("player", r.send, "a-2");
    const el = paint(t);
    type(el, "/ooc   brb  ");
    sendButton(el).click();
    await settle();
    expect(r.sent.length).toBe(1);
    const n = narration(r.sent[0]);
    expect(n.outOfCharacter).toBe(true);
    expect(n.text).toBe("brb");
    expect(n.as).toBe("");
    expect(n.actorId).toBe("");
  });

  // VTT-501
  test("a first word that is not exactly /ooc stays in character", async () => {
    const r = recorder();
    const t = table("player", r.send, "a-1");
    const el = paint(t);
    type(el, "/oocish hi");
    sendButton(el).click();
    await settle();
    const n = narration(r.sent[0]);
    expect(n.outOfCharacter).toBe(false);
    expect(n.text).toBe("/oocish hi");
    expect(n.actorId).toBe("a-1");
  });

  // VTT-501
  test("/ooc with nothing after it sends nothing", async () => {
    const r = recorder();
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "/ooc   ");
    sendButton(el).click();
    enter(el);
    await settle();
    expect(r.sent.length).toBe(0);
  });

  // VTT-501
  test("a blank line sends nothing", async () => {
    const r = recorder();
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "   ");
    sendButton(el).click();
    enter(el);
    await settle();
    expect(r.sent.length).toBe(0);
  });
});

describe("what a player's answered line leaves typed", () => {
  // VTT-518
  test("a refused line stays in say-text, at the next paint too", async () => {
    const r = recorder(async () => refused("not now"));
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    await settle();
    expect(r.sent.length).toBe(1);
    expect(line(el).value).toBe("Hello");
    expect(line(paint(t)).value).toBe("Hello");
  });

  // VTT-518
  test("a refused line asks for no repaint of its own", async () => {
    const r = recorder(async () => refused("not now"));
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "Hello");
    enter(el);
    await settle();
    expect(r.sent.length).toBe(1);
    expect(t.repaints).toBe(0);
  });

  // VTT-518
  test("an ok:false with no error keeps the line too", async () => {
    const closed = create(CommandResultSchema, { ok: false });
    const r = recorder(async () => closed);
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    await settle();
    expect(line(el).value).toBe("Hello");
    expect(line(paint(t)).value).toBe("Hello");
    expect(t.repaints).toBe(0);
  });

  // VTT-520
  test("an accepted line empties say-text and asks for a repaint", async () => {
    const r = recorder();
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    await settle();
    expect(r.sent.length).toBe(1);
    expect(line(el).value).toBe("");
    expect(line(paint(t)).value).toBe("");
    expect(t.repaints).toBeGreaterThan(0);
  });

  // VTT-520
  test("an accepted line clears what was typed while it was out", async () => {
    const h = held();
    const sent: ClientCommand[] = [];
    const t = table("player", (c) => { sent.push(c); return h.send(c); });
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    type(el, "Hello again");
    h.release(ok());
    await settle();
    expect(sent.length).toBe(1);
    expect(line(el).value).toBe("");
    expect(line(paint(t)).value).toBe("");
  });

  // VTT-522
  test("a repaint shows what was typed in say-text", () => {
    const t = table("player", recorder().send);
    type(paint(t), "half a thought");
    expect(line(paint(t)).value).toBe("half a thought");
  });

  // VTT-522
  test("a repaint shows only what was typed since an accepted line", async () => {
    const r = recorder();
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "first");
    sendButton(el).click();
    await settle();
    const next = paint(t);
    expect(line(next).value).toBe("");
    type(next, "second");
    expect(line(paint(t)).value).toBe("second");
  });

  // VTT-522
  test("a repaint after a refusal shows what was typed since", async () => {
    const r = recorder(async () => refused("no"));
    const t = table("player", r.send);
    const el = paint(t);
    type(el, "first");
    sendButton(el).click();
    await settle();
    type(paint(t), "first, again");
    expect(line(paint(t)).value).toBe("first, again");
  });
});

describe("one press, one line", () => {
  // VTT-523
  test("while a line is unanswered, Send and Enter send nothing", async () => {
    const h = held();
    const sent: ClientCommand[] = [];
    const t = table("player", (c) => { sent.push(c); return h.send(c); });
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    sendButton(el).click();
    enter(el);
    await settle();
    expect(sent.length).toBe(1);
    h.release(ok());
    await settle();
  });

  // VTT-523
  test("the lock holds across a repaint while unanswered", async () => {
    const h = held();
    const sent: ClientCommand[] = [];
    const t = table("player", (c) => { sent.push(c); return h.send(c); });
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    const again = paint(t);
    type(again, "Hello");
    sendButton(again).click();
    enter(again);
    await settle();
    expect(sent.length).toBe(1);
    h.release(ok());
    await settle();
  });

  // VTT-523
  test("an answer frees Send for the next line", async () => {
    const h = held();
    const sent: ClientCommand[] = [];
    const t = table("player", (c) => { sent.push(c); return h.send(c); });
    const el = paint(t);
    type(el, "one");
    sendButton(el).click();
    h.release(refused("no"));
    await settle();
    const next = paint(t);
    type(next, "two");
    sendButton(next).click();
    await settle();
    expect(sent.length).toBe(2);
    expect(narration(sent[1]).text).toBe("two");
    h.release(ok());
    await settle();
  });

  // VTT-523
  test("a line that sends nothing takes no lock", async () => {
    const h = held();
    const sent: ClientCommand[] = [];
    const t = table("player", (c) => { sent.push(c); return h.send(c); });
    const el = paint(t);
    type(el, "/ooc");
    sendButton(el).click();
    type(el, "real words");
    enter(el);
    await settle();
    expect(sent.length).toBe(1);
    expect(narration(sent[0]).text).toBe("real words");
    h.release(ok());
    await settle();
  });
});

describe("a press that sends draws nothing new before its answer", () => {
  // VTT-524
  test("Send changes nothing drawn until the answer", async () => {
    const h = held();
    const t = table("player", h.send);
    const el = paint(t);
    type(el, "Hello");
    const before = paint(t);
    const drawn = before.outerHTML;
    sendButton(before).click();
    await settle();
    expect(before.outerHTML).toBe(drawn);
    expect(line(before).value).toBe("Hello");
    expect(line(before).disabled).toBe(false);
    expect(sendButton(before).disabled).toBe(false);
    expect(t.repaints).toBe(0);
    const during = paint(t);
    expect(during.outerHTML).toBe(drawn);
    expect(line(during).value).toBe("Hello");
    h.release(ok());
    await settle();
  });

  // VTT-524
  test("Enter changes nothing drawn until the answer", async () => {
    const h = held();
    const t = table("player", h.send);
    const el = paint(t);
    type(el, "Hello");
    const drawn = el.outerHTML;
    enter(el);
    await settle();
    expect(el.outerHTML).toBe(drawn);
    expect(line(el).value).toBe("Hello");
    expect(t.repaints).toBe(0);
    h.release(ok());
    await settle();
  });

  // VTT-524
  test("a press the lock stops asks for no repaint", async () => {
    const h = held();
    const t = table("player", h.send);
    const el = paint(t);
    type(el, "Hello");
    sendButton(el).click();
    const drawn = el.outerHTML;
    sendButton(el).click();
    enter(el);
    await settle();
    expect(el.outerHTML).toBe(drawn);
    expect(t.repaints).toBe(0);
    h.release(ok());
    await settle();
  });
});

describe("a line whose send rejects", () => {
  // VTT-526
  test("a rejected send clears nothing and goes to console.error", async () => {
    const boom = new Error("socket gone");
    const r = recorder(() => Promise.reject(boom));
    const t = table("player", r.send);
    const logged: unknown[][] = [];
    const real = console.error;
    console.error = (...a: unknown[]) => { logged.push(a); };
    try {
      const el = paint(t);
      type(el, "Hello");
      sendButton(el).click();
      await settle();
      expect(r.sent.length).toBe(1);
      expect(line(el).value).toBe("Hello");
      expect(line(paint(t)).value).toBe("Hello");
      expect(t.repaints).toBe(0);
      expect(logged.some((args) => args.includes(boom))).toBe(true);
    } finally {
      console.error = real;
    }
  });

  // VTT-526
  test("a rejected send frees Send and Enter", async () => {
    let first = true;
    const r = recorder(() => {
      if (first) { first = false; return Promise.reject(new Error("gone")); }
      return Promise.resolve(ok());
    });
    const t = table("player", r.send);
    const real = console.error;
    console.error = () => {};
    try {
      const el = paint(t);
      type(el, "Hello");
      enter(el);
      await settle();
      enter(el);
      await settle();
      expect(r.sent.length).toBe(2);
      expect(narration(r.sent[1]).text).toBe("Hello");
    } finally {
      console.error = real;
    }
  });
});
