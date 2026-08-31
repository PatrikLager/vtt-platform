// Command construction.
//
// Not every builder here carries the same strength of proof. Four —
// moveToken, useAbility, upsertNote, loadAdventure — are asserted against
// COMMITTED protojson fixtures in contract/testdata (client/test/
// commands.test.ts), the same files the Go and TS contract round-trip tests
// use. That makes THEIR shape a cross-language claim: if the client's idea
// of one of those four drifts from what the server parses, a test fails
// instead of a player getting a rejection nobody can explain.
//
// The rest rest on weaker ground: a literal shape written by hand in the
// same test file (self-consistency — it catches the builder disagreeing
// with itself, not with the server), or a check further away still, such as
// a view test that only asserts which command.case was sent. openDoor,
// closeDoor and loadMap (Task 1, 2026-08-30) are in the self-consistency
// camp, not the fixture one — command-surface.test.ts only proves a
// same-named function exists, and commands.test.ts checks their wire shape
// against a literal this file's own author wrote, which is worth widening
// to a real fixture if these three ever need the cross-language guarantee.
//
// Two protojson facts these builders exist to get right:
//   * int64 fields travel as STRINGS. anchor_from_seq must be "4", not 4.
//   * empty scalars are OMITTED. A move with no reason must not carry
//     `"reason": ""`, which is a different shape than the server's fixtures.
// The generated types handle both, which is precisely why commands are built
// through them rather than as hand-written object literals.

import { create } from "@bufbuild/protobuf";
import {
  ClientCommandSchema,
  JoinDoor,
  type ClientCommand,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind, ActorSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fromJson } from "@bufbuild/protobuf";

export interface Point {
  x: number;
  y: number;
}

let counter = 0;

/**
 * A per-send unique request id. Correlation depends on uniqueness: Wire keys
 * its pending map by this, so a duplicate would resolve the wrong caller's
 * promise. crypto.randomUUID where available, with a monotonic counter as the
 * fallback so a non-browser embedder still gets distinct ids.
 */
function requestId(): string {
  const rand =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `${Date.now().toString(36)}-${(counter++).toString(36)}`;
  return `req-${rand}`;
}

export function moveToken(tokenId: string, to: Point, reason?: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: {
      case: "moveToken",
      value: { tokenId, to, ...(reason ? { reason } : {}) },
    },
  });
}

export function useAbility(
  actorId: string,
  abilityId: string,
  targetIds: string[],
): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "useAbility", value: { actorId, abilityId, targetIds } },
  });
}

/**
 * addNarration builds a story entry. `as` is the in-character speaker and is
 * omitted when empty; anchor is a BACKWARD-pointing inclusive range naming
 * the events this narration describes.
 */
export function addNarration(
  text: string,
  as?: string,
  anchor?: [number, number],
): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: {
      case: "addNarration",
      value: {
        text,
        ...(as ? { as } : {}),
        ...(anchor ? { anchorFromSeq: BigInt(anchor[0]), anchorToSeq: BigInt(anchor[1]) } : {}),
      },
    },
  });
}

export function upsertNote(key: string, title: string, text: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "upsertNote", value: { key, title, text } },
  });
}


// --- DM commands ------------------------------------------------------------

export function startSession(name: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "startSession", value: { name } },
  });
}

export function endSession(): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "endSession", value: {} },
  });
}

export function createScene(
  sceneId: string,
  name: string,
  gridWidth: number,
  gridHeight: number,
): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "createScene", value: { sceneId, name, gridWidth, gridHeight } },
  });
}

export function placeToken(
  tokenId: string,
  sceneId: string,
  actorId: string,
  position: Point,
): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    // `position` is always set, even at 0,0: protojson omits empty MESSAGES
    // as well as empty scalars, and a place with no position is rejected.
    command: { case: "placeToken", value: { tokenId, sceneId, actorId, position } },
  });
}

/**
 * Take a token off the board, for good (retraction-leaves spec §5.1: "takes
 * a piece off the board"). NOT the same thing as a token going out of sight —
 * that is a viewer-side TokenHidden the server projects; this appends a real
 * TokenRemoved that removes the piece for everyone, permanently. DM/agent
 * only (see internal/gateway/authz.go's commandRoles "remove_token" row).
 */
export function removeToken(tokenId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "removeToken", value: { tokenId } },
  });
}

