import { test, expect } from "bun:test";
import { fromJson, toJson } from "@bufbuild/protobuf";
import {
  TokenMovedSchema,
  AttackRolledSchema,
  ActorSchema,
  MoveTokenRequestSchema,
} from "./gen/ts/contract/v1/events_pb";

const cases = [
  ["token_moved.json", TokenMovedSchema],
  ["attack_rolled.json", AttackRolledSchema],
  ["actor.json", ActorSchema],
  ["move_token_request.json", MoveTokenRequestSchema],
] as const;

for (const [fixture, schema] of cases) {
  test(`${fixture} round-trips`, async () => {
    const raw = await Bun.file(
      new URL(`../fixtures/${fixture}`, import.meta.url),
    ).json();
    const msg = fromJson(schema as any, raw);
    expect(toJson(schema as any, msg)).toEqual(raw);
  });
}
