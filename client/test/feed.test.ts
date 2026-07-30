import { test, expect } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  SessionStartedSchema,
  TokenMovedSchema,
  NarrationAddedSchema,
  EventsRetractedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { buildFeed } from "../src/view/feed";

function env(seq: number, payload: Envelope["payload"]): Envelope {
  return create(EnvelopeSchema, { eventId: `evt-${seq}`, sequence: BigInt(seq), payload });
}

const moved = (seq: number, to: { x: number; y: number }) =>
  env(seq, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", to }) });

const said = (seq: number, text: string, anchor?: [number, number], as?: string) =>
  env(seq, {
    case: "narrationAdded",
    value: create(NarrationAddedSchema, {
      text,
      as: as ?? "",
      anchorFromSeq: anchor ? BigInt(anchor[0]) : 0n,
      anchorToSeq: anchor ? BigInt(anchor[1]) : 0n,
    }),
  });

test("entries are chronological", () => {
  const feed = buildFeed([
    env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
    moved(2, { x: 1, y: 1 }),
    said(3, "A hush falls."),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2, 3]);
});

test("anchored narration is grouped WITH the event it describes, not after it", () => {
  // This is the whole point of anchoring: "Lera steps toward the bar" belongs
  // beside the move it narrates. Rendering it as a separate later line reads
  // as a second, unrelated beat.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    said(2, "Lera steps toward the bar.", [1, 1]),
  ]);
  expect(feed).toHaveLength(1);
  expect(Number(feed[0]!.seq)).toBe(1);
  expect(feed[0]!.narrations.map((n) => n.text)).toEqual(["Lera steps toward the bar."]);
  expect(feed[0]!.events).toHaveLength(1);
});

test("a narration anchored across a RANGE groups every event in it", () => {
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 2, y: 2 }),
    moved(3, { x: 3, y: 3 }),
    said(4, "She crosses the room in three strides.", [1, 3]),
  ]);
  expect(feed).toHaveLength(1);
  expect(feed[0]!.events.map((e) => Number(e.sequence))).toEqual([1, 2, 3]);
});

test("unanchored narration stands on its own, in sequence", () => {
  const feed = buildFeed([moved(1, { x: 1, y: 1 }), said(2, "Table talk.")]);
  expect(feed).toHaveLength(2);
  expect(feed[1]!.events).toHaveLength(0);
  expect(feed[1]!.narrations[0]!.text).toBe("Table talk.");
});

test("in-character speech keeps its speaker", () => {
  const feed = buildFeed([said(1, "You'll not pass!", undefined, "Goblin Cutter")]);
  expect(feed[0]!.narrations[0]!.as).toBe("Goblin Cutter");
});

test("retracted events disappear from the feed, and so does narration anchored to them", () => {
  // The feed must agree with the fold. A retracted move that still showed in
  // the story would have a spectator watching a beat the DM took back.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 9, y: 9 }),
    said(3, "A misstep, undone.", [2, 2]),
    env(4, {
      case: "eventsRetracted",
      value: create(EventsRetractedSchema, { fromSequence: 2n, toSequence: 3n, reason: "undo" }),
    }),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1]);
});

test("narration anchored to a sequence that is not present still shows", () => {
  // A spectator who connected mid-session has a truncated log. Dropping the
  // narration because its anchor is below their cursor would silently hide
  // story from late joiners.
  const feed = buildFeed([said(9, "Earlier, a bargain was struck.", [2, 2])]);
  expect(feed).toHaveLength(1);
  expect(feed[0]!.narrations[0]!.text).toBe("Earlier, a bargain was struck.");
});

test("the retraction marker itself is never rendered", () => {
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    env(2, {
      case: "eventsRetracted",
      value: create(EventsRetractedSchema, { fromSequence: 1n, toSequence: 1n, reason: "undo" }),
    }),
  ]);
  expect(feed).toHaveLength(0);
});
