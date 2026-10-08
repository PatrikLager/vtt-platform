import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fromJson, type JsonObject } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { FoldError } from "../src/state";

const qaCmdBound = 128;
const qaCmdUtf8 = new TextEncoder();

function qaCmdBytes(s: string): number {
  return qaCmdUtf8.encode(s).length;
}

const qaCmdAccepted: [string, string][] = [
  ["one byte", "a"],
  ["127 ascii", "a".repeat(127)],
  ["128 ascii", "S" + "a".repeat(126) + "E"],
  ["64 two-byte runes", "é".repeat(64)],
  ["42 three-byte runes and two ascii", "€".repeat(42) + "ab"],
  ["32 four-byte runes", "😀".repeat(32)],
];

const qaCmdRefused: [string, string][] = [
  ["129 ascii", "a".repeat(129)],
  ["1000 ascii", "a".repeat(1000)],
  ["65 two-byte runes", "é".repeat(65)],
  ["one ascii then 64 two-byte runes", "a" + "é".repeat(64)],
  ["43 three-byte runes", "€".repeat(43)],
  ["32 four-byte runes and one ascii", "😀".repeat(32) + "a"],
];

const qaCmdField = {
  participant: /participant[ _.]?id/i,
  module: /module[ _.]?id/i,
  resource: /resource/i,
  attribute: /attribute/i,
  object: /object[ _.]?id/i,
  session: /session[ _.]?id/i,
  scene: /scene[ _.]?id/i,
};

type QaCmdActorEdit = (a: JsonObject) => void;

function qaCmdScene(id: string, name: string, ...objectIds: string[]): JsonObject {
  const objects = objectIds.map((objectId, i) => ({
    objectId,
    kind: "crate",
    at: { x: i % 8, y: Math.floor(i / 8) },
    width: 1,
    height: 1,
    art: "qa-crate",
  }));
  return { sceneCreated: { sceneId: id, name, gridWidth: 8, gridHeight: 8, objects } };
}

function qaCmdActor(id: string, ...edits: QaCmdActorEdit[]): JsonObject {
  const actor: JsonObject = { actorId: id, name: "qa", kind: "ACTOR_KIND_NON_PARTY" };
  for (const edit of edits) {
    edit(actor);
  }
  return { actorAdded: { actor } };
}

const qaCmdModule =
  (v: string): QaCmdActorEdit =>
  (a) => {
    a["moduleId"] = v;
  };

const qaCmdResources =
  (...keys: string[]): QaCmdActorEdit =>
  (a) => {
    const r: JsonObject = {};
    for (const k of keys) {
      r[k] = { current: 1, max: 2 };
    }
    a["resources"] = r;
  };

const qaCmdAttributes =
  (...keys: string[]): QaCmdActorEdit =>
  (a) => {
    const r: JsonObject = {};
    for (const k of keys) {
      r[k] = 3;
    }
    a["attributes"] = r;
  };

const qaCmdNamed =
  (name: string): QaCmdActorEdit =>
  (a) => {
    a["name"] = name;
  };

const qaCmdControlled: QaCmdActorEdit = (a) => {
  a["controllerId"] = "qa-participant";
  a["controllerIds"] = ["qa-participant"];
};

function qaCmdGrant(actorId: string, participantId: string): JsonObject {
  return { actorControlGranted: { actorId, participantId, kind: "ACTOR_KIND_PARTY_MEMBER" } };
}

function qaCmdRevoke(actorId: string, participantId: string): JsonObject {
  return { actorControlRevoked: { actorId, participantId } };
}

function qaCmdSession(sessionId: string, name: string): JsonObject {
  return { sessionId, sessionStarted: { name } };
}

function qaCmdPlace(tokenId: string, sceneId: string, actorId: string): JsonObject {
  return { tokenPlaced: { tokenId, sceneId, actorId, position: { x: 1, y: 1 } } };
}

function qaCmdMove(tokenId: string, sceneId: string, to: boolean, reason = ""): JsonObject {
  const moved: JsonObject = { tokenId, sceneId, from: { x: 1, y: 1 }, reason };
  if (to) {
    moved["to"] = { x: 2, y: 1 };
  }
  return { tokenMoved: moved };
}

