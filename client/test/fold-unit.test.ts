import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError } from "../src/state";

// The golden corpus cannot reach every fold variant, and the gap is a MEASURED
// list rather than an argument. Across all eight `scenarios/goldens/*/
// stream.json` AND both projected streams under `projections/*/`, three of
// fold.ts's twenty-one arms are never folded: **attackRolled, doorOpened and
// doorClosed**. Said at the level of the EVENT VARIANT, which is the level that
// matters for parity: `attackRolled` shares its arm BODY with `abilityUsed` and
// `adventureLoaded`, both of which the corpus does fold, so that code path runs —
// it is the variant that no corpus stream carries, and
// client/test/fold-rejections.test.ts's "an event kind the fold does not know is
// skipped, not fatal" is what folds one directly. Named rather than cited by line
// for the reason this file states below: an offset into another file rots the
// moment anything is inserted above it, and nothing pins it.
//
// The door pair has its own note at the terrain section, which owns why it
// matters. Not restated here — and the reason is worth one sentence, because this
// one pointed AND copied. The owner scopes its claim to "no CORPUS STREAM
// exercising it"; the copy here had already lost that scope and read "nothing
// exercising it", which is plainly false, since tests in this very file fold both
// door arms. A second copy does not stay a copy.
//
// (That read "four folds" for one draft, and the replacement count was wrong too:
// it tallied only the folds that SUCCEED, silently dropping the door envelopes
// that "a door event naming an unknown scene or no position is refused" folds —
// which enter both arms and throw from inside them, and which are themselves
// instances of the very claim being annotated. No number survives here,
// deliberately: the point needs none, and every number written in this spot has
// turned out to be one more thing that was not true. The first draft of THIS
// sentence still had one in it.)
//
// DICE ARE NOT THE CAUSE, and two earlier versions of this header said they
// were. It read "three of the seven scenarios roll dice and are excluded until a
// roller seam exists, which leaves resourceChanged, conditionApplied/Removed and
// the ability/adventure no-ops unexercised". Every clause was wrong: the corpus
// is EIGHT; TWO scenarios roll dice (adventure-night, toy-brawl) and both ARE in
// it; and those two do emit resourceChanged, conditionApplied, conditionRemoved,
// abilityUsed and adventureLoaded, so none of those arms is unexercised. Only
// goblin-fight is absent, and scenarios/goldens/README.md says of it that "it
// costs nothing in coverage: every command type it uses is already covered by
// toy-brawl" — so its absence explains none of the gap above either.
//
// tokenHidden/sceneSeen used to be listed here as a fourth, PERMANENT gap. That
// is no longer true. Visibility spec §5 makes both projection-only — no command
// produces them, so they cannot structurally reach a real LOG — but the corpus
// gained PROJECTED halves with the keystone, and those are not logs:
// scenarios/goldens/session-zero/projections/player/stream.json carries sceneSeen
// twice and tokenHidden once, and client/test/projection-parity.test.ts folds
// both arms against a hand-derived state.
//
// So what the 34 hand-written cases below are for is the ARMS' EDGES. THREE of
// the four in this file's tokenHidden/sceneSeen section are reached by no stream
// at all, recorded or derived: a re-sent hide (the corpus holds exactly one
// tokenHidden), Explored unioning across messages (no single stream folds two
// sceneSeen for one scene, and only one of the corpus's three carries `tiles` at
// all — Explored unions from tile keys, never from `visible`), and sceneSeen's
// objects replacing a repeated id (no sceneSeen anywhere carries objects).
//
// The remaining one — "tokenHidden forgets only that token", a hide that leaves
// its neighbours standing — is no longer one of them: session-zero's projected
// player stream places tok-goblin-archer at sequence 11 and hides it at 12 while
// tok-fighter and tok-healer stay on the board, and projection-parity.test.ts
// byte-compares the result against a hand-derived state holding exactly those
// two. Kept, since a fixture and a unit test fail differently and the unit test
// names the property; but it is coverage now, not absence.
//
// SCOPE IS WHERE THE sceneSeen-UNIONING PARENTHETICAL KEEPS GOING WRONG, so it
// is written per-stream now. (Named rather than called "the middle" one, which
// is an index into a list that has already changed size once and would have no
// middle at all if the corpus ever gained an objects-carrying sceneSeen.)
//
// It read "its two sceneSeen events are one per scene" until somebody counted it
// at the scope it is written at: its neighbours are corpus-wide, so it reads
// corpus-wide, and corpus-wide there are THREE —
// `camp` gets one in each projected stream. "Two" was the PLAYER stream's count,
// borrowed from the paragraph above into a sentence whose scope had widened.
// Unioning is a within-stream operation, so per-stream is both the scope that
// makes the claim true and the stronger reason.
//
// NO LINE-DISTANCE REFERENCES ANYWHERE IN THIS FILE, and that is a rule rather
// than a preference. Two were written during these corrections and both were
// wrong on arrival — "eleven lines up" was off by four, "twenty-two lines above"
// by three — because every re-wrap moves everything beneath it. Name the thing,
// not its offset.
//
// Two neighbouring edges live in neighbouring files, because that is where their
// subject lives — an EMPTY visible set in client/test/visibility.test.ts ("an
// empty sceneSeen darkens the scene and forgets no terrain") and a scene that
// never arrived in client/test/fold-rejections.test.ts ("sceneSeen naming an
// unknown scene is rejected"). Named with their paths rather than gestured at,
// because the sentence they replace claimed all three were in this file and
// neither of those two was.
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
// DOORS AND TERRAIN ARE NO LONGER THE SAME CASE, and this comment treated them
// as one until 2026-08-22. It said the corpus misses both "for the same reason:
// no scenario golden contains a door event at all, and the SceneCreated arm's
// terrain loops only run when a scene actually declares tiles" — true of doors,
// and already misleading about terrain on the day it was written. The dates, since
// "already" is the load-bearing word: adventure-night entered the corpus on
// 2026-07-31 with ZERO tiles, gained its 1024 on 2026-08-13 when the map compiler
// landed, and this comment was written on 2026-08-16 — three days after the thing
// it says is not covered became covered.
//
// Counted rather than reasoned, over all eight logs and both projected streams:
//
//   doors    — ZERO doorOpened/doorClosed events, and ZERO tiles of kind "door".
//              Nothing folds either arm, and nothing puts a closed door in front
//              of the SceneCreated arm's OpenDoors initialisation.
//   terrain  — REACHED. adventure-night declares 1024 tiles and session-zero
//              declares 180, so the tile loops run under fold-parity.
//
// So the door half of the original point stands in full: fold.ts's door handling
// was added in Task 5 PRECISELY to keep the cross-language keystone honest and
// still ships with no corpus stream exercising it — if the TS fold and
// internal/engine/apply.go disagreed about doors, the keystone would not notice.
// The terrain cases below therefore stand on DIFFERENT ground than they used to,
// and the parenthetical that first said so got two of its three examples wrong.
// Counted:
//
//   an object's every field surviving  — UNREACHED, and comfortably so. Zero
//       scene objects appear in any of the ten corpus streams and zero in any
//       state.json, so nothing pins the object round-trip but the case below.
//   the dump shape                     — REACHED FOR TERRAIN, unreached for the
//       rest. adventure-night's state.json byte-compares 1024 Tiles entries and
//       session-zero's 180; OpenDoors is empty in every one, and Objects has
//       nothing to carry (above). That case makes three assertions — Tiles,
//       Objects[0].RotationDegrees and OpenDoors — and the corpus reaches only
//       the first, so it is the DOOR and OBJECT assertions it misses, not the
//       case. (This said "the DOOR half", which undercounted its own detail one
//       bullet after stating it.)
//   Explored's omission                — REACHED HARDEST OF ANYTHING HERE, and
//       listing it as unreached was flatly wrong. Emitting `Explored: {}`
//       unconditionally reds TEN corpus cases: all 8 fold-parity goldens and both
//       projection-parity seats. This file says exactly that in the test named
//       "Explored reaches the dump when populated, and is OMITTED (not {}) when
//       empty" — the very case the parenthetical was describing — and
//       client/src/fold.ts says it a third time. Only the summary was wrong.
//
// The lesson is the one this file keeps re-learning: a summary written from
// memory of the cases below it drifts from them, and the cases were right all
// three times.

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

