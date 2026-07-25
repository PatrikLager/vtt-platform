package mcp_test

// Read-tools behavioral RED (task-2-brief.md Step 1): get_state folds
// Server's own accumulated history (never a second connection) and reports
// headSequence; get_events_since paginates that same history with correct
// `more`-flag boundary semantics and returns protojson envelopes (sequence
// as a STRING inside each one — the convention divergence the tool's own
// Description documents); a fold-with-retraction case (mirroring
// internal/harness/fold_test.go's own parity case) proves get_state reflects
// retraction, not just a naive apply-everything fold; list_tools reports 9.
//
// canned/seedEvents below are a parallel, not shared, implementation of
// internal/harness/fold_test.go's foldEnv helper (unexported in package
// harness_test, unreachable from here) — the same deliberate-duplication
// precedent server_test.go's fake wire itself already sets for this
// package.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// --- canned envelope construction ------------------------------------------

// canned builds a *vttv1.Envelope for one of the event variants this file
// seeds, mirroring internal/harness/fold_test.go's foldEnv switch shape.
func canned(seq int64, id string, payload any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: id, Sequence: seq, SessionId: "s1"}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.EventsRetracted:
		e.Payload = &vttv1.Envelope_EventsRetracted{EventsRetracted: p}
	}
	return e
}

// seedEvents pushes each envelope onto conn as a ServerFrame_Event, in
// order — the fake wire's proactive-push path (distinct from onCommand,
// which only fires for inbound ClientCommands).
func seedEvents(t *testing.T, conn *websocket.Conn, envs ...*vttv1.Envelope) {
	t.Helper()
	for _, env := range envs {
		sendFrame(t, conn, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: env}})
	}
}

// basicChain returns the SessionStarted/SceneCreated/ActorAdded/TokenPlaced/
// TokenMoved chain (sequences 1-5) fold_test.go's own parity test uses,
// placing tok-1 at (2,2) then moving it to (9,9).
func basicChain() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}),
		canned(2, "ev-scene", &vttv1.SceneCreated{SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10}),
		canned(3, "ev-actor", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-1", Name: "Ursus"}}),
		canned(4, "ev-place", &vttv1.TokenPlaced{
			TokenId: "tok-1", SceneId: "scn-1", ActorId: "act-1", Position: &vttv1.GridPosition{X: 2, Y: 2},
		}),
		canned(5, "ev-move", &vttv1.TokenMoved{
			TokenId: "tok-1", SceneId: "scn-1", From: &vttv1.GridPosition{X: 2, Y: 2}, To: &vttv1.GridPosition{X: 9, Y: 9},
		}),
	}
}

// --- response shapes for decoding tool output ------------------------------

type tokenShape struct {
	ID, SceneID, ActorID string
	X, Y                 int32
}

// getStateShape mirrors internal/engine.State's exported field names for
// the sub-shape these tests check, the same pattern cmd/vtt/client_e2e_
// test.go's dumpStateShape uses for `vtt state dump` (get_state is the
// SAME dump contract, per task-2-brief.md).
type getStateShape struct {
	Tokens       map[string]tokenShape
	HeadSequence int64 `json:"headSequence"`
}

type eventsSinceShape struct {
	Events        []json.RawMessage `json:"events"`
	HeadSequence  int64             `json:"headSequence"`
	More          bool              `json:"more"`
	WireConnected bool              `json:"wireConnected"`
}

// --- call helpers ------------------------------------------------------------

func callGetState(t *testing.T, cs *mcpsdk.ClientSession) (getStateShape, *mcpsdk.CallToolResult) {
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
	var st getStateShape
	if err := json.Unmarshal([]byte(text.Text), &st); err != nil {
		t.Fatalf("get_state: response did not decode: %v (body: %s)", err, text.Text)
	}
	return st, res
}

