import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError, type Resource } from "../src/state";

// The client's arms for resourceAdded and resourceRemoved, mirroring
// engine.Apply's: internal/engine/resource_change_test.go holds the server's
// cases, and cmd/vtt/resource_change_dump_test.go folds the same
// contract/testdata log and refusals through the server's fold.

function env(seq: number, payload: Record<string, unknown>): Envelope {
  return fromJson(EnvelopeSchema, {
    eventId: `evt-${seq}`,
    sequence: String(seq),
    sessionId: "sess-1",
    actorRole: "dm",
    participantId: "p-dm",
    ...payload,
  } as never);
}

const added = (seq: number, actorId: string, resource: string, value?: Record<string, number>) =>
  env(seq, { resourceAdded: value === undefined ? { actorId, resource } : { actorId, resource, value } });
const removed = (seq: number, actorId: string, resource: string) =>
  env(seq, { resourceRemoved: { actorId, resource } });

// a1 holds a value in every field an event can set, beside a2, a scene, a
// token, a condition and a note.
const table: Envelope[] = [
  env(1, { sessionStarted: { name: "S" } }),
  env(2, { sceneCreated: { sceneId: "s1", name: "N", gridWidth: 4, gridHeight: 4 } }),
  env(3, {
    actorAdded: {
      actor: {
        actorId: "a1", name: "Ada", moduleId: "m", kind: "ACTOR_KIND_PARTY_MEMBER",
        attributes: { brawn: 2 }, resources: { stamina: { current: 3, max: 5 } },
        abilityIds: ["rally", "fists"],
      },
    },
  }),
  env(4, { actorAdded: { actor: { actorId: "a2", name: "Bo", kind: "ACTOR_KIND_NON_PARTY" } } }),
  env(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "a cloaked shape" } }),
  env(6, { actorControlGranted: { actorId: "a1", participantId: "p1", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
  env(7, { conditionApplied: { actorId: "a1", conditionId: "marked", source: "trap" } }),
  env(8, { noteUpserted: { key: "k", title: "T", text: "x", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
];

/** The fold of table with a1's resources replaced by resources, and nothing else. */
function tableHolding(resources: Record<string, Resource>) {
  const st = fold(table);
  st.Actors["a1"]!.resources = resources;
  return st;
}

function rejects(log: Envelope[], message: string) {
  expect(() => fold(log)).toThrow(new FoldError(message));
}

// VTT-769 VTT-773
test("a resource added gives the actor the resource at its value and changes nothing else", () => {
  expect(fold([...table, added(9, "a1", "focus", { current: 0, max: 2 })])).toEqual(
    tableHolding({ stamina: { current: 3, max: 5 }, focus: { current: 0, max: 2 } }));
});

// VTT-769
test("a resource added with no max holds none, and one with an empty value holds zeros", () => {
  const st = fold([...table, added(9, "a2", "focus", { current: 4 }), added(10, "a2", "nerve", {})]);
  expect(st.Actors["a2"]!.resources).toEqual({ focus: { current: 4, max: 0 }, nerve: { current: 0, max: 0 } });
});

// VTT-771 VTT-773
test("a resource removed takes the resource from the actor and changes nothing else", () => {
  expect(fold([...table, removed(9, "a1", "stamina")])).toEqual(tableHolding({}));
});

// VTT-769 VTT-771
test("a resource removed then added again holds the new value", () => {
  expect(fold([...table, removed(9, "a1", "stamina"), added(10, "a1", "stamina", { current: 1, max: 1 })])).toEqual(
    tableHolding({ stamina: { current: 1, max: 1 } }));
});

// VTT-770
test("a resource added of an empty or overlong name is refused before anything else", () => {
  rejects([...table, added(9, "ghost", "")], "resource name is shorter than 1 bytes");
  rejects([...table, added(9, "a1", "r".repeat(129), { current: 1 })], "resource name exceeds 128 bytes");
  expect(fold([...table, added(9, "a1", "r".repeat(128), { current: 1 })]).Actors["a1"]!.resources["r".repeat(128)])
    .toEqual({ current: 1, max: 0 });
});

// VTT-770
test("a resource added with no value is refused before its actor is looked up", () => {
  rejects([...table, added(9, "a1", "focus")], "resource added with no value");
  rejects([...table, added(9, "ghost", "focus")], "resource added with no value");
});

// VTT-770
test("a resource added naming no actor the fold holds, or a resource the actor holds, is refused", () => {
  rejects([...table, added(9, "ghost", "focus", { current: 1 })], 'resource added to unknown actor "ghost"');
  rejects([...table, added(9, "a1", "stamina", { current: 1 })], 'resource "stamina" already held by actor "a1"');
});

// VTT-772
test("a resource removed naming no actor the fold holds, or a resource the actor does not hold, is refused", () => {
  rejects([...table, removed(9, "ghost", "stamina")], 'resource removed from unknown actor "ghost"');
  rejects([...table, removed(9, "a2", "stamina")], 'resource "stamina" not held by actor "a2"');
  rejects([...table, removed(9, "a1", "stamina"), removed(10, "a1", "stamina")], 'resource "stamina" not held by actor "a1"');
});

const testdata = (name: string) => join(import.meta.dir, "../../contract/testdata", name);
const envelopes = (name: string): Envelope[] =>
  (JSON.parse(readFileSync(testdata(name), "utf8")) as unknown[]).map((e) => fromJson(EnvelopeSchema, e as never));

// VTT-774
test("the dump after resources added and removed matches the committed dump, as Go's writeDump does", () => {
  expect(foldToDumpJSON(envelopes("resource_change_log.json"))).toBe(
    readFileSync(testdata("resource_change_dump.json"), "utf8"));
});

// VTT-774
test("each resource refusal in the committed fixture, the client's fold refuses", () => {
  const log = envelopes("resource_change_log.json");
  const refused = envelopes("resource_change_refused.json");
  expect(refused.length).toBeGreaterThan(0);
  for (const e of refused) expect(() => fold([...log, e])).toThrow(FoldError);
});
