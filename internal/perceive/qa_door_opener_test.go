package perceive_test

import (
	"fmt"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const qdScene = "keep"

func qdTiles() map[string]*vttv1.TileRef {
	t := map[string]*vttv1.TileRef{}
	for x := 0; x < 8; x++ {
		for y := 0; y < 4; y++ {
			t[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "wall"}
		}
	}
	for _, x := range []int{1, 2, 4, 6} {
		t[fmt.Sprintf("%d,1", x)] = &vttv1.TileRef{Kind: "floor"}
	}
	t["3,1"] = &vttv1.TileRef{Kind: "door"}
	t["5,1"] = &vttv1.TileRef{Kind: "door"}
	return t
}

func qdE(id string, p any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: id}
	switch v := p.(type) {
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: v}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: v}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: v}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: v}
	case *vttv1.DoorOpened:
		e.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: v}
	case *vttv1.DoorClosed:
		e.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: v}
	default:
		panic("payload")
	}
	return e
}

func qdActor(id string, k vttv1.ActorKind) *vttv1.Envelope {
	return qdE("add-"+id, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: k}})
}

func qdToken(id string, x, y int32) *vttv1.Envelope {
	return qdE("tok-"+id, &vttv1.TokenPlaced{TokenId: "t-" + id, SceneId: qdScene, ActorId: id, Position: &vttv1.GridPosition{X: x, Y: y}, Name: id})
}

func qdDoor(id string, open bool, x int32, actor string) *vttv1.Envelope {
	at := &vttv1.GridPosition{X: x, Y: 1}
	if open {
		return qdE(id, &vttv1.DoorOpened{SceneId: qdScene, At: at, ActorId: actor})
	}
	return qdE(id, &vttv1.DoorClosed{SceneId: qdScene, At: at, ActorId: actor})
}

func qdBase() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		qdE("scene", &vttv1.SceneCreated{SceneId: qdScene, Name: "Keep", GridWidth: 8, GridHeight: 4, Tiles: qdTiles()}),
		qdActor("hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qdE("grant", &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p1", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}),
		qdActor("scout", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qdActor("ogre", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qdActor("ghost", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qdToken("hero", 1, 1),
		qdToken("scout", 2, 1),
		qdToken("ogre", 4, 1),
	}
}

type qdViewer struct {
	t     *testing.T
	eyes  perceive.Eyes
	world *engine.State
	shown *engine.State
	pr    *perceive.Projection
	sent  []*vttv1.Envelope
}

func qdViewers() map[string]perceive.Eyes {
	return map[string]perceive.Eyes{
		"player":    {Controller: "p1"},
		"spectator": {Shoulder: "hero"},
	}
}

func qdNew(t *testing.T, eyes perceive.Eyes) *qdViewer {
	t.Helper()
	return &qdViewer{t: t, eyes: eyes, world: engine.NewState(), shown: engine.NewState(), pr: perceive.New(eyes, engine.NewState())}
}

func (v *qdViewer) feed(env *vttv1.Envelope) []*vttv1.Envelope {
	v.t.Helper()
	if err := engine.Apply(v.world, env); err != nil {
		v.t.Fatalf("world refused %s: %v", env.EventId, err)
	}
	frames := v.pr.Event(v.eyes, env, v.shown, v.world)
	for _, f := range frames {
		if err := engine.Apply(v.shown, f); err != nil {
			v.t.Fatalf("viewer's fold refused a frame after %s: %v (%v)", env.EventId, err, f)
		}
		v.sent = append(v.sent, f)
	}
	return frames
}

func (v *qdViewer) feedAll(envs ...*vttv1.Envelope) {
	v.t.Helper()
	for _, e := range envs {
		v.feed(e)
	}
}

func qdDoorFrames(frames []*vttv1.Envelope) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range frames {
		if f.GetDoorOpened() != nil || f.GetDoorClosed() != nil {
			out = append(out, f)
		}
	}
	return out
}

