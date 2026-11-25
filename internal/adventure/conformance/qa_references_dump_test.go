package conformance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var qaarRulesetFiles = map[string]string{
	"ruleset.json": `{
  "id": "qa-ar-rules", "name": "QA AR Rules",
  "format_version": "3", "version": "1",
  "properties": [
    {"key": "tango", "type": "number", "name": "Tango", "description": "A number."}
  ],
  "forms": [
    {"key": "zulu", "name": "Zulu", "description": "A form.", "properties": []},
    {"key": "yankee", "name": "Yankee", "description": "A form.", "properties": []}
  ],
  "places": [
    {"key": "mike", "name": "Mike", "description": "Holds copies.",
     "forms": ["zulu"], "copies": true, "states": ["lima", "oscar"]},
    {"key": "kilo", "name": "Kilo", "description": "Holds once.",
     "forms": ["zulu", "yankee"]}
  ]
}`,
	"guide.md": "# QA AR Rules\n",
	"catalogue/echo.json": `{"id": "echo", "name": "Echo",
  "description": "An entry.", "form": "zulu"}`,
	"catalogue/delta.json": `{"id": "delta", "name": "Delta",
  "description": "An entry.", "form": "yankee"}`,
}

const qaarHolder = `{"actor_id": "holder", "name": "Holder",
  "kind": "party_member", "attributes": {"tango": 1},
  "references": {
    "r2": {"entry_id": "echo", "place": "mike", "state": "oscar"},
    "r1": {"entry_id": "echo", "place": "mike", "state": "lima"},
    "r3": {"entry_id": "delta", "place": "kilo"}}}`

const qaarBare = `{"actor_id": "bare", "name": "Bare",
  "kind": "non_party", "attributes": {"tango": 2}}`

// Derived by hand from qaarHolder and qaarBare: never regenerate it
// from DumpCompiledBatch.
const qaarHandGolden = `[
  {"type": "adventure_loaded",
   "adventure_loaded": {"adventure_id": "qa-ar", "name": "QA AR"}},
  {"type": "actor_added", "actor_added": {
    "actor_id": "bare", "name": "Bare", "kind": "ACTOR_KIND_NON_PARTY",
    "attributes": {"tango": 2}}},
  {"type": "actor_added", "actor_added": {
    "actor_id": "holder", "name": "Holder", "kind": "ACTOR_KIND_PARTY_MEMBER",
    "attributes": {"tango": 1},
    "references": {
      "r1": {"entry_id": "echo", "place": "mike", "state": "lima"},
      "r2": {"entry_id": "echo", "place": "mike", "state": "oscar"},
      "r3": {"entry_id": "delta", "place": "kilo"STATE}}}},
  {"type": "narration_added", "narration_added": {"text": "It begins."}}
]`

func qaarWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func qaarTree(t *testing.T, golden string) (string, string) {
	t.Helper()
	root := t.TempDir()
	rulesets := filepath.Join(root, "rulesets")
	qaarWrite(t, filepath.Join(rulesets, "qa-ar-rules"), qaarRulesetFiles)
	adv := filepath.Join(root, "qa-ar")
	qaarWrite(t, adv, map[string]string{
		"adventure.json": `{"id": "qa-ar", "name": "QA AR",
  "format_version": "1", "ruleset": "qa-ar-rules",
  "opening_narration": "It begins."}`,
		"guide.md":                    "# QA AR\n\nA guide.\n",
		"actors/a-bare.json":          qaarBare,
		"actors/b-holder.json":        qaarHolder,
		"goldens/compiled-batch.json": golden,
	})
	return adv, rulesets
}

func qaarDump(t *testing.T, adv, rulesets string) []map[string]any {
	t.Helper()
	rs, err := rules.Load(filepath.Join(rulesets, "qa-ar-rules"))
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	a, err := adventure.Load(adv, rs)
	if err != nil {
		t.Fatalf("adventure.Load: %v", err)
	}
	envs, _, err := adventure.Compile(a, engine.NewState())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	raw, err := conformance.DumpCompiledBatch(envs)
	if err != nil {
		t.Fatalf("DumpCompiledBatch: %v", err)
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("dump is not a JSON array: %v", err)
	}
	return out
}

func qaarDumpedActor(t *testing.T, dump []map[string]any, id string) map[string]any {
	t.Helper()
	for _, e := range dump {
		if a, ok := e["actor_added"].(map[string]any); ok && a["actor_id"] == id {
			return a
		}
	}
	t.Fatalf("the dump holds no actor_added for %q", id)
	return nil
}

