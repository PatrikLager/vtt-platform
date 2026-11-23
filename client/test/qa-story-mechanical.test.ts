import "./support/dom";
import { expect, test } from "bun:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  ActorKind,
  EnvelopeSchema,
  NoteVisibility,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { buildFeed } from "../src/view/feed";
import { describe as words, renderSpectator, type ViewExtras } from "../src/view/spectator";
import { namer, type Namer } from "../src/view/naming";
import { fold } from "../src/fold";

type Payload = NonNullable<MessageInitShape<typeof EnvelopeSchema>["payload"]>;
type Kind = NonNullable<Envelope["payload"]["case"]>;

function env(seq: number, eid: boolean, payload: Payload): Envelope {
  return create(EnvelopeSchema, {
    eventId: eid ? `ev-${seq}` : "",
    sequence: BigInt(seq),
    sessionId: "sess-1",
    payload,
  });
}

const SHEET: Kind[] = ["resourceChanged", "abilityGiven", "abilityTaken", "resourceAdded", "resourceRemoved"];
const CONTROL: Kind[] = ["actorControlGranted", "actorControlRevoked"];
const BOARD: Kind[] = [
  "tokenPlaced", "tokenMoved", "tokenRemoved", "tokenDescribed", "sceneCreated",
  "sceneSeen", "noteUpserted", "noteDeleted", "adventureLoaded",
];
const HIDDEN: Kind[] = ["tokenHidden"];
const DOORS: Kind[] = ["doorOpened", "doorClosed"];
const KEPT = new Set<Kind>([
  "sessionStarted", "sessionEnded", "actorAdded", "actorRemoved", "conditionApplied",
  "conditionRemoved", "abilityUsed", "attackRolled", "narrationAdded", "whisperAdded", "rulesetNamed",
]);
const LEFT_OUT: { kind: Kind; id: string }[] = [
  ...SHEET.map((kind) => ({ kind, id: "VTT-809" })),
  ...CONTROL.map((kind) => ({ kind, id: "VTT-810" })),
  ...BOARD.map((kind) => ({ kind, id: "VTT-818" })),
  ...HIDDEN.map((kind) => ({ kind, id: "VTT-823" })),
];

const SCENE: Payload = {
  case: "sceneCreated",
  value: {
    sceneId: "s1", name: "Cellar", gridWidth: 10, gridHeight: 10,
    objects: [
      { objectId: "d1", kind: "door", at: { x: 4, y: 5 }, width: 1, height: 1 },
      { objectId: "d2", kind: "door", at: { x: 7, y: 2 }, width: 1, height: 1 },
    ],
  },
};
const SESSION: Payload = { case: "sessionStarted", value: { name: "Night one" } };

