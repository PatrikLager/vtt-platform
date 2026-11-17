import "./support/dom";
import { test, expect } from "bun:test";
import { renderPlayerPanel, type PanelLayout, type PlayerUIState, type Switcher } from "../src/view/player";
import { newState, type State } from "../src/state";
import type { Ability, ConditionDef, Me, Property, SheetPage } from "../src/metadata";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { create } from "@bufbuild/protobuf";
import { CommandResultSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";

const ME: Me = { participantId: "p-me", name: "Me", role: "player" };
const DM: Me = { participantId: "p-me", name: "Dee", role: "dm" };

function table(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 6, GridHeight: 4 };
  for (const [id, name, x] of [["a1", "Lera", 1], ["a2", "Bram", 3]] as const) {
    st.Actors[id] = {
      actorId: id, name, moduleId: "", attributes: { wit: 4, grit: 2 },
      resources: { vigor: { current: 3, max: 10 }, sparks: { current: 1, max: 2 } },
      controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.PARTY_MEMBER, abilityIds: ["jab", "flare", "rally"],
    };
    st.Tokens[`t-${id}`] = { ID: `t-${id}`, SceneID: "s1", ActorID: id, X: x, Y: 1 };
  }
  st.Conditions["a1"] = [{ ID: "glum", Source: "dm", AppliedSeq: 1 }, { ID: "dazed", Source: "dm", AppliedSeq: 2 }];
  return st;
}

