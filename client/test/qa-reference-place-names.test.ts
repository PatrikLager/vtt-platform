import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { newState, type Actor, type Reference, type State } from "../src/state";
import type { CatalogueEntry, PlaceDef, SheetPage, SheetSection, SheetUsage } from "../src/metadata";
import { renderPlayerPanel, type PanelLayout } from "../src/view/player";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { ok } from "./support/answers";

const CATALOGUE: CatalogueEntry[] = [
  { id: "echo", name: "Anvil bell", description: "Rings twice.", form: "kit", values: {} },
  { id: "delta", name: "Zither coat", description: "Hums when struck.", form: "kit", values: {} },
];

const RACK: PlaceDef = { key: "mike", name: "Rack", description: "", forms: [], copies: true, states: ["lima"] };
const POUCH: PlaceDef = { key: "kilo", name: "Pouch", description: "", forms: [], copies: true, states: ["oscar"] };

const ref = (entryId: string, place: string, state = ""): Reference => ({ entryId, place, state });

const character = (references: Record<string, Reference>): Actor => ({
  actorId: "c1", name: "Ivo", moduleId: "", controllerId: "p1", controllerIds: ["p1"], kind: ActorKind.PARTY_MEMBER,
  attributes: {}, resources: {}, abilityIds: [], references,
});

const world = (a: Actor): State => {
  const st = newState();
  st.Actors[a.actorId] = a;
  return st;
};

function panel(st: State, layout: PanelLayout | undefined, page?: string): HTMLElement {
  const ui = { selectedActorId: "c1", selectedAbilityId: "", doorsArmed: false, ...(page ? { page } : {}) };
  return renderPlayerPanel(
    st, { participantId: "p1", name: "Pia", role: "player" }, [], ui, async () => ok(), () => {},
    { actors: [{ actorId: "c1", name: "Ivo" }], onSwitch: () => {} }, () => {}, [], layout,
  );
}

function lines(node: HTMLElement, heading: string): string[] {
  const out: string[] = [];
  let inside = false;
  for (const el of Array.from(node.querySelector(".sheet")?.children ?? [])) {
    if (el.tagName === "H3") {
      inside = el.textContent === heading;
      continue;
    }
    if (inside) for (const r of Array.from(el.querySelectorAll(".reference"))) out.push(r.textContent ?? "");
  }
  return out;
}

const allLines = (node: HTMLElement) =>
  Array.from(node.querySelectorAll(".sheet .reference")).map((r) => r.textContent ?? "");

const TWO: SheetPage[] = [{ name: "Gear", sections: [{ name: "Kit", kind: "places", places: ["mike", "kilo"] }] }];
const ONE: SheetPage[] = [{ name: "Gear", sections: [{ name: "Kit", kind: "places", places: ["mike"] }] }];

// Keep a stated and a stateless reference in each place, mike's ids sorting first.
const HELD = {
  b2: ref("delta", "kilo"),
  a1: ref("echo", "mike", "lima"),
  b1: ref("echo", "kilo", "oscar"),
  a2: ref("delta", "mike"),
};

beforeEach(() => {
  document.body.innerHTML = "";
});

// VTT-1075
test("in a section listing two places each reference names its own place by the window's name", () => {
  const drawn = lines(panel(world(character(HELD)), { sheet: TWO, catalogue: CATALOGUE, places: [RACK, POUCH] }), "Kit");
  expect(drawn.length).toBe(4);
  const [a1, a2, b1, b2] = drawn as [string, string, string, string];
  for (const [line, entry, own, other, state] of [
    [a1, "Anvil bell", "Rack", "Pouch", "lima"], [a2, "Zither coat", "Rack", "Pouch", ""],
    [b1, "Anvil bell", "Pouch", "Rack", "oscar"], [b2, "Zither coat", "Pouch", "Rack", ""],
  ] as const) {
    expect(line.startsWith(entry)).toBe(true);
    expect(line).toContain(own);
    expect(line).not.toContain(other);
    expect(line).not.toContain("mike");
    expect(line).not.toContain("kilo");
    if (state !== "") expect(line).toContain(state);
  }
  expect([b1, b2].some((l) => l.includes("lima"))).toBe(false);
});

// VTT-1075
test("in a section listing two places a place the window does not hold is named by its key", () => {
  const drawn = lines(panel(world(character(HELD)), { sheet: TWO, catalogue: CATALOGUE, places: [RACK] }), "Kit");
  expect(drawn.length).toBe(4);
  for (const line of drawn.slice(0, 2)) {
    expect(line).toContain("Rack");
    expect(line).not.toContain("kilo");
  }
  for (const line of drawn.slice(2)) {
    expect(line).toContain("kilo");
    expect(line).not.toContain("Rack");
  }
  expect(drawn[2]).toContain("oscar");
});

