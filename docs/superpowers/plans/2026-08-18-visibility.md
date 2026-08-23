# Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A player sees only what their characters can see — on the board and on the wire — so session zero's goblin cannot be found by looking.

**Architecture:** One log, many views. `internal/store` keeps one append-only log and `engine.Apply` stays the only writer; **nothing about visibility enters the fold**. A new pure package `internal/sight` answers "can this viewer see that square", following `engine.State.Blocked`'s precedent as a derived query rather than folded state. `internal/gateway` projects the event stream per connection at the seam where `serve` already holds the participant and already marshals per connection. The client folds its projected stream unchanged, accumulating explored terrain locally.

**Tech Stack:** Go 1.26, protobuf/`vtt.v1` (additive only), TypeScript client under `bun test`, happy-dom (NO canvas).

## Global Constraints

- **ADR-009 airtight TDD.** Tests first; behavioural RED over compile-failure RED wherever a stub can compile; fault-injection proof per load-bearing assertion. No impl-then-test.
- **`task check` is the single gateway.** Never weaken a gate to pass it.
- **ADR-007: contract evolution is additive only.** `check:breaking` enforces. New messages and new field numbers only — never renumber, never remove.
- **CLAUDE.md rule 4 — one fold.** `engine.Apply` via `campaign.foldEvents` remains the only code that changes state. Visibility is DERIVED, never folded.
- **CLAUDE.md rule 5 — no game-system vocabulary in platform code.** Sight RANGE arrives as an input from the actor; the engine only ever asks "does anything block this ray".
- **Fail closed.** When the projection is uncertain it OMITS. A player losing a sighting is a bug; a player gaining one is the defect this arc exists to prevent.
- **The DM and agent receive the identity projection** — byte-for-byte what they receive today.
- **Every visibility test needs a PLAYER seat.** The DM sees everything, so no test exercising a DM can catch a projection bug.
- **`internal/sight` enters `tools/check-mutation.py` PACKAGES in the same commit that creates it** (Task 1). `internal/mapdef` shipped ungated and cost 13 live mutants found late.
- Client tests assert `DrawOp` arrays from the pure planner, **never pixels**.

---

## File Structure

**Create:**
- `internal/sight/sight.go` — the pure sight test. Rays, blockers, visible sets. No I/O, no gateway types.
- `internal/sight/sight_test.go`, `internal/sight/boundary_test.go`
- `internal/gateway/project.go` — the per-recipient projection.
- `internal/gateway/project_test.go`
- `internal/gateway/viewpoint.go` — spectator perch (connection state, not logged).
- `client/src/view/visibility.ts` — pure: turns state + explored into what the planner draws.
- `client/test/visibility.test.ts`, `client/test/projection-parity.test.ts`

**Modify:**
- `contract/vtt/v1/events.proto` — `TokenHidden`, `SceneSeen`, two new `Envelope` oneof fields.
- `contract/vtt/v1/commands.proto` — `SetViewpoint` in the `ClientCommand` oneof.
- `internal/engine/apply.go` — fold arms for the two new events.
- `internal/engine/state.go` — `Explored` on `Scene` (client-mirroring; see Task 4).
- `internal/gateway/server.go` — call the projection in the pump; special-case `SetViewpoint` before `ToEvent`.
- `internal/gateway/authz.go` — one `commandRoles` row for `set_viewpoint`.
- `client/src/fold.ts`, `client/src/state.ts` — the same two arms and `Explored`.
- `client/src/view/scene-plan.ts` — take visible tokens and explored terrain as INPUT.
- `.go-arch-lint.yml` — `sight` component; gateway may depend on it.
- `tools/check-mutation.py`, `tools/coverage-thresholds.txt`.

---

## Task 1: The sight test — pure geometry, gated from birth

**Files:**
- Create: `internal/sight/sight.go`, `internal/sight/sight_test.go`
- Modify: `.go-arch-lint.yml`, `tools/check-mutation.py`, `tools/coverage-thresholds.txt`

**Interfaces:**
- Consumes: `engine.Scene`, `engine.SceneObject`, `engine.Tile` (already exist; see `internal/engine/state.go:29-70`).
- Produces:
  ```go
  type Rect struct{ MinX, MinY, MaxX, MaxY float64 }
  func Blockers(sc engine.Scene) []Rect
  func Clear(from, to [2]float64, blockers []Rect) bool
  func VisibleFrom(sc engine.Scene, ox, oy int32, rangeSquares int32) map[string]bool
  ```

`Rect` is **float64 deliberately** — spec §3.5's "squares now, fractional later". This arc only ever builds whole-square rects; a later arc builds narrower ones and nothing else changes.

