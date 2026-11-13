import "./support/dom";
import { expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  AbilityUsedSchema, ActorKind, EnvelopeSchema, ResourceChangedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Ability, Me, Property } from "../src/metadata";
import { newState, type Resource, type State } from "../src/state";
import { tokensOnScene } from "../src/view/grid";
import { namer } from "../src/view/naming";
import { renderPlayerPanel } from "../src/view/player";
import { describe as qaPNLine, renderSpectator } from "../src/view/spectator";
import { ok } from "./support/answers";

const ARIA = "act-aria-01";
const GRUB = "act-grub-02";
const BRAN = "act-bran-03";
const SCENE = "scn-cellar";

const QA_PN_PROPS: Property[] = [
  { key: "hp", type: "resource", name: "Vigor", description: "How hard to fell" },
  { key: "mana", type: "resource", name: "Arcana", description: "Spell fuel" },
];

const QA_PN_NUMBER_PROPS: Property[] = [
  { key: "hp", type: "number", name: "Health", description: "A plain number" },
];

const QA_PN_ABILITIES: Ability[] = [
  {
    id: "ab-zap", name: "Zap", range: 5, maxTargets: 2,
    usage: { kind: "resource", resource: "mana", cost: 2 },
  },
  {
    id: "ab-focus", name: "Focus Shot", range: 5, maxTargets: 1,
    usage: { kind: "resource", resource: "focus", cost: 3 },
  },
  {
    id: "ab-hp", name: "Blood Rite", range: 5, maxTargets: 1,
    usage: { kind: "resource", resource: "hp", cost: 4 },
  },
  { id: "ab-poke", name: "Poke", range: 5, maxTargets: 1, usage: { kind: "atWill" } },
];

const QA_PN_ME: Me = { participantId: "p-pat", name: "Pat", role: "player" };

function qaPNActor(
  id: string, name: string, kind: ActorKind,
  resources: Record<string, Resource>, controllers: string[],
): State["Actors"][string] {
  return {
    actorId: id, name, moduleId: "mod", attributes: { ac: 17 }, resources,
    controllerId: controllers[0] ?? "", controllerIds: controllers, kind, abilityIds: [],
  };
}

function qaPNState(): State {
  const st = newState();
  st.Scenes[SCENE] = { ID: SCENE, Name: "Cellar", GridWidth: 6, GridHeight: 6 };
  st.Actors[ARIA] = qaPNActor(ARIA, "Aria", ActorKind.PARTY_MEMBER, {
    mana: { current: 1, max: 4 },
    hp: { current: 3, max: 10 },
    focus: { current: 2, max: 5 },
  }, ["p-pat"]);
  st.Actors[GRUB] = qaPNActor(GRUB, "Grub", ActorKind.NON_PARTY, {
    hp: { current: 5, max: 8 },
  }, []);
  st.Actors[BRAN] = qaPNActor(BRAN, "Bran", ActorKind.PARTY_MEMBER, {
    mana: { current: 4, max: 4 },
    hp: { current: 9, max: 9 },
    focus: { current: 3, max: 5 },
  }, ["p-pat"]);
  st.Tokens["tok-aria-01"] = {
    ID: "tok-aria-01", SceneID: SCENE, ActorID: ARIA, X: 1, Y: 1,
  };
  st.Tokens["tok-grub-02"] = {
    ID: "tok-grub-02", SceneID: SCENE, ActorID: GRUB, X: 2, Y: 1,
  };
  st.Tokens["tok-bran-03"] = {
    ID: "tok-bran-03", SceneID: SCENE, ActorID: BRAN, X: 3, Y: 1, Name: "Wren",
  };
  return st;
}

function qaPNChange(actorId: string, resource: string, newValue: number): Envelope {
  return create(EnvelopeSchema, {
    eventId: `ev-${actorId}-${resource}`, sequence: 1n,
    payload: {
      case: "resourceChanged",
      value: create(ResourceChangedSchema, { actorId, resource, delta: -2, newValue }),
    },
  });
}