test("Explored reaches the dump when populated, and is OMITTED (not {}) when empty", () => {
  // Fix-round-1 finding (C1): this is the same "reaches the dump" property
  // as the test above, but Explored needed a SECOND, opposite-direction
  // assertion the terrain/door test above didn't need. Go's Scene.Explored
  // carries `json:",omitempty"` (state.go), so json.Marshal DROPS the key
  // entirely whenever the map is nil or empty — not just when it is nil.
  // foldToDumpJSON has no struct tags to lean on, so it must reproduce that
  // omission by hand; naively always emitting `Explored: {}` looks harmless
  // in isolation but fails every scenarios/goldens/*/state.json byte
  // comparison (client/test/fold-parity.test.ts), because none of those is
  // derived from a stream containing SceneSeen — they are the LOGS, and no
  // log produces one. Verified directly: adding
  // `Explored: sortedMap(s.Explored ?? {}, v => v)` unconditionally and
  // running `bun test client/test/fold-parity.test.ts` fails all 8
  // scenarios with an unexpected `"Explored": {}` in every Scene; reverting
  // to the conditional-omission form in fold.ts passes all 8 again.
  // (Re-measured 2026-08-22, when session-zero took the corpus from 7 to 8.
  // The same injection also fails 2 of the PROJECTED cases in
  // client/test/projection-parity.test.ts — the bare-canvas scene, which has
  // a visible set and no terrain to remember.)
  const unseen = JSON.parse(foldToDumpJSON(sceneWithADoor()));
  expect("Explored" in unseen.Scenes["s1"]).toBe(false);

  const seen = JSON.parse(
    foldToDumpJSON([...sceneWithADoor(), env(3, { sceneSeen: { sceneId: "s1", tiles: { "0,0": { kind: "floor" } } } })]),
  );
  expect(seen.Scenes["s1"].Explored).toEqual({ "0,0": true });
});

