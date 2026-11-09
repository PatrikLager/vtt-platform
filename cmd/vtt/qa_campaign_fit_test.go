package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

const qaFitRulesetDir = "../../rulesets/tavern-brawl"

func qaFitSheet(id string, attrs map[string]int32, res map[string]*vttv1.Resource) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       id,
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Attributes: attrs,
		Resources:  res,
	}
}

func qaFitRes(current, maximum int32) *vttv1.Resource {
	return &vttv1.Resource{Current: current, Max: maximum}
}

func qaFitAdded(n int, a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId: fmt.Sprintf("qa-fit-event-%d", n),
		Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}},
	}
}

func qaFitCampaign(t *testing.T, envs ...*vttv1.Envelope) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	for _, env := range envs {
		if _, err := c.Append(env); err != nil {
			_ = c.Close()
			t.Fatalf("Append %s: %v", env.GetEventId(), err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return dir
}

func qaFitActors(t *testing.T, actors ...*vttv1.Actor) string {
	t.Helper()
	envs := make([]*vttv1.Envelope, 0, len(actors))
	for i, a := range actors {
		envs = append(envs, qaFitAdded(i, a))
	}
	return qaFitCampaign(t, envs...)
}

func qaFitCompose(t *testing.T, dir, rulesetDir string) error {
	t.Helper()
	srv, closeFn, err := composeServer(dir, "127.0.0.1:0", rulesetDir, "")
	if err != nil {
		if closeFn != nil {
			_ = closeFn()
		}
		return err
	}
	if srv == nil {
		t.Errorf("composeServer returned no server and no error")
	}
	if closeFn == nil {
		t.Fatalf("composeServer returned no close function")
	}
	if cerr := closeFn(); cerr != nil {
		t.Errorf("close: %v", cerr)
	}
	return nil
}

func qaFitRefused(t *testing.T, dir string) string {
	t.Helper()
	err := qaFitCompose(t, dir, qaFitRulesetDir)
	if err == nil {
		t.Fatalf("composeServer with the ruleset served a campaign whose sheets do not fit it")
	}
	return err.Error()
}

func qaFitNames(t *testing.T, msg string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(msg, w) {
			t.Errorf("refusal does not name %q:\n%s", w, msg)
		}
	}
}

func qaFitNamesNot(t *testing.T, msg string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(msg, u) {
			t.Errorf("refusal names %q, which it must not:\n%s", u, msg)
		}
	}
}

// VTT-678
func TestQAFitAnUndeclaredAttributeKeyIsRefusedNamingActorAndKey(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone",
		map[string]int32{"brawn": 2, "qaUndeclaredAttr": 1}, nil))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "qaUndeclaredAttr")
	qaFitNamesNot(t, msg, "brawn")
}

// VTT-678
func TestQAFitAnUndeclaredResourceKeyIsRefusedNamingActorAndKey(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone", nil,
		map[string]*vttv1.Resource{"stamina": qaFitRes(2, 2), "qaUndeclaredRes": qaFitRes(1, 1)}))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "qaUndeclaredRes")
	qaFitNamesNot(t, msg, "stamina")
}

// VTT-678
func TestQAFitAResourceKeyUnderAttributesIsRefused(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone",
		map[string]int32{"brawn": 1, "drink": 1}, nil))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "drink")
	qaFitNamesNot(t, msg, "brawn")
}

// VTT-678
func TestQAFitANumberKeyUnderResourcesIsRefused(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone", nil,
		map[string]*vttv1.Resource{"stamina": qaFitRes(1, 2), "grit": qaFitRes(1, 1)}))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "grit")
	qaFitNamesNot(t, msg, "stamina")
}

// VTT-678
func TestQAFitTheFirstMisfitAttributeInByteOrderIsNamed(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone",
		map[string]int32{"brawn": 1, "qaable": 1, "qaZed": 1, "qamid": 1, "grit": 1}, nil))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "qaZed")
	qaFitNamesNot(t, msg, "qaable", "qamid", "brawn", "grit")
}

