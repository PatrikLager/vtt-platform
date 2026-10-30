import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  ActorKind,
  EnvelopeSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { newState, type Actor, type State, type Token } from "../src/state";
import { describe as lineOf, renderSpectator } from "../src/view/spectator";
import { initialOf, namer, UNNAMED } from "../src/view/naming";
import { renderPlayerPanel } from "../src/view/player";
import type { Ability, Me } from "../src/metadata";
import { ok } from "./support/answers";

const PT = "pt-mira";
const DM_PT = "pt-dm";
const SCENE = "scn-hall";
const PARTY = ActorKind.PARTY_MEMBER;
const NON_PARTY = ActorKind.NON_PARTY;
const UNSET = ActorKind.UNSPECIFIED;

const ABILITIES: Ability[] = [{
  id: "ab-strike", name: "Sweeping Strike", range: 20, maxTargets: 1,
  usage: { kind: "atWill" },
}];

const IDS = [
  "act-p1", "act-p2", "act-c1", "act-c2", "act-c3", "act-c4",
  "tok-p1", "tok-p2", "tok-c1a", "tok-c1b", "tok-c2a", "tok-c2b",
  "tok-c3", "tok-c4",
];
const SECRETS = [
  "Lurker Prime", "Shade Twin", "Old Hermit",
  "Cave Lurker", "Weak to light.", "Shade Elder", "Shade Younger",
];

function actor(id: string, name: string, kind: ActorKind, ctl: string[] = []): Actor {
  return {
    actorId: id, name, moduleId: "mod-x", attributes: {}, resources: {},
    controllerId: ctl[0] ?? "", controllerIds: ctl, kind,
  };
}

type Texts = Pick<Token, "Name" | "Notes" | "GMName" | "GMNotes">;

function put(st: State, id: string, actorId: string, x: number, y: number,
  texts: Texts = {}): void {
  st.Tokens[id] = { ID: id, SceneID: SCENE, ActorID: actorId, X: x, Y: y, ...texts };
}

function world(): State {
  const st = newState();
  st.Scenes[SCENE] = {
    ID: SCENE, Name: "Hall", GridWidth: 8, GridHeight: 8,
    Tiles: {}, Objects: [], OpenDoors: {}, Explored: {},
  };
  st.Actors["act-p1"] = actor("act-p1", "lera Stone", PARTY, [PT, DM_PT]);
  st.Actors["act-p2"] = actor("act-p2", "Bram Holt", PARTY, ["pt-other"]);
  st.Actors["act-c1"] = actor("act-c1", "Lurker Prime", NON_PARTY);
  st.Actors["act-c2"] = actor("act-c2", "Shade Twin", NON_PARTY);
  st.Actors["act-c3"] = actor("act-c3", "", UNSET);
  st.Actors["act-c4"] = actor("act-c4", "Old Hermit", UNSET);
  put(st, "tok-p1", "act-p1", 1, 1);
  put(st, "tok-p2", "act-p2", 2, 1, { Name: "Scout", Notes: "Keeps watch." });
  put(st, "tok-c1b", "act-c1", 3, 3, {
    Name: "a hunched figure", Notes: "It breathes slowly.",
    GMName: "Cave Lurker", GMNotes: "Weak to light.",
  });
  put(st, "tok-c1a", "act-c1", 4, 3);
  put(st, "tok-c2b", "act-c2", 5, 5, { Name: "a cloaked shape", GMName: "Shade Elder" });
  put(st, "tok-c2a", "act-c2", 6, 5, { Name: "a tall shape", GMName: "Shade Younger" });
  put(st, "tok-c3", "act-c3", 1, 4);
  put(st, "tok-c4", "act-c4", 2, 4);
  return st;
}

function seen(st: State): State {
  const scene = st.Scenes[SCENE];
  if (scene) {
    scene.Visible = {};
    for (const t of Object.values(st.Tokens)) scene.Visible[`${t.X},${t.Y}`] = true;
  }
  return st;
}

function env(init: MessageInitShape<typeof EnvelopeSchema>): Envelope {
  return create(EnvelopeSchema, init);
}

