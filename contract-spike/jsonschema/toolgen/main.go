// Command toolgen derives the MCP tool definition for MoveTokenRequest.
// With JSON Schema as the source format this is embedding, not generation —
// the schema IS the inputSchema. Compare ../../proto/toolgen/main.go.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func buildTool() map[string]any {
	raw, err := os.ReadFile("../schemas/move_token_request.schema.json")
	if err != nil {
		panic(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(err)
	}
	// Strip metadata keys irrelevant to a tool inputSchema.
	for _, k := range []string{"$schema", "$id", "title"} {
		delete(schema, k)
	}
	return map[string]any{
		"name":        "move_token",
		"description": "Move a token to a new grid position on its scene.",
		"inputSchema": schema,
	}
}

func main() {
	out, _ := json.MarshalIndent(buildTool(), "", "  ")
	fmt.Println(string(out))
}
