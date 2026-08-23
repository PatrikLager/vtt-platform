import { test, expect } from "bun:test";
import { create, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  SessionStartedSchema,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { Wire } from "../src/wire";

/**
 * Wait for a CONDITION rather than a fixed delay.
 *
 * These were `await Bun.sleep(20)`, a guess at how long a real WebSocket takes
 * to deliver a frame. session.test.ts had the same guess at 30ms and it failed
 * 3 of 4 under the load of a mutation run — and because Stryker refuses to
 * start when its initial test run fails, a timing guess in a test file blocks
 * the whole mutation gate. Polling returns as soon as the frame lands and
 * treats the timeout as a backstop instead of a measurement.
 */
async function until(ready: () => boolean, what: string, timeoutMs = 2000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (ready()) return;
    await Bun.sleep(1);
  }
  throw new Error(`timed out after ${timeoutMs}ms waiting for ${what}`);
}


// A fake gateway. Records every connection's query string so the tests can
// assert the replay cursor, and lets each case script what it sends back.
function fakeGateway(handler: (ws: any, raw: string) => void) {
  const queries: string[] = [];
  const sockets: any[] = [];
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      queries.push(new URL(req.url).search);
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open(ws) {
        sockets.push(ws);
      },
      message(ws, raw) {
        handler(ws, String(raw));
      },
    },
  });
  return {
    url: `ws://localhost:${server.port}/ws`,
    queries,
    sockets,
    stop: () => server.stop(true),
  };
}

function envelopeJSON(seq: number) {
  return toJson(
    EnvelopeSchema,
    create(EnvelopeSchema, {
      eventId: `evt-${seq}`,
      sequence: BigInt(seq),
      payload: { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) },
    }),
  );
}

test("connect replays from after=0 and delivers events", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect(0n);

    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(1) }));
    await until(() => seen.length > 0, "the event to arrive");

    expect(seen).toEqual([1n]);
    expect(gw.queries[0]).toContain("after=0");
    expect(gw.queries[0]).toContain("token=tok-1");
    wire.close();
  } finally {
    gw.stop();
  }
});

test("a reconnect resumes one sequence below the last one seen, and says so", async () => {
  // Resuming from 0 would replay the whole log into a client that already
  // folded it — duplicated events, and a state that disagrees with the server.
  //
  // Resuming from exactly 7 is the OTHER wrong answer, and the one that only
  // became wrong when the gateway started projecting per seat: several
  // envelopes can now share a sequence, the cursor reaches 7 on the first of
  // them, and the server resumes strictly ABOVE a sequence — so a socket that
  // died mid-batch would never be sent the rest. Resuming from 6 and taking
  // sequence 7 again is the only expressible answer, and it is safe precisely
  // because the tail is rolled back first.
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const rolledBackThrough: bigint[] = [];
    wire.onRollback((seq) => rolledBackThrough.push(seq));
    await wire.connect(0n);
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(7) }));
    await until(() => wire.head === 7n, "the cursor to advance to 7");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");

    expect(gw.queries.length).toBe(2);
    expect(gw.queries[1]).toContain("after=6");
    // The rollback is announced BEFORE the redial and names the same cursor,
    // so whoever holds the log drops sequence 7 rather than folding it twice.
    expect(rolledBackThrough).toEqual([6n]);
    expect(wire.head).toBe(6n);
    wire.close();
  } finally {
    gw.stop();
  }
});

test("a reconnect before anything was ever seen still resumes from zero", async () => {
  // lastSeq - 1 must not go negative: a client that dropped before its first
  // event has nothing folded, and asking for after=-1 would be a 400 from
  // parseAfter's own caller rather than a replay.
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const rolledBackThrough: bigint[] = [];
    wire.onRollback((seq) => rolledBackThrough.push(seq));
    await wire.connect(0n);

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");

    expect(gw.queries[1]).toContain("after=0");
    expect(rolledBackThrough).toEqual([0n]);
    wire.close();
  } finally {
    gw.stop();
  }
});

