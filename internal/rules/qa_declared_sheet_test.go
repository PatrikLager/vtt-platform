package rules_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaSheetMoves = `{"name":"Moves","sections":[{"name":"Free","abilities":["at_will"]},{"name":"Costly","abilities":[{"spends":"stamina"}]}]}`

func qaSheetRead(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func qaSheetCopy(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if rel == "ruleset.json" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// qaSheetLoad loads a copy of the tavern-brawl directory whose ruleset.json
// is its own, with each of extra set on top.
func qaSheetLoad(t *testing.T, extra map[string]string) (*rules.Ruleset, error) {
	t.Helper()
	src := "../../rulesets/tavern-brawl"
	dir := filepath.Join(t.TempDir(), "tavern-brawl")
	qaSheetCopy(t, src, dir)
	m := qaSheetRead(t, filepath.Join(src, "ruleset.json"))
	for k, v := range extra {
		m[k] = json.RawMessage(v)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ruleset.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return rules.Load(dir)
}

func qaSheetSheet(pages ...string) map[string]string {
	return map[string]string{"sheet": "[" + strings.Join(pages, ",") + "]"}
}

func qaSheetMustRefuse(t *testing.T, extra map[string]string, want []string, notWant ...string) {
	t.Helper()
	_, err := qaSheetLoad(t, extra)
	if err == nil {
		t.Fatalf("Load accepted a ruleset it must refuse")
	}
	msg := err.Error()
	at := strings.Index(msg, "ruleset.json")
	if at < 0 {
		t.Fatalf("error %q does not name ruleset.json", msg)
	}
	tail := msg[at+len("ruleset.json"):]
	for _, w := range want {
		if !strings.Contains(tail, w) {
			t.Errorf("error %q does not name %q", msg, w)
		}
	}
	for _, w := range notWant {
		if strings.Contains(tail, w) {
			t.Errorf("error %q names %q, which is not the fault", msg, w)
		}
	}
}

func qaSheetMustLoad(t *testing.T, extra map[string]string) *rules.Ruleset {
	t.Helper()
	rs, err := qaSheetLoad(t, extra)
	if err != nil {
		t.Fatalf("Load refused a ruleset it must accept: %v", err)
	}
	return rs
}

type qaSheetJSONSection struct {
	Name       string            `json:"name"`
	Values     []string          `json:"values"`
	Conditions bool              `json:"conditions"`
	Abilities  []json.RawMessage `json:"abilities"`
}

type qaSheetJSONPage struct {
	Name     string               `json:"name"`
	Sections []qaSheetJSONSection `json:"sections"`
}

func qaSheetExpect(t *testing.T, raw json.RawMessage) []rules.SheetPage {
	t.Helper()
	var pages []qaSheetJSONPage
	if err := json.Unmarshal(raw, &pages); err != nil {
		t.Fatal(err)
	}
	var out []rules.SheetPage
	for _, p := range pages {
		sp := rules.SheetPage{Name: p.Name}
		for _, s := range p.Sections {
			ss := rules.SheetSection{Name: s.Name, Values: s.Values, Conditions: s.Conditions}
			for _, a := range s.Abilities {
				var str string
				if json.Unmarshal(a, &str) == nil {
					if str != "at_will" {
						t.Fatalf("unexpected usage %q", str)
					}
					ss.Abilities = append(ss.Abilities, rules.ShownUsage{AtWill: true})
					continue
				}
				var o struct {
					Spends string `json:"spends"`
				}
				if err := json.Unmarshal(a, &o); err != nil {
					t.Fatal(err)
				}
				ss.Abilities = append(ss.Abilities, rules.ShownUsage{Resource: o.Spends})
			}
			sp.Sections = append(sp.Sections, ss)
		}
		out = append(out, sp)
	}
	return out
}

// VTT-824: the declared sheet of a committed ruleset, in the order declared.
func TestQASheetCommittedRulesetReturnsItsDeclaredSheet(t *testing.T) {
	m := qaSheetRead(t, "../../rulesets/dnd45e-minimal/ruleset.json")
	raw, ok := m["sheet"]
	if !ok {
		t.Fatal("fixture declares no sheet")
	}
	want := qaSheetExpect(t, raw)
	if len(want) < 2 {
		t.Fatalf("fixture sheet has %d pages; order is not shown by fewer than two", len(want))
	}
	rs, err := rules.Load("../../rulesets/dnd45e-minimal")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rs.Sheet, want) {
		t.Fatalf("Sheet = %+v\nwant %+v", rs.Sheet, want)
	}
}

// VTT-824: pages, sections, values and usages come back in declared order,
// which is neither sorted nor the order the properties are declared in.
func TestQASheetOrderIsTheOrderDeclared(t *testing.T) {
	rs := qaSheetMustLoad(t, qaSheetSheet(
		`{"name":"Zeta","sections":[{"name":"Yank","values":["grit","footing","brawn"]},{"name":"Alpha","conditions":true},{"name":"Mid","values":["drink"]}]}`,
		`{"name":"Beta","sections":[{"name":"Zed","abilities":[{"spends":"stamina"},"at_will"]}]}`,
		`{"name":"Alpha","sections":[{"name":"Only","values":["stamina"]}]}`,
	))
	want := []rules.SheetPage{
		{Name: "Zeta", Sections: []rules.SheetSection{
			{Name: "Yank", Values: []string{"grit", "footing", "brawn"}},
			{Name: "Alpha", Conditions: true},
			{Name: "Mid", Values: []string{"drink"}},
		}},
		{Name: "Beta", Sections: []rules.SheetSection{
			{Name: "Zed", Abilities: []rules.ShownUsage{{Resource: "stamina"}, {AtWill: true}}},
		}},
		{Name: "Alpha", Sections: []rules.SheetSection{
			{Name: "Only", Values: []string{"stamina"}},
		}},
	}
	if !reflect.DeepEqual(rs.Sheet, want) {
		t.Fatalf("Sheet = %+v\nwant %+v", rs.Sheet, want)
	}
}

// VTT-824: a ruleset that declares no sheet loads and holds none.
func TestQASheetUndeclaredSheetIsEmpty(t *testing.T) {
	rs, err := rules.Load("../../rulesets/tavern-brawl")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Sheet) != 0 {
		t.Fatalf("Sheet = %+v, want none", rs.Sheet)
	}
}

// VTT-826, VTT-827: a key may stand as a value and in the usage
// that spends it, and a property need not stand at all.
func TestQASheetKeyMayBeValueAndSpentAndOthersMayBeAbsent(t *testing.T) {
	rs := qaSheetMustLoad(t, qaSheetSheet(
		`{"name":"Body","sections":[{"name":"Stats","values":["stamina"]}]}`,
		qaSheetMoves,
	))
	if len(rs.Sheet) != 2 {
		t.Fatalf("Sheet = %+v", rs.Sheet)
	}
}

// VTT-825.
func TestQASheetRefusesAMalformedShape(t *testing.T) {
	cases := []struct {
		name   string
		extra  map[string]string
		want   []string
		absent []string
	}{
		{"no page", map[string]string{"sheet": `[]`}, []string{"sheet"}, []string{"chair-swing"}},
		{"page empty name", qaSheetSheet(`{"name":"","sections":[{"name":"S","conditions":true}]}`, qaSheetMoves), []string{"sheet", "name"}, nil},
		{"page no name field", qaSheetSheet(`{"sections":[{"name":"S","conditions":true}]}`, qaSheetMoves), []string{"sheet", "name"}, nil},
		{"page name held twice", qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":true}]}`, `{"name":"P","sections":[{"name":"T","values":["brawn"]}]}`, qaSheetMoves), []string{"sheet", "name"}, nil},
		{"page name held by the last page", qaSheetSheet(`{"name":"Q","sections":[{"name":"S","conditions":true}]}`, qaSheetMoves, `{"name":"Moves","sections":[{"name":"T","values":["brawn"]}]}`), []string{"sheet", "name"}, nil},
		{"page empty sections", qaSheetSheet(`{"name":"P","sections":[]}`, qaSheetMoves), []string{"sheet", "sections"}, nil},
		{"page no sections field", qaSheetSheet(`{"name":"P"}`, qaSheetMoves), []string{"sheet", "sections"}, nil},
		{"section empty name", qaSheetSheet(`{"name":"P","sections":[{"name":"","conditions":true}]}`, qaSheetMoves), []string{"sheet", "name"}, nil},
		{"section no name field", qaSheetSheet(`{"name":"P","sections":[{"conditions":true}]}`, qaSheetMoves), []string{"sheet", "name"}, nil},
		{"section shows nothing", qaSheetSheet(`{"name":"P","sections":[{"name":"S"}]}`, qaSheetMoves), []string{"sheet"}, nil},
		{"section values and conditions", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"],"conditions":true}]}`, qaSheetMoves), []string{"sheet"}, nil},
		{"section values and abilities", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"],"abilities":[{"spends":"drink"}]}]}`, qaSheetMoves), []string{"sheet"}, nil},
		{"section conditions and abilities", qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":true,"abilities":[{"spends":"drink"}]}]}`, qaSheetMoves), []string{"sheet"}, nil},
		{"section all three", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"],"conditions":true,"abilities":[{"spends":"drink"}]}]}`, qaSheetMoves), []string{"sheet"}, nil},
		{"section empty values", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":[]}]}`, qaSheetMoves), []string{"sheet", "values"}, nil},
		{"section empty abilities", qaSheetSheet(`{"name":"P","sections":[{"name":"S","abilities":[]}]}`, qaSheetMoves), []string{"sheet", "abilities"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { qaSheetMustRefuse(t, c.extra, c.want, c.absent...) })
	}
}

// VTT-825: conditions set to false shows nothing, which is a section
// showing none.
func TestQASheetRefusesConditionsFalse(t *testing.T) {
	qaSheetMustRefuse(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":false}]}`, qaSheetMoves), []string{"sheet"})
}

// VTT-826.
func TestQASheetRefusesAKeyThatFitsNoProperty(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		want  []string
	}{
		{"value undeclared", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn","nosuchkey"]}]}`, qaSheetMoves), []string{"sheet", "values", "nosuchkey"}},
		{"value undeclared first", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["nosuchkey","brawn"]}]}`, qaSheetMoves), []string{"sheet", "values", "nosuchkey"}},
		{"value undeclared on a later page", qaSheetSheet(qaSheetMoves, `{"name":"P","sections":[{"name":"S","values":["nosuchkey"]}]}`), []string{"sheet", "values", "nosuchkey"}},
		{"spends undeclared", qaSheetSheet(`{"name":"P","sections":[{"name":"S","abilities":[{"spends":"nosuchkey"}]}]}`, qaSheetMoves), []string{"sheet", "abilities", "nosuchkey"}},
		{"spends a number property", qaSheetSheet(`{"name":"P","sections":[{"name":"S","abilities":[{"spends":"brawn"}]}]}`, qaSheetMoves), []string{"sheet", "abilities", "brawn"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { qaSheetMustRefuse(t, c.extra, c.want) })
	}
}

// VTT-827.
func TestQASheetRefusesARepeat(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		want  []string
	}{
		{"value in two sections", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]},{"name":"T","values":["grit","brawn"]}]}`, qaSheetMoves), []string{"sheet", "values", "brawn"}},
		{"value on two pages", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]}]}`, `{"name":"Q","sections":[{"name":"T","values":["brawn"]}]}`, qaSheetMoves), []string{"sheet", "values", "brawn"}},
		{"resource value in two sections", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["drink"]},{"name":"T","values":["drink"]}]}`, qaSheetMoves), []string{"sheet", "values", "drink"}},
		{"at will in two sections", qaSheetSheet(`{"name":"P","sections":[{"name":"S","abilities":["at_will"]}]}`, qaSheetMoves), []string{"sheet", "abilities", "at_will"}},
		{"spends in two sections", qaSheetSheet(`{"name":"P","sections":[{"name":"S","abilities":[{"spends":"stamina"}]}]}`, qaSheetMoves), []string{"sheet", "abilities", "stamina"}},
		{"conditions in two sections", qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":true},{"name":"T","conditions":true}]}`, qaSheetMoves), []string{"sheet", "conditions"}},
		{"conditions on two pages", qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":true}]}`, `{"name":"Q","sections":[{"name":"T","conditions":true}]}`, qaSheetMoves), []string{"sheet", "conditions"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { qaSheetMustRefuse(t, c.extra, c.want) })
	}
}

// VTT-827: the same key twice within one section's own list.
func TestQASheetRefusesAKeyTwiceInOneList(t *testing.T) {
	qaSheetMustRefuse(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn","brawn"]}]}`, qaSheetMoves), []string{"sheet", "values", "brawn"})
}

// VTT-828.
func TestQASheetRefusesAnAbilityWhoseUsageNoSectionNames(t *testing.T) {
	qaSheetMustRefuse(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]},{"name":"Free","abilities":["at_will"]}]}`),
		[]string{"chair-swing"})
	qaSheetMustRefuse(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]},{"name":"Costly","abilities":[{"spends":"stamina"}]}]}`),
		[]string{"shows ability"}, "chair-swing")
	qaSheetMustRefuse(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]},{"name":"Costly","abilities":[{"spends":"drink"}]},{"name":"Free","abilities":["at_will"]}]}`),
		[]string{"chair-swing"})
	qaSheetMustRefuse(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":true}]}`), []string{"shows ability"})
}

