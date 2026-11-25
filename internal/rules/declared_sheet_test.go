package rules_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type sheetDoc = map[string]any

func baseSheet() []any {
	return []any{
		sheetDoc{"name": "Front", "sections": []any{
			sheetDoc{"name": "Pool", "values": []any{"pool_a"}},
			sheetDoc{"name": "Marks", "conditions": true},
			sheetDoc{"name": "Moves", "abilities": []any{"at_will", sheetDoc{"spends": "pool_a"}}},
		}},
		sheetDoc{"name": "Back", "sections": []any{
			sheetDoc{"name": "Numbers", "values": []any{"guard", "brawn", "grit"}},
		}},
	}
}

func sheetPage(sheet []any, i int) sheetDoc { return sheet[i].(sheetDoc) }

func sheetSection(sheet []any, i, j int) sheetDoc {
	return sheetPage(sheet, i)["sections"].([]any)[j].(sheetDoc)
}

func addSheetSection(sheet []any, i int, s sheetDoc) {
	p := sheetPage(sheet, i)
	p["sections"] = append(p["sections"].([]any), s)
}

// sheetRuleset writes testdata/valid to a scratch directory with sheet as its
// manifest's sheet, or with none when sheet is nil, and no places.
func sheetRuleset(t *testing.T, sheet []any) string {
	t.Helper()
	dir := t.TempDir()
	copyDir(t, "testdata/valid", dir)
	path := filepath.Join(dir, "ruleset.json")
	m := readJSONFile(t, path)
	dropPlaces(m)
	delete(m, "sheet")
	if sheet != nil {
		m["sheet"] = sheet
	}
	writeJSONFile(t, path, m)
	return dir
}

func expectSheetRefusal(t *testing.T, sheet []any, field string, words ...string) {
	t.Helper()
	dir := sheetRuleset(t, sheet)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatalf("Load accepted the sheet; want a refusal naming field %q", field)
	}
	msg := err.Error()
	if !strings.Contains(msg, filepath.Join(dir, "ruleset.json")) {
		t.Errorf("refusal %q does not name the file", msg)
	}
	if field != "" && !strings.Contains(msg, fmt.Sprintf("field %q", field)) {
		t.Errorf("refusal %q does not name field %q", msg, field)
	}
	for _, w := range words {
		if !strings.Contains(msg, w) {
			t.Errorf("refusal %q does not say %q", msg, w)
		}
	}
}

// sheetAsJSON writes a loaded sheet back in ruleset.json's own shape.
func sheetAsJSON(pages []rules.SheetPage) []any {
	out := []any{}
	for _, p := range pages {
		sections := []any{}
		for _, s := range p.Sections {
			doc := sheetDoc{"name": s.Name}
			switch {
			case s.Values != nil:
				values := []any{}
				for _, v := range s.Values {
					values = append(values, v)
				}
				doc["values"] = values
			case s.Conditions:
				doc["conditions"] = true
			case s.Places != nil:
				places := []any{}
				for _, p := range s.Places {
					places = append(places, p)
				}
				doc["places"] = places
			default:
				usages := []any{}
				for _, u := range s.Abilities {
					if u.AtWill {
						usages = append(usages, "at_will")
					} else {
						usages = append(usages, map[string]any{"spends": u.Resource})
					}
				}
				doc["abilities"] = usages
			}
			sections = append(sections, doc)
		}
		out = append(out, map[string]any{"name": p.Name, "sections": sections})
	}
	return out
}

