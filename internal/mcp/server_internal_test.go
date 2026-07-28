package mcp

// server_internal_test.go is a white-box test file (package mcp, not
// mcp_test): it reaches Server's unexported fields directly and reassigns
// the unexported harnessDial package var (server.go), neither of which the
// external mcp_test package can do. Existing precedent for this split:
// internal/gateway/server_internal_test.go, internal/campaign/
// poison_internal_test.go.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// TestRedialClosesClientWhenContextCanceledBetweenDialSuccessAndInstall
// reproduces the TOCTOU race redial's post-Dial cancellation check exists
// to close (final review Fix 6c): harness.Dial's own doc comment says its
// returned Client's lifetime is independent of the ctx passed to it (bound
// only to the handshake) — so a Dial that succeeds in the same instant the
// caller's ctx is canceled would otherwise still get installed via
// setClient, even though nothing is left running that will ever Close it
// (Run's own shutdown defer already read s.currentClient() as nil before
// this goroutine reaches setClient). A real network race between "Dial's
// handshake completes" and "ctx gets canceled" cannot be timed reliably
// from a test, so this substitutes harnessDial with a wrapper that calls
// the real dial (against a real, if minimal, fake WS server) and cancels
// the SAME ctx redial passed in the instant it succeeds — forcing the
// exact interleaving deterministically.
func TestRedialClosesClientWhenContextCanceledBetweenDialSuccessAndInstall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		// Drain reads rather than parking on the request context. Parking
		// means this handler never sees the client's close frame and never
		// answers the closing handshake, so harness.Client.Close waits out
		// coder/websocket's 5s handshake timeout (it blocks on readerDone,
		// client.go:236-237) — this test cost exactly 5.00s for that reason.
		// Three such tests took the mcp suite to 16.6s, which made gremlins
		// time out 57 of 64 mutants and left the package unmeasurable.
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	realDial := harnessDial
	defer func() { harnessDial = realDial }()
	harnessDial = func(dctx context.Context, u, tok string, after int64) (*harness.Client, error) {
		c, err := realDial(dctx, u, tok, after)
		if err == nil {
			cancel() // the race: ctx dies the instant Dial succeeds, before redial can look at it
		}
		return c, err
	}

	s := &Server{cfg: Config{WSURL: wsURL, Token: "tok"}}

	before := runtime.NumGoroutine()
	if ok := s.redial(ctx); ok {
		t.Fatal("redial: want false when ctx is already canceled by the time Dial returns, got true")
	}
	if got := s.currentClient(); got != nil {
		t.Fatalf("redial: want no client installed after a post-Dial cancellation, got %v", got)
	}

	// The regression this proves: before the fix, the dangling client's
	// reader goroutine (harness.Client.readLoop, started unconditionally
	// by Dial) runs forever with nothing left to ever Close it. Give any
	// leaked goroutine a moment to either unwind (fixed: Close() was
	// called) or stabilize (unfixed: it never will), then assert the
	// count settled back near baseline.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && runtime.NumGoroutine() > before+2 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before+2 {
		t.Fatalf("redial: goroutine count = %d (baseline %d) after a post-Dial cancellation — the leaked client's reader goroutine was never closed", got, before)
	}
}
