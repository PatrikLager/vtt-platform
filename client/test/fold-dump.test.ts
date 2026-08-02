import { test, expect } from "bun:test";
import { create, fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";

// foldToDumpJSON reproduces Go's `vtt state dump` byte for byte, and its
// hardest part is the omitempty rules: Actor and Resource are protobuf types
// whose zero values VANISH from the JSON, while Scene/Token/Session are plain
// structs where they must appear.
//
// The golden corpus checks whole dumps, so it only exercises the omitempty
// arms its six scenarios happen to contain. Mutation testing found the rest
// unpinned: flipping `if (a.name !== "")` to always-true, dropping the
// trailing newline, and blanking the Conditions mapping all left every test
// green. Each case below drives ONE arm.

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

const started = env(1, { sessionStarted: { name: "S" } });

function dumpOf(log: Envelope[]): { raw: string; parsed: Record<string, any> } {
  const raw = foldToDumpJSON(log);
  return { raw, parsed: JSON.parse(raw) };
}

test("the dump ends with a trailing newline", () => {
  // `+ "\n"` -> `+ ""` survives the parity keystone because that test compares
  // with trimEnd() on both sides. The byte matters: Go's writeDump emits it,
  // and a diff against a real dump would show every file as changed.
  expect(foldToDumpJSON([started]).endsWith("\n")).toBe(true);
});

test("an actor's empty name is OMITTED, and a set one is present", () => {
  const { parsed } = dumpOf([
    started,
    env(2, { actorAdded: { actor: { actorId: "a1" } } }),
    env(3, { actorAdded: { actor: { actorId: "a2", name: "Named" } } }),
  ]);
  expect(parsed.Actors.a1).not.toHaveProperty("name");
  expect(parsed.Actors.a2.name).toBe("Named");
});

test("an actor's module_id is emitted when set and omitted when empty", () => {
  const { parsed } = dumpOf([
    started,
    env(2, { actorAdded: { actor: { actorId: "a1", name: "A", moduleId: "dnd45e" } } }),
    env(3, { actorAdded: { actor: { actorId: "a2", name: "B" } } }),
  ]);
  expect(parsed.Actors.a1.module_id).toBe("dnd45e");
  expect(parsed.Actors.a2).not.toHaveProperty("module_id");
});

test("a resource's zero max is OMITTED while a set max is present", () => {
  // max <= 0 means unlimited, and protobuf omitempty makes {current:0,max:0}
  // marshal as {}. Emitting `max: 0` would diverge from Go's dump.
  const { parsed } = dumpOf([
    started,
    env(2, {
      actorAdded: {
        actor: {
          actorId: "a1", name: "A",
          resources: { unlimited: { current: 3, max: 0 }, capped: { current: 2, max: 9 } },
        },
      },
    }),
  ]);
  expect(parsed.Actors.a1.resources.unlimited).not.toHaveProperty("max");
  expect(parsed.Actors.a1.resources.unlimited.current).toBe(3);
  expect(parsed.Actors.a1.resources.capped.max).toBe(9);
});

test("conditions reach the dump with their own fields intact", () => {
  const { parsed } = dumpOf([
    started,
    env(2, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
    env(3, { conditionApplied: { actorId: "a1", conditionId: "prone", source: "spell" } }),
  ]);
  expect(parsed.Conditions.a1).toEqual([{ ID: "prone", Source: "spell", AppliedSeq: 3 }]);
});

test("headSequence is the MAXIMUM sequence, not the last envelope's", () => {
  // Replay is ordered today. Pinning this stops the rule silently depending
  // on that, and kills `Number(e.sequence) > head` -> `true`, under which head
  // becomes whatever came last.
  const { parsed } = dumpOf([
    env(3, { sessionStarted: { name: "S" } }),
    env(1, { sceneCreated: { sceneId: "s1", name: "N", gridWidth: 2, gridHeight: 2 } }),
    env(2, { sceneCreated: { sceneId: "s2", name: "N", gridWidth: 2, gridHeight: 2 } }),
  ]);
  expect(parsed.headSequence).toBe(3);
});

test("an empty log dumps Sessions as null, not an empty array", () => {
  // A Go nil slice marshals as null. `[]` would be a byte difference against
  // every fresh-campaign dump.
  const { parsed, raw } = dumpOf([]);
  expect(parsed.Sessions).toBeNull();
  expect(raw).toContain('"Sessions": null');
});

test("an envelope with no payload at all is skipped rather than throwing", () => {
  // A malformed frame must not take the whole board down: fold runs on every
  // event, so a throw here blanks the client rather than dropping one row.
  const bare = create(EnvelopeSchema, { eventId: "bare", sequence: 2n });
  expect(() => fold([started, bare])).not.toThrow();
});

test("ending an already-ended session is rejected, not silently re-ended", () => {
  // `find((s) => s.EndSeq === 0)` -> `find(() => true)` returns the CLOSED
  // session and stamps it again. Only a log with a closed session present
  // distinguishes that from the correct behaviour — an empty session list
  // throws either way.
  expect(() => fold([started, env(2, { sessionEnded: {} }), env(3, { sessionEnded: {} })]))
    .toThrow("session ended with none open at sequence 3");
});

test("removing one of several conditions removes the NAMED one", () => {
  // `findIndex((c) => c.ID === v.conditionId)` -> `findIndex(() => true)`
  // always removes the first. With a single condition the two are identical.
  const st = fold([
    started,
    env(2, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
    env(3, { conditionApplied: { actorId: "a1", conditionId: "prone", source: "s" } }),
    env(4, { conditionApplied: { actorId: "a1", conditionId: "dazed", source: "s" } }),
    env(5, { conditionRemoved: { actorId: "a1", conditionId: "dazed" } }),
  ]);
  expect(st.Conditions["a1"]!.map((c) => c.ID)).toEqual(["prone"]);
});

test("every actor and token is stored under a key equal to its own id", () => {
  // LOAD-BEARING INVARIANT, pinned here because something else depends on it
  // from a distance: fifteen adjudications in tools/ts-mutation-equivalents.txt
  // declare player.ts's sort comparators equivalent, and the whole argument is
  // that the ids being sorted are UNIQUE. They are unique only because the map
  // key equals the id field, which is maintained HERE — fold.ts:77 and :87 —
  // not in the file those adjudications describe.
  //
  // Without this test, a fold that stored an actor under some other key would
  // quietly invalidate those fifteen claims with nothing failing.
  const st = fold([
    started,
    env(2, { actorAdded: { actor: { actorId: "a1", name: "A" } } }),
    env(3, { actorAdded: { actor: { actorId: "a2", name: "B" } } }),
    env(4, { sceneCreated: { sceneId: "s1", name: "N", gridWidth: 4, gridHeight: 4 } }),
    env(5, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } }),
    env(6, { tokenPlaced: { tokenId: "t2", sceneId: "s1", actorId: "a2", position: { x: 2, y: 2 } } }),
    env(7, { tokenMoved: { tokenId: "t1", to: { x: 3, y: 3 } } }),
  ]);

  for (const [key, actor] of Object.entries(st.Actors)) expect(actor.actorId).toBe(key);
  for (const [key, token] of Object.entries(st.Tokens)) expect(token.ID).toBe(key);

  // And therefore the sorted ids are distinct, which is the property the
  // adjudications actually rely on.
  const actorIds = Object.values(st.Actors).map((a) => a.actorId);
  const tokenIds = Object.values(st.Tokens).map((t) => t.ID);
  expect(new Set(actorIds).size).toBe(actorIds.length);
  expect(new Set(tokenIds).size).toBe(tokenIds.length);
});
