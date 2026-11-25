package contract_test

import (
	"encoding/json"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func qarfEnvelope(seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:       "ev-q",
		Sequence:      seq,
		OccurredAt:    &timestamppb.Timestamp{Seconds: 1795000000, Nanos: 123000000},
		SessionId:     "s-1",
		ActorRole:     "dm",
		ParticipantId: "p-1",
	}
}

func qarfActor() *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: "a-1", Name: "First", ModuleId: "mod-q",
		Attributes: map[string]int32{"kilo": 3},
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		AbilityIds: []string{"echo-ab"},
		References: map[string]*vttv1.Reference{
			"r-1": {EntryId: "echo", Place: "mike", State: "lima"},
			"r-2": {EntryId: "echo", Place: "kilo", State: "oscar"},
			"r-3": {EntryId: "delta"},
		},
	}
}

func qarfRoundTrip(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	js, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal: %v", err)
	}
	fromJSON := msg.ProtoReflect().New().Interface()
	if err := protojson.Unmarshal(js, fromJSON); err != nil {
		t.Fatalf("protojson.Unmarshal: %v", err)
	}
	if !proto.Equal(fromJSON, msg) {
		t.Errorf("protojson lost a field:\nsent %v\ngot  %v", msg, fromJSON)
	}
	bin, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	fromBin := msg.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(bin, fromBin); err != nil {
		t.Fatalf("proto.Unmarshal: %v", err)
	}
	if !proto.Equal(fromBin, msg) {
		t.Errorf("binary lost a field:\nsent %v\ngot  %v", msg, fromBin)
	}
	return js
}

// VTT-1027.
func TestQARFAnActorHoldingReferencesRoundTrips(t *testing.T) {
	qarfRoundTrip(t, qarfActor())
	env := qarfEnvelope(3)
	env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: qarfActor()}}
	qarfRoundTrip(t, env)
}

// VTT-1027.
func TestQARFEachReferenceEventRoundTrips(t *testing.T) {
	added := qarfEnvelope(4)
	added.Payload = &vttv1.Envelope_ReferenceAdded{ReferenceAdded: &vttv1.ReferenceAdded{
		ActorId: "a-1", ReferenceId: "r-4",
		Reference: &vttv1.Reference{EntryId: "delta", Place: "kilo", State: "lima"},
	}}
	removed := qarfEnvelope(5)
	removed.Payload = &vttv1.Envelope_ReferenceRemoved{ReferenceRemoved: &vttv1.ReferenceRemoved{
		ActorId: "a-1", ReferenceId: "r-2",
	}}
	changed := qarfEnvelope(6)
	changed.Payload = &vttv1.Envelope_ReferenceChanged{ReferenceChanged: &vttv1.ReferenceChanged{
		ActorId: "a-1", ReferenceId: "r-1", Place: "mike", State: "oscar",
	}}
	for name, env := range map[string]*vttv1.Envelope{
		"added": added, "removed": removed, "changed": changed,
	} {
		t.Run(name, func(t *testing.T) { qarfRoundTrip(t, env) })
	}
}

// VTT-1027.
func TestQARFAnEmptyReferenceRoundTripsAsPresent(t *testing.T) {
	env := qarfEnvelope(7)
	env.Payload = &vttv1.Envelope_ReferenceAdded{ReferenceAdded: &vttv1.ReferenceAdded{
		ActorId: "a-1", ReferenceId: "r-5", Reference: &vttv1.Reference{},
	}}
	qarfRoundTrip(t, env)
	bin, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var back vttv1.Envelope
	if err := proto.Unmarshal(bin, &back); err != nil {
		t.Fatal(err)
	}
	if back.GetReferenceAdded().Reference == nil {
		t.Error("binary dropped an empty reference")
	}
	js, err := protojson.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var backJS vttv1.Envelope
	if err := protojson.Unmarshal(js, &backJS); err != nil {
		t.Fatal(err)
	}
	if backJS.GetReferenceAdded().Reference == nil {
		t.Errorf("protojson dropped an empty reference: %s", js)
	}
}

