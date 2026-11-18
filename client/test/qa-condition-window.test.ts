import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { boot } from "../src/app";

type Frame = Record<string, unknown>;

class FakeSocket {
  static readonly OPEN = 1;
  static instances: FakeSocket[] = [];
  readyState = 0;
  readonly sent: string[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((ev: { code: number; reason: string }) => void) | null = null;
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
    this.onclose?.({ code: 1000, reason: "" });
  }
}

const nativeFetch = globalThis.fetch;
const nativeWS = globalThis.WebSocket;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL("http://localhost/");
});

afterEach(() => {
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
});

function root(): HTMLElement {
  const el = document.createElement("div");
  el.id = "app";
  document.body.replaceChildren(el);
  return el;
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

function opensOn(sock: FakeSocket, viewpoint: string, actors: [string, string][] = [["a1", "Lera"]]) {
  sock.deliver({ watchableChanged: { actors: actors.map(([actorId, name]) => ({ actorId, name })) } });
  sock.deliver({ viewpointChanged: viewpoint === "" ? {} : { actorId: viewpoint } });
}

function seedTable(sock: FakeSocket, actorId = "a1", participantId = "p", abilityIds: string[] = []) {
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  sock.deliver(envelope(2, { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8 } }));
  sock.deliver(envelope(3, { actorAdded: { actor: { actorId, name: "Lera", abilityIds } } }));
  sock.deliver(
    envelope(4, {
      actorControlGranted: { actorId, participantId, kind: "ACTOR_KIND_PARTY_MEMBER" },
    }),
  );
  sock.deliver(envelope(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId, position: { x: 1, y: 1 } } }));
}

type Cond = { id: string; name: string; description: string };
type Answer = { conditions?: Cond[] };

const reads: Array<(body: unknown) => void> = [];

function stubRoutes(role: string) {
  reads.length = 0;
  const json = (b: unknown) =>
    new Response(JSON.stringify(b), { status: 200, headers: { "content-type": "application/json" } });
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") return json({ participantId: "p", name: "Lera", role });
    if (path === "/api/adventures") return json({ adventures: [] });
    if (path === "/api/ruleset") {
      return new Promise<Response>((resolve) => {
        reads.push((body) => resolve(json(body)));
      });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;
}

function ruleset(a: Answer) {
  const body: Record<string, unknown> = { id: "r", name: "R", abilities: [], resources: [], properties: [] };
  if (a.conditions !== undefined) body["conditions"] = a.conditions;
  return body;
}

async function answer(i: number, a: Answer) {
  const read = reads[i];
  expect(read).toBeDefined();
  read!(ruleset(a));
  await settle();
}

const cond = (id: string, name: string): Cond => ({ id, name, description: "" });

type Win = { r: HTMLElement; s: ReturnType<typeof boot>; sock: FakeSocket };

async function playerWindow(): Promise<Win> {
  localStorage.setItem("vtt.token", "tok");
  stubRoutes("player");
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  FakeSocket.instances = [];
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  opensOn(sock, "a1");
  seedTable(sock, "a1", "p");
  sock.deliver(envelope(6, { conditionApplied: { actorId: "a1", conditionId: "stunned" } }));
  await settle();
  sock.deliver({
    watchableChanged: { actors: [{ actorId: "a1", name: "Lera", abilityIds: ["poke"] }] },
  });
  await settle();
  return { r, s, sock };
}

async function dmWindow(): Promise<Win> {
  localStorage.setItem("vtt.token", "tok");
  stubRoutes("dm");
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  FakeSocket.instances = [];
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock, "a1", "p-dm");
  sock.deliver(envelope(6, { conditionApplied: { actorId: "a1", conditionId: "stunned" } }));
  sock.deliver(envelope(7, { conditionApplied: { actorId: "a1", conditionId: "mystery" } }));
  sock.deliver(envelope(8, { conditionRemoved: { actorId: "a1", conditionId: "stunned" } }));
  sock.deliver(envelope(9, { conditionApplied: { actorId: "a1", conditionId: "stunned" } }));
  await settle();
  return { r, s, sock };
}

function dotTitles(r: HTMLElement): string[] {
  const disc = r.querySelector('[data-token-id="t1"]')!;
  expect(disc).not.toBeNull();
  return Array.from(disc.querySelectorAll(".dot")).map((d) => d.getAttribute("title") ?? "");
}

function feedText(r: HTMLElement): string {
  return r.querySelector(".feed")!.textContent ?? "";
}

function tickText(r: HTMLElement): string[] {
  return Array.from(r.querySelectorAll(".ticker .tick")).map((t) => t.textContent ?? "");
}

function removeButtons(r: HTMLElement): string[] {
  return Array.from(r.querySelectorAll('[data-action="remove-condition"]')).map((b) => b.textContent ?? "");
}

// VTT-865 VTT-862 VTT-863
test("a player's dot and Story take the newest answer's name", async () => {
  const { r, s } = await playerWindow();
  expect(reads.length).toBeGreaterThanOrEqual(2);
  await answer(reads.length - 1, { conditions: [cond("stunned", "Dazed")] });
  expect(dotTitles(r)).toEqual(["Dazed"]);
  expect(feedText(r)).toContain("gained Dazed");
  s?.close();
});

