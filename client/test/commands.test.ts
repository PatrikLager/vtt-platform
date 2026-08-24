import { test, expect } from "bun:test";
import { toJson } from "@bufbuild/protobuf";
import { ClientCommandSchema, JoinDoor, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  moveToken, useAbility, addNarration, upsertNote,
  setJoinDoor, rotateJoinLink, promoteParticipant, setViewpoint,
} from "../src/commands";

// Commands are asserted against the COMMITTED protojson fixtures in
// contract/testdata — the same files the Go and TS contract round-trip tests
// already use. That makes this a cross-language check rather than a
// self-consistency one: if the client's shape drifts from what the server
// parses, this fails, and no amount of "it works in my browser" hides it.
function fixture(name: string): Record<string, unknown> {
  return JSON.parse(
    readFileSync(join(import.meta.dir, "../../contract/testdata", name), "utf8"),
  ) as Record<string, unknown>;
}

/** Compare ignoring request_id, which is minted per send. */
function sameShape(got: Record<string, unknown>, want: Record<string, unknown>) {
  const { requestId: _g, ...g } = got;
  const { requestId: _w, ...w } = want;
  expect(g).toEqual(w);
}

test("moveToken matches the committed client_command fixture", () => {
  const cmd = moveToken("tok-ursus", { x: 5, y: 8 }, "charging the goblin line");
  sameShape(toJson(ClientCommandSchema, cmd) as Record<string, unknown>, fixture("client_command.json"));
});

test("useAbility matches the committed fixture, targets included", () => {
  const cmd = useAbility("tok-ursus", "reckless-strike", ["tok-goblin-2", "tok-goblin-3"]);
  sameShape(
    toJson(ClientCommandSchema, cmd) as Record<string, unknown>,
    fixture("use_ability_command.json"),
  );
});

test("upsertNote matches the committed fixture", () => {
  const f = fixture("upsert_note_command.json") as { upsertNote: Record<string, string> };
  const cmd = upsertNote(f.upsertNote["key"]!, f.upsertNote["title"]!, f.upsertNote["text"]!);
  sameShape(toJson(ClientCommandSchema, cmd) as Record<string, unknown>, f as never);
});

test("an omitted move reason is absent from the wire, not an empty string", () => {
  // protojson omits empty scalars, and sending `"reason": ""` would be a
  // different shape than the server's own fixtures show.
  const json = toJson(ClientCommandSchema, moveToken("t1", { x: 1, y: 1 })) as Record<string, any>;
  expect(json["moveToken"]).not.toHaveProperty("reason");
});

test("narration without a speaker omits `as`, and with one carries it", () => {
  const plain = toJson(ClientCommandSchema, addNarration("Table talk.")) as Record<string, any>;
  expect(plain["addNarration"]).not.toHaveProperty("as");

  const spoken = toJson(ClientCommandSchema, addNarration("You'll not pass!", "Goblin Cutter")) as Record<string, any>;
  expect(spoken["addNarration"]["as"]).toBe("Goblin Cutter");
});

test("an anchored narration sends both ends as strings (int64 on the wire)", () => {
  // int64 is a STRING in protojson. Sending numbers would be rejected by the
  // server's decoder, and only a fixture-shaped assertion catches it.
  const json = toJson(ClientCommandSchema, addNarration("Anchored.", "", [4, 6])) as Record<string, any>;
  expect(json["addNarration"]["anchorFromSeq"]).toBe("4");
  expect(json["addNarration"]["anchorToSeq"]).toBe("6");
});

test("every constructed command carries a request id", () => {
  // Without one the result is uncorrelatable and Wire.send never resolves.
  for (const cmd of [
    moveToken("t1", { x: 0, y: 0 }),
    useAbility("a1", "ab", ["t2"]),
    addNarration("x"),
    upsertNote("k", "t", "x"),
  ]) {
    expect(cmd.requestId).not.toBe("");
  }
});

