package adventure_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
)

// VTT-618
func TestAStatblockGivingAPropertyTheOtherTypesValueIsRefused(t *testing.T) {
	rs := loadFixtureRuleset(t)
	cases := []struct {
		name, old, replacement, field, want string
	}{
		{"a resource among the attributes", `"vim": 10,`, `"vim": 10, "focus": 3,`,
			`field "attributes"`, `gives resource property "focus" a number's value`},
		{"a number among the resources", `"resources": {`, `"resources": { "vim": { "current": 1, "max": 1 },`,
			`field "resources"`, `gives number property "vim" a resource's value`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := copyFixtureDirExcluding(t, "testdata/valid")
			path := filepath.Join(dir, "actors", "brace-guard.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), c.old) {
				t.Fatalf("brace-guard.json holds no %q", c.old)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), c.old, c.replacement, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = adventure.Load(dir, rs)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			for _, want := range []string{"brace-guard.json", c.field, c.want} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
	t.Run("each key given its own type's value loads", func(t *testing.T) {
		if _, err := adventure.Load(copyFixtureDirExcluding(t, "testdata/valid"), rs); err != nil {
			t.Fatalf("Load: %v", err)
		}
	})
}
