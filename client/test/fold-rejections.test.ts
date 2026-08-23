import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { FoldError } from "../src/state";

// fold.ts's opening line is "parity includes REJECTING what Go rejects", and
// mutation testing found that claim almost entirely unpinned: of 93 surviving
// mutants, 37 emptied an error MESSAGE and 35 deleted a GUARD outright, with
// every test still green. A fold that silently accepted a malformed event
// would show a player a board the log does not support — the exact failure the
// module says it exists to prevent.
//
// So each case below drives one guard and asserts the EXACT message. The
// message is not decoration: it is the only thing that says which parity rule
// fired, and "some FoldError was thrown" would be satisfied by any of them.

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

/** Asserts the fold rejects, and rejects for the stated reason. */
function rejects(log: Envelope[], message: string) {
  expect(() => fold(log)).toThrow(new FoldError(message));
}

const started = env(1, { sessionStarted: { name: "S" } });
const actor = (seq: number, id: string) =>
  env(seq, { actorAdded: { actor: { actorId: id, name: id.toUpperCase() } } });
const scene = (seq: number, id: string) =>
  env(seq, { sceneCreated: { sceneId: id, name: "N", gridWidth: 4, gridHeight: 4 } });

// --- sessions ---------------------------------------------------------------

test("a second session cannot open while one is still open", () => {
  rejects([started, env(2, { sessionStarted: { name: "T" } })],
    "session already open at sequence 2");
});

test("a session cannot end when none is open", () => {
  rejects([env(1, { sessionEnded: {} })], "session ended with none open at sequence 1");
});

test("a session CAN open once the previous one has ended", () => {
  // The negative space of the guard above: it must reject a second OPEN
  // session, not every second session. Deleting the guard passes the first
  // test; inverting it fails only this one.
  const st = fold([started, env(2, { sessionEnded: {} }), env(3, { sessionStarted: { name: "T" } })]);
  expect(st.Sessions).toHaveLength(2);
  expect(st.Sessions[1]!.EndSeq).toBe(0);
});

// --- scenes and actors ------------------------------------------------------

test("a duplicate scene id is rejected", () => {
  rejects([started, scene(2, "s1"), scene(3, "s1")], 'duplicate scene "s1"');
});

test("an actorAdded with no actor at all is rejected", () => {
  rejects([started, env(2, { actorAdded: {} })], "actor added with no actor or empty id");
});

test("an actorAdded with an empty actor id is rejected", () => {
  // Distinct from the case above: `!a` and `a.actorId === ""` are two arms of
  // one condition, and either can be deleted independently.
  rejects([started, env(2, { actorAdded: { actor: { name: "A" } } })],
    "actor added with no actor or empty id");
});

test("a duplicate actor id is rejected", () => {
  rejects([started, actor(2, "a1"), actor(3, "a1")], 'duplicate actor "a1"');
});

// --- tokens -----------------------------------------------------------------

const placeable = [started, scene(2, "s1"), actor(3, "a1")];

test("a token placed on an unknown scene is rejected", () => {
  rejects([...placeable, env(4, { tokenPlaced: { tokenId: "t1", sceneId: "nope", actorId: "a1", position: { x: 1, y: 1 } } })],
    'token placed on unknown scene "nope"');
});

test("a token placed for an unknown actor is rejected", () => {
  rejects([...placeable, env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "nope", position: { x: 1, y: 1 } } })],
    'token placed for unknown actor "nope"');
});

test("a token placed with no position is rejected", () => {
  rejects([...placeable, env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1" } })],
    'token "t1" placed with no position');
});

test("a duplicate token id is rejected, and duplicate is checked FIRST", () => {
  // Error ORDER is part of the parity contract (fold.ts:82 — Go checks
  // duplicate, scene, actor, position). This token is a duplicate AND names an
  // unknown scene; the duplicate message must win, or the two folds disagree
  // about which error a bad log produces.
  const place = env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } });
  rejects([...placeable, place,
    env(5, { tokenPlaced: { tokenId: "t1", sceneId: "unknown-too", actorId: "a1", position: { x: 2, y: 2 } } })],
    'duplicate token "t1"');
});

test("moving an unknown token is rejected", () => {
  rejects([...placeable, env(4, { tokenMoved: { tokenId: "ghost", to: { x: 1, y: 1 } } })],
    'unknown token "ghost" moved');
});

test("moving a token with no destination is rejected", () => {
  const place = env(4, { tokenPlaced: { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } });
  rejects([...placeable, place, env(5, { tokenMoved: { tokenId: "t1" } })],
    'token "t1" moved with no destination');
});

// --- sceneSeen (visibility spec §6) ------------------------------------------
//
// tokenHidden has no rejection case: hiding an absent token is deliberately a
// no-op (see fold-unit.test.ts), not an error — that tolerance IS the parity
// contract for this arm, so it belongs with the accept-path tests, not here.

