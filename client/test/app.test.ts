import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { boot } from "../src/app";
// The missing-tile marker's own colours, imported rather than copied: canvas.ts
// exports them "so a test can assert on the exact marker colour rather than
// just 'something non-empty was drawn'", and a copy here would go on agreeing
// with itself after the real one moved.
import { missingTileColors } from "../src/view/canvas";

// happy-dom starts at about:blank and does NOT move location on
// history.replaceState — setURL is its control surface. Discovered the hard
// way: a test that "set" the URL with replaceState was really testing the
// no-token path.
function setURL(url: string) {
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL(url);
}

beforeEach(() => {
  localStorage.clear();
  setURL("http://localhost/");
});

function root(): HTMLElement {
  const el = document.createElement("div");
  el.id = "app";
  document.body.replaceChildren(el);
  return el;
}

test("with no token at all the page says what to do instead of failing silently", () => {
  const r = root();
  const session = boot(r);
  expect(session).toBeNull();
  expect(r.textContent).toContain("invite token");
});

test("a token in the URL is stored, and the address bar is rewritten", () => {
  // A bearer credential left in the URL lands in browser history and in the
  // Referer of every outbound link.
  //
  // The STRIP is asserted as "replaceState was called without the token"
  // rather than by reading location afterwards: happy-dom does not move
  // location on replaceState, so reading it would pass whether or not the
  // code stripped anything. The real address bar is checked by the e2e, in a
  // real browser, where it means something.
  setURL("http://localhost/?token=tok-abc");
  const calls: string[] = [];
  const titles: unknown[] = [];
  const original = history.replaceState.bind(history);
  history.replaceState = ((...args: unknown[]) => {
    titles.push(args[1]);
    calls.push(String(args[2]));
    return original(args[0] as never, args[1] as string, args[2] as string);
  }) as typeof history.replaceState;

  const session = boot(root());
  expect(localStorage.getItem("vtt.token")).toBe("tok-abc");
  expect(calls).toHaveLength(1);
  expect(calls[0]).not.toContain("token");
  // AND THE ENTRY IS REPLACED WITHOUT CLAIMING A TITLE. The middle argument is
  // the legacy `title`, which no browser has ever done anything with and
  // happy-dom ignores too (measured: document.title does not move), so the spy
  // is the only place its value is visible at all — and the empty string is
  // the only honest thing to pass a parameter nothing reads. Anything else
  // would be this app announcing a document title it does not set, in a call
  // whose entire purpose is to change the URL and nothing else.
  expect(titles).toEqual([""]);
  session?.close();
  history.replaceState = original;
});

test("a stored token is reused on a later visit, and is the one that dials", () => {
  // Asserting the socket URL, not just that a Session came back: a boot that
  // connected with the wrong token — or an empty one — would still return a
  // non-null Session.
  localStorage.setItem("vtt.token", "tok-stored");
  useFakeSocket();
  const session = boot(root());
  expect(session).not.toBeNull();
  expect(FakeSocket.instances[0]!.url).toContain("token=tok-stored");
  session?.close();
});

test("a freshly pasted invite REPLACES a stored token", () => {
  // Re-invitation has to work. Previously app.ts read
  // `auth.get() ?? <url token>`, so a stored token won outright and the
  // pasted one was discarded — never stored, never dialled — leaving a
  // re-invited player connected as their OLD identity with no way out but
  // clearing site data.
  //
  // The trade Patrik took (2026-07-31): a link now overrides a stored
  // session, so on a SHARED machine whoever pastes a link last is who you
  // are. That is the same authority the link already carries — it is a
  // bearer credential — and it is recoverable, which the previous behaviour
  // was not.
  localStorage.setItem("vtt.token", "tok-old");
  setURL("http://localhost/?token=tok-new");
  useFakeSocket();

  const session = boot(root());
  expect(localStorage.getItem("vtt.token")).toBe("tok-new");
  expect(FakeSocket.instances[0]!.url).toContain("token=tok-new");
  expect(FakeSocket.instances[0]!.url).not.toContain("tok-old");
  session?.close();
});

// ---------------------------------------------------------------------------
// The wired-up shell.
//
// Everything below drives boot() with a fake socket and a fake metadata
// server. The point is the COMPOSITION — which callback reaches which
// renderer — because that is the only thing app.ts owns; the pieces it wires
// are unit-tested in their own files.
//
// Both fakes are installed per-test and torn down in afterEach: bun runs
// every test file in one process, so a leaked global fetch would reach
// metadata.test.ts and wire.test.ts, which drive a REAL server.

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
  /**
   * Whether close() DISPATCHES the close event or merely arms it.
   *
   * No real WebSocket fires `close` from inside close(): the closing handshake
   * is a server round trip and the event is delivered as a task afterwards,
   * whenever afterwards turns out to be. Firing it inline is a simplification
   * every test here is happy with except one — and that one is about precisely
   * that gap, so it arms the close and lands it after the redial that abandoned
   * the socket has already sent its next command.
   *
   * IT IS THE ORDERING THIS BUYS, not a faithful socket: a real one that has
   * already dispatched a close reports no second one, and it sits in CLOSING
   * rather than CLOSED while the event is in flight. The test that uses this
   * says which half of it is the realistic half and which is staging.
   */
  lateClose = false;
  private armedClose = false;

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
    if (this.lateClose) {
      this.armedClose = true;
      return;
    }
    this.onclose?.();
  }
  /**
   * Deliver a close that lateClose held back.
   *
   * THROWS when there is none, rather than doing nothing: a test whose whole
   * subject is a late close would otherwise pass while never delivering one,
   * which is a green proving the opposite of what it claims.
   */
  fireClose() {
    if (!this.armedClose) throw new Error("fireClose: no close was held back");
    this.armedClose = false;
    this.onclose?.();
  }
}

const nativeFetch = globalThis.fetch;
const nativeWS = globalThis.WebSocket;

/** Serve the metadata routes from a table; anything unlisted 404s. */
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

/** Let queued promise callbacks run. The metadata chain is three .then hops. */
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

afterEach(() => {
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
});

test("an unfoldable log replaces the board with the reason, not a plausible wrong board", async () => {
  // Holding the last good state and rendering a board anyway would invite a
  // player to act on a position that never existed. session.ts documents
  // that choice; this pins that app.ts actually surfaces it.
  localStorage.setItem("vtt.token", "tok-dm");
  stubMetadata({ "/api/me": { participantId: "p", name: "P", role: "spectator" } });
  useFakeSocket();

  const r = root();
  const session = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { tokenMoved: { tokenId: "ghost", sceneId: "s", to: { x: 1, y: 1 } } }));
  await settle();

  expect(r.textContent).toContain("Cannot derive state from the log");
  expect(r.textContent).toContain("ghost");
  expect(r.querySelector(".fatal")).not.toBeNull();
  session?.close();
});

test("a DM identity gets the console, and its actions reach the socket", async () => {
  localStorage.setItem("vtt.token", "tok-dm");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [{ id: "a-1", name: "Cave" }] },
  });
  useFakeSocket();

  const r = root();
  const session = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night One" } }));
  await settle();

  // The console only renders for dm/agent, and "End session" only appears
  // while a session is open — so finding it proves both the role branch and
  // that the folded state reached the renderer.
  const buttons = Array.from(r.querySelectorAll("button"));
  const end = buttons.find((b) => b.textContent === "End session");
  expect(end).toBeDefined();

  end!.click();
  await settle();

  expect(sock.sent).toHaveLength(1);
  const cmd = JSON.parse(sock.sent[0]!);
  expect(cmd).toHaveProperty("endSession");
  expect(cmd.requestId).toBeTruthy();
  session?.close();
});

test("a refused command is reported verbatim rather than silently doing nothing", async () => {
  localStorage.setItem("vtt.token", "tok-dm");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();

  const r = root();
  const session = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night One" } }));
  await settle();

  Array.from(r.querySelectorAll("button")).find((b) => b.textContent === "End session")!.click();
  await settle();

  const reqID = JSON.parse(sock.sent[0]!).requestId as string;
  sock.deliver({ result: { requestId: reqID, ok: false, error: "not authorized" } });
  await settle();

  expect(r.textContent).toContain("refused: not authorized");
  session?.close();
});

test("metadata being unavailable degrades to a spectator page, not a blank one", async () => {
  // /api/me 404s here. The board and story must still render: a blank page
  // is a worse failure than a missing panel.
  localStorage.setItem("vtt.token", "tok-x");
  stubMetadata({});
  useFakeSocket();

  const r = root();
  const session = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night One" } }));
  await settle();

  expect(r.querySelector(".fatal")).toBeNull();
  expect(Array.from(r.querySelectorAll("button")).some((b) => b.textContent === "End session")).toBe(false);
  expect(r.textContent).not.toBe("");
  session?.close();
});

test("connection status is shown as it changes", async () => {
  localStorage.setItem("vtt.token", "tok-x");
  stubMetadata({});
  useFakeSocket();

  const r = root();
  const session = boot(r);
  expect(r.textContent).toContain("connecting");

  const sock = FakeSocket.instances[0]!;
  sock.open();
  await settle();
  expect(r.textContent).toContain("connected");

  sock.close();
  await settle();
  expect(r.textContent).toContain("closed");
  session?.close();
});

// --- the composition root's own decisions -----------------------------------
//
// app.ts wires everything together, and almost none of its wiring was pinned:
// which panels a role gets, which scheme the socket dials, and what the page
// shows before any of the async work lands. Each of those is a decision the
// user sees immediately, and each was invisible to the suite.

test("the socket scheme follows the page's, so an https page never dials ws://", async () => {
  // Mixed content: a wss page dialling ws:// is blocked outright by the
  // browser, which presents as a board that never connects with nothing in
  // the UI to explain it.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({});
  useFakeSocket();
  setURL("https://table.example/");
  const s = boot(root());
  expect(FakeSocket.instances[0]!.url.startsWith("wss://table.example/ws")).toBe(true);
  s?.close();
});

test("a plain http page dials ws://, not wss://", async () => {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({});
  useFakeSocket();
  setURL("http://localhost:8080/");
  const s = boot(root());
  expect(FakeSocket.instances[0]!.url.startsWith("ws://localhost:8080/ws")).toBe(true);
  s?.close();
});

test("before anything connects the page says connecting, with no toast", async () => {
  // The very first paint happens before the socket opens and before metadata
  // lands. It has to show a truthful status rather than a blank or a stale
  // one, and it must not invent a notification.
  //
  // What this does NOT do is pin the `status`/`toast` INITIALISERS, despite
  // reading as though it might: the "connecting" observed here is written by
  // Wire.connect() itself (wire.ts:67) AFTER first paint, so a mutated
  // initialiser is repainted over before this assertion runs. Those two are
  // pinned by the between-the-paints test and the empty-toast test further
  // down, which is where to look if a mutant on either survives.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({});
  useFakeSocket();
  const r = root();
  const s = boot(r);
  expect(r.textContent).toContain("connecting");
  expect(r.textContent).not.toContain("refused");
  s?.close();
});

test("a spectator gets neither the player panel nor the DM console", async () => {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "S", role: "spectator" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  FakeSocket.instances[0]!.open();
  await settle();
  expect(r.querySelector(".player")).toBeNull();
  expect(r.textContent).not.toContain("DM console");
  s?.close();
});

test("a player gets the panel but not the console", async () => {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Lera", role: "player" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  FakeSocket.instances[0]!.open();
  await settle();
  expect(r.querySelector(".player")).not.toBeNull();
  expect(r.textContent).not.toContain("DM console");
  s?.close();
});

test("an agent is treated as a DM: panel AND console", async () => {
  // The agent role is the LLM seat. It acts, so it needs the panel, and it
  // runs the table, so it needs the console — dropping either from the role
  // test would quietly strip the agent of half its interface.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Agent", role: "agent" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  FakeSocket.instances[0]!.open();
  await settle();
  expect(r.querySelector(".player")).not.toBeNull();
  expect(r.textContent).toContain("DM console");
  s?.close();
});

