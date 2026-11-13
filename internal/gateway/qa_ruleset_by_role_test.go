package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaReadRulesetDir = "../../rulesets/dnd45e-minimal"

type qaReadTable struct {
	t      *testing.T
	camp   *campaign.Campaign
	ids    *identity.DB
	srv    *httptest.Server
	client *http.Client
	n      int
	conns  *atomic.Int32
	tokens map[string]string
	pids   map[string]string
}

func qaReadLoadRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(qaReadRulesetDir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaReadOpen(t *testing.T, rs *rules.Ruleset, configure func(*gateway.Server) *gateway.Server) *qaReadTable {
	t.Helper()
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	gw := gateway.New(c, ids)
	if rs != nil {
		gw = gw.WithRuleset(rs)
	}
	if configure != nil {
		gw = configure(gw)
	}
	conns := &atomic.Int32{}
	srv := httptest.NewUnstartedServer(gw.Handler())
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return &qaReadTable{
		t: t, camp: c, ids: ids, srv: srv, client: srv.Client(), conns: conns,
		tokens: map[string]string{}, pids: map[string]string{},
	}
}

func (q *qaReadTable) qaReadInvite(name string, role identity.Role) {
	q.t.Helper()
	tok, id, err := q.ids.CreateInvite(name, role)
	if err != nil {
		q.t.Fatalf("CreateInvite(%s): %v", name, err)
	}
	q.tokens[name] = tok
	q.pids[name] = id
}

func (q *qaReadTable) qaReadAppend(env *vttv1.Envelope) {
	q.t.Helper()
	q.n++
	env.EventId = fmt.Sprintf("qa-read-event-%d", q.n)
	if _, err := q.camp.Append(env); err != nil {
		q.t.Fatalf("Append: %v", err)
	}
}

func (q *qaReadTable) qaReadAddActor(id string, kind vttv1.ActorKind, abilityIDs ...string) {
	q.t.Helper()
	q.qaReadAppend(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: "QA " + id, Kind: kind, AbilityIds: abilityIDs},
	}}})
}

func (q *qaReadTable) qaReadGrant(actorID, who string, kind vttv1.ActorKind) {
	q.t.Helper()
	q.qaReadAppend(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actorID, ParticipantId: q.pids[who], Kind: kind,
	}}})
}

func (q *qaReadTable) qaReadRevoke(actorID, who string) {
	q.t.Helper()
	q.qaReadAppend(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actorID, ParticipantId: q.pids[who],
	}}})
}

