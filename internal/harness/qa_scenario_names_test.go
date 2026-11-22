package harness_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

var qaDigits = regexp.MustCompile(`[0-9]+$`)

func qaRoleNamed(name string) bool {
	n := strings.ToLower(qaDigits.ReplaceAllString(name, ""))
	return n == "player" || n == "spectator"
}

func qaNameViolations(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenario files in %s: %v", dir, err)
	}
	var bad []string
	seen := 0
	for _, p := range files {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var f struct {
			Participants []struct {
				Name string `json:"name"`
			} `json:"participants"`
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, pt := range f.Participants {
			seen++
			if qaRoleNamed(pt.Name) {
				bad = append(bad, fmt.Sprintf("%s: %s", filepath.Base(p), pt.Name))
			}
		}
	}
	if seen == 0 {
		t.Fatal("walk saw no participants")
	}
	return bad
}

// VTT-943
func TestQAScenarioNamesAreNotRoles(t *testing.T) {
	if bad := qaNameViolations(t, "../../scenarios"); len(bad) > 0 {
		t.Fatalf("role-named participants: %v", bad)
	}
}

// VTT-943
func TestQASoakNamesAreNotRoles(t *testing.T) {
	roster := harness.SoakParticipants()
	if len(roster) == 0 {
		t.Fatal("empty soak roster")
	}
	for _, p := range roster {
		if qaRoleNamed(p.Name) {
			t.Errorf("soak participant %q", p.Name)
		}
	}
}
