package mcp_test

// This file's fake wire server deliberately re-documents the gateway's wire
// protocol (contract/README.md's wire conventions) using only httptest,
// coder/websocket, and contract types — mirroring internal/harness/
// client_test.go's fakeServer pattern (that type is unexported in package
// harness_test and cannot be imported; this is a parallel, not a shared,
// implementation). It does NOT import internal/gateway: .go-arch-lint.yml's
// mcp component forbids gateway/campaign/store/identity, test files
// included (P1 rule, task-1-brief.md's binding rule extended to this
// package).
//
// This file also covers server-session-lifecycle behavior: an unknown tool
// name surfaces as an SDK-level protocol error (server.go's callTool path
// requires no code from us — proven here, not just asserted in the report),
// and a tool call made after the underlying wire connection is lost (and
// permanently refused on redial) returns a clean MCP error rather than
// hanging or crashing the process.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	mcppkg "github.com/PatrikLager/vtt-platform/internal/mcp"
)

// --- fake wire server ----------------------------------------------------

// fakeServer is a scripted stand-in for the gateway: one /ws endpoint that
// decodes each inbound ClientCommand and hands it to onCommand, which uses
// sendResult to script canned ServerFrames back. maxConns, when non-zero,
// refuses (HTTP 503, never upgraded) every connection attempt past the
// first maxConns — the deterministic way to simulate "the wire is
// permanently down" for the disconnected/reconnect test, without any
// timing-dependent race between a redial succeeding and the test's
// assertion.
type fakeServer struct {
	t   *testing.T
	srv *httptest.Server

	mu        sync.Mutex
	lastQuery url.Values
	connCount int
	maxConns  int
	conns     []*websocket.Conn

	onCommand func(conn *websocket.Conn, cmd *vttv1.ClientCommand)
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
	fs.connCount++
	n := fs.connCount
	fs.lastQuery = r.URL.Query()
	refuse := fs.maxConns > 0 && n > fs.maxConns
	fs.mu.Unlock()

	if refuse {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	fs.mu.Lock()
	fs.conns = append(fs.conns, conn)
	fs.mu.Unlock()

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

// wsURL rewrites the httptest server's http(s):// URL to ws(s)://.
func (fs *fakeServer) wsURL() string {
	return "ws" + strings.TrimPrefix(fs.srv.URL, "http") + "/ws"
}

// firstConn waits for and returns the first accepted connection, for tests
// that need to force a mid-session drop from the server side.
func (fs *fakeServer) firstConn(t *testing.T) *websocket.Conn {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		fs.mu.Lock()
		if len(fs.conns) > 0 {
			c := fs.conns[0]
			fs.mu.Unlock()
			return c
		}
		fs.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fakeServer: no connection accepted in time")
	return nil
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

// sendResult scripts a CommandResult frame back to the client. errStr may be
// empty (the ok=true case never sets it).
func sendResult(t *testing.T, conn *websocket.Conn, requestID string, ok bool, errStr string, seq int64) {
	t.Helper()
	sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Result{Result: &vttv1.CommandResult{
		RequestId: requestID, Ok: ok, Error: errStr, Sequence: seq,
	}}})
}

// --- shared test scaffolding ----------------------------------------------

// loadToolsJSON reads the COMMITTED contract/gen/tools/tools.json — the
// same artifact cmd/vtt will go:embed in Task 3 — so these tests exercise
// mcp.New against real production data, not a hand-maintained fixture that
// could quietly drift from it.
func loadToolsJSON(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatalf("loadToolsJSON: %v", err)
	}
	return raw
}

// startSession builds an mcp.Server against wsURL/token, runs it over an
// in-memory MCP transport pair, and connects an SDK client to it — the
// stub-first behavioral RED harness for every test in this package. Per
// NewInMemoryTransports' doc comment ("servers must be connected before
// clients, as the client initializes the MCP session during connection"),
// srv.Run is started in a goroutine first; net.Pipe's synchronous,
// unbuffered semantics mean the client's blocking Connect call simply waits
// for the server side's Run to reach mcp.Server.Run internally (after its
// own harness dial), rather than racing or erroring.
func startSession(t *testing.T, wsURL string) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	srv, err := mcppkg.New(mcppkg.Config{WSURL: wsURL, Token: "test-token", ToolsJSON: loadToolsJSON(t)})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx, serverTransport) }()

	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()
	cl := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mcp-test-client", Version: "0.0.1"}, nil)
	cs, err := cl.Connect(connectCtx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatalf("client Connect: %v", err)
	}

	cleanup := func() {
		cs.Close()
		cancel()
		select {
		case <-runErr:
		case <-time.After(5 * time.Second):
			t.Error("startSession: Server.Run did not return after ctx cancel")
		}
	}
	return cs, cleanup
}

