import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { boot } from "../src/app";
import { boardCamera, CELL } from "../src/view/spectator";

// Independent QA of the switch buttons and the board click, driven through
// boot() with a fake WebSocket. Each test names the requirement it pins.

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

function stubMetadata(routes: Record<string, unknown>) {
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (!(path in routes)) return new Response("", { status: 404 });
    return new Response(JSON.stringify(routes[path]), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }) as typeof fetch;
}

function useFakeSocket() {
  FakeSocket.instances = [];
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
}

async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

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

beforeEach(() => {
  localStorage.clear();
  setURL("http://localhost/");
});

afterEach(() => {
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
});

// --- fixtures ---------------------------------------------------------------

const PLAYER = "p-pia";
const PARTY = "ACTOR_KIND_PARTY_MEMBER";
const NON_PARTY = "ACTOR_KIND_NON_PARTY";

const sessionStarted: Frame = { sessionStarted: { name: "Night" } };
const scene = (tiles?: Record<string, Frame>): Frame => ({
  sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8, ...(tiles ? { tiles } : {}) },
});
const actor = (actorId: string, name: string): Frame => ({ actorAdded: { actor: { actorId, name } } });
const grant = (actorId: string, participantId: string, kind: string): Frame => ({
  actorControlGranted: { actorId, participantId, kind },
});
const token = (tokenId: string, actorId: string, x: number, y: number): Frame => ({
  tokenPlaced: { tokenId, sceneId: "s1", actorId, position: { x, y } },
});

/**
 * The player controls a-bram, b-lera, the non-party creature c-wolf and d-mira;
 * the set below names a-bram, b-lera and two the log never introduces.
 */
function mainLog(extra: { tiles?: Record<string, Frame>; bram?: [number, number]; more?: Frame[] } = {}): Frame[] {
  const [bx, by] = extra.bram ?? [5, 5];
  return [
    sessionStarted,
    scene(extra.tiles),
    actor("a-bram", "Bram"),
    grant("a-bram", PLAYER, PARTY),
    actor("b-lera", "Lera"),
    grant("b-lera", PLAYER, PARTY),
    actor("c-wolf", "Wolf"),
    grant("c-wolf", PLAYER, NON_PARTY),
    actor("d-mira", "Mira"),
    grant("d-mira", PLAYER, PARTY),
    token("t-bram", "a-bram", bx, by),
    token("t-lera", "b-lera", 1, 1),
    token("t-wolf", "c-wolf", 3, 3),
    token("t-mira", "d-mira", 6, 1),
    ...(extra.more ?? []),
  ];
}

type Watchable = { actorId: string; name: string };
// Out of id order on purpose; an entry with an empty name omits it, as
// protojson does.
const MAIN_SET: Watchable[] = [
  { actorId: "b-lera", name: "Lera" },
  { actorId: "g-ink", name: "" },
  { actorId: "a-bram", name: "Bram Stone" },
  { actorId: "e-tova", name: "Tova" },
];

function watchableFrame(set: Watchable[]): Frame {
  if (set.length === 0) return { watchableChanged: {} };
  return {
    watchableChanged: { actors: set.map((a) => (a.name ? { actorId: a.actorId, name: a.name } : { actorId: a.actorId })) },
  };
}

function viewpointFrame(actorId: string): Frame {
  return { viewpointChanged: actorId ? { actorId } : {} };
}

function deliverLog(sock: FakeSocket, payloads: Frame[]) {
  payloads.forEach((p, i) => sock.deliver(envelope(i + 1, p)));
}

const POKE = { id: "poke", name: "Poke", range: 1, maxTargets: 1, usage: { kind: "atWill" } };

interface Opening {
  role: "player" | "spectator" | "dm" | "agent";
  participantId: string;
  set?: Watchable[];
  viewpoint?: string;
  log: Frame[];
  abilities?: unknown[];
}

