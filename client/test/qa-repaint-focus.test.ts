import "./support/dom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { CommandResultSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { replaceKeepingFocus } from "../src/view/focus";
import { renderSpectator } from "../src/view/spectator";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { renderJoinView } from "../src/view/join";
import { boot } from "../src/app";

// Independent QA of a repaint keeping the box being typed in. Written
// from the ticket 2026-10-25-a-repaint-keeps-what-is-being-typed and
// VTT-491, VTT-492 and VTT-494 alone, without reading client/src.

type State = Parameters<typeof renderSpectator>[1];

const roots: HTMLElement[] = [];

function attachedRoot(): HTMLElement {
  const r = document.createElement("div");
  document.body.appendChild(r);
  roots.push(r);
  return r;
}

afterEach(() => {
  for (const r of roots.splice(0)) r.remove();
  const a = document.activeElement;
  if (a instanceof HTMLElement && a !== document.body) a.blur();
});

function box(
  tag: "input" | "textarea",
  field: string | null,
  value: string,
): HTMLInputElement | HTMLTextAreaElement {
  const el = document.createElement(tag);
  if (field !== null) el.setAttribute("data-field", field);
  el.value = value;
  return el;
}

function wrap(...kids: Node[]): HTMLElement {
  const d = document.createElement("div");
  d.append(...kids);
  return d;
}

function text(s: string): HTMLElement {
  const p = document.createElement("p");
  p.textContent = s;
  return p;
}

interface FocusCall {
  el: HTMLElement;
  opts: FocusOptions | undefined;
}

function spyFocus(): { calls: FocusCall[]; restore(): void } {
  const proto = HTMLElement.prototype;
  const orig = proto.focus;
  const calls: FocusCall[] = [];
  proto.focus = function (this: HTMLElement, opts?: FocusOptions): void {
    calls.push({ el: this, opts });
    orig.call(this, opts);
  };
  return { calls, restore: () => { proto.focus = orig; } };
}

function emptyState(): State {
  return {
    Scenes: {},
    Actors: {},
    Tokens: {},
    Sessions: [],
    Conditions: {},
    Notes: {},
  } as State;
}

function stateWithTokens(): State {
  const st = emptyState() as unknown as Record<string, unknown>;
  st["Scenes"] = {
    "qa-s1": { ID: "qa-s1", Name: "Cellar", GridWidth: 4, GridHeight: 4 },
  };
  st["Tokens"] = {
    "qa-t1": {
      ID: "qa-t1", SceneID: "qa-s1", ActorID: "qa-a1", X: 0, Y: 0, Name: "Rat",
    },
    "qa-t2": {
      ID: "qa-t2", SceneID: "qa-s1", ActorID: "qa-a2", X: 1, Y: 1, Name: "Bat",
    },
  };
  return st as unknown as State;
}

function withOpenSession(st: State): State {
  return {
    ...st,
    Sessions: [{ ID: "qa-sess", Name: "Open", StartSeq: 1, EndSeq: 0 }],
  } as State;
}

function deps(st: State): DMDeps {
  return {
    st,
    participants: [],
    adventures: [],
    maps: [],
    guideFor: async () => null,
    joinLink: null,
    roster: null,
    origin: "http://localhost:8080",
    refreshSharing: () => {},
    send: async () => create(CommandResultSchema, { ok: true }),
    rerender: () => {},
    notify: () => {},
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  };
}

function paintTable(root: HTMLElement, st: State): void {
  const console_ = renderDMConsole(deps(st));
  renderSpectator(root, st, [], "connected", {
    console: console_, dm: true, ticker: true,
  });
}

function field<T extends Element = HTMLInputElement>(
  root: ParentNode,
  name: string,
): T {
  const el = root.querySelector(`[data-field="${name}"]`);
  if (!el) throw new Error(`no [data-field="${name}"] in the tree`);
  return el as unknown as T;
}

