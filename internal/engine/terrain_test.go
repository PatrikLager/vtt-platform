package engine_test

import (
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// pos is a small literal-builder so table tests below read as coordinates,
// not proto boilerplate.
func pos(x, y int32) *vttv1.GridPosition {
	return &vttv1.GridPosition{X: x, Y: y}
}

// stateWithCellar builds a 3x3 scene named "cellar": walls on the north and
// south rows, a door at (0,1), open floor at (1,1) and (2,1), and a boulder
// object at (1,1) that blocks movement independently of the floor tile under
// it. Every test in this file exercises Blocked against this one fixture
// (task-5-brief.md step 1), so a layout bug here would silently invalidate
// all of them — hence the ASCII map in the comment.
//
//	 x: 0     1     2
//	y=0 wall  wall  wall
//	y=1 door  floor floor  (+ boulder object at 1,1, blocks_move)
//	y=2 wall  wall  wall
func stateWithCellar(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{
		SceneId: "cellar", Name: "Cellar", GridWidth: 3, GridHeight: 3,
		Tiles: map[string]*vttv1.TileRef{
			"0,0": {Kind: "wall"}, "1,0": {Kind: "wall"}, "2,0": {Kind: "wall"},
			"0,1": {Kind: "door"}, "1,1": {Kind: "floor"}, "2,1": {Kind: "floor"},
			"0,2": {Kind: "wall"}, "1,2": {Kind: "wall"}, "2,2": {Kind: "wall"},
		},
		Objects: []*vttv1.SceneObject{
			{
				ObjectId: "boulder-1", Kind: "boulder", At: pos(1, 1),
				Width: 1, Height: 1, BlocksMove: true,
			},
		},
	})))
	return st
}

func TestBlockedAnswersForWallsClosedDoorsAndScenery(t *testing.T) {
	st := stateWithCellar(t)
	for _, c := range []struct {
		name       string
		x, y       int32
		wantBlock  bool
		wantReason string
	}{
		{"a wall", 0, 0, true, "wall"},
		{"open floor", 2, 1, false, ""},
		{"a closed door", 0, 1, true, "door"},
		{"outside the grid", 9, 9, true, "outside"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, why := st.Blocked("cellar", c.x, c.y)
			if got != c.wantBlock {
				t.Fatalf("Blocked(%d,%d) = %v, want %v", c.x, c.y, got, c.wantBlock)
			}
			if c.wantBlock && !strings.Contains(why, c.wantReason) {
				t.Fatalf("reason %q does not mention %q", why, c.wantReason)
			}
		})
	}
}

func TestOpeningADoorUnblocksItAndSurvivesReplay(t *testing.T) {
	st := stateWithCellar(t)
	if blocked, _ := st.Blocked("cellar", 0, 1); !blocked {
		t.Fatal("the door started open; this test proves nothing")
	}
	must(t, engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: "cellar", At: pos(0, 1)},
	}}))
	if blocked, why := st.Blocked("cellar", 0, 1); blocked {
		t.Fatalf("an opened door still blocks: %s", why)
	}
	must(t, engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: "cellar", At: pos(0, 1)},
	}}))
	if blocked, _ := st.Blocked("cellar", 0, 1); !blocked {
		t.Fatal("a closed door does not block — door state is not folding")
	}
}

func TestSceneryWithBlocksMoveIsImpassable(t *testing.T) {
	st := stateWithCellar(t) // carries a boulder at 1,1 with BlocksMove
	if blocked, why := st.Blocked("cellar", 1, 1); !blocked {
		t.Fatalf("a boulder did not block movement: %q", why)
	}
}

// TestBlockedOnUnknownSceneBlocks locks in the fail-closed direction that is
// load-bearing for this task: refusing a move into a scene this state cannot
// describe is recoverable, permitting one is not. None of the table cases
// above exercise this — they all use "cellar", a scene that exists — so it
// needs its own test.
func TestBlockedOnUnknownSceneBlocks(t *testing.T) {
	st := engine.NewState()
	blocked, why := st.Blocked("nowhere", 0, 0)
	if !blocked {
		t.Fatal("Blocked on an unknown scene returned false — an unrecognised scene must fail closed")
	}
	if !strings.Contains(why, "nowhere") {
		t.Fatalf("reason %q does not name the unknown scene", why)
	}
}

// TestBlockedOnSceneWithoutTerrainNeverBlocks locks in Patrik's 2026-08-13
// ruling: an empty Tiles map is legal. A scene created before this branch
// existed — or one deliberately made without terrain — has to keep behaving
// exactly as every scene did before maps-as-geometry: nothing may ever be
// blocked by terrain that was never recorded. seedScene (apply_test.go)
// builds exactly that: a SceneCreated with no Tiles/Objects at all.
func TestBlockedOnSceneWithoutTerrainNeverBlocks(t *testing.T) {
	st := seedScene(t)
	if blocked, why := st.Blocked("scn", 5, 5); blocked {
		t.Fatalf("a scene with no terrain recorded blocked (5,5): %q", why)
	}
}

