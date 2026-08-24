package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "", "")
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
	token, _, err := ids.CreateInvite("DM", identity.RoleDM)
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

// TestServeSubprocessExitsCleanlyOnSIGTERM proves `vtt serve` — unlike
// `vtt events tail` (see events_tail.go's own SIGINT subprocess test in
// client_e2e_test.go) — actually watches the cancelable context main.go
// wires SIGINT/SIGTERM into, rather than blocking forever in
// srv.ListenAndServe() with nothing ever observing cmd.Context().Done().
// It builds the real binary, runs `vtt serve` as an OS subprocess against a
// fresh temp campaign, waits for it to be listening (healthz 200), sends
// SIGTERM, and asserts it exits promptly (bounded wait) with code 0 — a
// graceful, Shutdown-driven stop, not a hang that only SIGKILL (the
// subprocess teardown every OTHER `vtt serve` subprocess test in this
// package uses today, e.g. library_test.go's
// TestThreeRoleExitScenarioOverLiveServeSubprocess) can end. Before the
// fix, serve's RunE has no path that ever reads cmd.Context() at all, so
// the signal is silently swallowed and waitWithTimeout's bounded wait below
// times out with the subprocess still alive — see this test's own report
// entry for the captured RED transcript.
func TestServeSubprocessExitsCleanlyOnSIGTERM(t *testing.T) {
	binPath := buildVTTBinary(t)

	dir := t.TempDir()
	campaignPath := filepath.Join(dir, "campaign.db")
	addr := mustFreeAddr(t)

	cmd := exec.Command(binPath, "serve", "--campaign", campaignPath, "--addr", addr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start vtt serve subprocess: %v", err)
	}
	// Safety-net teardown in case the test fails/fatals before the process
	// has already exited on its own — a Kill on an already-dead process is a
	// harmless no-op error (same pattern as library_test.go's subprocess
	// test).
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	base := "http://" + addr
	if err := waitForHealthz(base, 5*time.Second); err != nil {
		t.Fatalf("vtt serve subprocess healthz never became ready: %v", err)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	// Bounded well past serve.go's own 5s Shutdown timeout: a correct
	// implementation exits close to immediately (nothing is holding
	// Shutdown up in this test — no live WS connection), so this margin is
	// purely to distinguish "slow but working" from "swallowed entirely"
	// without flaking on the former.
	if err := waitWithTimeout(cmd, 7*time.Second); err != nil {
		t.Fatalf("subprocess did not exit cleanly after SIGTERM: %v", err)
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("subprocess exit code after SIGTERM = %d, want 0", code)
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

// TestComposeServerFailsLoudlyOnAnUnreadableAdventureGuide covers the boot
// path added with the metadata endpoints (T3).
//
// Guides are read at BOOT rather than per request, deliberately: cmd/vtt owns
// the filesystem (ADR-008), and reading one lazily would turn a missing file
// into a 500 in the middle of a session instead of a refusal to start. This
// pins that posture — adventure.Load only RECORDS guide.md's path and never
// opens it, so without this step a missing guide would go unnoticed until a
// DM asked for it mid-game.
func TestComposeServerFailsLoudlyOnAnUnreadableAdventureGuide(t *testing.T) {
	advRoot := t.TempDir()
	dst := filepath.Join(advRoot, "goblin-ambush")
	if err := os.CopyFS(dst, os.DirFS(filepath.Join("..", "..", "adventures", "goblin-ambush"))); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dst, "guide.md")); err != nil {
		t.Fatal(err)
	}

	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	rulesetDir := filepath.Join("..", "..", "rulesets", "dnd45e-minimal")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir, advRoot, "")
	if err == nil {
		if closeFn != nil {
			_ = closeFn()
		}
		_ = srv
		t.Fatal("composeServer succeeded with a missing adventure guide; boot must fail loudly " +
			"rather than serving an adventure whose guide cannot be read")
	}
	if !strings.Contains(err.Error(), "guide") {
		t.Errorf("error should name the guide as the cause, got: %v", err)
	}
}

// TestComposeServerLoadsAdventureGuidesAtBoot is the happy path's other half:
// a well-formed adventures dir composes, which is what makes the failure
// above attributable to the missing guide rather than to the fixture.
func TestComposeServerLoadsAdventureGuidesAtBoot(t *testing.T) {
	// A dir holding ONLY goblin-ambush. The committed adventures/ directory
	// cannot be served wholesale: cellar-rats declares the tavern-brawl
	// ruleset and goblin-ambush declares dnd45e-minimal, and a server loads
	// exactly one ruleset, so pointing --adventures at the whole directory is
	// a boot error by design rather than a fixture quirk.
	advRoot := t.TempDir()
	if err := os.CopyFS(filepath.Join(advRoot, "goblin-ambush"),
		os.DirFS(filepath.Join("..", "..", "adventures", "goblin-ambush"))); err != nil {
		t.Fatal(err)
	}

	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	srv, closeFn, err := composeServer(
		campaignPath, "127.0.0.1:0",
		filepath.Join("..", "..", "rulesets", "dnd45e-minimal"),
		advRoot, "",
	)
	if err != nil {
		t.Fatalf("composeServer with the committed adventures dir: %v", err)
	}
	defer func() { _ = closeFn() }()
	if srv == nil {
		t.Fatal("composeServer returned a nil server")
	}
}