/** Boot and open a connection the way the gateway does for the role. */
async function open(o: Opening) {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: o.participantId, name: "Me", role: o.role },
    "/api/ruleset": { id: "r", name: "R", abilities: o.abilities ?? [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({ catchUpHead: { headSequence: String(o.log.length) } });
  sock.deliver({ presenceSnapshot: {} });
  if (o.role === "player" || o.role === "spectator") {
    sock.deliver(watchableFrame(o.set ?? []));
    sock.deliver(viewpointFrame(o.viewpoint ?? ""));
  }
  deliverLog(sock, o.log);
  await settle();
  return { r, s, sock };
}

const openPlayer = (extra: Partial<Opening> = {}) =>
  open({ role: "player", participantId: PLAYER, set: MAIN_SET, viewpoint: "b-lera", log: mainLog(), ...extra });

// --- reading the window -----------------------------------------------------

interface Cmd {
  requestId?: string;
  setViewpoint?: { actorId?: string };
  moveToken?: { tokenId?: string; to?: { x?: number; y?: number } };
  openDoor?: unknown;
  useAbility?: unknown;
}

const sentSince = (sock: FakeSocket, from: number): Cmd[] => sock.sent.slice(from).map((d) => JSON.parse(d) as Cmd);

function chips(r: HTMLElement): HTMLButtonElement[] {
  const row = r.querySelector(".player .row");
  return row ? Array.from(row.querySelectorAll<HTMLButtonElement>("button.chip")) : [];
}
const shoulders = (r: HTMLElement) =>
  Array.from(r.querySelectorAll<HTMLButtonElement>(".perch .shoulder:not(.unperch)"));
const unperch = (r: HTMLElement) => Array.from(r.querySelectorAll<HTMLButtonElement>(".perch .unperch"));
const labels = (bs: HTMLElement[]) => bs.map((b) => b.textContent ?? "").sort();
const marked = (bs: HTMLElement[], cls: string) =>
  bs.filter((b) => b.classList.contains(cls)).map((b) => b.textContent ?? "");
const byLabel = (bs: HTMLButtonElement[], label: string) => {
  const b = bs.find((x) => x.textContent === label);
  if (!b) throw new Error(`no button labelled ${label}`);
  return b;
};

function clickCell(r: HTMLElement, cx: number, cy: number) {
  const cam = boardCamera(8, 8);
  const px = cam.offsetX + (cx + 0.5) * CELL * cam.scale;
  const py = cam.offsetY + (cy + 0.5) * CELL * cam.scale;
  (r.querySelector(".grid") as HTMLElement).dispatchEvent(
    new MouseEvent("click", { clientX: px, clientY: py, bubbles: true }),
  );
}

const viewpointOf = (c: Cmd) => c.setViewpoint?.actorId ?? "";

/** Switch the open connection: the gateway's answer, a ViewpointChanged, and the new log from 1. */
async function switchTo(sock: FakeSocket, actorId: string, log: Frame[]) {
  sock.deliver(viewpointFrame(actorId));
  deliverLog(sock, log);
  await settle();
}

// --- the player's buttons ---------------------------------------------------

test("player: the chips are exactly the set, not the actors the folded log shows under their control", async () => {
  // VTT-402, VTT-404
  const { r, s } = await openPlayer();
  expect(labels(chips(r))).toEqual(["Bram Stone", "Lera", "Tova", "g-ink"].sort());
  s?.close();
});

test("player: the chip of the watched character, not the first by id, is the one marked sel", async () => {
  // VTT-407
  const { r, s } = await openPlayer();
  expect(marked(chips(r), "sel")).toEqual(["Lera"]);
  s?.close();
});

test("player: a new set replaces the chips, and an empty one leaves none while the fold still holds actors", async () => {
  // VTT-402, VTT-404
  const { r, s, sock } = await openPlayer();
  sock.deliver(watchableFrame([{ actorId: "b-lera", name: "Lera" }, { actorId: "d-mira", name: "Mira M" }]));
  await settle();
  expect(labels(chips(r))).toEqual(["Lera", "Mira M"]);
  sock.deliver(watchableFrame([]));
  await settle();
  expect(chips(r)).toHaveLength(0);
  s?.close();
});

test("player: after a switch the chips are still the last set, and sel follows the new ViewpointChanged", async () => {
  // VTT-402, VTT-407
  const { r, s, sock } = await openPlayer();
  await switchTo(sock, "a-bram", mainLog());
  expect(labels(chips(r))).toEqual(["Bram Stone", "Lera", "Tova", "g-ink"].sort());
  expect(marked(chips(r), "sel")).toEqual(["Bram Stone"]);
  s?.close();
});

test("player: a chip click sends set_viewpoint naming its party member, one the log has not introduced included", async () => {
  // VTT-405
  const { r, s, sock } = await openPlayer();
  let from = sock.sent.length;
  byLabel(chips(r), "Bram Stone").click();
  await settle();
  expect(sentSince(sock, from).map(viewpointOf)).toEqual(["a-bram"]);
  expect(sentSince(sock, from).every((c) => c.setViewpoint !== undefined)).toBe(true);

  from = sock.sent.length;
  byLabel(chips(r), "Tova").click();
  await settle();
  expect(sentSince(sock, from).map(viewpointOf)).toEqual(["e-tova"]);
  s?.close();
});

test("player: a click on the watched character's own chip sends set_viewpoint naming it", async () => {
  // VTT-405
  const { r, s, sock } = await openPlayer();
  const from = sock.sent.length;
  byLabel(chips(r), "Lera").click();
  await settle();
  expect(sentSince(sock, from).map((c) => c.setViewpoint?.actorId)).toEqual(["b-lera"]);
  s?.close();
});

// --- the player's board click ----------------------------------------------

test("player: a click on a listed party member's token sends set_viewpoint naming it", async () => {
  // VTT-406
  const { r, s, sock } = await openPlayer();
  const from = sock.sent.length;
  clickCell(r, 5, 5);
  await settle();
  const sent = sentSince(sock, from);
  expect(sent).toHaveLength(1);
  expect(sent[0]!.setViewpoint?.actorId).toBe("a-bram");
  s?.close();
});

test("player: a click on an empty square moves the watched character, which is not the first by id", async () => {
  // VTT-408
  const { r, s, sock } = await openPlayer();
  const from = sock.sent.length;
  clickCell(r, 7, 7);
  await settle();
  const sent = sentSince(sock, from);
  expect(sent).toHaveLength(1);
  expect(sent[0]!.moveToken?.tokenId).toBe("t-lera");
  expect(sent[0]!.moveToken?.to).toEqual({ x: 7, y: 7 });
  s?.close();
});

test("player: a click on the watched character's own token alone sends no set_viewpoint, and moves nothing else", async () => {
  // VTT-406 (only another listed party member switches), VTT-408
  const { r, s, sock } = await openPlayer();
  const from = sock.sent.length;
  clickCell(r, 1, 1);
  await settle();
  const sent = sentSince(sock, from);
  expect(sent.filter((c) => c.setViewpoint !== undefined)).toHaveLength(0);
  expect(sent.filter((c) => c.moveToken !== undefined && c.moveToken.tokenId !== "t-lera")).toHaveLength(0);
  s?.close();
});

test("player: the watched character and a listed party member on one square switch to the other", async () => {
  // VTT-406
  const { r, s, sock } = await openPlayer({ log: mainLog({ bram: [1, 1] }) });
  const from = sock.sent.length;
  clickCell(r, 1, 1);
  await settle();
  const sent = sentSince(sock, from);
  expect(sent).toHaveLength(1);
  expect(sent[0]!.setViewpoint?.actorId).toBe("a-bram");
  s?.close();
});

test("player: two listed party members on one square send one set_viewpoint naming one of them", async () => {
  // VTT-406
  const log = mainLog({ more: [actor("e-tova", "Tova"), grant("e-tova", PLAYER, PARTY), token("t-tova", "e-tova", 5, 5)] });
  const { r, s, sock } = await openPlayer({ log });
  const from = sock.sent.length;
  clickCell(r, 5, 5);
  await settle();
  const sent = sentSince(sock, from);
  expect(sent).toHaveLength(1);
  expect(["a-bram", "e-tova"]).toContain(sent[0]!.setViewpoint?.actorId ?? "none");
  s?.close();
});

test("player: a token of an actor the set does not name switches to nothing, and any move is the watched character's", async () => {
  // VTT-406, VTT-408; the player acts only as the character they watch (Q1).
  const { r, s, sock } = await openPlayer();
  for (const [x, y] of [[3, 3], [6, 1]] as const) {
    const from = sock.sent.length;
    clickCell(r, x, y);
    await settle();
    const sent = sentSince(sock, from);
    expect(sent.filter((c) => c.setViewpoint !== undefined)).toHaveLength(0);
    expect(sent.filter((c) => c.moveToken !== undefined).map((c) => c.moveToken?.tokenId)).not.toContain("t-wolf");
    expect(sent.filter((c) => c.moveToken !== undefined && c.moveToken.tokenId !== "t-lera")).toHaveLength(0);
  }
  s?.close();
});

test("player: after a switch, a move moves the newly watched character", async () => {
  // VTT-408, VTT-407
  const { r, s, sock } = await openPlayer();
  byLabel(chips(r), "Bram Stone").click();
  await settle();
  const req = JSON.parse(sock.sent[sock.sent.length - 1]!) as Cmd;
  sock.deliver({ result: { requestId: req.requestId, ok: true } });
  await switchTo(sock, "a-bram", mainLog());
  const from = sock.sent.length;
  clickCell(r, 7, 7);
  await settle();
  const sent = sentSince(sock, from);
  expect(sent).toHaveLength(1);
  expect(sent[0]!.moveToken?.tokenId).toBe("t-bram");
  s?.close();
});

test("player: with doors armed, a click on a listed party member's token sends nothing, and the door still works", async () => {
  // VTT-406: armed doors work a door or do nothing.
  const log = mainLog({ tiles: { "0,0": { kind: "door", material: "wood", art: "" } } });
  const { r, s, sock } = await openPlayer({ log });
  r.querySelector<HTMLButtonElement>('.player [data-action="arm-doors"]')!.click();
  await settle();
  let from = sock.sent.length;
  clickCell(r, 5, 5);
  await settle();
  expect(sentSince(sock, from)).toHaveLength(0);

  from = sock.sent.length;
  clickCell(r, 0, 0);
  await settle();
  const door = sentSince(sock, from);
  expect(door).toHaveLength(1);
  expect(door[0]).toHaveProperty("openDoor");
  s?.close();
});

test("player: with an ability armed and doors not, a click on a listed party member's token in range still switches", async () => {
  // VTT-406
  const { r, s, sock } = await openPlayer({ log: mainLog({ bram: [2, 2] }), abilities: [POKE] });
  const poke = Array.from(r.querySelectorAll<HTMLButtonElement>(".player button")).find((b) => b.textContent === "Poke");
  expect(poke).toBeDefined();
  poke!.click();
  await settle();
  const from = sock.sent.length;
  clickCell(r, 2, 2);
  await settle();
  expect(sentSince(sock, from).map((c) => c.setViewpoint?.actorId)).toEqual(["a-bram"]);
  s?.close();
});

// --- the spectator's perch --------------------------------------------------

const SPECTATOR = "p-sam";
const SPEC_SET: Watchable[] = [
  { actorId: "b-lera", name: "Lera" },
  { actorId: "g-ink", name: "" },
  { actorId: "a-bram", name: "Bram Stone" },
];

const openSpectator = (extra: Partial<Opening> = {}) =>
  open({ role: "spectator", participantId: SPECTATOR, set: SPEC_SET, viewpoint: "", log: [], ...extra });

test("spectator watching nobody: a shoulder per party member the set names, labelled from the set, and one No shoulder", async () => {
  // VTT-403, VTT-404, VTT-407
  const { r, s } = await openSpectator();
  expect(labels(shoulders(r))).toEqual(["Bram Stone", "Lera", "g-ink"].sort());
  expect(unperch(r).map((b) => b.textContent)).toEqual(["No shoulder"]);
  expect(marked(shoulders(r), "on")).toEqual([]);
  expect(r.querySelector(".perch .perched-on")?.textContent).toBe("Perched on: nobody");
  s?.close();
});

test("spectator on a shoulder: exactly the set, not the party members the log introduces, and the watched one is on", async () => {
  // VTT-403, VTT-407
  const { r, s } = await openSpectator({ viewpoint: "b-lera", log: mainLog() });
  expect(labels(shoulders(r))).toEqual(["Bram Stone", "Lera", "g-ink"].sort());
  expect(marked(shoulders(r), "on")).toEqual(["Lera"]);
  expect(r.querySelector(".perch .perched-on")?.textContent).toBe("Perched on: Lera");
  s?.close();
});

test("spectator: a new set replaces the shoulders, and an empty one offers none and says so", async () => {
  // VTT-403, VTT-404
  const { r, s, sock } = await openSpectator();
  sock.deliver(watchableFrame([{ actorId: "d-mira", name: "Mira" }]));
  await settle();
  expect(labels(shoulders(r))).toEqual(["Mira"]);
  sock.deliver(watchableFrame([]));
  await settle();
  expect(shoulders(r)).toHaveLength(0);
  expect(r.querySelector(".perch")?.textContent).toContain("No party members yet.");
  expect(unperch(r)).toHaveLength(1);
  s?.close();
});

test("spectator: each ViewpointChanged moves the on mark, and the shoulders stay the set", async () => {
  // VTT-407, VTT-403
  const { r, s, sock } = await openSpectator();
  await switchTo(sock, "a-bram", mainLog());
  expect(marked(shoulders(r), "on")).toEqual(["Bram Stone"]);
  await switchTo(sock, "", []);
  expect(marked(shoulders(r), "on")).toEqual([]);
  expect(labels(shoulders(r))).toEqual(["Bram Stone", "Lera", "g-ink"].sort());
  s?.close();
});

test("spectator: a shoulder click sends set_viewpoint naming it, and No shoulder names nobody", async () => {
  // VTT-405
  const { r, s, sock } = await openSpectator({ viewpoint: "b-lera", log: mainLog() });
  let from = sock.sent.length;
  byLabel(shoulders(r), "g-ink").click();
  await settle();
  expect(sentSince(sock, from).map((c) => c.setViewpoint?.actorId)).toEqual(["g-ink"]);

  from = sock.sent.length;
  unperch(r)[0]!.click();
  await settle();
  const off = sentSince(sock, from);
  expect(off).toHaveLength(1);
  expect(off[0]!.setViewpoint).toBeDefined();
  expect(viewpointOf(off[0]!)).toBe("");
  s?.close();
});

test("spectator: a board click sends nothing, on a listed party member's token or an empty square", async () => {
  // owner's Q2 ruling: "a spectator's click on the board sends nothing"
  const { r, s, sock } = await openSpectator({ viewpoint: "b-lera", log: mainLog() });
  const from = sock.sent.length;
  clickCell(r, 5, 5);
  clickCell(r, 7, 7);
  await settle();
  expect(sentSince(sock, from)).toHaveLength(0);
  s?.close();
});

// --- the DM and the agent -----------------------------------------------------

/** The world, in which the DM or agent controls a-bram and the non-party n-orc, and not b-lera. */
const worldLog = (me: string): Frame[] => [
  sessionStarted,
  scene(),
  actor("a-bram", "Bram"),
  grant("a-bram", PLAYER, PARTY),
  grant("a-bram", me, PARTY),
  actor("b-lera", "Lera"),
  grant("b-lera", PLAYER, PARTY),
  actor("n-orc", "Orc"),
  grant("n-orc", me, NON_PARTY),
  token("t-bram", "a-bram", 5, 5),
  token("t-orc", "n-orc", 2, 6),
];

for (const role of ["dm", "agent"] as const) {
  test(`${role}: the chips are every actor they control in the world, a click picks one and sends nothing`, async () => {
    // A DM's and an agent's chips are the actors granted to them, and send nothing.
    const me = `p-${role}`;
    const { r, s, sock } = await open({ role, participantId: me, log: worldLog(me) });
    expect(labels(chips(r))).toEqual(["Bram", "Orc"]);
    const notPicked = chips(r).find((b) => !b.classList.contains("sel"));
    expect(notPicked).toBeDefined();
    const label = notPicked!.textContent;
    const from = sock.sent.length;
    notPicked!.click();
    await settle();
    expect(sentSince(sock, from)).toHaveLength(0);
    expect(marked(chips(r), "sel")).toEqual([label ?? ""]);
    s?.close();
  });
}
