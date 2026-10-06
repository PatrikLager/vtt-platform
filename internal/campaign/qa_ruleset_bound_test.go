package campaign_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

const (
	qaRuleCondition = "ConditionApplied"
	qaRuleAbility   = "AbilityUsed"
	qaRuleActorID   = "qa-actor"
)

func qaRuleEvents(kind, id string) []*vttv1.Envelope {
	actor := &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: qaRuleActorID, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}}}}
	ev := &vttv1.Envelope{}
	switch kind {
	case qaRuleCondition:
		ev.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
			ActorId: qaRuleActorID, ConditionId: id, Source: "qa-source",
		}}
	case qaRuleAbility:
		ev.Payload = &vttv1.Envelope_AbilityUsed{AbilityUsed: &vttv1.AbilityUsed{
			ActorId: qaRuleActorID, AbilityId: id, TargetIds: []string{qaRuleActorID}, OutcomeSummary: "qa",
		}}
	}
	return []*vttv1.Envelope{actor, ev}
}

func qaRuleFoldError(t *testing.T, envs []*vttv1.Envelope) string {
	t.Helper()
	st := engine.NewState()
	for i, env := range envs {
		e := &vttv1.Envelope{EventId: fmt.Sprintf("qa-rule-fold-%d", i), Sequence: int64(i + 1), Payload: env.GetPayload()}
		if err := engine.Apply(st, e); err != nil {
			return err.Error()
		}
	}
	t.Fatalf("the fold accepted every event of the log the test expects a campaign to refuse")
	return ""
}

func qaRuleLog(t *testing.T, envs []*vttv1.Envelope) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	for i, env := range envs {
		env.EventId = fmt.Sprintf("qa-rule-log-%d", i)
		if _, err := s.Append(env); err != nil {
			t.Fatalf("store.Append %d: %v", i, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
	return dir
}

// VTT-285 VTT-286
func TestQARuleACampaignWhoseLogHoldsARulesetIDOutsideTheBoundDoesNotOpen(t *testing.T) {
	refused := []struct{ label, id string }{
		{"129 ascii", strings.Repeat("a", 129)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
		{"empty", ""},
	}
	for _, kind := range []string{qaRuleCondition, qaRuleAbility} {
		for _, c := range refused {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				foldErr := qaRuleFoldError(t, qaRuleEvents(kind, c.id))
				opened, err := campaign.Open(qaRuleLog(t, qaRuleEvents(kind, c.id)))
				if err == nil {
					_ = opened.Close()
					t.Fatalf("campaign.Open accepted a log holding a %s id of %d bytes", kind, len(c.id))
				}
				msg := err.Error()
				if !strings.Contains(msg, foldErr) {
					t.Errorf("campaign.Open's refusal does not carry the fold's %q", foldErr)
				}
				if c.id != "" {
					msg = strings.ReplaceAll(msg, c.id, "<id>")
				}
				t.Logf("refusal: %s", msg)
			})
		}
		t.Run(kind+"/42 three-byte runes and two ascii", func(t *testing.T) {
			id := strings.Repeat("€", 42) + "ab"
			opened, err := campaign.Open(qaRuleLog(t, qaRuleEvents(kind, id)))
			if err != nil {
				t.Fatalf("campaign.Open refused a log holding a %s id of %d bytes: %v", kind, len(id), err)
			}
			defer func() { _ = opened.Close() }()
			if kind != qaRuleCondition {
				return
			}
			held := false
			for _, c := range opened.State().Conditions[qaRuleActorID] {
				held = held || c.ID == id
			}
			if !held {
				t.Fatalf("the reopened campaign does not hold the %d-byte condition id", len(id))
			}
		})
	}
}
