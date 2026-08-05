package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// The end-to-end boundary for `vtt state dump`'s refusal message: an operator
// reads the sentence this command prints, and until 2026-08-06 that sentence
// named the 30s catch-up backstop for EVERY short read — including reads that
// ended in milliseconds because the stream closed under it.
//
// drainToHead's unit tests pin the reason it returns; nothing pinned that the
// command actually says it. Reverting the caller to dumpCatchUpTimeout broke
// no test, and cmd/vtt is deliberately not mutation-gated
// (tools/mutation-scope.md), so no gate would have caught the revert either.
//
// A whole gateway is not needed for this: the wire facts that matter are the
// opening CatchUpHead frame and a close before the head arrives.

// truncatingGateway announces head and then closes after sending one envelope,
// so the dump can never reach the head it was promised.
func truncatingGateway(t *testing.T, head int64, reason string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		// Derived from the request, not a fresh Background: a client that
		// disconnects mid-write should unblock this handler, and deriving is
		// what contextcheck is there to insist on.
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		write := func(ctx context.Context, f *vttv1.ServerFrame) bool {
			raw, err := protojson.Marshal(f)
			if err != nil {
				t.Errorf("truncatingGateway: marshal: %v", err)
				return false
			}
			return conn.Write(ctx, websocket.MessageText, raw) == nil
		}

		if !write(ctx, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_CatchUpHead{
			CatchUpHead: &vttv1.CatchUpHead{HeadSequence: head},
		}}) {
			return
		}
		if !write(ctx, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{
			Event: &vttv1.Envelope{Sequence: 1},
		}}) {
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, reason)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func TestStateDumpBlamesTheStreamCloseNotTheClock(t *testing.T) {
	url := truncatingGateway(t, 5, "gateway: shutting down")

	start := time.Now()
	out, err := runCLI(t, "state", "dump", "--server", url, "--token", "t")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("want a refusal: the head was never reached. stdout=%q", out)
	}
	msg := err.Error()

	// The whole point. The command returned in milliseconds; naming a 30s
	// timeout sends the next reader after gateway latency that never happened.
	if strings.Contains(msg, dumpCatchUpTimeout.String()) {
		t.Fatalf("after %s the message still blames the %s backstop: %q", elapsed, dumpCatchUpTimeout, msg)
	}
	if !strings.Contains(msg, "the event stream closed") {
		t.Fatalf("want the stream close named as the cause, got %q", msg)
	}
	// The refusal itself must survive: a truncated state is never printed.
	if !strings.Contains(msg, "refusing to print a truncated state") {
		t.Fatalf("want the refusal preserved, got %q", msg)
	}
	if strings.Contains(out, "headSequence") {
		t.Fatalf("a truncated dump must never reach stdout, got %q", out)
	}
}
