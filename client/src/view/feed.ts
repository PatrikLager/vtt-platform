// The story feed: the log rendered as a narrative, not as a list of rows.
//
// One rule does the work: anchored narration is grouped WITH the events it
// describes. "Lera steps toward the bar" belongs beside the move it narrates;
// rendering it as a separate later line reads as a second, unrelated beat.
// That grouping is the entire reason NarrationAdded carries an anchor range
// at all.
//
// Everything in the log is shown. The feed used to hide what a retraction
// marker covered, so that it agreed with the fold about what had happened;
// the log only goes forward now, so agreeing with the fold means showing
// every envelope it applies.

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
  // Narration is not a mechanical beat: it is the prose ABOUT the beats, and
  // it is placed by the anchor loop below rather than rendered as a row of
  // its own.
  const isMechanical = (e: Envelope) => e.payload.case !== "narrationAdded";

  // Decide which entry each mechanical event belongs to. By default an
  // event is its own beat; an anchored narration MERGES everything in its
  // range into one, because "she crosses the room in three strides" is a
  // single beat containing three moves, not one beat plus two orphan rows.
  const groupOf = new Map<bigint, bigint>();
  for (const e of log) if (isMechanical(e)) groupOf.set(e.sequence, e.sequence);

  for (const e of log) {
    if (e.payload.case !== "narrationAdded") continue;
    const n = e.payload.value;
    if (n.anchorFromSeq <= 0n || n.anchorToSeq <= 0n) continue;
    const inRange = log
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

  for (const e of log) {
    if (!isMechanical(e)) continue;
    entryAt(groupOf.get(e.sequence)!).events.push(e);
  }

  for (const e of log) {
    if (e.payload.case !== "narrationAdded") continue;
    const n = e.payload.value;
    const narration: Narration = { seq: e.sequence, text: n.text, as: n.as };
    const anchored = n.anchorFromSeq > 0n && n.anchorToSeq > 0n;
    const target = anchored
      ? log.find(
          (c) => isMechanical(c) && c.sequence >= n.anchorFromSeq && c.sequence <= n.anchorToSeq,
        )
      : undefined;
    // No event in the anchored range means a spectator who joined mid-session
    // has a truncated log. The narration stands on its own rather than being
    // dropped: silently hiding story from late joiners is worse than showing
    // it slightly out of place.
    entryAt(target ? groupOf.get(target.sequence)! : e.sequence).narrations.push(narration);
  }

  return [...bySeq.values()]
    .filter((e) => e.events.length > 0 || e.narrations.length > 0)
    .sort((a, b) => (a.seq < b.seq ? -1 : a.seq > b.seq ? 1 : 0));
}
