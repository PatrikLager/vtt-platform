package main

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

func abilityMisfitCampaign(t *testing.T) string {
	t.Helper()
	return writeFitCampaign(t,
		&vttv1.Actor{ActorId: "zed", AbilityIds: []string{"fists", "nope", "Bad"}},
		&vttv1.Actor{ActorId: "fit", AbilityIds: []string{"fists", "chair-swing"}},
		&vttv1.Actor{ActorId: "abe", Attributes: map[string]int32{"zz": 1}, AbilityIds: []string{"missing"}},
	)
}

// VTT-688
func TestACampaignWhoseActorsNameAbilitiesTheRulesetDoesNotHoldIsNotServed(t *testing.T) {
	dir := abilityMisfitCampaign(t)
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", fitRulesetDir, "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose actors name abilities the ruleset does not hold was served")
	}
	msg := err.Error()
	abe := strings.Index(msg, `actor "abe": attributes: references undeclared property "zz"`)
	zed := strings.Index(msg, `actor "zed": ability_ids: names ability "Bad", which the ruleset does not hold`)
	if abe < 0 || zed < 0 || abe > zed {
		t.Fatalf("error = %q, want abe's misfit key, then zed's first unheld ability", msg)
	}
	if strings.Contains(msg, `"fit"`) || strings.Contains(msg, `"missing"`) || strings.Contains(msg, `"nope"`) {
		t.Errorf("error = %q, names a fitting actor, or an ability after an actor's first fault", msg)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("the refused boot left the campaign held: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

// VTT-688
func TestACampaignWhoseActorsNameHeldAbilitiesIsServed(t *testing.T) {
	dir := writeFitCampaign(t, &vttv1.Actor{ActorId: "fit", AbilityIds: []string{"fists", "chair-swing", "fists"}})
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", fitRulesetDir, "")
	if err != nil {
		t.Fatalf("a campaign whose actors name held abilities was refused: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}

// VTT-689
func TestACampaignIsServedWithNoRulesetWhateverAbilitiesItsActorsName(t *testing.T) {
	_, closeFn, err := composeServer(abilityMisfitCampaign(t), "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("with no ruleset, a campaign was refused for its actors' abilities: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