// VTT-678
func TestQAFitTheFirstMisfitResourceInByteOrderIsNamed(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone", nil, map[string]*vttv1.Resource{
		"drink":   qaFitRes(0, 3),
		"qaapple": qaFitRes(1, 1),
		"qaQuux":  qaFitRes(1, 1),
		"stamina": qaFitRes(2, 2),
	}))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "qaQuux")
	qaFitNamesNot(t, msg, "qaapple", "drink", "stamina")
}

// VTT-678
func TestQAFitBothKindsOfMisfitShareOneByteOrder(t *testing.T) {
	undeclaredFirst := qaFitActors(t, qaFitSheet("qa-lone",
		map[string]int32{"stamina": 1, "qaundeclared": 1}, nil))
	msg := qaFitRefused(t, undeclaredFirst)
	qaFitNames(t, msg, "qa-lone", "qaundeclared")
	qaFitNamesNot(t, msg, "stamina")

	otherTypeFirst := qaFitActors(t, qaFitSheet("qa-lone",
		map[string]int32{"drink": 1, "qaundeclared": 1}, nil))
	msg = qaFitRefused(t, otherTypeFirst)
	qaFitNames(t, msg, "qa-lone", "drink")
	qaFitNamesNot(t, msg, "qaundeclared")
}

// VTT-678
func TestQAFitAttributesAreReadBeforeResources(t *testing.T) {
	dir := qaFitActors(t, qaFitSheet("qa-lone",
		map[string]int32{"brawn": 1, "qazAttr": 1},
		map[string]*vttv1.Resource{"qaAAres": qaFitRes(1, 1), "drink": qaFitRes(0, 3)}))
	msg := qaFitRefused(t, dir)
	qaFitNames(t, msg, "qa-lone", "qazAttr")
	qaFitNamesNot(t, msg, "qaAAres")
}

func qaFitSeveralMisfits(t *testing.T) string {
	t.Helper()
	return qaFitActors(t,
		qaFitSheet("qa-mira", map[string]int32{"qaMiraLater": 1, "qaMiraFirst": 1, "grit": 1}, nil),
		qaFitSheet("qa-fits", map[string]int32{"brawn": 3, "grit": 1, "footing": 2},
			map[string]*vttv1.Resource{"drink": qaFitRes(0, 4), "stamina": qaFitRes(2, 2)}),
		qaFitSheet("qa-Bram", map[string]int32{"footing": 1},
			map[string]*vttv1.Resource{"qaBramRes": qaFitRes(1, 1), "qaBramZz": qaFitRes(1, 1)}),
		qaFitSheet("qa-bare", nil, nil),
		qaFitSheet("qa-anna", nil, map[string]*vttv1.Resource{"brawn": qaFitRes(1, 1), "stamina": qaFitRes(1, 2)}),
	)
}

// VTT-678
func TestQAFitEveryMisfitActorIsNamedWithItsFirstKey(t *testing.T) {
	msg := qaFitRefused(t, qaFitSeveralMisfits(t))
	qaFitNames(t, msg, "qa-mira", "qaMiraFirst", "qa-Bram", "qaBramRes", "qa-anna", "brawn")
	qaFitNamesNot(t, msg, "qa-fits", "qa-bare", "qaMiraLater", "qaBramZz", "stamina", "footing")
}

// VTT-681
func TestQAFitMisfitActorsAreNamedInByteOrderOfTheirIds(t *testing.T) {
	msg := qaFitRefused(t, qaFitSeveralMisfits(t))
	order := []string{"qa-Bram", "qa-anna", "qa-mira"}
	last := -1
	for _, id := range order {
		at := strings.Index(msg, id)
		if at < 0 {
			t.Fatalf("refusal does not name %q:\n%s", id, msg)
		}
		if at <= last {
			t.Errorf("actor %q is named out of byte order of the ids %v:\n%s", id, order, msg)
		}
		last = at
	}
}

