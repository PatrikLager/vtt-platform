package rules_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type entryDoc = map[string]any

func dropCatalogue(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(dir, "catalogue")); err != nil {
		t.Fatal(err)
	}
}

func writeCatalogue(t *testing.T, dir string, files map[string]entryDoc) {
	t.Helper()
	catalogue := filepath.Join(dir, "catalogue")
	dropCatalogue(t, dir)
	if err := os.MkdirAll(catalogue, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, e := range files {
		writeJSONFile(t, filepath.Join(catalogue, name), e)
	}
}

func catalogueRuleset(t *testing.T, files map[string]entryDoc) string {
	t.Helper()
	dir := formsRuleset(t, crateForm(), plainForm())
	writeCatalogue(t, dir, files)
	return dir
}

func plainEntry() entryDoc {
	return entryDoc{"id": "qa-entry", "name": "Plain Entry", "description": "A plain entry.", "form": "qa-form",
		"values": entryDoc{"heft": 3, "grit": -2}}
}

func crateEntry() entryDoc {
	return entryDoc{"id": "crate-entry", "name": "Crate Entry", "description": "An entry of the crate form.", "form": "crate",
		"values": entryDoc{"heft": 7}}
}

func withEntry(e entryDoc) map[string]entryDoc {
	return map[string]entryDoc{"a-crate.json": crateEntry(), "b-plain.json": e}
}

func expectEntryRefusal(t *testing.T, dir, file, field, msg string) {
	t.Helper()
	_, err := rules.Load(dir)
	want := fmt.Sprintf("rules: %s: field %q: %s", filepath.Join(dir, "catalogue", file), field, msg)
	if err == nil || err.Error() != want {
		t.Fatalf("Load = %v, want %q", err, want)
	}
}

func expectEntry(t *testing.T, rs *rules.Ruleset, want rules.Entry) {
	t.Helper()
	got := rs.Catalogue[want.ID]
	if got == nil {
		t.Fatalf("the catalogue holds no %q: %v", want.ID, slices.Sorted(maps.Keys(rs.Catalogue)))
	}
	if got.ID != want.ID || got.Name != want.Name || got.Description != want.Description || got.Form != want.Form ||
		!maps.Equal(got.Values, want.Values) {
		t.Errorf("Catalogue[%q] = %+v, want %+v", want.ID, *got, want)
	}
}

// VTT-997
func TestARulesetKeepsEveryCatalogueEntryByItsID(t *testing.T) {
	rs, err := rules.Load(catalogueRuleset(t, map[string]entryDoc{
		"b-plain.json": plainEntry(),
		"a-crate.json": crateEntry(),
		"c-bare.json":  {"id": "bare-entry", "name": "Bare Entry", "description": "An entry carrying no value.", "form": "qa-form"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Catalogue) != 3 {
		t.Errorf("the catalogue holds %v, want three entries", slices.Sorted(maps.Keys(rs.Catalogue)))
	}
	expectEntry(t, rs, rules.Entry{ID: "qa-entry", Name: "Plain Entry", Description: "A plain entry.", Form: "qa-form",
		Values: map[string]int{"heft": 3, "grit": -2}})
	expectEntry(t, rs, rules.Entry{ID: "crate-entry", Name: "Crate Entry", Description: "An entry of the crate form.", Form: "crate",
		Values: map[string]int{"heft": 7}})
	expectEntry(t, rs, rules.Entry{ID: "bare-entry", Name: "Bare Entry", Description: "An entry carrying no value.", Form: "qa-form"})
}

// VTT-997
func TestARulesetWithNoCatalogueLoadsWithNone(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(t *testing.T, dir string)
	}{
		{"no directory", dropCatalogue},
		{"an empty directory", func(t *testing.T, dir string) {
			t.Helper()
			writeCatalogue(t, dir, nil)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			copyDir(t, "testdata/valid", dir)
			c.edit(t, dir)
			rs, err := rules.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(rs.Catalogue) != 0 {
				t.Errorf("Catalogue = %v, want none", rs.Catalogue)
			}
		})
	}
}

// VTT-998
func TestAnEntryIsRefusedWithoutItsIDNameDescriptionOrForm(t *testing.T) {
	for _, field := range []string{"id", "name", "description", "form"} {
		for _, c := range []struct {
			name string
			edit func(e entryDoc)
		}{
			{"absent", func(e entryDoc) { delete(e, field) }},
			{"empty", func(e entryDoc) { e[field] = "" }},
		} {
			t.Run(field+" "+c.name, func(t *testing.T) {
				e := plainEntry()
				c.edit(e)
				expectEntryRefusal(t, catalogueRuleset(t, withEntry(e)), "b-plain.json", field, "must not be empty")
			})
		}
	}
}

// VTT-998
func TestAnEntryIDAnEarlierFileDeclaresIsRefused(t *testing.T) {
	first := crateEntry()
	first["id"] = "qa-entry"
	dir := catalogueRuleset(t, map[string]entryDoc{"a-crate.json": first, "b-plain.json": plainEntry()})
	expectEntryRefusal(t, dir, "b-plain.json", "id",
		fmt.Sprintf("duplicate entry id %q, declared by %s", "qa-entry", filepath.Join(dir, "catalogue", "a-crate.json")))
}

// VTT-998
func TestAnEntryIDPastTheIDBoundIsRefused(t *testing.T) {
	for _, id := range []string{strings.Repeat("k", 129), strings.Repeat("é", 64) + "k"} {
		t.Run(fmt.Sprint(len([]rune(id))), func(t *testing.T) {
			if len(id) != 129 {
				t.Fatalf("fixture id is %d bytes, want 129", len(id))
			}
			e := plainEntry()
			e["id"] = id
			expectEntryRefusal(t, catalogueRuleset(t, withEntry(e)), "b-plain.json", "id", "must be at most 128 bytes, got 129")
		})
	}
}

// VTT-998
func TestAnEntryIDAtTheIDBoundLoads(t *testing.T) {
	for _, id := range []string{strings.Repeat("k", 128), strings.Repeat("é", 64)} {
		t.Run(fmt.Sprint(len([]rune(id))), func(t *testing.T) {
			if len(id) != 128 {
				t.Fatalf("fixture id is %d bytes, want 128", len(id))
			}
			e := plainEntry()
			e["id"] = id
			rs, err := rules.Load(catalogueRuleset(t, withEntry(e)))
			if err != nil {
				t.Fatalf("an entry id of 128 bytes was refused: %v", err)
			}
			expectEntry(t, rs, rules.Entry{ID: id, Name: "Plain Entry", Description: "A plain entry.", Form: "qa-form",
				Values: map[string]int{"heft": 3, "grit": -2}})
		})
	}
}

// VTT-998
func TestAnEntryNamingAFormNoFormDeclaresIsRefused(t *testing.T) {
	t.Run("another form declared", func(t *testing.T) {
		e := plainEntry()
		e["form"] = "nowhere"
		expectEntryRefusal(t, catalogueRuleset(t, withEntry(e)), "b-plain.json", "form", `names "nowhere", which no form declares`)
	})
	t.Run("no form declared", func(t *testing.T) {
		dir := formsManifest(t, func(m map[string]any) { delete(m, "forms") })
		writeCatalogue(t, dir, map[string]entryDoc{"b-plain.json": plainEntry()})
		expectEntryRefusal(t, dir, "b-plain.json", "form", `names "qa-form", which no form declares`)
	})
}

// VTT-998
func TestAFieldAnEntryDoesNotDefineIsRefused(t *testing.T) {
	for _, field := range []string{"colour", "key", "properties"} {
		t.Run(field, func(t *testing.T) {
			e := plainEntry()
			e[field] = "crate"
			dir := catalogueRuleset(t, withEntry(e))
			_, err := rules.Load(dir)
			want := fmt.Sprintf("rules: %s: json: unknown field %q", filepath.Join(dir, "catalogue", "b-plain.json"), field)
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-998
func TestACaseVariantOfAnEntryFieldIsReadAsIt(t *testing.T) {
	rs, err := rules.Load(catalogueRuleset(t, withEntry(entryDoc{
		"ID": "qa-entry", "NAME": "Plain Entry", "Description": "A plain entry.", "Form": "qa-form", "VALUES": entryDoc{"heft": 3},
	})))
	if err != nil {
		t.Fatal(err)
	}
	expectEntry(t, rs, rules.Entry{ID: "qa-entry", Name: "Plain Entry", Description: "A plain entry.", Form: "qa-form",
		Values: map[string]int{"heft": 3}})
}

// VTT-999
func TestAnEntryValueItsFormDoesNotListIsRefused(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		values           entryDoc
	}{
		{"listed by another form", "values.grit", `form "crate" does not list "grit"`, entryDoc{"heft": 1, "grit": 2}},
		{"declared by no property", "values.nowhere", `form "crate" does not list "nowhere"`, entryDoc{"nowhere": 1}},
		{"a resource", "values.pool_a", `form "crate" does not list "pool_a"`, entryDoc{"heft": 1, "pool_a": 1}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := crateEntry()
			e["values"] = c.values
			dir := catalogueRuleset(t, map[string]entryDoc{"a-plain.json": plainEntry(), "b-crate.json": e})
			expectEntryRefusal(t, dir, "b-crate.json", c.field, c.msg)
		})
	}
}

// VTT-999
func TestAnEntryHoldingSeveralUnlistedValuesIsRefusedOnTheFirstByKey(t *testing.T) {
	e := crateEntry()
	e["values"] = entryDoc{"zeta": 1, "alpha": 2, "mid": 3}
	dir := catalogueRuleset(t, map[string]entryDoc{"b-crate.json": e})
	for range 20 {
		expectEntryRefusal(t, dir, "b-crate.json", "values.alpha", `form "crate" does not list "alpha"`)
	}
}

// VTT-999
func TestAnEntryValueThatIsNotAnInt32IsRefused(t *testing.T) {
	notInt := func(got string) string { return "must be an integer from -2147483648 to 2147483647, got " + got }
	for _, c := range []struct {
		name, msg string
		value     any
	}{
		{"null", "must not be null", nil},
		{"a fraction", notInt("1.5"), json.Number("1.5")},
		{"one past the top", notInt("2147483648"), json.Number("2147483648")},
		{"one past the bottom", notInt("-2147483649"), json.Number("-2147483649")},
		{"a string", notInt(`"5"`), "5"},
		{"a boolean", notInt("true"), true},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := plainEntry()
			e["values"] = entryDoc{"grit": 1, "heft": c.value}
			expectEntryRefusal(t, catalogueRuleset(t, withEntry(e)), "b-plain.json", "values.heft", c.msg)
		})
	}
}

// VTT-999
func TestAnEntryValueAtEachEndOfAnInt32Loads(t *testing.T) {
	e := plainEntry()
	e["values"] = entryDoc{"heft": json.Number("2147483647"), "grit": json.Number("-2147483648")}
	rs, err := rules.Load(catalogueRuleset(t, withEntry(e)))
	if err != nil {
		t.Fatal(err)
	}
	expectEntry(t, rs, rules.Entry{ID: "qa-entry", Name: "Plain Entry", Description: "A plain entry.", Form: "qa-form",
		Values: map[string]int{"heft": 2147483647, "grit": -2147483648}})
}

// VTT-1000
func TestAnEntryCarryingNoValueForAListedPropertyHoldsNone(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(e entryDoc)
		want map[string]int
	}{
		{"one of two", func(e entryDoc) { e["values"] = entryDoc{"heft": 3} }, map[string]int{"heft": 3}},
		{"values absent", func(e entryDoc) { delete(e, "values") }, nil},
		{"values empty", func(e entryDoc) { e["values"] = entryDoc{} }, nil},
		{"values null", func(e entryDoc) { e["values"] = nil }, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := plainEntry()
			c.edit(e)
			rs, err := rules.Load(catalogueRuleset(t, withEntry(e)))
			if err != nil {
				t.Fatal(err)
			}
			expectEntry(t, rs, rules.Entry{ID: "qa-entry", Name: "Plain Entry", Description: "A plain entry.", Form: "qa-form", Values: c.want})
			if v, held := rs.Catalogue["qa-entry"].Values["grit"]; held {
				t.Errorf("the entry holds %d for grit, which it does not carry", v)
			}
		})
	}
}

// VTT-1001
func TestAnIDTwoOfAnEntryAnAbilityAndAConditionDeclareIsRefused(t *testing.T) {
	for _, c := range []struct {
		name, entryID, id, refused, other string
		condition                         entryDoc
	}{
		{"an entry and an ability", "strike", "strike", "catalogue/b-plain.json", "abilities/strike.json", nil},
		{"an entry and a condition", "guarded", "guarded", "conditions/guarded.json", "catalogue/b-plain.json", nil},
		{"an ability and a condition", "qa-entry", "strike", "conditions/qa-mark.json", "abilities/strike.json",
			entryDoc{"id": "strike", "name": "Marked", "description": "A mark."}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := plainEntry()
			e["id"] = c.entryID
			dir := catalogueRuleset(t, withEntry(e))
			if c.condition != nil {
				writeJSONFile(t, filepath.Join(dir, "conditions", "qa-mark.json"), c.condition)
			}
			_, err := rules.Load(dir)
			want := fmt.Sprintf("rules: %s: field %q: %q is declared by %s too; an entry, an ability and a condition never share an id",
				filepath.Join(dir, c.refused), "id", c.id, filepath.Join(dir, c.other))
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-1008
func TestTheSchemaDescribesEveryEntryField(t *testing.T) {
	schema := readSchema(t, "entry.schema.json")
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Errorf("an entry is %v with additionalProperties %v, want an object admitting no other field", schema["type"], schema["additionalProperties"])
	}
	fields := asMap(schema["properties"])
	want := []string{"id", "name", "description", "form", "extends", "values"}
	if len(fields) != len(want) {
		t.Errorf("an entry holds %d fields, the format defines %v", len(fields), want)
	}
	for _, k := range want[:5] {
		f := asMap(fields[k])
		if f["type"] != "string" || f["minLength"] != float64(1) {
			t.Errorf("entry %s = %v, want a string of minLength 1", k, f)
		}
	}
	idDesc, _ := asMap(fields["id"])["description"].(string)
	if !strings.Contains(idDesc, "At most 128 bytes") {
		t.Errorf("entry id description = %q, want it to state %q", idDesc, "At most 128 bytes")
	}
	values := asMap(fields["values"])
	value := asMap(values["additionalProperties"])
	if values["type"] != "object" || value["type"] != "integer" ||
		value["minimum"] != float64(-2147483648) || value["maximum"] != float64(2147483647) {
		t.Errorf("entry values = %v, want an object of integers within a 32-bit signed integer", values)
	}
	var req []string
	for _, r := range asList(schema["required"]) {
		req = append(req, r.(string))
	}
	slices.Sort(req)
	if !slices.Equal(req, []string{"description", "form", "id", "name"}) {
		t.Errorf("an entry requires %v, want id, name, description and form", req)
	}

	paths, err := filepath.Glob("testdata/valid/catalogue/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("testdata/valid/catalogue holds no entry to describe: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range paths {
		for k := range readJSONFile(t, p) {
			seen[k] = true
			if _, ok := fields[k]; !ok {
				t.Errorf("%s uses entry field %q, which entry.schema.json does not describe", p, k)
			}
		}
	}
	for _, k := range want {
		if !seen[k] {
			t.Errorf("testdata/valid/catalogue never uses %s, so this test cannot hold the schema to it", k)
		}
	}
}
