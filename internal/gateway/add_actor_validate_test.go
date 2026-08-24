package gateway

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestAddActorSeedingAControllerIsRefused closes the route by which an actor
// became a party member with NOBODY HAVING SAID SO (visibility spec §5.1) —
// the CONTROL half. The other half is that creation must state a kind at all
// (TestAddActorWithNoKindIsRefused below), and together they mean every actor
// is born with someone's word on what it is and nobody's hand on it.
//
// An actor created by add_actor with a controller_id and no kind had never
// been granted, carried no kind, and WAS a party member anyway, because
// §5.1's migration rule as it then stood read "no kind + a controller" as one.
// Both halves of that are gone in the same round: the rule is deleted (an
// absent kind is not a party member, always) and the shape is refused here AND
// in the fold (internal/engine/apply.go's ActorAdded arm).
//
// THE ACTOR HERE STATES A KIND, and that is load-bearing rather than tidy.
// Since 2026-08-24 a kindless add_actor is refused on its own terms
// (TestAddActorWithNoKindIsRefused), so a fixture that omitted the kind would
// still get an error with the controller check DELETED — this test would go on
// passing while pinning nothing. Stating it leaves the controller as the only
// thing wrong with this command.
func TestAddActorSeedingAControllerIsRefused(t *testing.T) {
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{
		ActorId: "act-hollis", Name: "Hollis Ketch", ControllerId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}

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
		ActorId: "act-hollis", Name: "Hollis Ketch", ControllerIds: []string{"p-2"},
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}
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
		ActorId: "act-hollis", ControllerIds: []string{""},
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}
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
//
// Every case here STATES A KIND, because after 2026-08-24 that is what
// creating an actor means (TestAddActorWithNoKindIsRefused below). Both values
// appear: a validator that accepted only PARTY_MEMBER would pass a
// single-valued version of this test while making every monster unaddable.
func TestAddActorWithoutAControllerIsAccepted(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor *vttv1.Actor
	}{
		{"a bandit the DM just invented", &vttv1.Actor{ActorId: "act-bandit", Name: "Bandit",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		{"a pregenerated character nobody is assigned to yet",
			&vttv1.Actor{ActorId: "act-hollis", Name: "Hollis Ketch",
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		{"an empty controller_id, which is what protojson sends for none",
			&vttv1.Actor{ActorId: "act-bandit", ControllerId: "",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		{"an empty controller set, which is what an omitted repeated field is",
			&vttv1.Actor{ActorId: "act-bandit", ControllerIds: []string{},
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAddActor(&vttv1.AddActor{Actor: tc.actor}); err != nil {
				t.Fatalf("add_actor is the runtime creation path and must still work: %v", err)
			}
		})
	}
}

// TestAddActorWithNoKindIsRefused is Task 7, and it corrects the argument the
// three tasks before it were justified by.
//
// Those tasks reasoned "one writer beats two" and closed every door but one.
// That was a proxy and slightly wrong. THE DANGER WAS NEVER MULTIPLICITY — IT
// WAS A WRITER THAT COULD STAY SILENT. The archer leak came from inference
// from absence: an actor holding a controller with no kind had to be guessed
// at, and the guess was wrong. Two writers are fine as long as neither can be
// mute, which is what this refusal makes true of the second one.
//
// Patrik, 2026-08-24: "when you create an actor you should know what for — is
// it an NPC or a PC. So add_actor should have kind defined."
func TestAddActorWithNoKindIsRefused(t *testing.T) {
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: "act-bandit", Name: "Bandit"}}
	if cmd.GetActor().GetKind() != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatal("fixture check: this actor must carry NO kind, or it pins nothing")
	}

	err := validateAddActor(cmd)
	if err == nil {
		t.Fatal("creating an actor must say what it IS: an unstated kind cannot be told " +
			"from a deliberate one, which is the whole reason grant_actor_control refuses it too")
	}
	// The message must offer the two answers, for the same reason the
	// controller refusal names grant_actor_control: the only reader who can act
	// on it is the caller who just failed to answer — an LLM DM reading a tool
	// result, or a human at the DM console.
	for _, want := range []string{"ACTOR_KIND_PARTY_MEMBER", "ACTOR_KIND_NON_PARTY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q never names %s, so the caller cannot answer it", err, want)
		}
	}
}

