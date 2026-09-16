package engine_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestIsPartyMemberIsTrueForExactlyOneKind exists because the predicate moved
// packages and the move took it OUT OF THE MUTATION GATE'S REACH. gremlins
// targets a mutant by the package holding the mutated file; in internal/gateway
// this equality was covered by the perch and roster tests and its `==` -> `!=`
// mutant was being killed. Arriving in internal/engine with no test in this
// package, it scored NOT COVERED — and a not-covered mutant is excluded by
// definition, so the gate would have gone on passing while the one predicate
// that decides what a player is told EXISTS at all went unmeasured.
// internal/engine clears its coverage floor by a wide enough margin that
// check:coverage absorbed the new uncovered statement in silence.
//
// The nil case is the one no caller reaches: every production call site guards
// on a map lookup's ok before asking. GetKind()'s nil receiver makes it
// answerable anyway, and answering it here is cheaper than leaving the boundary
// to be discovered by whoever first forgets the guard.
func TestIsPartyMemberIsTrueForExactlyOneKind(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor *vttv1.Actor
		want  bool
	}{
		{"a declared party member", &vttv1.Actor{ActorId: "a", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}, true},
		{"a declared non-party actor", &vttv1.Actor{ActorId: "a", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}, false},
		{"an actor whose kind was never declared", &vttv1.Actor{ActorId: "a"}, false},
		{"no actor at all", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := engine.IsPartyMember(tc.actor); got != tc.want {
				t.Fatalf("IsPartyMember = %v, want %v", got, tc.want)
			}
		})
	}
}
