package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

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

// TestSchemaForProducesArrayItemsForRepeatedMessageField covers the
// array/items shape itself (main.go's schemaFor, IsList branch):
// AbilityUsed.rolls ([]AbilityUsed.Roll) is the ruleset-interpreter
// contract's repeated-message field this path exercises — a NESTED message
// type (declared inside AbilityUsed itself) whose own fields include a
// repeated SCALAR (results, []int32), so this single assertion covers both
// "repeated message produces array/items of a full nested object schema"
// and "that nested object schema itself correctly renders a repeated
// scalar field" in one recursive pass. AbilityUsed is an event, not a
// command, so it has no toolgen manifest entry: this path is otherwise
// entirely unexercised by TestToolsMatchGolden. (Formerly exercised via
// AttackRolled.rolls — swapped to AbilityUsed.rolls now that it exists, so
// the newest production repeated-message field is the one load-bearing
// here; AttackRolled.rolls remains structurally identical and untouched.)
func TestSchemaForProducesArrayItemsForRepeatedMessageField(t *testing.T) {
	got := schemaFor((&vttv1.AbilityUsed{}).ProtoReflect().Descriptor())
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
				"expression": map[string]any{"type": "string"},
				"results":    map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
				"total":      map[string]any{"type": "integer"},
			},
			"required": []any{"expression", "results", "total"},
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
// findTool returns the buildTools() entry named name, or fails the test —
// shared lookup for the requiredOverride/fieldDocs tests below, which check
// specific nested schema nodes rather than a whole-output golden diff.
func findTool(t *testing.T, name string) map[string]any {
	t.Helper()
	for _, tl := range buildTools() {
		if tl["name"] == name {
			return tl
		}
	}
	t.Fatalf("buildTools: no tool named %q", name)
	return nil
}

// TestAddActorRequiredOverrideReplacesDerivedList covers the add_actor
// fabrication-trap fix (final review Fix 2): proto3 marks none of Actor's
// fields `optional` (ADR-007's synthetic-oneof annotation), so schemaFor's
// derived required list would otherwise force every one of them —
// including controllerId/moduleId/attributes/resources/moduleData, fields
// an LLM caller should almost always OMIT rather than invent a value for.
// The manifest's requiredOverride on add_actor must replace that derived
// list entirely, down to exactly actorId.
func TestAddActorRequiredOverrideReplacesDerivedList(t *testing.T) {
	tool := findTool(t, "add_actor")
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok {
		t.Fatalf("add_actor: inputSchema missing or not an object: %#v", tool["inputSchema"])
	}
	actorSchema, ok := schema["properties"].(map[string]any)["actor"].(map[string]any)
	if !ok {
		t.Fatalf("add_actor: properties.actor missing or not an object: %#v", schema["properties"])
	}
	got := actorSchema["required"]
	want := []any{"actorId"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("add_actor actor.required = %v, want %v", got, want)
	}
}

// TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication covers the
// per-field guidance half of the same fix: every field the required
// override demoted to optional must carry a "description" steering the LLM
// away from fabricating a value, and the one field that stayed required
// (actorId) must carry none (nothing to steer it away from).
func TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication(t *testing.T) {
	tool := findTool(t, "add_actor")
	schema := tool["inputSchema"].(map[string]any)
	actorSchema := schema["properties"].(map[string]any)["actor"].(map[string]any)
	props, ok := actorSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("add_actor: actor.properties missing or not an object: %#v", actorSchema["properties"])
	}

	wantSubstr := map[string]string{
		"name":         "Optional",
		"controllerId": "DM/agent-controlled",
		"moduleId":     "opaque",
		"attributes":   "opaque",
		"resources":    "opaque",
		"moduleData":   "opaque",
	}
	for field, substr := range wantSubstr {
		prop, ok := props[field].(map[string]any)
		if !ok {
			t.Fatalf("add_actor actor.properties[%q] missing or not an object: %#v", field, props[field])
		}
		desc, _ := prop["description"].(string)
		if !strings.Contains(desc, substr) {
			t.Fatalf("add_actor actor.properties[%q].description = %q, want it to contain %q", field, desc, substr)
		}
	}

	actorIDProp, ok := props["actorId"].(map[string]any)
	if !ok {
		t.Fatalf("add_actor actor.properties[\"actorId\"] missing or not an object: %#v", props["actorId"])
	}
	if _, hasDoc := actorIDProp["description"]; hasDoc {
		t.Fatalf("add_actor actor.properties[\"actorId\"] has a description, want none (it's the one field that stayed required)")
	}
}

// TestAddNarrationRequiredOverrideReplacesDerivedList covers the
// add_narration fabrication-trap fix (final review Fix — same shape as
// add_actor's, see TestAddActorRequiredOverrideReplacesDerivedList's own
// doc comment): none of AddNarration's fields are proto3 `optional`
// (ADR-007's synthetic-oneof annotation), so schemaFor's derived required
// list would otherwise force `as`/anchorFromSeq/anchorToSeq alongside
// `text` — spec §3 documents all three as optional (empty `as` = speak as
// yourself; 0/0 anchors = unanchored), so an LLM caller obeying the
// derived schema has no way to omit them and fabricates values instead.
// The manifest's requiredOverride on add_narration must replace the
// derived list entirely, down to exactly "text" — unlike add_actor's
// override (keyed on the nested "vtt.v1.Actor" message reached via
// AddActor.actor), AddNarration's fields are direct fields on AddNarration
// itself, so the override here is keyed on "vtt.v1.AddNarration".
func TestAddNarrationRequiredOverrideReplacesDerivedList(t *testing.T) {
	tool := findTool(t, "add_narration")
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok {
		t.Fatalf("add_narration: inputSchema missing or not an object: %#v", tool["inputSchema"])
	}
	got := schema["required"]
	want := []any{"text"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("add_narration required = %v, want %v", got, want)
	}
}

