package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// tcpBufSize is the SO_SNDBUF/SO_RCVBUF this test pins on both ends of the
// victim's socket. Explicitly setting either option disables the kernel's
// buffer autotuning for that socket (Linux and Darwin both do this) — left
// alone, loopback autotuning comfortably absorbs many thousands of small,
// un-read writes before a conn.Write ever actually blocks (verified
// empirically while writing this test), which is far too slow/flaky a
// precondition to build a deterministic regression test on. Pinning both
// ends small, AND sending oversized broadcast payloads (see bigSceneName
// below) so a handful of them exceed tcpBufSize outright, makes a stalled
// real peer force genuine TCP backpressure within a few writes instead.
const tcpBufSize = 8192

// sndbufListener wraps a net.Listener and pins SO_SNDBUF on every accepted
// TCP connection to tcpBufSize.
type sndbufListener struct {
	net.Listener
}

func (l *sndbufListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		if raw, err := tc.SyscallConn(); err == nil {
			raw.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, tcpBufSize)
			})
		}
	}
	return c, nil
}

// tinyRecvBufClient returns an http.Client whose dialed connections have
// SO_RCVBUF pinned to tcpBufSize — the client-side half of the same
// backpressure setup (see tcpBufSize's doc comment).
func tinyRecvBufClient() *http.Client {
	dialer := &net.Dialer{
		Control: func(_, _ string, c syscall.RawConn) error {
			var setErr error
			err := c.Control(func(fd uintptr) {
				setErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, tcpBufSize)
			})
			if err != nil {
				return err
			}
			return setErr
		},
	}
	return &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}
}

// bigSceneName is deliberately much larger than tcpBufSize (28KB vs 8KB):
// a broadcast Envelope carrying it cannot possibly be absorbed by the
// pinned send/recv buffers in one shot, so a peer that isn't reading forces
// the writer to genuinely stall after only a few such events — not merely
// slow down. Kept comfortably under coder/websocket's default 32KB
// per-message READ limit (which is what the server itself enforces on
// INCOMING ClientCommand frames, including this Name flowing straight
// through as a command field — CreateScene's Name is copied 1:1 into the
// broadcast SceneCreated) so the command carrying it is still accepted.
var bigSceneName = strings.Repeat("x", 28*1024)

