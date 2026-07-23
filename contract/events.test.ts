import { test, expect } from "bun:test";
import { fromJson, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  TokenMovedSchema,
  AttackRolledSchema,
  ActorSchema,
} from "./gen/ts/vtt/v1/events_pb";
import { MoveTokenRequestSchema } from "./gen/ts/vtt/v1/commands_pb";

const cases = [
  ["token_moved.json", TokenMovedSchema],
  ["attack_rolled.json", AttackRolledSchema],
  ["actor.json", ActorSchema],
  ["move_token_request.json", MoveTokenRequestSchema],
  ["envelope.json", EnvelopeSchema],
  ["scene_envelope.json", EnvelopeSchema],
  ["retraction_envelope.json", EnvelopeSchema],
] as const;

for (const [fixture, schema] of cases) {
  test(`${fixture} round-trips`, async () => {
    const raw = await Bun.file(
      new URL(`./testdata/${fixture}`, import.meta.url),
    ).json();
    const msg = fromJson(schema as any, raw);
    expect(toJson(schema as any, msg)).toEqual(raw);
  });
}

test("envelope payload is a discriminated case/value pair", async () => {
  const raw = await Bun.file(
    new URL("./testdata/envelope.json", import.meta.url),
  ).json();
  const env = fromJson(EnvelopeSchema, raw);
  expect(env.payload.case).toBe("tokenMoved");
  if (env.payload.case === "tokenMoved") {
    expect(env.payload.value.tokenId).toBe("tok-ursus");
  }
});
