package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

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
	`"sequence" field inside individual event envelopes. The REST of the ` +
	`body (everything but headSequence and wireConnected) is NOT ` +
	`protojson: it follows the state dump's own Go-JSON conventions ` +
	`instead — top-level keys and the plain Go types under them (Scenes/` +
	`Tokens/Sessions/Notes and their own fields) serialize as their exact ` +
	`Go struct field names, e.g. a session's "StartSeq"/"EndSeq" (plain ` +
	`JSON numbers, never strings), or a world note's "Title"/"Text"/` +
	`"UpdatedSeq" keyed under Notes by the note's own key — while nested ` +
	`Actor values use their ` +
	`protobuf-generated snake_case tags instead (e.g. "actor_id"). ` +
	`Neither matches protojson's camelCase; headSequence and wireConnected ` +
	`are the two deliberately camelCase keys added on top of this body. ` +
	`wireConnected is a plain JSON boolean: when false, this entire body ` +
	`is a frozen snapshot (the last state received before the underlying ` +
	`wire connection dropped) — commands will fail until reconnect, but ` +
	`this tool keeps working off accumulated history either way.`

const getEventsSinceDescription = `Return event envelopes recorded after ` +
	`afterSequence, oldest first, up to limit at a time (default 50, max ` +
	`200 — values above 200 are clamped, not rejected), plus the current ` +
	`headSequence, a "more" flag (true if additional events remain beyond ` +
	`this page), and a "wireConnected" flag. afterSequence, limit, ` +
	`headSequence, more, and wireConnected are plain JSON numbers/` +
	`booleans, ordinary MCP tool convention. This is DIFFERENT from the ` +
	`"sequence" field INSIDE each returned event envelope: that one is ` +
	`protojson and serializes as a JSON STRING (e.g. "sequence": "42"), ` +
	`per contract/README.md's wire conventions — do not compare a numeric ` +
	`afterSequence against a string envelope sequence without converting ` +
	`one first. wireConnected: when false, data is a frozen snapshot (the ` +
	`accumulated history up to the last live connection) — commands will ` +
	`fail until reconnect, but this tool keeps paginating that history ` +
	`either way.`

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
// table New builds the 13 generic command tools into (spec §4: "Both
// registered in the same tool table").
// GoRegisteredToolNames are the tools registered in GO rather than generated
// from the contract manifest — the ones no tools.json can be derived from.
//
// It exists so there is ONE place to update, and #30 is why: the count of MCP
// tools lived at four separate sites that each carried their own copy, and the
// adventure-format sub-project shipped a stale one three times on a single
// rename. internal/mcp's own ListTools test asserts the registered set equals
// the contract manifest plus exactly this list, so a tool added without being
// named here fails immediately rather than at whichever site is checked next.
var GoRegisteredToolNames = []string{
	"get_state",           // read_tools.go
	"get_events_since",    // read_tools.go
	"get_ruleset_guide",   // guide_tool.go
	"get_adventure_guide", // adventure_guide_tool.go
	"get_join_link",       // door_tools.go (#45)
	"get_participants",    // door_tools.go (#45)
}

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
	raw, err := marshalStateWithHead(st, head, s.currentClient() != nil)
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

// unknownFieldPattern extracts the offending key from encoding/json's own
// DisallowUnknownFields error text (`json: unknown field "<key>"` — there
// is no typed error for this case, only that fixed message shape).
var unknownFieldPattern = regexp.MustCompile(`unknown field "([^"]+)"`)

// decodeStrictGetEventsSinceArgs decodes raw into getEventsSinceArgs,
// rejecting any key the struct doesn't declare (DisallowUnknownFields) and
// any value of the wrong JSON type, both surfaced as a single clean,
// caller-facing message naming the offending JSON argument (final review
// Fix 6b) — never encoding/json's own frequently internals-leaking text
// (a wrong-type error, left as-is, names the Go struct field PATH —
// "getEventsSinceArgs.afterSequence" — not the plain "afterSequence" the
// caller actually sent and the tool's own inputSchema documents).
func decodeStrictGetEventsSinceArgs(raw []byte) (getEventsSinceArgs, error) {
	var args getEventsSinceArgs
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&args); err != nil {
		return getEventsSinceArgs{}, describeArgsDecodeError(err)
	}
	return args, nil
}

// describeArgsDecodeError turns a decodeStrictGetEventsSinceArgs failure
// into a message naming the field by its JSON argument name: a wrong-type
// error is a *json.UnmarshalTypeError (whose Field is already the JSON tag
// name, not the Go field name, since getEventsSinceArgs' fields ARE
// tagged); an unknown-key error is untyped, so unknownFieldPattern pulls
// the key back out of encoding/json's fixed message text instead.
func describeArgsDecodeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Errorf("argument %q must be a %s, got a %s", typeErr.Field, typeErr.Type, typeErr.Value)
	}
	if m := unknownFieldPattern.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("unknown argument %q", m[1])
	}
	return err
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
	args, err := decodeStrictGetEventsSinceArgs(raw)
	if err != nil {
		// Tool-level isError, NOT a returned Go error: an unknown key or
		// wrong-typed value is a caller mistake the LLM can see and
		// correct on its next call, not an MCP protocol failure (a
		// returned error here would surface as the SDK's own dispatch
		// error instead — see handleGetEventsSince's doc comment / final
		// review Fix 6b).
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("mcp: get_events_since: invalid arguments: %s", err)}},
			IsError: true,
		}, nil
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

	out, err := marshalEventsSinceResult(page, head, more, s.currentClient() != nil)
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
// "headSequence" and "wireConnected" as sibling top-level keys, re-marshal.
// Deliberately mirrors cmd/vtt/state_dump.go's writeDump in approach byte-
// for-byte (not shared code — see handleGetState's doc comment on why);
// wireConnected is this package's own addition on top of that shared
// shape (final review Fix 6a) — state_dump.go has no such notion since
// `vtt state dump` is a one-shot process with no persistent connection to
// report on.
func marshalStateWithHead(st *engine.State, head int64, wireConnected bool) ([]byte, error) {
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
	wireConnectedRaw, err := json.Marshal(wireConnected)
	if err != nil {
		return nil, fmt.Errorf("marshal wireConnected: %w", err)
	}
	fields["wireConnected"] = wireConnectedRaw
	return json.Marshal(fields)
}

// eventsSinceResult is get_events_since's response shape (spec §4 plus
// final review Fix 6a's wireConnected addition): Events holds each
// envelope's own protojson encoding verbatim (json.RawMessage so
// encoding/json never re-interprets it), HeadSequence/More/WireConnected
// are plain Go values encoding/json marshals as an ordinary number/bool —
// the exact convention split the tool's Description documents.
type eventsSinceResult struct {
	Events        []json.RawMessage `json:"events"`
	HeadSequence  int64             `json:"headSequence"`
	More          bool              `json:"more"`
	WireConnected bool              `json:"wireConnected"`
}

func marshalEventsSinceResult(events []*vttv1.Envelope, head int64, more, wireConnected bool) ([]byte, error) {
	out := eventsSinceResult{
		Events:        make([]json.RawMessage, len(events)),
		HeadSequence:  head,
		More:          more,
		WireConnected: wireConnected,
	}
	for i, env := range events {
		raw, err := protojson.Marshal(env)
		if err != nil {
			return nil, fmt.Errorf("marshal envelope at sequence %d: %w", env.GetSequence(), err)
		}
		out.Events[i] = raw
	}
	return json.Marshal(&out)
}
