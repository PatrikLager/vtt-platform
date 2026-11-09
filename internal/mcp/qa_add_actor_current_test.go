package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mcp"
)

type qaFakeGateway struct {
	mu    sync.Mutex
	dials int
	cmds  []*vttv1.ClientCommand
}

func (g *qaFakeGateway) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()
	ctx := r.Context()
	head, err := protojson.Marshal(&vttv1.ServerFrame{
		Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{}},
	})
	if err != nil || conn.Write(ctx, websocket.MessageText, head) != nil {
		return
	}
	g.mu.Lock()
	g.dials++
	g.mu.Unlock()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		cmd := &vttv1.ClientCommand{}
		if err := protojson.Unmarshal(data, cmd); err != nil {
			return
		}
		g.mu.Lock()
		g.cmds = append(g.cmds, cmd)
		g.mu.Unlock()
		out, err := protojson.Marshal(&vttv1.ServerFrame{
			Frame: &vttv1.ServerFrame_Result{Result: &vttv1.CommandResult{
				RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1,
			}},
		})
		if err != nil || conn.Write(ctx, websocket.MessageText, out) != nil {
			return
		}
	}
}

func (g *qaFakeGateway) dialed() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.dials
}

func (g *qaFakeGateway) addActors() []*vttv1.Actor {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []*vttv1.Actor
	for _, c := range g.cmds {
		if a := c.GetAddActor(); a != nil {
			out = append(out, a.GetActor())
		}
	}
	return out
}

func (g *qaFakeGateway) sent() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.cmds)
}

const qaRulesetJSON = `{"id":"qa","name":"QA","abilities":[],"conditions":[],` +
	`"resources":[],"properties":[{"key":"qa_pool","type":"resource",` +
	`"name":"QA pool","description":"A resource."}]}`

type qaAgent struct {
	gw   *qaFakeGateway
	sess *mcpsdk.ClientSession
}

func qaToolsJSON(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func qaSeatAgent(t *testing.T, withRuleset bool) *qaAgent {
	t.Helper()
	gw := &qaFakeGateway{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.serveWS)
	if withRuleset {
		mux.HandleFunc("/api/ruleset", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(qaRulesetJSON))
		})
	}
	hs := httptest.NewServer(mux)
	cfg := mcp.Config{
		WSURL:     "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws",
		Token:     "qa-token",
		ToolsJSON: qaToolsJSON(t),
	}
	if withRuleset {
		cfg.RulesetGuide = "QA guide."
	}
	srv, err := mcp.New(cfg)
	if err != nil {
		hs.Close()
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Run(ctx, serverT)
	}()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa", Version: "0"}, nil)
	sess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		hs.Close()
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() {
		_ = sess.Close()
		cancel()
		<-done
		hs.Close()
	})
	deadline := time.Now().Add(10 * time.Second)
	for gw.dialed() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the MCP server never dialed the fake gateway")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return &qaAgent{gw: gw, sess: sess}
}

func (a *qaAgent) call(t *testing.T, args string) (string, bool) {
	t.Helper()
	var arguments map[string]any
	if err := json.Unmarshal([]byte(args), &arguments); err != nil {
		t.Fatalf("bad test arguments %s: %v", args, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := a.sess.CallTool(ctx, &mcpsdk.CallToolParams{Name: "add_actor", Arguments: arguments})
	if err != nil {
		return err.Error(), true
	}
	var text []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			text = append(text, tc.Text)
		}
	}
	return strings.Join(text, "\n"), res.IsError
}

func (a *qaAgent) requireRefusedUnsent(t *testing.T, args, actorID, key string) {
	t.Helper()
	text, isErr := a.call(t, args)
	t.Logf("answer: %s", text)
	if !isErr {
		t.Fatalf("add_actor %s answered %q without an error; want it refused", args, text)
	}
	for _, want := range []string{actorID, key} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal %q does not name %q", text, want)
		}
	}
	if n := len(a.gw.addActors()); n != 0 {
		t.Errorf("the gateway was sent %d add_actor commands; want none", n)
	}
	if n := a.gw.sent(); n != 0 {
		t.Errorf("the gateway was sent %d commands; want none", n)
	}
}

func (a *qaAgent) requireSent(t *testing.T, args, actorID string) *vttv1.Actor {
	t.Helper()
	before := len(a.gw.addActors())
	text, isErr := a.call(t, args)
	if isErr {
		t.Fatalf("add_actor %s was refused: %q", args, text)
	}
	sent := a.gw.addActors()
	if len(sent) != before+1 {
		t.Fatalf("the gateway was sent %d add_actor commands; want %d", len(sent), before+1)
	}
	got := sent[len(sent)-1]
	if got.GetActorId() != actorID {
		t.Errorf("sent actor id = %q; want %q", got.GetActorId(), actorID)
	}
	return got
}

// VTT-671
func TestQAMcpACurrentWrittenNullIsRefusedBeforeAnyCommand(t *testing.T) {
	for _, withRuleset := range []bool{false, true} {
		a := qaSeatAgent(t, withRuleset)
		a.requireRefusedUnsent(t, `{"actor":{"actorId":"qa-mcp-null","kind":"ACTOR_KIND_NON_PARTY",
			"resources":{"qa_pool":{"current":null,"max":5}}}}`, "qa-mcp-null", "qa_pool")
	}
}

