package main

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

// mcpReferenceFixture is the agent's MCP session against referenceRulesetDir,
// with ada added holding k, delta in kilo, which holds an entry once.
func mcpReferenceFixture(t *testing.T) *mcpAbilitySession {
	t.Helper()
	fx := startMCPFixtureWithRuleset(t, referenceRulesetDir(t))
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	t.Cleanup(cleanup)
	s := &mcpAbilitySession{t: t, cs: cs}
	s.mustSucceed("add_actor", map[string]any{"actor": map[string]any{"actorId": "ada", "name": "Ada",
		"kind": "ACTOR_KIND_NON_PARTY", "references": map[string]any{"k": map[string]any{"entryId": "delta", "place": "kilo"}}}})
	return s
}

// appended calls tool with args, wants it accepted, and returns its sequence.
func (s *mcpAbilitySession) appended(tool string, args map[string]any) int64 {
	s.t.Helper()
	res := mustCallTool(s.t, s.cs, tool, args)
	result := toolCommandResult(s.t, res)
	seq, err := strconv.ParseInt(fmt.Sprint(result["sequence"]), 10, 64)
	if res.IsError || result["ok"] != true || err != nil {
		s.t.Fatalf("%s %v answered isError=%v %s, want ok with a sequence", tool, args, res.IsError, toolResultText(s.t, res))
	}
	return seq
}

// VTT-1046 VTT-1047 VTT-1048
func TestMCPAReferenceIsAddedChangedAndRemovedAndARefusedOneAppendsNothing(t *testing.T) {
	s := mcpReferenceFixture(t)
	var seqs []int64
	seqs = append(seqs, s.appended("add_reference", map[string]any{"actorId": "ada", "referenceId": "b",
		"reference": map[string]any{"entryId": "echo", "place": "mike", "state": "lima"}}))
	s.refused("add_reference", map[string]any{"actorId": "ada", "referenceId": "a",
		"reference": map[string]any{"entryId": "delta", "place": "kilo"}},
		`reference \"a\" names entry \"delta\", which reference \"k\" holds in place \"kilo\" already`)
	seqs = append(seqs, s.appended("change_reference", map[string]any{"actorId": "ada", "referenceId": "b",
		"place": "mike", "state": "oscar"}))
	s.refused("change_reference", map[string]any{"actorId": "ada", "referenceId": "b", "place": "kilo"},
		`reference \"b\" names entry \"echo\" of form \"zulu\", which place \"kilo\" does not hold`)
	seqs = append(seqs, s.appended("remove_reference", map[string]any{"actorId": "ada", "referenceId": "k"}))
	s.refused("remove_reference", map[string]any{"actorId": "ada", "referenceId": "k"},
		`actor \"ada\" does not hold reference \"k\"`)
	if seqs[1] != seqs[0]+1 || seqs[2] != seqs[1]+1 {
		t.Fatalf("the accepted commands landed at %v, want consecutive sequences: a refused one appended", seqs)
	}

	state := waitForMCPHeadSequence(t, s.cs, seqs[2])
	if head := int64(state["headSequence"].(float64)); head != seqs[2] {
		t.Fatalf("get_state's head is %d after the refused removal, want %d", head, seqs[2])
	}
	actors, _ := state["Actors"].(map[string]any)
	ada, _ := actors["ada"].(map[string]any)
	want := map[string]any{"b": map[string]any{"entry_id": "echo", "place": "mike", "state": "oscar"}}
	if got := ada["references"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ada's references are %v, want %v", got, want)
	}
}