// VTT-828: the refusal of an unnamed at-will usage names an at-will ability.
func TestQASheetUnnamedAtWillRefusalNamesAnAtWillAbility(t *testing.T) {
	_, err := qaSheetLoad(t, qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]},{"name":"Costly","abilities":[{"spends":"stamina"}]}]}`))
	if err == nil {
		t.Fatal("accepted")
	}
	ok := false
	for _, id := range []string{"fists", "sober-up", "splash-of-water"} {
		ok = ok || strings.Contains(err.Error(), id)
	}
	if !ok {
		t.Fatalf("error %q names no at-will ability", err)
	}
}

// VTT-996.
func TestQASheetRefusesAnUnknownField(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		want  []string
	}{
		{"top level with no sheet", map[string]string{"colour": `"red"`}, []string{"colour"}},
		{"top level beside a sheet", func() map[string]string {
			m := qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]}]}`, qaSheetMoves)
			m["colour"] = `"red"`
			return m
		}(), []string{"colour"}},
		{"page", qaSheetSheet(`{"name":"P","colour":"red","sections":[{"name":"S","values":["brawn"]}]}`, qaSheetMoves), []string{"colour"}},
		{"section", qaSheetSheet(`{"name":"P","sections":[{"name":"S","colour":"red","values":["brawn"]}]}`, qaSheetMoves), []string{"colour"}},
		{"section beside conditions", qaSheetSheet(`{"name":"P","sections":[{"name":"S","conditions":true,"colour":"red"}]}`, qaSheetMoves), []string{"colour"}},
		{"spends object", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]}]}`, `{"name":"Moves","sections":[{"name":"Free","abilities":["at_will"]},{"name":"Costly","abilities":[{"spends":"stamina","colour":"red"}]}]}`), []string{"colour"}},
		{"spends object spelled wrong", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]}]}`, `{"name":"Moves","sections":[{"name":"Free","abilities":["at_will"]},{"name":"Costly","abilities":[{"spend":"stamina"}]}]}`), nil},
		{"usage string other than at_will", qaSheetSheet(`{"name":"P","sections":[{"name":"S","values":["brawn"]}]}`, `{"name":"Moves","sections":[{"name":"Free","abilities":["at_will"]},{"name":"Costly","abilities":["whenever"]}]}`), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { qaSheetMustRefuse(t, c.extra, c.want) })
	}
}

