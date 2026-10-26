// "D1" below is decision D1 of
// docs/superpowers/plans/2026-10-26-a-player-speaks-as-their-character.md.
import { test, expect } from "bun:test";
import { fromJson, toJson, toBinary } from "@bufbuild/protobuf";
import { AddNarrationSchema } from "./gen/ts/vtt/v1/commands_pb";
import { NarrationAddedSchema } from "./gen/ts/vtt/v1/events_pb";

// D1: AddNarration.actor_id = 5, out_of_character = 6
test("qaNarr: command fields are 5 and 6 on the wire", () => {
  const m = fromJson(AddNarrationSchema, { actorId: "a", outOfCharacter: true });
  expect(Array.from(toBinary(AddNarrationSchema, m))).toEqual([
    0x2a, 0x01, 0x61, 0x30, 0x01,
  ]);
});

// D1: NarrationAdded.out_of_character = 5
test("qaNarr: event field is 5 on the wire", () => {
  const m = fromJson(NarrationAddedSchema, { outOfCharacter: true });
  expect(Array.from(toBinary(NarrationAddedSchema, m))).toEqual([0x28, 0x01]);
});

// D1: the command names actorId and outOfCharacter on the wire
test("qaNarr: command json round trips with the new names", () => {
  const j = { text: "hi", actorId: "ada", outOfCharacter: true };
  const m = fromJson(AddNarrationSchema, j);
  expect(m.actorId).toBe("ada");
  expect(m.outOfCharacter).toBe(true);
  expect(toJson(AddNarrationSchema, m)).toEqual(j);
});

// D1: "protojson omitting a false bool"
test("qaNarr: false and empty are omitted from json", () => {
  const c = fromJson(AddNarrationSchema, { text: "x" });
  expect(toJson(AddNarrationSchema, c)).toEqual({ text: "x" });
  const e = fromJson(NarrationAddedSchema, { text: "x", as: "Ada" });
  expect(e.outOfCharacter).toBe(false);
  expect(toJson(NarrationAddedSchema, e)).toEqual({ text: "x", as: "Ada" });
});

// D1: event carries the mark and the name in `as`
test("qaNarr: event carries the mark and the name", () => {
  const j = { text: "brb", as: "Ada", outOfCharacter: true };
  const m = fromJson(NarrationAddedSchema, j);
  expect(m.as).toBe("Ada");
  expect(m.outOfCharacter).toBe(true);
  expect(toJson(NarrationAddedSchema, m)).toEqual(j);
});

// VTT-505
test("qaNarr: the event has no actor id", () => {
  const names = NarrationAddedSchema.fields.map((f) => f.name);
  expect(names.filter((n) => n.includes("actor"))).toEqual([]);
  expect(() =>
    fromJson(NarrationAddedSchema, { text: "x", actorId: "a" }),
  ).toThrow();
});

// D1: "None is proto3 `optional`"
test("qaNarr: the new fields are not proto3 optional", () => {
  const fs = [
    AddNarrationSchema.field.actorId!,
    AddNarrationSchema.field.outOfCharacter!,
    NarrationAddedSchema.field.outOfCharacter!,
  ];
  for (const f of fs) expect(f.proto.proto3Optional).toBe(false);
});
