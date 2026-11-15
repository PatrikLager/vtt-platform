package perceive

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func addTo(seq int64, actor, resource string, value *vttv1.Resource) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceAdded{ResourceAdded: &vttv1.ResourceAdded{
		ActorId: actor, Resource: resource, Value: value}}})[0]
}

func removeFrom(seq int64, actor, resource string) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceRemoved{ResourceRemoved: &vttv1.ResourceRemoved{
		ActorId: actor, Resource: resource}}})[0]
}

// resourceChangesOf are an addition of focus to actor and a removal of held from
// it, as the log goes on at seq.
func resourceChangesOf(seq int64, actor, held string) map[string]*vttv1.Envelope {
	return map[string]*vttv1.Envelope{
		"an addition": addTo(seq, actor, "focus", &vttv1.Resource{Current: 1, Max: 2}),
		"a removal":   removeFrom(seq, actor, held),
	}
}

// VTT-779
func TestAPartyMembersOwnLogIsSentItsResourceAddedAndRemoved(t *testing.T) {
	for _, log := range map[string][]*vttv1.Envelope{
		"beside the hero": abilityLog(),
		"alone, far away": append(abilityLog(), rogueGoesFar(13)...),
	} {
		for name, change := range resourceChangesOf(20, "rogue", "pool") {
			if v := verdictAt(t, Eyes{Shoulder: "rogue"}, log, change); v != forwarded {
				t.Errorf("%s of the rogue's resource is %v for the rogue's own log, want forwarded", name, v)
			}
		}
	}
}

// VTT-779
func TestALogWhoseEyesSawAPartyMemberIsSentItsResourceAddedAndRemoved(t *testing.T) {
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Controller: "p-1"}} {
		for name, change := range resourceChangesOf(20, "rogue", "pool") {
			if v := verdictAt(t, eyes, abilityLog(), change); v != forwarded {
				t.Errorf("%s of the rogue's resource in sight is %v for %+v, want forwarded", name, v, eyes)
			}
		}
	}
}

// VTT-779
func TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoResourceAddedOrRemoved(t *testing.T) {
	log := append(abilityLog(), rogueGoesFar(13)...)
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Controller: "p-1"}} {
		for name, change := range resourceChangesOf(20, "rogue", "pool") {
			if v := verdictAt(t, eyes, log, change); v != withheld {
				t.Errorf("%s of the rogue's resource out of sight is %v for %+v, want withheld", name, v, eyes)
			}
		}
	}
}

// VTT-779
func TestNoLogIsSentACreaturesResourceAddedOrRemoved(t *testing.T) {
	log := append(abilityLog(), addTo(13, "goblin", "pool", &vttv1.Resource{Current: 2}))
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Shoulder: "rogue"}, {Controller: "p-1"}} {
		for name, change := range resourceChangesOf(20, "goblin", "pool") {
			if v := verdictAt(t, eyes, log, change); v != withheld {
				t.Errorf("%s of the goblin's resource in sight is %v for %+v, want withheld", name, v, eyes)
			}
		}
	}
}

// VTT-272
func TestAForwardedResourceAddedOrRemovedKeepsItsPayloadAndNamesNoIssuer(t *testing.T) {
	for name, change := range resourceChangesOf(20, "rogue", "pool") {
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
