import { afterEach, describe, expect, test } from "bun:test";
import type { Server, ServerWebSocket } from "bun";
import { create, toJson } from "@bufbuild/protobuf";
import {
  CatchUpHeadSchema,
  ServerFrameSchema,
  ViewpointChangedSchema,
  type ServerFrame,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { EnvelopeSchema, NarrationAddedSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
import { Wire, type WireStatus } from "../src/wire";
import { Session } from "../src/session";

// An id that must be encoded to survive a query string: '+', a non-ASCII
// letter, a slash, and an '&' that would forge a second `log` if left raw.
const REMEMBERED = "lera+ä/1&log=x";
const OTHER = "bram";
const TOKEN = "tok";

type DialData = { dial: number };

async function until(cond: () => boolean, what: string, ms = 5000): Promise<void> {
  const deadline = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`);
    await new Promise((r) => setTimeout(r, 5));
  }
}

async function within<T>(p: Promise<T>, what: string, ms = 5000): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(`timed out waiting for ${what}`)), ms);
  });
  try {
    return await Promise.race([p, timeout]);
  } finally {
    clearTimeout(timer);
  }
}

// Waits for a dial the gateway refuses to finish, whichever way the caller
// reports it.
async function settle(p: Promise<unknown>, ms = 5000): Promise<"resolved" | "rejected" | "pending"> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<"pending">((r) => {
    timer = setTimeout(() => r("pending"), ms);
  });
  try {
    return await Promise.race([
      p.then(
        () => "resolved" as const,
        () => "rejected" as const,
      ),
      timeout,
    ]);
  } finally {
    clearTimeout(timer);
  }
}

function frame(f: ServerFrame["frame"]): string {
  return JSON.stringify(toJson(ServerFrameSchema, create(ServerFrameSchema, { frame: f })));
}

const vc = (actorId: string): string =>
  frame({ case: "viewpointChanged", value: create(ViewpointChangedSchema, { actorId }) });

const entry = (seq: number): string =>
  frame({
    case: "event",
    value: create(EnvelopeSchema, {
      eventId: `e${seq}`,
      sequence: BigInt(seq),
      payload: { case: "narrationAdded", value: create(NarrationAddedSchema, { text: `line ${seq}` }) },
    }),
  });

const catchUp = (head: number): string =>
  frame({ case: "catchUpHead", value: create(CatchUpHeadSchema, { headSequence: BigInt(head) }) });

class FakeGateway {
  readonly dials: URLSearchParams[] = [];
  readonly sockets: (ServerWebSocket<DialData> | undefined)[] = [];
  readonly closed = new Set<number>();
  private readonly refused = new Set<number>();
  private readonly server: Server<DialData>;

  constructor() {
    this.server = Bun.serve({
      hostname: "127.0.0.1",
      port: 0,
      fetch: (req, srv) => {
        const n = this.dials.length;
        this.dials.push(new URL(req.url).searchParams);
        if (this.refused.has(n)) return new Response("refused", { status: 503 });
        if (srv.upgrade(req, { data: { dial: n } })) return undefined;
        return new Response("not a websocket", { status: 400 });
      },
      websocket: {
        data: {} as DialData,
        open: (ws) => {
          this.sockets[ws.data.dial] = ws;
        },
        message: () => {},
        close: (ws) => {
          this.closed.add(ws.data.dial);
        },
      },
    });
  }

  get url(): string {
    return `ws://127.0.0.1:${String(this.server.port)}/ws`;
  }

  refuse(dial: number): void {
    this.refused.add(dial);
  }

  async send(dial: number, ...frames: string[]): Promise<void> {
    await until(() => this.sockets[dial] !== undefined, `socket of dial ${dial}`);
    const ws = this.sockets[dial]!;
    for (const f of frames) ws.send(f);
  }

  async drop(dial: number): Promise<void> {
    await until(() => this.sockets[dial] !== undefined, `socket of dial ${dial}`);
    this.sockets[dial]!.close(1000, "dropped");
    await until(() => this.closed.has(dial), `close of dial ${dial}`);
  }

  dial(n: number): URLSearchParams {
    const q = this.dials[n];
    if (q === undefined) throw new Error(`no dial ${n}; ${this.dials.length} recorded`);
    return q;
  }

  stop(): void {
    this.server.stop(true);
  }
}

const gateways: FakeGateway[] = [];
const closers: (() => void)[] = [];

function gateway(): FakeGateway {
  const gw = new FakeGateway();
  gateways.push(gw);
  return gw;
}

type WireProbe = {
  w: Wire;
  statuses: WireStatus[];
  events: bigint[];
  viewpoints: string[];
};

function wireOn(gw: FakeGateway, remembered: string | null): WireProbe {
  const w = new Wire(gw.url, TOKEN, remembered);
  const probe: WireProbe = { w, statuses: [], events: [], viewpoints: [] };
  w.onStatus((s) => probe.statuses.push(s));
  w.onEvent((e) => probe.events.push(e.sequence));
  w.onViewpoint((a) => probe.viewpoints.push(a));
  closers.push(() => w.close());
  return probe;
}

type SessionProbe = {
  s: Session;
  statuses: WireStatus[];
  viewpoints: string[];
};

function sessionOn(gw: FakeGateway, remembered: string | null): SessionProbe {
  const s = new Session(gw.url, TOKEN, remembered);
  const probe: SessionProbe = { s, statuses: [], viewpoints: [] };
  s.onStatus((st) => probe.statuses.push(st));
  s.onViewpoint((a) => probe.viewpoints.push(a));
  s.onError(() => {});
  closers.push(() => s.close());
  return probe;
}

async function untilDown(statuses: WireStatus[], what: string): Promise<void> {
  await until(() => {
    const last = statuses.at(-1);
    return last === "closed" || last === "error";
  }, `${what} to report its socket down`);
}

afterEach(() => {
  for (const c of closers.splice(0)) {
    try {
      c();
    } catch {
      // a client already closed
    }
  }
  for (const gw of gateways.splice(0)) gw.stop();
});

describe("Wire given a remembered actor, before any ViewpointChanged", () => {
  // VTT-442
  test("qaRL: connect names the remembered actor in log, once, with after 0", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    expect(gw.dials).toHaveLength(1);
    expect(gw.dial(0).getAll("log")).toEqual([REMEMBERED]);
    expect(gw.dial(0).getAll("after")).toEqual(["0"]);
  });

  // VTT-442
  test("qaRL: a reconnect after a refused dial names the remembered actor with after 0", async () => {
    const gw = gateway();
    gw.refuse(0);
    const p = wireOn(gw, REMEMBERED);
    await settle(p.w.connect());
    await within(p.w.reconnect(), "reconnect");
    expect(gw.dials.length).toBeGreaterThanOrEqual(2);
    for (const q of gw.dials) {
      expect(q.getAll("log")).toEqual([REMEMBERED]);
      expect(q.getAll("after")).toEqual(["0"]);
    }
  });

  // VTT-442
  test("qaRL: reconnects after sockets opened and closed with no ViewpointChanged name the remembered actor with after 0", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "first reconnect");
    await gw.drop(1);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "second reconnect");
    expect(gw.dials.length).toBeGreaterThanOrEqual(3);
    for (const q of gw.dials) {
      expect(q.getAll("log")).toEqual([REMEMBERED]);
      expect(q.getAll("after")).toEqual(["0"]);
    }
  });
});

