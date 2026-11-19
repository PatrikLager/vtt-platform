package gateway_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const qdNoSuch = "There is no such character."

func qdTiles(w, h int, doors ...string) map[string]*vttv1.TileRef {
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	for _, d := range doors {
		tiles[d] = &vttv1.TileRef{Kind: "door"}
	}
	return tiles
}

// qdWorld: scene qd-scn has a door at 1,1; pc stands beside it, pc2 and the
// mob stand far away; qd-bare has no token; scene qd-far has a door and no token.
func qdWorld(t *testing.T) *gwFixture {
	t.Helper()
	f := newGWFixture(t)
	mustAppend(t, f.campaign, "qd-1", &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "qd-scn", Name: "qd", GridWidth: 8, GridHeight: 3, Tiles: qdTiles(8, 3, "1,1")}})
	mustAppend(t, f.campaign, "qd-2", &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "qd-far", Name: "qdfar", GridWidth: 8, GridHeight: 3, Tiles: qdTiles(8, 3, "0,0")}})
	for i, a := range []struct {
		id   string
		kind vttv1.ActorKind
	}{
		{"qd-pc", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		{"qd-pc2", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		{"qd-mob", vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		{"qd-bare", vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
	} {
		mustAppend(t, f.campaign, fmt.Sprintf("qd-a%d", i), &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: a.id, Name: a.id, Kind: a.kind}}})
	}
	for i, id := range []string{"qd-pc", "qd-pc2"} {
		mustAppend(t, f.campaign, fmt.Sprintf("qd-g%d", i), &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: id, ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	}
	for i, tk := range []struct {
		tok, actor string
		x, y       int32
	}{{"qd-tok-pc", "qd-pc", 2, 1}, {"qd-tok-pc2", "qd-pc2", 7, 2}, {"qd-tok-mob", "qd-mob", 7, 0}} {
		mustAppend(t, f.campaign, fmt.Sprintf("qd-t%d", i), &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tk.tok, SceneId: "qd-scn", ActorId: tk.actor, Position: &vttv1.GridPosition{X: tk.x, Y: tk.y}}})
	}
	return f
}

func qdCmd(open bool, rid, scene string, x, y int32, actor string) *vttv1.ClientCommand {
	at := &vttv1.GridPosition{X: x, Y: y}
	if open {
		return &vttv1.ClientCommand{RequestId: rid, Command: &vttv1.ClientCommand_OpenDoor{
			OpenDoor: &vttv1.OpenDoor{SceneId: scene, At: at, ActorId: actor}}}
	}
	return &vttv1.ClientCommand{RequestId: rid, Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: scene, At: at, ActorId: actor}}}
}

func qdOpener(env *vttv1.Envelope) (string, bool) {
	if e := env.GetDoorOpened(); e != nil {
		return e.GetActorId(), true
	}
	if e := env.GetDoorClosed(); e != nil {
		return e.GetActorId(), true
	}
	return "", false
}

func qdNoDoorEvent(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	for _, env := range drainEvents(t, conn, 300*time.Millisecond) {
		if _, ok := qdOpener(env); ok {
			t.Fatalf("third connection received a door event: %v", env)
		}
	}
}

// VTT-867 VTT-503
func TestQADoorGateDMAndAgentNameAnyActorOrNone(t *testing.T) {
	f := qdWorld(t)
	for _, who := range []struct{ name, tok string }{{"dm", f.dmToken}, {"agent", f.agentToken}} {
		conn := f.dial(who.tok, f.head(t))
		defer conn.CloseNow()
		for _, open := range []bool{true, false} {
			for _, actor := range []string{"qd-pc", "qd-pc2", "qd-mob", "qd-bare", ""} {
				name := fmt.Sprintf("%s/open=%v/%q", who.name, open, actor)
				accepted(t, conn, qdCmd(open, "qd-r-"+name, "qd-scn", 1, 1, actor))
				got, ok := qdOpener(readEvent(t, conn))
				if !ok || got != actor {
					t.Fatalf("%s: door event names %q (door event=%v), want %q", name, got, ok, actor)
				}
			}
		}
	}
}

