package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// declaredSheetAsRoute reads dir's ruleset.json's sheet and writes it in the
// shape /api/ruleset answers.
func declaredSheetAsRoute(t *testing.T, dir string) []any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ruleset.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Sheet []map[string]any `json:"sheet"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	out := []any{}
	for _, page := range m.Sheet {
		sections := []any{}
		for _, s := range page["sections"].([]any) {
			sec := s.(map[string]any)
			r := map[string]any{"name": sec["name"]}
			switch {
			case sec["values"] != nil:
				r["kind"], r["values"] = "values", sec["values"]
			case sec["conditions"] != nil:
				r["kind"] = "conditions"
			default:
				usages := []any{}
				for _, u := range sec["abilities"].([]any) {
					if word, ok := u.(string); ok && word == "at_will" {
						usages = append(usages, map[string]any{"kind": "atWill"})
						continue
					}
					usages = append(usages, map[string]any{"kind": "resource", "resource": u.(map[string]any)["spends"]})
				}
				r["kind"], r["abilities"] = "abilities", usages
			}
			sections = append(sections, r)
		}
		out = append(out, map[string]any{"name": page["name"], "sections": sections})
	}
	return out
}

func routeSheet(t *testing.T, f *metaFixture, token string) json.RawMessage {
	t.Helper()
	code, body := f.get("/api/ruleset", token)
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset status = %d, want 200", code)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	return got["sheet"]
}

// newTavernFixture is newMetaFixture serving tavern-brawl, a ruleset that
// declares no sheet.
func newTavernFixture(t *testing.T) *metaFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(campaign.LogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })
	tok, _, err := ids.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	f := &metaFixture{t: t, ids: ids, playerToken: tok}
	f.srv = httptest.NewServer(gateway.New(c, ids).WithRuleset(loadTavernBrawl(t)).Handler())
	t.Cleanup(f.srv.Close)
	return f
}

// VTT-831
func TestTheRulesetRouteAnswersEveryRoleTheDeclaredSheetInOrder(t *testing.T) {
	want := declaredSheetAsRoute(t, dnd45eMinimalDir(t))
	var pages []string
	for _, p := range want {
		pages = append(pages, p.(map[string]any)["name"].(string))
	}
	if len(pages) < 2 || slices.IsSorted(pages) {
		t.Fatalf("the ruleset must declare two pages out of name order to tell a sort apart; pages %v", pages)
	}
	f := newMetaFixture(t, true)
	for role, token := range map[string]string{"dm": f.dmToken, "agent": f.agentToken, "player": f.playerToken, "spectator": f.spectatorToken} {
		t.Run(role, func(t *testing.T) {
			var got []any
			if err := json.Unmarshal(routeSheet(t, f, token), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("sheet = %v, want %v", got, want)
			}
		})
	}
}

// VTT-832
func TestTheRulesetRouteAnswersNoSheetAsAnEmptyList(t *testing.T) {
	for name, f := range map[string]*metaFixture{"no ruleset": newMetaFixture(t, false), "tavern-brawl": newTavernFixture(t)} {
		t.Run(name, func(t *testing.T) {
			if got := routeSheet(t, f, f.playerToken); string(got) != "[]" {
				t.Errorf("sheet = %s, want []", got)
			}
		})
	}
}
