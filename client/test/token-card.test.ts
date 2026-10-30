import "./support/dom";

import { test, expect } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { AbilityUsedSchema, ActorKind, EnvelopeSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { describe as describeEvent, renderSpectator, type ViewExtras } from "../src/view/spectator";
import { renderPlayerPanel } from "../src/view/player";
import { newState, type State } from "../src/state";
import { UNNAMED, namer } from "../src/view/naming";
import type { Ability, Me } from "../src/metadata";
import { ok } from "./support/answers";

const IDS = ["act-lera", "act-gob", "act-rat", "t-lera", "tok-gob", "tok-rat"];

// Lera is a party member; the goblin's token carries all four texts; the rat
// has no name anywhere.
function table(): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 6, GridHeight: 4 };
  const actor = (id: string, name: string, kind: ActorKind, controllers: string[] = []) => {
    st.Actors[id] = {
      actorId: id, name, moduleId: "", attributes: {}, resources: {},
      controllerId: controllers[0] ?? "", controllerIds: controllers, kind,
    };
  };
  actor("act-lera", "Lera", ActorKind.PARTY_MEMBER, ["p-me"]);
  actor("act-gob", "Goblin Cutter", ActorKind.NON_PARTY);
  actor("act-rat", "", ActorKind.NON_PARTY);
  st.Tokens["t-lera"] = { ID: "t-lera", SceneID: "s1", ActorID: "act-lera", X: 2, Y: 1 };
  st.Tokens["tok-gob"] = {
    ID: "tok-gob", SceneID: "s1", ActorID: "act-gob", X: 3, Y: 1,
    Name: "a hunched goblin", Notes: "It sleeps.", GMName: "Goblin Cutter", GMNotes: "Wakes at any noise.",
  };
  st.Tokens["tok-rat"] = { ID: "tok-rat", SceneID: "s1", ActorID: "act-rat", X: 4, Y: 1 };
  return st;
}

function render(st: State, extras: ViewExtras = {}, log: Envelope[] = [], root = document.createElement("div")) {
  renderSpectator(root, st, log, "connected", extras);
  return root;
}

const disc = (root: HTMLElement, id: string) =>
  root.querySelector(`.token[data-token-id="${id}"]`) as HTMLElement;
const card = (root: HTMLElement, id: string, part: string) =>
  disc(root, id).querySelector(`.token-card .${part}`)?.textContent ?? null;

// VTT-475
test("a player's disc shows the naming's initial and title, ? and a creature for an unnamed one", () => {
  const root = render(table());
  expect(disc(root, "tok-gob").title).toBe("a hunched goblin");
  expect(disc(root, "tok-gob").querySelector(".initial")?.textContent).toBe("A");
  expect(disc(root, "t-lera").title).toBe("Lera");
  expect(disc(root, "tok-rat").title).toBe(UNNAMED);
  expect(disc(root, "tok-rat").querySelector(".initial")?.textContent).toBe("?");
  for (const d of Array.from(root.querySelectorAll(".token"))) {
    for (const id of IDS) expect((d as HTMLElement).title).not.toContain(id);
  }
});

// VTT-484
test("a DM's disc is titled by the GM name", () => {
  const root = render(table(), { dm: true });
  expect(disc(root, "tok-gob").title).toBe("Goblin Cutter");
  expect(disc(root, "tok-gob").querySelector(".initial")?.textContent).toBe("G");
});

// VTT-474 VTT-483
test("a player's card holds the token's name and notes and no GM text", () => {
  const root = render(table());
  expect(card(root, "tok-gob", "card-name")).toBe("a hunched goblin");
  expect(card(root, "tok-gob", "card-notes")).toBe("It sleeps.");
  expect(card(root, "tok-gob", "card-shown")).toBeNull();
  expect(card(root, "tok-gob", "card-gm-notes")).toBeNull();
  const text = disc(root, "tok-gob").querySelector(".token-card")?.textContent ?? "";
  expect(text).not.toContain("Goblin Cutter");
  expect(text).not.toContain("Wakes");
  expect(card(root, "tok-rat", "card-name")).toBe(UNNAMED);
  expect(card(root, "tok-rat", "card-notes")).toBeNull();
});

// VTT-474 VTT-483 VTT-484 VTT-490
test("a DM's card adds what players see and the GM notes", () => {
  const root = render(table(), { dm: true });
  expect(card(root, "tok-gob", "card-name")).toBe("Goblin Cutter");
  expect(card(root, "tok-gob", "card-shown")).toBe("players see: a hunched goblin");
  expect(card(root, "tok-gob", "card-notes")).toBe("It sleeps.");
  expect(card(root, "tok-gob", "card-gm-notes")).toBe("Wakes at any noise.");
  expect(card(root, "tok-rat", "card-shown")).toBe(`players see: ${UNNAMED}`);
  expect(card(root, "t-lera", "card-shown")).toBe("players see: Lera");
  expect(card(root, "tok-rat", "card-notes")).toBeNull();
  expect(card(root, "tok-rat", "card-gm-notes")).toBeNull();
});

