package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var qaCmdAdvObjectField = regexp.MustCompile(`(?i)objects?(\[\d+\])?[ _.]?id`)

func qaCmdAdvWriteJSON(t *testing.T, path string, doc any) {
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

func qaCmdAdvRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	qaCmdAdvWriteJSON(t, filepath.Join(dir, "ruleset.json"), map[string]any{
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

func qaCmdAdvObjects(ids []string) []map[string]any {
	objs := make([]map[string]any, 0, len(ids))
	for i, id := range ids {
		objs = append(objs, map[string]any{
			"id": id, "kind": "crate", "at": []int{i % 4, i / 4}, "size": []int{1, 1}, "rot": 0,
			"blocks_sight": false, "blocks_move": true, "art": "qa-crate",
		})
	}
	return objs
}

func qaCmdAdvWrite(t *testing.T, objectIDs []string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "qa-adventure")
	qaCmdAdvWriteJSON(t, filepath.Join(dir, "adventure.json"), map[string]any{
		"id": "qa-adv", "name": "QA", "format_version": "1", "ruleset": "qa-rules", "opening_narration": "qa",
	})
	qaCmdAdvWriteJSON(t, filepath.Join(dir, "scenes", "qa-scene.json"), map[string]any{
		"id": "qa-scene", "name": "QA", "grid_width": 4, "grid_height": 4,
		"objects":    qaCmdAdvObjects(objectIDs),
		"placements": []map[string]any{{"token_id": "qa-tok", "actor_id": "qa-hero", "x": 3, "y": 3}},
	})
	qaCmdAdvWriteJSON(t, filepath.Join(dir, "actors", "qa-hero.json"), map[string]any{
		"actor_id": "qa-hero", "name": "QA", "kind": "non_party", "attributes": map[string]int{"vim": 1},
	})
	if err := os.MkdirAll(filepath.Join(dir, "art"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "art", "qa-crate.png"), []byte("fake-png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("qa\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

type qaCmdAdvCase struct {
	label string
	ids   []string
	value string
}

func qaCmdAdvLong() []qaCmdAdvCase {
	return []qaCmdAdvCase{
		{"129 ascii", []string{"qa-1", strings.Repeat("a", 129), "qa-3"}, strings.Repeat("a", 129)},
		{"65 two-byte runes", []string{"qa-1", "qa-2", strings.Repeat("é", 65)}, strings.Repeat("é", 65)},
		{"43 three-byte runes", []string{strings.Repeat("€", 43), "qa-2"}, strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", []string{"qa-1", strings.Repeat("😀", 32) + "a"},
			strings.Repeat("😀", 32) + "a"},
	}
}

func qaCmdAdvBadShape() []qaCmdAdvCase {
	edge := strings.Repeat("😀", 32)
	return []qaCmdAdvCase{
		{"empty", []string{"qa-1", "qa-2", ""}, ""},
		{"repeated apart", []string{"qa-1", "qa-2", "qa-3", "qa-1"}, "qa-1"},
		{"repeated at the bound", []string{edge, "qa-2", edge}, edge},
	}
}

func qaCmdAdvRefused(t *testing.T, c qaCmdAdvCase) {
	t.Helper()
	dir := qaCmdAdvWrite(t, c.ids)
	adv, err := adventure.Load(dir, qaCmdAdvRuleset(t))
	if err == nil || adv != nil {
		t.Fatalf("Load: adv=%v err=%v, want a refusal", adv != nil, err)
	}
	const file = "scenes/qa-scene.json"
	msg := err.Error()
	if !strings.Contains(msg, file) {
		t.Errorf("refusal %q does not name the file %s", msg, file)
	}
	rest := strings.ReplaceAll(strings.ReplaceAll(msg, dir, "<dir>"), file, "<file>")
	if c.value != "" {
		rest = strings.ReplaceAll(rest, c.value, "<v>")
	}
	t.Logf("refusal: %s", rest)
	if !qaCmdAdvObjectField.MatchString(rest) {
		t.Errorf("refusal %q does not name the object id field", rest)
	}
}

// VTT-294
func TestQACmdAnAdventureSceneWhoseObjectIDIsOverTheBoundIsRefused(t *testing.T) {
	for _, c := range qaCmdAdvLong() {
		t.Run(c.label, func(t *testing.T) { qaCmdAdvRefused(t, c) })
	}
}

// VTT-295
func TestQACmdAnAdventureSceneWhoseObjectIDIsEmptyOrRepeatedIsRefused(t *testing.T) {
	for _, c := range qaCmdAdvBadShape() {
		t.Run(c.label, func(t *testing.T) { qaCmdAdvRefused(t, c) })
	}
}

func qaCmdAdvAtBound() [][]string {
	stem := strings.Repeat("é", 63) + "x"
	return [][]string{
		{"S" + strings.Repeat("a", 126) + "E", "qa-2", strings.Repeat("😀", 32)},
		{stem + "a", stem + "b", strings.Repeat("€", 42) + "ab"},
	}
}

func qaCmdFoldAll(t *testing.T, st *engine.State, envs []*vttv1.Envelope) {
	t.Helper()
	for i, env := range envs {
		env.EventId = "qa-cmd-" + strconv.Itoa(i+1)
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("the fold refused compiled event %d: %v", i, err)
		}
	}
}

func qaCmdHeldObjects(st *engine.State, sceneID string) []string {
	var ids []string
	for _, o := range st.Scenes[sceneID].Objects {
		ids = append(ids, o.ObjectID)
	}
	return ids
}

// VTT-294 VTT-295 VTT-291 VTT-292
func TestQACmdAnAdventureSceneWithObjectIDsUpToTheBoundLoadsCompilesAndFolds(t *testing.T) {
	rs := qaCmdAdvRuleset(t)
	for _, ids := range qaCmdAdvAtBound() {
		adv, err := adventure.Load(qaCmdAdvWrite(t, ids), rs)
		if err != nil {
			t.Fatalf("Load refused distinct object ids up to the bound: %v", err)
		}
		var loaded []string
		for _, o := range adv.Scenes[0].Objects {
			loaded = append(loaded, o.ID)
		}
		if !slices.Equal(loaded, ids) {
			t.Fatalf("loaded scene holds %d object ids, not the %d written whole", len(loaded), len(ids))
		}
		envs, _, err := adventure.Compile(adv, engine.NewState())
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		st := engine.NewState()
		qaCmdFoldAll(t, st, envs)
		if got := qaCmdHeldObjects(st, "qa-scene"); !slices.Equal(got, ids) {
			t.Fatalf("folded scene holds %d object ids, not the %d loaded", len(got), len(ids))
		}
	}
}

// VTT-291 VTT-292
func TestQACmdAMapWithObjectIDsUpToTheBoundCompilesAndFolds(t *testing.T) {
	for _, ids := range qaCmdAdvAtBound() {
		dir := t.TempDir()
		qaCmdAdvWriteJSON(t, filepath.Join(dir, "qa-map.json"), map[string]any{
			"format_version": 1, "id": "qa-map", "name": "qa", "grid_width": 4, "grid_height": 4,
			"objects": qaCmdAdvObjects(ids),
		})
		m, err := mapdef.LoadInstalled(dir, "qa-map", "")
		if err != nil {
			t.Fatalf("LoadInstalled: %v", err)
		}
		envs, _, err := mapdef.Compile(m, "")
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		st := engine.NewState()
		qaCmdFoldAll(t, st, envs)
		if got := qaCmdHeldObjects(st, "qa-map"); !slices.Equal(got, ids) {
			t.Fatalf("folded scene holds %d object ids, not the %d loaded", len(got), len(ids))
		}
	}
}

// VTT-291 VTT-292
func TestQACmdAnAdventureEditedPastTheLoaderIsRefusedByTheFold(t *testing.T) {
	rs := qaCmdAdvRuleset(t)
	long := strings.Repeat("é", 65)
	cases := []struct {
		label, id, want string
	}{
		{"over the bound", long, "engine: object id must be"},
		{"empty", "", "engine: object id must be"},
		{"repeated", "qa-1", `engine: object "qa-1" appears twice in scene "qa-scene"`},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			adv, err := adventure.Load(qaCmdAdvWrite(t, []string{"qa-1", "qa-2", "qa-3"}), rs)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			adv.Scenes[0].Objects[2].ID = c.id
			envs, _, err := adventure.Compile(adv, engine.NewState())
			if err != nil {
				t.Fatalf("Compile refused before the fold could: %v", err)
			}
			st := engine.NewState()
			for i, env := range envs {
				if ferr := engine.Apply(st, env); ferr != nil {
					msg := strings.ReplaceAll(ferr.Error(), long, "<v>")
					t.Logf("refusal at event %d: %s", i, msg)
					if !strings.HasPrefix(msg, c.want) {
						t.Fatalf("refusal %q, want it to begin %q", msg, c.want)
					}
					return
				}
			}
			t.Fatalf("the fold accepted every event of a scene whose third object id is %q", c.label)
		})
	}
}
