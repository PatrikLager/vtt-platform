package engine_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestApplyResourceChangedRejectsInt32WrapAcceptance pins the F2b fix: the
// engine must compute the clamp in int64 so a hand-crafted ResourceChanged
// cannot exploit int32 wraparound to be accepted.
//
// Pre-fix, res.Current + rc.Delta was computed in int32: int32max + 1 wraps
// to int32min, which the floor-clamp then pulls up to 0, so a dishonest
// new_value=0 matched "computed" and was ACCEPTED — silently zeroing a
// resource that should have grown. Computed in int64 the true value is
// 2147483648, which does not equal 0, so the event must be rejected and the
// resource left untouched (Apply validates before mutating).
func TestApplyResourceChangedRejectsInt32WrapAcceptance(t *testing.T) {
	st := seedScene(t)
	must(t, engine.Apply(st, actorAddedEnv(3, "a1", map[string]*vttv1.Resource{
		"vigor": {Current: 2147483647, Max: 0}, // uncapped, sitting at int32 max
	})))

	err := engine.Apply(st, env(4, &vttv1.ResourceChanged{
		ActorId: "a1", Resource: "vigor", Delta: 1, NewValue: 0, Reason: "wrap-exploit",
	}))
	if err == nil {
		t.Fatal("want rejection of an int32-wrap-crafted ResourceChanged, got nil")
	}
	if got := st.Actors["a1"].Resources["vigor"].GetCurrent(); got != 2147483647 {
		t.Fatalf("resource must be untouched by a rejected event: got current=%d, want 2147483647", got)
	}
}
