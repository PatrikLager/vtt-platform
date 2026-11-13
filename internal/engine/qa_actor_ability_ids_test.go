package engine_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Keep qaAblListed out of byte order and holding a duplicate, so that a sort
// or a set would change it.
var qaAblListed = []string{"rally", "fists", "rally", "goblin-scimitar"}

func qaAblEnv(seq int64, a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:  fmt.Sprintf("qa-e%d", seq),
		Sequence: seq,
		Payload:  &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}},
	}
}

// Keep every field an ActorAdded may carry before ability_ids set here,
// except module_data, which the client's fold does not hold, and no
// controller: both folds refuse an ActorAdded that carries one.
func qaAblRichActor() *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    "a1",
		Name:       "Ada",
		ModuleId:   "mod",
		Attributes: map[string]int32{"vigor": 3},
		Resources:  map[string]*vttv1.Resource{"stamina": {Current: 2, Max: 5}},
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		AbilityIds: slices.Clone(qaAblListed),
	}
}

func qaAblFold(t *testing.T, envs ...*vttv1.Envelope) *engine.State {
	t.Helper()
	st := engine.NewState()
	for _, env := range envs {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("Apply(%s): %v", env.GetEventId(), err)
		}
	}
	return st
}

// VTT-683
func TestQAActorDeclaresAbilityIdsAsRepeatedStringFieldTen(t *testing.T) {
	fields := (&vttv1.Actor{}).ProtoReflect().Descriptor().Fields()
	type want struct {
		name  string
		kind  protoreflect.Kind
		card  protoreflect.Cardinality
		isMap bool
	}
	declared := map[protoreflect.FieldNumber]want{
		1:  {"actor_id", protoreflect.StringKind, protoreflect.Optional, false},
		2:  {"name", protoreflect.StringKind, protoreflect.Optional, false},
		3:  {"module_id", protoreflect.StringKind, protoreflect.Optional, false},
		4:  {"attributes", protoreflect.MessageKind, protoreflect.Repeated, true},
		5:  {"resources", protoreflect.MessageKind, protoreflect.Repeated, true},
		6:  {"module_data", protoreflect.MessageKind, protoreflect.Optional, false},
		7:  {"controller_id", protoreflect.StringKind, protoreflect.Optional, false},
		8:  {"controller_ids", protoreflect.StringKind, protoreflect.Repeated, false},
		9:  {"kind", protoreflect.EnumKind, protoreflect.Optional, false},
		10: {"ability_ids", protoreflect.StringKind, protoreflect.Repeated, false},
	}
	for num, w := range declared {
		fd := fields.ByNumber(num)
		if fd == nil {
			t.Fatalf("Actor has no field %d", num)
		}
		if string(fd.Name()) != w.name || fd.Kind() != w.kind || fd.Cardinality() != w.card || fd.IsMap() != w.isMap {
			t.Errorf("field %d = %s %v %v map=%v, want %s %v %v map=%v",
				num, fd.Name(), fd.Kind(), fd.Cardinality(), fd.IsMap(), w.name, w.kind, w.card, w.isMap)
		}
	}
	ab := fields.ByNumber(10)
	if ab.JSONName() != "abilityIds" {
		t.Errorf("JSONName = %q, want abilityIds", ab.JSONName())
	}
	if ab.HasPresence() || ab.HasOptionalKeyword() || ab.ContainingOneof() != nil {
		t.Errorf("ability_ids has presence=%v optional=%v oneof=%v; a list is not proto3 optional",
			ab.HasPresence(), ab.HasOptionalKeyword(), ab.ContainingOneof())
	}
	sf, ok := reflect.TypeOf(vttv1.Actor{}).FieldByName("AbilityIds")
	if !ok {
		t.Fatal("generated Actor has no AbilityIds field")
	}
	if sf.Type != reflect.TypeOf([]string(nil)) {
		t.Errorf("AbilityIds type = %v, want []string", sf.Type)
	}
	if tag := sf.Tag.Get("json"); tag != "ability_ids,omitempty" {
		t.Errorf("AbilityIds json tag = %q, want ability_ids,omitempty", tag)
	}
}

