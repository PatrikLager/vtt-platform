import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  EnvelopeSchema, SessionStartedSchema, SessionEndedSchema, SceneCreatedSchema,
  ActorAddedSchema, TokenPlacedSchema, TokenMovedSchema, NarrationAddedSchema,
  NoteUpsertedSchema, NoteDeletedSchema, ConditionAppliedSchema,
  ConditionRemovedSchema, ResourceChangedSchema, AbilityUsedSchema,
  AttackRolledSchema, AdventureLoadedSchema, EventsRetractedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import { renderSpectator, describe as describeEvent } from "../src/view/spectator";
import { newState, type State } from "../src/state";
import { renderPlayerPanel } from "../src/view/player";
import type { Ability, Me } from "../src/metadata";

const env = (seq: number, payload: Envelope["payload"]): Envelope =>
  create(EnvelopeSchema, { eventId: `e${seq}`, sequence: BigInt(seq), payload });

function world(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "The Hall", GridWidth: 6, GridHeight: 4 };
  st.Actors["a1"] = {
    actorId: "a1", name: "Lera", moduleId: "", attributes: {},
    resources: { vigor: { current: 3, max: 10 } }, controllerId: "p-me",
  };
  st.Tokens["t1"] = { ID: "t1", SceneID: "s1", ActorID: "a1", X: 2, Y: 1 };
  st.Conditions["a1"] = [{ ID: "dazed", Source: "dm", AppliedSeq: 4 }];
  st.Notes["k"] = { Title: "A Note", Text: "body text", UpdatedSeq: 5 };
  st.Sessions = [{ ID: "sess-1", Name: "Night One", StartSeq: 1, EndSeq: 0 }];
  return st;
}

function render(st: State, log: Envelope[] = []): HTMLElement {
  const root = document.createElement("div");
  renderSpectator(root, st, log, "connected");
  return root;
}

test("the board draws a disc per token, with its initial", () => {
  const root = render(world());
  const tokens = root.querySelectorAll(".token");
  expect(tokens).toHaveLength(1);
  expect(root.querySelector(".initial")?.textContent).toBe("L");
});

test("every resource shows as a current/max chip, named on hover", () => {
  const chip = render(world()).querySelector(".chip") as HTMLElement;
  expect(chip.textContent).toBe("3/10");
  // The NAME is the tooltip, never abbreviated onto the face.
  expect(chip.title).toBe("vigor");
});

test("every condition shows as a dot carrying its name", () => {
  const dot = render(world()).querySelector(".dot") as HTMLElement;
  expect(dot.title).toBe("dazed");
});

test("a token is positioned by its grid coordinates", () => {
  const tok = render(world()).querySelector(".token") as HTMLElement;
  expect(tok.style.left).toBe("88px"); // x=2 * 44
  expect(tok.style.top).toBe("44px"); // y=1 * 44
});

test("notes render with title and body", () => {
  const notes = render(world()).querySelector(".notes")!;
  expect(notes.textContent).toContain("A Note");
  expect(notes.textContent).toContain("body text");
});

test("an open session is named in the status bar", () => {
  expect(render(world()).querySelector(".session")?.textContent).toContain("Night One");
});

test("empty state reads honestly rather than rendering a blank panel", () => {
  const root = render(newState());
  expect(root.querySelector(".board")?.textContent).toContain("No scene");
  expect(root.querySelector(".feed")?.textContent).toContain("Nothing has happened");
  expect(root.querySelector(".notes")?.textContent).toContain("No notes");
});

