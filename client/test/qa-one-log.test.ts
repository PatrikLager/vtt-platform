import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { boot } from "../src/app";
import { Session } from "../src/session";

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
    if (this.readyState === 3) return;
    this.readyState = 3;
    this.onclose?.();
  }
  /** The network drops an open connection. */
  drop() {
    this.close();
  }
  /** A dial that never opens: error, then close, as a browser reports it. */
  fail() {
    this.readyState = 3;
    this.onerror?.();
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

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setURL("http://localhost/");
});

afterEach(() => {
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
});

function entry(log: string, seq: number, payload: Frame): Frame {
  return {
    event: {
      eventId: `${log}-${seq}`,
      sequence: String(seq),
      sessionId: "sess-1",
      actorRole: "dm",
      participantId: "p-dm",
      ...payload,
    },
  };
}

const LERA = "a1";
const BRAM = "a2";
const party = (actorId: string, name: string) => ({
  actorAdded: { actor: { actorId, name, kind: "ACTOR_KIND_PARTY_MEMBER" } },
});
const grant = (actorId: string, participantId: string) => ({
  actorControlGranted: { actorId, participantId, kind: "ACTOR_KIND_PARTY_MEMBER" },
});
const narrate = (text: string) => ({
  narrationAdded: { text, as: "", anchorFromSeq: "0", anchorToSeq: "0" },
});

/** Lera's log, seven entries; it introduces Bram too, so a spectator on it is offered his shoulder. */
function leraLog(controller: string): Frame[] {
  return [
    entry("lera", 1, { sessionStarted: { name: "Night" } }),
    entry("lera", 2, { sceneCreated: { sceneId: "hall", name: "Hall", gridWidth: 8, gridHeight: 8 } }),
    entry("lera", 3, party(LERA, "Lera")),
    entry("lera", 4, party(BRAM, "Bram")),
    entry("lera", 5, grant(LERA, controller)),
    entry("lera", 6, { tokenPlaced: { tokenId: "t-lera", sceneId: "hall", actorId: LERA, position: { x: 1, y: 1 } } }),
    entry("lera", 7, narrate("The lamps gutter.")),
  ];
}

/** Bram's log, six entries, sharing no scene, token or narration with Lera's. */
function bramLog(controller: string): Frame[] {
  return [
    entry("bram", 1, { sessionStarted: { name: "Night" } }),
    entry("bram", 2, { sceneCreated: { sceneId: "cellar", name: "Cellar", gridWidth: 6, gridHeight: 6 } }),
    entry("bram", 3, party(BRAM, "Bram")),
    entry("bram", 4, grant(BRAM, controller)),
    entry("bram", 5, { tokenPlaced: { tokenId: "t-bram", sceneId: "cellar", actorId: BRAM, position: { x: 3, y: 2 } } }),
    entry("bram", 6, narrate("Rain on the roof.")),
  ];
}

/** Lera's log started again, four entries; kept beside the old fold, its scene and actor are duplicates. */
function leraRestarted(): Frame[] {
  return [
    entry("lera2", 1, { sessionStarted: { name: "Dawn" } }),
    entry("lera2", 2, { sceneCreated: { sceneId: "hall", name: "Hall", gridWidth: 8, gridHeight: 8 } }),
    entry("lera2", 3, party(LERA, "Lera")),
    entry("lera2", 4, narrate("Morning comes.")),
  ];
}

const vc = (actorId: string): Frame => ({ viewpointChanged: actorId === "" ? {} : { actorId } });

/** A connection's opening: head, presence, and, for a log seat, the set and the viewpoint. */
function openSeat(sock: FakeSocket, viewpoint: string | null, head: number) {
  sock.open();
  sock.deliver({ catchUpHead: { headSequence: String(head) } });
  sock.deliver({ presenceSnapshot: { present: [] } });
  if (viewpoint === null) return;
  sock.deliver({ watchableChanged: { actors: [{ actorId: LERA, name: "Lera" }, { actorId: BRAM, name: "Bram" }] } });
  sock.deliver(vc(viewpoint));
}

