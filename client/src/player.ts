// Player-side rules: what you control, are offered, can afford and can reach.
//
// These decide what the UI OFFERS. The server decides what is allowed — every
// one of these checks is mirrored by an authoritative one behind the wire, and
// none of them is a security boundary. Their job is to keep a player from
// firing commands that will bounce, not to enforce anything.

import type { Actor, ActorCondition, State, Token } from "./state";
import type { Ability, ConditionDef, Me, Property, SheetPage, SheetUsage } from "./metadata";
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

/** Whether actor's sheet names abilityId (VTT-695). */
export function sheetNames(actor: Actor, abilityId: string): boolean {
  return actor.abilityIds.includes(abilityId);
}

/** The window's abilities the actor's sheet names, in the window's order (VTT-695). */
export function offered(abilities: Ability[], actor: Actor): Ability[] {
  return abilities.filter((a) => sheetNames(actor, a.id));
}

/** ShownValue is one value as the sheet shows it: named, written and titled. */
export interface ShownValue {
  key: string;
  name: string;
  text: string;
  title: string;
}

/** ShownCondition is one condition as the sheet shows it. */
export interface ShownCondition {
  id: string;
  name: string;
  title: string;
}

/** ShownSection is one section of the sheet with what it shows the character. */
export type ShownSection =
  | { name: string; kind: "values"; values: ShownValue[] }
  | { name: string; kind: "conditions"; conditions: ShownCondition[] }
  | { name: string; kind: "abilities"; abilities: Ability[] };

/** ShownPage is one page of the sheet as the panel draws it. */
export interface ShownPage {
  name: string;
  sections: ShownSection[];
}

const byKey = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

// VTT-841
function shownValue(actor: Actor, key: string, type: "number" | "resource", properties: readonly Property[]): ShownValue | undefined {
  const held = type === "number" ? actor.attributes[key] : actor.resources[key];
  if (held === undefined) return undefined;
  const text = typeof held === "number" ? String(held) : `${held.current}/${held.max}`;
  const p = properties.find((x) => x.key === key && x.type === type);
  return { key, name: p?.name || key, text, title: p?.description ?? "" };
}

function heldValues(actor: Actor, key: string, properties: readonly Property[]): ShownValue[] {
  return [shownValue(actor, key, "number", properties), shownValue(actor, key, "resource", properties)]
    .filter((v): v is ShownValue => v !== undefined);
}

function usageShows(u: SheetUsage, a: Ability): boolean {
  switch (u.kind) {
    case "atWill":
      return a.usage.kind === "atWill";
    case "resource":
      return a.usage.kind === "resource" && a.usage.resource === u.resource;
  }
}

function shows(s: ShownSection): boolean {
  switch (s.kind) {
    case "values":
      return s.values.length > 0;
    case "conditions":
      return s.conditions.length > 0;
    case "abilities":
      return s.abilities.length > 0;
  }
}

/**
 * sheetFor is the character's sheet as the panel draws it: the window's
 * declared pages and sections in their order, each section left out when it
 * has nothing to show, or one unnamed page of "Values", "Conditions" and
 * "Abilities" when none is declared (VTT-839 to VTT-843).
 */
export function sheetFor(
  actor: Actor,
  conditions: readonly ActorCondition[],
  abilities: Ability[],
  properties: readonly Property[] | undefined,
  conditionDefs: readonly ConditionDef[] | undefined,
  sheet: readonly SheetPage[] | undefined,
): ShownPage[] {
  const props = properties ?? [];
  const shownConditions = conditions.map((c): ShownCondition => {
    const d = conditionDefs?.find((x) => x.id === c.ID);
    return { id: c.ID, name: d?.name || c.ID, title: d?.description ?? "" };
  });
  const offers = offered(abilities, actor);

  if (!sheet || sheet.length === 0) {
    const values: ShownValue[] = [];
    for (const p of props) {
      const v = shownValue(actor, p.key, p.type, props);
      if (v) values.push(v);
    }
    for (const key of [...new Set([...Object.keys(actor.attributes), ...Object.keys(actor.resources)])].sort(byKey)) {
      for (const type of ["number", "resource"] as const) {
        if (props.some((p) => p.key === key && p.type === type)) continue;
        const v = shownValue(actor, key, type, props);
        if (v) values.push(v);
      }
    }
    const sections: ShownSection[] = [
      { name: "Values", kind: "values", values },
      { name: "Conditions", kind: "conditions", conditions: shownConditions },
      { name: "Abilities", kind: "abilities", abilities: offers },
    ];
    return [{ name: "", sections: sections.filter(shows) }];
  }

  return sheet.map((page) => ({
    name: page.name,
    sections: page.sections.map((s): ShownSection => {
      switch (s.kind) {
        case "values":
          return { name: s.name, kind: "values", values: s.values.flatMap((k) => heldValues(actor, k, props)) };
        case "conditions":
          return { name: s.name, kind: "conditions", conditions: shownConditions };
        case "abilities":
          return { name: s.name, kind: "abilities", abilities: offers.filter((a) => s.abilities.some((u) => usageShows(u, a))) };
      }
    }).filter(shows),
  }));
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

/** Line is a Say line as readLine reads it. */
export type Line =
  | { kind: "say"; text: string }
  | { kind: "ooc"; text: string }
  | { kind: "whisper"; name: string; text: string }
  | { kind: "toDM"; text: string };

/**
 * readLine reads a Say line: /ooc out of character (VTT-501); /w or /whisper,
 * /dm, /gm or /togm in any case a whisper (VTT-668, VTT-599).
 */
export function readLine(raw: string): Line {
  const t = raw.trim();
  if (/^\/ooc(\s|$)/.test(t)) return { kind: "ooc", text: t.slice(4).trim() };
  const dm = /^\/(?:dm|gm|togm)(?:\s+|$)(.*)$/is.exec(t);
  if (dm) return { kind: "toDM", text: dm[1]!.trim() };
  const w = /^\/(?:w|whisper)(?:\s+|$)(.*)$/is.exec(t);
  if (!w) return { kind: "say", text: t };
  const rest = w[1]!;
  if (rest.startsWith('"')) {
    const close = rest.indexOf('"', 1);
    if (close < 0) return { kind: "whisper", name: rest.slice(1).trim(), text: "" };
    return { kind: "whisper", name: rest.slice(1, close).trim(), text: rest.slice(close + 1).trim() };
  }
  const [, name, text] = /^(\S*)\s*(.*)$/s.exec(rest)!;
  return { kind: "whisper", name: name!, text: text!.trim() };
}
