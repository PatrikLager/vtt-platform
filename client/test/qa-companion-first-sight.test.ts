import "./support/dom";

import { afterEach, beforeEach, expect, test } from "bun:test";
import { create, toJson, type MessageInitShape } from "@bufbuild/protobuf";
import {
  ActorKind,
  EnvelopeSchema,
  type ActorSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { boot } from "../src/app";
import { fold } from "../src/fold";
import { buildFeed } from "../src/view/feed";
import { renderSpectator, type ViewExtras } from "../src/view/spectator";

type Payload = NonNullable<MessageInitShape<typeof EnvelopeSchema>["payload"]>;
type ActorInit = MessageInitShape<typeof ActorSchema>;

const PARTY = ActorKind.PARTY_MEMBER;
const SCENE = "glade";

interface Entry {
  payload: Payload;
  eventId: string;
  out: boolean;
}

const own = (payload: Payload): Entry => ({ payload, eventId: "", out: false });
const fromWorld = (eventId: string, payload: Payload): Entry =>
  ({ payload, eventId, out: false });
const leftOut = (e: Entry): Entry => ({ ...e, out: true });

const removal = (actorId: string): Payload =>
  ({ case: "actorRemoved", value: { actorId } });
const addition = (actor: ActorInit): Payload =>
  ({ case: "actorAdded", value: { actor } });
const grant = (actorId: string, participantId: string): Payload =>
  ({ case: "actorControlGranted", value: { actorId, participantId, kind: PARTY } });
const condition = (actorId: string, conditionId: string): Payload =>
  ({ case: "conditionApplied", value: { actorId, conditionId } });
const placing = (tokenId: string, actorId: string, x: number, y: number): Payload =>
  ({
    case: "tokenPlaced",
    value: { tokenId, sceneId: SCENE, actorId, position: { x, y } },
  });
const sight = (visible: string[]): Payload =>
  ({ case: "sceneSeen", value: { sceneId: SCENE, visible } });
const move = (
  tokenId: string, x0: number, y0: number, x: number, y: number,
): Payload => ({
  case: "tokenMoved",
  value: { tokenId, sceneId: SCENE, from: { x: x0, y: y0 }, to: { x, y } },
});

const BRAM: ActorInit = {
  actorId: "bram", name: "Bram", moduleId: "mod", kind: PARTY,
  attributes: { grit: 3 }, resources: { hp: { current: 7, max: 9 } },
};
const ASH: ActorInit = { actorId: "ash", name: "Ash", moduleId: "mod", kind: PARTY };
const ASH_WHOLE: ActorInit = {
  ...ASH, attributes: { wit: 4 }, resources: { hp: { current: 5, max: 8 } },
};
const CARA: ActorInit = {
  actorId: "cara", name: "Cara", moduleId: "mod", kind: PARTY,
};
const CARA_WHOLE: ActorInit = {
  ...CARA, attributes: { nerve: 2 }, resources: { hp: { current: 3, max: 6 } },
};
const DOV: ActorInit = { actorId: "dov", name: "Dov", moduleId: "mod", kind: PARTY };
const GOB: ActorInit = {
  actorId: "gob", name: "Snag", moduleId: "mod", kind: ActorKind.NON_PARTY,
};

function opening(): Entry[] {
  return [
    own({ case: "sessionStarted", value: { name: "Dusk" } }),
    own({
      case: "sceneCreated",
      value: { sceneId: SCENE, name: "Glade", gridWidth: 6, gridHeight: 6 },
    }),
    own(addition(BRAM)),
    own(grant("bram", "p-me")),
    own(addition(ASH)),
    own(grant("ash", "p-ash")),
    own(addition(CARA)),
    own(grant("cara", "p-cara")),
    own(placing("t-bram", "bram", 1, 1)),
    own(sight(["1,1", "2,1"])),
  ];
}

function step(): Entry[] {
  return [
    fromWorld("evt-step", move("t-bram", 1, 1, 3, 3)),
    own(sight(["3,3", "4,4", "5,4"])),
  ];
}

function firstSight(): Entry[] {
  return [
    own(removal("ash")),
    own(addition(ASH_WHOLE)),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
  ].map(leftOut);
}

const arrival = (): Entry => own(placing("t-ash", "ash", 4, 4));

const base = (): Entry[] => [...opening(), ...step(), ...firstSight(), arrival()];
const held = (): Entry[] => [...opening(), ...step(), arrival()];

function numbered(entries: Entry[]): Envelope[] {
  return entries.map((e, i) => create(EnvelopeSchema, {
    eventId: e.eventId,
    sequence: BigInt(i + 1),
    sessionId: "sess-dusk",
    payload: e.payload,
  }));
}

function toldSequences(log: Envelope[]): bigint[] {
  return buildFeed(log).flatMap((f) => f.events.map((e) => e.sequence));
}

function expectedSequences(entries: Entry[]): bigint[] {
  return entries.flatMap((e, i) => (e.out ? [] : [BigInt(i + 1)]));
}

function expectTold(entries: Entry[], why: string): void {
  const log = numbered(entries);
  expect(() => fold(log), `${why}: the fixture folds`).not.toThrow();
  expect(toldSequences(log), why).toEqual(expectedSequences(entries));
  const el = root();
  renderSpectator(el, fold(log), log, "open", {});
  expect(storyLines(el).length, `${why}: one Story line per entry told`)
    .toBe(expectedSequences(entries).length);
}

function root(): HTMLElement {
  const el = document.createElement("div");
  el.id = "app";
  document.body.replaceChildren(el);
  return el;
}

// Assert on counts and texts, never on an element: bun hangs printing a DOM node.
function storyLines(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll(".feed .mechanical"))
    .map((p) => p.textContent ?? "");
}

