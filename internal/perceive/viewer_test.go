package perceive_test

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// testViewer drives one projection as a seat does: each Event is handed the
// fold of every frame returned before it, and what it returns is folded.
type testViewer struct {
	tb    testing.TB
	eyes  perceive.Eyes
	pr    *perceive.Projection
	shown *engine.State
}

func newViewer(tb testing.TB, eyes perceive.Eyes) *testViewer {
	tb.Helper()
	return &testViewer{tb: tb, eyes: eyes, pr: perceive.New(eyes, nil), shown: engine.NewState()}
}

// event projects env against st, the world after it, and folds the frames.
func (v *testViewer) event(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	v.tb.Helper()
	out := v.pr.Event(v.eyes, env, v.shown, st)
	// Fold with engine.Apply alone: this package may not import campaign
	// (TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight).
	for i, e := range out {
		// Keep the Fatal: it holds VTT-224 for every test here.
		if err := engine.Apply(v.shown, e); err != nil {
			v.tb.Fatalf("frame %d (%T) of seq %d does not fold onto what the viewer was sent: %v",
				i, e.GetPayload(), env.GetSequence(), err)
		}
	}
	return out
}

func player() perceive.Eyes { return perceive.Eyes{Controller: "p-1"} }

// key formats a square as the wire does, column then row.
func key(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// envelope wraps payload at seq. Keep the panic on a payload it does not know:
// an Envelope with none passes classify's unrecognised arm and proves nothing.
func envelope(seq int64, payload proto.Message) *vttv1.Envelope {
	env := &vttv1.Envelope{Sequence: seq}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		env.SessionId = "sess-1"
		env.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SessionEnded:
		env.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: p}
	case *vttv1.SceneCreated:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		env.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.TokenRemoved:
		env.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: p}
	case *vttv1.ActorRemoved:
		env.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: p}
	case *vttv1.DoorOpened:
		env.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: p}
	case *vttv1.DoorClosed:
		env.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: p}
	case *vttv1.NarrationAdded:
		env.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: p}
	case *vttv1.NoteUpserted:
		env.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: p}
	case *vttv1.NoteDeleted:
		env.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: p}
	case *vttv1.ActorControlGranted:
		env.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: p}
	case *vttv1.ActorControlRevoked:
		env.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: p}
	case *vttv1.SceneSeen:
		env.Payload = &vttv1.Envelope_SceneSeen{SceneSeen: p}
	case *vttv1.ConditionApplied:
		env.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: p}
	case *vttv1.ConditionRemoved:
		env.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: p}
	case *vttv1.ResourceChanged:
		env.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: p}
	case *vttv1.AttackRolled:
		env.Payload = &vttv1.Envelope_AttackRolled{AttackRolled: p}
	case *vttv1.AbilityUsed:
		env.Payload = &vttv1.Envelope_AbilityUsed{AbilityUsed: p}
	default:
		panic(fmt.Sprintf("envelope: no oneof arm wired for %T", payload))
	}
	return env
}

// Panic in mustApply rather than take a testing.TB: the fixtures below build
// their worlds outside any test, and one the fold refuses is a broken fixture.
func mustApply(st *engine.State, seq int64, payload proto.Message) {
	if err := engine.Apply(st, envelope(seq, payload)); err != nil {
		panic(fmt.Sprintf("mustApply seq %d: %v", seq, err))
	}
}

// twoRooms is 7x3 with a dividing wall at x=3 and a shut door at 3,1.
//
//	 x: 0    1    2    3     4    5    6
//	y=0 wall wall wall wall  wall wall wall
//	y=1 wall flr  flr  door  flr  flr  wall
//	y=2 wall wall wall wall  wall wall wall
func twoRoomsTiles() map[string]*vttv1.TileRef {
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
	return tiles
}

func twoRooms() *engine.State {
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()})
	// Keep both kinds declared and the grant last: an actor with no kind is no
	// party member, and control is conferred only by a grant (SPEC-013).
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "goblin", Name: "Goblin",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// firstPlace projects the goblin's placement: a fresh viewer's first event,
// which brings it its scene, its party and what it sees.
func firstPlace(pr *testViewer, st *engine.State) []*vttv1.Envelope {
	return pr.event(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
}

// sceneSeenIn is the last SceneSeen of sceneID in out, or nil.
func sceneSeenIn(out []*vttv1.Envelope, sceneID string) *vttv1.SceneSeen {
	var found *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil && ss.GetSceneId() == sceneID {
			found = ss
		}
	}
	return found
}

// step is one event of a hand-built log.
type step struct {
	seq     int64
	payload proto.Message
}

// runLogSteps folds each step into the world and projects it, one batch per
// step. isolate hands the projection a snapshot nothing mutates again, which
// makes an alias into live state observable.
func runLogSteps(tb testing.TB, eyes perceive.Eyes, steps []step, isolate bool) [][]*vttv1.Envelope {
	tb.Helper()
	st := engine.NewState()
	pr := newViewer(tb, eyes)
	out := make([][]*vttv1.Envelope, len(steps))
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runLog seq %d: %v", s.seq, err))
		}
		view := st
		if isolate {
			view = st.Snapshot()
		}
		out[i] = pr.event(env, view)
	}
	return out
}

func runLog(tb testing.TB, eyes perceive.Eyes, steps []step) []*vttv1.Envelope {
	tb.Helper()
	var out []*vttv1.Envelope
	for _, batch := range runLogSteps(tb, eyes, steps, false) {
		out = append(out, batch...)
	}
	return out
}

// forwardedOf is env as a player or a spectator is sent it (SPEC-016).
func forwardedOf(env *vttv1.Envelope) *vttv1.Envelope {
	c := proto.Clone(env).(*vttv1.Envelope)
	c.ParticipantId, c.ActorRole = "", ""
	if m := c.GetTokenMoved(); m != nil {
		m.Reason = ""
	}
	if r := c.GetResourceChanged(); r != nil {
		r.Reason = ""
	}
	if a := c.GetConditionApplied(); a != nil {
		a.Source = ""
	}
	if r := c.GetConditionRemoved(); r != nil {
		r.Reason = ""
	}
	return c
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
