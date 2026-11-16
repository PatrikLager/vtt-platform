package main

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-777
func TestACampaignWhoseActorsBreakTheSpendRuleIsNotServedNamingEachActorAndItsFirstFault(t *testing.T) {
	dir := writeFitCampaign(t,
		&vttv1.Actor{ActorId: "swinger", AbilityIds: []string{"fists", "chair-swing"}},
		&vttv1.Actor{ActorId: "fit", AbilityIds: []string{"chair-swing"},
			Resources: map[string]*vttv1.Resource{"stamina": {Current: 2, Max: 2}}},
		&vttv1.Actor{ActorId: "hoarder", Resources: map[string]*vttv1.Resource{"stamina": {Current: 1}}},
	)
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", fitRulesetDir, "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose actors break the spend rule was served")
	}
	msg := err.Error()
	hoarder := strings.Index(msg, `actor "hoarder": resources: holds resource "stamina", which no ability the sheet names spends`)
	swinger := strings.Index(msg, `actor "swinger": ability_ids: names ability "chair-swing", which spends resource "stamina" the sheet does not hold`)
	if hoarder < 0 || swinger < 0 || hoarder > swinger {
		t.Fatalf("error = %q, want hoarder's orphan, then swinger's ability without its resource", msg)
	}
	if strings.Contains(msg, `"fit"`) {
		t.Errorf("error = %q, names the fitting actor", msg)
	}
}

// VTT-778
func TestWithNoRulesetACampaignWhoseActorsBreakTheSpendRuleIsServed(t *testing.T) {
	dir := writeFitCampaign(t,
		&vttv1.Actor{ActorId: "swinger", AbilityIds: []string{"chair-swing"}},
		&vttv1.Actor{ActorId: "hoarder", Resources: map[string]*vttv1.Resource{"stamina": {Current: 1}}},
	)
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("with no ruleset served, the campaign was refused: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
