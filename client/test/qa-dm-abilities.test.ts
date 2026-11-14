import { GlobalRegistrator } from "@happy-dom/global-registrator";
if (typeof document === "undefined") GlobalRegistrator.register();

import { expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { ActorKind, EnvelopeSchema } from "../../contract/gen/ts/vtt/v1/events_pb";
import { CommandResultSchema } from "../../contract/gen/ts/vtt/v1/commands_pb";
import type { ClientCommand, CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { fold } from "../src/fold";
import { renderDMConsole } from "../src/view/dm";
import type { DMDeps } from "../src/view/dm";
import type { Ability } from "../src/metadata";

const ability = (id: string, name: string): Ability =>
  ({ id, name, range: 1, maxTargets: 1, usage: { kind: "atWill" } }) as unknown as Ability;

const LIST: Ability[] = [ability("x", "Ex"), ability("y", "Why"), ability("z", "Zed"), ability("w", "")];

let seq = 0;
const env = (payload: unknown) =>
  create(EnvelopeSchema, { eventId: `qa-${++seq}`, sequence: BigInt(seq), payload: payload as never });

const added = (actorId: string, name: string, abilityIds: string[]) =>
  env({ case: "actorAdded", value: { actor: { actorId, name, kind: ActorKind.PARTY_MEMBER, abilityIds } } });

interface Harness {
  deps: DMDeps;
  sent: ClientCommand[];
  notes: string[];
  rerenders: number;
  pending: Array<(r: CommandResult) => void>;
}

function harness(events: unknown[], abilities: readonly Ability[] = LIST, hold = false): Harness {
  const h: Harness = { sent: [], notes: [], rerenders: 0, pending: [], deps: undefined as unknown as DMDeps };
  h.deps = {
    st: fold(events as never),
    participants: [],
    adventures: [],
    maps: [],
    abilities,
    guideFor: async () => null,
    joinLink: null,
    roster: null,
    origin: "http://localhost",
    refreshSharing: () => {},
    send: (c: ClientCommand) => {
      h.sent.push(c);
      if (!hold) return Promise.resolve(create(CommandResultSchema, { ok: true }));
      return new Promise<CommandResult>((resolve) => h.pending.push(resolve));
    },
    rerender: () => {
      h.rerenders++;
    },
    notify: (m: string) => {
      h.notes.push(m);
    },
    confirm: () => true,
    doorsArmed: false,
    toggleDoors: () => {},
  } as unknown as DMDeps;
  return h;
}

const flush = () => new Promise((r) => setTimeout(r, 0));

function headings(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll("h3")).map((h) => h.textContent ?? "");
}

function abilitiesGroup(el: HTMLElement): HTMLElement | null {
  const h = Array.from(el.querySelectorAll("h3")).find((x) => x.textContent === "Abilities");
  return (h?.parentElement as HTMLElement | null) ?? null;
}

function row(el: HTMLElement, actorId: string): HTMLElement {
  const g = abilitiesGroup(el);
  if (!g) throw new Error("no Abilities group");
  const r = g.querySelector(`[data-actor="${actorId}"]`) as HTMLElement | null;
  if (!r) throw new Error(`no row for ${actorId}`);
  return r;
}

const chips = (r: HTMLElement) => Array.from(r.querySelectorAll('[data-action="take-ability"]')) as HTMLButtonElement[];
const giveButton = (r: HTMLElement) => r.querySelector('[data-action="give-ability"]') as HTMLButtonElement | null;
const giveSelect = (r: HTMLElement) => r.querySelector("select") as HTMLSelectElement | null;

function choose(sel: HTMLSelectElement, value: string) {
  sel.value = value;
  sel.dispatchEvent(new Event("input", { bubbles: true }));
  sel.dispatchEvent(new Event("change", { bubbles: true }));
}

function cmdOf(c: ClientCommand): { case: string | undefined; actorId?: string | undefined; abilityId?: string | undefined } {
  const v = c.command.value as { actorId?: string; abilityId?: string } | undefined;
  return { case: c.command.case, actorId: v?.actorId, abilityId: v?.abilityId };
}

// VTT-730
test("qaDMAb: no Abilities group without an actor; with one it sits after Remove condition and before Who controls what", () => {
  expect(headings(renderDMConsole(harness([]).deps))).not.toContain("Abilities");

  const h = harness([
    added("qa1-a", "Alpha", ["x"]),
    env({ case: "conditionApplied", value: { actorId: "qa1-a", conditionId: "c1", source: "" } }),
  ]);
  const hs = headings(renderDMConsole(h.deps));
  const at = hs.indexOf("Abilities");
  expect(at).toBeGreaterThan(-1);
  expect(hs[at - 1]).toBe("Remove condition");
  expect(hs[at + 1]).toBe("Who controls what");

  const bare = headings(renderDMConsole(harness([added("qa1-b", "", [])]).deps));
  expect(bare).toContain("Abilities");
  expect(bare.indexOf("Abilities")).toBeLessThan(bare.indexOf("Who controls what"));
});

// VTT-730
test("qaDMAb: one row per actor of the world, by id, named by its name else its id", () => {
  const h = harness([added("qa2-c", "Gamma", []), added("qa2-a", "", ["x"]), added("qa2-b", "Beta", ["y"])]);
  const el = renderDMConsole(h.deps);
  const g = abilitiesGroup(el)!;
  const rows = Array.from(g.querySelectorAll("[data-actor]")) as HTMLElement[];
  expect(rows.map((r) => r.dataset.actor)).toEqual(["qa2-a", "qa2-b", "qa2-c"]);
  expect(row(el, "qa2-a").textContent).toContain("qa2-a");
  expect(row(el, "qa2-b").textContent).toContain("Beta");
  expect(row(el, "qa2-c").textContent).toContain("Gamma");
});

// VTT-730
test("qaDMAb: a chip per distinct id the sheet names, in the sheet's order, labelled its display name else its id, with ✕", () => {
  const h = harness([added("qa3-a", "Alpha", ["y", "unlisted", "x", "y", "w", "x"])]);
  const r = row(renderDMConsole(h.deps), "qa3-a");
  const labels = chips(r).map((c) => (c.textContent ?? "").trim());
  expect(labels.length).toBe(4);
  expect(labels[0]).toContain("Why");
  expect(labels[1]).toContain("unlisted");
  expect(labels[2]).toContain("Ex");
  expect(labels[3]).toContain("w");
  for (const l of labels) expect(l).toContain("✕");
});

// VTT-730
test("qaDMAb: the give offers, after a blank option, the list's abilities the sheet does not name, in the list's order", () => {
  const h = harness([added("qa4-a", "Alpha", ["y"]), added("qa4-b", "Beta", ["x", "y", "z", "w"])]);
  const el = renderDMConsole(h.deps);
  const sel = giveSelect(row(el, "qa4-a"));
  expect(sel).not.toBeNull();
  const values = Array.from(sel!.options).map((o) => o.value);
  expect(values).toEqual(["", "x", "z", "w"]);
  expect(giveButton(row(el, "qa4-a"))).not.toBeNull();

  const full = row(el, "qa4-b");
  expect(giveSelect(full)).toBeNull();
  expect(giveButton(full)).toBeNull();
  expect(chips(full).length).toBe(4);
});

// VTT-730
test("qaDMAb: with an empty ability list the console offers no give and still offers each take by its id", () => {
  const h = harness([added("qa5-a", "Alpha", ["x", "q"])], []);
  const r = row(renderDMConsole(h.deps), "qa5-a");
  expect(giveSelect(r)).toBeNull();
  expect(giveButton(r)).toBeNull();
  const labels = chips(r).map((c) => (c.textContent ?? "").trim());
  expect(labels.length).toBe(2);
  expect(labels[0]).toContain("x");
  expect(labels[1]).toContain("q");
});

// VTT-730
test("qaDMAb: ✕ sends take_ability naming the actor and that ability", async () => {
  const h = harness([added("qa6-a", "Alpha", ["x", "y"]), added("qa6-b", "Beta", ["x"])]);
  const el = renderDMConsole(h.deps);
  const second = chips(row(el, "qa6-a")).find((c) => (c.textContent ?? "").includes("Why"))!;
  second.click();
  await flush();
  expect(h.sent.map(cmdOf)).toEqual([{ case: "takeAbility", actorId: "qa6-a", abilityId: "y" }]);
  chips(row(el, "qa6-b"))[0]!.click();
  await flush();
  expect(h.sent.map(cmdOf)[1]).toEqual({ case: "takeAbility", actorId: "qa6-b", abilityId: "x" });
  expect(h.notes).toEqual([]);
});

// VTT-730
test("qaDMAb: Give sends give_ability with the chosen ability for that row's actor", async () => {
  const h = harness([added("qa7-a", "Alpha", ["y"]), added("qa7-b", "Beta", [])]);
  const el = renderDMConsole(h.deps);
  choose(giveSelect(row(el, "qa7-b"))!, "z");
  giveButton(row(el, "qa7-b"))!.click();
  await flush();
  expect(h.sent.map(cmdOf)).toEqual([{ case: "giveAbility", actorId: "qa7-b", abilityId: "z" }]);
  expect(h.notes).toEqual([]);
});

// VTT-730
test("qaDMAb: Give with nothing chosen tells the DM to choose and sends nothing", async () => {
  const h = harness([added("qa8-a", "Alpha", [])]);
  const el = renderDMConsole(h.deps);
  giveButton(row(el, "qa8-a"))!.click();
  await flush();
  expect(h.sent).toEqual([]);
  expect(h.notes).toEqual(["Choose an ability to give first."]);
});

// VTT-730
test("qaDMAb: the chosen ability survives a re-render, per actor", () => {
  const h = harness([added("qa9-a", "Alpha", []), added("qa9-b", "Beta", [])]);
  const el = renderDMConsole(h.deps);
  choose(giveSelect(row(el, "qa9-a"))!, "y");
  const again = renderDMConsole(h.deps);
  expect(giveSelect(row(again, "qa9-a"))!.value).toBe("y");
  expect(giveSelect(row(again, "qa9-b"))!.value).toBe("");
  choose(giveSelect(row(again, "qa9-a"))!, "");
});

// VTT-730
test("qaDMAb: an unanswered give or take holds back only itself: keyed per actor and per ability", async () => {
  const h = harness([added("qa10-a", "Alpha", ["x", "y"]), added("qa10-b", "Beta", ["x"])], LIST, true);
  const el = renderDMConsole(h.deps);
  const take = (actor: string, label: string) =>
    chips(row(el, actor)).find((c) => (c.textContent ?? "").includes(label))!.click();

  take("qa10-a", "Ex");
  await flush();
  take("qa10-a", "Ex");
  await flush();
  expect(h.sent.length).toBe(1);

  take("qa10-a", "Why");
  take("qa10-b", "Ex");
  await flush();
  expect(h.sent.length).toBe(3);

  choose(giveSelect(row(el, "qa10-a"))!, "z");
  giveButton(row(el, "qa10-a"))!.click();
  await flush();
  giveButton(row(el, "qa10-a"))!.click();
  await flush();
  expect(h.sent.length).toBe(4);

  choose(giveSelect(row(el, "qa10-b"))!, "y");
  giveButton(row(el, "qa10-b"))!.click();
  await flush();
  expect(h.sent.map(cmdOf)).toEqual([
    { case: "takeAbility", actorId: "qa10-a", abilityId: "x" },
    { case: "takeAbility", actorId: "qa10-a", abilityId: "y" },
    { case: "takeAbility", actorId: "qa10-b", abilityId: "x" },
    { case: "giveAbility", actorId: "qa10-a", abilityId: "z" },
    { case: "giveAbility", actorId: "qa10-b", abilityId: "y" },
  ]);
  expect(h.rerenders).toBe(0);

  for (const r of h.pending.splice(0)) r(create(CommandResultSchema, { ok: true }));
  await flush();

  take("qa10-a", "Ex");
  await flush();
  expect(h.sent.length).toBe(6);
  for (const r of h.pending.splice(0)) r(create(CommandResultSchema, { ok: true }));
  await flush();
});

// VTT-730
test("qaDMAb: an accepted give clears its choice and asks for a repaint; a refused one keeps it and asks for none", async () => {
  const h = harness([added("qa11-a", "Alpha", []), added("qa11-b", "Beta", [])], LIST, true);
  let el = renderDMConsole(h.deps);
  choose(giveSelect(row(el, "qa11-a"))!, "x");
  choose(giveSelect(row(el, "qa11-b"))!, "z");

  giveButton(row(el, "qa11-b"))!.click();
  await flush();
  h.pending.shift()!(create(CommandResultSchema, { ok: false, error: "refused" }));
  await flush();
  expect(h.rerenders).toBe(0);
  el = renderDMConsole(h.deps);
  expect(giveSelect(row(el, "qa11-b"))!.value).toBe("z");

  giveButton(row(el, "qa11-a"))!.click();
  await flush();
  h.pending.shift()!(create(CommandResultSchema, { ok: true }));
  await flush();
  expect(h.rerenders).toBe(1);
  el = renderDMConsole(h.deps);
  expect(giveSelect(row(el, "qa11-a"))!.value).toBe("");
  expect(giveSelect(row(el, "qa11-b"))!.value).toBe("z");
  choose(giveSelect(row(el, "qa11-b"))!, "");
});

// VTT-717, VTT-720, VTT-721, VTT-722
test("qaDMAb: the client's fold refuses a give or a take the gateway refuses", () => {
  const base = () => [added("qa12-a", "Alpha", ["x", "y", "x"])];
  const given = (actorId: string, abilityId: string) => env({ case: "abilityGiven", value: { actorId, abilityId } });
  const taken = (actorId: string, abilityId: string) => env({ case: "abilityTaken", value: { actorId, abilityId } });

  expect(() => fold([...base(), given("nobody", "z")] as never)).toThrow();
  expect(() => fold([...base(), taken("nobody", "x")] as never)).toThrow();
  expect(() => fold([...base(), given("qa12-a", "x")] as never)).toThrow();
  expect(() => fold([...base(), taken("qa12-a", "z")] as never)).toThrow();
  expect(() => fold([...base(), given("qa12-a", "")] as never)).toThrow();

  const ok = fold([...base(), given("qa12-a", "z"), taken("qa12-a", "x")] as never);
  expect(ok.Actors["qa12-a"]!.abilityIds).toEqual(["y", "z"]);
});

// VTT-730
test("qaDMAb: the Abilities group's controls follow SPEC-022's data-field rule", () => {
  const h = harness([added("qa13-a", "Alpha", ["x"])]);
  const g = abilitiesGroup(renderDMConsole(h.deps))!;
  for (const el of Array.from(g.querySelectorAll("select, button"))) {
    expect({ tag: el.tagName, field: el.getAttribute("data-field") }).toEqual({ tag: el.tagName, field: null });
  }
});
