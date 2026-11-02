import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import { boot } from "../src/app";
import { Session } from "../src/session";
import { newState } from "../src/state";
import { renderSpectator } from "../src/view/spectator";
import { forgetTheSay } from "./support/say";

type Actor = { actorId: string; name: string };

const LERA: Actor = { actorId: "a1", name: "Lera" };
const BRAM: Actor = { actorId: "a2", name: "Bram" };
const QUILL: Actor = { actorId: "a3", name: "Quill" };
const PLAYS_NONE = "You play no character; the DM can give you one.";

class FakeSocket {
  static OPEN = 1;
  static all: FakeSocket[] = [];
  readonly OPEN = 1;
  readyState = 0;
  sent: string[] = [];
  onopen: ((e: unknown) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: { code: number; reason: string }) => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  constructor(readonly url: string) {
    FakeSocket.all.push(this);
  }
  send(data: string): void {
    this.sent.push(data);
  }
  close(): void {
    this.readyState = 3;
  }
}

const g = globalThis as unknown as Record<string, unknown>;
const nativeSocket = g.WebSocket;
const nativeFetch = g.fetch;
const roles = new Map<string, string>();
const meReads = new Map<string, number>();
const roots: HTMLElement[] = [];
const tokens: string[] = [];
let lastToken = "";
let serial = 0;

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    headers: { "content-type": "application/json" },
  });
}

function tokenOf(init?: RequestInit): string {
  const auth = new Headers(init?.headers).get("authorization") ?? "";
  const bearer = auth.replace(/^Bearer /, "");
  return roles.has(bearer) ? bearer : lastToken;
}

async function fakeFetch(input: unknown, init?: RequestInit): Promise<Response> {
  const url = String(input);
  if (url.endsWith("/api/me")) {
    const tok = tokenOf(init);
    meReads.set(tok, (meReads.get(tok) ?? 0) + 1);
    return json({ participantId: `p-${tok}`, name: tok, role: roles.get(tok) });
  }
  if (url.endsWith("/api/ruleset")) {
    return json({
      id: "r", name: "R", abilities: [], conditions: [], resources: [],
    });
  }
  if (url.endsWith("/api/adventures")) return json({ adventures: [] });
  return new Response("", { status: 404 });
}

async function settle(): Promise<void> {
  for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0));
}

function watch(actors: Actor[], controlsNone = false): object {
  const frame: Record<string, unknown> = { actors };
  if (controlsNone) frame.controlsNoPartyMember = true;
  return { watchableChanged: frame };
}

function viewpoint(actorId: string): object {
  return { viewpointChanged: actorId === "" ? {} : { actorId } };
}

interface Table {
  root: HTMLElement;
  token: string;
  socket(): FakeSocket;
  frames(...fs: object[]): Promise<void>;
  toast(): string | null;
  perch(): string | null;
  open(): Promise<void>;
}

async function openTable(role: string): Promise<Table> {
  g.WebSocket = FakeSocket;
  g.fetch = fakeFetch;
  const token = `tok${++serial}`;
  roles.set(token, role);
  tokens.push(token);
  lastToken = token;
  localStorage.setItem("vtt.token", token);
  const root = document.createElement("div");
  document.body.appendChild(root);
  roots.push(root);
  boot(root);
  const mine = () => FakeSocket.all.filter((s) => s.url.includes(`token=${token}`));
  const t: Table = {
    root,
    token,
    socket: () => {
      const s = mine()[mine().length - 1];
      if (!s) throw new Error(`no socket for ${token}`);
      return s;
    },
    frames: async (...fs) => {
      for (const f of fs) t.socket().onmessage?.({ data: JSON.stringify(f) });
      await settle();
    },
    toast: () => root.querySelector(".toast")?.textContent ?? null,
    perch: () => root.querySelector(".perch")?.textContent ?? null,
    open: async () => {
      const s = t.socket();
      s.readyState = 1;
      s.onopen?.({});
      await settle();
    },
  };
  await t.open();
  return t;
}

function lastCommand(t: Table): { requestId: string } & Record<string, unknown> {
  const sent = t.socket().sent;
  return JSON.parse(sent[sent.length - 1] ?? "{}");
}

function labels(t: Table, sel: string): string[] {
  return Array.from(t.root.querySelectorAll(sel)).map((b) => b.textContent ?? "");
}

