import "./support/dom";
import { test, expect, afterEach } from "bun:test";
import { boot } from "../src/app";
import { renderPlayerPanel, type PanelLayout, type PlayerUIState } from "../src/view/player";
import { newState, type State } from "../src/state";
import type { Ability, Me, Property } from "../src/metadata";

const me: Me = { participantId: "p", name: "Lera", role: "player" };
const props: Property[] = [{ key: "mana", type: "resource", name: "Mana", description: "" }];

function stateWith(abilityIds: string[], resources: Record<string, { current: number; max: number }> = {}): State {
  const st = newState();
  st.Actors["a1"] = {
    actorId: "a1", name: "Lera", moduleId: "", attributes: {}, resources,
    controllerId: "p", controllerIds: ["p"], kind: "ACTOR_KIND_PARTY_MEMBER" as never, abilityIds,
  };
  return st;
}
const ui = (): PlayerUIState => ({ selectedActorId: "a1", selectedAbilityId: "", doorsArmed: false });
const free = (id: string, name: string, description?: string): Ability => ({
  id, name, range: 1, maxTargets: 1, usage: { kind: "atWill" },
  ...(description === undefined ? {} : { description }),
});
const spell = (id: string, name: string, cost: number, description?: string): Ability => ({
  id, name, range: 1, maxTargets: 1, usage: { kind: "resource", resource: "mana", cost },
  ...(description === undefined ? {} : { description }),
});

function buttons(a: Ability[], layout?: PanelLayout, res: Record<string, { current: number; max: number }> = { mana: { current: 5, max: 10 } }, role: Me["role"] = "player") {
  const st = stateWith(a.map((x) => x.id), res);
  const el = renderPlayerPanel(st, { ...me, role }, a, ui(), async () => ({ ok: true }) as never, () => {}, undefined, undefined, props, layout);
  return Array.from(el.querySelectorAll("button"));
}
function titleOf(a: Ability, layout?: PanelLayout, res?: Record<string, { current: number; max: number }>, role?: Me["role"]) {
  const b = buttons([a], layout, res, role).find((x) => x.textContent === (a.name || a.id));
  expect(b).toBeDefined();
  return b!.getAttribute("title");
}

const T = (a: Ability, layout?: PanelLayout, res?: Record<string, { current: number; max: number }>, role?: Me["role"]) =>
  titleOf(a, layout, res, role) ?? "";
const named: PanelLayout = { freeUsageName: "At will" };

// VTT-854, VTT-844
test("a resource ability's title is its description, a line break, then the cost line", () => {
  expect(T(spell("s", "Spark", 2, "Zap"))).toBe("Zap\nMana 5/10, costs 2");
});

// VTT-854, VTT-844
test("an unaffordable resource ability is still titled, its cost line ending — not enough", () => {
  expect(T(spell("s", "Spark", 2, "Zap"), undefined, { mana: { current: 1, max: 10 } })).toBe(
    "Zap\nMana 1/10, costs 2 \u2014 not enough",
  );
});

// VTT-854
test("a resource ability with no description is titled by its cost line alone", () => {
  expect(T(spell("s", "Spark", 2))).toBe("Mana 5/10, costs 2");
});

// VTT-854
test("a resource ability with an empty description is titled by its cost line alone", () => {
  expect(T(spell("s", "Spark", 2, ""))).toBe("Mana 5/10, costs 2");
});

// VTT-844, VTT-854
test("a resource the sheet does not hold names neither current nor max", () => {
  expect(T(spell("s", "Spark", 2, "Zap"), undefined, {})).toBe("Zap\nMana, costs 2 \u2014 not enough");
});

// VTT-855, VTT-854
test("an ability that spends no resource: description, a line break, the layout's name", () => {
  expect(T(free("p", "Poke", "Jab"), named)).toBe("Jab\nAt will");
});