// VTT-474
test("a right-click on a disc is prevented and pins its card, which a repaint keeps", () => {
  const pinned: string[] = [];
  const extras: ViewExtras = { onCardPin: (id) => pinned.push(id) };
  const root = render(table(), extras);
  const ev = new MouseEvent("contextmenu", { bubbles: true, cancelable: true });
  disc(root, "tok-gob").dispatchEvent(ev);
  expect(ev.defaultPrevented).toBe(true);
  expect(pinned).toEqual(["tok-gob"]);

  const shown = render(table(), { ...extras, pinnedCard: "tok-gob" });
  render(table(), { ...extras, pinnedCard: "tok-gob" }, [], shown);
  expect(disc(shown, "tok-gob").classList.contains("pinned")).toBe(true);
  expect(disc(shown, "tok-rat").classList.contains("pinned")).toBe(false);
});

// VTT-474
test("a right-click on a window that pins nothing is still prevented", () => {
  const root = render(table());
  const ev = new MouseEvent("contextmenu", { bubbles: true, cancelable: true });
  disc(root, "tok-gob").dispatchEvent(ev);
  expect(ev.defaultPrevented).toBe(true);
  expect(disc(root, "tok-gob").classList.contains("pinned")).toBe(false);
});

// VTT-408
test("a click on a disc still reaches the board's click handler", () => {
  const cells: { x: number; y: number }[] = [];
  const root = render(table(), { onCell: (c) => cells.push(c) });
  disc(root, "tok-gob").dispatchEvent(new MouseEvent("click", { bubbles: true }));
  expect(cells).toHaveLength(1);
});

// VTT-477
test("an ability's line names its outcome by the window's naming and never the compared total", () => {
  const used = create(EnvelopeSchema, {
    sequence: 9n,
    payload: { case: "abilityUsed", value: create(AbilityUsedSchema, {
      actorId: "act-lera", abilityId: "longsword-strike", targetIds: ["act-gob"],
      outcomeSummary: "Longsword Strike on act-gob: hit (17 vs 15)",
      outcomes: [{ targetId: "act-gob", result: "hit", total: 17 }],
    }) },
  });
  const abilities: Ability[] = [
    { id: "longsword-strike", name: "Longsword Strike", range: 1, maxTargets: 1, usage: { kind: "atWill" } },
  ];
  const line = describeEvent(used, namer(table(), [used], false, abilities));
  expect(line).toBe("Lera used Longsword Strike on a hunched goblin: hit, rolled 17");
  expect(line).not.toContain("15");
  expect(line).not.toContain("act-gob");
  const root = render(table(), { abilities }, [used]);
  expect(root.querySelector(".feed .mechanical")?.textContent).toBe(line);
});

// VTT-475 VTT-484
test("the target buttons are labelled by the window's naming, never by an id", () => {
  const reach: Ability = { id: "reach", name: "Reach", range: 5, maxTargets: 1, usage: { kind: "atWill" } };
  const labels = (role: Me["role"]) => {
    const me: Me = { participantId: "p-me", name: "Me", role };
    const ui = { selectedActorId: "act-lera", selectedAbilityId: "reach", doorsArmed: false };
    const sent: ClientCommand[] = [];
    const node = renderPlayerPanel(table(), me, [reach], ui, async (c) => { sent.push(c); return ok(); }, () => {});
    return Array.from(node.querySelectorAll('[data-action="use-ability"]'), (b) => b.textContent);
  };
  expect(labels("player")).toEqual(["Lera", "a hunched goblin", UNNAMED]);
  expect(labels("dm")).toEqual(["Lera", "Goblin Cutter", UNNAMED]);
  expect(labels("agent")).toEqual(["Lera", "Goblin Cutter", UNNAMED]);
});

// VTT-474
test("a disc's card opens toward the board's middle, so the board's edge cannot clip it", () => {
  const st = table();
  st.Tokens["tok-low"] = { ID: "tok-low", SceneID: "s1", ActorID: "act-rat", X: 1, Y: 2 };
  const root = render(st);
  const sides = (id: string) => [
    disc(root, id).classList.contains("card-left"),
    disc(root, id).classList.contains("card-up"),
  ];
  expect(sides("t-lera")).toEqual([false, false]);
  expect(sides("tok-gob")).toEqual([true, false]);
  expect(sides("tok-low")).toEqual([false, true]);
});
