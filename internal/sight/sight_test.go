package sight_test

import (
	"fmt"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/sight"
)

// room builds a 5x3 scene: solid wall ring, floor inside, a door at 2,0.
//
//	 x: 0    1    2    3    4
//	y=0 wall wall door wall wall
//	y=1 wall floor floor floor wall
//	y=2 wall wall wall wall wall
func room() engine.Scene {
	tiles := map[string]engine.Tile{
		"0,0": {Kind: "wall"}, "1,0": {Kind: "wall"}, "2,0": {Kind: "door"},
		"3,0": {Kind: "wall"}, "4,0": {Kind: "wall"},
		"0,1": {Kind: "wall"}, "1,1": {Kind: "floor"}, "2,1": {Kind: "floor"},
		"3,1": {Kind: "floor"}, "4,1": {Kind: "wall"},
		"0,2": {Kind: "wall"}, "1,2": {Kind: "wall"}, "2,2": {Kind: "wall"},
		"3,2": {Kind: "wall"}, "4,2": {Kind: "wall"},
	}
	return engine.Scene{
		ID: "room", GridWidth: 5, GridHeight: 3,
		Tiles: tiles, OpenDoors: map[string]bool{},
	}
}

func TestAWallBlocksSightAndTheFloorBesideItDoesNot(t *testing.T) {
	sc := room()
	vis := sight.VisibleFrom(sc, 1, 1, 0)

	if !vis["3,1"] {
		t.Error("floor at 3,1 is down an open row from 1,1 and must be visible")
	}
	// 2,2 is wall. A viewer never sees THROUGH it; the wall square itself is
	// visible (you see the face of a wall) but nothing beyond it is.
	if !vis["2,2"] {
		t.Error("the wall face at 2,2 must be visible — you can see a wall")
	}
}

func TestAClosedDoorBlocksAndAnOpenOneDoesNot(t *testing.T) {
	sc := room()
	// Stand in 2,1 — the floor directly inside the door — and look straight
	// out through it. TWO THINGS ABOUT THESE COORDINATES, both learned by
	// getting them wrong first:
	//
	// The destination is PAST the door, not inside it. Clear treats the
	// segment as open at both ends, so a point within the door square is
	// exempt from that door BY CONSTRUCTION and can prove nothing about it.
	// Off-grid is fine: Clear is geometry over blockers, and the grid bounds
	// only ever constrain which squares VisibleFrom enumerates.
	//
	// And the origin is 2,1 rather than 1,1: from 1,1 the ray grazes the
	// corner of the WALL at 1,0, which blocks it whatever the door does.
	// That reading is correct — you genuinely cannot see out of a door you
	// stand diagonally behind — but it makes the door invisible as a cause.
	// The proof that the door is the cause is that ONLY OpenDoors changes
	// between the two assertions below.
	from := [2]float64{2.5, 1.5}
	beyond := [2]float64{2.5, -0.5}
	if got := sight.Clear(from, beyond, sight.Blockers(sc)); got {
		t.Error("a CLOSED door must block the ray through its square")
	}

	sc.OpenDoors["2,0"] = true
	if got := sight.Clear(from, beyond, sight.Blockers(sc)); !got {
		t.Error("an OPEN door must not block")
	}
}

func TestSightIsSymmetric(t *testing.T) {
	sc := room()
	a := sight.VisibleFrom(sc, 1, 1, 0)
	b := sight.VisibleFrom(sc, 3, 1, 0)
	// The agreement below is vacuously true when nothing is visible from
	// anywhere, so pin the value first: a symmetry claim over two falses is
	// satisfied by a VisibleFrom that always returns an empty map.
	if !a["3,1"] {
		t.Fatal("1,1 must see 3,1 down the open row, or the symmetry check below " +
			"is satisfied by a VisibleFrom that sees nothing at all")
	}
	if a["3,1"] != b["1,1"] {
		t.Errorf("asymmetric sight: 1,1 sees 3,1 = %v but 3,1 sees 1,1 = %v",
			a["3,1"], b["1,1"])
	}
}

func TestAnObjectBlocksSightOnlyWhenItSaysSo(t *testing.T) {
	sc := room()
	sc.Objects = []engine.SceneObject{
		{ObjectID: "pillar", Kind: "pillar", X: 2, Y: 1, Width: 1, Height: 1, BlocksSight: true},
	}
	if sight.Clear([2]float64{1.5, 1.5}, [2]float64{3.5, 1.5}, sight.Blockers(sc)) {
		t.Error("a blocks_sight pillar between the two must block")
	}

	sc.Objects[0].BlocksSight = false
	if !sight.Clear([2]float64{1.5, 1.5}, [2]float64{3.5, 1.5}, sight.Blockers(sc)) {
		t.Error("an object that does not block sight must be transparent")
	}
}

