import { test, expect } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  ActorAddedSchema, ActorKind, ActorRemovedSchema, EnvelopeSchema, TokenDescribedSchema,
  TokenHiddenSchema, TokenPlacedSchema, type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { newState, type State } from "../src/state";
import { UNNAMED, initialOf, namer } from "../src/view/naming";

const env = (seq: number, payload: Envelope["payload"]): Envelope =>
  create(EnvelopeSchema, { sequence: BigInt(seq), payload });

function actor(st: State, id: string, name: string, kind: ActorKind): void {
  st.Actors[id] = {
    actorId: id, name, moduleId: "", attributes: {}, resources: {},
    controllerId: "", controllerIds: [], kind,
  };
}

// A party member, a named creature and an unnamed one, each with a token.
function table(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 6, GridHeight: 4 };
  actor(st, "act-lera", "Lera", ActorKind.PARTY_MEMBER);
  actor(st, "act-gob", "Goblin Cutter", ActorKind.NON_PARTY);
  actor(st, "act-rat", "", ActorKind.NON_PARTY);
  actor(st, "act-nobody", "", ActorKind.PARTY_MEMBER);
  st.Tokens["tok-lera"] = { ID: "tok-lera", SceneID: "s1", ActorID: "act-lera", X: 0, Y: 0 };
  st.Tokens["tok-gob"] = {
    ID: "tok-gob", SceneID: "s1", ActorID: "act-gob", X: 1, Y: 0,
    Name: "a hunched goblin", GMName: "the cutter",
  };
  st.Tokens["tok-gob-2"] = { ID: "tok-gob-2", SceneID: "s1", ActorID: "act-gob", X: 2, Y: 0 };
  st.Tokens["tok-rat"] = { ID: "tok-rat", SceneID: "s1", ActorID: "act-rat", X: 3, Y: 0 };
  st.Tokens["tok-nobody"] = { ID: "tok-nobody", SceneID: "s1", ActorID: "act-nobody", X: 4, Y: 0 };
  return st;
}

// VTT-475
test("a player's window names a token by its name, else a party member's name, else a creature", () => {
  const n = namer(table(), [], false, []);
  expect(n.token("tok-gob")).toBe("a hunched goblin");
  expect(n.token("tok-lera")).toBe("Lera");
  expect(n.token("tok-gob-2")).toBe(UNNAMED);
  expect(n.token("tok-rat")).toBe(UNNAMED);
  expect(n.token("tok-nobody")).toBe(UNNAMED);
  expect(n.token("tok-unknown")).toBe(UNNAMED);
  expect(n.actor("act-gob")).toBe("a hunched goblin");
  expect(n.actor("act-lera")).toBe("Lera");
  expect(n.actor("act-rat")).toBe(UNNAMED);
  expect(n.actor("act-unknown")).toBe(UNNAMED);
  expect(n.dm).toBe(false);
});

// VTT-484
test("a DM's window names a token by its GM name, else its actor's name, else a creature", () => {
  const st = table();
  st.Tokens["tok-lera"]!.Name = "a tall figure";
  const n = namer(st, [], true, []);
  expect(n.token("tok-gob")).toBe("the cutter");
  expect(n.token("tok-gob-2")).toBe("Goblin Cutter");
  expect(n.token("tok-lera")).toBe("Lera");
  expect(n.token("tok-rat")).toBe(UNNAMED);
  expect(n.token("tok-unknown")).toBe(UNNAMED);
  expect(n.actor("act-gob")).toBe("the cutter");
  expect(n.actor("act-lera")).toBe("Lera");
  expect(n.actor("act-unknown")).toBe(UNNAMED);
  expect(n.dm).toBe(true);
});

// VTT-490
test("a DM's window knows what a player's window names each token", () => {
  const st = table();
  st.Tokens["tok-lera"]!.GMName = "Lera of the Hollow";
  const n = namer(st, [], true, []);
  expect(n.token("tok-lera")).toBe("Lera of the Hollow");
  expect(n.shown("tok-lera")).toBe("Lera");
  expect(n.shown("tok-gob")).toBe("a hunched goblin");
  expect(n.shown("tok-gob-2")).toBe(UNNAMED);
  expect(n.shown("tok-unknown")).toBe(UNNAMED);
});

