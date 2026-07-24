package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

const (
	// defaultEventsSinceLimit/maxEventsSinceLimit bound get_events_since's
	// limit argument (spec §4). Binding decisions (this task's own, beyond
	// what the brief pinned explicitly — flagged for reviewer sign-off):
	// an omitted OR non-positive limit defaults to 50 (0 is treated as
	// "unset", not "return nothing" — a degenerate value more useful to a
	// caller as a default than as a literal empty page); anything above
	// 200 is silently clamped rather than rejected as an error — a read
	// tool's pagination cap is a server-side resource bound, not a
	// caller-input-validation failure worth surfacing to the LLM as a
	// broken call.
	defaultEventsSinceLimit = 50
	maxEventsSinceLimit     = 200
)

const getStateDescription = `Return the campaign's current derived state, ` +
	`folded from every event this MCP server has received on its own live ` +
	`connection so far (never a second connection) — plus a top-level ` +
	`headSequence: the highest event sequence included in the fold. ` +
	`headSequence is a plain JSON number (ordinary MCP convention), the ` +
	`same as get_events_since's afterSequence/limit/headSequence — see ` +
	`that tool's description for how this differs from the protojson ` +
	`"sequence" field inside individual event envelopes.`

const getEventsSinceDescription = `Return event envelopes recorded after ` +
	`afterSequence, oldest first, up to limit at a time (default 50, max ` +
	`200 — values above 200 are clamped, not rejected), plus the current ` +
	`headSequence and a "more" flag (true if additional events remain ` +
	`beyond this page). afterSequence, limit, headSequence, and more are ` +
	`plain JSON numbers/booleans, ordinary MCP tool convention. This is ` +
	`DIFFERENT from the "sequence" field INSIDE each returned event ` +
	`envelope: that one is protojson and serializes as a JSON STRING ` +
	`(e.g. "sequence": "42"), per contract/README.md's wire conventions — ` +
	`do not compare a numeric afterSequence against a string envelope ` +
	`sequence without converting one first.`

var getStateInputSchema = map[string]any{
	"type":       "object",
	"properties": map[string]any{},
	"required":   []string{},
}

var getEventsSinceInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"afterSequence": map[string]any{
			"type":        "integer",
			"description": "Return events with sequence > this value. A plain JSON number, not a protojson string.",
		},
		"limit": map[string]any{
			"type":        "integer",
			"description": "Max events to return; default 50, max 200 (values above 200 are clamped).",
		},
	},
	"required": []string{"afterSequence"},
}

// registerReadTools adds get_state and get_events_since to the same tool
// table New builds the seven generic command tools into (spec §4: "Both
// registered in the same tool table").
func (s *Server) registerReadTools() {
	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_state",
		Description: getStateDescription,
		InputSchema: getStateInputSchema,
	}, s.handleGetState)

	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_events_since",
		Description: getEventsSinceDescription,
		InputSchema: getEventsSinceInputSchema,
	}, s.handleGetEventsSince)
}

// handleGetState folds Server's own accumulated history (historySnapshot —
// never a second connection, spec §3's binding consistency rule) via
// harness.Fold and returns the SAME JSON shape `vtt state dump` prints
// (cmd/vtt/state_dump.go's writeDump): the folded *engine.State marshaled
// through encoding/json — its exported fields' own Go/json-tag names (e.g.
// Token's ID/SceneID/ActorID/X/Y, Actor's snake_case protobuf-generated
// tags) verbatim, NOT re-shaped to protojson camelCase — with
// "headSequence" added as a sibling top-level key. "The dump contract
// verbatim" (task-2-brief.md) means exactly this marshaling approach,
// deliberately duplicated rather than shared: internal/mcp cannot import
// cmd/vtt (cmd depends on mcp, never the reverse; go-arch-lint's cmd
// component is the only one naming mcp as a dependency) — see
// marshalStateWithHead.
func (s *Server) handleGetState(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	events, head := s.historySnapshot()
	st, err := harness.Fold(events)
	if err != nil {
		return nil, fmt.Errorf("mcp: get_state: fold: %w", err)
	}
	raw, err := marshalStateWithHead(st, head)
	if err != nil {
		return nil, fmt.Errorf("mcp: get_state: %w", err)
	}
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(raw)}}}, nil
}

