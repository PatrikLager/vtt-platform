package rules_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// describedRuleset writes testdata/valid to a scratch directory, sets each
// file's fields as edits name them (a nil value deletes the field), and
// returns the directory.
func describedRuleset(t *testing.T, edits map[string]map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	copyDir(t, "testdata/valid", dir)
	for file, fields := range edits {
		path := filepath.Join(dir, file)
		obj := readJSONFile(t, path)
		for k, v := range fields {
			if v == nil {
				delete(obj, k)
				continue
			}
			obj[k] = v
		}
		writeJSONFile(t, path, obj)
	}
	return dir
}

func expectFieldRefusal(t *testing.T, dir, file, field string) {
	t.Helper()
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatalf("Load accepted the ruleset; want a refusal naming %s and field %q", file, field)
	}
	msg := err.Error()
	if !strings.Contains(msg, filepath.Join(dir, file)) || !strings.Contains(msg, fmt.Sprintf("field %q", field)) {
		t.Errorf("refusal %q does not name %s and field %q", msg, file, field)
	}
	if !strings.Contains(msg, "must not be empty") {
		t.Errorf("refusal %q does not say the field must not be empty", msg)
	}
}

// VTT-847
func TestAnAbilityKeepsTheDescriptionItDeclares(t *testing.T) {
	dir := describedRuleset(t, map[string]map[string]any{
		"abilities/strike.json":       {"description": "A line about the first move."},
		"abilities/guard-stance.json": {"description": "A line about the second\nmove."},
		"abilities/stand-down.json":   {"description": nil},
	})
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"strike":       "A line about the first move.",
		"guard-stance": "A line about the second\nmove.",
		"stand-down":   "",
	} {
		if got := rs.Compiled[id].Description; got != want {
			t.Errorf("%s's Description = %q, want %q", id, got, want)
		}
	}
}

// VTT-847
func TestAnAbilityDeclaringItsDescriptionNullHasNone(t *testing.T) {
	dir := describedRuleset(t, nil)
	path := filepath.Join(dir, "abilities", "strike.json")
	obj := readJSONFile(t, path)
	obj["description"] = nil
	writeJSONFile(t, path, obj)
	if v, ok := readJSONFile(t, path)["description"]; !ok || v != nil {
		t.Fatal("the fixture does not declare the description null")
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := rs.Compiled["strike"].Description; got != "" {
		t.Errorf("Description = %q for a description declared null, want none", got)
	}
}

// VTT-848
func TestAnEmptyAbilityDescriptionIsRefused(t *testing.T) {
	dir := describedRuleset(t, map[string]map[string]any{
		"abilities/strike.json": {"description": ""},
	})
	expectFieldRefusal(t, dir, filepath.Join("abilities", "strike.json"), "description")
}

// VTT-856 VTT-861
func TestARulesetKeepsTheNameItGivesAnAbilityThatSpendsNoResource(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"declared", "Freely", "Freely"},
		{"absent", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs, err := rules.Load(describedRuleset(t, map[string]map[string]any{
				"ruleset.json": {"free_usage_name": c.value},
			}))
			if err != nil {
				t.Fatal(err)
			}
			if rs.FreeUsageName != c.want {
				t.Errorf("FreeUsageName = %q, want %q", rs.FreeUsageName, c.want)
			}
		})
	}
	t.Run("null", func(t *testing.T) {
		dir := describedRuleset(t, nil)
		path := filepath.Join(dir, "ruleset.json")
		obj := readJSONFile(t, path)
		obj["free_usage_name"] = nil
		writeJSONFile(t, path, obj)
		rs, err := rules.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if rs.FreeUsageName != "" {
			t.Errorf("FreeUsageName = %q for a name declared null, want none", rs.FreeUsageName)
		}
	})
}

// VTT-857
func TestAnEmptyNameForAnAbilityThatSpendsNoResourceIsRefused(t *testing.T) {
	dir := describedRuleset(t, map[string]map[string]any{
		"ruleset.json": {"free_usage_name": ""},
	})
	expectFieldRefusal(t, dir, "ruleset.json", "free_usage_name")
}

// schemaField returns the schema node a top-level field of file describes,
// and whether file's schema lists it as required.
func schemaField(t *testing.T, file, field string) (map[string]any, bool) {
	t.Helper()
	schema := readSchema(t, file)
	props, _ := schema["properties"].(map[string]any)
	node, ok := props[field].(map[string]any)
	if !ok {
		t.Fatalf("%s does not describe %q", file, field)
	}
	var required []string
	for _, r := range asList(schema["required"]) {
		required = append(required, r.(string))
	}
	return node, slices.Contains(required, field)
}

// VTT-849
func TestTheAbilitySchemaDescribesAnOptionalNonEmptyDescription(t *testing.T) {
	node, required := schemaField(t, "ability.schema.json", "description")
	if required {
		t.Error("ability.schema.json lists description as required")
	}
	if node["type"] != "string" || node["minLength"] != float64(1) {
		t.Errorf("description = %v, want a string of minLength 1", node)
	}
	if _, ok := readJSONFile(t, "testdata/valid/abilities/strike.json")["description"]; !ok {
		t.Error("testdata/valid/abilities/strike.json declares no description for the schema's tests to read")
	}
}

// VTT-858
func TestTheRulesetSchemaDescribesAnOptionalNonEmptyName(t *testing.T) {
	node, required := schemaField(t, "ruleset.schema.json", "free_usage_name")
	if required {
		t.Error("ruleset.schema.json lists free_usage_name as required")
	}
	if node["type"] != "string" || node["minLength"] != float64(1) {
		t.Errorf("free_usage_name = %v, want a string of minLength 1", node)
	}
	if _, ok := readJSONFile(t, "testdata/valid/ruleset.json")["free_usage_name"]; !ok {
		t.Error("testdata/valid/ruleset.json declares no free_usage_name for the schema's tests to read")
	}
}
