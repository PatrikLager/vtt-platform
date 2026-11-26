import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type Reference, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import type { CatalogueEntry, PlaceDef } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import { held, ok, refused, settle } from "./support/answers";

// Each test names its own actors: the console keeps what was typed and chosen,
// and which command is unanswered, across renders, keyed by the actor.

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

function stateWith(actors: Record<string, { name: string; references?: Record<string, Reference> }>): State {
  const st = newState();
  for (const [actorId, a] of Object.entries(actors)) {
    st.Actors[actorId] = {
      actorId, name: a.name, moduleId: "", attributes: {}, resources: {},
      controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER, abilityIds: [], references: a.references ?? {},
    };
  }
  return st;
}

interface Opts {
  catalogue?: readonly CatalogueEntry[] | undefined;
  places?: readonly PlaceDef[] | undefined;
  answers?: ReturnType<typeof held>;
  answer?: CommandResult;
  yes?: boolean;
}

function console_(st: State, o: Opts = { catalogue: CATALOGUE, places: PLACES }) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  const asked: string[] = [];
  const node = renderDMConsole({
    st, participants: [], adventures: [], maps: [], abilities: [],
    guideFor: async () => null, joinLink: null, roster: null, origin: "https://table.example",
    refreshSharing: () => {},
    send: async (c) => {
      sent.push(c);
      return o.answers === undefined ? (o.answer ?? ok()) : o.answers.send(c);
    },
    rerender: () => {}, notify: (m) => notices.push(m),
    confirm: (m) => {
      asked.push(m);
      return o.yes ?? true;
    },
    doorsArmed: false, toggleDoors: () => {}, catalogue: o.catalogue, places: o.places,
  });
  document.body.appendChild(node);
  const row = (actorId: string) => node.querySelector<HTMLElement>(`.reference-actor[data-actor="${actorId}"]`);
  const line = (actorId: string, referenceId: string) =>
    row(actorId)?.querySelector<HTMLElement>(`.reference-row[data-reference="${referenceId}"]`) ?? null;
  const set = (control: HTMLInputElement | HTMLSelectElement, value: string) => {
    control.value = value;
    control.dispatchEvent(new Event(control.tagName === "SELECT" ? "change" : "input"));
  };
  return {
    node, sent, notices, asked, row, line, set,
    labels: (actorId: string) =>
      Array.from(row(actorId)?.querySelectorAll(".reference-label") ?? [], (l) => l.textContent),
    adder: (actorId: string) => ({
      id: row(actorId)?.querySelector<HTMLInputElement>(`[data-field="add-reference-id:${actorId}"]`) ?? null,
      entry: row(actorId)?.querySelector<HTMLSelectElement>("select.add-reference-entry") ?? null,
      place: row(actorId)?.querySelector<HTMLSelectElement>("select.add-reference-place") ?? null,
      state: row(actorId)?.querySelector<HTMLInputElement>(`[data-field="add-reference-state:${actorId}"]`) ?? null,
      add: row(actorId)?.querySelector<HTMLButtonElement>('[data-action="add-reference"]') ?? null,
    }),
    changer: (actorId: string, referenceId: string) => ({
      place: line(actorId, referenceId)?.querySelector<HTMLSelectElement>("select.change-reference-place") ?? null,
      state: line(actorId, referenceId)?.querySelector<HTMLInputElement>(
        `[data-field="change-reference-state:${actorId}:${referenceId}"]`) ?? null,
      change: line(actorId, referenceId)?.querySelector<HTMLButtonElement>('[data-action="change-reference"]') ?? null,
      remove: line(actorId, referenceId)?.querySelector<HTMLButtonElement>('[data-action="remove-reference"]') ?? null,
    }),
  };
}

const options = (s: HTMLSelectElement | null) => Array.from(s?.options ?? [], (o) => [o.value, o.textContent]);

beforeEach(() => {
  document.body.innerHTML = "";
});

// VTT-1057
test("the Catalogue group has a row per actor by id, headed by its name else its id, its references by id, named by the window", () => {
  const c = console_(stateWith({ "cat-b": { name: "Bram", references: CARRIED }, "cat-a": { name: "" } }));
  const group = Array.from(c.node.querySelectorAll(".dmgroup")).find((g) => g.querySelector("h3")?.textContent === "Catalogue");
  expect(Array.from(group?.querySelectorAll<HTMLElement>(".reference-actor") ?? [], (r) => r.dataset["actor"])).toEqual(["cat-a", "cat-b"]);
  expect(Array.from(group?.querySelectorAll(".reference-actor > .reference-name") ?? [], (n) => n.textContent)).toEqual(["cat-a", "Bram"]);
  expect(c.labels("cat-b")).toEqual(["r-a: Delta Mail in Kilo Hook", "r-b: Echo Blade in Mike Rack (lima)", "r-c: Delta Mail in Mike Rack (oscar)"]);
  expect(c.labels("cat-a")).toEqual([]);
});

