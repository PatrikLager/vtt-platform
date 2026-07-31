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
