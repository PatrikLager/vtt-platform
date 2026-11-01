import "./support/dom";
import { describe as group, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { create, fromJson } from "@bufbuild/protobuf";
import {
  ActorAddedSchema,
  ActorKind,
  ActorSchema,
  EnvelopeSchema,
  NarrationAddedSchema,
  WhisperAddedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold } from "../src/fold";
import { FoldError } from "../src/state";
import { describe, renderSpectator } from "../src/view/spectator";
import { namer, UNNAMED } from "../src/view/naming";

let seq = 0n;

function actor(id: string, name: string, kind = ActorKind.PARTY_MEMBER): Envelope {
  seq += 1n;
  return create(EnvelopeSchema, {
    eventId: `qaw-${seq}`,
    sequence: seq,
    payload: { case: "actorAdded", value: create(ActorAddedSchema, { actor: create(ActorSchema, { actorId: id, name, kind }) }) },
  });
}

function whisper(text: string, from: string, ...to: string[]): Envelope {
  seq += 1n;
  return create(EnvelopeSchema, {
    eventId: `qaw-${seq}`,
    sequence: seq,
    payload: { case: "whisperAdded", value: create(WhisperAddedSchema, { text, fromActorId: from, toActorIds: to }) },
  });
}

function narration(text: string): Envelope {
  seq += 1n;
  return create(EnvelopeSchema, {
    eventId: `qaw-${seq}`,
    sequence: seq,
    payload: { case: "narrationAdded", value: create(NarrationAddedSchema, { text }) },
  });
}

function table(): Envelope[] {
  return [actor("tobbe", "Tobbe"), actor("anna", "Anna"), actor("nameless", ""), actor("goblin", "Goblin", ActorKind.NON_PARTY)];
}

function names(log: Envelope[], dm = false) {
  return namer(fold(log), log, dm, undefined);
}

function draw(log: Envelope[], ticker: boolean): HTMLElement {
  const root = document.createElement("div");
  renderSpectator(root, fold(log), log, "open", { ticker, dm: ticker });
  return root;
}

const shapes: [string, (t: string) => Envelope][] = [
  ["to a party member", (t) => whisper(t, "", "tobbe")],
  ["from a party member", (t) => whisper(t, "tobbe")],
];

// Flatten a concatenated string first: Bun's TextEncoder.encode undercounts a rope.
const flat = (s: string): string => Array.from(s).join("");

const bounds: [string, string, boolean][] = [
  ["empty", "", false],
  ["8192 ascii", "a".repeat(8192), true],
  ["8193 ascii", "a".repeat(8193), false],
  ["8192 bytes of four-byte runes", "𝄞".repeat(2048), true],
  ["8196 bytes in 4098 UTF-16 units", "𝄞".repeat(2049), false],
  ["8192 code points, 8193 bytes", flat("a".repeat(8191) + "é"), false],
];

group("qa: the client fold bounds a whisper's text in bytes", () => {
  for (const [shape, mk] of shapes) {
    for (const [name, text, ok] of bounds) {
      test(`a whisper ${shape}, ${name}, ${ok ? "folds" : "is refused"}`, () => {
        // VTT-585
        const log = [...table(), mk(text)];
        if (ok) expect(() => fold(log)).not.toThrow();
        else expect(() => fold(log)).toThrow(FoldError);
      });
    }
  }
});

group("qa: the client fold refuses a whisper naming an actor it has not folded", () => {
  const cases: [string, Envelope][] = [
    ["to an unknown actor", whisper("hi", "", "nobody")],
    ["to a known and an unknown actor", whisper("hi", "", "tobbe", "nobody")],
    ["from an unknown actor", whisper("hi", "nobody")],
  ];
  for (const [name, w] of cases) {
    test(`a whisper ${name} is refused`, () => {
      // VTT-586
      expect(() => fold([...table(), w])).toThrow(FoldError);
    });
  }

  test("a whisper to an actor added only later is refused", () => {
    // VTT-586
    expect(() => fold([whisper("early", "", "tobbe"), actor("tobbe", "Tobbe")])).toThrow(FoldError);
  });

  test("a whisper to and from known actors folds and changes no actor", () => {
    // VTT-586
    const before = fold(table());
    const after = fold([...table(), whisper("hi", "", "tobbe", "anna"), whisper("yo", "anna")]);
    expect(after.Actors).toEqual(before.Actors);
  });
});

group("qa: describe words a whisper", () => {
  test("a whisper to two party members names both in order, joined by a comma", () => {
    // VTT-597
    const log = [...table(), whisper("You smell smoke.", "", "tobbe", "anna")];
    expect(describe(log.at(-1)!, names(log))).toBe("The DM whispers to Tobbe, Anna: You smell smoke.");
  });

  test("a whisper to one party member names it alone", () => {
    // VTT-597
    const log = [...table(), whisper("Only you.", "", "anna")];
    expect(describe(log.at(-1)!, names(log, true))).toBe("The DM whispers to Anna: Only you.");
  });

  test("a whisper from a party member reads as it whispering to the DM", () => {
    // VTT-597
    const log = [...table(), whisper("I pocket the ring.", "tobbe")];
    expect(describe(log.at(-1)!, names(log))).toBe("Tobbe whispers to the DM: I pocket the ring.");
  });

  test("a nameless party member is named as the window names it, never by its id", () => {
    // VTT-597
    const log = [...table(), whisper("psst", "", "nameless", "tobbe"), whisper("back", "nameless")];
    const n = names(log);
    const expected = n.actor("nameless");
    expect(expected).not.toContain("nameless");
    expect(describe(log.at(-2)!, n)).toBe(`The DM whispers to ${expected}, Tobbe: psst`);
    expect(describe(log.at(-1)!, n)).toBe(`${expected} whispers to the DM: back`);
    expect(expected).toBe(UNNAMED);
  });

  test("the contract's whisper fixture is worded with its addressee's name", () => {
    // VTT-597
    const raw = JSON.parse(readFileSync(new URL("../../contract/testdata/whisper_added_envelope.json", import.meta.url), "utf8"));
    const env = fromJson(EnvelopeSchema, raw);
    const log = [actor("act-tobbe", "Tobbe"), env];
    expect(describe(env, names(log))).toBe("The DM whispers to Tobbe: An arrow came from the dark, from the east.");
  });
});

group("qa: the Story and the Events panel draw a whisper", () => {
  const log = (): Envelope[] => [
    ...table(),
    narration("The torches gutter."),
    whisper("You smell smoke.", "", "tobbe", "anna"),
    whisper("I pocket the ring.", "tobbe"),
  ];

  for (const ticker of [true, false]) {
    test(`the Story draws both whispers with the class whisper (${ticker ? "DM" : "player"} window)`, () => {
      // VTT-597
      const root = draw(log(), ticker);
      const feed = root.querySelector("section.feed");
      expect(feed).not.toBeNull();
      const lines = Array.from(feed!.querySelectorAll(".whisper")).map((e) => (e.textContent ?? "").trim());
      expect(lines).toEqual(["The DM whispers to Tobbe, Anna: You smell smoke.", "Tobbe whispers to the DM: I pocket the ring."]);
      expect((feed!.textContent ?? "").includes("The torches gutter.")).toBe(true);
    });
  }

  test("the Events panel draws each whisper as describe words it", () => {
    // VTT-597
    const root = draw(log(), true);
    const ticks = Array.from(root.querySelectorAll(".tick")).map((e) => (e.textContent ?? "").trim());
    expect(ticks).toContain("The DM whispers to Tobbe, Anna: You smell smoke.");
    expect(ticks).toContain("Tobbe whispers to the DM: I pocket the ring.");
  });
});
