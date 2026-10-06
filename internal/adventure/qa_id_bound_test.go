package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaIDAdvIDs struct {
	adv, scene, actor, token string
}

func qaIDAdvDefaults() qaIDAdvIDs {
	return qaIDAdvIDs{adv: "qa-adv", scene: "qa-scene", actor: "qa-extra", token: "qa-tok"}
}

func qaIDAdvWriteJSON(t *testing.T, path string, doc any) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func qaIDAdvRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	qaIDAdvWriteJSON(t, filepath.Join(dir, "ruleset.json"), map[string]any{
		"id": "qa-rules", "name": "QA Rules", "format_version": "2",
		"attributes": []string{"vim"}, "defenses": []string{}, "resources": []any{},
	})
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("qa\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaIDAdvWrite(t *testing.T, ids qaIDAdvIDs) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "qa-adventure")
	qaIDAdvWriteJSON(t, filepath.Join(dir, "adventure.json"), map[string]any{
		"id": ids.adv, "name": "QA", "format_version": "1", "ruleset": "qa-rules", "opening_narration": "qa",
	})
	qaIDAdvWriteJSON(t, filepath.Join(dir, "scenes", ids.scene+".json"), map[string]any{
		"id": ids.scene, "name": "QA", "grid_width": 3, "grid_height": 3,
		"placements": []map[string]any{{"token_id": ids.token, "actor_id": "qa-hero", "x": 1, "y": 1}},
	})
	for _, id := range []string{"qa-hero", ids.actor} {
		qaIDAdvWriteJSON(t, filepath.Join(dir, "actors", id+".json"), map[string]any{
			"actor_id": id, "name": "QA", "kind": "non_party", "attributes": map[string]int{"vim": 1},
		})
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("qa\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

type qaIDAdvField struct {
	name  string
	set   func(*qaIDAdvIDs, string)
	file  func(qaIDAdvIDs) string
	field *regexp.Regexp
	got   func(*adventure.Adventure, qaIDAdvIDs) bool
}

var qaIDAdvBareID = regexp.MustCompile(`\bid\b`)

func qaIDAdvFields() []qaIDAdvField {
	return []qaIDAdvField{
		{
			name: "adventure id", set: func(i *qaIDAdvIDs, v string) { i.adv = v },
			file: func(qaIDAdvIDs) string { return "adventure.json" }, field: qaIDAdvBareID,
			got: func(a *adventure.Adventure, i qaIDAdvIDs) bool { return a.ID == i.adv },
		},
		{
			name: "actor id", set: func(i *qaIDAdvIDs, v string) { i.actor = v },
			file:  func(i qaIDAdvIDs) string { return "actors/" + i.actor + ".json" },
			field: regexp.MustCompile(`(?i)actor[ _.]?id`),
			got: func(a *adventure.Adventure, i qaIDAdvIDs) bool {
				for _, ac := range a.Actors {
					if ac.ID == i.actor {
						return true
					}
				}
				return false
			},
		},
		{
			name: "placement token id", set: func(i *qaIDAdvIDs, v string) { i.token = v },
			file:  func(i qaIDAdvIDs) string { return "scenes/" + i.scene + ".json" },
			field: regexp.MustCompile(`(?i)token[ _.]?id`),
			got: func(a *adventure.Adventure, i qaIDAdvIDs) bool {
				return len(a.Scenes) == 1 && len(a.Scenes[0].Placements) == 1 && a.Scenes[0].Placements[0].TokenID == i.token
			},
		},
	}
}

func qaIDAdvSceneField() qaIDAdvField {
	return qaIDAdvField{
		name: "scene id", set: func(i *qaIDAdvIDs, v string) { i.scene = v },
		file: func(i qaIDAdvIDs) string { return "scenes/" + i.scene + ".json" }, field: qaIDAdvBareID,
		got: func(a *adventure.Adventure, i qaIDAdvIDs) bool {
			return len(a.Scenes) == 1 && a.Scenes[0].ID == i.scene
		},
	}
}

var qaIDAdvRefused = []struct{ label, id string }{
	{"129 ascii", strings.Repeat("a", 129)},
	{"65 two-byte runes", strings.Repeat("é", 65)},
	{"one ascii then 64 two-byte runes", "a" + strings.Repeat("é", 64)},
	{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
}

var qaIDAdvAccepted = []struct{ label, id string }{
	{"128 ascii", "S" + strings.Repeat("a", 126) + "E"},
	{"64 two-byte runes", strings.Repeat("é", 64)},
	{"32 four-byte runes", strings.Repeat("😀", 32)},
}

func qaIDAdvRefusesNamingFileAndField(t *testing.T, f qaIDAdvField) {
	t.Helper()
	rs := qaIDAdvRuleset(t)
	for _, c := range qaIDAdvRefused {
		t.Run(c.label, func(t *testing.T) {
			ids := qaIDAdvDefaults()
			f.set(&ids, c.id)
			dir := qaIDAdvWrite(t, ids)
			adv, err := adventure.Load(dir, rs)
			if err == nil || adv != nil {
				t.Fatalf("Load of a %d-byte %s: adv=%v err=%v", len(c.id), f.name, adv != nil, err)
			}
			file := f.file(ids)
			msg := err.Error()
			t.Logf("refusal: %s", strings.ReplaceAll(strings.ReplaceAll(msg, dir, "<dir>"), c.id, "<id>"))
			if !strings.Contains(msg, file) {
				t.Errorf("refusal %q does not name the file %s", msg, file)
			}
			rest := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(msg, dir, ""), file, ""), c.id, "")
			if !f.field.MatchString(rest) {
				t.Errorf("refusal %q does not name the field %s", msg, f.field)
			}
		})
	}
}

// VTT-279
func TestQAIDAnAdventureWhoseIDsAreOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	for _, f := range qaIDAdvFields() {
		t.Run(f.name, func(t *testing.T) { qaIDAdvRefusesNamingFileAndField(t, f) })
	}
}

// VTT-282
func TestQAIDAnAdventureSceneIDOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	qaIDAdvRefusesNamingFileAndField(t, qaIDAdvSceneField())
}

// VTT-279 VTT-277
func TestQAIDAnAdventureWhoseIDsAreAtTheBoundLoadsCompilesAndFolds(t *testing.T) {
	rs := qaIDAdvRuleset(t)
	for _, f := range append(qaIDAdvFields(), qaIDAdvSceneField()) {
		for _, c := range qaIDAdvAccepted {
			t.Run(f.name+"/"+c.label, func(t *testing.T) {
				ids := qaIDAdvDefaults()
				f.set(&ids, c.id)
				adv, err := adventure.Load(qaIDAdvWrite(t, ids), rs)
				if err != nil {
					t.Fatalf("Load refused a %d-byte %s: %v", len(c.id), f.name, err)
				}
				if !f.got(adv, ids) {
					t.Fatalf("the loaded adventure does not hold the %d-byte %s whole", len(c.id), f.name)
				}
				envs, _, err := adventure.Compile(adv, engine.NewState())
				if err != nil {
					t.Fatalf("Compile: %v", err)
				}
				st := engine.NewState()
				for i, env := range envs {
					if err := engine.Apply(st, env); err != nil {
						t.Fatalf("the fold refused compiled event %d: %v", i, err)
					}
				}
			})
		}
	}
}
