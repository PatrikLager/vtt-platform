import "./support/dom";
import { expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import { newState, type State } from "../src/state";
import { renderSpectator, type ViewExtras } from "../src/view/spectator";

type Def = { id: string; name: string; description: string };

function world(ids: string[]): State {
  const st = newState();
  st.Scenes["s"] = { ID: "s", Name: "S", GridWidth: 4, GridHeight: 4 };
  st.Actors["a"] = {
    actorId: "a", name: "Ann", moduleId: "", attributes: {}, resources: {},
    controllerId: "", controllerIds: [], kind: 1, abilityIds: [], references: {},
  };
  st.Tokens["t"] = { ID: "t", SceneID: "s", ActorID: "a", X: 1, Y: 1 };
  st.Conditions["a"] = ids.map((id, i) => ({ ID: id, Source: "", AppliedSeq: i + 1 }));
  return st;
}

function applied(seq: number, id: string): Envelope {
  return create(EnvelopeSchema, {
    eventId: `e${seq}`, sequence: BigInt(seq),
    payload: { case: "conditionApplied", value: { actorId: "a", conditionId: id, source: "" } } as never,
  });
}

function removed(seq: number, id: string): Envelope {
  return create(EnvelopeSchema, {
    eventId: `e${seq}`, sequence: BigInt(seq),
    payload: { case: "conditionRemoved", value: { actorId: "a", conditionId: id, reason: "" } } as never,
  });
}

function draw(ids: string[], conditions: Def[] | undefined, extras: ViewExtras = {}, log: Envelope[] = []): HTMLElement {
  const root = document.createElement("div");
  const ex: ViewExtras = { ticker: true, ...extras };
  if (conditions !== undefined) ex.conditions = conditions;
  renderSpectator(root, world(ids), log, "ok", ex);
  return root;
}

function titles(root: HTMLElement): (string | null)[] {
  return Array.from(root.querySelectorAll(".dot")).map((d) => d.getAttribute("title"));
}

function story(root: HTMLElement): string[] {
  return Array.from(root.querySelectorAll(".feed p")).map((p) => p.textContent ?? "");
}

function events(root: HTMLElement): string[] {
  return Array.from(root.querySelectorAll(".tick")).map((p) => p.textContent ?? "");
}

const PRONE: Def = { id: "prone", name: "Prone", description: "Lying on the ground" };

// VTT-862
test("a dot is titled by the name, a line break and the description", () => {
  expect(titles(draw(["prone"], [PRONE]))).toEqual(["Prone\nLying on the ground"]);
});

// VTT-862
test("a dot with an empty description is titled by the name alone", () => {
  expect(titles(draw(["prone"], [{ ...PRONE, description: "" }]))).toEqual(["Prone"]);
});

// VTT-862
test("a dot with an empty name is titled by the id, a line break and the description", () => {
  expect(titles(draw(["prone"], [{ ...PRONE, name: "" }]))).toEqual(["prone\nLying on the ground"]);
});

// VTT-862
test("a dot with an empty name and an empty description is titled by the id alone", () => {
  expect(titles(draw(["prone"], [{ id: "prone", name: "", description: "" }]))).toEqual(["prone"]);
});

// VTT-862
test("a dot for an undeclared condition is titled by its id alone", () => {
  expect(titles(draw(["blind"], [PRONE]))).toEqual(["blind"]);
});

// VTT-862
test("a dot is titled by its id alone while the window holds no list", () => {
  expect(titles(draw(["prone"], undefined))).toEqual(["prone"]);
});

// VTT-862
test("a dot is titled by its id alone while the window holds an empty list", () => {
  expect(titles(draw(["prone"], []))).toEqual(["prone"]);
});

// VTT-862
test("a description's own line breaks and unusual characters stand as written", () => {
  const description = "line one\nline two\n\n  <b>&amp;</b> \"q\" 'a' åäö \u{1F40D} \t end ";
  expect(titles(draw(["prone"], [{ ...PRONE, description }]))).toEqual([`Prone\n${description}`]);
});

// VTT-862
test("a condition is found by its id, not by its place in the list", () => {
  const list: Def[] = [
    { id: "blind", name: "Blind", description: "Cannot see" },
    { id: "deaf", name: "Deaf", description: "" },
    PRONE,
  ];
  expect(titles(draw(["prone"], list))).toEqual(["Prone\nLying on the ground"]);
});

// VTT-862 VTT-835
test("several dots on one disc are each titled by their own entry, in applied order", () => {
  const list: Def[] = [
    PRONE,
    { id: "blind", name: "Blind", description: "Cannot see" },
    { id: "deaf", name: "", description: "" },
  ];
  const root = draw(["blind", "prone", "deaf", "mute"], list);
  expect(titles(root)).toEqual(["Blind\nCannot see", "Prone\nLying on the ground", "deaf", "mute"]);
  expect(root.querySelectorAll(".dot").length).toBe(4);
});

// VTT-862
test("every role titles the dot the same", () => {
  const list = [PRONE];
  const want = ["Prone\nLying on the ground"];
  expect(titles(draw(["prone"], list, { dm: true }))).toEqual(want);
  expect(titles(draw(["prone"], list, { dm: false }))).toEqual(want);
  expect(titles(draw(["prone"], list, { dm: false, perch: undefined, ticker: false }))).toEqual(want);
});

// VTT-835
test("the dot count follows the conditions the actor holds, with or without a list", () => {
  expect(titles(draw([], [PRONE]))).toEqual([]);
  expect(titles(draw(["prone", "prone2", "x"], undefined)).length).toBe(3);
  expect(titles(draw(["prone", "prone2", "x"], [PRONE])).length).toBe(3);
});

// VTT-863
test("the Story names a condition by its name in a player's window", () => {
  const root = draw([], [PRONE], { dm: false, ticker: false }, [applied(1, "prone"), removed(2, "prone")]);
  expect(story(root)).toContain("Ann gained Prone");
  expect(story(root)).toContain("Ann lost Prone");
});

// VTT-863
test("the Events panel names a condition by its name for a DM", () => {
  const root = draw([], [PRONE], { dm: true }, [applied(1, "prone"), removed(2, "prone")]);
  expect(events(root).sort()).toEqual(["Ann gained Prone", "Ann lost Prone"]);
});

// VTT-863
test("an empty name reads as the id in both places", () => {
  const root = draw([], [{ ...PRONE, name: "" }], { dm: true }, [applied(1, "prone"), removed(2, "prone")]);
  expect(story(root)).toContain("Ann gained prone");
  expect(story(root)).toContain("Ann lost prone");
  expect(events(root).sort()).toEqual(["Ann gained prone", "Ann lost prone"]);
});

// VTT-863
test("an undeclared condition reads as its id in both places", () => {
  const root = draw([], [PRONE], { dm: true }, [applied(1, "blind"), removed(2, "blind")]);
  expect(story(root)).toContain("Ann gained blind");
  expect(story(root)).toContain("Ann lost blind");
  expect(events(root).sort()).toEqual(["Ann gained blind", "Ann lost blind"]);
});

// VTT-863
test("with no list the lines read the id", () => {
  const root = draw([], undefined, { dm: true }, [applied(1, "prone"), removed(2, "prone")]);
  expect(story(root)).toContain("Ann gained prone");
  expect(story(root)).toContain("Ann lost prone");
  expect(events(root).sort()).toEqual(["Ann gained prone", "Ann lost prone"]);
});

// VTT-863
test("a description never reaches the lines", () => {
  const root = draw([], [PRONE], { dm: true }, [applied(1, "prone")]);
  expect(story(root).join("|")).not.toContain("Lying");
  expect(events(root).join("|")).not.toContain("Lying");
});

// VTT-863 VTT-475
test("the actor in a line is named as the window names it", () => {
  const st = world([]);
  st.Actors["a"] = { ...st.Actors["a"]!, name: "Ann" };
  st.Tokens["t"] = { ...st.Tokens["t"]!, Name: "Scout", GMName: "Boss" };
  const player = document.createElement("div");
  renderSpectator(player, st, [applied(1, "prone")], "ok", { dm: false, conditions: [PRONE] });
  expect(story(player)).toContain("Scout gained Prone");
  const dm = document.createElement("div");
  renderSpectator(dm, st, [applied(1, "prone")], "ok", { dm: true, ticker: true, conditions: [PRONE] });
  expect(story(dm)).toContain("Boss gained Prone");
  expect(events(dm)).toEqual(["Boss gained Prone"]);
});

// VTT-863
test("a player's window shows no Events panel and still names the condition in the Story", () => {
  const root = draw([], [PRONE], { dm: false, ticker: false }, [applied(1, "prone")]);
  expect(root.querySelectorAll(".tick").length).toBe(0);
  expect(story(root)).toContain("Ann gained Prone");
});
