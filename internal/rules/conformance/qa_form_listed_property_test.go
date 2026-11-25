package conformance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
	"github.com/PatrikLager/vtt-platform/internal/rules/conformance"
)

type qaivForm struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	Desc       string   `json:"description"`
	Properties []string `json:"properties"`
}

const qaivSmoke = `failed to resolve against the generated fixture actor: `

type qaivOpts struct {
	forms     []qaivForm
	noGoldens bool
	badGolden bool
}

func qaivWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// Keep jab reading quebec from its caster and romeo from its target, and
// poke reading str and def: the tests name which ability reads which key.
func qaivDir(t *testing.T, o qaivOpts) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rs")
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("testdata", "minimal"))); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	props, _ := m["properties"].([]any)
	for _, k := range []string{"quebec", "romeo", "sierra"} {
		props = append(props, map[string]any{
			"key": k, "type": "number", "name": "N " + k,
			"description": "An invented number.",
		})
	}
	m["properties"] = props
	if o.forms != nil {
		m["forms"] = o.forms
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	qaivWrite(t, path, string(out))
	qaivWrite(t, filepath.Join(dir, "atoms", "qaiv-roll.json"), `{"id": "qaiv-roll",
"params": [{"name": "a", "kind": "number"}, {"name": "d", "kind": "number"}],
"provides": ["strike"], "consumes": ["delivery"],
"contributes": [{"kind": "resolution", "key": "strike",
"roll": "1d20 + @caster.{a}", "vs": "@target.{d}", "branches": ["hit", "miss"]}]}`)
	qaivWrite(t, filepath.Join(dir, "abilities", "jab.json"), `{"id": "jab",
"name": "Victor", "usage": "at_will", "compose": [
{"atom": "melee-delivery", "bind": {"reach": 1}},
{"atom": "qaiv-roll", "bind": {"a": "quebec", "d": "romeo"}},
{"atom": "strike-damage", "bind": {}}]}`)
	qaivWrite(t, filepath.Join(dir, "goldens", "jab-out-of-range.json"), `{
"name": "jab-out-of-range", "ability_id": "jab",
"actors": [{"id": "caster", "x": 0, "y": 0, "attributes": {"quebec": 3}},
{"id": "target", "x": 5, "y": 0, "attributes": {"romeo": 9}}],
"actor_id": "caster", "target_ids": ["target"], "want_error": "out of range"}`)
	if o.badGolden {
		qaivWrite(t, filepath.Join(dir, "goldens", "jab-wrong.json"), `{
"name": "jab-wrong", "ability_id": "jab",
"actors": [{"id": "caster", "x": 0, "y": 0, "attributes": {"quebec": 3}},
{"id": "target", "x": 5, "y": 0, "attributes": {"romeo": 9}}],
"actor_id": "caster", "target_ids": ["target"], "want_error": "qaiv-never-said"}`)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for id, cp := range rs.Compiled {
		b, err := conformance.DumpCompiledPower(cp)
		if err != nil {
			t.Fatalf("dump %s: %v", id, err)
		}
		qaivWrite(t, filepath.Join(dir, "goldens", "compiled", id+".json"), string(b))
	}
	if o.noGoldens {
		if err := os.RemoveAll(filepath.Join(dir, "goldens")); err != nil {
			t.Fatalf("remove goldens: %v", err)
		}
	}
	return dir
}

func qaivOneForm(keys ...string) []qaivForm {
	return []qaivForm{{Key: "zulu", Name: "Lima", Desc: "An invented form.", Properties: keys}}
}

func qaivSmokeFault(t *testing.T, dir, ability, key string) string {
	t.Helper()
	err := conformance.Run(dir)
	if err == nil {
		t.Fatal("Run passed, want a refusal at the smoke pass")
	}
	msg := err.Error()
	want := `conformance: ` + dir + `: ability "` + ability + `" ` + qaivSmoke
	if !strings.HasPrefix(msg, want) {
		t.Fatalf("Run: %q, want it to open %q", msg, want)
	}
	if !strings.Contains(strings.TrimPrefix(msg, want), key) {
		t.Fatalf("Run: %q does not name the key %q", msg, key)
	}
	return msg
}

// VTT-1066
func TestQAIVRunPassesTheFixtureWithNoForm(t *testing.T) {
	if err := conformance.Run(qaivDir(t, qaivOpts{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// VTT-1066
func TestQAIVAnAbilityReadingAListedKeyFromItsCasterFails(t *testing.T) {
	dir := qaivDir(t, qaivOpts{forms: qaivOneForm("quebec")})
	t.Logf("Run: %s", qaivSmokeFault(t, dir, "jab", "quebec"))
}

// VTT-1066
func TestQAIVAnAbilityReadingAListedKeyFromItsTargetFails(t *testing.T) {
	qaivSmokeFault(t, qaivDir(t, qaivOpts{forms: qaivOneForm("romeo")}), "jab", "romeo")
}

// VTT-1066
func TestQAIVTheAbilityThatReadsTheKeyIsNamed(t *testing.T) {
	msg := qaivSmokeFault(t, qaivDir(t, qaivOpts{forms: qaivOneForm("str")}), "poke", "str")
	if strings.Contains(msg, `"jab"`) {
		t.Fatalf("Run: %q names jab, which does not read str", msg)
	}
}

// VTT-1066
func TestQAIVAKeyASecondFormListsFails(t *testing.T) {
	forms := append(qaivOneForm("sierra"), qaivForm{
		Key: "yankee", Name: "Kilo", Desc: "Another form.", Properties: []string{"quebec"},
	})
	qaivSmokeFault(t, qaivDir(t, qaivOpts{forms: forms}), "jab", "quebec")
}

// VTT-1066
func TestQAIVAFormListingAKeyNoAbilityReadsPasses(t *testing.T) {
	dir := qaivDir(t, qaivOpts{forms: qaivOneForm("sierra")})
	rs, err := rules.Load(dir)
	if err != nil || len(rs.Forms) != 1 {
		t.Fatalf("fixture: %v %+v", err, rs)
	}
	if err := conformance.Run(dir); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// VTT-1066
func TestQAIVTheSmokePassComesBeforeTheGoldens(t *testing.T) {
	listed := qaivOneForm("quebec")
	full := qaivSmokeFault(t, qaivDir(t, qaivOpts{forms: listed}), "jab", "quebec")
	bare := qaivSmokeFault(t, qaivDir(t, qaivOpts{forms: listed, noGoldens: true}), "jab", "quebec")
	bad := qaivSmokeFault(t, qaivDir(t, qaivOpts{forms: listed, badGolden: true}), "jab", "quebec")
	_, fullCause, _ := strings.Cut(full, qaivSmoke)
	_, bareCause, _ := strings.Cut(bare, qaivSmoke)
	_, badCause, _ := strings.Cut(bad, qaivSmoke)
	if fullCause != bareCause || fullCause != badCause {
		t.Fatalf("causes differ: %q, %q, %q", fullCause, bareCause, badCause)
	}
	later := []struct {
		o    qaivOpts
		want string
	}{
		{qaivOpts{noGoldens: true}, "has no golden scenario"},
		{qaivOpts{badGolden: true}, filepath.Join("goldens", "jab-wrong.json")},
	}
	for _, c := range later {
		err := conformance.Run(qaivDir(t, c.o))
		if err == nil || strings.Contains(err.Error(), qaivSmoke) ||
			!strings.Contains(err.Error(), c.want) {
			t.Fatalf("a later stage's fault read as %v, want %q", err, c.want)
		}
	}
}

// VTT-1066
func TestQAIVEveryShippedRulesetPassesRun(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "..", "rulesets", "*", "ruleset.json"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no shipped ruleset found: %v", err)
	}
	forms := 0
	for _, m := range dirs {
		dir := filepath.Dir(m)
		if err := conformance.Run(dir); err != nil {
			t.Fatalf("Run %s: %v", dir, err)
		}
		rs, err := rules.Load(dir)
		if err != nil {
			t.Fatalf("load %s: %v", dir, err)
		}
		forms += len(rs.Forms)
	}
	if forms == 0 {
		t.Fatal("no shipped ruleset declares a form")
	}
}

// VTT-1066
func TestQAIVAListedKeyReadInAnOutcomeFails(t *testing.T) {
	keys := map[string]string{"hit": `"strike"`, "miss": `"strike"`, "always": "null"}
	for _, branch := range []string{"hit", "miss", "always"} {
		t.Run(branch, func(t *testing.T) {
			dir := qaivDir(t, qaivOpts{forms: qaivOneForm("quebec")})
			qaivWrite(t, filepath.Join(dir, "abilities", "jab.json"), `{"id": "jab",
"name": "Victor", "usage": "at_will", "compose": [
{"atom": "melee-delivery", "bind": {"reach": 1}},
{"atom": "strike-roll", "bind": {"attack_stat": "str"}},
{"atom": "qaiv-read", "bind": {}}]}`)
			qaivWrite(t, filepath.Join(dir, "atoms", "qaiv-read.json"), `{"id": "qaiv-read",
"params": [], "provides": [], "consumes": ["strike"],
"contributes": [{"kind": "outcome", "key": `+keys[branch]+`, "branch": "`+branch+`",
"effects": [{"resource_change": {"resource": "pool", "delta_expr": "@caster.quebec"}}]}]}`)
			qaivWrite(t, filepath.Join(dir, "goldens", "jab-out-of-range.json"), `{
"name": "jab-out-of-range", "ability_id": "jab",
"actors": [{"id": "caster", "x": 0, "y": 0, "attributes": {"str": 3}},
{"id": "target", "x": 5, "y": 0, "attributes": {"def": 9}}],
"actor_id": "caster", "target_ids": ["target"], "want_error": "out of range"}`)
			rs, err := rules.Load(dir)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			b, err := conformance.DumpCompiledPower(rs.Compiled["jab"])
			if err != nil {
				t.Fatalf("dump: %v", err)
			}
			qaivWrite(t, filepath.Join(dir, "goldens", "compiled", "jab.json"), string(b))
			qaivSmokeFault(t, dir, "jab", "quebec")
		})
	}
}

func qaivRefs(cp *rules.CompiledPower) []string {
	var exprs []*rules.Expr
	if r := cp.Resolution; r != nil {
		exprs = append(exprs, r.Roll, r.Vs)
	}
	outs := append(append([]rules.Outcome{}, cp.Effects...), cp.BranchOutcomes[0]...)
	for _, o := range append(outs, cp.BranchOutcomes[1]...) {
		if o.ResourceChange != nil {
			exprs = append(exprs, o.ResourceChange.DeltaExpr)
		}
	}
	var keys []string
	for _, e := range exprs {
		if e != nil {
			attrs, _ := e.Refs()
			keys = append(keys, attrs...)
		}
	}
	return keys
}

// VTT-1066
func TestQAIVNoShippedAbilityReadsAListedKeyOnAnyBranch(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "..", "rulesets", "*", "ruleset.json"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no shipped ruleset found: %v", err)
	}
	read := 0
	for _, m := range dirs {
		rs, err := rules.Load(filepath.Dir(m))
		if err != nil {
			t.Fatalf("load %s: %v", m, err)
		}
		listed := map[string]string{}
		for _, f := range rs.Forms {
			for _, k := range f.Properties {
				listed[k] = f.Key
			}
		}
		for id, cp := range rs.Compiled {
			for _, k := range qaivRefs(cp) {
				read++
				if f, ok := listed[k]; ok {
					t.Errorf("%s: ability %q reads %q, which form %q lists", rs.ID, id, k, f)
				}
			}
		}
	}
	if read == 0 {
		t.Fatal("no shipped ability reads any attribute")
	}
}
