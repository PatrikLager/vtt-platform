import { test, expect } from "bun:test";
import { fromJson, toJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  TokenMovedSchema,
  AttackRolledSchema,
  ActorSchema,
} from "./gen/ts/vtt/v1/events_pb";
import {
  MoveTokenRequestSchema,
  ClientCommandSchema,
  ServerFrameSchema,
} from "./gen/ts/vtt/v1/commands_pb";

const cases = [
  ["token_moved.json", TokenMovedSchema],
  ["attack_rolled.json", AttackRolledSchema],
  ["actor.json", ActorSchema],
  ["move_token_request.json", MoveTokenRequestSchema],
  ["envelope.json", EnvelopeSchema],
  ["scene_envelope.json", EnvelopeSchema],
  ["client_command.json", ClientCommandSchema],
  ["server_frame_result.json", ServerFrameSchema],
  ["server_frame_error.json", ServerFrameSchema],
  ["server_frame_catch_up_head.json", ServerFrameSchema],
  ["ability_used_envelope.json", EnvelopeSchema],
  ["use_ability_command.json", ClientCommandSchema],
  ["narration_added_envelope.json", EnvelopeSchema],
  ["upsert_note_command.json", ClientCommandSchema],
  ["adventure_loaded_envelope.json", EnvelopeSchema],
  ["load_adventure_command.json", ClientCommandSchema],
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

/**
 * THE CONTRACT ITSELF NO LONGER KNOWS THE WORD.
 *
 * Every consumer lost retraction before this file did — the client and its
 * fold, the harness, the campaign, the gateway and the agent — and each proved
 * it with a test that had to BUILD the thing being removed. TWO SHAPES, told
 * apart here because an earlier draft of this comment collapsed them and was
 * wrong about four tests: most built an `EventsRetracted` EVENT — the folds
 * (campaign, harness, mcp, client) put one in a log and showed no sequence was
 * skipped, engine.Apply took one directly, and the gateway's projection put one
 * on the wire and showed no seat was offered it — while the gateway's authz and
 * conversion tests and the agent's end-to-end refusal built a `RetractEvents`
 * COMMAND and folded nothing at all. Neither shape can outlive its message.
 * This is what replaces all fifteen: the property stops being behavioural and
 * becomes structural, asserted once where the messages are declared rather than
 * at each reader in turn.
 *
 * Read from the GENERATED DESCRIPTORS, never from the .proto text, because
 * the descriptor is what every consumer actually compiles against — a message
 * deleted from the source but left in committed generated code would still be
 * on the wire, and grepping the .proto would call that clean.
 *
 * The match is a PATTERN, not the two names: re-adding the concept under a
 * new spelling (`Rescind`, `RetractRange`, `UnEvent`) is how a removed idea
 * comes back, and a test naming `EventsRetracted` alone would greet it green.
 * A pattern cannot catch a synonym, but it does catch the honest rename.
 *
 * FOR WHOEVER WRITES THE REPO-WIDE `check:no-retraction` GATE (Task 11 of the
 * same plan, and spec exit criterion 1): this file is the one place under
 * `contract/` where the word must survive, because writing the pattern down is
 * how the absence is enforced. Exempt it explicitly, the way that gate's own
 * script exempts itself — a gate that deleted its own enforcement would pass
 * while proving nothing.
 */
const RETRACTION = /retract/i;

/**
 * The two descriptor shapes this file reads, declared structurally rather than
 * imported: protobuf-es keeps DescFile/DescMessage in a subpath its package
 * root does not re-export, and naming only the fields actually used keeps this
 * check independent of that layout.
 */
type MessageDesc = { readonly name: string; readonly nestedMessages: readonly MessageDesc[] };
type OneofHolder = {
  readonly oneofs: readonly { readonly name: string; readonly fields: readonly { readonly localName: string }[] }[];
};

/** Every message the file declares, nested ones included. */
function allMessageNames(file: { readonly messages: readonly MessageDesc[] }): string[] {
  const out: string[] = [];
  const walk = (msgs: readonly MessageDesc[]) => {
    for (const m of msgs) {
      out.push(m.name);
      walk(m.nestedMessages);
    }
  };
  walk(file.messages);
  return out;
}

/** A oneof's case names, as a consumer's switch would see them. */
function oneofCases(schema: OneofHolder, oneof: string): string[] {
  const found = schema.oneofs.find((o) => o.name === oneof);
  if (!found) throw new Error(`no \`${oneof}\` oneof`);
  return found.fields.map((f) => f.localName);
}

/**
 * THE CONTROL, and it is what stops both tests below being vacuous — the same
 * job project_internal_test.go's own control does for the classify loop.
 *
 * Two ways these tests pass while proving nothing, and NEITHER is visible in a
 * green run. Narrow the pattern to `/^retract$/i` and both stay green forever
 * while a re-added `EventsRetracted` sails through. Break the descriptor walk
 * so it yields nothing and `[].filter(...)` is `[]`, which is the answer the
 * assertion wants. No mutation gate scores this file — `check:ts-mutation`'s
 * glob is `client/src/**` — and a repo-wide `retract` grep still sees the word
 * in the pattern and calls it clean, so nothing else catches either move.
 *
 * The bounds are LOWER BOUNDS well under the real sizes (58 messages, 42 arms
 * at the time of writing) and no count is asserted: this must not rot the day
 * a command is added, only fire if the corpus collapses.
 */
function checkControl(corpus: string[], floor: number): void {
  // The pattern must match what it exists to catch — both spellings, since the
  // proto and the generated arm name differ in case and separator.
  expect(RETRACTION.test("EventsRetracted")).toBe(true);
  expect(RETRACTION.test("RetractEvents")).toBe(true);
  expect(RETRACTION.test("retract_events")).toBe(true);
  expect(RETRACTION.test("eventsRetracted")).toBe(true);
  // ...and must not match everything, or it would red on the first green run.
  expect(RETRACTION.test("MoveTokenRequest")).toBe(false);
  // The corpus must be real, or the filter below has nothing to reject.
  expect(corpus.length).toBeGreaterThan(floor);
}

test("no message in the contract retracts anything", () => {
  const declared = [
    ...allMessageNames(ClientCommandSchema.file),
    ...allMessageNames(EnvelopeSchema.file),
  ];
  checkControl(declared, 30);
  expect(declared.filter((n) => RETRACTION.test(n))).toEqual([]);
});

test("no command arm and no payload arm retracts anything", () => {
  const arms = [
    ...oneofCases(ClientCommandSchema, "command"),
    ...oneofCases(EnvelopeSchema, "payload"),
  ];
  checkControl(arms, 20);
  expect(arms.filter((c) => RETRACTION.test(c))).toEqual([]);
});
