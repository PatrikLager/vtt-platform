package gateway

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestAddActorSeedingAControllerIsRefused closes the last route by which an
// actor became a party member with NOBODY HAVING SAID SO (visibility spec
// §5.1). Not the same as making that section's first sentence literally true —
// see TestAddActorMayStillDeclareAPartyMemberWithoutGrantingIt below.
//
// An actor created by add_actor with a controller_id and no kind had never
// been granted, carried no kind, and WAS a party member anyway, because
// §5.1's migration rule as it then stood read "no kind + a controller" as one.
// Both halves of that are gone in the same round: the rule is deleted (an
// absent kind is not a party member, always) and the shape is refused here AND
// in the fold (internal/engine/apply.go's ActorAdded arm).
//
// A REFUSAL RATHER THAN A KIND FIELD ON THE COMMAND. The alternative — let
// add_actor seed a controller as long as it also states a kind — keeps two
// commands able to confer control, and therefore two rules that must agree
// forever. RPTool shipped that shape and carries the resulting invariant bug
// in its tree today: clearAllOwners leaves ownerType == OWNER_TYPE_ALL, worked
// around at EditTokenDialog.java:885. One conferring command cannot disagree
// with itself.
func TestAddActorSeedingAControllerIsRefused(t *testing.T) {
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{
		ActorId: "act-hollis", Name: "Hollis Ketch", ControllerId: "p-2"}}

	err := validateAddActor(cmd)
	if err == nil {
		t.Fatal("add_actor no longer confers control: an actor declaring a controller_id " +
			"must be refused, not created with it")
	}
	// The refusal has to teach the two-step, because the only reader who can
	// act on it is the caller who tried the one-step: an LLM DM reading a tool
	// result, or a human at the DM console. Naming the replacement command is
	// the whole difference between a wall and a door.
	if !strings.Contains(err.Error(), "grant_actor_control") {
		t.Errorf("refusal %q never names the command that DOES confer control", err)
	}
}

// TestAddActorSeedingAControllerSetIsRefused is the same rule through the
// other spelling, and it is not a duplicate: controller_ids is the
// AUTHORITATIVE field (Actor.controller_ids' own doc comment) and
// controller_id is its mirror, so a check written against the mirror alone
// leaves the real door open. The wire has both because history has both.
func TestAddActorSeedingAControllerSetIsRefused(t *testing.T) {
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{
		ActorId: "act-hollis", Name: "Hollis Ketch", ControllerIds: []string{"p-2"}}}
	if cmd.GetActor().GetControllerId() != "" {
		t.Fatal("fixture check: this actor must carry ONLY the set, or it proves nothing " +
			"about the set")
	}

	if err := validateAddActor(cmd); err == nil {
		t.Fatal("controller_ids is the authoritative field; seeding it must be refused too")
	}
}

// TestAddActorSeedingAnEmptyControllerSetIsRefused pins the boundary rather
// than leaving it to be discovered. `controller_ids: [""]` is a NON-EMPTY set
// of nothing — an empty id names no participant — so a reader could argue it
// confers no control and should pass. It is refused anyway: a caller who wrote
// that field meant to seed a controller, and the answer to "you cannot seed a
// controller here" must not depend on whether the id they chose happened to be
// usable. The fold refuses it on the same terms; it used to STRIP such ids
// instead, and that stripping is what this replaced.
func TestAddActorSeedingAnEmptyControllerSetIsRefused(t *testing.T) {
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{
		ActorId: "act-hollis", ControllerIds: []string{""}}}
	if err := validateAddActor(cmd); err == nil {
		t.Fatal("a declared-but-empty controller set must be refused, not silently stripped")
	}
}

// TestAddActorWithoutAControllerIsAccepted is the control, and it is not
// decoration: add_actor STAYS (Patrik's ruling, 2026-08-24). Authored
// characters live in the campaign file; add_actor is the runtime path for the
// creature nobody wrote in advance — an LLM DM inventing a bandit when the
// party goes somewhere unplanned. A validator that refused everything would
// pass both tests above while deleting that path, which is the exact shape of
// failure grant_actor_control already shipped once (TestEveryClientCommandConverts).
func TestAddActorWithoutAControllerIsAccepted(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor *vttv1.Actor
	}{
		{"a bandit the DM just invented", &vttv1.Actor{ActorId: "act-bandit", Name: "Bandit"}},
		{"one that says what it is", &vttv1.Actor{ActorId: "act-bandit", Name: "Bandit",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		{"an empty controller_id, which is what protojson sends for none",
			&vttv1.Actor{ActorId: "act-bandit", ControllerId: ""}},
		{"an empty controller set, which is what an omitted repeated field is",
			&vttv1.Actor{ActorId: "act-bandit", ControllerIds: []string{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAddActor(&vttv1.AddActor{Actor: tc.actor}); err != nil {
				t.Fatalf("add_actor is the runtime creation path and must still work: %v", err)
			}
		})
	}
}

// TestAddActorMayStillDeclareAPartyMemberWithoutGrantingIt pins the ONE case
// this refusal deliberately does not cover, so that it is a decision on the
// record rather than a gap somebody finds later.
//
// §5.1's first rule reads "an ungranted actor is NOT a party member". After
// this change nothing INFERS party membership — not control, not anything —
// which is what that rule's own justification asks for. But Actor.kind is
// still a field on the actor, so a caller who types the words creates an
// ungranted party member. Task 2 kept the field on purpose: the pregen case
// needs it, and the DM console's paste path round-trips a whole Actor through
// fromJson. Removing it was not part of the 2026-08-24 ruling.
//
// So the sentence is not literally true and the SHAPE is not a leak: the
// information is present, stated by whoever created the actor, which is the
// whole thing §5.1's revision asks for ("ask, and each case states its own
// answer"). Flagged in validateAddActor's doc comment as the remaining work if
// the sentence is meant literally.
func TestAddActorMayStillDeclareAPartyMemberWithoutGrantingIt(t *testing.T) {
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{
		ActorId: "act-hollis", Name: "Hollis Ketch",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}
	if err := validateAddActor(cmd); err != nil {
		t.Fatalf("an actor may still say what it is at creation: %v", err)
	}
	if len(cmd.GetActor().GetControllerIds()) != 0 || cmd.GetActor().GetControllerId() != "" {
		t.Fatal("fixture check: it must confer no CONTROL, or this pins the wrong thing")
	}
}

// TestAddActorWithNoActorAtAllIsNotRefusedHere pins the seam rather than the
// behaviour. A nil Actor is engine.Apply's error to raise ("actor_added
// requires an actor with an id"), and this validator answering first would
// move that refusal to a second place with a second wording. The nil-receiver
// getters make the check safe rather than merely lucky, and this test is what
// says the safety is deliberate.
func TestAddActorWithNoActorAtAllIsNotRefusedHere(t *testing.T) {
	if err := validateAddActor(&vttv1.AddActor{}); err != nil {
		t.Fatalf("a missing actor is the fold's refusal to make, not this one's: %v", err)
	}
	if err := validateAddActor(nil); err != nil {
		t.Fatalf("a nil command must read as no controller declared, not panic or refuse: %v", err)
	}
}
