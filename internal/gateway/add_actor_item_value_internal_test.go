package gateway

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func itemValueRuleset() *rules.Ruleset {
	return &rules.Ruleset{
		Properties: []rules.Property{
			{Key: "quebec", Type: rules.PropertyNumber},
			{Key: "romeo", Type: rules.PropertyNumber},
			{Key: "sierra", Type: rules.PropertyNumber},
		},
		Forms: []rules.Form{
			{Key: "zulu", Properties: []string{"romeo"}},
			{Key: "yankee", Properties: []string{"quebec", "romeo"}},
		},
	}
}

// VTT-1062 VTT-677
func TestAddActorWithARulesetRefusesAValueACatalogueFormLists(t *testing.T) {
	cases := []struct {
		name       string
		attributes map[string]int32
		want       string
	}{
		{"a key a form lists", map[string]int32{"sierra": 1, "quebec": 2},
			`gateway: add_actor: actor "fit": attributes: holds property "quebec", which form "yankee" lists for its entries`},
		{"an undeclared key is told first", map[string]int32{"quebec": 2, "victor": 1},
			`gateway: add_actor: actor "fit": attributes: references undeclared property "victor"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, camp, dm := fitServer(t, itemValueRuleset())
			r := s.handleCommand(dm, fitAddActor("fit", c.attributes, nil), nil)
			if r.GetOk() {
				t.Fatal("add_actor accepted an actor holding a key a form lists")
			}
			if r.GetError() != c.want {
				t.Errorf("error = %q, want %q", r.GetError(), c.want)
			}
			if _, added := camp.State().Actors["fit"]; added {
				t.Error("the refused actor was appended")
			}
		})
	}
}

// VTT-1062
func TestAddActorWithARulesetAcceptsANumberNoFormLists(t *testing.T) {
	s, camp, dm := fitServer(t, itemValueRuleset())
	r := s.handleCommand(dm, fitAddActor("fit", map[string]int32{"sierra": 0}, nil), nil)
	if !r.GetOk() {
		t.Fatalf("add_actor refused a number no form lists: %s", r.GetError())
	}
	if _, added := camp.State().Actors["fit"]; !added {
		t.Error("the accepted actor was not appended")
	}
}
