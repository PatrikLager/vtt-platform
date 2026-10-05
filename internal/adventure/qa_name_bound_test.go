package adventure_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaNameAdvCase struct {
	label string
	name  string
}

type qaNameAdvFile struct {
	file string
	got  func(*adventure.Adventure) string
}

var qaNameAdvFiles = []qaNameAdvFile{
	{"adventure.json", func(a *adventure.Adventure) string { return a.Name }},
	{"scenes/cellar.json", func(a *adventure.Adventure) string {
		for _, s := range a.Scenes {
			if s.ID == "cellar" {
				return s.Name
			}
		}
		return ""
	}},
	{"actors/grit-scout.json", func(a *adventure.Adventure) string {
		for _, ac := range a.Actors {
			if ac.ID == "grit-scout" {
				return ac.Name
			}
		}
		return ""
	}},
}

func qaNameAdvRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(filepath.Join("testdata", "ruleset"))
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaNameAdvCopy(t *testing.T, file, name string) string {
	t.Helper()
	src := filepath.Join("testdata", "valid")
	dst := filepath.Join(t.TempDir(), "qa-name-bound")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o700)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), body, 0o600)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	path := filepath.Join(dst, filepath.FromSlash(file))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode %s: %v", file, err)
	}
	if _, ok := doc["name"]; !ok {
		t.Fatalf("%s has no name field to edit", file)
	}
	doc["name"] = name
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		t.Fatalf("encode %s: %v", file, err)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	return dst
}

func qaNameAdvAccepted() []qaNameAdvCase {
	return []qaNameAdvCase{
		{"256 ascii", strings.Repeat("a", 256)},
		{"128 two-byte runes", strings.Repeat("é", 128)},
		{"64 four-byte runes", strings.Repeat("😀", 64)},
	}
}

func qaNameAdvRefused() []qaNameAdvCase {
	return []qaNameAdvCase{
		{"257 ascii", strings.Repeat("a", 257)},
		{"129 two-byte runes", strings.Repeat("é", 129)},
		{"one ascii then 128 two-byte runes", "a" + strings.Repeat("é", 128)},
	}
}

// VTT-275 VTT-274
func TestQANameAnAdventureNamedAtTheBoundLoadsCompilesAndFolds(t *testing.T) {
	rs := qaNameAdvRuleset(t)
	for _, f := range qaNameAdvFiles {
		for _, c := range qaNameAdvAccepted() {
			t.Run(f.file+"/"+c.label, func(t *testing.T) {
				adv, err := adventure.Load(qaNameAdvCopy(t, f.file, c.name), rs)
				if err != nil {
					t.Fatalf("Load refused a %d-byte name in %s: %v", len(c.name), f.file, err)
				}
				if got := f.got(adv); got != c.name {
					t.Fatalf("loaded name is %d bytes, want %d", len(got), len(c.name))
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

// VTT-275
func TestQANameAnAdventureNamedOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	rs := qaNameAdvRuleset(t)
	for _, f := range qaNameAdvFiles {
		for _, c := range qaNameAdvRefused() {
			t.Run(f.file+"/"+c.label, func(t *testing.T) {
				dir := qaNameAdvCopy(t, f.file, c.name)
				adv, err := adventure.Load(dir, rs)
				if err == nil || adv != nil {
					t.Fatalf("Load of a %d-byte name (%d runes) in %s: adv=%v err=%v",
						len(c.name), len([]rune(c.name)), f.file, adv != nil, err)
				}
				msg := strings.ReplaceAll(err.Error(), c.name, "<name>")
				t.Logf("refusal: %s", strings.ReplaceAll(msg, dir, "<dir>"))
				if !strings.Contains(msg, f.file) {
					t.Errorf("refusal %q does not name the file %s", msg, f.file)
				}
				rest := strings.ReplaceAll(strings.ReplaceAll(msg, dir, ""), f.file, "")
				if !strings.Contains(rest, "name") {
					t.Errorf("refusal %q does not name the field", msg)
				}
			})
		}
	}
}