const PLAYER_TOKENS: Record<string, string> = {
  "tok-p1": "lera Stone", "tok-p2": "Scout", "tok-c1b": "a hunched figure",
  "tok-c1a": UNNAMED, "tok-c2b": "a cloaked shape", "tok-c2a": "a tall shape",
  "tok-c3": UNNAMED, "tok-c4": UNNAMED,
};
const DM_TOKENS: Record<string, string> = {
  "tok-p1": "lera Stone", "tok-p2": "Bram Holt", "tok-c1b": "Cave Lurker",
  "tok-c1a": "Lurker Prime", "tok-c2b": "Shade Elder", "tok-c2a": "Shade Younger",
  "tok-c3": UNNAMED, "tok-c4": "Old Hermit",
};
const PLAYER_ACTORS: Record<string, string> = {
  "act-p1": "lera Stone", "act-p2": "Scout", "act-c1": "a hunched figure",
  "act-c2": "a tall shape", "act-c3": UNNAMED, "act-c4": UNNAMED,
};
const DM_ACTORS: Record<string, string> = {
  "act-p1": "lera Stone", "act-p2": "Bram Holt", "act-c1": "Cave Lurker",
  "act-c2": "Shade Younger", "act-c3": UNNAMED, "act-c4": "Old Hermit",
};
const PLAYER_INITIALS: Record<string, string> = {
  "tok-p1": "L", "tok-p2": "S", "tok-c1b": "A", "tok-c1a": "?",
  "tok-c2b": "A", "tok-c2a": "A", "tok-c3": "?", "tok-c4": "?",
};
const DM_INITIALS: Record<string, string> = {
  "tok-p1": "L", "tok-p2": "B", "tok-c1b": "C", "tok-c1a": "L",
  "tok-c2b": "S", "tok-c2a": "S", "tok-c3": "?", "tok-c4": "O",
};

const roots: HTMLElement[] = [];
afterEach(() => {
  for (const r of roots.splice(0)) r.remove();
});

interface Drawn {
  root: HTMLElement;
  pins: string[];
  cells: { x: number; y: number }[];
}

function draw(st: State, dm: boolean, log: Envelope[] = [],
  pinnedCard?: string): Drawn {
  const root = document.createElement("div");
  document.body.appendChild(root);
  roots.push(root);
  const pins: string[] = [];
  const cells: { x: number; y: number }[] = [];
  renderSpectator(root, st, log, "open", {
    dm, abilities: ABILITIES,
    onCardPin: (id) => pins.push(id),
    onCell: (c) => cells.push(c),
    ...(pinnedCard === undefined ? {} : { pinnedCard }),
  });
  return { root, pins, cells };
}

function disc(root: HTMLElement, tokenId: string): HTMLElement {
  const el = root.querySelector<HTMLElement>(`.token[data-token-id="${tokenId}"]`);
  if (!el) throw new Error(`no disc for ${tokenId}`);
  return el;
}

function part(d: HTMLElement, cls: string): string | null {
  return d.querySelector(`.token-card .${cls}`)?.textContent ?? null;
}

function storyText(root: HTMLElement): string {
  return root.querySelector(".feed")?.textContent ?? "";
}

function targetLabels(panel: HTMLElement): string[] {
  return Array.from(panel.querySelectorAll('button[data-action="use-ability"]'))
    .map((b) => b.textContent ?? "");
}

function panelFor(role: Me["role"], sent: ClientCommand[] = []): HTMLElement {
  const me: Me = { participantId: role === "player" ? PT : DM_PT, name: "x", role };
  return renderPlayerPanel(world(), me, ABILITIES, {
    selectedActorId: "act-p1", selectedAbilityId: "ab-strike", doorsArmed: false,
  }, async (c) => { sent.push(c); return ok(); }, () => {});
}

// VTT-475
test("qaWS: VTT-475 a player's window names each token by its chain", () => {
  const n = namer(seen(world()), [], false, ABILITIES);
  for (const [id, want] of Object.entries(PLAYER_TOKENS)) {
    expect([id, n.token(id)]).toEqual([id, want]);
  }
});

// VTT-484
test("qaWS: VTT-484 a DM's window names each token by its chain", () => {
  const n = namer(world(), [], true, ABILITIES);
  for (const [id, want] of Object.entries(DM_TOKENS)) {
    expect([id, n.token(id)]).toEqual([id, want]);
  }
});