test("send resolves even when the command's EVENT arrives before its result", async () => {
  // The gateway does not order a CommandResult against the events that
  // command produced — see ServerFrame's contract in the proto. A client that
  // correlated by arrival order would hang here forever.
  const gw = fakeGateway((ws, raw) => {
    const cmd = JSON.parse(raw);
    ws.send(JSON.stringify({ event: envelopeJSON(1) })); // event FIRST
    ws.send(JSON.stringify({ result: { requestId: cmd.requestId, ok: true, sequence: "1" } }));
  });
  try {
    const wire = new Wire(gw.url, "tok-1");
    const events: bigint[] = [];
    wire.onEvent((e) => events.push(e.sequence));
    await wire.connect(0n);

    const res = await wire.send(create(ClientCommandSchema, {
      command: { case: "startSession", value: { name: "S" } },
    }));

    expect(res.ok).toBe(true);
    expect(events).toEqual([1n]); // the event was NOT swallowed while waiting
    wire.close();
  } finally {
    gw.stop();
  }
});

test("two in-flight commands resolve to their own results, not each other's", async () => {
  // Correlation is by request_id. Answering out of order is legal, and a
  // client that matched positionally would hand a player the wrong outcome.
  const pending: string[] = [];
  const gw = fakeGateway((ws, raw) => {
    pending.push(JSON.parse(raw).requestId);
    if (pending.length === 2) {
      // Reply in REVERSE order.
      ws.send(JSON.stringify({ result: { requestId: pending[1], ok: true, sequence: "2" } }));
      ws.send(JSON.stringify({ result: { requestId: pending[0], ok: false, error: "denied" } }));
    }
  });
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect(0n);

    const first = wire.send(create(ClientCommandSchema, {
      command: { case: "startSession", value: { name: "first" } },
    }));
    const second = wire.send(create(ClientCommandSchema, {
      command: { case: "startSession", value: { name: "second" } },
    }));

    const [a, b] = await Promise.all([first, second]);
    expect(a.ok).toBe(false);
    expect(a.error).toBe("denied");
    expect(b.ok).toBe(true);
    wire.close();
  } finally {
    gw.stop();
  }
});

test("every command is given a request id when the caller omits one", async () => {
  // Without one the result is uncorrelatable and send could never resolve.
  let seenID = "";
  const gw = fakeGateway((ws, raw) => {
    seenID = JSON.parse(raw).requestId ?? "";
    ws.send(JSON.stringify({ result: { requestId: seenID, ok: true } }));
  });
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect(0n);
    await wire.send(create(ClientCommandSchema, {
      command: { case: "startSession", value: { name: "S" } },
    }));
    expect(seenID).not.toBe("");
    wire.close();
  } finally {
    gw.stop();
  }
});

test("status transitions are reported to the caller", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const statuses: string[] = [];
    wire.onStatus((s) => statuses.push(s));
    await wire.connect(0n);
    wire.close();
    await until(() => statuses.includes("closed"), "the closed status");
    expect(statuses).toContain("connecting");
    expect(statuses).toContain("open");
    expect(statuses).toContain("closed");
  } finally {
    gw.stop();
  }
});

// --- failure paths and the guards nothing was exercising ---------------------

test("a connection that cannot be made rejects with a reason, and reports error status", async () => {
  // Port 1 is not listenable by a normal process, so the handshake fails
  // rather than hanging. Both halves matter: the promise must reject so the
  // caller stops waiting, and the status must reach the UI so the user sees
  // something other than a blank board.
  const wire = new Wire("ws://127.0.0.1:1/ws", "tok");
  const statuses: string[] = [];
  wire.onStatus((s) => statuses.push(s));
  await expect(wire.connect(0n)).rejects.toThrow("wire: connection failed");
  expect(statuses).toContain("error");
});

