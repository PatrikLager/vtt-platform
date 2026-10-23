import "./support/dom";
import { afterEach, beforeEach, expect, test } from "bun:test";
import { boot } from "../src/app";

type Frame = Record<string, unknown>;
type Cmd = Record<string, any>;

const ME = "p-me";
const PM = "ACTOR_KIND_PARTY_MEMBER";

class FakeWS {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: FakeWS[] = [];
  url: string;
  readyState = 0;
  sent: string[] = [];
  onopen: ((ev: unknown) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  onclose: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(url: string) {
    this.url = String(url);
    FakeWS.instances.push(this);
  }
  send(data: string): void {
    this.sent.push(String(data));
  }
  close(): void {
    this.readyState = 3;
  }
  open(): void {
    this.readyState = 1;
    this.onopen?.({});
  }
  deliver(frame: Frame): void {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
}

const g = globalThis as any;
const realWS = g.WebSocket;
const realFetch = g.fetch;
let root: HTMLElement;
let session: { close(): void } | null = null;
let role = "player";
let meGate: Promise<void> | null = null;
let meCalls = 0;

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "content-type": "application/json" } });
}

beforeEach(() => {
  try { localStorage.clear(); } catch {}
  try { sessionStorage.clear(); } catch {}
  g.happyDOM.setURL("http://localhost/?token=tok");
  FakeWS.instances = [];
  role = "player";
  meGate = null;
  meCalls = 0;
  g.WebSocket = FakeWS;
  g.fetch = async (input: unknown) => {
    const raw = typeof input === "string" ? input : (input as any)?.url ?? String(input);
    const path = new URL(raw, "http://localhost").pathname;
    if (path === "/api/me") {
      meCalls++;
      if (meGate) await meGate;
      return json({ participantId: ME, name: "Me", role });
    }
    if (path === "/api/ruleset") return json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    if (path === "/api/adventures") return json({ adventures: [] });
    return new Response("not found", { status: 404 });
  };
  root = document.createElement("div");
  document.body.appendChild(root);
});

afterEach(() => {
  try { session?.close(); } catch {}
  session = null;
  root.remove();
  g.WebSocket = realWS;
  g.fetch = realFetch;
  try { localStorage.clear(); } catch {}
  try { sessionStorage.clear(); } catch {}
  g.happyDOM.setURL("http://localhost/");
});

async function settle(): Promise<void> {
  for (let round = 0; round < 4; round++) {
    for (let i = 0; i < 20; i++) await Promise.resolve();
    await new Promise((r) => setTimeout(r, 0));
  }
}

async function until(pred: () => boolean, what: string): Promise<void> {
  for (let i = 0; i < 50; i++) {
    if (pred()) return;
    await settle();
  }
  throw new Error(`never: ${what}\n${root.innerHTML.slice(0, 3000)}`);
}

async function start(): Promise<FakeWS> {
  session = boot(root) as any;
  await settle();
  const ws = FakeWS.instances[FakeWS.instances.length - 1]!;
  ws.open();
  await settle();
  return ws;
}

async function send(ws: FakeWS, ...frames: Frame[]): Promise<void> {
  for (const f of frames) {
    ws.deliver(f);
    await settle();
  }
}

function logOf(payloads: Frame[]): Frame[] {
  return payloads.map((p, i) => ({ event: { eventId: `e-${i + 1}`, sequence: String(i + 1), sessionId: "s", ...p } }));
}

function board(): Frame[] {
  return logOf([
    { sessionStarted: { name: "Night" } },
    { sceneCreated: { sceneId: "sc1", name: "Hall", gridWidth: 4, gridHeight: 4 } },
    { actorAdded: { actor: { actorId: "a1", name: "Lera" } } },
    { actorControlGranted: { actorId: "a1", participantId: ME, kind: PM } },
    { actorAdded: { actor: { actorId: "a2", name: "Bram" } } },
    { actorControlGranted: { actorId: "a2", participantId: "p-other", kind: PM } },
    { actorAdded: { actor: { actorId: "a3", name: "" } } },
    { actorControlGranted: { actorId: "a3", participantId: "p-third", kind: PM } },
    { actorAdded: { actor: { actorId: "a4", name: "Dara" } } },
    { actorControlGranted: { actorId: "a4", participantId: ME, kind: PM } },
    { tokenPlaced: { tokenId: "t1", sceneId: "sc1", actorId: "a1", position: { x: 0, y: 0 } } },
    { tokenPlaced: { tokenId: "t2", sceneId: "sc1", actorId: "a2", position: { x: 2, y: 1 } } },
    { tokenPlaced: { tokenId: "t3", sceneId: "sc1", actorId: "a3", position: { x: 1, y: 2 } } },
    { tokenPlaced: { tokenId: "t4", sceneId: "sc1", actorId: "a4", position: { x: 1, y: 2 } } },
  ]);
}

