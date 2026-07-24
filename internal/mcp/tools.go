package mcp

import (
	"encoding/json"
	"fmt"
	"sort"

	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// toolManifestEntry is one committed tools.json entry (tools/toolgen/
// main.go's buildTools output): name, description, and a JSON Schema object
// good enough to hand straight to the SDK as Tool.InputSchema — see New's
// doc comment on why this package never re-derives the schema itself.
type toolManifestEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// parseToolsJSON decodes Config.ToolsJSON into its manifest entries.
func parseToolsJSON(raw []byte) ([]toolManifestEntry, error) {
	var entries []toolManifestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("mcp: parse tools.json: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("mcp: tools.json has no tool entries")
	}
	return entries, nil
}

// buildDispatch is the generic-dispatch payoff: it walks vttv1.ClientCommand's
// "command" oneof descriptor and matches each field's proto name (e.g.
// "move_token") against toolNames (the committed tools.json's tool names,
// which the toolgen manifest names identically — see tools/toolgen/main.go's
// manifest, where "name: move_token" derives from "vtt.v1.MoveTokenRequest"
// mapped to the SAME oneof field move_token = 10 in commands.proto).
//
// This is the ONLY place tool name resolves to a command shape — every tool
// call handler goes through the returned map and vttv1.ClientCommand's
// protoreflect API generically; there is deliberately no per-command
// switch anywhere in this package (self-review requirement, task-1-brief.md).
//
// Symmetric completeness check: tools.json naming a field the oneof doesn't
// have, or the oneof having a field tools.json never named, are both
// startup errors — tools.json and the oneof must agree exactly. (The
// broader "every future ClientCommand variant has a toolgen manifest entry"
// guarantee already lives in tools/toolgen's own TestManifestCoversAll
// CommandMessages; this check instead catches drift between the tools.json
// this package was CONFIGURED with and the vttv1 build it is running
// against — e.g. a stale embedded copy.)
func buildDispatch(toolNames []string) (map[string]protoreflect.FieldDescriptor, error) {
	cc := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor()
	oneof := cc.Oneofs().ByName("command")
	if oneof == nil {
		return nil, fmt.Errorf("mcp: vttv1.ClientCommand has no %q oneof", "command")
	}

	byFieldName := make(map[string]protoreflect.FieldDescriptor, oneof.Fields().Len())
	for i := 0; i < oneof.Fields().Len(); i++ {
		fd := oneof.Fields().Get(i)
		byFieldName[string(fd.Name())] = fd
	}

	byToolName := make(map[string]bool, len(toolNames))
	for _, n := range toolNames {
		byToolName[n] = true
	}

	dispatch := make(map[string]protoreflect.FieldDescriptor, len(toolNames))
	var missingField []string
	for _, n := range toolNames {
		fd, ok := byFieldName[n]
		if !ok {
			missingField = append(missingField, n)
			continue
		}
		dispatch[n] = fd
	}
	var missingTool []string
	for name := range byFieldName {
		if !byToolName[name] {
			missingTool = append(missingTool, name)
		}
	}

	if len(missingField) > 0 || len(missingTool) > 0 {
		sort.Strings(missingField)
		sort.Strings(missingTool)
		return nil, fmt.Errorf(
			"mcp: tools.json and vttv1.ClientCommand's \"command\" oneof disagree: "+
				"tools.json names with no oneof field %v; oneof fields with no tools.json entry %v",
			missingField, missingTool)
	}
	return dispatch, nil
}