func TestSightRangeIsAnInputAndZeroMeansUnlimited(t *testing.T) {
	sc := room()
	near := sight.VisibleFrom(sc, 1, 1, 1)
	if near["3,1"] {
		t.Error("3,1 is two squares away and must be outside a range of 1")
	}
	far := sight.VisibleFrom(sc, 1, 1, 0)
	if !far["3,1"] {
		t.Error("range 0 means unlimited, so 3,1 must be visible")
	}
}

func TestASquareExactlyAtTheSightRangeIsVisible(t *testing.T) {
	// The range is "how far can this creature see", so the square AT that
	// distance is the last one it sees, not the first one it does not.
	// 3,1 is Chebyshev 2 from 1,1.
	if vis := sight.VisibleFrom(room(), 1, 1, 2); !vis["3,1"] {
		t.Error("3,1 is exactly 2 squares away and must be inside a range of 2")
	}
}

func TestSightRangeIsMeasuredOnBothAxesAsADifference(t *testing.T) {
	// Chebyshev distance is max(|ax-bx|, |ay-by|), and BOTH halves are
	// differences: 1,2 is one square north of 1,1, not three. Stated on both
	// axes because they are separate expressions, and a scene walked along one
	// of them cannot tell the other apart.
	vis := sight.VisibleFrom(room(), 1, 1, 1)
	if !vis["2,1"] {
		t.Error("2,1 is one square east of 1,1 and must be inside a range of 1")
	}
	if !vis["1,2"] {
		t.Error("1,2 is one square north of 1,1 and must be inside a range of 1")
	}
}

// ---------------------------------------------------------------------------
// Everything below was written to kill a surviving mutant. Each one states the
// behaviour it pins in its own name and comment; the mutant is the reason it
// exists rather than the thing it asserts.
// ---------------------------------------------------------------------------

func TestATileRecordedOutsideTheGridIsNotABlocker(t *testing.T) {
	// GridWidth/GridHeight define the scene. A tile keyed outside them is not
	// part of it and must cast no shadow inside it — the grid is walked and
	// each square looked up, rather than sc.Tiles being ranged over, and this
	// is the difference between the two.
	sc := engine.Scene{
		ID: "two-by-two", GridWidth: 2, GridHeight: 2,
		Tiles: map[string]engine.Tile{
			"0,0": {Kind: "floor"}, "1,0": {Kind: "floor"},
			"0,1": {Kind: "floor"}, "1,1": {Kind: "floor"},
			"2,0": {Kind: "wall"}, // one column past the east edge
			"0,2": {Kind: "wall"}, // one row past the north edge
		},
		OpenDoors: map[string]bool{},
	}
	if got := sight.Blockers(sc); len(got) != 0 {
		t.Errorf("a 2x2 scene of nothing but floor must have no blockers, got %d: %+v",
			len(got), got)
	}
}

func TestASceneWithNoTerrainRecordedBlocksNothing(t *testing.T) {
	// engine.Scene's own doc comment pins this: Tiles, Objects and OpenDoors
	// may all be nil — a scene created before maps-as-geometry, or one
	// deliberately made without terrain, is what EVERY scene used to be and
	// must keep working. Nothing recorded means nothing solid, and the
	// fail-closed instinct is wrong here: a scene of solid rock would hide a
	// table's whole map the day someone loads an old campaign.
	sc := engine.Scene{ID: "no-terrain", GridWidth: 3, GridHeight: 3}
	if got := sight.Blockers(sc); len(got) != 0 {
		t.Errorf("a scene with no tiles recorded must have no blockers, got %d: %+v",
			len(got), got)
	}
	if vis := sight.VisibleFrom(sc, 1, 1, 0); len(vis) != 9 {
		t.Errorf("every square of a terrain-free 3x3 scene must be visible, got %d: %v",
			len(vis), vis)
	}
}

func TestVisibleFromNamesOnlySquaresInsideTheGrid(t *testing.T) {
	// Open ground, so every square that CAN be named is visible and the set is
	// exactly the grid. A caller keys its own state off these strings; a key
	// for a square the scene does not have is a lie it cannot check.
	sc := engine.Scene{
		ID: "open", GridWidth: 3, GridHeight: 3,
		Tiles:     map[string]engine.Tile{},
		OpenDoors: map[string]bool{},
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			sc.Tiles[key(x, y)] = engine.Tile{Kind: "floor"}
		}
	}

	vis := sight.VisibleFrom(sc, 1, 1, 0)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if !vis[key(x, y)] {
				t.Errorf("%s is open ground in an empty scene and must be visible", key(x, y))
			}
		}
	}
	if len(vis) != 9 {
		t.Errorf("a 3x3 scene has 9 squares, but VisibleFrom named %d: %v", len(vis), vis)
	}
}

