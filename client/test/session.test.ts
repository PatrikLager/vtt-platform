import { test, expect } from "bun:test";
import { create, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  SessionStartedSchema,
  SceneCreatedSchema,
  ActorAddedSchema,
  TokenPlacedSchema,
  TokenMovedSchema,
  EventsRetractedSchema,
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

test("a retraction arriving later UNDOES an earlier event", async () => {
  // This is why the session re-folds the whole stream rather than applying
  // each event to the previous state: a retraction invalidates history that
  // was already applied, and no incremental apply can walk that back. If this
  // ever regresses, a DM's undo would appear to work on the server and not on
  // the player's screen.
  const gw = gatewayServing([
    ...world,
    { event: env(5, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", to: { x: 5, y: 5 } }) }) },
    { event: env(6, { case: "eventsRetracted", value: create(EventsRetractedSchema, { fromSequence: 5n, toSequence: 5n, reason: "undo" }) }) },
  ]);
  try {
    const s = new Session(gw.url, "tok");
    await s.start();
    await until(() => s.head === 6n, "the replay to reach sequence 6");

    expect(s.head).toBe(6n);
    // Back at its placed position, not the retracted move's destination.
    expect(s.state.Tokens["t1"]).toMatchObject({ X: 0, Y: 0 });
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
  // this a resume rather than a fresh replay.
  //
  // The fake gateway REPLAYS BY CURSOR rather than scripting each connection,
  // because the cursor is now the thing under test: a reconnect resumes one
  // sequence before the highest seen (wire.ts's replay-cursor note), so a
  // server that ignored `after` would hide both halves of that — the sequence
  // rolled off the log and the same sequence arriving again.
  const seen: string[] = [];
  let opened = 0;
  let pendingAfter = 0n;
  const all = [
    ...world,
    { event: env(5, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", to: { x: 3, y: 3 } }) }) },
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
    // Two connections, and the second asked to resume rather than replay —
    // from 3, one sequence below the 4 it had folded, because sequence 4 may
    // have been a torn batch and is taken again whole.
    expect(seen.length).toBe(2);
    expect(seen[0]).toBe("0");
    expect(seen[1]).toBe("3");
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

test("a reconnect after a torn batch takes that sequence again, and folds it exactly once", async () => {
  // THE CASE THE PROJECTION CREATED. Since internal/gateway projects per seat,
  // one log event can reach a player as SEVERAL envelopes sharing one
  // sequence — something coming into view is an ActorAdded plus a
  // TokenPlaced. A socket that dies between them leaves this client holding
  // half of sequence 4.
  //
  // Both of the obvious cursors are wrong, which is why this test exists.
  // after=4 asks the server for events strictly above 4, so the TokenPlaced is
  // never sent again and the token is silently missing forever — and when the
  // torn envelope is a TokenHidden instead, the thing left behind is an enemy
  // token on a player's board, which is the leak this whole arc closes.
  // after=3 without dropping the half-batch first would fold the ActorAdded
  // twice, and a duplicate actor is a fold error, which Session turns into a
  // permanently frozen board.
  let opened = 0;
  let pendingAfter = 0n;
  const before = [
    { event: env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }) },
    { event: env(2, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "scn", name: "Cave", gridWidth: 8, gridHeight: 8 }) }) },
    { event: env(3, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "a1", name: "Hero" } }) }) },
  ];
  // One log event, two envelopes, one sequence: the goblin coming into view.
  const batch = [
    { event: env(4, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "a2", name: "Goblin" } }) }) },
    { event: env(4, { case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: "t2", sceneId: "scn", actorId: "a2", position: { x: 6, y: 6 } }) }) },
  ];
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      pendingAfter = BigInt(new URL(req.url).searchParams.get("after") ?? "0");
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open(ws) {
        opened += 1;
        const after = pendingAfter;
        for (const f of [...before, ...batch]) {
          if (BigInt((f as any).event.sequence) <= after) continue;
          // THE TEAR: on the first connection the socket dies after the first
          // envelope of sequence 4.
          if (opened === 1 && (f as any).event.sequence === "4" && (f as any).event.tokenPlaced) break;
          ws.send(JSON.stringify(f));
        }
      },
      message() {},
    },
  });
  const url = `ws://localhost:${server.port}/ws`;
  try {
    const s = new Session(url, "tok");
    const errors: string[] = [];
    s.onError((e) => errors.push(e.message));
    await s.start();
    await until(() => s.head === 4n, "the torn batch's first envelope");
    expect(s.state.Actors["a2"]).toBeDefined();
    expect(s.state.Tokens["t2"]).toBeUndefined(); // the tear, before recovery

    await s.reconnect();
    await until(() => s.state.Tokens["t2"] !== undefined, "the re-sent batch to complete the sighting");

    expect(errors).toEqual([]); // a duplicate ActorAdded would be here
    expect(s.state.Actors["a2"]).toBeDefined();
    expect(s.state.Tokens["t2"]).toMatchObject({ X: 6, Y: 6 });
    s.close();
  } finally {
    server.stop(true);
  }
});

