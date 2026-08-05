package harness_test

// This file's fake wire server deliberately re-documents the gateway's wire
// protocol (contract/README.md's wire conventions, docs/superpowers/specs/
// 2026-07-23-api-gateway-design.md §3: connect URL `/ws?token=<t>&after=<n>`,
// protojson TEXT frames, ClientCommand in / ServerFrame out) using only
// httptest, coder/websocket, and contract types — it does NOT import
// internal/gateway. That import boundary is the point of internal/harness
// (see client.go's package comment): Task 1's binding rule forbids
// store/campaign/gateway/identity from this package, including test files.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// --- fake wire server ----------------------------------------------------

// fakeServer is a scripted stand-in for the gateway: one /ws endpoint that
// decodes each inbound ClientCommand and hands it to onCommand, which uses
// sendResult/sendEvent to script canned ServerFrames back.
type fakeServer struct {
	t   *testing.T
	srv *httptest.Server

	mu        sync.Mutex
	lastQuery url.Values

	onCommand func(conn *websocket.Conn, cmd *vttv1.ClientCommand)

	// onConnect runs once per accepted connection, before the read loop, so
	// a test can reproduce the real gateway's opening CatchUpHead frame (and,
	// if it wants, hang up immediately afterwards). Set it before dialing;
	// the dial is what starts the server goroutine that reads it.
	onConnect func(conn *websocket.Conn)
}

func newFakeServer(t *testing.T, onCommand func(conn *websocket.Conn, cmd *vttv1.ClientCommand)) *fakeServer {
	t.Helper()
	fs := &fakeServer{t: t, onCommand: onCommand}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", fs.handleWS)
	fs.srv = httptest.NewServer(mux)
	t.Cleanup(fs.srv.Close)
	return fs
}

func (fs *fakeServer) handleWS(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	fs.lastQuery = r.URL.Query()
	fs.mu.Unlock()

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	if fs.onConnect != nil {
		fs.onConnect(conn)
	}

	ctx := r.Context()
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var cmd vttv1.ClientCommand
		if err := protojson.Unmarshal(raw, &cmd); err != nil {
			fs.t.Errorf("fakeServer: client sent malformed command: %v", err)
			return
		}
		if fs.onCommand != nil {
			fs.onCommand(conn, &cmd)
		}
	}
}

// query returns the value the most recent /ws connection was dialed with.
func (fs *fakeServer) query(key string) string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.lastQuery == nil {
		return ""
	}
	return fs.lastQuery.Get(key)
}

// wsURL rewrites the httptest server's http(s):// URL to ws(s)://.
func (fs *fakeServer) wsURL() string {
	return "ws" + strings.TrimPrefix(fs.srv.URL, "http") + "/ws"
}

func sendFrame(t *testing.T, conn *websocket.Conn, frame *vttv1.ServerFrame) {
	t.Helper()
	raw, err := protojson.Marshal(frame)
	if err != nil {
		t.Fatalf("fakeServer: marshal frame: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("fakeServer: write frame: %v", err)
	}
}

func sendResult(t *testing.T, conn *websocket.Conn, requestID string, ok bool, seq int64) {
	t.Helper()
	sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Result{Result: &vttv1.CommandResult{
		RequestId: requestID, Ok: ok, Sequence: seq,
	}}})
}

func sendEvent(t *testing.T, conn *websocket.Conn, eventID string, seq int64) {
	t.Helper()
	sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: &vttv1.Envelope{
		EventId: eventID, Sequence: seq,
	}}})
}

func endSessionCommand() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}}}
}

func dial(t *testing.T, fs *fakeServer, token string, after int64) *harness.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := harness.Dial(ctx, fs.wsURL(), token, after)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// --- tests -----------------------------------------------------------------

// TestDialSendsTokenAndAfterInURLQuery covers the connect contract: Dial
// puts token and after in the /ws URL's query string exactly as the gateway
// expects (spec §3: `/ws?token=<t>&after=<n>`).
func TestDialSendsTokenAndAfterInURLQuery(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	dial(t, fs, "tok-abc123", 42)

	if got := fs.query("token"); got != "tok-abc123" {
		t.Fatalf("token query param = %q, want tok-abc123", got)
	}
	if got := fs.query("after"); got != "42" {
		t.Fatalf("after query param = %q, want 42", got)
	}
}

