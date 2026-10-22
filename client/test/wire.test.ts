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

function viewpointJSON(actorId: string) {
  return JSON.stringify({ viewpointChanged: actorId === "" ? {} : { actorId } });
}

function queryOf(gw: { queries: string[] }, i: number) {
  return new URLSearchParams(gw.queries[i]);
}

test("connect replays from after=0 and delivers events", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect();

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

// VTT-397, VTT-398
test("a player's redial names the log it holds and the last sequence it folded", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON("a1"));
    for (const seq of [1, 2]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => wire.head === 2n, "a1's log to reach sequence 2");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a1");
    expect(queryOf(gw, 1).get("after")).toBe("2");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-397
test("a redial names a log whose id needs encoding exactly", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON("a 1&after=99"));
    await until(() => wire.viewpoint === "a 1&after=99", "the ViewpointChanged");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a 1&after=99");
    expect(queryOf(gw, 1).getAll("after")).toEqual(["0"]);
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-397, VTT-398
test("a DM's redial names its last world sequence and no log", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-dm");
    await wire.connect();
    for (const seq of [1, 2, 3]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => wire.head === 3n, "the world to reach sequence 3");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).has("log")).toBe(false);
    expect(queryOf(gw, 1).get("after")).toBe("3");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-398
test("a redial after a switch resumes the new log, not the old one", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON("a1"));
    for (const seq of [1, 2, 3, 4, 5]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => wire.head === 5n, "a1's log to reach sequence 5");
    gw.sockets[0].send(viewpointJSON("a2"));
    for (const seq of [1, 2]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => wire.viewpoint === "a2" && wire.head === 2n, "a2's log to reach sequence 2");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a2");
    expect(queryOf(gw, 1).get("after")).toBe("2");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-398
test("a redial before the new log's first entry names the new log from its start", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON("a1"));
    for (const seq of [1, 2, 3, 4, 5]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => wire.head === 5n, "a1's log to reach sequence 5");
    gw.sockets[0].send(viewpointJSON("a2"));
    await until(() => wire.viewpoint === "a2", "the switch to a2");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a2");
    expect(queryOf(gw, 1).get("after")).toBe("0");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-398
test("a redial after the held log started again resumes from where the new start reached", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect();
    gw.sockets[0].send(viewpointJSON("a1"));
    for (const seq of [1, 2, 3, 4, 5, 1, 2, 3]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => seen.length === 8, "a1's log and its new start");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a1");
    expect(queryOf(gw, 1).get("after")).toBe("3");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-397
test("a redial while watching nobody names no log", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-spectator");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON(""));
    await until(() => wire.viewpoint === "", "the ViewpointChanged naming nobody");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).has("log")).toBe(false);
    expect(queryOf(gw, 1).get("after")).toBe("0");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-442
test("a wire handed a remembered log names it, from after 0, on every dial until a ViewpointChanged arrives", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1", "a2");
    await wire.connect();
    expect(queryOf(gw, 0).get("log")).toBe("a2");
    expect(queryOf(gw, 0).get("after")).toBe("0");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a2");
    expect(queryOf(gw, 1).get("after")).toBe("0");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-442, VTT-397
test("a ViewpointChanged naming another log replaces the remembered one on the next dial", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1", "a2");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON("a1"));
    for (const seq of [1, 2]) gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(seq) }));
    await until(() => wire.head === 2n, "a1's log to reach sequence 2");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).get("log")).toBe("a1");
    expect(queryOf(gw, 1).get("after")).toBe("2");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-442, VTT-397
test("a ViewpointChanged naming nobody leaves the next dial naming no log, though a log was remembered", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-spectator", "a2");
    await wire.connect();
    gw.sockets[0].send(viewpointJSON(""));
    await until(() => wire.viewpoint === "", "the ViewpointChanged naming nobody");

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");
    expect(queryOf(gw, 1).has("log")).toBe(false);
    expect(queryOf(gw, 1).get("after")).toBe("0");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-407
test("a wire handed a remembered log shows no viewpoint until a ViewpointChanged names one", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1", "a2");
    await wire.connect();
    expect(wire.viewpoint).toBeNull();

    gw.sockets[0].send(viewpointJSON("a1"));
    await until(() => wire.viewpoint !== null, "the ViewpointChanged");
    expect(wire.viewpoint).toBe("a1");
    wire.close();
  } finally {
    gw.stop();
  }
});

// VTT-442
test("each ViewpointChanged reaches onViewpoint, nobody's included", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    const told: string[] = [];
    let sentinel = false;
    wire.onViewpoint((id) => told.push(id));
    wire.onWatchable(() => (sentinel = true));
    await wire.connect();
    for (const id of ["a1", "a1", ""]) gw.sockets[0].send(viewpointJSON(id));
    gw.sockets[0].send(JSON.stringify({ watchableChanged: {} }));
    await until(() => sentinel, "the frame sent after the three ViewpointChanged frames");
    expect(told).toEqual(["a1", "a1", ""]);
    wire.close();
  } finally {
    gw.stop();
  }
});

