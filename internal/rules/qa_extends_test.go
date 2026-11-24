package rules_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaexBase = "../../rulesets/tavern-brawl"

type qaexVals = map[string]int

// qaexCat maps a catalogue file's base name, without .json, to its document.
type qaexCat map[string]any

func qaexItem(id, form string, values qaexVals) map[string]any {
	m := map[string]any{
		"id":          id,
		"name":        "Thing " + strings.TrimPrefix(id, "ent-"),
		"description": "A thing at the table.",
		"form":        form,
	}
	if values != nil {
		m["values"] = values
	}
	return m
}

func qaexExt(doc map[string]any, extends string) map[string]any {
	doc["extends"] = extends
	return doc
}

func qaexForms() []map[string]any {
	return []map[string]any{
		{
			"key": "kit", "name": "Kit", "description": "A kind of thing.",
			"properties": []string{"brawn", "grit", "footing"},
		},
		{"key": "gear", "name": "Gear", "description": "Another kind."},
		{"key": "ware", "name": "Ware", "description": "A third kind."},
	}
}

func qaexCopy(t *testing.T, src, dst string) {
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
		t.Fatalf("copy %s: %v", src, err)
	}
}

func qaexBaseDir(t *testing.T, withForms bool) string {
	t.Helper()
	dir := t.TempDir()
	qaexCopy(t, qaexBase, dir)
	if !withForms {
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
	forms, err := json.Marshal(qaexForms())
	if err != nil {
		t.Fatal(err)
	}
	m["forms"] = forms
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func qaexBuild(t *testing.T, cat qaexCat) string {
	t.Helper()
	dir := qaexBaseDir(t, true)
	cd := filepath.Join(dir, "catalogue")
	if err := os.Mkdir(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, doc := range cat {
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cd, name+".json"), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Warning: a cycle the loader misses must fail here, never hang the suite.
func qaexLoad(t *testing.T, dir string) (*rules.Ruleset, error) {
	t.Helper()
	type result struct {
		rs  *rules.Ruleset
		err error
	}
	ch := make(chan result, 1)
	go func() {
		rs, err := rules.Load(dir)
		ch <- result{rs, err}
	}()
	select {
	case r := <-ch:
		return r.rs, r.err
	case <-time.After(5 * time.Second):
		t.Fatalf("rules.Load(%s) did not return within 5s", dir)
		return nil, nil
	}
}

func qaexMustLoad(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := qaexLoad(t, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return rs
}

func qaexRefused(t *testing.T, dir string, wants ...string) string {
	t.Helper()
	_, err := qaexLoad(t, dir)
	if err == nil {
		t.Fatalf("Load accepted; want a refusal naming %q", wants)
	}
	msg := err.Error()
	for _, w := range wants {
		if !strings.Contains(msg, w) {
			t.Errorf("refusal %q does not name %q", msg, w)
		}
	}
	return msg
}

func qaexValues(t *testing.T, rs *rules.Ruleset, id string, want qaexVals) {
	t.Helper()
	e := rs.Catalogue[id]
	if e == nil {
		t.Fatalf("Catalogue holds no %q", id)
	}
	if len(e.Values) != len(want) {
		t.Errorf("%s: Values = %v, want exactly %v", id, e.Values, want)
		return
	}
	for k, v := range want {
		got, ok := e.Values[k]
		if !ok || got != v {
			t.Errorf("%s: Values = %v, want exactly %v", id, e.Values, want)
			return
		}
	}
}

func qaexFile(name string) string {
	return filepath.Join("catalogue", name+".json")
}

func qaexPerms(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	var out [][]int
	for _, p := range qaexPerms(n - 1) {
		for i := 0; i <= len(p); i++ {
			q := make([]int, 0, n)
			q = append(q, p[:i]...)
			q = append(q, n-1)
			q = append(q, p[i:]...)
			out = append(out, q)
		}
	}
	return out
}

func TestQAEXNoCatalogueLoadsWithNone(t *testing.T) {
	t.Run("without forms", func(t *testing.T) {
		rs := qaexMustLoad(t, qaexBaseDir(t, false))
		// VTT-997
		if len(rs.Catalogue) != 0 {
			t.Errorf("Catalogue = %v, want none", rs.Catalogue)
		}
	})
	t.Run("with forms", func(t *testing.T) {
		rs := qaexMustLoad(t, qaexBaseDir(t, true))
		// VTT-997
		if len(rs.Catalogue) != 0 {
			t.Errorf("Catalogue = %v, want none", rs.Catalogue)
		}
	})
	t.Run("empty catalogue directory", func(t *testing.T) {
		rs := qaexMustLoad(t, qaexBuild(t, qaexCat{}))
		// VTT-997
		if len(rs.Catalogue) != 0 {
			t.Errorf("Catalogue = %v, want none", rs.Catalogue)
		}
	})
}

func TestQAEXEveryEntryHeldByItsIDAsWritten(t *testing.T) {
	first := qaexItem("ent-first", "kit", qaexVals{"brawn": 3, "grit": -2})
	first["name"] = "Ölstuga — the Long, Low One"
	first["description"] = "Two lines:\nboth kept, \"quoted\" too."
	second := qaexItem("ent-second", "gear", nil)
	third := qaexItem("ent-third", "kit", qaexVals{"footing": 2147483647})
	dir := qaexBuild(t, qaexCat{"zz-one": first, "aa-two": second, "mm-three": third})
	rs := qaexMustLoad(t, dir)

	// VTT-997
	if len(rs.Catalogue) != 3 {
		t.Fatalf("Catalogue holds %d entries, want 3: %v", len(rs.Catalogue), rs.Catalogue)
	}
	for _, file := range []string{"zz-one", "aa-two", "mm-three"} {
		// VTT-997
		if rs.Catalogue[file] != nil {
			t.Errorf("Catalogue holds an entry by its file name %q", file)
		}
	}
	for id, doc := range map[string]map[string]any{
		"ent-first": first, "ent-second": second, "ent-third": third,
	} {
		e := rs.Catalogue[id]
		if e == nil {
			t.Errorf("Catalogue holds no %q", id)
			continue
		}
		// VTT-997
		if e.ID != id || e.Name != doc["name"] || e.Description != doc["description"] ||
			e.Form != doc["form"] {
			t.Errorf("%s = %+v, want id, name, description and form of %v", id, e, doc)
		}
	}
	// VTT-997
	qaexValues(t, rs, "ent-first", qaexVals{"brawn": 3, "grit": -2})
	// VTT-997
	qaexValues(t, rs, "ent-second", qaexVals{})
	// VTT-997
	qaexValues(t, rs, "ent-third", qaexVals{"footing": 2147483647})
}

func TestQAEXNameKeptWithSurroundingSpaces(t *testing.T) {
	doc := qaexItem("ent-spaced", "kit", nil)
	doc["name"] = "  Spaced  "
	doc["description"] = " Around. "
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{"spaced": doc}))
	e := rs.Catalogue["ent-spaced"]
	// VTT-997
	if e == nil || e.Name != "  Spaced  " || e.Description != " Around. " {
		t.Errorf("entry = %+v, want name and description as written", e)
	}
}

func TestQAEXNoValueForAPropertyNotCarried(t *testing.T) {
	bare := qaexItem("ent-bare", "kit", nil)
	empty := qaexItem("ent-empty", "kit", qaexVals{})
	some := qaexItem("ent-some", "kit", qaexVals{"grit": 4})
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{"p": bare, "q": empty, "r": some}))
	// VTT-1000
	qaexValues(t, rs, "ent-bare", qaexVals{})
	// VTT-1000
	qaexValues(t, rs, "ent-empty", qaexVals{})
	// VTT-1000
	qaexValues(t, rs, "ent-some", qaexVals{"grit": 4})
}

func TestQAEXChildHoldsEveryParentValueItDoesNotSet(t *testing.T) {
	parent := qaexItem("ent-parent", "kit", qaexVals{"brawn": 1, "grit": 2, "footing": 3})
	child := qaexExt(qaexItem("ent-child", "kit", qaexVals{"grit": 20}), "ent-parent")
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{"p": parent, "c": child}))
	// VTT-1002
	qaexValues(t, rs, "ent-child", qaexVals{"brawn": 1, "grit": 20, "footing": 3})
	// VTT-1002
	qaexValues(t, rs, "ent-parent", qaexVals{"brawn": 1, "grit": 2, "footing": 3})
}