function deliverAll(sock: FakeSocket, frames: Frame[]) {
  for (const f of frames) sock.deliver(f);
}

async function tableAs(role: string) {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Me", role },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r)!;
  await settle();
  return { r, s, sock: FakeSocket.instances[0]! };
}

/** Click Reconnect once the connection is closed, and return the new dial. */
async function redial(r: HTMLElement): Promise<FakeSocket> {
  await settle();
  const btn = r.querySelector<HTMLButtonElement>(".reconnect");
  expect(btn).not.toBeNull();
  const before = FakeSocket.instances.length;
  btn!.click();
  await settle();
  expect(FakeSocket.instances.length).toBe(before + 1);
  return FakeSocket.instances[FakeSocket.instances.length - 1]!;
}

function dialOf(sock: FakeSocket): { log: string; after: string | null; hasLog: boolean } {
  const q = new URL(sock.url).searchParams;
  return { log: q.get("log") ?? "", after: q.get("after"), hasLog: (q.get("log") ?? "") !== "" };
}

const text = (r: HTMLElement) => r.textContent ?? "";
const perchedOn = (r: HTMLElement) => r.querySelector(".perch .perched-on")?.textContent ?? null;
const shoulder = (r: HTMLElement, name: string) =>
  Array.from(r.querySelectorAll<HTMLButtonElement>(".perch .shoulder")).find((b) => b.textContent === name);

// VTT-394
test.each(["spectator", "player"])("%s: a ViewpointChanged naming another party member discards Lera's fold and folds Bram's", async (role) => {
  const ctl = role === "player" ? "p" : "p-other";
  const { r, s, sock } = await tableAs(role);
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog(ctl));
  await settle();
  expect(text(r)).toContain("The lamps gutter.");

  sock.deliver(vc(BRAM));
  deliverAll(sock, bramLog(ctl));
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(text(r)).toContain("Rain on the roof.");
  expect(text(r)).not.toContain("The lamps gutter.");
  expect(Object.keys(s.state.Scenes)).toEqual(["cellar"]);
  expect(Object.keys(s.state.Tokens)).toEqual(["t-bram"]);
  expect(s.events).toHaveLength(6);
  s.close();
});

// VTT-394
test("a ViewpointChanged naming nobody while a log is held discards it and shows no board", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();
  expect(Object.keys(s.state.Tokens)).toEqual(["t-lera"]);

  sock.deliver(vc(""));
  await settle();

  expect(s.events).toHaveLength(0);
  expect(Object.keys(s.state.Scenes)).toEqual([]);
  expect(Object.keys(s.state.Tokens)).toEqual([]);
  expect(text(r)).not.toContain("The lamps gutter.");
  s.close();
});

// VTT-394
test("a log that could not fold is discarded at a switch, and the next log's board replaces the reason", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 8);
  deliverAll(sock, leraLog("p-other"));
  sock.deliver(entry("lera", 8, { tokenMoved: { tokenId: "ghost", sceneId: "hall", to: { x: 2, y: 2 } } }));
  await settle();
  expect(text(r)).toContain("Cannot derive state from the log");

  sock.deliver(vc(BRAM));
  deliverAll(sock, bramLog("p-other"));
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(text(r)).not.toContain("Cannot derive state from the log");
  expect(text(r)).toContain("Rain on the roof.");
  s.close();
});

// VTT-395
test("an entry of sequence 1 on the live connection, with no ViewpointChanged, starts the held log again", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  await settle();

  deliverAll(sock, leraRestarted());
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(s.events).toHaveLength(4);
  expect(text(r)).toContain("Morning comes.");
  expect(text(r)).not.toContain("The lamps gutter.");
  expect(Object.keys(s.state.Tokens)).toEqual([]);
  s.close();
});