// VTT-1057
test("an entry or a place the window does not name is shown by its id", () => {
  const st = stateWith({ "cat-ids": { name: "Lera", references: CARRIED } });
  expect(console_(st, { catalogue: [CATALOGUE[0]!], places: [PLACES[0]!] }).labels("cat-ids"))
    .toEqual(["r-a: delta in kilo", "r-b: Echo Blade in Mike Rack (lima)", "r-c: delta in Mike Rack (oscar)"]);
  document.body.innerHTML = "";
  expect(console_(st, {}).labels("cat-ids")).toEqual(["r-a: delta in kilo", "r-b: echo in mike (lima)", "r-c: delta in mike (oscar)"]);
});

// VTT-1057
test("an addition offers the window's entries and places, named, and sends the id, entry, place and state as given", async () => {
  const c = console_(stateWith({ "cat-add": { name: "Lera" } }));
  const a = c.adder("cat-add");
  expect(options(a.entry)).toEqual([["", "choose an entry"], ["echo", "Echo Blade"], ["delta", "Delta Mail"]]);
  expect(options(a.place)).toEqual([["", "choose a place"], ["mike", "Mike Rack"], ["kilo", "Kilo Hook"]]);
  c.set(a.id!, " r-new ");
  c.set(a.entry!, "echo");
  c.set(a.place!, "mike");
  c.set(a.state!, " lima ");
  a.add!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("addReference");
  expect(c.sent[0]!.command.value).toMatchObject({
    actorId: "cat-add", referenceId: "r-new", reference: { entryId: "echo", place: "mike", state: "lima" },
  });
});

// VTT-1057
test("an addition with no state sends an empty state", async () => {
  const c = console_(stateWith({ "cat-add-bare": { name: "Lera" } }));
  const a = c.adder("cat-add-bare");
  c.set(a.id!, "r-hook");
  c.set(a.entry!, "delta");
  c.set(a.place!, "kilo");
  a.add!.click();
  await settle();
  expect(c.sent[0]!.command.value).toMatchObject({ referenceId: "r-hook", reference: { entryId: "delta", place: "kilo", state: "" } });
});

// VTT-1057
test("an addition with no id, no entry or no place says so and sends nothing", async () => {
  const cases: [string, Record<"id" | "entry" | "place", string>][] = [
    ["cat-no-id", { id: "  ", entry: "echo", place: "mike" }],
    ["cat-no-entry", { id: "r-1", entry: "", place: "mike" }],
    ["cat-no-place", { id: "r-1", entry: "echo", place: "" }],
  ];
  for (const [actor, given] of cases) {
    document.body.innerHTML = "";
    const c = console_(stateWith({ [actor]: { name: "Lera" } }));
    const a = c.adder(actor);
    c.set(a.id!, given.id);
    c.set(a.entry!, given.entry);
    c.set(a.place!, given.place);
    a.add!.click();
    await settle();
    expect([actor, c.sent.length, c.notices]).toEqual([actor, 0, ["a reference needs an id, an entry and a place"]]);
  }
});

// VTT-1057
test("with no entry or no place in the window no addition is offered, and each reference is still changed and removed", () => {
  for (const [i, o] of [{ places: PLACES }, { catalogue: CATALOGUE }, {}].entries()) {
    document.body.innerHTML = "";
    const actor = `cat-none-${i}`;
    const c = console_(stateWith({ [actor]: { name: "Lera", references: { "r-b": CARRIED["r-b"]! } } }), o);
    expect([i, c.adder(actor).add === null]).toEqual([i, true]);
    expect([i, c.changer(actor, "r-b").change !== null, c.changer(actor, "r-b").remove !== null]).toEqual([i, true, true]);
  }
  document.body.innerHTML = "";
  const empty = console_(newState());
  expect(Array.from(empty.node.querySelectorAll(".dmgroup h3"), (h) => h.textContent)).not.toContain("Catalogue");
});

// VTT-1057
test("a change shows the reference's place and state, offers the window's places, and sends them untouched", async () => {
  const c = console_(stateWith({ "cat-keep": { name: "Lera", references: { "r-a": CARRIED["r-a"]!, "r-b": CARRIED["r-b"]! } } }));
  const ch = c.changer("cat-keep", "r-b");
  expect(options(ch.place)).toEqual([["mike", "Mike Rack"], ["kilo", "Kilo Hook"]]);
  expect([ch.place!.value, ch.state!.value]).toEqual(["mike", "lima"]);
  expect([c.changer("cat-keep", "r-a").place!.value, c.changer("cat-keep", "r-a").state!.value]).toEqual(["kilo", ""]);
  ch.change!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("changeReference");
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "cat-keep", referenceId: "r-b", place: "mike", state: "lima" });
});

