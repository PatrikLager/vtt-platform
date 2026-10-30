// Independent QA of the player panel's Say form, VTT-518 and VTT-520 to VTT-525.

import { expect, test } from "bun:test";
import "./support/dom";
import type {
  ClientCommand, CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Me } from "../src/metadata";
import { newState } from "../src/state";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { held, ok, refused, settle } from "./support/answers";

type Send = (c: ClientCommand) => Promise<CommandResult>;

interface Table {
  ui: PlayerUIState;
  sent: ClientCommand[];
  repaints: number;
  view: HTMLElement;
  paint(): HTMLElement;
}

function table(role: Me["role"], answer: Send): Table {
  const me: Me = { participantId: "p1", name: "Ann", role };
  const st = newState();
  st.Actors["a1"] = {
    actorId: "a1", name: "Brakka", moduleId: "", attributes: {},
    resources: {}, controllerId: "p1", controllerIds: ["p1"],
    kind: ActorKind.PARTY_MEMBER,
  };
  const send: Send = (c) => {
    t.sent.push(c);
    return answer(c);
  };
  const t: Table = {
    ui: { selectedActorId: "a1", selectedAbilityId: "", doorsArmed: false },
    sent: [],
    repaints: 0,
    view: document.createElement("div"),
    paint() {
      t.view = renderPlayerPanel(st, me, [], t.ui, send, () => {
        t.repaints++;
        t.paint();
      });
      return t.view;
    },
  };
  t.paint();
  return t;
}

const answering = (r: CommandResult): Send => () => Promise.resolve(r);

function field(root: HTMLElement, name: string): HTMLInputElement {
  const n = root.querySelector<HTMLInputElement>(`[data-field="${name}"]`);
  if (n === null) throw new Error("no box " + name);
  return n;
}

function type(root: HTMLElement, name: string, text: string): void {
  const n = field(root, name);
  n.value = text;
  n.dispatchEvent(new Event("input", { bubbles: true }));
}

function pressSend(root: HTMLElement): void {
  const b = root.querySelector<HTMLButtonElement>(
    '[data-action="add-narration"]',
  );
  if (b === null) throw new Error("no Send");
  b.click();
}

function pressEnter(root: HTMLElement): void {
  field(root, "say-text").dispatchEvent(
    new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
  );
}

function line(c: ClientCommand | undefined) {
  if (c === undefined || c.command.case !== "addNarration") {
    throw new Error("not a line");
  }
  return c.command.value;
}

function drawn(root: HTMLElement): string[] {
  const boxes = Array.from(root.querySelectorAll("input, textarea, select"));
  return [
    root.outerHTML,
    ...boxes.map((b) => (b as HTMLInputElement).value),
  ];
}

// VTT-518, VTT-500
test("a player's refused line is in say-text at the next paint", async () => {
  const t = table("player", answering(refused("not now")));
  expect(t.view.querySelector('[data-field="say-as"]')).toBeNull();
  type(t.view, "say-text", "I draw my sword");
  pressSend(t.view);
  await settle();
  expect(t.sent).toHaveLength(1);
  expect(line(t.sent[0]).text).toBe("I draw my sword");
  expect(line(t.sent[0]).actorId).toBe("a1");
  expect(field(t.paint(), "say-text").value).toBe("I draw my sword");
});

// VTT-518, VTT-501
test("a player's refused /ooc line is in say-text at the next paint", async () => {
  const t = table("player", answering(refused("not now")));
  type(t.view, "say-text", "/ooc where is the pizza");
  pressSend(t.view);
  await settle();
  expect(t.sent).toHaveLength(1);
  expect(line(t.sent[0]).outOfCharacter).toBe(true);
  expect(line(t.sent[0]).text).toBe("where is the pizza");
  expect(field(t.paint(), "say-text").value).toBe("/ooc where is the pizza");
});

// VTT-520
test("a player's accepted line leaves say-text empty", async () => {
  const t = table("player", answering(ok()));
  type(t.view, "say-text", "I draw my sword");
  pressSend(t.view);
  await settle();
  expect(t.sent).toHaveLength(1);
  expect(field(t.paint(), "say-text").value).toBe("");
});

// VTT-520, VTT-501
test("a player's accepted /ooc line leaves say-text empty", async () => {
  const t = table("player", answering(ok()));
  type(t.view, "say-text", "/ooc where is the pizza");
  pressSend(t.view);
  await settle();
  expect(line(t.sent[0]).outOfCharacter).toBe(true);
  expect(field(t.paint(), "say-text").value).toBe("");
});

// VTT-520
test("an accepted line empties the box the player sees", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "I draw my sword");
  pressSend(t.view);
  h.release(ok());
  await settle();
  expect(field(t.view, "say-text").value).toBe("");
});

// VTT-520
test("the panel asks for a paint once its line is accepted", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "I draw my sword");
  pressSend(t.view);
  const asked = t.repaints;
  h.release(ok());
  await settle();
  expect(t.repaints).toBeGreaterThan(asked);
});