test("the ticker shows sequence numbers, newest first", () => {
  const log = [
    env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
    env(2, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s1", name: "H", gridWidth: 6, gridHeight: 4 }) }),
  ];
  const ticks = render(world(), log).querySelectorAll(".tick");
  expect(ticks[0]!.textContent).toContain("#2");
});

test("the ticker is bounded — an all-night session must not render thousands of rows", () => {
  const log = Array.from({ length: 200 }, (_, i) =>
    env(i + 1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
  );
  const ticks = render(world(), log).querySelectorAll(".tick").length;
  expect(ticks).toBeLessThanOrEqual(40);
  // Paired lower bound: a ticker that rendered nothing at all would satisfy
  // the cap on its own, so the cap alone proves nothing.
  expect(ticks).toBeGreaterThan(0);
});

test("in-character speech is marked with its speaker; table talk is not", () => {
  const log = [
    env(1, { case: "narrationAdded", value: create(NarrationAddedSchema, { text: "Hi", as: "Goblin" }) }),
    env(2, { case: "narrationAdded", value: create(NarrationAddedSchema, { text: "ooc" }) }),
  ];
  const root = render(world(), log);
  expect(root.querySelector(".speaker")?.textContent).toBe("Goblin: ");
  expect(root.querySelectorAll(".narration")).toHaveLength(1);
});

test("describe renders a real label for every event kind, not the fallback", () => {
  // The previous version of this test asserted only `label.length > 0` and
  // `label !== "event"` over six kinds. describe()'s default branch is
  // `return p.case ?? "event"`, and every case name is a non-empty string
  // that is not "event" — so DELETING THE ENTIRE SWITCH left all six
  // assertions passing. It proved that protobuf sets `payload.case`.
  //
  // So: exact strings, and every kind describe() handles. The fallback is
  // separately caught by asserting no label equals its own case name.
  const cases: [Envelope, string][] = [
    [env(1, { case: "sessionStarted", value: create(SessionStartedSchema, { name: "S" }) }),
      "session started — S"],
    [env(2, { case: "sessionEnded", value: create(SessionEndedSchema, {}) }),
      "session ended"],
    [env(3, { case: "sceneCreated", value: create(SceneCreatedSchema, { sceneId: "s", name: "N", gridWidth: 1, gridHeight: 1 }) }),
      'scene "N"'],
    [env(4, { case: "actorAdded", value: create(ActorAddedSchema, { actor: { actorId: "a", name: "A" } }) }),
      "actor A joined"],
    [env(5, { case: "tokenPlaced", value: create(TokenPlacedSchema, { tokenId: "t", sceneId: "s", actorId: "a", position: { x: 1, y: 2 } }) }),
      "t placed at 1,2"],
    [env(6, { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t", to: { x: 3, y: 4 } }) }),
      "t moved to 3,4"],
    [env(7, { case: "conditionApplied", value: create(ConditionAppliedSchema, { actorId: "a", conditionId: "prone" }) }),
      "a gained prone"],
    [env(8, { case: "conditionRemoved", value: create(ConditionRemovedSchema, { actorId: "a", conditionId: "prone" }) }),
      "a lost prone"],
    [env(9, { case: "resourceChanged", value: create(ResourceChangedSchema, { actorId: "a", resource: "hp", newValue: 7 }) }),
      "a hp → 7"],
    [env(10, { case: "noteUpserted", value: create(NoteUpsertedSchema, { key: "k", title: "t", text: "x" }) }),
      'note "k" updated'],
    [env(11, { case: "noteDeleted", value: create(NoteDeletedSchema, { key: "k" }) }),
      'note "k" deleted'],
    [env(12, { case: "abilityUsed", value: create(AbilityUsedSchema, { actorId: "a", abilityId: "cleave" }) }),
      "a used cleave"],
    [env(13, { case: "attackRolled", value: create(AttackRolledSchema, { total: 17 }) }),
      "roll 17"],
    [env(14, { case: "adventureLoaded", value: create(AdventureLoadedSchema, { adventureId: "adv" }) }),
      "adventure adv loaded"],
  ];

  for (const [e, want] of cases) {
    expect(describeEvent(e)).toBe(want);
  }

  // Anything reaching the default branch renders as its own case name, which
  // is developer vocabulary leaking into the story feed.
  for (const [e] of cases) {
    expect(describeEvent(e)).not.toBe(e.payload.case);
  }
});

test("an event kind describe() does not handle degrades to its case name", () => {
  // Pins the fallback itself, so the test above cannot be satisfied BY it.
  const e = env(99, { case: "eventsRetracted", value: create(EventsRetractedSchema, { fromSequence: 1n, toSequence: 2n, reason: "r" }) });
  expect(describeEvent(e)).toBe("eventsRetracted");
});

// --- player panel -----------------------------------------------------------

const me: Me = { participantId: "p-me", name: "Me", role: "player", controls: ["a1"] };
const atWill: Ability = { id: "swing", name: "Swing", range: 2, maxTargets: 1, usage: { kind: "atWill" } };
const costly: Ability = {
  id: "surge", name: "Surge", range: 1, maxTargets: 1,
  usage: { kind: "resource", resource: "vigor", cost: 99 },
};

function panel(st: State, abilities: Ability[], ui = { selectedActorId: "", selectedAbilityId: "" }) {
  const sent: unknown[] = [];
  const node = renderPlayerPanel(st, me, abilities, ui, (c) => sent.push(c), () => {});
  return { node, sent, ui };
}

test("a participant controlling nothing is told so, not shown empty controls", () => {
  const st = world();
  st.Actors["a1"]!.controllerId = "someone-else";
  expect(panel(st, [atWill]).node.textContent).toContain("do not control");
});

test("an unaffordable ability is shown but DISABLED, not hidden", () => {
  // A player needs to know the ability exists and why it is unavailable.
  const node = panel(world(), [costly]).node;
  const btn = Array.from(node.querySelectorAll("button")).find((b) => b.textContent === "Surge")!;
  expect(btn.disabled).toBe(true);
  expect(btn.title).toContain("not enough");
});

test("arming an ability lists the targets in range", () => {
  const { node } = panel(world(), [atWill], { selectedActorId: "a1", selectedAbilityId: "swing" });
  expect(node.textContent).toContain("Targets (range 2)");
});

test("with no ability armed the panel invites a move instead", () => {
  expect(panel(world(), [atWill]).node.textContent).toContain("Click the board to move");
});

test("sending narration requires text, and clears the box afterwards", () => {
  const { node, sent } = panel(world(), []);
  const text = node.querySelector(".text") as HTMLInputElement;
  const send = Array.from(node.querySelectorAll("button")).find((b) => b.textContent === "Send")!;

  send.click(); // empty
  expect(sent).toHaveLength(0);

  text.value = "I step forward.";
  send.click();
  expect(sent).toHaveLength(1);
  expect(text.value).toBe("");
});
