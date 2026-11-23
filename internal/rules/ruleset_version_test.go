package rules_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// versionedRuleset is testdata/valid with its manifest's fields set as
// fields names them, a nil value deleting the field.
func versionedRuleset(t *testing.T, fields map[string]any) string {
	t.Helper()
	return describedRuleset(t, map[string]map[string]any{"ruleset.json": fields})
}

// VTT-970
func TestARulesetKeepsTheVersionItDeclares(t *testing.T) {
	for _, version := range []string{"1", " 2.0-rc1 ", "Ünder review"} {
		t.Run(version, func(t *testing.T) {
			rs, err := rules.Load(versionedRuleset(t, map[string]any{"version": version}))
			if err != nil {
				t.Fatal(err)
			}
			if rs.Version != version {
				t.Errorf("Version = %q, want %q", rs.Version, version)
			}
		})
	}
}

// VTT-970 VTT-973: a case variant of version is read as version.
func TestACaseVariantOfVersionIsReadAsTheVersion(t *testing.T) {
	rs, err := rules.Load(versionedRuleset(t, map[string]any{"version": nil, "VERSION": "7"}))
	if err != nil {
		t.Fatal(err)
	}
	if rs.Version != "7" {
		t.Errorf("Version = %q, want %q", rs.Version, "7")
	}
}

// VTT-971
func TestARulesetWithoutAVersionIsRefused(t *testing.T) {
	refused := func(t *testing.T, dir string) {
		t.Helper()
		_, err := rules.Load(dir)
		want := fmt.Sprintf("rules: %s: field %q: must not be empty", filepath.Join(dir, "ruleset.json"), "version")
		if err == nil || err.Error() != want {
			t.Fatalf("Load = %v, want %q", err, want)
		}
	}
	t.Run("absent", func(t *testing.T) {
		refused(t, versionedRuleset(t, map[string]any{"version": nil}))
	})
	t.Run("empty", func(t *testing.T) {
		refused(t, versionedRuleset(t, map[string]any{"version": ""}))
	})
	t.Run("null", func(t *testing.T) {
		dir := versionedRuleset(t, nil)
		path := filepath.Join(dir, "ruleset.json")
		obj := readJSONFile(t, path)
		obj["version"] = nil
		writeJSONFile(t, path, obj)
		refused(t, dir)
	})
}

// VTT-972
func TestARulesetWhoseIDVersionOrNameIsPastItsBoundIsRefused(t *testing.T) {
	cases := []struct {
		name, field, value string
		bound              int
	}{
		{"id of single bytes", "id", strings.Repeat("i", 129), 128},
		{"id of two-byte runes", "id", strings.Repeat("é", 64) + "i", 128},
		{"version of single bytes", "version", strings.Repeat("v", 129), 128},
		{"version of three-byte runes", "version", strings.Repeat("€", 43), 128},
		{"name of single bytes", "name", strings.Repeat("n", 257), 256},
		{"name of two-byte runes", "name", strings.Repeat("ñ", 128) + "n", 256},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.value) != c.bound+1 {
				t.Fatalf("fixture value is %d bytes, want %d", len(c.value), c.bound+1)
			}
			dir := versionedRuleset(t, map[string]any{c.field: c.value})
			_, err := rules.Load(dir)
			want := fmt.Sprintf("rules: %s: field %q: must be at most %d bytes, got %d",
				filepath.Join(dir, "ruleset.json"), c.field, c.bound, len(c.value))
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-972
func TestARulesetWhoseIDVersionAndNameAreAtTheirBoundsLoads(t *testing.T) {
	cases := []struct {
		name, id, version, rulesetName string
	}{
		{"single bytes", strings.Repeat("i", 128), strings.Repeat("v", 128), strings.Repeat("n", 256)},
		{"multi-byte runes", strings.Repeat("é", 64), strings.Repeat("€", 42) + "v1", strings.Repeat("ñ", 128)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.id) != 128 || len(c.version) != 128 || len(c.rulesetName) != 256 {
				t.Fatalf("fixture values are %d, %d and %d bytes", len(c.id), len(c.version), len(c.rulesetName))
			}
			rs, err := rules.Load(versionedRuleset(t, map[string]any{
				"id": c.id, "version": c.version, "name": c.rulesetName,
			}))
			if err != nil {
				t.Fatalf("a manifest whose id, version and name are at their bounds was refused: %v", err)
			}
			if rs.ID != c.id || rs.Version != c.version || rs.Name != c.rulesetName {
				t.Errorf("loaded id %q, version %q, name %q; want each as written", rs.ID, rs.Version, rs.Name)
			}
		})
	}
}

// VTT-973
func TestAnUndefinedFieldBesideTheVersionIsRefused(t *testing.T) {
	dir := versionedRuleset(t, map[string]any{"version": "1", "versions": "2"})
	_, err := rules.Load(dir)
	if err == nil || !strings.Contains(err.Error(), filepath.Join(dir, "ruleset.json")) ||
		!strings.Contains(err.Error(), `unknown field "versions"`) {
		t.Fatalf("Load = %v, want a refusal naming ruleset.json and the field versions", err)
	}
}

// VTT-986
func TestTheRulesetSchemaDescribesARequiredVersion(t *testing.T) {
	node, required := schemaField(t, "ruleset.schema.json", "version")
	if !required {
		t.Error("ruleset.schema.json does not list version as required")
	}
	if node["type"] != "string" || node["minLength"] != float64(1) {
		t.Errorf("version = %v, want a string of minLength 1", node)
	}
	if desc, _ := node["description"].(string); !strings.Contains(desc, "At most 128 bytes") {
		t.Errorf("version description = %q, want it to state %q", desc, "At most 128 bytes")
	}
	if _, ok := readJSONFile(t, "testdata/valid/ruleset.json")["version"]; !ok {
		t.Error("testdata/valid/ruleset.json declares no version for the schema's tests to read")
	}
}
