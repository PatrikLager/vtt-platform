package mcp_test

// guide_tool_test.go covers get_ruleset_guide (guide_tool.go, ruleset-
// interpreter Task 6): the tool returns Config.RulesetGuide verbatim when
// set, and a clean tool-level isError naming "no ruleset loaded" when it
// is empty — the P1-boundary-respecting design (this package never reads a
// ruleset directory itself; cmd/vtt's --ruleset flag already validated it
// via rules.Load and hands this package only the resulting guide TEXT, see
// mcp.Config.RulesetGuide's own doc comment).

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	mcppkg "github.com/PatrikLager/vtt-platform/internal/mcp"
)

// startSessionWithGuide is startSession's (server_test.go) sibling,
// configuring RulesetGuide too — a small, deliberate duplication (this
// package's own established precedent: server_test.go's fake wire is
// itself a parallel, not shared, implementation of a harness test helper).
func startSessionWithGuide(t *testing.T, wsURL, guide string) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	srv, err := mcppkg.New(mcppkg.Config{WSURL: wsURL, Token: "test-token", ToolsJSON: loadToolsJSON(t), RulesetGuide: guide})
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
			t.Error("startSessionWithGuide: Server.Run did not return after ctx cancel")
		}
	}
	return cs, cleanup
}

func TestGetRulesetGuideReturnsGuideTextWhenConfigured(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSessionWithGuide(t, fs.wsURL(), "# Tavern Brawl\n\nFists is at-will.")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_ruleset_guide", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("want IsError=false when a guide is configured, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	if text.Text != "# Tavern Brawl\n\nFists is at-will." {
		t.Fatalf("guide text = %q, want the configured guide verbatim", text.Text)
	}
}

func TestGetRulesetGuideNoRulesetLoadedCleanError(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSessionWithGuide(t, fs.wsURL(), "") // no --ruleset
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_ruleset_guide", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("want IsError=true when no ruleset is configured, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "no ruleset loaded") {
		t.Fatalf("error text = %q, want it to contain %q", text.Text, "no ruleset loaded")
	}
}

// TestGetRulesetGuideWorksWhileDisconnected proves the guide tool is a
// pure config read, independent of the underlying wire connection (unlike
// get_state/get_events_since, which still work off accumulated history
// while disconnected — this one needs no history at all): force the one
// connection closed server-side, then call the tool anyway.
func TestGetRulesetGuideWorksWhileDisconnected(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	fs.maxConns = 1
	cs, cleanup := startSessionWithGuide(t, fs.wsURL(), "guide contents")
	defer cleanup()

	conn := fs.firstConn(t)
	if err := conn.Close(websocket.StatusNormalClosure, "test: forcing a wire drop"); err != nil {
		t.Fatalf("forcing connection close: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_ruleset_guide", Arguments: map[string]any{}})
		cancel()
		if err == nil && !res.IsError {
			text, ok := res.Content[0].(*mcpsdk.TextContent)
			if ok && text.Text == "guide contents" {
				return // succeeded while disconnected — proof complete.
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("want get_ruleset_guide to keep working while the wire is disconnected")
}
