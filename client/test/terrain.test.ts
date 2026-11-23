import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { blocksMove } from "../src/view/terrain";
import type { Scene } from "../src/state";

const testdata = (name: string) => join(import.meta.dir, "../../contract/testdata", name);

interface Listed {
  sceneId: string;
  squares: { case: string; x: number; y: number; blocked: boolean }[];
}

// VTT-937: internal/engine's TestBlockedAnswersEverySquareTheSharedFixtureLists
// reads the same two files.
test("blocksMove answers every square the shared fixture lists as that fixture says", () => {
  const log = (JSON.parse(readFileSync(testdata("blocked_squares_log.json"), "utf8")) as unknown[])
    .map((e) => fromJson(EnvelopeSchema, e as never));
  const want = JSON.parse(readFileSync(testdata("blocked_squares.json"), "utf8")) as Listed;
  expect(want.squares.length).toBeGreaterThan(0);
  const scene = fold(log).Scenes[want.sceneId];
  expect(scene).toBeDefined();
  expect(want.squares.map((sq) => [sq.case, blocksMove(scene!, sq)]))
    .toEqual(want.squares.map((sq) => [sq.case, sq.blocked]));
});

// VTT-936
test("a door on a scene that holds no OpenDoors record blocks the move", () => {
  const scene: Scene = {
    ID: "s", Name: "s", GridWidth: 2, GridHeight: 1,
    Tiles: { "1,0": { Kind: "door", Material: "", Art: "" } },
  };
  expect(blocksMove(scene, { x: 1, y: 0 })).toBe(true);
});
