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

type toolSpec struct {
	message     string
	name        string
	description string
	descriptor  protoreflect.MessageDescriptor
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
}

func isOptional(f protoreflect.FieldDescriptor) bool {
	oo := f.ContainingOneof()
	return oo != nil && oo.IsSynthetic()
}

func schemaFor(md protoreflect.MessageDescriptor) map[string]any {
	props := map[string]any{}
	required := []any{}
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		name := f.JSONName()
		if f.IsMap() {
			props[name] = map[string]any{
				"type":                 "object",
				"additionalProperties": valueSchema(f.MapValue()),
			}
			if !isOptional(f) {
				required = append(required, name)
			}
			continue
		}
		if f.IsList() {
			props[name] = map[string]any{"type": "array", "items": valueSchema(f)}
			if !isOptional(f) {
				required = append(required, name)
			}
			continue
		}
		props[name] = valueSchema(f)
		if !isOptional(f) {
			required = append(required, name)
		}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

// valueSchema derives the JSON Schema for a single scalar/message value —
// shared by plain fields, map values, and list items. google.protobuf.Struct
// is emitted as a bare open object since the contract never inspects
// module-owned data (see README: Struct rules).
func valueSchema(f protoreflect.FieldDescriptor) map[string]any {
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
		return schemaFor(f.Message())
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
			"inputSchema": schemaFor(spec.descriptor),
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