// TestAddActorAcceptsAKindTheEnumDoesNotYetDefine pins that the check is
// UNSPECIFIED-ONLY rather than an allowlist of the two values that exist
// today — the same decision validateGrantActorControl made and for the same
// reason. ActorKind's own doc comment says a third value is foreseeable (a
// neutral, a familiar, a summon) and that every future value reads as "not a
// party member" until something deliberately says otherwise. An allowlist here
// would refuse a value the contract offers, turning additive growth into a
// broken command.
func TestAddActorAcceptsAKindTheEnumDoesNotYetDefine(t *testing.T) {
	// A value the enum does not define today. It is not UNSPECIFIED, so it is
	// not silence — the caller said something, and the readers fail it closed.
	future := vttv1.ActorKind(99)
	if future == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatal("fixture check: 99 must not be the zero value")
	}
	cmd := &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: "act-familiar", Kind: future}}
	if err := validateAddActor(cmd); err != nil {
		t.Fatalf("an unknown-but-stated kind is not silence; refusing it makes additive "+
			"enum growth a broken command: %v", err)
	}
}

// TestAddActorDeclaresAPartyMemberWithoutGrantingIt pins the case that used to
// be described here as the one gap this refusal did not cover. It is not a gap
// and never was: it is the design.
//
// An ungranted actor declaring itself a party member is a PREGENERATED
// CHARACTER — four sheets sitting in the campaign before anyone is assigned to
// one. The party seeing that those four exist is correct, not a leak: nobody
// inferred anything, the author said it. §5.1's first sentence ("an ungranted
// actor is NOT a party member") is what is wrong about that pairing, and
// Patrik is amending it; this test is the behaviour it will be amended to
// match.
//
// It still confers NO CONTROL, which is the half that stays true: creating a
// character says what it is, and a grant says who drives it. Two facts, two
// commands, and neither can be stated by silence.
func TestAddActorDeclaresAPartyMemberWithoutGrantingIt(t *testing.T) {
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
//
// IT GOT SHARPER WHEN add_actor STARTED REQUIRING A KIND. A nil Actor reads as
// UNSPECIFIED through the same getter a real kindless actor does, so the
// obvious kind check would answer here and tell a caller who sent no actor at
// all that their KIND was the problem — a refusal that misdescribes the rule
// teaches the wrong one. The kind check is therefore reached only once an
// actor is present, and this test is what holds that line.
func TestAddActorWithNoActorAtAllIsNotRefusedHere(t *testing.T) {
	if err := validateAddActor(&vttv1.AddActor{}); err != nil {
		t.Fatalf("a missing actor is the fold's refusal to make, not this one's: %v", err)
	}
	if err := validateAddActor(nil); err != nil {
		t.Fatalf("a nil command must read as no controller declared, not panic or refuse: %v", err)
	}
}

// TestAddActorWithAnActorThatHasNoIdIsNotRefusedHere is the same seam through
// the shape that actually arrives on the wire, and it is a review finding
// rather than a case somebody predicted.
//
// protojson delivers `{"addActor":{"actor":{}}}` as a NON-NIL, wholly empty
// Actor — so a nil check alone defers only `{"addActor":{}}`, and the far more
// likely mistake fell through to the kind refusal. The caller who sent an actor
// with no id at all was then told their KIND was the problem, which is the
// precise misdescription the seam exists to avoid: engine.Apply's "actor_added
// requires an actor with an id" names the actual mistake, and a second answer
// with a second wording in front of it teaches the wrong rule.
//
// A CONTROLLER IS STILL REFUSED FIRST, even with no id, and that is not an
// oversight. "Creation does not confer control" is a misunderstanding of the
// model rather than a forgotten field — worth saying to a caller whatever else
// is wrong with their command, since fixing the id alone would just produce the
// same refusal a round trip later.
func TestAddActorWithAnActorThatHasNoIdIsNotRefusedHere(t *testing.T) {
	empty := &vttv1.AddActor{Actor: &vttv1.Actor{}}
	if empty.GetActor() == nil {
		t.Fatal("fixture check: the actor must be PRESENT and empty, or this is the nil case again")
	}
	if err := validateAddActor(empty); err != nil {
		t.Fatalf("an actor with no id is the fold's refusal to make, and it names the id — "+
			"answering about the kind here teaches the wrong rule: %v", err)
	}

	// The controller rule does not defer, and the refusal still names the
	// command that confers control.
	held := &vttv1.AddActor{Actor: &vttv1.Actor{ControllerId: "p-2"}}
	err := validateAddActor(held)
	if err == nil {
		t.Fatal("a missing id does not license seeding a controller")
	}
	if !strings.Contains(err.Error(), "grant_actor_control") {
		t.Errorf("refusal %q never names the command that DOES confer control", err)
	}
}
