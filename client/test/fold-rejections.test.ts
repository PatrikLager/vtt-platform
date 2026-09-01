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

// --- doors (maps-as-geometry spec §4.1) -------------------------------------
//
// fold-unit.test.ts folds these same four bad events in "a door event naming an
// unknown scene or no position is refused" and asserts only that a FoldError
// comes out — which is what four emptied MESSAGES hid behind: replace all four
// templates with "" and every one of them still throws, still of the right
// type, with that file still green. The two failures are different bugs on the
// wire and want telling apart: an unknown scene means this seat was sent a door
// event for a scene it never received, while a missing position means an event
// the server would not have written at all. Both arms exist in duplicate, one
// per direction, so all four are asserted rather than one of each.
//
// The unit-test case is left standing rather than folded into these: it is the
// only thing that folds both arms in one pass, and this file's header explains
// why message assertions live here instead of there.

const doorScene = [started, scene(2, "s1")];

test("a door opened in an unknown scene is rejected", () => {
  rejects([...doorScene, env(3, { doorOpened: { sceneId: "nope", at: { x: 0, y: 1 } } })],
    'door opened in unknown scene "nope"');
});

test("a door opened with no position is rejected", () => {
  rejects([...doorScene, env(3, { doorOpened: { sceneId: "s1" } })],
    "door opened without position");
});

test("a door closed in an unknown scene is rejected", () => {
  rejects([...doorScene, env(3, { doorClosed: { sceneId: "nope", at: { x: 0, y: 1 } } })],
    'door closed in unknown scene "nope"');
});

