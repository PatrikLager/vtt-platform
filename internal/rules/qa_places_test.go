package rules_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaplObj = map[string]any

const qaplTavernDir = "../../rulesets/tavern-brawl"

const qaplBound = 128

var qaplFieldRE = regexp.MustCompile(`: field "([^"]*)":`)

var qaplBoundRE = regexp.MustCompile(`At most (\d+) bytes`)

func qaplField(err error) string {
	if err == nil {
		return ""
	}
	m := qaplFieldRE.FindStringSubmatch(err.Error())
	if m == nil {
		return ""
	}
	return m[1]
}

func qaplForms() []any {
	return []any{
		qaplObj{"key": "zulu", "name": "Zulu", "description": "A zulu at the table."},
		qaplObj{"key": "yankee", "name": "Yankee", "description": "A yankee at the table."},
		qaplObj{"key": "xray", "name": "Xray", "description": "An xray at the table."},
	}
}

func qaplPlaces() []any {
	return []any{
		qaplObj{"key": "mike", "name": "Mike ñ", "description": "Where a mike rests, for the table.",
			"forms": []any{"yankee", "zulu", "xray"}, "copies": true, "states": []any{"oscar", "lima"}},
		qaplObj{"key": "kilo", "name": "Kilo", "description": "Where a kilo rests.",
			"forms": []any{"zulu"}},
		qaplObj{"key": "november", "name": "November", "description": "Where a november rests.",
			"forms": []any{"xray", "yankee"}, "copies": false, "states": []any{}},
	}
}

func qaplSheet() []any {
	return []any{
		qaplObj{"name": "Papa", "sections": []any{
			qaplObj{"name": "Quebec", "values": []any{"brawn"}},
			qaplObj{"name": "Romeo", "abilities": []any{"at_will", qaplObj{"spends": "stamina"}}},
		}},
		qaplObj{"name": "Sierra", "sections": []any{
			qaplObj{"name": "Tango", "conditions": true},
			qaplObj{"name": "Uniform", "places": []any{"november", "mike"}},
			qaplObj{"name": "Victor", "places": []any{"kilo"}},
		}},
	}
}

