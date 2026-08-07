// Player-side rules: what you control, what you can afford, what you can reach.
//
// These decide what the UI OFFERS. The server decides what is allowed — every
// one of these checks is mirrored by an authoritative one behind the wire, and
// none of them is a security boundary. Their job is to keep a player from
// firing commands that will bounce, not to enforce anything.

import type { Actor, State, Token } from "./state";
import type { Ability, Me } from "./metadata";
import type { Point } from "./commands";

/** The actors this participant controls. */
export function controlledActors(st: State, participantId: string): Actor[] {
  // An empty participantId matches nothing: an uncontrolled actor has an EMPTY
  // control set, and an empty id must never be treated as a member of it, or
  // an unidentified viewer would appear to control every NPC on the board.
  // Mirrors gateway/authz.go's controls(), which makes the same refusal.
  if (participantId === "") return [];
  return Object.values(st.Actors)
    // MEMBERSHIP, not equality with the mirror. controllerId holds only
    // controllerIds[0], so comparing against it hid a SHARED character from
    // its second controller entirely — while the gateway (T3) would happily
    // let that same player move it. The client refused to show a character
    // the server had already authorised.
    .filter((a) => a.controllerIds.includes(participantId))
    .sort((a, b) => (a.actorId < b.actorId ? -1 : a.actorId > b.actorId ? 1 : 0));
}

/** Whether actor can currently pay for ability. */
export function affordable(ability: Ability, actor: Actor): boolean {
  if (ability.usage.kind === "atWill") return true;
  const name = ability.usage.resource ?? "";
  const res = actor.resources[name];
  // A resource the actor does not have is NOT free — treating it as zero-cost
  // would offer an ability the server is certain to reject.
  if (!res) return false;
  return res.current >= (ability.usage.cost ?? 0);
}

/**
 * Chebyshev distance, which is what Targeting.Range means in the ruleset
 * format: a diagonal step costs the same as an orthogonal one. Euclidean or
 * Manhattan here would grey out legal targets or offer illegal ones.
 */
export function withinRange(from: Point, to: Point, range: number): boolean {
  return Math.max(Math.abs(from.x - to.x), Math.abs(from.y - to.y)) <= range;
}

/** Tokens a given acting token could legally aim at with ability. */
export function targetableTokens(st: State, actingTokenId: string, ability: Ability): Token[] {
  const from = st.Tokens[actingTokenId];
  // A missing acting token is a transient state — the board re-renders on
  // every event, and a token can vanish between a click and its handler.
  if (!from) return [];
  return Object.values(st.Tokens)
    .filter((t) => t.SceneID === from.SceneID)
    .filter((t) => withinRange({ x: from.X, y: from.Y }, { x: t.X, y: t.Y }, ability.range))
    .sort((a, b) => (a.ID < b.ID ? -1 : a.ID > b.ID ? 1 : 0));
}


/**
 * The actors a viewer may issue commands as.
 *
 * A player is limited to what they control. A DM or agent may act as ANY
 * actor (client spec §4) — voicing an NPC is the normal case, and filtering
 * to "controlled" would leave a DM able to run nothing at all, since NPCs
 * deliberately have no controller.
 *
 * A spectator acts as nobody. That is a UI affordance, not a defence: the
 * server refuses their commands regardless.
 */
export function actableActors(st: State, me: Me): Actor[] {
  const all = Object.values(st.Actors).sort((a, b) =>
    a.actorId < b.actorId ? -1 : a.actorId > b.actorId ? 1 : 0,
  );
  switch (me.role) {
    case "dm":
    case "agent":
      return all;
    case "player":
      return controlledActors(st, me.participantId);
    default:
      return [];
  }
}
