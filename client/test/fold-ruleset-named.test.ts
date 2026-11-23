import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

// The client's arm for rulesetNamed, mirroring engine.Apply's:
// internal/engine/ruleset_named_test.go holds the server's cases, and
// cmd/vtt/ruleset_named_dump_test.go folds the same contract/testdata log and
// refusals through the server's fold.

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

const named = (seq: number, rulesetId: string, version: string, name: string) =>
  env(seq, { rulesetNamed: { rulesetId, version, name } });

const table: Envelope[] = [
  env(1, { sessionStarted: { name: "S" } }),
  env(2, { sceneCreated: { sceneId: "s1", name: "N", gridWidth: 4, gridHeight: 4 } }),
  env(3, {
    actorAdded: {
      actor: {
        actorId: "a1", name: "Ada", moduleId: "m", kind: "ACTOR_KIND_PARTY_MEMBER",
        attributes: { brawn: 2 }, resources: { stamina: { current: 3, max: 5 } }, abilityIds: ["rally"],
      },
    },
  }),
  env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 }, name: "a cloaked shape" } }),
  env(5, { actorControlGranted: { actorId: "a1", participantId: "p1", kind: "ACTOR_KIND_PARTY_MEMBER" } }),
  env(6, { conditionApplied: { actorId: "a1", conditionId: "marked", source: "trap" } }),
  env(7, { noteUpserted: { key: "k", title: "T", text: "x", visibility: "NOTE_VISIBILITY_PUBLIC" } }),
];

// VTT-979
test("a rulesetNamed names its ruleset's id, version and name, and changes nothing else", () => {
  expect(fold([...table, named(8, "qa-rules", "1", "Plain Rules")])).toEqual(
    { ...fold(table), RulesetID: "qa-rules", RulesetVersion: "1", RulesetName: "Plain Rules" });
});

// VTT-979
test("a later rulesetNamed replaces the ruleset the state names", () => {
  const st = fold([...table, named(8, "qa-rules", "1", "Plain Rules"), named(9, "other-rules", "2", "Other Rules")]);
  expect(st).toEqual({ ...fold(table), RulesetID: "other-rules", RulesetVersion: "2", RulesetName: "Other Rules" });
});

const testdata = (name: string) => join(import.meta.dir, "../../contract/testdata", name);
const envelopes = (name: string): Envelope[] =>
  (JSON.parse(readFileSync(testdata(name), "utf8")) as unknown[]).map((e) => fromJson(EnvelopeSchema, e as never));

// VTT-979 VTT-983
test("the dump after rulesets named matches the committed dump, as Go's writeDump does", () => {
  expect(foldToDumpJSON(envelopes("ruleset_named_log.json"))).toBe(
    readFileSync(testdata("ruleset_named_dump.json"), "utf8"));
});

// VTT-987
test("the dump of a log that names no ruleset writes nothing of one", () => {
  const log = envelopes("ruleset_named_log.json");
  expect(log[2]!.payload.case).toBe("rulesetNamed");
  expect(foldToDumpJSON(log.slice(0, 2))).not.toContain("Ruleset");
  expect(foldToDumpJSON(table)).not.toContain("Ruleset");
});

// VTT-974 VTT-983
test("each rulesetNamed refusal in the committed fixture, the client's fold refuses", () => {
  const log = envelopes("ruleset_named_log.json");
  const refused = envelopes("ruleset_named_refused.json");
  expect(refused.length).toBeGreaterThan(0);
  for (const e of refused) expect(() => fold([...log, e])).toThrow(FoldError);
});