// waitForHeadSequence polls get_state until headSequence reaches atLeast or
// the deadline elapses. Seeding envelopes onto the fake wire (seedEvents)
// only queues them for delivery — pump's drain (websocket read -> harness
// Client.Events() -> pump goroutine -> recordEvent) is asynchronous from
// the test's perspective, so tests must wait for that hand-off to land
// before asserting on accumulated state, the same polling-with-deadline
// shape server_test.go's TestCallToolWhileDisconnectedReturnsCleanError
// already uses for an analogous async-goroutine-state wait.
func waitForHeadSequence(t *testing.T, cs *mcpsdk.ClientSession, atLeast int64) getStateShape {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last getStateShape
	for time.Now().Before(deadline) {
		last, _ = callGetState(t, cs)
		if last.HeadSequence >= atLeast {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("get_state: headSequence never reached %d within deadline (last=%d)", atLeast, last.HeadSequence)
	return last
}

func callGetEventsSince(t *testing.T, cs *mcpsdk.ClientSession, args map[string]any) eventsSinceShape {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_events_since", Arguments: args})
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
	var out eventsSinceShape
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("get_events_since: response did not decode: %v (body: %s)", err, text.Text)
	}
	return out
}

// eventSequences decodes each envelope's "sequence" field into a Go string
// — which only succeeds if the underlying JSON value IS a string (protojson
// convention: encoding/json errors unmarshaling a bare JSON number into a
// Go string field), so a successful decode here is itself part of the
// wire-convention proof, not just a convenience extraction.
func eventSequences(t *testing.T, raws []json.RawMessage) []string {
	t.Helper()
	seqs := make([]string, len(raws))
	for i, raw := range raws {
		var env struct {
			Sequence string `json:"sequence"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("decode envelope[%d].sequence as a JSON string: %v (body: %s)", i, err, raw)
		}
		seqs[i] = env.Sequence
	}
	return seqs
}

// --- tests -------------------------------------------------------------------

// TestGetStateReturnsFoldedTokenPositionAndHeadSequence covers the plain
// fold case: session/scene/actor/place/move seeded, get_state must return
// the moved-to token position and headSequence == the last seeded sequence.
func TestGetStateReturnsFoldedTokenPositionAndHeadSequence(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	seedEvents(t, conn, basicChain()...)

	st := waitForHeadSequence(t, cs, 5)
	tok, ok := st.Tokens["tok-1"]
	if !ok {
		t.Fatalf("get_state: Tokens[\"tok-1\"] missing: %+v", st)
	}
	if tok.X != 9 || tok.Y != 9 {
		t.Fatalf("get_state: tok-1 position = (%d,%d), want (9,9) — the moved-to position", tok.X, tok.Y)
	}
	if tok.SceneID != "scn-1" || tok.ActorID != "act-1" {
		t.Fatalf("get_state: tok-1 = %+v, want SceneID=scn-1 ActorID=act-1", tok)
	}
	if st.HeadSequence != 5 {
		t.Fatalf("get_state: headSequence = %d, want 5", st.HeadSequence)
	}
}

// TestGetStateReflectsFoldWithRetraction reuses the fold-parity shape of
// internal/harness/fold_test.go's TestFoldParityAgainstIndependentEngine
// ApplyReplay: retracting the TokenMoved must make get_state show tok-1 at
// its PLACED position (2,2), not the moved-to one, proving get_state's fold
// is retraction-aware rather than a naive full-history apply.
func TestGetStateReflectsFoldWithRetraction(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	events := basicChain()
	events = append(events, canned(6, "ev-retract", &vttv1.EventsRetracted{FromSequence: 5, ToSequence: 5, Reason: "undo the move"}))
	seedEvents(t, conn, events...)

	st := waitForHeadSequence(t, cs, 6)
	tok, ok := st.Tokens["tok-1"]
	if !ok {
		t.Fatalf("get_state: Tokens[\"tok-1\"] missing: %+v", st)
	}
	if tok.X != 2 || tok.Y != 2 {
		t.Fatalf("get_state: tok-1 position = (%d,%d), want (2,2) — the retracted move must not have applied", tok.X, tok.Y)
	}
	if st.HeadSequence != 6 {
		t.Fatalf("get_state: headSequence = %d, want 6 (the retraction marker itself still advances it)", st.HeadSequence)
	}
}

// TestListToolsReturnsElevenToolsIncludingReadTools covers the top-level
// contract: get_state and get_events_since land in the SAME tool table as
// the nine generic command tools (wantCommandToolNames — grew from 7 to 9
// with use_ability/remove_condition, ruleset-interpreter sub-project 5a
// Task 1), bringing list_tools to exactly 11.
func TestListToolsReturnsElevenToolsIncludingReadTools(t *testing.T) {
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
	want := append(append([]string{}, wantCommandToolNames...), "get_state", "get_events_since")
	if len(got) != len(want) {
		t.Fatalf("list_tools returned %d distinct names, want %d: %v", len(got), len(want), got)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("list_tools missing %q", name)
		}
	}
}

// TestGetEventsSincePaginationWalksWithCorrectMoreFlag seeds a five-event
// history (sequences 1-5) and walks get_events_since across it, pinning the
// `more` flag at every named boundary from task-2-brief.md Step 1:
// exactly-limit (remaining count == limit), beyond-end (limit exceeds what
// remains), and afterSequence==head (nothing left at all).
func TestGetEventsSincePaginationWalksWithCorrectMoreFlag(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	events := []*vttv1.Envelope{
		canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}),
		canned(2, "ev-a", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-a", Name: "A"}}),
		canned(3, "ev-b", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-b", Name: "B"}}),
		canned(4, "ev-c", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-c", Name: "C"}}),
		canned(5, "ev-d", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-d", Name: "D"}}),
	}
	seedEvents(t, conn, events...)
	waitForHeadSequence(t, cs, 5)

	// Page 1: 2 of 5 consumed, 3 remain beyond this page -> more=true.
	p1 := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0, "limit": 2})
	if got := eventSequences(t, p1.Events); fmt.Sprint(got) != fmt.Sprint([]string{"1", "2"}) {
		t.Fatalf("page 1 sequences = %v, want [1 2]", got)
	}
	if !p1.More {
		t.Fatal("page 1: want more=true (3 events remain beyond this page)")
	}
	if p1.HeadSequence != 5 {
		t.Fatalf("page 1: headSequence = %d, want 5", p1.HeadSequence)
	}

	// Page 2: next 2 of 5 consumed, 1 remains -> more=true.
	p2 := callGetEventsSince(t, cs, map[string]any{"afterSequence": 2, "limit": 2})
	if got := eventSequences(t, p2.Events); fmt.Sprint(got) != fmt.Sprint([]string{"3", "4"}) {
		t.Fatalf("page 2 sequences = %v, want [3 4]", got)
	}
	if !p2.More {
		t.Fatal("page 2: want more=true (1 event remains beyond this page)")
	}

	// Page 3 ("beyond-end"): only 1 remains but limit=2 -> more=false.
	p3 := callGetEventsSince(t, cs, map[string]any{"afterSequence": 4, "limit": 2})
	if got := eventSequences(t, p3.Events); fmt.Sprint(got) != fmt.Sprint([]string{"5"}) {
		t.Fatalf("page 3 sequences = %v, want [5]", got)
	}
	if p3.More {
		t.Fatal("page 3: want more=false — limit (2) exceeds what remains (1), the beyond-end boundary")
	}

	// "exactly-limit": remaining count (5) equals limit (5) exactly -> more=false.
	all := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0, "limit": 5})
	if len(all.Events) != 5 {
		t.Fatalf("exactly-limit: got %d events, want 5", len(all.Events))
	}
	if all.More {
		t.Fatal("exactly-limit: want more=false — remaining count equals limit exactly, nothing beyond")
	}

	// afterSequence==head: nothing left at all -> empty + more=false.
	atHead := callGetEventsSince(t, cs, map[string]any{"afterSequence": 5, "limit": 2})
	if len(atHead.Events) != 0 {
		t.Fatalf("afterSequence==head: got %d events, want 0", len(atHead.Events))
	}
	if atHead.More {
		t.Fatal("afterSequence==head: want more=false")
	}
	if atHead.HeadSequence != 5 {
		t.Fatalf("afterSequence==head: headSequence = %d, want 5", atHead.HeadSequence)
	}
}

// TestGetEventsSinceEventsAreProtojsonSequenceStrings pins the wire-
// convention divergence the tool's Description documents: the "sequence"
// field INSIDE each returned envelope is protojson (a JSON string), even
// though the tool's own afterSequence/headSequence arguments and results
// are plain JSON numbers.
func TestGetEventsSinceEventsAreProtojsonSequenceStrings(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	seedEvents(t, conn, canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}))
	waitForHeadSequence(t, cs, 1)

	page := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0, "limit": 10})
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want 1", len(page.Events))
	}
	var generic map[string]any
	if err := json.Unmarshal(page.Events[0], &generic); err != nil {
		t.Fatalf("decode envelope as generic JSON: %v", err)
	}
	seq, ok := generic["sequence"]
	if !ok {
		t.Fatalf("envelope has no \"sequence\" key: %s", page.Events[0])
	}
	if _, isString := seq.(string); !isString {
		t.Fatalf("envelope's \"sequence\" = %#v (%T), want a JSON STRING (protojson convention), got a bare number", seq, seq)
	}
	if seq != "1" {
		t.Fatalf("envelope's \"sequence\" = %q, want \"1\"", seq)
	}
}

// TestGetEventsSinceDefaultLimitAppliesWhenOmitted seeds 60 events and
// calls get_events_since with no "limit" key at all: the default (50) must
// apply, returning exactly 50 with more=true.
func TestGetEventsSinceDefaultLimitAppliesWhenOmitted(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	events := make([]*vttv1.Envelope, 0, 60)
	events = append(events, canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}))
	for i := 2; i <= 60; i++ {
		events = append(events, canned(int64(i), fmt.Sprintf("ev-%d", i),
			&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: fmt.Sprintf("act-%d", i), Name: "A"}}))
	}
	seedEvents(t, conn, events...)
	waitForHeadSequence(t, cs, 60)

	page := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0})
	if len(page.Events) != 50 {
		t.Fatalf("omitted limit: got %d events, want 50 (the default)", len(page.Events))
	}
	if !page.More {
		t.Fatal("omitted limit: want more=true (10 events remain beyond the default-50 page)")
	}
}

// TestGetStateDescriptionDocumentsBodyShape covers final review Fix 3b:
// get_state's own tool description must explicitly state the body's Go-
// JSON casing convention and name a concrete numeric field (Session's
// StartSeq/EndSeq) plus headSequence's camelCase exception, not just point
// generically at the server Instructions' rule.
func TestGetStateDescriptionDocumentsBodyShape(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var desc string
	for _, tl := range res.Tools {
		if tl.Name == "get_state" {
			desc = tl.Description
		}
	}
	if desc == "" {
		t.Fatal("ListTools: get_state not found")
	}
	for _, want := range []string{"StartSeq", "EndSeq", "camelCase"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("get_state description does not mention %q:\n%s", want, desc)
		}
	}
}

// TestGetStateSessionSequenceFieldsAreGoJSONNumbersNotProtojsonStrings
// behaviorally proves the claim TestGetStateDescriptionDocumentsBodyShape
// pins in the tool's own text (final review Fix 3): unlike CommandResult
// and event envelopes' protojson int64-as-string convention, get_state's
// body follows the state dump's Go-JSON conventions instead — a session's
// StartSeq must decode as a JSON NUMBER, never a protojson-style string,
// under its exact Go struct field name ("StartSeq", "Sessions" — not
// camelCase).
func TestGetStateSessionSequenceFieldsAreGoJSONNumbersNotProtojsonStrings(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	seedEvents(t, conn, canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}))

	deadline := time.Now().Add(5 * time.Second)
	var generic map[string]any
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_state", Arguments: map[string]any{}})
		cancel()
		if err != nil {
			t.Fatalf("get_state: CallTool: %v", err)
		}
		text, ok := res.Content[0].(*mcpsdk.TextContent)
		if !ok {
			t.Fatalf("get_state: want text content, got %T", res.Content[0])
		}
		generic = nil
		if err := json.Unmarshal([]byte(text.Text), &generic); err != nil {
			t.Fatalf("get_state: decode generic JSON: %v", err)
		}
		if hs, ok := generic["headSequence"].(float64); ok && hs >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if generic == nil {
		t.Fatal("get_state: headSequence never reached 1 within deadline")
	}

	sessions, ok := generic["Sessions"].([]any)
	if !ok || len(sessions) != 1 {
		t.Fatalf(`get_state: "Sessions" missing or not a 1-element array: %#v`, generic["Sessions"])
	}
	session, ok := sessions[0].(map[string]any)
	if !ok {
		t.Fatalf("get_state: Sessions[0] not an object: %#v", sessions[0])
	}
	startSeq, isNumber := session["StartSeq"].(float64)
	if !isNumber {
		t.Fatalf(`get_state: Sessions[0]["StartSeq"] = %#v, want a JSON number`, session["StartSeq"])
	}
	if startSeq != 1 {
		t.Fatalf("get_state: Sessions[0].StartSeq = %v, want 1", startSeq)
	}
}

// TestReadToolsReportWireConnectedTrueWhileConnected covers final review
// Fix 6a's additive key on the connected-happy-path side: both get_state
// and get_events_since must report "wireConnected": true while the
// underlying harness.Client is live.
func TestReadToolsReportWireConnectedTrueWhileConnected(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	seedEvents(t, conn, canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}))
	waitForHeadSequence(t, cs, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_state", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("get_state: CallTool: %v", err)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_state: want text content, got %T", res.Content[0])
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(text.Text), &generic); err != nil {
		t.Fatalf("get_state: decode generic JSON: %v", err)
	}
	if wc, ok := generic["wireConnected"].(bool); !ok || !wc {
		t.Fatalf(`get_state: "wireConnected" = %#v, want true while connected`, generic["wireConnected"])
	}

	page := callGetEventsSince(t, cs, map[string]any{"afterSequence": int64(0)})
	if !page.WireConnected {
		t.Fatal("get_events_since: want wireConnected=true while connected")
	}
}

