package rules_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type placeDoc = map[string]any

func mikePlace() placeDoc {
	return placeDoc{"key": "mike", "name": "Mike", "description": "An invented place holding copies.",
		"forms": []any{"zulu", "yankee"}, "copies": true, "states": []any{"lima", "oscar"}}
}

func kiloPlace() placeDoc {
	return placeDoc{"key": "kilo", "name": "Kilo", "description": "An invented place holding an entry once.",
		"forms": []any{"yankee"}}
}

func mikeAsLoaded() rules.Place {
	return rules.Place{Key: "mike", Name: "Mike", Description: "An invented place holding copies.",
		Forms: []string{"zulu", "yankee"}, Copies: true, States: []string{"lima", "oscar"}}
}

func kiloAsLoaded() rules.Place {
	return rules.Place{Key: "kilo", Name: "Kilo", Description: "An invented place holding an entry once.",
		Forms: []string{"yankee"}}
}

// placesManifest copies testdata/valid with forms zulu and yankee beside its
// own, places mike and kilo, and no sheet, then applies edit to the manifest.
func placesManifest(t *testing.T, edit func(m map[string]any)) string {
	t.Helper()
	dir := t.TempDir()
	copyDir(t, "testdata/valid", dir)
	path := filepath.Join(dir, "ruleset.json")
	m := readJSONFile(t, path)
	m["forms"] = append(asList(m["forms"]),
		formDoc{"key": "zulu", "name": "Zulu", "description": "An invented form."},
		formDoc{"key": "yankee", "name": "Yankee", "description": "Another invented form."})
	m["places"] = []any{mikePlace(), kiloPlace()}
	delete(m, "sheet")
	edit(m)
	writeJSONFile(t, path, m)
	return dir
}

// dropPlaces removes a manifest's places and every sheet section showing one.
func dropPlaces(m map[string]any) {
	delete(m, "places")
	for _, p := range asList(m["sheet"]) {
		kept := []any{}
		for _, s := range asList(asMap(p)["sections"]) {
			if _, shows := asMap(s)["places"]; !shows {
				kept = append(kept, s)
			}
		}
		asMap(p)["sections"] = kept
	}
}

func placesRuleset(t *testing.T, places ...any) string {
	t.Helper()
	return placesManifest(t, func(m map[string]any) { m["places"] = places })
}

// placesSheet is baseSheet with a section "Carried" on its second page.
func placesSheet(section sheetDoc) []any {
	s := baseSheet()
	addSheetSection(s, 1, section)
	return s
}

func placesSheetRuleset(t *testing.T, sheet []any) string {
	t.Helper()
	return placesManifest(t, func(m map[string]any) { m["sheet"] = sheet })
}

func carried(places ...any) sheetDoc {
	return sheetDoc{"name": "Carried", "places": places}
}

func expectLoadedPlaces(t *testing.T, dir string, want []rules.Place) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rs.Places, want) {
		t.Fatalf("Places = %+v, want %+v", rs.Places, want)
	}
	return rs
}

// VTT-1015
func TestARulesetKeepsItsPlacesInDeclaredOrder(t *testing.T) {
	rs := expectLoadedPlaces(t, placesRuleset(t, mikePlace(), kiloPlace()), []rules.Place{mikeAsLoaded(), kiloAsLoaded()})
	if p := rs.Place("kilo"); p == nil || !reflect.DeepEqual(*p, kiloAsLoaded()) {
		t.Errorf("Place(kilo) = %+v, want %+v", p, kiloAsLoaded())
	}
	if p := rs.Place("mike"); p == nil || !reflect.DeepEqual(*p, mikeAsLoaded()) {
		t.Errorf("Place(mike) = %+v, want %+v", p, mikeAsLoaded())
	}
	if p := rs.Place("nowhere"); p != nil {
		t.Errorf("Place(nowhere) = %+v, want nil", p)
	}
}

