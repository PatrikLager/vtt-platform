import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type Actor, type Reference, type State } from "../src/state";
import type { Ability, CatalogueEntry, PlaceDef, SheetPage } from "../src/metadata";
import { sheetFor } from "../src/player";
import { renderPlayerPanel, type PanelLayout } from "../src/view/player";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { ok } from "./support/answers";

const CATALOGUE: CatalogueEntry[] = [
  { id: "echo", name: "Anvil bell", description: "Rings twice.", form: "kit", values: {} },
  { id: "delta", name: "Zither coat", description: "Hums when struck.", form: "kit", values: {} },
];

const PLACES: PlaceDef[] = [
  { key: "mike", name: "Rack", description: "", forms: [], copies: true, states: ["lima", "oscar"] },
  { key: "kilo", name: "Pouch", description: "", forms: [], copies: true, states: ["oscar"] },
];

const ref = (entryId: string, place: string, state = ""): Reference => ({ entryId, place, state });

const character = (actorId: string, name: string, references: Record<string, Reference>): Actor => ({
  actorId, name, moduleId: "", controllerId: "p1", controllerIds: ["p1"], kind: ActorKind.PARTY_MEMBER,
  attributes: {}, resources: {}, abilityIds: [], references,
});

const world = (...actors: Actor[]): State => {
  const st = newState();
  for (const a of actors) st.Actors[a.actorId] = a;
  return st;
};

const KIT_SHEET: SheetPage[] = [
  { name: "Gear", sections: [{ name: "Kit", kind: "places", places: ["mike", "kilo"] }] },
];

function panel(
  st: State,
  layout: PanelLayout | undefined,
  opts: { follow?: string; role?: "player" | "dm"; abilities?: Ability[] } = {},
): HTMLElement {
  const actors = Object.values(st.Actors).map((a) => ({ actorId: a.actorId, name: a.name }));
  const role = opts.role ?? "player";
  return renderPlayerPanel(
    st, { participantId: role === "dm" ? "d1" : "p1", name: "Pia", role }, opts.abilities ?? [],
    { selectedActorId: opts.follow ?? "c1", selectedAbilityId: "", doorsArmed: false },
    async () => ok(), () => {}, { actors, onSwitch: () => {} }, () => {}, [], layout,
  );
}

type Drawn = { name: string; refs: { text: string; title: string | null }[] };

function drawn(node: HTMLElement): Drawn[] {
  const out: Drawn[] = [];
  for (const el of Array.from(node.querySelector(".sheet")?.children ?? [])) {
    if (el.tagName === "H3") {
      out.push({ name: el.textContent ?? "", refs: [] });
      continue;
    }
    const cur = out[out.length - 1];
    for (const r of Array.from(el.querySelectorAll(".reference"))) {
      cur?.refs.push({ text: r.textContent ?? "", title: r.getAttribute("title") });
    }
  }
  return out;
}

const headings = (node: HTMLElement) => drawn(node).map((s) => s.name);

function which(text: string, table: Record<string, [string, string]>): string {
  const hits = Object.entries(table).filter(([, [name, state]]) =>
    state === "" ? text === name : text.startsWith(name) && text.includes(state));
  return hits.length === 1 ? (hits[0]?.[0] ?? "?") : `?${text}`;
}

beforeEach(() => {
  document.body.innerHTML = "";
});

// Keep these ids sorting apart from their entries' names, ids and insertion order.
const ORDERED = {
  m3: ref("echo", "mike", "oscar"),
  k1: ref("delta", "kilo", "oscar"),
  m2: ref("delta", "mike"),
  m1: ref("echo", "mike", "lima"),
};
const ORDERED_TEXT: Record<string, [string, string]> = {
  m1: ["Anvil bell", "lima"], m2: ["Zither coat", ""], m3: ["Anvil bell", "oscar"], k1: ["Zither coat", "oscar"],
};

// VTT-1055
test("a section of places draws the references by the order of its places, then by reference id", () => {
  const layout = { sheet: KIT_SHEET, catalogue: CATALOGUE, places: PLACES };
  const kit = drawn(panel(world(character("c1", "Ivo", ORDERED)), layout)).find((s) => s.name === "Kit");
  expect(kit?.refs.map((r) => which(r.text, ORDERED_TEXT))).toEqual(["m1", "m2", "m3", "k1"]);
});

