import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fromJson, fromJsonString, type JsonValue } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { ClientCommandSchema, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { fold } from "../src/fold";
import { blocksMove } from "../src/view/terrain";
import { playerClickFor, type PlayerUIState } from "../src/view/player";
import type { Me } from "../src/metadata";
import type { State } from "../src/state";
import { boardCamera, CELL } from "../src/view/spectator";
import { boot } from "../src/app";
import { settle } from "./support/answers";

// Independent QA for VTT-935 to VTT-940, beside VTT-408, VTT-885 and
// VTT-934. Every expected value is derived from those rows, the owner's
// ruling of 2026-11-21 and what SPEC-013's move gate asks of
// engine.State.Blocked, never from the client's code or the shared fixture.
// "Done n" below is item n of "Done looks like" in the ticket under
// docs/superpowers/specs/ named
// 2026-11-21-a-click-moves-only-onto-a-seen-open-square-design.md.

type Scene = NonNullable<State["Scenes"][string]>;

interface Pos { x?: number; y?: number }
interface RawTile { kind: string }
interface RawObject {
  objectId: string; kind: string; at: Pos; width: number; height: number;
  rotationDegrees?: number; blocksSight?: boolean; blocksMove?: boolean;
}
interface RawScene {
  sceneId: string; name: string; gridWidth: number; gridHeight: number;
  tiles?: Record<string, RawTile>; objects?: RawObject[];
}
interface RawDoor { sceneId: string; at: Pos }
type RawEntry = {
  sceneCreated?: RawScene; doorOpened?: RawDoor; doorClosed?: RawDoor;
} & Record<string, unknown>;

function log(payloads: readonly Record<string, unknown>[]): RawEntry[] {
  return payloads.map((p, i) => ({
    eventId: `evt-${i + 1}`, sequence: String(i + 1), sessionId: "sess-qa",
    actorRole: "dm", participantId: "p-dm", ...p,
  }));
}

function envelopes(raw: readonly RawEntry[]): Envelope[] {
  return raw.map((r) => fromJson(EnvelopeSchema, r as unknown as JsonValue));
}

function sceneOf(st: State, id: string): Scene {
  const scene = st.Scenes[id];
  if (!scene) throw new Error(`the fold holds no scene ${id}`);
  return scene;
}

// ---- "Blocked" as VTT-936 and SPEC-013's move gate mean it, over the raw log

function rawScene(raw: readonly RawEntry[], id: string): RawScene {
  const scene = raw.map((r) => r.sceneCreated).find((s) => s?.sceneId === id);
  if (!scene) throw new Error(`the log creates no scene ${id}`);
  return scene;
}

function covers(o: RawObject, x: number, y: number): boolean {
  const ax = o.at.x ?? 0;
  const ay = o.at.y ?? 0;
  return x >= ax && x < ax + o.width && y >= ay && y < ay + o.height;
}

function doorHistory(raw: readonly RawEntry[], id: string, x: number, y: number): string[] {
  const out: string[] = [];
  for (const r of raw) {
    const d = r.doorOpened ?? r.doorClosed;
    if (!d || d.sceneId !== id || (d.at.x ?? 0) !== x || (d.at.y ?? 0) !== y) continue;
    out.push(r.doorOpened ? "opened" : "closed");
  }
  return out;
}

function derivedBlocked(raw: readonly RawEntry[], id: string, x: number, y: number): boolean {
  const scene = rawScene(raw, id);
  const kind = scene.tiles?.[`${x},${y}`]?.kind;
  if (kind === "wall") return true;
  if (kind === "door" && doorHistory(raw, id, x, y).at(-1) !== "opened") return true;
  return (scene.objects ?? []).some((o) => o.blocksMove === true && covers(o, x, y));
}

// ---- The barrow: a 9x6 scene, created before a lower-sorting one ----------

const W = 9;
const H = 6;
const WALLS = ["1,1", "0,4"];
const DOORS = ["6,0", "7,0", "8,0", "2,5"];
const NO_TILE = ["6,2", "8,3", "0,0"];

function barrowTiles(): Record<string, RawTile> {
  const tiles: Record<string, RawTile> = {};
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      const k = `${x},${y}`;
      if (NO_TILE.includes(k)) continue;
      tiles[k] = { kind: WALLS.includes(k) ? "wall" : DOORS.includes(k) ? "door" : "floor" };
    }
  }
  return tiles;
}

