package gateway

import (
	"slices"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// abilityFitRuleset is fitRuleset holding the abilities named.
func abilityFitRuleset(ids ...string) *rules.Ruleset {
	rs := fitRuleset()
	rs.Compiled = map[string]*rules.CompiledPower{}
	for _, id := range ids {
		rs.Compiled[id] = &rules.CompiledPower{}
	}
	return rs
}

func abilityAddActor(id string, attributes map[string]int32, abilityIDs ...string) *vttv1.ClientCommand {
	cmd := fitAddActor(id, attributes, nil)
	cmd.GetAddActor().GetActor().AbilityIds = abilityIDs
	return cmd
}

// VTT-683
func TestAddActorAppendsTheAbilityIdsItNamesInTheirOrder(t *testing.T) {
	s, camp, dm := fitServer(t, abilityFitRuleset("fists", "rally"))
	r := s.handleCommand(dm, abilityAddActor("fit", nil, "rally", "fists", "rally"), nil)
	if !r.GetOk() {
		t.Fatalf("add_actor refused abilities the ruleset holds: %s", r.GetError())
	}
	if got, want := camp.State().Actors["fit"].GetAbilityIds(), []string{"rally", "fists", "rally"}; !slices.Equal(got, want) {
		t.Errorf("the appended actor names %q, want %q", got, want)
	}
}

// VTT-685
func TestAddActorWithARulesetRefusesAnAbilityTheRulesetDoesNotHold(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
		key  string
	}{
		{"an id the ruleset does not hold", []string{"fists", "nope"}, "nope"},
		{"the first such id in byte order, upper case before lower", []string{"zz", "b", "B", "fists"}, "B"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, camp, dm := fitServer(t, abilityFitRuleset("fists"))
			r := s.handleCommand(dm, abilityAddActor("fit", nil, c.ids...), nil)
			if r.GetOk() {
				t.Fatal("add_actor accepted an actor naming an ability the ruleset does not hold")
			}
			want := `gateway: add_actor: actor "fit": ability_ids: names ability "` + c.key + `", which the ruleset does not hold`
			if r.GetError() != want {
				t.Errorf("error = %q, want %q", r.GetError(), want)
			}
			if _, added := camp.State().Actors["fit"]; added {
				t.Error("the refused actor was appended")
			}
		})
	}
}

// VTT-685
func TestAddActorReadsAnActorsKeysBeforeItsAbilities(t *testing.T) {
	s, _, dm := fitServer(t, abilityFitRuleset())
	r := s.handleCommand(dm, abilityAddActor("fit", map[string]int32{"nope": 1}, "missing"), nil)
	if r.GetOk() {
		t.Fatal("add_actor accepted an actor holding an undeclared key")
	}
	if !strings.Contains(r.GetError(), `attributes: references undeclared property "nope"`) ||
		strings.Contains(r.GetError(), "missing") {
		t.Errorf("error = %q, want the key's refusal and not the ability's", r.GetError())
	}
}

// VTT-686
func TestAddActorWithNoRulesetReadsNoAbilityAgainstOne(t *testing.T) {
	s, camp, dm := fitServer(t, nil)
	r := s.handleCommand(dm, abilityAddActor("fit", nil, "nope", "fists"), nil)
	if !r.GetOk() {
		t.Fatalf("add_actor with no ruleset served refused an actor for its abilities: %s", r.GetError())
	}
	if got := camp.State().Actors["fit"].GetAbilityIds(); !slices.Equal(got, []string{"nope", "fists"}) {
		t.Errorf("the appended actor names %q, want [nope fists]", got)
	}
}
