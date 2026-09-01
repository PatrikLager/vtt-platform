import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";

// THE TYPESCRIPT HALF OF THE REMOVAL BATCH.
//
// WHY THIS FILE EXISTS AT ALL. Neither remove_token nor remove_actor, and
// neither TokenRemoved nor ActorRemoved, appears in any golden stream, any
// scenario definition, any soak action or any cmd/vtt e2e — so fold-parity,
// projection-parity and the golden corpus have no reach over the two events
// this arc added, in either language. Whole-branch review on 2026-09-01
// established by hand that Go's engine.Apply and this fold agree on a real
// projected removal stream. This is that check, kept.
//
// THE SAME BYTES, NOT A SECOND COPY OF THE STREAM.
// contract/testdata/removal_batch_projected_stream.json is read here and by
// internal/gateway's TestARemovalBatchProjectsToTheBytesBothFoldsRead, which
// RECOMPUTES the projection from a real engine.State and compares it envelope
// by envelope. So the fixture cannot drift from what the server emits, and
// this file cannot drift from the fixture: the two folds are consuming one
// artifact rather than two hand-written transcriptions of it, which is the
// trap scenarios/goldens/README.md calls "two identically-wrong folds
// agreeing".
//
// THE CASE IS MIXED VISIBILITY, which is the only interesting one. The seat is
// the hero's player. It was shown the goblin and its near token t-gob, and was
// never told that t-gob-far — behind a closed door — exists. remove_actor then
// emits three events (TokenRemoved t-gob, TokenRemoved t-gob-far, ActorRemoved
// goblin) and the seat receives two: a synthesized TokenHidden for the token it
// could see, nothing whatever for the one it could not, and the raw
// ActorRemoved because it holds the actor. A fold that threw on any of those —
// "removed unknown token", "removed unknown actor" — would freeze a real
// client permanently, because session.ts re-folds the whole log on every event.
//
// ASSERTED AGAINST FACTS RATHER THAN A state.json, and the reason is that no
// human-derived dump exists for this stream: the Go half asserts the same list
// of facts, written independently in its own language, rather than either side
// checking the other's output.

const fixture = join(import.meta.dir, "../../contract/testdata/removal_batch_projected_stream.json");

function stream(): Envelope[] {
  const raw = JSON.parse(readFileSync(fixture, "utf8")) as unknown[];
  // A collapsed fixture would make every assertion below vacuous — fold([])
  // throws nothing and returns an empty world, in which "the goblin is gone"
  // is trivially true.
  expect(raw.length).toBeGreaterThan(5);
  return raw.map((e) => fromJson(EnvelopeSchema, e as never));
}

test("the projected removal batch folds, and what it removes is gone", () => {
  const envelopes = stream();

  // THE CONTROL. The seat must actually hold the world before the batch, or
  // "removed" below means "never arrived". Folded up to the first removal
  // event, everything is present.
  const before = fold(envelopes.filter((e) => e.payload.case !== "tokenHidden" && e.payload.case !== "actorRemoved"));
  expect(Object.keys(before.Actors).sort()).toEqual(["goblin", "hero"]);
  expect(Object.keys(before.Tokens).sort()).toEqual(["t-gob", "t-hero"]);

  const st = fold(envelopes);
  expect(Object.keys(st.Actors)).toEqual(["hero"]);
  expect(Object.keys(st.Tokens)).toEqual(["t-hero"]);
  // The seat's own character and its board survive somebody else's removal.
  expect(st.Tokens["t-hero"]!.X).toBe(1);
  expect(st.Tokens["t-hero"]!.Y).toBe(1);
  expect(st.Scenes["s"]!.Name).toBe("S");
});

test("a token this seat was never shown is named nowhere in its stream", () => {
  // The mixed half. If the projection ever started forwarding the withheld
  // TokenRemoved, this fold would throw "removed unknown token" — but it would
  // throw for a reason no assertion above names, so the absence is asserted
  // directly on the bytes as well.
  const raw = readFileSync(fixture, "utf8");
  expect(raw).toContain("t-gob");
  expect(raw).not.toContain("t-gob-far");
});
