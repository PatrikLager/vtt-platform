package harness_test

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

func sendSwitch(t *testing.T, conn *websocket.Conn, actorID string) {
	t.Helper()
	sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_ViewpointChanged{
		ViewpointChanged: &vttv1.ViewpointChanged{ActorId: actorID},
	}})
}

// VTT-409
func TestAClientHandsItsReaderEachSwitchInOrderWithTheEntries(t *testing.T) {
	fs := newFakeServer(t, nil)
	fs.onConnect = func(conn *websocket.Conn) {
		sendSwitch(t, conn, "a1")
		sendEvent(t, conn, "", 1)
		sendEvent(t, conn, "", 2)
		sendSwitch(t, conn, "a2")
		sendEvent(t, conn, "", 1)
		sendSwitch(t, conn, "")
	}
	c := dial(t, fs, "tok", 0)

	type want struct {
		switchTo string
		seq      int64
	}
	wants := []want{{"a1", 0}, {"", 1}, {"", 2}, {"a2", 0}, {"", 1}, {"", 0}}
	for i, w := range wants {
		var d harness.Delivery
		select {
		case got, ok := <-c.Events():
			if !ok {
				t.Fatalf("delivery %d: stream ended: %v", i, c.CloseErr())
			}
			d = got
		case <-time.After(3 * time.Second):
			t.Fatalf("delivery %d: nothing arrived", i)
		}
		isSwitch := i == 0 || i == 3 || i == 5
		switch {
		case isSwitch && (d.Viewpoint == nil || d.Envelope != nil):
			t.Fatalf("delivery %d = %+v, want only a switch to %q", i, d, w.switchTo)
		case isSwitch && d.Viewpoint.GetActorId() != w.switchTo:
			t.Fatalf("delivery %d switches to %q, want %q", i, d.Viewpoint.GetActorId(), w.switchTo)
		case !isSwitch && (d.Envelope == nil || d.Viewpoint != nil):
			t.Fatalf("delivery %d = %+v, want only the entry of sequence %d", i, d, w.seq)
		case !isSwitch && d.Envelope.GetSequence() != w.seq:
			t.Fatalf("delivery %d is sequence %d, want %d", i, d.Envelope.GetSequence(), w.seq)
		}
	}
}

// VTT-410
func TestADialNamesTheLogItIsGivenAndNoneWhenItIsGivenNone(t *testing.T) {
	fs := newFakeServer(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := harness.Dial(ctx, fs.wsURL(), "tok", 3, "a1")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	fs.mu.Lock()
	logs, after := fs.lastQuery["log"], fs.lastQuery.Get("after")
	fs.mu.Unlock()
	c.Close()
	if len(logs) != 1 || logs[0] != "a1" || after != "3" {
		t.Fatalf("a dial naming a1 after 3 sent log=%q after=%q", logs, after)
	}

	c, err = harness.Dial(ctx, fs.wsURL(), "tok", 3, "")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	fs.mu.Lock()
	logs, present := fs.lastQuery["log"]
	fs.mu.Unlock()
	if present {
		t.Fatalf("a dial naming no log sent log=%q", logs)
	}
}
