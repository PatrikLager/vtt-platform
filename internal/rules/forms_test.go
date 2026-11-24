package rules_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type formDoc = map[string]any

// VTT-993
func formsManifest(t *testing.T, edit func(m map[string]any)) string {
	t.Helper()
	dir := t.TempDir()
	copyDir(t, "testdata/valid", dir)
	dropCatalogue(t, dir)
	path := filepath.Join(dir, "ruleset.json")
	m := readJSONFile(t, path)
	edit(m)
	writeJSONFile(t, path, m)
	return dir
}

func formsRuleset(t *testing.T, forms ...any) string {
	t.Helper()
	return formsManifest(t, func(m map[string]any) { m["forms"] = forms })
}

func crateForm() formDoc {
	return formDoc{"key": "crate", "name": "Crate", "description": "The fixture's crate.", "properties": []any{"heft"}}
}

func plainForm() formDoc {
	return formDoc{"key": "qa-form", "name": "Plain Form", "description": "A plain form.", "properties": []any{"heft", "grit"}}
}

func expectFormRefusal(t *testing.T, dir, field, msg string) {
	t.Helper()
	_, err := rules.Load(dir)
	want := fmt.Sprintf("rules: %s: field %q: %s", filepath.Join(dir, "ruleset.json"), field, msg)
	if err == nil || err.Error() != want {
		t.Fatalf("Load = %v, want %q", err, want)
	}
}

func expectForms(t *testing.T, got, want []rules.Form) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Forms = %+v, want %+v", got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Key != w.Key || g.Name != w.Name || g.Description != w.Description || !slices.Equal(g.Properties, w.Properties) {
			t.Errorf("Forms[%d] = %+v, want %+v", i, g, w)
		}
	}
}

// VTT-993
func TestARulesetKeepsItsFormsInDeclaredOrder(t *testing.T) {
	rs, err := rules.Load(formsRuleset(t,
		plainForm(),
		formDoc{"key": "bare-form", "name": "Bare Form", "description": "A form that lists nothing."},
		formDoc{"key": "other-form", "name": "Other Form", "description": "A form sharing a key.", "properties": []any{"guard", "grit"}},
		formDoc{"key": "empty-form", "name": "Empty Form", "description": "A form listing an empty list.", "properties": []any{}},
	))
	if err != nil {
		t.Fatal(err)
	}
	expectForms(t, rs.Forms, []rules.Form{
		{Key: "qa-form", Name: "Plain Form", Description: "A plain form.", Properties: []string{"heft", "grit"}},
		{Key: "bare-form", Name: "Bare Form", Description: "A form that lists nothing."},
		{Key: "other-form", Name: "Other Form", Description: "A form sharing a key.", Properties: []string{"guard", "grit"}},
		{Key: "empty-form", Name: "Empty Form", Description: "A form listing an empty list."},
	})
}

// VTT-993
func TestTheFixtureKeepsItsForm(t *testing.T) {
	rs, err := rules.Load("testdata/valid")
	if err != nil {
		t.Fatal(err)
	}
	expectForms(t, rs.Forms, []rules.Form{
		{Key: "crate", Name: "Crate", Description: "The fixture's crate.", Properties: []string{"heft"}},
	})
}

// VTT-993
func TestARulesetDeclaringNoFormsLoadsWithNone(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m map[string]any)
	}{
		{"absent", func(m map[string]any) { delete(m, "forms") }},
		{"null", func(m map[string]any) { m["forms"] = nil }},
		{"empty", func(m map[string]any) { m["forms"] = []any{} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs, err := rules.Load(formsManifest(t, c.edit))
			if err != nil {
				t.Fatal(err)
			}
			if rs.Forms != nil {
				t.Errorf("Forms = %+v, want none", rs.Forms)
			}
		})
	}
}

// VTT-994
func TestAFormIsRefusedWithoutItsKeyNameOrDescription(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		edit             func(f formDoc)
	}{
		{"key absent", "forms[1].key", "must not be empty", func(f formDoc) { delete(f, "key") }},
		{"key empty", "forms[1].key", "must not be empty", func(f formDoc) { f["key"] = "" }},
		{"name absent", "forms[1].name", `form "qa-form": must not be empty`, func(f formDoc) { delete(f, "name") }},
		{"name empty", "forms[1].name", `form "qa-form": must not be empty`, func(f formDoc) { f["name"] = "" }},
		{"description absent", "forms[1].description", `form "qa-form": must not be empty`, func(f formDoc) { delete(f, "description") }},
		{"description empty", "forms[1].description", `form "qa-form": must not be empty`, func(f formDoc) { f["description"] = "" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := plainForm()
			c.edit(f)
			expectFormRefusal(t, formsRuleset(t, crateForm(), f), c.field, c.msg)
		})
	}
}

// VTT-994
func TestAFormKeyAnEarlierFormDeclaresIsRefused(t *testing.T) {
	f := plainForm()
	f["key"] = "crate"
	expectFormRefusal(t, formsRuleset(t, crateForm(), f), "forms[1].key", `duplicate form key "crate"`)
}

// VTT-994
func TestAFormKeyPastTheIDBoundIsRefused(t *testing.T) {
	for _, key := range []string{strings.Repeat("k", 129), strings.Repeat("é", 64) + "k"} {
		t.Run(fmt.Sprint(len([]rune(key))), func(t *testing.T) {
			if len(key) != 129 {
				t.Fatalf("fixture key is %d bytes, want 129", len(key))
			}
			f := plainForm()
			f["key"] = key
			expectFormRefusal(t, formsRuleset(t, crateForm(), f), "forms[1].key", "must be at most 128 bytes, got 129")
		})
	}
}

