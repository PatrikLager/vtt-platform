import { test, expect } from "bun:test";
import { create, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  SessionStartedSchema,
  SceneCreatedSchema,
  ActorAddedSchema,
  TokenPlacedSchema,
  TokenMovedSchema,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { Session } from "../src/session";

/**
 * Wait until `ready()` holds, or fail loudly after `timeoutMs`.
 *
 * These tests replaced a fixed `await Bun.sleep(30)`, which was a guess at how
 * long a real WebSocket takes to deliver a replay. It is enough on an idle
 * machine and not enough on a busy one: under the load of a mutation run the
 * suite failed 3 of 4, deterministically, and Stryker refuses to start when
 * its initial test run fails — so a timing guess in one test file blocked the
 * whole mutation gate.
 *
 * Polling for the CONDITION is both faster (it returns as soon as the frame
 * lands, usually within a millisecond) and stable, because the timeout is a
 * backstop rather than the thing being measured.
 */
async function until(ready: () => boolean, what: string, timeoutMs = 2000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (ready()) return;
    await Bun.sleep(1);
  }
  throw new Error(`timed out after ${timeoutMs}ms waiting for ${what}`);
}


function env(seq: number, payload: any) {
  return toJson(
    EnvelopeSchema,
    create(EnvelopeSchema, { eventId: `evt-${seq}`, sequence: BigInt(seq), sessionId: "sess-1", payload }),
  );
}

function gatewayServing(frames: unknown[]) {
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open(ws) {
        for (const f of frames) ws.send(JSON.stringify(f));
      },
      message() {},
    },
  });
  return { url: `ws://localhost:${server.port}/ws`, stop: () => server.stop(true) };
}

const world = [
  { event: env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }) },
  { event: env(2, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s1", name: "S1", gridWidth: 9, gridHeight: 9 }) }) },
  { event: env(3, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "a1", name: "A" } }) }) },
  { event: env(4, { case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: "t1", sceneId: "s1", actorId: "a1", position: { x: 0, y: 0 } }) }) },
];

test("a session folds the replayed log into state", async () => {
  const gw = gatewayServing(world);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.head === 4n, "the replay to reach sequence 4");

    expect(s.head).toBe(4n);
    expect(s.state.Tokens["t1"]).toMatchObject({ SceneID: "s1", X: 0, Y: 0 });
    s.close();
  } finally {
    gw.stop();
  }
});

test("subscribers are notified when the state changes", async () => {
  const gw = gatewayServing(world);
  try {
    const s = new Session(gw.url, "tok");
    let notifications = 0;
    s.onChange(() => notifications++);
    await s.start();
    await until(() => notifications > 0, "a change notification");
    expect(notifications).toBeGreaterThan(0);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a fold error is surfaced instead of leaving a half-applied state", async () => {
  // A malformed log means the client cannot know the true state. Showing a
  // partial one would be worse than showing an error: the player would act on
  // a board that never existed.
  const gw = gatewayServing([
    { event: env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "A" }) }) },
    { event: env(2, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "B" }) }) },
  ]);
  try {
    const s = new Session(gw.url, "tok");
    const errors: string[] = [];
    s.onError((e) => errors.push(e.message));
    await s.start();
    await until(() => errors.length > 0, "the fold error to surface");
    expect(errors.length).toBeGreaterThan(0);
    expect(s.failure).toBe(errors[0]!);
    s.close();
  } finally {
    gw.stop();
  }
});