const SET = [
  { actorId: "a1", name: "Lera" },
  { actorId: "a2", name: "Bram" },
  { actorId: "a3" },
];

function watchable(actors: Frame[], flag?: boolean): Frame {
  const body: Frame = { actors };
  if (flag !== undefined) body.controlsNoPartyMember = flag;
  return { watchableChanged: body };
}

function viewpoint(actorId: string): Frame {
  return { viewpointChanged: actorId ? { actorId } : {} };
}

function count(sel: string): number {
  return root.querySelectorAll(sel).length;
}

function labels(sel: string): string[] {
  return Array.from(root.querySelectorAll(sel)).map((b) => (b.textContent ?? "").trim()).sort();
}

function shoulders(): string[] {
  return labels(".perch button.shoulder");
}

function chips(): string[] {
  return labels(".player .row button");
}

function switchButtons(): number {
  return count(".perch button.shoulder") + count("button.unperch") + count(".player .row button");
}

function perchedOn(): string {
  return (root.querySelector(".perch .perched-on")?.textContent ?? "").trim();
}

function cmds(ws: FakeWS): Cmd[] {
  return ws.sent.map((s) => JSON.parse(s));
}

function viewpointsSent(ws: FakeWS, from = 0): string[] {
  return cmds(ws).slice(from).filter((c) => c.setViewpoint !== undefined).map((c) => c.setViewpoint.actorId ?? "");
}

function click(x: number, y: number): void {
  const grid = root.querySelector(".grid");
  if (!grid) throw new Error("no board drawn");
  grid.dispatchEvent(new MouseEvent("click", { clientX: 80 + 120 * x + 10, clientY: 120 * y + 10, bubbles: true }));
}

function press(sel: string, label: string): void {
  const b = Array.from(root.querySelectorAll(sel)).find((e) => (e.textContent ?? "").trim() === label) as HTMLElement | undefined;
  if (!b) throw new Error(`no ${sel} labelled ${label}`);
  b.click();
}

async function perchedPlayer(): Promise<FakeWS> {
  const ws = await start();
  await send(ws, watchable(SET, true), viewpoint("a1"), ...board());
  await until(() => count(".perch") === 1 && count(`[data-token-id="t1"]`) === 1, "the perch and the board");
  return ws;
}

async function listedPlayer(flag?: boolean): Promise<FakeWS> {
  const ws = await start();
  await send(ws, watchable(SET, flag), viewpoint("a1"), ...board());
  await until(() => count(".player") === 1 && count(`[data-token-id="t1"]`) === 1, "the player panel and the board");
  return ws;
}

