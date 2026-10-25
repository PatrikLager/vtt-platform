import { ActorKind, type Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";
import type { State } from "../state";

/** UNNAMED is what a window calls a token or an actor it has no name for. */
export const UNNAMED = "a creature";

/** Namer names by a window's chain, never a token or actor by its id (VTT-475). */
export interface Namer {
  dm: boolean;
  token(id: string): string;
  /** shown is what a player's window names a token, whatever the role (VTT-490). */
  shown(id: string): string;
  actor(id: string | undefined): string;
  ability(id: string): string;
}

interface Known {
  actor: string;
  name: string;
  gmName: string;
}

/** namer builds a window's Namer from its fold, log, role and abilities. */
export function namer(
  st: State,
  log: readonly Envelope[],
  dm: boolean,
  abilities: readonly { id: string; name: string }[] | undefined,
): Namer {
  // Keep the newest entry: what the fold has lost is named as the log last named it.
  const pastTokens = new Map<string, Known>();
  const pastActors = new Map<string, { name: string; party: boolean }>();
  for (const e of log) {
    const p = e.payload;
    if (p.case === "tokenPlaced") {
      pastTokens.set(p.value.tokenId, { actor: p.value.actorId, name: p.value.name, gmName: p.value.gmName });
    } else if (p.case === "tokenDescribed") {
      pastTokens.set(p.value.tokenId, { ...pastTokens.get(p.value.tokenId)!, name: p.value.name, gmName: p.value.gmName });
    } else if (p.case === "actorAdded" && p.value.actor) {
      const a = p.value.actor;
      pastActors.set(a.actorId, { name: a.name, party: a.kind === ActorKind.PARTY_MEMBER });
    }
  }
  const tokenOf = (id: string): Known | undefined => {
    const t = st.Tokens[id];
    return t ? { actor: t.ActorID, name: t.Name ?? "", gmName: t.GMName ?? "" } : pastTokens.get(id);
  };
  const actorName = (id: string | undefined, gm: boolean): string => {
    const a = st.Actors[id!];
    const known = a ? { name: a.name, party: a.kind === ActorKind.PARTY_MEMBER } : pastActors.get(id!);
    return (gm || known?.party ? known?.name : "") || UNNAMED;
  };
  const own = (k: Known, gm: boolean): string => (gm ? k.gmName : k.name);
  const tokenBy = (id: string, gm: boolean): string => {
    const k = tokenOf(id);
    return k ? own(k, gm) || actorName(k.actor, gm) : UNNAMED;
  };
  const firstNamed = new Map<string, string>();
  for (const tid of [...new Set([...Object.keys(st.Tokens), ...pastTokens.keys()])].sort()) {
    const k = tokenOf(tid)!;
    if (own(k, dm) !== "" && !firstNamed.has(k.actor)) firstNamed.set(k.actor, own(k, dm));
  }

  return {
    dm,
    token: (id) => tokenBy(id, dm),
    shown: (id) => tokenBy(id, false),
    actor: (id) => firstNamed.get(id!) ?? actorName(id, dm),
    ability: (id) => abilities?.find((a) => a.id === id)?.name || id,
  };
}

/** initialOf is a disc's initial: the name's first character, or "?" for UNNAMED. */
export function initialOf(name: string): string {
  return name === UNNAMED ? "?" : [...name][0]!.toUpperCase();
}
