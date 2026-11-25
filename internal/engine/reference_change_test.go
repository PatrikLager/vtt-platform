package engine_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func referenceAdded(seq int64, actorID, id string, ref *vttv1.Reference) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_ReferenceAdded{ReferenceAdded: &vttv1.ReferenceAdded{
			ActorId: actorID, ReferenceId: id, Reference: ref}}}
}

func referenceRemoved(seq int64, actorID, id string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_ReferenceRemoved{ReferenceRemoved: &vttv1.ReferenceRemoved{
			ActorId: actorID, ReferenceId: id}}}
}

func referenceChanged(seq int64, actorID, id, place, state string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_ReferenceChanged{ReferenceChanged: &vttv1.ReferenceChanged{
			ActorId: actorID, ReferenceId: id, Place: place, State: state}}}
}

// sameReferences fails unless got holds exactly want, each reference equal.
func sameReferences(t *testing.T, who string, got, want map[string]*vttv1.Reference) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s holds %v, want %v", who, got, want)
	}
	for id, w := range want {
		if !proto.Equal(got[id], w) {
			t.Errorf("%s's %q is %v, want %v", who, id, got[id], w)
		}
	}
}

// sameExceptReferences fails unless the snapshot after is the snapshot before
// with a1's references replaced by want (sameExcept walks the rest).
func sameExceptReferences(t *testing.T, before, after *engine.State, want map[string]*vttv1.Reference) {
	t.Helper()
	sameReferences(t, "a1", after.Actors["a1"].GetReferences(), want)
	sameExcept(t, before, after, "references")
}

// refusedAsBefore applies env, wants exactly msg, and wants st as it was in
// every field of every actor.
func refusedAsBefore(t *testing.T, st *engine.State, env *vttv1.Envelope, msg string) {
	t.Helper()
	before := st.Snapshot()
	err := engine.Apply(st, env)
	if err == nil || err.Error() != msg {
		t.Fatalf("Apply = %v, want %q", err, msg)
	}
	sameExcept(t, before, st.Snapshot(), "")
}

// VTT-1023
func TestAnActorAddedKeepsItsReferencesWhole(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	refs := map[string]*vttv1.Reference{
		"r1": {EntryId: "echo", Place: "mike", State: "oscar"},
		"r2": {EntryId: "echo", Place: "mike", State: "oscar"},
		"r3": {EntryId: "delta", Place: "kilo"},
	}
	added := env(9, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a3", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		References: refs}})
	must(t, engine.Apply(st, added))
	want := map[string]*vttv1.Reference{
		"r1": {EntryId: "echo", Place: "mike", State: "oscar"},
		"r2": {EntryId: "echo", Place: "mike", State: "oscar"},
		"r3": {EntryId: "delta", Place: "kilo"},
	}
	sameReferences(t, "a3", st.Actors["a3"].GetReferences(), want)
	refs["r1"].Place, refs["r4"] = "kilo", &vttv1.Reference{EntryId: "delta"}
	sameReferences(t, "a3, after the event's map was written", st.Actors["a3"].GetReferences(), want)
	after := st.Snapshot()
	delete(after.Actors, "a3")
	sameExcept(t, before, after, "")
}

// VTT-1023
func TestAnActorAddedHoldingAReferenceWithAnEmptyIdIsRefusedAfterItsAttributeNames(t *testing.T) {
	st := abilityTable(t)
	empty := map[string]*vttv1.Reference{"r1": {EntryId: "echo", Place: "mike"}, "": {EntryId: "delta", Place: "kilo"}}
	refusedAsBefore(t, st, env(9, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a3", References: empty}}),
		`engine: actor_added: actor "a3" holds a reference with an empty id`)
	refusedAsBefore(t, st, env(9, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a3",
		Attributes: map[string]int32{"": 1}, References: empty}}),
		"engine: attribute name must be 1-128 bytes, got 0")
	must(t, engine.Apply(st, env(9, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a3",
		References: map[string]*vttv1.Reference{"r": {}}}})))
}

// VTT-1024
func TestAReferenceAddedGivesTheSheetTheReferenceAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	held := st.Actors["a1"].GetReferences()
	ref := &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}
	must(t, engine.Apply(st, referenceAdded(9, "a1", "r3", ref)))
	want := map[string]*vttv1.Reference{
		"r1": {EntryId: "echo", Place: "mike", State: "lima"},
		"r2": {EntryId: "delta", Place: "kilo"},
		"r3": {EntryId: "echo", Place: "mike", State: "oscar"},
	}
	sameExceptReferences(t, before, st.Snapshot(), want)
	if _, wrote := held["r3"]; wrote {
		t.Error("a reference map read before the add now holds r3: the add wrote into it")
	}
	ref.Place, ref.State = "kilo", ""
	sameExceptReferences(t, before, st.Snapshot(), want)
}

// VTT-1024
func TestAReferenceAddedToASheetHoldingNoneHoldsIt(t *testing.T) {
	st := abilityTable(t)
	must(t, engine.Apply(st, referenceAdded(9, "a2", "r1", &vttv1.Reference{EntryId: "delta", Place: "kilo"})))
	sameReferences(t, "a2", st.Actors["a2"].GetReferences(),
		map[string]*vttv1.Reference{"r1": {EntryId: "delta", Place: "kilo"}})
}