export function loadAdventure(adventureId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "loadAdventure", value: { adventureId } },
  });
}

export function deleteNote(key: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "deleteNote", value: { key } },
  });
}

export function removeCondition(actorId: string, conditionId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "removeCondition", value: { actorId, conditionId } },
  });
}

/**
 * parseActorJSON turns a DM's pasted actor into an addActor command.
 *
 * Returns an Error rather than throwing: the DM is typing into a box, and a
 * thrown exception blanks the console while a returned error can be shown
 * beside what they are editing.
 *
 * Validation is deliberately STRICT — fromJson rejects unknown fields — for
 * the same reason the server's decoder is. Silently dropping a misspelled key
 * would let a DM believe they set something they did not, and they would only
 * discover it mid-session when the actor behaved wrongly.
 */
export function parseActorJSON(raw: string): ClientCommand | Error {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (e) {
    return new Error(`not valid JSON: ${e instanceof Error ? e.message : String(e)}`);
  }
  try {
    const actor = fromJson(ActorSchema, parsed as never);
    // The server requires an id and rejects the actor without one. Saying so
    // here names the field; the server's rejection would not.
    if (actor.actorId === "") {
      return new Error('actor is missing "actorId", which the server requires');
    }
    // A CONTROLLER IS ANSWERED FIRST, in the server's own order (gateway
    // validateAddActor), and matching it is the whole point of checking here at
    // all. "Creation does not confer control" is a misunderstanding of the
    // model rather than a forgotten field: a DM told to add a kind first would
    // add one and paste the same forbidden shape again. Answering in the other
    // order costs exactly the round trip this check exists to save.
    if (actor.controllerId !== "" || actor.controllerIds.length > 0) {
      return new Error(
        'actor declares a controller, which creating it may not do: control is conferred ' +
          "by grant_actor_control, which also says what the actor is. Paste the actor with " +
          "no controller, then grant it",
      );
    }
    // And it requires the actor to say WHAT IT IS (visibility spec §5.1,
    // actor-kind Task 7). An omitted enum arrives here as UNSPECIFIED, which
    // is indistinguishable from a DM who typed the word — which is precisely
    // why the server refuses both. The two values are named because this
    // message is the only place the DM finds out what to type.
    if (actor.kind === ActorKind.UNSPECIFIED) {
      return new Error(
        'actor is missing "kind", which the server requires: ' +
          '"ACTOR_KIND_PARTY_MEMBER" for a character the party knows about, ' +
          '"ACTOR_KIND_NON_PARTY" for a creature they must discover',
      );
    }
    return create(ClientCommandSchema, {
      requestId: requestId(),
      command: { case: "addActor", value: { actor } },
    });
  } catch (e) {
    return new Error(`not a valid actor: ${e instanceof Error ? e.message : String(e)}`);
  }
}

/**
 * addActor from an already-built actor object (the form path).
 *
 * IT TAKES NO CONTROLLER, and that is the rule rather than an omission
 * (visibility spec §5.1, Patrik's ruling 2026-08-24). Creating an actor makes
 * a character; a grant gives it a controller AND a standing, together, always.
 * This builder used to take an optional `controllerId`, and a DM who typed one
 * into the console created a PARTY MEMBER — no kind stated, no refusal
 * anywhere on the path, the whole cloned Actor on every player's roster.
 *
 * The server refuses a controller here now, so restoring the parameter would
 * produce a bounced command rather than a leak; it is gone anyway, because a
 * builder that can express a refused shape is a trap for its next caller.
 *
 * IT TAKES A KIND, and the same reasoning runs the other way. `kind` is a
 * REQUIRED PARAMETER, exactly as grantActorControl's is: the server refuses an
 * add_actor that states none (gateway validateAddActor), so making it optional
 * here would move the omission from the wire into this file, where nothing
 * checks it — the caller would get a bounced command instead of a type error.
 * Every caller must ASK, which is what dm.ts's kind selector is.
 */
export function addActor(actorId: string, name: string, kind: ActorKind): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "addActor", value: { actor: { actorId, name, kind } } },
  });
}

