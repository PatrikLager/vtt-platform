// Independent QA: what the DM console keeps when the server refuses a
// command, what it clears when the server accepts one, and the press that
// sends nothing while its control's command is unanswered.

import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import type {
  ClientCommand, CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { renderDMConsole } from "../src/view/dm";
import { newState, type State } from "../src/state";
import { held, ok, refused, settle } from "./support/answers";

type Answer = (c: ClientCommand) => Promise<CommandResult>;
type Box = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

/** A console over one world, counting what it sends, says and repaints. */
class Console {
  sent: ClientCommand[] = [];
  notified: string[] = [];
  rerenders = 0;
  root: HTMLElement;
  constructor(readonly st: State, public answer: Answer) {
    this.root = this.paint();
  }
  paint(): HTMLElement {
    this.root = renderDMConsole({
      st: this.st,
      participants: [
        { participantId: "p1", displayName: "Pia" },
        { participantId: "p2", displayName: "Per" },
      ],
      adventures: [],
      maps: [],
      guideFor: async () => null,
      joinLink: null,
      roster: null,
      origin: "http://table.test",
      refreshSharing: () => {},
      send: (c) => {
        this.sent.push(c);
        return this.answer(c);
      },
      rerender: () => {
        this.rerenders++;
      },
      notify: (m) => {
        this.notified.push(m);
      },
      confirm: () => true,
      doorsArmed: false,
      toggleDoors: () => {},
    });
    return this.root;
  }
  cases(): (string | undefined)[] {
    return this.sent.map((c) => c.command.case);
  }
}

/** One scene, one actor and one token of that actor, named for `sfx`. */
function world(sfx: string): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Cave", GridWidth: 8, GridHeight: 8 };
  for (const a of [`act-${sfx}`, `act2-${sfx}`]) {
    st.Actors[a] = {
      actorId: a, name: `Name of ${a}`, moduleId: "", attributes: {},
      resources: {}, controllerId: "", controllerIds: [], kind: 1,
    };
  }
  for (const t of [`tok-${sfx}`, `tok2-${sfx}`]) {
    st.Tokens[t] = {
      ID: t, SceneID: "s1", ActorID: `act-${sfx}`, X: 1, Y: 1,
      Name: `${t} shown`, Notes: `${t} notes`,
      GMName: `${t} gm`, GMNotes: `${t} gm notes`,
    };
  }
  return st;
}

function find(root: HTMLElement, sel: string): HTMLElement {
  const n = root.querySelector(sel);
  if (!n) throw new Error(`nothing matches ${sel}`);
  return n as HTMLElement;
}

function byField(field: string): (root: HTMLElement) => Box {
  return (root) => find(root, `[data-field="${field}"]`) as Box;
}

function fill(b: Box, v: string): void {
  b.value = v;
  const ev = b instanceof HTMLSelectElement ? "change" : "input";
  b.dispatchEvent(new Event(ev, { bubbles: true }));
}

interface Field {
  name: string;
  get: (root: HTMLElement) => Box;
  typed: string;
  retyped: string;
  /** What an accepted answer leaves; null keeps what was typed. */
  blank: string | null;
}

interface Control {
  name: string;
  sends: string;
  fields: Field[];
  press: (root: HTMLElement) => void;
}

function click(sel: string): (root: HTMLElement) => void {
  return (root) => find(root, sel).click();
}

const KIND_PM = "ACTOR_KIND_PARTY_MEMBER";
const KIND_NP = "ACTOR_KIND_NON_PARTY";