// VTT-1075
test("in a section listing two places a window holding no places names each by its key", () => {
  const drawn = lines(panel(world(character(HELD)), { sheet: TWO, catalogue: CATALOGUE }), "Kit");
  expect(drawn.map((l) => [l.includes("mike"), l.includes("kilo")])).toEqual([
    [true, false], [true, false], [false, true], [false, true],
  ]);
});

// VTT-1075
test("a section listing one place does not name the place on its lines", () => {
  const st = world(character({ a1: ref("echo", "mike", "lima"), a2: ref("delta", "mike") }));
  const drawn = lines(panel(st, { sheet: ONE, catalogue: CATALOGUE, places: [RACK, POUCH] }), "Kit");
  expect(drawn.length).toBe(2);
  expect(drawn[0]).toContain("lima");
  expect(drawn[1]).toBe("Zither coat");
  for (const line of drawn) {
    expect(line).not.toContain("Rack");
    expect(line).not.toContain("mike");
  }
});

// VTT-1056 VTT-1075
test("the default page's place sections do not name the place on their lines", () => {
  const st = world(character(HELD));
  for (const layout of [{ catalogue: CATALOGUE, places: [RACK, POUCH] }, undefined]) {
    const drawn = allLines(panel(st, layout));
    expect(drawn.length).toBe(4);
    for (const line of drawn) {
      for (const place of ["Rack", "Pouch", "mike", "kilo"]) expect(line).not.toContain(place);
    }
  }
});

const readJSON = (path: string): any => JSON.parse(readFileSync(path, "utf8"));

function usage(u: any): SheetUsage {
  if (u === "at_will") return { kind: "atWill" };
  if (typeof u?.spends === "string") return { kind: "resource", resource: u.spends };
  throw new Error(`unread usage ${JSON.stringify(u)}`);
}

function section(s: any): SheetSection {
  if (Array.isArray(s.places)) return { name: s.name, kind: "places", places: s.places };
  if (Array.isArray(s.values)) return { name: s.name, kind: "values", values: s.values };
  if (s.conditions === true) return { name: s.name, kind: "conditions" };
  if (Array.isArray(s.abilities)) return { name: s.name, kind: "abilities", abilities: s.abilities.map(usage) };
  throw new Error(`unread section ${JSON.stringify(s)}`);
}

// VTT-1055 VTT-1075
test("the shipped Equipment section draws each of the fighter's references with its own place", () => {
  const rs = readJSON(join(import.meta.dir, "../../rulesets/dnd45e-minimal/ruleset.json"));
  const sheet: SheetPage[] = rs.sheet.map((p: any) => ({ name: p.name, sections: p.sections.map(section) }));
  const places: PlaceDef[] = rs.places.map((p: any) => ({
    key: p.key, name: p.name, description: p.description ?? "", forms: p.forms ?? [], copies: p.copies === true,
    states: p.states ?? [],
  }));
  const catalogue: CatalogueEntry[] = [
    readJSON(join(import.meta.dir, "../../rulesets/dnd45e-minimal/catalogue/longsword.json")),
    readJSON(join(import.meta.dir, "../../rulesets/dnd45e-minimal/catalogue/chainmail.json")),
    readJSON(join(import.meta.dir, "../../rulesets/dnd45e-minimal/catalogue/crossbow.json")),
  ].map((e) => ({ id: e.id, name: e.name, description: e.description ?? "", form: e.form, values: e.values ?? {} }));
  const fighter = readJSON(join(import.meta.dir, "../../adventures/goblin-ambush/actors/act-fighter.json"));
  const references: Record<string, Reference> = {};
  for (const [id, r] of Object.entries<any>(fighter.references)) references[id] = ref(r.entry_id, r.place, r.state ?? "");
  const drawn = lines(panel(world(character(references)), { sheet, catalogue, places }, "Character"), "Equipment");
  expect(drawn.length).toBe(3);
  const want = [["Longsword", "Wielded"], ["Chainmail", "Worn"], ["Crossbow", "Carried"]];
  drawn.forEach((line, i) => {
    const [entry, own] = want[i] as [string, string];
    expect(line.startsWith(entry)).toBe(true);
    expect(line).toContain(own);
    for (const [, other] of want) if (other !== own) expect(line).not.toContain(other);
    for (const key of ["wielded", "worn", "carried"]) expect(line).not.toContain(key);
  });
});