// VTT-824
func TestADeclaredSheetLoadsInTheOrderDeclared(t *testing.T) {
	rs, err := rules.Load(sheetRuleset(t, baseSheet()))
	if err != nil {
		t.Fatal(err)
	}
	want := []rules.SheetPage{
		{Name: "Front", Sections: []rules.SheetSection{
			{Name: "Pool", Values: []string{"pool_a"}},
			{Name: "Marks", Conditions: true},
			{Name: "Moves", Abilities: []rules.ShownUsage{{AtWill: true}, {Resource: "pool_a"}}},
		}},
		{Name: "Back", Sections: []rules.SheetSection{
			{Name: "Numbers", Values: []string{"guard", "brawn", "grit"}},
		}},
	}
	if !reflect.DeepEqual(rs.Sheet, want) {
		t.Errorf("Sheet = %+v, want %+v", rs.Sheet, want)
	}

	rs, err = rules.Load(sheetRuleset(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if rs.Sheet != nil {
		t.Errorf("a ruleset declaring no sheet loaded Sheet = %+v, want nil", rs.Sheet)
	}
}

// VTT-824
func TestTheShippedRulesetsLoadTheSheetTheyDeclare(t *testing.T) {
	for _, id := range []string{"dnd45e-minimal", "tavern-brawl"} {
		t.Run(id, func(t *testing.T) {
			dir := filepath.Join("..", "..", "rulesets", id)
			rs, err := rules.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			declared, ok := readJSONFile(t, filepath.Join(dir, "ruleset.json"))["sheet"]
			if !ok {
				if rs.Sheet != nil {
					t.Errorf("Sheet = %+v for a ruleset declaring none", rs.Sheet)
				}
				return
			}
			if rs.Sheet == nil {
				t.Fatal("the ruleset declares a sheet and Load returned none")
			}
			if got := sheetAsJSON(rs.Sheet); !reflect.DeepEqual(got, declared) {
				t.Errorf("Sheet = %v, want what ruleset.json declares, %v", got, declared)
			}
		})
	}
}

// VTT-1019
func TestASheetOfTheWrongShapeIsRefused(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(s []any) []any
		field string
	}{
		{"no page", func(s []any) []any { return []any{} }, "sheet"},
		{"an unnamed page", func(s []any) []any { sheetPage(s, 0)["name"] = ""; return s }, "sheet[0].name"},
		{"a page without a name", func(s []any) []any { delete(sheetPage(s, 0), "name"); return s }, "sheet[0].name"},
		{"a page named as another", func(s []any) []any { sheetPage(s, 1)["name"] = "Front"; return s }, "sheet[1].name"},
		{"a page with no section", func(s []any) []any { sheetPage(s, 1)["sections"] = []any{}; return s }, "sheet[1].sections"},
		{"a page without sections", func(s []any) []any { delete(sheetPage(s, 1), "sections"); return s }, "sheet[1].sections"},
		{"an unnamed section", func(s []any) []any { sheetSection(s, 0, 1)["name"] = ""; return s }, "sheet[0].sections[1].name"},
		{"a section showing nothing", func(s []any) []any { delete(sheetSection(s, 0, 0), "values"); return s }, "sheet[0].sections[0]"},
		{"a section showing two kinds", func(s []any) []any { sheetSection(s, 0, 0)["conditions"] = true; return s }, "sheet[0].sections[0]"},
		{"conditions shown as false", func(s []any) []any { sheetSection(s, 0, 1)["conditions"] = false; return s }, "sheet[0].sections[1].conditions"},
		{"an empty values list", func(s []any) []any { sheetSection(s, 1, 0)["values"] = []any{}; return s }, "sheet[1].sections[0].values"},
		{"an empty abilities list", func(s []any) []any { sheetSection(s, 0, 2)["abilities"] = []any{}; return s }, "sheet[0].sections[2].abilities"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectSheetRefusal(t, c.edit(baseSheet()), c.field, "must")
		})
	}
}

// VTT-826
func TestASheetNamingWhatTheRulesetLacksIsRefused(t *testing.T) {
	s := baseSheet()
	sheetSection(s, 1, 0)["values"] = []any{"guard", "nerve"}
	expectSheetRefusal(t, s, "sheet[1].sections[0].values[1]", `"nerve"`)

	for _, key := range []string{"brawn", "nerve", ""} {
		s = baseSheet()
		sheetSection(s, 0, 2)["abilities"] = []any{"at_will", sheetDoc{"spends": key}}
		expectSheetRefusal(t, s, "sheet[0].sections[2].abilities[1]", fmt.Sprintf("%q", key))
	}
}

// VTT-827
func TestASheetNamingAThingTwiceIsRefused(t *testing.T) {
	s := baseSheet()
	sheetSection(s, 1, 0)["values"] = []any{"guard", "brawn", "grit", "pool_a"}
	expectSheetRefusal(t, s, "sheet[1].sections[0].values[3]", `"pool_a"`)

	s = baseSheet()
	addSheetSection(s, 1, sheetDoc{"name": "Again", "abilities": []any{"at_will"}})
	expectSheetRefusal(t, s, "sheet[1].sections[1].abilities[0]")

	s = baseSheet()
	addSheetSection(s, 1, sheetDoc{"name": "Again", "abilities": []any{sheetDoc{"spends": "pool_a"}}})
	expectSheetRefusal(t, s, "sheet[1].sections[1].abilities[0]", `"pool_a"`)

	s = baseSheet()
	addSheetSection(s, 1, sheetDoc{"name": "Again", "conditions": true})
	expectSheetRefusal(t, s, "sheet[1].sections[1].conditions")

	if _, err := rules.Load(sheetRuleset(t, baseSheet())); err != nil {
		t.Errorf("a key shown as a value and in the usage that spends it was refused: %v", err)
	}
}