test("reconnect redials and resumes from the last folded sequence", async () => {
  // Session.reconnect is a one-line delegation to Wire.reconnect, and a
  // delegation with nothing asserting it is indistinguishable from an empty
  // body. That matters here more than the line count suggests: reconnect is
  // the ONLY recovery path after a dropped socket, so a silently-empty one
  // leaves a client that looks connected, never redials, and simply stops
  // receiving events — the failure the wire's status handling exists to
  // surface, arriving instead as an unexplained frozen board.
  //
  // The discriminator is a SECOND connection carrying an event the first one
  // never sent, and the `after` cursor it resumes from, which is what makes
  // this a resume rather than a fresh replay (VTT-398).
  const seen: string[] = [];
  let opened = 0;
  let pendingAfter = 0n;
  const all = [
    ...world,
    { event: env(5, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", sceneId: "s1", to: { x: 3, y: 3 } }) }) },
  ];
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      const after = new URL(req.url).searchParams.get("after") ?? "";
      seen.push(after);
      pendingAfter = BigInt(after || "0");
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open(ws) {
        // The log GREW while this client was away — sequence 5 is the event
        // the reconnect exists to collect — and the replay is `seq > afterSeq`,
        // exactly as internal/store/subscribe.go resumes.
        opened += 1;
        const after = pendingAfter;
        for (const f of opened === 1 ? world : all) {
          if (BigInt((f as any).event.sequence) > after) ws.send(JSON.stringify(f));
        }
      },
      message() {},
    },
  });
  const url = `ws://localhost:${server.port}/ws`;
  try {
    const s = new Session(url, "tok");
    await s.start();
    await until(() => s.head === 4n, "the first connection's replay");

    await s.reconnect();
    await until(() => s.head === 5n, "the redialled connection's event");

    expect(s.head).toBe(5n);
    expect(s.state.Tokens["t1"]).toMatchObject({ X: 3, Y: 3 });
    expect(seen.length).toBe(2);
    expect(seen[0]).toBe("0");
    expect(seen[1]).toBe("4");
    s.close();
  } finally {
    server.stop(true);
  }
});

// --- presence (T5) ---------------------------------------------------------

const pFrame = (id: string, name: string, state: string) => ({
  presenceChanged: { participantId: id, displayName: name, state },
});

