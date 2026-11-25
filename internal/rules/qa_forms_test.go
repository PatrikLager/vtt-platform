package rules_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func qaFMCopyBase(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "rulesets", "tavern-brawl")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o600)
	})
	if err != nil {
		t.Fatalf("copy base ruleset: %v", err)
	}
	return dst
}

func qaFMManifest(t *testing.T, edit func(m map[string]any)) string {
	t.Helper()
	dir := qaFMCopyBase(t)
	path := filepath.Join(dir, "ruleset.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	edit(m)
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}

func qaFMWithForms(t *testing.T, forms any) string {
	t.Helper()
	return qaFMManifest(t, func(m map[string]any) { m["forms"] = forms })
}

func qaFMForm(key, name, desc string, props ...string) map[string]any {
	if props == nil {
		props = []string{}
	}
	return map[string]any{
		"key": key, "name": name, "description": desc, "properties": props,
	}
}

func qaFMLoads(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load refused: %v", err)
	}
	return rs
}

// Warning: the temp dir's path carries the test's name, so it is cut out of
// the message before the field is looked for.
func qaFMRefused(t *testing.T, dir, field string) {
	t.Helper()
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatalf("Load succeeded; want a refusal naming the field %q", field)
	}
	msg := err.Error()
	t.Logf("refusal: %s", msg)
	if !strings.Contains(msg, "ruleset.json") {
		t.Fatalf("refusal %q does not name ruleset.json", msg)
	}
	rest := strings.ReplaceAll(msg, dir, "")
	if resolved, rerr := filepath.EvalSymlinks(dir); rerr == nil {
		rest = strings.ReplaceAll(rest, resolved, "")
	}
	if !strings.Contains(rest, field) {
		t.Fatalf("refusal %q does not name the field %q", msg, field)
	}
}

func TestQAFMTheBaseLoadsWithNoForms(t *testing.T) {
	rs := qaFMLoads(t, qaFMCopyBase(t))
	if len(rs.Forms) != 0 { // VTT-993
		t.Fatalf("Forms = %+v, want none", rs.Forms)
	}
}

func TestQAFMFormsAreKeptAsWrittenInDeclaredOrder(t *testing.T) {
	want := []rules.Form{
		{Key: "zulu", Name: "Ünit Zulu", Description: "The last by name.",
			Properties: []string{"footing", "brawn"}},
		{Key: "alpha", Name: "Ünit Alpha", Description: "The first by name.",
			Properties: []string{"grit"}},
		{Key: "mike", Name: "Ünit Mike", Description: "Between the two.",
			Properties: []string{"grit", "footing", "brawn"}},
	}
	forms := make([]any, 0, len(want))
	for _, f := range want {
		forms = append(forms,
			qaFMForm(f.Key, f.Name, f.Description, f.Properties...))
	}
	rs := qaFMLoads(t, qaFMWithForms(t, forms))
	if len(rs.Forms) != len(want) { // VTT-993
		t.Fatalf("Forms = %+v, want %+v", rs.Forms, want)
	}
	for i := range want {
		got := rs.Forms[i]
		if got.Key != want[i].Key || got.Name != want[i].Name ||
			got.Description != want[i].Description ||
			!slices.Equal(got.Properties, want[i].Properties) { // VTT-993
			t.Fatalf("Forms[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestQAFMAFormListingNoKeysLoadsWithNone(t *testing.T) {
	rs := qaFMLoads(t, qaFMWithForms(t, []any{
		qaFMForm("first", "First", "One that lists keys.", "brawn"),
		qaFMForm("second", "Second", "One that lists none."),
	}))
	if len(rs.Forms) != 2 || len(rs.Forms[1].Properties) != 0 { // VTT-993
		t.Fatalf("Forms = %+v, want the second listing no keys", rs.Forms)
	}
}

func TestQAFMNoFormsNullFormsAndEmptyFormsLoadWithNone(t *testing.T) {
	cases := map[string]func(m map[string]any){
		"absent": func(m map[string]any) { delete(m, "forms") },
		"null":   func(m map[string]any) { m["forms"] = nil },
		"empty":  func(m map[string]any) { m["forms"] = []any{} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			rs := qaFMLoads(t, qaFMManifest(t, edit))
			if len(rs.Forms) != 0 { // VTT-993
				t.Fatalf("Forms = %+v, want none", rs.Forms)
			}
		})
	}
}

func TestQAFMAFormWithoutAKeyNameOrDescriptionIsRefused(t *testing.T) {
	for _, field := range []string{"key", "name", "description"} {
		for _, mode := range []string{"absent", "empty"} {
			t.Run(field+"-"+mode, func(t *testing.T) {
				bad := qaFMForm("second", "Second", "The faulty one.", "grit")
				if mode == "absent" {
					delete(bad, field)
				} else {
					bad[field] = ""
				}
				dir := qaFMWithForms(t, []any{
					qaFMForm("first", "First", "A sound one.", "brawn"),
					bad,
				})
				qaFMRefused(t, dir, field) // VTT-994
			})
		}
	}
}

func TestQAFMAFormKeyPastTheBoundIsRefused(t *testing.T) {
	cases := map[string]string{
		"single-byte": strings.Repeat("k", 129),
		"multi-byte":  strings.Repeat("€", 43),
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			dir := qaFMWithForms(t, []any{
				qaFMForm("first", "First", "A sound one.", "brawn"),
				qaFMForm(key, "Long", "Its key is past the bound.", "grit"),
			})
			qaFMRefused(t, dir, "key") // VTT-994
		})
	}
}

