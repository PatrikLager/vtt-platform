package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const rulesetPath = "/api/ruleset"

const getRulesetDescription = "Read the table's ruleset as your role may: its id and name, its " +
	"conditions, every property it declares (each with its key, its type, number or resource, the " +
	"name the table calls it and what it means), and its abilities (range, targets, usage): every " +
	"ability for the DM and the agent, and for a player only the abilities named by the sheets of " +
	"the actors they control. A number property's value is under an actor's `attributes`, a " +
	"resource property's under its `resources`, and the abilities an actor may use are the ids its " +
	"sheet names (`abilityIds` in add_actor, `ability_ids` in get_state); use_ability refuses any " +
	"other. With no ruleset loaded, id and name are empty and every list is []. Takes no arguments. " +
	"For how to run this ruleset at the table, read get_ruleset_guide."

func (s *Server) registerRulesetTool() {
	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_ruleset",
		Description: getRulesetDescription,
		InputSchema: emptyObjectSchema,
	}, s.handleGetRuleset)
}

// VTT-621 VTT-633
func (s *Server) handleGetRuleset(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	return s.readMetadata(ctx, rulesetPath)
}
