package gateway_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var qaRRRoles = []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator}

var qaRRRulesetDirs = []string{"../../rulesets/tavern-brawl", "../../rulesets/dnd45e-minimal"}

type qaRRTable struct {
	url    string
	ids    *identity.DB
	tokens map[identity.Role]string
	pids   map[identity.Role]string
}

func qaRRStart(t *testing.T, rs *rules.Ruleset, configure func(*gateway.Server)) *qaRRTable {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = c.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	tbl := &qaRRTable{ids: ids, tokens: map[identity.Role]string{}, pids: map[identity.Role]string{}}
	for _, r := range qaRRRoles {
		tok, id, err := ids.CreateInvite("qa-"+string(r), r)
		if err != nil {
			t.Fatalf("CreateInvite(%s): %v", r, err)
		}
		tbl.tokens[r], tbl.pids[r] = tok, id
	}
	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	if configure != nil {
		configure(srv)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hs.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	tbl.url = hs.URL
	return tbl
}

func qaRRGetWithHeader(t *testing.T, url, authorization string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, body
}

func qaRRGet(t *testing.T, url, token string) (int, []byte) {
	t.Helper()
	return qaRRGetWithHeader(t, url, "Bearer "+token)
}

func qaRRLoad(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load(%s): %v", dir, err)
	}
	return rs
}

// Reversing a list declared in descending key order sorts it;
// TestQARRPropertyFixturesAreNotInKeyOrder guards that.
func qaRRReversedCopy(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("CopyFS: %v", err)
	}
	manifest := filepath.Join(dst, "ruleset.json")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	var props []json.RawMessage
	if err := json.Unmarshal(doc["properties"], &props); err != nil {
		t.Fatalf("decode properties: %v", err)
	}
	slices.Reverse(props)
	if doc["properties"], err = json.Marshal(props); err != nil {
		t.Fatalf("encode properties: %v", err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(manifest, out, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dst
}

func qaRRFixtures(t *testing.T) map[string]*rules.Ruleset {
	t.Helper()
	out := map[string]*rules.Ruleset{}
	for _, dir := range qaRRRulesetDirs {
		out[filepath.Base(dir)] = qaRRLoad(t, dir)
		out[filepath.Base(dir)+"-reversed"] = qaRRLoad(t, qaRRReversedCopy(t, dir))
	}
	return out
}

func qaRRResourceKeys(rs *rules.Ruleset) []string {
	keys := []string{}
	for _, p := range rs.Properties {
		if p.Type == rules.PropertyResource {
			keys = append(keys, p.Key)
		}
	}
	return keys
}

func qaRRPropertyKeys(rs *rules.Ruleset) []string {
	keys := []string{}
	for _, p := range rs.Properties {
		keys = append(keys, p.Key)
	}
	return keys
}

func qaRRObject(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		t.Fatalf("body is not a JSON object (%v): %s", err, body)
	}
	return obj
}

func qaRREmptyList(t *testing.T, obj map[string]json.RawMessage, field string) {
	t.Helper()
	raw, ok := obj[field]
	if !ok {
		t.Errorf("%q absent, want []", field)
		return
	}
	if got := string(bytes.TrimSpace(raw)); got != "[]" {
		t.Errorf("%q = %s, want []", field, got)
	}
}

func qaRRCheckProperties(t *testing.T, rs *rules.Ruleset, raw json.RawMessage) {
	t.Helper()
	var got []map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("properties: %v in %s", err, raw)
	}
	if len(got) != len(rs.Properties) {
		t.Fatalf("properties: %d entries, ruleset declares %d", len(got), len(rs.Properties))
	}
	for i, p := range rs.Properties {
		want := map[string]any{"key": p.Key, "type": p.Type, "name": p.Name, "description": p.Description}
		for field, v := range want {
			if got[i][field] != v {
				t.Errorf("properties[%d].%s = %v, want %v", i, field, got[i][field], v)
			}
		}
	}
}

func qaRRCheckResources(t *testing.T, rs *rules.Ruleset, raw json.RawMessage) {
	t.Helper()
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil || got == nil {
		t.Fatalf("resources not a list of strings (%v): %s", err, raw)
	}
	if want := qaRRResourceKeys(rs); !reflect.DeepEqual(got, want) {
		t.Errorf("resources = %v, want %v", got, want)
	}
}

