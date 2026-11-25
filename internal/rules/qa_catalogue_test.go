package rules_test

import (
	"encoding/json"
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

const qactBase = "../../rulesets/tavern-brawl"

type qactFields map[string]string

func qactStr(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (f qactFields) json() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, qactStr(k)+": "+f[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func (f qactFields) with(field, raw string) qactFields {
	out := qactFields{}
	for k, v := range f {
		out[k] = v
	}
	out[field] = raw
	return out
}

func (f qactFields) without(field string) qactFields {
	out := qactFields{}
	for k, v := range f {
		if k != field {
			out[k] = v
		}
	}
	return out
}

func qactValid(id string) qactFields {
	return qactFields{
		"id":          qactStr(id),
		"name":        qactStr("A Plain Thing"),
		"description": qactStr("Plain words about it."),
		"form":        qactStr("kit"),
		"values":      `{"brawn": 2, "grit": -3}`,
	}
}

func qactForms() []map[string]any {
	return []map[string]any{
		{"key": "kit", "name": "Kit", "description": "Something carried.",
			"properties": []string{"brawn", "grit"}},
		{"key": "tool", "name": "Tool", "description": "Something used.",
			"properties": []string{"footing"}},
		{"key": "trinket", "name": "Trinket", "description": "Something kept."},
	}
}

func qactCopyDir(t *testing.T, src, dst string) {
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
		return os.WriteFile(target, b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func qactRuleset(t *testing.T, forms []map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	qactCopyDir(t, qactBase, dir)
	if forms == nil {
		return dir
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	f, err := json.Marshal(forms)
	if err != nil {
		t.Fatal(err)
	}
	m["forms"] = f
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func qactWrite(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const qactLone = "lone-entry.json"

func qactLoadOne(t *testing.T, f qactFields) (*rules.Ruleset, error) {
	t.Helper()
	dir := qactRuleset(t, qactForms())
	qactWrite(t, dir, filepath.Join("catalogue", qactLone), f.json())
	return rules.Load(dir)
}

func qactWantRefusal(t *testing.T, err error, names ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load succeeded; want a refusal naming %q", names)
	}
	for _, n := range names {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("refusal %q does not name %q", err, n)
		}
	}
}

func qactValuesEqual(got, want map[string]int) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		g, ok := got[k]
		if !ok || g != v {
			return false
		}
	}
	return true
}

func qactEntrySchema(t *testing.T) map[string]any {
	t.Helper()
	var found []byte
	err := fs.WalkDir(rules.Schemas, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Base(p) == "entry.schema.json" {
			b, rerr := fs.ReadFile(rules.Schemas, p)
			found = b
			return rerr
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("rules.Schemas holds no entry.schema.json")
	}
	var m map[string]any
	if err := json.Unmarshal(found, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// VTT-997
func TestQACTARulesetWithNoCatalogueLoadsWithNone(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"no forms, no catalogue": func(t *testing.T) string {
			t.Helper()
			return qactRuleset(t, nil)
		},
		"forms, no catalogue": func(t *testing.T) string {
			t.Helper()
			return qactRuleset(t, qactForms())
		},
		"forms, an empty catalogue": func(t *testing.T) string {
			t.Helper()
			dir := qactRuleset(t, qactForms())
			if err := os.Mkdir(filepath.Join(dir, "catalogue"), 0o755); err != nil {
				t.Fatal(err)
			}
			return dir
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			rs, err := rules.Load(build(t))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(rs.Catalogue) != 0 {
				t.Errorf("Catalogue holds %d entries; want none", len(rs.Catalogue))
			}
		})
	}
}

// VTT-997, VTT-1000
func TestQACTEntriesAreKeptByIDWithTheirWordsFormAndValues(t *testing.T) {
	dir := qactRuleset(t, qactForms())
	files := map[string]qactFields{
		"first.json": qactValid("entry-a").
			with("name", qactStr("Röd Lykta ✦")).
			with("description", qactStr("Lights the way, \"quoted\"\nand on.")).
			with("values", `{"brawn": 4, "grit": -2}`),
		"second.json": qactValid("entry-b").
			with("name", qactStr("Grey Rope")).
			with("description", qactStr("Long and frayed.")).
			with("form", qactStr("tool")).
			with("values", `{"footing": 7}`),
		"third.json": qactValid("entry-c").
			with("name", qactStr("Small Lens")).
			with("values", `{"grit": 0}`),
		"fourth.json": qactValid("entry-d").
			with("name", qactStr("Odd Coin")).
			with("form", qactStr("trinket")).
			without("values"),
		"fifth.json": qactValid("entry-e").
			with("name", qactStr("Blunt Hook")).
			with("form", qactStr("tool")).
			with("values", `{}`),
	}
	for file, f := range files {
		qactWrite(t, dir, filepath.Join("catalogue", file), f.json())
	}
	want := map[string]rules.Entry{
		"entry-a": {ID: "entry-a", Name: "Röd Lykta ✦",
			Description: "Lights the way, \"quoted\"\nand on.", Form: "kit",
			Values: map[string]int{"brawn": 4, "grit": -2}},
		"entry-b": {ID: "entry-b", Name: "Grey Rope",
			Description: "Long and frayed.", Form: "tool",
			Values: map[string]int{"footing": 7}},
		"entry-c": {ID: "entry-c", Name: "Small Lens",
			Description: "Plain words about it.", Form: "kit",
			Values: map[string]int{"grit": 0}},
		"entry-d": {ID: "entry-d", Name: "Odd Coin",
			Description: "Plain words about it.", Form: "trinket",
			Values: map[string]int{}},
		"entry-e": {ID: "entry-e", Name: "Blunt Hook",
			Description: "Plain words about it.", Form: "tool",
			Values: map[string]int{}},
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rs.Catalogue) != len(want) {
		t.Errorf("Catalogue holds %d entries; want %d", len(rs.Catalogue), len(want))
	}
	for id, w := range want {
		got := rs.Catalogue[id]
		if got == nil {
			t.Errorf("Catalogue holds no entry %q", id)
			continue
		}
		if got.ID != w.ID || got.Name != w.Name ||
			got.Description != w.Description || got.Form != w.Form {
			t.Errorf("entry %q = %+v; want %+v", id, *got, w)
		}
		if !qactValuesEqual(got.Values, w.Values) {
			t.Errorf("entry %q values = %v; want %v", id, got.Values, w.Values)
		}
	}
}

// VTT-1000
func TestQACTAnEntryWithoutAValueForAListedPropertyHoldsNone(t *testing.T) {
	cases := map[string]struct {
		values  string
		absent  []string
		present map[string]int
	}{
		"one of two carried": {`{"brawn": 5}`, []string{"grit"},
			map[string]int{"brawn": 5}},
		"no values field": {"", []string{"brawn", "grit"}, map[string]int{}},
		"zero is carried": {`{"grit": 0}`, []string{"brawn"},
			map[string]int{"grit": 0}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := qactValid("entry-x").without("values")
			if c.values != "" {
				f = f.with("values", c.values)
			}
			rs, err := qactLoadOne(t, f)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			e := rs.Catalogue["entry-x"]
			if e == nil {
				t.Fatal("Catalogue holds no entry-x")
			}
			for _, k := range c.absent {
				if v, ok := e.Values[k]; ok {
					t.Errorf("entry holds %q = %d; want no value", k, v)
				}
			}
			if !qactValuesEqual(e.Values, c.present) {
				t.Errorf("values = %v; want %v", e.Values, c.present)
			}
		})
	}
}

// VTT-998
func TestQACTAnEntryWithAFieldAbsentOrEmptyIsRefused(t *testing.T) {
	for _, field := range []string{"id", "name", "description", "form"} {
		t.Run(field+" absent", func(t *testing.T) {
			_, err := qactLoadOne(t, qactValid("entry-x").without(field))
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone),
				strconv.Quote(field))
		})
		t.Run(field+" empty", func(t *testing.T) {
			_, err := qactLoadOne(t, qactValid("entry-x").with(field, `""`))
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone),
				strconv.Quote(field))
		})
	}
	t.Run("control loads", func(t *testing.T) {
		if _, err := qactLoadOne(t, qactValid("entry-x")); err != nil {
			t.Fatalf("Load: %v", err)
		}
	})
}

