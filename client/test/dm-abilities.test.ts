import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import type { Ability, Property } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import { held, ok, settle } from "./support/answers";

// Each test names its own actors: the console keeps what was chosen in a
// select across renders, keyed by the actor.

function ability(id: string, name: string): Ability {
  return { id, name, range: 1, maxTargets: 1, usage: { kind: "atWill" } };
}

const LIST = [ability("fists", "Fists"), ability("rally", ""), ability("chair-swing", "Chair Swing")];

function stateWith(actors: Record<string, { name: string; abilityIds: string[]; resources?: Record<string, { current: number; max: number }> }>): State {
  const st = newState();
  for (const [actorId, a] of Object.entries(actors)) {
    st.Actors[actorId] = {
      actorId, name: a.name, moduleId: "", attributes: {}, resources: a.resources ?? {},
      controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER, abilityIds: a.abilityIds, references: {},
    };
  }
  return st;
}

// A held console's answers wait until its release, as the server's do.
function console_(st: State, abilities: readonly Ability[] = LIST, answers?: ReturnType<typeof held>,
  properties?: readonly Property[]) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  const node = renderDMConsole({
    st, participants: [], adventures: [], maps: [], abilities,
    guideFor: async () => null, joinLink: null, roster: null, origin: "https://table.example",
    refreshSharing: () => {},
    send: async (c) => {
      sent.push(c);
      return answers === undefined ? ok() : answers.send(c);
    },
    rerender: () => {}, notify: (m) => notices.push(m), confirm: () => true,
    doorsArmed: false, toggleDoors: () => {}, properties,
  });
  document.body.appendChild(node);
  const row = (actorId: string) => node.querySelector(`.ability-actor[data-actor="${actorId}"]`) as HTMLElement | null;
  return {
    node, sent, notices, row,
    chips: (actorId: string) =>
      Array.from(row(actorId)?.querySelectorAll<HTMLButtonElement>('[data-action="take-ability"]') ?? []),
    pick: (actorId: string) => row(actorId)?.querySelector<HTMLSelectElement>("select.give-ability") ?? null,
    give: (actorId: string) => row(actorId)?.querySelector<HTMLButtonElement>('[data-action="give-ability"]') ?? null,
    choose: (actorId: string, abilityId: string) => {
      const pick = row(actorId)!.querySelector<HTMLSelectElement>("select.give-ability")!;
      pick.value = abilityId;
      pick.dispatchEvent(new Event("change"));
    },
  };
}

beforeEach(() => {
  document.body.innerHTML = "";
});

// VTT-730
test("each actor's row offers a take of each id its sheet names, once each, in the sheet's order", async () => {
  const c = console_(stateWith({
    "ab-take": { name: "Lera", abilityIds: ["rally", "fists", "rally", "no-such-move"] },
  }));
  const chips = c.chips("ab-take");
  expect(chips.map((b) => b.dataset["ability"])).toEqual(["rally", "fists", "no-such-move"]);
  expect(chips.map((b) => b.textContent)).toEqual(["rally ✕", "Fists ✕", "no-such-move ✕"]);
  chips[1]!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("takeAbility");
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "ab-take", abilityId: "fists" });
});

// VTT-730
test("each actor's row offers a give of each listed ability its sheet does not name, in the list's order", async () => {
  const c = console_(stateWith({ "ab-give": { name: "Lera", abilityIds: ["rally"] } }));
  const pick = c.pick("ab-give")!;
  expect(Array.from(pick.options).map((o) => o.value)).toEqual(["", "fists", "chair-swing"]);
  expect(Array.from(pick.options).map((o) => o.textContent)).toEqual(["choose an ability", "Fists", "Chair Swing"]);
  expect(c.give("ab-give")!.textContent).toBe("Give");
  pick.value = "chair-swing";
  pick.dispatchEvent(new Event("change"));
  c.give("ab-give")!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("giveAbility");
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "ab-give", abilityId: "chair-swing" });
});

// VTT-730
test("no ability the sheet names is offered to give, whatever the list's order", () => {
  const c = console_(stateWith({ "ab-named": { name: "", abilityIds: ["chair-swing", "fists"] } }));
  expect(Array.from(c.pick("ab-named")!.options).map((o) => o.value)).toEqual(["", "rally"]);
  expect(c.row("ab-named")!.querySelector(".ability-name")!.textContent).toBe("ab-named");
});

