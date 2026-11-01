import "./support/dom";
import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { CommandResultSchema, type CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { removeParticipant } from "../src/commands";
import { newState } from "../src/state";
import type { Roster } from "../src/metadata";
import tools from "../../contract/gen/tools/tools.json";

const ROSTER: Roster[] = [
  { participantId: "id-dm-1", name: "Dana", role: "dm" },
  { participantId: "id-agent-2", name: "Argo", role: "agent" },
  { participantId: "id-player-3", name: "Pippa", role: "player" },
  { participantId: "id-spectator-4", name: "Solveig", role: "spectator" },
  { participantId: "id-player-5", name: "Quentin", role: "player" },
];

interface Probe {
  sent: unknown[];
  confirms: string[];
  refreshes: number;
  settle: (r?: CommandResult) => void;
}

function mount(answer: boolean, roster: Roster[] = ROSTER): { root: HTMLElement; p: Probe } {
  let resolve: (r: CommandResult) => void = () => {};
  const pending = new Promise<CommandResult>((r) => (resolve = r));
  const p: Probe = {
    sent: [],
    confirms: [],
    refreshes: 0,
    settle: (r) => resolve(r ?? create(CommandResultSchema, { ok: true })),
  };
  const d: DMDeps = {
    st: newState(),
    participants: [],
    adventures: [],
    maps: [],
    guideFor: async () => null,
    joinLink: null,
    roster,
    origin: "http://localhost",
    refreshSharing: () => {
      p.refreshes++;
    },
    send: (c) => {
      p.sent.push(c);
      return pending;
    },
    rerender: () => {},
    notify: () => {},
    confirm: (m) => {
      p.confirms.push(m);
      return answer;
    },
    doorsArmed: false,
    toggleDoors: () => {},
  };
  const root = renderDMConsole(d);
  document.body.appendChild(root);
  return { root, p };
}

function bootButtons(root: HTMLElement): HTMLButtonElement[] {
  return Array.from(root.querySelectorAll("button")).filter((b) => b.textContent?.trim() === "Boot");
}

function rowOf(btn: HTMLElement, roster: Roster[] = ROSTER): Roster {
  let el: HTMLElement | null = btn.parentElement;
  while (el) {
    const hits = roster.filter((r) => (el!.textContent ?? "").includes(r.name));
    if (hits.length === 1) return hits[0]!;
    if (hits.length > 1) throw new Error("climbed past the row");
    el = el.parentElement;
  }
  throw new Error("no row");
}

function bootFor(root: HTMLElement, id: string): HTMLButtonElement {
  const b = bootButtons(root).find((x) => rowOf(x).participantId === id);
  if (!b) throw new Error("no Boot for " + id);
  return b;
}

function idOf(c: ReturnType<typeof removeParticipant>): string | undefined {
  return c.command.case === "removeParticipant" ? c.command.value.participantId : undefined;
}

const tick = () => new Promise((r) => setTimeout(r, 0));

describe("QA boot", () => {
  // VTT-568
  test("TestQABootIsOnPlayerAndSpectatorRowsOnly", () => {
    const { root } = mount(true);
    const roles = bootButtons(root).map((b) => rowOf(b).role).sort();
    expect(roles).toEqual(["player", "player", "spectator"]);
  });

  // VTT-568
  test("TestQABootAbsentWhenRosterHoldsOnlyDmAndAgent", () => {
    const { root } = mount(true, ROSTER.slice(0, 2));
    expect(bootButtons(root).length).toBe(0);
    const again = mount(true);
    expect(bootButtons(again.root).length).toBeGreaterThan(0);
  });

  // VTT-569
  test("TestQABootConfirmMessageHoldsTheDisplayName", () => {
    const { root, p } = mount(false);
    bootFor(root, "id-player-5").click();
    expect(p.confirms.length).toBe(1);
    expect(p.confirms[0]).toContain("Quentin");
    expect(p.confirms[0]).not.toContain("Pippa");
  });

  // VTT-569
  test("TestQABootCancelSendsNothing", async () => {
    const cancelled = mount(false);
    bootFor(cancelled.root, "id-player-3").click();
    cancelled.p.settle();
    await tick();
    expect(cancelled.p.confirms.length).toBe(1);
    expect(cancelled.p.sent.length).toBe(0);
    expect(cancelled.p.refreshes).toBe(0);
    const confirmed = mount(true);
    bootFor(confirmed.root, "id-player-3").click();
    expect(confirmed.p.sent.length).toBe(1);
  });

  // VTT-570
  test("TestQAConfirmedBootSendsOneRemovalOfItsRowsParticipant", () => {
    const { root, p } = mount(true);
    bootFor(root, "id-spectator-4").click();
    expect(p.sent.length).toBe(1);
    const c = p.sent[0] as ReturnType<typeof removeParticipant>;
    expect(c.command.case).toBe("removeParticipant");
    expect(idOf(c)).toBe("id-spectator-4");
    const second = mount(true);
    bootFor(second.root, "id-player-5").click();
    const c2 = second.p.sent[0] as ReturnType<typeof removeParticipant>;
    expect(idOf(c2)).toBe("id-player-5");
  });

  test("TestQARemoveParticipantBuilderCarriesTheId", () => {
    const c = removeParticipant("abc-123");
    expect(c.command.case).toBe("removeParticipant");
    expect(idOf(c)).toBe("abc-123");
  });

  // VTT-571
  test("TestQARefreshComesAfterTheAnswerNotTheSend", async () => {
    const { root, p } = mount(true);
    bootFor(root, "id-player-3").click();
    await tick();
    expect(p.sent.length).toBe(1);
    expect(p.refreshes).toBe(0);
    p.settle();
    await tick();
    expect(p.refreshes).toBe(1);
  });

  // VTT-567
  test("TestQAToolDescriptionSaysIrreversibleAndRevokesControl", () => {
    const t = (tools as { name: string; description: string }[]).find((x) => x.name === "remove_participant");
    expect(t).toBeDefined();
    const desc = t!.description;
    expect(desc).toMatch(/cannot be undone/i);
    expect(desc).toMatch(/revokes their control of every actor they control/i);
  });
});
