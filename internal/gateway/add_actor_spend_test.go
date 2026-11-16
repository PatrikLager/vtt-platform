package gateway_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func addActorCmd(a *vttv1.Actor) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}}}
}

func spendSheet(id string, abilities []string, resources map[string]*vttv1.Resource) *vttv1.Actor {
	return &vttv1.Actor{ActorId: id, Name: id, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Attributes: map[string]int32{"brawn": 1}, AbilityIds: abilities, Resources: resources}
}

var stamina = map[string]*vttv1.Resource{"stamina": {Current: 2, Max: 2}}

// VTT-775
func TestAddActorRefusesASheetThatBreaksTheSpendRule(t *testing.T) {
	f := newRulesetFixture(t, true)
	refusedAppendingNothing(t, f, addActorCmd(spendSheet("swinger", []string{"fists", "chair-swing"}, nil)),
		`gateway: add_actor: actor "swinger": ability_ids: names ability "chair-swing", which spends resource "stamina" the sheet does not hold`)
	refusedAppendingNothing(t, f, addActorCmd(spendSheet("hoarder", []string{"fists"}, stamina)),
		`gateway: add_actor: actor "hoarder": resources: holds resource "stamina", which no ability the sheet names spends`)
	refusedAppendingNothing(t, f, addActorCmd(spendSheet("both", nil, map[string]*vttv1.Resource{"stamina": {Current: 1}})),
		`resources: holds resource "stamina"`)
}

// VTT-775
func TestAddActorAcceptsASheetThatKeepsTheSpendRule(t *testing.T) {
	f := newRulesetFixture(t, true)
	acceptedAppending(t, f, addActorCmd(spendSheet("swinger", []string{"chair-swing"}, stamina)), 1)
	acceptedAppending(t, f, addActorCmd(spendSheet("drinker", nil, map[string]*vttv1.Resource{"drink": {Current: 1, Max: 4}})), 1)
}

// VTT-778
func TestWithNoRulesetAddActorAcceptsASheetThatBreaksTheSpendRule(t *testing.T) {
	f := newRulesetFixture(t, false)
	acceptedAppending(t, f, addActorCmd(spendSheet("swinger", []string{"chair-swing"}, nil)), 1)
	acceptedAppending(t, f, addActorCmd(spendSheet("hoarder", nil, stamina)), 1)
}
