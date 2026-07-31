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

test("a freshly pasted invite is IGNORED while an old token is stored", () => {
  // KNOWN GAP, pinned deliberately rather than fixed here.
  //
  // app.ts:31 is `auth.get() ?? <url token>`, so the stored token wins and
  // the pasted one is discarded — never stored, never dialled. A re-invited
  // player therefore stays connected as their OLD identity, and the only way
  // out is clearing site data.
  //
  // The previous version of this test asserted `toBeTruthy()` on the stored
  // key, which its own setItem on the line above already satisfied: it passed
  // for tok-old, for tok-new, and for boot() doing nothing whatsoever. It
  // described the hazard in a comment while being unable to detect it.
  localStorage.setItem("vtt.token", "tok-old");
  setURL("http://localhost/?token=tok-new");
  useFakeSocket();

  const session = boot(root());
  expect(localStorage.getItem("vtt.token")).toBe("tok-old");
  expect(FakeSocket.instances[0]!.url).toContain("token=tok-old");
  expect(FakeSocket.instances[0]!.url).not.toContain("tok-new");
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