func TestQAEXOwnZeroAndNegativeOverParent(t *testing.T) {
	parent := qaexItem("ent-parent", "kit", qaexVals{"brawn": 7, "grit": 5})
	child := qaexExt(qaexItem("ent-child", "kit", qaexVals{"brawn": 0, "grit": -5}),
		"ent-parent")
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{"p": parent, "c": child}))
	// VTT-1002
	qaexValues(t, rs, "ent-child", qaexVals{"brawn": 0, "grit": -5})
}

func TestQAEXChildOfOneHoldingNoValueHoldsNone(t *testing.T) {
	parent := qaexItem("ent-parent", "kit", qaexVals{"brawn": 1})
	child := qaexExt(qaexItem("ent-child", "kit", qaexVals{"grit": 2}), "ent-parent")
	bareParent := qaexItem("ent-bare-parent", "kit", nil)
	bareChild := qaexExt(qaexItem("ent-bare-child", "kit", nil), "ent-bare-parent")
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{
		"p": parent, "c": child, "bp": bareParent, "bc": bareChild,
	}))
	// VTT-1000
	qaexValues(t, rs, "ent-child", qaexVals{"brawn": 1, "grit": 2})
	// VTT-1000
	qaexValues(t, rs, "ent-bare-child", qaexVals{})
}

