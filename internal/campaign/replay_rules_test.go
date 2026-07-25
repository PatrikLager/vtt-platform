package campaign_test

import (
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// condIDs returns the ordered condition ids on an actor in a state snapshot.
func condIDs(st *engine.State, actorID string) []string {
	var out []string
	for _, c := range st.Conditions[actorID] {
		out = append(out, c.ID)
	}
	return out
}

// TestReopenReplaysResourceAndConditionEvents pins P3's rebuild-equals-live
// pillar for the rules event family this branch introduced (F4): after a
// batch of ResourceChanged + ConditionApplied + ConditionRemoved persists,
// closing and reopening the campaign must rebuild Resources AND Conditions
// from the log EXACTLY as they stood live. foldEvents is the single fold
// behind every reopen; dropping or mis-folding these variants there would
// silently lose all resource values and conditions on reopen — this test (and
// its statesEqual full-equality check, F9) is what catches that.
func TestReopenReplaysResourceAndConditionEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10}))
	must(t, c, cenv(nextID(), &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "a1", Name: "Hero", ModuleId: "m",
		Resources: map[string]*vttv1.Resource{"vigor": {Current: 5, Max: 10}},
	}}))

	// A batch exercising every new variant, with a ConditionRemoved sequence:
	// vigor 5->7, apply marked, apply bruised, remove marked. Net: vigor=7,
	// conditions=[bruised].
	envs := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.ResourceChanged{ActorId: "a1", Resource: "vigor", Delta: 2, NewValue: 7, Reason: "ability:x:hit"}),
		cenv(nextID(), &vttv1.ConditionApplied{ActorId: "a1", ConditionId: "marked", Source: "threshold:vigor"}),
		cenv(nextID(), &vttv1.ConditionApplied{ActorId: "a1", ConditionId: "bruised", Source: "ability:x:hit"}),
		cenv(nextID(), &vttv1.ConditionRemoved{ActorId: "a1", ConditionId: "marked", Reason: "ability:y:effect"}),
	}
	if _, err := c.AppendBatch(envs); err != nil {
		t.Fatalf("AppendBatch(rules events): %v", err)
	}

	before := c.State()
	if got := before.Actors["a1"].GetResources()["vigor"].GetCurrent(); got != 7 {
		t.Fatalf("pre-close vigor = %d, want 7", got)
	}
	if got := condIDs(before, "a1"); len(got) != 1 || got[0] != "bruised" {
		t.Fatalf("pre-close conditions = %v, want [bruised]", got)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	c2, err := campaign.Open(path) // rebuilds the projection from the log
	if err != nil {
		t.Fatalf("reopen: log must replay the rules events cleanly, got: %v", err)
	}
	t.Cleanup(func() { c2.Close() })
	after := c2.State()

	if got := after.Actors["a1"].GetResources()["vigor"].GetCurrent(); got != 7 {
		t.Fatalf("post-reopen vigor = %d, want 7 (ResourceChanged did not survive replay)", got)
	}
	if got := condIDs(after, "a1"); len(got) != 1 || got[0] != "bruised" {
		t.Fatalf("post-reopen conditions = %v, want [bruised] (Condition events did not survive replay)", got)
	}
	if !statesEqual(before, after) {
		t.Fatalf("rebuild != live for rules events\nbefore: %+v\nafter:  %+v", before, after)
	}
}
