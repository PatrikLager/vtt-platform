package campaign_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestStatesEqualDiscriminatesConditions pins F9 for the campaign keystone
// oracle: statesEqual (used by TestRebuildEqualsLiveProperty, TestExitScenario,
// and the undo restore-to-pre-batch assertion) must compare the
// engine.State.Conditions dimension this branch added — otherwise a
// rebuild-vs-live or undo divergence in conditions passes silently.
func TestStatesEqualDiscriminatesConditions(t *testing.T) {
	mk := func(conds []engine.ActorCondition) *engine.State {
		st := engine.NewState()
		st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "a1"}
		if conds != nil {
			st.Conditions["a1"] = conds
		}
		return st
	}
	withCond := mk([]engine.ActorCondition{{ID: "marked", Source: "s", AppliedSeq: 3}})
	without := mk(nil)

	if statesEqual(withCond, without) {
		t.Fatal("statesEqual must treat states that differ only in Conditions as unequal")
	}
	if !statesEqual(withCond, mk([]engine.ActorCondition{{ID: "marked", Source: "s", AppliedSeq: 3}})) {
		t.Fatal("statesEqual must treat states with identical Conditions as equal")
	}
}
