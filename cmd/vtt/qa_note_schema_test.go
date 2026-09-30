package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	vttmcp "github.com/PatrikLager/vtt-platform/internal/mcp"
)

const qaNSWait = 20 * time.Second

var qaNSManifests = []string{
	"tools.json",
	filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"),
}

type qaNSTool struct {
	Name        string          `json:"name"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type qaNSSchema struct {
	Required   []string `json:"required"`
	Properties map[string]struct {
		Enum []string `json:"enum"`
	} `json:"properties"`
}

func qaNSOfferedNames() []string {
	var out []string
	for n, name := range vttv1.NoteVisibility_name {
		if n != int32(vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func qaNSUpsertSchema(t *testing.T, tools []qaNSTool, where string) qaNSSchema {
	t.Helper()
	for _, tool := range tools {
		if tool.Name != "upsert_note" {
			continue
		}
		var s qaNSSchema
		if err := json.Unmarshal(tool.InputSchema, &s); err != nil {
			t.Fatalf("%s: upsert_note inputSchema: %v", where, err)
		}
		return s
	}
	t.Fatalf("%s: no upsert_note tool", where)
	return qaNSSchema{}
}

func qaNSAssertVisibilityRequired(t *testing.T, s qaNSSchema, where string) {
	t.Helper()
	found := false
	for _, r := range s.Required {
		if r == "visibility" {
			found = true
		}
	}
	if !found {
		t.Errorf("%s: upsert_note required = %v, want it to list visibility", where, s.Required)
	}
	prop, ok := s.Properties["visibility"]
	if !ok {
		t.Fatalf("%s: upsert_note has no visibility property", where)
	}
	got := append([]string(nil), prop.Enum...)
	sort.Strings(got)
	if want := qaNSOfferedNames(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s: visibility enum = %v, want exactly the contract's stated values %v", where, got, want)
	}
}

// VTT-232
func TestQAEveryCommittedManifestListsUpsertNoteVisibilityAsRequired(t *testing.T) {
	for _, path := range qaNSManifests {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var tools []qaNSTool
		if err := json.Unmarshal(raw, &tools); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		qaNSAssertVisibilityRequired(t, qaNSUpsertSchema(t, tools, path), path)
	}
}

type qaNSAgentTable struct {
	c       *campaign.Campaign
	session *mcpsdk.ClientSession
}

func qaNSStartAgent(t *testing.T) *qaNSAgentTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = c.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	ts := httptest.NewServer(gateway.New(c, ids).Handler())
	tok, _, err := ids.CreateInvite("agent", identity.RoleAgent)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	toolsJSON, err := os.ReadFile("tools.json")
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	srv, err := vttmcp.New(vttmcp.Config{
		WSURL:     "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws",
		Token:     tok,
		ToolsJSON: toolsJSON,
	})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverSide, clientSide := mcpsdk.NewInMemoryTransports()
	ran := make(chan error, 1)
	go func() { ran <- srv.Run(ctx, serverSide) }()
	connectCtx, connectCancel := context.WithTimeout(ctx, qaNSWait)
	session, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa", Version: "0"}, nil).Connect(connectCtx, clientSide, nil)
	connectCancel()
	if err != nil {
		cancel()
		t.Fatalf("MCP client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		select {
		case <-ran:
		case <-time.After(qaNSWait):
			t.Errorf("mcp Server.Run did not return after its context ended")
		}
		ts.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qaNSAgentTable{c: c, session: session}
}

func (a *qaNSAgentTable) head(t *testing.T) int64 {
	t.Helper()
	_, unsubscribe, head, err := a.c.Subscribe(0, 4096)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	unsubscribe()
	return head
}

func (a *qaNSAgentTable) call(t *testing.T, args map[string]any) (refused bool, text string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qaNSWait)
	defer cancel()
	res, err := a.session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "upsert_note", Arguments: args})
	if err != nil {
		t.Fatalf("upsert_note: MCP protocol error rather than a tool result: %v", err)
	}
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return res.IsError, strings.Join(parts, "\n")
}

// VTT-232
func TestQAAnMCPClientIsToldUpsertNoteVisibilityIsRequired(t *testing.T) {
	a := qaNSStartAgent(t)
	ctx, cancel := context.WithTimeout(context.Background(), qaNSWait)
	defer cancel()
	listed, err := a.session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var tools []qaNSTool
	for _, tool := range listed.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", tool.Name, err)
		}
		tools = append(tools, qaNSTool{Name: tool.Name, InputSchema: schema})
	}
	qaNSAssertVisibilityRequired(t, qaNSUpsertSchema(t, tools, "ListTools"), "ListTools")
}

// VTT-231
func TestQAAnAgentsUpsertNoteThroughMCPWithoutVisibilityIsRefused(t *testing.T) {
	a := qaNSStartAgent(t)
	before := a.head(t)
	refused, text := a.call(t, map[string]any{"key": "mcp-refused", "title": "t", "text": "x"})
	t.Logf("agent's upsert_note without visibility: refused=%v text=%q", refused, text)
	if !refused {
		t.Fatalf("an agent's upsert_note with no visibility was accepted: %q", text)
	}
	if !strings.Contains(strings.ToLower(text), "visibility") {
		t.Errorf("refusal %q does not name visibility", text)
	}
	if after := a.head(t); after != before {
		t.Fatalf("log head moved from %d to %d on a refused upsert", before, after)
	}
	if _, ok := a.c.State().Notes["mcp-refused"]; ok {
		t.Fatalf("the fold holds a note whose upsert was refused")
	}
}

// VTT-231
func TestQAAnAgentsUpsertNoteThroughMCPStatingAnOfferedVisibilityIsAccepted(t *testing.T) {
	a := qaNSStartAgent(t)
	for i, name := range qaNSOfferedNames() {
		key := "mcp-" + strings.ToLower(name)
		before := a.head(t)
		refused, text := a.call(t, map[string]any{"key": key, "title": "t", "text": "x", "visibility": name})
		if refused {
			t.Fatalf("an agent's upsert_note with %s was refused: %q", name, text)
		}
		deadline := time.Now().Add(qaNSWait)
		for a.head(t) == before && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if after := a.head(t); after != before+1 {
			t.Fatalf("value %d (%s): log head %d after an accepted upsert, want %d", i, name, after, before+1)
		}
		note, ok := a.c.State().Notes[key]
		if !ok {
			t.Fatalf("%s: accepted note is not in the fold", name)
		}
		if note.Visibility.String() != name {
			t.Errorf("fold records %s, want %s", note.Visibility, name)
		}
	}
}
