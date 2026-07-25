// Command toolgen derives MCP tool definitions from the contract's protobuf
// descriptors. proto3 `optional` fields (synthetic oneofs) are omitted from
// each tool's `required` list — this is the contract's optionality annotation
// (ADR-007). Output is committed at contract/gen/tools/tools.json and covered
// by the drift gate.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// fieldOverride is manifest-level authoring guidance for one proto message
// type's generated JSON Schema, applied wherever that message type is
// reached during schemaFor's recursion (keyed by message full name, not by
// nesting path — see toolSpec.overrides and schemaForWithOverrides).
// requiredOverride, when non-nil, REPLACES the derived `required` list
// (proto3's own optionality annotation — ADR-007's synthetic-oneof rule —
// otherwise decides it) entirely; fieldDocs adds a per-property JSON
// Schema "description" for fields named in it. Both exist for exactly the
// case proto3 itself cannot express: a field that is technically
// non-optional on the wire but almost always meant to be omitted by an LLM
// caller, who will otherwise fabricate a value rather than send nothing —
// the add_actor fix (final review Fix 2) is the first user.
type fieldOverride struct {
	requiredOverride []string
	fieldDocs        map[string]string
}

type toolSpec struct {
	message     string
	name        string
	description string
	descriptor  protoreflect.MessageDescriptor
	// overrides maps a proto message full name (e.g. "vtt.v1.Actor") to
	// authoring guidance for that message's generated schema — nil for
	// every tool that needs none. See fieldOverride's doc comment.
	overrides map[protoreflect.FullName]fieldOverride
}