func TestQAEXChainOfThree(t *testing.T) {
	root := qaexItem("ent-root", "kit", qaexVals{"brawn": 1, "grit": 2, "footing": 3})
	mid := qaexExt(qaexItem("ent-mid", "kit", qaexVals{"grit": 20}), "ent-root")
	leaf := qaexExt(qaexItem("ent-leaf", "kit", qaexVals{"footing": 300}), "ent-mid")
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{"x": root, "y": mid, "z": leaf}))
	// VTT-1002
	qaexValues(t, rs, "ent-leaf", qaexVals{"brawn": 1, "grit": 20, "footing": 300})
	// VTT-1002
	qaexValues(t, rs, "ent-mid", qaexVals{"brawn": 1, "grit": 20, "footing": 3})
	// VTT-1002
	qaexValues(t, rs, "ent-root", qaexVals{"brawn": 1, "grit": 2, "footing": 3})
}

func TestQAEXChainOfFour(t *testing.T) {
	root := qaexItem("ent-root", "kit", qaexVals{"brawn": 1, "grit": 2, "footing": 3})
	low := qaexExt(qaexItem("ent-low", "kit", qaexVals{"grit": 20, "footing": 30}),
		"ent-root")
	mid := qaexExt(qaexItem("ent-mid", "kit", qaexVals{"footing": 300}), "ent-low")
	leaf := qaexExt(qaexItem("ent-leaf", "kit", nil), "ent-mid")
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{
		"w": root, "x": low, "y": mid, "z": leaf,
	}))
	// VTT-1002
	qaexValues(t, rs, "ent-leaf", qaexVals{"brawn": 1, "grit": 20, "footing": 300})
	// VTT-1002
	qaexValues(t, rs, "ent-mid", qaexVals{"brawn": 1, "grit": 20, "footing": 300})
	// VTT-1002
	qaexValues(t, rs, "ent-low", qaexVals{"brawn": 1, "grit": 20, "footing": 30})
	// VTT-1002
	qaexValues(t, rs, "ent-root", qaexVals{"brawn": 1, "grit": 2, "footing": 3})
}

func TestQAEXDeepChain(t *testing.T) {
	const n = 40
	cat := qaexCat{}
	id := func(i int) string { return "ent-k" + string(rune('a'+i/26)) + string(rune('a'+i%26)) }
	for i := 0; i < n; i++ {
		var v qaexVals
		switch i {
		case 0:
			v = qaexVals{"brawn": 11, "grit": 22}
		case 20:
			v = qaexVals{"grit": 99}
		}
		doc := qaexItem(id(i), "kit", v)
		if i > 0 {
			qaexExt(doc, id(i-1))
		}
		cat["f"+id(n-1-i)] = doc
	}
	rs := qaexMustLoad(t, qaexBuild(t, cat))
	for i := 0; i < n; i++ {
		want := qaexVals{"brawn": 11, "grit": 22}
		if i >= 20 {
			want["grit"] = 99
		}
		// VTT-1002
		qaexValues(t, rs, id(i), want)
	}
}

func TestQAEXSiblingsShareAnAncestorWithoutACycle(t *testing.T) {
	root := qaexItem("ent-root", "kit", qaexVals{"brawn": 1, "grit": 2})
	s1 := qaexExt(qaexItem("ent-s1", "kit", qaexVals{"brawn": 10}), "ent-root")
	s2 := qaexExt(qaexItem("ent-s2", "kit", qaexVals{"grit": 20}), "ent-root")
	g1 := qaexExt(qaexItem("ent-g1", "kit", qaexVals{"footing": 5}), "ent-s1")
	g2 := qaexExt(qaexItem("ent-g2", "kit", nil), "ent-s2")
	rs := qaexMustLoad(t, qaexBuild(t, qaexCat{
		"e": root, "d": s1, "c": s2, "b": g1, "a": g2,
	}))
	// VTT-1002
	qaexValues(t, rs, "ent-root", qaexVals{"brawn": 1, "grit": 2})
	// VTT-1002
	qaexValues(t, rs, "ent-s1", qaexVals{"brawn": 10, "grit": 2})
	// VTT-1002
	qaexValues(t, rs, "ent-s2", qaexVals{"brawn": 1, "grit": 20})
	// VTT-1002
	qaexValues(t, rs, "ent-g1", qaexVals{"brawn": 10, "grit": 2, "footing": 5})
	// VTT-1002 VTT-997
	qaexValues(t, rs, "ent-g2", qaexVals{"brawn": 1, "grit": 20})
}