/** One of every kind; the narration (index 29) is anchored to 27..28, both left out. */
const SCRIPT: Payload[] = [
  SESSION,
  { case: "adventureLoaded", value: { adventureId: "two-rooms", name: "Two Rooms" } },
  SCENE,
  { case: "sceneSeen", value: { sceneId: "s1", visible: [] } },
  { case: "actorAdded", value: { actor: { actorId: "a1", name: "Tobbe", kind: ActorKind.PARTY_MEMBER, resources: { stamina: { current: 5, max: 8 } }, abilityIds: ["hunters-flurry"] } } },
  { case: "actorControlGranted", value: { actorId: "a1", participantId: "p1", kind: ActorKind.PARTY_MEMBER } },
  { case: "actorAdded", value: { actor: { actorId: "a2", name: "Goblin", kind: ActorKind.NON_PARTY, resources: { stamina: { current: 3, max: 3 } } } } },
  { case: "tokenPlaced", value: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "Tobbe" } },
  { case: "tokenPlaced", value: { tokenId: "t2", sceneId: "s1", actorId: "a2", position: { x: 6, y: 6 }, name: "Goblin" } },
  { case: "tokenMoved", value: { tokenId: "t1", sceneId: "s1", from: { x: 1, y: 1 }, to: { x: 2, y: 1 }, reason: "sneaks along the wall" } },
  { case: "tokenDescribed", value: { tokenId: "t2", name: "Goblin", notes: "snarling" } },
  { case: "abilityUsed", value: { actorId: "a1", abilityId: "hunters-flurry", targetIds: ["a2"], outcomes: [{ targetId: "a2", result: "hit", total: 14 }] } },
  { case: "resourceChanged", value: { actorId: "a2", resource: "stamina", delta: -2, newValue: 1 } },
  { case: "attackRolled", value: { attackerId: "a2", targetId: "a1", expression: "1d20+2", total: 9, outcome: "miss" } },
  { case: "conditionApplied", value: { actorId: "a2", conditionId: "prone", source: "flurry" } },
  { case: "abilityGiven", value: { actorId: "a1", abilityId: "shield-bash" } },
  { case: "resourceAdded", value: { actorId: "a1", resource: "flurry_uses", value: { current: 2, max: 2 } } },
  { case: "resourceRemoved", value: { actorId: "a1", resource: "flurry_uses" } },
  { case: "abilityTaken", value: { actorId: "a1", abilityId: "shield-bash" } },
  { case: "conditionRemoved", value: { actorId: "a2", conditionId: "prone" } },
  { case: "doorOpened", value: { sceneId: "s1", at: { x: 4, y: 5 } } },
  { case: "doorClosed", value: { sceneId: "s1", at: { x: 4, y: 5 } } },
  { case: "noteUpserted", value: { key: "rumour", title: "A rumour", text: "rats", visibility: NoteVisibility.PUBLIC } },
  { case: "noteDeleted", value: { key: "rumour" } },
  { case: "whisperAdded", value: { text: "you hear something", toActorIds: ["a1"] } },
  { case: "tokenHidden", value: { tokenId: "t2" } },
  { case: "tokenRemoved", value: { tokenId: "t1" } },
  { case: "actorControlRevoked", value: { actorId: "a1", participantId: "p1" } },
  { case: "actorRemoved", value: { actorId: "a2" } },
  { case: "narrationAdded", value: { text: "The cellar falls quiet", anchorFromSeq: 27n, anchorToSeq: 28n } },
  { case: "rulesetNamed", value: { rulesetId: "qa-rules", version: "1", name: "Plain Rules" } },
  { case: "sessionEnded", value: {} },
];

const VARIANTS = {
  "every entry carries an event id": (_k: Kind) => true,
  "the left-out entries carry none": (k: Kind) => KEPT.has(k),
  "no entry carries one": (_k: Kind) => false,
} as const;
type Variant = keyof typeof VARIANTS;

function worldLog(v: Variant): Envelope[] {
  return SCRIPT.map((p, i) => env(i + 1, VARIANTS[v](p.case as Kind), p));
}

interface Role { name: string; dm: boolean; extras: () => ViewExtras }
const ROLES: Role[] = [
  { name: "a DM's window", dm: true, extras: () => ({ dm: true, ticker: true, console: document.createElement("div") }) },
  { name: "an agent's window", dm: true, extras: () => ({ dm: true, ticker: true }) },
  { name: "a player's window", dm: false, extras: () => ({ panel: document.createElement("div") }) },
  { name: "a spectator's window", dm: false, extras: () => ({}) },
];

type Plain = { id: string; name: string };
type Prop = { key: string; type: string; name: string };
const ABILITIES: Plain[] = [{ id: "shield-bash", name: "Shield Bash" }];
const PROPERTIES: Prop[] = [{ key: "stamina", type: "resource", name: "Stamina" }];

interface Drawn { root: HTMLElement; story: string[]; ticker: string[]; names: Namer; beats: HTMLElement[] }

function draw(log: Envelope[], role: Role, abilities: Plain[] = ABILITIES, properties: Prop[] = PROPERTIES): Drawn {
  const root = document.createElement("div");
  const st = fold(log);
  renderSpectator(root, st, log, "open", {
    ...role.extras(),
    abilities: abilities as unknown as NonNullable<ViewExtras["abilities"]>,
    properties: properties as unknown as NonNullable<ViewExtras["properties"]>,
  });
  const texts = (sel: string) => Array.from(root.querySelectorAll(sel)).map((n) => n.textContent ?? "");
  // A window that cannot fold shows a reason in place of the Story; every
  // fixture here must fold.
  expect(root.querySelector(".feed")).not.toBeNull();
  return {
    root,
    story: texts(".feed .beat p"),
    ticker: texts("section.ticker .tick"),
    names: namer(st, log, role.dm, abilities, properties),
    beats: Array.from(root.querySelectorAll(".feed .beat")) as HTMLElement[],
  };
}