test("request ids are unique across commands", () => {
  const ids = new Set([
    moveToken("t1", { x: 0, y: 0 }).requestId,
    moveToken("t1", { x: 0, y: 0 }).requestId,
    moveToken("t1", { x: 0, y: 0 }).requestId,
  ]);
  expect(ids.size).toBe(3);
});

// --- DM commands (T8) -------------------------------------------------------

import {
  startSession, endSession, createScene, addActor, placeToken,
  loadAdventure, deleteNote, removeCondition, retractEvents, parseActorJSON,
} from "../src/commands";

test("loadAdventure matches the committed fixture", () => {
  sameShape(
    toJson(ClientCommandSchema, loadAdventure("goblin-ambush")) as Record<string, unknown>,
    fixture("load_adventure_command.json"),
  );
});

test("session start carries the name; end carries nothing", () => {
  const s = toJson(ClientCommandSchema, startSession("Night One")) as Record<string, any>;
  expect(s["startSession"]["name"]).toBe("Night One");
  const e = toJson(ClientCommandSchema, endSession()) as Record<string, any>;
  expect(e).toHaveProperty("endSession");
});

test("createScene and placeToken carry their geometry", () => {
  const sc = toJson(ClientCommandSchema, createScene("s1", "Hall", 10, 8)) as Record<string, any>;
  expect(sc["createScene"]).toMatchObject({ sceneId: "s1", name: "Hall", gridWidth: 10, gridHeight: 8 });

  const pt = toJson(ClientCommandSchema, placeToken("t1", "s1", "a1", { x: 2, y: 3 })) as Record<string, any>;
  expect(pt["placeToken"]).toMatchObject({ tokenId: "t1", sceneId: "s1", actorId: "a1" });
  expect(pt["placeToken"]["position"]).toMatchObject({ x: 2, y: 3 });
});

test("a zero position is still sent as a position, not omitted", () => {
  // protojson omits empty MESSAGES too. Placing at 0,0 must still carry a
  // position object, or the server sees "no position" and rejects the place.
  const pt = toJson(ClientCommandSchema, placeToken("t1", "s1", "a1", { x: 0, y: 0 })) as Record<string, any>;
  expect(pt["placeToken"]).toHaveProperty("position");
});

test("retractEvents sends an inclusive range as strings", () => {
  const r = toJson(ClientCommandSchema, retractEvents(4n, 6n, "undo")) as Record<string, any>;
  expect(r["retractEvents"]).toMatchObject({ fromSequence: "4", toSequence: "6", reason: "undo" });
});

test("retracting a single event uses the same sequence for both ends", () => {
  const r = toJson(ClientCommandSchema, retractEvents(7n, 7n, "undo")) as Record<string, any>;
  expect(r["retractEvents"]["fromSequence"]).toBe("7");
  expect(r["retractEvents"]["toSequence"]).toBe("7");
});

test("removeCondition names the actor and the condition", () => {
  const c = toJson(ClientCommandSchema, removeCondition("a1", "dazed")) as Record<string, any>;
  expect(c["removeCondition"]).toMatchObject({ actorId: "a1", conditionId: "dazed" });
});

test("deleteNote carries the key", () => {
  const d = toJson(ClientCommandSchema, deleteNote("kobold-den")) as Record<string, any>;
  expect(d["deleteNote"]["key"]).toBe("kobold-den");
});

// --- raw-JSON actor paste ---------------------------------------------------

test("a pasted actor is parsed into an addActor command", () => {
  const cmd = parseActorJSON(
    '{"actorId":"a1","name":"Lera","kind":"ACTOR_KIND_PARTY_MEMBER","attributes":{"brawn":3}}');
  expect(cmd).not.toBeInstanceOf(Error);
  const json = toJson(ClientCommandSchema, cmd as never) as Record<string, any>;
  expect(json["addActor"]["actor"]).toMatchObject({ actorId: "a1", name: "Lera" });
  expect(json["addActor"]["actor"]["attributes"]).toMatchObject({ brawn: 3 });
});

