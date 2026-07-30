import { test, expect } from "bun:test";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { foldToDumpJSON } from "../src/fold";

// THE KEYSTONE (client spec §6). The TypeScript fold must agree with the Go
// fold on the same corpus, byte for byte.
//
// It asserts against scenarios/goldens/<name>/state.json — the file a HUMAN
// derived by hand from the scenario definition, which internal/harness's
// TestFoldGoldenCorpus holds the Go fold to. Both languages are therefore
// measured against one human-derived expectation, not against each other: if
// they were compared to each other, two identically-wrong folds would agree
// and the gate would be silent.
//
// Byte comparison rather than deep equality is deliberate. A structural
// compare would forgive a missing field that JSON.stringify simply omits, and
// omission is exactly the failure mode when mirroring Go's omitempty tags.

const goldensDir = join(import.meta.dir, "../../scenarios/goldens");

const scenarios = readdirSync(goldensDir).filter((n) =>
  statSync(join(goldensDir, n)).isDirectory(),
);

test("the corpus is not empty", () => {
  // An empty corpus would make every case below vacuous.
  expect(scenarios.length).toBeGreaterThan(0);
});

for (const name of scenarios) {
  test(`fold parity: ${name}`, () => {
    const streamRaw = readFileSync(join(goldensDir, name, "stream.json"), "utf8");
    const want = readFileSync(join(goldensDir, name, "state.json"), "utf8");

    const envelopes: Envelope[] = (JSON.parse(streamRaw) as unknown[]).map((e) =>
      fromJson(EnvelopeSchema, e as never),
    );

    const got = foldToDumpJSON(envelopes);
    expect(got.trimEnd()).toBe(want.trimEnd());
  });
}
