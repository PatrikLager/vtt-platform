import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket
import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import type { CatalogueEntry, PlaceDef } from "../src/metadata";
import { namer } from "../src/view/naming";
import { renderSpectator } from "../src/view/spectator";

const envs = (items: readonly unknown[]): Envelope[] =>
  items.map((e) => fromJson(EnvelopeSchema, e as never));

const entry = (id: string, name: string): CatalogueEntry =>
  ({ id, name, description: "", form: "f", values: {} });

const place = (key: string, name: string): PlaceDef =>
  ({ key, name, description: "", forms: ["f"], copies: true, states: [] });

const CATALOGUE = [entry("echo", "Bright Lance"), entry("delta", "Grey Cloak")];
const PLACES = [place("mike", "Left Hand"), place("kilo", "Back Pack")];

interface Shown {
  catalogue?: CatalogueEntry[];
  places?: PlaceDef[];
}

// Warning: oldest first, the panel's own order reversed.
function rows(log: Envelope[], shown: Shown): string[] {
  const root = document.createElement("div");
  renderSpectator(root, fold(log), log, "open", { ticker: true, dm: true, ...shown });
  const ticks = Array.from(root.querySelectorAll(".ticker .tick"), (p) => p.textContent ?? "");
  expect(ticks.length).toBe(log.length);
  return ticks.reverse();
}

function story(log: Envelope[], shown: Shown): string {
  const root = document.createElement("div");
  renderSpectator(root, fold(log), log, "open", { ticker: true, dm: true, ...shown });
  return root.querySelector(".feed")?.textContent ?? "";
}

function names(line: string, ...want: string[]): void {
  for (const w of want) expect(line).toContain(w);
}

function omits(line: string, ...unwanted: string[]): void {
  for (const u of unwanted) expect(line).not.toContain(u);
}

const IDS = ["echo", "delta", "mike", "kilo", "act-rook", "act-wren"];

const SHEETS = envs([
  { sequence: "1", actorAdded: { actor: {
    actorId: "act-rook", name: "Rook",
    references: {
      "ref-b": { entryId: "echo", place: "mike", state: "lima" },
      "ref-a": { entryId: "echo", place: "kilo" },
    },
  } } },
  { sequence: "2", actorAdded: { actor: { actorId: "act-wren", name: "Wren" } } },
  { sequence: "3", referenceAdded: {
    actorId: "act-wren", referenceId: "ref-b",
    reference: { entryId: "delta", place: "kilo", state: "oscar" },
  } },
  { sequence: "4", referenceChanged: {
    actorId: "act-rook", referenceId: "ref-b", place: "mike", state: "",
  } },
  { sequence: "5", referenceRemoved: { actorId: "act-rook", referenceId: "ref-a" } },
  { sequence: "6", referenceRemoved: { actorId: "act-wren", referenceId: "ref-b" } },
]);

// VTT-1030
test("a reference's lines name the actor, the entry and the place by the window's names", () => {
  const r = rows(SHEETS, { catalogue: CATALOGUE, places: PLACES });
  names(r[2]!, "Wren", "Grey Cloak", "Back Pack");
  names(r[3]!, "Rook", "Bright Lance", "Left Hand");
  names(r[4]!, "Rook", "Bright Lance", "Back Pack");
  names(r[5]!, "Wren", "Grey Cloak", "Back Pack");
  for (const line of r.slice(2)) omits(line, ...IDS);
});

// VTT-1030
test("a removal names its own reference's entry and place, not another copy's or another sheet's", () => {
  const r = rows(SHEETS, { catalogue: CATALOGUE, places: PLACES });
  omits(r[4]!, "Left Hand", "Grey Cloak");
  omits(r[5]!, "Bright Lance", "Left Hand");
});

// VTT-1030
test("with no names held, the lines name the entry and the place by their ids", () => {
  const r = rows(SHEETS, {});
  names(r[2]!, "Wren", "delta", "kilo");
  names(r[3]!, "Rook", "echo", "mike");
  names(r[4]!, "Rook", "echo", "kilo");
  names(r[5]!, "Wren", "delta", "kilo");
  for (const line of r.slice(2)) omits(line, "Bright Lance", "Grey Cloak", "act-rook", "act-wren");
});