// VTT-998
func TestQACTAnIDPastOneHundredTwentyEightBytesIsRefused(t *testing.T) {
	cases := map[string]string{
		"129 ascii":                  strings.Repeat("a", 129),
		"65 two-byte runes":          strings.Repeat("é", 65),
		"43 three-byte runes":        strings.Repeat("€", 43),
		"128 runes, 129 bytes":       strings.Repeat("a", 127) + "é",
		"far past, four-byte runes":  strings.Repeat("𝄞", 40),
		"130 ascii":                  strings.Repeat("b", 130),
		"128 ascii then a two-byter": strings.Repeat("a", 128) + "é",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			if len(id) <= 128 {
				t.Fatalf("fixture is %d bytes", len(id))
			}
			_, err := qactLoadOne(t, qactValid(id))
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone), `"id"`)
		})
	}
}

// VTT-998
func TestQACTAnIDOfExactlyOneHundredTwentyEightBytesLoads(t *testing.T) {
	cases := map[string]string{
		"128 ascii":            strings.Repeat("a", 128),
		"64 two-byte runes":    strings.Repeat("é", 64),
		"127 runes, 128 bytes": strings.Repeat("a", 126) + "é",
		"32 four-byte runes":   strings.Repeat("𝄞", 32),
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			if len(id) != 128 {
				t.Fatalf("fixture is %d bytes", len(id))
			}
			rs, err := qactLoadOne(t, qactValid(id))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if e := rs.Catalogue[id]; e == nil || e.ID != id {
				t.Errorf("Catalogue does not hold the entry by its id")
			}
		})
	}
}