// VTT-996: a field whose name matches a defined one without regard to case
// is not an undefined field.
func TestQASheetCaseVariantOfADefinedFieldIsNotUndefined(t *testing.T) {
	rs := qaSheetMustLoad(t, map[string]string{"SHEET": qaSheetSheet(`{"NAME":"P","Sections":[{"name":"S","VALUES":["brawn"]}]}`, qaSheetMoves)["sheet"]})
	if len(rs.Sheet) != 2 || rs.Sheet[0].Name != "P" {
		t.Fatalf("Sheet = %+v", rs.Sheet)
	}
}

// VTT-829 VTT-858: the schema describes sheet and every field it holds.
func TestQASheetSchemaDescribesTheSheet(t *testing.T) {
	b, err := os.ReadFile("schema/ruleset.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	obj := func(v any, what string) map[string]any {
		t.Helper()
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("schema: %s is not an object", what)
		}
		return m
	}
	props := obj(root["properties"], "top properties")
	fields := []string{"id", "name", "format_version", "version", "properties", "sheet", "free_usage_name", "forms"}
	for _, k := range fields {
		if _, ok := props[k]; !ok {
			t.Errorf("schema top level lacks %q", k)
		}
	}
	if len(props) != len(fields) {
		t.Errorf("schema top level holds %d fields, the format defines %d", len(props), len(fields))
	}
	if root["additionalProperties"] != false {
		t.Error("schema top level admits fields the format does not define")
	}
	if req, _ := root["required"].([]any); len(req) != 4 {
		t.Errorf("sheet must stay optional: required = %v", root["required"])
	}
	sheet := obj(props["sheet"], "sheet")
	if sheet["type"] != "array" {
		t.Errorf("sheet type = %v", sheet["type"])
	}
	if sheet["minItems"] != float64(1) {
		t.Errorf("sheet minItems = %v", sheet["minItems"])
	}
	page := obj(sheet["items"], "page")
	if page["additionalProperties"] != false {
		t.Error("page admits undefined fields")
	}
	pprops := obj(page["properties"], "page properties")
	if len(pprops) != 2 {
		t.Errorf("page holds %d fields, want name and sections", len(pprops))
	}
	pname := obj(pprops["name"], "page name")
	if pname["type"] != "string" || pname["minLength"] != float64(1) {
		t.Errorf("page name = %v", pname)
	}
	sections := obj(pprops["sections"], "sections")
	if sections["type"] != "array" || sections["minItems"] != float64(1) {
		t.Errorf("sections = %v", sections)
	}
	pReq, _ := page["required"].([]any)
	if len(pReq) != 2 {
		t.Errorf("page required = %v", page["required"])
	}
	sec := obj(sections["items"], "section")
	if sec["additionalProperties"] != false {
		t.Error("section admits undefined fields")
	}
	sprops := obj(sec["properties"], "section properties")
	if len(sprops) != 4 {
		t.Errorf("section holds %d fields, want name, values, conditions, abilities", len(sprops))
	}
	sname := obj(sprops["name"], "section name")
	if sname["type"] != "string" || sname["minLength"] != float64(1) {
		t.Errorf("section name = %v", sname)
	}
	values := obj(sprops["values"], "values")
	if values["type"] != "array" || values["minItems"] != float64(1) {
		t.Errorf("values = %v", values)
	}
	if obj(values["items"], "values items")["type"] != "string" {
		t.Error("values items are not strings")
	}
	if obj(sprops["conditions"], "conditions")["const"] != true {
		t.Error("conditions is not const true")
	}
	abil := obj(sprops["abilities"], "abilities")
	if abil["type"] != "array" || abil["minItems"] != float64(1) {
		t.Errorf("abilities = %v", abil)
	}
	one, _ := obj(abil["items"], "abilities items")["oneOf"].([]any)
	if len(one) != 2 {
		t.Fatalf("abilities items oneOf = %v", one)
	}
	seenAtWill, seenSpends := false, false
	for _, v := range one {
		m := obj(v, "usage")
		if m["const"] == "at_will" {
			seenAtWill = true
		}
		if sp, ok := m["properties"].(map[string]any); ok {
			if _, ok := sp["spends"]; ok && len(sp) == 1 && m["additionalProperties"] == false {
				seenSpends = true
			}
		}
	}
	if !seenAtWill || !seenSpends {
		t.Errorf("usage forms at_will=%v spends=%v", seenAtWill, seenSpends)
	}
	if one, _ := sec["oneOf"].([]any); len(one) != 3 {
		t.Errorf("section oneOf = %v, want values, conditions, abilities", sec["oneOf"])
	}
	if req, _ := sec["required"].([]any); len(req) != 1 || req[0] != "name" {
		t.Errorf("section required = %v", sec["required"])
	}
}
