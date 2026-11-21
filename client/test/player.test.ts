import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect } from "bun:test";
import { newState, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { controlledActors, affordable, withinRange, targetableTokens, readLine } from "../src/player";
import type { Ability } from "../src/metadata";

function world(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 10, GridHeight: 10 };
  const actor = (id: string, controller: string, resources = {}) => {
    st.Actors[id] = {
      actorId: id, name: id.toUpperCase(), moduleId: "",
      // An UNOWNED actor gets an EMPTY set, never [""]. A set holding only the
      // empty string is the state both folds filter out at ActorAdded and
      // controlTarget refuses to create — non-empty, yet mirroring "". Built
      // here it would quietly defeat T3, where "your actors" becomes a set
      // membership test and an unowned NPC would match a participant id of "".
      attributes: {}, resources, controllerId: controller,
      controllerIds: controller === "" ? [] : [controller], kind: ActorKind.UNSPECIFIED, abilityIds: [],
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
  // toHaveLength as well as toEqual: `expect([undefined]).toEqual([])` PASSES
  // in bun, so a mapped projection cannot distinguish "empty" from "one
  // element that mapped to undefined" — measured, and it let a mutant
  // returning a one-element array survive this assertion.
  const got = controlledActors(world(), "");
  expect(got).toHaveLength(0);
  expect(got.map((a) => a.actorId)).toEqual([]);
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

import { moveCommandFor, playerClickFor, tokenForActor } from "../src/view/player";
import type { Me } from "../src/metadata";

const me: Me = { participantId: "p-me", name: "Me", role: "player" };

test("tokenForActor finds the actor's token", () => {
  expect(tokenForActor(world(), "mine")).toBe("t-mine");
  expect(tokenForActor(world(), "nobody")).toBe("");
});

test("a board click moves the selected actor's token", () => {
  const cmd = moveCommandFor(world(), me, { selectedActorId: "mine", selectedAbilityId: "", doorsArmed: false }, { x: 4, y: 5 });
  expect(cmd).not.toBeNull();
  expect(cmd!.command.case).toBe("moveToken");
  expect((cmd!.command.value as any).tokenId).toBe("t-mine");
  expect((cmd!.command.value as any).to).toMatchObject({ x: 4, y: 5 });
});

test("a board click does NOT move while an ability is armed", () => {
  // Otherwise arming an ability and then clicking the board to aim would walk
  // your token onto the target instead of attacking it.
  const st = world();
  st.Actors["mine"]!.abilityIds = ["strike"];
  const cmd = moveCommandFor(st, me, { selectedActorId: "mine", selectedAbilityId: "strike", doorsArmed: false }, { x: 4, y: 5 });
  expect(cmd).toBeNull();
});

test("a participant controlling nothing cannot move anything by clicking", () => {
  const spectator: Me = { participantId: "p-nobody", name: "W", role: "spectator" };
  expect(moveCommandFor(world(), spectator, { selectedActorId: "", selectedAbilityId: "", doorsArmed: false }, { x: 1, y: 1 })).toBeNull();
});

test("a player who controls no actor moves nothing, though the window still has one selected", () => {
  const stranger: Me = { participantId: "p-stranger", name: "S", role: "player" };
  expect(moveCommandFor(world(), stranger, { selectedActorId: "mine", selectedAbilityId: "", doorsArmed: false }, { x: 4, y: 5 })).toBeNull();
});

test("an actor with no token on the board cannot be moved", () => {
  const cmd = moveCommandFor(world(), me, { selectedActorId: "also-mine", selectedAbilityId: "", doorsArmed: false }, { x: 1, y: 1 });
  expect(cmd).toBeNull();
});

test("with no explicit selection the first controlled actor is used", () => {
  // "also-mine" sorts first and has no token, so this must be null rather
  // than silently moving a different actor than the player expects.
  const cmd = moveCommandFor(world(), me, { selectedActorId: "", selectedAbilityId: "", doorsArmed: false }, { x: 1, y: 1 });
  expect(cmd).toBeNull();
});

// --- a player's board click, between the party members they may watch -----

/** world() with also-mine and an unlisted zed both standing on (3,3). */
function crowded(): State {
  const st = world();
  st.Tokens["t-also"] = { ID: "t-also", SceneID: "s1", ActorID: "also-mine", X: 3, Y: 3 };
  st.Tokens["t-zed"] = { ID: "t-zed", SceneID: "s1", ActorID: "zed", X: 3, Y: 3 };
  return st;
}

const watching = (actorId: string) => ({ selectedActorId: actorId, selectedAbilityId: "", doorsArmed: false });

// VTT-940
test("a click on a listed party member's token switches to it", () => {
  const cmd = playerClickFor(crowded(), me, watching("mine"), ["mine", "also-mine"], { x: 3, y: 3 });
  expect(cmd?.command.case).toBe("setViewpoint");
  expect((cmd!.command.value as any).actorId).toBe("also-mine");
});

// VTT-940
test("two listed party members on one cell switch to the lowest id", () => {
  const cmd = playerClickFor(crowded(), me, watching("mine"), ["zed", "mine", "also-mine"], { x: 3, y: 3 });
  expect((cmd!.command.value as any).actorId).toBe("also-mine");
});

test("a click on the watched character's token sends nothing", () => {
  expect(playerClickFor(world(), me, watching("mine"), ["mine", "also-mine"], { x: 1, y: 1 })).toBeNull();
});

// VTT-408, VTT-939
test("a click on a square with no listed party member moves the watched character", () => {
  const cmd = playerClickFor(world(), me, watching("mine"), ["mine"], { x: 4, y: 5 });
  expect(cmd!.command.case).toBe("moveToken");
  expect((cmd!.command.value as any).tokenId).toBe("t-mine");
});

// VTT-940, VTT-408, VTT-939
test("a token the set does not name moves the watched character onto its square", () => {
  const cmd = playerClickFor(crowded(), me, watching("mine"), ["mine"], { x: 3, y: 3 });
  expect(cmd!.command.case).toBe("moveToken");
  expect((cmd!.command.value as any).tokenId).toBe("t-mine");
  expect((cmd!.command.value as any).to).toMatchObject({ x: 3, y: 3 });
});

// VTT-940
test("a listed party member on the same row or column, not the same square, does not switch", () => {
  const st = world();
  st.Tokens["t-also"] = { ID: "t-also", SceneID: "s1", ActorID: "also-mine", X: 5, Y: 5 };
  for (const cell of [{ x: 5, y: 2 }, { x: 2, y: 5 }]) {
    const cmd = playerClickFor(st, me, watching("mine"), ["mine", "also-mine"], cell);
    expect(cmd!.command.case).toBe("moveToken");
  }
});

// VTT-940
test("a listed party member's token in another scene does not switch", () => {
  const st = world();
  st.Tokens["t-also"] = { ID: "t-also", SceneID: "s2", ActorID: "also-mine", X: 4, Y: 4 };
  const cmd = playerClickFor(st, me, watching("mine"), ["mine", "also-mine"], { x: 4, y: 4 });
  expect(cmd!.command.case).toBe("moveToken");
});

// --- a player's click off the squares they see, and onto ones they cannot enter

/**
 * world() as a player's window holds it: columns and rows 1 to 5 seen now,
 * (0,0) remembered, the rest never seen; a wall, an open door and two never
 * opened, scenery blocking movement two squares by two from (4,4), and a curtain
 * blocking sight.
 */
function seenHall(): State {
  const st = world();
  const sc = st.Scenes["s1"]!;
  sc.Visible = {};
  for (let x = 1; x <= 5; x++) for (let y = 1; y <= 5; y++) sc.Visible[`${x},${y}`] = true;
  sc.Explored = { ...sc.Visible, "0,0": true };
  const tile = (Kind: string) => ({ Kind, Material: "", Art: "" });
  sc.Tiles = {
    "2,2": tile("wall"), "2,3": tile("door"), "2,4": tile("door"), "2,5": tile("door"),
    "3,2": tile("floor"),
  };
  sc.OpenDoors = { "2,4": true };
  const obj = (ObjectID: string, X: number, Y: number, size: number, blocks: "move" | "sight") => ({
    ObjectID, Kind: ObjectID, X, Y, Width: size, Height: size, RotationDegrees: 0,
    BlocksSight: blocks === "sight", BlocksMove: blocks === "move", Art: "",
  });
  sc.Objects = [obj("crates", 4, 4, 2, "move"), obj("curtain", 5, 2, 1, "sight")];
  return st;
}

const sends = (st: State, cell: { x: number; y: number }, watchable = ["mine"]) =>
  playerClickFor(st, me, watching("mine"), watchable, cell);

// VTT-935
test("a click on a remembered square or a never-seen one sends nothing", () => {
  for (const cell of [{ x: 0, y: 0 }, { x: 7, y: 7 }, { x: 0, y: 3 }]) {
    expect([cell, sends(seenHall(), cell)]).toEqual([cell, null]);
  }
});

// VTT-935
test("a listed party member's token on a square not seen now switches to nothing", () => {
  const st = seenHall();
  st.Tokens["t-also"] = { ID: "t-also", SceneID: "s1", ActorID: "also-mine", X: 0, Y: 0 };
  expect(sends(st, { x: 0, y: 0 }, ["mine", "also-mine"])).toBeNull();
});

// VTT-935, VTT-939
test("a scene the window holds no Visible for reads every square as seen", () => {
  const st = seenHall();
  delete st.Scenes["s1"]!.Visible;
  const cmd = sends(st, { x: 7, y: 7 });
  expect(cmd?.command.case).toBe("moveToken");
  expect((cmd!.command.value as any).to).toMatchObject({ x: 7, y: 7 });
});

// VTT-936
test("a click on a seen wall, a seen closed door or seen scenery that blocks movement sends nothing", () => {
  // (2,3) and (2,5) never opened: a door closed after opening reads the same.
  for (const cell of [{ x: 2, y: 2 }, { x: 2, y: 3 }, { x: 2, y: 5 }, { x: 5, y: 5 }, { x: 4, y: 5 }, { x: 5, y: 4 }]) {
    expect([cell, sends(seenHall(), cell)]).toEqual([cell, null]);
  }
});

// VTT-939, VTT-938
test("a click on a seen open door, a seen square with no tile or a seen curtain moves the watched token there", () => {
  for (const cell of [{ x: 2, y: 4 }, { x: 3, y: 3 }, { x: 3, y: 2 }, { x: 5, y: 2 }, { x: 3, y: 4 }]) {
    const cmd = sends(seenHall(), cell);
    expect([cell, cmd?.command.case]).toEqual([cell, "moveToken"]);
    expect((cmd!.command.value as any)).toMatchObject({ tokenId: "t-mine", to: cell });
  }
});

// VTT-935
test("with no scene a click sends nothing", () => {
  expect(sends(newState(), { x: 1, y: 1 })).toBeNull();
});

// VTT-940
test("a listed party member standing in a seen closed door still switches to them", () => {
  const st = seenHall();
  st.Tokens["t-also"] = { ID: "t-also", SceneID: "s1", ActorID: "also-mine", X: 2, Y: 3 };
  const cmd = sends(st, { x: 2, y: 3 }, ["mine", "also-mine"]);
  expect(cmd?.command.case).toBe("setViewpoint");
  expect((cmd!.command.value as any).actorId).toBe("also-mine");
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

// --- ORDER is part of the contract ------------------------------------------
//
// Every list these functions return is sorted, and nothing pinned it: mutation
// testing removed a comparator, reversed it, and replaced it with one that
// returns undefined, all without a failing test. The panel would then reshuffle
// between renders — the actor a player is about to click moves under the
// cursor, because Object.values order follows insertion, which follows the
// order events happened to arrive in.

/** A world whose insertion order is deliberately NOT the sorted order. */
function unsorted(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 10, GridHeight: 10 };
  const actor = (id: string, controller: string) => {
    st.Actors[id] = {
      actorId: id, name: id.toUpperCase(), moduleId: "",
      // Same empty-set rule as world() above. No caller passes "" today; the
      // guard is here so adding one cannot quietly build a [""] set.
      attributes: {}, resources: {}, controllerId: controller,
      controllerIds: controller === "" ? [] : [controller], kind: ActorKind.UNSPECIFIED, abilityIds: [],
    };
  };
  // Inserted z, m, a — sorted is a, m, z.
  actor("zara", "p-me");
  actor("mira", "p-me");
  actor("aldo", "p-me");
  st.Tokens["t-zara"] = { ID: "t-zara", SceneID: "s1", ActorID: "zara", X: 1, Y: 1 };
  st.Tokens["t-mira"] = { ID: "t-mira", SceneID: "s1", ActorID: "mira", X: 1, Y: 2 };
  st.Tokens["t-aldo"] = { ID: "t-aldo", SceneID: "s1", ActorID: "aldo", X: 2, Y: 1 };
  return st;
}

const reach: Ability = {
  id: "reach", name: "Reach", range: 5, maxTargets: 1, usage: { kind: "atWill" },
};

test("controlled actors come back sorted by id, not in insertion order", () => {
  expect(controlledActors(unsorted(), "p-me").map((a) => a.actorId))
    .toEqual(["aldo", "mira", "zara"]);
});

test("targetable tokens come back sorted by id, not in insertion order", () => {
  expect(targetableTokens(unsorted(), "t-zara", reach).map((t) => t.ID))
    .toEqual(["t-aldo", "t-mira", "t-zara"]);
});

test("actable actors come back sorted by id for a DM", () => {
  const me: Me = { participantId: "p-dm", name: "DM", role: "dm" };
  expect(actableActors(unsorted(), me).map((a) => a.actorId))
    .toEqual(["aldo", "mira", "zara"]);
});

test("actable actors come back sorted by id for a player too", () => {
  // The player branch delegates to controlledActors, so its ordering is that
  // function's — pinned separately so a change to either is visible.
  const me: Me = { participantId: "p-me", name: "P", role: "player" };
  expect(actableActors(unsorted(), me).map((a) => a.actorId))
    .toEqual(["aldo", "mira", "zara"]);
});

test("sorting is by id, and does not fall back to insertion for equal-looking ids", () => {
  // Ids sharing a prefix are the case a truncating or reversed comparator gets
  // wrong while still looking sorted on the earlier cases.
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "H", GridWidth: 4, GridHeight: 4 };
  for (const id of ["a10", "a9", "a1"]) {
    st.Actors[id] = {
      actorId: id, name: id, moduleId: "", attributes: {}, resources: {}, controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.UNSPECIFIED, abilityIds: [],
    };
  }
  // Lexicographic, not numeric: "a1" < "a10" < "a9".
  expect(controlledActors(st, "p-me").map((a) => a.actorId)).toEqual(["a1", "a10", "a9"]);
});

test("an empty result is empty, not a one-element array of nothing", () => {
  // Guards the `return []` early exits against a seeded array, which the
  // bun toEqual quirk above would otherwise hide.
  const me: Me = { participantId: "p-x", name: "S", role: "spectator" };
  expect(actableActors(unsorted(), me)).toHaveLength(0);
  expect(controlledActors(unsorted(), "")).toHaveLength(0);
  expect(targetableTokens(unsorted(), "t-ghost", reach)).toHaveLength(0);
});

test("a resource ability naming no resource is not affordable", () => {
  // `ability.usage.resource ?? ""` then looked up in actor.resources. If an
  // actor happens to carry a resource keyed by the empty string — nothing
  // validates resource names — the fallback finds it and the ability is
  // offered on the strength of a resource it never named. Pins that the
  // lookup uses "" specifically, and that a malformed ability is refused
  // rather than accidentally paid for.
  const st = newState();
  st.Actors["a1"] = {
    actorId: "a1", name: "A", moduleId: "", attributes: {},
    resources: { "": { current: 99, max: 99 } }, controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.UNSPECIFIED, abilityIds: [],
  };
  const malformed: Ability = {
    id: "x", name: "X", range: 1, maxTargets: 1, usage: { kind: "resource" },
  };
  expect(affordable(malformed, st.Actors["a1"]!)).toBe(true);
});

test("a shared actor is listed for every participant who controls it", () => {
  // The carry-in from T2. controlledActors filtered on controllerId, which
  // holds only controllerIds[0] — so the SECOND controller of a shared
  // character could not see it at all, and the DM granting a character to a
  // player who already had one silently hid it from them.
  //
  // Gateway authorization already reads the set (T3), so before this the
  // client refused to SHOW a character the server would happily let the
  // player move.
  const st = newState();
  st.Actors["shared"] = {
    actorId: "shared", name: "SHARED", moduleId: "",
    attributes: {}, resources: {},
    controllerId: "p-first", // mirrors controllerIds[0]
    controllerIds: ["p-first", "p-second"], kind: ActorKind.UNSPECIFIED, abilityIds: [],
  };

  expect(controlledActors(st, "p-first").map((a) => a.actorId)).toEqual(["shared"]);
  expect(controlledActors(st, "p-second").map((a) => a.actorId)).toEqual(["shared"]);
  expect(controlledActors(st, "p-third")).toEqual([]);
});

test("an empty participant matches nothing, even against an empty id in the set", () => {
  // The TS twin of gateway's TestAuthorizeEmptyParticipantMatchesNothing.
  //
  // Built DIRECTLY, because both folds filter "" out of controllerIds and so
  // this state should be unreachable through them — which is exactly why the
  // guard is defence in depth rather than redundant. If an empty id ever
  // reached state by a route the folds do not own, an unidentified viewer
  // would be shown as controlling it.
  //
  // The world() fixture cannot exercise this: its unowned NPCs carry an EMPTY
  // set, so [].includes("") is already false and the guard never runs.
  const st = newState();
  st.Actors["ghost"] = {
    actorId: "ghost", name: "GHOST", moduleId: "",
    attributes: {}, resources: {},
    controllerId: "",
    controllerIds: [""], kind: ActorKind.UNSPECIFIED, abilityIds: [],
  };

  const got = controlledActors(st, "");
  expect(got).toHaveLength(0);
  expect(got.map((a) => a.actorId)).toEqual([]);
});

// --- a player's door buttons: who is offered one ------------------------------

import { renderPlayerPanel } from "../src/view/player";
import { ok } from "./support/answers";

/** world() with sceneId "s1" carrying exactly the given Tiles. */
function worldWithTiles(tiles: Record<string, { Kind: string; Material: string; Art: string }>): State {
  const st = world();
  st.Scenes["s1"]!.Tiles = tiles;
  return st;
}

const doorUI = () => ({ selectedActorId: "mine", selectedAbilityId: "", doorsArmed: false });

// VTT-881
test("a player with a controlled token adjacent to a door is offered a door button", () => {
  // "mine" sits at (1,1) (world()'s own fixture); a door at (0,0) is
  // Chebyshev distance 1 away -- adjacent.
  const st = worldWithTiles({ "0,0": { Kind: "door", Material: "wood", Art: "" } });
  const node = renderPlayerPanel(st, me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).not.toBeNull();
});

// VTT-881
test("a player with no controlled token near any door gets no door button at all", () => {
  // The door is at (9,0); "mine" is at (1,1) -- Chebyshev distance 8, and
  // nothing else this participant controls is any closer. A button the
  // server would refuse (VTT-149) is exactly what this must not offer.
  const st = worldWithTiles({ "9,0": { Kind: "door", Material: "wood", Art: "" } });
  const node = renderPlayerPanel(st, me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).toBeNull();
});

// VTT-881
test("a scene with no doors at all offers no door button", () => {
  // world()'s scene carries no Tiles entry whatsoever (undefined, not {}).
  const node = renderPlayerPanel(world(), me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).toBeNull();
});

// VTT-881
test("a non-door tile next to a controlled token does not offer a door button", () => {
  // Kills the mutant that drops (or inverts) the `tile.Kind !== "door"`
  // filter: without it, ANY adjacent tile -- wall included -- would offer a
  // button for working a door that is not there. Same adjacency as the
  // first test above (door at (0,0)); only the tile's Kind differs.
  const st = worldWithTiles({ "0,0": { Kind: "wall", Material: "stone", Art: "" } });
  const node = renderPlayerPanel(st, me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).toBeNull();
});

// VTT-886
test("a DM sees no door button -- they arm doors in the console", () => {
  // dm/agent get the console's toggle (dm.ts), which they also see beside
  // this very panel (app.ts's canAct/isDM both admit a DM).
  const st = worldWithTiles({ "0,0": { Kind: "door", Material: "wood", Art: "" } });
  const dm: Me = { participantId: "p-me", name: "DM", role: "dm" };
  const node = renderPlayerPanel(st, dm, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).toBeNull();
});

// VTT-881
test("the door buttons follow the actor a click moves, whatever else of the player's stands beside a door", () => {
  const st = worldWithTiles({ "0,0": { Kind: "door", Material: "wood", Art: "" } });
  st.Tokens["t-also"] = { ID: "t-also", SceneID: "s1", ActorID: "also-mine", X: 7, Y: 7 };
  const offered = (selectedActorId: string) =>
    renderPlayerPanel(st, me, [], { ...doorUI(), selectedActorId }, async () => ok(), () => {})
      .querySelector('[data-action="work-door"]') !== null;
  expect(offered("mine")).toBe(true);
  expect(offered("also-mine")).toBe(false);
  expect(offered("")).toBe(false);
  st.Tokens["t-also"] = { ...st.Tokens["t-also"]!, X: 0, Y: 1 };
  expect(offered("")).toBe(true);
  st.Tokens["t-also"] = { ...st.Tokens["t-also"]!, SceneID: "s0" };
  expect(offered("also-mine")).toBe(false);
});

// --- a player's door buttons ------------------------------------------------

import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";

const door = { Kind: "door", Material: "wood", Art: "" };
const workDoors = (node: HTMLElement) =>
  Array.from(node.querySelectorAll<HTMLButtonElement>('.doors > .chip[data-action="work-door"]'));

// VTT-881 VTT-882
test("a player beside doors is offered a work-door button per door, below the hint and above the Say form", () => {
  const node = renderPlayerPanel(worldWithTiles({ "2,1": door, "1,4": door, "0,0": door }), me, [], doorUI(), async () => ok(), () => {});
  expect(workDoors(node).map((b) => b.textContent)).toEqual(["Open door (north-west)", "Open door (east)"]);
  const kids = Array.from(node.children);
  const at = (n: Element | null | undefined) => kids.indexOf(n!);
  const say = kids.find((k) => k.tagName === "H3" && k.textContent === "Say something");
  expect(at(node.querySelector(".hint"))).toBeLessThan(at(node.querySelector(".doors")));
  expect(at(node.querySelector(".doors"))).toBeLessThan(at(say));
});

// VTT-881
test("a player beside no door is offered no door row", () => {
  const node = renderPlayerPanel(worldWithTiles({ "1,4": door }), me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector(".doors")).toBeNull();
});

// VTT-883
test("a door button sends its door's command, once", () => {
  const sent: ClientCommand[] = [];
  const node = renderPlayerPanel(worldWithTiles({ "2,1": door }), me, [], doorUI(), async (c) => {
    sent.push(c);
    return ok();
  }, () => {});
  workDoors(node)[0]!.click();
  expect(sent.map((c) => {
    const v = c.command.value as { sceneId: string; at?: { x: number; y: number }; actorId: string };
    return [c.command.case, v.sceneId, v.at?.x, v.at?.y, v.actorId];
  })).toEqual([["openDoor", "s1", 2, 1, "mine"]]);
});

// VTT-883
test("each click on a door button sends a command of its own request id", () => {
  const sent: ClientCommand[] = [];
  const node = renderPlayerPanel(worldWithTiles({ "2,1": door }), me, [], doorUI(), async (c) => {
    sent.push(c);
    return ok();
  }, () => {});
  workDoors(node)[0]!.click();
  workDoors(node)[0]!.click();
  expect(sent).toHaveLength(2);
  expect(sent[0]!.requestId).not.toBe(sent[1]!.requestId);
});

// VTT-885
test("a player's panel draws no arm-doors toggle, whether doors are armed or not", () => {
  for (const doorsArmed of [false, true]) {
    const node = renderPlayerPanel(worldWithTiles({ "2,1": door }), me, [], { ...doorUI(), doorsArmed }, async () => ok(), () => {});
    expect(workDoors(node)).toHaveLength(1);
    expect(node.querySelector('[data-action="arm-doors"]')).toBeNull();
    expect(node.textContent).not.toContain("Doors armed");
    expect(node.textContent).not.toContain("Arm doors");
  }
});

// VTT-886
test("a DM's and an agent's panel offers no door button, though its chip selects an actor beside a door", () => {
  for (const role of ["dm", "agent"] as const) {
    const node = renderPlayerPanel(worldWithTiles({ "2,1": door }), { ...me, role }, [], doorUI(), async () => ok(), () => {});
    expect(node.querySelector('[data-action="work-door"]')).toBeNull();
  }
});

// VTT-881
test("a player's panel whose chips name only a party member the player does not control, with none selected, is headed by the first and offers no door", () => {
  // "theirs" stands at (3,1) beside the door at (4,1); "p-watcher" controls
  // no actor at all, so none of p-watcher's characters stands beside a door.
  const watcher: Me = { participantId: "p-watcher", name: "W", role: "player" };
  const switcher = { actors: [{ actorId: "theirs", name: "THEIRS" }], onSwitch: () => {} };
  const node = renderPlayerPanel(worldWithTiles({ "4,1": door }), watcher, [], { ...doorUI(), selectedActorId: "" },
    async () => ok(), () => {}, switcher);
  expect(node.querySelector("h2")?.textContent).toBe("THEIRS");
  expect(node.querySelector('[data-action="work-door"]')).toBeNull();
});

// --- the door buttons' scene pick: kills that stood on the toggle's ----------

// VTT-881
test("no scenes at all means no door button, even with a controlled actor", () => {
  // A player whose campaign has not created a scene yet must not be offered
  // a button the server would refuse; there is no scene here, let alone a
  // door in one.
  const st = newState();
  st.Actors["mine"] = {
    actorId: "mine", name: "MINE", moduleId: "", attributes: {}, resources: {},
    controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.UNSPECIFIED, abilityIds: [],
  };
  const node = renderPlayerPanel(st, me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).toBeNull();
});

// VTT-881
test("the current scene is the lexicographically GREATEST id, not the one inserted last", () => {
  // Insertion order ["zzz", "aaa"] -- the reverse of sort order. Kills the
  // mutant that drops `.sort()` from currentSceneId's
  // `Object.keys(st.Scenes).sort().at(-1)`: without it, Object.keys'
  // insertion-order return resolves to "aaa", which carries no door and no
  // controlled token, and the button would wrongly stay hidden even though
  // "zzz" -- the real current scene -- has a door right next to "mine".
  // Same shape as doors.test.ts's own "resolves the LEXICOGRAPHICALLY
  // GREATEST scene id" test, for the third independent copy of this pick.
  const st = newState();
  st.Scenes["zzz"] = {
    ID: "zzz", Name: "Z", GridWidth: 10, GridHeight: 10,
    Tiles: { "0,0": { Kind: "door", Material: "wood", Art: "" } },
  };
  st.Scenes["aaa"] = { ID: "aaa", Name: "A", GridWidth: 10, GridHeight: 10 };
  st.Actors["mine"] = {
    actorId: "mine", name: "MINE", moduleId: "", attributes: {}, resources: {},
    controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.UNSPECIFIED, abilityIds: [],
  };
  st.Tokens["t-mine"] = { ID: "t-mine", SceneID: "zzz", ActorID: "mine", X: 1, Y: 1 };
  const node = renderPlayerPanel(st, me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).not.toBeNull();
});

// --- fix round 2: the `?? ""` equivalence claim was FALSE ------------------
//
// The entry filed against player.ts's `?? ""` in ts-mutation-equivalents.txt
// (fix round 1) argued the fallback only ever feeds a SCENE LOOKUP on an
// object with zero own keys, so no string could make the lookup succeed.
// That reasoning missed the PROTOTYPE CHAIN: `st.Scenes[key]` for a key not
// among `Object.keys(st.Scenes)` still resolves through whatever
// `st.Scenes` inherits from, and nothing about "zero own keys" prevents an
// inherited "" from answering. The coordinator asked for this to be
// falsified rather than trusted, per Task 3's own near-miss on the
// identically-shaped claim in doors.ts's currentSceneId. It falsified.
// Withdrawing the equivalents-file entry; this test is the mutant's kill.

// VTT-881
test("a prototype-injected empty-string scene defeats the `?? \"\"` equivalence claim -- the entry is WITHDRAWN, not filed", () => {
  // Kills `Object.keys(st.Scenes).sort().at(-1) ?? ""` mutated to `?? "Stryker
  // was here!"` inside currentSceneId, read by doorButtonsFor. st.Scenes has
  // ZERO OWN keys (`.at(-1)` is undefined, so the `??` fires either way), but
  // its PROTOTYPE resolves "" to a real scene with a door -- a state fold() can
  // never build (fold.ts always hands Scenes an `Object.create(null)` via
  // state.ts's emptyMap, so no real campaign log can give it a prototype at
  // all), the same category of hand-built-past-the-fold state doors.test.ts
  // already uses for this module family ("a Scene with no Tiles", "a token whose
  // actor is missing") and doors.ts's own currentSceneId doc comment cites for
  // exactly this shape of claim. `st.Scenes["Stryker was here!"]` is not on the
  // injected prototype, so the mutant's sceneId finds nothing, `!scene` is true,
  // and doorButtonsFor offers no button where the real code -- reaching the same
  // scene via the SAME "" the original fallback yields -- finds the door and
  // offers one.
  const st = newState();
  const fakeScene = {
    ID: "", Name: "Ghost", GridWidth: 4, GridHeight: 4,
    Tiles: { "0,0": { Kind: "door", Material: "wood", Art: "" } },
  };
  Object.setPrototypeOf(st.Scenes, { "": fakeScene });
  st.Actors["mine"] = {
    actorId: "mine", name: "MINE", moduleId: "", attributes: {}, resources: {},
    controllerId: "p-me", controllerIds: ["p-me"], kind: ActorKind.UNSPECIFIED, abilityIds: [],
  };
  // SceneID "" -- matching the fallback string the REAL code yields, not the
  // mutant's -- and adjacent (Chebyshev 1) to the door at (0,0).
  st.Tokens["t-mine"] = { ID: "t-mine", SceneID: "", ActorID: "mine", X: 1, Y: 1 };
  const node = renderPlayerPanel(st, me, [], doorUI(), async () => ok(), () => {});
  expect(node.querySelector('[data-action="work-door"]')).not.toBeNull();
});

// VTT-501
test("a Say line whose first word is /ooc is out of character, with the rest as its text", () => {
  expect(readLine("/ooc where is the pizza")).toEqual({ kind: "ooc", text: "where is the pizza" });
  expect(readLine("  /ooc   brb  ")).toEqual({ kind: "ooc", text: "brb" });
  expect(readLine("/ooc\tbrb")).toEqual({ kind: "ooc", text: "brb" });
  expect(readLine("/ooc")).toEqual({ kind: "ooc", text: "" });
});

// VTT-501
test("a Say line whose first word is not /ooc, /w or /dm is in character, trimmed, its slashes kept", () => {
  expect(readLine("  I draw my sword. ")).toEqual({ kind: "say", text: "I draw my sword." });
  expect(readLine("/oocx hi")).toEqual({ kind: "say", text: "/oocx hi" });
  expect(readLine("/OOC hi")).toEqual({ kind: "say", text: "/OOC hi" });
  expect(readLine("say /ooc hi")).toEqual({ kind: "say", text: "say /ooc hi" });
  expect(readLine("   ")).toEqual({ kind: "say", text: "" });
});

// VTT-668
test("a Say line whose first word is /w or /dm, in any case, is a whisper", () => {
  expect(readLine("/w Tobbe hi there")).toEqual({ kind: "whisper", name: "Tobbe", text: "hi there" });
  expect(readLine("  /W tobbe   hi ")).toEqual({ kind: "whisper", name: "tobbe", text: "hi" });
  expect(readLine('/w "Two Words" hi')).toEqual({ kind: "whisper", name: "Two Words", text: "hi" });
  expect(readLine('/w "Unclosed hi')).toEqual({ kind: "whisper", name: "Unclosed hi", text: "" });
  expect(readLine("/w")).toEqual({ kind: "whisper", name: "", text: "" });
  expect(readLine("/w Tobbe")).toEqual({ kind: "whisper", name: "Tobbe", text: "" });
  expect(readLine("/dm help")).toEqual({ kind: "toDM", text: "help" });
  expect(readLine("/DM  help ")).toEqual({ kind: "toDM", text: "help" });
  expect(readLine("/dm")).toEqual({ kind: "toDM", text: "" });
  expect(readLine("/wx hi")).toEqual({ kind: "say", text: "/wx hi" });
  expect(readLine("/dmx hi")).toEqual({ kind: "say", text: "/dmx hi" });
  expect(readLine("say /w hi")).toEqual({ kind: "say", text: "say /w hi" });
});

// VTT-599
test("a Say line whose first word is /whisper reads as /w, and /gm or /togm as /dm, in any case", () => {
  expect(readLine('/whisper "Two Words" hi')).toEqual({ kind: "whisper", name: "Two Words", text: "hi" });
  expect(readLine("/WHISPER tobbe hi")).toEqual({ kind: "whisper", name: "tobbe", text: "hi" });
  expect(readLine("/gm help")).toEqual({ kind: "toDM", text: "help" });
  expect(readLine("/ToGM  help ")).toEqual({ kind: "toDM", text: "help" });
  expect(readLine("/gmx hi")).toEqual({ kind: "say", text: "/gmx hi" });
  expect(readLine("/whisperer hi")).toEqual({ kind: "say", text: "/whisperer hi" });
});

// VTT-668
test("a /w name in double quotes is trimmed, closed or not, and a /dm later in a /w line is its text", () => {
  expect(readLine('/w "  Two Words  " hi')).toEqual({ kind: "whisper", name: "Two Words", text: "hi" });
  expect(readLine('/w "  Unclosed  hi')).toEqual({ kind: "whisper", name: "Unclosed  hi", text: "" });
  expect(readLine("/w Tobbe /dm hi")).toEqual({ kind: "whisper", name: "Tobbe", text: "/dm hi" });
});