// VTT-395
test("an entry of sequence 1 clears the reason a log could not fold", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 8);
  deliverAll(sock, leraLog("p"));
  sock.deliver(entry("lera", 8, { tokenMoved: { tokenId: "ghost", sceneId: "hall", to: { x: 2, y: 2 } } }));
  await settle();
  expect(text(r)).toContain("Cannot derive state from the log");

  deliverAll(sock, leraRestarted());
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(text(r)).toContain("Morning comes.");
  s.close();
});

// VTT-396
test("a ViewpointChanged naming the held log on the live connection keeps the fold, and later entries add to it", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();

  sock.deliver(vc(LERA));
  sock.deliver(entry("lera", 8, narrate("A door creaks.")));
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(s.events).toHaveLength(8);
  expect(text(r)).toContain("The lamps gutter.");
  expect(text(r)).toContain("A door creaks.");
  expect(Object.keys(s.state.Tokens)).toEqual(["t-lera"]);
  s.close();
});

// VTT-396, VTT-397, VTT-398
test("a redial the gateway opens on the held log after the cursor keeps the fold, and the next redial resumes past it", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: LERA, after: "7", hasLog: true });
  openSeat(second, LERA, 9);
  second.deliver(entry("lera", 8, narrate("A door creaks.")));
  second.deliver(entry("lera", 9, { tokenMoved: { tokenId: "t-lera", sceneId: "hall", to: { x: 2, y: 1 } } }));
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(s.events).toHaveLength(9);
  expect(text(r)).toContain("The lamps gutter.");
  expect(text(r)).toContain("A door creaks.");
  expect(s.state.Tokens["t-lera"]).toMatchObject({ X: 2, Y: 1 });
  second.drop();

  const third = await redial(r);
  expect(dialOf(third)).toEqual({ log: LERA, after: "9", hasLog: true });
  s.close();
});

// VTT-394, VTT-397, VTT-398
test("a redial the gateway opens on another log shows that log alone, and the next redial names it", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  await settle();
  sock.drop();

  const second = await redial(r);
  openSeat(second, BRAM, 6);
  deliverAll(second, bramLog("p"));
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(text(r)).toContain("Rain on the roof.");
  expect(text(r)).not.toContain("The lamps gutter.");
  expect(s.events).toHaveLength(6);
  second.drop();

  const third = await redial(r);
  expect(dialOf(third)).toEqual({ log: BRAM, after: "6", hasLog: true });
  s.close();
});

// VTT-395, VTT-397, VTT-398
test("a redial the gateway opens on the held log from its first entry refolds it, and the next redial resumes the new start", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: LERA, after: "7", hasLog: true });
  openSeat(second, LERA, 4);
  deliverAll(second, leraRestarted());
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(s.events).toHaveLength(4);
  expect(text(r)).toContain("Morning comes.");
  expect(text(r)).not.toContain("The lamps gutter.");
  second.drop();

  const third = await redial(r);
  expect(dialOf(third)).toEqual({ log: LERA, after: "4", hasLog: true });
  s.close();
});

// VTT-397, VTT-398
test.each(["spectator", "player"])("%s: a redial names the log held and the highest sequence sent of it", async (role) => {
  const ctl = role === "player" ? "p" : "p-other";
  const { r, s, sock } = await tableAs(role);
  openSeat(sock, BRAM, 6);
  deliverAll(sock, bramLog(ctl));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: BRAM, after: "6", hasLog: true });
  s.close();
});

// VTT-397, VTT-398
test.each(["dm", "agent"])("%s: a redial names no log and the highest world sequence sent", async (role) => {
  const { r, s, sock } = await tableAs(role);
  openSeat(sock, null, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: "", after: "7", hasLog: false });
  s.close();
});

// VTT-397, VTT-398
test.each(["spectator", "player"])("%s: a redial while watching nobody from the opening names no log", async (role) => {
  const { r, s, sock } = await tableAs(role);
  openSeat(sock, "", 0);
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: "", after: "0", hasLog: false });
  s.close();
});

