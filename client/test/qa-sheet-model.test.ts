import "./support/dom";
import { test, expect } from "bun:test";
import { sheetFor } from "../src/player";
import type { Ability, ConditionDef, Property, SheetPage, SheetSection, SheetUsage } from "../src/metadata";
import type { Actor, ActorCondition } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

type Held = { attrs?: Record<string, number>; res?: Record<string, [number, number]>; abilityIds?: string[] };
const actor = (h: Held = {}): Actor => {
  const resources: Record<string, { current: number; max: number }> = {};
  for (const [k, [c, m]] of Object.entries(h.res ?? {})) resources[k] = { current: c, max: m };
  return {
    actorId: "a1", name: "Ayla", moduleId: "", attributes: { ...(h.attrs ?? {}) }, resources,
    controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER, abilityIds: h.abilityIds ?? [],
  };
};
const ab = (id: string, usage: Ability["usage"]): Ability =>
  ({ id, name: id.toUpperCase(), range: 1, maxTargets: 1, usage });
const prop = (key: string, type: Property["type"], name: string, description: string): Property =>
  ({ key, type, name, description });
const cdef = (id: string, name: string, description: string): ConditionDef =>
  ({ id, name, description });
const cond = (id: string, seq: number): ActorCondition => ({ ID: id, Source: "s", AppliedSeq: seq });
const sheet = (pages: SheetPage[]): SheetPage[] => pages;
const vals = (name: string, values: string[]): SheetSection => ({ name, kind: "values", values });
const conds = (name: string): SheetSection => ({ name, kind: "conditions" });
const abs = (name: string, abilities: SheetUsage[]): SheetSection => ({ name, kind: "abilities", abilities });
const names = (p: { sections: { name: string }[] }) => p.sections.map((s) => s.name);
const section = (p: any, name: string): any => p.sections.find((s: any) => s.name === name);
const keys = (s: any) => s.values.map((v: any) => v.key);
const ids = (s: any) => s.abilities.map((a: any) => a.id);

const PROPS = [
  prop("zeta", "number", "Zeta Name", "zeta desc"),
  prop("hp", "resource", "Hit Points", "hp desc"),
  prop("alpha", "number", "Alpha Name", "alpha desc"),
];

// VTT-839
test("a declared sheet draws its pages and sections in the declared order, with each section's values in the section's order", () => {
  const a = actor({ attrs: { alpha: 1, zeta: 9 }, res: { hp: [4, 10] } });
  const pages = sheetFor(a, [], [], PROPS, [], sheet([
    { name: "P-two", sections: [vals("S-b", ["hp", "zeta", "alpha"]), vals("S-a", ["alpha"])] },
    { name: "P-one", sections: [vals("S-c", ["zeta", "alpha"])] },
  ]));
  expect(pages.map((p) => p.name)).toEqual(["P-two", "P-one"]);
  expect(names(pages[0]!)).toEqual(["S-b", "S-a"]);
  expect(keys(section(pages[0], "S-b"))).toEqual(["hp", "zeta", "alpha"]);
  expect(keys(section(pages[1], "S-c"))).toEqual(["zeta", "alpha"]);
});

// VTT-839
test("a declared section with nothing to show is left out and a page left with no section is still drawn", () => {
  const a = actor({ attrs: { alpha: 1 } });
  const pages = sheetFor(a, [], [], PROPS, [], sheet([
    { name: "P1", sections: [vals("Empty", ["hp"]), vals("Full", ["alpha"]), conds("Cond"), abs("Abil", [{ kind: "atWill" }])] },
    { name: "P2", sections: [vals("Nothing", ["hp", "zeta"]), conds("NoCond")] },
  ]));
  expect(pages.map((p) => p.name)).toEqual(["P1", "P2"]);
  expect(names(pages[0]!)).toEqual(["Full"]);
  expect(pages[1]!.sections).toEqual([]);
});

// VTT-839
test("a declared sheet draws no section it does not declare, whatever the character holds", () => {
  const a = actor({ attrs: { alpha: 1, zeta: 2 }, res: { hp: [1, 2] } });
  const pages = sheetFor(a, [cond("c", 1)], [], PROPS, [], sheet([{ name: "P", sections: [vals("Only", ["alpha"])] }]));
  expect(pages).toHaveLength(1);
  expect(names(pages[0]!)).toEqual(["Only"]);
  expect(keys(pages[0]!.sections[0])).toEqual(["alpha"]);
});

