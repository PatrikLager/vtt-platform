import { test, expect } from "bun:test";
import { create, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  SessionStartedSchema,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { Wire } from "../src/wire";

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
    await Bun.sleep(20);

    expect(seen).toEqual([1n]);
    expect(gw.queries[0]).toContain("after=0");
    expect(gw.queries[0]).toContain("token=tok-1");
    wire.close();
  } finally {
    gw.stop();
  }
});

test("a reconnect resumes from the last sequence seen, not from zero", async () => {
  // Resuming from 0 would replay the whole log into a client that already
  // folded it — duplicated events, and a state that disagrees with the server.
  const gw = fakeGateway(() => {});
  try {
    const wire = new Wire(gw.url, "tok-1");
    await wire.connect(0n);
    gw.sockets[0].send(JSON.stringify({ event: envelopeJSON(7) }));
    await Bun.sleep(20);

    await wire.reconnect();
    await Bun.sleep(20);

    expect(gw.queries.length).toBe(2);
    expect(gw.queries[1]).toContain("after=7");
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
    await Bun.sleep(20);
    expect(statuses).toContain("connecting");
    expect(statuses).toContain("open");
    expect(statuses).toContain("closed");
  } finally {
    gw.stop();
  }
});