// VTT-1015
func TestAPlaceKeepsItsFormsAndStatesInTheOrderWritten(t *testing.T) {
	m := mikePlace()
	m["forms"] = []any{"yankee", "zulu"}
	m["states"] = []any{"oscar", "lima"}
	want := mikeAsLoaded()
	want.Forms = []string{"yankee", "zulu"}
	want.States = []string{"oscar", "lima"}
	expectLoadedPlaces(t, placesRuleset(t, kiloPlace(), m), []rules.Place{kiloAsLoaded(), want})
}

// VTT-1015
func TestAPlaceWritingNeitherCopiesNorStatesLoadsWithNoCopiesAndNoState(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(p placeDoc)
	}{
		{"both absent", func(p placeDoc) {}},
		{"copies false", func(p placeDoc) { p["copies"] = false }},
		{"copies null", func(p placeDoc) { p["copies"] = nil }},
		{"states null", func(p placeDoc) { p["states"] = nil }},
		{"states empty", func(p placeDoc) { p["states"] = []any{} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := kiloPlace()
			c.edit(p)
			expectLoadedPlaces(t, placesRuleset(t, p), []rules.Place{kiloAsLoaded()})
		})
	}
}

// VTT-1015
func TestTheFixtureKeepsItsPlaces(t *testing.T) {
	expectLoadedPlaces(t, "testdata/valid", []rules.Place{
		{Key: "shelf", Name: "Shelf", Description: "The fixture's shelf, which holds a crate more than once.",
			Forms: []string{"crate"}, Copies: true, States: []string{"stacked", "loose"}},
		{Key: "hook", Name: "Hook", Description: "The fixture's hook, which holds a crate once.",
			Forms: []string{"crate"}},
	})
}

// VTT-1015 VTT-1018
func TestARulesetDeclaringNoPlacesLoadsWithNone(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m map[string]any)
	}{
		{"absent", func(m map[string]any) { delete(m, "places") }},
		{"null", func(m map[string]any) { m["places"] = nil }},
		{"empty", func(m map[string]any) { m["places"] = []any{} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs := expectLoadedPlaces(t, placesManifest(t, c.edit), nil)
			if p := rs.Place("mike"); p != nil {
				t.Errorf("Place(mike) = %+v, want nil", p)
			}
		})
	}
}

// VTT-1016
func TestAPlaceIsRefusedWithoutItsKeyNameOrDescription(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		edit             func(p placeDoc)
	}{
		{"key absent", "places[1].key", "must not be empty", func(p placeDoc) { delete(p, "key") }},
		{"key null", "places[1].key", "must not be empty", func(p placeDoc) { p["key"] = nil }},
		{"key empty", "places[1].key", "must not be empty", func(p placeDoc) { p["key"] = "" }},
		{"name absent", "places[1].name", `place "kilo": must not be empty`, func(p placeDoc) { delete(p, "name") }},
		{"name null", "places[1].name", `place "kilo": must not be empty`, func(p placeDoc) { p["name"] = nil }},
		{"name empty", "places[1].name", `place "kilo": must not be empty`, func(p placeDoc) { p["name"] = "" }},
		{"description absent", "places[1].description", `place "kilo": must not be empty`, func(p placeDoc) { delete(p, "description") }},
		{"description null", "places[1].description", `place "kilo": must not be empty`, func(p placeDoc) { p["description"] = nil }},
		{"description empty", "places[1].description", `place "kilo": must not be empty`, func(p placeDoc) { p["description"] = "" }},
		{"key before name", "places[1].key", "must not be empty", func(p placeDoc) { p["key"], p["name"] = "", "" }},
		{"name before description", "places[1].name", `place "kilo": must not be empty`, func(p placeDoc) { p["name"], p["description"] = "", "" }},
		{"description before forms", "places[1].description", `place "kilo": must not be empty`, func(p placeDoc) { p["description"], p["forms"] = "", []any{} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := kiloPlace()
			c.edit(p)
			expectFormRefusal(t, placesRuleset(t, mikePlace(), p), c.field, c.msg)
		})
	}
}

// VTT-1016
func TestAPlaceKeyAnEarlierPlaceDeclaresIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(p placeDoc)
	}{
		{"alone", func(p placeDoc) {}},
		{"before its forms", func(p placeDoc) { p["forms"] = []any{} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := kiloPlace()
			p["key"] = "mike"
			c.edit(p)
			expectFormRefusal(t, placesRuleset(t, mikePlace(), p), "places[1].key", `duplicate place key "mike"`)
		})
	}
}

