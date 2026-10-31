import "./support/dom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { boot } from "../src/app";
import { CREDENTIAL_GONE } from "../src/wire";
import credentialClose from "../../contract/testdata/credential_close.json";

type CloseEv = { code: number; reason: string };
class FakeSocket {
  static OPEN = 1;
  static all: FakeSocket[] = [];
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  onclose: ((e: CloseEv) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  constructor(public url: string) {
    FakeSocket.all.push(this);
  }
  send(_d: string) {}
  close() {}
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  frame(f: unknown) {
    this.onmessage?.({ data: JSON.stringify(f) });
  }
  closeWith(code: number, reason: string) {
    this.readyState = 3;
    this.onclose?.({ code, reason });
  }
}

const g = globalThis as unknown as {
  WebSocket: unknown;
  fetch: unknown;
  happyDOM: { setURL(u: string): void };
};
const realWS = g.WebSocket;
const realFetch = g.fetch;

let meStatus = 200;
function stubFetch() {
  g.fetch = async (url: string | URL) => {
    const u = String(url);
    if (u.includes("/api/me")) {
      if (meStatus !== 200) return new Response("x", { status: meStatus });
      return new Response(JSON.stringify({ id: "p1", name: "Pia", role: "player" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    }
    return new Response("[]", { status: 200, headers: { "content-type": "application/json" } });
  };
}
async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

let root: HTMLElement;
beforeEach(() => {
  FakeSocket.all = [];
  meStatus = 200;
  g.WebSocket = FakeSocket;
  stubFetch();
  root = document.createElement("div");
  document.body.appendChild(root);
});
afterEach(() => {
  g.WebSocket = realWS;
  g.fetch = realFetch;
  localStorage.clear();
  sessionStorage.clear();
  root.remove();
  document.body.innerHTML = "";
});

const MINE = "tok-mine";
const OTHER = "tok-other";
const VP = (t: string) => `vtt.viewpoint.${t}`;

async function openWindow(token = MINE, withViewpoint = true) {
  g.happyDOM.setURL(`http://localhost:8080/?token=${token}`);
  boot(root);
  await settle();
  const ws = FakeSocket.all[FakeSocket.all.length - 1]!;
  ws.open();
  ws.frame({ catchUpHead: { headSequence: "0" } });
  if (withViewpoint) ws.frame({ viewpointChanged: { actorId: "a1" } });
  await settle();
  return ws;
}
const removedShown = () => /removed/i.test(root.textContent ?? "");
const reconnectShown = () => root.querySelector(".reconnect") !== null;

describe("the fixture", () => {
  test("the agreed close is the one the client names", () => {
    // VTT-572
    expect(credentialClose.code).toBe(1008);
    expect(credentialClose.reason).toBe("gateway: credential no longer valid");
    expect<number>(CREDENTIAL_GONE.code).toBe(credentialClose.code);
    expect<string>(CREDENTIAL_GONE.reason).toBe(credentialClose.reason);
  });
});

describe("a window open when its participant is removed", () => {
  test("the agreed close shows removal in place of Reconnect, forgets own token and viewpoint", async () => {
    // VTT-572 VTT-574 VTT-575
    sessionStorage.setItem(VP(OTHER), "keep-me");
    const ws = await openWindow();
    expect(localStorage.getItem("vtt.token")).toBe(MINE);
    expect(sessionStorage.getItem(VP(MINE))).toBe("a1");
    ws.closeWith(credentialClose.code, credentialClose.reason);
    await settle();
    expect(removedShown()).toBe(true);
    expect(reconnectShown()).toBe(false);
    expect(localStorage.getItem("vtt.token")).toBeNull();
    expect(sessionStorage.getItem(VP(MINE))).toBeNull();
    expect(sessionStorage.getItem(VP(OTHER))).toBe("keep-me");
  });

  test("another window's token in the browser is kept, this window's viewpoint is still forgotten", async () => {
    // VTT-574 VTT-575
    const ws = await openWindow();
    localStorage.setItem("vtt.token", OTHER);
    ws.closeWith(credentialClose.code, credentialClose.reason);
    await settle();
    expect(removedShown()).toBe(true);
    expect(localStorage.getItem("vtt.token")).toBe(OTHER);
    expect(sessionStorage.getItem(VP(MINE))).toBeNull();
  });

  test("a viewpoint that named nobody is forgotten too", async () => {
    // VTT-575
    const ws = await openWindow(MINE, false);
    ws.frame({ viewpointChanged: { actorId: "" } });
    await settle();
    expect(sessionStorage.getItem(VP(MINE))).toBe("");
    ws.closeWith(credentialClose.code, credentialClose.reason);
    await settle();
    expect(sessionStorage.getItem(VP(MINE))).toBeNull();
  });

  test("forgetting holds with no viewpoint ever received", async () => {
    // VTT-572 VTT-574
    const ws = await openWindow(MINE, false);
    ws.closeWith(credentialClose.code, credentialClose.reason);
    await settle();
    expect(removedShown()).toBe(true);
    expect(localStorage.getItem("vtt.token")).toBeNull();
  });

  test("after forgetting, the share link shows the join form", async () => {
    // VTT-577
    const ws = await openWindow();
    ws.closeWith(credentialClose.code, credentialClose.reason);
    await settle();
    expect(localStorage.getItem("vtt.token")).toBeNull();
    root.innerHTML = "";
    g.happyDOM.setURL("http://localhost:8080/?join=sekret");
    boot(root);
    await settle();
    expect(root.querySelector('[data-field="join-name"]')).not.toBeNull();
  });
});

describe("a close for any other reason", () => {
  const cases: [string, number, string][] = [
    ["1008 with another reason", 1008, "gateway: something else"],
    ["1000 with the credential reason", 1000, credentialClose.reason],
    ["1006 with no reason", 1006, ""],
    ["1011 with the credential reason", 1011, credentialClose.reason],
    ["1008 with an empty reason", 1008, ""],
  ];
  for (const [name, code, reason] of cases) {
    test(`${name} forgets nothing and offers Reconnect`, async () => {
      // VTT-576
      const ws = await openWindow();
      ws.closeWith(code, reason);
      await settle();
      expect(removedShown()).toBe(false);
      expect(reconnectShown()).toBe(true);
      expect(localStorage.getItem("vtt.token")).toBe(MINE);
      expect(sessionStorage.getItem(VP(MINE))).toBe("a1");
    });
  }
});

describe("a window opened after its participant was removed", () => {
  test("a 401 on /api/me shows removal and forgets", async () => {
    // VTT-573 VTT-574 VTT-575
    meStatus = 401;
    sessionStorage.setItem(VP(MINE), "a1");
    sessionStorage.setItem(VP(OTHER), "keep-me");
    g.happyDOM.setURL(`http://localhost:8080/?token=${MINE}`);
    boot(root);
    await settle();
    for (const ws of FakeSocket.all) ws.open();
    await settle();
    expect(removedShown()).toBe(true);
    expect(reconnectShown()).toBe(false);
    expect(localStorage.getItem("vtt.token")).toBeNull();
    expect(sessionStorage.getItem(VP(MINE))).toBeNull();
    expect(sessionStorage.getItem(VP(OTHER))).toBe("keep-me");
  });

  test("a 401 keeps another window's stored token", async () => {
    // VTT-574
    meStatus = 401;
    g.happyDOM.setURL(`http://localhost:8080/`);
    // token comes from storage, then storage is replaced before the answer is read
    localStorage.setItem("vtt.token", MINE);
    boot(root);
    localStorage.setItem("vtt.token", OTHER);
    await settle();
    expect(removedShown()).toBe(true);
    expect(localStorage.getItem("vtt.token")).toBe(OTHER);
  });

  for (const status of [500, 503, 403, 404]) {
    test(`a ${status} on /api/me forgets nothing and shows no removal`, async () => {
      // VTT-580
      meStatus = status;
      sessionStorage.setItem(VP(MINE), "a1");
      g.happyDOM.setURL(`http://localhost:8080/?token=${MINE}`);
      boot(root);
      await settle();
      for (const ws of FakeSocket.all) ws.open();
      await settle();
      expect(removedShown()).toBe(false);
      expect(localStorage.getItem("vtt.token")).toBe(MINE);
      expect(sessionStorage.getItem(VP(MINE))).toBe("a1");
    });
  }

  test("a network failure on /api/me forgets nothing and shows no removal", async () => {
    // VTT-580
    g.fetch = async () => {
      throw new TypeError("network down");
    };
    sessionStorage.setItem(VP(MINE), "a1");
    g.happyDOM.setURL(`http://localhost:8080/?token=${MINE}`);
    boot(root);
    await settle();
    expect(removedShown()).toBe(false);
    expect(localStorage.getItem("vtt.token")).toBe(MINE);
    expect(sessionStorage.getItem(VP(MINE))).toBe("a1");
  });
});