func TestAPillarHidesTheSquareBehindItButNotItself(t *testing.T) {
	// The object's footprint runs from its anchor to anchor+size. Getting that
	// arithmetic wrong does not merely move the shadow — an inverted rect
	// swallows the viewer's own eye and blackens the whole scene, so the
	// "still visible" half of this test is the load-bearing one.
	sc := room()
	sc.Objects = []engine.SceneObject{
		{ObjectID: "pillar", Kind: "pillar", X: 2, Y: 1, Width: 1, Height: 1, BlocksSight: true},
	}
	vis := sight.VisibleFrom(sc, 1, 1, 0)
	if vis["3,1"] {
		t.Error("the pillar stands between 1,1 and 3,1, so 3,1 must be hidden")
	}
	if !vis["2,1"] {
		t.Error("the pillar's own square must still be visible — you see the pillar")
	}
}

// eastWestCorridor is one row of floor with a wall at 1,0 and 7,0. A viewer at
// 2,0 stands against the first wall and four squares short of the second, so
// 0,0 and 8,0 are each hidden by the wall immediately beside them.
func eastWestCorridor() engine.Scene {
	sc := engine.Scene{
		ID: "east-west", GridWidth: 9, GridHeight: 1,
		Tiles: map[string]engine.Tile{}, OpenDoors: map[string]bool{},
	}
	for x := 0; x < 9; x++ {
		sc.Tiles[key(x, 0)] = engine.Tile{Kind: "floor"}
	}
	sc.Tiles["1,0"] = engine.Tile{Kind: "wall"}
	sc.Tiles["7,0"] = engine.Tile{Kind: "wall"}
	return sc
}

// northSouthCorridor is eastWestCorridor turned ninety degrees: one column,
// walls at 0,1 and 0,7, viewer at 0,2.
func northSouthCorridor() engine.Scene {
	sc := engine.Scene{
		ID: "north-south", GridWidth: 1, GridHeight: 9,
		Tiles: map[string]engine.Tile{}, OpenDoors: map[string]bool{},
	}
	for y := 0; y < 9; y++ {
		sc.Tiles[key(0, y)] = engine.Tile{Kind: "floor"}
	}
	sc.Tiles["0,1"] = engine.Tile{Kind: "wall"}
	sc.Tiles["0,7"] = engine.Tile{Kind: "wall"}
	return sc
}

func TestAWallHidesWhatIsEastAndWestOfIt(t *testing.T) {
	// THE SAMPLE POINTS ARE THE SUBJECT HERE. A square is visible when any of
	// its centre or four corners can be reached, and every one of those five
	// points must lie INSIDE the square it belongs to. A corner that spills
	// over its own edge lands in the neighbouring wall, and Clear exempts a
	// blocker containing the destination — so a corner one part in a billion
	// too far west would see straight through the wall it is buried in.
	//
	// Hence a wall immediately beside each target: it is the one arrangement
	// where "just inside" and "just outside" give opposite answers.
	vis := sight.VisibleFrom(eastWestCorridor(), 2, 0, 0)

	if vis["0,0"] {
		t.Error("the wall at 1,0 stands between 2,0 and 0,0, so 0,0 must be hidden")
	}
	if vis["8,0"] {
		t.Error("the wall at 7,0 stands between 2,0 and 8,0, so 8,0 must be hidden")
	}
	// Not vacuous: the corridor between the walls is open and must be seen,
	// including the far wall's own face four squares away.
	for _, k := range []string{"1,0", "2,0", "3,0", "6,0", "7,0"} {
		if !vis[k] {
			t.Errorf("%s is between the two walls and must be visible", k)
		}
	}
}

func TestAWallHidesWhatIsNorthAndSouthOfIt(t *testing.T) {
	// The north-south half of the case above. Both axes are asserted because
	// the five sample points are built from two independent coordinates, and a
	// scene laid out along one axis cannot tell the other one apart.
	vis := sight.VisibleFrom(northSouthCorridor(), 0, 2, 0)

	if vis["0,0"] {
		t.Error("the wall at 0,1 stands between 0,2 and 0,0, so 0,0 must be hidden")
	}
	if vis["0,8"] {
		t.Error("the wall at 0,7 stands between 0,2 and 0,8, so 0,8 must be hidden")
	}
	for _, k := range []string{"0,1", "0,2", "0,3", "0,6", "0,7"} {
		if !vis[k] {
			t.Errorf("%s is between the two walls and must be visible", k)
		}
	}
}