// VTT-840
test("with no sheet declared (undefined or empty) one page of Values, Conditions and Abilities in that order, each left out when empty", () => {
  const a = actor({ attrs: { alpha: 1 }, abilityIds: ["slash"] });
  const all = [ab("slash", { kind: "atWill" })];
  for (const sh of [undefined, [] as SheetPage[]]) {
    const pages = sheetFor(a, [cond("c", 1)], all, PROPS, [], sh);
    expect(pages).toHaveLength(1);
    expect(names(pages[0]!)).toEqual(["Values", "Conditions", "Abilities"]);
    const bare = sheetFor(actor({ attrs: { alpha: 1 } }), [], all, PROPS, [], sh);
    expect(names(bare[0]!)).toEqual(["Values"]);
    const onlyCond = sheetFor(actor(), [cond("c", 1)], all, PROPS, [], sh);
    expect(names(onlyCond[0]!)).toEqual(["Conditions"]);
    const onlyAb = sheetFor(actor({ abilityIds: ["slash"] }), [], all, PROPS, [], sh);
    expect(names(onlyAb[0]!)).toEqual(["Abilities"]);
  }
});

// VTT-840
test("with no ruleset at all the undeclared page holds every value the sheet holds, by key", () => {
  const a = actor({ attrs: { b: 2, c: 3, a: 1 }, res: { hp: [1, 2] } });
  const pages = sheetFor(a, [], [], undefined, undefined, undefined);
  expect(pages).toHaveLength(1);
  expect(keys(section(pages[0], "Values"))).toEqual(["a", "b", "c", "hp"]);
});

// VTT-840
test("the undeclared page orders values by the window's properties, then undeclared keys by key, an attribute before a resource of the same key", () => {
  const a = actor({
    attrs: { alpha: 1, zeta: 2, q: 5, m: 6, k: 7 },
    res: { hp: [3, 9], q: [1, 2], aa: [0, 1] },
  });
  const pages = sheetFor(a, [], [], PROPS, [], undefined);
  const v = section(pages[0], "Values");
  expect(v.values.map((x: any) => `${x.key}:${x.text}`)).toEqual([
    "zeta:2", "hp:3/9", "alpha:1", "aa:0/1", "k:7", "m:6", "q:5", "q:1/2",
  ]);
});

// VTT-840
test("the undeclared page orders undeclared keys by < (code unit order), not by locale", () => {
  const a = actor({ attrs: { b: 1, B: 2, a: 3, Z: 4, _x: 5 } });
  const v = section(sheetFor(a, [], [], [], [], undefined)[0], "Values");
  expect(keys(v)).toEqual(["B", "Z", "_x", "a", "b"]);
});

// VTT-840
test("the undeclared page leaves out a property the character does not hold", () => {
  const a = actor({ attrs: { alpha: 1 } });
  const v = section(sheetFor(a, [], [], PROPS, [], undefined)[0], "Values");
  expect(keys(v)).toEqual(["alpha"]);
});

// VTT-841
test("a value is named by the declaring property's display name, a number as itself, titled by the description", () => {
  const a = actor({ attrs: { alpha: 7 } });
  const v = section(sheetFor(a, [], [], PROPS, [], sheet([{ name: "P", sections: [vals("S", ["alpha"])] }]))[0], "S");
  expect(v.values).toHaveLength(1);
  const x = v.values[0];
  expect(x.name).toBe("Alpha Name");
  expect(x.text).toBe("7");
  expect(x.title).toBe("alpha desc");
});

// VTT-841
test("a resource value reads current/max, named and titled by its resource property", () => {
  const a = actor({ res: { hp: [4, 10] } });
  const v = section(sheetFor(a, [], [], PROPS, [], sheet([{ name: "P", sections: [vals("S", ["hp"])] }]))[0], "S");
  const x = v.values[0];
  expect(x.name).toBe("Hit Points");
  expect(x.text).toBe("4/10");
  expect(x.title).toBe("hp desc");
});