describe.each([
  ["nobody (\"\")", ""],
  ["nothing (null)", null],
] as const)("Wire remembering %s, before any ViewpointChanged", (_label, remembered) => {
  // VTT-442
  test("qaRL: connect, a reconnect after a closed socket and a reconnect after a refused dial name no log", async () => {
    const gw = gateway();
    gw.refuse(2);
    const p = wireOn(gw, remembered);
    await within(p.w.connect(), "connect");
    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "reconnect after close");
    await gw.drop(1);
    await untilDown(p.statuses, "the wire");
    await settle(p.w.reconnect());
    await within(p.w.reconnect(), "reconnect after refusal");
    expect(gw.dials.length).toBeGreaterThanOrEqual(4);
    for (const q of gw.dials) expect(q.has("log")).toBe(false);
  });
});

describe("Wire.onViewpoint", () => {
  // VTT-442
  test("qaRL: called with the actor of every ViewpointChanged, the remembered one, repeats and nobody included", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(REMEMBERED), vc(REMEMBERED), vc(OTHER), vc(""), vc(""), vc(OTHER), entry(1));
    await until(() => p.events.length === 1, "the sentinel entry");
    expect(p.viewpoints).toEqual([REMEMBERED, REMEMBERED, OTHER, "", "", OTHER]);

    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "reconnect");
    await gw.send(1, vc(OTHER), entry(2));
    await until(() => p.events.length === 2, "the second sentinel entry");
    expect(p.viewpoints).toEqual([REMEMBERED, REMEMBERED, OTHER, "", "", OTHER, OTHER]);
  });
});

