package conformance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func qaRulesetDirs(t *testing.T) []string {
	t.Helper()
	dirs, err := filepath.Glob("../../../rulesets/*/ruleset.json")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no rulesets found: %v", err)
	}
	for i, d := range dirs {
		dirs[i] = filepath.Dir(d)
	}
	return dirs
}

func qaReadJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return m
}

func qaDescriptionFault(v any) string {
	s, ok := v.(string)
	if !ok {
		return "not a string"
	}
	if strings.TrimSpace(s) == "" {
		return "empty"
	}
	if strings.IndexFunc(s, unicode.IsDigit) >= 0 {
		return "holds a digit"
	}
	return ""
}

// VTT-850
func TestQAShippedAbilityDescriptionsNonEmptyNoDigit(t *testing.T) {
	files, _ := filepath.Glob("../../../rulesets/*/abilities/*.json")
	if len(files) == 0 {
		t.Fatal("no ability files")
	}
	for _, f := range files {
		if fault := qaDescriptionFault(qaReadJSON(t, f)["description"]); fault != "" {
			t.Errorf("%s: description %s", f, fault)
		}
	}
}

func TestQAShippedDescriptionCheckerRejects(t *testing.T) {
	for name, v := range map[string]any{"digit": "hits for 2", "empty": "  ", "missing": nil, "number": 3.0} {
		if qaDescriptionFault(v) == "" {
			t.Errorf("%s accepted", name)
		}
	}
	if qaDescriptionFault("A plain sentence.") != "" {
		t.Error("plain sentence refused")
	}
}

// VTT-856
func TestQAShippedFreeUsageName(t *testing.T) {
	for _, d := range qaRulesetDirs(t) {
		rs, err := rules.Load(d)
		if err != nil {
			t.Fatal(err)
		}
		declared, _ := qaReadJSON(t, filepath.Join(d, "ruleset.json"))["free_usage_name"].(string)
		if rs.FreeUsageName != declared {
			t.Errorf("%s: FreeUsageName %q, file declares %q", d, rs.FreeUsageName, declared)
		}
	}
}

// VTT-847 VTT-850
func TestQAShippedCompiledDescriptionMatchesFile(t *testing.T) {
	n := 0
	for _, d := range qaRulesetDirs(t) {
		rs, err := rules.Load(d)
		if err != nil {
			t.Fatal(err)
		}
		files, _ := filepath.Glob(filepath.Join(d, "abilities", "*.json"))
		if len(rs.Compiled) != len(files) {
			t.Errorf("%s: %d compiled, %d files", d, len(rs.Compiled), len(files))
		}
		for _, f := range files {
			m := qaReadJSON(t, f)
			id, _ := m["id"].(string)
			want, _ := m["description"].(string)
			cp := rs.Compiled[id]
			if cp == nil {
				t.Errorf("%s: %q not compiled", f, id)
				continue
			}
			n++
			if cp.Description != want || want == "" {
				t.Errorf("%s: Description %q, file %q", f, cp.Description, want)
			}
		}
	}
	if n == 0 {
		t.Fatal("nothing compared")
	}
}