function typeLine(t: Table, text: string): void {
  const box = t.root.querySelector<HTMLInputElement>('[data-field="say-text"]');
  if (!box) throw new Error("no say-text");
  box.value = text;
  box.dispatchEvent(new Event("input"));
  t.root.querySelector<HTMLElement>('[data-action="add-narration"]')?.click();
}

async function answer(t: Table, result: Record<string, unknown>): Promise<void> {
  await t.frames({ result: { requestId: lastCommand(t).requestId, ...result } });
}

afterEach(() => {
  for (const r of roots.splice(0)) r.remove();
  for (const tok of tokens.splice(0)) {
    sessionStorage.removeItem(`vtt.viewpoint.${tok}`);
  }
  localStorage.removeItem("vtt.token");
  FakeSocket.all = [];
  g.WebSocket = nativeSocket;
  g.fetch = nativeFetch;
});

// VTT-604, VTT-407
test("a refused switch shows the error alone and keeps the perch", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA, BRAM], true), viewpoint("a1"));
  t.root.querySelector<HTMLElement>(".shoulder")?.click();
  expect(lastCommand(t).setViewpoint).toEqual({ actorId: "a2" });
  await answer(t, { ok: false, error: "You cannot move there: a wall." });
  expect(t.toast()).toBe("You cannot move there: a wall.");
  expect(t.root.querySelector(".perched-on")?.textContent).toBe("Perched on: Lera");
  t.root.querySelector<HTMLElement>(".unperch")?.click();
  await answer(t, { ok: false, error: "refused: as the server wrote it" });
  expect(t.toast()).toBe("refused: as the server wrote it");
});

// VTT-604, VTT-517
test("a refused console line shows the error alone and keeps the line", async () => {
  const t = await openTable("dm");
  try {
    typeLine(t, "The door creaks.");
    expect(lastCommand(t).addNarration).toBeDefined();
    await answer(t, { ok: false, error: "gateway: not authorized: nope" });
    expect(t.toast()).toBe("gateway: not authorized: nope");
    const box = t.root.querySelector<HTMLInputElement>('[data-field="say-text"]');
    expect(box?.value).toBe("The door creaks.");
  } finally {
    forgetTheSay();
  }
});

// VTT-604
test("an accepted command clears a refusal or shows its warnings", async () => {
  const t = await openTable("dm");
  try {
    typeLine(t, "one");
    await answer(t, { ok: false, error: "Not now." });
    expect(t.toast()).toBe("Not now.");
    typeLine(t, "two");
    await answer(t, { ok: true, warnings: ["art missing", "kind mismatch"] });
    expect(t.toast()).toBe("art missing; kind mismatch");
    typeLine(t, "three");
    await answer(t, { ok: true });
    expect(t.root.querySelector(".toast")).toBeNull();
  } finally {
    forgetTheSay();
  }
});

// VTT-605, VTT-457
test("a player who loses one of two party members is told which", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA, BRAM]), viewpoint("a1"));
  expect(t.root.querySelector(".toast")).toBeNull();
  await t.frames(watch([BRAM]));
  expect(t.toast()).toBe("You no longer play Lera.");
  await t.frames(viewpoint("a2"));
  expect(t.toast()).toBe("You no longer play Lera.");
  expect(labels(t, ".player .chip")).toEqual(["Bram"]);
});

// VTT-605, VTT-458
test("three lost at once are named in the previous set's order", async () => {
  const t = await openTable("player");
  await t.frames(watch([QUILL, LERA, BRAM]), viewpoint("a1"));
  await t.frames(watch([], true));
  expect(t.toast()).toBe("You no longer play Quill and Lera and Bram.");
  expect(t.root.querySelector(".player")).toBeNull();
  expect(labels(t, ".perch .shoulder")).toEqual([]);
  expect(labels(t, ".perch .unperch")).toHaveLength(1);
});

// VTT-605, VTT-404
test("a lost party member with no name is an unnamed character", async () => {
  const t = await openTable("player");
  const nameless: Actor = { actorId: "a9", name: "" };
  await t.frames(watch([nameless, BRAM, QUILL]), viewpoint("a2"));
  expect(labels(t, ".player .chip")).toEqual(["a9", "Bram", "Quill"]);
  await t.frames(watch([BRAM]));
  expect(t.toast()).toBe("You no longer play an unnamed character and Quill.");
});