function qaCmdEnvelopes(payloads: JsonObject[]): Envelope[] {
  return payloads.map((p, i) =>
    fromJson(EnvelopeSchema, { eventId: `qa-cmd-${i + 1}`, sequence: String(i + 1), ...p }),
  );
}

interface QaCmdView {
  Scenes: Record<string, { Objects: { ObjectID: string }[] }>;
  Actors: Record<
    string,
    {
      moduleId: string;
      controllerIds: string[];
      attributes: Record<string, number>;
      resources: Record<string, unknown>;
    }
  >;
  Tokens: Record<string, { X: number; Y: number }>;
  Sessions: { ID: string }[];
}

function qaCmdFold(payloads: JsonObject[]): QaCmdView {
  return fold(qaCmdEnvelopes(payloads)) as unknown as QaCmdView;
}

function qaCmdThrown(payloads: JsonObject[]): unknown {
  try {
    fold(qaCmdEnvelopes(payloads));
  } catch (e) {
    return e;
  }
  return undefined;
}

function qaCmdMessage(payloads: JsonObject[], ...values: string[]): string {
  const err = qaCmdThrown(payloads);
  expect(err).toBeInstanceOf(FoldError);
  let msg = (err as Error).message;
  for (const v of values) {
    if (v !== "") {
      msg = msg.replaceAll(v, "<v>");
    }
  }
  return msg;
}

function qaCmdOver(payloads: JsonObject[], field: RegExp, ...values: string[]): void {
  const msg = qaCmdMessage(payloads, ...values);
  expect(msg).toMatch(field);
  expect(msg).toEndWith(`exceeds ${qaCmdBound} bytes`);
}

function qaCmdEmpty(payloads: JsonObject[], field: RegExp): void {
  const msg = qaCmdMessage(payloads);
  expect(msg).toMatch(field);
  expect(msg).toEndWith("is shorter than 1 bytes");
}

const qaCmdBoard = (): JsonObject[] => [
  qaCmdScene("qa-s", "qa"),
  qaCmdActor("qa-a"),
  qaCmdPlace("qa-t", "qa-s", "qa-a"),
];

describe("the client's fold bounds a control event's participant id", () => {
  for (const [label, id] of qaCmdAccepted) {
    // VTT-289
    test(`grant and revoke accept ${label} (${qaCmdBytes(id)} bytes)`, () => {
      expect(qaCmdBytes(id)).toBeLessThanOrEqual(qaCmdBound);
      const granted = [qaCmdActor("qa-a"), qaCmdGrant("qa-a", "qa-other"), qaCmdGrant("qa-a", id)];
      expect(qaCmdFold(granted).Actors["qa-a"]?.controllerIds).toEqual(["qa-other", id]);
      const revoked = qaCmdFold([...granted, qaCmdRevoke("qa-a", id)]);
      expect(revoked.Actors["qa-a"]?.controllerIds).toEqual(["qa-other"]);
    });
  }
  for (const [label, id] of qaCmdRefused) {
    // VTT-289
    test(`grant refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      expect(qaCmdBytes(id)).toBeGreaterThan(qaCmdBound);
      qaCmdOver([qaCmdActor("qa-a"), qaCmdGrant("qa-a", id)], qaCmdField.participant, id);
    });
    // VTT-289
    test(`revoke refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([qaCmdActor("qa-a"), qaCmdRevoke("qa-a", id)], qaCmdField.participant, id);
    });
  }
});