func qaRRCheckAbilities(t *testing.T, rs *rules.Ruleset, raw json.RawMessage) {
	t.Helper()
	var got []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("abilities: %v in %s", err, raw)
	}
	if len(got) != len(rs.Compiled) {
		t.Errorf("abilities: %d entries, ruleset has %d", len(got), len(rs.Compiled))
	}
	seen := map[string]bool{}
	for _, entry := range got {
		for _, f := range []string{"id", "name", "range", "maxTargets", "usage"} {
			if _, ok := entry[f]; !ok {
				t.Errorf("ability entry %v lacks %q", entry, f)
			}
		}
		var id, name string
		var rng, maxTargets int
		_ = json.Unmarshal(entry["id"], &id)
		_ = json.Unmarshal(entry["name"], &name)
		_ = json.Unmarshal(entry["range"], &rng)
		_ = json.Unmarshal(entry["maxTargets"], &maxTargets)
		cp, ok := rs.Compiled[id]
		if !ok || seen[id] {
			t.Errorf("ability %q: unknown or repeated", id)
			continue
		}
		seen[id] = true
		if name != cp.Name || rng != cp.Targeting.Range || maxTargets != cp.Targeting.MaxTargets {
			t.Errorf("ability %q = (%q, %d, %d), want (%q, %d, %d)",
				id, name, rng, maxTargets, cp.Name, cp.Targeting.Range, cp.Targeting.MaxTargets)
		}
	}
}

func qaRRCheckConditions(t *testing.T, rs *rules.Ruleset, raw json.RawMessage) {
	t.Helper()
	var got []map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("conditions: %v in %s", err, raw)
	}
	if len(got) != len(rs.Conditions) {
		t.Errorf("conditions: %d entries, ruleset has %d", len(got), len(rs.Conditions))
	}
	for _, entry := range got {
		c, ok := rs.Conditions[entry["id"]]
		if !ok {
			t.Errorf("condition %q not in the ruleset", entry["id"])
			continue
		}
		want := map[string]string{"id": c.ID, "name": c.Name, "description": c.Description}
		if !reflect.DeepEqual(entry, want) {
			t.Errorf("condition entry %v, want %v", entry, want)
		}
	}
}

func qaRRCheckBody(t *testing.T, rs *rules.Ruleset, body []byte) {
	t.Helper()
	obj := qaRRObject(t, body)
	var id, name string
	_ = json.Unmarshal(obj["id"], &id)
	_ = json.Unmarshal(obj["name"], &name)
	if id != rs.ID || name != rs.Name {
		t.Errorf("id, name = %q, %q; want %q, %q", id, name, rs.ID, rs.Name)
	}
	qaRRCheckAbilities(t, rs, obj["abilities"])
	qaRRCheckConditions(t, rs, obj["conditions"])
	qaRRCheckResources(t, rs, obj["resources"])
	qaRRCheckProperties(t, rs, obj["properties"])
}

// VTT-632 VTT-710
func TestQARRRulesetRouteAnswersEveryRole(t *testing.T) {
	for name, rs := range qaRRFixtures(t) {
		tbl := qaRRStart(t, rs, nil)
		for _, role := range qaRRRoles {
			code, body := qaRRGet(t, tbl.url+"/api/ruleset", tbl.tokens[role])
			if code != http.StatusOK {
				t.Errorf("%s as %s: status %d, want 200: %s", name, role, code, body)
				continue
			}
			if role == identity.RoleDM || role == identity.RoleAgent {
				qaRRCheckBody(t, rs, body)
				continue
			}
			obj := qaRRObject(t, body)
			if string(obj["abilities"]) != "[]" {
				t.Errorf("%s as %s: abilities %s, want []", name, role, obj["abilities"])
			}
			qaRRCheckConditions(t, rs, obj["conditions"])
			qaRRCheckResources(t, rs, obj["resources"])
			qaRRCheckProperties(t, rs, obj["properties"])
		}
	}
}

// VTT-619
func TestQARRPropertiesAreTheRulesetsInItsOrder(t *testing.T) {
	for name, rs := range qaRRFixtures(t) {
		tbl := qaRRStart(t, rs, nil)
		code, body := qaRRGet(t, tbl.url+"/api/ruleset", tbl.tokens[identity.RolePlayer])
		if code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", name, code, body)
		}
		qaRRCheckProperties(t, rs, qaRRObject(t, body)["properties"])
	}
}

