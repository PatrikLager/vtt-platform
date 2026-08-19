import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

// The golden corpus cannot reach every fold variant: three of the seven
// scenarios roll dice and are excluded until a roller seam exists (see
// scenarios/goldens/README.md), which leaves resourceChanged,
// conditionApplied/Removed and the ability/adventure no-ops unexercised.
// tokenHidden/sceneSeen are unexercised for a different, permanent reason:
// visibility spec §5 says both are PROJECTION-ONLY — no command produces
// them, so they cannot structurally reach the real log the corpus is
// recorded from, ever.
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

// --- actor control is a SET, and controllerId mirrors it ---------------------
//
// This mirrors internal/engine/actor_control_test.go. Both folds
// are compared against scenarios/goldens, so a divergence here is not a display
// bug — it is the two implementations disagreeing about who controls a
// character. The mirror rule is controllerIds[0], never "blank when shared":
// an empty controllerId already means DM/agent-only, so blanking it for a
// shared actor would make it indistinguishable from an unowned one and drop it
// from its own player's list.

test("adding an actor with a controller seeds the set", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-1"]);
  expect(st.Actors["a1"]!.controllerId).toBe("p-1");
});

test("adding an actor with no controller leaves the set empty", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1" } } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual([]);
  expect(st.Actors["a1"]!.controllerId).toBe("");
});

test("granting a second controller keeps the first in controllerId", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
    env(3, { actorControlGranted: { actorId: "a1", participantId: "p-2" } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-1", "p-2"]);
  // The case the rejected rule got wrong: p-1 must still see this as theirs.
  expect(st.Actors["a1"]!.controllerId).toBe("p-1");
});

test("granting to an actor with no controller fills controllerId", () => {
  // The ONLY grant shape where the mirror is observable: every other grant
  // test starts from a non-empty set, where controllerId does not change, so
  // dropping the mirror call from the grant arm goes unnoticed. It did — a
  // fault injection removing it left the whole client suite green while Go
  // failed two tests, which is a Go/TS divergence about who controls a
  // character, shipping green through both fold gates.
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1" } } }),
    env(3, { actorControlGranted: { actorId: "a1", participantId: "p-1" } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-1"]);
  expect(st.Actors["a1"]!.controllerId).toBe("p-1");
});

test("granting is idempotent", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
    env(3, { actorControlGranted: { actorId: "a1", participantId: "p-2" } }),
    env(4, { actorControlGranted: { actorId: "a1", participantId: "p-2" } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-1", "p-2"]);
});

test("revoking the first controller promotes the next", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
    env(3, { actorControlGranted: { actorId: "a1", participantId: "p-2" } }),
    env(4, { actorControlRevoked: { actorId: "a1", participantId: "p-1" } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-2"]);
  expect(st.Actors["a1"]!.controllerId).toBe("p-2");
});

test("revoking the last controller empties both", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
    env(3, { actorControlRevoked: { actorId: "a1", participantId: "p-1" } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual([]);
  expect(st.Actors["a1"]!.controllerId).toBe("");
});

test("revoking someone who has no control is a no-op", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
    env(3, { actorControlRevoked: { actorId: "a1", participantId: "p-stranger" } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-1"]);
});

test("the dump carries controller_ids alongside controller_id", () => {
  // The dump shape is what scenarios/goldens pins and what BOTH folds are
  // compared against, so this is the parity surface rather than an internal.
  const dumped = JSON.parse(
    foldToDumpJSON([
      env(1, { sessionStarted: { name: "S" } }),
      env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-1" } } }),
      env(3, { actorControlGranted: { actorId: "a1", participantId: "p-2" } }),
    ]),
  ) as { Actors: Record<string, Record<string, unknown>> };
  expect(dumped.Actors["a1"]!["controller_id"]).toBe("p-1");
  expect(dumped.Actors["a1"]!["controller_ids"]).toEqual(["p-1", "p-2"]);
});

test("adding an actor drops empty ids from the control set", () => {
  // The TS twin of TestAddActorDropsEmptyIdsFromTheControlSet. The
  // grant/revoke guard does not cover actorAdded, and an empty id in the set
  // would give a non-empty set an empty mirror — unowned to every reader, and
  // unremovable, since revoke rejects an empty participant.
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerIds: ["", "p-1", ""] } } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-1"]);
  expect(st.Actors["a1"]!.controllerId).toBe("p-1");
});

// The TS twin of TestAddActorLetsTheSetOverrideTheDeclaredController. Without
// it, flipping copyActor's ternary to prefer the scalar leaves every TS test
// green while Go fails — a live divergence about who controls a character.
test("adding an actor lets the set override the declared controller", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "S" } }),
    env(2, { actorAdded: { actor: { actorId: "a1", controllerId: "p-a", controllerIds: ["p-b"] } } }),
    env(3, { actorAdded: { actor: { actorId: "a2", controllerId: "p-a", controllerIds: [""] } } }),
  ]);
  expect(st.Actors["a1"]!.controllerIds).toEqual(["p-b"]);
  expect(st.Actors["a1"]!.controllerId).toBe("p-b");
  // A set of only empty ids erases control rather than falling back to the
  // scalar: fails closed, and a later grant restores it.
  expect(st.Actors["a2"]!.controllerIds).toEqual([]);
  expect(st.Actors["a2"]!.controllerId).toBe("");
});