// VTT-1030
test("an entry or a place the window holds no name for is named by its id beside named ones", () => {
  const r = rows(SHEETS, { catalogue: [CATALOGUE[0]!], places: [PLACES[0]!] });
  names(r[2]!, "Wren", "delta", "kilo");
  names(r[3]!, "Rook", "Bright Lance", "Left Hand");
  names(r[4]!, "Rook", "Bright Lance", "kilo");
  omits(r[3]!, "echo", "mike");
  omits(r[4]!, "echo", "Back Pack");
});

// VTT-1030
test("a removal after a change names the place the change set", () => {
  const log = envs([
    { sequence: "1", actorAdded: { actor: {
      actorId: "act-rook", name: "Rook",
      references: { "ref-a": { entryId: "echo", place: "kilo" } },
    } } },
    { sequence: "2", referenceChanged: {
      actorId: "act-rook", referenceId: "ref-a", place: "mike", state: "lima",
    } },
    { sequence: "3", referenceRemoved: { actorId: "act-rook", referenceId: "ref-a" } },
  ]);
  const r = rows(log, { catalogue: CATALOGUE, places: PLACES });
  names(r[2]!, "Rook", "Bright Lance", "Left Hand");
  omits(r[2]!, "Back Pack");
});

const READDED = envs([
  { sequence: "1", actorAdded: { actor: {
    actorId: "act-rook", name: "Rook",
    references: { "ref-a": { entryId: "echo", place: "kilo" } },
  } } },
  { sequence: "2", referenceChanged: {
    actorId: "act-rook", referenceId: "ref-a", place: "mike", state: "lima",
  } },
  { sequence: "3", referenceRemoved: { actorId: "act-rook", referenceId: "ref-a" } },
  { sequence: "4", referenceAdded: {
    actorId: "act-rook", referenceId: "ref-a",
    reference: { entryId: "delta", place: "kilo" },
  } },
]);

// VTT-1030
test("a removal names what its reference held then, though the id is added again later", () => {
  const r = rows(READDED, { catalogue: CATALOGUE, places: PLACES });
  names(r[2]!, "Rook", "Bright Lance", "Left Hand");
  omits(r[2]!, "Grey Cloak", "Back Pack");
  names(r[3]!, "Rook", "Grey Cloak", "Back Pack");
});

// VTT-1030
test("a change names the entry its reference held then, though the id is added again later", () => {
  const r = rows(READDED, { catalogue: CATALOGUE, places: PLACES });
  names(r[1]!, "Rook", "Bright Lance", "Left Hand");
  omits(r[1]!, "Grey Cloak");
});

// VTT-1030
test("a reference's line names its actor as the window names any actor", () => {
  const log = envs([
    { sequence: "1", sceneCreated: { sceneId: "s1", name: "Hall", gridWidth: 4, gridHeight: 4 } },
    { sequence: "2", actorAdded: { actor: { actorId: "act-rook", name: "Rook" } } },
    { sequence: "3", tokenPlaced: {
      tokenId: "tok-rook", sceneId: "s1", actorId: "act-rook",
      position: { x: 1, y: 1 }, gmName: "the masked rook",
    } },
    { sequence: "4", referenceAdded: {
      actorId: "act-rook", referenceId: "ref-a", reference: { entryId: "echo", place: "mike" },
    } },
    { sequence: "5", actorAdded: { actor: {
      actorId: "act-kite", name: "Kite",
      references: { "ref-k": { entryId: "delta", place: "kilo" } },
    } } },
    { sequence: "6", referenceRemoved: { actorId: "act-kite", referenceId: "ref-k" } },
    { sequence: "7", actorRemoved: { actorId: "act-kite" } },
  ]);
  const shown = { catalogue: CATALOGUE, places: PLACES };
  const n = namer(fold(log), log, true, [], [], [], CATALOGUE, PLACES);
  expect(n.actor("act-rook")).toBe("the masked rook");
  expect(n.actor("act-kite")).toBe("Kite");
  const r = rows(log, shown);
  names(r[3]!, "the masked rook", "Bright Lance", "Left Hand");
  names(r[5]!, "Kite", "Grey Cloak", "Back Pack");
  omits(r[3]!, "act-rook");
  omits(r[5]!, "act-kite");
});

// VTT-1029
test("the Story draws no line for a reference added, changed or removed", () => {
  const s = story(SHEETS, { catalogue: CATALOGUE, places: PLACES });
  omits(s, "Bright Lance", "Grey Cloak", "Left Hand", "Back Pack", ...IDS.slice(0, 4), "ref-");
});
