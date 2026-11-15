package engine_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func resourceAdded(seq int64, actorID, name string, value *vttv1.Resource) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_ResourceAdded{ResourceAdded: &vttv1.ResourceAdded{
			ActorId: actorID, Resource: name, Value: value}}}
}

func resourceRemoved(seq int64, actorID, name string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_ResourceRemoved{ResourceRemoved: &vttv1.ResourceRemoved{
			ActorId: actorID, Resource: name}}}
}

// sameExceptResources fails unless the snapshot after is the snapshot before
// with a1's resources replaced by want (sameExcept walks the rest).
func sameExceptResources(t *testing.T, before, after *engine.State, want map[string]*vttv1.Resource) {
	t.Helper()
	got := after.Actors["a1"].GetResources()
	if len(got) != len(want) {
		t.Errorf("a1 holds %v, want %v", got, want)
	}
	for name, w := range want {
		if !proto.Equal(got[name], w) {
			t.Errorf("a1's %q is %v, want %v", name, got[name], w)
		}
	}
	sameExcept(t, before, after, "resources")
}

// VTT-769 VTT-773 VTT-792
func TestAResourceAddedGivesTheActorTheResourceAtItsValueAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	held := st.Actors["a1"].GetResources()
	must(t, engine.Apply(st, resourceAdded(9, "a1", "focus", &vttv1.Resource{Current: 0, Max: 2})))
	sameExceptResources(t, before, st.Snapshot(), map[string]*vttv1.Resource{
		"stamina": {Current: 3, Max: 5}, "focus": {Current: 0, Max: 2}})
	if _, wrote := held["focus"]; wrote {
		t.Error("a resource map read before the add now holds focus: the add wrote into it")
	}
}

// VTT-769
func TestAResourceAddedWithNoMaxHoldsNone(t *testing.T) {
	st := abilityTable(t)
	must(t, engine.Apply(st, resourceAdded(9, "a2", "focus", &vttv1.Resource{Current: 4})))
	if got := st.Actors["a2"].GetResources()["focus"]; !proto.Equal(got, &vttv1.Resource{Current: 4}) {
		t.Errorf("a2's focus is %v, want current 4 and no max", got)
	}
}

// VTT-771 VTT-773 VTT-792
func TestAResourceRemovedTakesTheResourceFromTheActorAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	held := st.Actors["a1"].GetResources()
	must(t, engine.Apply(st, resourceRemoved(9, "a1", "stamina")))
	sameExceptResources(t, before, st.Snapshot(), map[string]*vttv1.Resource{})
	if _, kept := held["stamina"]; !kept {
		t.Error("a resource map read before the removal lost stamina: the removal wrote into it")
	}
}

// VTT-769 VTT-771
func TestAResourceRemovedThenAddedAgainHoldsTheNewValue(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, resourceRemoved(9, "a1", "stamina")))
	must(t, engine.Apply(st, resourceAdded(10, "a1", "stamina", &vttv1.Resource{Current: 1, Max: 1})))
	sameExceptResources(t, before, st.Snapshot(), map[string]*vttv1.Resource{"stamina": {Current: 1, Max: 1}})
}

// refusedAsItWas applies env, wants exactly msg, and wants st as it was.
func refusedAsItWas(t *testing.T, st *engine.State, env *vttv1.Envelope, msg string) {
	t.Helper()
	before := st.Snapshot()
	err := engine.Apply(st, env)
	if err == nil || err.Error() != msg {
		t.Fatalf("Apply = %v, want %q", err, msg)
	}
	sameExceptResources(t, before, st.Snapshot(), before.Actors["a1"].GetResources())
}

// VTT-770
func TestAResourceAddedOfAnEmptyOrOverlongNameIsRefusedFirst(t *testing.T) {
	st := abilityTable(t)
	value := &vttv1.Resource{Current: 1}
	refusedAsItWas(t, st, resourceAdded(9, "ghost", "", nil), "engine: resource name must be 1-128 bytes, got 0")
	long := strings.Repeat("r", 129)
	refusedAsItWas(t, st, resourceAdded(9, "a1", long, value), "engine: resource name must be 1-128 bytes, got 129")
	must(t, engine.Apply(st, resourceAdded(9, "a1", strings.Repeat("r", 128), value)))
}

// VTT-770
func TestAResourceAddedWithNoValueIsRefusedBeforeItsActor(t *testing.T) {
	st := abilityTable(t)
	refusedAsItWas(t, st, resourceAdded(9, "a1", "focus", nil), "engine: resource_added requires a value")
	refusedAsItWas(t, st, resourceAdded(9, "ghost", "focus", nil), "engine: resource_added requires a value")
}

// VTT-770
func TestAResourceAddedNamingNoActorOrAHeldResourceIsRefused(t *testing.T) {
	st := abilityTable(t)
	value := &vttv1.Resource{Current: 1, Max: 1}
	refusedAsItWas(t, st, resourceAdded(9, "ghost", "focus", value), `engine: resource_added names unknown actor "ghost"`)
	refusedAsItWas(t, st, resourceAdded(9, "a1", "stamina", value), `engine: actor "a1" already holds resource "stamina"`)
}

// VTT-772
func TestAResourceRemovedNamingNoActorOrNoHeldResourceIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedAsItWas(t, st, resourceRemoved(9, "ghost", "stamina"), `engine: resource_removed names unknown actor "ghost"`)
	refusedAsItWas(t, st, resourceRemoved(9, "a2", "stamina"), `engine: actor "a2" does not hold resource "stamina"`)
	must(t, engine.Apply(st, resourceRemoved(9, "a1", "stamina")))
	refusedAsItWas(t, st, resourceRemoved(10, "a1", "stamina"), `engine: actor "a1" does not hold resource "stamina"`)
}