/** The nine controls, with ids made distinct by `sfx`. */
function controls(sfx: string): Control[] {
  const tok = `tok-${sfx}`;
  const act = `act-${sfx}`;
  const row = `.describe-row[data-token="${tok}"]`;
  const grant = `.control-actor[data-actor="${act}"]`;
  const json = (id: string) =>
    `{"actorId":"${id}","name":"Jay","kind":"${KIND_NP}"}`;
  const f = (
    name: string, typed: string, retyped: string, blank: string | null,
    get = byField(name),
  ): Field => ({ name, get, typed, retyped, blank });
  return [
    {
      name: "Start session", sends: "startSession",
      fields: [f("session-name", "Night One", "Night Two", "")],
      press: click('[data-action="start-session"]'),
    },
    {
      name: "Add actor", sends: "addActor",
      fields: [
        f("actor-id", `new-${sfx}`, `new2-${sfx}`, ""),
        f("actor-name", "Qa Added", "Qa Again", ""),
        f("actor-kind", KIND_NP, KIND_PM, "",
          (r) => find(r, "select.actor-kind") as Box),
      ],
      press: click('[data-action="add-actor"]'),
    },
    {
      name: "Remove actor", sends: "removeActor",
      fields: [f("actor-id", act, `act2-${sfx}`, "")],
      press: click('[data-action="remove-actor"]'),
    },
    {
      name: "Add from JSON", sends: "addActor",
      fields: [f("actor-json", json(`j-${sfx}`), json(`j2-${sfx}`), "")],
      press: (root) => {
        const b = Array.from(root.querySelectorAll("button"))
          .find((n) => n.textContent === "Add from JSON");
        if (!b) throw new Error("no Add from JSON button");
        b.click();
      },
    },
    {
      name: "Place token", sends: "placeToken",
      fields: [
        f("token-id", `placed-${sfx}`, `placed2-${sfx}`, ""),
        f("token-scene", "s1", "s1", ""),
        f("token-actor", act, `act2-${sfx}`, ""),
        f("token-x", "2", "4", ""),
        f("token-y", "3", "5", ""),
      ],
      press: click('[data-action="place-token"]'),
    },
    {
      name: "Remove token", sends: "removeToken",
      fields: [f("token-id", tok, `tok2-${sfx}`, "")],
      press: click('[data-action="remove-token"]'),
    },
    {
      name: "a row's Describe", sends: "describeToken",
      fields: [
        f(`describe-name:${tok}`, "Typed name", "Retyped name",
          `${tok} shown`),
        f(`describe-notes:${tok}`, "Typed notes", "Retyped notes",
          `${tok} notes`),
        f(`describe-gm-name:${tok}`, "Typed gm", "Retyped gm", `${tok} gm`),
        f(`describe-gm-notes:${tok}`, "Typed gm notes", "Retyped gm notes",
          `${tok} gm notes`),
      ],
      press: click(`${row} [data-action="describe-token"]`),
    },
    {
      name: "Save note", sends: "upsertNote",
      fields: [
        f("note-key", `key-${sfx}`, `key2-${sfx}`, null),
        f("note-title", "Qa title", "Qa title two", null),
        f("note-text", "Qa text", "Qa text two", null),
        f("note-visibility", "NOTE_VISIBILITY_PUBLIC",
          "NOTE_VISIBILITY_SECRET", "",
          (r) => find(r, "select.note-visibility") as Box),
      ],
      press: click('[data-action="upsert-note"]'),
    },
    {
      name: "a row's Grant", sends: "grantActorControl",
      fields: [
        f(`grant-${act}`, "p1", "p2", "",
          (r) => find(r, `${grant} select.grant-target`) as Box),
        f(`grant-kind-${act}`, KIND_PM, KIND_NP, "",
          (r) => find(r, `${grant} select.grant-kind`) as Box),
      ],
      press: click(`${grant} [data-action="grant-actor-control"]`),
    },
  ];
}

const NINE = controls("x").map((c) => c.name);

function controlNamed(name: string, sfx: string): Control {
  const c = controls(sfx).find((k) => k.name === name);
  if (!c) throw new Error(`no control ${name}`);
  return c;
}

function typeAll(root: HTMLElement, ctl: Control, which: "typed" | "retyped") {
  for (const fl of ctl.fields) fill(fl.get(root), fl[which]);
}

function read(root: HTMLElement, ctl: Control): Record<string, string> {
  return Object.fromEntries(ctl.fields.map((fl) => [fl.name, fl.get(root).value]));
}

function expected(
  ctl: Control, which: "typed" | "retyped" | "accepted",
  last: "typed" | "retyped" = "typed",
): Record<string, string> {
  return Object.fromEntries(ctl.fields.map((fl) => {
    if (which !== "accepted") return [fl.name, fl[which]];
    return [fl.name, fl.blank ?? fl[last]];
  }));
}

/** What a field shows with nothing typed in it, before a test types. */
function untouched(ctl: Control): Record<string, string> {
  return Object.fromEntries(ctl.fields.map((fl) => [
    fl.name,
    fl.blank ?? "",
  ]));
}

/** Leave no typed text another test could read. */
function reset(c: Console, ctl: Control): void {
  const root = c.paint();
  for (const fl of ctl.fields) fill(fl.get(root), "");
}

const holds: ReturnType<typeof held>[] = [];
const resets: (() => void)[] = [];

/** A held send that is answered, at the latest, when its test ends. */
function hold(): ReturnType<typeof held> {
  const h = held();
  holds.push(h);
  return h;
}

/** Wipe what `ctls` typed into `c` when the test ends, pass or fail. */
function tidy(c: Console, ...ctls: Control[]): void {
  resets.push(() => {
    for (const k of ctls) reset(c, k);
  });
}

