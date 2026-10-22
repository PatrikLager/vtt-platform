import "./support/dom";
import { test, expect, afterEach } from "bun:test";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { useAbility } from "../src/commands";
import { newState, type Actor, type State, type Token } from "../src/state";
import type { Ability, Me } from "../src/metadata";
import type { ClientCommand, UseAbility } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";

const SCENE = "cellar";
const OTHER_SCENE = "taproom";

const FISTS: Ability = { id: "fists", name: "Fists", range: 1, maxTargets: 1, usage: { kind: "atWill" } };
const SPLASH: Ability = {
  id: "splash-of-water", name: "Splash of Water", range: 1, maxTargets: 1, usage: { kind: "atWill" },
};
const SOBER: Ability = { id: "sober-up", name: "Sober Up", range: 0, maxTargets: 1, usage: { kind: "atWill" } };

const PLAYER: Me = { participantId: "p-mara", name: "Patrik", role: "player" };
const DM: Me = { participantId: "p-dm", name: "Dee", role: "dm" };
const AGENT: Me = { participantId: "p-agent", name: "Agent", role: "agent" };

function actor(id: string, name: string, controllers: string[], kind: ActorKind): Actor {
  return {
    actorId: id,
    name,
    moduleId: "tavern-brawl",
    attributes: { brawn: 4, grit: 1, footing: 10 },
    resources: { drink: { current: 1, max: 4 }, stamina: { current: 2, max: 2 } },
    controllerId: controllers[0] ?? "",
    controllerIds: [...controllers],
    kind,
  };
}

function token(id: string, actorId: string, x: number, y: number, sceneId = SCENE): Token {
  return { ID: id, SceneID: sceneId, ActorID: actorId, X: x, Y: y };
}

function world(actors: Actor[], tokens: Token[]): State {
  const st = newState();
  st.Scenes[SCENE] = { ID: SCENE, Name: "The Cellar", GridWidth: 12, GridHeight: 12 };
  st.Scenes[OTHER_SCENE] = { ID: OTHER_SCENE, Name: "The Taproom", GridWidth: 12, GridHeight: 12 };
  for (const a of actors) st.Actors[a.actorId] = a;
  for (const t of tokens) st.Tokens[t.ID] = t;
  return st;
}

function armed(actorId: string, abilityId: string): PlayerUIState {
  return { selectedActorId: actorId, selectedAbilityId: abilityId, doorsArmed: false };
}

interface Drawn {
  el: HTMLElement;
  sent: ClientCommand[];
}

function draw(st: State, me: Me, abilities: Ability[], ui: PlayerUIState): Drawn {
  const sent: ClientCommand[] = [];
  const el = renderPlayerPanel(st, me, abilities, ui, (c) => { sent.push(c); }, () => {});
  document.body.appendChild(el);
  return { el, sent };
}

function texts(el: HTMLElement): string[] {
  return Array.from(el.querySelectorAll("button")).map((b) => (b.textContent ?? "").trim());
}

function buttonsNaming(el: HTMLElement, name: string): HTMLButtonElement[] {
  return Array.from(el.querySelectorAll("button")).filter((b) => (b.textContent ?? "").includes(name));
}

function onlyUseAbility(sent: ClientCommand[]): UseAbility {
  expect(sent.length).toBe(1);
  const c = sent[0];
  if (c === undefined) throw new Error("nothing sent");
  expect(c.command.case).toBe("useAbility");
  if (c.command.case !== "useAbility") throw new Error(`sent ${String(c.command.case)}`);
  return c.command.value;
}

interface Shot {
  ua: UseAbility;
  ui: PlayerUIState;
}