function tilesAt(keys: readonly string[]): Record<string, RawTile> {
  const all = barrowTiles();
  const out: Record<string, RawTile> = {};
  for (const k of keys) {
    const t = all[k];
    if (t) out[k] = t;
  }
  return out;
}

const OBJECTS: RawObject[] = [
  // Columns 3 to 6, rows 2 to 4: wider than tall, interior at 4,3 and 5,3.
  { objectId: "cairn", kind: "cairn", at: { x: 3, y: 2 }, width: 4, height: 3, blocksMove: true },
  // Rotation is not read: it covers 0,2 and 1,2, never 0,3.
  {
    objectId: "bench", kind: "bench", at: { x: 0, y: 2 }, width: 2, height: 1,
    rotationDegrees: 90, blocksMove: true,
  },
  { objectId: "statue", kind: "statue", at: { x: 8, y: 4 }, width: 1, height: 1, blocksMove: true },
  { objectId: "veil", kind: "veil", at: { x: 7, y: 4 }, width: 1, height: 2, blocksSight: true },
];

const SEEN = [
  "1,3", "2,3", "7,3", "4,1", "4,5", "3,2", "4,3", "5,3", "6,4", "6,2", "3,4",
  "1,1", "6,0", "7,0", "8,0", "8,3", "7,4", "8,4", "1,2", "0,3", "2,5", "5,5",
];
const REMEMBERED = ["0,5", "8,5"];

const party = (actorId: string, name: string, abilityIds: string[] = []) => ({
  actorAdded: { actor: { actorId, name, kind: "ACTOR_KIND_PARTY_MEMBER", abilityIds } },
});
const grant = (actorId: string, participantId: string) => ({
  actorControlGranted: { actorId, participantId, kind: "ACTOR_KIND_PARTY_MEMBER" },
});
const place = (tokenId: string, sceneId: string, actorId: string, x: number, y: number) => ({
  tokenPlaced: { tokenId, sceneId, actorId, position: { x, y } },
});
const door = (verb: "doorOpened" | "doorClosed", x: number, y: number) => ({
  [verb]: { sceneId: "s-b", at: { x, y } },
});

const BARROW_PAYLOADS: Record<string, unknown>[] = [
  { sessionStarted: { name: "Night" } },
  {
    sceneCreated: {
      sceneId: "s-b", name: "Barrow", gridWidth: W, gridHeight: H,
      tiles: barrowTiles(), objects: OBJECTS,
    },
  },
  {
    sceneCreated: {
      sceneId: "s-a", name: "Antechamber", gridWidth: 4, gridHeight: 4,
      tiles: { "2,3": { kind: "floor" }, "1,1": { kind: "floor" } },
    },
  },
  party("a-mo", "Mo", ["second-wind"]), party("a-ann", "Ann"), party("a-zed", "Zed"),
  party("a-cy", "Cy"), party("a-nil", "Nil"),
  grant("a-mo", "p-1"), grant("a-ann", "p-1"), grant("a-zed", "p-1"), grant("a-nil", "p-1"),
  grant("a-cy", "p-2"),
  place("t-7", "s-b", "a-mo", 1, 3),
  place("t-ann", "s-b", "a-ann", 2, 5),
  place("t-zed", "s-b", "a-zed", 8, 5),
  place("t-cy", "s-b", "a-cy", 5, 5),
  place("t-old", "s-a", "a-zed", 2, 3),
  door("doorOpened", 7, 0),
  door("doorOpened", 8, 0),
  door("doorClosed", 8, 0),
  { sceneSeen: { sceneId: "s-b", visible: [...REMEMBERED, "1,3"], tiles: tilesAt([...REMEMBERED, "1,3"]) } },
  { sceneSeen: { sceneId: "s-b", visible: SEEN, tiles: tilesAt(SEEN) } },
];

const BARROW = log(BARROW_PAYLOADS);
const BARROW_NO_VISIBLE = log(BARROW_PAYLOADS.filter((p) => !("sceneSeen" in p)));

const ME: Me = { participantId: "p-1", name: "Pia", role: "player" };
const WATCHABLE = ["a-ann", "a-mo", "a-zed"];

function ui(over: Partial<PlayerUIState> = {}): PlayerUIState {
  return { selectedActorId: "a-mo", selectedAbilityId: "", doorsArmed: false, ...over };
}