// VTT-397, VTT-398
test("a redial after a switch to nobody names no log, though a log was held before", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  sock.deliver(vc(""));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: "", after: "0", hasLog: false });
  s.close();
});

// VTT-397, VTT-398
test("a redial after a switch names the new log and its highest sequence, not the old log's", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  sock.deliver(vc(BRAM));
  deliverAll(sock, bramLog("p").slice(0, 3));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: BRAM, after: "3", hasLog: true });
  s.close();
});

// VTT-397, VTT-398
test("a redial after a switch but before the new log's first entry names the new log from its start", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  sock.deliver(vc(BRAM));
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: BRAM, after: "0", hasLog: true });
  s.close();
});

// VTT-398
test("a redial after the held log started again names the highest sequence since the restart, not before it", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  deliverAll(sock, leraRestarted());
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: LERA, after: "4", hasLog: true });
  s.close();
});

// VTT-398
test("a player's command result carries a world sequence, which a redial's after does not take", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  await settle();

  r.querySelector(".grid")!.dispatchEvent(new MouseEvent("click", { clientX: 5, clientY: 5, bubbles: true }));
  await settle();
  expect(sock.sent.length).toBeGreaterThan(0);
  const requestId = JSON.parse(sock.sent[0]!).requestId as string;
  sock.deliver({ result: { requestId, ok: true, sequence: "40" } });
  await settle();
  sock.drop();

  const second = await redial(r);
  expect(dialOf(second)).toEqual({ log: LERA, after: "7", hasLog: true });
  s.close();
});

// VTT-397, VTT-398
test("Session: a redial that failed to open leaves the next one naming the same log and cursor, and the fold in place", async () => {
  useFakeSocket();
  const s = new Session("ws://localhost/ws", "tok");
  const started = s.start();
  const first = FakeSocket.instances[0]!;
  openSeat(first, LERA, 7);
  await started;
  deliverAll(first, leraLog("p"));
  await settle();
  first.drop();
  await settle();

  const failing = s.reconnect().then(() => "opened", () => "failed");
  const failed = FakeSocket.instances[1]!;
  expect(dialOf(failed)).toEqual({ log: LERA, after: "7", hasLog: true });
  failed.fail();
  expect(await failing).toBe("failed");
  expect(s.events).toHaveLength(7);

  const retrying = s.reconnect();
  const third = FakeSocket.instances[2]!;
  expect(dialOf(third)).toEqual({ log: LERA, after: "7", hasLog: true });
  openSeat(third, LERA, 8);
  await retrying;
  third.deliver(entry("lera", 8, narrate("A door creaks.")));
  await settle();

  expect(s.failure).toBe("");
  expect(s.events).toHaveLength(8);
  expect(Object.keys(s.state.Tokens)).toEqual(["t-lera"]);
  s.close();
});

// VTT-407
test("the perched-on line follows each ViewpointChanged: Lera, then Bram, then nobody", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();
  expect(perchedOn(r)).toBe("Perched on: Lera");

  sock.deliver(vc(BRAM));
  deliverAll(sock, bramLog("p-other"));
  await settle();
  expect(perchedOn(r)).toBe("Perched on: Bram");
  expect(s.viewpoint).toBe(BRAM);

  sock.deliver(vc(""));
  await settle();
  expect(perchedOn(r)).toBe("Perched on: nobody");
  expect(s.viewpoint).toBe("");
  s.close();
});

// VTT-407
test("an accepted set_viewpoint with no ViewpointChanged leaves the line on the shoulder the server last named", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();

  const bram = shoulder(r, "Bram");
  expect(bram).toBeDefined();
  bram!.click();
  await settle();
  const cmd = JSON.parse(sock.sent[0]!);
  expect(cmd.setViewpoint).toMatchObject({ actorId: BRAM });
  sock.deliver({ result: { requestId: cmd.requestId as string, ok: true } });
  await settle();
  expect(perchedOn(r)).toBe("Perched on: Lera");

  sock.deliver(vc(BRAM));
  deliverAll(sock, bramLog("p-other"));
  await settle();
  expect(perchedOn(r)).toBe("Perched on: Bram");
  s.close();
});

