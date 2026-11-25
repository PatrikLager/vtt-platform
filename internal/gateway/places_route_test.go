package gateway_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep the places, a place's forms and its states out of key order, and the
// section's places out of the declared order, or a sort passes for either
// order (VTT-1052, VTT-1053).
const placesRouteManifest = `{
  "id": "places-route", "name": "Places Route", "format_version": "3", "version": "1",
  "properties": [
    {"key": "sierra", "type": "number", "name": "Sierra", "description": "A number no form lists."},
    {"key": "romeo", "type": "number", "name": "Romeo", "description": "A number both forms list."},
    {"key": "quebec", "type": "number", "name": "Quebec", "description": "A number the first form lists."}
  ],
  "forms": [
    {"key": "zulu", "name": "Zulu", "description": "The first form.", "properties": ["quebec", "romeo"]},
    {"key": "yankee", "name": "Yankee", "description": "The second form.", "properties": ["romeo"]}
  ],
  "places": [
    {"key": "mike", "name": "Mike", "description": "Holds either form, more than once.",
     "forms": ["zulu", "yankee"], "copies": true, "states": ["oscar", "lima"]},
    {"key": "kilo", "name": "Kilo", "description": "Holds the second form once.", "forms": ["yankee"]}
  ],
  "sheet": [
    {"name": "Front", "sections": [
      {"name": "Numbers", "values": ["sierra"]},
      {"name": "Marks", "conditions": true},
      {"name": "Holdings", "places": ["kilo", "mike"]},
      {"name": "Moves", "abilities": ["at_will"]}
    ]}
  ]
}`

const placesRouteWantPlaces = `[
  {"key": "mike", "name": "Mike", "description": "Holds either form, more than once.",
   "forms": ["zulu", "yankee"], "copies": true, "states": ["oscar", "lima"]},
  {"key": "kilo", "name": "Kilo", "description": "Holds the second form once.",
   "forms": ["yankee"], "copies": false, "states": []}
]`

const placesRouteWantSheet = `[
  {"name": "Front", "sections": [
    {"name": "Numbers", "kind": "values", "values": ["sierra"]},
    {"name": "Marks", "kind": "conditions"},
    {"name": "Holdings", "kind": "places", "places": ["kilo", "mike"]},
    {"name": "Moves", "kind": "abilities", "abilities": [{"kind": "atWill"}]}
  ]}
]`

func placesRouteRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ruleset.json"), []byte(placesRouteManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("guide"), 0o600); err != nil {
		t.Fatal(err)
	}
	rs := qaRRLoad(t, dir)
	keys := make([]string, 0, len(rs.Places))
	for _, p := range rs.Places {
		keys = append(keys, p.Key)
	}
	if len(keys) < 2 || slices.IsSorted(keys) {
		t.Fatalf("fixture: places %v must be declared out of key order", keys)
	}
	return rs
}

// VTT-1052
func TestEveryRoleReadsThePlacesInTheirDeclaredOrder(t *testing.T) {
	want := decodedJSON(t, []byte(placesRouteWantPlaces))
	tb := qacrServe(t, placesRouteRuleset(t))
	for _, role := range qacrRoles {
		got := decodedJSON(t, tb.readRuleset(t, role)["places"])
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads places %v, want %v", role, got, want)
		}
	}
}

// VTT-1052
func TestAPlaceIsServedWhetherItCopiesAndAPlaceNamingNoStateAnEmptyList(t *testing.T) {
	tb := qacrServe(t, placesRouteRuleset(t))
	for _, role := range qacrRoles {
		var places []map[string]json.RawMessage
		if err := json.Unmarshal(tb.readRuleset(t, role)["places"], &places); err != nil || len(places) != 2 {
			t.Fatalf("%s: places %v (%v), want two", role, places, err)
		}
		for i, want := range []map[string]string{
			{"copies": "true", "states": `["oscar","lima"]`},
			{"copies": "false", "states": "[]"},
		} {
			for field, raw := range want {
				if got := string(places[i][field]); got != raw {
					t.Errorf("%s: places[%d].%s = %q, want %s", role, i, field, got, raw)
				}
			}
		}
	}
}

// VTT-1052
func TestNoPlacesAreAnEmptyListForEveryRole(t *testing.T) {
	tavern := loadTavernBrawl(t)
	if len(tavern.Places) != 0 {
		t.Fatalf("fixture: tavern-brawl declares %d places", len(tavern.Places))
	}
	for name, rs := range map[string]*rules.Ruleset{"tavern-brawl": tavern, "nothing loaded": nil} {
		tb := qacrServe(t, rs)
		for _, role := range qacrRoles {
			if got := string(tb.readRuleset(t, role)["places"]); got != "[]" {
				t.Errorf("%s as %s: places = %s, want []", name, role, got)
			}
		}
	}
}

// VTT-1053
func TestASheetSectionOfPlacesIsServedAsKindPlacesWithItsPlaceKeys(t *testing.T) {
	want := decodedJSON(t, []byte(placesRouteWantSheet))
	tb := qacrServe(t, placesRouteRuleset(t))
	for _, role := range qacrRoles {
		got := decodedJSON(t, tb.readRuleset(t, role)["sheet"])
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads sheet %v, want %v", role, got, want)
		}
	}
}
