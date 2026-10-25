// Independent QA for VTT-393: the window shows no log sequence number to any
// role, except in the reason it gives for a log it cannot fold.
//
// "The window" is everything drawn into the root boot() is handed, hover text
// (title) and accessible names (aria-*) included. A "log sequence number" is
// the sequence of an Envelope the connection was sent, or the world sequence a
// command's result names, however it is formatted. Every sequence a leak is
// searched for is five digits from a range no other number in these fixtures
// reaches, so a match can only be a sequence.

import "./support/dom";

import { test, expect, beforeEach, afterEach } from "bun:test";
import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { boot } from "../src/app";
import { fold } from "../src/fold";
import { renderSpectator, describe as describeEvent } from "../src/view/spectator";
import { namer } from "../src/view/naming";
import { newState } from "../src/state";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";

// --- harness, copied from app.test.ts's harness half -------------------------

function setURL(url: string) {
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL(url);
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setURL("http://localhost/");
});

function root(): HTMLElement {
  const el = document.createElement("div");
  el.id = "app";
  document.body.replaceChildren(el);
  return el;
}

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

async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

afterEach(() => {
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
});

// --- what the window shows ----------------------------------------------------

/** Attributes whose value is shown on hover or read aloud. Id references
 *  (aria-labelledby and kin) are excluded: a screen reader reads the element
 *  they point at, which textContent already covers. */
const ID_REFS = new Set([
  "aria-labelledby", "aria-describedby", "aria-controls", "aria-owns",
  "aria-activedescendant", "aria-flowto", "aria-details", "aria-errormessage",
]);
const SHOWN_ATTRS = ["title", "alt", "placeholder", "label"];

/** Every string the window shows: its text, hover text, accessible names and
 *  the current value of every form control. */
function shown(el: Element): string[] {
  const out: string[] = [el.textContent ?? ""];
  for (const e of [el, ...Array.from(el.querySelectorAll("*"))]) {
    for (const a of Array.from(e.attributes)) {
      const name = a.name.toLowerCase();
      if (SHOWN_ATTRS.includes(name) || (name.startsWith("aria-") && !ID_REFS.has(name))) {
        out.push(`${name}=${a.value}`);
      }
    }
    if (e instanceof HTMLInputElement || e instanceof HTMLTextAreaElement || e instanceof HTMLSelectElement) {
      out.push(`value=${e.value}`);
    }
  }
  return out;
}

/** A sequence in any of the usual decimal spellings: bare, or grouped in
 *  thousands by a comma, a period, an apostrophe, a space, a no-break space or
 *  a narrow no-break space. Never part of a longer run of digits. */
function seqPattern(seq: number): RegExp {
  let rest = String(seq);
  const groups: string[] = [];
  while (rest.length > 3) {
    groups.unshift(rest.slice(-3));
    rest = rest.slice(0, -3);
  }
  groups.unshift(rest);
  return new RegExp(`(?<!\\d)${groups.join("[,.'\\u0020\\u00a0\\u202f]?")}(?!\\d)`);
}

/** Every (sequence, shown string) pair where the string shows the sequence. */
function leaks(strings: string[], seqs: number[]): string[] {
  const found: string[] = [];
  for (const seq of seqs) {
    const re = seqPattern(seq);
    for (const s of strings) if (re.test(s)) found.push(`${seq} in ${JSON.stringify(s.slice(0, 160))}`);
  }
  return found;
}

const FOLD_FAILED = "Cannot derive state from the log: ";

// --- fixtures -------------------------------------------------------------------

const RULESET = {
  id: "barrow-rules",
  name: "Barrow Rules",
  abilities: [{ id: "strike", name: "Strike", range: 1, maxTargets: 1, usage: { kind: "atWill" } }],
  conditions: [{ id: "dazed", name: "Dazed", description: "Reeling" }],
  resources: ["hp"],
};

/** An event id with no digit in it, so an id the window shows can never be
 *  mistaken for a sequence; it depends on the entry's position, not on its
 *  sequence, so two numberings of one log carry the same ids. */
function eventId(log: string, i: number): string {
  let s = "";
  let k = i;
  do {
    s = "abcdefghijklmnopqrstuvwxyz"[k % 26] + s;
    k = Math.floor(k / 26) - 1;
  } while (k >= 0);
  return `evt-${log}-${s}`;
}