// TestSendCommandCorrelatesByRequestIDWithInterleavedEvent covers the core
// demux contract: the fake server sends an unrelated Envelope BEFORE the
// CommandResult for the in-flight command. SendCommand must still return
// only once ITS result arrives (not stop early on the interleaved event),
// and the interleaved event must still reach Events(), not get dropped.
func TestSendCommandCorrelatesByRequestIDWithInterleavedEvent(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		sendEvent(t, conn, "ev-interleaved", 1)
		sendResult(t, conn, cmd.GetRequestId(), true, 7)
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := endSessionCommand()
	res, err := c.SendCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if cmd.GetRequestId() == "" {
		t.Fatal("want SendCommand to assign a non-empty request_id on cmd")
	}
	if !res.GetOk() || res.GetSequence() != 7 {
		t.Fatalf("result = %+v, want ok=true sequence=7", res)
	}
	if res.GetRequestId() != cmd.GetRequestId() {
		t.Fatalf("result.RequestId = %q, want %q (the id SendCommand assigned)", res.GetRequestId(), cmd.GetRequestId())
	}

	select {
	case env := <-c.Events():
		if env.GetEventId() != "ev-interleaved" {
			t.Fatalf("event id = %q, want ev-interleaved", env.GetEventId())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("want the interleaved event to still arrive on Events()")
	}
}

// TestConcurrentSendCommandsCorrelateByRequestID proves correlation is keyed
// by request_id, not by call/arrival order: the fake server stalls its reply
// to the FIRST command it receives until AFTER it has already answered the
// second, so a send-order-based (or single-shared-channel) implementation
// would hand each caller the wrong result.
func TestConcurrentSendCommandsCorrelateByRequestID(t *testing.T) {
	var mu sync.Mutex
	seenCount := 0
	firstArrived := make(chan struct{})

	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		id := cmd.GetRequestId()
		if id == "" {
			t.Errorf("fakeServer: command arrived with empty request_id")
		}

		mu.Lock()
		seenCount++
		isFirst := seenCount == 1
		mu.Unlock()

		if isFirst {
			close(firstArrived)
			<-time.After(150 * time.Millisecond) // give the 2nd command time to arrive and be answered
			sendResult(t, conn, id, true, 100)
			return
		}
		<-firstArrived
		sendResult(t, conn, id, true, 200)
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			cmd := endSessionCommand()
			res, err := c.SendCommand(ctx, cmd)
			if err != nil {
				t.Errorf("SendCommand: %v", err)
				return
			}
			// The discriminating assertion: whichever sequence THIS call
			// got back, it must carry the SAME request_id this call sent —
			// proof the client paired result to caller by request_id, not
			// by wire arrival order (which is reversed here: request_id for
			// sequence=100 arrives on the wire strictly AFTER sequence=200).
			if res.GetRequestId() != cmd.GetRequestId() {
				t.Errorf("result.RequestId = %q, want %q (this call's own id); got sequence=%d",
					res.GetRequestId(), cmd.GetRequestId(), res.GetSequence())
			}
		}()
	}
	wg.Wait()
}

