// Undo: which events a DM may retract, and what "undo the last thing" means.
//
// Retraction is the one destructive-looking action in the console, so the
// rules here are about not surprising the person pressing it. The server
// validates independently; this decides what the UI offers and why.

import type { Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";

/** Sequences covered by any retraction marker's inclusive range. */
function retractedSet(log: Envelope[]): Set<bigint> {
  const out = new Set<bigint>();
  for (const e of log) {
    if (e.payload.case === "eventsRetracted") {
      const r = e.payload.value;
      for (let s = r.fromSequence; s <= r.toSequence; s++) out.add(s);
    }
  }
  return out;
}

export function isRetracted(log: Envelope[], seq: bigint): boolean {
  return retractedSet(log).has(seq);
}

/**
 * The event "undo" would take back: the newest one still live.
 *
 * Retraction MARKERS are never the target. Offering "undo the undo" as the
 * default is a trap — a DM pressing undo twice expects to step back two
 * events, and would instead reinstate the first one. Undoing a retraction is
 * a deliberate act, done by selecting that marker's range explicitly.
 */
export function lastUndoable(log: Envelope[]): bigint | null {
  const retracted = retractedSet(log);
  let best: bigint | null = null;
  for (const e of log) {
    if (e.payload.case === "eventsRetracted") continue;
    if (retracted.has(e.sequence)) continue;
    if (best === null || e.sequence > best) best = e.sequence;
  }
  return best;
}

/**
 * Validate a chosen range. Returns null when it is retractable, or a message
 * explaining why not — shown to the DM BEFORE the confirmation dialog, so a
 * pointless undo is never confirmed.
 */
export function retractableRange(log: Envelope[], from: bigint, to: bigint): string | null {
  if (from <= 0n || to <= 0n) {
    return "sequences start at 1";
  }
  if (from > to) {
    return "range is out of order — the first sequence must not exceed the last";
  }
  // A range must not extend past the head. Retracting [1, 99] on a log that
  // ends at 1 would mark sequences 2..99 as retracted BEFORE THEY EXIST, and
  // the events that later take those numbers would be born retracted —
  // invisible to every client, with nothing in the log explaining why. The
  // fold applies a marker's range unconditionally, so this cannot be caught
  // afterwards.
  let head = 0n;
  for (const e of log) if (e.sequence > head) head = e.sequence;
  if (to > head) {
    return `range extends past the last event (#${head}) — that would retract events before they happen`;
  }

  const retracted = retractedSet(log);
  const live = log.filter(
    (e) =>
      e.payload.case !== "eventsRetracted" &&
      !retracted.has(e.sequence) &&
      e.sequence >= from &&
      e.sequence <= to,
  );
  if (live.length === 0) {
    // Either the range is empty or everything in it is already gone. Both
    // would produce a retraction marker implying something changed when
    // nothing did.
    return "nothing live in that range — it is empty or already retracted";
  }
  return null;
}