// Do not find a target by its label: the actor chips carry the same names. A
// target is a button naming the actor whose click, in a fresh panel, sends a
// use_ability.
function shots(st: State, me: Me, abilities: Ability[], ui: PlayerUIState, name: string): Shot[] {
  const n = buttonsNaming(draw(st, me, abilities, { ...ui }).el, name).length;
  const out: Shot[] = [];
  for (let i = 0; i < n; i++) {
    document.body.innerHTML = "";
    const u = { ...ui };
    const { el, sent } = draw(st, me, abilities, u);
    const b = buttonsNaming(el, name)[i];
    if (b === undefined) throw new Error(`button ${i} naming ${name} vanished`);
    b.click();
    if (sent.some((c) => c.command.case === "useAbility")) out.push({ ua: onlyUseAbility(sent), ui: u });
  }
  return out;
}

function clickOnly(st: State, me: Me, abilities: Ability[], ui: PlayerUIState, name: string): UseAbility {
  const s = shots(st, me, abilities, ui, name);
  expect(s.length).toBe(1);
  const first = s[0];
  if (first === undefined) throw new Error(`no target button naming ${name}`);
  return first.ua;
}

afterEach(() => {
  document.body.innerHTML = "";
});

const MARA = () => actor("act-mara", "Mara Voss", [PLAYER.participantId], ActorKind.PARTY_MEMBER);
const HOLLIS = () => actor("act-hollis", "Hollis Brack", [], ActorKind.PARTY_MEMBER);
const RAT = () => actor("rat-1", "Cellar Rat", [], ActorKind.NON_PARTY);

// VTT-436
test("qaAT: one target — the button aims at the actor its token shows, never at the token", () => {
  const st = world([MARA(), HOLLIS()], [token("tok-mara", "act-mara", 5, 5), token("tok-hollis", "act-hollis", 6, 5)]);
  const ua = clickOnly(st, PLAYER, [FISTS], armed("act-mara", "fists"), "Hollis Brack");
  expect(ua.targetIds).toEqual(["act-hollis"]);
  expect(ua.targetIds).not.toContain("tok-hollis");
});

// VTT-441
test("qaAT: the command names the caster as actorId and the armed ability as abilityId", () => {
  const st = world([MARA(), HOLLIS()], [token("tok-mara", "act-mara", 5, 5), token("tok-hollis", "act-hollis", 5, 6)]);
  const ua = clickOnly(st, PLAYER, [FISTS, SPLASH], armed("act-mara", "splash-of-water"), "Hollis Brack");
  expect(ua.actorId).toBe("act-mara");
  expect(ua.abilityId).toBe("splash-of-water");
  expect(ua.targetIds).toEqual(["act-hollis"]);
});

// VTT-436
test("qaAT: several targets in range — each button aims at its own token's actor", () => {
  const actors = [MARA(), HOLLIS(), RAT(), actor("rat-2", "Bilge Rat", [], ActorKind.NON_PARTY)];
  const tokens = [
    token("tok-mara", "act-mara", 5, 5),
    token("tok-hollis", "act-hollis", 6, 6),
    token("t-7f3a", "rat-1", 4, 5),
    token("t-0c19", "rat-2", 5, 4),
  ];
  const st = world(actors, tokens);
  const want: [string, string][] = [["Hollis Brack", "act-hollis"], ["Cellar Rat", "rat-1"], ["Bilge Rat", "rat-2"]];
  for (const [name, actorId] of want) {
    document.body.innerHTML = "";
    const ua = clickOnly(st, PLAYER, [FISTS], armed("act-mara", "fists"), name);
    expect(ua.actorId).toBe("act-mara");
    expect(ua.targetIds).toEqual([actorId]);
  }
});

// VTT-440
test("qaAT: a named actor's target button is labelled by its name, not its token's id", () => {
  const st = world(
    [MARA(), HOLLIS(), RAT()],
    [token("tok-mara", "act-mara", 5, 5), token("tok-hollis", "act-hollis", 6, 6), token("t-7f3a", "rat-1", 4, 4)],
  );
  const ui = armed("act-mara", "fists");
  expect(shots(st, PLAYER, [FISTS], ui, "Hollis Brack").length).toBe(1);
  expect(shots(st, PLAYER, [FISTS], ui, "Cellar Rat").length).toBe(1);
  const { el } = draw(st, PLAYER, [FISTS], ui);
  for (const t of texts(el)) {
    expect(t).not.toContain("tok-hollis");
    expect(t).not.toContain("t-7f3a");
  }
});

