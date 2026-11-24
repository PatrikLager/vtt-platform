package gateway

import (
	"fmt"

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
	if a, held := st.Actors[cmd.GetActorId()]; held && !rules.Offers(a, cmd.GetAbilityId()) {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: fmt.Sprintf(
			"gateway: use_ability: actor %q does not name ability %q", cmd.GetActorId(), cmd.GetAbilityId())}
	}

	envs, err := rules.Resolve(s.ruleset, st, cmd, s.roller)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	if entry := s.rulesetNamed(st); entry != nil {
		envs = append([]*vttv1.Envelope{entry}, envs...)
	}

	// Stamp what Resolve and rulesetNamed leave zero: the store needs an EventId
	// (SPEC-012), and the entry names its issuer (VTT-976).
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

// Prepend its entry to each use's and take's batch; needs s.ruleset (SPEC-026).
func (s *Server) rulesetNamed(st *engine.State) *vttv1.Envelope {
	rs := s.ruleset
	if st.RulesetID == rs.ID && st.RulesetVersion == rs.Version {
		return nil
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_RulesetNamed{RulesetNamed: &vttv1.RulesetNamed{
		RulesetId: rs.ID, Version: rs.Version, Name: rs.Name}}}
}
