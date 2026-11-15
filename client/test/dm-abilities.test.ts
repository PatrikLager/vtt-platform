import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import type { Ability } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import { held, ok, settle } from "./support/answers";

// Each test names its own actors: the console keeps what was chosen in a
// select across renders, keyed by the actor.

function ability(id: string, name: string): Ability {
  return { id, name, range: 1, maxTargets: 1, usage: { kind: "atWill" } };
}

const LIST = [ability("fists", "Fists"), ability("rally", ""), ability("chair-swing", "Chair Swing")];

function stateWith(actors: Record<string, { name: string; abilityIds: string[] }>): State {
  const st = newState();
  for (const [actorId, a] of Object.entries(actors)) {
    st.Actors[actorId] = {
      actorId, name: a.name, moduleId: "", attributes: {}, resources: {},
      controllerId: "", controllerIds: [], kind: ActorKind.PARTY_MEMBER, abilityIds: a.abilityIds,
    };
  }
  return st;
}

// A held console's answers wait until its release, as the server's do.
function console_(st: State, abilities: readonly Ability[] = LIST, answers?: ReturnType<typeof held>) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  const node = renderDMConsole({
    st, participants: [], adventures: [], maps: [], abilities,
    guideFor: async () => null, joinLink: null, roster: null, origin: "https://table.example",
    refreshSharing: () => {},
    send: async (c) => {
      sent.push(c);
      return answers === undefined ? ok() : answers.send(c);
    },
    rerender: () => {}, notify: (m) => notices.push(m), confirm: () => true,
    doorsArmed: false, toggleDoors: () => {},
  });
  document.body.appendChild(node);
  const row = (actorId: string) => node.querySelector(`.ability-actor[data-actor="${actorId}"]`) as HTMLElement | null;
  return {
    node, sent, notices, row,
    chips: (actorId: string) =>
      Array.from(row(actorId)?.querySelectorAll<HTMLButtonElement>('[data-action="take-ability"]') ?? []),
    pick: (actorId: string) => row(actorId)?.querySelector<HTMLSelectElement>("select.give-ability") ?? null,
    give: (actorId: string) => row(actorId)?.querySelector<HTMLButtonElement>('[data-action="give-ability"]') ?? null,
    choose: (actorId: string, abilityId: string) => {
      const pick = row(actorId)!.querySelector<HTMLSelectElement>("select.give-ability")!;
      pick.value = abilityId;
      pick.dispatchEvent(new Event("change"));
    },
  };
}

beforeEach(() => {
  document.body.innerHTML = "";
});

// VTT-730
test("each actor's row offers a take of each id its sheet names, once each, in the sheet's order", async () => {
  const c = console_(stateWith({
    "ab-take": { name: "Lera", abilityIds: ["rally", "fists", "rally", "no-such-move"] },
  }));
  const chips = c.chips("ab-take");
  expect(chips.map((b) => b.dataset["ability"])).toEqual(["rally", "fists", "no-such-move"]);
  expect(chips.map((b) => b.textContent)).toEqual(["rally ✕", "Fists ✕", "no-such-move ✕"]);
  chips[1]!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("takeAbility");
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "ab-take", abilityId: "fists" });
});

// VTT-730
test("each actor's row offers a give of each listed ability its sheet does not name, in the list's order", async () => {
  const c = console_(stateWith({ "ab-give": { name: "Lera", abilityIds: ["rally"] } }));
  const pick = c.pick("ab-give")!;
  expect(Array.from(pick.options).map((o) => o.value)).toEqual(["", "fists", "chair-swing"]);
  expect(Array.from(pick.options).map((o) => o.textContent)).toEqual(["choose an ability", "Fists", "Chair Swing"]);
  expect(c.give("ab-give")!.textContent).toBe("Give");
  pick.value = "chair-swing";
  pick.dispatchEvent(new Event("change"));
  c.give("ab-give")!.click();
  await settle();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("giveAbility");
  expect(c.sent[0]!.command.value).toMatchObject({ actorId: "ab-give", abilityId: "chair-swing" });
});