function tickerLines(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll(".ticker .tick"))
    .map((p) => p.textContent ?? "");
}

const OPENING_STORY = [
  "session started — Dusk",
  'scene "Glade"',
  "Bram is in the party",
  "control of Bram given",
  "Ash is in the party",
  "control of Ash given",
  "Cara is in the party",
  "control of Cara given",
  "Bram is at 1,1",
  "the view changed",
];
const BASE_STORY = [
  ...OPENING_STORY,
  "Bram moved to 3,3",
  "the view changed",
  "Ash is at 4,4",
];
const BASE_EVERY_LINE = [
  ...OPENING_STORY,
  "Bram moved to 3,3",
  "the view changed",
  "Ash left",
  "Ash is in the party",
  "control of Ash given",
  "Ash gained blessed",
  "Ash is at 4,4",
];

const count = (lines: string[], line: string): number =>
  lines.filter((l) => l === line).length;

// VTT-662
test("the run at a first sight is left out of buildFeed's entries", () => {
  expectTold(base(), "VTT-662: the bare removal, the add, its grant and condition");
});

// VTT-662
test("a log with no first sight tells every one of its entries", () => {
  const entries = held();
  expect(entries.some((e) => e.out), "VTT-662: no entry is marked out").toBe(false);
  expectTold(entries, "VTT-662: a companion already held comes into view");
});

// VTT-662
test("the opening's introduction and grant of a companion are told", () => {
  const log = numbered(base());
  const told = toldSequences(log);
  for (const i of [4, 5, 6, 7]) {
    expect(told, `VTT-662: opening entry ${i + 1} has no removal before it`)
      .toContain(BigInt(i + 1));
  }
});

// VTT-662
test("an actorAdded first in the log, with no event id, is told", () => {
  const entries = [own(addition(ASH)), own(grant("ash", "p-ash"))];
  expectTold(entries, "VTT-662: no removal comes before the add");
});

// VTT-662
test("an actorRemoved that carries an event id is told, and all behind it", () => {
  expectTold([
    ...opening(), ...step(),
    fromWorld("evt-gone", removal("ash")),
    own(addition(ASH_WHOLE)),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
    arrival(),
  ], "VTT-662: the removal carries an event id");
});

// VTT-662
test("a bare removal directly followed by another actor's add is told", () => {
  expectTold([
    ...opening(), ...step(),
    own(removal("ash")),
    own(addition(DOV)),
    own(addition(ASH_WHOLE)),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
    arrival(),
  ], "VTT-662: the add after the removal is another actor's");
});

// VTT-662
test("a bare removal followed by another actor's add, at the end, is told", () => {
  expectTold([
    ...opening(), ...step(),
    own(removal("ash")),
    own(addition(DOV)),
  ], "VTT-662: the add after the removal is another actor's");
});

