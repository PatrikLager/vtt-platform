package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PatrikLager/vtt-platform/internal/identity"
	mcppkg "github.com/PatrikLager/vtt-platform/internal/mcp"
)

var qaRRRulesetDirs = []string{"../../rulesets/tavern-brawl", "../../rulesets/dnd45e-minimal"}

type qaRRGateway struct {
	direct string
	front  *httptest.Server
	token  string

	mu     sync.Mutex
	asked  []string
	refuse int
}

func qaRRGet(t *testing.T, url, token string) (int, []byte, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

func qaRRServe(t *testing.T, rulesetDir string) *qaRRGateway {
	t.Helper()
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir, "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})
	go func() { _ = srv.Serve(ln) }()
	g := &qaRRGateway{direct: "http://" + ln.Addr().String()}
	if err := waitForHealthz(g.direct, 3*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}
	g.token = mintInviteToken(t, campaignPath, identity.RoleAgent, "qa-rr-agent")
	target, err := url.Parse(g.direct)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	g.front = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ruleset" {
			g.mu.Lock()
			g.asked = append(g.asked, r.Method+" "+r.Header.Get("Authorization"))
			refuse := g.refuse
			g.mu.Unlock()
			if refuse != 0 {
				http.Error(w, "qa-rr refusal", refuse)
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(g.front.Close)
	return g
}

func (g *qaRRGateway) qaRRSetRefuse(status int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refuse = status
}

func (g *qaRRGateway) qaRRAsked() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.asked)
}

func qaRRSession(t *testing.T, g *qaRRGateway) *mcpsdk.ClientSession {
	t.Helper()
	srv, err := mcppkg.New(mcppkg.Config{
		WSURL:     "ws" + strings.TrimPrefix(g.front.URL, "http") + "/ws",
		Token:     g.token,
		ToolsJSON: toolsJSON,
	})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	clientEnd, serverEnd := mcpsdk.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, serverEnd) }()
	connectCtx, connectCancel := context.WithTimeout(ctx, 20*time.Second)
	defer connectCancel()
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa-rr", Version: "0"}, nil).Connect(connectCtx, clientEnd, nil)
	if err != nil {
		cancel()
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("Run did not return after the session closed")
		}
	})
	return cs
}

func qaRRCall(t *testing.T, cs *mcpsdk.ClientSession) (*mcpsdk.CallToolResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_ruleset"})
}

func qaRRText(t *testing.T, res *mcpsdk.CallToolResult, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("get_ruleset: %v", err)
	}
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("get_ruleset: isError %v, %d content items", res.IsError, len(res.Content))
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_ruleset content is %T, want text", res.Content[0])
	}
	return text.Text
}

func qaRRRouteBody(t *testing.T, g *qaRRGateway) []byte {
	t.Helper()
	code, body, err := qaRRGet(t, g.direct+"/api/ruleset", g.token)
	if err != nil || code != http.StatusOK {
		t.Fatalf("GET /api/ruleset: %d, %v: %s", code, err, body)
	}
	return body
}

func qaRRPropertyCount(t *testing.T, body []byte) int {
	t.Helper()
	var doc struct {
		Properties []json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("route body: %v", err)
	}
	return len(doc.Properties)
}

// VTT-621
func TestQARRGetRulesetReturnsTheRouteBodyVerbatim(t *testing.T) {
	for _, dir := range qaRRRulesetDirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			g := qaRRServe(t, dir)
			cs := qaRRSession(t, g)
			res, err := qaRRCall(t, cs)
			got := qaRRText(t, res, err)
			want := qaRRRouteBody(t, g)
			if qaRRPropertyCount(t, want) == 0 {
				t.Fatalf("degenerate fixture: the route serves no properties")
			}
			if got != string(want) {
				t.Errorf("get_ruleset text differs from the route body\n got %q\nwant %q", got, want)
			}
		})
	}
}

// VTT-621
func TestQARRGetRulesetVerbatimWithNothingLoaded(t *testing.T) {
	g := qaRRServe(t, "")
	cs := qaRRSession(t, g)
	res, err := qaRRCall(t, cs)
	got := qaRRText(t, res, err)
	want := qaRRRouteBody(t, g)
	if got != string(want) {
		t.Errorf("get_ruleset text differs from the route body\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(got, `"properties":[]`) {
		t.Errorf("nothing loaded, yet properties is not []: %s", got)
	}
}

// VTT-621
func TestQARRGetRulesetAsksWithItsOwnToken(t *testing.T) {
	g := qaRRServe(t, qaRRRulesetDirs[0])
	cs := qaRRSession(t, g)
	before := len(g.qaRRAsked())
	res, err := qaRRCall(t, cs)
	_ = qaRRText(t, res, err)
	asked := g.qaRRAsked()[before:]
	if len(asked) == 0 {
		t.Fatalf("get_ruleset answered without asking GET /api/ruleset")
	}
	for _, a := range asked {
		if a != "GET Bearer "+g.token {
			t.Errorf("asked %q, want GET with the server's own bearer token", a)
		}
	}
}

// VTT-633
func TestQARRGetRulesetErrorsWhenTheGatewayRefuses(t *testing.T) {
	g := qaRRServe(t, qaRRRulesetDirs[0])
	cs := qaRRSession(t, g)
	want := string(qaRRRouteBody(t, g))
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		g.qaRRSetRefuse(status)
		res, err := qaRRCall(t, cs)
		switch {
		case err != nil:
			if !strings.Contains(err.Error(), "/api/ruleset") {
				t.Errorf("gateway answered %d, and the error names no /api/ruleset: %v", status, err)
			}
		case !res.IsError:
			t.Errorf("gateway answered %d, tool returned content: %v", status, res.Content)
		default:
			for _, c := range res.Content {
				if tc, ok := c.(*mcpsdk.TextContent); ok {
					t.Logf("gateway answered %d: isError, text %q", status, tc.Text)
				}
			}
		}
	}
	g.qaRRSetRefuse(0)
	res, err := qaRRCall(t, cs)
	if got := qaRRText(t, res, err); got != want {
		t.Errorf("after the refusals: get_ruleset text differs from the route body")
	}
}

func TestQARRGetRulesetIsListedAndTakesNoArguments(t *testing.T) {
	if !slices.Contains(mcppkg.GoRegisteredToolNames, "get_ruleset") {
		t.Errorf("GoRegisteredToolNames = %v, lacks get_ruleset", mcppkg.GoRegisteredToolNames)
	}
	if bytes.Contains(toolsJSON, []byte(`"get_ruleset"`)) {
		t.Errorf("get_ruleset appears in the contract manifest")
	}
	g := qaRRServe(t, qaRRRulesetDirs[0])
	cs := qaRRSession(t, g)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := cs.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var tool *mcpsdk.Tool
	for _, tl := range list.Tools {
		if tl.Name == "get_ruleset" {
			tool = tl
		}
	}
	if tool == nil {
		t.Fatalf("the session does not list get_ruleset")
	}
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("schema %s: %v", raw, err)
	}
	if len(schema.Properties) != 0 || len(schema.Required) != 0 {
		t.Errorf("get_ruleset takes arguments: %s", raw)
	}
}