// VTT-828
func TestAnAbilityWhoseUsageNoSectionShowsIsRefused(t *testing.T) {
	s := baseSheet()
	sheetSection(s, 0, 2)["abilities"] = []any{"at_will"}
	expectSheetRefusal(t, s, "sheet", `"guard-stance"`)

	s = baseSheet()
	sheetSection(s, 0, 2)["abilities"] = []any{sheetDoc{"spends": "pool_a"}}
	expectSheetRefusal(t, s, "sheet", `"stand-down"`)
}

// VTT-1018
func TestAFieldTheSheetDoesNotDefineIsRefused(t *testing.T) {
	cases := []struct {
		name, word string
		edit       func(s []any)
	}{
		{"on a page", `"tabs"`, func(s []any) { sheetPage(s, 0)["tabs"] = 1 }},
		{"on a section", `"columns"`, func(s []any) { sheetSection(s, 0, 0)["columns"] = 2 }},
		{"on a usage", `"cost"`, func(s []any) {
			sheetSection(s, 0, 2)["abilities"] = []any{"at_will", sheetDoc{"spends": "pool_a", "cost": 1}}
		}},
		{"a usage word", `"at will"`, func(s []any) { sheetSection(s, 0, 2)["abilities"] = []any{"at will", sheetDoc{"spends": "pool_a"}} }},
		{"a usage naming no resource", `"spends"`, func(s []any) { sheetSection(s, 0, 2)["abilities"] = []any{"at_will", sheetDoc{}} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := baseSheet()
			c.edit(s)
			expectSheetRefusal(t, s, "", c.word)
		})
	}
}

// VTT-829 VTT-1021
func TestTheSchemaDescribesEverySheetField(t *testing.T) {
	schema := readSchema(t, "ruleset.schema.json")
	node := func(n map[string]any, path ...string) map[string]any {
		for _, k := range path {
			next, ok := n[k].(map[string]any)
			if !ok {
				t.Fatalf("ruleset.schema.json has no %s", strings.Join(path, "."))
			}
			n = next
		}
		return n
	}
	page := node(schema, "properties", "sheet", "items", "properties")
	section := node(page, "sections", "items", "properties")
	var spends map[string]any
	usages, _ := node(section, "abilities", "items")["oneOf"].([]any)
	for _, u := range usages {
		if props, ok := u.(map[string]any)["properties"].(map[string]any); ok {
			spends = props
		}
	}
	if spends == nil {
		t.Fatal("ruleset.schema.json describes no usage object under sheet's abilities")
	}

	sheet, _ := readJSONFile(t, "testdata/valid/ruleset.json")["sheet"].([]any)
	if len(sheet) < 2 {
		t.Fatal("testdata/valid/ruleset.json declares no sheet of more than one page to describe")
	}
	seen := map[string]bool{}
	check := func(where string, doc sheetDoc, described map[string]any) {
		for k := range doc {
			seen[where+"."+k] = true
			if _, ok := described[k]; !ok {
				t.Errorf("testdata/valid/ruleset.json uses %s field %q, which ruleset.schema.json does not describe", where, k)
			}
		}
	}
	for _, p := range sheet {
		check("page", p.(sheetDoc), page)
		for _, s := range p.(sheetDoc)["sections"].([]any) {
			check("section", s.(sheetDoc), section)
			for _, u := range asList(s.(sheetDoc)["abilities"]) {
				if obj, ok := u.(sheetDoc); ok {
					check("usage", obj, spends)
				}
			}
		}
	}
	for _, f := range []string{"page.name", "page.sections", "section.name", "section.values", "section.conditions", "section.abilities", "section.places", "usage.spends"} {
		if !seen[f] {
			t.Errorf("testdata/valid/ruleset.json's sheet never uses %s, so this test cannot hold the schema to it", f)
		}
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}