// VTT-475
test("qaWS: an actor is named by its first token, in id order, whose field is set", () => {
  const p = namer(seen(world()), [], false, ABILITIES);
  for (const [id, want] of Object.entries(PLAYER_ACTORS)) {
    expect([id, p.actor(id)]).toEqual([id, want]);
  }
  const d = namer(world(), [], true, ABILITIES);
  for (const [id, want] of Object.entries(DM_ACTORS)) {
    expect([id, d.actor(id)]).toEqual([id, want]);
  }
});

// VTT-475 VTT-484
test("qaWS: VTT-475 VTT-484 the naming never answers an id", () => {
  for (const dm of [false, true]) {
    const n = namer(world(), [], dm, ABILITIES);
    for (const id of IDS) {
      const got = id.startsWith("tok-") ? n.token(id) : n.actor(id);
      expect(got).not.toBe(id);
    }
    expect(n.token("tok-never")).toBe(UNNAMED);
    expect(n.actor("act-never")).toBe(UNNAMED);
  }
});

// VTT-475
test("qaWS: the fixed wording is 'a creature', and its initial '?'", () => {
  expect(UNNAMED).toBe("a creature");
  expect(initialOf(UNNAMED)).toBe("?");
  expect(initialOf("lera Stone")).toBe("L");
  expect(initialOf("\u{10428}ix")).toBe("\u{10400}");
});

// VTT-475 VTT-484
test("qaWS: VTT-475 VTT-484 each disc's initial and title are the naming", () => {
  for (const [dm, names, initials] of [
    [false, PLAYER_TOKENS, PLAYER_INITIALS],
    [true, DM_TOKENS, DM_INITIALS],
  ] as const) {
    const { root } = draw(dm ? world() : seen(world()), dm);
    for (const [id, want] of Object.entries(names)) {
      const d = disc(root, id);
      expect([id, d.title]).toEqual([id, want]);
      expect([id, d.querySelector(".initial")?.textContent])
        .toEqual([id, initials[id]]);
    }
  }
});

// VTT-474 VTT-475
test("qaWS: VTT-474 a player's card holds the naming and the notes", () => {
  const { root } = draw(seen(world()), false);
  for (const d of Array.from(root.querySelectorAll(".token"))) {
    expect(d.querySelectorAll(".token-card").length).toBe(1);
  }
  const c1b = disc(root, "tok-c1b");
  expect(part(c1b, "card-name")).toBe("a hunched figure");
  expect(part(c1b, "card-notes")).toBe("It breathes slowly.");
  const p2 = disc(root, "tok-p2");
  expect(part(p2, "card-name")).toBe("Scout");
  expect(part(p2, "card-notes")).toBe("Keeps watch.");
  const c1a = disc(root, "tok-c1a");
  expect(part(c1a, "card-name")).toBe(UNNAMED);
  // Draw each part only when it is non-empty (VTT-474 VTT-483).
  expect(part(c1a, "card-notes")).toBeNull();
});

// VTT-483
test("qaWS: VTT-483 a player's card shows no GM name or GM notes", () => {
  const { root } = draw(seen(world()), false);
  for (const d of Array.from(root.querySelectorAll<HTMLElement>(".token"))) {
    expect(d.querySelector(".card-gm-notes")).toBeNull();
    expect(d.querySelector(".card-shown")).toBeNull();
    const text = d.querySelector(".token-card")?.textContent ?? "";
    for (const s of SECRETS) expect([d.dataset["tokenId"], text.includes(s)])
      .toEqual([d.dataset["tokenId"], false]);
  }
});

// VTT-483 VTT-484
test("qaWS: VTT-483 a DM's card shows the GM name and GM notes", () => {
  const { root } = draw(world(), true);
  const c1b = disc(root, "tok-c1b");
  expect(part(c1b, "card-name")).toBe("Cave Lurker");
  expect(part(c1b, "card-notes")).toBe("It breathes slowly.");
  expect(part(c1b, "card-gm-notes")).toBe("Weak to light.");
  const c1a = disc(root, "tok-c1a");
  expect(part(c1a, "card-name")).toBe("Lurker Prime");
  // Draw each part only when it is non-empty (VTT-474 VTT-483).
  expect(part(c1a, "card-notes")).toBeNull();
  expect(part(c1a, "card-gm-notes")).toBeNull();
});

