package fixtures_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixturesAreValidJSON(t *testing.T) {
	names := []string{
		"token_moved.json", "attack_rolled.json", "actor.json",
		"move_token_request.json", "expected_tool.json",
	}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: invalid JSON: %v", name, err)
		}
		if len(v) == 0 {
			t.Fatalf("%s: empty object", name)
		}
	}
}
