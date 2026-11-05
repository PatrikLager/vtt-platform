package gateway_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

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
	var want []string
	numbers := 0
	for _, p := range loadDnd45eMinimal(t).Properties {
		if p.Type == rules.PropertyResource {
			want = append(want, p.Key)
		} else {
			numbers++
		}
	}
	if len(want) < 2 || numbers == 0 || slices.IsSorted(want) {
		t.Fatalf("the ruleset must declare numbers and two resources out of key order to tell a filter or a sort apart; resources %v, numbers %d", want, numbers)
	}
	if !slices.Equal(got.Resources, want) {
		t.Errorf("resources = %v, want %v", got.Resources, want)
	}
}
