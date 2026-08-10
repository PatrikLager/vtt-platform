package gateway

import (
	"context"
	"errors"
	"fmt"
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

// TestAWedgedConnectionIsTornDownAndOthersKeepServing succeeds
// TestOverflowForcesSocketClosedAndOthersKeepServing, which asserted the same
// property through a trigger that was itself the bug.
//
// That test relied on the store dropping a subscriber whose channel filled.
// Filling it never required the client to be STALLED — notifyLocked pushed a
// burst under the store lock faster than any pump could drain, so the victim
// was severed by scheduling. Measured after the fix, that same victim
// completes its 28KB writes in ~100ms and absorbs every one: slow, alive, and
// now correctly kept. The scenario could no longer produce a wedged client at
// all.
//
// So the property is split. The store's half — a subscriber that stops
// consuming is cut loose once the budget elapses — is pinned deterministically
// on a fake clock in internal/store/backpressure_test.go. THIS test keeps the
// gateway's half: a connection the server can no longer write to is torn down
// rather than left looking alive, and its peers are untouched.
//
// The victim gets its OWN Server so the aggressive budget applies to it alone.
// That is what makes this deterministic rather than a race: healthy peers are
// unaffected BY CONSTRUCTION, not by winning a timing margin against the
// victim. Both servers share one campaign, so the broadcast path is real.
func TestAWedgedConnectionIsTornDownAndOthersKeepServing(t *testing.T) {
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

	// The healthy side. noProgress is set EXPLICITLY and generously rather
	// than left at the 30s default: this test is documented below as having
	// taken 17.4s on a machine under disk pressure, and its healthy readers
	// are background goroutines. A starved reader tripping a 30s budget would
	// drop a healthy subscriber and report a logic failure that is not there.
	srv := New(c, ids)
	srv.noProgress = 10 * time.Minute
	httpSrv := httptest.NewUnstartedServer(srv.Handler())
	httpSrv.Listener = &sndbufListener{Listener: httpSrv.Listener}
	httpSrv.Start()
	defer httpSrv.Close()

	// The victim's side, on the same campaign. buffer 0 removes every slot the
	// writer could hide behind, and the budget is two orders of magnitude
	// below a stall this test GUARANTEES structurally: bigSceneName (28KB)
	// cannot fit the pinned 8KB socket buffers, so a peer that never reads
	// forces the write to park. The threshold is not standing in for the
	// condition — the condition is arranged, and 5ms only has to be shorter
	// than a park measured at ~100ms.
	victimSrv := New(c, ids)
	victimSrv.buffer = 0
	victimSrv.noProgress = 5 * time.Millisecond
	victimHTTP := httptest.NewUnstartedServer(victimSrv.Handler())
	victimHTTP.Listener = &sndbufListener{Listener: victimHTTP.Listener}
	victimHTTP.Start()
	defer victimHTTP.Close()

	dialTo := func(base, token string, after int64, client *http.Client) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		u := base + "/ws?token=" + token + "&after=" + strconv.FormatInt(after, 10)
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
	dial := func(token string, after int64, client *http.Client) *websocket.Conn {
		t.Helper()
		return dialTo(httpSrv.URL, token, after, client)
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
	victimConn := dialTo(victimHTTP.URL, victimToken, 0, tinyRecvBufClient())
	defer victimConn.CloseNow()
	victimConn.SetReadLimit(readLimit)

	// healthyConn and driverConn both actively drain everything in the
	// background for the whole test, so NEITHER ever stops making progress
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
	// below — it is the stalled subscriber this test wedges.

	// Burst oversized broadcast events (see bigSceneName) — no interleaved
	// read wait on driverConn (that's the background goroutine's job
	// above) — so the victim (buffer=2, SO_RCVBUF pinned tiny, never
	// reading) falls far behind within a handful of commands. A small pacing
	// sleep between writes keeps this a burst (still far faster than
	// victim's writer, which can genuinely stall indefinitely once stuck —
	// verified empirically) while giving the background reader goroutines
	// above, and the server's own SQLite-backed Append pipeline, regular
	// scheduling opportunities instead of a single CPU-monopolizing tight
	// loop — without it, driverConn/healthyConn's readers were observed to
	// starve too, and unrelated concurrent identity DB queries could hit
	// transient SQLite lock contention.
	const commandCount = 40

	// A BACKSTOP, not a measurement. These writes are the test's scaffolding —
	// driverConn is healthy and every one of them is EXPECTED to succeed; the
	// deadline exists only so a genuine wedge fails the suite instead of hanging
	// it forever. That makes it categorically different from the victim-read
	// deadlines below, where expiry is part of what is being asserted, and it is
	// why this one can be generous at no cost: a write that succeeds promptly
	// never touches it.
	//
	// It was 3s, and that failed DETERMINISTICALLY (5 of 5, always at write 14)
	// on a developer machine under disk pressure — these writes drain through
	// the server's SQLite-backed Append, so their latency tracks the disk, not
	// the code. Raising the bound made the same test pass in 17.4s. CI, on a
	// clean runner, never saw it. A scaffolding timeout tight enough to trip on
	// a slow disk reports a logic failure that is not there, and it blocked a
	// push for work in another language entirely.
	const driverWriteBackstop = 30 * time.Second
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
		wctx, wcancel := context.WithTimeout(context.Background(), driverWriteBackstop)
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
	// through the drive loop above (the no-progress drop this test exercises),
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
	// exited), despite the exact same burst that wedged the victim.
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
	// Budget, not a frame count: this connection is served a CatchUpHead
	// first and its CommandResult second, so 2 is the exact requirement and
	// would leave nothing for a third frame type added later. The loop's job
	// is to skip whatever is not a result; give it room to, so a future frame
	// shows up as a failure in the test that ADDED it rather than here, where
	// the message ("want ok=true CommandResult") would misdirect.
	const frameBudget = 5
	for tries := 0; tries < frameBudget && frame.GetResult() == nil; tries++ {
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
		t.Fatalf("want ok=true CommandResult on a fresh connection after the victim was reaped, got %v", &frame)
	}

	// Stop the background readers before the test (and its deferred
	// CloseNow calls) tears everything down.
	healthyConn.CloseNow()
	<-healthyAlive
	driverConn.CloseNow()
	<-driverAlive
}

// TestCatchUpHeadEncodeFailureClosesTheConnection pins that a connection which
// CANNOT announce its catch-up head is refused rather than served.
//
// The failure is unreachable in production — protojson does not fail on a
// single int64 — which is exactly why it needs injecting: the handling is
// otherwise a claim no test has ever checked. The stakes are what make it
// worth checking at all. A connection left open but permanently silent about
// its head is worse than a refused one, because harness.Client.CatchUpHead
// blocks on its context and `vtt state dump` passes main.go's signal context,
// which carries NO deadline. The dump would hang until Ctrl-C rather than
// failing — the one outcome worse than the truncated print this whole frame
// exists to prevent.
//
// Mirrors the subscribe-failure path directly above it in serve: close with
// StatusInternalError, do not serve.
func TestCatchUpHeadEncodeFailureClosesTheConnection(t *testing.T) {
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

	token, _, err := ids.CreateInvite("Watcher", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The seam is swapped on THIS Server, not on a package global. It used to
	// be global, and presence made that a data race: encodeFrame became
	// reachable from a connection's TEARDOWN, so this swap raced other tests'
	// connections unwinding after they had returned. Confirmed by review with
	// `go test -race -count=4 -cpu=1,4,8`; a per-Server field cannot reach
	// across.
	srv := New(c, ids)
	srv.encodeFrame = func(*vttv1.ServerFrame) ([]byte, error) {
		return nil, errors.New("gateway: injected encode failure")
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, httpSrv.URL+"/ws?token="+token+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// A bounded read is the whole assertion: the server must hang up on its
	// own. If it serves the connection instead, this blocks until the ctx
	// deadline and the failure is the hang itself.
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("want the connection closed when the catch-up head cannot be encoded, got a readable frame")
	}
	if status := websocket.CloseStatus(err); status != websocket.StatusInternalError {
		t.Fatalf("close status = %v, want StatusInternalError (err=%v)", status, err)
	}
}

// TestAClientThatStopsReadingEntirelyIsTornDown pins the only mechanism that
// can still tear down a wedged connection — and it is a mechanism the store
// change made load-bearing rather than redundant.
//
// The pump force-closes the socket when `events` closes. Under the store's
// no-progress policy a subscriber is dropped only after ZERO progress, which
// means this connection's writer is parked in conn.Write, which means the pump
// is parked handing off to outCh — so by construction the pump is NOT ranging
// `events` at the moment they close, and never observes it. The post-loop
// close is unreachable for exactly the case it was written for.
//
// Without conn.Write's deadline the result is a permanent leak: serve stuck in
// conn.Read, the writer in conn.Write, the pump on outCh, the client believing
// it is connected and receiving nothing, forever. shutdown() cannot rescue it
// either — it waits on pumpDone, which waits on the writer.
//
// Observed through onServeDone because the client must NOT read: reading is
// progress, and progress is the precondition being excluded. Reading to detect
// the close would destroy the state under test.
func TestAClientThatStopsReadingEntirelyIsTornDown(t *testing.T) {
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

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	deafToken, _, err := ids.CreateInvite("Deaf", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The DM drives on an ordinary server so its own writes are never subject
	// to the aggressive budget below.
	driverSrv := New(c, ids)
	driverHTTP := httptest.NewServer(driverSrv.Handler())
	defer driverHTTP.Close()

	served := make(chan struct{}, 4)
	deafSrv := New(c, ids)
	deafSrv.buffer = 0
	// The store budget stays LONG so it cannot drop this subscriber first —
	// that is what isolates the socket path. Only the write deadline is
	// aggressive, and it is the mechanism under test.
	deafSrv.noProgress = 30 * time.Second
	deafSrv.writeTimeout = 25 * time.Millisecond
	deafSrv.onServeDone = func() { served <- struct{}{} }
	deafHTTP := httptest.NewUnstartedServer(deafSrv.Handler())
	deafHTTP.Listener = &sndbufListener{Listener: deafHTTP.Listener}
	deafHTTP.Start()
	defer deafHTTP.Close()

	dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dcancel()
	driverConn, _, err := websocket.Dial(dctx, driverHTTP.URL+"/ws?token="+dmToken+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial driver: %v", err)
	}
	defer driverConn.CloseNow()
	driverConn.SetReadLimit(200 * 1024)
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, _, rerr := driverConn.Read(ctx)
			cancel()
			if rerr != nil {
				return
			}
		}
	}()

	deafConn, _, err := websocket.Dial(dctx, deafHTTP.URL+"/ws?token="+deafToken+"&after=0",
		&websocket.DialOptions{HTTPClient: tinyRecvBufClient()})
	if err != nil {
		t.Fatalf("dial deaf: %v", err)
	}
	defer deafConn.CloseNow()
	// Deliberately never read from deafConn. That is the whole scenario.

	// Enough oversized broadcasts that 28KB cannot fit the pinned 8KB socket
	// buffers — the writer is structurally forced to park, not merely slowed.
	for i := range 40 {
		cmd := &vttv1.ClientCommand{
			RequestId: strconv.Itoa(i),
			Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
				SceneId: "deaf-" + strconv.Itoa(i), Name: bigSceneName, GridWidth: 1, GridHeight: 1,
			}},
		}
		raw, merr := protojson.Marshal(cmd)
		if merr != nil {
			t.Fatal(merr)
		}
		wctx, wcancel := context.WithTimeout(context.Background(), 30*time.Second)
		werr := driverConn.Write(wctx, websocket.MessageText, raw)
		wcancel()
		if werr != nil {
			t.Fatalf("driver write %d: %v", i, werr)
		}
	}

	select {
	case <-served:
	case <-time.After(20 * time.Second):
		t.Fatal("a client that stopped reading entirely was never torn down — serve() is still " +
			"parked, leaking its goroutines and socket, with the client believing it is connected")
	}
}

