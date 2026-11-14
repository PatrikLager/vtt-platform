import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

// The client's arms for abilityGiven and abilityTaken, mirroring
// engine.Apply's: internal/engine/ability_change_test.go holds the same
// cases there, and cmd/vtt/ability_change_dump_test.go folds the same
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

const given = (seq: number, actorId: string, abilityId: string) =>
  env(seq, { abilityGiven: { actorId, abilityId } });
const taken = (seq: number, actorId: string, abilityId: string) =>
  env(seq, { abilityTaken: { actorId, abilityId } });

// a1 holds a value in every field an event can set, its list naming rally
// twice, beside a2, a scene, a token, a condition and a note.
const table: Envelope[] = [
  env(1, { sessionStarted: { name: "S" } }),
  env(2, { sceneCreated: { sceneId: "s1", name: "N", gridWidth: 4, gridHeight: 4 } }),
  env(3, {
    actorAdded: {
      actor: {
        actorId: "a1", name: "Ada", moduleId: "m", kind: "ACTOR_KIND_PARTY_MEMBER",
        attributes: { brawn: 2 }, resources: { stamina: { current: 3, max: 5 } },
        abilityIds: ["rally", "fists", "rally", "lunge"],
      },
    },
  }),
  env(4, { actorAdded: { actor: { actorId: "a2", name: "Bo", kind: "ACTOR_KIND_NON_PARTY", abilityIds: ["rally"] } } }),
  env(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "a cloaked shape" } }),
  env(6, { actorControlGranted: { actorId: "a1", participantId: "p1", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
  env(7, { conditionApplied: { actorId: "a1", conditionId: "marked", source: "trap" } }),
  env(8, { noteUpserted: { key: "k", title: "T", text: "x", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
];

/** The fold of table with a1's list replaced by ids, and nothing else. */
function tableNaming(ids: string[]) {
  const st = fold(table);
  st.Actors["a1"]!.abilityIds = ids;
  return st;
}

function rejects(log: Envelope[], message: string) {
  expect(() => fold(log)).toThrow(new FoldError(message));
}

// VTT-715
test("an ability given ends the actor's list and changes nothing else", () => {
  expect(fold([...table, given(9, "a1", "dodge")])).toEqual(
    tableNaming(["rally", "fists", "rally", "lunge", "dodge"]));
});

// VTT-715
test("an ability taken leaves no occurrence of it, the other ids in their order", () => {
  expect(fold([...table, taken(9, "a1", "rally")])).toEqual(tableNaming(["fists", "lunge"]));
});

// VTT-715
test("a give after a take ends the list with the id, and taking every id leaves none", () => {
  const back = [...table, taken(9, "a1", "fists"), given(10, "a1", "fists")];
  expect(fold(back).Actors["a1"]!.abilityIds).toEqual(["rally", "rally", "lunge", "fists"]);
  const none = [...back, taken(11, "a1", "lunge"), taken(12, "a1", "rally"), taken(13, "a1", "fists")];
  expect(fold(none)).toEqual(tableNaming([]));
});

// VTT-722
test("an ability given with an empty id is refused, before its actor is looked up", () => {
  rejects([...table, given(9, "a1", "")], "ability given with an empty ability id");
  rejects([...table, given(9, "ghost", "")], "ability given with an empty ability id");
});

// VTT-717
test("an ability given or taken naming no actor the fold holds is refused", () => {
  rejects([...table, given(9, "ghost", "rally")], 'ability given to unknown actor "ghost"');
  rejects([...table, taken(9, "ghost", "rally")], 'ability taken from unknown actor "ghost"');
  rejects([...table, taken(9, "", "rally")], 'ability taken from unknown actor ""');
});

// VTT-720
test("an ability given that the sheet already names is refused", () => {
  rejects([...table, given(9, "a1", "lunge")], 'ability "lunge" already named on actor "a1"');
  rejects([...table, given(9, "a2", "rally")], 'ability "rally" already named on actor "a2"');
});

// VTT-721
test("an ability taken that the sheet does not name is refused", () => {
  rejects([...table, taken(9, "a2", "fists")], 'ability "fists" not named on actor "a2"');
  rejects([...table, taken(9, "a1", "fists"), taken(10, "a1", "fists")], 'ability "fists" not named on actor "a1"');
});

const testdata = (name: string) => join(import.meta.dir, "../../contract/testdata", name);
const envelopes = (name: string): Envelope[] =>
  (JSON.parse(readFileSync(testdata(name), "utf8")) as unknown[]).map((e) => fromJson(EnvelopeSchema, e as never));

// VTT-727
test("the dump after gives and takes matches the committed dump, as Go's writeDump does", () => {
  expect(foldToDumpJSON(envelopes("ability_change_log.json"))).toBe(
    readFileSync(testdata("ability_change_dump.json"), "utf8"));
});

// VTT-727
test("each refusal in the committed fixture, the client's fold refuses", () => {
  const log = envelopes("ability_change_log.json");
  const refused = envelopes("ability_change_refused.json");
  expect(refused.length).toBeGreaterThan(0);
  for (const e of refused) expect(() => fold([...log, e])).toThrow(FoldError);
});