// VTT-730
test("a give with nothing chosen says so and sends nothing", async () => {
  const c = console_(stateWith({ "ab-none-chosen": { name: "Lera", abilityIds: [] } }));
  c.give("ab-none-chosen")!.click();
  await settle();
  expect(c.sent).toHaveLength(0);
  expect(c.notices).toEqual(["Choose an ability to give first."]);
});

// VTT-730
test("a choice is kept across renders, and an actor whose sheet names every listed ability is offered no give", () => {
  const st = stateWith({
    "ab-kept": { name: "Lera", abilityIds: [] },
    "ab-full": { name: "Dorn", abilityIds: ["fists", "rally", "chair-swing"] },
  });
  const first = console_(st);
  const pick = first.pick("ab-kept")!;
  pick.value = "rally";
  pick.dispatchEvent(new Event("change"));
  expect(console_(st).pick("ab-kept")!.value).toBe("rally");
  expect(first.pick("ab-full")).toBeNull();
  expect(first.give("ab-full")).toBeNull();
  expect(first.chips("ab-full")).toHaveLength(3);
});

// VTT-730
test("with no ability listed the console offers no give, and still a take; with no actor, no Abilities group", () => {
  const c = console_(stateWith({ "ab-unlisted": { name: "Lera", abilityIds: ["fists"] } }), []);
  expect(c.pick("ab-unlisted")).toBeNull();
  expect(c.chips("ab-unlisted").map((b) => b.textContent)).toEqual(["fists ✕"]);
  const empty = console_(newState());
  expect(Array.from(empty.node.querySelectorAll("h3")).map((h) => h.textContent)).not.toContain("Abilities");
  expect(Array.from(c.node.querySelectorAll("h3")).map((h) => h.textContent)).toContain("Abilities");
});

// VTT-730
test("a give still unanswered holds back that actor's give of that ability alone, not of another", async () => {
  const answers = held();
  const c = console_(stateWith({ "ab-lock-one": { name: "Lera", abilityIds: [] } }), LIST, answers);
  c.choose("ab-lock-one", "fists");
  c.give("ab-lock-one")!.click();
  c.give("ab-lock-one")!.click();
  c.choose("ab-lock-one", "chair-swing");
  c.give("ab-lock-one")!.click();
  answers.release();
  await settle();
  expect(c.sent.map((s) => s.command.value)).toMatchObject([
    { actorId: "ab-lock-one", abilityId: "fists" },
    { actorId: "ab-lock-one", abilityId: "chair-swing" },
  ]);
});

// VTT-730
test("a give still unanswered holds back no other actor's give of the same ability", async () => {
  const answers = held();
  const c = console_(stateWith({
    "ab-lock-a": { name: "Lera", abilityIds: [] },
    "ab-lock-b": { name: "Dorn", abilityIds: [] },
  }), LIST, answers);
  c.choose("ab-lock-a", "rally");
  c.give("ab-lock-a")!.click();
  c.choose("ab-lock-b", "rally");
  c.give("ab-lock-b")!.click();
  answers.release();
  await settle();
  expect(c.sent.map((s) => s.command.value)).toMatchObject([
    { actorId: "ab-lock-a", abilityId: "rally" },
    { actorId: "ab-lock-b", abilityId: "rally" },
  ]);
});