// VTT-998
func TestQACTAnIDTwoFilesDeclareIsRefused(t *testing.T) {
	dir := qactRuleset(t, qactForms())
	qactWrite(t, dir, "catalogue/alpha.json", qactValid("entry-x").json())
	qactWrite(t, dir, "catalogue/omega.json", qactValid("entry-x").
		with("name", qactStr("Another Thing")).with("form", qactStr("tool")).
		with("values", `{"footing": 1}`).json())
	_, err := rules.Load(dir)
	qactWantRefusal(t, err, `"id"`)
	if err != nil && !strings.Contains(err.Error(), "catalogue/alpha.json") &&
		!strings.Contains(err.Error(), "catalogue/omega.json") {
		t.Errorf("refusal %q names neither entry's file", err)
	}

	ctl := qactRuleset(t, qactForms())
	qactWrite(t, ctl, "catalogue/alpha.json", qactValid("entry-x").json())
	qactWrite(t, ctl, "catalogue/omega.json", qactValid("entry-y").json())
	rs, err := rules.Load(ctl)
	if err != nil {
		t.Fatalf("control: Load: %v", err)
	}
	if len(rs.Catalogue) != 2 {
		t.Errorf("control: Catalogue holds %d entries; want 2", len(rs.Catalogue))
	}
}

// VTT-998
func TestQACTAFormNoFormDeclaresIsRefused(t *testing.T) {
	for name, form := range map[string]string{
		"an unknown key":   "nosuch",
		"a property's key": "grit",
		"an ability's id":  "fists",
	} {
		t.Run(name, func(t *testing.T) {
			f := qactValid("entry-x").with("form", qactStr(form)).without("values")
			_, err := qactLoadOne(t, f)
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone), `"form"`)
		})
	}
	t.Run("a ruleset that declares no forms", func(t *testing.T) {
		dir := qactRuleset(t, nil)
		f := qactValid("entry-x").without("values")
		qactWrite(t, dir, filepath.Join("catalogue", qactLone), f.json())
		_, err := rules.Load(dir)
		qactWantRefusal(t, err, filepath.Join("catalogue", qactLone), `"form"`)
	})
}

