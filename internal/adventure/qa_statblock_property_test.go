package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaPFAdvDoc = map[string]any

const qaPFAdvManifest = `{
  "id": "qa-pf-adv",
  "name": "QA Statblock Ruleset",
  "format_version": "3",
  "properties": [
    {"key": "vim", "type": "number", "name": "Zeal", "description": "Eagerness to act."},
    {"key": "brace", "type": "number", "name": "Stance", "description": "How firmly one stands."},
    {"key": "focus", "type": "resource", "name": "Clarity", "description": "Attention one can still spend.", "default_max_expr": "10 + @vim"},
    {"key": "pool_b", "type": "resource", "name": "Store", "description": "Supplies on hand."}
  ]
}`

func qaPFAdvWrite(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func qaPFAdvRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := qaPFAdvWrite(t, map[string]string{"ruleset.json": qaPFAdvManifest, "guide.md": "# QA statblocks\n"})
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("the adventure's ruleset does not load: %v", err)
	}
	return rs
}

func qaPFAdvActors() map[string]qaPFAdvDoc {
	return map[string]qaPFAdvDoc{
		"scout": {"actor_id": "scout", "name": "Scout", "kind": "non_party",
			"attributes": qaPFAdvDoc{"vim": 3, "brace": 12},
			"resources":  qaPFAdvDoc{"focus": qaPFAdvDoc{"current": 5, "max": 10}, "pool_b": qaPFAdvDoc{"current": 1, "max": 2}}},
		"runner": {"actor_id": "runner", "name": "Runner", "kind": "party_member",
			"attributes": qaPFAdvDoc{"vim": 6, "brace": 9},
			"resources":  qaPFAdvDoc{"focus": qaPFAdvDoc{"current": 2, "max": 4}, "pool_b": qaPFAdvDoc{"current": 0, "max": 3}}},
	}
}