// VTT-1055
test("the sheet the panel draws holds a places section's references by place, then by reference id", () => {
  const pages = sheetFor(character("c1", "Ivo", ORDERED), [], [], [], [], KIT_SHEET, CATALOGUE, PLACES);
  const kit = pages[0]?.sections[0];
  expect(kit?.kind).toBe("places");
  expect(kit?.kind === "places" ? kit.references.map((r) => r.id) : []).toEqual(["m1", "m2", "m3", "k1"]);
});

// VTT-1055 D17
test("a reference is drawn by its entry's name with its state, titled by the entry's description", () => {
  const st = world(character("c1", "Ivo", { x1: ref("echo", "mike", "lima"), x2: ref("delta", "kilo") }));
  const kit = drawn(panel(st, { sheet: KIT_SHEET, catalogue: CATALOGUE, places: PLACES })).find((s) => s.name === "Kit");
  expect(kit?.refs.length).toBe(2);
  expect(kit?.refs[0]?.text.startsWith("Anvil bell")).toBe(true);
  expect(kit?.refs[0]?.text).toContain("lima");
  expect(kit?.refs[0]?.title).toBe("Rings twice.");
  expect(kit?.refs[1]?.text).toBe("Zither coat");
  expect(kit?.refs[1]?.title).toBe("Hums when struck.");
});

// VTT-1055
test("a reference whose entry the window's catalogue lacks is drawn by its entry's id", () => {
  const st = world(character("c1", "Ivo", { x1: ref("echo", "mike"), x2: ref("delta", "mike", "oscar") }));
  const only = CATALOGUE.filter((e) => e.id === "echo");
  const kit = drawn(panel(st, { sheet: KIT_SHEET, catalogue: only, places: PLACES })).find((s) => s.name === "Kit");
  expect(kit?.refs[0]?.text).toBe("Anvil bell");
  expect(kit?.refs[1]?.text.startsWith("delta")).toBe(true);
  expect(kit?.refs[1]?.text).toContain("oscar");
  expect(kit?.refs[1]?.text).not.toContain("Zither");
  const shown = sheetFor(st.Actors["c1"] as Actor, [], [], [], [], KIT_SHEET, only, PLACES)[0]?.sections[0];
  expect(shown?.kind === "places" ? shown.references.map((r) => [r.id, r.name, r.state]) : []).toEqual([
    ["x1", "Anvil bell", ""], ["x2", "delta", "oscar"],
  ]);
});

// VTT-1055
test("a section of places holding none of the character's references is left out", () => {
  const sheet: SheetPage[] = [{ name: "Gear", sections: [
    { name: "Hands", kind: "places", places: ["mike"] },
    { name: "Bags", kind: "places", places: ["kilo"] },
  ] }];
  const st = world(character("c1", "Ivo", { x1: ref("echo", "mike") }));
  expect(headings(panel(st, { sheet, catalogue: CATALOGUE, places: PLACES }))).toEqual(["Hands"]);
});

// VTT-1055
test("a reference held in a place the section does not list is not drawn in it", () => {
  const sheet: SheetPage[] = [{ name: "Gear", sections: [{ name: "Hands", kind: "places", places: ["mike"] }] }];
  const st = world(character("c1", "Ivo", { a1: ref("delta", "kilo", "oscar"), x1: ref("echo", "mike") }));
  const out = drawn(panel(st, { sheet, catalogue: CATALOGUE, places: PLACES }));
  expect(out.map((s) => [s.name, s.refs.map((r) => r.text)])).toEqual([["Hands", ["Anvil bell"]]]);
});

// VTT-1055
test("the panel draws the references of the character it follows and of no other", () => {
  const st = world(
    character("c1", "Ivo", { x1: ref("echo", "mike") }),
    character("c2", "Ada", { x2: ref("delta", "mike") }),
  );
  const layout = { sheet: KIT_SHEET, catalogue: CATALOGUE, places: PLACES };
  const texts = (n: HTMLElement) => drawn(n).flatMap((s) => s.refs.map((r) => r.text));
  expect(texts(panel(st, layout, { follow: "c2" }))).toEqual(["Zither coat"]);
  expect(texts(panel(st, layout, { follow: "c1" }))).toEqual(["Anvil bell"]);
  expect(texts(panel(st, layout, { follow: "c2", role: "dm" }))).toEqual(["Zither coat"]);
});

const JAB: Ability = { id: "jab", name: "Jab", range: 1, maxTargets: 1, usage: { kind: "atWill" } };