// VTT-436, VTT-441
test("qaAT: a token showing another actor the player controls is aimed at that actor", () => {
  const dog = actor("act-dog", "Biscuit", [PLAYER.participantId], ActorKind.PARTY_MEMBER);
  const st = world([MARA(), dog], [token("tok-mara", "act-mara", 5, 5), token("tok-biscuit", "act-dog", 6, 4)]);
  const ua = clickOnly(st, PLAYER, [SPLASH], armed("act-mara", "splash-of-water"), "Biscuit");
  expect(ua.actorId).toBe("act-mara");
  expect(ua.targetIds).toEqual(["act-dog"]);
});

// VTT-436
test("qaAT: the caster's own token at distance 0 is offered and aimed at the caster's actor", () => {
  const st = world([MARA(), HOLLIS()], [token("tok-mara", "act-mara", 5, 5), token("tok-hollis", "act-hollis", 6, 5)]);
  const ui = armed("act-mara", "sober-up");
  expect(shots(st, PLAYER, [SOBER], ui, "Hollis Brack").length).toBe(0);
  document.body.innerHTML = "";
  const ua = clickOnly(st, PLAYER, [SOBER], ui, "Mara Voss");
  expect(ua.actorId).toBe("act-mara");
  expect(ua.targetIds).toEqual(["act-mara"]);
});

// VTT-436
test("qaAT: a DM's panel aims at the actor the clicked token shows", () => {
  const goblin = actor("gob-boss", "Grizzle", [DM.participantId], ActorKind.NON_PARTY);
  const st = world([goblin, MARA()], [token("tok-grizzle", "gob-boss", 2, 2), token("tok-mara", "act-mara", 3, 3)]);
  const ua = clickOnly(st, DM, [FISTS], armed("gob-boss", "fists"), "Mara Voss");
  expect(ua.actorId).toBe("gob-boss");
  expect(ua.abilityId).toBe("fists");
  expect(ua.targetIds).toEqual(["act-mara"]);
});

// VTT-436
test("qaAT: an agent's panel aims at the actor the clicked token shows", () => {
  const goblin = actor("gob-boss", "Grizzle", [AGENT.participantId], ActorKind.NON_PARTY);
  const st = world([goblin, HOLLIS()], [token("tok-grizzle", "gob-boss", 2, 2), token("tok-hollis", "act-hollis", 2, 1)]);
  const ua = clickOnly(st, AGENT, [FISTS], armed("gob-boss", "fists"), "Hollis Brack");
  expect(ua.actorId).toBe("gob-boss");
  expect(ua.targetIds).toEqual(["act-hollis"]);
});

// VTT-436
test("qaAT: a token whose id is another actor's id is still aimed at the actor it shows", () => {
  const st = world(
    [MARA(), HOLLIS(), RAT()],
    [token("tok-mara", "act-mara", 5, 5), token("rat-1", "act-hollis", 6, 5), token("tok-rat", "rat-1", 5, 6, OTHER_SCENE)],
  );
  const ua = clickOnly(st, PLAYER, [FISTS], armed("act-mara", "fists"), "Hollis Brack");
  expect(ua.targetIds).toEqual(["act-hollis"]);
});

// VTT-436
test("qaAT: two tokens of one actor give two buttons, each aimed at that actor", () => {
  const st = world(
    [MARA(), RAT()],
    [token("tok-mara", "act-mara", 5, 5), token("t-aa01", "rat-1", 4, 4), token("t-aa02", "rat-1", 6, 6)],
  );
  const ui = armed("act-mara", "fists");
  const s = shots(st, PLAYER, [FISTS], ui, "Cellar Rat");
  expect(s.length).toBe(2);
  for (const shot of s) expect(shot.ua.targetIds).toEqual(["rat-1"]);
});