// TestOverflowForcesSocketClosedAndOthersKeepServing is the Fix 2
// regression test: a connection whose store-level subscribe channel
// overflows (because its client stopped reading) must have its socket
// force-closed by the server itself — not sit there silently zombied — and
// every other connection must be completely unaffected.
func TestOverflowForcesSocketClosedAndOthersKeepServing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ids, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()

	driverToken, _, err := ids.CreateInvite("Driver", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	victimToken, _, err := ids.CreateInvite("Victim", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}
	healthyToken, _, err := ids.CreateInvite("Healthy", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(c, ids)
	srv.buffer = 2 // tiny: a stalled reader overflows after just a couple of missed live events

	httpSrv := httptest.NewUnstartedServer(srv.Handler())
	httpSrv.Listener = &sndbufListener{Listener: httpSrv.Listener}
	httpSrv.Start()
	defer httpSrv.Close()

	dial := func(token string, after int64, client *http.Client) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		u := httpSrv.URL + "/ws?token=" + token + "&after=" + strconv.FormatInt(after, 10)
		var opts *websocket.DialOptions
		if client != nil {
			opts = &websocket.DialOptions{HTTPClient: client}
		}
		conn, _, err := websocket.Dial(ctx, u, opts)
		if err != nil {
			t.Fatalf("dial %s: %v", token, err)
		}
		return conn
	}

	// Every connection that might legitimately RECEIVE bigSceneName-sized
	// broadcasts needs its read limit raised above coder/websocket's
	// default 32KB cap (driverConn subscribes to its own broadcasts too).
	const readLimit = 200 * 1024

	driverConn := dial(driverToken, 0, nil)
	defer driverConn.CloseNow()
	driverConn.SetReadLimit(readLimit)
	healthyConn := dial(healthyToken, 0, nil)
	defer healthyConn.CloseNow()
	healthyConn.SetReadLimit(readLimit)
	victimConn := dial(victimToken, 0, tinyRecvBufClient())
	defer victimConn.CloseNow()
	victimConn.SetReadLimit(readLimit)

	// healthyConn and driverConn both actively drain everything in the
	// background for the whole test, so NEITHER ever overflows itself
	// (driver receives its own CommandResults and its own broadcast echoes
	// too) regardless of how fast the burst below fires — draining
	// concurrently, rather than synchronously between writes, is what lets
	// the burst below fire at the server's own processing speed instead of
	// being paced by a request/reply round trip on driverConn.
	driverAlive := make(chan struct{})
	go func() {
		defer close(driverAlive)
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, _, err := driverConn.Read(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}()
	healthyAlive := make(chan struct{})
	go func() {
		defer close(healthyAlive)
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, _, err := healthyConn.Read(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}()

	// victimConn: deliberately never read here until after the drive loop
	// below — it is the stalled subscriber this test overflows.

	// Burst oversized broadcast events (see bigSceneName) — no interleaved
	// read wait on driverConn (that's the background goroutine's job
	// above) — so the victim (buffer=2, SO_RCVBUF pinned tiny, never
	// reading) overflows within a handful of commands. A small pacing
	// sleep between writes keeps this a burst (still far faster than
	// victim's writer, which can genuinely stall indefinitely once stuck —
	// verified empirically) while giving the background reader goroutines
	// above, and the server's own SQLite-backed Append pipeline, regular
	// scheduling opportunities instead of a single CPU-monopolizing tight
	// loop — without it, driverConn/healthyConn's readers were observed to
	// starve too, and unrelated concurrent identity DB queries could hit
	// transient SQLite lock contention.
	const commandCount = 40
	for i := 0; i < commandCount; i++ {
		cmd := &vttv1.ClientCommand{
			RequestId: strconv.Itoa(i),
			Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
				SceneId: "scn-" + strconv.Itoa(i), Name: bigSceneName, GridWidth: 1, GridHeight: 1,
			}},
		}
		raw, err := protojson.Marshal(cmd)
		if err != nil {
			t.Fatal(err)
		}
		wctx, wcancel := context.WithTimeout(context.Background(), 3*time.Second)
		werr := driverConn.Write(wctx, websocket.MessageText, raw)
		wcancel()
		if werr != nil {
			t.Fatalf("driver write %d: %v", i, werr)
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Let the server's backlog (SQLite-backed Append + fan-out for all 40
	// oversized events) fully settle before the assertions below query the
	// identity DB again (a fresh dial) or expect prompt delivery on
	// healthyConn/driverConn — avoids transient contention flakiness on the
	// separate identity.DB handle sharing the same campaign file.
	time.Sleep(200 * time.Millisecond)

	// NOW check the victim: these are its first-ever Read calls. Even
	// though the server force-closed the connection server-side partway
	// through the drive loop above (the overflow this test exercises),
	// victimConn's own kernel receive buffer may still hold a small number
	// of complete messages that made it through before the close — so
	// drain up to a generous bound and require the closure to surface
	// within it, rather than asserting the very first read errors.
	closed := false
	for i := 0; i < commandCount+5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _, err := victimConn.Read(ctx)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("victim connection not closed — read timed out instead of observing close")
			}
			closed = true
			break
		}
	}
	if !closed {
		t.Fatalf("want victim connection force-closed after %d un-read oversized broadcasts, never observed a closed read", commandCount)
	}

	// The force-close must be observable on a SECOND read too (not just
	// the one that happened to race the close), proving the connection is
	// really closed, not just transiently erroring.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	if _, _, err := victimConn.Read(ctx2); err == nil {
		t.Fatal("want victim connection to stay closed on a subsequent read")
	}

	// The healthy connection (and driver's own connection) must be
	// completely unaffected: both are still alive and still actively
	// draining broadcasts (neither background reader goroutine has
	// exited), despite the exact same burst that overflowed the victim.
	select {
	case <-healthyAlive:
		t.Fatal("want healthy connection's reader still running, but it exited")
	default:
	}
	select {
	case <-driverAlive:
		t.Fatal("want driver connection's reader still running, but it exited")
	default:
	}

	// Final proof the server itself is still fine: a brand new connection
	// can still connect and issue a successful command. driverToken (DM
	// role) + CreateScene with a fresh id is used: always succeeds
	// regardless of prior state (no open-session precondition to trip over,
	// unlike EndSession). Dialed with a huge `after` cursor to skip
	// catch-up entirely (this connection only cares about ITS OWN
	// CommandResult, not replaying the whole burst).
	freshConn := dial(driverToken, 1<<30, nil)
	defer freshConn.CloseNow()
	cmd := &vttv1.ClientCommand{RequestId: "fresh", Command: &vttv1.ClientCommand_CreateScene{
		CreateScene: &vttv1.CreateScene{SceneId: "scn-fresh", Name: "s", GridWidth: 1, GridHeight: 1},
	}}
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	wctx, wcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer wcancel()
	if err := freshConn.Write(wctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	rctx, rcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer rcancel()
	var frame vttv1.ServerFrame
	for tries := 0; tries < 2 && frame.GetResult() == nil; tries++ {
		_, respRaw, err := freshConn.Read(rctx)
		if err != nil {
			t.Fatalf("fresh connection read (try %d): %v", tries, err)
		}
		frame = vttv1.ServerFrame{}
		if err := protojson.Unmarshal(respRaw, &frame); err != nil {
			t.Fatalf("unmarshal fresh frame (try %d): %v", tries, err)
		}
	}
	if frame.GetResult() == nil || !frame.GetResult().Ok {
		t.Fatalf("want ok=true CommandResult on a fresh connection after the overflow, got %v", &frame)
	}

	// Stop the background readers before the test (and its deferred
	// CloseNow calls) tears everything down.
	healthyConn.CloseNow()
	<-healthyAlive
	driverConn.CloseNow()
	<-driverAlive
}
