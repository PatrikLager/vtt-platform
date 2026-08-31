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

test("a retraction marker hides NOTHING — every envelope in the log reaches the feed", () => {
  // Retraction left the platform (Patrik, 2026-08-30), and the feed's job is
  // still to agree with the fold about what happened. The fold now applies
  // every envelope, so the feed shows every envelope. This asserts the
  // ABSENCE of the hiding: before the removal it was [1], because 2 and 3
  // were erased and 4 was bookkeeping nobody rendered.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 9, y: 9 }),
    said(3, "A misstep, undone.", [2, 2]),
    env(4, {
      case: "eventsRetracted",
      value: create(EventsRetractedSchema, { fromSequence: 2n, toSequence: 3n, reason: "undo" }),
    }),
  ]);
  // 3 groups onto 2, the move it anchors; 4 is its own beat like any other.
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2, 4]);
});

test("narration anchored to a sequence that is not present still shows", () => {
  // A spectator who connected mid-session has a truncated log. Dropping the
  // narration because its anchor is below their cursor would silently hide
  // story from late joiners.
  const feed = buildFeed([said(9, "Earlier, a bargain was struck.", [2, 2])]);
  expect(feed).toHaveLength(1);
  expect(feed[0]!.narrations[0]!.text).toBe("Earlier, a bargain was struck.");
});

test("the retraction marker is an ordinary envelope now, with a beat of its own", () => {
  // It used to be filtered out as bookkeeping, on the grounds that rendering
  // it would narrate an erasure. Nothing is erased, so there is nothing to
  // treat specially: it is an event that happened, shown where it happened.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    env(2, {
      case: "eventsRetracted",
      value: create(EventsRetractedSchema, { fromSequence: 1n, toSequence: 1n, reason: "undo" }),
    }),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2]);
});

// --- anchor boundaries, grouping rules, and what the feed refuses to show ----
//
// The anchor arithmetic is where this module's bugs would live, and it was
// almost entirely unpinned: the mutation gate found 42 survivors here, mostly
// in the range comparisons and the guards around them. These pin the EDGES,
// because an anchor is an inclusive range and an off-by-one silently swallows
// or orphans the event at either end.

test("an anchor range includes the events at BOTH of its ends", () => {
  // Inclusive at from and at to. Exclusive at either end would split the beat
  // the DM deliberately joined, leaving an orphan row beside it.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 2, y: 2 }),
    moved(3, { x: 3, y: 3 }),
    said(4, "Three strides.", [1, 3]),
  ]);
  expect(feed.length).toBe(1);
  expect(feed[0]!.events.map((e) => Number(e.sequence))).toEqual([1, 2, 3]);
});

test("events outside the anchor range keep their own entries", () => {
  // The other half of the same boundary: 1 and 5 are outside [2,3] and must
  // NOT be pulled in. A comparison flipped to always-true swallows the whole
  // log into one beat.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 2, y: 2 }),
    moved(3, { x: 3, y: 3 }),
    moved(5, { x: 5, y: 5 }),
    said(6, "The middle two.", [2, 3]),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2, 5]);
  const grouped = feed.find((e) => Number(e.seq) === 2)!;
  expect(grouped.events.map((e) => Number(e.sequence))).toEqual([2, 3]);
  expect(feed.find((e) => Number(e.seq) === 1)!.events.length).toBe(1);
  expect(feed.find((e) => Number(e.seq) === 5)!.events.length).toBe(1);
});

test("a half-specified anchor is treated as unanchored, not as a range from zero", () => {
  // Both ends must be > 0. A narration carrying only one end is malformed;
  // reading it as a range starting at 0 would hoover up the entire log into
  // the first beat.
  const onlyFrom = create(NarrationAddedSchema, { text: "half", anchorFromSeq: 2n, anchorToSeq: 0n });
  const onlyTo = create(NarrationAddedSchema, { text: "other half", anchorFromSeq: 0n, anchorToSeq: 3n });
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 2, y: 2 }),
    env(3, { case: "narrationAdded", value: onlyFrom }),
    env(4, { case: "narrationAdded", value: onlyTo }),
  ]);
  // Four separate beats: nothing was grouped.
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2, 3, 4]);
  expect(feed.find((e) => Number(e.seq) === 3)!.narrations[0]!.text).toBe("half");
  expect(feed.find((e) => Number(e.seq) === 4)!.narrations[0]!.text).toBe("other half");
});

