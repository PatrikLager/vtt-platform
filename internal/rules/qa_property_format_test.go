package rules_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaPFTail = `this loader reads format_version "3", which declares every property under "properties" with its key, type, name and description (docs/adr/012-the-rules-contract.md)`

type qaPFDoc = map[string]any

type qaPFNames struct {
	brawn, grit, guard, poolA, vim, braced, swing, steady string
}

type qaPFSet struct {
	docs map[string]qaPFDoc
	raw  map[string]string
}

type qaPFRefusal struct{ path, field, msg string }

func qaPFDefaultNames() qaPFNames {
	return qaPFNames{brawn: "brawn", grit: "grit", guard: "guard", poolA: "pool_a",
		vim: "vim", braced: "braced", swing: "swing", steady: "steady"}
}

func qaPFBuild(n qaPFNames) qaPFSet {
	manifest := qaPFDoc{
		"id": "qa-pf", "name": "QA Property Format", "format_version": "3",
		"properties": []any{
			qaPFDoc{"key": n.guard, "type": "number", "name": "Parry", "description": "How well one turns a blow aside."},
			qaPFDoc{"key": n.brawn, "type": "number", "name": "Might", "description": "How hard one pushes."},
			qaPFDoc{"key": n.vim, "type": "resource", "name": "Verve", "description": "Spirit left for a flourish.", "default_max_expr": "2"},
			qaPFDoc{"key": n.grit, "type": "number", "name": "Nerve", "description": "How long one holds on."},
			qaPFDoc{"key": n.poolA, "type": "resource", "name": "Reserve", "description": "What one can still spend.",
				"default_max_expr": "10 + @" + n.brawn,
				"thresholds":       []any{qaPFDoc{"when": "#" + n.poolA, "apply_condition": n.braced, "remove_when_false": true}}},
		},
	}
	docs := map[string]qaPFDoc{
		"ruleset.json":           manifest,
		"conditions/braced.json": {"id": n.braced, "name": "Braced", "description": "Set against the next push."},
		"atoms/reach-atom.json": {"id": "reach-atom", "params": []any{qaPFDoc{"name": "reach", "kind": "int"}},
			"provides": []any{"delivery"}, "consumes": []any{},
			"contributes": []any{qaPFDoc{"kind": "targeting", "range": "{reach}", "max_targets": 1}}},
		"atoms/self-atom.json": {"id": "self-atom", "params": []any{}, "provides": []any{"delivery"}, "consumes": []any{},
			"contributes": []any{qaPFDoc{"kind": "targeting", "range": 0, "max_targets": 1}}},
		"atoms/roll-atom.json": {"id": "roll-atom",
			"params": []any{qaPFDoc{"name": "attack_stat", "kind": "number"}, qaPFDoc{"name": "edge", "kind": "expr"},
				qaPFDoc{"name": "bar", "kind": "expr"}},
			"provides": []any{"clash"}, "consumes": []any{"delivery"},
			"contributes": []any{qaPFDoc{"kind": "resolution", "key": "clash",
				"roll": "1d20 + @caster.{attack_stat} + {edge}", "vs": "@target." + n.guard + " + {bar}",
				"branches": []any{"hit", "miss"}}}},
		"atoms/drain-atom.json": {"id": "drain-atom",
			"params":   []any{qaPFDoc{"name": "pool", "kind": "resource"}, qaPFDoc{"name": "amount", "kind": "expr"}},
			"provides": []any{}, "consumes": []any{"clash"},
			"contributes": []any{qaPFDoc{"kind": "outcome", "key": "clash", "branch": "hit", "effects": []any{
				qaPFDoc{"resource_change": qaPFDoc{"resource": "{pool}", "delta_expr": "0 - {amount}"}},
				qaPFDoc{"resource_change": qaPFDoc{"resource": n.vim, "delta_expr": "0 - @caster." + n.grit}},
			}}}},
		"atoms/mark-atom.json": {"id": "mark-atom", "params": []any{qaPFDoc{"name": "cond", "kind": "condition"}},
			"provides": []any{}, "consumes": []any{},
			"contributes": []any{qaPFDoc{"kind": "outcome", "key": nil, "branch": "always",
				"effects": []any{qaPFDoc{"apply_condition": qaPFDoc{"id": "{cond}"}}}}}},
		"abilities/swing.json": {"id": n.swing, "name": "Swing",
			"usage": qaPFDoc{"limited": qaPFDoc{"resource": n.vim, "cost": 1}},
			"compose": []any{
				qaPFDoc{"atom": "reach-atom", "bind": qaPFDoc{"reach": 1}},
				qaPFDoc{"atom": "roll-atom", "bind": qaPFDoc{"attack_stat": n.brawn, "edge": "0", "bar": "0"}},
				qaPFDoc{"atom": "drain-atom", "bind": qaPFDoc{"pool": n.poolA, "amount": "1 + @caster." + n.grit}},
			}},
		"abilities/steady.json": {"id": n.steady, "name": "Steady", "usage": "at_will",
			"compose": []any{
				qaPFDoc{"atom": "self-atom", "bind": qaPFDoc{}},
				qaPFDoc{"atom": "mark-atom", "bind": qaPFDoc{"cond": n.braced}},
			}},
	}
	return qaPFSet{docs: docs, raw: map[string]string{}}
}