// Keep a value, a condition and an ability, or the default page drops a section.
function fullWorld(references: Record<string, Reference>): State {
  const c = character("c1", "Ivo", references);
  c.attributes = { grit: 2 };
  c.abilityIds = ["jab"];
  const st = world(c);
  st.Conditions["c1"] = [{ ID: "dazed", Source: "dm", AppliedSeq: 1 }];
  return st;
}

const WINDOW_ORDER: PlaceDef[] = [
  { key: "mike", name: "Rack", description: "", forms: [], copies: true, states: [] },
  { key: "kilo", name: "Pouch", description: "", forms: [], copies: true, states: [] },
];

// VTT-1056
test("with no sheet declared, a section per place holding a reference follows Abilities, in the window's order", () => {
  const st = fullWorld({ k1: ref("delta", "kilo"), m1: ref("echo", "mike") });
  const node = panel(st, { catalogue: CATALOGUE, places: WINDOW_ORDER }, { abilities: [JAB] });
  expect(headings(node)).toEqual(["Values", "Conditions", "Abilities", "Rack", "Pouch"]);
  expect(node.querySelectorAll(".tab").length).toBe(0);
});

// VTT-1056
test("with a sheet declared empty, the place sections follow Abilities as with none", () => {
  const st = fullWorld({ k1: ref("delta", "kilo"), m1: ref("echo", "mike") });
  const node = panel(st, { sheet: [], catalogue: CATALOGUE, places: WINDOW_ORDER }, { abilities: [JAB] });
  expect(headings(node)).toEqual(["Values", "Conditions", "Abilities", "Rack", "Pouch"]);
});

// VTT-1056
test("with no sheet declared, a place the window lacks follows those it holds, named by its key", () => {
  const st = fullWorld({ k1: ref("delta", "kilo"), m1: ref("echo", "mike") });
  const mikeOnly = WINDOW_ORDER.filter((p) => p.key === "mike");
  const node = panel(st, { catalogue: CATALOGUE, places: mikeOnly }, { abilities: [JAB] });
  expect(headings(node)).toEqual(["Values", "Conditions", "Abilities", "Rack", "kilo"]);
});

// VTT-1056
test("with no ruleset, each place holding a reference is a section by key, its references by entry id", () => {
  const st = fullWorld({ m2: ref("echo", "mike"), k1: ref("delta", "kilo", "lima") });
  const out = drawn(panel(st, undefined, { abilities: [JAB] }));
  expect(out.map((s) => s.name)).toEqual(["Values", "Conditions", "Abilities", "kilo", "mike"]);
  expect(out[3]?.refs.map((r) => r.text.startsWith("delta") && r.text.includes("lima"))).toEqual([true]);
  expect(out[4]?.refs.map((r) => r.text)).toEqual(["echo"]);
});

// VTT-1055 VTT-1056 D17
test("with no sheet declared, a place's section holds its references by reference id, named and titled", () => {
  const st = fullWorld({ m3: ref("echo", "mike", "oscar"), m2: ref("delta", "mike"), m1: ref("echo", "mike", "lima") });
  const rack = drawn(panel(st, { catalogue: CATALOGUE, places: WINDOW_ORDER })).find((s) => s.name === "Rack");
  const table: Record<string, [string, string]> = {
    m1: ["Anvil bell", "lima"], m2: ["Zither coat", ""], m3: ["Anvil bell", "oscar"],
  };
  expect(rack?.refs.map((r) => which(r.text, table))).toEqual(["m1", "m2", "m3"]);
  expect(rack?.refs.map((r) => r.title)).toEqual(["Rings twice.", "Hums when struck.", "Rings twice."]);
});

// VTT-1056
test("with no sheet declared, a character holding no reference is drawn no place's section", () => {
  const node = panel(fullWorld({}), { catalogue: CATALOGUE, places: WINDOW_ORDER }, { abilities: [JAB] });
  expect(headings(node)).toEqual(["Values", "Conditions", "Abilities"]);
});

// VTT-1056
test("the sheet the panel draws with no sheet declared is one page ending in the places' sections", () => {
  const st = fullWorld({ k1: ref("delta", "kilo"), m1: ref("echo", "mike") });
  const pages = sheetFor(st.Actors["c1"] as Actor, st.Conditions["c1"] ?? [], [JAB], [], [], undefined, CATALOGUE,
    WINDOW_ORDER);
  expect(pages.length).toBe(1);
  expect(pages[0]?.sections.map((s) => [s.name, s.kind])).toEqual([
    ["Values", "values"], ["Conditions", "conditions"], ["Abilities", "abilities"],
    ["Rack", "places"], ["Pouch", "places"],
  ]);
});