type Seen =
  | null
  | { case: "moveToken"; tokenId: string; to: { x: number; y: number } }
  | { case: "setViewpoint"; actorId: string }
  | { case: string };

function summary(c: ClientCommand | null): Seen {
  if (c === null) return null;
  const v = c.command;
  if (v.case === "moveToken") {
    return { case: "moveToken", tokenId: v.value.tokenId, to: { x: v.value.to?.x ?? 0, y: v.value.to?.y ?? 0 } };
  }
  if (v.case === "setViewpoint") return { case: "setViewpoint", actorId: v.value.actorId };
  return { case: String(v.case) };
}

const MOVE = (x: number, y: number): Seen => ({ case: "moveToken", tokenId: "t-7", to: { x, y } });
const SWITCH = (actorId: string): Seen => ({ case: "setViewpoint", actorId });

// What VTT-935, VTT-936, VTT-939, VTT-940 and principle 14 of SPEC-022 say a
// player's click on x,y answers, watching a-mo with WATCHABLE listed.
function derivedClick(raw: readonly RawEntry[], visible: readonly string[] | null, x: number, y: number): Seen {
  const k = `${x},${y}`;
  if (visible !== null && !visible.includes(k)) return null;
  const here = (raw.map((r) => r.tokenPlaced) as ({ tokenId: string; sceneId: string; actorId: string; position: Pos } | undefined)[])
    .filter((t) => t !== undefined && t.sceneId === "s-b" && (t.position.x ?? 0) === x && (t.position.y ?? 0) === y);
  const others = here.map((t) => t!.actorId).filter((a) => WATCHABLE.includes(a) && a !== "a-mo").sort();
  if (others.length > 0) return SWITCH(others[0]!);
  if (here.some((t) => t!.actorId === "a-mo")) return null;
  if (derivedBlocked(raw, "s-b", x, y)) return null;
  return MOVE(x, y);
}

// ---- blocksMove -----------------------------------------------------------

const BLOCKS: [string, number, number, boolean][] = [
  ["a wall", 1, 1, true],
  ["a second wall", 0, 4, true],
  ["a floor", 2, 2, false],
  ["a square with no tile", 8, 3, false],
  ["a square with no tile at the origin", 0, 0, false],
  ["a door never opened", 6, 0, true],
  ["a door never opened, a token standing in it", 2, 5, true],
  ["a door opened", 7, 0, false],
  ["a door opened, then closed", 8, 0, true],
  ["the cairn's anchor", 3, 2, true],
  ["the cairn's interior", 4, 3, true],
  ["the cairn's interior, next column", 5, 3, true],
  ["the cairn's top edge", 4, 2, true],
  ["the cairn's top edge, next column", 5, 2, true],
  ["the cairn's top-right corner, with no tile", 6, 2, true],
  ["the cairn's left edge", 3, 3, true],
  ["the cairn's right edge", 6, 3, true],
  ["the cairn's bottom-left corner", 3, 4, true],
  ["the cairn's bottom edge", 4, 4, true],
  ["the cairn's bottom edge, next column", 5, 4, true],
  ["the cairn's far corner", 6, 4, true],
  ["just left of the cairn", 2, 3, false],
  ["just left of the cairn, its top row", 2, 2, false],
  ["just left of the cairn, its bottom row", 2, 4, false],
  ["just right of the cairn", 7, 3, false],
  ["just right of the cairn, its top row", 7, 2, false],
  ["just above the cairn", 4, 1, false],
  ["just above the cairn's anchor", 3, 1, false],
  ["just above the cairn's last column", 6, 1, false],
  ["just below the cairn", 4, 5, false],
  ["just below the cairn's anchor", 3, 5, false],
  ["just below the cairn's last column", 6, 5, false],
  ["the rotated bench's anchor", 0, 2, true],
  ["the rotated bench's second column", 1, 2, true],
  ["where the bench would reach if its rotation were read", 0, 3, false],
  ["a 1x1 statue", 8, 4, true],
  ["a sight-only veil", 7, 4, false],
  ["a sight-only veil's second row", 7, 5, false],
];

const barrow = sceneOf(fold(envelopes(BARROW)), "s-b");