// TestEventsStreamInOrder covers ordering: a burst of events must arrive on
// Events() in exactly the order the server sent them.
func TestEventsStreamInOrder(t *testing.T) {
	const n = 20
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		for i := 0; i < n; i++ {
			sendEvent(t, conn, fmt.Sprintf("ev-%d", i), int64(i+1))
		}
		sendResult(t, conn, cmd.GetRequestId(), true, int64(n+1))
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		_, _ = c.SendCommand(ctx, endSessionCommand())
	}()

	for i := 0; i < n; i++ {
		select {
		case env := <-c.Events():
			want := fmt.Sprintf("ev-%d", i)
			if env.GetEventId() != want {
				t.Fatalf("event[%d] = %q, want %q (out of order)", i, env.GetEventId(), want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}

// TestServerCloseClosesEventsAndErrorsPendingSendCommand covers disconnect
// semantics: when the server closes the connection while a command is
// in-flight, that SendCommand call must return an error, and Events() must
// close (not hang forever unconsumed).
func TestServerCloseClosesEventsAndErrorsPendingSendCommand(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		conn.Close(websocket.StatusNormalClosure, "fake: server closing")
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.SendCommand(ctx, endSessionCommand()); err == nil {
		t.Fatal("want SendCommand to error after the server closes the connection")
	}

	select {
	case _, ok := <-c.Events():
		if ok {
			t.Fatal("want Events() closed after server close, got a value instead")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("want Events() closed after server close")
	}
}

// wantBufferedEvents mirrors harness's unexported eventBuffer const (256).
// It can't be imported (package harness_test is a black-box test package,
// and the constant is deliberately unexported — see client.go), so it is
// re-declared here, at the boundary this test actually pins: "the client
// buffers exactly this many events before tearing itself down," not any
// internal field name.
const wantBufferedEvents = 256

// TestEventsBufferOverflowClosesConnectionAndErrorsSendCommand covers the
// overflow policy (client.go's eventBuffer/deliverEvent doc comments): the
// fake server floods wantBufferedEvents+1 events for one command, without
// this test ever draining Events() in the meantime, forcing the client's
// buffered channel past capacity. It asserts both halves of the contract:
// (a) the in-flight SendCommand call errors, wrapping ErrEventsOverflow
// (deliverEvent's overflow branch tears the connection down instead of
// blocking, so the command's CommandResult — which the fake server never
// even sends — can never arrive); (b) Events() still delivers exactly the
// wantBufferedEvents events that made it into the buffer, in order, before
// closing — overflow must not drop or reorder what was already safely
// buffered, only refuse the one event that didn't fit.
// TestCloseErrNamesOverflowSoAReaderCanTellWhyEventsEnded pins the half of the
// overflow contract that had no accessor at all.
//
// A consumer of Events() sees only a CLOSED CHANNEL. It cannot distinguish
// "the server finished sending" from "I was disconnected for reading too
// slowly" — and those demand opposite responses. That gap produced a real CI
// failure on 2026-08-04: the soak keystone's fresh catch-up overflowed at 257
// of 480 envelopes, and runSoakCheckpoint reported "never reached sequence 480
// within 30s" for a connection that had died in under three seconds. It sent
// the reader hunting slow CI while the cause was this buffer.
//
// SendCommand already surfaces the reason, but only to a caller who happens to
// have a command in flight. A pure reader has no such caller.
func TestCloseErrNamesOverflowSoAReaderCanTellWhyEventsEnded(t *testing.T) {
	const flood = wantBufferedEvents + 1
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		for i := 0; i < flood; i++ {
			sendEvent(t, conn, fmt.Sprintf("ev-%d", i), int64(i+1))
		}
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = c.SendCommand(ctx, endSessionCommand())

	// Drain to the close, exactly as a pure reader would.
	for range c.Events() {
	}

	if err := c.CloseErr(); !errors.Is(err, harness.ErrEventsOverflow) {
		t.Fatalf("CloseErr() = %v, want it to wrap harness.ErrEventsOverflow — a reader that "+
			"only sees a closed channel cannot otherwise tell an overflow from a completed stream", err)
	}
}

// TestCloseErrIsNilWhileTheConnectionIsHealthy is the other half: CloseErr must
// not manufacture a reason where there is none.
//
// Named for what it actually pins. It does NOT cover "the stream ended
// normally" — fakeServer loops back into conn.Read after onCommand returns, so
// the connection is still OPEN here and closeErr is nil by construction. A
// clean close on either side in fact reports NON-nil (the websocket read error
// that ended the loop), which is why the useful question is
// errors.Is(..., ErrEventsOverflow) and not a bare nil check.
func TestCloseErrIsNilWhileTheConnectionIsHealthy(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		sendEvent(t, conn, "ev-1", 1)
		sendResult(t, conn, cmd.GetRequestId(), true, 1)
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.SendCommand(ctx, endSessionCommand()); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if err := c.CloseErr(); err != nil {
		t.Errorf("CloseErr() = %v on a healthy connection, want nil", err)
	}
}

func TestEventsBufferOverflowClosesConnectionAndErrorsSendCommand(t *testing.T) {
	const flood = wantBufferedEvents + 1 // exactly one past capacity — the
	// minimum that guarantees the overflow branch fires exactly once, so
	// the fake server never attempts a write after the client has torn the
	// connection down (which would otherwise be a second, unrelated way for
	// this test to fail).
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		for i := 0; i < flood; i++ {
			sendEvent(t, conn, fmt.Sprintf("ev-%d", i), int64(i+1))
		}
		// Deliberately no CommandResult: overflow must tear the connection
		// down (and error the caller) before one would ever be needed.
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.SendCommand(ctx, endSessionCommand())
	if err == nil {
		t.Fatal("want SendCommand to error when the events buffer overflows")
	}
	if !errors.Is(err, harness.ErrEventsOverflow) {
		t.Fatalf("SendCommand error = %v, want it to wrap harness.ErrEventsOverflow", err)
	}

	for i := 0; i < wantBufferedEvents; i++ {
		select {
		case env, ok := <-c.Events():
			if !ok {
				t.Fatalf("Events() closed early at i=%d, want %d buffered events first", i, wantBufferedEvents)
			}
			want := fmt.Sprintf("ev-%d", i)
			if env.GetEventId() != want {
				t.Fatalf("event[%d] = %q, want %q", i, env.GetEventId(), want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for buffered event %d", i)
		}
	}

	select {
	case _, ok := <-c.Events():
		if ok {
			t.Fatal("want Events() closed after draining the buffered events, got another value instead")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("want Events() closed after draining the buffered events")
	}
}

// TestMalformedFrameFromServerErrorsLoudly covers protocol-violation
// handling: a syntactically invalid frame from the server must not be
// silently dropped — the in-flight SendCommand errors and Events() closes.
func TestMalformedFrameFromServerErrorsLoudly(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := conn.Write(ctx, websocket.MessageText, []byte("{not valid protojson")); err != nil {
			t.Fatalf("fakeServer: write malformed frame: %v", err)
		}
	})
	c := dial(t, fs, "tok", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.SendCommand(ctx, endSessionCommand()); err == nil {
		t.Fatal("want SendCommand to error after a malformed server frame")
	}

	select {
	case _, ok := <-c.Events():
		if ok {
			t.Fatal("want Events() closed after a malformed server frame")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("want Events() closed after a malformed server frame")
	}
}

// TestDialErrorNeverIncludesTheRawToken covers the token-redaction
// requirement (final review Fix 1): dialing a dead port (nothing listening,
// so the underlying HTTP handshake fails and coder/websocket's own dial
// error embeds the full request URL verbatim — net/http's *url.Error
// formats as `Get "<url>": <cause>`, and that URL is exactly the one Dial
// built with token= in its query string) must produce an error whose
// message never contains the raw invite token, but DOES carry a visible
// redaction marker in its place. This matters because every `vtt mcp`
// (and tail/dump/run) caller prints Dial's error straight to stderr, and
// an MCP host persists stderr — an unredacted token there is a credential
// leak into whatever log captures it.
func TestDialErrorNeverIncludesTheRawToken(t *testing.T) {
	const secretToken = "super-secret-invite-token-xyz"

	// 127.0.0.1:1 is a reserved/unassigned port nothing is listening on:
	// a fast, deterministic connection-refused failure with no real
	// network dependency.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := harness.Dial(ctx, "ws://127.0.0.1:1/ws", secretToken, 0)
	if err == nil {
		t.Fatal("Dial: want an error dialing a dead port, got nil")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Fatalf("Dial error contains the raw token: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("Dial error does not show a redaction marker: %v", err)
	}
}

// sendCatchUpHead writes the one frame every real gateway connection opens
// with, so a fake server can reproduce the announcement CatchUpHead reads.
func sendCatchUpHead(t *testing.T, conn *websocket.Conn, head int64) {
	t.Helper()
	sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_CatchUpHead{
		CatchUpHead: &vttv1.CatchUpHead{HeadSequence: head},
	}})
}

// TestCatchUpHeadReturnsAnnouncedHeadAfterConnectionEnds pins the precedence
// between the two facts CatchUpHead can hold at once: the head WAS announced,
// and the connection has SINCE ended. The announcement is a fact that outlives
// the connection, so it must win — the close only matters when the head never
// arrived at all.
//
// This is not hypothetical ordering pedantry. A server announces the head and
// then hangs up (end of session, gateway restart, an overflowing subscription
// force-closed); every caller that asks afterwards is entitled to the number.
// Reporting "connection ended before the server announced its catch-up head"
// for a head already in hand is a lie, and `vtt state dump` turns it into a
// refusal to print a snapshot it could have printed.
//
// Both channels are permanently closed by the time the loop runs, so each
// call independently re-samples any choice between them: a select that treats
// them as equals picks uniformly, and 200 draws make surviving this by luck a
// 2^-200 event. Close() is the synchronisation — it blocks on readerDone (see
// client.go), so there is nothing timing-dependent here to guess at.
func TestCatchUpHeadReturnsAnnouncedHeadAfterConnectionEnds(t *testing.T) {
	// The server must not hang up until the client has actually taken the
	// head off the wire. Closing straight after the write races handleWS's
	// own `defer conn.CloseNow()`, which tears the TCP connection down
	// abruptly and can strand the frame unread — the client then fails for
	// a reason that has nothing to do with the precedence under test.
	released := make(chan struct{})
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	fs.onConnect = func(conn *websocket.Conn) {
		sendCatchUpHead(t, conn, 7)
		<-released
		_ = conn.Close(websocket.StatusNormalClosure, "fake: announced, now hanging up")
	}

	c := dial(t, fs, "tok", 0)

	warmCtx, warmCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer warmCancel()
	if head, err := c.CatchUpHead(warmCtx); err != nil || head != 7 {
		t.Fatalf("setup: CatchUpHead on a live connection = (%d, %v), want (7, nil)", head, err)
	}
	close(released)

	// Events() is closed by readLoop's teardown, so draining it to closure
	// is the proof that the connection has really ended — no sleep, no
	// silence window. Close() then waits on readerDone specifically.
	for range c.Events() { //nolint:revive // draining to closure IS the wait
	}
	_ = c.Close()

	for i := range 200 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		head, err := c.CatchUpHead(ctx)
		cancel()
		if err != nil {
			t.Fatalf("call %d: CatchUpHead after the connection ended = error %v, want the announced head 7", i, err)
		}
		if head != 7 {
			t.Fatalf("call %d: CatchUpHead = %d, want 7", i, head)
		}
	}
}

// TestCatchUpHeadReturnsAnnouncedHeadDespiteCanceledContext is the same
// precedence question against the other competing signal: a caller whose ctx
// is already dead asking for a head the client is already holding. No wire
// access is needed to answer, so ctx expiry has nothing to abort — returning
// ctx.Err() would discard a fact rather than avoid a wait.
//
// Same 200-draw argument as above; here the connection stays live, so the
// only two ready signals are the announcement and the dead context.
func TestCatchUpHeadReturnsAnnouncedHeadDespiteCanceledContext(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	fs.onConnect = func(conn *websocket.Conn) { sendCatchUpHead(t, conn, 42) }

	c := dial(t, fs, "tok", 0)

	// Establish that the announcement has landed before racing it against a
	// dead context, so a failure below can only mean precedence.
	warmCtx, warmCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer warmCancel()
	if head, err := c.CatchUpHead(warmCtx); err != nil || head != 42 {
		t.Fatalf("setup: CatchUpHead = (%d, %v), want (42, nil)", head, err)
	}

	dead, deadCancel := context.WithCancel(context.Background())
	deadCancel()
	for i := range 200 {
		head, err := c.CatchUpHead(dead)
		if err != nil {
			t.Fatalf("call %d: CatchUpHead with a canceled ctx = error %v, want the known head 42", i, err)
		}
		if head != 42 {
			t.Fatalf("call %d: CatchUpHead = %d, want 42", i, head)
		}
	}
}