// VTT-683
func TestQAAnActorAddedKeepsItsAbilityIdsInOrderWithDuplicatesThroughProtojson(t *testing.T) {
	env := qaAblEnv(1, qaAblRichActor())
	wire, err := protojson.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var generic struct {
		ActorAdded struct {
			Actor map[string]json.RawMessage `json:"actor"`
		} `json:"actorAdded"`
	}
	if err := json.Unmarshal(wire, &generic); err != nil {
		t.Fatalf("decode wire: %v\n%s", err, wire)
	}
	raw, ok := generic.ActorAdded.Actor["abilityIds"]
	if !ok {
		t.Fatalf("wire actor has no abilityIds key: %s", wire)
	}
	if _, snake := generic.ActorAdded.Actor["ability_ids"]; snake {
		t.Errorf("wire actor carries ability_ids, the wire name is abilityIds: %s", wire)
	}
	var onWire []string
	if err := json.Unmarshal(raw, &onWire); err != nil {
		t.Fatalf("abilityIds is not a list of strings: %s", raw)
	}
	if !slices.Equal(onWire, qaAblListed) {
		t.Errorf("wire abilityIds = %q, want %q", onWire, qaAblListed)
	}
	back := &vttv1.Envelope{}
	if err := protojson.Unmarshal(wire, back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := back.GetActorAdded().GetActor().GetAbilityIds(); !slices.Equal(got, qaAblListed) {
		t.Errorf("round-tripped ability ids = %q, want %q", got, qaAblListed)
	}
	if !proto.Equal(env, back) {
		t.Errorf("round trip changed the envelope:\n sent %v\n back %v", env, back)
	}
}

// VTT-683
func TestQAAWireActorAddedWrittenByHandDecodesItsAbilityIdsInOrder(t *testing.T) {
	wire := []byte(`{"eventId":"qa-e1","sequence":"1","actorAdded":{"actor":` +
		`{"actorId":"a1","name":"Ada","kind":"ACTOR_KIND_PARTY_MEMBER",` +
		`"abilityIds":["rally","fists","rally","goblin-scimitar"]}}}`)
	env := &vttv1.Envelope{}
	if err := protojson.Unmarshal(wire, env); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := env.GetActorAdded().GetActor().GetAbilityIds(); !slices.Equal(got, qaAblListed) {
		t.Errorf("decoded ability ids = %q, want %q", got, qaAblListed)
	}
}

// VTT-706
func TestQATheGoFoldHoldsAnAddedActorsAbilityIdsAsTheEventGaveThem(t *testing.T) {
	st := qaAblFold(t,
		qaAblEnv(1, qaAblRichActor()),
		qaAblEnv(2, &vttv1.Actor{ActorId: "a2", Name: "Bo", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}),
		qaAblEnv(3, &vttv1.Actor{ActorId: "a3", Name: "Cy", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			AbilityIds: []string{"goblin-scimitar", "fists"}}),
	)
	if got := st.Actors["a1"].GetAbilityIds(); !slices.Equal(got, qaAblListed) {
		t.Errorf("a1 holds %q, want %q", got, qaAblListed)
	}
	if got := st.Actors["a2"].GetAbilityIds(); len(got) != 0 {
		t.Errorf("a2 was added with no list and holds %q", got)
	}
	if got := st.Actors["a3"].GetAbilityIds(); !slices.Equal(got, []string{"goblin-scimitar", "fists"}) {
		t.Errorf("a3 holds %q, want [goblin-scimitar fists]", got)
	}
	snap := st.Snapshot()
	if got := snap.Actors["a1"].GetAbilityIds(); !slices.Equal(got, qaAblListed) {
		t.Errorf("Snapshot's a1 holds %q, want %q", got, qaAblListed)
	}
}

// VTT-705
func TestQATheGoFoldKeepsAbilityIdsThroughEventsThatRewriteTheActor(t *testing.T) {
	st := qaAblFold(t,
		qaAblEnv(1, qaAblRichActor()),
		&vttv1.Envelope{EventId: "qa-e2", Sequence: 2, Payload: &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: "p1",
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
		&vttv1.Envelope{EventId: "qa-e3", Sequence: 3, Payload: &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: "p2",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}},
		&vttv1.Envelope{EventId: "qa-e4", Sequence: 4, Payload: &vttv1.Envelope_ResourceChanged{
			ResourceChanged: &vttv1.ResourceChanged{ActorId: "a1", Resource: "stamina", Delta: -1, NewValue: 1}}},
		&vttv1.Envelope{EventId: "qa-e5", Sequence: 5, Payload: &vttv1.Envelope_ActorControlRevoked{
			ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: "p1"}}},
	)
	if got := st.Actors["a1"].GetAbilityIds(); !slices.Equal(got, qaAblListed) {
		t.Errorf("after grants, a resource change and a revoke a1 holds %q, want %q", got, qaAblListed)
	}
	got, err := json.Marshal(st.Actors["a1"])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	js := string(got)
	ci, ki, ai := strings.Index(js, `"controller_ids":`), strings.Index(js, `"kind":`), strings.Index(js, `"ability_ids":`)
	if ci < 0 || ki < ci || ai < ki {
		t.Errorf("want controller_ids, then kind, then ability_ids: %s", js)
	}
	if !strings.HasSuffix(js, `"ability_ids":["rally","fists","rally","goblin-scimitar"]}`) {
		t.Errorf("ability_ids is not last with the ids in order: %s", js)
	}
}

