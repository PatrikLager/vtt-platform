import "./support/dom";
import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { renderSpectator, type ViewExtras } from "../src/view/spectator";
import { namer } from "../src/view/naming";

// The DM's and the agent's Events panel line for each reference event: the
// actor as the window names any actor, the entry and the place by the names
// the window holds, else by their ids.

function env(seq: number, payload: Record<string, unknown>): Envelope {
  return fromJson(EnvelopeSchema, { eventId: `evt-${seq}`, sequence: String(seq), sessionId: "sess-1", ...payload } as never);
}

const CATALOGUE = [
  { id: "echo", name: "Echo Blade", description: "", form: "zulu", values: {} },
  { id: "delta", name: "Delta Shield", description: "", form: "yankee", values: {} },
];
const PLACES = [
  { key: "mike", name: "Mike Rack", description: "", forms: ["zulu", "yankee"], copies: true, states: ["lima", "oscar"] },
  { key: "kilo", name: "Kilo Hook", description: "", forms: ["yankee"], copies: false, states: [] },
];

// Lera's ActorAdded holds r-delta; r-echo is added, both are changed and both
// removed, so a removal's and a change's entry and place are the log's alone.
const LOG: Envelope[] = [
  env(1, { sessionStarted: { name: "Night" } }),
  env(2, { actorAdded: { actor: { actorId: "a1", name: "Lera", kind: "ACTOR_KIND_PARTY_MEMBER",
    references: { "r-delta": { entryId: "delta", place: "kilo" } } } } }),
  env(3, { referenceAdded: { actorId: "a1", referenceId: "r-echo", reference: { entryId: "echo", place: "mike", state: "lima" } } }),
  env(4, { referenceChanged: { actorId: "a1", referenceId: "r-echo", place: "kilo" } }),
  env(5, { referenceChanged: { actorId: "a1", referenceId: "r-delta", place: "mike", state: "oscar" } }),
  env(6, { referenceRemoved: { actorId: "a1", referenceId: "r-echo" } }),
  env(7, { referenceRemoved: { actorId: "a1", referenceId: "r-delta" } }),
];

function ticks(log: Envelope[], extras: ViewExtras): string[] {
  const root = document.createElement("div");
  renderSpectator(root, fold(log), log, "open", { dm: true, ticker: true, ...extras });
  return Array.from(root.querySelectorAll(".ticker .tick"), (n) => n.textContent ?? "").reverse();
}

// VTT-1030
test("the Events panel names a reference's entry and place by the window's names, a removal's and a change's from the log", () => {
  expect(ticks(LOG, { catalogue: CATALOGUE, places: PLACES }).slice(2)).toEqual([
    "Lera now holds Echo Blade in Mike Rack (lima)",
    "Lera's Echo Blade is now in Kilo Hook",
    "Lera's Delta Shield is now in Mike Rack (oscar)",
    "Lera no longer holds Echo Blade in Kilo Hook",
    "Lera no longer holds Delta Shield in Mike Rack",
  ]);
});

// VTT-1030
test("the Events panel names an entry and a place the window holds no name for by its id", () => {
  const ids = [
    "Lera now holds echo in mike (lima)",
    "Lera's echo is now in kilo",
    "Lera's delta is now in mike (oscar)",
    "Lera no longer holds echo in kilo",
    "Lera no longer holds delta in mike",
  ];
  expect(ticks(LOG, {}).slice(2)).toEqual(ids);
  expect(ticks(LOG, { catalogue: [], places: [] }).slice(2)).toEqual(ids);
  const others = {
    catalogue: [{ ...CATALOGUE[0]!, id: "echo-2" }, { ...CATALOGUE[1]!, name: "" }],
    places: [{ ...PLACES[0]!, key: "mike-2" }, { ...PLACES[1]!, name: "" }],
  };
  expect(ticks(LOG, others).slice(2)).toEqual(ids);
});

