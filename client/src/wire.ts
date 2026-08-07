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
// First connect asks for after=0 and folds the whole log. Every event seen
// advances lastSeq, and a reconnect asks for after=<lastSeq>. Resuming from 0
// instead would replay events the client has already folded, duplicating them
// into a state that disagrees with the server's.

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
  private lastSeq = 0n;
  private nextID = 0;

  constructor(
    private readonly url: string,
    private readonly token: string,
  ) {}

  /** The highest sequence this client has observed. */
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
      ws.onmessage = (ev) => this.handleFrame(String(ev.data));
    });
  }

  /** Redial from the last sequence seen, so replay does not repeat history. */
  async reconnect(): Promise<void> {
    this.ws?.close();
    await this.connect(this.lastSeq);
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
        if (env.sequence > this.lastSeq) this.lastSeq = env.sequence;
        for (const fn of this.eventHandlers) fn(env);
        return;
      }
      case "presenceSnapshot": {
        // Deliberately NOT advancing lastSeq: presence is not an event and
        // carries no sequence. lastSeq is the replay cursor reconnect resumes
        // from, so touching it here would skip real events on the next redial.
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
