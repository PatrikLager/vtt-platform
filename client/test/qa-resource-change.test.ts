import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";
import { describe as describeEntry } from "../src/view/spectator";
import type { Namer } from "../src/view/naming";

type Dump = {
  Actors: Record<string, { resources?: Record<string, { current?: number; max?: number }> } & Record<string, unknown>>;
  headSequence?: number;
} & Record<string, unknown>;

function env(seq: number, payload: Record<string, unknown>): Envelope {
  return fromJson(EnvelopeSchema, {
    eventId: `qa-rc-${seq}`,
    sequence: String(seq),
    sessionId: "sess-qa",
    actorRole: "dm",
    participantId: "p-dm",
    ...payload,
  } as JsonValue);
}

function readLog(path: string): Envelope[] {
  const items = JSON.parse(readFileSync(path, "utf8")) as JsonValue[];
  return items.map((it) => fromJson(EnvelopeSchema, it));
}

const LOG_PATH = join(import.meta.dir, "../../contract/testdata/qa_resource_change_log.json");
const REFUSED_PATH = join(import.meta.dir, "../../contract/testdata/qa_resource_change_refused.json");
const DUMP_PATH = join(import.meta.dir, "../../contract/testdata/qa_resource_change_dump.json");

// Lera holds an ATTRIBUTE named focus and the goblin a RESOURCE named focus,
// so a fold that writes the wrong map or the wrong actor shows.
function base(): Envelope[] {
  return [
    env(1, { sessionStarted: { name: "Evening" } }),
    env(2, {
      actorAdded: {
        actor: {
          actorId: "act-lera",
          name: "Lera",
          moduleId: "mod-q",
          kind: "ACTOR_KIND_PARTY_MEMBER",
          attributes: { nerve: 2, focus: 7 },
          resources: { pool: { current: 4, max: 6 }, grit: { current: 1, max: 3 } },
          moduleData: { mood: "wary" },
          abilityIds: ["rally", "lunge"],
        },
      },
    }),
    env(3, {
      actorAdded: {
        actor: {
          actorId: "act-gob",
          name: "Goblin",
          kind: "ACTOR_KIND_NON_PARTY",
          attributes: { nerve: 1 },
          resources: { pool: { current: 2, max: 2 }, focus: { current: 5, max: 8 }, spark: {} },
        },
      },
    }),
  ];
}

