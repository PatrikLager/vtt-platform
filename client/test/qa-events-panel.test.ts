import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { boot } from "../src/app";
import { renderSpectator } from "../src/view/spectator";
import { newState } from "../src/state";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";

// Independent QA of the Events panel being the DM's alone, driven through
// boot() with a fake WebSocket and a stubbed fetch, and once through
// renderSpectator directly. Each test names the requirement it pins on the
// comment line above it.

function setURL(url: string) {
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL(url);
}

function root(): HTMLElement {
  const el = document.createElement("div");
  el.id = "app";
  document.body.replaceChildren(el);
  return el;
}

type Frame = Record<string, unknown>;

class FakeSocket {
  static readonly OPEN = 1;
  static instances: FakeSocket[] = [];
  readyState = 0;
  readonly sent: string[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(readonly url: string) {
    FakeSocket.instances.push(this);
  }
  open() {
    this.readyState = FakeSocket.OPEN;
    this.onopen?.();
  }
  deliver(frame: Frame) {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
}

const nativeFetch = globalThis.fetch;
const nativeWS = globalThis.WebSocket;

/** What /api/me answers: a body, or null for a 404; `hold` keeps it pending until released. */
class MeRoute {
  calls = 0;
  answer: Record<string, unknown> | null = null;
  private gate: Promise<void> | null = null;
  private open_: (() => void) | null = null;
  hold() {
    this.gate = new Promise<void>((r) => (this.open_ = r));
  }
  release() {
    const o = this.open_;
    this.gate = null;
    this.open_ = null;
    o?.();
  }
  async serve(): Promise<Response> {
    this.calls++;
    if (this.gate) await this.gate;
    if (this.answer === null) return new Response("", { status: 404 });
    return json(this.answer);
  }
}

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "content-type": "application/json" } });
}

let me = new MeRoute();

function stubMetadata() {
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") return me.serve();
    if (path === "/api/ruleset") return json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    if (path === "/api/adventures") return json({ adventures: [] });
    return new Response("", { status: 404 });
  }) as typeof fetch;
}

function useFakeSocket() {
  FakeSocket.instances = [];
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
}

async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
  for (let i = 0; i < 8; i++) await Promise.resolve();
}

const sessions: { close(): void }[] = [];

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setURL("http://localhost/");
  me = new MeRoute();
});

afterEach(() => {
  while (sessions.length) sessions.pop()!.close();
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
  localStorage.clear();
  sessionStorage.clear();
});

// --- fixtures ---------------------------------------------------------------

const PLAYER = "p-pia";
const SPECTATOR = "p-sam";
const PARTY = "ACTOR_KIND_PARTY_MEMBER";

function envelope(seq: number, payload: Frame): Frame {
  return {
    event: {
      eventId: `e-${seq}`,
      sequence: String(seq),
      sessionId: "sess-1",
      actorRole: "dm",
      participantId: "p-dm",
      ...payload,
    },
  };
}

const sessionStarted: Frame = { sessionStarted: { name: "Night" } };
// The grid is 3 by 3 and the token stands only on (1,1) and (2,1), so the
// only digits a row may legitimately carry are 1, 2 and 3.
const SETUP: Frame[] = [
  sessionStarted,
  { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 3, gridHeight: 3 } },
  { actorAdded: { actor: { actorId: "a-bram", name: "Bram" } } },
  { actorControlGranted: { actorId: "a-bram", participantId: PLAYER, kind: PARTY } },
  { tokenPlaced: { tokenId: "t-bram", sceneId: "s1", actorId: "a-bram", position: { x: 1, y: 1 } } },
];

const LETTERS = "abcdefghijklmnopqrstuvwxyz";
/** A digit-free, fixed-length tag naming move i, so no tag is a substring of another. */
const tag = (i: number) => `k${LETTERS[Math.floor(i / 26)]}${LETTERS[i % 26]}`;
const reasonOf = (i: number) => `route ${tag(i)}`;

/** Moves 1..n of t-bram between (1,1) and (2,1), move i carrying reasonOf(i). */
function moves(n: number, reason: (i: number) => string = reasonOf): Frame[] {
  const out: Frame[] = [];
  for (let i = 1; i <= n; i++) {
    const from = i % 2 === 1 ? { x: 1, y: 1 } : { x: 2, y: 1 };
    const to = i % 2 === 1 ? { x: 2, y: 1 } : { x: 1, y: 1 };
    out.push({ tokenMoved: { tokenId: "t-bram", sceneId: "s1", from, to, reason: reason(i) } });
  }
  return out;
}

