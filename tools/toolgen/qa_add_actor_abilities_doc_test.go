package main_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type qaAbProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Items       struct {
		Type string `json:"type"`
	} `json:"items"`
}

type qaAbTool struct {
	Name        string `json:"name"`
	InputSchema struct {
		Properties struct {
			Actor struct {
				Properties map[string]qaAbProperty `json:"properties"`
			} `json:"actor"`
		} `json:"properties"`
	} `json:"inputSchema"`
}

func qaAbAbilityIds(t *testing.T, path string) qaAbProperty {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var tools []qaAbTool
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	for _, tool := range tools {
		if tool.Name != "add_actor" {
			continue
		}
		p, ok := tool.InputSchema.Properties.Actor.Properties["abilityIds"]
		if !ok {
			t.Fatalf("%s: add_actor's actor has no abilityIds property", path)
		}
		return p
	}
	t.Fatalf("%s: no add_actor tool", path)
	return qaAbProperty{}
}

// VTT-700
func TestQAAbAddActorSchemaDocumentsAbilityIds(t *testing.T) {
	for _, path := range []string{"../../contract/gen/tools/tools.json", "../../cmd/vtt/tools.json"} {
		t.Run(path, func(t *testing.T) {
			p := qaAbAbilityIds(t, path)
			if p.Type != "array" || p.Items.Type != "string" {
				t.Fatalf("abilityIds is %q of %q, want an array of strings", p.Type, p.Items.Type)
			}
			d := strings.ToLower(p.Description)
			for _, phrase := range []string{"abilities", "id", "served ruleset", "with a ruleset served", "the ruleset does not hold", "refuses"} {
				if !strings.Contains(d, phrase) {
					t.Errorf("abilityIds description %q does not say %q", p.Description, phrase)
				}
			}
		})
	}
}
