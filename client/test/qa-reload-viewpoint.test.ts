// Independent QA: what a browser page dials and what its window shows, given
// what earlier pages in the same tab, or in other tabs, were told. Pins
// VTT-442 to VTT-444 and their seams with VTT-397 and VTT-407.
import "./support/dom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { boot } from "../src/app";

type Session = NonNullable<ReturnType<typeof boot>>;
type Handler = ((ev: unknown) => void) | null;

class FakeSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static all: FakeSocket[] = [];

  readonly url: string;
  readyState = 0;
  sent: string[] = [];
  onopen: Handler = null;
  onerror: Handler = null;
  onclose: Handler = null;
  onmessage: Handler = null;

  constructor(url: string | URL) {
    this.url = String(url);
    FakeSocket.all.push(this);
  }

  send(data: unknown): void {
    this.sent.push(String(data));
  }

  close(): void {
    if (this.readyState === 3) return;
    this.readyState = 3;
    const h = this.onclose;
    setTimeout(() => h?.({ code: 1000, reason: "", wasClean: true }), 0);
  }

  open(): void {
    this.readyState = 1;
    this.onopen?.({});
  }

  frame(f: unknown): void {
    this.onmessage?.({ data: JSON.stringify(f) });
  }

  drop(): void {
    this.readyState = 3;
    this.onerror?.({});
    this.onclose?.({ code: 1006, reason: "", wasClean: false });
  }
}

type Role = "dm" | "player" | "agent" | "spectator";
interface Me { participantId: string; name: string; role: Role }
interface Watchable { actorId: string; name: string }

const LERA: Watchable = { actorId: "a1", name: "Lera" };
const BO: Watchable = { actorId: "a2", name: "Bo" };
const ODD: Watchable = { actorId: "z+2 & b=c/?#ö", name: "Odd" };
const WATCHABLE: Watchable[] = [LERA, BO];

/** The head of each character log the fake gateway serves. */
const HEAD: Record<string, number> = { a1: 3, a2: 5, [ODD.actorId]: 2 };

let identities: Record<string, Me> = {};
let livePages: Page[] = [];
const originalWebSocket = (globalThis as unknown as { WebSocket: unknown }).WebSocket;
const originalFetch = globalThis.fetch;

function setURL(url: string): void {
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL(url);
}