// VTT-605, VTT-607, VTT-458
test("a set saying the player controls none names none as controlled", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA]), viewpoint("a1"));
  expect(t.root.querySelector(".perch")).toBeNull();
  await t.frames(watch([LERA], true));
  expect(t.toast()).toBe("You no longer play Lera.");
  expect(t.perch()).toContain(PLAYS_NONE);
  expect(labels(t, ".perch .shoulder")).toEqual(["Lera"]);
  expect(t.root.querySelector(".player")).toBeNull();
});

// VTT-605, VTT-606
test("a rename tells nothing; a new id, old name, tells who left", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA]), viewpoint("a1"));
  await t.frames(watch([{ actorId: "a1", name: "Lera the Bold" }]));
  expect(t.root.querySelector(".toast")).toBeNull();
  await t.frames(watch([{ actorId: "a5", name: "Lera the Bold" }]));
  expect(t.toast()).toBe("You no longer play Lera the Bold.");
});

// VTT-606, VTT-605
test("only a party member the set named as controlled is told as lost", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA, BRAM], true), viewpoint("a1"));
  await t.frames(watch([LERA], true));
  expect(t.root.querySelector(".toast")).toBeNull();
  await t.frames(watch([LERA, BRAM]));
  expect(t.root.querySelector(".toast")).toBeNull();
  await t.frames(viewpoint("a2"), watch([BRAM]));
  expect(t.toast()).toBe("You no longer play Lera.");
});

// VTT-606, VTT-607, VTT-457
test("a page's first set tells nothing, though it controls none", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA], true), viewpoint("a1"));
  expect(t.root.querySelector(".toast")).toBeNull();
  expect(t.perch()).toContain(PLAYS_NONE);
  await t.frames(watch([LERA]));
  expect(t.root.querySelector(".perch")).toBeNull();
  expect(labels(t, ".player .chip")).toEqual(["Lera"]);
  expect(t.root.textContent).not.toContain(PLAYS_NONE);
  await t.frames(watch([LERA], true));
  expect(t.toast()).toBe("You no longer play Lera.");
});

// VTT-606, VTT-605, VTT-403
test("a spectator's and a DM's window are told nothing; a player's is", async () => {
  const shrink = [watch([LERA, BRAM]), viewpoint("a1"), watch([BRAM])];
  const spectator = await openTable("spectator");
  await spectator.frames(...shrink);
  expect(spectator.root.querySelector(".toast")).toBeNull();
  expect(labels(spectator, ".perch .shoulder")).toEqual(["Bram"]);
  expect(labels(spectator, ".perch .unperch")).toHaveLength(1);
  const dm = await openTable("dm");
  await dm.frames(...shrink);
  expect(dm.root.querySelector(".toast")).toBeNull();
  const player = await openTable("player");
  await player.frames(...shrink);
  expect(player.toast()).toBe("You no longer play Lera.");
});

// VTT-606
test("a player the client re-reads as a spectator is told nothing", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA, BRAM]), viewpoint("a1"));
  expect(t.root.querySelector(".player")).not.toBeNull();
  roles.set(t.token, "spectator");
  await t.frames({
    presenceChanged: {
      participantId: `p-${t.token}`, displayName: t.token,
      state: "PRESENCE_STATE_CONNECTED",
    },
  });
  expect(meReads.get(t.token)).toBe(2);
  expect(t.root.querySelector(".perch")).not.toBeNull();
  await t.frames(watch([BRAM]));
  expect(t.root.querySelector(".toast")).toBeNull();
  const control = await openTable("player");
  await control.frames(watch([LERA, BRAM]), viewpoint("a1"), watch([BRAM]));
  expect(control.toast()).toBe("You no longer play Lera.");
});

// VTT-605
test("a spectator the client re-reads as a player is told who left", async () => {
  const t = await openTable("spectator");
  roles.set(t.token, "player");
  await t.frames({
    presenceChanged: {
      participantId: `p-${t.token}`, displayName: t.token,
      state: "PRESENCE_STATE_CONNECTED",
    },
  });
  expect(meReads.get(t.token)).toBe(2);
  await t.frames(watch([LERA, BRAM]), viewpoint("a1"));
  expect(t.root.querySelector(".player")).not.toBeNull();
  await t.frames(watch([LERA]));
  expect(t.toast()).toBe("You no longer play Bram.");
});

