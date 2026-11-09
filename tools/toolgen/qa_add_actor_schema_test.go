package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type qaSchemaTool struct {
	Name        string `json:"name"`
	InputSchema struct {
		Properties struct {
			Actor struct {
				Properties struct {
					Resources struct {
						AdditionalProperties struct {
							Properties map[string]json.RawMessage `json:"properties"`
							Required   []string                   `json:"required"`
						} `json:"additionalProperties"`
					} `json:"resources"`
				} `json:"properties"`
			} `json:"actor"`
		} `json:"properties"`
	} `json:"inputSchema"`
}

// VTT-674
func TestQASchemaAResourceRequiresItsCurrentAndNotItsMax(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"),
		filepath.Join("..", "..", "cmd", "vtt", "tools.json"),
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var tools []qaSchemaTool
		if err := json.Unmarshal(raw, &tools); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		found := false
		for _, tool := range tools {
			if tool.Name != "add_actor" {
				continue
			}
			found = true
			res := tool.InputSchema.Properties.Actor.Properties.Resources.AdditionalProperties
			for _, k := range []string{"current", "max"} {
				if _, ok := res.Properties[k]; !ok {
					t.Errorf("%s: a resource's schema has no %q property", path, k)
				}
			}
			required := map[string]bool{}
			for _, k := range res.Required {
				required[k] = true
			}
			if !required["current"] {
				t.Errorf("%s: a resource's required = %v; want current among them", path, res.Required)
			}
			if required["max"] {
				t.Errorf("%s: a resource's required = %v; want max not among them", path, res.Required)
			}
		}
		if !found {
			t.Errorf("%s: no add_actor tool", path)
		}
	}
}
