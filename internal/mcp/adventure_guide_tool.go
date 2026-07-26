package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// noAdventuresAvailableGuideMessage is get_adventure_guide's clean,
// tool-level isError message when the server was started with no
// --adventures-dir flag (Config.AdventureGuides empty, adventure-format
// Task 4 spec §7 binding: "no dir → tool returns clean 'no adventures
// available'"). Mirrors internal/gateway's own load_adventure wording
// (adventure.go's errNoAdventuresAvailable) and noRulesetLoadedGuideMessage's
// own precedent in this file's sibling (guide_tool.go), for consistency
// across the "no X configured" surfaces an LLM might hit in the same
// session.
const noAdventuresAvailableGuideMessage = "mcp: no adventures available"

const getAdventureGuideDescription = `Return an adventure's guide.md ` +
	`verbatim — LLM-facing DM affordances and secrets (beats, hidden rooms, ` +
	`when to reveal which note) for the adventure named by adventureId, ` +
	`written by the adventure's own author (not this server). This content ` +
	`NEVER enters the event log — load_adventure's own compiled batch ` +
	`carries only the REVEALED facts (scenes/actors/notes/opening ` +
	`narration); this tool is the only way to see the rest. If this server ` +
	`was started with no --adventures-dir flag, returns a tool-level error ` +
	`explaining that no adventures are available; an unrecognized ` +
	`adventureId returns a tool-level error naming it — load_adventure ` +
	`will also fail cleanly for the same unknown id.`

var getAdventureGuideInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"adventureId": map[string]any{
			"type":        "string",
			"description": "The adventure's id (matches load_adventure's own adventureId argument).",
		},
	},
	"required": []string{"adventureId"},
}

// getAdventureGuideArgs is decoded with encoding/json — NOT protojson, the
// same convention get_events_since's own arguments follow (read_tools.go's
// getEventsSinceArgs doc comment): this is a plain MCP tool call, not a
// protojson message body.
type getAdventureGuideArgs struct {
	AdventureID string `json:"adventureId"`
}

// decodeStrictGetAdventureGuideArgs decodes raw into getAdventureGuideArgs,
// rejecting any key the struct doesn't declare and any value of the wrong
// JSON type — read_tools.go's decodeStrictGetEventsSinceArgs precedent,
// reused in shape (not code — that helper is typed to
// getEventsSinceArgs specifically).
func decodeStrictGetAdventureGuideArgs(raw []byte) (getAdventureGuideArgs, error) {
	var args getAdventureGuideArgs
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&args); err != nil {
		return getAdventureGuideArgs{}, describeArgsDecodeError(err)
	}
	return args, nil
}

// registerAdventureGuideTool adds get_adventure_guide to the same tool
// table New builds every other tool into.
func (s *Server) registerAdventureGuideTool() {
	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_adventure_guide",
		Description: getAdventureGuideDescription,
		InputSchema: getAdventureGuideInputSchema,
	}, s.handleGetAdventureGuide)
}

// handleGetAdventureGuide returns s.cfg.AdventureGuides[adventureId]
// verbatim, or a clean tool-level isError when none is configured at all
// (empty map) or the id is unrecognized (non-empty map, unknown key). Like
// handleGetRulesetGuide (guide_tool.go), this is a pure, connection-
// independent read: the guide text was already fully resolved at server
// construction (cmd/vtt/mcp.go's own --adventures-dir boot step), so it
// keeps working even while the underlying wire is disconnected.
func (s *Server) handleGetAdventureGuide(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if len(s.cfg.AdventureGuides) == 0 {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: noAdventuresAvailableGuideMessage}},
			IsError: true,
		}, nil
	}

	raw := []byte(req.Params.Arguments)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	args, err := decodeStrictGetAdventureGuideArgs(raw)
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("mcp: get_adventure_guide: invalid arguments: %s", err)}},
			IsError: true,
		}, nil
	}

	guide, ok := s.cfg.AdventureGuides[args.AdventureID]
	if !ok {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("mcp: unknown adventure %q", args.AdventureID)}},
			IsError: true,
		}, nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: guide}},
	}, nil
}
