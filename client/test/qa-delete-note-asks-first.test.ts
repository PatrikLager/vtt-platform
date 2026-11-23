// Phase 4a QA: the console's Delete note asks the DM first.
// Derived from the register's rows VTT-956..963, not from the source.

import "./support/dom";
import { test, expect } from "bun:test";
import type {
  ClientCommand, CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { NoteVisibility } from "../../contract/gen/ts/vtt/v1/events_pb";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { newState, type Note, type State } from "../src/state";
import { held, ok, refused, settle } from "./support/answers";

type Send = (c: ClientCommand) => Promise<CommandResult>;
type Field = HTMLInputElement | HTMLTextAreaElement;

const NOTICE = "name the note to delete";

function question(n: string): string {
  return `Delete the note "${n}"? Its title and text go with it.`;
}

function note(title: string): Note {
  return {
    Title: title, Text: "body", UpdatedSeq: 1,
    Visibility: NoteVisibility.SECRET,
  };
}

function stateWith(notes: Record<string, string>): State {
  const st = newState();
  for (const [k, t] of Object.entries(notes)) st.Notes[k] = note(t);
  return st;
}

interface Rig {
  deps: DMDeps;
  sent: ClientCommand[];
  asked: string[];
  notices: string[];
  repaints: number;
  answers: boolean[];
  trail: string[];
}

function rig(st: State, send: Send = async () => ok()): Rig {
  const r: Rig = {
    deps: undefined as unknown as DMDeps,
    sent: [], asked: [], notices: [], repaints: 0, answers: [], trail: [],
  };
  r.deps = {
    st, participants: [], adventures: [], maps: [], abilities: [],
    guideFor: async () => null, joinLink: null, roster: null,
    origin: "http://localhost", refreshSharing: () => {},
    send: (c) => {
      r.sent.push(c);
      r.trail.push(`send:${c.command.case ?? ""}`);
      return send(c);
    },
    rerender: () => { r.repaints++; r.trail.push("repaint"); },
    notify: (m) => { r.notices.push(m); r.trail.push("notify"); },
    confirm: (m) => {
      r.asked.push(m);
      r.trail.push("ask");
      const a = r.answers.shift();
      if (a === undefined) throw new Error("confirm called with no answer queued");
      return a;
    },
    doorsArmed: false, toggleDoors: () => {},
  };
  return r;
}

function paint(d: DMDeps): HTMLElement {
  const el = renderDMConsole(d);
  document.body.replaceChildren(el);
  return el;
}

function field(el: HTMLElement, name: string): Field {
  const f = el.querySelector(`[data-field="${name}"]`);
  if (f === null) throw new Error(`no field ${name}`);
  return f as Field;
}

function type(el: HTMLElement, name: string, v: string): void {
  const f = field(el, name);
  f.value = v;
  f.dispatchEvent(new Event("input", { bubbles: true }));
}

function press(el: HTMLElement, action: string): void {
  const b = el.querySelector(`[data-action="${action}"]`);
  if (b === null) throw new Error(`no button ${action}`);
  (b as HTMLButtonElement).click();
}

function deletedKeys(r: Rig): string[] {
  const out: string[] = [];
  for (const c of r.sent) {
    if (c.command.case === "deleteNote") out.push(c.command.value.key);
  }
  return out;
}

function upserts(r: Rig): number {
  return r.sent.filter((c) => c.command.case === "upsertNote").length;
}

function choose(el: HTMLElement, v: string): void {
  const s = el.querySelector(".note-visibility");
  if (s === null) throw new Error("no visibility select");
  const sel = s as HTMLSelectElement;
  sel.value = v;
  sel.dispatchEvent(new Event("input", { bubbles: true }));
  sel.dispatchEvent(new Event("change", { bubbles: true }));
}

function empty(el: HTMLElement): void {
  for (const n of ["note-key", "note-title", "note-text"]) type(el, n, "");
  choose(el, "");
}

function formValues(el: HTMLElement): string[] {
  return ["note-key", "note-title", "note-text"].map((n) => field(el, n).value);
}

function buttonStates(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll("button")).map(
    (b) => `${b.textContent ?? ""}:${String(b.disabled)}`,
  );
}

// VTT-956 VTT-957
test("Delete note asks with the note's title, then sends one delete_note", async () => {
  const title = `The Duke's "secret" plan, part 2: (draft) & more!`;
  const r = rig(stateWith({ k1: title }));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    await settle();
    expect(r.asked).toEqual([question(title)]);
    expect(deletedKeys(r)).toEqual(["k1"]);
    expect(r.sent.length).toBe(1);
    expect(r.trail.slice(0, 2)).toEqual(["ask", "send:deleteNote"]);
  } finally {
    empty(el);
  }
});

// VTT-956
test("Delete note asks a note with an empty title by its key", async () => {
  const r = rig(stateWith({ "map notes 3": "" }));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "map notes 3");
    r.answers.push(false);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("map notes 3")]);
    expect(r.sent).toEqual([]);
  } finally {
    empty(el);
  }
});