// VTT-1030
test("the Events panel names a reference's actor as it names any actor", () => {
  const log = [
    ...LOG.slice(0, 2),
    env(3, { actorAdded: { actor: { actorId: "a2", name: "Goblin Cutter", kind: "ACTOR_KIND_NON_PARTY" } } }),
    env(4, { sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 4, gridHeight: 4 } }),
    env(5, { tokenPlaced: { tokenId: "t2", sceneId: "s1", actorId: "a2", position: { x: 1, y: 1 }, gmName: "the cutter" } }),
    env(6, { referenceAdded: { actorId: "a2", referenceId: "r-delta", reference: { entryId: "delta", place: "kilo" } } }),
    env(7, { referenceChanged: { actorId: "a2", referenceId: "r-delta", place: "mike", state: "lima" } }),
    env(8, { referenceRemoved: { actorId: "a2", referenceId: "r-delta" } }),
  ];
  expect(ticks(log, { catalogue: CATALOGUE, places: PLACES }).slice(5)).toEqual([
    "the cutter now holds Delta Shield in Kilo Hook",
    "the cutter's Delta Shield is now in Mike Rack (lima)",
    "the cutter no longer holds Delta Shield in Mike Rack",
  ]);
});

// VTT-1030
test("the namer holds, for each removal and change, the entry and place its reference held just before it", () => {
  const log = [
    ...LOG,
    env(8, { referenceAdded: { actorId: "a1", referenceId: "r-echo", reference: { entryId: "delta", place: "kilo" } } }),
    env(9, { referenceChanged: { actorId: "a1", referenceId: "r-echo", place: "mike", state: "oscar" } }),
    env(10, { referenceRemoved: { actorId: "a1", referenceId: "r-echo" } }),
  ];
  const names = namer(fold(log), log, true, undefined, undefined, undefined, CATALOGUE, PLACES);
  expect(log.map((e) => names.held(e))).toEqual([
    undefined, undefined, undefined,
    { entryId: "echo", place: "mike" },
    { entryId: "delta", place: "kilo" },
    { entryId: "echo", place: "kilo" },
    { entryId: "delta", place: "mike" },
    undefined,
    { entryId: "delta", place: "kilo" },
    { entryId: "delta", place: "mike" },
  ]);
  expect(names.entry("echo")).toBe("Echo Blade");
  expect(names.place("kilo")).toBe("Kilo Hook");
  // A log no fold accepts: the namer still holds nothing it was not told.
  const odd = [env(1, { referenceAdded: { actorId: "a1", referenceId: "r-bare" } }),
    env(2, { referenceRemoved: { actorId: "a1", referenceId: "r-bare" } }),
    env(3, { referenceChanged: { actorId: "a1", referenceId: "r-never", place: "kilo" } }),
    env(4, { referenceRemoved: { actorId: "a1", referenceId: "r-never" } })];
  const loose = namer(fold([]), odd, true, undefined);
  expect(odd.map((e) => loose.held(e))).toEqual([undefined, undefined, undefined, undefined]);
});

// VTT-1030
test("a removal's and a change's line name what the reference held then, though its id is added again as another entry", () => {
  const log = [
    ...LOG.slice(0, 4),
    env(5, { referenceRemoved: { actorId: "a1", referenceId: "r-echo" } }),
    env(6, { referenceAdded: { actorId: "a1", referenceId: "r-echo", reference: { entryId: "delta", place: "mike" } } }),
  ];
  expect(ticks(log, { catalogue: CATALOGUE, places: PLACES }).slice(2)).toEqual([
    "Lera now holds Echo Blade in Mike Rack (lima)",
    "Lera's Echo Blade is now in Kilo Hook",
    "Lera no longer holds Echo Blade in Kilo Hook",
    "Lera now holds Delta Shield in Mike Rack",
  ]);
});

// VTT-1030
test("each change's line names the place that change moved the reference to, whatever later events do", () => {
  const log = [
    ...LOG.slice(0, 3),
    env(4, { referenceChanged: { actorId: "a1", referenceId: "r-echo", place: "kilo" } }),
    env(5, { referenceChanged: { actorId: "a1", referenceId: "r-echo", place: "mike", state: "oscar" } }),
  ];
  expect(ticks(log, { catalogue: CATALOGUE, places: PLACES }).slice(3)).toEqual([
    "Lera's Echo Blade is now in Kilo Hook",
    "Lera's Echo Blade is now in Mike Rack (oscar)",
  ]);
});
