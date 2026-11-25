package adventure_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep zulu declared before yankee: the form named must be the first in
// declared order, which byte order would not give.
func qaivRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rs")
	src := filepath.Join("..", "rules", "testdata", "valid")
	if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	props, _ := m["properties"].([]any)
	for _, k := range []string{"quebec", "romeo", "sierra", "tango"} {
		props = append(props, map[string]any{
			"key": k, "type": "number", "name": "N " + k,
			"description": "An invented number.",
		})
	}
	m["properties"] = props
	forms, _ := m["forms"].([]any)
	m["forms"] = append([]any{
		map[string]any{
			"key": "zulu", "name": "Lima", "description": "An invented form.",
			"properties": []string{"romeo", "quebec"},
		},
		map[string]any{
			"key": "yankee", "name": "Kilo", "description": "Another form.",
			"properties": []string{"quebec", "tango"},
		},
	}, forms...)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return rs
}

func qaivWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func qaivAdventure(t *testing.T, rs *rules.Ruleset, actors map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "adv")
	qaivWrite(t, filepath.Join(dir, "adventure.json"), `{"id": "qaiv-adv",
"name": "QA", "format_version": "1", "ruleset": "`+rs.ID+`",
"opening_narration": "It begins."}`)
	qaivWrite(t, filepath.Join(dir, "guide.md"), "# QA\n\nA guide.\n")
	for name, body := range actors {
		qaivWrite(t, filepath.Join(dir, "actors", name), body)
	}
	return dir
}

func qaivActor(id, attrs string) string {
	return `{"actor_id": "` + id + `", "name": "A ` + id +
		`", "kind": "party_member", "attributes": {` + attrs + `}}`
}

func qaivRefusal(t *testing.T, dir string, rs *rules.Ruleset) string {
	t.Helper()
	adv, err := adventure.Load(dir, rs)
	if err == nil {
		t.Fatalf("loaded %+v, want a refusal", adv)
	}
	if errors.Is(err, adventure.ErrRulesetMismatch) {
		t.Fatalf("refused as another ruleset's: %v", err)
	}
	return err.Error()
}

// VTT-1063
func TestQAIVAStatblockHoldingAListedKeyIsRefused(t *testing.T) {
	rs := qaivRuleset(t)
	dir := qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"sierra": 1, "quebec": 2`),
	})
	msg := qaivRefusal(t, dir, rs)
	file := filepath.Join(dir, "actors", "a-one.json")
	for _, want := range []string{file, `field "attributes"`, `"quebec"`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal %q does not name %s", msg, want)
		}
	}
	t.Logf("refusal: %s", msg)
}

// VTT-1063
func TestQAIVTheRefusalIsShapedAsAnUndeclaredKeys(t *testing.T) {
	rs := qaivRuleset(t)
	listed := qaivRefusal(t, qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"tango": 2`),
	}), rs)
	undeclared := qaivRefusal(t, qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"xray": 2`),
	}), rs)
	prefix := func(s string) string {
		_, after, _ := strings.Cut(s, filepath.Join("actors", "a-one.json"))
		head, _, _ := strings.Cut(after, `"attributes"`)
		return head
	}
	if prefix(listed) != prefix(undeclared) || !strings.Contains(listed, `"tango"`) {
		t.Fatalf("refusal %q is not shaped as %q", listed, undeclared)
	}
	if !strings.Contains(listed, "yankee") {
		t.Fatalf("refusal %q does not name the form that lists the key", listed)
	}
}

// VTT-1063
func TestQAIVAnUndeclaredKeyIsToldBeforeAListedOne(t *testing.T) {
	rs := qaivRuleset(t)
	msg := qaivRefusal(t, qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"quebec": 2, "xray": 1`),
	}), rs)
	if !strings.Contains(msg, `"xray"`) || strings.Contains(msg, `"quebec"`) {
		t.Fatalf("refusal %q, want the undeclared xray", msg)
	}
}

// VTT-1063
func TestQAIVAListedKeyIsToldBeforeANullValue(t *testing.T) {
	rs := qaivRuleset(t)
	msg := qaivRefusal(t, qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"quebec": null`),
	}), rs)
	if !strings.Contains(msg, `field "attributes"`) || !strings.Contains(msg, "zulu") {
		t.Fatalf("refusal %q, want the listed key before its value", msg)
	}
}

// VTT-1063
func TestQAIVTheFileHoldingTheListedKeyIsNamed(t *testing.T) {
	rs := qaivRuleset(t)
	dir := qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"sierra": 1`),
		"b-two.json": qaivActor("b-two", `"romeo": 1`),
	})
	msg := qaivRefusal(t, dir, rs)
	if !strings.Contains(msg, filepath.Join(dir, "actors", "b-two.json")) ||
		strings.Contains(msg, "a-one.json") || !strings.Contains(msg, `"romeo"`) {
		t.Fatalf("refusal %q does not name b-two.json and romeo", msg)
	}
}

// VTT-1063
func TestQAIVAStatblockOfNumbersNoFormListsLoads(t *testing.T) {
	rs := qaivRuleset(t)
	dir := qaivAdventure(t, rs, map[string]string{
		"a-one.json": qaivActor("a-one", `"sierra": 1`),
	})
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if len(adv.Actors) != 1 || adv.Actors[0].Attributes["sierra"] != 1 {
		t.Fatalf("actors %+v, want a-one holding sierra", adv.Actors)
	}
}

// VTT-1063
func TestQAIVEveryShippedAdventureStillLoads(t *testing.T) {
	manifests, err := filepath.Glob(filepath.Join("..", "..", "adventures", "*", "adventure.json"))
	if err != nil || len(manifests) == 0 {
		t.Fatalf("no shipped adventure found: %v", err)
	}
	for _, m := range manifests {
		raw, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("read %s: %v", m, err)
		}
		var head struct {
			Ruleset string `json:"ruleset"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatalf("decode %s: %v", m, err)
		}
		rs, err := rules.Load(filepath.Join("..", "..", "rulesets", head.Ruleset))
		if err != nil {
			t.Fatalf("load %s: %v", head.Ruleset, err)
		}
		if _, err := adventure.Load(filepath.Dir(m), rs); err != nil {
			t.Fatalf("shipped %s refused: %v", m, err)
		}
	}
}
