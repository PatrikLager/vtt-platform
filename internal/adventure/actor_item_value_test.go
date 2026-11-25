package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func editJSONFile(t *testing.T, path string, edit func(m map[string]any)) {
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

func itemValueRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := copyFixtureDirExcluding(t, "testdata/ruleset")
	editJSONFile(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		props := m["properties"].([]any)
		for _, key := range []string{"quebec", "romeo", "sierra"} {
			props = append(props, map[string]any{"key": key, "type": "number", "name": key, "description": "A number."})
		}
		m["properties"] = props
		m["forms"] = []any{
			map[string]any{"key": "zulu", "name": "Zulu", "description": "A form.", "properties": []any{"romeo"}},
			map[string]any{"key": "yankee", "name": "Yankee", "description": "A form.", "properties": []any{"quebec", "romeo"}},
		}
	})
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func adventureWithBraceGuardHolding(t *testing.T, key string) (dir, path string) {
	t.Helper()
	dir = copyFixtureDirExcluding(t, "testdata/valid")
	path = filepath.Join(dir, "actors", "brace-guard.json")
	editJSONFile(t, path, func(m map[string]any) {
		m["attributes"].(map[string]any)[key] = 2
	})
	return dir, path
}

// VTT-1063
func TestAnAdventureWhoseStatblockHoldsAValueACatalogueFormListsIsRefused(t *testing.T) {
	dir, path := adventureWithBraceGuardHolding(t, "quebec")
	_, err := adventure.Load(dir, itemValueRuleset(t))
	want := `adventure: ` + path + `: field "attributes": holds property "quebec", which form "yankee" lists for its entries`
	if err == nil || err.Error() != want {
		t.Fatalf("Load = %v, want %q", err, want)
	}
}

// VTT-1063
func TestAnAdventureWhoseStatblockHoldsANumberNoFormListsLoads(t *testing.T) {
	dir, _ := adventureWithBraceGuardHolding(t, "sierra")
	if _, err := adventure.Load(dir, itemValueRuleset(t)); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
