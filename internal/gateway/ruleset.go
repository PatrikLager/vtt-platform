package gateway

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// errNoRulesetLoaded is handleUseAbility's clean, ok=false error when s has
// no ruleset configured (spec §7 binding: "serving without one keeps
// today's behavior — UseAbility commands are then rejected with 'no
// ruleset loaded'"). A plain string, not a sentinel error var: nothing else
// in this package needs to errors.Is/As against it, and every other
// clean-error path in this file (Resolve's own validation errors) is
// likewise surfaced as bare CommandResult.Error text, never a typed error
// value the wire protocol has no way to carry anyway.
const errNoRulesetLoaded = "gateway: no ruleset loaded"

// handleUseAbility runs the authorized-use_ability pipeline (spec §7,
// ruleset-interpreter Task 6): rules.Resolve against a read-only state
// snapshot and the server's crypto Roller, then one campaign.AppendBatch
// for the whole ordered event batch Resolve returns. Every failure —
// including "no ruleset loaded" and every validation error Resolve itself
// returns (unknown actor/ability, out of range, insufficient resource,
// etc.) — is a clean ok=false CommandResult, never a connection drop: the
// caller (handleCommand) already ran Authorize before reaching here, so
// everything from this point on is either a config gap or a game-rules
// rejection, not an authorization concern.
func (s *Server) handleUseAbility(requestID string, cmd *vttv1.UseAbility, st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	if s.ruleset == nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: errNoRulesetLoaded}
	}

	envs, err := rules.Resolve(s.ruleset, st, cmd, s.roller)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// rules.Resolve leaves EventId/ParticipantId/ActorRole/OccurredAt zero
	// on every envelope it returns (Resolve's own doc comment: "matching
	// gateway.ToEvent's convention for every other command-to-event
	// conversion") — stamp them here, the same four fields ToEvent stamps
	// for a single-event command, before handing the batch to
	// campaign.AppendBatch (which itself only stamps SessionId — see
	// campaign.go's AppendBatch doc comment — and store.AppendBatch REQUIRES
	// a non-empty EventId per envelope, store.go's own validation).
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
