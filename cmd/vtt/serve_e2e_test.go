package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestServeComposeEndToEnd drives composeServer's full real lifecycle on a
// temp campaign + a random OS-assigned port: healthz 200, an invite minted
// directly via identity (not through the running server), a WebSocket
// connect authenticated with that invite's token, a StartSession command
// round-tripped for a CommandResult, then a graceful Shutdown and a clean
// exit from the goroutine running Serve.
func TestServeComposeEndToEnd(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	t.Cleanup(func() {
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	base := "http://" + ln.Addr().String()

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- srv.Serve(ln) }()

	// --- healthz 200 ---

	if err := waitForHealthz(base, 3*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}

	// --- mint an invite via identity directly, not through the server ---

	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	token, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if err := ids.Close(); err != nil {
		t.Fatalf("identity Close: %v", err)
	}

	// --- WS connect authenticated with that token ---

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws://" + ln.Addr().String() + "/ws?token=" + token + "&after=0"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.CloseNow()

	// --- StartSession command round trip ---

	cmd := &vttv1.ClientCommand{
		RequestId: "e2e-start-session",
		Command: &vttv1.ClientCommand_StartSession{
			StartSession: &vttv1.StartSession{Name: "e2e session"},
		},
	}
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	writeCtx, writeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer writeCancel()
	if err := conn.Write(writeCtx, websocket.MessageText, raw); err != nil {
		t.Fatalf("ws write: %v", err)
	}

	result, err := readCommandResult(conn, 3*time.Second)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if !result.Ok {
		t.Fatalf("StartSession result.Ok = false, error = %q", result.Error)
	}
	if result.RequestId != "e2e-start-session" {
		t.Fatalf("result.RequestId = %q, want e2e-start-session", result.RequestId)
	}
	if result.Sequence != 1 {
		t.Fatalf("result.Sequence = %d, want 1 (first event on a fresh campaign)", result.Sequence)
	}

	conn.CloseNow()

	// --- graceful Shutdown ---

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// --- clean exit from the Serve goroutine ---

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve goroutine did not exit after Shutdown")
	}
}

// waitForHealthz polls /healthz until it returns 200 or the deadline
// elapses — the server starts serving in a goroutine, so a fixed sleep
// would be a race; this loop is not.
func waitForHealthz(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = errors.New("healthz status " + resp.Status)
		} else {
			lastErr = err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return lastErr
}

// readCommandResult reads frames until a CommandResult arrives (skipping
// any Envelope broadcast frames that race ahead of it), mirroring
// internal/gateway's readResult test helper.
func readCommandResult(conn *websocket.Conn, timeout time.Duration) (*vttv1.CommandResult, error) {
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_, raw, err := conn.Read(ctx)
		cancel()
		if err != nil {
			return nil, err
		}
		var frame vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &frame); err != nil {
			return nil, err
		}
		if r := frame.GetResult(); r != nil {
			return r, nil
		}
	}
	return nil, errors.New("no CommandResult within 10 frames")
}