// VTT-855
test("an ability that spends no resource and has no description is titled by the layout's name", () => {
  expect(T(free("p", "Poke"), named)).toBe("At will");
  expect(T(free("p", "Poke", ""), { freeUsageName: "Cantrip" })).toBe("Cantrip");
});

// VTT-855, VTT-854
test("with no free-usage name the title is the description alone", () => {
  expect(T(free("p", "Poke", "Jab"))).toBe("Jab");
  expect(T(free("p", "Poke", "Jab"), {})).toBe("Jab");
  expect(T(free("p", "Poke", "Jab"), { freeUsageName: undefined })).toBe("Jab");
  expect(T(free("p", "Poke", "Jab"), { freeUsageName: "" })).toBe("Jab");
});

// VTT-855, VTT-854
test("an ability with neither description nor cost line has an empty title, and never says at will", () => {
  expect(T(free("p", "Poke"))).toBe("");
  expect(T(free("p", "Poke", ""))).toBe("");
  expect(T(free("p", "Poke", ""), { freeUsageName: "" })).toBe("");
  expect(T(free("p", "Poke"), { sheet: [] })).not.toContain("at will");
});

// VTT-854
test("a description's own line breaks and unusual characters stand as written", () => {
  const d = "  first\n\nsecond\r\n<b>&amp;</b> \"q\" \u00e5\u00e4\u00f6 \u{1F4A5}\ttab  ";
  expect(T(free("p", "Poke", d), named)).toBe(d + "\nAt will");
  expect(T(spell("s", "Spark", 2, d))).toBe(d + "\nMana 5/10, costs 2");
});

// VTT-854
test("a description of only whitespace is a description as written", () => {
  expect(T(free("p", "Poke", " "), named)).toBe(" \nAt will");
});

// VTT-846
test("the label is the ability's name, else its id", () => {
  const a = buttons([free("p1", "Poke", "x"), free("p2", "", "y")], named).map((b) => b.textContent);
  expect(a).toContain("Poke");
  expect(a).toContain("p2");
  expect(a).not.toContain("");
});

// VTT-854
test("a DM's and an agent's panel titles its buttons the same way", () => {
  for (const role of ["dm", "agent"] as const) {
    expect(T(free("p", "Poke", "Jab"), named, undefined, role)).toBe("Jab\nAt will");
    expect(T(spell("s", "Spark", 2, "Zap"), undefined, undefined, role)).toBe("Zap\nMana 5/10, costs 2");
  }
});

type Frame = Record<string, unknown>;
class FakeSocket {
  static instances: FakeSocket[] = [];
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((ev: { code: number; reason: string }) => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(readonly url: string) { FakeSocket.instances.push(this); }
  open() { this.readyState = 1; this.onopen?.(); }
  deliver(f: Frame) { this.onmessage?.({ data: JSON.stringify(f) }); }
  send() {}
  close() { this.readyState = 3; this.onclose?.({ code: 1000, reason: "" }); }
}
const nativeFetch = globalThis.fetch;
const nativeWS = globalThis.WebSocket;
afterEach(() => { globalThis.fetch = nativeFetch; globalThis.WebSocket = nativeWS; });
async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}
const env = (seq: number, payload: Frame): Frame => ({
  event: { eventId: `e-${seq}`, sequence: String(seq), sessionId: "s", actorRole: "dm", participantId: "p-dm", ...payload },
});

