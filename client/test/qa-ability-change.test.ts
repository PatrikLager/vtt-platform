import { expect, test } from "bun:test";
import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";
import { namer } from "../src/view/naming";
import { describe as wordsOf } from "../src/view/spectator";

const WORLD = `[
  {"eventId": "w1", "sequence": "1", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sessionStarted": {"name": "Night"}},
  {"eventId": "w2", "sequence": "2", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sceneCreated": {"sceneId": "scn-1", "name": "Hall", "gridWidth": 3, "gridHeight": 1,
      "tiles": {"0,0": {"kind": "floor", "material": "stone"}, "1,0": {"kind": "door", "material": "oak"},
        "2,0": {"kind": "floor", "material": "stone"}}}},
  {"eventId": "w3", "sequence": "3", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-a", "name": "Aster", "moduleId": "mod-x",
      "attributes": {"might": 3, "wit": 2}, "resources": {"stamina": {"current": 4, "max": 6}},
      "moduleData": {"tone": "calm"}, "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["mu", "kappa", "mu", "alpha", "mu"]}}},
  {"eventId": "w4", "sequence": "4", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-b", "name": "Bram", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["kappa", "mu"]}}},
  {"eventId": "w5", "sequence": "5", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-c", "name": "Crag", "kind": "ACTOR_KIND_NON_PARTY",
      "abilityIds": ["claw"]}}},
  {"eventId": "w6", "sequence": "6", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-d", "name": "Dain", "kind": "ACTOR_KIND_PARTY_MEMBER"}}},
  {"eventId": "w7", "sequence": "7", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-r", "name": "Rook", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["mu"]}}},
  {"eventId": "w8", "sequence": "8", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorRemoved": {"actorId": "act-r"}},
  {"eventId": "w9", "sequence": "9", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-a", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "w10", "sequence": "10", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-b", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "w11", "sequence": "11", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-a", "sceneId": "scn-1", "actorId": "act-a", "position": {"x": 0, "y": 0},
      "name": "the tall one"}},
  {"eventId": "w12", "sequence": "12", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-c", "sceneId": "scn-1", "actorId": "act-c", "position": {"x": 2, "y": 0}}},
  {"eventId": "w13", "sequence": "13", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "conditionApplied": {"actorId": "act-a", "conditionId": "dazed", "source": "trap"}},
  {"eventId": "w14", "sequence": "14", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "resourceChanged": {"actorId": "act-a", "resource": "stamina", "delta": -1, "newValue": 3}},
  {"eventId": "w15", "sequence": "15", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "doorOpened": {"sceneId": "scn-1", "at": {"x": 1, "y": 0}}},
  {"eventId": "w16", "sequence": "16", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "noteUpserted": {"key": "lore", "title": "Lore", "text": "Old", "visibility": "NOTE_VISIBILITY_PUBLIC"}}
]`;

const DUMP_LOG = `[
  {"eventId": "d1", "sequence": "1", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sessionStarted": {"name": "Night"}},
  {"eventId": "d2", "sequence": "2", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-a", "name": "Aster", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["mu", "kappa", "mu", "alpha", "mu"]}}},
  {"eventId": "d3", "sequence": "3", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-b", "name": "Bram", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["kappa"]}}},
  {"eventId": "d4", "sequence": "4", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-c", "name": "Crag", "kind": "ACTOR_KIND_NON_PARTY",
      "abilityIds": ["claw", "claw"]}}},
  {"eventId": "d5", "sequence": "5", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-a", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "d6", "sequence": "6", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-a", "abilityId": "zeta"}},
  {"eventId": "d7", "sequence": "7", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-a", "abilityId": "mu"}},
  {"eventId": "d8", "sequence": "8", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-a", "abilityId": "mu"}},
  {"eventId": "d9", "sequence": "9", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-b", "abilityId": "mu"}},
  {"eventId": "d10", "sequence": "10", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-b", "abilityId": "kappa"}},
  {"eventId": "d11", "sequence": "11", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-b", "abilityId": "mu"}},
  {"eventId": "d12", "sequence": "12", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityTaken": {"actorId": "act-c", "abilityId": "claw"}},
  {"eventId": "d13", "sequence": "13", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "abilityGiven": {"actorId": "act-c", "abilityId": "bite"}}
]`;

const DUMP_WANT = `{
  "Actors": {
    "act-a": {
      "actor_id": "act-a",
      "name": "Aster",
      "controller_id": "p-a",
      "controller_ids": [
        "p-a"
      ],
      "kind": 1,
      "ability_ids": [
        "kappa",
        "alpha",
        "zeta",
        "mu"
      ]
    },
    "act-b": {
      "actor_id": "act-b",
      "name": "Bram",
      "kind": 1
    },
    "act-c": {
      "actor_id": "act-c",
      "name": "Crag",
      "kind": 2,
      "ability_ids": [
        "bite"
      ]
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {},
  "Sessions": [
    {
      "ID": "sess-1",
      "Name": "Night",
      "StartSeq": 1,
      "EndSeq": 0
    }
  ],
  "Tokens": {},
  "headSequence": 13
}`;

