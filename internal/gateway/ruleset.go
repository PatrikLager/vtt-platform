package gateway

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const errNoRulesetLoaded = "gateway: no ruleset loaded"

// Answer every failure as an ok=false result, never a close: authorization
// already ran in handleCommand (SPEC-012).
func (s *Server) handleUseAbility(requestID string, cmd *vttv1.UseAbility, st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	if s.ruleset == nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: errNoRulesetLoaded}
	}

	envs, err := rules.Resolve(s.ruleset, st, cmd, s.roller)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// Stamp the four fields Resolve leaves zero: store.AppendBatch requires an
	// EventId (SPEC-012).
	now := timestamppb.Now()
	for _, env := range envs {
		id, err := newEventID()
		if err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
		env.EventId = id
		env.ParticipantId = p.ID
		env.ActorRole = string(p.Role)
		env.OccurredAt = now
	}

	firstSeq, err := s.campaign.AppendBatch(envs)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq}
}