// TestAForceClosedClientIsAnnouncedGone is the teardown path the plan warned
// would be forgotten, and it is the one that matters most to a table.
//
// A client that stops reading never says goodbye. It is torn down by the
// WRITER'S DEADLINE (TestAClientThatStopsReadingEntirelyIsTornDown above owns
// that mechanism), not by any close handshake — so a presence implementation
// that deregisters on a clean quit alone leaves that participant listed as
// present forever. A ghost at the table is worse than no presence at all,
// because it is indistinguishable from someone who is genuinely there.
//
// The watcher reads continuously and so keeps making progress, which is what
// keeps the aggressive write budget off its back.
func TestAForceClosedClientIsAnnouncedGone(t *testing.T) {
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

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	deafToken, deafID, err := ids.CreateInvite("Deaf", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}

	// TWO servers over one campaign, SHARING the presence registry.
	//
	// The watcher cannot live on the deaf client's server. That server needs
	// buffer=0 and a 25ms write budget to wedge the deaf client, and those are
	// server-wide — so the watcher's own outCh becomes unbuffered too, and
	// every presence send to it needs a rendezvous with a writer that is busy
	// pushing 28KB frames through an 8KB socket buffer. It passed 20/20 here
	// and still failed on CI at the full timeout, because "usually finds a
	// rendezvous" is a race, not a guarantee, and a slower machine loses it.
	//
	// Splitting the servers isolates the two postures: the deaf client keeps
	// the aggressive budgets that force the tear-down under test, and the
	// watcher gets ordinary ones so observing it is not itself a race. The
	// shared registry is what lets one see the other — presence is per-Server
	// state, so this assignment is the whole reason a two-server arrangement
	// works here at all.
	watchSrv := New(c, ids)
	watchHTTP := httptest.NewServer(watchSrv.Handler())
	defer watchHTTP.Close()

	srv := New(c, ids)
	srv.presence = watchSrv.presence
	srv.buffer = 0
	// Long store budget, short socket budget: isolates the SOCKET path, the
	// one that produces a client gone without a goodbye.
	srv.noProgress = 30 * time.Second
	srv.writeTimeout = 25 * time.Millisecond
	httpSrv := httptest.NewUnstartedServer(srv.Handler())
	httpSrv.Listener = &sndbufListener{Listener: httpSrv.Listener}
	httpSrv.Start()
	defer httpSrv.Close()

	dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dcancel()

	watcherConn, _, err := websocket.Dial(dctx, watchHTTP.URL+"/ws?token="+dmToken+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial watcher: %v", err)
	}
	defer watcherConn.CloseNow()
	watcherConn.SetReadLimit(200 * 1024)

	// The watcher reads continuously; presence frames for the deaf client are
	// forwarded out for the assertion below.
	gone := make(chan struct{})
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			_, raw, rerr := watcherConn.Read(ctx)
			cancel()
			if rerr != nil {
				return
			}
			var f vttv1.ServerFrame
			if protojson.Unmarshal(raw, &f) != nil {
				continue
			}
			pc := f.GetPresenceChanged()
			if pc != nil && pc.GetParticipantId() == deafID &&
				pc.GetState() == vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED {
				close(gone)
				return
			}
		}
	}()

	deafConn, _, err := websocket.Dial(dctx, httpSrv.URL+"/ws?token="+deafToken+"&after=0",
		&websocket.DialOptions{HTTPClient: tinyRecvBufClient()})
	if err != nil {
		t.Fatalf("dial deaf: %v", err)
	}
	defer deafConn.CloseNow()
	// Never read from deafConn. That is the scenario.

	// Force the writer to park: more bytes than the pinned socket buffers can
	// hold, so the deadline is reached structurally rather than by timing.
	for i := range 40 {
		cmd := &vttv1.ClientCommand{
			RequestId: strconv.Itoa(i),
			Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
				SceneId: "deaf-" + strconv.Itoa(i), Name: bigSceneName, GridWidth: 1, GridHeight: 1,
			}},
		}
		raw, merr := protojson.Marshal(cmd)
		if merr != nil {
			t.Fatal(merr)
		}
		wctx, wcancel := context.WithTimeout(context.Background(), 30*time.Second)
		werr := watcherConn.Write(wctx, websocket.MessageText, raw)
		wcancel()
		if werr != nil {
			t.Fatalf("driver write %d: %v", i, werr)
		}
	}

	select {
	case <-gone:
	case <-time.After(25 * time.Second):
		t.Fatal("a client force-closed by the write deadline was never announced gone — " +
			"it stays listed at the table forever, indistinguishable from someone present")
	}
}