function parse(raw: string): Envelope[] {
  return (JSON.parse(raw) as JsonValue[]).map((j) => fromJson(EnvelopeSchema, j));
}

function change(kind: "abilityGiven" | "abilityTaken", seq: number, actorId: string, abilityId: string): Envelope {
  return fromJson(EnvelopeSchema, {
    eventId: "x", sequence: String(seq), sessionId: "sess-1", actorRole: "dm", participantId: "p-dm",
    [kind]: { actorId, abilityId },
  });
}

const give = (seq: number, actor: string, ability: string) => change("abilityGiven", seq, actor, ability);
const take = (seq: number, actor: string, ability: string) => change("abilityTaken", seq, actor, ability);

type Path = (string | number)[];

function diff(a: unknown, b: unknown, at: Path = [], out: Path[] = []): Path[] {
  if (a instanceof Map) a = [...a.entries()];
  if (b instanceof Map) b = [...b.entries()];
  if (a instanceof Set) a = [...a];
  if (b instanceof Set) b = [...b];
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) out.push(at);
    else a.forEach((x, i) => diff(x, b[i], [...at, i], out));
    return out;
  }
  if (a !== null && b !== null && typeof a === "object" && typeof b === "object") {
    const ao = a as Record<string, unknown>;
    const bo = b as Record<string, unknown>;
    const keys = new Set([...Object.keys(ao), ...Object.keys(bo)]);
    for (const k of [...keys].sort()) {
      if (!(k in ao) || !(k in bo)) out.push([...at, k]);
      else diff(ao[k], bo[k], [...at, k], out);
    }
    return out;
  }
  if (!Object.is(a, b)) out.push(at);
  return out;
}

function abilityIdsOf(log: Envelope[], id: string): string[] {
  const actor = fold(log).Actors[id];
  if (actor === undefined) throw new Error(`no actor ${id}`);
  return [...actor.abilityIds];
}

// VTT-715
test("VTT-715 the client fold of an abilityGiven changes only the actor's list, appended at the end", () => {
  const world = parse(WORLD);
  const before = fold(world);
  const after = fold([...world, give(17, "act-a", "zeta")]);
  expect(diff(before, after)).toEqual([["Actors", "act-a", "abilityIds"]]);
  expect(after.Actors["act-a"]?.abilityIds).toEqual(["mu", "kappa", "mu", "alpha", "mu", "zeta"]);
});

// VTT-715
test("VTT-715 the client fold appends each given id at the end, not in sorted order", () => {
  const world = parse(WORLD);
  const log = [...world, give(17, "act-a", "zeta"), give(18, "act-a", "beta"), give(19, "act-a", "claw")];
  expect(abilityIdsOf(log, "act-a")).toEqual(["mu", "kappa", "mu", "alpha", "mu", "zeta", "beta", "claw"]);
  expect(abilityIdsOf(log, "act-c")).toEqual(["claw"]);
});

// VTT-715
test("VTT-715 the client fold of an abilityTaken removes every occurrence and changes nothing else", () => {
  const world = parse(WORLD);
  const before = fold(world);
  const after = fold([...world, take(17, "act-a", "mu")]);
  expect(diff(before, after)).toEqual([["Actors", "act-a", "abilityIds"]]);
  expect(after.Actors["act-a"]?.abilityIds).toEqual(["kappa", "alpha"]);
});

// VTT-715
test("VTT-715 the client fold of an abilityTaken of a single id keeps the others in their order", () => {
  const world = parse(WORLD);
  expect(abilityIdsOf([...world, take(17, "act-a", "kappa")], "act-a")).toEqual(["mu", "mu", "alpha", "mu"]);
});

// VTT-715
test("VTT-715 the client fold changes a creature's list and nothing else", () => {
  const world = parse(WORLD);
  const given = [...world, give(17, "act-c", "mu")];
  expect(diff(fold(world), fold(given))).toEqual([["Actors", "act-c", "abilityIds"]]);
  expect(abilityIdsOf(given, "act-c")).toEqual(["claw", "mu"]);
  const taken = [...given, take(18, "act-c", "claw")];
  expect(diff(fold(given), fold(taken))).toEqual([["Actors", "act-c", "abilityIds"]]);
  expect(abilityIdsOf(taken, "act-c")).toEqual(["mu"]);
});