// --- tests -----------------------------------------------------------------

// TestCallUnknownToolIsSDKLevelError covers the "unknown tool" case: calling
// a tool name never registered via mcp.New surfaces as a protocol error from
// the SDK's own dispatch (server.go's callTool — see go-sdk mcp/server.go)
// with zero code of ours involved. No wire traffic is expected; onCommand
// must never fire.
func TestCallUnknownToolIsSDKLevelError(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		t.Fatalf("fakeServer: unexpected command for unknown-tool test: %v", cmd)
	})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "no_such_tool", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("want CallTool to error for an unregistered tool name")
	}
}

// TestCallToolWhileDisconnectedReturnsCleanError covers the reconnect-loss
// case (spec §9, resolved in server.go's Run/pump): the fake server accepts
// exactly one connection (maxConns=1); the test forces that connection
// closed from the server side, simulating a wire drop, and every subsequent
// dial attempt (the server's own redial loop) is refused at the HTTP layer
// — never upgraded — so there is no race window where a fast redial could
// flip the tool call back to "connected" before this test observes the
// disconnected state. The MCP session (the in-memory transport) stays
// alive throughout: only the underlying harness wire is down, and the tool
// call must return a clean error, not hang or crash the process.
func TestCallToolWhileDisconnectedReturnsCleanError(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		// Never reached in this test: the connection is dropped before any
		// tool call is made.
	})
	fs.maxConns = 1
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	if err := conn.Close(websocket.StatusNormalClosure, "test: forcing a wire drop"); err != nil {
		t.Fatalf("forcing connection close: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		_, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "end_session",
			Arguments: map[string]any{},
		})
		cancel()
		if err != nil {
			return // clean MCP error observed — the connection stayed intact
			// (the session itself never errors; only this tool call did).
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("want CallTool to eventually return an error while permanently disconnected, got nil (lastErr=%v)", lastErr)
}

// TestServerInstructionsScopeTheInt64AsStringRuleToNotGetState covers the
// honest-conventions fix (final review Fix 3a): before this fix, the
// server-level Instructions' int64-as-string bullet read as a blanket
// claim over every tool's output, which is false for get_state (its body
// follows the state dump's own Go-JSON conventions, not protojson — see
// read_tools_test.go's TestGetStateSessionSequenceFieldsAreGoJSONNumbers
// NotProtojsonStrings for the behavioral proof). Instructions must now
// name get_state as the documented exception, not just assert the rule.
func TestServerInstructionsScopeTheInt64AsStringRuleToNotGetState(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	instr := cs.InitializeResult().Instructions
	if !strings.Contains(instr, "get_state") {
		t.Fatalf("server Instructions never mentions get_state's exception to the int64-as-string rule:\n%s", instr)
	}
	if !strings.Contains(instr, "exception") {
		t.Fatalf("server Instructions does not scope the int64-as-string rule as having an exception:\n%s", instr)
	}
}

// TestServerInstructionsMentionNarrationAsTableMemory covers the world-
// layer spec §5 deliverable that was dropped in the spec-to-plan
// translation (merge-gate MUST-FIX): "Instructions text: one line telling
// the agent narration is how the table remembers its story." The shipped
// instructions const had no such line — an MCP agent had no way to
// discover add_narration/upsert_note's PURPOSE (as opposed to their bare
// names) from the server's own self-description. Pins both halves: the
// narration-is-story-memory line, and world notes named as the durable
// counterpart.
func TestServerInstructionsMentionNarrationAsTableMemory(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	instr := cs.InitializeResult().Instructions
	if !strings.Contains(instr, "narration") {
		t.Fatalf("server Instructions never mentions narration:\n%s", instr)
	}
	if !strings.Contains(instr, "remembers") {
		t.Fatalf("server Instructions does not say narration is how the table remembers its story:\n%s", instr)
	}
	if !strings.Contains(instr, "notes") {
		t.Fatalf("server Instructions never mentions world notes as the table's durable memory:\n%s", instr)
	}
}