test("a socket closing answers every in-flight command instead of hanging", async () => {
  // The pending map is keyed by request id and resolved by an inbound result.
  // If the socket dies first, nothing will ever resolve those promises — a UI
  // awaiting one waits for the rest of the session. Each is answered with an
  // explicit failure carrying ITS OWN request id, so a caller can tell which
  // command it lost.
  const gw = fakeGateway(() => {}); // never replies
  const wire = new Wire(gw.url, "tok");
  try {
    await wire.connect(0n);
    const inFlight = wire.send(create(ClientCommandSchema, { requestId: "mine-1" }));
    await until(() => gw.sockets.length > 0, "the socket");
    gw.sockets[0].close();

    const res = await inFlight;
    expect(res.ok).toBe(false);
    expect(res.error).toBe("wire: connection closed before a result arrived");
    expect(res.requestId).toBe("mine-1");
  } finally {
    gw.stop();
  }
});

test("sending without a connection is refused, not thrown", async () => {
  // send() is called from click handlers. Throwing would blank the console
  // and leave the button looking like it worked; a resolved failure renders
  // next to the control the user just pressed.
  const wire = new Wire("ws://127.0.0.1:1/ws", "tok");
  const res = await wire.send(create(ClientCommandSchema, { requestId: "r" }));
  expect(res.ok).toBe(false);
  expect(res.error).toBe("wire: not connected");
});

test("close and reconnect are safe before anything was ever connected", async () => {
  // Both reach for this.ws optionally. A component torn down before its
  // socket opened, or a retry that fires first, would otherwise throw on a
  // property of undefined — during teardown, where it is least visible.
  const wire = new Wire("ws://127.0.0.1:1/ws", "tok");
  expect(() => wire.close()).not.toThrow();
  await expect(wire.reconnect()).rejects.toThrow("wire: connection failed");
});

test("a result for an unknown request id is dropped, not fatal", async () => {
  // It can legitimately belong to a command abandoned by a previous
  // connection. Treating it as an error would kill a healthy socket over a
  // late reply nobody is waiting for.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  try {
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect(0n);
    gw.sockets[0].send(JSON.stringify({ result: { requestId: "nobody-waits-for-this", ok: true } }));
    // The socket must still be usable afterwards.
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(7) }));
    await until(() => seen.length > 0, "the event after the stray result");
    expect(seen).toEqual([7n]);
  } finally {
    gw.stop();
  }
});

test("an unknown frame kind is ignored and the stream keeps working", async () => {
  // Deliberate forward-compatibility: the server may add frame kinds, and an
  // older client must not fall over. This is the client half of the property
  // the Go harness client does NOT have — it tears the connection down on an
  // unrecognised frame — and the disagreement is recorded in ServerFrame's
  // ordering contract. Pinning it here keeps the client's half honest.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  try {
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect(0n);
    gw.sockets[0].send(JSON.stringify({ catchUpHead: { headSequence: "12" } }));
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(9) }));
    await until(() => seen.length > 0, "the event after the unknown frame");
    expect(seen).toEqual([9n]);
  } finally {
    gw.stop();
  }
});

test("an out-of-order replay never walks the resume cursor backwards", async () => {
  // lastSeq is what reconnect resumes from. Letting an older event lower it
  // would make a redial re-request history already folded, and the session
  // would double-apply it.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  try {
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect(0n);
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(5) }));
    await until(() => seen.length === 1, "the first event");
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(3) }));
    await until(() => seen.length === 2, "the older event");

    await wire.reconnect();
    // Two connections: the redial must resume from the HIGHEST sequence seen,
    // 5, not from the older 3 that arrived after it. One below either way (see
    // the reconnect test above), so the discriminator is 4 against 2.
    expect(gw.queries[1]).toContain("after=4");
  } finally {
    gw.stop();
  }
});

test("a caller's request id is used as-is, and a missing one is generated cleanly", async () => {
  // Correlation is by request id, so overwriting the caller's would strand
  // their promise. A generated one has to be well-formed too: a counter
  // running the wrong way yields "c--1", which reads as a different id scheme
  // to anything parsing it.
  const sent: string[] = [];
  const gw = fakeGateway((ws, raw) => {
    sent.push(JSON.parse(raw).requestId);
    ws.send(JSON.stringify({ result: { requestId: JSON.parse(raw).requestId, ok: true } }));
  });
  const wire = new Wire(gw.url, "tok");
  try {
    await wire.connect(0n);
    const mine = await wire.send(create(ClientCommandSchema, { requestId: "caller-chose-this" }));
    expect(mine.ok).toBe(true);
    expect(sent[0]).toBe("caller-chose-this");

    await wire.send(create(ClientCommandSchema, { requestId: "" }));
    expect(sent[1]).toMatch(/^c-\d+$/);
  } finally {
    gw.stop();
  }
});

