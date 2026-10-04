import { test, expect } from "bun:test";
import { fromJson, type JsonObject } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { FoldError, type State } from "../src/state";

const SCENE = "scn-qa-bound";
const ACTOR = "act-qa-bound";
const TOKEN = "tok-qa-bound";

const utf8Bytes = (s: string): number => new TextEncoder().encode(s).length;

function env(seq: number, payload: JsonObject): Envelope {
  return fromJson(EnvelopeSchema, {
    eventId: `evt-qa-bound-${seq}`,
    sequence: String(seq),
    occurredAt: "2026-10-04T00:00:00Z",
    actorRole: "dm",
    participantId: "p-qa",
    ...payload,
  });
}

function seed(): Envelope[] {
  return [
    env(1, { sceneCreated: { sceneId: SCENE, name: "QA scene", gridWidth: 10, gridHeight: 10 } }),
    env(2, { actorAdded: { actor: { actorId: ACTOR, name: "QA actor", kind: "ACTOR_KIND_NON_PARTY" } } }),
    env(3, { tokenPlaced: { tokenId: TOKEN, sceneId: SCENE, actorId: ACTOR, position: { x: 1, y: 1 } } }),
  ];
}

function move(tokenId: string, to: { x: number; y: number } | undefined, reason: string): Envelope {
  const tm: JsonObject = { tokenId, sceneId: SCENE, from: { x: 1, y: 1 }, reason };
  if (to) tm.to = to;
  return env(4, { tokenMoved: tm });
}

function narration(text: string, as: string): Envelope {
  return env(4, { narrationAdded: { text, as } });
}

function note(key: string, title: string, text: string): Envelope {
  return env(4, { noteUpserted: { key, title, text, visibility: "NOTE_VISIBILITY_SECRET" } });
}

const TO = { x: 2, y: 2 };

function refusalOf(envs: Envelope[]): Error {
  let caught: unknown;
  try {
    fold(envs);
  } catch (e) {
    caught = e;
  }
  if (caught === undefined) throw new Error("fold accepted the stream");
  return caught as Error;
}

function accepts(envs: Envelope[]): State {
  return fold(envs);
}

type Case = { name: string; text: string; bytes: number };

const atLimit256: Case[] = [
  { name: "ascii 256", text: "a".repeat(256), bytes: 256 },
  { name: "two-byte x128", text: "é".repeat(128), bytes: 256 },
  { name: "three-byte x85 plus ascii", text: "€".repeat(85) + "a", bytes: 256 },
  { name: "four-byte x64", text: "😀".repeat(64), bytes: 256 },
];

const over256: Case[] = [
  { name: "ascii 257", text: "a".repeat(257), bytes: 257 },
  { name: "two-byte x129 is 129 characters", text: "é".repeat(129), bytes: 258 },
  { name: "three-byte x86 is 86 characters", text: "€".repeat(86), bytes: 258 },
  { name: "four-byte x65 is 130 UTF-16 units", text: "😀".repeat(65), bytes: 260 },
  { name: "ascii 255 plus one two-byte", text: "a".repeat(255) + "é", bytes: 257 },
];

for (const c of atLimit256) {
  // VTT-264
  test(`the client's fold accepts a ${c.bytes}-byte reason (${c.name})`, () => {
    expect(utf8Bytes(c.text)).toBe(c.bytes);
    const st = accepts([...seed(), move(TOKEN, TO, c.text)]);
    expect(st.Tokens[TOKEN]?.X).toBe(2);
    expect(st.Tokens[TOKEN]?.Y).toBe(2);
  });
}

for (const c of over256) {
  // VTT-264
  test(`the client's fold refuses a ${c.bytes}-byte reason (${c.name})`, () => {
    expect(utf8Bytes(c.text)).toBe(c.bytes);
    const err = refusalOf([...seed(), move(TOKEN, TO, c.text)]);
    expect(err).toBeInstanceOf(FoldError);
    expect(err.message).toContain("exceeds 256 bytes");
    expect(err.message).toContain("reason");
  });
}

// SPEC-018 "How it works", table row `TokenMoved.reason` | 256 | yes.
test("the client's fold accepts an empty reason", () => {
  const st = accepts([...seed(), move(TOKEN, TO, "")]);
  expect(st.Tokens[TOKEN]?.X).toBe(2);
});

