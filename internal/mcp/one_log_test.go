package mcp_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func sendSwitch(t *testing.T, conn *websocket.Conn, actorID string) {
	t.Helper()
	sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_ViewpointChanged{
		ViewpointChanged: &vttv1.ViewpointChanged{ActorId: actorID},
	}})
}

func named(log json.RawMessage) string {
	if log == nil {
		return "no log"
	}
	return "log " + string(log)
}

func waitForLog(t *testing.T, cs *mcpsdk.ClientSession, log string, head int64) getStateShape {
	t.Helper()
	want, _ := json.Marshal(log)
	deadline := time.Now().Add(5 * time.Second)
	var last getStateShape
	for time.Now().Before(deadline) {
		last, _ = callGetState(t, cs)
		if string(last.Log) == string(want) && last.HeadSequence == head {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("get_state never named log %q at head %d; last %s at head %d", log, head, named(last.Log), last.HeadSequence)
	return last
}

func waitForConns(t *testing.T, fs *fakeServer, n int) *websocket.Conn {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		fs.mu.Lock()
		if len(fs.conns) >= n {
			c := fs.conns[n-1]
			fs.mu.Unlock()
			return c
		}
		fs.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fakeServer: no connection %d in time", n)
	return nil
}

var a1Log = []*vttv1.Envelope{
	canned(1, "", &vttv1.SessionStarted{Name: "s1"}),
	canned(2, "", &vttv1.SceneCreated{SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10}),
	canned(3, "", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-1", Name: "Ursus"}}),
}

// VTT-418 VTT-422
func TestASwitchedPlayerIsAnsweredFromTheNewLogAlone(t *testing.T) {
	fs := newFakeServer(t, nil)
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	conn := fs.firstConn(t)
	sendSwitch(t, conn, "a1")
	seedEvents(t, conn, a1Log...)
	waitForLog(t, cs, "a1", 3)
	sendSwitch(t, conn, "a2")
	seedEvents(t, conn,
		canned(1, "", &vttv1.SessionStarted{Name: "s1"}),
		canned(2, "", &vttv1.SceneCreated{SceneId: "scn-2", Name: "Cellar", GridWidth: 10, GridHeight: 10}))

	st := waitForLog(t, cs, "a2", 2)
	if len(st.Scenes) != 1 || st.Scenes["scn-2"] == nil {
		t.Fatalf("get_state after the switch folds scenes %s, want only a2's scn-2", st.Scenes)
	}
	since := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0})
	if len(since.Events) != 2 || string(since.Log) != `"a2"` {
		t.Fatalf("get_events_since from 0 after the switch = %d events, %s; want a2's 2", len(since.Events), named(since.Log))
	}
}

// VTT-420 VTT-421
func TestARedialNamesTheHeldLogAndItsHighestSequence(t *testing.T) {
	fs := newFakeServer(t, nil)
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	conn := fs.firstConn(t)
	sendSwitch(t, conn, "a1")
	seedEvents(t, conn, a1Log...)
	sendSwitch(t, conn, "a2")
	seedEvents(t, conn, a1Log[:2]...)
	waitForLog(t, cs, "a2", 2)
	_ = conn.Close(websocket.StatusNormalClosure, "")

	waitForConns(t, fs, 2)
	fs.mu.Lock()
	log, after := fs.lastQuery["log"], fs.lastQuery.Get("after")
	fs.mu.Unlock()
	if len(log) != 1 || log[0] != "a2" || after != "2" {
		t.Fatalf("the redial named log=%q after=%q, want a2 after 2", log, after)
	}
}

// VTT-419
func TestARedialToldTheHeldLogKeepsWhatWasHeld(t *testing.T) {
	fs := newFakeServer(t, nil)
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	conn := fs.firstConn(t)
	sendSwitch(t, conn, "a1")
	seedEvents(t, conn, a1Log...)
	waitForLog(t, cs, "a1", 3)
	_ = conn.Close(websocket.StatusNormalClosure, "")

	redial := waitForConns(t, fs, 2)
	sendSwitch(t, redial, "a1")
	seedEvents(t, redial, canned(4, "", &vttv1.TokenPlaced{
		TokenId: "tok-1", SceneId: "scn-1", ActorId: "act-1", Position: &vttv1.GridPosition{X: 2, Y: 2}}))
	st := waitForLog(t, cs, "a1", 4)
	if _, placed := st.Tokens["tok-1"]; !placed || st.Scenes["scn-1"] == nil || st.Actors["act-1"] == nil {
		t.Fatalf("get_state after a redial told a1 folds scenes %s, actors %s, tokens %v; want a1's entries 1 to 4", st.Scenes, st.Actors, st.Tokens)
	}
	if since := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0}); len(since.Events) != 4 {
		t.Fatalf("get_events_since from 0 after a redial told a1 = %d events, want a1's 4", len(since.Events))
	}
}

// VTT-422
func TestTheAgentsAnswersNameNoLog(t *testing.T) {
	fs := newFakeServer(t, nil)
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	seedEvents(t, fs.firstConn(t), basicChain()...)
	if st := waitForHeadSequence(t, cs, 5); st.Log != nil {
		t.Errorf("get_state on a world connection carries %s, want no log key", named(st.Log))
	}
	if since := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0}); since.Log != nil {
		t.Errorf("get_events_since on a world connection carries %s, want no log key", named(since.Log))
	}
}

// VTT-422
func TestAPlayerOnNobodyIsAnsweredUnderAnEmptyLog(t *testing.T) {
	fs := newFakeServer(t, nil)
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	sendSwitch(t, fs.firstConn(t), "")
	waitForLog(t, cs, "", 0)
	if since := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0}); string(since.Log) != `""` {
		t.Fatalf("get_events_since on nobody carries %s, want an empty log", named(since.Log))
	}
}
