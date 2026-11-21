// Independent QA: a board click outside the current scene's grid sends no
// command, whatever the window's role and whether its doors are armed
// (VTT-934). Beside the clicks off the grid, clicks inside it are the control
// that shows the window would have sent something: a player's move (VTT-408),
// an armed DM's or agent's door work (VTT-876, VTT-888), and an unarmed DM's
// or agent's move (SPEC-022's chips sentence, quoted where it is asserted).
//
// "The ticket" below is docs/superpowers/specs/'s
// 2026-11-21-a-click-moves-only-onto-a-seen-open-square-design.md.
//
// Each window is booted once and clicked in sequence: the client's mutation
// run executes this file once per mutant. Every per-click result is asserted
// in a table keyed by the side and the point, so a failure names the click.

import "./support/dom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { fromJsonString } from "@bufbuild/protobuf";
import { ClientCommandSchema, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { cellFromPoint, type Cell, type Geometry } from "../src/view/grid";
import { worldFromScreen, type Camera } from "../src/view/camera";
import { boardCamera, CELL, renderSpectator } from "../src/view/spectator";
import { newState, type State } from "../src/state";
import { boot } from "../src/app";

// ---------------------------------------------------------------- fixtures

interface Fixture {
  label: string;
  sceneId: string;
  w: number;
  h: number;
  /** One door on each edge of the grid, beside which the off-grid clicks land. */
  doors: Record<Side, Cell>;
  /** Where the acting actor's token stands. */
  token: Cell;
  /** A square inside the grid that holds no door and no token. */
  interior: Cell;
}

type Side = "left" | "right" | "top" | "bottom";
const SIDES: Side[] = ["left", "right", "top", "bottom"];

/** Wider than the pane's ratio: the slack is above and below the grid. */
const WIDE: Fixture = {
  label: "a 10x6 scene",
  sceneId: "s-wide",
  w: 10,
  h: 6,
  doors: { left: { x: 0, y: 2 }, right: { x: 9, y: 2 }, top: { x: 4, y: 0 }, bottom: { x: 4, y: 5 } },
  token: { x: 2, y: 3 },
  interior: { x: 6, y: 3 },
};

/** Taller than the pane's ratio: the slack is left and right of the grid. */
const TALL: Fixture = {
  label: "a 5x9 scene",
  sceneId: "s-tall",
  w: 5,
  h: 9,
  doors: { left: { x: 0, y: 4 }, right: { x: 4, y: 4 }, top: { x: 2, y: 0 }, bottom: { x: 2, y: 8 } },
  token: { x: 1, y: 6 },
  interior: { x: 3, y: 2 },
};

const FIXTURES = [WIDE, TALL];

interface Point { x: number; y: number }

interface Probe {
  side: Side;
  /** The door square on that edge. */
  door: Cell;
  /** Points off the grid beside that door: at the edge, and half a square out. */
  off: Point[];
  /** Half a screen pixel inside the same edge, on the door square. */
  inside: Point;
}

/** What each of a probe's off points is, in the order `off` holds them. */
const OFF_POINTS = ["at the edge", "half a square out"] as const;

/** The camera `boardCamera` gives `f`'s scene in the default pane. */
function cameraFor(f: Fixture): Camera {
  return boardCamera(f.w, f.h);
}

/** Screen geometry: a square at `offset + index * CELL * scale`. */
function screenOf(f: Fixture) {
  const cam = cameraFor(f);
  const sq = CELL * cam.scale;
  const left = cam.offsetX;
  const top = cam.offsetY;
  const right = left + f.w * sq;
  const bottom = top + f.h * sq;
  const midX = (c: number) => left + (c + 0.5) * sq;
  const midY = (r: number) => top + (r + 0.5) * sq;
  return { cam, sq, left, top, right, bottom, midX, midY };
}

function centreOf(f: Fixture, c: Cell): Point {
  const s = screenOf(f);
  return { x: s.midX(c.x), y: s.midY(c.y) };
}

function probesFor(f: Fixture): Probe[] {
  const s = screenOf(f);
  const d = f.doors;
  return [
    {
      side: "left", door: d.left,
      off: [{ x: s.left - 0.5, y: s.midY(d.left.y) }, { x: s.left - s.sq / 2, y: s.midY(d.left.y) }],
      inside: { x: s.left + 0.5, y: s.midY(d.left.y) },
    },
    {
      side: "right", door: d.right,
      off: [{ x: s.right, y: s.midY(d.right.y) }, { x: s.right + s.sq / 2, y: s.midY(d.right.y) }],
      inside: { x: s.right - 0.5, y: s.midY(d.right.y) },
    },
    {
      side: "top", door: d.top,
      off: [{ x: s.midX(d.top.x), y: s.top - 0.5 }, { x: s.midX(d.top.x), y: s.top - s.sq / 2 }],
      inside: { x: s.midX(d.top.x), y: s.top + 0.5 },
    },
    {
      side: "bottom", door: d.bottom,
      off: [{ x: s.midX(d.bottom.x), y: s.bottom }, { x: s.midX(d.bottom.x), y: s.bottom + s.sq / 2 }],
      inside: { x: s.midX(d.bottom.x), y: s.bottom - 0.5 },
    },
  ];
}

function probe(f: Fixture, side: Side): Probe {
  const p = probesFor(f).find((q) => q.side === side);
  if (!p) throw new Error(`no probe for ${side}`);
  return p;
}

/** The points a window is clicked at beside the grid: every side's off points. */
function offClicks(f: Fixture): { label: string; at: Point }[] {
  return SIDES.flatMap((side) =>
    probe(f, side).off.map((at, i) => ({ label: `${side}, ${OFF_POINTS[i] ?? i}`, at })));
}

/** The table an off-grid run must produce: no frame at any click. */
function nothingAtEveryOffClick(): [string, number][] {
  return SIDES.flatMap((side) => OFF_POINTS.map((which): [string, number] => [`${side}, ${which}`, 0]));
}

/** Every square floor, but one door on each edge. */
function tilesFor(f: Fixture): Record<string, { kind: string; material: string; art: string }> {
  const doors = new Set(Object.values(f.doors).map((c) => `${c.x},${c.y}`));
  const tiles: Record<string, { kind: string; material: string; art: string }> = {};
  for (let y = 0; y < f.h; y++) {
    for (let x = 0; x < f.w; x++) {
      const key = `${x},${y}`;
      tiles[key] = { kind: doors.has(key) ? "door" : "floor", material: "stone", art: "" };
    }
  }
  return tiles;
}

function clickBoard(root: HTMLElement, at: Point): void {
  const grid = root.querySelector<HTMLElement>(".grid");
  if (!grid) throw new Error("the window draws no .grid");
  grid.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, clientX: at.x, clientY: at.y }));
}