test("blocksMove answers each kind of square as the definition of blocked does", () => {
  for (const [name, x, y, want] of BLOCKS) {
    // VTT-936 VTT-937; SPEC-013's move gate
    expect({ square: name, blocked: blocksMove(barrow, { x, y }) }).toEqual({ square: name, blocked: want });
  }
});

test("blocksMove agrees with the definition on every square of a scene that is not square", () => {
  for (const [name, x, y, want] of BLOCKS) {
    expect({ square: name, derived: derivedBlocked(BARROW, "s-b", x, y) }).toEqual({ square: name, derived: want });
  }
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      // VTT-936; SPEC-013's move gate
      expect({ x, y, blocked: blocksMove(barrow, { x, y }) }).toEqual({ x, y, blocked: derivedBlocked(BARROW, "s-b", x, y) });
    }
  }
});

test("blocksMove reads a door's last state and blocks nothing on a scene with no terrain", () => {
  const raw = log([
    { sceneCreated: { sceneId: "s-b", name: "Hall", gridWidth: 3, gridHeight: 2, tiles: { "1,0": { kind: "door" } } } },
    { sceneCreated: { sceneId: "s-c", name: "Canvas", gridWidth: 3, gridHeight: 2 } },
    door("doorOpened", 1, 0), door("doorClosed", 1, 0), door("doorOpened", 1, 0),
  ]);
  const st = fold(envelopes(raw));
  // VTT-936: a door the window holds closed blocks; this one is open again
  expect(blocksMove(sceneOf(st, "s-b"), { x: 1, y: 0 })).toBe(false);
  for (let y = 0; y < 2; y++) {
    for (let x = 0; x < 3; x++) {
      // VTT-936 VTT-939: a square with no tile is no wall, door or scenery
      expect({ x, y, blocked: blocksMove(sceneOf(st, "s-c"), { x, y }) }).toEqual({ x, y, blocked: false });
    }
  }
});

const fixtureLog = JSON.parse(
  readFileSync(new URL("../../contract/testdata/blocked_squares_log.json", import.meta.url), "utf8"),
) as RawEntry[];
const fixture = JSON.parse(
  readFileSync(new URL("../../contract/testdata/blocked_squares.json", import.meta.url), "utf8"),
) as { sceneId: string; squares: { case: string; x: number; y: number; blocked: boolean }[] };