test("the adventures list reaches the DM console", async () => {
  // Third hop of the metadata chain, and the only one whose result is not
  // needed to render anything else — so an empty body there is invisible
  // unless the list itself is asserted.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [{ id: "keep", name: "The Cold Keep" }] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  FakeSocket.instances[0]!.open();
  await settle();
  expect(r.textContent).toContain("The Cold Keep");
  s?.close();
});

test("delivered events reach both the feed and the DM console", async () => {
  // The spectator view is handed `[...session.events]` and the console is
  // handed `session.state`; both come off the same Session, and either one
  // arriving empty renders a plausible, silent, wrong page — a board with no
  // story beside it, or a console that thinks the table has not started.
  // (It used to be two `[...session.events]` spreads; the console stopped
  // taking a log when retraction left, so the two halves now differ.)
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  await settle();
  sock.deliver(envelope(1, { sessionStarted: { name: "S" } }));
  sock.deliver(envelope(2, { narrationAdded: { text: "A hush falls.", as: "", anchorFromSeq: "0", anchorToSeq: "0" } }));
  await settle();
  // The feed's half: narration text only the spectator view renders.
  expect(r.textContent).toContain("A hush falls.");
  // The console's half, which needs its OWN assertion, because asserting story
  // text alone reached only the feed. The console is handed no log at all now
  // — retraction was the only thing that read one — so what is checked is the
  // state it IS handed: sequence 1 opened a session, and the open-session arm
  // offers "End session" where a console drawing from a stale or empty state
  // would still be offering "Start session".
  expect(byText(r, "End session")).toBeDefined();
  expect(byText(r, "Start session")).toBeUndefined();
  s?.close();
});

test("a DM gets the player panel as well as the console", async () => {
  // canAct lists player, dm and agent separately. Each arm needs its own
  // case or one role silently loses the ability to act while still looking
  // fully wired.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  FakeSocket.instances[0]!.open();
  await settle();
  expect(r.querySelector(".player")).not.toBeNull();
  expect(r.textContent).toContain("DM console");
  s?.close();
});

test("a fresh page carries no toast element at all", async () => {
  // toast starts empty and is passed through as `toast || undefined`, so an
  // empty one must not render a container. A stray toast on first paint is a
  // notification the user never triggered.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({});
  useFakeSocket();
  const r = root();
  const s = boot(r);
  expect(r.querySelector(".toast")).toBeNull();
  s?.close();
});

test("an adventures fetch that never happens leaves the console's list empty", async () => {
  // /api/ruleset is deliberately absent, so the metadata chain rejects at its
  // SECOND hop and the third never runs. That is the only way `adventures`
  // keeps its initial value: fetchAdventures answers [] on a 404 rather than
  // throwing, so a failing adventures route would overwrite the initializer
  // with the same empty list and prove nothing.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  FakeSocket.instances[0]!.open();
  await settle();
  expect(r.textContent).toContain("DM console");
  // The SECTION must be absent, not merely free of a sentinel string: a
  // phantom entry renders as "Load undefined", which contains nothing
  // recognisable to grep for but still offers the DM a button for content
  // the server does not have.
  expect(r.textContent).not.toContain("Adventures");
  s?.close();
});

