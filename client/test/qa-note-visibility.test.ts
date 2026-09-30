// Independent QA for VTT-228, derived from the requirement and the ticket
// docs/superpowers/specs/2026-09-29-a-note-says-who-may-read-it-design.md
// ("Done looks like" item 5), without reading the implementation.
import { test, expect } from "bun:test";
import { fromJson } from "@bufbuild/protobuf";
import {
  EnvelopeSchema,
  NoteVisibility,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import type { State } from "../src/state";

const NONE = NoteVisibility.UNSPECIFIED;
const PUBLIC = NoteVisibility.PUBLIC;
const SECRET = NoteVisibility.SECRET;

const WIRE: Record<NoteVisibility, string | undefined> = {
  [NoteVisibility.UNSPECIFIED]: undefined,
  [NoteVisibility.PUBLIC]: "NOTE_VISIBILITY_PUBLIC",
  [NoteVisibility.SECRET]: "NOTE_VISIBILITY_SECRET",
};

function upsert(seq: number, key: string, vis: NoteVisibility): Envelope {
  const note: Record<string, string> = { key, title: `title ${key}`, text: `text of ${key}` };
  const wire = WIRE[vis];
  if (wire !== undefined) note.visibility = wire;
  return fromJson(EnvelopeSchema, { sequence: String(seq), noteUpserted: note });
}

function del(seq: number, key: string): Envelope {
  return fromJson(EnvelopeSchema, { sequence: String(seq), noteDeleted: { key } });
}

// visibilityOf throws when the note is absent, so a zero read off a missing
// note can never pass as "none recorded".
function visibilityOf(st: State, key: string): NoteVisibility {
  const n = st.Notes[key];
  if (n === undefined) throw new Error(`note ${key} absent: ${JSON.stringify(st.Notes)}`);
  return n.Visibility;
}

// VTT-228
test("a note records the public visibility its upsert stated", () => {
  expect(visibilityOf(fold([upsert(1, "k", PUBLIC)]), "k")).toBe(PUBLIC);
});

// VTT-228
test("a note records the secret visibility its upsert stated", () => {
  expect(visibilityOf(fold([upsert(1, "k", SECRET)]), "k")).toBe(SECRET);
});

// VTT-228
test("a note records no visibility when its upsert stated none", () => {
  const n = fold([upsert(1, "k", NONE)]).Notes["k"];
  expect(n).toBeDefined();
  expect(n!.Title).toBe("title k");
  expect(n!.Text).toBe("text of k");
  expect(n!.Visibility).toBe(NONE);
});

// VTT-228
test.each([
  ["public then secret", PUBLIC, SECRET],
  ["secret then public", SECRET, PUBLIC],
  ["none then public", NONE, PUBLIC],
  ["none then secret", NONE, SECRET],
])("a later upsert replaces the visibility with the one it stated: %s", (_name, first, second) => {
  const st = fold([upsert(1, "k", first), upsert(2, "k", second)]);
  expect(visibilityOf(st, "k")).toBe(second);
});

// VTT-228
test.each([
  ["public then none", PUBLIC],
  ["secret then none", SECRET],
])("a later upsert stating none leaves the note with no visibility: %s", (_name, first) => {
  const st = fold([upsert(1, "k", first), upsert(2, "k", NONE)]);
  expect(visibilityOf(st, "k")).toBe(NONE);
});

// VTT-228
test.each([
  ["public, deleted, re-upserted with none", PUBLIC, NONE],
  ["secret, deleted, re-upserted with none", SECRET, NONE],
  ["secret, deleted, re-upserted public", SECRET, PUBLIC],
  ["public, deleted, re-upserted secret", PUBLIC, SECRET],
])("a note re-upserted after deletion carries only the re-upsert's visibility: %s", (_name, before, after) => {
  expect(fold([upsert(1, "k", before), del(2, "k")]).Notes["k"]).toBeUndefined();
  const st = fold([upsert(1, "k", before), del(2, "k"), upsert(3, "k", after)]);
  expect(visibilityOf(st, "k")).toBe(after);
});

// VTT-228
test("a note's visibility is held per note", () => {
  const log = [upsert(1, "a", PUBLIC), upsert(2, "b", SECRET), upsert(3, "a", NONE)];
  let st = fold(log);
  expect(visibilityOf(st, "a")).toBe(NONE);
  expect(visibilityOf(st, "b")).toBe(SECRET);
  st = fold([...log, upsert(4, "b", PUBLIC)]);
  expect(visibilityOf(st, "a")).toBe(NONE);
  expect(visibilityOf(st, "b")).toBe(PUBLIC);
});

// QA_PARITY_LOG is, byte for byte, qaNVParityLog in
// internal/engine/qa_note_visibility_test.go, and PARITY_WANT is its
// qaNVParityWant.
const QA_PARITY_LOG = `[
 {"sequence":"1","noteUpserted":{"key":"a","title":"A","text":"a1","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"2","noteUpserted":{"key":"b","title":"B","text":"b1","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"3","noteUpserted":{"key":"c","title":"C","text":"c1"}},
 {"sequence":"4","noteUpserted":{"key":"a","title":"A","text":"a2"}},
 {"sequence":"5","noteUpserted":{"key":"b","title":"B","text":"b2","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"6","noteUpserted":{"key":"c","title":"C","text":"c2","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"7","noteUpserted":{"key":"d","title":"D","text":"d1","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"8","noteDeleted":{"key":"d"}},
 {"sequence":"9","noteUpserted":{"key":"d","title":"D","text":"d2","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"10","noteUpserted":{"key":"e","title":"E","text":"e1","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"11","noteDeleted":{"key":"e"}},
 {"sequence":"12","noteUpserted":{"key":"e","title":"E","text":"e2"}},
 {"sequence":"13","noteUpserted":{"key":"f","title":"F","text":"f1","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"14","noteUpserted":{"key":"f","title":"F","text":"f2"}},
 {"sequence":"15","noteUpserted":{"key":"g","title":"G","text":"g1","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"16","noteUpserted":{"key":"h","title":"H","text":"h1","visibility":"NOTE_VISIBILITY_SECRET"}}
]`;

const PARITY_WANT: Record<string, NoteVisibility> = {
  a: NONE, b: PUBLIC, c: SECRET, d: PUBLIC,
  e: NONE, f: NONE, g: PUBLIC, h: SECRET,
};

function parityLog(): Envelope[] {
  return (JSON.parse(QA_PARITY_LOG) as unknown[]).map((e) => fromJson(EnvelopeSchema, e as never));
}

// VTT-228
test("the shared parity log folds to the visibilities its latest upserts stated", () => {
  const st = fold(parityLog());
  expect(Object.keys(st.Notes).sort()).toEqual(Object.keys(PARITY_WANT).sort());
  for (const [key, want] of Object.entries(PARITY_WANT)) {
    expect([key, visibilityOf(st, key)]).toEqual([key, want]);
  }
});

// Ticket item 5: "the fold-parity and projection goldens pass with it in the
// dumps". The ticket fixes that a stated visibility is in the dump, not its
// spelling; its spelling is pinned by client/test/fold-dump.test.ts's "a
// note's visibility is dumped when stated and omitted when not".
test("the dump carries a stated visibility, and public and secret dump differently", () => {
  const dump = JSON.parse(foldToDumpJSON([upsert(1, "p", PUBLIC), upsert(2, "s", SECRET)]));
  expect(dump.Notes.p).toHaveProperty("Visibility");
  expect(dump.Notes.s).toHaveProperty("Visibility");
  expect(dump.Notes.p.Visibility).not.toEqual(dump.Notes.s.Visibility);
});

// Ticket item 5, with VTT-228: a visibility replaced in the fold is replaced
// in the dump too, so the dump never shows an earlier upsert's value.
test("the dump shows the latest upsert's visibility, not an earlier one's", () => {
  const onlySecret = JSON.parse(foldToDumpJSON([upsert(1, "k", SECRET)])).Notes.k;
  const publicThenSecret = JSON.parse(
    foldToDumpJSON([upsert(1, "k", PUBLIC), upsert(2, "k", SECRET)]),
  ).Notes.k;
  expect(publicThenSecret).toHaveProperty("Visibility");
  expect(publicThenSecret.Visibility).toEqual(onlySecret.Visibility);
});