function qaPNBoard(
  st: State, log: Envelope[], properties: Property[] | undefined, dm = false,
): HTMLElement {
  const root = document.createElement("div");
  renderSpectator(root, st, log, "ok", {
    properties, abilities: QA_PN_ABILITIES, dm,
  });
  return root;
}

function qaPNChips(root: HTMLElement, tokenId: string): Element[] {
  const sel = `.token[data-token-id="${tokenId}"] .chip`;
  return Array.from(root.querySelectorAll(sel));
}

function qaPNChipTitles(root: HTMLElement, tokenId: string): (string | null)[] {
  return qaPNChips(root, tokenId).map((c) => c.getAttribute("title"));
}

function qaPNStory(root: HTMLElement): string[] {
  const lines = Array.from(root.querySelectorAll(".feed .mechanical"));
  return lines.map((p) => (p.textContent ?? "").trim());
}

function qaPNPanel(
  st: State, actorId: string, properties: Property[] | undefined,
  abilityId = "",
): HTMLElement {
  return renderPlayerPanel(
    st, QA_PN_ME, QA_PN_ABILITIES,
    { selectedActorId: actorId, selectedAbilityId: abilityId, doorsArmed: false },
    async () => ok(), () => {}, undefined, undefined, properties,
  );
}

function qaPNCost(panel: HTMLElement, abilityName: string): string | null {
  const buttons = Array.from(panel.querySelectorAll("button"));
  const b = buttons.find((x) => (x.textContent ?? "").trim() === abilityName);
  if (b === undefined) throw new Error(`no ability button ${abilityName}`);
  return b.getAttribute("title");
}

function qaPNStoryOf(e: Envelope, properties: Property[] | undefined): string {
  const st = qaPNState();
  const fromDescribe = qaPNLine(e, namer(st, [e], false, QA_PN_ABILITIES, properties));
  const rendered = qaPNStory(qaPNBoard(st, [e], properties));
  expect(rendered).toEqual([fromDescribe]);
  return fromDescribe;
}

// VTT-622
test("namer names a declared resource by the property's display name", () => {
  const n = namer(qaPNState(), [], false, QA_PN_ABILITIES, QA_PN_PROPS);
  expect(n.resource("hp")).toBe("Vigor");
  expect(n.resource("mana")).toBe("Arcana");
});

// VTT-622
test("a DM window's namer names a declared resource the same way", () => {
  const n = namer(qaPNState(), [], true, QA_PN_ABILITIES, QA_PN_PROPS);
  expect(n.resource("mana")).toBe("Arcana");
});

// VTT-623
test("namer names an undeclared resource by its key", () => {
  const n = namer(qaPNState(), [], false, QA_PN_ABILITIES, QA_PN_PROPS);
  expect(n.resource("focus")).toBe("focus");
});

// VTT-623
test("namer with no properties names every resource by its key", () => {
  const without = namer(qaPNState(), [], false, QA_PN_ABILITIES);
  const empty = namer(qaPNState(), [], false, QA_PN_ABILITIES, []);
  for (const key of ["hp", "mana", "focus"]) {
    expect(without.resource(key)).toBe(key);
    expect(empty.resource(key)).toBe(key);
  }
});

// VTT-622
test("a declared resource's chip is titled by its name, face current/max", () => {
  const root = qaPNBoard(qaPNState(), [], QA_PN_PROPS);
  const chips = qaPNChips(root, "tok-aria-01");
  const hp = chips.find((c) => c.getAttribute("title") === "Vigor");
  expect(hp?.textContent?.trim()).toBe("3/10");
  const mana = chips.find((c) => c.getAttribute("title") === "Arcana");
  expect(mana?.textContent?.trim()).toBe("1/4");
  expect(qaPNChipTitles(root, "tok-grub-02")).toEqual(["Vigor"]);
});

