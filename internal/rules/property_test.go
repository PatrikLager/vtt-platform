package rules_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// baseProperties is testdata/valid's property list, built fresh per call.
func baseProperties() []map[string]any {
	return []map[string]any{
		{"key": "brawn", "type": "number", "name": "Brawn", "description": "Force behind a strike."},
		{"key": "grit", "type": "number", "name": "Grit", "description": "What a body shrugs off."},
		{"key": "guard", "type": "number", "name": "Guard", "description": "What a strike must beat."},
		{"key": "pool_a", "type": "resource", "name": "Pool A", "description": "What a guard stance spends.",
			"default_max_expr": "10 + @brawn",
			"thresholds": []map[string]any{
				{"when": "#pool_a", "apply_condition": "guarded", "remove_when_false": true},
			}},
	}
}

// writeProperties writes dir's ruleset.json from baseProperties after edit
// has changed them.
func writeProperties(t *testing.T, dir string, edit func(props []map[string]any)) {
	t.Helper()
	props := baseProperties()
	edit(props)
	raw, err := json.MarshalIndent(map[string]any{
		"id": "test-ruleset", "name": "Test Ruleset", "format_version": "3", "properties": props,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "ruleset.json"), string(raw))
}

// propertyRuleset is a copy of testdata/valid with its properties edited.
func propertyRuleset(t *testing.T, edit func(props []map[string]any)) string {
	t.Helper()
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeProperties(t, dir, edit)
	return dir
}

func assertRefused(t *testing.T, err error, subs ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load: want an error naming %q, got nil", subs)
	}
	for _, s := range subs {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), s)
		}
	}
}

func noEdit([]map[string]any) {}

// VTT-610 VTT-611
func TestAPropertyIsRefusedWithoutANameADescriptionOrARunType(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(p map[string]any)
		field string
		want  string
	}{
		{"no name", func(p map[string]any) { delete(p, "name") }, "properties[1].name", "must not be empty"},
		{"an empty name", func(p map[string]any) { p["name"] = "" }, "properties[1].name", "must not be empty"},
		{"no description", func(p map[string]any) { delete(p, "description") }, "properties[1].description", "must not be empty"},
		{"an empty description", func(p map[string]any) { p["description"] = "" }, "properties[1].description", "must not be empty"},
		{"a type the engine does not run", func(p map[string]any) { p["type"] = "category" }, "properties[1].type", `unknown type "category"`},
		{"an empty type", func(p map[string]any) { p["type"] = "" }, "properties[1].type", `unknown type ""`},
		{"no type", func(p map[string]any) { delete(p, "type") }, "properties[1].type", `unknown type ""`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := propertyRuleset(t, func(props []map[string]any) { c.edit(props[1]) })
			_, err := rules.Load(dir)
			assertRefused(t, err, "ruleset.json", `field "`+c.field+`"`, `property "grit"`, c.want)
		})
	}
	t.Run("every property declared whole loads", func(t *testing.T) {
		if _, err := rules.Load(propertyRuleset(t, noEdit)); err != nil {
			t.Fatalf("Load: %v", err)
		}
	})
}

// VTT-612
func TestAKeyDeclaredTwiceIsRefused(t *testing.T) {
	cases := []struct {
		name  string
		index int
	}{
		{"two numbers", 2},
		{"a number and a resource", 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := propertyRuleset(t, func(props []map[string]any) { props[c.index]["key"] = "grit" })
			_, err := rules.Load(dir)
			assertRefused(t, err, "ruleset.json", fmt.Sprintf(`field "properties[%d].key"`, c.index), `duplicate property key "grit"`)
		})
	}
}

// propertySite is one place a ruleset reads a property, and the type it
// reads there.
type propertySite struct {
	name  string
	file  string
	field string
	reads string
	place func(t *testing.T, dir, key string)
}

// drainAtom binds a resource-kind param, which testdata/valid has no atom for.
const drainAtom = `{
  "id": "drain",
  "params": [{ "name": "pool", "kind": "resource" }],
  "provides": [],
  "consumes": [],
  "contributes": [
    { "kind": "outcome", "key": null, "branch": "always",
      "effects": [{ "resource_change": { "resource": "{pool}", "delta_expr": "0 - 1" } }] }
  ]
}`

func drainAbility(key string) string {
	return `{
  "id": "drain",
  "name": "Drain",
  "usage": "at_will",
  "compose": [
    { "atom": "self-delivery", "bind": {} },
    { "atom": "drain", "bind": { "pool": "` + key + `" } }
  ]
}`
}

