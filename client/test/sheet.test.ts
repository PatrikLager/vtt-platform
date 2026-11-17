import { test, expect } from "bun:test";
import { sheetFor, type ShownPage } from "../src/player";
import type { Actor, ActorCondition } from "../src/state";
import type { Ability, ConditionDef, Property, SheetPage } from "../src/metadata";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

const actor = (over: Partial<Actor> = {}): Actor => ({
  actorId: "a1", name: "Lera", moduleId: "", controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER,
  attributes: { wit: 4, grit: 2 },
  resources: { vigor: { current: 3, max: 10 }, sparks: { current: 1, max: 2 } },
  abilityIds: ["jab", "flare", "rally"],
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

// VTT-840
test("with no sheet declared, one page of Values, Conditions and Abilities", () => {
  for (const sheet of [undefined, []]) {
    const pages = sheetFor(actor(), applied("dazed"), ABILITIES, PROPS, CONDS, sheet);
    expect(outline(pages)).toEqual([["", [["Values", "values"], ["Conditions", "conditions"], ["Abilities", "abilities"]]]]);
  }
});

// VTT-840
test("the undeclared page leaves out each section with nothing to show", () => {
  const pages = sheetFor(actor({ abilityIds: [] }), [], ABILITIES, PROPS, CONDS, undefined);
  expect(outline(pages)).toEqual([["", [["Values", "values"]]]]);
  expect(outline(sheetFor(actor({ attributes: {}, resources: {}, abilityIds: [] }), [], ABILITIES, PROPS, CONDS, undefined)))
    .toEqual([["", []]]);
});

// VTT-840
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