var manifest = []toolSpec{
	{
		message:     "vtt.v1.MoveTokenRequest",
		name:        "move_token",
		description: "Move a token to a new grid position on its scene.",
		descriptor:  (&vttv1.MoveTokenRequest{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.CreateScene",
		name:        "create_scene",
		description: "Create a new scene with a grid.",
		descriptor:  (&vttv1.CreateScene{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.AddActor",
		name:        "add_actor",
		description: "Add an actor to the campaign.",
		descriptor:  (&vttv1.AddActor{}).ProtoReflect().Descriptor(),
		// The fabrication trap (final review Fix 2): none of Actor's
		// fields are proto3 `optional`, so the derived required list
		// forces every one of them — an LLM caller then has no way to
		// tell "must supply" from "the field just happens to be
		// non-optional on the wire", and fabricates a plausible-looking
		// moduleId/controllerId/etc. rather than sending nothing. Only
		// actorId is genuinely required to add an actor at all.
		overrides: map[protoreflect.FullName]fieldOverride{
			"vtt.v1.Actor": {
				requiredOverride: []string{"actorId"},
				fieldDocs: map[string]string{
					"name":         "Optional display label for the actor.",
					"controllerId": "Omit or empty = DM/agent-controlled; set a participant id to hand control to a player.",
					"moduleId":     "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
					"attributes":   "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
					"resources":    "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
					"moduleData":   "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
				},
			},
		},
	},
	{
		message:     "vtt.v1.PlaceToken",
		name:        "place_token",
		description: "Place an actor's token on a scene's grid.",
		descriptor:  (&vttv1.PlaceToken{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.StartSession",
		name:        "start_session",
		description: "Start a new play session.",
		descriptor:  (&vttv1.StartSession{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.EndSession",
		name:        "end_session",
		description: "End the current play session.",
		descriptor:  (&vttv1.EndSession{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.RetractEvents",
		name:        "retract_events",
		description: "Retract a range of events from the record with a stated reason.",
		descriptor:  (&vttv1.RetractEvents{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.UseAbility",
		name:        "use_ability",
		description: "Use one of the loaded ruleset's abilities as an actor against explicit targets.",
		descriptor:  (&vttv1.UseAbility{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.RemoveCondition",
		name:        "remove_condition",
		description: "Remove a named condition from an actor (DM-ended durations).",
		descriptor:  (&vttv1.RemoveCondition{}).ProtoReflect().Descriptor(),
	},
}

func isOptional(f protoreflect.FieldDescriptor) bool {
	oo := f.ContainingOneof()
	return oo != nil && oo.IsSynthetic()
}

// schemaFor derives md's JSON Schema with no manifest overrides applied —
// the plain recursive builder, used directly by tests that check this
// package's structural handling (arrays, Struct) in isolation from any
// tool's authoring guidance. buildTools always goes through
// schemaForWithOverrides instead.
func schemaFor(md protoreflect.MessageDescriptor) map[string]any {
	return schemaForWithOverrides(md, nil)
}

// schemaForWithOverrides is schemaFor plus overrides: a proto message full
// name -> fieldOverride map (see fieldOverride's doc comment), threaded
// through every recursive call so a message type reached at ANY nesting
// depth (not just top-level) picks up its own entry, if any, by full name.
func schemaForWithOverrides(md protoreflect.MessageDescriptor, overrides map[protoreflect.FullName]fieldOverride) map[string]any {
	props := map[string]any{}
	required := []any{}
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		name := f.JSONName()
		if f.IsMap() {
			props[name] = map[string]any{
				"type":                 "object",
				"additionalProperties": valueSchemaWithOverrides(f.MapValue(), overrides),
			}
			if !isOptional(f) {
				required = append(required, name)
			}
			continue
		}
		if f.IsList() {
			props[name] = map[string]any{"type": "array", "items": valueSchemaWithOverrides(f, overrides)}
			if !isOptional(f) {
				required = append(required, name)
			}
			continue
		}
		props[name] = valueSchemaWithOverrides(f, overrides)
		if !isOptional(f) {
			required = append(required, name)
		}
	}

	if ov, ok := overrides[md.FullName()]; ok {
		if ov.requiredOverride != nil {
			required = make([]any, len(ov.requiredOverride))
			for i, name := range ov.requiredOverride {
				required[i] = name
			}
		}
		for fieldName, doc := range ov.fieldDocs {
			propSchema, ok := props[fieldName].(map[string]any)
			if !ok {
				panic(fmt.Sprintf("toolgen: fieldDocs names %q, not a property of %s", fieldName, md.FullName()))
			}
			propSchema["description"] = doc
		}
	}

	return map[string]any{"type": "object", "properties": props, "required": required}
}

// valueSchema derives the JSON Schema for a single scalar/message value,
// with no overrides — see schemaFor's doc comment on when to use the plain
// form vs. valueSchemaWithOverrides.
func valueSchema(f protoreflect.FieldDescriptor) map[string]any {
	return valueSchemaWithOverrides(f, nil)
}

// valueSchemaWithOverrides is valueSchema plus overrides, threaded into any
// recursive schemaForWithOverrides call it makes — shared by plain fields,
// map values, and list items. google.protobuf.Struct is emitted as a bare
// open object since the contract never inspects module-owned data (see
// README: Struct rules).
func valueSchemaWithOverrides(f protoreflect.FieldDescriptor, overrides map[protoreflect.FullName]fieldOverride) map[string]any {
	switch f.Kind() {
	case protoreflect.StringKind:
		return map[string]any{"type": "string"}
	case protoreflect.BoolKind:
		return map[string]any{"type": "boolean"}
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return map[string]any{"type": "integer"}
	case protoreflect.MessageKind:
		if f.Message().FullName() == "google.protobuf.Struct" {
			return map[string]any{"type": "object"}
		}
		return schemaForWithOverrides(f.Message(), overrides)
	default:
		panic(fmt.Sprintf("toolgen: unhandled kind %v on %s", f.Kind(), f.FullName()))
	}
}

func buildTools() []map[string]any {
	var tools []map[string]any
	for _, spec := range manifest {
		tools = append(tools, map[string]any{
			"name":        spec.name,
			"description": spec.description,
			"inputSchema": schemaForWithOverrides(spec.descriptor, spec.overrides),
		})
	}
	return tools
}

func main() {
	out := flag.String("o", "", "output path (default stdout)")
	flag.Parse()
	data, err := json.MarshalIndent(buildTools(), "", "  ")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if *out == "" {
		fmt.Print(string(data))
		return
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		panic(err)
	}
}
