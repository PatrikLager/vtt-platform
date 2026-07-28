package mcp

// redial_internal_test.go covers the reconnect loop's retry path — the part
// of redial that keeps a seated agent connected across a dropped wire.
//
// The TOCTOU guard is already pinned by
// TestRedialClosesClientWhenContextCanceledBetweenDialSuccessAndInstall in
// server_internal_test.go. What was dark: the entry guard, the failure/
// backoff/retry cycle, cancellation DURING backoff, and — the one that
// actually carries semantics — that a retry resumes from lastSeen() rather
// than replaying the log from zero. A redial that silently passed after=0
// would re-deliver the entire campaign to the agent on every reconnect, and
// nothing in the suite would have noticed.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// fakeWSServer starts a minimal WebSocket endpoint that accepts and holds
// connections open, so harness.Dial can succeed against it.
//
// It must DRAIN READS, not just park on the request context. harness.Client's
// Close sends a close frame and then waits on readerDone (client.go:236-237),
// which needs the peer to complete the closing handshake. A handler blocked on
// <-r.Context().Done() never reads, never sees the close frame, and never
// replies — so every Close waited out coder/websocket's default handshake
// timeout of five seconds.
//
// That was not a harness problem, it was this helper's. Three mcp tests cost
// 5.00s each for it, taking the suite to 16.6s, which in turn made gremlins
// time out 57 of 64 mutants and rendered the package unmeasurable. Reading in
// a loop lets the handshake complete in microseconds.
func fakeWSServer(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		// Read until the peer closes. Returning on error is what lets
		// coder/websocket answer the close handshake promptly.
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

// stubDial replaces harnessDial for the duration of the test and restores it
// afterwards.
func stubDial(t *testing.T, fn func(context.Context, string, string, int64) (*harness.Client, error)) {
	t.Helper()
	real := harnessDial
	t.Cleanup(func() { harnessDial = real })
	harnessDial = fn
}

func TestRedialReturnsFalseWithoutDialingWhenContextAlreadyCanceled(t *testing.T) {
	var calls int
	stubDial(t, func(context.Context, string, string, int64) (*harness.Client, error) {
		calls++
		return nil, errors.New("should not be reached")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &Server{cfg: Config{WSURL: "ws://unused/ws", Token: "tok"}}
	if s.redial(ctx) {
		t.Error("redial: want false when ctx is already canceled")
	}
	if calls != 0 {
		t.Errorf("redial: dialed %d times with an already-canceled ctx, want 0", calls)
	}
}

// TestRedialRetriesAndResumesFromLastSeen is the load-bearing one: after a
// failed attempt, redial must keep trying, and every attempt must carry the
// highest sequence already seen so the server replays only what was missed.
func TestRedialRetriesAndResumesFromLastSeen(t *testing.T) {
	wsURL := fakeWSServer(t)

	var (
		mu     sync.Mutex
		afters []int64
	)
	real := harnessDial
	stubDial(t, func(dctx context.Context, u, tok string, after int64) (*harness.Client, error) {
		mu.Lock()
		afters = append(afters, after)
		n := len(afters)
		mu.Unlock()
		if n < 3 {
			return nil, errors.New("connection refused")
		}
		return real(dctx, u, tok, after)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Server{cfg: Config{WSURL: wsURL, Token: "tok"}}
	s.lastSeq = 41 // the agent has already seen up to sequence 41

	if !s.redial(ctx) {
		t.Fatal("redial: want true once a dial finally succeeds")
	}
	client := s.currentClient()
	if client == nil {
		t.Fatal("redial: want the successful client installed")
	}
	t.Cleanup(func() { client.Close() })

	mu.Lock()
	defer mu.Unlock()
	if len(afters) != 3 {
		t.Fatalf("redial: made %d attempts, want 3 (two failures then a success)", len(afters))
	}
	for i, after := range afters {
		if after != 41 {
			t.Errorf("redial attempt %d used after=%d, want 41 — a reconnect must resume from lastSeen, not replay from zero", i+1, after)
		}
	}
}

// TestRedialWakesImmediatelyWhenContextCanceledDuringBackoff pins the
// backoff select's ctx.Done() arm (server.go:304).
//
// The obvious assertion — "redial returns false" — is NOT sufficient, and an
// earlier version of this test made exactly that mistake: with the ctx.Done()
// arm deleted, redial still returns false, because the top-of-loop ctx.Err()
// guard catches it as soon as the backoff timer elapses. Both paths return
// false, so the return value cannot distinguish them and the test passed
// against the very code it claimed to cover.
//
// ELAPSED TIME is the only discriminator. Cancellation fires ~10ms in; with
// the arm present redial wakes then, and without it redial sleeps the full
// redialInitialBackoff (100ms) before the guard notices. Asserting the return
// lands well under one backoff period fails if the arm is removed.
func TestRedialWakesImmediatelyWhenContextCanceledDuringBackoff(t *testing.T) {
	const cancelAfter = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int
	stubDial(t, func(context.Context, string, string, int64) (*harness.Client, error) {
		calls++
		if calls == 1 {
			go func() {
				time.Sleep(cancelAfter)
				cancel()
			}()
		}
		return nil, errors.New("connection refused")
	})

	s := &Server{cfg: Config{WSURL: "ws://unused/ws", Token: "tok"}}

	done := make(chan bool, 1)
	start := time.Now()
	go func() { done <- s.redial(ctx) }()

	var ok bool
	select {
	case ok = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("redial did not return after its ctx was canceled during backoff")
	}
	elapsed := time.Since(start)

	if ok {
		t.Error("redial: want false when ctx is canceled during backoff")
	}
	// Midpoint between "woke on cancellation" (~10ms) and "slept the whole
	// backoff, then hit the entry guard" (~100ms).
	if limit := redialInitialBackoff / 2; elapsed >= limit {
		t.Errorf("redial took %v to notice cancellation (limit %v, one backoff is %v) — "+
			"it slept through the backoff instead of waking on ctx.Done(), so the "+
			"select's ctx.Done() arm is not doing the work",
			elapsed, limit, redialInitialBackoff)
	}
	if elapsed < cancelAfter {
		t.Errorf("redial returned in %v, before cancellation at %v — the entry guard "+
			"caught an already-canceled ctx and this test is not exercising the "+
			"backoff path at all", elapsed, cancelAfter)
	}
	if got := s.currentClient(); got != nil {
		t.Errorf("redial: want no client installed, got %v", got)
	}
}
