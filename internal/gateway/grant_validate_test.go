package gateway

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestAGrantWithNoKindIsRefused is visibility spec §5.1's third rule, which
// the spec calls the load-bearing one.
//
// proto3 has no `required`, so an omitted enum arrives as the zero value and
// is INDISTINGUISHABLE from a caller that deliberately said "unspecified".
// That is the entire failure mode. Defaulting is wrong in both directions —
// default to party member and one forgotten field publishes a monster's stat
// block to the whole table; default to non-party and one forgotten field
// drops a character out of its own party's roster — and §5.1's migration rule
// cannot rescue either, because it cannot tell a log written before this
// field existed from a grant issued today that forgot.
//
// So the answer is a refusal, and it has to be here rather than in the fold:
// the fold must keep accepting every kindless grant already recorded
// (internal/engine's TestAKindlessGrantDoesNotEraseAKindAlreadyDeclared).
func TestAGrantWithNoKindIsRefused(t *testing.T) {
	cmd := &vttv1.GrantActorControl{ActorId: "act-archer", ParticipantId: "p-2"}
	if k := cmd.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatalf("fixture check: this command must state NO kind, got %v", k)
	}

	err := validateGrantActorControl(cmd)
	if err == nil {
		t.Fatal("a grant that does not say what it is granting must be refused, not defaulted")
	}
	// The refusal has to teach the rule, because the only reader who can act
	// on it is the caller who forgot: an LLM DM reading a tool result, or a
	// human at the DM console. "invalid argument" tells neither what to do.
	if !strings.Contains(err.Error(), "kind") {
		t.Errorf("refusal %q never names the field that is missing", err)
	}
}

// TestAGrantThatSaysWhatItIsGrantingIsAccepted is the control, and it is not
// decoration: a validator that refused everything would pass the test above
// while making the DM console and the agent's grant tool both dead, which is
// the exact failure grant_actor_control already shipped once
// (TestEveryClientCommandConverts' own doc comment).
func TestAGrantThatSaysWhatItIsGrantingIsAccepted(t *testing.T) {
	for _, kind := range []vttv1.ActorKind{
		vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	} {
		t.Run(kind.String(), func(t *testing.T) {
			cmd := &vttv1.GrantActorControl{
				ActorId: "act-archer", ParticipantId: "p-2", Kind: kind}
			if err := validateGrantActorControl(cmd); err != nil {
				t.Fatalf("a grant stating %v was refused: %v", kind, err)
			}
		})
	}
}

// TestEveryActorKindTheContractOffersIsAcceptedByAGrant keeps the validator
// honest as the enum GROWS. ActorKind's own doc comment says a third value is
// foreseeable — a neutral, a familiar, a summon — and that every future value
// is "not a party member" to the roster rule until something deliberately
// says otherwise. A validator written as `kind == PARTY_MEMBER || kind ==
// NON_PARTY` would then refuse a value the contract offers, silently making
// the new case unusable rather than merely conservative. This iterates the
// descriptor, so a new value joins the gate by existing.
func TestEveryActorKindTheContractOffersIsAcceptedByAGrant(t *testing.T) {
	values := vttv1.ActorKind(0).Descriptor().Values()
	// A loop over a descriptor can run zero times and still report ok, which
	// is the "passes against nothing" shape this file's other tests guard
	// against explicitly. Three is what the enum holds today; a GROWN enum
	// still satisfies this, a broken descriptor lookup does not.
	if values.Len() < 3 {
		t.Fatalf("fixture check: ActorKind should offer UNSPECIFIED and at least two real "+
			"values, got %d — this loop would otherwise assert nothing", values.Len())
	}
	for i := range values.Len() {
		v := values.Get(i)
		kind := vttv1.ActorKind(v.Number())
		if kind == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			continue // the one value that is refused, above
		}
		t.Run(string(v.Name()), func(t *testing.T) {
			cmd := &vttv1.GrantActorControl{
				ActorId: "act-archer", ParticipantId: "p-2", Kind: kind}
			if err := validateGrantActorControl(cmd); err != nil {
				t.Fatalf("the contract offers %v and a grant stating it was refused: %v", kind, err)
			}
		})
	}
}