// VTT-994
func TestAFormKeyAtTheIDBoundLoads(t *testing.T) {
	for _, key := range []string{strings.Repeat("k", 128), strings.Repeat("é", 64)} {
		t.Run(fmt.Sprint(len([]rune(key))), func(t *testing.T) {
			if len(key) != 128 {
				t.Fatalf("fixture key is %d bytes, want 128", len(key))
			}
			f := plainForm()
			f["key"] = key
			rs, err := rules.Load(formsRuleset(t, crateForm(), f))
			if err != nil {
				t.Fatalf("a form key of 128 bytes was refused: %v", err)
			}
			if len(rs.Forms) != 2 || rs.Forms[1].Key != key {
				t.Errorf("Forms = %+v, want the key as written", rs.Forms)
			}
		})
	}
}

// VTT-995
func TestAFormListingAPropertyItMayNotCarryIsRefused(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		properties       []any
	}{
		{"undeclared", "forms[1].properties[1]", `names "nowhere", which no property declares`, []any{"heft", "nowhere"}},
		{"empty", "forms[1].properties[0]", `names "", which no property declares`, []any{"", "heft"}},
		{"listed twice", "forms[1].properties[2]", `names "heft", which the form lists already`, []any{"heft", "grit", "heft"}},
		{"a resource", "forms[1].properties[1]", `names resource property "pool_a"; a form lists number properties only`, []any{"heft", "pool_a"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := plainForm()
			f["properties"] = c.properties
			expectFormRefusal(t, formsRuleset(t, crateForm(), f), c.field, c.msg)
		})
	}
}

// VTT-996
func TestACaseVariantOfFormsIsReadAsForms(t *testing.T) {
	rs, err := rules.Load(formsManifest(t, func(m map[string]any) {
		delete(m, "forms")
		m["Forms"] = []any{formDoc{"KEY": "qa-form", "Name": "Plain Form", "DESCRIPTION": "A plain form.", "Properties": []any{"heft"}}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	expectForms(t, rs.Forms, []rules.Form{{Key: "qa-form", Name: "Plain Form", Description: "A plain form.", Properties: []string{"heft"}}})
}

// VTT-996
func TestAFieldAFormDoesNotDefineIsRefused(t *testing.T) {
	for _, field := range []string{"colour", "extends"} {
		t.Run(field, func(t *testing.T) {
			f := plainForm()
			f[field] = "crate"
			dir := formsRuleset(t, crateForm(), f)
			_, err := rules.Load(dir)
			if err == nil || !strings.Contains(err.Error(), filepath.Join(dir, "ruleset.json")) ||
				!strings.Contains(err.Error(), fmt.Sprintf("unknown field %q", field)) {
				t.Fatalf("Load = %v, want a refusal naming ruleset.json and the field %s", err, field)
			}
		})
	}
}

// VTT-1008
func TestTheSchemaDescribesEveryFormField(t *testing.T) {
	forms, required := schemaField(t, "ruleset.schema.json", "forms")
	if required {
		t.Error("ruleset.schema.json lists forms as required")
	}
	if forms["type"] != "array" {
		t.Errorf("forms type = %v, want array", forms["type"])
	}
	item, _ := forms["items"].(map[string]any)
	if item == nil {
		t.Fatal("ruleset.schema.json describes no form under forms' items")
	}
	if item["type"] != "object" || item["additionalProperties"] != false {
		t.Errorf("a form is %v with additionalProperties %v, want an object admitting no other field", item["type"], item["additionalProperties"])
	}
	fields, _ := item["properties"].(map[string]any)
	want := []string{"key", "name", "description", "properties"}
	if len(fields) != len(want) {
		t.Errorf("a form holds %d fields, the format defines %v", len(fields), want)
	}
	for _, k := range want[:3] {
		f, _ := fields[k].(map[string]any)
		if f["type"] != "string" || f["minLength"] != float64(1) {
			t.Errorf("form %s = %v, want a string of minLength 1", k, f)
		}
	}
	keyDesc, _ := asMap(fields["key"])["description"].(string)
	if !strings.Contains(keyDesc, "At most 128 bytes") {
		t.Errorf("form key description = %q, want it to state %q", keyDesc, "At most 128 bytes")
	}
	list := asMap(fields["properties"])
	if list["type"] != "array" || list["uniqueItems"] != true || asMap(list["items"])["type"] != "string" {
		t.Errorf("form properties = %v, want an array of distinct strings", list)
	}
	var req []string
	for _, r := range asList(item["required"]) {
		req = append(req, r.(string))
	}
	slices.Sort(req)
	if !slices.Equal(req, []string{"description", "key", "name"}) {
		t.Errorf("a form requires %v, want key, name and description", req)
	}

	fixture := asList(readJSONFile(t, "testdata/valid/ruleset.json")["forms"])
	if len(fixture) == 0 {
		t.Fatal("testdata/valid/ruleset.json declares no form to describe")
	}
	seen := map[string]bool{}
	for _, f := range fixture {
		for k := range asMap(f) {
			seen[k] = true
			if _, ok := fields[k]; !ok {
				t.Errorf("testdata/valid/ruleset.json uses form field %q, which ruleset.schema.json does not describe", k)
			}
		}
	}
	for _, k := range want {
		if !seen[k] {
			t.Errorf("testdata/valid/ruleset.json's forms never use %s, so this test cannot hold the schema to it", k)
		}
	}
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
