import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { boot } from "../src/app";

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
  const original = history.replaceState.bind(history);
  history.replaceState = ((...args: unknown[]) => {
    calls.push(String(args[2]));
    return original(args[0] as never, args[1] as string, args[2] as string);
  }) as typeof history.replaceState;

  const session = boot(root());
  expect(localStorage.getItem("vtt.token")).toBe("tok-abc");
  expect(calls).toHaveLength(1);
  expect(calls[0]).not.toContain("token");
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
  stubMetadata({ "/api/me": { participantId: "p", name: "P", role: "spectator", controls: [] } });
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
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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
    "/api/me": { participantId: "p", name: "S", role: "spectator", controls: [] },
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
    "/api/me": { participantId: "p", name: "Lera", role: "player", controls: ["a1"] },
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
    "/api/me": { participantId: "p", name: "Agent", role: "agent", controls: [] },
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
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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

test("delivered events reach both the feed and the DM console's log", async () => {
  // Two separate `[...session.events]` spreads feed those two views. An empty
  // array in either place renders a plausible, silent, wrong page: a board
  // with no story beside it.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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
  // The console's half, which needs its OWN assertion. The DM console does not
  // render the log as prose — it uses it for lastUndoable and
  // retractableRange, so an emptied `log` shows up as a dead undo affordance
  // and nothing else. Asserting story text alone reached only the feed.
  expect(byText(r, "Undo #2")).toBeDefined();
  s?.close();
});

test("a DM gets the player panel as well as the console", async () => {
  // canAct lists player, dm and agent separately. Each arm needs its own
  // case or one role silently loses the ability to act while still looking
  // fully wired.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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
      return Response.json({ participantId: "p", name: "DM", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p", name: "DM", role: "dm", controls: [] });
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

/** A minimal live table: a scene, one actor the caller controls, and its token. */
function seedTable(sock: FakeSocket, actorId = "a1", participantId = "p") {
  sock.deliver(envelope(1, { sessionStarted: { name: "Night" } }));
  sock.deliver(envelope(2, { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 8, gridHeight: 8 } }));
  sock.deliver(envelope(3, { actorAdded: { actor: { actorId, name: "Lera", controllerId: participantId } } }));
  sock.deliver(envelope(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId, position: { x: 1, y: 1 } } }));
}

test("the ruleset's abilities reach the player panel, and nothing else does", async () => {
  // With an actor on the board the panel actually renders its ability list,
  // which is the only place the `abilities` variable is observable. Without a
  // controlled actor the panel short-circuits and the list is never read —
  // which is why an earlier version of this test proved nothing.
  localStorage.setItem("vtt.token", "tok");
  stubMetadata({
    "/api/me": { participantId: "p", name: "Lera", role: "player", controls: ["a1"] },
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
  stubMetadata({ "/api/me": { participantId: "p", name: "Lera", role: "player", controls: ["a1"] } });
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
    "/api/me": { participantId: "p", name: "Lera", role: "player", controls: ["a1"] },
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
    "/api/me": { participantId: "p", name: "Watcher", role: "spectator", controls: [] },
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

/** A DM at a live table, with the metadata routes served from `routes`. */
async function dmTable(routes: Record<string, unknown>) {
  localStorage.setItem("vtt.token", "tok-dm");
  stubMetadata({
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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

test("undo asks for confirmation, and sends the retraction when granted", async () => {
  // window.confirm is deliberate for a destructive action. Both halves are
  // pinned: the dialog is consulted, and a granted one actually sends.
  const realConfirm = window.confirm;
  try {
    let asked = "";
    window.confirm = ((m: string) => { asked = m; return true; }) as typeof window.confirm;
    const { r, s, sock } = await dmTable({});
    byText(r, "Undo #1")!.click();
    await settle();
    expect(asked).toContain("Retract event #1");
    expect(sock.sent.length).toBe(1);
    expect(JSON.parse(sock.sent[0]!)).toHaveProperty("retractEvents");
    s?.close();
  } finally {
    window.confirm = realConfirm;
  }
});

test("a declined confirmation sends nothing at all", async () => {
  const realConfirm = window.confirm;
  try {
    window.confirm = (() => false) as typeof window.confirm;
    const { r, s, sock } = await dmTable({});
    byText(r, "Undo #1")!.click();
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
    "/api/me": { participantId: "p", name: "Lera", role: "player", controls: ["a1"] },
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
  sock.deliver(envelope(3, { actorAdded: { actor: { actorId: "a1", name: "Lera", controllerId: "p" } } }));
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
    "/api/me": { participantId: "p-dm", name: "DM", role: "dm", controls: [] },
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
  const original = history.replaceState.bind(history);
  history.replaceState = ((...args: unknown[]) => {
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p-me", name: "Robin", role, controls: ["a1"] });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { actorAdded: { actor: { actorId: "a1", name: "Ash", controllerIds: ["p-me"] } } }));
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
      return Response.json({ participantId: "p-1", name: "Lera", role: "player", controls: [] });
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p-a", name: "Aide", role: "agent", controls: [] });
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
      return Response.json({ participantId: "p-me", name: "Robin", role, controls: ["a1"] });
    }
    return new Response("", { status: 404 });
  }) as typeof fetch;

  const r = root();
  const session = boot(r);
  await settle();
  const sock = FakeSocket.instances[0]!;
  sock.open();
  sock.deliver(envelope(1, { actorAdded: { actor: { actorId: "a1", name: "Ash", controllerIds: ["p-me"] } } }));
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
      return Response.json({ participantId: "p-dm", name: "Ari", role: "dm", controls: [] });
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
