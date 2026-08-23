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
    controllerId: "p-player", controllerIds: ["p-player"],
  };
  st.Actors["a2"] = {
    actorId: "a2", name: "", moduleId: "", attributes: {}, resources: {}, controllerId: "", controllerIds: [],
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
  const discs = tokensOnScene(worldWith(), "s1", undefined);
  expect(discs.map((d) => d.tokenId)).toEqual(["t1"]);
});

test("a disc is labelled with the actor's initial", () => {
  expect(tokensOnScene(worldWith(), "s1", undefined)[0]!.initial).toBe("L");
});

test("an actor with no name still gets a stable label rather than a blank disc", () => {
  const discs = tokensOnScene(worldWith(), "s2", undefined);
  expect(discs[0]!.initial).not.toBe("");
});

test("EVERY resource is shown, in a stable order, with current and max", () => {
  // Spec §4: ALL resources as current/max chips. Picking a "primary" one
  // would need ruleset client-hints the format does not have (§9), and
  // guessing which matters is exactly the genre assumption the platform
  // refuses to make.
  const chips = tokensOnScene(worldWith(), "s1", undefined)[0]!.resources;
  expect(chips.map((c) => c.name)).toEqual(["focus", "vigor"]); // sorted, not map order
  expect(chips.find((c) => c.name === "vigor")).toEqual({ name: "vigor", current: 3, max: 10 });
});

test("EVERY condition is shown, in applied order", () => {
  // Applied order is meaningful — it is the order the table saw them happen —
  // and the fold preserves it deliberately, so the view must not re-sort.
  const dots = tokensOnScene(worldWith(), "s1", undefined)[0]!.conditions;
  expect(dots.map((c) => c.id)).toEqual(["dazed", "marked"]);
});

test("a token whose actor vanished does not crash the board", () => {
  // Defensive, but the alternative is one malformed log blanking the whole
  // grid instead of one disc.
  const st = worldWith();
  delete st.Actors["a1"];
  const discs = tokensOnScene(st, "s1", undefined);
  expect(discs).toHaveLength(1);
  expect(discs[0]!.resources).toEqual([]);
});

// --- draw order and the disc fallbacks --------------------------------------

test("discs come back in tokenId order regardless of insertion order", () => {
  // The board re-renders on every event. Without a deterministic order the
  // DOM reshuffles under the player's cursor between two renders of an
  // unchanged board — and Object.values follows INSERTION order, which is
  // whatever sequence the events happened to arrive in.
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 10, GridHeight: 8 };
  for (const id of ["t3", "t1", "t4", "t2"]) {
    st.Actors["a" + id] = {
      actorId: "a" + id, name: id.toUpperCase(), moduleId: "",
      attributes: {}, resources: {}, controllerId: "", controllerIds: [],
    };
    st.Tokens[id] = { ID: id, SceneID: "s1", ActorID: "a" + id, X: 0, Y: 0 };
  }
  expect(tokensOnScene(st, "s1", undefined).map((d) => d.tokenId)).toEqual(["t1", "t2", "t3", "t4"]);
});

test("the order is ASCENDING, and it is the comparator producing it", () => {
  // Two tokens is the smallest case that distinguishes ascending from
  // descending, from "left as inserted", and from a comparator that returns a
  // constant. Inserted high-then-low so every one of those yields a different
  // answer here.
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 4, GridHeight: 4 };
  st.Actors["a"] = { actorId: "a", name: "A", moduleId: "", attributes: {}, resources: {}, controllerId: "", controllerIds: []};
  st.Tokens["zz"] = { ID: "zz", SceneID: "s1", ActorID: "a", X: 0, Y: 0 };
  st.Tokens["aa"] = { ID: "aa", SceneID: "s1", ActorID: "a", X: 1, Y: 1 };
  const ids = tokensOnScene(st, "s1", undefined).map((d) => d.tokenId);
  expect(ids).toEqual(["aa", "zz"]);
  expect(ids[0]! < ids[1]!).toBe(true);
});

test("a token whose actor is gone still draws, with an empty name", () => {
  // Tokens and actors are separate maps and a token can outlive its actor
  // mid-replay. Drawing nothing would make the board disagree with the
  // server's state; the disc renders with no name and falls back to the actor
  // id for its initial.
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 4, GridHeight: 4 };
  st.Tokens["t9"] = { ID: "t9", SceneID: "s1", ActorID: "ghost", X: 1, Y: 1 };
  const [d] = tokensOnScene(st, "s1", undefined);
  expect(d!.name).toBe("");
  expect(d!.initial).toBe("G");
  expect(d!.resources).toEqual([]);
  expect(d!.conditions).toEqual([]);
});

test("a long reversed board still sorts, exercising the merge paths", () => {
  // A broad sort check over twenty descending elements: every id lands in
  // ascending order and the ends are where they belong.
  //
  // It does NOT catch a comparator that is right about "less" and wrong about
  // "greater", which an earlier version of this comment claimed. Measured:
  // under the `b.tokenId > a.tokenId ? 1 : 0` -> `: false ? 1 : 0` mutant this
  // file stays 13/13 green, and against a constant-0 or reversed comparator it
  // fails only alongside the two-element test above — a strict subset. That is
  // the same twenty-element experiment recorded in ts-mutation-equivalents.txt
  // for grid.ts 108:61, and the adjudication there is the correct reading:
  // equal keys are impossible, so the arm is unobservable rather than untested.
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 40, GridHeight: 40 };
  st.Actors["a"] = { actorId: "a", name: "A", moduleId: "", attributes: {}, resources: {}, controllerId: "", controllerIds: []};
  const ids = Array.from({ length: 20 }, (_, i) => `t${String(20 - i).padStart(2, "0")}`);
  for (const id of ids) st.Tokens[id] = { ID: id, SceneID: "s1", ActorID: "a", X: 0, Y: 0 };
  const got = tokensOnScene(st, "s1", undefined).map((d) => d.tokenId);
  expect(got).toEqual([...ids].sort());
  expect(got[0]).toBe("t01");
  expect(got[19]).toBe("t20");
});