func qaPFBase() qaPFSet { return qaPFBuild(qaPFDefaultNames()) }

func (s qaPFSet) literalMark(id string) qaPFSet {
	atom := s.docs["atoms/mark-atom.json"]
	atom["params"] = []any{}
	atom["contributes"].([]any)[0].(qaPFDoc)["effects"] = []any{qaPFDoc{"apply_condition": qaPFDoc{"id": id}}}
	delete(s.bind("steady", 1), "cond")
	return s
}

func (s qaPFSet) manifest() qaPFDoc { return s.docs["ruleset.json"] }

func (s qaPFSet) prop(key string) qaPFDoc {
	for _, p := range s.manifest()["properties"].([]any) {
		if d := p.(qaPFDoc); d["key"] == key {
			return d
		}
	}
	panic("qaPF fixture has no property " + key)
}

func (s qaPFSet) addProp(p qaPFDoc) {
	s.manifest()["properties"] = append(s.manifest()["properties"].([]any), p)
}

func (s qaPFSet) bind(ability string, i int) qaPFDoc {
	return s.docs["abilities/"+ability+".json"]["compose"].([]any)[i].(qaPFDoc)["bind"].(qaPFDoc)
}

func (s qaPFSet) contrib(atom string, i int) qaPFDoc {
	return s.docs["atoms/"+atom+".json"]["contributes"].([]any)[i].(qaPFDoc)
}

func (s qaPFSet) change(atom string, ci, ei int) qaPFDoc {
	return s.contrib(atom, ci)["effects"].([]any)[ei].(qaPFDoc)["resource_change"].(qaPFDoc)
}

func qaPFRender(t *testing.T, s qaPFSet) map[string]string {
	t.Helper()
	out := map[string]string{"guide.md": "# QA property format\n"}
	for rel, d := range s.docs {
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		out[rel] = string(b)
	}
	for rel, body := range s.raw {
		out[rel] = body
	}
	return out
}

