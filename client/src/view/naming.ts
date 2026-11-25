import { ActorKind, type Envelope } from "../../../contract/gen/ts/vtt/v1/events_pb";
import type { SheetChange } from "../session";
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
  resource(key: string): string;
  condition(id: string): string;
  conditionTitle(id: string): string;
  entry(id: string): string;
  place(key: string): string;
  /** held answers only an envelope of the log this namer walked (by identity). */
  held(e: Envelope): Held | undefined;
}

/** Held is the entry and place an event's reference held just before it in the log (VTT-1030). */
export interface Held {
  entryId: string;
  place: string;
}

interface Known {
  actor: string;
  name: string;
  gmName: string;
}

type Named = readonly { id: string; name: string }[] | undefined;

const nameIn = (list: Named, id: string): string => list?.find((x) => x.id === id)?.name || id;

/** namer builds a window's Namer from its fold, log, role and ruleset lists. */
export function namer(
  st: State,
  log: readonly Envelope[],
  dm: boolean,
  abilities: readonly { id: string; name: string }[] | undefined,
  properties?: readonly { key: string; type: string; name: string }[],
  conditions?: readonly { id: string; name: string; description: string }[] | undefined,
  catalogue?: Named,
  places?: readonly { key: string; name: string }[] | undefined,
): Namer {
  // Name what the fold lost as the log last named it, a reference as of each event.
  const pastTokens = new Map<string, Known>();
  const pastActors = new Map<string, { name: string; party: boolean }>();
  const holding = new Map<string, Held>();
  const before = new Map<Envelope, Held | undefined>();
  const refKey = (actorId: string, referenceId: string) => JSON.stringify([actorId, referenceId]);
  for (const e of log) {
    const p = e.payload;
    if (p.case === "tokenPlaced") {
      pastTokens.set(p.value.tokenId, { actor: p.value.actorId, name: p.value.name, gmName: p.value.gmName });
    } else if (p.case === "tokenDescribed") {
      pastTokens.set(p.value.tokenId, { ...pastTokens.get(p.value.tokenId)!, name: p.value.name, gmName: p.value.gmName });
    } else if (p.case === "actorAdded" && p.value.actor) {
      const a = p.value.actor;
      pastActors.set(a.actorId, { name: a.name, party: a.kind === ActorKind.PARTY_MEMBER });
      for (const [id, r] of Object.entries(a.references)) holding.set(refKey(a.actorId, id), { entryId: r.entryId, place: r.place });
    } else if (p.case === "referenceAdded" && p.value.reference) {
      const r = p.value.reference;
      holding.set(refKey(p.value.actorId, p.value.referenceId), { entryId: r.entryId, place: r.place });
    } else if (p.case === "referenceRemoved") {
      before.set(e, holding.get(refKey(p.value.actorId, p.value.referenceId)));
    } else if (p.case === "referenceChanged") {
      const key = refKey(p.value.actorId, p.value.referenceId);
      const was = holding.get(key);
      if (was) {
        before.set(e, was);
        holding.set(key, { entryId: was.entryId, place: p.value.place });
      }
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
  const conditionName = (id: string): string => nameIn(conditions, id);
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
    ability: (id) => nameIn(abilities, id),
    resource: (key) => properties?.find((p) => p.key === key && p.type === "resource")?.name || key,
    condition: (id) => conditionName(id),
    conditionTitle: (id) => {
      const description = conditions?.find((c) => c.id === id)?.description ?? "";
      return description === "" ? conditionName(id) : `${conditionName(id)}\n${description}`;
    },
    entry: (id) => nameIn(catalogue, id),
    place: (key) => places?.find((p) => p.key === key)?.name || key,
    held: (e) => before.get(e),
  };
}

/** sheetNews is a player's toast for what sets changed of their party members' ability ids (VTT-733, VTT-734). */
export function sheetNews(changes: readonly SheetChange[], abilities: readonly { id: string; name: string }[]): string {
  const named = (ids: readonly string[]) => ids.map((id) => nameIn(abilities, id)).join(" and ");
  const sentences: string[] = [];
  for (const c of changes) {
    const who = c.name || "An unnamed character";
    if (c.given.length > 0) sentences.push(`${who} can now use ${named(c.given)}.`);
    if (c.taken.length > 0) sentences.push(`${who} can no longer use ${named(c.taken)}.`);
  }
  return sentences.join(" ");
}

/** initialOf is a disc's initial: the name's first character, or "?" for UNNAMED. */
export function initialOf(name: string): string {
  return name === UNNAMED ? "?" : [...name][0]!.toUpperCase();
}