test("sending after the socket dropped is refused, not fired into a dead socket", async () => {
  // Distinct from "never connected": here this.ws EXISTS, it is simply no
  // longer OPEN. That is the ordinary case — the user clicks a moment after
  // the gateway went away — and it is the readyState half of the guard that
  // catches it. Without that half the command is written to a closed socket
  // and the caller's promise is registered in a pending map nothing will ever
  // resolve.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  try {
    const statuses: string[] = [];
    wire.onStatus((s) => statuses.push(s));
    await wire.connect(0n);
    await until(() => gw.sockets.length > 0, "the socket");
    gw.sockets[0].close();
    await until(() => statuses.includes("closed"), "the close to reach the client");

    const res = await wire.send(create(ClientCommandSchema, { requestId: "after-the-drop" }));
    expect(res.ok).toBe(false);
    expect(res.error).toBe("wire: not connected");
  } finally {
    gw.stop();
  }
});

// --- presence (T5) ---------------------------------------------------------

test("a presence snapshot and its deltas reach the presence handler in order", async () => {
  // Wire's job is DISPATCH, not bookkeeping: it forwards frames and keeps no
  // participant list of its own. Ordering is what the server's snapshot-inside-
  // the-lock guarantees (T4), so a handler applying snapshot-then-deltas is
  // correct only if this preserves arrival order.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const seen: string[] = [];
  wire.onPresence((batch) => { for (const p of batch) seen.push(`${p.state}:${p.participantId}`); });
  await wire.connect(0n);
  await until(() => gw.sockets.length > 0, "socket");

  gw.sockets[0].send(JSON.stringify({
    presenceSnapshot: {
      present: [
        { participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" },
        { participantId: "p-2", displayName: "Bo", state: "PRESENCE_STATE_CONNECTED" },
      ],
    },
  }));
  gw.sockets[0].send(JSON.stringify({
    presenceChanged: { participantId: "p-3", displayName: "Cy", state: "PRESENCE_STATE_CONNECTED" },
  }));
  gw.sockets[0].send(JSON.stringify({
    presenceChanged: { participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_DISCONNECTED" },
  }));

  await until(() => seen.length === 4, `4 presence events, saw ${seen.length}`);
  expect(seen).toEqual([
    "CONNECTED:p-1",
    "CONNECTED:p-2",
    "CONNECTED:p-3",
    "DISCONNECTED:p-1",
  ]);
  wire.close();
  gw.stop();
});

test("a presence frame does not advance the replay cursor", async () => {
  // lastSeq drives reconnect's after=<seq>. Presence is NOT an event and
  // carries no sequence; letting it touch the cursor would skip real events
  // on the next redial.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const seen: string[] = [];
  wire.onPresence((batch) => { for (const p of batch) seen.push(p.participantId); });
  await wire.connect(0n);
  await until(() => gw.sockets.length > 0, "socket");

  gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(4) }));
  await until(() => wire.head === 4n, "event 4");
  gw.sockets[0].send(JSON.stringify({
    presenceChanged: { participantId: "p-9", displayName: "Zed", state: "PRESENCE_STATE_CONNECTED" },
  }));
  await until(() => seen.length === 1, "presence frame");

  expect(wire.head).toBe(4n);
  wire.close();
  gw.stop();
});

