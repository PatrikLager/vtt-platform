// Independent QA of the DM console's value fields for a give.
// Pins the console's rows for a give's value, and holds VTT-730 unchanged.
import "./support/dom";
import { describe, expect, test } from "bun:test";
import { ok, refused, settle } from "./support/answers";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import {
  emptyMap, newState, type Actor, type Resource, type State,
} from "../src/state";
import type { Ability, Property } from "../src/metadata";
import type {
  ClientCommand, CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

const CHOOSE_FIRST = "Choose an ability to give first.";

const FLURRY: Ability = {
  id: "qa5-flurry", name: "Hunter's Flurry", range: 1, maxTargets: 1,
  usage: { kind: "resource", resource: "flurry_uses", cost: 1 },
};
const SWING: Ability = {
  id: "qa5-swing", name: "Chair Swing", range: 1, maxTargets: 1,
  usage: { kind: "resource", resource: "stamina", cost: 2 },
};
const STRIKE: Ability = {
  id: "qa5-strike", name: "Strike", range: 1, maxTargets: 1,
  usage: { kind: "atWill" },
};
const ABILITIES: readonly Ability[] = [FLURRY, SWING, STRIKE];

// The window names flurry_uses and hp; nothing names stamina.
const PROPERTIES: readonly Property[] = [
  { key: "flurry_uses", type: "resource", name: "Flurry Uses",
    description: "" },
  { key: "hp", type: "resource", name: "Hit Points", description: "" },
];

function actor(
  id: string, resources: Record<string, Resource> = {},
  abilityIds: string[] = [],
): Actor {
  const res = emptyMap<Resource>();
  for (const [k, v] of Object.entries(resources)) res[k] = v;
  return {
    actorId: id, name: id.toUpperCase(), moduleId: "",
    attributes: emptyMap<number>(), resources: res, controllerId: "",
    controllerIds: [], kind: ActorKind.PARTY_MEMBER, abilityIds,
  };
}

interface Opts {
  properties?: readonly Property[] | undefined;
  answer?: () => CommandResult;
}

interface Harness {
  st: State;
  sent: ClientCommand[];
  notes: string[];
  rerenders: number;
  paint: () => void;
  row: (id: string) => Element;
}

function harness(actors: Actor[], opts: Opts = {}): Harness {
  const st = newState();
  for (const a of actors) st.Actors[a.actorId] = a;
  const host = document.createElement("div");
  document.body.appendChild(host);
  const answer = opts.answer ?? ok;
  const h: Harness = {
    st, sent: [], notes: [], rerenders: 0,
    paint: () => host.replaceChildren(renderDMConsole(deps)),
    row: (id) => {
      const r = host.querySelector(`.ability-actor[data-actor="${id}"]`);
      if (!r) throw new Error(`no abilities row for ${id}`);
      return r;
    },
  };
  const deps: DMDeps = {
    st, participants: [], adventures: [], maps: [],
    abilities: ABILITIES,
    properties: "properties" in opts ? opts.properties : PROPERTIES,
    guideFor: async () => null, joinLink: null, roster: null,
    origin: "http://qa.invalid", refreshSharing: () => {},
    // As app.ts's act: paint at the answer, before the console handles it.
    send: async (c) => {
      h.sent.push(c);
      const r = answer();
      h.paint();
      return r;
    },
    rerender: () => { h.rerenders++; h.paint(); },
    // As app.ts's notify: set the toast and repaint.
    notify: (m) => { h.notes.push(m); h.paint(); },
    confirm: () => true, doorsArmed: false, toggleDoors: () => {},
  };
  h.paint();
  return h;
}

function select(h: Harness, id: string): HTMLSelectElement {
  const s = h.row(id).querySelector("select.give-ability");
  if (!s) throw new Error(`no give select for ${id}`);
  return s as HTMLSelectElement;
}

function field(h: Harness, id: string, which: "current" | "max") {
  const f = h.row(id).querySelector(
    `[data-field="give-ability-${which}-${id}"]`,
  );
  return f as HTMLInputElement | null;
}

function shown(el: Element | null): boolean {
  for (let e = el; e; e = e.parentElement) {
    if (e.hasAttribute("hidden")) return false;
    if ((e as HTMLElement).style?.display === "none") return false;
  }
  return el !== null;
}

function fieldsShown(h: Harness, id: string): [boolean, boolean] {
  return [shown(field(h, id, "current")), shown(field(h, id, "max"))];
}

function choose(h: Harness, id: string, abilityId: string): void {
  const s = select(h, id);
  s.value = abilityId;
  s.dispatchEvent(new Event("change", { bubbles: true }));
}

function type(
  h: Harness, id: string, which: "current" | "max", text: string,
): void {
  const f = field(h, id, which);
  if (!f) throw new Error(`no ${which} field for ${id}`);
  f.value = text;
  f.dispatchEvent(new Event("input", { bubbles: true }));
}

async function give(h: Harness, id: string): Promise<void> {
  const b = h.row(id).querySelector('[data-action="give-ability"]');
  if (!b) throw new Error(`no Give for ${id}`);
  (b as HTMLButtonElement).click();
  await settle();
}

// Every name a field is given: its label, placeholder, aria-label, title.
function labelOf(f: HTMLInputElement | null): string {
  if (!f) return "";
  const parts = [
    f.getAttribute("placeholder") ?? "",
    f.getAttribute("aria-label") ?? "",
    f.getAttribute("title") ?? "",
    f.closest("label")?.textContent ?? "",
  ];
  if (f.id) {
    const l = f.ownerDocument.querySelector(`label[for="${f.id}"]`);
    parts.push(l?.textContent ?? "");
  }
  return parts.join(" | ");
}

interface Given {
  actorId: string;
  abilityId: string;
  resources: Record<string, { current: number; max: number }>;
}

function gives(h: Harness): Given[] {
  return h.sent.flatMap((c) => {
    if (c.command.case !== "giveAbility") return [];
    const v = c.command.value;
    const resources: Given["resources"] = {};
    for (const [k, r] of Object.entries(v.resources)) {
      resources[k] = { current: r.current, max: r.max };
    }
    return [{ actorId: v.actorId, abilityId: v.abilityId, resources }];
  });
}

// draft is the module's and outlives the test (SPEC-022): empty it.
function forget(h: Harness, id: string): void {
  if (field(h, id, "current")) type(h, id, "current", "");
  if (field(h, id, "max")) type(h, id, "max", "");
  choose(h, id, "");
}

describe("qa-give-value: when the console asks for a value", () => {
  test("no field is shown before an ability is chosen", () => {
    // VTT-788
    const h = harness([actor("qa5-none")]);
    expect(fieldsShown(h, "qa5-none")).toEqual([false, false]);
  });

  test("a limited ability whose resource the sheet lacks shows both", () => {
    // VTT-788
    const h = harness([actor("qa5-lacks")]);
    choose(h, "qa5-lacks", FLURRY.id);
    expect(fieldsShown(h, "qa5-lacks")).toEqual([true, true]);
    forget(h, "qa5-lacks");
  });

  test("a limited ability whose resource the sheet holds shows none", () => {
    // VTT-788
    const h = harness([
      actor("qa5-holds", { flurry_uses: { current: 2, max: 3 } }),
    ]);
    choose(h, "qa5-holds", FLURRY.id);
    expect(fieldsShown(h, "qa5-holds")).toEqual([false, false]);
    forget(h, "qa5-holds");
  });

  test("a resource held at 0 of no maximum is held: no field", () => {
    // VTT-788; VTT-760 holds a resource "a current of 0 included".
    const h = harness([
      actor("qa5-zero", { flurry_uses: { current: 0, max: 0 } }),
    ]);
    choose(h, "qa5-zero", FLURRY.id);
    expect(fieldsShown(h, "qa5-zero")).toEqual([false, false]);
    forget(h, "qa5-zero");
  });

  test("an at-will ability shows no field", () => {
    // VTT-788
    const h = harness([actor("qa5-atwill")]);
    choose(h, "qa5-atwill", STRIKE.id);
    expect(fieldsShown(h, "qa5-atwill")).toEqual([false, false]);
    forget(h, "qa5-atwill");
  });

  test("choosing another ability, or none, hides the fields", () => {
    // VTT-788
    const h = harness([actor("qa5-switch")]);
    choose(h, "qa5-switch", FLURRY.id);
    expect(fieldsShown(h, "qa5-switch")).toEqual([true, true]);
    choose(h, "qa5-switch", STRIKE.id);
    expect(fieldsShown(h, "qa5-switch")).toEqual([false, false]);
    choose(h, "qa5-switch", FLURRY.id);
    expect(fieldsShown(h, "qa5-switch")).toEqual([true, true]);
    choose(h, "qa5-switch", "");
    expect(fieldsShown(h, "qa5-switch")).toEqual([false, false]);
  });

  test("a resource held by one actor and not another: per row", () => {
    // VTT-788: "the actor's sheet" is the row's own actor.
    const h = harness([
      actor("qa5-rowa"),
      actor("qa5-rowb", { flurry_uses: { current: 1, max: 1 } }),
    ]);
    choose(h, "qa5-rowa", FLURRY.id);
    choose(h, "qa5-rowb", FLURRY.id);
    expect(fieldsShown(h, "qa5-rowa")).toEqual([true, true]);
    expect(fieldsShown(h, "qa5-rowb")).toEqual([false, false]);
    forget(h, "qa5-rowa");
    forget(h, "qa5-rowb");
  });
});

describe("qa-give-value: what the fields are called", () => {
  test("named as the window names the resource, not by its key", () => {
    // VTT-788, VTT-806.
    const h = harness([actor("qa5-label")]);
    choose(h, "qa5-label", FLURRY.id);
    const cur = labelOf(field(h, "qa5-label", "current"));
    const max = labelOf(field(h, "qa5-label", "max"));
    expect(cur.includes("Flurry Uses")).toBe(true);
    expect(max.includes("Flurry Uses")).toBe(true);
    expect(cur.includes("flurry_uses")).toBe(false);
    expect(max.includes("flurry_uses")).toBe(false);
    expect(cur === max).toBe(false);
    forget(h, "qa5-label");
  });

  test("by its key when the window's properties do not name it", () => {
    // VTT-788, VTT-806; SPEC-022's Namer.resource, VTT-622 and VTT-623.
    const h = harness([actor("qa5-key")]);
    choose(h, "qa5-key", SWING.id);
    expect(labelOf(field(h, "qa5-key", "current")).includes("stamina"))
      .toBe(true);
    expect(labelOf(field(h, "qa5-key", "max")).includes("stamina"))
      .toBe(true);
    forget(h, "qa5-key");
  });

  test("by its key while the window has no properties", () => {
    // VTT-788, VTT-806; SPEC-022: "by its key while the window has no
    // properties" (VTT-623).
    const h = harness([actor("qa5-noprops")], { properties: undefined });
    choose(h, "qa5-noprops", FLURRY.id);
    expect(labelOf(field(h, "qa5-noprops", "current"))
      .includes("flurry_uses")).toBe(true);
    expect(labelOf(field(h, "qa5-noprops", "max"))
      .includes("flurry_uses")).toBe(true);
    forget(h, "qa5-noprops");
  });

  test("a number property under the key names no resource", () => {
    // VTT-788, VTT-806; SPEC-022: "a `number` property names no resource"
    // (VTT-622, VTT-623).
    const props: Property[] = [...PROPERTIES, {
      key: "stamina", type: "number", name: "Stamina Score",
      description: "",
    }];
    const h = harness([actor("qa5-numprop")], { properties: props });
    choose(h, "qa5-numprop", SWING.id);
    const cur = labelOf(field(h, "qa5-numprop", "current"));
    expect(cur.includes("stamina")).toBe(true);
    expect(cur.includes("Stamina Score")).toBe(false);
    forget(h, "qa5-numprop");
  });
});

describe("qa-give-value: what a give sends under resources", () => {
  test("the current and the max as typed", async () => {
    // VTT-788
    const h = harness([actor("qa5-both")]);
    choose(h, "qa5-both", FLURRY.id);
    type(h, "qa5-both", "current", "3");
    type(h, "qa5-both", "max", "5");
    await give(h, "qa5-both");
    expect(gives(h)).toEqual([{
      actorId: "qa5-both", abilityId: FLURRY.id,
      resources: { flurry_uses: { current: 3, max: 5 } },
    }]);
    expect(h.notes).toEqual([]);
  });

  test("a max left empty is sent as none, max 0 in the command", async () => {
    // VTT-788: "a max left empty as none"; VTT-679 for none.
    const h = harness([actor("qa5-nomax")]);
    choose(h, "qa5-nomax", FLURRY.id);
    type(h, "qa5-nomax", "current", "4");
    await give(h, "qa5-nomax");
    expect(gives(h)).toEqual([{
      actorId: "qa5-nomax", abilityId: FLURRY.id,
      resources: { flurry_uses: { current: 4, max: 0 } },
    }]);
    expect(h.notes).toEqual([]);
  });

  test("a current of 0 is a value and is sent", async () => {
    // VTT-788, VTT-789 (0 is an integer); VTT-760 "a current of 0".
    const h = harness([actor("qa5-zcur"), actor("qa5-zboth")]);
    choose(h, "qa5-zcur", FLURRY.id);
    type(h, "qa5-zcur", "current", "0");
    await give(h, "qa5-zcur");
    choose(h, "qa5-zboth", FLURRY.id);
    type(h, "qa5-zboth", "current", "0");
    type(h, "qa5-zboth", "max", "0");
    await give(h, "qa5-zboth");
    expect(gives(h).map((g) => g.resources)).toEqual([
      { flurry_uses: { current: 0, max: 0 } },
      { flurry_uses: { current: 0, max: 0 } },
    ]);
    expect(h.notes).toEqual([]);
  });

  test("negative integers are integers and are sent", async () => {
    // VTT-788, VTT-789, VTT-805, VTT-808: the console refuses only what is
    // not a whole number an int32 holds; the server's rules are VTT-760..763's.
    const h = harness([actor("qa5-negcur"), actor("qa5-negmax")]);
    choose(h, "qa5-negcur", FLURRY.id);
    type(h, "qa5-negcur", "current", "-2");
    type(h, "qa5-negcur", "max", "4");
    await give(h, "qa5-negcur");
    choose(h, "qa5-negmax", FLURRY.id);
    type(h, "qa5-negmax", "current", "1");
    type(h, "qa5-negmax", "max", "-1");
    await give(h, "qa5-negmax");
    expect(gives(h).map((g) => g.resources)).toEqual([
      { flurry_uses: { current: -2, max: 4 } },
      { flurry_uses: { current: 1, max: -1 } },
    ]);
    expect(h.notes).toEqual([]);
  });

  test("the resource is keyed by the ability's resource", async () => {
    // VTT-788; VTT-763 refuses a key the ability does not spend.
    const h = harness([actor("qa5-stam")]);
    choose(h, "qa5-stam", SWING.id);
    type(h, "qa5-stam", "current", "6");
    type(h, "qa5-stam", "max", "6");
    await give(h, "qa5-stam");
    expect(gives(h)).toEqual([{
      actorId: "qa5-stam", abilityId: SWING.id,
      resources: { stamina: { current: 6, max: 6 } },
    }]);
  });

  test("an at-will give sends no resources", async () => {
    // VTT-788, VTT-807; VTT-763.
    const h = harness([actor("qa5-sendatw")]);
    choose(h, "qa5-sendatw", STRIKE.id);
    await give(h, "qa5-sendatw");
    expect(gives(h)).toEqual([{
      actorId: "qa5-sendatw", abilityId: STRIKE.id, resources: {},
    }]);
  });

  test("a give of a held resource's ability sends none", async () => {
    // VTT-788, VTT-807; VTT-762 refuses a resource the sheet holds.
    const h = harness([
      actor("qa5-sendheld", { flurry_uses: { current: 1, max: 2 } }),
    ]);
    choose(h, "qa5-sendheld", FLURRY.id);
    await give(h, "qa5-sendheld");
    expect(gives(h)).toEqual([{
      actorId: "qa5-sendheld", abilityId: FLURRY.id, resources: {},
    }]);
    expect(h.notes).toEqual([]);
  });

  test("values typed, then an at-will chosen: none is sent", async () => {
    // VTT-788, VTT-807; VTT-763.
    const h = harness([actor("qa5-stale")]);
    choose(h, "qa5-stale", FLURRY.id);
    type(h, "qa5-stale", "current", "2");
    type(h, "qa5-stale", "max", "2");
    choose(h, "qa5-stale", STRIKE.id);
    await give(h, "qa5-stale");
    expect(gives(h)).toEqual([{
      actorId: "qa5-stale", abilityId: STRIKE.id, resources: {},
    }]);
  });

  test("one row's values never reach another row's give", async () => {
    // VTT-788: the fields are the row's actor's.
    const h = harness([actor("qa5-twoa"), actor("qa5-twob")]);
    choose(h, "qa5-twoa", FLURRY.id);
    choose(h, "qa5-twob", FLURRY.id);
    type(h, "qa5-twoa", "current", "7");
    type(h, "qa5-twoa", "max", "8");
    type(h, "qa5-twob", "current", "1");
    expect(field(h, "qa5-twob", "max")?.value).toBe("");
    await give(h, "qa5-twob");
    await give(h, "qa5-twoa");
    expect(gives(h)).toEqual([
      { actorId: "qa5-twob", abilityId: FLURRY.id,
        resources: { flurry_uses: { current: 1, max: 0 } } },
      { actorId: "qa5-twoa", abilityId: FLURRY.id,
        resources: { flurry_uses: { current: 7, max: 8 } } },
    ]);
  });
});

describe("qa-give-value: what the console refuses", () => {
  test("an empty current: says so, sends nothing", async () => {
    // VTT-789
    const h = harness([actor("qa5-empty1"), actor("qa5-empty2")]);
    choose(h, "qa5-empty1", FLURRY.id);
    await give(h, "qa5-empty1");
    choose(h, "qa5-empty2", FLURRY.id);
    type(h, "qa5-empty2", "max", "5");
    await give(h, "qa5-empty2");
    expect(h.sent.length).toBe(0);
    expect(h.notes.length).toBe(2);
    for (const n of h.notes) {
      expect(n.trim().length > 0).toBe(true);
      expect(n === CHOOSE_FIRST).toBe(false);
    }
    forget(h, "qa5-empty1");
    forget(h, "qa5-empty2");
  });

  const badCurrent = ["abc", "1.5", "2x", "   "];
  test.each(badCurrent)("current %p: says so, sends nothing", async (v) => {
    // VTT-789: empty or not an integer; blank is either.
    const id = `qa5-badcur-${badCurrent.indexOf(v)}`;
    const h = harness([actor(id)]);
    choose(h, id, FLURRY.id);
    type(h, id, "current", v);
    type(h, id, "max", "5");
    await give(h, id);
    expect(h.sent.length).toBe(0);
    expect(h.notes.length).toBe(1);
    expect((h.notes[0] ?? "").trim().length > 0).toBe(true);
    expect(h.notes[0] === CHOOSE_FIRST).toBe(false);
    forget(h, id);
  });

  const badMax = ["abc", "1.5", "2x"];
  test.each(badMax)("max %p: says so, sends nothing", async (v) => {
    // VTT-805
    const id = `qa5-badmax-${badMax.indexOf(v)}`;
    const h = harness([actor(id)]);
    choose(h, id, FLURRY.id);
    type(h, id, "current", "1");
    type(h, id, "max", v);
    await give(h, id);
    expect(h.sent.length).toBe(0);
    expect(h.notes.length).toBe(1);
    expect((h.notes[0] ?? "").trim().length > 0).toBe(true);
    expect(h.notes[0] === CHOOSE_FIRST).toBe(false);
    forget(h, id);
  });

  test("a bad max is told as the max's, not as the current's", async () => {
    // VTT-789, VTT-805: each "says so" of its own field.
    const h = harness([actor("qa5-whichc"), actor("qa5-whichm")]);
    choose(h, "qa5-whichc", FLURRY.id);
    type(h, "qa5-whichc", "current", "x");
    await give(h, "qa5-whichc");
    choose(h, "qa5-whichm", FLURRY.id);
    type(h, "qa5-whichm", "current", "1");
    type(h, "qa5-whichm", "max", "x");
    await give(h, "qa5-whichm");
    expect(h.sent.length).toBe(0);
    expect(h.notes.length).toBe(2);
    expect(h.notes[0] === h.notes[1]).toBe(false);
    forget(h, "qa5-whichc");
    forget(h, "qa5-whichm");
  });

  test("a refusal of the console's own keeps what was typed", async () => {
    // VTT-789, VTT-805; SPEC-022: a press the console's own checks
    // refuse sends nothing; the fields keep what was typed (VTT-517).
    const h = harness([actor("qa5-keepown")]);
    choose(h, "qa5-keepown", FLURRY.id);
    type(h, "qa5-keepown", "current", "1.5");
    type(h, "qa5-keepown", "max", "9");
    await give(h, "qa5-keepown");
    h.paint();
    expect(select(h, "qa5-keepown").value).toBe(FLURRY.id);
    expect(fieldsShown(h, "qa5-keepown")).toEqual([true, true]);
    expect(field(h, "qa5-keepown", "current")?.value).toBe("1.5");
    expect(field(h, "qa5-keepown", "max")?.value).toBe("9");
    type(h, "qa5-keepown", "current", "2");
    await give(h, "qa5-keepown");
    expect(gives(h).map((g) => g.resources)).toEqual([
      { flurry_uses: { current: 2, max: 9 } },
    ]);
  });
});

describe("qa-give-value: the drafts", () => {
  test("the choice and the values survive a repaint", async () => {
    // VTT-788: the fields keep their drafts across a repaint, as every
    // console field does (SPEC-022's draft).
    const h = harness([actor("qa5-repaint")]);
    choose(h, "qa5-repaint", FLURRY.id);
    type(h, "qa5-repaint", "current", "2");
    type(h, "qa5-repaint", "max", "7");
    h.paint();
    expect(select(h, "qa5-repaint").value).toBe(FLURRY.id);
    expect(fieldsShown(h, "qa5-repaint")).toEqual([true, true]);
    expect(field(h, "qa5-repaint", "current")?.value).toBe("2");
    expect(field(h, "qa5-repaint", "max")?.value).toBe("7");
    h.paint();
    await give(h, "qa5-repaint");
    expect(gives(h).map((g) => g.resources)).toEqual([
      { flurry_uses: { current: 2, max: 7 } },
    ]);
  });

  test("an accepted give clears the choice and the values", async () => {
    // VTT-788, VTT-533: the drafts that fed an accepted command clear.
    const h = harness([actor("qa5-clear")]);
    choose(h, "qa5-clear", FLURRY.id);
    type(h, "qa5-clear", "current", "3");
    type(h, "qa5-clear", "max", "3");
    const before = h.rerenders;
    await give(h, "qa5-clear");
    expect(gives(h).length).toBe(1);
    expect(h.rerenders > before).toBe(true);
    expect(select(h, "qa5-clear").value).toBe("");
    expect(fieldsShown(h, "qa5-clear")).toEqual([false, false]);
    h.paint();
    expect(select(h, "qa5-clear").value).toBe("");
    choose(h, "qa5-clear", FLURRY.id);
    expect(fieldsShown(h, "qa5-clear")).toEqual([true, true]);
    expect(field(h, "qa5-clear", "current")?.value).toBe("");
    expect(field(h, "qa5-clear", "max")?.value).toBe("");
    forget(h, "qa5-clear");
  });

  test("a give the server refuses keeps the choice and values", async () => {
    // VTT-788, VTT-517: a refusal clears nothing.
    const h = harness([actor("qa5-refkeep")], {
      answer: () => refused("qa5: refused"),
    });
    choose(h, "qa5-refkeep", FLURRY.id);
    type(h, "qa5-refkeep", "current", "5");
    type(h, "qa5-refkeep", "max", "6");
    await give(h, "qa5-refkeep");
    expect(gives(h).length).toBe(1);
    h.paint();
    expect(select(h, "qa5-refkeep").value).toBe(FLURRY.id);
    expect(fieldsShown(h, "qa5-refkeep")).toEqual([true, true]);
    expect(field(h, "qa5-refkeep", "current")?.value).toBe("5");
    expect(field(h, "qa5-refkeep", "max")?.value).toBe("6");
    forget(h, "qa5-refkeep");
  });
});

describe("qa-give-value: VTT-730 held unchanged", () => {
  test("Give with nothing chosen says so and sends nothing", async () => {
    // VTT-730
    const h = harness([actor("qa5-nochoice")]);
    await give(h, "qa5-nochoice");
    expect(h.sent.length).toBe(0);
    expect(h.notes).toEqual([CHOOSE_FIRST]);
    expect(fieldsShown(h, "qa5-nochoice")).toEqual([false, false]);
  });

  test("offers what the sheet lacks, takes what it names", async () => {
    // VTT-730
    const h = harness([
      actor("qa5-offers", { stamina: { current: 1, max: 1 } },
        [SWING.id]),
    ]);
    const offered = Array.from(select(h, "qa5-offers").options)
      .map((o) => o.value).filter((v) => v !== "");
    expect(offered).toEqual([FLURRY.id, STRIKE.id]);
    const take = h.row("qa5-offers").querySelector(
      `[data-action="take-ability"][data-ability="${SWING.id}"]`,
    );
    expect(take !== null).toBe(true);
    (take as HTMLElement).click();
    await settle();
    const cases = h.sent.map((c) => c.command.case);
    expect(cases).toEqual(["takeAbility"]);
    const c = h.sent[0]?.command;
    expect(c?.case === "takeAbility" && c.value.actorId).toBe("qa5-offers");
    expect(c?.case === "takeAbility" && c.value.abilityId).toBe(SWING.id);
  });
});