/** Let every queued continuation run: microtasks, then one macrotask. */
async function settle(): Promise<void> {
  for (let j = 0; j < 5; j++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

/** The longer wait a boot's chain of reads needs. */
async function flush(): Promise<void> {
  for (let i = 0; i < 4; i++) await settle();
}

// ------------------------------------------------------- globals and seams

class FakeSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static made: FakeSocket[] = [];

  readyState = 0;
  readonly sent: string[] = [];
  onopen: ((ev?: unknown) => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: ((ev?: unknown) => void) | null = null;
  onerror: ((ev?: unknown) => void) | null = null;

  constructor(readonly url: string) {
    FakeSocket.made.push(this);
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(): void {
    this.readyState = FakeSocket.CLOSED;
  }
}

interface Me { participantId: string; name: string; role: "player" | "dm" | "agent" | "spectator" }

function fakeFetch(me: Me): typeof fetch {
  const answer = (body: unknown) =>
    new Response(JSON.stringify(body), { status: 200, headers: { "content-type": "application/json" } });
  const f = async (input: unknown): Promise<Response> => {
    const raw = typeof input === "string" ? input : input instanceof URL ? input.href : (input as { url: string }).url;
    const path = new URL(raw, "http://qa.invalid").pathname;
    if (path.endsWith("/api/me")) return answer(me);
    if (path.endsWith("/api/ruleset")) return answer({ id: "qa", name: "QA", abilities: [], conditions: [], resources: [] });
    if (path.endsWith("/api/adventures")) return answer({ adventures: [] });
    return new Response("not found", { status: 404 });
  };
  return f as unknown as typeof fetch;
}

const TOKEN = "tok-qa-click-off-grid";

let savedFetch: typeof fetch;
let savedSocket: typeof WebSocket;
let localBefore: Set<string>;
let sessionBefore: Set<string>;
let mounted: HTMLElement[] = [];

function keysOf(s: Storage): Set<string> {
  const out = new Set<string>();
  for (let i = 0; i < s.length; i++) {
    const k = s.key(i);
    if (k !== null) out.add(k);
  }
  return out;
}

beforeEach(() => {
  savedFetch = globalThis.fetch;
  savedSocket = globalThis.WebSocket;
  localBefore = keysOf(localStorage);
  sessionBefore = keysOf(sessionStorage);
  FakeSocket.made = [];
});

afterEach(() => {
  globalThis.fetch = savedFetch;
  globalThis.WebSocket = savedSocket;
  for (const k of keysOf(localStorage)) if (!localBefore.has(k)) localStorage.removeItem(k);
  for (const k of keysOf(sessionStorage)) if (!sessionBefore.has(k)) sessionStorage.removeItem(k);
  for (const root of mounted) root.remove();
  mounted = [];
  FakeSocket.made = [];
});

function mount(): HTMLElement {
  const root = document.createElement("div");
  document.body.appendChild(root);
  mounted.push(root);
  return root;
}

// ------------------------------------------------------------ the windows

let seq = 0;
function event(payload: Record<string, unknown>): Record<string, unknown> {
  seq += 1;
  return {
    event: {
      eventId: `e-qa-${seq}`, sequence: String(seq), sessionId: "sess-qa",
      actorRole: "dm", participantId: "p-dm", ...payload,
    },
  };
}

function deliver(ws: FakeSocket, frame: unknown): void {
  if (!ws.onmessage) throw new Error("boot set no onmessage");
  ws.onmessage({ data: JSON.stringify(frame) });
}

interface Table {
  root: HTMLElement;
  ws: FakeSocket;
  /** How many frames the window has sent so far. */
  mark(): number;
  /** The commands sent since `mark`. */
  sentSince(mark: number): ClientCommand[];
  /** Answer every command sent since `mark`: accepted, or refused so a refusal is drawn. */
  answer(mark: number, accepted: boolean): Promise<void>;
}

async function openWindow(me: Me): Promise<{ root: HTMLElement; ws: FakeSocket }> {
  const root = mount();
  localStorage.setItem("vtt.token", TOKEN);
  globalThis.fetch = fakeFetch(me);
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  boot(root);
  await flush();
  const ws = FakeSocket.made.at(-1);
  if (!ws) throw new Error("boot opened no socket");
  ws.readyState = FakeSocket.OPEN;
  ws.onopen?.();
  await flush();
  return { root, ws };
}

function asTable(root: HTMLElement, ws: FakeSocket): Table {
  const parse = (from: number) => ws.sent.slice(from).map((s) => fromJsonString(ClientCommandSchema, s));
  return {
    root,
    ws,
    mark: () => ws.sent.length,
    sentSince: parse,
    async answer(from, accepted) {
      for (const c of parse(from)) {
        deliver(ws, accepted
          ? { result: { requestId: c.requestId, ok: true } }
          : { result: { requestId: c.requestId, ok: false, error: "QA refusal: nothing there" } });
      }
      await settle();
    },
  };
}

/** A larger scene whose id sorts below both fixtures', so it is never current. */
const EARLIER = { sceneId: "s-a-earlier", w: 20, h: 14 };

function sceneEvents(f: Fixture, withEarlier: boolean): Record<string, unknown>[] {
  const earlier = withEarlier
    ? [event({ sceneCreated: { sceneId: EARLIER.sceneId, name: "an earlier, larger scene", gridWidth: EARLIER.w, gridHeight: EARLIER.h } })]
    : [];
  return [
    event({ sessionStarted: { name: "QA night" } }),
    ...earlier,
    event({ sceneCreated: { sceneId: f.sceneId, name: f.label, gridWidth: f.w, gridHeight: f.h, tiles: tilesFor(f) } }),
  ];
}

/** A player whose one party member, Hero, stands on `f.token`, with every door open. */
async function playerWindow(f: Fixture, withEarlier = false): Promise<Table> {
  seq = 0;
  const { root, ws } = await openWindow({ participantId: "p-alice", name: "Alice", role: "player" });
  deliver(ws, { watchableChanged: { actors: [{ actorId: "a-hero", name: "Hero" }] } });
  deliver(ws, { viewpointChanged: { actorId: "a-hero" } });
  for (const e of [
    ...sceneEvents(f, withEarlier),
    event({ actorAdded: { actor: { actorId: "a-hero", name: "Hero", kind: "ACTOR_KIND_PARTY_MEMBER" } } }),
    event({ actorControlGranted: { actorId: "a-hero", participantId: "p-alice", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
    event({ tokenPlaced: { tokenId: "t-hero", sceneId: f.sceneId, actorId: "a-hero", position: f.token } }),
    ...SIDES.map((side) => event({ doorOpened: { sceneId: f.sceneId, at: f.doors[side] } })),
  ]) deliver(ws, e);
  await flush();
  return asTable(root, ws);
}

/** A spectator perched on Hero, whom they do not control. */
async function spectatorWindow(f: Fixture): Promise<Table> {
  seq = 0;
  const { root, ws } = await openWindow({ participantId: "p-sam", name: "Sam", role: "spectator" });
  deliver(ws, { watchableChanged: { actors: [{ actorId: "a-hero", name: "Hero" }], controlsNoPartyMember: true } });
  deliver(ws, { viewpointChanged: { actorId: "a-hero" } });
  for (const e of [
    ...sceneEvents(f, false),
    event({ actorAdded: { actor: { actorId: "a-hero", name: "Hero", kind: "ACTOR_KIND_PARTY_MEMBER" } } }),
    event({ tokenPlaced: { tokenId: "t-hero", sceneId: f.sceneId, actorId: "a-hero", position: f.token } }),
  ]) deliver(ws, e);
  await flush();
  return asTable(root, ws);
}

/**
 * A DM's or an agent's window holding the world, with Goblin granted to it and
 * selected by its chip, and the bottom door open.
 */
async function consoleWindow(f: Fixture, role: "dm" | "agent", withEarlier = false): Promise<Table> {
  seq = 0;
  const pid = role === "dm" ? "p-dm" : "p-agent";
  const { root, ws } = await openWindow({ participantId: pid, name: role === "dm" ? "Dana" : "Agent", role });
  for (const e of [
    ...sceneEvents(f, withEarlier),
    event({ actorAdded: { actor: { actorId: "a-goblin", name: "Goblin", kind: "ACTOR_KIND_NON_PARTY" } } }),
    event({ actorControlGranted: { actorId: "a-goblin", participantId: pid, kind: "ACTOR_KIND_NON_PARTY" } }),
    event({ tokenPlaced: { tokenId: "t-goblin", sceneId: f.sceneId, actorId: "a-goblin", position: f.token } }),
    event({ doorOpened: { sceneId: f.sceneId, at: f.doors.bottom } }),
  ]) deliver(ws, e);
  await flush();
  const chip = Array.from(root.querySelectorAll<HTMLButtonElement>("button"))
    .find((b) => b.textContent?.trim() === "Goblin");
  if (!chip) throw new Error("the window draws no chip for Goblin");
  chip.click();
  await settle();
  return asTable(root, ws);
}

async function armDoors(win: Table): Promise<void> {
  const toggle = win.root.querySelector<HTMLElement>('.dm [data-action="arm-doors"]');
  if (!toggle) throw new Error("the console draws no arm-doors toggle");
  toggle.click();
  await settle();
}

function moveOf(c: ClientCommand | undefined): { tokenId: string; to: Cell } | undefined {
  if (c?.command.case !== "moveToken") return undefined;
  return { tokenId: c.command.value.tokenId, to: { x: c.command.value.to?.x ?? 0, y: c.command.value.to?.y ?? 0 } };
}

function doorOf(c: ClientCommand | undefined): { kind: string; sceneId: string; at: Cell; actorId: string } | undefined {
  if (c?.command.case !== "openDoor" && c?.command.case !== "closeDoor") return undefined;
  const v = c.command.value;
  return { kind: c.command.case, sceneId: v.sceneId, at: { x: v.at?.x ?? 0, y: v.at?.y ?? 0 }, actorId: v.actorId };
}

/**
 * Click every off-grid point in turn and return, per click, how many frames it
 * sent, and how many all of them sent after a longer wait. Then refuse
 * anything sent, so a refusal would be drawn, and say whether the window is
 * drawn as it was before the first click.
 */
async function clickBesideTheGrid(win: Table, f: Fixture): Promise<{ table: [string, number][]; total: number; unchanged: boolean }> {
  const start = win.mark();
  const before = win.root.innerHTML;
  const table: [string, number][] = [];
  for (const { label, at } of offClicks(f)) {
    const m = win.mark();
    clickBoard(win.root, at);
    await settle();
    table.push([label, win.sentSince(m).length]);
  }
  await flush();
  const total = win.sentSince(start).length;
  await win.answer(start, false);
  return { table, total, unchanged: win.root.innerHTML === before };
}

/**
 * Click half a pixel inside each edge in turn and return, per side, how many
 * frames the click sent and what `read` makes of the first. Each command is
 * answered as accepted before the next click; no event follows, so the
 * window's state is the same at every click.
 */
async function clickInsideEachEdge(win: Table, f: Fixture, read: (c: ClientCommand | undefined) => unknown): Promise<[Side, number, unknown][]> {
  const table: [Side, number, unknown][] = [];
  for (const side of SIDES) {
    const m = win.mark();
    clickBoard(win.root, probe(f, side).inside);
    await settle();
    const sent = win.sentSince(m);
    table.push([side, sent.length, read(sent[0])]);
    await win.answer(m, true);
  }
  return table;
}

/** Click one square and return how many frames it sent and what `read` makes of the first. */
async function clickSquare(win: Table, at: Point, read: (c: ClientCommand | undefined) => unknown): Promise<[number, unknown]> {
  const m = win.mark();
  clickBoard(win.root, at);
  await settle();
  const sent = win.sentSince(m);
  const out: [number, unknown] = [sent.length, read(sent[0])];
  await win.answer(m, true);
  return out;
}

/**
 * How many of the Story's lines read "a door opened": the window holds every
 * door it drew such a line for open (VTT-820, VTT-871).
 */
function doorsOpenedInTheStory(win: Table): number {
  const feed = win.root.querySelector(".feed");
  return Array.from(feed?.querySelectorAll("p") ?? []).filter((p) => p.textContent?.trim() === "a door opened").length;
}

/** What a run of clicks inside each edge must produce when each moves `tokenId` onto that edge's door square. */
function movesOntoEachDoor(f: Fixture, tokenId: string): [Side, number, unknown][] {
  return SIDES.map((side): [Side, number, unknown] => [side, 1, { tokenId, to: probe(f, side).door }]);
}

// ===================================================== the fixtures' shape

describe("the fixtures are not degenerate", () => {
  // `boardCamera` must draw each fixture at a scale other than 1 with slack on
  // one axis only, or the clicks below run on a degenerate board.
  for (const f of FIXTURES) {
    test(`${f.label} is drawn at a scale other than 1, offset on exactly one axis`, () => {
      const s = screenOf(f);
      expect(f.w).not.toBe(f.h);
      expect(s.cam.scale).not.toBe(1);
      const offsets = [s.cam.offsetX, s.cam.offsetY].filter((o) => Math.abs(o) > 1e-6);
      expect(offsets).toHaveLength(1);
      expect(s.right).toBeLessThanOrEqual(640 + 1e-6);
      expect(s.bottom).toBeLessThanOrEqual(480 + 1e-6);
      expect(Math.max(s.right - s.left, s.bottom - s.top)).toBeGreaterThan(0);
    });
  }
  test("the slack lies on a different axis in each fixture", () => {
    expect(cameraFor(WIDE).offsetY).toBeGreaterThan(1);
    expect(cameraFor(WIDE).offsetX).toBeCloseTo(0, 6);
    expect(cameraFor(TALL).offsetX).toBeGreaterThan(1);
    expect(cameraFor(TALL).offsetY).toBeCloseTo(0, 6);
  });
});

// ===================================================== the board's geometry

describe("a square's screen position and worldFromScreen", () => {
  // The clicks below aim at `offset + index * CELL * scale` under
  // `boardCamera`'s camera; `worldFromScreen` must map that back to the
  // square's world pixels.
  for (const f of FIXTURES) {
    test(`worldFromScreen inverts offset + index * CELL * scale on ${f.label}`, () => {
      const s = screenOf(f);
      for (const [i, j] of [[0, 0], [1, 2], [f.w - 1, f.h - 1], [f.w, f.h]] as const) {
        const w = worldFromScreen(s.left + i * s.sq, s.top + j * s.sq, s.cam);
        expect(w.x).toBeCloseTo(i * CELL, 6);
        expect(w.y).toBeCloseTo(j * CELL, 6);
      }
    });
  }
});

// =============================================================== cellFromPoint

describe("cellFromPoint", () => {
  // VTT-934; the ticket's "Done looks like" item 1. A square spans
  // [index, index + 1) in world pixels, so the column at exactly GridWidth and
  // the row at exactly GridHeight are off the grid.
  for (const f of FIXTURES) {
    const geom: Geometry = { cell: CELL, width: f.w, height: f.h };
    const W = f.w * CELL;
    const H = f.h * CELL;

    test(`${f.label}: the first and the last square are on the grid`, () => {
      expect(cellFromPoint(0, 0, geom)).toEqual({ x: 0, y: 0 });
      expect(cellFromPoint(W - 0.001, H - 0.001, geom)).toEqual({ x: f.w - 1, y: f.h - 1 });
      expect(cellFromPoint(CELL - 0.001, CELL, geom)).toEqual({ x: 0, y: 1 });
      expect(cellFromPoint(2.5 * CELL, 1.5 * CELL, geom)).toEqual({ x: 2, y: 1 });
    });

    // VTT-934
    test(`${f.label}: a point exactly at GridWidth is off the grid`, () => {
      expect(cellFromPoint(W, CELL, geom)).toBeUndefined();
      expect(cellFromPoint(W, 0, geom)).toBeUndefined();
    });

    // VTT-934
    test(`${f.label}: a point exactly at GridHeight is off the grid`, () => {
      expect(cellFromPoint(CELL, H, geom)).toBeUndefined();
      expect(cellFromPoint(0, H, geom)).toBeUndefined();
    });

    // VTT-934
    test(`${f.label}: a point a fraction below zero is off the grid, on either axis`, () => {
      expect(cellFromPoint(-0.5, CELL, geom)).toBeUndefined();
      expect(cellFromPoint(CELL, -0.5, geom)).toBeUndefined();
      expect(cellFromPoint(-0.001, 0, geom)).toBeUndefined();
      expect(cellFromPoint(0, -0.001, geom)).toBeUndefined();
    });

    // VTT-934
    test(`${f.label}: a point a whole square or more beyond any edge is off the grid`, () => {
      expect(cellFromPoint(-CELL, CELL, geom)).toBeUndefined();
      expect(cellFromPoint(W + CELL, CELL, geom)).toBeUndefined();
      expect(cellFromPoint(CELL, -CELL, geom)).toBeUndefined();
      expect(cellFromPoint(CELL, H + CELL, geom)).toBeUndefined();
      expect(cellFromPoint(-CELL, -CELL, geom)).toBeUndefined();
      expect(cellFromPoint(W + 1, H + 1, geom)).toBeUndefined();
    });
  }
});

// ========================================================= renderSpectator

function boardState(f: Fixture): State {
  const st = newState();
  st.Scenes[f.sceneId] = { ID: f.sceneId, Name: f.label, GridWidth: f.w, GridHeight: f.h };
  return st;
}

describe("renderSpectator's board, clicked beside and inside the grid", () => {
  for (const f of FIXTURES) {
    // VTT-934 and the ticket's "Done looks like" item 1 for the clicks beside
    // the grid. No row holds what onCell is handed inside the grid: those
    // clicks are the control that shows the clicks beside it reach the board.
    test(`${f.label}: no click beside the grid calls onCell; a click inside each edge and one mid-square each call it once with that square`, () => {
      const calls: unknown[] = [];
      const root = mount();
      renderSpectator(root, boardState(f), [], "connected", { onCell: (c) => { calls.push(c); } });

      const beside: [string, number][] = [];
      for (const { label, at } of offClicks(f)) {
        const n = calls.length;
        clickBoard(root, at);
        beside.push([label, calls.length - n]);
      }
      expect(beside).toEqual(nothingAtEveryOffClick());
      expect(calls).toHaveLength(0);

      const inside: [Side, number, unknown[]][] = [];
      for (const side of SIDES) {
        const n = calls.length;
        clickBoard(root, probe(f, side).inside);
        inside.push([side, calls.length - n, calls.slice(n)]);
      }
      expect(inside).toStrictEqual(SIDES.map((side): [Side, number, unknown[]] => [side, 1, [probe(f, side).door]]));

      const n = calls.length;
      clickBoard(root, centreOf(f, f.interior));
      expect(calls.slice(n)).toHaveLength(1);
      expect(calls.slice(n)).toStrictEqual([f.interior]);
    });
  }
});

// ========================================================= a player's window

describe("a player's window", () => {
  for (const f of FIXTURES) {
    // VTT-934 and the ticket's "Done looks like" items 1 and 5 for the clicks
    // beside the grid; VTT-408 for the moves inside it. Every door is one the
    // window holds open: a click on a door it holds closed is VTT-936's. The
    // disc check shows the clicks aim where the board draws its squares.
    test(`${f.label}: beside the grid sends nothing and draws nothing new; inside each edge, on its open door, and on an empty square moves the watched token there`, async () => {
      const win = await playerWindow(f);
      expect(doorsOpenedInTheStory(win)).toBe(SIDES.length);

      const disc = win.root.querySelector<HTMLElement>('.grid .token[data-token-id="t-hero"]');
      expect(disc).not.toBeNull();
      const s = screenOf(f);
      const left = parseFloat(disc!.style.left);
      const top = parseFloat(disc!.style.top);
      expect(left).toBeGreaterThanOrEqual(s.left + f.token.x * s.sq - 1e-6);
      expect(left).toBeLessThan(s.left + (f.token.x + 1) * s.sq);
      expect(top).toBeGreaterThanOrEqual(s.top + f.token.y * s.sq - 1e-6);
      expect(top).toBeLessThan(s.top + (f.token.y + 1) * s.sq);

      const beside = await clickBesideTheGrid(win, f);
      expect(beside.table).toEqual(nothingAtEveryOffClick());
      expect(beside.total).toBe(0);
      expect(beside.unchanged).toBe(true);

      expect(await clickInsideEachEdge(win, f, moveOf)).toEqual(movesOntoEachDoor(f, "t-hero"));

      expect(await clickSquare(win, centreOf(f, f.interior), moveOf)).toEqual([1, { tokenId: "t-hero", to: f.interior }]);
    });

    // VTT-934 and the ticket's "Done looks like" item 1; SPEC-022: "The
    // current scene is the scene whose id sorts greatest"; VTT-408 for the
    // moves onto the open doors.
    test(`${f.label} current beside a larger earlier scene: beside the current grid sends nothing; inside each edge moves the watched token onto its open door`, async () => {
      const win = await playerWindow(f, true);
      expect(doorsOpenedInTheStory(win)).toBe(SIDES.length);
      const beside = await clickBesideTheGrid(win, f);
      expect(beside.table).toEqual(nothingAtEveryOffClick());
      expect(beside.total).toBe(0);
      expect(await clickInsideEachEdge(win, f, moveOf)).toEqual(movesOntoEachDoor(f, "t-hero"));
    });
  }
});

// ===================================================== a spectator's window

describe("a spectator's window", () => {
  for (const f of FIXTURES) {
    // VTT-934 and the ticket's "Done looks like" items 1 and 5; SPEC-022: "A
    // perching window's board click sends nothing".
    test(`${f.label}: beside the grid sends nothing and draws nothing new; inside sends nothing either`, async () => {
      const win = await spectatorWindow(f);
      const beside = await clickBesideTheGrid(win, f);
      expect(beside.table).toEqual(nothingAtEveryOffClick());
      expect(beside.total).toBe(0);
      expect(beside.unchanged).toBe(true);

      const m = win.mark();
      clickBoard(win.root, centreOf(f, f.interior));
      clickBoard(win.root, probe(f, "left").inside);
      await flush();
      expect(win.sentSince(m)).toHaveLength(0);
    });
  }
});

// ============================================= a DM's and an agent's window

for (const role of ["dm", "agent"] as const) {
  const who = role === "dm" ? "a DM's" : "an agent's";

  describe(`${who} window, doors unarmed`, () => {
    for (const f of FIXTURES) {
      // VTT-934 and the ticket's "Done looks like" items 1 and 5 for the
      // clicks beside the grid; for the moves inside it, SPEC-022: "A DM's
      // and an agent's chips select which of the actors granted them a board
      // click moves".
      test(`${f.label}: beside the grid, next to each door, sends nothing and draws nothing new; inside each edge and on an empty square moves the selected actor's token there`, async () => {
        const win = await consoleWindow(f, role);
        const beside = await clickBesideTheGrid(win, f);
        expect(beside.table).toEqual(nothingAtEveryOffClick());
        expect(beside.total).toBe(0);
        expect(beside.unchanged).toBe(true);

        expect(await clickInsideEachEdge(win, f, moveOf)).toEqual(movesOntoEachDoor(f, "t-goblin"));

        expect(await clickSquare(win, centreOf(f, f.interior), moveOf)).toEqual([1, { tokenId: "t-goblin", to: f.interior }]);
      });

      // VTT-934 and the ticket's "Done looks like" item 1; SPEC-022: "The
      // current scene is the scene whose id sorts greatest"; the chips
      // sentence above for the moves.
      test(`${f.label} current beside a larger earlier scene: beside the current grid sends nothing; inside each edge moves the selected actor's token there`, async () => {
        const win = await consoleWindow(f, role, true);
        const beside = await clickBesideTheGrid(win, f);
        expect(beside.table).toEqual(nothingAtEveryOffClick());
        expect(beside.total).toBe(0);
        expect(await clickInsideEachEdge(win, f, moveOf)).toEqual(movesOntoEachDoor(f, "t-goblin"));
      });
    }
  });

  describe(`${who} window, doors armed`, () => {
    for (const f of FIXTURES) {
      // VTT-934 and the ticket's "Done looks like" items 1 and 5 for the
      // clicks beside the grid; VTT-876 and VTT-888 for the door work inside
      // it.
      test(`${f.label}: beside the grid, next to each door, sends nothing and draws nothing new; inside each edge works that door naming no actor; a square with no door sends nothing`, async () => {
        const win = await consoleWindow(f, role);
        await armDoors(win);
        const beside = await clickBesideTheGrid(win, f);
        expect(beside.table).toEqual(nothingAtEveryOffClick());
        expect(beside.total).toBe(0);
        expect(beside.unchanged).toBe(true);

        expect(await clickInsideEachEdge(win, f, doorOf)).toEqual(
          SIDES.map((side): [Side, number, unknown] => [side, 1, {
            kind: side === "bottom" ? "closeDoor" : "openDoor", sceneId: f.sceneId, at: probe(f, side).door, actorId: "",
          }]));

        const m = win.mark();
        const [count] = await clickSquare(win, centreOf(f, f.interior), doorOf);
        expect(count).toBe(0);
        await flush();
        expect(win.sentSince(m)).toHaveLength(0);
      });
    }
  });
}