test("malformed JSON returns an explanatory Error rather than throwing", () => {
  // The DM is pasting by hand. A thrown exception blanks the console; a
  // returned error can be shown next to the box they are typing in.
  const err = parseActorJSON("{not json");
  expect(err).toBeInstanceOf(Error);
});

test("valid JSON that is not an actor is rejected before it reaches the wire", () => {
  // An actor with no id is refused by the server; catching it here tells the
  // DM which field is wrong instead of surfacing a generic rejection.
  const err = parseActorJSON('{"name":"No Id"}');
  expect(err).toBeInstanceOf(Error);
  expect((err as Error).message).toMatch(/actorId/i);
});

test("a pasted actor that does not say what it is names the field, not the wire", () => {
  // The paste path is the third way a DM creates an actor, and the server
  // refuses a kindless one exactly as it refuses a kindless form submission.
  // Catching it here names "kind" and offers the two values; the server's
  // rejection would arrive after the DM had moved on.
  const err = parseActorJSON('{"actorId":"a1","name":"Lera"}') as Error;
  expect(err).toBeInstanceOf(Error);
  expect(err.message).toMatch(/kind/i);
  expect(err.message).toContain("ACTOR_KIND_PARTY_MEMBER");
  expect(err.message).toContain("ACTOR_KIND_NON_PARTY");
});

test("a pasted actor that says what it is reaches the wire intact", () => {
  const cmd = parseActorJSON('{"actorId":"a1","name":"Lera","kind":"ACTOR_KIND_PARTY_MEMBER"}');
  expect(cmd).not.toBeInstanceOf(Error);
  const json = toJson(ClientCommandSchema, cmd as never) as Record<string, any>;
  expect(json["addActor"]["actor"]["kind"]).toBe("ACTOR_KIND_PARTY_MEMBER");
});

test("a pasted actor answers about its controller before its kind, as the server does", () => {
  // Review finding, 2026-08-24. The server checks controller-before-kind on
  // purpose (gateway validateAddActor): "creation does not confer control" is a
  // misunderstanding of the model, and a caller told to add a kind first would
  // add one and resend the same forbidden shape. This file answered in the
  // OPPOSITE order, so the DM paid exactly the round trip that ordering exists
  // to prevent — locally about the kind, then over the wire about the
  // controller.
  const err = parseActorJSON('{"actorId":"a1","name":"Lera","controllerId":"p-2"}') as Error;
  expect(err).toBeInstanceOf(Error);
  expect(err.message).toMatch(/grant_actor_control/);
  // And it must not be the kind message, even though the kind is absent too.
  expect(err.message).not.toMatch(/"kind"/);
});

test("an unknown field in a pasted actor is rejected, not silently dropped", () => {
  // Silently dropping it would let a DM believe they set something they did
  // not — the same reason the server's own decoder is strict.
  const err = parseActorJSON('{"actorId":"a1","hitPoints":10}');
  expect(err).toBeInstanceOf(Error);
});

test("addActor states what the actor IS, and the kind reaches the wire", () => {
  // Actor-kind Task 7. The server refuses an add_actor that states no kind
  // (gateway validateAddActor), so a builder that could not express one would
  // put a bounced command on every caller's path — the same argument that made
  // grantActorControl's `kind` a required parameter rather than an optional
  // one with a default.
  //
  // BOTH VALUES, because a builder that hardwired either would pass a
  // single-valued assertion while making half the callers wrong.
  const pc = toJson(ClientCommandSchema, addActor("a1", "Lera", ActorKind.PARTY_MEMBER)) as Record<string, any>;
  expect(pc["addActor"]["actor"]["kind"]).toBe("ACTOR_KIND_PARTY_MEMBER");
  const npc = toJson(ClientCommandSchema, addActor("a2", "Goblin", ActorKind.NON_PARTY)) as Record<string, any>;
  expect(npc["addActor"]["actor"]["kind"]).toBe("ACTOR_KIND_NON_PARTY");
});

