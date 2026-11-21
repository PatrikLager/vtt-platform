// Keep every rule this panel applies — what its sheet names, what you can
// afford, what is in reach — in player.ts as a tested pure function; none is
// a security boundary, since the server checks each.
// Offer every token in range, your own included: the ruleset format has no
// target kind, and the server refuses an illegal use.

import type { Ability, ConditionDef, Me, Property, SheetPage } from "../metadata";
import type { Actor, State } from "../state";
import { affordable, controlledActors, offered, readLine, sheetFor, sheetNames, targetableTokens, type ShownSection } from "../player";
import { moveToken, useAbility, setViewpoint, speakAs, speakOutOfCharacter, addWhisper } from "../commands";
import type { ClientCommand, CommandResult } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import { currentSceneId, doorButtonsFor } from "./doors";
import { namer, type Namer } from "./naming";

export interface PlayerUIState {
  selectedActorId: string;
  selectedAbilityId: string;
  /** Whether a DM's or an agent's board click works a door instead of moving a token (SPEC-022). */
  doorsArmed: boolean;
  /** The name of the sheet's page the panel shows (VTT-838). */
  page?: string;
  // VTT-522
  sayText?: string;
  /** A line the panel sent is unanswered: Send and Enter send nothing (VTT-523). */
  saying?: boolean;
}

/** PanelLayout is what the window's newest ruleset answer lays out (VTT-845, VTT-855). */
export interface PanelLayout {
  sheet?: SheetPage[] | undefined;
  conditions?: ConditionDef[] | undefined;
  freeUsageName?: string | undefined;
}

/** A player's switch between the party members the gateway lets them watch (VTT-918). */
export interface Switcher {
  actors: readonly { actorId: string; name: string }[];
  onSwitch: (actorId: string) => void;
}

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  // Stryker disable next-line ConditionalExpression: only forced true is equivalent
  // and only while n is empty, yet this hides forced false too: drop the directive
  // before n can hold children when this line runs.
  if (text !== undefined) n.textContent = text;
  return n;
}

/** The token this actor currently occupies, if any. */
export function tokenForActor(st: State, actorId: string): string {
  const t = Object.values(st.Tokens).find((tok) => tok.ActorID === actorId);
  return t?.ID ?? "";
}

/** actingActor is the actor a board click moves and a door button names (VTT-408, VTT-883). */
export function actingActor(st: State, me: Me, ui: PlayerUIState): string {
  return ui.selectedActorId || (controlledActors(st, me.participantId)[0]?.actorId ?? "");
}