const REFUSALS: { id: string; name: string; prefix: Envelope[]; env: Envelope }[] = [
  { id: "VTT-722", name: "give of an empty id", prefix: [], env: give(17, "act-a", "") },
  { id: "VTT-722", name: "give of an empty id to an unknown actor", prefix: [], env: give(17, "ghost", "") },
  { id: "VTT-722", name: "give of an empty id to no actor", prefix: [], env: give(17, "", "") },
  { id: "VTT-717", name: "give to an unknown actor", prefix: [], env: give(17, "ghost", "zeta") },
  { id: "VTT-717", name: "give to an unknown actor of an id others name", prefix: [], env: give(17, "ghost", "mu") },
  { id: "VTT-717", name: "give to a removed actor", prefix: [], env: give(17, "act-r", "zeta") },
  { id: "VTT-717", name: "give to no actor", prefix: [], env: give(17, "", "zeta") },
  { id: "VTT-717", name: "take from an unknown actor", prefix: [], env: take(17, "ghost", "mu") },
  { id: "VTT-717", name: "take from a removed actor of an id it named", prefix: [], env: take(17, "act-r", "mu") },
  { id: "VTT-717", name: "take of an empty id from an unknown actor", prefix: [], env: take(17, "ghost", "") },
  { id: "VTT-720", name: "give of an id the sheet repeats", prefix: [], env: give(17, "act-a", "mu") },
  { id: "VTT-720", name: "give of an id the sheet names once", prefix: [], env: give(17, "act-a", "alpha") },
  { id: "VTT-720", name: "give of the first id of another sheet", prefix: [], env: give(17, "act-b", "kappa") },
  { id: "VTT-720", name: "give of an id the sheet was given", prefix: [give(17, "act-b", "zeta")], env: give(18, "act-b", "zeta") },
  { id: "VTT-721", name: "take of an id only another sheet names", prefix: [], env: take(17, "act-a", "claw") },
  { id: "VTT-721", name: "take of an empty id", prefix: [], env: take(17, "act-a", "") },
  { id: "VTT-721", name: "take from a creature of an id it lacks", prefix: [], env: take(17, "act-c", "mu") },
  { id: "VTT-721", name: "take from a sheet with no ids", prefix: [], env: take(17, "act-d", "mu") },
  { id: "VTT-721", name: "take again of an id already taken", prefix: [take(17, "act-a", "mu")], env: take(18, "act-a", "mu") },
];

for (const c of REFUSALS) {
  // VTT-717, VTT-720, VTT-721, VTT-722, VTT-727
  test(`${c.id} VTT-727 the client fold refuses a ${c.name}, as engine.Apply does`, () => {
    const log = [...parse(WORLD), ...c.prefix];
    expect(() => fold(log)).not.toThrow();
    let caught: unknown;
    try {
      fold([...log, c.env]);
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(FoldError);
    expect((caught as Error).message.length).toBeGreaterThan(0);
  });
}

// VTT-727
test("VTT-727 the client fold accepts what engine.Apply accepts beside the refusals", () => {
  const world = parse(WORLD);
  expect(abilityIdsOf([...world, give(17, "act-a", "claw")], "act-a")).toEqual(["mu", "kappa", "mu", "alpha", "mu", "claw"]);
  expect(abilityIdsOf([...world, give(17, "act-d", "mu")], "act-d")).toEqual(["mu"]);
  expect(abilityIdsOf([...world, take(17, "act-b", "mu"), give(18, "act-b", "mu")], "act-b")).toEqual(["kappa", "mu"]);
});

// VTT-727
test("VTT-727 the client's dump of gives and takes is the one engine.Apply's fold prints", () => {
  const got = foldToDumpJSON(parse(DUMP_LOG));
  expect(JSON.parse(got)).toEqual(JSON.parse(DUMP_WANT));
  expect(got.trim()).toBe(DUMP_WANT);
});

const ABILITIES = [
  { id: "zeta", name: "Zephyr Step" },
  { id: "mu", name: "Mule Kick" },
  { id: "bite", name: "Bite" },
];

function wordsIn(dm: boolean, env: Envelope): string {
  const log = parse(WORLD);
  return wordsOf(env, namer(fold(log), log, dm, ABILITIES));
}

// VTT-813
test("VTT-813 the Events panel words an abilityGiven '<actor> can now use <ability>', both as the window names them", () => {
  expect(wordsIn(true, give(17, "act-a", "zeta"))).toBe("Aster can now use Zephyr Step");
  expect(wordsIn(false, give(17, "act-a", "zeta"))).toBe("the tall one can now use Zephyr Step");
  expect(wordsIn(true, give(17, "act-c", "bite"))).toBe("Crag can now use Bite");
  expect(wordsIn(false, give(17, "act-c", "bite"))).toBe("a creature can now use Bite");
});

// VTT-813
test("VTT-813 the Events panel words an abilityTaken '<actor> can no longer use <ability>', both as the window names them", () => {
  expect(wordsIn(true, take(17, "act-a", "mu"))).toBe("Aster can no longer use Mule Kick");
  expect(wordsIn(false, take(17, "act-a", "mu"))).toBe("the tall one can no longer use Mule Kick");
  expect(wordsIn(true, take(17, "act-c", "claw"))).toBe("Crag can no longer use claw");
  expect(wordsIn(true, take(17, "act-b", "kappa"))).toBe("Bram can no longer use kappa");
});
