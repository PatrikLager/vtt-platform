package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

type byRoleTable struct {
	t      *testing.T
	c      *campaign.Campaign
	url    string
	tokens map[string]string
	ids    map[string]string
	seq    int
}

// newByRoleTable serves dnd45e-minimal over a campaign in which lera controls a
// fighter and a scout of different sheets, bo a bare actor whose sheet names
// none, and a goblin belongs to nobody.
func newByRoleTable(t *testing.T) *byRoleTable {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })
	tb := &byRoleTable{t: t, c: c, tokens: map[string]string{}, ids: map[string]string{}}
	for name, role := range map[string]identity.Role{"dm": identity.RoleDM, "agent": identity.RoleAgent,
		"lera": identity.RolePlayer, "bo": identity.RolePlayer, "watcher": identity.RoleSpectator} {
		tok, id, err := ids.CreateInvite(name, role)
		if err != nil {
			t.Fatal(err)
		}
		tb.tokens[name], tb.ids[name] = tok, id
	}
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	for _, a := range []*vttv1.Actor{
		{ActorId: "fighter", Name: "Fighter", Kind: party, AbilityIds: []string{"rally", "longsword-strike"}},
		{ActorId: "scout", Name: "Scout", Kind: party, AbilityIds: []string{"crossbow-shot", "rally"}},
		{ActorId: "goblin", Name: "Goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, AbilityIds: []string{"goblin-scimitar"}},
		{ActorId: "bare", Name: "Bare", Kind: party},
	} {
		tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}})
	}
	tb.grant("fighter", "lera")
	tb.grant("scout", "lera")
	tb.grant("bare", "bo")
	hs := httptest.NewServer(gateway.New(c, ids).WithRuleset(loadDnd45eMinimal(t)).Handler())
	t.Cleanup(hs.Close)
	tb.url = hs.URL
	return tb
}

func (tb *byRoleTable) append(env *vttv1.Envelope) {
	tb.t.Helper()
	tb.seq++
	env.EventId = fmt.Sprintf("by-role-%d", tb.seq)
	if _, err := tb.c.Append(env); err != nil {
		tb.t.Fatal(err)
	}
}

func (tb *byRoleTable) grant(actor, who string) {
	tb.t.Helper()
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: tb.ids[who], Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}})
}

type byRoleRuleset struct {
	Abilities  []struct{ ID string }  `json:"abilities"`
	Conditions []struct{ ID string }  `json:"conditions"`
	Resources  []string               `json:"resources"`
	Properties []struct{ Key string } `json:"properties"`
}

func (tb *byRoleTable) read(who string) byRoleRuleset {
	tb.t.Helper()
	req, err := http.NewRequest(http.MethodGet, tb.url+"/api/ruleset", nil)
	if err != nil {
		tb.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tb.tokens[who])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tb.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		tb.t.Fatalf("%s: status %d: %s", who, resp.StatusCode, body)
	}
	var out byRoleRuleset
	if err := json.Unmarshal(body, &out); err != nil {
		tb.t.Fatal(err)
	}
	return out
}

func abilityIDsOf(rs byRoleRuleset) []string {
	out := []string{}
	for _, a := range rs.Abilities {
		out = append(out, a.ID)
	}
	return out
}

// VTT-691 VTT-118
func TestAPlayerReadsTheAbilitiesTheirCharactersSheetsName(t *testing.T) {
	tb := newByRoleTable(t)
	if got, want := abilityIDsOf(tb.read("lera")), []string{"crossbow-shot", "longsword-strike", "rally"}; !slices.Equal(got, want) {
		t.Errorf("lera reads %q, want the union of her characters' sheets in id order, %q", got, want)
	}
	if got := abilityIDsOf(tb.read("bo")); len(got) != 0 {
		t.Errorf("bo, whose character names none, reads %q", got)
	}
}

// VTT-691
func TestAPlayersReadFollowsAGrantAndARevocation(t *testing.T) {
	tb := newByRoleTable(t)
	tb.grant("goblin", "bo")
	if got, want := abilityIDsOf(tb.read("bo")), []string{"goblin-scimitar"}; !slices.Equal(got, want) {
		t.Errorf("after the grant bo reads %q, want %q", got, want)
	}
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: "goblin", ParticipantId: tb.ids["bo"]}}})
	if got := abilityIDsOf(tb.read("bo")); len(got) != 0 {
		t.Errorf("after the revocation bo reads %q", got)
	}
}

// VTT-692
func TestASpectatorReadsNoAbility(t *testing.T) {
	tb := newByRoleTable(t)
	if got := abilityIDsOf(tb.read("watcher")); len(got) != 0 {
		t.Errorf("the spectator reads %q", got)
	}
}

// VTT-693
func TestTheDMAndTheAgentReadEveryAbility(t *testing.T) {
	tb := newByRoleTable(t)
	want := []string{}
	for id := range loadDnd45eMinimal(t).Compiled {
		want = append(want, id)
	}
	slices.Sort(want)
	for _, who := range []string{"dm", "agent"} {
		if got := abilityIDsOf(tb.read(who)); !slices.Equal(got, want) {
			t.Errorf("%s reads %q, want every ability %q", who, got, want)
		}
	}
}

// VTT-619 VTT-631 VTT-710
func TestEveryRoleReadsTheSameConditionsResourcesAndProperties(t *testing.T) {
	tb := newByRoleTable(t)
	dm := tb.read("dm")
	if len(dm.Conditions) == 0 || len(dm.Resources) == 0 || len(dm.Properties) == 0 {
		t.Fatalf("fixture: the DM reads %+v", dm)
	}
	for _, who := range []string{"agent", "lera", "bo", "watcher"} {
		got := tb.read(who)
		if !slices.Equal(got.Conditions, dm.Conditions) || !slices.Equal(got.Resources, dm.Resources) ||
			!slices.Equal(got.Properties, dm.Properties) {
			t.Errorf("%s reads %+v, the DM %+v", who, got, dm)
		}
	}
}