describe("Wire once a ViewpointChanged has arrived", () => {
  // VTT-397 VTT-398
  test("qaRL: a redial names the actor the ViewpointChanged named, not the remembered one, at its highest sequence", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(OTHER), entry(1), entry(2), entry(3));
    await until(() => p.events.length === 3, "three entries");
    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "reconnect");
    expect(gw.dial(1).getAll("log")).toEqual([OTHER]);
    expect(gw.dial(1).getAll("after")).toEqual(["3"]);
  });

  // VTT-397
  test("qaRL: a redial after a ViewpointChanged naming nobody names no log, though an actor was remembered", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(""));
    await until(() => p.w.viewpoint === "", "the ViewpointChanged naming nobody");
    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "reconnect");
    expect(gw.dial(1).has("log")).toBe(false);
  });

  // VTT-397 VTT-398
  test("qaRL: a ViewpointChanged naming the remembered actor starts the cursor: the redial names it at the highest sequence sent, not 0", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(REMEMBERED), entry(1), entry(2), entry(5), entry(3), catchUp(9));
    await until(() => p.events.length === 4, "four entries");
    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "reconnect");
    expect(gw.dial(1).getAll("log")).toEqual([REMEMBERED]);
    expect(gw.dial(1).getAll("after")).toEqual(["5"]);
  });

  // VTT-397 VTT-398
  test("qaRL: later dials whose sockets carry no ViewpointChanged keep naming the held log at its cursor, never the remembered one", async () => {
    const gw = gateway();
    gw.refuse(2);
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(OTHER), entry(1), entry(2), entry(3));
    await until(() => p.events.length === 3, "three entries");
    await gw.drop(0);
    await untilDown(p.statuses, "the wire");
    await within(p.w.reconnect(), "reconnect after close");
    await gw.drop(1);
    await untilDown(p.statuses, "the wire");
    await settle(p.w.reconnect());
    await within(p.w.reconnect(), "reconnect after refusal");
    expect(gw.dials.length).toBeGreaterThanOrEqual(4);
    for (const q of gw.dials.slice(1)) {
      expect(q.getAll("log")).toEqual([OTHER]);
      expect(q.getAll("after")).toEqual(["3"]);
    }
  });

  // VTT-397 VTT-398
  test.each([
    ["nobody (\"\")", ""],
    ["nothing (null)", null],
  ] as const)(
    "qaRL: a Wire remembering %s, once told an actor, redials naming that actor at its cursor",
    async (_label, remembered) => {
      const gw = gateway();
      const p = wireOn(gw, remembered);
      await within(p.w.connect(), "connect");
      await gw.send(0, vc(OTHER), entry(1), entry(2));
      await until(() => p.events.length === 2, "two entries");
      await gw.drop(0);
      await untilDown(p.statuses, "the wire");
      await within(p.w.reconnect(), "reconnect");
      expect(gw.dial(1).getAll("log")).toEqual([OTHER]);
      expect(gw.dial(1).getAll("after")).toEqual(["2"]);
    },
  );
});

describe("Wire.viewpoint", () => {
  // VTT-407
  test("qaRL: a remembered actor is never Wire.viewpoint: null through a refused dial, an open socket sent its head and a close", async () => {
    const gw = gateway();
    gw.refuse(0);
    const p = wireOn(gw, REMEMBERED);
    expect(p.w.viewpoint).toBeNull();
    await settle(p.w.connect());
    expect(p.w.viewpoint).toBeNull();
    await within(p.w.reconnect(), "reconnect");
    expect(p.w.viewpoint).toBeNull();
    await gw.send(1, catchUp(2));
    await gw.drop(1);
    await untilDown(p.statuses, "the wire");
    expect(p.w.viewpoint).toBeNull();
    await within(p.w.reconnect(), "second reconnect");
    expect(p.w.viewpoint).toBeNull();
  });

  // VTT-407
  test("qaRL: after a ViewpointChanged, Wire.viewpoint is what it named: another actor, the remembered one, then nobody", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(OTHER));
    await until(() => p.viewpoints.length === 1, "first ViewpointChanged");
    expect(p.w.viewpoint).toBe(OTHER);
    await gw.send(0, vc(REMEMBERED));
    await until(() => p.viewpoints.length === 2, "second ViewpointChanged");
    expect(p.w.viewpoint).toBe(REMEMBERED);
    await gw.send(0, vc(""));
    await until(() => p.w.viewpoint === "", "third ViewpointChanged");
    expect(p.w.viewpoint).toBe("");
  });

  // VTT-407
  test("qaRL: a first ViewpointChanged naming nobody does not show the remembered actor as watched", async () => {
    const gw = gateway();
    const p = wireOn(gw, REMEMBERED);
    await within(p.w.connect(), "connect");
    await gw.send(0, vc(""));
    await until(() => p.w.viewpoint !== null, "the ViewpointChanged");
    expect(p.w.viewpoint).toBe("");
  });
});