func qaPFWrite(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func qaPFLoad(t *testing.T, s qaPFSet) (string, *rules.Ruleset, error) {
	t.Helper()
	dir := qaPFWrite(t, qaPFRender(t, s))
	rs, err := rules.Load(dir)
	return dir, rs, err
}

func qaPFMustLoad(t *testing.T, s qaPFSet) *rules.Ruleset {
	t.Helper()
	_, rs, err := qaPFLoad(t, s)
	if err != nil {
		t.Fatalf("Load refused a ruleset that should load: %v", err)
	}
	return rs
}

// qaPFParse reads a refusal of the form `rules: <path>: field "<field>": <message>`.
func qaPFParse(t *testing.T, err error) qaPFRefusal {
	t.Helper()
	if err == nil {
		t.Fatal("Load accepted a ruleset it should refuse")
	}
	s := err.Error()
	rest, ok := strings.CutPrefix(s, "rules: ")
	i := strings.Index(rest, `: field "`)
	if !ok || i < 0 {
		t.Fatalf("refusal %q is not of the form rules: <path>: field \"<field>\": <message>", s)
	}
	after := rest[i+len(`: field "`):]
	j := strings.Index(after, `": `)
	if j < 0 {
		t.Fatalf("refusal %q has no closing field quote", s)
	}
	return qaPFRefusal{path: rest[:i], field: after[:j], msg: after[j+3:]}
}

func qaPFWord(text, word string) bool {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_]|$)`).MatchString(text)
}

func qaPFExpect(t *testing.T, dir string, err error, files, fields []string, words ...string) qaPFRefusal {
	t.Helper()
	r := qaPFParse(t, err)
	got := filepath.ToSlash(r.path)
	named := ""
	for _, f := range files {
		if got == filepath.ToSlash(filepath.Join(dir, f)) || got == f {
			named = f
		}
	}
	if named == "" {
		t.Errorf("refusal names file %q, want one of %v", r.path, files)
	}
	fieldOK := false
	for _, f := range fields {
		fieldOK = fieldOK || strings.Contains(r.field, f)
	}
	if !fieldOK {
		t.Errorf("refusal field %q names none of %v; refusal: %v", r.field, fields, err)
	}
	for _, w := range words {
		if !qaPFWord(r.field+": "+r.msg, w) {
			t.Errorf("refusal does not name %q: %v", w, err)
		}
	}
	t.Logf("file %s, field %q, message %q", named, r.field, r.msg)
	return r
}

func qaPFExpectType(t *testing.T, r qaPFRefusal, typ string) {
	t.Helper()
	if !qaPFWord(r.msg, typ) {
		t.Errorf("refusal message %q does not name the property's type %q", r.msg, typ)
	}
}

// VTT-610
func TestQAPFBaselineLoadsWithEveryField(t *testing.T) {
	rs := qaPFMustLoad(t, qaPFBase())
	want := [][4]string{
		{"guard", "number", "Parry", "How well one turns a blow aside."},
		{"brawn", "number", "Might", "How hard one pushes."},
		{"vim", "resource", "Verve", "Spirit left for a flourish."},
		{"grit", "number", "Nerve", "How long one holds on."},
		{"pool_a", "resource", "Reserve", "What one can still spend."},
	}
	if len(rs.Properties) != len(want) {
		t.Fatalf("Properties has %d entries, want %d", len(rs.Properties), len(want))
	}
	for i, w := range want {
		p := rs.Properties[i]
		if got := [4]string{p.Key, p.Type, p.Name, p.Description}; got != w {
			t.Errorf("Properties[%d] = %v, want %v (declared order)", i, got, w)
		}
		if q := rs.Property(w[0]); q == nil || q.Name != w[2] {
			t.Errorf("Property(%q) = %+v, want the declared %q", w[0], q, w[2])
		}
	}
	if rs.Property("quux") != nil {
		t.Error("Property of an undeclared key is not nil")
	}
	pool := rs.Property("pool_a").Resource
	if pool == nil || pool.DefaultMaxExprSrc != "10 + @brawn" || len(pool.Thresholds) != 1 {
		t.Fatalf("pool_a resource = %+v, want its default_max_expr and one threshold", pool)
	}
	if th := pool.Thresholds[0]; th.WhenSrc != "#pool_a" || th.ApplyCondition != "braced" || !th.RemoveWhenFalse {
		t.Errorf("pool_a threshold = %+v, want the declared when, apply_condition and remove_when_false", th)
	}
	if len(rs.Compiled) != 2 {
		t.Errorf("Compiled has %d abilities, want 2", len(rs.Compiled))
	}
}

// VTT-610
func TestQAPFNameOrDescriptionMissingIsRefused(t *testing.T) {
	for _, key := range []string{"grit", "pool_a"} {
		for _, field := range []string{"name", "description"} {
			for _, absent := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/absent=%v", key, field, absent), func(t *testing.T) {
					s := qaPFBase()
					if absent {
						delete(s.prop(key), field)
					} else {
						s.prop(key)[field] = ""
					}
					dir, _, err := qaPFLoad(t, s)
					qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{field}, key)
				})
			}
		}
	}
}

// VTT-611
func TestQAPFTypeAbsentOrUnknownIsRefused(t *testing.T) {
	for _, typ := range []string{"", "attribute", "Number", "resources", "defense"} {
		t.Run("type="+typ, func(t *testing.T) {
			s := qaPFBase()
			s.prop("grit")["type"] = typ
			dir, _, err := qaPFLoad(t, s)
			r := qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"type"}, "grit")
			if typ != "" && !strings.Contains(r.msg, typ) {
				t.Errorf("refusal message %q does not name the type %q", r.msg, typ)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		s := qaPFBase()
		delete(s.prop("grit"), "type")
		dir, _, err := qaPFLoad(t, s)
		qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"type"}, "grit")
	})
}

// VTT-612
func TestQAPFDuplicateKeyIsRefusedWhateverItsType(t *testing.T) {
	cases := []struct{ key, typ string }{
		{"grit", "number"}, {"grit", "resource"}, {"vim", "resource"}, {"vim", "number"},
	}
	for _, c := range cases {
		t.Run(c.key+"/"+c.typ, func(t *testing.T) {
			s := qaPFBase()
			s.addProp(qaPFDoc{"key": c.key, "type": c.typ, "name": "Echo", "description": "A second entry."})
			dir, _, err := qaPFLoad(t, s)
			qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{""}, c.key)
			s = qaPFBase()
			s.addProp(qaPFDoc{"key": "echo", "type": c.typ, "name": "Echo", "description": "A second entry."})
			qaPFMustLoad(t, s)
		})
	}
}

type qaPFPlace struct {
	name   string
	files  []string
	fields []string
	edit   func(s qaPFSet, ref string)
}

func qaPFPlaces() map[string]qaPFPlace {
	rs, swing := []string{"ruleset.json"}, []string{"abilities/swing.json"}
	both := func(atom string) []string { return []string{"atoms/" + atom + ".json", "abilities/swing.json"} }
	return map[string]qaPFPlace{
		"default_max_expr": {"default_max_expr", rs, []string{"default_max_expr"},
			func(s qaPFSet, ref string) { s.prop("pool_a")["default_max_expr"] = "10 + " + ref }},
		"when": {"when", rs, []string{"when"}, func(s qaPFSet, ref string) {
			s.prop("pool_a")["thresholds"].([]any)[0].(qaPFDoc)["when"] = ref
		}},
		"cost": {"cost", swing, []string{"usage.limited.resource"}, func(s qaPFSet, name string) {
			s.docs["abilities/swing.json"]["usage"].(qaPFDoc)["limited"].(qaPFDoc)["resource"] = name
		}},
		"number-bind": {"number binding", swing, []string{"bind"},
			func(s qaPFSet, name string) { s.bind("swing", 1)["attack_stat"] = name }},
		"resource-bind": {"resource binding", swing, []string{"bind"},
			func(s qaPFSet, name string) { s.bind("swing", 2)["pool"] = name }},
		"roll": {"roll", both("roll-atom"), []string{"roll"}, func(s qaPFSet, ref string) {
			s.contrib("roll-atom", 0)["roll"] = "1d20 + @caster.{attack_stat} + {edge} + " + ref
		}},
		"roll-bound": {"roll as bound", both("roll-atom"), []string{"roll", "edge"},
			func(s qaPFSet, ref string) { s.bind("swing", 1)["edge"] = ref }},
		"vs": {"vs", both("roll-atom"), []string{"vs"}, func(s qaPFSet, ref string) {
			s.contrib("roll-atom", 0)["vs"] = "@target.guard + {bar} + " + ref
		}},
		"vs-bound": {"vs as bound", both("roll-atom"), []string{"vs", "bar"},
			func(s qaPFSet, ref string) { s.bind("swing", 1)["bar"] = ref }},
		"change": {"resource_change.resource", both("drain-atom"), []string{"resource_change.resource"},
			func(s qaPFSet, name string) { s.change("drain-atom", 0, 1)["resource"] = name }},
		"delta": {"delta_expr", both("drain-atom"), []string{"delta_expr"},
			func(s qaPFSet, ref string) { s.change("drain-atom", 0, 1)["delta_expr"] = "0 - " + ref }},
		"delta-bound": {"delta_expr as bound", both("drain-atom"), []string{"delta_expr", "amount"},
			func(s qaPFSet, ref string) { s.bind("swing", 2)["amount"] = "1 + " + ref }},
	}
}

// VTT-613
func TestQAPFUndeclaredKeyIsRefusedAtEveryPlace(t *testing.T) {
	refs := map[string]string{
		"default_max_expr": "@quux", "when": "#quux", "cost": "quux", "number-bind": "quux",
		"resource-bind": "quux", "roll": "@caster.quux", "roll-bound": "#caster.quux", "vs": "@target.quux",
		"vs-bound": "#target.quux", "change": "quux", "delta": "@caster.quux", "delta-bound": "#target.quux",
	}
	places := qaPFPlaces()
	for id, ref := range refs {
		p := places[id]
		t.Run(p.name, func(t *testing.T) {
			s := qaPFBase()
			p.edit(s, ref)
			dir, _, err := qaPFLoad(t, s)
			qaPFExpect(t, dir, err, p.files, p.fields, "quux")
		})
	}
}

// VTT-614
func TestQAPFWrongTypeIsRefusedAtEveryPlace(t *testing.T) {
	cases := []struct{ place, ref, key, typ string }{
		{"default_max_expr", "#grit", "grit", "number"},
		{"default_max_expr", "@vim", "vim", "resource"},
		{"when", "@pool_a", "pool_a", "resource"},
		{"when", "#brawn", "brawn", "number"},
		{"cost", "grit", "grit", "number"},
		{"number-bind", "vim", "vim", "resource"},
		{"resource-bind", "guard", "guard", "number"},
		{"roll", "#caster.grit", "grit", "number"},
		{"roll", "@caster.vim", "vim", "resource"},
		{"roll-bound", "#caster.brawn", "brawn", "number"},
		{"roll-bound", "@caster.pool_a", "pool_a", "resource"},
		{"vs", "#target.guard", "guard", "number"},
		{"vs", "@target.pool_a", "pool_a", "resource"},
		{"vs-bound", "@target.vim", "vim", "resource"},
		{"vs-bound", "#target.grit", "grit", "number"},
		{"change", "guard", "guard", "number"},
		{"delta", "#caster.grit", "grit", "number"},
		{"delta", "@caster.vim", "vim", "resource"},
		{"delta-bound", "@caster.pool_a", "pool_a", "resource"},
		{"delta-bound", "#target.brawn", "brawn", "number"},
	}
	places := qaPFPlaces()
	for _, c := range cases {
		p := places[c.place]
		t.Run(p.name+"/"+c.ref, func(t *testing.T) {
			s := qaPFBase()
			p.edit(s, c.ref)
			dir, _, err := qaPFLoad(t, s)
			r := qaPFExpect(t, dir, err, p.files, p.fields, c.key)
			qaPFExpectType(t, r, c.typ)
		})
	}
}

// VTT-613
func TestQAPFEveryPlaceLoadsWithADeclaredKeyOfTheRightType(t *testing.T) {
	refs := map[string]string{
		"default_max_expr": "@grit", "when": "#vim", "cost": "pool_a", "number-bind": "grit",
		"resource-bind": "vim", "roll": "#caster.vim", "roll-bound": "@caster.grit", "vs": "#target.pool_a",
		"vs-bound": "@target.brawn", "change": "pool_a", "delta": "#caster.pool_a", "delta-bound": "@target.guard",
	}
	for id, ref := range refs {
		p := qaPFPlaces()[id]
		t.Run(p.name, func(t *testing.T) {
			s := qaPFBase()
			p.edit(s, ref)
			qaPFMustLoad(t, s)
		})
	}
}

// VTT-629
func TestQAPFConditionBindingAndThresholdNameADeclaredCondition(t *testing.T) {
	s := qaPFBase()
	s.bind("steady", 1)["cond"] = "nowhere"
	dir, _, err := qaPFLoad(t, s)
	qaPFExpect(t, dir, err, []string{"abilities/steady.json", "atoms/mark-atom.json"}, []string{"bind", "cond", "id"}, "nowhere")
	s = qaPFBase()
	s.prop("pool_a")["thresholds"].([]any)[0].(qaPFDoc)["apply_condition"] = "nowhere"
	dir, _, err = qaPFLoad(t, s)
	qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"apply_condition"}, "nowhere")
}

// VTT-615
func TestQAPFNumberWithResourceFieldsIsRefused(t *testing.T) {
	cases := []struct {
		field string
		value any
	}{
		{"default_max_expr", "1"},
		{"thresholds", []any{}},
		{"thresholds", []any{qaPFDoc{"when": "@grit", "apply_condition": "braced", "remove_when_false": false}}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s=%v", c.field, c.value), func(t *testing.T) {
			s := qaPFBase()
			s.prop("grit")[c.field] = c.value
			dir, _, err := qaPFLoad(t, s)
			qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{c.field}, "grit")
		})
	}
}

// VTT-628
func TestQAPFThresholdFieldsAreEachRequired(t *testing.T) {
	for _, field := range []string{"when", "apply_condition", "remove_when_false"} {
		t.Run(field, func(t *testing.T) {
			s := qaPFBase()
			delete(s.prop("pool_a")["thresholds"].([]any)[0].(qaPFDoc), field)
			dir, _, err := qaPFLoad(t, s)
			qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{field})
		})
	}
}

// VTT-615
func TestQAPFResourceWithoutOptionalFieldsLoads(t *testing.T) {
	s := qaPFBase()
	delete(s.prop("vim"), "default_max_expr")
	s.prop("pool_a")["thresholds"] = []any{}
	rs := qaPFMustLoad(t, s)
	if p := rs.Property("vim"); p == nil || p.Type != rules.PropertyResource {
		t.Errorf("Property(vim) = %+v, want a resource", p)
	}
}

func qaPFManifestWith(t *testing.T, fv string) string {
	t.Helper()
	m := qaPFBase().manifest()
	delete(m, "format_version")
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if fv == "" {
		return string(b)
	}
	return strings.Replace(string(b), "{", "{\n  \"format_version\": "+fv+",", 1)
}

func qaPFLoadManifest(t *testing.T, body string) (string, error) {
	t.Helper()
	s := qaPFBase()
	delete(s.docs, "ruleset.json")
	s.raw["ruleset.json"] = body
	dir := qaPFWrite(t, qaPFRender(t, s))
	_, err := rules.Load(dir)
	return dir, err
}

// VTT-616
func TestQAPFFormatTwoIsRefusedInExactWords(t *testing.T) {
	retiredShape := `{
  "id": "qa-pf",
  "name": "QA Property Format",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [{"name": "pool_a", "default_max_expr": "10 + @brawn", "thresholds": []}],
  "format_version": "2"
}`
	bodies := map[string]string{
		"format 3 shape":            qaPFManifestWith(t, `"2"`),
		"retired shape, no id last": strings.Replace(retiredShape, `"id": "qa-pf",`, "", 1),
		"retired shape":             retiredShape,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			dir, err := qaPFLoadManifest(t, body)
			r := qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"format_version"})
			if r.field != "format_version" {
				t.Errorf("field = %q, want format_version", r.field)
			}
			if want := `unsupported value "2": ` + qaPFTail; r.msg != want {
				t.Errorf("message = %q\nwant      %q", r.msg, want)
			}
		})
	}
}

// VTT-616
func TestQAPFEveryOtherFormatVersionIsRefused(t *testing.T) {
	cases := []struct{ token, named, notNamed string }{
		{`"1"`, `"1"`, ""}, {`1`, `1`, `"1"`}, {`2`, `2`, `"2"`}, {`3`, `3`, `"3"`},
		{`""`, `""`, ""}, {`"3.0"`, `"3.0"`, ""}, {`" 3"`, `" 3"`, ""}, {`"v3"`, `"v3"`, ""},
		{`"03"`, `"03"`, ""}, {`null`, `null`, ""}, {`"4"`, `"4"`, ""},
	}
	for _, c := range cases {
		t.Run(c.token, func(t *testing.T) {
			dir, err := qaPFLoadManifest(t, qaPFManifestWith(t, c.token))
			r := qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"format_version"})
			if r.field != "format_version" {
				t.Errorf("field = %q, want format_version", r.field)
			}
			head, found := strings.CutSuffix(r.msg, qaPFTail)
			if !found {
				t.Fatalf("message %q does not end with the format 3 text", r.msg)
			}
			if !strings.Contains(head, c.named) {
				t.Errorf("message head %q does not name the value as written, %s", head, c.named)
			}
			if c.notNamed != "" && strings.Contains(head, c.notNamed) {
				t.Errorf("message head %q names a number as the string %s", head, c.notNamed)
			}
		})
	}
	t.Run("control", func(t *testing.T) {
		if _, err := qaPFLoadManifest(t, qaPFManifestWith(t, `"3"`)); err != nil {
			t.Fatalf("format_version \"3\" refused: %v", err)
		}
	})
	t.Run("absent", func(t *testing.T) {
		dir, err := qaPFLoadManifest(t, qaPFManifestWith(t, ""))
		r := qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"format_version"})
		if !strings.HasSuffix(r.msg, qaPFTail) {
			t.Errorf("message %q does not end with the format 3 text", r.msg)
		}
	})
}

// VTT-616
func TestQAPFFormatVersionIsReadBeforeTheRestOfTheFile(t *testing.T) {
	m := qaPFBase().manifest()
	m["format_version"] = "1"
	m["visibility"] = "hidden"
	m["properties"].([]any)[3].(qaPFDoc)["type"] = "attribute"
	delete(m, "id")
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := qaPFLoadManifest(t, string(b))
	r := qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"format_version"})
	if r.msg != `unsupported value "1": `+qaPFTail {
		t.Errorf("message = %q, want the format_version refusal", r.msg)
	}
}

// VTT-627
func TestQAPFUnknownManifestFieldIsRefused(t *testing.T) {
	s := qaPFBase()
	s.manifest()["attributes"] = []any{"brawn"}
	dir, _, err := qaPFLoad(t, s)
	if err == nil || !strings.Contains(err.Error(), filepath.Join(dir, "ruleset.json")) ||
		!strings.Contains(err.Error(), `"attributes"`) {
		t.Fatalf("refusal %v does not name ruleset.json and the field", err)
	}
}

// VTT-626
func TestQAPFKeyShapeIsChecked(t *testing.T) {
	for _, key := range []string{"nerve-2", "2nerve", "caster", "target", "", "a b", "nervé"} {
		t.Run("key="+key, func(t *testing.T) {
			s := qaPFBase()
			s.addProp(qaPFDoc{"key": key, "type": "number", "name": "Echo", "description": "A second entry."})
			dir, _, err := qaPFLoad(t, s)
			qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"key"})
		})
	}
	s := qaPFBase()
	s.addProp(qaPFDoc{"key": "_Nerve9", "type": "number", "name": "Echo", "description": "A second entry."})
	qaPFMustLoad(t, s)
}

// VTT-630
func TestQAPFReferenceScopeFollowsThePosition(t *testing.T) {
	s := qaPFBase()
	s.prop("pool_a")["default_max_expr"] = "10 + @caster.brawn"
	dir, _, err := qaPFLoad(t, s)
	qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"default_max_expr"})
	s = qaPFBase()
	s.contrib("roll-atom", 0)["roll"] = "1d20 + @brawn"
	dir, _, err = qaPFLoad(t, s)
	qaPFExpect(t, dir, err, []string{"atoms/roll-atom.json", "abilities/swing.json"}, []string{"roll"})
}

// VTT-613
func TestQAPFKeyIsCaseSensitive(t *testing.T) {
	s := qaPFBase()
	s.prop("pool_a")["default_max_expr"] = "10 + @Brawn"
	dir, _, err := qaPFLoad(t, s)
	qaPFExpect(t, dir, err, []string{"ruleset.json"}, []string{"default_max_expr"}, "Brawn")
}

// SPEC-018: a display name and a description are unbounded.
func TestQAPFNameAndDescriptionAreUnbounded(t *testing.T) {
	s := qaPFBase()
	s.prop("grit")["name"] = strings.Repeat("N", 70000)
	s.prop("grit")["description"] = strings.Repeat("d", 70000)
	rs := qaPFMustLoad(t, s)
	if got := rs.Property("grit").Name; len(got) != 70000 {
		t.Errorf("name kept %d bytes, want 70000", len(got))
	}
}

func qaPFPad(prefix, fill string, size int) string {
	for len(prefix)+len(fill) <= size {
		prefix += fill
	}
	return prefix + strings.Repeat("x", size-len(prefix))
}

// VTT-624
func TestQAPFEveryBoundedStringAtExactly128BytesLoads(t *testing.T) {
	n := qaPFNames{
		brawn: qaPFPad("brawn", "_", 128), grit: qaPFPad("grit", "_", 128), guard: qaPFPad("guard", "_", 128),
		poolA: qaPFPad("pool_a", "_", 128), vim: qaPFPad("vim", "_", 128), braced: qaPFPad("braced", "_", 128),
		swing: qaPFPad("swing", "-", 128), steady: qaPFPad("steady", "€", 128),
	}
	for _, v := range []string{n.brawn, n.grit, n.guard, n.poolA, n.vim, n.braced, n.swing, n.steady} {
		if len(v) != 128 {
			t.Fatalf("fixture string is %d bytes, want 128", len(v))
		}
	}
	rs := qaPFMustLoad(t, qaPFBuild(n))
	if rs.Property(n.poolA) == nil || rs.Conditions[n.braced] == nil || rs.Compiled[n.steady] == nil {
		t.Error("a 128-byte key, condition id or ability id is missing from the loaded ruleset")
	}
}

// VTT-624
func TestQAPFStringsOver128BytesAreRefused(t *testing.T) {
	steady := func(n *qaPFNames, v string) { n.steady = v }
	braced := func(n *qaPFNames, v string) { n.braced = v }
	cases := []struct {
		name, file, field string
		edit              func(n *qaPFNames, v string)
		value             string
	}{
		{"key 129 ascii", "ruleset.json", "key", func(n *qaPFNames, v string) { n.grit = v }, qaPFPad("grit", "_", 129)},
		{"ability id 129 ascii", "abilities/steady.json", "id", steady, qaPFPad("steady", "-", 129)},
		{"ability id 43 runes", "abilities/steady.json", "id", steady, strings.Repeat("€", 43)},
		{"condition id 129 ascii", "conditions/braced.json", "id", braced, qaPFPad("braced", "-", 129)},
		{"condition id 43 runes", "conditions/braced.json", "id", braced, strings.Repeat("€", 43)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.value) != 129 {
				t.Fatalf("fixture string is %d bytes, want 129", len(c.value))
			}
			n := qaPFDefaultNames()
			c.edit(&n, c.value)
			dir, _, err := qaPFLoad(t, qaPFBuild(n).literalMark(n.braced))
			qaPFExpect(t, dir, err, []string{c.file}, []string{c.field})
		})
	}
}

// VTT-624
func TestQAPFMultibyteIdsAt128BytesLoad(t *testing.T) {
	n := qaPFDefaultNames()
	n.steady = qaPFPad("", "€", 128)
	n.braced = qaPFPad("", "é", 128)
	if len(n.steady) != 128 || len(n.braced) != 128 {
		t.Fatalf("fixture ids are %d and %d bytes", len(n.steady), len(n.braced))
	}
	qaPFMustLoad(t, qaPFBuild(n).literalMark(n.braced))
}

type qaPFProbe struct {
	what    string
	files   func(t *testing.T, size int) map[string]string
	schema  string
	path    []string
	inScope bool
}

func qaPFNamed(edit func(n *qaPFNames, v string), prefix, fill string) func(t *testing.T, size int) map[string]string {
	return func(t *testing.T, size int) map[string]string {
		t.Helper()
		n := qaPFDefaultNames()
		edit(&n, qaPFPad(prefix, fill, size))
		return qaPFRender(t, qaPFBuild(n))
	}
}

func qaPFReplaced(token, fill string) func(t *testing.T, size int) map[string]string {
	return func(t *testing.T, size int) map[string]string {
		t.Helper()
		inner := strings.Trim(token, `"`)
		long := strings.Replace(token, inner, qaPFPad(inner, fill, size), 1)
		files := qaPFRender(t, qaPFBase())
		hits := 0
		for rel, body := range files {
			hits += strings.Count(body, token)
			files[rel] = strings.ReplaceAll(body, token, long)
		}
		if hits == 0 {
			t.Fatalf("token %s is not in the fixture", token)
		}
		return files
	}
}

