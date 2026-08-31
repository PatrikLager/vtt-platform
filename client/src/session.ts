// Session ties the wire to the fold: it accumulates the replayed log and
// derives state from it.
//
// # Why the whole stream is re-folded on every event
//
// Retraction used to be the answer: a marker invalidated history that had
// already been applied, and no incremental "apply this one event to the
// previous state" scheme could walk that back. Retraction has left the
// platform (Patrik, 2026-08-30) and the re-fold stays, because the reason
// underneath it did not go anywhere.
//
// fold's apply MUTATES the state it is handed. An incremental apply that
// throws halfway has already half-changed the board, and ingest's catch —
// which holds the LAST GOOD state rather than showing a plausible wrong one —
// would have nothing good left to hold. Re-folding from an empty state is
// what confines a failed fold's damage to a value nobody keeps. rollback
// rebuilds for a second reason: it TRUNCATES the log, and a state that has
// already been mutated forwards has no way to shrink except by being built
// again.
//
// The cost is re-folding a growing list. That is fine at table scale: a long
// session is thousands of events, not millions.

import { Wire, type WireStatus, type PresenceEvent } from "./wire";
import { fold } from "./fold";
import { newState, type State } from "./state";
import type { Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";

/** Someone currently at the table. */
export interface Participant {
  participantId: string;
  displayName: string;
}

export class Session {
  private readonly wire: Wire;
  private log: Envelope[] = [];
  private derived: State = newState();
  private changeHandlers: (() => void)[] = [];
  // Keyed by participant id, NOT a list: a client that reconnects receives a
  // fresh snapshot on top of whatever it already had, and the server announces
  // an arrival only on a participant's FIRST connection. Appending would list
  // the same person twice and never correct itself.
  private present = new Map<string, Participant>();
  private errorHandlers: ((e: Error) => void)[] = [];
  private presenceHandlers: ((batch: PresenceEvent[]) => void)[] = [];

  constructor(url: string, token: string) {
    this.wire = new Wire(url, token);
    this.wire.onEvent((e) => this.ingest(e));
    this.wire.onPresence((batch, replace) => this.presence(batch, replace));
    this.wire.onRollback((throughSeq) => this.rollback(throughSeq));
    this.wire.onRestart(() => this.empty());
  }

  get state(): State {
    return this.derived;
  }

  /**
   * Who is at the table, sorted by display name.
   *
   * Sorted rather than in arrival order: arrival order is a property of the
   * network, and a list that reshuffles whenever anyone joins or leaves is
   * hard to read and impossible to test stably. Ties break on participant id
   * so two people sharing a display name still have a fixed order.
   */
  get participants(): Participant[] {
    return [...this.present.values()].sort((a, b) =>
      a.displayName === b.displayName
        // Two arms, not three. A `? 1 : 0` tail would carry an UNREACHABLE
        // "equal" case: participantId is this map's KEY, so two entries can
        // never share one, and the mutants on that arm are unkillable by
        // construction. Same shape as the displayName comparison below.
        ? (a.participantId < b.participantId ? -1 : 1)
        : (a.displayName < b.displayName ? -1 : 1),
    );
  }

  get head(): bigint {
    return this.wire.head;
  }

  /**
   * The replayed log, in arrival order. The story feed and the event ticker
   * render from the LOG rather than from derived state, because state has no
   * memory of narration or of what happened in which order — that is the
   * whole point of keeping the log around after folding it.
   */
  get events(): readonly Envelope[] {
    return this.log;
  }

  onChange(fn: () => void): void {
    this.changeHandlers.push(fn);
  }

  /**
   * Raw presence batches, for a caller that needs more than the resulting
   * list.
   *
   * onChange cannot serve this: a promotion re-announces somebody who is
   * ALREADY present, so the participant list is unchanged and a listener
   * watching the list sees nothing at all. The batch is the only place that
   * frame is visible.
   */
  onPresence(fn: (batch: PresenceEvent[]) => void): void {
    this.presenceHandlers.push(fn);
  }

  onError(fn: (e: Error) => void): void {
    this.errorHandlers.push(fn);
  }

  onStatus(fn: (s: WireStatus) => void): void {
    this.wire.onStatus(fn);
  }

  /** Connect and replay the whole log from the beginning. */
  start(): Promise<void> {
    return this.wire.connect(0n);
  }

  /** Redial, resuming from the last sequence already folded. */
  reconnect(): Promise<void> {
    return this.wire.reconnect();
  }

  /**
   * Redial from the beginning, holding nothing — the spectator's redial.
   *
   * See Wire.restart for WHY a perched spectator cannot simply resume, and for
   * the measurements that ruled out the alternatives. What matters here is
   * that the emptying is not a rollback: rollback keeps everything at or below
   * its cursor and a perch frame carries sequence 0, so no cursor can reach
   * one. The log is dropped whole, by the handler this class registered.
   */
  restart(): Promise<void> {
    return this.wire.restart();
  }

  send(cmd: ClientCommand): Promise<CommandResult> {
    return this.wire.send(cmd);
  }

  close(): void {
    this.wire.close();
  }

  /**
   * Apply a presence batch and redraw.
   *
   * `replace` means this was a SNAPSHOT: the complete table, so whoever it
   * omits is no longer here. Clearing first is what makes a reconnect correct
   * — the server sends a snapshot on every connection, and anyone who left
   * while this client was disconnected appears in no frame it will ever
   * receive. Merging instead of replacing leaves them listed forever.
   *
   * onChange fires once per FRAME, including a frame that changes nothing.
   * Notifying unconditionally keeps this total: the alternative is diffing
   * before and after, and a view that misses a redraw shows a stale table,
   * which is worse than a redundant redraw. Presence frames are rare.
   */
  private presence(batch: PresenceEvent[], replace: boolean): void {
    if (replace) this.present.clear();
    for (const p of batch) {
      if (p.state === "CONNECTED") {
        this.present.set(p.participantId, {
          participantId: p.participantId,
          displayName: p.displayName,
        });
      } else {
        this.present.delete(p.participantId);
      }
    }
    for (const fn of this.presenceHandlers) fn(batch);
    for (const fn of this.changeHandlers) fn();
  }

  /**
   * Drop everything above throughSeq, because the server is about to send it
   * again.
   *
   * A reconnect resumes one sequence BEFORE the highest this client saw (see
   * wire.ts's replay-cursor note): one event is now several envelopes for a
   * projected seat, and a socket that dies mid-batch leaves this log holding
   * part of a sequence with no way to ask for the rest. Rolling that sequence
   * off and taking it whole again is what makes the resume point expressible.
   *
   * Truncating a log is always safe to FOLD — what remains is a prefix of
   * something that folded a moment ago — so this deliberately does not report
   * an error path it cannot reach. It re-folds rather than trusting the
   * previous state because there is nothing to trust: dropping envelopes off
   * the end of the log cannot be expressed as an edit to a state those
   * envelopes have already been applied to.
   */
  private rollback(throughSeq: bigint): void {
    const kept = this.log.filter((e) => e.sequence <= throughSeq);
    if (kept.length === this.log.length) return;
    this.log = kept;
    this.derived = fold(this.log);
    for (const fn of this.changeHandlers) fn();
  }

  /**
   * Drop the log entirely, because the whole of it is about to arrive again.
   *
   * NOT rollback(0n), which would keep the sequence-0 frames a perch produces
   * — 0 is at or below every cursor — and those are exactly the frames a
   * restart exists to be rid of: they came from a connection that no longer
   * exists, and the one being dialled will introduce their contents again.
   *
   * Notifies unconditionally, like presence: a view still painting the old
   * board while the replay is in flight is a board nobody is behind.
   */
  private empty(): void {
    this.log = [];
    this.derived = newState();
    for (const fn of this.changeHandlers) fn();
  }

  private ingest(e: Envelope): void {
    this.log.push(e);
    try {
      this.derived = fold(this.log);
    } catch (err) {
      // Deliberately NOT falling back to a partially-applied state: if the
      // log cannot be folded, the client does not know the true board, and
      // showing a plausible-looking wrong one invites a player to act on a
      // position that never existed. Report and hold the last good state.
      for (const fn of this.errorHandlers) {
        fn(err instanceof Error ? err : new Error(String(err)));
      }
      return;
    }
    for (const fn of this.changeHandlers) fn();
  }
}