test("a reconnect before anything was ever seen still resumes from zero", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();

    await wire.reconnect();
    await until(() => gw.queries.length === 2, "the redial to reach the gateway");

    expect(gw.queries[1]).toContain("after=0");
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
    await wire.connect();

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
    await wire.connect();

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

// 2026-09-02-art-is-a-flat-library Task 2: CommandResult.warnings (field 5)
// carries non-fatal facts about a command that SUCCEEDED — load_map fills it
// when a map names art that is not installed (art design spec §4). Wire
// resolves send()'s promise with the decoded CommandResult verbatim (see
// handleFrame's "result" case), so this pins that a field added to the proto
// reaches the caller through that same generic path, with no per-field
// plumbing of its own needed in this file.
test("a result's warnings reach whoever awaited send()", async () => {
  const gw = fakeGateway((ws, raw) => {
    const cmd = JSON.parse(raw);
    ws.send(JSON.stringify({
      result: { requestId: cmd.requestId, ok: true, sequence: "1", warnings: ["square 0,0 names art \"absent-art\", which is not installed"] },
    }));
  });
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();

    const res = await wire.send(create(ClientCommandSchema, {
      command: { case: "startSession", value: { name: "S" } },
    }));

    expect(res.ok).toBe(true);
    expect(res.warnings).toEqual(["square 0,0 names art \"absent-art\", which is not installed"]);
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
    await wire.connect();
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
    await wire.connect();
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
  await expect(wire.connect()).rejects.toThrow("wire: connection failed");
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
    await wire.connect();
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
    await wire.connect();
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
    await wire.connect();
    gw.sockets[0].send(JSON.stringify({ catchUpHead: { headSequence: "12" } }));
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(9) }));
    await until(() => seen.length > 0, "the event after the unknown frame");
    expect(seen).toEqual([9n]);
  } finally {
    gw.stop();
  }
});

test("an out-of-order replay never walks the resume cursor backwards", async () => {
  // An older entry is a re-send; it must not lower the cursor the redial
  // names (VTT-398).
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  try {
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));
    await wire.connect();
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(5) }));
    await until(() => seen.length === 1, "the first event");
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(3) }));
    await until(() => seen.length === 2, "the older event");

    expect(wire.head).toBe(5n);

    await wire.reconnect();
    expect(gw.queries[1]).toContain("after=5");
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
    await wire.connect();
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
    await wire.connect();
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
  await wire.connect();
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
  // The cursor is what a redial names in after=; presence carries no sequence.
  const gw = fakeGateway(() => {});
  const wire = new Wire(gw.url, "tok");
  const seen: string[] = [];
  wire.onPresence((batch) => { for (const p of batch) seen.push(p.participantId); });
  await wire.connect();
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
  await wire.connect();
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
  await wire.connect();
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
  await wire.connect();
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
  await wire.connect();
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
  // send()'s guard reads WebSocket.OPEN off the GLOBAL, which is this class
  // while a test has installed it. Without the constant the comparison is
  // against undefined, every send is refused as "wire: not connected", and a
  // test about what happens to an in-flight command would quietly have none.
  static readonly OPEN = 1;
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
  // A message queued before close() still runs; its envelope must neither
  // reach the fold nor move the cursor the redial named.
  const nativeWS = globalThis.WebSocket;
  ScriptedSocket.instances = [];
  globalThis.WebSocket = ScriptedSocket as unknown as typeof WebSocket;
  try {
    const wire = new Wire("ws://scripted/ws", "tok-1");
    const seen: bigint[] = [];
    wire.onEvent((e) => seen.push(e.sequence));

    const first = wire.connect();
    ScriptedSocket.instances[0]!.open();
    await first;
    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(2) });
    expect(seen).toEqual([2n]);

    const redial = wire.reconnect();
    ScriptedSocket.instances[1]!.open();
    await redial;

    expect(ScriptedSocket.instances[1]!.url).toContain("after=2");

    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(3) });

    expect(seen).toEqual([2n]);
    expect(wire.head).toBe(2n);
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

    const first = wire.connect();
    ScriptedSocket.instances[0]!.open();
    await first;

    const second = wire.connect();
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