func qaplTavern(t *testing.T) qaplObj {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(qaplTavernDir, "ruleset.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m qaplObj
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func qaplManifest(t *testing.T, withSheet bool) qaplObj {
	t.Helper()
	m := qaplTavern(t)
	m["forms"] = qaplForms()
	m["places"] = qaplPlaces()
	if withSheet {
		m["sheet"] = qaplSheet()
	}
	return m
}

func qaplPlace(m qaplObj, i int) qaplObj { return m["places"].([]any)[i].(qaplObj) }

func qaplForm(m qaplObj, i int) qaplObj { return m["forms"].([]any)[i].(qaplObj) }

func qaplPage(m qaplObj, i int) qaplObj { return m["sheet"].([]any)[i].(qaplObj) }

func qaplSection(m qaplObj, page, sec int) qaplObj {
	return qaplPage(m, page)["sections"].([]any)[sec].(qaplObj)
}

func qaplAddProperty(m qaplObj, p qaplObj) {
	m["properties"] = append(m["properties"].([]any), p)
}

func qaplCopyDir(t *testing.T, src, dst string) {
	t.Helper()
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
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func qaplLoad(t *testing.T, m qaplObj, files map[string]string) (*rules.Ruleset, string, error) {
	t.Helper()
	dir := t.TempDir()
	qaplCopyDir(t, qaplTavernDir, dir)
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ruleset.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Load(dir)
	return rs, path, err
}

func qaplRefused(t *testing.T, err error, path, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("loaded; want a refusal naming %s and field %q", path, field)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("refusal %q does not name %s", err, path)
	}
	if got := qaplField(err); got != field {
		t.Errorf("refusal names field %q, want %q: %v", got, field, err)
	}
}

func qaplRefusedUnder(t *testing.T, err error, path, prefix string) {
	t.Helper()
	if err == nil {
		t.Fatalf("loaded; want a refusal naming %s under %q", path, prefix)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("refusal %q does not name %s", err, path)
	}
	got := qaplField(err)
	if got != prefix && !strings.HasPrefix(got, prefix+".") && !strings.HasPrefix(got, prefix+"[") {
		t.Errorf("refusal names field %q, want %q or a field under it: %v", got, prefix, err)
	}
}

func qaplNames(err error, word string) bool {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_-]|$)`)
	return err != nil && re.MatchString(err.Error())
}

func qaplSameStrings(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func qaplBytes(n int) string {
	s := strings.Repeat("é", n/2)
	if n%2 == 1 {
		s += "a"
	}
	return s
}

func qaplLeast[V any](m map[string]V) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	if len(ks) == 0 {
		return ""
	}
	return ks[0]
}

func qaplSamePlaces(t *testing.T, got, want []rules.Place) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d places %+v, want %d %+v", len(got), got, len(want), want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Key != w.Key || g.Name != w.Name || g.Description != w.Description || g.Copies != w.Copies {
			t.Errorf("place %d: got %+v, want %+v", i, g, w)
		}
		if !qaplSameStrings(g.Forms, w.Forms...) {
			t.Errorf("place %d forms: got %q, want %q", i, g.Forms, w.Forms)
		}
		if !qaplSameStrings(g.States, w.States...) {
			t.Errorf("place %d states: got %q, want %q", i, g.States, w.States)
		}
	}
}

func qaplWantPlaces() []rules.Place {
	return []rules.Place{
		{Key: "mike", Name: "Mike ñ", Description: "Where a mike rests, for the table.",
			Forms: []string{"yankee", "zulu", "xray"}, Copies: true, States: []string{"oscar", "lima"}},
		{Key: "kilo", Name: "Kilo", Description: "Where a kilo rests.", Forms: []string{"zulu"}},
		{Key: "november", Name: "November", Description: "Where a november rests.",
			Forms: []string{"xray", "yankee"}},
	}
}

// VTT-1015.
func TestQAPLPlacesAreKeptAsWrittenInDeclaredOrder(t *testing.T) {
	for _, withSheet := range []bool{false, true} {
		t.Run(fmt.Sprintf("sheet=%v", withSheet), func(t *testing.T) {
			rs, _, err := qaplLoad(t, qaplManifest(t, withSheet), nil)
			if err != nil {
				t.Fatal(err)
			}
			qaplSamePlaces(t, rs.Places, qaplWantPlaces())
		})
	}
}

// VTT-1015.
func TestQAPLNoPlacesWhenAbsentNullOrEmpty(t *testing.T) {
	cases := map[string]func(m qaplObj){
		"absent": func(m qaplObj) { delete(m, "places") },
		"null":   func(m qaplObj) { m["places"] = nil },
		"empty":  func(m qaplObj) { m["places"] = []any{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := qaplManifest(t, false)
			mutate(m)
			rs, _, err := qaplLoad(t, m, nil)
			if err != nil {
				t.Fatal(err)
			}
			if rs.Places != nil {
				t.Errorf("Places = %#v, want nil", rs.Places)
			}
		})
	}
	for _, dir := range []string{qaplTavernDir} {
		rs, err := rules.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if rs.Places != nil {
			t.Errorf("%s: Places = %#v, want nil", dir, rs.Places)
		}
	}
}

// VTT-1016.
func TestQAPLAPlaceWithoutKeyNameOrDescriptionIsRefused(t *testing.T) {
	for _, i := range []int{1, 2} {
		for _, f := range []string{"key", "name", "description"} {
			for _, shape := range []string{"absent", "empty"} {
				t.Run(fmt.Sprintf("%d-%s-%s", i, f, shape), func(t *testing.T) {
					m := qaplManifest(t, false)
					if shape == "absent" {
						delete(qaplPlace(m, i), f)
					} else {
						qaplPlace(m, i)[f] = ""
					}
					_, path, err := qaplLoad(t, m, nil)
					qaplRefused(t, err, path, fmt.Sprintf("places[%d].%s", i, f))
				})
			}
		}
	}
}

// VTT-1016.
func TestQAPLAKeyAnEarlierPlaceDeclaresIsRefused(t *testing.T) {
	for _, earlier := range []string{"mike", "kilo"} {
		t.Run(earlier, func(t *testing.T) {
			m := qaplManifest(t, false)
			qaplPlace(m, 2)["key"] = earlier
			_, path, err := qaplLoad(t, m, nil)
			qaplRefused(t, err, path, "places[2].key")
		})
	}
}

// VTT-1016.
func TestQAPLAPlaceKeyAtTheBoundLoadsAndPastItIsRefused(t *testing.T) {
	for _, key := range []string{qaplBytes(qaplBound), strings.Repeat("k", qaplBound)} {
		m := qaplManifest(t, false)
		qaplPlace(m, 1)["key"] = key
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatalf("a %d-byte key: %v", len(key), err)
		}
		if rs.Places[1].Key != key {
			t.Errorf("key %q, want %q", rs.Places[1].Key, key)
		}
	}
	past := []string{qaplBytes(qaplBound + 1), strings.Repeat("k", qaplBound+1), qaplBytes(qaplBound + 2)}
	for _, key := range past {
		m := qaplManifest(t, false)
		qaplPlace(m, 1)["key"] = key
		_, path, err := qaplLoad(t, m, nil)
		qaplRefused(t, err, path, "places[1].key")
	}
}

// VTT-1016's free key: a place key may equal any other key or id.
func TestQAPLAPlaceKeyIsFreeInItsCharactersAndNamespace(t *testing.T) {
	base, err := rules.Load(qaplTavernDir)
	if err != nil {
		t.Fatal(err)
	}
	abilityID, conditionID := qaplLeast(base.Compiled), qaplLeast(base.Conditions)
	entry := `{"id": "lima-entry", "name": "E", "description": "D", "form": "zulu"}`
	keys := []string{"kilo two/é-3.x", "brawn", "zulu", abilityID, conditionID, "lima-entry"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			m := qaplManifest(t, false)
			qaplPlace(m, 1)["key"] = key
			rs, _, err := qaplLoad(t, m, map[string]string{"catalogue/lima-entry.json": entry})
			if err != nil {
				t.Fatal(err)
			}
			if rs.Places[1].Key != key {
				t.Errorf("key %q, want %q", rs.Places[1].Key, key)
			}
		})
	}
}

// VTT-1017.
func TestQAPLAPlacesFormsAreRefusedWhenAbsentEmptyUndeclaredOrRepeated(t *testing.T) {
	cases := []struct {
		name  string
		forms any
		field string
	}{
		{"absent", nil, "places[1].forms"},
		{"empty", []any{}, "places[1].forms"},
		{"undeclared", []any{"zulu", "nope"}, "places[1].forms[1]"},
		{"empty-key", []any{"zulu", ""}, "places[1].forms[1]"},
		{"a-property-key", []any{"zulu", "brawn"}, "places[1].forms[1]"},
		{"another-case", []any{"Zulu"}, "places[1].forms[0]"},
		{"repeated", []any{"yankee", "zulu", "yankee"}, "places[1].forms[2]"},
		{"undeclared-before-repeat", []any{"zulu", "nope", "zulu"}, "places[1].forms[1]"},
		{"repeat-before-undeclared", []any{"zulu", "zulu", "nope"}, "places[1].forms[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, false)
			if c.forms == nil {
				delete(qaplPlace(m, 1), "forms")
			} else {
				qaplPlace(m, 1)["forms"] = c.forms
			}
			_, path, err := qaplLoad(t, m, nil)
			qaplRefused(t, err, path, c.field)
		})
	}
}

// VTT-1017.
func TestQAPLAPlacesStatesAreRefusedWhenEmptyRepeatedOrPastTheBound(t *testing.T) {
	cases := []struct {
		name   string
		states []any
		field  string
	}{
		{"empty", []any{"lima", ""}, "places[1].states[1]"},
		{"empty-first", []any{"", "lima", "lima"}, "places[1].states[0]"},
		{"repeated", []any{"lima", "oscar", "lima"}, "places[1].states[2]"},
		{"past-the-bound", []any{"lima", qaplBytes(qaplBound + 1)}, "places[1].states[1]"},
		{"past-the-bound-ascii", []any{"lima", strings.Repeat("s", qaplBound+1)}, "places[1].states[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, false)
			qaplPlace(m, 1)["states"] = c.states
			_, path, err := qaplLoad(t, m, nil)
			qaplRefused(t, err, path, c.field)
		})
	}
	t.Run("at-the-bound", func(t *testing.T) {
		m := qaplManifest(t, false)
		at := qaplBytes(qaplBound)
		qaplPlace(m, 1)["states"] = []any{"lima", at}
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !qaplSameStrings(rs.Places[1].States, "lima", at) {
			t.Errorf("states %q", rs.Places[1].States)
		}
	})
	t.Run("another-places-states", func(t *testing.T) {
		m := qaplManifest(t, false)
		qaplPlace(m, 1)["states"] = []any{"lima", "oscar"}
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !qaplSameStrings(rs.Places[1].States, "lima", "oscar") {
			t.Errorf("states %q", rs.Places[1].States)
		}
	})
}

// VTT-1017.
func TestQAPLWithNoFormDeclaredAPlacesFormIsRefused(t *testing.T) {
	cases := map[string]func(m qaplObj){
		"absent": func(m qaplObj) { delete(m, "forms") },
		"null":   func(m qaplObj) { m["forms"] = nil },
		"empty":  func(m qaplObj) { m["forms"] = []any{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := qaplManifest(t, false)
			mutate(m)
			_, path, err := qaplLoad(t, m, nil)
			qaplRefused(t, err, path, "places[0].forms[0]")
		})
	}
}

// VTT-1016, VTT-1017: the order within a place and place by place.
func TestQAPLAPlacesRefusalsComeInTheirOrder(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(m qaplObj)
		field  string
	}{
		{"key-name-description", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["key"], p["name"], p["description"] = "", "", ""
		}, "places[1].key"},
		{"name-description", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["name"], p["description"] = "", ""
		}, "places[1].name"},
		{"description-forms", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["description"], p["forms"] = "", []any{}
		}, "places[1].description"},
		{"repeated-key-then-empty-name", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["key"], p["name"] = "mike", ""
		}, "places[1].name"},
		{"repeated-key-then-empty-description", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["key"], p["description"] = "mike", ""
		}, "places[1].description"},
		{"repeated-key-before-forms", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["key"], p["forms"] = "mike", []any{}
		}, "places[1].key"},
		{"forms-before-states", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["forms"], p["states"] = []any{}, []any{""}
		}, "places[1].forms"},
		{"a-form-before-states", func(m qaplObj) {
			p := qaplPlace(m, 1)
			p["forms"], p["states"] = []any{"nope"}, []any{""}
		}, "places[1].forms[0]"},
		{"place-by-place-forms", func(m qaplObj) {
			qaplPlace(m, 0)["forms"] = []any{}
			qaplPlace(m, 1)["key"] = ""
		}, "places[0].forms"},
		{"place-by-place-states", func(m qaplObj) {
			qaplPlace(m, 0)["states"] = []any{"oscar", "oscar"}
			qaplPlace(m, 1)["name"] = ""
		}, "places[0].states[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, false)
			c.mutate(m)
			_, path, err := qaplLoad(t, m, nil)
			qaplRefused(t, err, path, c.field)
		})
	}
}

type qaplOrderCase struct {
	name      string
	withSheet bool
	first     func(m qaplObj)
	second    func(m qaplObj)
	files     map[string]string
	field     string
	under     string
}

func qaplRunOrder(t *testing.T, cases []qaplOrderCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, c.withSheet)
			c.first(m)
			c.second(m)
			_, path, err := qaplLoad(t, m, c.files)
			if c.under == "" {
				qaplRefused(t, err, path, c.field)
			} else if got := qaplField(err); err == nil || !strings.HasPrefix(got, c.under+"[") {
				t.Errorf("refusal %v names field %q, want one under %q", err, got, c.under)
			}
			alone := qaplManifest(t, c.withSheet)
			c.second(alone)
			_, _, aloneErr := qaplLoad(t, alone, c.files)
			if aloneErr == nil || qaplField(aloneErr) == qaplField(err) {
				t.Errorf("the later refusal alone is %v: the order is not shown", aloneErr)
			}
		})
	}
}

func qaplLongKey(m qaplObj) { qaplPlace(m, 1)["key"] = qaplBytes(qaplBound + 1) }

func qaplLongState(m qaplObj) { qaplPlace(m, 1)["states"] = []any{"lima", qaplBytes(qaplBound + 1)} }

func qaplNothing(qaplObj) {}

func qaplBadEntry() map[string]string {
	return map[string]string{
		"catalogue/lima-entry.json": `{"id": "lima-entry", "name": "E", "description": "D", "form": "nope"}`,
	}
}

func qaplLongFormKey(m qaplObj) {
	m["forms"] = append(m["forms"].([]any), qaplObj{"key": strings.Repeat("f", qaplBound+1),
		"name": "F", "description": "F"})
}

// VTT-1016, VTT-1017: the bounds follow the manifest's, before any other refusal.
func TestQAPLThePlaceBoundsComeInTheirOrder(t *testing.T) {
	badCondition := map[string]string{"conditions/lima-cond.json": `{"id": "", "name": "C"}`}
	qaplRunOrder(t, []qaplOrderCase{
		{"before-a-place-field", false, qaplLongKey,
			func(m qaplObj) { qaplPlace(m, 0)["name"] = "" }, nil, "places[1].key", ""},
		{"state-before-a-place-field", false, qaplLongState,
			func(m qaplObj) { qaplPlace(m, 0)["forms"] = []any{"nope"} }, nil, "places[1].states[1]", ""},
		{"before-a-property", false, qaplLongKey, func(m qaplObj) {
			qaplAddProperty(m, qaplObj{"key": "golf", "type": "number", "name": "", "description": "G"})
		}, nil, "places[1].key", ""},
		{"state-before-a-form", false, qaplLongState,
			func(m qaplObj) { qaplForm(m, 1)["name"] = "" }, nil, "places[1].states[1]", ""},
		{"before-the-sheet", true, qaplLongKey,
			func(m qaplObj) { qaplPage(m, 1)["name"] = "" }, nil, "places[1].key", ""},
		{"before-a-condition", false, qaplLongKey, qaplNothing, badCondition, "places[1].key", ""},
		{"before-the-catalogue", false, qaplLongKey, qaplNothing, qaplBadEntry(), "places[1].key", ""},
		{"after-the-id", false, func(m qaplObj) { m["id"] = strings.Repeat("i", qaplBound+1) },
			qaplLongKey, nil, "id", ""},
		{"after-the-version", false, func(m qaplObj) { m["version"] = strings.Repeat("v", qaplBound+1) },
			qaplLongKey, nil, "version", ""},
		{"after-the-name", false, func(m qaplObj) { m["name"] = strings.Repeat("n", 257) },
			qaplLongKey, nil, "name", ""},
		{"after-a-property-key", false, func(m qaplObj) {
			qaplAddProperty(m, qaplObj{"key": strings.Repeat("g", qaplBound+1), "type": "number",
				"name": "G", "description": "G"})
		}, qaplLongKey, nil, "", "properties"},
		{"after-a-form-key", false, qaplLongFormKey, qaplLongKey, nil, "", "forms"},
		{"state-after-a-form-key", false, qaplLongFormKey, qaplLongState, nil, "", "forms"},
	})
}

// VTT-1016, VTT-1017: places load after properties and forms, before the catalogue.
func TestQAPLAPlacesRefusalComesAfterThePropertiesAndForms(t *testing.T) {
	qaplRunOrder(t, []qaplOrderCase{
		{"form-field-first", false, func(m qaplObj) { qaplForm(m, 1)["name"] = "" },
			func(m qaplObj) { qaplPlace(m, 0)["name"] = "" }, nil, "forms[1].name", ""},
		{"form-property-first", false, func(m qaplObj) { qaplForm(m, 0)["properties"] = []any{"nope"} },
			func(m qaplObj) { qaplPlace(m, 0)["forms"] = []any{} }, nil, "forms[0].properties[0]", ""},
		{"property-first", false, func(m qaplObj) {
			qaplAddProperty(m, qaplObj{"key": "golf", "type": "number", "name": "G", "description": ""})
		}, func(m qaplObj) { qaplPlace(m, 0)["key"] = "" }, nil, "", "properties"},
		{"place-before-the-catalogue", false, func(m qaplObj) { qaplPlace(m, 1)["name"] = "" },
			qaplNothing, qaplBadEntry(), "places[1].name", ""},
	})
}

type qaplUnknownCase struct {
	where     string
	field     string
	withSheet bool
	at        func(m qaplObj) qaplObj
}

// VTT-1018.
func TestQAPLAFieldTheFormatDoesNotDefineIsRefused(t *testing.T) {
	var cases []qaplUnknownCase
	top := func(m qaplObj) qaplObj { return m }
	place := func(m qaplObj) qaplObj { return qaplPlace(m, 1) }
	form := func(m qaplObj) qaplObj { return qaplForm(m, 1) }
	property := func(m qaplObj) qaplObj { return m["properties"].([]any)[1].(qaplObj) }
	page := func(m qaplObj) qaplObj { return qaplPage(m, 1) }
	section := func(m qaplObj) qaplObj { return qaplSection(m, 1, 1) }
	add := func(where string, at func(m qaplObj) qaplObj, withSheet bool, fields ...string) {
		for _, f := range fields {
			cases = append(cases, qaplUnknownCase{where, f, withSheet, at})
		}
	}
	add("top", top, false, "quebec", "copies", "states", "values", "place")
	add("place", place, false, "capacity", "occupies", "admits", "budget", "values", "place", "form", "places")
	add("form", form, false, "places", "copies", "states", "forms")
	add("property", property, false, "forms", "places")
	add("page", page, true, "places")
	add("section", section, true, "quebec", "place", "forms", "copies")
	for _, c := range cases {
		t.Run(c.where+"-"+c.field, func(t *testing.T) {
			m := qaplManifest(t, c.withSheet)
			c.at(m)[c.field] = []any{}
			_, path, err := qaplLoad(t, m, nil)
			if err == nil {
				t.Fatalf("loaded with %q in the %s", c.field, c.where)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("refusal %q does not name %s", err, path)
			}
			if !strings.Contains(err.Error(), "unknown field "+strconv.Quote(c.field)) {
				t.Errorf("refusal %q does not name the field %q", err, c.field)
			}
		})
	}
}

// VTT-1018: a case variant of a defined field is read as it.
func TestQAPLAFieldsNameIsComparedWithoutRegardToCase(t *testing.T) {
	t.Run("top-level", func(t *testing.T) {
		m := qaplManifest(t, true)
		m["PLACES"], m["Forms"], m["SHEET"] = m["places"], m["forms"], m["sheet"]
		delete(m, "places")
		delete(m, "forms")
		delete(m, "sheet")
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		qaplSamePlaces(t, rs.Places, qaplWantPlaces())
	})
	t.Run("in-a-place", func(t *testing.T) {
		m := qaplManifest(t, false)
		p := qaplPlace(m, 1)
		delete(p, "key")
		delete(p, "forms")
		p["KEY"], p["Forms"], p["Copies"], p["STATES"] = "kilo", []any{"zulu", "xray"}, true, []any{"lima"}
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := qaplWantPlaces()
		want[1].Forms, want[1].Copies, want[1].States = []string{"zulu", "xray"}, true, []string{"lima"}
		qaplSamePlaces(t, rs.Places, want)
	})
	t.Run("in-a-section", func(t *testing.T) {
		m := qaplManifest(t, true)
		s := qaplSection(m, 1, 1)
		s["Places"] = s["places"]
		delete(s, "places")
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := rs.Sheet[1].Sections[1].Places; !qaplSameStrings(got, "november", "mike") {
			t.Errorf("section places %q", got)
		}
	})
}

// VTT-824 for a places section.
func TestQAPLTheSheetKeepsEachPlacesSectionAsWritten(t *testing.T) {
	rs, _, err := qaplLoad(t, qaplManifest(t, true), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Sheet) != 2 || len(rs.Sheet[1].Sections) != 3 {
		t.Fatalf("sheet %+v", rs.Sheet)
	}
	u := rs.Sheet[1].Sections[1]
	if u.Name != "Uniform" || !qaplSameStrings(u.Places, "november", "mike") ||
		len(u.Values) != 0 || len(u.Abilities) != 0 || u.Conditions {
		t.Errorf("section %+v", u)
	}
	if v := rs.Sheet[1].Sections[2]; v.Name != "Victor" || !qaplSameStrings(v.Places, "kilo") {
		t.Errorf("section %+v", v)
	}
	for _, s := range []rules.SheetSection{rs.Sheet[0].Sections[0], rs.Sheet[0].Sections[1], rs.Sheet[1].Sections[0]} {
		if len(s.Places) != 0 {
			t.Errorf("section %q shows places %q", s.Name, s.Places)
		}
	}
}

// VTT-1019.
func TestQAPLASheetsShapeIsRefusedWithPlaces(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(m qaplObj)
		field  string
		under  string
	}{
		{"no-page", func(m qaplObj) { m["sheet"] = []any{} }, "sheet", ""},
		{"page-empty-name", func(m qaplObj) { qaplPage(m, 1)["name"] = "" }, "sheet[1].name", ""},
		{"page-repeated-name", func(m qaplObj) { qaplPage(m, 1)["name"] = "Papa" }, "sheet[1].name", ""},
		{"page-no-section", func(m qaplObj) { qaplPage(m, 1)["sections"] = []any{} }, "sheet[1].sections", ""},
		{"section-empty-name", func(m qaplObj) { qaplSection(m, 1, 1)["name"] = "" },
			"sheet[1].sections[1].name", ""},
		{"section-shows-none", func(m qaplObj) { delete(qaplSection(m, 1, 1), "places") },
			"", "sheet[1].sections[1]"},
		{"places-and-values", func(m qaplObj) { qaplSection(m, 1, 1)["values"] = []any{"grit"} },
			"", "sheet[1].sections[1]"},
		{"places-and-abilities", func(m qaplObj) {
			qaplSection(m, 1, 1)["abilities"] = []any{qaplObj{"spends": "drink"}}
		}, "", "sheet[1].sections[1]"},
		{"places-and-conditions", func(m qaplObj) {
			tango := qaplSection(m, 1, 0)
			delete(tango, "conditions")
			tango["values"] = []any{"grit"}
			qaplSection(m, 1, 1)["conditions"] = true
		}, "", "sheet[1].sections[1]"},
		{"places-empty", func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{} },
			"sheet[1].sections[1].places", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, true)
			c.mutate(m)
			_, path, err := qaplLoad(t, m, nil)
			if c.under == "" {
				qaplRefused(t, err, path, c.field)
				return
			}
			qaplRefusedUnder(t, err, path, c.under)
		})
	}
	for _, bad := range []any{"mike", []any{7}, true} {
		t.Run(fmt.Sprintf("places-of-type-%T", bad), func(t *testing.T) {
			m := qaplManifest(t, true)
			qaplSection(m, 1, 1)["places"] = bad
			_, path, err := qaplLoad(t, m, nil)
			if err == nil {
				t.Fatalf("loaded with places %v", bad)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("refusal %q does not name %s", err, path)
			}
		})
	}
}

// VTT-1019, VTT-824: a section's kind present as null reads as absent.
func TestQAPLANullKindBesidePlacesReadsAsAbsent(t *testing.T) {
	t.Run("null-places-beside-values", func(t *testing.T) {
		m := qaplManifest(t, true)
		u := qaplSection(m, 1, 1)
		u["places"], u["values"] = nil, []any{"grit"}
		qaplSection(m, 1, 2)["places"] = []any{"kilo", "november", "mike"}
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if s := rs.Sheet[1].Sections[1]; len(s.Places) != 0 || !qaplSameStrings(s.Values, "grit") {
			t.Errorf("section %+v", s)
		}
	})
	t.Run("null-others-beside-places", func(t *testing.T) {
		m := qaplManifest(t, true)
		u := qaplSection(m, 1, 1)
		u["values"], u["abilities"], u["conditions"] = nil, nil, nil
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if s := rs.Sheet[1].Sections[1]; !qaplSameStrings(s.Places, "november", "mike") {
			t.Errorf("section %+v", s)
		}
	})
}

func qaplValuesTwin(t *testing.T) qaplObj {
	t.Helper()
	m := qaplManifest(t, true)
	delete(m, "places")
	u := qaplSection(m, 1, 1)
	delete(u, "places")
	u["values"] = []any{"drink", "footing"}
	v := qaplSection(m, 1, 2)
	delete(v, "places")
	v["values"] = []any{"grit"}
	return m
}

// VTT-1060, VTT-1019: refused as values are.
func TestQAPLASectionsPlacesAreRefusedAsValuesAre(t *testing.T) {
	if _, _, err := qaplLoad(t, qaplValuesTwin(t), nil); err != nil {
		t.Fatalf("the values twin does not load: %v", err)
	}
	extra := func(m qaplObj, kind string, keys ...any) {
		p := qaplPage(m, 0)
		p["sections"] = append(p["sections"].([]any), qaplObj{"name": "Whiskey", kind: keys})
	}
	cases := []struct {
		name   string
		places func(m qaplObj)
		values func(m qaplObj)
		field  string
	}{
		{"empty-list",
			func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{} },
			func(m qaplObj) { qaplSection(m, 1, 1)["values"] = []any{} },
			"sheet[1].sections[1].places"},
		{"undeclared",
			func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{"november", "nope"} },
			func(m qaplObj) { qaplSection(m, 1, 1)["values"] = []any{"drink", "nope"} },
			"sheet[1].sections[1].places[1]"},
		{"empty-key",
			func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{"november", ""} },
			func(m qaplObj) { qaplSection(m, 1, 1)["values"] = []any{"drink", ""} },
			"sheet[1].sections[1].places[1]"},
		{"shown-twice-in-a-section",
			func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{"november", "november"} },
			func(m qaplObj) { qaplSection(m, 1, 1)["values"] = []any{"drink", "drink"} },
			"sheet[1].sections[1].places[1]"},
		{"shown-in-an-earlier-section",
			func(m qaplObj) { qaplSection(m, 1, 2)["places"] = []any{"kilo", "mike"} },
			func(m qaplObj) { qaplSection(m, 1, 2)["values"] = []any{"grit", "footing"} },
			"sheet[1].sections[2].places[1]"},
		{"shown-on-an-earlier-page",
			func(m qaplObj) { extra(m, "places", "mike") },
			func(m qaplObj) { extra(m, "values", "footing") },
			"sheet[1].sections[1].places[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, true)
			c.places(m)
			_, path, err := qaplLoad(t, m, nil)
			qaplRefused(t, err, path, c.field)
			tw := qaplValuesTwin(t)
			c.values(tw)
			_, _, twErr := qaplLoad(t, tw, nil)
			if twErr == nil {
				t.Fatal("the values twin loaded")
			}
			if got := strings.ReplaceAll(qaplField(twErr), "values", "places"); got != qaplField(err) {
				t.Errorf("places refused on %q, values on %q", qaplField(err), qaplField(twErr))
			}
		})
	}
}

// VTT-1060.
func TestQAPLASectionNamingNoDeclaredPlaceIsRefused(t *testing.T) {
	base, err := rules.Load(qaplTavernDir)
	if err != nil {
		t.Fatal(err)
	}
	abilityID := qaplLeast(base.Compiled)
	withJuliett := func(t *testing.T, key string) (string, error) {
		t.Helper()
		m := qaplManifest(t, true)
		m["places"] = append(m["places"].([]any), qaplObj{"key": "juliett", "name": "Juliett",
			"description": "Where a juliett rests.", "forms": []any{"zulu"}})
		qaplSection(m, 1, 2)["places"] = []any{"kilo", key}
		_, path, err := qaplLoad(t, m, nil)
		return path, err
	}
	if _, err := withJuliett(t, "juliett"); err != nil {
		t.Fatalf("the control, a declared place in that position, is refused: %v", err)
	}
	for _, key := range []string{"zulu", "brawn", "stamina", abilityID, "oscar", "Kilo", "Juliett"} {
		t.Run(key, func(t *testing.T) {
			path, err := withJuliett(t, key)
			qaplRefused(t, err, path, "sheet[1].sections[2].places[1]")
		})
	}
}

// VTT-1060, VTT-1020.
func TestQAPLARulesetDeclaringNoPlace(t *testing.T) {
	t.Run("a-section-names-one", func(t *testing.T) {
		m := qaplManifest(t, true)
		delete(m, "places")
		_, path, err := qaplLoad(t, m, nil)
		qaplRefused(t, err, path, "sheet[1].sections[1].places[0]")
	})
	t.Run("a-null-sheet-shows-none-and-loads", func(t *testing.T) {
		m := qaplManifest(t, false)
		m["sheet"] = nil
		rs, _, err := qaplLoad(t, m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if rs.Sheet != nil || len(rs.Places) != 3 {
			t.Errorf("sheet %+v, places %+v", rs.Sheet, rs.Places)
		}
	})
}

func qaplFirstAtWill(t *testing.T) string {
	t.Helper()
	base, err := rules.Load(qaplTavernDir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for id, p := range base.Compiled {
		if p.Usage.AtWill {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) < 2 {
		t.Fatalf("the fixture has at-will abilities %q", ids)
	}
	return ids[0]
}

// VTT-1020.
func TestQAPLASheetThatShowsNoPlaceIsRefused(t *testing.T) {
	dropVictor := func(m qaplObj) {
		p := qaplPage(m, 1)
		p["sections"] = p["sections"].([]any)[:2]
	}
	cases := []struct {
		name   string
		mutate func(m qaplObj)
		place  string
	}{
		{"declared-second", dropVictor, "kilo"},
		{"declared-first", func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{"november"} }, "mike"},
		{"declared-last", func(m qaplObj) { qaplSection(m, 1, 1)["places"] = []any{"mike"} }, "november"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := qaplManifest(t, true)
			c.mutate(m)
			_, path, err := qaplLoad(t, m, nil)
			if err == nil {
				t.Fatalf("loaded with %q shown by no section", c.place)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("refusal %q does not name %s", err, path)
			}
			if !qaplNames(err, c.place) {
				t.Errorf("refusal %q does not name the place %q", err, c.place)
			}
			if got := qaplField(err); got != "sheet" {
				t.Errorf("refusal names field %q, want %q as an unshown ability's: %v", got, "sheet", err)
			}
		})
	}
	t.Run("after-the-abilities", func(t *testing.T) {
		m := qaplManifest(t, true)
		dropVictor(m)
		qaplSection(m, 0, 1)["abilities"] = []any{qaplObj{"spends": "stamina"}}
		_, path, err := qaplLoad(t, m, nil)
		qaplRefused(t, err, path, "sheet")
		if want := qaplFirstAtWill(t); !qaplNames(err, want) || qaplNames(err, "kilo") {
			t.Errorf("refusal %q, want the ability %q and not the place", err, want)
		}
	})
	t.Run("before-the-catalogue", func(t *testing.T) {
		m := qaplManifest(t, true)
		dropVictor(m)
		files := map[string]string{
			"catalogue/lima-entry.json": `{"id": "lima-entry", "name": "E", "description": "D", "form": "nope"}`,
		}
		_, path, err := qaplLoad(t, m, files)
		if err == nil || !strings.Contains(err.Error(), path) || !qaplNames(err, "kilo") {
			t.Errorf("refusal %v, want one naming %s and the place", err, path)
		}
	})
	t.Run("no-sheet-shows-none-and-loads", func(t *testing.T) {
		if _, _, err := qaplLoad(t, qaplManifest(t, false), nil); err != nil {
			t.Fatal(err)
		}
	})
}

func qaplSchema(t *testing.T, name string) qaplObj {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("schema", name))
	if err != nil {
		t.Fatal(err)
	}
	var s qaplObj
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func qaplAt(t *testing.T, n qaplObj, path ...string) qaplObj {
	t.Helper()
	cur := n
	for _, p := range path {
		next, ok := cur[p].(qaplObj)
		if !ok {
			t.Fatalf("schema has no object at %q of %q", p, path)
		}
		cur = next
	}
	return cur
}

func qaplKeys(n qaplObj) []string {
	var ks []string
	for k := range n {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func qaplRequired(n qaplObj) []string {
	var rs []string
	for _, r := range n["required"].([]any) {
		rs = append(rs, r.(string))
	}
	sort.Strings(rs)
	return rs
}

// VTT-1021.
func TestQAPLTheSchemaDescribesPlaces(t *testing.T) {
	s := qaplSchema(t, "ruleset.schema.json")
	top := qaplAt(t, s, "properties")
	if got := qaplKeys(top); !qaplSameStrings(got, "format_version", "forms", "free_usage_name", "id",
		"name", "places", "properties", "sheet", "version") {
		t.Errorf("top-level fields %q", got)
	}
	places := qaplAt(t, top, "places")
	if places["type"] != "array" {
		t.Errorf("places type %v", places["type"])
	}
	item := qaplAt(t, places, "items")
	if item["type"] != "object" || item["additionalProperties"] != false {
		t.Errorf("a place is %v with additionalProperties %v", item["type"], item["additionalProperties"])
	}
	fields := qaplAt(t, item, "properties")
	if got := qaplKeys(fields); !qaplSameStrings(got, "copies", "description", "forms", "key", "name", "states") {
		t.Errorf("a place's fields %q", got)
	}
	if got := qaplRequired(item); !qaplSameStrings(got, "description", "forms", "key", "name") {
		t.Errorf("a place's required fields %q", got)
	}
	for _, f := range []string{"key", "name", "description"} {
		if n := qaplAt(t, fields, f); n["type"] != "string" || n["minLength"] != float64(1) {
			t.Errorf("%s is %v", f, n)
		}
	}
	if n := qaplAt(t, fields, "copies"); n["type"] != "boolean" {
		t.Errorf("copies is %v", n)
	}
	forms := qaplAt(t, fields, "forms")
	if forms["type"] != "array" || forms["minItems"] != float64(1) || qaplAt(t, forms, "items")["type"] != "string" {
		t.Errorf("forms is %v", forms)
	}
	states := qaplAt(t, fields, "states")
	sItems := qaplAt(t, states, "items")
	if states["type"] != "array" || sItems["type"] != "string" || sItems["minLength"] != float64(1) {
		t.Errorf("states is %v", states)
	}
	section := qaplAt(t, top, "sheet", "items", "properties", "sections", "items")
	sp := qaplAt(t, section, "properties", "places")
	if sp["type"] != "array" || sp["minItems"] != float64(1) || qaplAt(t, sp, "items")["type"] != "string" {
		t.Errorf("a section's places is %v", sp)
	}
	found := false
	for _, alt := range section["oneOf"].([]any) {
		if r, ok := alt.(qaplObj)["required"].([]any); ok && len(r) == 1 && r[0] == "places" {
			found = true
		}
	}
	if !found {
		t.Errorf("a section's oneOf %v has no places arm", section["oneOf"])
	}
}

// VTT-1021, VTT-1016, VTT-1017: the schema requires what Load refuses absent.
func TestQAPLTheSchemasRequiredPlaceFieldsAreTheLoadersOwn(t *testing.T) {
	item := qaplAt(t, qaplSchema(t, "ruleset.schema.json"), "properties", "places", "items")
	required := map[string]bool{}
	for _, r := range qaplRequired(item) {
		required[r] = true
	}
	for _, f := range qaplKeys(qaplAt(t, item, "properties")) {
		t.Run(f, func(t *testing.T) {
			m := qaplManifest(t, false)
			delete(qaplPlace(m, 0), f)
			_, path, err := qaplLoad(t, m, nil)
			if !required[f] {
				if err != nil {
					t.Errorf("optional %q absent: %v", f, err)
				}
				return
			}
			qaplRefused(t, err, path, "places[0]."+f)
		})
	}
}

func qaplStatedBound(t *testing.T, n qaplObj, what string) int {
	t.Helper()
	text, _ := n["description"].(string)
	if items, ok := n["items"].(qaplObj); ok {
		if d, ok := items["description"].(string); ok {
			text += " " + d
		}
	}
	m := qaplBoundRE.FindStringSubmatch(text)
	if m == nil {
		t.Errorf("%s states no bound: %q", what, text)
		return 0
	}
	b, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// VTT-1022.
func TestQAPLTheSchemasStateEachIDBound(t *testing.T) {
	rs := qaplSchema(t, "ruleset.schema.json")
	stated := map[string]qaplObj{
		"the ruleset's id": qaplAt(t, rs, "properties", "id"),
		"a property key":   qaplAt(t, rs, "properties", "properties", "items", "properties", "key"),
		"a form key":       qaplAt(t, rs, "properties", "forms", "items", "properties", "key"),
		"a place key":      qaplAt(t, rs, "properties", "places", "items", "properties", "key"),
		"a place's state":  qaplAt(t, rs, "properties", "places", "items", "properties", "states"),
		"an ability id":    qaplAt(t, qaplSchema(t, "ability.schema.json"), "properties", "id"),
		"a condition id":   qaplAt(t, qaplSchema(t, "condition.schema.json"), "properties", "id"),
		"an entry id":      qaplAt(t, qaplSchema(t, "entry.schema.json"), "properties", "id"),
		"a branch label":   qaplAt(t, qaplSchema(t, "atom.schema.json"), "$defs", "resolutionContribution", "properties", "branches"),
	}
	for what, n := range stated {
		if got := qaplStatedBound(t, n, what); got != qaplBound {
			t.Errorf("%s: stated bound %d, want %d", what, got, qaplBound)
		}
	}
}

// VTT-1022, VTT-1016, VTT-1017: the stated bound is the one Load enforces.
func TestQAPLTheStatedPlaceBoundsAreTheOnesLoadEnforces(t *testing.T) {
	fields := qaplAt(t, qaplSchema(t, "ruleset.schema.json"), "properties", "places", "items", "properties")
	keyBound := qaplStatedBound(t, qaplAt(t, fields, "key"), "a place key")
	stateBound := qaplStatedBound(t, qaplAt(t, fields, "states"), "a place's state")
	for n, wantRefused := range map[int]bool{keyBound: false, keyBound + 1: true} {
		m := qaplManifest(t, false)
		qaplPlace(m, 2)["key"] = qaplBytes(n)
		_, path, err := qaplLoad(t, m, nil)
		if wantRefused {
			qaplRefused(t, err, path, "places[2].key")
		} else if err != nil {
			t.Errorf("a %d-byte key: %v", n, err)
		}
	}
	for n, wantRefused := range map[int]bool{stateBound: false, stateBound + 1: true} {
		m := qaplManifest(t, false)
		qaplPlace(m, 2)["states"] = []any{"oscar", qaplBytes(n)}
		_, path, err := qaplLoad(t, m, nil)
		if wantRefused {
			qaplRefused(t, err, path, "places[2].states[1]")
		} else if err != nil {
			t.Errorf("a %d-byte state: %v", n, err)
		}
	}
}
