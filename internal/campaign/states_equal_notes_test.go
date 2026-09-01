package campaign_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestStatesEqualDiscriminatesNotes pins the world-layer engine fold for the
// campaign keystone oracle: statesEqual (used by TestRebuildEqualsLiveProperty
// and TestExitScenario) must compare the engine.State.Notes dimension this task
// added — otherwise a rebuild-vs-live divergence in notes passes silently (the
// 5c lesson: the oracle lags the state at our peril).
func TestStatesEqualDiscriminatesNotes(t *testing.T) {
	mk := func(notes map[string]engine.Note) *engine.State {
		st := engine.NewState()
		if notes != nil {
			st.Notes = notes
		}
		return st
	}
	withNote := mk(map[string]engine.Note{
		"town-hollowreach": {Title: "Hollowreach", Text: "A river town.", UpdatedSeq: 4},
	})
	without := mk(nil)

	if statesEqual(withNote, without) {
		t.Fatal("statesEqual must treat states that differ only in Notes as unequal")
	}
	if !statesEqual(withNote, mk(map[string]engine.Note{
		"town-hollowreach": {Title: "Hollowreach", Text: "A river town.", UpdatedSeq: 4},
	})) {
		t.Fatal("statesEqual must treat states with identical Notes as equal")
	}
}
