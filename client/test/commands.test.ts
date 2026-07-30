import { test, expect } from "bun:test";
import { toJson } from "@bufbuild/protobuf";
import { ClientCommandSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { moveToken, useAbility, addNarration, upsertNote } from "../src/commands";

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