func propertySites() []propertySite {
	return []propertySite{
		{"a resource's default_max_expr", "ruleset.json", "properties[3].default_max_expr", "number", func(t *testing.T, dir, key string) {
			t.Helper()
			writeProperties(t, dir, func(props []map[string]any) { props[3]["default_max_expr"] = "10 + @" + key })
		}},
		{"a threshold's when", "ruleset.json", "properties[3].thresholds[0].when", "resource", func(t *testing.T, dir, key string) {
			t.Helper()
			writeProperties(t, dir, func(props []map[string]any) {
				props[3]["thresholds"] = []map[string]any{{"when": "#" + key, "apply_condition": "guarded", "remove_when_false": true}}
			})
		}},
		{"a resolution's roll", "strike.json", "compose[1].resolution.roll", "number", func(t *testing.T, dir, key string) {
			t.Helper()
			rulesetEdit(t, dir, "atoms/strike-roll.json", `"1d20 + @caster.{attack_stat}"`, `"1d20 + @caster.{attack_stat} + @caster.`+key+`"`)
		}},
		{"a resolution's vs", "strike.json", "compose[1].resolution.vs", "resource", func(t *testing.T, dir, key string) {
			t.Helper()
			rulesetEdit(t, dir, "atoms/strike-roll.json", `"@target.guard"`, `"#target.`+key+`"`)
		}},
		{"a resource change's delta_expr", "strike.json", "resource_change.delta_expr", "number", func(t *testing.T, dir, key string) {
			t.Helper()
			rulesetEdit(t, dir, "atoms/strike-damage.json", `"0 - {power}"`, `"0 - {power} - @target.`+key+`"`)
		}},
		{"a resource change's resource", "strike.json", "resource_change.resource", "resource", func(t *testing.T, dir, key string) {
			t.Helper()
			rulesetEdit(t, dir, "atoms/strike-damage.json", `"resource": "pool_a"`, `"resource": "`+key+`"`)
		}},
		{"a number param's binding", "strike.json", "compose[1].bind.attack_stat", "number", func(t *testing.T, dir, key string) {
			t.Helper()
			rulesetEdit(t, dir, "abilities/strike.json", `"attack_stat": "brawn"`, `"attack_stat": "`+key+`"`)
		}},
		{"a resource param's binding", "drain.json", "compose[1].bind.pool", "resource", func(t *testing.T, dir, key string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "atoms", "drain.json"), drainAtom)
			writeFile(t, filepath.Join(dir, "abilities", "drain.json"), drainAbility(key))
		}},
		{"an ability's cost", "guard-stance.json", "usage.limited.resource", "resource", func(t *testing.T, dir, key string) {
			t.Helper()
			rulesetEdit(t, dir, "abilities/guard-stance.json", `"resource": "pool_a"`, `"resource": "`+key+`"`)
		}},
	}
}

// loadWithKeyAt loads testdata/valid with key placed at site.
func loadWithKeyAt(t *testing.T, site propertySite, key string) error {
	t.Helper()
	dir := propertyRuleset(t, noEdit)
	site.place(t, dir, key)
	_, err := rules.Load(dir)
	return err
}

// VTT-613
func TestAnUndeclaredKeyIsRefusedWhereverItIsNamed(t *testing.T) {
	for _, site := range propertySites() {
		t.Run(site.name, func(t *testing.T) {
			err := loadWithKeyAt(t, site, "nope")
			assertRefused(t, err, site.file, site.field, `references undeclared property "nope"`)
		})
	}
}

// VTT-614
func TestAPropertyReadAsTheOtherTypeIsRefused(t *testing.T) {
	other := map[string]struct{ key, want string }{
		"number":   {"pool_a", `reads resource property "pool_a" as a number`},
		"resource": {"grit", `reads number property "grit" as a resource`},
	}
	own := map[string]string{"number": "brawn", "resource": "pool_a"}
	for _, site := range propertySites() {
		t.Run(site.name, func(t *testing.T) {
			o := other[site.reads]
			assertRefused(t, loadWithKeyAt(t, site, o.key), site.file, site.field, o.want)
		})
		t.Run(site.name+" of its own type loads", func(t *testing.T) {
			if err := loadWithKeyAt(t, site, own[site.reads]); err != nil {
				t.Fatalf("Load: %v", err)
			}
		})
	}
}

// VTT-615
func TestANumberPropertyCarriesNoResourceField(t *testing.T) {
	cases := []struct {
		name  string
		field string
		value any
	}{
		{"default_max_expr", "default_max_expr", "2"},
		{"thresholds, even none", "thresholds", []any{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := propertyRuleset(t, func(props []map[string]any) { props[1][c.field] = c.value })
			_, err := rules.Load(dir)
			assertRefused(t, err, "ruleset.json", `field "properties[1].`+c.field+`"`, `property "grit" is a number`)
		})
	}
}

// replacedManifest is testdata/valid's ruleset.json as format v2 wrote it.
const replacedManifest = `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "2",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        {
          "when": "#pool_a",
          "apply_condition": "guarded",
          "remove_when_false": true
        }
      ]
    }
  ]
}`

// VTT-616
func TestARulesetInTheReplacedFormatIsRefusedWithWhatToDo(t *testing.T) {
	const what = `this loader reads format_version "3", which declares every property under "properties" with its key, type, name and description (docs/adr/012-the-rules-contract.md)`
	cases := []struct {
		name, written, want string
	}{
		{"format v2", `"format_version": "2"`, `unsupported value "2": ` + what},
		{"format v1", `"format_version": "1"`, `unsupported value "1": ` + what},
		{"an empty version", `"format_version": ""`, `unsupported value "": ` + what},
		{"a number", `"format_version": 3`, `unsupported value 3: ` + what},
		{"no version", `"no_version": "2"`, `is absent: ` + what},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			copyDir(t, fixture(t, "valid"), dir)
			path := filepath.Join(dir, "ruleset.json")
			writeFile(t, path, strings.Replace(replacedManifest, `"format_version": "2"`, c.written, 1))
			_, err := rules.Load(dir)
			want := fmt.Sprintf(`rules: %s: field "format_version": %s`, path, c.want)
			if err == nil || err.Error() != want {
				t.Errorf("Load:\n got %v\nwant %s", err, want)
			}
		})
	}
}

// numberProps is a number property per key, for a hand-built Ruleset.
func numberProps(keys ...string) []rules.Property {
	out := make([]rules.Property, 0, len(keys))
	for _, k := range keys {
		out = append(out, rules.Property{Key: k, Type: rules.PropertyNumber, Name: k, Description: k})
	}
	return out
}

// resourceProp is a resource property under key, for a hand-built Ruleset.
func resourceProp(key string, thresholds ...rules.Threshold) rules.Property {
	return rules.Property{Key: key, Type: rules.PropertyResource, Name: key, Description: key,
		Resource: &rules.ResourceDef{Thresholds: thresholds}}
}
