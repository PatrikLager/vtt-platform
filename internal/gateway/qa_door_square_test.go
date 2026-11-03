package gateway_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

const (
	qaDSNoDoor   = "There is no door there."
	qaDSFarWords = "has no token adjacent to that door"
)

type qaDSSquare struct {
	scene string
	x, y  int32
}

func qaDSOpen(sq qaDSSquare) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		RequestId: "qa-ds-open",
		Command: &vttv1.ClientCommand_OpenDoor{OpenDoor: &vttv1.OpenDoor{
			SceneId: sq.scene, At: &vttv1.GridPosition{X: sq.x, Y: sq.y},
		}},
	}
}

func qaDSClose(sq qaDSSquare) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		RequestId: "qa-ds-close",
		Command: &vttv1.ClientCommand_CloseDoor{CloseDoor: &vttv1.CloseDoor{
			SceneId: sq.scene, At: &vttv1.GridPosition{X: sq.x, Y: sq.y},
		}},
	}
}

func qaDSBoth(sq qaDSSquare) map[string]*vttv1.ClientCommand {
	return map[string]*vttv1.ClientCommand{
		"open_door": qaDSOpen(sq), "close_door": qaDSClose(sq),
	}
}

func qaDSSeedHall(t *testing.T, f *gwFixture) {
	t.Helper()
	tiles := map[string]*vttv1.TileRef{
		"0,1": {Kind: "floor"}, "1,1": {Kind: "door"},
		"2,1": {Kind: "floor"}, "3,1": {Kind: "wall"},
	}
	mustAppend(t, f.campaign, "qa-ds-scene", &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: "qa-hall", Name: "QA Hall",
			GridWidth: 4, GridHeight: 3, Tiles: tiles,
		},
	})
	mustAppend(t, f.campaign, "qa-ds-actor", &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "qa-runner", Name: "Runner",
		}},
	})
	mustAppend(t, f.campaign, "qa-ds-grant", &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "qa-runner", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		},
	})
	mustAppend(t, f.campaign, "qa-ds-place", &vttv1.Envelope_TokenPlaced{
		TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "qa-tok", SceneId: "qa-hall", ActorId: "qa-runner",
			Position: &vttv1.GridPosition{X: 0, Y: 1},
		},
	})
}

func qaDSDoorOpen(t *testing.T, f *gwFixture, sq qaDSSquare) bool {
	t.Helper()
	sc, ok := f.campaign.State().Scenes[sq.scene]
	if !ok {
		t.Fatalf("scene %q not in state", sq.scene)
	}
	return sc.OpenDoors[qaDSKey(sq)]
}

func qaDSKey(sq qaDSSquare) string {
	return fmt.Sprintf("%d,%d", sq.x, sq.y)
}

func qaDSRefusedWith(
	t *testing.T, f *gwFixture, conn *websocket.Conn,
	who string, squares []qaDSSquare, want string,
) {
	t.Helper()
	for _, sq := range squares {
		for name, cmd := range qaDSBoth(sq) {
			got := f.refusedUnmoved(t, conn, cmd)
			if got != want {
				t.Errorf("%s %s at %+v: refusal %q, want %q",
					who, name, sq, got, want)
			}
		}
	}
}

func qaDSNotDoors() []qaDSSquare {
	return []qaDSSquare{
		{"cellar", 0, 0}, {"cellar", 1, 2}, {"cellar", 2, 0},
		{"cellar", 1, 1}, {"cellar", 2, 1},
		{"cellar", 3, 1}, {"cellar", -1, 1}, {"cellar", 0, -1},
		{"cellar", 0, 3}, {"cellar", 1000, -1000},
		{"scn1", 0, 1}, {"scn1", 3, 7}, {"scn1", 9, 9},
		{"scn1", 10, 1}, {"scn1", 0, 10},
		{"qa-hall", 1, 0}, {"qa-hall", 1, 2}, {"qa-hall", 0, 1},
		{"qa-hall", 2, 1}, {"qa-hall", 3, 1}, {"qa-hall", 4, 1},
		{"qa-hall", 1, 3}, {"qa-hall", 0, 0},
		{"qa-nowhere", 0, 1}, {"qa-nowhere", 1, 1}, {"", 0, 1},
	}
}

// VTT-608 VTT-162
func TestQADSTheDMAndTheAgentAreRefusedAtEverySquareThatIsNotADoor(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	for who, tok := range map[string]string{
		"dm": f.dmToken, "agent": f.agentToken,
	} {
		conn := f.dial(tok, f.head(t))
		qaDSRefusedWith(t, f, conn, who, qaDSNotDoors(), qaDSNoDoor)
	}
}