`rangeSquares <= 0` means unlimited (spec §3.4: sight range is an input; absent means unobstructed).

- [ ] **Step 1: Write the failing test**

```go
package sight_test

import (
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
	// The square beyond the door, outside the room, is off-grid here; use the
	// door square itself as the observable: closed, it blocks what is past it.
	// Put a second floor beyond by widening: see TestSightThroughADoorway.
	if got := sight.Clear([2]float64{1.5, 1.5}, [2]float64{2.5, 0.5}, sight.Blockers(sc)); got {
		t.Error("a CLOSED door must block the ray through its square")
	}

	sc.OpenDoors["2,0"] = true
	if got := sight.Clear([2]float64{1.5, 1.5}, [2]float64{2.5, 0.5}, sight.Blockers(sc)); !got {
		t.Error("an OPEN door must not block")
	}
}

func TestSightIsSymmetric(t *testing.T) {
	sc := room()
	a := sight.VisibleFrom(sc, 1, 1, 0)
	b := sight.VisibleFrom(sc, 3, 1, 0)
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sight/ -run Test -v`
Expected: FAIL — `no required module provides package .../internal/sight`.

- [ ] **Step 3: Write the implementation**

```go
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
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/sight/ -count=1`
Expected: PASS.

- [ ] **Step 5: Gate the package IN THIS COMMIT**

In `.go-arch-lint.yml`, add to `components:` (near `mapdef: { in: internal/mapdef }`, line ~82):

```yaml
  sight:    { in: internal/sight }
```

and to `deps:`:

```yaml
  sight:    { mayDependOn: [sight, engine] }
```

and add `sight` to gateway's list (line ~126):

```yaml
  gateway:  { mayDependOn: [contract, campaign, engine, identity, gateway, rules, adventure, mapdef, sight] }
```

In `tools/check-mutation.py`, add to `PACKAGES` (ascending by runtime, so near the top):

```python
    # internal/sight (visibility): pure geometry with no I/O — exactly what a
    # mutation gate is best at, and gated from its FIRST commit. internal/mapdef
    # was in neither this list nor mutation-scope.md, so it shipped silently
    # ungated; adding it later found 13 live mutants and one real bug.
    "./internal/sight/",
```

In `tools/coverage-thresholds.txt`, add `internal/sight 95.0`.

- [ ] **Step 6: Verify the gates**

