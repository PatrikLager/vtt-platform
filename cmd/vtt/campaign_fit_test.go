package main

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

const fitRulesetDir = "../../rulesets/tavern-brawl"

func writeFitCampaign(t *testing.T, actors ...*vttv1.Actor) string {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actors {
		a.Kind = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
		if _, err := c.Append(&vttv1.Envelope{EventId: "add-" + a.GetActorId(),
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func misfitCampaign(t *testing.T) string {
	t.Helper()
	return writeFitCampaign(t,
		&vttv1.Actor{ActorId: "zed", Attributes: map[string]int32{"nope": 1, "brawn": 1}},
		&vttv1.Actor{ActorId: "fit", Attributes: map[string]int32{"brawn": 2},
			Resources: map[string]*vttv1.Resource{"drink": {Current: 0, Max: 5}}},
		&vttv1.Actor{ActorId: "abe", Attributes: map[string]int32{"zz": 1, "Za": 1},
			Resources: map[string]*vttv1.Resource{"brawn": {Current: 1}}},
	)
}

// VTT-678 VTT-681
func TestACampaignWhoseSheetsDoNotFitTheRulesetIsNotServed(t *testing.T) {
	dir := misfitCampaign(t)
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", fitRulesetDir, "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose actors hold keys the ruleset does not have was served")
	}
	msg := err.Error()
	abe := strings.Index(msg, `actor "abe": attributes: references undeclared property "Za"`)
	zed := strings.Index(msg, `actor "zed": attributes: references undeclared property "nope"`)
	if abe < 0 || zed < 0 || abe > zed {
		t.Fatalf("error = %q, want abe's first misfit key, then zed's", msg)
	}
	if strings.Contains(msg, `"fit"`) || strings.Contains(msg, `"zz"`) || strings.Contains(msg, `"brawn"`) {
		t.Errorf("error = %q, names a fitting actor or a key after an actor's first misfit", msg)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("the refused boot left the campaign held: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

// VTT-678
func TestACampaignWhoseSheetsFitTheRulesetIsServed(t *testing.T) {
	dir := writeFitCampaign(t,
		&vttv1.Actor{ActorId: "fit", Attributes: map[string]int32{"brawn": 2},
			Resources: map[string]*vttv1.Resource{"drink": {Current: 0, Max: 5}}},
		&vttv1.Actor{ActorId: "bare"},
	)
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", fitRulesetDir, "")
	if err != nil {
		t.Fatalf("a campaign whose sheets fit was refused: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}

// VTT-676
func TestACampaignIsServedWithNoRulesetWhateverKeysItsActorsHold(t *testing.T) {
	_, closeFn, err := composeServer(misfitCampaign(t), "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("with no ruleset, a campaign was refused for its actors' keys: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
