package rules_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep zulu declared before yankee: the form a fault names in declared order
// must not be the first in byte order of form keys.
func itemValueRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"id":"item-values","name":"Item Values","format_version":"3","version":"1","properties":[
	{"key":"quebec","type":"number","name":"Quebec","description":"A number only yankee lists."},
	{"key":"romeo","type":"number","name":"Romeo","description":"A number both forms list."},
	{"key":"sierra","type":"number","name":"Sierra","description":"A number no form lists."},
	{"key":"tango","type":"resource","name":"Tango","description":"A resource."}],
	"forms":[
	{"key":"zulu","name":"Zulu","description":"The form declared first.","properties":["romeo"]},
	{"key":"yankee","name":"Yankee","description":"The form declared second.","properties":["quebec","romeo"]}]}`
	for name, body := range map[string]string{"ruleset.json": manifest, "guide.md": "# Item Values\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

type itemValueCase struct {
	name                  string
	attributes, resources []string
	want                  *rules.SheetFault
}

func itemValueFault(key, form string) *rules.SheetFault {
	return &rules.SheetFault{Field: "attributes", Key: key, Form: form,
		Reason: `holds property "` + key + `", which form "` + form + `" lists for its entries`}
}

func runItemValueCases(t *testing.T, rs *rules.Ruleset, cases []itemValueCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rs.SheetKeyFault(c.attributes, c.resources)
			if c.want == nil {
				if got != nil {
					t.Fatalf("SheetKeyFault = %+v, want nil", *got)
				}
				return
			}
			if got == nil || *got != *c.want {
				t.Fatalf("SheetKeyFault = %+v, want %+v", got, *c.want)
			}
		})
	}
}

// VTT-1010
func TestSheetKeyFaultRefusesAnAttributeACatalogueFormLists(t *testing.T) {
	runItemValueCases(t, itemValueRuleset(t), []itemValueCase{
		{name: "a number no form lists fits",
			attributes: []string{"sierra"}, resources: []string{"tango"}},
		{name: "a key only the second form lists",
			attributes: []string{"quebec"}, want: itemValueFault("quebec", "yankee")},
		{name: "a key no form lists is passed over",
			attributes: []string{"sierra", "quebec"}, want: itemValueFault("quebec", "yankee")},
		{name: "the first listed key in byte order, whatever form lists it",
			attributes: []string{"romeo", "quebec"}, want: itemValueFault("quebec", "yankee")},
		{name: "a listed key under resources is the other type's value",
			resources: []string{"quebec"},
			want: &rules.SheetFault{Field: "resources", Key: "quebec",
				Reason: `gives number property "quebec" a resource's value`}},
	})
}

// VTT-1010
func TestSheetKeyFaultNamesTheFirstFormInDeclaredOrderThatListsTheKey(t *testing.T) {
	runItemValueCases(t, itemValueRuleset(t), []itemValueCase{
		{name: "two forms list the key",
			attributes: []string{"romeo"}, want: itemValueFault("romeo", "zulu")},
	})
}

// VTT-1010 VTT-617 VTT-618
func TestSheetKeyFaultReadsAFormsKeyAfterTheOtherKeyFaults(t *testing.T) {
	runItemValueCases(t, itemValueRuleset(t), []itemValueCase{
		{name: "an undeclared attribute sorting after a listed one",
			attributes: []string{"quebec", "victor"},
			want: &rules.SheetFault{Field: "attributes", Key: "victor",
				Reason: `references undeclared property "victor"`}},
		{name: "a resource among the attributes, sorting after a listed one",
			attributes: []string{"quebec", "tango"},
			want: &rules.SheetFault{Field: "attributes", Key: "tango",
				Reason: `gives resource property "tango" a number's value`}},
		{name: "an undeclared resource",
			attributes: []string{"quebec"}, resources: []string{"victor"},
			want: &rules.SheetFault{Field: "resources", Key: "victor",
				Reason: `references undeclared property "victor"`}},
		{name: "a number among the resources",
			attributes: []string{"quebec"}, resources: []string{"sierra"},
			want: &rules.SheetFault{Field: "resources", Key: "sierra",
				Reason: `gives number property "sierra" a resource's value`}},
	})
}

// Keep this test: SPEC-025 reads a sheet's keys from a sorted copy that
// leaves its caller's keys in their order.
func TestSheetKeyFaultsFormSweepLeavesItsCallersKeysInTheirOrder(t *testing.T) {
	keys := []string{"sierra", "quebec"}
	itemValueRuleset(t).SheetKeyFault(keys, nil)
	if !slices.Equal(keys, []string{"sierra", "quebec"}) {
		t.Fatalf("keys = %v after SheetKeyFault, want them as passed", keys)
	}
}
