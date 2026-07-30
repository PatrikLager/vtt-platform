import { test, expect } from "bun:test";
import { cellFromPoint, tokensOnScene, type Geometry } from "../src/view/grid";
import { newState, type State } from "../src/state";

const geom: Geometry = { cell: 40, width: 10, height: 8 };

test("a click maps to the cell under it", () => {
  expect(cellFromPoint(0, 0, geom)).toEqual({ x: 0, y: 0 });
  expect(cellFromPoint(41, 81, geom)).toEqual({ x: 1, y: 2 });
});

test("a click inside a cell maps to that cell, not the nearest edge", () => {
  // Rounding instead of flooring would make the right half of every cell
  // select its neighbour — a player aiming at a token would move past it.
  expect(cellFromPoint(39, 39, geom)).toEqual({ x: 0, y: 0 });
  expect(cellFromPoint(40, 40, geom)).toEqual({ x: 1, y: 1 });
});

test("clicks outside the board are clamped, never negative or past the edge", () => {
  // A negative cell would be sent to the server as a legal-looking move and
  // rejected with a confusing error; clamping keeps the click on the board.
  expect(cellFromPoint(-5, -5, geom)).toEqual({ x: 0, y: 0 });
  expect(cellFromPoint(9999, 9999, geom)).toEqual({ x: 9, y: 7 });
});

function worldWith(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 10, GridHeight: 8 };
  st.Scenes["s2"] = { ID: "s2", Name: "Cellar", GridWidth: 5, GridHeight: 5 };
  st.Actors["a1"] = {
    actorId: "a1",
    name: "Lera",
    moduleId: "",
    attributes: {},
    resources: { vigor: { current: 3, max: 10 }, focus: { current: 7, max: 7 } },
    controllerId: "p-player",
  };
  st.Actors["a2"] = {
    actorId: "a2", name: "", moduleId: "", attributes: {}, resources: {}, controllerId: "",
  };
  st.Tokens["t1"] = { ID: "t1", SceneID: "s1", ActorID: "a1", X: 2, Y: 3 };
  st.Tokens["t2"] = { ID: "t2", SceneID: "s2", ActorID: "a2", X: 0, Y: 0 };
  st.Conditions["a1"] = [
    { ID: "dazed", Source: "dm", AppliedSeq: 5 },
    { ID: "marked", Source: "dm", AppliedSeq: 6 },
  ];
  return st;
}

test("only tokens on the requested scene are returned", () => {
  const discs = tokensOnScene(worldWith(), "s1");
  expect(discs.map((d) => d.tokenId)).toEqual(["t1"]);
});

test("a disc is labelled with the actor's initial", () => {
  expect(tokensOnScene(worldWith(), "s1")[0]!.initial).toBe("L");
});

test("an actor with no name still gets a stable label rather than a blank disc", () => {
  const discs = tokensOnScene(worldWith(), "s2");
  expect(discs[0]!.initial).not.toBe("");
});

test("EVERY resource is shown, in a stable order, with current and max", () => {
  // Spec §4: ALL resources as current/max chips. Picking a "primary" one
  // would need ruleset client-hints the format does not have (§9), and
  // guessing which matters is exactly the genre assumption the platform
  // refuses to make.
  const chips = tokensOnScene(worldWith(), "s1")[0]!.resources;
  expect(chips.map((c) => c.name)).toEqual(["focus", "vigor"]); // sorted, not map order
  expect(chips.find((c) => c.name === "vigor")).toEqual({ name: "vigor", current: 3, max: 10 });
});

test("EVERY condition is shown, in applied order", () => {
  // Applied order is meaningful — it is the order the table saw them happen —
  // and the fold preserves it deliberately, so the view must not re-sort.
  const dots = tokensOnScene(worldWith(), "s1")[0]!.conditions;
  expect(dots.map((c) => c.id)).toEqual(["dazed", "marked"]);
});

test("a token whose actor vanished does not crash the board", () => {
  // Defensive, but the alternative is one malformed log blanking the whole
  // grid instead of one disc.
  const st = worldWith();
  delete st.Actors["a1"];
  const discs = tokensOnScene(st, "s1");
  expect(discs).toHaveLength(1);
  expect(discs[0]!.resources).toEqual([]);
});
