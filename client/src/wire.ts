// The WebSocket client: connect, replay, send, reconnect.
//
// Correlate a result by its request_id and never by arrival order: results and
// events come from two producers on the gateway (ServerFrame in
// contract/vtt/v1/commands.proto). A connection holds one log at a time, and
// a redial names it, or until then the log handed in (SPEC-022, VTT-397, VTT-442).

import { create, fromJson, toJson } from "@bufbuild/protobuf";
import {
  ClientCommandSchema,
  CommandResultSchema,
  ServerFrameSchema,
  PresenceState,
  type ClientCommand,
  type CommandResult,
  type WatchableActor,
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
  // Record the socket beside each resolver: only that socket's close may
  // answer it, and the map outlives every socket (connect's close handler).
  private readonly pending = new Map<string, { ws: WebSocket; resolve: (r: CommandResult) => void }>();
  private eventHandlers: ((e: Envelope) => void)[] = [];
  private statusHandlers: ((s: WireStatus) => void)[] = [];
  private presenceHandlers: ((batch: PresenceEvent[], replace: boolean) => void)[] = [];
  private restartHandlers: (() => void)[] = [];
  private watchableHandlers: ((actors: WatchableActor[], controlsNoPartyMember: boolean) => void)[] = [];
  private viewpointHandlers: ((actorId: string) => void)[] = [];
  // null until a ViewpointChanged names one: the DM's and the agent's world.
  private log: string | null = null;
  private lastSeq = 0n;
  private nextID = 0;

  constructor(
    private readonly url: string,
    private readonly token: string,
    private readonly remembered: string | null = null,
  ) {}

  /** The highest sequence this wire has been sent of the log it holds. */
  get head(): bigint {
    return this.lastSeq;
  }

  /** The actor whose log this wire holds, "" for nobody, null for the world or before a ViewpointChanged. */
  get viewpoint(): string | null {
    return this.log;
  }

  onEvent(fn: (e: Envelope) => void): void {
    this.eventHandlers.push(fn);
  }

  onStatus(fn: (s: WireStatus) => void): void {
    this.statusHandlers.push(fn);
  }

  /**
   * Called when the log this wire holds starts again: everything folded from
   * it must go (VTT-394, VTT-395).
   */
  onRestart(fn: () => void): void {
    this.restartHandlers.push(fn);
  }

  /** Called with each set the gateway sends this connection (VTT-454, VTT-458). */
  onWatchable(fn: (actors: WatchableActor[], controlsNoPartyMember: boolean) => void): void {
    this.watchableHandlers.push(fn);
  }

  /** Called with the actor each ViewpointChanged names, "" for nobody (VTT-442). */
  onViewpoint(fn: (actorId: string) => void): void {
    this.viewpointHandlers.push(fn);
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

  /** Dial, naming the log held, or before any ViewpointChanged the one handed in, and the cursor. */
  connect(): Promise<void> {
    this.status("connecting");
    // Keep the token in the query: a browser sets no header on a WebSocket
    // handshake (internal/gateway/metadata.go reads the header elsewhere).
    let u = `${this.url}?token=${encodeURIComponent(this.token)}&after=${this.lastSeq}`;
    const log = this.log ?? this.remembered;
    if (log) u += `&log=${encodeURIComponent(log)}`;
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
        // Answer THIS socket's commands only: a later socket's are still
        // waiting on the wire's map, and a stale close must not end them.
        for (const [id, waiting] of this.pending) {
          if (waiting.ws !== ws) continue;
          this.pending.delete(id);
          waiting.resolve(create(CommandResultSchema, {
            requestId: id,
            ok: false,
            error: "wire: connection closed before a result arrived",
          }));
        }
      };
      // Keep the identity guard: close() cancels no message already queued,
      // and a frame from an abandoned socket must not be folded.
      ws.onmessage = (ev) => {
        if (this.ws !== ws) return;
        this.handleFrame(String(ev.data));
      };
    });
  }

  /** Redial, naming the log held, or before any ViewpointChanged the one handed in, and the cursor. */
  async reconnect(): Promise<void> {
    // Forget the socket before closing it: the message handler reads which
    // socket is current, and close() cancels no event already queued.
    const old = this.ws;
    this.ws = null;
    old?.close();
    await this.connect();
  }

  private reset(): void {
    this.lastSeq = 0n;
    for (const fn of this.restartHandlers) fn();
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
          waiting.resolve(res);
        }
        // MATCHED ON THE REQUEST ID ALONE, though the entry now also carries a
        // socket. Two things make that safe where the close handler's walk was
        // not: the message handler has already established that this frame came
        // from the CURRENT socket, and an id is minted per COMMAND — by
        // commands.ts's requestId(), a UUID with a monotonic fallback where
        // crypto is absent — so two entries do not collide.
        // And a result is an answer to a command: an answer is welcome
        // whichever connection carries it, where a close is only ever news
        // about one.
        //
        // An unmatched result is dropped, not an error: it can legitimately
        // belong to a command abandoned by a previous connection.
        return;
      }
      case "event": {
        const env = frame.frame.value;
        // Reset at sequence 1 alone: an entry at or below the cursor is a
        // re-send, never a restart (VTT-395).
        if (env.sequence === 1n && this.lastSeq > 0n) this.reset();
        if (env.sequence > this.lastSeq) this.lastSeq = env.sequence;
        for (const fn of this.eventHandlers) fn(env);
        return;
      }
      case "viewpointChanged": {
        const actorId = frame.frame.value.actorId;
        if (actorId !== this.log) {
          this.log = actorId;
          this.reset();
        }
        for (const fn of this.viewpointHandlers) fn(actorId);
        return;
      }
      case "watchableChanged": {
        for (const fn of this.watchableHandlers) fn(frame.frame.value.actors, frame.frame.value.controlsNoPartyMember);
        return;
      }
      case "presenceSnapshot": {
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
      // THE SOCKET IT IS ACTUALLY WRITTEN TO, recorded beside the resolver
      // rather than read back off `this.ws` later: by the time a close comes to
      // answer this entry, `this.ws` may be a socket that has never heard of
      // this command. Only the close of THIS socket may end it — see the close
      // handler in connect() for what a close of any other one cost.
      this.pending.set(requestId, { ws, resolve });
      ws.send(JSON.stringify(toJson(ClientCommandSchema, withID)));
    });
  }
}

/** Re-exported so callers need not reach into the generated module. */
export type { Envelope, ClientCommand, CommandResult, WatchableActor };
export { EnvelopeSchema };
