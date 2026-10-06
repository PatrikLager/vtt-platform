import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fromJson, type JsonObject } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

const qaIdBound = 128;
const qaIdUtf8 = new TextEncoder();

function qaIdBytes(s: string): number {
  return qaIdUtf8.encode(s).length;
}

const qaIdAccepted: [string, string][] = [
  ["one byte", "a"],
  ["127 ascii", "a".repeat(127)],
  ["128 ascii", "S" + "a".repeat(126) + "E"],
  ["64 two-byte runes", "é".repeat(64)],
  ["42 three-byte runes and two ascii", "€".repeat(42) + "ab"],
  ["32 four-byte runes", "😀".repeat(32)],
];

const qaIdRefused: [string, string][] = [
  ["129 ascii", "a".repeat(129)],
  ["1000 ascii", "a".repeat(1000)],
  ["65 two-byte runes", "é".repeat(65)],
  ["one ascii then 64 two-byte runes", "a" + "é".repeat(64)],
  ["43 three-byte runes", "€".repeat(43)],
  ["32 four-byte runes and one ascii", "😀".repeat(32) + "a"],
];

const qaIdKinds = ["SceneCreated", "ActorAdded", "TokenPlaced", "AdventureLoaded"] as const;
type QaIdKind = (typeof qaIdKinds)[number];

const qaIdField: Record<QaIdKind, RegExp> = {
  SceneCreated: /scene[ _.]?id/i,
  ActorAdded: /actor[ _.]?id/i,
  TokenPlaced: /token[ _.]?id/i,
  AdventureLoaded: /adventure[ _.]?id/i,
};

const qaIdHeld: Record<QaIdKind, string | undefined> = {
  SceneCreated: "Scenes",
  ActorAdded: "Actors",
  TokenPlaced: "Tokens",
  AdventureLoaded: undefined,
};

function qaIdScene(id: string, name = "qa"): JsonObject {
  return { sceneCreated: { sceneId: id, name, gridWidth: 3, gridHeight: 3 } };
}

function qaIdActor(id: string, name = "qa", controller = ""): JsonObject {
  const actor: JsonObject = { actorId: id, name, kind: "ACTOR_KIND_NON_PARTY" };
  if (controller !== "") {
    actor["controllerId"] = controller;
    actor["controllerIds"] = [controller];
  }
  return { actorAdded: { actor } };
}

function qaIdToken(id: string, sceneId = "qa-scene", actorId = "qa-actor", placed = true): JsonObject {
  const token: JsonObject = { tokenId: id, sceneId, actorId };
  if (placed) {
    token["position"] = { x: 1, y: 1 };
  }
  return { tokenPlaced: token };
}

function qaIdAdventure(id: string, name = "qa"): JsonObject {
  return { adventureLoaded: { adventureId: id, name } };
}

function qaIdPayloads(kind: QaIdKind, id: string): JsonObject[] {
  switch (kind) {
    case "SceneCreated":
      return [qaIdScene(id)];
    case "ActorAdded":
      return [qaIdActor(id)];
    case "TokenPlaced":
      return [qaIdScene("qa-scene"), qaIdActor("qa-actor"), qaIdToken(id)];
    case "AdventureLoaded":
      return [qaIdAdventure(id)];
  }
}

function qaIdEnvelopes(payloads: JsonObject[]): Envelope[] {
  return payloads.map((p, i) =>
    fromJson(EnvelopeSchema, { eventId: `qa-${i + 1}`, sequence: String(i + 1), ...p }),
  );
}

function qaIdThrown(payloads: JsonObject[]): unknown {
  try {
    fold(qaIdEnvelopes(payloads));
  } catch (e) {
    return e;
  }
  return undefined;
}

function qaIdMessage(err: unknown, id: string): string {
  expect(err).toBeInstanceOf(FoldError);
  const msg = (err as Error).message;
  return id === "" ? msg : msg.replaceAll(id, "<id>");
}