function rich(): Envelope[] {
  const tiles: Record<string, { kind: string }> = {};
  for (let x = 0; x < 3; x++) for (let y = 0; y < 3; y++) tiles[`${x},${y}`] = { kind: "floor" };
  return [
    ...base(),
    env(4, { sceneCreated: { sceneId: "sc-1", name: "Hall", gridWidth: 3, gridHeight: 3, tiles } }),
    env(5, { actorControlGranted: { actorId: "act-lera", participantId: "p-1", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
    env(6, { conditionApplied: { actorId: "act-lera", conditionId: "dazed", source: "trap" } }),
    env(7, { tokenPlaced: { tokenId: "tok-lera", sceneId: "sc-1", actorId: "act-lera", position: { x: 1, y: 1 }, name: "Lera" } }),
    env(8, { tokenPlaced: { tokenId: "tok-gob", sceneId: "sc-1", actorId: "act-gob", position: { x: 2, y: 2 } } }),
    env(9, { noteUpserted: { key: "lore", title: "Lore", text: "old", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
  ];
}

function dumpOf(log: Envelope[]): Dump {
  return JSON.parse(foldToDumpJSON(log)) as Dump;
}

function resourcesOf(log: Envelope[], actor: string): Record<string, { current?: number; max?: number }> {
  return dumpOf(log).Actors[actor]?.resources ?? {};
}

const added = (seq: number, actorId: string, resource: string, value?: Record<string, number>): Envelope =>
  env(seq, { resourceAdded: value === undefined ? { actorId, resource } : { actorId, resource, value } });
const removed = (seq: number, actorId: string, resource: string): Envelope =>
  env(seq, { resourceRemoved: { actorId, resource } });

describe("the client's fold at a ResourceAdded", () => {
  // VTT-769: the client's fold gives the named actor the named resource at the
  // event's current and max.
  test("gives the named actor the resource at the event's current and max", () => {
    const res = resourcesOf([...base(), added(4, "act-lera", "focus", { current: 3, max: 5 })], "act-lera");
    expect(res.focus).toEqual({ current: 3, max: 5 });
  });

  // VTT-769 (0 is a value): a value whose numbers are 0 is given, and dumped as
  // `vtt state dump` writes a zero.
  test("gives a value whose numbers are zero", () => {
    expect(resourcesOf([...base(), added(4, "act-lera", "spark", {})], "act-lera")).toHaveProperty("spark", {});
    expect(resourcesOf([...base(), added(4, "act-lera", "spark", { max: 4 })], "act-lera").spark).toEqual({ max: 4 });
  });

  // VTT-791
  test("reads no number of the value", () => {
    expect(resourcesOf([...base(), added(4, "act-lera", "ember", { current: 9, max: 4 })], "act-lera").ember).toEqual({
      current: 9,
      max: 4,
    });
    expect(resourcesOf([...base(), added(4, "act-lera", "ember", { current: -2 })], "act-lera").ember).toEqual({
      current: -2,
    });
  });

  // VTT-769 with VTT-770: "already holds" is the named actor's resource; an
  // attribute of that name, or another actor's resource of that name, is not.
  test("gives a resource whose name another actor holds or is the actor's attribute", () => {
    const d = dumpOf([...base(), added(4, "act-lera", "focus", { current: 1, max: 2 })]);
    expect(d.Actors["act-lera"]?.resources?.focus).toEqual({ current: 1, max: 2 });
  });

  // VTT-770: the client's fold refuses a ResourceAdded naming an actor it does
  // not hold, a resource the actor already holds, no value, or a resource name
  // that is empty or longer than 128 bytes.
  const refusals: [string, Envelope][] = [
    ["an unknown actor", added(4, "act-ghost", "focus", { current: 1, max: 1 })],
    ["an empty actor", added(4, "", "focus", { current: 1, max: 1 })],
    ["a resource the actor holds", added(4, "act-lera", "pool", { current: 1, max: 1 })],
    ["a resource the actor holds at zero", added(4, "act-gob", "spark", { current: 1, max: 1 })],
    ["no value", added(4, "act-lera", "dust")],
    ["an empty resource name", added(4, "act-lera", "", { current: 1, max: 1 })],
    ["129 one-byte characters", added(4, "act-lera", "a".repeat(129), { current: 1, max: 1 })],
    ["65 two-byte characters (130 bytes, 65 code units)", added(4, "act-lera", "ö".repeat(65), { current: 1, max: 1 })],
    ["33 four-byte characters (132 bytes, 66 code units)", added(4, "act-lera", "😀".repeat(33), { current: 1, max: 1 })],
  ];
  for (const [name, ev] of refusals) {
    test(`refuses ${name}`, () => {
      expect(() => fold([...base(), ev])).toThrow(FoldError);
    });
  }

  // VTT-770: the bound is 128 BYTES, so a name of exactly 128 bytes is given
  // whatever its count of characters or code units.
  for (const [label, name] of [
    ["128 one-byte characters", "a".repeat(128)],
    ["64 two-byte characters", "ö".repeat(64)],
    ["32 four-byte characters", "😀".repeat(32)],
  ] as const) {
    test(`gives a 128-byte name of ${label}`, () => {
      expect(resourcesOf([...base(), added(4, "act-lera", name, { current: 2, max: 3 })], "act-lera")[name]).toEqual({
        current: 2,
        max: 3,
      });
    });
  }
});

describe("the client's fold at a ResourceRemoved", () => {
  // VTT-771: the client's fold takes the named resource from the named actor;
  // so taken, it can be added again (VTT-769).
  test("takes the named resource from the named actor", () => {
    const log = [...base(), removed(4, "act-lera", "grit")];
    expect(resourcesOf(log, "act-lera")).not.toHaveProperty("grit");
    expect(resourcesOf(log, "act-lera").pool).toEqual({ current: 4, max: 6 });
    const again = [...log, added(5, "act-lera", "grit", { current: 9, max: 9 })];
    expect(resourcesOf(again, "act-lera").grit).toEqual({ current: 9, max: 9 });
  });

  // VTT-771: a resource held at 0 and 0 is held, and is taken.
  test("takes a resource held at zero", () => {
    expect(resourcesOf([...base(), removed(4, "act-gob", "spark")], "act-gob")).not.toHaveProperty("spark");
  });

  // VTT-772: the client's fold refuses a ResourceRemoved naming an actor it
  // does not hold or a resource the actor does not hold.
  const refusals: [string, Envelope[]][] = [
    ["an unknown actor", [removed(4, "act-ghost", "pool")]],
    ["a resource not held", [removed(4, "act-lera", "dust")]],
    ["a resource another actor holds", [removed(4, "act-lera", "spark")]],
    ["an attribute's key", [removed(4, "act-lera", "nerve")]],
    ["an empty resource name", [removed(4, "act-lera", "")]],
    ["a resource already removed", [removed(4, "act-gob", "pool"), removed(5, "act-gob", "pool")]],
  ];
  for (const [name, evs] of refusals) {
    test(`refuses ${name}`, () => {
      expect(() => fold([...base(), ...evs])).toThrow(FoldError);
    });
  }
});

// VTT-773: the client's fold changes nothing at a ResourceAdded or a
// ResourceRemoved but the named resource of the named actor.
describe("the client's fold changes nothing else", () => {
  test("a ResourceAdded changes only the named actor's named resource", () => {
    const before = dumpOf(rich());
    const after = dumpOf([...rich(), added(10, "act-lera", "focus", { current: 3, max: 5 })]);
    expect(after.Actors["act-lera"]?.resources?.focus).toEqual({ current: 3, max: 5 });
    delete after.Actors["act-lera"]?.resources?.focus;
    delete before.headSequence;
    delete after.headSequence;
    expect(after).toEqual(before);
  });

  test("a ResourceRemoved changes only the named actor's named resource", () => {
    const before = dumpOf(rich());
    const after = dumpOf([...rich(), removed(10, "act-gob", "focus")]);
    expect(before.Actors["act-gob"]?.resources).toHaveProperty("focus");
    expect(after.Actors["act-gob"]?.resources ?? {}).not.toHaveProperty("focus");
    delete before.Actors["act-gob"]?.resources?.focus;
    delete before.headSequence;
    delete after.headSequence;
    expect(after).toEqual(before);
  });
});

// VTT-774: the client's fold of a log of ResourceAdded and ResourceRemoved
// gives the state `vtt state dump` prints for the same log, refusals included;
// cmd/vtt/qa_resource_change_dump_test.go holds the Go printer to the same
// golden and the same refusals.
describe("the client's dump of a resource log", () => {
  test("is the golden `vtt state dump` prints", () => {
    const got = foldToDumpJSON(readLog(LOG_PATH));
    expect(got.trimEnd()).toBe(readFileSync(DUMP_PATH, "utf8").trimEnd());
  });

  const refused = readLog(REFUSED_PATH);
  refused.forEach((r, i) => {
    test(`refuses refused[${i}] as the Go fold does`, () => {
      expect(() => fold([...readLog(LOG_PATH), r])).toThrow(FoldError);
    });
  });
});

// VTT-790: the Story words a ResourceAdded "<actor> now has <resource> at
// <current>" and a ResourceRemoved "<actor> no longer has <resource>", the
// resource named by Namer.resource.
describe("the Story's line of a resource event", () => {
  const names: Namer = {
    token: (id: string) => `TOKEN(${id})`,
    shown: (id: string) => `SHOWN(${id})`,
    actor: (id: string | undefined) => (id === "act-lera" ? "Lera" : `ACTOR(${id})`),
    ability: (id: string) => `ABILITY(${id})`,
    resource: (key: string) => (key === "focus" ? "Focus" : `RES(${key})`),
  } as Namer;

  test("a ResourceAdded reads the actor, the resource and its current", () => {
    expect(describeEntry(added(4, "act-lera", "focus", { current: 3, max: 5 }), names)).toBe("Lera now has Focus at 3");
  });

  test("a ResourceAdded at current zero reads 0", () => {
    expect(describeEntry(added(4, "act-lera", "focus", { max: 2 }), names)).toBe("Lera now has Focus at 0");
  });

  test("a ResourceRemoved reads the actor and the resource", () => {
    expect(describeEntry(removed(4, "act-lera", "focus"), names)).toBe("Lera no longer has Focus");
  });
});
