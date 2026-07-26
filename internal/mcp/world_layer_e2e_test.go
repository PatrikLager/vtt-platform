package mcp_test

// world_layer_e2e_test.go (world-layer sub-project 8, Task 3) is this
// package's own round-trip proof for add_narration/upsert_note, over the
// SAME fake-wire scaffolding server_test.go/read_tools_test.go already
// establish (newFakeServer/startSession/sendResult/sendFrame) — a
// deterministic, no-real-network companion to cmd/vtt/mcp_e2e_test.go's
// TestMCPWorldLayerRoundTrip (that one drives a REAL composeServer
// gateway; this one drives the scripted fake, exactly the same "two
// consumers of the same generic dispatch" split fold_test.go/harness's own
// client_test.go already establish elsewhere in this codebase). Both tools
// reach the fake wire through internal/mcp/tools.go's fully generic,
// protoreflect-driven dispatch — there is no per-command code anywhere in
// this package (tools.go's own doc comment) — so this test's payoff is
// proving that genericity actually covers the three new commands
// end-to-end, not just by name (TestListToolsReturnsSeventeenToolsIncluding
// ReadAndGuideTools, read_tools_test.go, already covers the name-only
// case).

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

// TestNarrationAndNoteRoundTripThroughFakeWire scripts the fake wire to
// both ack (CommandResult) and broadcast (Envelope) each of add_narration
// and upsert_note — the same "result + broadcast" shape a real gateway
// produces for every accepted command (server.go's handleCommand persists
// then the store notifies every subscriber, itself included) — then
// asserts the narration reaches get_events_since and the note reaches
// get_state's folded Notes map, exactly like the live-server variant in
// cmd/vtt/mcp_e2e_test.go.
func TestNarrationAndNoteRoundTripThroughFakeWire(t *testing.T) {
	const narrationText = "The party enters the ruined keep."
	const noteKey = "ruined-keep"
	const noteTitle = "Ruined Keep"
	const noteText = "Collapsed east wall; goblins nest within."

	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		switch c := cmd.GetCommand().(type) {
		case *vttv1.ClientCommand_AddNarration:
			sendResult(t, conn, cmd.GetRequestId(), true, "", 1)
			sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: &vttv1.Envelope{
				EventId: "ev-narration", Sequence: 1,
				Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
					Text: c.AddNarration.GetText(),
				}},
			}}})
		case *vttv1.ClientCommand_UpsertNote:
			sendResult(t, conn, cmd.GetRequestId(), true, "", 2)
			sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: &vttv1.Envelope{
				EventId: "ev-note", Sequence: 2,
				Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
					Key: c.UpsertNote.GetKey(), Title: c.UpsertNote.GetTitle(), Text: c.UpsertNote.GetText(),
				}},
			}}})
		default:
			t.Fatalf("fakeServer: unexpected command: %v", cmd)
		}
	})

	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	narrationSeq := mustCallToolOKMCP(t, cs, "add_narration", map[string]any{"text": narrationText})
	if narrationSeq != 1 {
		t.Fatalf("add_narration: result.Sequence = %d, want 1", narrationSeq)
	}
	noteSeq := mustCallToolOKMCP(t, cs, "upsert_note", map[string]any{
		"key": noteKey, "title": noteTitle, "text": noteText,
	})
	if noteSeq != 2 {
		t.Fatalf("upsert_note: result.Sequence = %d, want 2", noteSeq)
	}

	// --- note visible in get_state ---
	gotState := waitForHeadSequenceMCP(t, cs, noteSeq)
	notes, ok := gotState["Notes"].(map[string]any)
	if !ok {
		t.Fatalf(`get_state: "Notes" = %#v, want a JSON object`, gotState["Notes"])
	}
	note, ok := notes[noteKey].(map[string]any)
	if !ok {
		t.Fatalf("get_state: Notes[%q] = %#v, want a JSON object", noteKey, notes[noteKey])
	}
	if note["Title"] != noteTitle || note["Text"] != noteText {
		t.Fatalf("get_state: Notes[%q] = %#v, want Title=%q Text=%q", noteKey, note, noteTitle, noteText)
	}

	// --- narration visible in get_events_since ---
	assertNarrationInEventsSinceMCP(t, cs, narrationSeq, narrationText)
}

