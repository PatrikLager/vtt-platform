// The WebSocket client: connect, replay, send, reconnect.
//
// # Frames interleave — correlate, never assume order
//
// The gateway does NOT order a CommandResult against the Envelopes that same
// command produced. Results are enqueued by the command loop and events by
// the broadcast pump: two independent producers feeding one writer, and
// either can win (see ServerFrame's contract in contract/vtt/v1/commands.proto,
// and serve()'s "Writer choice" comment in internal/gateway/server.go).
//
// That is not theoretical — the gateway's own tests read positionally for
// months and it cost two distinct CI failure modes: a batch of events
// outrunning a result, and one inversion desynchronising a connection
// permanently. So send() resolves against a pending map keyed by request_id,
// and events are delivered as they arrive regardless of what is in flight.
//
// # The replay cursor
//
// First connect asks for after=0 and folds the whole log. Resuming from 0
// instead would replay events the client has already folded, duplicating them
// into a state that disagrees with the server's.
//
// TWO CURSORS ADVANCE ON EVERY EVENT AND ONLY ONE OF THEM IS THE RESUME
// POINT. `seenSeq` is the HIGH-WATER MARK, the highest sequence ever
// delivered, and it only ever grows. `lastSeq` is where the next redial
// resumes from, and a redial sets it BELOW the high-water mark — so it is the
// one number here that can go down, and it is never the number to derive the
// next resume point from.
//
// A RECONNECT ASKS FOR after=<seenSeq - 1>, AND ROLLS THE LAST SEQUENCE BACK
// OUT OF THE LOG FIRST. Deriving that from lastSeq instead is the regression
// this pair of cursors exists to prevent: after one redial lastSeq already IS
// seenSeq - 1, so the next redial would step back again, and the board rewinds
// one event per click of the Reconnect button — which is exactly when that
// button is on screen. One event is now several envelopes.
//
// Since the visibility projection landed (internal/gateway/project.go), a
// player's or a spectator's stream carries SYNTHESIZED envelopes stamped with
// the sequence of the event that caused them: something coming into view is an
// ActorAdded plus a TokenPlaced plus a SceneSeen, all at sequence N. So
// several envelopes share one sequence, and the server resumes STRICTLY after
// a sequence (internal/store/subscribe.go: `seq > afterSeq`).
//
// That makes a cursor of exactly the high-water mark unusable, because it
// reaches N on the FIRST envelope of the batch while the rest are still in
// flight. A socket
// that dies after 2 of 5 leaves this client holding two thirds of a sequence
// and no way to say so: after=N discards the three it never got — and a lost
// TokenHidden leaves an enemy token on the board, which is the leak the whole
// arc exists to close — while re-sending the two it already folded is a
// duplicate ActorAdded, which fold() rejects and Session turns into a
// permanent freeze.
//
// Resuming one sequence EARLIER makes the resume point expressible again. A
// sequence lower than the highest one seen is provably complete: envelopes
// arrive in order, so observing sequence N is proof that N-1's batch was
// finished. The possibly-torn tail is dropped from the log (onRollback, which
// Session uses to truncate and re-fold) and the server re-sends it whole. The
// cost is one sequence re-sent per reconnect; the alternative has no correct
// answer at all.