// VTT-475
test("an actor is named by the first of its tokens in id order whose name is set", () => {
  const st = newState();
  actor(st, "act-gob", "", ActorKind.NON_PARTY);
  st.Tokens["tok-b"] = { ID: "tok-b", SceneID: "s1", ActorID: "act-gob", X: 0, Y: 0, Name: "the second" };
  st.Tokens["tok-c"] = { ID: "tok-c", SceneID: "s1", ActorID: "act-gob", X: 0, Y: 0 };
  st.Tokens["tok-a"] = { ID: "tok-a", SceneID: "s1", ActorID: "act-gob", X: 0, Y: 0, Name: "the first" };
  expect(namer(st, [], false, []).actor("act-gob")).toBe("the first");
});

// VTT-489
test("a token and an actor the fold no longer holds are named by the newest entry that named them", () => {
  const log = [
    env(1, { case: "actorAdded", value: create(ActorAddedSchema, {
      actor: { actorId: "act-bram", name: "Bram", kind: ActorKind.PARTY_MEMBER } }) }),
    env(2, { case: "actorAdded", value: create(ActorAddedSchema, {
      actor: { actorId: "act-shade", name: "Shade", kind: ActorKind.NON_PARTY } }) }),
    env(3, { case: "tokenPlaced", value: create(TokenPlacedSchema, {
      tokenId: "tok-shade", sceneId: "s1", actorId: "act-shade", name: "a shadow", gmName: "the shade" }) }),
    env(4, { case: "tokenDescribed", value: create(TokenDescribedSchema, {
      tokenId: "tok-shade", name: "a tall shadow", gmName: "the tall shade" }) }),
    env(5, { case: "tokenPlaced", value: create(TokenPlacedSchema, {
      tokenId: "tok-bram", sceneId: "s1", actorId: "act-bram" }) }),
    env(6, { case: "tokenHidden", value: create(TokenHiddenSchema, { tokenId: "tok-shade" }) }),
    env(7, { case: "tokenHidden", value: create(TokenHiddenSchema, { tokenId: "tok-bram" }) }),
    env(8, { case: "actorRemoved", value: create(ActorRemovedSchema, { actorId: "act-bram" }) }),
    env(9, { case: "actorAdded", value: create(ActorAddedSchema, {
      actor: { actorId: "act-ghoul", name: "Ghoul", kind: ActorKind.NON_PARTY } }) }),
    env(10, { case: "actorRemoved", value: create(ActorRemovedSchema, { actorId: "act-ghoul" }) }),
    create(EnvelopeSchema, { sequence: 11n }),
    env(12, { case: "actorAdded", value: create(ActorAddedSchema, {}) }),
  ];
  const player = namer(newState(), log, false, []);
  expect(player.token("tok-shade")).toBe("a tall shadow");
  expect(player.token("tok-bram")).toBe("Bram");
  expect(player.actor("act-bram")).toBe("Bram");
  expect(player.actor("act-shade")).toBe("a tall shadow");
  expect(player.actor("act-ghoul")).toBe(UNNAMED);
  const dm = namer(newState(), log, true, []);
  expect(dm.token("tok-shade")).toBe("the tall shade");
  expect(dm.actor("act-shade")).toBe("the tall shade");
  expect(dm.actor("act-bram")).toBe("Bram");
  expect(dm.actor("act-ghoul")).toBe("Ghoul");
});

// VTT-477
test("an ability is named by the ruleset's display name, else its id", () => {
  const n = namer(newState(), [], false, [{ id: "longsword-strike", name: "Longsword Strike" }]);
  expect(n.ability("longsword-strike")).toBe("Longsword Strike");
  expect(n.ability("rally")).toBe("rally");
});

// VTT-475
test("a disc's initial is the name's first letter in upper case, and ? for a creature", () => {
  expect(initialOf("a hunched goblin")).toBe("A");
  expect(initialOf("Lera")).toBe("L");
  expect(initialOf("élan")).toBe("É");
  expect(initialOf(UNNAMED)).toBe("?");
});
