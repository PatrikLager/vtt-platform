// Grid geometry and the data a token disc renders from.
//
// Kept free of the DOM on purpose: the arithmetic is where the bugs live (an
// off-by-one cell sends a player's move somewhere they did not click), and it
// is testable without a browser.

import type { State } from "../state";
import { initialOf, type Namer } from "./naming";

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
  /** label is what the chip's hover names it (VTT-815, VTT-816). */
  label: string;
  current: number;
  max: number;
}

export interface ConditionDot {
  id: string;
  label: string;
}

export interface TokenDisc {
  tokenId: string;
  actorId: string;
  /** Single character shown on the disc. */
  initial: string;
  name: string;
  /** What a player's window names the token, for its card. */
  shown: string;
  /** The token's notes, for its card. */
  notes: string;
  /** The token's GM notes, which only a DM's or an agent's card shows. */
  gmNotes: string;
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
 * tokensOnScene returns what the board draws for one scene: a disc's resources
 * only in a DM's or an agent's window, its conditions in every window (VTT-834,
 * VTT-835, SPEC-022). Pass Scene.Visible as it is and never default it with
 * `?? {}`: undefined draws every token, {} draws none (state.ts, Scene.Visible).
 */
export function tokensOnScene(
  st: State,
  sceneId: string,
  visible: Record<string, boolean> | undefined,
  names: Namer,
): TokenDisc[] {
  const discs: TokenDisc[] = [];

  for (const tok of Object.values(st.Tokens)) {
    if (tok.SceneID !== sceneId) continue;
    // Keyed like Tiles/Explored/Visible: column,row (maps-as-geometry §4.1).
    if (visible !== undefined && !visible[`${tok.X},${tok.Y}`]) continue;
    const actor = st.Actors[tok.ActorID];

    // Resources are sorted by key: st.Actors[].resources is a dictionary
    // and its key order is not something to render a stable UI from.
    const resources: ResourceChip[] = actor && names.dm
      ? Object.keys(actor.resources)
          .sort()
          .map((name) => ({
            name,
            label: names.resource(name),
            current: actor.resources[name]!.current,
            max: actor.resources[name]!.max,
          }))
      : [];

    // Conditions keep APPLIED order — the order the table saw them happen.
    // The fold preserves it deliberately, so the view must not re-sort.
    const conditions: ConditionDot[] = (st.Conditions[tok.ActorID] ?? [])
      .map((c) => ({ id: c.ID, label: names.conditionTitle(c.ID) }));

    const name = names.token(tok.ID);
    discs.push({
      tokenId: tok.ID,
      actorId: tok.ActorID,
      initial: initialOf(name),
      name,
      shown: names.shown(tok.ID),
      notes: tok.Notes ?? "",
      gmNotes: tok.GMNotes ?? "",
      x: tok.X,
      y: tok.Y,
      resources,
      conditions,
    });
  }

  // Stable draw order, so a re-render never reshuffles the board.
  return discs.sort((a, b) => (a.tokenId < b.tokenId ? -1 : a.tokenId > b.tokenId ? 1 : 0));
}