afterEach(async () => {
  for (const h of holds.splice(0)) h.release(ok());
  await settle();
  for (const r of resets.splice(0)) r();
});

let n = 0;
const sfx = () => `qa${++n}`;

for (const why of ["the scene is busy", "wire: not connected"]) {
  for (const name of NINE) {
    // VTT-517
    test(`${name} refused (${why}) keeps each field as typed`, async () => {
      const s = sfx();
      const ctl = controlNamed(name, s);
      const c = new Console(world(s), async () => refused(why));
      tidy(c, ctl);
      expect(read(c.root, ctl)).toEqual(untouched(ctl));
      typeAll(c.root, ctl, "typed");
      ctl.press(c.root);
      await settle();
      expect(c.cases()).toEqual([ctl.sends]);
      expect(read(c.paint(), ctl)).toEqual(expected(ctl, "typed"));
    });
  }
}

for (const name of NINE) {
  // VTT-519
  test(`${name} accepted draws each field as with nothing typed`, async () => {
    const s = sfx();
    const ctl = controlNamed(name, s);
    const c = new Console(world(s), async () => ok());
    tidy(c, ctl);
    expect(read(c.root, ctl)).toEqual(untouched(ctl));
    typeAll(c.root, ctl, "typed");
    ctl.press(c.root);
    await settle();
    expect(c.cases()).toEqual([ctl.sends]);
    expect(read(c.paint(), ctl)).toEqual(expected(ctl, "accepted"));
  });
}

for (const name of NINE) {
  // VTT-519
  test(`${name} accepted asks the app to paint again`, async () => {
    const s = sfx();
    const ctl = controlNamed(name, s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, ctl);
    typeAll(c.root, ctl, "typed");
    ctl.press(c.root);
    await settle();
    const before = c.rerenders;
    h.release(ok());
    await settle();
    expect(c.rerenders).toBeGreaterThan(before);
  });
}

for (const name of NINE) {
  // VTT-523
  test(`${name} pressed again while unanswered sends nothing, then sends after ` +
    "each answer", async () => {
    const s = sfx();
    const ctl = controlNamed(name, s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, ctl);
    typeAll(c.root, ctl, "typed");
    ctl.press(c.root);
    await settle();
    expect(c.cases()).toEqual([ctl.sends]);
    expect(read(c.paint(), ctl)).toEqual(expected(ctl, "typed"));
    ctl.press(c.root);
    ctl.press(c.paint());
    await settle();
    expect(c.cases()).toEqual([ctl.sends]);
    h.release(refused("no"));
    await settle();
    ctl.press(c.paint());
    await settle();
    expect(c.cases()).toEqual([ctl.sends, ctl.sends]);
    h.release(ok());
    await settle();
    typeAll(c.paint(), ctl, "retyped");
    ctl.press(c.root);
    await settle();
    expect(c.cases()).toEqual([ctl.sends, ctl.sends, ctl.sends]);
    h.release(ok());
    await settle();
  });
}

for (const name of NINE) {
  // VTT-519
  test(`${name}: typed into while unanswered, an accepted answer still ` +
    "clears it", async () => {
    const s = sfx();
    const ctl = controlNamed(name, s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, ctl);
    typeAll(c.root, ctl, "typed");
    ctl.press(c.root);
    await settle();
    typeAll(c.paint(), ctl, "retyped");
    h.release(ok());
    await settle();
    expect(read(c.paint(), ctl)).toEqual(expected(ctl, "accepted", "retyped"));
  });
}

for (const name of NINE) {
  // VTT-517
  test(`${name}: typed into while unanswered, a refusal keeps the latest ` +
    "text", async () => {
    const s = sfx();
    const ctl = controlNamed(name, s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, ctl);
    typeAll(c.root, ctl, "typed");
    ctl.press(c.root);
    await settle();
    typeAll(c.paint(), ctl, "retyped");
    h.release(refused("no"));
    await settle();
    expect(read(c.paint(), ctl)).toEqual(expected(ctl, "retyped"));
  });
}

/** Every value, disabled bit and attribute of a paint. */
function look(root: HTMLElement): string {
  const all = Array.from(
    root.querySelectorAll("input,textarea,select,button"));
  return JSON.stringify([
    root.outerHTML,
    all.map((x) => (x as HTMLInputElement).value),
    all.map((x) => (x as HTMLInputElement).disabled),
  ]);
}

for (const name of NINE) {
  // VTT-524
  test(`${name}: the paint while unanswered is the paint before the press`,
    async () => {
      const s = sfx();
      const ctl = controlNamed(name, s);
      const h = hold();
      const c = new Console(world(s), h.send);
      tidy(c, ctl);
      typeAll(c.root, ctl, "typed");
      const before = look(c.paint());
      ctl.press(c.root);
      await settle();
      expect(c.cases()).toEqual([ctl.sends]);
      expect(look(c.paint())).toBe(before);
      h.release(ok());
      await settle();
    });
}

