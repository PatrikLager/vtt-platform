import "./support/dom";
import { describe, expect, test } from "bun:test";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { newState, type Actor, type State } from "../src/state";
import type { ConditionDef } from "../src/metadata";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type {
  ClientCommand, CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { held, ok, refused, settle } from "./support/answers";

const TAIL = "? Nothing in the console puts it back.";

function actor(id: string, name: string): Actor {
  return {
    actorId: id, name, moduleId: "", attributes: {}, resources: {},
    controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
    abilityIds: [], references: {},
  };
}

function world(
  actors: Record<string, string>, conds: Record<string, string[]>,
): State {
  const st = newState();
  for (const [id, name] of Object.entries(actors)) st.Actors[id] = actor(id, name);
  let seq = 1;
  for (const [id, cs] of Object.entries(conds)) {
    st.Conditions[id] = cs.map((c) => ({ ID: c, Source: "", AppliedSeq: seq++ }));
  }
  return st;
}

function def(id: string, name: string): ConditionDef {
  return { id, name, description: "" };
}

interface Rig {
  deps: DMDeps;
  sent: ClientCommand[];
  asked: string[];
  reply: { yes: boolean };
  repaints: { n: number };
  toggles: { n: number };
}

function rig(
  st: State,
  send: (c: ClientCommand) => Promise<CommandResult>,
  conditions?: readonly ConditionDef[],
): Rig {
  const sent: ClientCommand[] = [];
  const asked: string[] = [];
  const reply = { yes: true };
  const repaints = { n: 0 };
  const toggles = { n: 0 };
  const deps: DMDeps = {
    st, participants: [], adventures: [], maps: [], abilities: [],
    guideFor: async () => null, joinLink: null, roster: null,
    origin: "http://x", refreshSharing: () => {},
    send: (c) => { sent.push(c); return send(c); },
    rerender: () => { repaints.n++; },
    notify: () => {},
    confirm: (m) => { asked.push(m); return reply.yes; },
    doorsArmed: false, toggleDoors: () => { toggles.n++; },
  };
  if (conditions) deps.conditions = conditions;
  return { deps, sent, asked, reply, repaints, toggles };
}

function removals(el: HTMLElement): HTMLButtonElement[] {
  return Array.from(
    el.querySelectorAll<HTMLButtonElement>('button[data-action="remove-condition"]'),
  );
}

function labels(el: HTMLElement): string[] {
  return removals(el).map((b) => b.textContent ?? "").sort();
}

function button(el: HTMLElement, label: string): HTMLButtonElement {
  const found = removals(el).filter((b) => b.textContent === label);
  if (found.length !== 1) {
    throw new Error(`want one button "${label}", have ${JSON.stringify(labels(el))}`);
  }
  return found[0]!;
}

async function press(el: HTMLElement, label: string): Promise<void> {
  button(el, label).click();
  await settle();
}

function removal(c: ClientCommand | undefined): { actorId: string; conditionId: string } {
  const cmd = c?.command;
  if (cmd?.case !== "removeCondition") throw new Error(`not a removeCondition: ${cmd?.case}`);
  return { actorId: cmd.value.actorId, conditionId: cmd.value.conditionId };
}

function question(condition: string, who: string): string {
  return `Remove ${condition} from ${who}${TAIL}`;
}

const always = async (): Promise<CommandResult> => ok();

/** Every text field the console draws, so a test can type into it. */
function fields(el: HTMLElement): (HTMLInputElement | HTMLTextAreaElement)[] {
  const all = Array.from(
    el.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>("input, textarea"),
  );
  return all.filter((f) =>
    f instanceof HTMLTextAreaElement
    || ["text", "", "search", "url", "number"].includes(f.type));
}

function typeInto(el: HTMLElement): void {
  for (const [i, f] of fields(el).entries()) {
    f.value = f instanceof HTMLInputElement && f.type === "number" ? String(i + 1) : `typed ${i}`;
    f.dispatchEvent(new Event("input", { bubbles: true }));
  }
}

function emptyAll(el: HTMLElement): void {
  for (const f of fields(el)) {
    f.value = "";
    f.dispatchEvent(new Event("input", { bubbles: true }));
  }
}

function values(el: HTMLElement): string[] {
  return fields(el).map((f) => f.value);
}

describe("the Remove condition button's label and question", () => {
  // VTT-864, VTT-964: the label's names, and the question naming exactly them.
  test("a removal button reads the fold's actor name and the list's condition name, and asks in those names", async () => {
    const st = world(
      { "act-l1": "Lera", "act-b1": `Bo "the Bold", Jr.`, "act-h1": "<b>Ria</b> & co" },
      { "act-l1": ["dazed"], "act-b1": ["poisoned"], "act-h1": ["stunned"] },
    );
    const r = rig(st, always, [
      def("dazed", "Dazed"), def("poisoned", "Poisoned, badly!"),
      def("stunned", "<Stunned> & Prone"),
    ]);
    r.reply.yes = false;
    const el = renderDMConsole(r.deps);
    expect(labels(el)).toEqual([
      `<b>Ria</b> & co: <Stunned> & Prone ✕`,
      `Bo "the Bold", Jr.: Poisoned, badly! ✕`,
      "Lera: Dazed ✕",
    ].sort());
    await press(el, "Lera: Dazed ✕");
    await press(el, `Bo "the Bold", Jr.: Poisoned, badly! ✕`);
    await press(el, `<b>Ria</b> & co: <Stunned> & Prone ✕`);
    expect(r.asked).toEqual([
      "Remove Dazed from Lera? Nothing in the console puts it back.",
      question("Poisoned, badly!", `Bo "the Bold", Jr.`),
      question("<Stunned> & Prone", "<b>Ria</b> & co"),
    ]);
    expect(r.sent).toEqual([]);
  });

  // VTT-864, VTT-964: each fallback to an id, in the label and the question.
  test("a removal button falls back to the ids the fold and the list do not name, and asks in them", async () => {
    const st = world(
      { "act-l2": "Lera" },
      { "act-l2": ["undeclared-x", "blank"], "act-ghost2": ["dazed"] },
    );
    const r = rig(st, always, [def("dazed", "Dazed"), def("blank", "")]);
    r.reply.yes = false;
    const el = renderDMConsole(r.deps);
    expect(labels(el)).toEqual(
      ["Lera: undeclared-x ✕", "Lera: blank ✕", "act-ghost2: Dazed ✕"].sort(),
    );
    await press(el, "Lera: undeclared-x ✕");
    await press(el, "Lera: blank ✕");
    await press(el, "act-ghost2: Dazed ✕");
    expect(r.asked).toEqual([
      question("undeclared-x", "Lera"),
      question("blank", "Lera"),
      question("Dazed", "act-ghost2"),
    ]);
    expect(r.sent).toEqual([]);
  });

  // VTT-864, VTT-964: no condition list at all.
  test("with no condition list a removal button names the condition by its id, and asks in it", async () => {
    const st = world({ "act-l3": "Lera" }, { "act-l3": ["dazed"] });
    const r = rig(st, always);
    r.reply.yes = false;
    const el = renderDMConsole(r.deps);
    expect(labels(el)).toEqual(["Lera: dazed ✕"]);
    await press(el, "Lera: dazed ✕");
    expect(r.asked).toEqual([question("dazed", "Lera")]);
    expect(r.sent).toEqual([]);
  });

  // VTT-864, VTT-964, and D2 of
  // docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md:
  // whatever the label reads for an actor named "", the question names the
  // same two strings.
  test("an actor named empty is asked about in the words its button reads", async () => {
    const st = world({ "act-e4": "" }, { "act-e4": ["dazed"] });
    const r = rig(st, always, [def("dazed", "Dazed")]);
    r.reply.yes = false;
    const el = renderDMConsole(r.deps);
    const only = removals(el);
    expect(only.length).toBe(1);
    const label = only[0]!.textContent ?? "";
    expect(label.endsWith(": Dazed ✕")).toBe(true);
    const who = label.slice(0, label.length - ": Dazed ✕".length);
    only[0]!.click();
    await settle();
    expect(r.asked).toEqual([question("Dazed", who)]);
    expect(r.sent).toEqual([]);
    // VTT-864's "name, else its id" may read "" as a name or as none.
    expect(["", "act-e4"]).toContain(who);
  });
});

describe("asking first", () => {
  // VTT-964, VTT-965: a no sends nothing and holds nothing; a yes sends one.
  test("a declined removal sends nothing, and the next press asks again and sends on a yes", async () => {
    const st = world({ "act-l5": "Lera" }, { "act-l5": ["dazed"] });
    const r = rig(st, always, [def("dazed", "Dazed")]);
    const el = renderDMConsole(r.deps);
    r.reply.yes = false;
    await press(el, "Lera: Dazed ✕");
    expect(r.asked).toEqual([question("Dazed", "Lera")]);
    expect(r.sent).toEqual([]);
    expect(r.repaints.n).toBe(0);
    r.reply.yes = true;
    await press(el, "Lera: Dazed ✕");
    expect(r.asked).toEqual([question("Dazed", "Lera"), question("Dazed", "Lera")]);
    expect(r.sent.length).toBe(1);
    expect(removal(r.sent[0])).toEqual({ actorId: "act-l5", conditionId: "dazed" });
  });

  // VTT-965: the ids, not the names, go on the wire.
  test("an accepted removal sends the actor's id and the condition's id, never their names", async () => {
    const st = world({ "act-b6": "Bo, the Bold" }, { "act-b6": ["poisoned"] });
    const r = rig(st, always, [def("poisoned", "Poisoned!")]);
    const el = renderDMConsole(r.deps);
    await press(el, "Bo, the Bold: Poisoned! ✕");
    expect(r.asked).toEqual([question("Poisoned!", "Bo, the Bold")]);
    expect(r.sent.length).toBe(1);
    expect(removal(r.sent[0])).toEqual({ actorId: "act-b6", conditionId: "poisoned" });
  });
});

describe("the removal lock", () => {
  // VTT-966, and D1 and D3 of
  // docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md:
  // a held button asks nothing and sends nothing, at its own paint and at a
  // later one; an answer frees it.
  test("a held removal button asks nothing and sends nothing at any paint until it is answered", async () => {
    const st = world({ "act-l7": "Lera" }, { "act-l7": ["dazed"] });
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed")]);
    try {
      const el = renderDMConsole(r.deps);
      await press(el, "Lera: Dazed ✕");
      expect(r.asked.length).toBe(1);
      expect(r.sent.length).toBe(1);
      await press(el, "Lera: Dazed ✕");
      const later = renderDMConsole(r.deps);
      await press(later, "Lera: Dazed ✕");
      expect(r.asked.length).toBe(1);
      expect(r.sent.length).toBe(1);
      h.release(ok());
      await settle();
      await press(later, "Lera: Dazed ✕");
      expect(r.asked.length).toBe(2);
      expect(r.sent.length).toBe(2);
      expect(removal(r.sent[1])).toEqual({ actorId: "act-l7", conditionId: "dazed" });
    } finally {
      h.release();
      await settle();
    }
  });

  // VTT-541, and D3 of
  // docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md:
  // one lock per button.
  test("a held removal holds back neither another condition of that actor nor that condition on another actor", async () => {
    const st = world(
      { "act-l8": "Lera", "act-b8": "Bo" },
      { "act-l8": ["dazed", "poisoned"], "act-b8": ["dazed"] },
    );
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed"), def("poisoned", "Poisoned")]);
    try {
      const el = renderDMConsole(r.deps);
      await press(el, "Lera: Dazed ✕");
      await press(el, "Lera: Poisoned ✕");
      await press(el, "Bo: Dazed ✕");
      expect(r.asked).toEqual([
        question("Dazed", "Lera"), question("Poisoned", "Lera"), question("Dazed", "Bo"),
      ]);
      expect(r.sent.map(removal)).toEqual([
        { actorId: "act-l8", conditionId: "dazed" },
        { actorId: "act-l8", conditionId: "poisoned" },
        { actorId: "act-b8", conditionId: "dazed" },
      ]);
      const later = renderDMConsole(r.deps);
      await press(later, "Lera: Dazed ✕");
      await press(later, "Lera: Poisoned ✕");
      await press(later, "Bo: Dazed ✕");
      expect(r.asked.length).toBe(3);
      expect(r.sent.length).toBe(3);
    } finally {
      h.release();
      await settle();
    }
  });

  // VTT-541, and D3 of
  // docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md,
  // from the other side: the other actor first.
  test("a held removal on one actor leaves the same condition on another actor free at a later paint", async () => {
    const st = world(
      { "act-l9": "Lera", "act-b9": "Bo" },
      { "act-l9": ["dazed"], "act-b9": ["dazed"] },
    );
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed")]);
    try {
      await press(renderDMConsole(r.deps), "Bo: Dazed ✕");
      await press(renderDMConsole(r.deps), "Lera: Dazed ✕");
      expect(r.sent.map(removal)).toEqual([
        { actorId: "act-b9", conditionId: "dazed" },
        { actorId: "act-l9", conditionId: "dazed" },
      ]);
    } finally {
      h.release();
      await settle();
    }
  });

  // VTT-541: a held removal holds back no other console control. Every other
  // button is pressed, over typed fields, with and without a removal held,
  // and does the same; a refusal frees each.
  test("a held removal changes what no other console button does", async () => {
    const st = world({ "act-l10": "Lera" }, { "act-l10": ["dazed"] });
    const h = held();
    const answer = (c: ClientCommand): Promise<CommandResult> =>
      c.command.case === "removeCondition" ? h.send(c) : Promise.resolve(refused("no"));
    let typed: HTMLElement | undefined;
    const pressOthers = async (r: Rig): Promise<string[]> => {
      const el = renderDMConsole(r.deps);
      typed = el;
      typeInto(el);
      const others = Array.from(el.querySelectorAll<HTMLButtonElement>("button"))
        .filter((b) => b.dataset["action"] !== "remove-condition");
      const seen: string[] = [];
      for (const b of others) {
        const before = [r.sent.length, r.asked.length, r.toggles.n];
        b.click();
        await settle();
        const cases = r.sent.slice(before[0]).map((c) => c.command.case).join(",");
        seen.push(`${b.textContent}|${cases}|${r.asked.length - before[1]!}|${r.toggles.n - before[2]!}`);
      }
      return seen;
    };
    try {
      const free = rig(st, answer, [def("dazed", "Dazed")]);
      const baseline = await pressOthers(free);
      expect(baseline.filter((s) => s.split("|")[1] !== "").length).toBeGreaterThan(1);
      const r = rig(st, answer, [def("dazed", "Dazed")]);
      await press(renderDMConsole(r.deps), "Lera: Dazed ✕");
      expect(r.sent.length).toBe(1);
      expect(await pressOthers(r)).toEqual(baseline);
    } finally {
      h.release();
      await settle();
      if (typed) emptyAll(typed);
    }
  });
});

describe("other controls' locks", () => {
  // VTT-541, from the other side: an unanswered command of any other console
  // control holds back no removal button.
  test("no other console control's unanswered command holds back a removal button", async () => {
    const st = world({ "act-l15": "Lera" }, { "act-l15": ["dazed"] });
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed")]);
    let typed: HTMLElement | undefined;
    try {
      typed = renderDMConsole(r.deps);
      typeInto(typed);
      for (const b of Array.from(typed.querySelectorAll<HTMLButtonElement>("button"))) {
        if (b.dataset["action"] === "remove-condition") continue;
        b.click();
        await settle();
      }
      const others = r.sent.length;
      expect(others).toBeGreaterThan(1);
      const asked = r.asked.length;
      await press(renderDMConsole(r.deps), "Lera: Dazed ✕");
      expect(r.asked.slice(asked)).toEqual([question("Dazed", "Lera")]);
      expect(r.sent.length).toBe(others + 1);
      expect(removal(r.sent[others])).toEqual({ actorId: "act-l15", conditionId: "dazed" });
    } finally {
      h.release(refused("no"));
      await settle();
      if (typed) emptyAll(typed);
    }
  });
});

describe("what a removal draws and repaints", () => {
  // VTT-967, and D1 of
  // docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md:
  // nothing drawn changes until the answer, and an accepted removal then asks
  // for one repaint.
  test("an unanswered removal changes nothing drawn, and its acceptance asks for one repaint", async () => {
    const st = world(
      { "act-l11": "Lera", "act-b11": "Bo" },
      { "act-l11": ["dazed", "poisoned"], "act-b11": ["dazed"] },
    );
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed"), def("poisoned", "Poisoned")]);
    try {
      const el = renderDMConsole(r.deps);
      const drawn = el.outerHTML;
      await press(el, "Lera: Dazed ✕");
      expect(r.sent.length).toBe(1);
      expect(el.outerHTML).toBe(drawn);
      expect(renderDMConsole(r.deps).outerHTML).toBe(drawn);
      expect(r.repaints.n).toBe(0);
      h.release(ok());
      await settle();
      expect(r.repaints.n).toBe(1);
      expect(renderDMConsole(r.deps).outerHTML).toBe(drawn);
    } finally {
      h.release();
      await settle();
    }
  });

  // VTT-969: a refusal paints nothing of its own, clears nothing,
  // and frees the button.
  test("a refused removal asks for no repaint, clears no typed field, and frees its button", async () => {
    const st = world({ "act-l12": "Lera" }, { "act-l12": ["dazed"] });
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed")]);
    let typed: HTMLElement | undefined;
    try {
      typed = renderDMConsole(r.deps);
      typeInto(typed);
      const before = values(renderDMConsole(r.deps));
      r.repaints.n = 0;
      await press(typed, "Lera: Dazed ✕");
      h.release(refused("the server says no"));
      await settle();
      expect(r.repaints.n).toBe(0);
      expect(values(typed)).toEqual(before);
      expect(values(renderDMConsole(r.deps))).toEqual(before);
      await press(renderDMConsole(r.deps), "Lera: Dazed ✕");
      expect(r.asked.length).toBe(2);
      expect(r.sent.length).toBe(2);
    } finally {
      h.release();
      await settle();
      if (typed) emptyAll(typed);
    }
  });

  // An accepted removal clears nothing typed anywhere: D5 of
  // docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md.
  test("an accepted removal clears no field typed anywhere in the console", async () => {
    const st = world({ "act-l13": "Lera" }, { "act-l13": ["dazed"] });
    const h = held();
    const r = rig(st, h.send, [def("dazed", "Dazed")]);
    let typed: HTMLElement | undefined;
    try {
      typed = renderDMConsole(r.deps);
      expect(fields(typed).length).toBeGreaterThan(0);
      typeInto(typed);
      const before = values(renderDMConsole(r.deps));
      expect(before.some((v) => v !== "")).toBe(true);
      await press(typed, "Lera: Dazed ✕");
      h.release(ok());
      await settle();
      expect(r.sent.length).toBe(1);
      expect(values(typed)).toEqual(before);
      expect(values(renderDMConsole(r.deps))).toEqual(before);
    } finally {
      h.release();
      await settle();
      if (typed) emptyAll(typed);
    }
  });

  // VTT-968: a rejection clears nothing, paints nothing, frees the button,
  // and is reported with console.error, by the console and not happy-dom.
  test("a rejected removal clears nothing, asks for no repaint, frees its button and is reported", async () => {
    const st = world({ "act-l14": "Lera" }, { "act-l14": ["dazed"] });
    const boom = new Error("the socket fell over");
    let rejectNext = true;
    const send = (): Promise<CommandResult> => {
      if (rejectNext) { rejectNext = false; return Promise.reject(boom); }
      return Promise.resolve(ok());
    };
    const r = rig(st, send, [def("dazed", "Dazed")]);
    const reported: unknown[][] = [];
    const real = console.error;
    let windowErrors = 0;
    const onError = (): void => { windowErrors++; };
    window.addEventListener("error", onError);
    console.error = (...args: unknown[]) => { reported.push(args); };
    let typed: HTMLElement | undefined;
    try {
      typed = renderDMConsole(r.deps);
      typeInto(typed);
      const before = values(renderDMConsole(r.deps));
      const drawn = renderDMConsole(r.deps).outerHTML;
      r.repaints.n = 0;
      await press(typed, "Lera: Dazed ✕");
      await settle();
      expect(r.sent.length).toBe(1);
      expect(r.repaints.n).toBe(0);
      expect(windowErrors).toBe(0);
      expect(reported.length).toBeGreaterThan(0);
      expect(reported.some((args) => args.some((a) =>
        a === boom || (typeof a === "string" && a.includes(boom.message))))).toBe(true);
      expect(values(typed)).toEqual(before);
      expect(values(renderDMConsole(r.deps))).toEqual(before);
      expect(renderDMConsole(r.deps).outerHTML).toBe(drawn);
      await press(renderDMConsole(r.deps), "Lera: Dazed ✕");
      expect(r.asked.length).toBe(2);
      expect(r.sent.length).toBe(2);
      expect(removal(r.sent[1])).toEqual({ actorId: "act-l14", conditionId: "dazed" });
    } finally {
      console.error = real;
      window.removeEventListener("error", onError);
      await settle();
      if (typed) emptyAll(typed);
    }
  });
});
