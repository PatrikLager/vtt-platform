package engine_test

import (
	"reflect"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func qdEnv(id string, p any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: id}
	switch v := p.(type) {
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: v}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: v}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: v}
	case *vttv1.DoorOpened:
		e.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: v}
	case *vttv1.DoorClosed:
		e.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: v}
	default:
		panic("unknown payload")
	}
	return e
}

func qdDoorWorld(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	tiles := map[string]*vttv1.TileRef{
		"2,2": {Kind: "floor"}, "3,2": {Kind: "door"}, "4,2": {Kind: "floor"},
	}
	for _, e := range []*vttv1.Envelope{
		qdEnv("s1", &vttv1.SceneCreated{SceneId: "hall", Name: "Hall", GridWidth: 7, GridHeight: 5, Tiles: tiles}),
		qdEnv("a1", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}),
		qdEnv("g1", &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p1", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}),
		qdEnv("a2", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "ogre", Name: "Ogre", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}),
	} {
		if err := engine.Apply(st, e); err != nil {
			t.Fatalf("setup %s: %v", e.EventId, err)
		}
	}
	return st
}

func qdDoorEvent(open bool, scene string, at *vttv1.GridPosition, actor string) *vttv1.Envelope {
	if open {
		return qdEnv("d", &vttv1.DoorOpened{SceneId: scene, At: at, ActorId: actor})
	}
	return qdEnv("d", &vttv1.DoorClosed{SceneId: scene, At: at, ActorId: actor})
}

func qdDoors(st *engine.State) map[string]bool {
	out := map[string]bool{}
	for k, v := range st.Scenes["hall"].OpenDoors {
		out[k] = v
	}
	return out
}

// VTT-866
func TestQADoorOpenerAbsentActorRefusedAndStateUnchanged(t *testing.T) {
	for _, open := range []bool{true, false} {
		for _, actor := range []string{"ghost", "hasOwnProperty", "constructor", "__proto__", "toString"} {
			st := qdDoorWorld(t)
			if !open { // close needs something to close
				if err := engine.Apply(st, qdDoorEvent(true, "hall", &vttv1.GridPosition{X: 3, Y: 2}, "")); err != nil {
					t.Fatal(err)
				}
			}
			before := qdDoors(st)
			nActors := len(st.Actors)
			err := engine.Apply(st, qdDoorEvent(open, "hall", &vttv1.GridPosition{X: 3, Y: 2}, actor))
			if err == nil {
				t.Fatalf("open=%v actor %q: want refusal", open, actor)
			}
			if !reflect.DeepEqual(before, qdDoors(st)) || len(st.Actors) != nActors {
				t.Fatalf("open=%v actor %q: state changed by a refusal", open, actor)
			}
		}
	}
}

// VTT-866
func TestQADoorOpenerScenePositionRefusedForThatReasonWhateverTheActor(t *testing.T) {
	cases := map[string]struct {
		scene string
		at    *vttv1.GridPosition
	}{
		"unknown scene":    {"nowhere", &vttv1.GridPosition{X: 3, Y: 2}},
		"missing position": {"hall", nil},
	}
	for name, c := range cases {
		for _, open := range []bool{true, false} {
			base := qdDoorWorld(t)
			emptyErr := engine.Apply(base, qdDoorEvent(open, c.scene, c.at, ""))
			if emptyErr == nil {
				t.Fatalf("%s open=%v: empty actor must be refused for the scene or position", name, open)
			}
			for _, actor := range []string{"ghost", "hasOwnProperty", "hero"} {
				st := qdDoorWorld(t)
				err := engine.Apply(st, qdDoorEvent(open, c.scene, c.at, actor))
				if err == nil || err.Error() != emptyErr.Error() {
					t.Fatalf("%s open=%v actor %q: err %v, want the empty-actor error %q", name, open, actor, err, emptyErr)
				}
			}
		}
	}
	st := qdDoorWorld(t)
	sceneErr := engine.Apply(st, qdDoorEvent(true, "nowhere", &vttv1.GridPosition{X: 3, Y: 2}, ""))
	actorErr := engine.Apply(st, qdDoorEvent(true, "hall", &vttv1.GridPosition{X: 3, Y: 2}, "ghost"))
	if sceneErr == nil || actorErr == nil || sceneErr.Error() == actorErr.Error() {
		t.Fatalf("scene error %v and actor error %v must be distinct refusals", sceneErr, actorErr)
	}
}

// VTT-866
func TestQADoorOpenerHeldAndEmptyActorFold(t *testing.T) {
	for _, actor := range []string{"", "hero", "ogre"} {
		st := qdDoorWorld(t)
		if err := engine.Apply(st, qdDoorEvent(true, "hall", &vttv1.GridPosition{X: 3, Y: 2}, actor)); err != nil {
			t.Fatalf("open actor %q: %v", actor, err)
		}
		if !qdDoors(st)["3,2"] {
			t.Fatalf("actor %q: door not open after DoorOpened", actor)
		}
		if err := engine.Apply(st, qdDoorEvent(false, "hall", &vttv1.GridPosition{X: 3, Y: 2}, actor)); err != nil {
			t.Fatalf("close actor %q: %v", actor, err)
		}
		if qdDoors(st)["3,2"] {
			t.Fatalf("actor %q: door still open after DoorClosed", actor)
		}
	}
}