const BETWEEN: [string, Entry][] = [
  ["a view of the log's own", own(sight(["3,3", "4,4"]))],
  ["an event of the world", fromWorld("evt-wait", move("t-bram", 3, 3, 3, 2))],
  ["another actor's condition", own(condition("cara", "hidden"))],
];

// VTT-662
test.each(BETWEEN)("%s between the removal and the add: the run is told", (_, x) => {
  expectTold([
    ...opening(), ...step(),
    own(removal("ash")),
    x,
    own(addition(ASH_WHOLE)),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
    arrival(),
  ], "VTT-662: an entry lies between the removal and the add");
});

// VTT-662
test("a grant behind the add that carries an event id is told", () => {
  expectTold([
    ...opening(), ...step(),
    leftOut(own(removal("ash"))),
    leftOut(own(addition(ASH_WHOLE))),
    fromWorld("evt-grant", grant("ash", "p-ash")),
    arrival(),
  ], "VTT-662: the grant carries an event id");
});

// VTT-662
test("a condition behind the run that carries an event id is told", () => {
  expectTold([
    ...opening(), ...step(),
    leftOut(own(removal("ash"))),
    leftOut(own(addition(ASH_WHOLE))),
    leftOut(own(grant("ash", "p-ash"))),
    fromWorld("evt-bless", condition("ash", "blessed")),
    arrival(),
  ], "VTT-662: the condition carries an event id");
});