// TestDoorCommandsRejectUnknownSceneAndMissingPosition mirrors the
// validation every other position-bearing arm in Apply already does
// (TokenPlaced rejects a nil Position the same way) and confirms door
// events error rather than silently toggling a fabricated "0,0" door when
// the position is missing, or mutating a scene that was never created.
// Apply's own contract is "validates BEFORE mutating: any error return
// leaves st unchanged" (apply.go doc comment) — reusing one state across
// all four cases is itself part of the proof that none of them touch it.
func TestDoorCommandsRejectUnknownSceneAndMissingPosition(t *testing.T) {
	st := stateWithCellar(t)
	before := st.Snapshot()
	for _, c := range []struct {
		name string
		env  *vttv1.Envelope
	}{
		{"open: unknown scene", &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
			DoorOpened: &vttv1.DoorOpened{SceneId: "nowhere", At: pos(0, 1)},
		}}},
		{"open: no position", &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
			DoorOpened: &vttv1.DoorOpened{SceneId: "cellar"},
		}}},
		{"close: unknown scene", &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
			DoorClosed: &vttv1.DoorClosed{SceneId: "nowhere", At: pos(0, 1)},
		}}},
		{"close: no position", &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
			DoorClosed: &vttv1.DoorClosed{SceneId: "cellar"},
		}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := engine.Apply(st, c.env); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
	if !reflect.DeepEqual(before, st.Snapshot()) {
		t.Fatal("a rejected door command still mutated state")
	}
}

func TestSnapshotOfDoorStateDoesNotAliasLiveState(t *testing.T) {
	st := stateWithCellar(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: "cellar", At: pos(0, 1)},
	}}))
	after := st.Snapshot()
	if blocked, why := before.Blocked("cellar", 0, 1); !blocked {
		t.Fatalf("opening the door on live state changed a snapshot taken before it: blocked=%v why=%q", blocked, why)
	}
	if blocked, why := after.Blocked("cellar", 0, 1); blocked {
		t.Fatalf("a snapshot taken after opening the door still shows it closed: blocked=%v why=%q", blocked, why)
	}
	must(t, engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: "cellar", At: pos(0, 1)},
	}}))
	if blocked, why := after.Blocked("cellar", 0, 1); blocked {
		t.Fatalf("closing the door on live state changed an earlier snapshot: blocked=%v why=%q", blocked, why)
	}
}

// bareScene builds a Scene the way SceneCreated's fold arm never would: a
// literal assigned straight into st.Scenes, the way
// internal/rules/conformance/conformance.go does (semgrep-exempted there as
// synthetic state that never becomes campaign state — see
// .semgrep/event-sourcing.yml). OpenDoors is left nil on purpose: that
// package issues no door events today, but nothing stops a future one from
// reaching this arm with exactly this shape. A single door tile at (0,0)
// gives Blocked something to check OpenDoors against — a Scene with no
// Tiles at all would make the door arms' write silently unobservable, since
// Blocked's switch never reaches the "door" case without one, and a test
// against it could pass for the wrong reason (see the comment on
// TestDoorOpenedInitializesNilOpenDoors).
func bareScene(t *testing.T, st *engine.State, id string) {
	t.Helper()
	st.Scenes[id] = engine.Scene{
		ID: id, Name: id, GridWidth: 3, GridHeight: 3,
		Tiles: map[string]engine.Tile{"0,0": {Kind: "door"}},
	}
}

// TestDoorOpenedInitializesNilOpenDoors closes a fix-round-1 finding:
// DoorOpened wrote sc.OpenDoors[key] = true, and writing to a nil map
// panics in Go. Apply's own doc comment promises it "validates BEFORE
// mutating" — a bad or unusual event should produce an error, never crash
// the process folding every event in the log. Before the fix this test
// panics rather than failing an assertion, which is deliberately a
// different failure mode from every other test in this file: it proves the
// crash is real, not merely asserted.
func TestDoorOpenedInitializesNilOpenDoors(t *testing.T) {
	st := engine.NewState()
	bareScene(t, st, "bare")

	must(t, engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: "bare", At: pos(0, 0)},
	}}))

	// This has to read back through st.Blocked, not just check that Apply
	// returned nil: when OpenDoors starts nil, the fix's guard hands sc a
	// BRAND NEW map that exists only in Apply's local copy until written
	// back to st.Scenes. A version of the fix that forgot the write-back
	// would return nil here (no panic, no error) while leaving the live
	// scene's door reading closed — this assertion is what would catch that.
	if blocked, why := st.Blocked("bare", 0, 0); blocked {
		t.Fatalf("door did not open on a Scene with a nil OpenDoors map: blocked=%v why=%q", blocked, why)
	}
}

// TestDoorClosedInitializesNilOpenDoors is DoorClosed's counterpart. Go's
// delete() is a documented no-op on a nil map, so this arm was never at
// crash risk the way DoorOpened's assignment was — but the guard is added
// there too, for the same reason both arms document the same invariant: a
// reader of one arm should not need to recall a `delete`-on-nil subtlety to
// trust that OpenDoors is always safe to use after Apply returns nil, and a
// later edit to this arm could silently reintroduce the panic without it.
func TestDoorClosedInitializesNilOpenDoors(t *testing.T) {
	st := engine.NewState()
	bareScene(t, st, "bare")

	must(t, engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: "bare", At: pos(0, 0)},
	}}))

	if blocked, why := st.Blocked("bare", 0, 0); !blocked {
		t.Fatalf("a door that was never opened must still read closed: blocked=%v why=%q", blocked, why)
	}
}
