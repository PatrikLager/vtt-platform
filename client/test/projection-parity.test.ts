import { test, expect } from "bun:test";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { foldToDumpJSON } from "../src/fold";

// THE KEYSTONE'S TYPESCRIPT HALF (visibility spec §4.3).
//
// The Go half — internal/gateway/keystone_test.go — computes both sides of
//
//     fold(project(log, viewer)) == visibleState(fold(log), viewer)
//
// at every prefix of every golden, with the right-hand side derived
// independently from engine.State and internal/sight. THIS FILE CANNOT DO THAT,
// and the reason is a design decision rather than a shortfall: there is no
// TypeScript projector and there is deliberately never going to be one. Spec
// §6.2 — "server decides what you may see; the client draws it" — puts every
// visibility fact on the client's side of the wire as something it was SENT.
// The client owns exactly one thing, memory.
//
// So what this file holds is the half that is the client's to keep: given the
// bytes the projection actually emitted for a seat, client/src/fold.ts must
// derive the same state internal/engine's fold does. It asserts against
// scenarios/goldens/<name>/projections/<seat>/state.json, which is HAND-DERIVED
// from the scenario — the same rule every state.json in the corpus follows — so
// two identically-wrong folds cannot agree their way past a file neither of them
// produced.
//
// The independent sight oracle reaches these fixtures too, but TRANSITIVELY and
// it is worth being exact about how: the Go side pins
// projections/<seat>/stream.json by recomputing the projection and comparing
// BYTES, and the keystone runs its oracle against a fold of that same
// recomputed projection. So the oracle and this file are measured against the
// same stream, one step apart, rather than this file calling an oracle it does
// not have.
//
// WHY THIS IS NOT client/test/fold-parity.test.ts WITH MORE DIRECTORIES. That
// file folds the corpus's LOGS, which are the viewer = DM case: project(log,
// DM) is the identity, pinned by name on the Go side
// (TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees's identity arm
// requires the projector to return the event itself, by pointer). Every scene
// in those logs therefore has no Visible and no Explored at all — the corpus
// pinned both being ABSENT and nothing about the populated case, which is
// exactly what a keystone run over it alone would have been worth. These
// fixtures are the populated case.
//
// Byte comparison rather than deep equality, for fold-parity.test.ts's own
// reason: a structural compare forgives a missing field that JSON.stringify
// simply omits, and omission is the failure mode when mirroring Go's omitempty
// tags — which is precisely how Visible and Explored are rendered.

const goldensDir = join(import.meta.dir, "../../scenarios/goldens");

type ProjectedSeat = { golden: string; seat: string; dir: string };

const seats: ProjectedSeat[] = readdirSync(goldensDir)
  .filter((n) => statSync(join(goldensDir, n)).isDirectory())
  .flatMap((golden) => {
    const projections = join(goldensDir, golden, "projections");
    // MISSING IS ORDINARY; ANYTHING ELSE IS NOT. Most goldens carry no
    // projections at all, so ENOENT is the normal answer and returning []
    // is right. A permissions error or a corrupt directory is a different
    // thing entirely and must not be swallowed into "this golden has no
    // seats" — that reads as a shrinking corpus rather than as a broken one,
    // and the "the projected corpus is not empty" guard above is the only
    // thing that would notice, and only if EVERY golden failed at once.
    let entries: string[];
    try {
      entries = readdirSync(projections);
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === "ENOENT") return [];
      throw err;
    }
    return entries
      .filter((seat) => statSync(join(projections, seat)).isDirectory())
      .map((seat) => ({ golden, seat, dir: join(projections, seat) }));
  });

test("the projected corpus is not empty", () => {
  // Without this every case below is vacuous, and vacuous is the exact failure
  // spec §4.3 warns about: "a keystone run only over today's goldens would be a
  // test of the identity projection wearing the name of the general one".
  expect(seats.length).toBeGreaterThan(0);
});

/** One scene as a committed state.json carries it. */
type DumpedScene = {
  Tiles: Record<string, unknown>;
  Visible?: Record<string, boolean>;
  Explored?: Record<string, boolean>;
};

const scenes: DumpedScene[] = seats.flatMap(({ dir }) =>
  Object.values(
    (JSON.parse(readFileSync(join(dir, "state.json"), "utf8")) as {
      Scenes: Record<string, DumpedScene>;
    }).Scenes,
  ),
);

const size = (m?: Record<string, unknown>) => Object.keys(m ?? {}).length;

test("some projected scene actually has a visible set", () => {
  // The corpus could regress to projections that are all empty — a seat with no
  // eyes projects to almost nothing — and every byte comparison below would
  // still pass. This demands that at least one fixture exercises the field the
  // whole arc added.
  expect(scenes.filter((sc) => size(sc.Visible) > 0).length).toBeGreaterThan(0);
});

test("a bare-canvas scene is visible and remembers nothing", () => {
  // Spec §6.2 and §4.3's subset argument, pinned as a fixture rather than left
  // as prose. Explored is unioned from each sceneSeen's TILES keys, not from its
  // `visible` list, so a scene that declares no terrain has nothing to remember
  // however much of it is in sight. A reader who later "fixes" Explored to
  // follow Visible breaks this, and breaks it here where the reason is written
  // down — not somewhere the failure reads like an accident.
  const bare = scenes.filter((sc) => size(sc.Tiles) === 0 && size(sc.Visible) > 0);
  expect(bare.length).toBeGreaterThan(0);
  for (const sc of bare) expect(sc.Explored).toBeUndefined();
});

test("a tiled scene remembers exactly the terrain it was sent", () => {
  // The other half of the pair, and the reason the corpus carries one scene of
  // each kind side by side: where terrain IS declared, Explored is the tile keys
  // of every sceneSeen so far, so it must not be empty while Visible is not.
  const tiled = scenes.filter((sc) => size(sc.Tiles) > 0);
  expect(tiled.length).toBeGreaterThan(0);
  for (const sc of tiled) expect(size(sc.Explored)).toBe(size(sc.Tiles));
});

for (const { golden, seat, dir } of seats) {
  test(`projection parity: ${golden}/${seat}`, () => {
    const streamRaw = readFileSync(join(dir, "stream.json"), "utf8");
    const want = readFileSync(join(dir, "state.json"), "utf8");

    const envelopes: Envelope[] = (JSON.parse(streamRaw) as unknown[]).map((e) =>
      fromJson(EnvelopeSchema, e as never),
    );

    const got = foldToDumpJSON(envelopes);
    expect(got.trimEnd()).toBe(want.trimEnd());
  });
}
