import "./support/dom";
import { describe as suite, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  ActorAddedSchema,
  ActorKind,
  ActorSchema,
  DoorClosedSchema,
  DoorOpenedSchema,
  EnvelopeSchema,
  GridPositionSchema,
  SceneCreatedSchema,
  SessionStartedSchema,
  TokenPlacedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { describe, renderSpectator } from "../src/view/spectator";
import { namer } from "../src/view/naming";

let seq = 0n;
function env(payload: Envelope["payload"], eventId: string): Envelope {
  seq += 1n;
  return create(EnvelopeSchema, { eventId, sequence: seq, sessionId: "sess", participantId: "Participant Alice", payload });
}
const pos = (x: number, y: number) => create(GridPositionSchema, { x, y });
const actor = (id: string, name: string, kind: ActorKind) =>
  env({ case: "actorAdded", value: create(ActorAddedSchema, { actor: create(ActorSchema, { actorId: id, name, kind }) }) }, "ea-" + id);
const token = (id: string, a: string, name: string, gmName: string, x: number, y: number) =>
  env({ case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: id, sceneId: "s1", actorId: a, position: pos(x, y), name, gmName }) }, "et-" + id);
const opened = (a: string, x: number, y: number, eventId: string) =>
  env({ case: "doorOpened", value: create(DoorOpenedSchema, { sceneId: "s1", at: pos(x, y), actorId: a }) }, eventId);
const closed = (a: string, x: number, y: number, eventId: string) =>
  env({ case: "doorClosed", value: create(DoorClosedSchema, { sceneId: "s1", at: pos(x, y), actorId: a }) }, eventId);

const base = (): Envelope[] => [
  env({ case: "sessionStarted", value: create(SessionStartedSchema, { name: "Night" }) }, "e-sess"),
  env({ case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s1", name: "Hall", gridWidth: 12, gridHeight: 12 }) }, "e-scene"),
  actor("pc", "Tobbe", ActorKind.PARTY_MEMBER),
  actor("mon", "Goblin", ActorKind.UNSPECIFIED),
  actor("ghost", "", ActorKind.UNSPECIFIED),
  token("t-pc", "pc", "", "", 1, 1),
  token("t-mon", "mon", "Shade", "Orc Chief", 2, 2),
];

function windows(log: Envelope[], extras: object) {
  const root = document.createElement("div");
  renderSpectator(root, fold(log), log, "ok", extras);
  const q = (s: string) => Array.from(root.querySelectorAll(s)).map((n) => (n.textContent ?? "").trim());
  return { story: q(".feed .beat p"), events: q(".ticker .tick") };
}

const log = (): Envelope[] => [
  ...base(),
  opened("pc", 3, 7, "d1"),
  closed("mon", 5, 2, "d2"),
  opened("", 6, 4, "d3"),
  closed("ghost", 2, 8, "d4"),
  opened("pc", 9, 1, ""),
];
const doorOnly = (rows: string[]) => rows.filter((r) => /door/.test(r));

suite("a door names its opener in a player's window", () => {
  const w = windows(log(), {});
  // VTT-871
  test("Story: opener named by the player chain, no square, none named reads as today", () => {
    expect(doorOnly(w.story)).toEqual([
      "Tobbe opened a door",
      "Shade closed a door",
      "a door opened",
      "a creature closed a door",
    ]);
  });
  // VTT-820
  test("Story draws no line for a door with no event id", () => {
    expect(doorOnly(w.story).length).toBe(4);
  });
  // VTT-871
  test("no id, GM name, actor name or participant reaches the player's door lines", () => {
    const all = doorOnly(w.story).join("\n");
    for (const bad of ["pc", "mon", "ghost", "Orc Chief", "Goblin", "Alice"]) expect(all).not.toContain(bad);
  });
});

suite("a door names its opener in a DM's window", () => {
  const w = windows(log(), { dm: true, ticker: true });
  // VTT-871
  test("Story: opener named by the GM chain", () => {
    expect(doorOnly(w.story)).toEqual([
      "Tobbe opened a door",
      "Orc Chief closed a door",
      "a door opened",
      "a creature closed a door",
    ]);
  });
  // VTT-872
  test("Events panel: the Story's words, then ' at x,y'", () => {
    expect(doorOnly(w.events)).toEqual([
      "Tobbe opened a door at 9,1",
      "a creature closed a door at 2,8",
      "a door opened at 6,4",
      "Orc Chief closed a door at 5,2",
      "Tobbe opened a door at 3,7",
    ]);
  });
  // VTT-820
  test("Story leaves the no-event-id door out while the Events panel keeps it", () => {
    expect(doorOnly(w.story).length).toBe(4);
    expect(doorOnly(w.events).length).toBe(5);
  });
  // VTT-871
  test("Story carries no square", () => {
    expect(doorOnly(w.story).some((r) => /\d,\d/.test(r))).toBe(false);
  });
});

suite("describe", () => {
  const l = [...base(), opened("pc", 3, 7, "d1"), closed("mon", 5, 2, "d2"), opened("", 6, 4, "d3"), closed("", 8, 9, "d4")];
  const st = fold(l);
  const nm = namer(st, l, true, undefined);
  const get = (i: number) => describe(l[i]!, nm);
  const n = base().length;
  // VTT-872
  test("words opened and closed with the opener and the square", () => {
    expect(get(n)).toBe("Tobbe opened a door at 3,7");
    expect(get(n + 1)).toBe("Orc Chief closed a door at 5,2");
  });
  // VTT-872
  test("words a door naming none as today, with the square", () => {
    expect(get(n + 2)).toBe("a door opened at 6,4");
    expect(get(n + 3)).toBe("a door closed at 8,9");
  });
  // VTT-872
  test("a player's namer names the token, not the GM name", () => {
    const pn = namer(st, l, false, undefined);
    expect(describe(l[n + 1]!, pn)).toBe("Shade closed a door at 5,2");
  });
});
