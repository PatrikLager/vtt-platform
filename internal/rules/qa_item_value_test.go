package rules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaivFirstFormKey   = "zulu"
	qaivFirstFormName  = "Lima"
	qaivSecondFormKey  = "yankee"
	qaivSecondFormName = "Kilo"
)

// Keep zulu declared before yankee, and romeo before quebec in zulu: the
// order tests need declared order to differ from byte order.
func qaivRuleset(t *testing.T, withForms bool) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rs")
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("testdata", "valid"))); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	props, _ := m["properties"].([]any)
	for _, k := range []string{"quebec", "romeo", "sierra", "tango", "QUEBEC"} {
		props = append(props, map[string]any{
			"key": k, "type": "number", "name": "N " + k,
			"description": "An invented number.",
		})
	}
	props = append(props, map[string]any{
		"key": "uniform", "type": "resource", "name": "R uniform",
		"description": "An invented resource.",
	})
	m["properties"] = props
	if withForms {
		forms, _ := m["forms"].([]any)
		m["forms"] = append([]any{
			map[string]any{
				"key": qaivFirstFormKey, "name": qaivFirstFormName,
				"description": "An invented form.",
				"properties":  []string{"romeo", "quebec"},
			},
			map[string]any{
				"key": qaivSecondFormKey, "name": qaivSecondFormName,
				"description": "Another invented form.",
				"properties":  []string{"quebec", "tango"},
			},
		}, forms...)
		entry := `{"id": "qaiv-one", "name": "One", "description": "An entry.",
"form": "zulu", "values": {"quebec": 3}}`
		p := filepath.Join(dir, "catalogue", "qaiv-one.json")
		if err := os.WriteFile(p, []byte(entry), 0o644); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	} else {
		delete(m, "forms")
		dropPlaces(m)
		if err := os.RemoveAll(filepath.Join(dir, "catalogue")); err != nil {
			t.Fatalf("remove catalogue: %v", err)
		}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return rs
}

func qaivFormIn(reason, key, name string) bool {
	return strings.Contains(reason, key) || strings.Contains(reason, name)
}

func qaivWant(t *testing.T, f *rules.SheetFault, field, key string) {
	t.Helper()
	if f == nil {
		t.Fatalf("got nil, want a fault on %s %q", field, key)
	}
	if f.Field != field || f.Key != key {
		t.Fatalf("got %s %q (%s), want %s %q", f.Field, f.Key, f.Reason, field, key)
	}
	if !strings.Contains(f.Reason, key) {
		t.Fatalf("reason %q does not name %q", f.Reason, key)
	}
}

// VTT-1010
func TestQAIVAnAttributeAFormListsIsRefused(t *testing.T) {
	rs := qaivRuleset(t, true)
	f := rs.SheetKeyFault([]string{"quebec"}, nil)
	qaivWant(t, f, "attributes", "quebec")
	if strings.Contains(f.Reason, "undeclared") {
		t.Fatalf("reason %q is the undeclared key's", f.Reason)
	}
	t.Logf("reason: %s", f.Reason)
}

// VTT-1010
func TestQAIVTheFirstFormInDeclaredOrderIsNamed(t *testing.T) {
	rs := qaivRuleset(t, true)
	f := rs.SheetKeyFault([]string{"quebec"}, nil)
	qaivWant(t, f, "attributes", "quebec")
	if !qaivFormIn(f.Reason, qaivFirstFormKey, qaivFirstFormName) {
		t.Fatalf("reason %q does not name the first form", f.Reason)
	}
	if qaivFormIn(f.Reason, qaivSecondFormKey, qaivSecondFormName) {
		t.Fatalf("reason %q names the second form", f.Reason)
	}
	f = rs.SheetKeyFault([]string{"tango"}, nil)
	qaivWant(t, f, "attributes", "tango")
	if !qaivFormIn(f.Reason, qaivSecondFormKey, qaivSecondFormName) {
		t.Fatalf("reason %q does not name the form that lists it", f.Reason)
	}
	if qaivFormIn(f.Reason, qaivFirstFormKey, qaivFirstFormName) {
		t.Fatalf("reason %q names a form that does not list it", f.Reason)
	}
	f = rs.SheetKeyFault([]string{"heft"}, nil)
	qaivWant(t, f, "attributes", "heft")
	if !strings.Contains(f.Reason, "crate") && !strings.Contains(f.Reason, "Crate") {
		t.Fatalf("reason %q does not name the third form", f.Reason)
	}
}

// VTT-1010
func TestQAIVTheFirstListedKeyInByteOrderIsNamed(t *testing.T) {
	rs := qaivRuleset(t, true)
	cases := []struct {
		attrs []string
		want  string
	}{
		{[]string{"sierra", "romeo", "quebec"}, "quebec"},
		{[]string{"romeo", "quebec"}, "quebec"},
		{[]string{"tango", "sierra", "romeo"}, "romeo"},
		{[]string{"sierra", "tango"}, "tango"},
		{[]string{"tango", "QUEBEC", "sierra"}, "tango"},
	}
	for _, c := range cases {
		qaivWant(t, rs.SheetKeyFault(c.attrs, nil), "attributes", c.want)
	}
}

