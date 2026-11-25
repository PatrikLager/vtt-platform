package conformance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
)

// dumpedActor answers the actor_added object DumpCompiledBatch writes for an
// actor holding refs.
func dumpedActor(t *testing.T, refs map[string]*vttv1.Reference) map[string]any {
	t.Helper()
	dumped, err := conformance.DumpCompiledBatch([]*vttv1.Envelope{{
		Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, References: refs,
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var batch []struct {
		ActorAdded map[string]any `json:"actor_added"`
	}
	if err := json.Unmarshal(dumped, &batch); err != nil {
		t.Fatalf("the dump is not JSON: %v\n%s", err, dumped)
	}
	if len(batch) != 1 || batch[0].ActorAdded == nil {
		t.Fatalf("the dump holds no one actor_added:\n%s", dumped)
	}
	return batch[0].ActorAdded
}

func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// VTT-1045
func TestADumpWritesTheReferencesAnActorHolds(t *testing.T) {
	got := dumpedActor(t, map[string]*vttv1.Reference{
		"r-one": {EntryId: "echo", Place: "mike", State: "lima"},
		"r-two": {EntryId: "delta", Place: "kilo"},
	})["references"]
	want := decodeJSON(t, `{"r-one": {"entry_id": "echo", "place": "mike", "state": "lima"},
		"r-two": {"entry_id": "delta", "place": "kilo"}}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("references dumped as %v, want %v", got, want)
	}
}

// VTT-1045
func TestADumpWritesNoReferencesFieldForAnActorHoldingNone(t *testing.T) {
	for name, refs := range map[string]map[string]*vttv1.Reference{"none": nil, "an empty map": {}} {
		t.Run(name, func(t *testing.T) {
			if got, written := dumpedActor(t, refs)["references"]; written {
				t.Errorf("an actor holding %s is dumped with references %v, want no field", name, got)
			}
		})
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func writeJSONValue(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// referencesFixture copies testdata's ok adventure, its hero holding
// references r-one and r-two, and fixture-ruleset with forms zulu and yankee,
// places mike and kilo and entries echo and delta, and answers the adventure
// and the rulesets root.
func referencesFixture(t *testing.T) (adventureDir, rulesetsRoot string) {
	t.Helper()
	root := t.TempDir()
	rulesetsRoot = filepath.Join(root, "rulesets")
	rulesetDir := filepath.Join(rulesetsRoot, "fixture-ruleset")
	qaDataCopyTree(t, filepath.Join(fixtureRulesetsRoot(), "fixture-ruleset"), rulesetDir)
	manifest := readJSONMap(t, filepath.Join(rulesetDir, "ruleset.json"))
	manifest["forms"] = []any{
		map[string]any{"key": "zulu", "name": "Zulu", "description": "An invented form."},
		map[string]any{"key": "yankee", "name": "Yankee", "description": "Another invented form."},
	}
	manifest["places"] = []any{
		map[string]any{"key": "mike", "name": "Mike", "description": "Holds either form, more than once.",
			"forms": []any{"zulu", "yankee"}, "copies": true, "states": []any{"lima", "oscar"}},
		map[string]any{"key": "kilo", "name": "Kilo", "description": "Holds the second form once.",
			"forms": []any{"yankee"}},
	}
	writeJSONValue(t, filepath.Join(rulesetDir, "ruleset.json"), manifest)
	for _, e := range []struct{ id, form string }{{"echo", "zulu"}, {"delta", "yankee"}} {
		writeJSONValue(t, filepath.Join(rulesetDir, "catalogue", e.id+".json"),
			map[string]any{"id": e.id, "name": e.id, "description": "An invented entry.", "form": e.form})
	}
	adventureDir = filepath.Join(root, "ok")
	qaDataCopyTree(t, fixture("ok"), adventureDir)
	heroPath := filepath.Join(adventureDir, "actors", "hero.json")
	hero := readJSONMap(t, heroPath)
	hero["references"] = decodeJSON(t, heroReferences)
	writeJSONValue(t, heroPath, hero)
	return adventureDir, rulesetsRoot
}

const heroReferences = `{"r-one": {"entry_id": "echo", "place": "mike", "state": "lima"},
	"r-two": {"entry_id": "delta", "place": "kilo"}}`

// goldenWithHeroReferences writes the hero's actor_added in adventureDir's
// golden with refs, written as the JSON refs.
func goldenWithHeroReferences(t *testing.T, adventureDir, refs string) {
	t.Helper()
	path := filepath.Join(adventureDir, "goldens", "compiled-batch.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var batch []map[string]any
	if err := json.Unmarshal(raw, &batch); err != nil {
		t.Fatal(err)
	}
	hit := false
	for _, e := range batch {
		if added, ok := e["actor_added"].(map[string]any); ok && added["actor_id"] == "hero" {
			added["references"], hit = decodeJSON(t, refs), true
		}
	}
	if !hit {
		t.Fatal("the golden holds no actor_added for hero")
	}
	writeJSONValue(t, path, batch)
}

// VTT-1042, VTT-1045
func TestAGoldenPinsTheReferencesAnAdventuresActorHolds(t *testing.T) {
	t.Run("a golden holding them conforms", func(t *testing.T) {
		adv, rulesets := referencesFixture(t)
		goldenWithHeroReferences(t, adv, heroReferences)
		if err := conformance.Run(adv, rulesets); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
	t.Run("a golden holding none is refused", func(t *testing.T) {
		adv, rulesets := referencesFixture(t)
		err := conformance.Run(adv, rulesets)
		if err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("Run = %v, want the golden refused for the hero's references", err)
		}
	})
	t.Run("a golden holding another state is refused", func(t *testing.T) {
		adv, rulesets := referencesFixture(t)
		goldenWithHeroReferences(t, adv, strings.Replace(heroReferences, `"lima"`, `"oscar"`, 1))
		err := conformance.Run(adv, rulesets)
		if err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("Run = %v, want the golden refused for the hero's references", err)
		}
	})
}