test("a configured map's pack is fetched and its images requested with the Bearer token", async () => {
  // Task 10's seam: nothing before this task ever fetched a tile image (see
  // pack-assets.ts's own header comment). This proves the WIRING end to end
  // from GET /api/maps through to the pack manifest and its image files —
  // the one thing this suite CAN prove, since happy-dom implements neither
  // canvas nor createImageBitmap (canvas.ts's own header comment), so
  // whether the decoded result actually reaches paint() is verified by
  // reading spectator.ts, not by a DOM assertion here.
  localStorage.setItem("vtt.token", "tok");
  const asked: { path: string; auth: string | null }[] = [];
  const originalCIB = (globalThis as unknown as { createImageBitmap?: unknown }).createImageBitmap;
  (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap =
    async (_blob: Blob) => ({}) as unknown as ImageBitmap;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input)).pathname;
    asked.push({ path, auth: (init?.headers as Record<string, string> | undefined)?.["Authorization"] ?? null });
    if (path === "/api/me") {
      return Response.json({ participantId: "p", name: "DM", role: "dm" });
    }
    if (path === "/api/ruleset") {
      return Response.json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    }
    if (path === "/api/adventures") {
      return Response.json({ adventures: [] });
    }
    if (path === "/api/maps") {
      return Response.json({
        maps: [{
          id: "cellar", name: "The Sunken Cellar", gridWidth: 10, gridHeight: 9,
          pack: { id: "cellar-basics", name: "Cellar Basics", cellPx: 64 },
        }],
      });
    }
    if (path === "/api/packs/cellar-basics/pack.json") {
      return Response.json({
        id: "cellar-basics", name: "Cellar Basics", cell_px: 64,
        tiles: [{ name: "earth-1", kind: "floor", material: "earth", file: "earth_1.png" }],
        objects: [],
      });
    }
    if (path === "/api/packs/cellar-basics/earth_1.png") {
      return new Response("fake-png-bytes", { status: 200 });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;
  useFakeSocket();

  try {
    const r = root();
    const s = boot(r);
    FakeSocket.instances[0]!.open();
    await settle();

    expect(asked.map((a) => a.path)).toContain("/api/maps");
    expect(asked.map((a) => a.path)).toContain("/api/packs/cellar-basics/pack.json");
    expect(asked.map((a) => a.path)).toContain("/api/packs/cellar-basics/earth_1.png");
    // The token, as a Bearer header on every one of them — never a query
    // parameter (metadata.ts's own header comment: a token in a query string
    // leaks into access logs, Referer headers and browser history).
    //
    // Scoped to /api/ paths: boot() also fires an unconditional, UNauthenticated
    // fetch of the standard baseline pack at "/std-pack/..." (the next test
    // below), which is not part of what this test is proving and must not
    // make this assertion flaky against that call's own (correct) lack of a
    // token.
    for (const a of asked.filter((c) => c.path.startsWith("/api/"))) expect(a.auth).toBe("Bearer tok");
    s?.close();
  } finally {
    (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap = originalCIB;
  }
});

test("the standard baseline pack is fetched unconditionally, with NO Authorization header", async () => {
  // Review finding C2 (2026-08-16): both shipped adventures carry zero art
  // overrides, so every square needs the std:<kind>/<material> baseline this
  // pack supplies — fetched from the client's own bundle ("/std-pack/...",
  // NOT a pack id's /api/packs/{pack}/... route), unauthenticated by
  // construction (pack-assets.ts's own header comment on
  // loadStandardPackImages). No maps are configured at all here (GET
  // /api/maps answers empty) and no token exists in the request that
  // matters — this must fire regardless, not only when a map's own pack is
  // configured (unlike the "configured map's pack" test above).
  localStorage.setItem("vtt.token", "tok");
  const asked: { path: string; auth: string | null }[] = [];
  const originalCIB = (globalThis as unknown as { createImageBitmap?: unknown }).createImageBitmap;
  (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap =
    async (_blob: Blob) => ({}) as unknown as ImageBitmap;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input)).pathname;
    asked.push({ path, auth: (init?.headers as Record<string, string> | undefined)?.["Authorization"] ?? null });
    if (path === "/api/me") {
      return Response.json({ participantId: "p", name: "DM", role: "dm" });
    }
    if (path === "/api/ruleset") {
      return Response.json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    }
    if (path === "/api/adventures") {
      return Response.json({ adventures: [] });
    }
    if (path === "/api/maps") {
      return Response.json({ maps: [] });
    }
    if (path === "/std-pack/pack.json") {
      return Response.json({
        id: "std", name: "Standard Vocabulary", cell_px: 64,
        tiles: [{ name: "earth", kind: "floor", material: "earth", file: "std_earth_floor.png" }],
        objects: [],
      });
    }
    if (path === "/std-pack/std_earth_floor.png") {
      return new Response("fake-png-bytes", { status: 200 });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;
  useFakeSocket();

  try {
    const r = root();
    const s = boot(r);
    FakeSocket.instances[0]!.open();
    await settle();

    expect(asked.map((a) => a.path)).toContain("/std-pack/pack.json");
    expect(asked.map((a) => a.path)).toContain("/std-pack/std_earth_floor.png");
    const stdCalls = asked.filter((a) => a.path.startsWith("/std-pack/"));
    expect(stdCalls.length).toBeGreaterThan(0);
    for (const a of stdCalls) expect(a.auth).toBeNull();
    s?.close();
  } finally {
    (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap = originalCIB;
  }
});

/**
 * A 2D context that records what was actually DRAWN: every drawImage by the
 * tag its decoded stand-in carries, and every fill by its colour — which is
 * how a missing-tile marker (canvas.ts's magenta checker, spec §7) tells
 * itself apart from a picture.
 *
 * fillStyle goes through a getter/setter over a separate holder rather than
 * being a plain field the recorder reads off `this`: an object literal whose
 * own method reads the object it is initialising is an implicit-any under
 * this repo's tsconfig, and `bun test` would run it happily while
 * client:typecheck refused it.
 */
function recordingCtx(calls: string[]): CanvasRenderingContext2D {
  const pen = { fillStyle: "" };
  return {
    get fillStyle() {
      return pen.fillStyle;
    },
    set fillStyle(v: string) {
      pen.fillStyle = v;
    },
    strokeStyle: "",
    lineWidth: 0,
    save() {},
    restore() {},
    translate() {},
    rotate() {},
    beginPath() {},
    moveTo() {},
    lineTo() {},
    stroke() {},
    drawImage(image: unknown) {
      calls.push(`drawImage:${(image as { tag: string }).tag}`);
    },
    fillRect() {
      calls.push(`fillRect:${pen.fillStyle}`);
    },
  } as unknown as CanvasRenderingContext2D;
}

test("both packs' art reaches the canvas: an overridden square AND a plain one", async () => {
  // WHAT THE TWO TESTS ABOVE DO NOT PROVE. They assert that the pack files are
  // REQUESTED — the manifest, the image, with or without a Bearer header —
  // and stop there. Nothing observed the decoded pictures arriving in the
  // ImageMap app.ts hands to renderSpectator, so throwing either merge away
  // (or replacing it with a fresh, empty object) left both of them green while
  // every square of both shipped adventures drew the magenta missing-tile
  // marker. That is review finding C2's exact failure mode, which is a bad one
  // to be blind to twice.
  //
  // HOW IT IS OBSERVED. happy-dom's canvas.getContext("2d") always answers
  // null, so renderGrid's `if (ctx)` never opens and paint() is unreachable
  // (canvas.ts's own header comment). spectator.ts carries a getContext
  // override for exactly this, but it is a TEST-ONLY SEAM ON ViewExtras and
  // app.ts deliberately never sets it — so reaching app.ts's own default path
  // means making the real canvas.getContext answer. Patched on
  // HTMLCanvasElement.prototype and restored in a finally: bun runs every test
  // file in one process, so a leaked patch would reach the view tests, which
  // drive the same code through their own seam.
  //
  // BOTH LEVELS OF SPEC §4.2's resolution in one scene, because they arrive
  // from different fetches on different schedules: the overridden square wants
  // the map's own pack (Bearer, /api/packs/...), the plain one wants the
  // standard vocabulary (no token, /std-pack/...), and either merge losing its
  // half is a board that draws half a room.
  localStorage.setItem("vtt.token", "tok");
  const originalCIB = (globalThis as unknown as { createImageBitmap?: unknown }).createImageBitmap;
  // The decoded stand-in carries the FILE'S OWN BYTES as its tag, so a
  // drawImage can be traced back to the exact file it came from — "something
  // was drawn" would pass with the two packs' images swapped.
  (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap =
    async (blob: Blob) => ({ tag: await blob.text() }) as unknown as ImageBitmap;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") return Response.json({ participantId: "p", name: "DM", role: "dm" });
    if (path === "/api/ruleset") {
      return Response.json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    }
    if (path === "/api/adventures") return Response.json({ adventures: [] });
    if (path === "/api/maps") {
      return Response.json({
        maps: [{
          id: "cellar", name: "The Sunken Cellar", gridWidth: 2, gridHeight: 1,
          pack: { id: "cellar-basics", name: "Cellar Basics", cellPx: 64 },
        }],
      });
    }
    if (path === "/api/packs/cellar-basics/pack.json") {
      return Response.json({
        id: "cellar-basics", name: "Cellar Basics", cell_px: 64,
        tiles: [{ name: "masonry-1", kind: "floor", material: "stone", file: "masonry_1.png" }],
        objects: [],
      });
    }
    if (path === "/api/packs/cellar-basics/masonry_1.png") return new Response("pack-art");
    if (path === "/std-pack/pack.json") {
      return Response.json({
        id: "std", name: "Standard Vocabulary", cell_px: 64,
        tiles: [{ name: "earth", kind: "floor", material: "earth", file: "std_earth_floor.png" }],
        objects: [],
      });
    }
    if (path === "/std-pack/std_earth_floor.png") return new Response("std-art");
    return new Response("", { status: 404 });
  }) as typeof fetch;
  useFakeSocket();

  const drawn: string[] = [];
  const canvasProto = HTMLCanvasElement.prototype as unknown as { getContext: unknown };
  const originalGetContext = canvasProto.getContext;
  canvasProto.getContext = () => recordingCtx(drawn);
  try {
    const r = root();
    const s = boot(r);
    const sock = FakeSocket.instances[0]!;
    sock.open();
    sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
    sock.deliver(
      envelope(2, {
        sceneCreated: {
          sceneId: "s1", name: "Hall", gridWidth: 2, gridHeight: 1,
          tiles: {
            // Art empty: the standard vocabulary answers, by kind/material.
            "0,0": { kind: "floor", material: "earth", art: "" },
            // Art set: the map's own pack answers, by name (mapdef.Resolve
            // wrote this in at compile time — spec §4.2's two levels).
            "1,0": { kind: "floor", material: "stone", art: "masonry-1" },
          },
        },
      }),
    );
    await settle();
    await settle();

    // A FRESH PAINT, with the earlier ones discarded. Both loads repaint as
    // they land, so the accumulated record necessarily contains the honest
    // markers drawn before any art existed — and a marker assertion over that
    // could never pass. One more frame, and what this paints is the steady
    // state a table actually looks at.
    drawn.length = 0;
    sock.deliver(envelope(3, { actorAdded: { actor: { actorId: "a1", name: "Lera" } } }));
    await settle();

    expect(drawn).toContain("drawImage:std-art");
    expect(drawn).toContain("drawImage:pack-art");
    // AND NOTHING WAS MARKED MISSING. Without this, dropping one merge would
    // still leave the other's assertion above passing on its own half.
    expect(drawn.filter((c) => c.startsWith(`fillRect:${missingTileColors[0]}`))).toHaveLength(0);
    s?.close();
  } finally {
    canvasProto.getContext = originalGetContext;
    (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap = originalCIB;
  }
});

test("a map with no pack is skipped, and a pack two maps share is fetched once", async () => {
  // THE TWO WAYS THE LOOP OVER /api/maps CAN GO WRONG, and they are one line
  // apart. Configuring several maps is ordinary — a campaign is a handful of
  // them, and both shipped adventures ship more than one — and pack-assets.ts
  // loads EVERY configured map's pack because the wire gives no way to
  // correlate a live scene back to the map it came from.
  //
  //   - A map with NO pack at all is legal (MapMeta.pack is optional, and a
  //     map authored with no art overrides needs none). Reading its pack id
  //     anyway throws, and the throw lands in the metadata chain's own
  //     trailing .catch — so the symptom is not a stack trace, it is every
  //     LATER map's art silently never loading.
  //   - Two maps sharing a pack is the normal case for a multi-level dungeon.
  //     Without the loaded-set guard each one re-fetches the manifest and
  //     every image in it, which on a real pack is dozens of files per extra
  //     map and paints nothing new.
  //
  // The pack-less map is listed FIRST on purpose: it is the position where
  // reading its id takes down the maps behind it too.
  localStorage.setItem("vtt.token", "tok");
  const asked: string[] = [];
  const originalCIB = (globalThis as unknown as { createImageBitmap?: unknown }).createImageBitmap;
  (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap =
    async (_blob: Blob) => ({}) as unknown as ImageBitmap;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    asked.push(path);
    if (path === "/api/me") return Response.json({ participantId: "p", name: "DM", role: "dm" });
    if (path === "/api/ruleset") {
      return Response.json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    }
    if (path === "/api/adventures") return Response.json({ adventures: [] });
    if (path === "/api/maps") {
      return Response.json({
        maps: [
          { id: "attic", name: "The Attic", gridWidth: 4, gridHeight: 4 },
          {
            id: "cellar", name: "The Sunken Cellar", gridWidth: 10, gridHeight: 9,
            pack: { id: "cellar-basics", name: "Cellar Basics", cellPx: 64 },
          },
          {
            id: "cellar-north", name: "The North Cellar", gridWidth: 10, gridHeight: 9,
            pack: { id: "cellar-basics", name: "Cellar Basics", cellPx: 64 },
          },
        ],
      });
    }
    if (path === "/api/packs/cellar-basics/pack.json") {
      return Response.json({
        id: "cellar-basics", name: "Cellar Basics", cell_px: 64,
        tiles: [{ name: "earth-1", kind: "floor", material: "earth", file: "earth_1.png" }],
        objects: [],
      });
    }
    if (path === "/api/packs/cellar-basics/earth_1.png") return new Response("fake-png-bytes");
    return new Response("", { status: 404 });
  }) as typeof fetch;
  useFakeSocket();

  try {
    const r = root();
    const s = boot(r);
    FakeSocket.instances[0]!.open();
    await settle();

    // The shared pack loaded — once — and the pack-less map cost nothing.
    expect(asked.filter((p) => p === "/api/packs/cellar-basics/pack.json")).toHaveLength(1);
    expect(asked.filter((p) => p === "/api/packs/cellar-basics/earth_1.png")).toHaveLength(1);
    // And nothing was asked for on behalf of the map that has no pack: an id
    // read off `undefined` would 404 as the literal string, which is the shape
    // this failure takes when the guard is present but reversed.
    expect(asked.filter((p) => p.includes("undefined"))).toHaveLength(0);
    expect(asked.filter((p) => p.startsWith("/api/packs/"))).toHaveLength(2);
    s?.close();
  } finally {
    (globalThis as unknown as { createImageBitmap: unknown }).createImageBitmap = originalCIB;
  }
});

test("before /api/maps resolves, the console renders no Maps group at all", async () => {
  // Kills the ArrayDeclaration mutant on app.ts's `let maps: MapMeta[] =
  // [];`: a leaked `["Stryker was here"]` sentinel would render a Maps
  // group with one bogus entry the moment the DM console first paints --
  // well before the fetch that is SUPPOSED to populate it ever resolves.
  //
  // Task 2's own commit message named "zero maps renders nothing at all" as
  // the one behaviour worth testing; the actual test is dm-view.test.ts's
  // "no maps configured means no Maps group at all, not an empty one",
  // which pins that at renderDMConsole's boundary, with `maps: []` handed
  // in directly by the TEST -- it says nothing about what app.ts's own
  // composition starts that parameter as. This is the other half:
  // /api/maps held open so the console paints (me, ruleset and adventures
  // all resolved) while `maps` is still whatever its declaration
  // initialised it to.
  localStorage.setItem("vtt.token", "tok-dm");
  let releaseMaps: ((v: Response) => void) | null = null;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") return Response.json({ participantId: "p-dm", name: "DM", role: "dm" });
    if (path === "/api/ruleset") {
      return Response.json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    }
    if (path === "/api/adventures") return Response.json({ adventures: [] });
    if (path === "/api/maps") {
      // Held open deliberately: the console must already be rendering by
      // the time this resolves, so whatever `maps` starts as is what it
      // renders here, not what the fetch would eventually supply.
      return new Promise<Response>((r) => { releaseMaps = r; });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;
  useFakeSocket();

  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  await settle();

  // The fetch is genuinely still in flight, not merely unawaited -- the
  // guard below would be meaningless if the promise had already settled.
  expect(releaseMaps).not.toBeNull();
  expect(r.querySelector('[data-action="load-map"]')).toBeNull();
  expect(r.textContent).not.toContain("Maps");

  // Let it resolve and close cleanly, rather than leaving a dangling
  // fetch/timer for a later test to trip over.
  releaseMaps!(Response.json({ maps: [] }));
  await settle();
  s?.close();
});

/** A minimal live table: a scene, one actor the caller controls, and its token. */
// TWO EVENTS PER CHARACTER, because the fold refuses an actorAdded that names
// a controller: creation makes a character, a grant hands it over (visibility
// spec §5.1, 2026-08-24). This is also the shape the server's own projection
// now sends a seat — an introduction with the grants behind it.
function seedTable(sock: FakeSocket, actorId = "a1", participantId = "p") {
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  sock.deliver(envelope(2, { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8 } }));
  sock.deliver(envelope(3, { actorAdded: { actor: { actorId, name: "Lera" } } }));
  sock.deliver(
    envelope(4, {
      actorControlGranted: { actorId, participantId, kind: "ACTOR_KIND_PARTY_MEMBER" },
    }),
  );
  sock.deliver(envelope(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId, position: { x: 1, y: 1 } } }));
}

test("the ruleset's abilities reach the player panel, and nothing else does", async () => {
  // With an actor on the board the panel actually renders its ability list,
  // which is the only place the `abilities` variable is observable. Without a
  // controlled actor the panel short-circuits and the list is never read —
  // which is why an earlier version of this test proved nothing.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Lera", role: "player" },
    "/api/ruleset": {
      id: "r", name: "R", conditions: [], resources: [],
      abilities: [{ id: "poke", name: "Poke", range: 1, maxTargets: 1, usage: { kind: "atWill" } }],
    },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock);
  await settle();

  const panel = r.querySelector(".player")!;
  expect(panel.textContent).toContain("Abilities");
  expect(Array.from(panel.querySelectorAll("button")).some((b) => b.textContent === "Poke")).toBe(true);
  s?.close();
});

test("a ruleset that never loads leaves the panel with no ability list at all", async () => {
  // The other side of the same variable: the metadata chain's .catch is
  // silent by design, so whatever `abilities` started as is what the panel
  // shows forever. It must start EMPTY.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({ "/api/me": { participantId: "p", name: "Lera", role: "player" } });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock);
  await settle();

  expect(r.querySelector(".player")!.textContent).not.toContain("Abilities");
  s?.close();
});

test("clicking the board moves the player's token", async () => {
  // onCell is wired only for roles that may act, and it turns a pixel click
  // into a MoveToken. Nothing was driving it, so both the handler body and
  // its `if (cmd)` guard were free to disappear.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Lera", role: "player" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock);
  await settle();

  const board = r.querySelector(".grid") as HTMLElement;
  expect(board).not.toBeNull();
  board.dispatchEvent(new MouseEvent("click", { clientX: 5, clientY: 5, bubbles: true }));
  await settle();

  expect(sock.sent.length).toBeGreaterThan(0);
  expect(JSON.parse(sock.sent[0]!)).toHaveProperty("moveToken");
  s?.close();
});

test("a spectator's clicks on the board send nothing", async () => {
  // The same wiring, absent. onCell is undefined for a spectator, so the
  // board must not even offer the affordance.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Watcher", role: "spectator" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock);
  await settle();

  const board = r.querySelector(".grid") as HTMLElement;
  board?.dispatchEvent(new MouseEvent("click", { clientX: 5, clientY: 5, bubbles: true }));
  await settle();
  expect(sock.sent).toHaveLength(0);
  s?.close();
});

