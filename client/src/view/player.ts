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
import { mayWorkDoor } from "./doors";

export interface PlayerUIState {
  selectedActorId: string;
  selectedAbilityId: string;
  /**
   * Whether a board click works a door instead of moving a token (Task 4).
   *
   * ONE BIT, THREE ROUTES TO IT — not one shared object. app.ts hands THIS
   * PANEL the real PlayerUIState object, so this file's own toggle mutates
   * `ui.doorsArmed` directly. The DM console and the board each get only a
   * COPY of the current boolean (`doorsArmed: ui.doorsArmed`) read fresh at
   * every paint(); the console's own toggle writes back through a separate
   * `toggleDoors` closure app.ts owns (view/dm.ts's DMDeps), not through this
   * object at all. The conclusion the sharing exists for still holds — arm
   * it from either panel and the other panel and the board (spectator.ts's
   * armed indication) agree at the next paint — but it holds because all
   * three read the SAME app.ts-owned bit each time, not because they hold
   * the same reference.
   */
  doorsArmed: boolean;
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

/**
 * Whether ANY door in the current scene is one this player could actually
 * work — the gate for offering the arm-doors toggle at all, so a player with
 * no controlled token near any door gets no control whose every use
 * mayWorkDoor would refuse (doors.ts's own reasoning about mayWorkDoor,
 * applied one level up: don't even OFFER what it would refuse every time).
 *
 * Walks the scene's own door tiles and asks mayWorkDoor about each, rather
 * than re-deriving the adjacency geometry here: the two must never drift,
 * or this offers a control mayWorkDoor is certain to refuse.
 *
 * NO SCENE, NO TOGGLE: `if (!scene) return false` is what a player with no
 * campaign scene yet meets — not a fallback into "sure, offer it anyway".
 * Pinned by name, not just implied by the guard's shape — see
 * client/test/player.test.ts's "no scenes at all means no toggle, even with
 * a controlled actor" (that file, not this one: nothing in THIS file is
 * "below" a doc comment).
 *
 * The "current scene" pick — the lexicographically greatest scene id, via
 * `.sort().at(-1)` — is the SAME ONE-LINE COMPUTATION doors.ts's own
 * currentSceneId and spectator.ts's renderSpectator each write out
 * independently: three copies, not one shared helper. NEITHER OF THE OTHER
 * TWO COMMENTS EXPLAINS WHY IT IS A COPY RATHER THAN A CALL — an earlier
 * draft of this comment invented a reason and attributed it to
 * spectator.ts's comment, which says nothing of the kind (it says only that
 * the active scene is the most recently created one, and that a scene
 * selector belongs with the DM console). This file makes no claim on
 * either of their behalf; the fact stated here is only that the
 * computation is duplicated three times, not why.
 *
 * THE `?? ""` FALLBACK IS NOT EQUIVALENT, and an earlier draft of this
 * comment claimed it was — the same mistake Task 3 nearly made about
 * doors.ts's identically-shaped copy, caught there before it reached that
 * file and caught here only after it did. The false argument was that the
 * fallback feeds only a SCENE LOOKUP (`st.Scenes[sceneId]`) on an object
 * with zero own keys, so no string could make the lookup succeed. That
 * misses the PROTOTYPE CHAIN: `st.Scenes[key]` for a key outside
 * `Object.keys(st.Scenes)` still resolves through whatever `st.Scenes`
 * inherits from, and "zero own keys" says nothing about what is inherited.
 * player.test.ts's "a prototype-injected empty-string scene defeats the `??
 * ""` equivalence claim" builds exactly that state — Scenes with no own
 * keys but a prototype answering "" with a real scene — and kills the
 * mutant: the real fallback ("") finds that scene, the mutant
 * ("Stryker was here!") finds nothing. NOT a state fold() can ever build
 * (fold.ts always hands Scenes an `Object.create(null)` via state.ts's
 * emptyMap, so no real campaign log can give it a prototype at all) — the
 * same category of hand-built-past-the-fold state doors.test.ts already
 * uses for this module family, and doors.ts's own currentSceneId doc
 * comment cites for its own paired claim.
 */
function mayWorkAnyDoor(st: State, me: Me): boolean {
  const sceneId = Object.keys(st.Scenes).sort().at(-1) ?? "";
  const scene = st.Scenes[sceneId];
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
      b.dataset["action"] = "use-ability";
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
  const as = document.createElement("input");
  as.placeholder = "in character as… (optional)";
  as.className = "as";
  const text = document.createElement("input");
  text.placeholder = "say or narrate";
  text.className = "text";
  const send3 = el("button", "chip", "Send");
  send3.dataset["action"] = "add-narration";
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