func qdActorOf(f *vttv1.Envelope) string {
	if d := f.GetDoorOpened(); d != nil {
		return d.GetActorId()
	}
	return f.GetDoorClosed().GetActorId()
}

func (v *qdViewer) doorOpen(x int) bool {
	return v.shown.Scenes[qdScene].OpenDoors[fmt.Sprintf("%d,1", x)]
}

func qdWithEventID(frames []*vttv1.Envelope, id string) int {
	n := 0
	for _, f := range frames {
		if f.GetEventId() == id {
			n++
		}
	}
	return n
}

func qdEach(t *testing.T, fn func(v *qdViewer)) {
	t.Helper()
	for name, eyes := range qdViewers() {
		t.Run(name, func(t *testing.T) { fn(qdNew(t, eyes)) })
	}
}

// VTT-869 VTT-870 VTT-213
func TestQADoorOpenerOpenedByCreatureBehindTheDoorIsNotForwardedButItsStateIsSentOnce(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		if v.shown.Actors["ogre"] != nil {
			t.Fatal("fixture: ogre must be unseen behind the closed door")
		}
		frames := v.feed(qdDoor("open-by-ogre", true, 3, "ogre"))
		if n := qdWithEventID(frames, "open-by-ogre"); n != 0 {
			t.Fatalf("the unseen opener's event was forwarded %d times", n)
		}
		df := qdDoorFrames(frames)
		if len(df) != 1 {
			t.Fatalf("door frames = %d, want exactly 1 (VTT-870, VTT-214)", len(df))
		}
		if df[0].GetEventId() != "" || df[0].GetDoorOpened() == nil || df[0].GetDoorOpened().GetAt().GetX() != 3 {
			t.Fatalf("want an event-id-less DoorOpened at 3,1, got %v", df[0])
		}
		if !v.doorOpen(3) {
			t.Fatal("viewer's fold: door 3,1 not open")
		}
	})
}

// VTT-869 VTT-870 VTT-213
func TestQADoorOpenerClosedByActorNeverSeenIsNotForwardedButItsStateIsSent(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		v.feed(qdDoor("open-plain", true, 3, ""))
		frames := v.feed(qdDoor("close-by-ghost", false, 3, "ghost"))
		if n := qdWithEventID(frames, "close-by-ghost"); n != 0 {
			t.Fatalf("forwarded with event id %d times", n)
		}
		df := qdDoorFrames(frames)
		if len(df) != 1 || df[0].GetEventId() != "" || df[0].GetDoorClosed() == nil {
			t.Fatalf("want one event-id-less DoorClosed, got %v", df)
		}
		if v.doorOpen(3) {
			t.Fatal("viewer still has the door open")
		}
	})
}

// VTT-869 VTT-214
func TestQADoorOpenerClosedBySeenOgreIsForwardedWithItsActorEvenThoughItFallsOutOfSight(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		v.feed(qdDoor("open-plain", true, 3, ""))
		if v.shown.Actors["ogre"] == nil {
			t.Fatal("fixture: ogre must be seen through the open door")
		}
		frames := v.feed(qdDoor("close-by-ogre", false, 3, "ogre"))
		df := qdDoorFrames(frames)
		if len(df) != 1 || df[0].GetEventId() != "close-by-ogre" || qdActorOf(df[0]) != "ogre" || df[0].GetDoorClosed() == nil {
			t.Fatalf("want the one forwarded DoorClosed naming ogre, got %v", df)
		}
		if v.doorOpen(3) {
			t.Fatal("viewer still has the door open")
		}
	})
}