const logWith = (n: number) => [...SETUP, ...moves(n)];

function deliverLog(sock: FakeSocket, payloads: Frame[], firstSeq = 1) {
  payloads.forEach((p, i) => sock.deliver(envelope(firstSeq + i, p)));
}

type Role = "dm" | "agent" | "player" | "spectator";

interface Opening {
  /** The /api/me answer's role; null answers 404, "omit" answers with no role field. */
  role: Role | string | null | "omit";
  participantId: string;
  /** Send the set and the ViewpointChanged, as the gateway does for a player or a spectator. */
  seat?: { viewpoint: string };
  log: Frame[];
  /** Keep /api/me pending until me.release(). */
  holdMe?: boolean;
}

async function open(o: Opening) {
  localStorage.setItem("vtt.token", "tok");
  if (o.role === null) me.answer = null;
  else if (o.role === "omit") me.answer = { participantId: o.participantId, name: "Me" };
  else me.answer = { participantId: o.participantId, name: "Me", role: o.role };
  if (o.holdMe) me.hold();
  stubMetadata();
  useFakeSocket();
  const r = root();
  const s = boot(r);
  if (s) sessions.push(s);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({ catchUpHead: { headSequence: String(o.log.length) } });
  sock.deliver({ presenceSnapshot: {} });
  if (o.seat) {
    sock.deliver({ watchableChanged: { actors: [{ actorId: "a-bram", name: "Bram" }] } });
    sock.deliver({ viewpointChanged: o.seat.viewpoint ? { actorId: o.seat.viewpoint } : {} });
  }
  deliverLog(sock, o.log);
  await settle();
  return { r, sock };
}

const openWorld = (role: Role | string | null | "omit", log: Frame[], extra: Partial<Opening> = {}) =>
  open({ role, participantId: `p-${String(role)}`, log, ...extra });

const openSeat = (role: "player" | "spectator", log: Frame[]) =>
  open({ role, participantId: role === "player" ? PLAYER : SPECTATOR, seat: { viewpoint: "a-bram" }, log });

// --- reading the window -----------------------------------------------------

const count = (r: HTMLElement, sel: string) => r.querySelectorAll(sel).length;
const tickers = (r: HTMLElement) => count(r, "section.ticker");
const ticks = (r: HTMLElement) => Array.from(r.querySelectorAll("section.ticker p.tick")).map((p) => p.textContent ?? "");
const feedRows = (r: HTMLElement) => Array.from(r.querySelectorAll(".feed .mechanical")).map((p) => p.textContent ?? "");

/** The move whose reason a row carries, or -1 when it carries none of moves 1..max. */
function moveIn(row: string, max: number): number {
  const hits: number[] = [];
  for (let i = 1; i <= max; i++) if (row.includes(reasonOf(i))) hits.push(i);
  if (hits.length > 1) throw new Error(`row carries ${hits.length} moves: ${row}`);
  return hits[0] ?? -1;
}

/** Every number written in the text, other than the board's own 1, 2 and 3. */
const strayNumbers = (text: string) => (text.match(/\d+/g) ?? []).filter((n) => !["1", "2", "3"].includes(n));

const ownPanel: Record<Role, string> = { dm: ".dm", agent: ".dm", player: ".player", spectator: ".perch" };

// --- the DM and the agent have it --------------------------------------------