export function renderPlayerPanel(
  st: State,
  me: Me,
  abilities: Ability[],
  ui: PlayerUIState,
  send: (c: ClientCommand) => Promise<CommandResult>,
  rerender: () => void,
  switcher?: Switcher,
  notify?: (msg: string) => void,
  properties?: Property[],
  layout?: PanelLayout,
): HTMLElement {
  const names = namer(st, [], me.role === "dm" || me.role === "agent", abilities, properties);
  const wrap = el("section", "player");

  const mine = switcher ? switcher.actors : controlledActors(st, me.participantId);
  if (mine.length === 0) {
    wrap.appendChild(el("p", "empty", "You do not control an actor yet."));
    return wrap;
  }
  const actorId = ui.selectedActorId || mine[0]!.actorId;
  const actor = st.Actors[actorId];
  const chip = mine.find((a) => a.actorId === actorId);
  // VTT-836
  if (actor) wrap.appendChild(el("h2", undefined, chip ? chip.name || chip.actorId : actor.name || actorId));

  // --- actor selection ---
  const picker = el("div", "row");
  for (const a of mine) {
    const b = el("button", ui.selectedActorId === a.actorId ? "chip sel" : "chip", a.name || a.actorId);
    b.addEventListener("click", () => {
      if (switcher) {
        switcher.onSwitch(a.actorId);
        return;
      }
      ui.selectedActorId = a.actorId;
      ui.selectedAbilityId = "";
      rerender();
    });
    picker.appendChild(b);
  }
  wrap.appendChild(picker);

  const tokenId = tokenForActor(st, actorId);

  if (!actor) return wrap;
  if (tokenId === "") {
    wrap.appendChild(el("p", "empty", "That actor has no token on the board yet."));
  }

  const pages = sheetFor(actor, st.Conditions[actorId] ?? [], abilities, properties, layout?.conditions, layout?.sheet);
  const page = pages.find((p) => p.name === ui.page) ?? pages[0]!;
  if (pages.length > 1) {
    const tabs = el("div", "tabs");
    for (const p of pages) {
      const b = el("button", p === page ? "tab sel" : "tab", p.name);
      b.addEventListener("click", () => {
        ui.page = p.name;
        rerender();
      });
      tabs.appendChild(b);
    }
    wrap.appendChild(tabs);
  }
  const sheet = el("div", "sheet");
  const button = (ab: Ability) => {
    const can = affordable(ab, actor);
    const b = el("button", ui.selectedAbilityId === ab.id ? "chip sel" : "chip", ab.name || ab.id);
    // Show an unaffordable ability disabled, never hide it (VTT-695).
    (b as HTMLButtonElement).disabled = !can || tokenId === "";
    b.title = buttonTitle(ab.description, costTitle(ab, actor, names, can, layout?.freeUsageName));
    b.addEventListener("click", () => {
      ui.selectedAbilityId = ui.selectedAbilityId === ab.id ? "" : ab.id;
      rerender();
    });
    return b;
  };
  for (const s of page.sections) {
    sheet.appendChild(el("h3", undefined, s.name));
    sheet.appendChild(drawSection(s, button));
  }
  wrap.appendChild(sheet);
  const offers = offered(abilities, actor);

  // --- targets, when an ability is armed ---
  const armed = offers.find((a) => a.id === ui.selectedAbilityId);
  if (armed && tokenId !== "") {
    const targets = targetableTokens(st, tokenId, armed);
    wrap.appendChild(el("h3", undefined, `Targets (range ${armed.range})`));
    const list = el("div", "row");
    for (const t of targets) {
      const b = el("button", "chip", names.token(t.ID));
      b.dataset["action"] = "use-ability";
      b.addEventListener("click", () => {
        void send(useAbility(actorId, armed.id, [t.ActorID]));
        ui.selectedAbilityId = "";
        rerender();
      });
      list.appendChild(b);
    }
    // No empty-state here, deliberately: this list can never BE empty. The
    // acting token is at Chebyshev distance 0 from itself and shares its own
    // SceneID, so it passes both of targetableTokens' filters for any range
    // >= 0 — and the ruleset compiler rejects a negative one outright
    // (internal/rules/compile.go: "targeting.range must not be negative"), so
    // no ability reaching this client can carry it. targetableTokens' other
    // empty return, the missing acting token, is unreachable from here too:
    // tokenId comes from tokenForActor, which only ever returns an ID it
    // found IN st.Tokens, and the branch above requires it to be non-empty.
    wrap.appendChild(list);
  } else if (tokenId !== "") {
    wrap.appendChild(el("p", "hint", "Click the board to move."));
  }

  // VTT-881 VTT-883 VTT-886
  const doors = doorButtonsFor(st, me, actingActor(st, me, ui));
  if (doors.length > 0) {
    const row = el("div", "row doors");
    for (const d of doors) {
      const b = el("button", "chip", d.label);
      b.dataset["action"] = "work-door";
      b.addEventListener("click", () => void send(d.command()));
      row.appendChild(b);
    }
    wrap.appendChild(row);
  }

  // VTT-532 VTT-540
  if (me.role !== "player") return wrap;
  wrap.appendChild(el("h3", undefined, "Say something"));
  const form = el("div", "say");
  const text = document.createElement("input");
  text.placeholder = "say something, /ooc to step out of character, /dm to whisper to the DM";
  text.className = "text";
  text.dataset["field"] = "say-text";
  text.value = ui.sayText ?? "";
  text.addEventListener("input", () => {
    ui.sayText = text.value;
  });
  const send3 = el("button", "chip", "Send");
  send3.dataset["action"] = "add-narration";
  // VTT-524 VTT-595 VTT-596
  const submit = () => {
    if (ui.saying) return;
    const line = readLine(text.value);
    if (line.kind === "whisper") return void notify?.("Whisper to the DM with /dm text.");
    if (line.kind === "toDM" && line.text === "") return void notify?.("Say what to whisper: /dm text.");
    if (line.text === "") return;
    ui.saying = true;
    const command = line.kind === "toDM" ? addWhisper(line.text, [], actorId)
      : line.kind === "ooc" ? speakOutOfCharacter(line.text)
      : speakAs(line.text, actorId);
    const said = send(command);
    void said
      .then((res) => {
        if (!res.ok) return;
        text.value = "";
        ui.sayText = "";
        rerender();
      }, (e: unknown) => console.error(e))
      .finally(() => {
        ui.saying = false;
      });
  };
  send3.addEventListener("click", submit);
  text.addEventListener("keydown", (e) => {
    if ((e as KeyboardEvent).key === "Enter") submit();
  });
  form.append(text, send3);
  wrap.appendChild(form);

  return wrap;
}

