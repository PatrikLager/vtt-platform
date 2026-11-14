package main

import (
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpAbilitySession struct {
	t  *testing.T
	cs *mcpsdk.ClientSession
}

// Keep rookie's sheet naming no ability: the give must be the only reason its
// use resolves.
func mcpAbilityFixture(t *testing.T) *mcpAbilitySession {
	t.Helper()
	rulesetDir, err := resolveRulesetDir("tavern-brawl")
	if err != nil {
		t.Fatalf("resolveRulesetDir(tavern-brawl): %v", err)
	}
	fx := startMCPFixtureWithRuleset(t, rulesetDir)
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	t.Cleanup(cleanup)
	setUpTavernBrawlActorsViaMCP(t, cs)
	s := &mcpAbilitySession{t: t, cs: cs}
	s.mustSucceed("add_actor", map[string]any{"actor": map[string]any{"actorId": "rookie", "name": "Rookie",
		"kind": "ACTOR_KIND_PARTY_MEMBER", "attributes": map[string]any{"brawn": 3}}})
	s.mustSucceed("place_token", map[string]any{"tokenId": "tok-rookie", "sceneId": "tavern", "actorId": "rookie",
		"position": map[string]any{"x": 2, "y": 0}})
	return s
}

func (s *mcpAbilitySession) mustSucceed(tool string, args map[string]any) {
	s.t.Helper()
	if res := mustCallTool(s.t, s.cs, tool, args); res.IsError || toolCommandResult(s.t, res)["ok"] != true {
		s.t.Fatalf("%s %v answered isError=%v %s", tool, args, res.IsError, toolResultText(s.t, res))
	}
}

func (s *mcpAbilitySession) use(caster string) *mcpsdk.CallToolResult {
	s.t.Helper()
	return mustCallTool(s.t, s.cs, "use_ability", map[string]any{
		"actorId": caster, "abilityId": "fists", "targetIds": []any{"patron"}})
}

// VTT-713
func TestMCPGiveAbilityThenAUseResolves(t *testing.T) {
	s := mcpAbilityFixture(t)
	if res := s.use("rookie"); !res.IsError {
		t.Fatalf("a use before the give answered %s, want a tool error", toolResultText(t, res))
	}
	s.mustSucceed("give_ability", map[string]any{"actorId": "rookie", "abilityId": "fists"})
	if res := s.use("rookie"); res.IsError || toolCommandResult(t, res)["ok"] != true {
		t.Fatalf("a use after the give answered isError=%v %s, want it resolved", res.IsError, toolResultText(t, res))
	}
}

// VTT-714
func TestMCPTakeAbilityThenAUseIsAToolError(t *testing.T) {
	s := mcpAbilityFixture(t)
	s.mustSucceed("take_ability", map[string]any{"actorId": "brawler", "abilityId": "fists"})
	res := s.use("brawler")
	if want := `actor \"brawler\" does not name ability \"fists\"`; !res.IsError || !strings.Contains(toolResultText(t, res), want) {
		t.Fatalf("a use after the take answered isError=%v %s, want a tool error containing %s",
			res.IsError, toolResultText(t, res), want)
	}
}
