package rules_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func qaCopyRuleset(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("..", "..", "rulesets", name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	abilities, err := filepath.Glob(filepath.Join(dst, "abilities", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range abilities {
		qaEdit(t, a, func(m map[string]any) { delete(m, "description") })
	}
	qaEdit(t, filepath.Join(dst, "ruleset.json"), func(m map[string]any) { delete(m, "free_usage_name") })
	return dst
}

func qaEdit(t *testing.T, path string, f func(m map[string]any)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func qaAbility(dir, id string) string {
	return filepath.Join(dir, "abilities", id+".json")
}

func qaMustLoad(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return rs
}

func qaMustRefuse(t *testing.T, dir string, want ...string) {
	t.Helper()
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("expected refusal, loaded")
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("error %q lacks %q", err, w)
		}
	}
}

// VTT-847
func TestQADescKeptOnItsOwnAbilityOnly(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	qaEdit(t, qaAbility(dir, "fists"), func(m map[string]any) {
		m["description"] = "A plain wooden plank, swung wide."
	})
	rs := qaMustLoad(t, dir)
	if got := rs.Compiled["fists"].Description; got != "A plain wooden plank, swung wide." {
		t.Fatalf("fists: %q", got)
	}
	for id, p := range rs.Compiled {
		if id != "fists" && p.Description != "" {
			t.Fatalf("%s leaked description %q", id, p.Description)
		}
	}
}

// VTT-847
func TestQADescNoneWhenAbsentOrNull(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	rs := qaMustLoad(t, dir)
	for id, p := range rs.Compiled {
		if p.Description != "" {
			t.Fatalf("%s: %q", id, p.Description)
		}
	}
	qaEdit(t, qaAbility(dir, "fists"), func(m map[string]any) {
		m["description"] = nil
	})
	rs = qaMustLoad(t, dir)
	if got := rs.Compiled["fists"].Description; got != "" {
		t.Fatalf("null kept as %q", got)
	}
}

// VTT-848
func TestQADescEmptyRefused(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	qaEdit(t, qaAbility(dir, "fists"), func(m map[string]any) {
		m["description"] = ""
	})
	qaMustRefuse(t, dir, "fists.json", "description", "must not be empty")
}

// VTT-847
func TestQADescKeptAsWritten(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	want := "line one\nline two\r\n\ttabbed \"quoted\" \\ <b>å ö 🍺   end  "
	qaEdit(t, qaAbility(dir, "fists"), func(m map[string]any) {
		m["description"] = want
	})
	rs := qaMustLoad(t, dir)
	if got := rs.Compiled["fists"].Description; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	long := strings.Repeat("x", 100000)
	qaEdit(t, qaAbility(dir, "fists"), func(m map[string]any) {
		m["description"] = long
	})
	if got := qaMustLoad(t, dir).Compiled["fists"].Description; got != long {
		t.Fatal("long description altered")
	}
	qaEdit(t, qaAbility(dir, "fists"), func(m map[string]any) {
		m["description"] = " "
	})
	if got := qaMustLoad(t, dir).Compiled["fists"].Description; got != " " {
		t.Fatalf("blank description: %q", got)
	}
}

// VTT-856
func TestQADescFreeUsageNameKeptOrNone(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	if got := qaMustLoad(t, dir).FreeUsageName; got != "" {
		t.Fatalf("absent: %q", got)
	}
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["free_usage_name"] = nil
	})
	if got := qaMustLoad(t, dir).FreeUsageName; got != "" {
		t.Fatalf("null: %q", got)
	}
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["free_usage_name"] = "No cost"
	})
	if got := qaMustLoad(t, dir).FreeUsageName; got != "No cost" {
		t.Fatalf("kept: %q", got)
	}
}

// VTT-857
func TestQADescFreeUsageNameEmptyRefused(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["free_usage_name"] = ""
	})
	qaMustRefuse(t, dir, "ruleset.json", "free_usage_name", "must not be empty")
}

// VTT-996
func TestQADescUnknownManifestFieldStillRefused(t *testing.T) {
	dir := qaCopyRuleset(t, "tavern-brawl")
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["free_usage_nam"] = "x"
	})
	qaMustRefuse(t, dir, "ruleset.json", "free_usage_nam")

	dir = qaCopyRuleset(t, "tavern-brawl")
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["Bogus_Field"] = 1
	})
	qaMustRefuse(t, dir, "ruleset.json", "Bogus_Field")
}

// VTT-996
func TestQADescKnownFieldsLoadAlongsideSheet(t *testing.T) {
	dir := qaCopyRuleset(t, "dnd45e-minimal")
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["free_usage_name"] = "Free"
	})
	rs := qaMustLoad(t, dir)
	if rs.FreeUsageName != "Free" || len(rs.Sheet) == 0 {
		t.Fatalf("name %q sheet %d", rs.FreeUsageName, len(rs.Sheet))
	}
	// case variants of defined fields read as them
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["FREE_USAGE_NAME"] = m["free_usage_name"]
		delete(m, "free_usage_name")
		m["SHEET"] = m["sheet"]
		delete(m, "sheet")
	})
	rs = qaMustLoad(t, dir)
	if rs.FreeUsageName != "Free" || len(rs.Sheet) == 0 {
		t.Fatalf("case variants: %q %d", rs.FreeUsageName, len(rs.Sheet))
	}
	// an unknown field within sheet is refused
	qaEdit(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		pages := m["SHEET"].([]any)
		pages[0].(map[string]any)["Bogus_Field"] = 1
	})
	qaMustRefuse(t, dir, "ruleset.json", "Bogus_Field")
}

// VTT-849, VTT-858
func TestQADescSchemasDescribeBothFields(t *testing.T) {
	check := func(file, field string) {
		b, err := os.ReadFile(filepath.Join("schema", file))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		props := m["properties"].(map[string]any)
		f, ok := props[field].(map[string]any)
		if !ok {
			t.Fatalf("%s lacks %s", file, field)
		}
		if f["type"] != "string" || f["minLength"] != float64(1) {
			t.Fatalf("%s.%s: %v", file, field, f)
		}
		for _, r := range m["required"].([]any) {
			if r == field {
				t.Fatalf("%s requires %s", file, field)
			}
		}
	}
	check("ability.schema.json", "description")
	check("ruleset.schema.json", "free_usage_name")
}