/** What the Story should hold, in log order: kept kinds as describe words them, a door with an event id as an event. */
function expectedStory(log: Envelope[], names: Namer): string[] {
  return log.flatMap((e) => {
    const k = e.payload.case as Kind;
    if (k === "narrationAdded") return [(e.payload.value as { text: string }).text];
    if (k === "doorOpened") return e.eventId ? ["a door opened"] : [];
    if (k === "doorClosed") return e.eventId ? ["a door closed"] : [];
    return KEPT.has(k) ? [words(e, names)] : [];
  });
}

const MECHANICAL_FRAGMENTS = [
  "flurry_uses", "Shield Bash", "shield-bash", "Stamina", "stamina", "control of", " is at ",
  "moved to", "sneaks along", "went out of sight", "taken off the board", "described",
  "the view changed", "scene \"", "rumour", "two-rooms", "loaded", "4,5", "7,2",
];

// ---- every left-out kind, every role, with and without an event id ----

for (const role of ROLES) {
  for (const v of Object.keys(VARIANTS) as Variant[]) {
    const log = worldLog(v);

    // VTT-984
    test(`${role.name}, ${v}: the Story holds exactly the game's lines, in log order`, () => {
      const d = draw(log, role);
      expect(d.story).toEqual(expectedStory(log, d.names));
      expect(d.story.length).toBeGreaterThan(5);
    });

    // VTT-809 VTT-810 VTT-818 VTT-823
    test(`${role.name}, ${v}: no Story line carries a fragment of a sheet, control, board, scene, note or adventure entry`, () => {
      const d = draw(log, role);
      const hits = d.story.flatMap((line) => MECHANICAL_FRAGMENTS.filter((f) => line.includes(f)).map((f) => `${line} ⊃ ${f}`));
      expect(hits).toEqual([]);
    });

    for (const { kind, id } of LEFT_OUT) {
      const entries = log.filter((e) => e.payload.case === kind);
      // VTT-809 VTT-810 VTT-818 VTT-823 (the row this kind belongs to)
      test(`${id}: ${role.name}, ${v}: the Story draws no line for a ${kind}`, () => {
        const d = draw(log, role);
        const kept = expectedStory(log, d.names);
        expect(entries.length).toBeGreaterThan(0);
        for (const e of entries) {
          expect(e.eventId === "").toBe(!VARIANTS[v](kind));
          const w = words(e, d.names);
          expect(w.length).toBeGreaterThan(0);
          expect(kept).not.toContain(w);
          expect(d.story).not.toContain(w);
          if (role.dm) expect(d.ticker).toContain(w);
        }
      });
    }
  }
}

// ---- doors ----

function doorLog(eid: boolean): Envelope[] {
  return [
    env(1, true, SESSION),
    env(2, true, SCENE),
    env(3, eid, { case: "doorOpened", value: { sceneId: "s1", at: { x: 4, y: 5 } } }),
    env(4, eid, { case: "doorClosed", value: { sceneId: "s1", at: { x: 4, y: 5 } } }),
    env(5, eid, { case: "doorOpened", value: { sceneId: "s1", at: { x: 7, y: 2 } } }),
  ];
}

