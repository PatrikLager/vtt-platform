package mapdef_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

type qaIDMapCase struct {
	label string
	id    string
}

var (
	qaIDMapIDField    = regexp.MustCompile(`\bid\b`)
	qaIDMapTokenField = regexp.MustCompile(`(?i)token[ _.]?id`)
)

func qaIDMapRefused() []qaIDMapCase {
	return []qaIDMapCase{
		{"129 ascii", strings.Repeat("a", 129)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"one ascii then 64 two-byte runes", "a" + strings.Repeat("é", 64)},
		{"43 three-byte runes", strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
	}
}

func qaIDMapWrite(t *testing.T, dir, file, id, tokenID string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format_version": 1, "id": id, "name": "qa", "grid_width": 2, "grid_height": 2,
		"placements": []map[string]any{{"token_id": tokenID, "actor_id": "qa-actor", "x": 1, "y": 1}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, file)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func qaIDMapNamesFileAndField(t *testing.T, err error, file, value string, field *regexp.Regexp) {
	t.Helper()
	msg := err.Error()
	if !strings.Contains(msg, file) {
		t.Errorf("refusal %q does not name the file %s", msg, file)
	}
	rest := strings.ReplaceAll(msg, file, "")
	if value != "" {
		rest = strings.ReplaceAll(rest, value, "")
	}
	if !field.MatchString(rest) {
		t.Errorf("refusal %q does not name the field %s", msg, field)
	}
}

func qaIDMapRefusedBoth(t *testing.T, id, tokenID, value string, field *regexp.Regexp) {
	t.Helper()
	dir := t.TempDir()
	path := qaIDMapWrite(t, dir, "qa-map.json", id, tokenID)
	m, err := mapdef.Load(path)
	if err == nil || m != nil {
		t.Fatalf("Load: map=%v err=%v, want a refusal", m != nil, err)
	}
	t.Logf("Load refusal: %s", strings.ReplaceAll(err.Error(), dir, "<dir>"))
	qaIDMapNamesFileAndField(t, err, path, value, field)
	mapsDir := t.TempDir()
	qaIDMapWrite(t, mapsDir, id+".json", id, tokenID)
	m, err = mapdef.LoadInstalled(mapsDir, id, "")
	if err == nil || m != nil {
		t.Fatalf("LoadInstalled: map=%v err=%v, want a refusal", m != nil, err)
	}
	qaIDMapNamesFileAndField(t, err, "maps/"+id+".json", value, field)
	if strings.Contains(err.Error(), mapsDir) {
		t.Errorf("LoadInstalled refusal %q names the path opened", err)
	}
}

// VTT-279
func TestQAIDAMapWhoseIDIsOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	for _, c := range qaIDMapRefused() {
		t.Run(c.label, func(t *testing.T) {
			qaIDMapRefusedBoth(t, c.id, "qa-tok", c.id, qaIDMapIDField)
		})
	}
}

// VTT-279
func TestQAIDAMapPlacementTokenIDOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	for _, c := range qaIDMapRefused() {
		t.Run(c.label, func(t *testing.T) {
			qaIDMapRefusedBoth(t, "qa-place", c.id, c.id, qaIDMapTokenField)
		})
	}
}

// VTT-280
func TestQAIDAMapPlacementWithAnEmptyTokenIDIsRefusedNamingFileAndField(t *testing.T) {
	qaIDMapRefusedBoth(t, "qa-place", "", "", qaIDMapTokenField)
}