// TestASecondDeviceIsNotASecondArrivalOrDeparture is the reference count doing
// its job end to end (spec §4): one person on two devices is ONE seat at the
// table, so the second connection announces nothing and closing it announces
// nothing.
//
// Asserting that NOTHING happens is the hard part, and the first version of
// this test got it wrong in a way worth recording. It closed the second
// device, then closed the first and waited for DISCONNECTED — which a
// PREMATURE disconnect satisfies just as well as the correct one. Injecting
// "announce DISCONNECTED on every close" fired no test at all.
//
// The fix is a MARKER, not a timeout. onServeDone tells us the phone's serve
// has fully returned, so any frame that teardown would emit is already queued
// ahead of what we send next; a command issued after that point therefore
// arrives strictly later. Reading up to the marker and finding no presence
// frame is then a real negative, not a race we happened to win.
func TestASecondDeviceIsNotASecondArrivalOrDeparture(t *testing.T) {
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

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	playerToken, playerID, err := ids.CreateInvite("Lera", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}

	served := make(chan struct{}, 8)
	srv := New(c, ids)
	srv.onServeDone = func() { served <- struct{}{} }
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	dial := func(token string) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _, derr := websocket.Dial(ctx, httpSrv.URL+"/ws?token="+token+"&after=0", nil)
		if derr != nil {
			t.Fatalf("dial: %v", derr)
		}
		conn.SetReadLimit(200 * 1024)
		return conn
	}

	watcher := dial(dmToken)
	defer watcher.CloseNow()

	laptop := dial(playerToken)
	defer laptop.CloseNow()

	// The laptop's arrival is LEGITIMATE and must be consumed before we can
	// assert silence — the participant's first connection is a real arrival.
	// Leaving it in the stream made the first version of this test fail on
	// the very frame it exists to allow.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, b, rerr := watcher.Read(ctx)
		cancel()
		if rerr != nil {
			t.Fatalf("waiting for the laptop's arrival: %v", rerr)
		}
		var f vttv1.ServerFrame
		if protojson.Unmarshal(b, &f) != nil {
			continue
		}
		if pc := f.GetPresenceChanged(); pc != nil && pc.GetParticipantId() == playerID {
			if pc.GetState() != vttv1.PresenceState_PRESENCE_STATE_CONNECTED {
				t.Fatalf("first connection = %v, want CONNECTED", pc.GetState())
			}
			break
		}
	}

	phone := dial(playerToken) // same invite, second device

	// Close the second device and WAIT for its serve to return, so anything
	// its teardown emits is already queued.
	_ = phone.Close(websocket.StatusNormalClosure, "put the phone down")
	select {
	case <-served:
	case <-time.After(10 * time.Second):
		t.Fatal("the phone's serve never returned")
	}

	// The marker: a command whose event must reach the watcher.
	raw, err := protojson.Marshal(&vttv1.ClientCommand{
		RequestId: "marker",
		Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
			SceneId: "marker-scene", Name: "Marker", GridWidth: 1, GridHeight: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wctx, wcancel := context.WithTimeout(context.Background(), 5*time.Second)
	if werr := watcher.Write(wctx, websocket.MessageText, raw); werr != nil {
		t.Fatalf("marker write: %v", werr)
	}
	wcancel()

	// Read up to the marker. ANY presence frame for the player in this window
	// is a spurious announcement: neither the phone's arrival nor its
	// departure may be visible, because the laptop never left.
	deadline := time.Now().Add(15 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("the marker event never arrived")
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, b, rerr := watcher.Read(ctx)
		cancel()
		if rerr != nil {
			t.Fatalf("read: %v", rerr)
		}
		var f vttv1.ServerFrame
		if protojson.Unmarshal(b, &f) != nil {
			continue
		}
		if pc := f.GetPresenceChanged(); pc != nil && pc.GetParticipantId() == playerID {
			t.Fatalf("a second device produced a spurious PresenceChanged{%v} — the participant "+
				"never left, so the table must hear nothing", pc.GetState())
		}
		if ev := f.GetEvent(); ev != nil && ev.GetSceneCreated().GetSceneId() == "marker-scene" {
			break
		}
	}

	// And the LAST connection closing is a real departure.
	_ = laptop.Close(websocket.StatusNormalClosure, "bye")
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, b, rerr := watcher.Read(ctx)
		cancel()
		if rerr != nil {
			t.Fatalf("waiting for the real departure: %v", rerr)
		}
		var f vttv1.ServerFrame
		if protojson.Unmarshal(b, &f) != nil {
			continue
		}
		if pc := f.GetPresenceChanged(); pc != nil && pc.GetParticipantId() == playerID {
			if pc.GetState() != vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED {
				t.Fatalf("last connection closing = %v, want DISCONNECTED", pc.GetState())
			}
			return
		}
	}
}