// VTT-662
test("a bare condition behind a grant with an event id is told", () => {
  expectTold([
    ...opening(), ...step(),
    leftOut(own(removal("ash"))),
    leftOut(own(addition(ASH_WHOLE))),
    fromWorld("evt-grant", grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
  ], "VTT-662: the grant with an event id ends the run");
});

// VTT-662
test("another actor's condition behind the run is told", () => {
  expectTold([
    ...opening(), ...step(),
    ...firstSight().slice(0, 3),
    own(condition("bram", "steady")),
    arrival(),
  ], "VTT-662: the condition is Bram's, not Ash's");
});

// VTT-662
test("another actor's grant behind the add is told, and what follows it", () => {
  expectTold([
    ...opening(), ...step(),
    ...firstSight().slice(0, 2),
    own(grant("bram", "p-two")),
    own(condition("ash", "blessed")),
  ], "VTT-662: the grant is Bram's, and it ends Ash's run");
});

// VTT-662
test("two companions first seen at one event, each in its own run, are left out",
  () => {
  expectTold([
    ...opening(), ...step(),
    ...firstSight(),
    leftOut(own(removal("cara"))),
    leftOut(own(addition(CARA_WHOLE))),
    leftOut(own(grant("cara", "p-cara"))),
    leftOut(own(condition("cara", "keen"))),
    arrival(),
    own(placing("t-cara", "cara", 5, 4)),
  ], "VTT-662: two runs, one after the other");
});

// VTT-662
test("two companions' grants behind both adds are told", () => {
  expectTold([
    ...opening(), ...step(),
    leftOut(own(removal("ash"))),
    leftOut(own(addition(ASH_WHOLE))),
    leftOut(own(removal("cara"))),
    leftOut(own(addition(CARA_WHOLE))),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
    own(grant("cara", "p-cara")),
    own(condition("cara", "keen")),
    arrival(),
  ], "VTT-662: Ash's grant is not directly behind Ash's add");
});

// VTT-662
test("a bare condition after the token's arrival ends the run and is told", () => {
  expectTold([
    ...opening(), ...step(),
    ...firstSight().slice(0, 3),
    arrival(),
    own(condition("ash", "blessed")),
  ], "VTT-662: the token's arrival ends the run");
});

// VTT-662
test("a bare grant after a resource change ends the run and is told", () => {
  expectTold([
    ...opening(), ...step(),
    ...firstSight().slice(0, 2),
    own({
      case: "resourceChanged",
      value: { actorId: "ash", resource: "hp", delta: -1, newValue: 4 },
    }),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
  ], "VTT-662: a resource change ends the run");
});

const narration = (text: string, from = 0n, to = 0n): Payload =>
  ({
    case: "narrationAdded",
    value: { text, as: "", anchorFromSeq: from, anchorToSeq: to },
  });

function drawn(entries: Entry[]): { story: string[]; spoken: string[] } {
  const log = numbered(entries);
  const el = root();
  renderSpectator(el, fold(log), log, "open", {});
  const spoken = Array.from(el.querySelectorAll(".feed .narration"))
    .map((p) => p.textContent ?? "");
  return { story: storyLines(el), spoken };
}

// VTT-662
test("a narration between the removal and the add: the run is told", () => {
  const { story, spoken } = drawn([
    ...opening(), ...step(),
    own(removal("ash")),
    fromWorld("evt-say", narration("Leaves stir.")),
    own(addition(ASH_WHOLE)),
    own(grant("ash", "p-ash")),
    own(condition("ash", "blessed")),
    arrival(),
  ]);
  expect(spoken, "VTT-662: the narration is drawn").toEqual(["Leaves stir."]);
  expect(count(story, "Ash left"), "VTT-662: the removal is told").toBe(1);
  expect(count(story, "Ash is in the party"), "VTT-662: the add is told").toBe(2);
  expect(count(story, "control of Ash given"), "VTT-662: the grant is told").toBe(2);
  expect(count(story, "Ash gained blessed"), "VTT-662: the condition is told")
    .toBe(1);
});

// VTT-662
test("a narration anchored to the run alone is still drawn", () => {
  const entries = [...base(), own(narration("Ash steps out.", 13n, 16n))];
  expect(entries.slice(12, 16).every((e) => e.out), "VTT-662: the anchor is the run")
    .toBe(true);
  const { story, spoken } = drawn(entries);
  expect(spoken, "VTT-662: the narration is an entry of the log")
    .toEqual(["Ash steps out."]);
  expect(story, "VTT-662: the run stays untold").toEqual(BASE_STORY);
});

// VTT-662
test("a narration anchored over the run and the arrival is drawn with it", () => {
  const entries = [...base(), own(narration("Ash steps out.", 13n, 17n))];
  const { story, spoken } = drawn(entries);
  expect(spoken, "VTT-662: the narration is drawn").toEqual(["Ash steps out."]);
  expect(story, "VTT-662: the run stays untold").toEqual(BASE_STORY);
  const log = numbered(entries);
  const page = root();
  renderSpectator(page, fold(log), log, "open", {});
  const beat = Array.from(page.querySelectorAll(".feed .beat"))
    .find((b) => (b.textContent ?? "").includes("Ash steps out."));
  const lines = Array.from(beat?.querySelectorAll(".mechanical") ?? [])
    .map((p) => p.textContent ?? "");
  expect(lines, "VTT-662: the narration shares the arrival's beat")
    .toContain("Ash is at 4,4");
});

// VTT-662
test("a bare removal and an add with an event id are left out as the row words it",
  () => {
  expectTold([
    ...opening(), ...step(),
    leftOut(own(removal("ash"))),
    leftOut(fromWorld("evt-back", addition(ASH_WHOLE))),
    leftOut(own(grant("ash", "p-ash"))),
    leftOut(own(condition("ash", "blessed"))),
    arrival(),
  ], "VTT-662: only the removal is qualified by its event id");
});

// VTT-662
test("a creature's bare removal and re-introduction are left out", () => {
  expectTold([
    ...opening(),
    own(addition(GOB)),
    ...step(),
    leftOut(own(removal("gob"))),
    leftOut(own(addition(GOB))),
    own(placing("t-gob", "gob", 5, 4)),
  ], "VTT-662: the row names any actor, not only a companion");
});

// VTT-662
test("a run at the very end of the log is left out and nothing throws", () => {
  expectTold(
    [...opening(), ...step(), ...firstSight()],
    "VTT-662: the run ends the log",
  );
  expectTold(
    [...opening(), ...step(), ...firstSight().slice(0, 2)],
    "VTT-662: the add ends the log",
  );
});

// VTT-662
test("a bare removal as the last entry is told and nothing throws", () => {
  expectTold(
    [...opening(), ...step(), own(removal("ash"))],
    "VTT-662: no add follows the removal",
  );
  const lone = numbered([own(removal("ash"))]);
  expect(() => buildFeed(lone), "VTT-662: a lone removal").not.toThrow();
  expect(toldSequences(lone), "VTT-662: a lone removal is told").toEqual([1n]);
  expect(buildFeed([]), "VTT-662: an empty log").toEqual([]);
});

const PERCH: ViewExtras["perch"] = {
  actors: [{ actorId: "bram", name: "Bram" }],
  current: "bram",
  onPerch: () => {},
};
const VIEWS: [string, ViewExtras][] = [
  ["player", {}],
  ["spectator", { perch: PERCH }],
  ["dm", { dm: true, ticker: true }],
  ["agent", { dm: true, ticker: true }],
];

// VTT-662
test.each(VIEWS)("%s: the Story draws the token's arrival, not the run",
  (role, extras) => {
  const log = numbered(base());
  const el = root();
  renderSpectator(el, fold(log), log, "open", extras);
  const story = storyLines(el);
  expect(story, `VTT-662: ${role}'s Story`).toEqual(BASE_STORY);
  expect(count(story, "Ash left"), `VTT-662: ${role} is not told Ash left`).toBe(0);
  expect(count(story, "Ash is in the party"), `VTT-662: ${role}, joined once`)
    .toBe(1);
  expect(count(story, "control of Ash given"), `VTT-662: ${role}, one grant`)
    .toBe(1);
  expect(count(story, "Ash gained blessed"), `VTT-662: ${role}, no condition`)
    .toBe(0);
});

// VTT-662
test.each(VIEWS)("%s: the Story at a first sight equals the one of a companion held",
  (role, extras) => {
  const draw = (entries: Entry[]): string[] => {
    const log = numbered(entries);
    const el = root();
    renderSpectator(el, fold(log), log, "open", extras);
    return storyLines(el);
  };
  expect(draw(base()), `VTT-662: ${role}'s Story`).toEqual(draw(held()));
});

// VTT-447
test.each(VIEWS.slice(2))("%s: the Events panel still shows every entry of the run",
  (role, extras) => {
  const log = numbered(base());
  const el = root();
  renderSpectator(el, fold(log), log, "open", extras);
  expect(tickerLines(el), `VTT-447: ${role}'s panel, newest first`)
    .toEqual([...BASE_EVERY_LINE].reverse());
});

// VTT-662
test.each(VIEWS)("%s: the fold holds the companion whole after the run",
  (role, extras) => {
  const log = numbered(base());
  const st = fold(log);
  const ash = st.Actors["ash"];
  expect(ash?.attributes, "VTT-662: Ash's attributes").toEqual({ wit: 4 });
  expect(ash?.resources, "VTT-662: Ash's resources")
    .toEqual({ hp: { current: 5, max: 8 } });
  expect(ash?.controllerIds, "VTT-662: Ash's controllers").toEqual(["p-ash"]);
  expect(ash?.kind, "VTT-662: Ash's kind").toBe(PARTY);
  expect(st.Conditions["ash"]?.map((c) => c.ID), "VTT-662: Ash's conditions")
    .toEqual(["blessed"]);
  expect(st.Tokens["t-ash"], "VTT-662: Ash's token").toMatchObject({ X: 4, Y: 4 });

  const el = root();
  renderSpectator(el, st, log, "open", extras);
  const disc = el.querySelector('[data-token-id="t-ash"]');
  expect(disc !== null, `VTT-662: ${role}'s board draws Ash`).toBe(true);
  const chips = Array.from(disc?.querySelectorAll(".chip") ?? [])
    .map((c) => c.textContent);
  expect(chips, `VTT-662: ${role}'s board shows Ash's hp`).toEqual(["5/8"]);
  const dots = Array.from(disc?.querySelectorAll(".dot") ?? [])
    .map((d) => d.getAttribute("title"));
  expect(dots, `VTT-662: ${role}'s board shows Ash's condition`)
    .toEqual(["blessed"]);
  expect(count(storyLines(el), "Ash gained blessed"), `VTT-662: ${role}'s Story`)
    .toBe(0);
});

function setURL(url: string) {
  const g = globalThis as unknown as { happyDOM: { setURL(u: string): void } };
  g.happyDOM.setURL(url);
}

type Frame = Record<string, unknown>;

class FakeSocket {
  static readonly OPEN = 1;
  static instances: FakeSocket[] = [];
  readyState = 0;
  readonly sent: string[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((ev: { code: number; reason: string }) => void) | null = null;
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
    this.onclose?.({ code: 1000, reason: "" });
  }
}

const nativeFetch = globalThis.fetch;
const nativeWS = globalThis.WebSocket;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setURL("http://localhost/");
});