for (const role of ROLES) {
  // VTT-871
  test(`${role.name}: a door opened and closed with an event id is drawn as an event, naming no square`, () => {
    const d = draw(doorLog(true), role);
    expect(d.story).toEqual(["session started — Night one", "a door opened", "a door closed", "a door opened"]);
    for (const line of d.story) expect(/\d/.test(line)).toBe(false);
  });

  // VTT-820
  test(`${role.name}: a door opened and closed with no event id draws no Story line`, () => {
    const d = draw(doorLog(false), role);
    expect(d.story).toEqual(["session started — Night one"]);
    expect(d.story.some((l) => l.includes("door"))).toBe(false);
  });

  // VTT-871 VTT-820
  test(`${role.name}: a door with an event id is drawn and the one behind it with none is not`, () => {
    const log = [...doorLog(true).slice(0, 3), env(4, false, { case: "doorClosed", value: { sceneId: "s1", at: { x: 4, y: 5 } } })];
    const d = draw(log, role);
    expect(d.story).toEqual(["session started — Night one", "a door opened"]);
  });
}

// ---- the Events panel is unchanged ----

for (const role of ROLES.filter((r) => r.dm)) {
  for (const v of Object.keys(VARIANTS) as Variant[]) {
    // VTT-447
    test(`${role.name}, ${v}: the Events panel draws one row per entry, newest first, each as describe words it`, () => {
      const log = worldLog(v);
      expect(log.length).toBeLessThan(40);
      const d = draw(log, role);
      expect(d.ticker).toEqual([...log].reverse().map((e) => words(e, d.names)));
    });

    // VTT-447 VTT-821
    test(`${role.name}, ${v}: the Events panel words a door with its square and a move with its reason`, () => {
      const d = draw(worldLog(v), role);
      expect(d.ticker).toContain("a door opened at 4,5");
      expect(d.ticker).toContain("a door closed at 4,5");
      expect(d.ticker).toContain("Tobbe moved to 2,1 — sneaks along the wall");
    });

    // VTT-813
    test(`${role.name}, ${v}: the Events panel words a give and a take, the ability by its display name`, () => {
      const d = draw(worldLog(v), role);
      expect(d.ticker).toContain("Tobbe can now use Shield Bash");
      expect(d.ticker).toContain("Tobbe can no longer use Shield Bash");
    });

    // VTT-814
    test(`${role.name}, ${v}: the Events panel words a resource added and a resource removed`, () => {
      const d = draw(worldLog(v), role);
      expect(d.ticker).toContain("Tobbe now has flurry_uses at 2");
      expect(d.ticker).toContain("Tobbe no longer has flurry_uses");
    });
  }

  // VTT-813
  test(`${role.name}: with no ability answered the Events panel names a give's and a take's ability by its id`, () => {
    const d = draw(worldLog("every entry carries an event id"), role, []);
    expect(d.ticker).toContain("Tobbe can now use shield-bash");
    expect(d.ticker).toContain("Tobbe can no longer use shield-bash");
  });

  // VTT-815
  test(`${role.name}: the Events panel names a declared resource of a change by its display name`, () => {
    const d = draw(worldLog("every entry carries an event id"), role);
    expect(d.ticker).toContain("Goblin Stamina → 1");
  });

  // VTT-814 VTT-815
  test(`${role.name}: the Events panel words a declared resource added and removed by its display name`, () => {
    const props = [{ key: "flurry_uses", type: "resource", name: "Flurry uses" }, ...PROPERTIES];
    const d = draw(worldLog("every entry carries an event id"), role, ABILITIES, props);
    expect(d.ticker).toContain("Tobbe now has Flurry uses at 2");
    expect(d.ticker).toContain("Tobbe no longer has Flurry uses");
  });

  // VTT-816
  test(`${role.name}: the Events panel names a resource of a change by its key with no properties, or one declared as a number`, () => {
    expect(draw(worldLog("every entry carries an event id"), role, ABILITIES, []).ticker).toContain("Goblin stamina → 1");
    const numbered = [{ key: "stamina", type: "number", name: "Stamina" }];
    expect(draw(worldLog("every entry carries an event id"), role, ABILITIES, numbered).ticker).toContain("Goblin stamina → 1");
  });

  // VTT-447
  test(`${role.name}: a door with no event id still has its Events panel row, with its square`, () => {
    const d = draw(doorLog(false), role);
    expect(d.ticker).toEqual([
      "a door opened at 7,2", "a door closed at 4,5", "a door opened at 4,5", "scene \"Cellar\"", "session started — Night one",
    ]);
  });
}

