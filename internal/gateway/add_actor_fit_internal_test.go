package gateway

import (
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// fitServer serves a fresh campaign to a DM, with rs as its ruleset when rs
// is not nil.
func fitServer(t *testing.T, rs *rules.Ruleset) (*Server, *campaign.Campaign, *identity.Participant) {
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
	_, dmID, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	s := New(c, ids)
	if rs != nil {
		s = s.WithRuleset(rs)
	}
	return s, c, &identity.Participant{ID: dmID, Role: identity.RoleDM}
}

func fitRuleset() *rules.Ruleset {
	return &rules.Ruleset{Properties: []rules.Property{
		{Key: "brace", Type: rules.PropertyNumber},
		{Key: "focus", Type: rules.PropertyResource},
	}}
}

func fitAddActor(id string, attributes map[string]int32, resources map[string]*vttv1.Resource) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "add", Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: "Fit", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			Attributes: attributes, Resources: resources}}}}
}

// VTT-677
func TestAddActorWithARulesetRefusesAKeyTheRulesetDoesNotHave(t *testing.T) {
	pool := &vttv1.Resource{Current: 1, Max: 2}
	cases := []struct {
		name       string
		attributes map[string]int32
		resources  map[string]*vttv1.Resource
		want       string
	}{
		{"an undeclared attribute", map[string]int32{"brace": 1, "nope": 1}, nil,
			`actor "fit": attributes: references undeclared property "nope"`},
		{"a resource among the attributes", map[string]int32{"focus": 1}, nil,
			`actor "fit": attributes: gives resource property "focus" a number's value`},
		{"an undeclared resource", nil, map[string]*vttv1.Resource{"focus": pool, "nope": pool},
			`actor "fit": resources: references undeclared property "nope"`},
		{"a number among the resources", nil, map[string]*vttv1.Resource{"brace": pool},
			`actor "fit": resources: gives number property "brace" a resource's value`},
		{"the first key, attributes before resources, each in byte order of the keys",
			map[string]int32{"zz": 1, "yy": 1}, map[string]*vttv1.Resource{"aa": pool},
			`actor "fit": attributes: references undeclared property "yy"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, camp, dm := fitServer(t, fitRuleset())
			r := s.handleCommand(dm, fitAddActor("fit", c.attributes, c.resources), nil)
			if r.GetOk() {
				t.Fatal("add_actor accepted an actor holding a key the ruleset does not have")
			}
			if !strings.Contains(r.GetError(), c.want) {
				t.Errorf("error = %q, want it to contain %q", r.GetError(), c.want)
			}
			if _, added := camp.State().Actors["fit"]; added {
				t.Error("the refused actor was appended")
			}
		})
	}
}

// VTT-677
func TestAddActorWithARulesetAcceptsDeclaredKeys(t *testing.T) {
	s, camp, dm := fitServer(t, fitRuleset())
	r := s.handleCommand(dm, fitAddActor("fit", map[string]int32{"brace": 0},
		map[string]*vttv1.Resource{"focus": {Current: 0}}), nil)
	if !r.GetOk() {
		t.Fatalf("add_actor refused declared keys: %s", r.GetError())
	}
	if _, added := camp.State().Actors["fit"]; !added {
		t.Error("the accepted actor was not appended")
	}
}

// VTT-675
func TestAddActorWithNoRulesetReadsNoKeyAgainstOne(t *testing.T) {
	s, camp, dm := fitServer(t, nil)
	r := s.handleCommand(dm, fitAddActor("fit", map[string]int32{"nope": 1},
		map[string]*vttv1.Resource{"brace": {Current: 1}}), nil)
	if !r.GetOk() {
		t.Fatalf("add_actor with no ruleset served refused an actor for its keys: %s", r.GetError())
	}
	if _, added := camp.State().Actors["fit"]; !added {
		t.Error("the accepted actor was not appended")
	}
}

// Keep this test: the key check runs after validateAddActor, whose refusals
// teach the two-step and must stay what a misfit with a controller is told.
// VTT-677
func TestAddActorRefusesAControllerBeforeReadingKeys(t *testing.T) {
	s, _, dm := fitServer(t, fitRuleset())
	cmd := fitAddActor("fit", map[string]int32{"nope": 1}, nil)
	cmd.GetAddActor().GetActor().ControllerId = "p-2"
	r := s.handleCommand(dm, cmd, nil)
	if r.GetOk() || !strings.Contains(r.GetError(), "controller") || strings.Contains(r.GetError(), "nope") {
		t.Fatalf("error = %q, want the controller refusal and no key", r.GetError())
	}
}
