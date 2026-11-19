package perceive_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

var theDoor = &vttv1.GridPosition{X: 3, Y: 1}

// doorEvent is a door event of the log, carrying its event id, applied to st.
func doorEvent(st *engine.State, seq int64, open bool, actor string) *vttv1.Envelope {
	var p proto.Message = &vttv1.DoorClosed{SceneId: "s", At: theDoor, ActorId: actor}
	if open {
		p = &vttv1.DoorOpened{SceneId: "s", At: theDoor, ActorId: actor}
	}
	mustApply(st, seq, p)
	env := envelope(seq, p)
	env.EventId = "evt-door"
	return env
}

// doorFrames are the DoorOpened and DoorClosed frames of out.
func doorFrames(out []*vttv1.Envelope) []*vttv1.Envelope {
	var doors []*vttv1.Envelope
	for _, e := range out {
		if e.GetDoorOpened() != nil || e.GetDoorClosed() != nil {
			doors = append(doors, e)
		}
	}
	return doors
}

// withLurker is twoRooms with an actor that has no token, so no eyes ever see it.
func withLurker() *engine.State {
	st := twoRooms()
	mustApply(st, 7, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "lurker", Name: "Lurker",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})
	return st
}

func eyesOnTheHero() map[string]perceive.Eyes {
	return map[string]perceive.Eyes{"a player": player(), "a spectator on the hero's shoulder": {Shoulder: "hero"}}
}

// VTT-869 VTT-870
func TestADoorOpenedByAnActorTheEyesNeverSawArrivesAsItsStateAlone(t *testing.T) {
	for name, eyes := range eyesOnTheHero() {
		t.Run(name, func(t *testing.T) {
			st := twoRooms()
			v := newViewer(t, eyes)
			firstPlace(v, st)

			doors := doorFrames(v.event(doorEvent(st, 8, true, "goblin"), st))
			if len(doors) != 1 || doors[0].GetDoorOpened() == nil {
				t.Fatalf("door frames = %v, want one DoorOpened", doors)
			}
			if doors[0].GetEventId() != "" || doors[0].GetDoorOpened().GetActorId() != "" {
				t.Fatalf("door frame = %v, want the door's state with no event id and no actor", doors[0])
			}
			if !v.shown.Scenes["s"].OpenDoors[key(3, 1)] {
				t.Fatal("the eyes' fold keeps the door shut after it opened in their sight")
			}
			if _, ok := v.shown.Actors["goblin"]; !ok {
				t.Fatal("the goblin the opening brought into sight was not introduced")
			}
		})
	}
}

// VTT-869 VTT-870
func TestADoorClosedByAnActorTheEyesNeverSawArrivesAsItsStateAlone(t *testing.T) {
	for name, eyes := range eyesOnTheHero() {
		t.Run(name, func(t *testing.T) {
			st := withLurker()
			v := newViewer(t, eyes)
			firstPlace(v, st)
			v.event(doorEvent(st, 8, true, ""), st)

			doors := doorFrames(v.event(doorEvent(st, 9, false, "lurker"), st))
			if len(doors) != 1 || doors[0].GetDoorClosed() == nil {
				t.Fatalf("door frames = %v, want one DoorClosed", doors)
			}
			if doors[0].GetEventId() != "" || doors[0].GetDoorClosed().GetActorId() != "" {
				t.Fatalf("door frame = %v, want the door's state with no event id and no actor", doors[0])
			}
			if v.shown.Scenes["s"].OpenDoors[key(3, 1)] {
				t.Fatal("the eyes' fold keeps the door open after it closed in their sight")
			}
		})
	}
}

// VTT-869
func TestADoorEventNamingAnActorTheEyesSawOrNoneIsForwardedOnceWithItsEventID(t *testing.T) {
	for name, c := range map[string]struct {
		open  bool
		actor string
	}{
		"opened by the eyes' own character": {true, "hero"},
		"opened by nobody":                  {true, ""},
		"closed by the goblin, seen since":  {false, "goblin"},
	} {
		t.Run(name, func(t *testing.T) {
			st := twoRooms()
			v := newViewer(t, player())
			firstPlace(v, st)
			if !c.open {
				v.event(doorEvent(st, 8, true, ""), st)
			}

			doors := doorFrames(v.event(doorEvent(st, 9, c.open, c.actor), st))
			if len(doors) != 1 || doors[0].GetEventId() != "evt-door" {
				t.Fatalf("door frames = %v, want the event itself, once", doors)
			}
			got := doors[0].GetDoorOpened().GetActorId() + doors[0].GetDoorClosed().GetActorId()
			if got != c.actor {
				t.Fatalf("forwarded door names %q, want %q", got, c.actor)
			}
		})
	}
}

// VTT-212
func TestADoorEventAtASquareTheEyesDoNotSeeSendsNoDoor(t *testing.T) {
	st := twoRooms()
	mustApply(st, 7, &vttv1.SceneCreated{SceneId: "s2", Name: "Elsewhere", GridWidth: 3, GridHeight: 3,
		Tiles: map[string]*vttv1.TileRef{key(1, 1): {Kind: "door"}}})
	v := newViewer(t, player())
	firstPlace(v, st)

	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s2", At: &vttv1.GridPosition{X: 1, Y: 1}, ActorId: "hero"})
	in := envelope(8, &vttv1.DoorOpened{SceneId: "s2", At: &vttv1.GridPosition{X: 1, Y: 1}, ActorId: "hero"})
	in.EventId = "evt-door"
	if doors := doorFrames(v.event(in, st)); len(doors) != 0 {
		t.Fatalf("door frames = %v, want none for a square out of sight", doors)
	}
}
