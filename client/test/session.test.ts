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
