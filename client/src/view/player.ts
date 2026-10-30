// The player panel: pick an actor you control, move it, use an ability, speak.
//
// Every rule this panel applies — who you control, what you can afford, what
// is in reach — lives in player.ts as a tested pure function. None of them is
// a security boundary: the server checks all three authoritatively, and this
// only keeps a player from firing commands that would bounce.
//
// One ledgered limitation shows through here (T3): the ruleset format has no
// target-kind, so the picker cannot tell a heal from a strike. Every token in
// range is offered, including your own, and the server rejects illegal uses.

import type { State } from "../state";
import type { Ability, Me } from "../metadata";
import { affordable, controlledActors, readLine, targetableTokens } from "../player";
import { moveToken, useAbility, addNarration, setViewpoint, speakAs, speakOutOfCharacter } from "../commands";
import type { ClientCommand, CommandResult } from "../../../contract/gen/ts/vtt/v1/commands_pb";
import { currentSceneId, mayWorkDoor } from "./doors";
import { namer } from "./naming";

export interface PlayerUIState {
  selectedActorId: string;
  selectedAbilityId: string;
  /** Whether a board click works a door instead of moving a token (SPEC-022). */
  doorsArmed: boolean;
  /** What the Say boxes hold across paints (VTT-521, VTT-522). */
  sayAs?: string;
  sayText?: string;
  /** A line the panel sent is unanswered: Send and Enter send nothing (VTT-523). */
  saying?: boolean;
}

/** A player's switch between the party members the gateway lets them watch (VTT-457). */
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

/**
 * Whether any door in the current scene is one this player could work, the
 * gate for offering the arm-doors toggle at all. Ask mayWorkDoor per door tile
 * rather than re-deriving its geometry here: the two must never drift.
 */
function mayWorkAnyDoor(st: State, me: Me): boolean {
  const scene = st.Scenes[currentSceneId(st)];
  if (!scene) return false;
  return Object.entries(scene.Tiles ?? {}).some(([key, tile]) => {
    if (tile.Kind !== "door") return false;
    const [x, y] = key.split(",").map(Number);
    return mayWorkDoor(st, me, { x: x!, y: y! });
  });
}

export function renderPlayerPanel(
  st: State,
  me: Me,
  abilities: Ability[],
  ui: PlayerUIState,
  send: (c: ClientCommand) => Promise<CommandResult>,
  rerender: () => void,
  switcher?: Switcher,
): HTMLElement {
  const wrap = el("section", "player");
  wrap.appendChild(el("h2", undefined, "Your turn"));

  const mine = switcher ? switcher.actors : controlledActors(st, me.participantId);
  if (mine.length === 0) {
    wrap.appendChild(el("p", "empty", "You do not control an actor yet."));
    return wrap;
  }

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

  const actorId = ui.selectedActorId || mine[0]!.actorId;
  const actor = st.Actors[actorId];
  const tokenId = tokenForActor(st, actorId);

  if (!actor) return wrap;
  if (tokenId === "") {
    wrap.appendChild(el("p", "empty", "That actor has no token on the board yet."));
  }

  // --- abilities ---
  if (abilities.length > 0) {
    wrap.appendChild(el("h3", undefined, "Abilities"));
    const list = el("div", "row");
    for (const ab of abilities) {
      const can = affordable(ab, actor);
      const b = el("button", ui.selectedAbilityId === ab.id ? "chip sel" : "chip", ab.name || ab.id);
      // Unaffordable abilities are shown and disabled rather than hidden: a
      // player needs to know the ability exists and why it is unavailable.
      (b as HTMLButtonElement).disabled = !can || tokenId === "";
      b.title =
        ab.usage.kind === "resource"
          ? `${ab.usage.resource} ${ab.usage.cost}${can ? "" : " — not enough"}`
          : "at will";
      b.addEventListener("click", () => {
        ui.selectedAbilityId = ui.selectedAbilityId === ab.id ? "" : ab.id;
        rerender();
      });
      list.appendChild(b);
    }
    wrap.appendChild(list);
  }

  // --- targets, when an ability is armed ---
  const armed = abilities.find((a) => a.id === ui.selectedAbilityId);
  if (armed && tokenId !== "") {
    const targets = targetableTokens(st, tokenId, armed);
    const names = namer(st, [], me.role === "dm" || me.role === "agent", abilities);
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

  // --- doors ---
  //
  // PLAYER ONLY (Task 4): dm and agent already get this from the DM console,
  // which they also see beside this very panel (app.ts's canAct/isDM both
  // admit a DM) — offering it here too would be a second control for the
  // exact same bit. Gated on mayWorkAnyDoor so a player with no controlled
  // token near any door gets no toggle at all, rather than one whose every
  // use mayWorkDoor would refuse.
  if (me.role === "player" && mayWorkAnyDoor(st, me)) {
    const armBtn = el(
      "button",
      ui.doorsArmed ? "chip sel" : "chip",
      ui.doorsArmed ? "Doors armed" : "Arm doors",
    );
    armBtn.dataset["action"] = "arm-doors";
    armBtn.addEventListener("click", () => {
      ui.doorsArmed = !ui.doorsArmed;
      rerender();
    });
    wrap.appendChild(armBtn);
  }

  // --- speaking ---
  wrap.appendChild(el("h3", undefined, "Say something"));
  const form = el("div", "say");
  // Never draw or send a typed speaker for a player (VTT-500).
  const speaksFreely = me.role !== "player";
  const as = document.createElement("input");
  as.placeholder = "in character as… (optional)";
  as.className = "as";
  as.dataset["field"] = "say-as";
  as.value = ui.sayAs ?? "";
  as.addEventListener("input", () => {
    ui.sayAs = as.value;
  });
  const text = document.createElement("input");
  text.placeholder = speaksFreely ? "say or narrate" : "say something, or /ooc to step out of character";
  text.className = "text";
  text.dataset["field"] = "say-text";
  text.value = ui.sayText ?? "";
  text.addEventListener("input", () => {
    ui.sayText = text.value;
  });
  const send3 = el("button", "chip", "Send");
  send3.dataset["action"] = "add-narration";
  // Never empty or repaint at the press, nor touch `say-as` (VTT-521, VTT-524).
  const submit = () => {
    if (ui.saying) return;
    const line = readLine(text.value);
    if (line.text === "") return;
    const name = speaksFreely ? as.value.trim() || undefined : undefined;
    ui.saying = true;
    const said = line.ooc
      ? send(speakOutOfCharacter(line.text, name))
      : send(speaksFreely ? addNarration(line.text, name) : speakAs(line.text, actorId));
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
  if (speaksFreely) form.append(as);
  form.append(text, send3);
  wrap.appendChild(form);

  return wrap;
}

/** Build the move command for a board click, or null when it is not actionable. */
export function moveCommandFor(
  st: State,
  me: Me,
  ui: PlayerUIState,
  cell: { x: number; y: number },
): ClientCommand | null {
  const mine = controlledActors(st, me.participantId);
  if (mine.length === 0) return null;
  const actorId = ui.selectedActorId || mine[0]!.actorId;
  const tokenId = tokenForActor(st, actorId);
  if (tokenId === "") return null;
  // An armed ability means the click is aimed, not a move; targeting is
  // handled by the target buttons so a stray board click cannot fire it.
  if (ui.selectedAbilityId !== "") return null;
  return moveToken(tokenId, cell);
}

/**
 * A player's board click: a switch to a party member the gateway lets them
 * watch whose token is on the cell, nothing on the watched one's own token,
 * else a move of it (VTT-460, VTT-408).
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
