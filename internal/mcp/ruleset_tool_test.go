package mcp_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-621
func TestGetRulesetReturnsTheRulesetRouteVerbatim(t *testing.T) {
	const answer = `{"id":"qa","name":"QA","abilities":[],"conditions":[],"resources":["pool_a"],` +
		`"properties":[{"key":"pool_a","type":"resource","name":"Pool A","description":"What a stance spends."}]}`
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	var (
		mu               sync.Mutex
		gotPath, gotAuth string
	)
	fs.api = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(answer))
	}

	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	got, err := callTool(t, cs, "get_ruleset")
	if err != nil {
		t.Fatalf("get_ruleset: %v", err)
	}
	mu.Lock()
	path, auth := gotPath, gotAuth
	mu.Unlock()
	if path != "/api/ruleset" {
		t.Fatalf("the tool asked for %q, want /api/ruleset", path)
	}
	if auth != "Bearer test-token" {
		t.Fatalf("Authorization was %q, want the server's token", auth)
	}
	if got != answer {
		t.Fatalf("the tool returned\n %s\nwant the route's answer verbatim\n %s", got, answer)
	}
}
