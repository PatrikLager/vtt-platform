import { test, expect } from "bun:test";
import { sheetFor, type ShownPage, type ShownSection } from "../src/player";
import type { Actor, ActorCondition, Reference } from "../src/state";
import type { Ability, CatalogueEntry, ConditionDef, PlaceDef, Property, SheetPage } from "../src/metadata";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

const actor = (over: Partial<Actor> = {}): Actor => ({
  actorId: "a1", name: "Lera", moduleId: "", controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER,
  attributes: { wit: 4, grit: 2 },
  resources: { vigor: { current: 3, max: 10 }, sparks: { current: 1, max: 2 } },
  abilityIds: ["jab", "flare", "rally"], references: {},
  ...over,
});

const PROPS: Property[] = [
  { key: "vigor", type: "resource", name: "Vigor", description: "How much fight is left." },
  { key: "grit", type: "number", name: "Grit", description: "How long one holds on." },
  { key: "sparks", type: "resource", name: "Sparks", description: "What a flare spends." },
  { key: "wit", type: "number", name: "Wit", description: "How quick one is." },
];

const CONDS: ConditionDef[] = [
  { id: "dazed", name: "Dazed", description: "Slow to act." },
  { id: "braced", name: "Braced", description: "Set against a push." },
];

const ABILITIES: Ability[] = [
  { id: "rally", name: "Rally", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
  { id: "flare", name: "Flare", range: 3, maxTargets: 1, usage: { kind: "resource", resource: "sparks", cost: 1 } },
  { id: "jab", name: "Jab", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
  { id: "hidden", name: "Hidden", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
];

const applied = (...ids: string[]): ActorCondition[] => ids.map((ID, i) => ({ ID, Source: "dm", AppliedSeq: i + 1 }));

const SHEET: SheetPage[] = [
  { name: "Front", sections: [
    { name: "Life", kind: "values", values: ["vigor"] },
    { name: "Marks", kind: "conditions" },
    { name: "Moves", kind: "abilities", abilities: [{ kind: "atWill" }] },
    { name: "Costly", kind: "abilities", abilities: [{ kind: "resource", resource: "sparks" }] },
  ] },
  { name: "Back", sections: [
    { name: "Numbers", kind: "values", values: ["wit", "grit"] },
    { name: "Spent", kind: "values", values: ["sparks"] },
  ] },
];

const outline = (pages: ShownPage[]) => pages.map((p) => [p.name, p.sections.map((s) => [s.name, s.kind])]);

// VTT-839
test("a declared sheet keeps its pages and sections in order and invents none", () => {
  const pages = sheetFor(actor(), applied("dazed"), ABILITIES, PROPS, CONDS, SHEET);
  expect(outline(pages)).toEqual([
    ["Front", [["Life", "values"], ["Marks", "conditions"], ["Moves", "abilities"], ["Costly", "abilities"]]],
    ["Back", [["Numbers", "values"], ["Spent", "values"]]],
  ]);
});

// VTT-839
test("a section's values come in the order the section names them", () => {
  const back = sheetFor(actor(), [], ABILITIES, PROPS, CONDS, SHEET)[1]!;
  const numbers = back.sections[0]!;
  expect(numbers.kind === "values" && numbers.values.map((v) => v.key)).toEqual(["wit", "grit"]);
});

// VTT-839
test("a section with nothing to show for the character is left out, and its page stays", () => {
  const pages = sheetFor(actor({ resources: {}, abilityIds: ["jab"] }), [], ABILITIES, PROPS, CONDS, SHEET);
  expect(outline(pages)).toEqual([
    ["Front", [["Moves", "abilities"]]],
    ["Back", [["Numbers", "values"]]],
  ]);
  const bare = sheetFor(actor({ resources: {}, attributes: {}, abilityIds: [] }), [], ABILITIES, PROPS, CONDS, SHEET);
  expect(outline(bare)).toEqual([["Front", []], ["Back", []]]);
});

// VTT-1056
test("with no sheet declared, one page of Values, Conditions and Abilities", () => {
  for (const sheet of [undefined, []]) {
    const pages = sheetFor(actor(), applied("dazed"), ABILITIES, PROPS, CONDS, sheet);
    expect(outline(pages)).toEqual([["", [["Values", "values"], ["Conditions", "conditions"], ["Abilities", "abilities"]]]]);
  }
});

// VTT-1056
test("the undeclared page leaves out each section with nothing to show", () => {
  const pages = sheetFor(actor({ abilityIds: [] }), [], ABILITIES, PROPS, CONDS, undefined);
  expect(outline(pages)).toEqual([["", [["Values", "values"]]]]);
  expect(outline(sheetFor(actor({ attributes: {}, resources: {}, abilityIds: [] }), [], ABILITIES, PROPS, CONDS, undefined)))
    .toEqual([["", []]]);
});

// VTT-1056
test("the undeclared page's values follow the properties' order, then undeclared keys by key", () => {
  const a = actor({
    attributes: { zeal: 1, wit: 4, grit: 2, aim: 7 },
    resources: { vigor: { current: 3, max: 10 }, aim: { current: 1, max: 1 }, sparks: { current: 1, max: 2 } },
  });
  const values = sheetFor(a, [], ABILITIES, PROPS, CONDS, undefined)[0]!.sections[0]!;
  expect(values.kind === "values" && values.values.map((v) => [v.key, v.text])).toEqual([
    ["vigor", "3/10"], ["grit", "2"], ["sparks", "1/2"], ["wit", "4"], ["aim", "7"], ["aim", "1/1"], ["zeal", "1"],
  ]);
});

// VTT-1056 VTT-841
test("with no properties, the undeclared page draws each key the actor holds once, its own name included", () => {
  const a = actor({ attributes: {}, resources: { undefined: { current: 1, max: 2 }, vigor: { current: 3, max: 10 } } });
  const values = sheetFor(a, [], ABILITIES, undefined, CONDS, undefined)[0]!.sections[0]!;
  expect(values.kind === "values" && values.values).toEqual([
    { key: "undefined", name: "undefined", text: "1/2", title: "" },
    { key: "vigor", name: "vigor", text: "3/10", title: "" },
  ]);
});

// VTT-841
test("a value reads as its property's name and value, titled by its description", () => {
  const front = sheetFor(actor(), [], ABILITIES, PROPS, CONDS, SHEET)[0]!;
  const life = front.sections[0]!;
  expect(life.kind === "values" && life.values).toEqual([{ key: "vigor", name: "Vigor", text: "3/10", title: "How much fight is left." }]);
  const back = sheetFor(actor(), [], ABILITIES, PROPS, CONDS, SHEET)[1]!;
  const numbers = back.sections[0]!;
  expect(numbers.kind === "values" && numbers.values[0]).toEqual({ key: "wit", name: "Wit", text: "4", title: "How quick one is." });
});

// VTT-841
test("a key the ruleset does not declare as that type is named by its key, with no title", () => {
  const props: Property[] = [{ key: "wit", type: "resource", name: "Wit Pool", description: "Not a number." }];
  for (const p of [props, undefined]) {
    const values = sheetFor(actor({ resources: {} }), [], [], p, CONDS, undefined)[0]!.sections[0]!;
    expect(values.kind === "values" && values.values).toEqual([
      { key: "grit", name: "grit", text: "2", title: "" },
      { key: "wit", name: "wit", text: "4", title: "" },
    ]);
  }
});

// VTT-841
test("a key the character's sheet does not hold draws nothing", () => {
  const front = sheetFor(actor({ resources: { sparks: { current: 1, max: 2 } } }), [], ABILITIES, PROPS, CONDS, SHEET)[0]!;
  expect(front.sections.map((s) => s.name)).toEqual(["Moves", "Costly"]);
});

// VTT-842
test("conditions come in the order applied, named and titled by the ruleset, else by id", () => {
  const front = sheetFor(actor(), applied("glum", "dazed", "braced"), ABILITIES, PROPS, CONDS, SHEET)[0]!;
  const marks = front.sections[1]!;
  expect(marks.kind === "conditions" && marks.conditions).toEqual([
    { id: "glum", name: "glum", title: "" },
    { id: "dazed", name: "Dazed", title: "Slow to act." },
    { id: "braced", name: "Braced", title: "Set against a push." },
  ]);
});

// VTT-843
test("each offered ability stands in the one section whose usages name its usage, in the list's order", () => {
  const front = sheetFor(actor(), [], ABILITIES, PROPS, CONDS, SHEET)[0]!;
  const ids = (name: string) => {
    const s = front.sections.find((x) => x.name === name)!;
    return s.kind === "abilities" ? s.abilities.map((a) => a.id) : [];
  };
  expect(ids("Moves")).toEqual(["rally", "jab"]);
  expect(ids("Costly")).toEqual(["flare"]);
});

// VTT-843
test("an ability used at will stands in no section of a resource, whatever its usage carries", () => {
  const odd: Ability[] = [{ id: "jab", name: "Jab", range: 1, maxTargets: 1, usage: { kind: "atWill", resource: "sparks" } }];
  const front = sheetFor(actor(), [], odd, PROPS, CONDS, SHEET)[0]!;
  expect(front.sections.filter((s) => s.kind === "abilities").map((s) => s.name)).toEqual(["Moves"]);
});

const CATALOGUE: CatalogueEntry[] = [
  { id: "echo", name: "Echo Blade", description: "Rings when drawn.", form: "zulu", values: {} },
  { id: "delta", name: "Delta Mail", description: "", form: "yankee", values: {} },
];

// Mike before kilo, so the window's order is not the keys' order.
const PLACES: PlaceDef[] = [
  { key: "mike", name: "Mike Rack", description: "Holds a few.", forms: ["zulu", "yankee"], copies: true, states: ["lima", "oscar"] },
  { key: "kilo", name: "Kilo Hook", description: "Holds one.", forms: ["yankee"], copies: false, states: [] },
];

// Written out of id order, so an order by id is the sheet's and not the map's.
const CARRIED: Record<string, Reference> = {
  "r-c": { entryId: "delta", place: "mike", state: "oscar" },
  "r-a": { entryId: "delta", place: "kilo", state: "" },
  "r-b": { entryId: "echo", place: "mike", state: "lima" },
};

const refIds = (s: ShownSection | undefined) => (s?.kind === "places" ? s.references.map((r) => r.id) : undefined);

// VTT-1055
test("a section of places draws the references in the order it lists its places, then by reference id", () => {
  const sheet: SheetPage[] = [{ name: "Front", sections: [{ name: "Gear", kind: "places", places: ["mike", "kilo"] }] }];
  const front = sheetFor(actor({ references: CARRIED }), [], ABILITIES, PROPS, CONDS, sheet, CATALOGUE, PLACES)[0]!;
  expect(outline([front])).toEqual([["Front", [["Gear", "places"]]]]);
  expect(refIds(front.sections[0])).toEqual(["r-b", "r-c", "r-a"]);
});

// VTT-1055
test("a section of places draws only the references held in the places it lists", () => {
  const sheet: SheetPage[] = [
    { name: "Front", sections: [{ name: "Rack", kind: "places", places: ["mike"] }] },
    { name: "Back", sections: [{ name: "Hook", kind: "places", places: ["kilo"] }] },
  ];
  const [front, back] = sheetFor(actor({ references: CARRIED }), [], ABILITIES, PROPS, CONDS, sheet, CATALOGUE, PLACES);
  expect(refIds(front!.sections[0])).toEqual(["r-b", "r-c"]);
  expect(refIds(back!.sections[0])).toEqual(["r-a"]);
});

// VTT-1055
test("a reference is named by its entry's name, else its entry's id, with its state, titled by the entry's description", () => {
  const sheet: SheetPage[] = [{ name: "Front", sections: [{ name: "Gear", kind: "places", places: ["mike", "kilo"] }] }];
  const shown = (catalogue: CatalogueEntry[] | undefined) => {
    const s = sheetFor(actor({ references: CARRIED }), [], ABILITIES, PROPS, CONDS, sheet, catalogue, PLACES)[0]!.sections[0];
    return s?.kind === "places" ? s.references : undefined;
  };
  expect(shown(CATALOGUE)).toEqual([
    { id: "r-b", name: "Echo Blade", place: "Mike Rack", state: "lima", title: "Rings when drawn." },
    { id: "r-c", name: "Delta Mail", place: "Mike Rack", state: "oscar", title: "" },
    { id: "r-a", name: "Delta Mail", place: "Kilo Hook", state: "", title: "" },
  ]);
  expect(shown([CATALOGUE[0]!])?.map((r) => r.name)).toEqual(["Echo Blade", "delta", "delta"]);
  expect(shown(undefined)?.map((r) => [r.name, r.title])).toEqual([["echo", ""], ["delta", ""], ["delta", ""]]);
});

// VTT-1055 VTT-839
test("a section of places holding none of the character's references is left out, and its page stays", () => {
  const sheet: SheetPage[] = [{ name: "Front", sections: [
    { name: "Life", kind: "values", values: ["vigor"] },
    { name: "Hook", kind: "places", places: ["kilo"] },
  ] }];
  const inMike = { "r-b": CARRIED["r-b"]! };
  for (const references of [inMike, {}]) {
    expect(outline(sheetFor(actor({ references }), [], ABILITIES, PROPS, CONDS, sheet, CATALOGUE, PLACES)))
      .toEqual([["Front", [["Life", "values"]]]]);
  }
});

// VTT-1056
test("with no sheet declared, a section per place holding a reference follows Abilities, in the window's order of places", () => {
  const references = { "r-a": CARRIED["r-a"]!, "r-b": CARRIED["r-b"]! };
  for (const sheet of [undefined, []]) {
    const pages = sheetFor(actor({ references }), applied("dazed"), ABILITIES, PROPS, CONDS, sheet, CATALOGUE, PLACES);
    expect(outline(pages)).toEqual([["", [
      ["Values", "values"], ["Conditions", "conditions"], ["Abilities", "abilities"],
      ["Mike Rack", "places"], ["Kilo Hook", "places"],
    ]]]);
  }
});

// VTT-1056
test("with no sheet declared, a place the window does not hold follows those it holds, by key, named by its key", () => {
  const references = { "r-a": { ...CARRIED["r-a"]!, place: "mike" }, "r-b": { ...CARRIED["r-b"]!, place: "kilo" } };
  const names = (places: PlaceDef[] | undefined) =>
    sheetFor(actor({ references }), [], [], PROPS, CONDS, undefined, CATALOGUE, places)[0]!.sections.map((s) => s.name);
  expect(names([PLACES[1]!])).toEqual(["Values", "Kilo Hook", "mike"]);
  for (const places of [undefined, []]) expect(names(places)).toEqual(["Values", "kilo", "mike"]);
});

// VTT-1056
test("with no sheet declared, a place's section holds its references by reference id, named and titled as declared", () => {
  const pages = sheetFor(actor({ references: CARRIED }), [], [], PROPS, CONDS, undefined, CATALOGUE, PLACES);
  const mike = pages[0]!.sections.find((s) => s.name === "Mike Rack");
  expect(mike?.kind === "places" && mike.references).toEqual([
    { id: "r-b", name: "Echo Blade", place: "", state: "lima", title: "Rings when drawn." },
    { id: "r-c", name: "Delta Mail", place: "", state: "oscar", title: "" },
  ]);
});

// VTT-1056
test("with no sheet declared, a character holding no reference is drawn no place's section", () => {
  const pages = sheetFor(actor(), applied("dazed"), ABILITIES, PROPS, CONDS, undefined, CATALOGUE, PLACES);
  expect(outline(pages)).toEqual([["", [["Values", "values"], ["Conditions", "conditions"], ["Abilities", "abilities"]]]]);
});
