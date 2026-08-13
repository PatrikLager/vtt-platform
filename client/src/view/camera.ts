// The camera: a scale-and-offset transform between world pixels (grid index
// times cell size) and screen pixels. Kept free of the DOM and of canvas on
// purpose -- see scene-plan.ts's header for why. Task 9's canvas layer is the
// only code allowed to touch a rendering context; this file only does the
// arithmetic that decides where things land on screen.

export interface Camera {
  scale: number;
  offsetX: number;
  offsetY: number;
}

/**
 * fitCamera scales a scene to fit entirely inside the viewport (never
 * cropped) and centres it on the axis with slack, so a viewport wider or
 * taller than the fitted scene does not pin the map to a corner.
 *
 * The binding dimension is whichever axis needs the smaller scale to fit --
 * scaling by the larger one would push the other axis past the viewport,
 * which is the T1/#19 defect (spec §1.4) this whole file exists to close.
 */
export function fitCamera(
  sceneW: number,
  sceneH: number,
  cell: number,
  viewW: number,
  viewH: number,
): Camera {
  const worldW = sceneW * cell;
  const worldH = sceneH * cell;
  const scale = Math.min(viewW / worldW, viewH / worldH);
  return {
    scale,
    offsetX: (viewW - worldW * scale) / 2,
    offsetY: (viewH - worldH * scale) / 2,
  };
}

/**
 * worldFromScreen undoes the camera's scale and offset, returning WORLD
 * PIXELS -- the same units `cell` is measured in, not a grid cell index.
 *
 * It deliberately stops there. `cellFromPoint` (grid.ts:52) already divides
 * a point by `geom.cell` to find the cell underneath it; this function is the
 * camera transform that composes in front of that call, not a second place
 * that also knows about cell size. Two functions computing the same division
 * is exactly how they drift.
 */
export function worldFromScreen(px: number, py: number, cam: Camera): { x: number; y: number } {
  return { x: (px - cam.offsetX) / cam.scale, y: (py - cam.offsetY) / cam.scale };
}