// VTT-490
test("qaWS: a DM's card shows 'players see: ' and the token's name or UNNAMED", () => {
  const { root } = draw(world(), true);
  expect(part(disc(root, "tok-c1b"), "card-shown"))
    .toBe("players see: a hunched figure");
  expect(part(disc(root, "tok-c2a"), "card-shown"))
    .toBe("players see: a tall shape");
  expect(part(disc(root, "tok-p2"), "card-shown")).toBe("players see: Scout");
  expect(part(disc(root, "tok-c1a"), "card-shown")).toBe(`players see: ${UNNAMED}`);
});

// VTT-474
test("qaWS: VTT-474 a right-click hands the page its disc's token id", () => {
  const { root, pins, cells } = draw(seen(world()), false);
  const ev = new MouseEvent("contextmenu", { bubbles: true, cancelable: true, button: 2 });
  disc(root, "tok-c1b").dispatchEvent(ev);
  expect(pins).toEqual(["tok-c1b"]);
  expect(ev.defaultPrevented).toBe(true);
  expect(cells).toEqual([]);
});

// VTT-474
test("qaWS: VTT-474 the pinned card's disc alone carries the pin", () => {
  const { root } = draw(seen(world()), false, [], "tok-c2a");
  const pinned = Array.from(root.querySelectorAll<HTMLElement>(".token.pinned"))
    .map((d) => d.dataset["tokenId"]);
  expect(pinned).toEqual(["tok-c2a"]);
  expect(part(disc(root, "tok-c2a"), "card-name")).toBe("a tall shape");
  const none = draw(seen(world()), false);
  expect(none.root.querySelectorAll(".token.pinned").length).toBe(0);
});

// VTT-474
test("qaWS: a right-click on the pinned disc goes to the page with its id", () => {
  const { root, pins } = draw(seen(world()), false, [], "tok-c2a");
  const ev = new MouseEvent("contextmenu", { bubbles: true, cancelable: true, button: 2 });
  disc(root, "tok-c2a").dispatchEvent(ev);
  expect(pins).toEqual(["tok-c2a"]);
  expect(ev.defaultPrevented).toBe(true);
});

// VTT-408
test("qaWS: a left click on a disc or its card reaches the board", () => {
  const { root, cells, pins } = draw(seen(world()), false);
  const d = disc(root, "tok-c1b");
  d.dispatchEvent(new MouseEvent("click", { bubbles: true, button: 0 }));
  expect(cells.length).toBe(1);
  d.querySelector(".token-card")
    ?.dispatchEvent(new MouseEvent("click", { bubbles: true, button: 0 }));
  expect(cells.length).toBe(2);
  expect(pins).toEqual([]);
});

// VTT-475
test("qaWS: VTT-475 a player's target buttons are the window's naming", () => {
  const sent: ClientCommand[] = [];
  const panel = panelFor("player", sent);
  const labels = targetLabels(panel);
  for (const want of ["a hunched figure", "a tall shape", "Scout", UNNAMED]) {
    expect([want, labels.includes(want)]).toEqual([want, true]);
  }
  for (const l of labels) {
    for (const bad of [...IDS, ...SECRETS, "Bram Holt"]) {
      expect([l, l.includes(bad)]).toEqual([l, false]);
    }
  }
  const tall = Array.from(panel.querySelectorAll<HTMLButtonElement>(
    'button[data-action="use-ability"]')).find((b) => b.textContent === "a tall shape");
  tall?.click();
  const last = sent.at(-1)?.command;
  expect(last?.case).toBe("useAbility");
  expect(last?.case === "useAbility" ? last.value.targetIds : []).toEqual(["act-c2"]);
});

// VTT-484
test("qaWS: VTT-484 a DM's and an agent's target buttons are the GM chain", () => {
  for (const role of ["dm", "agent"] as const) {
    const labels = targetLabels(panelFor(role));
    for (const want of ["Cave Lurker", "Shade Younger", "Bram Holt", "Old Hermit"]) {
      expect([role, want, labels.includes(want)]).toEqual([role, want, true]);
    }
    for (const bad of ["a hunched figure", "a tall shape", "Scout"]) {
      expect([role, bad, labels.includes(bad)]).toEqual([role, bad, false]);
    }
  }
});