// VTT-998
func TestQACTAFieldTheEntryFormatDoesNotDefineIsRefused(t *testing.T) {
	for _, field := range []string{"shade", "value", "forms", "ids"} {
		t.Run(field, func(t *testing.T) {
			_, err := qactLoadOne(t, qactValid("entry-x").with(field, `"x"`))
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone),
				strconv.Quote(field))
		})
	}
}

// VTT-998
func TestQACTACaseVariantOfADefinedFieldIsReadAsIt(t *testing.T) {
	variants := map[string]string{
		"id": "ID", "name": "Name", "description": "DESCRIPTION",
		"form": "Form", "values": "VaLuEs",
	}
	build := func(fields ...string) qactFields {
		f := qactValid("entry-x").with("values", `{"brawn": 9}`)
		for _, field := range fields {
			f = f.without(field).with(variants[field], f[field])
		}
		return f
	}
	check := func(t *testing.T, f qactFields) {
		t.Helper()
		rs, err := qactLoadOne(t, f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		e := rs.Catalogue["entry-x"]
		if e == nil {
			t.Fatal("Catalogue holds no entry-x")
		}
		if e.Name != "A Plain Thing" || e.Description != "Plain words about it." ||
			e.Form != "kit" || !qactValuesEqual(e.Values, map[string]int{"brawn": 9}) {
			t.Errorf("entry = %+v", *e)
		}
	}
	for field := range variants {
		t.Run(field, func(t *testing.T) { check(t, build(field)) })
	}
	t.Run("all at once", func(t *testing.T) {
		check(t, build("id", "name", "description", "form", "values"))
	})
}

// VTT-999
func TestQACTAValueForAPropertyItsFormDoesNotListIsRefused(t *testing.T) {
	cases := map[string]struct{ form, values, key string }{
		"another form's property": {"kit", `{"brawn": 1, "footing": 2}`, "footing"},
		"an undeclared key":       {"kit", `{"brawn": 1, "nosuch": 2}`, "nosuch"},
		"a resource property":     {"kit", `{"grit": 1, "drink": 2}`, "drink"},
		"a form that lists none":  {"trinket", `{"brawn": 1}`, "brawn"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := qactValid("entry-x").with("form", qactStr(c.form)).
				with("values", c.values)
			_, err := qactLoadOne(t, f)
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone), c.key)
		})
	}
}

// VTT-999
func TestQACTAValueThatIsNotAThirtyTwoBitIntegerIsRefused(t *testing.T) {
	cases := map[string]string{
		"null":           `null`,
		"a fraction":     `1.5`,
		"a negative one": `-0.25`,
		"a string":       `"3"`,
		"a boolean":      `true`,
		"an array":       `[3]`,
		"one past max":   `2147483648`,
		"one past min":   `-2147483649`,
		"far past max":   `9223372036854775808`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			f := qactValid("entry-x").with("values", `{"brawn": 1, "grit": `+raw+`}`)
			_, err := qactLoadOne(t, f)
			qactWantRefusal(t, err, filepath.Join("catalogue", qactLone), "grit")
		})
	}
}

// VTT-999
func TestQACTEachEndOfAThirtyTwoBitIntegerLoads(t *testing.T) {
	f := qactValid("entry-x").
		with("values", `{"brawn": -2147483648, "grit": 2147483647}`)
	rs, err := qactLoadOne(t, f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]int{"brawn": -2147483648, "grit": 2147483647}
	if e := rs.Catalogue["entry-x"]; e == nil || !qactValuesEqual(e.Values, want) {
		t.Errorf("entry = %+v; want values %v", e, want)
	}
}