```bash
task check:arch && go vet ./internal/sight/ && task check:coverage
go tool gremlins unleash ./internal/sight/ 2>&1 | tail -5
```
Expected: arch OK, vet silent, coverage passes at the 95.0 floor. Record the mutation result; drive survivors to zero before Step 7 (expect `>=` vs `>` boundary survivors — that is where `mapdef`'s thirteen lived).

- [ ] **Step 7: Commit**

```bash
git add internal/sight .go-arch-lint.yml tools/check-mutation.py tools/coverage-thresholds.txt
git commit -m "The sight test: pure geometry, gated from its first commit"
```

---

## Task 2: Contract — two projection-only events and one unlogged command

**Files:**
- Modify: `contract/vtt/v1/events.proto`, `contract/vtt/v1/commands.proto`
- Test: `contract/` (regenerate + `task check:breaking`)

**Interfaces:**
- Produces: `vttv1.TokenHidden`, `vttv1.SceneSeen`, `vttv1.SetViewpoint`, and `Envelope` oneof fields 28/29, `ClientCommand` oneof field for `set_viewpoint`.

- [ ] **Step 1: Add the messages**

In `contract/vtt/v1/events.proto`, after `SceneCreated`:

```proto
// TokenHidden says a token left YOUR view. PROJECTION-ONLY: no command
// produces it, so it can never reach the log — internal/gateway synthesizes
// it per recipient (visibility spec §4.2). It is a fact about a viewer's
// knowledge, not about the world, which is why the world's log does not
// carry it.
message TokenHidden {
  string token_id = 1;
}

// SceneSeen carries what a viewer can see of a scene RIGHT NOW — the whole
// current visible set, never a delta.
//
// Whole-set is deliberate: it is idempotent, so the projection needs no
// per-connection memory of what it already sent, and visibility spec §4.1's
// purity (a function of log-so-far and viewer, with nothing stored per
// connection) survives. The client UNIONS these into its explored set, which
// is what makes terrain remembered and creatures not.
//
// PROJECTION-ONLY, like TokenHidden.
message SceneSeen {
  string scene_id = 1;
  map<string, TileRef> tiles = 2;
  repeated SceneObject objects = 3;
}
```

and in the `Envelope` oneof, after `actor_control_revoked = 27`:

```proto
    TokenHidden token_hidden = 28;
    SceneSeen scene_seen = 29;
```

In `contract/vtt/v1/commands.proto`, add the message and the oneof arm:

```proto
// SetViewpoint perches a spectator on a party member's shoulder (visibility
// spec §3.1.1, Patrik 2026-08-18: "like a bird hopping from one shoulder to
// another").
//
// APPENDS NOTHING — the same shape as SetJoinDoor, whose handler says so
// outright. Where a spectator points their camera is not a fact about the
// campaign: it is a view preference like zoom, so logging it would replay
// forever, add story-panel noise, and make it RETRACTABLE, letting a DM
// "undo" somebody having looked at Asme. A perch therefore lives on the
// connection and the client re-sends it on reconnect.
//
// An empty actor_id un-perches.
message SetViewpoint {
  string actor_id = 1;
}
```

- [ ] **Step 2: Regenerate and verify additivity**

```bash
task generate:contract
task check:breaking
```
Expected: `check:breaking` exits 0. Generated Go and TS change; `contract/gen` is committed.

- [ ] **Step 3: Commit**

```bash
git add contract/
git commit -m "Contract: a token can leave your view, and a scene can be partly seen"
```

---

## Task 3: The fold learns to forget — both languages

**Files:**
- Modify: `internal/engine/apply.go`, `internal/engine/state.go`, `client/src/fold.ts`, `client/src/state.ts`
- Test: `internal/engine/visibility_fold_test.go`, `client/test/fold.test.ts`

**Interfaces:**
- Consumes: `vttv1.TokenHidden`, `vttv1.SceneSeen` (Task 2).
- Produces: `Scene.Explored map[string]bool` in Go and `Scene.Explored: Record<string, Tile>` in TS; both folds handle the two new arms.

Both folds gain these arms because visibility spec §4.3's keystone folds a PROJECTION and must run in both languages. No log ever contains them.

- [ ] **Step 1: Write the failing Go test**

```go
package engine_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func TestTokenHiddenForgetsOnlyThatToken(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))
	for _, id := range []string{"a1", "a2"} {
		must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: id, Name: id}})))
	}
	must(t, engine.Apply(st, env(4, &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "s", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 0, Y: 0}})))
	must(t, engine.Apply(st, env(5, &vttv1.TokenPlaced{
		TokenId: "t2", SceneId: "s", ActorId: "a2",
		Position: &vttv1.GridPosition{X: 1, Y: 1}})))

	must(t, engine.Apply(st, env(6, &vttv1.TokenHidden{TokenId: "t1"})))

	if _, still := st.Tokens["t1"]; still {
		t.Error("t1 was hidden and must be gone from the viewer's state")
	}
	if _, gone := st.Tokens["t2"]; !gone {
		t.Error("t2 was not hidden and must survive")
	}
}

func TestHidingATokenTwiceIsNotAnError(t *testing.T) {
	// The projection is idempotent by design; a repeated hide must not fail
	// the fold and take a player's client down.
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	if err := engine.Apply(st, env(2, &vttv1.TokenHidden{TokenId: "never-existed"})); err != nil {
		t.Fatalf("hiding an unknown token must be a no-op, got %v", err)
	}
}

func TestSceneSeenUnionsIntoExploredAndNeverShrinks(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})))

	must(t, engine.Apply(st, env(3, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"0,0": {Kind: "floor"}}})))
	must(t, engine.Apply(st, env(4, &vttv1.SceneSeen{SceneId: "s",
		Tiles: map[string]*vttv1.TileRef{"1,1": {Kind: "wall"}}})))

	sc := st.Scenes["s"]
	if !sc.Explored["0,0"] {
		t.Error("0,0 was seen first and must still be explored — memory never shrinks")
	}
	if !sc.Explored["1,1"] {
		t.Error("1,1 was seen second and must be explored")
	}
	if sc.Tiles["1,1"].Kind != "wall" {
		t.Error("a seen tile must land in Tiles so it can be drawn")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/engine/ -run "TokenHidden|SceneSeen|HidingAToken" -v`
Expected: FAIL — `unknown event variant` from `Apply`.

- [ ] **Step 3: Implement the Go arms**

In `internal/engine/state.go`, add to `Scene`:

```go
	// Explored is the squares this VIEWER has ever seen, keyed like Tiles.
	// Client-side memory mirrored here so the same fold runs in both
	// languages (visibility spec §6). It only ever grows: terrain is
	// remembered, creatures are not.
	//
	// EMPTY FOR THE DM AND FOR THE LOG. Nothing in a campaign's log produces
	// SceneSeen, so a Scene folded from the real log has Explored nil — this
	// field is populated only when folding a PROJECTION.
	Explored map[string]bool
```

In `internal/engine/apply.go`, add two arms to the payload switch:

```go
	case *vttv1.Envelope_TokenHidden:
		// PROJECTION-ONLY (visibility spec §4.2): a viewer is being told a
		// token left their view. Deleting an absent token is deliberately NOT
		// an error — the projection is idempotent, and a strict refusal here
		// would take a player's client down over a duplicate, which is the
		// exact failure mode spec §8 names as worst.
		delete(st.Tokens, p.TokenHidden.GetTokenId())
		return nil

	case *vttv1.Envelope_SceneSeen:
		ss := p.SceneSeen
		sc, ok := st.Scenes[ss.GetSceneId()]
		if !ok {
			return fmt.Errorf("engine: scene seen for unknown scene %q", ss.GetSceneId())
		}
		if sc.Tiles == nil {
			sc.Tiles = map[string]Tile{}
		}
		if sc.Explored == nil {
			sc.Explored = map[string]bool{}
		}
		for key, ref := range ss.GetTiles() {
			sc.Tiles[key] = Tile{
				Kind: ref.GetKind(), Material: ref.GetMaterial(), Art: ref.GetArt(),
			}
			sc.Explored[key] = true
		}
		if objs := ss.GetObjects(); len(objs) > 0 {
			sc.Objects = mergeObjects(sc.Objects, objs)
		}
		st.Scenes[ss.GetSceneId()] = sc
		return nil
```

and the helper:

```go
// mergeObjects unions incoming objects into have, replacing by ObjectID.
// SceneSeen carries the whole currently-visible set each time, so the same
// object arrives repeatedly and must not accumulate duplicates.
func mergeObjects(have []SceneObject, incoming []*vttv1.SceneObject) []SceneObject {
	byID := make(map[string]int, len(have))
	for i, o := range have {
		byID[o.ObjectID] = i
	}
	for _, o := range incoming {
		got := SceneObject{
			ObjectID: o.GetObjectId(), Kind: o.GetKind(),
			X: o.GetAt().GetX(), Y: o.GetAt().GetY(),
			Width: o.GetWidth(), Height: o.GetHeight(),
			RotationDegrees: o.GetRotationDegrees(),
			BlocksSight:     o.GetBlocksSight(), BlocksMove: o.GetBlocksMove(),
			Art:             o.GetArt(),
		}
		if i, dup := byID[got.ObjectID]; dup {
			have[i] = got
			continue
		}
		byID[got.ObjectID] = len(have)
		have = append(have, got)
	}
	return have
}
```

- [ ] **Step 4: Run the Go tests**

Run: `go test ./internal/engine/ -count=1`
Expected: PASS.

- [ ] **Step 5: Mirror it in TypeScript**

In `client/src/state.ts`, add `Explored: Record<string, boolean>` to `Scene` and initialise it `{}` wherever a scene is created.

In `client/src/fold.ts`, add the two arms — same semantics, same tolerance:

```ts
    case "tokenHidden": {
      // Projection-only. Deleting an absent token is NOT an error: the
      // projection is idempotent and a strict throw here would take the
      // player's client down over a duplicate.
      delete st.Tokens[p.value.tokenId];
      return;
    }

    case "sceneSeen": {
      const v = p.value;
      const sc = st.Scenes[v.sceneId];
      if (!sc) throw new FoldError(`scene seen for unknown scene "${v.sceneId}"`);
      for (const [key, ref] of Object.entries(v.tiles)) {
        sc.Tiles[key] = { Kind: ref.kind, Material: ref.material, Art: ref.art };
        sc.Explored[key] = true;
      }
      for (const o of v.objects) {
        const i = sc.Objects.findIndex((e) => e.ObjectID === o.objectId);
        const got = {
          ObjectID: o.objectId, Kind: o.kind,
          X: o.at?.x ?? 0, Y: o.at?.y ?? 0,
          Width: o.width, Height: o.height,
          RotationDegrees: o.rotationDegrees,
          BlocksSight: o.blocksSight, BlocksMove: o.blocksMove, Art: o.art,
        };
        if (i >= 0) sc.Objects[i] = got; else sc.Objects.push(got);
      }
      return;
    }
```

- [ ] **Step 6: Run the TS tests**

Run: `bun test client/test`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/engine client/src
git commit -m "Both folds learn to forget a token and to remember a room"
```

---

## Task 4: The projection

**Files:**
- Create: `internal/gateway/project.go`, `internal/gateway/project_test.go`

**Interfaces:**
- Consumes: `sight.VisibleFrom` (Task 1); `vttv1.TokenHidden`, `vttv1.SceneSeen` (Task 2).
- Produces:
  ```go
  type Viewer struct {
      ParticipantID string
      Role          identity.Role
      Viewpoint     string // spectator perch: an actor id, or ""
  }
  type Projector struct{ /* per-connection */ }
  func NewProjector(v Viewer) *Projector
  func (pr *Projector) Project(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope
  ```

`Project` returns what THIS viewer should receive for one log event: zero, one, or several envelopes. `st` is the state AFTER `env` was folded.

- [ ] **Step 1: Write the failing test**

```go
package gateway_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// twoRooms: 7x3, a dividing wall at x=3 with a CLOSED door at 3,1.
//
//	 x: 0    1    2    3     4    5    6
//	y=0 wall wall wall wall  wall wall wall
//	y=1 wall flr  flr  door  flr  flr  wall
//	y=2 wall wall wall wall  wall wall wall
func twoRooms() *engine.State {
	st := engine.NewState()
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[key(x, 0)] = &vttv1.TileRef{Kind: "wall"}
		tiles[key(x, 2)] = &vttv1.TileRef{Kind: "wall"}
	}
	tiles[key(0, 1)] = &vttv1.TileRef{Kind: "wall"}
	tiles[key(6, 1)] = &vttv1.TileRef{Kind: "wall"}
	for _, x := range []int32{1, 2, 4, 5} {
		tiles[key(x, 1)] = &vttv1.TileRef{Kind: "floor"}
	}
	tiles[key(3, 1)] = &vttv1.TileRef{Kind: "door"}

	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: tiles})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "goblin", Name: "Goblin"}}) // no controller: an NPC
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	return st
}