func TestQAEXIndependentOfFileNamesAndReadOrder(t *testing.T) {
	ids := []string{"ent-root", "ent-low", "ent-mid", "ent-leaf"}
	docs := func() []map[string]any {
		return []map[string]any{
			qaexItem("ent-root", "kit", qaexVals{"brawn": 1, "grit": 2, "footing": 3}),
			qaexExt(qaexItem("ent-low", "kit", qaexVals{"grit": 20, "footing": 30}),
				"ent-root"),
			qaexExt(qaexItem("ent-mid", "kit", qaexVals{"footing": 300}), "ent-low"),
			qaexExt(qaexItem("ent-leaf", "kit", qaexVals{"brawn": -1}), "ent-mid"),
		}
	}
	want := map[string]qaexVals{
		"ent-root": {"brawn": 1, "grit": 2, "footing": 3},
		"ent-low":  {"brawn": 1, "grit": 20, "footing": 30},
		"ent-mid":  {"brawn": 1, "grit": 20, "footing": 300},
		"ent-leaf": {"brawn": -1, "grit": 20, "footing": 300},
	}
	names := []string{"a", "b", "c", "d"}
	for _, perm := range qaexPerms(len(ids)) {
		ds := docs()
		cat := qaexCat{}
		label := ""
		for i, p := range perm {
			cat[names[p]] = ds[i]
			label += names[p]
		}
		t.Run(label, func(t *testing.T) {
			rs := qaexMustLoad(t, qaexBuild(t, cat))
			for _, id := range ids {
				// VTT-1002
				qaexValues(t, rs, id, want[id])
			}
		})
	}
}

func TestQAEXRefusesAnUnresolvableExtends(t *testing.T) {
	cases := []struct {
		name, childForm, extends string
	}{
		{"written empty", "kit", ""},
		{"absent id", "kit", "ent-nowhere"},
		{"ability id", "kit", "fists"},
		{"condition id", "kit", "dazed-by-ale"},
		{"atom id", "kit", "brawl-reach"},
		{"form key", "kit", "kit"},
		{"file name of an entry", "kit", "pfile"},
		{"padded id", "kit", " ent-parent"},
		{"case variant", "kit", "ENT-PARENT"},
		{"another form", "gear", "ent-parent"},
		{"another form, reversed", "kit", "ent-other"},
		{"another form, same list", "gear", "ent-ware"},
	}
	build := func(t *testing.T, childForm, extends string) string {
		t.Helper()
		var v qaexVals
		if childForm == "kit" {
			v = qaexVals{"grit": 1}
		}
		return qaexBuild(t, qaexCat{
			"pfile": qaexItem("ent-parent", "kit", qaexVals{"brawn": 2}),
			"ofile": qaexItem("ent-other", "gear", nil),
			"wfile": qaexItem("ent-ware", "ware", nil),
			"cfile": qaexExt(qaexItem("ent-child", childForm, v), extends),
		})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := qaexRefused(t, build(t, c.childForm, c.extends))
			// VTT-1003
			for _, w := range []string{qaexFile("cfile"), "extends"} {
				if !strings.Contains(msg, w) {
					t.Errorf("refusal %q does not name %q", msg, w)
				}
			}
			t.Logf("refusal: %s", msg)
		})
	}
	t.Run("control", func(t *testing.T) {
		rs := qaexMustLoad(t, build(t, "kit", "ent-parent"))
		// VTT-1002
		qaexValues(t, rs, "ent-child", qaexVals{"brawn": 2, "grit": 1})
	})
}

