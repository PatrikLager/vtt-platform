import { test, expect } from "bun:test";
import Ajv2020 from "ajv/dist/2020";
import type { TokenMoved } from "./gen/ts/types";

const ajv = new Ajv2020();

const cases = [
  ["token_moved", "token_moved.json"],
  ["attack_rolled", "attack_rolled.json"],
  ["actor", "actor.json"],
  ["move_token_request", "move_token_request.json"],
] as const;

for (const [schemaName, fixture] of cases) {
  test(`${fixture} validates against ${schemaName}.schema.json`, async () => {
    const schema = await Bun.file(
      new URL(`./schemas/${schemaName}.schema.json`, import.meta.url),
    ).json();
    const data = await Bun.file(
      new URL(`../fixtures/${fixture}`, import.meta.url),
    ).json();
    const validate = ajv.compile(schema);
    expect(validate(data)).toBe(true);
  });
}

test("generated TS types are consumable", async () => {
  const data = (await Bun.file(
    new URL("../fixtures/token_moved.json", import.meta.url),
  ).json()) as TokenMoved;
  expect(data.tokenId).toBe("tok-ursus");
});
