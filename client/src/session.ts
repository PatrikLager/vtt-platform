// Session ties the wire to the fold: it accumulates the log it is sent and
// derives state from it.
//
// Re-fold the whole log on every event: fold's apply mutates the state it is
// handed, so an incremental apply that throws would leave ingest no last good
// state to hold.

import { Wire, type WireStatus, type PresenceEvent, type WatchableActor } from "./wire";
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
  private fail = "";
  private set: WatchableActor[] = [];
  private none = false;
  private errorHandlers: ((e: Error) => void)[] = [];
  private presenceHandlers: ((batch: PresenceEvent[]) => void)[] = [];

  constructor(url: string, token: string, remembered: string | null = null) {
    this.wire = new Wire(url, token, remembered);
    this.wire.onEvent((e) => this.ingest(e));
    this.wire.onPresence((batch, replace) => this.presence(batch, replace));
    this.wire.onRestart(() => this.empty());
    this.wire.onWatchable((actors, none) => {
      this.set = actors;
      this.none = none;
      for (const fn of this.changeHandlers) fn();
    });
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

  /** The actor the last ViewpointChanged named, "" for nobody, the world or before one (VTT-407). */
  get viewpoint(): string {
    return this.wire.viewpoint ?? "";
  }

  /** The party members the gateway last said this connection may watch; a reset keeps them (VTT-457). */
  get watchable(): readonly WatchableActor[] {
    return this.set;
  }

  /** Whether the last set said this participant controls no party member; a reset keeps it (VTT-458). */
  get controlsNoPartyMember(): boolean {
    return this.none;
  }

  /** Why the log this session holds cannot be folded, or "" (VTT-393). */
  get failure(): string {
    return this.fail;
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

  /** Called with the actor each ViewpointChanged names, "" for nobody (VTT-442). */
  onViewpoint(fn: (actorId: string) => void): void {
    this.wire.onViewpoint(fn);
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

  /** Connect, and be sent the log from its first entry. */
  start(): Promise<void> {
    return this.wire.connect();
  }

  /** Redial, naming the log held, or before any ViewpointChanged the one handed in, and the cursor (VTT-397, VTT-398, VTT-442). */
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
    for (const fn of this.presenceHandlers) fn(batch);
    for (const fn of this.changeHandlers) fn();
  }

  // Notify unconditionally: a view still painting the old board while the new
  // log is in flight is a board nobody is behind.
  private empty(): void {
    this.log = [];
    this.derived = newState();
    this.fail = "";
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
      const e = err instanceof Error ? err : new Error(String(err));
      this.fail = e.message;
      for (const fn of this.errorHandlers) fn(e);
      return;
    }
    for (const fn of this.changeHandlers) fn();
  }
}
