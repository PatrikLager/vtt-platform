import "./support/dom";
import { afterAll, describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  CommandResultSchema,
  type ClientCommand,
  type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { NoteVisibility } from "../../contract/gen/ts/vtt/v1/events_pb";
import { upsertNote } from "../src/commands";
import { newState, type Note, type State } from "../src/state";
import { renderDMConsole } from "../src/view/dm";
import { renderSpectator } from "../src/view/spectator";

// Independent QA for the ticket 2026-09-29-a-note-says-who-may-read-it,
// "Done looks like" item 7. Written from the requirements, the ticket and the
// contract only; the DOM hooks used here were observed by rendering.

interface Console {
  el: HTMLElement;
  sent: ClientCommand[];
  told: string[];
}

function dmConsole(): Console {
  const sent: ClientCommand[] = [];
  const told: string[] = [];
  const el = renderDMConsole({
    st: newState(),
    participants: [],
    adventures: [],
    maps: [],
    guideFor: async () => null,
    joinLink: null,
    roster: null,
    origin: "http://qa.invalid",
    refreshSharing: () => {},
    send: async (c: ClientCommand): Promise<CommandResult> => {
      sent.push(c);
      return create(CommandResultSchema, { requestId: c.requestId, ok: true });
    },
    notify: (msg: string) => {
      told.push(msg);
    },
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  });
  return { el, sent, told };
}

function one<T extends Element>(root: Element, selector: string): T {
  const found = root.querySelectorAll<T>(selector);
  expect(found.length).toBe(1);
  return found[0] as T;
}

function enter(control: HTMLInputElement | HTMLSelectElement, value: string): void {
  control.value = value;
  control.dispatchEvent(new Event("input"));
  control.dispatchEvent(new Event("change"));
}

const keyField = (c: Console) => one<HTMLInputElement>(c.el, '[data-field="note-key"]');
const titleField = (c: Console) => one<HTMLInputElement>(c.el, '[data-field="note-title"]');
const textField = (c: Console) => one<HTMLInputElement>(c.el, '[data-field="note-text"]');
const whoMayRead = (c: Console) => one<HTMLSelectElement>(c.el, "select.note-visibility");

const PUBLIC_CHOICE = "NOTE_VISIBILITY_PUBLIC";
const SECRET_CHOICE = "NOTE_VISIBILITY_SECRET";

// The console keeps its draft across renders and across test files in one bun
// process, so every test sets all four controls itself, the choice included.
function draft(c: Console, key: string, title: string, text: string, choice: string): void {
  enter(keyField(c), key);
  enter(titleField(c), title);
  enter(textField(c), text);
  enter(whoMayRead(c), choice);
}

async function save(c: Console): Promise<void> {
  one<HTMLButtonElement>(c.el, '[data-action="upsert-note"]').click();
  await new Promise((r) => setTimeout(r, 0));
}

function upserts(sent: ClientCommand[]) {
  return sent.flatMap((c) => (c.command.case === "upsertNote" ? [c.command.value] : []));
}

describe("the DM console's note form", () => {
  afterAll(() => draft(dmConsole(), "", "", "", ""));

  // VTT-229
  test("a fresh console shows no choice, and Save with a key and text sends no upsert_note", async () => {
    draft(dmConsole(), "", "", "", "");
    const c = dmConsole();
    expect(whoMayRead(c).value).toBe("");
    enter(keyField(c), "ravine");
    enter(titleField(c), "Ravine");
    enter(textField(c), "The trail is watched.");
    await save(c);
    expect(upserts(c.sent)).toEqual([]);
  });

  // VTT-229
  test("a choice taken back before Save sends no upsert_note", async () => {
    const c = dmConsole();
    draft(c, "ravine", "Ravine", "The trail is watched.", PUBLIC_CHOICE);
    enter(whoMayRead(c), "");
    await save(c);
    expect(upserts(c.sent)).toEqual([]);
  });

  // VTT-229
  test("choosing public sends PUBLIC with the key, title and text as typed", async () => {
    const c = dmConsole();
    draft(c, "ravine", "Ravine", "The trail is watched.", PUBLIC_CHOICE);
    await save(c);
    const sent = upserts(c.sent);
    expect(sent.length).toBe(1);
    expect(sent[0]?.visibility).toBe(NoteVisibility.PUBLIC);
    expect(sent[0]?.key).toBe("ravine");
    expect(sent[0]?.title).toBe("Ravine");
    expect(sent[0]?.text).toBe("The trail is watched.");
  });

  // VTT-229
  test("choosing DM only sends SECRET", async () => {
    const c = dmConsole();
    draft(c, "villain", "Villain", "The mayor is the cultist.", SECRET_CHOICE);
    await save(c);
    const sent = upserts(c.sent);
    expect(sent.length).toBe(1);
    expect(sent[0]?.visibility).toBe(NoteVisibility.SECRET);
    expect(sent[0]?.key).toBe("villain");
  });

  // VTT-229
  test("no option in the form sends UNSPECIFIED; public and DM only are both reachable", async () => {
    const values = Array.from(whoMayRead(dmConsole()).options).map((o) => o.value);
    const outcomes: (NoteVisibility | "nothing")[] = [];
    for (const v of values) {
      const c = dmConsole();
      draft(c, "k", "T", "body", v);
      await save(c);
      const sent = upserts(c.sent);
      expect(sent.length).toBeLessThanOrEqual(1);
      outcomes.push(sent[0]?.visibility ?? "nothing");
    }
    expect(outcomes).not.toContain(NoteVisibility.UNSPECIFIED);
    expect(outcomes).toContain(NoteVisibility.PUBLIC);
    expect(outcomes).toContain(NoteVisibility.SECRET);
    expect(outcomes.every((o) => o === "nothing" || o === NoteVisibility.PUBLIC || o === NoteVisibility.SECRET)).toBe(true);
  });

  // VTT-229
  test("a choice made for one note is not sent for the next note the DM writes", async () => {
    const first = dmConsole();
    draft(first, "ravine", "Ravine", "The trail is watched.", PUBLIC_CHOICE);
    await save(first);
    expect(upserts(first.sent).length).toBe(1);

    const next = dmConsole();
    enter(keyField(next), "villain");
    enter(titleField(next), "Villain");
    enter(textField(next), "The mayor is the cultist.");
    await save(next);
    expect(upserts(next.sent)).toEqual([]);
  });

  // Ticket, Done looks like 7: "The DM console asks for the visibility when
  // a note is saved"
  test("Save with no choice tells the DM something", async () => {
    const c = dmConsole();
    draft(c, "ravine", "Ravine", "The trail is watched.", "");
    await save(c);
    expect(upserts(c.sent)).toEqual([]);
    expect(c.told.length).toBeGreaterThan(0);
    expect(c.told.join("").trim()).not.toBe("");
  });
});

describe("upsertNote", () => {
  // VTT-229
  test("carries the visibility it is given", () => {
    for (const v of [NoteVisibility.PUBLIC, NoteVisibility.SECRET]) {
      const cmd = upsertNote("k", "T", "body", v);
      expect(cmd.command.case).toBe("upsertNote");
      expect(cmd.command.case === "upsertNote" ? cmd.command.value.visibility : undefined).toBe(v);
    }
  });
});

const DM_ONLY = /DM[\s-]only/i;

function note(title: string, text: string, visibility: NoteVisibility, seq: number): Note {
  return { Title: title, Text: text, UpdatedSeq: seq, Visibility: visibility };
}

function panelFor(st: State): HTMLElement {
  const root = document.createElement("div");
  document.body.appendChild(root);
  renderSpectator(root, st, [], "connected", {});
  return one<HTMLElement>(root, "section.notes");
}

function entryFor(panel: HTMLElement, text: string): HTMLElement {
  const hits = Array.from(panel.querySelectorAll<HTMLElement>("article")).filter((a) =>
    (a.textContent ?? "").includes(text),
  );
  expect(hits.length).toBe(1);
  return hits[0] as HTMLElement;
}

function marks(el: HTMLElement): number {
  return ((el.textContent ?? "").match(new RegExp(DM_ONLY.source, "gi")) ?? []).length;
}

function shown(el: HTMLElement, stop: HTMLElement): boolean {
  for (let n: HTMLElement | null = el; n !== null; n = n === stop ? null : n.parentElement) {
    if (n.hidden || n.style.display === "none" || n.style.visibility === "hidden") return false;
  }
  return true;
}

function markElement(entry: HTMLElement): HTMLElement {
  const carriers = Array.from(entry.querySelectorAll<HTMLElement>("*")).filter(
    (e) => DM_ONLY.test(e.textContent ?? "") && !Array.from(e.children).some((ch) => DM_ONLY.test(ch.textContent ?? "")),
  );
  expect(carriers.length).toBe(1);
  return carriers[0] as HTMLElement;
}

describe("the notes panel", () => {
  const dmState = (): State => {
    const st = newState();
    st.Notes["pub"] = note("Ravine trail", "Rocks overhang the path.", NoteVisibility.PUBLIC, 1);
    st.Notes["sec"] = note("The mayor", "He leads the cult.", NoteVisibility.SECRET, 2);
    st.Notes["none"] = note("Old ledger", "Recorded before visibility existed.", NoteVisibility.UNSPECIFIED, 3);
    return st;
  };

  // VTT-230
  test("marks a SECRET note and a note recorded with none as DM only, and not a PUBLIC one", () => {
    const panel = panelFor(dmState());
    expect(marks(entryFor(panel, "He leads the cult."))).toBe(1);
    expect(marks(entryFor(panel, "Recorded before visibility existed."))).toBe(1);
    expect(marks(entryFor(panel, "Rocks overhang the path."))).toBe(0);
    expect(marks(panel)).toBe(2);
  });

  // VTT-230
  test("the mark is text on the page, not an attribute, and not hidden", () => {
    const panel = panelFor(dmState());
    for (const text of ["He leads the cult.", "Recorded before visibility existed."]) {
      const entry = entryFor(panel, text);
      const mark = markElement(entry);
      expect(DM_ONLY.test(mark.textContent ?? "")).toBe(true);
      expect(shown(mark, entry)).toBe(true);
    }
  });

  // VTT-230
  test("a visibility this client does not know is marked DM only", () => {
    const st = newState();
    st.Notes["future"] = note("Later", "A value added after this client shipped.", 3 as NoteVisibility, 1);
    const panel = panelFor(st);
    expect(marks(entryFor(panel, "A value added after this client shipped."))).toBe(1);
  });

  // VTT-230
  test("a player's panel, holding only public notes, lists each and marks none", () => {
    const st = newState();
    st.Notes["pub"] = note("Ravine trail", "Rocks overhang the path.", NoteVisibility.PUBLIC, 1);
    st.Notes["pub2"] = note("Tavern", "The Drowned Rat serves stew.", NoteVisibility.PUBLIC, 2);
    const panel = panelFor(st);
    for (const [title, text] of [["Ravine trail", "Rocks overhang the path."], ["Tavern", "The Drowned Rat serves stew."]]) {
      const entry = entryFor(panel, text as string);
      expect(entry.textContent ?? "").toContain(title as string);
    }
    expect(marks(panel)).toBe(0);
  });
});
