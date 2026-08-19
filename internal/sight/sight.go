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
//
// SO IS TOLERANCE (spec §3.3.1, Patrik 2026-08-19: "keep the asymmetry and use
// the nine points"). How much of a square must be exposed before it counts as
// seen — one of nine sample points, or all nine — is the same kind of rules
// fact, and for the same reason: a platform that picked a number would be
// deciding how much cover a wall gives. tolerance <= 0 means unlimited's
// counterpart, "not supplied", and behaves as 1.
//
// Sight is NOT SYMMETRIC and that is deliberate. See VisibleFrom.
package sight

import (
	"fmt"
	"math"

	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Rect is a blocker's extent in CONTINUOUS square coordinates: square (x,y)
// spans x..x+1, y..y+1.
//
// INVARIANT: MinX <= MaxX and MinY <= MaxY. Blockers guarantees it, and Clear
// requires it. An inverted rect does not merely describe the wrong region — it
// makes containsPoint UNSATISFIABLE (no p is both >= 5 and <= 3), which kills
// Clear's open-endpoint exemption while segmentHitsRect still treats the extent
// as real, so the rect blinds anyone standing inside it. That is a silent
// failure, not a visible one, which is why the invariant is stated here rather
// than left as an obvious property of the word "Min".
//
// float64 rather than int32 deliberately (spec §3.5, "squares now, fractional
// later"). Every rect this arc builds is whole-square ALIGNED — its bounds land
// on integers — but not necessarily one square: an object's footprint is
// Width x Height and both may exceed 1. When a later arc adds sub-square
// extents it hands this package narrower rects and NOTHING HERE CHANGES, which
// is the whole reason the seam is the coordinate type rather than an interface
// with a single implementation.
//
// The earlier version of this comment justified float64 by claiming the wire
// "cannot express a narrower trunk". It can — SceneObject.Width/Height are
// plain int32 with no declared minimum, and only mapdef's map-FILE loader
// refuses a sub-1x1 footprint. The conclusion survives its wrong premise: the
// seam is worth having whether the format reaches it today or not.
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
// AN OBJECT COVERING NO SQUARE IS SKIPPED — Width < 1 or Height < 1 — and that
// is the same rule read off engine's covers(), which asks `x >= X && x < X+W`
// and so admits no square at all once W is 0 or negative. Movement treats such
// an object as occupying nothing; sight agrees, for the identical reason the
// rotation ruling above gives.
//
// NOT normalised into a real rect by swapping the bounds, which was the other
// option. Swapping would have sight hide squares that movement lets you walk
// through — the precise disagreement this doc comment already forbids two
// paragraphs up. And this is reachable from the WIRE, not just from a bad
// fixture: width/height are plain int32 with no minimum, the gateway passes
// CreateScene.Objects through unvalidated, the fold copies them verbatim, and
// create_scene is advertised to MCP. A zero-width object is the sharper case —
// it looks harmless and casts a shadow LINE across the map.
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
		if o.Width < 1 || o.Height < 1 {
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
// Rays run from the viewer's CENTRE — one point — to NINE points on each
// target: its four corners, its four edge midpoints, and its centre (spec
// §3.3.1). tolerance says how many of those nine must be reachable before the
// square counts as seen.
//
// TOLERANCE IS AN INPUT, in exactly the sense rangeSquares is (§3.4). 1 means a
// sliver of exposure reveals you; 9 means you must be fully in the open; which
// it should be is a rules question and the platform must not answer it.
// tolerance <= 0 means "not supplied" and behaves as 1, which is the behaviour
// this package shipped with. A tolerance above nine is not clamped and simply
// cannot be met — that is the caller asking for more exposure than a square
// has, and answering "nothing is visible" is the literal truth rather than a
// failure mode worth hiding.
//
// SIGHT IS DELIBERATELY NOT SYMMETRIC. If A sees B it does not follow that B
// sees A. Spec §7 pins the counterexample rather than assuming it away, because
// an earlier draft of the spec made symmetry a keystone property and was wrong:
//
//	3x3 of open floor, one wall at 1,0.
//	  from 0,0 the square 2,1 is NOT visible
//	  from 2,1 the square 0,0 IS     visible
//
// The cause is structural, not rounding — one point at the viewer against nine
// at the target is not a symmetric relation, whatever the geometry.
//
// KEPT, NOT FIXED, for two reasons. MapTool has shipped this exact shape for
// two decades, so asymmetry is not a defect that sinks a virtual tabletop. And
// symmetry would foreclose something this design wants later: MapTool's Hill
// and Pit VBL are deliberately ONE-DIRECTIONAL — outside a hill you see into it
// but not beyond, inside it you see out — which is cover AND vantage with no
// coordinate system at all, and it is how elevation gets modelled once §2's
// exclusion of it is lifted. A symmetric predicate cannot express a hill.
//
// rangeSquares <= 0 is unlimited (see the package comment). The distance is
// Chebyshev — the same "8-neighbour" notion mayWorkDoor already uses for
// door adjacency (internal/gateway/authz.go), so two spatial rules in this
// codebase do not disagree about what "one square away" means.
func VisibleFrom(sc engine.Scene, ox, oy int32, rangeSquares int32, tolerance int) map[string]bool {
	want := tolerance
	if want <= 0 {
		want = 1
	}
	blockers := Blockers(sc)
	eye := [2]float64{float64(ox) + 0.5, float64(oy) + 0.5}
	vis := map[string]bool{}

	for y := int32(0); y < sc.GridHeight; y++ {
		for x := int32(0); x < sc.GridWidth; x++ {
			if rangeSquares > 0 && chebyshev(ox, oy, x, y) > rangeSquares {
				continue
			}
			reached := 0
			for _, p := range samplePoints(x, y) {
				if Clear(eye, p, blockers) {
					reached++
				}
			}
			if reached >= want {
				vis[squareKey(x, y)] = true
			}
		}
	}
	return vis
}

// samplesPerSquare is the count spec §3.3.1 fixes, and it is a COUNT rather
// than a length: tolerance is read against it, so the two must not drift.
const samplesPerSquare = 9

// samplePoints is MapTool's nine: four corners, four edge midpoints, one
// centre.
//
// EVERY ONE IS PULLED INSIDE THE SQUARE by e, corners and edge midpoints alike,
// and that is load-bearing rather than tidy. A point sitting exactly ON the
// square's boundary also sits on the NEIGHBOUR's boundary, and containsPoint
// treats a blocker's edge as inside it — so a sample that spills over would be
// exempted from the very wall it had landed in and would see straight through
// it. The edge midpoints need this as much as the corners do: an un-inset south
// midpoint lies on the floor of the square below.
func samplePoints(x, y int32) [samplesPerSquare][2]float64 {
	fx, fy := float64(x), float64(y)
	const e = 1e-9
	return [samplesPerSquare][2]float64{
		{fx + 0.5, fy + 0.5},
		// corners
		{fx + e, fy + e}, {fx + 1 - e, fy + e},
		{fx + e, fy + 1 - e}, {fx + 1 - e, fy + 1 - e},
		// edge midpoints
		{fx + 0.5, fy + e}, {fx + 0.5, fy + 1 - e},
		{fx + e, fy + 0.5}, {fx + 1 - e, fy + 0.5},
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