// VTT-678
func TestQAFitACampaignWhoseSheetsFitIsServed(t *testing.T) {
	dir := qaFitActors(t,
		qaFitSheet("qa-whole", map[string]int32{"brawn": 3, "grit": 1, "footing": 2},
			map[string]*vttv1.Resource{"drink": qaFitRes(0, 4), "stamina": qaFitRes(2, 2)}),
		qaFitSheet("qa-bare", nil, nil),
		qaFitSheet("qa-numbers-only", map[string]int32{"grit": 0}, nil),
		qaFitSheet("qa-resources-only", nil, map[string]*vttv1.Resource{"stamina": qaFitRes(0, 0)}),
		qaFitSheet("qa-some-of-each", map[string]int32{"footing": 5},
			map[string]*vttv1.Resource{"drink": qaFitRes(2, 3)}),
	)
	if err := qaFitCompose(t, dir, qaFitRulesetDir); err != nil {
		t.Fatalf("a campaign whose sheets fit the ruleset was refused: %v", err)
	}
}

// VTT-678
func TestQAFitACampaignWithNoActorsIsServed(t *testing.T) {
	dir := qaFitCampaign(t)
	if err := qaFitCompose(t, dir, qaFitRulesetDir); err != nil {
		t.Fatalf("a campaign with no actors was refused: %v", err)
	}
}

// VTT-678
func TestQAFitARefusedCampaignCanBeOpenedAgain(t *testing.T) {
	dir := qaFitSeveralMisfits(t)
	_ = qaFitRefused(t, dir)

	c, err := campaign.Open(dir)
	if errors.Is(err, campaign.ErrHeld) {
		t.Fatalf("the refused boot left the campaign directory held: %v", err)
	}
	if err != nil {
		t.Fatalf("campaign.Open after a refused boot: %v", err)
	}
	if got := len(c.Actors()); got != 5 {
		t.Errorf("the reopened campaign holds %d actors, want the 5 it was given", got)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_ = qaFitRefused(t, dir)
	if err := qaFitCompose(t, dir, ""); err != nil {
		t.Fatalf("after two refused boots, the campaign was not served with no ruleset: %v", err)
	}
}

// VTT-676
func TestQAFitWithNoRulesetAMisfitCampaignIsServed(t *testing.T) {
	dir := qaFitActors(t,
		qaFitSheet("qa-undeclared", map[string]int32{"qaUndeclaredAttr": 1},
			map[string]*vttv1.Resource{"qaUndeclaredRes": qaFitRes(1, 1)}),
		qaFitSheet("qa-crossed", map[string]int32{"drink": 1},
			map[string]*vttv1.Resource{"grit": qaFitRes(1, 1)}),
	)
	if err := qaFitCompose(t, dir, qaFitRulesetDir); err == nil {
		t.Fatalf("precondition: with the ruleset, this campaign must be refused")
	}
	if err := qaFitCompose(t, dir, ""); err != nil {
		t.Fatalf("with no ruleset, a campaign was refused for its actors' keys: %v", err)
	}
}

// VTT-676
func TestQAFitWithNoRulesetTheSeveralMisfitCampaignIsServed(t *testing.T) {
	if err := qaFitCompose(t, qaFitSeveralMisfits(t), ""); err != nil {
		t.Fatalf("with no ruleset, a campaign was refused for its actors' keys: %v", err)
	}
}

// VTT-678
func TestQAFitARemovedMisfitActorDoesNotRefuseTheCampaign(t *testing.T) {
	dir := qaFitCampaign(t,
		qaFitAdded(0, qaFitSheet("qa-gone", map[string]int32{"qaUndeclaredAttr": 1}, nil)),
		qaFitAdded(1, qaFitSheet("qa-stays", map[string]int32{"brawn": 1}, nil)),
		&vttv1.Envelope{
			EventId: "qa-fit-event-removed",
			Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "qa-gone"}},
		},
	)
	if err := qaFitCompose(t, dir, qaFitRulesetDir); err != nil {
		t.Fatalf("a campaign whose only misfit actor was removed was refused: %v", err)
	}
}