/**
 * Hand control of an actor to a participant (spec §5.3, dm/agent only), SAYING
 * what that actor is (visibility spec §5.1).
 *
 * Additive: the server ADDS to controller_ids rather than replacing, so
 * granting does not take the character away from whoever already holds it.
 * That is what makes a character shareable, and it is why there is no
 * "transfer" builder — a transfer is a grant and a revoke, two decisions.
 *
 * `kind` is a REQUIRED PARAMETER, not an optional one with a default, and
 * that is the whole point: the server refuses a grant that states no kind,
 * because an omitted value cannot be told from a deliberate one. Making it
 * optional here would move the omission from the wire into this file, where
 * nothing checks it — the caller would get a bounced command instead of a
 * type error. Every caller must ASK, which is what dm.ts's kind selector is.
 */
export function grantActorControl(
  actorId: string,
  participantId: string,
  kind: ActorKind,
): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "grantActorControl", value: { actorId, participantId, kind } },
  });
}

/** Take one participant's control of an actor away (spec §5.3). */
export function revokeActorControl(actorId: string, participantId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "revokeActorControl", value: { actorId, participantId } },
  });
}

/**
 * Open or close the table's shared join link (joining-a-table spec §2).
 *
 * The boolean is this function's ARGUMENT, never the wire's. protojson omits
 * zero values, so a `bool open` field would carry CLOSED as an absent field
 * and make "shut the door" indistinguishable from a client that forgot to say
 * — which is why the contract carries an enum the server refuses to guess at.
 */
export function setJoinDoor(open: boolean): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: {
      case: "setJoinDoor",
      value: { door: open ? JoinDoor.OPEN : JoinDoor.CLOSED },
    },
  });
}

/**
 * OpenDoor / CloseDoor work a door in a WALL, and are not the join door.
 *
 * view/dm.ts renders buttons labelled "Open the door" and "Close the door"
 * already, and they are setJoinDoor — the admissions door for seating people
 * at the table. Two different doors, one word. These carry a scene and a
 * square; that one carries a policy.
 */
export function openDoor(sceneId: string, at: Point): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "openDoor", value: { sceneId, at } },
  });
}

export function closeDoor(sceneId: string, at: Point): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "closeDoor", value: { sceneId, at } },
  });
}

/**
 * LoadMap brings one standalone map into the campaign as a whole ordered
 * batch — a SceneCreated plus one TokenPlaced per declared placement — and is
 * rejected atomically if a placement names an actor that does not exist yet.
 * So a refusal means nothing loaded, and the caller may say so plainly.
 */
export function loadMap(mapId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "loadMap", value: { mapId } },
  });
}

/**
 * Perch a spectator on a party member's shoulder (visibility spec §3.1.1,
 * Patrik 2026-08-18: "like a bird hopping from one shoulder to another").
 *
 * THE EMPTY STRING IS A LEGAL ARGUMENT AND IT MEANS SOMETHING. It is how a
 * bird leaves a shoulder without taking another, and it is the state every
 * spectator starts a connection in — internal/gateway/viewpoint.go returns nil
 * for it explicitly rather than refusing it. So this builder must never
 * shortcut an empty id into "no command": a spectator with no way to stop
 * seeing is a spectator stuck inside someone's eyes.
 *
 * On the wire the empty id then VANISHES, because protojson omits empty
 * scalars — the meaning is carried by the PRESENCE of the set_viewpoint arm,
 * not by a field inside it, and the server reads an absent actor_id as
 * "un-perch". Same shape as an omitted move reason, opposite consequence.
 *
 * APPENDS NOTHING (commands.proto). Where a watcher points their camera is not
 * a fact about the campaign, so there is no event to wait for — which is also
 * why a perch does not survive a reconnect and the client is what re-sends it.
 * See app.ts's redial for what that costs.
 */
export function setViewpoint(actorId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "setViewpoint", value: { actorId } },
  });
}

/** Replace the join link's secret, locking out a link that has leaked. */
export function rotateJoinLink(): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "rotateJoinLink", value: {} },
  });
}

/**
 * Change what a participant is allowed to do (spec §3.1a).
 *
 * Only "player" and "spectator" are reachable: a shared link mints spectators,
 * so letting promotion reach dm or agent would make the link a route to full
 * authority in two steps. The server enforces that too — this signature is the
 * near reminder, not the guard.
 */
export function promoteParticipant(participantId: string, role: "player" | "spectator"): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "promoteParticipant", value: { participantId, role } },
  });
}
