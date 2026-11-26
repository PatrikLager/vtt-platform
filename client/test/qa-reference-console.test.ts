import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach, afterEach } from "bun:test";
import { newState, type Actor, type Reference, type State } from "../src/state";
import type { CatalogueEntry, PlaceDef } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import { addReference, changeReference, removeReference } from "../src/commands";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ok, refused, settle } from "./support/answers";
import { forgetTheSay } from "./support/say";

const CATALOGUE: CatalogueEntry[] = [
  { id: "echo", name: "Anvil bell", description: "Rings twice.", form: "kit", values: {} },
  { id: "delta", name: "Zither coat", description: "Hums when struck.", form: "kit", values: {} },
];

const PLACES: PlaceDef[] = [
  { key: "mike", name: "Rack", description: "", forms: [], copies: true, states: ["lima", "oscar"] },
  { key: "kilo", name: "Pouch", description: "", forms: [], copies: true, states: ["oscar"] },
];

const ref = (entryId: string, place: string, state = ""): Reference => ({ entryId, place, state });

const actor = (actorId: string, name: string, references: Record<string, Reference>): Actor => ({
  actorId, name, moduleId: "", controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER,
  attributes: {}, resources: {}, abilityIds: [], references,
});

// Drafts outlive a test: give each test's actors ids no other test uses.
let run = 0;

function table(): { st: State; a: string; b: string } {
  run++;
  const a = `qa-rc${run}-a`;
  const b = `qa-rc${run}-b`;
  const st = newState();
  st.Actors[a] = actor(a, "Wren", { r2: ref("echo", "mike", "lima"), r1: ref("delta", "kilo") });
  st.Actors[b] = actor(b, "Tamsin", { r3: ref("echo", "kilo", "oscar") });
  return { st, a, b };
}

function consoleOver(st: State, answer: CommandResult = ok()) {
  const sent: ClientCommand[] = [];
  const asked: string[] = [];
  let yes = false;
  let repaints = 0;
  const node = renderDMConsole({
    st, participants: [], adventures: [], maps: [], abilities: [], catalogue: CATALOGUE, places: PLACES,
    guideFor: async () => null, joinLink: null, roster: null, origin: "",
    refreshSharing: () => {},
    send: async (c) => {
      sent.push(c);
      return answer;
    },
    rerender: () => {
      repaints++;
    },
    notify: () => {},
    confirm: (m) => {
      asked.push(m);
      return yes;
    },
    doorsArmed: false, toggleDoors: () => {},
  });
  return { node, sent, asked, answerYes: () => (yes = true), repaints: () => repaints };
}

// D18
function catalogue(node: HTMLElement): HTMLElement {
  const h = Array.from(node.querySelectorAll("h3")).find((e) => e.textContent === "Catalogue");
  return h?.parentElement as HTMLElement;
}

const block = (node: HTMLElement, actorId: string) =>
  catalogue(node).querySelector(`[data-actor="${actorId}"]`) as HTMLElement;

const row = (node: HTMLElement, actorId: string, refId: string) =>
  block(node, actorId).querySelector(`[data-reference="${refId}"]`) as HTMLElement;

function addSelect(node: HTMLElement, actorId: string, value: string): HTMLSelectElement {
  const all = Array.from(block(node, actorId).querySelectorAll("select"));
  const found = all.filter((s) => !s.closest("[data-reference]"))
    .filter((s) => Array.from(s.options).some((o) => o.value === value));
  return found[0] as HTMLSelectElement;
}

const addId = (node: HTMLElement, a: string) =>
  node.querySelector(`[data-field="add-reference-id:${a}"]`) as HTMLInputElement;
const addState = (node: HTMLElement, a: string) =>
  node.querySelector(`[data-field="add-reference-state:${a}"]`) as HTMLInputElement;
const rowSelect = (node: HTMLElement, a: string, r: string) =>
  row(node, a, r).querySelector("select") as HTMLSelectElement;
const rowState = (node: HTMLElement, a: string, r: string) =>
  row(node, a, r).querySelector("input") as HTMLInputElement;
const press = (scope: HTMLElement, action: string) =>
  (scope.querySelector(`[data-action="${action}"]`) as HTMLButtonElement).click();

function type(box: HTMLInputElement | HTMLTextAreaElement, v: string): void {
  box.value = v;
  box.dispatchEvent(new Event("input"));
}

function choose(sel: HTMLSelectElement, v: string): void {
  sel.value = v;
  sel.dispatchEvent(new Event("input"));
  sel.dispatchEvent(new Event("change"));
}