// VTT-622
test("tokensOnScene labels a declared resource's chip by its name", () => {
  const st = qaPNState();
  const n = namer(st, [], false, QA_PN_ABILITIES, QA_PN_PROPS);
  const disc = tokensOnScene(st, SCENE, undefined, n)
    .find((d) => d.tokenId === "tok-aria-01");
  const hp = disc?.resources.find((r) => r.name === "hp");
  expect(hp).toEqual({ name: "hp", label: "Vigor", current: 3, max: 10 });
});

// VTT-622
test("a DM window titles a declared resource's chip by its name", () => {
  const root = qaPNBoard(qaPNState(), [], QA_PN_PROPS, true);
  expect(qaPNChipTitles(root, "tok-bran-03")).toContain("Arcana");
});

// VTT-623
test("an undeclared resource's chip is titled by its key", () => {
  const root = qaPNBoard(qaPNState(), [], QA_PN_PROPS);
  const focus = qaPNChips(root, "tok-aria-01")
    .find((c) => c.getAttribute("title") === "focus");
  expect(focus?.textContent?.trim()).toBe("2/5");
});

// VTT-623
test("with no properties every chip is titled by its key", () => {
  for (const props of [undefined, []]) {
    const root = qaPNBoard(qaPNState(), [], props);
    expect(qaPNChipTitles(root, "tok-aria-01")).toEqual(["focus", "hp", "mana"]);
    expect(qaPNChipTitles(root, "tok-grub-02")).toEqual(["hp"]);
  }
});

test("chips are ordered by key, not by display name or insertion", () => {
  const root = qaPNBoard(qaPNState(), [], QA_PN_PROPS);
  expect(qaPNChipTitles(root, "tok-aria-01")).toEqual(["focus", "Vigor", "Arcana"]);
  const st = qaPNState();
  const n = namer(st, [], false, QA_PN_ABILITIES, QA_PN_PROPS);
  const disc = tokensOnScene(st, SCENE, undefined, n)
    .find((d) => d.tokenId === "tok-aria-01");
  expect(disc?.resources.map((r) => r.name)).toEqual(["focus", "hp", "mana"]);
});

// VTT-622
test("the Story line of a declared resource's change has its whole form", () => {
  const line = qaPNStoryOf(qaPNChange(ARIA, "mana", 7), QA_PN_PROPS);
  expect(line).toMatch(/^Aria\s+Arcana\s+→\s+7$/);
});

// VTT-623
test("the Story line of an undeclared resource's change names its key", () => {
  const line = qaPNStoryOf(qaPNChange(ARIA, "focus", 6), QA_PN_PROPS);
  expect(line).toMatch(/^Aria\s+focus\s+→\s+6$/);
});

// VTT-623
test("with no properties the Story line names a resource by its key", () => {
  expect(qaPNStoryOf(qaPNChange(ARIA, "mana", 7), undefined))
    .toMatch(/^Aria\s+mana\s+→\s+7$/);
  expect(qaPNStoryOf(qaPNChange(ARIA, "hp", 8), []))
    .toMatch(/^Aria\s+hp\s+→\s+8$/);
});

// VTT-475
test("the Story line names the actor by token name, party name, or not", () => {
  const named = qaPNStoryOf(qaPNChange(BRAN, "mana", 7), QA_PN_PROPS);
  expect(named).toMatch(/^Wren\s+Arcana\s+→\s+7$/);
  const unnamed = qaPNStoryOf(qaPNChange(GRUB, "hp", 5), QA_PN_PROPS);
  expect(unnamed).toMatch(/^a creature\s+Vigor\s+→\s+5$/);
  expect(qaPNStoryOf(qaPNChange(ARIA, "hp", 8), QA_PN_PROPS)).toMatch(/^Aria\s+Vigor\s+→\s+8$/);
  for (const line of [named, unnamed]) {
    expect(line).not.toMatch(/act-|tok-|Bran|Grub/);
  }
});

// VTT-622
test("an unaffordable cost names the resource and says not enough", () => {
  const panel = qaPNPanel(qaPNState(), ARIA, QA_PN_PROPS);
  expect(qaPNCost(panel, "Zap")).toBe("Arcana 2 — not enough");
  expect(qaPNCost(panel, "Blood Rite")).toBe("Vigor 4 — not enough");
});

