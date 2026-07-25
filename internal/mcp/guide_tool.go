package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// noRulesetLoadedGuideMessage is get_ruleset_guide's clean, tool-level
// isError message when the server was started with no --ruleset flag
// (Config.RulesetGuide == "", the ruleset-interpreter Task 6 spec §7
// binding: "get_ruleset_guide returns guide.md ... or a clear 'no ruleset
// loaded' error"). Mirrors internal/gateway's own use_ability wording
// (ruleset.go's errNoRulesetLoaded) for consistency across the two "no
// ruleset configured" surfaces an LLM might hit in the same session.
const noRulesetLoadedGuideMessage = "mcp: no ruleset loaded"

const getRulesetGuideDescription = `Return the loaded ruleset's guide.md ` +
	`verbatim — LLM-facing documentation for the abilities/conditions/ ` +
	`resources this table's ruleset defines, written by the ruleset's own ` +
	`author (not this server). Takes no arguments. If this server was ` +
	`started with no --ruleset flag, returns a tool-level error explaining ` +
	`that no ruleset is loaded — use_ability/remove_condition will also ` +
	`fail cleanly in that case.`

var getRulesetGuideInputSchema = map[string]any{
	"type":       "object",
	"properties": map[string]any{},
	"required":   []string{},
}

// registerGuideTool adds get_ruleset_guide to the same tool table New
// builds every other tool into.
func (s *Server) registerGuideTool() {
	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_ruleset_guide",
		Description: getRulesetGuideDescription,
		InputSchema: getRulesetGuideInputSchema,
	}, s.handleGetRulesetGuide)
}

// handleGetRulesetGuide returns s.cfg.RulesetGuide verbatim, or a clean
// tool-level isError when the server has none configured. This is a pure,
// connection-independent read (unlike get_state/get_events_since, it
// never touches s.currentClient() or historySnapshot — the guide text was
// already fully resolved at server construction, cmd/vtt/mcp.go's own
// --ruleset boot step) so it keeps working even while the underlying wire
// is disconnected, exactly like every other read tool in this package.
func (s *Server) handleGetRulesetGuide(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if s.cfg.RulesetGuide == "" {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: noRulesetLoadedGuideMessage}},
			IsError: true,
		}, nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: s.cfg.RulesetGuide}},
	}, nil
}
