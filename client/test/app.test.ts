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