// VTT-854
function buttonTitle(description: string | undefined, costLine: string): string {
  return [description ?? "", costLine].filter((part) => part !== "").join("\n");
}

// VTT-844 VTT-855
function costTitle(ab: Ability, actor: Actor, names: Namer, can: boolean, freeUsageName: string | undefined): string {
  if (ab.usage.kind !== "resource") return freeUsageName ?? "";
  const key = ab.usage.resource ?? "";
  const held = actor.resources[key];
  const amount = held ? ` ${held.current}/${held.max}` : "";
  return `${names.resource(key)}${amount}, costs ${ab.usage.cost ?? 0}${can ? "" : " — not enough"}`;
}

// VTT-839 VTT-841 VTT-842 VTT-843
function drawSection(s: ShownSection, button: (ab: Ability) => HTMLElement): HTMLElement {
  switch (s.kind) {
    case "values": {
      const list = el("div", "values");
      for (const v of s.values) {
        const n = el("span", "value", `${v.name} ${v.text}`);
        if (v.title !== "") n.title = v.title;
        list.appendChild(n);
      }
      return list;
    }
    case "conditions": {
      const list = el("div", "conditions");
      for (const c of s.conditions) {
        const n = el("span", "condition", c.name);
        if (c.title !== "") n.title = c.title;
        list.appendChild(n);
      }
      return list;
    }
    case "abilities": {
      const row = el("div", "row");
      for (const ab of s.abilities) row.appendChild(button(ab));
      return row;
    }
  }
}

/** Build the move command for a board click, or null when it is not actionable. */
export function moveCommandFor(
  st: State,
  me: Me,
  ui: PlayerUIState,
  cell: { x: number; y: number },
): ClientCommand | null {
  if (controlledActors(st, me.participantId).length === 0) return null;
  const actorId = actingActor(st, me, ui);
  const tokenId = tokenForActor(st, actorId);
  if (tokenId === "") return null;
  // An armed ability means the click is aimed, not a move; targeting is
  // handled by the target buttons so a stray board click cannot fire it. Keep
  // a selection the sheet does not name unarmed (VTT-695).
  const actor = st.Actors[actorId];
  if (ui.selectedAbilityId !== "" && actor && sheetNames(actor, ui.selectedAbilityId)) return null;
  return moveToken(tokenId, cell);
}

/**
 * A player's board click: a switch to a party member the gateway lets them
 * watch whose token is on the cell, nothing on the watched one's own token,
 * else a move of it (VTT-919, VTT-408).
 */
export function playerClickFor(
  st: State,
  me: Me,
  ui: PlayerUIState,
  watchable: readonly string[],
  cell: { x: number; y: number },
): ClientCommand | null {
  const scene = currentSceneId(st);
  const here = Object.values(st.Tokens)
    .filter((t) => t.SceneID === scene && t.X === cell.x && t.Y === cell.y)
    .map((t) => t.ActorID);
  const others = watchable.filter((id) => id !== ui.selectedActorId && here.includes(id)).sort();
  if (others.length > 0) return setViewpoint(others[0]!);
  if (here.includes(ui.selectedActorId)) return null;
  return moveCommandFor(st, me, ui, cell);
}
