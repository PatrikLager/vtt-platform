package engine_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func opened(scene string, at *vttv1.GridPosition, actor string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: scene, At: at, ActorId: actor}}}
}

func closed(scene string, at *vttv1.GridPosition, actor string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: scene, At: at, ActorId: actor}}}
}

// cellarWithOpenDoor holds actor a1 and the cellar's door at 0,1 open.
func cellarWithOpenDoor(t *testing.T) *engine.State {
	t.Helper()
	st := stateWithCellar(t)
	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1"}})))
	must(t, engine.Apply(st, opened("cellar", pos(0, 1), "")))
	return st
}

// VTT-866
func TestADoorEventNamingAnActorTheStateLacksIsRefused(t *testing.T) {
	for name, c := range map[string]struct {
		ev   *vttv1.Envelope
		want string
	}{
		"an opening":            {opened("cellar", pos(0, 1), "ghost"), `engine: door opened names unknown actor "ghost"`},
		"a closing":             {closed("cellar", pos(0, 1), "ghost"), `engine: door closed names unknown actor "ghost"`},
		"an id like a property": {closed("cellar", pos(0, 1), "hasOwnProperty"), `engine: door closed names unknown actor "hasOwnProperty"`},
	} {
		t.Run(name, func(t *testing.T) {
			refusesUnchanged(t, cellarWithOpenDoor(t), c.ev, c.want)
		})
	}
}

// VTT-866
func TestADoorEventIsRefusedForItsSceneOrPositionBeforeItsActor(t *testing.T) {
	for name, c := range map[string]struct {
		ev   *vttv1.Envelope
		want string
	}{
		"opened, unknown scene": {opened("nowhere", pos(0, 1), "ghost"), `engine: door opened in unknown scene "nowhere"`},
		"opened, no position":   {opened("cellar", nil, "ghost"), "engine: door opened without position"},
		"closed, unknown scene": {closed("nowhere", pos(0, 1), "ghost"), `engine: door closed in unknown scene "nowhere"`},
		"closed, no position":   {closed("cellar", nil, "ghost"), "engine: door closed without position"},
	} {
		t.Run(name, func(t *testing.T) {
			refusesUnchanged(t, cellarWithOpenDoor(t), c.ev, c.want)
		})
	}
}

// VTT-866
func TestADoorEventNamingAnActorTheStateHoldsOrNoneFolds(t *testing.T) {
	for _, actor := range []string{"a1", ""} {
		st := cellarWithOpenDoor(t)
		must(t, engine.Apply(st, closed("cellar", pos(0, 1), actor)))
		if st.Scenes["cellar"].OpenDoors["0,1"] {
			t.Fatalf("a closing naming %q left the door open", actor)
		}
		must(t, engine.Apply(st, opened("cellar", pos(0, 1), actor)))
		if !st.Scenes["cellar"].OpenDoors["0,1"] {
			t.Fatalf("an opening naming %q left the door shut", actor)
		}
	}
}