// --- terrain and doors (maps-as-geometry) --------------------------------
//
// Same argument as this file's header, and the corpus misses these for the
// same reason: no scenario golden contains a door event at all, and the
// SceneCreated arm's terrain loops only run when a scene actually declares
// tiles. So the whole of fold.ts's door handling — added in Task 5 PRECISELY
// to keep the cross-language fold-parity keystone honest — shipped with
// nothing exercising it. If the TS fold and internal/engine/apply.go
// disagreed about doors, the keystone would not have noticed, which is the
// one failure it exists to prevent.

/** A 2x2 scene: floor everywhere but a wood door at (0,1). */
function sceneWithADoor(): Envelope[] {
  const tiles: Record<string, unknown> = {};
  for (let y = 0; y < 2; y++) {
    for (let x = 0; x < 2; x++) {
      tiles[`${x},${y}`] =
        x === 0 && y === 1
          ? { kind: "door", material: "wood", art: "" }
          : { kind: "floor", material: "stone", art: "" };
    }
  }
  return [
    env(1, { sessionStarted: { name: "S" } }),
    env(2, {
      sceneCreated: {
        sceneId: "s1", name: "Cell", gridWidth: 2, gridHeight: 2, tiles,
        objects: [
          {
            objectId: "pillar-1", kind: "pillar", at: { x: 1, y: 0 },
            width: 1, height: 1, rotationDegrees: 90,
            blocksSight: true, blocksMove: true, art: "pillar-stone",
          },
        ],
      },
    }),
  ];
}

test("a scene's tiles and objects survive the fold with every field intact", () => {
  const st = fold(sceneWithADoor());
  const sc = st.Scenes["s1"]!;
  expect(Object.keys(sc.Tiles ?? {})).toHaveLength(4);
  expect(sc.Tiles!["0,1"]).toEqual({ Kind: "door", Material: "wood", Art: "" });
  expect(sc.Tiles!["1,1"]).toEqual({ Kind: "floor", Material: "stone", Art: "" });

  // Every object field, not just the ones a renderer happens to read today:
  // rotation and the blocks_* flags are what the visibility arc will consume,
  // and a field silently dropped here would be found by that arc, not by this
  // one.
  expect(sc.Objects).toHaveLength(1);
  expect(sc.Objects![0]).toEqual({
    ObjectID: "pillar-1", Kind: "pillar", X: 1, Y: 0,
    Width: 1, Height: 1, RotationDegrees: 90,
    BlocksSight: true, BlocksMove: true, Art: "pillar-stone",
  });
});