// --- tokenHidden / sceneSeen (visibility spec §6) ---------------------------
//
// Both arms are PROJECTION-ONLY (spec §5). HOW they nonetheless reach the corpus,
// and WHICH of the four cases below it reaches, are both the HEADER's subject and
// neither is restated here.
//
// One fact, one place — the rule internal/gateway/keystone_test.go's own
// de-duplication follows. This banner carried a second copy of the coverage split
// until the two were compared, and then a second copy of the recorded-versus-
// derived argument after that; both are the header's. Pointing at the owner of a
// fact is not the same as deferring a fact: the pointer cannot go stale into a
// false statement, whereas a second copy of the fact silently can, which is this
// file's documented recurring defect.
//
// What this banner owns instead is PARITY. All FOUR have a named counterpart in
// internal/engine/visibility_fold_test.go —
// TestTokenHiddenForgetsOnlyThatToken, TestHidingATokenTwiceIsNotAnError,
// TestSceneSeenUnionsIntoExploredAndNeverShrinks and
// TestSceneSeenObjectsMergeReplacingDuplicatesAndAppendingNew — because the two
// folds are the load-bearing mirror the keystone (spec §4.3) depends on.
//
// THREE OF THE FOUR MIRROR THEIRS EXACTLY, same scenario shape and same
// assertions. The re-sent hide does NOT, deliberately: its Go counterpart hides
// a token that never existed on a state with no scene at all and asserts only
// that Apply returned nil, while the TS one builds a two-token scene and asserts
// the neighbour survives. That test's own comment says why the weaker form would
// not do here — fold.ts's default case SKIPS an unrecognised variant, so
// `not.toThrow()` alone cannot tell a wired arm from an unwired one. Said here
// because "same assertions" was claimed for all four until 2026-08-22, in this
// very banner, while the comment explaining why one of them is different sat
// inside "hiding a token twice is not an error" the whole time. ("three tests"
// until the same date, when the count was finally taken.)
//
// Named, not placed: that read "the test just below it", which pointed at
// "tokenHidden forgets only that token" — the wrong one, and wrong the moment it
// was written. The explaining comment has always sat inside "hiding a token twice
// is not an error" and never inside "tokenHidden forgets only that token".
//
// The first draft of THIS correction said "the SECOND test of this section rather
// than the first", which is two ordinals in the paragraph that forbids them, and
// carrying no information the two names above do not already carry. An ordinal is
// an offset into a list, and this section's membership is something the header
// actively counts and revises.

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
