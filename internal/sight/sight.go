// Package sight answers one question: from where a creature stands, which
// squares can it see?
//
// PURE GEOMETRY, and deliberately nothing else. It takes an engine.Scene and
// returns square keys; it knows nothing about participants, connections or
// the wire. That mirrors engine.State.Blocked (internal/engine/terrain.go),
// which is a derived spatial QUERY rather than folded state — visibility is
// computed on demand, never stored, and CLAUDE.md rule 4's one-fold
// invariant is untouched.
//
// SIGHT RANGE IS AN INPUT (spec §3.4, Patrik 2026-08-18: "this should not be
// driven by the engine. It should be input, to the engine"). This package
// only ever asks whether something blocks a ray. How far a creature sees is
// a rules fact supplied by its caller; rangeSquares <= 0 means unlimited.
package sight

import (
	"fmt"
	"math"

	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Rect is a blocker's extent in CONTINUOUS square coordinates: square (x,y)
// spans x..x+1, y..y+1.
//
// float64 rather than int32 deliberately (spec §3.5, "squares now, fractional
// later"). Every rect this arc builds is exactly one square, because
// SceneObject.Width/Height are int32 with a 1x1 minimum and the format cannot
// express a narrower trunk. When a later arc adds sub-square extents, it
// hands this package smaller rects and NOTHING HERE CHANGES — that is the
// whole reason the seam is the coordinate type rather than an interface with
// a single implementation.
type Rect struct{ MinX, MinY, MaxX, MaxY float64 }

// Blockers is every rect in sc that stops sight.
//
// Walls and CLOSED doors block because they fill their square. An open door
// blocks nothing — the same folded OpenDoors state movement already reads
// (engine.Blocked), so opening a door reveals a room in one event.
//
// Objects block over their footprint when BlocksSight is set. ROTATION IS
// IGNORED, exactly as covers() in internal/engine/terrain.go ignores it for
// movement, and for its stated reason: no spec defines how rotation reshapes
// a footprint, and inventing that transform for sight alone would make sight
// and movement disagree about the same object.
//
// ONLY SQUARES INSIDE THE DECLARED GRID are considered — this walks the grid
// and looks each square up, rather than ranging over sc.Tiles. Two reasons,
// and both are pinned by test: the output order must not depend on Go's
// randomised map iteration, and a tile recorded outside GridWidth/GridHeight
// is not part of the scene, so it must not cast a shadow inside it.
func Blockers(sc engine.Scene) []Rect {
	var out []Rect
	for y := int32(0); y < sc.GridHeight; y++ {
		for x := int32(0); x < sc.GridWidth; x++ {
			key := squareKey(x, y)
			t, ok := sc.Tiles[key]
			if !ok {
				continue
			}
			switch t.Kind {
			case "wall":
				out = append(out, square(x, y))
			case "door":
				if !sc.OpenDoors[key] {
					out = append(out, square(x, y))
				}
			}
		}
	}
	for _, o := range sc.Objects {
		if !o.BlocksSight {
			continue
		}
		out = append(out, Rect{
			MinX: float64(o.X), MinY: float64(o.Y),
			MaxX: float64(o.X + o.Width), MaxY: float64(o.Y + o.Height),
		})
	}
	return out
}

// Clear reports whether the open segment from..to reaches without crossing a
// blocker.
//
// The segment is treated as OPEN at both ends: a blocker containing the
// origin or the destination does not block that ray. Without this you cannot
// see the wall you are standing against, and a creature standing in a
// doorway could not see out of it.
func Clear(from, to [2]float64, blockers []Rect) bool {
	for _, b := range blockers {
		if containsPoint(b, from) || containsPoint(b, to) {
			continue
		}
		if segmentHitsRect(from, to, b) {
			return false
		}
	}
	return true
}

// VisibleFrom is the set of square keys visible from (ox, oy).
//
// A square is visible when ANY ray from the viewer's centre reaches ANY of
// that square's centre or four corners. Centre-to-centre alone is too harsh:
// a creature 90% exposed beside a pillar would vanish, which reads as a bug
// at the table rather than as cover.
//
// rangeSquares <= 0 is unlimited (see the package comment). The distance is
// Chebyshev — the same "8-neighbour" notion mayWorkDoor already uses for
// door adjacency (internal/gateway/authz.go), so two spatial rules in this
// codebase do not disagree about what "one square away" means.
func VisibleFrom(sc engine.Scene, ox, oy int32, rangeSquares int32) map[string]bool {
	blockers := Blockers(sc)
	eye := [2]float64{float64(ox) + 0.5, float64(oy) + 0.5}
	vis := map[string]bool{}

	for y := int32(0); y < sc.GridHeight; y++ {
		for x := int32(0); x < sc.GridWidth; x++ {
			if rangeSquares > 0 && chebyshev(ox, oy, x, y) > rangeSquares {
				continue
			}
			for _, p := range samplePoints(x, y) {
				if Clear(eye, p, blockers) {
					vis[squareKey(x, y)] = true
					break
				}
			}
		}
	}
	return vis
}

func samplePoints(x, y int32) [5][2]float64 {
	fx, fy := float64(x), float64(y)
	const e = 1e-9 // corners pulled inside, so a corner does not sit ON a neighbour's edge
	return [5][2]float64{
		{fx + 0.5, fy + 0.5},
		{fx + e, fy + e}, {fx + 1 - e, fy + e},
		{fx + e, fy + 1 - e}, {fx + 1 - e, fy + 1 - e},
	}
}

func chebyshev(ax, ay, bx, by int32) int32 {
	dx, dy := abs32(ax-bx), abs32(ay-by)
	if dx > dy {
		return dx
	}
	return dy
}

func abs32(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func square(x, y int32) Rect {
	return Rect{MinX: float64(x), MinY: float64(y), MaxX: float64(x + 1), MaxY: float64(y + 1)}
}

func containsPoint(r Rect, p [2]float64) bool {
	return p[0] >= r.MinX && p[0] <= r.MaxX && p[1] >= r.MinY && p[1] <= r.MaxY
}

// segmentHitsRect is the slab test: clip the segment's parameter range
// against each axis and report whether anything survives.
func segmentHitsRect(from, to [2]float64, r Rect) bool {
	t0, t1 := 0.0, 1.0
	d := [2]float64{to[0] - from[0], to[1] - from[1]}
	lo := [2]float64{r.MinX, r.MinY}
	hi := [2]float64{r.MaxX, r.MaxY}

	for i := 0; i < 2; i++ {
		if math.Abs(d[i]) < 1e-12 {
			if from[i] < lo[i] || from[i] > hi[i] {
				return false // parallel to this slab and outside it
			}
			continue
		}
		a := (lo[i] - from[i]) / d[i]
		b := (hi[i] - from[i]) / d[i]
		if a > b {
			a, b = b, a
		}
		if a > t0 {
			t0 = a
		}
		if b < t1 {
			t1 = b
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// squareKey formats a square the way the wire does: column then row, comma
// separated. Must agree with engine's gridKey and mapdef's — three copies of
// this format already exist and a fourth that disagrees would be silent.
func squareKey(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }
