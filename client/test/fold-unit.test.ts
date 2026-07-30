import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { FoldError } from "../src/state";

// The golden corpus cannot reach every fold variant: three of the seven
// scenarios roll dice and are excluded until a roller seam exists (see
// scenarios/goldens/README.md), which leaves resourceChanged,
// conditionApplied/Removed and the ability/adventure no-ops unexercised.
//
// Shipping them on the strength of "the corpus is green" would be shipping
// untested code behind a passing gate. These are hand-written against
// internal/engine/apply.go's behaviour instead.

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

/** A minimal world: one session, one actor carrying one resource. */
function worldWithResource(current: number, max: number): Envelope[] {
  return [
    env(1, { sessionStarted: { name: "S" } }),
    env(2, {
      actorAdded: {
        actor: { actorId: "a1", name: "A", resources: { hp: { current, max } } },
      },
    }),
  ];
}

test("resourceChanged clamps at zero rather than going negative", () => {
  const st = fold([
    ...worldWithResource(3, 10),
    env(3, { resourceChanged: { actorId: "a1", resource: "hp", delta: -5, newValue: 0 } }),
  ]);
  expect(st.Actors["a1"]!.resources["hp"]).toEqual({ current: 0, max: 10 });
});

test("resourceChanged clamps at max", () => {
  const st = fold([
    ...worldWithResource(8, 10),
    env(3, { resourceChanged: { actorId: "a1", resource: "hp", delta: 5, newValue: 10 } }),
  ]);
  expect(st.Actors["a1"]!.resources["hp"]).toEqual({ current: 10, max: 10 });
});

test("a max of zero means UNLIMITED, not a ceiling of zero", () => {
  // The trap: `max > 0` guards the clamp, so max 0 must NOT pin current to 0.
  const st = fold([
    ...worldWithResource(1, 0),
    env(3, { resourceChanged: { actorId: "a1", resource: "hp", delta: 99, newValue: 100 } }),
  ]);
  expect(st.Actors["a1"]!.resources["hp"]!.current).toBe(100);
});

test("resourceChanged rejects an event whose newValue disagrees with the computation", () => {
  // This is the integrity check: the event states its own result, and a fold
  // that trusted it blindly would let a forged or stale event rewrite state.
  expect(() =>
    fold([
      ...worldWithResource(3, 10),
      env(3, { resourceChanged: { actorId: "a1", resource: "hp", delta: 1, newValue: 9 } }),
    ]),
  ).toThrow(FoldError);
});

test("resourceChanged carries Max over from state, never from the event", () => {
  const st = fold([
    ...worldWithResource(5, 10),
    env(3, { resourceChanged: { actorId: "a1", resource: "hp", delta: 1, newValue: 6 } }),
  ]);
  expect(st.Actors["a1"]!.resources["hp"]!.max).toBe(10);
});

test("conditions keep insertion order and are never sorted", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
    env(3, { conditionApplied: { actorId: "a1", conditionId: "zzz", source: "dm" } }),
    env(4, { conditionApplied: { actorId: "a1", conditionId: "aaa", source: "dm" } }),
  ]);
  expect(st.Conditions["a1"]!.map((c) => c.ID)).toEqual(["zzz", "aaa"]);
});

test("removing the last condition RETAINS the actor's key as an empty list", () => {
  // Go keeps the map entry; an absent key and an empty list are different
  // bytes in the dump, so this is parity-visible.
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
    env(3, { conditionApplied: { actorId: "a1", conditionId: "c1", source: "dm" } }),
    env(4, { conditionRemoved: { actorId: "a1", conditionId: "c1" } }),
  ]);
  expect(Object.keys(st.Conditions)).toContain("a1");
  expect(st.Conditions["a1"]).toEqual([]);
});

test("a duplicate condition is rejected", () => {
  expect(() =>
    fold([
      env(1, { sessionStarted: { name: "S" } }),
      env(2, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
      env(3, { conditionApplied: { actorId: "a1", conditionId: "c1", source: "dm" } }),
      env(4, { conditionApplied: { actorId: "a1", conditionId: "c1", source: "dm" } }),
    ]),
  ).toThrow(FoldError);
});

test("note text length is measured in BYTES, not characters", () => {
  // A 3-byte emoji must count as 3 toward the 8192 limit, as Go's len() does.
  const long = "🎲".repeat(2731); // 10924 bytes, but only 2731 code points
  expect(() =>
    fold([
      env(1, { sessionStarted: { name: "S" } }),
      env(2, { noteUpserted: { key: "k", title: "t", text: long } }),
    ]),
  ).toThrow(FoldError);
});

test("narration validates but leaves no trace in state", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { narrationAdded: { text: "A hush falls." } }),
  ]);
  expect(Object.keys(st.Notes)).toHaveLength(0);
  expect(st.Sessions).toHaveLength(1);
});

test("a forward-pointing narration anchor is rejected", () => {
  expect(() =>
    fold([
      env(1, { sessionStarted: { name: "S" } }),
      env(2, { narrationAdded: { text: "t", anchorFromSeq: "5", anchorToSeq: "5" } }),
    ]),
  ).toThrow(FoldError);
});

test("sessionStarted takes its ID from the envelope, not the payload", () => {
  const st = fold([env(1, { sessionStarted: { name: "Named" } })]);
  expect(st.Sessions[0]!.ID).toBe("sess-1");
  expect(st.Sessions[0]!.Name).toBe("Named");
});

test("a second session cannot open while one is still open", () => {
  expect(() =>
    fold([env(1, { sessionStarted: { name: "A" } }), env(2, { sessionStarted: { name: "B" } })]),
  ).toThrow(FoldError);
});

test("tokenMoved ignores the event's from and sceneId entirely", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { sceneCreated: { sceneId: "s1", name: "S1", gridWidth: 9, gridHeight: 9 } }),
    env(3, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
    env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } }),
    // A lying `from` and a nonexistent `sceneId` must not affect the result.
    env(5, { tokenMoved: { tokenId: "t1", sceneId: "does-not-exist", from: { x: 7, y: 7 }, to: { x: 2, y: 3 } } }),
  ]);
  expect(st.Tokens["t1"]).toMatchObject({ SceneID: "s1", X: 2, Y: 3 });
});
