package main

import (
	"context"
	"strings"
	"testing"
	"time"

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
	return s.useAbility(caster, "fists")
}

func (s *mcpAbilitySession) useAbility(caster, ability string) *mcpsdk.CallToolResult {
	s.t.Helper()
	return mustCallTool(s.t, s.cs, "use_ability", map[string]any{
		"actorId": caster, "abilityId": ability, "targetIds": []any{"patron"}})
}

// refused calls tool with args and wants a tool error holding want.
func (s *mcpAbilitySession) refused(tool string, args map[string]any, want string) {
	s.t.Helper()
	res := mustCallTool(s.t, s.cs, tool, args)
	if text := toolResultText(s.t, res); !res.IsError || !strings.Contains(text, want) {
		s.t.Fatalf("%s %v answered isError=%v %s, want a tool error containing %s", tool, args, res.IsError, text, want)
	}
}

// VTT-764
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

// VTT-767
func TestMCPTakeAbilityThenAUseIsAToolError(t *testing.T) {
	s := mcpAbilityFixture(t)
	s.mustSucceed("take_ability", map[string]any{"actorId": "brawler", "abilityId": "fists"})
	res := s.use("brawler")
	if want := `actor \"brawler\" does not name ability \"fists\"`; !res.IsError || !strings.Contains(toolResultText(t, res), want) {
		t.Fatalf("a use after the take answered isError=%v %s, want a tool error containing %s",
			res.IsError, toolResultText(t, res), want)
	}
}

// VTT-760 VTT-761 VTT-787
func TestMCPGiveAbilityWithItsResourcesValueThenAUseResolves(t *testing.T) {
	s := mcpAbilityFixture(t)
	s.refused("give_ability", map[string]any{"actorId": "rookie", "abilityId": "chair-swing"}, `\"stamina\"`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "give_ability", Arguments: map[string]any{
		"actorId": "rookie", "abilityId": "chair-swing", "resources": map[string]any{"stamina": map[string]any{"max": 2}}}})
	if want := `actor "rookie": resources.stamina.current`; err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("a give with its current left out answered %v, want an error containing %s", err, want)
	}
	s.mustSucceed("give_ability", map[string]any{"actorId": "rookie", "abilityId": "chair-swing",
		"resources": map[string]any{"stamina": map[string]any{"current": 1, "max": 2}}})
	if res := s.useAbility("rookie", "chair-swing"); res.IsError || toolCommandResult(t, res)["ok"] != true {
		t.Fatalf("a use after the give answered isError=%v %s, want it resolved", res.IsError, toolResultText(t, res))
	}
}

// VTT-767
func TestMCPTakeAbilityTakesTheResourceItSpendsWithIt(t *testing.T) {
	s := mcpAbilityFixture(t)
	s.mustSucceed("give_ability", map[string]any{"actorId": "rookie", "abilityId": "chair-swing",
		"resources": map[string]any{"stamina": map[string]any{"current": 2, "max": 2}}})
	s.mustSucceed("take_ability", map[string]any{"actorId": "rookie", "abilityId": "chair-swing"})
	s.refused("give_ability", map[string]any{"actorId": "rookie", "abilityId": "chair-swing"}, `\"stamina\"`)
	s.mustSucceed("give_ability", map[string]any{"actorId": "rookie", "abilityId": "chair-swing",
		"resources": map[string]any{"stamina": map[string]any{"current": 0, "max": 2}}})
}
