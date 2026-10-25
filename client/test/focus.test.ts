import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { newState } from "../src/state";
import { renderDMConsole } from "../src/view/dm";
import { renderSpectator } from "../src/view/spectator";
import { CommandResultSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";

// A repaint rebuilds the table wholesale. These hold what it hands the new
// tree of the box that held the focus, through renderSpectator, the table's
// one painter.

function dmConsole(): HTMLElement {
  return renderDMConsole({
    st: newState(),
    participants: [],
    adventures: [],
    maps: [],
    guideFor: async () => null,
    joinLink: null,
    roster: null,
    origin: "https://table.example",
    refreshSharing: () => {},
    send: async () => create(CommandResultSchema, { ok: true }),
    notify: () => {},
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  });
}

let root: HTMLElement;

/** Draw the table with a fresh DM console, as app.ts's paint does. */
function repaint(): void {
  renderSpectator(root, newState(), [], "connected", { console: dmConsole() });
}

const box = (field: string) =>
  root.querySelector<HTMLInputElement | HTMLTextAreaElement>(`[data-field="${field}"]`)!;

/** Type into a box as a person does, so the console's draft follows it. */
function type(b: HTMLInputElement | HTMLTextAreaElement, text: string): void {
  b.value = text;
  b.dispatchEvent(new Event("input"));
}

beforeEach(() => {
  root = document.createElement("div");
  document.body.replaceChildren(root);
  repaint();
});

// VTT-491
test("a repaint hands the focused box's focus and backward selection to its successor", () => {
  // A textarea, and not the tree's first box, so that the textarea half of
  // each query and a pick of the first box are observable.
  const paste = box("actor-json") as HTMLTextAreaElement;
  paste.focus();
  type(paste, "abcdef");
  paste.setSelectionRange(1, 4, "backward");

  repaint();

  try {
    const next = box("actor-json") as HTMLTextAreaElement;
    expect(next === paste).toBe(false);
    expect(document.activeElement === next).toBe(true);
    expect(next.value).toBe("abcdef");
    expect([next.selectionStart, next.selectionEnd, next.selectionDirection])
      .toEqual([1, 4, "backward"]);
  } finally {
    // The console's draft outlives this test; leave it empty for later files.
    type(box("actor-json"), "");
  }
});

// VTT-492
test("a repaint leaves alone a focused box outside the root that shares a field", () => {
  const outside = document.createElement("input");
  outside.dataset["field"] = "session-name";
  document.body.appendChild(outside);
  outside.focus();

  repaint();

  expect(document.activeElement === outside).toBe(true);
  expect(root.querySelector(":focus") === null).toBe(true);
});

// VTT-492
test("a repaint gives no box the focus a button of the root held", () => {
  const start = root.querySelector<HTMLButtonElement>('[data-action="start-session"]')!;
  start.focus();
  expect(document.activeElement === start).toBe(true);

  repaint();

  expect(document.activeElement === document.body).toBe(true);
});

// VTT-494
test("a repaint focuses the successor without scrolling the page to it", () => {
  box("session-name").focus();
  const calls: unknown[] = [];
  const original = HTMLElement.prototype.focus;
  HTMLElement.prototype.focus = function (this: HTMLElement, options?: FocusOptions) {
    calls.push(options);
    original.call(this, options);
  };
  try {
    repaint();
  } finally {
    HTMLElement.prototype.focus = original;
  }

  expect(calls).toEqual([{ preventScroll: true }]);
  expect(document.activeElement === box("session-name")).toBe(true);
});