// VTT-407
test("a refusal arriving after the ViewpointChanged does not move the line back", async () => {
  const { r, s, sock } = await tableAs("spectator");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p-other"));
  await settle();

  shoulder(r, "Bram")!.click();
  await settle();
  const requestId = JSON.parse(sock.sent[0]!).requestId as string;
  sock.deliver(vc(BRAM));
  deliverAll(sock, bramLog("p-other"));
  sock.deliver({ result: { requestId, ok: false, error: "not a party member" } });
  await settle();

  expect(perchedOn(r)).toBe("Perched on: Bram");
  s.close();
});

// VTT-394, VTT-397, VTT-398, VTT-407
test("Session over real websockets: each dial names the log held and its cursor, and a switch on a redial refolds", async () => {
  const dials: string[] = [];
  const scripts: Array<(ws: { send(d: string): void; close(): void }) => void> = [
    (ws) => {
      for (const f of [{ catchUpHead: { headSequence: "7" } }, { presenceSnapshot: { present: [] } }, vc(LERA), ...leraLog("p")]) {
        ws.send(JSON.stringify(f));
      }
    },
    (ws) => {
      for (const f of [{ catchUpHead: { headSequence: "6" } }, { presenceSnapshot: { present: [] } }, vc(BRAM), ...bramLog("p")]) {
        ws.send(JSON.stringify(f));
      }
    },
    () => {},
  ];
  const sockets: Array<{ close(): void }> = [];
  const server = Bun.serve<{ url: string }>({
    port: 0,
    fetch(req, srv) {
      if (srv.upgrade(req, { data: { url: req.url } })) return undefined;
      return new Response("no", { status: 400 });
    },
    websocket: {
      open(ws) {
        dials.push(ws.data.url);
        sockets.push(ws);
        scripts[dials.length - 1]?.(ws);
      },
      message() {},
    },
  });
  const until = async (cond: () => boolean) => {
    for (let i = 0; i < 200 && !cond(); i++) await new Promise((res) => setTimeout(res, 10));
    expect(cond()).toBe(true);
  };
  const s = new Session(`ws://127.0.0.1:${server.port}/ws`, "tok");
  try {
    void s.start();
    await until(() => s.events.length === 7);
    expect(s.viewpoint).toBe(LERA);
    sockets[0]!.close();
    await new Promise((res) => setTimeout(res, 50));

    void s.reconnect();
    await until(() => dials.length === 2 && s.events.length === 6 && s.viewpoint === BRAM);
    const q2 = new URL(dials[1]!).searchParams;
    expect([q2.get("log"), q2.get("after")]).toEqual([LERA, "7"]);
    expect(Object.keys(s.state.Scenes)).toEqual(["cellar"]);
    expect(s.failure).toBe("");
    sockets[1]!.close();
    await new Promise((res) => setTimeout(res, 50));

    void s.reconnect();
    await until(() => dials.length === 3);
    const q3 = new URL(dials[2]!).searchParams;
    expect([q3.get("log"), q3.get("after")]).toEqual([BRAM, "6"]);
  } finally {
    s.close();
    server.stop(true);
  }
});

// VTT-398
test("window: a Reconnect whose dial fails keeps the fold and offers Reconnect again, naming the same log and cursor", async () => {
  const { r, s, sock } = await tableAs("player");
  openSeat(sock, LERA, 7);
  deliverAll(sock, leraLog("p"));
  await settle();
  sock.drop();

  const failed = await redial(r);
  failed.fail();
  await settle();
  expect(s.events).toHaveLength(7);
  expect(text(r)).toContain("The lamps gutter.");
  const third = await redial(r);
  expect(dialOf(third)).toEqual({ log: LERA, after: "7", hasLog: true });
  s.close();
});
