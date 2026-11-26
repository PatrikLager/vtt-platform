import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect } from "bun:test";
import { newState, type Actor, type Reference, type State } from "../src/state";
import type { CatalogueEntry, PlaceDef, SheetPage, SheetSection, SheetUsage } from "../src/metadata";
import { sheetFor, type ShownSection } from "../src/player";
import { renderPlayerPanel, type PanelLayout } from "../src/view/player";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { ok } from "./support/answers";
import shippedRuleset from "../../rulesets/dnd45e-minimal/ruleset.json";
import longsword from "../../rulesets/dnd45e-minimal/catalogue/longsword.json";
import chainmail from "../../rulesets/dnd45e-minimal/catalogue/chainmail.json";
import crossbow from "../../rulesets/dnd45e-minimal/catalogue/crossbow.json";
import fighterFile from "../../adventures/goblin-ambush/actors/act-fighter.json";

// The shipped sheet, places and entries in the shape /api/ruleset answers them.
type RawSection = { name: string; values?: string[]; conditions?: boolean; abilities?: (string | { spends: string })[]; places?: string[] };
type RawPlace = { key: string; name: string; description: string; forms: string[]; copies?: boolean; states?: string[] };

const usage = (u: string | { spends: string }): SheetUsage =>
  typeof u === "string" ? { kind: "atWill" } : { kind: "resource", resource: u.spends };
const section = (s: RawSection): SheetSection =>
  s.values ? { name: s.name, kind: "values", values: s.values }
    : s.conditions ? { name: s.name, kind: "conditions" }
      : s.abilities ? { name: s.name, kind: "abilities", abilities: s.abilities.map(usage) }
        : { name: s.name, kind: "places", places: s.places ?? [] };

const SHIPPED_SHEET: SheetPage[] = (shippedRuleset.sheet as { name: string; sections: RawSection[] }[])
  .map((p) => ({ name: p.name, sections: p.sections.map(section) }));
const SHIPPED_PLACES: PlaceDef[] = (shippedRuleset.places as RawPlace[])
  .map((p) => ({ key: p.key, name: p.name, description: p.description, forms: p.forms, copies: p.copies ?? false, states: p.states ?? [] }));
const SHIPPED_CATALOGUE: CatalogueEntry[] = [longsword, chainmail, crossbow];

function fighter(): Actor {
  const f = fighterFile as unknown as {
    actor_id: string; name: string; attributes: Record<string, number>;
    resources: Record<string, { current: number; max: number }>; ability_ids: string[];
    references: Record<string, { entry_id: string; place: string; state?: string }>;
  };
  const references: Record<string, Reference> = {};
  for (const [id, r] of Object.entries(f.references)) references[id] = { entryId: r.entry_id, place: r.place, state: r.state ?? "" };
  return {
    actorId: f.actor_id, name: f.name, moduleId: "", attributes: f.attributes, resources: f.resources,
    controllerId: "p1", controllerIds: ["p1"], kind: ActorKind.PARTY_MEMBER, abilityIds: f.ability_ids, references,
  };
}

function panel(st: State, actorId: string, layout: PanelLayout, page?: string): HTMLElement {
  const ui = { selectedActorId: actorId, selectedAbilityId: "", doorsArmed: false, ...(page === undefined ? {} : { page }) };
  return renderPlayerPanel(st, { participantId: "p1", name: "Pia", role: "player" }, [], ui, async () => ok(), () => {},
    { actors: [{ actorId, name: st.Actors[actorId]!.name }], onSwitch: () => {} }, () => {}, [], layout);
}

const lines = (node: HTMLElement) => Array.from(node.querySelectorAll(".sheet .references > .reference"), (r) => r.textContent);
const placesOf = (s: ShownSection | undefined) => (s?.kind === "places" ? s.references.map((r) => [r.id, r.name, r.place]) : undefined);

// VTT-1075
test("the shipped Equipment section names each of the fighter's references with its place", () => {
  const f = fighter();
  const st = newState();
  st.Actors[f.actorId] = f;
  const layout = { sheet: SHIPPED_SHEET, catalogue: SHIPPED_CATALOGUE, places: SHIPPED_PLACES };
  const node = panel(st, f.actorId, layout, "Character");
  expect(Array.from(node.querySelectorAll(".sheet h3"), (h) => h.textContent)).toContain("Equipment");
  expect(lines(node)).toEqual(["Longsword (Wielded)", "Chainmail (Worn)", "Crossbow (Carried)"]);
  const character = sheetFor(f, [], [], [], [], SHIPPED_SHEET, SHIPPED_CATALOGUE, SHIPPED_PLACES).find((p) => p.name === "Character");
  expect(placesOf(character?.sections.find((s) => s.name === "Equipment"))).toEqual([
    ["longsword", "Longsword", "Wielded"], ["chainmail", "Chainmail", "Worn"], ["crossbow", "Crossbow", "Carried"],
  ]);
});

