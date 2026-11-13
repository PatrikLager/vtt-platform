import { expect, test } from "bun:test";
import { fromJson, type JsonObject } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";

// Keep this out of byte order and holding a duplicate, so that a sort or a set
// would change it.
const listed = ["rally", "fists", "rally", "goblin-scimitar"];

function env(seq: number, payload: JsonObject): Envelope {
  return fromJson(EnvelopeSchema, { eventId: `qa-e${seq}`, sequence: String(seq), ...payload });
}

function added(seq: number, actor: JsonObject): Envelope {
  return env(seq, { actorAdded: { actor } });
}

// Keep every field an ActorAdded may carry before ability_ids set here,
// except module_data, which the client's fold does not hold, and no
// controller: both folds refuse an ActorAdded that carries one.
const richActor: JsonObject = {
  actorId: "a1",
  name: "Ada",
  moduleId: "mod",
  attributes: { vigor: 3 },
  resources: { stamina: { current: 2, max: 5 } },
  kind: "ACTOR_KIND_PARTY_MEMBER",
  abilityIds: listed,
};

const unlisted: JsonObject = { actorId: "a2", name: "Bo", kind: "ACTOR_KIND_NON_PARTY" };

const unspecifiedKind: JsonObject = {
  actorId: "a4",
  name: "Dee",
  moduleId: "mod",
  attributes: { vigor: 1 },
  resources: { stamina: { current: 1, max: 4 } },
  abilityIds: ["rally", "fists"],
};

function compactDump(envs: Envelope[]): string {
  return foldToDumpJSON(envs).replace(/\s+/g, "");
}

// VTT-706
test("the client fold holds an added actor's ability ids in the event's order, duplicates and all", () => {
  const st = fold([
    added(1, richActor),
    added(2, unlisted),
    added(3, { actorId: "a3", name: "Cy", kind: "ACTOR_KIND_NON_PARTY", abilityIds: ["goblin-scimitar", "fists"] }),
  ]);
  expect(st.Actors["a1"]?.abilityIds).toEqual(listed);
  expect(st.Actors["a3"]?.abilityIds).toEqual(["goblin-scimitar", "fists"]);
});

// VTT-706
test("the client fold holds an empty list for an actor added with none, beside one with a list", () => {
  const st = fold([
    added(1, richActor),
    added(2, unlisted),
    added(3, { actorId: "a3", name: "Cy", kind: "ACTOR_KIND_NON_PARTY", abilityIds: [] }),
  ]);
  const a2 = st.Actors["a2"];
  expect(a2).toBeDefined();
  expect(Array.isArray(a2?.abilityIds)).toBe(true);
  expect(a2?.abilityIds).toEqual([]);
  expect(st.Actors["a3"]?.abilityIds).toEqual([]);
  expect(st.Actors["a1"]?.abilityIds).toEqual(listed);
});

const rewrites: Envelope[] = [
  added(1, richActor),
  env(2, { actorControlGranted: { actorId: "a1", participantId: "p1", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
  env(3, { actorControlGranted: { actorId: "a1", participantId: "p2", kind: "ACTOR_KIND_NON_PARTY" } }),
  env(4, { resourceChanged: { actorId: "a1", resource: "stamina", delta: -1, newValue: 1 } }),
  env(5, { actorControlRevoked: { actorId: "a1", participantId: "p1" } }),
];

// VTT-705
test("the client fold keeps an actor's ability ids through grants, a resource change and a revoke", () => {
  expect(fold(rewrites).Actors["a1"]?.abilityIds).toEqual(listed);
});

// VTT-704; the literal is the one
// TestQAGoJSONOfAFoldedActorWritesAbilityIdsAfterKindInOrder pins for Go.
test("foldToDumpJSON writes ability_ids after kind with the ids in order, as Go's encoding/json does", () => {
  expect(compactDump([added(1, richActor)])).toContain(
    '"a1":{"actor_id":"a1","name":"Ada","module_id":"mod","attributes":{"vigor":3},' +
      '"resources":{"stamina":{"current":2,"max":5}},"kind":1,' +
      '"ability_ids":["rally","fists","rally","goblin-scimitar"]}',
  );
});

// VTT-704
test("foldToDumpJSON omits ability_ids for an actor with no list, beside one with a list", () => {
  const dump = compactDump([
    added(1, richActor),
    added(2, unlisted),
    added(3, { actorId: "a3", name: "Cy", kind: "ACTOR_KIND_NON_PARTY", abilityIds: [] }),
  ]);
  expect(dump).toContain('"a2":{"actor_id":"a2","name":"Bo","kind":2}');
  expect(dump).toContain('"a3":{"actor_id":"a3","name":"Cy","kind":2}');
  expect(dump.split('"ability_ids"').length - 1).toBe(1);
});

// VTT-704; the literal is the one
// TestQAGoJSONOfAnActorWithUnspecifiedKindWritesAbilityIdsLast pins for Go.
test("foldToDumpJSON writes ability_ids last for an actor whose kind is unspecified", () => {
  expect(compactDump([added(1, unspecifiedKind)])).toContain(
    '"a4":{"actor_id":"a4","name":"Dee","module_id":"mod","attributes":{"vigor":1},' +
      '"resources":{"stamina":{"current":1,"max":4}},"ability_ids":["rally","fists"]}',
  );
});

// VTT-704 VTT-705
test("foldToDumpJSON keeps ability_ids last, in order, after grants, a resource change and a revoke", () => {
  const dump = compactDump(rewrites);
  const start = dump.indexOf('"a1":{');
  const actor = dump.slice(start, dump.indexOf("}", dump.indexOf('"ability_ids"', start)) + 1);
  const ci = actor.indexOf('"controller_ids":');
  const ki = actor.indexOf('"kind":');
  const ai = actor.indexOf('"ability_ids":');
  expect(ci).toBeGreaterThan(-1);
  expect(ki).toBeGreaterThan(ci);
  expect(ai).toBeGreaterThan(ki);
  expect(actor.endsWith('"ability_ids":["rally","fists","rally","goblin-scimitar"]}')).toBe(true);
});