describe("Session given a remembered actor", () => {
  // VTT-442
  test("qaRL: start and every reconnect before a ViewpointChanged name the remembered actor with after 0", async () => {
    const gw = gateway();
    gw.refuse(2);
    const p = sessionOn(gw, REMEMBERED);
    await within(p.s.start(), "start");
    await gw.drop(0);
    await untilDown(p.statuses, "the session");
    await within(p.s.reconnect(), "reconnect after close");
    await gw.drop(1);
    await untilDown(p.statuses, "the session");
    await settle(p.s.reconnect());
    await within(p.s.reconnect(), "reconnect after refusal");
    expect(gw.dials.length).toBeGreaterThanOrEqual(4);
    for (const q of gw.dials) {
      expect(q.getAll("log")).toEqual([REMEMBERED]);
      expect(q.getAll("after")).toEqual(["0"]);
    }
  });

  // VTT-397 VTT-398
  test("qaRL: once a ViewpointChanged names another actor, the Session's redial names it at its cursor; once it names nobody, no log", async () => {
    const gw = gateway();
    const p = sessionOn(gw, REMEMBERED);
    await within(p.s.start(), "start");
    await gw.send(0, vc(OTHER), entry(1), entry(2));
    await gw.drop(0);
    await untilDown(p.statuses, "the session");
    await within(p.s.reconnect(), "reconnect");
    expect(gw.dial(1).getAll("log")).toEqual([OTHER]);
    expect(gw.dial(1).getAll("after")).toEqual(["2"]);

    await gw.send(1, vc(""));
    await gw.drop(1);
    await untilDown(p.statuses, "the session");
    await within(p.s.reconnect(), "second reconnect");
    expect(gw.dial(2).has("log")).toBe(false);
  });

  // VTT-442
  test("qaRL: Session.onViewpoint is called with the actor of every ViewpointChanged, the remembered one, repeats and nobody included", async () => {
    const gw = gateway();
    const p = sessionOn(gw, REMEMBERED);
    await within(p.s.start(), "start");
    await gw.send(0, vc(REMEMBERED), vc(REMEMBERED), vc(OTHER), vc(""), vc(""), vc(OTHER));
    await gw.drop(0);
    await untilDown(p.statuses, "the session");
    expect(p.viewpoints).toEqual([REMEMBERED, REMEMBERED, OTHER, "", "", OTHER]);
  });

  // VTT-407
  test("qaRL: Session.viewpoint is the empty string until a ViewpointChanged, then what each one named", async () => {
    const gw = gateway();
    const p = sessionOn(gw, REMEMBERED);
    expect(p.s.viewpoint).toBe("");
    await within(p.s.start(), "start");
    expect(p.s.viewpoint).toBe("");
    await gw.send(0, catchUp(1));
    await gw.drop(0);
    await untilDown(p.statuses, "the session");
    expect(p.s.viewpoint).toBe("");
    await within(p.s.reconnect(), "reconnect");
    expect(p.s.viewpoint).toBe("");
    await gw.send(1, vc(OTHER));
    await until(() => p.viewpoints.length === 1, "first ViewpointChanged");
    expect(p.s.viewpoint).toBe(OTHER);
    await gw.send(1, vc(REMEMBERED));
    await until(() => p.viewpoints.length === 2, "second ViewpointChanged");
    expect(p.s.viewpoint).toBe(REMEMBERED);
    await gw.send(1, vc(""));
    await until(() => p.s.viewpoint === "", "third ViewpointChanged");
    expect(p.s.viewpoint).toBe("");
  });
});

describe.each([
  ["nobody (\"\")", ""],
  ["nothing (null)", null],
] as const)("Session remembering %s", (_label, remembered) => {
  // VTT-442
  test("qaRL: start and reconnects before a ViewpointChanged name no log", async () => {
    const gw = gateway();
    const p = sessionOn(gw, remembered);
    await within(p.s.start(), "start");
    await gw.drop(0);
    await untilDown(p.statuses, "the session");
    await within(p.s.reconnect(), "reconnect");
    expect(gw.dials.length).toBeGreaterThanOrEqual(2);
    for (const q of gw.dials) expect(q.has("log")).toBe(false);
  });
});