// VTT-671
func TestQAMcpACurrentLeftOutIsRefusedBeforeAnyCommand(t *testing.T) {
	for _, withRuleset := range []bool{false, true} {
		a := qaSeatAgent(t, withRuleset)
		a.requireRefusedUnsent(t, `{"actor":{"actorId":"qa-mcp-out","kind":"ACTOR_KIND_NON_PARTY",
			"resources":{"qa_pool":{"max":5}}}}`, "qa-mcp-out", "qa_pool")
	}
}

// VTT-671
func TestQAMcpAnEmptyResourceIsRefusedBeforeAnyCommand(t *testing.T) {
	for _, withRuleset := range []bool{false, true} {
		a := qaSeatAgent(t, withRuleset)
		a.requireRefusedUnsent(t, `{"actor":{"actorId":"qa-mcp-empty","kind":"ACTOR_KIND_PARTY_MEMBER",
			"resources":{"qa_pool":{}}}}`, "qa-mcp-empty", "qa_pool")
	}
}

// VTT-671
func TestQAMcpTheValuelessResourceIsNamedAmongValuedOnes(t *testing.T) {
	a := qaSeatAgent(t, false)
	a.requireRefusedUnsent(t, `{"actor":{"actorId":"qa-mcp-mixed","kind":"ACTOR_KIND_NON_PARTY",
		"attributes":{"qa_num":2},
		"resources":{"qa_full":{"current":3,"max":4},"qa_gap":{"max":4},"qa_zero":{"current":0}}}}`,
		"qa-mcp-mixed", "qa_gap")
}

// VTT-671
func TestQAMcpAValuelessCurrentIsRefusedWhateverTheKind(t *testing.T) {
	a := qaSeatAgent(t, false)
	a.requireRefusedUnsent(t, `{"actor":{"actorId":"qa-mcp-nokind",
		"resources":{"qa_pool":{"current":null}}}}`, "qa-mcp-nokind", "qa_pool")
}

// VTT-671
func TestQAMcpACurrentOfZeroIsSent(t *testing.T) {
	for _, withRuleset := range []bool{false, true} {
		a := qaSeatAgent(t, withRuleset)
		got := a.requireSent(t, `{"actor":{"actorId":"qa-mcp-zero","kind":"ACTOR_KIND_NON_PARTY",
			"resources":{"qa_pool":{"current":0,"max":0},"qa_bare":{"current":0},"qa_some":{"current":3}}}}`,
			"qa-mcp-zero")
		for _, k := range []string{"qa_pool", "qa_bare", "qa_some"} {
			if _, ok := got.GetResources()[k]; !ok {
				t.Errorf("sent resources = %v; want %q among them", got.GetResources(), k)
			}
		}
		if c := got.GetResources()["qa_some"].GetCurrent(); c != 3 {
			t.Errorf("sent qa_some current = %d; want 3", c)
		}
	}
}

// VTT-674
func TestQAMcpTheServedSchemaRequiresACurrentAndNotAMax(t *testing.T) {
	a := qaSeatAgent(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	list, err := a.sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var schema any
	for _, tool := range list.Tools {
		if tool.Name == "add_actor" {
			schema = tool.InputSchema
		}
	}
	if schema == nil {
		t.Fatal("add_actor is not served")
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	qaRequireCurrentRequiredNotMax(t, raw)
}

func qaRequireCurrentRequiredNotMax(t *testing.T, inputSchema []byte) {
	t.Helper()
	var s struct {
		Properties struct {
			Actor struct {
				Properties struct {
					Resources struct {
						AdditionalProperties struct {
							Properties map[string]json.RawMessage `json:"properties"`
							Required   []string                   `json:"required"`
						} `json:"additionalProperties"`
					} `json:"resources"`
				} `json:"properties"`
			} `json:"actor"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(inputSchema, &s); err != nil {
		t.Fatalf("schema does not decode: %v", err)
	}
	res := s.Properties.Actor.Properties.Resources.AdditionalProperties
	for _, k := range []string{"current", "max"} {
		if _, ok := res.Properties[k]; !ok {
			t.Errorf("a resource's schema has no %q property: %v", k, res.Properties)
		}
	}
	required := map[string]bool{}
	for _, k := range res.Required {
		required[k] = true
	}
	if !required["current"] {
		t.Errorf("a resource's required = %v; want current among them", res.Required)
	}
	if required["max"] {
		t.Errorf("a resource's required = %v; want max not among them", res.Required)
	}
}

// VTT-671 VTT-679
func TestQAMcpAMaxWrittenNullIsSentAsNoMaximum(t *testing.T) {
	a := qaSeatAgent(t, false)
	got := a.requireSent(t, `{"actor":{"actorId":"qa-mcp-nomax","kind":"ACTOR_KIND_NON_PARTY",
		"resources":{"qa_pool":{"current":2,"max":null}}}}`, "qa-mcp-nomax")
	if r := got.GetResources()["qa_pool"]; r.GetCurrent() != 2 || r.GetMax() != 0 {
		t.Errorf("sent qa_pool = %v; want current 2 and no maximum", r)
	}
}
