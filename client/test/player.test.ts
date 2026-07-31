import { test, expect } from "bun:test";
import { newState, type State } from "../src/state";
import { controlledActors, affordable, withinRange, targetableTokens } from "../src/player";
import type { Ability } from "../src/metadata";

function world(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 10, GridHeight: 10 };
  const actor = (id: string, controller: string, resources = {}) => {
    st.Actors[id] = {
      actorId: id, name: id.toUpperCase(), moduleId: "",
      attributes: {}, resources, controllerId: controller,
    };
  };
  actor("mine", "p-me", { vigor: { current: 2, max: 10 } });
  actor("also-mine", "p-me");
  actor("theirs", "p-them");
  actor("npc", "");
  st.Tokens["t-mine"] = { ID: "t-mine", SceneID: "s1", ActorID: "mine", X: 1, Y: 1 };
  st.Tokens["t-theirs"] = { ID: "t-theirs", SceneID: "s1", ActorID: "theirs", X: 3, Y: 1 };
  st.Tokens["t-far"] = { ID: "t-far", SceneID: "s1", ActorID: "npc", X: 9, Y: 9 };
  st.Tokens["t-elsewhere"] = { ID: "t-elsewhere", SceneID: "s2", ActorID: "npc", X: 1, Y: 1 };
  return st;
}

test("you control exactly the actors whose controllerId is yours", () => {
  const mine = controlledActors(world(), "p-me").map((a) => a.actorId);
  expect(mine.sort()).toEqual(["also-mine", "mine"]);
});

test("an uncontrolled NPC belongs to nobody, including the empty participant", () => {
  // An empty controllerId means DM/agent-run. If the empty string matched an
  // absent participant id, an unauthenticated view would appear to control
  // every NPC on the board.
  expect(controlledActors(world(), "").map((a) => a.actorId)).toEqual([]);
});

test("an at-will ability is always affordable", () => {
  const ab: Ability = { id: "a", name: "A", range: 1, maxTargets: 1, usage: { kind: "atWill" } };
  expect(affordable(ab, world().Actors["mine"]!)).toBe(true);
});

test("a resource ability is affordable only when the actor can pay", () => {
  const ab: Ability = {
    id: "a", name: "A", range: 1, maxTargets: 1,
    usage: { kind: "resource", resource: "vigor", cost: 2 },
  };
  expect(affordable(ab, world().Actors["mine"]!)).toBe(true); // has exactly 2

  const dear = { ...ab, usage: { ...ab.usage, cost: 3 } } as Ability;
  expect(affordable(dear, world().Actors["mine"]!)).toBe(false);
});

test("an ability costing a resource the actor does not have is not affordable", () => {
  // Rather than treating a missing resource as zero-cost and letting the
  // player fire something the server will reject.
  const ab: Ability = {
    id: "a", name: "A", range: 1, maxTargets: 1,
    usage: { kind: "resource", resource: "mana", cost: 1 },
  };
  expect(affordable(ab, world().Actors["mine"]!)).toBe(false);
});

test("range is CHEBYSHEV — diagonals cost the same as orthogonals", () => {
  // Targeting.Range is documented as Chebyshev in the ruleset format. Using
  // Euclidean or Manhattan here would grey out legal targets, or offer
  // illegal ones the server then refuses.
  expect(withinRange({ x: 0, y: 0 }, { x: 1, y: 1 }, 1)).toBe(true);
  expect(withinRange({ x: 0, y: 0 }, { x: 2, y: 2 }, 2)).toBe(true);
  expect(withinRange({ x: 0, y: 0 }, { x: 3, y: 0 }, 2)).toBe(false);
});

test("a target on the same square is in range of anything", () => {
  expect(withinRange({ x: 4, y: 4 }, { x: 4, y: 4 }, 0)).toBe(true);
});

test("targetable tokens are on the same scene and within the ability's range", () => {
  const st = world();
  const ab: Ability = { id: "a", name: "A", range: 2, maxTargets: 1, usage: { kind: "atWill" } };
  const ids = targetableTokens(st, "t-mine", ab).map((t) => t.ID);
  // t-theirs is 2 away, t-far is 8 away, t-elsewhere is on another scene.
  // The ACTOR'S OWN token is included, at distance 0: the ruleset format has
  // no target-kind (T3's ledgered limitation), so the client cannot tell a
  // heal from a strike and must not pre-filter self or allies. Every actor in
  // reach is offered and the server rejects illegal uses — which is the
  // documented posture, not an oversight here.
  expect(ids).toEqual(["t-mine", "t-theirs"]);
});