// VTT-1024
func TestAReferenceAddedWithAnEmptyIdIsRefusedFirst(t *testing.T) {
	st := abilityTable(t)
	refusedAsBefore(t, st, referenceAdded(9, "ghost", "", nil), "engine: reference_added requires a reference id")
	refusedAsBefore(t, st, referenceAdded(9, "a1", "", &vttv1.Reference{EntryId: "echo", Place: "mike"}),
		"engine: reference_added requires a reference id")
}

// VTT-1024
func TestAReferenceAddedWithNoReferenceIsRefusedBeforeItsActor(t *testing.T) {
	st := abilityTable(t)
	refusedAsBefore(t, st, referenceAdded(9, "ghost", "r1", nil), "engine: reference_added requires a reference")
	refusedAsBefore(t, st, referenceAdded(9, "a2", "r1", nil), "engine: reference_added requires a reference")
}

// VTT-1024
func TestAReferenceAddedNamingNoActorOrAnIdTheSheetHoldsIsRefused(t *testing.T) {
	st := abilityTable(t)
	ref := &vttv1.Reference{EntryId: "delta", Place: "kilo"}
	refusedAsBefore(t, st, referenceAdded(9, "ghost", "r1", ref), `engine: reference_added names unknown actor "ghost"`)
	refusedAsBefore(t, st, referenceAdded(9, "a1", "r2", ref), `engine: actor "a1" already holds reference "r2"`)
	refusedAsBefore(t, st, referenceAdded(9, "a1", "r1", ref), `engine: actor "a1" already holds reference "r1"`)
}

// VTT-1025
func TestAReferenceRemovedTakesTheReferenceOffTheSheetAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	held := st.Actors["a1"].GetReferences()
	must(t, engine.Apply(st, referenceRemoved(9, "a1", "r1")))
	sameExceptReferences(t, before, st.Snapshot(), map[string]*vttv1.Reference{"r2": {EntryId: "delta", Place: "kilo"}})
	if _, kept := held["r1"]; !kept {
		t.Error("a reference map read before the removal lost r1: the removal wrote into it")
	}
	must(t, engine.Apply(st, referenceRemoved(10, "a1", "r2")))
	sameExceptReferences(t, before, st.Snapshot(), map[string]*vttv1.Reference{})
}

// VTT-1024 VTT-1025
func TestAReferenceRemovedThenAddedAgainHoldsTheNewReference(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, referenceRemoved(9, "a1", "r1")))
	must(t, engine.Apply(st, referenceAdded(10, "a1", "r1", &vttv1.Reference{EntryId: "delta", Place: "mike", State: "oscar"})))
	sameExceptReferences(t, before, st.Snapshot(), map[string]*vttv1.Reference{
		"r1": {EntryId: "delta", Place: "mike", State: "oscar"},
		"r2": {EntryId: "delta", Place: "kilo"},
	})
}

// VTT-1025
func TestAReferenceRemovedNamingNoActorOrAnIdTheSheetDoesNotHoldIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedAsBefore(t, st, referenceRemoved(9, "ghost", "r1"), `engine: reference_removed names unknown actor "ghost"`)
	refusedAsBefore(t, st, referenceRemoved(9, "a2", "r1"), `engine: actor "a2" does not hold reference "r1"`)
	refusedAsBefore(t, st, referenceRemoved(9, "a1", "r9"), `engine: actor "a1" does not hold reference "r9"`)
	must(t, engine.Apply(st, referenceRemoved(9, "a1", "r1")))
	refusedAsBefore(t, st, referenceRemoved(10, "a1", "r1"), `engine: actor "a1" does not hold reference "r1"`)
}

// VTT-1026
func TestAReferenceChangedSetsItsPlaceAndStateAndKeepsItsEntry(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	held := st.Actors["a1"].GetReferences()
	must(t, engine.Apply(st, referenceChanged(9, "a1", "r1", "kilo", "")))
	sameExceptReferences(t, before, st.Snapshot(), map[string]*vttv1.Reference{
		"r1": {EntryId: "echo", Place: "kilo"},
		"r2": {EntryId: "delta", Place: "kilo"},
	})
	if got, want := held["r1"], (&vttv1.Reference{EntryId: "echo", Place: "mike", State: "lima"}); !proto.Equal(got, want) {
		t.Errorf("a reference map read before the change reads r1 as %v, want %v: the change wrote into it", got, want)
	}
	must(t, engine.Apply(st, referenceChanged(10, "a1", "r2", "mike", "oscar")))
	sameExceptReferences(t, before, st.Snapshot(), map[string]*vttv1.Reference{
		"r1": {EntryId: "echo", Place: "kilo"},
		"r2": {EntryId: "delta", Place: "mike", State: "oscar"},
	})
}

// VTT-1026
func TestAReferenceChangedNamingNoActorOrAnIdTheSheetDoesNotHoldIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedAsBefore(t, st, referenceChanged(9, "ghost", "r1", "kilo", ""), `engine: reference_changed names unknown actor "ghost"`)
	refusedAsBefore(t, st, referenceChanged(9, "a2", "r1", "kilo", ""), `engine: actor "a2" does not hold reference "r1"`)
	refusedAsBefore(t, st, referenceChanged(9, "a1", "r9", "kilo", ""), `engine: actor "a1" does not hold reference "r9"`)
	must(t, engine.Apply(st, referenceRemoved(9, "a1", "r1")))
	refusedAsBefore(t, st, referenceChanged(10, "a1", "r1", "kilo", ""), `engine: actor "a1" does not hold reference "r1"`)
}
