import "./support/dom";
// "Plan" below is
// docs/superpowers/plans/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md,
// and "Ticket" is the ticket that plan names.
import { describe, expect, test } from "bun:test";
import { create, fromJson, type JsonValue } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import {
  CommandResultSchema,
  type ClientCommand,
  type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError, type State, type Token } from "../src/state";
import { renderDMConsole, type DMDeps } from "../src/view/dm";

function env(json: JsonValue): Envelope {
  return fromJson(EnvelopeSchema, json);
}

function scene(seq: number): Envelope {
  return env({ sequence: String(seq), sceneCreated: { sceneId: "s1", name: "Crypt", gridWidth: 10, gridHeight: 10 } });
}

function actor(seq: number): Envelope {
  return env({ sequence: String(seq), actorAdded: { actor: { actorId: "a1", name: "Goblin" } } });
}

function place(
  seq: number, id: string, x: number, y: number,
  name: string, notes: string, gmName: string, gmNotes: string,
): Envelope {
  return env({
    sequence: String(seq),
    tokenPlaced: { tokenId: id, sceneId: "s1", actorId: "a1", position: { x, y }, name, notes, gmName, gmNotes },
  });
}

function describeEv(seq: number, id: string, name: string, notes: string, gmName: string, gmNotes: string): Envelope {
  return env({ sequence: String(seq), tokenDescribed: { tokenId: id, name, notes, gmName, gmNotes } });
}

function token(st: State, id: string): Token {
  const t = st.Tokens[id];
  if (t === undefined) throw new Error(`token ${id} missing`);
  return t;
}

function texts(t: Token): [string | undefined, string | undefined, string | undefined, string | undefined] {
  return [t.Name, t.Notes, t.GMName, t.GMNotes];
}

