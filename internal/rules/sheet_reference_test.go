package rules_test

import (
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// referenceRuleset is placesManifest's mike and kilo and a hook holding a
// crate once, as testdata/valid's does, with entries echo of zulu and delta of
// yankee beside the crates.
func referenceRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := placesManifest(t, func(m map[string]any) {
		m["places"] = append(asList(m["places"]), placeDoc{"key": "hook", "name": "Hook",
			"description": "Holds a crate once.", "forms": []any{"crate"}})
	})
	catalogue := filepath.Join(dir, "catalogue")
	writeJSONFile(t, filepath.Join(catalogue, "echo.json"),
		entryDoc{"id": "echo", "name": "Echo", "description": "An entry of zulu.", "form": "zulu"})
	writeJSONFile(t, filepath.Join(catalogue, "delta.json"),
		entryDoc{"id": "delta", "name": "Delta", "description": "An entry of yankee.", "form": "yankee"})
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func ref(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func referenceFault(id, reason string) *rules.SheetFault {
	return &rules.SheetFault{Field: "references", Key: id, Reason: `reference "` + id + `" ` + reason}
}

func expectReferenceFault(t *testing.T, got, want *rules.SheetFault) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("fault = %+v, want nil", *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("fault = %+v, want %+v", got, *want)
	}
}

// VTT-1037
func TestReferenceFaultReadsAReferenceAgainstTheCatalogueAndItsPlace(t *testing.T) {
	cases := []struct {
		name string
		r    *vttv1.Reference
		want *rules.SheetFault
	}{
		{"an entry of a form its place holds, in a state the place names", ref("echo", "mike", "lima"), nil},
		{"the place's other form and other state", ref("delta", "mike", "oscar"), nil},
		{"no state, in a place naming none", ref("delta", "kilo", ""), nil},
		{"an entry the catalogue does not hold", ref("victor", "mike", "lima"),
			referenceFault("r", `names entry "victor", which the catalogue does not hold`)},
		{"a place the ruleset does not declare", ref("echo", "whiskey", "lima"),
			referenceFault("r", `is in place "whiskey", which the ruleset does not declare`)},
		{"an entry of a form its place does not hold", ref("echo", "kilo", ""),
			referenceFault("r", `names entry "echo" of form "zulu", which place "kilo" does not hold`)},
		{"a state its place does not name", ref("echo", "mike", "papa"),
			referenceFault("r", `is in state "papa", which place "mike" does not name`)},
		{"no state, in a place naming its states", ref("echo", "mike", ""),
			referenceFault("r", `names no state, and place "mike" names its states`)},
		{"another place's state, in a place naming none", ref("delta", "kilo", "lima"),
			referenceFault("r", `is in state "lima", and place "kilo" names none`)},
		{"the entry is read before the place", ref("victor", "whiskey", ""),
			referenceFault("r", `names entry "victor", which the catalogue does not hold`)},
		{"the form is read before the state", ref("echo", "kilo", "lima"),
			referenceFault("r", `names entry "echo" of form "zulu", which place "kilo" does not hold`)},
	}
	rs := referenceRuleset(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectReferenceFault(t, rs.ReferenceFault("r", c.r), c.want)
		})
	}
}

type sheetReferenceCase struct {
	name string
	refs map[string]*vttv1.Reference
	want *rules.SheetFault
}

func runSheetReferenceCases(t *testing.T, cases []sheetReferenceCase) {
	t.Helper()
	rs := referenceRuleset(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectReferenceFault(t, rs.SheetReferenceFault(c.refs), c.want)
		})
	}
}

// VTT-1037
func TestSheetReferenceFaultAnswersTheFirstReferenceThatDoesNotFitInByteOrder(t *testing.T) {
	runSheetReferenceCases(t, []sheetReferenceCase{
		{name: "a sheet holding none fits"},
		{name: "an empty map fits", refs: map[string]*vttv1.Reference{}},
		{name: "every reference fits", refs: map[string]*vttv1.Reference{
			"a": ref("echo", "mike", "lima"), "b": ref("delta", "kilo", ""), "c": ref("plain-crate", "hook", "")}},
		{name: "a reference sorting after those that fit", refs: map[string]*vttv1.Reference{
			"a": ref("echo", "mike", "lima"), "b": ref("delta", "kilo", ""), "c": ref("echo", "mike", "")},
			want: referenceFault("c", `names no state, and place "mike" names its states`)},
		{name: "the first in byte order, upper case before lower", refs: map[string]*vttv1.Reference{
			"zz": ref("victor", "mike", "lima"), "b": ref("echo", "whiskey", ""), "B": ref("echo", "mike", "papa")},
			want: referenceFault("B", `is in state "papa", which place "mike" does not name`)},
	})
}

// VTT-1038
func TestSheetReferenceFaultRefusesASecondReferenceToOneEntryInAPlaceHoldingItOnce(t *testing.T) {
	runSheetReferenceCases(t, []sheetReferenceCase{
		{name: "one entry twice in a place holding it once", refs: map[string]*vttv1.Reference{
			"b": ref("delta", "kilo", ""), "a": ref("delta", "kilo", "")},
			want: referenceFault("b", `names entry "delta", which reference "a" holds in place "kilo" already, `+
				`and that place holds an entry once`)},
		{name: "the first in byte order, naming the first that holds it", refs: map[string]*vttv1.Reference{
			"zz": ref("delta", "kilo", ""), "b": ref("delta", "kilo", ""), "B": ref("delta", "kilo", "")},
			want: referenceFault("b", `names entry "delta", which reference "B" holds in place "kilo" already, `+
				`and that place holds an entry once`)},
		{name: "one entry twice in a place holding copies", refs: map[string]*vttv1.Reference{
			"a": ref("echo", "mike", "lima"), "b": ref("echo", "mike", "lima")}},
		{name: "one entry in a place holding copies and in one holding it once", refs: map[string]*vttv1.Reference{
			"a": ref("delta", "mike", "lima"), "b": ref("delta", "kilo", "")}},
		{name: "two entries in a place holding each once", refs: map[string]*vttv1.Reference{
			"a": ref("plain-crate", "hook", ""), "b": ref("sturdy-crate", "hook", "")}},
		{name: "a reference that does not fit is told first", refs: map[string]*vttv1.Reference{
			"a": ref("delta", "kilo", ""), "b": ref("delta", "kilo", ""), "c": ref("victor", "kilo", "")},
			want: referenceFault("c", `names entry "victor", which the catalogue does not hold`)},
	})
}
