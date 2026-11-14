package perceive

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// stamped numbers envs from seq on and stamps each with the issuer a log
// carries, so a forwarded frame can be told from the event.
func stamped(seq int64, envs ...*vttv1.Envelope) []*vttv1.Envelope {
	for i, env := range envs {
		env.EventId, env.Sequence, env.SessionId = "e", seq+int64(i), "sess-1"
		env.ActorRole, env.ParticipantId = "dm", "p-dm"
	}
	return envs
}

// abilityLog is a table of two scenes: the hero, its player p-1's goblin
// and the rogue, p-2's, stand together in near; the far scene is empty.
func abilityLog() []*vttv1.Envelope {
	party, creature := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	scene := func(id string) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: id, GridWidth: 4, GridHeight: 1}}}
	}
	actor := func(a *vttv1.Actor) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
	}
	place := func(token, actor string, x int32) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: token, SceneId: "near", ActorId: actor, Position: &vttv1.GridPosition{X: x}}}}
	}
	grant := func(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actor, ParticipantId: participant, Kind: kind}}}
	}
	return stamped(1,
		&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "n"}}},
		scene("near"), scene("far"),
		actor(&vttv1.Actor{ActorId: "hero", Name: "Hero", Kind: party, AbilityIds: []string{"rally"}}),
		actor(&vttv1.Actor{ActorId: "rogue", Name: "Rogue", Kind: party, AbilityIds: []string{"rally", "lunge"},
			Resources: map[string]*vttv1.Resource{"pool": {Current: 5, Max: 5}}}),
		actor(&vttv1.Actor{ActorId: "goblin", Name: "Goblin", Kind: creature, AbilityIds: []string{"scimitar"}}),
		place("t-hero", "hero", 0), place("t-rogue", "rogue", 1), place("t-gob", "goblin", 2),
		grant("hero", "p-1", party), grant("goblin", "p-1", creature), grant("rogue", "p-2", party),
	)
}

// rogueGoesFar takes the rogue out of the near scene, out of the hero's sight.
func rogueGoesFar(seq int64) []*vttv1.Envelope {
	return stamped(seq,
		&vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "t-rogue"}}},
		&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "t-rogue-far", SceneId: "far", ActorId: "rogue", Position: &vttv1.GridPosition{}}}})
}

func giveTo(seq int64, actor, ability string) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{
		ActorId: actor, AbilityId: ability}}})[0]
}

func takeFrom(seq int64, actor, ability string) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{
		ActorId: actor, AbilityId: ability}}})[0]
}

// verdictAt projects log for eyes as a seat does, folding what it is sent,
// then folds change into the world and returns classify's ruling on it.
func verdictAt(t *testing.T, eyes Eyes, log []*vttv1.Envelope, change *vttv1.Envelope) verdict {
	t.Helper()
	world, shown := engine.NewState(), engine.NewState()
	pr := New(eyes, nil)
	for _, env := range log {
		if err := engine.Apply(world, env); err != nil {
			t.Fatalf("the world refuses seq %d: %v", env.GetSequence(), err)
		}
		for _, f := range pr.Event(eyes, env, shown, world) {
			if err := engine.Apply(shown, f); err != nil {
				t.Fatalf("frame %T of seq %d does not fold: %v", f.GetPayload(), env.GetSequence(), err)
			}
		}
	}
	if err := engine.Apply(world, change); err != nil {
		t.Fatalf("the world refuses the change: %v", err)
	}
	pr.through = eyes
	pr.recall(shown, world)
	return pr.classify(change, pr.look(world))
}

// changesOf are a give and a take of actor's list, as the log goes on at seq.
func changesOf(seq int64, actor, given, taken string) map[string]*vttv1.Envelope {
	return map[string]*vttv1.Envelope{"a give": giveTo(seq, actor, given), "a take": takeFrom(seq, actor, taken)}
}

// VTT-723
func TestAPartyMembersOwnLogIsSentItsGiveAndTake(t *testing.T) {
	for _, log := range map[string][]*vttv1.Envelope{
		"beside the hero": abilityLog(),
		"alone, far away": append(abilityLog(), rogueGoesFar(13)...),
	} {
		for name, change := range changesOf(20, "rogue", "dodge", "lunge") {
			if v := verdictAt(t, Eyes{Shoulder: "rogue"}, log, change); v != forwarded {
				t.Errorf("%s of the rogue is %v for the rogue's own log, want forwarded", name, v)
			}
		}
	}
}

// VTT-723
func TestALogWhoseEyesSawAPartyMemberIsSentItsGiveAndTake(t *testing.T) {
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Controller: "p-1"}} {
		for name, change := range changesOf(20, "rogue", "dodge", "lunge") {
			if v := verdictAt(t, eyes, abilityLog(), change); v != forwarded {
				t.Errorf("%s of the rogue in sight is %v for %+v, want forwarded", name, v, eyes)
			}
		}
	}
}

// VTT-723
func TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoGiveOrTake(t *testing.T) {
	log := append(abilityLog(), rogueGoesFar(13)...)
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Controller: "p-1"}} {
		for name, change := range changesOf(20, "rogue", "dodge", "lunge") {
			if v := verdictAt(t, eyes, log, change); v != withheld {
				t.Errorf("%s of the rogue out of sight is %v for %+v, want withheld", name, v, eyes)
			}
		}
	}
}

// VTT-723
func TestNoLogIsSentACreaturesGiveOrTake(t *testing.T) {
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Shoulder: "rogue"}, {Controller: "p-1"}} {
		for name, change := range changesOf(20, "goblin", "dodge", "scimitar") {
			if v := verdictAt(t, eyes, abilityLog(), change); v != withheld {
				t.Errorf("%s of the goblin in sight is %v for %+v, want withheld", name, v, eyes)
			}
		}
	}
}

// VTT-272
func TestAForwardedGiveOrTakeNamesItsActorAndAbilityAndNoIssuer(t *testing.T) {
	for name, change := range changesOf(20, "rogue", "dodge", "lunge") {
		want := proto.Clone(change).(*vttv1.Envelope)
		want.ParticipantId, want.ActorRole = "", ""
		if got := forwardable(change); !proto.Equal(got, want) {
			t.Errorf("%s is forwarded as %v, want %v", name, got, want)
		}
		if change.GetParticipantId() != "p-dm" {
			t.Errorf("forwardable wrote into %s it was handed: %v", name, change)
		}
	}
}
