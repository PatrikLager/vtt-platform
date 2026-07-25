package harness

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestStatesEqualDiscriminatesConditions pins F9 for the soak keystone oracle:
// soak.go's statesEqual (used by the soak checkpoints' rebuild-vs-live
// comparison) must compare the engine.State.Conditions dimension this branch
// added, or a soak-driven condition divergence passes silently. This is the
// harness's independently-duplicated copy of campaign's statesEqual.
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
