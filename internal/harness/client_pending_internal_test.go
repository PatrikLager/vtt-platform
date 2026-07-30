package harness

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestDropPendingRemovesTheEntry pins `if c.pending != nil` in dropPending
// (client.go).
//
// Inverted, dropPending becomes a no-op (delete on a nil map already is one),
// so an abandoned request's entry stays in the map forever when no reply ever
// arrives for it. No black-box test can see that: the pending channel is
// buffered(1), so a late reply to a stale entry neither blocks the read loop
// nor surfaces anywhere — it is written into a buffer nobody reads and
// discarded. The only observable is the map itself, which is why this is an
// internal test rather than another scenario.
//
// It also explains why this mutant appeared as a survivor in some runs and as
// "not covered" in others: dropPending only executes when a write fails or a
// caller's context is cancelled, and the real-WebSocket tests reach that
// path on timing. A deterministic unit test removes it from that lottery.
func TestDropPendingRemovesTheEntry(t *testing.T) {
	c := &Client{pending: map[string]chan *vttv1.CommandResult{
		"req-1": make(chan *vttv1.CommandResult, 1),
		"req-2": make(chan *vttv1.CommandResult, 1),
	}}

	c.dropPending("req-1")

	if _, still := c.pending["req-1"]; still {
		t.Error("dropPending left the abandoned request registered; with no reply ever coming " +
			"for it, that entry is retained for the life of the client")
	}
	if _, ok := c.pending["req-2"]; !ok {
		t.Error("dropPending removed an unrelated in-flight request")
	}
}

// TestDropPendingOnANilMapIsSafe covers the guard's stated purpose: dropPending
// runs after teardown has cleared the map.
func TestDropPendingOnANilMapIsSafe(t *testing.T) {
	c := &Client{}
	c.dropPending("req-1") // must not panic
}
