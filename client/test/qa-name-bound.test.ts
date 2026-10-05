import { describe, expect, test } from "bun:test";
import { fromJson, type JsonObject } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

const qaNameBound = 256;
const qaNameUtf8 = new TextEncoder();

function qaNameBytes(s: string): number {
  return qaNameUtf8.encode(s).length;
}

const qaNameAccepted: [string, string][] = [
  ["empty", ""],
  ["one byte", "a"],
  ["255 ascii", "a".repeat(255)],
  ["256 ascii", "S" + "a".repeat(254) + "E"],
  ["128 two-byte runes", "é".repeat(128)],
  ["85 three-byte runes and one ascii", "€".repeat(85) + "a"],
  ["64 four-byte runes", "😀".repeat(64)],
];

const qaNameRefused: [string, string][] = [
  ["257 ascii", "a".repeat(257)],
  ["1000 ascii", "a".repeat(1000)],
  ["129 two-byte runes", "é".repeat(129)],
  ["one ascii then 128 two-byte runes", "a" + "é".repeat(128)],
  ["86 three-byte runes", "€".repeat(86)],
  ["64 four-byte runes and one ascii", "😀".repeat(64) + "a"],
];

const qaNameKinds = ["SessionStarted", "SceneCreated", "ActorAdded", "AdventureLoaded"] as const;
type QaNameKind = (typeof qaNameKinds)[number];

function qaNameActor(id: string, name: string, controller?: string): JsonObject {
  const actor: JsonObject = { actorId: id, name, kind: "ACTOR_KIND_NON_PARTY" };
  if (controller !== undefined) {
    actor["controllerId"] = controller;
    actor["controllerIds"] = [controller];
  }
  return { actorAdded: { actor } };
}

function qaNamePayload(kind: QaNameKind, name: string): JsonObject {
  switch (kind) {
    case "SessionStarted":
      return { sessionStarted: { name } };
    case "SceneCreated":
      return { sceneCreated: { sceneId: "qa-scene", name, gridWidth: 2, gridHeight: 2 } };
    case "ActorAdded":
      return qaNameActor("qa-actor", name);
    case "AdventureLoaded":
      return { adventureLoaded: { adventureId: "qa-adventure", name } };
  }
}

function qaNameEnvelopes(...payloads: JsonObject[]): Envelope[] {
  return payloads.map((p, i) =>
    fromJson(EnvelopeSchema, { eventId: `qa-${i + 1}`, sequence: String(i + 1), ...p }),
  );
}

function qaNameThrown(envs: Envelope[]): unknown {
  try {
    fold(envs);
  } catch (e) {
    return e;
  }
  return undefined;
}

describe("both folds bound a name in UTF-8 bytes: the client's fold", () => {
  for (const kind of qaNameKinds) {
    for (const [label, name] of qaNameAccepted) {
      // SPEC-018 How it works: the table's "May be empty" column, yes for a name.
      // VTT-274
      test(`${kind} accepts ${label} (${qaNameBytes(name)} bytes)`, () => {
        expect(qaNameBytes(name)).toBeLessThanOrEqual(qaNameBound);
        const envs = qaNameEnvelopes(qaNamePayload(kind, name));
        expect(qaNameThrown(envs)).toBeUndefined();
        if (kind !== "AdventureLoaded" && name !== "") {
          expect(foldToDumpJSON(envs)).toContain(name);
        }
      });
    }
    for (const [label, name] of qaNameRefused) {
      // SPEC-018 How it works: "checkLen ... throws FoldError reading
      // <field> exceeds <max> bytes".
      // VTT-274
      test(`${kind} refuses ${label} (${qaNameBytes(name)} bytes)`, () => {
        expect(qaNameBytes(name)).toBeGreaterThan(qaNameBound);
        const err = qaNameThrown(qaNameEnvelopes(qaNamePayload(kind, name)));
        expect(err).toBeInstanceOf(FoldError);
        const msg = (err as Error).message.replaceAll(name, "<name>");
        expect(msg).toContain("name");
        expect(msg).toEndWith(`exceeds ${qaNameBound} bytes`);
      });
    }
  }
});

describe("the client's fold checks the earlier fault before the name", () => {
  const long = "a".repeat(qaNameBound + 1);
  const cases: [string, JsonObject, JsonObject, JsonObject][] = [
    [
      "duplicate scene",
      qaNamePayload("SceneCreated", "x"),
      qaNamePayload("SceneCreated", "y"),
      qaNamePayload("SceneCreated", long),
    ],
    [
      "actor without an id",
      qaNamePayload("SceneCreated", "x"),
      qaNameActor("", "y"),
      qaNameActor("", long),
    ],
    [
      "duplicate actor",
      qaNameActor("qa-actor", "x"),
      qaNameActor("qa-actor", "y"),
      qaNameActor("qa-actor", long),
    ],
    [
      "actor naming a controller",
      qaNamePayload("SceneCreated", "x"),
      qaNameActor("qa-other", "y", "qa-participant"),
      qaNameActor("qa-other", long, "qa-participant"),
    ],
    [
      "session already open",
      qaNamePayload("SessionStarted", "x"),
      qaNamePayload("SessionStarted", "y"),
      qaNamePayload("SessionStarted", long),
    ],
  ];
  for (const [label, first, short, faulty] of cases) {
    // SPEC-018 How it works: "fold.ts calls checkLen(what, s, min, max) for
    // each of the ten fields ... in the same order within each arm".
    // VTT-274
    test(label, () => {
      const want = qaNameThrown(qaNameEnvelopes(first, short));
      expect(want).toBeInstanceOf(FoldError);
      const got = qaNameThrown(qaNameEnvelopes(first, faulty));
      expect(got).toBeInstanceOf(FoldError);
      expect((got as Error).message).toBe((want as Error).message);
    });
  }
});