afterEach(() => {
  globalThis.fetch = nativeFetch;
  globalThis.WebSocket = nativeWS;
});

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

async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

const FOLD_FAILED = "Cannot derive state from the log: ";

type Role = "dm" | "agent" | "player" | "spectator";
const ROLES: Role[] = ["dm", "agent", "player", "spectator"];
const holdsALog = (r: Role) => r === "player" || r === "spectator";
const isDM = (r: Role) => r === "dm" || r === "agent";

async function bootAs(role: Role) {
  FakeSocket.instances = [];
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  stubMetadata({
    "/api/me": { participantId: "p-me", name: "Mira", role },
    "/api/ruleset": {
      id: "r", name: "R", abilities: [], conditions: [], resources: ["hp"],
    },
  });
  setURL("http://localhost/?token=tok-qa");
  const el = root();
  boot(el);
  await settle();
  const ws = FakeSocket.instances[0];
  if (!ws) throw new Error("boot dialled no socket");
  ws.open();
  await settle();
  ws.deliver({ catchUpHead: { headSequence: "0" } });
  ws.deliver({
    presenceSnapshot: {
      present: [{
        participantId: "p-me",
        displayName: "Mira",
        state: "PRESENCE_STATE_CONNECTED",
      }],
    },
  });
  if (holdsALog(role)) {
    ws.deliver({
      watchableChanged: { actors: [{ actorId: "bram", name: "Bram" }] },
    });
    ws.deliver({ viewpointChanged: { actorId: "bram" } });
  }
  await settle();
  return { el, ws };
}