test("a door closed with no position is rejected", () => {
  rejects([...doorScene, env(3, { doorClosed: { sceneId: "s1" } })],
    "door closed without position");
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

// tokenRemoved (retraction-leaves Task 8, spec §5.1). Same idiom as
// tokenMoved's own unknown-token case directly above, and the message
// mirrors it word for word ("moved" -> "removed") per task-8-brief.md's own
// requirement that the two read as the same wording.
test("removing an unknown token is rejected", () => {
  rejects([...placeable, env(4, { tokenRemoved: { tokenId: "ghost" } })],
    'unknown token "ghost" removed');
});

// actorRemoved (retraction-leaves Task 9, spec §5.2). Two guards, and the
// second is the one that makes remove_actor's batch atomic: engine.Apply's
// ActorRemoved arm refuses an actor whose tokens are still on the board, so
// campaign.AppendBatch — which validates by folding — rejects a batch that
// would leave a token whose actor nobody can introduce. This fold mirrors both
// refusals, because "parity includes REJECTING what Go rejects".

test("removing an unknown actor is rejected", () => {
  rejects([...placeable, env(4, { actorRemoved: { actorId: "nope" } })],
    'unknown actor "nope" removed');
});

// TWO TOKENS, PLACED IN REVERSE ID ORDER, because the arm names the FIRST
// standing token IN ID ORDER and a one-token fixture cannot tell that apart
// from naming whichever one the object happens to hand back first. The Go
// twin, TestActorRemovedRefusesWhileOneOfItsTokensStillStands, uses the same
// "t-z" then "t-a" pair for the same reason — there it defends against Go's
// randomised map iteration, here against insertion order.
//
// Load-bearing, proved by removing the thing it pins rather than assumed:
// with `.sort()` deleted from fold.ts's actorRemoved arm this test names
// "t-z" and fails, while the whole 708-test suite stayed green against the
// one-token fixture it replaces (measured 2026-09-01).
test("removing an actor whose token is still on the board is rejected", () => {
  rejects([
    ...placeable,
    env(4, { tokenPlaced: { tokenId: "t-z", sceneId: "s1", actorId: "a1", position: { x: 1, y: 1 } } }),
    env(5, { tokenPlaced: { tokenId: "t-a", sceneId: "s1", actorId: "a1", position: { x: 2, y: 1 } } }),
    env(6, { actorRemoved: { actorId: "a1" } }),
  ], 'actor "a1" still has token "t-a" on the board — a token cannot outlive its actor');
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

// --- ids that name a prototype member ---------------------------------------
//
// Every guard below asks "is this thing in the map?" by reading the map. On a
// Go map that question has one answer; on a JavaScript object literal it has
// two, because a lookup of "valueOf" or "toString" finds Object.prototype's
// member and answers YES for something no writer ever put there. So each of
// these is a rejection Go makes that this fold made differently — or did not
// make at all — until st's maps became prototype-less dictionaries (state.ts's
// emptyMap). client/test/fold-unit.test.ts's section of the same name holds
// the other direction: logs Go ACCEPTS that this fold used to refuse.
//
// Two of the five accepted silently, which is the worse failure and the reason
// this is a correctness fix rather than hardening. The log is append-only and
// session.ts re-folds all of it on every event, so a silently-accepted event
// is not one wrong frame: this client's board and the server's disagree from
// that sequence onwards, with nothing reported to anyone.

test("a note that was never written cannot be deleted, even when its key names a prototype member", () => {
  // ACCEPTED SILENTLY before the fix: `st.Notes["valueOf"]` found
  // Object.prototype.valueOf, the guard saw a truthy value and let the delete
  // through, and `delete` on an inherited member removes nothing and reports
  // success. Go rejects the event, so the two folds disagreed about whether
  // the log was valid at all.
  rejects([started, env(2, { noteDeleted: { key: "valueOf" } })],
    'note "valueOf" deleted but not present');
});

test("a token that was never placed cannot be moved, even when its id names a prototype member", () => {
  // The same silent acceptance with a second consequence: `tok` WAS
  // Object.prototype.toString, so `tok.X = v.to.x` wrote through onto the real
  // prototype member — process-wide, for every object in the client, from
  // folding one ordinary-looking event.
  rejects([...placeable, env(4, { tokenMoved: { tokenId: "toString", to: { x: 1, y: 1 } } })],
    'unknown token "toString" moved');
});

test("sceneSeen for a scene that never arrived is refused, even when its id names a prototype member", () => {
  // This is the lookup that reached the sceneSeen arm's three defaulting
  // guards: `sc` was truthy and had no Tiles, no Explored and no Objects, so
  // the guards fired and defaulted them ONTO Object.prototype.hasOwnProperty.
  // With the scene lookup answering honestly the arm rejects here instead, and
  // those guards go back to being unreachable — which is what the four
  // adjudications in tools/ts-mutation-equivalents.txt claim, and what was not
  // true while this test was failing.
  rejects([started, env(2, { sceneSeen: { sceneId: "hasOwnProperty", tiles: { "0,0": { kind: "floor" } } } })],
    'scene seen for unknown scene "hasOwnProperty"');
});

test("actor control granted for an actor that was never added is refused, even when its id names a prototype member", () => {
  // Not a FoldError before the fix but a TypeError from reading
  // `a.controllerIds` off a function. session.ts hands whatever comes out to
  // its error handlers, so the seat was told something went wrong without
  // being told which parity rule fired — and nothing that branches on
  // FoldError could recognise it.
  rejects(
    [started, env(2, { actorControlGranted: { actorId: "toString", participantId: "p-1" } })],
    'actor control granted names unknown actor "toString"',
  );
});

test("a resource the actor does not have is refused BY NAME, even when that name is a prototype member", () => {
  // The message is the assertion, as everywhere in this file. Before the fix
  // this threw too, so a bare "it rejects" would have been green — but for the
  // wrong reason and with the wrong words: `res` was a function, `res.current`
  // undefined, and the arithmetic check reported 'new_value 4 does not match
  // computed NaN' where Go names the resource that does not exist.
  rejects([...withHP(5, 10), env(3, { resourceChanged: { actorId: "a1", resource: "hasOwnProperty", delta: -1, newValue: 4 } })],
    'resource changed for unknown resource "hasOwnProperty" on actor "a1"');
});