// VTT-1010
func TestQAIVAnUndeclaredAttributeIsAnsweredFirst(t *testing.T) {
	rs := qaivRuleset(t, true)
	f := rs.SheetKeyFault([]string{"quebec", "xray"}, nil)
	qaivWant(t, f, "attributes", "xray")
	alone := rs.SheetKeyFault([]string{"xray"}, nil)
	if alone == nil || f.Reason != alone.Reason {
		t.Fatalf("reason %q is not the undeclared key's %v", f.Reason, alone)
	}
}

// VTT-1010
func TestQAIVAnAttributeOfTheOtherTypeIsAnsweredFirst(t *testing.T) {
	rs := qaivRuleset(t, true)
	f := rs.SheetKeyFault([]string{"quebec", "uniform"}, nil)
	qaivWant(t, f, "attributes", "uniform")
}

// VTT-1010
func TestQAIVAResourcesFaultIsAnsweredBeforeAListedAttribute(t *testing.T) {
	rs := qaivRuleset(t, true)
	qaivWant(t, rs.SheetKeyFault([]string{"quebec"}, []string{"sierra"}),
		"resources", "sierra")
	qaivWant(t, rs.SheetKeyFault([]string{"quebec"}, []string{"xray"}),
		"resources", "xray")
}

// VTT-1010
func TestQAIVAListedKeyUnderResourcesIsTheOtherType(t *testing.T) {
	rs := qaivRuleset(t, true)
	f := rs.SheetKeyFault(nil, []string{"quebec"})
	qaivWant(t, f, "resources", "quebec")
	plain := rs.SheetKeyFault(nil, []string{"sierra"})
	if plain == nil {
		t.Fatal("a number under resources fits")
	}
	if want := strings.ReplaceAll(plain.Reason, "sierra", "quebec"); f.Reason != want {
		t.Fatalf("reason %q, want the other type's %q", f.Reason, want)
	}
}

// VTT-1010
func TestQAIVASheetNoFormListsFits(t *testing.T) {
	rs := qaivRuleset(t, true)
	if f := rs.SheetKeyFault([]string{"sierra", "QUEBEC"}, []string{"uniform"}); f != nil {
		t.Fatalf("got %+v, want nil", f)
	}
	if f := rs.SheetKeyFault(nil, nil); f != nil {
		t.Fatalf("got %+v for an empty sheet, want nil", f)
	}
}

// VTT-1010
func TestQAIVWithNoFormAListedKeyFits(t *testing.T) {
	rs := qaivRuleset(t, false)
	if len(rs.Forms) != 0 {
		t.Fatalf("fixture declares forms %+v", rs.Forms)
	}
	if f := rs.SheetKeyFault([]string{"quebec", "romeo", "tango"}, nil); f != nil {
		t.Fatalf("got %+v, want nil", f)
	}
}

// VTT-1010
func TestQAIVTheSweepLeavesItsCallersKeysInTheirOrder(t *testing.T) {
	rs := qaivRuleset(t, true)
	attrs := []string{"tango", "sierra", "romeo", "quebec"}
	res := []string{"uniform"}
	wantA, wantR := slices.Clone(attrs), slices.Clone(res)
	qaivWant(t, rs.SheetKeyFault(attrs, res), "attributes", "quebec")
	if !slices.Equal(attrs, wantA) || !slices.Equal(res, wantR) {
		t.Fatalf("keys reordered to %v %v, want %v %v", attrs, res, wantA, wantR)
	}
}

// VTT-1010
func TestQAIVEveryKeyAShippedFormListsIsRefused(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "rulesets", "*", "ruleset.json"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no shipped ruleset found: %v", err)
	}
	listed := 0
	for _, manifest := range dirs {
		rs, err := rules.Load(filepath.Dir(manifest))
		if err != nil {
			t.Fatalf("load %s: %v", manifest, err)
		}
		first := map[string]string{}
		for _, f := range rs.Forms {
			for _, k := range f.Properties {
				if _, ok := first[k]; !ok {
					first[k] = f.Key
				}
			}
		}
		for _, p := range rs.Properties {
			if p.Type != rules.PropertyNumber {
				continue
			}
			f := rs.SheetKeyFault([]string{p.Key}, nil)
			form, isListed := first[p.Key]
			if !isListed {
				if f != nil {
					t.Fatalf("%s: unlisted %q refused: %+v", rs.ID, p.Key, f)
				}
				continue
			}
			listed++
			qaivWant(t, f, "attributes", p.Key)
			if !strings.Contains(f.Reason, `"`+form+`"`) {
				t.Fatalf("%s: reason %q does not name form %q", rs.ID, f.Reason, form)
			}
		}
	}
	if listed == 0 {
		t.Fatal("no shipped form lists a key")
	}
}