// VTT-476 VTT-475
test("qaWS: a player's Story words each case of the table", () => {
  const n = namer(seen(world()), [], false, ABILITIES);
  const cases: [string, MessageInitShape<typeof EnvelopeSchema>][] = [
    ["lera Stone is in the party", {
      payload: { case: "actorAdded", value: { actor: { actorId: "act-p1", kind: PARTY } } },
    }],
    ["a hunched figure comes into sight", {
      payload: { case: "actorAdded", value: { actor: { actorId: "act-c1", kind: NON_PARTY } } },
    }],
    ["a creature comes into sight", {
      payload: { case: "actorAdded", value: { actor: { actorId: "act-c4", kind: UNSET } } },
    }],
    ["a hunched figure is at 3,3", {
      payload: { case: "tokenPlaced", value: {
        tokenId: "tok-c1b", sceneId: SCENE, actorId: "act-c1", position: { x: 3, y: 3 },
      } },
    }],
    ["Scout moved to 4,0", {
      payload: { case: "tokenMoved", value: {
        tokenId: "tok-p2", sceneId: SCENE, from: { x: 2, y: 1 }, to: { x: 4, y: 0 },
      } },
    }],
    ["a creature taken off the board", {
      payload: { case: "tokenRemoved", value: { tokenId: "tok-c3" } },
    }],
    ["a tall shape went out of sight", {
      payload: { case: "tokenHidden", value: { tokenId: "tok-c2a" } },
    }],
    ["a cloaked shape described", {
      payload: { case: "tokenDescribed", value: { tokenId: "tok-c2b", name: "a cloaked shape" } },
    }],
    ["lera Stone left", { payload: { case: "actorRemoved", value: { actorId: "act-p1" } } }],
    ["a hunched figure gained dazed", {
      payload: { case: "conditionApplied", value: { actorId: "act-c1", conditionId: "dazed" } },
    }],
    ["a hunched figure lost dazed", {
      payload: { case: "conditionRemoved", value: { actorId: "act-c1", conditionId: "dazed" } },
    }],
    ["lera Stone vigor → 3", {
      payload: { case: "resourceChanged", value: {
        actorId: "act-p1", resource: "vigor", delta: -2, newValue: 3,
      } },
    }],
    ["control of Scout given", {
      payload: { case: "actorControlGranted", value: {
        actorId: "act-p2", participantId: PT, kind: PARTY,
      } },
    }],
    ["control of Scout taken back", {
      payload: { case: "actorControlRevoked", value: { actorId: "act-p2", participantId: PT } },
    }],
    ["a door opened at 3,0", {
      payload: { case: "doorOpened", value: { sceneId: SCENE, at: { x: 3, y: 0 } } },
    }],
    ["a door closed at 3,0", {
      payload: { case: "doorClosed", value: { sceneId: SCENE, at: { x: 3, y: 0 } } },
    }],
    ["the view changed", {
      payload: { case: "sceneSeen", value: { sceneId: SCENE, visible: ["1,1"] } },
    }],
    ["Narrator: The wind howls.", {
      payload: { case: "narrationAdded", value: { as: "Narrator", text: "The wind howls." } },
    }],
    ["The wind howls.", {
      payload: { case: "narrationAdded", value: { text: "The wind howls." } },
    }],
    ["something happened", {}],
  ];
  for (const [want, init] of cases) {
    expect(lineOf(env(init), n)).toBe(want);
  }
});

// VTT-476 VTT-475
test("qaWS: a DM's Story words by the GM chain", () => {
  const n = namer(world(), [], true, ABILITIES);
  expect(lineOf(env({ payload: { case: "actorAdded", value: {
    actor: { actorId: "act-c1", name: "Lurker Prime", kind: NON_PARTY },
  } } }), n)).toBe("Cave Lurker comes into sight");
  expect(lineOf(env({ payload: { case: "tokenPlaced", value: {
    tokenId: "tok-c1a", sceneId: SCENE, actorId: "act-c1", position: { x: 4, y: 3 },
  } } }), n)).toBe("Lurker Prime is at 4,3");
  expect(lineOf(env({ payload: { case: "actorControlGranted", value: {
    actorId: "act-p2", participantId: PT, kind: PARTY,
  } } }), n)).toBe("control of Bram Holt given");
});

