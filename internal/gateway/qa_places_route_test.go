package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var qaprRoles = []string{"dm", "agent", "player", "spectator"}

type qaprPlace struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Forms       []string `json:"forms"`
	Copies      bool     `json:"copies"`
	States      []string `json:"states"`
}

type qaprTable struct {
	url    string
	tokens map[string]string
}

const qaprProperties = `[
  {"key": "sierra", "type": "number", "name": "Sierra",
   "description": "A number no form lists."},
  {"key": "quebec", "type": "number", "name": "Quebec",
   "description": "A number the zulu form lists."},
  {"key": "romeo", "type": "number", "name": "Romeo",
   "description": "A number the yankee form lists."}
]`

const qaprForms = `[
  {"key": "zulu", "name": "Zulu kind", "description": "Things of the zulu kind.",
   "properties": ["quebec"]},
  {"key": "yankee", "name": "Yankee kind",
   "description": "Things of the yankee kind.", "properties": ["romeo"]}
]`

const qaprPlaces = `[
  {"key": "mike", "name": "Satchel", "description": "Carried on the back.",
   "forms": ["zulu", "yankee"], "copies": true, "states": ["oscar", "lima"]},
  {"key": "november", "name": "Strongbox",
   "description": "Locked away at home.", "forms": ["yankee"]},
  {"key": "kilo", "name": "Belt", "description": "Hung at the waist.",
   "forms": ["zulu"], "copies": false, "states": ["lima"]}
]`

var qaprWantPlaces = []qaprPlace{
	{Key: "mike", Name: "Satchel", Description: "Carried on the back.",
		Forms: []string{"zulu", "yankee"}, Copies: true,
		States: []string{"oscar", "lima"}},
	{Key: "november", Name: "Strongbox", Description: "Locked away at home.",
		Forms: []string{"yankee"}, Copies: false, States: []string{}},
	{Key: "kilo", Name: "Belt", Description: "Hung at the waist.",
		Forms: []string{"zulu"}, Copies: false, States: []string{"lima"}},
}

func qaprManifest(extra string) string {
	return `{"id": "qa-places", "name": "QA Places", "format_version": "3",
  "version": "1", "properties": ` + qaprProperties + `,
  "forms": ` + qaprForms + extra + `}`
}

func qaprLoad(t *testing.T, manifest string) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.json")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	guide := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(guide, []byte("# QA places\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("fixture ruleset does not load: %v", err)
	}
	return rs
}

func qaprStamp(env *vttv1.Envelope, seq int, by string) *vttv1.Envelope {
	env.EventId = fmt.Sprintf("qapr-event-%d", seq)
	env.ParticipantId = by
	env.ActorRole = "dm"
	env.OccurredAt = timestamppb.Now()
	return env
}

func qaprServe(t *testing.T, rs *rules.Ruleset) qaprTable {
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
	stored := map[string]identity.Role{
		"dm": identity.RoleDM, "agent": identity.RoleAgent,
		"player": identity.RoleSpectator, "spectator": identity.RoleSpectator,
	}
	pids := map[string]string{}
	for _, role := range qaprRoles {
		tok, id, err := ids.CreateInvite("qapr-"+role, stored[role])
		if err != nil {
			t.Fatal(err)
		}
		tokens[role], pids[role] = tok, id
	}
	added := &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "qapr-actor", Name: "Qapr",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}},
	}}
	granted := &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "qapr-actor", ParticipantId: pids["player"],
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		},
	}}
	for i, env := range []*vttv1.Envelope{added, granted} {
		if _, err := c.Append(qaprStamp(env, i, pids["dm"])); err != nil {
			t.Fatal(err)
		}
	}

	s := gateway.New(c, ids)
	if rs != nil {
		s = s.WithRuleset(rs)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	tab := qaprTable{url: srv.URL, tokens: tokens}
	for _, role := range qaprRoles {
		var me struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(qaprGet(t, tab, role, "/api/me"), &me); err != nil {
			t.Fatal(err)
		}
		if me.Role != role {
			t.Fatalf("fixture: the %s seat reads its role as %q", role, me.Role)
		}
	}
	return tab
}

func qaprGet(t *testing.T, tab qaprTable, role, path string) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, tab.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tab.tokens[role])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: status %d, body %s", role, path, resp.StatusCode, body)
	}
	return body
}

func qaprTop(t *testing.T, tab qaprTable, role string) map[string]json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(qaprGet(t, tab, role, "/api/ruleset"), &top); err != nil {
		t.Fatal(err)
	}
	return top
}

func qaprPlacesOf(t *testing.T, top map[string]json.RawMessage) []qaprPlace {
	t.Helper()
	raw, ok := top["places"]
	if !ok {
		t.Fatalf("no places field in %v", qaprKeys(top))
	}
	if string(raw) == "null" {
		t.Fatal("places is null")
	}
	var places []qaprPlace
	if err := json.Unmarshal(raw, &places); err != nil {
		t.Fatal(err)
	}
	return places
}

func qaprKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// VTT-1052.
func TestQAPREveryRoleReadsThePlacesInDeclaredOrder(t *testing.T) {
	tab := qaprServe(t, qaprLoad(t, qaprManifest(`, "places": `+qaprPlaces)))
	for _, role := range qaprRoles {
		got := qaprPlacesOf(t, qaprTop(t, tab, role))
		if !reflect.DeepEqual(got, qaprWantPlaces) {
			t.Errorf("%s reads places\n got %+v\nwant %+v", role, got, qaprWantPlaces)
		}
	}
}

// VTT-1052.
func TestQAPREachPlaceCarriesExactlyItsSixFields(t *testing.T) {
	tab := qaprServe(t, qaprLoad(t, qaprManifest(`, "places": `+qaprPlaces)))
	want := []string{"copies", "description", "forms", "key", "name", "states"}
	for _, role := range qaprRoles {
		var places []map[string]json.RawMessage
		if err := json.Unmarshal(qaprTop(t, tab, role)["places"], &places); err != nil {
			t.Fatal(err)
		}
		if len(places) != len(qaprWantPlaces) {
			t.Fatalf("%s reads %d places", role, len(places))
		}
		for i, p := range places {
			if got := qaprKeys(p); !slices.Equal(got, want) {
				t.Errorf("%s: place %d carries %v, want %v", role, i, got, want)
			}
			if string(p["states"]) == "null" || string(p["forms"]) == "null" {
				t.Errorf("%s: place %d carries a null list: %s %s",
					role, i, p["forms"], p["states"])
			}
		}
		if got := string(places[1]["copies"]); got != "false" {
			t.Errorf("%s: a place that holds an entry once has copies %s", role, got)
		}
		if got := string(places[0]["copies"]); got != "true" {
			t.Errorf("%s: a place that holds copies has copies %s", role, got)
		}
		if got := string(places[1]["states"]); got != "[]" {
			t.Errorf("%s: a place naming no state has states %s", role, got)
		}
	}
}

