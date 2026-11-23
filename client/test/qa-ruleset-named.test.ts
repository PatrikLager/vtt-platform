import { afterAll, describe as group, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { GlobalRegistrator } from "@happy-dom/global-registrator";
import {
  ActorAddedSchema,
  ActorControlGrantedSchema,
  ActorKind,
  ActorRemovedSchema,
  ActorSchema,
  ConditionAppliedSchema,
  EnvelopeSchema,
  GridPositionSchema,
  NoteUpsertedSchema,
  NoteVisibility,
  ResourceSchema,
  RulesetNamedSchema,
  SceneCreatedSchema,
  SessionStartedSchema,
  TokenPlacedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { fold, foldToDumpJSON } from "../src/fold";
import { FoldError, type State } from "../src/state";
import { buildFeed } from "../src/view/feed";
import { namer } from "../src/view/naming";
import { describe as describeEvent, renderSpectator } from "../src/view/spectator";

const registered = typeof document === "undefined";
if (registered) GlobalRegistrator.register();
afterAll(async () => {
  if (registered) await GlobalRegistrator.unregister();
});

const ID = "qa-rules";
const VERSION = "2.5.0";
const NAME = "Plain Rules";

type Payload = Envelope["payload"];

function env(seq: number, payload: Payload, eventId = `ev-${seq}`): Envelope {
  return create(EnvelopeSchema, {
    sequence: BigInt(seq),
    eventId,
    sessionId: "s-1",
    participantId: "p-dm",
    actorRole: "dm",
    payload,
  });
}

function rn(seq: number, rulesetId: string, version: string, name: string, eventId = `ev-${seq}`): Envelope {
  return env(seq, { case: "rulesetNamed", value: create(RulesetNamedSchema, { rulesetId, version, name }) }, eventId);
}

// The same log as internal/engine/qa_ruleset_named_test.go's qarnPrefix.
function prefix(): Envelope[] {
  return [
    env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "Evening" }) }),
    env(2, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "sc-1", name: "Hall", gridWidth: 6, gridHeight: 5 }) }),
    env(3, {
      case: "actorAdded",
      value: create(ActorAddedSchema, {
        actor: create(ActorSchema, {
          actorId: "a-1",
          name: "Ada",
          kind: ActorKind.PARTY_MEMBER,
          attributes: { k1: 3 },
          resources: { r1: create(ResourceSchema, { current: 2, max: 4 }) },
          abilityIds: ["ab-1"],
        }),
      }),
    }),
    env(4, {
      case: "actorAdded",
      value: create(ActorAddedSchema, { actor: create(ActorSchema, { actorId: "b-1", name: "Bram", kind: ActorKind.NON_PARTY }) }),
    }),
    env(5, {
      case: "actorControlGranted",
      value: create(ActorControlGrantedSchema, { actorId: "a-1", participantId: "p-1", kind: ActorKind.PARTY_MEMBER }),
    }),
    env(6, {
      case: "tokenPlaced",
      value: create(TokenPlacedSchema, { tokenId: "t-1", sceneId: "sc-1", actorId: "a-1", position: create(GridPositionSchema, { x: 1, y: 2 }), name: "Ada" }),
    }),
    env(7, {
      case: "tokenPlaced",
      value: create(TokenPlacedSchema, { tokenId: "t-2", sceneId: "sc-1", actorId: "b-1", position: create(GridPositionSchema, { x: 4, y: 3 }) }),
    }),
    env(8, { case: "conditionApplied", value: create(ConditionAppliedSchema, { actorId: "a-1", conditionId: "cond-1", source: "manual" }) }),
    env(9, {
      case: "noteUpserted",
      value: create(NoteUpsertedSchema, { key: "n-1", title: "Title", text: "Body", visibility: NoteVisibility.PUBLIC }),
    }),
  ];
}

type Field = "id" | "version" | "name";
const FIELDS: Field[] = ["id", "version", "name"];

function put(field: Field, value: string, id = ID, version = VERSION, name = NAME): [string, string, string] {
  if (field === "id") return [value, version, name];
  if (field === "version") return [id, value, name];
  return [id, version, value];
}