// VTT-1027. Keep this JSON written by hand: protojson's own output would
// grade the contract by itself.
func TestQARFHandWrittenJSONDecodesEveryField(t *testing.T) {
	cases := map[string]struct {
		js   string
		want *vttv1.Envelope
	}{
		"actor": {
			js: `{"sequence":"9","actorAdded":{"actor":{"actorId":"a-1",` +
				`"references":{"r-1":{"entryId":"echo","place":"mike","state":"lima"},` +
				`"r-2":{"entryId":"delta","place":"kilo","state":"oscar"}}}}}`,
			want: &vttv1.Envelope{Sequence: 9, Payload: &vttv1.Envelope_ActorAdded{
				ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
					ActorId: "a-1",
					References: map[string]*vttv1.Reference{
						"r-1": {EntryId: "echo", Place: "mike", State: "lima"},
						"r-2": {EntryId: "delta", Place: "kilo", State: "oscar"},
					},
				}},
			}},
		},
		"added": {
			js: `{"sequence":"10","referenceAdded":{"actorId":"a-1","referenceId":"r-4",` +
				`"reference":{"entryId":"echo","place":"kilo","state":"oscar"}}}`,
			want: &vttv1.Envelope{Sequence: 10, Payload: &vttv1.Envelope_ReferenceAdded{
				ReferenceAdded: &vttv1.ReferenceAdded{
					ActorId: "a-1", ReferenceId: "r-4",
					Reference: &vttv1.Reference{EntryId: "echo", Place: "kilo", State: "oscar"},
				},
			}},
		},
		"removed": {
			js: `{"sequence":"11","referenceRemoved":{"actorId":"a-1","referenceId":"r-4"}}`,
			want: &vttv1.Envelope{Sequence: 11, Payload: &vttv1.Envelope_ReferenceRemoved{
				ReferenceRemoved: &vttv1.ReferenceRemoved{ActorId: "a-1", ReferenceId: "r-4"},
			}},
		},
		"changed": {
			js: `{"sequence":"12","referenceChanged":{"actorId":"a-1","referenceId":"r-4",` +
				`"place":"mike","state":"lima"}}`,
			want: &vttv1.Envelope{Sequence: 12, Payload: &vttv1.Envelope_ReferenceChanged{
				ReferenceChanged: &vttv1.ReferenceChanged{
					ActorId: "a-1", ReferenceId: "r-4", Place: "mike", State: "lima",
				},
			}},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var got vttv1.Envelope
			if err := protojson.Unmarshal([]byte(c.js), &got); err != nil {
				t.Fatalf("decode %s: %v", c.js, err)
			}
			if !proto.Equal(&got, c.want) {
				t.Errorf("decoded %v, want %v", &got, c.want)
			}
			out, err := protojson.Marshal(&got)
			if err != nil {
				t.Fatal(err)
			}
			var a, b any
			if err := json.Unmarshal(out, &a); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(c.js), &b); err != nil {
				t.Fatal(err)
			}
			ja, _ := json.Marshal(a)
			jb, _ := json.Marshal(b)
			if string(ja) != string(jb) {
				t.Errorf("re-encoded as %s, want %s", ja, jb)
			}
		})
	}
}

func TestQARFOneReferenceIdWrittenTwiceIsRefused(t *testing.T) {
	js := `{"actorId":"a-1","references":{"r-1":{"entryId":"echo"},"r-1":{"entryId":"delta"}}}`
	var a vttv1.Actor
	if err := protojson.Unmarshal([]byte(js), &a); err == nil {
		t.Errorf("decoded a repeated id as %v", &a)
	}
	ok := strings.Replace(js, `"r-1":{"entryId":"delta"}`, `"r-2":{"entryId":"delta"}`, 1)
	if err := protojson.Unmarshal([]byte(ok), &a); err != nil {
		t.Errorf("two ids refused: %v", err)
	}
}

func TestQARFTheReferenceFieldNumbers(t *testing.T) {
	num := func(m proto.Message, field string) protoreflect.FieldNumber {
		fd := m.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(field))
		if fd == nil {
			t.Fatalf("%s has no field %s", m.ProtoReflect().Descriptor().FullName(), field)
		}
		return fd.Number()
	}
	cases := []struct {
		msg   proto.Message
		field string
		want  protoreflect.FieldNumber
	}{
		{&vttv1.Actor{}, "references", 11},
		{&vttv1.Reference{}, "entry_id", 1},
		{&vttv1.Reference{}, "place", 2},
		{&vttv1.Reference{}, "state", 3},
		{&vttv1.Envelope{}, "reference_added", 41},
		{&vttv1.Envelope{}, "reference_removed", 42},
		{&vttv1.Envelope{}, "reference_changed", 43},
	}
	for _, c := range cases {
		if got := num(c.msg, c.field); got != c.want {
			t.Errorf("%s.%s = %d, want %d",
				c.msg.ProtoReflect().Descriptor().FullName(), c.field, got, c.want)
		}
	}
	refs := (&vttv1.Actor{}).ProtoReflect().Descriptor().Fields().ByName("references")
	if !refs.IsMap() || refs.MapKey().Kind() != protoreflect.StringKind ||
		refs.MapValue().Message().FullName() != "vtt.v1.Reference" {
		t.Errorf("references is not map<string, Reference>: %v", refs)
	}
}
