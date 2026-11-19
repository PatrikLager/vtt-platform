import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { FoldError } from "../src/state";

let seq = 0;
const env = (payload: Record<string, unknown>): Envelope => {
  seq += 1;
  return fromJson(EnvelopeSchema, { eventId: `qd-${seq}`, sequence: String(seq), ...payload });
};

const world = (): Envelope[] => [
  env({
    sceneCreated: {
      sceneId: "hall",
      name: "Hall",
      gridWidth: 7,
      gridHeight: 5,
      tiles: { "2,2": { kind: "floor" }, "3,2": { kind: "door" }, "4,2": { kind: "floor" } },
    },
  }),
  env({ actorAdded: { actor: { actorId: "hero", name: "Hero", kind: "ACTOR_KIND_PARTY_MEMBER" } } }),
  env({ actorAdded: { actor: { actorId: "ogre", name: "Ogre", kind: "ACTOR_KIND_NON_PARTY" } } }),
];

const door = (kind: "doorOpened" | "doorClosed", over: Record<string, unknown>): Envelope =>
  env({ [kind]: { sceneId: "hall", at: { x: 3, y: 2 }, ...over } });

const KINDS = ["doorOpened", "doorClosed"] as const;
const ABSENT = ["ghost", "hasOwnProperty", "constructor", "__proto__", "toString", "valueOf"];

// VTT-866
test("a door event naming an actor the state does not hold is refused", () => {
  for (const kind of KINDS) {
    for (const actorId of ABSENT) {
      const log = [...world()];
      if (kind === "doorClosed") log.push(door("doorOpened", {}));
      log.push(door(kind, { actorId }));
      expect(() => fold(log), `${kind} ${actorId}`).toThrow(FoldError);
    }
  }
});

// VTT-866
test("an unknown scene or a missing position is refused whatever the actor", () => {
  for (const kind of KINDS) {
    for (const actorId of ["", "ghost", "hasOwnProperty", "hero"]) {
      const bad: Record<string, unknown>[] = [{ sceneId: "nowhere" }, { noAt: true }];
      const body = (over: Record<string, unknown>, withActor: boolean) => {
        const { noAt, ...rest } = over;
        const b: Record<string, unknown> = { sceneId: "hall", ...(noAt ? {} : { at: { x: 3, y: 2 } }), ...rest };
        if (withActor) b.actorId = actorId;
        return b;
      };
      for (const over of bad) {
        const e = env({ [kind]: body(over, true) });
        let msgFor: string | undefined;
        try {
          fold([...world(), e]);
        } catch (err) {
          expect(err).toBeInstanceOf(FoldError);
          msgFor = (err as Error).message;
        }
        expect(msgFor, `${kind} ${actorId} ${JSON.stringify(over)}`).toBeDefined();
        // the same words with no actor: the scene or the position is the reason
        const plain = env({ [kind]: body(over, false) });
        let msgPlain: string | undefined;
        try {
          fold([...world(), plain]);
        } catch (err) {
          msgPlain = (err as Error).message;
        }
        expect(msgFor).toBe(msgPlain!);
      }
    }
  }
});

// VTT-866
test("a held actor or no actor folds as a door event always did", () => {
  for (const actorId of ["", "hero", "ogre"]) {
    const opened = fold([...world(), door("doorOpened", { actorId })]);
    expect(opened.Scenes["hall"]?.OpenDoors?.["3,2"]).toBe(true);
    const closed = fold([...world(), door("doorOpened", { actorId }), door("doorClosed", { actorId })]);
    expect(closed.Scenes["hall"]?.OpenDoors?.["3,2"]).toBeUndefined();
  }
});