// ---- a narration anchored only to left-out entries ----

function narrationLog(eid: boolean, from: bigint, to: bigint): Envelope[] {
  return [
    env(1, true, SESSION),
    env(2, true, SCENE),
    env(3, true, { case: "actorAdded", value: { actor: { actorId: "a1", name: "Tobbe", kind: ActorKind.PARTY_MEMBER, resources: { stamina: { current: 5, max: 8 } } } } }),
    env(4, eid, { case: "tokenPlaced", value: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "Tobbe" } }),
    env(5, eid, { case: "tokenMoved", value: { tokenId: "t1", sceneId: "s1", from: { x: 1, y: 1 }, to: { x: 2, y: 1 } } }),
    env(6, eid, { case: "resourceChanged", value: { actorId: "a1", resource: "stamina", delta: -1, newValue: 4 } }),
    env(7, true, { case: "conditionApplied", value: { actorId: "a1", conditionId: "prone", source: "trip" } }),
    env(8, true, { case: "narrationAdded", value: { text: "Tobbe steps into the light", anchorFromSeq: from, anchorToSeq: to } }),
    env(9, true, { case: "conditionRemoved", value: { actorId: "a1", conditionId: "prone" } }),
  ];
}

function beatOf(d: Drawn, text: string): number {
  return d.beats.findIndex((b) => Array.from(b.querySelectorAll("p")).some((p) => p.textContent === text));
}

for (const role of ROLES) {
  for (const eid of [true, false]) {
    for (const [from, to] of [[4n, 4n], [4n, 6n]] as const) {
      // VTT-812
      test(`${role.name}: a narration anchored to ${from}..${to}, all left out (${eid ? "with" : "without"} event ids), is drawn in a beat of its own at its own place`, () => {
        const d = draw(narrationLog(eid, from, to), role);
        const n = beatOf(d, "Tobbe steps into the light");
        expect(n).toBeGreaterThanOrEqual(0);
        expect(d.beats[n]!.querySelectorAll("p").length).toBe(1);
        const applied = beatOf(d, "Tobbe gained prone");
        const removed = beatOf(d, "Tobbe lost prone");
        expect(applied).toBeGreaterThanOrEqual(0);
        expect(removed).toBeGreaterThanOrEqual(0);
        expect(Math.min(applied, removed) < n && n < Math.max(applied, removed)).toBe(true);
        expect(d.story).toEqual(["session started — Night one", "Tobbe is in the party", "Tobbe gained prone", "Tobbe steps into the light", "Tobbe lost prone"]);
      });
    }
  }
}

// ---- a party member corrected at its first sight, and the run boundaries ----

const MIRA = { actorId: "a3", name: "Mira", kind: ActorKind.PARTY_MEMBER };
function firstSightLog(): Envelope[] {
  return [
    env(1, true, SESSION),
    env(2, false, SCENE),
    env(3, false, { case: "actorAdded", value: { actor: { actorId: "a1", name: "Tobbe", kind: ActorKind.PARTY_MEMBER, resources: { stamina: { current: 5, max: 8 } } } } }),
    env(4, false, { case: "actorControlGranted", value: { actorId: "a1", participantId: "p1", kind: ActorKind.PARTY_MEMBER } }),
    env(5, false, { case: "actorAdded", value: { actor: MIRA } }),
    env(6, false, { case: "tokenPlaced", value: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "Tobbe" } }),
    env(7, true, { case: "tokenMoved", value: { tokenId: "t1", sceneId: "s1", from: { x: 1, y: 1 }, to: { x: 5, y: 1 } } }),
    env(8, false, { case: "resourceAdded", value: { actorId: "a3", resource: "flurry_uses", value: { current: 2, max: 2 } } }),
    env(9, false, { case: "abilityGiven", value: { actorId: "a3", abilityId: "hunters-flurry" } }),
    env(10, false, { case: "tokenPlaced", value: { tokenId: "t3", sceneId: "s1", actorId: "a3", position: { x: 6, y: 1 }, name: "Mira" } }),
  ];
}