for (const role of ["dm", "agent"] as const) {
  // VTT-446
  test(`qaEP: ${role}: the window draws one Events panel headed Events`, async () => {
    const { r } = await openWorld(role, logWith(2));
    expect(count(r, ownPanel[role])).toBeGreaterThan(0);
    expect(tickers(r)).toBe(1);
    expect(r.querySelector("section.ticker h2")?.textContent).toBe("Events");
  });

  // VTT-446
  test(`qaEP: ${role}: the Events panel is drawn before the log holds any entry, with no row`, async () => {
    const { r } = await openWorld(role, []);
    expect(count(r, ownPanel[role])).toBeGreaterThan(0);
    expect(tickers(r)).toBe(1);
    expect(ticks(r)).toHaveLength(0);
  });

  // VTT-447
  test(`qaEP: ${role}: fewer than 40 entries give one row each, newest first`, async () => {
    const { r } = await openWorld(role, logWith(4));
    const rows = ticks(r);
    expect(rows).toHaveLength(SETUP.length + 4);
    expect(rows.slice(0, 4).map((x) => moveIn(x, 4))).toEqual([4, 3, 2, 1]);
    expect(rows.slice(4).map((x) => moveIn(x, 4))).toEqual([-1, -1, -1, -1, -1]);
  });

  // VTT-447 VTT-448
  test(`qaEP: ${role}: of 50 entries the panel lists the newest 40, newest first, one row each`, async () => {
    const { r } = await openWorld(role, logWith(45));
    const rows = ticks(r);
    expect(rows).toHaveLength(40);
    const expected = Array.from({ length: 40 }, (_, i) => 45 - i);
    expect(rows.map((x) => moveIn(x, 45))).toEqual(expected);
  });

  // VTT-447 VTT-448
  test(`qaEP: ${role}: at exactly 40 entries every entry has a row, and the 41st drops the oldest`, async () => {
    const { r, sock } = await openWorld(role, logWith(35));
    expect(ticks(r)).toHaveLength(40);
    expect(ticks(r).slice(-SETUP.length).map((x) => moveIn(x, 36))).toEqual([-1, -1, -1, -1, -1]);
    deliverLog(sock, moves(36).slice(35), SETUP.length + 36);
    await settle();
    const rows = ticks(r);
    expect(rows).toHaveLength(40);
    expect(moveIn(rows[0]!, 36)).toBe(36);
    // The oldest entry, sessionStarted, is gone; the four setup entries
    // after it remain last.
    expect(rows.slice(-SETUP.length).map((x) => moveIn(x, 36))).toEqual([1, -1, -1, -1, -1]);
  });

  // VTT-447
  test(`qaEP: ${role}: a narration entry has its row like any other entry`, async () => {
    const log = [...SETUP, ...moves(2), { narrationAdded: { text: "The candles gutter.", as: "" } }, ...moves(3).slice(2)];
    const { r } = await openWorld(role, log);
    const rows = ticks(r);
    expect(rows).toHaveLength(log.length);
    expect(moveIn(rows[0]!, 3)).toBe(3);
    expect(moveIn(rows[1]!, 3)).toBe(-1);
    expect(moveIn(rows[2]!, 3)).toBe(2);
  });

  // VTT-263
  test(`qaEP: ${role}: the ticker and the feed label a move with its reason`, async () => {
    const { r } = await openWorld(role, [...SETUP, ...moves(1, () => "dodging the falling chandelier")]);
    expect(ticks(r)[0]).toContain("dodging the falling chandelier");
    expect(feedRows(r).some((x) => x.includes("dodging the falling chandelier"))).toBe(true);
  });

  // VTT-393
  test(`qaEP: ${role}: neither the ticker's rows nor the Story's show a log sequence number`, async () => {
    const { r } = await openWorld(role, logWith(45));
    const rows = ticks(r);
    expect(rows).toHaveLength(40);
    expect(rows.flatMap(strayNumbers)).toEqual([]);
    expect(feedRows(r).flatMap(strayNumbers)).toEqual([]);
    expect(strayNumbers(r.querySelector("section.ticker")?.textContent ?? "")).toEqual([]);
    expect(strayNumbers(r.textContent ?? "")).toEqual([]);
  });

  // VTT-446
  test(`qaEP: ${role}: while the window gives the reason it cannot fold its log, it draws no Events panel`, async () => {
    const { r } = await openWorld(role, [...logWith(2), sessionStarted]);
    expect(count(r, "p.fatal")).toBe(1);
    expect(tickers(r)).toBe(0);
  });

  // VTT-446 VTT-395
  test(`qaEP: ${role}: once a discard clears the reason, the Events panel is back, listing only the new log`, async () => {
    const { r, sock } = await openWorld(role, [...logWith(2), sessionStarted]);
    expect(count(r, "p.fatal")).toBe(1);
    expect(tickers(r)).toBe(0);
    deliverLog(sock, [...SETUP, ...moves(3, (i) => `afresh ${tag(i)}`)]);
    await settle();
    expect(count(r, "p.fatal")).toBe(0);
    expect(tickers(r)).toBe(1);
    const rows = ticks(r);
    expect(rows).toHaveLength(SETUP.length + 3);
    expect(rows[0]).toContain(`afresh ${tag(3)}`);
    expect(rows.some((x) => x.includes("route "))).toBe(false);
  });

  // VTT-447 VTT-395
  test(`qaEP: ${role}: after an entry of sequence 1 discards the log, the panel lists the new log and none of the old`, async () => {
    const { r, sock } = await openWorld(role, logWith(4));
    expect(ticks(r).some((x) => x.includes("route "))).toBe(true);
    deliverLog(sock, [...SETUP, ...moves(2, (i) => `afresh ${tag(i)}`)]);
    await settle();
    const rows = ticks(r);
    expect(rows).toHaveLength(SETUP.length + 2);
    expect(rows[0]).toContain(`afresh ${tag(2)}`);
    expect(rows.some((x) => x.includes("route "))).toBe(false);
  });
}

