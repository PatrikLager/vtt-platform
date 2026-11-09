package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
)

// writeBraceGuard rewrites testdata/valid's brace-guard.json in a copy of the
// adventure, its attributes and resources replaced by the JSON given.
func writeBraceGuard(t *testing.T, attributes, resources string) string {
	t.Helper()
	dir := copyFixtureDirExcluding(t, "testdata/valid")
	path := filepath.Join(dir, "actors", "brace-guard.json")
	body := `{"actor_id": "brace-guard", "name": "Brace Guard", "kind": "non_party",
		"attributes": ` + attributes + `, "resources": ` + resources + `}`
	if !json.Valid([]byte(body)) {
		t.Fatalf("test input is not JSON: %s", body)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// VTT-670
func TestAnActorValueWrittenNullOrACurrentLeftOutIsRefused(t *testing.T) {
	rs := loadFixtureRuleset(t)
	cases := []struct {
		name, attributes, resources, field string
	}{
		{"a number written null",
			`{"vim": null, "vigor": 10, "brace": 14}`, `{"focus": {"current": 10, "max": 10}}`,
			`field "attributes.vim"`},
		{"a resource written null",
			`{"vim": 10, "vigor": 10, "brace": 14}`, `{"focus": null}`,
			`field "resources.focus"`},
		{"a current written null",
			`{"vim": 10, "vigor": 10, "brace": 14}`, `{"focus": {"current": null, "max": 10}}`,
			`field "resources.focus.current"`},
		{"a current left out",
			`{"vim": 10, "vigor": 10, "brace": 14}`, `{"focus": {"max": 10}}`,
			`field "resources.focus.current"`},
		{"a resource written empty",
			`{"vim": 10, "vigor": 10, "brace": 14}`, `{"focus": {}}`,
			`field "resources.focus.current"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := adventure.Load(writeBraceGuard(t, c.attributes, c.resources), rs)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			for _, want := range []string{"brace-guard.json", c.field} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
}

// VTT-670
func TestAnActorValueOfZeroAndAResourceWithNoMaximumLoad(t *testing.T) {
	rs := loadFixtureRuleset(t)
	cases := []struct {
		name, resources string
		want            adventure.ResourceVal
	}{
		{"a current of 0", `{"focus": {"current": 0, "max": 10}}`, adventure.ResourceVal{Current: 0, Max: 10}},
		{"a maximum left out", `{"focus": {"current": 4}}`, adventure.ResourceVal{Current: 4, Max: 0}},
		{"a maximum written null", `{"focus": {"current": 4, "max": null}}`, adventure.ResourceVal{Current: 4, Max: 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			adv, err := adventure.Load(writeBraceGuard(t, `{"vim": 0, "vigor": 10, "brace": 14}`, c.resources), rs)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			for _, a := range adv.Actors {
				if a.ID != "brace-guard" {
					continue
				}
				if v, ok := a.Attributes["vim"]; !ok || v != 0 {
					t.Errorf("vim = %d, %v; want 0, held", v, ok)
				}
				if got := a.Resources["focus"]; got != c.want {
					t.Errorf("focus = %+v, want %+v", got, c.want)
				}
				return
			}
			t.Fatal("brace-guard was not loaded")
		})
	}
}

func TestAnActorsFirstUndeclaredKeyIsNamedOnRepeatedLoads(t *testing.T) {
	rs := loadFixtureRuleset(t)
	dir := writeBraceGuard(t, `{"zeta": 1, "alpha": 1, "mid": 1}`, `{"focus": {"current": 1, "max": 1}}`)
	for range 8 {
		_, err := adventure.Load(dir, rs)
		if err == nil || !strings.Contains(err.Error(), `references undeclared property "alpha"`) {
			t.Fatalf("error = %v, want the undeclared property \"alpha\" named", err)
		}
	}
}