// VTT-441
test("qaAT: the click disarms the ability", () => {
  const st = world([MARA(), HOLLIS()], [token("tok-mara", "act-mara", 5, 5), token("tok-hollis", "act-hollis", 6, 5)]);
  const ui = armed("act-mara", "fists");
  const s = shots(st, PLAYER, [FISTS], ui, "Hollis Brack");
  expect(s.length).toBe(1);
  const after = s[0]?.ui;
  if (after === undefined) throw new Error("no target button");
  expect(after.selectedAbilityId).toBe("");
  document.body.innerHTML = "";
  expect(shots(st, PLAYER, [FISTS], after, "Hollis Brack").length).toBe(0);
});

// VTT-439
test("qaAT: targets are tokens within Chebyshev range on the acting token's scene", () => {
  const actors = [MARA(), HOLLIS(), RAT(), actor("rat-2", "Bilge Rat", [], ActorKind.NON_PARTY)];
  const tokens = [
    token("tok-mara", "act-mara", 5, 5),
    token("tok-hollis", "act-hollis", 6, 6),
    token("t-7f3a", "rat-1", 7, 5),
    token("t-0c19", "rat-2", 5, 6, OTHER_SCENE),
  ];
  const st = world(actors, tokens);
  const ui = armed("act-mara", "fists");
  expect(shots(st, PLAYER, [FISTS], ui, "Hollis Brack").length).toBe(1);
  expect(shots(st, PLAYER, [FISTS], ui, "Cellar Rat").length).toBe(0);
  expect(shots(st, PLAYER, [FISTS], ui, "Bilge Rat").length).toBe(0);
});

// VTT-439
test("qaAT: a caster with no token on the board is offered no target", () => {
  const st = world([MARA(), HOLLIS()], [token("tok-hollis", "act-hollis", 6, 5)]);
  expect(shots(st, PLAYER, [FISTS], armed("act-mara", "fists"), "Hollis Brack").length).toBe(0);
});

// VTT-441
test("qaAT: with no actor selected the first controlled actor by id casts, from its own token", () => {
  const bram = actor("act-bram", "Bram Holt", [PLAYER.participantId], ActorKind.PARTY_MEMBER);
  const st = world(
    [MARA(), bram, HOLLIS()],
    [token("tok-mara", "act-mara", 9, 9), token("tok-bram", "act-bram", 0, 0), token("tok-hollis", "act-hollis", 1, 1)],
  );
  const ua = clickOnly(st, PLAYER, [FISTS], armed("", "fists"), "Hollis Brack");
  expect(ua.actorId).toBe("act-bram");
  expect(ua.targetIds).toEqual(["act-hollis"]);
});

// VTT-438
test("qaAT: one button per ability the ruleset lists", () => {
  const st = world([MARA()], [token("tok-mara", "act-mara", 5, 5)]);
  const { el } = draw(st, PLAYER, [FISTS, SPLASH, SOBER], armed("act-mara", ""));
  const t = texts(el);
  for (const name of ["Fists", "Splash of Water", "Sober Up"]) {
    expect(t.filter((x) => x.includes(name)).length).toBe(1);
  }
});

// UseAbility.target_ids in contract/vtt/v1/commands.proto
test("qaAT: useAbility carries the actor ids it is given as target_ids", () => {
  const c = useAbility("act-mara", "fists", ["act-hollis", "rat-1"]);
  const ua = onlyUseAbility([c]);
  expect(ua.actorId).toBe("act-mara");
  expect(ua.abilityId).toBe("fists");
  expect(ua.targetIds).toEqual(["act-hollis", "rat-1"]);
});