describe("the client's fold bounds an id in UTF-8 bytes", () => {
  for (const kind of qaIdKinds) {
    for (const [label, id] of qaIdAccepted) {
      // VTT-277
      test(`${kind} accepts ${label} (${qaIdBytes(id)} bytes)`, () => {
        expect(qaIdBytes(id)).toBeLessThanOrEqual(qaIdBound);
        const envs = qaIdEnvelopes(qaIdPayloads(kind, id));
        expect(() => fold(envs)).not.toThrow();
        const held = qaIdHeld[kind];
        if (held !== undefined) {
          const dump = JSON.parse(foldToDumpJSON(envs)) as Record<string, Record<string, unknown>>;
          expect(Object.keys(dump[held] ?? {})).toContain(id);
        }
      });
    }
    for (const [label, id] of qaIdRefused) {
      // VTT-277
      test(`${kind} refuses ${label} (${qaIdBytes(id)} bytes)`, () => {
        expect(qaIdBytes(id)).toBeGreaterThan(qaIdBound);
        const msg = qaIdMessage(qaIdThrown(qaIdPayloads(kind, id)), id);
        expect(msg).toMatch(qaIdField[kind]);
        expect(msg).toEndWith(`exceeds ${qaIdBound} bytes`);
      });
    }
    // VTT-278
    test(`${kind} refuses an empty id`, () => {
      const msg = qaIdMessage(qaIdThrown(qaIdPayloads(kind, "")), "");
      if (kind !== "ActorAdded") {
        expect(msg).toMatch(qaIdField[kind]);
        expect(msg).toEndWith("is shorter than 1 bytes");
      }
    });
  }
});

describe("the client's fold checks the id before the later faults", () => {
  const long = "a".repeat(qaIdBound + 1);
  const longName = "a".repeat(257);
  const known = [qaIdScene("qa-scene"), qaIdActor("qa-actor")];
  const cases: [string, JsonObject[], JsonObject[]][] = [
    ["scene id before its name", [qaIdScene(long)], [qaIdScene(long, longName)]],
    ["empty scene id before its name", [qaIdScene("")], [qaIdScene("", longName)]],
    ["actor id before a controller", [qaIdActor(long)], [qaIdActor(long, "qa", "qa-participant")]],
    ["actor id before its name", [qaIdActor(long)], [qaIdActor(long, longName)]],
    ["missing actor id before a controller and its name", [qaIdActor("")], [qaIdActor("", longName, "qa-participant")]],
    [
      "token id before an unknown scene and actor and no position",
      [...known, qaIdToken(long)],
      [qaIdToken(long, "qa-nowhere", "qa-nobody", false)],
    ],
    ["empty token id before an unknown scene", [...known, qaIdToken("")], [qaIdToken("", "qa-nowhere")]],
    ["adventure id before its name", [qaIdAdventure(long)], [qaIdAdventure(long, longName)]],
    ["empty adventure id before its name", [qaIdAdventure("")], [qaIdAdventure("", longName)]],
  ];
  for (const [label, idOnly, combined] of cases) {
    // VTT-277 VTT-278
    test(label, () => {
      const want = qaIdThrown(idOnly);
      expect(want).toBeInstanceOf(FoldError);
      const got = qaIdThrown(combined);
      expect(got).toBeInstanceOf(FoldError);
      expect((got as Error).message).toBe((want as Error).message);
    });
  }
});

type QaIdTool = { name: string; inputSchema: JsonObject };

function qaIdToolDescription(tool: string, path: string[]): string {
  const raw = readFileSync(`${import.meta.dir}/../../contract/gen/tools/tools.json`, "utf8");
  const found = (JSON.parse(raw) as QaIdTool[]).find((t) => t.name === tool);
  expect(found).toBeDefined();
  let node = found?.inputSchema as Record<string, unknown> | undefined;
  for (const p of path) {
    const props = node?.["properties"] as Record<string, unknown> | undefined;
    node = props?.[p] as Record<string, unknown> | undefined;
  }
  const desc = node?.["description"];
  return typeof desc === "string" ? desc : "";
}

describe("the tools state the id bound the client's fold enforces", () => {
  const cases: [string, string[], QaIdKind][] = [
    ["add_actor", ["actor", "actorId"], "ActorAdded"],
    ["place_token", ["tokenId"], "TokenPlaced"],
  ];
  for (const [tool, path, kind] of cases) {
    // VTT-281
    test(tool, () => {
      const m = /At most (\d+) bytes of UTF-8/.exec(qaIdToolDescription(tool, path));
      expect(m).not.toBeNull();
      const n = Number(m?.[1]);
      expect(n).toBe(qaIdBound);
      for (const id of ["a".repeat(n), "é".repeat(n / 2)]) {
        expect(qaIdThrown(qaIdPayloads(kind, id))).toBeUndefined();
        expect(qaIdThrown(qaIdPayloads(kind, id + "a"))).toBeInstanceOf(FoldError);
      }
    });
  }
});
