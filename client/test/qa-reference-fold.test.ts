import { test, expect } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fromJson } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError, type Reference, type State } from "../src/state";

// Warning: each dump below is what `vtt state dump`'s printer wrote for
// the log above it. Never edit one by hand; fold the log in engine.Apply
// again.

const envs = (items: readonly unknown[]): Envelope[] =>
  items.map((e) => fromJson(EnvelopeSchema, e as never));

const SHEET_LOG = envs([
  {"eventId": "e1", "sequence": "1", "sessionId": "sess-1", "sessionStarted": {"name": "S"}},
  {"eventId": "e2", "sequence": "2", "actorAdded": {"actor": {"actorId": "act-rook", "name": "Rook", "moduleId": "m", "kind": "ACTOR_KIND_PARTY_MEMBER", "attributes": {"str": 3}, "resources": {"hp": {"current": 5, "max": 9}}, "abilityIds": ["rally"], "references": {"ref-b": {"entryId": "echo", "place": "mike", "state": "lima"}, "ref-a": {"entryId": "echo", "place": "kilo"}}}}},
  {"eventId": "e3", "sequence": "3", "actorAdded": {"actor": {"actorId": "act-wren", "name": "Wren", "kind": "ACTOR_KIND_NON_PARTY"}}},
  {"eventId": "e4", "sequence": "4", "actorControlGranted": {"actorId": "act-rook", "participantId": "p-1", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "e5", "sequence": "5", "referenceAdded": {"actorId": "act-wren", "referenceId": "ref-b", "reference": {"entryId": "delta", "place": "kilo", "state": "oscar"}}},
  {"eventId": "e6", "sequence": "6", "referenceAdded": {"actorId": "act-wren", "referenceId": "ref-c", "reference": {}}},
  {"eventId": "e7", "sequence": "7", "referenceChanged": {"actorId": "act-rook", "referenceId": "ref-b", "place": "kilo", "state": ""}},
  {"eventId": "e8", "sequence": "8", "referenceRemoved": {"actorId": "act-rook", "referenceId": "ref-a"}},
  {"eventId": "e9", "sequence": "9", "referenceChanged": {"actorId": "act-wren", "referenceId": "ref-b", "place": "mike", "state": "lima"}},
  {"eventId": "e10", "sequence": "10", "referenceRemoved": {"actorId": "act-wren", "referenceId": "ref-c"}},
  {"eventId": "e11", "sequence": "11", "referenceAdded": {"actorId": "act-rook", "referenceId": "ref-a", "reference": {"entryId": "delta", "place": "mike", "state": "lima"}}},
  {"eventId": "e12", "sequence": "12", "actorAdded": {"actor": {"actorId": "act-kite", "name": "Kite", "references": {"ref-k": {"entryId": "delta"}}}}},
  {"eventId": "e13", "sequence": "13", "referenceRemoved": {"actorId": "act-kite", "referenceId": "ref-k"}},
  {"eventId": "e14", "sequence": "14", "referenceChanged": {"actorId": "act-rook", "referenceId": "ref-a", "place": "mike", "state": "lima"}}
]);

const SHEET_DUMP = String.raw`{
  "Actors": {
    "act-kite": {
      "actor_id": "act-kite",
      "name": "Kite"
    },
    "act-rook": {
      "actor_id": "act-rook",
      "name": "Rook",
      "module_id": "m",
      "attributes": {
        "str": 3
      },
      "resources": {
        "hp": {
          "current": 5,
          "max": 9
        }
      },
      "controller_id": "p-1",
      "controller_ids": [
        "p-1"
      ],
      "kind": 1,
      "ability_ids": [
        "rally"
      ],
      "references": {
        "ref-a": {
          "entry_id": "delta",
          "place": "mike",
          "state": "lima"
        },
        "ref-b": {
          "entry_id": "echo",
          "place": "kilo"
        }
      }
    },
    "act-wren": {
      "actor_id": "act-wren",
      "name": "Wren",
      "kind": 2,
      "references": {
        "ref-b": {
          "entry_id": "delta",
          "place": "mike",
          "state": "lima"
        }
      }
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {},
  "Sessions": [
    {
      "ID": "sess-1",
      "Name": "S",
      "StartSeq": 1,
      "EndSeq": 0
    }
  ],
  "Tokens": {},
  "headSequence": 14
}
`;

const SHEET_REFUSED_JSON = [
  {"eventId": "r0", "sequence": "20", "actorAdded": {"actor": {"actorId": "act-new", "references": {"": {"entryId": "echo"}}}}},
  {"eventId": "r1", "sequence": "20", "actorAdded": {"actor": {"actorId": "act-rook", "references": {"": {}}}}},
  {"eventId": "r2", "sequence": "20", "actorAdded": {"actor": {"actorId": "act-new", "attributes": {"": 1}, "references": {"": {}}}}},
  {"eventId": "r3", "sequence": "20", "actorAdded": {"actor": {"actorId": "act-new", "resources": {"": {"current": 1}}, "references": {"": {}}}}},
  {"eventId": "r4", "sequence": "20", "actorAdded": {"actor": {"actorId": "act-new", "controllerIds": ["p-1"], "references": {"": {}}}}},
  {"eventId": "r5", "sequence": "20", "referenceAdded": {"actorId": "act-ghost", "referenceId": ""}},
  {"eventId": "r6", "sequence": "20", "referenceAdded": {"actorId": "act-ghost", "referenceId": "ref-x"}},
  {"eventId": "r7", "sequence": "20", "referenceAdded": {"actorId": "act-ghost", "referenceId": "ref-x", "reference": {"entryId": "echo"}}},
  {"eventId": "r8", "sequence": "20", "referenceAdded": {"actorId": "act-rook", "referenceId": "ref-b", "reference": {"entryId": "delta"}}},
  {"eventId": "r9", "sequence": "20", "referenceAdded": {"actorId": "act-rook", "referenceId": "", "reference": {"entryId": "delta"}}},
  {"eventId": "r10", "sequence": "20", "referenceRemoved": {"actorId": "act-ghost", "referenceId": ""}},
  {"eventId": "r11", "sequence": "20", "referenceRemoved": {"actorId": "act-wren", "referenceId": "ref-c"}},
  {"eventId": "r12", "sequence": "20", "referenceRemoved": {"actorId": "act-kite", "referenceId": "ref-b"}},
  {"eventId": "r13", "sequence": "20", "referenceRemoved": {"actorId": "act-rook", "referenceId": ""}},
  {"eventId": "r14", "sequence": "20", "referenceChanged": {"actorId": "act-ghost", "referenceId": "", "place": "mike"}},
  {"eventId": "r15", "sequence": "20", "referenceChanged": {"actorId": "act-rook", "referenceId": "ref-z", "place": "mike"}},
  {"eventId": "r16", "sequence": "20", "referenceChanged": {"actorId": "act-kite", "referenceId": "ref-a", "place": "mike"}},
  {"eventId": "r17", "sequence": "20", "referenceChanged": {"actorId": "", "referenceId": "ref-a"}}
];

const SHEET_REFUSED = envs(SHEET_REFUSED_JSON);

// Warning: each is engine.Apply's word for the envelope at its index.
const SHEET_WORDS = [
  "engine: actor_added: actor \"act-new\" holds a reference with an empty id",
  null,
  null,
  null,
  null,
  "engine: reference_added requires a reference id",
  "engine: reference_added requires a reference",
  "engine: reference_added names unknown actor \"act-ghost\"",
  "engine: actor \"act-rook\" already holds reference \"ref-b\"",
  "engine: reference_added requires a reference id",
  "engine: reference_removed names unknown actor \"act-ghost\"",
  "engine: actor \"act-wren\" does not hold reference \"ref-c\"",
  "engine: actor \"act-kite\" does not hold reference \"ref-b\"",
  "engine: actor \"act-rook\" does not hold reference \"\"",
  "engine: reference_changed names unknown actor \"act-ghost\"",
  "engine: actor \"act-rook\" does not hold reference \"ref-z\"",
  "engine: actor \"act-kite\" does not hold reference \"ref-a\"",
  "engine: reference_changed names unknown actor \"\"",
];

const PROTO_LOG = envs([
  {"eventId": "e1", "sequence": "1", "actorAdded": {"actor": {"actorId": "act-rook", "name": "Rook", "references": {"valueOf": {"entryId": "echo", "place": "mike"}}}}},
  {"eventId": "e2", "sequence": "2", "referenceAdded": {"actorId": "act-rook", "referenceId": "constructor", "reference": {"entryId": "echo", "place": "mike"}}},
  {"eventId": "e3", "sequence": "3", "referenceAdded": {"actorId": "act-rook", "referenceId": "__proto__", "reference": {"entryId": "delta", "place": "kilo", "state": "oscar"}}},
  {"eventId": "e4", "sequence": "4", "referenceChanged": {"actorId": "act-rook", "referenceId": "constructor", "place": "kilo", "state": "lima"}},
  {"eventId": "e5", "sequence": "5", "referenceRemoved": {"actorId": "act-rook", "referenceId": "valueOf"}}
]);

const PROTO_DUMP = String.raw`{
  "Actors": {
    "act-rook": {
      "actor_id": "act-rook",
      "name": "Rook",
      "references": {
        "__proto__": {
          "entry_id": "delta",
          "place": "kilo",
          "state": "oscar"
        },
        "constructor": {
          "entry_id": "echo",
          "place": "kilo",
          "state": "lima"
        }
      }
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {},
  "Sessions": null,
  "Tokens": {},
  "headSequence": 5
}
`;

const PROTO_REFUSED = envs([
  {"eventId": "r0", "sequence": "9", "referenceRemoved": {"actorId": "act-rook", "referenceId": "toString"}},
  {"eventId": "r1", "sequence": "9", "referenceChanged": {"actorId": "act-rook", "referenceId": "hasOwnProperty", "place": "mike"}},
  {"eventId": "r2", "sequence": "9", "referenceAdded": {"actorId": "act-rook", "referenceId": "constructor", "reference": {"entryId": "delta"}}},
  {"eventId": "r3", "sequence": "9", "referenceAdded": {"actorId": "act-rook", "referenceId": "__proto__", "reference": {"entryId": "delta"}}},
  {"eventId": "r4", "sequence": "9", "referenceAdded": {"actorId": "constructor", "referenceId": "ref-x", "reference": {"entryId": "delta"}}}
]);

const PROTO_WORDS = [
  "engine: actor \"act-rook\" does not hold reference \"toString\"",
  "engine: actor \"act-rook\" does not hold reference \"hasOwnProperty\"",
  "engine: actor \"act-rook\" already holds reference \"constructor\"",
  "engine: actor \"act-rook\" already holds reference \"__proto__\"",
  "engine: reference_added names unknown actor \"constructor\"",
];

const ESCAPE_LOG = envs([
  {"eventId": "e1", "sequence": "1", "actorAdded": {"actor": {"actorId": "act-rook", "name": "Rook", "references": {"b": {"entryId": "echo"}, "B": {"entryId": "delta"}}}}},
  {"eventId": "e2", "sequence": "2", "referenceAdded": {"actorId": "act-rook", "referenceId": "a<b&c>", "reference": {"entryId": "delta", "place": "mike"}}},
  {"eventId": "e3", "sequence": "3", "referenceAdded": {"actorId": "act-rook", "referenceId": "A", "reference": {"entryId": "echo", "place": "kilo", "state": "oscar"}}}
]);

const ESCAPE_DUMP = String.raw`{
  "Actors": {
    "act-rook": {
      "actor_id": "act-rook",
      "name": "Rook",
      "references": {
        "A": {
          "entry_id": "echo",
          "place": "kilo",
          "state": "oscar"
        },
        "B": {
          "entry_id": "delta"
        },
        "a\u003cb\u0026c\u003e": {
          "entry_id": "delta",
          "place": "mike"
        },
        "b": {
          "entry_id": "echo"
        }
      }
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {},
  "Sessions": null,
  "Tokens": {},
  "headSequence": 3
}
`;

const BYTE_ORDER_LOG = envs([
  {"eventId": "e1", "sequence": "1", "actorAdded": {"actor": {"actorId": "act-rook", "name": "Rook", "references": {"9": {"entryId": "echo"}, "10": {"entryId": "delta"}}}}},
  {"eventId": "e2", "sequence": "2", "referenceAdded": {"actorId": "act-rook", "referenceId": "2", "reference": {"entryId": "echo", "place": "kilo"}}},
  {"eventId": "e3", "sequence": "3", "referenceAdded": {"actorId": "act-rook", "referenceId": "\uff5e", "reference": {"entryId": "echo"}}},
  {"eventId": "e4", "sequence": "4", "referenceAdded": {"actorId": "act-rook", "referenceId": "\ud83d\ude00", "reference": {"entryId": "delta"}}}
]);

const BYTE_ORDER_DUMP = `{
  "Actors": {
    "act-rook": {
      "actor_id": "act-rook",
      "name": "Rook",
      "references": {
        "10": {
          "entry_id": "delta"
        },
        "2": {
          "entry_id": "echo",
          "place": "kilo"
        },
        "9": {
          "entry_id": "echo"
        },
        "～": {
          "entry_id": "echo"
        },
        "😀": {
          "entry_id": "delta"
        }
      }
    }
  },
  "Conditions": {},
  "Notes": {},
  "Scenes": {},
  "Sessions": null,
  "Tokens": {},
  "headSequence": 4
}
`;

function refusalOf(log: readonly Envelope[], e: Envelope): string {
  try {
    fold([...log, e]);
  } catch (err) {
    if (err instanceof FoldError) return err.message;
    throw err;
  }
  return "ACCEPTED";
}

// The client words a reference refusal as engine.Apply does, less its
// `engine: ` prefix (D10 of the plan
// docs/superpowers/plans/2026-11-25-a-sheet-refers-to-the-catalogue.md).
function mirrorsServer(got: string, server: string): void {
  expect(got).toBe(server.replace(/^engine: /, ""));
}

const ref = (entryId: string, place: string, state: string): Reference =>
  ({ entryId, place, state });

const upTo = (seq: number): Envelope[] =>
  SHEET_LOG.filter((e) => Number(e.sequence) <= seq);

function only(log: readonly Envelope[], seq: number): Envelope {
  const e = log.find((x) => Number(x.sequence) === seq);
  if (e === undefined) throw new Error(`no entry ${seq}`);
  return e;
}

function actorOf(st: State, id: string) {
  const a = st.Actors[id];
  if (a === undefined) throw new Error(`no actor ${id}`);
  return a;
}

// VTT-1023, VTT-1024, VTT-1025, VTT-1026
test("a log of reference events dumps as the server's fold dumps it", () => {
  expect(foldToDumpJSON(SHEET_LOG)).toBe(SHEET_DUMP);
});

// VTT-1023, VTT-1024, VTT-1025, VTT-1026
test("each reference refusal is the server's, in the server's order", () => {
  expect(SHEET_REFUSED.length).toBe(SHEET_WORDS.length);
  SHEET_REFUSED.forEach((e, i) => {
    const server = SHEET_WORDS[i];
    if (server === undefined || server === null) return;
    mirrorsServer(refusalOf(SHEET_LOG, e), server);
  });
});

// VTT-1023
test("an ActorAdded refuses an empty reference id after its id, controller, resource and attribute checks", () => {
  const withoutTheReference = (e: Envelope): Envelope => {
    const c = envs([SHEET_REFUSED_JSON[SHEET_REFUSED.indexOf(e)]])[0]!;
    if (c.payload.case !== "actorAdded" || c.payload.value.actor === undefined) {
      throw new Error("not an actorAdded");
    }
    c.payload.value.actor.references = {};
    return c;
  };
  for (const i of [1, 2, 3, 4]) {
    const e = SHEET_REFUSED[i]!;
    const got = refusalOf(SHEET_LOG, e);
    expect(got).not.toContain("reference");
    expect(got).toBe(refusalOf(SHEET_LOG, withoutTheReference(e)));
    expect(got).not.toBe("ACCEPTED");
  }
});

// VTT-1023
test("an ActorAdded's references are held whole, empty fields included", () => {
  const st = fold(upTo(3));
  expect(actorOf(st, "act-rook").references).toEqual({
    "ref-b": ref("echo", "mike", "lima"),
    "ref-a": ref("echo", "kilo", ""),
  });
  expect(actorOf(st, "act-wren").references).toEqual({});
  expect(actorOf(fold(upTo(12)), "act-kite").references).toEqual({ "ref-k": ref("delta", "", "") });
});

// VTT-1024
test("a ReferenceAdded adds the reference to its actor's sheet and changes nothing else", () => {
  const want = fold(upTo(4));
  actorOf(want, "act-wren").references = { "ref-b": ref("delta", "kilo", "oscar") };
  expect(fold(upTo(5))).toEqual(want);

  const empty = fold(upTo(5));
  actorOf(empty, "act-wren").references = {
    "ref-b": ref("delta", "kilo", "oscar"),
    "ref-c": ref("", "", ""),
  };
  expect(fold(upTo(6))).toEqual(empty);
});

// VTT-1025
test("a ReferenceRemoved takes only that reference off its actor's sheet", () => {
  const want = fold(upTo(7));
  actorOf(want, "act-rook").references = { "ref-b": ref("echo", "kilo", "") };
  expect(fold(upTo(8))).toEqual(want);

  const emptied = fold(upTo(12));
  actorOf(emptied, "act-kite").references = {};
  expect(fold(upTo(13))).toEqual(emptied);
});

// VTT-1026
test("a ReferenceChanged sets the place and the state, keeps the entry and changes nothing else", () => {
  const intoNoState = fold(upTo(6));
  actorOf(intoNoState, "act-rook").references = {
    "ref-b": ref("echo", "kilo", ""),
    "ref-a": ref("echo", "kilo", ""),
  };
  expect(fold(upTo(7))).toEqual(intoNoState);

  const want = fold(upTo(8));
  actorOf(want, "act-wren").references = {
    "ref-b": ref("delta", "mike", "lima"),
    "ref-c": ref("", "", ""),
  };
  expect(fold(upTo(9))).toEqual(want);
});

// VTT-1026
test("a ReferenceChanged to the place and state it holds changes nothing", () => {
  expect(only(SHEET_LOG, 14).payload.case).toBe("referenceChanged");
  expect(fold(upTo(14))).toEqual(fold(upTo(13)));
});

// VTT-1024, VTT-1025, VTT-1026
test("folding a log changes none of the envelopes it folds", () => {
  const before = JSON.stringify(SHEET_LOG.map((e) => e.payload));
  fold(SHEET_LOG);
  foldToDumpJSON(SHEET_LOG);
  expect(JSON.stringify(SHEET_LOG.map((e) => e.payload))).toBe(before);
  expect(foldToDumpJSON(SHEET_LOG)).toBe(SHEET_DUMP);
});

// VTT-1024, VTT-1025, VTT-1026
test("reference ids that name a prototype member in a reference event fold and refuse as the server's do", () => {
  expect(foldToDumpJSON(PROTO_LOG)).toBe(PROTO_DUMP);
  expect(PROTO_REFUSED.length).toBe(PROTO_WORDS.length);
  PROTO_REFUSED.forEach((e, i) => mirrorsServer(refusalOf(PROTO_LOG, e), PROTO_WORDS[i]!));
});

// VTT-1023, VTT-1024
test("a sheet's references dump with the server's escaping, in its order", () => {
  expect(foldToDumpJSON(ESCAPE_LOG)).toBe(ESCAPE_DUMP);
});

// VTT-1023, VTT-1024
test("a sheet's reference ids dump in the server's byte order, digits and astral ids included", () => {
  expect(foldToDumpJSON(BYTE_ORDER_LOG)).toBe(BYTE_ORDER_DUMP);
});

const fixture = (name: string): string =>
  readFileSync(join(import.meta.dir, "../../contract/testdata", name), "utf8");

// VTT-1028
test("the committed reference log folds to the committed dump", () => {
  const log = envs(JSON.parse(fixture("reference_change_log.json")));
  expect(foldToDumpJSON(log)).toBe(fixture("reference_change_dump.json"));
});

// VTT-1028
test("each envelope of the committed refused log is refused after the committed log", () => {
  const log = envs(JSON.parse(fixture("reference_change_log.json")));
  const refused = envs(JSON.parse(fixture("reference_change_refused.json")));
  expect(refused.length).toBeGreaterThan(0);
  for (const e of refused) expect(refusalOf(log, e)).not.toBe("ACCEPTED");
});