// SPEC-018 "How it works": "A `TokenMoved` is checked for a known token, then
// for a destination at all, then its reason."
test("the client's fold checks a move's token, then its destination, then its reason", () => {
  const long = "a".repeat(257);
  const reasonErr = refusalOf([...seed(), move(TOKEN, TO, long)]);
  expect(reasonErr).toBeInstanceOf(FoldError);
  expect(reasonErr.message).toContain("exceeds 256 bytes");

  const tokenCtl = refusalOf([...seed(), move("tok-qa-bound-absent", TO, "")]);
  expect(tokenCtl).toBeInstanceOf(FoldError);
  const destCtl = refusalOf([...seed(), move(TOKEN, undefined, "")]);
  expect(destCtl).toBeInstanceOf(FoldError);

  const tokenFirst = refusalOf([...seed(), move("tok-qa-bound-absent", undefined, long)]);
  expect(tokenFirst).toBeInstanceOf(FoldError);
  expect(tokenFirst.message).toBe(tokenCtl.message);

  const tokenBeforeReason = refusalOf([...seed(), move("tok-qa-bound-absent", TO, long)]);
  expect(tokenBeforeReason.message).toBe(tokenCtl.message);

  const destFirst = refusalOf([...seed(), move(TOKEN, undefined, long)]);
  expect(destFirst).toBeInstanceOf(FoldError);
  expect(destFirst.message).toBe(destCtl.message);
  expect(destFirst.message).not.toContain("reason");
});

for (const c of [...atLimit256, { name: "empty", text: "", bytes: 0 }]) {
  // VTT-266
  test(`the client's fold accepts a ${c.bytes}-byte narration speaker (${c.name})`, () => {
    expect(utf8Bytes(c.text)).toBe(c.bytes);
    expect(() => accepts([...seed(), narration("Something happens.", c.text)])).not.toThrow();
  });
}

for (const c of over256) {
  // VTT-266
  test(`the client's fold refuses a ${c.bytes}-byte narration speaker (${c.name})`, () => {
    expect(utf8Bytes(c.text)).toBe(c.bytes);
    const err = refusalOf([...seed(), narration("Something happens.", c.text)]);
    expect(err).toBeInstanceOf(FoldError);
    expect(err.message).toContain("exceeds 256 bytes");
  });
}

const firstFieldCases: { name: string; want: string; e: Envelope }[] = [
  { name: "key, title and text all out of bounds", want: "note key", e: note("", "a".repeat(257), "") },
  { name: "title and text out of bounds", want: "note title", e: note("k", "a".repeat(257), "") },
  { name: "narration text and speaker out of bounds", want: "narration text", e: narration("", "a".repeat(257)) },
];

for (const c of firstFieldCases) {
  // SPEC-018 "How it works": "`fold.ts` calls `checkLen(what, s, min, max)`
  // for each of the six fields with the same numbers as literals, in the same
  // order within each arm".
  test(`the client's fold names the ${c.want} first when ${c.name}`, () => {
    const err = refusalOf([...seed(), c.e]);
    expect(err).toBeInstanceOf(FoldError);
    expect(err.message.startsWith(c.want)).toBe(true);
  });
}

type Field = { name: string; bound: number; mayEmpty: boolean; env: (s: string) => Envelope };

const otherFields: Field[] = [
  { name: "note key", bound: 128, mayEmpty: false, env: (s) => note(s, "Title", "Body.") },
  { name: "note title", bound: 256, mayEmpty: true, env: (s) => note("k", s, "Body.") },
  { name: "note text", bound: 8192, mayEmpty: false, env: (s) => note("k", "Title", s) },
  { name: "narration text", bound: 8192, mayEmpty: false, env: (s) => narration(s, "") },
];

for (const f of otherFields) {
  // SPEC-018 "How it works": the table's row for this field, and "throws
  // `FoldError` reading `<field> exceeds <max> bytes` or `<field> is shorter
  // than <min> bytes`".
  test(`the client's fold bounds the ${f.name} at ${f.bound} bytes${f.mayEmpty ? " and allows it empty" : " and refuses it empty"}`, () => {
    expect(() => accepts([...seed(), f.env("a".repeat(f.bound))])).not.toThrow();
    expect(() => accepts([...seed(), f.env("é".repeat(f.bound / 2))])).not.toThrow();

    const over = refusalOf([...seed(), f.env("a".repeat(f.bound + 1))]);
    expect(over).toBeInstanceOf(FoldError);
    expect(over.message).toContain(`exceeds ${f.bound} bytes`);

    const overMultibyte = refusalOf([...seed(), f.env("é".repeat(f.bound / 2 + 1))]);
    expect(overMultibyte).toBeInstanceOf(FoldError);
    expect(overMultibyte.message).toContain(`exceeds ${f.bound} bytes`);

    if (f.mayEmpty) {
      expect(() => accepts([...seed(), f.env("")])).not.toThrow();
    } else {
      const empty = refusalOf([...seed(), f.env("")]);
      expect(empty).toBeInstanceOf(FoldError);
      expect(empty.message).toContain("is shorter than 1 bytes");
    }
  });
}