test("a close answers the commands sent on ITS socket, and no others", async () => {
  // THE PENDING MAP OUTLIVES EVERY SOCKET, and the close handler used to walk
  // the whole of it: one socket's close resolved every command in flight —
  // including the ones a LATER socket was still waiting on — with
  // `ok:false, "wire: connection closed before a result arrived"`, and then
  // cleared the map. So the caller got a fast, confident, WRONG answer, and the
  // server's real result landed a moment later on a promise that had already
  // settled and an id the map no longer held.
  //
  // The message handler beside it was given an identity guard in 45ae70d
  // (`if (this.ws !== ws) return`), and the asymmetry between the two was the
  // defect: a stale frame could not reach the fold, but a stale close could end
  // a live command.
  //
  // Narrow through the UI today — the Reconnect button shows only on status
  // "closed", which this same handler sets one line before it walks the map, so
  // the app's sockets are strictly serial. But connect() is public API, and a
  // second connect() (below) holds two live sockets with one map between them.
  //
  // BOTH DIRECTIONS IN ONE TEST, deliberately, because the cheap way to pass
  // the first half is to answer nothing at all. A command written to the socket
  // that is closing can never be answered on the wire, so leaving its promise
  // unresolved hangs whatever the UI was awaiting for the rest of the session —
  // that guarantee is what "a socket closing answers every in-flight command
  // instead of hanging" pins for a lone socket, and this pins it for a map
  // holding two sockets' worth.
  const nativeWS = globalThis.WebSocket;
  ScriptedSocket.instances = [];
  globalThis.WebSocket = ScriptedSocket as unknown as typeof WebSocket;
  try {
    const wire = new Wire("ws://scripted/ws", "tok-1");
    const first = wire.connect();
    ScriptedSocket.instances[0]!.open();
    await first;
    const onOld = wire.send(create(ClientCommandSchema, { requestId: "sent-on-the-old-socket" }));

    // A second connect abandons the first socket WITHOUT closing it, which is
    // what holds two live sockets long enough to tell whose command is whose —
    // the same shape as the test above, which covers the frame half of it.
    const second = wire.connect();
    ScriptedSocket.instances[1]!.open();
    await second;
    const onNew = wire.send(create(ClientCommandSchema, { requestId: "sent-on-the-new-socket" }));
    // Recorded rather than awaited: a wrongly-answered command resolves on a
    // microtask, and awaiting it would hang the test on the correct behaviour
    // instead of failing on the wrong one.
    const answeredByTheClose: string[] = [];
    void onNew.then((r) => answeredByTheClose.push(r.error || "ok"));

    ScriptedSocket.instances[0]!.close();
    await Promise.resolve();
    await Promise.resolve();
    expect(answeredByTheClose).toEqual([]);

    // Its own command IS answered, and with its own request id, so a caller
    // can tell which command it lost.
    const old = await onOld;
    expect(old.ok).toBe(false);
    expect(old.error).toBe("wire: connection closed before a result arrived");
    expect(old.requestId).toBe("sent-on-the-old-socket");

    // And the surviving entry is still correlatable: a cleared map drops this
    // result as belonging to nobody.
    ScriptedSocket.instances[1]!.deliver({
      result: { requestId: "sent-on-the-new-socket", ok: true },
    });
    const fresh = await onNew;
    expect(fresh.ok).toBe(true);
    expect(fresh.error).toBe("");
  } finally {
    globalThis.WebSocket = nativeWS;
  }
});

test("reconnecting repeatedly does not walk the cursor backwards", async () => {
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect();
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(9) }));
    await until(() => wire.head === 9n, "the cursor to reach 9");

    for (let i = 0; i < 4; i++) {
      await wire.reconnect();
      await until(() => gw.queries.length === i + 2, `redial ${i + 1}`);
    }

    for (const q of gw.queries.slice(1)) {
      expect(q).toContain("after=9");
    }
    expect(wire.head).toBe(9n);
    wire.close();
  } finally {
    gw.stop();
  }
});

test("a reconnect whose dial fails leaves the cursor and the log", async () => {
  const nativeWS = globalThis.WebSocket;
  ScriptedSocket.instances = [];
  globalThis.WebSocket = ScriptedSocket as unknown as typeof WebSocket;
  try {
    const wire = new Wire("ws://scripted/ws", "tok-1");
    let emptied = 0;
    wire.onRestart(() => emptied++);

    const first = wire.connect();
    ScriptedSocket.instances[0]!.open();
    await first;
    ScriptedSocket.instances[0]!.deliver({ viewpointChanged: { actorId: "a1" } });
    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(1) });
    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(2) });
    ScriptedSocket.instances[0]!.deliver({ event: envelopeJSON(3) });
    expect(emptied).toBe(1);

    const doomed = wire.reconnect();
    ScriptedSocket.instances[1]!.onerror?.();
    await expect(doomed).rejects.toThrow("wire: connection failed");

    expect(emptied).toBe(1);
    expect(wire.head).toBe(3n);
    expect(wire.viewpoint).toBe("a1");
    const again = wire.reconnect();
    expect(ScriptedSocket.instances[2]!.url).toContain("after=3&log=a1");
    ScriptedSocket.instances[2]!.open();
    await again;
  } finally {
    globalThis.WebSocket = nativeWS;
  }
});