const frames = (log: Envelope[]): Frame[] =>
  log.map((e) => ({ event: toJson(EnvelopeSchema, e) }));

function expectPanelFor(role: Role, el: HTMLElement): void {
  if (isDM(role)) {
    expect(tickerLines(el), `VTT-446: ${role}'s panel shows the run`)
      .toEqual([...BASE_EVERY_LINE].reverse());
  } else {
    expect(el.querySelectorAll(".ticker").length, `VTT-445: ${role} draws no panel`)
      .toBe(0);
  }
}

// VTT-662
test.each(ROLES)("%s: a session's window does not tell the run at a first sight",
  async (role) => {
  const { el, ws } = await bootAs(role);
  for (const f of frames(numbered(base()))) ws.deliver(f);
  await settle();
  expect(el.textContent ?? "", `VTT-662: ${role}'s log folds`)
    .not.toContain(FOLD_FAILED);
  expect(storyLines(el), `VTT-662: ${role}'s Story`).toEqual(BASE_STORY);
  expectPanelFor(role, el);
});

// VTT-662
test.each(ROLES)("%s: entries arriving one at a time end with the run untold",
  async (role) => {
  const { el, ws } = await bootAs(role);
  const log = numbered(base());
  const all = frames(log);
  const removedAt = base().findIndex((e) => e.payload.case === "actorRemoved");
  for (const f of all.slice(0, removedAt + 1)) {
    ws.deliver(f);
    await settle();
  }
  expect(count(storyLines(el), "Ash left"), `VTT-662: ${role}, the removal is last`)
    .toBe(1);
  for (const f of all.slice(removedAt + 1)) {
    ws.deliver(f);
    await settle();
  }
  expect(storyLines(el), `VTT-662: ${role}'s Story once the run is whole`)
    .toEqual(BASE_STORY);
});

// VTT-662
test.each(ROLES)("%s: a log that cannot fold shows its reason and no Story",
  async (role) => {
  const { el, ws } = await bootAs(role);
  const again = own({ case: "sessionStarted", value: { name: "Again" } });
  const entries = [...base(), again];
  for (const f of frames(numbered(entries))) ws.deliver(f);
  await settle();
  expect(el.textContent ?? "", `VTT-662: ${role} shows the reason`)
    .toContain(FOLD_FAILED);
  expect(el.querySelectorAll(".feed").length, `VTT-662: ${role} draws no Story`)
    .toBe(0);
});
