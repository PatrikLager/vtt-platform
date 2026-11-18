package gateway_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const shippedDnd45eMinimal = "../../rulesets/dnd45e-minimal"

func declaredDescriptions(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "abilities", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no ability under %s: %v", dir, err)
	}
	out := map[string]string{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var a struct {
			ID          string `json:"id"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[a.ID] = a.Description
	}
	return out
}

func servedRuleset(t *testing.T, url, token string) map[string]json.RawMessage {
	t.Helper()
	code, body := qaRRGet(t, url+"/api/ruleset", token)
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset: status %d: %s", code, body)
	}
	return qaRRObject(t, body)
}

func servedAbilities(t *testing.T, url, token string) []map[string]json.RawMessage {
	t.Helper()
	var abilities []map[string]json.RawMessage
	if err := json.Unmarshal(servedRuleset(t, url, token)["abilities"], &abilities); err != nil {
		t.Fatal(err)
	}
	return abilities
}

func rawString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("%s is not a string: %v", raw, err)
	}
	return s
}

// VTT-851
func TestEveryRoleReadsTheDescriptionOfEachAbilityItIsServed(t *testing.T) {
	tb := newByRoleTable(t)
	want := declaredDescriptions(t, shippedDnd45eMinimal)
	for _, who := range []string{"dm", "agent", "lera", "bo", "watcher"} {
		abilities := servedAbilities(t, tb.url, tb.tokens[who])
		if (who == "dm" || who == "agent" || who == "lera") && len(abilities) == 0 {
			t.Fatalf("fixture: %s is served no ability", who)
		}
		for _, a := range abilities {
			id := rawString(t, a["id"])
			raw, has := a["description"]
			if !has || want[id] == "" || rawString(t, raw) != want[id] {
				t.Errorf("%s reads %s's description %s, its file declares %q", who, id, raw, want[id])
			}
		}
	}
}

// VTT-852
func TestAnAbilityWhoseRulesetDeclaresNoDescriptionIsAnsweredWithoutOne(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dnd45e-minimal")
	if err := os.CopyFS(dir, os.DirFS(shippedDnd45eMinimal)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "abilities", "rally.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, has := doc["description"]; !has {
		t.Fatal("fixture: rally.json declares no description to remove")
	}
	delete(doc, "description")
	if raw, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	tbl := qaRRStart(t, qaRRLoad(t, dir), nil)
	found, described := false, 0
	for _, a := range servedAbilities(t, tbl.url, tbl.tokens[identity.RoleDM]) {
		_, has := a["description"]
		if rawString(t, a["id"]) == "rally" {
			found = true
			if has {
				t.Errorf("rally is answered with description %s, its file declares none", a["description"])
			}
			continue
		}
		if has {
			described++
		}
	}
	if !found || described == 0 {
		t.Fatalf("fixture: rally served %v, described others %d", found, described)
	}
}

// VTT-859
func TestEveryRoleReadsTheRulesetsNameForAnAbilityThatSpendsNoResource(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(shippedDnd45eMinimal, "ruleset.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		FreeUsageName string `json:"free_usage_name"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FreeUsageName == "" {
		t.Fatal("fixture: dnd45e-minimal names no ability that spends no resource")
	}
	tb := newByRoleTable(t)
	for _, who := range []string{"dm", "agent", "lera", "bo", "watcher"} {
		got, has := servedRuleset(t, tb.url, tb.tokens[who])["freeUsageName"]
		if !has || rawString(t, got) != manifest.FreeUsageName {
			t.Errorf("%s reads freeUsageName %s, the ruleset names %q", who, got, manifest.FreeUsageName)
		}
	}
}

// VTT-859
func TestNoRoleReadsAFreeUsageNameWhenTheRulesetGivesNoneOrNoneIsLoaded(t *testing.T) {
	for name, rs := range map[string]*rules.Ruleset{"tavern-brawl": loadTavernBrawl(t), "nothing loaded": nil} {
		if rs != nil && rs.FreeUsageName != "" {
			t.Fatalf("fixture: %s names %q", name, rs.FreeUsageName)
		}
		tbl := qaRRStart(t, rs, nil)
		for _, role := range qaRRRoles {
			if got, has := servedRuleset(t, tbl.url, tbl.tokens[role])["freeUsageName"]; has {
				t.Errorf("%s as %s reads freeUsageName %s", name, role, got)
			}
		}
	}
}
