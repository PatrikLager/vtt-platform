import { describe, expect, test } from "bun:test";
import { fromJson, type JsonObject } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

const qaRuleBound = 128;
const qaRuleUtf8 = new TextEncoder();
const qaRuleActorId = "qa-actor";

function qaRuleBytes(s: string): number {
  return qaRuleUtf8.encode(s).length;
}

const qaRuleAccepted: [string, string][] = [
  ["one byte", "a"],
  ["127 ascii", "a".repeat(127)],
  ["128 ascii", "S" + "a".repeat(126) + "E"],
  ["64 two-byte runes", "é".repeat(64)],
  ["42 three-byte runes and two ascii", "€".repeat(42) + "ab"],
  ["32 four-byte runes", "😀".repeat(32)],
];

const qaRuleRefused: [string, string][] = [
  ["129 ascii", "a".repeat(129)],
  ["70000 ascii", "a".repeat(70000)],
  ["65 two-byte runes", "é".repeat(65)],
  ["one ascii then 64 two-byte runes", "a" + "é".repeat(64)],
  ["43 three-byte runes", "€".repeat(43)],
  ["32 four-byte runes and one ascii", "😀".repeat(32) + "a"],
];

const qaRuleKinds = ["ConditionApplied", "AbilityUsed"] as const;
type QaRuleKind = (typeof qaRuleKinds)[number];

const qaRuleField: Record<QaRuleKind, RegExp> = {
  ConditionApplied: /condition[ _.]?id/i,
  AbilityUsed: /ability[ _.]?id/i,
};

function qaRuleActor(): JsonObject {
  return { actorAdded: { actor: { actorId: qaRuleActorId, name: "qa", kind: "ACTOR_KIND_NON_PARTY" } } };
}

function qaRuleEvent(kind: QaRuleKind, id: string, actorId = qaRuleActorId): JsonObject {
  switch (kind) {
    case "ConditionApplied":
      return { conditionApplied: { actorId, conditionId: id, source: "qa-source" } };
    case "AbilityUsed":
      return { abilityUsed: { actorId, abilityId: id, targetIds: [actorId], outcomeSummary: "qa" } };
  }
}

function qaRuleEnvelopes(payloads: JsonObject[]): Envelope[] {
  return payloads.map((p, i) =>
    fromJson(EnvelopeSchema, { eventId: `qa-${i + 1}`, sequence: String(i + 1), ...p }),
  );
}

function qaRuleThrown(payloads: JsonObject[]): unknown {
  try {
    fold(qaRuleEnvelopes(payloads));
  } catch (e) {
    return e;
  }
  return undefined;
}

function qaRuleMessage(err: unknown, id: string): string {
  expect(err).toBeInstanceOf(FoldError);
  const msg = (err as Error).message;
  return id === "" ? msg : msg.replaceAll(id, "<id>");
}

function qaRuleHeldConditionIds(envs: Envelope[]): string[] {
  const dump = JSON.parse(foldToDumpJSON(envs)) as Record<string, unknown>;
  const conditions = (dump["Conditions"] ?? {}) as Record<string, { ID?: string }[]>;
  return (conditions[qaRuleActorId] ?? []).map((c) => c.ID ?? "");
}

describe("the client's fold bounds a ruleset id in UTF-8 bytes", () => {
  for (const kind of qaRuleKinds) {
    for (const [label, id] of qaRuleAccepted) {
      // VTT-285
      test(`${kind} accepts ${label} (${qaRuleBytes(id)} bytes)`, () => {
        expect(qaRuleBytes(id)).toBeLessThanOrEqual(qaRuleBound);
        const envs = qaRuleEnvelopes([qaRuleActor(), qaRuleEvent(kind, id)]);
        expect(() => fold(envs)).not.toThrow();
        if (kind === "ConditionApplied") {
          expect(qaRuleHeldConditionIds(envs)).toContain(id);
        }
      });
    }
    for (const [label, id] of qaRuleRefused) {
      // VTT-285
      test(`${kind} refuses ${label} (${qaRuleBytes(id)} bytes)`, () => {
        expect(qaRuleBytes(id)).toBeGreaterThan(qaRuleBound);
        const msg = qaRuleMessage(qaRuleThrown([qaRuleActor(), qaRuleEvent(kind, id)]), id);
        expect(msg).toMatch(qaRuleField[kind]);
        expect(msg).toEndWith(`exceeds ${qaRuleBound} bytes`);
      });
    }
    // VTT-286
    test(`${kind} refuses an empty id`, () => {
      const msg = qaRuleMessage(qaRuleThrown([qaRuleActor(), qaRuleEvent(kind, "")]), "");
      expect(msg).toMatch(qaRuleField[kind]);
      expect(msg).toEndWith("is shorter than 1 bytes");
    });
  }
});

describe("the client's fold checks a ruleset id before the later faults", () => {
  const long = "a".repeat(qaRuleBound + 1);
  for (const kind of qaRuleKinds) {
    for (const [label, id] of [
      ["long", long],
      ["empty", ""],
    ] as [string, string][]) {
      // VTT-285 VTT-286
      test(`${kind} with a ${label} id naming an unknown actor is refused for its id`, () => {
        const want = qaRuleThrown([qaRuleActor(), qaRuleEvent(kind, id)]);
        expect(want).toBeInstanceOf(FoldError);
        const got = qaRuleThrown([qaRuleEvent(kind, id)]);
        expect(got).toBeInstanceOf(FoldError);
        expect(qaRuleMessage(got, id)).toBe(qaRuleMessage(want, id));
      });
    }
  }
  for (const [label, id] of qaRuleAccepted.slice(2)) {
    // VTT-285
    test(`AbilityUsed of ${label} naming an unknown actor is accepted`, () => {
      expect(qaRuleThrown([qaRuleEvent("AbilityUsed", id)])).toBeUndefined();
    });
  }
});
