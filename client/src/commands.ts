// Command construction.
//
// Every builder here is asserted against the COMMITTED protojson fixtures in
// contract/testdata (client/test/commands.test.ts) — the same files the Go
// and TS contract round-trip tests use. That makes the shapes a
// cross-language claim rather than a self-consistent one: if the client's
// idea of a command drifts from what the server parses, a test fails instead
// of a player getting a rejection nobody can explain.
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
  type ClientCommand,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
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

/** Retract an INCLUSIVE sequence range. Undoing one event passes it twice. */
export function retractEvents(from: bigint, to: bigint, reason: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "retractEvents", value: { fromSequence: from, toSequence: to, reason } },
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
    return create(ClientCommandSchema, {
      requestId: requestId(),
      command: { case: "addActor", value: { actor } },
    });
  } catch (e) {
    return new Error(`not a valid actor: ${e instanceof Error ? e.message : String(e)}`);
  }
}

/** addActor from an already-built actor object (the form path). */
export function addActor(actorId: string, name: string, controllerId?: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: {
      case: "addActor",
      value: { actor: { actorId, name, ...(controllerId ? { controllerId } : {}) } },
    },
  });
}

/**
 * Hand control of an actor to a participant (spec §5.3, dm/agent only).
 *
 * Additive: the server ADDS to controller_ids rather than replacing, so
 * granting does not take the character away from whoever already holds it.
 * That is what makes a character shareable, and it is why there is no
 * "transfer" builder — a transfer is a grant and a revoke, two decisions.
 */
export function grantActorControl(actorId: string, participantId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "grantActorControl", value: { actorId, participantId } },
  });
}

/** Take one participant's control of an actor away (spec §5.3). */
export function revokeActorControl(actorId: string, participantId: string): ClientCommand {
  return create(ClientCommandSchema, {
    requestId: requestId(),
    command: { case: "revokeActorControl", value: { actorId, participantId } },
  });
}
