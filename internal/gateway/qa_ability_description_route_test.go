package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaDescSrc = "../../rulesets/dnd45e-minimal"

type qaDescTable struct {
	url    string
	tokens map[string]string
	ids    []string
	dir    string
}

func qaDescReadJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func qaDescWriteJSON(t *testing.T, path string, m map[string]any) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func qaDescCopy(t *testing.T) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "rs")
	if err := os.CopyFS(dst, os.DirFS(qaDescSrc)); err != nil {
		t.Fatal(err)
	}
	return dst
}

func qaDescAbilityIDs(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "abilities", "*.json"))
	if err != nil || len(files) < 3 {
		t.Fatalf("abilities: %v %v", files, err)
	}
	var ids []string
	for _, f := range files {
		ids = append(ids, qaDescReadJSON(t, f)["id"].(string))
	}
	slices.Sort(ids)
	return ids
}

func qaDescServe(t *testing.T, rsDir string) *qaDescTable {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	idb, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idb.Close() })
	tb := &qaDescTable{tokens: map[string]string{}, dir: rsDir}
	pids := map[string]string{}
	for name, role := range map[string]identity.Role{"dm": identity.RoleDM,
		"agent": identity.RoleAgent, "pa": identity.RolePlayer,
		"pb": identity.RolePlayer, "sp": identity.RoleSpectator} {
		tok, id, err := idb.CreateInvite(name, role)
		if err != nil {
			t.Fatal(err)
		}
		tb.tokens[name], pids[name] = tok, id
	}
	srv := gateway.New(c, idb)
	if rsDir != "" {
		tb.ids = qaDescAbilityIDs(t, rsDir)
		rs, err := rules.Load(rsDir)
		if err != nil {
			t.Fatal(err)
		}
		srv = srv.WithRuleset(rs)
	}
	n := 0
	app := func(env *vttv1.Envelope) {
		n++
		env.EventId = fmt.Sprintf("qa-desc-%d", n)
		if _, err := c.Append(env); err != nil {
			t.Fatal(err)
		}
	}
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	if rsDir != "" {
		for _, a := range []*vttv1.Actor{
			{ActorId: "ca", Name: "A", Kind: party, AbilityIds: tb.ids[0:2]},
			{ActorId: "cb", Name: "B", Kind: party},
			{ActorId: "cz", Name: "Z", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
				AbilityIds: tb.ids[2:3]},
		} {
			app(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
				ActorAdded: &vttv1.ActorAdded{Actor: a}}})
		}
		for actor, who := range map[string]string{"ca": "pa", "cb": "pb"} {
			app(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
				ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actor,
					ParticipantId: pids[who], Kind: party}}})
		}
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	tb.url = hs.URL
	return tb
}

func (tb *qaDescTable) read(t *testing.T, who string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, tb.url+"/api/ruleset", nil)
	req.Header.Set("Authorization", "Bearer "+tb.tokens[who])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: %d %s", who, resp.StatusCode, body)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func qaDescAbilities(t *testing.T, m map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	list, ok := m["abilities"].([]any)
	if !ok {
		t.Fatalf("abilities is not an array: %v", m["abilities"])
	}
	for _, a := range list {
		am := a.(map[string]any)
		out[am["id"].(string)] = am
	}
	return out
}

func qaDescFile(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	return qaDescReadJSON(t, filepath.Join(dir, "abilities", id+".json"))
}

var qaDescRoles = []string{"dm", "agent", "pa", "pb", "sp"}

// VTT-851
func TestQADescRouteEveryServedAbilityCarriesItsFilesDescription(t *testing.T) {
	tb := qaDescServe(t, qaDescSrc)
	want := map[string][]string{
		"dm": tb.ids, "agent": tb.ids, "pa": tb.ids[0:2], "pb": {}, "sp": {},
	}
	for _, who := range qaDescRoles {
		got := qaDescAbilities(t, tb.read(t, who))
		if len(got) != len(want[who]) {
			t.Fatalf("%s served %d abilities, want %d", who, len(got), len(want[who]))
		}
		for _, id := range want[who] {
			a, ok := got[id]
			if !ok {
				t.Fatalf("%s: %s not served", who, id)
			}
			d := qaDescFile(t, tb.dir, id)["description"]
			if d == nil || d == "" {
				t.Fatalf("fixture: %s has no description", id)
			}
			if a["description"] != d {
				t.Errorf("%s/%s: description %v, want %v", who, id, a["description"], d)
			}
		}
	}
}

