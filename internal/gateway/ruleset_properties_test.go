package gateway_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// declaredKeys reads, from dir's ruleset.json itself, the keys of the
// properties of type typ, or of every property when typ is "", in file order.
func declaredKeys(t *testing.T, dir, typ string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ruleset.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Properties []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range m.Properties {
		if typ == "" || p.Type == typ {
			keys = append(keys, p.Key)
		}
	}
	return keys
}

// VTT-631
func TestTheRulesetRouteListsTheResourcePropertiesKeysInDeclaredOrder(t *testing.T) {
	f := newMetaFixture(t, true)
	code, body := f.get("/api/ruleset", f.playerToken)
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset status = %d, want 200", code)
	}
	var got struct {
		Resources []string `json:"resources"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	want := declaredKeys(t, dnd45eMinimalDir(t), "resource")
	numbers := len(declaredKeys(t, dnd45eMinimalDir(t), "number"))
	if len(want) < 2 || numbers == 0 || slices.IsSorted(want) {
		t.Fatalf("the ruleset must declare numbers and two resources out of key order to tell a filter or a sort apart; resources %v, numbers %d", want, numbers)
	}
	if !slices.Equal(got.Resources, want) {
		t.Errorf("resources = %v, want %v", got.Resources, want)
	}
}

type rulesetRouteProperty struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// VTT-619
func TestTheRulesetRouteAnswersEveryPropertyInTheRulesetsWords(t *testing.T) {
	f := newMetaFixture(t, true)
	code, body := f.get("/api/ruleset", f.playerToken)
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset status = %d, want 200", code)
	}
	var got struct {
		Properties []rulesetRouteProperty `json:"properties"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	var want []rulesetRouteProperty
	for _, p := range loadDnd45eMinimal(t).Properties {
		want = append(want, rulesetRouteProperty{Key: p.Key, Type: p.Type, Name: p.Name, Description: p.Description})
	}
	if !slices.Equal(got.Properties, want) {
		t.Errorf("properties =\n %v\nwant\n %v", got.Properties, want)
	}
	keys := declaredKeys(t, dnd45eMinimalDir(t), "")
	if slices.IsSorted(keys) {
		t.Fatalf("the ruleset declares its properties in key order %v, so a sort could not be told apart", keys)
	}
	var served []string
	for _, p := range got.Properties {
		served = append(served, p.Key)
	}
	if !slices.Equal(served, keys) {
		t.Errorf("properties are served in the order %v, want ruleset.json's %v", served, keys)
	}
}

// VTT-620
func TestTheRulesetRouteAnswersNoPropertiesAsAnEmptyList(t *testing.T) {
	f := newMetaFixture(t, false)
	code, body := f.get("/api/ruleset", f.playerToken)
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset status = %d, want 200", code)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if string(got["properties"]) != "[]" {
		t.Errorf("properties = %s, want []", got["properties"])
	}
}