// VTT-1016
func TestAPlaceKeyPastTheIDBoundIsRefused(t *testing.T) {
	for _, key := range []string{strings.Repeat("k", 129), strings.Repeat("é", 64) + "k"} {
		t.Run(fmt.Sprint(len([]rune(key))), func(t *testing.T) {
			if len(key) != 129 {
				t.Fatalf("fixture key is %d bytes, want 129", len(key))
			}
			p := kiloPlace()
			p["key"] = key
			expectFormRefusal(t, placesRuleset(t, mikePlace(), p), "places[1].key", "must be at most 128 bytes, got 129")
		})
	}
}

// VTT-1016
func TestAPlaceKeyAtTheIDBoundLoads(t *testing.T) {
	for _, key := range []string{strings.Repeat("k", 128), strings.Repeat("é", 64)} {
		t.Run(fmt.Sprint(len([]rune(key))), func(t *testing.T) {
			if len(key) != 128 {
				t.Fatalf("fixture key is %d bytes, want 128", len(key))
			}
			p := kiloPlace()
			p["key"] = key
			want := kiloAsLoaded()
			want.Key = key
			expectLoadedPlaces(t, placesRuleset(t, mikePlace(), p), []rules.Place{mikeAsLoaded(), want})
		})
	}
}

// VTT-1016 VTT-1017
func TestTheBoundsOnAPlaceAreReadBeforeItsOtherFaults(t *testing.T) {
	long := strings.Repeat("k", 129)
	p := kiloPlace()
	p["key"], p["name"] = long, ""
	expectFormRefusal(t, placesRuleset(t, mikePlace(), p), "places[1].key", "must be at most 128 bytes, got 129")

	p = mikePlace()
	p["states"], p["forms"] = []any{"lima", long}, []any{"nowhere"}
	expectFormRefusal(t, placesRuleset(t, kiloPlace(), p), "places[1].states[1]", "must be at most 128 bytes, got 129")
}

// VTT-1017
func TestAPlaceNamingNoFormIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(p placeDoc)
	}{
		{"forms absent", func(p placeDoc) { delete(p, "forms") }},
		{"forms null", func(p placeDoc) { p["forms"] = nil }},
		{"forms empty", func(p placeDoc) { p["forms"] = []any{} }},
		{"before its states", func(p placeDoc) { p["forms"], p["states"] = []any{}, []any{""} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := kiloPlace()
			c.edit(p)
			expectFormRefusal(t, placesRuleset(t, mikePlace(), p), "places[1].forms", `place "kilo": must name a form`)
		})
	}
}

// VTT-1017
func TestAPlaceNamingAFormItMayNotHoldIsRefused(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		forms            []any
	}{
		{"undeclared", "places[1].forms[1]", `names "nowhere", which no form declares`, []any{"yankee", "nowhere"}},
		{"empty", "places[1].forms[0]", `names "", which no form declares`, []any{"", "yankee"}},
		{"a place", "places[1].forms[0]", `names "mike", which no form declares`, []any{"mike"}},
		{"listed twice", "places[1].forms[2]", `names "yankee", which the place lists already`, []any{"yankee", "zulu", "yankee"}},
		{"undeclared before listed twice", "places[1].forms[1]", `names "nowhere", which no form declares`, []any{"yankee", "nowhere", "yankee"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := kiloPlace()
			p["forms"], p["states"] = c.forms, []any{""}
			expectFormRefusal(t, placesRuleset(t, mikePlace(), p), c.field, c.msg)
		})
	}
}

// VTT-1017
func TestAPlaceStateThatIsEmptyOrRepeatedIsRefused(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		states           []any
	}{
		{"empty", "places[1].states[1]", "must not be empty", []any{"lima", ""}},
		{"listed twice", "places[1].states[2]", `names "lima", which the place lists already`, []any{"lima", "oscar", "lima"}},
		{"empty before listed twice", "places[1].states[1]", "must not be empty", []any{"lima", "", "lima"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := kiloPlace()
			p["states"] = c.states
			expectFormRefusal(t, placesRuleset(t, mikePlace(), p), c.field, c.msg)
		})
	}
}

