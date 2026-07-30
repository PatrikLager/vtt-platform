// The story feed: the log rendered as a narrative, not as a list of rows.
//
// Two rules do the work:
//
//   * Anchored narration is grouped WITH the events it describes. "Lera steps
//     toward the bar" belongs beside the move it narrates; rendering it as a
//     separate later line reads as a second, unrelated beat. That grouping is
//     the entire reason NarrationAdded carries an anchor range at all.
//   * Retracted events vanish, exactly as they do from the fold. A feed that
//     still showed a retracted move would have a spectator watching a beat
//     the DM took back — the client disagreeing with the server about what
//     happened, in the one place a human is looking.

import type { Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";

export interface Narration {
  seq: bigint;
  text: string;
  /** Speaker label for in-character speech; empty for table talk. */
  as: string;
}

export interface FeedEntry {
  /** Where this entry sits chronologically. */
  seq: bigint;
  /** Mechanical events, in sequence order. Empty for standalone narration. */
  events: Envelope[];
  /** Narration shown with this entry. */
  narrations: Narration[];
}

export function buildFeed(log: Envelope[]): FeedEntry[] {
  // Pass 1: what was taken back. Same inclusive-range rule the fold uses.
  const retracted = new Set<bigint>();
  for (const e of log) {
    if (e.payload.case === "eventsRetracted") {
      const r = e.payload.value;
      for (let s = r.fromSequence; s <= r.toSequence; s++) retracted.add(s);
    }
  }

  const live = log.filter((e) => !retracted.has(e.sequence));
  const isMechanical = (e: Envelope) =>
    // The retraction MARKER is bookkeeping, not a story beat: it changes the
    // feed by REMOVING entries, and rendering it too would narrate the
    // erasure as though it were something that happened at the table.
    e.payload.case !== "narrationAdded" && e.payload.case !== "eventsRetracted";

  // Pass 2: decide which entry each mechanical event belongs to. By default an
  // event is its own beat; an anchored narration MERGES everything in its
  // range into one, because "she crosses the room in three strides" is a
  // single beat containing three moves, not one beat plus two orphan rows.
  const groupOf = new Map<bigint, bigint>();
  for (const e of live) if (isMechanical(e)) groupOf.set(e.sequence, e.sequence);

  for (const e of live) {
    if (e.payload.case !== "narrationAdded") continue;
    const n = e.payload.value;
    if (n.anchorFromSeq <= 0n || n.anchorToSeq <= 0n) continue;
    const inRange = live
      .filter((c) => isMechanical(c) && c.sequence >= n.anchorFromSeq && c.sequence <= n.anchorToSeq)
      .map((c) => c.sequence);
    if (inRange.length === 0) continue;
    // Overlapping anchors are resolved last-writer-wins, deliberately: the
    // alternative is a union-find over ranges, and two DMs anchoring
    // overlapping narration to the same moves is not a case worth that
    // machinery. The later narration simply owns the grouping.
    const head = inRange[0]!;
    for (const seq of inRange) groupOf.set(seq, head);
  }

  const bySeq = new Map<bigint, FeedEntry>();
  const entryAt = (seq: bigint): FeedEntry => {
    let e = bySeq.get(seq);
    if (!e) {
      e = { seq, events: [], narrations: [] };
      bySeq.set(seq, e);
    }
    return e;
  };

  for (const e of live) {
    if (!isMechanical(e)) continue;
    entryAt(groupOf.get(e.sequence)!).events.push(e);
  }

  for (const e of live) {
    if (e.payload.case !== "narrationAdded") continue;
    const n = e.payload.value;
    const narration: Narration = { seq: e.sequence, text: n.text, as: n.as };
    const anchored = n.anchorFromSeq > 0n && n.anchorToSeq > 0n;
    const target = anchored
      ? live.find(
          (c) => isMechanical(c) && c.sequence >= n.anchorFromSeq && c.sequence <= n.anchorToSeq,
        )
      : undefined;
    // No live event in the anchored range means a spectator who joined
    // mid-session has a truncated log. The narration stands on its own rather
    // than being dropped: silently hiding story from late joiners is worse
    // than showing it slightly out of place.
    entryAt(target ? groupOf.get(target.sequence)! : e.sequence).narrations.push(narration);
  }

  return [...bySeq.values()]
    .filter((e) => e.events.length > 0 || e.narrations.length > 0)
    .sort((a, b) => (a.seq < b.seq ? -1 : a.seq > b.seq ? 1 : 0));
}
