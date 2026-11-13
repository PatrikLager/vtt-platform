import "./support/dom";
import { test, expect, afterEach } from "bun:test";
import { moveCommandFor, renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { offered } from "../src/player";
import { newState, type Actor, type State } from "../src/state";
import type { Ability, Me } from "../src/metadata";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { ok } from "./support/answers";

const FISTS: Ability = { id: "fists", name: "Fists", range: 1, maxTargets: 1, usage: { kind: "atWill" } };
const SPLASH: Ability = { id: "splash-of-water", name: "Splash of Water", range: 1, maxTargets: 1, usage: { kind: "atWill" } };
const SOBER: Ability = { id: "sober-up", name: "Sober Up", range: 0, maxTargets: 1, usage: { kind: "atWill" } };
const LIST = [FISTS, SOBER, SPLASH];

function sheet(controller: string, abilityIds: string[]): Actor {
  return {
    actorId: "act-mara", name: "Mara", moduleId: "", attributes: {}, resources: {},
    controllerId: controller, controllerIds: [controller], kind: ActorKind.PARTY_MEMBER, abilityIds,
  };
}

function table(a: Actor): State {
  const st = newState();
  st.Scenes["cellar"] = { ID: "cellar", Name: "Cellar", GridWidth: 6, GridHeight: 6 };
  st.Actors[a.actorId] = a;
  st.Tokens["tok-mara"] = { ID: "tok-mara", SceneID: "cellar", ActorID: a.actorId, X: 1, Y: 1 };
  return st;
}

function chips(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll("button")).map((b) => (b.textContent ?? "").trim());
}

afterEach(() => { document.body.innerHTML = ""; });

// VTT-695
test("an actor is offered the window's abilities its sheet names, in the list's order", () => {
  const a = sheet("p-mara", ["splash-of-water", "fists", "kick"]);
  expect(offered(LIST, a).map((x) => x.id)).toEqual(["fists", "splash-of-water"]);
  expect(offered(LIST, sheet("p-mara", []))).toEqual([]);
});

// VTT-695
test("the panel offers only what the actor's sheet names, in a player's, a DM's and an agent's window", () => {
  const roles: Me[] = [
    { participantId: "p-mara", name: "Patrik", role: "player" },
    { participantId: "p-dm", name: "Dee", role: "dm" },
    { participantId: "p-agent", name: "Agent", role: "agent" },
  ];
  for (const me of roles) {
    const ui: PlayerUIState = { selectedActorId: "act-mara", selectedAbilityId: "", doorsArmed: false };
    const el = renderPlayerPanel(table(sheet(me.participantId, ["splash-of-water"])), me, LIST, ui, async () => ok(), () => {});
    const t = chips(el);
    expect(t.filter((x) => x === "Splash of Water").length).toBe(1);
    expect(t.some((x) => x === "Fists" || x === "Sober Up")).toBe(false);
  }
});

// VTT-695
test("a selection the sheet does not name arms nothing", () => {
  const ui: PlayerUIState = { selectedActorId: "act-mara", selectedAbilityId: "fists", doorsArmed: false };
  const me: Me = { participantId: "p-mara", name: "Patrik", role: "player" };
  const el = renderPlayerPanel(table(sheet("p-mara", ["splash-of-water"])), me, LIST, ui, async () => ok(), () => {});
  expect(el.querySelector('[data-action="use-ability"]')).toBeNull();
  expect(Array.from(el.querySelectorAll("h3")).some((h) => (h.textContent ?? "").startsWith("Targets"))).toBe(false);
});

// VTT-695
test("an actor whose sheet names none is offered no ability and no Abilities heading", () => {
  const ui: PlayerUIState = { selectedActorId: "act-mara", selectedAbilityId: "", doorsArmed: false };
  const me: Me = { participantId: "p-mara", name: "Patrik", role: "player" };
  const el = renderPlayerPanel(table(sheet("p-mara", [])), me, LIST, ui, async () => ok(), () => {});
  expect(Array.from(el.querySelectorAll("h3")).some((h) => h.textContent === "Abilities")).toBe(false);
});

// Keep a board click with nothing selected a move, even for a sheet naming "".
test("a board click with nothing selected moves, whatever the sheet names", () => {
  const me: Me = { participantId: "p-mara", name: "Patrik", role: "player" };
  const ui: PlayerUIState = { selectedActorId: "act-mara", selectedAbilityId: "", doorsArmed: false };
  expect(moveCommandFor(table(sheet("p-mara", [""])), me, ui, { x: 3, y: 3 })?.command.case).toBe("moveToken");
});
