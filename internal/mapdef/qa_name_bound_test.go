package mapdef_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

type qaNameMapCase struct {
	label string
	name  string
}

func qaNameMapWrite(t *testing.T, dir, id, name string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format_version": 1, "id": id, "name": name, "grid_width": 2, "grid_height": 2,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func qaNameMapAccepted() []qaNameMapCase {
	return []qaNameMapCase{
		{"256 ascii", strings.Repeat("a", 256)},
		{"128 two-byte runes", strings.Repeat("é", 128)},
		{"85 three-byte runes and one ascii", strings.Repeat("€", 85) + "a"},
		{"64 four-byte runes", strings.Repeat("😀", 64)},
	}
}

func qaNameMapRefused() []qaNameMapCase {
	return []qaNameMapCase{
		{"257 ascii", strings.Repeat("a", 257)},
		{"129 two-byte runes", strings.Repeat("é", 129)},
		{"one ascii then 128 two-byte runes", "a" + strings.Repeat("é", 128)},
		{"64 four-byte runes and one ascii", strings.Repeat("😀", 64) + "a"},
	}
}

// VTT-275
func TestQANameAMapNamedAtTheBoundLoadsWhole(t *testing.T) {
	for _, c := range qaNameMapAccepted() {
		t.Run(c.label, func(t *testing.T) {
			dir := t.TempDir()
			m, err := mapdef.Load(qaNameMapWrite(t, dir, "qa-edge", c.name))
			if err != nil {
				t.Fatalf("Load refused a %d-byte name: %v", len(c.name), err)
			}
			if m.Name != c.name {
				t.Fatalf("loaded name is %d bytes, want %d", len(m.Name), len(c.name))
			}
			m, err = mapdef.LoadInstalled(dir, "qa-edge", "")
			if err != nil || m.Name != c.name {
				t.Fatalf("LoadInstalled: %v", err)
			}
		})
	}
}

// SPEC-014 What reaches a client from a refusal: "LoadInstalled's refusals,
// which name the file as maps/<id>.json and never by the path opened".
// VTT-275
func TestQANameAMapNamedOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	for _, c := range qaNameMapRefused() {
		t.Run(c.label, func(t *testing.T) {
			dir := t.TempDir()
			path := qaNameMapWrite(t, dir, "qa-long", c.name)
			m, err := mapdef.Load(path)
			if err == nil || m != nil {
				t.Fatalf("Load of a %d-byte name (%d runes): map=%v err=%v",
					len(c.name), len([]rune(c.name)), m, err)
			}
			t.Logf("Load refusal: %s", err)
			qaNameMapNamesFileAndField(t, err.Error(), path, c.name)
			m, err = mapdef.LoadInstalled(dir, "qa-long", "")
			if err == nil || m != nil {
				t.Fatalf("LoadInstalled of a %d-byte name: map=%v err=%v", len(c.name), m, err)
			}
			qaNameMapNamesFileAndField(t, err.Error(), "maps/qa-long.json", c.name)
			if strings.Contains(err.Error(), dir) {
				t.Errorf("LoadInstalled refusal %q names the path opened", err)
			}
		})
	}
}

func qaNameMapNamesFileAndField(t *testing.T, msg, file, name string) {
	t.Helper()
	if !strings.Contains(msg, file) {
		t.Errorf("refusal %q does not name the file %s", msg, file)
	}
	rest := strings.ReplaceAll(strings.ReplaceAll(msg, file, ""), name, "")
	if !strings.Contains(rest, "name") {
		t.Errorf("refusal %q does not name the field", msg)
	}
}
