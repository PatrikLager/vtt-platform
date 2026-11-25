import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError, type Reference } from "../src/state";

// The client's arms for referenceAdded, referenceRemoved and referenceChanged,
// and actorAdded's references, mirroring engine.Apply's:
// internal/engine/reference_change_test.go holds the same cases there, and
// cmd/vtt/reference_change_dump_test.go folds the same contract/testdata log
// and refusals through the server's fold.

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

type Ref = { entryId: string; place: string; state?: string };
const added = (seq: number, actorId: string, referenceId: string, reference?: Ref) =>
  env(seq, { referenceAdded: reference ? { actorId, referenceId, reference } : { actorId, referenceId } });
const removed = (seq: number, actorId: string, referenceId: string) =>
  env(seq, { referenceRemoved: { actorId, referenceId } });
const changed = (seq: number, actorId: string, referenceId: string, place: string, state = "") =>
  env(seq, { referenceChanged: { actorId, referenceId, place, state } });

const ECHO = { entryId: "echo", place: "mike", state: "lima" };
const DELTA = { entryId: "delta", place: "kilo" };
const held = (r: Ref): Reference => ({ entryId: r.entryId, place: r.place, state: r.state ?? "" });
const A1_REFS = { "r-echo": held(ECHO), "r-delta": held(DELTA) };

