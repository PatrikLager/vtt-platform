import "./support/dom";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { boot } from "../src/app";
import { newState, type State } from "../src/state";
import { renderSpectator } from "../src/view/spectator";

const realFetch = globalThis.fetch;
const realWS = (globalThis as any).WebSocket;

let sockets: FakeSocket[] = [];
let opened: ({ close(): void } | null)[] = [];

class FakeSocket {
  static OPEN = 1;
  readyState = 1;
  onopen: ((e: any) => void) | null = null;
  onmessage: ((e: any) => void) | null = null;
  onclose: ((e: any) => void) | null = null;
  onerror: ((e: any) => void) | null = null;
  listeners: Record<string, ((e: any) => void)[]> = {};
  sent: string[] = [];
  constructor(public url: string) {
    sockets.push(this);
    queueMicrotask(() => this.fire("open", {}));
  }
  addEventListener(t: string, f: (e: any) => void) {
    (this.listeners[t] ??= []).push(f);
  }
  removeEventListener() {}
  send(d: string) {
    this.sent.push(d);
  }
  close() {}
  fire(t: string, e: any) {
    (this as any)["on" + t]?.(e);
    for (const f of this.listeners[t] ?? []) f(e);
  }
  push(o: unknown) {
    this.fire("message", { data: JSON.stringify(o) });
  }
}

type Role = "dm" | "agent" | "player" | "spectator" | "never" | "other";

function installFetch(role: Role) {
  (globalThis as any).fetch = async (url: any) => {
    const u = String(url);
    const ok = (body: unknown) => ({
      ok: true,
      status: 200,
      json: async () => body,
      text: async () => JSON.stringify(body),
    });
    if (u.includes("/api/me")) {
      if (role === "never") return { ok: false, status: 500, json: async () => ({}), text: async () => "" };
      return ok({ participantId: "p1", name: "Pat", role: role === "other" ? "gremlin" : role });
    }
    if (u.includes("/api/ruleset")) return ok({ abilities: [], conditions: [], properties: [] });
    if (u.includes("/api/adventures")) return ok([]);
    return ok([]);
  };
}

let seq = 0;
function env(payload: Record<string, unknown>) {
  seq++;
  return {
    event: {
      eventId: `e${seq}`,
      sequence: String(seq),
      sessionId: "s",
      actorRole: "ACTOR_ROLE_DM",
      participantId: "p1",
      ...payload,
    },
  };
}

const flush = async () => {
  for (let i = 0; i < 20; i++) await new Promise((r) => setTimeout(r, 0));
};

interface Disc {
  resources?: Record<string, { current: number; max: number }>;
  conditions?: string[];
}

async function drive(role: Role, disc: Disc) {
  seq = 0;
  sockets = [];
  installFetch(role);
  localStorage.setItem("vtt.token", "tok");
  const root = document.createElement("div");
  root.id = "app";
  document.body.appendChild(root);
  (globalThis as any).WebSocket = FakeSocket;
  opened.push(boot(root));
  await flush();
  const s = sockets[0]!;
  if (role === "player" || role === "spectator") {
    s.push({ viewpointChanged: { actorId: "a1" } });
    s.push({ watchableChanged: { actors: [{ actorId: "a1", name: "Ann" }] } });
  }
  s.push(env({ sessionStarted: {} }));
  s.push(env({ sceneCreated: { sceneId: "sc1", name: "Hall", gridWidth: 6, gridHeight: 6 } }));
  s.push(env({ actorAdded: { actor: { actorId: "a1", name: "Ann", resources: disc.resources ?? {} } } }));
  s.push(env({ tokenPlaced: { tokenId: "t1", sceneId: "sc1", actorId: "a1", position: { x: 1, y: 1 } } }));
  for (const c of disc.conditions ?? []) s.push(env({ conditionApplied: { actorId: "a1", conditionId: c } }));
  await flush();
  const tok = root.querySelector('[data-token-id="t1"]');
  return { root, tok };
}

const chipsOf = (tok: Element | null) => Array.from(tok?.querySelectorAll(".chip") ?? []).map((e) => e.textContent ?? "");
const dotsOf = (tok: Element | null) => Array.from(tok?.querySelectorAll(".dot") ?? []).map((e) => e.getAttribute("title"));

