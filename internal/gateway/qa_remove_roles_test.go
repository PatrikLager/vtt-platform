package gateway_test

import (
	"errors"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func qaRemoveCmd(target string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveParticipant{
		RemoveParticipant: &vttv1.RemoveParticipant{ParticipantId: target}}}
}

// VTT-565
func TestQARemoveParticipantOnlyDmAndAgentMayIssueIt(t *testing.T) {
	cases := []struct {
		role identity.Role
		ok   bool
	}{
		{identity.RoleDM, true},
		{"agent", true},
		{"player", false},
		{"spectator", false},
	}
	for _, c := range cases {
		p := &identity.Participant{ID: "p-1", Name: "Somebody", Role: c.role}
		err := gateway.Authorize(p, qaRemoveCmd("someone-else"), engine.NewState())
		if c.ok && err != nil {
			t.Errorf("role %s refused: %v", c.role, err)
		}
		if !c.ok {
			if err == nil {
				t.Errorf("role %s allowed", c.role)
			} else if !errors.Is(err, gateway.ErrUnauthorized) {
				t.Errorf("role %s refusal does not wrap ErrUnauthorized: %v", c.role, err)
			}
		}
	}
}

// VTT-565
func TestQARemoveParticipantPlayerRefusedEvenForSelf(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Name: "Pip", Role: "player"}
	if err := gateway.Authorize(p, qaRemoveCmd("p-1"), engine.NewState()); err == nil {
		t.Fatal("a player removed themself")
	}
	dm := &identity.Participant{ID: "d-1", Name: "Dana", Role: identity.RoleDM}
	if err := gateway.Authorize(dm, qaRemoveCmd("p-1"), engine.NewState()); err != nil {
		t.Fatalf("dm refused: %v", err)
	}
}