test("the shared fixture's answers agree with the definition of blocked, case by case", () => {
  const scene = rawScene(fixtureLog, fixture.sceneId);
  const objects = scene.objects ?? [];
  const blockers = objects.filter((o) => o.blocksMove === true);
  const any = (f: (ax: number, ay: number, w: number, h: number) => boolean) =>
    blockers.some((o) => f(o.at.x ?? 0, o.at.y ?? 0, o.width, o.height));
  // Rotation is not read: a turned blocker two wide still covers its anchor and
  // the square right of it, and a quarter turn would instead cover the square
  // below the anchor.
  const turnedPairs = blockers.filter((o) =>
    o.width === 2 && o.height === 1 && (o.rotationDegrees ?? 0) % 180 !== 0);
  const turned = (f: (ax: number, ay: number) => boolean) =>
    turnedPairs.some((o) => f(o.at.x ?? 0, o.at.y ?? 0));
  const holds = (c: { case: string; x: number; y: number }): boolean => {
    const { x, y } = c;
    const kind = scene.tiles?.[`${x},${y}`]?.kind;
    const history = doorHistory(fixtureLog, fixture.sceneId, x, y).join();
    const bare = !objects.some((o) => covers(o, x, y));
    switch (c.case) {
      case "a wall": return kind === "wall";
      case "a floor": return kind === "floor" && bare;
      case "a square with no tile": return kind === undefined && bare;
      case "a door never opened": return kind === "door" && history === "";
      case "a door opened": return kind === "door" && history === "opened";
      case "a door opened, then closed": return kind === "door" && history === "opened,closed";
      case "the anchor of scenery that blocks movement":
        return any((ax, ay) => x === ax && y === ay);
      case "the far corner of scenery that blocks movement":
        return any((ax, ay, w, h) => x === ax + w - 1 && y === ay + h - 1);
      case "scenery that blocks movement, its anchor's row, last column":
        return any((ax, ay, w) => x === ax + w - 1 && y === ay);
      case "scenery that blocks movement, its anchor's column, last row":
        return any((ax, ay, _w, h) => x === ax && y === ay + h - 1);
      case "the floor just past the scenery's right edge":
        return kind === "floor" && any((ax, ay, w, h) => x === ax + w && y >= ay && y < ay + h);
      case "the floor just past the scenery's bottom edge":
        return kind === "floor" && any((ax, ay, w, h) => y === ay + h && x >= ax && x < ax + w);
      case "the floor just past the scenery's left edge":
        return kind === "floor" && any((ax, ay, _w, h) => x === ax - 1 && y >= ay && y < ay + h);
      case "the floor just past the scenery's top edge":
        return kind === "floor" && any((ax, ay, w) => y === ay - 1 && x >= ax && x < ax + w);
      case "scenery that blocks sight and not movement":
        return objects.some((o) => o.blocksSight === true && o.blocksMove !== true && covers(o, x, y));
      case "the anchor of rotated scenery two squares wide that blocks movement":
        return turned((ax, ay) => x === ax && y === ay);
      case "the second column of that rotated scenery, over a square with no tile":
        return kind === undefined && turned((ax, ay) => x === ax + 1 && y === ay);
      case "the square below that scenery's anchor, which reading its rotation would cover":
        return turned((ax, ay) => x === ax && y === ay + 1);
      case "an opened door under scenery that blocks movement":
        return kind === "door" && history.endsWith("opened")
          && blockers.some((o) => covers(o, x, y));
      default: return false;
    }
  };
  expect(fixture.squares.length).toBeGreaterThan(0);
  for (const c of fixture.squares) {
    // VTT-937; SPEC-013's move gate
    expect({ case: c.case, holds: holds(c), blocked: c.blocked })
      .toEqual({ case: c.case, holds: true, blocked: derivedBlocked(fixtureLog, fixture.sceneId, c.x, c.y) });
  }
  const cases = new Set(fixture.squares.map((c) => c.case));
  for (const kind of [
    "a wall", "a floor", "a square with no tile", "a door never opened", "a door opened",
    "a door opened, then closed", "the anchor of scenery that blocks movement",
    "scenery that blocks sight and not movement",
    "the anchor of rotated scenery two squares wide that blocks movement",
    "an opened door under scenery that blocks movement",
  ]) {
    expect({ kind, listed: cases.has(kind) }).toEqual({ kind, listed: true });
  }
});

test("blocksMove answers every square the shared fixture lists as that fixture says", () => {
  const scene = sceneOf(fold(envelopes(fixtureLog)), fixture.sceneId);
  for (const c of fixture.squares) {
    // VTT-937
    expect({ case: c.case, blocked: blocksMove(scene, { x: c.x, y: c.y }) }).toEqual({ case: c.case, blocked: c.blocked });
  }
});

// ---- playerClickFor ---------------------------------------------------------

const CLICKS: [string, number, number, Seen][] = [
  ["a seen floor just left of the cairn, below a listed member's column", 2, 3, MOVE(2, 3)],
  ["a seen floor just right of the cairn", 7, 3, MOVE(7, 3)],
  ["a seen floor just above the cairn", 4, 1, MOVE(4, 1)],
  ["a seen floor just below the cairn", 4, 5, MOVE(4, 5)],
  ["the seen cairn's anchor", 3, 2, null],
  ["the seen cairn's interior", 4, 3, null],
  ["the seen cairn's interior, next column", 5, 3, null],
  ["the seen cairn's far corner", 6, 4, null],
  ["the seen cairn's top-right corner, with no tile", 6, 2, null],
  ["the seen cairn's bottom-left corner", 3, 4, null],
  ["a seen wall", 1, 1, null],
  ["a seen door never opened", 6, 0, null],
  ["a seen door opened", 7, 0, MOVE(7, 0)],
  ["a seen door opened, then closed", 8, 0, null],
  ["a seen square with no tile", 8, 3, MOVE(8, 3)],
  ["a seen square under sight-only scenery", 7, 4, MOVE(7, 4)],
  ["a seen 1x1 statue", 8, 4, null],
  ["the seen rotated bench's second column", 1, 2, null],
  ["a seen floor the bench would reach if its rotation were read", 0, 3, MOVE(0, 3)],
  ["a listed party member standing in a seen closed door", 2, 5, SWITCH("a-ann")],
  ["an unlisted party member's token on a seen floor", 5, 5, MOVE(5, 5)],
  ["the watched one's own square", 1, 3, null],
  ["a remembered floor", 0, 5, null],
  ["a listed party member's token on a remembered square", 8, 5, null],
  ["a never-seen square with no tile", 0, 0, null],
  ["a never-seen floor", 8, 1, null],
  ["a never-seen wall", 0, 4, null],
];

