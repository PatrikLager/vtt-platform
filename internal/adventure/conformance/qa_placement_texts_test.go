package conformance_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
)

var (
	qaptGoblinDir   = filepath.Join("..", "..", "..", "adventures", "goblin-ambush")
	qaptRulesetsDir = filepath.Join("..", "..", "..", "rulesets")
)

func qaptCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func qaptEditFile(t *testing.T, path, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c := strings.Count(string(raw), from); c != 1 {
		t.Fatalf("%s: %q occurs %d times, want 1", path, from, c)
	}
	edited := strings.Replace(string(raw), from, to, 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
}

// In a copy of goblin-ambush, a golden whose texts differ from the compiled
// ones in any one field is refused, and so is a scene text the golden does
// not record.
func TestQAPTGoblinAmbushGoldenTextMismatchRefused(t *testing.T) {
	const golden = "goldens/compiled-batch.json"
	const scene = "scenes/ravine.json"
	cases := []struct{ name, token, file, from, to string }{
		{"golden name", "tok-archer", golden,
			`"name": "a goblin with a bow"`, `"name": "a goblin with a sling"`},
		{"golden notes", "tok-archer", golden,
			`"notes": "It keeps to the rocks above the trail."`,
			`"notes": "It keeps to the rocks below the trail."`},
		{"golden gm_name dropped", "tok-cutter", golden,
			"\"name\": \"a hunched goblin\",\n      \"gm_name\": \"Goblin Cutter\"",
			`"name": "a hunched goblin"`},
		{"golden gm_notes added", "tok-fighter", golden,
			"\"actor_id\": \"act-fighter\",\n      \"x\": 0,\n      \"y\": 0\n",
			"\"actor_id\": \"act-fighter\",\n      \"x\": 0,\n      \"y\": 0,\n" +
				"      \"gm_notes\": \"stray\"\n"},
		{"scene name only", "tok-archer", scene,
			`"name": "a goblin with a bow"`, `"name": "a goblin with a sling"`},
	}
	t.Run("unedited copy", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "goblin-ambush")
		qaptCopyTree(t, qaptGoblinDir, dir)
		if err := conformance.Run(dir, qaptRulesetsDir); err != nil {
			t.Fatalf("unedited copy refused: %v", err)
		}
	})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "goblin-ambush")
			qaptCopyTree(t, qaptGoblinDir, dir)
			qaptEditFile(t, filepath.Join(dir, c.file), c.from, c.to)
			err := conformance.Run(dir, qaptRulesetsDir)
			if err == nil {
				t.Fatalf("Run accepted the copy with %s edited", c.name)
			}
			if !strings.Contains(err.Error(), c.token) {
				t.Errorf("refusal does not show %s: %v", c.token, err)
			}
		})
	}
}