// VTT-841
test("a key the ruleset declares as the other type is named by its key with no title", () => {
  // hp is declared a resource, held as an attribute; alpha the other way round.
  const a = actor({ attrs: { hp: 3 }, res: { alpha: [2, 5] } });
  const pages = sheetFor(a, [], [], PROPS, [], sheet([{ name: "P", sections: [vals("S", ["hp", "alpha"])] }]));
  const v = section(pages[0], "S");
  const byKey = Object.fromEntries(v.values.map((x: any) => [x.key, x]));
  expect(byKey.hp.name).toBe("hp");
  expect(byKey.hp.text).toBe("3");
  expect(byKey.hp.title).toBe("");
  expect(byKey.alpha.name).toBe("alpha");
  expect(byKey.alpha.text).toBe("2/5");
  expect(byKey.alpha.title).toBe("");
  const und = section(sheetFor(a, [], [], PROPS, [], undefined)[0], "Values");
  for (const x of und.values) { expect(x.name).toBe(x.key); expect(x.title).toBe(""); }
});

// VTT-841
test("with no properties every value is named by its key with no title", () => {
  const a = actor({ attrs: { alpha: 1 }, res: { hp: [1, 2] } });
  for (const props of [undefined, [] as Property[]]) {
    const v = section(sheetFor(a, [], [], props, [], sheet([{ name: "P", sections: [vals("S", ["alpha", "hp"])] }]))[0], "S");
    expect(v.values.map((x: any) => [x.name, x.title, x.text])).toEqual([["alpha", "", "1"], ["hp", "", "1/2"]]);
  }
});

// VTT-841
test("a key the character does not hold draws nothing, in a declared section and on the undeclared page", () => {
  const a = actor({ attrs: { alpha: 1 } });
  const pages = sheetFor(a, [], [], PROPS, [], sheet([{ name: "P", sections: [vals("S", ["ghost", "alpha", "hp"])] }]));
  expect(keys(section(pages[0], "S"))).toEqual(["alpha"]);
  const und = section(sheetFor(a, [], [], [...PROPS, prop("ghost", "number", "G", "g")], [], undefined)[0], "Values");
  expect(keys(und)).toEqual(["alpha"]);
});

// VTT-842
test("conditions are listed in the order applied, named by the ruleset's name and titled by its description", () => {
  const defs = [cdef("a", "Alpha", "A desc"), cdef("m", "Mid", "M desc"), cdef("z", "Zed", "Z desc")];
  const applied = [cond("z", 5), cond("a", 9), cond("m", 12)];
  const pages = sheetFor(actor(), applied, [], [], defs, undefined);
  const c = section(pages[0], "Conditions");
  expect(c.conditions.map((x: any) => [x.id, x.name, x.title])).toEqual([
    ["z", "Zed", "Z desc"], ["a", "Alpha", "A desc"], ["m", "Mid", "M desc"],
  ]);
});

// VTT-842
test("a condition the ruleset does not declare is named by its id with no title, in place", () => {
  const defs = [cdef("a", "Alpha", "A desc")];
  const pages = sheetFor(actor(), [cond("x-unknown", 1), cond("a", 2), cond("b-unknown", 3)], [], [], defs, undefined);
  const c = section(pages[0], "Conditions");
  expect(c.conditions.map((x: any) => [x.id, x.name, x.title])).toEqual([
    ["x-unknown", "x-unknown", ""], ["a", "Alpha", "A desc"], ["b-unknown", "b-unknown", ""],
  ]);
  const none = section(sheetFor(actor(), [cond("q", 1)], [], [], undefined, undefined)[0], "Conditions");
  expect(none.conditions.map((x: any) => [x.name, x.title])).toEqual([["q", ""]]);
});

// VTT-842
test("a declared conditions section lists the conditions the same way", () => {
  const defs = [cdef("a", "Alpha", "A desc")];
  const pages = sheetFor(actor(), [cond("z", 1), cond("a", 2)], [], [], defs, sheet([{ name: "P", sections: [conds("Hurts")] }]));
  expect(section(pages[0], "Hurts").conditions.map((x: any) => x.name)).toEqual(["z", "Alpha"]);
});