// --- arming doors on the board (Task 4) -------------------------------------
//
// A live table with a 4x4 scene (fitCamera(4,4,44,640,480) -> scale
// 2.727273, offsetX 80, offsetY 0 -- boardCamera's own arithmetic, not
// hand-picked) carrying a door tile at (0,0) and the player's own token at
// (1,1): Chebyshev distance 1, adjacent, so mayWorkDoor passes and the panel
// offers the toggle at all. Pixel (90,10) resolves to world ((90-80)/2.727,
// 10/2.727) = (3.67, 3.67) -> cell (0,0), the door. Pixel (210,10) resolves
// to world ((210-80)/2.727, 3.67) = (47.67, 3.67) -> cell (1,0), a plain
// square with no Tiles entry at all.
async function playerTableWithDoor() {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Lera", role: "player" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  sock.deliver(
    envelope(2, {
      sceneCreated: {
        sceneId: "s1", name: "Hall", gridWidth: 4, gridHeight: 4,
        tiles: { "0,0": { kind: "door", material: "wood", art: "" } },
      },
    }),
  );
  sock.deliver(envelope(3, { actorAdded: { actor: { actorId: "a1", name: "Lera" } } }));
  sock.deliver(
    envelope(4, {
      actorControlGranted: { actorId: "a1", participantId: "p", kind: "ACTOR_KIND_PARTY_MEMBER" },
    }),
  );
  sock.deliver(envelope(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } }));
  await settle();
  return { r, s, sock };
}

const arm = (r: HTMLElement) => r.querySelector<HTMLButtonElement>('[data-action="arm-doors"]')!.click();
const clickBoard = (r: HTMLElement, clientX: number, clientY: number) =>
  (r.querySelector(".grid") as HTMLElement).dispatchEvent(
    new MouseEvent("click", { clientX, clientY, bubbles: true }),
  );

test("with doors armed, a click on a door square opens it and sends no move", async () => {
  const { r, s, sock } = await playerTableWithDoor();
  arm(r);
  clickBoard(r, 90, 10); // cell (0,0), the door
  await settle();

  expect(sock.sent).toHaveLength(1);
  expect(JSON.parse(sock.sent[0]!)).toHaveProperty("openDoor");
  s?.close();
});

test("with doors disarmed, the same click moves the token instead", async () => {
  const { r, s, sock } = await playerTableWithDoor();
  // Deliberately NOT armed -- doorsArmed starts false.
  clickBoard(r, 90, 10); // same cell (0,0) as the armed case above
  await settle();

  expect(sock.sent).toHaveLength(1);
  expect(JSON.parse(sock.sent[0]!)).toHaveProperty("moveToken");
  s?.close();
});

test("with doors armed, a click on a plain square sends nothing at all", async () => {
  // Armed means a non-door click does NOTHING -- it must not fall through to
  // a move. This is the behaviour change on the path players already use to
  // move tokens, so it gets its own direct assertion rather than relying on
  // the door-square case above to imply it.
  const { r, s, sock } = await playerTableWithDoor();
  arm(r);
  clickBoard(r, 210, 10); // cell (1,0): no Tiles entry at all, so not a door
  await settle();

  expect(sock.sent).toHaveLength(0);
  s?.close();
});

/** A DM at a live table, with the metadata routes served from `routes`. */
async function dmTable(routes: Record<string, unknown>) {
  localStorage.setItem("vtt.token", "tok-dm");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
    ...routes,
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  await settle();
  return { r, s, sock };
}

const byText = (r: HTMLElement, text: string) =>
  Array.from(r.querySelectorAll("button")).find((b) => b.textContent === text);

test("an adventure's guide is fetched and shown to the DM", async () => {
  // guideFor and notify are two separate closures app.ts hands the console,
  // and nothing was calling either. The guide button is the only path that
  // exercises both: fetch by id, then surface the result.
  const { r, s } = await dmTable({
    "/api/adventures": { adventures: [{ id: "keep", name: "The Cold Keep" }] },
    "/api/adventures/keep/guide": { guide: "The keep is cold and full of teeth." },
  });
  byText(r, "guide")!.click();
  await settle();
  expect(r.textContent).toContain("The keep is cold and full of teeth.");
  s?.close();
});

test("an adventure with no guide says so rather than showing nothing", async () => {
  // The `?? "no guide for that adventure"` arm. A silent button is
  // indistinguishable from a broken one.
  const { r, s } = await dmTable({
    "/api/adventures": { adventures: [{ id: "bare", name: "Bare" }] },
  });
  byText(r, "guide")!.click();
  await settle();
  expect(r.textContent).toContain("no guide for that adventure");
  s?.close();
});

test("with no adventures loaded the console shows no Adventures section", async () => {
  // adventures starts empty and stays empty when the server HAS none —
  // dmTable({}) answers 200 with an empty list, it does not fail. A phantom
  // entry would offer the DM a Load button for content the server does not
  // have.
  const { r, s } = await dmTable({});
  expect(r.textContent).not.toContain("Adventures");
  s?.close();
});

test("the DM's own console toggle arms doors, board-wide", async () => {
  // Exercises app.ts's real toggleDoors closure end to end, not the isolated
  // one dm-view.test.ts's harness tracks: a click on the CONSOLE's own
  // control (unlike the player-panel one, which mutates ui.doorsArmed
  // directly) goes through DMDeps.toggleDoors, which is this function.
  const { r, s, sock } = await dmTable({});
  // A scene, so there is a `.grid` at all to carry the class -- dmTable's
  // own default log has none (renderGrid's early "No scene yet." return
  // otherwise leaves nothing for the assertions below to find).
  sock.deliver(envelope(2, { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 4, gridHeight: 4 } }));
  await settle();
  // Re-queried after every click, deliberately: paint() rebuilds the whole
  // tree (renderSpectator's own root.replaceChildren), so a node reference
  // taken before a click is a DETACHED copy of the old DOM afterwards --
  // re-reading its textContent would silently pass on stale content.
  r.querySelector<HTMLButtonElement>('.dm [data-action="arm-doors"]')!.click();
  await settle();

  // The console's own label updates...
  expect(r.querySelector('.dm [data-action="arm-doors"]')!.textContent).toContain("armed");
  // ...and so does the board, which is the point of Task 4's board-visible
  // indication: a DM who arms doors and looks only at the board must still
  // be able to tell.
  expect(r.querySelector(".grid")!.classList.contains("armed")).toBe(true);
  expect(r.querySelector(".armed-label")).not.toBeNull();

  // Click again: it backs off, rather than being a one-way door.
  r.querySelector<HTMLButtonElement>('.dm [data-action="arm-doors"]')!.click();
  await settle();
  expect(r.querySelector(".grid")!.classList.contains("armed")).toBe(false);
  s?.close();
});

test("rotating the join link asks for confirmation, and sends when granted", async () => {
  // window.confirm is deliberate for a destructive action, and app.ts's
  // `confirm: (m) => window.confirm(m)` is the wiring under test. Both halves
  // are pinned: the dialog is consulted, and a granted one actually sends.
  //
  // This pair used to be pinned on Undo, the console's other confirming
  // control. Retraction left the platform, so rotation is now the ONLY
  // confirming control — the old secret is gone the moment it lands, and
  // nothing else in this suite would notice if the console stopped asking.
  const realConfirm = window.confirm;
  try {
    let asked = "";
    window.confirm = ((m: string) => { asked = m; return true; }) as typeof window.confirm;
    const { r, s, sock } = await dmTable({ "/api/join-link": { open: false, secret: "s3cret" } });
    byText(r, "New link")!.click();
    await settle();
    expect(asked).toContain("Replace the link?");
    expect(sock.sent.length).toBe(1);
    expect(JSON.parse(sock.sent[0]!)).toHaveProperty("rotateJoinLink");
    s?.close();
  } finally {
    window.confirm = realConfirm;
  }
});

test("a declined confirmation sends nothing at all", async () => {
  const realConfirm = window.confirm;
  try {
    window.confirm = (() => false) as typeof window.confirm;
    const { r, s, sock } = await dmTable({ "/api/join-link": { open: false, secret: "s3cret" } });
    byText(r, "New link")!.click();
    await settle();
    expect(sock.sent).toHaveLength(0);
    s?.close();
  } finally {
    window.confirm = realConfirm;
  }
});

test("a command that succeeds clears the toast rather than leaving one up", async () => {
  // The ok arm is an empty string on purpose. A stale "refused:" left over
  // from an earlier failure would misreport the command that just worked.
  const realConfirm = window.confirm;
  try {
    window.confirm = (() => true) as typeof window.confirm;
    const { r, s, sock } = await dmTable({});
    byText(r, "End session")!.click();
    await settle();
    const reqID = JSON.parse(sock.sent[0]!).requestId as string;
    sock.deliver({ result: { requestId: reqID, ok: true } });
    await settle();
    expect(r.textContent).not.toContain("refused");
    expect(r.querySelector(".toast")).toBeNull();
    s?.close();
  } finally {
    window.confirm = realConfirm;
  }
});

// REMOVED 2026-08-03: "a player with no token on the board clicks the board
// harmlessly". It claimed the socket staying silent pinned the `if (cmd)`
// guard at app.ts:104. It did not — wire.send dereferences cmd.requestId on
// the null and throws first, so the socket is silent with OR without the
// guard, and happy-dom swallows the listener exception. Proven by replacing
// the guard with act(cmd!): that test stayed green and only the one below
// went red. The test below is its correct replacement and was added in the
// same change; this one had been left BESIDE it rather than instead of it.

test("a null move command is never handed to send, even before the socket opens", async () => {
  // `if (cmd)` guards act() against moveCommandFor's null. Dropping the guard
  // is invisible on a live socket — send() throws on the null deep inside the
  // wire and the exception dies unnoticed in a DOM event handler. Before the
  // socket opens it is very visible instead: wire.send answers "not
  // connected" without ever touching the command, and that refusal is shown
  // as a toast. So a page that has not connected yet is where the missing
  // guard announces itself.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Lera", role: "player" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  // Deliberately NOT opened. Frames still arrive; the socket is not writable.
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  sock.deliver(envelope(2, { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8 } }));
  sock.deliver(envelope(3, { actorAdded: { actor: { actorId: "a1", name: "Lera" } } }));
  sock.deliver(
    envelope(4, {
      actorControlGranted: { actorId: "a1", participantId: "p", kind: "ACTOR_KIND_PARTY_MEMBER" },
    }),
  );
  await settle();

  const board = r.querySelector(".grid") as HTMLElement;
  board?.dispatchEvent(new MouseEvent("click", { clientX: 5, clientY: 5, bubbles: true }));
  await settle();

  expect(sock.sent).toHaveLength(0);
  expect(r.textContent).not.toContain("not connected");
  s?.close();
});

test("the board is painted with a status before any metadata request goes out", async () => {
  // boot() paints once and only THEN issues fetchMe, both synchronously. That
  // ordering is what puts something on screen before the network is touched,
  // and the fetch stub is the seam that can see it: whatever the DOM holds
  // when the first request is made is what a user with a slow connection
  // stares at.
  //
  // This kills the `status = "connecting"` initializer, which was wrongly
  // adjudicated equivalent on the reasoning that both paints happen in one
  // task so nothing can observe the first. Something can: anything boot()
  // calls between them.
  localStorage.setItem("vtt.token", "tok");
  useFakeSocket();
  const r = root();
  let domAtFirstFetch = "";
  const realFetch = globalThis.fetch;
  try {
    // Parameter declared, though unused: a zero-arg stub is not comparable to
    // `typeof fetch` (which carries `preconnect`), and TS2352 fails the
    // typecheck gate even though bun test is perfectly happy. Same shape as
    // stubMetadata above. bun test does not catch this; client:typecheck does.
    globalThis.fetch = (async (_input: RequestInfo | URL) => {
      if (domAtFirstFetch === "") domAtFirstFetch = r.textContent ?? "";
      return new Response("", { status: 404 });
    }) as typeof fetch;
    const s = boot(r);
    await settle();
    expect(domAtFirstFetch).toContain("connecting");
    s?.close();
  } finally {
    globalThis.fetch = realFetch;
  }
});

