package engine

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// IsPartyMember is whether a is one the party always knows about, whoever
// holds it now: its kind, never its controllers, and an absent kind is not a
// party member. Call it rather than spelling the predicate out again
// (SPEC-015, SPEC-016).
func IsPartyMember(a *vttv1.Actor) bool {
	return a.GetKind() == vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
}