// flurry spends focus; jab spends nothing.
const SPENDING: Ability[] = [
  { id: "flurry", name: "Flurry", range: 1, maxTargets: 1, usage: { kind: "resource", resource: "focus", cost: 1 } },
  { id: "jab", name: "Jab", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
];
const FOCUS: Property[] = [{ key: "focus", type: "resource", name: "Focus", description: "" }];

/** The value fields of actorId's row, each null while hidden. */
function shownFields(c: ReturnType<typeof console_>, actorId: string) {
  const at = (f: string) => c.row(actorId)?.querySelector<HTMLInputElement>(`[data-field="${f}-${actorId}"]`) ?? null;
  const shown = (i: HTMLInputElement | null) => (i !== null && !i.hidden ? i : null);
  return { current: shown(at("give-ability-current")), max: shown(at("give-ability-max")) };
}

/** Whether each value field shows: a failing assertion then prints booleans, not DOM nodes. */
function fieldsShown(c: ReturnType<typeof console_>, actorId: string): [boolean, boolean] {
  const f = shownFields(c, actorId);
  return [f.current !== null, f.max !== null];
}

function spendingConsole(st: State) {
  return console_(st, SPENDING, undefined, FOCUS);
}

function fill(i: HTMLInputElement, v: string) {
  i.value = v;
  i.dispatchEvent(new Event("input"));
}

// VTT-788 VTT-806
test("choosing an ability that spends a resource the sheet lacks shows a current and a max field, named as the window names it", () => {
  const c = spendingConsole(stateWith({ "av-shown": { name: "Lera", abilityIds: [] } }));
  expect(fieldsShown(c, "av-shown")).toEqual([false, false]);
  c.choose("av-shown", "flurry");
  const f = shownFields(c, "av-shown");
  expect([f.current?.placeholder, f.max?.placeholder]).toEqual(["Focus now", "Focus max"]);
  expect(Array.from(c.row("av-shown")!.children).map((e) =>
    e.tagName === "SELECT" ? "select" : e.getAttribute("data-field") ?? e.textContent))
    .toEqual(["Lera", "select", "give-ability-current-av-shown", "give-ability-max-av-shown", "Give"]);
  c.choose("av-shown", "jab");
  expect(fieldsShown(c, "av-shown")).toEqual([false, false]);
  expect(["current", "max"].map((f) => c.row("av-shown")
    ?.querySelector<HTMLInputElement>(`[data-field="give-ability-${f}-av-shown"]`)?.placeholder ?? ""))
    .toEqual(["", ""]);
  c.choose("av-shown", "flurry");
  document.body.innerHTML = "";
  expect(shownFields(spendingConsole(stateWith({ "av-shown": { name: "Lera", abilityIds: [] } })), "av-shown").current?.placeholder)
    .toBe("Focus now");
});

// VTT-788 VTT-806
test("a resource is named by its key while the window has no properties", () => {
  const c = console_(stateWith({ "av-key": { name: "Lera", abilityIds: [] } }), SPENDING);
  c.choose("av-key", "flurry");
  expect(shownFields(c, "av-key").current?.placeholder).toBe("focus now");
});

// VTT-788
test("a give with the value sends the resource under resources, 0 included", async () => {
  const c = spendingConsole(stateWith({ "av-zero": { name: "Lera", abilityIds: [] } }));
  c.choose("av-zero", "flurry");
  const f = shownFields(c, "av-zero");
  fill(f.current!, "0");
  fill(f.max!, "3");
  c.give("av-zero")!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "av-zero", abilityId: "flurry",
    resources: { focus: { current: 0, max: 3 } } });
});

// VTT-788
test("a max left empty is sent as none, and values with spaces around them are read", async () => {
  const c = spendingConsole(stateWith({ "av-nomax": { name: "Lera", abilityIds: [] } }));
  c.choose("av-nomax", "flurry");
  fill(shownFields(c, "av-nomax").current!, " -2 ");
  fill(shownFields(c, "av-nomax").max!, "  ");
  c.give("av-nomax")!.click();
  await settle();
  expect(c.sent[0]!.command.value).toMatchObject({ resources: { focus: { current: -2, max: 0 } } });
});

// VTT-788 VTT-807
test("no value field is shown for an ability whose resource the sheet holds, nor for one that spends none, and none is sent", async () => {
  const held = stateWith({ "av-held": { name: "Lera", abilityIds: [], resources: { focus: { current: 1, max: 2 } } } });
  for (const [st, actor, ability] of [[held, "av-held", "flurry"],
    [stateWith({ "av-free": { name: "Lera", abilityIds: [] } }), "av-free", "jab"]] as const) {
    const c = spendingConsole(st);
    c.choose(actor, ability);
    expect(fieldsShown(c, actor)).toEqual([false, false]);
    c.give(actor)!.click();
    await settle();
    expect(c.sent).toHaveLength(1);
    expect(Object.keys((c.sent[0]!.command.value as { resources: object }).resources)).toEqual([]);
  }
});