const LIST = [
  ab("fire", { kind: "resource", resource: "mana", cost: 2 }),
  ab("slash", { kind: "atWill" }),
  ab("frost", { kind: "resource", resource: "mana", cost: 1 }),
  ab("rage", { kind: "resource", resource: "fury", cost: 1 }),
  ab("jab", { kind: "atWill" }),
  ab("unknown", { kind: "atWill" }),
];

// VTT-843
test("each offered ability sits in exactly the one section whose usages name its usage, in the window's list order", () => {
  const a = actor({ abilityIds: ["jab", "rage", "frost", "slash", "fire"] });
  const pages = sheetFor(a, [], LIST, [], [], sheet([{ name: "P", sections: [
    abs("Basic", [{ kind: "atWill" }]),
    abs("Magic", [{ kind: "resource", resource: "mana" }]),
    abs("Rage", [{ kind: "resource", resource: "fury" }]),
  ] }]));
  const p = pages[0];
  expect(ids(section(p, "Basic"))).toEqual(["slash", "jab"]);
  expect(ids(section(p, "Magic"))).toEqual(["fire", "frost"]);
  expect(ids(section(p, "Rage"))).toEqual(["rage"]);
});

// VTT-843
test("a section naming several usages holds the abilities of each, in the list's order", () => {
  const a = actor({ abilityIds: ["jab", "rage", "frost", "slash", "fire"] });
  const pages = sheetFor(a, [], LIST, [], [], sheet([{ name: "P", sections: [
    abs("Mixed", [{ kind: "resource", resource: "mana" }, { kind: "atWill" }]),
    abs("Rest", [{ kind: "resource", resource: "fury" }]),
  ] }]));
  expect(ids(section(pages[0], "Mixed"))).toEqual(["fire", "slash", "frost", "jab"]);
  expect(ids(section(pages[0], "Rest"))).toEqual(["rage"]);
});

// VTT-843
test("an ability the character's sheet does not name is drawn in no section", () => {
  const a = actor({ abilityIds: ["slash"] });
  const pages = sheetFor(a, [], LIST, [], [], sheet([{ name: "P", sections: [
    abs("Basic", [{ kind: "atWill" }]),
    abs("Magic", [{ kind: "resource", resource: "mana" }]),
  ] }]));
  expect(names(pages[0]!)).toEqual(["Basic"]);
  expect(ids(section(pages[0], "Basic"))).toEqual(["slash"]);
});

// VTT-843
test("a usage no section names draws its ability nowhere on a declared sheet", () => {
  const a = actor({ abilityIds: ["slash", "fire"] });
  const pages = sheetFor(a, [], LIST, [], [], sheet([{ name: "P", sections: [abs("Basic", [{ kind: "atWill" }])] }]));
  expect(JSON.stringify(pages)).not.toContain("fire");
  expect(ids(section(pages[0], "Basic"))).toEqual(["slash"]);
});

// VTT-843
test("an abilities section whose usages match no offered ability is left out", () => {
  const a = actor({ abilityIds: ["slash"] });
  const pages = sheetFor(a, [], LIST, [], [], sheet([{ name: "P", sections: [abs("Magic", [{ kind: "resource", resource: "mana" }])] }]));
  expect(pages[0]!.sections).toEqual([]);
});

// VTT-843
test("a resource usage names only the resource it spends, not another resource", () => {
  const a = actor({ abilityIds: ["rage", "fire"] });
  const pages = sheetFor(a, [], LIST, [], [], sheet([{ name: "P", sections: [abs("Fury", [{ kind: "resource", resource: "fury" }])] }]));
  expect(ids(section(pages[0], "Fury"))).toEqual(["rage"]);
});

// VTT-840
test("the undeclared page's Abilities section holds every offered ability in the list's order and none the sheet does not name", () => {
  const a = actor({ abilityIds: ["jab", "rage", "frost", "slash", "fire"] });
  const pages = sheetFor(a, [], LIST, [], [], undefined);
  expect(ids(section(pages[0], "Abilities"))).toEqual(["fire", "slash", "frost", "rage", "jab"]);
});

// VTT-840
test("a character holding nothing on the undeclared page gets one page with no section", () => {
  const pages = sheetFor(actor(), [], LIST, PROPS, [], undefined);
  expect(pages).toHaveLength(1);
  expect(pages[0]!.sections).toEqual([]);
});