const GLOBAL_BOXES = [
  "session-name", "actor-id", "actor-name", "actor-json",
  "token-id", "token-scene", "token-actor", "token-x", "token-y",
  "note-key", "note-title", "note-text", "say-as", "say-text",
];

function controls(node: HTMLElement, t: { st: State; a: string; b: string }) {
  const out: Record<string, HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement> = {};
  for (const f of GLOBAL_BOXES) out[f] = node.querySelector(`[data-field="${f}"]`) as HTMLInputElement;
  for (const who of [t.a, t.b]) {
    out[`add-id:${who}`] = addId(node, who);
    out[`add-state:${who}`] = addState(node, who);
    out[`add-entry:${who}`] = addSelect(node, who, "echo");
    out[`add-place:${who}`] = addSelect(node, who, "mike");
    for (const r of Object.keys(t.st.Actors[who]?.references ?? {})) {
      out[`row-place:${who}:${r}`] = rowSelect(node, who, r);
      out[`row-state:${who}:${r}`] = rowState(node, who, r);
    }
  }
  return out;
}

function typeEverywhere(node: HTMLElement, t: { st: State; a: string; b: string }): Record<string, string> {
  const typed: Record<string, string> = {};
  for (const [key, el] of Object.entries(controls(node, t))) {
    let v = `typed ${key}`;
    if (key === "token-x" || key === "token-y") v = "7";
    if (key.startsWith("add-entry:")) v = "delta";
    if (key.startsWith("add-place:")) v = "kilo";
    if (key.startsWith("row-place:")) v = key.endsWith(":r1") ? "mike" : "kilo";
    if (el instanceof HTMLSelectElement) choose(el, v);
    else type(el, v);
    typed[key] = el.value;
  }
  return typed;
}

const readEverything = (node: HTMLElement, t: { st: State; a: string; b: string }) =>
  Object.fromEntries(Object.entries(controls(node, t)).map(([k, el]) => [k, el.value]));

let last: { st: State; a: string; b: string } | undefined;

beforeEach(() => {
  document.body.innerHTML = "";
});

afterEach(() => {
  for (const st of [newState(), last?.st ?? newState()]) {
    const node = consoleOver(st).node;
    for (const box of Array.from(node.querySelectorAll<HTMLInputElement>("input[data-field], textarea[data-field]"))) {
      type(box, "");
    }
  }
  forgetTheSay();
  last = undefined;
});

// D18
test("the console's Catalogue group lists each actor's references under that actor", () => {
  const t = (last = table());
  const node = consoleOver(t.st).node;
  const ids = (who: string) =>
    Array.from(block(node, who).querySelectorAll("[data-reference]")).map((e) => e.getAttribute("data-reference")).sort();
  expect(ids(t.a)).toEqual(["r1", "r2"]);
  expect(ids(t.b)).toEqual(["r3"]);
});

// D18
test("a reference's row names its entry and its place from the window's catalogue and places", () => {
  const t = (last = table());
  const node = consoleOver(t.st).node;
  const label = row(node, t.a, "r2").textContent ?? "";
  expect(label).toContain("Anvil bell");
  expect(label).toContain("Rack");
  expect(label).not.toContain("echo");
  expect(label).not.toContain("mike");
  const options = (s: HTMLSelectElement) =>
    Array.from(s.options).filter((o) => o.value !== "").map((o) => [o.value, o.textContent]).sort();
  expect(options(addSelect(node, t.a, "echo"))).toEqual([["delta", "Zither coat"], ["echo", "Anvil bell"]]);
  expect(options(addSelect(node, t.a, "mike"))).toEqual([["kilo", "Pouch"], ["mike", "Rack"]]);
  expect(options(rowSelect(node, t.a, "r2"))).toEqual([["kilo", "Pouch"], ["mike", "Rack"]]);
});

// VTT-1057
test("an addition sends exactly the command addReference builds from the id, entry, place and state", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  type(addId(h.node, t.a), "r9");
  choose(addSelect(h.node, t.a, "echo"), "delta");
  choose(addSelect(h.node, t.a, "mike"), "mike");
  type(addState(h.node, t.a), "oscar");
  press(block(h.node, t.a), "add-reference");
  await settle();
  expect(h.sent.map((c) => c.command)).toEqual([addReference(t.a, "r9", "delta", "mike", "oscar").command]);
});

// VTT-1057
test("a change sends the place and the state chosen for that reference", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  choose(rowSelect(h.node, t.a, "r2"), "kilo");
  type(rowState(h.node, t.a, "r2"), "oscar");
  press(row(h.node, t.a, "r2"), "change-reference");
  await settle();
  expect(h.sent.map((c) => c.command)).toEqual([changeReference(t.a, "r2", "kilo", "oscar").command]);
});