// VTT-263 VTT-476
test("qaWS: a move with a reason reads it after a dash, as before", () => {
  const n = namer(seen(world()), [], false, ABILITIES);
  const line = lineOf(env({ payload: { case: "tokenMoved", value: {
    tokenId: "tok-p2", sceneId: SCENE, to: { x: 5, y: 2 }, reason: "fled the noise",
  } } }), n);
  expect(line).toBe("Scout moved to 5,2 — fled the noise");
});

function used(outcomes: { targetId: string; result: string; total: number }[],
  targetIds = ["act-c1"]): Envelope {
  return env({ payload: { case: "abilityUsed", value: {
    actorId: "act-p1", abilityId: "ab-strike", targetIds, outcomes,
    outcomeSummary: "ab-strike on act-c1: landed (17 vs 15)",
    rolls: [
      { expression: "1d20+4", results: [13], total: 17 },
      { expression: "1d8+2", results: [6], total: 8 },
    ],
  } } });
}

// VTT-477
test("qaWS: VTT-477 an ability's line names the ability, target, result and roll", () => {
  const p = namer(seen(world()), [], false, ABILITIES);
  const one = used([{ targetId: "act-c1", result: "landed", total: 17 }]);
  const line = lineOf(one, p);
  expect(line).toBe("lera Stone used Sweeping Strike on a hunched figure: landed, rolled 17");
  expect(line).not.toContain("15");
  expect(line).not.toContain("vs");
  const d = namer(world(), [], true, ABILITIES);
  expect(lineOf(one, d))
    .toBe("lera Stone used Sweeping Strike on Cave Lurker: landed, rolled 17");
});

// VTT-477
test("qaWS: VTT-477 each outcome in order, joined by ';'", () => {
  const p = namer(seen(world()), [], false, ABILITIES);
  const two = used([
    { targetId: "act-c2", result: "glanced", total: 4 },
    { targetId: "act-c1", result: "landed", total: 17 },
  ], ["act-c1", "act-c2"]);
  expect(lineOf(two, p)).toBe("lera Stone used Sweeping Strike on a tall shape: " +
    "glanced, rolled 4; on a hunched figure: landed, rolled 17");
});

// VTT-477
test("qaWS: an ability with no outcomes names its targets", () => {
  const p = namer(seen(world()), [], false, ABILITIES);
  expect(lineOf(used([], ["act-c1", "act-c2"]), p))
    .toBe("lera Stone used Sweeping Strike on a hunched figure, a tall shape");
});

// VTT-477
test("qaWS: an ability without a ruleset name is named by its id", () => {
  const out = [{ targetId: "act-c1", result: "landed", total: 17 }];
  for (const abilities of [undefined, []]) {
    const p = namer(seen(world()), [], false, abilities);
    expect(lineOf(used(out), p))
      .toBe("lera Stone used ab-strike on a hunched figure: landed, rolled 17");
  }
  expect(namer(world(), [], false, ABILITIES).ability("ab-strike"))
    .toBe("Sweeping Strike");
});

// VTT-489
test("qaWS: a token the fold no longer holds keeps its newest name", () => {
  const log = [
    env({ sequence: 1n, payload: { case: "tokenPlaced", value: {
      tokenId: "tok-x", sceneId: SCENE, actorId: "act-c1", position: { x: 2, y: 2 },
      name: "a lean figure", gmName: "Pit Scout",
    } } }),
    env({ sequence: 2n, payload: { case: "tokenDescribed", value: {
      tokenId: "tok-x", name: "a limping figure", gmName: "Pit Scout Hurt",
    } } }),
    env({ sequence: 3n, payload: { case: "tokenPlaced", value: {
      tokenId: "tok-y", sceneId: SCENE, actorId: "act-c2", position: { x: 1, y: 2 },
      name: "a pale figure",
    } } }),
    env({ sequence: 4n, payload: { case: "tokenRemoved", value: { tokenId: "tok-x" } } }),
    env({ sequence: 5n, payload: { case: "tokenHidden", value: { tokenId: "tok-y" } } }),
  ];
  const removed = log[3];
  const hidden = log[4];
  if (!removed || !hidden) throw new Error("fixture");
  const p = namer(seen(world()), log, false, ABILITIES);
  expect(p.token("tok-x")).toBe("a limping figure");
  expect(lineOf(removed, p)).toBe("a limping figure taken off the board");
  expect(lineOf(hidden, p)).toBe("a pale figure went out of sight");
  const d = namer(world(), log, true, ABILITIES);
  expect(d.token("tok-x")).toBe("Pit Scout Hurt");
});