test("the acting token can target itself when the ability reaches its own square", () => {
  const st = world();
  const ab: Ability = { id: "a", name: "A", range: 0, maxTargets: 1, usage: { kind: "atWill" } };
  expect(targetableTokens(st, "t-mine", ab).map((t) => t.ID)).toEqual(["t-mine"]);
});

test("targeting from a token that does not exist yields nothing rather than throwing", () => {
  const ab: Ability = { id: "a", name: "A", range: 5, maxTargets: 1, usage: { kind: "atWill" } };
  expect(targetableTokens(world(), "t-ghost", ab)).toEqual([]);
});

// --- board-click behaviour --------------------------------------------------

import { moveCommandFor, tokenForActor } from "../src/view/player";
import type { Me } from "../src/metadata";

const me: Me = { participantId: "p-me", name: "Me", role: "player", controls: ["mine"] };

test("tokenForActor finds the actor's token", () => {
  expect(tokenForActor(world(), "mine")).toBe("t-mine");
  expect(tokenForActor(world(), "nobody")).toBe("");
});

test("a board click moves the selected actor's token", () => {
  const cmd = moveCommandFor(world(), me, { selectedActorId: "mine", selectedAbilityId: "" }, { x: 4, y: 5 });
  expect(cmd).not.toBeNull();
  expect(cmd!.command.case).toBe("moveToken");
  expect((cmd!.command.value as any).tokenId).toBe("t-mine");
  expect((cmd!.command.value as any).to).toMatchObject({ x: 4, y: 5 });
});

test("a board click does NOT move while an ability is armed", () => {
  // Otherwise arming an ability and then clicking the board to aim would walk
  // your token onto the target instead of attacking it.
  const cmd = moveCommandFor(world(), me, { selectedActorId: "mine", selectedAbilityId: "strike" }, { x: 4, y: 5 });
  expect(cmd).toBeNull();
});

test("a participant controlling nothing cannot move anything by clicking", () => {
  const spectator: Me = { participantId: "p-nobody", name: "W", role: "spectator", controls: [] };
  expect(moveCommandFor(world(), spectator, { selectedActorId: "", selectedAbilityId: "" }, { x: 1, y: 1 })).toBeNull();
});

test("an actor with no token on the board cannot be moved", () => {
  const cmd = moveCommandFor(world(), me, { selectedActorId: "also-mine", selectedAbilityId: "" }, { x: 1, y: 1 });
  expect(cmd).toBeNull();
});

test("with no explicit selection the first controlled actor is used", () => {
  // "also-mine" sorts first and has no token, so this must be null rather
  // than silently moving a different actor than the player expects.
  const cmd = moveCommandFor(world(), me, { selectedActorId: "", selectedAbilityId: "" }, { x: 1, y: 1 });
  expect(cmd).toBeNull();
});

// --- who each role may act as ----------------------------------------------

import { actableActors } from "../src/player";

test("a player may act only as actors they control", () => {
  const ids = actableActors(world(), { ...me, role: "player" }).map((a) => a.actorId);
  expect(ids).toEqual(["also-mine", "mine"]);
});

test("a DM may act as ANY actor, including NPCs nobody controls", () => {
  // Spec §4: "act as ANY actor". A DM voicing a goblin is the normal case,
  // and filtering to controlled actors would leave them able to run nothing.
  const ids = actableActors(world(), { ...me, role: "dm" }).map((a) => a.actorId);
  expect(ids).toEqual(["also-mine", "mine", "npc", "theirs"]);
});

test("an agent may act as any actor too — it runs the table alongside the DM", () => {
  expect(actableActors(world(), { ...me, role: "agent" })).toHaveLength(4);
});

test("a spectator may act as nobody", () => {
  expect(actableActors(world(), { ...me, role: "spectator" })).toEqual([]);
});
