import { test, expect } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  EnvelopeSchema, SessionStartedSchema, TokenMovedSchema, EventsRetractedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { lastUndoable, retractableRange, isRetracted } from "../src/undo";

const env = (seq: number, payload: Envelope["payload"]): Envelope =>
  create(EnvelopeSchema, { eventId: `evt-${seq}`, sequence: BigInt(seq), payload });

const moved = (seq: number) =>
  env(seq, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", to: { x: 1, y: 1 } }) });

const retraction = (seq: number, from: number, to: number) =>
  env(seq, {
    case: "eventsRetracted",
    value: create(EventsRetractedSchema, { fromSequence: BigInt(from), toSequence: BigInt(to), reason: "undo" }),
  });

test("the last undoable event is the newest live one", () => {
  expect(lastUndoable([
    env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
    moved(2),
  ])).toBe(2n);
});

test("an already-retracted event is not offered again", () => {
  // Undo must step BACK through history, not repeatedly offer the same move.
  expect(lastUndoable([moved(1), moved(2), retraction(3, 2, 2)])).toBe(1n);
});

test("a retraction MARKER is never itself the undo target", () => {
  // Offering "undo the undo" as the default action is a trap: the DM presses
  // undo twice expecting to go back two steps and instead reinstates the
  // first one.
  expect(lastUndoable([moved(1), retraction(2, 1, 1)])).toBeNull();
});

test("an empty or fully-retracted log has nothing to undo", () => {
  expect(lastUndoable([])).toBeNull();
  expect(lastUndoable([moved(1), retraction(2, 1, 1)])).toBeNull();
});

test("isRetracted reports coverage by any marker's inclusive range", () => {
  const log = [moved(1), moved(2), moved(3), retraction(4, 2, 3)];
  expect(isRetracted(log, 1n)).toBe(false);
  expect(isRetracted(log, 2n)).toBe(true);
  expect(isRetracted(log, 3n)).toBe(true);
});

test("a valid range is accepted", () => {
  expect(retractableRange([moved(1), moved(2), moved(3)], 1n, 3n)).toBeNull();
});

test("an inverted range is refused with a reason", () => {
  const err = retractableRange([moved(1), moved(2)], 2n, 1n);
  expect(err).toMatch(/order/i);
});

test("a range containing nothing live is refused", () => {
  // Otherwise the DM gets a confirmation dialog for an undo that would do
  // nothing, and a retraction marker in the log implying something changed.
  const err = retractableRange([moved(1), retraction(2, 1, 1)], 1n, 1n);
  expect(err).toMatch(/already retracted|nothing/i);
});

test("a range beyond the log is refused", () => {
  const err = retractableRange([moved(1)], 1n, 99n);
  expect(err).not.toBeNull();
});

test("a zero or negative bound is refused — sequences start at 1", () => {
  expect(retractableRange([moved(1)], 0n, 1n)).not.toBeNull();
});

// --- boundaries -------------------------------------------------------------
//
// Everything below was written to kill surviving mutants. undo.ts sat at 100%
// LINE coverage and 78.75% mutation score: every line ran, and nothing checked
// the comparisons. The survivors were `>= from` -> `> from`, `<= to` -> `< to`
// and `> head` -> `>= head` — the inclusive [from,to] range that IS retraction,
// and the off-by-one that would let a DM retract an event that has not
// happened yet.

test("a range of exactly one event, at the head, is retractable", () => {
  // Kills `e.sequence >= from` -> `> from`, `e.sequence <= to` -> `< to`, and
  // `to > head` -> `>= head`: each of those makes this exact case fail.
  expect(retractableRange([moved(1), moved(2)], 2n, 2n)).toBeNull();
  expect(retractableRange([moved(1), moved(2)], 1n, 1n)).toBeNull();
});

test("a range ending exactly at the head is allowed; one past it is not", () => {
  const log = [moved(1), moved(2)];
  expect(retractableRange(log, 1n, 2n)).toBeNull();
  expect(retractableRange(log, 1n, 3n)).toBe(
    "range extends past the last event (#2) — that would retract events before they happen",
  );
});

test("the past-the-head message names the actual head", () => {
  // Kills the StringLiteral mutant that empties this message. The number is
  // the whole point: a DM told only "invalid" cannot tell what to type next.
  expect(retractableRange([moved(1), moved(2), moved(3)], 1n, 9n)).toContain("(#3)");
});

test("sequence zero is refused from either end, with the reason", () => {
  // Kills `from <= 0n || to <= 0n` -> `false`, the `to < 0n` boundary shift,
  // and the empty-string mutant on the message.
  const log = [moved(1)];
  expect(retractableRange(log, 0n, 1n)).toBe("sequences start at 1");
  expect(retractableRange(log, 1n, 0n)).toBe("sequences start at 1");
  expect(retractableRange(log, 0n, 0n)).toBe("sequences start at 1");
});

test("an out-of-order range is refused with its own distinct message", () => {
  expect(retractableRange([moved(1), moved(2)], 2n, 1n)).toBe(
    "range is out of order — the first sequence must not exceed the last",
  );
});

test("a range holding only retraction markers is refused, not accepted as live", () => {
  // Kills `e.payload.case !== "eventsRetracted"` -> `true` and the
  // StringLiteral mutant on that case name: with either, the marker itself
  // counts as live and the range looks retractable.
  const log = [moved(1), retraction(2, 1, 1)];
  expect(retractableRange(log, 2n, 2n)).toBe(
    "nothing live in that range — it is empty or already retracted",
  );
});

test("a range of already-retracted events is refused", () => {
  const log = [moved(1), moved(2), retraction(3, 1, 2)];
  expect(retractableRange(log, 1n, 2n)).toBe(
    "nothing live in that range — it is empty or already retracted",
  );
});

test("head is the MAXIMUM sequence, not the last element's", () => {
  // Kills `e.sequence > head` -> `true`, which would leave head as the final
  // event's sequence regardless of order. Replay is ordered today; this pins
  // that the rule does not silently depend on it.
  const log = [moved(3), moved(1), moved(2)];
  expect(retractableRange(log, 3n, 3n)).toBeNull();
  expect(retractableRange(log, 1n, 4n)).toContain("(#3)");
});

test("lastUndoable picks the highest sequence, not the last seen", () => {
  // Kills `best === null || e.sequence > best` -> `true` (last wins),
  // -> `false` (never updates), -> `best !== null`, and `>` -> `>=`.
  expect(lastUndoable([moved(3), moved(1), moved(2)])).toBe(3n);
});

test("lastUndoable is null for a log with nothing live in it", () => {
  expect(lastUndoable([])).toBeNull();
  expect(lastUndoable([moved(1), retraction(2, 1, 1)])).toBeNull();
});

test("only retraction MARKERS retract — an ordinary event never does", () => {
  // Kills `e.payload.case === "eventsRetracted"` -> `true` in retractedSet,
  // under which every event would mark a range as retracted.
  expect(isRetracted([moved(1), moved(2)], 1n)).toBe(false);
  expect(isRetracted([moved(1), retraction(2, 1, 1)], 1n)).toBe(true);
});

test("an event below `from` is not counted as live in the range", () => {
  // Kills `e.sequence >= from` -> `true`. Without the lower bound, event 1
  // makes the range look retractable when the only event actually IN it is
  // already gone.
  const log = [moved(1), moved(2), retraction(3, 2, 2)];
  expect(retractableRange(log, 2n, 2n)).toBe(
    "nothing live in that range — it is empty or already retracted",
  );
});

test("an event above `to` is not counted as live in the range", () => {
  // Kills `e.sequence <= to` -> `true`, the mirror of the above.
  const log = [moved(1), moved(2), retraction(3, 1, 1)];
  expect(retractableRange(log, 1n, 1n)).toBe(
    "nothing live in that range — it is empty or already retracted",
  );
});

test("an envelope carrying no payload at all is handled, not thrown on", () => {
  // A malformed frame must not take the console down: undo runs on every
  // render, so a throw here blanks the DM's screen rather than skipping one
  // row. Also kills `e.payload.case === "eventsRetracted"` -> `true`, under
  // which a payload-less envelope dereferences undefined.
  const bare = create(EnvelopeSchema, { eventId: "bare", sequence: 1n });
  expect(() => isRetracted([bare], 1n)).not.toThrow();
  expect(isRetracted([bare], 1n)).toBe(false);
});

test("a sequence-0 event is still offered as undoable", () => {
  // Sequences start at 1 — retractableRange says so in its own message — so a
  // 0 here means a default-constructed or malformed envelope. Current
  // behaviour offers it, and "Undo #0" is arguably the wrong thing to show;
  // that is a product call, so this PINS the behaviour rather than changing
  // it.
  //
  // It also kills `best === null` -> `false` in lastUndoable, which is NOT an
  // equivalent mutant even though it looks like one: the mutated condition
  // falls through to `e.sequence > best`, and `0n > null` coerces null to 0,
  // so the mutant returns null here where the original returns 0n. Every
  // sequence >= 1 hides the difference, which is exactly why it needs pinning
  // at 0 rather than adjudicating away.
  const zero = env(0, {
    case: "tokenMoved",
    value: create(TokenMovedSchema, { tokenId: "t1", to: { x: 1, y: 1 } }),
  });
  expect(lastUndoable([zero])).toBe(0n);
});