describe("qaTT: the fold", () => {
  // Ticket item 2: "`engine.Apply` and `client/src/fold.ts` both store them on the
  // token."
  test("qaTT: a placement with all four texts holds them", () => {
    const st = fold([scene(1), actor(2), place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich.")]);
    const t = token(st, "t1");
    expect(texts(t)).toEqual(["Grub", "Limps.", "Grub the Spy", "Reports to the lich."]);
    expect([t.ID, t.SceneID, t.ActorID, t.X, t.Y]).toEqual(["t1", "s1", "a1", 1, 2]);
  });

  // Plan D6: "The fold sets all four on every `tokenPlaced` and `tokenDescribed`,
  // mirroring Go's zero values."
  test("qaTT: a placement with no texts holds four empty texts", () => {
    const st = fold([scene(1), actor(2), place(3, "t2", 3, 4, "", "", "", "")]);
    expect(texts(token(st, "t2"))).toEqual(["", "", "", ""]);
  });

  // VTT-479
  test("qaTT: a describe replaces all four, an emptied one included, and leaves id, scene, actor and position", () => {
    const st = fold([
      scene(1), actor(2),
      place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      describeEv(4, "t1", "", "Now runs.", "", "Turned."),
    ]);
    const t = token(st, "t1");
    expect(texts(t)).toEqual(["", "Now runs.", "", "Turned."]);
    expect([t.ID, t.SceneID, t.ActorID, t.X, t.Y]).toEqual(["t1", "s1", "a1", 1, 2]);
  });

  // VTT-479
  test("qaTT: a second describe replaces the first's texts whole", () => {
    const st = fold([
      scene(1), actor(2),
      place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      describeEv(4, "t1", "", "Now runs.", "", "Turned."),
      describeEv(5, "t1", "Grub", "", "Grub the Spy", ""),
    ]);
    expect(texts(token(st, "t1"))).toEqual(["Grub", "", "Grub the Spy", ""]);
  });

  // VTT-479
  test("qaTT: a describe with four empty texts clears them all", () => {
    const st = fold([
      scene(1), actor(2),
      place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      describeEv(4, "t1", "", "", "", ""),
    ]);
    expect(texts(token(st, "t1"))).toEqual(["", "", "", ""]);
  });

  // VTT-479
  test("qaTT: a describe leaves the other tokens alone", () => {
    const st = fold([
      scene(1), actor(2),
      place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      place(4, "t2", 3, 4, "Snag", "Tall.", "Snag the Elder", "Owes a debt."),
      describeEv(5, "t1", "", "", "", ""),
    ]);
    expect(texts(token(st, "t2"))).toEqual(["Snag", "Tall.", "Snag the Elder", "Owes a debt."]);
  });

  // VTT-480
  test("qaTT: a describe naming no token is refused", () => {
    const log = [scene(1), actor(2), place(3, "t1", 1, 2, "Grub", "", "", ""), describeEv(4, "ghost", "X", "X", "X", "X")];
    let caught: unknown;
    try {
      fold(log);
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(FoldError);
  });

  // VTT-480
  test("qaTT: a describe with an empty token id is refused", () => {
    expect(() => fold([scene(1), actor(2), place(3, "t1", 1, 2, "Grub", "", "", ""), describeEv(4, "", "X", "", "", "")]))
      .toThrow(FoldError);
  });

  // VTT-480
  test("qaTT: a describe of a removed token is refused", () => {
    expect(() => fold([
      scene(1), actor(2), place(3, "t1", 1, 2, "Grub", "", "", ""),
      env({ sequence: "4", tokenRemoved: { tokenId: "t1" } }),
      describeEv(5, "t1", "Back", "", "", ""),
    ])).toThrow(FoldError);
  });

  // Ticket item 2: "A token carries four more fields: a name and notes, which
  // players see, and a GM name and GM notes"
  test("qaTT: the texts survive a move", () => {
    const st = fold([
      scene(1), actor(2),
      place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      env({ sequence: "4", tokenMoved: { tokenId: "t1", sceneId: "s1", from: { x: 1, y: 2 }, to: { x: 2, y: 2 } } }),
    ]);
    const t = token(st, "t1");
    expect(texts(t)).toEqual(["Grub", "Limps.", "Grub the Spy", "Reports to the lich."]);
    expect([t.X, t.Y]).toEqual([2, 2]);
  });

  // Ticket item 2: "None of the four is bounded in bytes."
  test("qaTT: a long text is held whole", () => {
    const long = "ab".repeat(64 * 1024);
    const st = fold([scene(1), actor(2), place(3, "t1", 1, 2, long, long, long, long), describeEv(4, "t1", long + "x", long + "y", long + "z", long + "w")]);
    expect(texts(token(st, "t1"))).toEqual([long + "x", long + "y", long + "z", long + "w"]);
  });
});

// String.raw is handed its non-ASCII letter by interpolation: bun's transpiler
// otherwise rewrites it to an escape that String.raw then keeps.
// Two logs, and the bytes `vtt state dump` printed for them when these tests
// were written: Go's writeDump is the side foldToDumpJSON must match.
const parityLog = (t3Notes: string, t3GMName: string): Envelope[] => [
  scene(1), actor(2),
  place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
  place(4, "t2", 3, 4, "", "", "", ""),
  place(5, "t3", 5, 6, "", t3Notes, t3GMName, ""),
  describeEv(6, "t1", "", "Now runs.", "", "Turned."),
];

const parityDump = String.raw`{
  "Actors": {
    "a1": {
      "actor_id": "a1",
      "name": "Goblin"
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {
    "s1": {
      "ID": "s1",
      "Name": "Crypt",
      "GridWidth": 10,
      "GridHeight": 10,
      "Tiles": {},
      "Objects": [],
      "OpenDoors": {}
    }
  },
  "Sessions": null,
  "Tokens": {
    "t1": {
      "ID": "t1",
      "SceneID": "s1",
      "ActorID": "a1",
      "X": 1,
      "Y": 2,
      "Notes": "Now runs.",
      "GMNotes": "Turned."
    },
    "t2": {
      "ID": "t2",
      "SceneID": "s1",
      "ActorID": "a1",
      "X": 3,
      "Y": 4
    },
    "t3": {
      "ID": "t3",
      "SceneID": "s1",
      "ActorID": "a1",
      "X": 5,
      "Y": 6,
      "Notes": "\u003ctrap\u003e \u0026 \"pit\"",
      "GMName": "${"\u00c5"}sa\u2028x"
    }
  },
  "headSequence": 6
}
`;

const parityDumpPlain = String.raw`{
  "Actors": {
    "a1": {
      "actor_id": "a1",
      "name": "Goblin"
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {
    "s1": {
      "ID": "s1",
      "Name": "Crypt",
      "GridWidth": 10,
      "GridHeight": 10,
      "Tiles": {},
      "Objects": [],
      "OpenDoors": {}
    }
  },
  "Sessions": null,
  "Tokens": {
    "t1": {
      "ID": "t1",
      "SceneID": "s1",
      "ActorID": "a1",
      "X": 1,
      "Y": 2,
      "Notes": "Now runs.",
      "GMNotes": "Turned."
    },
    "t2": {
      "ID": "t2",
      "SceneID": "s1",
      "ActorID": "a1",
      "X": 3,
      "Y": 4
    },
    "t3": {
      "ID": "t3",
      "SceneID": "s1",
      "ActorID": "a1",
      "X": 5,
      "Y": 6,
      "Notes": "Odd.",
      "GMName": "${"\u00c5"}sa"
    }
  },
  "headSequence": 6
}
`;

describe("qaTT: the dump", () => {
  // Plan D5: "`foldToDumpJSON` emits each only when non-empty, after `Y`, in that
  // order."
  test("qaTT: each text is dumped only when non-empty, after Y, in order", () => {
    const dump = foldToDumpJSON([
      scene(1), actor(2),
      place(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      place(4, "t2", 3, 4, "", "", "", ""),
      place(5, "t3", 5, 6, "", "Odd.", "", "Hidden."),
      place(6, "t4", 7, 8, "Lone", "", "", ""),
    ]);
    const want = [
      `  "Tokens": {`,
      `    "t1": {`,
      `      "ID": "t1",`, `      "SceneID": "s1",`, `      "ActorID": "a1",`, `      "X": 1,`, `      "Y": 2,`,
      `      "Name": "Grub",`, `      "Notes": "Limps.",`, `      "GMName": "Grub the Spy",`, `      "GMNotes": "Reports to the lich."`,
      `    },`,
      `    "t2": {`,
      `      "ID": "t2",`, `      "SceneID": "s1",`, `      "ActorID": "a1",`, `      "X": 3,`, `      "Y": 4`,
      `    },`,
      `    "t3": {`,
      `      "ID": "t3",`, `      "SceneID": "s1",`, `      "ActorID": "a1",`, `      "X": 5,`, `      "Y": 6,`,
      `      "Notes": "Odd.",`, `      "GMNotes": "Hidden."`,
      `    },`,
      `    "t4": {`,
      `      "ID": "t4",`, `      "SceneID": "s1",`, `      "ActorID": "a1",`, `      "X": 7,`, `      "Y": 8,`,
      `      "Name": "Lone"`,
      `    }`,
      `  },`,
    ].join("\n");
    expect(dump).toContain(want);
  });

  // Plan D5: "`foldToDumpJSON` emits each only when non-empty, after `Y`, in that
  // order."
  test("qaTT: the TS dump of the shared log is Go's bytes", () => {
    expect(foldToDumpJSON(parityLog("Odd.", "\u00c5sa"))).toBe(parityDumpPlain);
  });

  // writeDump escapes < > &, U+2028 and U+2029: docs/verification-debt.md,
  // 2026-10-24.
  test("qaTT: the TS dump of texts holding < > & and U+2028 is Go's bytes", () => {
    expect(foldToDumpJSON(parityLog("<trap> & \"pit\"", "\u00c5sa\u2028x"))).toBe(parityDump);
  });
});

interface Harness {
  sent: ClientCommand[];
  deps: (st: State) => DMDeps;
}

function harness(): Harness {
  const sent: ClientCommand[] = [];
  return {
    sent,
    deps: (st: State): DMDeps => ({
      st,
      participants: [],
      adventures: [],
      maps: [],
      guideFor: async () => null,
      joinLink: null,
      roster: null,
      origin: "http://localhost",
      refreshSharing: () => {},
      send: async (c: ClientCommand): Promise<CommandResult> => {
        sent.push(c);
        return create(CommandResultSchema, { requestId: c.requestId, ok: true });
      },
      notify: () => {},
      confirm: () => true,
      doorsArmed: false,
      toggleDoors: () => {},
    }),
  };
}

type Field = HTMLInputElement | HTMLTextAreaElement;

function field(root: HTMLElement, key: string): Field {
  const hits = Array.from(root.querySelectorAll("input, textarea"))
    .filter((el) => Array.from(el.attributes).some((a) => a.value === key));
  if (hits.length !== 1) throw new Error(`expected one field keyed ${key}, found ${hits.length}`);
  return hits[0] as Field;
}

function four(root: HTMLElement, id: string): [Field, Field, Field, Field] {
  return [
    field(root, `describe-name:${id}`),
    field(root, `describe-notes:${id}`),
    field(root, `describe-gm-name:${id}`),
    field(root, `describe-gm-notes:${id}`),
  ];
}

function values(root: HTMLElement, id: string): string[] {
  return four(root, id).map((f) => f.value);
}

function type(f: Field, value: string): void {
  f.value = value;
  f.dispatchEvent(new Event("input", { bubbles: true }));
}

function describeButton(root: HTMLElement, id: string): HTMLButtonElement {
  let el: Element | null = field(root, `describe-name:${id}`);
  while (el !== null) {
    const buttons = el.querySelectorAll('[data-action="describe-token"]');
    if (buttons.length === 1) return buttons[0] as HTMLButtonElement;
    if (buttons.length > 1) break;
    el = el.parentElement;
  }
  throw new Error(`no single describe-token button in the row of ${id}`);
}

async function settle(): Promise<void> {
  await new Promise((r) => setTimeout(r, 0));
}

function stateWith(id: string, name: string, notes: string, gmName: string, gmNotes: string): State {
  return fold([scene(1), actor(2), place(3, id, 1, 2, name, notes, gmName, gmNotes)]);
}

function describedArm(c: ClientCommand | undefined): [string, string, string, string, string] {
  if (c === undefined || c.command.case !== "describeToken") throw new Error(`sent ${c?.command.case} instead of describeToken`);
  const v = c.command.value;
  return [v.tokenId, v.name, v.notes, v.gmName, v.gmNotes];
}

describe("qaTT: the DM console's describe rows", () => {
  // Plan D16: "From Task 3, which gives the TS `Token` its four fields (D6), a field
  // with no draft shows the token's present value."
  test("qaTT: a field with no draft shows the token's present text", () => {
    const h = harness();
    const root = renderDMConsole(h.deps(stateWith("c1", "Grub", "Limps.", "Grub the Spy", "Reports to the lich.")));
    const [name, notes, gmName, gmNotes] = four(root, "c1");
    expect([name.tagName, notes.tagName, gmName.tagName, gmNotes.tagName]).toEqual(["INPUT", "TEXTAREA", "INPUT", "TEXTAREA"]);
    expect(values(root, "c1")).toEqual(["Grub", "Limps.", "Grub the Spy", "Reports to the lich."]);
  });

  // Plan D16: "a field with no draft shows the token's present value."
  test("qaTT: a token with no texts shows four empty fields", () => {
    const h = harness();
    const root = renderDMConsole(h.deps(stateWith("c2", "", "", "", "")));
    expect(values(root, "c2")).toEqual(["", "", "", ""]);
  });

  // Plan D16: "Each field shows its draft."
  test("qaTT: a typed draft wins across a repaint, a newer present text included", async () => {
    const h = harness();
    const st = stateWith("c3", "Grub", "Limps.", "Grub the Spy", "Reports to the lich.");
    const first = renderDMConsole(h.deps(st));
    type(field(first, "describe-name:c3"), "Typed name");
    type(field(first, "describe-gm-notes:c3"), "Typed gm notes");
    const second = renderDMConsole(h.deps(st));
    expect(values(second, "c3")).toEqual(["Typed name", "Limps.", "Grub the Spy", "Typed gm notes"]);

    const newer = fold([
      scene(1), actor(2), place(3, "c3", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
      describeEv(4, "c3", "Other", "Other notes", "Other gm", "Other gm notes"),
    ]);
    const third = renderDMConsole(h.deps(newer));
    expect(values(third, "c3")).toEqual(["Typed name", "Other notes", "Other gm", "Typed gm notes"]);
    describeButton(third, "c3").click();
    await settle();
  });

  // Plan D16: "Each field shows its draft."
  test("qaTT: an emptied draft stays empty across a repaint and is sent empty", async () => {
    const h = harness();
    const st = stateWith("c4", "Grub", "Limps.", "Grub the Spy", "Reports to the lich.");
    const first = renderDMConsole(h.deps(st));
    type(field(first, "describe-notes:c4"), "");
    const second = renderDMConsole(h.deps(st));
    expect(values(second, "c4")).toEqual(["Grub", "", "Grub the Spy", "Reports to the lich."]);

    describeButton(second, "c4").click();
    await settle();
    expect(h.sent.length).toBe(1);
    expect(describedArm(h.sent[0])).toEqual(["c4", "Grub", "", "Grub the Spy", "Reports to the lich."]);
  });

  // VTT-479; plan D16: "The button sends `describeToken(id, name, notes, gmName,
  // gmNotes)` as typed, and clears that row's four drafts."
  test("qaTT: Describe sends the four fields as shown, untouched ones with their present text", async () => {
    const h = harness();
    const st = stateWith("c5", "Grub", "Limps.", "Grub the Spy", "Reports to the lich.");
    const root = renderDMConsole(h.deps(st));
    type(field(root, "describe-name:c5"), "Renamed");
    describeButton(root, "c5").click();
    await settle();
    expect(h.sent.length).toBe(1);
    expect(describedArm(h.sent[0])).toEqual(["c5", "Renamed", "Limps.", "Grub the Spy", "Reports to the lich."]);
  });

  // Plan D16: "The button sends ... as typed, and clears that row's four drafts."
  test("qaTT: after Describe is clicked, the present text shows again", async () => {
    const h = harness();
    const st = stateWith("c6", "Grub", "Limps.", "Grub the Spy", "Reports to the lich.");
    const first = renderDMConsole(h.deps(st));
    type(field(first, "describe-name:c6"), "Typed name");
    type(field(first, "describe-notes:c6"), "");
    type(field(first, "describe-gm-name:c6"), "Typed gm");
    type(field(first, "describe-gm-notes:c6"), "Typed gm notes");
    describeButton(first, "c6").click();
    await settle();
    const second = renderDMConsole(h.deps(st));
    expect(values(second, "c6")).toEqual(["Grub", "Limps.", "Grub the Spy", "Reports to the lich."]);
  });

  // Plan D16: "clears that row's four drafts."
  test("qaTT: Describe on one row keeps another row's drafts", async () => {
    const h = harness();
    const st = fold([
      scene(1), actor(2),
      place(3, "c7", 1, 2, "Grub", "Limps.", "", ""),
      place(4, "c8", 3, 4, "Snag", "Tall.", "", ""),
    ]);
    const first = renderDMConsole(h.deps(st));
    type(field(first, "describe-name:c7"), "Seven");
    type(field(first, "describe-name:c8"), "Eight");
    describeButton(first, "c7").click();
    await settle();
    const second = renderDMConsole(h.deps(st));
    expect(field(second, "describe-name:c7").value).toBe("Grub");
    expect(field(second, "describe-name:c8").value).toBe("Eight");
    describeButton(second, "c8").click();
    await settle();
  });

  // Plan D16: "draws one row per token of the world, sorted by id."
  test("qaTT: one row per token, sorted by id", () => {
    const h = harness();
    const st = fold([
      scene(1), actor(2),
      place(3, "k3", 1, 1, "", "", "", ""),
      place(4, "k1", 2, 2, "", "", "", ""),
      place(5, "k2", 3, 3, "", "", "", ""),
    ]);
    const root = renderDMConsole(h.deps(st));
    const ids = Array.from(root.querySelectorAll("input"))
      .flatMap((el) => Array.from(el.attributes).map((a) => a.value))
      .filter((v) => v.startsWith("describe-name:"))
      .map((v) => v.slice("describe-name:".length));
    expect(ids).toEqual(["k1", "k2", "k3"]);
    expect(root.querySelectorAll('[data-action="describe-token"]').length).toBe(3);
  });
});