test("addActor cannot confer control, whatever a caller passes it", () => {
  // Visibility spec §5.1: control is conferred exactly once, by a grant that
  // says what it is conferring. The builder used to take an optional
  // `controllerId` and put it on the wire, which created a party member with
  // no kind stated and no refusal on the path.
  //
  // The SECOND assertion is the load-bearing one. Dropping the parameter from
  // the signature is a TypeScript fact, and `bun test` runs JavaScript — a
  // builder that still spread a third argument onto the actor would pass a
  // shape assertion written against a two-argument call and fail here. The
  // cast is what lets a JS caller try what the type system has stopped
  // forbidding, which is exactly the caller the server's refusal exists for.
  const npc = toJson(ClientCommandSchema, addActor("a1", "Goblin", ActorKind.NON_PARTY)) as Record<string, any>;
  expect(npc["addActor"]["actor"]).not.toHaveProperty("controllerId");
  expect(npc["addActor"]["actor"]).not.toHaveProperty("controllerIds");

  const smuggle = addActor as unknown as (...args: unknown[]) => ClientCommand;
  const pc = toJson(ClientCommandSchema, smuggle("a2", "Lera", ActorKind.PARTY_MEMBER, "p-1")) as Record<string, any>;
  expect(pc["addActor"]["actor"]).not.toHaveProperty("controllerId");
  expect(pc["addActor"]["actor"]).not.toHaveProperty("controllerIds");
});

// --- the two rejection messages, and the id fallback -------------------------

test("malformed JSON is reported AS malformed JSON, not as a bad actor", () => {
  // Both failure arms return an Error, so "is an Error" cannot tell them
  // apart — and they send the DM to different places. "not valid JSON" means
  // look at the brackets; "not a valid actor" means the shape is wrong. An
  // empty catch here silently falls through to the second arm and misreports
  // every syntax error as a schema error.
  const err = parseActorJSON("{not json") as Error;
  expect(err).toBeInstanceOf(Error);
  expect(err.message).toMatch(/^not valid JSON: /);
});

test("a well-formed non-actor is reported as a bad actor, and says why", () => {
  // The other arm, asserted the same way: the underlying decoder's complaint
  // is preserved after the prefix, because "not a valid actor" alone does not
  // say WHICH field offended.
  const err = parseActorJSON('{"actorId":"a1","hitPoints":10}') as Error;
  expect(err).toBeInstanceOf(Error);
  expect(err.message).toMatch(/^not a valid actor: /);
  expect(err.message.length).toBeGreaterThan("not a valid actor: ".length);
});

test("request ids stay unique when crypto.randomUUID is unavailable", () => {
  // The fallback exists for non-browser embedders, and nothing ever ran it —
  // every test executes under Bun, where crypto is present. Uniqueness is the
  // whole contract: Wire keys its pending map by request id, so a collision
  // resolves the WRONG caller's promise, which is a bug that presents as one
  // command's result appearing under another command.
  const real = globalThis.crypto;
  try {
    // @ts-expect-error deliberately removing a global to reach the fallback
    delete globalThis.crypto;
    const ids = [
      moveToken("t1", { x: 0, y: 0 }).requestId,
      moveToken("t1", { x: 0, y: 0 }).requestId,
      moveToken("t1", { x: 0, y: 0 }).requestId,
    ];
    expect(new Set(ids).size).toBe(3);
    for (const id of ids) {
      // `req-` then base36 time, then base36 counter. Pinned as a shape
      // because a counter running the wrong way injects a second `-` and
      // makes the two components ambiguous to anything that splits on it.
      expect(id).toMatch(/^req-[0-9a-z]+-[0-9a-z]+$/);
    }
  } finally {
    Object.defineProperty(globalThis, "crypto", { value: real, configurable: true, writable: true });
  }
});

