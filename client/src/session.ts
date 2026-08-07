// Session ties the wire to the fold: it accumulates the replayed log and
// derives state from it.
//
// # Why the whole stream is re-folded on every event
//
// Because retraction exists. An EventsRetracted marker invalidates events
// that were already applied, and no incremental "apply this one event to the
// previous state" scheme can walk that back — the retracted event's effects
// are already baked in. internal/harness/fold.go has the same shape for the
// same reason: two passes, collect retractions first, then apply.
//
// The cost is re-folding a growing list. That is fine at table scale (a long
// session is thousands of events, not millions) and the alternative is a
// client whose undo silently disagrees with the server's.

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
  private readonly log: Envelope[] = [];
  private derived: State = newState();
  private changeHandlers: (() => void)[] = [];
  // Keyed by participant id, NOT a list: a client that reconnects receives a
  // fresh snapshot on top of whatever it already had, and the server announces
  // an arrival only on a participant's FIRST connection. Appending would list
  // the same person twice and never correct itself.
  private present = new Map<string, Participant>();
  private errorHandlers: ((e: Error) => void)[] = [];

  constructor(url: string, token: string) {
    this.wire = new Wire(url, token);
    this.wire.onEvent((e) => this.ingest(e));
    this.wire.onPresence((batch, replace) => this.presence(batch, replace));
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
