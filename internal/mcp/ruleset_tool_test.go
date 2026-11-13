package mcp_test

import (
	"context"
	"net/http"
	"strings"
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

// VTT-694
func TestGetRulesetSaysWhichAbilitiesEachRoleReads(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name != "get_ruleset" {
			continue
		}
		for _, want := range []string{"every ability for the DM and the agent",
			"for a player only the abilities named by the sheets of the actors they control"} {
			if !strings.Contains(tool.Description, want) {
				t.Errorf("get_ruleset's description = %q, want it to say %q", tool.Description, want)
			}
		}
		if strings.Contains(tool.Description, "as every participant can") {
			t.Errorf("get_ruleset's description still says it reads %q", "as every participant can")
		}
		return
	}
	t.Fatal("no get_ruleset tool is served")
}