func (q *qaReadTable) qaReadGetWith(path, authorization string) (int, []byte) {
	q.t.Helper()
	req, err := http.NewRequest(http.MethodGet, q.srv.URL+path, nil)
	if err != nil {
		q.t.Fatalf("NewRequest: %v", err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		q.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		q.t.Fatalf("read %s: %v", path, err)
	}
	return resp.StatusCode, body
}

func (q *qaReadTable) qaReadGet(path, who string) (int, []byte) {
	q.t.Helper()
	return q.qaReadGetWith(path, "Bearer "+q.tokens[who])
}

func (q *qaReadTable) qaReadRuleset(who string) map[string]json.RawMessage {
	q.t.Helper()
	status, body := q.qaReadGet("/api/ruleset", who)
	if status != http.StatusOK {
		q.t.Fatalf("%s: GET /api/ruleset = %d %s, want 200", who, status, body)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(body, &out); err != nil {
		q.t.Fatalf("%s: decode %s: %v", who, body, err)
	}
	return out
}

func qaReadAbilityIDs(t *testing.T, who string, rs map[string]json.RawMessage) []string {
	t.Helper()
	raw, ok := rs["abilities"]
	if !ok || string(raw) == "null" {
		t.Fatalf("%s: abilities is %q, want a non-null list", who, raw)
	}
	var abilities []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &abilities); err != nil {
		t.Fatalf("%s: decode abilities %s: %v", who, raw, err)
	}
	ids := []string{}
	for _, a := range abilities {
		ids = append(ids, a.ID)
	}
	return ids
}

func (q *qaReadTable) qaReadWantAbilities(who string, want ...string) {
	q.t.Helper()
	if want == nil {
		want = []string{}
	}
	got := qaReadAbilityIDs(q.t, who, q.qaReadRuleset(who))
	if !slices.Equal(got, want) {
		q.t.Errorf("%s reads abilities %v, want %v", who, got, want)
	}
}

func qaReadEveryAbility(rs *rules.Ruleset) []string {
	var all []string
	for id := range rs.Compiled {
		all = append(all, id)
	}
	slices.Sort(all)
	return all
}

// VTT-691 VTT-118
func TestQAReadAPlayerReadsTheUnionOverTheActorsTheyControlInIdOrder(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-dm", identity.RoleDM)
	q.qaReadInvite("qa-player-one", identity.RolePlayer)
	q.qaReadInvite("qa-player-two", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "staggering-blow", "longsword-strike")
	q.qaReadAddActor("qa-pet", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadAddActor("qa-charmed", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "goblin-shortbow", "crossbow-shot")
	q.qaReadAddActor("qa-other", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "hunters-flurry")
	q.qaReadAddActor("qa-goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "goblin-scimitar")
	q.qaReadGrant("qa-hero", "qa-player-one", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-pet", "qa-player-one", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-charmed", "qa-player-one", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	q.qaReadGrant("qa-other", "qa-player-two", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-player-one", "crossbow-shot", "goblin-shortbow", "longsword-strike", "rally", "staggering-blow")
	q.qaReadWantAbilities("qa-player-two", "hunters-flurry")
}

// VTT-691
func TestQAReadANonPartyActorAddedAsSuchAndGrantedCounts(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadAddActor("qa-wolf", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "goblin-scimitar")
	q.qaReadAddActor("qa-unkind", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, "rally")
	q.qaReadGrant("qa-wolf", "qa-player", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)
	q.qaReadGrant("qa-unkind", "qa-player", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)
	q.qaReadWantAbilities("qa-player", "goblin-scimitar", "rally")
}

// VTT-691
func TestQAReadAPlayerControllingNoActorReadsNone(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadInvite("qa-busy", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally", "crossbow-shot")
	q.qaReadAddActor("qa-unowned", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "hunters-flurry")
	q.qaReadGrant("qa-hero", "qa-busy", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-player")
	q.qaReadWantAbilities("qa-busy", "crossbow-shot", "rally")
}

// VTT-691
func TestQAReadSharedControlGivesEachControllerTheSharedSheet(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-first", identity.RolePlayer)
	q.qaReadInvite("qa-second", identity.RolePlayer)
	q.qaReadAddActor("qa-shared", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadAddActor("qa-own", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "hunters-flurry")
	q.qaReadGrant("qa-shared", "qa-first", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-shared", "qa-second", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-own", "qa-first", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-first", "hunters-flurry", "rally")
	q.qaReadWantAbilities("qa-second", "rally")
	q.qaReadRevoke("qa-shared", "qa-first")
	q.qaReadWantAbilities("qa-first", "hunters-flurry")
	q.qaReadWantAbilities("qa-second", "rally")
}

// VTT-691
func TestQAReadAGrantAndARevocationReachTheNextReadOnTheSameConnection(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "staggering-blow")
	q.qaReadAddActor("qa-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "crossbow-shot")
	q.qaReadWantAbilities("qa-player")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-player", "staggering-blow")
	q.qaReadGrant("qa-pet", "qa-player", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	q.qaReadWantAbilities("qa-player", "crossbow-shot", "staggering-blow")
	q.qaReadRevoke("qa-hero", "qa-player")
	q.qaReadWantAbilities("qa-player", "crossbow-shot")
	q.qaReadRevoke("qa-pet", "qa-player")
	q.qaReadWantAbilities("qa-player")
	if n := q.conns.Load(); n != 1 {
		t.Errorf("the reads opened %d connections, want 1", n)
	}
}

// VTT-691
func TestQAReadASheetAbilityTheRulesetDoesNotHoldIsNotInvented(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	q := qaReadOpen(t, rs, nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadInvite("qa-dm", identity.RoleDM)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "qa-not-in-the-ruleset", "rally", "Rally")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-player", "rally")
	q.qaReadWantAbilities("qa-dm", qaReadEveryAbility(rs)...)
}

// List an ability once however many controlled sheets name it, and however often.
func TestQAReadAnAbilityNamedTwiceIsReadOnce(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally", "rally", "crossbow-shot")
	q.qaReadAddActor("qa-pet", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "crossbow-shot")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-pet", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-player", "crossbow-shot", "rally")
}

// VTT-691
func TestQAReadAPlayersAbilityIsTheSameEntryTheDMReads(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadInvite("qa-dm", identity.RoleDM)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "hunters-flurry", "crossbow-shot")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	var dmAbilities, playerAbilities []map[string]any
	if err := json.Unmarshal(q.qaReadRuleset("qa-dm")["abilities"], &dmAbilities); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(q.qaReadRuleset("qa-player")["abilities"], &playerAbilities); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, a := range dmAbilities {
		byID[a["id"].(string)] = a
	}
	if len(playerAbilities) != 2 {
		t.Fatalf("player reads %d abilities, want 2", len(playerAbilities))
	}
	for _, a := range playerAbilities {
		if !reflect.DeepEqual(a, byID[a["id"].(string)]) {
			t.Errorf("player's entry %v differs from the DM's %v", a, byID[a["id"].(string)])
		}
	}
}

// VTT-692
func TestQAReadASpectatorReadsNoAbilityEvenWhereItsIdControlsAnActor(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-watcher", identity.RoleSpectator)
	q.qaReadInvite("qa-demoted", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally", "crossbow-shot")
	q.qaReadGrant("qa-hero", "qa-demoted", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadGrant("qa-hero", "qa-watcher", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-watcher")
	q.qaReadWantAbilities("qa-demoted", "crossbow-shot", "rally")
	if err := q.ids.SetRole(q.pids["qa-demoted"], identity.RoleSpectator); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	q.qaReadWantAbilities("qa-demoted")
}

// VTT-693 VTT-118
func TestQAReadTheDMAndTheAgentReadEveryAbilityInIdOrder(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	q := qaReadOpen(t, rs, nil)
	q.qaReadInvite("qa-dm", identity.RoleDM)
	q.qaReadInvite("qa-agent", identity.RoleAgent)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-dm", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	every := qaReadEveryAbility(rs)
	if len(every) < 3 {
		t.Fatalf("fixture ruleset holds %v, too few to show a filter", every)
	}
	q.qaReadWantAbilities("qa-dm", every...)
	q.qaReadWantAbilities("qa-agent", every...)
}

// VTT-693
func TestQAReadAPlayerPromotedToDMReadsEveryAbilityAtTheNextRead(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	q := qaReadOpen(t, rs, nil)
	q.qaReadInvite("qa-rising", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-rising", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	q.qaReadWantAbilities("qa-rising", "rally")
	if err := q.ids.SetRole(q.pids["qa-rising"], identity.RoleDM); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	q.qaReadWantAbilities("qa-rising", qaReadEveryAbility(rs)...)
}

func qaReadEveryRole(q *qaReadTable) []string {
	q.qaReadInvite("qa-dm", identity.RoleDM)
	q.qaReadInvite("qa-agent", identity.RoleAgent)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadInvite("qa-idle", identity.RolePlayer)
	q.qaReadInvite("qa-watcher", identity.RoleSpectator)
	return []string{"qa-dm", "qa-agent", "qa-player", "qa-idle", "qa-watcher"}
}

// VTT-710
func TestQAReadEveryRoleReadsEveryCondition(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	q := qaReadOpen(t, rs, nil)
	roles := qaReadEveryRole(q)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	var want []string
	for id := range rs.Conditions {
		want = append(want, id)
	}
	slices.Sort(want)
	if len(want) < 2 {
		t.Fatalf("fixture ruleset holds conditions %v, too few", want)
	}
	for _, who := range roles {
		var conds []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(q.qaReadRuleset(who)["conditions"], &conds); err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		var got []string
		for _, c := range conds {
			got = append(got, c.ID)
			if c.Name != rs.Conditions[c.ID].Name || c.Description != rs.Conditions[c.ID].Description {
				t.Errorf("%s: condition %s reads %q/%q", who, c.ID, c.Name, c.Description)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s reads conditions %v, want %v", who, got, want)
		}
	}
}

// VTT-619 VTT-631
func TestQAReadEveryRoleReadsTheSameResourcesAndProperties(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	q := qaReadOpen(t, rs, nil)
	roles := qaReadEveryRole(q)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	wantResources := []string{}
	var wantKeys []string
	for _, p := range rs.Properties {
		wantKeys = append(wantKeys, p.Key)
		if p.Resource != nil {
			wantResources = append(wantResources, p.Key)
		}
	}
	first := q.qaReadRuleset("qa-dm")
	for _, who := range roles {
		got := q.qaReadRuleset(who)
		var resources []string
		if err := json.Unmarshal(got["resources"], &resources); err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		if !slices.Equal(resources, wantResources) {
			t.Errorf("%s reads resources %v, want %v", who, resources, wantResources)
		}
		var props []struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(got["properties"], &props); err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		var keys []string
		for _, p := range props {
			keys = append(keys, p.Key)
		}
		if !slices.Equal(keys, wantKeys) {
			t.Errorf("%s reads property keys %v, want %v", who, keys, wantKeys)
		}
		for _, field := range []string{"id", "name", "conditions", "resources", "properties"} {
			if string(got[field]) != string(first[field]) {
				t.Errorf("%s reads %s %s, the DM reads %s", who, field, got[field], first[field])
			}
		}
	}
}

// VTT-632 VTT-114 VTT-620
func TestQAReadNothingLoadedAnswersEveryRoleEmptyListsWhateverItControls(t *testing.T) {
	q := qaReadOpen(t, nil, nil)
	roles := qaReadEveryRole(q)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	for _, who := range roles {
		got := q.qaReadRuleset(who)
		for _, field := range []string{"abilities", "resources", "properties"} {
			if string(got[field]) != "[]" {
				t.Errorf("%s: %s is %s, want []", who, field, got[field])
			}
		}
	}
}

// VTT-581
func TestQAReadTheRulesetRouteAnswers401BeforeItLooksAtControl(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-player", identity.RolePlayer)
	q.qaReadInvite("qa-gone", identity.RolePlayer)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-gone", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	if err := q.ids.Revoke(q.pids["qa-gone"]); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	cases := map[string]string{
		"no header":     "",
		"no prefix":     q.tokens["qa-player"],
		"basic scheme":  "Basic " + q.tokens["qa-player"],
		"unknown token": "Bearer qa-read-not-a-token",
		"revoked token": "Bearer " + q.tokens["qa-gone"],
	}
	for _, path := range []string{"/api/ruleset", "/api/me", "/api/art/qa-read-piece.png"} {
		for name, auth := range cases {
			status, body := q.qaReadGetWith(path, auth)
			if status != http.StatusUnauthorized {
				t.Errorf("%s with %s = %d %s, want 401", path, name, status, body)
			}
			if strings.Contains(string(body), "rally") {
				t.Errorf("%s with %s carries an ability: %s", path, name, body)
			}
		}
	}
}

// Keep the guide route behind the bearer check: a request with no accepted
// token is 401, never 403.
func TestQAReadTheRulesetGuideAnswers401BeforeTheRole(t *testing.T) {
	q := qaReadOpen(t, qaReadLoadRuleset(t), nil)
	q.qaReadInvite("qa-gone", identity.RoleDM)
	if err := q.ids.Revoke(q.pids["qa-gone"]); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	for name, auth := range map[string]string{
		"no header":     "",
		"unknown token": "Bearer qa-read-not-a-token",
		"revoked token": "Bearer " + q.tokens["qa-gone"],
	} {
		if status, body := q.qaReadGetWith("/api/ruleset/guide", auth); status != http.StatusUnauthorized {
			t.Errorf("guide with %s = %d %s, want 401", name, status, body)
		}
	}
}

// VTT-702
func TestQAReadTheRulesetGuideByRoleWithAGuideLoaded(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	if len(rs.Guide) < 40 {
		t.Fatalf("fixture guide is too short to recognise: %q", rs.Guide)
	}
	q := qaReadOpen(t, rs, nil)
	qaReadEveryRole(q)
	q.qaReadAddActor("qa-hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally")
	q.qaReadGrant("qa-hero", "qa-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
	for _, who := range []string{"qa-dm", "qa-agent"} {
		status, body := q.qaReadGet("/api/ruleset/guide", who)
		var got struct {
			Guide string `json:"guide"`
		}
		if status != http.StatusOK || json.Unmarshal(body, &got) != nil || got.Guide != rs.Guide {
			t.Errorf("%s: guide = %d, want 200 with the ruleset's guide", who, status)
		}
	}
	snippet := strings.TrimSpace(rs.Guide)[:40]
	for _, who := range []string{"qa-player", "qa-idle", "qa-watcher"} {
		status, body := q.qaReadGet("/api/ruleset/guide", who)
		if status != http.StatusForbidden {
			t.Errorf("%s: guide = %d, want 403", who, status)
		}
		if strings.TrimSpace(string(body)) != "gateway: not authorized" {
			t.Errorf("%s: guide refusal body %q", who, body)
		}
		if strings.Contains(string(body), snippet) || strings.Contains(string(body), "guide\"") {
			t.Errorf("%s: refusal carries the guide: %s", who, body)
		}
	}
}

// VTT-702 VTT-711
func TestQAReadTheRulesetGuideByRoleWithNoGuide(t *testing.T) {
	empty := qaReadLoadRuleset(t)
	empty.Guide = ""
	for name, rs := range map[string]*rules.Ruleset{"no ruleset": nil, "an empty guide": empty} {
		q := qaReadOpen(t, rs, nil)
		qaReadEveryRole(q)
		for _, who := range []string{"qa-dm", "qa-agent"} {
			if status, body := q.qaReadGet("/api/ruleset/guide", who); status != http.StatusNotFound {
				t.Errorf("%s, %s: guide = %d %s, want 404", name, who, status, body)
			}
		}
		for _, who := range []string{"qa-player", "qa-idle", "qa-watcher"} {
			status, body := q.qaReadGet("/api/ruleset/guide", who)
			if status != http.StatusForbidden || strings.TrimSpace(string(body)) != "gateway: not authorized" {
				t.Errorf("%s, %s: guide = %d %q, want 403 gateway: not authorized", name, who, status, body)
			}
		}
	}
}

func qaReadWithAdventure(t *testing.T, rs *rules.Ruleset, artDir string) func(*gateway.Server) *gateway.Server {
	t.Helper()
	adv, err := adventure.Load("../../adventures/goblin-ambush", rs)
	if err != nil {
		t.Fatalf("adventure.Load: %v", err)
	}
	return func(s *gateway.Server) *gateway.Server {
		return s.WithAdventures(map[string]*adventure.Adventure{adv.ID: adv}).
			WithAdventureGuides(map[string]string{adv.ID: "QA adventure guide text"}).
			WithArtDir(artDir)
	}
}

// VTT-709
func TestQAReadTheAdventuresMapsAndArtAreAnsweredToEveryRole(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	artDir := t.TempDir()
	piece := []byte("\x89PNG\r\n\x1a\nqa-read-piece")
	if err := os.WriteFile(filepath.Join(artDir, "qa-read-piece.png"), piece, 0o600); err != nil {
		t.Fatal(err)
	}
	q := qaReadOpen(t, rs, qaReadWithAdventure(t, rs, artDir))
	for _, who := range qaReadEveryRole(q) {
		status, body := q.qaReadGet("/api/adventures", who)
		if status != http.StatusOK || !strings.Contains(string(body), "goblin-ambush") {
			t.Errorf("%s: adventures = %d %s", who, status, body)
		}
		status, body = q.qaReadGet("/api/maps", who)
		if status != http.StatusOK || !strings.Contains(string(body), "\"maps\":[]") {
			t.Errorf("%s: maps = %d %s", who, status, body)
		}
		status, body = q.qaReadGet("/api/art/qa-read-piece.png", who)
		if status != http.StatusOK || string(body) != string(piece) {
			t.Errorf("%s: art = %d %q", who, status, body)
		}
	}
}

// VTT-116 VTT-711
func TestQAReadTheAdventureGuideStaysTheDMsAndAnUnknownOneIs404(t *testing.T) {
	rs := qaReadLoadRuleset(t)
	q := qaReadOpen(t, rs, qaReadWithAdventure(t, rs, t.TempDir()))
	qaReadEveryRole(q)
	for _, who := range []string{"qa-dm", "qa-agent"} {
		status, body := q.qaReadGet("/api/adventures/goblin-ambush/guide", who)
		if status != http.StatusOK || !strings.Contains(string(body), "QA adventure guide text") {
			t.Errorf("%s: adventure guide = %d %s", who, status, body)
		}
		if status, body := q.qaReadGet("/api/adventures/qa-no-such/guide", who); status != http.StatusNotFound {
			t.Errorf("%s: unknown adventure guide = %d %s, want 404", who, status, body)
		}
	}
	for _, who := range []string{"qa-player", "qa-watcher"} {
		status, body := q.qaReadGet("/api/adventures/goblin-ambush/guide", who)
		if status != http.StatusForbidden || strings.Contains(string(body), "QA adventure guide text") {
			t.Errorf("%s: adventure guide = %d %s, want 403 with no guide", who, status, body)
		}
	}
}