func qaPFBound(t *testing.T, p qaPFProbe, start int) int {
	t.Helper()
	loads := func(size int) bool {
		t.Helper()
		_, err := rules.Load(qaPFWrite(t, p.files(t, size)))
		return err == nil
	}
	const ceiling = 70000
	if !loads(start) {
		t.Fatalf("%s: the fixture refuses at %d bytes", p.what, start)
	}
	if loads(ceiling) {
		return -1
	}
	lo, hi := start, ceiling
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if loads(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

func qaPFSchemaText(t *testing.T, file string, path []string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("schema", file))
	if err != nil {
		t.Fatal(err)
	}
	var node any
	if err := json.Unmarshal(b, &node); err != nil {
		t.Fatal(err)
	}
	for _, k := range path {
		m, ok := node.(map[string]any)
		if !ok || m[k] == nil {
			t.Fatalf("%s has nothing at %v", file, path)
		}
		node = m[k]
	}
	d, _ := node.(map[string]any)["description"].(string)
	return d
}

// VTT-625
func TestQAPFSchemasStateTheBoundsLoadEnforces(t *testing.T) {
	probes := []qaPFProbe{
		{"property key", qaPFNamed(func(n *qaPFNames, v string) { n.grit = v }, "grit", "_"),
			"ruleset.schema.json", []string{"properties", "properties", "items", "properties", "key"}, true},
		{"ability id", qaPFNamed(func(n *qaPFNames, v string) { n.steady = v }, "steady", "-"),
			"ability.schema.json", []string{"properties", "id"}, true},
		{"condition id", qaPFNamed(func(n *qaPFNames, v string) { n.braced = v }, "braced", "_"),
			"condition.schema.json", []string{"properties", "id"}, true},
		{"branch label", qaPFReplaced(`"hit"`, "x"),
			"atom.schema.json", []string{"$defs", "resolutionContribution", "properties", "branches"}, true},
		{"atom id", qaPFReplaced(`"roll-atom"`, "x"), "atom.schema.json", []string{"properties", "id"}, true},
		{"ruleset id", qaPFReplaced(`"qa-pf"`, "x"), "ruleset.schema.json", []string{"properties", "id"}, true},
		{"resolution key", qaPFReplaced(`"clash"`, "x"),
			"atom.schema.json", []string{"$defs", "resolutionContribution", "properties", "key"}, false},
		{"param name", qaPFReplaced(`attack_stat`, "x"),
			"atom.schema.json", []string{"$defs", "param", "properties", "name"}, false},
		{"ability name", qaPFReplaced(`"Swing"`, "x"), "ability.schema.json", []string{"properties", "name"}, false},
		{"condition name", qaPFReplaced(`"Braced"`, "x"), "condition.schema.json", []string{"properties", "name"}, false},
		{"ruleset name", qaPFReplaced(`"QA Property Format"`, "x"), "ruleset.schema.json", []string{"properties", "name"}, false},
	}
	for _, p := range probes {
		t.Run(p.what, func(t *testing.T) {
			bound := qaPFBound(t, p, 20)
			text := qaPFSchemaText(t, p.schema, p.path)
			states := regexp.MustCompile(`(\d+) bytes`).FindStringSubmatch(text)
			t.Logf("%s: Load bound %d (-1 = none below 70000); schema says %q", p.what, bound, states)
			if !p.inScope {
				return
			}
			switch {
			case bound >= 0 && (states == nil || states[1] != fmt.Sprint(bound)):
				t.Errorf("Load bounds the %s at %d bytes; the schema description %q does not state it", p.what, bound, text)
			case bound < 0 && states != nil:
				t.Errorf("the schema states %s bytes for the %s; Load enforces no bound", states[1], p.what)
			}
		})
	}
}
