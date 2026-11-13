import "./support/dom";
import { describe, expect, test } from "bun:test";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import type { Ability, Me } from "../src/metadata";
import { offered } from "../src/player";
import { newState, type Actor, type State } from "../src/state";
import { playerClickFor, renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { ok, settle } from "./support/answers";

const qaWinList: Ability[] = [
  { id: "alpha", name: "Arc Bolt", range: 6, maxTargets: 1, usage: { kind: "resource", resource: "hp", cost: 1 } },
  { id: "bravo", name: "Bash", range: 2, maxTargets: 1, usage: { kind: "atWill" } },
  { id: "charlie", name: "Cleave", range: 2, maxTargets: 1, usage: { kind: "atWill" } },
  { id: "delta", name: "Dart", range: 6, maxTargets: 1, usage: { kind: "atWill" } },
];

function qaWinActor(actorId: string, name: string, controllerIds: string[], abilityIds: string[], kind = ActorKind.PARTY_MEMBER): Actor {
  return {
    actorId, name, moduleId: "qa", attributes: {}, resources: { hp: { current: 9, max: 9 } },
    controllerId: "", controllerIds, kind, abilityIds,
  };
}

function qaWinState(actors: Actor[]): State {
  const st = newState();
  st.Scenes["s1"] = { ID: "s1", Name: "Hall", GridWidth: 8, GridHeight: 8 };
  actors.forEach((a, i) => {
    st.Actors[a.actorId] = a;
    st.Tokens["t-" + a.actorId] = { ID: "t-" + a.actorId, SceneID: "s1", ActorID: a.actorId, X: 1 + i, Y: 1, Name: a.name };
  });
  return st;
}

const qaWinMe: Record<"player" | "dm" | "agent", Me> = {
  player: { participantId: "p1", name: "Pia", role: "player" },
  dm: { participantId: "dm1", name: "Dora", role: "dm" },
  agent: { participantId: "ag1", name: "Agent", role: "agent" },
};

function qaWinUI(selectedActorId: string, selectedAbilityId = ""): PlayerUIState {
  return { selectedActorId, selectedAbilityId, doorsArmed: false };
}

function qaWinPanel(st: State, me: Me, ui: PlayerUIState, list: Ability[] = qaWinList): { node: HTMLElement; sent: ClientCommand[] } {
  const sent: ClientCommand[] = [];
  const send = async (c: ClientCommand): Promise<CommandResult> => {
    sent.push(c);
    return ok();
  };
  const node = renderPlayerPanel(st, me, list, ui, send, () => {});
  return { node, sent };
}

function qaWinHeading(node: HTMLElement, text: string): HTMLElement | undefined {
  return Array.from(node.querySelectorAll("h3")).find((h) => h.textContent === text);
}

function qaWinHeadings(node: HTMLElement): string[] {
  return Array.from(node.querySelectorAll("h3")).map((h) => h.textContent ?? "");
}

function qaWinAbilityButtons(node: HTMLElement): HTMLButtonElement[] {
  const row = qaWinHeading(node, "Abilities")?.nextElementSibling;
  if (!row) return [];
  return Array.from(row.querySelectorAll("button"));
}

function qaWinAbilityNames(node: HTMLElement): string[] {
  return qaWinAbilityButtons(node).map((b) => b.textContent ?? "");
}

function qaWinEveryButtonText(node: HTMLElement): string[] {
  return Array.from(node.querySelectorAll("button")).map((b) => b.textContent ?? "");
}

function qaWinUses(sent: ClientCommand[]): { actorId: string; abilityId: string; targetIds: string[] }[] {
  return sent.flatMap((c) => (c.command.case === "useAbility"
    ? [{ actorId: c.command.value.actorId, abilityId: c.command.value.abilityId, targetIds: [...c.command.value.targetIds] }]
    : []));
}

function qaWinActorFor(role: "player" | "dm" | "agent", abilityIds: string[]): Actor {
  return qaWinActor("pc-1", "Ayla", [qaWinMe[role].participantId], abilityIds);
}

describe("offered", () => {
  // VTT-695
  test("lists the entries the sheet names in the list's order, not the sheet's", () => {
    const actor = qaWinActor("pc-1", "Ayla", ["p1"], ["delta", "bravo"]);
    expect(offered(qaWinList, actor).map((a) => a.id)).toEqual(["bravo", "delta"]);
  });

  // VTT-695
  test("hands back the list's own entries, whole", () => {
    const actor = qaWinActor("pc-1", "Ayla", ["p1"], ["delta", "alpha"]);
    expect(offered(qaWinList, actor)).toEqual([qaWinList[0]!, qaWinList[3]!]);
  });

  // VTT-695
  test("ignores an id the list does not hold", () => {
    const actor = qaWinActor("pc-1", "Ayla", ["p1"], ["ghost", "charlie", "phantom"]);
    expect(offered(qaWinList, actor).map((a) => a.id)).toEqual(["charlie"]);
    const onlyUnknown = qaWinActor("pc-2", "Bo", ["p1"], ["ghost", "phantom"]);
    expect(offered(qaWinList, onlyUnknown)).toEqual([]);
  });

  // VTT-695
  test("offers nothing for an empty sheet", () => {
    expect(offered(qaWinList, qaWinActor("pc-1", "Ayla", ["p1"], []))).toEqual([]);
  });

  // VTT-695
  test("offers nothing from an empty list, whatever the sheet names", () => {
    expect(offered([], qaWinActor("pc-1", "Ayla", ["p1"], ["alpha", "bravo"]))).toEqual([]);
  });

  // VTT-695
  test("offers each entry once when the sheet names it twice", () => {
    const actor = qaWinActor("pc-1", "Ayla", ["p1"], ["delta", "bravo", "delta"]);
    expect(offered(qaWinList, actor).map((a) => a.id)).toEqual(["bravo", "delta"]);
  });
});

describe("the panel offers what the selected actor's sheet names", () => {
  for (const role of ["player", "dm", "agent"] as const) {
    // VTT-695
    test(`a ${role}'s panel lists one button per named ability, in the list's order`, () => {
      const st = qaWinState([qaWinActorFor(role, ["delta", "ghost", "bravo"])]);
      const { node } = qaWinPanel(st, qaWinMe[role], qaWinUI("pc-1"));
      expect(qaWinAbilityNames(node)).toEqual(["Bash", "Dart"]);
      const all = qaWinEveryButtonText(node);
      expect(all).not.toContain("Arc Bolt");
      expect(all).not.toContain("Cleave");
      expect(all).not.toContain("ghost");
    });

    // VTT-695
    test(`a ${role}'s panel offers no ability for an empty sheet or one the list names none of`, () => {
      for (const sheet of [[], ["ghost"]]) {
        const { node } = qaWinPanel(qaWinState([qaWinActorFor(role, sheet)]), qaWinMe[role], qaWinUI("pc-1"));
        expect(qaWinAbilityNames(node)).toEqual([]);
        for (const a of qaWinList) expect(qaWinEveryButtonText(node)).not.toContain(a.name);
      }
    });

    // Keep the Abilities heading off a panel that offers nothing.
    test(`a ${role}'s panel shows no Abilities heading when it offers nothing`, () => {
      for (const sheet of [[], ["ghost"]]) {
        const { node } = qaWinPanel(qaWinState([qaWinActorFor(role, sheet)]), qaWinMe[role], qaWinUI("pc-1"));
        expect(qaWinHeadings(node)).not.toContain("Abilities");
      }
      const fromEmptyList = qaWinPanel(qaWinState([qaWinActorFor(role, ["bravo"])]), qaWinMe[role], qaWinUI("pc-1"), []);
      expect(qaWinHeadings(fromEmptyList.node)).not.toContain("Abilities");
    });

    // Keep arming on what the panel offers: its target sends the armed ability.
    test(`a ${role}'s click on a named ability arms it, and its target sends that ability`, async () => {
      const gob = qaWinActor("gob", "Gob", [], [], ActorKind.NON_PARTY);
      const st = qaWinState([qaWinActorFor(role, ["delta", "bravo"]), gob]);
      const ui = qaWinUI("pc-1");
      const first = qaWinPanel(st, qaWinMe[role], ui);
      const dart = qaWinAbilityButtons(first.node).find((b) => b.textContent === "Dart");
      expect(dart).toBeDefined();
      dart!.click();
      expect(ui.selectedAbilityId).toBe("delta");
      const armed = qaWinPanel(st, qaWinMe[role], ui);
      expect(qaWinHeadings(armed.node)).toContain("Targets (range 6)");
      const targets = Array.from(armed.node.querySelectorAll<HTMLButtonElement>('[data-action="use-ability"]'));
      const onGob = targets.find((b) => b.textContent === "Gob");
      expect(onGob).toBeDefined();
      onGob!.click();
      await settle();
      expect(qaWinUses(armed.sent)).toEqual([{ actorId: "pc-1", abilityId: "delta", targetIds: ["gob"] }]);
    });

    // Keep an armed ability the sheet does not name unarmed: no target, no send.
    test(`a ${role}'s carried-over ability the sheet does not name arms nothing and sends nothing`, async () => {
      const gob = qaWinActor("gob", "Gob", [], [], ActorKind.NON_PARTY);
      for (const stale of ["charlie", "alpha", "ghost"]) {
        const st = qaWinState([qaWinActorFor(role, ["bravo", "ghost"]), gob]);
        const { node, sent } = qaWinPanel(st, qaWinMe[role], qaWinUI("pc-1", stale));
        expect(qaWinAbilityNames(node)).toEqual(["Bash"]);
        expect(qaWinAbilityButtons(node).some((b) => b.classList.contains("sel"))).toBe(false);
        expect(qaWinHeadings(node).filter((h) => h.startsWith("Targets"))).toEqual([]);
        expect(node.querySelectorAll('[data-action="use-ability"]').length).toBe(0);
        for (const b of Array.from(node.querySelectorAll<HTMLButtonElement>("button"))) {
          if (b.dataset["action"] !== "add-narration") b.click();
        }
        await settle();
        expect(qaWinUses(sent)).toEqual([]);
      }
    });
  }

  // VTT-695
  test("a DM's panel lists the abilities of the chip selected, and only its", () => {
    const st = qaWinState([
      qaWinActor("a", "Ayla", ["dm1"], ["bravo"]),
      qaWinActor("b", "Bo", ["dm1"], ["charlie", "alpha"], ActorKind.NON_PARTY),
    ]);
    const ui = qaWinUI("a");
    const onA = qaWinPanel(st, qaWinMe.dm, ui);
    expect(qaWinAbilityNames(onA.node)).toEqual(["Bash"]);
    const boChip = Array.from(onA.node.querySelectorAll("button")).find((b) => b.textContent === "Bo");
    expect(boChip).toBeDefined();
    boChip!.click();
    const onB = qaWinPanel(st, qaWinMe.dm, ui);
    expect(qaWinAbilityNames(onB.node)).toEqual(["Arc Bolt", "Cleave"]);
  });

  // VTT-695
  test("a player's panel lists the abilities of the actor it acts as, not another they control", () => {
    const st = qaWinState([
      qaWinActor("a", "Ayla", ["p1"], ["bravo"]),
      qaWinActor("b", "Bo", ["p1"], ["delta", "charlie"]),
    ]);
    const switcher = { actors: [{ actorId: "a", name: "Ayla" }, { actorId: "b", name: "Bo" }], onSwitch: () => {} };
    const node = renderPlayerPanel(st, qaWinMe.player, qaWinList, qaWinUI("b"), async () => ok(), () => {}, switcher);
    expect(qaWinAbilityNames(node)).toEqual(["Cleave", "Dart"]);
  });

  // Keep a board click under an unoffered armed ability moving, as with none armed.
  test("a player's board click with a carried-over ability the sheet does not name moves the watched token", () => {
    const st = qaWinState([qaWinActor("pc-1", "Ayla", ["p1"], ["bravo"])]);
    const plain = playerClickFor(st, qaWinMe.player, qaWinUI("pc-1"), ["pc-1"], { x: 5, y: 5 });
    expect(plain?.command.case).toBe("moveToken");
    const stale = playerClickFor(st, qaWinMe.player, qaWinUI("pc-1", "charlie"), ["pc-1"], { x: 5, y: 5 });
    expect(stale?.command.case).toBe("moveToken");
  });
});