// VTT-518
test("a refused line stays in the box the player sees", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "I draw my sword");
  pressSend(t.view);
  h.release(refused("not now"));
  await settle();
  expect(field(t.view, "say-text").value).toBe("I draw my sword");
});

// VTT-518, VTT-523, VTT-500
test("after a refusal, Send sends the same line again", async () => {
  const t = table("player", answering(refused("not now")));
  type(t.view, "say-text", "I draw my sword");
  pressSend(t.view);
  await settle();
  pressSend(t.paint());
  await settle();
  expect(t.sent).toHaveLength(2);
  expect(line(t.sent[1]).text).toBe("I draw my sword");
  expect(line(t.sent[1]).actorId).toBe("a1");
});

// VTT-504, VTT-520, VTT-521, VTT-522
test("a DM's accepted line keeps say-as and the next line carries it", async () => {
  const t = table("dm", answering(ok()));
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  await settle();
  const next = t.paint();
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("");
  type(next, "say-text", "Bye");
  pressSend(next);
  await settle();
  expect(t.sent).toHaveLength(2);
  expect(line(t.sent[0]).as).toBe("Grimble");
  expect(line(t.sent[1]).text).toBe("Bye");
  expect(line(t.sent[1]).as).toBe("Grimble");
});

// VTT-518, VTT-521, VTT-522
test("a DM's refused line keeps both say-as and say-text", async () => {
  const t = table("dm", answering(refused("not now")));
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  await settle();
  expect(t.sent).toHaveLength(1);
  const next = t.paint();
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("Hello");
});

// VTT-518, VTT-521, VTT-501
test("a DM's refused /ooc line keeps both boxes", async () => {
  const t = table("dm", answering(refused("not now")));
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "/ooc brb");
  pressSend(t.view);
  await settle();
  expect(line(t.sent[0]).outOfCharacter).toBe(true);
  expect(line(t.sent[0]).as).toBe("Grimble");
  const next = t.paint();
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("/ooc brb");
});

// VTT-520, VTT-521, VTT-501
test("a DM's accepted /ooc line empties say-text, keeps say-as", async () => {
  const t = table("dm", answering(ok()));
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "/ooc brb");
  pressSend(t.view);
  await settle();
  expect(line(t.sent[0]).outOfCharacter).toBe(true);
  const next = t.paint();
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("");
  type(next, "say-text", "/ooc back");
  pressSend(next);
  await settle();
  expect(line(t.sent[1]).as).toBe("Grimble");
});

// VTT-522
test("a DM's say-as shows what was typed after a repaint", () => {
  const t = table("dm", answering(ok()));
  type(t.view, "say-as", "Grimble");
  expect(field(t.paint(), "say-as").value).toBe("Grimble");
  expect(t.sent).toHaveLength(0);
});

// VTT-522, VTT-521
test("a speaker changed while waiting is kept after the answer", async () => {
  const h = held();
  const t = table("dm", h.send);
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  type(t.paint(), "say-as", "Mog");
  h.release(ok());
  await settle();
  expect(field(t.paint(), "say-as").value).toBe("Mog");
});

// VTT-504, VTT-520, VTT-521
test("an agent's accepted line keeps say-as, empties say-text", async () => {
  const t = table("agent", answering(ok()));
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  pressEnter(t.view);
  await settle();
  expect(line(t.sent[0]).as).toBe("Grimble");
  const next = t.paint();
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("");
});

// VTT-518, VTT-521
test("an agent's refused line keeps both boxes", async () => {
  const t = table("agent", answering(refused("not now")));
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  await settle();
  expect(t.sent).toHaveLength(1);
  const next = t.paint();
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("Hello");
});

// VTT-523
test("Send twice before the answer sends one line", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  pressSend(t.view);
  expect(t.sent).toHaveLength(1);
  h.release(ok());
  await settle();
  type(t.view, "say-text", "Again");
  pressSend(t.view);
  expect(t.sent).toHaveLength(2);
  h.release(ok());
  await settle();
});

// VTT-523
test("Send then Enter before the answer sends one line", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  pressEnter(t.view);
  expect(t.sent).toHaveLength(1);
  h.release(refused("not now"));
  await settle();
  type(t.view, "say-text", "Again");
  pressEnter(t.view);
  expect(t.sent).toHaveLength(2);
  h.release(ok());
  await settle();
});

// VTT-523
test("Enter twice before the answer sends one line", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "Hello");
  pressEnter(t.view);
  pressEnter(t.view);
  expect(t.sent).toHaveLength(1);
  h.release(ok());
  await settle();
  type(t.view, "say-text", "Again");
  pressEnter(t.view);
  expect(t.sent).toHaveLength(2);
  h.release(ok());
  await settle();
});

// VTT-523
test("Send or Enter on a later paint before the answer sends nothing", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  const later = t.paint();
  type(later, "say-text", "Hello again");
  pressSend(later);
  pressEnter(later);
  expect(t.sent).toHaveLength(1);
  h.release(ok());
  await settle();
  type(t.paint(), "say-text", "Third");
  pressSend(t.view);
  expect(t.sent).toHaveLength(2);
  h.release(ok());
  await settle();
});