// --- local helpers (this file's own — deliberately not shared with
// server_test.go/read_tools_test.go's inline-per-test decode style, which
// this file follows for the same reason those files give as their own
// precedent: keeping each test's expectations visible at the call site) ---

// mustCallToolOKMCP calls name/args and asserts a plain ok=true
// CommandResult, returning its Sequence.
func mustCallToolOKMCP(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: CallTool: %v", name, err)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("%s: want text content, got %T", name, res.Content[0])
	}
	var result vttv1.CommandResult
	if err := protojson.Unmarshal([]byte(text.Text), &result); err != nil {
		t.Fatalf("%s: result not valid CommandResult protojson: %v", name, err)
	}
	if res.IsError || !result.GetOk() {
		t.Fatalf("%s: want ok=true, got IsError=%v result=%+v", name, res.IsError, &result)
	}
	return result.GetSequence()
}

// callGetStateMCP calls get_state and decodes it into a generic map — the
// dump contract's own Go-JSON shape (read_tools.go's marshalStateWithHead
// doc comment), not protojson.
func callGetStateMCP(t *testing.T, cs *mcpsdk.ClientSession) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_state", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("get_state: CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_state: want IsError=false, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_state: want text content, got %T", res.Content[0])
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text.Text), &m); err != nil {
		t.Fatalf("get_state: decode JSON: %v (body: %s)", err, text.Text)
	}
	return m
}

// waitForHeadSequenceMCP polls get_state until headSequence reaches
// atLeast — pump's drain of the fake wire's broadcast frames is a separate
// goroutine from the command call's own ack, so this can lag briefly
// (mirrors cmd/vtt/mcp_e2e_test.go's waitForMCPHeadSequence exactly, a
// deliberate parallel implementation for the reasons this file's own
// package doc comment gives).
func waitForHeadSequenceMCP(t *testing.T, cs *mcpsdk.ClientSession, atLeast int64) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		last = callGetStateMCP(t, cs)
		if hs, ok := last["headSequence"].(float64); ok && int64(hs) >= atLeast {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("get_state: headSequence never reached %d (last=%v)", atLeast, last)
	return last
}

// assertNarrationInEventsSinceMCP pages get_events_since from the
// beginning and asserts a NarrationAdded envelope at wantSeq carries
// wantText — each event is its own protojson encoding (unlike get_state).
func assertNarrationInEventsSinceMCP(t *testing.T, cs *mcpsdk.ClientSession, wantSeq int64, wantText string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_events_since",
		Arguments: map[string]any{"afterSequence": int64(0), "limit": 200},
	})
	if err != nil {
		t.Fatalf("get_events_since: CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_events_since: want IsError=false, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_events_since: want text content, got %T", res.Content[0])
	}
	var page struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal([]byte(text.Text), &page); err != nil {
		t.Fatalf("get_events_since: decode: %v (body: %s)", err, text.Text)
	}

	var found *vttv1.NarrationAdded
	var foundSeq int64
	for _, raw := range page.Events {
		var env vttv1.Envelope
		if err := protojson.Unmarshal(raw, &env); err != nil {
			t.Fatalf("get_events_since: envelope did not decode as protojson: %v (body: %s)", err, raw)
		}
		if na := env.GetNarrationAdded(); na != nil {
			found = na
			foundSeq = env.GetSequence()
			break
		}
	}
	if found == nil {
		t.Fatalf("get_events_since: no NarrationAdded event found among %d events", len(page.Events))
	}
	if foundSeq != wantSeq {
		t.Fatalf("get_events_since: NarrationAdded sequence = %d, want %d", foundSeq, wantSeq)
	}
	if found.Text != wantText {
		t.Fatalf("get_events_since: NarrationAdded.Text = %q, want %q", found.Text, wantText)
	}
}