// VTT-489
test("qaWS: an actor the fold no longer holds keeps its newest name", () => {
  const log = [
    env({ sequence: 1n, payload: { case: "actorAdded", value: {
      actor: { actorId: "act-w", name: "Wren", kind: PARTY },
    } } }),
    env({ sequence: 2n, payload: { case: "actorRemoved", value: { actorId: "act-w" } } }),
    env({ sequence: 3n, payload: { case: "actorAdded", value: {
      actor: { actorId: "act-w", name: "Wren the Bold", kind: PARTY },
    } } }),
    env({ sequence: 4n, payload: { case: "actorRemoved", value: { actorId: "act-w" } } }),
  ];
  const left = log[3];
  if (!left) throw new Error("fixture");
  for (const dm of [false, true]) {
    const n = namer(world(), log, dm, ABILITIES);
    expect(n.actor("act-w")).toBe("Wren the Bold");
    expect(lineOf(left, n)).toBe("Wren the Bold left");
  }
});

// VTT-489
test("qaWS: a creature the fold no longer holds is named by its token in the log", () => {
  const log = [
    env({ sequence: 1n, payload: { case: "actorAdded", value: {
      actor: { actorId: "act-g", kind: NON_PARTY },
    } } }),
    env({ sequence: 2n, payload: { case: "tokenPlaced", value: {
      tokenId: "tok-g", sceneId: SCENE, actorId: "act-g", position: { x: 2, y: 2 },
      name: "a grey figure",
    } } }),
    env({ sequence: 3n, payload: { case: "tokenHidden", value: { tokenId: "tok-g" } } }),
    env({ sequence: 4n, payload: { case: "actorRemoved", value: { actorId: "act-g" } } }),
  ];
  const left = log[3];
  if (!left) throw new Error("fixture");
  const p = namer(seen(world()), log, false, ABILITIES);
  expect(p.actor("act-g")).toBe("a grey figure");
  expect(lineOf(left, p)).toBe("a grey figure left");
});

type PayloadCase = NonNullable<Envelope["payload"]["case"]>;

function everyCase(): Record<PayloadCase, Envelope> {
  const at = { x: 3, y: 3 };
  return {
    tokenMoved: env({ payload: { case: "tokenMoved", value: {
      tokenId: "tok-c1b", sceneId: SCENE, to: at } } }),
    attackRolled: env({ payload: { case: "attackRolled", value: {
      attackerId: "act-p1", targetId: "act-c1", expression: "1d20", total: 12,
      versus: "guard", outcome: "landed" } } }),
    sceneCreated: env({ payload: { case: "sceneCreated", value: {
      sceneId: SCENE, name: "Hall", gridWidth: 8, gridHeight: 8 } } }),
    actorAdded: env({ payload: { case: "actorAdded", value: {
      actor: { actorId: "act-c1", kind: NON_PARTY } } } }),
    tokenPlaced: env({ payload: { case: "tokenPlaced", value: {
      tokenId: "tok-c1b", sceneId: SCENE, actorId: "act-c1", position: at } } }),
    sessionStarted: env({ payload: { case: "sessionStarted", value: { name: "First night" } } }),
    sessionEnded: env({ payload: { case: "sessionEnded", value: {} } }),
    abilityUsed: used([{ targetId: "act-c1", result: "landed", total: 17 }]),
    resourceChanged: env({ payload: { case: "resourceChanged", value: {
      actorId: "act-c1", resource: "vigor", delta: -1, newValue: 2 } } }),
    conditionApplied: env({ payload: { case: "conditionApplied", value: {
      actorId: "act-c1", conditionId: "dazed" } } }),
    conditionRemoved: env({ payload: { case: "conditionRemoved", value: {
      actorId: "act-c1", conditionId: "dazed" } } }),
    narrationAdded: env({ payload: { case: "narrationAdded", value: {
      text: "The wind howls." } } }),
    noteUpserted: env({ payload: { case: "noteUpserted", value: {
      key: "note-1", title: "Rumour", text: "A light in the hills." } } }),
    noteDeleted: env({ payload: { case: "noteDeleted", value: { key: "note-1" } } }),
    adventureLoaded: env({ payload: { case: "adventureLoaded", value: {
      adventureId: "adv-crypt", name: "The Crypt" } } }),
    actorControlGranted: env({ payload: { case: "actorControlGranted", value: {
      actorId: "act-c1", participantId: PT, kind: PARTY } } }),
    actorControlRevoked: env({ payload: { case: "actorControlRevoked", value: {
      actorId: "act-c1", participantId: PT } } }),
    doorOpened: env({ payload: { case: "doorOpened", value: { sceneId: SCENE, at } } }),
    doorClosed: env({ payload: { case: "doorClosed", value: { sceneId: SCENE, at } } }),
    tokenHidden: env({ payload: { case: "tokenHidden", value: { tokenId: "tok-c1b" } } }),
    sceneSeen: env({ payload: { case: "sceneSeen", value: { sceneId: SCENE } } }),
    tokenRemoved: env({ payload: { case: "tokenRemoved", value: { tokenId: "tok-c1b" } } }),
    actorRemoved: env({ payload: { case: "actorRemoved", value: { actorId: "act-c1" } } }),
    tokenDescribed: env({ payload: { case: "tokenDescribed", value: {
      tokenId: "tok-c1b", name: "a hunched figure" } } }),
  };
}