function typeInto(el: HTMLInputElement | HTMLTextAreaElement, v: string): void {
  el.value = v;
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

function selectionOf(el: HTMLInputElement | HTMLTextAreaElement): unknown[] {
  return [el.selectionStart, el.selectionEnd, el.selectionDirection];
}

describe("qaRF: replaceKeepingFocus", () => {
  // VTT-491
  test("qaRF: a focused input's range and direction land on the new input", () => {
    const root = attachedRoot();
    const old = box("input", "who", "Open Table");
    root.append(wrap(text("before"), old));
    old.focus();
    old.setSelectionRange(2, 7, "backward");

    const fresh = box("input", "who", "Open Table");
    replaceKeepingFocus(root, text("first"), wrap(text("x"), fresh));

    expect(old.isConnected).toBe(false);
    expect(document.activeElement === fresh).toBe(true);
    expect(selectionOf(fresh)).toEqual([2, 7, "backward"]);
  });

  // VTT-491
  test("qaRF: a forward range in a textarea lands on the new textarea", () => {
    const root = attachedRoot();
    const old = box("textarea", "notes", "hello wide world");
    root.append(old, box("textarea", "other", "abc"));
    old.focus();
    old.setSelectionRange(3, 9, "forward");

    const other = box("textarea", "other", "abc");
    const fresh = box("textarea", "notes", "hello wide world");
    replaceKeepingFocus(root, other, fresh);

    expect(document.activeElement === fresh).toBe(true);
    expect(selectionOf(fresh)).toEqual([3, 9, "forward"]);
  });

  // VTT-491
  test("qaRF: a caret mid-text lands on the new box at the same place", () => {
    const root = attachedRoot();
    const old = box("input", "who", "Open");
    root.append(old);
    old.focus();
    old.setSelectionRange(2, 2);

    const fresh = box("input", "who", "Open");
    replaceKeepingFocus(root, fresh);

    expect(document.activeElement === fresh).toBe(true);
    expect(fresh.selectionStart).toBe(2);
    expect(fresh.selectionEnd).toBe(2);
  });

  // VTT-491
  test("qaRF: the box is found past a non-box carrying the same field", () => {
    const root = attachedRoot();
    const old = box("input", "who", "Open");
    root.append(old);
    old.focus();
    old.setSelectionRange(1, 3, "backward");

    const decoy = document.createElement("span");
    decoy.setAttribute("data-field", "who");
    decoy.tabIndex = 0;
    const fresh = box("input", "who", "Open");
    replaceKeepingFocus(root, decoy, fresh);

    expect(document.activeElement === fresh).toBe(true);
    expect(selectionOf(fresh)).toEqual([1, 3, "backward"]);
  });

  // VTT-491
  test("qaRF: a field holding a quote and a backslash still finds its box", () => {
    const root = attachedRoot();
    const name = `describe-name:o"b\\r]`;
    const old = box("input", name, "Rat");
    root.append(old);
    old.focus();
    old.setSelectionRange(1, 2, "backward");

    const fresh = box("input", name, "Rat");
    expect(() => replaceKeepingFocus(root, fresh)).not.toThrow();
    expect(document.activeElement === fresh).toBe(true);
    expect(selectionOf(fresh)).toEqual([1, 2, "backward"]);
  });

  // VTT-491
  test("qaRF: a focused input with no selection API is focused, nothing thrown", () => {
    const root = attachedRoot();
    const old = box("input", "count", "");
    (old as HTMLInputElement).type = "number";
    root.append(old);
    old.focus();

    const fresh = box("input", "count", "");
    (fresh as HTMLInputElement).type = "number";
    expect(() => replaceKeepingFocus(root, fresh)).not.toThrow();
    expect(document.activeElement === fresh).toBe(true);
  });

  // VTT-494
  test("qaRF: the box is given its focus without scrolling the page", () => {
    const root = attachedRoot();
    const old = box("input", "who", "Open");
    root.append(old);
    old.focus();

    const fresh = box("input", "who", "Open");
    const spy = spyFocus();
    try {
      replaceKeepingFocus(root, fresh);
    } finally {
      spy.restore();
    }
    const onFresh = spy.calls.filter((c) => c.el === fresh);
    expect(onFresh.length).toBeGreaterThan(0);
    for (const c of onFresh) expect(c.opts?.preventScroll).toBe(true);
  });

  // VTT-492
  test("qaRF: focus outside the root moves nowhere in the new tree", () => {
    const root = attachedRoot();
    const outside = box("input", "who", "elsewhere");
    document.body.appendChild(outside);
    roots.push(outside);
    root.append(box("input", "who", "Open"));
    outside.focus();

    const fresh = box("input", "who", "Open");
    const spy = spyFocus();
    try {
      replaceKeepingFocus(root, fresh);
    } finally {
      spy.restore();
    }
    expect(document.activeElement === outside).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: a focused input with no data-field focuses nothing new", () => {
    const root = attachedRoot();
    const old = box("input", null, "Open");
    root.append(old);
    old.focus();

    const spy = spyFocus();
    try {
      replaceKeepingFocus(root, box("input", null, "Open"), box("input", "who", ""));
    } finally {
      spy.restore();
    }
    expect(document.activeElement === document.body).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: a focused select carrying a data-field focuses nothing new", () => {
    const root = attachedRoot();
    const sel = document.createElement("select");
    sel.setAttribute("data-field", "kind");
    root.append(sel);
    sel.focus();
    expect(document.activeElement === sel).toBe(true);

    const freshSel = document.createElement("select");
    freshSel.setAttribute("data-field", "kind");
    const spy = spyFocus();
    try {
      replaceKeepingFocus(root, freshSel);
    } finally {
      spy.restore();
    }
    expect(document.activeElement === document.body).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: a focused button of the root focuses nothing new", () => {
    const root = attachedRoot();
    const b = document.createElement("button");
    b.setAttribute("data-field", "who");
    root.append(b, box("input", "who", ""));
    b.focus();
    expect(document.activeElement === b).toBe(true);

    const spy = spyFocus();
    try {
      replaceKeepingFocus(root, box("input", "who", ""));
    } finally {
      spy.restore();
    }
    expect(document.activeElement === document.body).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: the field held only by a non-box in the new tree is not focused", () => {
    const root = attachedRoot();
    const old = box("input", "who", "Open");
    root.append(old);
    old.focus();

    const decoy = document.createElement("div");
    decoy.setAttribute("data-field", "who");
    decoy.tabIndex = 0;
    const sel = document.createElement("select");
    sel.setAttribute("data-field", "who");
    const spy = spyFocus();
    try {
      replaceKeepingFocus(root, decoy, sel);
    } finally {
      spy.restore();
    }
    expect(document.activeElement === document.body).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: a field the new tree lacks leaves the body focused, no throw", () => {
    const root = attachedRoot();
    const old = box("input", "who", "Open");
    root.append(old);
    old.focus();

    const spy = spyFocus();
    try {
      expect(() =>
        replaceKeepingFocus(root, box("input", "what", ""), text("x")),
      ).not.toThrow();
    } finally {
      spy.restore();
    }
    expect(document.activeElement === document.body).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });
});

describe("qaRF: the table's repaint", () => {
  // VTT-491
  test("qaRF: the session-name box keeps its focus, text and caret", () => {
    const root = attachedRoot();
    paintTable(root, emptyState());
    const old = field(root, "session-name");
    old.focus();
    typeInto(old, "Open");
    const typed = document.activeElement as HTMLInputElement;
    expect(typed.getAttribute("data-field")).toBe("session-name");
    typed.setSelectionRange(2, 2);

    try {
      paintTable(root, emptyState());
      const fresh = field(root, "session-name");
      expect(fresh === typed).toBe(false);
      expect(document.activeElement === fresh).toBe(true);
      expect(fresh.value).toBe("Open");
      expect([fresh.selectionStart, fresh.selectionEnd]).toEqual([2, 2]);
    } finally {
      typeInto(field(root, "session-name"), "");
    }
  });

  // VTT-491
  test("qaRF: a console box's selected range survives the repaint", () => {
    const root = attachedRoot();
    const st = stateWithTokens();
    paintTable(root, st);
    const old = field<HTMLTextAreaElement>(root, "describe-notes:qa-t2");
    old.focus();
    typeInto(old, "the rat bites");
    const typed = document.activeElement as HTMLTextAreaElement;
    expect(typed.getAttribute("data-field")).toBe("describe-notes:qa-t2");
    typed.setSelectionRange(4, 7, "backward");

    try {
      paintTable(root, st);
      const fresh = field<HTMLTextAreaElement>(root, "describe-notes:qa-t2");
      expect(fresh === typed).toBe(false);
      expect(document.activeElement === fresh).toBe(true);
      expect(fresh.value).toBe("the rat bites");
      expect(selectionOf(fresh)).toEqual([4, 7, "backward"]);
    } finally {
      typeInto(field(root, "describe-notes:qa-t2"), "");
    }
  });

  // VTT-494
  test("qaRF: the table gives the focus back without scrolling", () => {
    const root = attachedRoot();
    paintTable(root, emptyState());
    const old = field(root, "session-name");
    old.focus();

    const spy = spyFocus();
    try {
      paintTable(root, emptyState());
    } finally {
      spy.restore();
    }
    const fresh = field(root, "session-name");
    const onFresh = spy.calls.filter((c) => c.el === fresh);
    expect(onFresh.length).toBeGreaterThan(0);
    for (const c of onFresh) expect(c.opts?.preventScroll).toBe(true);
  });

  // VTT-492
  test("qaRF: the session-name box gone once a session runs leaves body", () => {
    const root = attachedRoot();
    paintTable(root, emptyState());
    const old = field(root, "session-name");
    old.focus();
    expect(document.activeElement === old).toBe(true);

    const spy = spyFocus();
    try {
      expect(() => paintTable(root, withOpenSession(emptyState()))).not.toThrow();
    } finally {
      spy.restore();
    }
    expect(root.querySelector('[data-field="session-name"]') === null).toBe(true);
    expect(document.activeElement === document.body).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: focus outside the table's root stays where it is", () => {
    const root = attachedRoot();
    paintTable(root, emptyState());
    const outside = box("input", "session-name", "elsewhere");
    document.body.appendChild(outside);
    roots.push(outside);
    outside.focus();

    const spy = spyFocus();
    try {
      paintTable(root, emptyState());
    } finally {
      spy.restore();
    }
    expect(document.activeElement === outside).toBe(true);
    expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
  });

  // VTT-492
  test("qaRF: a focused console select or button focuses nothing new", () => {
    for (const pick of ["select", "button"]) {
      const root = attachedRoot();
      paintTable(root, emptyState());
      const el = root.querySelector(pick) as HTMLElement;
      expect(el.hasAttribute("data-field")).toBe(false);
      el.focus();
      expect(document.activeElement === el).toBe(true);

      const spy = spyFocus();
      try {
        paintTable(root, emptyState());
      } finally {
        spy.restore();
      }
      expect(document.activeElement === document.body).toBe(true);
      expect(spy.calls.filter((c) => root.contains(c.el)).map((c) => c.el.tagName)).toEqual([]);
    }
  });
});

describe("qaRF: the join form's repaint", () => {
  const handlers = { onName: () => {}, onSubmit: () => {} };

  // VTT-491
  test("qaRF: the name box keeps focus and range through busy and refusal", () => {
    const root = attachedRoot();
    renderJoinView(root, { name: "Alva", busy: false, error: "" }, handlers);
    const first = field(root, "join-name");
    first.focus();
    first.setSelectionRange(1, 3, "backward");

    renderJoinView(root, { name: "Alva", busy: true, error: "" }, handlers);
    const busy = field(root, "join-name");
    expect(busy === first).toBe(false);
    expect(document.activeElement === busy).toBe(true);
    expect(selectionOf(busy)).toEqual([1, 3, "backward"]);

    renderJoinView(root, { name: "Alva", busy: false, error: "refused" }, handlers);
    const refused = field(root, "join-name");
    expect(refused === busy).toBe(false);
    expect(document.activeElement === refused).toBe(true);
    expect(refused.value).toBe("Alva");
    expect(selectionOf(refused)).toEqual([1, 3, "backward"]);
  });

  // VTT-494
  test("qaRF: the join form gives the focus back without scrolling", () => {
    const root = attachedRoot();
    renderJoinView(root, { name: "Alva", busy: false, error: "" }, handlers);
    field(root, "join-name").focus();

    const spy = spyFocus();
    try {
      renderJoinView(root, { name: "Alva", busy: true, error: "" }, handlers);
    } finally {
      spy.restore();
    }
    const fresh = field(root, "join-name");
    const onFresh = spy.calls.filter((c) => c.el === fresh);
    expect(onFresh.length).toBeGreaterThan(0);
    for (const c of onFresh) expect(c.opts?.preventScroll).toBe(true);
  });

  // VTT-492
  test("qaRF: the Join button focused leaves nothing focused after", () => {
    const root = attachedRoot();
    renderJoinView(root, { name: "Alva", busy: false, error: "" }, handlers);
    const b = root.querySelector("button") as HTMLButtonElement;
    b.focus();
    expect(document.activeElement === b).toBe(true);

    renderJoinView(root, { name: "Alva", busy: false, error: "refused" }, handlers);
    expect(document.activeElement === document.body).toBe(true);
  });
});

// A fake socket and fetch for boot. The frames are protojson ServerFrames,
// the shapes a real `vtt serve` sends.

class FakeSocket {
  static all: FakeSocket[] = [];
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readyState = 0;
  url: string;
  sent: string[] = [];
  onopen: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: unknown) => void) | null = null;
  onclose: ((ev: unknown) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  private ls = new Map<string, ((ev: unknown) => void)[]>();

  constructor(url: string | URL) {
    this.url = String(url);
    FakeSocket.all.push(this);
    setTimeout(() => {
      this.readyState = 1;
      this.emit("open", {});
    }, 0);
  }

  addEventListener(t: string, fn: (ev: unknown) => void): void {
    this.ls.set(t, [...(this.ls.get(t) ?? []), fn]);
  }

  removeEventListener(t: string, fn: (ev: unknown) => void): void {
    this.ls.set(t, (this.ls.get(t) ?? []).filter((f) => f !== fn));
  }

  send(d: string): void {
    this.sent.push(String(d));
  }

  close(): void {
    this.readyState = 3;
  }

  emit(t: string, ev: unknown): void {
    const own = (this as unknown as Record<string, unknown>)[`on${t}`];
    if (typeof own === "function") (own as (e: unknown) => void)(ev);
    for (const f of this.ls.get(t) ?? []) f(ev);
  }

  frame(obj: unknown): void {
    this.emit("message", { data: JSON.stringify(obj) });
  }
}