function boots(abilityIds: string[]) {
  localStorage.clear(); sessionStorage.clear();
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL("http://localhost/");
  localStorage.setItem("vtt.token", "tok");
  const answers: ((body: Record<string, unknown>) => void)[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    const json = (o: unknown) => new Response(JSON.stringify(o), { status: 200, headers: { "content-type": "application/json" } });
    if (path === "/api/me") return json({ participantId: "p", name: "Lera", role: "player" });
    if (path === "/api/adventures") return json({ adventures: [] });
    if (path === "/api/ruleset") return new Promise<Response>((res) => answers.push((b) => res(json(b))));
    return new Response("", { status: 404 });
  }) as typeof fetch;
  FakeSocket.instances = [];
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  const r = document.createElement("div");
  r.id = "app";
  document.body.replaceChildren(r);
  const s = boot(r);
  const sock = FakeSocket.instances[0]!;
  sock.open();
  return { r, s, sock, answers, abilityIds };
}
const body = (extra: Record<string, unknown>) => ({
  id: "r", name: "R", conditions: [], resources: [],
  abilities: [{ id: "poke", name: "Poke", description: "Jab", range: 1, maxTargets: 1, usage: { kind: "atWill" } }],
  ...extra,
});
function seed(sock: FakeSocket, ids: string[], actors: { actorId: string; name: string; abilityIds?: string[] }[]) {
  sock.deliver({ watchableChanged: { actors } });
  sock.deliver({ viewpointChanged: { actorId: "a1" } });
  sock.deliver(env(1, { sessionStarted: { name: "N" } }));
  sock.deliver(env(2, { sceneCreated: { sceneId: "s1", name: "H", gridWidth: 8, gridHeight: 8 } }));
  sock.deliver(env(3, { actorAdded: { actor: { actorId: "a1", name: "Lera", abilityIds: ids } } }));
  sock.deliver(env(4, { actorControlGranted: { actorId: "a1", participantId: "p", kind: "ACTOR_KIND_PARTY_MEMBER" } }));
  sock.deliver(env(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } }));
}
const pokeTitle = (r: HTMLElement) =>
  Array.from(r.querySelectorAll(".player button")).find((b) => b.textContent === "Poke")?.getAttribute("title") ?? null;

const last = (a: unknown[]) => a.length - 1;

// VTT-855
test("a booted window titles a free ability by the name its newest answer gives", async () => {
  const { r, s, sock, answers } = boots(["poke"]);
  await settle();
  seed(sock, ["poke"], [{ actorId: "a1", name: "Lera", abilityIds: ["poke"] }]);
  await settle();
  answers[last(answers)]!(body({ freeUsageName: "Anytime" }));
  await settle();
  expect(pokeTitle(r)).toBe("Jab\nAnytime");
  s?.close();
});

// VTT-855
test("an older answer arriving after the newest changes nothing, and an answer without the name reads as none", async () => {
  const { r, s, sock, answers } = boots(["poke"]);
  await settle();
  seed(sock, ["poke"], [{ actorId: "a1", name: "Lera", abilityIds: ["poke"] }]);
  await settle();
  const first = answers.length;
  sock.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Lera", abilityIds: ["poke", "other"] }] } });
  await settle();
  expect(answers.length).toBeGreaterThan(first);
  answers[last(answers)]!(body({ freeUsageName: "Newest" }));
  await settle();
  expect(pokeTitle(r)).toBe("Jab\nNewest");
  for (let i = 0; i < last(answers); i++) answers[i]!(body({ freeUsageName: "Older" }));
  await settle();
  expect(pokeTitle(r)).toBe("Jab\nNewest");
  const before = answers.length;
  sock.deliver({ watchableChanged: { actors: [{ actorId: "a1", name: "Lera", abilityIds: ["poke"] }] } });
  await settle();
  expect(answers.length).toBeGreaterThan(before);
  answers[last(answers)]!(body({}));
  await settle();
  expect(pokeTitle(r)).toBe("Jab");
  s?.close();
});

// VTT-855
test("an answer giving an empty free-usage name reads as none", async () => {
  const { r, s, sock, answers } = boots(["poke"]);
  await settle();
  seed(sock, ["poke"], [{ actorId: "a1", name: "Lera", abilityIds: ["poke"] }]);
  await settle();
  answers[last(answers)]!(body({ freeUsageName: "" }));
  await settle();
  expect(pokeTitle(r)).toBe("Jab");
  s?.close();
});