// VTT-1017
func TestAPlaceStatePastTheIDBoundIsRefused(t *testing.T) {
	for _, state := range []string{strings.Repeat("s", 129), strings.Repeat("é", 64) + "s"} {
		t.Run(fmt.Sprint(len([]rune(state))), func(t *testing.T) {
			if len(state) != 129 {
				t.Fatalf("fixture state is %d bytes, want 129", len(state))
			}
			p := mikePlace()
			p["states"] = []any{"lima", state}
			expectFormRefusal(t, placesRuleset(t, kiloPlace(), p), "places[1].states[1]", "must be at most 128 bytes, got 129")
		})
	}
}

// VTT-1017
func TestAPlaceStateAtTheIDBoundLoads(t *testing.T) {
	for _, state := range []string{strings.Repeat("s", 128), strings.Repeat("é", 64)} {
		t.Run(fmt.Sprint(len([]rune(state))), func(t *testing.T) {
			if len(state) != 128 {
				t.Fatalf("fixture state is %d bytes, want 128", len(state))
			}
			p := mikePlace()
			p["states"] = []any{"lima", state}
			want := mikeAsLoaded()
			want.States = []string{"lima", state}
			expectLoadedPlaces(t, placesRuleset(t, kiloPlace(), p), []rules.Place{kiloAsLoaded(), want})
		})
	}
}

// VTT-1018
func TestAFieldAPlaceDoesNotDefineIsRefused(t *testing.T) {
	for _, field := range []string{"capacity", "admits"} {
		t.Run(field, func(t *testing.T) {
			p := kiloPlace()
			p[field] = "yankee"
			dir := placesRuleset(t, mikePlace(), p)
			_, err := rules.Load(dir)
			if err == nil || !strings.Contains(err.Error(), filepath.Join(dir, "ruleset.json")) ||
				!strings.Contains(err.Error(), fmt.Sprintf("unknown field %q", field)) {
				t.Fatalf("Load = %v, want a refusal naming ruleset.json and the field %s", err, field)
			}
		})
	}
}

// VTT-1018
func TestACaseVariantOfPlacesIsReadAsPlaces(t *testing.T) {
	dir := placesManifest(t, func(m map[string]any) {
		delete(m, "places")
		m["Places"] = []any{placeDoc{"KEY": "mike", "Name": "Mike", "DESCRIPTION": "An invented place holding copies.",
			"Forms": []any{"zulu", "yankee"}, "COPIES": true, "States": []any{"lima", "oscar"}}}
		m["sheet"] = placesSheet(sheetDoc{"name": "Carried", "PLACES": []any{"mike"}})
	})
	rs := expectLoadedPlaces(t, dir, []rules.Place{mikeAsLoaded()})
	if got := rs.Sheet[1].Sections[1].Places; !slices.Equal(got, []string{"mike"}) {
		t.Errorf("the section's places = %v, want [mike]", got)
	}
}