test("crypto.randomUUID is used when it IS available", () => {
  // The other side of the same branch. Without this, a condition forced to
  // always-fallback looks identical to correct behaviour, and the platform
  // UUID — the thing that makes collisions negligible rather than merely
  // unlikely — silently stops being used.
  const real = globalThis.crypto;
  try {
    Object.defineProperty(globalThis, "crypto", {
      value: { randomUUID: () => "FIXED-UUID" },
      configurable: true,
      writable: true,
    });
    expect(moveToken("t1", { x: 0, y: 0 }).requestId).toBe("req-FIXED-UUID");
  } finally {
    Object.defineProperty(globalThis, "crypto", { value: real, configurable: true, writable: true });
  }
});

test("the door commands name themselves and carry an EXPLICIT open/closed", () => {
  // The enum is the point. protojson omits zero values, so a bool field would
  // put CLOSED on the wire as an absent field — making "shut the door"
  // indistinguishable from a client that forgot to say. The server refuses
  // UNSPECIFIED rather than guessing, so the builder must never produce it.
  const open = setJoinDoor(true);
  expect(open.command.case).toBe("setJoinDoor");
  expect(open.command.value).toMatchObject({ door: JoinDoor.OPEN });

  const shut = setJoinDoor(false);
  expect(shut.command.case).toBe("setJoinDoor");
  expect(shut.command.value).toMatchObject({ door: JoinDoor.CLOSED });
  // Asserted explicitly: OPEN and CLOSED must differ, and neither may be the
  // zero value. A builder that produced UNSPECIFIED for one of them would be
  // refused by the server every time, and the console would look broken.
  expect(JoinDoor.CLOSED).not.toBe(JoinDoor.UNSPECIFIED);
  expect(JoinDoor.OPEN).not.toBe(JoinDoor.UNSPECIFIED);
});

test("rotating the link is its own command, carrying nothing", () => {
  const cmd = rotateJoinLink();
  expect(cmd.command.case).toBe("rotateJoinLink");
  expect(cmd.requestId).not.toBe("");
});

test("a perch names the shoulder the spectator chose", () => {
  // The client half of set_viewpoint, which shipped without one: the command
  // reached the contract, MayPerch and the projection in this arc's Tasks 2-6,
  // and no control could issue it — `viewpoint` appeared exactly once in
  // client/src, in a comment.
  const cmd = setViewpoint("act-fighter");
  expect(cmd.command.case).toBe("setViewpoint");
  expect(cmd.command.value).toMatchObject({ actorId: "act-fighter" });
  expect(cmd.requestId).not.toBe("");
});

test("the empty actor id is a COMMAND, not a silence", () => {
  // "Naming no actor is how a bird LEAVES a shoulder without immediately
  // sitting on another" (internal/gateway/viewpoint.go). It is a real state
  // the server accepts and acts on, so the builder must produce a real
  // command for it — a builder that answered null, or that skipped the send,
  // would leave a spectator with no way to stop seeing.
  //
  // On the wire the empty string VANISHES, because protojson omits empty
  // scalars. That is the shape the server reads as "un-perch": what carries
  // the meaning is the PRESENCE of the set_viewpoint arm, not a field inside
  // it. Both halves are asserted, because an implementation that dropped the
  // arm as well would look identical from inside the built object.
  const cmd = setViewpoint("");
  expect(cmd.command.case).toBe("setViewpoint");
  const json = toJson(ClientCommandSchema, cmd) as Record<string, any>;
  expect(json).toHaveProperty("setViewpoint");
  expect(json["setViewpoint"]).not.toHaveProperty("actorId");
});

test("promotion names the participant and the role", () => {
  // The client half of promote_participant, which shipped without one: the
  // command reached the contract, authz and the MCP tool list in J3 and no
  // console could issue it.
  const cmd = promoteParticipant("p-7", "player");
  expect(cmd.command.case).toBe("promoteParticipant");
  expect(cmd.command.value).toMatchObject({ participantId: "p-7", role: "player" });
});
