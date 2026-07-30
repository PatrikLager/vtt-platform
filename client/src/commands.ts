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