function caseNames(): string[] {
  const oneof = EnvelopeSchema.oneofs.find((o) => o.name === "payload");
  return (oneof?.fields ?? []).map((f) => f.localName);
}

// VTT-476 VTT-475
test("qaWS: VTT-476 VTT-475 no line of any case is a payload's name or an id",
  () => {
  const all = everyCase();
  expect(Object.keys(all).sort()).toEqual(caseNames().sort());
  const names = [...caseNames(), ...EnvelopeSchema.fields
    .flatMap((f) => (f.message ? [f.message.name] : []))];
  for (const dm of [false, true]) {
    const n = namer(dm ? world() : seen(world()), [], dm, ABILITIES);
    for (const [c, e] of [...Object.entries(all), ["none", env({})] as const]) {
      const line = lineOf(e, n);
      expect([c, line.length > 0]).toEqual([c, true]);
      for (const bad of [...names, ...IDS]) {
        expect([c, line, line.toLowerCase().includes(bad.toLowerCase())])
          .toEqual([c, line, false]);
      }
    }
  }
});

// VTT-475 VTT-476 VTT-477 VTT-483
test("qaWS: VTT-475 VTT-483 a player's whole window shows no id and no GM text", () => {
  const log = [...Object.values(everyCase()), env({})];
  const { root } = draw(seen(world()), false, log);
  const titles = Array.from(root.querySelectorAll("[title]"))
    .map((el) => el.getAttribute("title") ?? "");
  const shown = [root.textContent ?? "", ...titles].join("\n");
  for (const bad of [...IDS, ...SECRETS, "vs 15", "rolled 8"]) {
    expect([bad, shown.includes(bad)]).toEqual([bad, false]);
  }
  for (const c of caseNames()) {
    expect([c, storyText(root).includes(c)]).toEqual([c, false]);
  }
});

// VTT-475 VTT-477
test("qaWS: VTT-475 VTT-477 a player's Story shows the window's naming", () => {
  const log = [
    everyCase().tokenPlaced,
    used([{ targetId: "act-c1", result: "landed", total: 17 }]),
  ];
  const story = storyText(draw(seen(world()), false, log).root);
  expect(story).toContain("a hunched figure is at 3,3");
  expect(story).toContain(
    "lera Stone used Sweeping Strike on a hunched figure: landed, rolled 17");
});

// VTT-484 VTT-477
test("qaWS: VTT-484 VTT-477 a DM's Story shows the GM chain", () => {
  const log = [
    everyCase().tokenPlaced,
    used([{ targetId: "act-c1", result: "landed", total: 17 }]),
  ];
  const story = storyText(draw(world(), true, log).root);
  expect(story).toContain("Cave Lurker is at 3,3");
  expect(story).toContain(
    "lera Stone used Sweeping Strike on Cave Lurker: landed, rolled 17");
  expect(story).not.toContain("vs 15");
});
