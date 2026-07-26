package harness

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestStatesEqualDiscriminatesNotes pins the world-layer engine fold for the
// soak keystone oracle: soak.go's statesEqual (used by the soak checkpoints'
// rebuild-vs-live comparison) must compare the engine.State.Notes dimension
// this task added, or a soak-driven note divergence passes silently. This is
// the harness's independently-duplicated copy of campaign's statesEqual.
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
