import "./support/dom";
import { describe, expect, test } from "bun:test";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { newState, type Actor, type State } from "../src/state";
import type { ConditionDef } from "../src/metadata";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ok } from "./support/answers";

function actor(actorId: string, name: string): Actor {
  return {
    actorId, name, moduleId: "", attributes: {}, resources: {},
    controllerId: "", controllerIds: [], kind: 0 as Actor["kind"], abilityIds: [], references: {},
  };
}

function build(
  st: State,
  conditions: readonly ConditionDef[] | undefined,
): { root: HTMLElement; sent: ClientCommand[] } {
  const sent: ClientCommand[] = [];
  const deps: DMDeps = {
    st, participants: [], adventures: [], maps: [], abilities: [],
    guideFor: async () => null, joinLink: null, roster: null, origin: "http://x",
    refreshSharing: () => {}, send: async (c) => { sent.push(c); return ok(); },
    rerender: () => {}, notify: () => {}, confirm: () => true,
    doorsArmed: false, toggleDoors: () => {},
    ...(conditions === undefined ? {} : { conditions }),
  };
  return { root: renderDMConsole(deps), sent };
}

function holding(actors: Actor[], held: Record<string, string[]>): State {
  const st = newState();
  for (const a of actors) st.Actors[a.actorId] = a;
  for (const [id, cs] of Object.entries(held)) {
    st.Conditions[id] = cs.map((c, i) => ({ ID: c, Source: "", AppliedSeq: i + 1 }));
  }
  return st;
}

function labels(root: HTMLElement): string[] {
  return Array.from(root.querySelectorAll('[data-action="remove-condition"]'))
    .map((b) => (b.textContent ?? "").trim());
}

const def = (id: string, name: string): ConditionDef => ({ id, name, description: "" });

describe("console remove-condition buttons", () => {
  // VTT-864
  test("reads actor name, condition name, cross", () => {
    const { root } = build(holding([actor("a1", "Mira")], { a1: ["c1"] }), [def("c1", "Poisoned")]);
    expect(labels(root)).toEqual(["Mira: Poisoned ✕"]);
  });

  // VTT-864
  test("actor named by the fold even when its token has other names", () => {
    const st = holding([actor("a1", "Mira")], { a1: ["c1"] });
    st.Tokens["t1"] = { ID: "t1", SceneID: "s", ActorID: "a1", X: 0, Y: 0, Name: "Shade", GMName: "Boss" };
    const { root } = build(st, [def("c1", "Poisoned")]);
    expect(labels(root)).toEqual(["Mira: Poisoned ✕"]);
  });

  // VTT-864
  test("actor with an empty name reads by id", () => {
    const { root } = build(holding([actor("a1", "")], { a1: ["c1"] }), [def("c1", "Poisoned")]);
    expect(labels(root)).toEqual(["a1: Poisoned ✕"]);
  });

  // VTT-864
  test("actor id the state does not hold reads by id", () => {
    const { root } = build(holding([], { ghost: ["c1"] }), [def("c1", "Poisoned")]);
    expect(labels(root)).toEqual(["ghost: Poisoned ✕"]);
  });

  // VTT-864
  test("condition declared with an empty name reads by id", () => {
    const { root } = build(holding([actor("a1", "Mira")], { a1: ["c1"] }), [def("c1", "")]);
    expect(labels(root)).toEqual(["Mira: c1 ✕"]);
  });

  // VTT-864
  test("undeclared condition reads by id", () => {
    const { root } = build(holding([actor("a1", "Mira")], { a1: ["c9"] }), [def("c1", "Poisoned")]);
    expect(labels(root)).toEqual(["Mira: c9 ✕"]);
  });

  // VTT-864
  test("no condition list reads by id", () => {
    const { root } = build(holding([actor("a1", "Mira")], { a1: ["c1"] }), undefined);
    expect(labels(root)).toEqual(["Mira: c1 ✕"]);
  });

  // VTT-864
  test("several actors and conditions each by their own entries", () => {
    const st = holding([actor("a1", "Mira"), actor("a2", "Orin")], { a1: ["c1", "c2"], a2: ["c2"] });
    const { root } = build(st, [def("c1", "Poisoned"), def("c2", "Prone")]);
    expect(labels(root).sort()).toEqual(
      ["Mira: Poisoned ✕", "Mira: Prone ✕", "Orin: Prone ✕"].sort(),
    );
  });

  // VTT-864
  test("click sends removeCondition with ids for a named case", () => {
    const { root, sent } = build(holding([actor("a1", "Mira")], { a1: ["c1"] }), [def("c1", "Poisoned")]);
    const b = root.querySelector('[data-action="remove-condition"]') as HTMLElement;
    b.click();
    expect(sent.length).toBe(1);
    const c = sent[0]!.command;
    expect(c.case).toBe("removeCondition");
    expect(c.value).toMatchObject({ actorId: "a1", conditionId: "c1" });
  });

  // VTT-864
  test("click sends ids for an unnamed case", () => {
    const { root, sent } = build(holding([actor("a1", "")], { a1: ["c1"] }), undefined);
    (root.querySelector('[data-action="remove-condition"]') as HTMLElement).click();
    expect(sent.length).toBe(1);
    const c = sent[0]!.command;
    expect(c.case).toBe("removeCondition");
    expect(c.value).toMatchObject({ actorId: "a1", conditionId: "c1" });
  });

  test("no group when no actor holds a condition", () => {
    const { root } = build(holding([actor("a1", "Mira")], {}), [def("c1", "Poisoned")]);
    expect(labels(root)).toEqual([]);
    expect(root.textContent ?? "").not.toContain("Remove condition");
  });
});
