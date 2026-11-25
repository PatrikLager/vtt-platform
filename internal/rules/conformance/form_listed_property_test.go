package conformance_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
	"github.com/PatrikLager/vtt-platform/internal/rules/conformance"
)

func editJSON(t *testing.T, path string, edit func(m map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func formListedCopy(t *testing.T, attackStat string) string {
	t.Helper()
	src := fixture("minimal-v2")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	editJSON(t, filepath.Join(dst, "ruleset.json"), func(m map[string]any) {
		m["properties"] = append(m["properties"].([]any), map[string]any{
			"key": "quebec", "type": "number", "name": "Quebec", "description": "A number form zulu lists."})
		m["forms"] = []any{map[string]any{
			"key": "zulu", "name": "Zulu", "description": "A form.", "properties": []any{"quebec"}}}
	})
	editJSON(t, filepath.Join(dst, "abilities", "poke.json"), func(m map[string]any) {
		m["compose"].([]any)[1].(map[string]any)["bind"].(map[string]any)["attack_stat"] = attackStat
	})
	return dst
}

// VTT-1066
func TestRunFailsAtTheSmokePassAnAbilityReadingAPropertyAFormLists(t *testing.T) {
	err := conformance.Run(formListedCopy(t, "quebec"))
	if err == nil {
		t.Fatal("Run: want the smoke pass to fail poke, got nil")
	}
	// Keep the smoke pass's own words here: a fixture actor holding quebec
	// passes the smoke pass and fails at poke-hit's golden, naming poke too.
	want := `ability "poke" failed to resolve against the generated fixture actor: ` +
		`the resolution roll reads attribute "quebec" from the caster, which form "zulu" lists: no sheet holds it`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Run error = %q, want it to contain %q", err.Error(), want)
	}
}

// VTT-1066
func TestRunPassesARulesetWhoseAbilityReadsAPropertyNoFormLists(t *testing.T) {
	if err := conformance.Run(formListedCopy(t, "str")); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func outcomeCopy(t *testing.T, branch, deltaExpr string) string {
	t.Helper()
	dir := formListedCopy(t, "str")
	var key any
	consumes := []any{}
	if branch != "always" {
		key, consumes = "strike", []any{"strike"}
	}
	atom, err := json.Marshal(map[string]any{"id": "extra-outcome", "params": []any{}, "provides": []any{},
		"consumes": consumes, "contributes": []any{map[string]any{"kind": "outcome", "key": key,
			"branch": branch, "effects": []any{map[string]any{
				"resource_change": map[string]any{"resource": "pool", "delta_expr": deltaExpr}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "atoms", "extra-outcome.json"), atom, 0o644); err != nil {
		t.Fatal(err)
	}
	editJSON(t, filepath.Join(dir, "abilities", "poke.json"), func(m map[string]any) {
		m["compose"] = append(m["compose"].([]any), map[string]any{"atom": "extra-outcome", "bind": map[string]any{}})
	})
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	golden, err := conformance.DumpCompiledPower(rs.Compiled["poke"])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goldens", "compiled", "poke.json"), golden, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// VTT-1066
func TestRunFailsAtTheSmokePassAnAbilityReadingAListedKeyOnlyWhenItMisses(t *testing.T) {
	for _, c := range []struct{ name, expr, reason string }{
		{"from the caster", "@caster.quebec",
			`the miss outcome reads attribute "quebec" from the caster, which form "zulu" lists: no sheet holds it`},
		{"from the target", "@target.quebec",
			`the miss outcome reads attribute "quebec" from the target, which form "zulu" lists: no sheet holds it`},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := conformance.Run(outcomeCopy(t, "miss", c.expr))
			want := `ability "poke" failed to resolve against the generated fixture actor: ` + c.reason
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Run = %v, want an error containing %q", err, want)
			}
		})
	}
}

// VTT-1066
func TestRunPassesAnAbilityWhoseEveryBranchReadsOnlyKeysTheFixtureHolds(t *testing.T) {
	if err := conformance.Run(outcomeCopy(t, "miss", "0 - (@caster.str + @target.def)")); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// Keep each cause pinned whole: Resolve's own errors for these reads carry the
// smoke pass's prefix and the key too.
// VTT-1066
func TestRunNamesTheVsAndTheEffectThatReadAListedKey(t *testing.T) {
	for _, c := range []struct {
		name  string
		dir   func(t *testing.T) string
		cause string
	}{
		{"the resolution vs", func(t *testing.T) string {
			t.Helper()
			dir := formListedCopy(t, "str")
			editJSON(t, filepath.Join(dir, "atoms", "strike-roll.json"), func(m map[string]any) {
				m["contributes"].([]any)[0].(map[string]any)["vs"] = "@target.quebec"
			})
			return dir
		}, `the resolution vs reads attribute "quebec" from the target, which form "zulu" lists: no sheet holds it`},
		{"an effect", func(t *testing.T) string {
			t.Helper()
			return outcomeCopy(t, "always", "@caster.quebec")
		},
			`the effect reads attribute "quebec" from the caster, which form "zulu" lists: no sheet holds it`},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := conformance.Run(c.dir(t))
			want := `ability "poke" failed to resolve against the generated fixture actor: ` + c.cause
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Run = %v, want an error containing %q", err, want)
			}
		})
	}
}
