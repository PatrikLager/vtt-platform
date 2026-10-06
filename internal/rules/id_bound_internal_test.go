package rules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// VTT-283
func TestTheIDBoundMirrorsEngine(t *testing.T) {
	if maxIDBytes != 128 {
		t.Errorf("maxIDBytes = %d, want 128, internal/engine's bound on an id", maxIDBytes)
	}
}

// VTT-287
func TestTheSchemasStateTheIDBound(t *testing.T) {
	want := fmt.Sprintf("At most %d bytes", maxIDBytes)
	for _, c := range []struct {
		file string
		path []string
	}{
		{"ability", []string{"properties", "id"}},
		{"condition", []string{"properties", "id"}},
		{"ruleset", []string{"properties", "attributes"}},
		{"ruleset", []string{"properties", "defenses"}},
		{"ruleset", []string{"properties", "resources", "items", "properties", "name"}},
		{"atom", []string{"$defs", "resolutionContribution", "properties", "branches"}},
	} {
		raw, err := Schemas.ReadFile("schema/" + c.file + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		var node map[string]any
		if err := json.Unmarshal(raw, &node); err != nil {
			t.Fatal(err)
		}
		for _, k := range c.path {
			next, ok := node[k].(map[string]any)
			if !ok {
				t.Fatalf("%s.schema.json has no %s", c.file, strings.Join(c.path, "."))
			}
			node = next
		}
		if desc, _ := node["description"].(string); !strings.Contains(desc, want) {
			t.Errorf("%s.schema.json %s description = %q, want it to state %q", c.file, strings.Join(c.path, "."), desc, want)
		}
	}
}
