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
import { affordable, controlledActors, targetableTokens } from "../player";
import { moveToken, useAbility, addNarration } from "../commands";
import type { ClientCommand } from "../../../contract/gen/ts/vtt/v1/commands_pb";

export interface PlayerUIState {
  selectedActorId: string;
  selectedAbilityId: string;
}

function el(tag: string, cls?: string, text?: string): HTMLElement {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  // Stryker disable next-line ConditionalExpression: the third copy of this
  // guard (see view/dm.ts and view/spectator.ts). Removing it sets textContent
  // to undefined, which a real browser stringifies and happy-dom coerces to ""
  // — observable in production, invisible to the harness, therefore NOT
  // equivalent and deliberately not filed in ts-mutation-equivalents.txt.
  if (text !== undefined) n.textContent = text;
  return n;
}

/** The token this actor currently occupies, if any. */
export function tokenForActor(st: State, actorId: string): string {
  const t = Object.values(st.Tokens).find((tok) => tok.ActorID === actorId);
  return t?.ID ?? "";
}

export function renderPlayerPanel(
  st: State,
  me: Me,
  abilities: Ability[],
  ui: PlayerUIState,
  send: (c: ClientCommand) => void,
  rerender: () => void,
): HTMLElement {
  const wrap = el("section", "player");
  wrap.appendChild(el("h2", undefined, "Your turn"));

  const mine = controlledActors(st, me.participantId);
  if (mine.length === 0) {
    wrap.appendChild(el("p", "empty", "You do not control an actor yet."));
    return wrap;
  }

  // --- actor selection ---
  const picker = el("div", "row");
  for (const a of mine) {
    const b = el("button", ui.selectedActorId === a.actorId ? "chip sel" : "chip", a.name || a.actorId);
    b.addEventListener("click", () => {
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
    wrap.appendChild(el("h3", undefined, `Targets (range ${armed.range})`));
    const list = el("div", "row");
    for (const t of targets) {
      const label = st.Actors[t.ActorID]?.name || t.ID;
      const b = el("button", "chip", label);
      b.addEventListener("click", () => {
        send(useAbility(actorId, armed.id, [t.ID]));
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

  // --- speaking ---
  wrap.appendChild(el("h3", undefined, "Say something"));
  const form = el("div", "say");
  const as = document.createElement("input");
  as.placeholder = "in character as… (optional)";
  as.className = "as";
  const text = document.createElement("input");
  text.placeholder = "say or narrate";
  text.className = "text";
  const send3 = el("button", "chip", "Send");
  const submit = () => {
    const t = text.value.trim();
    if (t === "") return;
    send(addNarration(t, as.value.trim() || undefined));
    text.value = "";
    rerender();
  };
  send3.addEventListener("click", submit);
  text.addEventListener("keydown", (e) => {
    if ((e as KeyboardEvent).key === "Enter") submit();
  });
  form.append(as, text, send3);
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
