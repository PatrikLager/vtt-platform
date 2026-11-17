package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var qaSheetRoles = []string{"dm", "agent", "player", "spectator"}

type qaSheetServer struct {
	url    string
	tokens map[string]string
}

func qaSheetStart(t *testing.T, rs *rules.Ruleset) qaSheetServer {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	tokens := map[string]string{}
	for _, name := range qaSheetRoles {
		role, err := identity.ParseRole(name)
		if err != nil {
			t.Fatal(err)
		}
		tok, _, err := ids.CreateInvite("qa-"+name, role)
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = tok
	}
	s := gateway.New(c, ids)
	if rs != nil {
		s = s.WithRuleset(rs)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return qaSheetServer{url: srv.URL, tokens: tokens}
}

func (s qaSheetServer) body(t *testing.T, role string) map[string]json.RawMessage {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.url+"/api/ruleset", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+s.tokens[role])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", role, resp.StatusCode)
	}
	var m map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func qaSheetWriteRuleset(t *testing.T, sheet any) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	m := map[string]any{
		"id": "qa-sheet", "name": "QA Sheet", "format_version": "3",
		"properties": []any{
			map[string]any{"key": "zeta", "type": "number", "name": "Zeta", "description": "z"},
			map[string]any{"key": "alpha", "type": "number", "name": "Alpha", "description": "a"},
			map[string]any{"key": "mu", "type": "number", "name": "Mu", "description": "m"},
			map[string]any{"key": "pool", "type": "resource", "name": "Pool", "description": "p", "default_max_expr": "2"},
			map[string]any{"key": "stock", "type": "resource", "name": "Stock", "description": "s", "default_max_expr": "2"},
		},
	}
	if sheet != nil {
		m["sheet"] = sheet
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ruleset.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func qaSheetDecode(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("sheet %s: %v", raw, err)
	}
	return out
}

// qaSheetExpect writes a declared sheet in the route's words.
func qaSheetExpect(declared []any) []any {
	var pages []any
	for _, p := range declared {
		page := p.(map[string]any)
		var secs []any
		for _, s := range page["sections"].([]any) {
			sec := s.(map[string]any)
			out := map[string]any{"name": sec["name"]}
			switch {
			case sec["values"] != nil:
				out["kind"] = "values"
				out["values"] = sec["values"]
			case sec["conditions"] != nil:
				out["kind"] = "conditions"
			default:
				out["kind"] = "abilities"
				var us []any
				for _, u := range sec["abilities"].([]any) {
					if str, ok := u.(string); ok && str == "at_will" {
						us = append(us, map[string]any{"kind": "atWill"})
						continue
					}
					us = append(us, map[string]any{"kind": "resource", "resource": u.(map[string]any)["spends"]})
				}
				out["abilities"] = us
			}
			secs = append(secs, out)
		}
		pages = append(pages, map[string]any{"name": page["name"], "sections": secs})
	}
	return pages
}

func qaSheetCompare(t *testing.T, role string, raw json.RawMessage, want []any) {
	t.Helper()
	var got any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	wantNorm, _ := json.Marshal(want)
	var w any
	_ = json.Unmarshal(wantNorm, &w)
	if !reflect.DeepEqual(got, w) {
		t.Errorf("%s: sheet\n got %s\nwant %s", role, raw, wantNorm)
	}
}

// VTT-831
func TestQASheetRouteServesCommittedSheetToEveryRole(t *testing.T) {
	rs, err := rules.Load("../../rulesets/dnd45e-minimal")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Sheet []any `json:"sheet"`
	}
	raw, err := os.ReadFile("../../rulesets/dnd45e-minimal/ruleset.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Sheet) < 2 {
		t.Fatal("fixture declares no sheet")
	}
	srv := qaSheetStart(t, rs)
	for _, role := range qaSheetRoles {
		m := srv.body(t, role)
		qaSheetCompare(t, role, m["sheet"], qaSheetExpect(manifest.Sheet))
	}
}

// VTT-831
func TestQASheetRouteKeepsDeclaredOrderAndShapes(t *testing.T) {
	declared := []any{
		map[string]any{"name": "Zulu page", "sections": []any{
			map[string]any{"name": "Yankee", "values": []any{"zeta", "mu", "alpha"}},
			map[string]any{"name": "Bravo", "abilities": []any{
				map[string]any{"spends": "stock"}, "at_will", map[string]any{"spends": "pool"},
			}},
			map[string]any{"name": "Alpha", "conditions": true},
		}},
		map[string]any{"name": "Beta page", "sections": []any{
			map[string]any{"name": "Quebec", "values": []any{"pool"}},
		}},
	}
	srv := qaSheetStart(t, qaSheetWriteRuleset(t, declared))
	for _, role := range qaSheetRoles {
		m := srv.body(t, role)
		qaSheetCompare(t, role, m["sheet"], qaSheetExpect(declared))
		pages := qaSheetDecode(t, m["sheet"])
		for _, p := range pages {
			if len(p) != 2 {
				t.Errorf("%s: page keys %v", role, p)
			}
			for _, s := range p["sections"].([]any) {
				sec := s.(map[string]any)
				if sec["kind"] == "conditions" && len(sec) != 2 {
					t.Errorf("%s: conditions section carries %v", role, sec)
				}
			}
		}
	}
}

// VTT-832
func TestQASheetRouteIsEmptyListWithNoRulesetOrNoSheet(t *testing.T) {
	brawl, err := rules.Load("../../rulesets/tavern-brawl")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]*rules.Ruleset{
		"no ruleset":    nil,
		"tavern-brawl":  brawl,
		"own, no sheet": qaSheetWriteRuleset(t, nil),
	}
	for name, rs := range cases {
		srv := qaSheetStart(t, rs)
		for _, role := range qaSheetRoles {
			m := srv.body(t, role)
			raw, ok := m["sheet"]
			if !ok {
				t.Errorf("%s/%s: no sheet key", name, role)
				continue
			}
			if string(raw) != "[]" {
				t.Errorf("%s/%s: sheet = %s", name, role, raw)
			}
		}
	}
}