// VTT-619
func TestQARRPropertyFixturesAreNotInKeyOrder(t *testing.T) {
	var unsortedProps, unsortedResources, resourceFirst int
	for _, rs := range qaRRFixtures(t) {
		if !slices.IsSorted(qaRRPropertyKeys(rs)) {
			unsortedProps++
		}
		if res := qaRRResourceKeys(rs); len(res) > 1 && !slices.IsSorted(res) {
			unsortedResources++
		}
		if len(rs.Properties) > 0 && rs.Properties[0].Type == rules.PropertyResource {
			resourceFirst++
		}
	}
	if unsortedProps == 0 || unsortedResources == 0 || resourceFirst == 0 {
		t.Fatalf("degenerate fixtures: unsorted properties %d, unsorted resources %d, resource-first %d",
			unsortedProps, unsortedResources, resourceFirst)
	}
}

// VTT-631
func TestQARRResourcesAreResourceKeysInDeclaredOrder(t *testing.T) {
	for name, rs := range qaRRFixtures(t) {
		if len(qaRRResourceKeys(rs)) < 2 {
			t.Fatalf("%s: fixture declares fewer than two resources", name)
		}
		tbl := qaRRStart(t, rs, nil)
		code, body := qaRRGet(t, tbl.url+"/api/ruleset", tbl.tokens[identity.RoleSpectator])
		if code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", name, code, body)
		}
		qaRRCheckResources(t, rs, qaRRObject(t, body)["resources"])
	}
}

// VTT-114 VTT-620
func TestQARRNothingLoadedAnswersEmptyLists(t *testing.T) {
	tbl := qaRRStart(t, nil, nil)
	code, body := qaRRGet(t, tbl.url+"/api/ruleset", tbl.tokens[identity.RolePlayer])
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset: status %d: %s", code, body)
	}
	obj := qaRRObject(t, body)
	for _, f := range []string{"abilities", "conditions", "resources", "properties"} {
		qaRREmptyList(t, obj, f)
	}
	for route, field := range map[string]string{"/api/adventures": "adventures", "/api/maps": "maps"} {
		code, body := qaRRGet(t, tbl.url+route, tbl.tokens[identity.RolePlayer])
		if code != http.StatusOK {
			t.Errorf("%s: status %d: %s", route, code, body)
			continue
		}
		qaRREmptyList(t, qaRRObject(t, body), field)
	}
}

// VTT-581
func TestQARRRulesetRouteAnswers401WithoutAnAcceptedToken(t *testing.T) {
	rs := qaRRLoad(t, qaRRRulesetDirs[0])
	tbl := qaRRStart(t, rs, nil)
	if err := tbl.ids.Revoke(tbl.pids[identity.RolePlayer]); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	refused := map[string]string{
		"no header":      "",
		"empty bearer":   "Bearer ",
		"other scheme":   "Basic " + tbl.tokens[identity.RoleDM],
		"unknown token":  "Bearer qa-not-a-token-anyone-was-given",
		"revoked token":  "Bearer " + tbl.tokens[identity.RolePlayer],
		"token, no word": tbl.tokens[identity.RoleDM],
	}
	for name, header := range refused {
		code, body := qaRRGetWithHeader(t, tbl.url+"/api/ruleset", header)
		if code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401: %s", name, code, body)
		}
		if bytes.Contains(body, []byte(rs.Properties[0].Description)) {
			t.Errorf("%s: refused answer carries the ruleset", name)
		}
	}
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RoleSpectator} {
		if code, body := qaRRGet(t, tbl.url+"/api/ruleset", tbl.tokens[role]); code != http.StatusOK {
			t.Errorf("%s beside a revoked player: status %d: %s", role, code, body)
		}
	}
}

// VTT-709
func TestQARRMetadataRoutesAnswerEveryRole(t *testing.T) {
	rs := qaRRLoad(t, qaRRRulesetDirs[0])
	art := t.TempDir()
	artBytes := []byte("qa-rr art bytes")
	if err := os.WriteFile(filepath.Join(art, "qa-rr-art.png"), artBytes, 0o600); err != nil {
		t.Fatalf("write art: %v", err)
	}
	tbl := qaRRStart(t, rs, func(s *gateway.Server) { s.WithArtDir(art) })
	routes := []string{"/api/adventures", "/api/maps", "/api/art/qa-rr-art.png"}
	for _, role := range qaRRRoles {
		for _, route := range routes {
			code, body := qaRRGet(t, tbl.url+route, tbl.tokens[role])
			if code != http.StatusOK || len(body) == 0 {
				t.Errorf("%s as %s: status %d, body %q", route, role, code, body)
			}
			if strings.HasPrefix(route, "/api/art/") && !bytes.Equal(body, artBytes) {
				t.Errorf("%s as %s: body %q, want the art file", route, role, body)
			}
		}
	}
}