// Mirrors the Go cases: ASCII, and multi-byte text whose code point count and
// UTF-16 length both stay under the bound while its bytes reach or pass it.
const AT_128: Record<string, string> = { ascii: "r".repeat(128), "3-byte": "€".repeat(42) + "rr", "4-byte": "😀".repeat(32) };
const VERSION_AT_128: Record<string, string> = { ascii: "v".repeat(128), "3-byte": "vv" + "€".repeat(42), "4-byte": "😃".repeat(32) };
const PAST_128: Record<string, string> = { ascii: "r".repeat(129), "3-byte": "€".repeat(43), "4-byte": "😀".repeat(32) + "r" };
const AT_256: Record<string, string> = { ascii: "n".repeat(256), "3-byte": "€".repeat(85) + "n", "4-byte": "😀".repeat(64) };
const PAST_256: Record<string, string> = { ascii: "n".repeat(257), "3-byte": "€".repeat(86), "4-byte": "😀".repeat(64) + "n" };
const ENCODINGS = ["ascii", "3-byte", "4-byte"] as const;

const at = (f: Field) => (f === "name" ? AT_256 : AT_128);
const past = (f: Field) => (f === "name" ? PAST_256 : PAST_128);

function words(msg: string): Set<string> {
  const out = new Set<string>();
  for (const part of msg.split(/[^A-Za-z0-9]+/)) {
    for (const w of part.match(/[A-Z]?[a-z]+|[A-Z]+(?:[a-z]+)?|[0-9]+/g) ?? []) out.add(w.toLowerCase());
  }
  return out;
}

function refusal(f: () => unknown): Error {
  try {
    f();
  } catch (e) {
    return e as Error;
  }
  throw new Error("no refusal");
}

function expectNamesField(e: Error, field: Field): void {
  const w = words(e.message);
  for (const f of FIELDS) {
    // VTT-974
    expect({ message: e.message, field: f, named: w.has(f) }).toEqual({ message: e.message, field: f, named: f === field });
  }
}

function names(st: State): [string | undefined, string | undefined, string | undefined] {
  return [st.RulesetID, st.RulesetVersion, st.RulesetName];
}

function withoutRuleset(st: State): Omit<State, "RulesetID" | "RulesetVersion" | "RulesetName"> {
  const { RulesetID: _i, RulesetVersion: _v, RulesetName: _n, ...rest } = st;
  return rest;
}

group("qaRN: VTT-974 bounds, both folds alike", () => {
  for (const field of FIELDS) {
    for (const enc of ENCODINGS) {
      test(`qaRN: a ${field} exactly at its bound (${enc}) is folded`, () => {
        const v = at(field)[enc]!;
        const [id, version, name] = put(field, v);
        // VTT-974
        const st = fold([rn(1, id, version, name)]);
        // VTT-979
        expect(names(st)).toEqual([id, version, name]);
      });
    }
  }

  for (const enc of ENCODINGS) {
    test(`qaRN: id, version and name all at their bounds together (${enc}) are folded`, () => {
      const [id, version, name] = [AT_128[enc]!, VERSION_AT_128[enc]!, AT_256[enc]!];
      // VTT-974
      const st = fold([rn(1, id, version, name)]);
      // VTT-979
      expect(names(st)).toEqual([id, version, name]);
    });
  }

  for (const field of FIELDS) {
    const cases: [string, string][] = [["empty", ""], ...ENCODINGS.map((enc): [string, string] => [`past/${enc}`, past(field)[enc]!])];
    for (const [label, v] of cases) {
      test(`qaRN: a ${field} ${label} is refused by a FoldError naming the ${field}, and the dump refuses it too`, () => {
        const [id, version, name] = put(field, v);
        const log = [...prefix(), rn(10, id, version, name)];
        const e = refusal(() => fold(log));
        // VTT-974
        expect(e).toBeInstanceOf(FoldError);
        expectNamesField(e, field);
        // VTT-983
        expect(refusal(() => foldToDumpJSON(log))).toBeInstanceOf(FoldError);
      });
    }
  }

  for (const field of FIELDS) {
    test(`qaRN: a refused ${field} after a ruleset was named is still refused, naming the ${field}`, () => {
      const [id, version, name] = put(field, past(field)["4-byte"]!, "other-rules", "2.5.1", "Plain Rules II");
      const log = [...prefix(), rn(10, ID, VERSION, NAME), rn(11, id, version, name)];
      const e = refusal(() => fold(log));
      // VTT-974
      expect(e).toBeInstanceOf(FoldError);
      expectNamesField(e, field);
    });
  }
});

