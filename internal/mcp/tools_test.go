package mcp_test

// Generic-dispatch behavioral RED (task-1-brief.md Step 2): list_tools
// exposes the command tools from the committed tools.json by name; a
// valid call wraps its JSON arguments as the protojson body of the matching
// ClientCommand oneof field and reaches the fake wire with a fresh
// request_id; an ok=false CommandResult surfaces as an MCP isError result
// carrying the CommandResult JSON (binding decision, task-1-brief.md).

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// wantCommandToolNames are the 15 oneof-field names in vttv1.ClientCommand's
// "command" oneof, restated here (not derived from the SDK under test) so
// this test pins the actual expected set rather than trivially re-deriving
// whatever tools.go happened to register. Grew from 7 to 9 with
// use_ability/remove_condition (ruleset-interpreter sub-project 5a, Task 1
// — contract's rules vocabulary), 9 to 12 with add_narration/upsert_note/
// delete_note (world-layer sub-project 8, P11 Task 1), 12 to 13 with
// load_adventure (adventure-format sub-project 9, P12 Task 1 — fix-wave F5
// sweep: this pin had silently stopped growing with the oneof itself,
// leaving load_adventure unpinned here even though it landed on this
// branch), and 13 to 15 with open_door/close_door (maps-as-geometry
// sub-project, Task 1 — contract's terrain and door vocabulary; not yet
// authorized or convertible by the gateway, which is Task 6's job, but
// already a real MCP tool per spec §5: "open_door and close_door appear as
// MCP tools automatically, the way load_adventure did").
var wantCommandToolNames = []string{
	"move_token", "create_scene", "add_actor", "place_token",
	"start_session", "end_session", "retract_events",
	"use_ability", "remove_condition",
	"add_narration", "upsert_note", "delete_note",
	"load_adventure",
	"grant_actor_control", "revoke_actor_control",
	"promote_participant",
	"set_join_door", "rotate_join_link",
	"open_door", "close_door",
}

// This test scopes itself to "the command tools are present by name" — it
// deliberately does NOT assert the total tool count, since P7 Task 2
// (read_tools.go) and Task 6 register four more (get_state,
// get_events_since, get_ruleset_guide, get_adventure_guide) into the SAME
// tool table; TestListToolsReturnsEveryCommandAndReadTool
// (read_tools_test.go) owns the total-count assertion for the full,
// current contract.
func TestListToolsReturnsAllCommandToolsByName(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	got := make(map[string]bool, len(res.Tools))
	for _, tl := range res.Tools {
		got[tl.Name] = true
	}
	for _, name := range wantCommandToolNames {
		if !got[name] {
			t.Errorf("list_tools missing %q", name)
		}
	}
}

// TestCallToolDispatchesGenericCommandWithFreshRequestID is the payoff test:
// calling move_token with JSON args reaches the fake wire as a well-formed
// ClientCommand whose MoveToken oneof field carries exactly those args
// (protojson-shaped: tokenId/to.x/to.y/reason), with a non-empty request_id
// that differs across calls (harness.Client.SendCommand's per-call
// assignment — proof this handler never reuses one *vttv1.ClientCommand
// across calls, the no-shared-cmd rule). The tool result carries the
// sequence the fake wire scripted back.
func TestCallToolDispatchesGenericCommandWithFreshRequestID(t *testing.T) {
	type captured struct {
		cmd *vttv1.ClientCommand
	}
	seen := make(chan captured, 4)
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		seen <- captured{cmd: cmd}
		sendResult(t, conn, cmd.GetRequestId(), true, "", 42)
	})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	callArgs := map[string]any{
		"tokenId": "tok-1",
		"to":      map[string]any{"x": 3, "y": 4},
		"reason":  "testing dispatch",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "move_token", Arguments: callArgs})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("want IsError=false for an ok result, got %+v", res)
	}

	var cap1 captured
	select {
	case cap1 = <-seen:
	case <-time.After(3 * time.Second):
		t.Fatal("fake wire never received the dispatched command")
	}

	mv := cap1.cmd.GetMoveToken()
	if mv == nil {
		t.Fatalf("want ClientCommand.MoveToken populated, got %+v", cap1.cmd)
	}
	if mv.GetTokenId() != "tok-1" || mv.GetTo().GetX() != 3 || mv.GetTo().GetY() != 4 || mv.GetReason() != "testing dispatch" {
		t.Fatalf("MoveToken args mismatch: %+v", mv)
	}
	if cap1.cmd.GetRequestId() == "" {
		t.Fatal("want a fresh (non-empty) request_id on the dispatched command")
	}

	// The tool result must carry the CommandResult protojson, sequence
	// included (read-your-writes per the spec).
	textContent, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	var result vttv1.CommandResult
	if err := protojson.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("tool result content is not valid CommandResult protojson: %v\n%s", err, textContent.Text)
	}
	if !result.GetOk() || result.GetSequence() != 42 {
		t.Fatalf("tool result CommandResult = %+v, want ok=true sequence=42", &result)
	}
	if result.GetRequestId() != cap1.cmd.GetRequestId() {
		t.Fatalf("tool result request_id = %q, want %q (the dispatched command's own id)", result.GetRequestId(), cap1.cmd.GetRequestId())
	}

	// A second call must carry a DIFFERENT request_id — proof each call
	// builds its own fresh *vttv1.ClientCommand rather than reusing one
	// (harness.Client.SendCommand's documented no-shared-cmd rule).
	if _, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "move_token", Arguments: callArgs}); err != nil {
		t.Fatalf("second CallTool: %v", err)
	}
	var cap2 captured
	select {
	case cap2 = <-seen:
	case <-time.After(3 * time.Second):
		t.Fatal("fake wire never received the second dispatched command")
	}
	if cap2.cmd.GetRequestId() == cap1.cmd.GetRequestId() {
		t.Fatalf("want distinct request_ids across calls, both were %q", cap1.cmd.GetRequestId())
	}
}

// TestCallToolOkFalseIsMCPError covers the binding ok=false decision: a
// CommandResult with ok=false must produce an MCP tool result with
// IsError=true, carrying the CommandResult JSON (error string included) as
// content — a structured failure the LLM can see and react to, NOT a
// protocol-level MCP error (that channel is reserved for infrastructure
// failures: unknown tool, disconnected wire).
func TestCallToolOkFalseIsMCPError(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		sendResult(t, conn, cmd.GetRequestId(), false, "gateway: not your turn", 0)
	})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "end_session",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("want IsError=true for an ok=false CommandResult, got %+v", res)
	}

	textContent, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	var result vttv1.CommandResult
	if err := protojson.Unmarshal([]byte(textContent.Text), &result); err != nil {
		t.Fatalf("error tool result content is not valid CommandResult protojson: %v\n%s", err, textContent.Text)
	}
	if result.GetOk() {
		t.Fatalf("want ok=false in the surfaced CommandResult, got %+v", &result)
	}
	if result.GetError() != "gateway: not your turn" {
		t.Fatalf("result.Error = %q, want the gateway's error string", result.GetError())
	}
	// Sanity: the content is genuinely structured (round-trips through
	// encoding/json too), not an ad hoc string the LLM would have to parse
	// by convention.
	var generic map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &generic); err != nil {
		t.Fatalf("tool result content is not valid JSON: %v", err)
	}
}