// VTT-458 VTT-404 VTT-445
test("qaNP: a player whose set says they control no party member draws the perch with exactly the set and nobody, and no player panel", async () => {
  await perchedPlayer();
  expect(count(".perch h2")).toBe(1);
  expect((root.querySelector(".perch h2")?.textContent ?? "").trim()).toBe("Whose eyes");
  expect(shoulders()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(labels("button.unperch")).toEqual(["No shoulder"]);
  expect(count(".player")).toBe(0);
  expect(count(".player .row button")).toBe(0);
  expect(count("section.ticker")).toBe(0);
});

// VTT-407
test("qaNP: a perched player's perch shows as watched the actor each ViewpointChanged names, not a command's result", async () => {
  const ws = await perchedPlayer();
  expect(perchedOn()).toBe("Perched on: Lera");
  press(".perch button.shoulder", "Bram");
  await settle();
  const sent = cmds(ws).filter((c) => c.setViewpoint !== undefined);
  expect(sent.length).toBe(1);
  await send(ws, { result: { requestId: sent[0]!.requestId, ok: true } });
  expect(perchedOn()).toBe("Perched on: Lera");
  await send(ws, viewpoint("a2"), ...board());
  expect(perchedOn()).toBe("Perched on: Bram");
  await send(ws, viewpoint(""));
  expect(perchedOn()).toBe("Perched on: nobody");
  expect(count(".player")).toBe(0);
});

// VTT-459
test("qaNP: No shoulder in a player's perch sends set_viewpoint naming nobody", async () => {
  const ws = await perchedPlayer();
  const before = ws.sent.length;
  press("button.unperch", "No shoulder");
  await settle();
  expect(viewpointsSent(ws, before)).toEqual([""]);
});

// VTT-405
test("qaNP: a shoulder in a player's perch sends set_viewpoint naming it, the watched one and one the log has not shown included", async () => {
  const ws = await perchedPlayer();
  const before = ws.sent.length;
  press(".perch button.shoulder", "Bram");
  await settle();
  press(".perch button.shoulder", "Lera");
  await settle();
  press(".perch button.shoulder", "a3");
  await settle();
  expect(viewpointsSent(ws, before)).toEqual(["a2", "a1", "a3"]);
});

// VTT-461 VTT-460 VTT-408
test("qaNP: a perched player's board click sends nothing, where a listed player's same click switches or moves", async () => {
  const ws = await perchedPlayer();
  const before = ws.sent.length;
  click(2, 1);
  await settle();
  click(1, 2);
  await settle();
  click(3, 3);
  await settle();
  click(0, 0);
  await settle();
  expect(ws.sent.length).toBe(before);
  await send(ws, watchable(SET));
  await until(() => count(".player") === 1 && count(".grid") === 1, "the player panel after the flag drops");
  const live = ws.sent.length;
  click(2, 1);
  await settle();
  expect(viewpointsSent(ws, live)).toEqual(["a2"]);
  const moveFrom = ws.sent.length;
  click(3, 3);
  await settle();
  const moves = cmds(ws).slice(moveFrom).filter((c) => c.moveToken !== undefined);
  expect(moves.length).toBe(1);
  expect(moves[0]!.moveToken.tokenId).toBe("t1");
});

// VTT-457
test("qaNP: a player whose set does not say they control no party member is offered exactly the set, and no nobody", async () => {
  await listedPlayer();
  expect(chips()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(0);
  expect(count(".perch button.shoulder")).toBe(0);
  expect(count("section.ticker")).toBe(0);
});

// VTT-457
test("qaNP: a set that says controlsNoPartyMember false keeps the player panel with exactly the set", async () => {
  await listedPlayer(false);
  expect(chips()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(0);
  expect(count(".perch")).toBe(0);
});

// VTT-457
test("qaNP: a player is offered no switch button before a set, whatever the log shows them controlling", async () => {
  const ws = await start();
  await until(() => count(".player") === 1, "the player panel");
  await send(ws, viewpoint("a1"), ...board());
  await until(() => count(`[data-token-id="t1"]`) === 1, "the board");
  expect(switchButtons()).toBe(0);
  await send(ws, watchable(SET));
  await until(() => count(".player .row button") > 0, "chips once a set arrives");
  expect(chips()).toEqual(["Bram", "Lera", "a3"].sort());
});

// VTT-457 VTT-458
test("qaNP: each set's flag moves a player between the perch and the player panel", async () => {
  const ws = await perchedPlayer();
  await send(ws, watchable(SET));
  await until(() => count(".player") === 1, "the player panel");
  expect(count(".perch")).toBe(0);
  expect(count("button.unperch")).toBe(0);
  expect(chips()).toEqual(["Bram", "Lera", "a3"].sort());
  const SET2 = [{ actorId: "a2", name: "Bram" }, { actorId: "a4", name: "Dara" }];
  await send(ws, watchable(SET2, true));
  await until(() => count(".perch") === 1, "the perch again");
  expect(count(".player")).toBe(0);
  expect(shoulders()).toEqual(["Bram", "Dara"]);
  expect(count("button.unperch")).toBe(1);
});

// VTT-458
test("qaNP: after a log that cannot fold, a discard keeps the set, so the player is drawn the perch again", async () => {
  const ws = await perchedPlayer();
  await send(ws, { event: { eventId: "e-99", sequence: "15", sessionId: "s", sessionStarted: { name: "Again" } } });
  await until(() => count("p.fatal") === 1, "the reason the log cannot fold");
  await send(ws, viewpoint("a2"), ...board());
  await until(() => count(".perch") === 1, "the perch after the discard");
  expect(count("p.fatal")).toBe(0);
  expect(count(".player")).toBe(0);
  expect(shoulders()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(1);
});

// VTT-458 VTT-445
test("qaNP: a set sent before /api/me answers player still draws the perch once it does", async () => {
  let release: () => void = () => {};
  meGate = new Promise<void>((r) => { release = r; });
  const ws = await start();
  await send(ws, watchable(SET, true), viewpoint("a1"), ...board());
  await until(() => count(`[data-token-id="t1"]`) === 1, "the board");
  expect(count("section.ticker")).toBe(0);
  release();
  await until(() => count(".perch") === 1 && count(".perch button.shoulder") > 0, "the perch once the role is read");
  expect(count(".player")).toBe(0);
  expect(shoulders()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(1);
  expect(count("section.ticker")).toBe(0);
});

// VTT-458 VTT-457
test("qaNP: a spectator re-read as player keeps the perch while the set says no party member, and gets the panel once it does not", async () => {
  role = "spectator";
  const ws = await start();
  await send(ws, watchable(SET, true), viewpoint("a1"), ...board());
  await until(() => count(".perch") === 1, "the spectator's perch");
  role = "player";
  const reads = meCalls;
  await send(ws, { presenceChanged: { participantId: ME, displayName: "Me", state: "PRESENCE_STATE_CONNECTED" } });
  await until(() => meCalls === reads + 1, "the re-read of /api/me");
  await settle();
  expect(count(".perch")).toBe(1);
  expect(count(".player")).toBe(0);
  expect(shoulders()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(1);
  await send(ws, watchable(SET));
  await until(() => count(".player") === 1, "the player panel once the set has no flag");
  expect(count(".perch")).toBe(0);
  expect(chips()).toEqual(["Bram", "Lera", "a3"].sort());
});

// VTT-403
test("qaNP: a spectator is drawn the perch with exactly the set and nobody, whether or not the set carries the flag", async () => {
  role = "spectator";
  const ws = await start();
  await send(ws, watchable(SET), viewpoint("a1"), ...board());
  await until(() => count(".perch") === 1, "the spectator's perch");
  expect(shoulders()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(1);
  expect(count(".player")).toBe(0);
  await send(ws, watchable(SET, true));
  await settle();
  expect(shoulders()).toEqual(["Bram", "Lera", "a3"].sort());
  expect(count("button.unperch")).toBe(1);
  expect(count(".player")).toBe(0);
});

// VTT-460
test("qaNP: a listed player's click on a cell of set members switches to one of them, never to one the set does not name", async () => {
  const ws = await listedPlayer();
  const before = ws.sent.length;
  click(1, 2);
  await settle();
  expect(viewpointsSent(ws, before)).toEqual(["a3"]);
  const next = ws.sent.length;
  click(2, 1);
  await settle();
  expect(viewpointsSent(ws, next)).toEqual(["a2"]);
});

// VTT-408
test("qaNP: a listed player's click on an empty cell moves the watched character's token", async () => {
  const ws = await listedPlayer();
  const before = ws.sent.length;
  click(3, 3);
  await settle();
  const moves = cmds(ws).slice(before).filter((c) => c.moveToken !== undefined);
  expect(moves.length).toBe(1);
  expect(moves[0]!.moveToken.tokenId).toBe("t1");
  expect(moves[0]!.moveToken.to).toEqual({ x: 3, y: 3 });
  expect(viewpointsSent(ws, before)).toEqual([]);
});

// VTT-458
test("qaNP: a perched player whose set names no party member is offered only nobody, and no player panel", async () => {
  const ws = await start();
  await send(ws, watchable([], true), viewpoint(""));
  await until(() => count(".perch") === 1 && count("button.unperch") === 1, "the perch");
  expect(count(".perch button.shoulder")).toBe(0);
  expect(labels("button.unperch")).toEqual(["No shoulder"]);
  expect(count(".player")).toBe(0);
});

// VTT-457
test("qaNP: a player whose set names no party member and no flag is offered no switch button at all", async () => {
  const ws = await start();
  await send(ws, watchable([]), viewpoint("a1"), ...board());
  await until(() => count(".player") === 1 && count(`[data-token-id="t1"]`) === 1, "the player panel and the board");
  expect(switchButtons()).toBe(0);
  expect(count(".perch")).toBe(0);
});
