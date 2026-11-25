// The Story's beats (SPEC-022). Keep an anchored narration in the beat of the
// events its range names, and leave out the runs reintroductions names and
// every entry told() refuses (VTT-984). Leave them out here alone: the fold,
// the namer and the Events panel are handed the whole log.

import type { Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";

type Kind = NonNullable<Envelope["payload"]["case"]>;

// Decide here whether the Story draws each kind the contract adds
// (VTT-1029, VTT-810, VTT-818, VTT-984, VTT-823).
const STORY: Record<Kind, boolean> = {
  sessionStarted: true,
  sessionEnded: true,
  actorAdded: true,
  actorRemoved: true,
  conditionApplied: true,
  conditionRemoved: true,
  abilityUsed: true,
  attackRolled: true,
  narrationAdded: true,
  whisperAdded: true,
  rulesetNamed: true,
  doorOpened: true,
  doorClosed: true,
  resourceChanged: false,
  abilityGiven: false,
  abilityTaken: false,
  resourceAdded: false,
  resourceRemoved: false,
  referenceAdded: false,
  referenceRemoved: false,
  referenceChanged: false,
  actorControlGranted: false,
  actorControlRevoked: false,
  tokenPlaced: false,
  tokenMoved: false,
  tokenHidden: false,
  tokenRemoved: false,
  tokenDescribed: false,
  sceneCreated: false,
  sceneSeen: false,
  noteUpserted: false,
  noteDeleted: false,
  adventureLoaded: false,
};

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
  const log = whole.filter((e) => !left.has(e) && told(e));

  // Narration is not a mechanical beat: it is the prose ABOUT the beats, and
  // it is placed by the anchor loop below rather than rendered as a row of
  // its own.
  const isMechanical = (e: Envelope) => e.payload.case !== "narrationAdded";

  // Merge every kept entry in an anchored narration's range into one beat
  // (SPEC-007).
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
    // Let the later narration own the grouping where anchors overlap
    // (feed.test.ts, "overlapping anchors resolve last-writer-wins").
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
    // Never drop a narration whose range names no entry kept here: give it a
    // beat of its own (VTT-812).
    entryAt(target ? groupOf.get(target.sequence)! : e.sequence).narrations.push(narration);
  }

  return [...bySeq.values()]
    .filter((e) => e.events.length > 0 || e.narrations.length > 0)
    .sort((a, b) => (a.seq < b.seq ? -1 : a.seq > b.seq ? 1 : 0));
}

// VTT-820 VTT-984
function told(e: Envelope): boolean {
  const kind = e.payload.case;
  if (kind === undefined) return true;
  const door = kind === "doorOpened" || kind === "doorClosed";
  return STORY[kind] && (!door || e.eventId !== "");
}

// Find the runs in the whole log, before told() leaves an entry out (VTT-984).
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