import { create, fromJson, toJson } from "@bufbuild/protobuf";
import {
  ClientCommandSchema,
  CommandResultSchema,
  ServerFrameSchema,
  PresenceState,
  type ClientCommand,
  type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";

export type WireStatus = "connecting" | "open" | "closed" | "error";

/**
 * One participant's presence, flattened out of the two wire frames that carry
 * it — a snapshot entry and a delta have the same shape and the same meaning,
 * so a consumer needs one handler, not two.
 *
 * `state` is narrowed to the two states that can appear: UNSPECIFIED exists on
 * the wire only to catch a sender that forgot to set it, and is dropped here
 * rather than passed on as a third case every consumer must then handle.
 */
export interface PresenceEvent {
  participantId: string;
  displayName: string;
  state: "CONNECTED" | "DISCONNECTED";
}

export class Wire {
  private ws: WebSocket | null = null;
  private readonly pending = new Map<string, (r: CommandResult) => void>();
  private eventHandlers: ((e: Envelope) => void)[] = [];
  private statusHandlers: ((s: WireStatus) => void)[] = [];
  private presenceHandlers: ((batch: PresenceEvent[], replace: boolean) => void)[] = [];
  private rollbackHandlers: ((throughSeq: bigint) => void)[] = [];
  // TWO CURSORS, because they answer different questions and sharing one made
  // a reconnect rewind the board — see the replay-cursor note at the top of
  // this file, which names seenSeq as the one a resume point is derived from.
  // Deriving from seenSeq each time makes repeated redials idempotent;
  // deriving from lastSeq, which a redial has already moved down, steps back
  // again on every click.
  private seenSeq = 0n;
  private lastSeq = 0n;
  private nextID = 0;

  constructor(
    private readonly url: string,
    private readonly token: string,
  ) {}

  /** The sequence this client's next redial would resume after. */
  get head(): bigint {
    return this.lastSeq;
  }

  onEvent(fn: (e: Envelope) => void): void {
    this.eventHandlers.push(fn);
  }

  onStatus(fn: (s: WireStatus) => void): void {
    this.statusHandlers.push(fn);
  }

  /**
   * Called before a reconnect with the sequence the redial will resume AFTER:
   * everything above it is about to be sent again and must be dropped first.
   *
   * A handler is required for correctness, not a convenience. The redial asks
   * for one sequence earlier than the highest seen (see the replay-cursor note
   * at the top of this file), so a holder of the log that does not truncate
   * would fold the re-sent batch twice — which is the duplicate introduction
   * the server's fold and this client's both refuse.
   */
  onRollback(fn: (throughSeq: bigint) => void): void {
    this.rollbackHandlers.push(fn);
  }

  /**
   * Subscribe to presence.
   *
   * `replace` distinguishes a SNAPSHOT from a delta, and that distinction is
   * load-bearing rather than informational. A snapshot is the complete table,
   * so its meaning is as much in who is ABSENT as in who is listed; the server
   * sends one on every connection, reconnects included. Flattening it into
   * individual arrivals discards the absence, and a client that reconnects
   * after someone left keeps showing them forever — the ghost this whole
   * feature exists to prevent.
   */
  onPresence(fn: (batch: PresenceEvent[], replace: boolean) => void): void {
    this.presenceHandlers.push(fn);
  }

  /** Narrow one wire record, or drop it. */
  private one(participantId: string, displayName: string, state: PresenceState): PresenceEvent | null {
    // UNSPECIFIED is dropped, not guessed. Treating it as CONNECTED would add
    // a phantom to the table and treating it as DISCONNECTED would evict a
    // real one; both are worse than ignoring a frame a correct server never
    // sends.
    if (state === PresenceState.CONNECTED) return { participantId, displayName, state: "CONNECTED" };
    if (state === PresenceState.DISCONNECTED) return { participantId, displayName, state: "DISCONNECTED" };
    return null;
  }

  private emitPresence(batch: PresenceEvent[], replace: boolean): void {
    for (const fn of this.presenceHandlers) fn(batch, replace);
  }

  private status(s: WireStatus): void {
    for (const fn of this.statusHandlers) fn(s);
  }

  connect(after: bigint): Promise<void> {
    this.status("connecting");
    // The token is a query parameter here and a Bearer header on the metadata
    // routes. Not an inconsistency: browsers cannot set headers on a
    // WebSocket handshake, so the WS route has no alternative. The HTTP
    // routes, which do, use the header (see internal/gateway/metadata.go).
    const u = `${this.url}?token=${encodeURIComponent(this.token)}&after=${after}`;
    const ws = new WebSocket(u);
    this.ws = ws;

    return new Promise((resolve, reject) => {
      ws.onopen = () => {
        this.status("open");
        resolve();
      };
      ws.onerror = () => {
        this.status("error");
        reject(new Error("wire: connection failed"));
      };
      ws.onclose = () => {
        this.status("closed");
        // Anything still waiting will never be answered on this socket.
        // Rejecting is kinder than a promise that hangs for the session.
        for (const [id, resolveFn] of this.pending) {
          resolveFn(create(CommandResultSchema, {
            requestId: id,
            ok: false,
            error: "wire: connection closed before a result arrived",
          }));
        }
        this.pending.clear();
      };
      // GUARDED ON IDENTITY, and this one line is the whole fix. The handler
      // is bound per socket but knows nothing about which socket it belongs
      // to, and close() does not cancel a `message` event already queued — so
      // an envelope from an abandoned socket would be folded AFTER a rollback
      // had truncated the log, re-adding the very sequence the redial is about
      // to send again. Measured before the fix as log 1,2,2 and
      // `duplicate actor "a1"`: the permanent freeze this design exists to
      // avoid.
      //
      // It covers BOTH ways a socket is abandoned, which is why it is here
      // rather than in reconnect(): a redial forgets its socket explicitly,
      // and a bare second connect() simply overwrites this.ws and leaves the
      // first one live with its handlers still bound. Each has its own test.
      ws.onmessage = (ev) => {
        if (this.ws !== ws) return;
        this.handleFrame(String(ev.data));
      };
    });
  }

  /**
   * Redial from one sequence BELOW the high-water mark, having first told
   * whoever holds the log to drop that last sequence — see the replay-cursor
   * note at the top of this file for why a cursor of exactly the highest
   * sequence seen cannot be expressed once one event can be several
   * envelopes, and why the step back is taken from seenSeq rather than from
   * lastSeq.
   *
   * READ THIS BEFORE WIRING A SPECTATOR PERCH (visibility spec §3.1.1). A perch
   * is CONNECTION state on the server, like the catch-up cursor: a redial
   * forgets it, and the client is what re-sends it. Doing that naively breaks
   * the client permanently, so the rule is worth stating before anyone writes
   * the obvious three lines:
   *
   *   - a redial resumes at seenSeq-1 and this client KEEPS its folded log, but
   *     the server's projector is reborn empty and — perched on nobody — sends
   *     nothing at all during the replay. Re-sending the perch on that
   *     connection therefore re-introduces a scene, actors and tokens this
   *     client is still holding, and a duplicate introduction is a fold throw,
   *     which freezes state for good (session.ts re-folds the whole log on
   *     every event);
   *   - perch frames carry SEQUENCE 0, deliberately, so that no undo can ever
   *     name one (see the server's perchSequence). One consequence lands here:
   *     `session.ts`'s rollback keeps everything at or below the cursor, and 0
   *     is below every cursor, so perch frames SURVIVE a rollback that drops
   *     ordinary ones.
   *
   * So re-perch on a connection that resumed from 0 with an empty log, or drop
   * the sequence-0 frames from the log before re-sending. Do not simply call
   * setViewpoint again after reconnect().
   */
  async reconnect(): Promise<void> {
    // Abandon the old socket by forgetting it, then close it. Forgetting is
    // what the message handler's identity check reads; closing is only how the
    // socket is disposed of, and it does NOT stop an event already queued on
    // it. Between the two assignments this wire has no socket, which is what
    // send() reports.
    const old = this.ws;
    this.ws = null;
    old?.close();
    const resume = this.seenSeq > 0n ? this.seenSeq - 1n : 0n;
    this.lastSeq = resume;
    for (const fn of this.rollbackHandlers) fn(resume);
    await this.connect(resume);
  }

  close(): void {
    this.ws?.close();
  }

  private handleFrame(raw: string): void {
    const frame = fromJson(ServerFrameSchema, JSON.parse(raw));
    switch (frame.frame.case) {
      case "result": {
        const res = frame.frame.value;
        const waiting = this.pending.get(res.requestId);
        if (waiting) {
          this.pending.delete(res.requestId);
          waiting(res);
        }
        // An unmatched result is dropped, not an error: it can legitimately
        // belong to a command abandoned by a previous connection.
        return;
      }
      case "event": {
        const env = frame.frame.value;
        if (env.sequence > this.seenSeq) this.seenSeq = env.sequence;
        if (env.sequence > this.lastSeq) this.lastSeq = env.sequence;
        for (const fn of this.eventHandlers) fn(env);
        return;
      }
      case "presenceSnapshot": {
        // Deliberately NOT advancing either cursor: presence is not an event
        // and carries no sequence. They are what a reconnect resumes from, so
        // touching them here would skip real events on the next redial.
        const batch: PresenceEvent[] = [];
        for (const e of frame.frame.value.present) {
          const one = this.one(e.participantId, e.displayName, e.state);
          if (one) batch.push(one);
        }
        // replace=true even when the batch is EMPTY: a table that emptied is a
        // fact, and skipping the call would leave the previous list standing.
        this.emitPresence(batch, true);
        return;
      }
      case "presenceChanged": {
        const p = frame.frame.value;
        const one = this.one(p.participantId, p.displayName, p.state);
        if (one) this.emitPresence([one], false);
        return;
      }
      default:
        // Unknown frame kinds are ignored rather than fatal, matching the
        // forward-compatibility the server's own replay gives.
        return;
    }
  }

  send(cmd: ClientCommand): Promise<CommandResult> {
    const ws = this.ws;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return Promise.resolve(create(CommandResultSchema, {
        ok: false,
        error: "wire: not connected",
      }));
    }
    // A command with no request_id yields a result nothing can be matched to,
    // so send() would never resolve. Assign one rather than letting the
    // caller's omission hang the UI.
    const requestId = cmd.requestId !== "" ? cmd.requestId : `c-${++this.nextID}`;
    const withID = create(ClientCommandSchema, { ...cmd, requestId });

    return new Promise((resolve) => {
      this.pending.set(requestId, resolve);
      ws.send(JSON.stringify(toJson(ClientCommandSchema, withID)));
    });
  }
}

/** Re-exported so callers need not reach into the generated module. */
export type { Envelope, ClientCommand, CommandResult };
export { EnvelopeSchema };
