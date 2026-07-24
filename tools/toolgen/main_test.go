package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func TestToolsMatchGolden(t *testing.T) {
	raw, err := os.ReadFile("../../contract/testdata/expected_tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(buildTools())
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("tools mismatch:\nwant %v\ngot  %v", want, got)
	}
}

// Every command message must have a manifest entry — forgetting one means the
// LLM silently loses a capability. Two registries are checked: legacy
// "Request"-suffixed messages (pre-ClientCommand convention), and every
// message that appears as a ClientCommand oneof variant — the latter IS the
// command registry now that commands are imperative-named (CreateScene, not
// CreateSceneRequest) and dispatched through ClientCommand's oneof.
func TestManifestCoversAllCommandMessages(t *testing.T) {
	msgs := vttv1.File_vtt_v1_commands_proto.Messages()
	for i := 0; i < msgs.Len(); i++ {
		name := string(msgs.Get(i).FullName())
		if !strings.HasSuffix(name, "Request") {
			continue
		}
		requireManifestEntry(t, name)
	}

	cc := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor()
	fields := cc.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		oo := f.ContainingOneof()
		if oo == nil || oo.IsSynthetic() {
			continue // not a command oneof variant (e.g. request_id)
		}
		requireManifestEntry(t, string(f.Message().FullName()))
	}
}

func requireManifestEntry(t *testing.T, message string) {
	t.Helper()
	for _, spec := range manifest {
		if spec.message == message {
			return
		}
	}
	t.Fatalf("command message %s has no toolgen manifest entry", message)
}

// TestListValueValuesFieldIsRecognizedAsList covers the trigger condition
// for schemaFor's array/items branch (main.go: `if f.IsList()`):
// google.protobuf.ListValue.values — protobuf's own canonical "repeated
// Value" wrapper, exactly the type named for this gap — is a repeated,
// message-kind field.
//
// This test stops at field-descriptor recognition rather than calling
// schemaFor/valueSchema through to a full nested schema for
// google.protobuf.Value: Value's own fields include an enum member
// (null_value NullValue), a kind valueSchema's switch has no case for
// (confirmed by hand: it panics "unhandled kind enum"), and Value.list_value
// is self-referential back through ListValue.values, so a full recursive
// expansion would not even terminate. Both are pre-existing gaps orthogonal
// to IsList/Struct-special-case correctness — worth a ledger entry — but
// out of Task 2's scope (no toolgen production-code change beyond the
// Notify guard is listed for this gap). TestSchemaForProducesArrayItems...
// and TestStructSpecialCaseFiresOnNestedValue below exercise the real,
// already-terminating array/items and Struct-special-case code paths.
func TestListValueValuesFieldIsRecognizedAsList(t *testing.T) {
	f := (&structpb.ListValue{}).ProtoReflect().Descriptor().Fields().ByName("values")
	if f == nil {
		t.Fatal("google.protobuf.ListValue has no \"values\" field")
	}
	if !f.IsList() {
		t.Fatal("want ListValue.values recognized as a repeated (list) field")
	}
	if f.Kind() != protoreflect.MessageKind || f.Message().FullName() != "google.protobuf.Value" {
		t.Fatalf("want message-kind google.protobuf.Value element, got kind=%v message=%v", f.Kind(), f.Message().FullName())
	}
}

// TestSchemaForProducesArrayItemsForRepeatedMessageField covers the
// array/items shape itself (main.go's schemaFor, IsList branch):
// AttackRolled.rolls ([]DieRoll) — the contract's only production repeated-
// message field to date — must produce {"type":"array","items": <element
// message schema>}, not a bare object or scalar. AttackRolled is an event,
// not a command, so it has no toolgen manifest entry: this path is
// otherwise entirely unexercised by TestToolsMatchGolden.
func TestSchemaForProducesArrayItemsForRepeatedMessageField(t *testing.T) {
	got := schemaFor((&vttv1.AttackRolled{}).ProtoReflect().Descriptor())
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("want a properties map, got %#v", got["properties"])
	}
	rolls, ok := props["rolls"].(map[string]any)
	if !ok {
		t.Fatalf("want a \"rolls\" property, got %#v", props)
	}
	want := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"die":    map[string]any{"type": "integer"},
				"result": map[string]any{"type": "integer"},
			},
			"required": []any{"die", "result"},
		},
	}
	if !reflect.DeepEqual(rolls, want) {
		t.Fatalf("rolls schema mismatch:\nwant %v\ngot  %v", want, rolls)
	}
}

// TestStructSpecialCaseFiresOnNestedValue covers the Struct special case
// (main.go's valueSchema, google.protobuf.Struct branch) firing during
// recursive descent, not just for a schema's own top-level fields:
// Actor.module_data is a google.protobuf.Struct field on a message (Actor)
// that only appears NESTED inside another message's schema —
// AddActor.actor — so building it requires schemaFor to recurse from
// AddActor into Actor before reaching module_data. The special case must
// still emit a bare {"type":"object"} there, rather than expanding Struct's
// own `fields` map (which would recurse into google.protobuf.Value and hit
// the same unrelated gap TestListValueValuesFieldIsRecognizedAsList's
// comment documents).
func TestStructSpecialCaseFiresOnNestedValue(t *testing.T) {
	got := schemaFor((&vttv1.AddActor{}).ProtoReflect().Descriptor())
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("want a properties map, got %#v", got["properties"])
	}
	actor, ok := props["actor"].(map[string]any)
	if !ok {
		t.Fatalf("want an \"actor\" property, got %#v", props)
	}
	actorProps, ok := actor["properties"].(map[string]any)
	if !ok {
		t.Fatalf("want actor.properties, got %#v", actor["properties"])
	}
	moduleData, ok := actorProps["moduleData"]
	if !ok {
		t.Fatalf("want a nested \"moduleData\" property, got %#v", actorProps)
	}
	want := map[string]any{"type": "object"}
	if !reflect.DeepEqual(moduleData, want) {
		t.Fatalf("moduleData special-case mismatch:\nwant %v\ngot  %v", want, moduleData)
	}
}
