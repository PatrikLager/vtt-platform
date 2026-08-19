package sight_test

import (
	"fmt"
	"reflect"
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
	vis := sight.VisibleFrom(sc, 1, 1, 0, 0)

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

// oneWall is spec §3.3.1's counterexample scene: a 3x3 of open floor with a
// single wall at 1,0.
func oneWall() engine.Scene {
	sc := openGrid(3, 3)
	sc.ID = "one-wall"
	sc.Tiles["1,0"] = engine.Tile{Kind: "wall"}
	return sc
}

func TestSightIsNotSymmetric(t *testing.T) {
	// THIS TEST EXISTS TO STOP SOMEBODY "FIXING" IT. Spec §7 asks for the
	// counterexample asserted out loud, because an earlier draft of §3.3.1 made
	// symmetry a keystone property and it is false.
	//
	// The cause is structural, not rounding: the VIEWER is sampled at one point
	// (its centre) while the TARGET is sampled at nine, and one-against-many is
	// not a symmetric relation whatever the geometry. Making it symmetric would
	// mean sampling the viewer at nine points too — which is a different design
	// and, per §3.3.1, the wrong one: MapTool has shipped this exact asymmetry
	// for two decades, and its one-directional Hill/Pit VBL is how elevation
	// gets modelled later with no coordinate system at all. A symmetric
	// predicate cannot express a hill.
	sc := oneWall()
	from00 := sight.VisibleFrom(sc, 0, 0, 0, 0)
	from21 := sight.VisibleFrom(sc, 2, 1, 0, 0)

	if from00["2,1"] {
		t.Error("from 0,0 the square 2,1 must NOT be visible — the wall at 1,0 " +
			"covers all nine of its sample points")
	}
	if !from21["0,0"] {
		t.Error("from 2,1 the square 0,0 MUST be visible — and that it is, while " +
			"the reverse is not, is exactly the asymmetry this test pins")
	}
}

func TestToleranceIsAnInputAndNineDemandsFullExposure(t *testing.T) {
	// Tolerance says how many of the nine sample points must be reachable
	// before a square counts as seen (§3.3.1). It is an INPUT in the same sense
	// rangeSquares is: 1 means a sliver of exposure reveals you, 9 means you
	// must be fully in the open, and which it should be is a rules question the
	// platform must not answer.
	//
	// 1,1 sits diagonally behind the wall at 1,0 and is PARTIALLY exposed from
	// 0,0 — some of its nine points are reachable and some are not, which is
	// the only fixture on which a tolerance can be observed at all.
	sc := oneWall()
	if !sight.VisibleFrom(sc, 0, 0, 0, 1)["1,1"] {
		t.Error("tolerance 1: one reachable point is enough, so 1,1 must be visible")
	}
	if sight.VisibleFrom(sc, 0, 0, 0, 9)["1,1"] {
		t.Error("tolerance 9: 1,1 is only partly exposed from 0,0, so demanding all " +
			"nine points must hide it")
	}
	// Not vacuous — tolerance 9 must not hide EVERYTHING. 0,1 is straight north
	// of the viewer with nothing between, so all nine of its points are clear.
	if !sight.VisibleFrom(sc, 0, 0, 0, 9)["0,1"] {
		t.Error("tolerance 9: 0,1 is fully in the open and must stay visible")
	}
}

func TestToleranceAtOrBelowZeroMeansAnySinglePoint(t *testing.T) {
	// The default keeps the behaviour this package shipped with: absent a rules
	// answer, a sliver of exposure reveals you. Both 0 and a negative are
	// spelled out because "<= 0" is one comparison and a test of only 0 leaves
	// `== 0` passing.
	sc := oneWall()
	want := sight.VisibleFrom(sc, 0, 0, 0, 1)["1,1"]
	if !want {
		t.Fatal("premise: 1,1 must be visible at tolerance 1, or this test proves nothing")
	}
	for _, tol := range []int{0, -1, -9} {
		if !sight.VisibleFrom(sc, 0, 0, 0, tol)["1,1"] {
			t.Errorf("tolerance %d must behave as 1, so 1,1 must be visible", tol)
		}
	}
}

func TestANegativeSightRangeIsUnlimited(t *testing.T) {
	// Both doc comments say `rangeSquares <= 0` is unlimited, but only 0 was
	// ever asserted — an implementation written `rangeSquares != 0` passed the
	// whole suite while treating -1 as a range of minus one square.
	if vis := sight.VisibleFrom(room(), 1, 1, -1, 0); !vis["3,1"] {
		t.Error("a negative sight range means unlimited, so 3,1 must be visible")
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
	near := sight.VisibleFrom(sc, 1, 1, 1, 0)
	if near["3,1"] {
		t.Error("3,1 is two squares away and must be outside a range of 1")
	}
	far := sight.VisibleFrom(sc, 1, 1, 0, 0)
	if !far["3,1"] {
		t.Error("range 0 means unlimited, so 3,1 must be visible")
	}
}

func TestASquareExactlyAtTheSightRangeIsVisible(t *testing.T) {
	// The range is "how far can this creature see", so the square AT that
	// distance is the last one it sees, not the first one it does not.
	// 3,1 is Chebyshev 2 from 1,1.
	if vis := sight.VisibleFrom(room(), 1, 1, 2, 0); !vis["3,1"] {
		t.Error("3,1 is exactly 2 squares away and must be inside a range of 2")
	}
}

func TestSightRangeIsMeasuredOnBothAxesAsADifference(t *testing.T) {
	// Chebyshev distance is max(|ax-bx|, |ay-by|), and BOTH halves are
	// differences: 1,2 is one square north of 1,1, not three. Stated on both
	// axes because they are separate expressions, and a scene walked along one
	// of them cannot tell the other apart.
	vis := sight.VisibleFrom(room(), 1, 1, 1, 0)
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

func TestBlockersComeBackInGridOrder(t *testing.T) {
	// The SECOND reason Blockers walks the grid rather than ranging over
	// sc.Tiles, and the one that has no other test: Go randomises map
	// iteration, so a Blockers built from a map range would return the same
	// SET in a different ORDER on every run. Row-major, column within row,
	// then objects in scene order.
	//
	// Nothing in this package's own answers depends on that — Clear consults
	// every blocker — so this is the assertion that keeps the claim in
	// Blockers' doc comment true rather than aspirational.
	sc := room()
	sc.Objects = []engine.SceneObject{
		{ObjectID: "pillar", Kind: "pillar", X: 2, Y: 1, Width: 1, Height: 1, BlocksSight: true},
	}
	want := []sight.Rect{
		// y=0: four walls and the CLOSED door at 2,0.
		{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1}, {MinX: 1, MinY: 0, MaxX: 2, MaxY: 1},
		{MinX: 2, MinY: 0, MaxX: 3, MaxY: 1}, {MinX: 3, MinY: 0, MaxX: 4, MaxY: 1},
		{MinX: 4, MinY: 0, MaxX: 5, MaxY: 1},
		// y=1: the two side walls; the floor between them is not a blocker.
		{MinX: 0, MinY: 1, MaxX: 1, MaxY: 2}, {MinX: 4, MinY: 1, MaxX: 5, MaxY: 2},
		// y=2: the back wall.
		{MinX: 0, MinY: 2, MaxX: 1, MaxY: 3}, {MinX: 1, MinY: 2, MaxX: 2, MaxY: 3},
		{MinX: 2, MinY: 2, MaxX: 3, MaxY: 3}, {MinX: 3, MinY: 2, MaxX: 4, MaxY: 3},
		{MinX: 4, MinY: 2, MaxX: 5, MaxY: 3},
		// Objects come after every tile, in scene order.
		{MinX: 2, MinY: 1, MaxX: 3, MaxY: 2},
	}
	if got := sight.Blockers(sc); !reflect.DeepEqual(got, want) {
		t.Errorf("Blockers must come back in grid order, tiles then objects\n got %+v\nwant %+v",
			got, want)
	}
}

func TestAnObjectWithNoFootprintBlocksNothing(t *testing.T) {
	// A footprint narrower than one square is NOT merely a bad fixture, though
	// it is also not the unvalidated-input story an earlier version of this
	// comment told: mapdef.CheckObjectFootprints rejects W or H below 1 on
	// every ingest path, create_scene at the gateway included. What is
	// unchecked is REPLAY — the fold copies a stored SceneCreated's objects
	// verbatim — so a log written before that check landed arrives here intact.
	// And sight is a library besides: it cannot see which path built the scene
	// it was handed.
	//
	// engine's covers() asks `x >= X && x < X+Width`, which NO square satisfies
	// once Width < 1 — movement treats such an object as occupying nothing. So
	// sight skips it rather than normalising it into a real rect, because the
	// stated reason Blockers ignores rotation is that sight and movement must
	// not disagree about the same object, and a swapped rect would have sight
	// hiding squares movement walks straight through.
	//
	// Left unhandled this is worse than a wrong shape: MinX > MaxX makes
	// containsPoint unsatisfiable, so the open-endpoint exemption dies and the
	// rect BLINDS whoever stands inside it.
	for _, o := range []engine.SceneObject{
		{ObjectID: "negative", Kind: "glitch", X: 5, Y: 3, Width: -2, Height: -2, BlocksSight: true},
		{ObjectID: "zero", Kind: "glitch", X: 5, Y: 3, Width: 0, Height: 0, BlocksSight: true},
		{ObjectID: "flat-column", Kind: "glitch", X: 5, Y: 3, Width: 0, Height: 4, BlocksSight: true},
		{ObjectID: "flat-row", Kind: "glitch", X: 5, Y: 3, Width: 4, Height: 0, BlocksSight: true},
		{ObjectID: "negative-w", Kind: "glitch", X: 5, Y: 3, Width: -2, Height: 2, BlocksSight: true},
		{ObjectID: "negative-h", Kind: "glitch", X: 5, Y: 3, Width: 2, Height: -2, BlocksSight: true},
	} {
		sc := openGrid(8, 8)
		sc.Objects = []engine.SceneObject{o}

		if got := sight.Blockers(sc); len(got) != 0 {
			t.Errorf("%s (%dx%d): an object covering no square must produce no blocker, got %+v",
				o.ObjectID, o.Width, o.Height, got)
		}
		if vis := sight.VisibleFrom(sc, 0, 0, 0, 0); len(vis) != 64 {
			t.Errorf("%s (%dx%d): open ground must stay wholly visible, got %d of 64 squares",
				o.ObjectID, o.Width, o.Height, len(vis))
		}
	}
}

func TestEveryBlockerIsAWellFormedRect(t *testing.T) {
	// Rect's documented invariant: MinX <= MaxX and MinY <= MaxY. Asserted over
	// every producer path in one place, because an inverted rect does not fail
	// loudly — it quietly disables containsPoint and blinds a viewer standing
	// in it.
	sc := room()
	sc.Objects = []engine.SceneObject{
		{ObjectID: "pillar", Kind: "pillar", X: 2, Y: 1, Width: 1, Height: 1, BlocksSight: true},
		{ObjectID: "slab", Kind: "slab", X: 1, Y: 1, Width: 3, Height: 2, BlocksSight: true},
		{ObjectID: "glitch", Kind: "glitch", X: 4, Y: 2, Width: -3, Height: -1, BlocksSight: true},
	}
	for _, r := range sight.Blockers(sc) {
		if r.MinX > r.MaxX || r.MinY > r.MaxY {
			t.Errorf("inverted blocker %+v: Rect promises Min <= Max on both axes, and "+
				"containsPoint can never be true for one that breaks it", r)
		}
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
	if vis := sight.VisibleFrom(sc, 1, 1, 0, 0); len(vis) != 9 {
		t.Errorf("every square of a terrain-free 3x3 scene must be visible, got %d: %v",
			len(vis), vis)
	}
}

func TestVisibleFromNamesOnlySquaresInsideTheGrid(t *testing.T) {
	// Open ground, so every square that CAN be named is visible and the set is
	// exactly the grid. A caller keys its own state off these strings; a key
	// for a square the scene does not have is a lie it cannot check.
	sc := openGrid(3, 3)

	vis := sight.VisibleFrom(sc, 1, 1, 0, 0)
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
	vis := sight.VisibleFrom(sc, 1, 1, 0, 0)
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
	sc := openGrid(9, 1)
	sc.ID = "east-west"
	sc.Tiles["1,0"] = engine.Tile{Kind: "wall"}
	sc.Tiles["7,0"] = engine.Tile{Kind: "wall"}
	return sc
}

// northSouthCorridor is eastWestCorridor turned ninety degrees: one column,
// walls at 0,1 and 0,7, viewer at 0,2.
func northSouthCorridor() engine.Scene {
	sc := openGrid(1, 9)
	sc.ID = "north-south"
	sc.Tiles["0,1"] = engine.Tile{Kind: "wall"}
	sc.Tiles["0,7"] = engine.Tile{Kind: "wall"}
	return sc
}

func TestAWallHidesWhatIsEastAndWestOfIt(t *testing.T) {
	// THE SAMPLE POINTS ARE THE SUBJECT HERE. A square is tested at nine points
	// — its centre, its four corners and its four edge midpoints — and every one
	// of the nine must lie INSIDE the square it belongs to. A point that spills
	// over its own edge lands in the neighbouring wall, and Clear exempts a
	// blocker containing the destination — so a point one part in a billion too
	// far west would see straight through the wall it is buried in.
	//
	// Hence a wall immediately beside each target: it is the one arrangement
	// where "just inside" and "just outside" give opposite answers.
	vis := sight.VisibleFrom(eastWestCorridor(), 2, 0, 0, 0)

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
	// The north-south half of the case above. Both axes are asserted because the
	// nine sample points are built from two independent coordinates, and a scene
	// laid out along one axis cannot tell the other one apart.
	vis := sight.VisibleFrom(northSouthCorridor(), 0, 2, 0, 0)

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

// openGrid is w by h squares of nothing but floor: no walls, no doors, no
// objects, so every blocker in a scene built from it is one the test put there.
func openGrid(w, h int32) engine.Scene {
	sc := engine.Scene{
		ID: "open", GridWidth: w, GridHeight: h,
		Tiles: map[string]engine.Tile{}, OpenDoors: map[string]bool{},
	}
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			sc.Tiles[key(int(x), int(y))] = engine.Tile{Kind: "floor"}
		}
	}
	return sc
}