// VTT-730
test("no ability the sheet names is offered to give, whatever the list's order", () => {
  const c = console_(stateWith({ "ab-named": { name: "", abilityIds: ["chair-swing", "fists"] } }));
  expect(Array.from(c.pick("ab-named")!.options).map((o) => o.value)).toEqual(["", "rally"]);
  expect(c.row("ab-named")!.querySelector(".ability-name")!.textContent).toBe("ab-named");
});

// VTT-730
test("a give with nothing chosen says so and sends nothing", async () => {
  const c = console_(stateWith({ "ab-none-chosen": { name: "Lera", abilityIds: [] } }));
  c.give("ab-none-chosen")!.click();
  await settle();
  expect(c.sent).toHaveLength(0);
  expect(c.notices).toEqual(["Choose an ability to give first."]);
});

// VTT-730
test("a choice is kept across renders, and an actor whose sheet names every listed ability is offered no give", () => {
  const st = stateWith({
    "ab-kept": { name: "Lera", abilityIds: [] },
    "ab-full": { name: "Dorn", abilityIds: ["fists", "rally", "chair-swing"] },
  });
  const first = console_(st);
  const pick = first.pick("ab-kept")!;
  pick.value = "rally";
  pick.dispatchEvent(new Event("change"));
  expect(console_(st).pick("ab-kept")!.value).toBe("rally");
  expect(first.pick("ab-full")).toBeNull();
  expect(first.give("ab-full")).toBeNull();
  expect(first.chips("ab-full")).toHaveLength(3);
});

// VTT-730
test("with no ability listed the console offers no give, and still a take; with no actor, no Abilities group", () => {
  const c = console_(stateWith({ "ab-unlisted": { name: "Lera", abilityIds: ["fists"] } }), []);
  expect(c.pick("ab-unlisted")).toBeNull();
  expect(c.chips("ab-unlisted").map((b) => b.textContent)).toEqual(["fists ✕"]);
  const empty = console_(newState());
  expect(Array.from(empty.node.querySelectorAll("h3")).map((h) => h.textContent)).not.toContain("Abilities");
  expect(Array.from(c.node.querySelectorAll("h3")).map((h) => h.textContent)).toContain("Abilities");
});

// VTT-730
test("a give still unanswered holds back that actor's give of that ability alone, not of another", async () => {
  const answers = held();
  const c = console_(stateWith({ "ab-lock-one": { name: "Lera", abilityIds: [] } }), LIST, answers);
  c.choose("ab-lock-one", "fists");
  c.give("ab-lock-one")!.click();
  c.give("ab-lock-one")!.click();
  c.choose("ab-lock-one", "chair-swing");
  c.give("ab-lock-one")!.click();
  answers.release();
  await settle();
  expect(c.sent.map((s) => s.command.value)).toMatchObject([
    { actorId: "ab-lock-one", abilityId: "fists" },
    { actorId: "ab-lock-one", abilityId: "chair-swing" },
  ]);
});

// VTT-730
test("a give still unanswered holds back no other actor's give of the same ability", async () => {
  const answers = held();
  const c = console_(stateWith({
    "ab-lock-a": { name: "Lera", abilityIds: [] },
    "ab-lock-b": { name: "Dorn", abilityIds: [] },
  }), LIST, answers);
  c.choose("ab-lock-a", "rally");
  c.give("ab-lock-a")!.click();
  c.choose("ab-lock-b", "rally");
  c.give("ab-lock-b")!.click();
  answers.release();
  await settle();
  expect(c.sent.map((s) => s.command.value)).toMatchObject([
    { actorId: "ab-lock-a", abilityId: "rally" },
    { actorId: "ab-lock-b", abilityId: "rally" },
  ]);
});
