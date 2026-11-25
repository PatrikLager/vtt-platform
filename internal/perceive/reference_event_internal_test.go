package perceive

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func refAdd(seq int64, actor, id string, ref *vttv1.Reference) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceAdded{ReferenceAdded: &vttv1.ReferenceAdded{
		ActorId: actor, ReferenceId: id, Reference: ref}}})[0]
}

func refRemove(seq int64, actor, id string) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceRemoved{ReferenceRemoved: &vttv1.ReferenceRemoved{
		ActorId: actor, ReferenceId: id}}})[0]
}

func refChange(seq int64, actor, id, place, state string) *vttv1.Envelope {
	return stamped(seq, &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceChanged{ReferenceChanged: &vttv1.ReferenceChanged{
		ActorId: actor, ReferenceId: id, Place: place, State: state}}})[0]
}

// referenceLog is abilityLog with r1 on the rogue's sheet and on the goblin's.
func referenceLog() []*vttv1.Envelope {
	return append(abilityLog(),
		refAdd(13, "rogue", "r1", &vttv1.Reference{EntryId: "echo", Place: "mike", State: "lima"}),
		refAdd(14, "goblin", "r1", &vttv1.Reference{EntryId: "delta", Place: "kilo"}))
}

// referenceChangesOf are an addition of r2 to actor, a removal of its r1 and a
// change of its r1, as the log goes on at seq.
func referenceChangesOf(seq int64, actor string) map[string]*vttv1.Envelope {
	return map[string]*vttv1.Envelope{
		"an addition": refAdd(seq, actor, "r2", &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}),
		"a removal":   refRemove(seq, actor, "r1"),
		"a change":    refChange(seq, actor, "r1", "kilo", ""),
	}
}

// VTT-1032
func TestAPartyMembersOwnLogIsSentItsReferenceEvents(t *testing.T) {
	for _, log := range map[string][]*vttv1.Envelope{
		"beside the hero": referenceLog(),
		"alone, far away": append(referenceLog(), rogueGoesFar(15)...),
	} {
		for name, change := range referenceChangesOf(20, "rogue") {
			if v := verdictAt(t, Eyes{Shoulder: "rogue"}, log, change); v != forwarded {
				t.Errorf("%s of the rogue's reference is %v for the rogue's own log, want forwarded", name, v)
			}
		}
	}
}

// VTT-1032
func TestALogWhoseEyesSawAPartyMemberIsSentItsReferenceEvents(t *testing.T) {
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Controller: "p-1"}} {
		for name, change := range referenceChangesOf(20, "rogue") {
			if v := verdictAt(t, eyes, referenceLog(), change); v != forwarded {
				t.Errorf("%s of the rogue's reference in sight is %v for %+v, want forwarded", name, v, eyes)
			}
		}
	}
}

// VTT-1032
func TestALogWhoseEyesDidNotSeeAPartyMemberIsSentNoReferenceEvent(t *testing.T) {
	log := append(referenceLog(), rogueGoesFar(15)...)
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Controller: "p-1"}} {
		for name, change := range referenceChangesOf(20, "rogue") {
			if v := verdictAt(t, eyes, log, change); v != withheld {
				t.Errorf("%s of the rogue's reference out of sight is %v for %+v, want withheld", name, v, eyes)
			}
		}
	}
}

// VTT-1032
func TestNoLogIsSentACreaturesReferenceEvent(t *testing.T) {
	for _, eyes := range []Eyes{{Shoulder: "hero"}, {Shoulder: "rogue"}, {Controller: "p-1"}} {
		for name, change := range referenceChangesOf(20, "goblin") {
			if v := verdictAt(t, eyes, referenceLog(), change); v != withheld {
				t.Errorf("%s of the goblin's reference in sight is %v for %+v, want withheld", name, v, eyes)
			}
		}
	}
}

// VTT-272
func TestAForwardedReferenceEventKeepsItsPayloadAndNamesNoIssuer(t *testing.T) {
	for name, change := range referenceChangesOf(20, "rogue") {
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
