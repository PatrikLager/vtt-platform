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

import { Wire, type WireStatus } from "./wire";
import { fold } from "./fold";
import { newState, type State } from "./state";
import type { Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";

export class Session {
  private readonly wire: Wire;
  private readonly log: Envelope[] = [];
  private derived: State = newState();
  private changeHandlers: (() => void)[] = [];
  private errorHandlers: ((e: Error) => void)[] = [];

  constructor(url: string, token: string) {
    this.wire = new Wire(url, token);
    this.wire.onEvent((e) => this.ingest(e));
  }

  get state(): State {
    return this.derived;
  }

  get head(): bigint {
    return this.wire.head;
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
