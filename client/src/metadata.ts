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

/** Who this token makes you: the participant id every control check is a
 *  membership test against, and the role that decides which panels render.
 *
 *  NOT what you control. This interface carried a `controls: string[]` until
 *  2026-08-24, mirroring a server field fed by a SQLite column no grant ever
 *  wrote — so it could report a character the log had never given you. The
 *  client never read it: controlledActors (player.ts) filters the folded
 *  st.Actors on controllerIds, which is the log talking. */
export interface Me {
  participantId: string;
  name: string;
  role: "dm" | "player" | "agent" | "spectator";
}

export interface AdventureMeta {
  id: string;
  name: string;
}

/** One entry from GET /api/maps (metadata.go's mapMetaJSON): a standalone map
 *  the campaign's own maps/ has loaded and validated.
 *
 *  IT CARRIED A `pack?: PackRef` UNTIL 2026-09-05 — id, display name, cell size
 *  — and by then the field had been unreachable for two tasks. Task 5 of
 *  2026-09-02-art-is-a-flat-library deleted mapdef.Map.Pack and made a map file
 *  declaring one a refusal (that plan's design spec §7), so handleMaps had
 *  nothing to look a pack up by and dropped its packRefJSON; Task 7 deleted the
 *  route the id was for. The TypeScript went on compiling with the field always
 *  undefined and app.ts's loadMapPacks went on iterating and finding nothing,
 *  which is why client/test/art-assets.test.ts now carries the absence test the
 *  removal should have had. */
export interface MapMeta {
  id: string;
  name: string;
  gridWidth: number;
  gridHeight: number;
  /** This map's grid resolution, ALREADY RESOLVED by the server: the map file's
   *  own cell_px when it declared one, and the campaign's default otherwise. See
   *  MapsResponse below for why nothing here reads it yet. */
  cellPx: number;
}

/** GET /api/maps in full: the list, plus the campaign's DEFAULT grid resolution.
 *
 *  cellPx appears TWICE and the two answer different questions. Here, at the top
 *  level, it is the campaign's default — what the next map that declares nothing
 *  will draw at, from campaign/campaign.json, which is optional and defaults to
 *  64. On each MapMeta it is that map's own resolved value.
 *
 *  IT WAS CAMPAIGN-LEVEL AND ONLY THAT until 2026-09-05, when Patrik moved it
 *  per-map from how MapTool solves the same problem (grid size lives on the
 *  Zone, not the campaign). Design spec §6's original argument — "a grid is
 *  uniform... One number per campaign says that plainly" — is true of one MAP
 *  and false of a campaign: grid size is exactly what differs between an art set
 *  drawn at 64 and one drawn at 128.
 *
 *  NOTHING IN THIS CLIENT READS IT, AND THAT IS NOT AN OVERSIGHT — it is the
 *  same state the old pack.cellPx was in, declared and never consumed, and this
 *  paragraph exists so the next reader does not "restore" a reader that never
 *  existed. view/spectator.ts's CELL = 44 is the only cell size the renderer
 *  uses, and it is a SCREEN size: how large a square is drawn. cellPx is a
 *  SOURCE size: how large the picture is. drawImage scales one to the other
 *  whatever they are, so art at any resolution already draws correctly, and
 *  wiring cellPx into a draw call would change pixels for no requirement. An
 *  earlier comment here claimed the renderer read it "to draw at the right
 *  scale"; that was false and was corrected in review on 2026-09-04.
 *
 *  WHERE THE NUMBER DOES BIND is `vtt art install` (cmd/vtt/art.go's
 *  warnOffGrid), which warns when an installed picture is not a whole number of
 *  cell_px squares — with the operator holding the file, which is the only place
 *  anyone can act on it. It is declared here because the server sends it and a
 *  response type that hid a field would be lying about the wire, not because
 *  anything here has a use for it yet. */
export interface MapsResponse {
  maps: MapMeta[];
  cellPx: number;
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

/**
 * fetchMaps lists every standalone map the campaign has loaded (GET /api/maps),
 * for the DM console's map picker.
 *
 * IT USED TO BE HOW THE CLIENT LEARNED WHERE ART LIVED, and it is not any more.
 * The wire carries no container on a live Scene — SceneCreated resolves art
 * names into facts at compile time and stops there — so this list was the only
 * place a client could learn a map's pack id, and app.ts loaded every configured
 * map's pack from it. There are no containers now: a Tile.Art is a filename stem
 * in one flat directory and view/art-assets.ts asks for it directly, off the
 * folded scene, with no listing involved. This route is back to being a listing.
 *
 * A 404 (no maps installed) degrades to an empty list, matching fetchAdventures'
 * own posture, rather than surfacing as an error the DM console has no route
 * naming maps to explain.
 */
export async function fetchMaps(base: string, token: string): Promise<MapMeta[]> {
  const body = await getJSON<MapsResponse>(base, "/api/maps", token);
  return body?.maps ?? [];
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