group("qaRN: VTT-979 the state names the ruleset and nothing else changes", () => {
  for (const [label, pre] of [
    ["empty", [] as Envelope[]],
    ["populated", prefix()],
  ] as const) {
    test(`qaRN: ${label}: a rulesetNamed names its id, version and name and changes nothing else`, () => {
      const before = fold([...pre]);
      const after = fold([...pre, rn(10, ID, VERSION, NAME)]);
      // VTT-979
      expect(names(after)).toEqual([ID, VERSION, NAME]);
      // VTT-979
      expect(withoutRuleset(after)).toEqual(withoutRuleset(before));
    });
  }

  test("qaRN: a later rulesetNamed replaces each field the state names", () => {
    const steps: [string, string, string][] = [
      [ID, VERSION, NAME],
      [ID, "2.6.0", "Plain Rules Revised"],
      ["other-rules", VERSION, NAME],
      ["other-rules", VERSION, "Other Rules"],
    ];
    const log = prefix();
    steps.forEach(([id, version, name], i) => {
      log.push(rn(10 + i, id, version, name));
      // VTT-979
      expect(names(fold(log))).toEqual([id, version, name]);
    });
    // VTT-979
    expect(withoutRuleset(fold(log))).toEqual(withoutRuleset(fold(prefix())));
  });

  test("qaRN: the dump names the ruleset under the keys Go's encoder gives engine.State, and nothing else changes", () => {
    const name = "€".repeat(85) + "n";
    const before = JSON.parse(foldToDumpJSON(prefix())) as Record<string, unknown>;
    const after = JSON.parse(foldToDumpJSON([...prefix(), rn(10, ID, VERSION, name)])) as Record<string, unknown>;
    // VTT-979, VTT-983
    expect([after["RulesetID"], after["RulesetVersion"], after["RulesetName"]]).toEqual([ID, VERSION, name]);
    for (const k of ["RulesetID", "RulesetVersion", "RulesetName", "headSequence"]) {
      delete after[k];
      delete before[k];
    }
    // VTT-979
    expect(after).toEqual(before);
  });
});

group("qaRN: VTT-987 no ruleset in the dump of a log that names none", () => {
  for (const [label, log] of [
    ["empty", [] as Envelope[]],
    ["populated", prefix()],
  ] as const) {
    test(`qaRN: ${label}: foldToDumpJSON writes nothing of a ruleset`, () => {
      // VTT-987
      expect(foldToDumpJSON([...log]).toLowerCase()).not.toContain("ruleset");
    });
  }
});

function section(root: HTMLElement, heading: string): HTMLElement | null {
  for (const s of Array.from(root.querySelectorAll("section"))) {
    if (s.querySelector("h2")?.textContent === heading) return s as HTMLElement;
  }
  return null;
}

const ROLES = [
  { label: "DM or agent", dm: true, ticker: true },
  { label: "player or spectator", dm: false, ticker: false },
  { label: "DM flag without the panel", dm: true, ticker: false },
  { label: "panel without the DM flag", dm: false, ticker: true },
];

function draw(log: Envelope[], dm: boolean, ticker: boolean): HTMLElement {
  const root = document.createElement("div");
  renderSpectator(root, fold(log), log, "connected", { dm, ticker });
  return root;
}

const inFeed = (log: Envelope[]): Envelope[] => buildFeed(log).flatMap((e) => e.events);

