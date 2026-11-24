package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PatrikLager/vtt-platform/internal/mcp"
)

const qacrToken = "qacr-agent-token"

// Keep the keys out of byte order and the escape unexpanded: a tool that
// re-encodes the body passes a canonical one unchanged (VTT-621).
const qacrRouteBody = `{"name":"Fixture","id":"fixture","catalogue":[{"values":{"grit":-3,"brawn":0},"id":"bravo","name":"Quill","description":"An entry \u00e9","form":"zulu"}],"forms":[{"properties":["grit","brawn"],"key":"zulu","name":"Ledger","description":"A form"}],"abilities":[]}`

type qacrGateway struct {
	wsURL    string
	rulesets atomic.Int32
}

func qacrFakeGateway(t *testing.T) *qacrGateway {
	t.Helper()
	g := &qacrGateway{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ruleset", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+qacrToken {
			http.Error(w, "gateway: unauthorized", http.StatusUnauthorized)
			return
		}
		g.rulesets.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qacrRouteBody))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			if _, _, err := c.Read(r.Context()); err != nil {
				return
			}
		}
	})
	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	g.wsURL = "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws"
	return g
}

func qacrSession(t *testing.T, g *qacrGateway) *mcpsdk.ClientSession {
	t.Helper()
	toolsJSON, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := mcp.New(mcp.Config{WSURL: g.wsURL, Token: qacrToken, ToolsJSON: toolsJSON})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	serverSide, clientSide := mcpsdk.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, serverSide) }()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qacr", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientSide, nil)
	if err != nil {
		cancel()
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		cancel()
		<-done
	})
	return cs
}

func qacrDescription(t *testing.T, cs *mcpsdk.ClientSession) string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "get_ruleset" {
			return tool.Description
		}
	}
	t.Fatal("no get_ruleset tool listed")
	return ""
}

var qacrSentenceEnd = regexp.MustCompile(`\.\s+`)

// Keep the claims bound to the sentences naming forms or the catalogue: the
// description already says "every role" and "[]" of other fields (VTT-1006).
func qacrFormsAndCatalogueSentences(desc string) string {
	var kept []string
	for _, s := range qacrSentenceEnd.Split(desc, -1) {
		l := strings.ToLower(s)
		if strings.Contains(l, "forms") || strings.Contains(l, "catalogue") {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, " | ")
}

func TestQACRGetRulesetSaysItAnswersEveryRoleTheFormsAndTheCatalogue(t *testing.T) {
	desc := qacrDescription(t, qacrSession(t, qacrFakeGateway(t)))
	scope := strings.ToLower(qacrFormsAndCatalogueSentences(desc))
	// VTT-1006: the forms, the catalogue, to every role, each entry's values after what
	// it extends, and [] when none.
	for _, claim := range []string{"forms", "catalogue", "every role", "values", "extend", "[]"} {
		if !strings.Contains(scope, claim) {
			t.Errorf("the sentences naming forms or the catalogue do not say %q:\n%s\n--- whole description ---\n%s", claim, scope, desc)
		}
	}
}

func TestQACRGetRulesetReturnsTheRoutesBodyVerbatim(t *testing.T) {
	g := qacrFakeGateway(t)
	cs := qacrSession(t, g)
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "get_ruleset", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_ruleset answered an error: %+v", res.Content)
	}
	if g.rulesets.Load() == 0 {
		t.Fatal("get_ruleset did not read /api/ruleset with its token")
	}
	if len(res.Content) != 1 {
		t.Fatalf("%d content items; want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content is %T; want text", res.Content[0])
	}
	// VTT-621: a 200 JSON body, forms and catalogue included, is the tool's text byte
	// for byte.
	if text.Text != qacrRouteBody {
		t.Errorf("get_ruleset text:\n%s\nwant the route body:\n%s", text.Text, qacrRouteBody)
	}
}