test("the participant list starts from the snapshot and follows the deltas", async () => {
  const gw = gatewayServing([
    {
      presenceSnapshot: {
        present: [
          // Ids deliberately DISAGREE with name order: p-3 sorts first by
          // name, last by id. The previous version of this test disconnected
          // the only out-of-order element, so what reached the assertion was
          // already in insertion order and the comparator was invisible —
          // eleven sort mutants survived it.
          { participantId: "p-3", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" },
          { participantId: "p-1", displayName: "Cy", state: "PRESENCE_STATE_CONNECTED" },
        ],
      },
    },
    pFrame("p-2", "Bo", "PRESENCE_STATE_CONNECTED"),
  ]);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.participants.length === 3, "presence to settle");

    // Sorted by display name, so the list does not reshuffle on every delta —
    // arrival order is a property of the network, not of the table.
    expect(s.participants.map((p) => p.displayName)).toEqual(["Ada", "Bo", "Cy"]);
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-3", "p-2", "p-1"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("participants sharing a display name are ordered by id", async () => {
  // The tie-break arm. DELIVERY ORDER IS THE POINT, and it is why there are
  // two of these: a comparator that always returns -1 happens to produce the
  // right answer for one arrival order and the wrong one for the other, so a
  // single test cannot see it. Measured — the first version of this test used
  // the order that the broken comparator gets right, and the mutant survived
  // CI.
  const gw = gatewayServing([
    pFrame("p-2", "Sam", "PRESENCE_STATE_CONNECTED"),
    pFrame("p-9", "Sam", "PRESENCE_STATE_CONNECTED"),
  ]);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.participants.length === 2, "both Sams");
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-2", "p-9"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("the display-name tie-break holds whichever order they arrive in", async () => {
  // The mirror of the above, arriving reversed. Kills the "always 1" half.
  const gw = gatewayServing([
    pFrame("p-9", "Sam", "PRESENCE_STATE_CONNECTED"),
    pFrame("p-2", "Sam", "PRESENCE_STATE_CONNECTED"),
  ]);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.participants.length === 2, "both Sams");
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-2", "p-9"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a DISCONNECTED delta removes that participant", async () => {
  // The ordinary departure — someone closes their tab while this client is
  // connected to hear about it. Rewriting the sort test earlier deleted the
  // only case that drove this path, and CI caught it: emptying the delete
  // branch survived the whole suite.
  const gw = gatewayServing([
    pFrame("p-1", "Ada", "PRESENCE_STATE_CONNECTED"),
    pFrame("p-2", "Bo", "PRESENCE_STATE_CONNECTED"),
    pFrame("p-1", "Ada", "PRESENCE_STATE_DISCONNECTED"),
  ]);
  try {
    const s = new Session(gw.url, "tok");
    let changes = 0;
    s.onChange(() => changes++);
    await s.start();
    await until(() => changes >= 3, "all three frames");
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-2"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a departure a client never saw is corrected by the next snapshot", async () => {
  // The ghost. A reconnect replays a full snapshot, and anyone who left while
  // this client was disconnected appears in NO frame it will ever receive —
  // the DISCONNECTED went out while it was not there to hear it. Merging the
  // snapshot instead of replacing leaves them at the table forever, which is
  // exactly the failure presence exists to prevent, reached through the
  // Reconnect button T5 adds.
  const gw = gatewayServing([
    {
      presenceSnapshot: {
        present: [
          { participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" },
          { participantId: "p-2", displayName: "Bo", state: "PRESENCE_STATE_CONNECTED" },
        ],
      },
    },
    // The second snapshot is what a reconnect delivers: Bo is simply absent.
    {
      presenceSnapshot: {
        present: [{ participantId: "p-1", displayName: "Ada", state: "PRESENCE_STATE_CONNECTED" }],
      },
    },
  ]);
  try {
    const s = new Session(gw.url, "tok");
    let changes = 0;
    s.onChange(() => changes++);
    await s.start();
    await until(() => changes >= 2, "both snapshots");
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-1"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a participant reconnecting is not listed twice", async () => {
  // The server announces CONNECTED on a participant's FIRST connection only,
  // but a client that reconnects replays a fresh snapshot on top of a list it
  // already has — so the list must be keyed by participant, not appended to.
  const gw = gatewayServing([
    pFrame("p-1", "Ada", "PRESENCE_STATE_CONNECTED"),
    pFrame("p-1", "Ada", "PRESENCE_STATE_CONNECTED"),
  ]);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.participants.length === 1, "one participant");
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-1"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a presence change notifies the view", async () => {
  // Without this the list is correct and never drawn: presence arrives on its
  // own frames, not on an event, so nothing else fires onChange.
  const gw = gatewayServing([pFrame("p-1", "Ada", "PRESENCE_STATE_CONNECTED")]);
  try {
    const s = new Session(gw.url, "tok");
    let changes = 0;
    s.onChange(() => changes++);
    await s.start();
    await until(() => changes > 0, "a change notification");
    expect(s.participants).toHaveLength(1);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a departure for someone never seen leaves the list alone", async () => {
  const gw = gatewayServing([
    pFrame("p-1", "Ada", "PRESENCE_STATE_CONNECTED"),
    pFrame("p-ghost", "Nobody", "PRESENCE_STATE_DISCONNECTED"),
  ]);
  try {
    const s = new Session(gw.url, "tok");
    let changes = 0;
    s.onChange(() => changes++);
    await s.start();
    await until(() => changes >= 2, "both frames handled");
    expect(s.participants.map((p) => p.participantId)).toEqual(["p-1"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("close closes the socket", async () => {
  let closed = false;
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open() {},
      message() {},
      close() {
        closed = true;
      },
    },
  });
  try {
    const s = new Session(`ws://localhost:${server.port}/ws`, "tok");
    await s.start();
    s.close();
    await until(() => closed, "the gateway to see the socket close");
  } finally {
    server.stop(true);
  }
});

// --- one log at a time -------------------------------------------------------

const viewpoint = (actorId: string) => ({ viewpointChanged: actorId === "" ? {} : { actorId } });

// A second log that opens a session and a scene the first one also held.
const a2Log = [
  { event: env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }) },
  { event: env(2, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s1", name: "S1", gridWidth: 9, gridHeight: 9 }) }) },
  { event: env(3, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "a2", name: "B" } }) }) },
];

// VTT-394
test("a ViewpointChanged naming another log empties what was folded", async () => {
  const gw = gatewayServing([viewpoint("a1"), ...world, viewpoint("a2"), ...a2Log]);
  try {
    const s = new Session(gw.url, "tok");
    const errors: string[] = [];
    s.onError((e) => errors.push(e.message));
    await s.start();
    await until(() => s.viewpoint === "a2" && s.events.length === 3, "a2's log");

    expect(errors).toEqual([]);
    expect(s.state.Actors["a2"]).toBeDefined();
    expect(s.state.Actors["a1"]).toBeUndefined();
    expect(s.state.Tokens["t1"]).toBeUndefined();
    s.close();
  } finally {
    gw.stop();
  }
});

const watchable = (...ids: string[]) => ({ watchableChanged: { actors: ids.map((actorId) => ({ actorId, name: actorId.toUpperCase() })) } });

test("the set survives a switch", async () => {
  const gw = gatewayServing([watchable("a1", "a2"), viewpoint("a1"), ...world, viewpoint("a2"), ...a2Log]);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.viewpoint === "a2" && s.events.length === 3, "a2's log");
    expect(s.watchable.map((a) => a.actorId)).toEqual(["a1", "a2"]);
    s.close();
  } finally {
    gw.stop();
  }
});

test("a new set replaces the last", async () => {
  const gw = gatewayServing([watchable("a1", "a2"), viewpoint("a1"), ...world, watchable("a1")]);
  try {
    const s = new Session(gw.url, "tok");
    let sets = 0;
    s.onChange(() => (sets = s.watchable.length === 1 ? sets + 1 : sets));
    await s.start();
    await until(() => sets > 0, "the second set");
    expect(s.watchable.map((a) => [a.actorId, a.name])).toEqual([["a1", "A1"]]);
    expect(s.events).toHaveLength(4);
    s.close();
  } finally {
    gw.stop();
  }
});

// VTT-394
test("a ViewpointChanged naming nobody empties what was folded", async () => {
  const gw = gatewayServing([viewpoint("a1"), ...world, viewpoint("")]);
  try {
    const s = new Session(gw.url, "tok");
    let painted = -1;
    s.onChange(() => (painted = s.events.length));
    await s.start();
    await until(() => s.viewpoint === "" && painted === 0, "the switch to nobody");

    expect(s.events).toHaveLength(0);
    expect(s.state.Scenes["s1"]).toBeUndefined();
    s.close();
  } finally {
    gw.stop();
  }
});

// VTT-396
test("a ViewpointChanged naming the log held keeps it", async () => {
  const moved = { event: env(5, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", sceneId: "s1", to: { x: 3, y: 3 } }) }) };
  const gw = gatewayServing([viewpoint("a1"), ...world, viewpoint("a1"), moved]);
  try {
    const s = new Session(gw.url, "tok");
    const errors: string[] = [];
    s.onError((e) => errors.push(e.message));
    await s.start();
    await until(() => s.head === 5n, "the entry after the second ViewpointChanged");

    expect(errors).toEqual([]);
    expect(s.events).toHaveLength(5);
    expect(s.state.Tokens["t1"]).toMatchObject({ X: 3, Y: 3 });
    s.close();
  } finally {
    gw.stop();
  }
});

// VTT-395
test("an entry of sequence 1 starts the held log again", async () => {
  const gw = gatewayServing([viewpoint("a1"), ...world, ...world.slice(0, 2)]);
  try {
    const s = new Session(gw.url, "tok");
    const errors: string[] = [];
    s.onError((e) => errors.push(e.message));
    let emptied = false;
    s.onChange(() => (emptied ||= s.events.length === 0));
    await s.start();
    await until(() => emptied && s.events.length === 2, "the log from its first entry again");

    expect(errors).toEqual([]);
    expect(s.head).toBe(2n);
    expect(s.state.Scenes["s1"]).toBeDefined();
    expect(s.state.Tokens["t1"]).toBeUndefined();
    s.close();
  } finally {
    gw.stop();
  }
});

// VTT-394
test("a reset clears the fold failure", async () => {
  const gw = gatewayServing([
    viewpoint("a1"),
    { event: env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "A" }) }) },
    { event: env(2, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "B" }) }) },
    viewpoint("a2"),
    ...a2Log,
  ]);
  try {
    const s = new Session(gw.url, "tok");
    let failed = "";
    s.onError((e) => (failed = e.message));
    await s.start();
    await until(() => failed !== "" && s.viewpoint === "a2" && s.events.length === 3, "the failure, then a2's log");

    expect(s.failure).toBe("");
    expect(s.state.Actors["a2"]).toBeDefined();
    s.close();
  } finally {
    gw.stop();
  }
});