// VTT-214
func TestQADoorOpenerOpenedByOwnCharacterNoneOrSeenActorIsForwardedWithItsEventID(t *testing.T) {
	for _, actor := range []string{"hero", "", "scout"} {
		t.Run("actor="+actor, func(t *testing.T) {
			qdEach(t, func(v *qdViewer) {
				t := v.t
				v.feedAll(qdBase()...)
				frames := v.feed(qdDoor("open-evt", true, 3, actor))
				df := qdDoorFrames(frames)
				if len(df) != 1 {
					t.Fatalf("door frames = %d, want 1: %v", len(df), df)
				}
				if df[0].GetEventId() != "open-evt" || qdActorOf(df[0]) != actor {
					t.Fatalf("want event id open-evt naming %q, got %v", actor, df[0])
				}
				if !v.doorOpen(3) {
					t.Fatal("viewer's fold: door not open")
				}
				frames = v.feed(qdDoor("close-evt", false, 3, actor))
				df = qdDoorFrames(frames)
				if len(df) != 1 || df[0].GetEventId() != "close-evt" || qdActorOf(df[0]) != actor || df[0].GetDoorClosed() == nil {
					t.Fatalf("close: want one forwarded DoorClosed naming %q, got %v", actor, df)
				}
				if v.doorOpen(3) {
					t.Fatal("viewer's fold: door still open")
				}
			})
		})
	}
}

// VTT-212
func TestQADoorOpenerDoorEventAtASquareNotSeenSendsNothingWhoeverIsNamed(t *testing.T) {
	for _, actor := range []string{"hero", "", "ghost", "ogre"} {
		t.Run("actor="+actor, func(t *testing.T) {
			qdEach(t, func(v *qdViewer) {
				t := v.t
				v.feedAll(qdBase()...)
				frames := v.feed(qdDoor("far-open", true, 5, actor))
				if len(frames) != 0 {
					t.Fatalf("frames = %d, want none for a door out of sight: %v", len(frames), frames)
				}
				if v.doorOpen(5) {
					t.Fatal("viewer learned of a door it cannot see")
				}
			})
		})
	}
}

// VTT-869
func TestQADoorOpenerUnseenActorOnADoorEventThatChangesNothingSendsNoFrame(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		v.feed(qdDoor("open-plain", true, 3, ""))
		frames := v.feed(qdDoor("reopen-by-ghost", true, 3, "ghost"))
		if len(frames) != 0 {
			t.Fatalf("opening an open door by an unseen actor sent %d frames: %v", len(frames), frames)
		}
		if !v.doorOpen(3) {
			t.Fatal("viewer's door state lost")
		}
	})
}

// VTT-869
func TestQADoorOpenerUnseenActorClosingAClosedDoorSendsNoFrame(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		frames := v.feed(qdDoor("reclose-by-ghost", false, 3, "ghost"))
		if len(frames) != 0 {
			t.Fatalf("closing a closed door by an unseen actor sent %d frames: %v", len(frames), frames)
		}
		if v.doorOpen(3) {
			t.Fatal("door open in viewer")
		}
	})
}

func TestQADoorOpenerSeenActorOnAnUnchangingDoorEventIsForwardedOnce(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		v.feed(qdDoor("open-plain", true, 3, ""))
		frames := v.feed(qdDoor("reopen-by-scout", true, 3, "scout"))
		df := qdDoorFrames(frames)
		if len(df) != 1 || df[0].GetEventId() != "reopen-by-scout" || qdActorOf(df[0]) != "scout" {
			t.Fatalf("want exactly the forwarded event naming scout, got %v", df)
		}
	})
}

// VTT-869 VTT-214
func TestQADoorOpenerOnlyTheActorsSightBeforeTheEventCountsAndNoFrameRepeats(t *testing.T) {
	qdEach(t, func(v *qdViewer) {
		t := v.t
		v.feedAll(qdBase()...)
		v.feed(qdDoor("open-by-ogre", true, 3, "ogre"))
		if v.shown.Actors["ogre"] == nil {
			t.Fatal("fixture: ogre should now be seen through the open door")
		}
		frames := v.feed(qdDoor("close-by-ogre", false, 3, "ogre"))
		df := qdDoorFrames(frames)
		if len(df) != 1 || df[0].GetEventId() != "close-by-ogre" || qdActorOf(df[0]) != "ogre" {
			t.Fatalf("want forwarded DoorClosed naming the now-seen ogre, got %v", df)
		}
	})
}