// VTT-865 VTT-862 VTT-863
test("a player's older answer arriving after the newest changes neither dot nor Story", async () => {
  const { r, s } = await playerWindow();
  expect(reads.length).toBeGreaterThanOrEqual(2);
  const newest = reads.length - 1;
  await answer(newest, { conditions: [cond("stunned", "Dazed")] });
  await answer(0, { conditions: [cond("stunned", "Old")] });
  expect(dotTitles(r)).toEqual(["Dazed"]);
  expect(feedText(r)).toContain("gained Dazed");
  expect(feedText(r)).not.toContain("Old");
  s?.close();
});

// VTT-865 VTT-862 VTT-863
test("a player's newer answer with a different name replaces the old name", async () => {
  const { r, sock, s } = await playerWindow();
  await answer(reads.length - 1, { conditions: [cond("stunned", "Old")] });
  expect(dotTitles(r)).toEqual(["Old"]);
  const before = reads.length;
  sock.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Lera", abilityIds: ["poke", "jab"] }] } });
  await settle();
  expect(reads.length).toBe(before + 1);
  await answer(reads.length - 1, { conditions: [cond("stunned", "Dazed")] });
  expect(dotTitles(r)).toEqual(["Dazed"]);
  expect(feedText(r)).toContain("gained Dazed");
  expect(feedText(r)).not.toContain("Old");
  s?.close();
});

// VTT-865 VTT-862 VTT-863
test("a player's newest answer with no condition list reads the id", async () => {
  const { r, sock, s } = await playerWindow();
  await answer(reads.length - 1, { conditions: [cond("stunned", "Old")] });
  expect(dotTitles(r)).toEqual(["Old"]);
  const before = reads.length;
  sock.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Lera", abilityIds: ["poke", "jab"] }] } });
  await settle();
  expect(reads.length).toBe(before + 1);
  await answer(reads.length - 1, {});
  expect(dotTitles(r)).toEqual(["stunned"]);
  expect(feedText(r)).toContain("gained stunned");
  s?.close();
});

// VTT-865 VTT-862 VTT-863
test("a player's newest answer with an empty condition list reads the id", async () => {
  const { r, sock, s } = await playerWindow();
  await answer(reads.length - 1, { conditions: [cond("stunned", "Old")] });
  expect(dotTitles(r)).toEqual(["Old"]);
  const before = reads.length;
  sock.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Lera", abilityIds: ["poke", "jab"] }] } });
  await settle();
  expect(reads.length).toBe(before + 1);
  await answer(reads.length - 1, { conditions: [] });
  expect(dotTitles(r)).toEqual(["stunned"]);
  expect(feedText(r)).toContain("gained stunned");
  s?.close();
});

// VTT-865 VTT-862 VTT-863 VTT-864
test("a DM's dot, Story, Events row and removal button take its answer's name", async () => {
  const { r, s } = await dmWindow();
  expect(reads.length).toBe(1);
  await answer(0, { conditions: [cond("stunned", "Dazed")] });
  expect(dotTitles(r)).toContain("Dazed");
  expect(feedText(r)).toContain("gained Dazed");
  expect(feedText(r)).toContain("lost Dazed");
  expect(tickText(r).some((t) => t.includes("gained Dazed"))).toBe(true);
  expect(tickText(r).some((t) => t.includes("lost Dazed"))).toBe(true);
  expect(removeButtons(r)).toContain("Lera: Dazed ✕");
  s?.close();
});

// VTT-865 VTT-862 VTT-863 VTT-864
test("a DM's window reads the id of a condition its answer does not declare, everywhere", async () => {
  const { r, s } = await dmWindow();
  await answer(0, { conditions: [cond("stunned", "Dazed")] });
  expect(dotTitles(r)).toContain("mystery");
  expect(feedText(r)).toContain("gained mystery");
  expect(tickText(r).some((t) => t.includes("gained mystery"))).toBe(true);
  expect(removeButtons(r)).toContain("Lera: mystery ✕");
  s?.close();
});

// VTT-865 VTT-862 VTT-863 VTT-864
test("a DM's window holding no answer reads every id", async () => {
  const { r, s } = await dmWindow();
  expect(dotTitles(r)).toContain("stunned");
  expect(feedText(r)).toContain("gained stunned");
  expect(removeButtons(r)).toContain("Lera: stunned ✕");
  s?.close();
});

// VTT-865 VTT-862
test("a spectator's dot is titled by its answer's name", async () => {
  localStorage.setItem("vtt.token", "tok");
  stubRoutes("spectator");
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  FakeSocket.instances = [];
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  opensOn(sock, "a1");
  seedTable(sock, "a1", "p");
  sock.deliver(envelope(6, { conditionApplied: { actorId: "a1", conditionId: "stunned" } }));
  await settle();
  expect(reads.length).toBe(1);
  await answer(0, { conditions: [cond("stunned", "Dazed")] });
  expect(dotTitles(r)).toEqual(["Dazed"]);
  expect(feedText(r)).toContain("gained Dazed");
  s?.close();
});