// VTT-1057
test("a change sends the place chosen and the state typed, whole", async () => {
  const c = console_(stateWith({ "cat-move": { name: "Lera", references: { "r-b": CARRIED["r-b"]! } } }));
  const ch = c.changer("cat-move", "r-b");
  c.set(ch.place!, "kilo");
  c.set(ch.state!, "  ");
  ch.change!.click();
  await settle();
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "cat-move", referenceId: "r-b", place: "kilo", state: "" });
  document.body.innerHTML = "";
  const d = console_(stateWith({ "cat-restate": { name: "Lera", references: { "r-b": CARRIED["r-b"]! } } }));
  d.set(d.changer("cat-restate", "r-b").state!, " oscar ");
  d.changer("cat-restate", "r-b").change!.click();
  await settle();
  expect(d.sent[0]!.command.value).toMatchObject({ referenceId: "r-b", place: "mike", state: "oscar" });
});

// VTT-1057
test("a reference in a place the window does not hold offers that place too, by its key", () => {
  const c = console_(stateWith({ "cat-odd": { name: "Lera", references: { "r-a": CARRIED["r-a"]! } } }), { catalogue: CATALOGUE, places: [PLACES[0]!] });
  const ch = c.changer("cat-odd", "r-a");
  expect(options(ch.place)).toEqual([["mike", "Mike Rack"], ["kilo", "kilo"]]);
  expect(ch.place!.value).toBe("kilo");
});

// VTT-1057 VTT-1074
test("what was typed and chosen for an addition or a change is kept across renders, and cleared once accepted", async () => {
  const st = stateWith({ "cat-kept": { name: "Lera", references: { "r-b": CARRIED["r-b"]! } } });
  const first = console_(st);
  const a = first.adder("cat-kept");
  first.set(a.id!, "r-new");
  first.set(a.entry!, "delta");
  first.set(a.place!, "kilo");
  first.set(a.state!, "oscar");
  first.set(first.changer("cat-kept", "r-b").place!, "kilo");
  first.set(first.changer("cat-kept", "r-b").state!, "");
  const again = console_(st, { catalogue: CATALOGUE, places: PLACES, answer: refused("no") });
  const b = again.adder("cat-kept");
  expect([b.id!.value, b.entry!.value, b.place!.value, b.state!.value]).toEqual(["r-new", "delta", "kilo", "oscar"]);
  expect([again.changer("cat-kept", "r-b").place!.value, again.changer("cat-kept", "r-b").state!.value]).toEqual(["kilo", ""]);
  b.add!.click();
  again.changer("cat-kept", "r-b").change!.click();
  await settle();
  const afterRefusal = console_(st);
  expect([afterRefusal.adder("cat-kept").id!.value, afterRefusal.changer("cat-kept", "r-b").place!.value]).toEqual(["r-new", "kilo"]);
  afterRefusal.adder("cat-kept").add!.click();
  await settle();
  const afterAddition = console_(st);
  const c = afterAddition.adder("cat-kept");
  expect([c.id!.value, c.entry!.value, c.place!.value, c.state!.value]).toEqual(["", "", "", ""]);
  const kept = afterAddition.changer("cat-kept", "r-b");
  expect([kept.place!.value, kept.state!.value]).toEqual(["kilo", ""]);
  afterAddition.set(c.id!, "r-next");
  kept.change!.click();
  await settle();
  expect([...afterRefusal.sent, ...afterAddition.sent].map((s) => s.command.case)).toEqual(["addReference", "changeReference"]);
  const afterChange = console_(st);
  const ch = afterChange.changer("cat-kept", "r-b");
  expect([ch.place!.value, ch.state!.value]).toEqual(["mike", "lima"]);
  expect(afterChange.adder("cat-kept").id!.value).toBe("r-next");
  afterChange.set(afterChange.adder("cat-kept").id!, "");
});