// --- players and spectators do not ------------------------------------------

for (const role of ["player", "spectator"] as const) {
  // VTT-445
  test(`qaEP: ${role}: the window draws no Events panel while its log holds 50 entries`, async () => {
    const { r } = await openSeat(role, logWith(45));
    expect(count(r, ownPanel[role])).toBeGreaterThan(0);
    expect(feedRows(r).length).toBeGreaterThan(0);
    expect(tickers(r)).toBe(0);
    expect(count(r, "p.tick")).toBe(0);
    expect(Array.from(r.querySelectorAll("h2")).map((h) => h.textContent)).not.toContain("Events");
  });

  // VTT-449
  test(`qaEP: ${role}: the Story still shows every entry of the log, more than 40 included`, async () => {
    const { r } = await openSeat(role, logWith(45));
    const rows = feedRows(r);
    expect(rows).toHaveLength(SETUP.length + 45);
    for (let i = 1; i <= 45; i++) expect(rows.some((x) => x.includes(reasonOf(i)))).toBe(true);
  });

  // VTT-393
  test(`qaEP: ${role}: the window shows no log sequence number`, async () => {
    const { r } = await openSeat(role, logWith(45));
    expect(count(r, ownPanel[role])).toBeGreaterThan(0);
    expect(feedRows(r)).toHaveLength(SETUP.length + 45);
    expect(strayNumbers(r.textContent ?? "")).toEqual([]);
  });

  // VTT-263
  test(`qaEP: ${role}: the Story labels a move with its reason`, async () => {
    const { r } = await openSeat(role, [...SETUP, ...moves(1, () => "dodging the falling chandelier")]);
    expect(count(r, ownPanel[role])).toBeGreaterThan(0);
    expect(feedRows(r).some((x) => x.includes("dodging the falling chandelier"))).toBe(true);
  });
}

for (const role of ["dm", "agent"] as const) {
  // VTT-449
  test(`qaEP: ${role}: the Story shows every entry of the log while the panel shows 40`, async () => {
    const { r } = await openWorld(role, logWith(45));
    expect(ticks(r)).toHaveLength(40);
    const rows = feedRows(r);
    expect(rows).toHaveLength(SETUP.length + 45);
    for (let i = 1; i <= 45; i++) expect(rows.some((x) => x.includes(reasonOf(i)))).toBe(true);
  });
}

// --- no role known ------------------------------------------------------------

// VTT-445 VTT-446
test("qaEP: until /api/me answers, a page sent the world draws no Events panel, and draws it once the answer is dm", async () => {
  const { r } = await openWorld("dm", logWith(3), { holdMe: true });
  expect(me.calls).toBeGreaterThan(0);
  expect(feedRows(r)).toHaveLength(SETUP.length + 3);
  expect(tickers(r)).toBe(0);
  me.release();
  await settle();
  expect(count(r, ".dm")).toBeGreaterThan(0);
  expect(tickers(r)).toBe(1);
  expect(ticks(r)).toHaveLength(SETUP.length + 3);
});

// VTT-445
test("qaEP: a first /api/me that answers 404 leaves no role, and the page sent the world draws no Events panel", async () => {
  const { r } = await openWorld(null, logWith(3));
  expect(me.calls).toBeGreaterThan(0);
  expect(feedRows(r)).toHaveLength(SETUP.length + 3);
  expect(tickers(r)).toBe(0);
  expect(count(r, "p.tick")).toBe(0);
});