func TestQAEXRefusesACycle(t *testing.T) {
	cases := []struct {
		name  string
		links [][2]string
		cycle []string
	}{
		{"of one", [][2]string{{"ent-north", "ent-north"}}, []string{"ent-north"}},
		{"of two", [][2]string{
			{"ent-north", "ent-south"}, {"ent-south", "ent-north"},
		}, []string{"ent-north", "ent-south"}},
		{"of three", [][2]string{
			{"ent-north", "ent-south"}, {"ent-south", "ent-east"},
			{"ent-east", "ent-north"},
		}, []string{"ent-north", "ent-south", "ent-east"}},
		{"of five", [][2]string{
			{"ent-north", "ent-south"}, {"ent-south", "ent-east"},
			{"ent-east", "ent-west"}, {"ent-west", "ent-centre"},
			{"ent-centre", "ent-north"},
		}, []string{"ent-north", "ent-south", "ent-east", "ent-west", "ent-centre"}},
		{"reached from outside", [][2]string{
			{"ent-far", "ent-out"}, {"ent-out", "ent-north"},
			{"ent-north", "ent-south"}, {"ent-south", "ent-east"},
			{"ent-east", "ent-north"},
		}, []string{"ent-north", "ent-south", "ent-east"}},
		{"beside sound entries", [][2]string{
			{"ent-up", ""}, {"ent-down", "ent-up"},
			{"ent-north", "ent-south"}, {"ent-south", "ent-north"},
		}, []string{"ent-north", "ent-south"}},
	}
	for _, c := range cases {
		n := len(c.links)
		for rot := 0; rot < 2*n; rot++ {
			cat := qaexCat{}
			files := map[string]string{}
			for i, l := range c.links {
				j := (i + rot) % n
				if rot >= n {
					j = n - 1 - (i+rot)%n
				}
				file := "f" + string(rune('a'+j))
				doc := qaexItem(l[0], "kit", qaexVals{"brawn": i})
				if l[1] != "" {
					qaexExt(doc, l[1])
				}
				cat[file] = doc
				files[l[0]] = file
			}
			t.Run(c.name, func(t *testing.T) {
				msg := qaexRefused(t, qaexBuild(t, cat))
				for _, id := range c.cycle {
					// VTT-1003
					if !strings.Contains(msg, id) {
						t.Errorf("refusal %q does not name %q", msg, id)
					}
				}
				// VTT-1003
				if !strings.Contains(msg, "extends") {
					t.Errorf("refusal %q does not name the field extends", msg)
				}
				named := false
				for _, id := range c.cycle {
					named = named || strings.Contains(msg, qaexFile(files[id]))
				}
				// VTT-1003
				if !named {
					t.Errorf("refusal %q names no file of the cycle", msg)
				}
				t.Logf("refusal: %s", msg)
			})
		}
	}
}

// VTT-1001
func TestQAEXRulingOneIDNamespace(t *testing.T) {
	for _, c := range []struct{ name, id string }{
		{"ability id", "fists"},
		{"condition id", "dazed-by-ale"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := qaexBuild(t, qaexCat{"one": qaexItem(c.id, "kit", nil)})
			qaexRefused(t, dir, c.id)
		})
	}
	t.Run("two entries", func(t *testing.T) {
		dir := qaexBuild(t, qaexCat{
			"one": qaexItem("ent-twin", "kit", qaexVals{"brawn": 1}),
			"two": qaexItem("ent-twin", "kit", qaexVals{"brawn": 2}),
		})
		qaexRefused(t, dir, "ent-twin")
	})
	t.Run("control", func(t *testing.T) {
		qaexMustLoad(t, qaexBuild(t, qaexCat{"one": qaexItem("ent-solo", "kit", nil)}))
	})
}

// VTT-999
func TestQAEXRulingValuesAreNumbersOnly(t *testing.T) {
	for _, c := range []struct {
		name  string
		value any
	}{
		{"string", "3"}, {"bool", true}, {"null", nil},
		{"object", map[string]any{}}, {"list", []int{3}},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := qaexItem("ent-odd", "kit", nil)
			doc["values"] = map[string]any{"brawn": c.value}
			qaexRefused(t, qaexBuild(t, qaexCat{"odd": doc}), qaexFile("odd"))
		})
	}
	t.Run("control", func(t *testing.T) {
		doc := qaexItem("ent-odd", "kit", nil)
		doc["values"] = map[string]any{"brawn": 3}
		qaexMustLoad(t, qaexBuild(t, qaexCat{"odd": doc}))
	})
}

// VTT-997
func TestQAEXRulingOneEntryPerFile(t *testing.T) {
	a := qaexItem("ent-a", "kit", nil)
	b := qaexItem("ent-b", "kit", nil)
	t.Run("list in one file", func(t *testing.T) {
		qaexRefused(t, qaexBuild(t, qaexCat{"both": []any{a, b}}), qaexFile("both"))
	})
	t.Run("control", func(t *testing.T) {
		rs := qaexMustLoad(t, qaexBuild(t, qaexCat{"one": a, "two": b}))
		if len(rs.Catalogue) != 2 {
			t.Errorf("Catalogue = %v, want two entries", rs.Catalogue)
		}
	})
}