const st = fold(envelopes(BARROW));
const stNoVisible = fold(envelopes(BARROW_NO_VISIBLE));

test("the fold holds the barrow's Visible and Explored as its two sceneSeen entries say", () => {
  const scene = sceneOf(st, "s-b");
  expect(Object.keys(scene.Visible ?? {}).sort()).toEqual([...SEEN].sort());
  for (const k of REMEMBERED) expect({ k, explored: scene.Explored?.[k] === true }).toEqual({ k, explored: true });
  expect(sceneOf(stNoVisible, "s-b").Visible).toBeUndefined();
});

test("a player's click on each square of a scene with a Visible answers by what is seen now and held there", () => {
  for (const [name, x, y, want] of CLICKS) {
    // VTT-935 VTT-936 VTT-939 VTT-940 VTT-408; SPEC-022 principle 14
    expect({ click: name, sent: summary(playerClickFor(st, ME, ui(), WATCHABLE, { x, y })) })
      .toEqual({ click: name, sent: want });
  }
});

test("every square of the barrow answers a player's click as the rows derive it", () => {
  for (const [name, x, y, want] of CLICKS) {
    expect({ click: name, derived: derivedClick(BARROW, SEEN, x, y) }).toEqual({ click: name, derived: want });
  }
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      // VTT-935 VTT-936 VTT-939 VTT-940
      expect({ x, y, sent: summary(playerClickFor(st, ME, ui(), WATCHABLE, { x, y })) })
        .toEqual({ x, y, sent: derivedClick(BARROW, SEEN, x, y) });
    }
  }
});

test("with doors armed a player's click answers as unarmed and works no door anywhere", () => {
  for (const [name, x, y, want] of CLICKS) {
    // VTT-938; SPEC-022 "whatever the window's arming bit holds"
    expect({ click: name, sent: summary(playerClickFor(st, ME, ui({ doorsArmed: true }), WATCHABLE, { x, y })) })
      .toEqual({ click: name, sent: want });
  }
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      for (const s of [st, stNoVisible]) {
        const c = summary(playerClickFor(s, ME, ui({ doorsArmed: true }), WATCHABLE, { x, y }));
        // VTT-938
        expect({ x, y, door: c !== null && (c.case === "openDoor" || c.case === "closeDoor") })
          .toEqual({ x, y, door: false });
      }
    }
  }
});

test("a selection the watched one's sheet does not name is not armed, so each click answers as with none", () => {
  for (const [name, x, y, want] of CLICKS) {
    // VTT-939; SPEC-022: "a selection the watched one's sheet does not name
    // is not armed"
    expect({ click: name, sent: summary(playerClickFor(st, ME, ui({ selectedAbilityId: "ghost-step" }), WATCHABLE, { x, y })) })
      .toEqual({ click: name, sent: want });
  }
});

test("a scene the window holds no Visible for reads every square as seen", () => {
  const table: [string, number, number, Seen][] = [
    ["a floor never in a sceneSeen", 8, 1, MOVE(8, 1)],
    ["a floor a sceneSeen would remember", 0, 5, MOVE(0, 5)],
    ["a square with no tile", 0, 0, MOVE(0, 0)],
    ["a listed party member's token", 8, 5, SWITCH("a-zed")],
    ["a listed party member standing in a closed door", 2, 5, SWITCH("a-ann")],
    ["a wall", 0, 4, null],
    ["a door never opened", 6, 0, null],
    ["a door opened", 7, 0, MOVE(7, 0)],
    ["the cairn's interior", 5, 3, null],
    ["the watched one's own square", 1, 3, null],
  ];
  for (const [name, x, y, want] of table) {
    // VTT-939 VTT-940; the owner's ruling of 2026-11-21: a scene with no
    // Visible reads as every square seen
    expect({ click: name, sent: summary(playerClickFor(stNoVisible, ME, ui(), WATCHABLE, { x, y })) })
      .toEqual({ click: name, sent: want });
  }
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      expect({ x, y, sent: summary(playerClickFor(stNoVisible, ME, ui(), WATCHABLE, { x, y })) })
        .toEqual({ x, y, sent: derivedClick(BARROW_NO_VISIBLE, null, x, y) });
    }
  }
});