// VTT-691
func TestQADescRouteAPlayerIsServedNoDescriptionOfAnAbilityNotServed(t *testing.T) {
	tb := qaDescServe(t, qaDescSrc)
	withheld := tb.ids[2]
	wd := qaDescFile(t, tb.dir, withheld)["description"].(string)
	for _, who := range []string{"pa", "pb", "sp"} {
		m := tb.read(t, who)
		if _, ok := qaDescAbilities(t, m)[withheld]; ok {
			t.Errorf("%s was served %s", who, withheld)
		}
		raw, _ := json.Marshal(m)
		if strings.Contains(string(raw), wd) {
			t.Errorf("%s: answer carries the withheld description", who)
		}
	}
}

// VTT-852 VTT-851
func TestQADescRouteAnAbilityWithoutADescriptionHasNoDescriptionKey(t *testing.T) {
	dir := qaDescCopy(t)
	ids := qaDescAbilityIDs(t, dir)
	bare := ids[0]
	p := filepath.Join(dir, "abilities", bare+".json")
	f := qaDescReadJSON(t, p)
	delete(f, "description")
	qaDescWriteJSON(t, p, f)
	tb := qaDescServe(t, dir)
	for _, who := range []string{"dm", "agent", "pa"} {
		got := qaDescAbilities(t, tb.read(t, who))
		a, ok := got[bare]
		if !ok {
			t.Fatalf("%s: %s not served", who, bare)
		}
		if _, has := a["description"]; has {
			t.Errorf("%s: %s has a description key: %v", who, bare, a["description"])
		}
		for _, id := range ids[1:] {
			b, ok := got[id]
			if !ok {
				continue
			}
			if b["description"] != qaDescFile(t, dir, id)["description"] {
				t.Errorf("%s/%s lost its description", who, id)
			}
		}
	}
}

// VTT-859
func TestQADescRouteFreeUsageNameIsTheRulesetsForEveryRole(t *testing.T) {
	tb := qaDescServe(t, qaDescSrc)
	rj := qaDescReadJSON(t, filepath.Join(qaDescSrc, "ruleset.json"))
	want := rj["free_usage_name"]
	if want == nil || want == "" {
		t.Fatal("fixture declares no free_usage_name")
	}
	for _, who := range qaDescRoles {
		if got := tb.read(t, who)["freeUsageName"]; got != want {
			t.Errorf("%s: freeUsageName %v, want %v", who, got, want)
		}
	}
}

// VTT-859
func TestQADescRouteFreeUsageNameIsAbsentWhenTheRulesetDeclaresNone(t *testing.T) {
	dir := qaDescCopy(t)
	p := filepath.Join(dir, "ruleset.json")
	r := qaDescReadJSON(t, p)
	delete(r, "free_usage_name")
	qaDescWriteJSON(t, p, r)
	tb := qaDescServe(t, dir)
	for _, who := range qaDescRoles {
		if v, has := tb.read(t, who)["freeUsageName"]; has {
			t.Errorf("%s: freeUsageName present: %v", who, v)
		}
	}
}

// VTT-859
func TestQADescRouteFreeUsageNameIsAbsentWhenNoRulesetIsLoaded(t *testing.T) {
	tb := qaDescServe(t, "")
	for _, who := range qaDescRoles {
		m := tb.read(t, who)
		if v, has := m["freeUsageName"]; has {
			t.Errorf("%s: freeUsageName present: %v", who, v)
		}
		if len(qaDescAbilities(t, m)) != 0 {
			t.Errorf("%s: abilities served with no ruleset", who)
		}
	}
}