func player() gateway.Viewer {
	return gateway.Viewer{ParticipantID: "p-1", Role: identity.RolePlayer}
}

func TestAPlayerNeverReceivesATokenBehindAClosedDoor(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())

	// The goblin's own placement, replayed to this player.
	out := pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)

	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			t.Fatal("the goblin is behind a closed door and must not be on this player's wire")
		}
	}
}

func TestTheDMReceivesEverythingUnchanged(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM})

	in := envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	out := pr.Project(in, st)

	if len(out) != 1 || out[0] != in {
		t.Fatalf("the DM's projection must be the identity function, got %d envelopes", len(out))
	}
}

func TestOpeningTheDoorIntroducesTheGoblinToThePlayer(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)

	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.Project(envelope(7, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var sawActor, sawToken bool
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "goblin" {
			sawActor = true
		}
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			sawToken = true
			if e.GetSequence() != 7 {
				t.Errorf("a synthesized introduction carries the CAUSING sequence, got %d",
					e.GetSequence())
			}
		}
	}
	if !sawActor || !sawToken {
		t.Fatalf("opening the door must introduce the goblin (actor=%v token=%v)",
			sawActor, sawToken)
	}
}

func TestClosingTheDoorHidesTheGoblinAgain(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	pr.Project(envelope(7, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	mustApply(st, 8, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.Project(envelope(8, &vttv1.DoorClosed{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var hidden bool
	for _, e := range out {
		if h := e.GetTokenHidden(); h != nil && h.GetTokenId() == "t-gob" {
			hidden = true
		}
	}
	if !hidden {
		t.Fatal("closing the door must hide the goblin from this player")
	}
}

func TestAPartyMemberStaysKnownEvenWhenOutOfSight(t *testing.T) {
	// Spec §5: player-controlled actors are ALWAYS known. You know your party
	// exists when the rogue is two rooms away; you merely cannot see their
	// token. Dropping them from your own roster because they turned a corner
	// reads as a bug, not as fog.
	st := twoRooms()
	mustApply(st, 7, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "rogue", Name: "Rogue", ControllerIds: []string{"p-2"}}})
	mustApply(st, 8, &vttv1.TokenPlaced{TokenId: "t-rogue", SceneId: "s",
		ActorId: "rogue", Position: &vttv1.GridPosition{X: 5, Y: 1}}) // behind the closed door

	pr := gateway.NewProjector(player())
	out := pr.Project(envelope(7, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "rogue", Name: "Rogue", ControllerIds: []string{"p-2"}}}), st)

	var knowsRogue bool
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "rogue" {
			knowsRogue = true
		}
	}
	if !knowsRogue {
		t.Error("a player-controlled actor must reach every player's roster even out of sight")
	}

	// ...but their TOKEN must not, because creatures are pure line of sight.
	out = pr.Project(envelope(8, &vttv1.TokenPlaced{TokenId: "t-rogue",
		SceneId: "s", ActorId: "rogue",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-rogue" {
			t.Error("the rogue is behind a closed door — knowing they exist is not seeing them")
		}
	}
}

func TestAnUnrecognisedPayloadIsWithheldFromAPlayer(t *testing.T) {
	// FAIL CLOSED (spec §4.4). A payload the projection does not understand
	// must NOT be forwarded — that default is how this ships broken.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	out := pr.Project(&vttv1.Envelope{Sequence: 99}, st) // no payload at all
	if len(out) != 0 {
		t.Fatalf("an unknown payload must be withheld from a player, got %d", len(out))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/gateway/ -run "Projector|Projection|Player|DM" -v`
Expected: FAIL — `undefined: gateway.NewProjector`.

- [ ] **Step 3: Implement the projector**

Create `internal/gateway/project.go`. The essential shape:

```go
// Project returns the envelopes THIS viewer should receive for one log event.
// Zero, one, or several: an event can be withheld, passed through, or turned
// into an introduction plus the event itself.
//
// st is the state AFTER env was folded, so "what can this viewer see now"
// is answered against the world the event just created.
func (pr *Projector) Project(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	if pr.viewer.Role == identity.RoleDM || pr.viewer.Role == identity.RoleAgent {
		return []*vttv1.Envelope{env} // identity projection
	}

	out := pr.transitions(env, st)          // introductions + hides + SceneSeen
	if pr.passes(env, st) {
		out = append(out, env)
	}
	return out
}
```

with these rules, each of which a test above pins:

- `passes` returns true only for events whose subject the viewer can currently see. **Switch exhaustively over the payload oneof and return FALSE in `default`.**
- `transitions` diffs the viewer's visible token set against `pr.lastVisible` and emits `ActorAdded`+`TokenPlaced` for arrivals, `TokenHidden` for departures, each stamped with `env.GetSequence()`.
- `SceneSeen` is emitted whenever the viewer's visible square set changes, carrying the WHOLE current set.
- Sight comes from `sight.VisibleFrom` over the union of the viewer's actors (or the perched actor for a spectator).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/gateway/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/project.go internal/gateway/project_test.go
git commit -m "The projection: what one seat may know, computed per event"
```

---

## Task 5: Wire the projection into the pump

**Files:**
- Modify: `internal/gateway/server.go` (the pump, ~line 570-600)
- Test: `internal/gateway/server_visibility_test.go`

**Interfaces:**
- Consumes: `Projector` (Task 4).

- [ ] **Step 1: Write the failing end-to-end test**

```go
// TestSessionZeroCannotHappenAgain replays the defect this whole arc exists
// for: a player must not be able to SEE or TARGET a hidden creature's square.
// Session zero, seq 20: a player put their token on the Goblin Archer's exact
// square (19,8) on a 32x32 grid, having simply looked at the board.
func TestSessionZeroCannotHappenAgain(t *testing.T) {
	// Live server, a DM and a PLAYER seat (the DM sees everything, so a
	// DM-only test proves nothing — visibility spec §7).
	// 1. DM loads a scene with a wall between the player and the goblin.
	// 2. Assert NO envelope reaching the player mentions the goblin token.
	// 3. Assert a move onto the goblin's square is REFUSED for the player.
}
```

Fill this in against the existing live-server helpers in `internal/gateway/server_test.go` (see `TestThreeRoleExitScenarioOverLiveWebSockets` for the pattern, and `frameQueue` for reading frames per connection without positional drift).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/gateway/ -run TestSessionZeroCannotHappenAgain -v`
Expected: FAIL — the goblin's `TokenPlaced` arrives on the player's connection.

- [ ] **Step 3: Insert the projection into the pump**

In `internal/gateway/server.go`, in the broadcast pump (the block whose comment begins "Marshaled per connection, deliberately", ~line 583), replace the single-envelope encode with a projected loop:

```go
			// PROJECTED PER RECIPIENT (visibility spec §4). This is the seam
			// the whole arc turns on, and it already existed: this goroutine
			// holds p for the connection's life and already marshals per
			// connection, so filtering here costs no new machinery.
			for _, pe := range projector.Project(env, snapshot) {
				b, err := EncodeFrame(&vttv1.ServerFrame{
					Frame: &vttv1.ServerFrame_Event{Event: pe}})
				if err != nil {
					continue
				}
				select {
				case outCh <- b:
				case <-writerDone:
					return
				}
			}
```

Construct `projector := NewProjector(viewerFor(p))` once, before the loop.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/gateway/ -count=1 -p 1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway
git commit -m "Session zero cannot happen again: the pump projects per seat"
```

---

## Task 6: The spectator perch

**Files:**
- Create: `internal/gateway/viewpoint.go`
- Modify: `internal/gateway/server.go` (~878-892), `internal/gateway/authz.go`
- Test: `internal/gateway/viewpoint_test.go`

**Interfaces:**
- Consumes: `Viewer.Viewpoint` (Task 4), `vttv1.SetViewpoint` (Task 2).

- [ ] **Step 1: Write the failing test**

```go
func TestASpectatorMayPerchOnAPartyMemberButNotOnAnNPC(t *testing.T) {
	st := twoRooms() // "hero" is player-controlled; "goblin" is not
	spec := &identity.Participant{ID: "s-1", Role: identity.RoleSpectator}

	if err := gateway.MayPerch(spec, "hero", st); err != nil {
		t.Errorf("a spectator may ride a party member: %v", err)
	}
	// THE CONSTRAINT THE WHOLE IDEA RESTS ON. A spectator perched on the
	// Goblin Archer watches the ambush from inside it.
	if err := gateway.MayPerch(spec, "goblin", st); err == nil {
		t.Fatal("perching on an NPC must be REFUSED by the server, not merely absent from a menu")
	}
}

func TestPerchingAppendsNothingToTheLog(t *testing.T) {
	// Same shape as handleJoinDoor, whose comment says "It appends NOTHING".
	// Where a spectator looks is a view preference, not campaign history:
	// logging it would replay forever and make it retractable.
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/gateway/ -run Perch -v`
Expected: FAIL — `undefined: gateway.MayPerch`.

- [ ] **Step 3: Implement**

```go
// MayPerch reports whether p may ride actorID's shoulder.
//
// ONLY PLAYER-CONTROLLED ACTORS (visibility spec §3.1.1). An actor with an
// empty controller set is DM/agent-only — an NPC — and a spectator perched on
// the Goblin Archer would watch the ambush from inside it, undoing this arc
// in one click. Enforced HERE, never by which names the UI offers.
func MayPerch(p *identity.Participant, actorID string, st *engine.State) error {
	if actorID == "" {
		return nil // un-perch
	}
	a, ok := st.Actors[actorID]
	if !ok {
		return fmt.Errorf("%w: no actor %q", ErrUnauthorized, actorID)
	}
	if len(a.GetControllerIds()) == 0 {
		return fmt.Errorf("%w: %q is not a party member", ErrUnauthorized, actorID)
	}
	return nil
}
```

In `server.go`, beside the existing `LoadMap`/`SetJoinDoor` special cases (~878-892):

```go
	if v, ok := cmd.GetCommand().(*vttv1.ClientCommand_SetViewpoint); ok {
		return s.handleSetViewpoint(requestID, v.SetViewpoint, p, conn)
	}
```

`handleSetViewpoint` validates with `MayPerch`, updates the connection's `Viewer.Viewpoint`, re-emits a fresh `SceneSeen` for the new perch, and returns a `CommandResult` — **appending nothing**.

Add the `commandRoles` row in `authz.go`:

```go
	"set_viewpoint": {identity.RoleSpectator: true},
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/gateway/ -count=1 -p 1`
Expected: PASS. `authz_test.go`'s literal matrix count grows by one row — update the expected count there.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway
git commit -m "A spectator rides a shoulder, and the server says which shoulders exist"
```

---

## Task 7: The client draws only what it may

**Files:**
- Create: `client/src/view/visibility.ts`, `client/test/visibility.test.ts`
- Modify: `client/src/view/scene-plan.ts`

**Interfaces:**
- Consumes: `Scene.Explored` (Task 3).
- Produces: `planScene` takes visible tokens and explored terrain as INPUT.

- [ ] **Step 1: Write the failing test**

```ts
import { expect, test } from "bun:test";
import { planScene } from "../src/view/scene-plan";

test("a token outside line of sight produces NO DrawOp at all", () => {
  // NOT a DrawOp something later declines to paint — none. happy-dom has no
  // canvas, so the planner's output IS the observable.
  const ops = planScene(sceneWithGoblinHidden(), camera(), { visibleTokens: [] });
  expect(ops.filter((o) => o.kind === "token")).toHaveLength(0);
});

test("explored-but-unseen terrain is drawn dimmed, and unexplored is not drawn", () => {
  const ops = planScene(sceneHalfExplored(), camera(), { visibleTokens: [] });
  const tiles = ops.filter((o) => o.kind === "tile");
  expect(tiles.some((o) => o.dim === true)).toBe(true);
  expect(tiles.some((o) => o.key === "9,9")).toBe(false); // never seen
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `bun test client/test/visibility.test.ts`
Expected: FAIL — `planScene` does not accept a third argument.

- [ ] **Step 3: Implement**

`planScene` gains an options argument carrying `visibleTokens`. Tiles are emitted for `Explored` squares only, tagged `dim: !currentlyVisible`. **The planner never receives all tokens and filters at paint time** — that is where a leak would hide (spec §6).

- [ ] **Step 4: Run to verify it passes**

Run: `bun test client/test && task client:typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add client/
git commit -m "The board draws what this seat knows, and nothing else"
```

---

## Task 8: The keystone — projection parity in both languages

**Files:**
- Create: `client/test/projection-parity.test.ts`, `internal/harness/projection_golden_test.go`
- Modify: `scenarios/goldens/README.md`

**Interfaces:**
- Consumes: everything above.

This is spec §4.3, and the most valuable test in the arc.

- [ ] **Step 1: Write the failing test**

```go
// TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees is the
// keystone (visibility spec §4.3):
//
//	fold(project(log, viewer)) == visibleState(fold(log), viewer)
//
// Disagreement in EITHER direction is a defect: a player seeing a goblin they
// should not, or missing one they should.
func TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees(t *testing.T) {
	for _, g := range goldenCorpus(t) {
		for _, v := range []gateway.Viewer{dmViewer(), playerViewer(), spectatorViewer()} {
			projected := projectAll(g.Log, v)
			got := foldAll(projected)
			want := visibleState(foldAll(g.Log), v)
			if diff := stateDiff(got, want); diff != "" {
				t.Errorf("%s / %s: %s", g.Name, v.ParticipantID, diff)
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/harness/ -run Keystone -v`
Expected: FAIL until the helpers exist.

- [ ] **Step 3: Implement, and add the TS mirror**

The TS side folds the same projected corpus and asserts the same equality, extending `client/test/fold-parity.test.ts`'s existing shape. Today's goldens are the `viewer = DM` case where projection is identity — assert that explicitly, so the corpus proves the DM path unchanged.

- [ ] **Step 4: Run both**

Run: `go test ./internal/harness/ -count=1 && bun test client/test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/harness client/test scenarios/goldens/README.md
git commit -m "The keystone: a projection folds to exactly what its viewer may see"
```

---

## Task 9: Gates, docs and the demo

**Files:**
- Modify: `tools/mutation-scope.md`, `docs/map-format.md` (the `blocks_sight` note), `README.md`

- [ ] **Step 1: Correct the now-false claim in docs/map-format.md**

§5 currently says `blocks_sight` "is not [enforced] — nothing reads it yet" and "right now every participant sees the whole board". Both become false with this arc. Rewrite to state what `blocks_sight` now does.

- [ ] **Step 2: Run the full gate**

```bash
task check
```
Expected: green. If the mutation phase is SIGTERMed by the environment, run `check:mutation` package by package and say so — a gate that did not finish is not a green gate.

- [ ] **Step 3: Demo gate**

Screenshots, remote-friendly: a player's board with a closed door and no goblin; the same board after the door opens; the same player having walked away, terrain dimmed and the goblin gone; a spectator perched on one character then another.

- [ ] **Step 4: Commit**

```bash
git add docs README.md tools/mutation-scope.md
git commit -m "The docs stop saying every participant sees the whole board"
```

---

## Self-Review

**Spec coverage.** §3.1 whose eyes → Tasks 4, 6. §3.1.1 perch → Task 6. §3.2 memory → Tasks 3, 7. §3.3 blockers → Task 1. §3.4 range as input → Task 1 (`rangeSquares`). §3.5 trees/fractional seam → Task 1 (`Rect` float64). §4 projection → Tasks 4, 5. §4.3 keystone → Task 8. §5 contract → Task 2. §6 client → Tasks 3, 7. §7 testing → every task. §8 fail-closed → Task 4 Step 1's `default` test.

**Gap found and closed inline:** spec §5's rule that player-controlled actors are ALWAYS known had no test. `TestAPartyMemberStaysKnownEvenWhenOutOfSight` is now in Task 4 Step 1, and it pins BOTH halves — the rogue reaches the roster, their token does not.

**Placeholder scan.** Task 5 Step 1 and Task 6 Step 1's second test are described rather than written, because both need live-server helpers whose exact shape lives in `server_test.go`; each names the existing test to copy. Task 9 Step 3 is a demo, not code. No other step defers work.

**Type consistency.** `Viewer`/`Projector`/`Project` consistent across Tasks 4-6, 8. `Rect`, `Blockers`, `Clear`, `VisibleFrom` consistent between Task 1 and Task 4. `Explored` is `map[string]bool` in Go and `Record<string, boolean>` in TS — Task 3 defines both, Task 7 consumes the TS one.
