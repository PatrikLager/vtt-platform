// Player-side rules: what you control, what you can afford, what you can reach.
//
// These decide what the UI OFFERS. The server decides what is allowed — every
// one of these checks is mirrored by an authoritative one behind the wire, and
// none of them is a security boundary. Their job is to keep a player from
// firing commands that will bounce, not to enforce anything.

import type { Actor, State, Token } from "./state";
import type { Ability } from "./metadata";
import type { Point } from "./commands";

/** The actors this participant controls. */
export function controlledActors(st: State, participantId: string): Actor[] {
  // An empty participantId matches nothing: an uncontrolled actor also has an
  // empty controllerId, and letting those compare equal would show an
  // unidentified viewer as controlling every NPC on the board.
  if (participantId === "") return [];
  return Object.values(st.Actors)
    .filter((a) => a.controllerId === participantId)
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