const PROPS: Property[] = [
  { key: "vigor", type: "resource", name: "Vigor", description: "How much fight is left." },
  { key: "grit", type: "number", name: "Grit", description: "How long one holds on." },
  { key: "sparks", type: "resource", name: "Sparks", description: "What a flare spends." },
  { key: "wit", type: "number", name: "Wit", description: "How quick one is." },
];
const CONDS: ConditionDef[] = [{ id: "dazed", name: "Dazed", description: "Slow to act." }];
const ABILITIES: Ability[] = [
  { id: "rally", name: "Rally", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
  { id: "flare", name: "Flare", range: 3, maxTargets: 1, usage: { kind: "resource", resource: "sparks", cost: 1 } },
  { id: "jab", name: "", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
];
const SHEET: SheetPage[] = [
  { name: "Front", sections: [
    { name: "Life", kind: "values", values: ["vigor", "sparks"] },
    { name: "Marks", kind: "conditions" },
    { name: "Moves", kind: "abilities", abilities: [{ kind: "atWill" }] },
    { name: "Costly", kind: "abilities", abilities: [{ kind: "resource", resource: "sparks" }] },
  ] },
  { name: "Back", sections: [{ name: "Numbers", kind: "values", values: ["wit", "grit"] }] },
];
const SWITCH: Switcher = { actors: [{ actorId: "a1", name: "Lera" }, { actorId: "a2", name: "Bram" }], onSwitch: () => {} };

function draw(o: { st?: State; me?: Me; ui?: Partial<PlayerUIState>; layout?: PanelLayout; abilities?: Ability[]; switcher?: Switcher | null } = {}) {
  const ui: PlayerUIState = { selectedActorId: "a1", selectedAbilityId: "", doorsArmed: false, ...o.ui };
  const st = o.st ?? table();
  const me = o.me ?? ME;
  const switcher = o.switcher === null ? undefined : (o.switcher ?? (me.role === "player" ? SWITCH : undefined));
  let node!: HTMLElement;
  const paint = () => {
    node = renderPlayerPanel(st, me, o.abilities ?? ABILITIES, ui, async () => create(CommandResultSchema, { ok: true }),
      paint, switcher, undefined, PROPS, o.layout ?? { sheet: SHEET, conditions: CONDS });
  };
  paint();
  return {
    ui,
    node: () => node,
    tabs: () => Array.from(node.querySelectorAll(".tabs .tab"), (b) => [b.textContent, b.classList.contains("sel")]),
    h3: () => Array.from(node.querySelectorAll(".sheet h3"), (h) => h.textContent),
    click: (label: string) => (Array.from(node.querySelectorAll("button")).find((b) => b.textContent === label) as HTMLButtonElement).click(),
    repaint: paint,
  };
}

// VTT-836
test("the panel is headed by the name of the character it shows", () => {
  expect(Array.from(draw().node().querySelectorAll("h2"), (h) => h.textContent)).toEqual(["Lera"]);
  expect(Array.from(draw({ ui: { selectedActorId: "a2" } }).node().querySelectorAll("h2"), (h) => h.textContent)).toEqual(["Bram"]);
  const dm = draw({ me: DM, ui: { selectedActorId: "" } });
  expect(Array.from(dm.node().querySelectorAll("h2"), (h) => h.textContent)).toEqual(["Lera"]);
});

// VTT-836
test("a panel showing no character draws no heading", () => {
  const st = table();
  for (const a of Object.values(st.Actors)) { a.controllerIds = []; a.controllerId = ""; }
  const p = draw({ st, me: DM, ui: { selectedActorId: "" } });
  expect(p.node().querySelectorAll("h2")).toHaveLength(0);
  expect(p.node().querySelector(".empty")?.textContent).toBe("You do not control an actor yet.");
});

// VTT-837
test("a sheet of several pages draws one tab per page in order and shows the first page", () => {
  const p = draw();
  expect(p.tabs()).toEqual([["Front", true], ["Back", false]]);
  expect(p.h3()).toEqual(["Life", "Marks", "Moves", "Costly"]);
  p.click("Back");
  expect(p.tabs()).toEqual([["Front", false], ["Back", true]]);
  expect(p.h3()).toEqual(["Numbers"]);
});

// VTT-837
test("a sheet of one page draws no tab", () => {
  const p = draw({ layout: { sheet: [SHEET[1]!], conditions: CONDS } });
  expect(p.node().querySelector(".tabs")).toBeNull();
  expect(p.h3()).toEqual(["Numbers"]);
});

// VTT-838
test("the page shown survives a repaint and a switch of character, and falls back to the first", () => {
  const p = draw();
  p.click("Back");
  p.repaint();
  expect(p.h3()).toEqual(["Numbers"]);
  p.ui.selectedActorId = "a2";
  p.repaint();
  expect(p.h3()).toEqual(["Numbers"]);
  const gone = draw({ ui: { page: "Gone" } });
  expect(gone.tabs()).toEqual([["Front", true], ["Back", false]]);
  expect(gone.h3()).toEqual(["Life", "Marks", "Moves", "Costly"]);
});

// VTT-838
test("a DM's switch of character by its chip keeps the page shown", () => {
  const p = draw({ me: DM, ui: { selectedActorId: "" } });
  p.click("Back");
  p.click("Bram");
  expect(Array.from(p.node().querySelectorAll("h2"), (h) => h.textContent)).toEqual(["Bram"]);
  expect(p.h3()).toEqual(["Numbers"]);
});

// VTT-836
test("a selection no character answers to draws no heading, and one no chip names is headed by its name", () => {
  const gone = draw({ me: DM, ui: { selectedActorId: "a9" } });
  expect(gone.node().querySelectorAll("h2")).toHaveLength(0);
  const st = table();
  st.Actors["a2"]!.controllerIds = [];
  st.Actors["a2"]!.controllerId = "";
  const unchipped = draw({ st, me: DM, ui: { selectedActorId: "a2" } });
  expect(Array.from(unchipped.node().querySelectorAll("h2"), (h) => h.textContent)).toEqual(["Bram"]);
});

// VTT-839 VTT-841
test("a page draws its sections in order, headed by name, values in the section's order", () => {
  const p = draw({ ui: { page: "Back" } });
  const values = Array.from(p.node().querySelectorAll(".value"), (v) => [v.textContent, v.getAttribute("title")]);
  expect(values).toEqual([["Wit 4", "How quick one is."], ["Grit 2", "How long one holds on."]]);
  const front = draw();
  expect(Array.from(front.node().querySelectorAll(".value"), (v) => v.textContent)).toEqual(["Vigor 3/10", "Sparks 1/2"]);
});

// VTT-839
test("a section with nothing to show for the character draws no heading", () => {
  const st = table();
  st.Actors["a1"]!.resources = {};
  st.Conditions["a1"] = [];
  expect(draw({ st }).h3()).toEqual(["Moves", "Costly"]);
});

// VTT-840
test("with no sheet declared the panel draws no tab and one page of Values, Conditions and Abilities", () => {
  for (const sheet of [undefined, []]) {
    const p = draw({ layout: { sheet, conditions: CONDS } });
    expect(p.node().querySelector(".tabs")).toBeNull();
    expect(p.h3()).toEqual(["Values", "Conditions", "Abilities"]);
    expect(Array.from(p.node().querySelectorAll(".value"), (v) => v.textContent))
      .toEqual(["Vigor 3/10", "Grit 2", "Sparks 1/2", "Wit 4"]);
  }
});

// VTT-841
test("a value whose key the ruleset does not declare is named by its key, untitled", () => {
  const st = table();
  st.Actors["a1"]!.attributes = { nerve: 5 };
  const p = draw({ st, layout: { conditions: CONDS } });
  const v = Array.from(p.node().querySelectorAll(".value")).find((x) => x.textContent === "nerve 5");
  expect(v).toBeDefined();
  expect(v!.hasAttribute("title") ? v!.getAttribute("title") : "").toBe("");
});

// VTT-842
test("conditions are drawn in the order applied, by name and description, else by id", () => {
  const cs = Array.from(draw().node().querySelectorAll(".condition"), (c) => [c.textContent, c.getAttribute("title") ?? ""]);
  expect(cs).toEqual([["glum", ""], ["Dazed", "Slow to act."]]);
});

// VTT-843 VTT-846
test("each offered ability is one button in the section of its usage, labelled by its name, else its id", () => {
  const p = draw();
  const rowAfter = (heading: string) => {
    const h = Array.from(p.node().querySelectorAll("h3")).find((x) => x.textContent === heading)!;
    return Array.from(h.nextElementSibling!.querySelectorAll("button"), (b) => b.textContent);
  };
  expect(rowAfter("Moves")).toEqual(["Rally", "jab"]);
  expect(rowAfter("Costly")).toEqual(["Flare"]);
  expect(Array.from(p.node().querySelectorAll("button")).filter((b) => b.textContent === "Flare")).toHaveLength(1);
});

// VTT-844
test("a resource ability's button names the resource, its current and max, and the cost", () => {
  const btn = (st: State) => Array.from(draw({ st }).node().querySelectorAll("button")).find((b) => b.textContent === "Flare")!;
  expect(btn(table()).title).toBe("Sparks 1/2, costs 1");
  const spent = table();
  spent.Actors["a1"]!.resources["sparks"] = { current: 0, max: 2 };
  expect(btn(spent).title).toBe("Sparks 0/2, costs 1 — not enough");
  const none = table();
  delete none.Actors["a1"]!.resources["sparks"];
  expect(btn(none).title).toBe("Sparks, costs 1 — not enough");
  expect(Array.from(draw().node().querySelectorAll("button")).find((b) => b.textContent === "Rally")!.title).toBe("at will");
});

// VTT-837 VTT-839
test("an ability armed on one page keeps its targets drawn on another", () => {
  const p = draw();
  p.click("Rally");
  p.click("Back");
  expect(p.h3()).toEqual(["Numbers"]);
  expect(Array.from(p.node().querySelectorAll("h3"), (h) => h.textContent)).toContain("Targets (range 1)");
  expect(p.node().querySelector('[data-action="use-ability"]')).not.toBeNull();
});
