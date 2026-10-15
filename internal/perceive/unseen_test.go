package perceive_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// unseenWorld feeds a player who controls hero, standing at (1,1) before the
// shut door at (3,1), every event it is handed, from an empty world.
type unseenWorld struct {
	tb  testing.TB
	st  *engine.State
	v   *testViewer
	seq int64
}

func newUnseenWorld(tb testing.TB) *unseenWorld {
	tb.Helper()
	w := &unseenWorld{tb: tb, st: engine.NewState(), v: newViewer(tb, perceive.Eyes{Controller: "p-1"})}
	w.feed(&vttv1.SessionStarted{Name: "n"})
	w.feed(&vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()})
	w.feed(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	w.feed(&vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
		Position: &vttv1.GridPosition{X: 1, Y: 1}})
	w.feed(&vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return w
}

func (w *unseenWorld) feed(payload proto.Message) []*vttv1.Envelope {
	w.tb.Helper()
	w.seq++
	env := envelope(w.seq, payload)
	if err := engine.Apply(w.st, env); err != nil {
		w.tb.Fatalf("fixture: the world refused event %d: %v", w.seq, err)
	}
	return w.v.event(env, w.st)
}

// addScout stands scout at (2,1), in hero's sight, with the weight given.
func (w *unseenWorld) addScout(weight int32) []*vttv1.Envelope {
	w.feed(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, Attributes: map[string]int32{"weight": weight}}})
	return w.feed(&vttv1.TokenPlaced{TokenId: "t-scout", SceneId: "s", ActorId: "scout",
		Position: &vttv1.GridPosition{X: 2, Y: 1}})
}

// VTT-213
func TestADoorClosedWhileTheViewerSawNothingIsCorrectedOnSight(t *testing.T) {
	w := newUnseenWorld(t)
	w.feed(&vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	if !w.v.shown.Scenes["s"].OpenDoors[key(3, 1)] {
		t.Fatal("fixture: the viewer must see the door open before it leaves sight")
	}
	w.feed(&vttv1.TokenRemoved{TokenId: "t-hero"})
	w.feed(&vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	if !w.v.shown.Scenes["s"].OpenDoors[key(3, 1)] {
		t.Fatal("a door closed while the viewer had no eyes reached it")
	}
	back := w.feed(&vttv1.TokenPlaced{TokenId: "t-hero-2", SceneId: "s", ActorId: "hero",
		Position: &vttv1.GridPosition{X: 1, Y: 1}})
	closed := 0
	for _, f := range back {
		if dc := f.GetDoorClosed(); dc != nil && dc.GetAt().GetX() == 3 && dc.GetAt().GetY() == 1 {
			closed++
		}
	}
	if closed != 1 || w.v.shown.Scenes["s"].OpenDoors[key(3, 1)] {
		t.Errorf("on sight the viewer must be sent one DoorClosed at (3,1) and hold the door shut; sent %d", closed)
	}
}

// VTT-248
func TestAControllerRevokedWhileUnseenIsRevokedOnSight(t *testing.T) {
	w := newUnseenWorld(t)
	w.addScout(1)
	w.feed(&vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	w.feed(&vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-3", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	if got := w.v.shown.Actors["scout"].GetControllerIds(); len(got) != 2 {
		t.Fatalf("fixture: the viewer must see both grants, holds %v", got)
	}
	w.feed(&vttv1.TokenRemoved{TokenId: "t-scout"})
	w.feed(&vttv1.ActorControlRevoked{ActorId: "scout", ParticipantId: "p-3"})
	back := w.feed(&vttv1.TokenPlaced{TokenId: "t-scout-2", SceneId: "s", ActorId: "scout",
		Position: &vttv1.GridPosition{X: 2, Y: 1}})
	var revoked []string
	for _, f := range back {
		if r := f.GetActorControlRevoked(); r != nil {
			revoked = append(revoked, r.GetParticipantId())
		}
	}
	if len(revoked) != 1 || revoked[0] != "p-3" {
		t.Errorf("on sight the viewer must be sent one ActorControlRevoked for p-3, got %v", revoked)
	}
	if got := w.v.shown.Actors["scout"].GetControllerIds(); len(got) != 1 || got[0] != "p-2" {
		t.Errorf("the viewer must end holding scout controlled by p-2 alone, holds %v", got)
	}
}

// VTT-313
func TestAnActorAddedAgainWithAnotherAttributeIsIntroducedAfresh(t *testing.T) {
	w := newUnseenWorld(t)
	w.addScout(1)
	w.feed(&vttv1.TokenRemoved{TokenId: "t-scout"})
	w.feed(&vttv1.ActorRemoved{ActorId: "scout"})
	w.feed(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, Attributes: map[string]int32{"weight": 2}}})
	back := w.feed(&vttv1.TokenPlaced{TokenId: "t-scout-2", SceneId: "s", ActorId: "scout",
		Position: &vttv1.GridPosition{X: 2, Y: 1}})
	var removed, added bool
	for _, f := range back {
		if f.GetActorRemoved().GetActorId() == "scout" {
			removed = !added
		}
		if f.GetActorAdded().GetActor().GetActorId() == "scout" {
			added = true
		}
	}
	if !removed || !added {
		t.Errorf("an actor added again with another attribute must be withdrawn, then introduced; removed %v, added %v", removed, added)
	}
	if got := w.v.shown.Actors["scout"].GetAttributes()["weight"]; got != 2 {
		t.Errorf("the viewer must hold the new scout's weight 2, holds %d", got)
	}
}