function bearer(input: unknown, init: unknown): string {
  const fromHeaders = (h: unknown): string | null => {
    if (!h) return null;
    if (typeof (h as Headers).get === "function") return (h as Headers).get("Authorization");
    const rec = h as Record<string, string>;
    return rec["Authorization"] ?? rec["authorization"] ?? null;
  };
  const a = fromHeaders((init as { headers?: unknown } | undefined)?.headers)
    ?? (input instanceof Request ? input.headers.get("Authorization") : null)
    ?? "";
  return a.replace(/^Bearer\s+/i, "");
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

async function stubFetch(input: unknown, init?: unknown): Promise<Response> {
  const raw = typeof input === "string" ? input : input instanceof URL ? input.href : (input as Request).url;
  const url = new URL(raw, "http://localhost/");
  const me = identities[bearer(input, init)];
  switch (url.pathname) {
    case "/api/me":
      return me ? json(me) : json({ error: "unknown token" }, 401);
    case "/api/ruleset":
      return json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    case "/api/adventures":
      return json({ adventures: [] });
    default:
      return json({ error: "not found" }, 404);
  }
}

async function settle(): Promise<void> {
  for (let round = 0; round < 3; round++) {
    for (let i = 0; i < 20; i++) await Promise.resolve();
    await new Promise((r) => setTimeout(r, 0));
  }
}

interface Page { root: HTMLElement; session: Session; firstSocket: number }

async function openPage(url: string): Promise<Page> {
  setURL(url);
  const root = document.createElement("div");
  document.body.appendChild(root);
  const firstSocket = FakeSocket.all.length;
  const session = boot(root);
  if (!session) throw new Error("boot returned null: no token reached the page");
  const page = { root, session, firstSocket };
  livePages.push(page);
  await settle();
  return page;
}

function socketsOf(p: Page): FakeSocket[] {
  return FakeSocket.all.slice(p.firstSocket);
}

function current(p: Page): FakeSocket {
  const s = socketsOf(p).at(-1);
  if (!s) throw new Error("the page dialled nothing");
  return s;
}

async function closePage(p: Page): Promise<void> {
  p.session.close();
  livePages = livePages.filter((q) => q !== p);
  await settle();
  p.root.remove();
}

interface Dial { token: string | null; after: string | null; hasLog: boolean; log: string | null }

function dialOf(s: FakeSocket): Dial {
  const u = new URL(s.url);
  return { token: u.searchParams.get("token"), after: u.searchParams.get("after"), hasLog: u.searchParams.has("log"), log: u.searchParams.get("log") };
}

function entry(seq: number): unknown {
  const e: Record<string, unknown> = { eventId: `e-${seq}`, sequence: String(seq), sessionId: "s" };
  if (seq === 1) e["sessionStarted"] = { name: "Night" };
  else e["narrationAdded"] = { text: `line ${seq}` };
  return { event: e };
}

function roleOf(s: FakeSocket): Role {
  const me = identities[dialOf(s).token ?? ""];
  if (!me) throw new Error("the page dialled under a token the fake gateway does not know");
  return me.role;
}

/** The participant's default: nobody for a spectator, the first party member by id for a player. */
function gatewayDefault(role: Role, watchable: Watchable[]): string {
  if (role === "spectator") return "";
  return watchable.map((w) => w.actorId).sort()[0] ?? "";
}

/**
 * A correct gateway's opening of a player's or a spectator's connection: the
 * set, then the switch, then the log it opened on. A dial naming an actor the
 * participant may watch opens on that actor's log after the dial's cursor;
 * any other dial opens on the default from the first entry.
 */
async function gatewayOpens(s: FakeSocket, watchable: Watchable[] = WATCHABLE): Promise<string> {
  const d = dialOf(s);
  const honoured = d.log !== null && watchable.some((w) => w.actorId === d.log);
  const vp = honoured ? (d.log ?? "") : gatewayDefault(roleOf(s), watchable);
  const head = vp ? (HEAD[vp] ?? 0) : 0;
  const from = honoured ? Number(d.after ?? "0") + 1 : 1;
  s.open();
  s.frame({ catchUpHead: { headSequence: String(head) } });
  s.frame({ presenceSnapshot: { present: [] } });
  s.frame({ watchableChanged: { actors: watchable } });
  s.frame({ viewpointChanged: vp ? { actorId: vp } : {} });
  for (let i = from; i <= head; i++) s.frame(entry(i));
  await settle();
  return vp;
}

/** The set and nothing more: a connection that has not yet been sent its switch. */
async function gatewaySendsOnlyTheSet(s: FakeSocket, watchable: Watchable[] = WATCHABLE): Promise<void> {
  s.open();
  s.frame({ catchUpHead: { headSequence: "0" } });
  s.frame({ presenceSnapshot: { present: [] } });
  s.frame({ watchableChanged: { actors: watchable } });
  await settle();
}

/** A correct gateway's opening of a DM's connection: the world, no switch. */
async function gatewayOpensWorld(s: FakeSocket, entries: number): Promise<void> {
  s.open();
  s.frame({ catchUpHead: { headSequence: String(entries) } });
  s.frame({ presenceSnapshot: { present: [] } });
  for (let i = 1; i <= entries; i++) s.frame(entry(i));
  await settle();
}

function sentSince(s: FakeSocket, from: number): Record<string, unknown>[] {
  return s.sent.slice(from).map((t) => JSON.parse(t) as Record<string, unknown>);
}

function buttonLabelled(p: Page, label: string): HTMLButtonElement {
  const all = [...chips(p), ...Array.from(p.root.querySelectorAll<HTMLButtonElement>(".perch .shoulder"))];
  const b = all.find((x) => x.textContent?.trim() === label);
  if (!b) throw new Error(`no chip or shoulder labelled ${label}; have: ${all.map((x) => x.textContent).join(",")}`);
  return b;
}

/**
 * The participant clicks a chip or a shoulder (null: No shoulder), and a
 * correct gateway applies the set_viewpoint the page sent: the switch, the
 * new log from its first entry, and an accepting result.
 */
async function userSwitches(p: Page, label: string | null): Promise<string> {
  const s = current(p);
  const mark = s.sent.length;
  const btn = label === null ? p.root.querySelector<HTMLButtonElement>(".perch .unperch") : buttonLabelled(p, label);
  if (!btn) throw new Error("no No shoulder button");
  btn.click();
  await settle();
  const ask = sentSince(s, mark).find((f) => "setViewpoint" in f);
  if (!ask) throw new Error("the click sent no setViewpoint");
  const target = String((ask["setViewpoint"] as { actorId?: string }).actorId ?? "");
  s.frame({ viewpointChanged: target ? { actorId: target } : {} });
  for (let i = 1; i <= (target ? (HEAD[target] ?? 0) : 0); i++) s.frame(entry(i));
  s.frame({ result: { requestId: String(ask["requestId"] ?? ""), ok: true } });
  await settle();
  return target;
}

async function dropAndReconnect(p: Page): Promise<FakeSocket> {
  current(p).drop();
  await settle();
  return reconnect(p);
}

async function reconnect(p: Page): Promise<FakeSocket> {
  const before = socketsOf(p).length;
  const btn = p.root.querySelector<HTMLButtonElement>(".reconnect");
  if (!btn) throw new Error("no .reconnect button after the socket closed");
  btn.click();
  await settle();
  const n = socketsOf(p).length - before;
  if (n !== 1) throw new Error(`reconnect made ${n} dials, want 1`);
  return current(p);
}

function snapshotSession(): Record<string, string> {
  const out: Record<string, string> = {};
  for (let i = 0; i < sessionStorage.length; i++) {
    const k = sessionStorage.key(i);
    if (k !== null) out[k] = sessionStorage.getItem(k) ?? "";
  }
  return out;
}

function restoreSession(snap: Record<string, string>): void {
  sessionStorage.clear();
  for (const [k, v] of Object.entries(snap)) sessionStorage.setItem(k, v);
}

function perchedOn(p: Page): string | null {
  return p.root.querySelector(".perch .perched-on")?.textContent?.trim() ?? null;
}

function chips(p: Page): HTMLButtonElement[] {
  const row = p.root.querySelector(".player .row");
  return row ? Array.from(row.querySelectorAll<HTMLButtonElement>("button")) : [];
}

function chipNamed(p: Page, name: string): HTMLButtonElement {
  const c = chips(p).find((b) => b.textContent?.trim() === name);
  if (!c) throw new Error(`no chip named ${name}; chips: ${chips(p).map((b) => b.textContent).join(",")}`);
  return c;
}

function selected(b: HTMLButtonElement): boolean {
  return b.classList.contains("sel");
}

/** A fresh player page in this tab, opened on the default and then switched to Bo. */
async function playerToldBo(url: string): Promise<Page> {
  const p = await openPage(url);
  expect(await gatewayOpens(current(p))).toBe("a1");
  expect(await userSwitches(p, "Bo")).toBe("a2");
  return p;
}

/** A fresh spectator page in this tab, opened on nobody and then perched on Bo. */
async function spectatorToldBo(url: string): Promise<Page> {
  const p = await openPage(url);
  expect(await gatewayOpens(current(p))).toBe("");
  expect(await userSwitches(p, "Bo")).toBe("a2");
  return p;
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setURL("http://localhost/");
  FakeSocket.all = [];
  livePages = [];
  identities = {
    tokP: { participantId: "p-1", name: "Pia", role: "player" },
    tokQ: { participantId: "p-2", name: "Quinn", role: "player" },
    tokS: { participantId: "p-3", name: "Sam", role: "spectator" },
    tokD: { participantId: "p-dm", name: "Dana", role: "dm" },
  };
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeSocket;
  globalThis.fetch = stubFetch as unknown as typeof fetch;
});