// TestAddNarrationFieldDocsNameOptionalFieldsAgainstFabrication covers the
// per-field guidance half of the same fix: as/anchorFromSeq/anchorToSeq —
// each demoted from required to optional — must carry a "description"
// steering the LLM away from fabricating a value (omit-when-unanchored /
// speaking-as-self), and text (the one field that stayed required) must
// carry none.
func TestAddNarrationFieldDocsNameOptionalFieldsAgainstFabrication(t *testing.T) {
	tool := findTool(t, "add_narration")
	schema := tool["inputSchema"].(map[string]any)
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("add_narration: properties missing or not an object: %#v", schema["properties"])
	}

	wantSubstr := map[string]string{
		"as":            "speak as yourself",
		"anchorFromSeq": "unanchored",
		"anchorToSeq":   "unanchored",
	}
	for field, substr := range wantSubstr {
		prop, ok := props[field].(map[string]any)
		if !ok {
			t.Fatalf("add_narration properties[%q] missing or not an object: %#v", field, props[field])
		}
		desc, _ := prop["description"].(string)
		if !strings.Contains(desc, substr) {
			t.Fatalf("add_narration properties[%q].description = %q, want it to contain %q", field, desc, substr)
		}
	}

	textProp, ok := props["text"].(map[string]any)
	if !ok {
		t.Fatalf(`add_narration properties["text"] missing or not an object: %#v`, props["text"])
	}
	if _, hasDoc := textProp["description"]; hasDoc {
		t.Fatalf(`add_narration properties["text"] has a description, want none (it's the one field that stayed required)`)
	}
}

// TestUpsertNoteRequiredOverrideReplacesDerivedList covers the analogous,
// lower-stakes fabrication trap the same review flagged: none of
// UpsertNote's fields are proto3 `optional` either, so the derived list
// would force `title` alongside key/text even though empty title is
// adjudicated-legal (spec-permitted "may be empty" ruling, engine only
// enforces a max — internal/engine/apply.go's NoteUpserted case). The
// override replaces the derived list with exactly ["key", "text"].
func TestUpsertNoteRequiredOverrideReplacesDerivedList(t *testing.T) {
	tool := findTool(t, "upsert_note")
	schema, ok := tool["inputSchema"].(map[string]any)
	if !ok {
		t.Fatalf("upsert_note: inputSchema missing or not an object: %#v", tool["inputSchema"])
	}
	got := schema["required"]
	want := []any{"key", "text"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("upsert_note required = %v, want %v", got, want)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("upsert_note: properties missing or not an object: %#v", schema["properties"])
	}
	titleProp, ok := props["title"].(map[string]any)
	if !ok {
		t.Fatalf(`upsert_note properties["title"] missing or not an object: %#v`, props["title"])
	}
	if desc, _ := titleProp["description"].(string); !strings.Contains(desc, "may be empty") {
		t.Fatalf(`upsert_note properties["title"].description = %q, want it to contain "may be empty"`, desc)
	}
}

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

// TestEnumFieldOffersOnlyTheChoicesThatCanSucceed pins the schema for the
// first ENUM ever to appear in a ClientCommand (SetJoinDoor.door).
//
// toolgen PANICKED on it rather than guessing — "unhandled kind enum" — which
// is the right instinct and is why this is a test rather than a bug report: a
// generator that invented a plausible schema for a kind it had never seen
// would have advertised something no server accepts.
//
// The zero value is left OUT. JOIN_DOOR_UNSPECIFIED exists so the wire can
// tell "shut the door" from "forgot to say", and the server refuses it — so
// offering it to a model is offering a choice that can only fail. It is
// dropped only when it is BOTH number 0 and named _UNSPECIFIED, so an enum
// with a meaningful zero would still be advertised in full rather than
// silently losing a value.
func TestEnumFieldOffersOnlyTheChoicesThatCanSucceed(t *testing.T) {
	schema := schemaFor((&vttv1.SetJoinDoor{}).ProtoReflect().Descriptor())

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("no properties in %#v", schema)
	}
	door, ok := props["door"].(map[string]any)
	if !ok {
		t.Fatalf("no door property in %#v", props)
	}
	if door["type"] != "string" {
		t.Fatalf("door type = %v, want string — protojson carries enum VALUES as names", door["type"])
	}
	got, ok := door["enum"].([]any)
	if !ok {
		t.Fatalf("door has no enum list: %#v", door)
	}
	want := []any{"JOIN_DOOR_OPEN", "JOIN_DOOR_CLOSED"}
	if len(got) != len(want) {
		t.Fatalf("enum = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("enum = %v, want %v", got, want)
		}
	}
}
