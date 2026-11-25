package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mcp"
)

// Keep this socket a stand-in: the package may not import the gateway.
func qaivSocket(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		head, _ := protojson.Marshal(&vttv1.ServerFrame{
			Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{}},
		})
		_ = conn.Write(r.Context(), websocket.MessageText, head)
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func qaivTools(t *testing.T) map[string]*mcpsdk.Tool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "vtt", "tools.json"))
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	s, err := mcp.New(mcp.Config{WSURL: qaivSocket(t), Token: "qaiv", ToolsJSON: raw})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	st, ct := mcpsdk.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx, st) }()
	cl := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qaiv", Version: "0"}, nil)
	cs, err := cl.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		cancel()
		<-done
	})
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	out := map[string]*mcpsdk.Tool{}
	for _, tool := range res.Tools {
		out[tool.Name] = tool
	}
	return out
}

var qaivSentence = regexp.MustCompile(`[^.]*[.]`)

func qaivSays(text string) (string, bool) {
	for _, s := range qaivSentence.FindAllString(text, -1) {
		l := strings.ToLower(s)
		if strings.Contains(l, "form") && strings.Contains(l, "lists") &&
			(strings.Contains(l, "refus") || strings.Contains(l, "never")) {
			return strings.TrimSpace(s), true
		}
	}
	return "", false
}

// VTT-1014
func TestQAIVGetRulesetSaysAListedKeyIsRefusedOnASheet(t *testing.T) {
	tool := qaivTools(t)["get_ruleset"]
	if tool == nil {
		t.Fatal("get_ruleset is not listed")
	}
	got, ok := qaivSays(tool.Description)
	if !ok || !strings.Contains(strings.ToLower(got), "sheet") {
		t.Fatalf("get_ruleset's description says no such thing:\n%s", tool.Description)
	}
	t.Logf("get_ruleset: %s", got)
}

// VTT-1014
func TestQAIVAddActorsAttributesSayAListedKeyIsRefused(t *testing.T) {
	tool := qaivTools(t)["add_actor"]
	if tool == nil {
		t.Fatal("add_actor is not listed")
	}
	schema, _ := tool.InputSchema.(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	actor, _ := props["actor"].(map[string]any)
	fields, _ := actor["properties"].(map[string]any)
	attrs, _ := fields["attributes"].(map[string]any)
	desc, _ := attrs["description"].(string)
	got, ok := qaivSays(desc)
	if !ok || !strings.Contains(got, "refuses") {
		t.Fatalf("add_actor's attributes say no such thing:\n%s", desc)
	}
	t.Logf("add_actor.attributes: %s", got)
}