afterEach(async () => {
  for (const p of livePages) {
    p.session.close();
    p.root.remove();
  }
  livePages = [];
  await settle();
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = originalWebSocket;
  globalThis.fetch = originalFetch;
  localStorage.clear();
  sessionStorage.clear();
  setURL("http://localhost/");
});

describe("a reload in the same tab, under the same token", () => {
  // VTT-442
  test("qaRV: a reload names the actor the LAST switch named, not the first", async () => {
    const p1 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p1)).hasLog).toBe(false);
    expect(await gatewayOpens(current(p1))).toBe("a1");
    expect(await userSwitches(p1, "Bo")).toBe("a2");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    const d = dialOf(current(p2));
    expect(d.token).toBe("tokP");
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-442
  test("qaRV: a reload dials after 0 though the earlier page held a cursor in that log", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    const held = await dropAndReconnect(p1);
    expect(dialOf(held).log).toBe("a2");
    expect(dialOf(held).after).toBe("5");
    expect(await gatewayOpens(held)).toBe("a2");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    const d = dialOf(current(p2));
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-442
  test("qaRV: a reload whose token comes from storage, not the URL, is under the same token", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/");
    const d = dialOf(current(p2));
    expect(d.token).toBe("tokP");
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-442
  test("qaRV: a reload names no log when the last switch the earlier page was told named nobody", async () => {
    const p1 = await spectatorToldBo("http://localhost/?token=tokS");
    expect(await userSwitches(p1, null)).toBe("");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokS");
    expect(dialOf(current(p2)).hasLog).toBe(false);
  });

  // VTT-442
  test("qaRV: a page told nothing in between leaves the earlier page's switch remembered", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
    current(p2).drop();
    await settle();
    await closePage(p2);

    const p3 = await openPage("http://localhost/?token=tokP");
    const d = dialOf(current(p3));
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-442
  test("qaRV: a later page opened on another actor than it dialled replaces the remembered one", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
    expect(await gatewayOpens(current(p2), [LERA])).toBe("a1");
    await closePage(p2);

    const p3 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p3)).log).toBe("a1");
  });

  // VTT-442
  test("qaRV: the remembered actor outlives a change of role between spectator and player", async () => {
    const p1 = await spectatorToldBo("http://localhost/?token=tokS");
    await closePage(p1);

    identities["tokS"] = { participantId: "p-3", name: "Sam", role: "player" };
    const p2 = await openPage("http://localhost/?token=tokS");
    const d = dialOf(current(p2));
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-442 VTT-407
  test("qaRV: a refused switch is not remembered, the last switch is", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    expect(selected(chipNamed(p1, "Bo"))).toBe(true);
    const s = current(p1);
    const mark = s.sent.length;
    chipNamed(p1, "Lera").click();
    await settle();
    const ask = sentSince(s, mark).find((f) => "setViewpoint" in f);
    expect(ask?.["setViewpoint"]).toEqual({ actorId: "a1" });
    s.frame({ watchableChanged: { actors: [BO] } });
    s.frame({ result: { requestId: String(ask?.["requestId"] ?? ""), ok: false, error: "you may not watch a1" } });
    await settle();
    expect(selected(chipNamed(p1, "Bo"))).toBe(true);
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
  });

  // VTT-442
  test("qaRV: a reload remembers the switch though the earlier page was never closed", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    p1.root.remove();

    const p2 = await openPage("http://localhost/?token=tokP");
    const d = dialOf(current(p2));
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-442
  test("qaRV: a switch the earlier page was told on a later connection is the one remembered", async () => {
    const p1 = await openPage("http://localhost/?token=tokP");
    expect(await gatewayOpens(current(p1))).toBe("a1");
    const again = await dropAndReconnect(p1);
    expect(dialOf(again).log).toBe("a1");
    expect(await gatewayOpens(again, [BO])).toBe("a2");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
  });

  // VTT-442
  test("qaRV: a remembered actor id with URL-special characters reaches the dial unchanged", async () => {
    const p1 = await openPage("http://localhost/?token=tokP");
    expect(await gatewayOpens(current(p1), [ODD, ...WATCHABLE])).toBe("a1");
    expect(await userSwitches(p1, "Odd")).toBe(ODD.actorId);
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    const d = dialOf(current(p2));
    expect(d.token).toBe("tokP");
    expect(d.log).toBe(ODD.actorId);
    expect(d.after).toBe("0");
  });
});