// VTT-868 VTT-162
func TestQADoorGateAbsentActorRefusedForEveryRole(t *testing.T) {
	f := qdWorld(t)
	bad := []string{"nope", "hasOwnProperty", "__proto__", "constructor", "qd-tok-pc", "qd-scn", "QD-PC", "qd-pc ", " qd-pc", "qd-pc\x00", "qd-pc2,qd-pc"}
	for _, who := range []struct{ name, tok, third string }{
		{"dm", f.dmToken, f.agentToken}, {"agent", f.agentToken, f.dmToken}, {"player", f.playerToken, f.agentToken}} {
		conn := f.dial(who.tok, f.head(t))
		defer conn.CloseNow()
		third := f.dial(who.third, f.head(t))
		defer third.CloseNow()
		for _, open := range []bool{true, false} {
			for _, actor := range bad {
				name := fmt.Sprintf("%s/open=%v/%q", who.name, open, actor)
				if got := f.refusedUnmoved(t, conn, qdCmd(open, "qd-r-"+name, "qd-scn", 1, 1, actor)); got != qdNoSuch {
					t.Fatalf("%s: refusal %q, want %q", name, got, qdNoSuch)
				}
			}
		}
		qdNoDoorEvent(t, third)
	}
}

// VTT-608
func TestQADoorGateNotADoorIsAnsweredBeforeAbsentActor(t *testing.T) {
	f := qdWorld(t)
	for _, who := range []struct{ name, tok string }{{"dm", f.dmToken}, {"agent", f.agentToken}} {
		conn := f.dial(who.tok, f.head(t))
		defer conn.CloseNow()
		if got := f.refusedUnmoved(t, conn, qdCmd(true, "qd-nd-"+who.name, "qd-scn", 3, 1, "nope")); got != "There is no door there." {
			t.Fatalf("%s: %q", who.name, got)
		}
	}
	conn := f.dial(f.playerToken, f.head(t))
	defer conn.CloseNow()
	if got := f.refusedUnmoved(t, conn, qdCmd(true, "qd-nd-pl", "qd-scn", 3, 1, "nope")); got == qdNoSuch {
		t.Fatalf("player at a non-door square got %q", got)
	}
}

// VTT-880
func TestQADoorGatePlayerFarFromTheDoorGetsThePlayerRule(t *testing.T) {
	f := qdWorld(t)
	conn := f.dial(f.playerToken, f.head(t))
	defer conn.CloseNow()
	farRefusal := f.refusedUnmoved(t, conn, qdCmd(true, "qd-far-1", "qd-far", 0, 0, ""))
	if farRefusal == qdNoSuch || farRefusal == "There is no door there." {
		t.Fatalf("far player named none and got %q", farRefusal)
	}
	if got := f.refusedUnmoved(t, conn, qdCmd(true, "qd-far-2", "qd-far", 0, 0, "nope")); got != farRefusal {
		t.Fatalf("far player naming an absent actor got %q, want the player rule's %q", got, farRefusal)
	}
	if got := f.refusedUnmoved(t, conn, qdCmd(false, "qd-far-3", "qd-far", 0, 0, "hasOwnProperty")); got != farRefusal {
		t.Fatalf("far player (close) got %q, want %q", got, farRefusal)
	}
}

// VTT-149 VTT-503
func TestQADoorGatePlayerBesideDoorNamingNoneIsAccepted(t *testing.T) {
	f := qdWorld(t)
	conn := f.dial(f.playerToken, f.head(t))
	defer conn.CloseNow()
	witness := f.dial(f.dmToken, f.head(t))
	defer witness.CloseNow()
	for _, open := range []bool{true, false} {
		accepted(t, conn, qdCmd(open, fmt.Sprintf("qd-pn-%v", open), "qd-scn", 1, 1, ""))
		got, ok := qdOpener(readEvent(t, witness))
		if !ok || got != "" {
			t.Fatalf("open=%v: door event=%v names %q, want none", open, ok, got)
		}
	}
}
