package harness

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func TestStatesEqualTellsApartTheRulesetEachNames(t *testing.T) {
	mk := func(id, version, name string) *engine.State {
		st := engine.NewState()
		if id == "" {
			return st
		}
		if err := engine.Apply(st, &vttv1.Envelope{Payload: &vttv1.Envelope_RulesetNamed{
			RulesetNamed: &vttv1.RulesetNamed{RulesetId: id, Version: version, Name: name}}}); err != nil {
			t.Fatal(err)
		}
		return st
	}
	named := mk("qa-rules", "1", "Plain Rules")
	if !statesEqual(named, mk("qa-rules", "1", "Plain Rules")) {
		t.Fatal("statesEqual tells apart two states naming one ruleset")
	}
	for what, other := range map[string]*engine.State{
		"none":            mk("", "", ""),
		"another id":      mk("other-rules", "1", "Plain Rules"),
		"another version": mk("qa-rules", "2", "Plain Rules"),
		"another name":    mk("qa-rules", "1", "Other Rules"),
	} {
		if statesEqual(named, other) {
			t.Errorf("statesEqual does not tell a state naming a ruleset from one naming %s", what)
		}
	}
}