/** A bare removal and re-add of Mira, then `between` (event id?, payload), then a bare condition of Mira. */
function runLog(between: [boolean, Payload][]): Envelope[] {
  const head = [
    env(1, true, SESSION),
    env(2, false, { case: "actorAdded", value: { actor: { actorId: "a1", name: "Tobbe", kind: ActorKind.PARTY_MEMBER } } }),
    env(3, false, { case: "actorAdded", value: { actor: MIRA } }),
    env(4, false, { case: "actorRemoved", value: { actorId: "a3" } }),
    env(5, false, { case: "actorAdded", value: { actor: MIRA } }),
  ];
  const mid = between.map(([eid, p], i) => env(6 + i, eid, p));
  return [...head, ...mid, env(6 + mid.length, false, { case: "conditionApplied", value: { actorId: "a3", conditionId: "prone", source: "trip" } })];
}

for (const role of ROLES) {
  // VTT-809 VTT-810 VTT-818 VTT-984
  test(`${role.name}: a party member corrected at its first sight draws no "now has", "can now use" or "is at" line`, () => {
    const d = draw(firstSightLog(), role);
    expect(d.story).toEqual(["session started — Night one", "Tobbe is in the party", "Mira is in the party"]);
    if (role.dm) expect(d.ticker).toContain("Mira now has flurry_uses at 2");
    if (role.dm) expect(d.ticker).toContain("Mira can now use hunters-flurry");
  });

  // VTT-984 VTT-809
  test(`${role.name}: a bare condition behind a run with a correction's sheet frame between is drawn`, () => {
    const d = draw(runLog([
      [false, { case: "actorControlGranted", value: { actorId: "a3", participantId: "p2", kind: ActorKind.PARTY_MEMBER } }],
      [false, { case: "resourceAdded", value: { actorId: "a3", resource: "flurry_uses", value: { current: 2, max: 2 } } }],
    ]), role);
    expect(d.story).toEqual(["session started — Night one", "Tobbe is in the party", "Mira is in the party", "Mira gained prone"]);
  });

  // VTT-984 VTT-810
  test(`${role.name}: a grant with an event id ends the run, and the bare condition behind it is drawn, the grant is not`, () => {
    const d = draw(runLog([
      [true, { case: "actorControlGranted", value: { actorId: "a3", participantId: "p2", kind: ActorKind.PARTY_MEMBER } }],
    ]), role);
    expect(d.story).toEqual(["session started — Night one", "Tobbe is in the party", "Mira is in the party", "Mira gained prone"]);
  });

  // VTT-984
  test(`${role.name}: a run's bare grant and bare condition directly behind it are left out with it`, () => {
    const d = draw(runLog([
      [false, { case: "actorControlGranted", value: { actorId: "a3", participantId: "p2", kind: ActorKind.PARTY_MEMBER } }],
    ]), role);
    expect(d.story).toEqual(["session started — Night one", "Tobbe is in the party", "Mira is in the party"]);
  });
}

// ---- buildFeed directly ----

for (const v of Object.keys(VARIANTS) as Variant[]) {
  // VTT-809 VTT-810 VTT-818 VTT-820 VTT-984 VTT-823
  test(`buildFeed, ${v}: its entries hold every kept event once and no left-out one`, () => {
    const log = worldLog(v);
    const feed = buildFeed(log);
    const held = feed.flatMap((f) => f.events.map((e) => Number(e.sequence)));
    const want = log
      .filter((e) => {
        const k = e.payload.case as Kind;
        if (k === "narrationAdded") return false;
        if (DOORS.includes(k)) return e.eventId !== "";
        return KEPT.has(k);
      })
      .map((e) => Number(e.sequence));
    expect(held).toEqual(want);
    for (const f of feed) for (const e of f.events) expect(LEFT_OUT.map((l) => l.kind)).not.toContain(e.payload.case as Kind);
    expect(feed.flatMap((f) => f.narrations.map((n) => n.text))).toEqual(["The cellar falls quiet"]);
  });
}