// VTT-789
test("a give whose current is empty or not an integer says so and sends nothing", async () => {
  for (const current of ["", "   ", "x", "1.5", "2 3", "1e2", "--1"]) {
    const actor = "av-bad";
    document.body.innerHTML = "";
    const c = spendingConsole(stateWith({ [actor]: { name: "Lera", abilityIds: [] } }));
    c.choose(actor, "flurry");
    fill(shownFields(c, actor).current!, current);
    c.give(actor)!.click();
    await settle();
    expect([current, c.sent]).toEqual([current, []]);
    expect(c.notices).toEqual(["Give Focus a current value that is a whole number from -2147483648 to 2147483647."]);
  }
});

// VTT-805
test("a give whose max is neither empty nor an integer says so and sends nothing", async () => {
  const c = spendingConsole(stateWith({ "av-badmax": { name: "Lera", abilityIds: [] } }));
  c.choose("av-badmax", "flurry");
  const f = shownFields(c, "av-badmax");
  fill(f.current!, "1");
  fill(f.max!, "1.5");
  c.give("av-badmax")!.click();
  await settle();
  expect(c.sent).toEqual([]);
  expect(c.notices).toEqual(["Give Focus a max that is a whole number from -2147483648 to 2147483647, or leave it empty for none."]);
});

// VTT-788 VTT-533
test("an accepted give clears the value fields with the choice", async () => {
  const st = stateWith({ "av-clear": { name: "Lera", abilityIds: [] } });
  const c = spendingConsole(st);
  c.choose("av-clear", "flurry");
  fill(shownFields(c, "av-clear").current!, "1");
  fill(shownFields(c, "av-clear").max!, "2");
  c.give("av-clear")!.click();
  await settle();
  document.body.innerHTML = "";
  const again = spendingConsole(st);
  expect(again.pick("av-clear")!.value).toBe("");
  again.choose("av-clear", "flurry");
  expect([shownFields(again, "av-clear").current?.value, shownFields(again, "av-clear").max?.value]).toEqual(["", ""]);
});

// VTT-788
test("a sign before a whole number is read", async () => {
  const c = spendingConsole(stateWith({ "av-sign": { name: "Lera", abilityIds: [] } }));
  c.choose("av-sign", "flurry");
  fill(shownFields(c, "av-sign").current!, "+1");
  fill(shownFields(c, "av-sign").max!, "+2147483647");
  c.give("av-sign")!.click();
  await settle();
  expect(c.sent[0]!.command.value).toMatchObject({ resources: { focus: { current: 1, max: 2147483647 } } });
});

// VTT-808
test("a current or a max a 32-bit field cannot hold says so and sends nothing", async () => {
  for (const [now, top, notice] of [
    ["2147483648", "", "Give Focus a current value that is a whole number from -2147483648 to 2147483647."],
    ["-2147483649", "", "Give Focus a current value that is a whole number from -2147483648 to 2147483647."],
    ["1", "99999999999", "Give Focus a max that is a whole number from -2147483648 to 2147483647, or leave it empty for none."],
  ] as const) {
    document.body.innerHTML = "";
    const c = spendingConsole(stateWith({ "av-wide": { name: "Lera", abilityIds: [] } }));
    c.choose("av-wide", "flurry");
    fill(shownFields(c, "av-wide").current!, now);
    fill(shownFields(c, "av-wide").max!, top);
    c.give("av-wide")!.click();
    await settle();
    expect([now, top, c.sent.length, c.notices]).toEqual([now, top, 0, [notice]]);
  }
  document.body.innerHTML = "";
  const edge = spendingConsole(stateWith({ "av-edge": { name: "Lera", abilityIds: [] } }));
  edge.choose("av-edge", "flurry");
  fill(shownFields(edge, "av-edge").current!, "-2147483648");
  edge.give("av-edge")!.click();
  await settle();
  expect(edge.sent[0]!.command.value).toMatchObject({ resources: { focus: { current: -2147483648, max: 0 } } });
});

// VTT-807
test("values typed for a resource are not sent once an ability that asks none is chosen", async () => {
  const c = spendingConsole(stateWith({ "av-stale": { name: "Lera", abilityIds: [] } }));
  c.choose("av-stale", "flurry");
  fill(shownFields(c, "av-stale").current!, "4");
  c.choose("av-stale", "jab");
  c.give("av-stale")!.click();
  await settle();
  expect(Object.keys((c.sent[0]!.command.value as { resources: object }).resources)).toEqual([]);
});