// VTT-523
test("a different control sends while Start session is unanswered",
  async () => {
    const s = sfx();
    const start = controlNamed("Start session", s);
    const add = controlNamed("Add actor", s);
    const remove = controlNamed("Remove actor", s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, start, add, remove);
    typeAll(c.root, start, "typed");
    start.press(c.root);
    typeAll(c.paint(), add, "typed");
    add.press(c.root);
    await settle();
    expect(c.cases()).toEqual(["startSession", "addActor"]);
    remove.press(c.root);
    await settle();
    expect(c.cases()).toEqual(["startSession", "addActor", "removeActor"]);
    h.release(ok());
    await settle();
  });

// VTT-523
test("another row's Describe sends while one row's is unanswered",
  async () => {
    const s = sfx();
    const h = hold();
    const c = new Console(world(s), h.send);
    const press = (t: string) => find(c.paint(),
      `.describe-row[data-token="${t}"] [data-action="describe-token"]`)
      .click();
    press(`tok-${s}`);
    await settle();
    press(`tok2-${s}`);
    await settle();
    expect(c.cases()).toEqual(["describeToken", "describeToken"]);
    const ids = c.sent.map((x) =>
      x.command.case === "describeToken" ? x.command.value.tokenId : "");
    expect(ids).toEqual([`tok-${s}`, `tok2-${s}`]);
    h.release(ok());
    await settle();
  });

// VTT-523
test("another row's Grant sends while one row's is unanswered", async () => {
  const s = sfx();
  const h = hold();
  const c = new Console(world(s), h.send);
  resets.push(() => {
    for (const a of [`act-${s}`, `act2-${s}`]) {
      const row = `.control-actor[data-actor="${a}"]`;
      fill(find(c.paint(), `${row} select.grant-target`) as Box, "");
      fill(find(c.root, `${row} select.grant-kind`) as Box, "");
    }
  });
  for (const a of [`act-${s}`, `act2-${s}`]) {
    const row = `.control-actor[data-actor="${a}"]`;
    fill(find(c.paint(), `${row} select.grant-target`) as Box, "p1");
    fill(find(c.root, `${row} select.grant-kind`) as Box, KIND_PM);
    find(c.root, `${row} [data-action="grant-actor-control"]`).click();
    await settle();
  }
  expect(c.cases()).toEqual(["grantActorControl", "grantActorControl"]);
  h.release(ok());
  await settle();
});

// VTT-523
test("a Start session the console refuses itself does not hold the next press",
  async () => {
    const s = sfx();
    const ctl = controlNamed("Start session", s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, ctl);
    ctl.press(c.root);
    await settle();
    expect(c.notified.length).toBe(1);
    expect(c.sent).toEqual([]);
    typeAll(c.paint(), ctl, "typed");
    ctl.press(c.root);
    await settle();
    expect(c.cases()).toEqual(["startSession"]);
    h.release(ok());
    await settle();
  });

// VTT-523
test("an Add actor the console refuses itself does not hold the next press",
  async () => {
    const s = sfx();
    const ctl = controlNamed("Add actor", s);
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, ctl);
    fill(byField("actor-id")(c.root), `new-${s}`);
    ctl.press(c.root);
    await settle();
    expect(c.notified.length).toBe(1);
    expect(c.sent).toEqual([]);
    typeAll(c.paint(), ctl, "typed");
    ctl.press(c.root);
    await settle();
    expect(c.cases()).toEqual(["addActor"]);
    h.release(ok());
    await settle();
  });

// VTT-517
test("a refused Start session keeps Night One at the paint after the answer",
  async () => {
    const s = sfx();
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, controlNamed("Start session", s));
    fill(byField("session-name")(c.root), "Night One");
    find(c.root, '[data-action="start-session"]').click();
    await settle();
    h.release(refused("a session is already open"));
    await settle();
    expect(byField("session-name")(c.paint()).value).toBe("Night One");
  });

// VTT-519 VTT-524
test("an accepted Start session draws session-name empty at the next paint",
  async () => {
    const s = sfx();
    const h = hold();
    const c = new Console(world(s), h.send);
    tidy(c, controlNamed("Start session", s));
    fill(byField("session-name")(c.root), "Night One");
    find(c.root, '[data-action="start-session"]').click();
    await settle();
    expect(byField("session-name")(c.paint()).value).toBe("Night One");
    h.release(ok());
    await settle();
    expect(byField("session-name")(c.paint()).value).toBe("");
  });
