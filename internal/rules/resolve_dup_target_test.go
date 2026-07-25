package rules_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// TestResolveRejectsDuplicateTargetIDs pins F3: a UseAbility that lists the
// SAME actor id more than once (within max_targets) must be rejected with a
// clean validation error, not applied per-target N times against one actor —
// which would let a wire client concentrate a full max_targets fan-out (and
// its stateful resource_change outcomes) on a single actor.
func TestResolveRejectsDuplicateTargetIDs(t *testing.T) {
	// fixtureRuleset (resolve_test.go) already populates Compiled directly —
	// Resolve executes CompiledPower exclusively, for every hand-built
	// fixture in this package.
	rs := fixtureRuleset(t)
	st := newTestState()
	putActor(st, "a", map[string]int32{"brawn": 3}, nil)
	putActor(st, "b", map[string]int32{"guard": 10}, map[string]*vttv1.Resource{"vigor": res(0, 10)})
	putToken(st, "ta", "s1", "a", 0, 0)
	putToken(st, "tb", "s1", "b", 1, 0)

	// sweep's max_targets is 2, so ["b","b"] clears the count gate; the
	// duplicate must still be rejected.
	envs, err := rules.Resolve(rs, st, useAbility("a", "sweep", "b", "b"), &queueRoller{queue: []int{15, 15}})
	wantErr(t, err, "duplicate target id")
	wantNoEvents(t, envs)
}