for (const eid of [true, false]) {
  // VTT-812
  test(`buildFeed: a narration anchored only to left-out entries (${eid ? "with" : "without"} event ids) is an entry of its own, between its neighbours`, () => {
    const feed = buildFeed(narrationLog(eid, 4n, 6n));
    const at = (seq: number) => feed.findIndex((f) => f.events.some((e) => e.sequence === BigInt(seq)));
    const n = feed.findIndex((f) => f.narrations.some((x) => x.text === "Tobbe steps into the light"));
    expect(n).toBeGreaterThanOrEqual(0);
    expect(feed[n]!.events).toEqual([]);
    expect(at(7)).toBeLessThan(n);
    expect(n).toBeLessThan(at(9));
  });
}

// VTT-809 VTT-818
test("buildFeed: a party member corrected at its first sight leaves no sheet or board entry", () => {
  const kinds = buildFeed(firstSightLog()).flatMap((f) => f.events.map((e) => e.payload.case));
  expect(kinds).toEqual(["sessionStarted", "actorAdded", "actorAdded"]);
});

// ---- more of what stays ----

for (const role of ROLES) {
  // VTT-812 VTT-820
  test(`${role.name}: a narration anchored only to a door with no event id is drawn in a beat of its own`, () => {
    const log = [
      env(1, true, SESSION),
      env(2, true, SCENE),
      env(3, false, { case: "doorOpened", value: { sceneId: "s1", at: { x: 4, y: 5 } } }),
      env(4, true, { case: "narrationAdded", value: { text: "The door creaks", anchorFromSeq: 3n, anchorToSeq: 3n } }),
      env(5, true, { case: "sessionEnded", value: {} }),
    ];
    const d = draw(log, role);
    expect(d.story).toEqual(["session started — Night one", "The door creaks", "session ended"]);
    expect(d.beats[beatOf(d, "The door creaks")]!.querySelectorAll("p").length).toBe(1);
  });

  // VTT-984
  test(`${role.name}: a narration anchored over a kept entry and left-out ones is drawn once`, () => {
    const d = draw(narrationLog(true, 3n, 6n), role);
    expect(d.story.filter((l) => l === "Tobbe steps into the light").length).toBe(1);
    expect(d.story.filter((l) => l === "Tobbe is in the party").length).toBe(1);
    expect(d.story.some((l) => l.includes("moved to") || l.includes(" is at ") || l.includes("→"))).toBe(false);
  });

  // VTT-984
  test(`${role.name}: a removal that carries an event id is drawn, and the same actor's add behind it`, () => {
    const log = [
      env(1, true, SESSION),
      env(2, true, { case: "actorAdded", value: { actor: MIRA } }),
      env(3, true, { case: "actorRemoved", value: { actorId: "a3" } }),
      env(4, false, { case: "actorAdded", value: { actor: MIRA } }),
    ];
    const d = draw(log, role);
    expect(d.story).toEqual(["session started — Night one", "Mira is in the party", "Mira left", "Mira is in the party"]);
  });

  // VTT-984
  test(`${role.name}: a bare removal followed by another actor's add is drawn, and that add`, () => {
    const log = [
      env(1, true, SESSION),
      env(2, false, { case: "actorAdded", value: { actor: MIRA } }),
      env(3, false, { case: "actorRemoved", value: { actorId: "a3" } }),
      env(4, false, { case: "actorAdded", value: { actor: { actorId: "a1", name: "Tobbe", kind: ActorKind.PARTY_MEMBER } } }),
    ];
    const d = draw(log, role);
    expect(d.story).toEqual(["session started — Night one", "Mira is in the party", "Mira left", "Tobbe is in the party"]);
  });

  // VTT-984
  test(`${role.name}: an ability's use, a roll and a whisper read as today`, () => {
    const d = draw(worldLog("no entry carries one"), role);
    expect(d.story).toContain("Tobbe used hunters-flurry on Goblin: hit, rolled 14");
    expect(d.story).toContain("The DM whispers to Tobbe: you hear something");
    expect(d.story).toContain("Goblin gained prone");
    expect(d.story).toContain("Goblin lost prone");
    expect(d.story).toContain("Goblin left");
    expect(d.story).toContain("Goblin comes into sight");
  });
}