func qaPFAdvLoad(t *testing.T, actors map[string]qaPFAdvDoc) (string, *adventure.Adventure, error) {
	t.Helper()
	files := map[string]string{
		"adventure.json": `{"id": "qa-yard", "name": "QA Yard", "format_version": "1", "ruleset": "qa-pf-adv",
  "opening_narration": "The yard is quiet."}`,
		"guide.md": "# QA Yard\n",
		"scenes/yard.json": `{"id": "yard", "name": "The Yard", "grid_width": 6, "grid_height": 6,
  "placements": [{"token_id": "tok-1", "actor_id": "scout", "x": 1, "y": 1},
                 {"token_id": "tok-2", "actor_id": "runner", "x": 2, "y": 2}]}`,
		"notes/opening.json": `[{"key": "rumor", "title": "Rumor", "text": "Something stirs."}]`,
	}
	for id, a := range actors {
		b, err := json.MarshalIndent(a, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		files["actors/"+id+".json"] = string(b)
	}
	dir := qaPFAdvWrite(t, files)
	adv, err := adventure.Load(dir, qaPFAdvRuleset(t))
	return dir, adv, err
}

func qaPFAdvWord(text, word string) bool {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_]|$)`).MatchString(text)
}

func qaPFAdvRefused(t *testing.T, dir string, err error, file string, words ...string) string {
	t.Helper()
	if err == nil {
		t.Fatal("adventure.Load accepted a statblock it should refuse")
	}
	full := filepath.Join(dir, filepath.FromSlash(file))
	if !strings.Contains(err.Error(), full) {
		t.Errorf("refusal %q does not name the file %s", err, file)
	}
	rest := strings.ReplaceAll(err.Error(), full, "<file>")
	rest = strings.ReplaceAll(rest, dir, "<dir>")
	for _, w := range words {
		if !qaPFAdvWord(rest, w) {
			t.Errorf("refusal %q does not name %q", rest, w)
		}
	}
	t.Logf("refusal: %s", rest)
	return rest
}

// VTT-617
func TestQAPFStatblockOfDeclaredKeysLoads(t *testing.T) {
	_, adv, err := qaPFAdvLoad(t, qaPFAdvActors())
	if err != nil {
		t.Fatalf("a statblock of declared keys is refused: %v", err)
	}
	var scout *adventure.AdventureActor
	for i := range adv.Actors {
		if adv.Actors[i].ID == "scout" {
			scout = &adv.Actors[i]
		}
	}
	if scout == nil {
		t.Fatal("the scout actor is not loaded")
	}
	if scout.Attributes["vim"] != 3 || scout.Attributes["brace"] != 12 || len(scout.Attributes) != 2 {
		t.Errorf("attributes = %v, want vim 3 and brace 12", scout.Attributes)
	}
	if got := scout.Resources["focus"]; got != (adventure.ResourceVal{Current: 5, Max: 10}) || len(scout.Resources) != 2 {
		t.Errorf("resources = %v, want focus 5 of 10 and pool_b", scout.Resources)
	}
}

// VTT-617
func TestQAPFStatblockUndeclaredKeyIsRefused(t *testing.T) {
	cases := []struct {
		name, field, key string
		edit             func(a qaPFAdvDoc)
	}{
		{"attributes", "attributes", "quux", func(a qaPFAdvDoc) { a["attributes"].(qaPFAdvDoc)["quux"] = 4 }},
		{"resources", "resources", "quux", func(a qaPFAdvDoc) {
			a["resources"].(qaPFAdvDoc)["quux"] = qaPFAdvDoc{"current": 1, "max": 1}
		}},
		{"attributes, case variant", "attributes", "Vim", func(a qaPFAdvDoc) { a["attributes"].(qaPFAdvDoc)["Vim"] = 4 }},
		{"resources, case variant", "resources", "Focus", func(a qaPFAdvDoc) {
			a["resources"].(qaPFAdvDoc)["Focus"] = qaPFAdvDoc{"current": 1, "max": 1}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actors := qaPFAdvActors()
			c.edit(actors["runner"])
			dir, _, err := qaPFAdvLoad(t, actors)
			qaPFAdvRefused(t, dir, err, "actors/runner.json", c.field, c.key)
		})
	}
}

// VTT-618
func TestQAPFStatblockKeyOfTheOtherTypeIsRefused(t *testing.T) {
	cases := []struct {
		name, field, key, typ string
		edit                  func(a qaPFAdvDoc)
	}{
		{"resource under attributes", "attributes", "focus", "resource", func(a qaPFAdvDoc) {
			a["attributes"].(qaPFAdvDoc)["focus"] = 4
		}},
		{"second resource under attributes", "attributes", "pool_b", "resource", func(a qaPFAdvDoc) {
			a["attributes"].(qaPFAdvDoc)["pool_b"] = 1
		}},
		{"number under resources", "resources", "brace", "number", func(a qaPFAdvDoc) {
			a["resources"].(qaPFAdvDoc)["brace"] = qaPFAdvDoc{"current": 1, "max": 1}
		}},
		{"second number under resources", "resources", "vim", "number", func(a qaPFAdvDoc) {
			a["resources"].(qaPFAdvDoc)["vim"] = qaPFAdvDoc{"current": 1, "max": 1}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actors := qaPFAdvActors()
			c.edit(actors["runner"])
			dir, _, err := qaPFAdvLoad(t, actors)
			qaPFAdvRefused(t, dir, err, "actors/runner.json", c.field, c.key, c.typ)
		})
	}
}

// VTT-618
func TestQAPFStatblockMovedBetweenMapsIsRefused(t *testing.T) {
	actors := qaPFAdvActors()
	scout := actors["scout"]
	delete(scout["resources"].(qaPFAdvDoc), "focus")
	scout["attributes"].(qaPFAdvDoc)["focus"] = 5
	dir, _, err := qaPFAdvLoad(t, actors)
	qaPFAdvRefused(t, dir, err, "actors/scout.json", "attributes", "focus", "resource")
}