describe("the client's fold bounds an actor's module id and key names", () => {
  for (const [label, id] of qaCmdAccepted) {
    // VTT-290
    test(`accepts ${label} (${qaCmdBytes(id)} bytes) in each`, () => {
      const st = qaCmdFold([
        qaCmdActor("qa-a", qaCmdModule(id), qaCmdResources("focus", id), qaCmdAttributes(id, "vim")),
      ]);
      const a = st.Actors["qa-a"];
      expect(a?.moduleId).toBe(id);
      expect(Object.keys(a?.resources ?? {}).sort()).toEqual(["focus", id].sort());
      expect(Object.keys(a?.attributes ?? {}).sort()).toEqual([id, "vim"].sort());
    });
  }
  // VTT-290
  test("accepts an actor with no module id", () => {
    const st = qaCmdFold([qaCmdActor("qa-a", qaCmdModule(""), qaCmdResources("focus"), qaCmdAttributes("vim"))]);
    expect(st.Actors["qa-a"]).toBeDefined();
  });
  for (const [label, id] of qaCmdRefused) {
    // VTT-290
    test(`module id refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([qaCmdActor("qa-a", qaCmdModule(id))], qaCmdField.module, id);
    });
    // VTT-290
    test(`resource name refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([qaCmdActor("qa-a", qaCmdResources("qa-short", id, "qa-other"))], qaCmdField.resource, id);
    });
    // VTT-290
    test(`attribute name refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([qaCmdActor("qa-a", qaCmdAttributes("qa-short", id, "qa-other"))], qaCmdField.attribute, id);
    });
  }
  // VTT-297
  test("an empty resource name is refused", () => {
    qaCmdEmpty([qaCmdActor("qa-a", qaCmdResources("qa-short", "", "qa-other"))], qaCmdField.resource);
  });
  // VTT-297
  test("an empty attribute name is refused", () => {
    qaCmdEmpty([qaCmdActor("qa-a", qaCmdAttributes("qa-short", "", "qa-other"))], qaCmdField.attribute);
  });
});

describe("the client's fold bounds a scene's object ids", () => {
  for (const [label, id] of qaCmdAccepted) {
    // VTT-291 VTT-292
    test(`accepts ${label} (${qaCmdBytes(id)} bytes) among distinct ids`, () => {
      const ids = ["qa-1", id, "qa-3"];
      const st = qaCmdFold([qaCmdScene("qa-s", "qa", ...ids)]);
      expect(st.Scenes["qa-s"]?.Objects.map((o) => o.ObjectID)).toEqual(ids);
    });
  }
  // VTT-291 VTT-292
  test("accepts three ids at the bound differing in the last byte", () => {
    const stem = "é".repeat(63) + "x";
    const ids = [stem + "a", stem + "b", stem + "c"];
    const st = qaCmdFold([qaCmdScene("qa-s", "qa", ...ids)]);
    expect(st.Scenes["qa-s"]?.Objects.map((o) => o.ObjectID)).toEqual(ids);
  });
  for (const [label, id] of qaCmdRefused) {
    // VTT-291
    test(`refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([qaCmdScene("qa-s", "qa", "qa-1", id, "qa-3")], qaCmdField.object, id);
    });
  }
  // VTT-292
  test("refuses an empty object id", () => {
    qaCmdEmpty([qaCmdScene("qa-s", "qa", "qa-1", "", "qa-3")], qaCmdField.object);
  });
  const edge = "😀".repeat(32);
  const repeats: [string, string[]][] = [
    ["adjacent", ["qa-1", "qa-2", "qa-2"]],
    ["apart", ["qa-1", "qa-2", "qa-3", "qa-2"]],
    ["first and last", ["qa-1", "qa-2", "qa-3", "qa-1"]],
    ["at the bound", [edge, "qa-2", edge]],
  ];
  for (const [label, ids] of repeats) {
    // VTT-292
    test(`refuses a repeated object id ${label}`, () => {
      expect(qaCmdThrown([qaCmdScene("qa-s", "qa", ...ids)])).toBeInstanceOf(FoldError);
    });
  }
});