// VTT-704
func TestQAGoJSONOfAFoldedActorWritesAbilityIdsAfterKindInOrder(t *testing.T) {
	st := qaAblFold(t, qaAblEnv(1, qaAblRichActor()))
	got, err := json.Marshal(st.Actors["a1"])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	const want = `{"actor_id":"a1","name":"Ada","module_id":"mod","attributes":{"vigor":3},` +
		`"resources":{"stamina":{"current":2,"max":5}},"kind":1,` +
		`"ability_ids":["rally","fists","rally","goblin-scimitar"]}`
	if string(got) != want {
		t.Errorf("json.Marshal(actor)\n got %s\nwant %s", got, want)
	}
}

// VTT-704
func TestQAGoJSONOfAFoldedActorWithNoListOmitsAbilityIds(t *testing.T) {
	st := qaAblFold(t,
		qaAblEnv(1, qaAblRichActor()),
		qaAblEnv(2, &vttv1.Actor{ActorId: "a2", Name: "Bo", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			AbilityIds: []string{}}),
	)
	got, err := json.Marshal(st.Actors["a2"])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	const want = `{"actor_id":"a2","name":"Bo","kind":2}`
	if string(got) != want {
		t.Errorf("json.Marshal(actor)\n got %s\nwant %s", got, want)
	}
}

// VTT-704
func TestQAGoJSONOfAnActorWithUnspecifiedKindWritesAbilityIdsLast(t *testing.T) {
	st := qaAblFold(t, qaAblEnv(1, &vttv1.Actor{
		ActorId: "a4", Name: "Dee", ModuleId: "mod",
		Attributes: map[string]int32{"vigor": 1},
		Resources:  map[string]*vttv1.Resource{"stamina": {Current: 1, Max: 4}},
		AbilityIds: []string{"rally", "fists"},
	}))
	got, err := json.Marshal(st.Actors["a4"])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	const want = `{"actor_id":"a4","name":"Dee","module_id":"mod","attributes":{"vigor":1},` +
		`"resources":{"stamina":{"current":1,"max":4}},"ability_ids":["rally","fists"]}`
	if string(got) != want {
		t.Errorf("json.Marshal(actor)\n got %s\nwant %s", got, want)
	}
}