test("clicking Reconnect redials from the last folded sequence", () => {
  // The whole T5 wiring end to end: status "closed" surfaces the control, the
  // click reaches session.reconnect(), and the redial resumes near where the
  // client left off instead of replaying the campaign from zero.
  //
  // after=6 rather than after=7, and that is the cursor's own contract rather
  // than a fudge here: since the gateway projects per seat, several envelopes
  // can share one sequence, so the last sequence seen may be incomplete and is
  // rolled back and taken again (see wire.ts's replay-cursor note). What this
  // test is for is unchanged — an emptied handler redials nothing at all.
  //
  // Without this the app.ts handler body could be emptied and every other test
  // still passed — the button existed and did nothing.
  localStorage.setItem("vtt.token", "tok-1");
  useFakeSocket();
  const r = root();
  const session = boot(r);

  const first = FakeSocket.instances[0]!;
  first.open();
  first.deliver(envelope(7, { sessionStarted: { name: "S" } }));
  first.close(); // the network drops; status becomes "closed"

  const btn = r.querySelector(".reconnect") as HTMLButtonElement | null;
  expect(btn).not.toBeNull();
  btn!.click();

  expect(FakeSocket.instances).toHaveLength(2);
  expect(FakeSocket.instances[1]!.url).toContain("after=6");
  session?.close();
});

test("presence reaches the DM console's grant dropdown", async () => {
  // The SEAM. Everything either side of it is tested — Session keeps the
  // participant list, the console renders whatever it is handed — but the one
  // line joining them was covered by nothing. Replacing it with `[]` left all
  // 438 tests green while making the panel decorative: every controller shows
  // as a raw uuid and the DM can never grant to anybody.
  //
  // That is this branch's own recurring shape: complete everywhere except the
  // seam. The gateway's ToEvent arm was the same bug one layer down.
  localStorage.setItem("vtt.token", "tok-dm");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm" },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();

  const r = root();
  const session = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { sessionStarted: { name: "Night One" } }));
  sock.deliver(envelope(2, { actorAdded: { actor: { actorId: "act-warden", name: "Warden" } } }));
  sock.deliver({
    presenceSnapshot: {
      present: [{ participantId: "p-lera", displayName: "Lera", state: "PRESENCE_STATE_CONNECTED" }],
    },
  });
  await settle();

  const target = r.querySelector(".grant-target") as HTMLSelectElement | null;
  expect(target).not.toBeNull();
  expect(Array.from(target!.querySelectorAll("option")).map((o) => o.textContent))
    .toEqual(["choose a participant", "Lera"]);

  // The OTHER participants seam, one line over in app.ts: the status header's
  // presence list. It shipped with T5 uncovered by the same omission, and
  // emptying it left every test green — the table would simply look deserted.
  expect(Array.from(r.querySelectorAll(".participant")).map((n) => n.textContent))
    .toEqual(["Lera"]);
  session?.close();
});

// --- the shared join link (plan J5) --------------------------------------

/** Answer /join with one canned response; everything else 404s. */
function stubJoinRoute(status: number, body: string): { bodies: string[] } {
  const bodies: string[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input)).pathname;
    if (path !== "/join") return new Response("", { status: 404 });
    bodies.push(String(init?.body ?? ""));
    return new Response(body, { status });
  }) as typeof fetch;
  return { bodies };
}

test("a shared join link with no credential yet asks who you are", async () => {
  // The whole point of the feature: somebody with no invite, no token and no
  // account opens ONE link and is asked a single question.
  setURL("http://localhost/?join=s3cret");
  const r = root();

  const session = boot(r);

  expect(session).toBeNull();
  expect(r.querySelector('[data-field="join-name"]')).not.toBeNull();
  // And emphatically NOT the dead end that used to be the only other branch.
  expect(r.textContent).not.toContain("invite token");
  // The form starts EMPTY and unalarmed. Both halves matter: a prefilled name
  // is somebody else's, and an error box before anything has been tried reads
  // as "this is already broken" to a person who has not typed a character.
  expect(r.querySelector<HTMLInputElement>('[data-field="join-name"]')!.value).toBe("");
  expect(r.querySelector(".error")).toBeNull();
});

test("a stored token wins over the join link, so reopening it does not mint a second you", async () => {
  // THE OPPOSITE PRECEDENCE FROM ?token=, and deliberately so. A ?token= link
  // is an act of re-invitation aimed at one person, so it overrides. A ?join=
  // link is a durable URL that everybody at the table keeps and reopens — if
  // it won, every visit would mint a NEW participant, and the returning player
  // would arrive as a stranger with none of their characters, while the DM
  // watched the roster fill with duplicates nobody can tell apart.
  localStorage.setItem("vtt.token", "tok-mine");
  setURL("http://localhost/?join=s3cret");
  useFakeSocket();
  stubMetadata({});

  const session = boot(root());
  await settle();

  expect(FakeSocket.instances).toHaveLength(1);
  expect(FakeSocket.instances[0]!.url).toContain("token=tok-mine");
  expect(document.querySelector('[data-field="join-name"]')).toBeNull();
  session?.close();
});

test("joining stores the credential, strips the secret from the address bar, and dials", async () => {
  // All three, because each has failed on its own elsewhere in this client:
  // a token that is fetched but not stored means the next reload asks again;
  // a secret left in the URL is a shareable credential in browser history;
  // and a join that never dials leaves a form that looks like it worked.
  setURL("http://localhost/?join=s3cret");
  useFakeSocket();
  const calls: string[] = [];
  const titles: unknown[] = [];
  const original = history.replaceState.bind(history);
  history.replaceState = ((...args: unknown[]) => {
    titles.push(args[1]);
    calls.push(String(args[2]));
    return original(args[0] as never, args[1] as string, args[2] as string);
  }) as typeof history.replaceState;
  const seen = stubJoinRoute(200, JSON.stringify({ token: "tok-joined" }));

  const r = root();
  boot(r);
  r.querySelector<HTMLInputElement>('[data-field="join-name"]')!.value = "Kim";
  r.querySelector<HTMLInputElement>('[data-field="join-name"]')!.dispatchEvent(new Event("input"));
  r.querySelector<HTMLButtonElement>('[data-action="join"]')!.click();
  await settle();

  expect(seen.bodies).toHaveLength(1);
  expect(JSON.parse(seen.bodies[0]!)).toEqual({ secret: "s3cret", displayName: "Kim" });
  expect(localStorage.getItem("vtt.token")).toBe("tok-joined");
  // toHaveLength FIRST. `calls.some(...)` on an empty array is false, so the
  // old form was satisfied by replaceState never being called at all —
  // deleting the strip left all 486 tests passing. The join secret is the more
  // dangerous of the two credentials this client handles: it admits ANYONE, so
  // leaving it in history and in every outbound Referer is worse than leaking
  // one person's token.
  expect(calls).toHaveLength(1);
  expect(calls[0]).not.toContain("join");
  // The same "no title claimed" assertion as the ?token= strip above carries,
  // and for the same reason: two call sites strip two different credentials,
  // and each is the only place its own arguments can be seen.
  expect(titles).toEqual([""]);
  expect(FakeSocket.instances).toHaveLength(1);
  expect(FakeSocket.instances[0]!.url).toContain("token=tok-joined");

  history.replaceState = original;
  FakeSocket.instances[0]!.close();
});

test("a refused join leaves the form up, says why, and stores nothing", async () => {
  // The failure that must not silently half-succeed: storing a credential the
  // server refused would make every later reload dial with a dead token and
  // report a connection problem instead of a closed door.
  setURL("http://localhost/?join=s3cret");
  useFakeSocket();
  stubJoinRoute(403, "gateway: this link is not accepting anyone\n");

  const r = root();
  boot(r);
  r.querySelector<HTMLButtonElement>('[data-action="join"]')!.click();
  await settle();

  expect(r.querySelector('[data-field="join-name"]')).not.toBeNull();
  expect(r.querySelector(".error")!.textContent).toContain("DM");
  expect(localStorage.getItem("vtt.token")).toBeNull();
  expect(FakeSocket.instances).toHaveLength(0);
  // AND THEY CAN TRY AGAIN. The in-flight flag has to be cleared on the way
  // out of a refusal, or the button stays disabled for good and the only
  // recovery from a closed door is reloading the page — which somebody who
  // has just been told "ask your DM" has no reason to think of.
  expect(r.querySelector<HTMLButtonElement>('[data-action="join"]')!.disabled).toBe(false);
});

test("Enter while a join is in flight does not mint a second you", async () => {
  // Found reviewing my own wiring rather than by a test, which is why it is
  // here: the double-press guard is the DISABLED BUTTON, and Enter never
  // touches the button. Every post mints a participant, so a second one puts
  // the same person at the table twice holding two credentials — and nothing
  // will ever revoke the spare, because nobody knows it exists.
  setURL("http://localhost/?join=s3cret");
  useFakeSocket();
  const posts: string[] = [];
  let release: (r: Response) => void = () => {};
  globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
    posts.push(String(init?.body ?? ""));
    // Never settles until this test says so, so the second press lands while
    // the first is genuinely still in flight rather than after it.
    return new Promise<Response>((r) => {
      release = r;
    });
  }) as unknown as typeof fetch;

  const r = root();
  boot(r);
  const typeInto = () => {
    const i = r.querySelector<HTMLInputElement>('[data-field="join-name"]')!;
    i.value = "Kim";
    i.dispatchEvent(new Event("input"));
    return i;
  };
  typeInto();
  r.querySelector<HTMLButtonElement>('[data-action="join"]')!.click();
  await settle();

  // The repaint rebuilt the input, so this is deliberately re-queried: asking
  // the stale element would dispatch at a node no longer in the document and
  // pass whether or not the guard exists.
  // Nothing is reported as wrong while the answer is still outstanding: the
  // submit clears any previous error before it starts, so a retry after a
  // closed door does not sit under the old message while it runs.
  expect(r.querySelector(".error")).toBeNull();

  r.querySelector<HTMLInputElement>('[data-field="join-name"]')!.dispatchEvent(
    new KeyboardEvent("keydown", { key: "Enter" }),
  );
  await settle();

  expect(posts).toHaveLength(1);

  release(new Response(JSON.stringify({ token: "tok-joined" }), { status: 200 }));
  await settle();
  FakeSocket.instances[0]?.close();
});