// a1 holds a value in every field an event can set, beside a2 holding no
// reference, a scene, a token, a condition and a note.
const table: Envelope[] = [
  env(1, { sessionStarted: { name: "S" } }),
  env(2, { sceneCreated: { sceneId: "s1", name: "N", gridWidth: 4, gridHeight: 4 } }),
  env(3, {
    actorAdded: {
      actor: {
        actorId: "a1", name: "Ada", moduleId: "m", kind: "ACTOR_KIND_PARTY_MEMBER",
        attributes: { sierra: 2 }, resources: { stamina: { current: 3, max: 5 } }, abilityIds: ["rally"],
        references: { "r-echo": ECHO, "r-delta": DELTA },
      },
    },
  }),
  env(4, { actorAdded: { actor: { actorId: "a2", name: "Bo", kind: "ACTOR_KIND_NON_PARTY" } } }),
  env(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "a cloaked shape" } }),
  env(6, { actorControlGranted: { actorId: "a1", participantId: "p1", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
  env(7, { conditionApplied: { actorId: "a1", conditionId: "marked", source: "trap" } }),
  env(8, { noteUpserted: { key: "k", title: "T", text: "x", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
];

/** The fold of table with one actor's references replaced, and nothing else. */
function tableHolding(actorId: string, refs: Record<string, Reference>) {
  const st = fold(table);
  st.Actors[actorId]!.references = refs;
  return st;
}

function rejects(log: Envelope[], message: string) {
  expect(() => fold(log)).toThrow(new FoldError(message));
}

const bram = (actor: Record<string, unknown>) =>
  env(9, { actorAdded: { actor: { actorId: "a3", name: "Bram", ...actor } } });

// VTT-1023
test("an actorAdded keeps its actor's references whole, and an actor added with none holds none", () => {
  const st = fold(table);
  expect(st.Actors["a1"]!.references).toEqual(A1_REFS);
  expect(st.Actors["a2"]!.references).toEqual({});
});

// VTT-1023
test("an actorAdded's references are the fold's own, not the event's", () => {
  const a3 = bram({ references: { "r-echo": ECHO, "r-delta": DELTA } });
  const st = fold([...table, a3]);
  const refs = (a3.payload.value as { actor: { references: Record<string, Reference> } }).actor.references;
  refs["r-echo"]!.place = "kilo";
  refs["r-new"] = held(ECHO);
  expect(st.Actors["a3"]!.references).toEqual(A1_REFS);
});

// VTT-1023
test("an actorAdded holding a reference with an empty id is refused, after its attribute names", () => {
  rejects([...table, bram({ references: { "": ECHO } })], 'actor_added: actor "a3" holds a reference with an empty id');
  rejects([...table, bram({ attributes: { "": 1 }, references: { "": ECHO } })], "attribute name is shorter than 1 bytes");
  expect(fold([...table, bram({ references: { " ": ECHO } })]).Actors["a3"]!.references).toEqual({ " ": held(ECHO) });
});

// VTT-1024
test("a referenceAdded gives the sheet the reference and changes nothing else", () => {
  const two = { entryId: "echo", place: "mike", state: "oscar" };
  expect(fold([...table, added(9, "a1", "r-echo-2", two)])).toEqual(
    tableHolding("a1", { ...A1_REFS, "r-echo-2": held(two) }));
});

// VTT-1024
test("a referenceAdded to a sheet holding none holds it, its empty state kept empty", () => {
  expect(fold([...table, added(9, "a2", "r-delta", DELTA)])).toEqual(tableHolding("a2", { "r-delta": held(DELTA) }));
});

// VTT-1024
test("a referenceAdded's reference is the fold's own, not the event's", () => {
  const log = [...table, added(9, "a2", "r-echo", ECHO)];
  const st = fold(log);
  (log[8]!.payload.value as { reference: Reference }).reference.place = "kilo";
  expect(st.Actors["a2"]!.references).toEqual({ "r-echo": held(ECHO) });
});

// VTT-1024
test("a referenceAdded with an empty id is refused first", () => {
  rejects([...table, added(9, "a1", "", ECHO)], "reference_added requires a reference id");
  rejects([...table, added(9, "ghost", "")], "reference_added requires a reference id");
});

// VTT-1024
test("a referenceAdded with no reference is refused before its actor and its id are looked up", () => {
  rejects([...table, added(9, "ghost", "r-new")], "reference_added requires a reference");
  rejects([...table, added(9, "a1", "r-echo")], "reference_added requires a reference");
});

// VTT-1024
test("a referenceAdded naming no actor the fold holds, then an id the sheet holds, is refused", () => {
  rejects([...table, added(9, "ghost", "r-echo", ECHO)], 'reference_added names unknown actor "ghost"');
  rejects([...table, added(9, "a1", "r-delta", ECHO)], 'actor "a1" already holds reference "r-delta"');
  rejects([...table, added(9, "a2", "r-echo", ECHO), added(10, "a2", "r-echo", DELTA)],
    'actor "a2" already holds reference "r-echo"');
});

// VTT-1025
test("a referenceRemoved takes the reference off the sheet and changes nothing else", () => {
  expect(fold([...table, removed(9, "a1", "r-echo")])).toEqual(tableHolding("a1", { "r-delta": held(DELTA) }));
  expect(fold([...table, removed(9, "a1", "r-echo"), removed(10, "a1", "r-delta")])).toEqual(tableHolding("a1", {}));
});

// VTT-1025
test("a referenceRemoved naming no actor the fold holds, then an id the sheet does not hold, is refused", () => {
  rejects([...table, removed(9, "ghost", "r-echo")], 'reference_removed names unknown actor "ghost"');
  rejects([...table, removed(9, "a2", "r-echo")], 'actor "a2" does not hold reference "r-echo"');
  rejects([...table, removed(9, "a1", "r-echo"), removed(10, "a1", "r-echo")], 'actor "a1" does not hold reference "r-echo"');
});

// VTT-1024 VTT-1025
test("a reference removed and added again under its id holds the new reference", () => {
  const log = [...table, removed(9, "a1", "r-echo"), added(10, "a1", "r-echo", DELTA)];
  expect(fold(log)).toEqual(tableHolding("a1", { "r-delta": held(DELTA), "r-echo": held(DELTA) }));
});

// VTT-1026
test("a referenceChanged sets its place and state and keeps its entry, changing nothing else", () => {
  expect(fold([...table, changed(9, "a1", "r-echo", "kilo")])).toEqual(
    tableHolding("a1", { ...A1_REFS, "r-echo": { entryId: "echo", place: "kilo", state: "" } }));
  expect(fold([...table, changed(9, "a1", "r-delta", "mike", "oscar")])).toEqual(
    tableHolding("a1", { ...A1_REFS, "r-delta": { entryId: "delta", place: "mike", state: "oscar" } }));
});

// VTT-1026
test("a referenceChanged leaves the reference an earlier event carried as it was", () => {
  const a3 = bram({ references: { "r-echo": ECHO } });
  const add = added(10, "a3", "r-delta", ECHO);
  fold([...table, a3, add, changed(11, "a3", "r-echo", "kilo"), changed(12, "a3", "r-delta", "kilo")]);
  expect((a3.payload.value as { actor: { references: Record<string, Reference> } }).actor.references["r-echo"])
    .toMatchObject(held(ECHO));
  expect((add.payload.value as { reference: Reference }).reference).toMatchObject(held(ECHO));
});

// VTT-1026
test("a referenceChanged naming no actor the fold holds, then an id the sheet does not hold, is refused", () => {
  rejects([...table, changed(9, "ghost", "r-echo", "kilo")], 'reference_changed names unknown actor "ghost"');
  rejects([...table, changed(9, "a2", "r-echo", "kilo")], 'actor "a2" does not hold reference "r-echo"');
  rejects([...table, removed(9, "a1", "r-echo"), changed(10, "a1", "r-echo", "kilo")],
    'actor "a1" does not hold reference "r-echo"');
});

const testdata = (name: string) => join(import.meta.dir, "../../contract/testdata", name);
const envelopes = (name: string): Envelope[] =>
  (JSON.parse(readFileSync(testdata(name), "utf8")) as unknown[]).map((e) => fromJson(EnvelopeSchema, e as never));

// VTT-1028
test("the dump after reference changes matches the committed dump, as Go's writeDump does", () => {
  expect(foldToDumpJSON(envelopes("reference_change_log.json"))).toBe(
    readFileSync(testdata("reference_change_dump.json"), "utf8"));
});

// VTT-1028
test("each refusal in the committed reference fixture, the client's fold refuses", () => {
  const log = envelopes("reference_change_log.json");
  const refused = envelopes("reference_change_refused.json");
  expect(refused.length).toBeGreaterThan(0);
  for (const e of refused) expect(() => fold([...log, e])).toThrow(FoldError);
});

// Go's encoding/json writes a map's keys in the byte order of their UTF-8.
const BYTE_ORDER = ["10", "2", "9", "a", "～", "😀"];
const SCRAMBLED = ["😀", "9", "～", "10", "a", "2"];

/** The keys the dump writes at one depth, in its order. */
function keysAt(dump: string, depth: number): string[] {
  const line = new RegExp(`^ {${depth}}"([^"]+)": `);
  return dump.split("\n").flatMap((l) => {
    const m = line.exec(l);
    return m ? [m[1]!] : [];
  });
}

// VTT-1028
test("the dump writes actor ids, attribute keys and reference ids in the byte order of their UTF-8", () => {
  const actors = SCRAMBLED.map((id, i) => env(i + 1, { actorAdded: { actor: { actorId: id } } }));
  expect(keysAt(foldToDumpJSON(actors), 4)).toEqual(BYTE_ORDER);
  const attributes = Object.fromEntries(SCRAMBLED.map((k, i) => [k, i]));
  expect(keysAt(foldToDumpJSON([bram({ attributes })]), 8)).toEqual(BYTE_ORDER);
  const references = Object.fromEntries(SCRAMBLED.map((k) => [k, ECHO]));
  expect(keysAt(foldToDumpJSON([bram({ references })]), 8)).toEqual(BYTE_ORDER);
});
