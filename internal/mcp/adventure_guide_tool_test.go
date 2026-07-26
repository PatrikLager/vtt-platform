package mcp_test

// adventure_guide_tool_test.go covers get_adventure_guide (adventure_guide_
// tool.go, adventure-format Task 4): guide_tool_test.go's get_ruleset_guide
// precedent, extended with an adventure_id argument — the tool returns
// Config.AdventureGuides[adventure_id] verbatim when present, a clean
// tool-level isError naming "no adventures available" when the map is
// empty (no --adventures-dir), and a clean tool-level isError naming the
// unknown id when the map is non-empty but doesn't contain it.

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

// startSessionWithAdventureGuides is startSessionWithGuide's (guide_tool_
// test.go) sibling, configuring AdventureGuides instead of RulesetGuide.
func startSessionWithAdventureGuides(t *testing.T, wsURL string, guides map[string]string) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	srv, err := mcppkg.New(mcppkg.Config{WSURL: wsURL, Token: "test-token", ToolsJSON: loadToolsJSON(t), AdventureGuides: guides})
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
			t.Error("startSessionWithAdventureGuides: Server.Run did not return after ctx cancel")
		}
	}
	return cs, cleanup
}

func TestGetAdventureGuideReturnsGuideTextWhenConfigured(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	guides := map[string]string{"goblin-ambush": "# Goblin Ambush\n\nThe archer flees once badly wounded."}
	cs, cleanup := startSessionWithAdventureGuides(t, fs.wsURL(), guides)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_adventure_guide",
		Arguments: map[string]any{"adventureId": "goblin-ambush"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("want IsError=false for a configured adventure id, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	if text.Text != guides["goblin-ambush"] {
		t.Fatalf("guide text = %q, want the configured guide verbatim (%q)", text.Text, guides["goblin-ambush"])
	}
}

func TestGetAdventureGuideNoAdventuresConfiguredCleanError(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSessionWithAdventureGuides(t, fs.wsURL(), nil) // no --adventures-dir
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_adventure_guide",
		Arguments: map[string]any{"adventureId": "goblin-ambush"},
	})
	if err != nil {
		t.Fatalf("CallTool: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("want IsError=true when no adventures are configured, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "no adventures available") {
		t.Fatalf("error text = %q, want it to contain %q", text.Text, "no adventures available")
	}
}

func TestGetAdventureGuideUnknownIdCleanError(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	guides := map[string]string{"goblin-ambush": "guide contents"}
	cs, cleanup := startSessionWithAdventureGuides(t, fs.wsURL(), guides)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_adventure_guide",
		Arguments: map[string]any{"adventureId": "no-such-adventure"},
	})
	if err != nil {
		t.Fatalf("CallTool: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("want IsError=true for an unknown adventure id, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "no-such-adventure") {
		t.Fatalf("error text = %q, want it to name the unknown adventure id", text.Text)
	}
}

// TestGetAdventureGuideWorksWhileDisconnected mirrors
// TestGetRulesetGuideWorksWhileDisconnected (guide_tool_test.go): a pure
// config read, independent of the underlying wire connection.
func TestGetAdventureGuideWorksWhileDisconnected(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	fs.maxConns = 1
	guides := map[string]string{"goblin-ambush": "guide contents"}
	cs, cleanup := startSessionWithAdventureGuides(t, fs.wsURL(), guides)
	defer cleanup()

	conn := fs.firstConn(t)
	if err := conn.Close(websocket.StatusNormalClosure, "test: forcing a wire drop"); err != nil {
		t.Fatalf("forcing connection close: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "get_adventure_guide",
			Arguments: map[string]any{"adventureId": "goblin-ambush"},
		})
		cancel()
		if err == nil && !res.IsError {
			text, ok := res.Content[0].(*mcpsdk.TextContent)
			if ok && text.Text == "guide contents" {
				return // succeeded while disconnected — proof complete.
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("want get_adventure_guide to keep working while the wire is disconnected")
}