// VTT-956 VTT-957
test("Delete note asks a key the state holds no note of by that key and sends it", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "ghost");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("ghost")]);
    expect(deletedKeys(r)).toEqual(["ghost"]);
  } finally {
    empty(el);
  }
});

// VTT-956 VTT-957
test("Delete note trims a padded key for the question and the command", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "  k1\t ");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("Grim Tidings")]);
    expect(deletedKeys(r)).toEqual(["k1"]);
  } finally {
    empty(el);
  }
});

// VTT-956 VTT-957
test("a note stored under a padded key lends no title to the trimmed key", async () => {
  const r = rig(stateWith({ "  k1  ": "Padded" }));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "  k1  ");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("k1")]);
    expect(deletedKeys(r)).toEqual(["k1"]);
  } finally {
    empty(el);
  }
});

// VTT-956
test("keys named like Object members are asked by the key unless a note holds them", async () => {
  const r = rig(stateWith({ valueOf: "Worth" }));
  const el = paint(r.deps);
  try {
    for (const k of ["constructor", "toString", "__proto__", "hasOwnProperty"]) {
      type(el, "note-key", k);
      r.answers.push(false);
      press(el, "delete-note");
    }
    type(el, "note-key", "valueOf");
    r.answers.push(false);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([
      question("constructor"), question("toString"), question("__proto__"),
      question("hasOwnProperty"), question("Worth"),
    ]);
    expect(r.sent).toEqual([]);
  } finally {
    empty(el);
  }
});

// VTT-956 VTT-958
test("a declined Delete note sends nothing and keeps the key as typed at the next paint", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }));
  const el = paint(r.deps);
  let next: HTMLElement | undefined;
  try {
    type(el, "note-key", "  k1  ");
    r.answers.push(false);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("Grim Tidings")]);
    expect(r.sent).toEqual([]);
    expect(field(el, "note-key").value).toBe("  k1  ");
    next = paint(r.deps);
    expect(field(next, "note-key").value).toBe("  k1  ");
  } finally {
    empty(next ?? el);
  }
});

// VTT-956 VTT-957
test("a declined question followed by an accepted one sends exactly one delete_note", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(false);
    press(el, "delete-note");
    await settle();
    expect(r.sent).toEqual([]);
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("Grim Tidings"), question("Grim Tidings")]);
    expect(deletedKeys(r)).toEqual(["k1"]);
    expect(r.sent.length).toBe(1);
  } finally {
    empty(el);
  }
});

// VTT-959
test("Delete note with a blank key shows the notice and asks nothing", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings", "": "Nameless" }));
  const el = paint(r.deps);
  try {
    for (const blank of ["", "   ", "\t \t"]) {
      r.notices.length = 0;
      type(el, "note-key", blank);
      press(el, "delete-note");
      await settle();
      expect(r.notices).toEqual([NOTICE]);
    }
    expect(r.asked).toEqual([]);
    expect(r.sent).toEqual([]);
  } finally {
    empty(el);
  }
});

// VTT-959
test("Delete note on a fresh console with nothing typed shows the notice", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }));
  const el = paint(r.deps);
  try {
    press(el, "delete-note");
    await settle();
    expect(r.notices).toEqual([NOTICE]);
    expect(r.asked).toEqual([]);
    expect(r.sent).toEqual([]);
  } finally {
    empty(el);
  }
});

// VTT-960 VTT-956
test("a second Delete note while one is unanswered sends nothing, and the answer frees it", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings", k2: "Second" }), h.send);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true, true, true);
    press(el, "delete-note");
    await settle();
    expect(deletedKeys(r)).toEqual(["k1"]);
    press(el, "delete-note");
    type(el, "note-key", "k2");
    press(el, "delete-note");
    await settle();
    expect(deletedKeys(r)).toEqual(["k1"]);
    const askedWhileHeld = r.asked.length;
    h.release(ok());
    await settle();
    r.answers.length = 0;
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked.length).toBe(askedWhileHeld + 1);
    expect(r.asked[r.asked.length - 1]).toBe(question("Second"));
    expect(deletedKeys(r)).toEqual(["k1", "k2"]);
  } finally {
    h.release(ok());
    await settle();
    empty(el);
  }
});

// VTT-960
test("Delete note pressed at a later paint while one is unanswered sends nothing", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings" }), h.send);
  const el = paint(r.deps);
  let next: HTMLElement | undefined;
  try {
    type(el, "note-key", "k1");
    r.answers.push(true, true);
    press(el, "delete-note");
    await settle();
    next = paint(r.deps);
    press(next, "delete-note");
    await settle();
    expect(deletedKeys(r)).toEqual(["k1"]);
  } finally {
    h.release(ok());
    await settle();
    empty(next ?? el);
  }
});

// VTT-960
test("a blank Delete note while one is unanswered sends nothing", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings" }), h.send);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    type(el, "note-key", "   ");
    press(el, "delete-note");
    await settle();
    expect(r.sent.length).toBe(1);
  } finally {
    h.release(ok());
    await settle();
    empty(el);
  }
});