const g = globalThis as unknown as Record<string, unknown>;
let realFetch: unknown;
let realSocket: unknown;
let realURL = "";
let joinGate: Promise<void> = Promise.resolve();

function json(v: unknown): Response {
  return new Response(JSON.stringify(v), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

async function fakeFetch(u: unknown): Promise<Response> {
  const path = new URL(String(u), "http://localhost:8080").pathname;
  switch (path) {
    case "/api/me":
      return json({ participantId: "qa-dm", name: "DM", role: "dm" });
    case "/api/ruleset":
      return json({ id: "qa-rules", name: "QA", abilities: [] });
    case "/api/adventures":
      return json({ adventures: [] });
    case "/api/maps":
      return json({ cellPx: 64, maps: [] });
    case "/api/join-link":
      return json({ open: false, secret: "s", admitted: 0, admitLimit: 0 });
    case "/api/participants":
      return json([]);
    case "/join":
      await joinGate;
      return new Response("gateway: this link is not accepting anyone", {
        status: 403,
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    default:
      return new Response("not found", { status: 404 });
  }
}

function happy(): { setURL(u: string): void } {
  const w = g["happyDOM"] ?? (g["window"] as Record<string, unknown>)["happyDOM"];
  return w as { setURL(u: string): void };
}

async function until(what: string, ok: () => boolean): Promise<void> {
  for (let i = 0; i < 200; i++) {
    if (ok()) return;
    await new Promise((r) => setTimeout(r, 5));
  }
  throw new Error(`timed out waiting for ${what}`);
}

describe("qaRF: boot's paints", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    realFetch = g["fetch"];
    realSocket = g["WebSocket"];
    realURL = location.href;
    g["fetch"] = fakeFetch;
    g["WebSocket"] = FakeSocket;
    FakeSocket.all = [];
    joinGate = Promise.resolve();
  });

  afterEach(() => {
    g["fetch"] = realFetch;
    g["WebSocket"] = realSocket;
    happy().setURL(realURL);
    localStorage.clear();
    sessionStorage.clear();
  });

  // VTT-491
  test("qaRF: an event's repaint keeps the DM's session-name box typed in", async () => {
    happy().setURL("http://localhost:8080/?token=qa-dm-token");
    const root = attachedRoot();
    const session = boot(root);
    expect(session).not.toBeNull();
    try {
      await until("the socket to open", () => FakeSocket.all[0]?.readyState === 1);
      const ws = FakeSocket.all[0]!;
      ws.frame({ catchUpHead: {} });
      await until("the console", () =>
        root.querySelector('[data-field="session-name"]') !== null);

      field(root, "session-name").focus();
      typeInto(field(root, "session-name"), "Open");
      const typed = document.activeElement as HTMLInputElement;
      expect(typed.getAttribute("data-field")).toBe("session-name");
      typed.setSelectionRange(2, 2);

      ws.frame({
        event: {
          eventId: "qa-e1",
          sequence: "1",
          actorRole: "dm",
          participantId: "qa-other",
          narrationAdded: { text: "A wind rises" },
        },
      });
      await until("the narration", () =>
        (root.textContent ?? "").includes("A wind rises"));

      const fresh = field(root, "session-name");
      expect(fresh === typed).toBe(false);
      expect(document.activeElement === fresh).toBe(true);
      expect(fresh.value).toBe("Open");
      expect([fresh.selectionStart, fresh.selectionEnd]).toEqual([2, 2]);
    } finally {
      const left = root.querySelector('[data-field="session-name"]');
      if (left) typeInto(left as HTMLInputElement, "");
      session?.close();
    }
  });

  // VTT-491
  test("qaRF: a presence frame's repaint keeps the DM's range", async () => {
    happy().setURL("http://localhost:8080/?token=qa-dm-token");
    const root = attachedRoot();
    const session = boot(root);
    try {
      await until("the socket to open", () => FakeSocket.all[0]?.readyState === 1);
      FakeSocket.all[0]!.frame({ catchUpHead: {} });
      await until("the console", () =>
        root.querySelector('[data-field="note-text"]') !== null);

      field(root, "note-text").focus();
      typeInto(field(root, "note-text"), "a cold draught");
      const typed = document.activeElement as HTMLInputElement;
      expect(typed.getAttribute("data-field")).toBe("note-text");
      typed.setSelectionRange(2, 6, "backward");

      FakeSocket.all[0]!.frame({
        presenceChanged: {
          participantId: "qa-p2",
          displayName: "Lera",
          state: "PRESENCE_STATE_CONNECTED",
        },
      });
      await until("the presence frame's repaint", () =>
        field(root, "note-text") !== typed);

      const fresh = field(root, "note-text");
      expect(document.activeElement === fresh).toBe(true);
      expect(fresh.value).toBe("a cold draught");
      expect(selectionOf(fresh)).toEqual([2, 6, "backward"]);
    } finally {
      const left = root.querySelector('[data-field="note-text"]');
      if (left) typeInto(left as HTMLInputElement, "");
      session?.close();
    }
  });

  // VTT-492
  test("qaRF: a session starting under the typed box leaves body focused", async () => {
    happy().setURL("http://localhost:8080/?token=qa-dm-token");
    const root = attachedRoot();
    const session = boot(root);
    const errors: unknown[] = [];
    const onErr = (e: unknown): void => { errors.push(e); };
    window.addEventListener("error", onErr);
    try {
      await until("the socket to open", () => FakeSocket.all[0]?.readyState === 1);
      const ws = FakeSocket.all[0]!;
      ws.frame({ catchUpHead: {} });
      await until("the console", () =>
        root.querySelector('[data-field="session-name"]') !== null);
      field(root, "session-name").focus();
      expect(document.activeElement === field(root, "session-name")).toBe(true);

      ws.frame({
        event: {
          eventId: "qa-e2",
          sequence: "1",
          sessionId: "qa-sess",
          actorRole: "dm",
          participantId: "qa-other",
          sessionStarted: { name: "Night" },
        },
      });
      await until("the End session button", () =>
        Array.from(root.querySelectorAll("button"))
          .some((b) => b.textContent === "End session"));

      expect(root.querySelector('[data-field="session-name"]') === null).toBe(true);
      expect(document.activeElement === document.body).toBe(true);
      expect(errors.map(String)).toEqual([]);
    } finally {
      window.removeEventListener("error", onErr);
      session?.close();
    }
  });

  // VTT-491
  test("qaRF: a refused join's repaint keeps the name box focused", async () => {
    happy().setURL("http://localhost:8080/?join=qa-secret");
    let open = (): void => {};
    joinGate = new Promise<void>((r) => { open = r; });
    const root = attachedRoot();
    expect(boot(root) === null).toBe(true);
    await until("the join form", () =>
      root.querySelector('[data-field="join-name"]') !== null);

    field(root, "join-name").focus();
    typeInto(field(root, "join-name"), "Alva");
    const typed = document.activeElement as HTMLInputElement;
    expect(typed.getAttribute("data-field")).toBe("join-name");
    typed.setSelectionRange(1, 3, "backward");
    typed.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));

    await until("the busy paint", () =>
      (root.querySelector("button") as HTMLButtonElement | null)?.disabled === true);
    const busy = field(root, "join-name");
    expect(document.activeElement === busy).toBe(true);

    open();
    await until("the refusal", () => root.querySelector(".error") !== null);
    const fresh = field(root, "join-name");
    expect(fresh === typed).toBe(false);
    expect(document.activeElement === fresh).toBe(true);
    expect(fresh.value).toBe("Alva");
    expect(selectionOf(fresh)).toEqual([1, 3, "backward"]);
  });
});
