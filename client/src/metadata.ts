// Typed wrappers over the gateway's read-only metadata routes.
//
// Auth is a Bearer HEADER, matching internal/gateway/metadata.go: a token in
// a query string leaks into access logs, Referer headers and browser history.
// The WebSocket route uses ?token= only because a browser cannot set headers
// on a handshake.

export interface Ability {
  id: string;
  name: string;
  range: number;
  maxTargets: number;
  usage: { kind: "atWill" | "resource"; resource?: string; cost?: number };
}

export interface ConditionDef {
  id: string;
  name: string;
  description: string;
}

export interface RulesetMeta {
  id: string;
  name: string;
  abilities: Ability[];
  conditions: ConditionDef[];
  resources: string[];
}

/** Who this token makes you. Without it the client cannot tell which actors
 *  it controls, and cannot choose which panels to render. */
export interface Me {
  participantId: string;
  name: string;
  role: "dm" | "player" | "agent" | "spectator";
  controls: string[];
}

export interface AdventureMeta {
  id: string;
  name: string;
}

async function getJSON<T>(base: string, path: string, token: string): Promise<T | null> {
  const resp = await fetch(base + path, {
    headers: { Authorization: `Bearer ${token}` },
  });
  switch (resp.status) {
    case 200:
      return (await resp.json()) as T;
    case 404:
      // Ordinary absence, not a failure: "there is no guide" is a state the
      // UI renders as an empty panel.
      return null;
    case 401:
      throw new Error("metadata: unauthorized — the token is unknown or revoked");
    case 403:
      // Deliberately distinct from 404. A DM debugging an empty panel needs
      // to know whether the content is missing or merely not theirs to read.
      throw new Error("metadata: forbidden — this role may not read that");
    default:
      throw new Error(`metadata: ${path} returned ${resp.status}`);
  }
}

export async function fetchMe(base: string, token: string): Promise<Me> {
  const me = await getJSON<Me>(base, "/api/me", token);
  if (!me) throw new Error("metadata: /api/me is unavailable");
  return me;
}

export async function fetchRuleset(base: string, token: string): Promise<RulesetMeta> {
  const rs = await getJSON<RulesetMeta>(base, "/api/ruleset", token);
  // The server answers 200 with empty collections when nothing is loaded, so
  // a null here would mean the route is missing entirely.
  if (!rs) throw new Error("metadata: /api/ruleset is unavailable");
  return rs;
}

export async function fetchAdventures(base: string, token: string): Promise<AdventureMeta[]> {
  const body = await getJSON<{ adventures: AdventureMeta[] }>(base, "/api/adventures", token);
  return body?.adventures ?? [];
}

export async function fetchRulesetGuide(base: string, token: string): Promise<string | null> {
  const body = await getJSON<{ guide: string }>(base, "/api/ruleset/guide", token);
  return body?.guide ?? null;
}

export async function fetchAdventureGuide(
  base: string,
  token: string,
  id: string,
): Promise<string | null> {
  const body = await getJSON<{ guide: string }>(
    base,
    `/api/adventures/${encodeURIComponent(id)}/guide`,
    token,
  );
  return body?.guide ?? null;
}

/** The shared join link and whether the door is open (spec §2). */
export interface JoinLink {
  open: boolean;
  secret: string;
}

/** One person at the table, with what they are allowed to do (spec §3.1). */
export interface Roster {
  participantId: string;
  name: string;
  role: "dm" | "player" | "agent" | "spectator";
}

/**
 * Read the join link. DM/agent only — the secret admits ANYBODY who holds it.
 *
 * null on any failure, matching the other fetches here: a console that cannot
 * read the link degrades to one without a sharing panel, which is better than
 * a blank page. The caller distinguishes "not loaded yet" from "you are not a
 * DM" by whether it asked at all.
 */
export async function fetchJoinLink(base: string, token: string): Promise<JoinLink | null> {
  return await getJSON<JoinLink>(base, "/api/join-link", token);
}

/**
 * Read the table's roster: who exists, and what each of them may do.
 *
 * NOT derived from presence. Presence answers "who is connected right now" and
 * carries no role, deliberately — a role folded into a presence frame would go
 * stale the moment somebody was promoted without reconnecting, which is
 * exactly what live re-resolution made possible (spec §3.2).
 */
export async function fetchParticipants(base: string, token: string): Promise<Roster[] | null> {
  // null on failure rather than [], because the two mean different things and
  // the caller renders them differently. List() always contains at least the
  // caller, so an EMPTY roster cannot happen — an empty array would therefore
  // be a failure wearing the costume of an ordinary answer, and the console
  // would quietly drop its "Who may do what" panel with nothing said.
  return await getJSON<Roster[]>(base, "/api/participants", token);
}