describe("each dial before a switch arrives, and the first redial after one", () => {
  // VTT-442
  test("qaRV: a redial before any switch arrives names the remembered actor again, with after 0", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
    const again = await dropAndReconnect(p2);
    const d = dialOf(again);
    expect(d.log).toBe("a2");
    expect(d.after).toBe("0");
  });

  // VTT-397 VTT-442 VTT-407
  test("qaRV: once a switch arrives, the redial names its actor and the cursor, not the remembered one", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
    expect(await gatewayOpens(current(p2), [LERA])).toBe("a1");
    expect(selected(chipNamed(p2, "Lera"))).toBe(true);
    const again = await dropAndReconnect(p2);
    const d = dialOf(again);
    expect(d.log).toBe("a1");
    expect(d.after).toBe("3");
  });

  // VTT-397 VTT-442
  test("qaRV: once a switch to the remembered actor arrives, the redial dials after its cursor, not 0", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
    expect(await gatewayOpens(current(p2))).toBe("a2");
    const again = await dropAndReconnect(p2);
    const d = dialOf(again);
    expect(d.log).toBe("a2");
    expect(d.after).toBe("5");
  });

  // VTT-397 VTT-442
  test("qaRV: once a switch to nobody arrives, the redial names no log though an actor was remembered", async () => {
    const p1 = await spectatorToldBo("http://localhost/?token=tokS");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokS");
    expect(dialOf(current(p2)).log).toBe("a2");
    expect(await gatewayOpens(current(p2), [LERA])).toBe("");
    const again = await dropAndReconnect(p2);
    expect(dialOf(again).hasLog).toBe(false);
  });
});