func TestQAFMAFormKeyOfExactlyTheBoundLoads(t *testing.T) {
	cases := map[string]string{
		"single-byte": strings.Repeat("k", 128),
		"multi-byte":  strings.Repeat("é", 64),
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			rs := qaFMLoads(t, qaFMWithForms(t, []any{
				qaFMForm("first", "First", "A sound one.", "brawn"),
				qaFMForm(key, "Long", "Its key is at the bound.", "grit"),
			}))
			if len(rs.Forms) != 2 || rs.Forms[1].Key != key { // VTT-994
				t.Fatalf("Forms = %+v, want the second keyed %q", rs.Forms, key)
			}
		})
	}
}

func TestQAFMAFormKeyAnEarlierFormDeclaresIsRefused(t *testing.T) {
	dir := qaFMWithForms(t, []any{
		qaFMForm("first", "First", "The one declaring it.", "brawn"),
		qaFMForm("second", "Second", "Another.", "grit"),
		qaFMForm("first", "Again", "Declares the key again.", "footing"),
	})
	qaFMRefused(t, dir, "key") // VTT-994
}

func TestQAFMAFormListingAKeyNoPropertyDeclaresIsRefused(t *testing.T) {
	for name, listed := range map[string]string{
		"undeclared": "nowhere", "empty": "",
	} {
		t.Run(name, func(t *testing.T) {
			dir := qaFMWithForms(t, []any{
				qaFMForm("first", "First", "A sound one.", "brawn"),
				qaFMForm("second", "Second", "Lists a stray.", "grit", listed),
			})
			qaFMRefused(t, dir, "properties") // VTT-995
		})
	}
}

func TestQAFMAFormListingAKeyTwiceIsRefused(t *testing.T) {
	dir := qaFMWithForms(t, []any{
		qaFMForm("first", "First", "A sound one.", "brawn"),
		qaFMForm("second", "Second", "Lists one twice.",
			"grit", "footing", "grit"),
	})
	qaFMRefused(t, dir, "properties") // VTT-995
}

func TestQAFMAFormListingAResourcePropertyIsRefused(t *testing.T) {
	for _, listed := range []string{"drink", "stamina"} {
		t.Run(listed, func(t *testing.T) {
			dir := qaFMWithForms(t, []any{
				qaFMForm("first", "First", "A sound one.", "brawn"),
				qaFMForm("second", "Second", "Lists a resource.", "grit", listed),
			})
			qaFMRefused(t, dir, "properties") // VTT-995
		})
	}
}

func TestQAFMAnUndefinedTopLevelFieldIsRefused(t *testing.T) {
	for _, field := range []string{"catalog", "form"} {
		t.Run(field, func(t *testing.T) {
			dir := qaFMManifest(t, func(m map[string]any) {
				m["forms"] = []any{qaFMForm("first", "First", "A sound one.")}
				m[field] = []any{}
			})
			qaFMRefused(t, dir, `unknown field "`+field+`"`) // VTT-1018
		})
	}
}

