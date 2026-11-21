import { doorKey } from "../fold";
import type { Scene } from "../state";

// VTT-937: keep in step with engine.State.Blocked's arms inside the grid
// (internal/engine/terrain.go) and contract/testdata/blocked_squares.json.
/** blocksMove: the window holds a wall, a closed door or blocking scenery there (VTT-936). */
export function blocksMove(scene: Scene, cell: { x: number; y: number }): boolean {
  const key = doorKey(cell.x, cell.y);
  const kind = scene.Tiles?.[key]?.Kind;
  if (kind === "wall") return true;
  if (kind === "door" && scene.OpenDoors?.[key] !== true) return true;
  return (scene.Objects ?? []).some((o) => o.BlocksMove
    && cell.x >= o.X && cell.x < o.X + o.Width && cell.y >= o.Y && cell.y < o.Y + o.Height);
}