describe("another token in the same tab", () => {
  // VTT-443
  test("qaRV: a page under another URL token names no log though this tab's earlier page was told one", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokQ");
    const d = dialOf(current(p2));
    expect(d.token).toBe("tokQ");
    expect(d.hasLog).toBe(false);
  });

  // VTT-443
  test("qaRV: a page under another stored token names no log though this tab's earlier page was told one", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    localStorage.setItem("vtt.token", "tokQ");
    const p2 = await openPage("http://localhost/");
    const d = dialOf(current(p2));
    expect(d.token).toBe("tokQ");
    expect(d.hasLog).toBe(false);
  });

  // VTT-442 VTT-443
  test("qaRV: two tokens in one tab each keep their own remembered actor", async () => {
    const pA = await playerToldBo("http://localhost/?token=tokP");
    await closePage(pA);

    const pB = await openPage("http://localhost/?token=tokQ");
    expect(dialOf(current(pB)).hasLog).toBe(false);
    expect(await gatewayOpens(current(pB))).toBe("a1");
    await closePage(pB);

    const pA2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(pA2)).log).toBe("a2");
    await closePage(pA2);

    const pB2 = await openPage("http://localhost/?token=tokQ");
    expect(dialOf(current(pB2)).log).toBe("a1");
  });

  // VTT-443 VTT-442
  test("qaRV: a DM's page and its redial name no log though this tab's player page was told one", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokD");
    expect(dialOf(current(p2)).hasLog).toBe(false);
    await gatewayOpensWorld(current(p2), 3);
    const again = await dropAndReconnect(p2);
    expect(dialOf(again).hasLog).toBe(false);
    await closePage(p2);

    const p3 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p3)).log).toBe("a2");
  });
});

