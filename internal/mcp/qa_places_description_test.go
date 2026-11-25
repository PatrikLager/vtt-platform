package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PatrikLager/vtt-platform/internal/mcp"
)

const qaprRulesetBody = `{"id":"qa-places","name":"QA Places",` +
	`"places":[{"key":"mike","name":"Satchel","description":"On the back.",` +
	`"forms":["zulu","yankee"],"copies":true,"states":["oscar","lima"]},` +
	`{"key":"kilo","name":"Belt","description":"At the waist.",` +
	`"forms":["zulu"],"copies":false,"states":[]}],` +
	`"sheet":[{"name":"Page one","sections":[{"name":"Carried",` +
	`"kind":"places","places":["kilo","mike"]}]}]}`

type qaprGateway struct {
	mu     sync.Mutex
	bearer []string
}

func (g *qaprGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/ruleset" {
		g.mu.Lock()
		g.bearer = append(g.bearer, r.Header.Get("Authorization"))
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qaprRulesetBody))
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()
	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}

func qaprSession(t *testing.T) (*mcpsdk.ClientSession, *qaprGateway) {
	t.Helper()
	gw := &qaprGateway{}
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	tools, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools",
		"tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := mcp.New(mcp.Config{
		WSURL:     "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws",
		Token:     "qapr-token",
		ToolsJSON: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Run(ctx, serverT)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qapr", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, gw
}

func qaprDescription(t *testing.T, session *mcpsdk.ClientSession) string {
	t.Helper()
	res, err := session.ListTools(context.Background(), &mcpsdk.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "get_ruleset" {
			return tool.Description
		}
	}
	t.Fatal("get_ruleset is not listed")
	return ""
}

var qaprSentenceEnd = regexp.MustCompile(`[.;]\s`)

func qaprSentences(desc string) []string {
	return qaprSentenceEnd.Split(desc, -1)
}

func qaprAnySentence(desc string, without string, words ...string) bool {
	for _, s := range qaprSentences(strings.ToLower(desc)) {
		all := without == "" || !strings.Contains(s, without)
		for _, w := range words {
			all = all && strings.Contains(s, w)
		}
		if all {
			return true
		}
	}
	return false
}

func qaprDescriptionFaults(desc string) []string {
	var faults []string
	if !qaprAnySentence(desc, "section", "answers", "`places`") {
		faults = append(faults, "has no sentence answering `places` itself")
	}
	if !qaprAnySentence(desc, "", "section", "`places`") {
		faults = append(faults, "has no sentence on what a section of places shows")
	}
	if !qaprAnySentence(desc, "", "form", "refuse") {
		faults = append(faults, "has no sentence refusing a key a form lists")
	}
	return faults
}

// VTT-1054, VTT-1014.
func TestQAPRGetRulesetSaysItAnswersThePlacesAndWhatTheirSectionShows(t *testing.T) {
	session, _ := qaprSession(t)
	desc := qaprDescription(t, session)
	for _, f := range qaprDescriptionFaults(desc) {
		t.Errorf("get_ruleset's description %s:\n%s", f, desc)
	}
}

// VTT-621, SPEC-012.
func TestQAPRGetRulesetAnswersThePlacesTheRouteServesVerbatim(t *testing.T) {
	session, gw := qaprSession(t)
	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "get_ruleset", Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("get_ruleset answers %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_ruleset answers %T", res.Content[0])
	}
	if text.Text != qaprRulesetBody {
		t.Errorf("get_ruleset answers\n%s\nwant\n%s", text.Text, qaprRulesetBody)
	}
	gw.mu.Lock()
	defer gw.mu.Unlock()
	if len(gw.bearer) != 1 || gw.bearer[0] != "Bearer qapr-token" {
		t.Errorf("get_ruleset read the route with %q", gw.bearer)
	}
}