// VTT-1052.
func TestQAPRAPlaceWritingNoStatesInAnySpellingIsServedAnEmptyList(t *testing.T) {
	for _, states := range []string{``, `, "states": null`, `, "states": []`} {
		places := `[{"key": "november", "name": "Strongbox",
  "description": "Locked away at home.", "forms": ["yankee"], "copies": true` +
			states + `}]`
		rs := qaprLoad(t, qaprManifest(`, "places": `+places))
		tab := qaprServe(t, rs)
		for _, role := range qaprRoles {
			var got []map[string]json.RawMessage
			if err := json.Unmarshal(qaprTop(t, tab, role)["places"], &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 {
				t.Fatalf("states%q: %s reads %d places", states, role, len(got))
			}
			if s := string(got[0]["states"]); s != "[]" {
				t.Errorf("states%q: %s reads states %s, want []", states, role, s)
			}
			if s := string(got[0]["copies"]); s != "true" {
				t.Errorf("states%q: %s reads copies %s, want true", states, role, s)
			}
		}
	}
}

// VTT-1052.
func TestQAPRNoPlacesInAnySpellingIsAnEmptyListForEveryRole(t *testing.T) {
	for _, extra := range []string{``, `, "places": null`, `, "places": []`} {
		tab := qaprServe(t, qaprLoad(t, qaprManifest(extra)))
		for _, role := range qaprRoles {
			if got := string(qaprTop(t, tab, role)["places"]); got != "[]" {
				t.Errorf("places%q: %s reads places %s, want []", extra, role, got)
			}
		}
	}
}

// VTT-1052.
func TestQAPRNoRulesetIsAnEmptyListOfPlacesForEveryRole(t *testing.T) {
	tab := qaprServe(t, nil)
	for _, role := range qaprRoles {
		if got := string(qaprTop(t, tab, role)["places"]); got != "[]" {
			t.Errorf("%s reads places %s with no ruleset, want []", role, got)
		}
	}
}

// VTT-1052.
func TestQAPRThePlacesKeepTheirOrderOnEveryRead(t *testing.T) {
	tab := qaprServe(t, qaprLoad(t, qaprManifest(`, "places": `+qaprPlaces)))
	for range 20 {
		for _, role := range qaprRoles {
			got := qaprPlacesOf(t, qaprTop(t, tab, role))
			keys := make([]string, 0, len(got))
			for _, p := range got {
				keys = append(keys, p.Key)
			}
			if !slices.Equal(keys, []string{"mike", "november", "kilo"}) {
				t.Fatalf("%s reads the places in the order %v", role, keys)
			}
		}
	}
}

type qaprSection struct {
	Name   string                     `json:"name"`
	Kind   string                     `json:"kind"`
	Places []string                   `json:"places"`
	Raw    map[string]json.RawMessage `json:"-"`
}

type qaprPage struct {
	Name     string        `json:"name"`
	Sections []qaprSection `json:"sections"`
}

func qaprSheetOf(t *testing.T, top map[string]json.RawMessage) []qaprPage {
	t.Helper()
	var pages []qaprPage
	if err := json.Unmarshal(top["sheet"], &pages); err != nil {
		t.Fatal(err)
	}
	var raw []struct {
		Sections []map[string]json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(top["sheet"], &raw); err != nil {
		t.Fatal(err)
	}
	for i := range pages {
		for j := range pages[i].Sections {
			pages[i].Sections[j].Raw = raw[i].Sections[j]
		}
	}
	return pages
}

const qaprSheet = `, "sheet": [
  {"name": "Page one", "sections": [
    {"name": "Numbers", "values": ["sierra"]},
    {"name": "Carried", "places": ["november", "kilo", "mike"]}]}
]`

const qaprSheetOfTwo = `, "sheet": [
  {"name": "Page one", "sections": [
    {"name": "Numbers", "values": ["sierra"]},
    {"name": "Carried", "places": ["kilo", "november"]}]},
  {"name": "Page two", "sections": [
    {"name": "Marks", "conditions": true},
    {"name": "Kept", "places": ["mike"]}]}
]`

func qaprCheckPlacesSection(t *testing.T, role string, s qaprSection, name string,
	keys []string) {
	t.Helper()
	if s.Name != name || s.Kind != "places" || !slices.Equal(s.Places, keys) {
		t.Errorf("%s reads the section %+v, want %q of kind places with %v",
			role, s, name, keys)
	}
	if got := qaprKeys(s.Raw); !slices.Equal(got, []string{"kind", "name", "places"}) {
		t.Errorf("%s: the places section %q carries %v", role, name, got)
	}
}

// VTT-1053, VTT-831.
func TestQAPRASectionOfPlacesIsServedAsKindPlacesWithItsKeysInItsOrder(t *testing.T) {
	tab := qaprServe(t, qaprLoad(t, qaprManifest(`, "places": `+qaprPlaces+qaprSheet)))
	for _, role := range qaprRoles {
		pages := qaprSheetOf(t, qaprTop(t, tab, role))
		if len(pages) != 1 || len(pages[0].Sections) != 2 {
			t.Fatalf("%s reads the sheet %+v", role, pages)
		}
		if got := pages[0].Sections[0]; got.Kind != "values" {
			t.Errorf("%s reads the values section as kind %q", role, got.Kind)
		}
		qaprCheckPlacesSection(t, role, pages[0].Sections[1], "Carried",
			[]string{"november", "kilo", "mike"})
	}
}

// VTT-1053, VTT-831.
func TestQAPRTwoSectionsOfPlacesEachServeTheirOwnKeys(t *testing.T) {
	tab := qaprServe(t, qaprLoad(t,
		qaprManifest(`, "places": `+qaprPlaces+qaprSheetOfTwo)))
	for _, role := range qaprRoles {
		pages := qaprSheetOf(t, qaprTop(t, tab, role))
		if len(pages) != 2 || len(pages[0].Sections) != 2 ||
			len(pages[1].Sections) != 2 {
			t.Fatalf("%s reads the sheet %+v", role, pages)
		}
		qaprCheckPlacesSection(t, role, pages[0].Sections[1], "Carried",
			[]string{"kilo", "november"})
		qaprCheckPlacesSection(t, role, pages[1].Sections[1], "Kept",
			[]string{"mike"})
		if got := pages[1].Sections[0]; got.Kind != "conditions" || got.Places != nil {
			t.Errorf("%s reads the conditions section as %+v", role, got)
		}
		if _, ok := pages[0].Sections[0].Raw["places"]; ok {
			t.Errorf("%s: the values section carries places", role)
		}
	}
}

// VTT-1052, VTT-1053.
func TestQAPRTheCommittedFixtureServesThePlacesAndTheSectionItLoads(t *testing.T) {
	rs, err := rules.Load(filepath.Join("..", "rules", "testdata", "valid"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Places) < 2 {
		t.Fatalf("fixture: the committed ruleset loads %d places", len(rs.Places))
	}
	want := make([]qaprPlace, 0, len(rs.Places))
	for _, p := range rs.Places {
		states := p.States
		if states == nil {
			states = []string{}
		}
		want = append(want, qaprPlace{Key: p.Key, Name: p.Name,
			Description: p.Description, Forms: p.Forms, Copies: p.Copies,
			States: states})
	}
	var wantSections [][]string
	for _, page := range rs.Sheet {
		for _, s := range page.Sections {
			if len(s.Places) > 0 {
				wantSections = append(wantSections, s.Places)
			}
		}
	}
	if len(wantSections) == 0 {
		t.Fatal("fixture: the committed ruleset loads no section of places")
	}
	tab := qaprServe(t, rs)
	for _, role := range qaprRoles {
		top := qaprTop(t, tab, role)
		if got := qaprPlacesOf(t, top); !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads places\n got %+v\nwant %+v", role, got, want)
		}
		var gotSections [][]string
		for _, page := range qaprSheetOf(t, top) {
			for _, s := range page.Sections {
				if s.Kind == "places" {
					gotSections = append(gotSections, s.Places)
				}
			}
		}
		if !reflect.DeepEqual(gotSections, wantSections) {
			t.Errorf("%s reads sections of places %v, want %v",
				role, gotSections, wantSections)
		}
	}
}