// VTT-147
func TestQADSTheDMAndTheAgentWorkADoorFromAnywhereOpenOrClosed(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	dm := f.dial(f.dmToken, f.head(t))
	agent := f.dial(f.agentToken, f.head(t))
	for _, door := range []qaDSSquare{{"cellar", 0, 1}, {"qa-hall", 1, 1}} {
		steps := []struct {
			conn *websocket.Conn
			cmd  *vttv1.ClientCommand
			open bool
		}{
			{dm, qaDSOpen(door), true},
			{agent, qaDSClose(door), false},
			{agent, qaDSOpen(door), true},
			{dm, qaDSClose(door), false},
		}
		for i, s := range steps {
			before := f.head(t)
			seq := accepted(t, s.conn, s.cmd)
			if seq <= before {
				t.Errorf("%+v step %d: sequence %d, head was %d",
					door, i, seq, before)
			}
			if got := qaDSDoorOpen(t, f, door); got != s.open {
				t.Errorf("%+v step %d: open=%v, want %v", door, i, got, s.open)
			}
		}
	}
}

// VTT-608 VTT-162
func TestQADSAPlayerBesideASquareThatIsNotADoorGetsTheDoorText(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	conn := f.dial(f.playerToken, f.head(t))
	beside := []qaDSSquare{
		{"cellar", 2, 1}, {"cellar", 1, 1}, {"cellar", 1, 0},
		{"cellar", 2, 0}, {"cellar", 1, 2}, {"cellar", 2, 2},
		{"cellar", 3, 0}, {"cellar", 3, 1}, {"cellar", 3, 2},
		{"scn1", 3, 7}, {"scn1", 2, 6}, {"scn1", 4, 8}, {"scn1", 4, 7},
		{"qa-hall", 0, 1}, {"qa-hall", 0, 0}, {"qa-hall", 1, 0},
		{"qa-hall", 1, 2}, {"qa-hall", 0, 2},
		{"qa-hall", -1, 0}, {"qa-hall", -1, 1}, {"qa-hall", -1, 2},
	}
	qaDSRefusedWith(t, f, conn, "player", beside, qaDSNoDoor)
}

// VTT-149
func TestQADSAPlayerBesideADoorWorksItOpenOrClosed(t *testing.T) {
	f := newGWFixture(t)
	qaDSSeedHall(t, f)
	conn := f.dial(f.playerToken, f.head(t))
	door := qaDSSquare{"qa-hall", 1, 1}
	steps := []struct {
		cmd  *vttv1.ClientCommand
		open bool
	}{
		{qaDSOpen(door), true}, {qaDSClose(door), false},
		{qaDSOpen(door), true},
	}
	for i, s := range steps {
		accepted(t, conn, s.cmd)
		if got := qaDSDoorOpen(t, f, door); got != s.open {
			t.Errorf("step %d: open=%v, want %v", i, got, s.open)
		}
	}
}

func qaDSOneFarText(
	t *testing.T, f *gwFixture, conn *websocket.Conn,
	who string, squares []qaDSSquare,
) {
	t.Helper()
	first := ""
	for _, sq := range squares {
		for name, cmd := range qaDSBoth(sq) {
			got := f.refusedUnmoved(t, conn, cmd)
			if first == "" {
				first = got
			}
			if got != first {
				t.Errorf("%s %s at %+v: refusal %q, want the one text %q",
					who, name, sq, got, first)
			}
		}
	}
	if first == qaDSNoDoor || !strings.Contains(first, qaDSFarWords) {
		t.Errorf("%s far refusal %q: want mayWorkDoor's words", who, first)
	}
}

// VTT-609 VTT-149
func TestQADSAPlayerFarFromTheSquareGetsOneTextWhateverTheSquareIs(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	accepted(t, f.dial(f.dmToken, f.head(t)), qaDSOpen(qaDSSquare{"cellar", 0, 1}))
	conn := f.dial(f.playerToken, f.head(t))
	far := []qaDSSquare{
		{"cellar", 0, 1}, {"cellar", 0, 0}, {"cellar", 0, 2},
		{"cellar", 5, 1}, {"cellar", 3, 7}, {"cellar", -1, 1},
		{"qa-hall", 3, 1}, {"qa-hall", 2, 1}, {"qa-hall", 2, 0},
		{"qa-hall", 5, 5}, {"qa-hall", 3, 7},
		{"scn1", 9, 9}, {"scn1", 5, 7}, {"scn1", 3, 9}, {"scn1", 2, 1},
		{"qa-nowhere", 0, 1}, {"qa-nowhere", 3, 7}, {"qa-nowhere", 2, 1},
	}
	qaDSOneFarText(t, f, conn, "lera", far)
	if !qaDSDoorOpen(t, f, qaDSSquare{"cellar", 0, 1}) {
		t.Errorf("a refused close_door shut the cellar door")
	}
}