group("qaRN: VTT-984 the Story shows every rulesetNamed entry", () => {
  const first = rn(10, ID, VERSION, NAME);
  const second = rn(11, "other-rules", "3.0.1", "Other Rules", "");
  const log = [...prefix(), first, second];

  test("qaRN: buildFeed keeps a rulesetNamed with an event id and one with none", () => {
    const kept = inFeed(log);
    // VTT-984
    expect(kept).toContain(first);
    expect(kept).toContain(second);
  });

  for (const r of ROLES) {
    test(`qaRN: ${r.label}: the Story draws both rulesetNamed lines`, () => {
      const story = section(draw(log, r.dm, r.ticker), "Story");
      // VTT-984
      expect(story).not.toBeNull();
      const text = story!.textContent ?? "";
      for (const s of ["Plain Rules", VERSION, "Other Rules", "3.0.1"]) expect(text).toContain(s);
    });
  }

  function withRun(middle: Envelope[]): Envelope[] {
    const d = env(10, {
      case: "actorAdded",
      value: create(ActorAddedSchema, { actor: create(ActorSchema, { actorId: "d-1", name: "Dag", kind: ActorKind.PARTY_MEMBER }) }),
    });
    return [...prefix(), d, ...middle];
  }
  const bareRemove = () => env(11, { case: "actorRemoved", value: create(ActorRemovedSchema, { actorId: "d-1" }) }, "");
  const bareAdd = (seq: number) =>
    env(seq, {
      case: "actorAdded",
      value: create(ActorAddedSchema, { actor: create(ActorSchema, { actorId: "d-1", name: "Dag", kind: ActorKind.PARTY_MEMBER }) }),
    }, "");
  const bareCondition = (seq: number) =>
    env(seq, { case: "conditionApplied", value: create(ConditionAppliedSchema, { actorId: "d-1", conditionId: "cond-2" }) }, "");

  test("qaRN: a bare rulesetNamed behind a run ends it: it and the bare condition behind it are shown, the run is not", () => {
    const remove = bareRemove();
    const add = bareAdd(12);
    const named = rn(13, ID, VERSION, NAME, "");
    const cond = bareCondition(14);
    const kept = inFeed(withRun([remove, add, named, cond]));
    // VTT-984
    expect(kept).toContain(named);
    expect(kept).toContain(cond);
    expect(kept).not.toContain(remove);
    expect(kept).not.toContain(add);
  });

  test("qaRN: a bare rulesetNamed between a bare removal and the add is shown, and so are both", () => {
    const remove = bareRemove();
    const named = rn(12, ID, VERSION, NAME, "");
    const add = bareAdd(13);
    const kept = inFeed(withRun([remove, named, add]));
    // VTT-984
    expect(kept).toContain(remove);
    expect(kept).toContain(named);
    expect(kept).toContain(add);
  });
});

group("qaRN: VTT-985 a rulesetNamed is worded by the ruleset's name and its version", () => {
  const cases = [
    { label: "ASCII", id: ID, version: VERSION, name: NAME },
    { label: "multi-byte", id: "qa-règles", version: "2.5.0-β", name: "Règles Simples ✦" },
  ];
  for (const c of cases) {
    for (const eventId of ["ev-rn", ""]) {
      const tag = `${c.label}, ${eventId === "" ? "no event id" : "an event id"}`;
      const named = rn(10, c.id, c.version, c.name, eventId);
      const log = [...prefix(), named];

      for (const dm of [true, false]) {
        test(`qaRN: ${tag}: describe words it by name and version (dm ${dm})`, () => {
          const line = describeEvent(named, namer(fold(log), log, dm, []));
          // VTT-985
          expect(line).toContain(c.name);
          expect(line).toContain(c.version);
        });
      }

      test(`qaRN: ${tag}: the Story's line and the Events panel's row name the ruleset and its version`, () => {
        const root = draw(log, true, true);
        const story = section(root, "Story");
        const events = section(root, "Events");
        // VTT-985
        expect(story).not.toBeNull();
        expect(events).not.toBeNull();
        const storyLine = Array.from(story!.querySelectorAll("p")).find((p) => p.textContent?.includes(c.name));
        const eventRow = Array.from(events!.querySelectorAll("p")).find((p) => p.textContent?.includes(c.name));
        expect(storyLine?.textContent).toContain(c.version);
        expect(eventRow?.textContent).toContain(c.version);
      });
    }
  }
});