test("a door is CLOSED until opened, and closing restores the never-touched state", () => {
  // Mirrors internal/engine/apply.go's arms exactly, which is the point: Go
  // initialises OpenDoors empty and DELETES on close rather than storing
  // false, so "just shut" and "never touched" are indistinguishable. If TS
  // stored false instead, the two folds would diverge on a scene nobody had
  // touched, and every golden would still pass.
  const base = sceneWithADoor();
  const st0 = fold(base);
  expect(st0.Scenes["s1"]!.OpenDoors ?? {}).toEqual({});

  const opened = fold([...base, env(3, { doorOpened: { sceneId: "s1", at: { x: 0, y: 1 } } })]);
  expect(opened.Scenes["s1"]!.OpenDoors!["0,1"]).toBe(true);

  const closed = fold([
    ...base,
    env(3, { doorOpened: { sceneId: "s1", at: { x: 0, y: 1 } } }),
    env(4, { doorClosed: { sceneId: "s1", at: { x: 0, y: 1 } } }),
  ]);
  expect(closed.Scenes["s1"]!.OpenDoors).toEqual({});
  expect("0,1" in closed.Scenes["s1"]!.OpenDoors!).toBe(false);
});

test("a door event naming an unknown scene or no position is refused", () => {
  const base = sceneWithADoor();
  for (const bad of [
    { doorOpened: { sceneId: "nope", at: { x: 0, y: 1 } } },
    { doorClosed: { sceneId: "nope", at: { x: 0, y: 1 } } },
    { doorOpened: { sceneId: "s1" } },
    { doorClosed: { sceneId: "s1" } },
  ]) {
    expect(() => fold([...base, env(3, bad)])).toThrow(FoldError);
  }
});

test("terrain and door state reach the dump the parity keystone compares", () => {
  // foldToDumpJSON is the half the Go/TS keystone actually diffs. Terrain that
  // folds correctly but never reaches the dump would make the keystone blind
  // to exactly the fields this arc added.
  const dump = JSON.parse(
    foldToDumpJSON([...sceneWithADoor(), env(3, { doorOpened: { sceneId: "s1", at: { x: 0, y: 1 } } })]),
  );
  const sc = dump.Scenes["s1"];
  expect(sc.Tiles["0,1"]).toEqual({ Kind: "door", Material: "wood", Art: "" });
  expect(sc.Objects[0].RotationDegrees).toBe(90);
  expect(sc.OpenDoors["0,1"]).toBe(true);
});

// --- tokenHidden / sceneSeen (visibility spec §6) ---------------------------
//
// Both arms are PROJECTION-ONLY: no command produces them (spec §5), so they
// never reach scenarios/goldens' recorded streams. Mirrors
// internal/engine/visibility_fold_test.go's three tests exactly — same
// scenario shapes, same assertions — because the two folds are the load-
// bearing mirror the keystone (spec §4.3) depends on.

/** A 3x3 scene with two actors, each carrying a token. */
function twoTokenScene(): Envelope[] {
  return [
    env(1, { sessionStarted: { name: "n" } }),
    env(2, { sceneCreated: { sceneId: "s", name: "S", gridWidth: 3, gridHeight: 3 } }),
    env(3, { actorAdded: { actor: { actorId: "a1", name: "a1" } } }),
    env(3, { actorAdded: { actor: { actorId: "a2", name: "a2" } } }),
    env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s", actorId: "a1", position: { x: 0, y: 0 } } }),
    env(5, { tokenPlaced: { tokenId: "t2", sceneId: "s", actorId: "a2", position: { x: 1, y: 1 } } }),
  ];
}

