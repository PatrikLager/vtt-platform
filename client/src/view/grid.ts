// Grid geometry and the data a token disc renders from.
//
// Kept free of the DOM on purpose: the arithmetic is where the bugs live (an
// off-by-one cell sends a player's move somewhere they did not click), and it
// is testable without a browser.

import type { State } from "../state";

export interface Geometry {
  /** Pixel size of one square cell. */
  cell: number;
  /** Grid dimensions in cells. */
  width: number;
  height: number;
}

export interface Cell {
  x: number;
  y: number;
}

export interface ResourceChip {
  name: string;
  current: number;
  max: number;
}

export interface ConditionDot {
  id: string;
}

export interface TokenDisc {
  tokenId: string;
  actorId: string;
  /** Single character shown on the disc. */
  initial: string;
  name: string;
  x: number;
  y: number;
  resources: ResourceChip[];
  conditions: ConditionDot[];
}

/**
 * cellFromPoint converts a click in board pixels to a cell.
 *
 * Floor, not round: rounding would make the right half of every cell select
 * its neighbour, so a player aiming at a token would move past it. Clamped,
 * because a click in the margin must land on the board rather than produce a
 * negative coordinate the server rejects with a confusing error.
 */
export function cellFromPoint(px: number, py: number, geom: Geometry): Cell {
  const clamp = (v: number, hi: number) => Math.min(Math.max(v, 0), hi);
  return {
    x: clamp(Math.floor(px / geom.cell), geom.width - 1),
    y: clamp(Math.floor(py / geom.cell), geom.height - 1),
  };
}

/**
 * tokensOnScene returns what the board draws for one scene.
 *
 * ALL resources and ALL conditions are included (client spec §4). Choosing a
 * "primary" resource to feature would need ruleset client-hints the format
 * does not have (§9), and guessing which one matters is precisely the genre
 * assumption this platform refuses to make.
 *
 * VISIBLE IS AN INPUT, AND THAT IS THE POINT (visibility spec §6/§6.1). A
 * creature is pure line of sight — you remember the room, not the goblin
 * standing in it — so a token on ground this seat merely REMEMBERS produces no
 * disc AT ALL, rather than a disc something downstream declines to paint.
 * Passing the set in rather than deriving it here is what keeps the decision
 * out of the painting layer, which is where a leak would hide; RPTool reached
 * this same seam, published a visible-token set from it, and then handed its
 * renderer the full list anyway (spec §6.1). This is that migration finished.
 *
 * UNDEFINED IS NOT AN EMPTY SET. Undefined means this stream carries no
 * projection at all — the DM's and the agent's, which are the identity
 * function (spec §3.1) — and every token is drawn. `{}` means a projection
 * arrived and this seat can currently see nothing, and NOTHING is drawn. The
 * parameter is required rather than optional so no call site can slide into
 * the draw-everything answer by omission; state.ts's Scene.Visible is what to
 * pass, and it carries exactly that distinction.
 */
export function tokensOnScene(
  st: State,
  sceneId: string,
  visible: Record<string, boolean> | undefined,
): TokenDisc[] {
  const discs: TokenDisc[] = [];

  for (const tok of Object.values(st.Tokens)) {
    if (tok.SceneID !== sceneId) continue;
    // Keyed like Tiles/Explored/Visible: column,row (maps-as-geometry §4.1).
    if (visible !== undefined && !visible[`${tok.X},${tok.Y}`]) continue;
    const actor = st.Actors[tok.ActorID];

    // Resources are sorted by name: st.Actors[].resources is a plain object
    // and its key order is not something to render a stable UI from.
    const resources: ResourceChip[] = actor
      ? Object.keys(actor.resources)
          .sort()
          .map((name) => ({
            name,
            current: actor.resources[name]!.current,
            max: actor.resources[name]!.max,
          }))
      : [];

    // Conditions keep APPLIED order — the order the table saw them happen.
    // The fold preserves it deliberately, so the view must not re-sort.
    const conditions: ConditionDot[] = (st.Conditions[tok.ActorID] ?? []).map((c) => ({ id: c.ID }));

    const name = actor?.name ?? "";
    discs.push({
      tokenId: tok.ID,
      actorId: tok.ActorID,
      // A blank disc is unreadable, so an unnamed actor falls back to its id.
      // Array indexing rather than [0] to keep a multi-byte first character
      // (an emoji, a non-Latin letter) intact instead of splitting it.
      initial: ([...(name || tok.ActorID || "?")][0] ?? "?").toUpperCase(),
      name,
      x: tok.X,
      y: tok.Y,
      resources,
      conditions,
    });
  }

  // Stable draw order, so a re-render never reshuffles the board.
  return discs.sort((a, b) => (a.tokenId < b.tokenId ? -1 : a.tokenId > b.tokenId ? 1 : 0));
}
