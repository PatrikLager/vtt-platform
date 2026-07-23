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
		if f.IsList() || f.IsMap() {
			panic(fmt.Sprintf("toolgen: repeated/map field %s not yet supported", f.FullName()))
		}
		switch f.Kind() {
		case protoreflect.StringKind:
			props[name] = map[string]any{"type": "string"}
		case protoreflect.BoolKind:
			props[name] = map[string]any{"type": "boolean"}
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			props[name] = map[string]any{"type": "integer"}
		case protoreflect.MessageKind:
			props[name] = schemaFor(f.Message())
		default:
			panic(fmt.Sprintf("toolgen: unhandled kind %v on %s", f.Kind(), f.FullName()))
		}
		if !isOptional(f) {
			required = append(required, name)
		}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
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