// VTT-622
test("an affordable cost names the resource with no shortfall", () => {
  const panel = qaPNPanel(qaPNState(), BRAN, QA_PN_PROPS);
  expect(qaPNCost(panel, "Zap")).toBe("Arcana 2");
  expect(qaPNCost(panel, "Blood Rite")).toBe("Vigor 4");
});

// VTT-622
test("a cost exactly equal to what the actor holds is affordable", () => {
  const st = qaPNState();
  const bran = st.Actors[BRAN];
  if (bran === undefined) throw new Error("fixture lost Bran");
  bran.resources["mana"] = { current: 2, max: 4 };
  expect(qaPNCost(qaPNPanel(st, BRAN, QA_PN_PROPS), "Zap")).toBe("Arcana 2");
});

test("an undeclared resource's cost is named by its key", () => {
  const st = qaPNState();
  expect(qaPNCost(qaPNPanel(st, ARIA, QA_PN_PROPS), "Focus Shot"))
    .toBe("focus 3 — not enough");
  expect(qaPNCost(qaPNPanel(st, BRAN, QA_PN_PROPS), "Focus Shot")).toBe("focus 3");
});

test("with no properties every cost is named by its key", () => {
  const st = qaPNState();
  for (const props of [undefined, []]) {
    expect(qaPNCost(qaPNPanel(st, ARIA, props), "Zap")).toBe("mana 2 — not enough");
    expect(qaPNCost(qaPNPanel(st, BRAN, props), "Blood Rite")).toBe("hp 4");
  }
});

// VTT-623
test("a number property does not name a resource chip of the same key", () => {
  const root = qaPNBoard(qaPNState(), [], QA_PN_NUMBER_PROPS);
  expect(qaPNChipTitles(root, "tok-grub-02")).toEqual(["hp"]);
});

// VTT-623
test("a number property does not name a Story line of the same key", () => {
  const line = qaPNStoryOf(qaPNChange(ARIA, "hp", 8), QA_PN_NUMBER_PROPS);
  expect(line).toMatch(/^Aria\s+hp\s+→\s+8$/);
});

test("a number property does not name a cost of the same key", () => {
  const panel = qaPNPanel(qaPNState(), BRAN, QA_PN_NUMBER_PROPS);
  expect(qaPNCost(panel, "Blood Rite")).toBe("hp 4");
});

// VTT-475
test("the player's target buttons name tokens, never by id", () => {
  const panel = qaPNPanel(qaPNState(), BRAN, QA_PN_PROPS, "ab-poke");
  const sel = 'button[data-action="use-ability"]';
  const names = Array.from(panel.querySelectorAll(sel))
    .map((b) => (b.textContent ?? "").trim()).sort();
  expect(names).toEqual(["Aria", "Wren", "a creature"]);
});

// VTT-477
test("an ability's Story line names ability, targets, results, totals", () => {
  const e = create(EnvelopeSchema, {
    eventId: "ev-zap", sequence: 1n,
    payload: {
      case: "abilityUsed",
      value: create(AbilityUsedSchema, {
        actorId: BRAN, abilityId: "ab-zap", targetIds: [GRUB, ARIA],
        rolls: [{ expression: "1d20+2", results: [11], total: 99 }],
        outcomeSummary: "13 against 17: hit",
        outcomes: [
          { targetId: GRUB, result: "hit", total: 13 },
          { targetId: ARIA, result: "graze", total: 4 },
        ],
      }),
    },
  });
  const line = qaPNStoryOf(e, QA_PN_PROPS);
  for (const part of [/\bZap\b/, /a creature/, /\bAria\b/, /\bhit\b/, /\b13\b/]) {
    expect(line).toMatch(part);
  }
  expect(line).toMatch(/\bgraze\b/);
  expect(line).toMatch(/\b4\b/);
  expect(line).not.toMatch(/\b17\b|act-|ab-zap|Grub/);
});