test("a redial that drops nothing does not repaint", async () => {
  // The rollback is a no-op WHENEVER the resume cursor already sits at or above
  // everything held, and that is not an exotic case: it is the state of a
  // spectator who has perched and seen nothing else happen. Perch frames carry
  // sequence 0 deliberately (the gateway's perchSequence), 0 never advances the
  // high-water mark, so a redial resumes at 0, keeps the whole log, and has
  // nothing to say about it.
  //
  // Saying it anyway is not free. onChange is what every view redraws on, and a
  // redraw the state did not earn throws away whatever the DOM was holding —
  // the same hazard the DM console's draft buffer exists for, one layer down.
  // "Nothing changed" must therefore be silent, not merely correct.
  //
  // The DISCRIMINATOR is the count, not the state: the log and the board are
  // identical either way, because re-folding an untruncated log reproduces
  // exactly what was already there. Only the notification tells the two apart.
  const perchFrame = {
    event: env(0, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "perched", name: "P", gridWidth: 2, gridHeight: 2 }) }),
  };
  let opened = 0;
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open(ws) {
        opened += 1;
        // Only the first connection speaks. A second replay would fold a
        // duplicate scene and notify for a reason this test is not about.
        if (opened === 1) ws.send(JSON.stringify(perchFrame));
      },
      message() {},
    },
  });
  try {
    const s = new Session(`ws://localhost:${server.port}/ws`, "tok");
    await s.start();
    await until(() => s.state.Scenes["perched"] !== undefined, "the perch frame");

    let changes = 0;
    s.onChange(() => changes++);
    await s.reconnect();

    expect(changes).toBe(0);
    // And the frame the rollback had no reason to drop is still here: 0 is at
    // or below every cursor, so a redial keeps it. Emptying is restart()'s job.
    expect(s.events).toHaveLength(1);
    expect(s.state.Scenes["perched"]).toBeDefined();
    s.close();
  } finally {
    server.stop(true);
  }
});

// --- restart: the redial a spectator's perch needs ---------------------------

