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
// That makes a cursor of exactly the high-water mark unusable,
// because it reaches N on the FIRST envelope of the batch while
// the rest are still in flight. A socket that dies after 2 of 5
// leaves this client holding two thirds of a sequence and no
// way to say so: after=N discards the three it never got — and
// a lost TokenHidden leaves an enemy token on the board, which
// is the leak the whole arc exists to close — while re-sending
// the two it already folded is a duplicate ActorAdded, which
// fold() rejects and Session turns into a permanent freeze.
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
  // ONE MAP, MANY SOCKETS — so every entry records the socket it was written
  // to. A wire outlives its connections and this map outlives them with it, so
  // "who is waiting for a result" and "which connection is that result coming
  // back on" are two questions, and only the resolver answered the first. The
  // close handler in connect() is where the second one is read, and why.
  //
  // WHAT THE NARROWING COSTS, stated rather than glossed. reconnect() and
  // restart() both CLOSE the socket they forget, so anything sent on it still
  // has a close of its own coming and is still answered. A bare second
  // connect() does not — it leaves the previous socket live (see the message
  // handler's own note) — so an entry made on THAT socket now waits for a close
  // that may never be dispatched, where before it was ended early by the next
  // close of any socket at all. Neither is a result; the trade is one caller
  // waiting on a socket nobody closed against every caller being told their
  // command failed when it did not. No path in this client reaches it: the app
  // dials once and redials only through reconnect() or restart().
  private readonly pending = new Map<string, { ws: WebSocket; resolve: (r: CommandResult) => void }>();
  private eventHandlers: ((e: Envelope) => void)[] = [];
  private statusHandlers: ((s: WireStatus) => void)[] = [];
  private presenceHandlers: ((batch: PresenceEvent[], replace: boolean) => void)[] = [];
  private rollbackHandlers: ((throughSeq: bigint) => void)[] = [];
  private restartHandlers: (() => void)[] = [];
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

  /** How far this client has folded — lastSeq.
   *
   *  NOT the redial resume point, which reconnect() derives from seenSeq and
   *  which is a DIFFERENT number once one event can arrive as several
   *  envelopes (see the replay-cursor note at the top of this file). This doc
   *  said "the sequence this client's next redial would resume after" until
   *  2026-08-25, which is true only in the moment just after a redial.
   *
   *  Exported for observation. Nothing in client/src reads it today — the only
   *  callers are tests, through Session.head. That is worth knowing before
   *  reasoning about what a change here can break.
   */
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
   * Called before a restart: this client is about to be sent the whole log
   * again, so whatever it holds must go.
   *
   * A SEPARATE CHANNEL FROM onRollback, and it cannot be expressed as one.
   * Rollback keeps everything at or below its cursor, and a perch frame
   * carries sequence 0 — below every cursor there is — so no rollback can ever
   * drop one. Emptying is a different instruction, not a rollback with a
   * smaller number.
   */
  onRestart(fn: () => void): void {
    this.restartHandlers.push(fn);
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

  /**
   * Dial, replaying after `after`.
   *
   * `onOpen` runs INSIDE the socket's open event, before this promise resolves
   * and before any frame can be delivered. That window is not available to a
   * caller awaiting the promise — measured, and the measurement is the reason
   * this parameter exists: emptying the log in `await connect(...)`'s
   * continuation instead let the server's replay arrive FIRST, so the log was
   * folded and then thrown away, leaving a client holding nothing with no
   * frames left to come (client/test/session.test.ts's restart test, which
   * timed out waiting for a replay it had already discarded). `open` is
   * dispatched ahead of the first `message` for the same socket, so a hook
   * here has no such window — and a dial that never opens never runs it, which
   * is what lets Wire.restart leave a failed redial costing nothing.
   */
  connect(after: bigint, onOpen?: () => void): Promise<void> {
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
        // FIRST, ahead of the status handler as well as of the resolve: status
        // "open" repaints, and a repaint between a restart's dial and its
        // discard would draw the board this connection is about to replace.
        onOpen?.();
        this.status("open");
        resolve();
      };
      ws.onerror = () => {
        this.status("error");
        reject(new Error("wire: connection failed"));
      };
      ws.onclose = () => {
        this.status("closed");
        // THIS SOCKET'S COMMANDS ONLY, and the identity check is the whole of
        // the difference. Anything written to THIS socket will never be
        // answered on it now, so failing it explicitly is kinder than a promise
        // that hangs for the rest of the session — that half has always been
        // right. What was wrong is that the map belongs to the WIRE, not to the
        // socket, so walking all of it let a close end commands a LATER socket
        // was still waiting on.
        //
        // Its sibling below was given the same guard for the same reason
        // (45ae70d), and until this one had it too the asymmetry was the
        // defect: a stale frame could not reach the fold, but a stale close
        // could end a live command — with a failure the server never sent.
        //
        // NARROW THROUGH THE UI TODAY, and stated that way because the honest
        // version is the useful one — the same shape as the message handler's
        // note below. The Reconnect button appears only on status "closed"
        // (view/spectator.ts), and that status is set at the top of this very
        // handler, in the same task that walks this map — so in the shipped
        // client the sockets are strictly serial and no close has ever run with
        // another socket's commands in front of it. What makes it worth
        // guarding anyway is that connect(), reconnect() and restart() are
        // public API and restart() is close-then-send by construction: it
        // closes the old socket, dials, and its caller re-sends on the new one.
        // Any automatic redial, or any embedder holding two connections, puts a
        // live command in this loop's path — and what it would lose is a
        // setViewpoint the server is about to ACCEPT, leaving app.ts reading a
        // refusal nobody sent while the perch frames arrive and are folded
        // anyway. client/test/app.test.ts's "a redial whose abandoned socket
        // closes late still ends up perch-shaped" walks the whole of that.
        //
        // Deleted entry by entry rather than cleared, which is the same rule
        // stated for the map instead of for the promises. Clearing takes the
        // newer socket's entries with it, so the result the server sends for
        // one of them arrives as an unknown request id and is dropped — and a
        // command with no result and no failure is the hang the loop below
        // exists to prevent.
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
   * SO A CLIENT WHOSE LOG A PERCH HAS SHAPED MUST NOT RESUME — not with a
   * re-perch, not without one. Use restart(), which dials after=0 and empties
   * the log, and re-send the shoulder yourself afterwards, the way app.ts's
   * redial() does. restart() itself sends nothing — it dials, and the re-perch
   * is the caller's line.
   *
   * restart()'s table below records all four candidates run against the real
   * gateway, with the outcome each produced. Read them there, not here.
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

  /**
   * Dial from the very beginning, throwing away everything this client holds.
   *
   * THIS IS THE SPECTATOR'S REDIAL, and it exists because reconnect() cannot
   * be one. A perch is CONNECTION state (visibility spec §3.1.1): the server's
   * projector is reborn perched on nobody, and it replays the log through
   * those empty eyes — so it holds no memory of the scene, the actors or the
   * tokens the previous connection's perch put on this client's board. Nothing
   * on the wire un-introduces any of them, and both folds refuse a second
   * introduction, so a resumed connection and this client disagree about what
   * has been said and the next perch is a duplicate.
   *
   * MEASURED AGAINST THE REAL GATEWAY (2026-08-24), on a watcher who had
   * perched and then seen a goblin walk into view. All four of the available
   * answers, so that the dead ones are not re-tried:
   *
   *   - resume, then re-perch: `engine: scene "ambush" already exists`. The
   *     duplicate-introduction freeze — session.ts re-folds its whole log on
   *     every event, so it is permanent.
   *   - resume, drop the sequence-0 frames, then re-perch: fails EARLIER, with
   *     `engine: token placed in unknown scene "ambush"`, and before the
   *     re-perch is even sent. The frames an ordinary event delivered to a
   *     perched seat — the goblin's arrival — are stamped with the CAUSING
   *     sequence, not with 0, so they survive the rollback while depending on
   *     a scene only the perch ever introduced. Dropping the perch frames
   *     breaks the log that is left behind.
   *   - resume and do not re-perch: folds, and the board is dead. The reborn
   *     projector sends this seat nothing at all, and the sequence the
   *     rollback dropped is never re-sent either, so the watcher is left
   *     looking at a stale room forever.
   *   - THIS: dial after=0 holding nothing, then re-perch. Folds cleanly, and
   *     is the only one of the four that also shows a live board.
   *
   * The cost is one full replay for a watcher who may not have needed it. That
   * is the honest price of a view preference the log deliberately does not
   * record, and it is paid by the one role whose board is otherwise a lie.
   *
   * BOTH CURSORS GO BACK TO ZERO, which is not tidiness: seenSeq is what the
   * next redial derives its resume point from, and a stale mark over an
   * emptied log would ask for events after a sequence this client never
   * folded — silently skipping everything between, which is the direction that
   * leaves an enemy token on a board.
   */
  async restart(): Promise<void> {
    // Same order as reconnect(): forget the socket before closing it, because
    // forgetting is what the message handler's identity check reads and
    // closing does not cancel an event already queued on it. A frame from the
    // abandoned socket would otherwise be folded into a log that is about to
    // be emptied and re-sent whole.
    const old = this.ws;
    this.ws = null;
    old?.close();

    // NOTHING IS DISCARDED UNTIL THE NEW SOCKET IS OPEN, which is the one
    // place this deliberately does NOT follow reconnect(). Reconnect throws
    // away one sequence up front, and a failed redial costs that sequence;
    // this throws away EVERYTHING, and doing it before a dial that then fails
    // would leave a watcher staring at a blank page with no board to go back
    // to. A refused dial is a no-op here — log and cursors intact — because
    // the hook below never runs.
    //
    // IN THE OPEN EVENT rather than after awaiting it, and connect's own
    // comment records what awaiting cost: the replay arrived before the
    // continuation, so the log was folded and then discarded, and nothing was
    // left to fold again.
    await this.connect(0n, () => {
      this.seenSeq = 0n;
      this.lastSeq = 0n;
      for (const fn of this.restartHandlers) fn();
    });
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
export type { Envelope, ClientCommand, CommandResult };
export { EnvelopeSchema };