// VTT-1057
test("a change that leaves the place as drawn sends the reference's present place", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  type(rowState(h.node, t.a, "r1"), "oscar");
  press(row(h.node, t.a, "r1"), "change-reference");
  await settle();
  expect(h.sent.map((c) => c.command)).toEqual([changeReference(t.a, "r1", "kilo", "oscar").command]);
});

// VTT-1058
test("a removal asks first, and a no sends nothing", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  press(row(h.node, t.a, "r2"), "remove-reference");
  await settle();
  expect(h.asked.length).toBe(1);
  expect(h.sent.length).toBe(0);
});

// VTT-1058
test("a removal the DM answers yes sends exactly the command removeReference builds", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  h.answerYes();
  press(row(h.node, t.b, "r3"), "remove-reference");
  await settle();
  expect(h.asked.length).toBe(1);
  expect(h.sent.map((c) => c.command)).toEqual([removeReference(t.b, "r3").command]);
});

// VTT-1074
test("an accepted addition clears its own four fields and no other field of the console", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  const typed = typeEverywhere(h.node, t);
  type(addId(h.node, t.a), "r9");
  choose(addSelect(h.node, t.a, "echo"), "echo");
  choose(addSelect(h.node, t.a, "mike"), "mike");
  type(addState(h.node, t.a), "lima");
  press(block(h.node, t.a), "add-reference");
  await settle();
  expect(h.sent.length).toBe(1);
  expect(h.repaints()).toBeGreaterThan(0);
  const want = { ...typed };
  want[`add-id:${t.a}`] = "";
  want[`add-state:${t.a}`] = "";
  want[`add-entry:${t.a}`] = "";
  want[`add-place:${t.a}`] = "";
  expect(readEverything(consoleOver(t.st).node, t)).toEqual(want);
});

// VTT-1074
test("an accepted change draws its row as untouched and clears no other field of the console", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  const typed = typeEverywhere(h.node, t);
  choose(rowSelect(h.node, t.a, "r2"), "kilo");
  type(rowState(h.node, t.a, "r2"), "oscar");
  press(row(h.node, t.a, "r2"), "change-reference");
  await settle();
  expect(h.sent.length).toBe(1);
  expect(h.repaints()).toBeGreaterThan(0);
  const want = { ...typed };
  want[`row-place:${t.a}:r2`] = "mike";
  want[`row-state:${t.a}:r2`] = "lima";
  expect(readEverything(consoleOver(t.st).node, t)).toEqual(want);
});

// VTT-1074
test("an accepted removal clears no field of the console", async () => {
  const t = (last = table());
  const h = consoleOver(t.st);
  const typed = typeEverywhere(h.node, t);
  h.answerYes();
  press(row(h.node, t.a, "r1"), "remove-reference");
  await settle();
  expect(h.sent.length).toBe(1);
  expect(readEverything(consoleOver(t.st).node, t)).toEqual(typed);
});

// VTT-517
test("a refused addition keeps what was typed and asks no repaint of its own", async () => {
  const t = (last = table());
  const h = consoleOver(t.st, refused("no such entry"));
  const typed = typeEverywhere(h.node, t);
  type(addId(h.node, t.b), "r9");
  choose(addSelect(h.node, t.b, "echo"), "echo");
  choose(addSelect(h.node, t.b, "mike"), "mike");
  type(addState(h.node, t.b), "lima");
  press(block(h.node, t.b), "add-reference");
  await settle();
  expect(h.sent.length).toBe(1);
  expect(h.repaints()).toBe(0);
  const want = { ...typed };
  want[`add-id:${t.b}`] = "r9";
  want[`add-state:${t.b}`] = "lima";
  want[`add-entry:${t.b}`] = "echo";
  want[`add-place:${t.b}`] = "mike";
  expect(readEverything(consoleOver(t.st).node, t)).toEqual(want);
});

// VTT-517
test("a refused change keeps the place and state typed in its row", async () => {
  const t = (last = table());
  const h = consoleOver(t.st, refused("not a state of that place"));
  const typed = typeEverywhere(h.node, t);
  choose(rowSelect(h.node, t.b, "r3"), "mike");
  type(rowState(h.node, t.b, "r3"), "lima");
  press(row(h.node, t.b, "r3"), "change-reference");
  await settle();
  expect(h.sent.map((c) => c.command)).toEqual([changeReference(t.b, "r3", "mike", "lima").command]);
  expect(h.repaints()).toBe(0);
  const want = { ...typed };
  want[`row-place:${t.b}:r3`] = "mike";
  want[`row-state:${t.b}:r3`] = "lima";
  expect(readEverything(consoleOver(t.st).node, t)).toEqual(want);
});