// VTT-1057
test("an addition or a change still unanswered holds back only itself", async () => {
  const answers = held();
  const st = stateWith({ "cat-lock": { name: "Lera", references: { "r-b": CARRIED["r-b"]!, "r-c": CARRIED["r-c"]! } } });
  const c = console_(st, { catalogue: CATALOGUE, places: PLACES, answers });
  try {
    const a = c.adder("cat-lock");
    c.set(a.id!, "r-new");
    c.set(a.entry!, "echo");
    c.set(a.place!, "mike");
    a.add!.click();
    a.add!.click();
    c.changer("cat-lock", "r-b").change!.click();
    c.changer("cat-lock", "r-b").change!.click();
    c.changer("cat-lock", "r-c").change!.click();
    expect(c.sent.map((s) => [s.command.case, (s.command.value as { referenceId: string }).referenceId]))
      .toEqual([["addReference", "r-new"], ["changeReference", "r-b"], ["changeReference", "r-c"]]);
  } finally {
    answers.release(refused("no"));
    await settle();
  }
  c.adder("cat-lock").add!.click();
  await settle();
  expect(c.sent).toHaveLength(4);
});

// VTT-1058
test("a removal asks first, naming the entry, the place and the actor, and a no sends nothing", async () => {
  const c = console_(stateWith({ "cat-no": { name: "Lera", references: CARRIED } }), { catalogue: CATALOGUE, places: PLACES, yes: false });
  c.changer("cat-no", "r-b").remove!.click();
  c.changer("cat-no", "r-a").remove!.click();
  await settle();
  expect(c.asked).toEqual([
    "Remove r-b: Echo Blade in Mike Rack from Lera? Its id and state go with it.",
    "Remove r-a: Delta Mail in Kilo Hook from Lera? Its id and state go with it.",
  ]);
  expect(c.sent).toHaveLength(0);
});

// VTT-1058
test("a removal answered yes sends one remove_reference naming the actor and the reference", async () => {
  const c = console_(stateWith({ "cat-yes": { name: "", references: CARRIED } }));
  c.changer("cat-yes", "r-c").remove!.click();
  await settle();
  expect(c.asked).toEqual(["Remove r-c: Delta Mail in Mike Rack from cat-yes? Its id and state go with it."]);
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("removeReference");
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "cat-yes", referenceId: "r-c" });
});

// VTT-1058
test("a removal still unanswered asks nothing and sends nothing when pressed again", async () => {
  const answers = held();
  const c = console_(stateWith({ "cat-twice": { name: "Lera", references: CARRIED } }), { catalogue: CATALOGUE, places: PLACES, answers });
  try {
    c.changer("cat-twice", "r-b").remove!.click();
    c.changer("cat-twice", "r-b").remove!.click();
    c.changer("cat-twice", "r-a").remove!.click();
    expect([c.asked.length, c.sent.length]).toEqual([2, 2]);
  } finally {
    answers.release(refused("no"));
    await settle();
  }
  c.changer("cat-twice", "r-b").remove!.click();
  await settle();
  expect([c.asked.length, c.sent.length]).toEqual([3, 3]);
});

// VTT-1057
test("an actor's Catalogue row is drawn while it holds a reference or an addition is offered, and the group only with a row", () => {
  const st = stateWith({ "cat-row-bare": { name: "Bare" }, "cat-row-held": { name: "Held", references: { "r-b": CARRIED["r-b"]! } } });
  const actors = (o: Opts) => {
    document.body.innerHTML = "";
    const c = console_(st, o);
    return Array.from(c.node.querySelectorAll<HTMLElement>(".reference-actor"), (r) => r.dataset["actor"]);
  };
  expect(actors({ catalogue: CATALOGUE, places: PLACES })).toEqual(["cat-row-bare", "cat-row-held"]);
  for (const o of [{}, { catalogue: CATALOGUE }, { places: PLACES }]) expect(actors(o)).toEqual(["cat-row-held"]);
  document.body.innerHTML = "";
  const bare = console_(stateWith({ "cat-row-only": { name: "Bare" } }), {});
  expect(Array.from(bare.node.querySelectorAll(".dmgroup h3"), (h) => h.textContent)).not.toContain("Catalogue");
  expect(Array.from(bare.node.querySelectorAll(".dmgroup h3"), (h) => h.textContent)).toContain("Abilities");
});

// VTT-1057 VTT-1058
test("each reference's row shows its id, so two copies of one entry in one place and state are told apart", async () => {
  const twin = { entryId: "echo", place: "mike", state: "lima" };
  const c = console_(stateWith({ "cat-twins": { name: "Lera", references: { "r-2": { ...twin }, "r-1": { ...twin } } } }));
  expect(c.labels("cat-twins")).toEqual(["r-1: Echo Blade in Mike Rack (lima)", "r-2: Echo Blade in Mike Rack (lima)"]);
  c.changer("cat-twins", "r-2").remove!.click();
  await settle();
  expect(c.asked).toEqual(["Remove r-2: Echo Blade in Mike Rack from Lera? Its id and state go with it."]);
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "cat-twins", referenceId: "r-2" });
});