test("a scene whose Visible names no square answers every click with nothing", () => {
  const blind: State = { ...st, Scenes: { ...st.Scenes, "s-b": { ...sceneOf(st, "s-b"), Visible: {} } } };
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      // VTT-935
      expect({ x, y, sent: summary(playerClickFor(blind, ME, ui(), WATCHABLE, { x, y })) })
        .toEqual({ x, y, sent: null });
    }
  }
});

test("a player watching a party member with no token moves no other token", () => {
  const watchable = ["a-ann", "a-mo", "a-nil", "a-zed"];
  const watchNil = ui({ selectedActorId: "a-nil" });
  // VTT-408: a click that moves a token moves the watched one's
  expect(summary(playerClickFor(st, ME, watchNil, watchable, { x: 2, y: 3 }))?.case ?? null).not.toBe("moveToken");
  // VTT-940: a-mo is listed and not watched now
  expect(summary(playerClickFor(st, ME, watchNil, watchable, { x: 1, y: 3 }))).toEqual(SWITCH("a-mo"));
});

// ---- Through the window -----------------------------------------------------

const TOKEN = "qa-seen-open-square-token";
const g = globalThis as unknown as Record<string, unknown>;
const savedWebSocket = g["WebSocket"];
const savedFetch = g["fetch"];
let roots: HTMLElement[] = [];
let sockets: FakeSocket[] = [];

class FakeSocket {
  static readonly OPEN = 1;
  readyState = 1;
  sent: string[] = [];
  onopen: ((e: unknown) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: unknown) => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  constructor(readonly url: string) {
    sockets.push(this);
    setTimeout(() => this.onopen?.({}), 0);
  }
  send(s: string): void { this.sent.push(s); }
  close(): void {}
  deliver(frame: unknown): void { this.onmessage?.({ data: JSON.stringify(frame) }); }
}

afterEach(() => {
  g["WebSocket"] = savedWebSocket;
  g["fetch"] = savedFetch;
  for (const r of roots) r.remove();
  roots = [];
  sockets = [];
  localStorage.removeItem("vtt.token");
  sessionStorage.removeItem(`vtt.viewpoint.${TOKEN}`);
});

async function openWindow(me: Me, frames: unknown[]): Promise<{ root: HTMLElement; sock: FakeSocket }> {
  const json = (b: unknown) => new Response(JSON.stringify(b), { status: 200, headers: { "content-type": "application/json" } });
  g["fetch"] = async (input: unknown) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url.endsWith("/api/me")) return json(me);
    if (url.endsWith("/api/ruleset")) return json({ id: "r", name: "R", abilities: [], conditions: [], resources: [] });
    if (url.endsWith("/api/adventures")) return json({ adventures: [] });
    return new Response("not found", { status: 404 });
  };
  g["WebSocket"] = FakeSocket;
  localStorage.setItem("vtt.token", TOKEN);
  const root = document.createElement("div");
  document.body.appendChild(root);
  roots.push(root);
  boot(root);
  for (let i = 0; i < 5 && sockets.length === 0; i++) await settle();
  await settle();
  const sock = sockets[0];
  if (!sock) throw new Error("the window opened no socket");
  for (const f of frames) sock.deliver(f);
  await settle();
  return { root, sock };
}

async function click(w: { root: HTMLElement; sock: FakeSocket }, x: number, y: number): Promise<ClientCommand[]> {
  const cam = boardCamera(W, H);
  const grid = w.root.querySelector(".grid");
  if (!grid) throw new Error("the window draws no .grid");
  const before = w.sock.sent.length;
  grid.dispatchEvent(new MouseEvent("click", {
    bubbles: true,
    clientX: cam.offsetX + (x + 0.5) * CELL * cam.scale,
    clientY: cam.offsetY + (y + 0.5) * CELL * cam.scale,
  }));
  await settle();
  const fresh = w.sock.sent.slice(before).map((s) => fromJsonString(ClientCommandSchema, s));
  for (const f of fresh) w.sock.deliver({ result: { requestId: f.requestId, ok: true } });
  await settle();
  return fresh;
}