describe("another tab under the same token", () => {
  // VTT-444
  test("qaRV: a page in another tab under the same token names no log", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(p1);

    sessionStorage.clear();
    const p2 = await openPage("http://localhost/");
    const d = dialOf(current(p2));
    expect(d.token).toBe("tokP");
    expect(d.hasLog).toBe(false);
  });

  // VTT-442 VTT-444
  test("qaRV: another tab's switch does not move what this tab remembers", async () => {
    const t1 = await playerToldBo("http://localhost/?token=tokP");
    await closePage(t1);
    const tab1 = snapshotSession();

    sessionStorage.clear();
    const t2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(t2)).hasLog).toBe(false);
    expect(await gatewayOpens(current(t2))).toBe("a1");
    await closePage(t2);
    const tab2 = snapshotSession();

    restoreSession(tab1);
    const t1b = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(t1b)).log).toBe("a2");
    await closePage(t1b);

    restoreSession(tab2);
    const t2b = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(t2b)).log).toBe("a1");
  });
});

describe("what the window shows as watched before a switch arrives", () => {
  // VTT-407 VTT-442
  test("qaRV: a spectator's perch shows nobody before the switch arrives, the remembered actor included", async () => {
    const p1 = await spectatorToldBo("http://localhost/?token=tokS");
    expect(perchedOn(p1)).toBe("Perched on: Bo");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokS");
    expect(dialOf(current(p2)).log).toBe("a2");
    const s = current(p2);
    await gatewaySendsOnlyTheSet(s);
    expect(perchedOn(p2)).toBe("Perched on: nobody");

    s.drop();
    await settle();
    expect(perchedOn(p2)).toBe("Perched on: nobody");

    const again = await reconnect(p2);
    expect(dialOf(again).log).toBe("a2");
    expect(dialOf(again).after).toBe("0");
    expect(await gatewayOpens(again)).toBe("a2");
    expect(perchedOn(p2)).toBe("Perched on: Bo");
  });

  // VTT-407
  test("qaRV: a dial that fails before any frame shows the remembered actor as watched nowhere", async () => {
    const p1 = await spectatorToldBo("http://localhost/?token=tokS");
    expect(perchedOn(p1)).toBe("Perched on: Bo");
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokS");
    expect(dialOf(current(p2)).log).toBe("a2");
    current(p2).drop();
    await settle();
    const text = p2.root.textContent ?? "";
    expect(text).not.toContain("Perched on: Bo");
    expect(text).not.toContain("Perched on: a2");
    const line = perchedOn(p2);
    expect(line).toBe("Perched on: nobody");
  });

  // VTT-407
  test("qaRV: a player's chips mark nobody before the switch arrives, the remembered actor included", async () => {
    const p1 = await playerToldBo("http://localhost/?token=tokP");
    expect(selected(chipNamed(p1, "Bo"))).toBe(true);
    await closePage(p1);

    const p2 = await openPage("http://localhost/?token=tokP");
    expect(dialOf(current(p2)).log).toBe("a2");
    const s = current(p2);
    await gatewaySendsOnlyTheSet(s);
    expect(chips(p2).map((b) => b.textContent?.trim())).toEqual(["Lera", "Bo"]);
    expect(selected(chipNamed(p2, "Bo"))).toBe(false);
    expect(chips(p2).filter(selected)).toEqual([]);

    s.frame({ viewpointChanged: { actorId: "a2" } });
    await settle();
    expect(selected(chipNamed(p2, "Bo"))).toBe(true);
    expect(selected(chipNamed(p2, "Lera"))).toBe(false);
  });
});