// VTT-445
test("qaEP: a /api/me that answers with no role field draws no Events panel", async () => {
  const { r } = await openWorld("omit", logWith(3));
  expect(me.calls).toBeGreaterThan(0);
  expect(feedRows(r)).toHaveLength(SETUP.length + 3);
  expect(tickers(r)).toBe(0);
});

// VTT-445
test("qaEP: a /api/me role that is neither dm nor agent, gm say, draws no Events panel", async () => {
  const { r } = await openWorld("gm", logWith(3));
  expect(me.calls).toBeGreaterThan(0);
  expect(feedRows(r)).toHaveLength(SETUP.length + 3);
  expect(tickers(r)).toBe(0);
});

// --- the role the client LAST read ---------------------------------------------

const presenceChanged = (participantId: string): Frame => ({
  presenceChanged: { participantId, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" },
});
const presenceSnapshot = (participantId: string): Frame => ({
  presenceSnapshot: { present: [{ participantId, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" }] },
});

// VTT-445
test("qaEP: a dm re-read as player at a presence frame naming itself loses the Events panel", async () => {
  const { r, sock } = await open({ role: "dm", participantId: "p-me", log: logWith(3) });
  expect(tickers(r)).toBe(1);
  const before = me.calls;
  me.answer = { participantId: "p-me", name: "Me", role: "player" };
  sock.deliver(presenceChanged("p-me"));
  await settle();
  expect(me.calls).toBeGreaterThan(before);
  expect(count(r, ".player")).toBeGreaterThan(0);
  expect(tickers(r)).toBe(0);
});

// VTT-446
test("qaEP: a player re-read as dm at a presence frame naming itself gains the Events panel", async () => {
  const { r, sock } = await open({ role: "player", participantId: "p-me", seat: { viewpoint: "a-bram" }, log: logWith(3) });
  expect(count(r, ".player")).toBeGreaterThan(0);
  expect(tickers(r)).toBe(0);
  const before = me.calls;
  me.answer = { participantId: "p-me", name: "Me", role: "dm" };
  sock.deliver(presenceChanged("p-me"));
  await settle();
  expect(me.calls).toBeGreaterThan(before);
  expect(tickers(r)).toBe(1);
  expect(ticks(r)).toHaveLength(SETUP.length + 3);
});

// VTT-446
test("qaEP: a spectator re-read as agent at a presence snapshot naming itself gains the Events panel", async () => {
  const { r, sock } = await open({ role: "spectator", participantId: "p-me", seat: { viewpoint: "a-bram" }, log: logWith(3) });
  expect(count(r, ".perch")).toBeGreaterThan(0);
  expect(tickers(r)).toBe(0);
  const before = me.calls;
  me.answer = { participantId: "p-me", name: "Me", role: "agent" };
  sock.deliver(presenceSnapshot("p-me"));
  await settle();
  expect(me.calls).toBeGreaterThan(before);
  expect(tickers(r)).toBe(1);
});

// VTT-445
test("qaEP: a dm whose re-read answers with no role field knows no role and loses the Events panel", async () => {
  const { r, sock } = await open({ role: "dm", participantId: "p-me", log: logWith(3) });
  expect(tickers(r)).toBe(1);
  const before = me.calls;
  me.answer = { participantId: "p-me", name: "Me" };
  sock.deliver(presenceChanged("p-me"));
  await settle();
  expect(me.calls).toBeGreaterThan(before);
  expect(feedRows(r)).toHaveLength(SETUP.length + 3);
  expect(tickers(r)).toBe(0);
});

// --- the view function --------------------------------------------------------

function env(seq: number, reason: string): Envelope {
  return create(EnvelopeSchema, {
    eventId: `e-${seq}`,
    sequence: BigInt(seq),
    payload: { case: "tokenMoved", value: { tokenId: "t-bram", sceneId: "s1", reason } },
  });
}

// VTT-447 VTT-448
test("qaEP: renderSpectator told to draw the ticker lists the newest 40 of 45 entries, newest first", () => {
  const r = root();
  const log = Array.from({ length: 45 }, (_, i) => env(i + 1, reasonOf(i + 1)));
  renderSpectator(r, newState(), log, "connected", { ticker: true });
  const rows = ticks(r);
  expect(rows).toHaveLength(40);
  expect(rows.map((x) => moveIn(x, 45))).toEqual(Array.from({ length: 40 }, (_, i) => 45 - i));
});