test("restart dials from zero and drops the whole log, perch frames and all", async () => {
  // A PERCH IS CONNECTION STATE (visibility spec §3.1.1). The server's
  // projector is reborn perched on nobody, so a redial that RESUMES leaves
  // this client holding a board the new connection knows nothing about, and
  // the next perch re-introduces every scene, actor and token in it — the
  // duplicate-introduction freeze, measured against the real gateway.
  //
  // reconnect() cannot serve here for two separate reasons, and this test
  // discriminates on both. Its cursor is seenSeq-1, not 0; and its rollback
  // keeps everything AT OR BELOW that cursor, so the sequence-0 frames a perch
  // produces — stamped 0 deliberately, so that no undo can name one — survive
  // a rollback that drops ordinary ones.
  const seen: string[] = [];
  let opened = 0;
  let pendingAfter = 0n;
  // What a perch delivers: a scene the LOG does not contain, at sequence 0 —
  // sent on the connection that asked for it and never replayed to another,
  // exactly as the gateway's perch frames are (internal/gateway/project.go's
  // perchSequence, and seat.perch, whose output skips the replay filter).
  const perchFrame = {
    event: env(0, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "perched", name: "P", gridWidth: 2, gridHeight: 2 }) }),
  };
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
        opened += 1;
        const after = pendingAfter;
        for (const f of world) {
          if (BigInt((f as any).event.sequence) > after) ws.send(JSON.stringify(f));
        }
        if (opened === 1) ws.send(JSON.stringify(perchFrame));
      },
      message() {},
    },
  });
  try {
    const s = new Session(`ws://localhost:${server.port}/ws`, "tok");
    await s.start();
    await until(() => s.state.Scenes["perched"] !== undefined, "the first connection's replay and its perch");

    const errors: string[] = [];
    s.onError((e) => errors.push(e.message));
    // Every log length a view was told about. The emptying must NOTIFY, not
    // just mutate: a spectator who redials with no shoulder sends nothing
    // afterwards, so nothing else repaints until the replay lands and the old
    // board would sit there in the meantime, behind a connection that no
    // longer holds it.
    const painted: number[] = [];
    s.onChange(() => painted.push(s.events.length));
    await s.restart();
    await until(() => s.events.length === 4, "the whole log again");
    expect(painted).toContain(0);

    expect(seen).toEqual(["0", "0"]);
    // The perch frame is GONE, not re-folded: it came from a connection that
    // no longer exists, and the server will re-send its own if asked again.
    expect(s.state.Scenes["perched"]).toBeUndefined();
    expect(s.state.Tokens["t1"]).toMatchObject({ SceneID: "s1", X: 0, Y: 0 });
    // A kept log would have made the whole replay a duplicate introduction.
    expect(errors).toEqual([]);
    expect(s.events).toHaveLength(4);
    s.close();
  } finally {
    server.stop(true);
  }
});

test("a restart forgets the cursors, so the next redial cannot resume from a log this client no longer holds", async () => {
  // seenSeq is the high-water mark a redial derives its resume point from. A
  // restart empties the log, so leaving that mark standing would have the NEXT
  // reconnect ask for events after a sequence this client has never folded —
  // and the events between are silently never sent, which is the leak
  // direction (a lost TokenHidden leaves an enemy token on the board).
  //
  // The discriminator is a second connection that replays NOTHING: with the
  // cursors reset the third dial asks for 0, and without them it asks for 3.
  const seen: string[] = [];
  let opened = 0;
  const server = Bun.serve({
    port: 0,
    fetch(req, srv) {
      seen.push(new URL(req.url).searchParams.get("after") ?? "");
      if (srv.upgrade(req)) return undefined;
      return new Response("expected websocket", { status: 400 });
    },
    websocket: {
      open(ws) {
        opened += 1;
        if (opened > 1) return; // the campaign is gone; nothing to replay
        for (const f of world) ws.send(JSON.stringify(f));
      },
      message() {},
    },
  });
  try {
    const s = new Session(`ws://localhost:${server.port}/ws`, "tok");
    await s.start();
    await until(() => s.head === 4n, "the first connection's replay");

    await s.restart();
    await s.reconnect();

    expect(seen).toEqual(["0", "0", "0"]);
    s.close();
  } finally {
    server.stop(true);
  }
});

test("a restart that cannot dial leaves the board standing", async () => {
  // The cost of restart() over reconnect() is what it throws away, so what it
  // does on FAILURE is not a detail. A watcher clicking Reconnect while the
  // network is still down would otherwise trade a stale board for a blank
  // page, and get no replay to fill it.
  const gw = gatewayServing(world);
  const s = new Session(gw.url, "tok");
  try {
    await s.start();
    await until(() => s.events.length === 4, "the replay");
    gw.stop(); // the table is unreachable now

    await expect(s.restart()).rejects.toThrow();

    expect(s.events).toHaveLength(4);
    expect(s.state.Tokens["t1"]).toMatchObject({ SceneID: "s1", X: 0, Y: 0 });
  } finally {
    s.close();
  }
});