// VTT-1045.
func TestQAARTheDumpWritesReferencesOnlyWhereHeld(t *testing.T) {
	adv, rulesets := qaarTree(t, "[]")
	dump := qaarDump(t, adv, rulesets)
	holder := qaarDumpedActor(t, dump, "holder")
	refs, ok := holder["references"].(map[string]any)
	if !ok {
		t.Fatalf("the holder's dump writes no references object: %v", holder)
	}
	want := map[string]map[string]any{
		"r1": {"entry_id": "echo", "place": "mike", "state": "lima"},
		"r2": {"entry_id": "echo", "place": "mike", "state": "oscar"},
		"r3": {"entry_id": "delta", "place": "kilo"},
	}
	if len(refs) != len(want) {
		t.Fatalf("the dump writes %d references, want %d: %v", len(refs), len(want), refs)
	}
	for id, w := range want {
		g, ok := refs[id].(map[string]any)
		if !ok {
			t.Errorf("the dump writes no reference %q: %v", id, refs)
			continue
		}
		for k, v := range w {
			if g[k] != v {
				t.Errorf("reference %q field %q = %v, want %v", id, k, g[k], v)
			}
		}
		if st, held := g["state"]; held && id == "r3" {
			t.Logf("a reference in no state dumps \"state\": %q", st)
		}
	}
	if _, held := refs["r3"].(map[string]any)["state"]; !held {
		t.Logf("a reference in no state dumps no state field")
	}
	bare := qaarDumpedActor(t, dump, "bare")
	if r, held := bare["references"]; held {
		t.Errorf("an actor holding no references dumps a references field: %v", r)
	}
}

func qaarGolden(stateOfR3 string) string {
	return strings.Replace(qaarHandGolden, "STATE", stateOfR3, 1)
}

// VTT-1045.
func TestQAARAHandDerivedGoldenPinningReferencesPasses(t *testing.T) {
	for name, st := range map[string]string{
		"state left out":      "",
		"state written empty": `, "state": ""`,
	} {
		t.Run(name, func(t *testing.T) {
			adv, rulesets := qaarTree(t, qaarGolden(st))
			err := conformance.Run(adv, rulesets)
			t.Logf("Run with r3's %s: %v", name, err)
			if name == "state left out" && err != nil {
				t.Errorf("Run refuses the hand-derived golden: %v", err)
			}
		})
	}
	empty := strings.Replace(qaarGolden(""), `"attributes": {"tango": 2}`,
		`"attributes": {"tango": 2}, "references": {}`, 1)
	adv, rulesets := qaarTree(t, empty)
	t.Logf("Run with the bare actor's golden writing references {}: %v",
		conformance.Run(adv, rulesets))
}

// VTT-1045.
func TestQAARAGoldenDisagreeingWithReferencesIsRefused(t *testing.T) {
	base := qaarGolden("")
	r1 := `"r1": {"entry_id": "echo", "place": "mike", "state": "lima"},`
	cases := map[string][2]string{
		"a state changed":  {`"state": "lima"`, `"state": "oscar"`},
		"an entry changed": {`"entry_id": "delta"`, `"entry_id": "echo"`},
		"a place changed":  {`"place": "kilo"`, `"place": "mike"`},
		"an id changed":    {`"r1":`, `"r9":`},
		"one dropped":      {r1, ``},
		"all dropped": {`,
    "references": {
      ` + r1 + `
      "r2": {"entry_id": "echo", "place": "mike", "state": "oscar"},
      "r3": {"entry_id": "delta", "place": "kilo"}}`, ``},
		"one given the bare actor": {`"attributes": {"tango": 2}`,
			`"attributes": {"tango": 2}, "references": {"r1": ` +
				`{"entry_id": "echo", "place": "kilo"}}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if strings.Count(base, c[0]) != 1 {
				t.Fatalf("the edit %q does not occur exactly once", c[0])
			}
			golden := strings.Replace(base, c[0], c[1], 1)
			if !json.Valid([]byte(golden)) {
				t.Fatalf("the edited golden is not JSON: %s", golden)
			}
			adv, rulesets := qaarTree(t, golden)
			err := conformance.Run(adv, rulesets)
			t.Logf("Run: %v", err)
			if err == nil {
				t.Errorf("Run accepts a golden with %s", name)
			}
		})
	}
}

// VTT-1045.
func TestQAARShippedGoldensHoldNoReferencesAndPass(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	dirs, err := filepath.Glob(filepath.Join(repo, "adventures", "*", "adventure.json"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no shipped adventure found: %v", err)
	}
	for _, manifest := range dirs {
		dir := filepath.Dir(manifest)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			if err := conformance.Run(dir, filepath.Join(repo, "rulesets")); err != nil {
				t.Errorf("Run: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "goldens", "compiled-batch.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"references"`) {
				t.Errorf("the shipped golden writes a references field")
			}
		})
	}
}