beforeEach(() => {
  try {
    sessionStorage.clear();
    localStorage.clear();
  } catch {}
});
afterEach(() => {
  for (const o of opened) o?.close();
  opened = [];
  (globalThis as any).fetch = realFetch;
  (globalThis as any).WebSocket = realWS;
  document.body.innerHTML = "";
});

const TWO = { hp: { current: 3, max: 9 }, mana: { current: 1, max: 4 } };

describe("whole window: resource chips by role", () => {
  // VTT-834
  for (const role of ["dm", "agent"] as const) {
    test(`${role} window draws one chip per resource`, async () => {
      const { tok } = await drive(role, { resources: TWO });
      expect(tok).not.toBeNull();
      const chips = chipsOf(tok);
      expect(chips.length).toBe(2);
      expect(chips.join("|")).toContain("3/9");
      expect(chips.join("|")).toContain("1/4");
    });
  }

  // VTT-834
  for (const role of ["player", "spectator", "never", "other"] as const) {
    test(`${role} window draws no chip`, async () => {
      const { tok } = await drive(role, { resources: TWO });
      expect(tok).not.toBeNull();
      expect(chipsOf(tok)).toEqual([]);
    });
  }

  // VTT-834
  test("a DM disc with a single resource draws exactly one chip", async () => {
    const { tok } = await drive("dm", { resources: { hp: { current: 5, max: 5 } } });
    expect(chipsOf(tok).length).toBe(1);
  });

  // VTT-834
  test("a DM disc whose actor holds no resource draws none", async () => {
    const { tok } = await drive("dm", {});
    expect(tok).not.toBeNull();
    expect(chipsOf(tok)).toEqual([]);
  });
});

describe("whole window: condition dots by role", () => {
  const CONDS = ["poisoned", "blinded", "afraid"];
  // VTT-835
  for (const role of ["dm", "agent", "player", "spectator", "never", "other"] as const) {
    test(`${role} window draws the dots in the order applied`, async () => {
      const { tok } = await drive(role, { resources: TWO, conditions: CONDS });
      expect(tok).not.toBeNull();
      expect(dotsOf(tok)).toEqual(CONDS);
    });
  }

  // VTT-835
  test("applied order is not alphabetical order", async () => {
    const { tok } = await drive("player", { conditions: ["zzz", "aaa"] });
    expect(dotsOf(tok)).toEqual(["zzz", "aaa"]);
  });

  // VTT-835
  test("one condition draws one dot", async () => {
    const { tok } = await drive("spectator", { conditions: ["prone"] });
    expect(dotsOf(tok)).toEqual(["prone"]);
  });

  // VTT-835
  test("no condition draws no dot", async () => {
    const { tok } = await drive("dm", { resources: TWO });
    expect(dotsOf(tok)).toEqual([]);
  });
});

describe("renderSpectator seam: extras.dm", () => {
  function world(): State {
    const st = newState();
    st.Scenes["sc1"] = { ID: "sc1", Name: "Hall", GridWidth: 6, GridHeight: 6 } as any;
    st.Actors["a1"] = {
      actorId: "a1",
      name: "Ann",
      moduleId: "",
      attributes: {},
      resources: TWO,
      controllerId: "",
      controllerIds: [],
      kind: "creature",
      abilityIds: [],
    } as any;
    st.Tokens["t1"] = { ID: "t1", SceneID: "sc1", ActorID: "a1", X: 1, Y: 1 };
    st.Conditions["a1"] = [
      { ID: "poisoned", Source: "x", AppliedSeq: 5 },
      { ID: "afraid", Source: "x", AppliedSeq: 9 },
    ];
    return st;
  }
  function paint(extras: any) {
    const root = document.createElement("div");
    document.body.appendChild(root);
    renderSpectator(root, world(), [], "ok", extras);
    return root.querySelector('[data-token-id="t1"]');
  }

  // VTT-834
  test("dm true draws two chips", () => {
    expect(chipsOf(paint({ dm: true })).length).toBe(2);
  });
  // VTT-834
  test("dm false draws none", () => {
    expect(chipsOf(paint({ dm: false }))).toEqual([]);
  });
  // VTT-834
  test("dm left out draws none", () => {
    expect(chipsOf(paint({}))).toEqual([]);
  });
  // VTT-835
  test("dots appear for dm true, false and unset, in order applied", () => {
    for (const x of [{ dm: true }, { dm: false }, {}]) {
      expect(dotsOf(paint(x))).toEqual(["poisoned", "afraid"]);
    }
  });
});