func TestAnEndpointOnABlockersEdgeCountsAsInsideIt(t *testing.T) {
	// Clear's open-end rule reaches the blocker's BOUNDARY, not just its
	// interior: standing flush against a wall, or looking at the exact face of
	// one, must not blind you. Each case below puts an endpoint on one of the
	// four edges and approaches it from outside.
	blocker := []sight.Rect{{MinX: 2, MinY: 1, MaxX: 3, MaxY: 2}}
	cases := []struct {
		edge     string
		from, to [2]float64
	}{
		{"west face", [2]float64{1.5, 1.5}, [2]float64{2, 1.5}},
		{"east face", [2]float64{3.5, 1.5}, [2]float64{3, 1.5}},
		{"south face", [2]float64{2.5, 0.5}, [2]float64{2.5, 1}},
		{"north face", [2]float64{2.5, 2.5}, [2]float64{2.5, 2}},
	}
	for _, c := range cases {
		if !sight.Clear(c.from, c.to, blocker) {
			t.Errorf("%s: a ray ending exactly ON the blocker's edge must reach it — "+
				"%v to %v was reported blocked", c.edge, c.from, c.to)
		}
	}
}

func TestARaySlidingAlongAWallFaceIsBlocked(t *testing.T) {
	// The mirror of the case above, and the reason that one is stated as
	// ENDPOINTS rather than as "anything touching the edge is free": a ray that
	// runs along the face and CONTINUES PAST it has not merely touched the
	// wall, it has travelled the length of it. Both faces, because the two
	// bounds are separate comparisons.
	blocker := []sight.Rect{{MinX: 2, MinY: 1, MaxX: 3, MaxY: 2}}
	for _, c := range []struct {
		face     string
		from, to [2]float64
	}{
		{"west", [2]float64{2, 0.5}, [2]float64{2, 3}},
		{"east", [2]float64{3, 0.5}, [2]float64{3, 3}},
	} {
		if sight.Clear(c.from, c.to, blocker) {
			t.Errorf("%s face: a ray running the whole length of the wall's face and out "+
				"the other side must be blocked — %v to %v was reported clear",
				c.face, c.from, c.to)
		}
	}
}

func TestARayGrazingAWallCornerIsBlocked(t *testing.T) {
	// A ray that touches a wall at exactly one point still touches it. This is
	// the case the FIRST draft of the door test above tripped over: from 1,1
	// towards the door's centre the ray passes through 2,1 — the corner of the
	// wall at 1,0 — and is stopped there, by the wall, not by the door.
	//
	// Treating a tangent as clear is the classic way sight leaks diagonally
	// between two walls that meet at a corner, which is exactly what a player
	// hiding behind that corner is relying on.
	if sight.Clear([2]float64{1.5, 1.5}, [2]float64{2.5, 0.5}, sight.Blockers(room())) {
		t.Error("the ray from 1.5,1.5 to 2.5,0.5 touches the corner of the wall at 1,0 " +
			"and must be blocked by it")
	}
}

func TestANearlyParallelRayIsStillClippedByItsSlab(t *testing.T) {
	// segmentHitsRect treats a ray as parallel to an axis when its drift along
	// that axis is under 1e-12, and skips clipping against that axis. The
	// comparison is STRICT: a drift of exactly 1e-12 is a real direction and
	// must be clipped like any other.
	//
	// The difference is observable rather than academic. This ray leaves the
	// blocker's x-range halfway along, well before it reaches the blocker's
	// y-range at 0.7 — so it misses. Skip the x-clip and it hits.
	blocker := []sight.Rect{{MinX: -1, MinY: 70, MaxX: 5e-13, MaxY: 90}}
	if !sight.Clear([2]float64{0, 0}, [2]float64{1e-12, 100}, blocker) {
		t.Error("a ray drifting exactly 1e-12 in x is NOT parallel to y: it leaves the " +
			"blocker's x-range at t=0.5 and never reaches its y-range at t=0.7")
	}
}

// key formats a square the way the wire and this package both do. The
// production copy is unexported, and a test that reached for it would stop
// being a test of the boundary.
func key(x, y int) string { return fmt.Sprintf("%d,%d", x, y) }