describe("the client's fold bounds a session id and a move's scene id", () => {
  for (const [label, id] of qaCmdAccepted) {
    // VTT-293
    test(`session started accepts ${label} (${qaCmdBytes(id)} bytes)`, () => {
      expect(qaCmdFold([qaCmdSession(id, "qa")]).Sessions.map((s) => s.ID)).toEqual([id]);
    });
    // VTT-293
    test(`token moved accepts ${label} (${qaCmdBytes(id)} bytes)`, () => {
      const st = qaCmdFold([
        qaCmdScene(id, "qa"),
        qaCmdActor("qa-a"),
        qaCmdPlace("qa-t", id, "qa-a"),
        qaCmdMove("qa-t", id, true),
      ]);
      expect(st.Tokens["qa-t"]).toMatchObject({ X: 2, Y: 1 });
    });
  }
  for (const [label, id] of qaCmdRefused) {
    // VTT-293
    test(`session started refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([qaCmdSession(id, "qa")], qaCmdField.session, id);
    });
    // VTT-293
    test(`token moved refuses ${label} (${qaCmdBytes(id)} bytes)`, () => {
      qaCmdOver([...qaCmdBoard(), qaCmdMove("qa-t", id, true)], qaCmdField.scene, id);
    });
  }
  // VTT-297
  test("session started refuses an empty session id", () => {
    qaCmdEmpty([qaCmdSession("", "qa")], qaCmdField.session);
  });
  // VTT-297
  test("token moved refuses an empty scene id", () => {
    qaCmdEmpty([...qaCmdBoard(), qaCmdMove("qa-t", "", true)], qaCmdField.scene);
  });
});

type QaCmdOrder = [string, JsonObject[], JsonObject, JsonObject];

function qaCmdOrder(cases: QaCmdOrder[]): void {
  for (const [label, setup, alone, both] of cases) {
    // VTT-289 VTT-290 VTT-291 VTT-292 VTT-293 VTT-297
    test(label, () => {
      const want = qaCmdThrown([...setup, alone]);
      expect(want).toBeInstanceOf(FoldError);
      const got = qaCmdThrown([...setup, both]);
      expect(got).toBeInstanceOf(FoldError);
      expect((got as Error).message).toBe((want as Error).message);
    });
  }
}

describe("the client's fold checks each new bound in the arm's order", () => {
  const long = "a".repeat(129);
  const longName = "a".repeat(257);
  const known = [qaCmdActor("qa-a")];
  const sceneKnown = [qaCmdScene("qa-s", "qa")];
  const open = [qaCmdSession("qa-first", "qa")];
  qaCmdOrder([
    ["grant long participant before unknown actor", known, qaCmdGrant("qa-a", long), qaCmdGrant("qa-nobody", long)],
    ["grant missing participant before unknown actor", known, qaCmdGrant("qa-a", ""), qaCmdGrant("qa-nobody", "")],
    ["revoke long participant before unknown actor", known, qaCmdRevoke("qa-a", long), qaCmdRevoke("qa-nobody", long)],
    ["revoke missing participant before unknown actor", known, qaCmdRevoke("qa-a", ""), qaCmdRevoke("qa-nobody", "")],
    ["actor id before a long module id", [], qaCmdActor(long), qaCmdActor(long, qaCmdModule(long))],
    ["duplicate actor before a long module id", known, qaCmdActor("qa-a"), qaCmdActor("qa-a", qaCmdModule(long))],
    [
      "declared controller before a long module id",
      [],
      qaCmdActor("qa-b", qaCmdControlled),
      qaCmdActor("qa-b", qaCmdControlled, qaCmdModule(long)),
    ],
    [
      "actor name before a long module id",
      [],
      qaCmdActor("qa-b", qaCmdNamed(longName)),
      qaCmdActor("qa-b", qaCmdNamed(longName), qaCmdModule(long)),
    ],
    [
      "long module id before a long resource name",
      [],
      qaCmdActor("qa-b", qaCmdModule(long)),
      qaCmdActor("qa-b", qaCmdModule(long), qaCmdResources(long)),
    ],
    [
      "long module id before an empty attribute name",
      [],
      qaCmdActor("qa-b", qaCmdModule(long)),
      qaCmdActor("qa-b", qaCmdModule(long), qaCmdAttributes("")),
    ],
    [
      "long resource name before a long attribute name",
      [],
      qaCmdActor("qa-b", qaCmdResources(long)),
      qaCmdActor("qa-b", qaCmdResources(long), qaCmdAttributes(long)),
    ],
    [
      "empty resource name before a long attribute name",
      [],
      qaCmdActor("qa-b", qaCmdResources("")),
      qaCmdActor("qa-b", qaCmdResources(""), qaCmdAttributes(long)),
    ],
    ["scene id before a long object id", [], qaCmdScene(long, "qa"), qaCmdScene(long, "qa", "qa-1", long)],
    ["scene id before a repeated object id", [], qaCmdScene(long, "qa"), qaCmdScene(long, "qa", "qa-1", "qa-1")],
    [
      "duplicate scene before an empty object id",
      sceneKnown,
      qaCmdScene("qa-s", "qa"),
      qaCmdScene("qa-s", "qa", "qa-1", ""),
    ],
    [
      "scene name before a long object id",
      [],
      qaCmdScene("qa-s", longName),
      qaCmdScene("qa-s", longName, "qa-1", long),
    ],
    [
      "scene name before a repeated object id",
      [],
      qaCmdScene("qa-s", longName),
      qaCmdScene("qa-s", longName, "qa-1", "qa-1"),
    ],
    [
      "a long object id before a later repeat",
      [],
      qaCmdScene("qa-s", "qa", "qa-1", long),
      qaCmdScene("qa-s", "qa", "qa-1", long, "qa-1"),
    ],
    ["open session before a long session id", open, qaCmdSession("qa-second", "qa"), qaCmdSession(long, "qa")],
    ["open session before an empty session id", open, qaCmdSession("qa-second", "qa"), qaCmdSession("", "qa")],
    ["session name before a long session id", [], qaCmdSession("qa-s", longName), qaCmdSession(long, longName)],
    [
      "unknown token before a long move scene id",
      qaCmdBoard(),
      qaCmdMove("qa-nope", "qa-s", true),
      qaCmdMove("qa-nope", long, true),
    ],
    ["no destination before an empty move scene id", qaCmdBoard(), qaCmdMove("qa-t", "qa-s", false), qaCmdMove("qa-t", "", false)],
    [
      "long reason before a long move scene id",
      qaCmdBoard(),
      qaCmdMove("qa-t", "qa-s", true, longName),
      qaCmdMove("qa-t", long, true, longName),
    ],
  ]);
});

type QaCmdTool = { name: string; inputSchema: JsonObject };

function qaCmdToolDescription(tool: string, path: string[]): string {
  const raw = readFileSync(`${import.meta.dir}/../../contract/gen/tools/tools.json`, "utf8");
  const found = (JSON.parse(raw) as QaCmdTool[]).find((t) => t.name === tool);
  expect(found).toBeDefined();
  let node = found?.inputSchema as Record<string, unknown> | undefined;
  for (const p of path) {
    const props = node?.["properties"] as Record<string, unknown> | undefined;
    node = props?.[p] as Record<string, unknown> | undefined;
  }
  const desc = node?.["description"];
  return typeof desc === "string" ? desc : "";
}

describe("add_actor states the bound the client's fold enforces", () => {
  const cases: [string, (v: string) => QaCmdActorEdit, boolean][] = [
    ["moduleId", qaCmdModule, false],
    ["resources", (v) => qaCmdResources("qa-short", v), true],
    ["attributes", (v) => qaCmdAttributes("qa-short", v), true],
  ];
  for (const [field, edit, keys] of cases) {
    // VTT-296
    test(field, () => {
      const desc = qaCmdToolDescription("add_actor", ["actor", field]);
      const m = /At most (\d+) bytes of UTF-8/.exec(desc);
      expect(m).not.toBeNull();
      const n = Number(m?.[1]);
      expect(n).toBe(qaCmdBound);
      for (const v of ["a".repeat(n), "é".repeat(n / 2)]) {
        expect(qaCmdThrown([qaCmdActor("qa-a", edit(v))])).toBeUndefined();
        expect(qaCmdThrown([qaCmdActor("qa-a", edit(v + "a"))])).toBeInstanceOf(FoldError);
      }
      if (keys) {
        expect(desc).toMatch(/no empty name/);
        expect(qaCmdThrown([qaCmdActor("qa-a", edit(""))])).toBeInstanceOf(FoldError);
      }
    });
  }
});