// VTT-824 VTT-1019
func TestASectionShowingPlacesLoadsThemInTheOrderWritten(t *testing.T) {
	rs, err := rules.Load(placesSheetRuleset(t, placesSheet(carried("kilo", "mike"))))
	if err != nil {
		t.Fatal(err)
	}
	got := rs.Sheet[1].Sections[1]
	if want := (rules.SheetSection{Name: "Carried", Places: []string{"kilo", "mike"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("section = %+v, want %+v", got, want)
	}

	s := placesSheet(carried("mike"))
	addSheetSection(s, 0, carried("kilo"))
	sheetSection(s, 0, 3)["name"] = "Hung"
	rs, err = rules.Load(placesSheetRuleset(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if a, b := rs.Sheet[0].Sections[3].Places, rs.Sheet[1].Sections[1].Places; !slices.Equal(a, []string{"kilo"}) || !slices.Equal(b, []string{"mike"}) {
		t.Errorf("sections' places = %v and %v, want [kilo] and [mike]", a, b)
	}
}

// VTT-1019
func TestASectionShowingPlacesAndAnotherKindIsRefused(t *testing.T) {
	const want = `section "Carried" must show exactly one of values, conditions, abilities and places`
	for _, c := range []struct {
		name  string
		other func(s sheetDoc)
	}{
		{"values", func(s sheetDoc) { s["values"] = []any{"guard"} }},
		{"conditions", func(s sheetDoc) { s["conditions"] = true }},
		{"abilities", func(s sheetDoc) { s["abilities"] = []any{"at_will"} }},
		{"no kind at all", func(s sheetDoc) { delete(s, "places") }},
		{"places null", func(s sheetDoc) { s["places"] = nil }},
	} {
		t.Run(c.name, func(t *testing.T) {
			section := carried("mike", "kilo")
			c.other(section)
			expectFormRefusal(t, placesSheetRuleset(t, placesSheet(section)), "sheet[1].sections[1]", want)
		})
	}
}

// VTT-1019
func TestASectionShowingAnEmptyListOfPlacesIsRefused(t *testing.T) {
	empty := sheetDoc{"name": "Carried", "places": []any{}}
	expectFormRefusal(t, placesSheetRuleset(t, placesSheet(empty)), "sheet[1].sections[1].places", "must not be empty")
}

// VTT-1060
func TestASectionNamingAPlaceTheRulesetLacksIsRefused(t *testing.T) {
	for _, c := range []struct {
		name, field, msg string
		places           []any
	}{
		{"undeclared", "sheet[1].sections[1].places[1]", `names "nowhere", which no place declares`, []any{"mike", "nowhere", "kilo"}},
		{"empty", "sheet[1].sections[1].places[0]", `names "", which no place declares`, []any{"", "mike", "kilo"}},
		{"a form", "sheet[1].sections[1].places[2]", `names "zulu", which no place declares`, []any{"mike", "kilo", "zulu"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			expectFormRefusal(t, placesSheetRuleset(t, placesSheet(carried(c.places...))), c.field, c.msg)
		})
	}
}

// VTT-1060
func TestASheetShowingAPlaceTwiceIsRefused(t *testing.T) {
	t.Run("in one list", func(t *testing.T) {
		expectFormRefusal(t, placesSheetRuleset(t, placesSheet(carried("mike", "kilo", "mike"))),
			"sheet[1].sections[1].places[2]", `names "mike", which the sheet shows already`)
	})
	t.Run("in two sections", func(t *testing.T) {
		s := placesSheet(carried("mike", "kilo"))
		addSheetSection(s, 1, sheetDoc{"name": "Again", "places": []any{"kilo"}})
		expectFormRefusal(t, placesSheetRuleset(t, s), "sheet[1].sections[2].places[0]", `names "kilo", which the sheet shows already`)
	})
	t.Run("on two pages", func(t *testing.T) {
		s := placesSheet(carried("mike", "kilo"))
		s = append(s, sheetDoc{"name": "Third", "sections": []any{sheetDoc{"name": "Again", "places": []any{"mike"}}}})
		expectFormRefusal(t, placesSheetRuleset(t, s), "sheet[2].sections[0].places[0]", `names "mike", which the sheet shows already`)
	})
	t.Run("undeclared before twice", func(t *testing.T) {
		expectFormRefusal(t, placesSheetRuleset(t, placesSheet(carried("mike", "kilo", "nowhere", "mike"))),
			"sheet[1].sections[1].places[2]", `names "nowhere", which no place declares`)
	})
}

// VTT-1020
func TestAPlaceNoSectionShowsIsRefused(t *testing.T) {
	expectFormRefusal(t, placesSheetRuleset(t, placesSheet(carried("mike"))), "sheet", `no section shows place "kilo"`)
	expectFormRefusal(t, placesSheetRuleset(t, baseSheet()), "sheet", `no section shows place "mike"`)
}

// VTT-1020
func TestAnUnshownAbilityIsNamedBeforeAnUnshownPlace(t *testing.T) {
	s := baseSheet()
	sheetSection(s, 0, 2)["abilities"] = []any{"at_will"}
	expectFormRefusal(t, placesSheetRuleset(t, s), "sheet", `no section shows ability "guard-stance", whose usage is spends "pool_a"`)
}

// VTT-1020
func TestPlacesLoadWithoutASheetToShowThem(t *testing.T) {
	expectLoadedPlaces(t, placesRuleset(t, mikePlace(), kiloPlace()), []rules.Place{mikeAsLoaded(), kiloAsLoaded()})
	expectLoadedPlaces(t, placesSheetRuleset(t, placesSheet(carried("kilo", "mike"))), []rules.Place{mikeAsLoaded(), kiloAsLoaded()})
}

// VTT-1021
func TestTheSchemaDescribesEveryPlaceField(t *testing.T) {
	places, required := schemaField(t, "ruleset.schema.json", "places")
	if required {
		t.Error("ruleset.schema.json lists places as required")
	}
	if places["type"] != "array" {
		t.Errorf("places type = %v, want array", places["type"])
	}
	item := asMap(places["items"])
	if item["type"] != "object" || item["additionalProperties"] != false {
		t.Errorf("a place is %v with additionalProperties %v, want an object admitting no other field", item["type"], item["additionalProperties"])
	}
	fields := asMap(item["properties"])
	want := []string{"key", "name", "description", "forms", "copies", "states"}
	if len(fields) != len(want) {
		t.Errorf("a place holds %d fields, the format defines %v", len(fields), want)
	}
	for _, k := range want[:3] {
		if f := asMap(fields[k]); f["type"] != "string" || f["minLength"] != float64(1) {
			t.Errorf("place %s = %v, want a string of minLength 1", k, f)
		}
	}
	for _, k := range []string{"forms", "states"} {
		list := asMap(fields[k])
		if list["type"] != "array" || list["uniqueItems"] != true || asMap(list["items"])["type"] != "string" ||
			asMap(list["items"])["minLength"] != float64(1) {
			t.Errorf("place %s = %v, want an array of distinct non-empty strings", k, list)
		}
	}
	if asMap(fields["forms"])["minItems"] != float64(1) {
		t.Errorf("place forms = %v, want at least one", fields["forms"])
	}
	if _, ok := asMap(fields["states"])["minItems"]; ok {
		t.Errorf("place states = %v, want a place to name none", fields["states"])
	}
	if asMap(fields["copies"])["type"] != "boolean" {
		t.Errorf("place copies = %v, want a boolean", fields["copies"])
	}
	var req []string
	for _, r := range asList(item["required"]) {
		req = append(req, r.(string))
	}
	slices.Sort(req)
	if !slices.Equal(req, []string{"description", "forms", "key", "name"}) {
		t.Errorf("a place requires %v, want key, name, description and forms", req)
	}

	sheet, _ := schemaField(t, "ruleset.schema.json", "sheet")
	sec := asMap(asMap(asMap(asMap(sheet["items"])["properties"])["sections"])["items"])
	shown := asMap(asMap(sec["properties"])["places"])
	if shown["type"] != "array" || shown["minItems"] != float64(1) || asMap(shown["items"])["type"] != "string" {
		t.Errorf("a section's places = %v, want a non-empty array of strings", shown)
	}
	kinds := 0
	for _, branch := range asList(sec["oneOf"]) {
		if r := asList(asMap(branch)["required"]); len(r) == 1 && r[0] == "places" {
			kinds++
		}
	}
	if kinds != 1 {
		t.Errorf("section oneOf = %v, want one branch requiring places", sec["oneOf"])
	}

	fixture := asList(readJSONFile(t, "testdata/valid/ruleset.json")["places"])
	seen := map[string]bool{}
	for _, p := range fixture {
		for k := range asMap(p) {
			seen[k] = true
			if _, ok := fields[k]; !ok {
				t.Errorf("testdata/valid/ruleset.json uses place field %q, which ruleset.schema.json does not describe", k)
			}
		}
	}
	for _, k := range want {
		if !seen[k] {
			t.Errorf("testdata/valid/ruleset.json's places never use %s, so this test cannot hold the schema to it", k)
		}
	}
}