test("a narration whose whole anchor range is MISSING stands alone rather than vanishing", () => {
  // Kept, with its mechanism swapped: the range used to be emptied by a
  // retraction and is now emptied by a truncated log, which is the only way
  // left to empty one. The assertions are unchanged, including the one the
  // neighbouring "still shows" test does not make — that the standalone
  // entry carries NO events. Drop that and a narration could silently be
  // stapled to a beat it does not describe.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    said(3, "About the move.", [2, 2]),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 3]);
  expect(feed.find((e) => Number(e.seq) === 3)!.narrations[0]!.text).toBe("About the move.");
  expect(feed.find((e) => Number(e.seq) === 3)!.events).toEqual([]);
});

test("overlapping anchors resolve last-writer-wins", () => {
  // Documented as deliberate: the alternative is a union-find over ranges. The
  // LATER narration owns the grouping, and this pins that it is the later one
  // rather than the first or an arbitrary winner.
  const feed = buildFeed([
    moved(1, { x: 1, y: 1 }),
    moved(2, { x: 2, y: 2 }),
    moved(3, { x: 3, y: 3 }),
    said(4, "first claim", [1, 2]),
    said(5, "second claim", [2, 3]),
  ]);
  // The second anchor regrouped 2 and 3 onto head 2, and 1 was left with the
  // first claim's grouping — one beat at 1, one at 2.
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2]);
  const all = feed.flatMap((e) => e.narrations.map((n) => n.text));
  expect(all).toContain("first claim");
  expect(all).toContain("second claim");
});

test("entries are ordered ascending even when the log arrives shuffled", () => {
  // buildFeed reads a Map's values, whose order is insertion order — which is
  // the order events were seen, not sequence order. The sort is what makes the
  // feed chronological, and two events are not enough to prove direction.
  const feed = buildFeed([
    moved(9, { x: 9, y: 9 }),
    moved(3, { x: 3, y: 3 }),
    moved(7, { x: 7, y: 7 }),
    moved(1, { x: 1, y: 1 }),
    moved(5, { x: 5, y: 5 }),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 3, 5, 7, 9]);
});

test("an anchor whose range is empty does not capture the next event after it", () => {
  // The placement pass finds the FIRST live mechanical event in [from,to].
  // Drop the upper bound from that search and the find happily returns the
  // next event past the range — so narration anchored to a moment that is not
  // in this log gets stapled onto an unrelated later one, which reads as the
  // DM having described something they did not.
  //
  // Range [2,2] with nothing at 2 and an event at 3 is the smallest case that
  // separates "no match" from "first match past the end".
  const feed = buildFeed([
    moved(3, { x: 3, y: 3 }),
    said(4, "About a moment not in this log.", [2, 2]),
  ]);
  expect(feed.map((e) => Number(e.seq))).toEqual([3, 4]);
  expect(feed.find((e) => Number(e.seq) === 3)!.narrations).toEqual([]);
  expect(feed.find((e) => Number(e.seq) === 4)!.narrations[0]!.text).toBe(
    "About a moment not in this log.",
  );
});

test("an envelope whose payload variant is unset is carried, not choked on", () => {
  // The contract's payload oneof has a declared unset member,
  // `{ case: undefined; value?: undefined }`, and it is a first-class input
  // rather than a curiosity: fold.ts SKIPS unknown payload variants by design
  // ("the same forward compatibility the server's own replay gives"), so a
  // client meeting a newer server folds such an envelope without error and
  // hands it straight here.
  //
  // Both payload-kind guards in buildFeed exist to keep it away from fields it
  // does not have. Bypass either and `e.payload.value` is undefined, the
  // dereference throws, and the whole feed disappears — a blank story panel
  // beside a working board, for a log the fold considered fine.
  //
  // This was adjudicated "equivalent" on the reasoning that no OTHER payload
  // carries fromSequence/toSequence, which is true and irrelevant: the unset
  // member carries nothing at all.
  const bare = create(EnvelopeSchema, { eventId: "x", sequence: 1n });
  expect(bare.payload.case).toBeUndefined();
  expect(buildFeed([bare]).map((e) => Number(e.seq))).toEqual([1]);
});

test("an unset payload alongside real events leaves the real ones intact", () => {
  // The same input in the position it would actually arrive in — mid-log,
  // between events an older client does understand.
  const bare = create(EnvelopeSchema, { eventId: "x", sequence: 2n });
  const feed = buildFeed([moved(1, { x: 1, y: 1 }), bare, moved(3, { x: 3, y: 3 })]);
  expect(feed.map((e) => Number(e.seq))).toEqual([1, 2, 3]);
});
