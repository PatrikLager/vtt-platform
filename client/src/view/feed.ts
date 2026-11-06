// The Story's beats (SPEC-022). Keep an anchored narration in the beat of the
// events its range names, and show every entry of the log but the runs
// reintroductions names (VTT-662). Leave them out here alone: the fold, the
// namer and the Events panel are handed the whole log.

import type { Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";

export interface Narration {
  seq: bigint;
  text: string;
  /** Speaker label: a character's, or the sender's out of character; empty for narration. */
  as: string;
  ooc: boolean;
}

export interface FeedEntry {
  /** Where this entry sits chronologically. */
  seq: bigint;
  /** Mechanical events, in sequence order. Empty for standalone narration. */
  events: Envelope[];
  /** Narration shown with this entry. */
  narrations: Narration[];
}

export function buildFeed(whole: Envelope[]): FeedEntry[] {
  const left = reintroductions(whole);
  const log = whole.filter((e) => !left.has(e));

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
    const narration: Narration = { seq: e.sequence, text: n.text, as: n.as, ooc: n.outOfCharacter };
    const anchored = n.anchorFromSeq > 0n && n.anchorToSeq > 0n;
    const target = anchored
      ? log.find(
          (c) => isMechanical(c) && c.sequence >= n.anchorFromSeq && c.sequence <= n.anchorToSeq,
        )
      : undefined;
    // Never drop a narration whose range names no entry kept here, as after a
    // left-out run: give it a beat of its own (VTT-662).
    entryAt(target ? groupOf.get(target.sequence)! : e.sequence).narrations.push(narration);
  }

  return [...bySeq.values()]
    .filter((e) => e.events.length > 0 || e.narrations.length > 0)
    .sort((a, b) => (a.seq < b.seq ? -1 : a.seq > b.seq ? 1 : 0));
}

/** reintroductions is what the Story leaves out of log (VTT-662). */
function reintroductions(log: readonly Envelope[]): Set<Envelope> {
  const out = new Set<Envelope>();
  for (let i = 0; i + 1 < log.length; i++) {
    const gone = log[i]!;
    const back = log[i + 1]!.payload;
    if (gone.eventId !== "" || gone.payload.case !== "actorRemoved" || back.case !== "actorAdded") continue;
    const id = gone.payload.value.actorId;
    if (back.value.actor?.actorId !== id) continue;
    out.add(gone).add(log[i + 1]!);
    for (let j = i + 2; j < log.length && restates(log[j]!, id); j++) out.add(log[j]!);
  }
  return out;
}

function restates(e: Envelope, id: string): boolean {
  const p = e.payload;
  return e.eventId === "" && (p.case === "actorControlGranted" || p.case === "conditionApplied") && p.value.actorId === id;
}