// VTT-1001
func TestQACTOneIDNamingTwoKindsIsRefusedNamingBothFiles(t *testing.T) {
	cases := map[string]struct{ rel, body, other string }{
		"an entry and an ability": {"catalogue/echo.json",
			qactValid("fists").json(), "abilities/fists.json"},
		"an entry and a condition": {"catalogue/echo.json",
			qactValid("dazed-by-ale").json(), "conditions/dazed-by-ale.json"},
		"an ability and a condition": {"conditions/echo.json",
			`{"id": "fists", "name": "Echo", "description": "A mark."}`,
			"abilities/fists.json"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := qactRuleset(t, qactForms())
			qactWrite(t, dir, c.rel, c.body)
			_, err := rules.Load(dir)
			qactWantRefusal(t, err, c.rel, c.other, `"id"`)
		})
	}
	t.Run("control: distinct ids load", func(t *testing.T) {
		dir := qactRuleset(t, qactForms())
		qactWrite(t, dir, "catalogue/echo.json", qactValid("echo-entry").json())
		qactWrite(t, dir, "conditions/echo.json",
			`{"id": "echo-mark", "name": "Echo", "description": "A mark."}`)
		if _, err := rules.Load(dir); err != nil {
			t.Fatalf("Load: %v", err)
		}
	})
}

// VTT-1008
func TestQACTTheEntrySchemaDescribesEveryFieldARequiredFieldAndValues(t *testing.T) {
	s := qactEntrySchema(t)
	props, _ := s["properties"].(map[string]any)
	for _, field := range []string{"id", "name", "description", "form", "values"} {
		p, _ := props[field].(map[string]any)
		if p == nil {
			t.Errorf("schema does not describe %q", field)
			continue
		}
		if d, _ := p["description"].(string); d == "" {
			t.Errorf("schema's %q has no description", field)
		}
	}
	if ap, ok := s["additionalProperties"].(bool); !ok || ap {
		t.Errorf("additionalProperties = %v; want false", s["additionalProperties"])
	}
	var required []string
	reqRaw, _ := s["required"].([]any)
	for _, r := range reqRaw {
		required = append(required, r.(string))
	}
	sort.Strings(required)
	want := []string{"description", "form", "id", "name"}
	if strings.Join(required, ",") != strings.Join(want, ",") {
		t.Errorf("required = %v; want %v", required, want)
	}
	values, _ := props["values"].(map[string]any)
	item, _ := values["additionalProperties"].(map[string]any)
	if item["type"] != "integer" || item["minimum"] != float64(-2147483648) ||
		item["maximum"] != float64(2147483647) {
		t.Errorf("values' items = %v; want an integer in the int32 range", item)
	}

	for field := range props {
		if field == "extends" {
			continue
		}
		_, err := qactLoadOne(t, qactValid("entry-x").without(field))
		isReq := false
		for _, r := range required {
			isReq = isReq || r == field
		}
		if isReq && err == nil {
			t.Errorf("schema requires %q; Load accepts an entry without it", field)
		}
		if !isReq && err != nil {
			t.Errorf("schema does not require %q; Load refuses its absence: %v",
				field, err)
		}
	}
}

// VTT-1008, VTT-1022
func TestQACTTheEntrySchemaStatesTheIDBoundLoadEnforces(t *testing.T) {
	s := qactEntrySchema(t)
	props, _ := s["properties"].(map[string]any)
	id, _ := props["id"].(map[string]any)
	desc, _ := id["description"].(string)
	m := regexp.MustCompile(`(\d+) bytes`).FindStringSubmatch(desc)
	if m == nil {
		t.Fatalf("id's description %q states no bound in bytes", desc)
	}
	bound, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if bound != 128 {
		t.Errorf("schema states %d bytes; Load's bound is 128", bound)
	}
	at := strings.Repeat("é", bound/2) + strings.Repeat("a", bound%2)
	if _, err := qactLoadOne(t, qactValid(at)); err != nil {
		t.Errorf("an id of the stated %d bytes is refused: %v", bound, err)
	}
	past := at + "a"
	if _, err := qactLoadOne(t, qactValid(past)); err == nil {
		t.Errorf("an id of %d bytes loads; the schema states %d", len(past), bound)
	}
}