test("a DM's roster learns about somebody who joins after the console loaded", async () => {
  // The gap the e2e found, and the only test that could: every layer worked —
  // /join minted the spectator, presence announced them, the console rendered
  // a roster — and the roster was read ONCE at boot, so the one person who
  // needed a promote button was the one person who never had one.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  let rosterCalls = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/join-link") return Response.json({ open: true, secret: "s3cret" });
    if (path === "/api/participants") {
      rosterCalls++;
      // FIRST read predates the joiner, exactly as at a real table.
      return Response.json(
        rosterCalls === 1
          ? [{ participantId: "p-dm", name: "Ari", role: "dm" }]
          : [
              { participantId: "p-dm", name: "Ari", role: "dm" },
              { participantId: "p-new", name: "Robin", role: "spectator" },
            ],
      );
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  const sock = FakeSocket.instances[0]!;
  sock.open();
  await settle();
  expect(rosterCalls).toBe(1);

  // Somebody arrives. Presence is the ONLY signal — no event is appended,
  // because joining is not campaign history.
  sock.deliver({
    presenceChanged: { participantId: "p-new", displayName: "Robin", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();

  expect(rosterCalls).toBe(2);
  expect(r.querySelector('[data-action="promote-p-new"]')).not.toBeNull();

  // And it does NOT re-read on every subsequent frame: a console that fetched
  // per event would put a request behind every token move at the table.
  const after = rosterCalls;
  sock.deliver(envelope(9, { sceneCreated: { sceneId: "s9", name: "Vault", gridWidth: 4, gridHeight: 4 } }));
  await settle();
  expect(rosterCalls).toBe(after);

  session?.close();
});

test("a promoted spectator's own screen catches up without reconnecting", async () => {
  // The half of promotion that live re-resolution does NOT reach. The server
  // starts accepting this person's commands the moment the DM promotes them —
  // and their browser read its role once, at connect. Without this they can
  // act and their own client offers them nothing to act with: the server says
  // yes to a screen with no controls on it.
  //
  // The server re-announces a promoted participant for exactly this reason, so
  // a presence frame naming ME means "read your role again". Watching the
  // participant LIST cannot serve: they were already present, so it does not
  // change.
  setURL("http://localhost/?token=tok-watcher");
  useFakeSocket();
  let role = "spectator";
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-me", name: "Robin", role });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { actorAdded: { actor: { actorId: "a1", name: "Ash" } } }));
  sock.deliver(
    envelope(2, {
      actorControlGranted: { actorId: "a1", participantId: "p-me", kind: "ACTOR_KIND_PARTY_MEMBER" },
    }),
  );
  await settle();
  expect(r.querySelector(".player")).toBeNull(); // a spectator watches

  // The DM promotes them elsewhere; the server nudges this connection.
  role = "player";
  sock.deliver({
    presenceChanged: { participantId: "p-me", displayName: "Robin", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();

  expect(r.querySelector(".player")).not.toBeNull();

  session?.close();
});

test("only a DM or agent reads the join link, and a refusal does not become an unhandled rejection", async () => {
  // Two properties, both about the routes that are role-gated server-side.
  //
  // A player must not ASK: the request would 403, and getJSON turns a 403 into
  // a thrown error rather than an absence — so asking would cost an unhandled
  // rejection for a panel the player was never going to see.
  setURL("http://localhost/?token=tok-player");
  useFakeSocket();
  const asked: string[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    asked.push(path);
    if (path === "/api/me") {
      return Response.json({ participantId: "p-1", name: "Lera", role: "player" });
    }
    if (path === "/api/join-link" || path === "/api/participants") {
      return new Response("gateway: not authorized", { status: 403 });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const session = boot(root());
  await settle();
  // And still not when presence churns — including a frame naming the player
  // THEMSELVES, which is what a promotion sends. Re-reading your own role is
  // right; asking for a link you may not have is not.
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({
    presenceChanged: { participantId: "p-1", displayName: "Lera", state: "PRESENCE_STATE_CONNECTED" },
  });
  sock.deliver({
    presenceChanged: { participantId: "p-2", displayName: "Ari", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();

  expect(asked).toContain("/api/me");
  expect(asked).not.toContain("/api/join-link");
  expect(asked).not.toContain("/api/participants");
  session?.close();
});

test("a DM whose credential is pulled mid-session loses the panels, not the page", async () => {
  // The refusal path that IS reachable: a DM asks legitimately, and by the
  // time the request lands they have been revoked. getJSON throws; without a
  // catch that is an unhandled rejection, and the console keeps rendering a
  // link nobody can use.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  let allowed = true;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/join-link") {
      return allowed
        ? Response.json({ open: true, secret: "s3cret" })
        : new Response("gateway: not authorized", { status: 403 });
    }
    if (path === "/api/participants") {
      return allowed
        ? Response.json([{ participantId: "p-dm", name: "Ari", role: "dm" }])
        : new Response("gateway: not authorized", { status: 403 });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).not.toBeNull();

  allowed = false;
  const sock = FakeSocket.instances[0]!;
  sock.open();
  // Names the DM THEMSELVES, so the self-role re-read runs too and its own
  // follow-up refresh is exercised rather than skipped.
  sock.deliver({
    presenceChanged: { participantId: "p-dm", displayName: "Ari", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();

  // The panel is gone and the page is still a page.
  expect(r.querySelector('[data-field="join-link"]')).toBeNull();
  expect(r.querySelector(".conn")).not.toBeNull();
  session?.close();
});

test("an AGENT gets the sharing panel too, and a presence frame keeps the roster fresh", async () => {
  // Agents are dm-equivalent for the door and the roster (spec §5), and the
  // role check is written once — so this is the case that proves "once" means
  // both, rather than "dm" with an unreached second clause.
  setURL("http://localhost/?token=tok-agent");
  useFakeSocket();
  let rosterCalls = 0;
  let meCalls = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      meCalls++;
      return Response.json({ participantId: "p-a", name: "Aide", role: "agent" });
    }
    if (path === "/api/join-link") return Response.json({ open: false, secret: "s3cret" });
    if (path === "/api/participants") {
      rosterCalls++;
      return Response.json([{ participantId: "p-a", name: "Aide", role: "agent" }]);
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).not.toBeNull();
  expect(rosterCalls).toBe(1);

  const sock = FakeSocket.instances[0]!;
  sock.open();
  // A batch naming SOMEBODY ELSE. The roster still has to be re-read — that is
  // the whole newcomer case — even though nothing about this agent changed.
  sock.deliver({
    presenceChanged: { participantId: "p-new", displayName: "Robin", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  expect(rosterCalls).toBe(2);
  // But NOT a re-read of this agent's own role: the frame names somebody else,
  // and my role cannot have changed because theirs did. A client that re-read
  // /api/me on every presence frame would put a request behind every arrival
  // and departure at the table, for an answer it already has.
  expect(meCalls).toBe(1);

  // And an ordinary EVENT does not: onChange fires per frame, and a roster
  // read behind every token move at the table is not a feature.
  const before = rosterCalls;
  sock.deliver(envelope(4, { sceneCreated: { sceneId: "s4", name: "Attic", gridWidth: 3, gridHeight: 3 } }));
  await settle();
  expect(rosterCalls).toBe(before);

  session?.close();
});

test("a batch naming me AND somebody else still re-reads my own role", async () => {
  // The snapshot every connection gets names everyone, and a promotion's
  // re-announcement can arrive batched with an arrival. A check that only
  // acted when the batch named ME ALONE would skip exactly those.
  setURL("http://localhost/?token=tok-watcher");
  useFakeSocket();
  let meCalls = 0;
  let role = "spectator";
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      meCalls++;
      return Response.json({ participantId: "p-me", name: "Robin", role });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { actorAdded: { actor: { actorId: "a1", name: "Ash" } } }));
  sock.deliver(
    envelope(2, {
      actorControlGranted: { actorId: "a1", participantId: "p-me", kind: "ACTOR_KIND_PARTY_MEMBER" },
    }),
  );
  await settle();
  const before = meCalls;

  role = "player";
  sock.deliver({
    presenceSnapshot: {
      present: [
        { participantId: "p-other", displayName: "Ari", state: "PRESENCE_STATE_CONNECTED" },
        { participantId: "p-me", displayName: "Robin", state: "PRESENCE_STATE_CONNECTED" },
      ],
    },
  });
  await settle();

  expect(meCalls).toBe(before + 1);
  expect(r.querySelector(".player")).not.toBeNull();
  // Promoted to PLAYER, not to dm — so the re-read must not go on to ask for
  // the sharing panel. A role change is a reason to re-check what you may do,
  // not a reason to assume it grew.
  expect(r.querySelector('[data-field="join-link"]')).toBeNull();
  session?.close();
});

test("a network blip keeps the DM's panels; only a refusal takes them away", async () => {
  // Two different failures that getJSON reports the same way (both throw), and
  // they deserve opposite answers. A 403 means "you may not read this" — a
  // revocation mid-session — and dropping the panel is right. A network fault
  // means "this read failed", and dropping the panel for that takes the door
  // and the roster away from a DM who still has every right to them, silently.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  let mode: "ok" | "blip" | "refused" = "ok";
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/join-link" || path === "/api/participants") {
      if (mode === "blip") throw new TypeError("Failed to fetch");
      if (mode === "refused") return new Response("gateway: not authorized", { status: 403 });
      return path === "/api/join-link"
        ? Response.json({ open: true, secret: "s3cret" })
        : Response.json([{ participantId: "p-dm", name: "Ari", role: "dm" }]);
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).not.toBeNull();

  const sock = FakeSocket.instances[0]!;
  sock.open();
  const nudge = (id: string) =>
    sock.deliver({
      presenceChanged: { participantId: id, displayName: "X", state: "PRESENCE_STATE_CONNECTED" },
    });

  // A blip: the panel STAYS. There is nothing new to show, and nothing was
  // withdrawn.
  mode = "blip";
  nudge("p-a");
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).not.toBeNull();

  // A refusal: the panel goes.
  mode = "refused";
  nudge("p-b");
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).toBeNull();

  session?.close();
});

test("a slow roster answer cannot overwrite a fresher one", async () => {
  // Two refreshes can be in flight at once — a promotion and an arrival a
  // moment apart — and both assignments are last-writer-wins on COMPLETION,
  // not on issue order. The first read describes the table BEFORE the
  // promotion; if it lands second it repaints a promoted player as the
  // spectator they were, and nothing corrects it until the next presence
  // frame.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  const gate: ((v: Response) => void)[] = [];
  let rosterCall = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/join-link") return Response.json({ open: true, secret: "s" });
    if (path === "/api/participants") {
      rosterCall++;
      const body =
        rosterCall === 1
          ? [{ participantId: "p-k", name: "Kim", role: "spectator" }] // the OLD truth
          : [{ participantId: "p-k", name: "Kim", role: "player" }]; // after the promotion
      // Held open so the test decides the ORDER they resolve in.
      return new Promise<Response>((r) => gate.push(() => r(Response.json(body))));
    }
    return new Response("", { status: 404 });
  }) as unknown as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  expect(gate).toHaveLength(1); // the boot read, in flight

  // A presence frame issues a SECOND refresh while the first is outstanding.
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({
    presenceChanged: { participantId: "p-k", displayName: "Kim", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  expect(gate).toHaveLength(2);

  // The FRESH one lands first, then the stale one. Out of order on purpose:
  // that is the whole scenario, and it is the order a slow first request
  // produces.
  gate[1]!(undefined as never);
  await settle();
  expect(r.querySelector(".roster-row .role")!.textContent).toBe("player");

  gate[0]!(undefined as never);
  await settle();
  expect(r.querySelector(".roster-row .role")!.textContent).toBe("player");

  session?.close();
});

test("a stale refusal cannot clear a panel that has been refreshed since", async () => {
  // The catch path needs the same ticket the success path has. A 403 answering
  // a request issued BEFORE a revocation was undone — or simply before a newer
  // read succeeded — would otherwise pull the door and the roster off a
  // console that had just been told it may have them.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  const gate: (() => void)[] = [];
  let call = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/participants") {
      return Response.json([{ participantId: "p-dm", name: "Ari", role: "dm" }]);
    }
    if (path === "/api/join-link") {
      call++;
      const n = call;
      return new Promise<Response>((resolve, reject) => {
        gate.push(() => {
          // The FIRST read is refused; the second succeeds.
          if (n === 1) reject(new Error("metadata: forbidden — this role may not read that"));
          else resolve(Response.json({ open: true, secret: "s3cret" }));
        });
      });
    }
    return new Response("", { status: 404 });
  }) as unknown as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();

  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({
    presenceChanged: { participantId: "p-x", displayName: "X", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  expect(gate).toHaveLength(2);

  // The fresh read succeeds, THEN the stale refusal arrives.
  gate[1]!();
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).not.toBeNull();

  gate[0]!();
  await settle();
  expect(r.querySelector('[data-field="join-link"]')).not.toBeNull();

  session?.close();
});

test("a stale join-link answer cannot overwrite a fresher one", async () => {
  // The success path's twin of the roster test above. A door read issued
  // before the DM closed the door, landing after the read that saw it shut,
  // repaints the console as OPEN — and the DM believes the link is live.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  const gate: (() => void)[] = [];
  let call = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/participants") {
      return Response.json([{ participantId: "p-dm", name: "Ari", role: "dm" }]);
    }
    if (path === "/api/join-link") {
      call++;
      const open = call === 1; // the first read saw it open; it has since shut
      return new Promise<Response>((resolve) => {
        gate.push(() => resolve(Response.json({ open, secret: "s3cret" })));
      });
    }
    return new Response("", { status: 404 });
  }) as unknown as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({
    presenceChanged: { participantId: "p-x", displayName: "X", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  expect(gate).toHaveLength(2);

  gate[1]!(); // the fresh answer: the door is SHUT
  await settle();
  expect(r.querySelector(".door-state")!.textContent).toBe("door: closed");

  gate[0]!(); // the stale answer: it was open when this was asked
  await settle();
  expect(r.querySelector(".door-state")!.textContent).toBe("door: closed");

  session?.close();
});

test("a refusal DOES clear the roster, and a stale one does not", async () => {
  // Both directions of the roster's catch. Without the first, a revoked DM
  // keeps promote buttons that no longer work; without the second, a refusal
  // answering an old request pulls the panel off a console that has since been
  // told it may have it.
  setURL("http://localhost/?token=tok-dm");
  useFakeSocket();
  const gate: (() => void)[] = [];
  let call = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/me") {
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm" });
    }
    if (path === "/api/join-link") return Response.json({ open: true, secret: "s" });
    if (path === "/api/participants") {
      call++;
      const n = call;
      return new Promise<Response>((resolve, reject) => {
        gate.push(() => {
          // Reads 2 AND 3 are refused. Read 3 is the one that lands stale,
          // behind read 4's success — a refusal arriving late is the case the
          // ticket exists for, and an earlier version of this test made the
          // stale one a SUCCESS, so the clause was never exercised.
          if (n === 2 || n === 3) {
            reject(new Error("metadata: forbidden — this role may not read that"));
          } else {
            resolve(Response.json([{ participantId: "p-dm", name: "Ari", role: "dm" }]));
          }
        });
      });
    }
    return new Response("", { status: 404 });
  }) as unknown as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  gate[0]!();
  await settle();
  expect(r.querySelector(".roster-row")).not.toBeNull();

  // A LIVE refusal clears it.
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver({
    presenceChanged: { participantId: "p-x", displayName: "X", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  gate[1]!();
  await settle();
  expect(r.querySelector(".roster-row")).toBeNull();

  // Now a fresh success, then a STALE refusal behind it: the panel stays.
  sock.deliver({
    presenceChanged: { participantId: "p-y", displayName: "Y", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  sock.deliver({
    presenceChanged: { participantId: "p-z", displayName: "Z", state: "PRESENCE_STATE_CONNECTED" },
  });
  await settle();
  expect(gate).toHaveLength(4);
  gate[3]!(); // read 4: fresh success, the panel comes back
  await settle();
  expect(r.querySelector(".roster-row")).not.toBeNull();
  gate[2]!(); // read 3: a REFUSAL, now stale — it must not pull the panel again
  await settle();
  expect(r.querySelector(".roster-row")).not.toBeNull();

  session?.close();
});

// --- the spectator's perch (visibility spec §3.1.1) -------------------------

/** Boot at a seeded table under one role, and return the root plus the socket. */
async function tableAs(role: string) {
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Watcher", role },
    "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
    "/api/adventures": { adventures: [] },
  });
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock);
  await settle();
  return { r, s, sock };
}

test("only a spectator is offered a shoulder", async () => {
  // MayPerch refuses every other role outright: "role %q does not perch — a
  // viewpoint is the spectator's". The DM and the agent already see
  // everything, and an unassigned PLAYER's answer to an empty board is to be
  // GIVEN a character, which is the onboarding flow working as intended.
  //
  // All four roles in one test on purpose: the positive and the negative
  // differ by one branch, and a test that only asserted the spectator's
  // control would pass an app.ts that rendered it for everybody.
  const watcher = await tableAs("spectator");
  expect(watcher.r.querySelector(".perch")).not.toBeNull();
  watcher.s?.close();

  for (const role of ["player", "dm", "agent"]) {
    const other = await tableAs(role);
    expect(other.r.querySelector(".perch")).toBeNull();
    expect(other.r.textContent).not.toContain("Perched on");
    other.s?.close();
  }
});

test("choosing a shoulder sends set_viewpoint over the wire", async () => {
  // THE SEAM. Everything either side of it is tested — the builder makes the
  // command, the control offers the party — and the one line joining them was
  // covered by nothing, which is this branch's recurring shape.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  expect(asme).toBeDefined();
  asme.click();
  await settle();

  expect(sock.sent).toHaveLength(1);
  const cmd = JSON.parse(sock.sent[0]!);
  expect(cmd).toHaveProperty("setViewpoint");
  expect(cmd.setViewpoint).toMatchObject({ actorId: "a1" });
  s?.close();
});

test("the indicator follows the SERVER's answer, never the click", async () => {
  // A refused perch that still moved the indicator would say the spectator is
  // riding a shoulder the server never gave them — and the board, which comes
  // from the server, would disagree with the label above it. That is exactly
  // the "the board changes under you with no way to know why" failure the
  // indicator exists to prevent, inverted.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  const reqID = JSON.parse(sock.sent[0]!).requestId as string;
  sock.deliver({ result: { requestId: reqID, ok: false, error: "not a party member" } });
  await settle();

  expect(r.querySelector(".perch .perched-on")?.textContent).toBe("Perched on: nobody");
  expect(r.textContent).toContain("refused: not a party member");
  s?.close();
});

test("an accepted perch moves the indicator", async () => {
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  const reqID = JSON.parse(sock.sent[0]!).requestId as string;
  sock.deliver({ result: { requestId: reqID, ok: true } });
  await settle();

  expect(r.querySelector(".perch .perched-on")?.textContent).toBe("Perched on: Lera");
  s?.close();
});

test("an accepted perch CLEARS the refusal the last one left up", async () => {
  // The success arm of perchOn's toast, which the refusal tests above cannot
  // see: they only ever assert that a refusal APPEARS, and a success that
  // wrote some other string — or left the old refusal standing — would satisfy
  // every one of them. A watcher who is told "refused: not a party member"
  // while sitting on the shoulder the server just granted them has been handed
  // the same contradiction the indicator exists to prevent, and would have no
  // reason to trust either line again.
  //
  // The refusal FIRST, deliberately: asserting only that an accepted perch
  // shows no toast would also pass on a page that never had one, so the
  // clearing is what is actually pinned.
  const { r, s, sock } = await tableAs("spectator");
  const shoulder = () =>
    Array.from(r.querySelectorAll(".perch .shoulder"))
      .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  shoulder().click();
  await settle();
  sock.deliver({
    result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: false, error: "not a party member" },
  });
  await settle();
  expect(r.querySelector(".toast")?.textContent).toContain("refused: not a party member");

  shoulder().click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[1]!).requestId as string, ok: true } });
  await settle();

  expect(r.querySelector(".toast")).toBeNull();
  expect(r.textContent).not.toContain("refused");
  s?.close();
});

test("a spectator's reconnect starts the log over, and re-sends the shoulder", async () => {
  // THE HAZARD, and the reason this is a task rather than an afternoon.
  //
  // A perch is CONNECTION state: the server's projector is reborn perched on
  // nobody, so a redial that resumes leaves this client holding a board the
  // new connection knows nothing about. Measured against the real gateway
  // (internal/gateway, 2026-08-24) with a watcher who had perched and then
  // seen a goblin walk into view:
  //
  //   - resume + re-perch  -> `engine: scene "ambush" already exists`, the
  //     duplicate-introduction freeze, because the reborn projector
  //     re-introduces everything the perch shows it;
  //   - resume + drop the sequence-0 frames first -> `engine: token placed in
  //     unknown scene "ambush"`, and it fails BEFORE the re-perch: the frames
  //     an ordinary event delivered (the goblin's arrival at sequence 12) are
  //     stamped with the causing sequence, survive the rollback, and depend on
  //     a scene that only the perch ever introduced. Dropping the perch frames
  //     breaks the log that is left;
  //   - resume and DON'T re-perch -> folds, and shows a board that never
  //     updates again: the reborn projector sends this seat nothing, and the
  //     sequence the rollback dropped is never re-sent either;
  //   - dial after=0 with an empty log, then re-perch -> folds cleanly, and is
  //     the only one of the four that also shows a live board.
  //
  // So a spectator redials by STARTING OVER. Asserted here as the cursor
  // (after=0, not seenSeq-1), the emptied log, and the re-sent shoulder.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: true } });
  // The board the perch produced: sequence-0 frames, which no rollback drops.
  sock.deliver(envelope(0, { sceneCreated: { sceneId: "s2", name: "Ambush", gridWidth: 4, gridHeight: 4 } }));
  await settle();
  expect(s!.state.Scenes["s2"]).toBeDefined();

  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();

  expect(FakeSocket.instances).toHaveLength(2);
  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=0");
  // NOT YET EMPTIED, and that is the ordering rather than an accident of this
  // test: a dial that fails must cost this watcher nothing, so the board they
  // have stays up until there is a connection to replace it. Wire.restart says
  // so; without this line moving the discard back above the dial goes
  // unnoticed, and a refused redial blanks the page.
  expect(s!.state.Scenes["s2"]).toBeDefined();

  redial.open();
  await settle();
  // NOW it is emptied, perch frames and all. `after=0` alone would not be
  // enough: the whole log is about to arrive again, and keeping the old one
  // double-folds it.
  expect(s!.state.Scenes["s2"]).toBeUndefined();
  expect(s!.events).toHaveLength(0);
  const resent = redial.sent.map((raw) => JSON.parse(raw));
  expect(resent).toHaveLength(1);
  expect(resent[0]).toHaveProperty("setViewpoint");
  expect(resent[0].setViewpoint).toMatchObject({ actorId: "a1" });
  s?.close();
});

test("a redial whose abandoned socket closes late still ends up perch-shaped", async () => {
  // THE OTHER END of wire.test.ts's "a close answers the commands sent on ITS
  // socket, and no others", seen from the only place the defect can hurt
  // anybody.
  //
  // restart() is close-then-send by construction: it closes the old socket,
  // dials a new one, and this app re-sends the perch on the new one. So a close
  // landing after the dial has a command in front of it that it knows nothing
  // about, and a close handler that walked the whole pending map answered that
  // command with "connection closed" — a refusal the server never sent.
  // Everything downstream follows:
  //
  //   - perchOn saw ok=false, so it showed the refusal and, worse, left
  //     `perchShaped` FALSE;
  //   - the server, which heard none of this, accepted the perch and sent the
  //     perch frames, which the new socket folded — so the log IS perch-shaped
  //     while the flag says it is not;
  //   - so the NEXT redial resumed instead of starting over, holding frames
  //     only a perch ever produced, and the reborn projector re-introduced the
  //     scene: `engine: scene "..." already exists`, the duplicate-introduction
  //     freeze Wire.restart exists to prevent, arriving by a different road.
  //     session.ts re-folds its whole log on every event, so it is permanent.
  //
  // The last of those is the assertion at the bottom, and it is the one that
  // catches the whole chain — verified by removing the assertions above it and
  // watching the redial ask for after=4. The refusal it also asserts is the
  // same fault one step earlier, at the only point a watcher sees anything at
  // all.
  //
  // WHAT THIS STAGES, AND WHAT IT DOES NOT. The ordering is the realistic part:
  // no WebSocket dispatches `close` from inside close(), so a close of a live
  // socket lands after whatever the caller did next — which is what lateClose
  // models, and FakeSocket's inline dispatch is what hides. The scenario is
  // NOT one today's UI produces: the Reconnect button appears only on status
  // "closed", and the wire sets that status in the same handler that walks the
  // pending map, so the app's sockets are strictly serial and the close the
  // drop below delivers has already walked an empty map. Reaching the defect
  // through this door therefore takes a second close on a socket that has
  // already reported one. That makes this a test of the DESIGN restart() is
  // one caller of — the first automatic redial, or any second live connection,
  // makes it a test of the shipped path — and its narrative above is a fault
  // chain traced through the code, not measured history.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: true } });
  await settle();

  // The connection drops. THIS close dispatches immediately — it is what turns
  // the status to "closed" and offers the Reconnect button — and there is
  // nothing in flight for it to answer.
  sock.close();
  await settle();

  // From here the socket holds its close back: restart() calls close() on it a
  // moment from now, and a real one would report back some time after that.
  sock.lateClose = true;
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();

  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=0");
  redial.open();
  await settle();
  // The shoulder, re-taken on the NEW socket — this is the command the stale
  // close must not touch.
  expect(redial.sent).toHaveLength(1);
  const perchReq = JSON.parse(redial.sent[0]!).requestId as string;
  // The replay this connection promised, so the shoulder has a name again.
  seedTable(redial);
  await settle();

  // NOW the socket abandoned two steps ago finally reports its close.
  sock.fireClose();
  await settle();

  // The server accepted the perch and answers on the socket the command was
  // actually written to. That command must still be waiting for it.
  redial.deliver({ result: { requestId: perchReq, ok: true } });
  await settle();
  // THE TOAST IS THE VISIBLE SYMPTOM, and the indicator is not: `viewpoint`
  // holds the shoulder the server last CONFIRMED, and the confirmation came
  // before the drop, so a refusal here leaves the label reading "Perched on:
  // Lera" either way — measured, not assumed. What a watcher actually sees is
  // a refusal of a command the server accepted, sitting under a board that
  // went on updating.
  expect(r.textContent).not.toContain("refused:");

  // AND THE FLAG THAT DECIDES THE NEXT REDIAL. after=0 is the restart path; a
  // resume would ask after=4, and it would be asking for events sitting on top
  // of frames only a perch ever produced.
  redial.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();
  expect(FakeSocket.instances).toHaveLength(3);
  expect(FakeSocket.instances[2]!.url).toContain("after=0");
  s?.close();
});

test("perch frames that outran a result the drop swallowed still start the next redial over", async () => {
  // THE ROUTE NO IDENTITY GUARD CAN CLOSE, and the sibling above is exactly the
  // one it is not: THAT socket was abandoned and reported its close late, so a
  // guard on identity keeps its news to itself. THIS socket is the live one,
  // and it genuinely dies with a command still on it.
  //
  // The gateway does not order a CommandResult against the envelopes the same
  // command produced (wire.ts's header note), and for a perch the two are not
  // even written by the same goroutine: the PUMP computes and enqueues the
  // perch's frames (internal/gateway/server.go's `case <-perches.wake` arm)
  // while the READ LOOP enqueues the result. So the pump wins, the sequence-0
  // frames land and are folded, and then the connection drops before the result
  // gets out. wire.ts answers the waiting command "connection closed before a
  // result arrived" — a refusal for a perch the server granted and has already
  // acted on.
  //
  // A flag read off THAT answer says false over a log that holds the perch's
  // whole board, and everything follows: the next redial resumes; rollback
  // keeps everything at or below its cursor and perch frames carry sequence 0,
  // so they survive it; the reborn projector, perched on nobody, sends this
  // seat nothing; and the next accepted perch re-introduces the scene, which
  // fold.ts refuses. session.ts re-folds the whole log on every event, so the
  // fatal panel never leaves. The same end state as the sibling above, reached
  // with one socket and no staging.
  //
  // AND IT NEEDS NO RACE AT ALL by the second door: server.go drops a result it
  // cannot encode (`if err != nil { continue }`) and the connection lives on,
  // leaving the promise unresolved over a log the perch has already shaped.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();

  // THE PUMP WINS. Sequence 0 is what makes these a perch's frames and nothing
  // else's: every other synthesized envelope carries the sequence of the event
  // that caused it, and real sequences start at 1 (project.go's perchSequence).
  sock.deliver(envelope(0, { sceneCreated: { sceneId: "s2", name: "Ambush", gridWidth: 4, gridHeight: 4 } }));
  await settle();
  expect(s!.state.Scenes["s2"]).toBeDefined();

  // ...and the socket dies with the result still behind it.
  sock.close();
  await settle();

  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();
  const redial = FakeSocket.instances[1]!;
  // after=4 is the resume, and it would be asking for events to sit on top of a
  // scene only that perch ever introduced.
  expect(redial.url).toContain("after=0");
  redial.open();
  await settle();
  expect(s!.events).toHaveLength(0);
  expect(s!.state.Scenes["s2"]).toBeUndefined();
  // NO shoulder is re-taken, and that is right rather than a shortfall:
  // `viewpoint` holds what the server CONFIRMED, nothing ever confirmed this
  // one, and the indicator must not claim a shoulder on the strength of a
  // refusal. The watcher lands perched on nobody over a clean log with the
  // perch control in front of them — which works, where the freeze did not.
  expect(redial.sent).toHaveLength(0);

  // AND THE FRESH LOG IS UNSHAPED AGAIN, so the redial after this one resumes.
  // The witness is read off the log this client is holding, so emptying that
  // log answers it; one that latched instead would replay the whole campaign on
  // every reconnect for the rest of the session.
  seedTable(redial);
  await settle();
  redial.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();
  expect(FakeSocket.instances[2]!.url).toContain("after=4");
  s?.close();
});

test("'no shoulder' reaches the wire, and is not quietly skipped", async () => {
  // THE THIRD BEHAVIOUR, and the plan calls it the one most likely to be got
  // wrong. Everything either side of this seam is pinned — the builder makes a
  // real command for "" (commands.test.ts), the control calls back with ""
  // (spectator-view.test.ts) — and the app is where an `if (id === "") return`
  // would hide, leaving a watcher with no way to stop seeing and every test
  // still green.
  const { r, s, sock } = await tableAs("spectator");
  (r.querySelector(".perch .unperch") as HTMLButtonElement).click();
  await settle();

  expect(sock.sent).toHaveLength(1);
  const cmd = JSON.parse(sock.sent[0]!);
  expect(cmd).toHaveProperty("setViewpoint");
  // protojson omits the empty scalar, so the ARM is what carries the meaning.
  expect(cmd.setViewpoint).not.toHaveProperty("actorId");
  s?.close();
});

test("a spectator who has never perched resumes like anybody else", async () => {
  // The condition is about the LOG, not about the role. A watcher who has
  // perched on nobody holds only what the replay would send them again, so
  // there is nothing for a restart to repair and a full re-replay of the
  // campaign would be a cost for nothing.
  const { r, s, sock } = await tableAs("spectator");
  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();

  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=4");
  expect(s!.events.length).toBeGreaterThan(0); // the log is KEPT
  redial.open();
  await settle();
  // NOT setViewpoint(""), which would be a command with nothing to do: a fresh
  // connection is already perched on nobody.
  expect(redial.sent).toHaveLength(0);
  s?.close();
});

test("a REFUSED perch does not shape the log, so that watcher still resumes", async () => {
  // The flag follows the SERVER's answer, exactly as the indicator beside it
  // does — and for a sharper reason. A refused perch put nothing in this log:
  // the server sent no frames, so there is nothing a restart could repair, and
  // a client that shaped its flag on the CLICK would replay the whole campaign
  // on every reconnect for the rest of the session, paying the full re-fold
  // cost for a perch it was never granted.
  //
  // The rest of this family says which QUESTION the flag asks ("has a perch
  // shaped this log", not "am I perched" or "am I a spectator"); this one says
  // what counts as an answer.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  sock.deliver({
    result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: false, error: "not a party member" },
  });
  await settle();
  expect(r.querySelector(".perch .perched-on")?.textContent).toBe("Perched on: nobody");

  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();

  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=4");
  expect(s!.events.length).toBeGreaterThan(0); // the log is KEPT
  redial.open();
  await settle();
  // And no shoulder is re-taken, because none was ever given.
  expect(redial.sent).toHaveLength(0);
  s?.close();
});

test("a spectator who hopped OFF a shoulder still starts over", async () => {
  // "Am I perched RIGHT NOW" is the wrong question, and this is the case that
  // proves it: nothing on the wire un-introduces a scene or an actor, so a
  // watcher who perched and then hopped off is still holding everything the
  // perch put in their log. Un-perching hides tokens; it does not un-say them.
  const { r, s, sock } = await tableAs("spectator");
  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: true } });
  await settle();
  (r.querySelector(".perch .unperch") as HTMLButtonElement).click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[1]!).requestId as string, ok: true } });
  await settle();

  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();
  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=0");
  redial.open();
  await settle();
  expect(redial.sent).toHaveLength(0); // no shoulder to re-take

  // AND THE FRESH LOG IS NO LONGER PERCH-SHAPED, so the redial after THIS one
  // resumes again. A flag left standing would make every later reconnect
  // replay the whole campaign.
  seedTable(redial);
  await settle();
  redial.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();
  expect(FakeSocket.instances[2]!.url).toContain("after=4");
  s?.close();
});