const CATALOGUE: CatalogueEntry[] = [
  { id: "echo", name: "Echo Blade", description: "", form: "zulu", values: {} },
  { id: "delta", name: "Delta Mail", description: "", form: "yankee", values: {} },
];
const PLACES: PlaceDef[] = [
  { key: "mike", name: "Mike Rack", description: "", forms: ["zulu", "yankee"], copies: true, states: ["lima", "oscar"] },
  { key: "kilo", name: "Kilo Hook", description: "", forms: ["yankee"], copies: false, states: [] },
];
const CARRIED: Record<string, Reference> = {
  "r-c": { entryId: "delta", place: "mike", state: "oscar" },
  "r-a": { entryId: "delta", place: "kilo", state: "" },
  "r-b": { entryId: "echo", place: "mike", state: "lima" },
};

function carrier(): Actor {
  return {
    actorId: "c1", name: "Ivo", moduleId: "", attributes: {}, resources: {}, controllerId: "p1", controllerIds: ["p1"],
    kind: ActorKind.PARTY_MEMBER, abilityIds: [], references: CARRIED,
  };
}

const TWO: SheetPage[] = [{ name: "Gear", sections: [{ name: "Gear", kind: "places", places: ["mike", "kilo"] }] }];
const ONE: SheetPage[] = [{ name: "Gear", sections: [
  { name: "Rack", kind: "places", places: ["mike"] }, { name: "Hook", kind: "places", places: ["kilo"] },
] }];

// VTT-1075
test("in a section listing two places each reference carries its own place, named by the window, else by its key", () => {
  const shown = (places: PlaceDef[] | undefined) => placesOf(sheetFor(carrier(), [], [], [], [], TWO, CATALOGUE, places)[0]!.sections[0]);
  expect(shown(PLACES)).toEqual([["r-b", "Echo Blade", "Mike Rack"], ["r-c", "Delta Mail", "Mike Rack"], ["r-a", "Delta Mail", "Kilo Hook"]]);
  expect(shown([PLACES[0]!])).toEqual([["r-b", "Echo Blade", "Mike Rack"], ["r-c", "Delta Mail", "Mike Rack"], ["r-a", "Delta Mail", "kilo"]]);
  expect(shown(undefined)).toEqual([["r-b", "Echo Blade", "mike"], ["r-c", "Delta Mail", "mike"], ["r-a", "Delta Mail", "kilo"]]);
});

// VTT-1075
test("a section listing two places names the place though the character holds references in only one", () => {
  const one: Actor = { ...carrier(), references: { "r-b": CARRIED["r-b"]! } };
  expect(placesOf(sheetFor(one, [], [], [], [], TWO, CATALOGUE, PLACES)[0]!.sections[0]))
    .toEqual([["r-b", "Echo Blade", "Mike Rack"]]);
});

// VTT-1075
test("a section listing one place, and the default page's place sections, carry no place", () => {
  const [one] = sheetFor(carrier(), [], [], [], [], ONE, CATALOGUE, PLACES);
  expect(one!.sections.map(placesOf)).toEqual([
    [["r-b", "Echo Blade", ""], ["r-c", "Delta Mail", ""]], [["r-a", "Delta Mail", ""]],
  ]);
  const [bare] = sheetFor(carrier(), [], [], [], [], undefined, CATALOGUE, PLACES);
  expect(bare!.sections.map((s) => [s.name, placesOf(s)])).toEqual([
    ["Mike Rack", [["r-b", "Echo Blade", ""], ["r-c", "Delta Mail", ""]]], ["Kilo Hook", [["r-a", "Delta Mail", ""]]],
  ]);
});

// VTT-1075
test("the panel draws a reference of a two-place section as its name, then its place and its state in parentheses", () => {
  const st = newState();
  st.Actors["c1"] = carrier();
  expect(lines(panel(st, "c1", { sheet: TWO, catalogue: CATALOGUE, places: PLACES })))
    .toEqual(["Echo Blade (Mike Rack, lima)", "Delta Mail (Mike Rack, oscar)", "Delta Mail (Kilo Hook)"]);
  expect(lines(panel(st, "c1", { sheet: TWO, catalogue: CATALOGUE })))
    .toEqual(["Echo Blade (mike, lima)", "Delta Mail (mike, oscar)", "Delta Mail (kilo)"]);
});

// VTT-1075
test("the panel draws a reference of a one-place section, or of the default page, with no place", () => {
  const st = newState();
  st.Actors["c1"] = carrier();
  const drawn = ["Echo Blade (lima)", "Delta Mail (oscar)", "Delta Mail"];
  expect(lines(panel(st, "c1", { sheet: ONE, catalogue: CATALOGUE, places: PLACES }))).toEqual(drawn);
  expect(lines(panel(st, "c1", { catalogue: CATALOGUE, places: PLACES }))).toEqual(drawn);
});
