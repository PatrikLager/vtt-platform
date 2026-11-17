import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { CommandResultSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { boot } from "../src/app";
import type { Ability, ConditionDef, Me, Property, SheetPage } from "../src/metadata";
import { newState, type Actor, type ActorCondition } from "../src/state";
import { renderPlayerPanel, type PanelLayout, type PlayerUIState } from "../src/view/player";

const ok = async () => create(CommandResultSchema, { ok: true });

function actor(id: string, name: string, o: Partial<Actor> = {}): Actor {
  return {
    actorId: id,
    name,
    moduleId: "",
    attributes: {},
    resources: {},
    controllerId: "p1",
    controllerIds: ["p1"],
    kind: ActorKind.PARTY_MEMBER,
    abilityIds: [],
    ...o,
  };
}

function prop(key: string, type: "number" | "resource", name: string, description = ""): Property {
  return { key, type, name, description };
}

function ab(id: string, name: string, usage: Ability["usage"]): Ability {
  return { id, name, range: 5, maxTargets: 1, usage };
}

class Rig {
  st = newState();
  ui: PlayerUIState = { selectedActorId: "a1", selectedAbilityId: "", doorsArmed: false };
  abilities: Ability[] = [];
  properties: Property[] | undefined;
  layout: PanelLayout | undefined;
  me: Me = { participantId: "p1", name: "P", role: "player" };
  chips: { actorId: string; name: string }[] | undefined;
  repaints = 0;
  constructor(...actors: Actor[]) {
    for (const a of actors) this.st.Actors[a.actorId] = a;
    this.chips = actors.map((a) => ({ actorId: a.actorId, name: a.name }));
  }
  conditions(actorId: string, ids: string[]): void {
    const list: ActorCondition[] = ids.map((id, i) => ({ ID: id, Source: "", AppliedSeq: i + 1 }));
    this.st.Conditions[actorId] = list;
  }
  draw(): HTMLElement {
    const chips = this.chips;
    return renderPlayerPanel(
      this.st,
      this.me,
      this.abilities,
      this.ui,
      ok,
      () => {
        this.repaints++;
      },
      chips ? { actors: chips, onSwitch: () => {} } : undefined,
      undefined,
      this.properties,
      this.layout,
    );
  }
}

interface Group {
  name: string;
  nodes: Element[];
}

function groups(el: HTMLElement): Group[] {
  const sheet = el.querySelector(".sheet");
  const out: Group[] = [];
  if (!sheet) return out;
  for (const c of Array.from(sheet.children)) {
    if (c.tagName === "H3") out.push({ name: c.textContent ?? "", nodes: [] });
    else out[out.length - 1]?.nodes.push(c);
  }
  return out;
}

function within(g: Group | undefined, sel: string): HTMLElement[] {
  const found: HTMLElement[] = [];
  for (const n of g?.nodes ?? []) {
    if (n.matches(sel) && n instanceof HTMLElement) found.push(n);
    for (const d of Array.from(n.querySelectorAll(sel))) {
      if (d instanceof HTMLElement) found.push(d);
    }
  }
  return found;
}

const names = (el: HTMLElement): string[] => groups(el).map((g) => g.name);
const texts = (els: Element[]): string[] => els.map((e) => e.textContent ?? "");
const section = (el: HTMLElement, name: string): Group | undefined =>
  groups(el).find((g) => g.name === name);
const tabs = (el: HTMLElement): HTMLElement[] =>
  Array.from(el.querySelectorAll(".tabs .tab")).filter((e): e is HTMLElement => e instanceof HTMLElement);
const shownTab = (el: HTMLElement): string[] =>
  texts(tabs(el).filter((t) => t.classList.contains("sel")));

function valuesSection(name: string, values: string[]): PanelLayout["sheet"] {
  return [{ name: "P", sections: [{ name, kind: "values", values }] }];
}

// Three pages and three sections, none in name order.
const PAGES: SheetPage[] = [
  {
    name: "Zeta",
    sections: [
      { name: "Zed", kind: "values", values: ["str"] },
      { name: "Alpha", kind: "values", values: ["hp"] },
    ],
  },
  { name: "Alpha", sections: [{ name: "Only alpha", kind: "values", values: ["hp"] }] },
  { name: "Mid", sections: [{ name: "Only mid", kind: "values", values: ["str"] }] },
];

const STATS = [prop("str", "number", "Strength", "melee"), prop("hp", "resource", "Health", "life")];

function stdRig(): Rig {
  const r = new Rig(actor("a1", "Ann", { attributes: { str: 3 }, resources: { hp: { current: 4, max: 9 } } }));
  r.properties = STATS;
  r.layout = { sheet: PAGES };
  return r;
}

afterEach(() => {
  document.body.innerHTML = "";
});

// VTT-836
test("heading is the selected chip's label, the id when it is empty", () => {
  const r = new Rig(actor("a1", "Fold name"));
  r.chips = [
    { actorId: "a1", name: "Chip name" },
    { actorId: "a2", name: "Other" },
  ];
  expect(r.draw().querySelector("h2")?.textContent).toBe("Chip name");
  r.chips = [{ actorId: "a1", name: "" }];
  expect(r.draw().querySelector("h2")?.textContent).toBe("a1");
});

// VTT-836
test("a DM's panel without a switcher is headed by the actor's name", () => {
  const r = new Rig(actor("a1", "Ann", { controllerId: "d", controllerIds: ["d"] }));
  r.me = { participantId: "d", name: "D", role: "dm" };
  r.chips = undefined;
  expect(r.draw().querySelector("h2")?.textContent).toBe("Ann");
});

// VTT-836
test("with no character the panel draws no heading", () => {
  const r = new Rig();
  r.chips = [];
  const el = r.draw();
  expect(el.querySelector("h2")).toBeNull();
  expect(el.textContent).toContain("You do not control an actor yet.");
});

// VTT-837
test("several pages draw one tab each, in declared order, first shown", () => {
  const r = stdRig();
  const el = r.draw();
  expect(texts(tabs(el))).toEqual(["Zeta", "Alpha", "Mid"]);
  expect(shownTab(el)).toEqual(["Zeta"]);
  expect(names(el)).toEqual(["Zed", "Alpha"]);
});

// VTT-837
test("a click on a tab shows its page after the repaint", () => {
  const r = stdRig();
  tabs(r.draw())[2]?.click();
  expect(r.repaints).toBeGreaterThan(0);
  const el = r.draw();
  expect(shownTab(el)).toEqual(["Mid"]);
  expect(names(el)).toEqual(["Only mid"]);
  tabs(el)[1]?.click();
  const el2 = r.draw();
  expect(shownTab(el2)).toEqual(["Alpha"]);
  expect(names(el2)).toEqual(["Only alpha"]);
});

// VTT-837
test("one page, an empty sheet and no sheet draw no tab", () => {
  const r = stdRig();
  r.layout = { sheet: [PAGES[0] as SheetPage] };
  expect(tabs(r.draw())).toHaveLength(0);
  expect(names(r.draw())).toEqual(["Zed", "Alpha"]);
  r.layout = { sheet: [] };
  expect(tabs(r.draw())).toHaveLength(0);
  r.layout = {};
  expect(tabs(r.draw())).toHaveLength(0);
  r.layout = undefined;
  expect(tabs(r.draw())).toHaveLength(0);
  expect(r.draw().querySelector(".tabs")).toBeNull();
});

// VTT-838
test("the page is kept across a repaint and a switch of character", () => {
  const r = stdRig();
  r.st.Actors["a2"] = actor("a2", "Bo", { attributes: { str: 7 } });
  r.chips = [
    { actorId: "a1", name: "Ann" },
    { actorId: "a2", name: "Bo" },
  ];
  tabs(r.draw())[2]?.click();
  expect(shownTab(r.draw())).toEqual(["Mid"]);
  expect(shownTab(r.draw())).toEqual(["Mid"]);
  r.ui.selectedActorId = "a2";
  const el = r.draw();
  expect(el.querySelector("h2")?.textContent).toBe("Bo");
  expect(shownTab(el)).toEqual(["Mid"]);
  expect(texts(within(section(el, "Only mid"), ".value"))).toEqual(["Strength 7"]);
});

// VTT-838
test("a page the sheet no longer declares shows the first page", () => {
  const r = stdRig();
  tabs(r.draw())[2]?.click();
  r.layout = { sheet: [PAGES[1] as SheetPage, PAGES[0] as SheetPage] };
  const el = r.draw();
  expect(shownTab(el)).toEqual(["Alpha"]);
  expect(names(el)).toEqual(["Only alpha"]);
});

// VTT-839
test("sections of the shown page only, in order, each headed by its name", () => {
  const r = stdRig();
  expect(names(r.draw())).toEqual(["Zed", "Alpha"]);
  expect(texts(within(section(r.draw(), "Zed"), ".value"))).toEqual(["Strength 3"]);
  expect(texts(within(section(r.draw(), "Alpha"), ".value"))).toEqual(["Health 4/9"]);
});

// VTT-839
test("values follow the section's order, not the properties'", () => {
  const r = stdRig();
  r.properties = [prop("a", "number", "A"), prop("b", "number", "B"), prop("c", "number", "C")];
  r.st.Actors["a1"] = actor("a1", "Ann", { attributes: { a: 1, b: 2, c: 3 } });
  r.layout = { sheet: valuesSection("S", ["c", "a", "b"]) };
  expect(texts(within(section(r.draw(), "S"), ".value"))).toEqual(["C 3", "A 1", "B 2"]);
});

// VTT-839
test("a section with nothing to show is left out with its heading", () => {
  const r = stdRig();
  r.layout = {
    sheet: [
      {
        name: "P",
        sections: [
          { name: "Missing", kind: "values", values: ["dex"] },
          { name: "Shown", kind: "values", values: ["dex", "str"] },
          { name: "Conds", kind: "conditions" },
          { name: "Acts", kind: "abilities", abilities: [{ kind: "atWill" }] },
        ],
      },
    ],
  };
  const el = r.draw();
  expect(names(el)).toEqual(["Shown"]);
  expect(texts(within(section(el, "Shown"), ".value"))).toEqual(["Strength 3"]);
  expect(el.textContent).not.toContain("Missing");
});

// VTT-840
test("no sheet: one page of Values, Conditions, Abilities, no tab", () => {
  const r = new Rig(actor("a1", "Ann", { attributes: { str: 3 }, abilityIds: ["x"] }));
  r.abilities = [ab("x", "Slash", { kind: "atWill" })];
  r.conditions("a1", ["prone"]);
  for (const layout of [undefined, {}, { sheet: [] }] as (PanelLayout | undefined)[]) {
    r.layout = layout;
    const el = r.draw();
    expect(names(el)).toEqual(["Values", "Conditions", "Abilities"]);
    expect(tabs(el)).toHaveLength(0);
    expect(texts(within(section(el, "Abilities"), "button"))).toEqual(["Slash"]);
  }
});

// VTT-840
test("no sheet: each section with nothing to show is left out", () => {
  const r = new Rig(actor("a1", "Ann"));
  expect(names(r.draw())).toEqual([]);
  r.conditions("a1", ["prone"]);
  expect(names(r.draw())).toEqual(["Conditions"]);
  r.st.Actors["a1"] = actor("a1", "Ann", { attributes: { str: 1 } });
  r.conditions("a1", []);
  expect(names(r.draw())).toEqual(["Values"]);
});

// VTT-840
test("no sheet: values in the properties' order", () => {
  const r = new Rig(
    actor("a1", "Ann", {
      attributes: { a: 1, z: 2 },
      resources: { m: { current: 1, max: 2 } },
    }),
  );
  r.properties = [prop("z", "number", "Zed"), prop("m", "resource", "Em"), prop("a", "number", "Ay")];
  expect(texts(within(section(r.draw(), "Values"), ".value"))).toEqual(["Zed 2", "Em 1/2", "Ay 1"]);
});

// VTT-840
test("no sheet: by key, an attribute before a resource of the same key", () => {
  const r = new Rig(
    actor("a1", "Ann", {
      attributes: { b: 1, a: 2, d: 6 },
      resources: { a: { current: 3, max: 4 }, c: { current: 5, max: 5 }, b: { current: 7, max: 8 } },
    }),
  );
  expect(texts(within(section(r.draw(), "Values"), ".value"))).toEqual([
    "a 2",
    "a 3/4",
    "b 1",
    "b 7/8",
    "c 5/5",
    "d 6",
  ]);
});

// VTT-841
test("a value is the display name and value, titled by the description", () => {
  const r = stdRig();
  const vs = within(section(r.draw(), "Zed"), ".value");
  expect(texts(vs)).toEqual(["Strength 3"]);
  expect(vs[0]?.title).toBe("melee");
  const hp = within(section(r.draw(), "Alpha"), ".value");
  expect(texts(hp)).toEqual(["Health 4/9"]);
  expect(hp[0]?.title).toBe("life");
});

// VTT-841
test("a key the ruleset does not declare as that type is named by its key", () => {
  const r = stdRig();
  r.properties = [prop("str", "resource", "Strength", "melee"), prop("hp", "number", "Health", "life")];
  const el = r.draw();
  const a = within(section(el, "Zed"), ".value");
  const b = within(section(el, "Alpha"), ".value");
  expect(texts(a)).toEqual(["str 3"]);
  expect(texts(b)).toEqual(["hp 4/9"]);
  expect(a[0]?.getAttribute("title") ?? "").toBe("");
  expect(b[0]?.getAttribute("title") ?? "").toBe("");
  r.properties = undefined;
  expect(texts(within(section(r.draw(), "Zed"), ".value"))).toEqual(["str 3"]);
  r.properties = [];
  expect(texts(within(section(r.draw(), "Zed"), ".value"))).toEqual(["str 3"]);
});

// VTT-842
test("conditions in the order applied, named and titled by the ruleset", () => {
  const r = stdRig();
  const defs: ConditionDef[] = [
    { id: "zap", name: "Zapped", description: "shocked" },
    { id: "bad", name: "Badly off", description: "poorly" },
    { id: "mid", name: "Middling", description: "so-so" },
  ];
  r.layout = { sheet: [{ name: "P", sections: [{ name: "C", kind: "conditions" }] }], conditions: defs };
  r.conditions("a1", ["mid", "zap", "bad"]);
  const cs = within(section(r.draw(), "C"), ".condition");
  expect(texts(cs)).toEqual(["Middling", "Zapped", "Badly off"]);
  expect(cs.map((c) => c.title)).toEqual(["so-so", "shocked", "poorly"]);
});

// VTT-842
test("an undeclared condition is named by its id with no title", () => {
  const r = stdRig();
  const sheet: SheetPage[] = [{ name: "P", sections: [{ name: "C", kind: "conditions" }] }];
  r.layout = { sheet, conditions: [{ id: "zap", name: "Zapped", description: "shocked" }] };
  r.conditions("a1", ["unknown-one", "zap"]);
  const cs = within(section(r.draw(), "C"), ".condition");
  expect(texts(cs)).toEqual(["unknown-one", "Zapped"]);
  expect(cs[0]?.getAttribute("title") ?? "").toBe("");
  r.layout = { sheet };
  const bare = within(section(r.draw(), "C"), ".condition");
  expect(texts(bare)).toEqual(["unknown-one", "zap"]);
});

const SPELLS: Ability[] = [
  ab("jab", "Jab", { kind: "atWill" }),
  ab("smash", "Smash", { kind: "resource", resource: "rage", cost: 1 }),
  ab("slash", "Slash", { kind: "atWill" }),
  ab("bolt", "Bolt", { kind: "resource", resource: "mana", cost: 2 }),
  ab("unknown", "Offsheet", { kind: "atWill" }),
];

function abilityRig(): Rig {
  const r = new Rig(
    actor("a1", "Ann", {
      resources: { mana: { current: 5, max: 8 }, rage: { current: 0, max: 3 } },
      abilityIds: ["slash", "bolt", "smash", "jab"],
    }),
  );
  r.abilities = SPELLS;
  r.properties = [prop("mana", "resource", "Mana"), prop("rage", "resource", "Rage")];
  r.layout = {
    sheet: [
      {
        name: "P",
        sections: [
          { name: "Spells", kind: "abilities", abilities: [{ kind: "resource", resource: "mana" }] },
          { name: "Fury", kind: "abilities", abilities: [{ kind: "resource", resource: "rage" }] },
          { name: "Free", kind: "abilities", abilities: [{ kind: "atWill" }] },
        ],
      },
    ],
  };
  return r;
}

// VTT-843
test("each offered ability is one button in the section naming its usage", () => {
  const el = abilityRig().draw();
  expect(names(el)).toEqual(["Spells", "Fury", "Free"]);
  expect(texts(within(section(el, "Spells"), "button"))).toEqual(["Bolt"]);
  expect(texts(within(section(el, "Fury"), "button"))).toEqual(["Smash"]);
  expect(texts(within(section(el, "Free"), "button"))).toEqual(["Jab", "Slash"]);
  expect(el.textContent).not.toContain("Offsheet");
});

// VTT-843
test("a section naming two usages holds both, in the window's list order", () => {
  const r = abilityRig();
  r.layout = {
    sheet: [
      {
        name: "P",
        sections: [
          {
            name: "All",
            kind: "abilities",
            abilities: [{ kind: "resource", resource: "mana" }, { kind: "atWill" }],
          },
        ],
      },
    ],
  };
  expect(texts(within(section(r.draw(), "All"), "button"))).toEqual(["Jab", "Slash", "Bolt"]);
});

// VTT-843
test("an ability the sheet does not name gets no button", () => {
  const r = abilityRig();
  r.st.Actors["a1"] = actor("a1", "Ann", {
    resources: { mana: { current: 5, max: 8 } },
    abilityIds: ["slash"],
  });
  const el = r.draw();
  expect(names(el)).toEqual(["Free"]);
  expect(texts(within(section(el, "Free"), "button"))).toEqual(["Slash"]);
});

// VTT-844
test("a resource ability's title: affordable, unaffordable, off the sheet", () => {
  const r = abilityRig();
  const bolt = () => within(section(r.draw(), "Spells"), "button")[0]?.title;
  expect(bolt()).toBe("Mana 5/8, costs 2");
  r.st.Actors["a1"] = actor("a1", "Ann", {
    resources: { mana: { current: 2, max: 8 } },
    abilityIds: ["bolt"],
  });
  expect(bolt()).toBe("Mana 2/8, costs 2");
  r.st.Actors["a1"] = actor("a1", "Ann", {
    resources: { mana: { current: 1, max: 8 } },
    abilityIds: ["bolt"],
  });
  expect(bolt()).toBe("Mana 1/8, costs 2 — not enough");
  r.st.Actors["a1"] = actor("a1", "Ann", { abilityIds: ["bolt"] });
  expect(bolt()).toBe("Mana, costs 2 — not enough");
});

// VTT-844
test("a resource the window does not declare is named by its key", () => {
  const r = abilityRig();
  r.properties = undefined;
  expect(within(section(r.draw(), "Spells"), "button")[0]?.title).toBe("mana 5/8, costs 2");
  r.properties = [prop("mana", "number", "Mana")];
  expect(within(section(r.draw(), "Spells"), "button")[0]?.title).toBe("mana 5/8, costs 2");
});

// VTT-844
test("an at-will button's title says at will and names no cost", () => {
  const title = within(section(abilityRig().draw(), "Free"), "button")[0]?.title ?? "";
  expect(title).toContain("at will");
  expect(title).not.toContain("costs");
});

// VTT-846
test("a button is labelled by the ability's name, else its id", () => {
  const r = abilityRig();
  r.abilities = [ab("jab", "", { kind: "atWill" }), ab("slash", "Slash", { kind: "atWill" })];
  r.st.Actors["a1"] = actor("a1", "Ann", { abilityIds: ["jab", "slash"] });
  expect(texts(within(section(r.draw(), "Free"), "button"))).toEqual(["jab", "Slash"]);
});

// VTT-843
test("an armed ability's targets are drawn outside the page, on every page", () => {
  const r = abilityRig();
  r.st.Actors["a2"] = actor("a2", "Goblin", { kind: ActorKind.NON_PARTY });
  r.st.Scenes["s1"] = { ID: "s1", Name: "S", GridWidth: 5, GridHeight: 5 };
  r.st.Tokens["t1"] = { ID: "t1", SceneID: "s1", ActorID: "a1", X: 0, Y: 0 };
  r.st.Tokens["t2"] = { ID: "t2", SceneID: "s1", ActorID: "a2", X: 1, Y: 0 };
  const slash = within(section(r.draw(), "Free"), "button")[1];
  slash?.click();
  expect(r.ui.selectedAbilityId).toBe("slash");
  const el = r.draw();
  const targets = Array.from(el.querySelectorAll('[data-action="use-ability"]'));
  expect(targets.length).toBeGreaterThan(0);
  expect(targets.some((t) => t.closest(".sheet") !== null)).toBe(false);
  r.layout = {
    sheet: [
      { name: "One", sections: [{ name: "Free", kind: "abilities", abilities: [{ kind: "atWill" }] }] },
      { name: "Two", sections: [{ name: "Vals", kind: "values", values: ["mana"] }] },
    ],
  };
  tabs(r.draw())[1]?.click();
  const el2 = r.draw();
  expect(shownTab(el2)).toEqual(["Two"]);
  expect(el2.querySelectorAll('[data-action="use-ability"]').length).toBe(targets.length);
});

class FakeWS extends EventTarget {
  static instances: FakeWS[] = [];
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  readyState = 0;
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  constructor(readonly url: string) {
    super();
    FakeWS.instances.push(this);
    setTimeout(() => {
      this.readyState = 1;
      const ev = new Event("open");
      this.dispatchEvent(ev);
    }, 0);
  }
  send(_data: string): void {}
  close(): void {
    this.readyState = 3;
  }
  deliver(frame: object): void {
    const ev = new MessageEvent("message", { data: JSON.stringify(frame) });
    this.dispatchEvent(ev);
  }
}

const tick = async (n = 6) => {
  for (let i = 0; i < n; i++) await new Promise((r) => setTimeout(r, 0));
};

function rulesetBody(tag: string, pages: string[]): object {
  return {
    id: "r",
    name: "R",
    abilities: [],
    resources: [],
    properties: [],
    conditions: [{ id: "burn", name: `Burn ${tag}`, description: "" }],
    sheet: pages.map((p) => ({ name: p, sections: [{ name: `Sec ${p}`, kind: "conditions" }] })),
  };
}

async function newestReadWins(answerOrder: "newestFirst" | "oldestFirst") {
  const realFetch = globalThis.fetch;
  const realWS = globalThis.WebSocket;
  const reads: ((body: object) => void)[] = [];
  const respond = (body: object) => new Response(JSON.stringify(body));
  const fakeFetch = async (input: string | URL | Request) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    if (url.includes("/api/me")) return respond({ participantId: "p1", name: "P", role: "player" });
    if (url.includes("/api/adventures")) return respond({ adventures: [] });
    if (url.includes("/api/ruleset")) {
      return new Promise<Response>((res) => reads.push((b) => res(respond(b))));
    }
    return respond({});
  };
  Object.defineProperty(globalThis, "fetch", { value: fakeFetch, configurable: true, writable: true });
  Object.defineProperty(globalThis, "WebSocket", { value: FakeWS, configurable: true, writable: true });
  FakeWS.instances = [];
  localStorage.setItem("vtt.token", "tok");
  const root = document.createElement("div");
  document.body.append(root);
  const session = boot(root);
  try {
    await tick();
    reads[0]?.(rulesetBody("start", ["Start page"]));
    await tick();
    const ws = FakeWS.instances[0];
    if (!ws) throw new Error("no socket opened");
    const env = (sequence: number, c: object) => ({
      event: { eventId: `e${sequence}`, sequence, sessionId: "s", actorRole: "dm", participantId: "d", ...c },
    });
    ws.deliver({ viewpointChanged: { actorId: "a1" } });
    ws.deliver(
      env(1, {
        actorAdded: {
          actor: { actorId: "a1", name: "Ann", resources: {}, attributes: {}, abilityIds: [], kind: "ACTOR_KIND_PARTY_MEMBER" },
        },
      }),
    );
    ws.deliver(env(2, { conditionApplied: { actorId: "a1", conditionId: "burn" } }));
    ws.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Ann" }] } });
    await tick();
    ws.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Ann" }, { actorId: "a2", name: "Bo" }] } });
    await tick();
    expect(reads.length).toBeGreaterThanOrEqual(3);
    const older = reads[reads.length - 2];
    const newest = reads[reads.length - 1];
    if (answerOrder === "newestFirst") {
      newest?.(rulesetBody("NEW", ["New A", "New B"]));
      await tick();
      older?.(rulesetBody("OLD", ["Old A", "Old B"]));
    } else {
      older?.(rulesetBody("OLD", ["Old A", "Old B"]));
      await tick();
      newest?.(rulesetBody("NEW", ["New A", "New B"]));
    }
    await tick();
    const panel = root.querySelector("section.player");
    if (!(panel instanceof HTMLElement)) throw new Error("no player panel: " + root.innerHTML);
    return {
      tabs: texts(tabs(panel)),
      sections: names(panel),
      conditions: texts(Array.from(panel.querySelectorAll(".condition"))),
    };
  } finally {
    session?.close();
    Object.defineProperty(globalThis, "fetch", { value: realFetch, configurable: true, writable: true });
    Object.defineProperty(globalThis, "WebSocket", { value: realWS, configurable: true, writable: true });
  }
}

// VTT-845
test("an older read's answer arriving after the newest's changes nothing", async () => {
  const got = await newestReadWins("newestFirst");
  expect(got.tabs).toEqual(["New A", "New B"]);
  expect(got.sections).toEqual(["Sec New A"]);
  expect(got.conditions).toEqual(["Burn NEW"]);
});

// VTT-845
test("answers in order also leave the newest's sheet and names", async () => {
  const got = await newestReadWins("oldestFirst");
  expect(got.tabs).toEqual(["New A", "New B"]);
  expect(got.conditions).toEqual(["Burn NEW"]);
});
