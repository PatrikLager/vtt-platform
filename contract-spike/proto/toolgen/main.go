// Command toolgen derives an MCP tool definition for MoveTokenRequest from
// its protobuf descriptor. Exists to measure the custom-code cost of the
// proto -> LLM-tool path; see ../EVIDENCE.md.
package main

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	contractv1 "github.com/PatrikLager/vtt-platform/contract-spike/proto/gen/go/contract/v1"
)

func schemaFor(md protoreflect.MessageDescriptor) map[string]any {
	props := map[string]any{}
	var required []any
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		name := f.JSONName()
		switch f.Kind() {
		case protoreflect.StringKind:
			props[name] = map[string]any{"type": "string"}
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			props[name] = map[string]any{"type": "integer"}
		case protoreflect.MessageKind:
			props[name] = schemaFor(f.Message())
		default:
			panic(fmt.Sprintf("toolgen: unhandled kind %v", f.Kind()))
		}
		required = append(required, name)
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func buildTool() map[string]any {
	md := (&contractv1.MoveTokenRequest{}).ProtoReflect().Descriptor()
	return map[string]any{
		"name":        "move_token",
		"description": "Move a token to a new grid position on its scene.",
		"inputSchema": schemaFor(md),
	}
}

func main() {
	out, _ := json.MarshalIndent(buildTool(), "", "  ")
	fmt.Println(string(out))
}
