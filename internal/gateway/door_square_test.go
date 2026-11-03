package gateway_test

import (
	"fmt"
	"testing"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const noDoorThere = "There is no door there."

func doorCmd(requestID string, open bool, sceneID string, x, y int32) *vttv1.ClientCommand {
	at := &vttv1.GridPosition{X: x, Y: y}
	if open {
		return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_OpenDoor{
			OpenDoor: &vttv1.OpenDoor{SceneId: sceneID, At: at}}}
	}
	return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: sceneID, At: at}}}
}

// VTT-608 VTT-147
func TestOnlyADoorOpensOrClosesForEveryRole(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	agent := f.dial(f.agentToken, f.head(t))
	dm := f.dial(f.dmToken, f.head(t))
	player := f.dial(f.playerToken, f.head(t))
	type square struct {
		name, scene string
		x, y        int32
	}
	wall := square{"wall", "cellar", 2, 0}
	floor := square{"floor", "cellar", 1, 1}
	bare := square{"no-tile", "scn1", 5, 5}
	off := square{"off-the-grid", "cellar", 5, 5}
	nowhere := square{"no-such-scene", "nowhere", 0, 1}
	for _, c := range []struct {
		who  string
		conn *websocket.Conn
		sq   square
	}{
		{"agent", agent, wall}, {"dm", dm, wall},
		{"agent", agent, floor}, {"dm", dm, floor},
		{"agent", agent, bare}, {"dm", dm, bare},
		{"agent", agent, off}, {"dm", dm, off},
		{"agent", agent, nowhere}, {"dm", dm, nowhere},
		{"player", player, wall}, {"player", player, floor},
		{"player", player, square{"no-tile-beside-t1", "scn1", 3, 6}},
	} {
		for _, open := range []bool{true, false} {
			id := fmt.Sprintf("%s-open-%v-%s", c.who, open, c.sq.name)
			if got := f.refusedUnmoved(t, c.conn, doorCmd(id, open, c.sq.scene, c.sq.x, c.sq.y)); got != noDoorThere {
				t.Fatalf("%s was refused with %q, want %q", id, got, noDoorThere)
			}
		}
	}
	for who, conn := range map[string]*websocket.Conn{"agent": agent, "dm": dm} {
		accepted(t, conn, doorCmd(who+"-opens-the-door", true, "cellar", 0, 1))
		accepted(t, conn, doorCmd(who+"-closes-the-door", false, "cellar", 0, 1))
	}
	accepted(t, dm, cellarMove("beside-the-door", 1, 1))
	accepted(t, player, doorCmd("player-opens-the-door", true, "cellar", 0, 1))
	accepted(t, player, doorCmd("player-closes-the-door", false, "cellar", 0, 1))
}

// VTT-609
func TestAPlayerFarFromASquareIsToldTheSameWhetherItIsADoor(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	player := f.dial(f.playerToken, f.head(t))
	far := f.refusedUnmoved(t, player, doorCmd("far-wall", true, "cellar", 0, 0))
	if far == noDoorThere {
		t.Fatalf("a far wall was refused with the door gate's %q", far)
	}
	for _, open := range []bool{true, false} {
		for _, c := range []struct {
			name, scene string
			x, y        int32
		}{{"wall", "cellar", 0, 0}, {"door", "cellar", 0, 1}, {"no-such-scene", "nowhere", 0, 1}} {
			id := fmt.Sprintf("far-open-%v-%s", open, c.name)
			if got := f.refusedUnmoved(t, player, doorCmd(id, open, c.scene, c.x, c.y)); got != far {
				t.Fatalf("%s was refused with %q, want the far wall's %q", id, got, far)
			}
		}
	}
	if got := f.refusedUnmoved(t, player, doorCmd("beside-wall", true, "cellar", 2, 0)); got != noDoorThere {
		t.Fatalf("a wall beside the player's token was refused with %q, want %q", got, noDoorThere)
	}
}