test("sceneSeen naming an unknown scene is rejected", () => {
  rejects([started, env(2, { sceneSeen: { sceneId: "nope", tiles: { "0,0": { kind: "floor" } } } })],
    'scene seen for unknown scene "nope"');
});

// --- conditions -------------------------------------------------------------

test("a condition applied to an unknown actor is rejected", () => {
  rejects([started, env(2, { conditionApplied: { actorId: "ghost", conditionId: "prone", source: "s" } })],
    'condition applied to unknown actor "ghost"');
});

test("a duplicate condition on one actor is rejected", () => {
  const apply = (seq: number) =>
    env(seq, { conditionApplied: { actorId: "a1", conditionId: "prone", source: "s" } });
  rejects([started, actor(2, "a1"), apply(3), apply(4)],
    'duplicate condition "prone" on actor "a1"');
});

test("the same condition on a DIFFERENT actor is fine", () => {
  // The duplicate guard is per-actor. Widening it to global would pass the
  // test above while breaking this one.
  const st = fold([started, actor(2, "a1"), actor(3, "a2"),
    env(4, { conditionApplied: { actorId: "a1", conditionId: "prone", source: "s" } }),
    env(5, { conditionApplied: { actorId: "a2", conditionId: "prone", source: "s" } })]);
  expect(st.Conditions["a1"]).toHaveLength(1);
  expect(st.Conditions["a2"]).toHaveLength(1);
});

test("a condition removed from an unknown actor is rejected", () => {
  rejects([started, env(2, { conditionRemoved: { actorId: "ghost", conditionId: "prone" } })],
    'condition removed from unknown actor "ghost"');
});

test("removing a condition the actor does not have is rejected", () => {
  rejects([started, actor(2, "a1"), env(3, { conditionRemoved: { actorId: "a1", conditionId: "prone" } })],
    'condition "prone" not present on actor "a1"');
});

// --- notes ------------------------------------------------------------------

test("a note key must not be empty, and the message names the limit", () => {
  rejects([started, env(2, { noteUpserted: { key: "", title: "T", text: "x" } })],
    "note key is shorter than 1 bytes");
});

test("a note key longer than 128 bytes is rejected", () => {
  rejects([started, env(2, { noteUpserted: { key: "k".repeat(129), title: "T", text: "x" } })],
    "note key exceeds 128 bytes");
});

test("a note key of exactly 128 bytes is ACCEPTED", () => {
  // The boundary itself. `n > max` -> `n >= max` passes the rejection test
  // above and fails only here.
  const st = fold([started, env(2, { noteUpserted: { key: "k".repeat(128), title: "T", text: "x" } })]);
  expect(Object.keys(st.Notes)).toHaveLength(1);
});

test("a note title may be EMPTY, unlike a key or text", () => {
  // checkLen("note title", v.title, 0, 256) — the 0 minimum is the difference,
  // and a mutant raising it to 1 is invisible without this.
  const st = fold([started, env(2, { noteUpserted: { key: "k", title: "", text: "x" } })]);
  expect(st.Notes["k"]!.Title).toBe("");
});

test("a note title longer than 256 bytes is rejected", () => {
  rejects([started, env(2, { noteUpserted: { key: "k", title: "t".repeat(257), text: "x" } })],
    "note title exceeds 256 bytes");
});

test("empty note text is rejected", () => {
  rejects([started, env(2, { noteUpserted: { key: "k", title: "T", text: "" } })],
    "note text is shorter than 1 bytes");
});

test("note text longer than 8192 bytes is rejected", () => {
  rejects([started, env(2, { noteUpserted: { key: "k", title: "T", text: "x".repeat(8193) } })],
    "note text exceeds 8192 bytes");
});

test("deleting a note that is not present is rejected", () => {
  rejects([started, env(2, { noteDeleted: { key: "gone" } })], 'note "gone" deleted but not present');
});

// --- resources --------------------------------------------------------------

const withHP = (current: number, max: number) => [
  started,
  env(2, { actorAdded: { actor: { actorId: "a1", name: "A", resources: { hp: { current, max } } } } }),
];

test("a resource change on an unknown actor is rejected", () => {
  rejects([started, env(2, { resourceChanged: { actorId: "ghost", resource: "hp", delta: -1, newValue: 0 } })],
    'resource changed on unknown actor "ghost"');
});

test("a change to a resource the actor does not have is rejected", () => {
  rejects([...withHP(5, 10), env(3, { resourceChanged: { actorId: "a1", resource: "mana", delta: -1, newValue: 4 } })],
    'resource changed for unknown resource "mana" on actor "a1"');
});

test("a newValue disagreeing with the computation is rejected, and both numbers are named", () => {
  // The engine VERIFIES the event's arithmetic rather than trusting it. The
  // message carries both numbers because "mismatch" alone cannot be acted on.
  rejects([...withHP(5, 10), env(3, { resourceChanged: { actorId: "a1", resource: "hp", delta: -2, newValue: 99 } })],
    'resource "hp" on actor "a1": event new_value 99 does not match computed 3');
});