// VTT-606, VTT-605
test("a redial sent the same set tells nothing", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA, BRAM]), viewpoint("a1"));
  t.socket().onclose?.({ code: 1006, reason: "" });
  await settle();
  const before = t.socket();
  t.root.querySelector<HTMLElement>(".reconnect")?.click();
  expect(t.socket()).not.toBe(before);
  await t.open();
  await t.frames(watch([LERA, BRAM]), viewpoint("a1"));
  expect(t.root.querySelector(".toast")).toBeNull();
  await t.frames(watch([LERA]));
  expect(t.toast()).toBe("You no longer play Bram.");
});

// VTT-605
test("a redial sent a set without a party member tells who left", async () => {
  const t = await openTable("player");
  await t.frames(watch([LERA, BRAM]), viewpoint("a1"));
  t.socket().onclose?.({ code: 1006, reason: "" });
  await settle();
  t.root.querySelector<HTMLElement>(".reconnect")?.click();
  await t.open();
  await t.frames(watch([LERA]), viewpoint("a1"));
  expect(t.toast()).toBe("You no longer play Bram.");
});

// VTT-605, VTT-606
test("Session tells who left, not for a first, equal or renamed set", async () => {
  const sly: Actor = { actorId: "a3", name: "Quill the Sly" };
  const frames = [
    watch([QUILL, LERA, BRAM]),
    watch([QUILL, LERA, BRAM]),
    watch([sly, LERA, BRAM]),
    watch([LERA]),
    watch([LERA, BRAM], true),
    watch([BRAM], true),
    viewpoint("done"),
  ];
  const server = Bun.serve({
    hostname: "127.0.0.1",
    port: 0,
    fetch: (req, srv) =>
      srv.upgrade(req) ? undefined : new Response("", { status: 400 }),
    websocket: {
      open: (ws) => {
        for (const f of frames) ws.send(JSON.stringify(f));
      },
      message: () => {},
    },
  });
  const session = new Session(`ws://127.0.0.1:${server.port}/ws`, "tok");
  const told: Actor[][] = [];
  session.onPartyMembersLeft((left) => {
    told.push(left.map((a) => ({ actorId: a.actorId, name: a.name })));
  });
  try {
    await session.start();
    for (let i = 0; i < 200 && session.viewpoint !== "done"; i++) await settle();
    expect(session.viewpoint).toBe("done");
    expect(told).toEqual([[sly, BRAM], [LERA]]);
    expect(session.controlsNoPartyMember).toBe(true);
  } finally {
    session.close();
    server.stop(true);
  }
});

// VTT-607
test("the perch says the window plays no character only when told to", () => {
  const perchWith = (playsNone?: boolean): string => {
    const root = document.createElement("div");
    const perch = { actors: [LERA], current: "a1", onPerch: () => {} };
    renderSpectator(root, newState(), [], "open", {
      perch: playsNone === undefined ? perch : { ...perch, playsNone },
    });
    return root.querySelector(".perch")?.textContent ?? "";
  };
  expect(perchWith(true)).toContain(PLAYS_NONE);
  expect(perchWith(false)).toContain("Whose eyes");
  expect(perchWith(false)).not.toContain(PLAYS_NONE);
  expect(perchWith()).not.toContain(PLAYS_NONE);
});

// VTT-607, VTT-403
test("the perch tells a player they play none, not a spectator", async () => {
  const player = await openTable("player");
  await player.frames(watch([LERA, BRAM], true), viewpoint("a1"));
  expect(player.perch()).toContain(PLAYS_NONE);
  const spectator = await openTable("spectator");
  await spectator.frames(watch([LERA, BRAM], true), viewpoint("a1"));
  expect(spectator.perch()).toContain("Whose eyes");
  expect(spectator.perch()).not.toContain(PLAYS_NONE);
  await spectator.frames(watch([LERA, BRAM]));
  expect(spectator.perch()).not.toContain(PLAYS_NONE);
  expect(labels(spectator, ".perch .shoulder")).toEqual(["Bram", "Lera"]);
});