func TestQAFMAnUndefinedFieldInAFormIsRefused(t *testing.T) {
	for _, field := range []string{"extra", "type"} {
		t.Run(field, func(t *testing.T) {
			bad := qaFMForm("second", "Second", "Holds a stray field.", "grit")
			bad[field] = "number"
			dir := qaFMWithForms(t, []any{
				qaFMForm("first", "First", "A sound one.", "brawn"),
				bad,
			})
			qaFMRefused(t, dir, field) // VTT-1018
		})
	}
}

func TestQAFMAnUndefinedFieldInTheSheetIsRefused(t *testing.T) {
	dir := qaFMManifest(t, func(m map[string]any) {
		m["sheet"] = []any{map[string]any{
			"name":     "Page",
			"sections": []any{map[string]any{"name": "S", "values": []any{"grit"}}},
			"stray":    true,
		}}
	})
	qaFMRefused(t, dir, "stray") // VTT-1018
}

func TestQAFMCaseVariantsOfFormsAndItsFieldsAreReadAsThem(t *testing.T) {
	cases := map[string][5]string{
		"upper": {"FORMS", "KEY", "NAME", "DESCRIPTION", "PROPERTIES"},
		"title": {"Forms", "Key", "Name", "Description", "Properties"},
		"mixed": {"fOrMs", "kEy", "nAmE", "DeScRiPtIoN", "pRoPeRtIeS"},
	}
	for name, n := range cases {
		t.Run(name, func(t *testing.T) {
			dir := qaFMManifest(t, func(m map[string]any) {
				m[n[0]] = []any{
					map[string]any{n[1]: "one", n[2]: "One", n[3]: "First.",
						n[4]: []any{"footing", "grit"}},
					map[string]any{n[1]: "two", n[2]: "Two", n[3]: "Second.",
						n[4]: []any{"brawn"}},
				}
			})
			rs := qaFMLoads(t, dir)
			want := []rules.Form{
				{Key: "one", Name: "One", Description: "First.",
					Properties: []string{"footing", "grit"}},
				{Key: "two", Name: "Two", Description: "Second.",
					Properties: []string{"brawn"}},
			}
			if len(rs.Forms) != len(want) { // VTT-1018
				t.Fatalf("Forms = %+v, want %+v", rs.Forms, want)
			}
			for i := range want {
				got := rs.Forms[i]
				if got.Key != want[i].Key || got.Name != want[i].Name ||
					got.Description != want[i].Description ||
					!slices.Equal(got.Properties, want[i].Properties) { // VTT-1018
					t.Fatalf("Forms[%d] = %+v, want %+v", i, got, want[i])
				}
			}
		})
	}
}

type qaFMField struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type qaFMSchema struct {
	Required   []string `json:"required"`
	Properties map[string]struct {
		Type  string `json:"type"`
		Items struct {
			Required   []string             `json:"required"`
			Properties map[string]qaFMField `json:"properties"`
		} `json:"items"`
	} `json:"properties"`
}

func TestQAFMTheSchemaDescribesFormsAndTheKeyBound(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("schema", "ruleset.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var s qaFMSchema
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	forms, ok := s.Properties["forms"]
	if !ok || forms.Type != "array" { // VTT-1008
		t.Fatalf("schema has no array forms: %+v", forms)
	}
	if slices.Contains(s.Required, "forms") { // VTT-993, VTT-1008
		t.Fatalf("schema requires forms: %v", s.Required)
	}
	for _, f := range []string{"key", "name", "description", "properties"} {
		if _, ok := forms.Items.Properties[f]; !ok { // VTT-1008
			t.Errorf("a form's schema does not describe %q", f)
		}
	}
	for _, f := range []string{"key", "name", "description"} {
		if !slices.Contains(forms.Items.Required, f) { // VTT-1008
			t.Errorf("a form's schema does not require %q: %v",
				f, forms.Items.Required)
		}
	}
	keyDoc := forms.Items.Properties["key"].Description
	if !strings.Contains(keyDoc, "128 bytes") { // VTT-1008, VTT-1022
		t.Errorf("a form key's description %q does not state 128 bytes", keyDoc)
	}
}