// VTT-961
test("a Delete note that sends changes nothing drawn until it is answered", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings" }), h.send);
  const el = paint(r.deps);
  try {
    type(el, "note-key", " k1 ");
    type(el, "note-title", "A \"quoted\" title");
    type(el, "note-text", "Some text, with punctuation!");
    const html = document.body.innerHTML;
    const values = formValues(el);
    const buttons = buttonStates(el);
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    await settle();
    expect(r.sent.length).toBe(1);
    expect(document.body.innerHTML).toBe(html);
    expect(formValues(el)).toEqual(values);
    expect(buttonStates(el)).toEqual(buttons);
    expect(r.repaints).toBe(0);
    expect(formValues(paint(r.deps))).toEqual(values);
  } finally {
    h.release(ok());
    await settle();
    empty(el);
  }
});

// No row: D1 of docs/superpowers/plans/2026-11-23-delete-note-asks-first.md
// names this repaint a consequence of sending through `sending`.
test("an accepted Delete note asks for one repaint after the answer", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings" }), h.send);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.repaints).toBe(0);
    h.release(ok());
    await settle();
    await settle();
    expect(r.repaints).toBe(1);
  } finally {
    h.release(ok());
    await settle();
    empty(el);
  }
});

// VTT-962
test("a rejected delete_note clears nothing, asks no repaint, frees Delete note and logs", async () => {
  const boom = new Error("socket gone");
  let rejectNext = true;
  const r = rig(stateWith({ k1: "Grim Tidings" }), async () => {
    if (rejectNext) { rejectNext = false; throw boom; }
    return ok();
  });
  const errors: unknown[][] = [];
  const realError = console.error;
  console.error = (...args: unknown[]) => { errors.push(args); };
  const el = paint(r.deps);
  let next: HTMLElement | undefined;
  try {
    type(el, "note-key", "k1");
    type(el, "note-title", "Kept title");
    type(el, "note-text", "Kept text");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    await settle();
    expect(errors.some((args) => args.includes(boom))).toBe(true);
    expect(r.repaints).toBe(0);
    expect(formValues(el)).toEqual(["k1", "Kept title", "Kept text"]);
    next = paint(r.deps);
    expect(formValues(next)).toEqual(["k1", "Kept title", "Kept text"]);
    r.answers.push(true);
    press(next, "delete-note");
    await settle();
    expect(r.asked.length).toBe(2);
    expect(deletedKeys(r)).toEqual(["k1", "k1"]);
  } finally {
    console.error = realError;
    empty(next ?? el);
  }
});

// VTT-963
test("a refused delete_note asks for no repaint", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }), async () => refused("no note k1"));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    await settle();
    expect(deletedKeys(r)).toEqual(["k1"]);
    expect(r.repaints).toBe(0);
  } finally {
    empty(el);
  }
});

// VTT-956 VTT-957
test("after a refused delete_note the next Delete note asks and sends again", async () => {
  const r = rig(stateWith({ k1: "Grim Tidings" }), async () => refused("no note k1"));
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true, true);
    press(el, "delete-note");
    await settle();
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("Grim Tidings"), question("Grim Tidings")]);
    expect(deletedKeys(r)).toEqual(["k1", "k1"]);
  } finally {
    empty(el);
  }
});

// VTT-956 VTT-957 VTT-541
test("an unanswered Save does not hold Delete note back", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings" }), h.send);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    type(el, "note-title", "Grim Tidings");
    type(el, "note-text", "Rewritten");
    choose(el, "NOTE_VISIBILITY_SECRET");
    press(el, "upsert-note");
    await settle();
    expect(upserts(r)).toBe(1);
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("Grim Tidings")]);
    expect(deletedKeys(r)).toEqual(["k1"]);
  } finally {
    h.release(ok());
    await settle();
    empty(el);
  }
});

// VTT-541
test("an unanswered Delete note does not hold Save back", async () => {
  const h = held();
  const r = rig(stateWith({ k1: "Grim Tidings" }), h.send);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(deletedKeys(r)).toEqual(["k1"]);
    type(el, "note-title", "New title");
    type(el, "note-text", "New text");
    choose(el, "NOTE_VISIBILITY_PUBLIC");
    press(el, "upsert-note");
    await settle();
    expect(upserts(r)).toBe(1);
  } finally {
    h.release(ok());
    await settle();
    empty(el);
  }
});

// VTT-956 VTT-957
test("a note removed from the state after the paint is asked by its key", async () => {
  const st = stateWith({ k1: "Grim Tidings" });
  const r = rig(st);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    delete st.Notes["k1"];
    r.answers.push(true);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("k1")]);
    expect(deletedKeys(r)).toEqual(["k1"]);
  } finally {
    empty(el);
  }
});

// VTT-956
test("a note retitled in the state after the paint is asked by its new title", async () => {
  const st = stateWith({ k1: "Grim Tidings" });
  const r = rig(st);
  const el = paint(r.deps);
  try {
    type(el, "note-key", "k1");
    st.Notes["k1"] = note("Better Tidings");
    r.answers.push(false);
    press(el, "delete-note");
    await settle();
    expect(r.asked).toEqual([question("Better Tidings")]);
    expect(r.sent).toEqual([]);
  } finally {
    empty(el);
  }
});
