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