/** Number a log's entries from `base`, one apart, as `event` frames. */
function numbered(log: string, base: number, entries: ((seq: (i: number) => number) => Frame)[]): Frame[] {
  const seq = (i: number) => base + i;
  return entries.map((entry, i) => ({
    event: {
      eventId: eventId(log, i),
      sequence: String(seq(i)),
      sessionId: "sess-night",
      ...entry(seq),
    },
  }));
}

const CRYPT = {
  sceneId: "crypt",
  name: "Crypt",
  gridWidth: 6,
  gridHeight: 5,
  tiles: {
    "1,0": { kind: "door", material: "oak", art: "" },
    "2,3": { kind: "floor", material: "stone", art: "" },
    "3,3": { kind: "floor", material: "stone", art: "" },
  },
  objects: [],
};

const HERO = { actorId: "hero", name: "Lera", moduleId: "m", attributes: { brawn: 3 }, resources: { hp: { current: 7, max: 9 } }, kind: "ACTOR_KIND_PARTY_MEMBER" };
const GOBLIN = { actorId: "gob", name: "Snag", moduleId: "m", attributes: {}, resources: { hp: { current: 4, max: 4 } }, kind: "ACTOR_KIND_NON_PARTY" };
const MAGE = { actorId: "mage", name: "Quill", moduleId: "m", attributes: {}, resources: {}, kind: "ACTOR_KIND_PARTY_MEMBER" };

const dm = { actorRole: "dm", participantId: "p-dm" };

/** The world, as the DM's and the agent's connections are sent it: one entry
 *  of every kind the contract has. */