test("a presence frame with no state is dropped, not guessed", async () => {
  // An absent `state` decodes to PRESENCE_STATE_UNSPECIFIED — the realistic
  // shape, since protojson omits zero values, so a sender that forgot to set
  // it produces exactly this frame.
  //
  // Dropping is the only safe reading. Treating it as CONNECTED puts a phantom
  // at the table; treating it as DISCONNECTED evicts someone who is really
  // there. The enum exists (rather than a bool) precisely so this case is
  // representable and can be refused instead of guessed.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const seen: string[] = [];
  wire.onPresence((batch) => { for (const p of batch) seen.push(`${p.state}:${p.participantId}`); });
  await wire.connect(0n);
  await until(() => gw.sockets.length > 0, "socket");

  gw.sockets[0].send(JSON.stringify({
    presenceChanged: { participantId: "p-nostate", displayName: "Mystery" },
  }));
  // A well-formed frame AFTER it, so the assertion is about the first being
  // dropped rather than about nothing having arrived yet.
  gw.sockets[0].send(JSON.stringify({
    presenceChanged: { participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" },
  }));

  await until(() => seen.length > 0, "the well-formed frame");
  expect(seen).toEqual(["CONNECTED:p-1"]);
  wire.close();
  gw.stop();
});

test("a snapshot is flagged as a replacement; a delta is not", async () => {
  // The distinction Session needs to clear its map. A snapshot is the COMPLETE
  // table, so whoever it omits has left — and after a manual reconnect that is
  // the only way this client can ever learn it, because the DISCONNECTED
  // broadcast went out while it was not connected to receive it.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const calls: { ids: string[]; replace: boolean }[] = [];
  wire.onPresence((batch, replace) => calls.push({ ids: batch.map((p) => p.participantId), replace }));
  await wire.connect(0n);
  await until(() => gw.sockets.length > 0, "socket");

  gw.sockets[0].send(JSON.stringify({
    presenceSnapshot: {
      present: [{ participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" }],
    },
  }));
  gw.sockets[0].send(JSON.stringify({
    presenceChanged: { participantId: "p-2", displayName: "Bo", state: "PRESENCE_STATE_CONNECTED" },
  }));
  await until(() => calls.length === 2, "both frames");

  expect(calls[0]).toEqual({ ids: ["p-1"], replace: true });
  expect(calls[1]).toEqual({ ids: ["p-2"], replace: false });
  wire.close();
  gw.stop();
});

test("an EMPTY snapshot still replaces", async () => {
  // A table that emptied is a fact. Skipping the call for an empty batch would
  // leave the previous list standing — the same ghost, arrived at by an
  // optimisation rather than an oversight.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const calls: { n: number; replace: boolean }[] = [];
  wire.onPresence((batch, replace) => calls.push({ n: batch.length, replace }));
  await wire.connect(0n);
  await until(() => gw.sockets.length > 0, "socket");

  gw.sockets[0].send(JSON.stringify({ presenceSnapshot: {} }));
  await until(() => calls.length === 1, "the empty snapshot");
  expect(calls[0]).toEqual({ n: 0, replace: true });
  wire.close();
  gw.stop();
});

test("a snapshot entry with no state is skipped, and the rest still arrive", async () => {
  // The snapshot path's own guard. The delta path had a test for this; the
  // snapshot loop did not, so dropping its check survived CI — a malformed
  // entry would have been pushed through as a null and then crashed or
  // corrupted whoever applied the batch.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const calls: string[][] = [];
  wire.onPresence((batch) => calls.push(batch.map((p) => p.participantId)));
  await wire.connect(0n);
  await until(() => gw.sockets.length > 0, "socket");

  gw.sockets[0].send(JSON.stringify({
    presenceSnapshot: {
      present: [
        { participantId: "p-bad", displayName: "Mystery" },
        { participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" },
      ],
    },
  }));
  await until(() => calls.length === 1, "the snapshot");
  expect(calls[0]).toEqual(["p-1"]);
  wire.close();
  gw.stop();
});

// A scripted WebSocket, so a stale frame can be delivered on a socket the wire
// has already abandoned. The live-server fake above cannot do it: once the
// client closes, the runtime decides whether a queued frame still lands, and a
// test whose subject is a race must not be one.
class ScriptedSocket {
  static instances: ScriptedSocket[] = [];
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(readonly url: string) {
    ScriptedSocket.instances.push(this);
  }
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  deliver(frame: unknown) {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
  send() {}
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
}

test("a frame from the socket a reconnect abandoned is never folded", async () => {
  // THE ROLLBACK'S OWN RACE. reconnect() closes the socket, announces the
  // rollback, and redials — but the handler is bound per socket at connect
  // time and knows nothing about which socket it belongs to. A `message` event
  // queued before close() still runs, and its envelope lands AFTER the log was
  // truncated: the rolled-back sequence is re-added, the redial delivers it a
  // second time, and the duplicate introduction freezes the client
  // permanently. Measured before the fix as log 1,2,2 and
  // `duplicate actor "a1"`.
  //
  // Narrow through the UI today — the Reconnect button only shows on status
  // "closed" — but reconnect() is public API and any automatic redial opens it
  // wide.
  const nativeWS = globalThis.WebSocket;
  ScriptedSocket.instances = [];
  globalThis.WebSocket = ScriptedSocket as unknown as typeof WebSocket;
  try {
    const wire = new Wire("ws://scripted/ws", "tok-1");
    const seen: bigint[] = [];
    const rolledBack: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    wire.onRollback((s) => rolledBack.push(s));

    const first = wire.connect(0n);
    ScriptedSocket.instances[0]!.open();
    await first;
    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(2) });
    expect(seen).toEqual([2n]);

    const redial = wire.reconnect();
    ScriptedSocket.instances[1]!.open();
    await redial;

    // The abandoned socket speaks anyway. Nothing it says may reach the fold,
    // and nothing it says may move the cursor the redial just set.
    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(2) });

    expect(seen).toEqual([2n]);
    expect(rolledBack).toEqual([1n]);
    expect(wire.head).toBe(1n);
  } finally {
    globalThis.WebSocket = nativeWS;
  }
});

test("a second connect silences the socket the first one left behind", async () => {
  // The other half of the same hazard, and the half detach() does not cover.
  // reconnect() abandons its socket deliberately and clears its handlers;
  // connect() called directly — Session.start() twice, a caller redialling by
  // hand — simply overwrites this.ws and leaves the previous socket live with
  // its handlers still bound. The identity check in the message handler is
  // what makes that harmless, and this is the case that distinguishes the two
  // guards: deleting the check leaves this test failing and the abandoned-
  // socket test above still passing.
  const nativeWS = globalThis.WebSocket;
  ScriptedSocket.instances = [];
  globalThis.WebSocket = ScriptedSocket as unknown as typeof WebSocket;
  try {
    const wire = new Wire("ws://scripted/ws", "tok-1");
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));

    const first = wire.connect(0n);
    ScriptedSocket.instances[0]!.open();
    await first;

    const second = wire.connect(0n);
    ScriptedSocket.instances[1]!.open();
    await second;

    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(4) });
    expect(seen).toEqual([]);
    expect(wire.head).toBe(0n);

    ScriptedSocket.instances[1]!.deliver({ event: envelopeJSON(4) });
    expect(seen).toEqual([4n]);
  } finally {
    globalThis.WebSocket = nativeWS;
  }
});

test("reconnecting repeatedly does not walk the cursor backwards", async () => {
  // The resume cursor steps back one sequence per redial so a torn batch can
  // be taken again whole. It must step back from the HIGHEST SEQUENCE EVER
  // SEEN, not from wherever the previous redial left it — otherwise every
  // click of the Reconnect button rewinds the board by one more event, which
  // is precisely when that button is on screen. The DM and the agent reach
  // this too, and for them the tear it guards against cannot even happen.
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect(0n);
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(9) }));
    await until(() => wire.head === 9n, "the cursor to reach 9");

    for (let i = 0; i < 4; i++) {
      await wire.reconnect();
      await until(() => gw.queries.length === i + 2, `redial ${i + 1}`);
    }

    for (const q of gw.queries.slice(1)) {
      expect(q).toContain("after=8");
    }
    expect(wire.head).toBe(8n);
    wire.close();
  } finally {
    gw.stop();
  }
});