test("hopping off a shoulder nobody ever took leaves the log unshaped", async () => {
  // The EMPTY id is the other half of what shapes a log, and it shapes
  // nothing. setViewpoint("") is a real command with a real effect — it is
  // never skipped, which the "'no shoulder' reaches the wire" test above pins
  // — but the effect is to stop seeing, and stopping seeing introduces no
  // scene, no actor and no token. A watcher who has only ever pressed this
  // button holds precisely what the replay would hand them again.
  //
  // Distinct from "a spectator who hopped OFF a shoulder still starts over"
  // just above, and the pair is the whole point: THAT one perched first, so
  // its log carries frames nothing on the wire un-says. This one never did.
  // A flag set by any accepted set_viewpoint, rather than by an accepted
  // SHOULDER, cannot tell the two apart and makes every later reconnect replay
  // the campaign.
  const { r, s, sock } = await tableAs("spectator");
  (r.querySelector(".perch .unperch") as HTMLButtonElement).click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: true } });
  await settle();

  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();

  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=4");
  expect(s!.events.length).toBeGreaterThan(0); // the log is KEPT
  redial.open();
  await settle();
  expect(redial.sent).toHaveLength(0);
  s?.close();
});

test("a spectator PROMOTED to player still starts over, because their log is perch-shaped", async () => {
  // "Am I a spectator" is the other wrong question, and a role is not fixed:
  // app.ts re-reads /api/me on a presence frame naming this participant, so a
  // promotion lands mid-session. Their log still holds the frames their perch
  // introduced, while their new seat's projector has never sent them any of
  // it — and the first thing it introduces above the resume cursor that they
  // already hold is the duplicate-introduction freeze.
  localStorage.setItem("vtt.token", "tok");
  let role = "spectator";
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    const table: Record<string, unknown> = {
      "/api/me": { participantId: "p", name: "Watcher", role },
      "/api/ruleset": { id: "r", name: "R", abilities: [], conditions: [], resources: [] },
      "/api/adventures": { adventures: [] },
    };
    if (!(path in table)) return new Response("", { status: 404 });
    return new Response(JSON.stringify(table[path]), {
      status: 200, headers: { "content-type": "application/json" },
    });
  }) as typeof fetch;
  useFakeSocket();
  const r = root();
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  seedTable(sock);
  await settle();

  const asme = Array.from(r.querySelectorAll(".perch .shoulder"))
    .find((b) => b.textContent === "Lera") as HTMLButtonElement;
  asme.click();
  await settle();
  sock.deliver({ result: { requestId: JSON.parse(sock.sent[0]!).requestId as string, ok: true } });
  await settle();

  // The DM promotes them; the server re-announces, and app.ts re-reads.
  role = "player";
  sock.deliver({ presenceChanged: { participantId: "p", displayName: "Watcher", state: "PRESENCE_STATE_CONNECTED" } });
  await settle();
  expect(r.querySelector(".perch")).toBeNull(); // the control is gone, as it must be

  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();
  expect(FakeSocket.instances[1]!.url).toContain("after=0");
  FakeSocket.instances[1]!.open();
  await settle();
  expect(s!.events).toHaveLength(0);
  // AND THE SHOULDER IS NOT RE-TAKEN, because this seat may no longer have one.
  // MayPerch refuses every role but the spectator's ("role %q does not perch —
  // a viewpoint is the spectator's"), so a redial that re-sends the last
  // confirmed shoulder regardless of role greets a promoted watcher with a
  // refusal toast for a command they never issued — over a board that is
  // already correct, since a player's own eyes are what the replay just rebuilt.
  // The restart above is still right (their log holds perch frames); the
  // re-perch is what their new role has no use for.
  expect(FakeSocket.instances[1]!.sent).toHaveLength(0);
  s?.close();
});

test("a player's reconnect still resumes, and sends no perch", async () => {
  // The other half of the branch. A player's projector is rebuilt from their
  // own fixed eyes by the replay, so their resume is correct and a full
  // re-replay of the campaign would be a cost for nothing.
  const { r, s, sock } = await tableAs("player");
  sock.close();
  await settle();
  (r.querySelector(".reconnect") as HTMLButtonElement).click();
  await settle();

  const redial = FakeSocket.instances[1]!;
  expect(redial.url).toContain("after=4");
  expect(s!.events.length).toBeGreaterThan(0); // the log is KEPT
  redial.open();
  await settle();
  expect(redial.sent).toHaveLength(0);
  s?.close();
});