// VTT-609
func TestQADSAPlayerWhoControlsNothingGetsOneTextAtEverySquare(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	conn := f.dial(f.otherPlayerToken, f.head(t))
	squares := []qaDSSquare{
		{"cellar", 0, 1}, {"cellar", 0, 0}, {"cellar", 2, 1},
		{"qa-hall", 1, 1}, {"qa-hall", 0, 1}, {"scn1", 3, 7},
		{"qa-nowhere", 0, 1},
	}
	qaDSOneFarText(t, f, conn, "ivo", squares)
}

// VTT-139
func TestQADSASpectatorIsRefusedByItsRoleAtADoorAndAtAWall(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	conn := f.dial(f.spectatorToken, f.head(t))
	first := map[string]string{}
	for _, sq := range []qaDSSquare{
		{"cellar", 0, 1}, {"cellar", 0, 0}, {"qa-nowhere", 0, 1},
	} {
		for name, cmd := range qaDSBoth(sq) {
			got := f.refusedUnmoved(t, conn, cmd)
			if first[name] == "" {
				first[name] = got
			}
			if got != first[name] || !strings.Contains(got, `"spectator"`) {
				t.Errorf("spectator %s at %+v: refusal %q (first %q)",
					name, sq, got, first[name])
			}
		}
	}
}

func qaDSDoorEvents(envs []*vttv1.Envelope) int {
	n := 0
	for _, e := range envs {
		switch e.GetPayload().(type) {
		case *vttv1.Envelope_DoorOpened, *vttv1.Envelope_DoorClosed:
			n++
		}
	}
	return n
}

// VTT-162 VTT-608
func TestQADSARefusalAtANonDoorReachesNoOtherConnection(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	watcher := f.dial(f.dmToken, f.head(t))
	agent := f.dial(f.agentToken, f.head(t))
	player := f.dial(f.playerToken, f.head(t))
	f.refusedUnmoved(t, agent, qaDSOpen(qaDSSquare{"cellar", 0, 0}))
	f.refusedUnmoved(t, player, qaDSClose(qaDSSquare{"cellar", 2, 0}))
	f.refusedUnmoved(t, agent, qaDSOpen(qaDSSquare{"qa-nowhere", 0, 1}))
	if n := qaDSDoorEvents(drainEvents(t, watcher, 300*time.Millisecond)); n != 0 {
		t.Errorf("watcher received %d door events from refusals", n)
	}
	accepted(t, agent, qaDSOpen(qaDSSquare{"cellar", 0, 1}))
	if n := qaDSDoorEvents(drainEvents(t, watcher, 300*time.Millisecond)); n != 1 {
		t.Errorf("watcher received %d door events from one open, want 1", n)
	}
}

func TestQADSIsDoorAnswersForADoorTileOpenOrClosed(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	qaDSSeedHall(t, f)
	want := map[qaDSSquare]bool{
		{"cellar", 0, 1}: true, {"qa-hall", 1, 1}: true,
		{"cellar", 0, 0}: false, {"cellar", 1, 1}: false,
		{"cellar", 3, 1}: false, {"cellar", -1, 1}: false,
		{"qa-hall", 1, 0}: false, {"scn1", 0, 1}: false,
		{"qa-nowhere", 0, 1}: false, {"scn1", 1, 1}: false,
	}
	check := func(when string) {
		st := f.campaign.State()
		for sq, w := range want {
			if got := st.IsDoor(sq.scene, sq.x, sq.y); got != w {
				t.Errorf("%s: IsDoor%+v = %v, want %v", when, sq, got, w)
			}
		}
	}
	check("closed")
	accepted(t, f.dial(f.dmToken, f.head(t)), qaDSOpen(qaDSSquare{"cellar", 0, 1}))
	check("open")
}

// VTT-608
func TestQADSATileNearlyADoorIsNotADoor(t *testing.T) {
	f := newGWFixture(t)
	mustAppend(t, f.campaign, "qa-ds-near", &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: "qa-near", Name: "QA Near", GridWidth: 4, GridHeight: 1,
			Tiles: map[string]*vttv1.TileRef{
				"0,0": {Kind: "Door"}, "1,0": {Kind: " door"},
				"2,0": {Kind: ""}, "3,0": {Kind: "doors"},
			},
		},
	})
	conn := f.dial(f.dmToken, f.head(t))
	squares := []qaDSSquare{
		{"qa-near", 0, 0}, {"qa-near", 1, 0}, {"qa-near", 2, 0}, {"qa-near", 3, 0},
	}
	for _, sq := range squares {
		if f.campaign.State().IsDoor(sq.scene, sq.x, sq.y) {
			t.Errorf("IsDoor%+v = true for a tile that is not a door", sq)
		}
	}
	qaDSRefusedWith(t, f, conn, "dm", squares, qaDSNoDoor)
}
