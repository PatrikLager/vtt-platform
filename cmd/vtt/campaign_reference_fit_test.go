package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// referenceRulesetDir is internal/rules/testdata/valid, whose guard-stance
// spends pool_a, with forms zulu and yankee, places mike and kilo in place of
// its own, entries echo of zulu and delta of yankee, and no sheet.
func referenceRulesetDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "internal", "rules", "testdata", "valid"))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["forms"] = append(m["forms"].([]any),
		map[string]any{"key": "zulu", "name": "Zulu", "description": "An invented form."},
		map[string]any{"key": "yankee", "name": "Yankee", "description": "Another invented form."})
	m["places"] = []any{
		map[string]any{"key": "mike", "name": "Mike", "description": "Holds either form, more than once.",
			"forms": []any{"zulu", "yankee"}, "copies": true, "states": []any{"lima", "oscar"}},
		map[string]any{"key": "kilo", "name": "Kilo", "description": "Holds the second form once.",
			"forms": []any{"yankee"}},
	}
	delete(m, "sheet")
	writeReferenceJSON(t, path, m)
	writeReferenceJSON(t, filepath.Join(dir, "catalogue", "echo.json"),
		map[string]any{"id": "echo", "name": "Echo", "description": "An entry of zulu.", "form": "zulu"})
	writeReferenceJSON(t, filepath.Join(dir, "catalogue", "delta.json"),
		map[string]any{"id": "delta", "name": "Delta", "description": "An entry of yankee.", "form": "yankee"})
	return dir
}

func writeReferenceJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func referenceRefused(t *testing.T, dir, rulesetDir string) string {
	t.Helper()
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", rulesetDir, "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose actors hold references that do not fit was served")
	}
	return err.Error()
}

// VTT-1044, VTT-1071
func TestACampaignWhoseActorsHoldReferencesThatDoNotFitIsNotServed(t *testing.T) {
	dir := writeFitCampaign(t,
		&vttv1.Actor{ActorId: "zed", References: map[string]*vttv1.Reference{
			"b": {EntryId: "echo", Place: "kilo"}, "a": {EntryId: "victor", Place: "mike", State: "lima"}}},
		&vttv1.Actor{ActorId: "fit", References: map[string]*vttv1.Reference{
			"a": {EntryId: "echo", Place: "mike", State: "lima"}, "b": {EntryId: "echo", Place: "mike", State: "lima"},
			"c": {EntryId: "delta", Place: "kilo"}}},
		&vttv1.Actor{ActorId: "abe", References: map[string]*vttv1.Reference{
			"q": {EntryId: "delta", Place: "kilo"}, "r": {EntryId: "delta", Place: "kilo"}}},
	)
	msg := referenceRefused(t, dir, referenceRulesetDir(t))
	abe := strings.Index(msg, `actor "abe": references: reference "r" names entry "delta", which reference "q" holds in place "kilo" already, and that place holds an entry once`)
	zed := strings.Index(msg, `actor "zed": references: reference "a" names entry "victor", which the catalogue does not hold`)
	if abe < 0 || zed < 0 || abe > zed {
		t.Fatalf("error = %q, want abe's second reference to delta in kilo, then zed's first reference", msg)
	}
	if strings.Contains(msg, `"fit"`) || strings.Contains(msg, `reference "b"`) {
		t.Errorf("error = %q, names the fitting actor or a fault after an actor's first", msg)
	}
}

// VTT-1044 VTT-777
func TestACampaignActorsReferencesAreReadAfterTheSpendRule(t *testing.T) {
	dir := writeFitCampaign(t, &vttv1.Actor{ActorId: "zed", AbilityIds: []string{"guard-stance"},
		References: map[string]*vttv1.Reference{"a": {EntryId: "victor", Place: "mike", State: "lima"}}})
	msg := referenceRefused(t, dir, referenceRulesetDir(t))
	if !strings.Contains(msg, `actor "zed": ability_ids: names ability "guard-stance", which spends resource "pool_a" the sheet does not hold`) ||
		strings.Contains(msg, ": references: ") {
		t.Fatalf("error = %q, want the spend rule's fault and not the reference's", msg)
	}
}

// VTT-1044
func TestACampaignWhoseActorsReferencesFitIsServed(t *testing.T) {
	dir := writeFitCampaign(t, &vttv1.Actor{ActorId: "fit", References: map[string]*vttv1.Reference{
		"a": {EntryId: "echo", Place: "mike", State: "lima"}, "b": {EntryId: "echo", Place: "mike", State: "oscar"},
		"c": {EntryId: "delta", Place: "mike", State: "lima"}, "d": {EntryId: "delta", Place: "kilo"}}})
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", referenceRulesetDir(t), "")
	if err != nil {
		t.Fatalf("a campaign whose references fit was refused: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
