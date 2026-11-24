package gateway_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep the forms and the tool's properties out of key order, or a sort passes
// for the declared order (VTT-1004).
const catalogueRouteManifest = `{
  "id": "catalogue-route", "name": "Catalogue Route", "format_version": "3", "version": "1",
  "properties": [
    {"key": "reach", "type": "number", "name": "Reach", "description": "How far the thing reaches."},
    {"key": "heft", "type": "number", "name": "Heft", "description": "How heavy the thing is."}
  ],
  "forms": [
    {"key": "tool", "name": "Tool", "description": "A thing held to work with.", "properties": ["reach", "heft"]},
    {"key": "charm", "name": "Charm", "description": "A thing worn for luck."}
  ]
}`

// Keep the entries in descending id order: they are written to files in this
// order, and a map filled in id order can iterate in it, so a missing sort
// passes (VTT-1005).
var catalogueRouteEntries = []string{
	`{"id": "zinc-charm", "name": "Zinc Charm", "description": "A grey charm.", "form": "charm"}`,
	`{"id": "stout-tool", "name": "Stout Tool", "description": "A heavier long tool.", "form": "tool",
	  "extends": "long-tool", "values": {"heft": 9}}`,
	`{"id": "plain-tool", "name": "Plain Tool", "description": "A tool.", "form": "tool",
	  "values": {"reach": 1, "heft": 4}}`,
	`{"id": "long-tool", "name": "Long Tool", "description": "A tool that reaches further.", "form": "tool",
	  "extends": "plain-tool", "values": {"reach": 3}}`,
	`{"id": "amber-charm", "name": "Amber Charm", "description": "A yellow charm.", "form": "charm"}`,
}

const catalogueRouteWantForms = `[
  {"key": "tool", "name": "Tool", "description": "A thing held to work with.", "properties": ["reach", "heft"]},
  {"key": "charm", "name": "Charm", "description": "A thing worn for luck.", "properties": []}
]`

const catalogueRouteWantCatalogue = `[
  {"id": "amber-charm", "name": "Amber Charm", "description": "A yellow charm.", "form": "charm", "values": {}},
  {"id": "long-tool", "name": "Long Tool", "description": "A tool that reaches further.", "form": "tool",
   "values": {"reach": 3, "heft": 4}},
  {"id": "plain-tool", "name": "Plain Tool", "description": "A tool.", "form": "tool",
   "values": {"reach": 1, "heft": 4}},
  {"id": "stout-tool", "name": "Stout Tool", "description": "A heavier long tool.", "form": "tool",
   "values": {"reach": 3, "heft": 9}},
  {"id": "zinc-charm", "name": "Zinc Charm", "description": "A grey charm.", "form": "charm", "values": {}}
]`

func catalogueRouteRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ruleset.json"), []byte(catalogueRouteManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("guide"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "catalogue"), 0o700); err != nil {
		t.Fatal(err)
	}
	for i, entry := range catalogueRouteEntries {
		name := filepath.Join(dir, "catalogue", fmt.Sprintf("entry-%d.json", i))
		if err := os.WriteFile(name, []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return qaRRLoad(t, dir)
}

func decodedJSON(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return v
}

// VTT-1004
func TestEveryRoleReadsTheFormsInTheirDeclaredOrder(t *testing.T) {
	rs := catalogueRouteRuleset(t)
	keys := make([]string, 0, len(rs.Forms))
	for _, f := range rs.Forms {
		keys = append(keys, f.Key)
	}
	if len(keys) < 2 || slices.IsSorted(keys) {
		t.Fatalf("fixture: forms %v must be declared out of key order", keys)
	}
	want := decodedJSON(t, []byte(catalogueRouteWantForms))
	tbl := qaRRStart(t, rs, nil)
	for _, role := range qaRRRoles {
		got := decodedJSON(t, servedRuleset(t, tbl.url, tbl.tokens[role])["forms"])
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads forms %v, want %v", role, got, want)
		}
	}
}

// VTT-1005
func TestEveryRoleReadsTheCatalogueSortedByIDWithWhatEachExtends(t *testing.T) {
	want := decodedJSON(t, []byte(catalogueRouteWantCatalogue))
	tbl := qaRRStart(t, catalogueRouteRuleset(t), nil)
	for _, role := range qaRRRoles {
		got := decodedJSON(t, servedRuleset(t, tbl.url, tbl.tokens[role])["catalogue"])
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads catalogue %v, want %v", role, got, want)
		}
	}
}

// VTT-1004 VTT-1005
func TestNoFormsAndNoCatalogueAreEmptyListsForEveryRole(t *testing.T) {
	tavern := loadTavernBrawl(t)
	if len(tavern.Forms) != 0 || len(tavern.Catalogue) != 0 {
		t.Fatalf("fixture: tavern-brawl declares %d forms and %d entries", len(tavern.Forms), len(tavern.Catalogue))
	}
	for name, rs := range map[string]*rules.Ruleset{"tavern-brawl": tavern, "nothing loaded": nil} {
		tbl := qaRRStart(t, rs, nil)
		for _, role := range qaRRRoles {
			obj := servedRuleset(t, tbl.url, tbl.tokens[role])
			for _, field := range []string{"forms", "catalogue"} {
				if got := string(obj[field]); got != "[]" {
					t.Errorf("%s as %s: %s = %s, want []", name, role, field, got)
				}
			}
		}
	}
}