// TestAJoinerDoesNotWaitForItsOwnArrivalToBeAnnounced pins whose time the
// presence fan-out spends.
//
// serve announced the arrival BEFORE it started the event pump, and broadcast
// walks the registry SERIALLY, waiting up to presenceSendBudget per connection.
// So the announcement of your arrival was on the critical path of your own
// catch-up: N wedged peers cost N x budget, paid by the person joining, who has
// done nothing wrong. Measured at the registry: 1 stalled peer 101ms, 2 202ms,
// 3 302ms against a 100ms budget — with the production 3s budget and two dead
// tabs left open somewhere, a new player waits six seconds to see the board.
//
// The bound is deliberate and stays (spec §4.1): a client that is merely busy
// must not lose the frame. What changes is WHO WAITS. The announcement is other
// people's news; the catch-up is the joiner's own reason for connecting.
//
// The stall is ARRANGED, not raced: the wedged peer gets buffer 0 and never
// reads, so its outCh cannot accept anything, and the budget below is two
// orders of magnitude above the microseconds a healthy send takes.
func TestAJoinerDoesNotWaitForItsOwnArrivalToBeAnnounced(t *testing.T) {
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

	wedgedToken, _, err := ids.CreateInvite("Wedged", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}
	joinerToken, _, err := ids.CreateInvite("Joiner", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}

	const budget = 2 * time.Second
	srv := New(c, ids)
	srv.buffer = 0 // no slot for the wedged peer to hide behind
	srv.noProgress = 10 * time.Minute
	srv.presence.sendBudget = budget
	httpSrv := httptest.NewUnstartedServer(srv.Handler())
	httpSrv.Listener = &sndbufListener{Listener: httpSrv.Listener}
	httpSrv.Start()
	defer httpSrv.Close()

	dial := func(token string, client *http.Client) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var opts *websocket.DialOptions
		if client != nil {
			opts = &websocket.DialOptions{HTTPClient: client}
		}
		conn, _, err := websocket.Dial(ctx, httpSrv.URL+"/ws?token="+token+"&after=0", opts)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return conn
	}

	// The wedged peer: a pinned-small receive buffer, and it never reads. Its
	// catch-up alone exceeds the socket buffers, so the server's writer parks
	// and outCh stays full.
	wedged := dial(wedgedToken, tinyRecvBufClient())
	defer wedged.CloseNow()

	// Oversized events, appended AFTER it is connected and while it never
	// reads. Live broadcast is what parks the writer; a catch-up backlog does
	// not — an earlier version of this test seeded the log first and the peer
	// drained it happily, so the test measured nothing and passed against the
	// unfixed code. 28KB against pinned 8KB buffers forces real TCP
	// backpressure within a few writes.
	for i := range 40 {
		if _, err := c.Append(&vttv1.Envelope{
			EventId: fmt.Sprintf("seed-%d", i),
			Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
				SceneId: fmt.Sprintf("scn-%d", i), Name: bigSceneName, GridWidth: 4, GridHeight: 4,
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(500 * time.Millisecond) // let the writer genuinely park

	joiner := dial(joinerToken, nil)
	defer joiner.CloseNow()

	// Time to the joiner's own first EVENT, not its first frame.
	//
	// The catch-up HEAD is written straight to outCh before any of this and
	// arrives in about a millisecond either way — measuring that proves
	// nothing, and an earlier version of this test did exactly that and
	// passed against the unfixed code. The events are what the pump carries,
	// and the pump is what starts late.
	//
	// The window is not pure socket time: dial returns at the HTTP 101, so the
	// store's backlog read (~1.1MB, synchronous, under its lock) also falls
	// inside it. Warm pages and 22x of headroom cover that, but it is the one
	// honest flake vector on a disk-pressured machine.
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		_, raw, err := joiner.Read(ctx)
		if err != nil {
			t.Fatalf("joiner never received its catch-up events: %v", err)
		}
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &f); err != nil {
			t.Fatalf("frame did not decode: %v (raw=%s)", err, raw)
		}
		if f.GetEvent() != nil {
			break
		}
	}
	elapsed := time.Since(start)

	// AND THE WEDGE MUST HAVE BEEN REAL. "Fast" on its own cannot tell a
	// working fix from a peer that was never wedged — remove the tiny socket
	// buffers and the oversized events and this still finishes in 4ms, which
	// is exactly how the first three versions of this test passed against the
	// unfixed code.
	//
	// The witness is free and already here: the READ LOOP still starts after
	// the announcement (deliberately — see server.go), so the joiner's own
	// first command result is gated on the fan-out. If that came back quickly
	// the budget was never spent and this test proved nothing.
	cmdStart := time.Now()
	raw, err := protojson.Marshal(&vttv1.ClientCommand{
		RequestId: "probe",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wcancel()
	if err := joiner.Write(wctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("joiner write: %v", err)
	}
	for {
		_, rawFrame, err := joiner.Read(ctx)
		if err != nil {
			t.Fatalf("joiner never got its command result: %v", err)
		}
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(rawFrame, &f); err != nil {
			t.Fatalf("frame did not decode: %v", err)
		}
		if f.GetResult() != nil {
			break
		}
	}
	if spent := time.Since(cmdStart); spent < budget/2 {
		t.Fatalf("the joiner's own command was answered in %v, so the presence fan-out never "+
			"cost the %v budget — the peer was not actually wedged and this test proves nothing",
			spent.Round(time.Millisecond), budget)
	}

	// Half the budget is the margin: a healthy send is microseconds, so
	// anything approaching the budget means the fan-out is on this path.
	// Measured across 21 runs: worst 45ms under -race on a loaded machine,
	// against a 1000ms threshold, with the unfixed failure at 2.005s.
	if elapsed > budget/2 {
		t.Fatalf("the joiner waited %v for its first EVENT against a %v presence budget — its "+
			"own catch-up is queued behind announcing its arrival to a wedged stranger",
			elapsed.Round(time.Millisecond), budget)
	}
	t.Logf("joiner's first EVENT after %v (budget %v)", elapsed.Round(time.Millisecond), budget)
}