function worldLog(base: number): Frame[] {
  return numbered("world", base, [
    () => ({ ...dm, adventureLoaded: { adventureId: "barrow", name: "The Barrow" } }),
    () => ({ ...dm, sessionStarted: { name: "Night" } }),
    () => ({ ...dm, sceneCreated: CRYPT }),
    () => ({ ...dm, actorAdded: { actor: HERO } }),
    () => ({ ...dm, actorControlGranted: { actorId: "hero", participantId: "p-ann", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
    () => ({ ...dm, actorAdded: { actor: GOBLIN } }),
    () => ({ ...dm, tokenPlaced: { tokenId: "t-hero", sceneId: "crypt", actorId: "hero", position: { x: 2, y: 3 } } }),
    () => ({ ...dm, tokenPlaced: { tokenId: "t-gob", sceneId: "crypt", actorId: "gob", position: { x: 4, y: 2 } } }),
    () => ({ actorRole: "player", participantId: "p-ann", tokenMoved: { tokenId: "t-hero", sceneId: "crypt", from: { x: 2, y: 3 }, to: { x: 3, y: 3 }, reason: "creeps" } }),
    (s) => ({ ...dm, narrationAdded: { text: "The torch gutters.", as: "", anchorFromSeq: String(s(6)), anchorToSeq: String(s(8)) } }),
    () => ({ ...dm, noteUpserted: { key: "clue", title: "Clue", text: "A bent key", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
    () => ({ ...dm, noteUpserted: { key: "plan", title: "Plan", text: "Ambush", visibility: "NOTE_VISIBILITY_SECRET" } }),
    () => ({ ...dm, resourceChanged: { actorId: "hero", resource: "hp", delta: -2, newValue: 5, reason: "trap" } }),
    () => ({ ...dm, conditionApplied: { actorId: "hero", conditionId: "dazed", source: "trap" } }),
    () => ({ actorRole: "player", participantId: "p-ann", abilityUsed: { actorId: "hero", abilityId: "strike", targetIds: ["gob"], rolls: [{ expression: "1d6", results: [4], total: 4 }], outcomeSummary: "misses" } }),
    () => ({ ...dm, attackRolled: { attackerId: "gob", targetId: "hero", expression: "1d20+2", rolls: [{ die: 20, result: 11 }], modifiers: [{ source: "knife", value: 2 }], total: 13, versus: "guard", outcome: "hit" } }),
    () => ({ ...dm, doorOpened: { sceneId: "crypt", at: { x: 1, y: 0 } } }),
    () => ({ ...dm, doorClosed: { sceneId: "crypt", at: { x: 1, y: 0 } } }),
    () => ({ ...dm, conditionRemoved: { actorId: "hero", conditionId: "dazed", reason: "shakes it off" } }),
    () => ({ actorRole: "player", participantId: "p-ann", narrationAdded: { text: "Back, beast!", as: "Lera" } }),
    () => ({ ...dm, noteDeleted: { key: "plan" } }),
    () => ({ ...dm, tokenRemoved: { tokenId: "t-gob" } }),
    () => ({ ...dm, actorRemoved: { actorId: "gob" } }),
    () => ({ ...dm, actorControlRevoked: { actorId: "hero", participantId: "p-ann" } }),
    () => ({ ...dm, sessionEnded: {} }),
  ]);
}

/** One party member's character log, as a player's or a spectator's
 *  connection is sent it: what that party member experienced, numbered by its
 *  own log (SPEC-015, VTT-338). `holder` is who controls the party member. */
function characterLog(base: number, holder: string): Frame[] {
  return numbered("hero", base, [
    () => ({ sessionStarted: { name: "Night" } }),
    () => ({ sceneCreated: CRYPT }),
    () => ({ actorAdded: { actor: HERO } }),
    () => ({ actorControlGranted: { actorId: "hero", participantId: holder, kind: "ACTOR_KIND_PARTY_MEMBER" } }),
    () => ({ actorAdded: { actor: MAGE } }),
    () => ({ actorControlGranted: { actorId: "mage", participantId: "p-bo", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
    () => ({ tokenPlaced: { tokenId: "t-hero", sceneId: "crypt", actorId: "hero", position: { x: 2, y: 3 } } }),
    () => ({ sceneSeen: { sceneId: "crypt", tiles: { "2,3": CRYPT.tiles["2,3"], "3,3": CRYPT.tiles["3,3"] }, objects: [], visible: ["2,3", "3,3", "4,2"] } }),
    () => ({ actorAdded: { actor: GOBLIN } }),
    () => ({ tokenPlaced: { tokenId: "t-gob", sceneId: "crypt", actorId: "gob", position: { x: 4, y: 2 } } }),
    () => ({ tokenMoved: { tokenId: "t-hero", sceneId: "crypt", from: { x: 2, y: 3 }, to: { x: 3, y: 3 }, reason: "creeps" } }),
    (s) => ({ narrationAdded: { text: "The torch gutters.", as: "", anchorFromSeq: String(s(9)), anchorToSeq: String(s(10)) } }),
    () => ({ tokenHidden: { tokenId: "t-gob" } }),
    () => ({ noteUpserted: { key: "clue", title: "Clue", text: "A bent key", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
    () => ({ resourceChanged: { actorId: "hero", resource: "hp", delta: -2, newValue: 5, reason: "trap" } }),
    () => ({ conditionApplied: { actorId: "hero", conditionId: "dazed", source: "trap" } }),
    () => ({ narrationAdded: { text: "Back, beast!", as: "Lera" } }),
  ]);
}

/** A second party member's character log whose content shares no id with
 *  characterLog's, so it folds whether or not the client keeps what it held. */
function towerLog(base: number): Frame[] {
  return numbered("mage", base, [
    () => ({ sceneCreated: { sceneId: "tower", name: "Tower", gridWidth: 4, gridHeight: 4, tiles: {}, objects: [] } }),
    () => ({ actorAdded: { actor: { actorId: "owl", name: "Hoot", moduleId: "m", attributes: {}, resources: {}, kind: "ACTOR_KIND_NON_PARTY" } } }),
    () => ({ tokenPlaced: { tokenId: "t-owl", sceneId: "tower", actorId: "owl", position: { x: 1, y: 1 } } }),
    () => ({ narrationAdded: { text: "Wind howls.", as: "" } }),
  ]);
}

function seqsOf(frames: Frame[]): number[] {
  return frames.map((f) => Number((f["event"] as { sequence: string }).sequence));
}

type Role = "dm" | "agent" | "player" | "spectator";
const ROLES: Role[] = ["dm", "agent", "player", "spectator"];
const watchesALog = (r: Role) => r === "player" || r === "spectator";

async function bootAs(role: Role) {
  useFakeSocket();
  stubMetadata({
    "/api/me": { participantId: "p-me", name: "Mira", role },
    "/api/ruleset": RULESET,
  });
  setURL("http://localhost/?token=tok-qa");
  const el = root();
  boot(el);
  await settle();
  const ws = FakeSocket.instances[0];
  if (!ws) throw new Error("boot dialled no socket");
  ws.open();
  await settle();
  return { el, ws };
}

/** Open a connection the way SPEC-007 says one opens: the head, the presence
 *  snapshot, then for a log seat the ViewpointChanged, then the log. */
function openWith(ws: FakeSocket, role: Role, log: Frame[], head: number) {
  ws.deliver({ catchUpHead: { headSequence: String(head) } });
  ws.deliver({
    presenceSnapshot: {
      present: [
        { participantId: "p-me", displayName: "Mira", state: "PRESENCE_STATE_CONNECTED" },
        { participantId: "p-ann", displayName: "Ann", state: "PRESENCE_STATE_CONNECTED" },
      ],
    },
  });
  if (watchesALog(role)) {
    ws.deliver({ watchableChanged: { actors: [{ actorId: "hero", name: "Lera" }, { actorId: "mage", name: "Quill" }] } });
    ws.deliver({ viewpointChanged: { actorId: "hero" } });
  }
  for (const f of log) ws.deliver(f);
}

/** The log a role's connection is sent, numbered from `base`. A player
 *  watches the party member they hold; a spectator watches Ann's. */
function logFor(role: Role, base: number): Frame[] {
  if (role === "dm" || role === "agent") return worldLog(base);
  return characterLog(base, role === "player" ? "p-me" : "p-ann");
}

/** The window really drew the log, so an absence of numbers means something. */
function expectLogDrawn(el: HTMLElement) {
  const text = el.textContent ?? "";
  expect(text).not.toContain(FOLD_FAILED);
  expect(text).toContain("The torch gutters.");
  expect(text).toContain("Back, beast!");
  expect(text).toContain("Crypt");
  expect(el.querySelector('[title="Lera"]')).not.toBeNull();
}

function lastRequestId(ws: FakeSocket): string {
  const last = ws.sent[ws.sent.length - 1];
  if (last === undefined) throw new Error("nothing was sent");
  return (JSON.parse(last) as { requestId: string }).requestId;
}

// --- the window after a full log, per role --------------------------------------

const HIGH = 90417;

// VTT-393
test.each(ROLES)("%s: a window drawn from a whole log shows none of its sequences, nor the head", async (role) => {
  const { el, ws } = await bootAs(role);
  const log = logFor(role, HIGH);
  const seqs = seqsOf(log);
  openWith(ws, role, log, seqs[seqs.length - 1]!);
  await settle();

  expectLogDrawn(el);
  expect(leaks(shown(el), seqs)).toEqual([]);
});

// VTT-393
test.each(ROLES)("%s: the window is the same whatever numbers the log carries", async (role) => {
  // Format-independent: if no number of the log reaches the window, two
  // numberings of the same content draw the same window — whatever the
  // spelling, and with the low numbers a fresh log really starts at.
  const draw = async (base: number) => {
    const { el, ws } = await bootAs(role);
    const log = logFor(role, base);
    const seqs = seqsOf(log);
    openWith(ws, role, log, seqs[seqs.length - 1]!);
    await settle();
    expectLogDrawn(el);
    return shown(el);
  };
  const fromOne = await draw(1);
  const fromHigh = await draw(HIGH);
  expect(fromHigh).toEqual(fromOne);
});

// --- a window that changes over time --------------------------------------------

// VTT-393
test.each(ROLES)("%s: entries arriving one at a time after the first paint never show a sequence", async (role) => {
  const { el, ws } = await bootAs(role);
  const log = logFor(role, HIGH);
  const seqs = seqsOf(log);
  const firstPaint = 6;
  openWith(ws, role, log.slice(0, firstPaint), seqs[firstPaint - 1]!);
  await settle();
  expect(el.textContent ?? "").toContain("Crypt");
  expect(leaks(shown(el), seqs)).toEqual([]);

  for (const f of log.slice(firstPaint)) {
    ws.deliver(f);
    await settle();
    expect(el.textContent ?? "").not.toContain(FOLD_FAILED);
    expect(leaks(shown(el), seqs)).toEqual([]);
  }
  expectLogDrawn(el);
});

// VTT-393
test("spectator: perching on another shoulder, and the new log that follows, show no sequence of either log", async () => {
  const { el, ws } = await bootAs("spectator");
  const first = characterLog(HIGH, "p-ann");
  const second = towerLog(70011);
  const seqs = [...seqsOf(first), ...seqsOf(second)];
  openWith(ws, "spectator", first, seqsOf(first).at(-1)!);
  await settle();
  expectLogDrawn(el);
  expect(leaks(shown(el), seqs)).toEqual([]);

  const quill = Array.from(el.querySelectorAll("button")).find((b) => b.textContent === "Quill");
  if (!quill) throw new Error("no shoulder offered for Quill");
  quill.click();
  await settle();
  expect(JSON.parse(ws.sent.at(-1)!)).toMatchObject({ setViewpoint: { actorId: "mage" } });
  ws.deliver({ result: { requestId: lastRequestId(ws), ok: true } });
  await settle();
  expect(leaks(shown(el), seqs)).toEqual([]);

  ws.deliver({ viewpointChanged: { actorId: "mage" } });
  for (const f of second) {
    ws.deliver(f);
    await settle();
    expect(el.textContent ?? "").not.toContain(FOLD_FAILED);
    expect(leaks(shown(el), seqs)).toEqual([]);
  }
  expect(el.textContent ?? "").toContain("Wind howls.");
});

// VTT-393
test("dm: a dropped connection, and the resumed one, show no sequence", async () => {
  const { el, ws } = await bootAs("dm");
  const log = worldLog(HIGH);
  const seqs = seqsOf(log);
  openWith(ws, "dm", log, seqs.at(-1)!);
  await settle();

  ws.close();
  await settle();
  expect(leaks(shown(el), seqs)).toEqual([]);

  const reconnect = Array.from(el.querySelectorAll("button")).find((b) => /reconnect/i.test(b.textContent ?? ""));
  if (!reconnect) throw new Error("no reconnect offered on a closed connection");
  reconnect.click();
  await settle();
  const redial = FakeSocket.instances[1];
  if (!redial) throw new Error("reconnect dialled no socket");
  redial.open();
  await settle();
  // Whatever the redial resumes after, the server re-sends what follows it.
  const after = Number(new URL(redial.url.replace(/^ws/, "http")).searchParams.get("after"));
  openWith(redial, "dm", log.filter((f) => seqsOf([f])[0]! > after), seqs.at(-1)!);
  await settle();
  expectLogDrawn(el);
  expect(leaks(shown(el), seqs)).toEqual([]);
});

// --- a command's result ----------------------------------------------------------

// VTT-393
test("dm: a command whose result names the sequence it produced shows that sequence nowhere", async () => {
  const { el, ws } = await bootAs("dm");
  const log = worldLog(HIGH);
  const produced = HIGH + log.length;
  const seqs = [...seqsOf(log), produced];
  openWith(ws, "dm", log, seqs.at(-2)!);
  await settle();

  const name = el.querySelector<HTMLInputElement>('input[placeholder="session name"]');
  const start = Array.from(el.querySelectorAll("button")).find((b) => b.textContent === "Start session");
  if (!name || !start) throw new Error("the DM console offers no session start");
  name.value = "Dawn";
  name.dispatchEvent(new Event("input"));
  start.click();
  await settle();
  expect(JSON.parse(ws.sent.at(-1)!)).toMatchObject({ startSession: { name: "Dawn" } });

  // SPEC-007: the result may arrive before the events it produced.
  ws.deliver({ result: { requestId: lastRequestId(ws), ok: true, sequence: String(produced) } });
  await settle();
  expect(leaks(shown(el), seqs)).toEqual([]);

  ws.deliver({ event: { eventId: "evt-world-dawn", sequence: String(produced), sessionId: "sess-dawn", ...dm, sessionStarted: { name: "Dawn" } } });
  await settle();
  expect(el.textContent ?? "").toContain("Dawn");
  expect(el.textContent ?? "").not.toContain(FOLD_FAILED);
  expect(leaks(shown(el), seqs)).toEqual([]);
});

// VTT-393
test("player: neither the world sequence in a command's result nor the character log's own reaches the window", async () => {
  // The owner's ruling: a player must not see the world sequence, so that two
  // messages cannot reveal how much the DM did between them.
  const { el, ws } = await bootAs("player");
  const log = characterLog(HIGH, "p-me");
  const own = HIGH + log.length;
  const world = 77123;
  const seqs = [...seqsOf(log), own, world];
  openWith(ws, "player", log, seqsOf(log).at(-1)!);
  await settle();
  expectLogDrawn(el);

  const say = el.querySelector<HTMLInputElement>('input[placeholder="say or narrate"]');
  const send = Array.from(el.querySelectorAll("button")).find((b) => b.textContent === "Send");
  if (!say || !send) throw new Error("the player panel offers nothing to say");
  say.value = "Hold the line";
  say.dispatchEvent(new Event("input"));
  send.click();
  await settle();
  expect(JSON.parse(ws.sent.at(-1)!)).toMatchObject({ addNarration: { text: "Hold the line" } });

  // SPEC-007: the events may arrive before the result.
  ws.deliver({ event: { eventId: "evt-hero-said", sequence: String(own), sessionId: "sess-night", narrationAdded: { text: "Hold the line", as: "" } } });
  await settle();
  expect(el.textContent ?? "").toContain("Hold the line");
  expect(leaks(shown(el), seqs)).toEqual([]);

  ws.deliver({ result: { requestId: lastRequestId(ws), ok: true, sequence: String(world) } });
  await settle();
  expect(el.textContent ?? "").not.toContain(FOLD_FAILED);
  expect(leaks(shown(el), seqs)).toEqual([]);
});

// --- the board's own drawing, and the labels behind feed and ticker --------------

function envelopes(frames: Frame[]): Envelope[] {
  return frames.map((f) => fromJson(EnvelopeSchema, f["event"] as JsonValue));
}

/** A 2D context that records every call and answers only what drawing needs
 *  (`measureText`, `getLineDash`), so whatever the board writes onto its canvas
 *  can be read back. */
function recordingContext(calls: { name: string; args: unknown[] }[]): CanvasRenderingContext2D {
  const store: Record<string | symbol, unknown> = {};
  return new Proxy(store, {
    get(target, prop) {
      if (prop in target) return target[prop];
      return (...args: unknown[]) => {
        calls.push({ name: String(prop), args });
        if (prop === "measureText") return { width: 10 };
        if (prop === "getLineDash") return [];
        return undefined;
      };
    },
    set(target, prop, value) {
      target[prop] = value;
      return true;
    },
  }) as unknown as CanvasRenderingContext2D;
}

// VTT-393
test.each(["world", "character"] as const)("renderSpectator over a %s log writes no sequence as text, onto the canvas or into the page", (kind) => {
  const frames = kind === "world" ? worldLog(HIGH) : characterLog(HIGH, "p-me");
  const log = envelopes(frames);
  const seqs = seqsOf(frames);
  const calls: { name: string; args: unknown[] }[] = [];
  const el = root();
  renderSpectator(el, fold(log), log, "open", {
    participants: [{ participantId: "p-me", displayName: "Mira" }],
    getContext: () => recordingContext(calls),
    ticker: kind === "world",
  });

  expectLogDrawn(el);
  if (kind === "world") expect(el.querySelectorAll(".ticker .tick").length).toBeGreaterThan(0);
  expect(leaks(shown(el), seqs)).toEqual([]);
  expect(calls.length).toBeGreaterThan(0);
  const written = calls
    .filter((c) => c.name === "fillText" || c.name === "strokeText")
    .map((c) => String(c.args[0]));
  expect(leaks(written, seqs)).toEqual([]);
});

// VTT-393
test("describe, the label feed and ticker show for an event, names no sequence for any kind of event", () => {
  const frames = [...worldLog(HIGH), ...characterLog(HIGH + 100, "p-me")];
  const seqs = seqsOf(frames);
  const labels = envelopes(frames).map((e) => describeEvent(e, namer(newState(), [], false, undefined)));
  expect(labels.length).toBe(frames.length);
  expect(leaks(labels, seqs)).toEqual([]);
});

// --- the named exception ----------------------------------------------------------

// VTT-393
test.each(ROLES)("%s: a log that cannot fold shows the reason, and no sequence outside it", async (role) => {
  // The reason itself may name a sequence: that is the exception VTT-393
  // names, and nothing here asserts on it. The entry chosen is one whose
  // reason does name it, so the window around the reason is what is tested.
  const { el, ws } = await bootAs(role);
  const good = logFor(role, HIGH);
  const unfoldable = watchesALog(role) ? { sessionStarted: { name: "Again" } } : { sessionEnded: {} };
  const bad = {
    event: { eventId: "evt-bad", sequence: String(HIGH + good.length), sessionId: "sess-night", ...dm, ...unfoldable },
  };
  const seqs = [...seqsOf(good), HIGH + good.length];
  openWith(ws, role, [...good, bad], seqs.at(-1)!);
  await settle();

  const text = el.textContent ?? "";
  expect(text).toContain(FOLD_FAILED);
  const reason = Array.from(el.querySelectorAll("*")).filter((e) => (e.textContent ?? "").startsWith(FOLD_FAILED)).at(-1);
  if (!reason) throw new Error("no element carries the fold's reason");
  const rest = el.cloneNode(true) as HTMLElement;
  const path: number[] = [];
  for (let n: Element = reason; n !== el; n = n.parentElement!) path.unshift(Array.from(n.parentElement!.children).indexOf(n));
  let twin: Element = rest;
  for (const i of path) twin = twin.children[i]!;
  twin.remove();
  expect(leaks(shown(rest), seqs)).toEqual([]);
});