test("tokenHidden forgets only that token", () => {
  const st = fold([...twoTokenScene(), env(6, { tokenHidden: { tokenId: "t1" } })]);
  expect(st.Tokens["t1"]).toBeUndefined();
  expect(st.Tokens["t2"]).toBeDefined();
});

test("hiding a token twice is not an error", () => {
  // The projection is idempotent by design; a repeated hide must not throw.
  //
  // `not.toThrow()` alone would pass even with no tokenHidden arm at all —
  // fold.ts's default case SKIPS an unrecognised variant rather than
  // throwing (forward compatibility), so an unwired case is silently
  // tolerant too and this assertion could not tell the two apart. Asserting
  // t1 is actually GONE after the first hide forces the arm to be real:
  // that assertion fails if tokenHidden falls through to the skip-default
  // instead of deleting.
  const once = fold([...twoTokenScene(), env(6, { tokenHidden: { tokenId: "t1" } })]);
  expect(once.Tokens["t1"]).toBeUndefined();
  expect(once.Tokens["t2"]).toBeDefined();

  // Hiding it again must not throw (this call itself is the assertion — an
  // uncaught throw fails the test) and must leave the same state behind.
  const twice = fold([
    ...twoTokenScene(),
    env(6, { tokenHidden: { tokenId: "t1" } }),
    env(7, { tokenHidden: { tokenId: "t1" } }),
  ]);
  expect(twice.Tokens["t1"]).toBeUndefined();
  expect(twice.Tokens["t2"]).toBeDefined();
});

test("sceneSeen unions into Explored and never shrinks", () => {
  const st = fold([
    env(1, { sessionStarted: { name: "n" } }),
    env(2, { sceneCreated: { sceneId: "s", name: "S", gridWidth: 3, gridHeight: 3 } }),
    env(3, { sceneSeen: { sceneId: "s", tiles: { "0,0": { kind: "floor" } } } }),
    env(4, { sceneSeen: { sceneId: "s", tiles: { "1,1": { kind: "wall" } } } }),
  ]);
  const sc = st.Scenes["s"]!;
  expect(sc.Explored!["0,0"]).toBe(true); // seen first, still explored
  expect(sc.Explored!["1,1"]).toBe(true); // seen second
  expect(sc.Tiles!["1,1"]!.Kind).toBe("wall"); // a seen tile lands in Tiles too
});

test("sceneSeen's objects REPLACE a repeated id in place and APPEND a new one", () => {
  // Mirrors internal/engine/visibility_fold_test.go's
  // TestSceneSeenObjectsMergeReplacingDuplicatesAndAppendingNew — same
  // scenario, same assertions. SceneSeen carries the whole currently-visible
  // set each time (spec §5), so the same object arrives on every frame it
  // stays visible and must not accumulate duplicates.
  const st = fold([
    env(1, { sessionStarted: { name: "n" } }),
    env(2, { sceneCreated: { sceneId: "s", name: "S", gridWidth: 3, gridHeight: 3 } }),
    env(3, {
      sceneSeen: {
        sceneId: "s",
        objects: [{ objectId: "crate-1", kind: "crate", at: { x: 0, y: 0 }, blocksSight: false }],
      },
    }),
    env(4, {
      sceneSeen: {
        sceneId: "s",
        objects: [
          // crate-1 moved AND its sight-blocking changed: same id, must replace.
          { objectId: "crate-1", kind: "crate", at: { x: 1, y: 0 }, blocksSight: true },
          // pillar-1 is new: must append, not disturb crate-1's slot.
          { objectId: "pillar-1", kind: "pillar", at: { x: 2, y: 2 } },
        ],
      },
    }),
  ]);

  const objs = st.Scenes["s"]!.Objects!;
  expect(objs).toHaveLength(2);
  const crate = objs.find((o) => o.ObjectID === "crate-1");
  const pillar = objs.find((o) => o.ObjectID === "pillar-1");
  expect(crate).toMatchObject({ X: 1, BlocksSight: true });
  expect(pillar).toBeDefined();
});