// TestReadToolsReportWireConnectedFalseAfterConnectionLoss covers Fix 6a's
// actual payoff: once the underlying wire connection is lost and redial is
// permanently refused (maxConns=1, the same deterministic drop pattern
// server_test.go's TestCallToolWhileDisconnectedReturnsCleanError uses),
// both read tools must still SUCCEED (their accumulated history survives
// the drop — that's the whole point of read tools staying usable while
// disconnected) but report "wireConnected": false — the caller's signal
// that this is a frozen snapshot, not a live view, and that command tools
// will fail until reconnect.
func TestReadToolsReportWireConnectedFalseAfterConnectionLoss(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	fs.maxConns = 1
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	seedEvents(t, conn, canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}))
	waitForHeadSequence(t, cs, 1)

	if err := conn.Close(websocket.StatusNormalClosure, "test: forcing a wire drop"); err != nil {
		t.Fatalf("forcing connection close: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var gotState map[string]any
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_state", Arguments: map[string]any{}})
		cancel()
		if err != nil {
			t.Fatalf("get_state: CallTool: %v", err)
		}
		text, ok := res.Content[0].(*mcpsdk.TextContent)
		if !ok {
			t.Fatalf("get_state: want text content, got %T", res.Content[0])
		}
		var generic map[string]any
		if err := json.Unmarshal([]byte(text.Text), &generic); err != nil {
			t.Fatalf("get_state: decode generic JSON: %v", err)
		}
		if wc, ok := generic["wireConnected"].(bool); ok && !wc {
			gotState = generic
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if gotState == nil {
		t.Fatal("get_state: wireConnected never became false within deadline")
	}
	if hs, ok := gotState["headSequence"].(float64); !ok || int64(hs) != 1 {
		t.Fatalf("get_state: headSequence = %v, want 1 (accumulated history must survive the drop)", gotState["headSequence"])
	}

	page := callGetEventsSince(t, cs, map[string]any{"afterSequence": int64(0)})
	if page.WireConnected {
		t.Fatal("get_events_since: want wireConnected=false after permanent connection loss")
	}
	if len(page.Events) != 1 {
		t.Fatalf("get_events_since: got %d events, want 1 (accumulated history must survive the drop)", len(page.Events))
	}
}

// TestGetEventsSinceUnknownArgumentIsCleanIsErrorNamingField covers final
// review Fix 6b: an unrecognized argument key must come back as a TOOL-
// LEVEL isError (res.IsError=true, err=nil from CallTool — the LLM can see
// and course-correct on it) naming the offending key, not a raw protocol-
// level error from the SDK's own dispatch (today's behavior: the plain
// json.Unmarshal decode silently accepts and ignores unknown keys, so this
// exact case doesn't even fail today — DisallowUnknownFields is what makes
// it fail at all).
func TestGetEventsSinceUnknownArgumentIsCleanIsErrorNamingField(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_events_since",
		Arguments: map[string]any{"afterSequence": 0, "bogusKey": 1},
	})
	if err != nil {
		t.Fatalf("get_events_since: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("get_events_since: want IsError=true for an unknown argument, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_events_since: want text content, got %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "bogusKey") {
		t.Fatalf("get_events_since: error text does not name the offending field %q: %q", "bogusKey", text.Text)
	}
}

// TestGetEventsSinceWrongArgumentTypeIsCleanIsErrorNamingField covers the
// wrong-JSON-type half of Fix 6b: afterSequence sent as a string (an easy
// LLM mistake given get_events_since's own Description warns about the
// protojson int64-as-string convention elsewhere on the wire) must also
// come back as a clean tool-level isError naming "afterSequence", not a
// protocol-level error or (worse) a silent zero-value coercion.
func TestGetEventsSinceWrongArgumentTypeIsCleanIsErrorNamingField(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_events_since",
		Arguments: map[string]any{"afterSequence": "not-a-number"},
	})
	if err != nil {
		t.Fatalf("get_events_since: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("get_events_since: want IsError=true for a wrong-typed argument, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_events_since: want text content, got %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "afterSequence") {
		t.Fatalf("get_events_since: error text does not name the offending field %q: %q", "afterSequence", text.Text)
	}
}

// TestGetEventsSinceLimitClampedToMaximum seeds 205 events and requests a
// limit far above 200: the response must be clamped to exactly 200, never
// the raw requested value.
func TestGetEventsSinceLimitClampedToMaximum(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	conn := fs.firstConn(t)
	events := make([]*vttv1.Envelope, 0, 205)
	events = append(events, canned(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}))
	for i := 2; i <= 205; i++ {
		events = append(events, canned(int64(i), fmt.Sprintf("ev-%d", i),
			&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: fmt.Sprintf("act-%d", i), Name: "A"}}))
	}
	seedEvents(t, conn, events...)
	waitForHeadSequence(t, cs, 205)

	page := callGetEventsSince(t, cs, map[string]any{"afterSequence": 0, "limit": 100000})
	if len(page.Events) != 200 {
		t.Fatalf("oversized limit: got %d events, want 200 (clamped max)", len(page.Events))
	}
	if !page.More {
		t.Fatal("oversized limit: want more=true (5 events remain beyond the clamped-200 page)")
	}
}
