package gateway_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

var qaDoorRoles = []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer}

func qaDoorCmd(open bool, actor string) *vttv1.ClientCommand {
	at := &vttv1.GridPosition{X: 7, Y: 3}
	if open {
		return &vttv1.ClientCommand{RequestId: "r1", Command: &vttv1.ClientCommand_OpenDoor{
			OpenDoor: &vttv1.OpenDoor{SceneId: "scene-q", At: at, ActorId: actor}}}
	}
	return &vttv1.ClientCommand{RequestId: "r1", Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: "scene-q", At: at, ActorId: actor}}}
}

func qaDoorEvent(t *testing.T, open bool, actor string, role identity.Role) (scene string, x, y int32, got string) {
	t.Helper()
	env, err := gateway.ToEvent(qaDoorCmd(open, actor), &identity.Participant{ID: "p-q", Name: "Q", Role: role})
	if err != nil {
		t.Fatalf("ToEvent: %v", err)
	}
	if open {
		d := env.GetDoorOpened()
		if d == nil {
			t.Fatalf("payload is %T, want DoorOpened", env.GetPayload())
		}
		return d.GetSceneId(), d.GetAt().GetX(), d.GetAt().GetY(), d.GetActorId()
	}
	d := env.GetDoorClosed()
	if d == nil {
		t.Fatalf("payload is %T, want DoorClosed", env.GetPayload())
	}
	return d.GetSceneId(), d.GetAt().GetX(), d.GetAt().GetY(), d.GetActorId()
}

// VTT-503
func TestQADoorOpenedNamesTheActorTheCommandNamedForEveryRole(t *testing.T) {
	for _, r := range qaDoorRoles {
		s, x, y, a := qaDoorEvent(t, true, "actor-tobbe", r)
		if s != "scene-q" || x != 7 || y != 3 || a != "actor-tobbe" {
			t.Errorf("%s: got %q (%d,%d) actor %q", r, s, x, y, a)
		}
	}
}

// VTT-503
func TestQADoorClosedNamesTheActorTheCommandNamedForEveryRole(t *testing.T) {
	for _, r := range qaDoorRoles {
		s, x, y, a := qaDoorEvent(t, false, "actor-tobbe", r)
		if s != "scene-q" || x != 7 || y != 3 || a != "actor-tobbe" {
			t.Errorf("%s: got %q (%d,%d) actor %q", r, s, x, y, a)
		}
	}
}

// VTT-503
func TestQADoorOpenedNamesNoActorWhenTheCommandNamedNone(t *testing.T) {
	for _, r := range qaDoorRoles {
		s, x, y, a := qaDoorEvent(t, true, "", r)
		if s != "scene-q" || x != 7 || y != 3 || a != "" {
			t.Errorf("%s: got %q (%d,%d) actor %q", r, s, x, y, a)
		}
	}
}

// VTT-503
func TestQADoorClosedNamesNoActorWhenTheCommandNamedNone(t *testing.T) {
	for _, r := range qaDoorRoles {
		s, x, y, a := qaDoorEvent(t, false, "", r)
		if s != "scene-q" || x != 7 || y != 3 || a != "" {
			t.Errorf("%s: got %q (%d,%d) actor %q", r, s, x, y, a)
		}
	}
}

func TestQADoorMessagesWriteActorIdOnTheWireAndOmitAnEmptyOne(t *testing.T) {
	at := &vttv1.GridPosition{X: 7, Y: 3}
	msgs := map[string][2]proto.Message{
		"OpenDoor":   {&vttv1.OpenDoor{SceneId: "s", At: at, ActorId: "actor-tobbe"}, &vttv1.OpenDoor{SceneId: "s", At: at}},
		"CloseDoor":  {&vttv1.CloseDoor{SceneId: "s", At: at, ActorId: "actor-tobbe"}, &vttv1.CloseDoor{SceneId: "s", At: at}},
		"DoorOpened": {&vttv1.DoorOpened{SceneId: "s", At: at, ActorId: "actor-tobbe"}, &vttv1.DoorOpened{SceneId: "s", At: at}},
		"DoorClosed": {&vttv1.DoorClosed{SceneId: "s", At: at, ActorId: "actor-tobbe"}, &vttv1.DoorClosed{SceneId: "s", At: at}},
	}
	for name, pair := range msgs {
		b, err := protojson.Marshal(pair[0])
		if err != nil || !strings.Contains(string(b), `"actorId":"actor-tobbe"`) {
			t.Errorf("%s named: %s (%v)", name, b, err)
		}
		b, err = protojson.Marshal(pair[1])
		if err != nil || strings.Contains(string(b), "actorId") || strings.Contains(string(b), "actor_id") {
			t.Errorf("%s empty: %s (%v)", name, b, err)
		}
	}
	var back vttv1.OpenDoor
	if err := protojson.Unmarshal([]byte(`{"sceneId":"s","at":{"x":7,"y":3},"actorId":"actor-tobbe"}`), &back); err != nil || back.GetActorId() != "actor-tobbe" {
		t.Errorf("unmarshal OpenDoor: %v %q", err, back.GetActorId())
	}
	var ev vttv1.DoorClosed
	if err := protojson.Unmarshal([]byte(`{"sceneId":"s","at":{"x":7,"y":3},"actorId":"actor-tobbe"}`), &ev); err != nil || ev.GetActorId() != "actor-tobbe" {
		t.Errorf("unmarshal DoorClosed: %v %q", err, ev.GetActorId())
	}
}