// --- narration --------------------------------------------------------------

test("empty narration text is rejected", () => {
  rejects([started, env(2, { narrationAdded: { text: "", as: "DM" } })],
    "narration text is shorter than 1 bytes");
});

test("narration longer than 8192 bytes is rejected", () => {
  rejects([started, env(2, { narrationAdded: { text: "x".repeat(8193), as: "DM" } })],
    "narration text exceeds 8192 bytes");
});

test("a half-set narration anchor is rejected from either end", () => {
  rejects([started, env(3, { narrationAdded: { text: "t", as: "DM", anchorFromSeq: "1" } })],
    "narration anchor requires both ends set");
  rejects([started, env(3, { narrationAdded: { text: "t", as: "DM", anchorToSeq: "1" } })],
    "narration anchor requires both ends set");
});

test("a narration anchor whose ends are out of order is rejected", () => {
  rejects([started, env(4, { narrationAdded: { text: "t", as: "DM", anchorFromSeq: "3", anchorToSeq: "2" } })],
    "narration anchor_from_seq must not exceed anchor_to_seq");
});

test("a narration anchor pointing at its OWN sequence is rejected", () => {
  // `anchorToSeq >= env.sequence` — the boundary. An anchor may point at the
  // event just before it, never at itself.
  rejects([started, env(3, { narrationAdded: { text: "t", as: "DM", anchorFromSeq: "3", anchorToSeq: "3" } })],
    "narration anchor must point backwards");
});

test("a narration anchor pointing at the immediately preceding event is ACCEPTED", () => {
  const st = fold([started, env(2, { narrationAdded: { text: "t", as: "DM", anchorFromSeq: "1", anchorToSeq: "1" } })]);
  expect(st.Sessions).toHaveLength(1);
});

// --- retraction -------------------------------------------------------------

test("a retraction marker removes its whole INCLUSIVE range and leaves no trace itself", () => {
  const st = fold([
    started, scene(2, "s1"), scene(3, "s2"),
    env(4, { eventsRetracted: { fromSequence: "2", toSequence: "3", reason: "undo" } }),
  ]);
  expect(Object.keys(st.Scenes)).toHaveLength(0);
  expect(st.Sessions).toHaveLength(1);
});

test("a retraction of a MALFORMED event stops it being rejected", () => {
  // The two passes are ordered for this reason: a retracted event is never
  // applied, so it cannot fail the fold. Applying first and retracting after
  // would throw on a log the server considers valid.
  const st = fold([
    started,
    env(2, { tokenMoved: { tokenId: "ghost", to: { x: 1, y: 1 } } }),
    env(3, { eventsRetracted: { fromSequence: "2", toSequence: "2", reason: "undo" } }),
  ]);
  expect(st.Sessions).toHaveLength(1);
});

// --- forward compatibility --------------------------------------------------

test("an event kind the fold does not know is skipped, not fatal", () => {
  // Matches the server's own replay. Turning the default branch into a throw
  // would make every client crash the day the contract gains an event.
  const st = fold([started,
    env(2, { attackRolled: { total: 17 } }),
    env(3, { abilityUsed: { actorId: "a1", abilityId: "x" } }),
    env(4, { adventureLoaded: { adventureId: "adv" } }),
  ]);
  expect(st.Sessions).toHaveLength(1);
  expect(Object.keys(st.Actors)).toHaveLength(0);
});

// --- actor control ----------------------------------------------------------
//
// The TS fold has to reject exactly what internal/engine's fold rejects, for
// the same reasons — an unknown actor leaves the log meaning nothing, and an
// empty participant would put "" in the set, making controllerIds non-empty
// while controllerId mirrors an empty string. That is the "shared or unowned?"
// ambiguity the mirror rule exists to remove.

test("actor control granted names an unknown actor", () => {
  rejects(
    [started, env(2, { actorControlGranted: { actorId: "ghost", participantId: "p-1" } })],
    'actor control granted names unknown actor "ghost"',
  );
});

test("actor control revoked names an unknown actor", () => {
  rejects(
    [started, env(2, { actorControlRevoked: { actorId: "ghost", participantId: "p-1" } })],
    'actor control revoked names unknown actor "ghost"',
  );
});

test("actor control granted with no participant", () => {
  rejects(
    [started, actor(2, "a1"), env(3, { actorControlGranted: { actorId: "a1", participantId: "" } })],
    "actor control granted requires a participant id",
  );
});

test("actor control revoked with no participant", () => {
  rejects(
    [started, actor(2, "a1"), env(3, { actorControlRevoked: { actorId: "a1", participantId: "" } })],
    "actor control revoked requires a participant id",
  );
});