// VTT-523
test("a DM's Send then Enter before the answer sends one line", async () => {
  const h = held();
  const t = table("dm", h.send);
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  pressEnter(t.paint());
  pressSend(t.view);
  expect(t.sent).toHaveLength(1);
  h.release(ok());
  await settle();
  type(t.view, "say-text", "Bye");
  pressSend(t.view);
  expect(t.sent).toHaveLength(2);
  h.release(ok());
  await settle();
});

// VTT-523
test("an agent's Send twice before the answer sends one line", async () => {
  const h = held();
  const t = table("agent", h.send);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  pressSend(t.view);
  expect(t.sent).toHaveLength(1);
  h.release(refused("not now"));
  await settle();
  pressSend(t.view);
  expect(t.sent).toHaveLength(2);
  h.release(ok());
  await settle();
});

// VTT-523, VTT-501
test("a blank line or a bare /ooc sends nothing and takes no lock", async () => {
  const h = held();
  const t = table("player", h.send);
  for (const blank of ["", "   ", "/ooc", "/ooc   "]) {
    type(t.view, "say-text", blank);
    pressSend(t.view);
    pressEnter(t.view);
  }
  expect(t.sent).toHaveLength(0);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  expect(t.sent).toHaveLength(1);
  h.release(ok());
  await settle();
});

// VTT-524
test("a player's unanswered Send changes nothing a paint draws", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "I draw my sword");
  const before = t.paint();
  const was = drawn(before);
  expect(drawn(t.paint())).toEqual(was);
  pressSend(before);
  expect(t.sent).toHaveLength(1);
  expect(drawn(before)).toEqual(was);
  expect(drawn(t.paint())).toEqual(was);
  h.release(ok());
  await settle();
});

// VTT-524
test("a player's unanswered Enter changes nothing a paint draws", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "/ooc where is the pizza");
  const before = t.paint();
  const was = drawn(before);
  pressEnter(before);
  expect(t.sent).toHaveLength(1);
  expect(drawn(before)).toEqual(was);
  expect(drawn(t.paint())).toEqual(was);
  h.release(refused("not now"));
  await settle();
});

// VTT-524
test("a DM's unanswered line changes nothing a paint draws", async () => {
  const h = held();
  const t = table("dm", h.send);
  type(t.view, "say-as", "Grimble");
  type(t.view, "say-text", "Hello");
  const before = t.paint();
  const was = drawn(before);
  pressSend(before);
  expect(t.sent).toHaveLength(1);
  expect(drawn(before)).toEqual(was);
  const next = t.paint();
  expect(drawn(next)).toEqual(was);
  expect(field(next, "say-as").value).toBe("Grimble");
  expect(field(next, "say-text").value).toBe("Hello");
  h.release(ok());
  await settle();
});

// VTT-520, VTT-522
test("an accepted answer empties say-text typed into while waiting", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  type(t.paint(), "say-text", "Hello and more");
  h.release(ok());
  await settle();
  expect(line(t.sent[0]).text).toBe("Hello");
  expect(field(t.paint(), "say-text").value).toBe("");
});

// VTT-518, VTT-522
test("a refused answer keeps what was typed while waiting", async () => {
  const h = held();
  const t = table("player", h.send);
  type(t.view, "say-text", "Hello");
  pressSend(t.view);
  type(t.paint(), "say-text", "Hello and more");
  h.release(refused("not now"));
  await settle();
  expect(field(t.paint(), "say-text").value).toBe("Hello and more");
});

// VTT-522, VTT-521, VTT-525
test("a send that rejects keeps both boxes as typed", async () => {
  const real = console.error;
  console.error = () => {};
  try {
    const t = table("dm", () => Promise.reject(new Error("a bug")));
    type(t.view, "say-as", "Grimble");
    type(t.view, "say-text", "Hello");
    pressSend(t.view);
    await settle();
    expect(t.sent).toHaveLength(1);
    const next = t.paint();
    expect(field(next, "say-as").value).toBe("Grimble");
    expect(field(next, "say-text").value).toBe("Hello");
    expect(field(t.view, "say-text").value).toBe("Hello");
  } finally {
    console.error = real;
  }
});

// VTT-525
test("a send that rejects frees the form, logs, and asks no paint", async () => {
  const errors: unknown[][] = [];
  const real = console.error;
  console.error = (...args: unknown[]) => {
    errors.push(args);
  };
  try {
    const t = table("player", () => Promise.reject(new Error("a bug")));
    type(t.view, "say-text", "Hello");
    pressSend(t.view);
    const asked = t.repaints;
    expect(errors).toHaveLength(0);
    await settle();
    expect(errors.length).toBeGreaterThan(0);
    expect(t.repaints).toBe(asked);
    pressSend(t.paint());
    expect(t.sent).toHaveLength(2);
    await settle();
  } finally {
    console.error = real;
  }
});