test("a player's window sends nothing for an unseen or a blocked seen square and one move for an open seen one", async () => {
  const w = await openWindow(ME, [
    { watchableChanged: { actors: [{ actorId: "a-ann", name: "Ann" }, { actorId: "a-mo", name: "Mo" }, { actorId: "a-zed", name: "Zed" }] } },
    { viewpointChanged: { actorId: "a-mo" } },
    ...BARROW.map((e) => ({ event: e })),
  ]);
  // VTT-885
  expect(w.root.querySelectorAll('[data-action="arm-doors"]')).toHaveLength(0);

  const quiet: [string, number, number][] = [
    ["a remembered floor", 0, 5],
    ["a never-seen floor", 8, 1],
    ["a seen wall", 1, 1],
    ["a seen door never opened", 6, 0],
    ["the seen cairn's interior", 4, 3],
    ["a listed party member's token on a remembered square", 8, 5],
  ];
  for (const [name, x, y] of quiet) {
    const sent = await click(w, x, y);
    // VTT-935 VTT-936 VTT-938; Done 2, 3 and 5; the owner's ruling of
    // 2026-11-21: "no command and no warning"
    expect({ click: name, sent: sent.length, toasts: document.querySelectorAll(".toast").length })
      .toEqual({ click: name, sent: 0, toasts: 0 });
    expect(sent).toHaveLength(0);
  }

  const moved = await click(w, 2, 3);
  // VTT-939 VTT-408; Done 4
  expect(moved).toHaveLength(1);
  expect(summary(moved[0]!)).toEqual(MOVE(2, 3));
  expect(document.querySelectorAll(".toast")).toHaveLength(0);

  w.sock.deliver({
    event: {
      eventId: "evt-open", sequence: String(BARROW.length + 1), sessionId: "sess-qa", actorRole: "dm",
      participantId: "p-dm", ...door("doorOpened", 6, 0),
    },
  });
  await settle();
  const through = await click(w, 6, 0);
  // VTT-938 VTT-939
  expect(through).toHaveLength(1);
  expect(summary(through[0]!)).toEqual(MOVE(6, 0));

  const cam = boardCamera(W, H);
  const grid = w.root.querySelector(".grid")!;
  const before = w.sock.sent.length;
  grid.dispatchEvent(new MouseEvent("click", {
    bubbles: true, clientX: cam.offsetX + 2.5 * CELL * cam.scale, clientY: cam.offsetY + (H + 0.25) * CELL * cam.scale,
  }));
  await settle();
  // VTT-934; Done 1
  expect(w.sock.sent.slice(before)).toHaveLength(0);
  expect(document.querySelectorAll(".toast")).toHaveLength(0);
});

test("a DM's window moves its selected actor onto a wall, scenery and a closed door, doors unarmed", async () => {
  const dm: Me = { participantId: "p-dm", name: "Dee", role: "dm" };
  const raw = log([
    { sessionStarted: { name: "Night" } },
    BARROW_PAYLOADS[1]!,
    { actorAdded: { actor: { actorId: "a-pet", name: "Pet", kind: "ACTOR_KIND_NON_PARTY" } } },
    { actorControlGranted: { actorId: "a-pet", participantId: "p-dm", kind: "ACTOR_KIND_NON_PARTY" } },
    place("t-pet", "s-b", "a-pet", 2, 3),
  ]);
  const w = await openWindow(dm, raw.map((e) => ({ event: e })));
  // A DM's window draws the arm-doors control, so the selector the VTT-885
  // check above uses is not vacuous.
  expect(w.root.querySelectorAll('[data-action="arm-doors"]')).toHaveLength(1);
  const chip = Array.from(w.root.querySelectorAll<HTMLButtonElement>("button.chip")).find((b) => b.textContent === "Pet");
  if (!chip) throw new Error("the DM's window draws no chip for Pet");
  chip.click();
  await settle();
  const table: [string, number, number][] = [
    ["a wall", 1, 1],
    ["the cairn's interior", 4, 3],
    ["a door never opened", 6, 0],
  ];
  for (const [name, x, y] of table) {
    const sent = await click(w, x, y);
    // Done 6; the owner's ruling of 2026-11-21: "A DM's and an agent's
    // window are not changed by these rows inside the grid"
    expect({ click: name, sent: sent.map(summary) })
      .toEqual({ click: name, sent: [{ case: "moveToken", tokenId: "t-pet", to: { x, y } }] });
  }
});