// getEventsSinceArgs is decoded with encoding/json — NOT protojson.
// get_events_since's arguments are a plain MCP tool call, not a protojson
// message body; that is precisely the wire-convention divergence the
// tool's own Description documents for the LLM (bare "afterSequence": 42,
// never a protojson-style "afterSequence": "42").
type getEventsSinceArgs struct {
	AfterSequence int64 `json:"afterSequence"`
	Limit         int64 `json:"limit"`
}

// handleGetEventsSince paginates Server's accumulated history (the SAME
// historySnapshot get_state folds — single source, spec §3) strictly after
// args.AfterSequence, clamps/defaults the limit, and returns each
// surviving envelope as its own protojson encoding (so the envelope's
// internal "sequence" field is a STRING, per contract/README.md's wire
// convention — see marshalEventsSinceResult) alongside a plain-JSON-number
// headSequence and a more flag.
func (s *Server) handleGetEventsSince(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	raw := []byte(req.Params.Arguments)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var args getEventsSinceArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("mcp: get_events_since: invalid arguments: %w", err)
	}

	limit := args.Limit
	if limit <= 0 {
		limit = defaultEventsSinceLimit
	}
	if limit > maxEventsSinceLimit {
		limit = maxEventsSinceLimit
	}

	events, head := s.historySnapshot()
	page, more := paginateSince(events, args.AfterSequence, int(limit))

	out, err := marshalEventsSinceResult(page, head, more)
	if err != nil {
		return nil, fmt.Errorf("mcp: get_events_since: %w", err)
	}
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(out)}}}, nil
}

// paginateSince returns the slice of history strictly after `after`
// (history is guaranteed ascending-by-sequence with no duplicates —
// Server.recordEvent's own invariant, server.go), capped at limit, plus
// whether any further events remain beyond that cap. history is walked
// linearly rather than binary-searched: table-scale campaign logs (this
// task's documented retention assumption — Server.history's doc comment,
// server.go) make the O(n) scan entirely negligible, and a linear scan
// over a value already proven sorted needs no separate invariant of its
// own to maintain.
func paginateSince(history []*vttv1.Envelope, after int64, limit int) (page []*vttv1.Envelope, more bool) {
	start := 0
	for start < len(history) && history[start].GetSequence() <= after {
		start++
	}
	remaining := history[start:]
	if len(remaining) > limit {
		return remaining[:limit], true
	}
	return remaining, false
}

// marshalStateWithHead is get_state's "dump contract verbatim" shaping:
// marshal st with encoding/json, re-decode into a generic field map, add
// "headSequence" as a sibling top-level key, re-marshal. Deliberately
// mirrors cmd/vtt/state_dump.go's writeDump in approach byte-for-byte (not
// shared code — see handleGetState's doc comment on why).
func marshalStateWithHead(st *engine.State, head int64) ([]byte, error) {
	raw, err := json.Marshal(st)
	if err != nil {
		return nil, fmt.Errorf("marshal state: %w", err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("re-decode state: %w", err)
	}
	headRaw, err := json.Marshal(head)
	if err != nil {
		return nil, fmt.Errorf("marshal headSequence: %w", err)
	}
	fields["headSequence"] = headRaw
	return json.Marshal(fields)
}

// eventsSinceResult is get_events_since's response shape (spec §4): Events
// holds each envelope's own protojson encoding verbatim (json.RawMessage
// so encoding/json never re-interprets it), HeadSequence/More are plain Go
// values encoding/json marshals as an ordinary number/bool — the exact
// convention split the tool's Description documents.
type eventsSinceResult struct {
	Events       []json.RawMessage `json:"events"`
	HeadSequence int64             `json:"headSequence"`
	More         bool              `json:"more"`
}

func marshalEventsSinceResult(events []*vttv1.Envelope, head int64, more bool) ([]byte, error) {
	out := eventsSinceResult{Events: make([]json.RawMessage, len(events)), HeadSequence: head, More: more}
	for i, env := range events {
		raw, err := protojson.Marshal(env)
		if err != nil {
			return nil, fmt.Errorf("marshal envelope at sequence %d: %w", env.GetSequence(), err)
		}
		out.Events[i] = raw
	}
	return json.Marshal(&out)
}
